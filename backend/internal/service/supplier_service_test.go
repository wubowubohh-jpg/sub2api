//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/channelmonitorhistory"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/supplierledger"
	"github.com/Wei-Shaw/sub2api/ent/supplierresourcerequest"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSupplierResourceRequestSupplierViewRedactsInternalFields(t *testing.T) {
	reviewerID, groupID, accountID, monitorID := int64(91), int64(12), int64(13), int64(14)
	request := &dbent.SupplierResourceRequest{
		ID:              7,
		SupplierID:      3,
		GroupName:       "A0003-fast",
		RelayName:       "Fast relay",
		RelayURL:        "https://relay.example.com/v1",
		APIKeyEncrypted: "enc:v1:secret",
		Model:           "gpt-5.5",
		Status:          supplierresourcerequest.StatusApproved,
		ReviewedBy:      &reviewerID,
		GroupID:         &groupID,
		AccountID:       &accountID,
		MonitorID:       &monitorID,
	}
	view := SupplierResourceRequestView{
		SupplierResourceRequest: request,
		UpstreamBillingProbe: &SupplierResourceProbeView{
			AccountID: accountID,
			Enabled:   true,
			Snapshot:  map[string]any{"status": "ok"},
		},
	}
	safe := supplierResourceRequestForSupplier(view)
	body, err := json.Marshal(safe)
	require.NoError(t, err)
	encoded := string(body)
	require.Contains(t, encoded, "relay.example.com")
	require.NotContains(t, encoded, "supplier_id")
	require.NotContains(t, encoded, "reviewed_by")
	require.NotContains(t, encoded, "group_id")
	require.NotContains(t, encoded, "account_id")
	require.NotContains(t, encoded, "monitor_id")
	require.NotContains(t, encoded, "api_key_encrypted")
}

func TestSupplierBillItemDoesNotExposeConsumerIdentity(t *testing.T) {
	body, err := json.Marshal(SupplierBillItem{
		ID:           1,
		GroupID:      2,
		GroupName:    "A0001-settlement",
		Model:        "gpt-5.5",
		ModelCostUSD: 1.25,
		AmountCNY:    0.4,
	})
	require.NoError(t, err)

	encoded := string(body)
	require.NotContains(t, encoded, "user_id")
	require.NotContains(t, encoded, "user_email")
	require.NotContains(t, encoded, "username")
	require.NotContains(t, encoded, "api_key_id")
	require.NotContains(t, encoded, "account_id")
	require.NotContains(t, encoded, "request_id")
}

func newSupplierTestService(t *testing.T) (*SupplierService, context.Context) {
	db, err := sql.Open("sqlite", "file:supplier_service?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	svc := NewSupplierService(client, nil, nil)
	t.Cleanup(func() { svc.Stop(); _ = client.Close(); _ = db.Close() })
	return svc, context.Background()
}

type supplierTestEncryptor struct{}

func (supplierTestEncryptor) Encrypt(plaintext string) (string, error) {
	return "encrypted:" + plaintext, nil
}

func (supplierTestEncryptor) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, "encrypted:") {
		return "", fmt.Errorf("invalid ciphertext")
	}
	return strings.TrimPrefix(ciphertext, "encrypted:"), nil
}

func TestSupplierEffectiveRateLocalOverridesGlobal(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	user := svc.db.User.Create().SetEmail("supplier@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("S").SetRelayURL("https://example.com").SetStatus("approved").SaveX(ctx)
	svc.db.Setting.Create().SetKey(SettingKeySupplierGlobalRateAdjustment).SetValue("0.02").SaveX(ctx)
	g := svc.db.Group.Create().SetName("G").SetSupplierID(sp.ID).SetPlatform("openai").SetRateMultiplier(0.05).SetSupplierAdminAdjustment(0.01).SaveX(ctx)
	rate, adjustment, err := svc.EffectiveRate(ctx, g)
	require.NoError(t, err)
	require.InDelta(t, 0.05, rate, 0.000001)
	require.InDelta(t, 0.01, adjustment, 0.000001)
}

func TestReconcileUsageCreatesSupplierBill(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	user := svc.db.User.Create().SetEmail("supplier-bill@example.com").SetUsername("bill-user").SetPasswordHash("x").SaveX(ctx)
	adminGroup := svc.db.Group.Create().
		SetName("platform-group").
		SetPlatform(PlatformOpenAI).
		SaveX(ctx)
	adminAccount := svc.db.Account.Create().
		SetName("Platform account").
		SetPlatform(PlatformOpenAI).
		SetType(AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "sk-platform"}).
		SaveX(ctx)
	adminAPIKey := svc.db.APIKey.Create().
		SetUserID(user.ID).
		SetKey("sk-platform-user-key").
		SetName("Platform key").
		SetGroupID(adminGroup.ID).
		SaveX(ctx)
	adminLog := svc.db.UsageLog.Create().
		SetUserID(user.ID).
		SetAPIKeyID(adminAPIKey.ID).
		SetAccountID(adminAccount.ID).
		SetRequestID("platform-request-before-supplier").
		SetModel("gpt-5.5").
		SetGroupID(adminGroup.ID).
		SaveX(ctx)
	supplier := svc.db.Supplier.Create().
		SetUserID(user.ID).
		SetName("Bill Supplier").
		SetRelayURL("https://relay.example.com").
		SetStatus("approved").
		SaveX(ctx)
	group := svc.db.Group.Create().
		SetSupplierID(supplier.ID).
		SetName("A0001-bill").
		SetPlatform(PlatformOpenAI).
		SetRateMultiplier(0.04).
		SaveX(ctx)
	account := svc.db.Account.Create().
		SetSupplierID(supplier.ID).
		SetName("Bill account").
		SetPlatform(PlatformOpenAI).
		SetType(AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "sk-bill"}).
		SaveX(ctx)
	apiKey := svc.db.APIKey.Create().
		SetUserID(user.ID).
		SetKey("sk-bill-user-key").
		SetName("Bill key").
		SetGroupID(group.ID).
		SaveX(ctx)
	log := svc.db.UsageLog.Create().
		SetUserID(user.ID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetRequestID("supplier-bill-request").
		SetModel("gpt-5.5").
		SetGroupID(group.ID).
		SetActualCost(2).
		SetRateMultiplier(1).
		SaveX(ctx)

	// The administrator log has a lower ID. A one-row batch must still select
	// the supplier log rather than repeatedly consuming administrator history.
	count, err := svc.ReconcileUsage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Nil(t, svc.db.UsageLog.GetX(ctx, adminLog.ID).SupplierID)
	bills, err := svc.Bills(ctx, supplier.ID, "", 100)
	require.NoError(t, err)
	require.Len(t, bills, 1)
	require.Equal(t, group.Name, bills[0].GroupName)
	require.Equal(t, "gpt-5.5", bills[0].Model)
	require.InDelta(t, 2, bills[0].ModelCostUSD, 0.000001)
	require.InDelta(t, 1, bills[0].RechargeRatio, 0.000001)
	require.InDelta(t, 0.08, bills[0].EarningUSD, 0.000001)
	require.InDelta(t, 0.08, bills[0].AmountCNY, 0.000001)
	require.Equal(t, "earning", bills[0].EntryType)
	require.Equal(t, "pending", bills[0].Status)
	adminBills, total, summary, err := svc.AdminBills(ctx, supplier.ID, "", 20, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, adminBills, 1)
	require.InDelta(t, 0.08, summary.SupplierEarningCNY, 0.000001)
	require.Zero(t, summary.AdminMarkupEarningCNY)
	require.InDelta(t, 0.08, summary.SettlementTotalCNY, 0.000001)
	require.Equal(t, user.ID, *adminBills[0].UserID)
	require.Equal(t, user.Email, adminBills[0].UserEmail)
	require.Equal(t, user.Username, adminBills[0].Username)
	require.Equal(t, log.ID, *adminBills[0].UsageLogID)
	require.Equal(t, log.RequestID, adminBills[0].RequestID)

	// Reconciliation is idempotent; the same usage row must not create a
	// duplicate ledger entry when the endpoint is refreshed.
	count, err = svc.ReconcileUsage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 0, count)
	bills, err = svc.Bills(ctx, supplier.ID, "", 100)
	require.NoError(t, err)
	require.Len(t, bills, 1)
}

