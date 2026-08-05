//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSyncSupplierResourceProbeRateUpdatesBaseAndEffectiveRates(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close(); _ = db.Close() })
	ctx := context.Background()

	user := client.User.Create().SetEmail("supplier-probe-rate@example.com").SetPasswordHash("x").SaveX(ctx)
	supplier := client.Supplier.Create().SetUserID(user.ID).SetName("Supplier").SetRelayURL("https://relay.example.com").SetStatus("approved").SaveX(ctx)
	group := client.Group.Create().SetSupplierID(supplier.ID).SetName("A0001-live").SetPlatform(service.PlatformOpenAI).SetRateMultiplier(0.04).SaveX(ctx)
	account := client.Account.Create().
		SetSupplierID(supplier.ID).
		SetName("Supplier account").
		SetPlatform(service.PlatformOpenAI).
		SetType(service.AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "sk-test"}).
		SetExtra(map[string]any{
			service.UpstreamBillingProbeEnabledExtraKey:    true,
			service.UpstreamBillingRateSyncEnabledExtraKey: true,
		}).
		SetRateMultiplier(0.04).
		SaveX(ctx)
	request := client.SupplierResourceRequest.Create().
		SetSupplierID(supplier.ID).
		SetGroupName(group.Name).
		SetRelayName("Supplier relay").
		SetRelayURL("https://relay.example.com").
		SetAPIKeyEncrypted("encrypted").
		SetStatus("approved").
		SetGroupID(group.ID).
		SetAccountID(account.ID).
		SetRateMultiplier(0.04).
		SaveX(ctx)
	adjustmentSetting := client.Setting.Create().SetKey(service.SettingKeySupplierGlobalRateAdjustment).SetValue("0.02").SaveX(ctx)

	supplierID := supplier.ID
	probeAccount := &service.Account{
		ID:         account.ID,
		SupplierID: &supplierID,
		Extra:      account.Extra,
	}
	repo := newAccountRepositoryWithSQL(client, db, nil)
	baseRate := 0.075
	groupID, err := repo.syncSupplierResourceProbeRate(ctx, probeAccount, &baseRate)
	require.NoError(t, err)
	require.NotNil(t, groupID)
	require.Equal(t, group.ID, *groupID)
	require.InDelta(t, baseRate, client.SupplierResourceRequest.GetX(ctx, request.ID).RateMultiplier, 0.000001)
	require.InDelta(t, 0.095, client.Group.GetX(ctx, group.ID).RateMultiplier, 0.000001)
	require.InDelta(t, 0.095, client.Account.GetX(ctx, account.ID).RateMultiplier, 0.000001)

	client.Group.UpdateOneID(group.ID).SetSupplierAdminAdjustment(0.01).ExecX(ctx)
	baseRate = 0.08
	_, err = repo.syncSupplierResourceProbeRate(ctx, probeAccount, &baseRate)
	require.NoError(t, err)
	require.InDelta(t, baseRate, client.SupplierResourceRequest.GetX(ctx, request.ID).RateMultiplier, 0.000001)
	require.InDelta(t, 0.09, client.Group.GetX(ctx, group.ID).RateMultiplier, 0.000001)
	require.InDelta(t, 0.09, client.Account.GetX(ctx, account.ID).RateMultiplier, 0.000001)

	client.Group.UpdateOneID(group.ID).ClearSupplierAdminAdjustment().ExecX(ctx)
	client.Setting.UpdateOneID(adjustmentSetting.ID).SetValue("-0.2").ExecX(ctx)
	baseRate = 0.08
	_, err = repo.syncSupplierResourceProbeRate(ctx, probeAccount, &baseRate)
	require.ErrorContains(t, err, "invalid supplier effective rate")
	require.InDelta(t, 0.08, client.SupplierResourceRequest.GetX(ctx, request.ID).RateMultiplier, 0.000001)
	require.InDelta(t, 0.09, client.Group.GetX(ctx, group.ID).RateMultiplier, 0.000001)
	require.InDelta(t, 0.09, client.Account.GetX(ctx, account.ID).RateMultiplier, 0.000001)
}
