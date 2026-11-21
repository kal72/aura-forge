package service

import (
	"context"
	"fmt"

	"aura-forge/internal/domain"
)

// SetSDERepository wires the optional EVE SDE blueprint repository into CopilotService.
func (s *CopilotService) SetSDERepository(sde domain.SDERepository) {
	s.sde = sde
}

// SearchBlueprints lists matching EVE SDE blueprints for Blueprint Calculator Mode.
func (s *CopilotService) SearchBlueprints(ctx context.Context, query, category string, limit int) ([]domain.SDEBlueprint, error) {
	if s.sde == nil {
		return []domain.SDEBlueprint{}, nil
	}
	return s.sde.SearchBlueprints(ctx, query, category, limit)
}

// CalculateBlueprintProfit loads an SDE blueprint and its BOM, fetches live Jita material/product prices via Public ESI,
// and computes ME/TE-adjusted build cost, duration, ISK/hour, net profit, and EVE Multibuy text.
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

	// Collect unique TypeIDs (product + all materials) for batch Jita Public ESI lookup
	seen := make(map[int32]struct{}, len(reqs)+1)
	typeIDs := make([]int32, 0, len(reqs)+1)
	addID := func(id int32) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			typeIDs = append(typeIDs, id)
		}
	}
	addID(bp.ProductTypeID)
	for _, r := range reqs {
		addID(r.MaterialTypeID)
	}

	prices := make(map[int32]domain.JitaPrice)
	if s.market != nil && len(typeIDs) > 0 {
		if fetched, pErr := s.market.GetJitaPrices(ctx, typeIDs); pErr == nil && fetched != nil {
			prices = fetched
		}
	}

	res, err := domain.CalculateBlueprintManufacturing(*bp, reqs, in, prices)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
