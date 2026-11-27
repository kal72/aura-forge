package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aura-forge/internal/domain"

	"golang.org/x/sync/singleflight"
)

var embeddedAISingleflight singleflight.Group

// GetDashboardDailyStrip generates the Top 3 concrete prioritized action cards (Craft / Relist / Avoid)
// using live dashboard state + Opportunity Radar, enhanced by cached Embedded Background AI when enabled.
func (s *CopilotService) GetDashboardDailyStrip(ctx context.Context) *domain.DashboardDailyStrip {
	dash, err := s.GetDashboard(ctx)
	if err != nil || dash == nil {
		dash = &DashboardSummary{}
	}
	radar := s.GetRadarSnapshot(ctx)

	fallback := buildDeterministicDailyStrip(dash, radar)
	if s.aiProvider == nil || !s.aiProvider.IsEnabled() {
		return fallback
	}

	sig := fmt.Sprintf("cap:%.0f|undercut:%d|slow:%d|slots:%d/%d|radar:%d",
		dash.AvailableCapital, dash.UndercutBatchesCount, dash.SlowMoverBatchesCount,
		dash.MfgSlotsUsed, dash.MfgSlotsTotal, len(radar.TopOpportunities),
	)
	sum := sha256.Sum256([]byte(sig))
	ctxHash := hex.EncodeToString(sum[:])
	const featureKey = "dashboard_daily_strip"

	if s.tacticalRepo != nil {
		if raw, hit, err := s.tacticalRepo.GetAICache(ctx, featureKey, ctxHash); err == nil && hit && len(raw) > 0 {
			var cached domain.DashboardDailyStrip
			if json.Unmarshal(raw, &cached) == nil && len(cached.Cards) > 0 {
				cached.Source = "ai_cached"
				return &cached
			}
		}
	}

	val, _, _ := embeddedAISingleflight.Do(ctxHash, func() (any, error) {
		aiCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
		defer cancel()

		topRadar := "none"
		if len(radar.TopOpportunities) > 0 {
			t := radar.TopOpportunities[0]
			topRadar = fmt.Sprintf("%s (%.1f%% net margin, %.0f ISK net profit)", t.ProductName, t.NetMarginPercent, t.NetProfit)
		}
		prompt := fmt.Sprintf(
			"Available Capital: %.0f ISK | Locked: %.0f ISK | Active Batches: %d | Undercuts: %d | Slow Movers: %d | Mfg Slots: %d/%d | Top Radar Opportunity: %s.\n"+
				"Return strict JSON with 3 concise beginner-safe daily action cards (CRAFT, RELIST, AVOID or SCALE).",
			dash.AvailableCapital, dash.LockedCapital, dash.ActiveBatchesCount,
			dash.UndercutBatchesCount, dash.SlowMoverBatchesCount,
			dash.MfgSlotsUsed, dash.MfgSlotsTotal, topRadar,
		)

		resp, err := s.aiProvider.ChatCompletion(aiCtx, domain.AIChatRequest{
			Temperature: 0.2,
			MaxTokens:   450,
			Messages: []domain.AIChatMessage{
				{
					Role: "system",
					Content: `You are Aura Forge's embedded daily operations engine for a beginner EVE Online industrialist scaling toward 100B+ ISK.
Respond ONLY with raw JSON (no markdown fences):
{"headline":"1 short sentence","cards":[{"action_type":"CRAFT|RELIST|AVOID|SCALE","title":"3-6 words","metric_badge":"short badge","reason":"1 crisp sentence","action_label":"2-3 words","action_url":"/scanner|/manufacture|/tracker|/marketplace|/tactical"}]}`,
				},
				{Role: "user", Content: prompt},
			},
		})
		if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
			return fallback, nil
		}

		cleaned := strings.TrimSpace(resp.Content)
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		if sIdx, eIdx := strings.Index(cleaned, "{"), strings.LastIndex(cleaned, "}"); sIdx >= 0 && eIdx > sIdx {
			cleaned = cleaned[sIdx : eIdx+1]
		}
		var out domain.DashboardDailyStrip
		if json.Unmarshal([]byte(cleaned), &out) != nil || len(out.Cards) == 0 {
			return fallback, nil
		}
		out.Source = "ai_live"
		if len(out.Cards) > 3 {
			out.Cards = out.Cards[:3]
		}
		if s.tacticalRepo != nil {
			if payload, mErr := json.Marshal(out); mErr == nil {
				_ = s.tacticalRepo.SaveAICache(aiCtx, featureKey, ctxHash, resp.ModelUsed, payload, 30*time.Minute)
			}
		}
		return &out, nil
	})
	if res, ok := val.(*domain.DashboardDailyStrip); ok && res != nil {
		return res
	}
	return fallback
}

