-- Snapshot supplier settlement inputs in the same transaction that persists
-- the usage row. Later changes to supplier/admin rates or recharge settings
-- must not change the ownership split for an already completed request.
CREATE OR REPLACE FUNCTION snapshot_supplier_usage_billing()
RETURNS TRIGGER AS $$
DECLARE
    snapshot_supplier_id BIGINT;
    snapshot_effective_rate NUMERIC;
    snapshot_local_adjustment NUMERIC;
    snapshot_admin_adjustment NUMERIC := 0;
    snapshot_base_rate NUMERIC;
    snapshot_recharge_ratio NUMERIC := 1;
    snapshot_model_cost_usd NUMERIC;
BEGIN
    IF NEW.group_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT g.supplier_id, g.rate_multiplier, g.supplier_admin_adjustment
    INTO snapshot_supplier_id, snapshot_effective_rate, snapshot_local_adjustment
    FROM groups AS g
    WHERE g.id = NEW.group_id;

    IF snapshot_supplier_id IS NULL THEN
        RETURN NEW;
    END IF;

    IF snapshot_local_adjustment IS NOT NULL THEN
        snapshot_admin_adjustment := snapshot_local_adjustment;
    ELSE
        SELECT CASE
            WHEN BTRIM(s.value) ~ '^[+-]?([0-9]+([.][0-9]*)?|[.][0-9]+)([eE][+-]?[0-9]+)?$'
                THEN s.value::NUMERIC
            ELSE 0
        END
        INTO snapshot_admin_adjustment
        FROM settings AS s
        WHERE s.key = 'supplier_global_rate_adjustment'
        LIMIT 1;

        snapshot_admin_adjustment := COALESCE(snapshot_admin_adjustment, 0);
    END IF;

    -- The approved resource request owns the supplier's base rate. A
    -- deterministic latest-row fallback also tolerates legacy duplicate links.
    SELECT r.rate_multiplier::NUMERIC
    INTO snapshot_base_rate
    FROM supplier_resource_requests AS r
    WHERE r.supplier_id = snapshot_supplier_id
      AND r.group_id = NEW.group_id
      AND r.status = 'approved'
    ORDER BY r.reviewed_at DESC NULLS LAST, r.id DESC
    LIMIT 1;

    IF snapshot_base_rate IS NULL THEN
        snapshot_base_rate := snapshot_effective_rate - snapshot_admin_adjustment;
    END IF;

    SELECT CASE
        WHEN BTRIM(s.value) ~ '^([0-9]+([.][0-9]*)?|[.][0-9]+)([eE][+-]?[0-9]+)?$'
            THEN CASE WHEN s.value::NUMERIC > 0 THEN s.value::NUMERIC ELSE 1 END
        ELSE 1
    END
    INTO snapshot_recharge_ratio
    FROM settings AS s
    WHERE s.key = 'BALANCE_RECHARGE_MULTIPLIER'
    LIMIT 1;

    snapshot_recharge_ratio := COALESCE(snapshot_recharge_ratio, 1);
    snapshot_model_cost_usd := CASE
        WHEN COALESCE(NEW.rate_multiplier, 0) > 0
            THEN COALESCE(NEW.actual_cost, 0) / NEW.rate_multiplier
        ELSE COALESCE(NEW.total_cost, 0)
    END;

    NEW.supplier_id := snapshot_supplier_id;
    NEW.supplier_base_rate := snapshot_base_rate;
    NEW.supplier_admin_adjustment := snapshot_admin_adjustment;
    NEW.supplier_model_cost_usd := snapshot_model_cost_usd;
    NEW.supplier_recharge_ratio := snapshot_recharge_ratio;
    NEW.supplier_earning_cny :=
        (snapshot_model_cost_usd * snapshot_base_rate) / snapshot_recharge_ratio;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_snapshot_supplier_usage_billing ON usage_logs;
CREATE TRIGGER trg_snapshot_supplier_usage_billing
BEFORE INSERT ON usage_logs
FOR EACH ROW
EXECUTE FUNCTION snapshot_supplier_usage_billing();
