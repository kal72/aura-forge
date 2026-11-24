package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

// PilotHangarItem aggregates multiple asset stacks of the same TypeID for clean display.
type PilotHangarItem struct {
	TypeID     int32  `json:"type_id"`
	TypeName   string `json:"type_name"`
	TotalQty   int64  `json:"total_qty"`
	StackCount int    `json:"stack_count"`
}

// PilotHubViewData bundles all live EVE SSO character data for the dedicated /pilot hub
// with search, type filtering, and pagination across all 4 tabs.
type PilotHubViewData struct {
	ActiveTab        string                      `json:"active_tab"`
	SearchQuery      string                      `json:"search_query"`
	TypeFilter       string                      `json:"type_filter"`
	Page             int                         `json:"page"`
	PageSize         int                         `json:"page_size"`
	TotalFiltered    int                         `json:"total_filtered"`
	TotalPages       int                         `json:"total_pages"`
	StartIdx         int                         `json:"start_idx"`
	EndIdx           int                         `json:"end_idx"`
	Profile          *domain.CharacterProfile    `json:"profile,omitempty"`
	Settings         domain.UserSettings         `json:"settings"`
	ActiveHub        domain.TradeHub             `json:"active_hub"`
	AvailableCapital float64                     `json:"available_capital"`
	CapitalSource    string                      `json:"capital_source"`
	TotalJobs        int                         `json:"total_jobs"`
	TotalOrders      int                         `json:"total_orders"`
	TotalBlueprints  int                         `json:"total_blueprints"`
	TotalHangarItems int                         `json:"total_hangar_items"`
	Jobs             []domain.CharacterJob       `json:"jobs"`
	Orders           []domain.CharacterOrder     `json:"orders"`
	Blueprints       []domain.CharacterBlueprint `json:"blueprints"`
	HangarItems      []PilotHangarItem           `json:"hangar_items"`
	TotalSellISK     float64                     `json:"total_sell_isk"`
	TotalBuyISK      float64                     `json:"total_buy_isk"`
	ReadyJobsCount   int                         `json:"ready_jobs_count"`
}

