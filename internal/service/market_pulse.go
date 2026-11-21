package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
)

// MarketPulseCategory keys for the Dashboard Live Market Pulse widget.
const (
	PulseCategoryMinerals   = "minerals"
	PulseCategoryGas        = "gas"
	PulseCategoryPI         = "pi"
	PulseCategoryMacro      = "macro"
	PulseCategoryBlueprints = "blueprints"
	PulseCategoryGlobal     = "global"
)

// MarketPulseResponse bundles quotes plus metadata (sort mode & global snapshot stats) for the UI.
type MarketPulseResponse struct {
	Category    string                      `json:"category"`
	SortBy      string                      `json:"sort_by"`
	Hub         domain.TradeHub             `json:"hub"`
	Quotes      []MineralQuote              `json:"quotes"`
	GlobalStats *domain.GlobalSnapshotStats `json:"global_stats,omitempty"`
}

type pulseSeed struct {
	TypeID       int32
	Name         string
	FallbackSell float64
	FallbackBuy  float64
	Category     string
	IsPI         bool
}

var marketPulseSeeds = []pulseSeed{
	// 8 Core Minerals (T1 & T2 Manufacturing)
	{TypeID: 34, Name: "Tritanium", FallbackSell: 4.30, FallbackBuy: 3.95, Category: PulseCategoryMinerals},
	{TypeID: 35, Name: "Pyerite", FallbackSell: 11.50, FallbackBuy: 10.60, Category: PulseCategoryMinerals},
	{TypeID: 36, Name: "Mexallon", FallbackSell: 55.00, FallbackBuy: 51.00, Category: PulseCategoryMinerals},
	{TypeID: 37, Name: "Isogen", FallbackSell: 320.00, FallbackBuy: 295.00, Category: PulseCategoryMinerals},
	{TypeID: 38, Name: "Nocxium", FallbackSell: 780.00, FallbackBuy: 720.00, Category: PulseCategoryMinerals},
	{TypeID: 39, Name: "Zydrine", FallbackSell: 1650.00, FallbackBuy: 1520.00, Category: PulseCategoryMinerals},
	{TypeID: 40, Name: "Megacyte", FallbackSell: 2950.00, FallbackBuy: 2720.00, Category: PulseCategoryMinerals},
	{TypeID: 11399, Name: "Morphite", FallbackSell: 32500.00, FallbackBuy: 30200.00, Category: PulseCategoryMinerals},

	// 8 Core Gas Cloud Materials (Wormhole Fullerites + Low/Null Mykoserocin for T3, Boosters, Battleships & Reactions)
	{TypeID: 30370, Name: "Fullerite-C50", FallbackSell: 11500, FallbackBuy: 10400, Category: PulseCategoryGas},
	{TypeID: 30371, Name: "Fullerite-C60", FallbackSell: 12800, FallbackBuy: 11600, Category: PulseCategoryGas},
	{TypeID: 30372, Name: "Fullerite-C70", FallbackSell: 14200, FallbackBuy: 12900, Category: PulseCategoryGas},
	{TypeID: 30373, Name: "Fullerite-C72", FallbackSell: 16500, FallbackBuy: 15000, Category: PulseCategoryGas},
	{TypeID: 30377, Name: "Fullerite-C320", FallbackSell: 62000, FallbackBuy: 56500, Category: PulseCategoryGas},
	{TypeID: 30378, Name: "Fullerite-C540", FallbackSell: 74000, FallbackBuy: 67500, Category: PulseCategoryGas},
	{TypeID: 28694, Name: "Amber Mykoserocin", FallbackSell: 19500, FallbackBuy: 17600, Category: PulseCategoryGas},
	{TypeID: 28697, Name: "Golden Mykoserocin", FallbackSell: 28500, FallbackBuy: 25800, Category: PulseCategoryGas},

	// 6 High-Liquidity PI Benchmarks (P2 - P4)
	{TypeID: 3689, Name: "Mechanical Parts", FallbackSell: 11500, FallbackBuy: 10400, Category: PulseCategoryPI, IsPI: true},
	{TypeID: 9832, Name: "Coolant", FallbackSell: 10800, FallbackBuy: 9800, Category: PulseCategoryPI, IsPI: true},
	{TypeID: 44, Name: "Enriched Uranium", FallbackSell: 11200, FallbackBuy: 10100, Category: PulseCategoryPI, IsPI: true},
	{TypeID: 9848, Name: "Robotics", FallbackSell: 112000, FallbackBuy: 103000, Category: PulseCategoryPI, IsPI: true},
	{TypeID: 9834, Name: "Guidance Systems", FallbackSell: 95000, FallbackBuy: 87000, Category: PulseCategoryPI, IsPI: true},
	{TypeID: 2867, Name: "Wetware Mainframe", FallbackSell: 1950000, FallbackBuy: 1780000, Category: PulseCategoryPI, IsPI: true},

	// 6 Macro & Structure Fuel Benchmarks
	{TypeID: 44992, Name: "PLEX", FallbackSell: 5600000, FallbackBuy: 5450000, Category: PulseCategoryMacro},
	{TypeID: 40520, Name: "Large Skill Injector", FallbackSell: 920000000, FallbackBuy: 895000000, Category: PulseCategoryMacro},
	{TypeID: 4051, Name: "Nitrogen Fuel Block", FallbackSell: 24500, FallbackBuy: 22800, Category: PulseCategoryMacro},
	{TypeID: 4246, Name: "Hydrogen Fuel Block", FallbackSell: 24200, FallbackBuy: 22500, Category: PulseCategoryMacro},
	{TypeID: 4247, Name: "Helium Fuel Block", FallbackSell: 25100, FallbackBuy: 23400, Category: PulseCategoryMacro},
	{TypeID: 4312, Name: "Oxygen Fuel Block", FallbackSell: 24800, FallbackBuy: 23000, Category: PulseCategoryMacro},
}

