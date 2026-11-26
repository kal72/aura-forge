CREATE TABLE IF NOT EXISTS market_transactions (
    id              BIGSERIAL PRIMARY KEY,
    character_id    BIGINT NOT NULL DEFAULT 0,
    external_tx_id  BIGINT,
    dedup_hash      VARCHAR(64) NOT NULL UNIQUE,
    transacted_at   TIMESTAMPTZ NOT NULL,
    side            VARCHAR(8) NOT NULL,
    type_id         INTEGER NOT NULL DEFAULT 0,
    item_name       VARCHAR(255) NOT NULL,
    quantity        INTEGER NOT NULL,
    unit_price      NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    total_isk       NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    estimated_fee   NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    location_id     BIGINT NOT NULL DEFAULT 0,
    hub_name        VARCHAR(120) NOT NULL DEFAULT '',
    source          VARCHAR(16) NOT NULL DEFAULT 'MANUAL',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_market_tx_transacted_at ON market_transactions (transacted_at DESC);
CREATE INDEX IF NOT EXISTS idx_market_tx_side ON market_transactions (side);
CREATE INDEX IF NOT EXISTS idx_market_tx_type_id ON market_transactions (type_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_market_tx_external_id ON market_transactions (external_tx_id) WHERE external_tx_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS wallet_fee_entries (
    id              BIGSERIAL PRIMARY KEY,
    character_id    BIGINT NOT NULL DEFAULT 0,
    journal_id      BIGINT NOT NULL UNIQUE,
    recorded_at     TIMESTAMPTZ NOT NULL,
    ref_type        VARCHAR(64) NOT NULL,
    amount_isk      NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    description     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wallet_fees_recorded_at ON wallet_fee_entries (recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_wallet_fees_ref_type ON wallet_fee_entries (ref_type);

CREATE TABLE IF NOT EXISTS character_order_history (
    order_id        BIGINT PRIMARY KEY,
    character_id    BIGINT NOT NULL DEFAULT 0,
    type_id         INTEGER NOT NULL DEFAULT 0,
    type_name       VARCHAR(255) NOT NULL DEFAULT '',
    is_buy_order    BOOLEAN NOT NULL DEFAULT FALSE,
    price           NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    volume_total    INTEGER NOT NULL DEFAULT 0,
    volume_remain   INTEGER NOT NULL DEFAULT 0,
    state           VARCHAR(32) NOT NULL DEFAULT 'expired',
    region_id       INTEGER NOT NULL DEFAULT 0,
    location_id     BIGINT NOT NULL DEFAULT 0,
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_days   INTEGER NOT NULL DEFAULT 90,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_char_order_hist_char_issued ON character_order_history (character_id, issued_at DESC);
