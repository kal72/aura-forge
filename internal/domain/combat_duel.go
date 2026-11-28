package domain

import (
	"fmt"
	"math"
	"strings"
)

// DuelOpponentPreset defines an enemy PvP or PvE boss profile for 1v1 second-by-second timeline simulation.
type DuelOpponentPreset struct {
	Key            string  `json:"key"`
	Name           string  `json:"name"`
	Role           string  `json:"role"` // Brawler | Kiter | Ganker | PvE Boss | Abyssal
	ShipTypeID     int32   `json:"ship_type_id"`
	TotalEHP       float64 `json:"total_ehp"`
	DPS            float64 `json:"dps"`
	ActiveTankEHPs float64 `json:"active_tank_ehps"`
	OptimalKM      float64 `json:"optimal_km"`
	FalloffKM      float64 `json:"falloff_km"`
	SpeedMS        float64 `json:"speed_ms"`
	SigRadiusM     float64 `json:"sig_radius_m"`
	NeutGJs        float64 `json:"neut_gjs"`
	Summary        string  `json:"summary"`
}

// BuiltInDuelOpponents returns curated PvE boss & PvP enemy archetypes for 1v1 simulation.
func BuiltInDuelOpponents() []DuelOpponentPreset {
	return []DuelOpponentPreset{
		{
			Key: "l3_boss", Name: "L3 Pirate Cruiser Boss", Role: "PvE Boss", ShipTypeID: 17715,
			TotalEHP: 24000, DPS: 210, ActiveTankEHPs: 45, OptimalKM: 22, FalloffKM: 12,
			SpeedMS: 220, SigRadiusM: 145, NeutGJs: 0,
			Summary: "Sustained Level 3 mission wave pressure with moderate active tank.",
		},
		{
			Key: "catalyst_gank", Name: "High-Sec Catalyst Gank Squad (×3)", Role: "Ganker", ShipTypeID: 16240,
			TotalEHP: 6500, DPS: 1350, ActiveTankEHPs: 0, OptimalKM: 5, FalloffKM: 4,
			SpeedMS: 380, SigRadiusM: 68, NeutGJs: 0,
			Summary: "Extreme close-range Void blaster burst before CONCORD arrives (~18s in 0.6).",
		},
		{
			Key: "pvp_brawler", Name: "Faction Warfare Brawler Cruiser", Role: "Brawler", ShipTypeID: 626,
			TotalEHP: 32000, DPS: 420, ActiveTankEHPs: 115, OptimalKM: 9, FalloffKM: 6,
			SpeedMS: 520, SigRadiusM: 130, NeutGJs: 14,
			Summary: "Close-range scram-web brawler with medium energy neutralizer pressure.",
		},
		{
			Key: "pvp_kiter", Name: "Nano Kiter Cruiser (24 km)", Role: "Kiter", ShipTypeID: 2006,
			TotalEHP: 19500, DPS: 340, ActiveTankEHPs: 30, OptimalKM: 26, FalloffKM: 10,
			SpeedMS: 1450, SigRadiusM: 115, NeutGJs: 0,
			Summary: "High-transversal point kiter operating at 24 km edge of Warp Disruptor.",
		},
		{
			Key: "abyssal_vedmak", Name: "Abyssal T3 Triglavian Vedmak", Role: "Abyssal", ShipTypeID: 47270,
			TotalEHP: 28000, DPS: 310, ActiveTankEHPs: 95, OptimalKM: 18, FalloffKM: 8,
			SpeedMS: 440, SigRadiusM: 120, NeutGJs: 18,
			Summary: "Ramping disintegrator damage (+2%/sec) paired with energy neutralizer.",
		},
	}
}

// LookupDuelOpponent returns the requested opponent archetype or the default L3 Boss.
func LookupDuelOpponent(key string) DuelOpponentPreset {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, op := range BuiltInDuelOpponents() {
		if op.Key == k {
			return op
		}
	}
	return BuiltInDuelOpponents()[0]
}

// DuelSimRequest specifies the parameters for a second-by-second 1v1 combat timeline simulation.
type DuelSimRequest struct {
	Fit         ShipFit `json:"fit"`
	OpponentKey string  `json:"opponent_key"`
	RangeKM     float64 `json:"range_km"`
	EnemyWeb    bool    `json:"enemy_web"`
	EnemyScram  bool    `json:"enemy_scram"`
	EnemyNeut   bool    `json:"enemy_neut"`
}

