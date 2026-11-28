package domain

import "fmt"

// TacticalModuleSwap is a structured 1-click module swap recommendation from the Embedded AI Strategist.
type TacticalModuleSwap struct {
	SlotKind          FitSlotKind `json:"slot_kind"` // high | mid | low | rig
	CurrentModule     string      `json:"current_module"`
	SuggestedModule   string      `json:"suggested_module"`
	SuggestedTypeID   int32       `json:"suggested_type_id,omitempty"`
	ImpactBadge       string      `json:"impact_badge"` // e.g. "+18% EHP" or "Cap Stable"
	Reason            string      `json:"reason"`
}

// TacticalPlaybook provides a 4-step operational checklist for flying the fit in the selected scenario.
type TacticalPlaybook struct {
	PreWarp          string `json:"pre_warp"`
	Positioning      string `json:"positioning"`
	ThreatPriority   string `json:"threat_priority"`
	DisengageTrigger string `json:"disengage_trigger"`
}

// TacticalAuditResult is the strict JSON schema returned by Embedded Background AI (or deterministic fallback).
type TacticalAuditResult struct {
	Grade            string               `json:"grade"` // S | A | B | C | RISK
	Source           string               `json:"source"` // "ai_live" | "ai_cached" | "deterministic"
	Summary          string               `json:"summary"`
	Weaknesses       []string             `json:"weaknesses"`
	RecommendedSwaps []TacticalModuleSwap `json:"recommended_swaps"`
	Playbook         TacticalPlaybook     `json:"playbook"`
}

// BuildDeterministicTacticalAudit produces an instant, zero-token structured tactical audit
// from deterministic EVE Dogma simulation metrics (also used as fallback when AI is disabled).
func BuildDeterministicTacticalAudit(sim *FitSimulationResult) *TacticalAuditResult {
	weaknesses := make([]string, 0, 3)
	swaps := make([]TacticalModuleSwap, 0, 2)

	if sim.CPU.Overloaded {
		weaknesses = append(weaknesses, fmt.Sprintf("CPU overloaded at %.0f%% (%.1f / %.0f tf).", sim.CPU.Percent, sim.CPU.Used, sim.CPU.Total))
		swaps = append(swaps, TacticalModuleSwap{
			SlotKind:        SlotLow,
			CurrentModule:   "Empty / Excess Low Slot",
			SuggestedModule: "Co-Processor II",
			SuggestedTypeID: 1201,
			ImpactBadge:     "+10% CPU",
			Reason:          "Resolves CPU overload so all modules can come online immediately.",
		})
	}
	if sim.Powergrid.Overloaded {
		weaknesses = append(weaknesses, fmt.Sprintf("Powergrid overloaded at %.0f%% (%.1f / %.0f MW).", sim.Powergrid.Percent, sim.Powergrid.Used, sim.Powergrid.Total))
		swaps = append(swaps, TacticalModuleSwap{
			SlotKind:        SlotLow,
			CurrentModule:   "Empty / Excess Low Slot",
			SuggestedModule: "Reactor Control Unit II",
			SuggestedTypeID: 1403,
			ImpactBadge:     "+15% Powergrid",
			Reason:          "Adds Powergrid headroom to bring heavy weapons or shield extenders online.",
		})
	}
	if sim.Shield.HasHole && sim.Shield.RawHP >= sim.Armor.RawHP {
		weaknesses = append(weaknesses, fmt.Sprintf("Critical Shield %s hole vs %s.", sim.Shield.LowestType, sim.TargetFaction.ShortLabel))
		swaps = append(swaps, TacticalModuleSwap{
			SlotKind:        SlotMid,
			CurrentModule:   "Open Medium Slot",
			SuggestedModule: "Multispectrum Shield Hardener II",
			SuggestedTypeID: 2281,
			ImpactBadge:     "+30% All Shield Resists",
			Reason:          "Plugs the resistance hole against " + sim.TargetFaction.Name + " incoming damage.",
		})
	} else if sim.Armor.HasHole && sim.Armor.RawHP > sim.Shield.RawHP {
		weaknesses = append(weaknesses, fmt.Sprintf("Critical Armor %s hole vs %s.", sim.Armor.LowestType, sim.TargetFaction.ShortLabel))
		swaps = append(swaps, TacticalModuleSwap{
			SlotKind:        SlotLow,
			CurrentModule:   "Open Low Slot",
			SuggestedModule: "Multispectrum Energized Membrane II",
			SuggestedTypeID: 11269,
			ImpactBadge:     "+20% All Armor Resists",
			Reason:          "Plugs the armor resistance hole against " + sim.TargetFaction.Name + ".",
		})
	}
	if !sim.Capacitor.Stable && sim.Capacitor.TimeToEmptySec < 150 {
		weaknesses = append(weaknesses, fmt.Sprintf("Capacitor runs dry in %s under continuous load.", sim.Capacitor.FormattedTime))
		swaps = append(swaps, TacticalModuleSwap{
			SlotKind:        SlotMid,
			CurrentModule:   "Open Medium Slot",
			SuggestedModule: "Cap Recharger II",
			SuggestedTypeID: 2032,
			ImpactBadge:     "+20% Cap Recharge",
			Reason:          "Stabilizes active repairer and propulsion capacitor sustainability.",
		})
	}
	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "No critical fitting flaws detected for "+sim.TargetFaction.ShortLabel+" engagements.")
	}

	optKM := sim.Offense.OptimalRangeKM
	if optKM <= 0 {
		optKM = 18
	}

	return &TacticalAuditResult{
		Grade:            sim.ReadinessGrade,
		Source:           "deterministic",
		Summary:          fmt.Sprintf("%s rated Grade %s vs %s — %.0f EHP, %.0f EHP/s tank, %.0f DPS.", sim.Fit.ShipName, sim.ReadinessGrade, sim.TargetFaction.ShortLabel, sim.TotalEHP, sim.SustainedTankEHPs, sim.Offense.TotalDPS),
		Weaknesses:       weaknesses,
		RecommendedSwaps: swaps,
		Playbook: TacticalPlaybook{
			PreWarp:          fmt.Sprintf("Load %s ammunition, verify drones in bay, and pre-activate hardeners on warp-in.", joinOrDefault(sim.TargetFaction.WeakAgainst, "optimal")),
			Positioning:      fmt.Sprintf("Orbit at %.0f km with propulsion active to maintain transversal velocity and weapon application.", mathMax(optKM*0.8, 10)),
			ThreatPriority:   fmt.Sprintf("Clear fast tackle & EWAR frigates first (%s), then focus cruiser/battleship DPS hulls.", sim.TargetFaction.EWARWarning),
			DisengageTrigger: fmt.Sprintf("Align out immediately if Shield/Armor drops below 35%% or Capacitor falls under 25%% (align time %.2fs).", sim.Navigation.AlignTimeSeconds),
		},
	}
}

func joinOrDefault(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}
	out := items[0]
	for i := 1; i < len(items); i++ {
		out += " / " + items[i]
	}
	return out
}

func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