// AllMarketPulseTypeIDs returns all 20 unique TypeIDs used by the fixed Dashboard Market Pulse tabs
// so the Background Jita Price Updater keeps them warm in PostgreSQL esi_cache.
func AllMarketPulseTypeIDs() []int32 {
	ids := make([]int32, 0, len(marketPulseSeeds))
	for _, seed := range marketPulseSeeds {
		ids = append(ids, seed.TypeID)
	}
	return ids
}

// NormalizePulseCategory returns a valid Market Pulse category key (defaults to "minerals").
func NormalizePulseCategory(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case PulseCategoryGas:
		return PulseCategoryGas
	case PulseCategoryPI:
		return PulseCategoryPI
	case PulseCategoryMacro:
		return PulseCategoryMacro
	case PulseCategoryBlueprints:
		return PulseCategoryBlueprints
	case PulseCategoryGlobal:
		return PulseCategoryGlobal
	default:
		return PulseCategoryMinerals
	}
}

// GetLiveMarketPulse returns the slice of Market Pulse quotes for the requested category and hub.
func (s *CopilotService) GetLiveMarketPulse(ctx context.Context, category string, hubKey string) []MineralQuote {
	return s.GetLiveMarketPulseView(ctx, category, hubKey, "demand").Quotes
}

// GetLiveMarketPulseView fetches cache-first hub prices, 7d/30d volume surge, 7-day trend signal,
// sparklines, and optional global snapshot stats for any Market Pulse tab.
func (s *CopilotService) GetLiveMarketPulseView(ctx context.Context, category string, hubKey string, sortBy string) MarketPulseResponse {
	cat := NormalizePulseCategory(category)
	hub := domain.LookupTradeHub(hubKey)
	sortKey := strings.ToLower(strings.TrimSpace(sortBy))
	if sortKey != "price" {
		sortKey = "demand"
	}

	resp := MarketPulseResponse{
		Category: cat,
		SortBy:   sortKey,
		Hub:      hub,
	}

	switch cat {
	case PulseCategoryBlueprints:
		resp.Quotes = s.getHotBlueprintsPulse(ctx, hub, sortKey)
		return resp
	case PulseCategoryGlobal:
		quotes, stats := s.getGlobalMoversPulse(ctx, hub)
		resp.Quotes = quotes
		resp.GlobalStats = stats
		return resp
	}

	// Fixed benchmark categories: Minerals (8), PI Benchmarks (6), Macro & Fuel (6)
	seeds := make([]pulseSeed, 0, 8)
	typeIDs := make([]int32, 0, 8)
	for _, seed := range marketPulseSeeds {
		if seed.Category == cat {
			seeds = append(seeds, seed)
			typeIDs = append(typeIDs, seed.TypeID)
		}
	}

	var livePrices map[int32]domain.JitaPrice
	if s.market != nil {
		if p, err := s.fetchHubPrices(ctx, hub, typeIDs); err == nil {
			livePrices = p
		}
	}

	historyByID := s.fetchPulseHistories(ctx, hub.RegionID, typeIDs, 7)

	out := make([]MineralQuote, 0, len(seeds))
	for _, seed := range seeds {
		q := MineralQuote{
			TypeID:     seed.TypeID,
			Name:       seed.Name,
			LowestSell: seed.FallbackSell,
			HighestBuy: seed.FallbackBuy,
			IsLive:     false,
			Category:   cat,
			HubKey:     hub.Key,
			HubName:    hub.Name,
			IsPI:       seed.IsPI,
			Signal:     "FAIR",
		}

		if livePrices != nil {
			if lp, ok := livePrices[seed.TypeID]; ok && lp.LowestSell > 0 {
				q.LowestSell = lp.LowestSell
				q.HighestBuy = lp.HighestBuy
				q.IsLive = true
			}
		}

		if q.LowestSell > 0 && q.HighestBuy > 0 && q.LowestSell >= q.HighestBuy {
			q.SpreadPercent = math.Round(((q.LowestSell-q.HighestBuy)/q.LowestSell)*1000) / 10
		}

		if hd, ok := historyByID[seed.TypeID]; ok && hd.avg7dPrice > 0 {
			q.Avg7dPrice = hd.avg7dPrice
			q.DailyVolume7d = hd.avg7dVol
			q.SparklinePoints = hd.sparkline
			if q.LowestSell > 0 {
				pct := ((q.LowestSell - hd.avg7dPrice) / hd.avg7dPrice) * 100.0
				q.Change7dPercent = math.Round(pct*10) / 10
				q.Signal = classifyPulseSignal(q.Change7dPercent)
			}
		}

		out = append(out, q)
	}
	resp.Quotes = out
	return resp
}

