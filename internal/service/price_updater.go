package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
)

// PriceSyncStatus exposes the live state of the scoped Jita background price updater to the UI.
type PriceSyncStatus struct {
	Enabled         bool       `json:"enabled"`
	IntervalMinutes int        `json:"interval_minutes"`
	IsRunning       bool       `json:"is_running"`
	LastSyncedAt    *time.Time `json:"last_synced_at,omitempty"`
	LastSyncedCount int        `json:"last_synced_count"`
	CatalogUpdated  int        `json:"catalog_updated"`
	LastDurationMs  int64      `json:"last_duration_ms"`
	LastError       string     `json:"last_error,omitempty"`
	NextSyncAt      *time.Time `json:"next_sync_at,omitempty"`
}

// FreshnessLabel returns a concise, human-readable status string for the sidebar and settings page.
func (st PriceSyncStatus) FreshnessLabel() string {
	if st.IsRunning {
		return "Syncing Jita..."
	}
	if st.LastSyncedAt != nil {
		ago := time.Since(*st.LastSyncedAt)
		switch {
		case ago < time.Minute:
			return fmt.Sprintf("Jita synced just now (%d)", st.LastSyncedCount)
		case ago < time.Hour:
			return fmt.Sprintf("Jita synced %dm ago (%d)", int(ago.Minutes()), st.LastSyncedCount)
		default:
			return fmt.Sprintf("Jita synced %dh ago (%d)", int(ago.Hours()), st.LastSyncedCount)
		}
	}
	if st.Enabled {
		interval := domain.ClampPriceSyncInterval(st.IntervalMinutes)
		return fmt.Sprintf("Auto-Sync: Every %dm", interval)
	}
	return "Auto-Sync: Manual Only"
}

// PriceSyncResult summarizes one completed scoped Jita price sync cycle.
type PriceSyncResult struct {
	TotalTypeIDs   int   `json:"total_type_ids"`
	WarmedCount    int   `json:"warmed_count"`
	CatalogUpdated int   `json:"catalog_updated"`
	DurationMs     int64 `json:"duration_ms"`
	SkippedBusy    bool  `json:"skipped_busy"`
}

type priceSyncState struct {
	mu                   sync.RWMutex
	running              bool
	lastSyncedAt         *time.Time
	lastSyncedCount      int
	catalogUpdated       int
	lastDurationMs       int64
	lastError            string
	nextSyncAt           *time.Time
	lastGlobalSnapshotAt time.Time
	wakeCh               chan struct{}
}

func newPriceSyncState() *priceSyncState {
	return &priceSyncState{
		wakeCh: make(chan struct{}, 1),
	}
}

func (p *priceSyncState) notifyConfigChanged() {
	if p == nil {
		return
	}
	select {
	case p.wakeCh <- struct{}{}:
	default:
	}
}

// GetPriceSyncStatus returns a thread-safe snapshot of the background Jita price updater.
func (s *CopilotService) GetPriceSyncStatus(ctx context.Context) PriceSyncStatus {
	enabled := true
	interval := 15
	if cfg, err := s.GetSettings(ctx); err == nil && cfg != nil {
		enabled = cfg.AutoPriceSyncEnabled
		interval = domain.ClampPriceSyncInterval(cfg.AutoPriceSyncIntervalMinutes)
	}

	st := PriceSyncStatus{
		Enabled:         enabled,
		IntervalMinutes: interval,
	}
	if s.priceSync == nil {
		return st
	}

	s.priceSync.mu.RLock()
	defer s.priceSync.mu.RUnlock()
	st.IsRunning = s.priceSync.running
	st.LastSyncedCount = s.priceSync.lastSyncedCount
	st.CatalogUpdated = s.priceSync.catalogUpdated
	st.LastDurationMs = s.priceSync.lastDurationMs
	st.LastError = s.priceSync.lastError
	if s.priceSync.lastSyncedAt != nil {
		t := *s.priceSync.lastSyncedAt
		st.LastSyncedAt = &t
	}
	if s.priceSync.nextSyncAt != nil {
		t := *s.priceSync.nextSyncAt
		st.NextSyncAt = &t
	}
	return st
}

