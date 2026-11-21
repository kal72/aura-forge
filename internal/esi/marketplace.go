package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

type esiOrderBookRaw struct {
	OrderID      int64   `json:"order_id"`
	TypeID       int32   `json:"type_id"`
	LocationID   int64   `json:"location_id"`
	SystemID     int32   `json:"system_id"`
	Price        float64 `json:"price"`
	VolumeRemain int     `json:"volume_remain"`
	VolumeTotal  int     `json:"volume_total"`
	IsBuyOrder   bool    `json:"is_buy_order"`
	Issued       string  `json:"issued"`
}

// GetRegionOrderBook fetches live public sell & buy orders for a specific item at a regional trade hub.
// Uses PostgreSQL esi_cache, singleflight deduplication, and respects X-Pages (capped at MaxOrderPages).
func (p *PublicMarketClient) GetRegionOrderBook(ctx context.Context, hub domain.TradeHub, typeID int32, limit int) ([]domain.MarketOrderBookEntry, []domain.MarketOrderBookEntry, domain.HubPriceSummary, error) {
	summary := domain.HubPriceSummary{Hub: hub}
	if typeID <= 0 {
		return nil, nil, summary, fmt.Errorf("invalid type_id: %d", typeID)
	}
	if limit <= 0 {
		limit = 20
	}

	pathPage1 := fmt.Sprintf("/markets/%d/orders/?datasource=tranquility&order_type=all&type_id=%d&page=1", hub.RegionID, typeID)
	body, hdrs, err := p.client.DoGetWithHeaders(ctx, pathPage1, "", true)
	if err != nil {
		return nil, nil, summary, fmt.Errorf("fetching %s orders for type %d: %w", hub.Name, typeID, err)
	}

	var allOrders []esiOrderBookRaw
	if err := json.Unmarshal(body, &allOrders); err != nil {
		return nil, nil, summary, fmt.Errorf("decoding %s orders for type %d: %w", hub.Name, typeID, err)
	}

	totalPages := 1
	if hdrs != nil {
		if rawPages := strings.TrimSpace(hdrs.Get("X-Pages")); rawPages != "" {
			if n, pErr := strconv.Atoi(rawPages); pErr == nil && n > 1 {
				totalPages = n
			}
		}
	}
	if totalPages > MaxOrderPages {
		totalPages = MaxOrderPages
	}

	for page := 2; page <= totalPages; page++ {
		select {
		case <-ctx.Done():
			return nil, nil, summary, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
		pagePath := fmt.Sprintf("/markets/%d/orders/?datasource=tranquility&order_type=all&type_id=%d&page=%d", hub.RegionID, typeID, page)
		pageBody, _, pErr := p.client.DoGetWithHeaders(ctx, pagePath, "", true)
		if pErr != nil {
			break
		}
		var pageOrders []esiOrderBookRaw
		if uErr := json.Unmarshal(pageBody, &pageOrders); uErr == nil {
			allOrders = append(allOrders, pageOrders...)
		}
	}

	// Prefer hub station / solar system orders; fallback to full region if hub has no direct orders
	hubOrders := make([]esiOrderBookRaw, 0, len(allOrders))
	for _, o := range allOrders {
		if (hub.StationID > 0 && o.LocationID == hub.StationID) || (hub.SystemID > 0 && o.SystemID == hub.SystemID) {
			hubOrders = append(hubOrders, o)
		}
	}
	target := hubOrders
	if len(target) == 0 {
		target = allOrders
	}

	sells := make([]domain.MarketOrderBookEntry, 0)
	buys := make([]domain.MarketOrderBookEntry, 0)
	lowestSell := math.MaxFloat64
	highestBuy := 0.0
	var sellVol, buyVol int64

	for _, o := range target {
		entry := domain.MarketOrderBookEntry{
			OrderID:      o.OrderID,
			Price:        domain.RoundISK(o.Price),
			VolumeRemain: o.VolumeRemain,
			VolumeTotal:  o.VolumeTotal,
			LocationID:   o.LocationID,
			SystemID:     o.SystemID,
			IsBuyOrder:   o.IsBuyOrder,
			Issued:       o.Issued,
		}
		if o.IsBuyOrder {
			buys = append(buys, entry)
			buyVol += int64(o.VolumeRemain)
			if o.Price > highestBuy {
				highestBuy = o.Price
			}
		} else {
			sells = append(sells, entry)
			sellVol += int64(o.VolumeRemain)
			if o.Price < lowestSell {
				lowestSell = o.Price
			}
		}
	}

	sort.Slice(sells, func(i, j int) bool { return sells[i].Price < sells[j].Price })
	sort.Slice(buys, func(i, j int) bool { return buys[i].Price > buys[j].Price })

	summary.SellOrdersCount = len(sells)
	summary.BuyOrdersCount = len(buys)
	summary.SellVolume = sellVol
	summary.BuyVolume = buyVol
	if len(sells) > 0 {
		summary.LowestSell = domain.RoundISK(lowestSell)
	}
	if len(buys) > 0 {
		summary.HighestBuy = domain.RoundISK(highestBuy)
	}
	if summary.LowestSell > 0 && summary.HighestBuy > 0 {
		summary.SpreadISK = domain.RoundISK(summary.LowestSell - summary.HighestBuy)
		summary.SpreadPercent = domain.RoundPercent((summary.SpreadISK / summary.LowestSell) * 100.0)
	}

	if len(sells) > limit {
		sells = sells[:limit]
	}
	if len(buys) > limit {
		buys = buys[:limit]
	}

	return sells, buys, summary, nil
}

// GetMarketHistory fetches the last N days of regional price & volume candles from Public ESI.
func (p *PublicMarketClient) GetMarketHistory(ctx context.Context, regionID int32, typeID int32, days int) ([]domain.MarketHistoryPoint, error) {
	if regionID <= 0 {
		regionID = TheForgeRegionID
	}
	if typeID <= 0 {
		return nil, fmt.Errorf("invalid type_id: %d", typeID)
	}
	if days <= 0 {
		days = 30
	}

	path := fmt.Sprintf("/markets/%d/history/?datasource=tranquility&type_id=%d", regionID, typeID)
	body, err := p.client.DoGet(ctx, path, "", true)
	if err != nil {
		return nil, fmt.Errorf("fetching market history for type %d: %w", typeID, err)
	}

	var raw []esiMarketHistoryEntry
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding market history for type %d: %w", typeID, err)
	}

	start := len(raw) - days
	if start < 0 {
		start = 0
	}
	slice := raw[start:]
	out := make([]domain.MarketHistoryPoint, 0, len(slice))
	for _, r := range slice {
		out = append(out, domain.MarketHistoryPoint{
			Date:       r.Date,
			Average:    domain.RoundISK(r.Average),
			Highest:    domain.RoundISK(r.Highest),
			Lowest:     domain.RoundISK(r.Lowest),
			Volume:     r.Volume,
			OrderCount: r.OrderCount,
		})
	}
	return out, nil
}
