package domain_test

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestEvaluateStationTrade_DeductsBothBrokerFeesAndSalesTax(t *testing.T) {
	// Buy 100,000 ISK, Sell 120,000 ISK, Broker 3%, Sales Tax 3.6%
	// Buy total = 103,000; Sell net = 120,000 * (1 - 0.066) = 112,080 -> Net = +9,080 ISK (~7.57%)
	opp := domain.EvaluateStationTrade(2454, "Hobgoblin I", "Drones", domain.JitaPrice{
		TypeID:     2454,
		HighestBuy: 100_000,
		LowestSell: 120_000,
	}, 3.0, 3.6)

	if opp.NetFlipProfitUnit <= 0 || opp.Verdict != domain.VerdictModerate {
		t.Fatalf("expected profitable MODERATE flip, got profit=%.2f margin=%.2f verdict=%s", opp.NetFlipProfitUnit, opp.NetFlipMarginPct, opp.Verdict)
	}

	// Tight 4% spread should become AVOID after 9.6% combined round-trip fees
	tight := domain.EvaluateStationTrade(34, "Tritanium", "Minerals", domain.JitaPrice{
		TypeID:     34,
		HighestBuy: 96,
		LowestSell: 100,
	}, 3.0, 3.6)
	if tight.NetFlipProfitUnit >= 0 || tight.Verdict != domain.VerdictAvoid {
		t.Fatalf("expected tight spread to be AVOID, got profit=%.2f verdict=%s", tight.NetFlipProfitUnit, tight.Verdict)
	}
}

func TestEvaluateArbitrage_IncludesFreightAndTaxes(t *testing.T) {
	jita := domain.LookupTradeHub("jita")
	amarr := domain.LookupTradeHub("amarr")

	// Buy in Jita at 500k, Sell in Amarr at 650k, 5 m3 * 300 ISK/m3 = 1,500 ISK freight
	opp := domain.EvaluateArbitrage(2046, "Damage Control I", "Modules", jita, amarr, 500_000, 650_000, 520_000, 5.0, 300.0, 3.0, 3.6)
	if opp.HaulCostPerUnit != 1500 {
		t.Fatalf("expected freight 1500, got %.2f", opp.HaulCostPerUnit)
	}
	if opp.NetProfitUnit <= 0 || opp.Verdict != domain.VerdictStrong {
		t.Fatalf("expected STRONG arbitrage, got profit=%.2f margin=%.2f verdict=%s", opp.NetProfitUnit, opp.NetMarginPct, opp.Verdict)
	}
}
