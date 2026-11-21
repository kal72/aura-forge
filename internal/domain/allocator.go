package domain

import (
	"errors"
	"sort"
)

// BucketType defines the four capital allocation baskets.
type BucketType string

const (
	BucketFastCashflow BucketType = "FAST_CASHFLOW"
	BucketMidTier      BucketType = "MID_TIER"
	BucketHighROI      BucketType = "HIGH_ROI"
	BucketCashBuffer   BucketType = "CASH_BUFFER"
)

const (
	DefaultFastCashflowPct = 35.0
	DefaultMidTierPct      = 30.0
	DefaultHighROIPct      = 20.0
	DefaultCashBufferPct   = 15.0
	MinCashBufferPct       = 10.0
)

var (
	ErrInvalidCapital       = errors.New("available capital must be greater than zero")
	ErrBufferBelowMinimum   = errors.New("cash buffer percentage must be at least 10%")
	ErrInvalidBucketWeights = errors.New("bucket allocation percentages must sum to 100%")
)

// AllocationConfig configures the split percentages across the 4 buckets.
type AllocationConfig struct {
	FastCashflowPct float64 `json:"fast_cashflow_pct"`
	MidTierPct      float64 `json:"mid_tier_pct"`
	HighROIPct      float64 `json:"high_roi_pct"`
	CashBufferPct   float64 `json:"cash_buffer_pct"`
}

// DefaultAllocationConfig returns the recommended 35/30/20/15 beginner industrialist split.
func DefaultAllocationConfig() AllocationConfig {
	return AllocationConfig{
		FastCashflowPct: DefaultFastCashflowPct,
		MidTierPct:      DefaultMidTierPct,
		HighROIPct:      DefaultHighROIPct,
		CashBufferPct:   DefaultCashBufferPct,
	}
}

// AllocatedItem represents an item recommendation inside an allocation bucket.
type AllocatedItem struct {
	TypeID          int32   `json:"type_id"`
	Name            string  `json:"name"`
	RecommendedRuns int     `json:"recommended_runs"`
	BatchCost       float64 `json:"batch_cost"`
	TotalCost       float64 `json:"total_cost"`
	EstimatedProfit float64 `json:"estimated_profit"`
	ROIPercent      float64 `json:"roi_percent"`
	DailyVolume     float64 `json:"daily_volume"`
}

// CapitalBucket represents one of the 4 allocation buckets with its budget and assigned items.
type CapitalBucket struct {
	Type            BucketType      `json:"type"`
	Label           string          `json:"label"`
	Description     string          `json:"description"`
	Percent         float64         `json:"percent"`
	BudgetISK       float64         `json:"budget_isk"`
	UsedISK         float64         `json:"used_isk"`
	RemainingISK    float64         `json:"remaining_isk"`
	EstimatedProfit float64         `json:"estimated_profit"`
	Items           []AllocatedItem `json:"items"`
}

// AllocationResult summarizes the full 4-bucket smart capital allocation.
type AllocationResult struct {
	TotalCapital         float64         `json:"total_capital"`
	CapitalSource        string          `json:"capital_source"`
	ActiveHubName        string          `json:"active_hub_name,omitempty"`
	DeployableCapital    float64         `json:"deployable_capital"`
	CashBufferISK        float64         `json:"cash_buffer_isk"`
	TotalAllocatedISK    float64         `json:"total_allocated_isk"`
	TotalEstimatedProfit float64         `json:"total_estimated_profit"`
	Buckets              []CapitalBucket `json:"buckets"`
}

// ResolveAvailableCapital selects live wallet balance when SSO is active, otherwise manual starting capital.
func ResolveAvailableCapital(settings UserSettings, profile *CharacterProfile, ssoEnabled bool) (float64, string) {
	if ssoEnabled && profile != nil && profile.WalletBalance > 0 {
		return RoundISK(profile.WalletBalance), "EVE SSO Wallet (" + profile.CharacterName + ")"
	}
	return RoundISK(settings.StartingCapital), "Manual Settings"
}

