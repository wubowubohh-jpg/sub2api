//go:build unit

package service

import (
	"context"
	"database/sql"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newSupplierTestService(t *testing.T) (*SupplierService, context.Context) {
	db, err := sql.Open("sqlite", "file:supplier_service?mode=memory&cache=shared")
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close(); _ = db.Close() })
	return NewSupplierService(client), context.Background()
}

func TestSupplierEffectiveRateLocalOverridesGlobal(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	user := svc.db.User.Create().SetEmail("supplier@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("S").SetRelayURL("https://example.com").SetStatus("approved").SaveX(ctx)
	svc.db.Setting.Create().SetKey(SettingKeySupplierGlobalRateAdjustment).SetValue("0.02").SaveX(ctx)
	g := svc.db.Group.Create().SetName("G").SetSupplierID(sp.ID).SetPlatform("openai").SetRateMultiplier(0.04).SetSupplierAdminAdjustment(0.01).SaveX(ctx)
	rate, adjustment, err := svc.EffectiveRate(ctx, g)
	require.NoError(t, err)
	require.InDelta(t, 0.05, rate, 0.000001)
	require.InDelta(t, 0.01, adjustment, 0.000001)
}

func TestSupplierAccountRejectsForeignGroup(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	u1 := svc.db.User.Create().SetEmail("a@example.com").SetPasswordHash("x").SaveX(ctx)
	u2 := svc.db.User.Create().SetEmail("b@example.com").SetPasswordHash("x").SaveX(ctx)
	s1 := svc.db.Supplier.Create().SetUserID(u1.ID).SetName("A").SetRelayURL("https://a.example.com").SetStatus("approved").SaveX(ctx)
	s2 := svc.db.Supplier.Create().SetUserID(u2.ID).SetName("B").SetRelayURL("https://b.example.com").SetStatus("approved").SaveX(ctx)
	foreign := svc.db.Group.Create().SetName("foreign").SetSupplierID(s2.ID).SetPlatform("openai").SaveX(ctx)
	_, err := svc.CreateAccount(ctx, s1.ID, SupplierAccountInput{Name: "account", Platform: "openai", Type: "api_key", Credentials: map[string]any{"api_key": "secret"}, RateMultiplier: 1, Status: "active", Schedulable: true, GroupIDs: []int64{foreign.ID}})
	require.ErrorContains(t, err, "all groups must belong")
}
