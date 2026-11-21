package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
)

// SSOAuthenticator abstracts optional OAuth2 login/callback operations.
type SSOAuthenticator interface {
	IsEnabled() bool
	AuthURL(state string) (string, error)
	ExchangeCode(ctx context.Context, code string) (*domain.EVEToken, error)
}

// ModeInfo describes the current operational mode (Public ESI Standard vs EVE SSO Connected)
// along with the currently authenticated application user and background Jita price sync status.
type ModeInfo struct {
	SSOEnabled    bool            `json:"sso_enabled"`
	SSOConnected  bool            `json:"sso_connected"`
	CharacterID   int64           `json:"character_id"`
	CharacterName string          `json:"character_name"`
	ModeLabel     string          `json:"mode_label"`
	StatusHint    string          `json:"status_hint"`
	CurrentUser   *domain.AppUser `json:"current_user,omitempty"`
	PriceSync     PriceSyncStatus `json:"price_sync"`
}

// DashboardSummary aggregates top-level KPIs and daily assistant tasks.
type DashboardSummary struct {
	Mode                 ModeInfo           `json:"mode"`
	AvailableCapital     float64            `json:"available_capital"`
	CapitalSource        string             `json:"capital_source"`
	LockedCapital        float64            `json:"locked_capital"`
	RealizedProfit       float64            `json:"realized_profit"`
	MfgSlotsTotal        int                `json:"mfg_slots_total"`
	MfgSlotsUsed         int                `json:"mfg_slots_used"`
	SciSlotsTotal        int                `json:"sci_slots_total"`
	SciSlotsUsed         int                `json:"sci_slots_used"`
	TotalCatalogItems    int                `json:"total_catalog_items"`
	LayakItemsCount      int                `json:"layak_items_count"`
	JebakanItemsCount    int                `json:"jebakan_items_count"`
	ActiveBatchesCount   int                `json:"active_batches_count"`
	UndercutBatchesCount int                `json:"undercut_batches_count"`
	ActivePlan           *domain.WeeklyPlan `json:"active_plan,omitempty"`
	DailyChecklist       []string           `json:"daily_checklist"`
}

// MineralQuote holds a live Jita Public ESI price snapshot for a core industrial mineral.
type MineralQuote struct {
	TypeID     int32   `json:"type_id"`
	Name       string  `json:"name"`
	LowestSell float64 `json:"lowest_sell"`
	HighestBuy float64 `json:"highest_buy"`
	IsLive     bool    `json:"is_live"`
}

// PlannerViewData bundles all data required to render the Weekly Production & Slot Planner.
type PlannerViewData struct {
	ActivePlan   *domain.WeeklyPlan              `json:"active_plan"`
	AllPlans     []domain.WeeklyPlan             `json:"all_plans"`
	SlotReport   domain.SlotCapacityReport       `json:"slot_report"`
	ShoppingList domain.ConsolidatedShoppingList `json:"shopping_list"`
	Growth4Weeks []domain.GrowthProjection       `json:"growth_4_weeks"`
	Settings     domain.UserSettings             `json:"settings"`
}

// CopilotService orchestrates all domain use-cases across Standard Mode and Optional SSO Mode.
type CopilotService struct {
	items     domain.ItemRepository
	batches   domain.BatchRepository
	plans     domain.PlanRepository
	chars     domain.CharacterRepository
	settings  domain.SettingsRepository
	market    domain.MarketDataProvider
	charSync  domain.CharacterSyncProvider
	sso       SSOAuthenticator
	sde       domain.SDERepository
	wealth    domain.NetWorthRepository
	pi        domain.PIRepository
	priceSync *priceSyncState
}

// NewCopilotService constructs a CopilotService via Dependency Inversion interfaces.
func NewCopilotService(
	items domain.ItemRepository,
	batches domain.BatchRepository,
	plans domain.PlanRepository,
	chars domain.CharacterRepository,
	settings domain.SettingsRepository,
	market domain.MarketDataProvider,
	charSync domain.CharacterSyncProvider,
	sso SSOAuthenticator,
) *CopilotService {
	return &CopilotService{
		items:     items,
		batches:   batches,
		plans:     plans,
		chars:     chars,
		settings:  settings,
		market:    market,
		charSync:  charSync,
		sso:       sso,
		priceSync: newPriceSyncState(),
	}
}

// GetModeInfo inspects SSO config, stored character tokens, and authenticated user context.
func (s *CopilotService) GetModeInfo(ctx context.Context) ModeInfo {
	currentUser := domain.AppUserFromContext(ctx)
	priceSync := s.GetPriceSyncStatus(ctx)
	ssoEnabled := s.sso != nil && s.sso.IsEnabled()
	if !ssoEnabled {
		return ModeInfo{
			SSOEnabled:  false,
			ModeLabel:   "Public ESI Active",
			StatusHint:  "Optional: Set ESI_CLIENT_ID & SECRET in .env for Character Auto-Sync",
			CurrentUser: currentUser,
			PriceSync:   priceSync,
		}
	}

	if s.chars != nil {
		if tok, err := s.chars.GetPrimaryToken(ctx); err == nil && tok != nil {
			return ModeInfo{
				SSOEnabled:    true,
				SSOConnected:  true,
				CharacterID:   tok.CharacterID,
				CharacterName: tok.CharacterName,
				ModeLabel:     "EVE SSO Connected",
				StatusHint:    fmt.Sprintf("Connected as %s", tok.CharacterName),
				CurrentUser:   currentUser,
				PriceSync:     priceSync,
			}
		}
	}

	return ModeInfo{
		SSOEnabled:   true,
		SSOConnected: false,
		ModeLabel:    "Public ESI Active",
		StatusHint:   "EVE SSO Ready: Click Connect EVE SSO to synchronize your character",
		CurrentUser:  currentUser,
		PriceSync:    priceSync,
	}
}

