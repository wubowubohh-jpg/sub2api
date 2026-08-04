package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/ent/channelmonitor"
	"github.com/Wei-Shaw/sub2api/ent/channelmonitorhistory"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/ent/supplier"
	"github.com/Wei-Shaw/sub2api/ent/supplierledger"
	"github.com/Wei-Shaw/sub2api/ent/suppliermetricbucket"
	"github.com/Wei-Shaw/sub2api/ent/supplierresourcerequest"
	"github.com/Wei-Shaw/sub2api/ent/supplierwithdrawal"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

type SupplierService struct {
	db                   *dbent.Client
	encryptor            SecretEncryptor
	authCacheInvalidator APIKeyAuthCacheInvalidator
	stop                 chan struct{}
	stopOnce             sync.Once
}

func NewSupplierService(db *dbent.Client, encryptor SecretEncryptor, authCacheInvalidator APIKeyAuthCacheInvalidator) *SupplierService {
	s := &SupplierService{
		db:                   db,
		encryptor:            encryptor,
		authCacheInvalidator: authCacheInvalidator,
		stop:                 make(chan struct{}),
	}
	go s.runSettlementWorker()
	return s
}

type SupplierResourceApplication struct {
	GroupName       string   `json:"group_name"`
	GroupNameSuffix string   `json:"group_name_suffix"`
	RelayName       string   `json:"relay_name"`
	RelayURL        string   `json:"relay_url"`
	APIKey          string   `json:"api_key"`
	Model           string   `json:"model"`
	ProbeModel      string   `json:"probe_model"`
	MonitorModel    string   `json:"monitor_model"`
	SupportedModels []string `json:"supported_models"`
	ProbeEnabled    *bool    `json:"upstream_billing_probe_enabled"`
	RateMultiplier  *float64 `json:"rate_multiplier"`
}

type SupplierResourceAdminUpdate struct {
	GroupName           string   `json:"group_name"`
	RelayName           string   `json:"relay_name"`
	RelayURL            string   `json:"relay_url"`
	APIKey              string   `json:"api_key"`
	MonitorModel        string   `json:"monitor_model"`
	SupportedModels     []string `json:"supported_models"`
	ProbeEnabled        bool     `json:"upstream_billing_probe_enabled"`
	RateMultiplier      float64  `json:"rate_multiplier"`
	AdminRateAdjustment *float64 `json:"admin_rate_adjustment"`
	ReviewNote          string   `json:"review_note"`
}

const supplierCredentialCipherPrefix = "enc:v1:"

type SupplierResourceProbeView struct {
	AccountID             int64   `json:"account_id"`
	Enabled               bool    `json:"enabled"`
	RateSyncEnabled       bool    `json:"rate_sync_enabled"`
	AccountRateMultiplier float64 `json:"account_rate_multiplier"`
	Snapshot              any     `json:"snapshot,omitempty"`
}

type SupplierResourceProbeSupplierView struct {
	Enabled  bool `json:"enabled"`
	Snapshot any  `json:"snapshot,omitempty"`
}

type SupplierResourceRequestView struct {
	*dbent.SupplierResourceRequest
	GroupNameSuffix             string                     `json:"group_name_suffix,omitempty"`
	MonitorModel                string                     `json:"monitor_model,omitempty"`
	RateSource                  string                     `json:"rate_source"`
	AppliedRateMultiplier       float64                    `json:"applied_rate_multiplier"`
	AdminRateAdjustment         float64                    `json:"admin_rate_adjustment"`
	EffectiveRateMultiplier     float64                    `json:"effective_rate_multiplier"`
	UpstreamBillingProbeEnabled bool                       `json:"upstream_billing_probe_enabled"`
	UpstreamProbeStatus         string                     `json:"upstream_probe_status,omitempty"`
	UpstreamRate                *float64                   `json:"upstream_rate,omitempty"`
	UpstreamRateUpdatedAt       *time.Time                 `json:"upstream_rate_updated_at,omitempty"`
	UpstreamProbeError          string                     `json:"upstream_probe_error,omitempty"`
	CredentialsNeedUpdate       bool                       `json:"credentials_need_update"`
	CredentialsValid            *bool                      `json:"credentials_valid,omitempty"`
	UpstreamBillingProbe        *SupplierResourceProbeView `json:"upstream_billing_probe,omitempty"`
}

// SupplierResourceRequestSupplierView is the intentionally limited resource
// application shape returned to suppliers. Internal ownership and reviewer
// identifiers are kept in the admin view only.
type SupplierResourceRequestSupplierView struct {
	ID                          int64                              `json:"id"`
	GroupName                   string                             `json:"group_name"`
	GroupNameSuffix             string                             `json:"group_name_suffix,omitempty"`
	RelayName                   string                             `json:"relay_name"`
	RelayURL                    string                             `json:"relay_url"`
	Model                       string                             `json:"model"`
	SupportedModels             []string                           `json:"supported_models,omitempty"`
	ProbeEnabled                bool                               `json:"probe_enabled"`
	RateMultiplier              float64                            `json:"rate_multiplier"`
	RateSource                  string                             `json:"rate_source"`
	AppliedRateMultiplier       float64                            `json:"applied_rate_multiplier"`
	AdminRateAdjustment         float64                            `json:"admin_rate_adjustment"`
	EffectiveRateMultiplier     float64                            `json:"effective_rate_multiplier"`
	Status                      string                             `json:"status"`
	ReviewNote                  string                             `json:"review_note,omitempty"`
	ReviewedAt                  *time.Time                         `json:"reviewed_at,omitempty"`
	UpstreamBillingProbeEnabled bool                               `json:"upstream_billing_probe_enabled"`
	UpstreamProbeStatus         string                             `json:"upstream_probe_status,omitempty"`
	UpstreamRate                *float64                           `json:"upstream_rate,omitempty"`
	UpstreamRateUpdatedAt       *time.Time                         `json:"upstream_rate_updated_at,omitempty"`
	UpstreamProbeError          string                             `json:"upstream_probe_error,omitempty"`
	CredentialsNeedUpdate       bool                               `json:"credentials_need_update"`
	CredentialsValid            *bool                              `json:"credentials_valid,omitempty"`
	UpstreamBillingProbe        *SupplierResourceProbeSupplierView `json:"upstream_billing_probe,omitempty"`
	CreatedAt                   time.Time                          `json:"created_at"`
}

func supplierResourceRequestForSupplier(view SupplierResourceRequestView) SupplierResourceRequestSupplierView {
	request := view.SupplierResourceRequest
	result := SupplierResourceRequestSupplierView{
		ID:                          view.ID,
		GroupName:                   view.GroupName,
		GroupNameSuffix:             view.GroupNameSuffix,
		RelayName:                   view.RelayName,
		RelayURL:                    view.RelayURL,
		Model:                       view.Model,
		SupportedModels:             append([]string(nil), view.SupportedModels...),
		ProbeEnabled:                view.ProbeEnabled,
		RateMultiplier:              view.RateMultiplier,
		RateSource:                  view.RateSource,
		AppliedRateMultiplier:       view.AppliedRateMultiplier,
		AdminRateAdjustment:         view.AdminRateAdjustment,
		EffectiveRateMultiplier:     view.EffectiveRateMultiplier,
		Status:                      string(view.Status),
		ReviewNote:                  view.ReviewNote,
		ReviewedAt:                  view.ReviewedAt,
		UpstreamBillingProbeEnabled: view.UpstreamBillingProbeEnabled,
		UpstreamProbeStatus:         view.UpstreamProbeStatus,
		UpstreamRate:                view.UpstreamRate,
		UpstreamRateUpdatedAt:       view.UpstreamRateUpdatedAt,
		UpstreamProbeError:          view.UpstreamProbeError,
		CredentialsNeedUpdate:       view.CredentialsNeedUpdate,
		CredentialsValid:            view.CredentialsValid,
	}
	if request != nil {
		result.CreatedAt = request.CreatedAt
	}
	if view.UpstreamBillingProbe != nil {
		result.UpstreamBillingProbe = &SupplierResourceProbeSupplierView{
			Enabled:  view.UpstreamBillingProbe.Enabled,
			Snapshot: view.UpstreamBillingProbe.Snapshot,
		}
	}
	return result
}

func (s *SupplierService) ResourceRequestsForSupplier(ctx context.Context, supplierID int64) ([]SupplierResourceRequestSupplierView, error) {
	views, err := s.ResourceRequests(ctx, &supplierID, "")
	if err != nil {
		return nil, err
	}
	result := make([]SupplierResourceRequestSupplierView, 0, len(views))
	for _, view := range views {
		result = append(result, supplierResourceRequestForSupplier(view))
	}
	return result, nil
}

func (s *SupplierService) ResourceRequestForSupplier(ctx context.Context, supplierID, requestID int64) (*SupplierResourceRequestSupplierView, error) {
	view, err := s.resourceRequestView(ctx, supplierID, requestID)
	if err != nil {
		return nil, err
	}
	result := supplierResourceRequestForSupplier(*view)
	return &result, nil
}

