package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

// PublicMarketClient implements domain.MarketDataProvider using unauthenticated EVE Public ESI endpoints.
type PublicMarketClient struct {
	client *Client
}

// NewPublicMarketClient creates a new PublicMarketClient.
func NewPublicMarketClient(client *Client) *PublicMarketClient {
	return &PublicMarketClient{client: client}
}

type esiUniverseIDsResponse struct {
	InventoryTypes []struct {
		ID   int32  `json:"id"`
		Name string `json:"name"`
	} `json:"inventory_types"`
}

// ResolveTypeIDs resolves EVE item names to official type_id values via POST /universe/ids/.
func (p *PublicMarketClient) ResolveTypeIDs(ctx context.Context, names []string) (map[string]int32, error) {
	cleaned := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		trimmed := strings.TrimSpace(n)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; !exists {
			seen[trimmed] = struct{}{}
			cleaned = append(cleaned, trimmed)
		}
	}

	result := make(map[string]int32)
	if len(cleaned) == 0 {
		return result, nil
	}

	body, err := p.client.DoPost(ctx, "/universe/ids/?datasource=tranquility", cleaned)
	if err != nil {
		return nil, fmt.Errorf("resolving type IDs: %w", err)
	}

	var parsed esiUniverseIDsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding universe/ids response: %w", err)
	}

	for _, t := range parsed.InventoryTypes {
		result[t.Name] = t.ID
	}

	return result, nil
}

type esiMarketOrder struct {
	OrderID      int64   `json:"order_id"`
	TypeID       int32   `json:"type_id"`
	LocationID   int64   `json:"location_id"`
	SystemID     int32   `json:"system_id"`
	Price        float64 `json:"price"`
	VolumeRemain int     `json:"volume_remain"`
	IsBuyOrder   bool    `json:"is_buy_order"`
}

// GetJitaPrices fetches live lowest sell and highest buy prices in Jita 4-4 (The Forge) for each type_id.
func (p *PublicMarketClient) GetJitaPrices(ctx context.Context, typeIDs []int32) (map[int32]domain.JitaPrice, error) {
	prices := make(map[int32]domain.JitaPrice, len(typeIDs))
	now := time.Now()

	for _, typeID := range typeIDs {
		if typeID <= 0 {
			continue
		}

		path := fmt.Sprintf("/markets/%d/orders/?datasource=tranquility&order_type=all&type_id=%d", TheForgeRegionID, typeID)
		body, err := p.client.DoGet(ctx, path, "", true)
		if err != nil {
			return nil, fmt.Errorf("fetching Jita orders for type %d: %w", typeID, err)
		}

		var orders []esiMarketOrder
		if err := json.Unmarshal(body, &orders); err != nil {
			return nil, fmt.Errorf("decoding market orders for type %d: %w", typeID, err)
		}

		jp := aggregateJitaOrders(typeID, orders, now)
		prices[typeID] = jp
	}

	return prices, nil
}

func aggregateJitaOrders(typeID int32, orders []esiMarketOrder, now time.Time) domain.JitaPrice {
	// Prefer Jita 4-4 / Jita solar system orders; fallback to all The Forge orders if none in Jita
	jitaOrders := make([]esiMarketOrder, 0, len(orders))
	for _, o := range orders {
		if o.LocationID == Jita44StationID || o.SystemID == JitaSolarSystemID {
			jitaOrders = append(jitaOrders, o)
		}
	}
	target := jitaOrders
	if len(target) == 0 {
		target = orders
	}

	lowestSell := math.MaxFloat64
	highestBuy := 0.0
	sellCount := 0
	buyCount := 0

	for _, o := range target {
		if o.IsBuyOrder {
			buyCount++
			if o.Price > highestBuy {
				highestBuy = o.Price
			}
		} else {
			sellCount++
			if o.Price < lowestSell {
				lowestSell = o.Price
			}
		}
	}

	if sellCount == 0 {
		lowestSell = 0
	}

	return domain.JitaPrice{
		TypeID:          typeID,
		LowestSell:      domain.RoundISK(lowestSell),
		HighestBuy:      domain.RoundISK(highestBuy),
		SellOrdersCount: sellCount,
		BuyOrdersCount:  buyCount,
		UpdatedTime:     now,
	}
}

type esiMarketHistoryEntry struct {
	Date       string  `json:"date"`
	Average    float64 `json:"average"`
	Highest    float64 `json:"highest"`
	Lowest     float64 `json:"lowest"`
	OrderCount int64   `json:"order_count"`
	Volume     int64   `json:"volume"`
}

// GetDailyVolume computes average daily unit volume over the last 7 and 30 days in The Forge (Jita).
func (p *PublicMarketClient) GetDailyVolume(ctx context.Context, typeID int32) (float64, float64, error) {
	if typeID <= 0 {
		return 0, 0, fmt.Errorf("invalid type_id: %d", typeID)
	}

	path := fmt.Sprintf("/markets/%d/history/?datasource=tranquility&type_id=%d", TheForgeRegionID, typeID)
	body, err := p.client.DoGet(ctx, path, "", true)
	if err != nil {
		return 0, 0, fmt.Errorf("fetching market history for type %d: %w", typeID, err)
	}

	var history []esiMarketHistoryEntry
	if err := json.Unmarshal(body, &history); err != nil {
		return 0, 0, fmt.Errorf("decoding market history for type %d: %w", typeID, err)
	}

	if len(history) == 0 {
		return 0, 0, nil
	}

	avg7 := averageLastNDays(history, 7)
	avg30 := averageLastNDays(history, 30)
	return domain.RoundISK(avg7), domain.RoundISK(avg30), nil
}

func averageLastNDays(history []esiMarketHistoryEntry, days int) float64 {
	if len(history) == 0 || days <= 0 {
		return 0
	}
	start := len(history) - days
	if start < 0 {
		start = 0
	}
	slice := history[start:]
	var total int64
	for _, entry := range slice {
		total += entry.Volume
	}
	return float64(total) / float64(len(slice))
}

type esiIndustrySystem struct {
	SolarSystemID int32 `json:"solar_system_id"`
	CostIndices   []struct {
		Activity  string  `json:"activity"`
		CostIndex float64 `json:"cost_index"`
	} `json:"cost_indices"`
}

// GetSystemCostIndex fetches manufacturing, invention, and other activity cost indices for a solar system.
func (p *PublicMarketClient) GetSystemCostIndex(ctx context.Context, solarSystemID int32) (map[string]float64, error) {
	if solarSystemID <= 0 {
		solarSystemID = JitaSolarSystemID
	}

	body, err := p.client.DoGet(ctx, "/industry/systems/?datasource=tranquility", "", true)
	if err != nil {
		return nil, fmt.Errorf("fetching industry systems: %w", err)
	}

	var systems []esiIndustrySystem
	if err := json.Unmarshal(body, &systems); err != nil {
		return nil, fmt.Errorf("decoding industry systems: %w", err)
	}

	indices := make(map[string]float64)
	for _, sys := range systems {
		if sys.SolarSystemID == solarSystemID {
			for _, ci := range sys.CostIndices {
				indices[ci.Activity] = ci.CostIndex
			}
			return indices, nil
		}
	}

	return indices, fmt.Errorf("solar system %d not found in ESI industry systems", solarSystemID)
}