// GetSSOAuthURL returns the OAuth2 login URL or ErrSSODisabled.
func (s *CopilotService) GetSSOAuthURL(state string) (string, error) {
	if s.sso == nil || !s.sso.IsEnabled() {
		return "", esi.ErrSSODisabled
	}
	return s.sso.AuthURL(state)
}

// HandleSSOCallback exchanges the OAuth2 code, persists the token, and triggers initial character sync.
func (s *CopilotService) HandleSSOCallback(ctx context.Context, code string) (*domain.EVEToken, error) {
	if s.sso == nil || !s.sso.IsEnabled() {
		return nil, esi.ErrSSODisabled
	}
	tok, err := s.sso.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if s.chars != nil {
		if err := s.chars.SaveToken(ctx, tok); err != nil {
			return nil, fmt.Errorf("saving EVE token: %w", err)
		}
		_, _ = s.SyncCharacterSSO(ctx)
	}
	return tok, nil
}

// DisconnectSSO removes the primary character token.
func (s *CopilotService) DisconnectSSO(ctx context.Context) error {
	if s.chars == nil {
		return nil
	}
	tok, err := s.chars.GetPrimaryToken(ctx)
	if err != nil || tok == nil {
		return err
	}
	return s.chars.DeleteToken(ctx, tok.CharacterID)
}

// GetSettings returns user settings or defaults if repository is empty.
func (s *CopilotService) GetSettings(ctx context.Context) (*domain.UserSettings, error) {
	if s.settings == nil {
		d := domain.DefaultUserSettings()
		return &d, nil
	}
	return s.settings.GetSettings(ctx)
}

// UpdateSettings persists user settings and re-evaluates catalog trap statuses.
func (s *CopilotService) UpdateSettings(ctx context.Context, cfg *domain.UserSettings) error {
	if cfg == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	if cfg.ManufacturingSlots <= 0 {
		cfg.ManufacturingSlots = 1
	}
	if cfg.ScienceSlots < 0 {
		cfg.ScienceSlots = 0
	}
	cfg.AutoPriceSyncIntervalMinutes = domain.ClampPriceSyncInterval(cfg.AutoPriceSyncIntervalMinutes)
	if s.settings != nil {
		if err := s.settings.SaveSettings(ctx, cfg); err != nil {
			return err
		}
	}
	s.priceSync.notifyConfigChanged()
	return nil
}

func (s *CopilotService) resolveProfileAndCapital(ctx context.Context, settings domain.UserSettings) (*domain.CharacterProfile, float64, string) {
	mode := s.GetModeInfo(ctx)
	var profile *domain.CharacterProfile
	if mode.SSOConnected && s.chars != nil {
		if p, err := s.chars.GetProfile(ctx, mode.CharacterID); err == nil {
			profile = p
		}
	}
	capISK, source := domain.ResolveAvailableCapital(settings, profile, mode.SSOConnected)
	return profile, capISK, source
}

// GetDashboard aggregates live KPIs for the main dashboard view.
func (s *CopilotService) GetDashboard(ctx context.Context) (*DashboardSummary, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	mode := s.GetModeInfo(ctx)
	profile, availCap, capSource := s.resolveProfileAndCapital(ctx, *settings)

	summary := &DashboardSummary{
		Mode:             mode,
		AvailableCapital: availCap,
		CapitalSource:    capSource,
		MfgSlotsTotal:    settings.ManufacturingSlots,
		SciSlotsTotal:    settings.ScienceSlots,
	}

	if profile != nil && profile.TotalManufacturingSlots > 0 {
		summary.MfgSlotsTotal = profile.TotalManufacturingSlots
		summary.MfgSlotsUsed = profile.UsedManufacturingSlots
		summary.SciSlotsTotal = profile.TotalScienceSlots
		summary.SciSlotsUsed = profile.UsedScienceSlots
	}

	if s.items != nil {
		if items, err := s.items.ListItems(ctx); err == nil {
			summary.TotalCatalogItems = len(items)
			for _, it := range items {
				switch it.TrapAnalysis.Status {
				case domain.TrapStatusLayak:
					summary.LayakItemsCount++
				case domain.TrapStatusJebakan:
					summary.JebakanItemsCount++
				}
			}
		}
	}

	if s.batches != nil {
		if batches, err := s.batches.ListBatches(ctx, nil); err == nil {
			for _, b := range batches {
				switch b.Status {
				case domain.BatchStatusInProduction:
					summary.LockedCapital += b.ActualProductionCost
					summary.ActiveBatchesCount++
					if profile == nil {
						summary.MfgSlotsUsed += b.Runs
					}
				case domain.BatchStatusListed, domain.BatchStatusSlowMover:
					summary.LockedCapital += b.ActualProductionCost
					summary.ActiveBatchesCount++
					if b.IsUndercut {
						summary.UndercutBatchesCount++
					}
				case domain.BatchStatusSold:
					summary.RealizedProfit += b.NetProfit
				}
			}
		}
	}

	summary.LockedCapital = domain.RoundISK(summary.LockedCapital)
	summary.RealizedProfit = domain.RoundISK(summary.RealizedProfit)

	if s.plans != nil {
		if activePlan, err := s.plans.GetActivePlan(ctx); err == nil && activePlan != nil {
			summary.ActivePlan = activePlan
		}
	}

	summary.DailyChecklist = buildDailyChecklist(summary)
	return summary, nil
}

