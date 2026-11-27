package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"aura-forge/internal/domain"
)

type majorLPCorp struct {
	ID      int32
	Name    string
	HubHint string
}

// majorEmpireLPCorps covers the primary High-Sec & Faction LP Stores in EVE Online.
var majorEmpireLPCorps = []majorLPCorp{
	{ID: 1000035, Name: "Caldari Navy", HubHint: "Jita IV - Moon 4 - Caldari Navy Assembly Plant (The Forge)"},
	{ID: 1000084, Name: "Amarr Navy", HubHint: "Amarr VIII (Oris) / Emperor Family & Amarr Navy stations (Domain)"},
	{ID: 1000120, Name: "Federation Navy", HubHint: "Dodixie IX - Moon 20 - Federation Navy Assembly Plant (Sinq Laison)"},
	{ID: 1000051, Name: "Republic Fleet", HubHint: "Rens VI - Moon 8 - Brutor Tribe / Republic Fleet stations (Heimatar)"},
	{ID: 1000130, Name: "Sisters of EVE", HubHint: "Apanake / Osmon / Lanngisi - Sisters of EVE Bureau"},
	{ID: 1000033, Name: "Caldari Business Tribunal", HubHint: "Caldari State CBT stations (The Forge / Lonetrek)"},
	{ID: 1000066, Name: "Imperial Shipment", HubHint: "Amarr Empire Imperial Shipment stations (Domain)"},
	{ID: 1000116, Name: "Federal Intelligence Office", HubHint: "Gallente FIO stations (Sinq Laison / Essence)"},
	{ID: 1000160, Name: "Thukker Mix", HubHint: "Great Wildlands / Metropolis Thukker stations"},
	{ID: 1000127, Name: "Mordu's Legion", HubHint: "Pure Blind (5ZXX-K) Mordu's Legion Testing Facilities"},
}

var knownGankChokepoints = map[string]string{
	"Uedama":  "Notorious High-Sec gank chokepoint (0.5) on Jita–Amarr route",
	"Sivala":  "Frequent High-Sec freighter/hauler gank system (0.6) near Jita",
	"Ahbazon": "Deadly Low-Sec shortcut (0.4) on Jita–Amarr — NEVER take solo with cargo",
	"Rancer":  "Notorious Low-Sec gatecamp chokepoint (0.4)",
	"Tama":    "High-activity Low-Sec PvP/gatecamp entry (0.3)",
	"Vecamia": "Low-Sec shortcut system (0.4) on Amarr–Dodixie route",
	"Aufay":   "High-Sec gank chokepoint (0.5) near Dodixie",
}

