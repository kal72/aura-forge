package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"aura-forge/internal/domain"
)

// FindBPCAndBPOSources resolves a blueprint by name or product name from CCP SDE, then concurrently fetches:
// 1. Live Public Item Exchange Contracts (BPC & researched BPO) in the chosen Trade Hub region
// 2. Official NPC BPO seed sell orders (duration=365) in the chosen Trade Hub region
// 3. Major NPC Corporation LP Store offers (for Faction BPCs)
func (s *CopilotService) FindBPCAndBPOSources(ctx context.Context, query string, hubKey string) (*domain.BPCFinderResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("please enter an item or blueprint name (e.g. Hobgoblin I, Caracal, Caldari Navy Hookbill)")
	}
	hub := domain.LookupTradeHub(hubKey)

	if s.sde == nil {
		return nil, fmt.Errorf("CCP SDE blueprint catalog is not loaded yet — import SDE in Settings first")
	}

	bpID := s.ResolveBlueprintID(ctx, 0, 0, q)
	if bpID <= 0 {
		// Also try stripping " Blueprint" suffix if user typed it
		cleaned := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(q, " Blueprint"), " blueprint"))
		if cleaned != q && cleaned != "" {
			bpID = s.ResolveBlueprintID(ctx, 0, 0, cleaned)
		}
	}
	if bpID <= 0 {
		return nil, fmt.Errorf("no blueprint found in CCP SDE matching %q", q)
	}

	bp, err := s.sde.GetBlueprintByID(ctx, bpID)
	if err != nil || bp == nil {
		return nil, fmt.Errorf("failed to load blueprint #%d from SDE", bpID)
	}

	res := &domain.BPCFinderResult{
		Query:     q,
		Hub:       hub,
		Blueprint: bp,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// 1. Live Public BPC/BPO Contracts in selected hub region
	if contractProv, ok := s.market.(domain.PublicContractProvider); ok && contractProv != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			contracts, scanned, scannedPages, totalPages, cErr := contractProv.FindPublicBlueprintContracts(
				ctx, hub.RegionID, hub.RegionName, bp.BlueprintTypeID, bp.Name, bp.ProductName,
			)
			mu.Lock()
			defer mu.Unlock()
			if cErr != nil {
				res.Notice = "Public Contracts ESI lookup warning: " + cErr.Error()
				return
			}
			res.BPCContracts = contracts
			res.ScannedContracts = scanned
			res.ScannedPages = scannedPages
			res.TotalPages = totalPages
		}()
	}

	// 2. Official NPC BPO Seed Orders & Major LP Store Offers
	if intelProv, ok := s.market.(domain.UniverseIntelProvider); ok && intelProv != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seeds, sErr := intelProv.FindNPCBPOSeedOrders(ctx, hub.RegionID, hub.RegionName, bp.BlueprintTypeID)
			if sErr == nil && len(seeds) > 0 {
				mu.Lock()
				res.NPCBPOOrders = seeds
				mu.Unlock()
			}
		}()

		// Check major faction LP stores (same 10 curated corps as Aura AI)
		majorLPCorps := []struct {
			id   int32
			name string
		}{
			{1000035, "Caldari Navy"},
			{1000084, "Amarr Navy"},
			{1000120, "Federation Navy"},
			{1000051, "Republic Fleet"},
			{1000125, "Sisters of EVE"},
			{1000127, "Guristas"},
			{1000135, "Serpentis"},
			{1000124, "Arch Angels"},
			{1000134, "Blood Raiders"},
			{1000161, "Sansha's Nation"},
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			var lpWg sync.WaitGroup
			sem := make(chan struct{}, 4)
			var lpOffers []domain.LoyaltyStoreOffer
			var lpMu sync.Mutex
			for _, corp := range majorLPCorps {
				lpWg.Add(1)
				go func(cid int32, cname string) {
					defer lpWg.Done()
					select {
					case sem <- struct{}{}:
						defer func() { <-sem }()
					case <-ctx.Done():
						return
					}
					offers, oErr := intelProv.GetLoyaltyStoreOffers(ctx, cid, cname)
					if oErr != nil {
						return
					}
					for _, of := range offers {
						if of.TypeID == bp.BlueprintTypeID || of.TypeID == bp.ProductTypeID {
							lpMu.Lock()
							lpOffers = append(lpOffers, of)
							lpMu.Unlock()
						}
					}
				}(corp.id, corp.name)
			}
			lpWg.Wait()
			if len(lpOffers) > 0 {
				mu.Lock()
				res.LPStoreOffers = lpOffers
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return res, nil
}
