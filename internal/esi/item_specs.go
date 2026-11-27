package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"aura-forge/internal/domain"
)

type esiUniverseTypeFull struct {
	TypeID          int32   `json:"type_id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	GroupID         int32   `json:"group_id"`
	Mass            float64 `json:"mass"`
	Volume          float64 `json:"volume"`
	PackagedVolume  float64 `json:"packaged_volume"`
	Capacity        float64 `json:"capacity"`
	DogmaAttributes []struct {
		AttributeID int32   `json:"attribute_id"`
		Value       float64 `json:"value"`
	} `json:"dogma_attributes"`
}

type esiUniverseGroupMeta struct {
	GroupID    int32  `json:"group_id"`
	Name       string `json:"name"`
	CategoryID int32  `json:"category_id"`
}

// GetItemSpecs fetches full technical Dogma attributes, slot layout, resistance profile,
// description, and required skills for any EVE ship, module, drone, or rig (cached in esi_cache).
func (p *PublicMarketClient) GetItemSpecs(ctx context.Context, typeID int32) (*domain.ItemSpecDetail, error) {
	if typeID <= 0 {
		return nil, fmt.Errorf("invalid type_id")
	}

	path := fmt.Sprintf("/universe/types/%d/?datasource=tranquility", typeID)
	body, err := p.client.DoGet(ctx, path, "", true)
	if err != nil {
		return nil, fmt.Errorf("fetching item specs for type %d: %w", typeID, err)
	}

	var raw esiUniverseTypeFull
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding item specs for type %d: %w", typeID, err)
	}

	attrs := make(map[int32]float64, len(raw.DogmaAttributes))
	for _, a := range raw.DogmaAttributes {
		attrs[a.AttributeID] = a.Value
	}

	// EVE Dogma stores resistances as damage resonance (e.g. 0.60 resonance = 40% resistance).
	resPct := func(attrID int32) float64 {
		v, ok := attrs[attrID]
		if !ok || v <= 0 || v > 1.0 {
			return 0
		}
		return math.Round((1.0-v)*1000) / 10
	}

	detail := &domain.ItemSpecDetail{
		TypeID:         raw.TypeID,
		Name:           raw.Name,
		Description:    domain.StripEVEHTML(raw.Description),
		TechLevel:      int(attrs[422]),
		MetaLevel:      int(attrs[633]),
		Mass:           raw.Mass,
		Volume:         raw.Volume,
		PackagedVolume: raw.PackagedVolume,
		Capacity:       raw.Capacity,
		Slots: domain.SlotLayout{
			HighSlots:     int(attrs[14]),
			MidSlots:      int(attrs[13]),
			LowSlots:      int(attrs[12]),
			RigSlots:      int(attrs[1137]),
			TurretSlots:   int(attrs[102]),
			LauncherSlots: int(attrs[101]),
			Subsystems:    int(attrs[1367]),
		},
		ShieldResists: domain.ResistanceProfile{
			EM:        resPct(271),
			Thermal:   resPct(274),
			Kinetic:   resPct(273),
			Explosive: resPct(272),
		},
		ArmorResists: domain.ResistanceProfile{
			EM:        resPct(267),
			Thermal:   resPct(270),
			Kinetic:   resPct(269),
			Explosive: resPct(268),
		},
		Attributes: attrs,
	}
	if detail.TechLevel <= 0 {
		detail.TechLevel = 1
	}

	// Resolve group name + category (category_id == 6 is Ship) & required skill names concurrently
	skillPairs := [][2]int32{
		{182, 277},
		{183, 278},
		{184, 279},
		{1285, 1286},
	}
	reqSkills := make([]domain.RequiredSkill, 0, 4)
	for _, pair := range skillPairs {
		sid := int32(attrs[pair[0]])
		lvl := int(attrs[pair[1]])
		if sid > 0 && lvl > 0 {
			reqSkills = append(reqSkills, domain.RequiredSkill{
				SkillTypeID: sid,
				SkillName:   fmt.Sprintf("Skill #%d", sid),
				Level:       lvl,
			})
		}
	}

	var wg sync.WaitGroup
	if raw.GroupID > 0 {
		wg.Add(1)
		go func(gid int32) {
			defer wg.Done()
			gBody, gErr := p.client.DoGet(ctx, fmt.Sprintf("/universe/groups/%d/?datasource=tranquility", gid), "", true)
			if gErr != nil {
				return
			}
			var gm esiUniverseGroupMeta
			if json.Unmarshal(gBody, &gm) == nil {
				detail.GroupName = gm.Name
				detail.IsShip = gm.CategoryID == 6
			}
		}(raw.GroupID)
	}

	for idx := range reqSkills {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sBody, sErr := p.client.DoGet(ctx, fmt.Sprintf("/universe/types/%d/?datasource=tranquility", reqSkills[i].SkillTypeID), "", true)
			if sErr != nil {
				return
			}
			var sm esiTypeNameResp
			if json.Unmarshal(sBody, &sm) == nil && sm.Name != "" {
				reqSkills[i].SkillName = sm.Name
			}
		}(idx)
	}
	wg.Wait()

	if !detail.IsShip && detail.Slots.HasSlots() {
		detail.IsShip = true
	}
	detail.RequiredSkills = reqSkills

	return detail, nil
}