func (s *CopilotService) runFindBlueprintSources(
	ctx context.Context,
	defaultHub domain.TradeHub,
	argsRaw string,
) (any, string, error) {
	var args struct {
		Item      string `json:"item"`
		RegionHub string `json:"region_hub"`
	}
	_ = json.Unmarshal([]byte(argsRaw), &args)
	itemQuery := strings.TrimSpace(args.Item)
	if itemQuery == "" {
		return nil, "", fmt.Errorf("item name is required")
	}

	hub := defaultHub
	if strings.TrimSpace(args.RegionHub) != "" {
		hub = domain.LookupTradeHub(args.RegionHub)
	}

	intel, ok := s.market.(domain.UniverseIntelProvider)
	if !ok || s.market == nil {
		return nil, "", fmt.Errorf("Public ESI universe intelligence provider is not available")
	}

	// Build candidate names for both the product and its Blueprint.
	baseName := strings.TrimSpace(strings.TrimSuffix(itemQuery, " Blueprint"))
	bpName := baseName + " Blueprint"

	var (
		productTypeID   int32
		blueprintTypeID int32
	)

	// 1. Check local SDE first for exact/fuzzy blueprint match (0 ESI calls).
	if s.sde != nil {
		if bps, err := s.sde.SearchBlueprints(ctx, baseName, "", 5); err == nil {
			for _, bp := range bps {
				if strings.EqualFold(bp.ProductName, baseName) || strings.EqualFold(bp.Name, bpName) {
					blueprintTypeID = bp.BlueprintTypeID
					productTypeID = bp.ProductTypeID
					baseName = bp.ProductName
					bpName = bp.Name
					break
				}
			}
			if blueprintTypeID == 0 && len(bps) > 0 {
				blueprintTypeID = bps[0].BlueprintTypeID
				productTypeID = bps[0].ProductTypeID
			}
		}
	}

	// 2. Resolve via Public ESI POST /universe/ids/ for items not in starter SDE (e.g. Faction BPCs).
	if resolved, err := s.market.ResolveTypeIDs(ctx, []string{baseName, bpName, itemQuery}); err == nil {
		if productTypeID == 0 {
			if id, exists := resolved[baseName]; exists {
				productTypeID = id
			} else if id, exists := resolved[itemQuery]; exists {
				productTypeID = id
			}
		}
		if blueprintTypeID == 0 {
			if id, exists := resolved[bpName]; exists {
				blueprintTypeID = id
			}
		}
	}

	if productTypeID == 0 && blueprintTypeID == 0 {
		return map[string]any{
			"query":   itemQuery,
			"found":   false,
			"message": "Could not resolve an exact EVE Online item or blueprint name. Check exact spelling (e.g. 'Caldari Navy Ballistic Control System' or 'Caracal').",
		}, "Item not found in SDE/ESI", nil
	}

	// 3. Parallel fan-out:
	//    (a) NPC BPO Seed Orders (duration=365) in active hub region + The Forge (Jita)
	//    (b) Major Empire/Faction LP Store offers matching blueprintTypeID or productTypeID
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		npcSeeds  []domain.NPCBPOSeedOrder
		lpMatches []domain.LoyaltyStoreOffer
	)

	// Regions to scan for NPC BPO seed orders
	regionsToScan := []domain.TradeHub{hub}
	if hub.Key != "jita" {
		regionsToScan = append(regionsToScan, domain.LookupTradeHub("jita"))
	}

	if blueprintTypeID > 0 {
		for _, regHub := range regionsToScan {
			wg.Add(1)
			go func(h domain.TradeHub) {
				defer wg.Done()
				seeds, err := intel.FindNPCBPOSeedOrders(ctx, h.RegionID, h.RegionName, blueprintTypeID)
				if err != nil || len(seeds) == 0 {
					return
				}
				mu.Lock()
				npcSeeds = append(npcSeeds, seeds...)
				mu.Unlock()
			}(regHub)
		}
	}

	// Prioritize corp matching the item name prefix first, then scan major LP corps with bounded semaphore (4 workers).
	corps := make([]majorLPCorp, len(majorEmpireLPCorps))
	copy(corps, majorEmpireLPCorps)
	lowerItem := strings.ToLower(baseName)
	sort.SliceStable(corps, func(i, j int) bool {
		mi := strings.Contains(lowerItem, strings.ToLower(corps[i].Name))
		mj := strings.Contains(lowerItem, strings.ToLower(corps[j].Name))
		return mi && !mj
	})

	lpSem := make(chan struct{}, 4)
	for _, corp := range corps {
		wg.Add(1)
		go func(c majorLPCorp) {
			defer wg.Done()
			select {
			case lpSem <- struct{}{}:
				defer func() { <-lpSem }()
			case <-ctx.Done():
				return
			}
			offers, err := intel.GetLoyaltyStoreOffers(ctx, c.ID, c.Name)
			if err != nil {
				return
			}
			var matched []domain.LoyaltyStoreOffer
			for _, off := range offers {
				if (blueprintTypeID > 0 && off.TypeID == blueprintTypeID) || (productTypeID > 0 && off.TypeID == productTypeID) {
					if off.TypeID == blueprintTypeID {
						off.TypeName = bpName + " (BPC)"
					} else {
						off.TypeName = baseName + " (Direct Item)"
					}
					matched = append(matched, off)
				}
			}
			if len(matched) > 0 {
				mu.Lock()
				lpMatches = append(lpMatches, matched...)
				mu.Unlock()
			}
		}(corp)
	}
	wg.Wait()

	// Resolve required item names for LP Store offers in a single batch call.
	if len(lpMatches) > 0 {
		if globalProv, ok := s.market.(domain.GlobalMarketPriceProvider); ok {
			reqIDs := make([]int32, 0, 16)
			for _, m := range lpMatches {
				for _, ri := range m.RequiredItems {
					reqIDs = append(reqIDs, ri.TypeID)
				}
			}
			if len(reqIDs) > 0 {
				if names, err := globalProv.ResolveTypeNames(ctx, reqIDs); err == nil {
					for i := range lpMatches {
						for j := range lpMatches[i].RequiredItems {
							if nm, exists := names[lpMatches[i].RequiredItems[j].TypeID]; exists {
								lpMatches[i].RequiredItems[j].Name = nm
							}
						}
					}
				}
			}
		}
	}

	// Build LP store hub hints map
	corpHints := make(map[int32]string, len(majorEmpireLPCorps))
	for _, c := range majorEmpireLPCorps {
		corpHints[c.ID] = c.HubHint
	}

	type lpOfferSummary struct {
		Corporation     string                            `json:"corporation"`
		LPStoreLocation string                            `json:"lp_store_location_hint"`
		Reward          string                            `json:"reward"`
		Quantity        int                               `json:"quantity"`
		LPCost          int                               `json:"lp_cost"`
		ISKCost         int64                             `json:"isk_cost"`
		RequiredItems   []domain.LoyaltyStoreRequiredItem `json:"required_items,omitempty"`
	}
	lpOut := make([]lpOfferSummary, 0, len(lpMatches))
	for _, m := range lpMatches {
		lpOut = append(lpOut, lpOfferSummary{
			Corporation:     m.CorporationName,
			LPStoreLocation: corpHints[m.CorporationID],
			Reward:          m.TypeName,
			Quantity:        m.Quantity,
			LPCost:          m.LPCost,
			ISKCost:         m.ISKCost,
			RequiredItems:   m.RequiredItems,
		})
	}

	guidance := ""
	switch {
	case len(npcSeeds) > 0:
		guidance = fmt.Sprintf("Official NPC BPO seed orders found in %d station(s). Buy directly from NPC sell orders (365d duration) to avoid player markup.", len(npcSeeds))
	case len(lpOut) > 0:
		guidance = "No NPC market BPO exists — this is a Faction/LP Store item. Obtain its BPC (or direct item) from the listed NPC Corporation LP Store(s), or buy BPCs via in-game Player Contracts."
	default:
		guidance = "No NPC BPO seed order or major Empire LP Store offer found. This blueprint is likely obtained via T2 Invention (from T1 BPC + Datacores), Exploration/Combat DED loot drops, Pirate LP Stores, or Player Contracts."
	}

	res := map[string]any{
		"item":              baseName,
		"blueprint_name":    bpName,
		"product_type_id":   productTypeID,
		"blueprint_type_id": blueprintTypeID,
		"scanned_hub":       hub.Name,
		"npc_bpo_stations":  npcSeeds,
		"lp_store_offers":   lpOut,
		"guidance":          guidance,
	}
	summary := fmt.Sprintf("Blueprint sources for %s: %d NPC BPO station(s), %d LP Store offer(s)", baseName, len(npcSeeds), len(lpOut))
	return res, summary, nil
}

