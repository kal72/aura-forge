package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"aura-forge/internal/domain"
)

// RadarSnapshot holds the latest background multi-category blueprint sweep results for the Dashboard.
type RadarSnapshot struct {
	HubKey           string                            `json:"hub_key"`
	HubName          string                            `json:"hub_name"`
	EvaluatedCount   int                               `json:"evaluated_count"`
	ProfitableCount  int                               `json:"profitable_count"`
	UpdatedAt        time.Time                         `json:"updated_at"`
	Running          bool                              `json:"running"`
	LastError        string                            `json:"last_error,omitempty"`
	TopOpportunities []domain.ManufacturingOpportunity `json:"top_opportunities"`
}

type opportunityRadarState struct {
	mu       sync.RWMutex
	snapshot RadarSnapshot
	running  bool
}

func newOpportunityRadarState() *opportunityRadarState {
	return &opportunityRadarState{}
}

func (s *CopilotService) ensureRadarState() *opportunityRadarState {
	if s.radar == nil {
		s.radar = newOpportunityRadarState()
	}
	return s.radar
}

// GetRadarSnapshot returns a thread-safe copy of the latest Background Top-Opportunities Radar snapshot (0ms).
func (s *CopilotService) GetRadarSnapshot(ctx context.Context) RadarSnapshot {
	st := s.ensureRadarState()
	st.mu.RLock()
	snap := st.snapshot
	snap.Running = st.running
	if len(st.snapshot.TopOpportunities) > 0 {
		snap.TopOpportunities = make([]domain.ManufacturingOpportunity, len(st.snapshot.TopOpportunities))
		copy(snap.TopOpportunities, st.snapshot.TopOpportunities)
	}
	st.mu.RUnlock()

	if snap.HubName == "" && s.settings != nil {
		if settings, err := s.GetSettings(ctx); err == nil && settings != nil {
			h := settings.ActiveTradeHub()
			snap.HubKey = h.Key
			snap.HubName = h.Name
		}
	}
	return snap
}

// TriggerRadarRefresh launches a non-blocking background sweep if one is not already running.
func (s *CopilotService) TriggerRadarRefresh() bool {
	if s.sde == nil || s.market == nil {
		return false
	}
	st := s.ensureRadarState()
	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		return false
	}
	st.running = true
	st.mu.Unlock()

	go func() {
		sweepCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		s.runRadarSweep(sweepCtx)
	}()
	return true
}

// StartBackgroundOpportunityRadar launches a supervised background goroutine that periodically sweeps
// up to 160 liquid SDE blueprints across all major categories (using cache-first ESI) so the Dashboard
// always displays fresh Top 8 Manufacturing Opportunities without blocking user page loads.
func (s *CopilotService) StartBackgroundOpportunityRadar(ctx context.Context) {
	if s.sde == nil || s.market == nil {
		return
	}
	s.ensureRadarState()

	go func() {
		// Wait 25s after startup so initial HTTP traffic and price cache warmup finish first
		select {
		case <-ctx.Done():
			return
		case <-time.After(25 * time.Second):
		}

		s.TriggerRadarRefresh()

		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.TriggerRadarRefresh()
			}
		}
	}()
}

