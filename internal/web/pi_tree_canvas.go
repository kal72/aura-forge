package web

import (
	"encoding/json"
	"fmt"

	"aura-forge/internal/domain"
)

// PICanvasNode is a positioned node card for the horizontal pan-and-zoom PI recipe canvas.
type PICanvasNode struct {
	NodeID        string
	ParentID      string
	TypeID        int32
	Name          string
	Tier          domain.PITier
	Quantity      int
	SourcePlanets []string
	X             int // Output -> Raw (root on left)
	RevX          int // Raw -> Output (P0 on left, root on right)
	Y             int
	Width         int
	Height        int
	IsRoot        bool
	CanOpenRecipe bool
}

// PICanvasEdge is a smooth cubic Bezier SVG connector between a parent node and a child input node.
type PICanvasEdge struct {
	FromID   string
	ToID     string
	PathD    string // Output -> Raw
	RevPathD string // Raw -> Output
	Tier     domain.PITier
}

// PICanvasLayout holds the full computed 2D geometry for a horizontal PI recipe tree,
// supporting both Output->Raw and Raw->Output directions plus O(1) branch highlighting.
type PICanvasLayout struct {
	Nodes         []PICanvasNode
	Edges         []PICanvasEdge
	Width         int
	Height        int
	BranchMapJSON string // JSON map[nodeID] -> []nodeID (ancestors + self + descendants)
}

const (
	piCanvasNodeW   = 216
	piCanvasNodeH   = 68
	piCanvasColStep = 280
	piCanvasRowStep = 86
	piCanvasPadX    = 36
	piCanvasPadY    = 36
)