// GetPilotHubView loads the connected character's profile, industry jobs, open orders, blueprints, and hangar stock
// with search, category filter, and pagination.
func (s *CopilotService) GetPilotHubView(ctx context.Context, activeTab, searchQuery, typeFilter string, page int) (*PilotHubViewData, error) {
	switch strings.ToLower(strings.TrimSpace(activeTab)) {
	case "orders", "blueprints", "assets":
		activeTab = strings.ToLower(strings.TrimSpace(activeTab))
	default:
		activeTab = "jobs"
	}
	if page < 1 {
		page = 1
	}
	const pageSize = 25

	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	prof, availCap, capSource := s.resolveProfileAndCapital(ctx, *settings)

	view := &PilotHubViewData{
		ActiveTab:        activeTab,
		SearchQuery:      strings.TrimSpace(searchQuery),
		TypeFilter:       strings.ToUpper(strings.TrimSpace(typeFilter)),
		Page:             page,
		PageSize:         pageSize,
		Settings:         *settings,
		ActiveHub:        settings.ActiveTradeHub(),
		AvailableCapital: availCap,
		CapitalSource:    capSource,
	}

	mode := s.GetModeInfo(ctx)
	if !mode.SSOConnected || s.chars == nil {
		return view, nil
	}

	charID := mode.CharacterID
	if prof != nil {
		view.Profile = prof
	} else {
		view.Profile = &domain.CharacterProfile{
			CharacterID:             charID,
			CharacterName:           mode.CharacterName,
			TotalManufacturingSlots: settings.ManufacturingSlots,
			TotalScienceSlots:       settings.ScienceSlots,
		}
	}

	jobs, _ := s.chars.ListJobs(ctx, charID)
	orders, _ := s.chars.ListOrders(ctx, charID)
	bps, _ := s.chars.ListBlueprints(ctx, charID)
	assets, _ := s.chars.ListAssets(ctx, charID)

	// Instant local PostgreSQL SDE resolution only (0ms, zero network ESI calls during tab/page navigation)
	s.resolveCharacterTypeNamesLocalOnly(ctx, orders, jobs, bps, assets)

	now := time.Now()
	for _, j := range jobs {
		if j.Status == "ready" || (!j.EndDate.IsZero() && j.EndDate.Before(now) && j.Status == "active") {
			view.ReadyJobsCount++
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].EndDate.Before(jobs[j].EndDate)
	})
	view.TotalJobs = len(jobs)

	for _, o := range orders {
		val := o.Price * float64(o.VolumeRemain)
		if o.IsBuyOrder {
			view.TotalBuyISK += val
		} else {
			view.TotalSellISK += val
		}
	}
	view.TotalSellISK = domain.RoundISK(view.TotalSellISK)
	view.TotalBuyISK = domain.RoundISK(view.TotalBuyISK)
	view.TotalOrders = len(orders)

	sort.Slice(bps, func(i, j int) bool {
		if bps[i].IsCopy != bps[j].IsCopy {
			return !bps[i].IsCopy
		}
		if bps[i].MaterialEfficiency != bps[j].MaterialEfficiency {
			return bps[i].MaterialEfficiency > bps[j].MaterialEfficiency
		}
		return bps[i].TypeName < bps[j].TypeName
	})
	view.TotalBlueprints = len(bps)

	byType := make(map[int32]*PilotHangarItem)
	for _, a := range assets {
		if a.Quantity <= 0 {
			continue
		}
		if cur, ok := byType[a.TypeID]; ok {
			cur.TotalQty += a.Quantity
			cur.StackCount++
		} else {
			byType[a.TypeID] = &PilotHangarItem{
				TypeID:     a.TypeID,
				TypeName:   a.TypeName,
				TotalQty:   a.Quantity,
				StackCount: 1,
			}
		}
	}
	hangar := make([]PilotHangarItem, 0, len(byType))
	for _, it := range byType {
		hangar = append(hangar, *it)
	}
	sort.Slice(hangar, func(i, j int) bool {
		return hangar[i].TotalQty > hangar[j].TotalQty
	})
	view.TotalHangarItems = len(hangar)

	qLower := strings.ToLower(view.SearchQuery)
	tf := view.TypeFilter

	paginate := func(total int) (start, end int) {
		view.TotalFiltered = total
		if total == 0 {
			view.TotalPages = 1
			view.Page = 1
			return 0, 0
		}
		view.TotalPages = int(math.Ceil(float64(total) / float64(pageSize)))
		if view.Page > view.TotalPages {
			view.Page = view.TotalPages
		}
		start = (view.Page - 1) * pageSize
		end = start + pageSize
		if end > total {
			end = total
		}
		view.StartIdx = start + 1
		view.EndIdx = end
		return start, end
	}

	switch activeTab {
	case "jobs":
		filtered := make([]domain.CharacterJob, 0, len(jobs))
		for _, j := range jobs {
			if tf == "MFG" && j.ActivityID != 1 {
				continue
			}
			if tf == "RESEARCH" && j.ActivityID == 1 {
				continue
			}
			if qLower != "" && !strings.Contains(strings.ToLower(j.TypeName), qLower) {
				continue
			}
			filtered = append(filtered, j)
		}
		sIdx, eIdx := paginate(len(filtered))
		view.Jobs = filtered[sIdx:eIdx]

	case "orders":
		filtered := make([]domain.CharacterOrder, 0, len(orders))
		for _, o := range orders {
			if tf == "BUY" && !o.IsBuyOrder {
				continue
			}
			if tf == "SELL" && o.IsBuyOrder {
				continue
			}
			if qLower != "" && !strings.Contains(strings.ToLower(o.TypeName), qLower) {
				continue
			}
			filtered = append(filtered, o)
		}
		sIdx, eIdx := paginate(len(filtered))
		view.Orders = filtered[sIdx:eIdx]

	case "blueprints":
		filtered := make([]domain.CharacterBlueprint, 0, len(bps))
		for _, bp := range bps {
			if tf == "BPO" && bp.IsCopy {
				continue
			}
			if tf == "BPC" && !bp.IsCopy {
				continue
			}
			if qLower != "" && !strings.Contains(strings.ToLower(bp.TypeName), qLower) {
				continue
			}
			filtered = append(filtered, bp)
		}
		sIdx, eIdx := paginate(len(filtered))
		view.Blueprints = filtered[sIdx:eIdx]

	case "assets":
		filtered := make([]PilotHangarItem, 0, len(hangar))
		for _, it := range hangar {
			if qLower != "" && !strings.Contains(strings.ToLower(it.TypeName), qLower) {
				continue
			}
			filtered = append(filtered, it)
		}
		sIdx, eIdx := paginate(len(filtered))
		view.HangarItems = filtered[sIdx:eIdx]
	}

	return view, nil
}

// SyncWorkingCapitalFromWallet explicitly copies the connected character's live EVE Wallet Balance
// into Settings.StartingCapital.
func (s *CopilotService) SyncWorkingCapitalFromWallet(ctx context.Context) (float64, error) {
	mode := s.GetModeInfo(ctx)
	if !mode.SSOConnected || s.chars == nil {
		return 0, fmt.Errorf("EVE SSO is not connected")
	}
	prof, err := s.chars.GetProfile(ctx, mode.CharacterID)
	if err != nil || prof == nil || prof.WalletBalance <= 0 {
		return 0, fmt.Errorf("no synced wallet balance available — click Sync Now first")
	}
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return 0, err
	}
	cfg.StartingCapital = domain.RoundISK(prof.WalletBalance)
	if err := s.UpdateSettings(ctx, cfg); err != nil {
		return 0, err
	}
	return cfg.StartingCapital, nil
}

