package web

import (
	"encoding/json"
	"testing"

	"aura-forge/internal/domain"
)

func TestBuildPIHorizontalCanvasLayout_Robotics(t *testing.T) {
	tree, err := domain.BuildPIRecipeTree(9848, nil, 5.0)
	if err != nil {
		t.Fatalf("BuildPIRecipeTree failed: %v", err)
	}

	layout := BuildPIHorizontalCanvasLayout(tree.RootNode)
	if len(layout.Nodes) != len(tree.FlatTree) {
		t.Fatalf("expected %d canvas nodes, got %d", len(tree.FlatTree), len(layout.Nodes))
	}
	if len(layout.Edges) != len(layout.Nodes)-1 {
		t.Fatalf("expected %d edges for tree with %d nodes, got %d", len(layout.Nodes)-1, len(layout.Nodes), len(layout.Edges))
	}
	if layout.Width < 520 || layout.Height < 260 {
		t.Fatalf("unexpected canvas dimensions: %dx%d", layout.Width, layout.Height)
	}

	// Verify root node is at column 0 in Output->Raw and mirrored at the right in Raw->Output
	foundRoot := false
	for _, n := range layout.Nodes {
		if n.IsRoot {
			foundRoot = true
			if n.X != piCanvasPadX {
				t.Fatalf("expected root X=%d, got %d", piCanvasPadX, n.X)
			}
			if n.RevX != layout.Width-piCanvasPadX-n.Width {
				t.Fatalf("expected mirrored root RevX=%d, got %d", layout.Width-piCanvasPadX-n.Width, n.RevX)
			}
		}
	}
	if !foundRoot {
		t.Fatalf("expected root node to be present")
	}

	for _, e := range layout.Edges {
		if e.FromID == "" || e.ToID == "" || e.PathD == "" || e.RevPathD == "" {
			t.Fatalf("incomplete canvas edge: %+v", e)
		}
	}

	var branchMap map[string][]string
	if err := json.Unmarshal([]byte(layout.BranchMapJSON), &branchMap); err != nil {
		t.Fatalf("invalid BranchMapJSON: %v", err)
	}
	if len(branchMap) != len(layout.Nodes) {
		t.Fatalf("expected %d branchMap entries, got %d", len(layout.Nodes), len(branchMap))
	}
}
