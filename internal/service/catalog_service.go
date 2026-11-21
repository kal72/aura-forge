package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"aura-forge/internal/domain"
)

// CatalogSummary aggregates top-level counts for the Catalog & Trap Guard KPI banner.
type CatalogSummary struct {
	TotalCount     int `json:"total_count"`
	ViableCount    int `json:"viable_count"`
	WatchlistCount int `json:"watchlist_count"`
	TrapCount      int `json:"trap_count"`
	SDESyncedCount int `json:"sde_synced_count"`
	SpikeCount     int `json:"spike_count"`
}

// SummarizeCatalog computes KPI totals across a catalog slice.
func SummarizeCatalog(items []domain.MarketItem) CatalogSummary {
	var sum CatalogSummary
	sum.TotalCount = len(items)
	for _, it := range items {
		switch it.TrapAnalysis.Status {
		case domain.TrapStatusLayak:
			sum.ViableCount++
		case domain.TrapStatusJebakan:
			sum.TrapCount++
		default:
			sum.WatchlistCount++
		}
		if it.TrapAnalysis.BOMSyncedFromSDE {
			sum.SDESyncedCount++
		}
		if it.TrapAnalysis.IsPriceSpike {
			sum.SpikeCount++
		}
	}
	return sum
}

// ListCatalogSorted returns filtered catalog items plus a full-catalog KPI summary,
// ensuring live profit/ROI metrics are populated and sorting by the requested key.
func (s *CopilotService) ListCatalogSorted(ctx context.Context, categoryFilter, statusFilter, search, sortBy string) ([]domain.MarketItem, CatalogSummary, error) {
	if s.items == nil {
		return []domain.MarketItem{}, CatalogSummary{}, nil
	}
	all, err := s.items.ListItems(ctx)
	if err != nil {
		return nil, CatalogSummary{}, err
	}

	settings, _ := s.GetSettings(ctx)
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)

	// Hydrate profit/margin/ROI on legacy rows that were saved prior to v2 JSONB fields.
	for i := range all {
		if all[i].TrapAnalysis.EstimatedNetProfit == 0 && all[i].TrapAnalysis.EstimatedROIPct == 0 &&
			(all[i].EstimatedJitaSell > 0 || all[i].MaterialCost > 0) {
			all[i].TrapAnalysis = domain.EvaluateTrapGuard(all[i], *settings, availCap)
		}
	}

	summary := SummarizeCatalog(all)

	q := strings.ToLower(strings.TrimSpace(search))
	filtered := make([]domain.MarketItem, 0, len(all))
	for _, it := range all {
		if categoryFilter != "" && string(it.Category) != categoryFilter {
			continue
		}
		if statusFilter != "" && string(it.TrapAnalysis.Status) != statusFilter {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(it.Name), q) {
			continue
		}
		filtered = append(filtered, it)
	}

	SortCatalogItems(filtered, sortBy)
	return filtered, summary, nil
}

// SortCatalogItems orders catalog rows in-place by roi, profit, volume, risk, or name.
func SortCatalogItems(items []domain.MarketItem, sortBy string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "roi":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].TrapAnalysis.EstimatedROIPct != items[j].TrapAnalysis.EstimatedROIPct {
				return items[i].TrapAnalysis.EstimatedROIPct > items[j].TrapAnalysis.EstimatedROIPct
			}
			return items[i].Name < items[j].Name
		})
	case "profit":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].TrapAnalysis.EstimatedNetProfit != items[j].TrapAnalysis.EstimatedNetProfit {
				return items[i].TrapAnalysis.EstimatedNetProfit > items[j].TrapAnalysis.EstimatedNetProfit
			}
			return items[i].Name < items[j].Name
		})
	case "volume":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].DailyVolume != items[j].DailyVolume {
				return items[i].DailyVolume > items[j].DailyVolume
			}
			return items[i].Name < items[j].Name
		})
	case "risk":
		rank := func(st domain.TrapStatus) int {
			switch st {
			case domain.TrapStatusLayak:
				return 1
			case domain.TrapStatusWaspadaNiche:
				return 2
			case domain.TrapStatusWaspadaModal:
				return 3
			case domain.TrapStatusJebakan:
				return 4
			default:
				return 5
			}
		}
		sort.SliceStable(items, func(i, j int) bool {
			ri, rj := rank(items[i].TrapAnalysis.Status), rank(items[j].TrapAnalysis.Status)
			if ri != rj {
				return ri < rj
			}
			return items[i].TrapAnalysis.EstimatedROIPct > items[j].TrapAnalysis.EstimatedROIPct
		})
	default:
		sort.SliceStable(items, func(i, j int) bool {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		})
	}
}

