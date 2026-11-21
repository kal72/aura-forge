package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// IndustryDecision represents the Make-vs-Buy verdict for a node in a recursive manufacturing tree.
type IndustryDecision string

const (
	DecisionMake IndustryDecision = "MAKE"
	DecisionBuy  IndustryDecision = "BUY"
	DecisionRaw  IndustryDecision = "RAW"
)

// IndustryTreeNode is one node in a recursive multi-level EVE manufacturing BOM tree.
type IndustryTreeNode struct {
	TypeID              int32              `json:"type_id"`
	Name                string             `json:"name"`
	Quantity            int64              `json:"quantity"`
	RunsRequired        int                `json:"runs_required"`
	Depth               int                `json:"depth"`
	BlueprintID         int32              `json:"blueprint_id,omitempty"`
	TechLevel           int                `json:"tech_level"`
	UnitPrice           float64            `json:"unit_price"`
	BuyCost             float64            `json:"buy_cost"`
	MakeCost            float64            `json:"make_cost"`
	ActiveCost          float64            `json:"active_cost"`
	SavingsISK          float64            `json:"savings_isk"` // positive means MAKE is cheaper than BUY
	JobInstallCost      float64            `json:"job_install_cost"`
	BuildSeconds        int64              `json:"build_seconds"`
	VolumeM3            float64            `json:"volume_m3"`
	RecommendedDecision IndustryDecision   `json:"recommended_decision"`
	ActiveDecision      IndustryDecision   `json:"active_decision"`
	IsOverridden        bool               `json:"is_overridden"`
	Children            []IndustryTreeNode `json:"children,omitempty"`
}

// IndustrySubJob represents one manufacturing step in the bottom-up execution queue.
type IndustrySubJob struct {
	StepOrder       int     `json:"step_order"`
	Depth           int     `json:"depth"`
	BlueprintID     int32   `json:"blueprint_id"`
	ProductTypeID   int32   `json:"product_type_id"`
	ProductName     string  `json:"product_name"`
	Runs            int     `json:"runs"`
	OutputQty       int64   `json:"output_qty"`
	DurationSeconds int64   `json:"duration_seconds"`
	DurationLabel   string  `json:"duration_label"`
	JobInstallCost  float64 `json:"job_install_cost"`
}

// IndustryTreeAnalysis holds the complete recursive Make-vs-Buy optimization, 3-scenario comparison,
// consolidated EVE Multibuy shopping list, and bottom-up sub-job execution plan.
type IndustryTreeAnalysis struct {
	Hub                TradeHub                    `json:"hub"`
	RootNode           IndustryTreeNode            `json:"root_node"`
	FlatNodes          []IndustryTreeNode          `json:"flat_nodes"`
	HasSubBlueprints   bool                        `json:"has_sub_blueprints"`
	BuyAllDirectCost   float64                     `json:"buy_all_direct_cost"`
	MakeAllCost        float64                     `json:"make_all_cost"`
	OptimalCost        float64                     `json:"optimal_cost"` // active cost respecting manual overrides
	OptimalSavingsISK  float64                     `json:"optimal_savings_isk"`
	OptimalSavingsPct  float64                     `json:"optimal_savings_pct"`
	OptimalVolumeM3    float64                     `json:"optimal_volume_m3"`
	OptimalMultibuy    string                      `json:"optimal_multibuy"`
	SubJobs            []IndustrySubJob            `json:"sub_jobs"`
	TotalSubJobSeconds int64                       `json:"total_sub_job_seconds"`
	TotalSubJobLabel   string                      `json:"total_sub_job_label"`
	ActiveOverrides    map[int32]IndustryDecision  `json:"active_overrides,omitempty"`
}

// ParseMakeBuyOverrides parses a compact query/form string like "2454:BUY,11483:MAKE" into a map.
func ParseMakeBuyOverrides(raw string) map[int32]IndustryDecision {
	out := make(map[int32]IndustryDecision)
	for _, part := range strings.Split(strings.TrimSpace(raw), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		var tid int32
		if _, err := fmt.Sscanf(strings.TrimSpace(kv[0]), "%d", &tid); err != nil || tid <= 0 {
			continue
		}
		dec := IndustryDecision(strings.ToUpper(strings.TrimSpace(kv[1])))
		if dec == DecisionMake || dec == DecisionBuy {
			out[tid] = dec
		}
	}
	return out
}