func (s *CopilotService) runCheckRouteAndHaulingRisk(
	ctx context.Context,
	defaultHub domain.TradeHub,
	argsRaw string,
) (any, string, error) {
	var args struct {
		Origin        string  `json:"origin"`
		Destination   string  `json:"destination"`
		CargoValueISK float64 `json:"cargo_value_isk"`
		ShipClass     string  `json:"ship_class"`
	}
	_ = json.Unmarshal([]byte(argsRaw), &args)

	intel, ok := s.market.(domain.UniverseIntelProvider)
	if !ok || s.market == nil {
		return nil, "", fmt.Errorf("Public ESI universe intelligence provider is not available")
	}

	originName := strings.TrimSpace(args.Origin)
	if originName == "" {
		originName = defaultHub.Key
	}
	destName := strings.TrimSpace(args.Destination)
	if destName == "" {
		if strings.EqualFold(originName, "jita") {
			destName = "amarr"
		} else {
			destName = "jita"
		}
	}

	originID, resolvedOrigin, err := intel.ResolveSolarSystemID(ctx, originName)
	if err != nil {
		return nil, "", err
	}
	destID, resolvedDest, err := intel.ResolveSolarSystemID(ctx, destName)
	if err != nil {
		return nil, "", err
	}

	// Fetch secure route, shortest route, and New Eden 1h ship kills in parallel.
	var (
		wg            sync.WaitGroup
		secureRoute   []int32
		shortestRoute []int32
		killsMap      map[int32]domain.SystemKillStat
		routeErr      error
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		secureRoute, routeErr = intel.GetRoute(ctx, originID, destID, "secure")
	}()
	go func() {
		defer wg.Done()
		shortestRoute, _ = intel.GetRoute(ctx, originID, destID, "shortest")
	}()
	go func() {
		defer wg.Done()
		killsMap, _ = intel.GetSystemKills(ctx)
	}()
	wg.Wait()

	if routeErr != nil {
		return nil, "", routeErr
	}

	// Resolve solar system names & security status along the secure route in parallel (max 6 workers).
	sysInfos := make([]*domain.SolarSystemInfo, len(secureRoute))
	sem := make(chan struct{}, 6)
	var sysWG sync.WaitGroup
	for i, sysID := range secureRoute {
		sysWG.Add(1)
		go func(idx int, sid int32) {
			defer sysWG.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			info, err := intel.GetSystemInfo(ctx, sid)
			if err == nil {
				sysInfos[idx] = info
			}
		}(i, sysID)
	}
	sysWG.Wait()

	type hotspot struct {
		System         string  `json:"system"`
		SecurityStatus float64 `json:"security_status"`
		SpaceType      string  `json:"space_type"`
		ShipKills1h    int     `json:"ship_kills_1h"`
		PodKills1h     int     `json:"pod_kills_1h"`
		Warning        string  `json:"warning"`
	}

	var (
		lowSecCount  int
		nullSecCount int
		minSec       = 1.0
		totalKills1h int
		hotspots     []hotspot
	)
	for _, info := range sysInfos {
		if info == nil {
			continue
		}
		sec := math.Round(info.SecurityStatus*100) / 100
		if sec < minSec {
			minSec = sec
		}
		spaceType := "HIGH-SEC"
		if sec <= 0.0 {
			spaceType = "NULL-SEC"
			nullSecCount++
		} else if sec < 0.45 {
			spaceType = "LOW-SEC"
			lowSecCount++
		}

		ks := killsMap[info.SystemID]
		totalKills1h += ks.ShipKills

		note, isKnownChoke := knownGankChokepoints[info.Name]
		if spaceType != "HIGH-SEC" || isKnownChoke || ks.ShipKills >= 5 {
			warn := note
			if warn == "" && spaceType != "HIGH-SEC" {
				warn = fmt.Sprintf("Non-CONCORD %s system — high gatecamp/gank danger", spaceType)
			} else if warn == "" {
				warn = fmt.Sprintf("Elevated PvP/gank activity (%d ship kills in last 1h)", ks.ShipKills)
			}
			hotspots = append(hotspots, hotspot{
				System:         info.Name,
				SecurityStatus: sec,
				SpaceType:      spaceType,
				ShipKills1h:    ks.ShipKills,
				PodKills1h:     ks.PodKills,
				Warning:        warn,
			})
		}
	}

	// Ship class safe cargo threshold (beginner-first anti-gank protection)
	shipClass := strings.ToLower(strings.TrimSpace(args.ShipClass))
	safeLimitISK := 150_000_000.0
	shipLabel := "T1 Industrial Hauler (e.g. Bestower / Badger / Nereus / Epithal)"
	switch shipClass {
	case "dst":
		safeLimitISK = 1_500_000_000.0
		shipLabel = "Deep Space Transport (DST — Impel / Bustard / Occator / Mastodon)"
	case "blockade_runner", "br":
		safeLimitISK = 3_000_000_000.0
		shipLabel = "Blockade Runner (Covert Ops Cloak)"
	case "freighter":
		safeLimitISK = 1_000_000_000.0
		shipLabel = "T1 Freighter (High-Sec slow hull — never exceed 1B without webber/scout)"
	default:
		shipClass = "t1_hauler"
	}

	verdict := "SAFE"
	advice := "High-Sec secure route with low recent PvP activity."
	switch {
	case nullSecCount > 0 || lowSecCount > 0:
		verdict = "AVOID_SOLO"
		advice = fmt.Sprintf("Secure route still crosses %d Low-Sec and %d Null-Sec system(s). Do NOT haul solo in a T1 hauler — use a public Courier Contract (Red Frog / PushX) or scouted Blockade Runner.", lowSecCount, nullSecCount)
	case args.CargoValueISK > safeLimitISK*1.5:
		verdict = "HIGH_RISK"
		advice = fmt.Sprintf("Cargo value (%s) far exceeds safe threshold (%s) for %s. Split into smaller trips or use a Courier Contract.", fmtCompactISK(args.CargoValueISK), fmtCompactISK(safeLimitISK), shipLabel)
	case args.CargoValueISK > safeLimitISK || len(hotspots) > 0:
		verdict = "CAUTION"
		advice = fmt.Sprintf("Route passes %d watch system(s) (%d total ship kills in 1h). Keep cargo under %s for %s, never autopilot, and use MWD+Cloak or instant-undock bookmarks.", len(hotspots), totalKills1h, fmtCompactISK(safeLimitISK), shipLabel)
	}

	secureJumps := len(secureRoute) - 1
	if secureJumps < 0 {
		secureJumps = 0
	}
	shortestJumps := len(shortestRoute) - 1
	if shortestJumps < 0 {
		shortestJumps = 0
	}

	res := map[string]any{
		"origin":              resolvedOrigin,
		"destination":         resolvedDest,
		"secure_jumps":        secureJumps,
		"shortest_jumps":      shortestJumps,
		"lowest_security":     minSec,
		"low_sec_systems":     lowSecCount,
		"null_sec_systems":    nullSecCount,
		"route_ship_kills_1h": totalKills1h,
		"hotspots":            hotspots,
		"ship_class":          shipLabel,
		"cargo_value_isk":     args.CargoValueISK,
		"safe_cargo_limit":    safeLimitISK,
		"verdict":             verdict,
		"advice":              advice,
	}
	summary := fmt.Sprintf("Route %s → %s: %d secure jumps (min sec %.2f, %d hotspots, verdict %s)", resolvedOrigin, resolvedDest, secureJumps, minSec, len(hotspots), verdict)
	return res, summary, nil
}

