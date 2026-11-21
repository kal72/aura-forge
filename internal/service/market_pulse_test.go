package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"aura-forge/internal/domain"
)

type stubPulseMarket struct {
	stubPriceMarket
}

func (m *stubPulseMarket) GetRegionOrderBook(ctx context.Context, hub domain.TradeHub, typeID int32, limit int) ([]domain.MarketOrderBookEntry, []domain.MarketOrderBookEntry, domain.HubPriceSummary, error) {
	return nil, nil, domain.HubPriceSummary{
		LowestSell: 900,
		HighestBuy: 810,
	}, nil
}

func (m *stubPulseMarket) GetMarketHistory(ctx context.Context, regionID int32, typeID int32, days int) ([]domain.MarketHistoryPoint, error) {
	// 7-day average = 1000.0; when sell=900 -> -10% => CHEAP
	return []domain.MarketHistoryPoint{
		{Date: "2026-11-15", Average: 980},
		{Date: "2026-11-16", Average: 1000},
		{Date: "2026-11-17", Average: 1020},
		{Date: "2026-11-18", Average: 1000},
		{Date: "2026-11-19", Average: 990},
		{Date: "2026-11-20", Average: 1010},
		{Date: "2026-11-21", Average: 1000},
	}, nil
}

func TestGetLiveMarketPulse_CategoriesSignalsAndSparkline(t *testing.T) {
	svc := NewCopilotService(nil, nil, nil, nil, nil, &stubPulseMarket{}, nil, nil)

	minerals := svc.GetLiveMarketPulse(context.Background(), "minerals", "jita")
	if len(minerals) != 8 {
		t.Fatalf("expected 8 minerals, got %d", len(minerals))
	}
	if !minerals[0].IsLive || minerals[0].SpreadPercent <= 0 {
		t.Fatalf("expected live mineral quote with positive spread, got %+v", minerals[0])
	}
	if minerals[0].SparklinePoints == "" || !strings.Contains(minerals[0].SparklinePoints, ",") {
		t.Fatalf("expected valid SVG sparkline points, got %q", minerals[0].SparklinePoints)
	}

	piQuotes := svc.GetLiveMarketPulse(context.Background(), "pi", "amarr")
	if len(piQuotes) != 6 {
		t.Fatalf("expected 6 PI benchmarks, got %d", len(piQuotes))
	}
	if piQuotes[0].HubKey != "amarr" || !piQuotes[0].IsPI {
		t.Fatalf("expected Amarr PI quote, got %+v", piQuotes[0])
	}
	// Amarr stub returns LowestSell=900 vs 7d avg=1000 => -10.0% => CHEAP
	if piQuotes[0].Signal != "CHEAP" {
		t.Fatalf("expected CHEAP signal for -10%% below 7d avg, got %s (change=%.1f%%)", piQuotes[0].Signal, piQuotes[0].Change7dPercent)
	}

	macro := svc.GetLiveMarketPulse(context.Background(), "macro", "jita")
	if len(macro) != 6 {
		t.Fatalf("expected 6 Macro & Fuel benchmarks, got %d", len(macro))
	}
}

type stubHotBPMarket struct {
	prices map[int32]domain.JitaPrice
	hist   map[int32][]domain.MarketHistoryPoint
}

