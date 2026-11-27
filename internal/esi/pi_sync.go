package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"aura-forge/internal/domain"
)

type esiCharacterPlanet struct {
	PlanetID      int32  `json:"planet_id"`
	SolarSystemID int32  `json:"solar_system_id"`
	PlanetType    string `json:"planet_type"`
	UpgradeLevel  int    `json:"upgrade_level"`
	NumPins       int    `json:"num_pins"`
	LastUpdate    string `json:"last_update"`
}

type esiPlanetDetail struct {
	Pins []struct {
		PinID            int64   `json:"pin_id"`
		TypeID           int32   `json:"type_id"`
		SchematicID      int32   `json:"schematic_id"`
		Latitude         float64 `json:"latitude"`
		Longitude        float64 `json:"longitude"`
		ExpiryTime       string  `json:"expiry_time"`
		ExtractorDetails *struct {
			ProductTypeID int32   `json:"product_type_id"`
			CycleTime     int     `json:"cycle_time"`
			QtyPerCycle   int     `json:"qty_per_cycle"`
			HeadRadius    float64 `json:"head_radius"`
			Heads         []struct {
				HeadID    int     `json:"head_id"`
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"heads"`
		} `json:"extractor_details"`
		Contents []struct {
			TypeID int32 `json:"type_id"`
			Amount int64 `json:"amount"`
		} `json:"contents"`
	} `json:"pins"`
	Links []struct {
		SourcePinID      int64 `json:"source_pin_id"`
		DestinationPinID int64 `json:"destination_pin_id"`
		LinkLevel        int   `json:"link_level"`
	} `json:"links"`
	Routes []struct {
		RouteID          int64   `json:"route_id"`
		SourcePinID      int64   `json:"source_pin_id"`
		DestinationPinID int64   `json:"destination_pin_id"`
		ContentTypeID    int32   `json:"content_type_id"`
		Quantity         float64 `json:"quantity"`
	} `json:"routes"`
}

type esiPlanetMeta struct {
	Name   string `json:"name"`
	TypeID int32  `json:"type_id"`
}

type esiSchematicMeta struct {
	SchematicName string `json:"schematic_name"`
	CycleTime     int    `json:"cycle_time"`
}

// SyncPIPlanets fetches all Planetary Industry colonies, locations, active P0–P4 production,
// launchpad/storage contents, and extractor expiry times for the authenticated EVE character.
func (c *CharacterSyncClient) SyncPIPlanets(ctx context.Context, token *domain.EVEToken) ([]domain.PIColony, error) {
	if token == nil || token.CharacterID <= 0 {
		return nil, fmt.Errorf("no connected EVE Online character")
	}
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/characters/%d/planets/?datasource=tranquility", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("fetching character PI planets (ensure esi-planets.manage_planets.v1 is enabled and reconnect EVE SSO): %w", err)
	}

	var rawPlanets []esiCharacterPlanet
	if err := json.Unmarshal(body, &rawPlanets); err != nil {
		return nil, fmt.Errorf("decoding character PI planets: %w", err)
	}
	if len(rawPlanets) == 0 {
		return []domain.PIColony{}, nil
	}

	// Lookup helper from domain PI catalog (name -> PICommodity for factory schematic resolution)
	allPI := domain.ListProduciblePICommodities("ALL", "")
	piByName := make(map[string]domain.PICommodity, len(allPI))
	for _, it := range allPI {
		piByName[strings.ToLower(it.Name)] = it
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		sem      = make(chan struct{}, 4)
		colonies = make([]domain.PIColony, 0, len(rawPlanets))
	)

	for _, rp := range rawPlanets {
		wg.Add(1)
		go func(p esiCharacterPlanet) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			planetName := fmt.Sprintf("Planet #%d", p.PlanetID)
			if b, pErr := c.esiClient.DoGet(ctx, fmt.Sprintf("/universe/planets/%d/?datasource=tranquility", p.PlanetID), "", true); pErr == nil {
				var pm esiPlanetMeta
				if json.Unmarshal(b, &pm) == nil && pm.Name != "" {
					planetName = pm.Name
				}
			}

			systemName := fmt.Sprintf("System #%d", p.SolarSystemID)
			secStatus := 0.0
			if b, sErr := c.esiClient.DoGet(ctx, fmt.Sprintf("/universe/systems/%d/?datasource=tranquility", p.SolarSystemID), "", true); sErr == nil {
				var sm esiSystemDetail
				if json.Unmarshal(b, &sm) == nil && sm.Name != "" {
					systemName = sm.Name
					secStatus = sm.SecurityStatus
				}
			}

			// Inspect live planet pins (extractors, factories, storage contents)
			var (
				earliestExpiry time.Time
				primaryProdID  int32
				primaryName    string
				primaryTier    domain.PITier
				factoryNames   []string
				seenFactory    = make(map[string]struct{})
				storageTotals  = make(map[int32]int64)
			)

			detailPath := fmt.Sprintf("/characters/%d/planets/%d/?datasource=tranquility", validTok.CharacterID, p.PlanetID)
			if b, dErr := c.esiClient.DoGet(ctx, detailPath, validTok.AccessToken, false); dErr == nil {
				var detail esiPlanetDetail
				if json.Unmarshal(b, &detail) == nil {
					for _, pin := range detail.Pins {
						// 1. Extractor expiry & extracted P0 product
						if pin.ExtractorDetails != nil {
							if pin.ExpiryTime != "" {
								if exp, tErr := time.Parse(time.RFC3339, pin.ExpiryTime); tErr == nil {
									if earliestExpiry.IsZero() || exp.Before(earliestExpiry) {
										earliestExpiry = exp
									}
								}
							}
							if primaryProdID == 0 && pin.ExtractorDetails.ProductTypeID > 0 {
								if it, ok := domain.FindPICommodityByID(pin.ExtractorDetails.ProductTypeID); ok {
									primaryProdID = it.TypeID
									primaryName = it.Name
									primaryTier = it.Tier
								}
							}
						}

						// 2. Factory schematic (P1–P4 output)
						if pin.SchematicID > 0 {
							schPath := fmt.Sprintf("/universe/schematics/%d/?datasource=tranquility", pin.SchematicID)
							if sb, scErr := c.esiClient.DoGet(ctx, schPath, "", true); scErr == nil {
								var sch esiSchematicMeta
								if json.Unmarshal(sb, &sch) == nil && sch.SchematicName != "" {
									if _, dup := seenFactory[sch.SchematicName]; !dup {
										seenFactory[sch.SchematicName] = struct{}{}
										factoryNames = append(factoryNames, sch.SchematicName)
									}
									if it, ok := piByName[strings.ToLower(sch.SchematicName)]; ok {
										// Prefer higher-tier factory output as primary colony product
										if primaryTier == "" || tierRank(it.Tier) >= tierRank(primaryTier) {
											primaryProdID = it.TypeID
											primaryName = it.Name
											primaryTier = it.Tier
										}
									}
								}
							}
						}

						// 3. Storage / Launchpad contents
						for _, item := range pin.Contents {
							if item.TypeID > 0 && item.Amount > 0 {
								storageTotals[item.TypeID] += item.Amount
							}
						}
					}
				}
			}

			if primaryName == "" {
				if len(factoryNames) > 0 {
					primaryName = factoryNames[0]
					primaryTier = domain.PITierP1
				} else {
					primaryName = "Idle / No Active Production"
					primaryTier = domain.PITierP0
				}
			}

			// Format stored items sorted by highest PI Tier first (P4 -> P3 -> P2 -> P1 -> P0), then quantity descending,
			// so finished products (e.g. P2 Enriched Uranium) are never hidden behind large P0/P1 raw input piles.
			type storedKV struct {
				name   string
				tier   domain.PITier
				amount int64
			}
			storedList := make([]storedKV, 0, len(storageTotals))
			for tid, amt := range storageTotals {
				name := fmt.Sprintf("Item #%d", tid)
				var tier domain.PITier
				if it, ok := domain.FindPICommodityByID(tid); ok {
					name = it.Name
					tier = it.Tier
				}
				storedList = append(storedList, storedKV{name: name, tier: tier, amount: amt})
			}
			sort.Slice(storedList, func(i, j int) bool {
				ri, rj := tierRank(storedList[i].tier), tierRank(storedList[j].tier)
				if ri != rj {
					return ri > rj
				}
				if storedList[i].amount != storedList[j].amount {
					return storedList[i].amount > storedList[j].amount
				}
				return storedList[i].name < storedList[j].name
			})
			storageParts := make([]string, 0, len(storedList))
			for _, kv := range storedList {
				storageParts = append(storageParts, fmt.Sprintf("%s ×%d", kv.name, kv.amount))
			}

			note := ""
			if len(factoryNames) > 1 {
				sort.Strings(factoryNames)
				note = "Factories: " + strings.Join(factoryNames, ", ")
			}

			pType := strings.TrimSpace(p.PlanetType)
			if len(pType) > 0 {
				pType = strings.ToUpper(pType[:1]) + strings.ToLower(pType[1:])
			}

			if earliestExpiry.IsZero() {
				// Factory-only planet (no extractor) — set far-future sentinel or last update
				earliestExpiry = time.Now().Add(72 * time.Hour)
			}

			mu.Lock()
			colonies = append(colonies, domain.PIColony{
				CharacterID:        validTok.CharacterID,
				PlanetID:           p.PlanetID,
				PlanetName:         planetName,
				SolarSystem:        systemName,
				SecurityStatus:     secStatus,
				PlanetType:         pType,
				UpgradeLevel:       p.UpgradeLevel,
				NumPins:            p.NumPins,
				ProductTypeID:      primaryProdID,
				ProductName:        primaryName,
				Tier:               primaryTier,
				ExtractorExpiresAt: earliestExpiry,
				StorageSummary:     strings.Join(storageParts, " • "),
				SyncedFromSSO:      true,
				Note:               note,
			})
			mu.Unlock()
		}(rp)
	}
	wg.Wait()

	sort.Slice(colonies, func(i, j int) bool {
		return colonies[i].PlanetName < colonies[j].PlanetName
	})
	return colonies, nil
}

func tierRank(t domain.PITier) int {
	switch t {
	case domain.PITierP4:
		return 4
	case domain.PITierP3:
		return 3
	case domain.PITierP2:
		return 2
	case domain.PITierP1:
		return 1
	default:
		return 0
	}
}

type esiTypeNameResp struct {
	Name string `json:"name"`
}

func classifyPIPinRole(typeName string, hasExtractor bool, schematicID int32) string {
	lower := strings.ToLower(typeName)
	switch {
	case hasExtractor || strings.Contains(lower, "extractor"):
		return "extractor"
	case strings.Contains(lower, "command center"):
		return "command_center"
	case strings.Contains(lower, "launchpad"):
		return "launchpad"
	case strings.Contains(lower, "storage"):
		return "storage"
	case strings.Contains(lower, "high-tech"):
		return "hightech_factory"
	case strings.Contains(lower, "advanced"):
		return "advanced_factory"
	case strings.Contains(lower, "basic") || schematicID > 0:
		return "basic_factory"
	default:
		return "facility"
	}
}

// GetPIPlanetLayout fetches the complete pin coordinates, extractor heads, links, and routes for one PI planet.
func (c *CharacterSyncClient) GetPIPlanetLayout(ctx context.Context, token *domain.EVEToken, planetID int32) (*domain.PIPlanetLayout, error) {
	if token == nil || token.CharacterID <= 0 || planetID <= 0 {
		return nil, fmt.Errorf("invalid character or planet_id")
	}
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	detailPath := fmt.Sprintf("/characters/%d/planets/%d/?datasource=tranquility", validTok.CharacterID, planetID)
	body, err := c.esiClient.DoGet(ctx, detailPath, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("fetching planet #%d layout: %w", planetID, err)
	}
	var detail esiPlanetDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("decoding planet #%d layout: %w", planetID, err)
	}

	planetName := fmt.Sprintf("Planet #%d", planetID)
	var planetIconTypeID int32 = 11
	if b, pErr := c.esiClient.DoGet(ctx, fmt.Sprintf("/universe/planets/%d/?datasource=tranquility", planetID), "", true); pErr == nil {
		var pm esiPlanetMeta
		if json.Unmarshal(b, &pm) == nil {
			if pm.Name != "" {
				planetName = pm.Name
			}
			if pm.TypeID > 0 {
				planetIconTypeID = pm.TypeID
			}
		}
	}

	allPI := domain.ListProduciblePICommodities("ALL", "")
	piByID := make(map[int32]string, len(allPI))
	piIDByName := make(map[string]int32, len(allPI))
	for _, it := range allPI {
		piByID[it.TypeID] = it.Name
		piIDByName[strings.ToLower(it.Name)] = it.TypeID
	}

	// Cache pin type names and schematic names (all cached in esi_cache)
	typeNameCache := make(map[int32]string)
	resolveTypeName := func(tid int32) string {
		if tid <= 0 {
			return ""
		}
		if comm, ok := domain.FindPICommodityByID(tid); ok {
			return comm.Name
		}
		if n, ok := piByID[tid]; ok {
			return n
		}
		if n, ok := typeNameCache[tid]; ok {
			return n
		}
		if b, tErr := c.esiClient.DoGet(ctx, fmt.Sprintf("/universe/types/%d/?datasource=tranquility", tid), "", true); tErr == nil {
			var tr esiTypeNameResp
			if json.Unmarshal(b, &tr) == nil && tr.Name != "" {
				typeNameCache[tid] = tr.Name
				return tr.Name
			}
		}
		return fmt.Sprintf("Type #%d", tid)
	}

	schematicCache := make(map[int32]string)
	resolveSchematic := func(sid int32) string {
		if sid <= 0 {
			return ""
		}
		if n, ok := schematicCache[sid]; ok {
			return n
		}
		if b, sErr := c.esiClient.DoGet(ctx, fmt.Sprintf("/universe/schematics/%d/?datasource=tranquility", sid), "", true); sErr == nil {
			var sm esiSchematicMeta
			if json.Unmarshal(b, &sm) == nil && sm.SchematicName != "" {
				schematicCache[sid] = sm.SchematicName
				return sm.SchematicName
			}
		}
		return fmt.Sprintf("Schematic #%d", sid)
	}

	pins := make([]domain.PIPlanetPin, 0, len(detail.Pins))
	for _, p := range detail.Pins {
		tName := resolveTypeName(p.TypeID)
		role := classifyPIPinRole(tName, p.ExtractorDetails != nil, p.SchematicID)
		schName := resolveSchematic(p.SchematicID)

		pin := domain.PIPlanetPin{
			PinID:         p.PinID,
			TypeID:        p.TypeID,
			Role:          role,
			Label:         tName,
			Latitude:      p.Latitude,
			Longitude:     p.Longitude,
			SchematicName: schName,
			ExpiryTime:    p.ExpiryTime,
		}
		if schName != "" {
			if comm, ok := domain.FindPICommodityByName(schName); ok {
				pin.ProductTypeID = comm.TypeID
				pin.ProductName = comm.Name
				pin.ProductTier = comm.Tier
			} else if prodID, ok := piIDByName[strings.ToLower(schName)]; ok {
				pin.ProductTypeID = prodID
				pin.ProductName = schName
			}
		}
		if p.ExtractorDetails != nil {
			pin.HeadRadius = p.ExtractorDetails.HeadRadius
			pin.QtyPerCycle = p.ExtractorDetails.QtyPerCycle
			pin.CycleSeconds = p.ExtractorDetails.CycleTime
			if p.ExtractorDetails.ProductTypeID > 0 {
				pin.ProductTypeID = p.ExtractorDetails.ProductTypeID
				pin.ProductName = resolveTypeName(p.ExtractorDetails.ProductTypeID)
				if comm, ok := domain.FindPICommodityByID(p.ExtractorDetails.ProductTypeID); ok {
					pin.ProductTier = comm.Tier
				}
			}
			for _, h := range p.ExtractorDetails.Heads {
				pin.Heads = append(pin.Heads, domain.PIExtractorHead{
					HeadID:    h.HeadID,
					Latitude:  h.Latitude,
					Longitude: h.Longitude,
				})
			}
		}
		for _, ct := range p.Contents {
			if ct.TypeID > 0 && ct.Amount > 0 {
				var ctTier domain.PITier
				if comm, ok := domain.FindPICommodityByID(ct.TypeID); ok {
					ctTier = comm.Tier
				}
				pin.Contents = append(pin.Contents, domain.PIPinContent{
					TypeID: ct.TypeID,
					Name:   resolveTypeName(ct.TypeID),
					Tier:   ctTier,
					Amount: ct.Amount,
				})
			}
		}
		pins = append(pins, pin)
	}

	links := make([]domain.PIPlanetLink, 0, len(detail.Links))
	for _, l := range detail.Links {
		links = append(links, domain.PIPlanetLink{
			SourcePinID:      l.SourcePinID,
			DestinationPinID: l.DestinationPinID,
			LinkLevel:        l.LinkLevel,
		})
	}

	routes := make([]domain.PIPlanetRoute, 0, len(detail.Routes))
	for _, rt := range detail.Routes {
		var rtTier domain.PITier
		if comm, ok := domain.FindPICommodityByID(rt.ContentTypeID); ok {
			rtTier = comm.Tier
		}
		routes = append(routes, domain.PIPlanetRoute{
			RouteID:          rt.RouteID,
			SourcePinID:      rt.SourcePinID,
			DestinationPinID: rt.DestinationPinID,
			ContentTypeID:    rt.ContentTypeID,
			ContentName:      resolveTypeName(rt.ContentTypeID),
			ContentTier:      rtTier,
			Quantity:         rt.Quantity,
		})
	}

	return &domain.PIPlanetLayout{
		PlanetID:   planetID,
		PlanetName: planetName,
		PlanetIcon: planetIconTypeID,
		Pins:       pins,
		Links:      links,
		Routes:     routes,
	}, nil
}
