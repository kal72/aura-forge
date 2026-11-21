package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

// SetPIRepository wires the PostgreSQL PIRepository into CopilotService.
func (s *CopilotService) SetPIRepository(repo domain.PIRepository) {
	s.pi = repo
}

// InspectPIRecipeTree builds the full recursive P4->P3->P2->P1->P0 recipe tree with required source planets
// and live Jita Public ESI pricing.
func (s *CopilotService) InspectPIRecipeTree(ctx context.Context, typeID int32, pocoTaxPct float64) (*domain.PIRecipeAnalysis, error) {
	if typeID <= 0 {
		return nil, fmt.Errorf("please select a PI item to inspect")
	}
	typeIDs := domain.CollectPITreeTypeIDs(typeID)
	prices := make(map[int32]domain.JitaPrice)
	if s.market != nil && len(typeIDs) > 0 {
		if fetched, err := s.market.GetJitaPrices(ctx, typeIDs); err == nil && fetched != nil {
			prices = fetched
		}
	}
	return domain.BuildPIRecipeTree(typeID, prices, pocoTaxPct)
}

// ScanPIProfitability scans all PI items in a chosen tier (P1, P2, P3, or P4) against live Jita Public ESI prices
// and ranks them by estimated monthly factory profit.
func (s *CopilotService) ScanPIProfitability(ctx context.Context, tier string, pocoTaxPct float64) ([]domain.PIProfitRow, error) {
	if s.market == nil {
		return nil, fmt.Errorf("Public ESI market provider is not available")
	}
	tier = strings.ToUpper(strings.TrimSpace(tier))
	if tier == "" || tier == "ALL" {
		tier = "P2"
	}

	items := domain.ListProduciblePICommodities(tier, "")
	if len(items) == 0 {
		return []domain.PIProfitRow{}, nil
	}

	// Collect unique TypeIDs for output + direct inputs in one batch
	seen := make(map[int32]struct{})
	typeIDs := make([]int32, 0, len(items)*3)
	addID := func(id int32) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			typeIDs = append(typeIDs, id)
		}
	}
	for _, it := range items {
		addID(it.TypeID)
		for _, req := range it.Inputs {
			if c, ok := domain.GetPICommodity(req.TypeID); ok && c.Tier != domain.PITierP0 {
				addID(req.TypeID)
			}
		}
	}

	prices, err := s.market.GetJitaPrices(ctx, typeIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching live Jita PI prices: %w", err)
	}

	rows := make([]domain.PIProfitRow, 0, len(items))
	for _, it := range items {
		analysis, err := domain.BuildPIRecipeTree(it.TypeID, prices, pocoTaxPct)
		if err != nil || analysis.OutputUnitSell <= 0 {
			continue
		}
		verdict := domain.VerdictAvoid
		switch {
		case analysis.NetProfitPerCycle > 0 && analysis.NetMarginPercent >= 15.0:
			verdict = domain.VerdictStrong
		case analysis.NetProfitPerCycle > 0 && analysis.NetMarginPercent >= 6.0:
			verdict = domain.VerdictModerate
		case analysis.NetProfitPerCycle > 0:
			verdict = domain.VerdictWeak
		}
		rows = append(rows, domain.PIProfitRow{
			TypeID:            it.TypeID,
			Name:              it.Name,
			Tier:              it.Tier,
			OutputUnitSell:    analysis.OutputUnitSell,
			DirectInputCost:   analysis.DirectInputCost,
			POCOTaxISK:        analysis.POCOTaxISK,
			NetProfitPerCycle: analysis.NetProfitPerCycle,
			NetMarginPercent:  analysis.NetMarginPercent,
			EstMonthlyISK:     analysis.EstMonthlyISK,
			Verdict:           verdict,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].EstMonthlyISK > rows[j].EstMonthlyISK
	})
	return rows, nil
}

// ListPIColonies returns all tracked player PI colonies ordered by soonest extractor expiry.
func (s *CopilotService) ListPIColonies(ctx context.Context) ([]domain.PIColony, error) {
	if s.pi == nil {
		return []domain.PIColony{}, nil
	}
	return s.pi.ListColonies(ctx)
}

// SavePIColony persists a new PI colony entry.
func (s *CopilotService) SavePIColony(ctx context.Context, c *domain.PIColony) (*domain.PIColony, error) {
	if s.pi == nil {
		return nil, fmt.Errorf("PI colony repository is not available")
	}
	if c == nil || strings.TrimSpace(c.PlanetName) == "" || strings.TrimSpace(c.ProductName) == "" {
		return nil, fmt.Errorf("planet name and target product are required")
	}
	if c.ExtractorExpiresAt.IsZero() {
		c.ExtractorExpiresAt = time.Now().Add(48 * time.Hour)
	}
	if err := s.pi.SaveColony(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ResetPIColonyExtractor extends a colony's extractor timer by N hours from now.
func (s *CopilotService) ResetPIColonyExtractor(ctx context.Context, id int64, hours int) error {
	if s.pi == nil {
		return nil
	}
	if hours <= 0 {
		hours = 48
	}
	return s.pi.ResetExtractor(ctx, id, time.Now().Add(time.Duration(hours)*time.Hour))
}

// DeletePIColony deletes a PI colony record.
func (s *CopilotService) DeletePIColony(ctx context.Context, id int64) error {
	if s.pi == nil {
		return nil
	}
	return s.pi.DeleteColony(ctx, id)
}