// ActiveCatalogHub returns the user's configured Primary Trade Hub (defaults to Jita 4-4).
func (s *CopilotService) ActiveCatalogHub(ctx context.Context) domain.TradeHub {
	settings, _ := s.GetSettings(ctx)
	if settings == nil {
		return domain.LookupTradeHub("jita")
	}
	return settings.ActiveTradeHub()
}

// EstimateSDEMaterialCost computes the live BOM material cost for a catalog product at the user's active hub.
func (s *CopilotService) EstimateSDEMaterialCost(
	ctx context.Context,
	productTypeID int32,
	batchOutputQty int,
	preloadedPrices map[int32]domain.JitaPrice,
) (float64, bool) {
	return s.EstimateSDEMaterialCostForHub(ctx, s.ActiveCatalogHub(ctx), productTypeID, batchOutputQty, preloadedPrices)
}

// EstimateSDEMaterialCostForHub computes the live BOM material cost for a catalog product at any of the 5 trade hubs
// using Official CCP SDE recipes (ME 10 for T1, ME 2 for T2). Returns (cost, foundInSDE).
func (s *CopilotService) EstimateSDEMaterialCostForHub(
	ctx context.Context,
	hub domain.TradeHub,
	productTypeID int32,
	batchOutputQty int,
	preloadedPrices map[int32]domain.JitaPrice,
) (float64, bool) {
	if s.sde == nil || productTypeID <= 0 {
		return 0, false
	}
	bp, err := s.sde.GetBlueprintByProductID(ctx, productTypeID)
	if err != nil || bp == nil {
		return 0, false
	}
	mats, err := s.sde.GetBlueprintMaterials(ctx, bp.BlueprintTypeID)
	if err != nil || len(mats) == 0 {
		return 0, false
	}

	qtyPerRun := bp.ProductQtyPerRun
	if qtyPerRun <= 0 {
		qtyPerRun = 1
	}
	if batchOutputQty <= 0 {
		batchOutputQty = qtyPerRun
	}
	runs := int(math.Ceil(float64(batchOutputQty) / float64(qtyPerRun)))
	if runs < 1 {
		runs = 1
	}

	defaultME := 10
	if bp.TechLevel >= 2 {
		defaultME = 2
	}

	// Collect any missing material prices in a single batch call at the target hub
	prices := preloadedPrices
	if prices == nil {
		prices = make(map[int32]domain.JitaPrice, len(mats))
	}
	if s.market != nil {
		missing := make([]int32, 0)
		for _, m := range mats {
			if _, ok := prices[m.MaterialTypeID]; !ok {
				missing = append(missing, m.MaterialTypeID)
			}
		}
		if len(missing) > 0 {
			if fetched, fErr := s.fetchHubPrices(ctx, hub, missing); fErr == nil {
				for id, jp := range fetched {
					prices[id] = jp
				}
			}
		}
	}

	totalCost := 0.0
	for _, m := range mats {
		adjQty := domain.ComputeAdjustedMaterialQty(m.BaseQuantity, runs, defaultME, 0)
		if jp, ok := prices[m.MaterialTypeID]; ok {
			unitPrice := jp.LowestSell
			if unitPrice <= 0 {
				unitPrice = jp.HighestBuy
			}
			totalCost += float64(adjQty) * unitPrice
		}
	}
	if totalCost <= 0 {
		return 0, false
	}
	return math.Round(totalCost*100) / 100, true
}

