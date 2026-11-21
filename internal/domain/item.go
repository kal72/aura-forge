package domain

import "time"

// ItemCategory defines the category classification for EVE items.
type ItemCategory string

const (
	CategoryAmmunition  ItemCategory = "Ammunition"
	CategoryDrones      ItemCategory = "Drones"
	CategoryShips       ItemCategory = "Ships"
	CategoryModules     ItemCategory = "Modules"
	CategoryRigs        ItemCategory = "Rigs"
	CategoryCommodities ItemCategory = "Commodities"
	CategoryComponents  ItemCategory = "Components"
	CategoryOther       ItemCategory = "Other"
)

// TrapStatus defines the assessment status of an item.
type TrapStatus string

const (
	TrapStatusLayak         TrapStatus = "LAYAK_PRODUKSI"
	TrapStatusWaspadaModal  TrapStatus = "WASPADA_MODAL"
	TrapStatusWaspadaNiche  TrapStatus = "WASPADA_NICHE"
	TrapStatusJebakan       TrapStatus = "JEBAKAN"
)

// RiskLevel defines the risk tier.
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// MarketItem represents an industrial or trading candidate item.
type MarketItem struct {
	ID                     int64        `json:"id"`
	TypeID                 int32        `json:"type_id"`
	Name                   string       `json:"name"`
	Category               ItemCategory `json:"category"`
	EstimatedJitaSell      float64      `json:"estimated_jita_sell"`
	EstimatedJitaBuy       float64      `json:"estimated_jita_buy"`
	DailyVolume            float64      `json:"daily_volume"`
	Volume30d              float64      `json:"volume_30d"`
	MaterialCost           float64      `json:"material_cost"`
	BatchOutputQty         int          `json:"batch_output_qty"`
	ProductionTimeSeconds  int          `json:"production_time_seconds"`
	IsManufacturable       bool         `json:"is_manufacturable"`
	RequiresRareBPC        bool         `json:"requires_rare_bpc"`
	ExtraSetupCost         float64      `json:"extra_setup_cost"`
	IsFrequentlyDestroyed  bool         `json:"is_frequently_destroyed"`
	TrapAnalysis           TrapReport   `json:"trap_analysis"`
	CreatedAt              time.Time    `json:"created_at"`
	UpdatedAt              time.Time    `json:"updated_at"`
}

// TrapReport contains copilot risk evaluation results for an item.
type TrapReport struct {
	Status              TrapStatus `json:"status"`
	RiskLevel           RiskLevel  `json:"risk_level"`
	Reasons             []string   `json:"reasons"`
	Recommendation      string     `json:"recommendation"`
	IsBPCJebakan        bool       `json:"is_bpc_jebakan"`
	IsNonManufacturable bool       `json:"is_non_manufacturable"`
	IsSlowMover         bool       `json:"is_slow_mover"`
	IsProfitReceh       bool       `json:"is_profit_receh"`
	IsModalLockup       bool       `json:"is_modal_lockup"`
	IsPriceSpike        bool       `json:"is_price_spike"`
	Avg7dPrice          float64    `json:"avg_7d_price,omitempty"`
	PriceSpikePct       float64    `json:"price_spike_pct,omitempty"`
	EstimatedNetProfit  float64    `json:"estimated_net_profit"`
	EstimatedMarginPct  float64    `json:"estimated_margin_pct"`
	EstimatedROIPct     float64    `json:"estimated_roi_pct"`
	BOMSyncedFromSDE    bool       `json:"bom_synced_from_sde,omitempty"`
	EvaluatedAt         time.Time  `json:"evaluated_at"`
}
