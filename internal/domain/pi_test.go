package domain_test

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestListProduciblePICommodities_TiersP1ToP4(t *testing.T) {
	all := domain.ListProduciblePICommodities("ALL", "")
	if len(all) != 68 {
		t.Fatalf("expected 68 producible P1-P4 PI items, got %d", len(all))
	}
	p1 := domain.ListProduciblePICommodities("P1", "")
	p2 := domain.ListProduciblePICommodities("P2", "")
	p3 := domain.ListProduciblePICommodities("P3", "")
	p4 := domain.ListProduciblePICommodities("P4", "")
	if len(p1) != 15 || len(p2) != 24 || len(p3) != 21 || len(p4) != 8 {
		t.Fatalf("unexpected tier counts: P1=%d P2=%d P3=%d P4=%d", len(p1), len(p2), len(p3), len(p4))
	}
}

func TestBuildPIRecipeTree_RoboticsShowsRecursiveInputsAndPlanets(t *testing.T) {
	prices := map[int32]domain.JitaPrice{
		9848: {LowestSell: 110_000}, // Robotics (P3)
		3689: {LowestSell: 11_000},  // Mechanical Parts (P2)
		9836: {LowestSell: 10_000},  // Consumer Electronics (P2)
	}

	res, err := domain.BuildPIRecipeTree(9848, prices, 5.0)
	if err != nil {
		t.Fatalf("BuildPIRecipeTree failed: %v", err)
	}
	if res.Commodity.Name != "Robotics" || res.Commodity.Tier != domain.PITierP3 {
		t.Fatalf("unexpected root commodity: %+v", res.Commodity)
	}
	if len(res.P0Requirements) != 4 {
		t.Fatalf("expected 4 P0 raw resources for Robotics (Noble, Base, Non-CS Crystals, Heavy Metals), got %d", len(res.P0Requirements))
	}
	if len(res.RequiredPlanets) == 0 {
		t.Fatalf("expected source planets to be populated")
	}
	if res.NetProfitPerCycle <= 0 || res.EstMonthlyISK <= 0 {
		t.Fatalf("expected positive Robotics factory profit, got cycle=%.2f monthly=%.2f", res.NetProfitPerCycle, res.EstMonthlyISK)
	}
}

func TestBuildPlanetCatalog_All8PlanetsWithResourcesAndSinglePlanetP2(t *testing.T) {
	catalog := domain.BuildPlanetCatalog()
	if len(catalog) != 8 {
		t.Fatalf("expected 8 planet types, got %d", len(catalog))
	}
	for _, pl := range catalog {
		if pl.PlanetTypeID <= 0 {
			t.Fatalf("planet %s missing official TypeID", pl.PlanetType)
		}
		if len(pl.P0Resources) != 5 || len(pl.P1Products) != 5 {
			t.Fatalf("planet %s expected 5 P0 and 5 P1, got P0=%d P1=%d", pl.PlanetType, len(pl.P0Resources), len(pl.P1Products))
		}
		if len(pl.SinglePlanetP2) == 0 {
			t.Fatalf("planet %s expected at least 1 single-planet P2 product", pl.PlanetType)
		}
	}
}

func TestPISalesCalculator_MultiItemAndPaste(t *testing.T) {
	paste := "Coolant\t2,000\nMechanical Parts\t1500\nRandom Junk Item\t50"
	parsed, unmatched := domain.ParsePIInventoryPaste(paste)
	if len(parsed) != 2 {
		t.Fatalf("expected 2 recognized PI items from paste, got %d", len(parsed))
	}
	if len(unmatched) != 1 {
		t.Fatalf("expected 1 unmatched non-PI line, got %d", len(unmatched))
	}

	prices := map[int32]domain.JitaPrice{
		9832: {LowestSell: 11_000, HighestBuy: 10_000}, // Coolant (P2)
		3689: {LowestSell: 12_000, HighestBuy: 11_000}, // Mechanical Parts (P2)
	}
	opts := domain.PISalesCalcOptions{
		Hub:              domain.LookupTradeHub("amarr"),
		CostMode:         "self_extracted",
		SellMode:         "sell_order",
		POCOTaxPercent:   5.0,
		BrokerFeePercent: 3.0,
		SalesTaxPercent:  3.6,
	}

	res := domain.CalculatePISalesProfit(parsed, prices, opts)
	if len(res.Lines) != 2 || res.TotalUnits != 3500 {
		t.Fatalf("expected 2 lines and 3500 units, got %d lines / %d units", len(res.Lines), res.TotalUnits)
	}
	if res.TotalGrossRevenue != 40_000_000 {
		t.Fatalf("expected gross 40,000,000 ISK, got %.2f", res.TotalGrossRevenue)
	}
	if res.TotalNetProfit <= 0 || res.TotalPOCOTax <= 0 || res.TotalMarketFees <= 0 {
		t.Fatalf("expected positive net profit, poco tax, and market fees, got %+v", res)
	}
	// 3,500 P2 units * 1.5 m3 = 5,250 m3 -> fits 1 Epithal trip
	if res.TotalVolumeM3 != 5250.0 || res.EpithalTrips != 1 {
		t.Fatalf("expected 5250.0 m3 and 1 Epithal trip, got %.1f m3 / %d trips", res.TotalVolumeM3, res.EpithalTrips)
	}

	// Instant Sell mode must use HighestBuy and zero broker fee
	opts.SellMode = "instant_buy"
	inst := domain.CalculatePISalesProfit(parsed, prices, opts)
	if inst.TotalGrossRevenue != 36_500_000 {
		t.Fatalf("expected instant sell gross 36,500,000 ISK, got %.2f", inst.TotalGrossRevenue)
	}
}
