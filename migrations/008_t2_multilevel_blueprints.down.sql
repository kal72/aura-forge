-- 008_t2_multilevel_blueprints.down.sql

DELETE FROM sde_blueprints WHERE blueprint_type_id IN (
    11485, 11535, 11537, 11540, 11545,
    2457, 2489, 2186,
    2049, 5976, 3245, 3842, 10859
);

DROP INDEX IF EXISTS idx_sde_blueprints_product_id;