func (s *CopilotService) runCheckMarketHistoryAndVolatility(
	ctx context.Context,
	defaultHub domain.TradeHub,
	argsRaw string,
) (any, string, error) {
	var args struct {
		Item string `json:"item"`
		Hub  string `json:"hub"`
		Days int    `json:"days"`
	}
	_ = json.Unmarshal([]byte(argsRaw), &args)
	itemName := strings.TrimSpace(args.Item)
	if itemName == "" {
		return nil, "", fmt.Errorf("item name is required")
	}

	hub := defaultHub
	if strings.TrimSpace(args.Hub) != "" {
		hub = domain.LookupTradeHub(args.Hub)
	}
	days := args.Days
	if days <= 0 {
		days = 30
	}
	if days > 60 {
		days = 60
	}

	browser, ok := s.market.(domain.MarketBrowserProvider)
	if !ok || s.market == nil {
		return nil, "", fmt.Errorf("Public ESI market history provider is not available")
	}

	// Resolve item type_id via SDE or ESI
	var typeID int32
	if s.sde != nil {
		if bps, err := s.sde.SearchBlueprints(ctx, itemName, "", 5); err == nil {
			for _, bp := range bps {
				if strings.EqualFold(bp.ProductName, itemName) {
					typeID = bp.ProductTypeID
					itemName = bp.ProductName
					break
				}
			}
		}
	}
	if typeID == 0 {
		if resolved, err := s.market.ResolveTypeIDs(ctx, []string{itemName}); err == nil {
			for k, id := range resolved {
				typeID = id
				itemName = k
				break
			}
		}
	}
	if typeID == 0 {
		return nil, "", fmt.Errorf("could not resolve EVE item %q", itemName)
	}

	// Fetch historical candles and live order book in parallel.
	var (
		wg         sync.WaitGroup
		hist       []domain.MarketHistoryPoint
		histErr    error
		liveSummary domain.HubPriceSummary
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		hist, histErr = browser.GetMarketHistory(ctx, hub.RegionID, typeID, days)
	}()
	go func() {
		defer wg.Done()
		_, _, liveSummary, _ = browser.GetRegionOrderBook(ctx, hub, typeID, 5)
	}()
	wg.Wait()

	if histErr != nil {
		return nil, "", histErr
	}
	if len(hist) == 0 {
		return map[string]any{
			"item":    itemName,
			"type_id": typeID,
			"hub":     hub.Name,
			"verdict": "NO_HISTORY",
			"advice":  "No recent trade history in this region. Avoid stocking — illiquid item.",
		}, fmt.Sprintf("No market history for %s in %s", itemName, hub.Name), nil
	}

	var sumPrice30, sumVol30 float64
	var sumPrice7, sumVol7 float64
	minPrice30 := math.MaxFloat64
	maxPrice30 := 0.0
	n := len(hist)
	start7 := n - 7
	if start7 < 0 {
		start7 = 0
	}
	count7 := 0

	for i, pt := range hist {
		sumPrice30 += pt.Average
		sumVol30 += float64(pt.Volume)
		if pt.Lowest > 0 && pt.Lowest < minPrice30 {
			minPrice30 = pt.Lowest
		}
		if pt.Highest > maxPrice30 {
			maxPrice30 = pt.Highest
		}
		if i >= start7 {
			sumPrice7 += pt.Average
			sumVol7 += float64(pt.Volume)
			count7++
		}
	}

	avgPrice30 := sumPrice30 / float64(n)
	avgVol30 := sumVol30 / float64(n)
	avgPrice7 := avgPrice30
	avgVol7 := avgVol30
	if count7 > 0 {
		avgPrice7 = sumPrice7 / float64(count7)
		avgVol7 = sumVol7 / float64(count7)
	}
	if minPrice30 == math.MaxFloat64 {
		minPrice30 = avgPrice30
	}

	// Price standard deviation / volatility %
	var varianceSum float64
	for _, pt := range hist {
		d := pt.Average - avgPrice30
		varianceSum += d * d
	}
	stdDev := math.Sqrt(varianceSum / float64(n))
	volatilityPct := 0.0
	if avgPrice30 > 0 {
		volatilityPct = (stdDev / avgPrice30) * 100
	}

	trend7vs30Pct := 0.0
	if avgPrice30 > 0 {
		trend7vs30Pct = ((avgPrice7 - avgPrice30) / avgPrice30) * 100
	}

	spikeVs30Pct := 0.0
	if avgPrice30 > 0 && liveSummary.LowestSell > 0 {
		spikeVs30Pct = ((liveSummary.LowestSell - avgPrice30) / avgPrice30) * 100
	}

	verdict := "STABLE"
	advice := "Healthy price stability and consistent daily volume."
	switch {
	case avgVol7 < 5 && avgVol30 < 5:
		verdict = "THIN_LIQUIDITY"
		advice = "Very low daily volume (<5 units/day). High risk of capital lock-up; avoid large batches."
	case spikeVs30Pct > 25:
		verdict = "SPIKE_WARNING"
		advice = fmt.Sprintf("Current lowest sell (%s) is +%.1f%% above 30-day average (%s). Possible temporary price spike or market manipulation — do NOT assume this high margin will last.", fmtCompactISK(liveSummary.LowestSell), spikeVs30Pct, fmtCompactISK(avgPrice30))
	case volatilityPct > 20:
		verdict = "HIGH_VOLATILITY"
		advice = fmt.Sprintf("High 30-day price swings (%.1f%% volatility). Use small batch sizes and check Safe Break-Even Floor before listing.", volatilityPct)
	case trend7vs30Pct >= 8:
		verdict = "TRENDING_UP"
		advice = fmt.Sprintf("7-day average is +%.1f%% above 30-day average with %.0f units/day — rising demand.", trend7vs30Pct, avgVol7)
	case trend7vs30Pct <= -8:
		verdict = "TRENDING_DOWN"
		advice = fmt.Sprintf("7-day average is %.1f%% below 30-day average — falling price trend; verify margin at lower sell prices.", trend7vs30Pct)
	}

	res := map[string]any{
		"item":                itemName,
		"type_id":             typeID,
		"hub":                 hub.Name,
		"days_analyzed":       n,
		"current_lowest_sell": liveSummary.LowestSell,
		"current_highest_buy": liveSummary.HighestBuy,
		"avg_price_7d":        math.Round(avgPrice7*100) / 100,
		"avg_price_30d":       math.Round(avgPrice30*100) / 100,
		"min_price_30d":       minPrice30,
		"max_price_30d":       maxPrice30,
		"trend_7d_vs_30d_pct": math.Round(trend7vs30Pct*10) / 10,
		"live_vs_30d_pct":     math.Round(spikeVs30Pct*10) / 10,
		"volatility_pct":      math.Round(volatilityPct*10) / 10,
		"avg_daily_volume_7d": math.Round(avgVol7*10) / 10,
		"avg_daily_volume_30d": math.Round(avgVol30*10) / 10,
		"verdict":             verdict,
		"advice":              advice,
	}
	summary := fmt.Sprintf("History %s (%s, %dd): 7d=%s vs 30d=%s (%.1f%%), vol=%.0f/d, verdict=%s",
		itemName, hub.Name, n, fmtCompactISK(avgPrice7), fmtCompactISK(avgPrice30), trend7vs30Pct, avgVol7, verdict)
	return res, summary, nil
}

