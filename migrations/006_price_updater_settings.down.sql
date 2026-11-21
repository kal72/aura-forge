-- 006_price_updater_settings.down.sql

ALTER TABLE user_settings
    DROP COLUMN IF EXISTS auto_price_sync_interval_minutes,
    DROP COLUMN IF EXISTS auto_price_sync_enabled;
