package domain

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// SlotLayout holds a ship's fitting slot counts and turret/launcher hardpoints.
type SlotLayout struct {
	HighSlots    int `json:"high_slots"`
	MidSlots     int `json:"mid_slots"`
	LowSlots     int `json:"low_slots"`
	RigSlots     int `json:"rig_slots"`
	TurretSlots  int `json:"turret_slots"`
	LauncherSlots int `json:"launcher_slots"`
	Subsystems   int `json:"subsystems"`
}

func (s SlotLayout) HasSlots() bool {
	return s.HighSlots > 0 || s.MidSlots > 0 || s.LowSlots > 0 || s.RigSlots > 0
}

// ResistanceProfile holds Shield or Armor damage resistances in percent (0–100).
type ResistanceProfile struct {
	EM        float64 `json:"em"`
	Thermal   float64 `json:"thermal"`
	Kinetic   float64 `json:"kinetic"`
	Explosive float64 `json:"explosive"`
}

func (r ResistanceProfile) HasAny() bool {
	return r.EM > 0 || r.Thermal > 0 || r.Kinetic > 0 || r.Explosive > 0
}

// RequiredSkill represents a skill prerequisite for flying a ship or fitting a module.
type RequiredSkill struct {
	SkillTypeID int32  `json:"skill_type_id"`
	SkillName   string `json:"skill_name"`
	Level       int    `json:"level"`
}

// ItemSpecDetail holds rich technical attributes, visual defense/slot profiles, and live market price for one EVE item.
type ItemSpecDetail struct {
	TypeID          int32             `json:"type_id"`
	Name            string            `json:"name"`
	GroupName       string            `json:"group_name"`
	IsShip          bool              `json:"is_ship"`
	TechLevel       int               `json:"tech_level"`
	MetaLevel       int               `json:"meta_level"`
	Description     string            `json:"description"`
	Mass            float64           `json:"mass"`
	Volume          float64           `json:"volume"`
	PackagedVolume  float64           `json:"packaged_volume"`
	Capacity        float64           `json:"capacity"`
	LiveSellPrice   float64           `json:"live_sell_price"`
	BlueprintTypeID int32             `json:"blueprint_type_id,omitempty"`
	Slots           SlotLayout        `json:"slots"`
	ShieldResists   ResistanceProfile `json:"shield_resists"`
	ArmorResists    ResistanceProfile `json:"armor_resists"`
	RequiredSkills  []RequiredSkill   `json:"required_skills,omitempty"`
	Attributes      map[int32]float64 `json:"attributes"`
}

// CompareMetricCell holds one item's formatted value, relative bar width (0-100), and best-in-row flag.
type CompareMetricCell struct {
	HasValue   bool    `json:"has_value"`
	RawValue   float64 `json:"raw_value"`
	Formatted  string  `json:"formatted"`
	BarPercent float64 `json:"bar_percent"`
	IsBest     bool    `json:"is_best"`
}

// CompareMetricRow represents one aligned attribute row across 1–3 compared items.
type CompareMetricRow struct {
	Key           string              `json:"key"`
	Label         string              `json:"label"`
	LowerIsBetter bool                `json:"lower_is_better"`
	Cells         []CompareMetricCell `json:"cells"`
}

// CompareSection groups related attribute rows (e.g. Fitting, Defense, Navigation, Module Performance).
type CompareSection struct {
	ID    string             `json:"id"`
	Title string             `json:"title"`
	Color string             `json:"color"` // amber | sky | emerald | violet | cyan
	Rows  []CompareMetricRow `json:"rows"`
}

// ItemCompareResult bundles 1–3 resolved EVE items with aligned visual comparison sections.
type ItemCompareResult struct {
	Hub      TradeHub         `json:"hub"`
	Items    []ItemSpecDetail `json:"items"`
	Sections []CompareSection `json:"sections"`
}

type dogmaMetricDef struct {
	AttrID        int32 // 0 = computed from struct field
	FieldKey      string
	SectionID     string
	Label         string
	Unit          string
	Scale         float64 // e.g. 0.001 to convert ms -> s or m -> km
	Decimals      int
	LowerIsBetter bool
	PercentDelta  bool // e.g. multiplier 1.10 -> +10%
}