func TestReconcileUsageUsesSupplierBaseRateAndPreservesUsageRate(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	supplierUser := svc.db.User.Create().SetEmail("supplier-settlement-owner@example.com").SetPasswordHash("x").SaveX(ctx)
	consumer := svc.db.User.Create().SetEmail("supplier-settlement-consumer@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(supplierUser.ID).SetName("Settlement Supplier").SetRelayURL("https://supplier.example.com").SetStatus("approved").SaveX(ctx)
	adminAdjustment := 0.02
	group := svc.db.Group.Create().
		SetSupplierID(sp.ID).
		SetName("A0001-settlement").
		SetPlatform(PlatformOpenAI).
		SetRateMultiplier(0.06).
		SetSupplierAdminAdjustment(adminAdjustment).
		SaveX(ctx)
	resourceRequest := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(sp.ID).
		SetGroupName(group.Name).
		SetRelayName("Settlement Relay").
		SetRelayURL("https://supplier.example.com/v1").
		SetAPIKeyEncrypted("encrypted:settlement").
		SetModel("gpt-5.5").
		SetRateMultiplier(0.04).
		SetStatus(supplierresourcerequest.StatusApproved).
		SetGroupID(group.ID).
		SaveX(ctx)
	account := svc.db.Account.Create().
		SetSupplierID(sp.ID).
		SetName("Settlement account").
		SetPlatform(PlatformOpenAI).
		SetType(AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "sk-settlement"}).
		SaveX(ctx)
	apiKey := svc.db.APIKey.Create().
		SetUserID(consumer.ID).
		SetKey("sk-settlement-consumer").
		SetName("Settlement consumer key").
		SetGroupID(group.ID).
		SaveX(ctx)
	rechargeSetting := svc.db.Setting.Create().SetKey(SettingBalanceRechargeMult).SetValue("7").SaveX(ctx)

	// The user was charged with a separate 0.09x snapshot. Settlement must
	// retain that usage record while paying the supplier only at 0.04x.
	usageRate := 0.09
	log := svc.db.UsageLog.Create().
		SetUserID(consumer.ID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetRequestID("supplier-base-rate-snapshot").
		SetModel("gpt-5.5").
		SetGroupID(group.ID).
		SetTotalCost(2).
		SetActualCost(0.18).
		SetRateMultiplier(usageRate).
		SetSupplierID(sp.ID).
		SetSupplierBaseRate(0.04).
		SetSupplierAdminAdjustment(adminAdjustment).
		SetSupplierModelCostUsd(2).
		SetSupplierRechargeRatio(7).
		SetSupplierEarningCny(0.08 / 7).
		SaveX(ctx)

	// Change every mutable source before reconciliation. The ledger must use
	// the call-time usage snapshot instead of these later settings.
	svc.db.Group.UpdateOneID(group.ID).SetRateMultiplier(0.9).SetSupplierAdminAdjustment(0.5).ExecX(ctx)
	svc.db.SupplierResourceRequest.UpdateOneID(resourceRequest.ID).SetRateMultiplier(0.4).ExecX(ctx)
	svc.db.Setting.UpdateOneID(rechargeSetting.ID).SetValue("1").ExecX(ctx)

	count, err := svc.ReconcileUsage(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	ledger := svc.db.SupplierLedger.Query().OnlyX(ctx)
	require.InDelta(t, 0.04, ledger.BaseRate, 0.000001)
	require.InDelta(t, 0.02, ledger.AdminAdjustment, 0.000001)
	require.InDelta(t, 0.06, ledger.EffectiveRate, 0.000001)
	require.InDelta(t, 2, ledger.ModelCostUsd, 0.000001)
	require.InDelta(t, 0.08, ledger.EarningUsd, 0.000001)
	require.InDelta(t, 0.08/7, ledger.AmountCny, 0.000001)

	persistedLog := svc.db.UsageLog.GetX(ctx, log.ID)
	require.InDelta(t, usageRate, persistedLog.RateMultiplier, 0.000001)
	require.NotNil(t, persistedLog.SupplierBaseRate)
	require.InDelta(t, 0.04, *persistedLog.SupplierBaseRate, 0.000001)
	require.NotNil(t, persistedLog.SupplierAdminAdjustment)
	require.InDelta(t, 0.02, *persistedLog.SupplierAdminAdjustment, 0.000001)
	require.InDelta(t, 0.08/7, *persistedLog.SupplierEarningCny, 0.000001)
	require.InDelta(t, 0.08/7, svc.db.Supplier.GetX(ctx, sp.ID).PendingBalanceCny, 0.000001)
	adminBills, total, summary, err := svc.AdminBills(ctx, sp.ID, "", 20, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, adminBills, 1)
	require.InDelta(t, 0.08, adminBills[0].SupplierEarningUSD, 0.000001)
	require.InDelta(t, 0.08/7, adminBills[0].SupplierEarningCNY, 0.000001)
	require.InDelta(t, 0.04, adminBills[0].AdminMarkupEarningUSD, 0.000001)
	require.InDelta(t, 0.04/7, adminBills[0].AdminMarkupEarningCNY, 0.000001)
	require.InDelta(t, 0.12, adminBills[0].SettlementTotalUSD, 0.000001)
	require.InDelta(t, 0.12/7, adminBills[0].SettlementTotalCNY, 0.000001)
	require.InDelta(t, 0.08/7, summary.SupplierEarningCNY, 0.000001)
	require.InDelta(t, 0.04/7, summary.AdminMarkupEarningCNY, 0.000001)
	require.InDelta(t, 0.12/7, summary.SettlementTotalCNY, 0.000001)
	adminList, err := svc.AdminList(ctx, "")
	require.NoError(t, err)
	require.Len(t, adminList, 1)
	require.Equal(t, sp.ID, adminList[0].ID)
	require.InDelta(t, 0.08/7, adminList[0].SupplierEarningCNY, 0.000001)
	require.InDelta(t, 0.04/7, adminList[0].AdminMarkupEarningCNY, 0.000001)
	require.InDelta(t, 0.12/7, adminList[0].SettlementTotalCNY, 0.000001)

	reversal := svc.db.SupplierLedger.Create().
		SetSupplierID(sp.ID).
		SetGroupID(group.ID).
		SetUsageLogID(log.ID).
		SetReversalOfID(ledger.ID).
		SetEventKey("reversal:supplier-base-rate-snapshot").
		SetEntryType(supplierledger.EntryTypeReversal).
		SetBucket(supplierledger.BucketPending).
		SetBaseRate(0.04).
		SetAdminAdjustment(0.02).
		SetEffectiveRate(0.06).
		SetModelCostUsd(2).
		SetRechargeRatio(7).
		SetEarningUsd(-0.08).
		SetAmountCny(-0.08 / 7).
		SaveX(ctx)
	reversedBills, reversedTotal, reversedSummary, err := svc.AdminBills(ctx, sp.ID, "", 20, 0)
	require.NoError(t, err)
	require.Equal(t, 2, reversedTotal)
	require.Len(t, reversedBills, 2)
	require.Equal(t, reversal.ID, reversedBills[0].ID)
	require.InDelta(t, -0.04, reversedBills[0].AdminMarkupEarningUSD, 0.000001)
	require.InDelta(t, -0.04/7, reversedBills[0].AdminMarkupEarningCNY, 0.000001)
	require.InDelta(t, -0.12/7, reversedBills[0].SettlementTotalCNY, 0.000001)
	require.InDelta(t, 0, reversedSummary.SupplierEarningCNY, 0.000001)
	require.InDelta(t, 0, reversedSummary.AdminMarkupEarningCNY, 0.000001)
	require.InDelta(t, 0, reversedSummary.SettlementTotalCNY, 0.000001)

	// Additional changes after settlement also leave both snapshots untouched.
	svc.db.Group.UpdateOneID(group.ID).SetRateMultiplier(1.2).SetSupplierAdminAdjustment(0.7).ExecX(ctx)
	svc.db.SupplierResourceRequest.UpdateOneID(resourceRequest.ID).SetRateMultiplier(0.5).ExecX(ctx)
	svc.db.Setting.UpdateOneID(rechargeSetting.ID).SetValue("0.5").ExecX(ctx)
	ledger = svc.db.SupplierLedger.GetX(ctx, ledger.ID)
	require.InDelta(t, 0.04, ledger.BaseRate, 0.000001)
	require.InDelta(t, 0.02, ledger.AdminAdjustment, 0.000001)
	require.InDelta(t, 0.08/7, ledger.AmountCny, 0.000001)
}

func TestSupplierSettlementRejectsInvalidRechargeRatioAndEarningInput(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.db.Setting.Create().SetKey(SettingBalanceRechargeMult).SetValue("NaN").SaveX(ctx)
	require.Equal(t, 1.0, svc.supplierRechargeRatio(ctx))
	converted, err := supplierCNYFromUSD(10, 0.14)
	require.NoError(t, err)
	require.InDelta(t, 10/0.14, converted, 0.000001)
	converted, err = supplierCNYFromUSD(-1, 0.14)
	require.NoError(t, err)
	require.InDelta(t, -1/0.14, converted, 0.000001)

	user := svc.db.User.Create().SetEmail("invalid-settlement@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("Invalid Settlement").SetRelayURL("https://supplier.example.com").SetStatus("approved").SaveX(ctx)
	group := svc.db.Group.Create().SetSupplierID(sp.ID).SetName("A0001-invalid").SetPlatform(PlatformOpenAI).SaveX(ctx)
	err = svc.RecordEarning(ctx, "invalid-supplier-earning", 1, sp.ID, group.ID, math.Inf(1), 0.04, 0.01, 1)
	require.ErrorContains(t, err, "invalid supplier earning inputs")
	err = svc.RecordEarning(ctx, "negative-effective-rate", 1, sp.ID, group.ID, 1, 0.01, -0.02, 1)
	require.ErrorContains(t, err, "invalid supplier earning")
	require.Equal(t, 0, svc.db.SupplierLedger.Query().CountX(ctx))
}

func TestSupplierReleaseDueMovesBalanceOnce(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	user := svc.db.User.Create().SetEmail("release-once@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("Release Once").SetRelayURL("https://supplier.example.com").SetStatus("approved").SaveX(ctx)
	group := svc.db.Group.Create().SetSupplierID(sp.ID).SetName("A0001-release").SetPlatform(PlatformOpenAI).SaveX(ctx)
	require.NoError(t, svc.RecordEarning(ctx, "release-once", 1, sp.ID, group.ID, 2, 0.04, 0.01, 7))
	entry := svc.db.SupplierLedger.Query().OnlyX(ctx)
	svc.db.SupplierLedger.UpdateOneID(entry.ID).SetAvailableAt(time.Now().Add(-time.Minute)).ExecX(ctx)

	count, err := svc.ReleaseDue(ctx, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, count)
	count, err = svc.ReleaseDue(ctx, time.Now())
	require.NoError(t, err)
	require.Zero(t, count)

	updated := svc.db.Supplier.GetX(ctx, sp.ID)
	require.InDelta(t, 0, updated.PendingBalanceCny, 0.000001)
	require.InDelta(t, 0.08/7, updated.AvailableBalanceCny, 0.000001)
	require.Equal(t, "available", string(svc.db.SupplierLedger.GetX(ctx, entry.ID).Bucket))
}

func TestSupplierGroupMetricsUsesBestModelAvailability(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	group := svc.db.Group.Create().
		SetName("availability-max").
		SetPlatform(PlatformOpenAI).
		SaveX(ctx)
	monitor := svc.db.ChannelMonitor.Create().
		SetName("availability-monitor").
		SetProvider("openai").
		SetEndpoint("https://example.com").
		SetAPIKeyEncrypted("encrypted").
		SetPrimaryModel("gpt-primary").
		SetExtraModels([]string{"gpt-secondary"}).
		SetGroupID(group.ID).
		SetIntervalSeconds(60).
		SetCreatedBy(1).
		SaveX(ctx)

	checkedAt := time.Now().Add(-time.Minute)
	statuses := map[string][]channelmonitorhistory.Status{
		"gpt-primary": {
			channelmonitorhistory.StatusOperational,
			channelmonitorhistory.StatusFailed,
			channelmonitorhistory.StatusFailed,
			channelmonitorhistory.StatusFailed,
		},
		"gpt-secondary": {
			channelmonitorhistory.StatusFailed,
			channelmonitorhistory.StatusOperational,
			channelmonitorhistory.StatusDegraded,
			channelmonitorhistory.StatusOperational,
		},
	}
	for model, modelStatuses := range statuses {
		for index, status := range modelStatuses {
			svc.db.ChannelMonitorHistory.Create().
				SetMonitorID(monitor.ID).
				SetModel(model).
				SetStatus(status).
				SetCheckedAt(checkedAt.Add(time.Duration(index) * time.Second)).
				SaveX(ctx)
		}
	}

	metrics, err := svc.GroupMetrics(ctx, group.ID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.NotNil(t, metrics.Availability)
	require.InDelta(t, 75, *metrics.Availability, 0.000001)
	require.Equal(t, string(channelmonitorhistory.StatusOperational), metrics.MonitorStatus)
}

func TestSupplierSettlementSettingsRemoveMinimumAndConfigureDelay(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	settings, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, 7, settings.SettlementDelayDays)

	settings, err = svc.UpdateSettings(ctx, SupplierSettings{GlobalRateAdjustment: 0.02, SettlementDelayDays: 3})
	require.NoError(t, err)
	require.InDelta(t, 0.02, settings.GlobalRateAdjustment, 0.000001)
	require.Equal(t, 3, settings.SettlementDelayDays)
	require.Equal(t, 3, svc.supplierSettlementDelayDays(ctx))

	rateOwner := svc.db.User.Create().SetEmail("settings-rate-owner@example.com").SetPasswordHash("x").SaveX(ctx)
	rateSupplier := svc.db.Supplier.Create().SetUserID(rateOwner.ID).SetName("Settings Rate Supplier").SetRelayURL("https://rate.example.com").SetStatus("approved").SaveX(ctx)
	rateGroup := svc.db.Group.Create().SetSupplierID(rateSupplier.ID).SetName("A0001-settings-rate").SetPlatform(PlatformOpenAI).SetRateMultiplier(0.06).SaveX(ctx)
	svc.db.SupplierResourceRequest.Create().SetSupplierID(rateSupplier.ID).SetGroupName(rateGroup.Name).SetRelayName("Settings Rate Relay").SetRelayURL("https://rate.example.com/v1").SetAPIKeyEncrypted("encrypted:settings-rate").SetModel("gpt-5.5").SetRateMultiplier(0.04).SetStatus(supplierresourcerequest.StatusApproved).SetGroupID(rateGroup.ID).SaveX(ctx)
	_, err = svc.UpdateSettings(ctx, SupplierSettings{GlobalRateAdjustment: -0.05, SettlementDelayDays: 3})
	require.ErrorContains(t, err, "rate negative")
	settings, err = svc.GetSettings(ctx)
	require.NoError(t, err)
	require.InDelta(t, 0.02, settings.GlobalRateAdjustment, 0.000001)

	user := svc.db.User.Create().SetEmail("small-withdrawal@example.com").SetPasswordHash("x").SaveX(ctx)
	supplier := svc.db.Supplier.Create().SetUserID(user.ID).SetName("Small Withdrawal").SetRelayURL("https://example.com").SetStatus("approved").SetAvailableBalanceCny(1).SaveX(ctx)
	withdrawal, err := svc.Withdraw(ctx, supplier.ID, 0.5, "alipay", map[string]any{"account": "test"})
	require.NoError(t, err)
	require.Equal(t, 0.5, withdrawal.AmountCny)
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

func TestSupplierGroupsAlwaysUseOwnedPrefix(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	user := svc.db.User.Create().SetEmail("group-prefix@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("Prefix Supplier").SetRelayURL("https://example.com").SetStatus("approved").SaveX(ctx)

	created, err := svc.CreateGroup(ctx, sp.ID, SupplierGroupInput{Name: "fast", Platform: "openai", RateMultiplier: 0.04})
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("A%04d-fast", sp.ID), created.Name)

	updated, err := svc.UpdateGroup(ctx, sp.ID, created.ID, SupplierGroupInput{Name: "slow", Platform: "openai", RateMultiplier: 0.05})
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("A%04d-slow", sp.ID), updated.Name)
}

func TestSupplierBuildResourceRequestTestAccount(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
	user := svc.db.User.Create().SetEmail("resource-test@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("Resource Test").SetRelayURL("https://relay.example.com").SetStatus("approved").SaveX(ctx)
	req := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(sp.ID).
		SetGroupName("Resource Group").
		SetRelayName("Resource Relay").
		SetRelayURL("https://relay.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-resource-secret").
		SetModel("gpt-5.5").
		SaveX(ctx)

	account, model, err := svc.BuildResourceRequestTestAccount(ctx, req.ID)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.5", model)
	require.Equal(t, -req.ID, account.ID)
	require.Equal(t, PlatformOpenAI, account.Platform)
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, "sk-resource-secret", account.GetCredential("api_key"))
	require.Equal(t, "https://relay.example.com/v1", account.GetOpenAIBaseURL())
	require.False(t, account.Schedulable)
	require.False(t, account.Extra["openai_responses_supported"].(bool))
}

func TestSupplierCreateResourceRequestNormalizesNameModelsAndCipher(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
	user := svc.db.User.Create().SetEmail("resource-create@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("Resource Create").SetRelayURL("https://example.com").SetStatus("approved").SaveX(ctx)
	probeEnabled := false
	rateMultiplier := 0.04

	created, err := svc.CreateResourceRequest(ctx, sp.ID, SupplierResourceApplication{
		GroupNameSuffix: "fast-line",
		RelayName:       "Fast Relay",
		RelayURL:        "https://example.com/v1",
		APIKey:          "sk-resource-create",
		MonitorModel:    "gpt-5.5",
		SupportedModels: []string{"gpt-5.5", "gpt-5.4", "gpt-5.5"},
		ProbeEnabled:    &probeEnabled,
		RateMultiplier:  &rateMultiplier,
	})
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("A%04d-fast-line", sp.ID), created.GroupName)
	require.Equal(t, []string{"gpt-5.5", "gpt-5.4"}, created.SupportedModels)
	require.Equal(t, "gpt-5.5", created.Model)
	require.False(t, created.ProbeEnabled)
	require.InDelta(t, 0.04, created.RateMultiplier, 0.000001)
	require.Equal(t, "enc:v1:encrypted:sk-resource-create", created.APIKeyEncrypted)

	rotated, err := svc.CreateResourceRequest(ctx, sp.ID, SupplierResourceApplication{
		GroupName:       "fast-line",
		RelayName:       "Fast Relay 2",
		RelayURL:        "https://example.com/v1",
		APIKey:          "sk-resource-rotated",
		ProbeModel:      "gpt-5.4",
		SupportedModels: []string{"gpt-5.4"},
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, rotated.ID)
	require.Equal(t, "enc:v1:encrypted:sk-resource-rotated", rotated.APIKeyEncrypted)
	require.True(t, rotated.ProbeEnabled)
	require.InDelta(t, 0.04, rotated.RateMultiplier, 0.000001)
}

func TestSupplierCredentialCompatibilityIsExplicit(t *testing.T) {
	svc, _ := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}

	plain, err := svc.decryptSupplierAPIKey("encrypted:sk-legacy-cipher")
	require.NoError(t, err)
	require.Equal(t, "sk-legacy-cipher", plain)

	plain, err = svc.decryptSupplierAPIKey("sk-legacy-plaintext")
	require.NoError(t, err)
	require.Equal(t, "sk-legacy-plaintext", plain)

	_, err = svc.decryptSupplierAPIKey("enc:v1:not-valid-for-current-key")
	require.Error(t, err)
	_, err = svc.decryptSupplierAPIKey("base64-looking-but-undecryptable")
	require.Error(t, err)
}

func TestSupplierReviewResourceRequestCreatesTestableOpenAIAccount(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
	user := svc.db.User.Create().SetEmail("resource-review@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("Resource Review").SetRelayURL("https://relay.example.com").SetStatus("approved").SaveX(ctx)
	req := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(sp.ID).
		SetGroupName("Approved Group").
		SetRelayName("Approved Relay").
		SetRelayURL("https://relay.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-approved-secret").
		SetModel("gpt-5.5").
		SetRateMultiplier(0.06).
		SaveX(ctx)

	approved, err := svc.ReviewResourceRequest(ctx, req.ID, user.ID, true, "approved")
	require.NoError(t, err)
	require.NotNil(t, approved.AccountID)
	account := svc.db.Account.GetX(ctx, *approved.AccountID)
	createdGroup := svc.db.Group.GetX(ctx, *approved.GroupID)
	require.Equal(t, fmt.Sprintf("A%04d-Approved Group", sp.ID), createdGroup.Name)
	require.InDelta(t, 0.06, createdGroup.RateMultiplier, 0.000001)
	require.Nil(t, createdGroup.DailyLimitUsd)
	require.Nil(t, createdGroup.WeeklyLimitUsd)
	require.Nil(t, createdGroup.MonthlyLimitUsd)
	require.Zero(t, createdGroup.RpmLimit)
	require.Empty(t, createdGroup.MaxReasoningEffort)
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, "sk-approved-secret", account.Credentials["api_key"])
	require.Equal(t, "https://relay.example.com/v1", account.Credentials["base_url"])
	require.Equal(t, map[string]any{"gpt-5.5": "gpt-5.5"}, account.Credentials["model_mapping"])
	require.Equal(t, false, account.Extra["openai_responses_supported"])
	require.Equal(t, true, account.Extra[UpstreamBillingProbeEnabledExtraKey])
	require.Equal(t, true, account.Extra[UpstreamBillingRateSyncEnabledExtraKey])

	monitor := svc.db.ChannelMonitor.GetX(ctx, *approved.MonitorID)
	require.Equal(t, "gpt-5.5", monitor.PrimaryModel)
	require.Equal(t, "encrypted:sk-approved-secret", monitor.APIKeyEncrypted)
}

func TestSupplierResourceProbeAndCredentialUpdatesAreOwnerScoped(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
	ownerUser := svc.db.User.Create().SetEmail("probe-owner@example.com").SetPasswordHash("x").SaveX(ctx)
	otherUser := svc.db.User.Create().SetEmail("probe-other@example.com").SetPasswordHash("x").SaveX(ctx)
	owner := svc.db.Supplier.Create().SetUserID(ownerUser.ID).SetName("Owner").SetRelayURL("https://owner.example.com").SetStatus("approved").SaveX(ctx)
	other := svc.db.Supplier.Create().SetUserID(otherUser.ID).SetName("Other").SetRelayURL("https://other.example.com").SetStatus("approved").SaveX(ctx)
	req := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(owner.ID).
		SetGroupName("Owner Resource").
		SetRelayName("Owner Relay").
		SetRelayURL("https://relay.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-owner-secret").
		SetModel("gpt-5.5").
		SaveX(ctx)
	approved, err := svc.ReviewResourceRequest(ctx, req.ID, ownerUser.ID, true, "approved")
	require.NoError(t, err)

	_, err = svc.UpdateResourceRequestProbe(ctx, other.ID, approved.ID, false)
	require.Error(t, err)
	view, err := svc.UpdateResourceRequestProbe(ctx, owner.ID, approved.ID, false)
	require.NoError(t, err)
	require.False(t, view.UpstreamBillingProbeEnabled)
	require.Equal(t, "disabled", view.UpstreamProbeStatus)
	accountAfterDisable := svc.db.Account.GetX(ctx, *approved.AccountID)
	require.Equal(t, false, accountAfterDisable.Extra[UpstreamBillingRateSyncEnabledExtraKey])
	view, err = svc.UpdateResourceRequestProbe(ctx, owner.ID, approved.ID, true)
	require.NoError(t, err)
	require.True(t, view.UpstreamBillingProbeEnabled)
	accountAfterEnable := svc.db.Account.GetX(ctx, *approved.AccountID)
	require.Equal(t, true, accountAfterEnable.Extra[UpstreamBillingRateSyncEnabledExtraKey])

	_, err = svc.UpdateResourceRequestAPIKey(ctx, other.ID, approved.ID, "sk-foreign")
	require.Error(t, err)
	view, err = svc.UpdateResourceRequestAPIKey(ctx, owner.ID, approved.ID, "sk-owner-rotated")
	require.NoError(t, err)
	account := svc.db.Account.GetX(ctx, *approved.AccountID)
	require.Equal(t, "sk-owner-rotated", account.Credentials["api_key"])
	monitor := svc.db.ChannelMonitor.GetX(ctx, *approved.MonitorID)
	require.Equal(t, "encrypted:sk-owner-rotated", monitor.APIKeyEncrypted)
	storedRequest := svc.db.SupplierResourceRequest.GetX(ctx, approved.ID)
	require.Equal(t, "enc:v1:encrypted:sk-owner-rotated", storedRequest.APIKeyEncrypted)
}

func TestSupplierResourceAvailabilityUpdatesRuntimeResources(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
	invalidator := &authCacheInvalidatorStub{}
	svc.authCacheInvalidator = invalidator
	ownerUser := svc.db.User.Create().SetEmail("availability-owner@example.com").SetPasswordHash("x").SaveX(ctx)
	otherUser := svc.db.User.Create().SetEmail("availability-other@example.com").SetPasswordHash("x").SaveX(ctx)
	owner := svc.db.Supplier.Create().SetUserID(ownerUser.ID).SetName("Availability Owner").SetRelayURL("https://owner.example.com").SetStatus("approved").SaveX(ctx)
	other := svc.db.Supplier.Create().SetUserID(otherUser.ID).SetName("Availability Other").SetRelayURL("https://other.example.com").SetStatus("approved").SaveX(ctx)
	req := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(owner.ID).
		SetGroupName("Availability Resource").
		SetRelayName("Availability Relay").
		SetRelayURL("https://relay.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-availability-secret").
		SetModel("gpt-5.5").
		SaveX(ctx)
	approved, err := svc.ReviewResourceRequest(ctx, req.ID, ownerUser.ID, true, "approved")
	require.NoError(t, err)

	_, err = svc.SetResourceRequestOnline(ctx, other.ID, approved.ID, false)
	require.Error(t, err)
	view, err := svc.SetResourceRequestOnline(ctx, owner.ID, approved.ID, false)
	require.NoError(t, err)
	require.False(t, view.ResourceOnline)
	require.False(t, view.ForcedOffline)
	require.Equal(t, "disabled", svc.db.Group.GetX(ctx, *approved.GroupID).Status)
	require.False(t, svc.db.Account.GetX(ctx, *approved.AccountID).Schedulable)
	require.False(t, svc.db.ChannelMonitor.GetX(ctx, *approved.MonitorID).Enabled)
	require.Equal(t, []int64{*approved.GroupID}, invalidator.groupIDs)

	view, err = svc.SetResourceRequestOnline(ctx, owner.ID, approved.ID, true)
	require.NoError(t, err)
	require.True(t, view.ResourceOnline)
	require.Equal(t, "active", svc.db.Group.GetX(ctx, *approved.GroupID).Status)
	require.True(t, svc.db.Account.GetX(ctx, *approved.AccountID).Schedulable)
	require.True(t, svc.db.ChannelMonitor.GetX(ctx, *approved.MonitorID).Enabled)
	require.Equal(t, []int64{*approved.GroupID, *approved.GroupID}, invalidator.groupIDs)

	svc.db.Group.UpdateOneID(*approved.GroupID).SetSupplierForcedOffline(true).ExecX(ctx)
	_, err = svc.SetResourceRequestOnline(ctx, owner.ID, approved.ID, false)
	require.NoError(t, err)
	supplierView, err := svc.ResourceRequestForSupplier(ctx, owner.ID, approved.ID)
	require.NoError(t, err)
	require.True(t, supplierView.ForcedOffline)
	require.False(t, supplierView.ResourceOnline)
	_, err = svc.SetResourceRequestOnline(ctx, owner.ID, approved.ID, true)
	require.Error(t, err)
	require.Equal(t, "disabled", svc.db.Group.GetX(ctx, *approved.GroupID).Status)

	_, err = svc.UpdateResourceRequestAPIKey(ctx, owner.ID, approved.ID, "sk-availability-rotated")
	require.NoError(t, err)
	require.False(t, svc.db.Account.GetX(ctx, *approved.AccountID).Schedulable)
	require.False(t, svc.db.ChannelMonitor.GetX(ctx, *approved.MonitorID).Enabled)
}

func TestSupplierResourceModelUpdatesAreImmediateAndOwnerScoped(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
	invalidator := &authCacheInvalidatorStub{}
	svc.authCacheInvalidator = invalidator
	ownerUser := svc.db.User.Create().SetEmail("models-owner@example.com").SetPasswordHash("x").SaveX(ctx)
	otherUser := svc.db.User.Create().SetEmail("models-other@example.com").SetPasswordHash("x").SaveX(ctx)
	owner := svc.db.Supplier.Create().SetUserID(ownerUser.ID).SetName("Model Owner").SetRelayURL("https://owner.example.com").SetStatus("approved").SaveX(ctx)
	other := svc.db.Supplier.Create().SetUserID(otherUser.ID).SetName("Model Other").SetRelayURL("https://other.example.com").SetStatus("approved").SaveX(ctx)
	req := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(owner.ID).
		SetGroupName("Model Resource").
		SetRelayName("Model Relay").
		SetRelayURL("https://relay.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-model-secret").
		SetModel("gpt-5.5").
		SetSupportedModels([]string{"gpt-5.5"}).
		SaveX(ctx)
	approved, err := svc.ReviewResourceRequest(ctx, req.ID, ownerUser.ID, true, "approved")
	require.NoError(t, err)

	_, err = svc.UpdateResourceRequestModels(ctx, other.ID, approved.ID, []string{"gpt-5.6"}, "gpt-5.6")
	require.Error(t, err)
	_, err = svc.UpdateResourceRequestModels(ctx, owner.ID, approved.ID, []string{"gpt-5.6"}, "gpt-5.4")
	require.Error(t, err)

	view, err := svc.UpdateResourceRequestModels(
		ctx,
		owner.ID,
		approved.ID,
		[]string{"gpt-5.6", "gpt-5.4", "gpt-5.6"},
		"gpt-5.4",
	)
	require.NoError(t, err)
	require.Equal(t, supplierresourcerequest.StatusApproved, view.Status)
	require.Equal(t, "approved", view.ReviewNote)
	require.Equal(t, "gpt-5.4", view.Model)
	require.Equal(t, []string{"gpt-5.6", "gpt-5.4"}, view.SupportedModels)
	require.Equal(t, map[string]any{"gpt-5.6": "gpt-5.6", "gpt-5.4": "gpt-5.4"}, svc.db.Account.GetX(ctx, *approved.AccountID).Credentials["model_mapping"])
	monitor := svc.db.ChannelMonitor.GetX(ctx, *approved.MonitorID)
	require.Equal(t, "gpt-5.4", monitor.PrimaryModel)
	require.Equal(t, []string{"gpt-5.6"}, monitor.ExtraModels)
	require.Equal(t, []int64{*approved.GroupID}, invalidator.groupIDs)

	pending := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(owner.ID).
		SetGroupName("Pending Model Resource").
		SetRelayName("Pending Model Relay").
		SetRelayURL("https://pending.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-pending-model").
		SetModel("gpt-5.5").
		SetSupportedModels([]string{"gpt-5.5"}).
		SaveX(ctx)
	pendingView, err := svc.UpdateResourceRequestModels(ctx, owner.ID, pending.ID, []string{"gpt-5.6"}, "gpt-5.6")
	require.NoError(t, err)
	require.Equal(t, supplierresourcerequest.StatusPending, pendingView.Status)
	require.Equal(t, "gpt-5.6", pendingView.Model)
	require.Equal(t, []string{"gpt-5.6"}, pendingView.SupportedModels)
	require.Equal(t, []int64{*approved.GroupID}, invalidator.groupIDs)
}

func TestSupplierResourceRateUpdatesConfiguredRateImmediately(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
	invalidator := &authCacheInvalidatorStub{}
	svc.authCacheInvalidator = invalidator
	ownerUser := svc.db.User.Create().SetEmail("rate-owner@example.com").SetPasswordHash("x").SaveX(ctx)
	otherUser := svc.db.User.Create().SetEmail("rate-other@example.com").SetPasswordHash("x").SaveX(ctx)
	owner := svc.db.Supplier.Create().SetUserID(ownerUser.ID).SetName("Rate Owner").SetRelayURL("https://owner.example.com").SetStatus("approved").SaveX(ctx)
	other := svc.db.Supplier.Create().SetUserID(otherUser.ID).SetName("Rate Other").SetRelayURL("https://other.example.com").SetStatus("approved").SaveX(ctx)
	req := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(owner.ID).
		SetGroupName("Rate Resource").
		SetRelayName("Rate Relay").
		SetRelayURL("https://relay.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-rate-secret").
		SetModel("gpt-5.5").
		SetRateMultiplier(0.04).
		SaveX(ctx)
	approved, err := svc.ReviewResourceRequest(ctx, req.ID, ownerUser.ID, true, "approved")
	require.NoError(t, err)

	_, err = svc.UpdateResourceRequestRate(ctx, other.ID, approved.ID, 0.05)
	require.Error(t, err)
	view, err := svc.UpdateResourceRequestRate(ctx, owner.ID, approved.ID, 0.05)
	require.NoError(t, err)
	require.InDelta(t, 0.05, view.RateMultiplier, 0.000001)
	require.Equal(t, "configured", view.RateSource)
	require.InDelta(t, 0.05, view.AppliedRateMultiplier, 0.000001)
	require.InDelta(t, 0.05, svc.db.Group.GetX(ctx, *approved.GroupID).RateMultiplier, 0.000001)
	require.Equal(t, []int64{*approved.GroupID}, invalidator.groupIDs)

	configuredRate, adminIncrement := 0.06, 0.02
	view, err = svc.AdminUpdateResourceRequestRate(
		ctx, approved.ID, &configuredRate, &adminIncrement,
	)
	require.NoError(t, err)
	require.InDelta(t, 0.06, view.RateMultiplier, 0.000001)
	require.InDelta(t, 0.02, view.AdminRateAdjustment, 0.000001)
	require.Equal(t, []int64{*approved.GroupID, *approved.GroupID}, invalidator.groupIDs)

	account := svc.db.Account.GetX(ctx, *approved.AccountID)
	extra := shallowCopyMap(account.Extra)
	extra[UpstreamBillingProbeEnabledExtraKey] = true
	extra[UpstreamBillingProbeExtraKey] = map[string]any{
		"status": "ok",
		"data":   map[string]any{"effective_rate_multiplier": 0.07},
	}
	svc.db.Account.UpdateOne(account).SetExtra(extra).ExecX(ctx)
	view, err = svc.resourceRequestView(ctx, owner.ID, approved.ID)
	require.NoError(t, err)
	require.Equal(t, "configured", view.RateSource)
	require.InDelta(t, 0.07, *view.UpstreamRate, 0.000001)
	require.InDelta(t, 0.06, view.AppliedRateMultiplier, 0.000001)
	require.InDelta(t, 0.08, view.EffectiveRateMultiplier, 0.000001)

	view, err = svc.UpdateResourceRequestProbe(ctx, owner.ID, approved.ID, false)
	require.NoError(t, err)
	require.Equal(t, "configured", view.RateSource)
	require.InDelta(t, 0.06, view.AppliedRateMultiplier, 0.000001)
	require.InDelta(t, 0.08, view.EffectiveRateMultiplier, 0.000001)
}

func TestAdminUpdateSupplierResourceSynchronizesRuntimeResources(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
	invalidator := &authCacheInvalidatorStub{}
	svc.authCacheInvalidator = invalidator
	user := svc.db.User.Create().SetEmail("admin-resource-edit@example.com").SetPasswordHash("x").SaveX(ctx)
	sp := svc.db.Supplier.Create().SetUserID(user.ID).SetName("Editable Supplier").SetRelayURL("https://supplier.example.com").SetStatus("approved").SaveX(ctx)
	req := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(sp.ID).
		SetGroupName("original").
		SetRelayName("Original Relay").
		SetRelayURL("https://old.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-old-secret").
		SetModel("gpt-5.5").
		SetSupportedModels([]string{"gpt-5.5"}).
		SetProbeEnabled(true).
		SetRateMultiplier(0.04).
		SaveX(ctx)
	approved, err := svc.ReviewResourceRequest(ctx, req.ID, user.ID, true, "initial review")
	require.NoError(t, err)

	adjustment := 0.015
	view, err := svc.AdminUpdateResourceRequest(ctx, approved.ID, SupplierResourceAdminUpdate{
		GroupName:           "premium-line",
		RelayName:           "Premium Relay",
		RelayURL:            "https://new.example.com/api",
		APIKey:              "sk-new-secret",
		MonitorModel:        "gpt-5.6",
		SupportedModels:     []string{"gpt-5.6", "gpt-5.5"},
		ProbeEnabled:        false,
		RateMultiplier:      0.075,
		AdminRateAdjustment: &adjustment,
		ReviewNote:          "administrator updated all resource fields",
	})
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("A%04d-premium-line", sp.ID), view.GroupName)
	require.Equal(t, "Premium Relay", view.RelayName)
	require.Equal(t, "https://new.example.com/api", view.RelayURL)
	require.Equal(t, "gpt-5.6", view.Model)
	require.Equal(t, []string{"gpt-5.6", "gpt-5.5"}, view.SupportedModels)
	require.False(t, view.ProbeEnabled)
	require.InDelta(t, 0.075, view.RateMultiplier, 0.000001)
	require.InDelta(t, adjustment, view.AdminRateAdjustment, 0.000001)
	require.Equal(t, "administrator updated all resource fields", view.ReviewNote)

	storedRequest := svc.db.SupplierResourceRequest.GetX(ctx, approved.ID)
	require.Equal(t, "enc:v1:encrypted:sk-new-secret", storedRequest.APIKeyEncrypted)
	storedGroup := svc.db.Group.GetX(ctx, *approved.GroupID)
	require.Equal(t, view.GroupName, storedGroup.Name)
	require.Equal(t, "Premium Relay", *storedGroup.Description)
	require.InDelta(t, 0.09, storedGroup.RateMultiplier, 0.000001)
	require.NotNil(t, storedGroup.SupplierAdminAdjustment)
	require.InDelta(t, adjustment, *storedGroup.SupplierAdminAdjustment, 0.000001)

	storedAccount := svc.db.Account.GetX(ctx, *approved.AccountID)
	require.Equal(t, "Premium Relay", storedAccount.Name)
	require.Equal(t, "sk-new-secret", storedAccount.Credentials["api_key"])
	require.Equal(t, "https://new.example.com/api", storedAccount.Credentials["base_url"])
	require.Equal(t, map[string]any{"gpt-5.6": "gpt-5.6", "gpt-5.5": "gpt-5.5"}, storedAccount.Credentials["model_mapping"])
	require.InDelta(t, 0.09, storedAccount.RateMultiplier, 0.000001)
	require.Equal(t, false, storedAccount.Extra[UpstreamBillingProbeEnabledExtraKey])
	require.Equal(t, false, storedAccount.Extra[UpstreamBillingRateSyncEnabledExtraKey])

	storedMonitor := svc.db.ChannelMonitor.GetX(ctx, *approved.MonitorID)
	require.Equal(t, "Premium Relay", storedMonitor.Name)
	require.Equal(t, "https://new.example.com/api", storedMonitor.Endpoint)
	require.Equal(t, "gpt-5.6", storedMonitor.PrimaryModel)
	require.Equal(t, []string{"gpt-5.5"}, storedMonitor.ExtraModels)
	require.Equal(t, view.GroupName, storedMonitor.GroupName)
	require.Equal(t, "encrypted:sk-new-secret", storedMonitor.APIKeyEncrypted)
	require.Equal(t, []int64{*approved.GroupID}, invalidator.groupIDs)
}

func TestSupplierPendingResourceRateUpdateDoesNotTriggerReview(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	invalidator := &authCacheInvalidatorStub{}
	svc.authCacheInvalidator = invalidator
	user := svc.db.User.Create().SetEmail("pending-rate@example.com").SetPasswordHash("x").SaveX(ctx)
	owner := svc.db.Supplier.Create().
		SetUserID(user.ID).
		SetName("Pending Rate Owner").
		SetRelayURL("https://owner.example.com").
		SetStatus("approved").
		SaveX(ctx)
	request := svc.db.SupplierResourceRequest.Create().
		SetSupplierID(owner.ID).
		SetGroupName("Pending Resource").
		SetRelayName("Pending Relay").
		SetRelayURL("https://relay.example.com/v1").
		SetAPIKeyEncrypted("encrypted:sk-pending-secret").
		SetModel("gpt-5.5").
		SetRateMultiplier(0.04).
		SaveX(ctx)

	view, err := svc.UpdateResourceRequestRate(ctx, owner.ID, request.ID, 0)
	require.NoError(t, err)
	require.Equal(t, supplierresourcerequest.StatusPending, view.Status)
	require.Nil(t, view.ReviewedAt)
	require.Nil(t, view.ReviewedBy)
	require.Nil(t, view.GroupID)
	require.Zero(t, view.RateMultiplier)
	require.Empty(t, invalidator.groupIDs)

	_, err = svc.UpdateResourceRequestRate(ctx, owner.ID, request.ID, -0.01)
	require.Error(t, err)
	require.Zero(t, svc.db.SupplierResourceRequest.GetX(ctx, request.ID).RateMultiplier)
}
