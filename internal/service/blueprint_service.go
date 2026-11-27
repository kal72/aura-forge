package service

import (
	"context"
	"fmt"
	"strings"

	"aura-forge/internal/domain"
)

// SetSDERepository wires the optional EVE SDE blueprint repository into CopilotService.
func (s *CopilotService) SetSDERepository(sde domain.SDERepository) {
	s.sde = sde
}

// ResolveBlueprintID resolves a blueprint_type_id from an explicit blueprint ID,
// a product_type_id (e.g. from Catalog or Marketplace), or a text query (item/blueprint name).
func (s *CopilotService) ResolveBlueprintID(ctx context.Context, bpID, productID int32, query string) int32 {
	if s.sde == nil {
		return 0
	}
	if bpID > 0 {
		if bp, err := s.sde.GetBlueprintByID(ctx, bpID); err == nil && bp != nil {
			return bp.BlueprintTypeID
		}
		// Fallback in case a caller passed product_type_id in bp_id
		if bp, err := s.sde.GetBlueprintByProductID(ctx, bpID); err == nil && bp != nil {
			return bp.BlueprintTypeID
		}
	}
	if productID > 0 {
		if bp, err := s.sde.GetBlueprintByProductID(ctx, productID); err == nil && bp != nil {
			return bp.BlueprintTypeID
		}
	}
	q := strings.TrimSpace(query)
	if q != "" {
		if list, err := s.sde.SearchBlueprints(ctx, q, "", 20); err == nil && len(list) > 0 {
			for _, bp := range list {
				if strings.EqualFold(bp.ProductName, q) || strings.EqualFold(bp.Name, q) {
					return bp.BlueprintTypeID
				}
			}
			return list[0].BlueprintTypeID
		}
	}
	return 0
}

// SearchBlueprints lists matching EVE SDE blueprints for Blueprint Calculator Mode.
func (s *CopilotService) SearchBlueprints(ctx context.Context, query, category string, limit int) ([]domain.SDEBlueprint, error) {
	if s.sde == nil {
		return []domain.SDEBlueprint{}, nil
	}
	return s.sde.SearchBlueprints(ctx, query, category, limit)
}

