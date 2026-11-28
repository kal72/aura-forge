package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// StackingPenalty returns the official EVE Online Dogma stacking effectiveness S(n) = exp(-(n/2.67)^2)
// where n is the 0-based rank (0 = strongest modifier = 1.000, 1 = 0.869, 2 = 0.571, 3 = 0.283).
func StackingPenalty(rank int) float64 {
	if rank <= 0 {
		return 1.0
	}
	x := float64(rank) / 2.67
	return math.Exp(-(x * x))
}

// DamageProfile represents a 4-element damage share or DPS split (EM, Thermal, Kinetic, Explosive).
type DamageProfile struct {
	EM        float64 `json:"em"`
	Thermal   float64 `json:"thermal"`
	Kinetic   float64 `json:"kinetic"`
	Explosive float64 `json:"explosive"`
}

// PvEFactionPreset defines an EVE Online pirate faction or combat scenario for readiness checks.
type PvEFactionPreset struct {
	Key            string        `json:"key"`
	Name           string        `json:"name"`
	ShortLabel     string        `json:"short_label"`
	Color          string        `json:"color"` // sky | amber | rose | emerald | violet | orange
	IncomingDamage DamageProfile `json:"incoming_damage"`
	WeakAgainst    []string      `json:"weak_against"`
	EWARWarning    string        `json:"ewar_warning,omitempty"`
	BenchmarkDPS   float64       `json:"benchmark_dps"` // Typical L3 Mission / Anomaly incoming DPS
	RecommendedTip string        `json:"recommended_tip"`
}

// BuiltInPvEFactions returns the standard EVE Online PvE pirate & anti-gank target profiles.
func BuiltInPvEFactions() []PvEFactionPreset {
	return []PvEFactionPreset{
		{
			Key:            "guristas",
			Name:           "Guristas Pirates",
			ShortLabel:     "Guristas",
			Color:          "sky",
			IncomingDamage: DamageProfile{EM: 0.05, Thermal: 0.25, Kinetic: 0.65, Explosive: 0.05},
			WeakAgainst:    []string{"Kinetic", "Thermal"},
			EWARWarning:    "ECM Jamming (Target Lock Breaks)",
			BenchmarkDPS:   180,
			RecommendedTip: "Stack Kinetic & Thermal hardeners; bring drones or Kinetic missiles.",
		},
		{
			Key:            "sansha",
			Name:           "Sansha's Nation",
			ShortLabel:     "Sansha",
			Color:          "amber",
			IncomingDamage: DamageProfile{EM: 0.55, Thermal: 0.45, Kinetic: 0.00, Explosive: 0.00},
			WeakAgainst:    []string{"Electromagnetic", "Thermal"},
			EWARWarning:    "Tracking Disruption",
			BenchmarkDPS:   210,
			RecommendedTip: "Plug your Shield EM hole immediately — Sansha lasers punish 0% EM resist.",
		},
		{
			Key:            "blood_raiders",
			Name:           "Blood Raiders",
			ShortLabel:     "Blood Raiders",
			Color:          "rose",
			IncomingDamage: DamageProfile{EM: 0.50, Thermal: 0.50, Kinetic: 0.00, Explosive: 0.00},
			WeakAgainst:    []string{"Electromagnetic", "Thermal"},
			EWARWarning:    "Capacitor Neutralizers & Webifiers",
			BenchmarkDPS:   195,
			RecommendedTip: "Maintain high Cap Stability (>55%) or passive tank; prioritize neuting frigates first.",
		},
		{
			Key:            "serpentis",
			Name:           "Serpentis Corporation",
			ShortLabel:     "Serpentis",
			Color:          "emerald",
			IncomingDamage: DamageProfile{EM: 0.00, Thermal: 0.55, Kinetic: 0.45, Explosive: 0.00},
			WeakAgainst:    []string{"Kinetic", "Thermal"},
			EWARWarning:    "Sensor Dampeners & Stasis Webifiers",
			BenchmarkDPS:   190,
			RecommendedTip: "Fit Thermal/Kinetic resists and watch lock range reduction from dampeners.",
		},
		{
			Key:            "angel",
			Name:           "Angel Cartel",
			ShortLabel:     "Angel Cartel",
			Color:          "orange",
			IncomingDamage: DamageProfile{EM: 0.05, Thermal: 0.10, Kinetic: 0.25, Explosive: 0.60},
			WeakAgainst:    []string{"Explosive", "Kinetic"},
			EWARWarning:    "Target Painters (Signature Bloom)",
			BenchmarkDPS:   220,
			RecommendedTip: "Plug Armor Explosive hole; fast Angel frigates close range rapidly.",
		},
		{
			Key:            "triglavian",
			Name:           "Triglavian / Abyssal",
			ShortLabel:     "Triglavian",
			Color:          "violet",
			IncomingDamage: DamageProfile{EM: 0.10, Thermal: 0.50, Kinetic: 0.10, Explosive: 0.30},
			WeakAgainst:    []string{"Explosive", "Thermal"},
			EWARWarning:    "Ramping Disintegrator DPS & Neutralizers",
			BenchmarkDPS:   280,
			RecommendedTip: "Kill ramping Damavik/Vedmak hulls early before their damage multiplier peaks.",
		},
	}
}

// LookupPvEFaction returns the matching PvE preset or Guristas by default.
func LookupPvEFaction(key string) PvEFactionPreset {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, f := range BuiltInPvEFactions() {
		if f.Key == k {
			return f
		}
	}
	return BuiltInPvEFactions()[0]
}

// ResourceGauge holds usage, total capacity, percentage, and overload status for CPU/PG/Calibration.
type ResourceGauge struct {
	Used       float64 `json:"used"`
	Total      float64 `json:"total"`
	Percent    float64 `json:"percent"`
	Remaining  float64 `json:"remaining"`
	Overloaded bool    `json:"overloaded"`
}

// DefenseLayerStats holds Raw HP, Effective HP (against chosen damage profile), and 4 resistances.
type DefenseLayerStats struct {
	RawHP      float64           `json:"raw_hp"`
	EHP        float64           `json:"ehp"`
	Resists    ResistanceProfile `json:"resists"`
	LowestType string            `json:"lowest_type"`
	HasHole    bool              `json:"has_hole"`
}

