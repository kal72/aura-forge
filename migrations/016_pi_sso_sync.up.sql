-- 016_pi_sso_sync.up.sql: Add EVE SSO synced metadata columns to pi_colonies.
ALTER TABLE pi_colonies
    ADD COLUMN IF NOT EXISTS character_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS planet_id INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upgrade_level INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS num_pins INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS security_status DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS storage_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS synced_from_sso BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_pi_colonies_character_planet ON pi_colonies (character_id, planet_id);
