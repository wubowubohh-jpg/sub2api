//go:build unit

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/supplier"
	"github.com/Wei-Shaw/sub2api/ent/supplierledger"
	"github.com/stretchr/testify/require"
)

func TestAdminBillsPostgresSummaryDoesNotInheritEntryOrder(t *testing.T) {
	matcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		upper := strings.ToUpper(actual)
		switch expected {
		case "supplier_by_id":
			if !strings.Contains(actual, `FROM "suppliers"`) {
				return fmt.Errorf("expected supplier lookup, got %s", actual)
			}
		case "ledger_count":
			if !strings.Contains(actual, `FROM "supplier_ledgers"`) || !strings.Contains(upper, "COUNT(") {
				return fmt.Errorf("expected ledger count, got %s", actual)
			}
		case "ledger_summary_without_order":
			if !strings.Contains(actual, `FROM "supplier_ledgers"`) || !strings.Contains(upper, "SUM(") {
				return fmt.Errorf("expected ledger summary, got %s", actual)
			}
			if strings.Contains(upper, "ORDER BY") {
				return fmt.Errorf("ledger summary must not contain ORDER BY: %s", actual)
			}
		case "ledger_entries_with_order":
			if !strings.Contains(actual, `FROM "supplier_ledgers"`) || !strings.Contains(upper, "ORDER BY") {
				return fmt.Errorf("expected ordered ledger entries, got %s", actual)
			}
		default:
			return fmt.Errorf("unexpected query expectation %q", expected)
		}
		return nil
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	svc := NewSupplierService(client, nil, nil)
	t.Cleanup(svc.Stop)

	now := time.Now()
	mock.ExpectQuery("supplier_by_id").WillReturnRows(sqlmock.NewRows(supplier.Columns).AddRow(
		int64(7), int64(8), "Supplier", "https://relay.example.com", "", "approved",
		nil, "", nil, "", 0.0, 0.0, 0.0, []byte(`{}`), now, now,
	))
	mock.ExpectQuery("ledger_count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("ledger_summary_without_order").WillReturnRows(sqlmock.NewRows([]string{
		"supplier_earning_cny",
		"admin_markup_earning_cny",
	}).AddRow(0.0, 0.0))
	mock.ExpectQuery("ledger_entries_with_order").WillReturnRows(sqlmock.NewRows(supplierledger.Columns))

	items, total, summary, err := svc.AdminBills(context.Background(), 7, "", 20, 0)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
	require.Zero(t, summary.SupplierEarningCNY)
	require.Zero(t, summary.AdminMarkupEarningCNY)
	require.Zero(t, summary.SettlementTotalCNY)
	require.NoError(t, mock.ExpectationsWereMet())
}