// CapacitorStats holds EVE Capacitor stability metrics.
type CapacitorStats struct {
	CapacityGJ      float64 `json:"capacity_gj"`
	RechargeSeconds float64 `json:"recharge_seconds"`
	PeakRechargeGJs float64 `json:"peak_recharge_gjs"`
	ActiveDrainGJs  float64 `json:"active_drain_gjs"`
	DeltaGJs        float64 `json:"delta_gjs"`
	Stable          bool    `json:"stable"`
	StablePercent   float64 `json:"stable_percent"`
	TimeToEmptySec  float64 `json:"time_to_empty_sec"`
	FormattedTime   string  `json:"formatted_time"`
}

// OffenseStats holds weapon + drone DPS, volley, and damage type breakdown.
type OffenseStats struct {
	TurretDPS      float64       `json:"turret_dps"`
	MissileDPS     float64       `json:"missile_dps"`
	DroneDPS       float64       `json:"drone_dps"`
	TotalDPS       float64       `json:"total_dps"`
	AlphaVolley    float64       `json:"alpha_volley"`
	OptimalRangeKM float64       `json:"optimal_range_km"`
	FalloffRangeKM float64       `json:"falloff_range_km"`
	DamageMix      DamageProfile `json:"damage_mix"`
}

// NavigationStats holds propulsion, agility, warp, and cargo metrics.
type NavigationStats struct {
	MaxVelocityMS    float64 `json:"max_velocity_ms"`
	WarpSpeedAUS     float64 `json:"warp_speed_aus"`
	InertiaModifier  float64 `json:"inertia_modifier"`
	MassKG           float64 `json:"mass_kg"`
	AlignTimeSeconds float64 `json:"align_time_seconds"`
	AlignRating      string  `json:"align_rating"` // INSTA-WARP | AGILE | STANDARD | HEAVY
	SignatureRadiusM float64 `json:"signature_radius_m"`
	CargoCapacityM3  float64 `json:"cargo_capacity_m3"`
	DroneBandwidth   float64 `json:"drone_bandwidth"`
	DroneBandwidthUsed float64 `json:"drone_bandwidth_used"`
	DroneBayM3       float64 `json:"drone_bay_m3"`
	DroneBayUsedM3   float64 `json:"drone_bay_used_m3"`
}