func (s *CopilotService) runScanGasReactions(
	ctx context.Context,
	defaultHub domain.TradeHub,
	argsRaw string,
) (any, string, error) {
	var args struct {
		Hub            string `json:"hub"`
		MaterialHub    string `json:"material_hub"`
		Refinery       string `json:"refinery"`
		Scope          string `json:"scope"`
		ProfitableOnly *bool  `json:"profitable_only"`
	}
	_ = json.Unmarshal([]byte(argsRaw), &args)

	hubKey := strings.TrimSpace(args.Hub)
	if hubKey == "" {
		hubKey = defaultHub.Key
	}
	matHubKey := strings.TrimSpace(args.MaterialHub)
	if matHubKey == "" {
		matHubKey = hubKey
	}
	refineryKey := strings.TrimSpace(args.Refinery)
	if refineryKey == "" {
		refineryKey = "athanor_t1"
	}
	scope := strings.TrimSpace(args.Scope)
	if scope == "" {
		scope = "hybrid"
	}
	profOnly := true
	if args.ProfitableOnly != nil {
		profOnly = *args.ProfitableOnly
	}

	rows, err := s.ScanGasReactions(ctx, GasReactionScanRequest{
		HubKey:              hubKey,
		MaterialHubKey:      matHubKey,
		RefineryKey:         refineryKey,
		Scope:               scope,
		Runs:                0, // Auto ~24h cycle
		MaterialPriceSource: "jita_sell",
		ProfitableOnly:      profOnly,
	})
	if err != nil {
		return nil, "", err
	}
	if len(rows) > 10 {
		rows = rows[:10]
	}

	type reactionSummary struct {
		Name           string  `json:"name"`
		Runs24h        int     `json:"runs_24h"`
		OutputQty      int     `json:"output_qty"`
		CapitalISK     float64 `json:"capital_isk"`
		NetProfit24h   float64 `json:"net_profit_24h"`
		MarginPct      float64 `json:"margin_pct"`
		ISKPerHour     float64 `json:"isk_per_hour"`
		EstISKPerDay   float64 `json:"est_isk_per_day_per_slot"`
		RecSellPrice   float64 `json:"rec_sell_price"`
		BreakEvenFloor float64 `json:"break_even_floor"`
		DailyVolume    float64 `json:"daily_volume"`
		InputsSummary  string  `json:"inputs_summary"`
		Verdict        string  `json:"verdict"`
	}

	items := make([]reactionSummary, 0, len(rows))
	for _, r := range rows {
		items = append(items, reactionSummary{
			Name:           r.ProductName,
			Runs24h:        r.Runs,
			OutputQty:      r.OutputQty,
			CapitalISK:     math.Round(r.InputCost + r.JobFeeISK),
			NetProfit24h:   math.Round(r.NetProfit),
			MarginPct:      math.Round(r.NetMarginPercent*10) / 10,
			ISKPerHour:     math.Round(r.ISKPerHour),
			EstISKPerDay:   math.Round(r.ISKPerHour * 24),
			RecSellPrice:   r.RecommendedSell,
			BreakEvenFloor: r.BreakEvenSell,
			DailyVolume:    math.Round(r.DailyVolume7d*10) / 10,
			InputsSummary:  r.InputsSummary,
			Verdict:        string(r.Verdict),
		})
	}

	res := map[string]any{
		"sell_hub":     domain.LookupTradeHub(hubKey).Name,
		"material_hub": domain.LookupTradeHub(matHubKey).Name,
		"refinery":     domain.LookupRefineryProfile(refineryKey).Name,
		"scope":        scope,
		"count":        len(items),
		"reactions":    items,
	}
	summary := fmt.Sprintf("Gas Reactions (%s, %s): %d profitable reaction(s)", domain.LookupTradeHub(hubKey).Name, scope, len(items))
	return res, summary, nil
}

