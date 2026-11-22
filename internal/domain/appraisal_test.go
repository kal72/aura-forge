package domain_test

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestParseEVEClipboard_AllFormatsAndDedup(t *testing.T) {
	raw := `[Caracal, PvP Fit]
Tritanium	50,000	Mineral		500 m3	225,000.00 ISK
Tritanium	25,000	Mineral
10x Hobgoblin II
Damage Control II x2
Pyerite 1,500
[Empty Low slot]
Warp Disruptor II`

	items, _ := domain.ParseEVEClipboard(raw)
	got := make(map[string]int64, len(items))
	for _, it := range items {
		got[it.Name] = it.Quantity
	}

	if got["Caracal"] != 1 {
		t.Errorf("expected Caracal=1, got %d", got["Caracal"])
	}
	if got["Tritanium"] != 75000 {
		t.Errorf("expected deduplicated Tritanium=75000, got %d", got["Tritanium"])
	}
	if got["Hobgoblin II"] != 10 {
		t.Errorf("expected Hobgoblin II=10, got %d", got["Hobgoblin II"])
	}
	if got["Damage Control II"] != 2 {
		t.Errorf("expected Damage Control II=2, got %d", got["Damage Control II"])
	}
	if got["Pyerite"] != 1500 {
		t.Errorf("expected Pyerite=1500, got %d", got["Pyerite"])
	}
	if got["Warp Disruptor II"] != 1 {
		t.Errorf("expected Warp Disruptor II=1, got %d", got["Warp Disruptor II"])
	}
}

func TestBuildAppraisalResult_TotalsTaxesAndGankRisk(t *testing.T) {
	hub := domain.LookupTradeHub("amarr")
	types := []domain.SDEType{
		{TypeID: 44992, Name: "PLEX", Category: "Commodities", VolumeM3: 0.01},
		{TypeID: 34, Name: "Tritanium", Category: "Minerals", VolumeM3: 0.01},
	}
	qty := map[int32]int64{44992: 50, 34: 100000}
	prices := map[int32]domain.JitaPrice{
		44992: {LowestSell: 5_000_000, HighestBuy: 4_800_000},
		34:    {LowestSell: 5.0, HighestBuy: 4.5},
	}

	res := domain.BuildAppraisalResult(hub, types, qty, prices, nil, 3.0, 3.6, 90.0)
	if res.TotalSell != 250_500_000 {
		t.Fatalf("expected TotalSell 250,500,000, got %.2f", res.TotalSell)
	}
	if !res.GankRiskWarning {
		t.Errorf("expected GankRiskWarning=true for 250.5M ISK haul")
	}
	if res.NetSellAfterFees >= res.TotalSell || res.NetBuyAfterFees >= res.TotalBuy {
		t.Errorf("expected net values after tax to be less than gross: %+v", res)
	}
	if res.BuybackValue != domain.RoundISK(res.TotalBuy*0.90) {
		t.Errorf("expected 90%% buyback value, got %.2f vs %.2f", res.BuybackValue, res.TotalBuy*0.90)
	}
}
