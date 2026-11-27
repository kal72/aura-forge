package service

import (
	"context"
	"fmt"
	"math"
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
	WalletBalance float64         `json:"wallet_balance"`
	ModeLabel     string          `json:"mode_label"`
	StatusHint    string          `json:"status_hint"`
	CurrentUser   *domain.AppUser `json:"current_user,omitempty"`
	PriceSync     PriceSyncStatus `json:"price_sync"`
	AI            domain.AIStatus `json:"ai"`
}

// IsAdmin returns true for authenticated administrators (and in auth-less unit test routers where CurrentUser is nil).
// In production, RequireAuth guarantees CurrentUser is always non-nil, so role 'user' always evaluates to false.
func (m ModeInfo) IsAdmin() bool {
	return m.CurrentUser == nil || m.CurrentUser.IsAdmin()
}

// CopilotAction represents one prioritized, 1-click actionable directive for the Daily Copilot Assistant.
type CopilotAction struct {
	Priority    string `json:"priority"` // "URGENT" | "ACTION" | "TIP" | "OK"
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	ActionLabel string `json:"action_label"`
	ActionURL   string `json:"action_url,omitempty"`
	ActionPost  string `json:"action_post,omitempty"`
}

// DashboardSummary aggregates top-level KPIs and daily assistant tasks.
type DashboardSummary struct {
	Mode                  ModeInfo           `json:"mode"`
	ActiveHub             domain.TradeHub    `json:"active_hub"`
	AvailableCapital      float64            `json:"available_capital"`
	CapitalSource         string             `json:"capital_source"`
	LockedCapital         float64            `json:"locked_capital"`
	RealizedProfit        float64            `json:"realized_profit"`
	MfgSlotsTotal         int                `json:"mfg_slots_total"`
	MfgSlotsUsed          int                `json:"mfg_slots_used"`
	SciSlotsTotal         int                `json:"sci_slots_total"`
	SciSlotsUsed          int                `json:"sci_slots_used"`
	TotalCatalogItems     int                `json:"total_catalog_items"`
	LayakItemsCount       int                `json:"layak_items_count"`
	JebakanItemsCount     int                `json:"jebakan_items_count"`
	ActiveBatchesCount    int                `json:"active_batches_count"`
	DraftBatchesCount     int                `json:"draft_batches_count"`
	SlowMoverBatchesCount int                `json:"slow_mover_batches_count"`
	UndercutBatchesCount  int                `json:"undercut_batches_count"`
	ExpiringColoniesCount int                `json:"expiring_colonies_count"`
	ExpiredColoniesCount  int                `json:"expired_colonies_count"`
	CapturedWealthToday   bool               `json:"captured_wealth_today"`
	CurrentNetWorth       float64            `json:"current_net_worth"`
	NextMilestoneLabel    string             `json:"next_milestone_label"`
	NextMilestonePct      float64            `json:"next_milestone_pct"`
	ActivePlan            *domain.WeeklyPlan `json:"active_plan,omitempty"`
	DailyChecklist        []string           `json:"daily_checklist"`
	CopilotActions        []CopilotAction    `json:"copilot_actions"`
}

// MineralQuote holds a live trade-hub price snapshot, 7-day trend signal, volume surge, and sparkline for the Dashboard Market Pulse.
type MineralQuote struct {
	TypeID            int32   `json:"type_id"`
	Name              string  `json:"name"`
	LowestSell        float64 `json:"lowest_sell"`
	HighestBuy        float64 `json:"highest_buy"`
	SpreadPercent     float64 `json:"spread_percent"`
	Avg7dPrice        float64 `json:"avg_7d_price"`
	Change7dPercent   float64 `json:"change_7d_percent"`
	Signal            string  `json:"signal"` // CHEAP | FAIR | HIGH | HOT
	SparklinePoints   string  `json:"sparkline_points,omitempty"`
	Category          string  `json:"category"`
	HubKey            string  `json:"hub_key"`
	HubName           string  `json:"hub_name"`
	IsPI              bool    `json:"is_pi"`
	IsLive            bool    `json:"is_live"`
	BlueprintID       int32   `json:"blueprint_id,omitempty"`
	BlueprintCategory string  `json:"blueprint_category,omitempty"`
	DailyVolume7d     float64 `json:"daily_volume_7d,omitempty"`
	VolumeRatio       float64 `json:"volume_ratio,omitempty"` // 7d avg vol / 30d avg vol
	IsGlobalMover     bool    `json:"is_global_mover,omitempty"`
}

