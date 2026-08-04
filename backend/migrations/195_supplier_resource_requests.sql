CREATE TABLE IF NOT EXISTS supplier_resource_requests (
    id BIGSERIAL PRIMARY KEY, supplier_id BIGINT NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
    group_name VARCHAR(100) NOT NULL, relay_name VARCHAR(100) NOT NULL, relay_url VARCHAR(500) NOT NULL,
    api_key_encrypted TEXT NOT NULL, model VARCHAR(200) NOT NULL DEFAULT 'gpt-5.5',
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    reviewed_by BIGINT NULL REFERENCES users(id), review_note TEXT NOT NULL DEFAULT '', group_id BIGINT NULL REFERENCES groups(id), account_id BIGINT NULL REFERENCES accounts(id), monitor_id BIGINT NULL REFERENCES channel_monitors(id), reviewed_at TIMESTAMPTZ NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_supplier_resource_requests_status ON supplier_resource_requests(supplier_id, status, created_at);
