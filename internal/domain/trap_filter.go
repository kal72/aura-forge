package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	// MinAcceptableBatchProfitISK defines the floor profit per batch to avoid wasting industry slots.
	MinAcceptableBatchProfitISK = 500_000.0
	// MinAcceptableMarginPercent defines the minimum net margin percentage.
	MinAcceptableMarginPercent = 5.0
	// MaxSingleBatchCapitalRatio defines the maximum safe share of available capital in one batch.
	MaxSingleBatchCapitalRatio = 0.50
	// MinDailyVolumeRatio defines minimum daily Jita volume relative to 1 batch output.
	MinDailyVolumeRatio = 1.0
	// MaxSafePriceSpikePercent defines the threshold above 7-day average sell price that flags a fake price spike trap.
	MaxSafePriceSpikePercent = 25.0
)

var rareBPCKeywords = []string{
	"faction", "deadspace", "storyline", "officer", "subverted", "abyssal",
	"caldari navy", "imperial navy", "federation navy", "republic fleet",
	"true sansha", "dark blood", "dread guristas", "shadow serpentis", "domination",
	"a-type", "b-type", "c-type", "x-type", "pithum", "gistum", "corpii", "centii", "coreli",
}

var nonManufacturableKeywords = []string{
	"compressed ", "plex", "skill injector", "skill extractor",
}

// DetectRareBPC checks whether an item name or flag indicates a rare/faction/deadspace BPC.
func DetectRareBPC(item MarketItem) bool {
	if item.RequiresRareBPC {
		return true
	}
	lowerName := strings.ToLower(strings.TrimSpace(item.Name))
	for _, kw := range rareBPCKeywords {
		if strings.Contains(lowerName, kw) {
			return true
		}
	}
	return false
}

// DetectNonManufacturable checks whether an item is a raw commodity / compressed ice rather than manufactured.
func DetectNonManufacturable(item MarketItem) bool {
	if !item.IsManufacturable {
		return true
	}
	if item.Category == CategoryCommodities {
		return true
	}
	lowerName := strings.ToLower(strings.TrimSpace(item.Name))
	for _, kw := range nonManufacturableKeywords {
		if strings.Contains(lowerName, kw) {
			return true
		}
	}
	return false
}

// EvaluateTrapGuard inspects a candidate MarketItem against market rules, liquidity, and available capital.
func EvaluateTrapGuard(item MarketItem, settings UserSettings, availableCapital float64) TrapReport {
	return EvaluateTrapGuardWithHistory(item, settings, availableCapital, item.TrapAnalysis.Avg7dPrice)
}