// PriceIntelligence summarizes live Jita prices, 7-day trading range, break-even floor,
// and a beginner-safe Recommended Sell Price for a single unit.
type PriceIntelligence struct {
	LowestSell           float64 `json:"lowest_sell"`
	HighestBuy           float64 `json:"highest_buy"`
	Low7d                float64 `json:"low_7d"`
	Avg7d                float64 `json:"avg_7d"`
	High7d               float64 `json:"high_7d"`
	BreakEvenPrice       float64 `json:"break_even_price"`
	RecommendedSellPrice float64 `json:"recommended_sell_price"`
	RecommendationNote   string  `json:"recommendation_note"`
	PriceRegime          string  `json:"price_regime"` // "COMPETITIVE" | "SPIKE" | "BELOW_COST" | "ESTIMATED"
}

// RoundEVE4SigFigs rounds an ISK price to EVE Online's 4 significant figures market tick rule.
func RoundEVE4SigFigs(val float64) float64 {
	if val <= 0 || math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	if val < 100 {
		return math.Round(val*100) / 100
	}
	digits := math.Floor(math.Log10(val)) + 1
	scale := math.Pow(10, digits-4)
	if scale <= 0 {
		return math.Round(val*100) / 100
	}
	return math.Round(val/scale) * scale
}

// ComputePriceIntelligence computes 7d range insights, break-even unit price, and a safe
// Recommended Sell Price that avoids price-spike traps and below-cost undercuts.
func ComputePriceIntelligence(
	lowestSell, highestBuy, low7d, avg7d, high7d, materialCost float64,
	batchQty int,
	settings domain.UserSettings,
) PriceIntelligence {
	pi := PriceIntelligence{
		LowestSell: lowestSell,
		HighestBuy: highestBuy,
		Low7d:      low7d,
		Avg7d:      avg7d,
		High7d:     high7d,
	}

	if batchQty <= 0 {
		batchQty = 1
	}

	// Compute unit break-even price when material cost is known
	if materialCost > 0 {
		calcRes, err := domain.CalculateProfit(domain.ProfitCalcInput{
			OutputQty:          batchQty,
			UnitSellPrice:      math.Max(lowestSell, avg7d),
			MaterialCost:       materialCost,
			SystemCostIndex:    settings.SystemCostIndex,
			FacilityTaxPercent: settings.FacilityTaxPercent,
			BrokerFeePercent:   settings.BrokerFeePercent,
			SalesTaxPercent:    settings.SalesTaxPercent,
			RelistFeePercent:   settings.RelistFeePercent,
			EstimatedRelists:   1,
			BatchesPerWeek:     1,
		})
		if err == nil && calcRes.BreakEvenUnitPrice > 0 {
			pi.BreakEvenPrice = math.Round(calcRes.BreakEvenUnitPrice*100) / 100
		}
	}

	// Determine beginner-safe Recommended Sell Price
	if lowestSell <= 0 && avg7d <= 0 {
		if pi.BreakEvenPrice > 0 {
			pi.RecommendedSellPrice = RoundEVE4SigFigs(pi.BreakEvenPrice * 1.10)
			pi.PriceRegime = "ESTIMATED"
			pi.RecommendationNote = "No live Jita sell orders found — estimated at Break-Even +10% margin."
		}
		return pi
	}

	refLive := lowestSell
	if refLive <= 0 {
		refLive = avg7d
	}

	// Case 1: Fake Price Spike (>= +25% above 7d average)
	if avg7d > 0 && lowestSell >= avg7d*(1.0+domain.MaxSafePriceSpikePercent/100.0) {
		spikePct := ((lowestSell - avg7d) / avg7d) * 100.0
		target := avg7d * 1.03
		if pi.BreakEvenPrice > 0 && target < pi.BreakEvenPrice*1.08 {
			target = pi.BreakEvenPrice * 1.08
		}
		if target > lowestSell*0.999 {
			target = lowestSell * 0.999
		}
		pi.RecommendedSellPrice = RoundEVE4SigFigs(target)
		pi.PriceRegime = "SPIKE"
		pi.RecommendationNote = "Live Jita Sell is spiked (+" + formatOneDecimal(spikePct) + "% vs 7d avg). Price realistically near 7d average so your order fills before prices drop back."
		return pi
	}

	// Case 2: Current Jita Sell is below or dangerously close to Break-Even
	if pi.BreakEvenPrice > 0 && refLive < pi.BreakEvenPrice*1.03 {
		floorPrice := RoundEVE4SigFigs(pi.BreakEvenPrice * 1.08)
		pi.RecommendedSellPrice = floorPrice
		pi.PriceRegime = "BELOW_COST"
		pi.RecommendationNote = "Current Jita Sell is below safe profit margin vs break-even. List at this safe floor (+8% margin) or wait for Jita prices to recover."
		return pi
	}

	// Case 3: Healthy competitive Jita market — 1-tick undercut (0.1% below Lowest Sell, rounded to 4 sig figs)
	competitive := RoundEVE4SigFigs(refLive * 0.999)
	if competitive >= refLive && refLive > 1 {
		competitive = RoundEVE4SigFigs(refLive * 0.998)
	}
	if pi.BreakEvenPrice > 0 && competitive < pi.BreakEvenPrice*1.03 {
		competitive = RoundEVE4SigFigs(pi.BreakEvenPrice * 1.05)
	}
	pi.RecommendedSellPrice = competitive
	pi.PriceRegime = "COMPETITIVE"
	pi.RecommendationNote = "Competitive 1-tick undercut below Jita Lowest Sell (above break-even) for fast order fill without triggering a price war."
	return pi
}

