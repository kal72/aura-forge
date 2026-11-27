package domain

import "testing"

func TestParsePastedCorpBlueprints(t *testing.T) {
	raw := `Name	Quantity	Group	Category	Slot	Volume
Hobgoblin I Blueprint	1	Drone Blueprint	Blueprint		0.01 m3	Original	ME: 10	TE: 20
Damage Control II Blueprint	1	Module Blueprint	Copy	ME: 2	TE: 4	10 runs
Caracal Blueprint ME8 TE16
Venture Blueprint BPC 5 runs
`
	bps := ParsePastedCorpBlueprints(2122063793, raw, "Corp Hangar 2", 10, 20)
	if len(bps) != 4 {
		t.Fatalf("expected 4 parsed blueprints, got %d: %+v", len(bps), bps)
	}

	if bps[0].TypeName != "Hobgoblin I Blueprint" || bps[0].IsCopy || bps[0].MaterialEfficiency != 10 || bps[0].TimeEfficiency != 20 {
		t.Errorf("unexpected first blueprint: %+v", bps[0])
	}
	if bps[1].TypeName != "Damage Control II Blueprint" || !bps[1].IsCopy || bps[1].MaterialEfficiency != 2 || bps[1].TimeEfficiency != 4 || bps[1].Runs != 10 {
		t.Errorf("unexpected second blueprint: %+v", bps[1])
	}
	if bps[2].TypeName != "Caracal Blueprint" || bps[2].IsCopy || bps[2].MaterialEfficiency != 8 || bps[2].TimeEfficiency != 16 || bps[2].LocationFlag != "Corp Hangar 2" {
		t.Errorf("unexpected third blueprint: %+v", bps[2])
	}
	if bps[3].TypeName != "Venture Blueprint" || !bps[3].IsCopy || bps[3].Runs != 5 {
		t.Errorf("unexpected fourth blueprint: %+v", bps[3])
	}
	for _, bp := range bps {
		if bp.ItemID >= 0 || !bp.IsCorp {
			t.Errorf("expected negative synthetic ItemID and IsCorp=true, got %+v", bp)
		}
	}
}