// PlannerViewData bundles all data required to render the Weekly Production & Slot Planner.
type PlannerViewData struct {
	ActivePlan        *domain.WeeklyPlan              `json:"active_plan"`
	AllPlans          []domain.WeeklyPlan             `json:"all_plans"`
	SlotReport        domain.SlotCapacityReport       `json:"slot_report"`
	ShoppingList      domain.ConsolidatedShoppingList `json:"shopping_list"`
	Growth4Weeks      []domain.GrowthProjection       `json:"growth_4_weeks"`
	Settings          domain.UserSettings             `json:"settings"`
	Hub               domain.TradeHub                 `json:"hub"`
	SDEMatchedItems   int                             `json:"sde_matched_items"`
	TotalPlannedItems int                             `json:"total_planned_items"`
	UsingRealSDE      bool                            `json:"using_real_sde"`
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
	sde             domain.SDERepository
	wealth          domain.NetWorthRepository
	pi              domain.PIRepository
	globalSnapshots domain.GlobalPriceSnapshotRepository
	aiProvider      domain.AIProvider
	aiRepo          domain.AIRepository
	ledger          domain.TradeLedgerRepository
	priceSync       *priceSyncState
	sdeImport       *sdeImportState
	radar           *opportunityRadarState
}

// SetGlobalPriceSnapshotRepository attaches the optional PostgreSQL global price snapshot repository.
func (s *CopilotService) SetGlobalPriceSnapshotRepository(repo domain.GlobalPriceSnapshotRepository) {
	s.globalSnapshots = repo
}

// SetAIProvider attaches the optional OpenAI-compatible AI provider.
func (s *CopilotService) SetAIProvider(p domain.AIProvider) {
	s.aiProvider = p
}

// SetAIRepository attaches the PostgreSQL AI conversation & pilot memory repository.
func (s *CopilotService) SetAIRepository(r domain.AIRepository) {
	s.aiRepo = r
}

// SetTradeLedgerRepository attaches the PostgreSQL trade ledger & order history repository.
func (s *CopilotService) SetTradeLedgerRepository(r domain.TradeLedgerRepository) {
	s.ledger = r
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
		sdeImport: newSDEImportState(),
	}
}

