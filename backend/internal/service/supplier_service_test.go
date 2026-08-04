//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
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

func newSupplierTestService(t *testing.T) (*SupplierService, context.Context) {
	db, err := sql.Open("sqlite", "file:supplier_service?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	svc := NewSupplierService(client, nil)
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
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, "sk-approved-secret", account.Credentials["api_key"])
	require.Equal(t, "https://relay.example.com/v1", account.Credentials["base_url"])
	require.Equal(t, map[string]any{"gpt-5.5": "gpt-5.5"}, account.Credentials["model_mapping"])
	require.Equal(t, false, account.Extra["openai_responses_supported"])
	require.Equal(t, true, account.Extra[UpstreamBillingProbeEnabledExtraKey])
	require.Equal(t, false, account.Extra[UpstreamBillingRateSyncEnabledExtraKey])

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

func TestSupplierResourceRateUsesProbeOrConfiguredRateWithAdminIncrement(t *testing.T) {
	svc, ctx := newSupplierTestService(t)
	svc.encryptor = supplierTestEncryptor{}
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
	require.InDelta(t, 0.05, svc.db.Group.GetX(ctx, *approved.GroupID).RateMultiplier, 0.000001)

	configuredRate, adminIncrement := 0.06, 0.02
	view, err = svc.AdminUpdateResourceRequestRate(
		ctx, approved.ID, &configuredRate, &adminIncrement,
	)
	require.NoError(t, err)
	require.InDelta(t, 0.06, view.RateMultiplier, 0.000001)
	require.InDelta(t, 0.02, view.AdminRateAdjustment, 0.000001)

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
	require.Equal(t, "probe", view.RateSource)
	require.InDelta(t, 0.07, view.AppliedRateMultiplier, 0.000001)
	require.InDelta(t, 0.09, view.EffectiveRateMultiplier, 0.000001)

	view, err = svc.UpdateResourceRequestProbe(ctx, owner.ID, approved.ID, false)
	require.NoError(t, err)
	require.Equal(t, "configured", view.RateSource)
	require.InDelta(t, 0.06, view.AppliedRateMultiplier, 0.000001)
	require.InDelta(t, 0.08, view.EffectiveRateMultiplier, 0.000001)
}