var compareMetricDefs = []dogmaMetricDef{
	// Fitting & Power
	{AttrID: 48, SectionID: "fitting", Label: "CPU Output", Unit: "tf", Decimals: 0},
	{AttrID: 11, SectionID: "fitting", Label: "Powergrid Output", Unit: "MW", Decimals: 0},
	{AttrID: 1132, SectionID: "fitting", Label: "Calibration", Unit: "pts", Decimals: 0},
	{AttrID: 482, SectionID: "fitting", Label: "Capacitor Capacity", Unit: "GJ", Decimals: 0},
	{AttrID: 55, SectionID: "fitting", Label: "Cap Recharge Time", Unit: "s", Scale: 0.001, Decimals: 1, LowerIsBetter: true},
	{AttrID: 50, SectionID: "fitting", Label: "CPU Usage", Unit: "tf", Decimals: 1, LowerIsBetter: true},
	{AttrID: 30, SectionID: "fitting", Label: "Powergrid Usage", Unit: "MW", Decimals: 1, LowerIsBetter: true},
	{AttrID: 6, SectionID: "fitting", Label: "Activation Cost", Unit: "GJ", Decimals: 1, LowerIsBetter: true},
	{AttrID: 73, SectionID: "fitting", Label: "Cycle Duration", Unit: "s", Scale: 0.001, Decimals: 2, LowerIsBetter: true},

	// Defense & Tank
	{AttrID: 263, SectionID: "defense", Label: "Shield HP", Unit: "HP", Decimals: 0},
	{AttrID: 265, SectionID: "defense", Label: "Armor HP", Unit: "HP", Decimals: 0},
	{AttrID: 9, SectionID: "defense", Label: "Structure HP", Unit: "HP", Decimals: 0},
	{AttrID: 552, SectionID: "defense", Label: "Signature Radius", Unit: "m", Decimals: 0, LowerIsBetter: true},
	{AttrID: 68, SectionID: "defense", Label: "Shield Boost Amount", Unit: "HP", Decimals: 0},
	{AttrID: 84, SectionID: "defense", Label: "Armor Repair Amount", Unit: "HP", Decimals: 0},
	{AttrID: 72, SectionID: "defense", Label: "Shield Capacity Bonus", Unit: "HP", Decimals: 0},
	{AttrID: 1159, SectionID: "defense", Label: "Armor HP Bonus", Unit: "HP", Decimals: 0},

	// Navigation & Cargo
	{AttrID: 37, SectionID: "navigation", Label: "Max Velocity", Unit: "m/s", Decimals: 0},
	{AttrID: 600, SectionID: "navigation", Label: "Warp Speed", Unit: "AU/s", Decimals: 2},
	{AttrID: 70, SectionID: "navigation", Label: "Inertia Modifier", Unit: "×", Decimals: 3, LowerIsBetter: true},
	{FieldKey: "capacity", SectionID: "navigation", Label: "Cargo Capacity", Unit: "m³", Decimals: 0},
	{FieldKey: "packaged_volume", SectionID: "navigation", Label: "Packaged Volume", Unit: "m³", Decimals: 1, LowerIsBetter: true},
	{AttrID: 283, SectionID: "navigation", Label: "Drone Bay", Unit: "m³", Decimals: 0},
	{AttrID: 1271, SectionID: "navigation", Label: "Drone Bandwidth", Unit: "Mbit/s", Decimals: 0},

	// Targeting & Offense / Module Effects
	{AttrID: 76, SectionID: "offense", Label: "Max Targeting Range", Unit: "km", Scale: 0.001, Decimals: 1},
	{AttrID: 192, SectionID: "offense", Label: "Max Locked Targets", Unit: "", Decimals: 0},
	{AttrID: 564, SectionID: "offense", Label: "Scan Resolution", Unit: "mm", Decimals: 0},
	{AttrID: 54, SectionID: "offense", Label: "Optimal Range", Unit: "km", Scale: 0.001, Decimals: 1},
	{AttrID: 158, SectionID: "offense", Label: "Accuracy Falloff", Unit: "km", Scale: 0.001, Decimals: 1},
	{AttrID: 64, SectionID: "offense", Label: "Damage Multiplier", Unit: "×", Decimals: 2},
	{AttrID: 213, SectionID: "offense", Label: "Missile Damage Bonus", Unit: "×", Decimals: 2},
	{AttrID: 20, SectionID: "offense", Label: "Speed Factor Bonus", Unit: "%", Decimals: 1},
	{AttrID: 147, SectionID: "offense", Label: "Velocity Bonus", Unit: "%", Decimals: 1},
	{AttrID: 77, SectionID: "offense", Label: "Mining Yield", Unit: "m³", Decimals: 1},
}

var sectionOrder = []struct {
	ID    string
	Title string
	Color string
}{
	{ID: "fitting", Title: "Fitting & Capacitor", Color: "amber"},
	{ID: "defense", Title: "Defense & Tank", Color: "sky"},
	{ID: "navigation", Title: "Navigation, Cargo & Drones", Color: "emerald"},
	{ID: "offense", Title: "Targeting & Performance", Color: "violet"},
}

