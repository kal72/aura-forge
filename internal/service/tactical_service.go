package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"aura-forge/internal/domain"
)

// TacticalSDESearcher is an optional extension implemented by postgres.SDERepo for fast local ship/module queries.
type TacticalSDESearcher interface {
	SearchTacticalShips(ctx context.Context, hullClass, query string, limit int) ([]domain.SDEType, error)
	SearchTacticalModules(ctx context.Context, slotKind, query string, limit int) ([]domain.SDEType, error)
}

// TacticalPresetSummary describes one built-in beginner-friendly starter doctrine.
type TacticalPresetSummary struct {
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	ShipName     string         `json:"ship_name"`
	ShipTypeID   int32          `json:"ship_type_id"`
	HullClass    string         `json:"hull_class"`
	Tag          string         `json:"tag"`
	FactionTheme string         `json:"faction_theme"`
	Summary      string         `json:"summary"`
	Fit          domain.ShipFit `json:"fit"`
}

// TacticalViewData bundles initial state for the fullscreen Aura Tactical Studio page.
type TacticalViewData struct {
	Mode          ModeInfo                       `json:"mode"`
	ActiveHub     domain.TradeHub                `json:"active_hub"`
	Factions      []domain.PvEFactionPreset      `json:"factions"`
	DuelOpponents []domain.DuelOpponentPreset    `json:"duel_opponents"`
	Presets       []TacticalPresetSummary        `json:"presets"`
	SavedFittings []domain.TacticalFittingRecord `json:"saved_fittings"`
}