func buildDeterministicDailyStrip(dash *DashboardSummary, radar RadarSnapshot) *domain.DashboardDailyStrip {
	cards := make([]domain.DailyActionCard, 0, 3)

	// Card 1: Top Craft / Opportunity
	if len(radar.TopOpportunities) > 0 {
		top := radar.TopOpportunities[0]
		cards = append(cards, domain.DailyActionCard{
			ActionType:  "CRAFT",
			Title:       "Craft " + top.ProductName,
			MetricBadge: fmt.Sprintf("%.1f%% Margin", top.NetMarginPercent),
			Reason:      fmt.Sprintf("Top Opportunity Radar pick with strong velocity and %.1fM ISK estimated net profit.", top.NetProfit/1_000_000),
			ActionLabel: "Open Manufacture",
			ActionURL:   "/manufacture",
		})
	} else {
		cards = append(cards, domain.DailyActionCard{
			ActionType:  "CRAFT",
			Title:       "Scan High-Velocity T1 Builds",
			MetricBadge: fmt.Sprintf("%d/%d Slots", dash.MfgSlotsUsed, dash.MfgSlotsTotal),
			Reason:      "Keep your manufacturing lines active with fast-moving ammunition, rigs, or drones.",
			ActionLabel: "Run Scanner",
			ActionURL:   "/scanner",
		})
	}

	// Card 2: Relist / Order Protection
	if dash.UndercutBatchesCount > 0 {
		cards = append(cards, domain.DailyActionCard{
			ActionType:  "RELIST",
			Title:       fmt.Sprintf("Protect %d Undercut Orders", dash.UndercutBatchesCount),
			MetricBadge: fmt.Sprintf("%d Undercut", dash.UndercutBatchesCount),
			Reason:      "Verify Relist Fee Guard before modifying prices so broker fees do not erase net margin.",
			ActionLabel: "Review Tracker",
			ActionURL:   "/tracker",
		})
	} else {
		cards = append(cards, domain.DailyActionCard{
			ActionType:  "SCALE",
			Title:       "Diversify Active Capital",
			MetricBadge: fmt.Sprintf("%.0fM Free", dash.AvailableCapital/1_000_000),
			Reason:      "Split free ISK across 3–5 liquid items instead of locking all capital into one slow hull.",
			ActionLabel: "Open Marketplace",
			ActionURL:   "/marketplace",
		})
	}

	// Card 3: Avoid Trap / Risk Guard
	if dash.SlowMoverBatchesCount > 0 {
		cards = append(cards, domain.DailyActionCard{
			ActionType:  "AVOID",
			Title:       fmt.Sprintf("Pause %d Slow Movers", dash.SlowMoverBatchesCount),
			MetricBadge: "Capital Guard",
			Reason:      "Avoid restocking slow-moving batches until existing sell orders clear and free up ISK.",
			ActionLabel: "Audit Batches",
			ActionURL:   "/tracker",
		})
	} else {
		cards = append(cards, domain.DailyActionCard{
			ActionType:  "AVOID",
			Title:       "Check Anti-Gank Hauler EHP",
			MetricBadge: "Risk Guard",
			Reason:      "Before hauling high-value cargo between trade hubs, verify your hauler fit in Aura Tactical.",
			ActionLabel: "Open Tactical",
			ActionURL:   "/tactical",
		})
	}

	return &domain.DashboardDailyStrip{
		Headline: "Today's Top 3 Prioritized Operations",
		Source:   "deterministic",
		Cards:    cards,
	}
}