// ReadinessFinding represents one beginner-friendly diagnostic warning or pass check.
type ReadinessFinding struct {
	Severity string `json:"severity"` // critical | warn | ok
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

// RangeCurvePoint represents Applied DPS at a specific distance (km) for visual charting.
type RangeCurvePoint struct {
	DistanceKM float64 `json:"distance_km"`
	AppliedDPS float64 `json:"applied_dps"`
}

// FitSimulationResult is the complete deterministic calculation output rendered in the Tactical HUD.
type FitSimulationResult struct {
	Fit               ShipFit            `json:"fit"`
	Slots             SlotLayout         `json:"slots"`
	Traits            *ItemTraits        `json:"traits,omitempty"`
	CPU               ResourceGauge      `json:"cpu"`
	Powergrid         ResourceGauge      `json:"powergrid"`
	Calibration       ResourceGauge      `json:"calibration"`
	TurretsUsed       int                `json:"turrets_used"`
	LaunchersUsed     int                `json:"launchers_used"`
	Shield            DefenseLayerStats  `json:"shield"`
	Armor             DefenseLayerStats  `json:"armor"`
	Structure         DefenseLayerStats  `json:"structure"`
	TotalRawHP        float64            `json:"total_raw_hp"`
	TotalEHP          float64            `json:"total_ehp"`
	SustainedTankEHPs float64            `json:"sustained_tank_ehps"`
	Capacitor         CapacitorStats     `json:"capacitor"`
	Offense           OffenseStats       `json:"offense"`
	Navigation        NavigationStats    `json:"navigation"`
	TargetFaction     PvEFactionPreset   `json:"target_faction"`
	ReadinessGrade    string             `json:"readiness_grade"` // S | A | B | C | RISK
	Findings          []ReadinessFinding `json:"findings"`
	MissingSkills     []MissingSkillGap  `json:"missing_skills,omitempty"`
	LockedSlotKeys    []string           `json:"locked_slot_keys,omitempty"` // e.g. "high-0", "mid-2"
	PilotSkillsActive bool               `json:"pilot_skills_active"`
	PilotTotalSP      int64              `json:"pilot_total_sp,omitempty"`
	RangeCurve        []RangeCurvePoint  `json:"range_curve"`
	TotalFitCostISK   float64            `json:"total_fit_cost_isk"`
	HullCostISK       float64            `json:"hull_cost_isk"`
	ModulesCostISK    float64            `json:"modules_cost_isk"`
	HubName           string             `json:"hub_name"`
	EFTExport         string             `json:"eft_export"`
	MultibuyText      string             `json:"multibuy_text"`
}

// ResolveSkillProfileLevel maps a SkillProfilePreset to its effective scaling level (0..5)
// and prerequisite gap evaluation cap (0..5).
func ResolveSkillProfileLevel(profile SkillProfilePreset) (float64, int) {
	switch profile {
	case SkillProfileAll0:
		return 0.0, 0
	case SkillProfileAll1:
		return 1.0, 1
	case SkillProfileAll2:
		return 2.0, 2
	case SkillProfileAll3, SkillProfileBeginner:
		return 3.0, 3
	case SkillProfileAll4, SkillProfileIntermediate:
		return 4.0, 4
	case SkillProfileAll5, SkillProfileMaxed:
		return 5.0, 5
	case SkillProfileMaxAlpha:
		// Max Alpha clone: core fitting & combat skills capped at Alpha limits (Level IV cap for prerequisites)
		return 4.0, 4
	default:
		return 4.0, 4
	}
}

// ComputePilotCoreSkillLevel averages the pilot's real EVE SSO core fitting/combat skills
// (CPU Management, Power Grid Management, Capacitor Management/Ops, Shield/Armor/Hull, Navigation).
func ComputePilotCoreSkillLevel(liveSkills map[int32]int) float64 {
	if len(liveSkills) == 0 {
		return 4.0
	}
	coreSkillIDs := []int32{
		3426, // CPU Management
		3413, // Power Grid Management
		3418, // Capacitor Management
		3417, // Capacitor Systems Operation
		3419, // Shield Management
		3394, // Hull Upgrades
		3392, // Mechanics
		3449, // Navigation
		3453, // Evasive Maneuvering
	}
	sum := 0.0
	for _, id := range coreSkillIDs {
		sum += float64(liveSkills[id])
	}
	return sum / float64(len(coreSkillIDs))
}

// ComputeFitSimulation performs full deterministic EVE Dogma simulation for a ShipFit
// given the ship's ItemSpecDetail and a lookup map of fitted module/drone ItemSpecDetails.
func ComputeFitSimulation(
	fit ShipFit,
	shipSpec *ItemSpecDetail,
	itemSpecs map[int32]*ItemSpecDetail,
	hubName string,
) *FitSimulationResult {
	return ComputeFitSimulationWithSkills(fit, shipSpec, itemSpecs, hubName, nil)
}

// ComputeFitSimulationWithSkills performs deterministic EVE Dogma simulation with an optional
// live EVE SSO character skill map.
func ComputeFitSimulationWithSkills(
	fit ShipFit,
	shipSpec *ItemSpecDetail,
	itemSpecs map[int32]*ItemSpecDetail,
	hubName string,
	liveSkills map[int32]int,
) *FitSimulationResult {
	if shipSpec == nil {
		return nil
	}
	if fit.SkillProfile == "" {
		fit.SkillProfile = SkillProfileAll4
	}
	if fit.FactionTheme == "" {
		fit.FactionTheme = InferFactionTheme(shipSpec.Name, shipSpec.Description)
	}
	faction := LookupPvEFaction(fit.TargetPreset)
	fit.TargetPreset = faction.Key

	skillLevel, gapCapLevel := ResolveSkillProfileLevel(fit.SkillProfile)
	if fit.SkillProfile == SkillProfilePilotLive && len(liveSkills) > 0 {
		skillLevel = ComputePilotCoreSkillLevel(liveSkills)
	}

	// Base ship attributes with pilot skill scaling (CPU Management +5%/lvl, Power Grid Management +5%/lvl,
	// Hull/Shield/Armor Management +5%/lvl, Capacitor Management +5%/lvl, Capacitor Systems Operation -5%/lvl)
	attrs := shipSpec.Attributes
	baseCPU := attrs[48] * (1.0 + 0.05*skillLevel)
	basePG := attrs[11] * (1.0 + 0.05*skillLevel)
	baseCalib := attrs[1132]
	if baseCalib <= 0 && shipSpec.Slots.RigSlots > 0 {
		baseCalib = 400
	}

	shieldHP := attrs[263] * (1.0 + 0.05*skillLevel)
	armorHP := attrs[265] * (1.0 + 0.05*skillLevel)
	structHP := attrs[9] * (1.0 + 0.05*skillLevel)
	if structHP <= 0 {
		structHP = 500
	}

	capCapacity := attrs[482] * (1.0 + 0.05*skillLevel)
	capRechargeMs := attrs[55] * (1.0 - 0.05*skillLevel)
	if capRechargeMs <= 0 {
		capRechargeMs = 150000
	}

	maxVelocity := attrs[37] * (1.0 + 0.05*skillLevel)
	warpSpeed := attrs[600]
	if warpSpeed <= 0 {
		warpSpeed = 3.0
	}
	inertia := attrs[70] * (1.0 - 0.05*skillLevel)
	if inertia <= 0 {
		inertia = 1.0
	}
	mass := shipSpec.Mass
	if mass <= 0 {
		mass = 1_000_000
	}
	sigRadius := attrs[552]
	if sigRadius <= 0 {
		sigRadius = 40
	}
	cargoCap := shipSpec.Capacity
	droneBay := attrs[283]
	droneBW := attrs[1271]

	// Base resonances (1.0 - resistPct/100)
	shieldRes := [4]float64{
		1.0 - shipSpec.ShieldResists.EM/100.0,
		1.0 - shipSpec.ShieldResists.Thermal/100.0,
		1.0 - shipSpec.ShieldResists.Kinetic/100.0,
		1.0 - shipSpec.ShieldResists.Explosive/100.0,
	}
	armorRes := [4]float64{
		1.0 - shipSpec.ArmorResists.EM/100.0,
		1.0 - shipSpec.ArmorResists.Thermal/100.0,
		1.0 - shipSpec.ArmorResists.Kinetic/100.0,
		1.0 - shipSpec.ArmorResists.Explosive/100.0,
	}
	structRes := [4]float64{0.67, 0.67, 0.67, 0.67} // Standard 33% hull resistance in EVE

	// Accumulators for stacking-penalized bonuses
	var (
		shieldResistMods [4][]float64 // negative percentages like -30 -> 0.70 resonance multiplier
		armorResistMods  [4][]float64
		damageMods       []float64 // percentage bonus e.g. 0.10
		rofMods          []float64 // percentage bonus e.g. 0.10
		speedMods        []float64
		inertiaMods      []float64
	)

	var (
		cpuUsed, pgUsed, calibUsed float64
		turretsUsed, launchersUsed int
		activeCapDrain             float64
		shieldBoostHPs             float64
		armorRepHPs                float64
		turretDPS, missileDPS      float64
		alphaVolley                float64
		bestOptimalKM              float64
		bestFalloffKM              float64
		dmgEM, dmgTh, dmgKi, dmgEx float64
		modulesCostISK             float64
		cpuBonusPct, pgBonusPct    float64
		dcApplied                  bool
	)

	for _, mod := range fit.Modules {
		if mod.TypeID <= 0 {
			continue
		}
		spec := itemSpecs[mod.TypeID]
		if spec != nil && spec.LiveSellPrice > 0 {
			modulesCostISK += spec.LiveSellPrice
		} else if mod.UnitPriceISK > 0 {
			modulesCostISK += mod.UnitPriceISK
		}
		if mod.State == ModuleOffline || spec == nil {
			continue
		}

		ma := spec.Attributes
		cpuUsed += ma[50]
		pgUsed += ma[30]
		if mod.SlotKind == SlotRig {
			calibUsed += ma[1153] // upgradeCost
		}

		// Overheat bonus (+15% performance when overheated)
		heatMult := 1.0
		if mod.State == ModuleOverheated {
			heatMult = 1.15
		}

		// Capacitor drain for active modules
		if (mod.State == ModuleActive || mod.State == ModuleOverheated) && ma[6] > 0 && ma[73] > 0 {
			cycleSec := ma[73] / 1000.0
			if cycleSec > 0 {
				activeCapDrain += ma[6] / cycleSec
			}
		}

		// Flat HP bonuses (Shield Extenders attr 72, Armor Plates attr 1159)
		if ma[72] > 0 {
			shieldHP += ma[72]
		}
		if ma[1159] > 0 {
			armorHP += ma[1159]
		}
		// Percentage HP bonuses (e.g. Shield/Armor Rigs)
		if ma[150] > 0 {
			shieldHP *= (1.0 + ma[150]/100.0)
		}
		if ma[335] > 0 {
			armorHP *= (1.0 + ma[335]/100.0)
		}
		// CPU / Powergrid output mods (Co-Processor attr 202, Reactor Control Unit attr 145)
		if ma[202] > 0 {
			cpuBonusPct += ma[202]
		}
		if ma[145] > 0 {
			pgBonusPct += ma[145]
		}

		// Signature radius & mass penalties (Shield Extenders / MWD / Plates)
		if ma[554] > 0 {
			sigRadius += ma[554]
		}
		if ma[796] > 0 {
			mass += ma[796]
		}

		// Propulsion (Afterburner / Microwarpdrive: attr 20 speedFactor, attr 567 thrust)
		if (mod.State == ModuleActive || mod.State == ModuleOverheated) && ma[20] > 0 {
			speedMods = append(speedMods, (ma[20]/100.0)*heatMult)
		}
		// Agility mods (Inertial Stabilizers / Nanofiber: attr 169 agilityMultiplier negative %)
		if ma[169] < 0 {
			inertiaMods = append(inertiaMods, math.Abs(ma[169])/100.0)
		}

		// Damage Control special check (unpenalized hull/shield/armor resist boost)
		lowerName := strings.ToLower(spec.Name)
		if strings.Contains(lowerName, "damage control") && !dcApplied {
			dcApplied = true
			// Standard Damage Control II values if specific attrs absent
			sB := math.Abs(ma[984])
			if sB == 0 {
				sB = 12.5
			}
			aB := math.Abs(ma[985])
			if aB == 0 {
				aB = 15.0
			}
			hB := math.Abs(ma[986])
			if hB == 0 {
				hB = 40.0
			}
			for i := 0; i < 4; i++ {
				shieldRes[i] *= (1.0 - sB/100.0)
				armorRes[i] *= (1.0 - aB/100.0)
				structRes[i] *= (1.0 - hB/100.0)
			}
		} else {
			// Standard Shield & Armor Resistance Hardeners / Amplifiers / Rigs
			// EVE stores resist bonuses as negative percentages (e.g. -30 = +30% resist)
			sAttrs := [4]int32{984, 987, 986, 985}
			if ma[984] < 0 || ma[985] < 0 || ma[986] < 0 || ma[987] < 0 {
				// Generic resist attributes
				for i, aid := range sAttrs {
					if ma[aid] < 0 {
						shieldResistMods[i] = append(shieldResistMods[i], math.Abs(ma[aid])/100.0*heatMult)
					}
				}
			}
			// Explicit Shield Resist Bonus attrs: EM 984 / 455, Th 458, Ki 457, Ex 456
			// Explicit Shield/Armor Hardener attrs: EM 271/267 or bonus 984..991
			if v := firstNegAbs(ma, 455, 988); v > 0 {
				shieldResistMods[0] = append(shieldResistMods[0], v/100.0*heatMult)
			}
			if v := firstNegAbs(ma, 458, 991); v > 0 {
				shieldResistMods[1] = append(shieldResistMods[1], v/100.0*heatMult)
			}
			if v := firstNegAbs(ma, 457, 990); v > 0 {
				shieldResistMods[2] = append(shieldResistMods[2], v/100.0*heatMult)
			}
			if v := firstNegAbs(ma, 456, 989); v > 0 {
				shieldResistMods[3] = append(shieldResistMods[3], v/100.0*heatMult)
			}
			// Armor Resist Bonus attrs: EM 992/450, Th 995/453, Ki 994/452, Ex 993/451
			if v := firstNegAbs(ma, 992, 450); v > 0 {
				armorResistMods[0] = append(armorResistMods[0], v/100.0*heatMult)
			}
			if v := firstNegAbs(ma, 995, 453); v > 0 {
				armorResistMods[1] = append(armorResistMods[1], v/100.0*heatMult)
			}
			if v := firstNegAbs(ma, 994, 452); v > 0 {
				armorResistMods[2] = append(armorResistMods[2], v/100.0*heatMult)
			}
			if v := firstNegAbs(ma, 993, 451); v > 0 {
				armorResistMods[3] = append(armorResistMods[3], v/100.0*heatMult)
			}
			// Multispectrum Shield Hardener / Energized Adaptive Nano Membrane fallback by name
			if strings.Contains(lowerName, "multispectrum shield hardener") {
				b := 0.25 * heatMult
				if spec.TechLevel >= 2 {
					b = 0.30 * heatMult
				}
				for i := 0; i < 4; i++ {
					shieldResistMods[i] = append(shieldResistMods[i], b)
				}
			} else if strings.Contains(lowerName, "multispectrum energized membrane") || strings.Contains(lowerName, "energized adaptive nano") {
				b := 0.16
				if spec.TechLevel >= 2 {
					b = 0.20
				}
				for i := 0; i < 4; i++ {
					armorResistMods[i] = append(armorResistMods[i], b)
				}
			}
		}

		// Active Shield Booster (attr 68) & Armor Repairer (attr 84)
		if (mod.State == ModuleActive || mod.State == ModuleOverheated) && ma[68] > 0 && ma[73] > 0 {
			cycleSec := ma[73] / 1000.0
			shieldBoostHPs += (ma[68] * heatMult) / cycleSec
		}
		if (mod.State == ModuleActive || mod.State == ModuleOverheated) && ma[84] > 0 && ma[73] > 0 {
			cycleSec := ma[73] / 1000.0
			armorRepHPs += (ma[84] * heatMult) / cycleSec
		}

		// Damage Amplifiers (Gyrostabilizer, Heat Sink, MagStab, Ballistic Control System)
		if ma[64] > 1.0 && mod.SlotKind == SlotLow {
			damageMods = append(damageMods, ma[64]-1.0)
		}
		if ma[213] > 1.0 && mod.SlotKind == SlotLow {
			damageMods = append(damageMods, ma[213]-1.0)
		}
		if ma[204] > 0 && ma[204] < 1.0 && mod.SlotKind == SlotLow {
			rofMods = append(rofMods, 1.0-ma[204])
		}

		// High-slot weapons (Turrets vs Launchers)
		if mod.SlotKind == SlotHigh {
			isLauncher := strings.Contains(lowerName, "launcher") || strings.Contains(strings.ToLower(spec.GroupName), "launcher")
			isTurret := ma[64] > 0 && !isLauncher && (strings.Contains(lowerName, "blaster") || strings.Contains(lowerName, "railgun") ||
				strings.Contains(lowerName, "laser") || strings.Contains(lowerName, "beam") || strings.Contains(lowerName, "pulse") ||
				strings.Contains(lowerName, "autocannon") || strings.Contains(lowerName, "artillery") || ma[54] > 0)

			if isLauncher {
				launchersUsed++
				cycleSec := ma[51] / 1000.0 // speed / ROF ms
				if cycleSec <= 0 {
					cycleSec = 5.0
				}
				baseVolley := 85.0
				if spec.TechLevel >= 2 {
					baseVolley = 105.0
				}
				if mod.ChargeTypeID > 0 {
					if cs := itemSpecs[mod.ChargeTypeID]; cs != nil {
						cv := cs.Attributes[114] + cs.Attributes[116] + cs.Attributes[117] + cs.Attributes[118]
						if cv > 0 {
							baseVolley = cv
							dmgEM += cs.Attributes[114] / cycleSec
							dmgEx += cs.Attributes[116] / cycleSec
							dmgKi += cs.Attributes[117] / cycleSec
							dmgTh += cs.Attributes[118] / cycleSec
						}
					}
				} else {
					dmgKi += (baseVolley * 0.65) / cycleSec
					dmgTh += (baseVolley * 0.35) / cycleSec
				}
				volley := baseVolley * (1.0 + 0.05*skillLevel) * heatMult
				alphaVolley += volley
				missileDPS += volley / cycleSec
				if bestOptimalKM < 28 {
					bestOptimalKM = 28
				}
			} else if isTurret {
				turretsUsed++
				cycleSec := ma[51] / 1000.0
				if cycleSec <= 0 {
					cycleSec = 4.0
				}
				dmgMult := ma[64]
				if dmgMult <= 0 {
					dmgMult = 2.0
				}
				baseCharge := 18.0
				if mod.ChargeTypeID > 0 {
					if cs := itemSpecs[mod.ChargeTypeID]; cs != nil {
						cv := cs.Attributes[114] + cs.Attributes[116] + cs.Attributes[117] + cs.Attributes[118]
						if cv > 0 {
							baseCharge = cv
							dmgEM += (cs.Attributes[114] * dmgMult) / cycleSec
							dmgEx += (cs.Attributes[116] * dmgMult) / cycleSec
							dmgKi += (cs.Attributes[117] * dmgMult) / cycleSec
							dmgTh += (cs.Attributes[118] * dmgMult) / cycleSec
						}
					}
				} else {
					dmgTh += (baseCharge * dmgMult * 0.55) / cycleSec
					dmgKi += (baseCharge * dmgMult * 0.45) / cycleSec
				}
				volley := baseCharge * dmgMult * (1.0 + 0.05*skillLevel) * heatMult
				alphaVolley += volley
				turretDPS += volley / cycleSec
				optKM := ma[54] / 1000.0
				fallKM := ma[158] / 1000.0
				if optKM > bestOptimalKM {
					bestOptimalKM = optKM
				}
				if fallKM > bestFalloffKM {
					bestFalloffKM = fallKM
				}
			}
		}
	}

	// Apply CPU / PG percentage bonuses + Pilot Implants / Boosters
	hasImplant := func(key string) bool {
		for _, imp := range fit.Implants {
			if strings.EqualFold(strings.TrimSpace(imp), key) {
				return true
			}
		}
		return false
	}
	if hasImplant("cpu_3") {
		cpuBonusPct += 3.0
	}
	if hasImplant("pg_3") {
		pgBonusPct += 3.0
	}
	if hasImplant("cap_5") {
		capCapacity *= 1.05
	}
	if hasImplant("blue_pill") {
		shieldBoostHPs *= 1.20
	}
	if hasImplant("exile") {
		armorRepHPs *= 1.20
	}

	totalCPU := baseCPU * (1.0 + cpuBonusPct/100.0)
	totalPG := basePG * (1.0 + pgBonusPct/100.0)

	// Apply official EVE Stacking Penalty to resistances
	for i := 0; i < 4; i++ {
		shieldRes[i] = applyStackedResistances(shieldRes[i], shieldResistMods[i])
		armorRes[i] = applyStackedResistances(armorRes[i], armorResistMods[i])
	}

	// Apply official Abyssal Deadspace Weather environmental modifiers
	switch strings.ToLower(strings.TrimSpace(fit.AbyssalWeather)) {
	case "electrical":
		// -50% EM Resistance, +50% Capacitor Recharge Speed
		shieldRes[0] = clampResonance(shieldRes[0] * 1.50)
		armorRes[0] = clampResonance(armorRes[0] * 1.50)
		capRechargeMs *= 0.50
	case "exotic":
		// -50% Kinetic Resistance
		shieldRes[2] = clampResonance(shieldRes[2] * 1.50)
		armorRes[2] = clampResonance(armorRes[2] * 1.50)
	case "firestorm":
		// -50% Thermal Resistance, +50% Armor Hit Points
		shieldRes[1] = clampResonance(shieldRes[1] * 1.50)
		armorRes[1] = clampResonance(armorRes[1] * 1.50)
		armorHP *= 1.50
	case "gamma":
		// -50% Explosive Resistance, +50% Shield Hit Points
		shieldRes[3] = clampResonance(shieldRes[3] * 1.50)
		armorRes[3] = clampResonance(armorRes[3] * 1.50)
		shieldHP *= 1.50
	case "dark":
		// -50% Turret Range, +50% Maximum Velocity
		bestOptimalKM *= 0.50
		bestFalloffKM *= 0.50
		maxVelocity *= 1.50
	}

	// Apply Stacking Penalty to weapon damage & ROF mods
	dmgAmp := applyStackedMultipliers(damageMods)
	rofAmp := applyStackedMultipliers(rofMods)
	if hasImplant("dmg_3") {
		dmgAmp *= 1.03
	}
	turretDPS *= dmgAmp * rofAmp
	missileDPS *= dmgAmp * rofAmp
	alphaVolley *= dmgAmp

	// Apply Stacking Penalty to speed & inertia
	maxVelocity *= applyStackedMultipliers(speedMods)
	if len(inertiaMods) > 0 {
		sort.Slice(inertiaMods, func(i, j int) bool { return inertiaMods[i] > inertiaMods[j] })
		for idx, v := range inertiaMods {
			inertia *= (1.0 - v*StackingPenalty(idx))
		}
	}

	// Drones calculation
	var droneDPS, droneBWUsed, droneBayUsed float64
	for _, d := range fit.Drones {
		if d.Quantity <= 0 {
			continue
		}
		sp := itemSpecs[d.TypeID]
		unitVol := 5.0
		unitBW := 5.0
		unitDPS := 16.0
		if sp != nil {
			if sp.LiveSellPrice > 0 {
				modulesCostISK += sp.LiveSellPrice * float64(d.Quantity)
			}
			if sp.Volume > 0 {
				unitVol = sp.Volume
			}
			if sp.Attributes[1272] > 0 {
				unitBW = sp.Attributes[1272]
			}
			dmgSum := sp.Attributes[114] + sp.Attributes[116] + sp.Attributes[117] + sp.Attributes[118]
			dmgMult := sp.Attributes[64]
			if dmgMult <= 0 {
				dmgMult = 1.0
			}
			cyc := sp.Attributes[51] / 1000.0
			if dmgSum > 0 && cyc > 0 {
				unitDPS = (dmgSum * dmgMult) / cyc
			} else if unitBW >= 25 {
				unitDPS = 52.0
			} else if unitBW >= 10 {
				unitDPS = 28.0
			}
		}
		droneBayUsed += unitVol * float64(d.Quantity)
		if d.Active {
			activeCount := d.Quantity
			if droneBW > 0 && unitBW > 0 {
				maxByBW := int(math.Floor((droneBW - droneBWUsed) / unitBW))
				if activeCount > maxByBW {
					activeCount = maxByBW
				}
			}
			if activeCount > 5 {
				activeCount = 5
			}
			if activeCount > 0 {
				droneBWUsed += unitBW * float64(activeCount)
				dTotal := unitDPS * float64(activeCount) * (1.0 + 0.05*skillLevel)
				droneDPS += dTotal
				dmgTh += dTotal * 0.60
				dmgKi += dTotal * 0.40
			}
		}
	}

	// Build Defense Layers & EHP against selected PvE faction incoming damage
	shieldLayer := buildDefenseLayer(shieldHP, shieldRes, faction.IncomingDamage)
	armorLayer := buildDefenseLayer(armorHP, armorRes, faction.IncomingDamage)
	structLayer := buildDefenseLayer(structHP, structRes, faction.IncomingDamage)

	totalRawHP := shieldLayer.RawHP + armorLayer.RawHP + structLayer.RawHP
	totalEHP := shieldLayer.EHP + armorLayer.EHP + structLayer.EHP

	// Effective Sustained Active Tank (EHP/s)
	shieldEffMult := 1.0
	if shieldLayer.RawHP > 0 {
		shieldEffMult = shieldLayer.EHP / shieldLayer.RawHP
	}
	armorEffMult := 1.0
	if armorLayer.RawHP > 0 {
		armorEffMult = armorLayer.EHP / armorLayer.RawHP
	}
	// Passive shield peak regen = 2.5 * ShieldHP / ShieldRechargeTime
	shieldRechargeSec := attrs[479] / 1000.0
	if shieldRechargeSec <= 0 {
		shieldRechargeSec = 625.0
	}
	passiveShieldHPs := (2.5 * shieldHP) / shieldRechargeSec
	sustainedTankEHPs := (passiveShieldHPs+shieldBoostHPs)*shieldEffMult + armorRepHPs*armorEffMult

	// Capacitor stability calculation (EVE Peak Recharge = 2.5 * Capacity / RechargeTime)
	capRechargeSec := capRechargeMs / 1000.0
	peakRecharge := 0.0
	if capRechargeSec > 0 {
		peakRecharge = (2.5 * capCapacity) / capRechargeSec
	}
	capStats := computeCapacitorStats(capCapacity, capRechargeSec, peakRecharge, activeCapDrain)

	// Align Time = -ln(0.25) * Inertia * (Mass / 1,000,000)
	alignSec := -math.Log(0.25) * inertia * (mass / 1_000_000.0)
	alignRating := "STANDARD"
	switch {
	case alignSec < 2.0:
		alignRating = "INSTA-WARP"
	case alignSec < 4.5:
		alignRating = "AGILE"
	case alignSec >= 10.0:
		alignRating = "HEAVY"
	}

	// Damage Mix normalization
	totalDPS := turretDPS + missileDPS + droneDPS
	dmgMix := normalizeDamageMix(dmgEM, dmgTh, dmgKi, dmgEx)

	// Applied DPS vs Distance curve (0..80 km in 5 km steps)
	rangeCurve := buildRangeCurve(turretDPS, missileDPS, droneDPS, bestOptimalKM, bestFalloffKM)

	cpuGauge := buildResourceGauge(cpuUsed, totalCPU)
	pgGauge := buildResourceGauge(pgUsed, totalPG)
	calibGauge := buildResourceGauge(calibUsed, baseCalib)

	// Beginner-friendly Readiness Diagnostics
	grade, findings := evaluateReadiness(
		cpuGauge, pgGauge, calibGauge,
		shieldLayer, armorLayer,
		sustainedTankEHPs, totalDPS, capStats,
		alignSec, faction,
	)

	hullCost := shipSpec.LiveSellPrice
	totalFitCost := RoundISK(hullCost + modulesCostISK)

	// Skill prerequisite gap evaluation (works for All 0..5, Max Alpha, or live EVE SSO skills)
	missingSkills, lockedKeys := EvaluateFitSkillGaps(fit, shipSpec, itemSpecs, gapCapLevel, liveSkills)

	return &FitSimulationResult{
		Fit:               fit,
		Slots:             shipSpec.Slots,
		Traits:            shipSpec.Traits,
		CPU:               cpuGauge,
		Powergrid:         pgGauge,
		Calibration:       calibGauge,
		TurretsUsed:       turretsUsed,
		LaunchersUsed:     launchersUsed,
		Shield:            shieldLayer,
		Armor:             armorLayer,
		Structure:         structLayer,
		TotalRawHP:        math.Round(totalRawHP),
		TotalEHP:          math.Round(totalEHP),
		SustainedTankEHPs: math.Round(sustainedTankEHPs*10) / 10,
		Capacitor:         capStats,
		Offense: OffenseStats{
			TurretDPS:      math.Round(turretDPS*10) / 10,
			MissileDPS:     math.Round(missileDPS*10) / 10,
			DroneDPS:       math.Round(droneDPS*10) / 10,
			TotalDPS:       math.Round(totalDPS*10) / 10,
			AlphaVolley:    math.Round(alphaVolley),
			OptimalRangeKM: math.Round(bestOptimalKM*10) / 10,
			FalloffRangeKM: math.Round(bestFalloffKM*10) / 10,
			DamageMix:      dmgMix,
		},
		Navigation: NavigationStats{
			MaxVelocityMS:      math.Round(maxVelocity),
			WarpSpeedAUS:       math.Round(warpSpeed*100) / 100,
			InertiaModifier:    math.Round(inertia*1000) / 1000,
			MassKG:             mass,
			AlignTimeSeconds:   math.Round(alignSec*100) / 100,
			AlignRating:        alignRating,
			SignatureRadiusM:   math.Round(sigRadius),
			CargoCapacityM3:    math.Round(cargoCap),
			DroneBandwidth:     droneBW,
			DroneBandwidthUsed: droneBWUsed,
			DroneBayM3:         droneBay,
			DroneBayUsedM3:     math.Round(droneBayUsed*10) / 10,
		},
		TargetFaction:   faction,
		ReadinessGrade:  grade,
		Findings:        findings,
		MissingSkills:   missingSkills,
		LockedSlotKeys:  lockedKeys,
		RangeCurve:      rangeCurve,
		TotalFitCostISK: totalFitCost,
		HullCostISK:     RoundISK(hullCost),
		ModulesCostISK:  RoundISK(modulesCostISK),
		HubName:         hubName,
		EFTExport:       ExportEFT(fit),
		MultibuyText:    FormatMultibuy(fit),
	}
}

// EvaluateFitSkillGaps checks ship hull & every fitted module's required skills against either
// a live EVE SSO pilot skill map or a uniform preset level (3, 4, or 5).
func EvaluateFitSkillGaps(
	fit ShipFit,
	shipSpec *ItemSpecDetail,
	itemSpecs map[int32]*ItemSpecDetail,
	presetLevel int,
	liveSkills map[int32]int,
) ([]MissingSkillGap, []string) {
	levelOf := func(skillID int32) int {
		if liveSkills != nil {
			return liveSkills[skillID]
		}
		return presetLevel
	}

	gaps := make([]MissingSkillGap, 0)
	lockedSet := make(map[string]struct{})
	lockedKeys := make([]string, 0)

	if shipSpec != nil {
		for _, req := range shipSpec.RequiredSkills {
			cur := levelOf(req.SkillTypeID)
			if cur < req.Level {
				gaps = append(gaps, MissingSkillGap{
					ItemName:      shipSpec.Name,
					SlotIndex:     -1,
					SkillTypeID:   req.SkillTypeID,
					SkillName:     req.SkillName,
					RequiredLevel: req.Level,
					PilotLevel:    cur,
				})
			}
		}
	}

	for _, m := range fit.Modules {
		sp := itemSpecs[m.TypeID]
		if sp == nil {
			continue
		}
		slotKey := fmt.Sprintf("%s-%d", m.SlotKind, m.SlotIndex)
		for _, req := range sp.RequiredSkills {
			cur := levelOf(req.SkillTypeID)
			if cur < req.Level {
				gaps = append(gaps, MissingSkillGap{
					ItemName:      sp.Name,
					SlotKind:      m.SlotKind,
					SlotIndex:     m.SlotIndex,
					SkillTypeID:   req.SkillTypeID,
					SkillName:     req.SkillName,
					RequiredLevel: req.Level,
					PilotLevel:    cur,
				})
				if _, seen := lockedSet[slotKey]; !seen {
					lockedSet[slotKey] = struct{}{}
					lockedKeys = append(lockedKeys, slotKey)
				}
			}
		}
	}
	return gaps, lockedKeys
}

func firstNegAbs(attrs map[int32]float64, ids ...int32) float64 {
	for _, id := range ids {
		if v := attrs[id]; v < 0 {
			return math.Abs(v)
		}
	}
	return 0
}

func applyStackedResistances(baseResonance float64, bonuses []float64) float64 {
	if len(bonuses) == 0 {
		return clampResonance(baseResonance)
	}
	cp := append([]float64(nil), bonuses...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] > cp[j] })
	res := baseResonance
	for idx, b := range cp {
		eff := b * StackingPenalty(idx)
		res *= (1.0 - eff)
	}
	return clampResonance(res)
}