func buildDailyChecklist(d *DashboardSummary) []string {
	list := make([]string, 0, 5)
	if d.TotalCatalogItems == 0 {
		list = append(list, "Run the Opportunity Scanner or click 'Load Sample Data' in Catalog to start discovering profitable items.")
	} else {
		list = append(list, "Click 'Sync Jita Prices' to refresh live Sell/Buy prices and daily volumes from Public ESI.")
	}
	if d.UndercutBatchesCount > 0 {
		list = append(list, fmt.Sprintf("ALERT: %d listed batch(es) are currently undercut in Jita — verify Safe Relist Price in Tracker.", d.UndercutBatchesCount))
	}
	if d.MfgSlotsUsed < d.MfgSlotsTotal {
		list = append(list, fmt.Sprintf("%d manufacturing slot(s) are idle — start batches from Weekly Planner to keep ISK compounding.", d.MfgSlotsTotal-d.MfgSlotsUsed))
	}
	if d.ActivePlan == nil {
		list = append(list, "Run Smart Capital Allocator and send recommendations to Weekly Production Planner.")
	} else {
		list = append(list, fmt.Sprintf("Follow your Day 1–7 schedule on active plan: %s.", d.ActivePlan.Title))
	}
	return list
}

// ListCatalog returns catalog items with optional category, status, and search filtering.
func (s *CopilotService) ListCatalog(ctx context.Context, categoryFilter, statusFilter, search string) ([]domain.MarketItem, error) {
	if s.items == nil {
		return []domain.MarketItem{}, nil
	}
	all, err := s.items.ListItems(ctx)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(search))
	filtered := make([]domain.MarketItem, 0, len(all))
	for _, it := range all {
		if categoryFilter != "" && string(it.Category) != categoryFilter {
			continue
		}
		if statusFilter != "" && string(it.TrapAnalysis.Status) != statusFilter {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(it.Name), q) {
			continue
		}
		filtered = append(filtered, it)
	}
	return filtered, nil
}

// SaveCatalogItem evaluates TrapGuard and saves a MarketItem.
func (s *CopilotService) SaveCatalogItem(ctx context.Context, item *domain.MarketItem) (*domain.MarketItem, error) {
	if item == nil || strings.TrimSpace(item.Name) == "" {
		return nil, fmt.Errorf("item name is required")
	}
	if item.BatchOutputQty <= 0 {
		item.BatchOutputQty = 1
	}

	// Attempt Public ESI type_id resolution if missing
	if item.TypeID == 0 && s.market != nil {
		if resolved, err := s.market.ResolveTypeIDs(ctx, []string{item.Name}); err == nil {
			if tid, ok := resolved[item.Name]; ok {
				item.TypeID = tid
			}
		}
	}

	settings, _ := s.GetSettings(ctx)
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)
	item.TrapAnalysis = domain.EvaluateTrapGuard(*item, *settings, availCap)

	if s.items != nil {
		if err := s.items.SaveItem(ctx, item); err != nil {
			return nil, err
		}
	}
	return item, nil
}

// DeleteCatalogItem removes an item by ID.
func (s *CopilotService) DeleteCatalogItem(ctx context.Context, id int64) error {
	if s.items == nil {
		return nil
	}
	return s.items.DeleteItem(ctx, id)
}

// SyncJitaPrices refreshes live Jita prices, 7d/30d daily volume, and TrapGuard for all catalog items using Public ESI.
func (s *CopilotService) SyncJitaPrices(ctx context.Context) (int, error) {
	if s.items == nil || s.market == nil {
		return 0, fmt.Errorf("market provider or item repository unavailable")
	}

	items, err := s.items.ListItems(ctx)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	// 1. Resolve missing TypeIDs
	var missingNames []string
	for _, it := range items {
		if it.TypeID == 0 {
			missingNames = append(missingNames, it.Name)
		}
	}
	if len(missingNames) > 0 {
		if resolved, err := s.market.ResolveTypeIDs(ctx, missingNames); err == nil {
			for i := range items {
				if items[i].TypeID == 0 {
					if tid, ok := resolved[items[i].Name]; ok {
						items[i].TypeID = tid
					}
				}
			}
		}
	}

	// 2. Collect valid TypeIDs
	typeIDs := make([]int32, 0, len(items))
	for _, it := range items {
		if it.TypeID > 0 {
			typeIDs = append(typeIDs, it.TypeID)
		}
	}

	prices, err := s.market.GetJitaPrices(ctx, typeIDs)
	if err != nil {
		return 0, fmt.Errorf("fetching Jita prices: %w", err)
	}

	settings, _ := s.GetSettings(ctx)
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)

	updatedCount := 0
	for i := range items {
		if items[i].TypeID <= 0 {
			continue
		}
		if jp, ok := prices[items[i].TypeID]; ok {
			if jp.LowestSell > 0 {
				items[i].EstimatedJitaSell = jp.LowestSell
			}
			if jp.HighestBuy > 0 {
				items[i].EstimatedJitaBuy = jp.HighestBuy
			}
			updatedCount++
		}
		if avg7, avg30, vErr := s.market.GetDailyVolume(ctx, items[i].TypeID); vErr == nil {
			items[i].DailyVolume = avg7
			items[i].Volume30d = avg30
		}
		items[i].TrapAnalysis = domain.EvaluateTrapGuard(items[i], *settings, availCap)
	}

	if err := s.items.BatchUpsertItems(ctx, items); err != nil {
		return updatedCount, fmt.Errorf("saving synced items: %w", err)
	}
	return updatedCount, nil
}

// SyncSystemCostIndex fetches the live manufacturing SCI for a solar system via Public ESI and saves it to settings.
func (s *CopilotService) SyncSystemCostIndex(ctx context.Context, solarSystemID int32) (float64, error) {
	if s.market == nil {
		return 0, fmt.Errorf("market provider unavailable")
	}
	indices, err := s.market.GetSystemCostIndex(ctx, solarSystemID)
	if err != nil {
		return 0, err
	}
	mfgSCI := indices["manufacturing"]
	settings, err := s.GetSettings(ctx)
	if err == nil && settings != nil && mfgSCI > 0 {
		settings.SystemCostIndex = mfgSCI
		_ = s.UpdateSettings(ctx, settings)
	}
	return mfgSCI, nil
}

