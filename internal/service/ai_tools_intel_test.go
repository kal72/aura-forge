package service_test

import (
	"context"
	"strings"
	"testing"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
	"aura-forge/internal/service"
)

type fakeIntelMarket struct {
	bpSeeds  map[int32][]domain.NPCBPOSeedOrder
	lpOffers map[int32][]domain.LoyaltyStoreOffer
	routes   map[string][]int32
	systems  map[int32]*domain.SolarSystemInfo
	kills    map[int32]domain.SystemKillStat
	history  []domain.MarketHistoryPoint
	sell     float64
	buy      float64
}

func (f *fakeIntelMarket) ResolveTypeIDs(_ context.Context, names []string) (map[string]int32, error) {
	out := make(map[string]int32)
	for _, n := range names {
		switch n {
		case "Caldari Navy Ballistic Control System":
			out[n] = 15681
		case "Caldari Navy Ballistic Control System Blueprint":
			out[n] = 15682
		case "Hobgoblin I":
			out[n] = 2454
		case "Hobgoblin I Blueprint":
			out[n] = 2455
		}
	}
	return out, nil
}
func (f *fakeIntelMarket) GetJitaPrices(_ context.Context, _ []int32) (map[int32]domain.JitaPrice, error) {
	return map[int32]domain.JitaPrice{}, nil
}
func (f *fakeIntelMarket) GetDailyVolume(_ context.Context, _ int32) (float64, float64, error) {
	return 100, 100, nil
}
func (f *fakeIntelMarket) GetSystemCostIndex(_ context.Context, _ int32) (map[string]float64, error) {
	return map[string]float64{"manufacturing": 0.04}, nil
}
func (f *fakeIntelMarket) GetRegionOrderBook(_ context.Context, _ domain.TradeHub, _ int32, _ int) ([]domain.MarketOrderBookEntry, []domain.MarketOrderBookEntry, domain.HubPriceSummary, error) {
	return nil, nil, domain.HubPriceSummary{LowestSell: f.sell, HighestBuy: f.buy}, nil
}
func (f *fakeIntelMarket) GetMarketHistory(_ context.Context, _ int32, _ int32, _ int) ([]domain.MarketHistoryPoint, error) {
	return f.history, nil
}
func (f *fakeIntelMarket) GetGlobalMarketPrices(_ context.Context) ([]domain.GlobalMarketPrice, error) {
	return nil, nil
}
func (f *fakeIntelMarket) ResolveTypeNames(_ context.Context, ids []int32) (map[int32]string, error) {
	out := make(map[int32]string)
	for _, id := range ids {
		if id == 22291 {
			out[id] = "Ballistic Control System I"
		}
	}
	return out, nil
}
func (f *fakeIntelMarket) FindNPCBPOSeedOrders(_ context.Context, _ int32, _ string, bpID int32) ([]domain.NPCBPOSeedOrder, error) {
	return f.bpSeeds[bpID], nil
}
func (f *fakeIntelMarket) GetLoyaltyStoreOffers(_ context.Context, corpID int32, _ string) ([]domain.LoyaltyStoreOffer, error) {
	return f.lpOffers[corpID], nil
}
func (f *fakeIntelMarket) GetRoute(_ context.Context, _ int32, _ int32, flag string) ([]int32, error) {
	return f.routes[flag], nil
}
func (f *fakeIntelMarket) GetSystemInfo(_ context.Context, id int32) (*domain.SolarSystemInfo, error) {
	return f.systems[id], nil
}
func (f *fakeIntelMarket) GetSystemKills(_ context.Context) (map[int32]domain.SystemKillStat, error) {
	return f.kills, nil
}
func (f *fakeIntelMarket) ResolveSolarSystemID(_ context.Context, name string) (int32, string, error) {
	if strings.EqualFold(name, "jita") {
		return 30000142, "Jita", nil
	}
	return 30002187, "Amarr", nil
}

type scriptedAIProvider struct {
	calls     int
	toolName  string
	toolArgs  string
	seenTools []string
}

func (m *scriptedAIProvider) IsEnabled() bool { return true }
func (m *scriptedAIProvider) Status() domain.AIStatus {
	return domain.AIStatus{Enabled: true, BaseURL: "http://mock/v1", Model: "mock-intel"}
}
func (m *scriptedAIProvider) SetModel(_ string) error                { return nil }
func (m *scriptedAIProvider) Ping(_ context.Context) (string, error) { return "OK", nil }
func (m *scriptedAIProvider) ChatCompletion(_ context.Context, req domain.AIChatRequest) (*domain.AIChatResponse, error) {
	m.calls++
	if m.calls == 1 {
		return &domain.AIChatResponse{
			ToolCalls: []domain.AIToolCall{{
				ID:   "call_intel_1",
				Type: "function",
				Function: domain.AIToolCallFunction{
					Name:      m.toolName,
					Arguments: m.toolArgs,
				},
			}},
		}, nil
	}
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			m.seenTools = append(m.seenTools, msg.Content)
		}
	}
	return &domain.AIChatResponse{Content: "Intel verified."}, nil
}