func clampResonance(r float64) float64 {
	if r < 0.02 {
		return 0.02
	}
	if r > 1.0 {
		return 1.0
	}
	return r
}

func applyStackedMultipliers(bonuses []float64) float64 {
	if len(bonuses) == 0 {
		return 1.0
	}
	cp := append([]float64(nil), bonuses...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] > cp[j] })
	mult := 1.0
	for idx, b := range cp {
		mult *= (1.0 + b*StackingPenalty(idx))
	}
	return mult
}

func buildDefenseLayer(rawHP float64, res [4]float64, incoming DamageProfile) DefenseLayerStats {
	pcts := ResistanceProfile{
		EM:        math.Round((1.0-res[0])*1000) / 10,
		Thermal:   math.Round((1.0-res[1])*1000) / 10,
		Kinetic:   math.Round((1.0-res[2])*1000) / 10,
		Explosive: math.Round((1.0-res[3])*1000) / 10,
	}
	weightedResonance := incoming.EM*res[0] + incoming.Thermal*res[1] + incoming.Kinetic*res[2] + incoming.Explosive*res[3]
	if weightedResonance <= 0.01 {
		weightedResonance = 0.25
	}
	ehp := rawHP / weightedResonance

	lowestType := "EM"
	lowestVal := pcts.EM
	hasHole := false
	checks := []struct {
		label string
		val   float64
		share float64
	}{
		{"EM", pcts.EM, incoming.EM},
		{"Thermal", pcts.Thermal, incoming.Thermal},
		{"Kinetic", pcts.Kinetic, incoming.Kinetic},
		{"Explosive", pcts.Explosive, incoming.Explosive},
	}
	for _, c := range checks {
		if c.val < lowestVal {
			lowestVal = c.val
			lowestType = c.label
		}
		if c.share >= 0.25 && c.val < 25.0 {
			hasHole = true
		}
	}
	return DefenseLayerStats{
		RawHP:      math.Round(rawHP),
		EHP:        math.Round(ehp),
		Resists:    pcts,
		LowestType: lowestType,
		HasHole:    hasHole,
	}
}