// ImportBulkItems parses pasted TSV/CSV data, optionally enriches with Public ESI, evaluates TrapGuard, and saves.
func (s *CopilotService) ImportBulkItems(ctx context.Context, rawText string, enrichWithESI bool) (*domain.ImportResult, error) {
	res := domain.ParseBulkTable(rawText)
	if len(res.Items) == 0 {
		return &res, nil
	}

	settings, _ := s.GetSettings(ctx)
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)

	if enrichWithESI && s.market != nil {
		names := make([]string, 0, len(res.Items))
		for _, it := range res.Items {
			names = append(names, it.Name)
		}
		if resolved, err := s.market.ResolveTypeIDs(ctx, names); err == nil {
			var tids []int32
			for i := range res.Items {
				if tid, ok := resolved[res.Items[i].Name]; ok {
					res.Items[i].TypeID = tid
					tids = append(tids, tid)
				}
			}
			if len(tids) > 0 {
				if prices, err := s.market.GetJitaPrices(ctx, tids); err == nil {
					for i := range res.Items {
						if jp, ok := prices[res.Items[i].TypeID]; ok {
							if jp.LowestSell > 0 {
								res.Items[i].EstimatedJitaSell = jp.LowestSell
							}
							if jp.HighestBuy > 0 {
								res.Items[i].EstimatedJitaBuy = jp.HighestBuy
							}
						}
						if avg7, avg30, vErr := s.market.GetDailyVolume(ctx, res.Items[i].TypeID); vErr == nil {
							res.Items[i].DailyVolume = avg7
							res.Items[i].Volume30d = avg30
						}
					}
				}
			}
		}
	}

	for i := range res.Items {
		// Assign synthetic negative TypeID if unresolved so DB unique constraint doesn't collide
		if res.Items[i].TypeID == 0 {
			res.Items[i].TypeID = -int32(time.Now().UnixNano()%1_000_000 + int64(i+1))
		}
		res.Items[i].TrapAnalysis = domain.EvaluateTrapGuard(res.Items[i], *settings, availCap)
	}

	if s.items != nil {
		if err := s.items.BatchUpsertItems(ctx, res.Items); err != nil {
			return &res, fmt.Errorf("persisting imported items: %w", err)
		}
	}

	return &res, nil
}

// LookupItemPriceESI resolves an item name and returns its live Jita price and 7d daily volume.
func (s *CopilotService) LookupItemPriceESI(ctx context.Context, itemName string) (*domain.JitaPrice, float64, error) {
	if s.market == nil {
		return nil, 0, fmt.Errorf("market provider unavailable")
	}
	resolved, err := s.market.ResolveTypeIDs(ctx, []string{itemName})
	if err != nil {
		return nil, 0, err
	}
	tid, ok := resolved[strings.TrimSpace(itemName)]
	if !ok || tid == 0 {
		// Try case-insensitive match
		for k, v := range resolved {
			if strings.EqualFold(k, strings.TrimSpace(itemName)) {
				tid = v
				ok = true
				break
			}
		}
	}
	if !ok || tid == 0 {
		return nil, 0, fmt.Errorf("item %q tidak ditemukan di EVE ESI", itemName)
	}

	prices, err := s.market.GetJitaPrices(ctx, []int32{tid})
	if err != nil {
		return nil, 0, err
	}
	jp := prices[tid]
	avg7, _, _ := s.market.GetDailyVolume(ctx, tid)
	return &jp, avg7, nil
}

// CalculateProfit executes the domain net profit calculator, applying saved user tax defaults when omitted.
func (s *CopilotService) CalculateProfit(ctx context.Context, in domain.ProfitCalcInput) (domain.ProfitCalcResult, error) {
	settings, err := s.GetSettings(ctx)
	if err == nil && settings != nil {
		if in.BrokerFeePercent == 0 {
			in.BrokerFeePercent = settings.BrokerFeePercent
		}
		if in.SalesTaxPercent == 0 {
			in.SalesTaxPercent = settings.SalesTaxPercent
		}
		if in.RelistFeePercent == 0 {
			in.RelistFeePercent = settings.RelistFeePercent
		}
		if in.SystemCostIndex == 0 {
			in.SystemCostIndex = settings.SystemCostIndex
		}
		if in.FacilityTaxPercent == 0 {
			in.FacilityTaxPercent = settings.FacilityTaxPercent
		}
	}
	return domain.CalculateProfit(in)
}

