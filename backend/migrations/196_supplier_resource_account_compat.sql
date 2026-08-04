-- Normalize supplier resource accounts created by the initial marketplace
-- implementation. New approvals already use this representation; this
-- backfill keeps existing approved resources testable after deployment.
UPDATE accounts AS a
SET type = 'apikey',
    credentials = CASE
        WHEN COALESCE(NULLIF(a.credentials->>'base_url', ''), NULLIF(a.extra->>'base_url', '')) IS NULL
            THEN COALESCE(a.credentials, '{}'::jsonb)
        ELSE jsonb_set(
            COALESCE(a.credentials, '{}'::jsonb),
            '{base_url}',
            to_jsonb(COALESCE(NULLIF(a.credentials->>'base_url', ''), NULLIF(a.extra->>'base_url', ''))),
            true
        )
    END,
    extra = (COALESCE(a.extra, '{}'::jsonb) - 'base_url') || jsonb_build_object('openai_responses_supported', false)
FROM supplier_resource_requests AS r
WHERE r.account_id = a.id
  AND a.supplier_id = r.supplier_id
  AND a.type = 'api_key';
