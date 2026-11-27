package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"aura-forge/internal/domain"

	"golang.org/x/sync/singleflight"
)

var tacticalAISingleflight singleflight.Group

// SimulateTacticalDuel runs a second-by-second 1v1 or PvE boss combat timeline simulation for a ShipFit.
func (s *CopilotService) SimulateTacticalDuel(ctx context.Context, req domain.DuelSimRequest) (*domain.DuelSimulationResult, error) {
	sim, err := s.SimulateTacticalFit(ctx, req.Fit)
	if err != nil {
		return nil, err
	}
	return domain.SimulateDuelTimeline(sim, req), nil
}

// AuditTacticalFit evaluates a ShipFit using Embedded Background AI (strict JSON schema + PostgreSQL cache + singleflight),
// falling back seamlessly to deterministic Dogma auditing when AI is disabled or unreachable.
func (s *CopilotService) AuditTacticalFit(ctx context.Context, fit domain.ShipFit) (*domain.TacticalAuditResult, error) {
	sim, err := s.SimulateTacticalFit(ctx, fit)
	if err != nil {
		return nil, err
	}
	fallback := domain.BuildDeterministicTacticalAudit(sim)

	if s.aiProvider == nil || !s.aiProvider.IsEnabled() {
		return fallback, nil
	}

	ctxHash := computeTacticalAuditHash(sim)
	const featureKey = "tactical_fit_audit"

	// 1. Check PostgreSQL structured JSON cache first (0 tokens on cache hit)
	if s.tacticalRepo != nil {
		if raw, hit, cErr := s.tacticalRepo.GetAICache(ctx, featureKey, ctxHash); cErr == nil && hit && len(raw) > 0 {
			var cached domain.TacticalAuditResult
			if json.Unmarshal(raw, &cached) == nil && cached.Grade != "" {
				cached.Source = "ai_cached"
				return &cached, nil
			}
		}
	}

	// 2. Deduplicate concurrent identical audit requests via singleflight
	val, sfErr, _ := tacticalAISingleflight.Do(ctxHash, func() (any, error) {
		aiCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()

		prompt := buildTacticalAuditPrompt(sim)
		resp, err := s.aiProvider.ChatCompletion(aiCtx, domain.AIChatRequest{
			Temperature: 0.25,
			MaxTokens:   700,
			Messages: []domain.AIChatMessage{
				{
					Role: "system",
					Content: `You are Aura Tactical, an elite EVE Online ship-fitting and combat strategist helping a beginner pilot fly safely and profitably.
Respond ONLY with valid raw JSON (no markdown fences, no prose outside JSON) matching this exact schema:
{
  "grade": "S|A|B|C|RISK",
  "summary": "1 concise sentence summarizing survivability and combat role",
  "weaknesses": ["max 3 crisp bullet strings"],
  "recommended_swaps": [
    {
      "slot_kind": "high|mid|low|rig",
      "current_module": "exact name of current module or Open Slot",
      "suggested_module": "exact official EVE Online module name",
      "impact_badge": "short impact e.g. +24% EHP or Cap Stable",
      "reason": "1 short beginner-friendly sentence"
    }
  ],
  "playbook": {
    "pre_warp": "1 short step",
    "positioning": "1 short step with km range",
    "threat_priority": "1 short step",
    "disengage_trigger": "1 short step with % threshold"
  }
}`,
				},
				{Role: "user", Content: prompt},
			},
		})
		if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
			return fallback, nil
		}

		parsed, ok := parseTacticalAuditJSON(resp.Content)
		if !ok {
			return fallback, nil
		}
		parsed.Source = "ai_live"

		// Resolve official TypeIDs for suggested swaps so 1-click "Apply Swap" works directly on the Orbital Wheel
		for i := range parsed.RecommendedSwaps {
			if parsed.RecommendedSwaps[i].SuggestedModule != "" {
				if tid, rErr := s.resolveCompareQueryTypeID(aiCtx, parsed.RecommendedSwaps[i].SuggestedModule); rErr == nil && tid > 0 {
					parsed.RecommendedSwaps[i].SuggestedTypeID = tid
				}
			}
		}

		if s.tacticalRepo != nil {
			if payload, mErr := json.Marshal(parsed); mErr == nil {
				_ = s.tacticalRepo.SaveAICache(aiCtx, featureKey, ctxHash, resp.ModelUsed, payload, 6*time.Hour)
			}
		}
		return parsed, nil
	})
	if sfErr != nil {
		return fallback, nil
	}
	if res, ok := val.(*domain.TacticalAuditResult); ok && res != nil {
		return res, nil
	}
	return fallback, nil
}