// RunCapitalAllocation computes the 4-bucket smart capital allocation from catalog candidates.
func (s *CopilotService) RunCapitalAllocation(ctx context.Context, cfg domain.AllocationConfig) (*domain.AllocationResult, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	_, availCap, capSource := s.resolveProfileAndCapital(ctx, *settings)

	var candidates []domain.MarketItem
	if s.items != nil {
		candidates, _ = s.items.ListItems(ctx)
	}

	res, err := domain.AllocateCapital(availCap, capSource, cfg, candidates, *settings)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// CreatePlanFromAllocation converts a 4-bucket allocation directly into an active WeeklyPlan.
func (s *CopilotService) CreatePlanFromAllocation(ctx context.Context, title string, weekNum int, cfg domain.AllocationConfig) (*domain.WeeklyPlan, error) {
	alloc, err := s.RunCapitalAllocation(ctx, cfg)
	if err != nil {
		return nil, err
	}
	settings, _ := s.GetSettings(ctx)

	var planItems []domain.PlanItem
	for _, b := range alloc.Buckets {
		if b.Type == domain.BucketCashBuffer {
			continue
		}
		for _, it := range b.Items {
			catItem, _ := s.items.GetItemByTypeID(ctx, it.TypeID)
			outPerBatch := 1
			sellPrice := 0.0
			prodHours := 6.0
			if catItem != nil {
				if catItem.BatchOutputQty > 0 {
					outPerBatch = catItem.BatchOutputQty
				}
				sellPrice = catItem.EstimatedJitaSell
				if catItem.ProductionTimeSeconds > 0 {
					prodHours = float64(catItem.ProductionTimeSeconds) / 3600.0
				}
			}
			totalUnits := it.RecommendedRuns * outPerBatch
			grossRev := domain.RoundISK(sellPrice * float64(totalUnits))
			planItems = append(planItems, domain.PlanItem{
				TypeID:              it.TypeID,
				Name:                it.Name,
				BatchRuns:           it.RecommendedRuns,
				OutputPerBatch:      outPerBatch,
				TotalUnits:          totalUnits,
				UnitSellPrice:       sellPrice,
				TotalMaterialCost:   it.TotalCost,
				TotalCost:           it.TotalCost,
				TotalGrossRevenue:   grossRev,
				NetProfit:           it.EstimatedProfit,
				ProductionTimeHours: prodHours,
			})
		}
	}

	plan := domain.BuildWeeklyPlan(title, weekNum, alloc.TotalCapital, settings.ManufacturingSlots, settings.ScienceSlots, planItems)
	plan.Status = domain.PlanStatusActive

	if s.plans != nil {
		if err := s.plans.SavePlan(ctx, &plan); err != nil {
			return nil, err
		}
	}
	return &plan, nil
}

// CreateDraftBatchesFromAllocation converts allocation recommendations directly into Draft BatchRecords.
func (s *CopilotService) CreateDraftBatchesFromAllocation(ctx context.Context, cfg domain.AllocationConfig) (int, error) {
	plan, err := s.CreatePlanFromAllocation(ctx, "Alokasi Langsung", 1, cfg)
	if err != nil {
		return 0, err
	}
	return s.ConvertPlanToTrackerBatches(ctx, plan.ID)
}

// GetPlannerView builds the complete Weekly Production & Slot Planner payload.
func (s *CopilotService) GetPlannerView(ctx context.Context, planID int64) (*PlannerViewData, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)

	var allPlans []domain.WeeklyPlan
	var selected *domain.WeeklyPlan

	if s.plans != nil {
		allPlans, _ = s.plans.ListPlans(ctx)
		if planID > 0 {
			selected, _ = s.plans.GetPlanByID(ctx, planID)
		} else {
			selected, _ = s.plans.GetActivePlan(ctx)
			if selected == nil && len(allPlans) > 0 {
				p := allPlans[0]
				selected = &p
			}
		}
	}

	if selected == nil {
		empty := domain.BuildWeeklyPlan("Rencana Minggu #1 (Draft)", 1, availCap, settings.ManufacturingSlots, settings.ScienceSlots, nil)
		selected = &empty
	}

	slotReport := domain.ValidateSlotCapacity(*selected)

	// Build consolidated shopping list from plan items using live Jita mineral prices from Public ESI
	liveMinerals := s.GetLiveJitaMinerals(ctx)
	reqs := buildEstimatedBOMRequirements(selected.Items, liveMinerals)

	// Deduct hangar assets if SSO connected
	hangarStock := make(map[int32]int64)
	mode := s.GetModeInfo(ctx)
	if mode.SSOConnected && s.chars != nil {
		if assets, err := s.chars.ListAssets(ctx, mode.CharacterID); err == nil {
			for _, a := range assets {
				hangarStock[a.TypeID] += a.Quantity
			}
		}
	}

	shoppingList := domain.BuildConsolidatedShoppingList(reqs, hangarStock)

	weeklyROI := selected.ProjectedROI
	if weeklyROI <= 0 {
		weeklyROI = 18.0
	}
	growth, _ := domain.SimulateFourWeekGrowth(availCap, weeklyROI, domain.DefaultReinvestRate)

	return &PlannerViewData{
		ActivePlan:   selected,
		AllPlans:     allPlans,
		SlotReport:   slotReport,
		ShoppingList: shoppingList,
		Growth4Weeks: growth,
		Settings:     *settings,
	}, nil
}

// GetLiveJitaMinerals fetches live Jita 4-4 prices for core EVE industrial minerals via Public ESI (cached).
func (s *CopilotService) GetLiveJitaMinerals(ctx context.Context) []MineralQuote {
	defaults := []MineralQuote{
		{TypeID: 34, Name: "Tritanium", LowestSell: 4.50, HighestBuy: 4.10},
		{TypeID: 35, Name: "Pyerite", LowestSell: 12.00, HighestBuy: 11.20},
		{TypeID: 36, Name: "Mexallon", LowestSell: 55.00, HighestBuy: 51.00},
		{TypeID: 37, Name: "Isogen", LowestSell: 180.00, HighestBuy: 168.00},
		{TypeID: 38, Name: "Nocxium", LowestSell: 1450.00, HighestBuy: 1380.00},
		{TypeID: 39, Name: "Zydrine", LowestSell: 1950.00, HighestBuy: 1860.00},
	}

	if s.market == nil {
		return defaults
	}

	tids := make([]int32, 0, len(defaults))
	for _, m := range defaults {
		tids = append(tids, m.TypeID)
	}

	esiCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	prices, err := s.market.GetJitaPrices(esiCtx, tids)
	if err != nil {
		return defaults
	}

	for i := range defaults {
		if jp, ok := prices[defaults[i].TypeID]; ok && jp.LowestSell > 0 {
			defaults[i].LowestSell = jp.LowestSell
			defaults[i].HighestBuy = jp.HighestBuy
			defaults[i].IsLive = true
		}
	}
	return defaults
}

