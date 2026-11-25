package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"aura-forge/internal/domain"
)

// ManufacturingScanRequest encapsulates all filters for Opportunity Scanner v2 Tab 1 (Manufacturing ISK/hr).
type ManufacturingScanRequest struct {
	HubKey              string
	MaterialHubKey      string // Optional: separate hub to buy materials (blank = same as HubKey)
	Category            string
	TechLevel           int // 0 = All, 1 = T1, 2 = T2, 3 = T3
	SearchQuery         string
	Runs                int
	ME                  int
	TE                  int
	StructureKey        string
	MaterialPriceSource string
	ProfitableOnly      bool
	FitsCapitalOnly     bool
	SortBy              string // isk_hr | roi | profit | volume
}

// scannerBasketItem defines a high-liquidity EVE item used for Station Trading and Inter-Hub Arbitrage scans.
type scannerBasketItem struct {
	TypeID   int32
	Name     string
	Category string
	VolumeM3 float64
}

var defaultStationTradeBasket = []scannerBasketItem{
	// Commodities & Macro
	{TypeID: 44992, Name: "PLEX", Category: "Commodities", VolumeM3: 0.01},
	{TypeID: 40520, Name: "Large Skill Injector", Category: "Commodities", VolumeM3: 0.01},
	// PI & Structure Fuel
	{TypeID: 9848, Name: "Robotics", Category: "PI & Fuel", VolumeM3: 6.0},
	{TypeID: 9834, Name: "Guidance Systems", Category: "PI & Fuel", VolumeM3: 6.0},
	{TypeID: 3689, Name: "Mechanical Parts", Category: "PI & Fuel", VolumeM3: 1.5},
	{TypeID: 9832, Name: "Coolant", Category: "PI & Fuel", VolumeM3: 1.5},
	{TypeID: 44, Name: "Enriched Uranium", Category: "PI & Fuel", VolumeM3: 1.5},
	{TypeID: 4051, Name: "Nitrogen Fuel Block", Category: "PI & Fuel", VolumeM3: 5.0},
	{TypeID: 4246, Name: "Hydrogen Fuel Block", Category: "PI & Fuel", VolumeM3: 5.0},
	{TypeID: 4247, Name: "Helium Fuel Block", Category: "PI & Fuel", VolumeM3: 5.0},
	{TypeID: 4312, Name: "Oxygen Fuel Block", Category: "PI & Fuel", VolumeM3: 5.0},
	// Drones
	{TypeID: 2454, Name: "Hobgoblin I", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2456, Name: "Hobgoblin II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2486, Name: "Warrior I", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2488, Name: "Warrior II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2183, Name: "Hammerhead I", Category: "Drones", VolumeM3: 10.0},
	{TypeID: 2185, Name: "Hammerhead II", Category: "Drones", VolumeM3: 10.0},
	{TypeID: 2203, Name: "Acolyte II", Category: "Drones", VolumeM3: 5.0},
	// Modules
	{TypeID: 2046, Name: "Damage Control I", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 2048, Name: "Damage Control II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 3841, Name: "Large Shield Extender II", Category: "Modules", VolumeM3: 25.0},
	{TypeID: 3831, Name: "Medium Shield Extender II", Category: "Modules", VolumeM3: 10.0},
	{TypeID: 5975, Name: "5MN Y-T8 Compact Microwarpdrive", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 527, Name: "Stasis Webifier I", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 526, Name: "Stasis Webifier II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 3244, Name: "Warp Scrambler II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 3242, Name: "Warp Disruptor II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 1952, Name: "Sensor Booster II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 2205, Name: "Ballistic Control System II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 519, Name: "Gyrostabilizer II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 10190, Name: "Magnetic Field Stabilizer II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 2364, Name: "Heat Sink II", Category: "Modules", VolumeM3: 5.0},
	// Ships
	{TypeID: 32880, Name: "Venture", Category: "Ships", VolumeM3: 2500.0},
	{TypeID: 587, Name: "Rifter", Category: "Ships", VolumeM3: 2500.0},
	{TypeID: 16240, Name: "Catalyst", Category: "Ships", VolumeM3: 5000.0},
	{TypeID: 16242, Name: "Thrasher", Category: "Ships", VolumeM3: 5000.0},
	{TypeID: 621, Name: "Caracal", Category: "Ships", VolumeM3: 10000.0},
}

// Compact, high-demand items suitable for beginner high-sec hauling arbitrage.
var defaultArbitrageBasket = []scannerBasketItem{
	// Compact T2 & Core Modules (5–25 m3)
	{TypeID: 2048, Name: "Damage Control II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 3841, Name: "Large Shield Extender II", Category: "Modules", VolumeM3: 25.0},
	{TypeID: 3831, Name: "Medium Shield Extender II", Category: "Modules", VolumeM3: 10.0},
	{TypeID: 3244, Name: "Warp Scrambler II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 3242, Name: "Warp Disruptor II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 526, Name: "Stasis Webifier II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 2205, Name: "Ballistic Control System II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 519, Name: "Gyrostabilizer II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 10190, Name: "Magnetic Field Stabilizer II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 2364, Name: "Heat Sink II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 1952, Name: "Sensor Booster II", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 5975, Name: "5MN Y-T8 Compact Microwarpdrive", Category: "Modules", VolumeM3: 5.0},
	{TypeID: 2046, Name: "Damage Control I", Category: "Modules", VolumeM3: 5.0},
	// Drones (5–10 m3)
	{TypeID: 2456, Name: "Hobgoblin II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2488, Name: "Warrior II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2185, Name: "Hammerhead II", Category: "Drones", VolumeM3: 10.0},
	{TypeID: 2203, Name: "Acolyte II", Category: "Drones", VolumeM3: 5.0},
	{TypeID: 2454, Name: "Hobgoblin I", Category: "Drones", VolumeM3: 5.0},
	// PI & Fuel (1.5–6 m3)
	{TypeID: 9848, Name: "Robotics", Category: "PI & Fuel", VolumeM3: 6.0},
	{TypeID: 9834, Name: "Guidance Systems", Category: "PI & Fuel", VolumeM3: 6.0},
	{TypeID: 3689, Name: "Mechanical Parts", Category: "PI & Fuel", VolumeM3: 1.5},
	{TypeID: 9832, Name: "Coolant", Category: "PI & Fuel", VolumeM3: 1.5},
	{TypeID: 44, Name: "Enriched Uranium", Category: "PI & Fuel", VolumeM3: 1.5},
	{TypeID: 4051, Name: "Nitrogen Fuel Block", Category: "PI & Fuel", VolumeM3: 5.0},
	{TypeID: 4246, Name: "Hydrogen Fuel Block", Category: "PI & Fuel", VolumeM3: 5.0},
	{TypeID: 4247, Name: "Helium Fuel Block", Category: "PI & Fuel", VolumeM3: 5.0},
	{TypeID: 4312, Name: "Oxygen Fuel Block", Category: "PI & Fuel", VolumeM3: 5.0},
	// High-value Commodities
	{TypeID: 44992, Name: "PLEX", Category: "Commodities", VolumeM3: 0.01},
}

// fetchHubDailyVolumesBatch fetches 7-day average daily volumes for a slice of typeIDs using 4 bounded workers.
func (s *CopilotService) fetchHubDailyVolumesBatch(ctx context.Context, hub domain.TradeHub, typeIDs []int32) map[int32]float64 {
	vols := make(map[int32]float64, len(typeIDs))
	if s.market == nil || len(typeIDs) == 0 {
		return vols
	}
	const maxWorkers = 4
	sem := make(chan struct{}, maxWorkers)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for _, id := range typeIDs {
		if id <= 0 {
			continue
		}
		wg.Add(1)
		go func(tid int32) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			avg7, _ := s.fetchHubDailyVolume(ctx, hub, tid)
			mu.Lock()
			vols[tid] = avg7
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return vols
}

// ScanManufacturingOpportunities is a backward-compatible wrapper delegating to ScanManufacturingOpportunitiesV2.
func (s *CopilotService) ScanManufacturingOpportunities(
	ctx context.Context,
	category string,
	runs, me, te int,
	structureKey, materialPriceSource string,
) ([]domain.ManufacturingOpportunity, error) {
	return s.ScanManufacturingOpportunitiesV2(ctx, ManufacturingScanRequest{
		Category:            category,
		Runs:                runs,
		ME:                  me,
		TE:                  te,
		StructureKey:        structureKey,
		MaterialPriceSource: materialPriceSource,
	})
}

// selectBlueprintsForScan picks up to maxLimit relevant SDE blueprints without falling into the
// alphabetical "A–Z first 100" trap after importing the full 5,000+ Official CCP SDE.
func (s *CopilotService) selectBlueprintsForScan(ctx context.Context, req ManufacturingScanRequest, maxLimit int) ([]domain.SDEBlueprint, error) {
	q := strings.TrimSpace(req.SearchQuery)
	cat := strings.TrimSpace(req.Category)

	// Case 1: Specific search keyword or category selected — query the full SDE directly (paged + tech filter)
	if q != "" || cat != "" {
		if ext, ok := s.sde.(domain.SDEExtendedRepository); ok {
			pageRes, err := ext.SearchBlueprintsPaged(ctx, q, cat, req.TechLevel, 1, maxLimit)
			if err == nil && pageRes != nil {
				return pageRes.Items, nil
			}
		}
		raw, err := s.sde.SearchBlueprints(ctx, q, cat, maxLimit*2)
		if err != nil {
			return nil, err
		}
		out := make([]domain.SDEBlueprint, 0, maxLimit)
		for _, bp := range raw {
			if req.TechLevel > 0 && bp.TechLevel != req.TechLevel {
				continue
			}
			out = append(out, bp)
			if len(out) >= maxLimit {
				break
			}
		}
		return out, nil
	}

	// Case 2: Default "All Blueprints" scan — prioritize high-liquidity starter pool + user's own Catalog items
	// so popular items (Hobgoblin, Rifter, Venture, Scourge, Damage Control, etc.) are always evaluated!
	selected := make([]domain.SDEBlueprint, 0, maxLimit)
	seenBP := make(map[int32]struct{}, maxLimit)
	addBP := func(bp domain.SDEBlueprint) {
		if len(selected) >= maxLimit {
			return
		}
		if req.TechLevel > 0 && bp.TechLevel != req.TechLevel {
			return
		}
		if _, exists := seenBP[bp.BlueprintTypeID]; exists {
			return
		}
		seenBP[bp.BlueprintTypeID] = struct{}{}
		selected = append(selected, bp)
	}

	// 2a. User Catalog items with SDE blueprints first
	if s.items != nil {
		if catItems, err := s.items.ListItems(ctx); err == nil {
			for _, it := range catItems {
				if it.TypeID <= 0 {
					continue
				}
				if bp, err := s.sde.GetBlueprintByProductID(ctx, it.TypeID); err == nil && bp != nil {
					addBP(*bp)
				}
			}
		}
	}

	// 2b. Curated popular T1/T2 starter blueprints
	for _, seed := range starterCatalogPool {
		if len(selected) >= maxLimit {
			break
		}
		found, err := s.sde.SearchBlueprints(ctx, seed.ProductName, "", 5)
		if err != nil {
			continue
		}
		for _, bp := range found {
			if strings.EqualFold(strings.TrimSpace(bp.ProductName), seed.ProductName) {
				addBP(bp)
				break
			}
		}
	}

	// 2c. Top up remaining slots from SDE (respecting TechLevel)
	if len(selected) < maxLimit {
		if ext, ok := s.sde.(domain.SDEExtendedRepository); ok {
			if pageRes, err := ext.SearchBlueprintsPaged(ctx, "", "", req.TechLevel, 1, maxLimit); err == nil && pageRes != nil {
				for _, bp := range pageRes.Items {
					addBP(bp)
				}
			}
		} else if extra, err := s.sde.SearchBlueprints(ctx, "", "", maxLimit); err == nil {
			for _, bp := range extra {
				addBP(bp)
			}
		}
	}

	return selected, nil
}

// ScanManufacturingOpportunitiesV2 evaluates SDE blueprints at any of the 5 major trade hubs
// with Tech Level, Search, Daily Volume (7d), Fits-Capital, and Profitable-Only filtering.
func (s *CopilotService) ScanManufacturingOpportunitiesV2(ctx context.Context, req ManufacturingScanRequest) ([]domain.ManufacturingOpportunity, error) {
	if s.sde == nil {
		return nil, fmt.Errorf("EVE SDE blueprint database is not available")
	}
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market provider is not available")
	}

	settings, _ := s.GetSettings(ctx)
	_, availCap, _ := s.resolveProfileAndCapital(ctx, *settings)
	if availCap <= 0 {
		availCap = settings.StartingCapital
	}

	hub := settings.ActiveTradeHub()
	if strings.TrimSpace(req.HubKey) != "" {
		hub = domain.LookupTradeHub(req.HubKey)
	}
	matHub := hub
	if strings.TrimSpace(req.MaterialHubKey) != "" {
		matHub = domain.LookupTradeHub(req.MaterialHubKey)
	}
	crossHub := matHub.Key != hub.Key

	// Optional instant local PostgreSQL lookup of connected pilot's owned blueprints (0ms, zero ESI call)
	ownedByTypeID := make(map[int32]domain.CharacterBlueprint)
	if s.chars != nil {
		mode := s.GetModeInfo(ctx)
		if mode.CharacterID > 0 {
			if pilotBPs, err := s.chars.ListBlueprints(ctx, mode.CharacterID); err == nil {
				for _, pbp := range pilotBPs {
					prev, exists := ownedByTypeID[pbp.TypeID]
					if !exists || pbp.MaterialEfficiency > prev.MaterialEfficiency ||
						(pbp.MaterialEfficiency == prev.MaterialEfficiency && pbp.TimeEfficiency > prev.TimeEfficiency) {
						ownedByTypeID[pbp.TypeID] = pbp
					}
				}
			}
		}
	}

	const maxScanBlueprints = 40
	bps, err := s.selectBlueprintsForScan(ctx, req, maxScanBlueprints)
	if err != nil {
		return nil, fmt.Errorf("loading blueprints: %w", err)
	}
	if len(bps) == 0 {
		return []domain.ManufacturingOpportunity{}, nil
	}

	matsByBP := make(map[int32][]domain.SDEMaterialRequirement, len(bps))
	seenSell := make(map[int32]struct{})
	seenMat := make(map[int32]struct{})
	sellTypeIDs := make([]int32, 0, len(bps)+24)
	matTypeIDs := make([]int32, 0, len(bps)+24)
	productIDs := make([]int32, 0, len(bps))
	addSellID := func(id int32) {
		if id <= 0 {
			return
		}
		if _, ok := seenSell[id]; !ok {
			seenSell[id] = struct{}{}
			sellTypeIDs = append(sellTypeIDs, id)
		}
	}
	addMatID := func(id int32) {
		if id <= 0 {
			return
		}
		if _, ok := seenMat[id]; !ok {
			seenMat[id] = struct{}{}
			matTypeIDs = append(matTypeIDs, id)
		}
	}

	for _, bp := range bps {
		addSellID(bp.ProductTypeID)
		productIDs = append(productIDs, bp.ProductTypeID)
		reqs, mErr := s.sde.GetBlueprintMaterials(ctx, bp.BlueprintTypeID)
		if mErr != nil || len(reqs) == 0 {
			continue
		}
		matsByBP[bp.BlueprintTypeID] = reqs
		for _, r := range reqs {
			if crossHub {
				addMatID(r.MaterialTypeID)
			} else {
				addSellID(r.MaterialTypeID)
			}
		}
	}

	var (
		sellPrices map[int32]domain.JitaPrice
		matPrices  map[int32]domain.JitaPrice
		vols       map[int32]float64
		sellErr    error
		matErr     error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		sellPrices, sellErr = s.fetchHubPrices(ctx, hub, sellTypeIDs)
	}()
	go func() {
		defer wg.Done()
		vols = s.fetchHubDailyVolumesBatch(ctx, hub, productIDs)
	}()
	if crossHub {
		wg.Add(1)
		go func() {
			defer wg.Done()
			matPrices, matErr = s.fetchHubPrices(ctx, matHub, matTypeIDs)
		}()
	}
	wg.Wait()

	if sellErr != nil {
		return nil, fmt.Errorf("fetching live %s prices from Public ESI: %w", hub.Name, sellErr)
	}
	if matErr != nil {
		return nil, fmt.Errorf("fetching live %s material prices from Public ESI: %w", matHub.Name, matErr)
	}
	if !crossHub {
		matPrices = sellPrices
	}

	results := make([]domain.ManufacturingOpportunity, 0, len(bps))

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
			ownedCopy := owned
			ownedPtr = &ownedCopy
		}

		me, te, ownedMatched, _ := domain.RecommendManufacturingMETE(tech, ownedPtr, req.ME, req.TE)

		baseInput := domain.BlueprintCalcInput{
			BlueprintTypeID:     bp.BlueprintTypeID,
			Runs:                1,
			ME:                  me,
			TE:                  te,
			StructureKey:        req.StructureKey,
			HubKey:              hub.Key,
			MaterialPriceSource: req.MaterialPriceSource,
			SystemCostIndex:     settings.SystemCostIndex,
			FacilityTaxPercent:  settings.FacilityTaxPercent,
			FreightCostPerM3:    settings.FreightCostPerM3,
			BrokerFeePercent:    settings.BrokerFeePercent,
			SalesTaxPercent:     settings.SalesTaxPercent,
			RelistFeePercent:    settings.RelistFeePercent,
			EstimatedRelists:    1,
			BatchesPerWeek:      4,
		}

		var (
			calcRes   domain.BlueprintCalcResult
			cErr      error
			runs      int
			runReason string
			autoSized bool
		)

		dailyVol := vols[bp.ProductTypeID]

		// Build per-blueprint price map so Cross-Hub Manufacturing buys materials at matHub and sells product at hub
		bpPrices := sellPrices
		if crossHub {
			bpPrices = make(map[int32]domain.JitaPrice, len(reqs)+1)
			for _, r := range reqs {
				if mp, ok := matPrices[r.MaterialTypeID]; ok {
					bpPrices[r.MaterialTypeID] = mp
				}
			}
			if sp, ok := sellPrices[bp.ProductTypeID]; ok {
				bpPrices[bp.ProductTypeID] = sp
			}
		}

		if req.Runs > 0 {
			runs, runReason, autoSized = domain.RecommendManufacturingRuns(0, 0, bp.ProductQtyPerRun, dailyVol, availCap, req.Runs)
			baseInput.Runs = runs
			calcRes, cErr = domain.CalculateBlueprintManufacturing(bp, reqs, baseInput, bpPrices)
		} else {
			// Evaluate 1-run baseline to measure exact duration & direct cost, then auto-size optimal runs
			singleRes, sErr := domain.CalculateBlueprintManufacturing(bp, reqs, baseInput, bpPrices)
			if sErr != nil || singleRes.ProductUnitSellPrice <= 0 {
				continue
			}
			outPerRun := bp.ProductQtyPerRun
			if outPerRun <= 0 {
				outPerRun = 1
			}
			runs, runReason, autoSized = domain.RecommendManufacturingRuns(
				float64(singleRes.EstimatedDurationSeconds),
				singleRes.Profit.TotalDirectCost,
				outPerRun,
				dailyVol,
				availCap,
				0,
			)
			if runs == 1 {
				calcRes, cErr = singleRes, nil
			} else {
				baseInput.Runs = runs
				calcRes, cErr = domain.CalculateBlueprintManufacturing(bp, reqs, baseInput, bpPrices)
			}
		}

		if cErr != nil || calcRes.ProductUnitSellPrice <= 0 {
			continue
		}

		verdict := domain.ClassifyManufacturingVerdict(calcRes.Profit.NetProfitPerBatch, calcRes.Profit.NetMarginPercent, calcRes.ISKPerHour)
		fitsCap := availCap <= 0 || calcRes.Profit.TotalDirectCost <= availCap
		if req.ProfitableOnly && verdict == domain.VerdictAvoid {
			continue
		}
		if req.FitsCapitalOnly && !fitsCap {
			continue
		}

		recSell := domain.RoundEVETick(calcRes.ProductUnitSellPrice * 0.999)
		if recSell <= 0 {
			recSell = calcRes.ProductUnitSellPrice
		}

		results = append(results, domain.ManufacturingOpportunity{
			BlueprintTypeID:   bp.BlueprintTypeID,
			ProductTypeID:     bp.ProductTypeID,
			ProductName:       bp.ProductName,
			Category:          bp.Category,
			TechLevel:         tech,
			HubKey:            hub.Key,
			HubName:           hub.Name,
			MaterialHubKey:    matHub.Key,
			MaterialHubName:   matHub.Name,
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
			Verdict:           verdict,
		})
	}

	domain.SortManufacturingOpportunitiesBy(results, req.SortBy)
	return results, nil
}

// ScanStationTradingOpportunities is a backward-compatible wrapper delegating to ScanStationTradingOpportunitiesV2.
func (s *CopilotService) ScanStationTradingOpportunities(ctx context.Context, category string) ([]domain.StationTradeOpportunity, error) {
	return s.ScanStationTradingOpportunitiesV2(ctx, "", category, false)
}

// ScanStationTradingOpportunitiesV2 scans high-liquidity items + user Catalog items at any of the 5 trade hubs
// for profitable Buy->Sell flips, enriched with 7d daily volume and Recommended Buy/Sell Order prices.
func (s *CopilotService) ScanStationTradingOpportunitiesV2(ctx context.Context, hubKey, category string, profitableOnly bool) ([]domain.StationTradeOpportunity, error) {
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market provider is not available")
	}

	settings, _ := s.GetSettings(ctx)
	hub := settings.ActiveTradeHub()
	if strings.TrimSpace(hubKey) != "" {
		hub = domain.LookupTradeHub(hubKey)
	}
	catFilter := strings.TrimSpace(category)

	basket := make([]scannerBasketItem, 0, len(defaultStationTradeBasket)+12)
	seen := make(map[int32]struct{})

	for _, b := range defaultStationTradeBasket {
		if catFilter != "" && !strings.EqualFold(b.Category, catFilter) {
			continue
		}
		basket = append(basket, b)
		seen[b.TypeID] = struct{}{}
	}

	// Merge user's own catalog items that have resolved TypeIDs
	if s.items != nil {
		if catalogItems, err := s.items.ListItems(ctx); err == nil {
			for _, it := range catalogItems {
				if it.TypeID <= 0 {
					continue
				}
				if _, ok := seen[it.TypeID]; ok {
					continue
				}
				if catFilter != "" && !strings.EqualFold(string(it.Category), catFilter) {
					continue
				}
				basket = append(basket, scannerBasketItem{
					TypeID:   it.TypeID,
					Name:     it.Name,
					Category: string(it.Category),
				})
				seen[it.TypeID] = struct{}{}
			}
		}
	}

	typeIDs := make([]int32, 0, len(basket))
	for _, b := range basket {
		typeIDs = append(typeIDs, b.TypeID)
	}

	prices, err := s.fetchHubPrices(ctx, hub, typeIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching %s station trading prices: %w", hub.Name, err)
	}
	vols := s.fetchHubDailyVolumesBatch(ctx, hub, typeIDs)

	results := make([]domain.StationTradeOpportunity, 0, len(basket))
	for _, b := range basket {
		jp, ok := prices[b.TypeID]
		if !ok || jp.LowestSell <= 0 || jp.HighestBuy <= 0 {
			continue
		}
		opp := domain.EvaluateStationTradeForHub(
			b.TypeID, b.Name, b.Category, hub, jp, vols[b.TypeID],
			settings.BrokerFeePercent, settings.SalesTaxPercent,
		)
		if profitableOnly && opp.Verdict == domain.VerdictAvoid {
			continue
		}
		results = append(results, opp)
	}

	domain.SortStationTradeOpportunities(results)
	return results, nil
}

// ScanInterHubArbitrageOpportunities is a backward-compatible wrapper delegating to ScanInterHubArbitrageOpportunitiesV2.
func (s *CopilotService) ScanInterHubArbitrageOpportunities(ctx context.Context, originKey, destKey string) ([]domain.ArbitrageOpportunity, error) {
	return s.ScanInterHubArbitrageOpportunitiesV2(ctx, originKey, destKey, "", 5000, 150_000_000, false)
}

// ScanInterHubArbitrageOpportunitiesV2 compares live prices between two major Empire trade hubs in parallel
// after deducting hauling cost, destination taxes, cargo cap (m3), budget, and destination daily liquidity.
func (s *CopilotService) ScanInterHubArbitrageOpportunitiesV2(
	ctx context.Context,
	originKey, destKey, category string,
	cargoCapM3, tripBudgetISK float64,
	profitableOnly bool,
) ([]domain.ArbitrageOpportunity, error) {
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market provider is not available")
	}

	originHub := domain.LookupTradeHub(originKey)
	destHub := domain.LookupTradeHub(destKey)
	if originHub.Key == destHub.Key {
		return nil, fmt.Errorf("origin hub and destination hub must be different")
	}

	settings, _ := s.GetSettings(ctx)
	catFilter := strings.TrimSpace(category)

	basket := make([]scannerBasketItem, 0, len(defaultArbitrageBasket)+10)
	seen := make(map[int32]struct{})
	for _, b := range defaultArbitrageBasket {
		if catFilter != "" && !strings.EqualFold(b.Category, catFilter) {
			continue
		}
		basket = append(basket, b)
		seen[b.TypeID] = struct{}{}
	}

	// Merge user's catalog items with valid TypeIDs
	if s.items != nil {
		if catalogItems, err := s.items.ListItems(ctx); err == nil {
			for _, it := range catalogItems {
				if it.TypeID <= 0 {
					continue
				}
				if _, ok := seen[it.TypeID]; ok {
					continue
				}
				if catFilter != "" && !strings.EqualFold(string(it.Category), catFilter) {
					continue
				}
				basket = append(basket, scannerBasketItem{
					TypeID:   it.TypeID,
					Name:     it.Name,
					Category: string(it.Category),
					VolumeM3: 10.0,
				})
				seen[it.TypeID] = struct{}{}
			}
		}
	}

	typeIDs := make([]int32, 0, len(basket))
	for _, b := range basket {
		typeIDs = append(typeIDs, b.TypeID)
	}

	// Fetch Origin prices, Destination prices, and Destination 7d daily volumes concurrently (cache-first, bounded workers)
	var (
		originPrices map[int32]domain.JitaPrice
		destPrices   map[int32]domain.JitaPrice
		destVols     map[int32]float64
		originErr    error
		destErr      error
		wg           sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		originPrices, originErr = s.fetchHubPrices(ctx, originHub, typeIDs)
	}()
	go func() {
		defer wg.Done()
		destPrices, destErr = s.fetchHubPrices(ctx, destHub, typeIDs)
	}()
	go func() {
		defer wg.Done()
		destVols = s.fetchHubDailyVolumesBatch(ctx, destHub, typeIDs)
	}()
	wg.Wait()

	if originErr != nil {
		return nil, fmt.Errorf("fetching %s prices: %w", originHub.Name, originErr)
	}
	if destErr != nil {
		return nil, fmt.Errorf("fetching %s prices: %w", destHub.Name, destErr)
	}

	results := make([]domain.ArbitrageOpportunity, 0, len(basket))
	for _, item := range basket {
		op, okO := originPrices[item.TypeID]
		dp, okD := destPrices[item.TypeID]
		if !okO || !okD || op.LowestSell <= 0 || dp.LowestSell <= 0 {
			continue
		}

		opp := domain.EvaluateArbitrageWithTrip(
			item.TypeID, item.Name, item.Category,
			originHub, destHub,
			op.LowestSell, dp.LowestSell, dp.HighestBuy,
			item.VolumeM3, destVols[item.TypeID], cargoCapM3, tripBudgetISK,
			settings.FreightCostPerM3,
			settings.BrokerFeePercent, settings.SalesTaxPercent,
		)
		if profitableOnly && opp.Verdict == domain.VerdictAvoid {
			continue
		}
		results = append(results, opp)
	}

	domain.SortArbitrageOpportunities(results)
	return results, nil
}

