package domain

import (
	"math"
	"sort"
)

// DecryptorSpec defines an official EVE Online T2 Invention Decryptor and its modifiers.
type DecryptorSpec struct {
	TypeID         int32   `json:"type_id"` // 0 = No Decryptor
	Name           string  `json:"name"`
	ProbMultiplier float64 `json:"prob_multiplier"`
	RunsMod        int     `json:"runs_mod"`
	MEMod          int     `json:"me_mod"`
	TEMod          int     `json:"te_mod"`
}

// OfficialDecryptors lists None + all 8 official EVE Online T2 Invention Decryptors.
var OfficialDecryptors = []DecryptorSpec{
	{TypeID: 0, Name: "No Decryptor", ProbMultiplier: 1.0, RunsMod: 0, MEMod: 0, TEMod: 0},
	{TypeID: 34201, Name: "Accelerant Decryptor", ProbMultiplier: 1.2, RunsMod: 1, MEMod: 2, TEMod: 10},
	{TypeID: 34202, Name: "Attainment Decryptor", ProbMultiplier: 1.8, RunsMod: 4, MEMod: -1, TEMod: 4},
	{TypeID: 34203, Name: "Augmentation Decryptor", ProbMultiplier: 0.6, RunsMod: 9, MEMod: -2, TEMod: 2},
	{TypeID: 34204, Name: "Parity Decryptor", ProbMultiplier: 1.5, RunsMod: 3, MEMod: 1, TEMod: -2},
	{TypeID: 34205, Name: "Process Decryptor", ProbMultiplier: 1.1, RunsMod: 0, MEMod: 3, TEMod: 6},
	{TypeID: 34206, Name: "Symmetry Decryptor", ProbMultiplier: 1.0, RunsMod: 2, MEMod: 1, TEMod: 8},
	{TypeID: 34207, Name: "Optimized Attainment Decryptor", ProbMultiplier: 1.9, RunsMod: 2, MEMod: 1, TEMod: -2},
	{TypeID: 34208, Name: "Optimized Augmentation Decryptor", ProbMultiplier: 0.9, RunsMod: 7, MEMod: 2, TEMod: 0},
}

// T2InventionPreset represents a popular beginner-to-intermediate T2 product for instant live invention optimization.
type T2InventionPreset struct {
	Key              string  `json:"key"`
	ProductTypeID    int32   `json:"product_type_id"`
	ProductName      string  `json:"product_name"`
	Category         string  `json:"category"`
	BaseChance       float64 `json:"base_chance"` // e.g. 0.34 for modules/drones, 0.30 for frigates
	BaseT2Runs       int     `json:"base_t2_runs"`
	OutputQtyPerRun  int     `json:"output_qty_per_run"`
	Datacore1TypeID  int32   `json:"datacore_1_type_id"`
	Datacore1Name    string  `json:"datacore_1_name"`
	Datacore2TypeID  int32   `json:"datacore_2_type_id"`
	Datacore2Name    string  `json:"datacore_2_name"`
	DatacoresPerTry  int     `json:"datacores_per_try"` // quantity of each datacore per invention attempt
	DefaultBuildCost float64 `json:"default_build_cost"` // estimated ME 0 T2 material cost per run (user-editable)
}