// BuildItemCompareResult aligns 1–3 ItemSpecDetails into visual comparison sections with relative bars and best-value highlights.
func BuildItemCompareResult(items []ItemSpecDetail, hub TradeHub) *ItemCompareResult {
	res := &ItemCompareResult{
		Hub:      hub,
		Items:    items,
		Sections: make([]CompareSection, 0, len(sectionOrder)),
	}
	if len(items) == 0 {
		return res
	}

	bySection := make(map[string][]CompareMetricRow, len(sectionOrder))

	for _, def := range compareMetricDefs {
		vals := make([]float64, len(items))
		has := make([]bool, len(items))
		anyVal := false
		maxVal := 0.0
		bestVal := 0.0
		hasBest := false

		for i, it := range items {
			var v float64
			var ok bool
			if def.FieldKey == "capacity" {
				if it.Capacity > 0 {
					v, ok = it.Capacity, true
				}
			} else if def.FieldKey == "packaged_volume" {
				pv := it.PackagedVolume
				if pv <= 0 {
					pv = it.Volume
				}
				if pv > 0 {
					v, ok = pv, true
				}
			} else if def.AttrID > 0 {
				if raw, exists := it.Attributes[def.AttrID]; exists && raw != 0 {
					v = raw
					if def.Scale > 0 {
						v *= def.Scale
					}
					ok = true
				}
			}
			if ok {
				vals[i] = v
				has[i] = true
				anyVal = true
				if math.Abs(v) > maxVal {
					maxVal = math.Abs(v)
				}
				if !hasBest {
					bestVal = v
					hasBest = true
				} else if def.LowerIsBetter && v < bestVal {
					bestVal = v
				} else if !def.LowerIsBetter && v > bestVal {
					bestVal = v
				}
			}
		}

		if !anyVal {
			continue
		}

		// Only highlight "Best" when comparing 2+ items and values actually differ
		allEqual := true
		firstSeen := false
		var firstV float64
		for i := range items {
			if !has[i] {
				continue
			}
			if !firstSeen {
				firstV = vals[i]
				firstSeen = true
			} else if math.Abs(vals[i]-firstV) > 1e-6 {
				allEqual = false
			}
		}

		cells := make([]CompareMetricCell, len(items))
		for i := range items {
			if !has[i] {
				continue
			}
			pct := 0.0
			if maxVal > 0 {
				pct = math.Min(100, math.Max(8, (math.Abs(vals[i])/maxVal)*100.0))
			}
			isBest := len(items) > 1 && !allEqual && math.Abs(vals[i]-bestVal) <= 1e-6
			cells[i] = CompareMetricCell{
				HasValue:   true,
				RawValue:   vals[i],
				Formatted:  formatMetricValue(vals[i], def.Decimals, def.Unit),
				BarPercent: math.Round(pct*10) / 10,
				IsBest:     isBest,
			}
		}

		bySection[def.SectionID] = append(bySection[def.SectionID], CompareMetricRow{
			Key:           def.Label,
			Label:         def.Label,
			LowerIsBetter: def.LowerIsBetter,
			Cells:         cells,
		})
	}

	for _, sec := range sectionOrder {
		if rows := bySection[sec.ID]; len(rows) > 0 {
			res.Sections = append(res.Sections, CompareSection{
				ID:    sec.ID,
				Title: sec.Title,
				Color: sec.Color,
				Rows:  rows,
			})
		}
	}

	return res
}

func formatMetricValue(v float64, decimals int, unit string) string {
	var num string
	if decimals == 0 {
		num = formatWithCommas(math.Round(v))
	} else {
		fmtStr := fmt.Sprintf("%%.%df", decimals)
		num = fmt.Sprintf(fmtStr, v)
		if strings.Contains(num, ".") {
			num = strings.TrimRight(strings.TrimRight(num, "0"), ".")
		}
	}
	if unit == "" {
		return num
	}
	return num + " " + unit
}

func formatWithCommas(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%.0f", v)
	n := len(s)
	if n <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	rem := n % 3
	if rem > 0 {
		b.WriteString(s[:rem])
	}
	for i := rem; i < n; i += 3 {
		if b.Len() > 0 && !(neg && b.Len() == 1) {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

var (
	reEVEBr   = regexp.MustCompile(`(?i)<br\s*/?>`)
	reEVETags = regexp.MustCompile(`<[^>]+>`)
)

// StripEVEHTML converts CCP's EVE item description HTML into clean readable plain text.
func StripEVEHTML(raw string) string {
	if raw == "" {
		return ""
	}
	s := reEVEBr.ReplaceAllString(raw, "\n")
	s = reEVETags.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	lines := strings.Split(s, "\n")
	clean := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			clean = append(clean, l)
		}
	}
	return strings.Join(clean, "\n")
}