func TestAuraAI_NewIntelTools(t *testing.T) {
	market := &fakeIntelMarket{
		bpSeeds: map[int32][]domain.NPCBPOSeedOrder{
			2455: {{OrderID: 1, TypeID: 2455, StationID: 60003760, StationName: "Jita IV - Moon 4 - Caldari Navy Assembly Plant", RegionName: "The Forge", Price: 120000, Duration: 365}},
		},
		lpOffers: map[int32][]domain.LoyaltyStoreOffer{
			1000035: {{
				OfferID:         99,
				CorporationID:   1000035,
				CorporationName: "Caldari Navy",
				TypeID:          15682,
				Quantity:        1,
				LPCost:          48000,
				ISKCost:         48000000,
				RequiredItems:   []domain.LoyaltyStoreRequiredItem{{TypeID: 22291, Quantity: 1}},
			}},
		},
		routes: map[string][]int32{
			"secure":   {30000142, 30002768, 30002187},
			"shortest": {30000142, 30005196, 30002187},
		},
		systems: map[int32]*domain.SolarSystemInfo{
			30000142: {SystemID: 30000142, Name: "Jita", SecurityStatus: 0.95},
			30002768: {SystemID: 30002768, Name: "Uedama", SecurityStatus: 0.50},
			30002187: {SystemID: 30002187, Name: "Amarr", SecurityStatus: 1.00},
		},
		kills: map[int32]domain.SystemKillStat{
			30002768: {SystemID: 30002768, ShipKills: 12, PodKills: 4},
		},
		history: []domain.MarketHistoryPoint{
			{Date: "2026-11-20", Average: 10000, Lowest: 9500, Highest: 10500, Volume: 500},
			{Date: "2026-11-21", Average: 10100, Lowest: 9600, Highest: 10600, Volume: 520},
			{Date: "2026-11-22", Average: 10200, Lowest: 9800, Highest: 10800, Volume: 480},
		},
		sell: 15000, // +48% spike above ~10100 avg
		buy:  9900,
	}

	sso := esi.NewSSOClient("", "", "http://localhost:8080/auth/callback", nil)

	tests := []struct {
		name         string
		toolName     string
		toolArgs     string
		expectSubstr []string
	}{
		{
			name:         "find_blueprint_sources_lp_store",
			toolName:     "find_blueprint_sources",
			toolArgs:     `{"item":"Caldari Navy Ballistic Control System"}`,
			expectSubstr: []string{"Caldari Navy", "48000", "Ballistic Control System I"},
		},
		{
			name:         "check_route_and_hauling_risk",
			toolName:     "check_route_and_hauling_risk",
			toolArgs:     `{"origin":"jita","destination":"amarr","cargo_value_isk":300000000,"ship_class":"t1_hauler"}`,
			expectSubstr: []string{"Uedama", "HIGH_RISK", "ship_kills_1h"},
		},
		{
			name:         "check_market_history_and_volatility",
			toolName:     "check_market_history_and_volatility",
			toolArgs:     `{"item":"Hobgoblin I","hub":"amarr","days":30}`,
			expectSubstr: []string{"SPIKE_WARNING", "avg_price_30d"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			aiMock := &scriptedAIProvider{toolName: tc.toolName, toolArgs: tc.toolArgs}
			svc := service.NewCopilotService(nil, nil, nil, nil, nil, market, esi.NewNoOpCharacterSyncProvider(), sso)
			svc.SetAIProvider(aiMock)

			res, err := svc.AskAuraCopilot(context.Background(), service.AIChatTurnInput{
				Prompt:      "Run intel tool",
				ContextPage: "dashboard",
			})
			if err != nil {
				t.Fatalf("AskAuraCopilot failed: %v", err)
			}
			if res.LatestReply == nil || len(aiMock.seenTools) == 0 {
				t.Fatalf("expected tool execution output")
			}
			for _, sub := range tc.expectSubstr {
				if !strings.Contains(aiMock.seenTools[0], sub) {
					t.Errorf("expected tool output to contain %q, got: %s", sub, aiMock.seenTools[0])
				}
			}
		})
	}
}