// DefaultT2Presets lists 8 high-velocity T2 products for scaling from 1B to 100B+ ISK.
var DefaultT2Presets = []T2InventionPreset{
	{Key: "hobgoblin_ii", ProductTypeID: 2456, ProductName: "Hobgoblin II", Category: "Drones", BaseChance: 0.34, BaseT2Runs: 10, OutputQtyPerRun: 1, Datacore1TypeID: 20410, Datacore1Name: "Datacore - Gallentean Starship Engineering", Datacore2TypeID: 20424, Datacore2Name: "Datacore - Mechanical Engineering", DatacoresPerTry: 2, DefaultBuildCost: 480_000},
	{Key: "hammerhead_ii", ProductTypeID: 2185, ProductName: "Hammerhead II", Category: "Drones", BaseChance: 0.34, BaseT2Runs: 10, OutputQtyPerRun: 1, Datacore1TypeID: 20410, Datacore1Name: "Datacore - Gallentean Starship Engineering", Datacore2TypeID: 20424, Datacore2Name: "Datacore - Mechanical Engineering", DatacoresPerTry: 2, DefaultBuildCost: 920_000},
	{Key: "warrior_ii", ProductTypeID: 2488, ProductName: "Warrior II", Category: "Drones", BaseChance: 0.34, BaseT2Runs: 10, OutputQtyPerRun: 1, Datacore1TypeID: 20172, Datacore1Name: "Datacore - Minmatar Starship Engineering", Datacore2TypeID: 20424, Datacore2Name: "Datacore - Mechanical Engineering", DatacoresPerTry: 2, DefaultBuildCost: 460_000},
	{Key: "damage_control_ii", ProductTypeID: 2048, ProductName: "Damage Control II", Category: "Modules", BaseChance: 0.34, BaseT2Runs: 10, OutputQtyPerRun: 1, Datacore1TypeID: 20424, Datacore1Name: "Datacore - Mechanical Engineering", Datacore2TypeID: 20418, Datacore2Name: "Datacore - Electronic Engineering", DatacoresPerTry: 2, DefaultBuildCost: 650_000},
	{Key: "lse_ii", ProductTypeID: 3841, ProductName: "Large Shield Extender II", Category: "Modules", BaseChance: 0.34, BaseT2Runs: 10, OutputQtyPerRun: 1, Datacore1TypeID: 20413, Datacore1Name: "Datacore - Electromagnetic Physics", Datacore2TypeID: 20414, Datacore2Name: "Datacore - Quantum Physics", DatacoresPerTry: 2, DefaultBuildCost: 1_150_000},
	{Key: "warp_scrambler_ii", ProductTypeID: 3244, ProductName: "Warp Scrambler II", Category: "Modules", BaseChance: 0.34, BaseT2Runs: 10, OutputQtyPerRun: 1, Datacore1TypeID: 20418, Datacore1Name: "Datacore - Electronic Engineering", Datacore2TypeID: 20416, Datacore2Name: "Datacore - Graviton Physics", DatacoresPerTry: 2, DefaultBuildCost: 780_000},
	{Key: "bcs_ii", ProductTypeID: 2205, ProductName: "Ballistic Control System II", Category: "Modules", BaseChance: 0.34, BaseT2Runs: 10, OutputQtyPerRun: 1, Datacore1TypeID: 20420, Datacore1Name: "Datacore - Rocket Science", Datacore2TypeID: 20418, Datacore2Name: "Datacore - Electronic Engineering", DatacoresPerTry: 2, DefaultBuildCost: 950_000},
	{Key: "gyrostabilizer_ii", ProductTypeID: 519, ProductName: "Gyrostabilizer II", Category: "Modules", BaseChance: 0.34, BaseT2Runs: 10, OutputQtyPerRun: 1, Datacore1TypeID: 20424, Datacore1Name: "Datacore - Mechanical Engineering", Datacore2TypeID: 20415, Datacore2Name: "Datacore - High Energy Physics", DatacoresPerTry: 2, DefaultBuildCost: 920_000},
}

// LookupT2Preset returns the matching T2InventionPreset (defaults to Hobgoblin II).
func LookupT2Preset(key string) T2InventionPreset {
	for _, p := range DefaultT2Presets {
		if p.Key == key {
			return p
		}
	}
	return DefaultT2Presets[0]
}

// InventionInput holds user parameters for evaluating all 9 Decryptors on a T2 product.
type InventionInput struct {
	PresetKey             string  `json:"preset_key"`
	EncryptionSkill       int     `json:"encryption_skill"`        // 1..5
	Datacore1Skill        int     `json:"datacore_1_skill"`        // 1..5
	Datacore2Skill        int     `json:"datacore_2_skill"`        // 1..5
	BaseBuildCostPerRun   float64 `json:"base_build_cost_per_run"` // ME 0 T2 material cost per run
	ProductSellOverride   float64 `json:"product_sell_override"`   // 0 = live Jita sell
	InventionJobFee       float64 `json:"invention_job_fee"`       // per invention attempt
	BrokerFeePercent      float64 `json:"broker_fee_percent"`
	SalesTaxPercent       float64 `json:"sales_tax_percent"`
}

