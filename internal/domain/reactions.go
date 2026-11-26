package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ReactionInputLine represents one raw material required per reaction run (base ME 0).
type ReactionInputLine struct {
	TypeID   int32  `json:"type_id"`
	Name     string `json:"name"`
	BaseQty  int64  `json:"base_qty"`
	Category string `json:"category"` // "Gas", "Mineral", "Fuel"
}

// ReactionFormula defines one canonical EVE Online Hybrid Polymer Reaction Formula.
type ReactionFormula struct {
	ProductTypeID   int32               `json:"product_type_id"`
	ProductName     string              `json:"product_name"`
	OutputQtyPerRun int                 `json:"output_qty_per_run"`
	BaseTimeSeconds int                 `json:"base_time_seconds"` // 10800s (3h)
	Inputs          []ReactionInputLine `json:"inputs"`
}

// RefineryProfile models EVE Online Athanor / Tatara reaction rig ME & TE bonuses.
type RefineryProfile struct {
	Key        string  `json:"key"`
	Name       string  `json:"name"`
	MEBonusPct float64 `json:"me_bonus_pct"`
	TEBonusPct float64 `json:"te_bonus_pct"`
}

// DefaultRefineryProfiles lists standard EVE Online Refinery setups for Hybrid Polymer Reactions.
var DefaultRefineryProfiles = []RefineryProfile{
	{Key: "athanor_t1", Name: "Athanor + T1 Reactor Rig (ME 2.0% / TE 20%)", MEBonusPct: 2.0, TEBonusPct: 20.0},
	{Key: "athanor_t2", Name: "Athanor + T2 Reactor Rig (ME 2.4% / TE 24%)", MEBonusPct: 2.4, TEBonusPct: 24.0},
	{Key: "tatara_t2", Name: "Tatara + T2 Reactor Rig (ME 2.4% / TE 43%)", MEBonusPct: 2.4, TEBonusPct: 43.0},
	{Key: "athanor_base", Name: "Athanor — No Reactor Rig (ME 0% / TE 0%)", MEBonusPct: 0.0, TEBonusPct: 0.0},
}

// LookupRefineryProfile resolves a RefineryProfile by key (defaults to Athanor + T1 Rig).
func LookupRefineryProfile(key string) RefineryProfile {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, p := range DefaultRefineryProfiles {
		if p.Key == k {
			return p
		}
	}
	return DefaultRefineryProfiles[0]
}

