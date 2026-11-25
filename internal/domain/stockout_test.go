package domain_test

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestEvaluateStockoutOpportunity(t *testing.T) {
	amarr := domain.LookupTradeHub("amarr")

	// 1. Complete stockout (0 sellers at Amarr, active 80/day demand)
	opp, ok := domain.EvaluateStockoutOpportunity(
		2456, "Hobgoblin II", "Drones", amarr,
		domain.JitaPrice{SellOrdersCount: 0, LowestSell: 0},
		domain.JitaPrice{SellOrdersCount: 40, LowestSell: 700_000},
		80,
	)
	if !ok || opp.Signal != domain.SignalStockout || opp.Verdict != domain.VerdictStrong {
		t.Fatalf("expected STRONG STOCKOUT, got ok=%v opp=%+v", ok, opp)
	}

	// 2. Thin supply (2 sellers at Amarr, +18% over Jita)
	opp2, ok2 := domain.EvaluateStockoutOpportunity(
		9832, "Coolant", "PI & Fuel", amarr,
		domain.JitaPrice{SellOrdersCount: 2, LowestSell: 11_800},
		domain.JitaPrice{SellOrdersCount: 50, LowestSell: 10_000},
		400,
	)
	if !ok2 || opp2.Signal != domain.SignalThinSupply || opp2.PremiumOverJitaPct != 18.0 {
		t.Fatalf("expected THIN_SUPPLY +18%%, got ok=%v opp=%+v", ok2, opp2)
	}

	// 3. Dead/low-demand item (<5/day) must be ignored
	if _, ok3 := domain.EvaluateStockoutOpportunity(
		999, "Dead Item", "Other", amarr,
		domain.JitaPrice{SellOrdersCount: 0, LowestSell: 0},
		domain.JitaPrice{SellOrdersCount: 1, LowestSell: 1000},
		2,
	); ok3 {
		t.Fatalf("expected low-demand item to be filtered out")
	}
}
