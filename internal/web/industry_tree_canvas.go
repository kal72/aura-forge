package web

import (
	"encoding/json"
	"fmt"

	"aura-forge/internal/domain"
)

// IndustryCanvasNode is a positioned node card in the horizontal pan-and-zoom Industry Tree Canvas.
type IndustryCanvasNode struct {
	NodeID         string
	ParentID       string
	TypeID         int32
	Name           string
	Quantity       int64
	Decision       domain.IndustryDecision
	IsOverridden   bool
	ActiveCost     float64
	SavingsISK     float64
	X              int // Output -> Raw (root on left)
	RevX           int // Raw -> Output (raw on left, root on right)
	Y              int
	Width          int
	Height         int
	IsRoot         bool
	CanToggleMake  bool
}

// IndustryCanvasEdge is a smooth cubic Bezier SVG connector between parent and child BOM nodes.
type IndustryCanvasEdge struct {
	FromID   string
	ToID     string
	PathD    string
	RevPathD string
	Decision domain.IndustryDecision
}

// IndustryCanvasLayout holds the 2D geometry for a recursive manufacturing tree in both horizontal orientations.
type IndustryCanvasLayout struct {
	Nodes         []IndustryCanvasNode
	Edges         []IndustryCanvasEdge
	Width         int
	Height        int
	BranchMapJSON string
}

const (
	indCanvasNodeW   = 226
	indCanvasNodeH   = 72
	indCanvasColStep = 290
	indCanvasRowStep = 90
	indCanvasPadX    = 36
	indCanvasPadY    = 36
)

