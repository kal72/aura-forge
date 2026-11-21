package service

import (
	"context"
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

// EstimateSDEMaterialCost computes the live Jita Sell BOM material cost for a catalog product
// using Official CCP SDE recipes (ME 10 for T1, ME 2 for T2). Returns (cost, foundInSDE).
func (s *CopilotService) EstimateSDEMaterialCost(
	ctx context.Context,
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

	// Collect any missing material prices in a single batch call
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
			if fetched, fErr := s.market.GetJitaPrices(ctx, missing); fErr == nil {
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

// fetchCatalogAvg7dPrice returns the 7-day average Jita price from cached Public ESI Market History.
func (s *CopilotService) fetchCatalogAvg7dPrice(ctx context.Context, typeID int32) float64 {
	if s.market == nil || typeID <= 0 {
		return 0
	}
	browser, ok := s.market.(domain.MarketBrowserProvider)
	if !ok {
		return 0
	}
	pts, err := browser.GetMarketHistory(ctx, 10000002, typeID, 7)
	if err != nil || len(pts) == 0 {
		return 0
	}
	recent := pts
	if len(recent) > 7 {
		recent = recent[len(recent)-7:]
	}
	sum := 0.0
	count := 0
	for _, p := range recent {
		if p.Average > 0 {
			sum += p.Average
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}