// HybridPolymerFormulas holds all 8 canonical EVE Online Fullerite Hybrid Polymer Reaction Formulas (3h base cycle).
var HybridPolymerFormulas = []ReactionFormula{
	{
		ProductTypeID:   30303,
		ProductName:     "C3-FTM Acid",
		OutputQtyPerRun: 100,
		BaseTimeSeconds: 10800,
		Inputs: []ReactionInputLine{
			{TypeID: 30374, Name: "Fullerite-C84", BaseQty: 100, Category: "Gas"},
			{TypeID: 30378, Name: "Fullerite-C540", BaseQty: 100, Category: "Gas"},
			{TypeID: 40, Name: "Megacyte", BaseQty: 80, Category: "Mineral"},
			{TypeID: 4247, Name: "Helium Fuel Block", BaseQty: 5, Category: "Fuel"},
		},
	},
	{
		ProductTypeID:   30305,
		ProductName:     "Carbon-86 Epoxy Resin",
		OutputQtyPerRun: 160,
		BaseTimeSeconds: 10800,
		Inputs: []ReactionInputLine{
			{TypeID: 30376, Name: "Fullerite-C32", BaseQty: 100, Category: "Gas"},
			{TypeID: 30377, Name: "Fullerite-C320", BaseQty: 100, Category: "Gas"},
			{TypeID: 39, Name: "Zydrine", BaseQty: 30, Category: "Mineral"},
			{TypeID: 4051, Name: "Nitrogen Fuel Block", BaseQty: 5, Category: "Fuel"},
		},
	},
	{
		ProductTypeID:   30306,
		ProductName:     "Fullerene Intercalated Graphite",
		OutputQtyPerRun: 120,
		BaseTimeSeconds: 10800,
		Inputs: []ReactionInputLine{
			{TypeID: 30371, Name: "Fullerite-C60", BaseQty: 100, Category: "Gas"},
			{TypeID: 30372, Name: "Fullerite-C70", BaseQty: 100, Category: "Gas"},
			{TypeID: 36, Name: "Mexallon", BaseQty: 600, Category: "Mineral"},
			{TypeID: 4246, Name: "Hydrogen Fuel Block", BaseQty: 5, Category: "Fuel"},
		},
	},
	{
		ProductTypeID:   30307,
		ProductName:     "Fulleroferrocene",
		OutputQtyPerRun: 120,
		BaseTimeSeconds: 10800,
		Inputs: []ReactionInputLine{
			{TypeID: 30370, Name: "Fullerite-C50", BaseQty: 100, Category: "Gas"},
			{TypeID: 30371, Name: "Fullerite-C60", BaseQty: 100, Category: "Gas"},
			{TypeID: 34, Name: "Tritanium", BaseQty: 1000, Category: "Mineral"},
			{TypeID: 4312, Name: "Oxygen Fuel Block", BaseQty: 5, Category: "Fuel"},
		},
	},
	{
		ProductTypeID:   30308,
		ProductName:     "Graphene Nanoribbons",
		OutputQtyPerRun: 120,
		BaseTimeSeconds: 10800,
		Inputs: []ReactionInputLine{
			{TypeID: 30375, Name: "Fullerite-C28", BaseQty: 100, Category: "Gas"},
			{TypeID: 30376, Name: "Fullerite-C32", BaseQty: 100, Category: "Gas"},
			{TypeID: 38, Name: "Nocxium", BaseQty: 400, Category: "Mineral"},
			{TypeID: 4051, Name: "Nitrogen Fuel Block", BaseQty: 5, Category: "Fuel"},
		},
	},
	{
		ProductTypeID:   30309,
		ProductName:     "Lanthanum Metallofullerene",
		OutputQtyPerRun: 120,
		BaseTimeSeconds: 10800,
		Inputs: []ReactionInputLine{
			{TypeID: 30372, Name: "Fullerite-C70", BaseQty: 100, Category: "Gas"},
			{TypeID: 30374, Name: "Fullerite-C84", BaseQty: 100, Category: "Gas"},
			{TypeID: 38, Name: "Nocxium", BaseQty: 200, Category: "Mineral"},
			{TypeID: 4312, Name: "Oxygen Fuel Block", BaseQty: 5, Category: "Fuel"},
		},
	},
	{
		ProductTypeID:   30310,
		ProductName:     "Methanofullerene",
		OutputQtyPerRun: 160,
		BaseTimeSeconds: 10800,
		Inputs: []ReactionInputLine{
			{TypeID: 30372, Name: "Fullerite-C70", BaseQty: 100, Category: "Gas"},
			{TypeID: 30373, Name: "Fullerite-C72", BaseQty: 100, Category: "Gas"},
			{TypeID: 37, Name: "Isogen", BaseQty: 300, Category: "Mineral"},
			{TypeID: 4246, Name: "Hydrogen Fuel Block", BaseQty: 5, Category: "Fuel"},
		},
	},
	{
		ProductTypeID:   30311,
		ProductName:     "PPD Fullerene Fibers",
		OutputQtyPerRun: 250,
		BaseTimeSeconds: 10800,
		Inputs: []ReactionInputLine{
			{TypeID: 30370, Name: "Fullerite-C50", BaseQty: 300, Category: "Gas"},
			{TypeID: 30371, Name: "Fullerite-C60", BaseQty: 100, Category: "Gas"},
			{TypeID: 35, Name: "Pyerite", BaseQty: 800, Category: "Mineral"},
			{TypeID: 4246, Name: "Hydrogen Fuel Block", BaseQty: 5, Category: "Fuel"},
		},
	},
}

