package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"aura-forge/internal/domain"
)

type esiSeedMarketOrder struct {
	OrderID      int64   `json:"order_id"`
	TypeID       int32   `json:"type_id"`
	LocationID   int64   `json:"location_id"`
	SystemID     int32   `json:"system_id"`
	Price        float64 `json:"price"`
	VolumeRemain int     `json:"volume_remain"`
	Duration     int     `json:"duration"`
	IsBuyOrder   bool    `json:"is_buy_order"`
}

type esiStationDetail struct {
	StationID int64  `json:"station_id"`
	Name      string `json:"name"`
	SystemID  int32  `json:"system_id"`
}

type esiSystemDetail struct {
	SystemID       int32   `json:"system_id"`
	Name           string  `json:"name"`
	SecurityStatus float64 `json:"security_status"`
}

type esiLoyaltyOffer struct {
	OfferID       int32 `json:"offer_id"`
	TypeID        int32 `json:"type_id"`
	Quantity      int   `json:"quantity"`
	LPCost        int   `json:"lp_cost"`
	ISKCost       int64 `json:"isk_cost"`
	RequiredItems []struct {
		TypeID   int32 `json:"type_id"`
		Quantity int   `json:"quantity"`
	} `json:"required_items"`
}

type esiSystemKillsEntry struct {
	SystemID  int32 `json:"system_id"`
	ShipKills int   `json:"ship_kills"`
	PodKills  int   `json:"pod_kills"`
	NPCKills  int   `json:"npc_kills"`
}

type esiUniverseSystemsIDsResp struct {
	Systems []struct {
		ID   int32  `json:"id"`
		Name string `json:"name"`
	} `json:"systems"`
}

// FindNPCBPOSeedOrders queries cached Public ESI region sell orders for a blueprint type_id,
// filters official NPC seed orders (duration == 365 and NPC station ID < 100_000_000),
// and resolves station names with bounded concurrency.
func (p *PublicMarketClient) FindNPCBPOSeedOrders(ctx context.Context, regionID int32, regionName string, blueprintTypeID int32) ([]domain.NPCBPOSeedOrder, error) {
	if regionID <= 0 || blueprintTypeID <= 0 {
		return nil, nil
	}

	path := fmt.Sprintf("/markets/%d/orders/?datasource=tranquility&order_type=sell&type_id=%d", regionID, blueprintTypeID)
	body, err := p.client.DoGet(ctx, path, "", true)
	if err != nil {
		return nil, fmt.Errorf("fetching region orders for blueprint %d: %w", blueprintTypeID, err)
	}

	var orders []esiSeedMarketOrder
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("decoding blueprint orders: %w", err)
	}

	// Filter NPC seed orders: CCP NPC BPO seed orders have duration == 365 on NPC stations (< 100,000,000).
	seenStation := make(map[int64]struct{})
	seeds := make([]domain.NPCBPOSeedOrder, 0, 8)
	for _, o := range orders {
		if o.IsBuyOrder || o.Duration != 365 || o.LocationID <= 0 || o.LocationID >= 100_000_000 {
			continue
		}
		if _, dup := seenStation[o.LocationID]; dup {
			continue
		}
		seenStation[o.LocationID] = struct{}{}
		seeds = append(seeds, domain.NPCBPOSeedOrder{
			OrderID:    o.OrderID,
			TypeID:     o.TypeID,
			StationID:  o.LocationID,
			SystemID:   o.SystemID,
			RegionName: regionName,
			Price:      o.Price,
			Duration:   o.Duration,
		})
	}

	sort.Slice(seeds, func(i, j int) bool {
		return seeds[i].Price < seeds[j].Price
	})
	if len(seeds) > 8 {
		seeds = seeds[:8]
	}

	// Resolve station names in parallel with bounded concurrency (max 4 workers).
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i := range seeds {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			stPath := fmt.Sprintf("/universe/stations/%d/?datasource=tranquility", seeds[idx].StationID)
			stBody, stErr := p.client.DoGet(ctx, stPath, "", true)
			if stErr != nil {
				seeds[idx].StationName = fmt.Sprintf("NPC Station #%d", seeds[idx].StationID)
				return
			}
			var st esiStationDetail
			if json.Unmarshal(stBody, &st) == nil && st.Name != "" {
				seeds[idx].StationName = st.Name
			} else {
				seeds[idx].StationName = fmt.Sprintf("NPC Station #%d", seeds[idx].StationID)
			}
		}(i)
	}
	wg.Wait()

	return seeds, nil
}