// DecryptorScenario holds the calculated economics for one Decryptor option.
type DecryptorScenario struct {
	Decryptor            DecryptorSpec `json:"decryptor"`
	SuccessChancePct     float64       `json:"success_chance_pct"`
	ResultME             int           `json:"result_me"`
	ResultTE             int           `json:"result_te"`
	ResultRuns           int           `json:"result_runs"`
	DecryptorPrice       float64       `json:"decryptor_price"`
	InventionCostPerBPC  float64       `json:"invention_cost_per_bpc"`
	InventionCostPerUnit float64       `json:"invention_cost_per_unit"`
	BuildCostPerUnit     float64       `json:"build_cost_per_unit"`
	TotalCostPerUnit     float64       `json:"total_cost_per_unit"`
	NetProfitPerUnit     float64       `json:"net_profit_per_unit"`
	NetProfitPerBPC      float64       `json:"net_profit_per_bpc"`
	NetMarginPercent     float64       `json:"net_margin_percent"`
	ROIPercent           float64       `json:"roi_percent"`
	IsBest               bool          `json:"is_best"`
}

// InventionOptimizationResult bundles all 9 decryptor scenarios for a T2 item.
type InventionOptimizationResult struct {
	Preset             T2InventionPreset   `json:"preset"`
	ProductSellPrice   float64             `json:"product_sell_price"`
	DatacoreCostPerTry float64             `json:"datacore_cost_per_try"`
	BestScenario       DecryptorScenario   `json:"best_scenario"`
	Scenarios          []DecryptorScenario `json:"scenarios"`
}

// ComputeInventionSuccessChance implements EVE Online's official T2 Invention probability formula.
func ComputeInventionSuccessChance(baseChance float64, encSkill, dc1Skill, dc2Skill int, probMult float64) float64 {
	if baseChance <= 0 || probMult <= 0 {
		return 0
	}
	skillBonus := 1.0 + (float64(encSkill) / 40.0) + (float64(dc1Skill+dc2Skill) / 30.0)
	chance := baseChance * skillBonus * probMult
	if chance > 1.0 {
		chance = 1.0
	}
	return chance
}

// OptimizeT2Invention evaluates all 9 Decryptor options and ranks them by Net Profit per Unit.
func OptimizeT2Invention(
	preset T2InventionPreset,
	in InventionInput,
	prices map[int32]JitaPrice,
) InventionOptimizationResult {
	dc1Price := prices[preset.Datacore1TypeID].LowestSell
	dc2Price := prices[preset.Datacore2TypeID].LowestSell
	datacoreCostPerTry := RoundISK(float64(preset.DatacoresPerTry) * (dc1Price + dc2Price))

	productSell := prices[preset.ProductTypeID].LowestSell
	if in.ProductSellOverride > 0 {
		productSell = RoundISK(in.ProductSellOverride)
	}

	baseBuildCost := in.BaseBuildCostPerRun
	if baseBuildCost <= 0 {
		baseBuildCost = preset.DefaultBuildCost
	}

	qtyPerRun := preset.OutputQtyPerRun
	if qtyPerRun <= 0 {
		qtyPerRun = 1
	}

	feeFactor := 1.0 - ((in.BrokerFeePercent + in.SalesTaxPercent) / 100.0)
	if feeFactor < 0 {
		feeFactor = 0
	}
	netRevenuePerUnit := productSell * feeFactor

	scenarios := make([]DecryptorScenario, 0, len(OfficialDecryptors))
	for _, dec := range OfficialDecryptors {
		chance := ComputeInventionSuccessChance(preset.BaseChance, in.EncryptionSkill, in.Datacore1Skill, in.Datacore2Skill, dec.ProbMultiplier)
		resME := 2 + dec.MEMod
		resTE := 4 + dec.TEMod
		resRuns := preset.BaseT2Runs + dec.RunsMod
		if resRuns < 1 {
			resRuns = 1
		}

		decPrice := 0.0
		if dec.TypeID > 0 {
			decPrice = RoundISK(prices[dec.TypeID].LowestSell)
		}

		attemptCost := datacoreCostPerTry + decPrice + math.Max(0, in.InventionJobFee)
		invCostPerBPC := 0.0
		if chance > 0 {
			invCostPerBPC = RoundISK(attemptCost / chance)
		}

		totalUnitsPerBPC := float64(resRuns * qtyPerRun)
		invCostPerUnit := RoundISK(invCostPerBPC / totalUnitsPerBPC)

		// Apply resulting BPC ME modifier to base ME 0 build cost
		meFactor := 1.0 - (float64(resME) / 100.0)
		if meFactor < 0.5 {
			meFactor = 0.5
		}
		buildCostPerUnit := RoundISK((baseBuildCost * meFactor) / float64(qtyPerRun))
		totalCostPerUnit := RoundISK(buildCostPerUnit + invCostPerUnit)
		netProfitPerUnit := RoundISK(netRevenuePerUnit - totalCostPerUnit)
		netProfitPerBPC := RoundISK(netProfitPerUnit * totalUnitsPerBPC)

		marginPct := 0.0
		if productSell > 0 {
			marginPct = RoundPercent((netProfitPerUnit / productSell) * 100.0)
		}
		roiPct := 0.0
		if totalCostPerUnit > 0 {
			roiPct = RoundPercent((netProfitPerUnit / totalCostPerUnit) * 100.0)
		}

		scenarios = append(scenarios, DecryptorScenario{
			Decryptor:            dec,
			SuccessChancePct:     RoundPercent(chance * 100.0),
			ResultME:             resME,
			ResultTE:             resTE,
			ResultRuns:           resRuns,
			DecryptorPrice:       decPrice,
			InventionCostPerBPC:  invCostPerBPC,
			InventionCostPerUnit: invCostPerUnit,
			BuildCostPerUnit:     buildCostPerUnit,
			TotalCostPerUnit:     totalCostPerUnit,
			NetProfitPerUnit:     netProfitPerUnit,
			NetProfitPerBPC:      netProfitPerBPC,
			NetMarginPercent:     marginPct,
			ROIPercent:           roiPct,
		})
	}

	sort.Slice(scenarios, func(i, j int) bool {
		if scenarios[i].NetProfitPerUnit != scenarios[j].NetProfitPerUnit {
			return scenarios[i].NetProfitPerUnit > scenarios[j].NetProfitPerUnit
		}
		return scenarios[i].NetProfitPerBPC > scenarios[j].NetProfitPerBPC
	})

	var best DecryptorScenario
	if len(scenarios) > 0 {
		scenarios[0].IsBest = true
		best = scenarios[0]
	}

	return InventionOptimizationResult{
		Preset:             preset,
		ProductSellPrice:   RoundISK(productSell),
		DatacoreCostPerTry: datacoreCostPerTry,
		BestScenario:       best,
		Scenarios:          scenarios,
	}
}

