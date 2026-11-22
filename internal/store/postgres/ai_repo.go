package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aura-forge/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AIRepo persists Aura AI Copilot conversations, messages, and pilot memory in PostgreSQL.
type AIRepo struct {
	pool *pgxpool.Pool
}

func NewAIRepo(pool *pgxpool.Pool) *AIRepo {
	return &AIRepo{pool: pool}
}

func (r *AIRepo) CreateConversation(ctx context.Context, conv *domain.AIConversation) error {
	if conv == nil {
		return nil
	}
	if strings.TrimSpace(conv.Title) == "" {
		conv.Title = "Aura Copilot Session"
	}
	if strings.TrimSpace(conv.ContextPage) == "" {
		conv.ContextPage = "dashboard"
	}
	const q = `
		INSERT INTO ai_conversations (user_id, title, context_page, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.pool.QueryRow(ctx, q, conv.UserID, conv.Title, conv.ContextPage).
		Scan(&conv.ID, &conv.CreatedAt, &conv.UpdatedAt)
}

func (r *AIRepo) ListConversations(ctx context.Context, userID int64, limit int) ([]domain.AIConversation, error) {
	if limit <= 0 {
		limit = 20
	}
	const q = `
		SELECT id, user_id, title, context_page, created_at, updated_at
		FROM ai_conversations
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AIConversation
	for rows.Next() {
		var c domain.AIConversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.ContextPage, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *AIRepo) GetConversation(ctx context.Context, id int64, userID int64) (*domain.AIConversation, error) {
	const q = `
		SELECT id, user_id, title, context_page, created_at, updated_at
		FROM ai_conversations
		WHERE id = $1 AND user_id = $2
	`
	var c domain.AIConversation
	err := r.pool.QueryRow(ctx, q, id, userID).
		Scan(&c.ID, &c.UserID, &c.Title, &c.ContextPage, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *AIRepo) DeleteConversation(ctx context.Context, id int64, userID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM ai_conversations WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func (r *AIRepo) AppendMessage(ctx context.Context, msg *domain.AIMessage) error {
	if msg == nil {
		return nil
	}
	tracesJSON, _ := json.Marshal(msg.ToolTraces)
	if len(msg.ToolTraces) == 0 {
		tracesJSON = []byte("[]")
	}
	cardsJSON, _ := json.Marshal(msg.ActionCards)
	if len(msg.ActionCards) == 0 {
		cardsJSON = []byte("[]")
	}

	const q = `
		INSERT INTO ai_messages (conversation_id, role, content, tool_trace_json, action_cards_json, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, created_at
	`
	if err := r.pool.QueryRow(ctx, q, msg.ConversationID, msg.Role, msg.Content, tracesJSON, cardsJSON).
		Scan(&msg.ID, &msg.CreatedAt); err != nil {
		return err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE ai_conversations SET updated_at = NOW() WHERE id = $1`, msg.ConversationID)
	return nil
}

func (r *AIRepo) ListMessages(ctx context.Context, conversationID int64, limit int) ([]domain.AIMessage, error) {
	if limit <= 0 {
		limit = 40
	}
	const q = `
		SELECT id, conversation_id, role, content, tool_trace_json, action_cards_json, created_at
		FROM (
			SELECT id, conversation_id, role, content, tool_trace_json, action_cards_json, created_at
			FROM ai_messages
			WHERE conversation_id = $1
			ORDER BY id DESC
			LIMIT $2
		) sub
		ORDER BY id ASC
	`
	rows, err := r.pool.Query(ctx, q, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AIMessage
	for rows.Next() {
		var (
			m          domain.AIMessage
			tracesRaw  []byte
			cardsRaw   []byte
		)
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &tracesRaw, &cardsRaw, &m.CreatedAt); err != nil {
			return nil, err
		}
		if len(tracesRaw) > 0 {
			_ = json.Unmarshal(tracesRaw, &m.ToolTraces)
		}
		if len(cardsRaw) > 0 {
			_ = json.Unmarshal(cardsRaw, &m.ActionCards)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *AIRepo) GetPilotMemory(ctx context.Context, userID int64) (*domain.AIPilotMemory, error) {
	const q = `
		SELECT user_id, playstyle_notes, risk_tolerance, daily_playtime, updated_at
		FROM ai_pilot_memory
		WHERE user_id = $1
	`
	var mem domain.AIPilotMemory
	err := r.pool.QueryRow(ctx, q, userID).
		Scan(&mem.UserID, &mem.PlaystyleNotes, &mem.RiskTolerance, &mem.DailyPlaytime, &mem.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &domain.AIPilotMemory{
			UserID:        userID,
			RiskTolerance: "balanced",
			DailyPlaytime: "1-2 hours/day",
			UpdatedAt:     time.Now(),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &mem, nil
}

func (r *AIRepo) SavePilotMemory(ctx context.Context, mem *domain.AIPilotMemory) error {
	if mem == nil {
		return nil
	}
	if strings.TrimSpace(mem.RiskTolerance) == "" {
		mem.RiskTolerance = "balanced"
	}
	if strings.TrimSpace(mem.DailyPlaytime) == "" {
		mem.DailyPlaytime = "1-2 hours/day"
	}
	const q = `
		INSERT INTO ai_pilot_memory (user_id, playstyle_notes, risk_tolerance, daily_playtime, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			playstyle_notes = EXCLUDED.playstyle_notes,
			risk_tolerance  = EXCLUDED.risk_tolerance,
			daily_playtime  = EXCLUDED.daily_playtime,
			updated_at      = NOW()
		RETURNING updated_at
	`
	return r.pool.QueryRow(ctx, q, mem.UserID, strings.TrimSpace(mem.PlaystyleNotes), mem.RiskTolerance, strings.TrimSpace(mem.DailyPlaytime)).
		Scan(&mem.UpdatedAt)
}
