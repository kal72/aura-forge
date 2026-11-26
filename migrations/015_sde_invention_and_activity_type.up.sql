-- 015_sde_invention_and_activity_type.up.sql
-- Expand official CCP SDE storage with:
--   1. activity_type ('manufacturing' vs 'reaction') on sde_blueprints
--   2. sde_inventions & sde_invention_materials (T1 -> T2 invention probability, runs, datacores)
--   3. inventions_count tracking on sde_metadata

ALTER TABLE sde_blueprints
    ADD COLUMN IF NOT EXISTS activity_type VARCHAR(24) NOT NULL DEFAULT 'manufacturing';

UPDATE sde_blueprints
SET activity_type = 'reaction'
WHERE LOWER(category) = 'reactions';

CREATE INDEX IF NOT EXISTS idx_sde_blueprints_activity ON sde_blueprints (activity_type);

CREATE TABLE IF NOT EXISTS sde_inventions (
    t1_blueprint_type_id INT NOT NULL,
    t2_blueprint_type_id INT NOT NULL,
    t2_product_type_id INT NOT NULL,
    t2_product_name VARCHAR(255) NOT NULL,
    base_probability NUMERIC(10,4) NOT NULL DEFAULT 0.26,
    base_runs INT NOT NULL DEFAULT 1,
    base_time_seconds INT NOT NULL DEFAULT 3600,
    category VARCHAR(80) NOT NULL DEFAULT 'Other',
    PRIMARY KEY (t1_blueprint_type_id, t2_blueprint_type_id)
);

CREATE INDEX IF NOT EXISTS idx_sde_inventions_t2_product ON sde_inventions (LOWER(t2_product_name));
CREATE INDEX IF NOT EXISTS idx_sde_inventions_t2_bp ON sde_inventions (t2_blueprint_type_id);

CREATE TABLE IF NOT EXISTS sde_invention_materials (
    t1_blueprint_type_id INT NOT NULL,
    material_type_id INT NOT NULL REFERENCES sde_types(type_id) ON DELETE CASCADE,
    quantity INT NOT NULL,
    PRIMARY KEY (t1_blueprint_type_id, material_type_id)
);

CREATE INDEX IF NOT EXISTS idx_sde_inv_materials_bp ON sde_invention_materials (t1_blueprint_type_id);

ALTER TABLE sde_metadata
    ADD COLUMN IF NOT EXISTS inventions_count INT NOT NULL DEFAULT 0;
