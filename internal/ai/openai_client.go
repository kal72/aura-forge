package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

// OpenAIClient implements domain.AIProvider against any OpenAI-compatible /chat/completions endpoint
// (OpenAI, Gemini OpenAI-compat, DeepSeek, Groq, OpenRouter, LiteLLM, vLLM, or local Ollama).
type OpenAIClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIClient constructs a lightweight OpenAI-compatible HTTP client.
// When apiKey or baseURL is empty, IsEnabled() returns false and the app runs cleanly without AI.
func NewOpenAIClient(baseURL, apiKey, model string, timeout time.Duration) *OpenAIClient {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return &OpenAIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *OpenAIClient) IsEnabled() bool {
	return c != nil && c.apiKey != "" && c.baseURL != "" && c.model != ""
}

// SetHTTPClient overrides the underlying HTTP client (primarily for hermetic unit tests).
func (c *OpenAIClient) SetHTTPClient(hc *http.Client) {
	if c != nil && hc != nil {
		c.httpClient = hc
	}
}

func (c *OpenAIClient) Status() domain.AIStatus {
	if !c.IsEnabled() {
		return domain.AIStatus{Enabled: false}
	}
	return domain.AIStatus{
		Enabled: true,
		BaseURL: c.baseURL,
		Model:   c.model,
	}
}

type openAIRequestBody struct {
	Model       string                    `json:"model"`
	Messages    []domain.AIChatMessage    `json:"messages"`
	Tools       []domain.AIToolDefinition `json:"tools,omitempty"`
	ToolChoice  string                    `json:"tool_choice,omitempty"`
	Temperature *float64                  `json:"temperature,omitempty"`
	MaxTokens   int                       `json:"max_tokens,omitempty"`
}

type openAIResponseBody struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role      string              `json:"role"`
			Content   string              `json:"content"`
			ToolCalls []domain.AIToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// ChatCompletion executes one OpenAI-compatible /chat/completions request (supporting tool_calls).
func (c *OpenAIClient) ChatCompletion(ctx context.Context, req domain.AIChatRequest) (*domain.AIChatResponse, error) {
	if !c.IsEnabled() {
		return nil, fmt.Errorf("AI provider is not configured (set AI_API_KEY, AI_BASE_URL, and AI_MODEL in .env)")
	}

	temp := req.Temperature
	if temp <= 0 {
		temp = 0.25
	}
	body := openAIRequestBody{
		Model:       c.model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		Temperature: &temp,
		MaxTokens:   req.MaxTokens,
	}
	if len(req.Tools) > 0 {
		body.ToolChoice = "auto"
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal AI request: %w", err)
	}

	endpoint := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build AI HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("AI endpoint unreachable (%s): %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read AI response: %w", err)
	}

	var parsed openAIResponseBody
	_ = json.Unmarshal(rawBody, &parsed)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errDetail := strings.TrimSpace(string(rawBody))
		if parsed.Error != nil && parsed.Error.Message != "" {
			errDetail = parsed.Error.Message
		}
		if len(errDetail) > 260 {
			errDetail = errDetail[:260] + "..."
		}
		return nil, fmt.Errorf("AI API HTTP %d: %s", resp.StatusCode, errDetail)
	}

	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("AI API returned no choices")
	}

	choice := parsed.Choices[0]
	modelUsed := parsed.Model
	if modelUsed == "" {
		modelUsed = c.model
	}

	return &domain.AIChatResponse{
		Content:          strings.TrimSpace(choice.Message.Content),
		ToolCalls:        choice.Message.ToolCalls,
		FinishReason:     choice.FinishReason,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		ModelUsed:        modelUsed,
	}, nil
}

// Ping verifies connectivity and credentials against the configured OpenAI-compatible provider.
func (c *OpenAIClient) Ping(ctx context.Context) (string, error) {
	if !c.IsEnabled() {
		return "", fmt.Errorf("AI is disabled (AI_API_KEY not set)")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	res, err := c.ChatCompletion(pingCtx, domain.AIChatRequest{
		Messages: []domain.AIChatMessage{
			{Role: "system", Content: "Reply with exactly: OK"},
			{Role: "user", Content: "Ping"},
		},
		MaxTokens:   16,
		Temperature: 0.1,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Connected to %s (%s)", res.ModelUsed, c.baseURL), nil
}