// ScanLocalStockoutOpportunities detects high-demand items that are stocked out (0 sell orders),
// have thin seller competition (<=3 sellers), or trade at a >=15% local premium over Jita at the chosen Trade Hub.
func (s *CopilotService) ScanLocalStockoutOpportunities(ctx context.Context, hubKey, category string) ([]domain.StockoutOpportunity, error) {
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market provider is not available")
	}

	settings, _ := s.GetSettings(ctx)
	hub := settings.ActiveTradeHub()
	if strings.TrimSpace(hubKey) != "" {
		hub = domain.LookupTradeHub(hubKey)
	}
	jitaHub := domain.LookupTradeHub("jita")
	catFilter := strings.TrimSpace(category)

	basket := make([]scannerBasketItem, 0, len(defaultStationTradeBasket)+16)
	seen := make(map[int32]struct{})
	for _, b := range defaultStationTradeBasket {
		if catFilter != "" && !strings.EqualFold(b.Category, catFilter) {
			continue
		}
		basket = append(basket, b)
		seen[b.TypeID] = struct{}{}
	}
	if s.items != nil {
		if catalogItems, err := s.items.ListItems(ctx); err == nil {
			for _, it := range catalogItems {
				if it.TypeID <= 0 {
					continue
				}
				if _, ok := seen[it.TypeID]; ok {
					continue
				}
				if catFilter != "" && !strings.EqualFold(string(it.Category), catFilter) {
					continue
				}
				basket = append(basket, scannerBasketItem{
					TypeID:   it.TypeID,
					Name:     it.Name,
					Category: string(it.Category),
				})
				seen[it.TypeID] = struct{}{}
			}
		}
	}

	typeIDs := make([]int32, 0, len(basket))
	for _, b := range basket {
		typeIDs = append(typeIDs, b.TypeID)
	}

	var (
		hubPrices  map[int32]domain.JitaPrice
		jitaPrices map[int32]domain.JitaPrice
		hubVols    map[int32]float64
		hubErr     error
		wg         sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		hubPrices, hubErr = s.fetchHubPrices(ctx, hub, typeIDs)
	}()
	go func() {
		defer wg.Done()
		if hub.Key == "jita" {
			return
		}
		jitaPrices, _ = s.fetchHubPrices(ctx, jitaHub, typeIDs)
	}()
	go func() {
		defer wg.Done()
		hubVols = s.fetchHubDailyVolumesBatch(ctx, hub, typeIDs)
	}()
	wg.Wait()

	if hubErr != nil {
		return nil, fmt.Errorf("fetching %s stockout prices: %w", hub.Name, hubErr)
	}
	if hub.Key == "jita" {
		jitaPrices = hubPrices
	}

	results := make([]domain.StockoutOpportunity, 0, len(basket)/2+1)
	for _, item := range basket {
		hp := hubPrices[item.TypeID]
		jp := jitaPrices[item.TypeID]
		vol := hubVols[item.TypeID]
		if opp, ok := domain.EvaluateStockoutOpportunity(item.TypeID, item.Name, item.Category, hub, hp, jp, vol); ok {
			results = append(results, opp)
		}
	}

	domain.SortStockoutOpportunities(results)
	return results, nil
}
