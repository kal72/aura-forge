package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"aura-forge/internal/ai"
	"aura-forge/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestOpenAIClient_DisabledWhenNoKey(t *testing.T) {
	c := ai.NewOpenAIClient("https://api.openai.com/v1", "", "gpt-4.1-mini", 5*time.Second)
	if c.IsEnabled() {
		t.Fatalf("expected IsEnabled()=false when apiKey is empty")
	}
	if _, err := c.Ping(context.Background()); err == nil {
		t.Fatalf("expected error on Ping when disabled")
	}
}

func TestOpenAIClient_ChatCompletionAndToolCalls(t *testing.T) {
	mockTransport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"invalid api key"}}`)),
			}, nil
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		var payload string
		if _, hasTools := req["tools"]; hasTools {
			payload = `{
				"model": "mock-gpt-4",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "",
						"tool_calls": [{
							"id": "call_1",
							"type": "function",
							"function": {
								"name": "search_marketplace",
								"arguments": "{\"item\":\"Hobgoblin I\",\"hub\":\"amarr\"}"
							}
						}]
					},
					"finish_reason": "tool_calls"
				}],
				"usage": {"prompt_tokens": 120, "completion_tokens": 25}
			}`
		} else {
			payload = `{
				"model": "mock-gpt-4",
				"choices": [{
					"message": {"role": "assistant", "content": "OK"},
					"finish_reason": "stop"
				}]
			}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(payload)),
		}, nil
	})

	client := ai.NewOpenAIClient("https://mock.local/v1", "test-secret", "mock-gpt-4", 5*time.Second)
	client.SetHTTPClient(&http.Client{Transport: mockTransport})

	if !client.IsEnabled() {
		t.Fatalf("expected IsEnabled()=true")
	}

	// 1. Ping test
	msg, err := client.Ping(context.Background())
	if err != nil || !strings.Contains(msg, "mock-gpt-4") {
		t.Fatalf("expected successful ping, got msg=%q err=%v", msg, err)
	}

	// 2. Tool call parsing test
	resp, err := client.ChatCompletion(context.Background(), domain.AIChatRequest{
		Messages: []domain.AIChatMessage{{Role: "user", Content: "Check Hobgoblin I in Amarr"}},
		Tools: []domain.AIToolDefinition{{
			Type: "function",
			Function: domain.AIToolFunction{
				Name:        "search_marketplace",
				Description: "Search EVE marketplace",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Function.Name != "search_marketplace" {
		t.Fatalf("expected 1 tool call for search_marketplace, got %+v", resp.ToolCalls)
	}
}
