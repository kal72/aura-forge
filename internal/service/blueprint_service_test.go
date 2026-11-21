package service_test

import (
	"context"
	"testing"

	"aura-forge/internal/domain"
	"aura-forge/internal/service"
)

type stubSDERepo struct {
	bps  map[int32]domain.SDEBlueprint
	mats map[int32][]domain.SDEMaterialRequirement
}

func (s *stubSDERepo) SearchBlueprints(_ context.Context, _ string, _ string, _ int) ([]domain.SDEBlueprint, error) {
	out := make([]domain.SDEBlueprint, 0, len(s.bps))
	for _, bp := range s.bps {
		out = append(out, bp)
	}
	return out, nil
}

func (s *stubSDERepo) GetBlueprintByID(_ context.Context, id int32) (*domain.SDEBlueprint, error) {
	bp, ok := s.bps[id]
	if !ok {
		return nil, nil
	}
	return &bp, nil
}

func (s *stubSDERepo) GetBlueprintMaterials(_ context.Context, id int32) ([]domain.SDEMaterialRequirement, error) {
	return s.mats[id], nil
}

func (s *stubSDERepo) UpsertSDEData(_ context.Context, _ []domain.SDEType, _ []domain.SDEBlueprint, _ map[int32][]domain.SDEMaterialRequirement) error {
	return nil
}

type stubMarketProvider struct {
	prices map[int32]domain.JitaPrice
}

func (m *stubMarketProvider) ResolveTypeIDs(_ context.Context, _ []string) (map[string]int32, error) {
	return map[string]int32{}, nil
}
func (m *stubMarketProvider) GetJitaPrices(_ context.Context, _ []int32) (map[int32]domain.JitaPrice, error) {
	return m.prices, nil
}
func (m *stubMarketProvider) GetDailyVolume(_ context.Context, _ int32) (float64, float64, error) {
	return 1000, 1000, nil
}
func (m *stubMarketProvider) GetSystemCostIndex(_ context.Context, _ int32) (map[string]float64, error) {
	return map[string]float64{"manufacturing": 0.045}, nil
}

func TestCopilotService_CalculateBlueprintProfit(t *testing.T) {
	ctx := context.Background()
	sde := &stubSDERepo{
		bps: map[int32]domain.SDEBlueprint{
			786: {
				BlueprintTypeID:  786,
				Name:             "Rifter Blueprint",
				ProductTypeID:    587,
				ProductName:      "Rifter",
				ProductQtyPerRun: 1,
				BaseTimeSeconds:  4800,
				TechLevel:        1,
				Category:         "Ships",
			},
		},
		mats: map[int32][]domain.SDEMaterialRequirement{
			786: {
				{MaterialTypeID: 34, MaterialName: "Tritanium", BaseQuantity: 32000, VolumeM3: 0.01},
				{MaterialTypeID: 35, MaterialName: "Pyerite", BaseQuantity: 6000, VolumeM3: 0.01},
			},
		},
	}
	market := &stubMarketProvider{
		prices: map[int32]domain.JitaPrice{
			34:  {TypeID: 34, LowestSell: 5.0, HighestBuy: 4.5},
			35:  {TypeID: 35, LowestSell: 10.0, HighestBuy: 9.0},
			587: {TypeID: 587, LowestSell: 650000.0, HighestBuy: 550000.0},
		},
	}

	svc := service.NewCopilotService(nil, nil, nil, nil, nil, market, nil, nil)
	svc.SetSDERepository(sde)

	res, err := svc.CalculateBlueprintProfit(ctx, domain.BlueprintCalcInput{
		BlueprintTypeID:     786,
		Runs:                5,
		ME:                  10,
		TE:                  20,
		StructureKey:        "raitaru_t1",
		MaterialPriceSource: "jita_sell",
	})
	if err != nil {
		t.Fatalf("CalculateBlueprintProfit failed: %v", err)
	}
	if !res.LivePricesUsed || res.TotalMaterialCost <= 0 || res.Profit.GrossRevenue != 3250000 {
		t.Fatalf("unexpected blueprint calc result: %+v", res)
	}
	if !res.Profit.IsProfitable || res.ISKPerHour <= 0 {
		t.Fatalf("expected profitable Rifter build, got profit=%.2f isk/hr=%.2f", res.Profit.NetProfitPerBatch, res.ISKPerHour)
	}
}
