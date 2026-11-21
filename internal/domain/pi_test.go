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