func computeCapacitorStats(capGJ, rechargeSec, peakRecharge, drainGJs float64) CapacitorStats {
	st := CapacitorStats{
		CapacityGJ:      math.Round(capGJ),
		RechargeSeconds: math.Round(rechargeSec*10) / 10,
		PeakRechargeGJs: math.Round(peakRecharge*10) / 10,
		ActiveDrainGJs:  math.Round(drainGJs*10) / 10,
		DeltaGJs:        math.Round((peakRecharge-drainGJs)*10) / 10,
	}
	if drainGJs <= 0.01 {
		st.Stable = true
		st.StablePercent = 100
		st.FormattedTime = "Stable 100%"
		return st
	}
	if peakRecharge >= drainGJs && peakRecharge > 0 {
		// Equilibrium cap percentage from EVE recharge parabola: R(x) = 4 * peak * sqrt(x) * (1 - sqrt(x))
		ratio := drainGJs / peakRecharge
		disc := math.Max(0, 1.0-ratio)
		sqrtX := 0.5 * (1.0 + math.Sqrt(disc))
		pct := math.Round(sqrtX*sqrtX*1000) / 10
		if pct > 99.9 {
			pct = 99.9
		}
		st.Stable = true
		st.StablePercent = pct
		st.FormattedTime = fmt.Sprintf("Stable %.0f%%", pct)
		return st
	}

	// Unstable: approximate time until cap empty
	netDrain := drainGJs - (peakRecharge * 0.72)
	if netDrain <= 0.1 {
		netDrain = 0.1
	}
	secs := capGJ / netDrain
	st.Stable = false
	st.TimeToEmptySec = math.Round(secs)
	mins := int(secs) / 60
	rem := int(secs) % 60
	st.FormattedTime = fmt.Sprintf("%02d:%02d", mins, rem)
	return st
}

