package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

// MarketplacePreset represents a popular EVE item shortcut for 1-click browsing.
type MarketplacePreset struct {
	Name     string
	Category string
}

// DefaultMarketplacePresets lists commonly traded EVE Online items for instant exploration.
var DefaultMarketplacePresets = []MarketplacePreset{
	{Name: "PLEX", Category: "Commodities"},
	{Name: "Tritanium", Category: "Minerals"},
	{Name: "Pyerite", Category: "Minerals"},
	{Name: "Mexallon", Category: "Minerals"},
	{Name: "Isogen", Category: "Minerals"},
	{Name: "Hobgoblin II", Category: "Drones"},
	{Name: "Hammerhead II", Category: "Drones"},
	{Name: "Venture", Category: "Ships"},
	{Name: "Rifter", Category: "Ships"},
	{Name: "Catalyst", Category: "Ships"},
	{Name: "Damage Control II", Category: "Modules"},
	{Name: "Large Shield Extender II", Category: "Modules"},
}

// GetMarketplaceSnapshot queries Public ESI for live order book, 30-day history, and multi-hub price comparison.
func (s *CopilotService) GetMarketplaceSnapshot(ctx context.Context, query string, hubKey string, compareHubs bool) (*domain.MarketplaceSnapshot, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("masukkan nama item EVE Online (contoh: PLEX, Tritanium, Hobgoblin II)")
	}
	if s.market == nil {
		return nil, fmt.Errorf("layanan Public ESI Market belum tersedia")
	}

	selectedHub := domain.LookupTradeHub(hubKey)

	var (
		typeID   int32
		typeName = query
	)

	// Allow numeric TypeID or exact EVE item name
	if parsedID, err := strconv.ParseInt(query, 10, 32); err == nil && parsedID > 0 {
		typeID = int32(parsedID)
		typeName = fmt.Sprintf("Type #%d", typeID)
	} else {
		resolved, err := s.market.ResolveTypeIDs(ctx, []string{query})
		if err != nil {
			return nil, fmt.Errorf("gagal mencari item di Public ESI: %w", err)
		}
		for name, id := range resolved {
			if strings.EqualFold(name, query) {
				typeID = id
				typeName = name
				break
			}
		}
		if typeID == 0 {
			for name, id := range resolved {
				typeID = id
				typeName = name
				break
			}
		}
	}

	if typeID <= 0 {
		return nil, fmt.Errorf("item '%s' tidak ditemukan di EVE Online (pastikan ejaan persis, mis. 'Hobgoblin II' atau 'Tritanium')", query)
	}

	snap := &domain.MarketplaceSnapshot{
		Query:       query,
		TypeID:      typeID,
		TypeName:    typeName,
		SelectedHub: selectedHub,
		UpdatedAt:   time.Now(),
	}

	browser, hasBrowser := s.market.(domain.MarketBrowserProvider)
	if hasBrowser {
		sells, buys, summary, err := browser.GetRegionOrderBook(ctx, selectedHub, typeID, 20)
		if err != nil {
			return nil, err
		}
		snap.SellOrders = sells
		snap.BuyOrders = buys
		snap.LowestSell = summary.LowestSell
		snap.HighestBuy = summary.HighestBuy
		snap.SpreadISK = summary.SpreadISK
		snap.SpreadPercent = summary.SpreadPercent
		snap.TotalSellVolume = summary.SellVolume
		snap.TotalBuyVolume = summary.BuyVolume
		snap.SellOrdersCount = summary.SellOrdersCount
		snap.BuyOrdersCount = summary.BuyOrdersCount

		if hist, hErr := browser.GetMarketHistory(ctx, selectedHub.RegionID, typeID, 30); hErr == nil {
			snap.History = hist
			snap.AvgDailyVolume7d = avgHistoryVolume(hist, 7)
			snap.AvgDailyVolume30d = avgHistoryVolume(hist, 30)
		}

		if compareHubs {
			comparisons := make([]domain.HubPriceSummary, 0, len(domain.MajorTradeHubs))
			for _, hub := range domain.MajorTradeHubs {
				if hub.Key == selectedHub.Key {
					comparisons = append(comparisons, summary)
					continue
				}
				// Gentle pacing between uncached hubs to stay CCP-friendly
				select {
				case <-ctx.Done():
					break
				case <-time.After(25 * time.Millisecond):
				}
				_, _, hubSum, hErr := browser.GetRegionOrderBook(ctx, hub, typeID, 1)
				if hErr != nil {
					hubSum = domain.HubPriceSummary{Hub: hub}
				}
				comparisons = append(comparisons, hubSum)
			}
			snap.HubComparisons = comparisons
		}
	} else {
		// Fallback to basic MarketDataProvider (e.g. unit tests with mock provider)
		prices, err := s.market.GetJitaPrices(ctx, []int32{typeID})
		if err != nil {
			return nil, err
		}
		if jp, ok := prices[typeID]; ok {
			snap.LowestSell = jp.LowestSell
			snap.HighestBuy = jp.HighestBuy
			snap.SellOrdersCount = jp.SellOrdersCount
			snap.BuyOrdersCount = jp.BuyOrdersCount
			if jp.LowestSell > 0 && jp.HighestBuy > 0 {
				snap.SpreadISK = domain.RoundISK(jp.LowestSell - jp.HighestBuy)
				snap.SpreadPercent = domain.RoundPercent((snap.SpreadISK / jp.LowestSell) * 100.0)
			}
		}
		if v7, v30, vErr := s.market.GetDailyVolume(ctx, typeID); vErr == nil {
			snap.AvgDailyVolume7d = v7
			snap.AvgDailyVolume30d = v30
		}
	}

	if snap.LowestSell > 0 && snap.AvgDailyVolume7d > 0 {
		snap.EstimatedDailyISK = domain.RoundISK(snap.LowestSell * snap.AvgDailyVolume7d)
	}

	return snap, nil
}

func avgHistoryVolume(hist []domain.MarketHistoryPoint, days int) float64 {
	if len(hist) == 0 || days <= 0 {
		return 0
	}
	start := len(hist) - days
	if start < 0 {
		start = 0
	}
	slice := hist[start:]
	var total int64
	for _, pt := range slice {
		total += pt.Volume
	}
	return domain.RoundISK(float64(total) / float64(len(slice)))
}

// QuickAddMarketplaceItemToCatalog adds an item inspected in Marketplace directly into the Catalog & Trap Guard.
func (s *CopilotService) QuickAddMarketplaceItemToCatalog(ctx context.Context, typeID int32, name, category string, sellPrice, buyPrice, dailyVol float64) (*domain.MarketItem, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("nama item wajib diisi")
	}
	if category == "" {
		category = "Other"
	}
	item := &domain.MarketItem{
		TypeID:            typeID,
		Name:              strings.TrimSpace(name),
		Category:          domain.ItemCategory(category),
		EstimatedJitaSell: sellPrice,
		EstimatedJitaBuy:  buyPrice,
		DailyVolume:       dailyVol,
		BatchOutputQty:    1,
		IsManufacturable:  true,
	}
	return s.SaveCatalogItem(ctx, item)
}
