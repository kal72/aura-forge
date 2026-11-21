package service

import (
	"context"
	"math"
	"sort"
	"strings"

	"aura-forge/internal/domain"
)

// CatalogRecommendation represents one capital-matched, Trap-Guard-verified starter item recommendation.
type CatalogRecommendation struct {
	TypeID                int32               `json:"type_id"`
	BlueprintTypeID       int32               `json:"blueprint_type_id"`
	Name                  string              `json:"name"`
	Category              domain.ItemCategory `json:"category"`
	TechLevel             int                 `json:"tech_level"`
	BatchOutputQty        int                 `json:"batch_output_qty"`
	JitaSell              float64             `json:"jita_sell"`
	JitaBuy               float64             `json:"jita_buy"`
	RecommendedSell       float64             `json:"recommended_sell"`
	Low7d                 float64             `json:"low_7d"`
	Avg7d                 float64             `json:"avg_7d"`
	High7d                float64             `json:"high_7d"`
	DailyVolume           float64             `json:"daily_volume"`
	MaterialCost          float64             `json:"material_cost"`
	NetProfit             float64             `json:"net_profit"`
	ROIPercent            float64             `json:"roi_percent"`
	MarginPercent         float64             `json:"margin_percent"`
	TrapStatus            domain.TrapStatus   `json:"trap_status"`
	PriceRegime           string              `json:"price_regime"`
	BeginnerReason        string              `json:"beginner_reason"`
	AlreadyInCatalog      bool                `json:"already_in_catalog"`
	IsFrequentlyDestroyed bool                `json:"is_frequently_destroyed"`
}

// CatalogRecommendResult holds the complete Auto-Recommend response for the user's current capital & active hub.
type CatalogRecommendResult struct {
	Hub              domain.TradeHub         `json:"hub"`
	AvailableCapital float64                 `json:"available_capital"`
	CapitalPhase     string                  `json:"capital_phase"`
	Items            []CatalogRecommendation `json:"items"`
	ViableCount      int                     `json:"viable_count"`
}

type starterCandidateSeed struct {
	ProductName           string
	Category              domain.ItemCategory
	MinCapital            float64
	DefaultRuns           int
	IsFrequentlyDestroyed bool
	BeginnerReason        string
}