func (s *CopilotService) runEvaluateT2Invention(
	ctx context.Context,
	defaultHub domain.TradeHub,
	argsRaw string,
) (any, string, error) {
	var args struct {
		PresetKey       string `json:"preset_key"`
		Hub             string `json:"hub"`
		EncryptionSkill int    `json:"encryption_skill"`
		DatacoreSkill   int    `json:"datacore_skill"`
	}
	_ = json.Unmarshal([]byte(argsRaw), &args)

	hubKey := strings.TrimSpace(args.Hub)
	if hubKey == "" {
		hubKey = defaultHub.Key
	}
	encSkill := args.EncryptionSkill
	if encSkill < 1 || encSkill > 5 {
		encSkill = 4
	}
	dcSkill := args.DatacoreSkill
	if dcSkill < 1 || dcSkill > 5 {
		dcSkill = 4
	}

	presetKey := strings.ToLower(strings.TrimSpace(args.PresetKey))
	if presetKey != "" && presetKey != "all" {
		opt, err := s.OptimizeT2InventionLive(ctx, domain.InventionInput{
			PresetKey:       presetKey,
			HubKey:          hubKey,
			EncryptionSkill: encSkill,
			Datacore1Skill:  dcSkill,
			Datacore2Skill:  dcSkill,
		})
		if err != nil {
			return nil, "", err
		}
		topOpts := opt.Scenarios
		if len(topOpts) > 3 {
			topOpts = topOpts[:3]
		}
		res := map[string]any{
			"hub":           opt.HubName,
			"product":       opt.Preset.ProductName,
			"preset_key":    opt.Preset.Key,
			"best_scenario": opt.BestScenario,
			"top_3_options": topOpts,
		}
		summary := fmt.Sprintf("T2 Invention %s (%s): best=%s (%.1f%% chance, net %s/BPC)",
			opt.Preset.ProductName, opt.HubName, opt.BestScenario.Decryptor.Name, opt.BestScenario.SuccessChancePct, fmtCompactISK(opt.BestScenario.NetProfitPerBPC))
		return res, summary, nil
	}

	// Evaluate all 8 canonical T2 presets in parallel (bounded 4 workers) and rank by NetProfitPerBPC.
	type rankedT2 struct {
		PresetKey        string  `json:"preset_key"`
		ProductName      string  `json:"product_name"`
		Category         string  `json:"category"`
		BestDecryptor    string  `json:"best_decryptor"`
		SuccessChancePct float64 `json:"success_chance_pct"`
		T2RunsPerBPC     int     `json:"t2_runs_per_bpc"`
		ResultME         int     `json:"result_me"`
		NetProfitPerBPC  float64 `json:"net_profit_per_bpc"`
		NetProfitPerUnit float64 `json:"net_profit_per_unit"`
		MarginPct        float64 `json:"margin_pct"`
		Verdict          string  `json:"verdict"`
	}

	results := make([]rankedT2, 0, len(domain.DefaultT2Presets))
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		sem = make(chan struct{}, 4)
	)
	for _, p := range domain.DefaultT2Presets {
		wg.Add(1)
		go func(preset domain.T2InventionPreset) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			opt, err := s.OptimizeT2InventionLive(ctx, domain.InventionInput{
				PresetKey:       preset.Key,
				HubKey:          hubKey,
				EncryptionSkill: encSkill,
				Datacore1Skill:  dcSkill,
				Datacore2Skill:  dcSkill,
			})
			if err != nil || opt == nil {
				return
			}
			best := opt.BestScenario
			verdict := "VIABLE"
			if best.NetProfitPerBPC <= 0 {
				verdict = "AVOID"
			} else if best.NetMarginPercent < 10 {
				verdict = "CAUTION"
			}
			mu.Lock()
			results = append(results, rankedT2{
				PresetKey:        preset.Key,
				ProductName:      preset.ProductName,
				Category:         preset.Category,
				BestDecryptor:    best.Decryptor.Name,
				SuccessChancePct: math.Round(best.SuccessChancePct*10) / 10,
				T2RunsPerBPC:     best.ResultRuns,
				ResultME:         best.ResultME,
				NetProfitPerBPC:  math.Round(best.NetProfitPerBPC),
				NetProfitPerUnit: math.Round(best.NetProfitPerUnit),
				MarginPct:        math.Round(best.NetMarginPercent*10) / 10,
				Verdict:          verdict,
			})
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].NetProfitPerBPC > results[j].NetProfitPerBPC
	})

	res := map[string]any{
		"hub":               domain.LookupTradeHub(hubKey).Name,
		"skills_assumed":    fmt.Sprintf("Encryption %d / Datacores %d", encSkill, dcSkill),
		"evaluated_presets": len(results),
		"ranked_t2":         results,
	}
	summary := fmt.Sprintf("T2 Invention scan (%s): evaluated %d canonical T2 items", domain.LookupTradeHub(hubKey).Name, len(results))
	return res, summary, nil
}
