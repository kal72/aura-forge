package service

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aura-forge/internal/domain"
)

func TestBuildTraitsJSON_ArrayAndMapFormats(t *testing.T) {
	typeInfo := map[int32]ccpTypeRow{
		3334: {Key: 3334, Name: localizedString("Caldari Cruiser")},
	}

	rawJSON := `{
		"roleBonuses": [
			{"bonus": 50, "bonusText": {"en": "bonus to <a href=showinfo:123>Light, Heavy Assault and Heavy Missile</a> velocity"}, "importance": 1, "unitID": 105}
		],
		"types": [
			{"_key": 3334, "_value": [
				{"bonus": 5, "bonusText": {"en": "bonus to <b>Heavy Missile</b> rate of fire"}, "importance": 1, "unitID": 105},
				{"bonus": 10, "bonusText": {"en": "bonus to missile velocity"}, "importance": 2, "unitID": 105}
			]}
		]
	}`

	var raw ccpTraitsRow
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		t.Fatalf("unmarshal traits failed: %v", err)
	}

	encoded := buildTraitsJSON(&raw, typeInfo)
	if encoded == "" {
		t.Fatalf("expected non-empty encoded traits JSON")
	}

	var parsed domain.ItemTraits
	if err := json.Unmarshal([]byte(encoded), &parsed); err != nil {
		t.Fatalf("unmarshal encoded ItemTraits failed: %v", err)
	}
	if len(parsed.RoleBonuses) != 1 || parsed.RoleBonuses[0].Prefix != "50%" {
		t.Fatalf("unexpected role bonuses: %+v", parsed.RoleBonuses)
	}
	if parsed.RoleBonuses[0].Text != "bonus to Light, Heavy Assault and Heavy Missile velocity" {
		t.Fatalf("HTML tags not stripped cleanly: %q", parsed.RoleBonuses[0].Text)
	}
	if len(parsed.SkillBonuses) != 1 || parsed.SkillBonuses[0].SkillName != "Caldari Cruiser" {
		t.Fatalf("unexpected skill bonuses: %+v", parsed.SkillBonuses)
	}
	if len(parsed.SkillBonuses[0].Bonuses) != 2 || parsed.SkillBonuses[0].Bonuses[0].Prefix != "5%" {
		t.Fatalf("unexpected skill bonus entries: %+v", parsed.SkillBonuses[0].Bonuses)
	}

	// Verify map-encoded traits.types format also decodes cleanly
	mapJSON := `{"types": {"3334": [{"bonus": 7.5, "bonusText": {"en": "bonus to shield booster amount"}, "importance": 1, "unitID": 105}]}}`
	var rawMap ccpTraitsRow
	if err := json.Unmarshal([]byte(mapJSON), &rawMap); err != nil {
		t.Fatalf("unmarshal map traits failed: %v", err)
	}
	encodedMap := buildTraitsJSON(&rawMap, typeInfo)
	var parsedMap domain.ItemTraits
	if err := json.Unmarshal([]byte(encodedMap), &parsedMap); err != nil {
		t.Fatalf("unmarshal encoded map ItemTraits failed: %v", err)
	}
	if len(parsedMap.SkillBonuses) != 1 || parsedMap.SkillBonuses[0].Bonuses[0].Prefix != "7.5%" {
		t.Fatalf("unexpected map skill bonus: %+v", parsedMap.SkillBonuses)
	}
}

func TestParseCCPSDEZipFull_ReadsTypeBonusJSONL(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "sde.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	writeEntry := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}

	writeEntry("categories.jsonl", `{"_key":6,"name":{"en":"Ship"}}`+"\n")
	writeEntry("groups.jsonl", `{"_key":26,"categoryID":6,"name":{"en":"Cruiser"}}`+"\n")
	writeEntry("types.jsonl",
		`{"_key":621,"groupID":26,"marketGroupID":77,"name":{"en":"Caracal"},"volume":92000,"portionSize":1,"published":true}`+"\n"+
			`{"_key":3334,"groupID":26,"marketGroupID":0,"name":{"en":"Caldari Cruiser"},"volume":0.01,"portionSize":1,"published":true}`+"\n",
	)
	writeEntry("typeBonus.jsonl",
		`{"_key":621,"roleBonuses":[{"bonus":50,"bonusText":{"en":"bonus to Light, Heavy Assault and Heavy Missile velocity"},"importance":1,"unitID":105}],"types":[{"_key":3334,"_value":[{"bonus":5,"bonusText":{"en":"bonus to Rapid Light, Heavy Assault and Heavy Missile Launcher rate of fire"},"importance":1,"unitID":105}]}]}`+"\n",
	)
	writeEntry("blueprints.jsonl", "")

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	types, _, _, _, _, _, err := ParseCCPSDEZipFull(zipPath)
	if err != nil {
		t.Fatalf("ParseCCPSDEZipFull failed: %v", err)
	}
	var caracal *domain.SDEType
	for i := range types {
		if types[i].TypeID == 621 {
			caracal = &types[i]
			break
		}
	}
	if caracal == nil {
		t.Fatalf("Caracal (621) not found in parsed types")
	}
	if caracal.TraitsJSON == "" {
		t.Fatalf("expected non-empty TraitsJSON for Caracal from typeBonus.jsonl")
	}
	var tr domain.ItemTraits
	if err := json.Unmarshal([]byte(caracal.TraitsJSON), &tr); err != nil {
		t.Fatalf("unmarshal Caracal TraitsJSON: %v", err)
	}
	if len(tr.RoleBonuses) != 1 || len(tr.SkillBonuses) != 1 || tr.SkillBonuses[0].SkillName != "Caldari Cruiser" {
		t.Fatalf("unexpected Caracal traits: %+v", tr)
	}
}