// AllocateCapital divides available capital into 4 buckets and assigns eligible catalog items.
func AllocateCapital(
	totalCapital float64,
	capitalSource string,
	cfg AllocationConfig,
	candidates []MarketItem,
	settings UserSettings,
) (AllocationResult, error) {
	if totalCapital <= 0 {
		return AllocationResult{}, ErrInvalidCapital
	}
	if cfg.CashBufferPct < MinCashBufferPct {
		return AllocationResult{}, ErrBufferBelowMinimum
	}

	sumPct := RoundPercent(cfg.FastCashflowPct + cfg.MidTierPct + cfg.HighROIPct + cfg.CashBufferPct)
	if sumPct < 99.99 || sumPct > 100.01 {
		return AllocationResult{}, ErrInvalidBucketWeights
	}

	fastBudget := RoundISK(totalCapital * (cfg.FastCashflowPct / 100.0))
	midBudget := RoundISK(totalCapital * (cfg.MidTierPct / 100.0))
	highBudget := RoundISK(totalCapital * (cfg.HighROIPct / 100.0))
	bufferBudget := RoundISK(totalCapital - (fastBudget + midBudget + highBudget))

	fastBucket := CapitalBucket{
		Type:         BucketFastCashflow,
		Label:        "Fast Cashflow (Daily Turnover)",
		Description:  "High daily volume, low-cost items for steady daily ISK velocity.",
		Percent:      cfg.FastCashflowPct,
		BudgetISK:    fastBudget,
		RemainingISK: fastBudget,
		Items:        make([]AllocatedItem, 0),
	}
	midBucket := CapitalBucket{
		Type:         BucketMidTier,
		Label:        "Mid-Tier (Balanced Profit)",
		Description:  "Balanced sales velocity and reliable profit margins.",
		Percent:      cfg.MidTierPct,
		BudgetISK:    midBudget,
		RemainingISK: midBudget,
		Items:        make([]AllocatedItem, 0),
	}
	highBucket := CapitalBucket{
		Type:         BucketHighROI,
		Label:        "High ROI (Capital Accelerator)",
		Description:  "Higher ROI or profit-per-batch items to accelerate capital compounding.",
		Percent:      cfg.HighROIPct,
		BudgetISK:    highBudget,
		RemainingISK: highBudget,
		Items:        make([]AllocatedItem, 0),
	}
	bufferBucket := CapitalBucket{
		Type:         BucketCashBuffer,
		Label:        "Operational Cash Buffer",
		Description:  "Reserved 10%–15% safety cushion for Broker Fees, Job Install Costs, and Relists.",
		Percent:      cfg.CashBufferPct,
		BudgetISK:    bufferBudget,
		RemainingISK: bufferBudget,
		Items:        make([]AllocatedItem, 0),
	}

	// Evaluate & score eligible candidates
	type scoredCandidate struct {
		item      MarketItem
		batchCost float64
		netProfit float64
		roi       float64
	}

	var eligible []scoredCandidate
	for _, item := range candidates {
		report := EvaluateTrapGuard(item, settings, totalCapital)
		if report.Status == TrapStatusJebakan || report.IsNonManufacturable {
			continue
		}
		outputQty := item.BatchOutputQty
		if outputQty <= 0 {
			outputQty = 1
		}
		res, err := CalculateProfit(ProfitCalcInput{
			ItemName:           item.Name,
			OutputQty:          outputQty,
			UnitSellPrice:      item.EstimatedJitaSell,
			MaterialCost:       item.MaterialCost,
			ExtraSetupCost:     item.ExtraSetupCost,
			SystemCostIndex:    settings.SystemCostIndex,
			FacilityTaxPercent: settings.FacilityTaxPercent,
			BrokerFeePercent:   settings.BrokerFeePercent,
			SalesTaxPercent:    settings.SalesTaxPercent,
			RelistFeePercent:   settings.RelistFeePercent,
			EstimatedRelists:   1,
			BatchesPerWeek:     1,
		})
		if err != nil || !res.IsProfitable || res.TotalDirectCost <= 0 {
			continue
		}
		eligible = append(eligible, scoredCandidate{
			item:      item,
			batchCost: res.TotalDirectCost,
			netProfit: res.NetProfitPerBatch,
			roi:       res.ROIPercent,
		})
	}

	// Classify candidates into buckets:
	// Fast Cashflow: high daily volume (>= 50/day) or frequently destroyed, affordable cost
	// High ROI: ROI >= 20% or highest ROI candidates
	// Mid-Tier: remaining profitable candidates
	var fastPool, midPool, highPool []scoredCandidate
	for _, c := range eligible {
		if c.item.DailyVolume >= 50 || c.item.IsFrequentlyDestroyed {
			fastPool = append(fastPool, c)
		} else if c.roi >= 20.0 {
			highPool = append(highPool, c)
		} else {
			midPool = append(midPool, c)
		}
	}

	// Sort each pool by efficiency
	sort.Slice(fastPool, func(i, j int) bool {
		return fastPool[i].item.DailyVolume > fastPool[j].item.DailyVolume
	})
	sort.Slice(midPool, func(i, j int) bool {
		return midPool[i].netProfit > midPool[j].netProfit
	})
	sort.Slice(highPool, func(i, j int) bool {
		return highPool[i].roi > highPool[j].roi
	})

	fillBucket := func(bucket *CapitalBucket, pool []scoredCandidate) {
		for _, c := range pool {
			if c.batchCost <= 0 || c.batchCost > bucket.RemainingISK {
				continue
			}
			// Cap runs per item to avoid over-concentration (max 5 batches per item)
			maxRuns := int(bucket.RemainingISK / c.batchCost)
			if maxRuns <= 0 {
				continue
			}
			if maxRuns > 5 {
				maxRuns = 5
			}
			totalCost := RoundISK(c.batchCost * float64(maxRuns))
			estProfit := RoundISK(c.netProfit * float64(maxRuns))

			bucket.Items = append(bucket.Items, AllocatedItem{
				TypeID:          c.item.TypeID,
				Name:            c.item.Name,
				RecommendedRuns: maxRuns,
				BatchCost:       c.batchCost,
				TotalCost:       totalCost,
				EstimatedProfit: estProfit,
				ROIPercent:      c.roi,
				DailyVolume:     c.item.DailyVolume,
			})
			bucket.UsedISK = RoundISK(bucket.UsedISK + totalCost)
			bucket.RemainingISK = RoundISK(bucket.BudgetISK - bucket.UsedISK)
			bucket.EstimatedProfit = RoundISK(bucket.EstimatedProfit + estProfit)
		}
	}

	fillBucket(&fastBucket, fastPool)
	fillBucket(&midBucket, midPool)
	fillBucket(&highBucket, highPool)

	// If midBucket or highBucket had no specific pool items, allow fallback from remaining eligible
	if len(midBucket.Items) == 0 && len(fastPool) > 0 {
		fillBucket(&midBucket, fastPool)
	}
	if len(highBucket.Items) == 0 && len(eligible) > 0 {
		sort.Slice(eligible, func(i, j int) bool {
			return eligible[i].roi > eligible[j].roi
		})
		fillBucket(&highBucket, eligible)
	}

	totalAllocated := RoundISK(fastBucket.UsedISK + midBucket.UsedISK + highBucket.UsedISK)
	totalEstProfit := RoundISK(fastBucket.EstimatedProfit + midBucket.EstimatedProfit + highBucket.EstimatedProfit)

	if capitalSource == "" {
		capitalSource = "Manual Settings"
	}

	return AllocationResult{
		TotalCapital:         RoundISK(totalCapital),
		CapitalSource:        capitalSource,
		DeployableCapital:    RoundISK(fastBudget + midBudget + highBudget),
		CashBufferISK:        bufferBudget,
		TotalAllocatedISK:    totalAllocated,
		TotalEstimatedProfit: totalEstProfit,
		Buckets:              []CapitalBucket{fastBucket, midBucket, highBucket, bufferBucket},
	}, nil
}
