-- 017_corp_blueprints.up.sql: Support Corporation Blueprints alongside personal character blueprints.
ALTER TABLE character_blueprints
    ADD COLUMN IF NOT EXISTS is_corp BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS location_flag TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_character_blueprints_char_corp ON character_blueprints (character_id, is_corp);
