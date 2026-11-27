-- 016_pi_sso_sync.down.sql
DROP INDEX IF EXISTS idx_pi_colonies_character_planet;
ALTER TABLE pi_colonies
    DROP COLUMN IF EXISTS synced_from_sso,
    DROP COLUMN IF EXISTS storage_summary,
    DROP COLUMN IF EXISTS security_status,
    DROP COLUMN IF EXISTS num_pins,
    DROP COLUMN IF EXISTS upgrade_level,
    DROP COLUMN IF EXISTS planet_id,
    DROP COLUMN IF EXISTS character_id;
