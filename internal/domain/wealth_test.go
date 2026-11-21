package domain_test

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestResolveCapitalPhase_AndMilestones(t *testing.T) {
	p1 := domain.ResolveCapitalPhase(50_000_000)
	if p1.TierNumber != 1 {
		t.Fatalf("expected Tier 1 for 50M ISK, got Tier %d", p1.TierNumber)
	}

	p2 := domain.ResolveCapitalPhase(500_000_000)
	if p2.TierNumber != 2 {
		t.Fatalf("expected Tier 2 for 500M ISK, got Tier %d", p2.TierNumber)
	}

	p5 := domain.ResolveCapitalPhase(120_000_000_000)
	if p5.TierNumber != 5 {
		t.Fatalf("expected Tier 5 for 120B ISK, got Tier %d", p5.TierNumber)
	}

	milestones, next := domain.BuildWealthMilestones(250_000_000)
	if len(milestones) != 5 {
		t.Fatalf("expected 5 milestones, got %d", len(milestones))
	}
	if !milestones[0].Reached || next.Label != "1B ISK" || next.ProgressPct != 25.0 {
		t.Fatalf("unexpected milestone state: first=%+v next=%+v", milestones[0], next)
	}
}

func TestAggregateTopProfitItems(t *testing.T) {
	batches := []domain.BatchRecord{
		{ItemName: "Hobgoblin I", Status: domain.BatchStatusSold, UnitsOutput: 100, ActualProductionCost: 1_000_000, NetProfit: 300_000},
		{ItemName: "Hobgoblin I", Status: domain.BatchStatusSold, UnitsOutput: 100, ActualProductionCost: 1_000_000, NetProfit: 200_000},
		{ItemName: "Rifter", Status: domain.BatchStatusSold, UnitsOutput: 10, ActualProductionCost: 5_000_000, NetProfit: 1_200_000},
		{ItemName: "Venture", Status: domain.BatchStatusListed, UnitsOutput: 5, ActualProductionCost: 2_000_000, NetProfit: 500_000},
	}

	top := domain.AggregateTopProfitItems(batches, 5)
	if len(top) != 2 {
		t.Fatalf("expected 2 sold items, got %d", len(top))
	}
	if top[0].ItemName != "Rifter" || top[0].TotalProfit != 1_200_000 {
		t.Fatalf("expected Rifter first with 1.2M profit, got %+v", top[0])
	}
	if top[1].ItemName != "Hobgoblin I" || top[1].BatchesSold != 2 || top[1].TotalProfit != 500_000 {
		t.Fatalf("expected Hobgoblin I second with 500k profit, got %+v", top[1])
	}
}
