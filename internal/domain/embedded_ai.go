package domain

import (
	"fmt"
	"math"
)

// DailyActionCard is one concrete, prioritized Embedded AI directive on the Dashboard.
type DailyActionCard struct {
	ActionType  string `json:"action_type"`  // CRAFT | RELIST | AVOID | SCALE
	Title       string `json:"title"`
	MetricBadge string `json:"metric_badge"` // e.g. "+18.4% Margin" or "2 Undercuts"
	Reason      string `json:"reason"`
	ActionLabel string `json:"action_label"`
	ActionURL   string `json:"action_url"`
}

// DashboardDailyStrip bundles the top 3 Embedded AI daily action cards.
type DashboardDailyStrip struct {
	Headline string            `json:"headline"`
	Source   string            `json:"source"` // ai_live | ai_cached | deterministic
	Cards    []DailyActionCard `json:"cards"`
}

// MarketplaceVerdictResult is a compact inline order-book verdict rendered natively in /marketplace.
type MarketplaceVerdictResult struct {
	Verdict         string   `json:"verdict"` // STATION TRADE | CRAFT IT | HAUL | AVOID TRAP
	ConfidenceScore int      `json:"confidence_score"`
	Headline        string   `json:"headline"`
	Bullets         []string `json:"bullets"`
	Tone            string   `json:"tone"` // emerald | sky | amber | rose
}

// CompareBeginnerPick crowns the best beginner-friendly item when comparing 2-3 ships or modules.
type CompareBeginnerPick struct {
	WinnerName   string   `json:"winner_name"`
	WinnerTypeID int32    `json:"winner_type_id"`
	Badge        string   `json:"badge"`
	Rationale    []string `json:"rationale"`
}

// EvaluateMarketplaceVerdict computes a deterministic, zero-latency order-book verdict from live market data.
func EvaluateMarketplaceVerdict(snap *MarketplaceSnapshot) *MarketplaceVerdictResult {
	if snap == nil {
		return nil
	}
	bullets := make([]string, 0, 3)
	dailyVol := snap.AvgDailyVolume7d
	if dailyVol <= 0 {
		dailyVol = snap.AvgDailyVolume30d
	}

	// Check liquidity & spread traps first
	if dailyVol < 15 || snap.SellOrdersCount == 0 {
		bullets = append(bullets, fmt.Sprintf("Thin daily volume (%.0f units/day) creates high capital lock-up risk.", dailyVol))
		if snap.SpreadPercent > 35 {
			bullets = append(bullets, fmt.Sprintf("Wide %.1f%% spread indicates manipulative or stale orders.", snap.SpreadPercent))
		}
		return &MarketplaceVerdictResult{
			Verdict:         "AVOID TRAP",
			ConfidenceScore: 88,
			Headline:        "Low liquidity / high capital lock-up risk for beginners.",
			Bullets:         bullets,
			Tone:            "rose",
		}
	}

	// Check strong crafting savings first if blueprint exists
	if snap.HasBlueprint && snap.CraftSavingsPct >= 10 {
		bullets = append(bullets, fmt.Sprintf("Manufacturing saves ~%.1f%% vs buying directly on the market.", snap.CraftSavingsPct))
		bullets = append(bullets, fmt.Sprintf("Steady regional velocity (%.0f units/day) supports reliable sell-order turnover.", dailyVol))
		return &MarketplaceVerdictResult{
			Verdict:         "CRAFT IT",
			ConfidenceScore: 87,
			Headline:        "Strong blueprint margin — craft from materials rather than flipping.",
			Bullets:         bullets,
			Tone:            "sky",
		}
	}

	// Check strong station trading flip
	if snap.NetFlipProfitUnit > 0 && snap.SpreadPercent >= 7.5 && snap.SpreadPercent <= 35 && dailyVol >= 50 {
		bullets = append(bullets, fmt.Sprintf("Positive net flip after taxes with %.1f%% gross spread.", snap.SpreadPercent))
		bullets = append(bullets, fmt.Sprintf("Healthy turnover (%.0f units/day across %d active sell orders).", dailyVol, snap.SellOrdersCount))
		return &MarketplaceVerdictResult{
			Verdict:         "STATION TRADE",
			ConfidenceScore: 86,
			Headline:        "Healthy margin & liquidity — viable for Buy-to-Sell station trading.",
			Bullets:         bullets,
			Tone:            "emerald",
		}
	}

	// Check inter-hub arbitrage opportunity
	if snap.BestArbNetMarginPct >= 10 && snap.BestArbDestHub.Name != "" && dailyVol >= 25 {
		bullets = append(bullets, fmt.Sprintf("%.1f%% net arbitrage margin detected toward %s.", snap.BestArbNetMarginPct, snap.BestArbDestHub.Name))
		bullets = append(bullets, "Verify cargo m³ & anti-gank hauler EHP in Aura Tactical before hauling.")
		return &MarketplaceVerdictResult{
			Verdict:         "HAUL",
			ConfidenceScore: 82,
			Headline:        fmt.Sprintf("Inter-hub arbitrage spread available toward %s.", snap.BestArbDestHub.Name),
			Bullets:         bullets,
			Tone:            "amber",
		}
	}

	// Default: Craft It / Tight Spread
	bullets = append(bullets, fmt.Sprintf("Tight %.1f%% spread favors manufacturing from raw materials over station flipping.", snap.SpreadPercent))
	bullets = append(bullets, fmt.Sprintf("Steady market demand (~%.0f units/day) supports reliable sell-order velocity.", dailyVol))
	return &MarketplaceVerdictResult{
		Verdict:         "CRAFT IT",
		ConfidenceScore: 78,
		Headline:        "Tight market spread — check Blueprint material cost before buying.",
		Bullets:         bullets,
		Tone:            "sky",
	}
}