// GetModeInfo inspects SSO config, stored character tokens, and authenticated user context.
func (s *CopilotService) GetModeInfo(ctx context.Context) ModeInfo {
	currentUser := domain.AppUserFromContext(ctx)
	priceSync := s.GetPriceSyncStatus(ctx)
	aiStatus := domain.AIStatus{}
	if s.aiProvider != nil {
		aiStatus = s.aiProvider.Status()
	}
	ssoEnabled := s.sso != nil && s.sso.IsEnabled()
	if !ssoEnabled {
		return ModeInfo{
			SSOEnabled:  false,
			ModeLabel:   "Public ESI Active",
			StatusHint:  "Optional: Set ESI_CLIENT_ID & SECRET in .env for Character Auto-Sync",
			CurrentUser: currentUser,
			PriceSync:   priceSync,
			AI:          aiStatus,
		}
	}

	if s.chars != nil {
		if tok, err := s.chars.GetPrimaryToken(ctx); err == nil && tok != nil {
			var walletBal float64
			if prof, pErr := s.chars.GetProfile(ctx, tok.CharacterID); pErr == nil && prof != nil {
				walletBal = prof.WalletBalance
			}
			return ModeInfo{
				SSOEnabled:    true,
				SSOConnected:  true,
				CharacterID:   tok.CharacterID,
				CharacterName: tok.CharacterName,
				WalletBalance: walletBal,
				ModeLabel:     "EVE SSO Connected",
				StatusHint:    fmt.Sprintf("Connected as %s", tok.CharacterName),
				CurrentUser:   currentUser,
				PriceSync:     priceSync,
				AI:            aiStatus,
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
		AI:           aiStatus,
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
	s.priceSync.updateSchedulePreview(cfg.AutoPriceSyncEnabled, cfg.AutoPriceSyncIntervalMinutes)
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
	// Keep Business Working Capital (Settings) cleanly separated from personal EVE Character Wallet.
	capISK := settings.StartingCapital
	if capISK <= 0 {
		capISK = 100_000_000.0
	}
	return profile, capISK, "Working Capital (Settings)"
}

// GetDashboard aggregates live KPIs for the main dashboard view.
func (s *CopilotService) GetDashboard(ctx context.Context) (*DashboardSummary, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	mode := s.GetModeInfo(ctx)
	profile, availCap, capSource := s.resolveProfileAndCapital(ctx, *settings)
	hub := settings.ActiveTradeHub()

	summary := &DashboardSummary{
		Mode:             mode,
		ActiveHub:        hub,
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
				case domain.BatchStatusDraft:
					summary.DraftBatchesCount++
				case domain.BatchStatusInProduction:
					summary.LockedCapital += b.ActualProductionCost
					summary.ActiveBatchesCount++
					if profile == nil {
						summary.MfgSlotsUsed += b.Runs
					}
				case domain.BatchStatusListed, domain.BatchStatusSlowMover:
					summary.LockedCapital += b.ActualProductionCost
					summary.ActiveBatchesCount++
					if b.Status == domain.BatchStatusSlowMover {
						summary.SlowMoverBatchesCount++
					}
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

	// Check PI Colony extractor timers
	if s.pi != nil {
		if cols, err := s.pi.ListColonies(ctx); err == nil {
			now := time.Now()
			for _, c := range cols {
				status, _ := c.StatusLabel(now)
				switch status {
				case "EXPIRED":
					summary.ExpiredColoniesCount++
				case "EXPIRING":
					summary.ExpiringColoniesCount++
				}
			}
		}
	}

	// Check Net Worth snapshots for today's capture & next 100B+ milestone progress
	today := time.Now().Format("2006-01-02")
	estNetWorth := domain.RoundISK(summary.AvailableCapital + summary.LockedCapital)
	if s.wealth != nil {
		if snaps, err := s.wealth.ListSnapshots(ctx, 14); err == nil && len(snaps) > 0 {
			latest := snaps[len(snaps)-1]
			estNetWorth = latest.TotalNetWorth
			for _, sn := range snaps {
				if sn.SnapshotDate == today {
					summary.CapturedWealthToday = true
					break
				}
			}
		}
	}
	summary.CurrentNetWorth = estNetWorth
	_, nextMs := domain.BuildWealthMilestones(estNetWorth)
	summary.NextMilestoneLabel = nextMs.Label
	summary.NextMilestonePct = nextMs.ProgressPct

	summary.CopilotActions = buildCopilotActions(summary)
	summary.DailyChecklist = buildDailyChecklist(summary)
	return summary, nil
}

func buildCopilotActions(d *DashboardSummary) []CopilotAction {
	hubName := d.ActiveHub.Name
	if hubName == "" {
		hubName = "Primary Hub"
	}
	actions := make([]CopilotAction, 0, 6)

	// 1. URGENT: Undercut sell orders
	if d.UndercutBatchesCount > 0 {
		actions = append(actions, CopilotAction{
			Priority:    "URGENT",
			Title:       fmt.Sprintf("%d Listed Order(s) Undercut in %s", d.UndercutBatchesCount, hubName),
			Detail:      "Competitors beat your sell price. Check Safe Relist Floor before modifying orders so broker fees don't erase your margin.",
			ActionLabel: "Fix Undercuts →",
			ActionURL:   "/tracker?status=LISTED",
		})
	}

	// 2. URGENT: Slow Mover capital lock-up
	if d.SlowMoverBatchesCount > 0 {
		actions = append(actions, CopilotAction{
			Priority:    "URGENT",
			Title:       fmt.Sprintf("%d Slow-Mover Batch(es) Locking Capital", d.SlowMoverBatchesCount),
			Detail:      "Stale listings tie up working capital. Review Safe Break-Even Floor and re-price or liquidate to free ISK.",
			ActionLabel: "Inspect Slow Movers →",
			ActionURL:   "/tracker?status=SLOW_MOVER",
		})
	}

	// 3. URGENT: Expired or Expiring PI Colonies
	if d.ExpiredColoniesCount > 0 || d.ExpiringColoniesCount > 0 {
		actions = append(actions, CopilotAction{
			Priority:    "URGENT",
			Title:       fmt.Sprintf("PI Extractors Need Attention (%d expired, %d expiring)", d.ExpiredColoniesCount, d.ExpiringColoniesCount),
			Detail:      "Idle extractors stop passive daily ISK generation. Reset your planet timers in 1 click.",
			ActionLabel: "Open PI Colonies →",
			ActionURL:   "/pi",
		})
	}

	// 4. ACTION: Unstarted Draft Batches waiting in Tracker
	if d.DraftBatchesCount > 0 {
		actions = append(actions, CopilotAction{
			Priority:    "ACTION",
			Title:       fmt.Sprintf("%d Draft Batch(es) Ready to Start in Tracker", d.DraftBatchesCount),
			Detail:      "You have queued draft batches waiting to begin production or market listing.",
			ActionLabel: "Start Drafts (Step 4) →",
			ActionURL:   "/tracker?status=DRAFT",
		})
	}

	// 5. ACTION: Idle Manufacturing Slots (strictly follows Step 1 Catalog -> Step 2 Allocator -> Step 3 Planner)
	if d.MfgSlotsUsed < d.MfgSlotsTotal {
		idle := d.MfgSlotsTotal - d.MfgSlotsUsed
		var targetURL, btnLabel, detail string
		switch {
		case d.TotalCatalogItems == 0:
			targetURL = "/catalog"
			btnLabel = "Step 1: Fill Catalog →"
			detail = fmt.Sprintf("Add viable items to your %s Catalog first (or use Scanner), then split capital in Allocator.", hubName)
		case d.ActivePlan == nil:
			targetURL = "/allocator"
			btnLabel = "Step 2: Split Capital →"
			detail = "Use Smart Capital Allocator to size safe batches from your Catalog and generate your Weekly Plan."
		default:
			targetURL = "/planner"
			btnLabel = "Step 3: Fill Slots (Planner) →"
			detail = "Empty industry slots slow down compounding toward 100B+ Total Wealth. Queue your next profitable wave."
		}
		actions = append(actions, CopilotAction{
			Priority:    "ACTION",
			Title:       fmt.Sprintf("%d of %d Manufacturing Slot(s) Sitting Idle", idle, d.MfgSlotsTotal),
			Detail:      detail,
			ActionLabel: btnLabel,
			ActionURL:   targetURL,
		})
	}

	// 6. TIP: Only prompt Live Price Sync if Catalog exists AND cache is stale (>30m) or never synced
	if d.TotalCatalogItems > 0 {
		isStale := d.Mode.PriceSync.LastSyncedAt == nil || time.Since(*d.Mode.PriceSync.LastSyncedAt) > 30*time.Minute
		if isStale && !d.Mode.PriceSync.IsRunning {
			actions = append(actions, CopilotAction{
				Priority:    "TIP",
				Title:       fmt.Sprintf("Refresh Live %s Prices & Volumes", hubName),
				Detail:      fmt.Sprintf("Cached prices for your %d catalog item(s) are older than 30 minutes. Sync now for fresh spreads.", d.TotalCatalogItems),
				ActionLabel: "Sync " + hubName + " Prices",
				ActionPost:  "/esi/sync-prices",
			})
		}
	}

	// 5. ACTION: Today's Net Worth Snapshot
	if !d.CapturedWealthToday {
		actions = append(actions, CopilotAction{
			Priority:    "ACTION",
			Title:       fmt.Sprintf("Log Today's Total Wealth (Next: %s — %.1f%%)", d.NextMilestoneLabel, d.NextMilestonePct),
			Detail:      "Capture a daily snapshot to plot your equity curve and verify real compounding velocity.",
			ActionLabel: "Auto-Capture Today",
			ActionPost:  "/wealth/auto-snapshot",
		})
	}

	if len(actions) == 0 {
		actions = append(actions, CopilotAction{
			Priority:    "OK",
			Title:       "All Systems Optimal — Keep Compounding!",
			Detail:      "All manufacturing slots are busy, no active undercuts detected, and today's Net Worth snapshot is logged.",
			ActionLabel: "Open Manufacture →",
			ActionURL:   "/manufacture",
		})
	}
	return actions
}

func buildDailyChecklist(d *DashboardSummary) []string {
	hubName := d.ActiveHub.Name
	if hubName == "" {
		hubName = "Primary Hub"
	}
	list := make([]string, 0, 5)
	if d.TotalCatalogItems == 0 {
		list = append(list, "Run the Opportunity Scanner or Auto-Recommend in Catalog to start discovering profitable items.")
	} else {
		list = append(list, fmt.Sprintf("Sync live %s prices to refresh Sell/Buy spreads and daily volumes from Public ESI.", hubName))
	}
	if d.UndercutBatchesCount > 0 {
		list = append(list, fmt.Sprintf("ALERT: %d listed batch(es) are currently undercut in %s — verify Safe Relist Price in Tracker.", d.UndercutBatchesCount, hubName))
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

// SaveCatalogItem evaluates TrapGuard (including optional SDE BOM cost & 7d spike check) and saves a MarketItem.
func (s *CopilotService) SaveCatalogItem(ctx context.Context, item *domain.MarketItem) (*domain.MarketItem, error) {
	if item == nil || strings.TrimSpace(item.Name) == "" {
		return nil, fmt.Errorf("item name is required")
	}
	if item.BatchOutputQty <= 0 {
		item.BatchOutputQty = 1
	}

	// Preserve existing metadata on edit when unchanged
	if item.ID > 0 && s.items != nil {
		if existing, err := s.items.GetItemByID(ctx, item.ID); err == nil && existing != nil {
			if item.TypeID == 0 {
				item.TypeID = existing.TypeID
			}
			if item.Volume30d == 0 {
				item.Volume30d = existing.Volume30d
			}
			if item.ProductionTimeSeconds <= 0 {
				item.ProductionTimeSeconds = existing.ProductionTimeSeconds
			}
			if item.TrapAnalysis.Avg7dPrice == 0 {
				item.TrapAnalysis.Avg7dPrice = existing.TrapAnalysis.Avg7dPrice
			}
		}
	}

	// Attempt Public ESI type_id resolution if missing
	if item.TypeID == 0 && s.market != nil {
		if resolved, err := s.market.ResolveTypeIDs(ctx, []string{item.Name}); err == nil {
			if tid, ok := resolved[item.Name]; ok {
				item.TypeID = tid
			}
		}
	}

	// Auto-populate MaterialCost from Official CCP SDE BOM when left blank (0)
	if item.MaterialCost <= 0 && item.TypeID > 0 && item.IsManufacturable {
		if sdeCost, ok := s.EstimateSDEMaterialCost(ctx, item.TypeID, item.BatchOutputQty, nil); ok {
			item.MaterialCost = sdeCost
			item.TrapAnalysis.BOMSyncedFromSDE = true
		}
	}

	avg7dPrice := item.TrapAnalysis.Avg7dPrice
	if avg7dPrice <= 0 && item.TypeID > 0 {
		avg7dPrice = s.fetchCatalogAvg7dPrice(ctx, item.TypeID)
	}

	settings, _ := s.GetSettings(ctx)
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)
	item.TrapAnalysis = domain.EvaluateTrapGuardWithHistory(*item, *settings, availCap, avg7dPrice)

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

	// 2. Collect valid product TypeIDs + all unique SDE BOM material TypeIDs for 1 batched price lookup
	typeIDSet := make(map[int32]struct{}, len(items)*2)
	for _, it := range items {
		if it.TypeID > 0 {
			typeIDSet[it.TypeID] = struct{}{}
			if s.sde != nil && it.IsManufacturable {
				if bp, bpErr := s.sde.GetBlueprintByProductID(ctx, it.TypeID); bpErr == nil && bp != nil {
					if mats, mErr := s.sde.GetBlueprintMaterials(ctx, bp.BlueprintTypeID); mErr == nil {
						for _, m := range mats {
							if m.MaterialTypeID > 0 {
								typeIDSet[m.MaterialTypeID] = struct{}{}
							}
						}
					}
				}
			}
		}
	}
	typeIDs := make([]int32, 0, len(typeIDSet))
	for id := range typeIDSet {
		typeIDs = append(typeIDs, id)
	}

	settings, _ := s.GetSettings(ctx)
	hub := settings.ActiveTradeHub()

	prices, err := s.fetchHubPrices(ctx, hub, typeIDs)
	if err != nil {
		return 0, fmt.Errorf("fetching %s prices: %w", hub.Name, err)
	}

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
		if avg7, avg30 := s.fetchHubDailyVolume(ctx, hub, items[i].TypeID); avg7 > 0 || avg30 > 0 {
			items[i].DailyVolume = avg7
			items[i].Volume30d = avg30
		}

		// Auto-recompute MaterialCost from Official CCP SDE BOM at the active trade hub
		if items[i].IsManufacturable {
			if sdeCost, ok := s.EstimateSDEMaterialCostForHub(ctx, hub, items[i].TypeID, items[i].BatchOutputQty, prices); ok {
				items[i].MaterialCost = sdeCost
				items[i].TrapAnalysis.BOMSyncedFromSDE = true
			}
		}

		_, avg7dPrice, _ := s.fetchHub7dRange(ctx, hub, items[i].TypeID)
		items[i].TrapAnalysis = domain.EvaluateTrapGuardWithHistory(items[i], *settings, availCap, avg7dPrice)
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
	res.ActiveHubName = settings.ActiveTradeHub().Name
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
		// Prevent duplicate clutter when re-tuning Capital Allocator for the same Week & Title:
		// update the existing matching plan in-place instead of appending endless copies.
		if existingPlans, err := s.plans.ListPlans(ctx); err == nil {
			for _, ep := range existingPlans {
				if ep.WeekNumber == plan.WeekNumber && strings.EqualFold(strings.TrimSpace(ep.Title), strings.TrimSpace(plan.Title)) {
					plan.ID = ep.ID
					break
				}
			}
		}
		if err := s.plans.SavePlan(ctx, &plan); err != nil {
			return nil, err
		}
	}
	return &plan, nil
}

// DeleteWeeklyPlan permanently deletes a saved Weekly Plan by ID.
func (s *CopilotService) DeleteWeeklyPlan(ctx context.Context, planID int64) error {
	if s.plans == nil || planID <= 0 {
		return nil
	}
	return s.plans.DeletePlan(ctx, planID)
}

// ClearAllWeeklyPlans removes all saved Weekly Plans so the user can start fresh.
func (s *CopilotService) ClearAllWeeklyPlans(ctx context.Context) (int, error) {
	if s.plans == nil {
		return 0, nil
	}
	plans, err := s.plans.ListPlans(ctx)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, p := range plans {
		if err := s.plans.DeletePlan(ctx, p.ID); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

// CreateDraftBatchesFromAllocation converts allocation recommendations directly into Draft BatchRecords.
func (s *CopilotService) CreateDraftBatchesFromAllocation(ctx context.Context, cfg domain.AllocationConfig) (int, error) {
	plan, err := s.CreatePlanFromAllocation(ctx, "Direct Allocation", 1, cfg)
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
	hub := settings.ActiveTradeHub()
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
		empty := domain.BuildWeeklyPlan("Week #1 Plan (Draft)", 1, availCap, settings.ManufacturingSlots, settings.ScienceSlots, nil)
		selected = &empty
	}

	slotReport := domain.ValidateSlotCapacity(*selected)

	// Build consolidated shopping list using real CCP SDE BOMs priced at the user's active Trade Hub,
	// falling back to hub-priced mineral estimates only for items without a matching SDE blueprint.
	reqs, sdeMatched := s.buildRealSDEConsolidatedBOM(ctx, hub, selected.Items)

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
		ActivePlan:        selected,
		AllPlans:          allPlans,
		SlotReport:        slotReport,
		ShoppingList:      shoppingList,
		Growth4Weeks:      growth,
		Settings:          *settings,
		Hub:               hub,
		SDEMatchedItems:   sdeMatched,
		TotalPlannedItems: len(selected.Items),
		UsingRealSDE:      sdeMatched > 0,
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

	settings, _ := s.GetSettings(ctx)
	hub := settings.ActiveTradeHub()

	prices, err := s.fetchHubPrices(ctx, hub, typeIDs)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch live %s prices: %w", hub.Name, err)
	}
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

// buildRealSDEConsolidatedBOM resolves real CCP SDE blueprint materials for every PlanItem,
// prices all unique materials in one batched Public ESI lookup at the user's active Trade Hub,
// and falls back to hub-priced mineral estimates only for items without an SDE blueprint.
func (s *CopilotService) buildRealSDEConsolidatedBOM(ctx context.Context, hub domain.TradeHub, items []domain.PlanItem) ([]domain.MaterialRequirement, int) {
	if len(items) == 0 {
		return nil, 0
	}

	var sdeReqs []domain.MaterialRequirement
	var fallbackItems []domain.PlanItem
	sdeMatched := 0

	if s.sde != nil {
		for _, it := range items {
			var bp *domain.SDEBlueprint
			if it.TypeID > 0 {
				bp, _ = s.sde.GetBlueprintByProductID(ctx, it.TypeID)
			}
			if bp == nil && strings.TrimSpace(it.Name) != "" {
				if matches, err := s.sde.SearchBlueprints(ctx, strings.TrimSpace(it.Name), "", 5); err == nil {
					for i := range matches {
						if strings.EqualFold(matches[i].ProductName, strings.TrimSpace(it.Name)) {
							bp = &matches[i]
							break
						}
					}
				}
			}
			if bp == nil {
				fallbackItems = append(fallbackItems, it)
				continue
			}

			mats, err := s.sde.GetBlueprintMaterials(ctx, bp.BlueprintTypeID)
			if err != nil || len(mats) == 0 {
				fallbackItems = append(fallbackItems, it)
				continue
			}

			qtyPerRun := bp.ProductQtyPerRun
			if qtyPerRun <= 0 {
				qtyPerRun = 1
			}
			totalUnits := it.TotalUnits
			if totalUnits <= 0 {
				outPerBatch := it.OutputPerBatch
				if outPerBatch <= 0 {
					outPerBatch = qtyPerRun
				}
				runsCount := it.BatchRuns
				if runsCount <= 0 {
					runsCount = 1
				}
				totalUnits = runsCount * outPerBatch
			}
			runs := int(math.Ceil(float64(totalUnits) / float64(qtyPerRun)))
			if runs < 1 {
				runs = 1
			}

			defaultME := 10
			if bp.TechLevel >= 2 {
				defaultME = 2
			}

			for _, m := range mats {
				adjQty := domain.ComputeAdjustedMaterialQty(m.BaseQuantity, runs, defaultME, 0)
				if adjQty <= 0 {
					continue
				}
				vol := m.VolumeM3
				if vol <= 0 {
					vol = 0.01
				}
				sdeReqs = append(sdeReqs, domain.MaterialRequirement{
					TypeID:       m.MaterialTypeID,
					TypeName:     m.MaterialName,
					Quantity:     adjQty,
					UnitVolumeM3: vol,
				})
			}
			sdeMatched++
		}
	} else {
		fallbackItems = items
	}

	// Fetch live prices for all unique SDE materials in a single batched hub call
	if len(sdeReqs) > 0 && s.market != nil {
		seen := make(map[int32]struct{}, len(sdeReqs))
		matIDs := make([]int32, 0, len(sdeReqs))
		for _, r := range sdeReqs {
			if r.TypeID > 0 {
				if _, ok := seen[r.TypeID]; !ok {
					seen[r.TypeID] = struct{}{}
					matIDs = append(matIDs, r.TypeID)
				}
			}
		}
		if len(matIDs) > 0 {
			esiCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			prices, err := s.fetchHubPrices(esiCtx, hub, matIDs)
			cancel()
			if err == nil {
				for i := range sdeReqs {
					if jp, ok := prices[sdeReqs[i].TypeID]; ok {
						if jp.LowestSell > 0 {
							sdeReqs[i].UnitPrice = jp.LowestSell
						} else if jp.HighestBuy > 0 {
							sdeReqs[i].UnitPrice = jp.HighestBuy
						}
					}
				}
			}
		}
	}

	// Fallback mineral estimate only for non-SDE items, priced at the active hub
	if len(fallbackItems) > 0 {
		hubMinerals := s.getLiveHubMinerals(ctx, hub)
		sdeReqs = append(sdeReqs, buildEstimatedBOMRequirements(fallbackItems, hubMinerals)...)
	}

	return sdeReqs, sdeMatched
}

// getLiveHubMinerals fetches live core mineral quotes at the specified Trade Hub.
func (s *CopilotService) getLiveHubMinerals(ctx context.Context, hub domain.TradeHub) []MineralQuote {
	defaults := []MineralQuote{
		{TypeID: 34, Name: "Tritanium", LowestSell: 4.50, HighestBuy: 4.10},
		{TypeID: 35, Name: "Pyerite", LowestSell: 12.00, HighestBuy: 11.20},
		{TypeID: 36, Name: "Mexallon", LowestSell: 55.00, HighestBuy: 51.00},
		{TypeID: 37, Name: "Isogen", LowestSell: 180.00, HighestBuy: 168.00},
	}
	if s.market == nil {
		return defaults
	}
	tids := []int32{34, 35, 36, 37}
	esiCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	prices, err := s.fetchHubPrices(esiCtx, hub, tids)
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

// buildEstimatedBOMRequirements generates a consolidated mineral bill of materials priced with live ESI quotes.
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
		return 0, fmt.Errorf("plan #%d not found", planID)
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

// TrackerSummary aggregates top-level KPI metrics across all production & market batches.
type TrackerSummary struct {
	TotalCount     int     `json:"total_count"`
	ActiveCount    int     `json:"active_count"`
	ListedCount    int     `json:"listed_count"`
	SoldCount      int     `json:"sold_count"`
	UndercutCount  int     `json:"undercut_count"`
	LockedCapital  float64 `json:"locked_capital"`
	RealizedProfit float64 `json:"realized_profit"`
}

// SummarizeTrackerBatches computes KPI totals across all batches.
func SummarizeTrackerBatches(all []domain.BatchRecord) TrackerSummary {
	var sum TrackerSummary
	sum.TotalCount = len(all)
	for _, b := range all {
		switch b.Status {
		case domain.BatchStatusSold:
			sum.SoldCount++
			sum.RealizedProfit += b.NetProfit
		default:
			sum.ActiveCount++
			cost := b.ActualProductionCost
			if cost <= 0 {
				cost = b.TotalMaterialCost
			}
			sum.LockedCapital += cost
			if b.Status == domain.BatchStatusListed || b.Status == domain.BatchStatusSlowMover {
				sum.ListedCount++
				if b.IsUndercut {
					sum.UndercutCount++
				}
			}
		}
	}
	sum.LockedCapital = domain.RoundISK(sum.LockedCapital)
	sum.RealizedProfit = domain.RoundISK(sum.RealizedProfit)
	return sum
}

// SaveBatch creates or updates a BatchRecord, auto-resolving TypeID from local SDE or Public ESI when missing.
func (s *CopilotService) SaveBatch(ctx context.Context, b *domain.BatchRecord) (*domain.BatchRecord, error) {
	if b == nil || strings.TrimSpace(b.ItemName) == "" {
		return nil, fmt.Errorf("batch item name is required")
	}
	b.EntryType = b.NormalizedEntryType()
	if b.Runs <= 0 {
		b.Runs = 1
	}
	if b.UnitsOutput <= 0 {
		b.UnitsOutput = b.Runs
	}
	if b.Status == "" {
		b.Status = domain.BatchStatusDraft
	}

	// Auto-resolve TypeID from local sde_types first (0 ESI calls), then fallback to Public ESI
	if b.TypeID <= 0 {
		name := strings.TrimSpace(b.ItemName)
		if s.sde != nil {
			if ts, ok := s.sde.(domain.SDETypeSearchRepository); ok {
				if matches, err := ts.SearchTypes(ctx, name, 5); err == nil {
					for _, m := range matches {
						if strings.EqualFold(m.Name, name) {
							b.TypeID = m.TypeID
							break
						}
					}
				}
			}
		}
		if b.TypeID <= 0 && s.market != nil {
			if resolved, err := s.market.ResolveTypeIDs(ctx, []string{name}); err == nil {
				if tid, ok := resolved[name]; ok {
					b.TypeID = tid
				}
			}
		}
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
	jobs, _ := s.charSync.SyncJobs(ctx, tok)
	bps, _ := s.charSync.SyncBlueprints(ctx, tok)
	assets, _ := s.charSync.SyncAssets(ctx, tok)

	// Resolve human-readable EVE item names for synced orders, jobs, blueprints, and hangar assets
	s.resolveCharacterTypeNames(ctx, orders, jobs, bps, assets)

	if len(orders) > 0 {
		_ = s.chars.UpsertOrders(ctx, tok.CharacterID, orders)
	}

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
	if len(bps) > 0 {
		_ = s.chars.UpsertBlueprints(ctx, tok.CharacterID, bps)
	}
	if len(assets) > 0 {
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

	// Best-effort sync of Private ESI wallet transactions, journal fees, and 90-day order history
	_, _ = s.SyncPrivateESILedger(ctx)

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
