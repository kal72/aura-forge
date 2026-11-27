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