func buildResourceGauge(used, total float64) ResourceGauge {
	pct := 0.0
	if total > 0 {
		pct = (used / total) * 100.0
	}
	return ResourceGauge{
		Used:       math.Round(used*10) / 10,
		Total:      math.Round(total*10) / 10,
		Percent:    math.Round(pct*10) / 10,
		Remaining:  math.Round((total-used)*10) / 10,
		Overloaded: total > 0 && used > total+0.01,
	}
}

func normalizeDamageMix(em, th, ki, ex float64) DamageProfile {
	sum := em + th + ki + ex
	if sum <= 0 {
		return DamageProfile{EM: 25, Thermal: 25, Kinetic: 25, Explosive: 25}
	}
	return DamageProfile{
		EM:        math.Round((em / sum) * 100),
		Thermal:   math.Round((th / sum) * 100),
		Kinetic:   math.Round((ki / sum) * 100),
		Explosive: math.Round((ex / sum) * 100),
	}
}

func buildRangeCurve(turretDPS, missileDPS, droneDPS, optKM, fallKM float64) []RangeCurvePoint {
	if optKM <= 0 && turretDPS > 0 {
		optKM = 12
	}
	if fallKM <= 0 && turretDPS > 0 {
		fallKM = 8
	}
	pts := make([]RangeCurvePoint, 0, 17)
	for d := 0.0; d <= 80.0; d += 5.0 {
		appTurret := turretDPS
		if turretDPS > 0 && d > optKM {
			ratio := (d - optKM) / math.Max(fallKM, 1.0)
			appTurret = turretDPS * math.Pow(0.5, ratio*ratio)
		}
		appMissile := missileDPS
		if missileDPS > 0 && d > math.Max(optKM, 30) {
			appMissile = 0
		}
		appDrone := droneDPS
		if droneDPS > 0 && d > 55 {
			appDrone = 0
		}
		pts = append(pts, RangeCurvePoint{
			DistanceKM: d,
			AppliedDPS: math.Round((appTurret+appMissile+appDrone)*10) / 10,
		})
	}
	return pts
}