func computeTacticalAuditHash(sim *domain.FitSimulationResult) string {
	parts := make([]string, 0, len(sim.Fit.Modules)+4)
	parts = append(parts,
		fmt.Sprintf("ship:%d", sim.Fit.ShipTypeID),
		"skill:"+string(sim.Fit.SkillProfile),
		"target:"+sim.TargetFaction.Key,
	)
	for _, m := range sim.Fit.Modules {
		parts = append(parts, fmt.Sprintf("%s:%d:%s", m.SlotKind, m.TypeID, m.State))
	}
	sort.Strings(parts[3:])
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func buildTacticalAuditPrompt(sim *domain.FitSimulationResult) string {
	var mods []string
	for _, m := range sim.Fit.Modules {
		mods = append(mods, fmt.Sprintf("[%s] %s (%s)", m.SlotKind, m.Name, m.State))
	}
	return fmt.Sprintf(
		"Ship: %s (%s) | Skill Profile: %s | Target Faction: %s (Benchmark %.0f DPS, EWAR: %s)\n"+
			"CPU: %.1f/%.0f tf (%.0f%%) | PG: %.1f/%.0f MW (%.0f%%) | Calib: %.0f/%.0f\n"+
			"Total EHP: %.0f | Sustained Tank: %.1f EHP/s | Cap: %s\n"+
			"Shield Resists (EM/Th/Ki/Ex): %.0f/%.0f/%.0f/%.0f%% | Armor Resists: %.0f/%.0f/%.0f/%.0f%%\n"+
			"Offense: %.1f DPS (Volley %.0f, Optimal %.1f km) | Velocity: %.0f m/s | Align: %.2fs\n"+
			"Fitted Modules: %s\n"+
			"Provide max 2 concrete module swaps that fit within CPU/PG limits and improve survivability or clear speed.",
		sim.Fit.ShipName, sim.Fit.HullClass, sim.Fit.SkillProfile,
		sim.TargetFaction.Name, sim.TargetFaction.BenchmarkDPS, sim.TargetFaction.EWARWarning,
		sim.CPU.Used, sim.CPU.Total, sim.CPU.Percent,
		sim.Powergrid.Used, sim.Powergrid.Total, sim.Powergrid.Percent,
		sim.Calibration.Used, sim.Calibration.Total,
		sim.TotalEHP, sim.SustainedTankEHPs, sim.Capacitor.FormattedTime,
		sim.Shield.Resists.EM, sim.Shield.Resists.Thermal, sim.Shield.Resists.Kinetic, sim.Shield.Resists.Explosive,
		sim.Armor.Resists.EM, sim.Armor.Resists.Thermal, sim.Armor.Resists.Kinetic, sim.Armor.Resists.Explosive,
		sim.Offense.TotalDPS, sim.Offense.AlphaVolley, sim.Offense.OptimalRangeKM,
		sim.Navigation.MaxVelocityMS, sim.Navigation.AlignTimeSeconds,
		strings.Join(mods, ", "),
	)
}

func parseTacticalAuditJSON(raw string) (*domain.TacticalAuditResult, bool) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start >= 0 && end > start {
		cleaned = cleaned[start : end+1]
	}
	var out domain.TacticalAuditResult
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil || out.Grade == "" {
		return nil, false
	}
	return &out, true
}
