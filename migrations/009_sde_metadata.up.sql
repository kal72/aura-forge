-- 009_sde_metadata.up.sql: Tracks installed Official CCP SDE build number and counts

CREATE TABLE IF NOT EXISTS sde_metadata (
    id INT PRIMARY KEY DEFAULT 1,
    build_number BIGINT NOT NULL DEFAULT 0,
    release_date TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'starter',
    types_count INT NOT NULL DEFAULT 0,
    blueprints_count INT NOT NULL DEFAULT 0,
    materials_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
