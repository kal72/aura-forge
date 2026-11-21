package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

// scannerBasketItem defines a high-liquidity EVE item used for Station Trading and Inter-Hub Arbitrage scans.
type scannerBasketItem struct {
	TypeID   int32
	Name     string
	Category string
	VolumeM3 float64
}

var defaultStationTradeBasket = []scannerBasketItem{
	{TypeID: 44992, Name: "PLEX", Category: "Commodities", VolumeM3: 0.01},
	{TypeID: 2456, Name: "Hobgoblin II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2488, Name: "Warrior II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2185, Name: "Hammerhead II", Category: "Drones", VolumeM3: 10.0},
	{TypeID: 2048, Name: "Damage Control II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 3841, Name: "Large Shield Extender II", Category: "Modules", VolumeM3: 25.0},
	{TypeID: 5975, Name: "5MN Y-T8 Compact Microwarpdrive", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 527, Name: "Stasis Webifier I", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 3244, Name: "Warp Scrambler II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 1952, Name: "Sensor Booster II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 2205, Name: "Ballistic Control System II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 519, Name: "Gyrostabilizer II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 10190, Name: "Magnetic Field Stabilizer II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 32880, Name: "Venture", Category: "Ships", VolumeM3: 2500.0},
	{TypeID: 587, Name: "Rifter", Category: "Ships", VolumeM3: 2500.0},
	{TypeID: 16240, Name: "Catalyst", Category: "Ships", VolumeM3: 5000.0},
	{TypeID: 16242, Name: "Thrasher", Category: "Ships", VolumeM3: 5000.0},
	{TypeID: 621, Name: "Caracal", Category: "Ships", VolumeM3: 10000.0},
	{TypeID: 2454, Name: "Hobgoblin I", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2046, Name: "Damage Control I", Category: "Modules", VolumeM3: 5.0},
}

// Compact, high-demand items suitable for beginner high-sec hauling arbitrage.
var defaultArbitrageBasket = []scannerBasketItem{
	{TypeID: 2048, Name: "Damage Control II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 3841, Name: "Large Shield Extender II", Category: "Modules", VolumeM3: 25.0},
	{TypeID: 2456, Name: "Hobgoblin II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2488, Name: "Warrior II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2185, Name: "Hammerhead II", Category: "Drones", VolumeM3: 10.0},
	{TypeID: 3244, Name: "Warp Scrambler II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 2205, Name: "Ballistic Control System II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 519, Name: "Gyrostabilizer II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 10190, Name: "Magnetic Field Stabilizer II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 1952, Name: "Sensor Booster II", Category: "Modules", VolumeM3: 5.0},
}

// ScanManufacturingOpportunities evaluates all SDE blueprints against live Jita Public ESI prices
// and ranks them by ISK/Hour per slot.
func (s *CopilotService) ScanManufacturingOpportunities(
	ctx context.Context,
	category string,
	runs, me, te int,
	structureKey, materialPriceSource string,
) ([]domain.ManufacturingOpportunity, error) {
	if s.sde == nil {
		return nil, fmt.Errorf("EVE SDE blueprint database is not available")
	}
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market provider is not available")
	}
	if runs <= 0 {
		runs = 10
	}

	bps, err := s.sde.SearchBlueprints(ctx, "", category, 100)
	if err != nil {
		return nil, fmt.Errorf("loading blueprints: %w", err)
	}
	if len(bps) == 0 {
		return []domain.ManufacturingOpportunity{}, nil
	}

	settings, _ := s.GetSettings(ctx)
	matsByBP := make(map[int32][]domain.SDEMaterialRequirement, len(bps))
	seen := make(map[int32]struct{})
	typeIDs := make([]int32, 0, len(bps)+16)
	addID := func(id int32) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			typeIDs = append(typeIDs, id)
		}
	}

	for _, bp := range bps {
		addID(bp.ProductTypeID)
		reqs, mErr := s.sde.GetBlueprintMaterials(ctx, bp.BlueprintTypeID)
		if mErr != nil || len(reqs) == 0 {
			continue
		}
		matsByBP[bp.BlueprintTypeID] = reqs
		for _, r := range reqs {
			addID(r.MaterialTypeID)
		}
	}

	prices, err := s.market.GetJitaPrices(ctx, typeIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching live Jita prices from Public ESI: %w", err)
	}

	results := make([]domain.ManufacturingOpportunity, 0, len(bps))
	for _, bp := range bps {
		reqs := matsByBP[bp.BlueprintTypeID]
		if len(reqs) == 0 {
			continue
		}
		calcRes, cErr := domain.CalculateBlueprintManufacturing(bp, reqs, domain.BlueprintCalcInput{
			BlueprintTypeID:     bp.BlueprintTypeID,
			Runs:                runs,
			ME:                  me,
			TE:                  te,
			StructureKey:        structureKey,
			MaterialPriceSource: materialPriceSource,
			SystemCostIndex:     settings.SystemCostIndex,
			FacilityTaxPercent:  settings.FacilityTaxPercent,
			FreightCostPerM3:    settings.FreightCostPerM3,
			BrokerFeePercent:    settings.BrokerFeePercent,
			SalesTaxPercent:     settings.SalesTaxPercent,
			RelistFeePercent:    settings.RelistFeePercent,
			EstimatedRelists:    1,
			BatchesPerWeek:      4,
		}, prices)
		if cErr != nil || calcRes.ProductUnitSellPrice <= 0 {
			continue
		}

		results = append(results, domain.ManufacturingOpportunity{
			BlueprintTypeID:   bp.BlueprintTypeID,
			ProductTypeID:     bp.ProductTypeID,
			ProductName:       bp.ProductName,
			Category:          bp.Category,
			Runs:              runs,
			OutputQty:         calcRes.TotalOutputQty,
			MaterialCost:      calcRes.TotalMaterialCost,
			ProductUnitSell:   calcRes.ProductUnitSellPrice,
			NetProfit:         calcRes.Profit.NetProfitPerBatch,
			NetMarginPercent:  calcRes.Profit.NetMarginPercent,
			ROIPercent:        calcRes.Profit.ROIPercent,
			ISKPerHour:        calcRes.ISKPerHour,
			DurationFormatted: calcRes.DurationFormatted,
			Verdict:           domain.ClassifyManufacturingVerdict(calcRes.Profit.NetProfitPerBatch, calcRes.Profit.NetMarginPercent, calcRes.ISKPerHour),
		})
	}

	domain.SortManufacturingOpportunities(results)
	return results, nil
}

