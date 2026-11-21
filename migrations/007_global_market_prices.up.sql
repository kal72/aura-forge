-- 007_global_market_prices.up.sql: Daily global EVE market price snapshots (from 1-call GET /markets/prices/)

CREATE TABLE IF NOT EXISTS global_market_price_snapshots (
    snapshot_date DATE NOT NULL,
    type_id INT NOT NULL,
    average_price NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    adjusted_price NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (snapshot_date, type_id)
);

CREATE INDEX IF NOT EXISTS idx_global_market_prices_type_date
    ON global_market_price_snapshots (type_id, snapshot_date DESC);
