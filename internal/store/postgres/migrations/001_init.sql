-- 001_init.sql: Schema initialization for aura-forge

CREATE TABLE IF NOT EXISTS user_settings (
    id INT PRIMARY KEY DEFAULT 1,
    starting_capital NUMERIC(20,2) NOT NULL DEFAULT 100000000.00,
    broker_fee_percent NUMERIC(6,3) NOT NULL DEFAULT 3.000,
    sales_tax_percent NUMERIC(6,3) NOT NULL DEFAULT 3.600,
    relist_fee_percent NUMERIC(6,3) NOT NULL DEFAULT 0.500,
    manufacturing_slots INT NOT NULL DEFAULT 10,
    science_slots INT NOT NULL DEFAULT 1,
    system_cost_index NUMERIC(8,5) NOT NULL DEFAULT 0.04500,
    freight_cost_per_m3 NUMERIC(20,2) NOT NULL DEFAULT 300.00,
    facility_tax_percent NUMERIC(6,3) NOT NULL DEFAULT 0.250,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default settings if not exists
INSERT INTO user_settings (id, starting_capital, broker_fee_percent, sales_tax_percent, relist_fee_percent, manufacturing_slots, science_slots, system_cost_index, freight_cost_per_m3, facility_tax_percent)
VALUES (1, 100000000.00, 3.000, 3.600, 0.500, 10, 1, 0.04500, 300.00, 0.250)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS character_profiles (
    character_id BIGINT PRIMARY KEY,
    character_name VARCHAR(255) NOT NULL,
    wallet_balance NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    total_manufacturing_slots INT NOT NULL DEFAULT 10,
    used_manufacturing_slots INT NOT NULL DEFAULT 0,
    total_science_slots INT NOT NULL DEFAULT 1,
    used_science_slots INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS eve_tokens (
    character_id BIGINT PRIMARY KEY,
    character_name VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    scope TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS character_orders (
    order_id BIGINT PRIMARY KEY,
    character_id BIGINT NOT NULL,
    type_id INT NOT NULL,
    type_name VARCHAR(255) NOT NULL,
    price NUMERIC(20,2) NOT NULL,
    volume_remain INT NOT NULL,
    volume_total INT NOT NULL,
    is_buy_order BOOLEAN NOT NULL DEFAULT FALSE,
    region_id INT NOT NULL,
    location_id BIGINT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    is_undercut BOOLEAN NOT NULL DEFAULT FALSE,
    competitor_lowest_price NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    safe_relist_price NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS character_jobs (
    job_id BIGINT PRIMARY KEY,
    character_id BIGINT NOT NULL,
    type_id INT NOT NULL,
    type_name VARCHAR(255) NOT NULL,
    activity_id INT NOT NULL,
    activity_name VARCHAR(100) NOT NULL,
    runs INT NOT NULL,
    status VARCHAR(50) NOT NULL,
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ NOT NULL,
    completed_date TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS character_blueprints (
    item_id BIGINT PRIMARY KEY,
    character_id BIGINT NOT NULL,
    type_id INT NOT NULL,
    type_name VARCHAR(255) NOT NULL,
    location_id BIGINT NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    material_efficiency INT NOT NULL DEFAULT 0,
    time_efficiency INT NOT NULL DEFAULT 0,
    runs INT NOT NULL DEFAULT -1,
    is_copy BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS character_assets (
    item_id BIGINT PRIMARY KEY,
    character_id BIGINT NOT NULL,
    type_id INT NOT NULL,
    type_name VARCHAR(255) NOT NULL,
    location_id BIGINT NOT NULL,
    quantity BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS market_items (
    id BIGSERIAL PRIMARY KEY,
    type_id INT NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL DEFAULT 'Other',
    estimated_jita_sell NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    estimated_jita_buy NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    daily_volume NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    volume_30d NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    material_cost NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    batch_output_qty INT NOT NULL DEFAULT 1,
    production_time_seconds INT NOT NULL DEFAULT 0,
    is_manufacturable BOOLEAN NOT NULL DEFAULT TRUE,
    requires_rare_bpc BOOLEAN NOT NULL DEFAULT FALSE,
    extra_setup_cost NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    is_frequently_destroyed BOOLEAN NOT NULL DEFAULT FALSE,
    trap_analysis JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS weekly_plans (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    week_number INT NOT NULL DEFAULT 1,
    starting_cash NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    manufacturing_slots_total INT NOT NULL DEFAULT 10,
    manufacturing_slots_used INT NOT NULL DEFAULT 0,
    science_slots_total INT NOT NULL DEFAULT 1,
    science_slots_used INT NOT NULL DEFAULT 0,
    days JSONB NOT NULL DEFAULT '[]'::jsonb,
    items JSONB NOT NULL DEFAULT '[]'::jsonb,
    total_estimated_cost NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    total_estimated_revenue NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    total_estimated_profit NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    projected_roi NUMERIC(8,4) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS production_batches (
    id BIGSERIAL PRIMARY KEY,
    plan_id BIGINT REFERENCES weekly_plans(id) ON DELETE SET NULL,
    type_id INT NOT NULL,
    item_name VARCHAR(255) NOT NULL,
    runs INT NOT NULL DEFAULT 1,
    units_output INT NOT NULL DEFAULT 1,
    total_material_cost NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    actual_production_cost NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    actual_sell_price NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    total_revenue NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    net_profit NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    roi NUMERIC(8,4) NOT NULL DEFAULT 0.00,
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT',
    market_order_id BIGINT,
    current_market_price NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    is_undercut BOOLEAN NOT NULL DEFAULT FALSE,
    safe_relist_price NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    remaining_safe_margin NUMERIC(20,2) NOT NULL DEFAULT 0.00,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    listed_at TIMESTAMPTZ,
    sold_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS esi_cache (
    cache_key VARCHAR(512) PRIMARY KEY,
    data BYTEA NOT NULL,
    etag VARCHAR(255) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_market_items_category ON market_items(category);
CREATE INDEX IF NOT EXISTS idx_batches_status ON production_batches(status);
CREATE INDEX IF NOT EXISTS idx_batches_plan_id ON production_batches(plan_id);
CREATE INDEX IF NOT EXISTS idx_plans_status ON weekly_plans(status);
CREATE INDEX IF NOT EXISTS idx_esi_cache_expires ON esi_cache(expires_at);
