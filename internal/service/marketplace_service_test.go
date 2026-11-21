package service

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestMarketplaceV2_7dSignalAndBestArbitrage(t *testing.T) {
	// 1. Price signal detection (SPIKE vs CHEAP vs FAIR)
	hist := []domain.MarketHistoryPoint{
		{Date: "2026-11-15", Average: 100_000, Lowest: 95_000, Highest: 105_000, Volume: 500},
		{Date: "2026-11-16", Average: 100_000, Lowest: 96_000, Highest: 104_000, Volume: 600},
	}

	snapSpike := &domain.MarketplaceSnapshot{LowestSell: 130_000} // +30% vs 100k avg
	populateMarketplace7dSignal(snapSpike, hist)
	if snapSpike.PriceSignal != "SPIKE" || snapSpike.Avg7dPrice != 100_000 || snapSpike.Low7dPrice != 95_000 || snapSpike.High7dPrice != 105_000 {
		t.Fatalf("expected SPIKE with 7d range 95k/100k/105k, got %+v", snapSpike)
	}

	snapCheap := &domain.MarketplaceSnapshot{LowestSell: 90_000} // -10% vs 100k avg
	populateMarketplace7dSignal(snapCheap, hist)
	if snapCheap.PriceSignal != "CHEAP" {
		t.Fatalf("expected CHEAP signal, got %s", snapCheap.PriceSignal)
	}

	// 2. Best 5-Hub Arbitrage detection (Jita 800k -> Amarr 1.1M)
	jita := domain.LookupTradeHub("jita")
	amarr := domain.LookupTradeHub("amarr")
	dodixie := domain.LookupTradeHub("dodixie")
	snapArb := &domain.MarketplaceSnapshot{
		HubComparisons: []domain.HubPriceSummary{
			{Hub: jita, LowestSell: 800_000},
			{Hub: amarr, LowestSell: 1_100_000},
			{Hub: dodixie, LowestSell: 950_000},
		},
	}
	populateBestMarketplaceArbitrage(snapArb, domain.DefaultUserSettings())
	if snapArb.BestArbOriginHub.Key != "jita" || snapArb.BestArbDestHub.Key != "amarr" {
		t.Fatalf("expected best arbitrage Jita -> Amarr, got %s -> %s", snapArb.BestArbOriginHub.Key, snapArb.BestArbDestHub.Key)
	}
	if snapArb.BestArbNetMarginPct <= 15.0 {
		t.Errorf("expected strong positive arbitrage net margin (>15%%), got %.2f%%", snapArb.BestArbNetMarginPct)
	}
}
