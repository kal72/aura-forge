package domain

import "testing"

func TestEvaluateOrderUndercut_SellOrder(t *testing.T) {
	// Top sell order
	top := EvaluateOrderUndercut(
		CharacterOrder{OrderID: 1, IsBuyOrder: false, Price: 100_000},
		JitaPrice{LowestSell: 100_000},
	)
	if top.Status != OrderStatusTop {
		t.Fatalf("expected TOP, got %s", top.Status)
	}

	// Undercut sell order
	uc := EvaluateOrderUndercut(
		CharacterOrder{OrderID: 2, IsBuyOrder: false, Price: 105_000},
		JitaPrice{LowestSell: 100_000},
	)
	if uc.Status != OrderStatusUndercut {
		t.Fatalf("expected UNDERCUT, got %s", uc.Status)
	}
	if uc.GapISK != 5000 {
		t.Fatalf("expected GapISK 5000, got %.2f", uc.GapISK)
	}
	if uc.RecommendedPrice >= 100_000 || uc.RecommendedPrice <= 0 {
		t.Fatalf("expected RecommendedPrice < 100000, got %.2f", uc.RecommendedPrice)
	}
}

func TestEvaluateOrderUndercut_BuyOrder(t *testing.T) {
	// Top buy order
	top := EvaluateOrderUndercut(
		CharacterOrder{OrderID: 3, IsBuyOrder: true, Price: 50_000},
		JitaPrice{HighestBuy: 50_000},
	)
	if top.Status != OrderStatusTop {
		t.Fatalf("expected TOP, got %s", top.Status)
	}

	// Outbid buy order
	uc := EvaluateOrderUndercut(
		CharacterOrder{OrderID: 4, IsBuyOrder: true, Price: 48_000},
		JitaPrice{HighestBuy: 50_000},
	)
	if uc.Status != OrderStatusUndercut {
		t.Fatalf("expected UNDERCUT, got %s", uc.Status)
	}
	if uc.RecommendedPrice <= 50_000 {
		t.Fatalf("expected RecommendedPrice > 50000, got %.2f", uc.RecommendedPrice)
	}
}

func TestLookupTradeHubByRegion(t *testing.T) {
	fallback := LookupTradeHub("jita")
	amarr := LookupTradeHubByRegion(10000043, fallback)
	if amarr.Key != "amarr" {
		t.Fatalf("expected amarr for Domain region, got %s", amarr.Key)
	}
	unknown := LookupTradeHubByRegion(99999999, fallback)
	if unknown.Key != "jita" {
		t.Fatalf("expected fallback jita, got %s", unknown.Key)
	}
}
