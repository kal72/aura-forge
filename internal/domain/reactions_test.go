package domain

import (
	"strings"
	"testing"
)

func TestEvaluateGasReaction_ProfitableAndBreakEven(t *testing.T) {
	formula := HybridPolymerFormulas[0] // C3-FTM Acid (100 out/run)
	matPrices := map[int32]JitaPrice{
		30374: {LowestSell: 10000}, // Fullerite-C84 x100 = 1,000,000
		30378: {LowestSell: 20000}, // Fullerite-C540 x100 = 2,000,000
		40:    {LowestSell: 2500},  // Megacyte x80 = 200,000
		4247:  {LowestSell: 25000}, // Helium Fuel Block x5 = 125,000
	}
	sellPrices := map[int32]JitaPrice{
		30303: {LowestSell: 45000}, // 100 out * 45,000 = 4,500,000/run
	}

	opp, ok := EvaluateGasReaction(formula, ReactionCalcParams{
		Runs:                0, // Auto ~24h
		Refinery:            LookupRefineryProfile("athanor_t1"),
		SellHub:             LookupTradeHub("amarr"),
		MaterialHub:         LookupTradeHub("jita"),
		MaterialPriceSource: "jita_sell",
		SystemCostIndex:     0.03,
		FacilityTaxPercent:  1.0,
		BrokerFeePercent:    3.0,
		SalesTaxPercent:     3.6,
		RelistFeePercent:    0.5,
	}, matPrices, sellPrices, 15000)

	if !ok {
		t.Fatal("expected valid reaction opportunity")
	}
	if opp.Runs <= 0 {
		t.Fatalf("expected auto runs > 0, got %d", opp.Runs)
	}
	if opp.NetProfit <= 0 {
		t.Fatalf("expected positive net profit, got %.2f", opp.NetProfit)
	}
	if opp.RecommendedSell >= 45000 || opp.RecommendedSell <= 0 {
		t.Fatalf("expected 1-tick undercut RecommendedSell < 45000, got %.2f", opp.RecommendedSell)
	}
	if opp.BreakEvenSell <= 0 || opp.BreakEvenSell >= opp.RecommendedSell {
		t.Fatalf("expected valid BreakEvenSell < RecommendedSell, got floor=%.2f rec=%.2f", opp.BreakEvenSell, opp.RecommendedSell)
	}
	if !strings.Contains(opp.MultibuyText, "Fullerite-C84") || !strings.Contains(opp.MultibuyText, "Helium Fuel Block") {
		t.Fatalf("expected multibuy text with inputs, got %q", opp.MultibuyText)
	}
}

func TestHybridPolymerFormulas_AllEightDefined(t *testing.T) {
	if len(HybridPolymerFormulas) != 8 {
		t.Fatalf("expected 8 Hybrid Polymer formulas, got %d", len(HybridPolymerFormulas))
	}
	for _, f := range HybridPolymerFormulas {
		if f.ProductTypeID <= 0 || f.OutputQtyPerRun <= 0 || len(f.Inputs) != 4 {
			t.Fatalf("invalid formula %+v", f)
		}
	}
}
