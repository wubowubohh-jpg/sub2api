package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration200CorrectsCurrencyDirectionWithoutMaskingNegativeBalances(t *testing.T) {
	content, err := FS.ReadFile("200_supplier_settlement_currency_conversion.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "amount_cny = earning_usd / recharge_ratio")
	require.Contains(t, sql, "supplier_model_cost_usd * supplier_base_rate) / supplier_recharge_ratio")
	require.Contains(t, sql, "available_balance_cny = COALESCE(l.released_cny, 0) - COALESCE(w.deducted_cny, 0)")
	require.NotContains(t, strings.ToUpper(sql), "GREATEST(")
}

func TestMigration201SnapshotsSupplierBillingAtUsageInsert(t *testing.T) {
	content, err := FS.ReadFile("201_supplier_usage_billing_snapshot.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "BEFORE INSERT ON usage_logs")
	require.Contains(t, sql, "r.supplier_id = snapshot_supplier_id")
	require.Contains(t, sql, "r.group_id = NEW.group_id")
	require.Contains(t, sql, "r.status = 'approved'")
	require.Contains(t, sql, "ORDER BY r.reviewed_at DESC NULLS LAST, r.id DESC")
	require.Contains(t, sql, "supplier_global_rate_adjustment")
	require.Contains(t, sql, "BALANCE_RECHARGE_MULTIPLIER")
	require.Contains(t, sql, "NEW.actual_cost, 0) / NEW.rate_multiplier")
	require.Contains(t, sql, "ELSE COALESCE(NEW.total_cost, 0)")
	require.Contains(t, sql, "NEW.supplier_base_rate := snapshot_base_rate")
	require.Contains(t, sql, "NEW.supplier_admin_adjustment := snapshot_admin_adjustment")
	require.Contains(t, sql, "NEW.supplier_model_cost_usd := snapshot_model_cost_usd")
	require.Contains(t, sql, "NEW.supplier_recharge_ratio := snapshot_recharge_ratio")
	require.Contains(t, sql, "NEW.supplier_earning_cny :=")
}
