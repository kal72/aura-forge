package domain

import (
	"strings"
	"testing"
)

func TestBuildItemCompareResult_NoAbbreviatedLabels(t *testing.T) {
	items := []ItemSpecDetail{
		{
			TypeID:         621,
			Name:           "Caracal",
			Capacity:       460,
			PackagedVolume: 10000,
			Attributes: map[int32]float64{
				48:   380,
				11:   740,
				1132: 400,
				482:  1250,
				55:   380000,
				263:  2150,
				265:  1100,
				9:    1300,
				37:   245,
				76:   62500,
				192:  7,
			},
		},
		{
			TypeID:         17634,
			Name:           "Caracal Navy Issue",
			Capacity:       500,
			PackagedVolume: 10000,
			Attributes: map[int32]float64{
				48:   440,
				11:   880,
				1132: 400,
				482:  1500,
				55:   350000,
				263:  2800,
				265:  1400,
				9:    1600,
				37:   265,
				76:   68000,
				192:  8,
			},
		},
	}

	res := BuildItemCompareResult(items, LookupTradeHub("jita"))
	if res == nil || len(res.Sections) < 4 {
		t.Fatalf("expected 4 sections, got %+v", res)
	}

	forbiddenSubstrings := []string{"Cap Recharge", "Max Velocity", "Max Targeting", "Max Locked", " pts"}
	for _, sec := range res.Sections {
		for _, row := range sec.Rows {
			for _, bad := range forbiddenSubstrings {
				if strings.Contains(row.Label, bad) {
					t.Errorf("row label %q contains forbidden abbreviation %q", row.Label, bad)
				}
			}
			for _, cell := range row.Cells {
				if strings.Contains(cell.Formatted, " pts") {
					t.Errorf("formatted value %q contains forbidden abbreviation 'pts'", cell.Formatted)
				}
			}
		}
	}
}
