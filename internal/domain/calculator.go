package domain

import (
	"errors"
	"math"
)

// ErrInvalidCalculationInput is returned when calculator inputs violate business invariants.
var (
	ErrInvalidOutputQty    = errors.New("output quantity per batch must be at least 1")
	ErrNegativePriceOrCost = errors.New("prices, costs, and fee percentages cannot be negative")
	ErrExcessiveFeeRate    = errors.New("combined market fee rate cannot reach or exceed 100%")
)

// ProfitCalcInput holds all parameters required to compute net profit and break-even price.
type ProfitCalcInput struct {
	ItemName           string  `json:"item_name"`
	OutputQty          int     `json:"output_qty"`
	UnitSellPrice      float64 `json:"unit_sell_price"`
	MaterialCost       float64 `json:"material_cost"`
	ExtraSetupCost     float64 `json:"extra_setup_cost"`
	JobInstallCost     float64 `json:"job_install_cost"`
	SystemCostIndex    float64 `json:"system_cost_index"`
	FacilityTaxPercent float64 `json:"facility_tax_percent"`
	FreightCost        float64 `json:"freight_cost"`
	BrokerFeePercent   float64 `json:"broker_fee_percent"`
	SalesTaxPercent    float64 `json:"sales_tax_percent"`
	RelistFeePercent   float64 `json:"relist_fee_percent"`
	EstimatedRelists   int     `json:"estimated_relists"`
	BatchesPerWeek     int     `json:"batches_per_week"`
}

// ProfitCalcResult contains the complete financial breakdown of a production batch.
type ProfitCalcResult struct {
	GrossRevenue       float64 `json:"gross_revenue"`
	BrokerFee          float64 `json:"broker_fee"`
	SalesTax           float64 `json:"sales_tax"`
	RelistFee          float64 `json:"relist_fee"`
	JobInstallCost     float64 `json:"job_install_cost"`
	TotalDirectCost    float64 `json:"total_direct_cost"`
	TotalMarketFees    float64 `json:"total_market_fees"`
	TotalActualCost    float64 `json:"total_actual_cost"`
	NetProfitPerBatch  float64 `json:"net_profit_per_batch"`
	NetProfitPerUnit   float64 `json:"net_profit_per_unit"`
	NetMarginPercent   float64 `json:"net_margin_percent"`
	ROIPercent         float64 `json:"roi_percent"`
	BreakEvenUnitPrice float64 `json:"break_even_unit_price"`
	WeeklyGrossRevenue float64 `json:"weekly_gross_revenue"`
	WeeklyTotalCost    float64 `json:"weekly_total_cost"`
	WeeklyNetProfit    float64 `json:"weekly_net_profit"`
	IsProfitable       bool    `json:"is_profitable"`
}

// RoundISK rounds an ISK value to 2 decimal places.
func RoundISK(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return math.Round(val*100) / 100
}

// RoundPercent rounds a percentage value to 2 decimal places.
func RoundPercent(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return math.Round(val*100) / 100
}

// CalculateProfit computes net profit, taxes, break-even price, ROI, and weekly projection.
func CalculateProfit(in ProfitCalcInput) (ProfitCalcResult, error) {
	// Guard clauses
	if in.OutputQty <= 0 {
		return ProfitCalcResult{}, ErrInvalidOutputQty
	}
	if in.UnitSellPrice < 0 || in.MaterialCost < 0 || in.ExtraSetupCost < 0 ||
		in.JobInstallCost < 0 || in.FreightCost < 0 ||
		in.BrokerFeePercent < 0 || in.SalesTaxPercent < 0 || in.RelistFeePercent < 0 ||
		in.SystemCostIndex < 0 || in.FacilityTaxPercent < 0 {
		return ProfitCalcResult{}, ErrNegativePriceOrCost
	}

	relists := in.EstimatedRelists
	if relists < 0 {
		relists = 0
	}

	batchesPerWeek := in.BatchesPerWeek
	if batchesPerWeek <= 0 {
		batchesPerWeek = 1
	}

	totalFeeRate := (in.BrokerFeePercent + in.SalesTaxPercent + (in.RelistFeePercent * float64(relists))) / 100.0
	if totalFeeRate >= 1.0 {
		return ProfitCalcResult{}, ErrExcessiveFeeRate
	}

	grossRevenue := in.UnitSellPrice * float64(in.OutputQty)

	// Calculate job install cost from SCI if not explicitly supplied
	jobInstall := in.JobInstallCost
	if jobInstall == 0 && in.SystemCostIndex > 0 && in.MaterialCost > 0 {
		jobInstall = in.MaterialCost * in.SystemCostIndex * (1.0 + in.FacilityTaxPercent/100.0)
	}

	brokerFee := grossRevenue * (in.BrokerFeePercent / 100.0)
	salesTax := grossRevenue * (in.SalesTaxPercent / 100.0)
	relistFee := grossRevenue * (in.RelistFeePercent / 100.0) * float64(relists)

	totalDirectCost := in.MaterialCost + in.ExtraSetupCost + jobInstall + in.FreightCost
	totalMarketFees := brokerFee + salesTax + relistFee
	totalActualCost := totalDirectCost + totalMarketFees

	netProfit := grossRevenue - totalActualCost
	netProfitPerUnit := netProfit / float64(in.OutputQty)

	var netMargin float64
	if grossRevenue > 0 {
		netMargin = (netProfit / grossRevenue) * 100.0
	}

	var roi float64
	if totalDirectCost > 0 {
		roi = (netProfit / totalDirectCost) * 100.0
	}

	// Break-even unit price:
	// GrossRevenue * (1 - totalFeeRate) = TotalDirectCost
	// UnitPrice = TotalDirectCost / (OutputQty * (1 - totalFeeRate))
	breakEvenUnitPrice := totalDirectCost / (float64(in.OutputQty) * (1.0 - totalFeeRate))

	return ProfitCalcResult{
		GrossRevenue:       RoundISK(grossRevenue),
		BrokerFee:          RoundISK(brokerFee),
		SalesTax:           RoundISK(salesTax),
		RelistFee:          RoundISK(relistFee),
		JobInstallCost:     RoundISK(jobInstall),
		TotalDirectCost:    RoundISK(totalDirectCost),
		TotalMarketFees:    RoundISK(totalMarketFees),
		TotalActualCost:    RoundISK(totalActualCost),
		NetProfitPerBatch:  RoundISK(netProfit),
		NetProfitPerUnit:   RoundISK(netProfitPerUnit),
		NetMarginPercent:   RoundPercent(netMargin),
		ROIPercent:         RoundPercent(roi),
		BreakEvenUnitPrice: RoundISK(breakEvenUnitPrice),
		WeeklyGrossRevenue: RoundISK(grossRevenue * float64(batchesPerWeek)),
		WeeklyTotalCost:    RoundISK(totalActualCost * float64(batchesPerWeek)),
		WeeklyNetProfit:    RoundISK(netProfit * float64(batchesPerWeek)),
		IsProfitable:       netProfit > 0,
	}, nil
}

// CalculateSafeRelistPrice determines the minimum sell price per unit that avoids losing ISK
// after accounting for sales tax and one additional relist fee.
func CalculateSafeRelistPrice(totalDirectCost float64, outputQty int, salesTaxPercent, relistFeePercent float64) float64 {
	if outputQty <= 0 || totalDirectCost <= 0 {
		return 0
	}
	feeRate := (salesTaxPercent + relistFeePercent) / 100.0
	if feeRate >= 1.0 {
		return 0
	}
	return RoundISK(totalDirectCost / (float64(outputQty) * (1.0 - feeRate)))
}
