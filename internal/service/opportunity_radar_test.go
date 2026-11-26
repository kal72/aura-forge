package service

import (
	"testing"

	"aura-forge/internal/domain"
)

func TestRankRadarOpportunities_FiltersAndPrioritizes(t *testing.T) {
	input := []domain.ManufacturingOpportunity{
		{ProductName: "Loss Trap", NetProfit: -50000, Verdict: domain.VerdictAvoid, FitsCapital: true, DailyVolume7d: 500, ISKPerHour: -10000},
		{ProductName: "Dead Illiquid", NetProfit: 900000, Verdict: domain.VerdictStrong, FitsCapital: true, DailyVolume7d: 2, ISKPerHour: 900000},
		{ProductName: "Over Cap Strong", NetProfit: 5000000, Verdict: domain.VerdictStrong, FitsCapital: false, DailyVolume7d: 100, ISKPerHour: 2000000},
		{ProductName: "Fit Moderate", NetProfit: 300000, Verdict: domain.VerdictModerate, FitsCapital: true, DailyVolume7d: 200, ISKPerHour: 350000},
		{ProductName: "Fit Strong Top", NetProfit: 600000, Verdict: domain.VerdictStrong, FitsCapital: true, DailyVolume7d: 150, ISKPerHour: 800000},
	}

	got := rankRadarOpportunities(input, 8)
	if len(got) != 3 {
		t.Fatalf("expected 3 filtered opportunities (excluding loss & dead illiquid), got %d", len(got))
	}
	if got[0].ProductName != "Fit Strong Top" {
		t.Fatalf("expected Fit Strong Top first, got %q", got[0].ProductName)
	}
	if got[1].ProductName != "Fit Moderate" {
		t.Fatalf("expected Fit Moderate second, got %q", got[1].ProductName)
	}
	if got[2].ProductName != "Over Cap Strong" {
		t.Fatalf("expected Over Cap Strong third, got %q", got[2].ProductName)
	}
}