// ReactionOpportunity represents one ranked Hybrid Polymer reaction profitability result.
type ReactionOpportunity struct {
	ProductTypeID     int32              `json:"product_type_id"`
	ProductName       string             `json:"product_name"`
	HubKey            string             `json:"hub_key"`
	HubName           string             `json:"hub_name"`
	MaterialHubKey    string             `json:"material_hub_key"`
	MaterialHubName   string             `json:"material_hub_name"`
	RefineryName      string             `json:"refinery_name"`
	Runs              int                `json:"runs"`
	OutputQty         int                `json:"output_qty"`
	InputCost         float64            `json:"input_cost"`
	JobFeeISK         float64            `json:"job_fee_isk"`
	ProductUnitSell   float64            `json:"product_unit_sell"`
	RecommendedSell   float64            `json:"recommended_sell"`
	BreakEvenSell     float64            `json:"break_even_sell"`
	DailyVolume7d     float64            `json:"daily_volume_7d"`
	NetProfit         float64            `json:"net_profit"`
	NetMarginPercent  float64            `json:"net_margin_percent"`
	ROIPercent        float64            `json:"roi_percent"`
	ISKPerHour        float64            `json:"isk_per_hour"`
	DurationFormatted string             `json:"duration_formatted"`
	InputsSummary     string             `json:"inputs_summary"`
	MultibuyText      string             `json:"multibuy_text"`
	Verdict           OpportunityVerdict `json:"verdict"`
}

// ReactionCalcParams configures the fee, hub, and refinery settings for evaluating a Hybrid Polymer reaction.
type ReactionCalcParams struct {
	Runs                int
	Refinery            RefineryProfile
	SellHub             TradeHub
	MaterialHub         TradeHub
	MaterialPriceSource string // "jita_sell" or "jita_buy"
	SystemCostIndex     float64
	FacilityTaxPercent  float64
	BrokerFeePercent    float64
	SalesTaxPercent     float64
	RelistFeePercent    float64
}