// CheckTrackerUndercutsESI checks live Jita lowest sell prices for all LISTED/SLOW_MOVER batches via Public ESI.
func (s *CopilotService) CheckTrackerUndercutsESI(ctx context.Context) (int, int, error) {
	if s.batches == nil || s.market == nil {
		return 0, 0, fmt.Errorf("batch repository atau Public ESI market provider tidak tersedia")
	}

	all, err := s.batches.ListBatches(ctx, nil)
	if err != nil {
		return 0, 0, err
	}

	var active []domain.BatchRecord
	var missingNames []string
	for _, b := range all {
		if b.Status == domain.BatchStatusListed || b.Status == domain.BatchStatusSlowMover {
			active = append(active, b)
			if b.TypeID <= 0 && strings.TrimSpace(b.ItemName) != "" {
				missingNames = append(missingNames, b.ItemName)
			}
		}
	}
	if len(active) == 0 {
		return 0, 0, nil
	}

	// Resolve missing TypeIDs via Public ESI
	if len(missingNames) > 0 {
		if resolved, err := s.market.ResolveTypeIDs(ctx, missingNames); err == nil {
			for i := range active {
				if active[i].TypeID <= 0 {
					if tid, ok := resolved[active[i].ItemName]; ok {
						active[i].TypeID = tid
					}
				}
			}
		}
	}

	typeIDSet := make(map[int32]struct{})
	var typeIDs []int32
	for _, b := range active {
		if b.TypeID > 0 {
			if _, seen := typeIDSet[b.TypeID]; !seen {
				typeIDSet[b.TypeID] = struct{}{}
				typeIDs = append(typeIDs, b.TypeID)
			}
		}
	}
	if len(typeIDs) == 0 {
		return 0, 0, nil
	}

	prices, err := s.market.GetJitaPrices(ctx, typeIDs)
	if err != nil {
		return 0, 0, fmt.Errorf("gagal mengambil harga Jita live: %w", err)
	}

	settings, _ := s.GetSettings(ctx)
	checked := 0
	undercuts := 0

	for i := range active {
		b := active[i]
		jp, ok := prices[b.TypeID]
		if !ok || jp.LowestSell <= 0 {
			continue
		}
		checked++
		b.CurrentMarketPrice = jp.LowestSell
		b.IsUndercut = b.ActualSellPrice > 0 && jp.LowestSell < b.ActualSellPrice
		if b.IsUndercut {
			undercuts++
		}
		b.SafeRelistPrice = domain.CalculateSafeRelistPrice(b.ActualProductionCost, b.UnitsOutput, settings.SalesTaxPercent, settings.RelistFeePercent)
		b.RemainingSafeMargin = domain.RoundISK((jp.LowestSell - b.SafeRelistPrice) * float64(b.UnitsOutput))
		_ = s.batches.SaveBatch(ctx, &b)
	}

	return checked, undercuts, nil
}

// buildEstimatedBOMRequirements generates a consolidated mineral bill of materials priced with live Jita ESI quotes.
func buildEstimatedBOMRequirements(items []domain.PlanItem, minerals []MineralQuote) []domain.MaterialRequirement {
	if len(items) == 0 {
		return nil
	}

	priceByID := map[int32]float64{
		34: 4.5,
		35: 12.0,
		36: 55.0,
		37: 180.0,
	}
	for _, m := range minerals {
		if m.LowestSell > 0 {
			priceByID[m.TypeID] = m.LowestSell
		}
	}

	tritPrice := priceByID[34]
	pyerPrice := priceByID[35]
	mexPrice := priceByID[36]
	isoPrice := priceByID[37]

	var reqs []domain.MaterialRequirement
	for _, it := range items {
		if it.TotalMaterialCost <= 0 {
			continue
		}
		// Standard industrial mineral breakdown (Tritanium 45%, Pyerite 25%, Mexallon 20%, Isogen 10%)
		tritQty := int64((it.TotalMaterialCost * 0.45) / tritPrice)
		pyerQty := int64((it.TotalMaterialCost * 0.25) / pyerPrice)
		mexQty := int64((it.TotalMaterialCost * 0.20) / mexPrice)
		isoQty := int64((it.TotalMaterialCost * 0.10) / isoPrice)

		if tritQty > 0 {
			reqs = append(reqs, domain.MaterialRequirement{TypeID: 34, TypeName: "Tritanium", Quantity: tritQty, UnitPrice: tritPrice, UnitVolumeM3: 0.01})
		}
		if pyerQty > 0 {
			reqs = append(reqs, domain.MaterialRequirement{TypeID: 35, TypeName: "Pyerite", Quantity: pyerQty, UnitPrice: pyerPrice, UnitVolumeM3: 0.01})
		}
		if mexQty > 0 {
			reqs = append(reqs, domain.MaterialRequirement{TypeID: 36, TypeName: "Mexallon", Quantity: mexQty, UnitPrice: mexPrice, UnitVolumeM3: 0.01})
		}
		if isoQty > 0 {
			reqs = append(reqs, domain.MaterialRequirement{TypeID: 37, TypeName: "Isogen", Quantity: isoQty, UnitPrice: isoPrice, UnitVolumeM3: 0.01})
		}
	}
	return reqs
}

