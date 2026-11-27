package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"aura-forge/internal/domain"
)

// OpenAIClient implements domain.AIProvider against any OpenAI-compatible /chat/completions endpoint
// (OpenAI, Gemini OpenAI-compat, DeepSeek, Groq, OpenRouter, LiteLLM, vLLM, or local Ollama).
type OpenAIClient struct {
	mu         sync.RWMutex
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
	if c == nil {
		return false
	}
	c.mu.RLock()
	model := c.model
	c.mu.RUnlock()
	return c.apiKey != "" && c.baseURL != "" && model != ""
}

// SetModel overrides the active model name in memory (not persisted to DB; resets on server restart).
func (c *OpenAIClient) SetModel(model string) error {
	if c == nil {
		return fmt.Errorf("AI client is not initialized")
	}
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return fmt.Errorf("AI model name cannot be empty")
	}
	if len(trimmed) > 128 {
		return fmt.Errorf("AI model name is too long (max 128 characters)")
	}
	c.mu.Lock()
	c.model = trimmed
	c.mu.Unlock()
	return nil
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
	c.mu.RLock()
	model := c.model
	c.mu.RUnlock()
	return domain.AIStatus{
		Enabled: true,
		BaseURL: c.baseURL,
		Model:   model,
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
			Role         string              `json:"role"`
			Content      string              `json:"content"`
			ToolCalls    []domain.AIToolCall `json:"tool_calls"`
			ExtraContent json.RawMessage     `json:"extra_content,omitempty"`
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

	c.mu.RLock()
	activeModel := c.model
	c.mu.RUnlock()

	temp := req.Temperature
	if temp <= 0 {
		temp = 0.25
	}
	body := openAIRequestBody{
		Model:       activeModel,
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

	const maxAttempts = 3
	var (
		resp    *http.Response
		rawBody []byte
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build AI HTTP request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err = c.httpClient.Do(httpReq)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("AI provider timed out (%s): upstream model may be overloaded or slow; try again or increase AI_TIMEOUT_SECONDS (%w)", c.baseURL, err)
			}
			return nil, fmt.Errorf("AI endpoint unreachable (%s): %w", c.baseURL, err)
		}

		rawBody, err = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read AI response: %w", err)
		}

		// Retry transient upstream overload errors (502/503/504) with short context-aware backoff.
		if (resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout) && attempt < maxAttempts {
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("AI provider timed out during retry (%s): %w", c.baseURL, ctx.Err())
			case <-time.After(backoff):
				continue
			}
		}
		break
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
		modelUsed = activeModel
	}

	return &domain.AIChatResponse{
		Content:          strings.TrimSpace(choice.Message.Content),
		ToolCalls:        choice.Message.ToolCalls,
		ExtraContent:     choice.Message.ExtraContent,
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
	pingTimeout := 60 * time.Second
	if c.httpClient != nil && c.httpClient.Timeout > 0 {
		pingTimeout = c.httpClient.Timeout
	}
	if pingTimeout < 15*time.Second {
		pingTimeout = 15 * time.Second
	}
	if pingTimeout > 120*time.Second {
		pingTimeout = 120 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	res, err := c.ChatCompletion(pingCtx, domain.AIChatRequest{
		Messages: []domain.AIChatMessage{
			{Role: "system", Content: "Reply with exactly: OK"},
			{Role: "user", Content: "Ping"},
		},
		MaxTokens:   128,
		Temperature: 0.1,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Connected to %s (%s)", res.ModelUsed, c.baseURL), nil
}
