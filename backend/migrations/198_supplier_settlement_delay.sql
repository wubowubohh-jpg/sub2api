-- Configurable supplier settlement release delay. Existing deployments keep
-- the historical seven-day behavior unless an administrator changes it.
INSERT INTO settings (key, value, updated_at)
VALUES ('supplier_settlement_delay_days', '7', NOW())
ON CONFLICT (key) DO NOTHING;

DELETE FROM settings
WHERE key IN ('supplier_min_withdrawal_usd', 'platform_supply_enabled', 'platform_supplier_name');