// MakeVsBuyResult compares buying an intermediate component in Jita vs manufacturing it in-house.
type MakeVsBuyResult struct {
	ComponentName      string  `json:"component_name"`
	Quantity           int     `json:"quantity"`
	BuyUnitPrice       float64 `json:"buy_unit_price"`
	TotalBuyCost       float64 `json:"total_buy_cost"`
	RawMaterialPerUnit float64 `json:"raw_material_per_unit"`
	JobInstallTotal    float64 `json:"job_install_total"`
	TotalMakeCost      float64 `json:"total_make_cost"`
	SavingsISK         float64 `json:"savings_isk"` // positive = MAKE saves ISK; negative = BUY saves ISK
	SavingsPercent     float64 `json:"savings_percent"`
	Recommendation     string  `json:"recommendation"` // "MAKE COMPONENT" or "BUY IN JITA"
}

// EvaluateMakeVsBuy determines whether manufacturing a component in-house beats buying it off Jita market.
func EvaluateMakeVsBuy(name string, qty int, jitaUnitPrice, rawMatPerUnit, sci, facilityTaxPct float64) MakeVsBuyResult {
	if qty <= 0 {
		qty = 1
	}
	totalBuy := RoundISK(jitaUnitPrice * float64(qty))
	totalRaw := RoundISK(rawMatPerUnit * float64(qty))
	jobInstall := RoundISK(totalRaw * (sci + facilityTaxPct/100.0))
	totalMake := RoundISK(totalRaw + jobInstall)
	savings := RoundISK(totalBuy - totalMake)

	savingsPct := 0.0
	if totalBuy > 0 {
		savingsPct = RoundPercent((math.Abs(savings) / totalBuy) * 100.0)
	}

	rec := "BUY IN JITA"
	if savings > 0 {
		rec = "MAKE COMPONENT"
	}

	return MakeVsBuyResult{
		ComponentName:      name,
		Quantity:           qty,
		BuyUnitPrice:       RoundISK(jitaUnitPrice),
		TotalBuyCost:       totalBuy,
		RawMaterialPerUnit: RoundISK(rawMatPerUnit),
		JobInstallTotal:    jobInstall,
		TotalMakeCost:      totalMake,
		SavingsISK:         savings,
		SavingsPercent:     savingsPct,
		Recommendation:     rec,
	}
}
