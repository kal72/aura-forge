ALTER TABLE user_settings
ADD COLUMN IF NOT EXISTS default_trade_hub VARCHAR(16) NOT NULL DEFAULT 'jita';