// BuildPIHorizontalCanvasLayout computes clean, non-overlapping horizontal (X, Y) coordinates
// in both Output->Raw and Raw->Output orientations, smooth cubic Bezier connectors, and
// a branch-membership map for instant node focus highlighting.
func BuildPIHorizontalCanvasLayout(root domain.PITreeNode) PICanvasLayout {
	layout := PICanvasLayout{
		Nodes: make([]PICanvasNode, 0, 32),
		Edges: make([]PICanvasEdge, 0, 32),
	}

	leafIndex := 0
	nextNodeSeq := 0
	maxX := 0
	maxY := 0

	// Track tree topology for branch highlighting
	parentOf := make(map[string]string)
	childrenOf := make(map[string][]string)

	type rawEdge struct {
		fromID string
		toID   string
		px, py int
		cx, cy int
		tier   domain.PITier
	}
	rawEdges := make([]rawEdge, 0, 32)

	var place func(node domain.PITreeNode, depth int, parentID string) (nodeID string, x int, y int)
	place = func(node domain.PITreeNode, depth int, parentID string) (string, int, int) {
		nodeID := fmt.Sprintf("n%d", nextNodeSeq)
		nextNodeSeq++

		parentOf[nodeID] = parentID
		if parentID != "" {
			childrenOf[parentID] = append(childrenOf[parentID], nodeID)
		}

		x := piCanvasPadX + depth*piCanvasColStep
		var y int

		if len(node.Children) == 0 {
			y = piCanvasPadY + leafIndex*piCanvasRowStep
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
					fromID: nodeID,
					toID:   childIDs[i],
					px:     x,
					py:     y,
					cx:     childXs[i],
					cy:     childYs[i],
					tier:   ch.Tier,
				})
			}
		}

		layout.Nodes = append(layout.Nodes, PICanvasNode{
			NodeID:        nodeID,
			ParentID:      parentID,
			TypeID:        node.TypeID,
			Name:          node.Name,
			Tier:          node.Tier,
			Quantity:      node.Quantity,
			SourcePlanets: node.SourcePlanets,
			X:             x,
			Y:             y,
			Width:         piCanvasNodeW,
			Height:        piCanvasNodeH,
			IsRoot:        depth == 0,
			CanOpenRecipe: depth > 0 && node.Tier != domain.PITierP0,
		})

		if x+piCanvasNodeW > maxX {
			maxX = x + piCanvasNodeW
		}
		if y+piCanvasNodeH > maxY {
			maxY = y + piCanvasNodeH
		}
		return nodeID, x, y
	}

	place(root, 0, "")

	layout.Width = maxX + piCanvasPadX
	layout.Height = maxY + piCanvasPadY
	if layout.Width < 520 {
		layout.Width = 520
	}
	if layout.Height < 260 {
		layout.Height = 260
	}

	// Compute exact horizontal mirror X (Raw -> Output) for every node
	for i := range layout.Nodes {
		layout.Nodes[i].RevX = layout.Width - layout.Nodes[i].X - layout.Nodes[i].Width
	}

	// Compute both Output->Raw and Raw->Output smooth cubic Bezier SVG paths
	for _, re := range rawEdges {
		// Output -> Raw: parent right edge -> child left edge
		pRightX := re.px + piCanvasNodeW
		pMidY := re.py + piCanvasNodeH/2
		cLeftX := re.cx
		cMidY := re.cy + piCanvasNodeH/2
		midX := (pRightX + cLeftX) / 2
		pathD := fmt.Sprintf("M %d %d C %d %d, %d %d, %d %d", pRightX, pMidY, midX, pMidY, midX, cMidY, cLeftX, cMidY)

		// Raw -> Output: child right edge -> parent left edge
		revCX := layout.Width - re.cx - piCanvasNodeW
		revPX := layout.Width - re.px - piCanvasNodeW
		revCRightX := revCX + piCanvasNodeW
		revPLeftX := revPX
		revMidX := (revCRightX + revPLeftX) / 2
		revPathD := fmt.Sprintf("M %d %d C %d %d, %d %d, %d %d", revCRightX, cMidY, revMidX, cMidY, revMidX, pMidY, revPLeftX, pMidY)

		layout.Edges = append(layout.Edges, PICanvasEdge{
			FromID:   re.fromID,
			ToID:     re.toID,
			PathD:    pathD,
			RevPathD: revPathD,
			Tier:     re.tier,
		})
	}

	// Build branch membership map: for each node, include ancestors + self + all descendants
	branchMap := make(map[string][]string, len(layout.Nodes))
	var collectDescendants func(id string, out *[]string)
	collectDescendants = func(id string, out *[]string) {
		for _, ch := range childrenOf[id] {
			*out = append(*out, ch)
			collectDescendants(ch, out)
		}
	}
	for _, n := range layout.Nodes {
		members := []string{n.NodeID}
		for cur := parentOf[n.NodeID]; cur != ""; cur = parentOf[cur] {
			members = append(members, cur)
		}
		collectDescendants(n.NodeID, &members)
		branchMap[n.NodeID] = members
	}

	if raw, err := json.Marshal(branchMap); err == nil {
		layout.BranchMapJSON = string(raw)
	} else {
		layout.BranchMapJSON = "{}"
	}

	return layout
}

func piCanvasEdgeStroke(t domain.PITier) string {
	switch t {
	case domain.PITierP3:
		return "rgba(167, 139, 250, 0.58)" // violet
	case domain.PITierP2:
		return "rgba(56, 189, 248, 0.58)" // sky
	case domain.PITierP1:
		return "rgba(52, 211, 153, 0.58)" // emerald
	default:
		return "rgba(251, 191, 36, 0.48)" // amber for P0
	}
}

func piCanvasNodeBorderClass(t domain.PITier, isRoot bool) string {
	if isRoot {
		return "border-amber-300/75 bg-slate-900/95 shadow-[0_0_20px_rgba(251,191,36,0.22)]"
	}
	switch t {
	case domain.PITierP3:
		return "border-violet-400/55 bg-slate-900/90"
	case domain.PITierP2:
		return "border-sky-400/50 bg-slate-900/90"
	case domain.PITierP1:
		return "border-emerald-400/45 bg-slate-900/90"
	default:
		return "border-slate-500/45 bg-slate-950/90"
	}
}