// runRadarSweep evaluates up to 160 blueprints in bounded 40-blueprint chunks using cache-first ESI
// and stores the ranked Top 8 liquid opportunities in memory.
func (s *CopilotService) runRadarSweep(ctx context.Context) {
	st := s.ensureRadarState()
	defer func() {
		st.mu.Lock()
		st.running = false
		st.mu.Unlock()
	}()

	settings, err := s.GetSettings(ctx)
	if err != nil || settings == nil {
		st.mu.Lock()
		st.snapshot.LastError = "unable to load settings"
		st.mu.Unlock()
		return
	}
	hub := settings.ActiveTradeHub()
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)
	if availCap <= 0 {
		availCap = settings.StartingCapital
	}

	// Optional instant local PostgreSQL lookup of connected pilot's owned personal + corporation blueprints
	ownedByTypeID := make(map[int32]domain.CharacterBlueprint)
	mergeOwned := func(list []domain.CharacterBlueprint) {
		for _, pbp := range list {
			if pbp.TypeID <= 0 {
				continue
			}
			prev, exists := ownedByTypeID[pbp.TypeID]
			if !exists || pbp.MaterialEfficiency > prev.MaterialEfficiency ||
				(pbp.MaterialEfficiency == prev.MaterialEfficiency && pbp.TimeEfficiency > prev.TimeEfficiency) {
				ownedByTypeID[pbp.TypeID] = pbp
			}
		}
	}
	if s.chars != nil {
		mode := s.GetModeInfo(ctx)
		if mode.CharacterID > 0 {
			if pilotBPs, bErr := s.chars.ListBlueprints(ctx, mode.CharacterID); bErr == nil {
				mergeOwned(pilotBPs)
			}
			if corpRepo, ok := s.chars.(domain.CorpBlueprintRepository); ok && corpRepo != nil {
				if corpBPs, cErr := corpRepo.ListCorpBlueprints(ctx, mode.CharacterID); cErr == nil {
					mergeOwned(corpBPs)
				}
			}
		}
	}

	// Fetch New Eden global average prices once per sweep (cached) for Missing Material fallback & Price Spike Guard
	globalAvg := make(map[int32]float64)
	if gp, ok := s.market.(domain.GlobalMarketPriceProvider); ok && gp != nil {
		if list, gErr := gp.GetGlobalMarketPrices(ctx); gErr == nil {
			for _, item := range list {
				if item.AveragePrice > 0 {
					globalAvg[item.TypeID] = item.AveragePrice
				} else if item.AdjustedPrice > 0 {
					globalAvg[item.TypeID] = item.AdjustedPrice
				}
			}
		}
	}

	const maxRadarBlueprints = 160
	bps := s.collectRadarBlueprints(ctx, ownedByTypeID, maxRadarBlueprints)
	if len(bps) == 0 {
		st.mu.Lock()
		st.snapshot = RadarSnapshot{
			HubKey:    hub.Key,
			HubName:   hub.Name,
			UpdatedAt: time.Now(),
			LastError: "no SDE blueprints loaded yet",
		}
		st.mu.Unlock()
		return
	}

	allResults := make([]domain.ManufacturingOpportunity, 0, len(bps))
	const chunkSize = 40

	for start := 0; start < len(bps); start += chunkSize {
		if ctx.Err() != nil {
			break
		}
		end := start + chunkSize
		if end > len(bps) {
			end = len(bps)
		}
		chunk := bps[start:end]

		chunkRes := s.evaluateRadarChunk(ctx, chunk, hub, *settings, availCap, ownedByTypeID, globalAvg)
		allResults = append(allResults, chunkRes...)

		if end < len(bps) {
			select {
			case <-ctx.Done():
			case <-time.After(500 * time.Millisecond):
			}
		}
	}

	profitableCount := 0
	for _, r := range allResults {
		if r.NetProfit > 0 && r.Verdict != domain.VerdictAvoid && !r.IsPriceSpike && r.DailyVolume7d >= 5 {
			profitableCount++
		}
	}

	top := rankRadarOpportunities(allResults, 8)

	st.mu.Lock()
	st.snapshot = RadarSnapshot{
		HubKey:           hub.Key,
		HubName:          hub.Name,
		EvaluatedCount:   len(bps),
		ProfitableCount:  profitableCount,
		UpdatedAt:        time.Now(),
		TopOpportunities: top,
	}
	st.mu.Unlock()

	log.Printf("[INFO] Background Opportunity Radar finished for %s: evaluated=%d profitable=%d top=%d",
		hub.Name, len(bps), profitableCount, len(top))
}