// DuelTimelinePoint captures both ships' exact health & capacitor percentages at second t.
type DuelTimelinePoint struct {
	Second        int     `json:"second"`
	PilotHP       float64 `json:"pilot_hp"`
	PilotHPPct    float64 `json:"pilot_hp_pct"`
	PilotCapPct   float64 `json:"pilot_cap_pct"`
	EnemyHP       float64 `json:"enemy_hp"`
	EnemyHPPct    float64 `json:"enemy_hp_pct"`
	PilotApplied  float64 `json:"pilot_applied_dps"`
	EnemyApplied  float64 `json:"enemy_applied_dps"`
}

// DuelSimulationResult is the complete second-by-second combat outcome for visual timeline scrubbing.
type DuelSimulationResult struct {
	Opponent          DuelOpponentPreset  `json:"opponent"`
	RangeKM           float64             `json:"range_km"`
	Verdict           string              `json:"verdict"` // VICTORY | DEFEAT | STALEMATE
	VerdictDetail     string              `json:"verdict_detail"`
	TimeToKillSec     int                 `json:"time_to_kill_sec"`     // 0 if enemy survived
	TimeToDieSec      int                 `json:"time_to_die_sec"`      // 0 if pilot survived
	CapBurnoutSec     int                 `json:"cap_burnout_sec"`      // 0 if cap stayed alive
	PilotAppliedDPS   float64             `json:"pilot_applied_dps"`
	EnemyAppliedDPS   float64             `json:"enemy_applied_dps"`
	DurationSec       int                 `json:"duration_sec"`
	Points            []DuelTimelinePoint `json:"points"`
}

