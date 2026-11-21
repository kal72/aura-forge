package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
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

// SearchMarketplaceTypes performs instant local autocomplete against PostgreSQL sde_types (0 ESI calls).
func (s *CopilotService) SearchMarketplaceTypes(ctx context.Context, query string, limit int) []domain.SDEType {
	q := strings.TrimSpace(query)
	if len(q) < 2 {
		return []domain.SDEType{}
	}
	if limit <= 0 || limit > 25 {
		limit = 12
	}
	if s.sde != nil {
		if ts, ok := s.sde.(domain.SDETypeSearchRepository); ok {
			if types, err := ts.SearchTypes(ctx, q, limit); err == nil && len(types) > 0 {
				return types
			}
		}
	}

	// Fallback matching against default presets when SDE types search isn't populated
	ql := strings.ToLower(q)
	out := make([]domain.SDEType, 0, limit)
	for _, p := range DefaultMarketplacePresets {
		if strings.Contains(strings.ToLower(p.Name), ql) {
			out = append(out, domain.SDEType{Name: p.Name, Category: p.Category})
		}
	}
	return out
}

// BrowseMarketplaceTypes returns paginated SDE market items filtered by category, sub-group, and search query (0 ESI calls).
func (s *CopilotService) BrowseMarketplaceTypes(ctx context.Context, query, category, groupName string, page, pageSize int) (*domain.MarketTypePageResult, error) {
	if s.sde != nil {
		if ts, ok := s.sde.(domain.SDETypeSearchRepository); ok {
			return ts.BrowseTypesPaged(ctx, query, category, groupName, page, pageSize)
		}
	}
	// Fallback for mock/unit-test environments without PostgreSQL SDERepo
	items := make([]domain.SDEType, 0, len(DefaultMarketplacePresets))
	for _, p := range DefaultMarketplacePresets {
		if category != "" && !strings.EqualFold(p.Category, category) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) {
			continue
		}
		items = append(items, domain.SDEType{Name: p.Name, Category: p.Category})
	}
	return &domain.MarketTypePageResult{
		Items:      items,
		Total:      len(items),
		Page:       1,
		PageSize:   24,
		TotalPages: 1,
		Categories: []domain.CategoryCount{
			{Category: "Minerals", Count: 4},
			{Category: "Ships", Count: 3},
			{Category: "Drones", Count: 2},
			{Category: "Modules", Count: 2},
			{Category: "Commodities", Count: 1},
		},
		Groups: []string{},
	}, nil
}