// BuiltInTacticalPresets returns curated starter doctrines ready to fly & explore on the Orbital Fitting Wheel.
func BuiltInTacticalPresets() []TacticalPresetSummary {
	return []TacticalPresetSummary{
		{
			Key:          "omen_amarr",
			Name:         "Omen — Imperial Pulse Cruiser",
			ShipName:     "Omen",
			ShipTypeID:   2006,
			HullClass:    "Cruiser",
			Tag:          "PvE",
			FactionTheme: "amarr",
			Summary:      "Amarr laser cruiser with dual armor reps and balanced EM/Thermal projection.",
			Fit: domain.ShipFit{
				Name:         "Omen — Imperial Pulse Cruiser",
				ShipTypeID:   2006,
				ShipName:     "Omen",
				HullClass:    "Cruiser",
				FactionTheme: "amarr",
				Tag:          "PvE",
				SkillProfile: domain.SkillProfileIntermediate,
				TargetPreset: "sansha",
				Modules: []domain.FitModuleSlot{
					{SlotKind: domain.SlotHigh, SlotIndex: 0, TypeID: 3566, Name: "Focused Medium Pulse Laser II", ChargeTypeID: 237, ChargeName: "Multifrequency M", State: domain.ModuleActive},
					{SlotKind: domain.SlotHigh, SlotIndex: 1, TypeID: 3566, Name: "Focused Medium Pulse Laser II", ChargeTypeID: 237, ChargeName: "Multifrequency M", State: domain.ModuleActive},
					{SlotKind: domain.SlotHigh, SlotIndex: 2, TypeID: 3566, Name: "Focused Medium Pulse Laser II", ChargeTypeID: 237, ChargeName: "Multifrequency M", State: domain.ModuleActive},
					{SlotKind: domain.SlotHigh, SlotIndex: 3, TypeID: 3566, Name: "Focused Medium Pulse Laser II", ChargeTypeID: 237, ChargeName: "Multifrequency M", State: domain.ModuleActive},
					{SlotKind: domain.SlotHigh, SlotIndex: 4, TypeID: 3566, Name: "Focused Medium Pulse Laser II", ChargeTypeID: 237, ChargeName: "Multifrequency M", State: domain.ModuleActive},
					{SlotKind: domain.SlotMid, SlotIndex: 0, TypeID: 12058, Name: "10MN Afterburner II", State: domain.ModuleActive},
					{SlotKind: domain.SlotMid, SlotIndex: 1, TypeID: 2032, Name: "Cap Recharger II", State: domain.ModulePassive},
					{SlotKind: domain.SlotMid, SlotIndex: 2, TypeID: 527, Name: "Stasis Webifier II", State: domain.ModuleActive},
					{SlotKind: domain.SlotLow, SlotIndex: 0, TypeID: 3530, Name: "Medium Armor Repairer II", State: domain.ModuleActive},
					{SlotKind: domain.SlotLow, SlotIndex: 1, TypeID: 11269, Name: "Multispectrum Energized Membrane II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 2, TypeID: 2048, Name: "Damage Control II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 3, TypeID: 2364, Name: "Heat Sink II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 4, TypeID: 2364, Name: "Heat Sink II", State: domain.ModulePassive},
					{SlotKind: domain.SlotRig, SlotIndex: 0, TypeID: 31360, Name: "Medium Auxiliary Nano Pump I", State: domain.ModulePassive},
					{SlotKind: domain.SlotRig, SlotIndex: 1, TypeID: 31790, Name: "Medium Capacitor Control Circuit I", State: domain.ModulePassive},
				},
				Drones: []domain.FitDroneEntry{
					{TypeID: 2488, Name: "Warrior II", Quantity: 5, Active: true},
				},
			},
		},
		{
			Key:          "caracal_guristas",
			Name:         "Caracal — L2/L3 Guristas Striker",
			ShipName:     "Caracal",
			ShipTypeID:   621,
			HullClass:    "Cruiser",
			Tag:          "PvE",
			FactionTheme: "caldari",
			Summary:      "Beginner-friendly Rapid Light Missile cruiser with high Kinetic shield tank.",
			Fit: domain.ShipFit{
				Name:         "Caracal — L2/L3 Guristas Striker",
				ShipTypeID:   621,
				ShipName:     "Caracal",
				HullClass:    "Cruiser",
				FactionTheme: "caldari",
				Tag:          "PvE",
				SkillProfile: domain.SkillProfileBeginner,
				TargetPreset: "guristas",
				Modules: []domain.FitModuleSlot{
					{SlotKind: domain.SlotHigh, SlotIndex: 0, TypeID: 2410, Name: "Rapid Light Missile Launcher II", ChargeTypeID: 209, ChargeName: "Scourge Light Missile", State: domain.ModuleActive},
					{SlotKind: domain.SlotHigh, SlotIndex: 1, TypeID: 2410, Name: "Rapid Light Missile Launcher II", ChargeTypeID: 209, ChargeName: "Scourge Light Missile", State: domain.ModuleActive},
					{SlotKind: domain.SlotHigh, SlotIndex: 2, TypeID: 2410, Name: "Rapid Light Missile Launcher II", ChargeTypeID: 209, ChargeName: "Scourge Light Missile", State: domain.ModuleActive},
					{SlotKind: domain.SlotHigh, SlotIndex: 3, TypeID: 2410, Name: "Rapid Light Missile Launcher II", ChargeTypeID: 209, ChargeName: "Scourge Light Missile", State: domain.ModuleActive},
					{SlotKind: domain.SlotHigh, SlotIndex: 4, TypeID: 2410, Name: "Rapid Light Missile Launcher II", ChargeTypeID: 209, ChargeName: "Scourge Light Missile", State: domain.ModuleActive},
					{SlotKind: domain.SlotMid, SlotIndex: 0, TypeID: 3841, Name: "Large Shield Extender II", State: domain.ModulePassive},
					{SlotKind: domain.SlotMid, SlotIndex: 1, TypeID: 3841, Name: "Large Shield Extender II", State: domain.ModulePassive},
					{SlotKind: domain.SlotMid, SlotIndex: 2, TypeID: 2281, Name: "Multispectrum Shield Hardener II", State: domain.ModuleActive},
					{SlotKind: domain.SlotMid, SlotIndex: 3, TypeID: 12058, Name: "10MN Afterburner II", State: domain.ModuleActive},
					{SlotKind: domain.SlotLow, SlotIndex: 0, TypeID: 22291, Name: "Ballistic Control System II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 1, TypeID: 22291, Name: "Ballistic Control System II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 2, TypeID: 2048, Name: "Damage Control II", State: domain.ModulePassive},
					{SlotKind: domain.SlotRig, SlotIndex: 0, TypeID: 31718, Name: "Medium Core Defense Field Extender I", State: domain.ModulePassive},
					{SlotKind: domain.SlotRig, SlotIndex: 1, TypeID: 31718, Name: "Medium Core Defense Field Extender I", State: domain.ModulePassive},
				},
				Drones: []domain.FitDroneEntry{
					{TypeID: 2456, Name: "Hobgoblin II", Quantity: 2, Active: true},
				},
			},
		},
		{
			Key:          "vexor_ratting",
			Name:         "Vexor — Drone Anomaly Cruiser",
			ShipName:     "Vexor",
			ShipTypeID:   626,
			HullClass:    "Cruiser",
			Tag:          "PvE",
			FactionTheme: "gallente",
			Summary:      "AFK-friendly Gallente drone boat with 10MN Afterburner speed tanking.",
			Fit: domain.ShipFit{
				Name:         "Vexor — Drone Anomaly Cruiser",
				ShipTypeID:   626,
				ShipName:     "Vexor",
				HullClass:    "Cruiser",
				FactionTheme: "gallente",
				Tag:          "PvE",
				SkillProfile: domain.SkillProfileIntermediate,
				TargetPreset: "serpentis",
				Modules: []domain.FitModuleSlot{
					{SlotKind: domain.SlotMid, SlotIndex: 0, TypeID: 12058, Name: "10MN Afterburner II", State: domain.ModuleActive},
					{SlotKind: domain.SlotMid, SlotIndex: 1, TypeID: 3841, Name: "Large Shield Extender II", State: domain.ModulePassive},
					{SlotKind: domain.SlotMid, SlotIndex: 2, TypeID: 2281, Name: "Multispectrum Shield Hardener II", State: domain.ModuleActive},
					{SlotKind: domain.SlotLow, SlotIndex: 0, TypeID: 4405, Name: "Drone Damage Amplifier II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 1, TypeID: 4405, Name: "Drone Damage Amplifier II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 2, TypeID: 4405, Name: "Drone Damage Amplifier II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 3, TypeID: 2048, Name: "Damage Control II", State: domain.ModulePassive},
					{SlotKind: domain.SlotRig, SlotIndex: 0, TypeID: 31718, Name: "Medium Core Defense Field Extender I", State: domain.ModulePassive},
				},
				Drones: []domain.FitDroneEntry{
					{TypeID: 2185, Name: "Hammerhead II", Quantity: 5, Active: true},
				},
			},
		},
		{
			Key:          "nereus_antigank",
			Name:         "Nereus — Anti-Gank High-Sec Hauler",
			ShipName:     "Nereus",
			ShipTypeID:   650,
			HullClass:    "Hauler",
			Tag:          "Hauling",
			FactionTheme: "gallente",
			Summary:      "Heavy shield-buffer industrial hauler built to survive High-Sec Catalyst ganks.",
			Fit: domain.ShipFit{
				Name:         "Nereus — Anti-Gank High-Sec Hauler",
				ShipTypeID:   650,
				ShipName:     "Nereus",
				HullClass:    "Hauler",
				FactionTheme: "gallente",
				Tag:          "Hauling",
				SkillProfile: domain.SkillProfileIntermediate,
				TargetPreset: "guristas",
				Modules: []domain.FitModuleSlot{
					{SlotKind: domain.SlotMid, SlotIndex: 0, TypeID: 3841, Name: "Large Shield Extender II", State: domain.ModulePassive},
					{SlotKind: domain.SlotMid, SlotIndex: 1, TypeID: 3841, Name: "Large Shield Extender II", State: domain.ModulePassive},
					{SlotKind: domain.SlotMid, SlotIndex: 2, TypeID: 2281, Name: "Multispectrum Shield Hardener II", State: domain.ModuleActive},
					{SlotKind: domain.SlotMid, SlotIndex: 3, TypeID: 2281, Name: "Multispectrum Shield Hardener II", State: domain.ModuleActive},
					{SlotKind: domain.SlotLow, SlotIndex: 0, TypeID: 2048, Name: "Damage Control II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 1, TypeID: 1405, Name: "Inertial Stabilizers II", State: domain.ModulePassive},
					{SlotKind: domain.SlotLow, SlotIndex: 2, TypeID: 1405, Name: "Inertial Stabilizers II", State: domain.ModulePassive},
					{SlotKind: domain.SlotRig, SlotIndex: 0, TypeID: 31718, Name: "Medium Core Defense Field Extender I", State: domain.ModulePassive},
					{SlotKind: domain.SlotRig, SlotIndex: 1, TypeID: 31718, Name: "Medium Core Defense Field Extender I", State: domain.ModulePassive},
				},
			},
		},
	}
}

// GetTacticalViewData builds the initial state for /tactical.
func (s *CopilotService) GetTacticalViewData(ctx context.Context) *TacticalViewData {
	mode := s.GetModeInfo(ctx)
	hub := s.ActiveCatalogHub(ctx)
	var saved []domain.TacticalFittingRecord
	if s.tacticalRepo != nil {
		var uid int64
		if mode.CurrentUser != nil {
			uid = mode.CurrentUser.ID
		}
		if list, err := s.tacticalRepo.ListFittings(ctx, uid, 40); err == nil {
			saved = list
		}
	}
	return &TacticalViewData{
		Mode:          mode,
		ActiveHub:     hub,
		Factions:      domain.BuiltInPvEFactions(),
		DuelOpponents: domain.BuiltInDuelOpponents(),
		Presets:       BuiltInTacticalPresets(),
		SavedFittings: saved,
	}
}

// SearchTacticalShips queries local SDE ship hulls (0 ESI calls) with built-in popular fallback.
func (s *CopilotService) SearchTacticalShips(ctx context.Context, hullClass, query string, limit int) []domain.SDEType {
	if searcher, ok := s.sde.(TacticalSDESearcher); ok && searcher != nil {
		if items, err := searcher.SearchTacticalShips(ctx, hullClass, query, limit); err == nil && len(items) > 0 {
			return items
		}
	}
	// Fallback popular ships if SDE isn't loaded yet
	fallback := []domain.SDEType{
		{TypeID: 2006, Name: "Omen", GroupName: "Cruiser", Category: "Ship"},
		{TypeID: 621, Name: "Caracal", GroupName: "Cruiser", Category: "Ship"},
		{TypeID: 626, Name: "Vexor", GroupName: "Cruiser", Category: "Ship"},
		{TypeID: 622, Name: "Stabber", GroupName: "Cruiser", Category: "Ship"},
		{TypeID: 17715, Name: "Gila", GroupName: "Cruiser", Category: "Ship"},
		{TypeID: 24698, Name: "Drake", GroupName: "Combat Battlecruiser", Category: "Ship"},
		{TypeID: 24696, Name: "Harbinger", GroupName: "Combat Battlecruiser", Category: "Ship"},
		{TypeID: 24702, Name: "Hurricane", GroupName: "Combat Battlecruiser", Category: "Ship"},
		{TypeID: 638, Name: "Raven", GroupName: "Battleship", Category: "Ship"},
		{TypeID: 645, Name: "Dominix", GroupName: "Battleship", Category: "Ship"},
		{TypeID: 597, Name: "Punisher", GroupName: "Frigate", Category: "Ship"},
		{TypeID: 605, Name: "Heron", GroupName: "Frigate", Category: "Ship"},
		{TypeID: 593, Name: "Tristan", GroupName: "Frigate", Category: "Ship"},
		{TypeID: 16240, Name: "Catalyst", GroupName: "Destroyer", Category: "Ship"},
		{TypeID: 16236, Name: "Coercer", GroupName: "Destroyer", Category: "Ship"},
		{TypeID: 650, Name: "Nereus", GroupName: "Hauler", Category: "Ship"},
		{TypeID: 649, Name: "Tayra", GroupName: "Hauler", Category: "Ship"},
	}
	q := strings.ToLower(strings.TrimSpace(query))
	hc := strings.ToLower(strings.TrimSpace(hullClass))
	out := make([]domain.SDEType, 0, len(fallback))
	for _, f := range fallback {
		if hc != "" && hc != "all" && !strings.Contains(strings.ToLower(f.GroupName), hc) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(f.Name), q) && !strings.Contains(strings.ToLower(f.GroupName), q) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// SearchTacticalModules queries local SDE modules/rigs/drones/charges (0 ESI calls).
func (s *CopilotService) SearchTacticalModules(ctx context.Context, slotKind, query string, limit int) []domain.SDEType {
	if searcher, ok := s.sde.(TacticalSDESearcher); ok && searcher != nil {
		if items, err := searcher.SearchTacticalModules(ctx, slotKind, query, limit); err == nil && len(items) > 0 {
			return items
		}
	}
	// Fallback curated modules when SDE isn't imported yet
	curated := map[string][]domain.SDEType{
		"high": {
			{TypeID: 3566, Name: "Focused Medium Pulse Laser II", GroupName: "Energy Weapon"},
			{TypeID: 2410, Name: "Rapid Light Missile Launcher II", GroupName: "Missile Launcher Rapid Light"},
			{TypeID: 2929, Name: "Heavy Neutron Blaster II", GroupName: "Hybrid Weapon"},
			{TypeID: 2969, Name: "220mm Vulcan AutoCannon II", GroupName: "Projectile Weapon"},
			{TypeID: 11578, Name: "Improved Cloaking Device II", GroupName: "Cloaking Device"},
		},
		"mid": {
			{TypeID: 3841, Name: "Large Shield Extender II", GroupName: "Shield Extender"},
			{TypeID: 2281, Name: "Multispectrum Shield Hardener II", GroupName: "Shield Hardener"},
			{TypeID: 12058, Name: "10MN Afterburner II", GroupName: "Propulsion Module"},
			{TypeID: 35658, Name: "50MN Microwarpdrive II", GroupName: "Propulsion Module"},
			{TypeID: 2032, Name: "Cap Recharger II", GroupName: "Capacitor Recharger"},
			{TypeID: 527, Name: "Stasis Webifier II", GroupName: "Stasis Web"},
			{TypeID: 448, Name: "Warp Scrambler II", GroupName: "Warp Scrambler"},
		},
		"low": {
			{TypeID: 2048, Name: "Damage Control II", GroupName: "Damage Control"},
			{TypeID: 3530, Name: "Medium Armor Repairer II", GroupName: "Armor Repair Unit"},
			{TypeID: 11269, Name: "Multispectrum Energized Membrane II", GroupName: "Armor Coating"},
			{TypeID: 2364, Name: "Heat Sink II", GroupName: "Heat Sink"},
			{TypeID: 22291, Name: "Ballistic Control System II", GroupName: "Ballistic Control system"},
			{TypeID: 4405, Name: "Drone Damage Amplifier II", GroupName: "Drone Damage Modules"},
			{TypeID: 1405, Name: "Inertial Stabilizers II", GroupName: "Inertial Stabilizer"},
		},
		"rig": {
			{TypeID: 31718, Name: "Medium Core Defense Field Extender I", GroupName: "Rig Shield"},
			{TypeID: 31360, Name: "Medium Auxiliary Nano Pump I", GroupName: "Rig Armor"},
			{TypeID: 31790, Name: "Medium Capacitor Control Circuit I", GroupName: "Rig Energy Grid"},
		},
		"drone": {
			{TypeID: 2456, Name: "Hobgoblin II", GroupName: "Combat Drone"},
			{TypeID: 2488, Name: "Warrior II", GroupName: "Combat Drone"},
			{TypeID: 2185, Name: "Hammerhead II", GroupName: "Combat Drone"},
		},
		"charge": {
			{TypeID: 237, Name: "Multifrequency M", GroupName: "Frequency Crystal"},
			{TypeID: 12814, Name: "Scorch M", GroupName: "Advanced Pulse Laser Crystal"},
			{TypeID: 12818, Name: "Conflagration M", GroupName: "Advanced Pulse Laser Crystal"},
			{TypeID: 243, Name: "Radio M", GroupName: "Frequency Crystal"},
			{TypeID: 209, Name: "Scourge Light Missile", GroupName: "Light Missile"},
			{TypeID: 210, Name: "Inferno Light Missile", GroupName: "Light Missile"},
			{TypeID: 211, Name: "Mjolnir Light Missile", GroupName: "Light Missile"},
			{TypeID: 212, Name: "Nova Light Missile", GroupName: "Light Missile"},
			{TypeID: 222, Name: "Antimatter Charge M", GroupName: "Hybrid Charge"},
			{TypeID: 12789, Name: "Void M", GroupName: "Advanced Blaster Charge"},
			{TypeID: 12791, Name: "Null M", GroupName: "Advanced Blaster Charge"},
			{TypeID: 192, Name: "EMP M", GroupName: "Projectile Ammo"},
			{TypeID: 193, Name: "Phased Plasma M", GroupName: "Projectile Ammo"},
			{TypeID: 194, Name: "Fusion M", GroupName: "Projectile Ammo"},
			{TypeID: 12779, Name: "Barrage M", GroupName: "Advanced Autocannon Ammo"},
		},
	}
	list := curated[strings.ToLower(strings.TrimSpace(slotKind))]
	if len(list) == 0 {
		for _, v := range curated {
			list = append(list, v...)
		}
	}
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]domain.SDEType, 0, len(list))
	for _, it := range list {
		if q != "" && !strings.Contains(strings.ToLower(it.Name), q) && !strings.Contains(strings.ToLower(it.GroupName), q) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// SimulateTacticalFit hydrates Dogma specs + traits + live Trade Hub prices concurrently
// (bounded worker pool) and executes deterministic EVE Dogma combat math.
func (s *CopilotService) SimulateTacticalFit(ctx context.Context, fit domain.ShipFit) (*domain.FitSimulationResult, error) {
	if fit.ShipTypeID <= 0 {
		return nil, fmt.Errorf("ship_type_id is required")
	}

	hub := s.ActiveCatalogHub(ctx)

	// Collect unique type IDs to hydrate
	idSet := map[int32]struct{}{fit.ShipTypeID: {}}
	for _, m := range fit.Modules {
		if m.TypeID > 0 {
			idSet[m.TypeID] = struct{}{}
		}
		if m.ChargeTypeID > 0 {
			idSet[m.ChargeTypeID] = struct{}{}
		}
	}
	for _, d := range fit.Drones {
		if d.TypeID > 0 {
			idSet[d.TypeID] = struct{}{}
		}
	}
	allIDs := make([]int32, 0, len(idSet))
	for id := range idSet {
		allIDs = append(allIDs, id)
	}

	var (
		mu        sync.Mutex
		specs     = make(map[int32]*domain.ItemSpecDetail, len(allIDs))
		prices    map[int32]domain.JitaPrice
		shipTrait *domain.ItemTraits
		wg        sync.WaitGroup
	)

	// 1. Live hub prices in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		prices, _ = s.fetchHubPrices(ctx, hub, allIDs)
	}()

	// 2. Local SDE hull traits in parallel
	if trRepo, ok := s.sde.(domain.SDETypeTraitsRepository); ok && trRepo != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tr, err := trRepo.GetTypeTraits(ctx, fit.ShipTypeID); err == nil && tr != nil && tr.HasAny() {
				shipTrait = tr
			}
		}()
	}

	// 3. Bounded parallel Dogma spec fetches (cached in PostgreSQL esi_cache)
	if specProv, ok := s.market.(domain.ItemSpecProvider); ok && specProv != nil {
		sem := make(chan struct{}, 6)
		for _, tid := range allIDs {
			wg.Add(1)
			go func(id int32) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				sp, err := specProv.GetItemSpecs(ctx, id)
				if err == nil && sp != nil {
					mu.Lock()
					specs[id] = sp
					mu.Unlock()
				}
			}(tid)
		}
	}
	wg.Wait()

	// Attach live prices onto specs
	for id, sp := range specs {
		if jp, ok := prices[id]; ok && jp.LowestSell > 0 {
			sp.LiveSellPrice = domain.RoundISK(jp.LowestSell)
		}
	}

	shipSpec := specs[fit.ShipTypeID]
	if shipSpec == nil {
		// Graceful synthetic fallback if offline/ESI unreachable
		shipSpec = &domain.ItemSpecDetail{
			TypeID:        fit.ShipTypeID,
			Name:          fit.ShipName,
			GroupName:     fit.HullClass,
			IsShip:        true,
			Mass:          11_000_000,
			Capacity:      400,
			Slots:         domain.SlotLayout{HighSlots: 5, MidSlots: 4, LowSlots: 5, RigSlots: 3, TurretSlots: 5},
			ShieldResists: domain.ResistanceProfile{EM: 0, Thermal: 20, Kinetic: 40, Explosive: 50},
			ArmorResists:  domain.ResistanceProfile{EM: 50, Thermal: 35, Kinetic: 25, Explosive: 20},
			Attributes: map[int32]float64{
				48: 350, 11: 950, 1132: 400,
				263: 1500, 265: 2200, 9: 1800,
				482: 1400, 55: 280000,
				37: 250, 600: 3.0, 70: 0.55, 552: 130,
				283: 40, 1271: 25,
			},
		}
	}
	if shipTrait != nil {
		shipSpec.Traits = shipTrait
	}
	if fit.ShipName == "" {
		fit.ShipName = shipSpec.Name
	}
	if fit.HullClass == "" {
		fit.HullClass = shipSpec.GroupName
	}

	// Enrich module metadata (Name, Group, Price)
	for i := range fit.Modules {
		if sp := specs[fit.Modules[i].TypeID]; sp != nil {
			if fit.Modules[i].Name == "" {
				fit.Modules[i].Name = sp.Name
			}
			fit.Modules[i].GroupName = sp.GroupName
			fit.Modules[i].TechLevel = sp.TechLevel
			fit.Modules[i].MetaLevel = sp.MetaLevel
			fit.Modules[i].UnitPriceISK = sp.LiveSellPrice
		}
		if fit.Modules[i].ChargeTypeID > 0 {
			if cs := specs[fit.Modules[i].ChargeTypeID]; cs != nil && fit.Modules[i].ChargeName == "" {
				fit.Modules[i].ChargeName = cs.Name
			}
		}
	}
	for i := range fit.Drones {
		if sp := specs[fit.Drones[i].TypeID]; sp != nil {
			if fit.Drones[i].Name == "" {
				fit.Drones[i].Name = sp.Name
			}
			fit.Drones[i].UnitPriceISK = sp.LiveSellPrice
		}
	}

	// Optional Live EVE SSO Pilot Skills (cached in esi_cache)
	var (
		liveSkills map[int32]int
		totalSP    int64
	)
	if fit.SkillProfile == domain.SkillProfilePilotLive && s.chars != nil && s.charSync != nil {
		if skProv, ok := s.charSync.(domain.CharacterSkillsProvider); ok {
			if tok, tErr := s.chars.GetPrimaryToken(ctx); tErr == nil && tok != nil {
				if sk, sp, sErr := skProv.SyncSkills(ctx, tok); sErr == nil && len(sk) > 0 {
					liveSkills = sk
					totalSP = sp
				}
			}
		}
	}

	sim := domain.ComputeFitSimulationWithSkills(fit, shipSpec, specs, hub.Name, liveSkills)
	if len(liveSkills) > 0 {
		sim.PilotSkillsActive = true
		sim.PilotTotalSP = totalSP
	}
	return sim, nil
}

