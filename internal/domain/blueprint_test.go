package domain_test

import (
	"strings"
	"testing"

	"aura-forge/internal/domain"
)

func TestComputeAdjustedMaterialQty_MEAndStructureStacking(t *testing.T) {
	// 1000 base * 10 runs * 0.90 (ME 10) * 1.0 (NPC) = 9000
	if got := domain.ComputeAdjustedMaterialQty(1000, 10, 10, 0); got != 9000 {
		t.Fatalf("expected 9000, got %d", got)
	}

	// Minimum 1 unit per run rule: baseQty=1, runs=5, ME 10 -> must still be 5
	if got := domain.ComputeAdjustedMaterialQty(1, 5, 10, 5.2); got != 5 {
		t.Fatalf("expected minimum 5 (1 per run), got %d", got)
	}

	// Stacking ME 10% + Raitaru T1 (3%): 1000 * 1 * 0.90 * 0.97 = 873
	if got := domain.ComputeAdjustedMaterialQty(1000, 1, 10, 3.0); got != 873 {
		t.Fatalf("expected 873, got %d", got)
	}
}

func TestCalculateBlueprintManufacturing_EndToEnd(t *testing.T) {
	bp := domain.SDEBlueprint{
		BlueprintTypeID:  2455,
		Name:             "Hobgoblin I Blueprint",
		ProductTypeID:    2454,
		ProductName:      "Hobgoblin I",
		ProductQtyPerRun: 1,
		BaseTimeSeconds:  1800,
		TechLevel:        1,
		Category:         "Drones",
	}
	reqs := []domain.SDEMaterialRequirement{
		{MaterialTypeID: 34, MaterialName: "Tritanium", BaseQuantity: 1000, VolumeM3: 0.01},
		{MaterialTypeID: 35, MaterialName: "Pyerite", BaseQuantity: 500, VolumeM3: 0.01},
	}
	prices := map[int32]domain.JitaPrice{
		34:   {TypeID: 34, LowestSell: 5.0, HighestBuy: 4.5},
		35:   {TypeID: 35, LowestSell: 10.0, HighestBuy: 9.0},
		2454: {TypeID: 2454, LowestSell: 15000.0, HighestBuy: 12000.0},
	}

	res, err := domain.CalculateBlueprintManufacturing(bp, reqs, domain.BlueprintCalcInput{
		BlueprintTypeID:     2455,
		Runs:                10,
		ME:                  10,
		TE:                  20,
		StructureKey:        "npc_station",
		MaterialPriceSource: "jita_sell",
		SystemCostIndex:     0.045,
		FacilityTaxPercent:  0.25,
		BrokerFeePercent:    3.0,
		SalesTaxPercent:     3.6,
		BatchesPerWeek:      5,
	}, prices)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tritanium: 1000 * 10 * 0.9 = 9000 * 5 = 45,000 ISK
	// Pyerite:   500 * 10 * 0.9 = 4500 * 10 = 45,000 ISK
	// Total material cost = 90,000 ISK
	if res.TotalMaterialCost != 90000 {
		t.Fatalf("expected total material cost 90000, got %.2f", res.TotalMaterialCost)
	}
	// Duration: 1800 * 10 * 0.8 = 14400s (4h)
	if res.EstimatedDurationSeconds != 14400 || res.DurationFormatted != "4h" {
		t.Fatalf("expected 14400s (4h), got %ds (%s)", res.EstimatedDurationSeconds, res.DurationFormatted)
	}
	if !res.Profit.IsProfitable || res.ISKPerHour <= 0 {
		t.Fatalf("expected profitable result with positive ISK/hr, got profit=%.2f isk/hr=%.2f", res.Profit.NetProfitPerBatch, res.ISKPerHour)
	}
	if !strings.Contains(res.MultibuyText, "Tritanium\t9000") || !strings.Contains(res.MultibuyText, "Pyerite\t4500") {
		t.Fatalf("unexpected multibuy text:\n%s", res.MultibuyText)
	}
}
