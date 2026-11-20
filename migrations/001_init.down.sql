-- 001_init.down.sql: Rollback initial schema for aura-forge

DROP INDEX IF EXISTS idx_esi_cache_expires;
DROP INDEX IF EXISTS idx_plans_status;
DROP INDEX IF EXISTS idx_batches_plan_id;
DROP INDEX IF EXISTS idx_batches_status;
DROP INDEX IF EXISTS idx_market_items_category;

DROP TABLE IF EXISTS esi_cache;
DROP TABLE IF EXISTS production_batches;
DROP TABLE IF EXISTS weekly_plans;
DROP TABLE IF EXISTS market_items;
DROP TABLE IF EXISTS character_assets;
DROP TABLE IF EXISTS character_blueprints;
DROP TABLE IF EXISTS character_jobs;
DROP TABLE IF EXISTS character_orders;
DROP TABLE IF EXISTS eve_tokens;
DROP TABLE IF EXISTS character_profiles;
DROP TABLE IF EXISTS user_settings;