// GetMarketplaceSnapshot queries Public ESI in parallel for live order book, 30-day history,
// 5-hub comparison, trading & pricing intelligence, best arbitrage route, and optional SDE craft cost.
func (s *CopilotService) GetMarketplaceSnapshot(ctx context.Context, query string, hubKey string, compareHubs bool) (*domain.MarketplaceSnapshot, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("please enter an EVE Online item name (e.g. PLEX, Tritanium, Hobgoblin II)")
	}
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market service is not available")
	}

	settings, _ := s.GetSettings(ctx)
	selectedHub := settings.ActiveTradeHub()
	if strings.TrimSpace(hubKey) != "" {
		selectedHub = domain.LookupTradeHub(hubKey)
	}

	var (
		typeID   int32
		typeName = query
	)

	// Allow numeric TypeID, fast local SDE exact match (0 ESI calls), or Public ESI ResolveTypeIDs
	if parsedID, err := strconv.ParseInt(query, 10, 32); err == nil && parsedID > 0 {
		typeID = int32(parsedID)
		typeName = fmt.Sprintf("Type #%d", typeID)
	} else {
		// 1. Try fast local SDE exact match first
		if s.sde != nil {
			if ts, ok := s.sde.(domain.SDETypeSearchRepository); ok {
				if matches, mErr := ts.SearchTypes(ctx, query, 5); mErr == nil {
					for _, m := range matches {
						if strings.EqualFold(m.Name, query) {
							typeID = m.TypeID
							typeName = m.Name
							break
						}
					}
				}
			}
		}
		// 2. Fall back to Public ESI /universe/ids/
		if typeID == 0 {
			resolved, err := s.market.ResolveTypeIDs(ctx, []string{query})
			if err != nil {
				return nil, fmt.Errorf("failed looking up item in Public ESI: %w", err)
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
	}

	if typeID <= 0 {
		return nil, fmt.Errorf("item '%s' was not found in EVE Online (try picking from the live autocomplete suggestions)", query)
	}

	snap := &domain.MarketplaceSnapshot{
		Query:           query,
		TypeID:          typeID,
		TypeName:        typeName,
		SelectedHub:     selectedHub,
		RoundTripFeePct: domain.RoundPercent(settings.BrokerFeePercent*2 + settings.SalesTaxPercent),
		PriceSignal:     "FAIR",
		UpdatedAt:       time.Now(),
	}

	browser, hasBrowser := s.market.(domain.MarketBrowserProvider)
	if hasBrowser {
		// Parallel fetch: selected hub order book + 30d history + other 4 hubs simultaneously (cache-first)
		var (
			sells      []domain.MarketOrderBookEntry
			buys       []domain.MarketOrderBookEntry
			summary    domain.HubPriceSummary
			obErr      error
			hist       []domain.MarketHistoryPoint
			hubResults = make([]domain.HubPriceSummary, len(domain.MajorTradeHubs))
			wg         sync.WaitGroup
		)

		wg.Add(2)
		go func() {
			defer wg.Done()
			sells, buys, summary, obErr = browser.GetRegionOrderBook(ctx, selectedHub, typeID, 20)
		}()
		go func() {
			defer wg.Done()
			if h, hErr := browser.GetMarketHistory(ctx, selectedHub.RegionID, typeID, 30); hErr == nil {
				hist = h
			}
		}()

		if compareHubs {
			for idx, hub := range domain.MajorTradeHubs {
				if hub.Key == selectedHub.Key {
					continue
				}
				wg.Add(1)
				go func(i int, h domain.TradeHub) {
					defer wg.Done()
					_, _, hubSum, hErr := browser.GetRegionOrderBook(ctx, h, typeID, 1)
					if hErr != nil {
						hubSum = domain.HubPriceSummary{Hub: h}
					}
					hubResults[i] = hubSum
				}(idx, hub)
			}
		}
		wg.Wait()

		if obErr != nil {
			return nil, obErr
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

		if len(hist) > 0 {
			snap.History = hist
			snap.AvgDailyVolume7d = avgHistoryVolume(hist, 7)
			snap.AvgDailyVolume30d = avgHistoryVolume(hist, 30)
			populateMarketplace7dSignal(snap, hist)
		}

		if compareHubs {
			for idx, hub := range domain.MajorTradeHubs {
				if hub.Key == selectedHub.Key {
					hubResults[idx] = summary
				}
			}
			snap.HubComparisons = hubResults
			populateBestMarketplaceArbitrage(snap, *settings)
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

	// Compute Net Station Flip Profit, Margin %, and Recommended 4-sig-fig Buy/Sell Order Prices
	if snap.LowestSell > 0 && snap.HighestBuy > 0 {
		st := domain.EvaluateStationTradeForHub(
			typeID, typeName, "", selectedHub,
			domain.JitaPrice{
				TypeID:          typeID,
				LowestSell:      snap.LowestSell,
				HighestBuy:      snap.HighestBuy,
				SellOrdersCount: snap.SellOrdersCount,
				BuyOrdersCount:  snap.BuyOrdersCount,
			},
			snap.AvgDailyVolume7d,
			settings.BrokerFeePercent, settings.SalesTaxPercent,
		)
		snap.NetFlipProfitUnit = st.NetFlipProfitUnit
		snap.NetFlipMarginPct = st.NetFlipMarginPct
		snap.RecommendedBuyOrder = st.RecommendedBuyOrder
		snap.RecommendedSellOrder = st.RecommendedSellOrder
	}

	// Optional SDE Craft Cost vs Market Buy comparison
	if s.sde != nil {
		if bp, err := s.sde.GetBlueprintByProductID(ctx, typeID); err == nil && bp != nil {
			snap.HasBlueprint = true
			snap.BlueprintTypeID = bp.BlueprintTypeID
			qtyPerRun := bp.ProductQtyPerRun
			if qtyPerRun <= 0 {
				qtyPerRun = 1
			}
			if runCost, ok := s.EstimateSDEMaterialCostForHub(ctx, selectedHub, typeID, qtyPerRun, nil); ok && runCost > 0 {
				unitCraft := domain.RoundISK(runCost / float64(qtyPerRun))
				snap.EstCraftUnitCost = unitCraft
				if snap.LowestSell > 0 {
					snap.CraftSavingsPct = domain.RoundPercent(((snap.LowestSell - unitCraft) / snap.LowestSell) * 100.0)
				}
			}
		}
	}

	return snap, nil
}

func populateMarketplace7dSignal(snap *domain.MarketplaceSnapshot, hist []domain.MarketHistoryPoint) {
	if len(hist) == 0 {
		return
	}
	recent := hist
	if len(recent) > 7 {
		recent = recent[len(recent)-7:]
	}
	sum := 0.0
	cnt := 0
	minLow := math.MaxFloat64
	maxHigh := 0.0
	for _, pt := range recent {
		if pt.Average > 0 {
			sum += pt.Average
			cnt++
		}
		if pt.Lowest > 0 && pt.Lowest < minLow {
			minLow = pt.Lowest
		}
		if pt.Highest > maxHigh {
			maxHigh = pt.Highest
		}
	}
	if cnt > 0 {
		snap.Avg7dPrice = domain.RoundISK(sum / float64(cnt))
	}
	if minLow < math.MaxFloat64 {
		snap.Low7dPrice = domain.RoundISK(minLow)
	}
	if maxHigh > 0 {
		snap.High7dPrice = domain.RoundISK(maxHigh)
	}
	if snap.Avg7dPrice > 0 && snap.LowestSell > 0 {
		chg := ((snap.LowestSell - snap.Avg7dPrice) / snap.Avg7dPrice) * 100.0
		snap.Change7dPct = domain.RoundPercent(chg)
		switch {
		case chg >= domain.MaxSafePriceSpikePercent:
			snap.PriceSignal = "SPIKE"
		case chg >= 3.0:
			snap.PriceSignal = "HIGH"
		case chg <= -3.0:
			snap.PriceSignal = "CHEAP"
		default:
			snap.PriceSignal = "FAIR"
		}
	}
}

func populateBestMarketplaceArbitrage(snap *domain.MarketplaceSnapshot, settings domain.UserSettings) {
	var (
		cheapest domain.HubPriceSummary
		highest  domain.HubPriceSummary
	)
	for _, h := range snap.HubComparisons {
		if h.LowestSell <= 0 {
			continue
		}
		if cheapest.LowestSell == 0 || h.LowestSell < cheapest.LowestSell {
			cheapest = h
		}
		if h.LowestSell > highest.LowestSell {
			highest = h
		}
	}
	if cheapest.LowestSell <= 0 || highest.LowestSell <= 0 || cheapest.Hub.Key == highest.Hub.Key {
		return
	}
	feeFactor := 1.0 - (settings.BrokerFeePercent+settings.SalesTaxPercent)/100.0
	netRev := highest.LowestSell * feeFactor
	netProfit := netRev - cheapest.LowestSell
	netMargin := (netProfit / highest.LowestSell) * 100.0

	snap.BestArbOriginHub = cheapest.Hub
	snap.BestArbDestHub = highest.Hub
	snap.BestArbOriginSell = cheapest.LowestSell
	snap.BestArbDestSell = highest.LowestSell
	snap.BestArbNetMarginPct = domain.RoundPercent(netMargin)
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
		return nil, fmt.Errorf("item name is required")
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
