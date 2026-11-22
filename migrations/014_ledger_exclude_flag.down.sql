DROP INDEX IF EXISTS idx_market_tx_excluded;

ALTER TABLE market_transactions
    DROP COLUMN IF EXISTS is_excluded;