// CalculateBlueprintProfit loads an SDE blueprint and recursively expands all sub-blueprints (up to depth 5),
// fetches live trade-hub prices (cache-first, 4-worker bounded) for every unique TypeID in the tree,
// and computes ME/TE-adjusted BOM, Optimal Make-vs-Buy decisions, consolidated Multibuy, Sub-Job Queue, ISK/hour, and profit.
func (s *CopilotService) CalculateBlueprintProfit(ctx context.Context, in domain.BlueprintCalcInput) (*domain.BlueprintCalcResult, error) {
	if s.sde == nil {
		return nil, fmt.Errorf("EVE SDE blueprint database is not available")
	}
	if in.BlueprintTypeID <= 0 {
		return nil, fmt.Errorf("please select a blueprint to calculate")
	}

	bp, err := s.sde.GetBlueprintByID(ctx, in.BlueprintTypeID)
	if err != nil {
		return nil, fmt.Errorf("loading blueprint: %w", err)
	}
	if bp == nil {
		return nil, fmt.Errorf("blueprint #%d was not found in the SDE catalog", in.BlueprintTypeID)
	}

	reqs, err := s.sde.GetBlueprintMaterials(ctx, bp.BlueprintTypeID)
	if err != nil {
		return nil, fmt.Errorf("loading blueprint materials: %w", err)
	}
	if len(reqs) == 0 {
		return nil, fmt.Errorf("no Bill of Materials found for %s", bp.Name)
	}

	// Fill default tax/SCI/freight settings if not explicitly supplied
	settings, _ := s.GetSettings(ctx)
	if settings != nil {
		if in.SystemCostIndex <= 0 {
			in.SystemCostIndex = settings.SystemCostIndex
		}
		if in.FacilityTaxPercent <= 0 {
			in.FacilityTaxPercent = settings.FacilityTaxPercent
		}
		if in.BrokerFeePercent <= 0 {
			in.BrokerFeePercent = settings.BrokerFeePercent
		}
		if in.SalesTaxPercent <= 0 {
			in.SalesTaxPercent = settings.SalesTaxPercent
		}
		if in.RelistFeePercent <= 0 {
			in.RelistFeePercent = settings.RelistFeePercent
		}
		if in.FreightCostPerM3 < 0 {
			in.FreightCostPerM3 = settings.FreightCostPerM3
		}
	}

	hub := domain.LookupTradeHub(in.HubKey)
	in.HubKey = hub.Key

	// Recursively load sub-blueprints and their materials from PostgreSQL SDE (0 ESI calls)
	bpsByProduct := make(map[int32]domain.SDEBlueprint)
	matsByBP := map[int32][]domain.SDEMaterialRequirement{
		bp.BlueprintTypeID: reqs,
	}

	seenTypeIDs := make(map[int32]struct{}, len(reqs)*2+4)
	typeIDs := make([]int32, 0, len(reqs)*2+4)
	addID := func(id int32) {
		if id <= 0 {
			return
		}
		if _, ok := seenTypeIDs[id]; !ok {
			seenTypeIDs[id] = struct{}{}
			typeIDs = append(typeIDs, id)
		}
	}
	addID(bp.ProductTypeID)

	visitedProduct := map[int32]bool{bp.ProductTypeID: true}
	var expandMaterials func(list []domain.SDEMaterialRequirement, depth int)
	expandMaterials = func(list []domain.SDEMaterialRequirement, depth int) {
		for _, r := range list {
			addID(r.MaterialTypeID)
			if depth >= 5 || visitedProduct[r.MaterialTypeID] {
				continue
			}
			subBP, subErr := s.sde.GetBlueprintByProductID(ctx, r.MaterialTypeID)
			if subErr != nil || subBP == nil {
				continue
			}
			subMats, mErr := s.sde.GetBlueprintMaterials(ctx, subBP.BlueprintTypeID)
			if mErr != nil || len(subMats) == 0 {
				continue
			}
			visitedProduct[r.MaterialTypeID] = true
			bpsByProduct[r.MaterialTypeID] = *subBP
			matsByBP[subBP.BlueprintTypeID] = subMats
			expandMaterials(subMats, depth+1)
		}
	}
	expandMaterials(reqs, 1)

	// Fetch live trade-hub prices for every unique TypeID in the recursive tree (cache-first, 4-worker bounded)
	prices := make(map[int32]domain.JitaPrice)
	if s.market != nil && len(typeIDs) > 0 {
		if fetched, pErr := s.fetchHubPrices(ctx, hub, typeIDs); pErr == nil && fetched != nil {
			prices = fetched
		}
	}

	res, err := domain.CalculateBlueprintManufacturing(*bp, reqs, in, prices)
	if err != nil {
		return nil, err
	}
	res.PriceHub = hub

	// Evaluate Recursive Industry Tree & Optimal Make-vs-Buy
	tree := domain.EvaluateIndustryTree(*bp, bpsByProduct, matsByBP, prices, in)
	res.IndustryTree = tree

	// When sub-blueprints exist, recompute headline KPIs & Multibuy using the active Optimal Make/Buy plan
	if tree != nil && tree.HasSubBlueprints {
		// OptimalCost already includes sub-job + root job install fees; subtract root job fee to pass clean material cost to CalculateProfit
		optMaterialCost := domain.RoundISK(tree.OptimalCost - tree.RootNode.JobInstallCost)
		if optMaterialCost < 0 {
			optMaterialCost = 0
		}
		res.TotalMaterialCost = optMaterialCost
		res.TotalMaterialVolumeM3 = tree.OptimalVolumeM3
		res.FreightCost = domain.RoundISK(tree.OptimalVolumeM3 * in.FreightCostPerM3)
		res.MultibuyText = tree.OptimalMultibuy

		profitIn := domain.ProfitCalcInput{
			ItemName:           bp.ProductName,
			OutputQty:          res.TotalOutputQty,
			UnitSellPrice:      res.ProductUnitSellPrice,
			MaterialCost:       optMaterialCost,
			ExtraSetupCost:     in.ExtraSetupCost,
			SystemCostIndex:    in.SystemCostIndex,
			FacilityTaxPercent: in.FacilityTaxPercent,
			FreightCost:        res.FreightCost,
			BrokerFeePercent:   in.BrokerFeePercent,
			SalesTaxPercent:    in.SalesTaxPercent,
			RelistFeePercent:   in.RelistFeePercent,
			EstimatedRelists:   in.EstimatedRelists,
			BatchesPerWeek:     in.BatchesPerWeek,
		}
		if optProfit, pErr := domain.CalculateProfit(profitIn); pErr == nil {
			res.Profit = optProfit
			if res.EstimatedDurationSeconds > 0 {
				hours := float64(res.EstimatedDurationSeconds) / 3600.0
				res.ISKPerHour = domain.RoundISK(optProfit.NetProfitPerBatch / hours)
			}
		}
	}

	return &res, nil
}