// collectRadarBlueprints gathers up to maxTotal unique blueprints across Owned BPOs, User Catalog,
// curated popular T1/T2 items across all 6 categories, and starter pool (skipping unpriced Rare/Faction BPCs).
func (s *CopilotService) collectRadarBlueprints(ctx context.Context, owned map[int32]domain.CharacterBlueprint, maxTotal int) []domain.SDEBlueprint {
	selected := make([]domain.SDEBlueprint, 0, maxTotal)
	seen := make(map[int32]struct{}, maxTotal)
	addBP := func(bp domain.SDEBlueprint, allowRare bool) {
		if len(selected) >= maxTotal || bp.BlueprintTypeID <= 0 || bp.ProductTypeID <= 0 {
			return
		}
		if !allowRare && domain.DetectRareBPC(domain.MarketItem{Name: bp.ProductName}) {
			return
		}
		if _, ok := seen[bp.BlueprintTypeID]; ok {
			return
		}
		seen[bp.BlueprintTypeID] = struct{}{}
		selected = append(selected, bp)
	}

	// 1. Owned pilot & corp BPOs/BPCs first (allow rare BPCs if pilot actually owns them)
	for bpTypeID := range owned {
		if bp, err := s.sde.GetBlueprintByID(ctx, bpTypeID); err == nil && bp != nil {
			addBP(*bp, true)
		}
	}

	// 2. User catalog items
	if s.items != nil {
		if items, err := s.items.ListItems(ctx); err == nil {
			for _, it := range items {
				if it.TypeID <= 0 {
					continue
				}
				if bp, err := s.sde.GetBlueprintByProductID(ctx, it.TypeID); err == nil && bp != nil {
					addBP(*bp, false)
				}
			}
		}
	}

	// 3. Curated high-liquidity T1/T2 blueprints across all 6 core categories (avoids A–Z alphabetical trap)
	categories := []string{"Drones", "Ammunition", "Ships", "Modules", "Rigs", "Components"}
	for _, cat := range categories {
		if len(selected) >= maxTotal {
			break
		}
		for _, prodName := range popularCategoryBlueprints[cat] {
			if len(selected) >= maxTotal {
				break
			}
			if found, err := s.sde.SearchBlueprints(ctx, prodName, cat, 5); err == nil {
				for _, bp := range found {
					if strings.EqualFold(strings.TrimSpace(bp.ProductName), prodName) {
						addBP(bp, false)
						break
					}
				}
			}
		}
	}

	// 4. Starter liquid pool top-up
	for _, seed := range starterCatalogPool {
		if len(selected) >= maxTotal {
			break
		}
		if found, err := s.sde.SearchBlueprints(ctx, seed.ProductName, "", 3); err == nil {
			for _, bp := range found {
				if strings.EqualFold(strings.TrimSpace(bp.ProductName), seed.ProductName) {
					addBP(bp, false)
					break
				}
			}
		}
	}

	return selected
}

