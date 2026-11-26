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
{"_key":25,"name":{"en":"Asteroid"}}
{"_key":17,"name":{"en":"Commodity"}}
`)
	writeEntry("groups.jsonl", `{"_key":100,"categoryID":18,"name":{"en":"Combat Drone"}}
{"_key":18,"categoryID":4,"name":{"en":"Mineral"}}
{"_key":462,"categoryID":25,"name":{"en":"Veldspar"}}
{"_key":1875,"categoryID":17,"name":{"en":"PLEX"}}
`)
	writeEntry("types.jsonl", `{"_key":34,"groupID":18,"marketGroupID":1857,"name":{"en":"Tritanium"},"volume":0.01,"portionSize":1,"published":true}
{"_key":2456,"groupID":100,"marketGroupID":837,"name":{"en":"Hobgoblin II"},"volume":5.0,"portionSize":1,"metaGroupID":2,"published":true}
{"_key":2457,"groupID":100,"name":{"en":"Hobgoblin II Blueprint"},"volume":0.01,"portionSize":1,"published":true}
{"_key":30305,"groupID":18,"marketGroupID":1859,"name":{"en":"C3-FTM Acid"},"volume":1.0,"portionSize":1,"published":true}
{"_key":30306,"groupID":18,"name":{"en":"C3-FTM Acid Reaction Formula"},"volume":0.01,"portionSize":1,"published":true}
{"_key":1230,"groupID":462,"marketGroupID":512,"name":{"en":"Veldspar"},"volume":0.1,"portionSize":100,"published":true}
{"_key":44992,"groupID":1875,"marketGroupID":1923,"name":{"en":"PLEX"},"volume":0.01,"portionSize":1,"published":true}
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
	if len(types) != 5 {
		t.Fatalf("expected 5 types (3 blueprint refs + Veldspar + PLEX market items), got %d", len(types))
	}
	var foundVeldspar, foundPLEX bool
	for _, tp := range types {
		if tp.TypeID == 1230 && tp.Category == "Ore & Ice" {
			foundVeldspar = true
		}
		if tp.TypeID == 44992 && tp.Category == "Commodities & PI" {
			foundPLEX = true
		}
	}
	if !foundVeldspar || !foundPLEX {
		t.Fatalf("expected non-blueprint published market items Veldspar (Ore & Ice) and PLEX (Commodities & PI), got %+v", types)
	}
	if matCount != 2 || len(mats[2457]) != 1 || len(mats[30306]) != 1 {
		t.Fatalf("unexpected material lines: total=%d mats=%+v", matCount, mats)
	}

	var foundT2, foundReaction bool
	for _, bp := range bps {
		if bp.BlueprintTypeID == 2457 && bp.TechLevel == 2 && bp.Category == "Drones" && bp.ActivityType == "manufacturing" {
			foundT2 = true
		}
		if bp.BlueprintTypeID == 30306 && bp.Category == "Reactions" && bp.ActivityType == "reaction" && bp.ProductQtyPerRun == 100 {
			foundReaction = true
		}
	}
	if !foundT2 || !foundReaction {
		t.Fatalf("expected T2 Drone (manufacturing) & Reaction blueprints parsed accurately, got %+v", bps)
	}
}

func TestParseCCPSDEZipFull_ExtractsInventions(t *testing.T) {
	tmp, err := os.CreateTemp("", "test-ccp-sde-inv-*.zip")
	if err != nil {
		t.Fatalf("temp file: %v", err)
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

	writeEntry("categories.jsonl", `{"_key":18,"name":{"en":"Drone"}}
{"_key":16,"name":{"en":"Skill"}}
`)
	writeEntry("groups.jsonl", `{"_key":100,"categoryID":18,"name":{"en":"Combat Drone"}}
{"_key":333,"categoryID":16,"name":{"en":"Datacores"}}
`)
	writeEntry("types.jsonl", `{"_key":34,"groupID":18,"name":{"en":"Tritanium"},"volume":0.01,"portionSize":1,"published":true}
{"_key":20410,"groupID":333,"marketGroupID":973,"name":{"en":"Datacore - Gallente Starship Engineering"},"volume":0.1,"portionSize":1,"published":true}
{"_key":2454,"groupID":100,"marketGroupID":837,"name":{"en":"Hobgoblin I"},"volume":5.0,"portionSize":1,"published":true}
{"_key":2455,"groupID":100,"name":{"en":"Hobgoblin I Blueprint"},"volume":0.01,"portionSize":1,"published":true}
{"_key":2456,"groupID":100,"marketGroupID":837,"name":{"en":"Hobgoblin II"},"volume":5.0,"portionSize":1,"metaGroupID":2,"published":true}
{"_key":2457,"groupID":100,"name":{"en":"Hobgoblin II Blueprint"},"volume":0.01,"portionSize":1,"published":true}
`)
	writeEntry("blueprints.jsonl", `{"_key":2455,"activities":{"manufacturing":{"time":1200,"materials":[{"typeID":34,"quantity":200}],"products":[{"typeID":2454,"quantity":1}]},"invention":{"time":63900,"materials":[{"typeID":20410,"quantity":2}],"products":[{"typeID":2457,"quantity":10,"probability":0.34}]}}}
{"_key":2457,"activities":{"manufacturing":{"time":5400,"materials":[{"typeID":34,"quantity":1000}],"products":[{"typeID":2456,"quantity":1}]}}}
`)
	_ = zw.Close()
	_ = tmp.Close()

	_, _, _, _, inventions, invMats, err := ParseCCPSDEZipFull(zipPath)
	if err != nil {
		t.Fatalf("ParseCCPSDEZipFull failed: %v", err)
	}
	if len(inventions) != 1 {
		t.Fatalf("expected 1 invention pathway, got %d", len(inventions))
	}
	inv := inventions[0]
	if inv.T1BlueprintTypeID != 2455 || inv.T2BlueprintTypeID != 2457 || inv.T2ProductTypeID != 2456 || inv.T2ProductName != "Hobgoblin II" {
		t.Fatalf("unexpected invention row: %+v", inv)
	}
	if inv.BaseProbability != 0.34 || inv.BaseRuns != 10 {
		t.Fatalf("expected prob=0.34 runs=10, got prob=%.2f runs=%d", inv.BaseProbability, inv.BaseRuns)
	}
	if len(invMats[2455]) != 1 || invMats[2455][0].MaterialTypeID != 20410 || invMats[2455][0].Quantity != 2 {
		t.Fatalf("unexpected invention datacores: %+v", invMats[2455])
	}
}
