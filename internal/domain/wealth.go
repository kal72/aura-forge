package domain

import (
	"math"
	"sort"
	"time"
)

// NetWorthSnapshot records a daily breakdown of total EVE Online assets.
type NetWorthSnapshot struct {
	ID            int64     `json:"id"`
	SnapshotDate  string    `json:"snapshot_date"` // YYYY-MM-DD
	WalletISK     float64   `json:"wallet_isk"`
	SellOrdersISK float64   `json:"sell_orders_isk"`
	EscrowISK     float64   `json:"escrow_isk"`
	InventoryISK  float64   `json:"inventory_isk"`
	ActiveJobsISK float64   `json:"active_jobs_isk"`
	TotalNetWorth float64   `json:"total_net_worth"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ComputeTotal recalculates and rounds TotalNetWorth from its asset components.
func (s *NetWorthSnapshot) ComputeTotal() float64 {
	total := math.Max(0, s.WalletISK) +
		math.Max(0, s.SellOrdersISK) +
		math.Max(0, s.EscrowISK) +
		math.Max(0, s.InventoryISK) +
		math.Max(0, s.ActiveJobsISK)
	s.TotalNetWorth = RoundISK(total)
	return s.TotalNetWorth
}

// CapitalPhase provides beginner-to-100B+ guidance tailored to the player's current asset tier.
type CapitalPhase struct {
	TierNumber     int      `json:"tier_number"`
	Name           string   `json:"name"`
	RangeLabel     string   `json:"range_label"`
	FocusSummary   string   `json:"focus_summary"`
	Strategies     []string `json:"strategies"`
	PrioritySkills []string `json:"priority_skills"`
	RiskRules      []string `json:"risk_rules"`
}

// WealthMilestone tracks progress toward a major ISK net-worth target.
type WealthMilestone struct {
	Label       string  `json:"label"`
	TargetISK   float64 `json:"target_isk"`
	ProgressPct float64 `json:"progress_pct"`
	Remaining   float64 `json:"remaining"`
	Reached     bool    `json:"reached"`
}

// ItemProfitBreakdown aggregates realized profit per item from SOLD batches.
type ItemProfitBreakdown struct {
	ItemName     string  `json:"item_name"`
	BatchesSold  int     `json:"batches_sold"`
	UnitsSold    int     `json:"units_sold"`
	TotalCost    float64 `json:"total_cost"`
	TotalProfit  float64 `json:"total_profit"`
	AvgROIPct    float64 `json:"avg_roi_pct"`
}

// WealthSummary bundles all KPIs, milestones, phase advice, equity curve data, and profit contributors.
type WealthSummary struct {
	CurrentNetWorth float64               `json:"current_net_worth"`
	DailyChangeISK  float64               `json:"daily_change_isk"`
	DailyChangePct  float64               `json:"daily_change_pct"`
	WalletISK       float64               `json:"wallet_isk"`
	SellOrdersISK   float64               `json:"sell_orders_isk"`
	EscrowISK       float64               `json:"escrow_isk"`
	InventoryISK    float64               `json:"inventory_isk"`
	ActiveJobsISK   float64               `json:"active_jobs_isk"`
	CurrentPhase    CapitalPhase          `json:"current_phase"`
	NextMilestone   WealthMilestone       `json:"next_milestone"`
	Milestones      []WealthMilestone     `json:"milestones"`
	Snapshots       []NetWorthSnapshot    `json:"snapshots"`
	TopProfitItems  []ItemProfitBreakdown `json:"top_profit_items"`
}

// ResolveCapitalPhase returns the recommended beginner-to-tycoon playbook for a given net worth in ISK.
func ResolveCapitalPhase(netWorthISK float64) CapitalPhase {
	switch {
	case netWorthISK < 100_000_000:
		return CapitalPhase{
			TierNumber:   1,
			Name:         "Seed Capital",
			RangeLabel:   "0 – 100M ISK",
			FocusSummary: "Build fast cashflow velocity with high-turnover T1 items and protect every ISK from fees.",
			Strategies: []string{
				"Manufacture fast-selling T1 Drones (Hobgoblin I, Warrior I) and Ammunition in Jita/Perimeter.",
				"Run small-ticket Jita 4-4 Station Trading flips with >10% net margin after 2x Broker Fee + Sales Tax.",
				"Keep at least 15–20% liquid ISK reserve to avoid running out of order/relist capital.",
			},
			PrioritySkills: []string{
				"Accounting IV (reduces Sales Tax)",
				"Broker Relations IV (reduces Broker Fee)",
				"Mass Production IV (unlocks 5 manufacturing slots)",
				"Trade III & Retail III (more active market orders)",
			},
			RiskRules: []string{
				"Never lock more than 20% of total capital into a single item.",
				"Avoid slow-moving items (<20 daily Jita volume) and expensive BPCs.",
			},
		}
	case netWorthISK < 1_000_000_000:
		return CapitalPhase{
			TierNumber:   2,
			Name:         "First Billion Builder",
			RangeLabel:   "100M – 1B ISK",
			FocusSummary: "Fill all 8–10 manufacturing slots continuously and add compact inter-hub arbitrage.",
			Strategies: []string{
				"Scale T1 Frigates (Venture, Rifter, Tristan), Destroyers (Catalyst, Thrasher), and Core Modules using ME 10 BPOs.",
				"Use Opportunity Scanner -> Inter-Hub Arbitrage to haul compact T2 modules/drones from Jita to Amarr/Dodixie.",
				"Buy minerals via Jita Buy Orders instead of Jita Sell to boost manufacturing margins by 4–8%.",
			},
			PrioritySkills: []string{
				"Advanced Mass Production IV (up to 9–10 manufacturing slots)",
				"Accounting V (max Sales Tax reduction)",
				"Supply Chain Management III (remote job management)",
				"Gallente/Caldari Hauler IV (safe high-sec hauling)",
			},
			RiskRules: []string{
				"Keep hauling cargo value under 150M–250M ISK per T1 industrial trip to deter high-sec ganks.",
				"Check Relist & Undercut Guard before dropping prices in Jita.",
			},
		}
	case netWorthISK < 10_000_000_000:
		return CapitalPhase{
			TierNumber:   3,
			Name:         "Industrial Scale-Up",
			RangeLabel:   "1B – 10B ISK",
			FocusSummary: "Transition into T2 Manufacturing/Invention, Engineering Complexes (Raitaru/Azbel), and regional portfolios.",
			Strategies: []string{
				"Move production into low-tax Raitaru/Azbel structures with ME/TE rigs to beat NPC station competitors.",
				"Diversify across 15–25 active product lines and regional hubs (Jita + Amarr + Dodixie).",
				"Begin T2 module & drone invention pipelines using Blueprint SDE Mode to verify ISK/Hour per slot.",
			},
			PrioritySkills: []string{
				"Broker Relations V & Caldari Navy Standings (lowest Jita broker fees)",
				"Science V, Metallurgy IV, Research IV (T2 invention & BPO research)",
				"Wholesale IV & Margin Trading",
			},
			RiskRules: []string{
				"Diversify across at least 3 item categories so a single market crash cannot stall cashflow.",
				"Track weekly net worth snapshots to verify true compounding velocity.",
			},
		}
	case netWorthISK < 100_000_000_000:
		return CapitalPhase{
			TierNumber:   4,
			Name:         "Market Empire",
			RangeLabel:   "10B – 100B ISK",
			FocusSummary: "Compound at scale through multi-character industry slots, component chains, and high-value trading.",
			Strategies: []string{
				"Run multi-character manufacturing & science slots (20–30+ concurrent jobs) synced via EVE SSO.",
				"Manufacture T2 ships, rigs, and intermediate components where Make-vs-Buy favors vertical integration.",
				"Maintain multi-hub regional market making across all 5 Empire hubs.",
			},
			PrioritySkills: []string{
				"Tycoon IV–V (hundreds of concurrent market orders)",
				"Advanced Industry V & Reactions V",
				"Jump Freighter / Courier Contract logistics",
			},
			RiskRules: []string{
				"Use collateralized courier contracts for high-value hauls (>1B ISK).",
				"Cap exposure per market segment at 15% of total net worth.",
			},
		}
	default:
		return CapitalPhase{
			TierNumber:   5,
			Name:         "100B+ New Eden Tycoon",
			RangeLabel:   "100B+ ISK",
			FocusSummary: "Capital preservation, institutional-grade regional supply chains, and high-margin industrial pipelines.",
			Strategies: []string{
				"Operate diversified T2/T3/Capital component production lines across multiple industrial alts.",
				"Combine long-term patch/speculation positions with steady high-velocity Jita cashflow.",
				"Continuously prune low ISK/Hour lines in favor of top-quartile slot efficiency.",
			},
			PrioritySkills: []string{
				"Maxed Trade & Industry across dedicated pilot roster",
				"Corporation / Alliance logistics & structure management",
			},
			RiskRules: []string{
				"Keep 15–20% liquid ISK buffer for opportunistic market dips.",
				"Monitor daily net worth drawdowns and slow-mover aging weekly.",
			},
		}
	}
}

// BuildWealthMilestones computes progress across the 100M -> 1B -> 10B -> 50B -> 100B ISK ladder.
func BuildWealthMilestones(currentNetWorth float64) ([]WealthMilestone, WealthMilestone) {
	targets := []struct {
		label  string
		target float64
	}{
		{"100M ISK", 100_000_000},
		{"1B ISK", 1_000_000_000},
		{"10B ISK", 10_000_000_000},
		{"50B ISK", 50_000_000_000},
		{"100B ISK", 100_000_000_000},
	}

	milestones := make([]WealthMilestone, 0, len(targets))
	var next WealthMilestone
	foundNext := false

	for _, t := range targets {
		pct := 0.0
		if t.target > 0 && currentNetWorth > 0 {
			pct = math.Min(100.0, RoundPercent((currentNetWorth/t.target)*100.0))
		}
		rem := math.Max(0, RoundISK(t.target-currentNetWorth))
		reached := currentNetWorth >= t.target
		m := WealthMilestone{
			Label:       t.label,
			TargetISK:   t.target,
			ProgressPct: pct,
			Remaining:   rem,
			Reached:     reached,
		}
		milestones = append(milestones, m)
		if !reached && !foundNext {
			next = m
			foundNext = true
		}
	}

	if !foundNext && len(milestones) > 0 {
		next = milestones[len(milestones)-1]
	}
	return milestones, next
}

// AggregateTopProfitItems summarizes realized profit by item from SOLD batches.
func AggregateTopProfitItems(batches []BatchRecord, limit int) []ItemProfitBreakdown {
	byItem := make(map[string]*ItemProfitBreakdown)
	for _, b := range batches {
		if b.Status != BatchStatusSold {
			continue
		}
		acc, ok := byItem[b.ItemName]
		if !ok {
			acc = &ItemProfitBreakdown{ItemName: b.ItemName}
			byItem[b.ItemName] = acc
		}
		acc.BatchesSold++
		acc.UnitsSold += b.UnitsOutput
		acc.TotalCost = RoundISK(acc.TotalCost + b.ActualProductionCost)
		acc.TotalProfit = RoundISK(acc.TotalProfit + b.NetProfit)
	}

	out := make([]ItemProfitBreakdown, 0, len(byItem))
	for _, acc := range byItem {
		if acc.TotalCost > 0 {
			acc.AvgROIPct = RoundPercent((acc.TotalProfit / acc.TotalCost) * 100.0)
		}
		out = append(out, *acc)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].TotalProfit > out[j].TotalProfit
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
