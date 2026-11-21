package domain_test

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestComputeInventionSuccessChance(t *testing.T) {
	// Base 34%, skills 4/4/4, Accelerant (1.2x):
	// 0.34 * (1 + 4/40 + 8/30) * 1.2 = 0.34 * 1.366666... * 1.2 = 0.5576
	got := domain.ComputeInventionSuccessChance(0.34, 4, 4, 4, 1.2)
	if got < 0.55 || got > 0.565 {
		t.Fatalf("expected ~0.5576 invention chance, got %.4f", got)
	}
}

func TestOptimizeT2Invention_RanksAll9Decryptors(t *testing.T) {
	preset := domain.LookupT2Preset("hobgoblin_ii")
	prices := map[int32]domain.JitaPrice{
		preset.ProductTypeID:   {LowestSell: 850_000},
		preset.Datacore1TypeID: {LowestSell: 80_000},
		preset.Datacore2TypeID: {LowestSell: 75_000},
		34201:                  {LowestSell: 1_200_000},
		34202:                  {LowestSell: 1_800_000},
		34203:                  {LowestSell: 4_500_000},
		34204:                  {LowestSell: 1_500_000},
		34205:                  {LowestSell: 600_000},
		34206:                  {LowestSell: 900_000},
		34207:                  {LowestSell: 2_500_000},
		34208:                  {LowestSell: 6_000_000},
	}

	res := domain.OptimizeT2Invention(preset, domain.InventionInput{
		PresetKey:           "hobgoblin_ii",
		EncryptionSkill:     4,
		Datacore1Skill:      4,
		Datacore2Skill:      4,
		BaseBuildCostPerRun: 480_000,
		BrokerFeePercent:    3.0,
		SalesTaxPercent:     3.6,
	}, prices)

	if len(res.Scenarios) != 9 {
		t.Fatalf("expected 9 decryptor scenarios, got %d", len(res.Scenarios))
	}
	if !res.Scenarios[0].IsBest {
		t.Fatalf("expected top scenario to be flagged IsBest")
	}
	if res.Scenarios[0].NetProfitPerUnit < res.Scenarios[8].NetProfitPerUnit {
		t.Fatalf("expected scenarios to be sorted descending by NetProfitPerUnit")
	}
}

func TestEvaluateMakeVsBuy(t *testing.T) {
	makeRes := domain.EvaluateMakeVsBuy("Construction Blocks", 100, 12_000, 9_500, 0.045, 0.25)
	if makeRes.Recommendation != "MAKE COMPONENT" || makeRes.SavingsISK <= 0 {
		t.Fatalf("expected MAKE COMPONENT with positive savings, got %+v", makeRes)
	}

	buyRes := domain.EvaluateMakeVsBuy("Nanotransistors", 100, 8_000, 8_200, 0.045, 0.25)
	if buyRes.Recommendation != "BUY IN JITA" || buyRes.SavingsISK >= 0 {
		t.Fatalf("expected BUY IN JITA, got %+v", buyRes)
	}
}
