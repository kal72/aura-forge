package domain

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FitSlotKind enumerates EVE Online ship fitting rack categories.
type FitSlotKind string

const (
	SlotHigh      FitSlotKind = "high"
	SlotMid       FitSlotKind = "mid"
	SlotLow       FitSlotKind = "low"
	SlotRig       FitSlotKind = "rig"
	SlotSubsystem FitSlotKind = "subsystem"
)

// ModuleState controls whether a fitted module is active, passive, offline, or overheated.
type ModuleState string

const (
	ModuleActive     ModuleState = "active"
	ModulePassive    ModuleState = "passive"
	ModuleOffline    ModuleState = "offline"
	ModuleOverheated ModuleState = "overheated"
)

// SkillProfilePreset selects pilot skill scaling for deterministic simulation.
type SkillProfilePreset string

const (
	SkillProfileBeginner     SkillProfilePreset = "beginner"     // All Level III
	SkillProfileIntermediate SkillProfilePreset = "intermediate" // All Level IV
	SkillProfileMaxed        SkillProfilePreset = "maxed"        // All Level V
	SkillProfilePilotLive    SkillProfilePreset = "pilot_live"   // Live EVE SSO Pilot Skills
)

// CharacterSkillsProvider is an optional interface implemented by esi.CharacterSyncClient.
type CharacterSkillsProvider interface {
	SyncSkills(ctx context.Context, token *EVEToken) (map[int32]int, int64, error)
}

// MissingSkillGap describes one prerequisite skill that the active skill profile lacks for a ship or module.
type MissingSkillGap struct {
	ItemName      string      `json:"item_name"`
	SlotKind      FitSlotKind `json:"slot_kind,omitempty"` // empty for hull
	SlotIndex     int         `json:"slot_index"`
	SkillTypeID   int32       `json:"skill_type_id"`
	SkillName     string      `json:"skill_name"`
	RequiredLevel int         `json:"required_level"`
	PilotLevel    int         `json:"pilot_level"`
}

// FitModuleSlot represents one module installed in a specific radial slot on the fitting wheel.
type FitModuleSlot struct {
	SlotKind     FitSlotKind `json:"slot_kind"`
	SlotIndex    int         `json:"slot_index"`
	TypeID       int32       `json:"type_id"`
	Name         string      `json:"name"`
	GroupName    string      `json:"group_name,omitempty"`
	TechLevel    int         `json:"tech_level,omitempty"`
	MetaLevel    int         `json:"meta_level,omitempty"`
	ChargeTypeID int32       `json:"charge_type_id,omitempty"`
	ChargeName   string      `json:"charge_name,omitempty"`
	State        ModuleState `json:"state"`
	UnitPriceISK float64     `json:"unit_price_isk,omitempty"`
}

// FitDroneEntry represents a stack of drones carried in the ship's drone bay.
type FitDroneEntry struct {
	TypeID       int32   `json:"type_id"`
	Name         string  `json:"name"`
	Quantity     int     `json:"quantity"`
	Active       bool    `json:"active"`
	UnitPriceISK float64 `json:"unit_price_isk,omitempty"`
}

// ShipFit is the canonical representation of a ship fitting in Aura Tactical.
type ShipFit struct {
	ID           int64              `json:"id,omitempty"`
	Name         string             `json:"name"`
	ShipTypeID   int32              `json:"ship_type_id"`
	ShipName     string             `json:"ship_name"`
	HullClass    string             `json:"hull_class,omitempty"`
	FactionTheme string             `json:"faction_theme,omitempty"` // amarr | caldari | gallente | minmatar | triglavian | neutral
	Tag          string             `json:"tag,omitempty"`           // PvE | PvP | Hauling | Abyssal
	SkillProfile SkillProfilePreset `json:"skill_profile,omitempty"`
	TargetPreset string             `json:"target_preset,omitempty"` // guristas | sansha | blood_raiders | serpentis | angel | triglavian | gank
	Modules      []FitModuleSlot    `json:"modules"`
	Drones       []FitDroneEntry    `json:"drones"`
}