// SimulateDuelTimeline runs a deterministic 1-second-resolution combat simulation (up to 180s).
func SimulateDuelTimeline(sim *FitSimulationResult, req DuelSimRequest) *DuelSimulationResult {
	op := LookupDuelOpponent(req.OpponentKey)
	distKM := req.RangeKM
	if distKM <= 0 {
		distKM = op.OptimalKM
	}

	pilotMaxHP := math.Max(sim.TotalEHP, 1000)
	pilotHP := pilotMaxHP
	pilotMaxCap := math.Max(sim.Capacitor.CapacityGJ, 100)
	pilotCap := pilotMaxCap
	enemyMaxHP := math.Max(op.TotalEHP, 1000)
	enemyHP := enemyMaxHP

	// Compute applied DPS at engagement distance
	pilotApplied := computeAppliedDPSAtRange(sim.Offense, distKM, op.SpeedMS, op.SigRadiusM, req.EnemyWeb)
	enemyRangeFactor := 1.0
	if distKM > op.OptimalKM {
		r := (distKM - op.OptimalKM) / math.Max(op.FalloffKM, 1.0)
		enemyRangeFactor = math.Pow(0.5, r*r)
	}
	pilotSpeed := sim.Navigation.MaxVelocityMS
	if req.EnemyWeb {
		pilotSpeed *= 0.40
	}
	if req.EnemyScram {
		pilotSpeed *= 0.65
	}
	enemyTransPenalty := 1.0
	if distKM > 0 && pilotSpeed > 250 {
		ang := pilotSpeed / (distKM * 1000.0)
		enemyTransPenalty = math.Max(0.35, 1.0-math.Min(0.60, ang*2.5*(125.0/math.Max(sim.Navigation.SignatureRadiusM, 30))))
	}
	enemyBaseApplied := op.DPS * enemyRangeFactor * enemyTransPenalty

	// Capacitor dynamics per second
	capDrain := sim.Capacitor.ActiveDrainGJs
	if req.EnemyNeut {
		neut := op.NeutGJs
		if neut <= 0 {
			neut = 16.0
		}
		capDrain += neut
	}
	capPeak := sim.Capacitor.PeakRechargeGJs

	var (
		ttkSec        int
		ttdSec        int
		capBurnoutSec int
		points        = make([]DuelTimelinePoint, 0, 95)
	)

	maxSec := 180
	for t := 0; t <= maxSec; t++ {
		// Triglavian ramp (+1.8% per sec up to 2.5x)
		rampMult := 1.0
		if op.Key == "abyssal_vedmak" {
			rampMult = math.Min(2.5, 1.0+float64(t)*0.018)
		}
		enemyCurDPS := enemyBaseApplied * rampMult

		// Record every 2 seconds (plus t=0 and final tick) for crisp smooth chart curves
		if t%2 == 0 || pilotHP <= 0 || enemyHP <= 0 || t == maxSec {
			points = append(points, DuelTimelinePoint{
				Second:       t,
				PilotHP:      math.Round(math.Max(0, pilotHP)),
				PilotHPPct:   math.Round(math.Max(0, pilotHP/pilotMaxHP*100)*10) / 10,
				PilotCapPct:  math.Round(math.Max(0, pilotCap/pilotMaxCap*100)*10) / 10,
				EnemyHP:      math.Round(math.Max(0, enemyHP)),
				EnemyHPPct:   math.Round(math.Max(0, enemyHP/enemyMaxHP*100)*10) / 10,
				PilotApplied: math.Round(pilotApplied*10) / 10,
				EnemyApplied: math.Round(enemyCurDPS*10) / 10,
			})
		}

		if enemyHP <= 0 {
			ttkSec = t
			break
		}
		if pilotHP <= 0 {
			ttdSec = t
			break
		}

		// Capacitor recharge parabola: R(x) = 4 * peak * sqrt(x) * (1 - sqrt(x))
		capFrac := math.Max(0.01, math.Min(1.0, pilotCap/pilotMaxCap))
		capRegen := 4.0 * capPeak * math.Sqrt(capFrac) * (1.0 - math.Sqrt(capFrac))
		pilotCap = math.Min(pilotMaxCap, math.Max(0, pilotCap+capRegen-capDrain))

		activeTank := sim.SustainedTankEHPs
		if pilotCap <= pilotMaxCap*0.04 {
			if capBurnoutSec == 0 && capDrain > 0 {
				capBurnoutSec = t
			}
			activeTank *= 0.15 // Only passive shield regen remains when capped out
		}

		// Apply 1 second of combat exchange
		pilotHP = math.Min(pilotMaxHP, pilotHP+activeTank-enemyCurDPS)
		enemyHP = math.Min(enemyMaxHP, enemyHP+op.ActiveTankEHPs-pilotApplied)
	}

	verdict := "STALEMATE"
	detail := "Neither ship broke the other's tank within 180 seconds. Adjust engagement distance, ammo, or overheat."
	switch {
	case ttkSec > 0 && (ttdSec == 0 || ttkSec < ttdSec):
		verdict = "VICTORY"
		detail = fmt.Sprintf("Target destroyed in %ds with %.0f%% hull/EHP remaining.", ttkSec, math.Max(0, pilotHP/pilotMaxHP*100))
	case ttdSec > 0:
		verdict = "DEFEAT"
		detail = fmt.Sprintf("Your ship was destroyed at %ds (enemy left at %.0f%% HP). Increase tank or pull range.", ttdSec, math.Max(0, enemyHP/enemyMaxHP*100))
	}

	dur := maxSec
	if len(points) > 0 {
		dur = points[len(points)-1].Second
	}

	return &DuelSimulationResult{
		Opponent:        op,
		RangeKM:         distKM,
		Verdict:         verdict,
		VerdictDetail:   detail,
		TimeToKillSec:   ttkSec,
		TimeToDieSec:    ttdSec,
		CapBurnoutSec:   capBurnoutSec,
		PilotAppliedDPS: math.Round(pilotApplied*10) / 10,
		EnemyAppliedDPS: math.Round(enemyBaseApplied*10) / 10,
		DurationSec:     dur,
		Points:          points,
	}
}

func computeAppliedDPSAtRange(off OffenseStats, distKM, targetSpeedMS, targetSigM float64, webOnEnemy bool) float64 {
	opt := math.Max(off.OptimalRangeKM, 12)
	fall := math.Max(off.FalloffRangeKM, 8)
	rangeFactor := 1.0
	if distKM > opt {
		r := (distKM - opt) / fall
		rangeFactor = math.Pow(0.5, r*r)
	}
	spd := targetSpeedMS
	if webOnEnemy && distKM <= 12 {
		spd *= 0.45
	}
	ang := spd / math.Max(distKM*1000.0, 1000.0)
	trackPenalty := math.Max(0.20, 1.0-math.Min(0.72, ang*3.0*(125.0/math.Max(targetSigM, 25))))
	tDPS := off.TurretDPS * rangeFactor * trackPenalty
	mDPS := 0.0
	if distKM <= math.Max(opt, 34) {
		mDPS = off.MissileDPS * math.Min(1.0, targetSigM/95.0)
	}
	dDPS := 0.0
	if distKM <= 55 {
		dDPS = off.DroneDPS
	}
	return tDPS + mDPS + dDPS
}