// resolveCharacterTypeNamesLocalOnly resolves placeholder names strictly from local PostgreSQL SDE (0ms, no network calls).
func (s *CopilotService) resolveCharacterTypeNamesLocalOnly(
	ctx context.Context,
	orders []domain.CharacterOrder,
	jobs []domain.CharacterJob,
	bps []domain.CharacterBlueprint,
	assets []domain.HangarAsset,
) {
	s.resolveCharacterTypeNamesWithMode(ctx, orders, jobs, bps, assets, false)
}

// resolveCharacterTypeNames resolves placeholder "Type #..." / "Blueprint #..." names from
// local PostgreSQL SDE first (0ms, 100% reliable), then falls back to ESI /universe/names/ in safe batches of 50.
func (s *CopilotService) resolveCharacterTypeNames(
	ctx context.Context,
	orders []domain.CharacterOrder,
	jobs []domain.CharacterJob,
	bps []domain.CharacterBlueprint,
	assets []domain.HangarAsset,
) {
	s.resolveCharacterTypeNamesWithMode(ctx, orders, jobs, bps, assets, true)
}

func (s *CopilotService) resolveCharacterTypeNamesWithMode(
	ctx context.Context,
	orders []domain.CharacterOrder,
	jobs []domain.CharacterJob,
	bps []domain.CharacterBlueprint,
	assets []domain.HangarAsset,
	allowESIFallback bool,
) {
	isPlaceholder := func(name string) bool {
		return name == "" || strings.HasPrefix(name, "Type #") || strings.HasPrefix(name, "Blueprint #")
	}

	needed := make(map[int32]struct{})
	for _, o := range orders {
		if o.TypeID > 0 && isPlaceholder(o.TypeName) {
			needed[o.TypeID] = struct{}{}
		}
	}
	for _, j := range jobs {
		if j.TypeID > 0 && isPlaceholder(j.TypeName) {
			needed[j.TypeID] = struct{}{}
		}
	}
	for _, bp := range bps {
		if bp.TypeID > 0 && isPlaceholder(bp.TypeName) {
			needed[bp.TypeID] = struct{}{}
		}
	}
	for _, a := range assets {
		if a.TypeID > 0 && isPlaceholder(a.TypeName) {
			needed[a.TypeID] = struct{}{}
		}
	}
	if len(needed) == 0 {
		return
	}

	ids := make([]int32, 0, len(needed))
	for id := range needed {
		ids = append(ids, id)
	}

	resolved := make(map[int32]string, len(ids))

	// 1. Instant local PostgreSQL SDE resolution (sde_types + sde_blueprints)
	if s.sde != nil {
		if resolver, ok := s.sde.(domain.SDETypeNameResolver); ok {
			if localNames, err := resolver.GetTypeNamesByIDs(ctx, ids); err == nil {
				for k, v := range localNames {
					resolved[k] = v
				}
			}
		}
	}

	// 2. Fallback to Public ESI /universe/names/ in safe batches of 50 for any IDs not in local SDE
	var remaining []int32
	for _, id := range ids {
		if _, ok := resolved[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	if allowESIFallback && len(remaining) > 0 {
		if gp, ok := s.market.(domain.GlobalMarketPriceProvider); ok && gp != nil {
			for start := 0; start < len(remaining); start += 50 {
				end := start + 50
				if end > len(remaining) {
					end = len(remaining)
				}
				esiCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
				if names, err := gp.ResolveTypeNames(esiCtx, remaining[start:end]); err == nil {
					for k, v := range names {
						resolved[k] = v
					}
				}
				cancel()
			}
		}
	}

	if len(resolved) == 0 {
		return
	}

	for i := range orders {
		if name, ok := resolved[orders[i].TypeID]; ok && name != "" {
			orders[i].TypeName = name
		}
	}
	for i := range jobs {
		if name, ok := resolved[jobs[i].TypeID]; ok && name != "" {
			jobs[i].TypeName = name
		}
	}
	for i := range bps {
		if name, ok := resolved[bps[i].TypeID]; ok && name != "" {
			bps[i].TypeName = name
		}
	}
	for i := range assets {
		if name, ok := resolved[assets[i].TypeID]; ok && name != "" {
			assets[i].TypeName = name
		}
	}
}
