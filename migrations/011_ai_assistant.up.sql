CREATE TABLE IF NOT EXISTS ai_conversations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL DEFAULT 0,
    title VARCHAR(200) NOT NULL DEFAULT 'Aura Copilot Session',
    context_page VARCHAR(64) NOT NULL DEFAULT 'dashboard',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_conversations_user_updated
    ON ai_conversations (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS ai_messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    role VARCHAR(24) NOT NULL,
    content TEXT NOT NULL,
    tool_trace_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    action_cards_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_messages_conversation
    ON ai_messages (conversation_id, id ASC);

CREATE TABLE IF NOT EXISTS ai_pilot_memory (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE,
    playstyle_notes TEXT NOT NULL DEFAULT '',
    risk_tolerance VARCHAR(32) NOT NULL DEFAULT 'balanced',
    daily_playtime VARCHAR(64) NOT NULL DEFAULT '1-2 hours/day',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