// BuildIndustryHorizontalCanvasLayout computes non-overlapping horizontal coordinates, mirrored coordinates,
// smooth cubic Bezier edges, and branch membership JSON for any IndustryTreeNode.
func BuildIndustryHorizontalCanvasLayout(root domain.IndustryTreeNode) IndustryCanvasLayout {
	layout := IndustryCanvasLayout{
		Nodes: make([]IndustryCanvasNode, 0, 32),
		Edges: make([]IndustryCanvasEdge, 0, 32),
	}

	leafIndex := 0
	nextSeq := 0
	maxX := 0
	maxY := 0

	parentOf := make(map[string]string)
	childrenOf := make(map[string][]string)

	type rawEdge struct {
		fromID   string
		toID     string
		px, py   int
		cx, cy   int
		decision domain.IndustryDecision
	}
	rawEdges := make([]rawEdge, 0, 32)

	var place func(node domain.IndustryTreeNode, depth int, parentID string) (string, int, int)
	place = func(node domain.IndustryTreeNode, depth int, parentID string) (string, int, int) {
		nodeID := fmt.Sprintf("i%d", nextSeq)
		nextSeq++

		parentOf[nodeID] = parentID
		if parentID != "" {
			childrenOf[parentID] = append(childrenOf[parentID], nodeID)
		}

		x := indCanvasPadX + depth*indCanvasColStep
		var y int

		// Only expand children visually when root OR active decision is MAKE
		showChildren := len(node.Children) > 0 && (depth == 0 || node.ActiveDecision == domain.DecisionMake)

		if !showChildren {
			y = indCanvasPadY + leafIndex*indCanvasRowStep
			leafIndex++
		} else {
			childIDs := make([]string, len(node.Children))
			childXs := make([]int, len(node.Children))
			childYs := make([]int, len(node.Children))
			for i, ch := range node.Children {
				cid, cx, cy := place(ch, depth+1, nodeID)
				childIDs[i] = cid
				childXs[i] = cx
				childYs[i] = cy
			}
			y = (childYs[0] + childYs[len(childYs)-1]) / 2

			for i, ch := range node.Children {
				rawEdges = append(rawEdges, rawEdge{
					fromID:   nodeID,
					toID:     childIDs[i],
					px:       x,
					py:       y,
					cx:       childXs[i],
					cy:       childYs[i],
					decision: ch.ActiveDecision,
				})
			}
		}

		layout.Nodes = append(layout.Nodes, IndustryCanvasNode{
			NodeID:        nodeID,
			ParentID:      parentID,
			TypeID:        node.TypeID,
			Name:          node.Name,
			Quantity:      node.Quantity,
			Decision:      node.ActiveDecision,
			IsOverridden:  node.IsOverridden,
			ActiveCost:    node.ActiveCost,
			SavingsISK:    node.SavingsISK,
			X:             x,
			Y:             y,
			Width:         indCanvasNodeW,
			Height:        indCanvasNodeH,
			IsRoot:        depth == 0,
			CanToggleMake: depth > 0 && node.BlueprintID > 0,
		})

		if x+indCanvasNodeW > maxX {
			maxX = x + indCanvasNodeW
		}
		if y+indCanvasNodeH > maxY {
			maxY = y + indCanvasNodeH
		}
		return nodeID, x, y
	}

	place(root, 0, "")

	layout.Width = maxX + indCanvasPadX
	layout.Height = maxY + indCanvasPadY
	if layout.Width < 540 {
		layout.Width = 540
	}
	if layout.Height < 260 {
		layout.Height = 260
	}

	for i := range layout.Nodes {
		layout.Nodes[i].RevX = layout.Width - layout.Nodes[i].X - layout.Nodes[i].Width
	}

	for _, re := range rawEdges {
		pRightX := re.px + indCanvasNodeW
		pMidY := re.py + indCanvasNodeH/2
		cLeftX := re.cx
		cMidY := re.cy + indCanvasNodeH/2
		midX := (pRightX + cLeftX) / 2
		pathD := fmt.Sprintf("M %d %d C %d %d, %d %d, %d %d", pRightX, pMidY, midX, pMidY, midX, cMidY, cLeftX, cMidY)

		revCX := layout.Width - re.cx - indCanvasNodeW
		revPX := layout.Width - re.px - indCanvasNodeW
		revCRightX := revCX + indCanvasNodeW
		revPLeftX := revPX
		revMidX := (revCRightX + revPLeftX) / 2
		revPathD := fmt.Sprintf("M %d %d C %d %d, %d %d, %d %d", revCRightX, cMidY, revMidX, cMidY, revMidX, pMidY, revPLeftX, pMidY)

		layout.Edges = append(layout.Edges, IndustryCanvasEdge{
			FromID:   re.fromID,
			ToID:     re.toID,
			PathD:    pathD,
			RevPathD: revPathD,
			Decision: re.decision,
		})
	}

	branchMap := make(map[string][]string, len(layout.Nodes))
	var collectDesc func(id string, out *[]string)
	collectDesc = func(id string, out *[]string) {
		for _, ch := range childrenOf[id] {
			*out = append(*out, ch)
			collectDesc(ch, out)
		}
	}
	for _, n := range layout.Nodes {
		members := []string{n.NodeID}
		for cur := parentOf[n.NodeID]; cur != ""; cur = parentOf[cur] {
			members = append(members, cur)
		}
		collectDesc(n.NodeID, &members)
		branchMap[n.NodeID] = members
	}
	if raw, err := json.Marshal(branchMap); err == nil {
		layout.BranchMapJSON = string(raw)
	} else {
		layout.BranchMapJSON = "{}"
	}

	return layout
}

func indDecisionBadgeClass(d domain.IndustryDecision) string {
	switch d {
	case domain.DecisionMake:
		return "border-emerald-400/55 bg-emerald-500/20 text-emerald-200"
	case domain.DecisionBuy:
		return "border-sky-400/55 bg-sky-500/20 text-sky-200"
	default:
		return "border-slate-500/45 bg-slate-700/35 text-slate-300"
	}
}

func indCanvasEdgeStroke(d domain.IndustryDecision) string {
	switch d {
	case domain.DecisionMake:
		return "rgba(52, 211, 153, 0.62)" // emerald
	case domain.DecisionBuy:
		return "rgba(56, 189, 248, 0.62)" // sky
	default:
		return "rgba(167, 139, 250, 0.45)" // violet/slate
	}
}

func indCanvasNodeBorderClass(d domain.IndustryDecision, isRoot bool) string {
	if isRoot {
		return "border-amber-300/75 bg-slate-900/95 shadow-[0_0_20px_rgba(251,191,36,0.22)]"
	}
	switch d {
	case domain.DecisionMake:
		return "border-emerald-400/55 bg-slate-900/92"
	case domain.DecisionBuy:
		return "border-sky-400/55 bg-slate-900/92"
	default:
		return "border-slate-500/45 bg-slate-950/90"
	}
}
