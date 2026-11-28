package domain

import (
	"math"
	"strings"
	"testing"
)

func TestStackingPenalty_OfficialEVEValues(t *testing.T) {
	cases := []struct {
		rank int
		want float64
	}{
		{0, 1.000},
		{1, 0.869},
		{2, 0.571},
		{3, 0.283},
	}
	for _, tc := range cases {
		got := StackingPenalty(tc.rank)
		if math.Abs(got-tc.want) > 0.002 {
			t.Errorf("StackingPenalty(%d) = %.4f, want ~%.3f", tc.rank, got, tc.want)
		}
	}
}

func TestParseAndExportEFT_RoundTrip(t *testing.T) {
	raw := `[Caracal, L2 Guristas Starter]

Ballistic Control System II
Ballistic Control System II
Damage Control II

Large Shield Extender II
Multispectrum Shield Hardener II
10MN Afterburner II

Rapid Light Missile Launcher II, Scourge Light Missile
Rapid Light Missile Launcher II, Scourge Light Missile

Medium Core Defense Field Extender I


Hobgoblin II x2
`
	parsed, err := ParseEFT(raw)
	if err != nil {
		t.Fatalf("ParseEFT failed: %v", err)
	}
	if parsed.ShipName != "Caracal" || parsed.FitName != "L2 Guristas Starter" {
		t.Fatalf("unexpected header: ship=%q fit=%q", parsed.ShipName, parsed.FitName)
	}
	if len(parsed.Low) != 3 || len(parsed.Mid) != 3 || len(parsed.High) != 2 || len(parsed.Rig) != 1 || len(parsed.Drones) != 1 {
		t.Fatalf("unexpected slot counts: low=%d mid=%d high=%d rig=%d drones=%d",
			len(parsed.Low), len(parsed.Mid), len(parsed.High), len(parsed.Rig), len(parsed.Drones))
	}
	if parsed.High[0].ChargeName != "Scourge Light Missile" {
		t.Errorf("expected charge Scourge Light Missile, got %q", parsed.High[0].ChargeName)
	}
	if parsed.Drones[0].Name != "Hobgoblin II" || parsed.Drones[0].Quantity != 2 {
		t.Errorf("unexpected drone entry: %+v", parsed.Drones[0])
	}

	fit := ShipFit{
		Name:     parsed.FitName,
		ShipName: parsed.ShipName,
		Modules: []FitModuleSlot{
			{SlotKind: SlotLow, SlotIndex: 0, Name: "Damage Control II", State: ModulePassive},
			{SlotKind: SlotHigh, SlotIndex: 0, Name: "Rapid Light Missile Launcher II", ChargeName: "Scourge Light Missile", State: ModuleActive},
		},
		Drones: []FitDroneEntry{{Name: "Hobgoblin II", Quantity: 2, Active: true}},
	}
	exported := ExportEFT(fit)
	if !strings.Contains(exported, "[Caracal, L2 Guristas Starter]") || !strings.Contains(exported, "Rapid Light Missile Launcher II, Scourge Light Missile") || !strings.Contains(exported, "Hobgoblin II x2") {
		t.Fatalf("unexpected ExportEFT output:\n%s", exported)
	}
	multibuy := FormatMultibuy(fit)
	if !strings.Contains(multibuy, "Caracal x1") || !strings.Contains(multibuy, "Hobgoblin II x2") {
		t.Fatalf("unexpected Multibuy output:\n%s", multibuy)
	}
}

func TestComputeFitSimulation_EHPAlignCapAndFactionHole(t *testing.T) {
	ship := &ItemSpecDetail{
		TypeID:   621,
		Name:     "Caracal",
		GroupName: "Cruiser",
		IsShip:   true,
		Mass:     11_550_000,
		Capacity: 400,
		LiveSellPrice: 12_500_000,
		Slots: SlotLayout{HighSlots: 5, MidSlots: 5, LowSlots: 4, RigSlots: 3, LauncherSlots: 5},
		ShieldResists: ResistanceProfile{EM: 0, Thermal: 20, Kinetic: 40, Explosive: 50},
		ArmorResists:  ResistanceProfile{EM: 50, Thermal: 45, Kinetic: 25, Explosive: 10},
		Attributes: map[int32]float64{
			48: 400, 11: 750, 1132: 400,
			263: 2200, 265: 1300, 9: 1500,
			482: 1200, 55: 250000,
			37: 235, 600: 3.0, 70: 0.52, 552: 125,
			283: 10, 1271: 10,
		},
	}

	lse := &ItemSpecDetail{
		TypeID: 3841, Name: "Large Shield Extender II", TechLevel: 2, LiveSellPrice: 1_800_000,
		Attributes: map[int32]float64{50: 40, 30: 165, 72: 1700, 554: 25},
	}
	rlml := &ItemSpecDetail{
		TypeID: 2410, Name: "Rapid Light Missile Launcher II", GroupName: "Missile Launcher Rapid Light", TechLevel: 2, LiveSellPrice: 1_200_000,
		Attributes: map[int32]float64{50: 35, 30: 25, 51: 4200},
	}

	fit := ShipFit{
		Name:         "Caracal Test",
		ShipTypeID:   621,
		ShipName:     "Caracal",
		SkillProfile: SkillProfileIntermediate,
		TargetPreset: "sansha", // Sansha deals 55% EM — should trigger Shield EM Hole warning!
		Modules: []FitModuleSlot{
			{SlotKind: SlotMid, SlotIndex: 0, TypeID: 3841, Name: lse.Name, State: ModulePassive},
			{SlotKind: SlotHigh, SlotIndex: 0, TypeID: 2410, Name: rlml.Name, State: ModuleActive},
			{SlotKind: SlotHigh, SlotIndex: 1, TypeID: 2410, Name: rlml.Name, State: ModuleActive},
		},
	}

	res := ComputeFitSimulation(fit, ship, map[int32]*ItemSpecDetail{3841: lse, 2410: rlml}, "Jita 4-4")
	if res == nil {
		t.Fatal("expected non-nil simulation result")
	}
	if !res.Shield.HasHole {
		t.Errorf("expected Shield EM resistance hole to be detected against Sansha")
	}
	if res.TotalEHP <= res.TotalRawHP {
		t.Errorf("expected TotalEHP (%.1f) > TotalRawHP (%.1f)", res.TotalEHP, res.TotalRawHP)
	}
	if res.Offense.MissileDPS <= 0 || res.LaunchersUsed != 2 {
		t.Errorf("expected positive missile DPS and 2 launchers, got dps=%.1f launchers=%d", res.Offense.MissileDPS, res.LaunchersUsed)
	}
	if res.Navigation.AlignTimeSeconds <= 0 {
		t.Errorf("expected positive align time, got %.2f", res.Navigation.AlignTimeSeconds)
	}
	if res.TotalFitCostISK != 16_700_000 {
		t.Errorf("expected TotalFitCostISK=16700000, got %.2f", res.TotalFitCostISK)
	}
}
