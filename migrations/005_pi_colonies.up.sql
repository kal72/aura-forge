-- 005_pi_colonies.up.sql
-- Planetary Industry (PI) colony tracker with extractor expiry timers.

CREATE TABLE IF NOT EXISTS pi_colonies (
    id BIGSERIAL PRIMARY KEY,
    planet_name VARCHAR(120) NOT NULL,
    solar_system VARCHAR(120) NOT NULL DEFAULT '',
    planet_type VARCHAR(32) NOT NULL DEFAULT 'Barren',
    product_type_id INT NOT NULL DEFAULT 0,
    product_name VARCHAR(120) NOT NULL,
    tier VARCHAR(8) NOT NULL DEFAULT 'P2',
    extractor_expires_at TIMESTAMPTZ NOT NULL,
    est_daily_isk NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pi_colonies_expires ON pi_colonies (extractor_expires_at ASC);
