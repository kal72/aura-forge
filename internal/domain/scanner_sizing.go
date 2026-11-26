package domain

import "math"

// RecommendManufacturingMETE resolves the effective ME and TE for a blueprint scan.
// Passing negative override values (< 0) enables Auto mode:
//   - If the pilot owns a matching blueprint in hangar, its real ME/TE is used.
//   - Otherwise, EVE industry standards are applied (T1: 10/20, T2: 2/4, T3: 0/0).
func RecommendManufacturingMETE(techLevel int, owned *CharacterBlueprint, overrideME, overrideTE int) (me, te int, ownedMatched bool, autoMETE bool) {
	switch {
	case techLevel == 2:
		me, te = 2, 4
	case techLevel >= 3:
		me, te = 0, 0
	default:
		me, te = 10, 20
	}

	autoMETE = true
	if owned != nil && overrideME < 0 && overrideTE < 0 {
		me = clampInt(owned.MaterialEfficiency, 0, 10)
		te = clampInt(owned.TimeEfficiency, 0, 20)
		ownedMatched = true
	}

	if overrideME >= 0 {
		me = clampInt(overrideME, 0, 10)
		autoMETE = false
	}
	if overrideTE >= 0 {
		te = clampInt(overrideTE, 0, 20)
		autoMETE = false
	}
	return me, te, ownedMatched, autoMETE
}

// RecommendManufacturingRuns calculates a beginner-safe recommended run count for a blueprint
// when overrideRuns <= 0, bounded by:
//  1. 24-hour slot cycle (prevents locking beginner slots for days)
//  2. 25% of available capital per job (protects against over-concentration)
//  3. 20% of 7-day average daily hub volume (prevents flooding thin markets)
func RecommendManufacturingRuns(
	singleRunDurationSec float64,
	singleRunCost float64,
	outputPerRun int,
	dailyVolume7d float64,
	availableCapital float64,
	overrideRuns int,
) (runs int, reason string, autoSized bool) {
	if overrideRuns > 0 {
		return clampInt(overrideRuns, 1, 1000), "Custom", false
	}

	const (
		maxAutoRuns       = 500
		targetSlotSeconds = 86400.0 // 24 hours
		capitalShareCap   = 0.25    // max 25% of available capital per single job
		dailyVolShareCap  = 0.20    // max 20% of 7d daily hub volume
	)

	best := 10
	reason = "24h slot"
	if singleRunDurationSec > 0 {
		byTime := int(math.Floor(targetSlotSeconds / singleRunDurationSec))
		if byTime < 1 {
			byTime = 1
		}
		if byTime > maxAutoRuns {
			byTime = maxAutoRuns
		}
		best = byTime
	}

	if availableCapital > 0 && singleRunCost > 0 {
		byCap := int(math.Floor((availableCapital * capitalShareCap) / singleRunCost))
		if byCap < 1 {
			byCap = 1
		}
		if byCap < best {
			best = byCap
			reason = "25% capital"
		}
	}

	if dailyVolume7d > 0 && outputPerRun > 0 {
		byVol := int(math.Floor((dailyVolume7d * dailyVolShareCap) / float64(outputPerRun)))
		if byVol < 1 {
			byVol = 1
		}
		if byVol < best {
			best = byVol
			reason = "20% daily vol"
		}
	} else if dailyVolume7d <= 0 && best > 10 {
		best = 10
		reason = "Safe default"
	}

	best = clampInt(best, 1, maxAutoRuns)
	return best, reason, true
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
