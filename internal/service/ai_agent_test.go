package service_test

import (
	"context"
	"strings"
	"testing"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
	"aura-forge/internal/service"
)

type mockAIProvider struct {
	enabled bool
	model   string
	calls   int
}

func (m *mockAIProvider) IsEnabled() bool { return m.enabled }
func (m *mockAIProvider) Status() domain.AIStatus {
	mdl := m.model
	if mdl == "" {
		mdl = "mock-model"
	}
	return domain.AIStatus{Enabled: m.enabled, BaseURL: "http://mock-llm/v1", Model: mdl}
}
func (m *mockAIProvider) SetModel(model string) error {
	m.model = model
	return nil
}
func (m *mockAIProvider) Ping(ctx context.Context) (string, error) { return "Connected to mock-model", nil }

func (m *mockAIProvider) ChatCompletion(ctx context.Context, req domain.AIChatRequest) (*domain.AIChatResponse, error) {
	m.calls++
	// Turn 1: call get_business_snapshot + propose_action_card
	if m.calls == 1 {
		return &domain.AIChatResponse{
			ToolCalls: []domain.AIToolCall{
				{
					ID:   "call_snap",
					Type: "function",
					Function: domain.AIToolCallFunction{
						Name:      "get_business_snapshot",
						Arguments: `{}`,
					},
				},
				{
					ID:   "call_card",
					Type: "function",
					Function: domain.AIToolCallFunction{
						Name:      "propose_action_card",
						Arguments: `{"kind":"add_catalog","title":"Add Hobgoblin I","description":"Fast cashflow T1 drone","button_label":"+ Add to Catalog","item_name":"Hobgoblin I","type_id":2454,"category":"Drones","output_qty":50,"unit_sell_price":14500,"material_cost":500000,"daily_volume":2500}`,
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}
	// Turn 2: final synthesized answer
	return &domain.AIChatResponse{
		Content:      "Based on your live business snapshot, start with Hobgoblin I to keep your manufacturing slots busy.",
		FinishReason: "stop",
	}, nil
}

func TestAskAuraCopilot_Level4ToolCallingAndActionCards(t *testing.T) {
	sso := esi.NewSSOClient("", "", "http://localhost:8080/auth/callback", nil)
	svc := service.NewCopilotService(
		nil, nil, nil, nil, nil,
		esi.NewPublicMarketClient(esi.NewClient("", "test", nil, nil)),
		esi.NewNoOpCharacterSyncProvider(),
		sso,
	)

	// 1. Disabled AI must fail gracefully
	if _, err := svc.AskAuraCopilot(context.Background(), service.AIChatTurnInput{Prompt: "Hi"}); err == nil {
		t.Fatalf("expected error when AI provider is not configured")
	}

	// 2. Enabled AI executes tool calls + emits Human-in-the-Loop action card
	mock := &mockAIProvider{enabled: true}
	svc.SetAIProvider(mock)

	res, err := svc.AskAuraCopilot(context.Background(), service.AIChatTurnInput{
		Prompt:      "What should I build today?",
		ContextPage: "dashboard",
	})
	if err != nil {
		t.Fatalf("AskAuraCopilot failed: %v", err)
	}
	if mock.calls != 2 {
		t.Fatalf("expected 2 LLM turns (tool call + final answer), got %d", mock.calls)
	}
	if res.LatestReply == nil || !strings.Contains(res.LatestReply.Content, "Hobgoblin I") {
		t.Fatalf("unexpected final reply: %+v", res.LatestReply)
	}
	if len(res.LatestReply.ToolTraces) != 2 {
		t.Fatalf("expected 2 tool traces, got %d", len(res.LatestReply.ToolTraces))
	}
	if len(res.LatestReply.ActionCards) != 1 || res.LatestReply.ActionCards[0].URL != "/catalog/items" {
		t.Fatalf("expected 1 add_catalog action card, got %+v", res.LatestReply.ActionCards)
	}
}