// ScanStationTradingOpportunities scans high-liquidity Jita items + user Catalog items for profitable Buy->Sell flips.
func (s *CopilotService) ScanStationTradingOpportunities(ctx context.Context, category string) ([]domain.StationTradeOpportunity, error) {
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market provider is not available")
	}

	settings, _ := s.GetSettings(ctx)
	catFilter := strings.TrimSpace(category)

	basket := make([]scannerBasketItem, 0, len(defaultStationTradeBasket)+10)
	seen := make(map[int32]struct{})

	for _, b := range defaultStationTradeBasket {
		if catFilter != "" && !strings.EqualFold(b.Category, catFilter) {
			continue
		}
		basket = append(basket, b)
		seen[b.TypeID] = struct{}{}
	}

	// Merge user's own catalog items that have resolved TypeIDs
	if s.items != nil {
		if catalogItems, err := s.items.ListItems(ctx); err == nil {
			for _, it := range catalogItems {
				if it.TypeID <= 0 {
					continue
				}
				if _, ok := seen[it.TypeID]; ok {
					continue
				}
				if catFilter != "" && !strings.EqualFold(string(it.Category), catFilter) {
					continue
				}
				basket = append(basket, scannerBasketItem{
					TypeID:   it.TypeID,
					Name:     it.Name,
					Category: string(it.Category),
				})
				seen[it.TypeID] = struct{}{}
			}
		}
	}

	typeIDs := make([]int32, 0, len(basket))
	for _, b := range basket {
		typeIDs = append(typeIDs, b.TypeID)
	}

	prices, err := s.market.GetJitaPrices(ctx, typeIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching Jita station trading prices: %w", err)
	}

	results := make([]domain.StationTradeOpportunity, 0, len(basket))
	for _, b := range basket {
		jp, ok := prices[b.TypeID]
		if !ok || jp.LowestSell <= 0 || jp.HighestBuy <= 0 {
			continue
		}
		results = append(results, domain.EvaluateStationTrade(
			b.TypeID, b.Name, b.Category, jp,
			settings.BrokerFeePercent, settings.SalesTaxPercent,
		))
	}

	domain.SortStationTradeOpportunities(results)
	return results, nil
}

// ScanInterHubArbitrageOpportunities compares live prices between two major Empire trade hubs
// after deducting hauling/freight cost and destination market taxes.
func (s *CopilotService) ScanInterHubArbitrageOpportunities(ctx context.Context, originKey, destKey string) ([]domain.ArbitrageOpportunity, error) {
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market provider is not available")
	}
	browser, ok := s.market.(domain.MarketBrowserProvider)
	if !ok {
		return nil, fmt.Errorf("multi-hub order book provider is not available")
	}

	originHub := domain.LookupTradeHub(originKey)
	destHub := domain.LookupTradeHub(destKey)
	if originHub.Key == destHub.Key {
		return nil, fmt.Errorf("origin hub and destination hub must be different")
	}

	settings, _ := s.GetSettings(ctx)
	results := make([]domain.ArbitrageOpportunity, 0, len(defaultArbitrageBasket))

	for i, item := range defaultArbitrageBasket {
		if i > 0 {
			select {
			case <-ctx.Done():
				return results, ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}
		}
		_, _, originSum, oErr := browser.GetRegionOrderBook(ctx, originHub, item.TypeID, 1)
		if oErr != nil || originSum.LowestSell <= 0 {
			continue
		}
		_, _, destSum, dErr := browser.GetRegionOrderBook(ctx, destHub, item.TypeID, 1)
		if dErr != nil || destSum.LowestSell <= 0 {
			continue
		}

		opp := domain.EvaluateArbitrage(
			item.TypeID, item.Name, item.Category,
			originHub, destHub,
			originSum.LowestSell, destSum.LowestSell, destSum.HighestBuy,
			item.VolumeM3, settings.FreightCostPerM3,
			settings.BrokerFeePercent, settings.SalesTaxPercent,
		)
		results = append(results, opp)
	}

	domain.SortArbitrageOpportunities(results)
	return results, nil
}