func (s *SupplierService) CreateResourceRequest(ctx context.Context, supplierID int64, in SupplierResourceApplication) (*dbent.SupplierResourceRequest, error) {
	if strings.TrimSpace(in.GroupNameSuffix) != "" {
		in.GroupName = in.GroupNameSuffix
	}
	in.GroupName, in.RelayName, in.RelayURL, in.APIKey = strings.TrimSpace(in.GroupName), strings.TrimSpace(in.RelayName), strings.TrimSpace(in.RelayURL), strings.TrimSpace(in.APIKey)
	requestedGroupName := in.GroupName
	normalizedURL, err := urlvalidator.ValidateHTTPSURL(in.RelayURL, urlvalidator.ValidationOptions{AllowPrivate: false})
	if in.GroupName == "" || in.RelayName == "" || in.APIKey == "" || err != nil {
		return nil, fmt.Errorf("invalid resource application")
	}
	in.RelayURL = normalizedURL
	sp, err := s.db.Supplier.Get(ctx, supplierID)
	if err != nil || sp.Status != supplier.StatusApproved {
		return nil, fmt.Errorf("supplier is not approved")
	}
	in.GroupName, err = supplierMarketplaceGroupName(supplierID, in.GroupName)
	if err != nil {
		return nil, err
	}
	existingPending, queryErr := s.db.SupplierResourceRequest.Query().Where(
		supplierresourcerequest.SupplierID(supplierID),
		supplierresourcerequest.GroupNameIn(in.GroupName, requestedGroupName),
		supplierresourcerequest.StatusEQ(supplierresourcerequest.StatusPending),
	).Only(ctx)
	if queryErr != nil && !dbent.IsNotFound(queryErr) {
		return nil, queryErr
	}
	if exists, queryErr := s.db.SupplierResourceRequest.Query().Where(
		supplierresourcerequest.SupplierID(supplierID),
		supplierresourcerequest.GroupNameIn(in.GroupName, requestedGroupName),
		supplierresourcerequest.StatusEQ(supplierresourcerequest.StatusApproved),
	).Exist(ctx); queryErr != nil {
		return nil, queryErr
	} else if exists {
		return nil, fmt.Errorf("supplier group name already exists")
	}
	if exists, queryErr := s.db.Group.Query().Where(group.NameEQ(in.GroupName)).Exist(ctx); queryErr != nil {
		return nil, queryErr
	} else if exists {
		return nil, fmt.Errorf("supplier group name already exists")
	}

	probeModel := strings.TrimSpace(in.ProbeModel)
	if probeModel == "" {
		probeModel = strings.TrimSpace(in.MonitorModel)
	}
	if probeModel == "" {
		probeModel = strings.TrimSpace(in.Model)
	}
	models, probeModel, err := normalizeSupplierResourceModels(in.SupportedModels, probeModel)
	if err != nil {
		return nil, err
	}
	probeEnabled := true
	if in.ProbeEnabled != nil {
		probeEnabled = *in.ProbeEnabled
	}
	rateMultiplier := 1.0
	if existingPending != nil {
		rateMultiplier = existingPending.RateMultiplier
	}
	if in.RateMultiplier != nil {
		rateMultiplier = *in.RateMultiplier
	}
	if rateMultiplier < 0 || math.IsNaN(rateMultiplier) || math.IsInf(rateMultiplier, 0) {
		return nil, fmt.Errorf("invalid rate multiplier")
	}
	encrypted, err := s.encryptSupplierAPIKey(in.APIKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}
	if existingPending != nil {
		return existingPending.Update().
			SetGroupName(in.GroupName).
			SetRelayName(in.RelayName).
			SetRelayURL(in.RelayURL).
			SetAPIKeyEncrypted(encrypted).
			SetModel(probeModel).
			SetSupportedModels(models).
			SetProbeEnabled(probeEnabled).
			SetRateMultiplier(rateMultiplier).
			Save(ctx)
	}
	return s.db.SupplierResourceRequest.Create().SetSupplierID(supplierID).SetGroupName(in.GroupName).SetRelayName(in.RelayName).SetRelayURL(in.RelayURL).SetAPIKeyEncrypted(encrypted).SetModel(probeModel).SetSupportedModels(models).SetProbeEnabled(probeEnabled).SetRateMultiplier(rateMultiplier).Save(ctx)
}

func (s *SupplierService) ResourceRequests(ctx context.Context, supplierID *int64, status string) ([]SupplierResourceRequestView, error) {
	q := s.db.SupplierResourceRequest.Query().Order(dbent.Desc(supplierresourcerequest.FieldCreatedAt))
	if supplierID != nil {
		q.Where(supplierresourcerequest.SupplierID(*supplierID))
	}
	if status != "" {
		q.Where(supplierresourcerequest.StatusEQ(supplierresourcerequest.Status(status)))
	}
	requests, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	accountIDs := make([]int64, 0, len(requests))
	groupIDs := make([]int64, 0, len(requests))
	for _, request := range requests {
		if request.AccountID != nil {
			accountIDs = append(accountIDs, *request.AccountID)
		}
		if request.GroupID != nil {
			groupIDs = append(groupIDs, *request.GroupID)
		}
	}
	groupsByID := make(map[int64]*dbent.Group, len(groupIDs))
	if len(groupIDs) > 0 {
		groups, groupErr := s.db.Group.Query().Where(group.IDIn(groupIDs...)).All(ctx)
		if groupErr != nil {
			return nil, groupErr
		}
		for _, item := range groups {
			groupsByID[item.ID] = item
		}
	}
	globalAdjustment := s.supplierGlobalRateAdjustment(ctx)
	accountsByID := make(map[int64]*dbent.Account, len(accountIDs))
	if len(accountIDs) > 0 {
		accounts, accountErr := s.db.Account.Query().Where(account.IDIn(accountIDs...)).All(ctx)
		if accountErr != nil {
			return nil, accountErr
		}
		for _, item := range accounts {
			accountsByID[item.ID] = item
		}
	}

	out := make([]SupplierResourceRequestView, 0, len(requests))
	for _, request := range requests {
		view := SupplierResourceRequestView{
			SupplierResourceRequest:     request,
			GroupNameSuffix:             supplierGroupNameSuffix(request.SupplierID, request.GroupName),
			MonitorModel:                request.Model,
			RateSource:                  "configured",
			AppliedRateMultiplier:       request.RateMultiplier,
			UpstreamBillingProbeEnabled: request.ProbeEnabled,
		}
		if request.AccountID == nil && request.ProbeEnabled {
			view.UpstreamProbeStatus = "pending"
		}
		if request.AccountID != nil {
			if item := accountsByID[*request.AccountID]; item != nil && item.SupplierID != nil && *item.SupplierID == request.SupplierID {
				probe := &SupplierResourceProbeView{AccountID: item.ID, AccountRateMultiplier: item.RateMultiplier}
				if item.Extra != nil {
					probe.Enabled, _ = item.Extra[UpstreamBillingProbeEnabledExtraKey].(bool)
					probe.RateSyncEnabled, _ = item.Extra[UpstreamBillingRateSyncEnabledExtraKey].(bool)
					probe.Snapshot = item.Extra[UpstreamBillingProbeExtraKey]
				}
				view.UpstreamBillingProbeEnabled = probe.Enabled
				view.UpstreamProbeStatus, view.UpstreamRate, view.UpstreamRateUpdatedAt, view.UpstreamProbeError, view.CredentialsValid = supplierProbeFields(probe)
				view.CredentialsNeedUpdate = view.CredentialsValid != nil && !*view.CredentialsValid
				view.UpstreamBillingProbe = probe
			}
		} else {
			view.UpstreamProbeStatus = "no_data"
		}
		view.AdminRateAdjustment = globalAdjustment
		configuredRate := request.RateMultiplier
		if request.GroupID != nil {
			if item := groupsByID[*request.GroupID]; item != nil && item.SupplierID != nil && *item.SupplierID == request.SupplierID {
				if item.SupplierAdminAdjustment != nil {
					view.AdminRateAdjustment = *item.SupplierAdminAdjustment
				}
			}
		}
		view.RateSource, view.AppliedRateMultiplier, view.EffectiveRateMultiplier = supplierResourceRateDetails(
			configuredRate,
			view.UpstreamBillingProbeEnabled,
			view.UpstreamRate,
			view.AdminRateAdjustment,
		)
		out = append(out, view)
	}
	return out, nil
}

func (s *SupplierService) resourceRequestView(ctx context.Context, supplierID, requestID int64) (*SupplierResourceRequestView, error) {
	views, err := s.ResourceRequests(ctx, &supplierID, "")
	if err != nil {
		return nil, err
	}
	for i := range views {
		if views[i].ID == requestID {
			return &views[i], nil
		}
	}
	return nil, fmt.Errorf("resource application not found")
}