// EvaluateCompareBeginnerPick scores 2-3 compared EVE items deterministically to highlight the safest Beginner Pick.
func EvaluateCompareBeginnerPick(res *ItemCompareResult) *CompareBeginnerPick {
	if res == nil || len(res.Items) < 2 {
		return nil
	}

	bestIdx := 0
	bestScore := -1e9
	bestReasons := []string{}

	minPrice := 0.0
	for _, it := range res.Items {
		if it.LiveSellPrice > 0 && (minPrice == 0 || it.LiveSellPrice < minPrice) {
			minPrice = it.LiveSellPrice
		}
	}

	for i, it := range res.Items {
		score := 0.0
		reasons := make([]string, 0, 2)

		// Lower Tech Level & fewer/lower required skills are friendlier for beginners
		if it.TechLevel <= 1 {
			score += 22
			reasons = append(reasons, "Tech 1 accessibility — fast skill training and lower replacement risk.")
		} else {
			score += 8
		}

		// Lower live market price reduces ISK loss exposure
		if it.LiveSellPrice > 0 && minPrice > 0 {
			ratio := minPrice / it.LiveSellPrice
			score += ratio * 35
			if ratio >= 0.95 {
				reasons = append(reasons, "Most cost-effective hull/module price at the active Trade Hub.")
			}
		}

		// Count best-in-row technical wins across comparison sections
		wins := 0
		for _, sec := range res.Sections {
			for _, row := range sec.Rows {
				if i < len(row.Cells) && row.Cells[i].IsBest {
					wins++
				}
			}
		}
		score += float64(wins) * 6.5
		if wins > 0 && len(reasons) < 2 {
			reasons = append(reasons, fmt.Sprintf("Leads in %d key technical metrics (fitting, tank, or performance).", wins))
		}
		if len(reasons) == 0 {
			reasons = append(reasons, "Balanced all-around stats with manageable fitting and ISK cost.")
		}
		if len(reasons) > 2 {
			reasons = reasons[:2]
		}

		if score > bestScore || (math.Abs(score-bestScore) < 0.01 && i < bestIdx) {
			bestScore = score
			bestIdx = i
			bestReasons = reasons
		}
	}

	winner := res.Items[bestIdx]
	return &CompareBeginnerPick{
		WinnerName:   winner.Name,
		WinnerTypeID: winner.TypeID,
		Badge:        "BEGINNER PICK",
		Rationale:    bestReasons,
	}
}

// AIWeeklyPlanItem represents one diversified production line inside an Embedded AI Weekly Plan.
type AIWeeklyPlanItem struct {
	TypeID          int32   `json:"type_id"`
	Name            string  `json:"name"`
	Bucket          string  `json:"bucket"` // FAST_CASHFLOW | MID_TIER | HIGH_ROI
	Runs            int     `json:"runs"`
	EstimatedCost   float64 `json:"estimated_cost"`
	EstimatedProfit float64 `json:"estimated_profit"`
	MarginPercent   float64 `json:"margin_percent"`
	Rationale       string  `json:"rationale"`
}

// AIWeeklyPlanDraft is a ready-to-save Weekly Manufacturing Plan produced by Embedded Background AI.
type AIWeeklyPlanDraft struct {
	Title              string             `json:"title"`
	BudgetISK          float64            `json:"budget_isk"`
	RiskProfile        string             `json:"risk_profile"` // conservative | balanced | aggressive
	Source             string             `json:"source"`       // ai_live | ai_cached | deterministic
	StrategySummary    string             `json:"strategy_summary"`
	FastCashflowPct    float64            `json:"fast_cashflow_pct"`
	MidTierPct         float64            `json:"mid_tier_pct"`
	HighROIPct         float64            `json:"high_roi_pct"`
	CashBufferPct      float64            `json:"cash_buffer_pct"`
	TotalAllocatedISK  float64            `json:"total_allocated_isk"`
	CashBufferISK      float64            `json:"cash_buffer_isk"`
	EstimatedNetProfit float64            `json:"estimated_net_profit"`
	Items              []AIWeeklyPlanItem `json:"items"`
}
