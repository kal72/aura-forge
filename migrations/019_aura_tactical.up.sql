-- Aura Tactical: Saved Ship Fittings Library & Embedded AI Structured Cache
CREATE TABLE IF NOT EXISTS tactical_fittings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES app_users(id) ON DELETE CASCADE,
    name VARCHAR(160) NOT NULL,
    ship_type_id INTEGER NOT NULL,
    ship_name VARCHAR(160) NOT NULL,
    hull_class VARCHAR(64) NOT NULL DEFAULT '',
    tag VARCHAR(32) NOT NULL DEFAULT 'PvE',
    skill_profile VARCHAR(32) NOT NULL DEFAULT 'intermediate',
    eft_text TEXT NOT NULL DEFAULT '',
    fit_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    estimated_cost_isk NUMERIC(20, 2) NOT NULL DEFAULT 0,
    ehp NUMERIC(14, 2) NOT NULL DEFAULT 0,
    dps NUMERIC(12, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tactical_fittings_user_updated
    ON tactical_fittings (user_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_tactical_fittings_ship
    ON tactical_fittings (ship_type_id);

CREATE TABLE IF NOT EXISTS ai_tactical_cache (
    id BIGSERIAL PRIMARY KEY,
    feature VARCHAR(64) NOT NULL,
    context_hash VARCHAR(64) NOT NULL,
    model VARCHAR(96) NOT NULL DEFAULT '',
    payload_json JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (feature, context_hash)
);

CREATE INDEX IF NOT EXISTS idx_ai_tactical_cache_expires
    ON ai_tactical_cache (expires_at);
