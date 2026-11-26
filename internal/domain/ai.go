package domain

import (
	"context"
	"encoding/json"
	"time"
)

// AIStatus describes whether the optional OpenAI-compatible AI gateway is configured and active.
type AIStatus struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url,omitempty"`
	Model   string `json:"model,omitempty"`
}

// AIToolTrace records one internal tool call made by the Level 4 AI Agent during a turn.
type AIToolTrace struct {
	ToolName string `json:"tool_name"`
	ArgsJSON string `json:"args_json,omitempty"`
	Summary  string `json:"summary"`
	Error    string `json:"error,omitempty"`
}

// AIActionCard represents a Human-in-the-Loop 1-click action proposed by the AI Copilot.
// The AI never mutates business tables directly; the user explicitly clicks to execute.
type AIActionCard struct {
	Kind        string            `json:"kind"`         // "add_catalog" | "create_batch" | "open_link"
	Title       string            `json:"title"`        // e.g. "Add Hobgoblin I to Catalog"
	Description string            `json:"description"`  // concise rationale
	ButtonLabel string            `json:"button_label"` // e.g. "+ Add to Catalog"
	URL         string            `json:"url"`          // GET link or HTMX POST endpoint
	Method      string            `json:"method"`       // "GET" | "POST"
	Payload     map[string]string `json:"payload,omitempty"`
}

// AIConversation represents one persisted Aura AI Copilot chat thread.
type AIConversation struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Title       string    `json:"title"`
	ContextPage string    `json:"context_page"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AIMessage represents one user or assistant message in a persisted Aura AI conversation.
type AIMessage struct {
	ID             int64          `json:"id"`
	ConversationID int64          `json:"conversation_id"`
	Role           string         `json:"role"` // "user" | "assistant"
	Content        string         `json:"content"`
	ToolTraces     []AIToolTrace  `json:"tool_traces,omitempty"`
	ActionCards    []AIActionCard `json:"action_cards,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// AIPilotMemory stores persistent pilot preferences so Aura AI tailors advice consistently.
type AIPilotMemory struct {
	UserID         int64     `json:"user_id"`
	PlaystyleNotes string    `json:"playstyle_notes"`
	RiskTolerance  string    `json:"risk_tolerance"` // "conservative" | "balanced" | "aggressive"
	DailyPlaytime  string    `json:"daily_playtime"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// --- OpenAI Chat Completions Wire Models (Dependency Inversion) ---

// AIToolFunction describes a callable function schema sent to an OpenAI-compatible LLM.
type AIToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// AIToolDefinition wraps a function tool definition in OpenAI format.
type AIToolDefinition struct {
	Type     string         `json:"type"` // always "function"
	Function AIToolFunction `json:"function"`
}

// AIToolCallFunction holds the function name and raw JSON arguments requested by the LLM.
type AIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// AIToolCall represents one tool invocation emitted by the LLM.
type AIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // "function"
	Function AIToolCallFunction `json:"function"`
}

// AIChatMessage is one OpenAI-compatible chat message (system, user, assistant, or tool).
type AIChatMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content,omitempty"`
	Name       string       `json:"name,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	ToolCalls  []AIToolCall `json:"tool_calls,omitempty"`
}

// AIChatRequest is sent to AIProvider.ChatCompletion.
type AIChatRequest struct {
	Messages    []AIChatMessage    `json:"messages"`
	Tools       []AIToolDefinition `json:"tools,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
}

// AIChatResponse is returned by AIProvider.ChatCompletion.
type AIChatResponse struct {
	Content          string       `json:"content"`
	ToolCalls        []AIToolCall `json:"tool_calls,omitempty"`
	FinishReason     string       `json:"finish_reason,omitempty"`
	PromptTokens     int          `json:"prompt_tokens,omitempty"`
	CompletionTokens int          `json:"completion_tokens,omitempty"`
	ModelUsed        string       `json:"model_used,omitempty"`
}

// AIProvider abstracts any OpenAI-compatible LLM backend (OpenAI, Gemini, DeepSeek, Groq, Ollama, etc.).
type AIProvider interface {
	IsEnabled() bool
	Status() AIStatus
	ChatCompletion(ctx context.Context, req AIChatRequest) (*AIChatResponse, error)
	Ping(ctx context.Context) (string, error)
}

// AIRepository persists Aura AI conversations, messages, and pilot memory in PostgreSQL.
type AIRepository interface {
	CreateConversation(ctx context.Context, conv *AIConversation) error
	ListConversations(ctx context.Context, userID int64, limit int) ([]AIConversation, error)
	GetConversation(ctx context.Context, id int64, userID int64) (*AIConversation, error)
	DeleteConversation(ctx context.Context, id int64, userID int64) error
	AppendMessage(ctx context.Context, msg *AIMessage) error
	ListMessages(ctx context.Context, conversationID int64, limit int) ([]AIMessage, error)
	GetPilotMemory(ctx context.Context, userID int64) (*AIPilotMemory, error)
	SavePilotMemory(ctx context.Context, mem *AIPilotMemory) error
}