// SaveWeeklyPlan persists a WeeklyPlan.
func (s *CopilotService) SaveWeeklyPlan(ctx context.Context, plan *domain.WeeklyPlan) (*domain.WeeklyPlan, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan cannot be nil")
	}
	if len(plan.Days) == 0 {
		plan.Days = domain.GenerateWeeklySchedule(plan.Items, plan.ManufacturingSlotsTotal)
	}
	if s.plans != nil {
		if err := s.plans.SavePlan(ctx, plan); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

// ConvertPlanToTrackerBatches converts a WeeklyPlan's items into Draft BatchRecords in the Tracker.
func (s *CopilotService) ConvertPlanToTrackerBatches(ctx context.Context, planID int64) (int, error) {
	if s.plans == nil || s.batches == nil {
		return 0, fmt.Errorf("plan or batch repository unavailable")
	}
	plan, err := s.plans.GetPlanByID(ctx, planID)
	if err != nil || plan == nil {
		return 0, fmt.Errorf("plan #%d tidak ditemukan", planID)
	}
	settings, _ := s.GetSettings(ctx)
	batches := domain.ConvertPlanToBatches(*plan, *settings)
	if len(batches) == 0 {
		return 0, nil
	}
	if err := s.batches.CreateBatchesFromPlan(ctx, plan.ID, batches); err != nil {
		return 0, err
	}
	return len(batches), nil
}

// ListBatches retrieves tracked batches optionally filtered by status.
func (s *CopilotService) ListBatches(ctx context.Context, statusFilter string) ([]domain.BatchRecord, error) {
	if s.batches == nil {
		return []domain.BatchRecord{}, nil
	}
	var st *domain.BatchStatus
	if statusFilter != "" {
		sVal := domain.BatchStatus(statusFilter)
		st = &sVal
	}
	return s.batches.ListBatches(ctx, st)
}

// SaveBatch creates or updates a BatchRecord.
func (s *CopilotService) SaveBatch(ctx context.Context, b *domain.BatchRecord) (*domain.BatchRecord, error) {
	if b == nil || strings.TrimSpace(b.ItemName) == "" {
		return nil, fmt.Errorf("batch item name is required")
	}
	if b.Runs <= 0 {
		b.Runs = 1
	}
	if b.UnitsOutput <= 0 {
		b.UnitsOutput = b.Runs
	}
	if b.Status == "" {
		b.Status = domain.BatchStatusDraft
	}

	settings, _ := s.GetSettings(ctx)
	b.TotalRevenue = domain.RoundISK(b.ActualSellPrice * float64(b.UnitsOutput))
	if b.ActualProductionCost <= 0 {
		b.ActualProductionCost = b.TotalMaterialCost
	}
	fees := b.TotalRevenue * ((settings.BrokerFeePercent + settings.SalesTaxPercent) / 100.0)
	b.NetProfit = domain.RoundISK(b.TotalRevenue - b.ActualProductionCost - fees)
	if b.ActualProductionCost > 0 {
		b.ROI = domain.RoundPercent((b.NetProfit / b.ActualProductionCost) * 100.0)
	}
	b.SafeRelistPrice = domain.CalculateSafeRelistPrice(b.ActualProductionCost, b.UnitsOutput, settings.SalesTaxPercent, settings.RelistFeePercent)
	b.RemainingSafeMargin = domain.RoundISK((b.ActualSellPrice - b.SafeRelistPrice) * float64(b.UnitsOutput))

	if s.batches != nil {
		if err := s.batches.SaveBatch(ctx, b); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// UpdateBatchStatus transitions a batch to a new lifecycle state.
func (s *CopilotService) UpdateBatchStatus(ctx context.Context, id int64, status domain.BatchStatus) error {
	if s.batches == nil {
		return nil
	}
	return s.batches.UpdateBatchStatus(ctx, id, status)
}

// DeleteBatch removes a batch record.
func (s *CopilotService) DeleteBatch(ctx context.Context, id int64) error {
	if s.batches == nil {
		return nil
	}
	return s.batches.DeleteBatch(ctx, id)
}

// CheckRelistGuard evaluates whether a proposed sell price stays safely profitable for a batch.
func (s *CopilotService) CheckRelistGuard(ctx context.Context, batchID int64, proposedPrice float64) (*domain.RelistGuardResult, error) {
	if s.batches == nil {
		return nil, fmt.Errorf("batch repository unavailable")
	}
	batch, err := s.batches.GetBatchByID(ctx, batchID)
	if err != nil || batch == nil {
		return nil, fmt.Errorf("batch #%d tidak ditemukan", batchID)
	}
	settings, _ := s.GetSettings(ctx)
	res := domain.EvaluateRelistGuard(*batch, proposedPrice, *settings)
	return &res, nil
}

// SyncCharacterSSO synchronizes wallet, orders, industry jobs, blueprints, and hangar assets when SSO is active.
func (s *CopilotService) SyncCharacterSSO(ctx context.Context) (*domain.CharacterProfile, error) {
	if s.sso == nil || !s.sso.IsEnabled() {
		return nil, esi.ErrSSODisabled
	}
	if s.chars == nil || s.charSync == nil {
		return nil, fmt.Errorf("character sync dependencies unavailable")
	}

	tok, err := s.chars.GetPrimaryToken(ctx)
	if err != nil || tok == nil {
		return nil, fmt.Errorf("belum ada karakter EVE SSO yang terhubung")
	}

	wallet, err := s.charSync.SyncWallet(ctx, tok)
	if err != nil {
		return nil, err
	}

	orders, _ := s.charSync.SyncOrders(ctx, tok)
	if len(orders) > 0 {
		_ = s.chars.UpsertOrders(ctx, tok.CharacterID, orders)
	}

	jobs, _ := s.charSync.SyncJobs(ctx, tok)
	usedMfg := 0
	usedSci := 0
	for _, j := range jobs {
		if j.Status == "active" {
			if j.ActivityID == 1 {
				usedMfg++
			} else {
				usedSci++
			}
		}
	}
	if len(jobs) > 0 {
		_ = s.chars.UpsertJobs(ctx, tok.CharacterID, jobs)
	}

	if bps, err := s.charSync.SyncBlueprints(ctx, tok); err == nil && len(bps) > 0 {
		_ = s.chars.UpsertBlueprints(ctx, tok.CharacterID, bps)
	}

	if assets, err := s.charSync.SyncAssets(ctx, tok); err == nil && len(assets) > 0 {
		_ = s.chars.UpsertAssets(ctx, tok.CharacterID, assets)
	}

	settings, _ := s.GetSettings(ctx)
	profile := &domain.CharacterProfile{
		CharacterID:             tok.CharacterID,
		CharacterName:           tok.CharacterName,
		WalletBalance:           wallet,
		TotalManufacturingSlots: settings.ManufacturingSlots,
		UsedManufacturingSlots:  usedMfg,
		TotalScienceSlots:       settings.ScienceSlots,
		UsedScienceSlots:        usedSci,
		UpdatedAt:               time.Now(),
	}
	_ = s.chars.UpsertProfile(ctx, profile)
	return profile, nil
}

// LoadSampleData populates realistic beginner industrial items and trap items for instant testing.
func (s *CopilotService) LoadSampleData(ctx context.Context) (int, error) {
	if s.items == nil {
		return 0, fmt.Errorf("item repository unavailable")
	}

	settings, _ := s.GetSettings(ctx)
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)
	now := time.Now()

	samples := []domain.MarketItem{
		{
			TypeID:                2456,
			Name:                  "Hobgoblin I",
			Category:              domain.CategoryDrones,
			EstimatedJitaSell:     14_500,
			EstimatedJitaBuy:      11_800,
			DailyVolume:           45_000,
			Volume30d:             42_000,
			MaterialCost:          4_900_000,
			BatchOutputQty:        500,
			ProductionTimeSeconds: 14400, // 4h
			IsManufacturable:      true,
			IsFrequentlyDestroyed: true,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		{
			TypeID:                2488,
			Name:                  "Warrior II",
			Category:              domain.CategoryDrones,
			EstimatedJitaSell:     720_000,
			EstimatedJitaBuy:      640_000,
			DailyVolume:           12_500,
			Volume30d:             11_800,
			MaterialCost:          14_200_000,
			BatchOutputQty:        30,
			ProductionTimeSeconds: 28800, // 8h
			IsManufacturable:      true,
			IsFrequentlyDestroyed: true,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		{
			TypeID:                587,
			Name:                  "Rifter",
			Category:              domain.CategoryShips,
			EstimatedJitaSell:     680_000,
			EstimatedJitaBuy:      590_000,
			DailyVolume:           950,
			Volume30d:             890,
			MaterialCost:          4_600_000,
			BatchOutputQty:        10,
			ProductionTimeSeconds: 21600, // 6h
			IsManufacturable:      true,
			IsFrequentlyDestroyed: true,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		{
			TypeID:                209,
			Name:                  "Scourge Heavy Missile",
			Category:              domain.CategoryAmmunition,
			EstimatedJitaSell:     89,
			EstimatedJitaBuy:      74,
			DailyVolume:           8_500_000,
			Volume30d:             7_900_000,
			MaterialCost:          5_800_000,
			BatchOutputQty:        100_000,
			ProductionTimeSeconds: 10800, // 3h
			IsManufacturable:      true,
			IsFrequentlyDestroyed: true,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		{
			TypeID:                31716,
			Name:                  "Small Core Defense Field Extender I",
			Category:              domain.CategoryRigs,
			EstimatedJitaSell:     3_250_000,
			EstimatedJitaBuy:      2_800_000,
			DailyVolume:           620,
			Volume30d:             580,
			MaterialCost:          11_500_000,
			BatchOutputQty:        5,
			ProductionTimeSeconds: 18000, // 5h
			IsManufacturable:      true,
			IsFrequentlyDestroyed: true,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		{
			// Trap #1: Rare Faction BPC with 0 ExtraSetupCost
			TypeID:                15681,
			Name:                  "Caldari Navy Ballistic Control System",
			Category:              domain.CategoryModules,
			EstimatedJitaSell:     88_000_000,
			EstimatedJitaBuy:      78_000_000,
			DailyVolume:           180,
			Volume30d:             170,
			MaterialCost:          12_000_000,
			BatchOutputQty:        1,
			ProductionTimeSeconds: 7200,
			IsManufacturable:      true,
			RequiresRareBPC:       true,
			ExtraSetupCost:        0,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		{
			// Trap #2: Non-Manufacturable Compressed Ice
			TypeID:                28433,
			Name:                  "Compressed Blue Ice",
			Category:              domain.CategoryCommodities,
			EstimatedJitaSell:     415_000,
			EstimatedJitaBuy:      395_000,
			DailyVolume:           120_000,
			Volume30d:             115_000,
			MaterialCost:          400_000,
			BatchOutputQty:        100,
			ProductionTimeSeconds: 0,
			IsManufacturable:      false,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
		{
			// Trap #3: Slow Mover Niche Rig
			TypeID:                31065,
			Name:                  "Large Auxiliary Nano Pump I",
			Category:              domain.CategoryRigs,
			EstimatedJitaSell:     14_500_000,
			EstimatedJitaBuy:      11_000_000,
			DailyVolume:           2.0,
			Volume30d:             2.5,
			MaterialCost:          28_000_000,
			BatchOutputQty:        3,
			ProductionTimeSeconds: 21600,
			IsManufacturable:      true,
			CreatedAt:             now,
			UpdatedAt:             now,
		},
	}

	for i := range samples {
		samples[i].TrapAnalysis = domain.EvaluateTrapGuard(samples[i], *settings, availCap)
	}

	if err := s.items.BatchUpsertItems(ctx, samples); err != nil {
		return 0, err
	}
	return len(samples), nil
}
