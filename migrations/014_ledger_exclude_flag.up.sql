ALTER TABLE market_transactions
    ADD COLUMN IF NOT EXISTS is_excluded BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_market_tx_excluded
    ON market_transactions (is_excluded, transacted_at DESC);