// TacticalFitComparison holds concurrent side-by-side simulation results for two ShipFits.
type TacticalFitComparison struct {
	FitA *domain.FitSimulationResult `json:"fit_a"`
	FitB *domain.FitSimulationResult `json:"fit_b"`
}

// CompareTacticalFits simulates two ship fittings concurrently via sync.WaitGroup for side-by-side comparison.
func (s *CopilotService) CompareTacticalFits(ctx context.Context, fitA, fitB domain.ShipFit) (*TacticalFitComparison, error) {
	var (
		resA, resB *domain.FitSimulationResult
		errA, errB error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		resA, errA = s.SimulateTacticalFit(ctx, fitA)
	}()
	go func() {
		defer wg.Done()
		resB, errB = s.SimulateTacticalFit(ctx, fitB)
	}()
	wg.Wait()
	if errA != nil {
		return nil, errA
	}
	if errB != nil {
		return nil, errB
	}
	return &TacticalFitComparison{FitA: resA, FitB: resB}, nil
}

// ImportTacticalEFT parses an EFT clipboard block, resolves all TypeIDs concurrently, and runs simulation.
func (s *CopilotService) ImportTacticalEFT(ctx context.Context, rawEFT string, skillProfile domain.SkillProfilePreset, targetPreset string) (*domain.FitSimulationResult, error) {
	parsed, err := domain.ParseEFT(rawEFT)
	if err != nil {
		return nil, err
	}

	// Collect all unique names to resolve
	nameSet := map[string]struct{}{parsed.ShipName: {}}
	addLines := func(lines []domain.ParsedEFTLine) {
		for _, ln := range lines {
			nameSet[ln.ModuleName] = struct{}{}
			if ln.ChargeName != "" {
				nameSet[ln.ChargeName] = struct{}{}
			}
		}
	}
	addLines(parsed.Low)
	addLines(parsed.Mid)
	addLines(parsed.High)
	addLines(parsed.Rig)
	for _, d := range parsed.Drones {
		nameSet[d.Name] = struct{}{}
	}

	// Resolve names concurrently (local SDE first, Public ESI /universe/ids fallback)
	resolved := make(map[string]int32, len(nameSet))
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, 6)
	)
	for name := range nameSet {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if tid, rErr := s.resolveCompareQueryTypeID(ctx, n); rErr == nil && tid > 0 {
				mu.Lock()
				resolved[n] = tid
				mu.Unlock()
			}
		}(name)
	}
	wg.Wait()

	shipID := resolved[parsed.ShipName]
	if shipID <= 0 {
		return nil, fmt.Errorf("ship hull %q not found in EVE SDE or ESI", parsed.ShipName)
	}

	fit := domain.ShipFit{
		Name:         parsed.FitName,
		ShipTypeID:   shipID,
		ShipName:     parsed.ShipName,
		Tag:          "PvE",
		SkillProfile: skillProfile,
		TargetPreset: targetPreset,
	}

	appendRack := func(kind domain.FitSlotKind, lines []domain.ParsedEFTLine) {
		for idx, ln := range lines {
			tid := resolved[ln.ModuleName]
			if tid <= 0 {
				continue
			}
			st := domain.ModuleActive
			if kind == domain.SlotRig {
				st = domain.ModulePassive
			}
			if ln.Offline {
				st = domain.ModuleOffline
			}
			fit.Modules = append(fit.Modules, domain.FitModuleSlot{
				SlotKind:     kind,
				SlotIndex:    idx,
				TypeID:       tid,
				Name:         ln.ModuleName,
				ChargeTypeID: resolved[ln.ChargeName],
				ChargeName:   ln.ChargeName,
				State:        st,
			})
		}
	}
	appendRack(domain.SlotHigh, parsed.High)
	appendRack(domain.SlotMid, parsed.Mid)
	appendRack(domain.SlotLow, parsed.Low)
	appendRack(domain.SlotRig, parsed.Rig)

	for _, d := range parsed.Drones {
		if tid := resolved[d.Name]; tid > 0 {
			fit.Drones = append(fit.Drones, domain.FitDroneEntry{
				TypeID:   tid,
				Name:     d.Name,
				Quantity: d.Quantity,
				Active:   true,
			})
		}
	}

	return s.SimulateTacticalFit(ctx, fit)
}

