package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// SDEMetadata tracks the currently installed EVE Online SDE build and record counts.
type SDEMetadata struct {
	BuildNumber     int64     `json:"build_number"`
	ReleaseDate     string    `json:"release_date"`
	Source          string    `json:"source"` // "starter" | "ccp_jsonl"
	TypesCount      int       `json:"types_count"`
	BlueprintsCount int       `json:"blueprints_count"`
	MaterialsCount  int       `json:"materials_count"`
	InventionsCount int       `json:"inventions_count"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CategoryCount pairs a normalized EVE category name with its item count in sde_types.
type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// MarketTypePageResult holds paginated SDE market item browse results plus category & group facets.
type MarketTypePageResult struct {
	Items      []SDEType       `json:"items"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
	Categories []CategoryCount `json:"categories"`
	Groups     []string        `json:"groups"`
}

// BlueprintPageResult holds paginated blueprint search results plus available categories for the Modal Popup.
type BlueprintPageResult struct {
	Items      []SDEBlueprint `json:"items"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
	Categories []string       `json:"categories"`
}

// SDEType represents an EVE Online inventory type from the Static Data Export.
type SDEType struct {
	TypeID      int32   `json:"type_id"`
	Name        string  `json:"name"`
	GroupName   string  `json:"group_name"`
	Category    string  `json:"category"`
	VolumeM3    float64 `json:"volume_m3"`
	PortionSize int     `json:"portion_size"`
}

// SDEBlueprint represents a manufacturing blueprint definition from EVE SDE.
type SDEBlueprint struct {
	BlueprintTypeID int32  `json:"blueprint_type_id"`
	Name            string `json:"name"`
	ProductTypeID   int32  `json:"product_type_id"`
	ProductName     string `json:"product_name"`
	ProductQtyPerRun int   `json:"product_qty_per_run"`
	BaseTimeSeconds int    `json:"base_time_seconds"`
	TechLevel       int    `json:"tech_level"`
	Category        string `json:"category"`
	ActivityType    string `json:"activity_type,omitempty"` // "manufacturing" or "reaction"
}

// SDEInvention represents one official CCP SDE T1 -> T2 invention pathway from blueprints.jsonl.
type SDEInvention struct {
	T1BlueprintTypeID int32   `json:"t1_blueprint_type_id"`
	T2BlueprintTypeID int32   `json:"t2_blueprint_type_id"`
	T2ProductTypeID   int32   `json:"t2_product_type_id"`
	T2ProductName     string  `json:"t2_product_name"`
	BaseProbability   float64 `json:"base_probability"`
	BaseRuns          int     `json:"base_runs"`
	BaseTimeSeconds   int     `json:"base_time_seconds"`
	Category          string  `json:"category"`
}

// SDEInventionMaterial represents one datacore/material required per T2 invention attempt.
type SDEInventionMaterial struct {
	T1BlueprintTypeID int32  `json:"t1_blueprint_type_id"`
	MaterialTypeID    int32  `json:"material_type_id"`
	MaterialName      string `json:"material_name,omitempty"`
	Quantity          int    `json:"quantity"`
}

// SDEMaterialRequirement represents one raw material line required by a blueprint at ME 0.
type SDEMaterialRequirement struct {
	MaterialTypeID int32   `json:"material_type_id"`
	MaterialName   string  `json:"material_name"`
	BaseQuantity   int64   `json:"base_quantity"`
	VolumeM3       float64 `json:"volume_m3"`
}

// StructureProfile defines manufacturing facility ME and TE bonuses.
type StructureProfile struct {
	Key        string  `json:"key"`
	Name       string  `json:"name"`
	MEBonusPct float64 `json:"me_bonus_pct"`
	TEBonusPct float64 `json:"te_bonus_pct"`
}

// DefaultStructureProfiles lists common High-Sec & Engineering Complex manufacturing profiles.
var DefaultStructureProfiles = []StructureProfile{
	{Key: "npc_station", Name: "NPC Station (0% ME / 0% TE)", MEBonusPct: 0.0, TEBonusPct: 0.0},
	{Key: "raitaru_t1", Name: "Raitaru + T1 Rig (3.0% ME / 35% TE)", MEBonusPct: 3.0, TEBonusPct: 35.0},
	{Key: "raitaru_t2", Name: "Raitaru + T2 Rig (3.4% ME / 39.2% TE)", MEBonusPct: 3.4, TEBonusPct: 39.2},
	{Key: "azbel_t1", Name: "Azbel + T1 Rig (3.0% ME / 40% TE)", MEBonusPct: 3.0, TEBonusPct: 40.0},
	{Key: "azbel_t2", Name: "Azbel + T2 Rig (5.2% ME / 44% TE)", MEBonusPct: 5.2, TEBonusPct: 44.0},
}

// LookupStructureProfile returns the StructureProfile matching key (defaults to NPC Station).
func LookupStructureProfile(key string) StructureProfile {
	for _, p := range DefaultStructureProfiles {
		if p.Key == key {
			return p
		}
	}
	return DefaultStructureProfiles[0]
}

// BlueprintCalcInput holds all parameters for an SDE Blueprint profit & BOM calculation.
type BlueprintCalcInput struct {
	BlueprintTypeID     int32   `json:"blueprint_type_id"`
	Runs                int     `json:"runs"`
	ME                  int     `json:"me"` // 0..10
	TE                  int     `json:"te"` // 0..20
	StructureKey        string                     `json:"structure_key"`
	HubKey              string                     `json:"hub_key"`               // jita | amarr | dodixie | rens | hek
	MaterialPriceSource string                     `json:"material_price_source"` // "jita_sell" or "jita_buy"
	MakeBuyOverrides    map[int32]IndustryDecision `json:"make_buy_overrides,omitempty"`
	ProductSellOverride float64 `json:"product_sell_override"` // optional manual override; 0 = live Jita sell
	ExtraSetupCost      float64 `json:"extra_setup_cost"`
	SystemCostIndex     float64 `json:"system_cost_index"`
	FacilityTaxPercent  float64 `json:"facility_tax_percent"`
	FreightCostPerM3    float64 `json:"freight_cost_per_m3"`
	BrokerFeePercent    float64 `json:"broker_fee_percent"`
	SalesTaxPercent     float64 `json:"sales_tax_percent"`
	RelistFeePercent    float64 `json:"relist_fee_percent"`
	EstimatedRelists    int     `json:"estimated_relists"`
	BatchesPerWeek      int     `json:"batches_per_week"`
}

// BlueprintMaterialLine holds a single ME-adjusted material requirement priced against live Jita data.
type BlueprintMaterialLine struct {
	MaterialTypeID int32   `json:"material_type_id"`
	MaterialName   string  `json:"material_name"`
	BaseQtyPerRun  int64   `json:"base_qty_per_run"`
	AdjustedQty    int64   `json:"adjusted_qty"`
	UnitPrice      float64 `json:"unit_price"`
	TotalCost      float64 `json:"total_cost"`
	TotalVolumeM3  float64 `json:"total_volume_m3"`
}

// BlueprintCalcResult contains the complete SDE BOM breakdown, build duration, ISK/hour, and profit analysis.
type BlueprintCalcResult struct {
	Blueprint                SDEBlueprint            `json:"blueprint"`
	Structure                StructureProfile        `json:"structure"`
	PriceHub                 TradeHub                `json:"price_hub"`
	Runs                     int                     `json:"runs"`
	ME                       int                     `json:"me"`
	TE                       int                     `json:"te"`
	MaterialPriceSource      string                  `json:"material_price_source"`
	TotalOutputQty           int                     `json:"total_output_qty"`
	ProductUnitSellPrice     float64                 `json:"product_unit_sell_price"`
	ProductUnitBuyPrice      float64                 `json:"product_unit_buy_price"`
	Materials                []BlueprintMaterialLine `json:"materials"`
	TotalMaterialCost        float64                 `json:"total_material_cost"`
	TotalMaterialVolumeM3    float64                 `json:"total_material_volume_m3"`
	FreightCost              float64                 `json:"freight_cost"`
	EstimatedDurationSeconds int64                   `json:"estimated_duration_seconds"`
	DurationFormatted        string                  `json:"duration_formatted"`
	ISKPerHour               float64                 `json:"isk_per_hour"`
	Profit                   ProfitCalcResult        `json:"profit"`
	MultibuyText             string                  `json:"multibuy_text"`
	LivePricesUsed           bool                    `json:"live_prices_used"`
	IndustryTree             *IndustryTreeAnalysis   `json:"industry_tree,omitempty"`
}

// ComputeAdjustedMaterialQty applies EVE Online's official ME and structure material bonus formula.
func ComputeAdjustedMaterialQty(baseQty int64, runs int, me int, structMEBonusPct float64) int64 {
	if baseQty <= 0 || runs <= 0 {
		return 0
	}
	if me < 0 {
		me = 0
	}
	if me > 10 {
		me = 10
	}
	if structMEBonusPct < 0 {
		structMEBonusPct = 0
	}

	meFactor := 1.0 - (float64(me) / 100.0)
	structFactor := 1.0 - (structMEBonusPct / 100.0)
	raw := float64(baseQty) * float64(runs) * meFactor * structFactor
	// Avoid floating point precision artifacts (e.g., 900.0000000000001 -> 901)
	rounded := math.Ceil(math.Round(raw*1e6) / 1e6)
	qty := int64(rounded)
	if qty < int64(runs) {
		qty = int64(runs)
	}
	return qty
}

// ComputeAdjustedBuildSeconds applies EVE Online's TE and structure time reduction formula.
func ComputeAdjustedBuildSeconds(baseTimeSeconds int, runs int, te int, structTEBonusPct float64) int64 {
	if baseTimeSeconds <= 0 || runs <= 0 {
		return 0
	}
	if te < 0 {
		te = 0
	}
	if te > 20 {
		te = 20
	}
	if structTEBonusPct < 0 {
		structTEBonusPct = 0
	}
	teFactor := 1.0 - (float64(te) / 100.0)
	structFactor := 1.0 - (structTEBonusPct / 100.0)
	sec := int64(math.Ceil(float64(baseTimeSeconds) * float64(runs) * teFactor * structFactor))
	if sec < 1 {
		sec = 1
	}
	return sec
}

// FormatDurationSeconds formats a duration in seconds as a compact human-readable string (e.g., "2h 15m").
func FormatDurationSeconds(sec int64) string {
	if sec <= 0 {
		return "0m"
	}
	days := sec / 86400
	rem := sec % 86400
	hours := rem / 3600
	mins := (rem % 3600) / 60

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	return strings.Join(parts, " ")
}

// CalculateBlueprintManufacturing evaluates a full blueprint BOM, build time, ISK/hour, and net profit.
func CalculateBlueprintManufacturing(
	bp SDEBlueprint,
	reqs []SDEMaterialRequirement,
	in BlueprintCalcInput,
	prices map[int32]JitaPrice,
) (BlueprintCalcResult, error) {
	runs := in.Runs
	if runs <= 0 {
		runs = 1
	}
	me := in.ME
	if me < 0 {
		me = 0
	}
	if me > 10 {
		me = 10
	}
	te := in.TE
	if te < 0 {
		te = 0
	}
	if te > 20 {
		te = 20
	}

	structure := LookupStructureProfile(in.StructureKey)
	priceSource := strings.ToLower(strings.TrimSpace(in.MaterialPriceSource))
	if priceSource != "jita_buy" {
		priceSource = "jita_sell"
	}

	matLines := make([]BlueprintMaterialLine, 0, len(reqs))
	multibuyLines := make([]string, 0, len(reqs))
	var totalMatCost float64
	var totalMatVol float64
	liveUsed := false

	for _, req := range reqs {
		adjQty := ComputeAdjustedMaterialQty(req.BaseQuantity, runs, me, structure.MEBonusPct)
		var unitPrice float64
		if jp, ok := prices[req.MaterialTypeID]; ok {
			if priceSource == "jita_buy" && jp.HighestBuy > 0 {
				unitPrice = jp.HighestBuy
				liveUsed = true
			} else if jp.LowestSell > 0 {
				unitPrice = jp.LowestSell
				liveUsed = true
			} else if jp.HighestBuy > 0 {
				unitPrice = jp.HighestBuy
				liveUsed = true
			}
		}
		lineCost := RoundISK(unitPrice * float64(adjQty))
		lineVol := math.Round((req.VolumeM3*float64(adjQty))*100) / 100
		totalMatCost += lineCost
		totalMatVol += lineVol

		matLines = append(matLines, BlueprintMaterialLine{
			MaterialTypeID: req.MaterialTypeID,
			MaterialName:   req.MaterialName,
			BaseQtyPerRun:  req.BaseQuantity,
			AdjustedQty:    adjQty,
			UnitPrice:      RoundISK(unitPrice),
			TotalCost:      lineCost,
			TotalVolumeM3:  lineVol,
		})
		multibuyLines = append(multibuyLines, fmt.Sprintf("%s\t%d", req.MaterialName, adjQty))
	}

	totalMatCost = RoundISK(totalMatCost)
	totalMatVol = math.Round(totalMatVol*100) / 100
	freightCost := RoundISK(totalMatVol * math.Max(0, in.FreightCostPerM3))

	var prodSell, prodBuy float64
	if jp, ok := prices[bp.ProductTypeID]; ok {
		prodSell = jp.LowestSell
		prodBuy = jp.HighestBuy
		if prodSell > 0 {
			liveUsed = true
		}
	}
	if in.ProductSellOverride > 0 {
		prodSell = RoundISK(in.ProductSellOverride)
	}

	qtyPerRun := bp.ProductQtyPerRun
	if qtyPerRun <= 0 {
		qtyPerRun = 1
	}
	totalOutputQty := qtyPerRun * runs

	profitRes, err := CalculateProfit(ProfitCalcInput{
		ItemName:           bp.ProductName,
		OutputQty:          totalOutputQty,
		UnitSellPrice:      prodSell,
		MaterialCost:       totalMatCost,
		ExtraSetupCost:     in.ExtraSetupCost,
		SystemCostIndex:    in.SystemCostIndex,
		FacilityTaxPercent: in.FacilityTaxPercent,
		FreightCost:        freightCost,
		BrokerFeePercent:   in.BrokerFeePercent,
		SalesTaxPercent:    in.SalesTaxPercent,
		RelistFeePercent:   in.RelistFeePercent,
		EstimatedRelists:   in.EstimatedRelists,
		BatchesPerWeek:     in.BatchesPerWeek,
	})
	if err != nil {
		return BlueprintCalcResult{}, err
	}

	durSec := ComputeAdjustedBuildSeconds(bp.BaseTimeSeconds, runs, te, structure.TEBonusPct)
	var iskPerHour float64
	if durSec > 0 {
		iskPerHour = RoundISK(profitRes.NetProfitPerBatch / (float64(durSec) / 3600.0))
	}

	return BlueprintCalcResult{
		Blueprint:                bp,
		Structure:                structure,
		Runs:                     runs,
		ME:                       me,
		TE:                       te,
		MaterialPriceSource:      priceSource,
		TotalOutputQty:           totalOutputQty,
		ProductUnitSellPrice:     RoundISK(prodSell),
		ProductUnitBuyPrice:      RoundISK(prodBuy),
		Materials:                matLines,
		TotalMaterialCost:        totalMatCost,
		TotalMaterialVolumeM3:    totalMatVol,
		FreightCost:              freightCost,
		EstimatedDurationSeconds: durSec,
		DurationFormatted:        FormatDurationSeconds(durSec),
		ISKPerHour:               iskPerHour,
		Profit:                   profitRes,
		MultibuyText:             strings.Join(multibuyLines, "\n"),
		LivePricesUsed:           liveUsed,
	}, nil
}
