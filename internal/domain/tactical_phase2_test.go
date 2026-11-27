package domain

import "testing"

func TestSimulateDuelTimeline_VictoryAndDefeat(t *testing.T) {
	strongSim := &FitSimulationResult{
		TotalEHP:          45000,
		SustainedTankEHPs: 220,
		Capacitor:         CapacitorStats{CapacityGJ: 2200, PeakRechargeGJs: 40, ActiveDrainGJs: 18, Stable: true},
		Offense:           OffenseStats{TurretDPS: 480, TotalDPS: 480, OptimalRangeKM: 20, FalloffRangeKM: 10},
		Navigation:        NavigationStats{MaxVelocityMS: 650, SignatureRadiusM: 120},
	}
	res := SimulateDuelTimeline(strongSim, DuelSimRequest{
		OpponentKey: "l3_boss",
		RangeKM:     18,
	})
	if res.Verdict != "VICTORY" || res.TimeToKillSec <= 0 {
		t.Errorf("expected VICTORY vs L3 boss, got %s (TTK=%d, TTD=%d)", res.Verdict, res.TimeToKillSec, res.TimeToDieSec)
	}

	weakSim := &FitSimulationResult{
		TotalEHP:          4200,
		SustainedTankEHPs: 10,
		Capacitor:         CapacitorStats{CapacityGJ: 400, PeakRechargeGJs: 8, ActiveDrainGJs: 15, Stable: false},
		Offense:           OffenseStats{TurretDPS: 60, TotalDPS: 60, OptimalRangeKM: 10, FalloffRangeKM: 5},
		Navigation:        NavigationStats{MaxVelocityMS: 200, SignatureRadiusM: 180},
	}
	gankRes := SimulateDuelTimeline(weakSim, DuelSimRequest{
		OpponentKey: "catalyst_gank",
		RangeKM:     4,
		EnemyScram:  true,
		EnemyWeb:    true,
	})
	if gankRes.Verdict != "DEFEAT" || gankRes.TimeToDieSec <= 0 {
		t.Errorf("expected DEFEAT vs Catalyst Gank Squad, got %s (TTD=%d)", gankRes.Verdict, gankRes.TimeToDieSec)
	}
}

func TestAnalyzeDScan_GankAlertAndGatecamp(t *testing.T) {
	gankDump := `16240	Uedama Ganker 1	Catalyst	0.4 AU
16240	Uedama Ganker 2	Catalyst	0.4 AU
16240	Uedama Ganker 3	Catalyst	0.4 AU
17918	Combat Scanner Probe	Combat Scanner Probe	1,200 km`

	res, err := AnalyzeDScan(gankDump)
	if err != nil {
		t.Fatalf("AnalyzeDScan failed: %v", err)
	}
	if res.ThreatLevel != "GANK ALERT" || res.GankerCount != 3 || !res.HasProbes {
		t.Errorf("expected GANK ALERT with 3 gankers & probes, got %+v", res)
	}
}

func TestAnalyzeCombatLog_HitQualityAndTopThreat(t *testing.T) {
	logDump := `[ 2026.11.27 14:22:01 ] (combat) 340 from Sansha's Centurion - Penetrates
[ 2026.11.27 14:22:03 ] (combat) 290 from Sansha's Centurion - Hits
[ 2026.11.27 14:22:05 ] (combat) 410 to Sansha's Centurion - Smashes
[ 2026.11.27 14:22:07 ] (combat) 85 from Sansha's Frigate - Glances Off
[ 2026.11.27 14:22:09 ] (combat) Energy Neutralizer drained 60 GJ`

	res, err := AnalyzeCombatLog(logDump)
	if err != nil {
		t.Fatalf("AnalyzeCombatLog failed: %v", err)
	}
	if res.IncomingDamage != 715 || res.OutgoingDamage != 410 {
		t.Errorf("unexpected damage totals: in=%.0f out=%.0f", res.IncomingDamage, res.OutgoingDamage)
	}
	if len(res.TopThreats) == 0 || res.TopThreats[0].SourceName != "Sansha's Centurion" {
		t.Errorf("expected Sansha's Centurion as top threat, got %+v", res.TopThreats)
	}
	if len(res.EWARTags) == 0 {
		t.Errorf("expected Capacitor Neutralizer EWAR tag")
	}
}