// getHotBlueprintsPulse dynamically ranks SDE manufacturing products (ships, drones, ammo, modules, rigs)
// by 7d-vs-30d volume surge ("demand") or 7d price gain ("price"), strictly cache-first & ESI-safe.
func (s *CopilotService) getHotBlueprintsPulse(ctx context.Context, hub domain.TradeHub, sortBy string) []MineralQuote {
	if s.sde == nil {
		return nil
	}

	bps, err := s.sde.SearchBlueprints(ctx, "", "", 40)
	if err != nil || len(bps) == 0 {
		return nil
	}

	// Deduplicate by ProductTypeID
	typeIDs := make([]int32, 0, len(bps))
	bpByProduct := make(map[int32]domain.SDEBlueprint, len(bps))
	for _, bp := range bps {
		if bp.ProductTypeID <= 0 {
			continue
		}
		if _, exists := bpByProduct[bp.ProductTypeID]; !exists {
			bpByProduct[bp.ProductTypeID] = bp
			typeIDs = append(typeIDs, bp.ProductTypeID)
		}
	}

	var livePrices map[int32]domain.JitaPrice
	if s.market != nil {
		if p, err := s.fetchHubPrices(ctx, hub, typeIDs); err == nil {
			livePrices = p
		}
	}

	// Fetch 30-day history (cache-first, max 4 workers, stops immediately if ESI rate limit hits)
	historyByID := s.fetchPulseHistories(ctx, hub.RegionID, typeIDs, 30)

	quotes := make([]MineralQuote, 0, len(typeIDs))
	for _, tid := range typeIDs {
		bp := bpByProduct[tid]
		q := MineralQuote{
			TypeID:            tid,
			Name:              bp.ProductName,
			Category:          PulseCategoryBlueprints,
			HubKey:            hub.Key,
			HubName:           hub.Name,
			BlueprintID:       bp.BlueprintTypeID,
			BlueprintCategory: bp.Category,
			Signal:            "FAIR",
		}

		if livePrices != nil {
			if lp, ok := livePrices[tid]; ok && lp.LowestSell > 0 {
				q.LowestSell = lp.LowestSell
				q.HighestBuy = lp.HighestBuy
				q.IsLive = true
			}
		}

		if q.LowestSell > 0 && q.HighestBuy > 0 && q.LowestSell >= q.HighestBuy {
			q.SpreadPercent = math.Round(((q.LowestSell-q.HighestBuy)/q.LowestSell)*1000) / 10
		}

		if hd, ok := historyByID[tid]; ok {
			q.Avg7dPrice = hd.avg7dPrice
			q.DailyVolume7d = hd.avg7dVol
			q.VolumeRatio = hd.volRatio
			q.SparklinePoints = hd.sparkline
			if hd.avg7dPrice > 0 && q.LowestSell > 0 {
				pct := ((q.LowestSell - hd.avg7dPrice) / hd.avg7dPrice) * 100.0
				q.Change7dPercent = math.Round(pct*10) / 10
			}
			// Prioritize HOT badge when 7d volume surges >= 1.25x above 30d average
			if q.VolumeRatio >= 1.25 && q.DailyVolume7d >= 10 {
				q.Signal = "HOT"
			} else if hd.avg7dPrice > 0 && q.LowestSell > 0 {
				q.Signal = classifyPulseSignal(q.Change7dPercent)
			}
		}

		if q.LowestSell > 0 || q.DailyVolume7d > 0 {
			quotes = append(quotes, q)
		}
	}

	sort.Slice(quotes, func(i, j int) bool {
		if sortBy == "price" {
			if quotes[i].Change7dPercent != quotes[j].Change7dPercent {
				return quotes[i].Change7dPercent > quotes[j].Change7dPercent
			}
			return quotes[i].DailyVolume7d > quotes[j].DailyVolume7d
		}
		// Default: "demand" — prioritize genuine volume surge ratio, then daily volume
		if quotes[i].VolumeRatio != quotes[j].VolumeRatio {
			return quotes[i].VolumeRatio > quotes[j].VolumeRatio
		}
		return quotes[i].DailyVolume7d > quotes[j].DailyVolume7d
	})

	if len(quotes) > 8 {
		quotes = quotes[:8]
	}
	return quotes
}

