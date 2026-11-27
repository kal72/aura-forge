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
		PinID            int64  `json:"pin_id"`
		TypeID           int32  `json:"type_id"`
		SchematicID      int32  `json:"schematic_id"`
		ExpiryTime       string `json:"expiry_time"`
		ExtractorDetails *struct {
			ProductTypeID int32 `json:"product_type_id"`
			CycleTime     int   `json:"cycle_time"`
			QtyPerCycle   int   `json:"qty_per_cycle"`
		} `json:"extractor_details"`
		Contents []struct {
			TypeID int32 `json:"type_id"`
			Amount int64 `json:"amount"`
		} `json:"contents"`
	} `json:"pins"`
}

type esiPlanetMeta struct {
	Name string `json:"name"`
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

	// Lookup helper from domain PI catalog (typeID & name -> PICommodity)
	allPI := domain.ListProduciblePICommodities("ALL", "")
	piByID := make(map[int32]domain.PICommodity, len(allPI))
	piByName := make(map[string]domain.PICommodity, len(allPI))
	for _, it := range allPI {
		piByID[it.TypeID] = it
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
								if it, ok := piByID[pin.ExtractorDetails.ProductTypeID]; ok {
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

			// Format top 3 stored items for quick hauling overview
			type storedKV struct {
				name   string
				amount int64
			}
			storedList := make([]storedKV, 0, len(storageTotals))
			for tid, amt := range storageTotals {
				name := fmt.Sprintf("Item #%d", tid)
				if it, ok := piByID[tid]; ok {
					name = it.Name
				}
				storedList = append(storedList, storedKV{name: name, amount: amt})
			}
			sort.Slice(storedList, func(i, j int) bool { return storedList[i].amount > storedList[j].amount })
			if len(storedList) > 3 {
				storedList = storedList[:3]
			}
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