// CollectScopedJitaTypeIDs gathers the deduplicated set of active business TypeIDs:
// 1. User Catalog items (market_items)
// 2. All producible Planetary Industry (PI) P1–P4 items
// 3. Active SDE Blueprints & their required materials
func (s *CopilotService) CollectScopedJitaTypeIDs(ctx context.Context) []int32 {
	seen := make(map[int32]struct{}, 200)
	addID := func(id int32) {
		if id > 0 {
			seen[id] = struct{}{}
		}
	}

	// 1. Catalog items
	if s.items != nil {
		if items, err := s.items.ListItems(ctx); err == nil {
			for _, it := range items {
				addID(it.TypeID)
			}
		}
	}

	// 2. All producible PI items (P1–P4)
	for _, piItem := range domain.ListProduciblePICommodities("", "") {
		addID(piItem.TypeID)
	}

	// 2b. Dashboard Live Market Pulse benchmarks (8 Minerals + 6 PI + 6 Macro/Fuel)
	for _, id := range AllMarketPulseTypeIDs() {
		addID(id)
	}

	// 3. Active SDE Blueprints & their materials
	if s.sde != nil {
		if bps, err := s.sde.SearchBlueprints(ctx, "", "", 100); err == nil {
			for _, bp := range bps {
				addID(bp.ProductTypeID)
				if mats, mErr := s.sde.GetBlueprintMaterials(ctx, bp.BlueprintTypeID); mErr == nil {
					for _, m := range mats {
						addID(m.MaterialTypeID)
					}
				}
			}
		}
	}

	ids := make([]int32, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// SyncScopedJitaPrices performs a cache-first Jita price refresh for Catalog items (persisting
// updated Sell/Buy/Volume & TrapGuard to market_items) and warms PostgreSQL esi_cache for all
// PI (P1–P4) and SDE Blueprint items so PI Hub, Scanner, Calculator, and Marketplace load instantly.
func (s *CopilotService) SyncScopedJitaPrices(ctx context.Context) (*PriceSyncResult, error) {
	if s.market == nil {
		return nil, fmt.Errorf("market provider unavailable")
	}

	if s.priceSync == nil {
		s.priceSync = newPriceSyncState()
	}

	s.priceSync.mu.Lock()
	if s.priceSync.running {
		s.priceSync.mu.Unlock()
		return &PriceSyncResult{SkippedBusy: true}, nil
	}
	s.priceSync.running = true
	s.priceSync.lastError = ""
	s.priceSync.mu.Unlock()

	started := time.Now()
	var syncErr error
	catalogUpdated := 0
	warmedCount := 0
	totalIDs := 0

	defer func() {
		elapsed := time.Since(started).Milliseconds()
		now := time.Now()
		s.priceSync.mu.Lock()
		s.priceSync.running = false
		s.priceSync.lastDurationMs = elapsed
		if syncErr != nil {
			s.priceSync.lastError = syncErr.Error()
		} else {
			s.priceSync.lastSyncedAt = &now
			s.priceSync.lastSyncedCount = warmedCount
			s.priceSync.catalogUpdated = catalogUpdated
			s.priceSync.lastError = ""
		}
		s.priceSync.mu.Unlock()
	}()

	// Step 1: Refresh Catalog items (updates market_items + 7d/30d volume + TrapGuard)
	if s.items != nil {
		cnt, err := s.SyncJitaPrices(ctx)
		if err != nil {
			if errors.Is(err, esi.ErrErrorLimitReached) || errors.Is(err, esi.ErrRateLimitCooldown) {
				syncErr = err
				return nil, err
			}
			// Non-fatal if catalog is empty or partial; record and continue warming PI/SDE
			log.Printf("[WARN] Scoped Jita sync: catalog refresh warning: %v", err)
		} else {
			catalogUpdated = cnt
		}
	}

	// Step 2: Warm PostgreSQL esi_cache for all scoped TypeIDs (Catalog + PI P1–P4 + SDE Blueprints)
	// Processed in gentle chunks of 20 so ESI traffic stays smooth and cancellable.
	allIDs := s.CollectScopedJitaTypeIDs(ctx)
	totalIDs = len(allIDs)

	const chunkSize = 20
	for i := 0; i < len(allIDs); i += chunkSize {
		if err := ctx.Err(); err != nil {
			syncErr = err
			return nil, err
		}
		end := i + chunkSize
		if end > len(allIDs) {
			end = len(allIDs)
		}
		prices, err := s.market.GetJitaPrices(ctx, allIDs[i:end])
		if err != nil {
			if errors.Is(err, esi.ErrErrorLimitReached) || errors.Is(err, esi.ErrRateLimitCooldown) {
				syncErr = err
				return nil, err
			}
			log.Printf("[WARN] Scoped Jita sync chunk %d-%d warning: %v", i, end, err)
			continue
		}
		warmedCount += len(prices)
	}

	// Step 3: Capture daily 1-call GET /markets/prices/ global snapshot (throttled to max 1x/hour, ESI-safe)
	if err := s.SyncGlobalPriceSnapshot(ctx, false); err != nil {
		if errors.Is(err, esi.ErrErrorLimitReached) || errors.Is(err, esi.ErrRateLimitCooldown) {
			syncErr = err
			return nil, err
		}
		log.Printf("[WARN] Global /markets/prices/ snapshot warning: %v", err)
	}

	return &PriceSyncResult{
		TotalTypeIDs:   totalIDs,
		WarmedCount:    warmedCount,
		CatalogUpdated: catalogUpdated,
		DurationMs:     time.Since(started).Milliseconds(),
	}, nil
}

// SyncGlobalPriceSnapshot fetches GET /markets/prices/ (1 single cached ESI call for ~15,000 items)
// and bulk-upserts today's UTC snapshot into PostgreSQL, throttled to at most once per hour.
func (s *CopilotService) SyncGlobalPriceSnapshot(ctx context.Context, force bool) error {
	if s.globalSnapshots == nil || s.market == nil {
		return nil
	}
	gp, ok := s.market.(domain.GlobalMarketPriceProvider)
	if !ok || gp == nil {
		return nil
	}

	if !force && s.priceSync != nil {
		s.priceSync.mu.RLock()
		lastAt := s.priceSync.lastGlobalSnapshotAt
		s.priceSync.mu.RUnlock()
		if !lastAt.IsZero() && time.Since(lastAt) < time.Hour {
			return nil
		}
	}

	prices, err := gp.GetGlobalMarketPrices(ctx)
	if err != nil {
		return err
	}

	saved, err := s.globalSnapshots.UpsertDailySnapshot(ctx, time.Now().UTC(), prices)
	if err != nil {
		return err
	}
	_ = s.globalSnapshots.PruneOldSnapshots(ctx, 14)

	if s.priceSync != nil {
		s.priceSync.mu.Lock()
		s.priceSync.lastGlobalSnapshotAt = time.Now()
		s.priceSync.mu.Unlock()
	}
	log.Printf("[INFO] Global /markets/prices/ snapshot stored: %d EVE items for %s UTC.", saved, time.Now().UTC().Format("2006-01-02"))
	return nil
}

// StartBackgroundPriceUpdater launches a single supervised goroutine that periodically warms
// Jita prices in PostgreSQL esi_cache (Catalog + PI P1–P4 + SDE Blueprints) according to
// UserSettings.AutoPriceSyncEnabled and AutoPriceSyncIntervalMinutes.
func (s *CopilotService) StartBackgroundPriceUpdater(ctx context.Context) {
	if s.market == nil {
		return
	}
	if s.priceSync == nil {
		s.priceSync = newPriceSyncState()
	}

	go func() {
		log.Println("[INFO] Background Jita Price Updater started (Scope: Catalog + PI P1–P4 + SDE Blueprints).")

		// Short startup grace period so HTTP server is immediately responsive
		startupTimer := time.NewTimer(10 * time.Second)
		select {
		case <-ctx.Done():
			startupTimer.Stop()
			log.Println("[INFO] Background Jita Price Updater stopped.")
			return
		case <-startupTimer.C:
		case <-s.priceSync.wakeCh:
			startupTimer.Stop()
		}

		for {
			cfg, _ := s.GetSettings(ctx)
			enabled := true
			intervalMin := 15
			if cfg != nil {
				enabled = cfg.AutoPriceSyncEnabled
				intervalMin = domain.ClampPriceSyncInterval(cfg.AutoPriceSyncIntervalMinutes)
			}

			if enabled {
				runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				res, err := s.SyncScopedJitaPrices(runCtx)
				cancel()
				if err != nil {
					if ctx.Err() != nil {
						log.Println("[INFO] Background Jita Price Updater stopped.")
						return
					}
					log.Printf("[WARN] Background Jita Price Updater cycle failed: %v", err)
				} else if res != nil && !res.SkippedBusy {
					log.Printf("[INFO] Background Jita Price Updater synced %d/%d items (%d catalog) in %dms.",
						res.WarmedCount, res.TotalTypeIDs, res.CatalogUpdated, res.DurationMs)
				}
			}

			waitDur := time.Duration(intervalMin) * time.Minute
			if !enabled {
				// Re-check periodically or wake immediately when user updates Settings
				waitDur = 5 * time.Minute
				s.priceSync.mu.Lock()
				s.priceSync.nextSyncAt = nil
				s.priceSync.mu.Unlock()
			} else {
				next := time.Now().Add(waitDur)
				s.priceSync.mu.Lock()
				s.priceSync.nextSyncAt = &next
				s.priceSync.mu.Unlock()
			}

			timer := time.NewTimer(waitDur)
			select {
			case <-ctx.Done():
				timer.Stop()
				log.Println("[INFO] Background Jita Price Updater stopped.")
				return
			case <-timer.C:
			case <-s.priceSync.wakeCh:
				timer.Stop()
				// Settings changed — re-evaluate immediately on next loop iteration
			}
		}
	}()
}
