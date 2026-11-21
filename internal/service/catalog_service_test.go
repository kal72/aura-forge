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
