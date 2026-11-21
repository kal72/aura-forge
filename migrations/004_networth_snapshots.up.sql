-- 004_networth_snapshots.up.sql
-- Daily Net Worth snapshots for tracking asset growth from 0 toward 100B+ ISK.

CREATE TABLE IF NOT EXISTS networth_snapshots (
    id BIGSERIAL PRIMARY KEY,
    snapshot_date DATE NOT NULL UNIQUE,
    wallet_isk NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    sell_orders_isk NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    escrow_isk NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    inventory_isk NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    active_jobs_isk NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    total_net_worth NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_networth_snapshots_date ON networth_snapshots (snapshot_date DESC);