func formatOneDecimal(v float64) string {
	return fmt.Sprintf("%.1f", v)
}

// GetItemPriceIntelligence gathers 7-day low/avg/high history at the active trade hub and computes a Recommended Sell Price.
func (s *CopilotService) GetItemPriceIntelligence(ctx context.Context, typeID int32, lowestSell, highestBuy, materialCost float64, batchQty int) PriceIntelligence {
	return s.GetItemPriceIntelligenceForHub(ctx, s.ActiveCatalogHub(ctx), typeID, lowestSell, highestBuy, materialCost, batchQty)
}

// GetItemPriceIntelligenceForHub gathers 7-day low/avg/high history at the specified trade hub and computes a Recommended Sell Price.
func (s *CopilotService) GetItemPriceIntelligenceForHub(ctx context.Context, hub domain.TradeHub, typeID int32, lowestSell, highestBuy, materialCost float64, batchQty int) PriceIntelligence {
	low7d, avg7d, high7d := s.fetchHub7dRange(ctx, hub, typeID)
	settings, _ := s.GetSettings(ctx)
	return ComputePriceIntelligence(lowestSell, highestBuy, low7d, avg7d, high7d, materialCost, batchQty, *settings)
}

// LookupItemPriceForHub resolves an item name and returns live sell/buy prices and 7d daily volume at the specified trade hub.
func (s *CopilotService) LookupItemPriceForHub(ctx context.Context, hub domain.TradeHub, itemName string) (*domain.JitaPrice, float64, error) {
	if s.market == nil {
		return nil, 0, fmt.Errorf("market provider unavailable")
	}
	resolved, err := s.market.ResolveTypeIDs(ctx, []string{itemName})
	if err != nil {
		return nil, 0, err
	}
	trimmed := strings.TrimSpace(itemName)
	tid, ok := resolved[trimmed]
	if !ok || tid == 0 {
		for k, v := range resolved {
			if strings.EqualFold(k, trimmed) {
				tid = v
				ok = true
				break
			}
		}
	}
	if !ok || tid == 0 {
		return nil, 0, fmt.Errorf("item %q not found in EVE ESI", itemName)
	}

	prices, err := s.fetchHubPrices(ctx, hub, []int32{tid})
	if err != nil {
		return nil, 0, err
	}
	jp := prices[tid]
	jp.TypeID = tid
	avg7dVol, _ := s.fetchHubDailyVolume(ctx, hub, tid)
	return &jp, avg7dVol, nil
}