// TacticalFittingRecord represents a persisted fitting row in PostgreSQL.
type TacticalFittingRecord struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	Name             string    `json:"name"`
	ShipTypeID       int32     `json:"ship_type_id"`
	ShipName         string    `json:"ship_name"`
	HullClass        string    `json:"hull_class"`
	Tag              string    `json:"tag"`
	SkillProfile     string    `json:"skill_profile"`
	EFTText          string    `json:"eft_text"`
	Fit              ShipFit   `json:"fit"`
	EstimatedCostISK float64   `json:"estimated_cost_isk"`
	EHP              float64   `json:"ehp"`
	DPS              float64   `json:"dps"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TacticalRepository defines PostgreSQL persistence for Aura Tactical fittings and AI caches.
type TacticalRepository interface {
	ListFittings(ctx context.Context, userID int64, limit int) ([]TacticalFittingRecord, error)
	GetFitting(ctx context.Context, id int64, userID int64) (*TacticalFittingRecord, error)
	SaveFitting(ctx context.Context, rec *TacticalFittingRecord) (*TacticalFittingRecord, error)
	DeleteFitting(ctx context.Context, id int64, userID int64) error
	GetAICache(ctx context.Context, feature, contextHash string) ([]byte, bool, error)
	SaveAICache(ctx context.Context, feature, contextHash, model string, payload []byte, ttl time.Duration) error
}

// ParsedEFT holds the raw names extracted from an EVE Fitting Tool (EFT) clipboard block.
type ParsedEFT struct {
	ShipName string
	FitName  string
	Low      []ParsedEFTLine
	Mid      []ParsedEFTLine
	High     []ParsedEFTLine
	Rig      []ParsedEFTLine
	Drones   []ParsedEFTDrone
}

// ParsedEFTLine represents one module line (with optional ", Charge Name") from EFT format.
type ParsedEFTLine struct {
	ModuleName string
	ChargeName string
	Offline    bool
}

// ParsedEFTDrone represents one drone line (e.g. "Hobgoblin II x5") from EFT format.
type ParsedEFTDrone struct {
	Name     string
	Quantity int
}

// ParseEFT parses an official EVE Online / Pyfa / zKillboard EFT fitting block.
func ParseEFT(raw string) (*ParsedEFT, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := &ParsedEFT{}

	headerIdx := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") && strings.Contains(t, ",") {
			inner := strings.TrimSuffix(strings.TrimPrefix(t, "["), "]")
			parts := strings.SplitN(inner, ",", 2)
			out.ShipName = strings.TrimSpace(parts[0])
			out.FitName = strings.TrimSpace(parts[1])
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 || out.ShipName == "" {
		return nil, fmt.Errorf("invalid EFT format: missing header line like [Caracal, L2 Mission Fit]")
	}
	if out.FitName == "" {
		out.FitName = out.ShipName + " Fit"
	}

	// Standard EFT sections separated by blank lines:
	// 0: Low, 1: Mid, 2: High, 3: Rig, 4+: Subsystems / Drones / Cargo
	section := 0
	prevBlank := true
	for i := headerIdx + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			if !prevBlank {
				section++
				prevBlank = true
			}
			continue
		}
		prevBlank = false

		// Skip empty slot placeholders like "[Empty High slot]"
		if strings.HasPrefix(t, "[Empty") && strings.HasSuffix(t, "]") {
			continue
		}

		// Drone or cargo line with trailing " x<qty>"
		if qtyName, qty, isQty := parseEFTQtyLine(t); isQty {
			if section >= 4 {
				out.Drones = append(out.Drones, ParsedEFTDrone{Name: qtyName, Quantity: qty})
				continue
			}
		}

		offline := false
		if strings.HasSuffix(strings.ToLower(t), "/offline") {
			offline = true
			t = strings.TrimSpace(t[:len(t)-len("/offline")])
		}

		modName := t
		chargeName := ""
		if comma := strings.Index(t, ","); comma > 0 {
			modName = strings.TrimSpace(t[:comma])
			chargeName = strings.TrimSpace(t[comma+1:])
		}
		if modName == "" {
			continue
		}

		entry := ParsedEFTLine{
			ModuleName: modName,
			ChargeName: chargeName,
			Offline:    offline,
		}
		switch section {
		case 0:
			out.Low = append(out.Low, entry)
		case 1:
			out.Mid = append(out.Mid, entry)
		case 2:
			out.High = append(out.High, entry)
		case 3:
			out.Rig = append(out.Rig, entry)
		default:
			// Extra section: treat as drone if it has no comma, else ignore cargo
			if chargeName == "" {
				out.Drones = append(out.Drones, ParsedEFTDrone{Name: modName, Quantity: 1})
			}
		}
	}

	return out, nil
}

func parseEFTQtyLine(line string) (string, int, bool) {
	idx := strings.LastIndex(line, " x")
	if idx <= 0 || idx+2 >= len(line) {
		return "", 0, false
	}
	qty, err := strconv.Atoi(strings.TrimSpace(line[idx+2:]))
	if err != nil || qty <= 0 {
		return "", 0, false
	}
	return strings.TrimSpace(line[:idx]), qty, true
}

// ExportEFT serializes a ShipFit into official EVE Online / Pyfa EFT clipboard format.
func ExportEFT(fit ShipFit) string {
	fitName := strings.TrimSpace(fit.Name)
	if fitName == "" {
		fitName = fit.ShipName + " Tactical Fit"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[%s, %s]\n\n", fit.ShipName, fitName))

	writeSection := func(kind FitSlotKind) {
		var mods []FitModuleSlot
		for _, m := range fit.Modules {
			if m.SlotKind == kind && m.Name != "" {
				mods = append(mods, m)
			}
		}
		sort.Slice(mods, func(i, j int) bool { return mods[i].SlotIndex < mods[j].SlotIndex })
		for _, m := range mods {
			line := m.Name
			if m.ChargeName != "" {
				line += ", " + m.ChargeName
			}
			if m.State == ModuleOffline {
				line += " /offline"
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	writeSection(SlotLow)
	writeSection(SlotMid)
	writeSection(SlotHigh)
	writeSection(SlotRig)

	if len(fit.Drones) > 0 {
		b.WriteString("\n")
		for _, d := range fit.Drones {
			if d.Name != "" && d.Quantity > 0 {
				b.WriteString(fmt.Sprintf("%s x%d\n", d.Name, d.Quantity))
			}
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

// FormatMultibuy generates an EVE Online Multibuy clipboard list (Hull + all fitted modules + drones).
func FormatMultibuy(fit ShipFit) string {
	counts := make(map[string]int)
	order := make([]string, 0, len(fit.Modules)+len(fit.Drones)+1)
	add := func(name string, qty int) {
		name = strings.TrimSpace(name)
		if name == "" || qty <= 0 {
			return
		}
		if _, ok := counts[name]; !ok {
			order = append(order, name)
		}
		counts[name] += qty
	}

	add(fit.ShipName, 1)
	for _, m := range fit.Modules {
		add(m.Name, 1)
		if m.ChargeName != "" {
			add(m.ChargeName, 1000)
		}
	}
	for _, d := range fit.Drones {
		add(d.Name, d.Quantity)
	}

	var b strings.Builder
	for _, name := range order {
		b.WriteString(fmt.Sprintf("%s x%d\n", name, counts[name]))
	}
	return strings.TrimSpace(b.String())
}

// InferFactionTheme maps an EVE ship name or group into a visual cockpit hologram theme.
func InferFactionTheme(shipName, description string) string {
	low := strings.ToLower(shipName + " " + description)
	switch {
	case strings.Contains(low, "triglavian") || strings.Contains(low, "vedmak") || strings.Contains(low, "lesha") || strings.Contains(low, "kikimora") || strings.Contains(low, "damavik"):
		return "triglavian"
	case strings.Contains(low, "amarr") || strings.Contains(low, "imperial") || strings.Contains(low, "punisher") || strings.Contains(low, "omen") || strings.Contains(low, "maller") || strings.Contains(low, "harbinger") || strings.Contains(low, "apocalypse") || strings.Contains(low, "abaddon") || strings.Contains(low, "coercer"):
		return "amarr"
	case strings.Contains(low, "caldari") || strings.Contains(low, "caracal") || strings.Contains(low, "drake") || strings.Contains(low, "raven") || strings.Contains(low, "gila") || strings.Contains(low, "heron") || strings.Contains(low, "kestrel") || strings.Contains(low, "moa") || strings.Contains(low, "cormorant") || strings.Contains(low, "tayra") || strings.Contains(low, "badger"):
		return "caldari"
	case strings.Contains(low, "gallente") || strings.Contains(low, "vexor") || strings.Contains(low, "ishtar") || strings.Contains(low, "tristan") || strings.Contains(low, "catalyst") || strings.Contains(low, "algos") || strings.Contains(low, "myrmidon") || strings.Contains(low, "dominix") || strings.Contains(low, "nereus") || strings.Contains(low, "iteron"):
		return "gallente"
	case strings.Contains(low, "minmatar") || strings.Contains(low, "rifter") || strings.Contains(low, "thrasher") || strings.Contains(low, "stabber") || strings.Contains(low, "rupture") || strings.Contains(low, "hurricane") || strings.Contains(low, "maelstrom") || strings.Contains(low, "tornado") || strings.Contains(low, "wreathe"):
		return "minmatar"
	default:
		return "neutral"
	}
}
