-- BALANCE_RECHARGE_MULTIPLIER means paid CNY * multiplier = credited USD.
-- Supplier settlement therefore converts USD back to CNY by division. Earlier
-- supplier entries used multiplication, so correct their immutable snapshots
-- once and rebuild the materialized supplier balance buckets.

UPDATE supplier_ledgers
SET amount_cny = earning_usd / recharge_ratio
WHERE recharge_ratio > 0;

UPDATE usage_logs
SET supplier_earning_cny = (supplier_model_cost_usd * supplier_base_rate) / supplier_recharge_ratio
WHERE supplier_id IS NOT NULL
  AND supplier_model_cost_usd IS NOT NULL
  AND supplier_base_rate IS NOT NULL
  AND supplier_recharge_ratio > 0;

WITH ledger_balances AS (
    SELECT
        supplier_id,
        COALESCE(SUM(amount_cny) FILTER (WHERE bucket = 'pending'), 0) AS pending_cny,
        COALESCE(SUM(amount_cny) FILTER (WHERE bucket = 'available'), 0) AS released_cny
    FROM supplier_ledgers
    GROUP BY supplier_id
), withdrawal_balances AS (
    SELECT
        supplier_id,
        COALESCE(SUM(amount_cny) FILTER (WHERE status IN ('pending', 'approved')), 0) AS frozen_cny,
        COALESCE(SUM(amount_cny) FILTER (WHERE status IN ('pending', 'approved', 'paid')), 0) AS deducted_cny
    FROM supplier_withdrawals
    GROUP BY supplier_id
)
UPDATE suppliers AS s
-- Keep the balance constraints as an integrity guard. A negative rebuild must
-- stop the migration for investigation instead of silently erasing debt.
SET pending_balance_cny = COALESCE(l.pending_cny, 0),
    available_balance_cny = COALESCE(l.released_cny, 0) - COALESCE(w.deducted_cny, 0),
    frozen_balance_cny = COALESCE(w.frozen_cny, 0),
    updated_at = NOW()
FROM (SELECT id FROM suppliers) AS scope
LEFT JOIN ledger_balances AS l ON l.supplier_id = scope.id
LEFT JOIN withdrawal_balances AS w ON w.supplier_id = scope.id
WHERE s.id = scope.id;
