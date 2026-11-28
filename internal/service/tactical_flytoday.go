package service

import (
	"context"
	"strings"
	"sync"

	"aura-forge/internal/domain"
)

// FlyTodaySwapRecord documents one Tech 2 -> Beginner Meta/T1 downgrade applied by "Fly Today".
type FlyTodaySwapRecord struct {
	SlotKind    domain.FitSlotKind `json:"slot_kind"`
	FromModule  string             `json:"from_module"`
	ToModule    string             `json:"to_module"`
	FromTypeID  int32              `json:"from_type_id"`
	ToTypeID    int32              `json:"to_type_id"`
}

// FlyTodayResult bundles the beginner-adapted ShipFit simulation and savings summary.
type FlyTodayResult struct {
	Simulation   *domain.FitSimulationResult `json:"simulation"`
	SwappedCount int                         `json:"swapped_count"`
	ISKSaved     float64                     `json:"isk_saved"`
	Swaps        []FlyTodaySwapRecord        `json:"swaps"`
}

// AdaptFitToFlyToday converts Tech 2 ('II') modules on a ShipFit into beginner-accessible
// Compact / Meta / Tech 1 equivalents using local SDE lookups (0 ESI calls) and re-simulates.
func (s *CopilotService) AdaptFitToFlyToday(ctx context.Context, fit domain.ShipFit) (*FlyTodayResult, error) {
	origSim, err := s.SimulateTacticalFit(ctx, fit)
	if err != nil {
		return nil, err
	}

	adapted := origSim.Fit
	adapted.Modules = append([]domain.FitModuleSlot(nil), origSim.Fit.Modules...)
	adapted.SkillProfile = domain.SkillProfileBeginner

	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		swaps = make([]FlyTodaySwapRecord, 0)
	)

	for i := range adapted.Modules {
		m := adapted.Modules[i]
		if !strings.HasSuffix(strings.TrimSpace(m.Name), " II") {
			continue
		}
		wg.Add(1)
		go func(idx int, mod domain.FitModuleSlot) {
			defer wg.Done()
			base := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(mod.Name), " II"))
			if base == "" {
				return
			}
			// Search local SDE for Compact / Enduring / Meta or Tech 1 equivalent
			candidates := s.SearchTacticalModules(ctx, string(mod.SlotKind), base, 15)
			var chosen *domain.SDEType
			// Priority: Compact -> Enduring -> scoped/restrained -> Tech 1 (" I")
			for _, pref := range []string{"compact", "enduring", "restrained", "scoped", " i"} {
				for j := range candidates {
					c := candidates[j]
					low := strings.ToLower(c.Name)
					if c.TypeID == mod.TypeID || strings.HasSuffix(low, " ii") || strings.Contains(low, "abyssal") {
						continue
					}
					if pref == " i" {
						if strings.EqualFold(c.Name, base+" I") {
							chosen = &candidates[j]
							break
						}
					} else if strings.Contains(low, pref) {
						chosen = &candidates[j]
						break
					}
				}
				if chosen != nil {
					break
				}
			}
			if chosen == nil {
				// Fallback: any non-II candidate matching base name
				for j := range candidates {
					c := candidates[j]
					if c.TypeID != mod.TypeID && !strings.HasSuffix(strings.ToLower(c.Name), " ii") {
						chosen = &candidates[j]
						break
					}
				}
			}
			if chosen == nil {
				return
			}

			mu.Lock()
			adapted.Modules[idx].TypeID = chosen.TypeID
			adapted.Modules[idx].Name = chosen.Name
			adapted.Modules[idx].GroupName = chosen.GroupName
			swaps = append(swaps, FlyTodaySwapRecord{
				SlotKind:   mod.SlotKind,
				FromModule: mod.Name,
				ToModule:   chosen.Name,
				FromTypeID: mod.TypeID,
				ToTypeID:   chosen.TypeID,
			})
			mu.Unlock()
		}(i, m)
	}
	wg.Wait()

	newSim, err := s.SimulateTacticalFit(ctx, adapted)
	if err != nil {
		return nil, err
	}
	iskSaved := domain.RoundISK(origSim.TotalFitCostISK - newSim.TotalFitCostISK)
	if iskSaved < 0 {
		iskSaved = 0
	}

	return &FlyTodayResult{
		Simulation:   newSim,
		SwappedCount: len(swaps),
		ISKSaved:     iskSaved,
		Swaps:        swaps,
	}, nil
}
