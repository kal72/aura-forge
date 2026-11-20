package domain

import (
	"fmt"
	"time"
)

// BatchStatus tracks the lifecycle of an individual production lot.
type BatchStatus string

const (
	BatchStatusDraft        BatchStatus = "DRAFT"
	BatchStatusInProduction BatchStatus = "IN_PRODUCTION"
	BatchStatusListed       BatchStatus = "LISTED"
	BatchStatusSold         BatchStatus = "SOLD"
	BatchStatusSlowMover    BatchStatus = "SLOW_MOVER"
)

// BatchRecord represents a tracked manufacturing batch from blueprint to market execution.
type BatchRecord struct {
	ID                   int64       `json:"id"`
	PlanID               *int64      `json:"plan_id,omitempty"`
	TypeID               int32       `json:"type_id"`
	ItemName             string      `json:"item_name"`
	Runs                 int         `json:"runs"`
	UnitsOutput          int         `json:"units_output"`
	TotalMaterialCost    float64     `json:"total_material_cost"`
	ActualProductionCost float64     `json:"actual_production_cost"`
	ActualSellPrice      float64     `json:"actual_sell_price"`
	TotalRevenue         float64     `json:"total_revenue"`
	NetProfit            float64     `json:"net_profit"`
	ROI                  float64     `json:"roi"`
	Status               BatchStatus `json:"status"`
	MarketOrderID        *int64      `json:"market_order_id,omitempty"`
	CurrentMarketPrice   float64     `json:"current_market_price"`
	IsUndercut           bool        `json:"is_undercut"`
	SafeRelistPrice      float64     `json:"safe_relist_price"`
	RemainingSafeMargin  float64     `json:"remaining_safe_margin"`
	StartedAt            *time.Time  `json:"started_at,omitempty"`
	FinishedAt           *time.Time  `json:"finished_at,omitempty"`
	ListedAt             *time.Time  `json:"listed_at,omitempty"`
	SoldAt               *time.Time  `json:"sold_at,omitempty"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
}

// RelistGuardResult summarizes the undercut check and safe relist calculation for a listed batch.
type RelistGuardResult struct {
	CurrentPrice          float64 `json:"current_price"`
	ProposedPrice         float64 `json:"proposed_price"`
	SafeRelistPrice       float64 `json:"safe_relist_price"`
	EstimatedRelistCost   float64 `json:"estimated_relist_cost"`
	ProjectedNetProfit    float64 `json:"projected_net_profit"`
	RemainingSafeMargin   float64 `json:"remaining_safe_margin"`
	IsUndercut            bool    `json:"is_undercut"`
	IsSafeToRelist        bool    `json:"is_safe_to_relist"`
	Advice                string  `json:"advice"`
}

// EvaluateRelistGuard calculates whether lowering a Sell Order price to match/undercut a competitor remains profitable.
func EvaluateRelistGuard(batch BatchRecord, proposedUnitPrice float64, settings UserSettings) RelistGuardResult {
	units := batch.UnitsOutput
	if units <= 0 {
		units = 1
	}

	directCost := batch.ActualProductionCost
	if directCost <= 0 {
		directCost = batch.TotalMaterialCost
	}

	safeRelist := CalculateSafeRelistPrice(directCost, units, settings.SalesTaxPercent, settings.RelistFeePercent)
	grossAtProposed := proposedUnitPrice * float64(units)
	salesTax := grossAtProposed * (settings.SalesTaxPercent / 100.0)
	relistCost := grossAtProposed * (settings.RelistFeePercent / 100.0)
	// Include initial broker fee approximation if already paid on original sell price
	initialBrokerFee := (batch.ActualSellPrice * float64(units)) * (settings.BrokerFeePercent / 100.0)
	projectedNet := grossAtProposed - directCost - salesTax - relistCost - initialBrokerFee

	remainingMargin := RoundISK((proposedUnitPrice - safeRelist) * float64(units))
	isUndercut := proposedUnitPrice > 0 && batch.ActualSellPrice > 0 && proposedUnitPrice < batch.ActualSellPrice
	isSafe := proposedUnitPrice >= safeRelist && projectedNet > 0

	var advice string
	if isSafe {
		advice = fmt.Sprintf("AMAN RELIST: Masih menyisakan estimasi profit %.2f ISK (batas aman: %.2f ISK/unit).", projectedNet, safeRelist)
	} else {
		advice = fmt.Sprintf("TAHAN HARGA / JANGAN RELIST: Harga %.2f ISK menembus batas aman %.2f ISK/unit dan berisiko rugi %.2f ISK.", proposedUnitPrice, safeRelist, projectedNet)
	}

	return RelistGuardResult{
		CurrentPrice:        RoundISK(batch.ActualSellPrice),
		ProposedPrice:       RoundISK(proposedUnitPrice),
		SafeRelistPrice:     safeRelist,
		EstimatedRelistCost: RoundISK(relistCost),
		ProjectedNetProfit:  RoundISK(projectedNet),
		RemainingSafeMargin: remainingMargin,
		IsUndercut:          isUndercut,
		IsSafeToRelist:      isSafe,
		Advice:              advice,
	}
}
