ALTER TABLE supplier_resource_requests
    ADD COLUMN IF NOT EXISTS supported_models JSONB NOT NULL DEFAULT '["gpt-5.5"]'::jsonb,
    ADD COLUMN IF NOT EXISTS probe_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS rate_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK (rate_multiplier >= 0);

UPDATE supplier_resource_requests
SET supported_models = jsonb_build_array(model)
WHERE supported_models IS NULL
   OR jsonb_typeof(supported_models) <> 'array'
   OR jsonb_array_length(supported_models) = 0;

-- Supplier resource accounts should participate in upstream billing discovery
-- by default. Rate synchronization remains opt-in; the supplier sees the
-- declared multiplier without silently changing account billing.
UPDATE accounts AS a
SET extra = COALESCE(a.extra, '{}'::jsonb)
    || jsonb_build_object(
        'upstream_billing_probe_enabled', true,
        'upstream_billing_rate_sync_enabled', false
    )
FROM supplier_resource_requests AS r
WHERE r.account_id = a.id
  AND a.supplier_id = r.supplier_id
  AND r.status = 'approved'
  AND a.type IN ('apikey', 'api_key');
