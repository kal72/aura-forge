-- 017_corp_blueprints.down.sql
DROP INDEX IF EXISTS idx_character_blueprints_char_corp;
ALTER TABLE character_blueprints
    DROP COLUMN IF EXISTS location_flag,
    DROP COLUMN IF EXISTS is_corp;