// SaveTacticalFitting persists a ShipFit into PostgreSQL after computing fresh simulation metrics.
func (s *CopilotService) SaveTacticalFitting(ctx context.Context, fit domain.ShipFit) (*domain.TacticalFittingRecord, error) {
	if s.tacticalRepo == nil {
		return nil, fmt.Errorf("tactical repository not configured")
	}
	sim, err := s.SimulateTacticalFit(ctx, fit)
	if err != nil {
		return nil, err
	}
	mode := s.GetModeInfo(ctx)
	var uid int64
	if mode.CurrentUser != nil {
		uid = mode.CurrentUser.ID
	}
	name := strings.TrimSpace(fit.Name)
	if name == "" {
		name = sim.Fit.ShipName + " Tactical Fit"
	}
	tag := strings.TrimSpace(fit.Tag)
	if tag == "" {
		tag = "PvE"
	}
	rec := &domain.TacticalFittingRecord{
		ID:               fit.ID,
		UserID:           uid,
		Name:             name,
		ShipTypeID:       sim.Fit.ShipTypeID,
		ShipName:         sim.Fit.ShipName,
		HullClass:        sim.Fit.HullClass,
		Tag:              tag,
		SkillProfile:     string(sim.Fit.SkillProfile),
		EFTText:          sim.EFTExport,
		Fit:              sim.Fit,
		EstimatedCostISK: sim.TotalFitCostISK,
		EHP:              sim.TotalEHP,
		DPS:              sim.Offense.TotalDPS,
	}
	return s.tacticalRepo.SaveFitting(ctx, rec)
}

// ListTacticalFittings returns saved fittings for the active user.
func (s *CopilotService) ListTacticalFittings(ctx context.Context) ([]domain.TacticalFittingRecord, error) {
	if s.tacticalRepo == nil {
		return []domain.TacticalFittingRecord{}, nil
	}
	mode := s.GetModeInfo(ctx)
	var uid int64
	if mode.CurrentUser != nil {
		uid = mode.CurrentUser.ID
	}
	return s.tacticalRepo.ListFittings(ctx, uid, 50)
}

// DeleteTacticalFitting removes a saved fitting by ID.
func (s *CopilotService) DeleteTacticalFitting(ctx context.Context, id int64) error {
	if s.tacticalRepo == nil {
		return fmt.Errorf("tactical repository not configured")
	}
	mode := s.GetModeInfo(ctx)
	var uid int64
	if mode.CurrentUser != nil {
		uid = mode.CurrentUser.ID
	}
	return s.tacticalRepo.DeleteFitting(ctx, id, uid)
}