func evaluateReadiness(
	cpu, pg, calib ResourceGauge,
	shield, armor DefenseLayerStats,
	sustainedEHPs, totalDPS float64,
	cap CapacitorStats,
	alignSec float64,
	faction PvEFactionPreset,
) (string, []ReadinessFinding) {
	findings := make([]ReadinessFinding, 0, 5)
	score := 100

	if cpu.Overloaded || pg.Overloaded || calib.Overloaded {
		score -= 45
		findings = append(findings, ReadinessFinding{
			Severity: "critical",
			Title:    "Fitting Resource Overload",
			Detail:   "CPU, Powergrid, or Calibration exceeds 100%. Swap a module to Compact or fit a Co-Processor / Reactor Control Unit.",
		})
	}
	if shield.HasHole && shield.RawHP >= armor.RawHP {
		score -= 22
		findings = append(findings, ReadinessFinding{
			Severity: "critical",
			Title:    fmt.Sprintf("Shield %s Resistance Hole vs %s", shield.LowestType, faction.ShortLabel),
			Detail:   faction.RecommendedTip,
		})
	} else if armor.HasHole && armor.RawHP > shield.RawHP {
		score -= 22
		findings = append(findings, ReadinessFinding{
			Severity: "critical",
			Title:    fmt.Sprintf("Armor %s Resistance Hole vs %s", armor.LowestType, faction.ShortLabel),
			Detail:   faction.RecommendedTip,
		})
	}
	if !cap.Stable && cap.TimeToEmptySec < 120 {
		score -= 20
		findings = append(findings, ReadinessFinding{
			Severity: "warn",
			Title:    fmt.Sprintf("Capacitor Depletes in %s", cap.FormattedTime),
			Detail:   "Add a Cap Recharger, Capacitor Flux Coil, or pulse your active repairer/propulsion module.",
		})
	}
	if sustainedEHPs < faction.BenchmarkDPS*0.45 && totalDPS > 0 {
		score -= 15
		findings = append(findings, ReadinessFinding{
			Severity: "warn",
			Title:    fmt.Sprintf("Light Tank (%.0f EHP/s vs %.0f DPS Benchmark)", sustainedEHPs, faction.BenchmarkDPS),
			Detail:   "Maintain range/transversal speed or upgrade your active tank before running higher-tier sites.",
		})
	}
	if len(findings) == 0 {
		findings = append(findings, ReadinessFinding{
			Severity: "ok",
			Title:    fmt.Sprintf("Combat Ready vs %s", faction.ShortLabel),
			Detail:   fmt.Sprintf("Resources balanced, %.0f EHP/s sustained tank, align time %.2fs.", sustainedEHPs, alignSec),
		})
	}

	grade := "S"
	switch {
	case score < 55:
		grade = "RISK"
	case score < 70:
		grade = "C"
	case score < 82:
		grade = "B"
	case score < 94:
		grade = "A"
	}
	return grade, findings
}