// getGlobalMoversPulse reads Top 8 day-over-day price risers from PostgreSQL global_market_price_snapshots
// (populated by 1-call GET /markets/prices/) and enriches ONLY those Top 8 winners via cache-first ESI.
func (s *CopilotService) getGlobalMoversPulse(ctx context.Context, hub domain.TradeHub) ([]MineralQuote, *domain.GlobalSnapshotStats) {
	if s.globalSnapshots == nil {
		return nil, &domain.GlobalSnapshotStats{}
	}

	stats, _ := s.globalSnapshots.GetSnapshotStats(ctx)
	if stats == nil {
		stats = &domain.GlobalSnapshotStats{}
	}
	if stats.DaysAvailable < 2 {
		return nil, stats
	}

	movers, err := s.globalSnapshots.ListTopPriceMovers(ctx, 8, 1000.0)
	if err != nil || len(movers) == 0 {
		return nil, stats
	}

	// Resolve any missing names in 1 single batch POST /universe/names/ call (at most 8 IDs)
	missingIDs := make([]int32, 0, len(movers))
	typeIDs := make([]int32, 0, len(movers))
	for _, m := range movers {
		typeIDs = append(typeIDs, m.TypeID)
		if strings.TrimSpace(m.Name) == "" {
			missingIDs = append(missingIDs, m.TypeID)
		}
	}
	resolvedNames := map[int32]string{}
	if len(missingIDs) > 0 {
		if gp, ok := s.market.(domain.GlobalMarketPriceProvider); ok && gp != nil {
			if names, nErr := gp.ResolveTypeNames(ctx, missingIDs); nErr == nil {
				resolvedNames = names
			}
		}
	}

	// Enrich ONLY the Top 8 winners with cache-first hub prices + 7d sparkline/volume
	var livePrices map[int32]domain.JitaPrice
	if s.market != nil {
		if p, pErr := s.fetchHubPrices(ctx, hub, typeIDs); pErr == nil {
			livePrices = p
		}
	}
	historyByID := s.fetchPulseHistories(ctx, hub.RegionID, typeIDs, 7)

	out := make([]MineralQuote, 0, len(movers))
	for _, m := range movers {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			if rn, ok := resolvedNames[m.TypeID]; ok && rn != "" {
				name = rn
			} else {
				name = fmt.Sprintf("Type #%d", m.TypeID)
			}
		}

		q := MineralQuote{
			TypeID:            m.TypeID,
			Name:              name,
			LowestSell:        m.CurrentAvg,
			Avg7dPrice:        m.PreviousAvg,
			Change7dPercent:   math.Round(m.ChangePercent*10) / 10,
			Signal:            "HIGH",
			Category:          PulseCategoryGlobal,
			HubKey:            hub.Key,
			HubName:           hub.Name,
			BlueprintCategory: m.Category,
			IsGlobalMover:     true,
		}

		if livePrices != nil {
			if lp, ok := livePrices[m.TypeID]; ok && lp.LowestSell > 0 {
				q.LowestSell = lp.LowestSell
				q.HighestBuy = lp.HighestBuy
				q.IsLive = true
			}
		}
		if q.LowestSell > 0 && q.HighestBuy > 0 && q.LowestSell >= q.HighestBuy {
			q.SpreadPercent = math.Round(((q.LowestSell-q.HighestBuy)/q.LowestSell)*1000) / 10
		}
		if hd, ok := historyByID[m.TypeID]; ok {
			q.DailyVolume7d = hd.avg7dVol
			q.SparklinePoints = hd.sparkline
		}

		out = append(out, q)
	}
	return out, stats
}