func (s *CopilotService) evaluateRadarChunk(
	ctx context.Context,
	bps []domain.SDEBlueprint,
	hub domain.TradeHub,
	settings domain.UserSettings,
	availCap float64,
	ownedByTypeID map[int32]domain.CharacterBlueprint,
	globalAvg map[int32]float64,
) []domain.ManufacturingOpportunity {
	matsByBP := make(map[int32][]domain.SDEMaterialRequirement, len(bps))
	seen := make(map[int32]struct{}, len(bps)*3)
	typeIDs := make([]int32, 0, len(bps)*3)
	productIDs := make([]int32, 0, len(bps))
	addID := func(id int32) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			typeIDs = append(typeIDs, id)
		}
	}

	for _, bp := range bps {
		addID(bp.ProductTypeID)
		productIDs = append(productIDs, bp.ProductTypeID)
		reqs, err := s.sde.GetBlueprintMaterials(ctx, bp.BlueprintTypeID)
		if err != nil || len(reqs) == 0 {
			continue
		}
		matsByBP[bp.BlueprintTypeID] = reqs
		for _, r := range reqs {
			addID(r.MaterialTypeID)
		}
	}

	var (
		prices   map[int32]domain.JitaPrice
		vols     map[int32]float64
		priceErr error
		wg       sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		prices, priceErr = s.fetchHubPrices(ctx, hub, typeIDs)
	}()
	go func() {
		defer wg.Done()
		vols = s.fetchHubDailyVolumesBatch(ctx, hub, productIDs)
	}()
	wg.Wait()

	if priceErr != nil || len(prices) == 0 {
		return nil
	}

	out := make([]domain.ManufacturingOpportunity, 0, len(bps))
	for _, bp := range bps {
		reqs := matsByBP[bp.BlueprintTypeID]
		if len(reqs) == 0 {
			continue
		}
		tech := bp.TechLevel
		if tech <= 0 {
			tech = 1
		}
		var ownedPtr *domain.CharacterBlueprint
		if owned, ok := ownedByTypeID[bp.BlueprintTypeID]; ok {
			c := owned
			ownedPtr = &c
		}
		me, te, ownedMatched, _ := domain.RecommendManufacturingMETE(tech, ownedPtr, -1, -1)

		// Dynamic relist scaling based on live hub competition
		sellOrders := prices[bp.ProductTypeID].SellOrdersCount
		estRelists := 1
		if sellOrders >= 40 {
			estRelists = 3
		} else if sellOrders >= 15 {
			estRelists = 2
		}

		// Conservative per-run T2 BPC / Invention cost floor when pilot does not own a matching T2 BPC
		needsInventionBPC := tech >= 2 && !ownedMatched
		perRunInventionFloor := 0.0
		if needsInventionBPC {
			if strings.EqualFold(bp.Category, "Ships") {
				perRunInventionFloor = 350_000.0
			} else {
				perRunInventionFloor = 150_000.0
			}
		}

		// Per-blueprint price map with globalAvg fallback for materials missing live orders at hub
		bpPrices := make(map[int32]domain.JitaPrice, len(reqs)+1)
		for _, r := range reqs {
			mp := prices[r.MaterialTypeID]
			if mp.LowestSell <= 0 && mp.HighestBuy <= 0 {
				if gAvg := globalAvg[r.MaterialTypeID]; gAvg > 0 {
					mp.LowestSell = gAvg
					mp.HighestBuy = gAvg
				}
			}
			bpPrices[r.MaterialTypeID] = mp
		}
		if sp, ok := prices[bp.ProductTypeID]; ok {
			bpPrices[bp.ProductTypeID] = sp
		}

		baseIn := domain.BlueprintCalcInput{
			BlueprintTypeID:     bp.BlueprintTypeID,
			Runs:                1,
			ME:                  me,
			TE:                  te,
			StructureKey:        "npc_station",
			HubKey:              hub.Key,
			MaterialPriceSource: "jita_sell",
			ExtraSetupCost:      perRunInventionFloor,
			SystemCostIndex:     settings.SystemCostIndex,
			FacilityTaxPercent:  settings.FacilityTaxPercent,
			FreightCostPerM3:    settings.FreightCostPerM3,
			BrokerFeePercent:    settings.BrokerFeePercent,
			SalesTaxPercent:     settings.SalesTaxPercent,
			RelistFeePercent:    settings.RelistFeePercent,
			EstimatedRelists:    estRelists,
			BatchesPerWeek:      4,
		}

		singleRes, sErr := domain.CalculateBlueprintManufacturing(bp, reqs, baseIn, bpPrices)
		if sErr != nil || singleRes.ProductUnitSellPrice <= 0 || singleRes.MissingMaterialPrices > 0 {
			continue
		}
		dailyVol := vols[bp.ProductTypeID]
		outPerRun := bp.ProductQtyPerRun
		if outPerRun <= 0 {
			outPerRun = 1
		}
		runs, runReason, autoSized := domain.RecommendManufacturingRuns(
			float64(singleRes.EstimatedDurationSeconds),
			singleRes.Profit.TotalDirectCost,
			outPerRun,
			dailyVol,
			availCap,
			0,
		)

		calcRes := singleRes
		if runs > 1 {
			baseIn.Runs = runs
			baseIn.ExtraSetupCost = perRunInventionFloor * float64(runs)
			r2, err2 := domain.CalculateBlueprintManufacturing(bp, reqs, baseIn, bpPrices)
			if err2 != nil || r2.ProductUnitSellPrice <= 0 || r2.MissingMaterialPrices > 0 {
				continue
			}
			calcRes = r2
		}

		verdict := domain.ClassifyManufacturingVerdict(calcRes.Profit.NetProfitPerBatch, calcRes.Profit.NetMarginPercent, calcRes.ISKPerHour)
		isSpike := false
		spikePct := 0.0
		if refPrice := globalAvg[bp.ProductTypeID]; refPrice > 0 && calcRes.ProductUnitSellPrice >= refPrice*1.35 {
			isSpike = true
			spikePct = ((calcRes.ProductUnitSellPrice - refPrice) / refPrice) * 100.0
		} else if tech == 1 && calcRes.Profit.NetMarginPercent >= 70.0 {
			isSpike = true
		}
		if isSpike && (verdict == domain.VerdictStrong || verdict == domain.VerdictModerate) {
			verdict = domain.VerdictWeak
		}

		fitsCap := availCap <= 0 || calcRes.Profit.TotalDirectCost <= availCap
		recSell := domain.RoundEVETick(calcRes.ProductUnitSellPrice * 0.999)
		if recSell <= 0 {
			recSell = calcRes.ProductUnitSellPrice
		}

		out = append(out, domain.ManufacturingOpportunity{
			BlueprintTypeID:   bp.BlueprintTypeID,
			ProductTypeID:     bp.ProductTypeID,
			ProductName:       bp.ProductName,
			Category:          bp.Category,
			TechLevel:         tech,
			HubKey:            hub.Key,
			HubName:           hub.Name,
			MaterialHubKey:    hub.Key,
			MaterialHubName:   hub.Name,
			Runs:              runs,
			ME:                me,
			TE:                te,
			OwnedBPO:          ownedMatched,
			AutoSized:         autoSized,
			RunSizingReason:   runReason,
			OutputQty:         calcRes.TotalOutputQty,
			MaterialCost:      calcRes.TotalMaterialCost,
			ProductUnitSell:   calcRes.ProductUnitSellPrice,
			RecommendedSell:   recSell,
			BreakEvenSell:     calcRes.Profit.BreakEvenUnitPrice,
			DailyVolume7d:     dailyVol,
			FitsCapital:       fitsCap,
			NetProfit:         calcRes.Profit.NetProfitPerBatch,
			NetMarginPercent:  calcRes.Profit.NetMarginPercent,
			ROIPercent:        calcRes.Profit.ROIPercent,
			ISKPerHour:        calcRes.ISKPerHour,
			DurationFormatted: calcRes.DurationFormatted,
			IsPriceSpike:      isSpike,
			PriceSpikePct:     spikePct,
			NeedsInventionBPC: needsInventionBPC,
			SellOrdersCount:   sellOrders,
			EstimatedRelists:  estRelists,
			Verdict:           verdict,
		})
	}
	return out
}