// fetchHubDailyVolume returns (avg7dVolume, avg30dVolume) for a typeID in the target trade hub's region.
func (s *CopilotService) fetchHubDailyVolume(ctx context.Context, hub domain.TradeHub, typeID int32) (avg7d, avg30d float64) {
	if s.market == nil || typeID <= 0 {
		return 0, 0
	}
	if hub.Key == "jita" {
		if a7, a30, err := s.market.GetDailyVolume(ctx, typeID); err == nil {
			return a7, a30
		}
		return 0, 0
	}
	browser, ok := s.market.(domain.MarketBrowserProvider)
	if !ok {
		if a7, a30, err := s.market.GetDailyVolume(ctx, typeID); err == nil {
			return a7, a30
		}
		return 0, 0
	}
	pts, err := browser.GetMarketHistory(ctx, hub.RegionID, typeID, 30)
	if err != nil || len(pts) == 0 {
		return 0, 0
	}
	sum30 := 0.0
	for _, p := range pts {
		sum30 += float64(p.Volume)
	}
	avg30d = math.Round((sum30/float64(len(pts)))*10) / 10

	recent := pts
	if len(recent) > 7 {
		recent = recent[len(recent)-7:]
	}
	sum7 := 0.0
	for _, p := range recent {
		sum7 += float64(p.Volume)
	}
	avg7d = math.Round((sum7/float64(len(recent)))*10) / 10
	return avg7d, avg30d
}

// fetchHub7dRange returns (low7d, avg7d, high7d) from cached Public ESI Market History in the target hub's region.
func (s *CopilotService) fetchHub7dRange(ctx context.Context, hub domain.TradeHub, typeID int32) (low7d, avg7d, high7d float64) {
	if s.market == nil || typeID <= 0 {
		return 0, 0, 0
	}
	browser, ok := s.market.(domain.MarketBrowserProvider)
	if !ok {
		return 0, 0, 0
	}
	pts, err := browser.GetMarketHistory(ctx, hub.RegionID, typeID, 7)
	if err != nil || len(pts) == 0 {
		return 0, 0, 0
	}
	recent := pts
	if len(recent) > 7 {
		recent = recent[len(recent)-7:]
	}
	sum := 0.0
	count := 0
	minLow := math.MaxFloat64
	maxHigh := 0.0
	for _, p := range recent {
		if p.Average > 0 {
			sum += p.Average
			count++
		}
		if p.Lowest > 0 && p.Lowest < minLow {
			minLow = p.Lowest
		}
		if p.Highest > maxHigh {
			maxHigh = p.Highest
		}
	}
	if count > 0 {
		avg7d = math.Round((sum/float64(count))*100) / 100
	}
	if minLow < math.MaxFloat64 {
		low7d = math.Round(minLow*100) / 100
	}
	if maxHigh > 0 {
		high7d = math.Round(maxHigh*100) / 100
	}
	return low7d, avg7d, high7d
}

// fetchCatalogAvg7dPrice returns the 7-day average price in the active hub from cached Public ESI Market History.
func (s *CopilotService) fetchCatalogAvg7dPrice(ctx context.Context, typeID int32) float64 {
	_, avg7d, _ := s.fetchHub7dRange(ctx, s.ActiveCatalogHub(ctx), typeID)
	return avg7d
}
