package service

import (
	"context"
	"fmt"
	"strings"

	"aura-forge/internal/domain"
)

// OptimizeT2InventionLive fetches live Trade Hub prices for all 8 Decryptors, the preset's 2 Datacores,
// and the T2 product from Public ESI (cached in PostgreSQL), then ranks all 9 Decryptor strategies.
func (s *CopilotService) OptimizeT2InventionLive(ctx context.Context, in domain.InventionInput) (*domain.InventionOptimizationResult, error) {
	preset := domain.LookupT2Preset(in.PresetKey)
	settings, _ := s.GetSettings(ctx)
	hub := domain.LookupTradeHub("jita")
	if settings != nil {
		hub = settings.ActiveTradeHub()
		if in.BrokerFeePercent <= 0 {
			in.BrokerFeePercent = settings.BrokerFeePercent
		}
		if in.SalesTaxPercent <= 0 {
			in.SalesTaxPercent = settings.SalesTaxPercent
		}
	}
	if strings.TrimSpace(in.HubKey) != "" {
		hub = domain.LookupTradeHub(in.HubKey)
	}

	if in.EncryptionSkill < 1 {
		in.EncryptionSkill = 4
	}
	if in.Datacore1Skill < 1 {
		in.Datacore1Skill = 4
	}
	if in.Datacore2Skill < 1 {
		in.Datacore2Skill = 4
	}

	// Collect unique TypeIDs: T2 Product + 2 Datacores + all 8 Official Decryptors
	seen := make(map[int32]struct{}, 12)
	typeIDs := make([]int32, 0, 12)
	addID := func(id int32) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			typeIDs = append(typeIDs, id)
		}
	}
	addID(preset.ProductTypeID)
	addID(preset.Datacore1TypeID)
	addID(preset.Datacore2TypeID)
	for _, d := range domain.OfficialDecryptors {
		addID(d.TypeID)
	}

	prices := make(map[int32]domain.JitaPrice)
	if s.market != nil && len(typeIDs) > 0 {
		fetched, err := s.fetchHubPrices(ctx, hub, typeIDs)
		if err != nil {
			return nil, fmt.Errorf("fetching live %s prices for T2 invention: %w", hub.Name, err)
		}
		prices = fetched
	}

	res := domain.OptimizeT2Invention(preset, in, prices)
	res.HubName = hub.Name
	return &res, nil
}

// EvaluateMakeVsBuy compares buying an intermediate component on the market vs manufacturing it in-house.
func (s *CopilotService) EvaluateMakeVsBuy(ctx context.Context, name string, qty int, jitaUnitPrice, rawMatPerUnit float64) (domain.MakeVsBuyResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.MakeVsBuyResult{}, fmt.Errorf("component name is required")
	}
	if jitaUnitPrice <= 0 || rawMatPerUnit <= 0 {
		return domain.MakeVsBuyResult{}, fmt.Errorf("both hub unit price and raw material cost per unit must be greater than 0")
	}
	settings, _ := s.GetSettings(ctx)
	sci := 0.045
	facTax := 0.25
	if settings != nil {
		sci = settings.SystemCostIndex
		facTax = settings.FacilityTaxPercent
	}
	return domain.EvaluateMakeVsBuy(name, qty, jitaUnitPrice, rawMatPerUnit, sci, facTax), nil
}
