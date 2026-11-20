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
	report := TrapReport{
		Status:      TrapStatusLayak,
		RiskLevel:   RiskLow,
		Reasons:     make([]string, 0),
		EvaluatedAt: time.Now(),
	}

	if availableCapital <= 0 {
		availableCapital = settings.StartingCapital
	}

	// 1. Check Non-Manufacturable
	if DetectNonManufacturable(item) {
		report.IsNonManufacturable = true
		report.Status = TrapStatusJebakan
		report.RiskLevel = RiskHigh
		report.Reasons = append(report.Reasons, "NON-MANUFAKTUR: Item ini adalah komoditas mentah / Compressed Ice, bukan hasil manufaktur blueprint.")
	}

	// 2. Check Rare BPC Trap
	if DetectRareBPC(item) && item.ExtraSetupCost <= 0 {
		report.IsBPCJebakan = true
		report.Status = TrapStatusJebakan
		report.RiskLevel = RiskCritical
		report.Reasons = append(report.Reasons, "JEBAKAN BPC LANGKA: Item membutuhkan BPC khusus (Faction/Deadspace/Storyline/Officer/Abyssal), tetapi biaya ExtraSetupCost masih 0 ISK.")
	}

	// 3. Calculate Net Profit
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
		// Check negative or tiny profit
		if !calcRes.IsProfitable {
			report.IsProfitReceh = true
			report.Status = TrapStatusJebakan
			report.RiskLevel = RiskCritical
			report.Reasons = append(report.Reasons, fmt.Sprintf("RUGI BERSIH: Estimasi Net Profit negatif (%.2f ISK) setelah pajak & broker fee.", calcRes.NetProfitPerBatch))
		} else if calcRes.NetProfitPerBatch < MinAcceptableBatchProfitISK || calcRes.NetMarginPercent < MinAcceptableMarginPercent {
			report.IsProfitReceh = true
			if report.Status != TrapStatusJebakan {
				report.Status = TrapStatusWaspadaNiche
				report.RiskLevel = maxRisk(report.RiskLevel, RiskMedium)
			}
			report.Reasons = append(report.Reasons, fmt.Sprintf("PROFIT RECEH: Net profit %.0f ISK (margin %.2f%%) terlalu tipis dibanding biaya slot & risiko relist.", calcRes.NetProfitPerBatch, calcRes.NetMarginPercent))
		}

		// 4. Check Capital Lock-Up
		if availableCapital > 0 {
			if calcRes.TotalDirectCost > availableCapital {
				report.IsModalLockup = true
				if report.Status != TrapStatusJebakan {
					report.Status = TrapStatusWaspadaModal
					report.RiskLevel = maxRisk(report.RiskLevel, RiskHigh)
				}
				report.Reasons = append(report.Reasons, fmt.Sprintf("MODAL TIDAK CUKUP: Biaya bahan %.0f ISK melebihi kas tersedia (%.0f ISK).", calcRes.TotalDirectCost, availableCapital))
			} else if calcRes.TotalDirectCost > availableCapital*MaxSingleBatchCapitalRatio {
				report.IsModalLockup = true
				if report.Status != TrapStatusJebakan {
					report.Status = TrapStatusWaspadaModal
					report.RiskLevel = maxRisk(report.RiskLevel, RiskMedium)
				}
				report.Reasons = append(report.Reasons, fmt.Sprintf("LOCK-UP MODAL: Satu batch mengunci %.1f%% dari total kas tersedia (batas aman 50%%).", (calcRes.TotalDirectCost/availableCapital)*100.0))
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
		report.Reasons = append(report.Reasons, fmt.Sprintf("SLOW MOVER: Volume harian Jita (%.1f unit/hari) terlalu rendah dibanding output batch (%d unit).", item.DailyVolume, outputQty))
	}

	// 6. Generate Human-Readable Recommendation
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
			return "HINDARI atau masukkan harga beli BPC asli ke kolom ExtraSetupCost sebelum menghitung ulang profit."
		}
		if r.IsNonManufacturable {
			return "HINDARI untuk manufaktur. Gunakan item ini hanya untuk trading/hauling jika memang diinginkan."
		}
		return "HINDARI PRODUKSI: Item berisiko rugi bersih setelah pajak Jita dan broker fee."
	case TrapStatusWaspadaModal:
		return "TUNDA HINGGA MODAL CUKUP: Produksi item Fast Cashflow terlebih dahulu sampai modal aman (<50% kas per batch)."
	case TrapStatusWaspadaNiche:
		if r.IsSlowMover {
			return "PRODUKSI TERBATAS: Jalankan maksimal 1 batch kecil dan pantau persaingan harga Jita agar tidak nyangkut."
		}
		return "OPTIMASI BIAYA: Beli bahan baku via Buy Order atau tingkatkan ME blueprint untuk menaikkan margin."
	default:
		return "LAYAK PRODUKSI: Margin sehat, likuiditas Jita memadai, dan aman untuk kapasitas modal saat ini."
	}
}
