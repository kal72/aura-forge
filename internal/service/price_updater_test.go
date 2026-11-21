package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"aura-forge/internal/domain"
)

type stubPriceMarket struct {
	fetchedIDs []int32
}

func (m *stubPriceMarket) ResolveTypeIDs(ctx context.Context, names []string) (map[string]int32, error) {
	return map[string]int32{}, nil
}

func (m *stubPriceMarket) GetJitaPrices(ctx context.Context, typeIDs []int32) (map[int32]domain.JitaPrice, error) {
	out := make(map[int32]domain.JitaPrice, len(typeIDs))
	for _, id := range typeIDs {
		m.fetchedIDs = append(m.fetchedIDs, id)
		out[id] = domain.JitaPrice{
			TypeID:     id,
			LowestSell: 1000.0,
			HighestBuy: 900.0,
		}
	}
	return out, nil
}

func (m *stubPriceMarket) GetDailyVolume(ctx context.Context, typeID int32) (float64, float64, error) {
	return 100, 120, nil
}

func (m *stubPriceMarket) GetSystemCostIndex(ctx context.Context, solarSystemID int32) (map[string]float64, error) {
	return map[string]float64{"manufacturing": 0.045}, nil
}

type stubPriceSDE struct {
	bps  []domain.SDEBlueprint
	mats map[int32][]domain.SDEMaterialRequirement
}

func (s *stubPriceSDE) SearchBlueprints(ctx context.Context, query, category string, limit int) ([]domain.SDEBlueprint, error) {
	return s.bps, nil
}

func (s *stubPriceSDE) GetBlueprintByID(ctx context.Context, blueprintTypeID int32) (*domain.SDEBlueprint, error) {
	return nil, nil
}

func (s *stubPriceSDE) GetBlueprintMaterials(ctx context.Context, blueprintTypeID int32) ([]domain.SDEMaterialRequirement, error) {
	return s.mats[blueprintTypeID], nil
}

func (s *stubPriceSDE) UpsertSDEData(ctx context.Context, types []domain.SDEType, blueprints []domain.SDEBlueprint, materials map[int32][]domain.SDEMaterialRequirement) error {
	return nil
}

func TestSyncScopedJitaPrices_CollectsPIAndSDEAndUpdatesStatus(t *testing.T) {
	market := &stubPriceMarket{}
	svc := NewCopilotService(nil, nil, nil, nil, nil, market, nil, nil)
	svc.SetSDERepository(&stubPriceSDE{
		bps: []domain.SDEBlueprint{
			{BlueprintTypeID: 900001, ProductTypeID: 587, ProductName: "Rifter"},
		},
		mats: map[int32][]domain.SDEMaterialRequirement{
			900001: {
				{MaterialTypeID: 34, MaterialName: "Tritanium", BaseQuantity: 10000},
				{MaterialTypeID: 35, MaterialName: "Pyerite", BaseQuantity: 5000},
			},
		},
	})

	ids := svc.CollectScopedJitaTypeIDs(context.Background())
	// 68 PI (P1-P4) + 8 Minerals (includes Tritanium 34 & Pyerite 35) + 6 Macro/Fuel + Rifter (587) = 83 unique TypeIDs
	if len(ids) != 83 {
		t.Fatalf("expected 83 scoped TypeIDs (68 PI + 14 Market Pulse non-PI + 1 Rifter), got %d", len(ids))
	}

	res, err := svc.SyncScopedJitaPrices(context.Background())
	if err != nil {
		t.Fatalf("SyncScopedJitaPrices returned error: %v", err)
	}
	if res.SkippedBusy {
		t.Fatalf("expected sync not to be skipped")
	}
	if res.WarmedCount != 83 || res.TotalTypeIDs != 83 {
		t.Fatalf("expected 83/83 warmed items, got %d/%d", res.WarmedCount, res.TotalTypeIDs)
	}

	status := svc.GetPriceSyncStatus(context.Background())
	if status.LastSyncedAt == nil {
		t.Fatalf("expected LastSyncedAt to be set")
	}
	if status.LastSyncedCount != 83 {
		t.Fatalf("expected LastSyncedCount=83, got %d", status.LastSyncedCount)
	}
	if !strings.Contains(status.FreshnessLabel(), "Jita synced just now (83)") {
		t.Fatalf("unexpected FreshnessLabel: %q", status.FreshnessLabel())
	}
}

func TestClampPriceSyncIntervalAndFreshnessLabel(t *testing.T) {
	if got := domain.ClampPriceSyncInterval(1); got != 5 {
		t.Fatalf("expected min clamp 5, got %d", got)
	}
	if got := domain.ClampPriceSyncInterval(999); got != 120 {
		t.Fatalf("expected max clamp 120, got %d", got)
	}
	if got := domain.ClampPriceSyncInterval(15); got != 15 {
		t.Fatalf("expected 15, got %d", got)
	}

	if lbl := (PriceSyncStatus{IsRunning: true}).FreshnessLabel(); lbl != "Syncing Jita..." {
		t.Fatalf("unexpected running label: %q", lbl)
	}
	if lbl := (PriceSyncStatus{Enabled: false}).FreshnessLabel(); lbl != "Auto-Sync: Manual Only" {
		t.Fatalf("unexpected manual label: %q", lbl)
	}
	tenMinAgo := time.Now().Add(-10 * time.Minute)
	if lbl := (PriceSyncStatus{LastSyncedAt: &tenMinAgo, LastSyncedCount: 80}).FreshnessLabel(); !strings.Contains(lbl, "10m ago (80)") {
		t.Fatalf("unexpected synced label: %q", lbl)
	}
}