type pulseHistorySummary struct {
	avg7dPrice float64
	avg7dVol   float64
	avg30dVol  float64
	volRatio   float64
	sparkline  string
}

// fetchPulseHistories fetches cached market history in bounded 4-worker parallel and immediately
// aborts remaining items if ESI error limit or Retry-After cooldown triggers.
func (s *CopilotService) fetchPulseHistories(ctx context.Context, regionID int32, typeIDs []int32, days int) map[int32]pulseHistorySummary {
	out := make(map[int32]pulseHistorySummary, len(typeIDs))
	browser, ok := s.market.(domain.MarketBrowserProvider)
	if !ok || browser == nil || len(typeIDs) == 0 {
		return out
	}

	const maxWorkers = 4
	jobs := make(chan int32, len(typeIDs))
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		aborted atomic.Bool
	)
	workers := maxWorkers
	if len(typeIDs) < workers {
		workers = len(typeIDs)
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tid := range jobs {
				if aborted.Load() || ctx.Err() != nil {
					return
				}
				pts, err := browser.GetMarketHistory(ctx, regionID, tid, days)
				if err != nil {
					if errors.Is(err, esi.ErrErrorLimitReached) || errors.Is(err, esi.ErrRateLimitCooldown) {
						aborted.Store(true)
						return
					}
					continue
				}
				if len(pts) == 0 {
					continue
				}
				summary := computePulseHistorySummary(pts)
				mu.Lock()
				out[tid] = summary
				mu.Unlock()
			}
		}()
	}

	for _, tid := range typeIDs {
		jobs <- tid
	}
	close(jobs)
	wg.Wait()
	return out
}

// classifyPulseSignal turns % difference vs 7-day average into a beginner-friendly action badge.
func classifyPulseSignal(changePct float64) string {
	switch {
	case changePct <= -3.0:
		return "CHEAP"
	case changePct >= 3.0:
		return "HIGH"
	default:
		return "FAIR"
	}
}

// computePulseHistorySummary computes 7d average price, 7d vs 30d daily volume ratio, and a 7-day SVG sparkline.
func computePulseHistorySummary(pts []domain.MarketHistoryPoint) pulseHistorySummary {
	if len(pts) == 0 {
		return pulseHistorySummary{}
	}

	// 30d average volume across all available points
	sum30Vol := 0.0
	for _, p := range pts {
		sum30Vol += float64(p.Volume)
	}
	avg30Vol := sum30Vol / float64(len(pts))

	// Last 7 points for 7d average price, 7d average volume, and sparkline
	recent := pts
	if len(recent) > 7 {
		recent = recent[len(recent)-7:]
	}

	sum7Price := 0.0
	sum7Vol := 0.0
	minV := math.MaxFloat64
	maxV := -math.MaxFloat64
	validPrices := make([]float64, 0, len(recent))
	for _, p := range recent {
		sum7Vol += float64(p.Volume)
		if p.Average <= 0 {
			continue
		}
		validPrices = append(validPrices, p.Average)
		sum7Price += p.Average
		if p.Average < minV {
			minV = p.Average
		}
		if p.Average > maxV {
			maxV = p.Average
		}
	}

	avg7Vol := sum7Vol / float64(len(recent))
	volRatio := 0.0
	if avg30Vol > 0 {
		volRatio = math.Round((avg7Vol/avg30Vol)*100) / 100
	}

	var avg7Price float64
	var sparkline string
	if len(validPrices) > 0 {
		avg7Price = sum7Price / float64(len(validPrices))
		const (
			svgW = 72.0
			svgH = 20.0
			pad  = 2.0
		)
		if len(validPrices) == 1 {
			sparkline = fmt.Sprintf("%.1f,%.1f %.1f,%.1f", pad, svgH/2, svgW-pad, svgH/2)
		} else {
			span := maxV - minV
			coords := make([]string, 0, len(validPrices))
			stepX := (svgW - 2*pad) / float64(len(validPrices)-1)
			for i, v := range validPrices {
				x := pad + float64(i)*stepX
				y := svgH / 2
				if span > 0 {
					norm := (v - minV) / span
					y = (svgH - pad) - norm*(svgH-2*pad)
				}
				coords = append(coords, fmt.Sprintf("%.1f,%.1f", x, y))
			}
			sparkline = strings.Join(coords, " ")
		}
	}

	return pulseHistorySummary{
		avg7dPrice: avg7Price,
		avg7dVol:   math.Round(avg7Vol*10) / 10,
		avg30dVol:  math.Round(avg30Vol*10) / 10,
		volRatio:   volRatio,
		sparkline:  sparkline,
	}
}
