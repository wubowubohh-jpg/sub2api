-- Supplier onboarding, ownership, marketplace metrics and settlement.
CREATE TABLE IF NOT EXISTS suppliers (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    relay_url VARCHAR(500) NOT NULL,
    application_note TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','frozen')),
    reviewed_by BIGINT NULL REFERENCES users(id),
    review_note TEXT NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ NULL,
    freeze_reason TEXT NOT NULL DEFAULT '',
    pending_balance_cny DECIMAL(20,8) NOT NULL DEFAULT 0 CHECK (pending_balance_cny >= 0),
    available_balance_cny DECIMAL(20,8) NOT NULL DEFAULT 0 CHECK (available_balance_cny >= 0),
    frozen_balance_cny DECIMAL(20,8) NOT NULL DEFAULT 0 CHECK (frozen_balance_cny >= 0),
    payout_profile JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_suppliers_user_id ON suppliers(user_id);
CREATE INDEX IF NOT EXISTS idx_suppliers_status ON suppliers(status);

ALTER TABLE groups ADD COLUMN IF NOT EXISTS supplier_id BIGINT NULL REFERENCES suppliers(id);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS supplier_admin_adjustment DECIMAL(10,4) NULL;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS supplier_forced_offline BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_groups_supplier_status ON groups(supplier_id, status, supplier_forced_offline);

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS supplier_id BIGINT NULL REFERENCES suppliers(id);
CREATE INDEX IF NOT EXISTS idx_accounts_supplier_status ON accounts(supplier_id, status, schedulable);

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS supplier_id BIGINT NULL REFERENCES suppliers(id);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS supplier_base_rate DECIMAL(10,4) NULL;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS supplier_admin_adjustment DECIMAL(10,4) NULL;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS supplier_model_cost_usd DECIMAL(20,10) NULL;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS supplier_recharge_ratio DECIMAL(20,10) NULL;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS supplier_earning_cny DECIMAL(20,10) NULL;
CREATE INDEX IF NOT EXISTS idx_usage_logs_supplier_created ON usage_logs(supplier_id, created_at) WHERE supplier_id IS NOT NULL;

ALTER TABLE channel_monitors ADD COLUMN IF NOT EXISTS group_id BIGINT NULL REFERENCES groups(id);
CREATE INDEX IF NOT EXISTS idx_channel_monitors_group_id ON channel_monitors(group_id);
ALTER TABLE channel_monitor_histories ADD COLUMN IF NOT EXISTS first_token_ms INTEGER NULL;
ALTER TABLE channel_monitor_daily_rollups ADD COLUMN IF NOT EXISTS sum_first_token_ms BIGINT NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_daily_rollups ADD COLUMN IF NOT EXISTS count_first_token INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS supplier_documents (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
    storage_key VARCHAR(500) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(50) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_supplier_documents_supplier_created ON supplier_documents(supplier_id, created_at);

CREATE TABLE IF NOT EXISTS supplier_ledgers (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    usage_log_id BIGINT NULL REFERENCES usage_logs(id),
    reversal_of_id BIGINT NULL REFERENCES supplier_ledgers(id),
    event_key VARCHAR(128) NOT NULL,
    entry_type VARCHAR(24) NOT NULL CHECK (entry_type IN ('earning','reversal','release','withdrawal','withdrawal_refund')),
    bucket VARCHAR(16) NOT NULL CHECK (bucket IN ('pending','available','frozen')),
    base_rate DECIMAL(10,4) NOT NULL,
    admin_adjustment DECIMAL(10,4) NOT NULL,
    effective_rate DECIMAL(10,4) NOT NULL,
    model_cost_usd DECIMAL(20,10) NOT NULL,
    recharge_ratio DECIMAL(20,10) NOT NULL,
    earning_usd DECIMAL(20,10) NOT NULL,
    amount_cny DECIMAL(20,10) NOT NULL,
    available_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_supplier_ledgers_event_key ON supplier_ledgers(event_key);
CREATE INDEX IF NOT EXISTS idx_supplier_ledgers_release ON supplier_ledgers(supplier_id, bucket, available_at);
CREATE INDEX IF NOT EXISTS idx_supplier_ledgers_usage_log ON supplier_ledgers(usage_log_id);

CREATE TABLE IF NOT EXISTS supplier_withdrawals (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id),
    request_no VARCHAR(64) NOT NULL,
    amount_cny DECIMAL(20,8) NOT NULL CHECK (amount_cny > 0),
    method VARCHAR(16) NOT NULL CHECK (method IN ('alipay','wechat','bank')),
    payout_snapshot JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','paid')),
    reviewed_by BIGINT NULL REFERENCES users(id),
    review_note TEXT NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ NULL,
    payment_proof_key VARCHAR(500) NULL,
    paid_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_supplier_withdrawals_request_no ON supplier_withdrawals(request_no);
CREATE INDEX IF NOT EXISTS idx_supplier_withdrawals_supplier_status ON supplier_withdrawals(supplier_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_supplier_withdrawals_status ON supplier_withdrawals(status, created_at);

CREATE TABLE IF NOT EXISTS supplier_metric_buckets (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    bucket_start TIMESTAMPTZ NOT NULL,
    resolution VARCHAR(4) NOT NULL CHECK (resolution IN ('5m','1h','1d')),
    request_count BIGINT NOT NULL DEFAULT 0,
    success_count BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    duration_ms_sum BIGINT NOT NULL DEFAULT 0,
    first_token_ms_sum BIGINT NOT NULL DEFAULT 0,
    first_token_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_supplier_metric_bucket_unique ON supplier_metric_buckets(group_id, resolution, bucket_start);
CREATE INDEX IF NOT EXISTS idx_supplier_metric_bucket_window ON supplier_metric_buckets(resolution, bucket_start);

INSERT INTO settings (key, value, updated_at)
VALUES
    ('supplier_global_rate_adjustment', '0', NOW(), NOW()),
    ('supplier_min_withdrawal_usd', '100', NOW(), NOW())
ON CONFLICT (key) DO NOTHING;
