package domain_test

import (
	"math"
	"strings"
	"testing"

	"aura-forge/internal/domain"
)

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestCalculateProfit_HappyPathAndGuardClauses(t *testing.T) {
	// Guard clauses
	if _, err := domain.CalculateProfit(domain.ProfitCalcInput{OutputQty: 0}); err != domain.ErrInvalidOutputQty {
		t.Fatalf("expected ErrInvalidOutputQty, got %v", err)
	}
	if _, err := domain.CalculateProfit(domain.ProfitCalcInput{OutputQty: 1, MaterialCost: -10}); err != domain.ErrNegativePriceOrCost {
		t.Fatalf("expected ErrNegativePriceOrCost, got %v", err)
	}
	if _, err := domain.CalculateProfit(domain.ProfitCalcInput{OutputQty: 1, BrokerFeePercent: 60, SalesTaxPercent: 45}); err != domain.ErrExcessiveFeeRate {
		t.Fatalf("expected ErrExcessiveFeeRate, got %v", err)
	}

	// Happy path
	res, err := domain.CalculateProfit(domain.ProfitCalcInput{
		ItemName:           "Hobgoblin II",
		OutputQty:          10,
		UnitSellPrice:      1_000_000, // Gross = 10,000,000
		MaterialCost:       6_500_000,
		ExtraSetupCost:     200_000,
		SystemCostIndex:    0.04, // 4% -> 260,000 * 1.0025 = 260,650
		FacilityTaxPercent: 0.25,
		FreightCost:        40_000,
		BrokerFeePercent:   3.0, // 300,000
		SalesTaxPercent:    3.6, // 360,000
		RelistFeePercent:   0.5, // 50,000 * 2 = 100,000
		EstimatedRelists:   2,
		BatchesPerWeek:     5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.GrossRevenue != 10_000_000 {
		t.Errorf("expected GrossRevenue 10,000,000, got %.2f", res.GrossRevenue)
	}
	if !almostEqual(res.JobInstallCost, 260_650, 0.01) {
		t.Errorf("expected JobInstallCost 260650, got %.2f", res.JobInstallCost)
	}
	if !almostEqual(res.TotalMarketFees, 760_000, 0.01) {
		t.Errorf("expected TotalMarketFees 760000, got %.2f", res.TotalMarketFees)
	}
	if !res.IsProfitable {
		t.Errorf("expected batch to be profitable")
	}
	if !almostEqual(res.WeeklyNetProfit, res.NetProfitPerBatch*5, 0.05) {
		t.Errorf("expected WeeklyNetProfit to be 5x NetProfitPerBatch, got %.2f vs %.2f", res.WeeklyNetProfit, res.NetProfitPerBatch*5)
	}
	// Verify BreakEvenUnitPrice produces ~0 net profit
	if res.BreakEvenUnitPrice <= 0 || res.BreakEvenUnitPrice >= 1_000_000 {
		t.Errorf("unexpected BreakEvenUnitPrice: %.2f", res.BreakEvenUnitPrice)
	}
}

func TestEvaluateTrapGuard_DetectsAllTraps(t *testing.T) {
	settings := domain.DefaultUserSettings()
	capital := 100_000_000.0

	// 1. Rare BPC Trap (ExtraSetupCost == 0)
	rareItem := domain.MarketItem{
		Name:              "Caldari Navy Ballistic Control System",
		Category:          domain.CategoryModules,
		EstimatedJitaSell: 85_000_000,
		MaterialCost:      10_000_000,
		BatchOutputQty:    1,
		DailyVolume:       150,
		IsManufacturable:  true,
		ExtraSetupCost:    0,
	}
	r1 := domain.EvaluateTrapGuard(rareItem, settings, capital)
	if r1.Status != domain.TrapStatusJebakan || !r1.IsBPCJebakan || r1.RiskLevel != domain.RiskCritical {
		t.Errorf("expected JEBAKAN BPC LANGKA (CRITICAL), got %+v", r1)
	}

	// 2. Non-Manufacturable Trap (Compressed Ice)
	iceItem := domain.MarketItem{
		Name:              "Compressed Blue Ice",
		Category:          domain.CategoryCommodities,
		EstimatedJitaSell: 400_000,
		MaterialCost:      350_000,
		BatchOutputQty:    100,
		DailyVolume:       10_000,
		IsManufacturable:  false,
	}
	r2 := domain.EvaluateTrapGuard(iceItem, settings, capital)
	if r2.Status != domain.TrapStatusJebakan || !r2.IsNonManufacturable {
		t.Errorf("expected JEBAKAN non-manufacturable, got %+v", r2)
	}

	// 3. Slow Mover Niche
	slowItem := domain.MarketItem{
		Name:              "Large Auxiliary Nano Pump I",
		Category:          domain.CategoryRigs,
		EstimatedJitaSell: 15_000_000,
		MaterialCost:      10_000_000,
		BatchOutputQty:    5,
		DailyVolume:       1.5, // Less than 5 units/batch
		IsManufacturable:  true,
	}
	r3 := domain.EvaluateTrapGuard(slowItem, settings, capital)
	if !r3.IsSlowMover || r3.Status != domain.TrapStatusWaspadaNiche {
		t.Errorf("expected WASPADA_NICHE slow mover, got %+v", r3)
	}

	// 4. Capital Lock-Up (>50% of 100M capital)
	heavyItem := domain.MarketItem{
		Name:              "Caracal",
		Category:          domain.CategoryShips,
		EstimatedJitaSell: 12_000_000,
		MaterialCost:      65_000_000, // 65M for 10 batch
		BatchOutputQty:    10,
		DailyVolume:       200,
		IsManufacturable:  true,
	}
	r4 := domain.EvaluateTrapGuard(heavyItem, settings, capital)
	if !r4.IsModalLockup || r4.Status != domain.TrapStatusWaspadaModal {
		t.Errorf("expected WASPADA_MODAL lock-up, got %+v", r4)
	}

	// 5. Healthy Layak Produksi item
	goodItem := domain.MarketItem{
		Name:              "Scourge Heavy Missile",
		Category:          domain.CategoryAmmunition,
		EstimatedJitaSell: 85,
		MaterialCost:      5_000_000,
		BatchOutputQty:    100_000, // Gross 8.5M
		DailyVolume:       5_000_000,
		IsManufacturable:  true,
	}
	r5 := domain.EvaluateTrapGuard(goodItem, settings, capital)
	if r5.Status != domain.TrapStatusLayak || r5.RiskLevel != domain.RiskLow {
		t.Errorf("expected LAYAK_PRODUKSI (LOW), got %+v", r5)
	}
}

func TestAllocateCapital_FourBucketsAndGuardClauses(t *testing.T) {
	settings := domain.DefaultUserSettings()

	// Test ResolveAvailableCapital
	manualCap, src := domain.ResolveAvailableCapital(settings, nil, false)
	if manualCap != 100_000_000 || !strings.Contains(src, "Manual") {
		t.Errorf("expected manual capital 100M, got %.2f (%s)", manualCap, src)
	}
	ssoCap, ssoSrc := domain.ResolveAvailableCapital(settings, &domain.CharacterProfile{
		CharacterName: "Aura Pilot",
		WalletBalance: 250_000_000,
	}, true)
	if ssoCap != 250_000_000 || !strings.Contains(ssoSrc, "Aura Pilot") {
		t.Errorf("expected SSO capital 250M, got %.2f (%s)", ssoCap, ssoSrc)
	}

	// Guard clauses
	cfg := domain.DefaultAllocationConfig()
	if _, err := domain.AllocateCapital(0, "", cfg, nil, settings); err != domain.ErrInvalidCapital {
		t.Fatalf("expected ErrInvalidCapital, got %v", err)
	}
	badBuffer := domain.AllocationConfig{FastCashflowPct: 40, MidTierPct: 35, HighROIPct: 20, CashBufferPct: 5}
	if _, err := domain.AllocateCapital(100_000_000, "", badBuffer, nil, settings); err != domain.ErrBufferBelowMinimum {
		t.Fatalf("expected ErrBufferBelowMinimum, got %v", err)
	}
	badSum := domain.AllocationConfig{FastCashflowPct: 40, MidTierPct: 40, HighROIPct: 20, CashBufferPct: 15}
	if _, err := domain.AllocateCapital(100_000_000, "", badSum, nil, settings); err != domain.ErrInvalidBucketWeights {
		t.Fatalf("expected ErrInvalidBucketWeights, got %v", err)
	}

	// Valid allocation filtering out JEBAKAN items
	candidates := []domain.MarketItem{
		{
			TypeID:                2456,
			Name:                  "Hobgoblin I",
			Category:              domain.CategoryDrones,
			EstimatedJitaSell:     12_000,
			MaterialCost:          4_500_000,
			BatchOutputQty:        500,
			DailyVolume:           5000,
			IsManufacturable:      true,
			IsFrequentlyDestroyed: true,
		},
		{
			TypeID:            587,
			Name:              "Rifter",
			Category:          domain.CategoryShips,
			EstimatedJitaSell: 650_000,
			MaterialCost:      4_200_000,
			BatchOutputQty:    10,
			DailyVolume:       40,
			IsManufacturable:  true,
		},
		{
			TypeID:            99999,
			Name:              "Republic Fleet Gyrostabilizer", // Rare BPC trap with 0 setup cost
			Category:          domain.CategoryModules,
			EstimatedJitaSell: 90_000_000,
			MaterialCost:      10_000_000,
			BatchOutputQty:    1,
			DailyVolume:       100,
			IsManufacturable:  true,
			ExtraSetupCost:    0,
		},
	}

	res, err := domain.AllocateCapital(100_000_000, "Manual", cfg, candidates, settings)
	if err != nil {
		t.Fatalf("unexpected allocation error: %v", err)
	}
	if len(res.Buckets) != 4 {
		t.Fatalf("expected 4 buckets, got %d", len(res.Buckets))
	}
	if res.CashBufferISK != 15_000_000 {
		t.Errorf("expected 15M cash buffer, got %.2f", res.CashBufferISK)
	}
	// Ensure trap item was never allocated
	for _, b := range res.Buckets {
		for _, it := range b.Items {
			if it.TypeID == 99999 {
				t.Errorf("trap item should not be allocated into bucket %s", b.Type)
			}
		}
	}
}

func TestWeeklyPlanner_SlotsScheduleShoppingListAndGrowth(t *testing.T) {
	items := []domain.PlanItem{
		{
			TypeID:              2456,
			Name:                "Hobgoblin I",
			BatchRuns:           4,
			OutputPerBatch:      500,
			UnitSellPrice:       12_000,
			TotalMaterialCost:   18_000_000,
			TotalCost:           19_500_000,
			TotalGrossRevenue:   24_000_000,
			NetProfit:           4_500_000,
			ProductionTimeHours: 12.0,
		},
		{
			TypeID:              2488,
			Name:                "Warrior II",
			BatchRuns:           2,
			OutputPerBatch:      50,
			UnitSellPrice:       650_000,
			TotalMaterialCost:   45_000_000,
			TotalCost:           48_000_000,
			TotalGrossRevenue:   65_000_000,
			NetProfit:           17_000_000,
			ProductionTimeHours: 24.0,
			RequiresScienceSlot: true,
		},
	}

	plan := domain.BuildWeeklyPlan("Kampanye Minggu 1", 1, 100_000_000, 10, 1, items)
	if len(plan.Days) != 7 {
		t.Fatalf("expected 7 days schedule, got %d", len(plan.Days))
	}
	if plan.TotalEstimatedProfit != 21_500_000 {
		t.Errorf("expected total profit 21.5M, got %.2f", plan.TotalEstimatedProfit)
	}

	// Slot capacity validation (fits)
	capReport := domain.ValidateSlotCapacity(plan)
	if !capReport.FitsSlotCapacity {
		t.Errorf("expected plan to fit slot capacity, warning: %s", capReport.Warning)
	}

	// Slot capacity exceeded test (0 science slots)
	planNoSci := domain.BuildWeeklyPlan("No Sci", 1, 100_000_000, 10, 0, items)
	capFail := domain.ValidateSlotCapacity(planNoSci)
	if capFail.FitsSlotCapacity {
		t.Errorf("expected slot capacity validation to fail when science slots are insufficient")
	}

	// Consolidated Shopping List + Hangar Deduction + EVE Multibuy
	reqs := []domain.MaterialRequirement{
		{TypeID: 34, TypeName: "Tritanium", Quantity: 100_000, UnitPrice: 4.5, UnitVolumeM3: 0.01},
		{TypeID: 34, TypeName: "Tritanium", Quantity: 50_000, UnitPrice: 4.5, UnitVolumeM3: 0.01},
		{TypeID: 35, TypeName: "Pyerite", Quantity: 20_000, UnitPrice: 12.0, UnitVolumeM3: 0.01},
	}
	hangar := map[int32]int64{
		34: 30_000, // Deduct 30,000 Tritanium from 150,000 required -> buy 120,000
	}
	shop := domain.BuildConsolidatedShoppingList(reqs, hangar)
	if len(shop.Items) != 2 {
		t.Fatalf("expected 2 consolidated materials, got %d", len(shop.Items))
	}
	if shop.TotalItemsDeducted != 30_000 {
		t.Errorf("expected 30,000 deducted, got %d", shop.TotalItemsDeducted)
	}
	if !strings.Contains(shop.MultibuyText, "Tritanium\t120000") {
		t.Errorf("expected EVE Multibuy to contain 'Tritanium\\t120000', got:\n%s", shop.MultibuyText)
	}
	if !strings.Contains(shop.MultibuyText, "Pyerite\t20000") {
		t.Errorf("expected EVE Multibuy to contain 'Pyerite\\t20000', got:\n%s", shop.MultibuyText)
	}

	// 4-Week Growth Simulation
	growth, err := domain.SimulateFourWeekGrowth(100_000_000, 20.0, 0.85)
	if err != nil {
		t.Fatalf("unexpected simulation error: %v", err)
	}
	if len(growth) != 4 {
		t.Fatalf("expected 4 weeks of growth projection, got %d", len(growth))
	}
	if growth[3].EndingCapital <= growth[0].StartingCapital {
		t.Errorf("expected capital to grow over 4 weeks, start=%.2f end=%.2f", growth[0].StartingCapital, growth[3].EndingCapital)
	}

	// Convert Plan to Batches
	batches := domain.ConvertPlanToBatches(plan, domain.DefaultUserSettings())
	if len(batches) != 2 {
		t.Fatalf("expected 2 draft batches, got %d", len(batches))
	}
	if batches[0].Status != domain.BatchStatusDraft || batches[0].SafeRelistPrice <= 0 {
		t.Errorf("unexpected converted batch: %+v", batches[0])
	}
}

func TestEvaluateRelistGuard(t *testing.T) {
	settings := domain.DefaultUserSettings()
	batch := domain.BatchRecord{
		ItemName:             "Caracal",
		UnitsOutput:          10,
		ActualProductionCost: 100_000_000, // 10M per unit direct cost
		ActualSellPrice:      13_000_000,  // Listed at 13M
	}

	// Safe undercut at 12.5M
	safeRes := domain.EvaluateRelistGuard(batch, 12_500_000, settings)
	if !safeRes.IsSafeToRelist || !safeRes.IsUndercut {
		t.Errorf("expected 12.5M to be safe relist undercut, got %+v", safeRes)
	}

	// Unsafe undercut below cost at 10.1M (after taxes & broker fees -> loss)
	unsafeRes := domain.EvaluateRelistGuard(batch, 10_100_000, settings)
	if unsafeRes.IsSafeToRelist {
		t.Errorf("expected 10.1M to be unsafe relist, got %+v", unsafeRes)
	}
}

func TestImporter_ParseBulkTableAndNotations(t *testing.T) {
	val, err := domain.ParseISKNotation("12.5M ISK")
	if err != nil || val != 12_500_000 {
		t.Errorf("expected 12,500,000, got %.2f (err: %v)", val, err)
	}
	valK, err := domain.ParseISKNotation("450K")
	if err != nil || valK != 450_000 {
		t.Errorf("expected 450,000, got %.2f (err: %v)", valK, err)
	}
	pct, err := domain.ParsePercentNotation("18.75%")
	if err != nil || pct != 18.75 {
		t.Errorf("expected 18.75, got %.2f (err: %v)", pct, err)
	}

	raw := `Name	Sell Price	Material Cost	Daily Volume	Batch Qty
Hobgoblin II	1.2M	8.5M	15K	10
Compressed Blue Ice	420K	390K	50K	100
Broken Item	INVALID_NUM	1M	500	1`

	res := domain.ParseBulkTable(raw)
	if res.TotalLines != 3 {
		t.Errorf("expected 3 data lines, got %d", res.TotalLines)
	}
	if res.SuccessCount != 2 {
		t.Errorf("expected 2 valid items, got %d", res.SuccessCount)
	}
	if res.ErrorCount != 1 || len(res.Errors) != 1 {
		t.Errorf("expected 1 row error, got %d", res.ErrorCount)
	}
	if res.Errors[0].LineNumber != 4 {
		t.Errorf("expected error on line 4, got line %d", res.Errors[0].LineNumber)
	}
	if res.Items[0].EstimatedJitaSell != 1_200_000 || res.Items[0].DailyVolume != 15_000 {
		t.Errorf("unexpected parsed item[0]: %+v", res.Items[0])
	}
	if res.Items[1].IsManufacturable {
		t.Errorf("expected Compressed Blue Ice to be marked non-manufacturable")
	}
}
