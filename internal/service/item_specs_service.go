package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"aura-forge/internal/domain"
)

// CompareItems resolves 1 to 3 EVE Online ships, modules, drones, or rigs concurrently,
// fetches their full Dogma attributes, resistance profiles, slot layouts, required skills,
// and live trade-hub sell prices, and builds a visual comparison table.
func (s *CopilotService) CompareItems(ctx context.Context, queries []string, hubKey string) (*domain.ItemCompareResult, error) {
	specProv, ok := s.market.(domain.ItemSpecProvider)
	if !ok || specProv == nil {
		return nil, fmt.Errorf("item specifications provider is not available")
	}

	cleaned := make([]string, 0, 3)
	for _, raw := range queries {
		q := strings.TrimSpace(raw)
		if q != "" {
			cleaned = append(cleaned, q)
			if len(cleaned) >= 3 {
				break
			}
		}
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("please enter at least 1 ship or module name (e.g. Caracal, Caracal Navy Issue, Damage Control II)")
	}

	hub := s.ActiveCatalogHub(ctx)
	if strings.TrimSpace(hubKey) != "" {
		hub = domain.LookupTradeHub(hubKey)
	}

	// Resolve each query -> typeID preserving slot order
	resolvedIDs := make([]int32, len(cleaned))
	resolveErrs := make([]error, len(cleaned))
	var resWg sync.WaitGroup
	for i, q := range cleaned {
		resWg.Add(1)
		go func(idx int, query string) {
			defer resWg.Done()
			tid, err := s.resolveCompareQueryTypeID(ctx, query)
			if err != nil {
				resolveErrs[idx] = err
				return
			}
			resolvedIDs[idx] = tid
		}(i, q)
	}
	resWg.Wait()

	validIDs := make([]int32, 0, len(resolvedIDs))
	seen := make(map[int32]struct{}, len(resolvedIDs))
	for i, tid := range resolvedIDs {
		if resolveErrs[i] != nil || tid <= 0 {
			continue
		}
		if _, dup := seen[tid]; dup {
			continue
		}
		seen[tid] = struct{}{}
		validIDs = append(validIDs, tid)
	}
	if len(validIDs) == 0 {
		for _, e := range resolveErrs {
			if e != nil {
				return nil, e
			}
		}
		return nil, fmt.Errorf("could not resolve any matching EVE item")
	}

	// Fetch specs, live hub prices, and blueprint IDs concurrently
	specs := make([]*domain.ItemSpecDetail, len(validIDs))
	var (
		prices   map[int32]domain.JitaPrice
		fetchWg  sync.WaitGroup
	)
	fetchWg.Add(1)
	go func() {
		defer fetchWg.Done()
		prices, _ = s.fetchHubPrices(ctx, hub, validIDs)
	}()

	for i, tid := range validIDs {
		fetchWg.Add(1)
		go func(idx int, id int32) {
			defer fetchWg.Done()
			sp, err := specProv.GetItemSpecs(ctx, id)
			if err != nil || sp == nil {
				return
			}
			if s.sde != nil {
				if bp, bErr := s.sde.GetBlueprintByProductID(ctx, id); bErr == nil && bp != nil {
					sp.BlueprintTypeID = bp.BlueprintTypeID
				}
			}
			specs[idx] = sp
		}(i, tid)
	}
	fetchWg.Wait()

	items := make([]domain.ItemSpecDetail, 0, len(validIDs))
	for _, sp := range specs {
		if sp == nil {
			continue
		}
		if jp, ok := prices[sp.TypeID]; ok && jp.LowestSell > 0 {
			sp.LiveSellPrice = domain.RoundISK(jp.LowestSell)
		}
		items = append(items, *sp)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("failed to load item specifications from EVE ESI")
	}

	return domain.BuildItemCompareResult(items, hub), nil
}

func (s *CopilotService) resolveCompareQueryTypeID(ctx context.Context, query string) (int32, error) {
	q := strings.TrimSpace(query)
	if id, err := strconv.ParseInt(q, 10, 32); err == nil && id > 0 {
		return int32(id), nil
	}
	// Fast local SDE lookup first
	matches := s.SearchMarketplaceTypes(ctx, q, 8)
	for _, m := range matches {
		if strings.EqualFold(strings.TrimSpace(m.Name), q) {
			return m.TypeID, nil
		}
	}
	if len(matches) > 0 {
		return matches[0].TypeID, nil
	}
	// Fallback to Public ESI /universe/ids/
	if s.market != nil {
		if idMap, err := s.market.ResolveTypeIDs(ctx, []string{q}); err == nil {
			for k, id := range idMap {
				if strings.EqualFold(k, q) && id > 0 {
					return id, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("item %q not found in EVE SDE or ESI", q)
}