// EvaluateTrapGuardWithHistory evaluates a MarketItem including optional 7-day ESI Market History
// average price to detect manipulated Jita price spikes alongside standard trap checks.
func EvaluateTrapGuardWithHistory(item MarketItem, settings UserSettings, availableCapital float64, avg7dPrice float64) TrapReport {
	report := TrapReport{
		Status:           TrapStatusLayak,
		RiskLevel:        RiskLow,
		Reasons:          make([]string, 0),
		Avg7dPrice:       avg7dPrice,
		BOMSyncedFromSDE: item.TrapAnalysis.BOMSyncedFromSDE,
		EvaluatedAt:      time.Now(),
	}

	if availableCapital <= 0 {
		availableCapital = settings.StartingCapital
	}

	// 1. Check Non-Manufacturable
	if DetectNonManufacturable(item) {
		report.IsNonManufacturable = true
		report.Status = TrapStatusJebakan
		report.RiskLevel = RiskHigh
		report.Reasons = append(report.Reasons, "NON-MANUFACTURABLE: Raw commodity or compressed ice — cannot be manufactured from a blueprint.")
	}

	// 2. Check Rare BPC Trap
	if DetectRareBPC(item) && item.ExtraSetupCost <= 0 {
		report.IsBPCJebakan = true
		report.Status = TrapStatusJebakan
		report.RiskLevel = RiskCritical
		report.Reasons = append(report.Reasons, "RARE BPC TRAP: Requires a Faction/Deadspace/Storyline/Officer/Abyssal BPC, but ExtraSetupCost is still 0 ISK.")
	}

	// 3. Calculate Net Profit, Margin %, and ROI %
	outputQty := item.BatchOutputQty
	if outputQty <= 0 {
		outputQty = 1
	}

	calcIn := ProfitCalcInput{
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
	}

	calcRes, err := CalculateProfit(calcIn)
	if err == nil {
		report.EstimatedNetProfit = calcRes.NetProfitPerBatch
		report.EstimatedMarginPct = calcRes.NetMarginPercent
		report.EstimatedROIPct = calcRes.ROIPercent

		// Check negative or tiny profit
		if !calcRes.IsProfitable {
			report.IsProfitReceh = true
			report.Status = TrapStatusJebakan
			report.RiskLevel = RiskCritical
			report.Reasons = append(report.Reasons, fmt.Sprintf("NET LOSS: Estimated profit is negative (%.0f ISK) after Jita taxes, SCI, and broker fees.", calcRes.NetProfitPerBatch))
		} else if calcRes.NetProfitPerBatch < MinAcceptableBatchProfitISK || calcRes.NetMarginPercent < MinAcceptableMarginPercent {
			report.IsProfitReceh = true
			if report.Status != TrapStatusJebakan {
				report.Status = TrapStatusWaspadaNiche
				report.RiskLevel = maxRisk(report.RiskLevel, RiskMedium)
			}
			report.Reasons = append(report.Reasons, fmt.Sprintf("THIN MARGIN: Net profit %.0f ISK (%.2f%% margin) is too thin to absorb relist fees or undercuts.", calcRes.NetProfitPerBatch, calcRes.NetMarginPercent))
		}

		// 4. Check Capital Lock-Up
		if availableCapital > 0 {
			if calcRes.TotalDirectCost > availableCapital {
				report.IsModalLockup = true
				if report.Status != TrapStatusJebakan {
					report.Status = TrapStatusWaspadaModal
					report.RiskLevel = maxRisk(report.RiskLevel, RiskHigh)
				}
				report.Reasons = append(report.Reasons, fmt.Sprintf("INSUFFICIENT CAPITAL: Direct batch cost %.0f ISK exceeds available capital (%.0f ISK).", calcRes.TotalDirectCost, availableCapital))
			} else if calcRes.TotalDirectCost > availableCapital*MaxSingleBatchCapitalRatio {
				report.IsModalLockup = true
				if report.Status != TrapStatusJebakan {
					report.Status = TrapStatusWaspadaModal
					report.RiskLevel = maxRisk(report.RiskLevel, RiskMedium)
				}
				report.Reasons = append(report.Reasons, fmt.Sprintf("CAPITAL LOCK-UP: One batch locks %.1f%% of available capital (safe cap: 50%%).", (calcRes.TotalDirectCost/availableCapital)*100.0))
			}
		}
	}

	// 5. Check Low Liquidity (Slow Mover)
	minExpectedVol := float64(outputQty) * MinDailyVolumeRatio
	if item.DailyVolume <= 0 || item.DailyVolume < minExpectedVol {
		report.IsSlowMover = true
		if report.Status == TrapStatusLayak {
			report.Status = TrapStatusWaspadaNiche
			report.RiskLevel = maxRisk(report.RiskLevel, RiskMedium)
		} else if report.Status != TrapStatusJebakan {
			report.RiskLevel = maxRisk(report.RiskLevel, RiskHigh)
		}
		report.Reasons = append(report.Reasons, fmt.Sprintf("SLOW MOVER: Daily Jita volume (%.1f u/day) is lower than 1 batch output (%d u).", item.DailyVolume, outputQty))
	}

	// 6. Check Fake Price Spike Trap (Live Lowest Sell >= +25% above 7d average)
	if avg7dPrice > 0 && item.EstimatedJitaSell > 0 {
		spikePct := ((item.EstimatedJitaSell - avg7dPrice) / avg7dPrice) * 100.0
		report.PriceSpikePct = spikePct
		if spikePct >= MaxSafePriceSpikePercent {
			report.IsPriceSpike = true
			if report.Status == TrapStatusLayak {
				report.Status = TrapStatusWaspadaNiche
				report.RiskLevel = maxRisk(report.RiskLevel, RiskMedium)
			} else if report.Status != TrapStatusJebakan {
				report.RiskLevel = maxRisk(report.RiskLevel, RiskHigh)
			}
			report.Reasons = append(report.Reasons, fmt.Sprintf("PRICE SPIKE TRAP: Live Jita Sell is +%.1f%% above its 7-day average — margin may collapse before your batch sells.", spikePct))
		}
	}

	// 7. Generate Human-Readable Recommendation
	report.Recommendation = buildTrapRecommendation(report)

	return report
}

func maxRisk(a, b RiskLevel) RiskLevel {
	rank := map[RiskLevel]int{
		RiskLow:      1,
		RiskMedium:   2,
		RiskHigh:     3,
		RiskCritical: 4,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func buildTrapRecommendation(r TrapReport) string {
	switch r.Status {
	case TrapStatusJebakan:
		if r.IsBPCJebakan {
			return "AVOID or edit this item to include the real BPC contract price in ExtraSetupCost before re-evaluating."
		}
		if r.IsNonManufacturable {
			return "AVOID for manufacturing. Use this item only for station trading or inter-hub hauling."
		}
		return "AVOID PRODUCTION: Projected net loss after Jita taxes, SCI, and broker fees."
	case TrapStatusWaspadaModal:
		return "HOLD UNTIL CAPITAL GROWS: Build fast-cashflow items first until one batch uses <50% of available capital."
	case TrapStatusWaspadaNiche:
		if r.IsPriceSpike {
			return "VERIFY ORDER BOOK FIRST: Live Jita price spiked sharply vs 7d average — check Marketplace before buying materials."
		}
		if r.IsSlowMover {
			return "LIMITED PRODUCTION: Run at most 1 small batch and monitor Jita competition closely to avoid stuck stock."
		}
		return "OPTIMIZE COST: Buy materials via Jita Buy Orders or manufacture in a rigged Raitaru to lift margins."
	default:
		return "VIABLE TO BUILD: Healthy net margin, strong Jita liquidity, and safe for your current capital."
	}
}