// EvaluateGasReaction computes full Hybrid Polymer reaction economics including Rig ME/TE,
// reaction job fee, broker fee, sales tax, relist fee, break-even floor, and EVE Multibuy text.
func EvaluateGasReaction(
	formula ReactionFormula,
	params ReactionCalcParams,
	matPrices map[int32]JitaPrice,
	sellPrices map[int32]JitaPrice,
	dailyVol7d float64,
) (ReactionOpportunity, bool) {
	sp, ok := sellPrices[formula.ProductTypeID]
	if !ok || sp.LowestSell <= 0 {
		return ReactionOpportunity{}, false
	}

	runs := params.Runs
	if runs <= 0 {
		// Default to ~24 hours of continuous reaction cycles
		singleSec := ComputeAdjustedBuildSeconds(formula.BaseTimeSeconds, 1, 0, params.Refinery.TEBonusPct)
		if singleSec > 0 {
			runs = int(math.Max(1, math.Floor(86400.0/float64(singleSec))))
		} else {
			runs = 8
		}
	}
	runs = clampInt(runs, 1, 1000)

	useBuy := strings.EqualFold(strings.TrimSpace(params.MaterialPriceSource), "jita_buy")

	var totalInputCost float64
	summaryParts := make([]string, 0, len(formula.Inputs))
	multibuyLines := make([]string, 0, len(formula.Inputs))

	for _, inp := range formula.Inputs {
		qty := ComputeAdjustedMaterialQty(inp.BaseQty, runs, 0, params.Refinery.MEBonusPct)
		mp := matPrices[inp.TypeID]
		unitPrice := mp.LowestSell
		if useBuy && mp.HighestBuy > 0 {
			unitPrice = mp.HighestBuy
		}
		if unitPrice <= 0 {
			return ReactionOpportunity{}, false
		}
		totalInputCost += float64(qty) * unitPrice
		if inp.Category == "Gas" {
			summaryParts = append(summaryParts, strings.TrimPrefix(inp.Name, "Fullerite-"))
		}
		multibuyLines = append(multibuyLines, fmt.Sprintf("%s %d", inp.Name, qty))
	}

	totalOutput := formula.OutputQtyPerRun * runs
	if totalOutput <= 0 {
		return ReactionOpportunity{}, false
	}

	// Reaction job install cost = Estimated Item Value * (SCI + FacilityTax)
	sci := params.SystemCostIndex
	if sci <= 0 {
		sci = 0.03
	}
	facTax := params.FacilityTaxPercent
	if facTax < 0 {
		facTax = 0
	}
	jobFee := RoundISK(totalInputCost * (sci + facTax/100.0))

	grossRev := sp.LowestSell * float64(totalOutput)
	feePct := (params.BrokerFeePercent + params.SalesTaxPercent + params.RelistFeePercent) / 100.0
	marketFees := RoundISK(grossRev * feePct)
	totalCost := RoundISK(totalInputCost + jobFee + marketFees)
	netProfit := RoundISK(grossRev - totalCost)

	netMargin := 0.0
	if grossRev > 0 {
		netMargin = RoundPercent((netProfit / grossRev) * 100.0)
	}
	directCost := RoundISK(totalInputCost + jobFee)
	roi := 0.0
	if directCost > 0 {
		roi = RoundPercent((netProfit / directCost) * 100.0)
	}

	breakEven := 0.0
	if feeFactor := 1.0 - feePct; feeFactor > 0 {
		breakEven = RoundISK((directCost / float64(totalOutput)) / feeFactor)
	}

	recSell := RoundEVETick(sp.LowestSell * 0.999)
	if recSell <= 0 {
		recSell = RoundISK(sp.LowestSell)
	}

	durSec := ComputeAdjustedBuildSeconds(formula.BaseTimeSeconds, runs, 0, params.Refinery.TEBonusPct)
	iskPerHour := 0.0
	if durSec > 0 {
		iskPerHour = RoundISK(netProfit / (float64(durSec) / 3600.0))
	}

	verdict := ClassifyManufacturingVerdict(netProfit, netMargin, iskPerHour)

	return ReactionOpportunity{
		ProductTypeID:     formula.ProductTypeID,
		ProductName:       formula.ProductName,
		HubKey:            params.SellHub.Key,
		HubName:           params.SellHub.Name,
		MaterialHubKey:    params.MaterialHub.Key,
		MaterialHubName:   params.MaterialHub.Name,
		RefineryName:      params.Refinery.Name,
		Runs:              runs,
		OutputQty:         totalOutput,
		InputCost:         RoundISK(totalInputCost),
		JobFeeISK:         jobFee,
		ProductUnitSell:   RoundISK(sp.LowestSell),
		RecommendedSell:   recSell,
		BreakEvenSell:     breakEven,
		DailyVolume7d:     dailyVol7d,
		NetProfit:         netProfit,
		NetMarginPercent:  netMargin,
		ROIPercent:        roi,
		ISKPerHour:        iskPerHour,
		DurationFormatted: FormatDurationSeconds(durSec),
		InputsSummary:     strings.Join(summaryParts, " + "),
		MultibuyText:      strings.Join(multibuyLines, "\n"),
		Verdict:           verdict,
	}, true
}

// SortReactionOpportunities orders reaction results by ISK/Hour descending, then Net Margin descending.
func SortReactionOpportunities(list []ReactionOpportunity) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].ISKPerHour != list[j].ISKPerHour {
			return list[i].ISKPerHour > list[j].ISKPerHour
		}
		return list[i].NetMarginPercent > list[j].NetMarginPercent
	})
}