// EncodeMakeBuyOverrides serializes overrides into a deterministic comma-separated string ("2454:BUY,11483:MAKE").
func EncodeMakeBuyOverrides(overrides map[int32]IndustryDecision) string {
	if len(overrides) == 0 {
		return ""
	}
	keys := make([]int32, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		dec := overrides[k]
		if dec == DecisionMake || dec == DecisionBuy {
			parts = append(parts, fmt.Sprintf("%d:%s", k, dec))
		}
	}
	return strings.Join(parts, ",")
}

// EvaluateIndustryTree builds and optimizes a multi-level manufacturing tree from pre-loaded SDE blueprints/materials
// and trade-hub prices, computing BUY vs MAKE at every manufacturable sub-node and generating an optimal Multibuy list.
func EvaluateIndustryTree(
	rootBP SDEBlueprint,
	bpsByProduct map[int32]SDEBlueprint,
	matsByBP map[int32][]SDEMaterialRequirement,
	prices map[int32]JitaPrice,
	in BlueprintCalcInput,
) *IndustryTreeAnalysis {
	runs := in.Runs
	if runs <= 0 {
		runs = 1
	}
	structure := LookupStructureProfile(in.StructureKey)
	hub := LookupTradeHub(in.HubKey)
	useBuyPrice := strings.EqualFold(strings.TrimSpace(in.MaterialPriceSource), "jita_buy")
	feeRate := (in.SystemCostIndex + in.FacilityTaxPercent/100.0)
	if feeRate < 0 {
		feeRate = 0
	}

	unitPriceOf := func(typeID int32) float64 {
		p, ok := prices[typeID]
		if !ok {
			return 0
		}
		if useBuyPrice && p.HighestBuy > 0 {
			return p.HighestBuy
		}
		if p.LowestSell > 0 {
			return p.LowestSell
		}
		return p.HighestBuy
	}

	hasSubBPs := false
	visited := make(map[int32]bool)

	var buildNode func(typeID int32, name string, qty int64, volPerUnit float64, depth int, bpOverride *SDEBlueprint) IndustryTreeNode
	buildNode = func(typeID int32, name string, qty int64, volPerUnit float64, depth int, bpOverride *SDEBlueprint) IndustryTreeNode {
		unitPrice := unitPriceOf(typeID)
		buyCost := RoundISK(float64(qty) * unitPrice)

		node := IndustryTreeNode{
			TypeID:              typeID,
			Name:                name,
			Quantity:            qty,
			Depth:               depth,
			UnitPrice:           unitPrice,
			BuyCost:             buyCost,
			ActiveCost:          buyCost,
			VolumeM3:            float64(qty) * volPerUnit,
			RecommendedDecision: DecisionRaw,
			ActiveDecision:      DecisionRaw,
		}

		// Determine if this node is manufacturable
		var subBP SDEBlueprint
		hasBP := false
		if bpOverride != nil {
			subBP = *bpOverride
			hasBP = true
		} else if depth > 0 && depth < 5 && !visited[typeID] {
			if found, ok := bpsByProduct[typeID]; ok {
				if mats := matsByBP[found.BlueprintTypeID]; len(mats) > 0 {
					subBP = found
					hasBP = true
					hasSubBPs = true
				}
			}
		}

		if !hasBP {
			return node
		}

		visited[typeID] = true
		defer delete(visited, typeID)

		qtyPerRun := subBP.ProductQtyPerRun
		if qtyPerRun <= 0 {
			qtyPerRun = 1
		}
		nodeRuns := int(math.Ceil(float64(qty) / float64(qtyPerRun)))
		if depth == 0 {
			nodeRuns = runs
		}
		if nodeRuns < 1 {
			nodeRuns = 1
		}

		// ME/TE for root uses user input; sub-blueprints use sensible ME (10 for T1, 2 for T2)
		nodeME := in.ME
		nodeTE := in.TE
		if depth > 0 {
			if subBP.TechLevel >= 2 {
				nodeME = 2
				nodeTE = 4
			} else {
				nodeME = 10
				nodeTE = 20
			}
		}

		node.BlueprintID = subBP.BlueprintTypeID
		node.TechLevel = subBP.TechLevel
		node.RunsRequired = nodeRuns
		node.BuildSeconds = ComputeAdjustedBuildSeconds(subBP.BaseTimeSeconds, nodeRuns, nodeTE, structure.TEBonusPct)

		reqs := matsByBP[subBP.BlueprintTypeID]
		children := make([]IndustryTreeNode, 0, len(reqs))
		childActiveTotal := 0.0
		childDirectBuyTotal := 0.0

		for _, req := range reqs {
			adjQty := ComputeAdjustedMaterialQty(req.BaseQuantity, nodeRuns, nodeME, structure.MEBonusPct)
			child := buildNode(req.MaterialTypeID, req.MaterialName, adjQty, req.VolumeM3, depth+1, nil)
			children = append(children, child)
			childActiveTotal += child.ActiveCost
			childDirectBuyTotal += child.BuyCost
		}
		node.Children = children

		// Estimate job install fee (SCI + Facility Tax on direct input EIV proxy)
		jobFee := RoundISK(childDirectBuyTotal * feeRate)
		node.JobInstallCost = jobFee
		node.MakeCost = RoundISK(childActiveTotal + jobFee)
		node.SavingsISK = RoundISK(node.BuyCost - node.MakeCost)

		if depth == 0 {
			node.RecommendedDecision = DecisionMake
			node.ActiveDecision = DecisionMake
			node.ActiveCost = node.MakeCost
			return node
		}

		// Automatic Make-vs-Buy recommendation for sub-components
		if node.MakeCost > 0 && (node.BuyCost <= 0 || node.MakeCost < node.BuyCost) {
			node.RecommendedDecision = DecisionMake
		} else {
			node.RecommendedDecision = DecisionBuy
		}

		node.ActiveDecision = node.RecommendedDecision
		if in.MakeBuyOverrides != nil {
			if ov, ok := in.MakeBuyOverrides[typeID]; ok && (ov == DecisionMake || ov == DecisionBuy) {
				node.ActiveDecision = ov
				node.IsOverridden = (ov != node.RecommendedDecision)
			}
		}

		if node.ActiveDecision == DecisionMake {
			node.ActiveCost = node.MakeCost
		} else {
			node.ActiveCost = node.BuyCost
		}
		return node
	}

	rootQty := int64(runs * rootBP.ProductQtyPerRun)
	if rootQty <= 0 {
		rootQty = int64(runs)
	}
	root := buildNode(rootBP.ProductTypeID, rootBP.ProductName, rootQty, 0, 0, &rootBP)

	// Flatten pre-order for clean table rendering
	flat := make([]IndustryTreeNode, 0, 24)
	var flatten func(n IndustryTreeNode)
	flatten = func(n IndustryTreeNode) {
		flat = append(flat, n)
		for _, ch := range n.Children {
			flatten(ch)
		}
	}
	flatten(root)

	// Compute 3-scenario totals:
	// 1. BuyAllDirectCost = sum of direct children BuyCost + root job fee
	buyAllDirect := root.JobInstallCost
	for _, ch := range root.Children {
		buyAllDirect += ch.BuyCost
	}
	buyAllDirect = RoundISK(buyAllDirect)

	// 2. MakeAllCost = recursively force MAKE on every manufacturable node
	var calcMakeAll func(n IndustryTreeNode) float64
	calcMakeAll = func(n IndustryTreeNode) float64 {
		if len(n.Children) == 0 {
			return n.BuyCost
		}
		sum := n.JobInstallCost
		for _, ch := range n.Children {
			sum += calcMakeAll(ch)
		}
		return RoundISK(sum)
	}
	makeAll := calcMakeAll(root)

	// 3. OptimalCost (active cost respecting overrides) + Consolidated Optimal Multibuy + Sub-Job Queue
	type buyAgg struct {
		Name string
		Qty  int64
		Vol  float64
	}
	toBuy := make(map[int32]*buyAgg)
	subJobsByBP := make(map[int32]*IndustrySubJob)

	var collectExec func(n IndustryTreeNode)
	collectExec = func(n IndustryTreeNode) {
		if n.Depth > 0 && (n.ActiveDecision == DecisionBuy || n.ActiveDecision == DecisionRaw || len(n.Children) == 0) {
			if cur, ok := toBuy[n.TypeID]; ok {
				cur.Qty += n.Quantity
				cur.Vol += n.VolumeM3
			} else {
				toBuy[n.TypeID] = &buyAgg{Name: n.Name, Qty: n.Quantity, Vol: n.VolumeM3}
			}
			return
		}
		// Node is MAKE: traverse children first, then register this manufacturing job
		for _, ch := range n.Children {
			collectExec(ch)
		}
		if n.BlueprintID > 0 {
			if cur, ok := subJobsByBP[n.BlueprintID]; ok {
				cur.Runs += n.RunsRequired
				cur.OutputQty += n.Quantity
				cur.DurationSeconds += n.BuildSeconds
				cur.JobInstallCost = RoundISK(cur.JobInstallCost + n.JobInstallCost)
				cur.DurationLabel = FormatDurationSeconds(cur.DurationSeconds)
				if n.Depth > cur.Depth {
					cur.Depth = n.Depth
				}
			} else {
				subJobsByBP[n.BlueprintID] = &IndustrySubJob{
					Depth:           n.Depth,
					BlueprintID:     n.BlueprintID,
					ProductTypeID:   n.TypeID,
					ProductName:     n.Name,
					Runs:            n.RunsRequired,
					OutputQty:       n.Quantity,
					DurationSeconds: n.BuildSeconds,
					DurationLabel:   FormatDurationSeconds(n.BuildSeconds),
					JobInstallCost:  n.JobInstallCost,
				}
			}
		}
	}
	collectExec(root)

	// Format Optimal Multibuy sorted alphabetically
	buyList := make([]buyAgg, 0, len(toBuy))
	optVol := 0.0
	for _, b := range toBuy {
		buyList = append(buyList, *b)
		optVol += b.Vol
	}
	sort.Slice(buyList, func(i, j int) bool { return buyList[i].Name < buyList[j].Name })
	mbLines := make([]string, 0, len(buyList))
	for _, b := range buyList {
		mbLines = append(mbLines, fmt.Sprintf("%s\t%d", b.Name, b.Qty))
	}

	// Order SubJobs bottom-up (deepest sub-components first, root final build last)
	subJobs := make([]IndustrySubJob, 0, len(subJobsByBP))
	totalSubSec := int64(0)
	for _, j := range subJobsByBP {
		subJobs = append(subJobs, *j)
		totalSubSec += j.DurationSeconds
	}
	sort.Slice(subJobs, func(i, j int) bool {
		if subJobs[i].Depth != subJobs[j].Depth {
			return subJobs[i].Depth > subJobs[j].Depth
		}
		return subJobs[i].ProductName < subJobs[j].ProductName
	})
	for i := range subJobs {
		subJobs[i].StepOrder = i + 1
	}

	savings := RoundISK(buyAllDirect - root.ActiveCost)
	savingsPct := 0.0
	if buyAllDirect > 0 {
		savingsPct = math.Round((savings/buyAllDirect)*1000) / 10
	}

	return &IndustryTreeAnalysis{
		Hub:                hub,
		RootNode:           root,
		FlatNodes:          flat,
		HasSubBlueprints:   hasSubBPs,
		BuyAllDirectCost:   buyAllDirect,
		MakeAllCost:        makeAll,
		OptimalCost:        root.ActiveCost,
		OptimalSavingsISK:  savings,
		OptimalSavingsPct:  savingsPct,
		OptimalVolumeM3:    math.Round(optVol*100) / 100,
		OptimalMultibuy:    strings.Join(mbLines, "\n"),
		SubJobs:            subJobs,
		TotalSubJobSeconds: totalSubSec,
		TotalSubJobLabel:   FormatDurationSeconds(totalSubSec),
		ActiveOverrides:    in.MakeBuyOverrides,
	}
}
