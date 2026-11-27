package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aura-forge/internal/domain"
)

type esiPublicContract struct {
	ContractID      int64   `json:"contract_id"`
	Type            string  `json:"type"`
	Price           float64 `json:"price"`
	Volume          float64 `json:"volume"`
	StartLocationID int64   `json:"start_location_id"`
	DateExpired     string  `json:"date_expired"`
	Title           string  `json:"title"`
	ForCorporation  bool    `json:"for_corporation"`
}

type esiPublicContractItem struct {
	RecordID           int64 `json:"record_id"`
	TypeID             int32 `json:"type_id"`
	Quantity           int   `json:"quantity"`
	IsIncluded         bool  `json:"is_included"`
	IsBlueprintCopy    bool  `json:"is_blueprint_copy"`
	MaterialEfficiency int   `json:"material_efficiency"`
	TimeEfficiency     int   `json:"time_efficiency"`
	Runs               int   `json:"runs"`
}

// FindPublicBlueprintContracts scans cached Public ESI item_exchange contracts in a region for a specific blueprintTypeID.
// Uses EVE's 0.01 m³ blueprint volume heuristic + title matching to avoid unnecessary /items calls while respecting
// bounded goroutines, X-Pages caps, and esi_cache.
func (p *PublicMarketClient) FindPublicBlueprintContracts(
	ctx context.Context,
	regionID int32,
	regionName string,
	blueprintTypeID int32,
	blueprintName string,
	productName string,
) ([]domain.PublicBPCContract, int, int, int, error) {
	if regionID <= 0 || blueprintTypeID <= 0 {
		return nil, 0, 0, 0, nil
	}

	const maxContractPages = 15
	const maxInspectContracts = 50

	pathPage1 := fmt.Sprintf("/contracts/public/%d/?datasource=tranquility&page=1", regionID)
	body1, hdrs1, err := p.client.DoGetWithHeaders(ctx, pathPage1, "", true)
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("fetching public contracts page 1: %w", err)
	}

	var allRaw []esiPublicContract
	if err := json.Unmarshal(body1, &allRaw); err != nil {
		return nil, 0, 0, 0, fmt.Errorf("decoding public contracts page 1: %w", err)
	}

	totalPages := 1
	if rawPages := strings.TrimSpace(hdrs1.Get("X-Pages")); rawPages != "" {
		if parsed, pErr := strconv.Atoi(rawPages); pErr == nil && parsed > 1 {
			totalPages = parsed
		}
	}
	scannedPages := totalPages
	if scannedPages > maxContractPages {
		scannedPages = maxContractPages
	}

	// Fetch remaining contract pages in parallel (bounded to 5 concurrent ESI requests)
	if scannedPages > 1 {
		var (
			mu  sync.Mutex
			wg  sync.WaitGroup
			sem = make(chan struct{}, 5)
		)
		for page := 2; page <= scannedPages; page++ {
			wg.Add(1)
			go func(pg int) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				pagePath := fmt.Sprintf("/contracts/public/%d/?datasource=tranquility&page=%d", regionID, pg)
				b, _, pErr := p.client.DoGetWithHeaders(ctx, pagePath, "", true)
				if pErr != nil {
					return
				}
				var pageContracts []esiPublicContract
				if jErr := json.Unmarshal(b, &pageContracts); jErr == nil && len(pageContracts) > 0 {
					mu.Lock()
					allRaw = append(allRaw, pageContracts...)
					mu.Unlock()
				}
			}(page)
		}
		wg.Wait()
	}

	// Pre-filter item_exchange contracts that can plausibly contain blueprints (volume <= 1.0 m³; every EVE BP is 0.01 m³)
	bpLower := strings.ToLower(strings.TrimSpace(blueprintName))
	prodLower := strings.ToLower(strings.TrimSpace(productName))
	titleMatches := make([]esiPublicContract, 0, 24)
	volCandidates := make([]esiPublicContract, 0, 64)

	for _, c := range allRaw {
		if c.Type != "item_exchange" || c.Price <= 0 || c.ForCorporation {
			continue
		}
		if c.Volume <= 0 || c.Volume > 1.0 {
			continue
		}
		titleLower := strings.ToLower(c.Title)
		if (prodLower != "" && strings.Contains(titleLower, prodLower)) ||
			(bpLower != "" && strings.Contains(titleLower, bpLower)) {
			titleMatches = append(titleMatches, c)
		} else {
			volCandidates = append(volCandidates, c)
		}
	}

	// Sort each pool by cheapest price first so we inspect the best deals first
	sort.Slice(titleMatches, func(i, j int) bool { return titleMatches[i].Price < titleMatches[j].Price })
	sort.Slice(volCandidates, func(i, j int) bool { return volCandidates[i].Price < volCandidates[j].Price })

	toInspect := make([]esiPublicContract, 0, maxInspectContracts)
	toInspect = append(toInspect, titleMatches...)
	if len(toInspect) < maxInspectContracts {
		remain := maxInspectContracts - len(toInspect)
		if remain > len(volCandidates) {
			remain = len(volCandidates)
		}
		toInspect = append(toInspect, volCandidates[:remain]...)
	}
	if len(toInspect) > maxInspectContracts {
		toInspect = toInspect[:maxInspectContracts]
	}

	// Inspect candidate contract items in parallel (bounded to 6 concurrent ESI requests)
	var (
		resMu   sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, 6)
		matches = make([]domain.PublicBPCContract, 0, 12)
	)
	for _, cand := range toInspect {
		wg.Add(1)
		go func(c esiPublicContract) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			itemPath := fmt.Sprintf("/contracts/public/items/%d/?datasource=tranquility", c.ContractID)
			body, iErr := p.client.DoGet(ctx, itemPath, "", true)
			if iErr != nil {
				return
			}
			var items []esiPublicContractItem
			if jErr := json.Unmarshal(body, &items); jErr != nil || len(items) == 0 {
				return
			}

			var matchedCount, totalRuns, bestME, bestTE int
			isCopy := false
			for _, it := range items {
				if !it.IsIncluded || it.TypeID != blueprintTypeID {
					continue
				}
				qty := it.Quantity
				if qty <= 0 {
					qty = 1
				}
				matchedCount += qty
				if it.IsBlueprintCopy {
					isCopy = true
				}
				if it.Runs > 0 {
					totalRuns += it.Runs * qty
				}
				if it.MaterialEfficiency > bestME {
					bestME = it.MaterialEfficiency
				}
				if it.TimeEfficiency > bestTE {
					bestTE = it.TimeEfficiency
				}
			}
			if matchedCount == 0 {
				return
			}

			pricePerRun := 0.0
			if totalRuns > 0 {
				pricePerRun = domain.RoundISK(c.Price / float64(totalRuns))
			}
			expTime, _ := time.Parse(time.RFC3339, c.DateExpired)

			resMu.Lock()
			matches = append(matches, domain.PublicBPCContract{
				ContractID:         c.ContractID,
				BlueprintTypeID:    blueprintTypeID,
				BlueprintName:      blueprintName,
				ProductName:        productName,
				Price:              domain.RoundISK(c.Price),
				PricePerRun:        pricePerRun,
				Runs:               totalRuns,
				MaterialEfficiency: bestME,
				TimeEfficiency:     bestTE,
				IsBlueprintCopy:    isCopy,
				Quantity:           matchedCount,
				StartLocationID:    c.StartLocationID,
				StationName:        fmt.Sprintf("Location #%d", c.StartLocationID),
				RegionName:         regionName,
				DateExpired:        expTime,
				Title:              strings.TrimSpace(c.Title),
			})
			resMu.Unlock()
		}(cand)
	}
	wg.Wait()

	// Resolve NPC station names for matched contracts in parallel (bounded to 4)
	stationCache := make(map[int64]string)
	var stMu sync.Mutex
	var stWg sync.WaitGroup
	stSem := make(chan struct{}, 4)
	uniqueLocs := make(map[int64]struct{})
	for _, m := range matches {
		if m.StartLocationID > 0 && m.StartLocationID < 100_000_000 {
			uniqueLocs[m.StartLocationID] = struct{}{}
		}
	}
	for locID := range uniqueLocs {
		stWg.Add(1)
		go func(id int64) {
			defer stWg.Done()
			select {
			case stSem <- struct{}{}:
				defer func() { <-stSem }()
			case <-ctx.Done():
				return
			}
			stPath := fmt.Sprintf("/universe/stations/%d/?datasource=tranquility", id)
			if b, sErr := p.client.DoGet(ctx, stPath, "", true); sErr == nil {
				var st esiStationDetail
				if json.Unmarshal(b, &st) == nil && st.Name != "" {
					stMu.Lock()
					stationCache[id] = st.Name
					stMu.Unlock()
				}
			}
		}(locID)
	}
	stWg.Wait()

	for i := range matches {
		if name, ok := stationCache[matches[i].StartLocationID]; ok {
			matches[i].StationName = name
		} else if matches[i].StartLocationID >= 100_000_000 {
			matches[i].StationName = fmt.Sprintf("Player Structure #%d (%s)", matches[i].StartLocationID, regionName)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].PricePerRun > 0 && matches[j].PricePerRun > 0 && matches[i].PricePerRun != matches[j].PricePerRun {
			return matches[i].PricePerRun < matches[j].PricePerRun
		}
		return matches[i].Price < matches[j].Price
	})
	if len(matches) > 25 {
		matches = matches[:25]
	}

	return matches, len(allRaw), scannedPages, totalPages, nil
}
