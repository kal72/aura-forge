-- 006_price_updater_settings.up.sql: Background Jita price updater settings

ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS auto_price_sync_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS auto_price_sync_interval_minutes INT NOT NULL DEFAULT 15;