var starterCatalogPool = []starterCandidateSeed{
	// T1 Drones — cheap NPC BPOs, massive daily PvP/PvE replacement demand
	{ProductName: "Hobgoblin I", Category: domain.CategoryDrones, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Cheap NPC BPO • Huge daily PvP/PvE loss rate • Fast cashflow"},
	{ProductName: "Hornet I", Category: domain.CategoryDrones, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Cheap NPC BPO • High-volume light combat drone"},
	{ProductName: "Warrior I", Category: domain.CategoryDrones, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Cheap NPC BPO • Fastest light drone, steady Jita turnover"},
	{ProductName: "Acolyte I", Category: domain.CategoryDrones, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Cheap NPC BPO • Popular armor/PvP light drone"},
	{ProductName: "Hammerhead I", Category: domain.CategoryDrones, MinCapital: 10_000_000, DefaultRuns: 10, IsFrequentlyDestroyed: true, BeginnerReason: "Cheap NPC BPO • Medium combat drone with solid ISK/batch"},

	// T1 Ammunition & Missiles — ultra-liquid consumables
	{ProductName: "Scourge Heavy Missile", Category: domain.CategoryAmmunition, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Consumable ammo • Millions traded daily in Jita"},
	{ProductName: "Antimatter Charge S", Category: domain.CategoryAmmunition, MinCapital: 0, DefaultRuns: 30, IsFrequentlyDestroyed: true, BeginnerReason: "Core blaster ammo • Ultra-fast liquidity for small capital"},
	{ProductName: "Antimatter Charge M", Category: domain.CategoryAmmunition, MinCapital: 0, DefaultRuns: 25, IsFrequentlyDestroyed: true, BeginnerReason: "Popular cruiser hybrid ammo • High daily turnover"},

	// T1 Starter Ships — constantly destroyed by newbros, miners & Faction Warfare
	{ProductName: "Venture", Category: domain.CategoryShips, MinCapital: 10_000_000, DefaultRuns: 5, IsFrequentlyDestroyed: true, BeginnerReason: "Top beginner mining/gas hull • Always in demand in Jita"},
	{ProductName: "Rifter", Category: domain.CategoryShips, MinCapital: 10_000_000, DefaultRuns: 5, IsFrequentlyDestroyed: true, BeginnerReason: "Iconic Faction Warfare frigate • High PvP destruction"},
	{ProductName: "Tristan", Category: domain.CategoryShips, MinCapital: 10_000_000, DefaultRuns: 5, IsFrequentlyDestroyed: true, BeginnerReason: "Popular drone frigate for PvP & PvE • High turnover"},
	{ProductName: "Heron", Category: domain.CategoryShips, MinCapital: 10_000_000, DefaultRuns: 5, IsFrequentlyDestroyed: true, BeginnerReason: "Top exploration frigate • Frequently lost in wormholes"},
	{ProductName: "Cormorant", Category: domain.CategoryShips, MinCapital: 25_000_000, DefaultRuns: 5, IsFrequentlyDestroyed: true, BeginnerReason: "Fleet destroyer staple • Strong volume & healthy batch size"},
	{ProductName: "Catalyst", Category: domain.CategoryShips, MinCapital: 25_000_000, DefaultRuns: 5, IsFrequentlyDestroyed: true, BeginnerReason: "Most-destroyed destroyer in New Eden • Relentless Jita demand"},

	// T1 Core Fitting Modules — used on almost every ship
	{ProductName: "Damage Control I", Category: domain.CategoryModules, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Fitted on nearly every ship • Cheap NPC BPO & fast fill"},
	{ProductName: "1MN Afterburner I", Category: domain.CategoryModules, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Core frigate propulsion module • Consistent beginner demand"},
	{ProductName: "Warp Scrambler I", Category: domain.CategoryModules, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Core PvP tackle module • High destruction rate"},
	{ProductName: "Medium Shield Extender I", Category: domain.CategoryModules, MinCapital: 10_000_000, DefaultRuns: 15, IsFrequentlyDestroyed: true, BeginnerReason: "Core shield tank module • Steady cruiser/BC turnover"},
	{ProductName: "Small Armor Repairer I", Category: domain.CategoryModules, MinCapital: 0, DefaultRuns: 20, IsFrequentlyDestroyed: true, BeginnerReason: "Core frigate repairer • Low capital, steady sales"},

	// T2 Scale-Up Candidates (unlocked automatically at >= 250M ISK capital)
	{ProductName: "Hobgoblin II", Category: domain.CategoryDrones, MinCapital: 250_000_000, DefaultRuns: 10, IsFrequentlyDestroyed: true, BeginnerReason: "T2 Scale-Up • Best starter T2 invention drone with massive volume"},
	{ProductName: "Hammerhead II", Category: domain.CategoryDrones, MinCapital: 250_000_000, DefaultRuns: 10, IsFrequentlyDestroyed: true, BeginnerReason: "T2 Scale-Up • High-value medium T2 drone with strong margin"},
	{ProductName: "Damage Control II", Category: domain.CategoryModules, MinCapital: 250_000_000, DefaultRuns: 10, IsFrequentlyDestroyed: true, BeginnerReason: "T2 Scale-Up • Most fitted module in EVE Online"},
	{ProductName: "Warp Scrambler II", Category: domain.CategoryModules, MinCapital: 250_000_000, DefaultRuns: 10, IsFrequentlyDestroyed: true, BeginnerReason: "T2 Scale-Up • Core PvP module with fast Jita velocity"},
}

func resolveCapitalPhaseLabel(cap float64) string {
	switch {
	case cap < 100_000_000:
		return "Tier 1: Seed Capital (<100M ISK) — Focus on Fast T1 Drones, Ammo & Core Modules"
	case cap < 1_000_000_000:
		return "Tier 2: First Billion (100M–1B ISK) — T1 Frigates/Destroyers + Starter T2 Scaling"
	case cap < 10_000_000_000:
		return "Tier 3: Scale-Up (1B–10B ISK) — Multi-Slot T1/T2 High-Velocity Lines"
	default:
		return "Tier 4+: Market Empire (10B+ ISK) — High-Volume T2 & Industrial Scaling"
	}
}

// RecommendCatalogForCapital evaluates curated beginner-safe SDE blueprints against live prices
// at the user's active Trade Hub, 7d history, and Available Capital to suggest the top VIABLE items.
func (s *CopilotService) RecommendCatalogForCapital(ctx context.Context) (*CatalogRecommendResult, error) {
	settings, _ := s.GetSettings(ctx)
	hub := settings.ActiveTradeHub()
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)
	if availCap <= 0 {
		availCap = settings.StartingCapital
	}

	res := &CatalogRecommendResult{
		Hub:              hub,
		AvailableCapital: availCap,
		CapitalPhase:     resolveCapitalPhaseLabel(availCap),
		Items:            make([]CatalogRecommendation, 0),
	}

	if s.sde == nil {
		return res, nil
	}

	// Track items already in the user's catalog
	existingByType := make(map[int32]bool)
	existingByName := make(map[string]bool)
	if s.items != nil {
		if existing, err := s.items.ListItems(ctx); err == nil {
			for _, it := range existing {
				if it.TypeID > 0 {
					existingByType[it.TypeID] = true
				}
				existingByName[strings.ToLower(strings.TrimSpace(it.Name))] = true
			}
		}
	}

	type resolvedCand struct {
		seed starterCandidateSeed
		bp   domain.SDEBlueprint
		mats []domain.SDEMaterialRequirement
	}

	candidates := make([]resolvedCand, 0, len(starterCatalogPool))
	priceIDSet := make(map[int32]struct{})

	for _, seed := range starterCatalogPool {
		if availCap < seed.MinCapital {
			continue
		}
		found, err := s.sde.SearchBlueprints(ctx, seed.ProductName, "", 5)
		if err != nil || len(found) == 0 {
			continue
		}
		var matched *domain.SDEBlueprint
		for i := range found {
			if strings.EqualFold(strings.TrimSpace(found[i].ProductName), seed.ProductName) {
				matched = &found[i]
				break
			}
		}
		if matched == nil {
			continue
		}
		mats, err := s.sde.GetBlueprintMaterials(ctx, matched.BlueprintTypeID)
		if err != nil || len(mats) == 0 {
			continue
		}
		candidates = append(candidates, resolvedCand{seed: seed, bp: *matched, mats: mats})
		priceIDSet[matched.ProductTypeID] = struct{}{}
		for _, m := range mats {
			if m.MaterialTypeID > 0 {
				priceIDSet[m.MaterialTypeID] = struct{}{}
			}
		}
	}

	if len(candidates) == 0 {
		return res, nil
	}

	// Single cache-first batched hub price fetch for all candidate products + materials
	prices := make(map[int32]domain.JitaPrice)
	if s.market != nil && len(priceIDSet) > 0 {
		ids := make([]int32, 0, len(priceIDSet))
		for id := range priceIDSet {
			ids = append(ids, id)
		}
		if fetched, err := s.fetchHubPrices(ctx, hub, ids); err == nil {
			prices = fetched
		}
	}

	for _, c := range candidates {
		jp := prices[c.bp.ProductTypeID]
		if jp.LowestSell <= 0 && jp.HighestBuy <= 0 {
			continue
		}

		qtyPerRun := c.bp.ProductQtyPerRun
		if qtyPerRun <= 0 {
			qtyPerRun = 1
		}
		runs := c.seed.DefaultRuns
		if runs < 1 {
			runs = 1
		}

		defaultME := 10
		if c.bp.TechLevel >= 2 {
			defaultME = 2
		}

		// Compute 1-run material cost first, then scale runs so batch stays safely <= 35% of available capital
		computeCost := func(r int) float64 {
			total := 0.0
			for _, m := range c.mats {
				adj := domain.ComputeAdjustedMaterialQty(m.BaseQuantity, r, defaultME, 0)
				mp := prices[m.MaterialTypeID]
				unit := mp.LowestSell
				if unit <= 0 {
					unit = mp.HighestBuy
				}
				total += float64(adj) * unit
			}
			return math.Round(total*100) / 100
		}

		matCost := computeCost(runs)
		if availCap > 0 && matCost > availCap*0.35 && runs > 1 {
			oneRunCost := computeCost(1)
			if oneRunCost > 0 {
				safeRuns := int(math.Floor((availCap * 0.35) / oneRunCost))
				if safeRuns < 1 {
					safeRuns = 1
				}
				if safeRuns < runs {
					runs = safeRuns
					matCost = computeCost(runs)
				}
			}
		}
		if matCost <= 0 {
			continue
		}

		batchQty := runs * qtyPerRun
		dailyVol, _ := s.fetchHubDailyVolume(ctx, hub, c.bp.ProductTypeID)
		low7d, avg7d, high7d := s.fetchHub7dRange(ctx, hub, c.bp.ProductTypeID)
		pi := ComputePriceIntelligence(jp.LowestSell, jp.HighestBuy, low7d, avg7d, high7d, matCost, batchQty, *settings)

		evalSell := jp.LowestSell
		if evalSell <= 0 {
			evalSell = pi.RecommendedSellPrice
		}

		candidateItem := domain.MarketItem{
			TypeID:                c.bp.ProductTypeID,
			Name:                  c.bp.ProductName,
			Category:              c.seed.Category,
			EstimatedJitaSell:     evalSell,
			EstimatedJitaBuy:      jp.HighestBuy,
			DailyVolume:           dailyVol,
			MaterialCost:          matCost,
			BatchOutputQty:        batchQty,
			ProductionTimeSeconds: c.bp.BaseTimeSeconds * runs,
			IsManufacturable:      true,
			IsFrequentlyDestroyed: c.seed.IsFrequentlyDestroyed,
			TrapAnalysis:          domain.TrapReport{BOMSyncedFromSDE: true},
		}

		report := domain.EvaluateTrapGuardWithHistory(candidateItem, *settings, availCap, avg7d)
		inCatalog := existingByType[c.bp.ProductTypeID] || existingByName[strings.ToLower(c.bp.ProductName)]

		rec := CatalogRecommendation{
			TypeID:                c.bp.ProductTypeID,
			BlueprintTypeID:       c.bp.BlueprintTypeID,
			Name:                  c.bp.ProductName,
			Category:              c.seed.Category,
			TechLevel:             c.bp.TechLevel,
			BatchOutputQty:        batchQty,
			JitaSell:              evalSell,
			JitaBuy:               jp.HighestBuy,
			RecommendedSell:       pi.RecommendedSellPrice,
			Low7d:                 low7d,
			Avg7d:                 avg7d,
			High7d:                high7d,
			DailyVolume:           dailyVol,
			MaterialCost:          matCost,
			NetProfit:             report.EstimatedNetProfit,
			ROIPercent:            report.EstimatedROIPct,
			MarginPercent:         report.EstimatedMarginPct,
			TrapStatus:            report.Status,
			PriceRegime:           pi.PriceRegime,
			BeginnerReason:        c.seed.BeginnerReason,
			AlreadyInCatalog:      inCatalog,
			IsFrequentlyDestroyed: c.seed.IsFrequentlyDestroyed,
		}
		if report.Status == domain.TrapStatusLayak {
			res.ViableCount++
		}
		res.Items = append(res.Items, rec)
	}

	// Sort: VIABLE first, then not-yet-in-catalog, then highest ROI %
	statusRank := func(st domain.TrapStatus) int {
		switch st {
		case domain.TrapStatusLayak:
			return 1
		case domain.TrapStatusWaspadaNiche:
			return 2
		case domain.TrapStatusWaspadaModal:
			return 3
		default:
			return 4
		}
	}
	sort.SliceStable(res.Items, func(i, j int) bool {
		ri, rj := statusRank(res.Items[i].TrapStatus), statusRank(res.Items[j].TrapStatus)
		if ri != rj {
			return ri < rj
		}
		if res.Items[i].AlreadyInCatalog != res.Items[j].AlreadyInCatalog {
			return !res.Items[i].AlreadyInCatalog
		}
		if res.Items[i].ROIPercent != res.Items[j].ROIPercent {
			return res.Items[i].ROIPercent > res.Items[j].ROIPercent
		}
		return res.Items[i].DailyVolume > res.Items[j].DailyVolume
	})

	if len(res.Items) > 8 {
		res.Items = res.Items[:8]
	}
	return res, nil
}
