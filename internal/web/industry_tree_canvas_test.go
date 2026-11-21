package web

import (
	"encoding/json"
	"testing"

	"aura-forge/internal/domain"
)

func TestBuildIndustryHorizontalCanvasLayout(t *testing.T) {
	root := domain.IndustryTreeNode{
		TypeID:         2456,
		Name:           "Hobgoblin II",
		Quantity:       10,
		Depth:          0,
		BlueprintID:    2457,
		ActiveDecision: domain.DecisionMake,
		ActiveCost:     500_000,
		Children: []domain.IndustryTreeNode{
			{
				TypeID:         2454,
				Name:           "Hobgoblin I",
				Quantity:       10,
				Depth:          1,
				BlueprintID:    2455,
				ActiveDecision: domain.DecisionMake,
				ActiveCost:     40_000,
				Children: []domain.IndustryTreeNode{
					{TypeID: 34, Name: "Tritanium", Quantity: 9000, Depth: 2, ActiveDecision: domain.DecisionRaw, ActiveCost: 36_000},
				},
			},
			{
				TypeID:         11483,
				Name:           "R.A.M.-Robotics",
				Quantity:       10,
				Depth:          1,
				BlueprintID:    11485,
				ActiveDecision: domain.DecisionBuy, // children hidden when BUY!
				ActiveCost:     5_000,
				Children: []domain.IndustryTreeNode{
					{TypeID: 34, Name: "Tritanium", Quantity: 5000, Depth: 2, ActiveDecision: domain.DecisionRaw, ActiveCost: 20_000},
				},
			},
		},
	}

	layout := BuildIndustryHorizontalCanvasLayout(root)
	// Root + Hobgoblin I + Tritanium (under Hob I) + R.A.M.-Robotics (BUY, collapsed) = 4 nodes
	if len(layout.Nodes) != 4 {
		t.Fatalf("expected 4 visible canvas nodes (BUY child collapsed), got %d", len(layout.Nodes))
	}
	if len(layout.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(layout.Edges))
	}

	var branchMap map[string][]string
	if err := json.Unmarshal([]byte(layout.BranchMapJSON), &branchMap); err != nil {
		t.Fatalf("invalid BranchMapJSON: %v", err)
	}
	if len(branchMap) != 4 {
		t.Fatalf("expected 4 branchMap entries, got %d", len(branchMap))
	}
}
