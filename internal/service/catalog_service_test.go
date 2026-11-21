package service

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestSummarizeAndSortCatalogItems(t *testing.T) {
	items := []domain.MarketItem{
		{
			Name:        "Zebra Drone",
			DailyVolume: 500,
			TrapAnalysis: domain.TrapReport{
				Status:             domain.TrapStatusJebakan,
				EstimatedNetProfit: -1_000_000,
				EstimatedROIPct:    -12.5,
			},
		},
		{
			Name:        "Alpha Missile",
			DailyVolume: 25_000,
			TrapAnalysis: domain.TrapReport{
				Status:             domain.TrapStatusLayak,
				EstimatedNetProfit: 4_500_000,
				EstimatedROIPct:    18.0,
				BOMSyncedFromSDE:   true,
			},
		},
		{
			Name:        "Middle Frigate",
			DailyVolume: 2_000,
			TrapAnalysis: domain.TrapReport{
				Status:             domain.TrapStatusWaspadaNiche,
				EstimatedNetProfit: 9_000_000,
				EstimatedROIPct:    32.0,
				IsPriceSpike:       true,
				BOMSyncedFromSDE:   true,
			},
		},
	}

	sum := SummarizeCatalog(items)
	if sum.TotalCount != 3 || sum.ViableCount != 1 || sum.WatchlistCount != 1 || sum.TrapCount != 1 {
		t.Fatalf("unexpected summary counts: %+v", sum)
	}
	if sum.SDESyncedCount != 2 || sum.SpikeCount != 1 {
		t.Fatalf("expected SDESyncedCount=2 and SpikeCount=1, got %+v", sum)
	}

	SortCatalogItems(items, "roi")
	if items[0].Name != "Middle Frigate" || items[2].Name != "Zebra Drone" {
		t.Errorf("expected ROI sort [Middle Frigate, ..., Zebra Drone], got %s .. %s", items[0].Name, items[2].Name)
	}

	SortCatalogItems(items, "profit")
	if items[0].Name != "Middle Frigate" {
		t.Errorf("expected Profit sort first=Middle Frigate, got %s", items[0].Name)
	}

	SortCatalogItems(items, "volume")
	if items[0].Name != "Alpha Missile" {
		t.Errorf("expected Volume sort first=Alpha Missile, got %s", items[0].Name)
	}

	SortCatalogItems(items, "risk")
	if items[0].TrapAnalysis.Status != domain.TrapStatusLayak || items[2].TrapAnalysis.Status != domain.TrapStatusJebakan {
		t.Errorf("expected Risk sort LAYAK first and JEBAKAN last, got %s .. %s", items[0].TrapAnalysis.Status, items[2].TrapAnalysis.Status)
	}

	SortCatalogItems(items, "name")
	if items[0].Name != "Alpha Missile" || items[2].Name != "Zebra Drone" {
		t.Errorf("expected Name sort [Alpha Missile, ..., Zebra Drone], got %s .. %s", items[0].Name, items[2].Name)
	}
}

func TestComputePriceIntelligence_Regimes(t *testing.T) {
	settings := domain.DefaultUserSettings()

	// 1. Competitive normal market: 1-tick undercut below Lowest Sell, above Break-Even
	piComp := ComputePriceIntelligence(100_000, 92_000, 95_000, 99_000, 105_000, 700_000, 10, settings)
	if piComp.PriceRegime != "COMPETITIVE" {
		t.Fatalf("expected COMPETITIVE regime, got %+v", piComp)
	}
	if piComp.RecommendedSellPrice >= 100_000 || piComp.RecommendedSellPrice <= piComp.BreakEvenPrice {
		t.Errorf("expected competitive recommended price between break-even (%.2f) and lowest sell (100000), got %.2f",
			piComp.BreakEvenPrice, piComp.RecommendedSellPrice)
	}

	// 2. Price Spike market (+40% above 7d avg): pulls recommended price realistically back near 7d avg
	piSpike := ComputePriceIntelligence(140_000, 95_000, 94_000, 100_000, 145_000, 700_000, 10, settings)
	if piSpike.PriceRegime != "SPIKE" {
		t.Fatalf("expected SPIKE regime, got %+v", piSpike)
	}
	if piSpike.RecommendedSellPrice >= 140_000 || piSpike.RecommendedSellPrice < 100_000 {
		t.Errorf("expected spike recommended price near 7d avg (~103000), got %.2f", piSpike.RecommendedSellPrice)
	}

	// 3. Below-cost market: lifts recommended price to safe floor above Break-Even
	piLow := ComputePriceIntelligence(75_000, 70_000, 72_000, 76_000, 80_000, 800_000, 10, settings)
	if piLow.PriceRegime != "BELOW_COST" {
		t.Fatalf("expected BELOW_COST regime, got %+v", piLow)
	}
	if piLow.RecommendedSellPrice <= piLow.BreakEvenPrice {
		t.Errorf("expected recommended price above break-even (%.2f), got %.2f", piLow.BreakEvenPrice, piLow.RecommendedSellPrice)
	}

	// 4. EVE 4-significant-figure rounding check
	if got := RoundEVE4SigFigs(123456.78); got != 123500 {
		t.Errorf("expected RoundEVE4SigFigs(123456.78) == 123500, got %.2f", got)
	}
}