// GetLoyaltyStoreOffers fetches cached Public ESI LP Store offers for an NPC Corporation.
func (p *PublicMarketClient) GetLoyaltyStoreOffers(ctx context.Context, corpID int32, corpName string) ([]domain.LoyaltyStoreOffer, error) {
	if corpID <= 0 {
		return nil, nil
	}
	path := fmt.Sprintf("/loyalty/stores/%d/offers/?datasource=tranquility", corpID)
	body, err := p.client.DoGet(ctx, path, "", true)
	if err != nil {
		return nil, fmt.Errorf("fetching LP store offers for corp %d: %w", corpID, err)
	}

	var raw []esiLoyaltyOffer
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding LP store offers: %w", err)
	}

	out := make([]domain.LoyaltyStoreOffer, 0, len(raw))
	for _, r := range raw {
		reqItems := make([]domain.LoyaltyStoreRequiredItem, 0, len(r.RequiredItems))
		for _, ri := range r.RequiredItems {
			reqItems = append(reqItems, domain.LoyaltyStoreRequiredItem{
				TypeID:   ri.TypeID,
				Quantity: ri.Quantity,
			})
		}
		out = append(out, domain.LoyaltyStoreOffer{
			OfferID:         r.OfferID,
			CorporationID:   corpID,
			CorporationName: corpName,
			TypeID:          r.TypeID,
			Quantity:        r.Quantity,
			LPCost:          r.LPCost,
			ISKCost:         r.ISKCost,
			RequiredItems:   reqItems,
		})
	}
	return out, nil
}

// GetRoute fetches a cached Public ESI route between two EVE solar systems (flag: "secure" | "shortest" | "insecure").
func (p *PublicMarketClient) GetRoute(ctx context.Context, originSystemID, destSystemID int32, flag string) ([]int32, error) {
	if originSystemID <= 0 || destSystemID <= 0 {
		return nil, fmt.Errorf("origin and destination solar system IDs must be positive")
	}
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case "shortest", "insecure":
		flag = strings.ToLower(strings.TrimSpace(flag))
	default:
		flag = "secure"
	}

	path := fmt.Sprintf("/route/%d/%d/?datasource=tranquility&flag=%s", originSystemID, destSystemID, flag)
	body, err := p.client.DoGet(ctx, path, "", true)
	if err != nil {
		return nil, fmt.Errorf("fetching %s route %d->%d: %w", flag, originSystemID, destSystemID, err)
	}
	var systems []int32
	if err := json.Unmarshal(body, &systems); err != nil {
		return nil, fmt.Errorf("decoding route response: %w", err)
	}
	return systems, nil
}

// GetSystemInfo fetches cached Public ESI solar system metadata (name and security status).
func (p *PublicMarketClient) GetSystemInfo(ctx context.Context, systemID int32) (*domain.SolarSystemInfo, error) {
	if systemID <= 0 {
		return nil, fmt.Errorf("invalid solar system ID")
	}
	path := fmt.Sprintf("/universe/systems/%d/?datasource=tranquility", systemID)
	body, err := p.client.DoGet(ctx, path, "", true)
	if err != nil {
		return nil, fmt.Errorf("fetching solar system %d: %w", systemID, err)
	}
	var detail esiSystemDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("decoding solar system %d: %w", systemID, err)
	}
	return &domain.SolarSystemInfo{
		SystemID:       detail.SystemID,
		Name:           detail.Name,
		SecurityStatus: detail.SecurityStatus,
	}, nil
}

// GetSystemKills fetches 1-hour ship/pod/NPC kill counts across all of New Eden in a single cached Public ESI call.
func (p *PublicMarketClient) GetSystemKills(ctx context.Context) (map[int32]domain.SystemKillStat, error) {
	body, err := p.client.DoGet(ctx, "/universe/system_kills/?datasource=tranquility", "", true)
	if err != nil {
		return nil, fmt.Errorf("fetching system kills: %w", err)
	}
	var entries []esiSystemKillsEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decoding system kills: %w", err)
	}
	out := make(map[int32]domain.SystemKillStat, len(entries))
	for _, e := range entries {
		out[e.SystemID] = domain.SystemKillStat{
			SystemID:  e.SystemID,
			ShipKills: e.ShipKills,
			PodKills:  e.PodKills,
			NPCKills:  e.NPCKills,
		}
	}
	return out, nil
}

// ResolveSolarSystemID resolves a major hub key or arbitrary EVE solar system name via POST /universe/ids/.
func (p *PublicMarketClient) ResolveSolarSystemID(ctx context.Context, name string) (int32, string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return 0, "", fmt.Errorf("solar system name is required")
	}

	// Fast-path for the 5 major trade hub keys/names (0 ESI calls).
	switch strings.ToLower(trimmed) {
	case "jita", "jita 4-4", "jita iv - moon 4":
		return 30000142, "Jita", nil
	case "amarr", "amarr viii":
		return 30002187, "Amarr", nil
	case "dodixie":
		return 30002659, "Dodixie", nil
	case "rens":
		return 30002510, "Rens", nil
	case "hek":
		return 30002053, "Hek", nil
	case "perimeter":
		return 30000144, "Perimeter", nil
	}

	body, err := p.client.DoPost(ctx, "/universe/ids/?datasource=tranquility", []string{trimmed})
	if err != nil {
		return 0, "", fmt.Errorf("resolving solar system %q: %w", trimmed, err)
	}
	var parsed esiUniverseSystemsIDsResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, "", fmt.Errorf("decoding solar system IDs: %w", err)
	}
	if len(parsed.Systems) == 0 {
		return 0, "", fmt.Errorf("solar system %q not found in EVE Online", trimmed)
	}
	return parsed.Systems[0].ID, parsed.Systems[0].Name, nil
}
