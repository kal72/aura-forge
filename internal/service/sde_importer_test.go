package service

import (
	"archive/zip"
	"os"
	"testing"
)

func TestParseCCPSDEZip_ManufacturingAndReaction(t *testing.T) {
	tmp, err := os.CreateTemp("", "test-ccp-sde-*.zip")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	zipPath := tmp.Name()
	defer func() { _ = os.Remove(zipPath) }()

	zw := zip.NewWriter(tmp)
	writeEntry := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}

	writeEntry("categories.jsonl", `{"_key":6,"name":{"en":"Ship"}}
{"_key":4,"name":{"en":"Material"}}
{"_key":18,"name":{"en":"Drone"}}
`)
	writeEntry("groups.jsonl", `{"_key":100,"categoryID":18,"name":{"en":"Combat Drone"}}
{"_key":18,"categoryID":4,"name":{"en":"Mineral"}}
`)
	writeEntry("types.jsonl", `{"_key":34,"groupID":18,"name":{"en":"Tritanium"},"volume":0.01,"portionSize":1,"published":true}
{"_key":2456,"groupID":100,"name":{"en":"Hobgoblin II"},"volume":5.0,"portionSize":1,"metaGroupID":2,"published":true}
{"_key":2457,"groupID":100,"name":{"en":"Hobgoblin II Blueprint"},"volume":0.01,"portionSize":1,"published":true}
{"_key":30305,"groupID":18,"name":{"en":"C3-FTM Acid"},"volume":1.0,"portionSize":1,"published":true}
{"_key":30306,"groupID":18,"name":{"en":"C3-FTM Acid Reaction Formula"},"volume":0.01,"portionSize":1,"published":true}
`)
	writeEntry("blueprints.jsonl", `{"_key":2457,"activities":{"manufacturing":{"time":5400,"materials":[{"typeID":34,"quantity":1000}],"products":[{"typeID":2456,"quantity":1}]}}}
{"_key":30306,"activities":{"reaction":{"time":10800,"materials":[{"typeID":34,"quantity":500}],"products":[{"typeID":30305,"quantity":100}]}}}
`)

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	_ = tmp.Close()

	types, bps, mats, matCount, err := ParseCCPSDEZip(zipPath)
	if err != nil {
		t.Fatalf("ParseCCPSDEZip failed: %v", err)
	}
	if len(bps) != 2 {
		t.Fatalf("expected 2 blueprints (1 manufacturing + 1 reaction), got %d", len(bps))
	}
	if len(types) != 3 {
		t.Fatalf("expected 3 referenced types (Tritanium, Hobgoblin II, C3-FTM Acid), got %d", len(types))
	}
	if matCount != 2 || len(mats[2457]) != 1 || len(mats[30306]) != 1 {
		t.Fatalf("unexpected material lines: total=%d mats=%+v", matCount, mats)
	}

	var foundT2, foundReaction bool
	for _, bp := range bps {
		if bp.BlueprintTypeID == 2457 && bp.TechLevel == 2 && bp.Category == "Drones" {
			foundT2 = true
		}
		if bp.BlueprintTypeID == 30306 && bp.Category == "Reactions" && bp.ProductQtyPerRun == 100 {
			foundReaction = true
		}
	}
	if !foundT2 || !foundReaction {
		t.Fatalf("expected T2 Drone & Reaction blueprints parsed accurately, got %+v", bps)
	}
}
