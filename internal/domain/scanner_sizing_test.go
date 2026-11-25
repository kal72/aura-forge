package domain

import "testing"

func TestRecommendManufacturingMETE(t *testing.T) {
	// T1 default -> 10/20
	me, te, owned, auto := RecommendManufacturingMETE(1, nil, -1, -1)
	if me != 10 || te != 20 || owned || !auto {
		t.Fatalf("T1 default got me=%d te=%d owned=%v auto=%v", me, te, owned, auto)
	}

	// T2 default -> 2/4
	me, te, owned, auto = RecommendManufacturingMETE(2, nil, -1, -1)
	if me != 2 || te != 4 || owned || !auto {
		t.Fatalf("T2 default got me=%d te=%d owned=%v auto=%v", me, te, owned, auto)
	}

	// Owned BPO overrides standard in auto mode
	bp := &CharacterBlueprint{TypeID: 683, MaterialEfficiency: 8, TimeEfficiency: 14}
	me, te, owned, auto = RecommendManufacturingMETE(1, bp, -1, -1)
	if me != 8 || te != 14 || !owned || !auto {
		t.Fatalf("Owned BPO got me=%d te=%d owned=%v auto=%v", me, te, owned, auto)
	}

	// Explicit user override disables auto
	me, te, owned, auto = RecommendManufacturingMETE(1, bp, 5, 10)
	if me != 5 || te != 10 || owned || auto {
		t.Fatalf("Manual override got me=%d te=%d owned=%v auto=%v", me, te, owned, auto)
	}
}

func TestRecommendManufacturingRuns(t *testing.T) {
	// Manual override returns Custom
	runs, reason, auto := RecommendManufacturingRuns(3600, 1_000_000, 1, 500, 100_000_000, 25)
	if runs != 25 || reason != "Custom" || auto {
		t.Fatalf("Manual override got runs=%d reason=%q auto=%v", runs, reason, auto)
	}

	// Bounded by 24h slot (3600s/run -> 24 runs) when capital and volume are huge
	runs, reason, auto = RecommendManufacturingRuns(3600, 100_000, 1, 5000, 1_000_000_000, 0)
	if runs != 24 || reason != "24h slot" || !auto {
		t.Fatalf("24h cap got runs=%d reason=%q auto=%v", runs, reason, auto)
	}

	// Bounded by 25% capital (100M * 0.25 = 25M / 5M = 5 runs)
	runs, reason, auto = RecommendManufacturingRuns(1800, 5_000_000, 1, 5000, 100_000_000, 0)
	if runs != 5 || reason != "25% capital" || !auto {
		t.Fatalf("Capital cap got runs=%d reason=%q auto=%v", runs, reason, auto)
	}

	// Bounded by 20% daily volume (50/day * 0.20 = 10 units / 1 unit per run = 10 runs)
	runs, reason, auto = RecommendManufacturingRuns(600, 100_000, 1, 50, 1_000_000_000, 0)
	if runs != 10 || reason != "20% daily vol" || !auto {
		t.Fatalf("Volume cap got runs=%d reason=%q auto=%v", runs, reason, auto)
	}

	// Zero volume falls back to safe default cap of 10
	runs, reason, auto = RecommendManufacturingRuns(300, 10_000, 1, 0, 1_000_000_000, 0)
	if runs != 10 || reason != "Safe default" || !auto {
		t.Fatalf("Safe default got runs=%d reason=%q auto=%v", runs, reason, auto)
	}
}