func (m *stubHotBPMarket) ResolveTypeIDs(ctx context.Context, names []string) (map[string]int32, error) {
	return map[string]int32{}, nil
}
func (m *stubHotBPMarket) GetJitaPrices(ctx context.Context, typeIDs []int32) (map[int32]domain.JitaPrice, error) {
	out := make(map[int32]domain.JitaPrice, len(typeIDs))
	for _, id := range typeIDs {
		if p, ok := m.prices[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}
func (m *stubHotBPMarket) GetDailyVolume(ctx context.Context, typeID int32) (float64, float64, error) {
	return 100, 100, nil
}
func (m *stubHotBPMarket) GetSystemCostIndex(ctx context.Context, solarSystemID int32) (map[string]float64, error) {
	return map[string]float64{"manufacturing": 0.045}, nil
}
func (m *stubHotBPMarket) GetRegionOrderBook(ctx context.Context, hub domain.TradeHub, typeID int32, limit int) ([]domain.MarketOrderBookEntry, []domain.MarketOrderBookEntry, domain.HubPriceSummary, error) {
	p := m.prices[typeID]
	return nil, nil, domain.HubPriceSummary{LowestSell: p.LowestSell, HighestBuy: p.HighestBuy}, nil
}
func (m *stubHotBPMarket) GetMarketHistory(ctx context.Context, regionID int32, typeID int32, days int) ([]domain.MarketHistoryPoint, error) {
	return m.hist[typeID], nil
}
func (m *stubHotBPMarket) GetGlobalMarketPrices(ctx context.Context) ([]domain.GlobalMarketPrice, error) {
	return []domain.GlobalMarketPrice{
		{TypeID: 587, AveragePrice: 700000},
		{TypeID: 2456, AveragePrice: 15000},
	}, nil
}
func (m *stubHotBPMarket) ResolveTypeNames(ctx context.Context, typeIDs []int32) (map[int32]string, error) {
	return map[int32]string{99999: "Deadspace Shield Booster"}, nil
}

type stubSnapshotRepo struct {
	stats  *domain.GlobalSnapshotStats
	movers []domain.GlobalPriceMover
}

func (r *stubSnapshotRepo) UpsertDailySnapshot(ctx context.Context, snapshotDate time.Time, prices []domain.GlobalMarketPrice) (int, error) {
	return len(prices), nil
}
func (r *stubSnapshotRepo) ListTopPriceMovers(ctx context.Context, limit int, minPrevPrice float64) ([]domain.GlobalPriceMover, error) {
	return r.movers, nil
}
func (r *stubSnapshotRepo) GetSnapshotStats(ctx context.Context) (*domain.GlobalSnapshotStats, error) {
	return r.stats, nil
}
func (r *stubSnapshotRepo) PruneOldSnapshots(ctx context.Context, keepDays int) error {
	return nil
}

func TestGetLiveMarketPulse_HotBlueprintsSortByDemandAndPrice(t *testing.T) {
	// Item 587 (Rifter): huge volume surge (2x HOT), flat price (+1%)
	// Item 2456 (Hobgoblin I): steady volume (1x), huge price jump (+20%)
	rifterHist := make([]domain.MarketHistoryPoint, 0, 14)
	hobHist := make([]domain.MarketHistoryPoint, 0, 14)
	for i := 0; i < 14; i++ {
		rVol := int64(100)
		if i >= 7 {
			rVol = 300 // 7d avg=300 vs 14d avg=200 => 1.5x HOT
		}
		rifterHist = append(rifterHist, domain.MarketHistoryPoint{Average: 700_000, Volume: rVol})
		hobHist = append(hobHist, domain.MarketHistoryPoint{Average: 10_000, Volume: 1000})
	}

	market := &stubHotBPMarket{
		prices: map[int32]domain.JitaPrice{
			587:  {TypeID: 587, LowestSell: 707_000, HighestBuy: 650_000}, // +1%
			2456: {TypeID: 2456, LowestSell: 12_000, HighestBuy: 10_500},  // +20%
		},
		hist: map[int32][]domain.MarketHistoryPoint{
			587:  rifterHist,
			2456: hobHist,
		},
	}

	svc := NewCopilotService(nil, nil, nil, nil, nil, market, nil, nil)
	svc.SetSDERepository(&stubPriceSDE{
		bps: []domain.SDEBlueprint{
			{BlueprintTypeID: 687, ProductTypeID: 587, ProductName: "Rifter", Category: "Ships"},
			{BlueprintTypeID: 2457, ProductTypeID: 2456, ProductName: "Hobgoblin I", Category: "Drones"},
		},
	})

	byDemand := svc.GetLiveMarketPulseView(context.Background(), PulseCategoryBlueprints, "jita", "demand")
	if len(byDemand.Quotes) != 2 {
		t.Fatalf("expected 2 hot blueprints, got %d", len(byDemand.Quotes))
	}
	if byDemand.Quotes[0].Name != "Rifter" || byDemand.Quotes[0].Signal != "HOT" {
		t.Fatalf("expected Rifter (HOT) first when sorted by demand, got %+v", byDemand.Quotes[0])
	}

	byPrice := svc.GetLiveMarketPulseView(context.Background(), PulseCategoryBlueprints, "jita", "price")
	if byPrice.Quotes[0].Name != "Hobgoblin I" || byPrice.Quotes[0].Change7dPercent != 20.0 {
		t.Fatalf("expected Hobgoblin I (+20%%) first when sorted by price, got %+v", byPrice.Quotes[0])
	}
}

func TestGlobalMovers_BaselineAndDayOverDayRanking(t *testing.T) {
	market := &stubHotBPMarket{
		prices: map[int32]domain.JitaPrice{
			99999: {TypeID: 99999, LowestSell: 250_000_000, HighestBuy: 230_000_000},
		},
	}
	svc := NewCopilotService(nil, nil, nil, nil, nil, market, nil, nil)

	// Day 1: only 1 day available -> returns empty quotes + stats with DaysAvailable=1
	svc.SetGlobalPriceSnapshotRepository(&stubSnapshotRepo{
		stats: &domain.GlobalSnapshotStats{DaysAvailable: 1, LatestItems: 15234},
	})
	day1 := svc.GetLiveMarketPulseView(context.Background(), PulseCategoryGlobal, "jita", "demand")
	if len(day1.Quotes) != 0 || day1.GlobalStats == nil || day1.GlobalStats.LatestItems != 15234 {
		t.Fatalf("expected Day 1 baseline status with 0 quotes, got %+v", day1)
	}

	// Day 2+: returns resolved global movers
	svc.SetGlobalPriceSnapshotRepository(&stubSnapshotRepo{
		stats: &domain.GlobalSnapshotStats{DaysAvailable: 2, LatestItems: 15240},
		movers: []domain.GlobalPriceMover{
			{TypeID: 99999, Name: "", Category: "Market", CurrentAvg: 245_000_000, PreviousAvg: 200_000_000, ChangePercent: 22.5},
		},
	})
	day2 := svc.GetLiveMarketPulseView(context.Background(), PulseCategoryGlobal, "jita", "demand")
	if len(day2.Quotes) != 1 || day2.Quotes[0].Name != "Deadspace Shield Booster" || day2.Quotes[0].Change7dPercent != 22.5 {
		t.Fatalf("expected resolved Global Mover on Day 2, got %+v", day2.Quotes)
	}
}
