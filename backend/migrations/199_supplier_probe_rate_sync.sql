-- Supplier resource probe is also the source of its live base multiplier.
-- Existing approved resources used the old observe-only default, so align
-- both account switches with the resource-level probe switch.
UPDATE accounts AS a
SET extra = COALESCE(a.extra, '{}'::jsonb)
    || jsonb_build_object(
        'upstream_billing_probe_enabled', r.probe_enabled,
        'upstream_billing_rate_sync_enabled', r.probe_enabled
    )
FROM supplier_resource_requests AS r
WHERE r.account_id = a.id
  AND r.status = 'approved'
  AND a.supplier_id = r.supplier_id
  AND a.deleted_at IS NULL;