// rankRadarOpportunities filters out unprofitable/illiquid traps and returns the top N opportunities
// ranked by: FitsCapital & STRONG/MODERATE tier first, then ISK/Hour descending.
func rankRadarOpportunities(all []domain.ManufacturingOpportunity, limit int) []domain.ManufacturingOpportunity {
	if limit <= 0 {
		limit = 8
	}
	candidates := make([]domain.ManufacturingOpportunity, 0, len(all))
	for _, r := range all {
		if r.NetProfit <= 0 || r.Verdict == domain.VerdictAvoid || r.IsPriceSpike {
			continue
		}
		// Strictly skip dead/illiquid markets (< 5 units/day, including 0/day).
		if r.DailyVolume7d < 5 {
			continue
		}
		candidates = append(candidates, r)
	}

	verdictRank := func(v domain.OpportunityVerdict) int {
		switch v {
		case domain.VerdictStrong:
			return 0
		case domain.VerdictModerate:
			return 1
		case domain.VerdictWeak:
			return 2
		default:
			return 3
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].FitsCapital != candidates[j].FitsCapital {
			return candidates[i].FitsCapital
		}
		ri, rj := verdictRank(candidates[i].Verdict), verdictRank(candidates[j].Verdict)
		if ri != rj {
			return ri < rj
		}
		if candidates[i].ISKPerHour != candidates[j].ISKPerHour {
			return candidates[i].ISKPerHour > candidates[j].ISKPerHour
		}
		return candidates[i].NetMarginPercent > candidates[j].NetMarginPercent
	})

	if len(candidates) > limit {
		return candidates[:limit]
	}
	return candidates
}

// FormatRadarAge formats a human-friendly relative timestamp for the radar widget (e.g. "Just now", "4m ago").
func FormatRadarAge(t time.Time) string {
	if t.IsZero() {
		return "Pending first sweep"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "Just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
