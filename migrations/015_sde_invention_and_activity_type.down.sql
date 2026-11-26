-- 015_sde_invention_and_activity_type.down.sql

ALTER TABLE sde_metadata
    DROP COLUMN IF EXISTS inventions_count;

DROP TABLE IF EXISTS sde_invention_materials;
DROP TABLE IF EXISTS sde_inventions;

DROP INDEX IF EXISTS idx_sde_blueprints_activity;
ALTER TABLE sde_blueprints
    DROP COLUMN IF EXISTS activity_type;