// BuildAIWeeklyPlan generates a diversified, risk-adjusted Weekly Manufacturing Plan for a given ISK budget
// and risk profile (conservative | balanced | aggressive), cached in PostgreSQL ai_tactical_cache.
func (s *CopilotService) BuildAIWeeklyPlan(ctx context.Context, budgetISK float64, riskProfile string) (*domain.AIWeeklyPlanDraft, error) {
	if budgetISK <= 0 {
		if dash, err := s.GetDashboard(ctx); err == nil && dash != nil && dash.AvailableCapital > 0 {
			budgetISK = dash.AvailableCapital
		} else {
			budgetISK = 150_000_000
		}
	}
	risk := strings.ToLower(strings.TrimSpace(riskProfile))
	var fastPct, midPct, highPct, bufPct float64
	switch risk {
	case "conservative":
		fastPct, midPct, highPct, bufPct = 50, 25, 10, 15
	case "aggressive":
		fastPct, midPct, highPct, bufPct = 25, 30, 35, 10
	default:
		risk = "balanced"
		fastPct, midPct, highPct, bufPct = 35, 30, 20, 15
	}

	radar := s.GetRadarSnapshot(ctx)
	deployable := budgetISK * (1.0 - bufPct/100.0)
	bufISK := domain.RoundISK(budgetISK - deployable)

	// Build deterministic candidate items from live Opportunity Radar (or curated beginner staples if radar empty)
	items := make([]domain.AIWeeklyPlanItem, 0, 5)
	var totalAlloc, totalProfit float64

	if len(radar.TopOpportunities) > 0 {
		limit := len(radar.TopOpportunities)
		if limit > 5 {
			limit = 5
		}
		perItemCap := deployable / float64(limit)
		buckets := []string{"FAST_CASHFLOW", "FAST_CASHFLOW", "MID_TIER", "MID_TIER", "HIGH_ROI"}
		for i := 0; i < limit; i++ {
			op := radar.TopOpportunities[i]
			unitCost := op.MaterialCost
			if unitCost <= 0 {
				unitCost = 2_500_000
			}
			runs := int(perItemCap / unitCost)
			if runs < 1 {
				runs = 1
			}
			if runs > 40 {
				runs = 40
			}
			cost := domain.RoundISK(float64(runs) * unitCost)
			if totalAlloc+cost > deployable && len(items) > 0 {
				continue
			}
			profitPerRun := op.NetProfit
			if op.Runs > 1 {
				profitPerRun = op.NetProfit / float64(op.Runs)
			}
			estProf := domain.RoundISK(profitPerRun * float64(runs))
			totalAlloc += cost
			totalProfit += estProf
			items = append(items, domain.AIWeeklyPlanItem{
				TypeID:          op.ProductTypeID,
				Name:            op.ProductName,
				Bucket:          buckets[i],
				Runs:            runs,
				EstimatedCost:   cost,
				EstimatedProfit: estProf,
				MarginPercent:   op.NetMarginPercent,
				Rationale:       fmt.Sprintf("%.0f units/day regional turnover with %.1f%% net margin.", op.DailyVolume7d, op.NetMarginPercent),
			})
		}
	}

	if len(items) == 0 {
		// Curated beginner-safe fallback portfolio when radar hasn't swept yet
		fallbacks := []struct {
			id     int32
			name   string
			bucket string
			share  float64
			margin float64
			why    string
		}{
			{2454, "Hobgoblin I", "FAST_CASHFLOW", 0.35, 16.5, "High-velocity combat drone used by every Gallente/cruiser pilot."},
			{2486, "Warrior I", "FAST_CASHFLOW", 0.25, 15.2, "Fast-selling Minmatar light drone with reliable daily volume."},
			{31718, "Medium Core Defense Field Extender I", "MID_TIER", 0.25, 19.0, "Core PvE shield rig with steady cruiser demand."},
			{2048, "Damage Control I", "HIGH_ROI", 0.15, 21.5, "Universal hull tank module with healthy crafting spread."},
		}
		for _, fb := range fallbacks {
			cost := domain.RoundISK(deployable * fb.share)
			prof := domain.RoundISK(cost * (fb.margin / 100.0))
			totalAlloc += cost
			totalProfit += prof
			items = append(items, domain.AIWeeklyPlanItem{
				TypeID:          fb.id,
				Name:            fb.name,
				Bucket:          fb.bucket,
				Runs:            10,
				EstimatedCost:   cost,
				EstimatedProfit: prof,
				MarginPercent:   fb.margin,
				Rationale:       fb.why,
			})
		}
	}

	draft := &domain.AIWeeklyPlanDraft{
		Title:              fmt.Sprintf("AI %s Plan (%.0fM ISK)", strings.Title(risk), budgetISK/1_000_000),
		BudgetISK:          domain.RoundISK(budgetISK),
		RiskProfile:        risk,
		Source:             "deterministic",
		StrategySummary:    fmt.Sprintf("Diversified %d-item %s portfolio keeping %.0f%% Cash Buffer (%s) protected for broker fees & relists.", len(items), risk, bufPct, formatCompactISK(bufISK)),
		FastCashflowPct:    fastPct,
		MidTierPct:         midPct,
		HighROIPct:         highPct,
		CashBufferPct:      bufPct,
		TotalAllocatedISK:  domain.RoundISK(totalAlloc),
		CashBufferISK:      bufISK,
		EstimatedNetProfit: domain.RoundISK(totalProfit),
		Items:              items,
	}

	if s.aiProvider == nil || !s.aiProvider.IsEnabled() {
		return draft, nil
	}

	// Enhance strategy summary via cached Embedded Background AI
	sig := fmt.Sprintf("plan:%.0f|%s|%d", budgetISK, risk, len(items))
	sum := sha256.Sum256([]byte(sig))
	ctxHash := hex.EncodeToString(sum[:])
	const featureKey = "allocator_weekly_plan"

	if s.tacticalRepo != nil {
		if raw, hit, cErr := s.tacticalRepo.GetAICache(ctx, featureKey, ctxHash); cErr == nil && hit && len(raw) > 0 {
			var cached domain.AIWeeklyPlanDraft
			if json.Unmarshal(raw, &cached) == nil && cached.StrategySummary != "" {
				draft.StrategySummary = cached.StrategySummary
				draft.Source = "ai_cached"
				return draft, nil
			}
		}
	}

	val, _, _ := embeddedAISingleflight.Do(ctxHash, func() (any, error) {
		aiCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
		defer cancel()
		names := make([]string, 0, len(items))
		for _, it := range items {
			names = append(names, fmt.Sprintf("%s (%s, %.1f%% margin)", it.Name, it.Bucket, it.MarginPercent))
		}
		resp, err := s.aiProvider.ChatCompletion(aiCtx, domain.AIChatRequest{
			Temperature: 0.2,
			MaxTokens:   180,
			Messages: []domain.AIChatMessage{
				{Role: "system", Content: `Respond ONLY with raw JSON: {"strategy_summary":"2 concise beginner-friendly sentences explaining why this split grows ISK safely without capital lock-up."}`},
				{Role: "user", Content: fmt.Sprintf("Budget: %.0f ISK | Risk: %s | Split: %.0f/%.0f/%.0f/%.0f | Items: %s | Est Profit: %.0f ISK", budgetISK, risk, fastPct, midPct, highPct, bufPct, strings.Join(names, ", "), totalProfit)},
			},
		})
		if err != nil || resp == nil {
			return draft, nil
		}
		cleaned := strings.TrimSpace(resp.Content)
		if sIdx, eIdx := strings.Index(cleaned, "{"), strings.LastIndex(cleaned, "}"); sIdx >= 0 && eIdx > sIdx {
			var parsed struct {
				StrategySummary string `json:"strategy_summary"`
			}
			if json.Unmarshal([]byte(cleaned[sIdx:eIdx+1]), &parsed) == nil && parsed.StrategySummary != "" {
				draft.StrategySummary = parsed.StrategySummary
				draft.Source = "ai_live"
				if s.tacticalRepo != nil {
					if payload, mErr := json.Marshal(draft); mErr == nil {
						_ = s.tacticalRepo.SaveAICache(aiCtx, featureKey, ctxHash, resp.ModelUsed, payload, 30*time.Minute)
					}
				}
			}
		}
		return draft, nil
	})
	if res, ok := val.(*domain.AIWeeklyPlanDraft); ok && res != nil {
		return res, nil
	}
	return draft, nil
}

func formatCompactISK(val float64) string {
	if val >= 1e9 {
		return fmt.Sprintf("%.2fB ISK", val/1e9)
	}
	if val >= 1e6 {
		return fmt.Sprintf("%.1fM ISK", val/1e6)
	}
	return fmt.Sprintf("%.0f ISK", val)
}