// UpdateResourceRequestAPIKey lets a supplier replace a credential that was
// entered before a restart or that has become invalid. Pending applications
// remain pending; an already approved resource updates its own account and
// monitor atomically without exposing the credential.
func (s *SupplierService) UpdateResourceRequestAPIKey(ctx context.Context, supplierID, requestID int64, apiKey string) (*SupplierResourceRequestView, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	req, err := s.db.SupplierResourceRequest.Query().Where(supplierresourcerequest.ID(requestID), supplierresourcerequest.SupplierID(supplierID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("resource application not found")
	}
	encrypted, err := s.encryptSupplierAPIKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}
	if req.Status != supplierresourcerequest.StatusApproved {
		update := req.Update().SetAPIKeyEncrypted(encrypted)
		if req.Status == supplierresourcerequest.StatusRejected {
			update.SetStatus(supplierresourcerequest.StatusPending).SetReviewNote("").ClearReviewedBy().ClearReviewedAt()
		}
		if _, err = update.Save(ctx); err != nil {
			return nil, err
		}
		return s.resourceRequestView(ctx, supplierID, requestID)
	}
	if req.AccountID == nil || req.MonitorID == nil {
		return nil, fmt.Errorf("approved resource is missing its runtime resources")
	}
	models, _, modelErr := normalizeSupplierResourceModels(req.SupportedModels, req.Model)
	if modelErr != nil {
		return nil, modelErr
	}
	mapping := make(map[string]any, len(models))
	for _, model := range models {
		mapping[model] = model
	}
	monitorCiphertext, err := s.encryptor.Encrypt(apiKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt monitor api key: %w", err)
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	a, err := tx.Account.Query().Where(account.ID(*req.AccountID), account.SupplierID(supplierID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("supplier account not found")
	}
	credentials := shallowCopyMap(a.Credentials)
	if credentials == nil {
		credentials = make(map[string]any)
	}
	credentials["api_key"] = apiKey
	credentials["base_url"] = req.RelayURL
	credentials["model_mapping"] = mapping
	if _, err = tx.Account.UpdateOne(a).SetCredentials(credentials).SetStatus(StatusActive).SetSchedulable(true).Save(ctx); err != nil {
		return nil, err
	}
	monitor, err := tx.ChannelMonitor.Query().Where(channelmonitor.ID(*req.MonitorID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("supplier monitor not found")
	}
	if _, err = tx.ChannelMonitor.UpdateOne(monitor).SetAPIKeyEncrypted(monitorCiphertext).SetEnabled(true).Save(ctx); err != nil {
		return nil, err
	}
	if _, err = tx.SupplierResourceRequest.UpdateOneID(requestID).SetAPIKeyEncrypted(encrypted).Save(ctx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.resourceRequestView(ctx, supplierID, requestID)
}

// UpdateResourceRequestProbe changes only the owning supplier account's
// upstream billing probe switch. Enabling it never enables rate write-back.
func (s *SupplierService) UpdateResourceRequestProbe(ctx context.Context, supplierID, requestID int64, enabled bool) (*SupplierResourceRequestView, error) {
	req, err := s.db.SupplierResourceRequest.Query().Where(supplierresourcerequest.ID(requestID), supplierresourcerequest.SupplierID(supplierID)).Only(ctx)
	if err != nil || req.Status != supplierresourcerequest.StatusApproved || req.AccountID == nil {
		return nil, fmt.Errorf("approved resource application not found")
	}
	a, err := s.db.Account.Query().Where(account.ID(*req.AccountID), account.SupplierID(supplierID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("supplier account not found")
	}
	extra := shallowCopyMap(a.Extra)
	if extra == nil {
		extra = make(map[string]any)
	}
	extra[UpstreamBillingProbeEnabledExtraKey] = enabled
	if !enabled {
		extra[UpstreamBillingRateSyncEnabledExtraKey] = false
	}
	if _, err = a.Update().SetExtra(extra).Save(ctx); err != nil {
		return nil, err
	}
	return s.resourceRequestView(ctx, supplierID, requestID)
}

func validSupplierRate(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// UpdateResourceRequestRate updates the supplier-controlled rate. Approved
// resources update their linked billing group in the same transaction.
func (s *SupplierService) UpdateResourceRequestRate(ctx context.Context, supplierID, requestID int64, rate float64) (*SupplierResourceRequestView, error) {
	return s.updateResourceRequestRate(ctx, &supplierID, requestID, &rate, nil)
}

// AdminUpdateResourceRequestRate updates the supplier rate and the per-group
// administrator increment in one transaction.
func (s *SupplierService) AdminUpdateResourceRequestRate(ctx context.Context, requestID int64, rate, adjustment *float64) (*SupplierResourceRequestView, error) {
	return s.updateResourceRequestRate(ctx, nil, requestID, rate, adjustment)
}

// AdminUpdateResourceRequest updates all administrator-editable resource
// fields. Approved resources keep their request, group, account and monitor in
// sync in one transaction.
func (s *SupplierService) AdminUpdateResourceRequest(ctx context.Context, requestID int64, in SupplierResourceAdminUpdate) (*SupplierResourceRequestView, error) {
	req, err := s.db.SupplierResourceRequest.Get(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("resource application not found")
	}

	groupName, err := supplierMarketplaceGroupName(req.SupplierID, in.GroupName)
	if err != nil {
		return nil, err
	}
	relayName := strings.TrimSpace(in.RelayName)
	if relayName == "" || len([]rune(relayName)) > 100 {
		return nil, fmt.Errorf("invalid relay name")
	}
	relayURL, err := urlvalidator.ValidateHTTPSURL(strings.TrimSpace(in.RelayURL), urlvalidator.ValidationOptions{AllowPrivate: false})
	if err != nil {
		return nil, fmt.Errorf("invalid relay url")
	}
	models, monitorModel, err := normalizeSupplierResourceModels(in.SupportedModels, in.MonitorModel)
	if err != nil {
		return nil, err
	}
	if !validSupplierRate(in.RateMultiplier) {
		return nil, fmt.Errorf("invalid supplier rate")
	}
	if in.AdminRateAdjustment != nil && !validSupplierRate(*in.AdminRateAdjustment) {
		return nil, fmt.Errorf("invalid administrator rate increment")
	}
	if req.GroupID == nil && in.AdminRateAdjustment != nil {
		return nil, fmt.Errorf("administrator rate increment requires an approved resource")
	}

	groupQuery := s.db.Group.Query().Where(group.NameEQ(groupName))
	if req.GroupID != nil {
		groupQuery.Where(group.IDNEQ(*req.GroupID))
	}
	if exists, queryErr := groupQuery.Exist(ctx); queryErr != nil {
		return nil, queryErr
	} else if exists {
		return nil, fmt.Errorf("supplier group name already exists")
	}
	if exists, queryErr := s.db.SupplierResourceRequest.Query().Where(
		supplierresourcerequest.IDNEQ(req.ID),
		supplierresourcerequest.GroupNameEQ(groupName),
		supplierresourcerequest.StatusIn(supplierresourcerequest.StatusPending, supplierresourcerequest.StatusApproved),
	).Exist(ctx); queryErr != nil {
		return nil, queryErr
	} else if exists {
		return nil, fmt.Errorf("supplier group name already exists")
	}

	apiKey := strings.TrimSpace(in.APIKey)
	var requestCiphertext, monitorCiphertext string
	if apiKey != "" {
		requestCiphertext, err = s.encryptSupplierAPIKey(apiKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt api key: %w", err)
		}
		if req.MonitorID != nil {
			if s.encryptor == nil {
				return nil, fmt.Errorf("supplier credential encryption is not configured")
			}
			monitorCiphertext, err = s.encryptor.Encrypt(apiKey)
			if err != nil {
				return nil, fmt.Errorf("encrypt monitor api key: %w", err)
			}
		}
	}

	modelMapping := make(map[string]any, len(models))
	extraModels := make([]string, 0, len(models)-1)
	for _, model := range models {
		modelMapping[model] = model
		if model != monitorModel {
			extraModels = append(extraModels, model)
		}
	}

	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	requestUpdate := tx.SupplierResourceRequest.UpdateOneID(req.ID).
		SetGroupName(groupName).
		SetRelayName(relayName).
		SetRelayURL(relayURL).
		SetModel(monitorModel).
		SetSupportedModels(models).
		SetProbeEnabled(in.ProbeEnabled).
		SetRateMultiplier(in.RateMultiplier).
		SetReviewNote(strings.TrimSpace(in.ReviewNote))
	if requestCiphertext != "" {
		requestUpdate.SetAPIKeyEncrypted(requestCiphertext)
	}
	if _, err = requestUpdate.Save(ctx); err != nil {
		return nil, err
	}

	if req.GroupID != nil {
		g, groupErr := tx.Group.Query().Where(group.ID(*req.GroupID), group.SupplierID(req.SupplierID)).Only(ctx)
		if groupErr != nil {
			return nil, fmt.Errorf("supplier group not found")
		}
		adminAdjustment := s.supplierGlobalRateAdjustment(ctx)
		if g.SupplierAdminAdjustment != nil {
			adminAdjustment = *g.SupplierAdminAdjustment
		}
		if in.AdminRateAdjustment != nil {
			adminAdjustment = *in.AdminRateAdjustment
		}
		finalRate := in.RateMultiplier + adminAdjustment
		groupUpdate := tx.Group.UpdateOne(g).
			SetName(groupName).
			SetDescription(relayName).
			SetRateMultiplier(finalRate)
		if in.AdminRateAdjustment != nil {
			groupUpdate.SetSupplierAdminAdjustment(*in.AdminRateAdjustment)
		}
		if _, err = groupUpdate.Save(ctx); err != nil {
			return nil, err
		}

		if req.AccountID == nil || req.MonitorID == nil {
			return nil, fmt.Errorf("approved resource is missing its runtime resources")
		}
		a, accountErr := tx.Account.Query().Where(account.ID(*req.AccountID), account.SupplierID(req.SupplierID)).Only(ctx)
		if accountErr != nil {
			return nil, fmt.Errorf("supplier account not found")
		}
		credentials := shallowCopyMap(a.Credentials)
		if credentials == nil {
			credentials = make(map[string]any)
		}
		credentials["base_url"] = relayURL
		credentials["model_mapping"] = modelMapping
		if apiKey != "" {
			credentials["api_key"] = apiKey
		}
		extra := shallowCopyMap(a.Extra)
		if extra == nil {
			extra = make(map[string]any)
		}
		extra[UpstreamBillingProbeEnabledExtraKey] = in.ProbeEnabled
		if !in.ProbeEnabled {
			extra[UpstreamBillingRateSyncEnabledExtraKey] = false
		}
		if _, err = tx.Account.UpdateOne(a).SetName(relayName).SetCredentials(credentials).SetExtra(extra).SetRateMultiplier(finalRate).Save(ctx); err != nil {
			return nil, err
		}

		monitor, monitorErr := tx.ChannelMonitor.Query().Where(channelmonitor.ID(*req.MonitorID), channelmonitor.GroupID(*req.GroupID)).Only(ctx)
		if monitorErr != nil {
			return nil, fmt.Errorf("supplier monitor not found")
		}
		monitorUpdate := tx.ChannelMonitor.UpdateOne(monitor).
			SetName(relayName).
			SetEndpoint(relayURL).
			SetPrimaryModel(monitorModel).
			SetExtraModels(extraModels).
			SetGroupName(groupName)
		if monitorCiphertext != "" {
			monitorUpdate.SetAPIKeyEncrypted(monitorCiphertext)
		}
		if _, err = monitorUpdate.Save(ctx); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if req.GroupID != nil && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, *req.GroupID)
	}
	return s.resourceRequestView(ctx, req.SupplierID, req.ID)
}

func (s *SupplierService) updateResourceRequestRate(ctx context.Context, supplierID *int64, requestID int64, rate, adjustment *float64) (*SupplierResourceRequestView, error) {
	if rate == nil && adjustment == nil {
		return nil, fmt.Errorf("rate update is required")
	}
	if rate != nil && !validSupplierRate(*rate) {
		return nil, fmt.Errorf("invalid supplier rate")
	}
	if adjustment != nil && !validSupplierRate(*adjustment) {
		return nil, fmt.Errorf("invalid administrator rate increment")
	}
	query := s.db.SupplierResourceRequest.Query().Where(supplierresourcerequest.ID(requestID))
	if supplierID != nil {
		query.Where(supplierresourcerequest.SupplierID(*supplierID))
	}
	req, err := query.Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("resource application not found")
	}
	if adjustment != nil && req.GroupID == nil {
		return nil, fmt.Errorf("administrator rate increment requires an approved resource")
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	requestUpdate := tx.SupplierResourceRequest.UpdateOneID(req.ID)
	if rate != nil {
		requestUpdate.SetRateMultiplier(*rate)
	}
	if _, err = requestUpdate.Save(ctx); err != nil {
		return nil, err
	}
	if req.GroupID != nil {
		g, groupErr := tx.Group.Query().Where(
			group.ID(*req.GroupID), group.SupplierID(req.SupplierID),
		).Only(ctx)
		if groupErr != nil {
			return nil, fmt.Errorf("supplier group not found")
		}
		configuredRate := req.RateMultiplier
		if rate != nil {
			configuredRate = *rate
		}
		adminAdjustment := s.supplierGlobalRateAdjustment(ctx)
		if g.SupplierAdminAdjustment != nil {
			adminAdjustment = *g.SupplierAdminAdjustment
		}
		if adjustment != nil {
			adminAdjustment = *adjustment
		}
		finalRate := configuredRate + adminAdjustment
		groupUpdate := tx.Group.UpdateOne(g).SetRateMultiplier(finalRate)
		if adjustment != nil {
			groupUpdate.SetSupplierAdminAdjustment(*adjustment)
		}
		if _, err = groupUpdate.Save(ctx); err != nil {
			return nil, err
		}
		if req.AccountID != nil {
			if _, err = tx.Account.UpdateOneID(*req.AccountID).SetRateMultiplier(finalRate).Save(ctx); err != nil {
				return nil, err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if req.GroupID != nil && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, *req.GroupID)
	}
	return s.resourceRequestView(ctx, req.SupplierID, req.ID)
}

func supplierMarketplaceGroupName(supplierID int64, suffix string) (string, error) {
	prefix := fmt.Sprintf("A%04d-", supplierID)
	suffix = strings.TrimSpace(suffix)
	if strings.HasPrefix(suffix, prefix) {
		suffix = strings.TrimSpace(strings.TrimPrefix(suffix, prefix))
	}
	suffix = strings.TrimLeft(suffix, "-")
	if suffix == "" || strings.ContainsAny(suffix, "\r\n\t") {
		return "", fmt.Errorf("group name suffix is required")
	}
	name := prefix + suffix
	if len([]rune(name)) > 100 {
		return "", fmt.Errorf("supplier group name must not exceed 100 characters")
	}
	return name, nil
}

func supplierGroupNameSuffix(supplierID int64, name string) string {
	prefix := fmt.Sprintf("A%04d-", supplierID)
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return name
}

func supplierProbeFields(probe *SupplierResourceProbeView) (status string, rate *float64, updatedAt *time.Time, errText string, credentialsValid *bool) {
	if probe == nil || !probe.Enabled {
		return "disabled", nil, nil, "", nil
	}
	snapshot, ok := probe.Snapshot.(map[string]any)
	if !ok || len(snapshot) == 0 {
		return "pending", nil, nil, "", nil
	}
	switch value, _ := snapshot["status"].(string); value {
	case UpstreamBillingProbeStatusOK:
		status = "available"
	case UpstreamBillingProbeStatusUnsupported:
		status = "no_data"
	case UpstreamBillingProbeStatusFailed:
		status = "failed"
	default:
		status = "pending"
	}
	if value, ok := snapshot["last_error"].(string); ok {
		errText = value
	}
	if value, ok := snapshot["received_at"].(string); ok {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, value); parseErr == nil {
			updatedAt = &parsed
		}
	}
	if data, ok := snapshot["data"].(map[string]any); ok {
		for _, key := range []string{"effective_rate_multiplier", "resolved_rate_multiplier"} {
			if value, ok := supplierProbeNumber(data[key]); ok {
				rate = &value
				break
			}
		}
	}
	if httpStatus, ok := supplierProbeNumber(snapshot["http_status"]); ok && (httpStatus == 401 || httpStatus == 403) {
		valid := false
		credentialsValid = &valid
		status = "credential_invalid"
	} else if status == "available" {
		valid := true
		credentialsValid = &valid
	}
	return status, rate, updatedAt, errText, credentialsValid
}

func supplierProbeNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	default:
		return 0, false
	}
}

// Probe results are informational. Billing always uses the supplier-controlled
// configured rate plus the administrator adjustment.
func supplierResourceRateDetails(configuredRate float64, _ bool, _ *float64, adminAdjustment float64) (string, float64, float64) {
	return "configured", configuredRate, configuredRate + adminAdjustment
}

func normalizeSupplierResourceModels(models []string, probeModel string) ([]string, string, error) {
	probeModel = strings.TrimSpace(probeModel)
	if probeModel == "" {
		probeModel = "gpt-5.5"
	}
	if !validSupplierResourceModel(probeModel) {
		return nil, "", fmt.Errorf("invalid default probe model")
	}
	if len(models) == 0 {
		models = []string{probeModel}
	}
	if len(models) > 100 {
		return nil, "", fmt.Errorf("too many supported models")
	}
	seen := make(map[string]struct{}, len(models)+1)
	out := make([]string, 0, len(models)+1)
	for _, model := range models {
		model = strings.TrimSpace(model)
		if !validSupplierResourceModel(model) {
			return nil, "", fmt.Errorf("invalid supported model")
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	if _, ok := seen[probeModel]; !ok {
		return nil, "", fmt.Errorf("default probe model must be included in supported models")
	}
	return out, probeModel, nil
}

func validSupplierResourceModel(model string) bool {
	if model == "" || len([]rune(model)) > 200 {
		return false
	}
	for _, r := range model {
		if r <= 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func (s *SupplierService) encryptSupplierAPIKey(apiKey string) (string, error) {
	if s.encryptor == nil {
		return "", fmt.Errorf("supplier credential encryption is not configured")
	}
	ciphertext, err := s.encryptor.Encrypt(strings.TrimSpace(apiKey))
	if err != nil {
		return "", err
	}
	return supplierCredentialCipherPrefix + ciphertext, nil
}

func (s *SupplierService) decryptSupplierAPIKey(stored string) (string, error) {
	if s.encryptor == nil {
		return "", fmt.Errorf("supplier credential encryption is not configured")
	}
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", fmt.Errorf("resource application api key is empty")
	}
	ciphertext := stored
	versioned := strings.HasPrefix(stored, supplierCredentialCipherPrefix)
	if versioned {
		ciphertext = strings.TrimPrefix(stored, supplierCredentialCipherPrefix)
	}
	plaintext, err := s.encryptor.Decrypt(ciphertext)
	if err == nil && strings.TrimSpace(plaintext) != "" {
		return strings.TrimSpace(plaintext), nil
	}
	if !versioned && legacyPlainSupplierAPIKey(stored) {
		return stored, nil
	}
	if err == nil {
		err = fmt.Errorf("decrypted api key is empty")
	}
	return "", err
}

func legacyPlainSupplierAPIKey(value string) bool {
	if len(value) < 8 || len(value) > 2048 || (!strings.HasPrefix(value, "sk-") && !strings.HasPrefix(value, "sk_")) {
		return false
	}
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// BuildResourceRequestTestAccount resolves an application into an in-memory
// account for an admin connectivity probe. The decrypted key stays inside the
// service/handler call chain and is never serialized in an API response.
func (s *SupplierService) BuildResourceRequestTestAccount(ctx context.Context, requestID int64) (*Account, string, error) {
	req, err := s.db.SupplierResourceRequest.Get(ctx, requestID)
	if err != nil {
		return nil, "", fmt.Errorf("resource application not found")
	}
	if req.Status != supplierresourcerequest.StatusPending {
		return nil, "", fmt.Errorf("resource application is not pending")
	}
	if s.encryptor == nil {
		return nil, "", fmt.Errorf("supplier credential encryption is not configured")
	}
	key, err := s.decryptSupplierAPIKey(req.APIKeyEncrypted)
	if err != nil {
		return nil, "", fmt.Errorf("API Key 密文已失效，请供应商在资源申请记录中更新 API Key 后重试")
	}
	if strings.TrimSpace(key) == "" {
		return nil, "", fmt.Errorf("resource application api key is empty")
	}

	models, model, modelErr := normalizeSupplierResourceModels(req.SupportedModels, req.Model)
	if modelErr != nil {
		return nil, "", modelErr
	}
	modelMapping := make(map[string]any, len(models))
	for _, supportedModel := range models {
		modelMapping[supportedModel] = supportedModel
	}
	return &Account{
		// Ent IDs are positive. A request-scoped negative ID prevents failed
		// probes from updating the runtime state of an unrelated real account.
		ID:          -req.ID,
		Name:        req.RelayName,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: false,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":       key,
			"base_url":      req.RelayURL,
			"model_mapping": modelMapping,
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported: false,
			"transient_account_test":                 true,
		},
	}, model, nil
}

func (s *SupplierService) ReviewResourceRequest(ctx context.Context, requestID, reviewerID int64, approved bool, note string) (*dbent.SupplierResourceRequest, error) {
	req, err := s.db.SupplierResourceRequest.Get(ctx, requestID)
	if err != nil || req.Status != supplierresourcerequest.StatusPending {
		return nil, fmt.Errorf("pending resource application not found")
	}
	if !approved {
		return req.Update().SetStatus(supplierresourcerequest.StatusRejected).SetReviewedBy(reviewerID).SetReviewedAt(time.Now()).SetReviewNote(note).Save(ctx)
	}
	sp, err := s.db.Supplier.Get(ctx, req.SupplierID)
	if err != nil || sp.Status != supplier.StatusApproved {
		return nil, fmt.Errorf("supplier must be approved before creating resources")
	}
	key, err := s.decryptSupplierAPIKey(req.APIKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("API Key 密文已失效，请供应商在资源申请记录中更新 API Key 后重试")
	}
	groupName, err := supplierMarketplaceGroupName(req.SupplierID, req.GroupName)
	if err != nil {
		return nil, err
	}
	models, probeModel, err := normalizeSupplierResourceModels(req.SupportedModels, req.Model)
	if err != nil {
		return nil, err
	}
	modelMapping := make(map[string]any, len(models))
	for _, model := range models {
		modelMapping[model] = model
	}
	monitorCiphertext, err := s.encryptor.Encrypt(key)
	if err != nil {
		return nil, fmt.Errorf("encrypt monitor api key: %w", err)
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	adminAdjustment := s.supplierGlobalRateAdjustment(ctx)
	finalRate := req.RateMultiplier + adminAdjustment
	g, err := tx.Group.Create().
		SetSupplierID(req.SupplierID).
		SetName(groupName).
		SetDescription(req.RelayName).
		SetPlatform("openai").
		SetSubscriptionType("standard").
		SetRateMultiplier(finalRate).
		SetStatus("active").
		SetIsExclusive(false).
		SetRpmLimit(0).
		SetMaxReasoningEffort("").
		Save(ctx)
	if err != nil {
		return nil, err
	}
	a, err := tx.Account.Create().SetSupplierID(req.SupplierID).SetName(req.RelayName).SetPlatform(PlatformOpenAI).SetType(AccountTypeAPIKey).SetCredentials(map[string]any{"api_key": key, "base_url": req.RelayURL, "model_mapping": modelMapping}).SetExtra(map[string]any{openai_compat.ExtraKeyResponsesSupported: false, UpstreamBillingProbeEnabledExtraKey: req.ProbeEnabled, UpstreamBillingRateSyncEnabledExtraKey: false}).SetRateMultiplier(finalRate).SetStatus(StatusActive).SetSchedulable(true).AddGroupIDs(g.ID).Save(ctx)
	if err != nil {
		return nil, err
	}
	extraModels := make([]string, 0, len(models)-1)
	for _, model := range models {
		if model != probeModel {
			extraModels = append(extraModels, model)
		}
	}
	m, err := tx.ChannelMonitor.Create().SetName(req.RelayName).SetProvider("openai").SetAPIMode("chat_completions").SetEndpoint(req.RelayURL).SetAPIKeyEncrypted(monitorCiphertext).SetPrimaryModel(probeModel).SetExtraModels(extraModels).SetGroupName(groupName).SetGroupID(g.ID).SetEnabled(true).SetIntervalSeconds(60).SetJitterSeconds(5).SetCreatedBy(reviewerID).Save(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := tx.SupplierResourceRequest.UpdateOneID(req.ID).SetGroupName(groupName).SetModel(probeModel).SetSupportedModels(models).SetStatus(supplierresourcerequest.StatusApproved).SetReviewedBy(reviewerID).SetReviewedAt(time.Now()).SetReviewNote(note).SetGroupID(g.ID).SetAccountID(a.ID).SetMonitorID(m.ID).Save(ctx)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}
func (s *SupplierService) runSettlementWorker() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			_, _ = s.ReconcileUsage(ctx, 1000)
			_, _ = s.ReleaseDue(ctx, time.Now())
			cancel()
		case <-s.stop:
			return
		}
	}
}
func (s *SupplierService) Stop() {
	if s != nil {
		s.stopOnce.Do(func() { close(s.stop) })
	}
}

type SupplierApplication struct {
	Name            string  `json:"name"`
	RelayURL        string  `json:"relay_url"`
	ApplicationNote string  `json:"application_note"`
	DocumentIDs     []int64 `json:"document_ids"`
}

type SupplierGroupInput struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	Platform             string   `json:"platform"`
	SubscriptionType     string   `json:"subscription_type"`
	RateMultiplier       float64  `json:"rate_multiplier"`
	IsExclusive          bool     `json:"is_exclusive"`
	Status               string   `json:"status"`
	SortOrder            int      `json:"sort_order"`
	SupportedModelScopes []string `json:"supported_model_scopes"`
}
type SupplierAccountInput struct {
	Name            string         `json:"name"`
	Platform        string         `json:"platform"`
	Type            string         `json:"type"`
	Credentials     map[string]any `json:"credentials"`
	Extra           map[string]any `json:"extra"`
	Concurrency     int            `json:"concurrency"`
	Priority        int            `json:"priority"`
	RateMultiplier  float64        `json:"rate_multiplier"`
	Status          string         `json:"status"`
	Schedulable     bool           `json:"schedulable"`
	GroupIDs        []int64        `json:"group_ids"`
	ParentAccountID *int64         `json:"parent_account_id"`
}

type SupplierMetricPoint struct {
	At              time.Time `json:"at"`
	Requests        int64     `json:"requests"`
	AvgLatencyMs    *float64  `json:"avg_latency_ms"`
	AvgFirstTokenMs *float64  `json:"avg_first_token_ms"`
	CacheHitRate    *float64  `json:"cache_hit_rate"`
	TPS             *float64  `json:"tps"`
}
type SupplierGroupMetrics struct {
	RequestCount    int64                 `json:"request_count"`
	AvgLatencyMs    *float64              `json:"avg_latency_ms"`
	AvgFirstTokenMs *float64              `json:"avg_first_token_ms"`
	ProbeLatencyMs  *float64              `json:"probe_latency_ms"`
	CacheHitRate    *float64              `json:"cache_hit_rate"`
	TPS             *float64              `json:"tps"`
	Availability    *float64              `json:"availability"`
	LatestProbeAt   *time.Time            `json:"latest_probe_at"`
	Timeline        []SupplierMetricPoint `json:"timeline"`
}

type SupplierSettings struct {
	GlobalRateAdjustment float64 `json:"global_rate_adjustment"`
	SettlementDelayDays  int     `json:"settlement_delay_days"`
}

type SupplierBillItem struct {
	ID              int64      `json:"id"`
	GroupID         int64      `json:"group_id"`
	GroupName       string     `json:"group_name"`
	Model           string     `json:"model"`
	InputTokens     int        `json:"input_tokens"`
	OutputTokens    int        `json:"output_tokens"`
	CacheReadTokens int        `json:"cache_read_tokens"`
	BaseRate        float64    `json:"base_rate"`
	EffectiveRate   float64    `json:"effective_rate"`
	AmountCNY       float64    `json:"amount_cny"`
	Status          string     `json:"status"`
	AvailableAt     *time.Time `json:"available_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

func (s *SupplierService) Bills(ctx context.Context, supplierID int64, bucket string, limit int) ([]SupplierBillItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := s.db.SupplierLedger.Query().Where(supplierledger.SupplierID(supplierID)).Order(dbent.Desc(supplierledger.FieldCreatedAt)).Limit(limit)
	if bucket != "" {
		if bucket != "pending" && bucket != "available" && bucket != "frozen" {
			return nil, fmt.Errorf("invalid bill status")
		}
		q.Where(supplierledger.BucketEQ(supplierledger.Bucket(bucket)))
	}
	entries, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SupplierBillItem, 0, len(entries))
	for _, entry := range entries {
		item := SupplierBillItem{ID: entry.ID, GroupID: entry.GroupID, BaseRate: entry.BaseRate, EffectiveRate: entry.EffectiveRate, AmountCNY: entry.AmountCny, Status: string(entry.Bucket), AvailableAt: entry.AvailableAt, CreatedAt: entry.CreatedAt}
		if g, err := s.db.Group.Get(ctx, entry.GroupID); err == nil && g.SupplierID != nil && *g.SupplierID == supplierID {
			item.GroupName = g.Name
		}
		if entry.UsageLogID != nil {
			if log, err := s.db.UsageLog.Get(ctx, *entry.UsageLogID); err == nil && log.SupplierID != nil && *log.SupplierID == supplierID {
				item.Model = log.Model
				item.InputTokens = log.InputTokens
				item.OutputTokens = log.OutputTokens
				item.CacheReadTokens = log.CacheReadTokens
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *SupplierService) GetSettings(ctx context.Context) (SupplierSettings, error) {
	out := SupplierSettings{SettlementDelayDays: 7}
	rows, err := s.db.Setting.Query().Where(setting.KeyIn(SettingKeySupplierGlobalRateAdjustment, SettingKeySupplierSettlementDelayDays)).All(ctx)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		if row.Key == SettingKeySupplierGlobalRateAdjustment {
			v, parseErr := strconv.ParseFloat(strings.TrimSpace(row.Value), 64)
			if parseErr == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
				out.GlobalRateAdjustment = v
			}
		} else if row.Key == SettingKeySupplierSettlementDelayDays {
			if v, parseErr := strconv.Atoi(strings.TrimSpace(row.Value)); parseErr == nil && v >= 0 && v <= 365 {
				out.SettlementDelayDays = v
			}
		}
	}
	return out, nil
}

func (s *SupplierService) UpdateSettings(ctx context.Context, in SupplierSettings) (SupplierSettings, error) {
	if math.IsNaN(in.GlobalRateAdjustment) || math.IsInf(in.GlobalRateAdjustment, 0) || in.SettlementDelayDays < 0 || in.SettlementDelayDays > 365 {
		return SupplierSettings{}, fmt.Errorf("invalid supplier settings")
	}
	values := map[string]string{
		SettingKeySupplierGlobalRateAdjustment: strconv.FormatFloat(in.GlobalRateAdjustment, 'f', -1, 64),
		SettingKeySupplierSettlementDelayDays:  strconv.Itoa(in.SettlementDelayDays),
	}
	for key, formatted := range values {
		row, err := s.db.Setting.Query().Where(setting.KeyEQ(key)).Only(ctx)
		if dbent.IsNotFound(err) {
			if _, err = s.db.Setting.Create().SetKey(key).SetValue(formatted).Save(ctx); err != nil {
				return SupplierSettings{}, err
			}
		} else if err != nil {
			return SupplierSettings{}, err
		} else if _, err = row.Update().SetValue(formatted).Save(ctx); err != nil {
			return SupplierSettings{}, err
		}
	}
	if err := s.syncGlobalAdjustedResourceRates(ctx, in.GlobalRateAdjustment); err != nil {
		return SupplierSettings{}, err
	}
	return s.GetSettings(ctx)
}

func (s *SupplierService) syncGlobalAdjustedResourceRates(ctx context.Context, adjustment float64) error {
	requests, err := s.db.SupplierResourceRequest.Query().
		Where(
			supplierresourcerequest.StatusEQ(supplierresourcerequest.StatusApproved),
			supplierresourcerequest.GroupIDNotNil(),
		).
		All(ctx)
	if err != nil {
		return err
	}
	for _, request := range requests {
		g, groupErr := s.db.Group.Get(ctx, *request.GroupID)
		if groupErr != nil || g.SupplierID == nil || g.SupplierAdminAdjustment != nil {
			continue
		}
		finalRate := request.RateMultiplier + adjustment
		if _, err = g.Update().SetRateMultiplier(finalRate).Save(ctx); err != nil {
			return err
		}
		if request.AccountID != nil {
			if _, err = s.db.Account.UpdateOneID(*request.AccountID).SetRateMultiplier(finalRate).Save(ctx); err != nil {
				return err
			}
		}
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, g.ID)
		}
	}
	return nil
}

func (s *SupplierService) AddDocument(ctx context.Context, supplierID int64, storageKey, originalName, contentType string, size int64) (*dbent.SupplierDocument, error) {
	return s.db.SupplierDocument.Create().SetSupplierID(supplierID).SetStorageKey(storageKey).SetOriginalName(originalName).SetContentType(contentType).SetSizeBytes(size).Save(ctx)
}

func (s *SupplierService) GroupMetrics(ctx context.Context, groupID int64, since time.Time) (SupplierGroupMetrics, error) {
	rows, err := s.db.SupplierMetricBucket.Query().Where(suppliermetricbucket.GroupID(groupID), suppliermetricbucket.ResolutionEQ(suppliermetricbucket.Resolution5m), suppliermetricbucket.BucketStartGTE(since)).Order(dbent.Asc(suppliermetricbucket.FieldBucketStart)).All(ctx)
	if err != nil {
		return SupplierGroupMetrics{}, err
	}
	out := SupplierGroupMetrics{Timeline: make([]SupplierMetricPoint, 0, len(rows))}
	var duration, tokens, input, cache int64
	for _, r := range rows {
		out.RequestCount += r.RequestCount
		duration += r.DurationMsSum
		tokens += r.TotalTokens
		input += r.InputTokens
		cache += r.CacheReadTokens
		p := SupplierMetricPoint{At: r.BucketStart, Requests: r.RequestCount}
		if r.RequestCount > 0 {
			v := float64(r.DurationMsSum) / float64(r.RequestCount)
			p.AvgLatencyMs = &v
		}
		if r.FirstTokenCount > 0 {
			v := float64(r.FirstTokenMsSum) / float64(r.FirstTokenCount)
			p.AvgFirstTokenMs = &v
		}
		if r.InputTokens+r.CacheReadTokens > 0 {
			v := float64(r.CacheReadTokens) / float64(r.InputTokens+r.CacheReadTokens) * 100
			p.CacheHitRate = &v
		}
		if r.DurationMsSum > 0 {
			v := float64(r.TotalTokens) * 1000 / float64(r.DurationMsSum)
			p.TPS = &v
		}
		out.Timeline = append(out.Timeline, p)
	}
	if out.RequestCount > 0 {
		v := float64(duration) / float64(out.RequestCount)
		out.AvgLatencyMs = &v
	}
	if input+cache > 0 {
		v := float64(cache) / float64(input+cache) * 100
		out.CacheHitRate = &v
	}
	if duration > 0 {
		v := float64(tokens) * 1000 / float64(duration)
		out.TPS = &v
	}
	// Merge active probe data. New monitors use group_id; legacy admin monitors
	// are matched by the exact group name.
	if g, groupErr := s.db.Group.Get(ctx, groupID); groupErr == nil {
		monitors, _ := s.db.ChannelMonitor.Query().Where(channelmonitor.Enabled(true), channelmonitor.Or(channelmonitor.GroupID(groupID), channelmonitor.GroupNameEQ(g.Name))).All(ctx)
		var checks, healthy, latencySum, latencyCount, probeFirstSum, probeFirstCount int64
		for _, monitor := range monitors {
			history, _ := s.db.ChannelMonitorHistory.Query().Where(channelmonitorhistory.MonitorID(monitor.ID), channelmonitorhistory.CheckedAtGTE(since)).All(ctx)
			for _, point := range history {
				if out.LatestProbeAt == nil || point.CheckedAt.After(*out.LatestProbeAt) {
					checkedAt := point.CheckedAt
					out.LatestProbeAt = &checkedAt
				}
				checks++
				if point.Status == channelmonitorhistory.StatusOperational || point.Status == channelmonitorhistory.StatusDegraded {
					healthy++
				}
				if point.LatencyMs != nil {
					latencySum += int64(*point.LatencyMs)
					latencyCount++
				}
				if point.FirstTokenMs != nil {
					probeFirstSum += int64(*point.FirstTokenMs)
					probeFirstCount++
				}
			}
		}
		if checks > 0 {
			v := float64(healthy) / float64(checks) * 100
			out.Availability = &v
		}
		if latencyCount > 0 {
			v := float64(latencySum) / float64(latencyCount)
			out.ProbeLatencyMs = &v
		}
		if probeFirstCount > 0 {
			v := float64(probeFirstSum) / float64(probeFirstCount)
			out.AvgFirstTokenMs = &v
		}
	}
	return out, nil
}

func (s *SupplierService) addMetricBucket(ctx context.Context, log *dbent.UsageLog) error {
	if log.GroupID == nil {
		return nil
	}
	at := log.CreatedAt.Truncate(5 * time.Minute)
	total := int64(log.InputTokens + log.OutputTokens + log.CacheCreationTokens + log.CacheReadTokens)
	duration := int64(0)
	if log.DurationMs != nil {
		duration = int64(*log.DurationMs)
	}
	ft := int64(0)
	ftc := int64(0)
	if log.FirstTokenMs != nil {
		ft = int64(*log.FirstTokenMs)
		ftc = 1
	}
	r, err := s.db.SupplierMetricBucket.Query().Where(suppliermetricbucket.GroupID(*log.GroupID), suppliermetricbucket.ResolutionEQ(suppliermetricbucket.Resolution5m), suppliermetricbucket.BucketStartEQ(at)).Only(ctx)
	if err == nil {
		return s.db.SupplierMetricBucket.UpdateOne(r).AddRequestCount(1).AddSuccessCount(1).AddTotalTokens(total).AddCacheReadTokens(int64(log.CacheReadTokens)).AddInputTokens(int64(log.InputTokens)).AddDurationMsSum(duration).AddFirstTokenMsSum(ft).AddFirstTokenCount(ftc).Exec(ctx)
	}
	return s.db.SupplierMetricBucket.Create().SetGroupID(*log.GroupID).SetBucketStart(at).SetResolution(suppliermetricbucket.Resolution5m).SetRequestCount(1).SetSuccessCount(1).SetTotalTokens(total).SetCacheReadTokens(int64(log.CacheReadTokens)).SetInputTokens(int64(log.InputTokens)).SetDurationMsSum(duration).SetFirstTokenMsSum(ft).SetFirstTokenCount(ftc).Exec(ctx)
}

func (s *SupplierService) Apply(ctx context.Context, userID int64, in SupplierApplication) (*dbent.Supplier, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.RelayURL = strings.TrimSpace(in.RelayURL)
	relayURL, parseErr := urlvalidator.ValidateHTTPSURL(in.RelayURL, urlvalidator.ValidationOptions{AllowPrivate: false})
	if in.Name == "" || parseErr != nil {
		return nil, fmt.Errorf("name and relay_url are required")
	}
	in.RelayURL = relayURL
	if existing, err := s.db.Supplier.Query().Where(supplier.UserID(userID)).Only(ctx); err == nil {
		if existing.Status != "rejected" {
			return nil, fmt.Errorf("supplier application already exists")
		}
		return s.db.Supplier.UpdateOneID(existing.ID).SetName(in.Name).SetRelayURL(in.RelayURL).SetApplicationNote(in.ApplicationNote).SetStatus(supplier.StatusPending).SetReviewNote("").Save(ctx)
	}
	return s.db.Supplier.Create().SetUserID(userID).SetName(in.Name).SetRelayURL(in.RelayURL).SetApplicationNote(in.ApplicationNote).Save(ctx)
}

func (s *SupplierService) GetByUser(ctx context.Context, userID int64) (*dbent.Supplier, error) {
	return s.db.Supplier.Query().Where(supplier.UserID(userID)).Only(ctx)
}
func (s *SupplierService) Get(ctx context.Context, id int64) (*dbent.Supplier, error) {
	return s.db.Supplier.Get(ctx, id)
}
func (s *SupplierService) List(ctx context.Context, status string) ([]*dbent.Supplier, error) {
	q := s.db.Supplier.Query().Order(dbent.Asc(supplier.FieldCreatedAt))
	if status != "" {
		q.Where(supplier.StatusEQ(supplier.Status(status)))
	}
	return q.All(ctx)
}
func (s *SupplierService) Review(ctx context.Context, id, reviewer int64, status, note string) (*dbent.Supplier, error) {
	if status != "approved" && status != "rejected" && status != "frozen" {
		return nil, fmt.Errorf("invalid supplier status")
	}
	u := s.db.Supplier.UpdateOneID(id).SetStatus(supplier.Status(status)).SetReviewedBy(reviewer).SetReviewedAt(time.Now()).SetReviewNote(note)
	if status != "frozen" {
		u.SetFreezeReason("")
	}
	return u.Save(ctx)
}
func (s *SupplierService) Freeze(ctx context.Context, id int64, reviewer int64, reason string) (*dbent.Supplier, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	sp, err := tx.Supplier.UpdateOneID(id).SetStatus(supplier.StatusFrozen).SetReviewedBy(reviewer).SetReviewedAt(time.Now()).SetFreezeReason(reason).Save(ctx)
	if err != nil {
		return nil, err
	}
	if err = tx.Group.Update().Where(group.SupplierID(id)).SetSupplierForcedOffline(true).Exec(ctx); err != nil {
		return nil, err
	}
	if err = tx.Account.Update().Where(account.SupplierID(id)).SetSchedulable(false).Exec(ctx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return sp, nil
}

func (s *SupplierService) Unfreeze(ctx context.Context, id int64, reviewer int64) (*dbent.Supplier, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	sp, err := tx.Supplier.Get(ctx, id)
	if err != nil || sp.Status != supplier.StatusFrozen {
		return nil, fmt.Errorf("frozen supplier not found")
	}
	sp, err = tx.Supplier.UpdateOne(sp).SetStatus(supplier.StatusApproved).SetReviewedBy(reviewer).SetReviewedAt(time.Now()).SetFreezeReason("").Save(ctx)
	if err != nil {
		return nil, err
	}
	if err = tx.Group.Update().Where(group.SupplierID(id)).SetSupplierForcedOffline(false).Exec(ctx); err != nil {
		return nil, err
	}
	if err = tx.Account.Update().Where(account.SupplierID(id), account.StatusEQ("active")).SetSchedulable(true).Exec(ctx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return sp, nil
}

func (s *SupplierService) SetGroupModeration(ctx context.Context, groupID int64, adjustment *float64, clearAdjustment bool, forcedOffline *bool) (*dbent.Group, error) {
	g, err := s.db.Group.Get(ctx, groupID)
	if err != nil || g.SupplierID == nil {
		return nil, fmt.Errorf("supplier group not found")
	}
	u := s.db.Group.UpdateOne(g)
	oldAdjustment := s.supplierGlobalRateAdjustment(ctx)
	if g.SupplierAdminAdjustment != nil {
		oldAdjustment = *g.SupplierAdminAdjustment
	}
	newAdjustment := oldAdjustment
	if clearAdjustment {
		u.ClearSupplierAdminAdjustment()
		newAdjustment = s.supplierGlobalRateAdjustment(ctx)
	} else if adjustment != nil {
		u.SetSupplierAdminAdjustment(*adjustment)
		newAdjustment = *adjustment
	}
	baseRate := g.RateMultiplier - oldAdjustment
	request, requestErr := s.db.SupplierResourceRequest.Query().
		Where(supplierresourcerequest.GroupID(groupID)).
		Only(ctx)
	if requestErr == nil {
		baseRate = request.RateMultiplier
	}
	finalRate := baseRate + newAdjustment
	u.SetRateMultiplier(finalRate)
	if forcedOffline != nil {
		u.SetSupplierForcedOffline(*forcedOffline)
	}
	updated, err := u.Save(ctx)
	if err == nil && requestErr == nil && request.AccountID != nil {
		_, err = s.db.Account.UpdateOneID(*request.AccountID).SetRateMultiplier(finalRate).Save(ctx)
	}
	if err == nil && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, groupID)
	}
	return updated, err
}

func (s *SupplierService) Groups(ctx context.Context, supplierID int64) ([]*dbent.Group, error) {
	return s.db.Group.Query().Where(group.SupplierID(supplierID)).Order(dbent.Asc(group.FieldSortOrder), dbent.Asc(group.FieldID)).All(ctx)
}
func (s *SupplierService) CreateGroup(ctx context.Context, supplierID int64, in SupplierGroupInput) (*dbent.Group, error) {
	if in.Name == "" || in.Platform == "" || in.RateMultiplier < 0 {
		return nil, fmt.Errorf("invalid group")
	}
	groupName, err := supplierMarketplaceGroupName(supplierID, in.Name)
	if err != nil {
		return nil, err
	}
	if exists, err := s.db.Group.Query().Where(group.NameEQ(groupName)).Exist(ctx); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("supplier group name already exists")
	}
	if in.SubscriptionType == "" {
		in.SubscriptionType = "standard"
	}
	if in.Status != "active" && in.Status != "disabled" {
		in.Status = "disabled"
	}
	b := s.db.Group.Create().SetSupplierID(supplierID).SetName(groupName).SetDescription(in.Description).SetPlatform(in.Platform).SetSubscriptionType(in.SubscriptionType).SetRateMultiplier(in.RateMultiplier).SetIsExclusive(in.IsExclusive).SetStatus(in.Status).SetSortOrder(in.SortOrder)
	if len(in.SupportedModelScopes) > 0 {
		b.SetSupportedModelScopes(in.SupportedModelScopes)
	}
	return b.Save(ctx)
}
func (s *SupplierService) UpdateGroup(ctx context.Context, supplierID, groupID int64, in SupplierGroupInput) (*dbent.Group, error) {
	g, err := s.db.Group.Query().Where(group.ID(groupID), group.SupplierID(supplierID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("group not found")
	}
	groupName, err := supplierMarketplaceGroupName(supplierID, in.Name)
	if err != nil {
		return nil, err
	}
	if groupName != g.Name {
		if exists, err := s.db.Group.Query().Where(group.NameEQ(groupName), group.IDNEQ(groupID)).Exist(ctx); err != nil {
			return nil, err
		} else if exists {
			return nil, fmt.Errorf("supplier group name already exists")
		}
	}
	if g.SupplierForcedOffline && in.Status == "active" {
		return nil, fmt.Errorf("group is forced offline by administrator")
	}
	if in.RateMultiplier < 0 {
		return nil, fmt.Errorf("invalid rate multiplier")
	}
	u := s.db.Group.UpdateOne(g).SetName(groupName).SetDescription(in.Description).SetPlatform(in.Platform).SetSubscriptionType(in.SubscriptionType).SetRateMultiplier(in.RateMultiplier).SetIsExclusive(in.IsExclusive).SetStatus(in.Status).SetSortOrder(in.SortOrder)
	if len(in.SupportedModelScopes) > 0 {
		u.SetSupportedModelScopes(in.SupportedModelScopes)
	}
	updated, err := u.Save(ctx)
	if err == nil && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, groupID)
	}
	return updated, err
}
func (s *SupplierService) Accounts(ctx context.Context, supplierID int64) ([]*dbent.Account, error) {
	return s.db.Account.Query().Where(account.SupplierID(supplierID)).WithGroups().Order(dbent.Asc(account.FieldID)).All(ctx)
}
func (s *SupplierService) validateOwnedRefs(ctx context.Context, supplierID int64, groupIDs []int64, parent *int64) error {
	if len(groupIDs) > 0 {
		n, err := s.db.Group.Query().Where(group.IDIn(groupIDs...), group.SupplierID(supplierID)).Count(ctx)
		if err != nil || n != len(groupIDs) {
			return fmt.Errorf("all groups must belong to the supplier")
		}
	}
	if parent != nil {
		ok, err := s.db.Account.Query().Where(account.ID(*parent), account.SupplierID(supplierID)).Exist(ctx)
		if err != nil || !ok {
			return fmt.Errorf("parent account must belong to the supplier")
		}
	}
	return nil
}
func (s *SupplierService) CreateAccount(ctx context.Context, supplierID int64, in SupplierAccountInput) (*dbent.Account, error) {
	if err := s.validateOwnedRefs(ctx, supplierID, in.GroupIDs, in.ParentAccountID); err != nil {
		return nil, err
	}
	if in.Name == "" || in.Platform == "" || in.Type == "" {
		return nil, fmt.Errorf("invalid account")
	}
	if in.Concurrency <= 0 {
		in.Concurrency = 3
	}
	if in.Priority <= 0 {
		in.Priority = 50
	}
	if in.RateMultiplier < 0 {
		return nil, fmt.Errorf("invalid rate multiplier")
	}
	b := s.db.Account.Create().SetSupplierID(supplierID).SetName(in.Name).SetPlatform(in.Platform).SetType(in.Type).SetCredentials(in.Credentials).SetExtra(in.Extra).SetConcurrency(in.Concurrency).SetPriority(in.Priority).SetRateMultiplier(in.RateMultiplier).SetStatus(in.Status).SetSchedulable(in.Schedulable).AddGroupIDs(in.GroupIDs...)
	if in.ParentAccountID != nil {
		b.SetParentAccountID(*in.ParentAccountID)
	}
	return b.Save(ctx)
}
func (s *SupplierService) UpdateAccount(ctx context.Context, supplierID, accountID int64, in SupplierAccountInput) (*dbent.Account, error) {
	if err := s.validateOwnedRefs(ctx, supplierID, in.GroupIDs, in.ParentAccountID); err != nil {
		return nil, err
	}
	a, err := s.db.Account.Query().Where(account.ID(accountID), account.SupplierID(supplierID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("account not found")
	}
	u := s.db.Account.UpdateOne(a).SetName(in.Name).SetPlatform(in.Platform).SetType(in.Type).SetConcurrency(in.Concurrency).SetPriority(in.Priority).SetRateMultiplier(in.RateMultiplier).SetStatus(in.Status).SetSchedulable(in.Schedulable).ClearGroups().AddGroupIDs(in.GroupIDs...)
	if in.Credentials != nil {
		u.SetCredentials(in.Credentials)
	}
	if in.Extra != nil {
		u.SetExtra(in.Extra)
	}
	if in.ParentAccountID != nil {
		u.SetParentAccountID(*in.ParentAccountID)
	} else {
		u.ClearParentAccountID()
	}
	return u.Save(ctx)
}
func (s *SupplierService) PublicGroups(ctx context.Context) ([]*dbent.Group, error) {
	owner := group.HasSupplierWith(supplier.StatusEQ(supplier.StatusApproved))
	owner = group.Or(group.SupplierIDIsNil(), owner)
	return s.db.Group.Query().Where(group.StatusEQ("active"), group.SupplierForcedOffline(false), owner).Order(dbent.Asc(group.FieldSortOrder), dbent.Asc(group.FieldID)).All(ctx)
}

func (s *SupplierService) EffectiveRate(ctx context.Context, g *dbent.Group) (float64, float64, error) {
	if g.SupplierID == nil {
		return g.RateMultiplier, 0, nil
	}
	adjust := s.supplierGlobalRateAdjustment(ctx)
	if g.SupplierAdminAdjustment != nil {
		adjust = *g.SupplierAdminAdjustment
	}
	return g.RateMultiplier, adjust, nil
}

func (s *SupplierService) supplierGlobalRateAdjustment(ctx context.Context) float64 {
	adjust := 0.0
	if settingRow, err := s.db.Setting.Query().Where(setting.KeyEQ("supplier_global_rate_adjustment")).Only(ctx); err == nil {
		_, _ = fmt.Sscanf(settingRow.Value, "%f", &adjust)
	}
	if math.IsNaN(adjust) || math.IsInf(adjust, 0) {
		return 0
	}
	return adjust
}

func (s *SupplierService) supplierSettlementDelayDays(ctx context.Context) int {
	const defaultDelayDays = 7
	row, err := s.db.Setting.Query().Where(setting.KeyEQ(SettingKeySupplierSettlementDelayDays)).Only(ctx)
	if err != nil {
		return defaultDelayDays
	}
	days, err := strconv.Atoi(strings.TrimSpace(row.Value))
	if err != nil || days < 0 || days > 365 {
		return defaultDelayDays
	}
	return days
}

// RecordEarning is idempotent and snapshots all mutable billing inputs.
func (s *SupplierService) RecordEarning(ctx context.Context, eventKey string, usageID, supplierID, groupID int64, modelCostUSD, baseRate, adminAdjustment, rechargeRatio float64) error {
	if _, err := s.db.SupplierLedger.Query().Where(supplierledger.EventKey(eventKey)).Only(ctx); err == nil {
		return nil
	}
	amount := modelCostUSD * baseRate * rechargeRatio
	if math.IsNaN(amount) || amount < 0 {
		return fmt.Errorf("invalid supplier earning")
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	delayDays := s.supplierSettlementDelayDays(ctx)
	if err = tx.SupplierLedger.Create().SetSupplierID(supplierID).SetGroupID(groupID).SetUsageLogID(usageID).SetEventKey(eventKey).SetEntryType(supplierledger.EntryTypeEarning).SetBucket(supplierledger.BucketPending).SetBaseRate(baseRate).SetAdminAdjustment(adminAdjustment).SetEffectiveRate(baseRate + adminAdjustment).SetModelCostUsd(modelCostUSD).SetRechargeRatio(rechargeRatio).SetEarningUsd(modelCostUSD * baseRate).SetAmountCny(amount).SetAvailableAt(time.Now().AddDate(0, 0, delayDays)).Exec(ctx); err != nil {
		return err
	}
	if err = tx.Supplier.UpdateOneID(supplierID).AddPendingBalanceCny(amount).Exec(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

// ReconcileUsage converts persisted supplier-group usage into idempotent settlement entries.
func (s *SupplierService) ReconcileUsage(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	logs, err := s.db.UsageLog.Query().Where(usagelog.GroupIDNotNil(), usagelog.SupplierIDIsNil()).Order(dbent.Asc(usagelog.FieldID)).Limit(limit).All(ctx)
	if err != nil {
		return 0, err
	}
	ratio := 1.0
	if row, e := s.db.Setting.Query().Where(setting.KeyEQ(SettingBalanceRechargeMult)).Only(ctx); e == nil {
		_, _ = fmt.Sscanf(row.Value, "%f", &ratio)
	}
	if ratio <= 0 {
		ratio = 1
	}
	count := 0
	for _, log := range logs {
		if log.GroupID == nil {
			continue
		}
		g, e := s.db.Group.Get(ctx, *log.GroupID)
		if e != nil || g.SupplierID == nil {
			continue
		}
		effective, adjustment, _ := s.EffectiveRate(ctx, g)
		baseRate := effective - adjustment
		if baseRate < 0 {
			baseRate = 0
		}
		modelCost := log.TotalCost
		if log.RateMultiplier > 0 {
			modelCost = log.ActualCost / log.RateMultiplier
		}
		earning := modelCost * baseRate * ratio
		if e = s.RecordEarning(ctx, fmt.Sprintf("usage:%d", log.ID), log.ID, *g.SupplierID, g.ID, modelCost, baseRate, adjustment, ratio); e != nil {
			return count, e
		}
		if e = s.db.UsageLog.UpdateOneID(log.ID).SetSupplierID(*g.SupplierID).SetSupplierBaseRate(baseRate).SetSupplierAdminAdjustment(adjustment).SetSupplierModelCostUsd(modelCost).SetSupplierRechargeRatio(ratio).SetSupplierEarningCny(earning).SetRateMultiplier(effective).Exec(ctx); e != nil {
			return count, e
		}
		if e = s.addMetricBucket(ctx, log); e != nil {
			return count, e
		}
		count++
	}
	return count, nil
}

func (s *SupplierService) ReleaseDue(ctx context.Context, now time.Time) (int, error) {
	entries, err := s.db.SupplierLedger.Query().Where(supplierledger.BucketEQ(supplierledger.BucketPending), supplierledger.AvailableAtLTE(now)).All(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		tx, err := s.db.Tx(ctx)
		if err != nil {
			return count, err
		}
		if err = tx.SupplierLedger.UpdateOneID(e.ID).SetBucket(supplierledger.BucketAvailable).SetEntryType(supplierledger.EntryTypeRelease).Exec(ctx); err == nil {
			err = tx.Supplier.UpdateOneID(e.SupplierID).AddPendingBalanceCny(-e.AmountCny).AddAvailableBalanceCny(e.AmountCny).Exec(ctx)
		}
		if err != nil {
			_ = tx.Rollback()
			return count, err
		}
		if err = tx.Commit(); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *SupplierService) Withdraw(ctx context.Context, supplierID int64, amount float64, method string, profile map[string]any) (*dbent.SupplierWithdrawal, error) {
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, fmt.Errorf("amount must be positive")
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	sp, err := tx.Supplier.Get(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	if sp.Status != supplier.StatusApproved || sp.AvailableBalanceCny < amount {
		return nil, fmt.Errorf("supplier is not eligible for withdrawal")
	}
	sp, err = tx.Supplier.UpdateOneID(supplierID).AddAvailableBalanceCny(-amount).AddFrozenBalanceCny(amount).Save(ctx)
	if err != nil {
		return nil, err
	}
	w, err := tx.SupplierWithdrawal.Create().SetSupplierID(supplierID).SetRequestNo(fmt.Sprintf("SW-%d-%d", time.Now().UnixNano(), supplierID)).SetAmountCny(amount).SetMethod(supplierwithdrawal.Method(method)).SetPayoutSnapshot(profile).Save(ctx)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *SupplierService) ListWithdrawals(ctx context.Context, supplierID *int64, status string) ([]*dbent.SupplierWithdrawal, error) {
	q := s.db.SupplierWithdrawal.Query().Order(dbent.Desc(supplierwithdrawal.FieldCreatedAt))
	if supplierID != nil {
		q.Where(supplierwithdrawal.SupplierID(*supplierID))
	}
	if status != "" {
		q.Where(supplierwithdrawal.StatusEQ(supplierwithdrawal.Status(status)))
	}
	return q.All(ctx)
}

func (s *SupplierService) ReviewWithdrawal(ctx context.Context, id, reviewer int64, next, note, proof string) (*dbent.SupplierWithdrawal, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	w, err := tx.SupplierWithdrawal.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	valid := (w.Status == supplierwithdrawal.StatusPending && (next == "approved" || next == "rejected")) || (w.Status == supplierwithdrawal.StatusApproved && next == "paid")
	if !valid {
		return nil, fmt.Errorf("invalid withdrawal status transition")
	}
	u := tx.SupplierWithdrawal.UpdateOne(w).SetStatus(supplierwithdrawal.Status(next)).SetReviewedBy(reviewer).SetReviewedAt(time.Now()).SetReviewNote(note)
	if next == "paid" {
		if proof == "" {
			return nil, fmt.Errorf("payment proof is required")
		}
		u.SetPaymentProofKey(proof).SetPaidAt(time.Now())
		err = tx.Supplier.UpdateOneID(w.SupplierID).AddFrozenBalanceCny(-w.AmountCny).Exec(ctx)
	}
	if next == "rejected" {
		err = tx.Supplier.UpdateOneID(w.SupplierID).AddFrozenBalanceCny(-w.AmountCny).AddAvailableBalanceCny(w.AmountCny).Exec(ctx)
	}
	if err != nil {
		return nil, err
	}
	saved, err := u.Save(ctx)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *SupplierService) UserIsSupplier(ctx context.Context, userID int64) (*dbent.Supplier, error) {
	return s.db.Supplier.Query().Where(supplier.UserID(userID), supplier.StatusEQ(supplier.StatusApproved)).Only(ctx)
}
