package domain_test

import (
	"strings"
	"testing"

	"aura-forge/internal/domain"
)

func TestEvaluateIndustryTree_OptimalMakeVsBuyAndManualOverride(t *testing.T) {
	rootBP := domain.SDEBlueprint{
		BlueprintTypeID:  2457,
		Name:             "Hobgoblin II Blueprint",
		ProductTypeID:    2456,
		ProductName:      "Hobgoblin II",
		ProductQtyPerRun: 1,
		BaseTimeSeconds:  5400,
		TechLevel:        2,
		Category:         "Drones",
	}
	hob1BP := domain.SDEBlueprint{
		BlueprintTypeID:  2455,
		Name:             "Hobgoblin I Blueprint",
		ProductTypeID:    2454,
		ProductName:      "Hobgoblin I",
		ProductQtyPerRun: 1,
		BaseTimeSeconds:  1800,
		TechLevel:        1,
		Category:         "Drones",
	}
	ramBP := domain.SDEBlueprint{
		BlueprintTypeID:  11485,
		Name:             "R.A.M.-Robotics Blueprint",
		ProductTypeID:    11483,
		ProductName:      "R.A.M.-Robotics",
		ProductQtyPerRun: 100,
		BaseTimeSeconds:  3600,
		TechLevel:        1,
		Category:         "Components",
	}

	bpsByProduct := map[int32]domain.SDEBlueprint{
		2454:  hob1BP,
		11483: ramBP,
	}
	matsByBP := map[int32][]domain.SDEMaterialRequirement{
		2457: {
			{MaterialTypeID: 2454, MaterialName: "Hobgoblin I", BaseQuantity: 1, VolumeM3: 5},
			{MaterialTypeID: 11483, MaterialName: "R.A.M.-Robotics", BaseQuantity: 1, VolumeM3: 0.04},
			{MaterialTypeID: 11399, MaterialName: "Morphite", BaseQuantity: 8, VolumeM3: 0.01},
		},
		2455: {
			{MaterialTypeID: 34, MaterialName: "Tritanium", BaseQuantity: 1000, VolumeM3: 0.01},
		},
		11485: {
			{MaterialTypeID: 34, MaterialName: "Tritanium", BaseQuantity: 5000, VolumeM3: 0.01},
		},
	}

	// Hobgoblin I: BUY = 20,000 ISK vs MAKE = ~4,000 ISK => should auto-choose MAKE
	// R.A.M.-Robotics: BUY = 500 ISK vs MAKE = ~20,000 ISK (1 run=100) => should auto-choose BUY
	prices := map[int32]domain.JitaPrice{
		2456:  {LowestSell: 800_000},
		2454:  {LowestSell: 20_000},
		11483: {LowestSell: 500},
		11399: {LowestSell: 30_000},
		34:    {LowestSell: 4},
	}

	in := domain.BlueprintCalcInput{
		BlueprintTypeID:    2457,
		Runs:               10,
		ME:                 2,
		TE:                 4,
		HubKey:             "jita",
		SystemCostIndex:    0.04,
		FacilityTaxPercent: 0.25,
	}

	tree := domain.EvaluateIndustryTree(rootBP, bpsByProduct, matsByBP, prices, in)
	if !tree.HasSubBlueprints {
		t.Fatalf("expected HasSubBlueprints=true")
	}
	if tree.OptimalSavingsISK <= 0 {
		t.Fatalf("expected positive OptimalSavingsISK when making Hobgoblin I, got %.2f", tree.OptimalSavingsISK)
	}
	if tree.OptimalCost >= tree.BuyAllDirectCost {
		t.Fatalf("expected OptimalCost (%.2f) < BuyAllDirectCost (%.2f)", tree.OptimalCost, tree.BuyAllDirectCost)
	}

	// Verify Multibuy includes Tritanium (for MAKE Hobgoblin I), R.A.M.-Robotics (BUY), and Morphite (RAW), NOT Hobgoblin I
	if strings.Contains(tree.OptimalMultibuy, "Hobgoblin I\t") {
		t.Fatalf("did not expect Hobgoblin I in OptimalMultibuy when set to MAKE: %s", tree.OptimalMultibuy)
	}
	if !strings.Contains(tree.OptimalMultibuy, "Tritanium\t") || !strings.Contains(tree.OptimalMultibuy, "R.A.M.-Robotics\t") {
		t.Fatalf("expected Tritanium and R.A.M.-Robotics in OptimalMultibuy: %s", tree.OptimalMultibuy)
	}

	// Verify bottom-up SubJobs: Step 1 Hobgoblin I -> Step 2 Hobgoblin II
	if len(tree.SubJobs) != 2 || tree.SubJobs[0].ProductName != "Hobgoblin I" || tree.SubJobs[1].ProductName != "Hobgoblin II" {
		t.Fatalf("unexpected SubJobs order: %+v", tree.SubJobs)
	}

	// Test Manual Override: force Hobgoblin I -> BUY
	in.MakeBuyOverrides = domain.ParseMakeBuyOverrides("2454:BUY")
	overridden := domain.EvaluateIndustryTree(rootBP, bpsByProduct, matsByBP, prices, in)
	if !strings.Contains(overridden.OptimalMultibuy, "Hobgoblin I\t") {
		t.Fatalf("expected Hobgoblin I in OptimalMultibuy after manual BUY override: %s", overridden.OptimalMultibuy)
	}
	if len(overridden.SubJobs) != 1 {
		t.Fatalf("expected only 1 SubJob (root) after overriding Hobgoblin I to BUY, got %d", len(overridden.SubJobs))
	}
}
