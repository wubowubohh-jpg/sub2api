package service

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/ent/supplier"
	"github.com/Wei-Shaw/sub2api/ent/supplierledger"
	"github.com/Wei-Shaw/sub2api/ent/suppliermetricbucket"
	"github.com/Wei-Shaw/sub2api/ent/supplierresourcerequest"
	"github.com/Wei-Shaw/sub2api/ent/supplierwithdrawal"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
)

type SupplierService struct {
	db        *dbent.Client
	encryptor SecretEncryptor
	stop      chan struct{}
	stopOnce  sync.Once
}

func NewSupplierService(db *dbent.Client, encryptor SecretEncryptor) *SupplierService {
	s := &SupplierService{db: db, encryptor: encryptor, stop: make(chan struct{})}
	go s.runSettlementWorker()
	return s
}

type SupplierResourceApplication struct {
	GroupName string `json:"group_name"`
	RelayName string `json:"relay_name"`
	RelayURL  string `json:"relay_url"`
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`
}

func (s *SupplierService) CreateResourceRequest(ctx context.Context, supplierID int64, in SupplierResourceApplication) (*dbent.SupplierResourceRequest, error) {
	in.GroupName, in.RelayName, in.RelayURL, in.APIKey = strings.TrimSpace(in.GroupName), strings.TrimSpace(in.RelayName), strings.TrimSpace(in.RelayURL), strings.TrimSpace(in.APIKey)
	parsed, err := url.Parse(in.RelayURL)
	if in.GroupName == "" || in.RelayName == "" || in.APIKey == "" || err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid resource application")
	}
	if in.Model == "" {
		in.Model = "gpt-5.5"
	}
	sp, err := s.db.Supplier.Get(ctx, supplierID)
	if err != nil || sp.Status != supplier.StatusApproved {
		return nil, fmt.Errorf("supplier is not approved")
	}
	encrypted, err := s.encryptor.Encrypt(in.APIKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}
	return s.db.SupplierResourceRequest.Create().SetSupplierID(supplierID).SetGroupName(in.GroupName).SetRelayName(in.RelayName).SetRelayURL(in.RelayURL).SetAPIKeyEncrypted(encrypted).SetModel(in.Model).Save(ctx)
}

func (s *SupplierService) ResourceRequests(ctx context.Context, supplierID *int64, status string) ([]*dbent.SupplierResourceRequest, error) {
	q := s.db.SupplierResourceRequest.Query().Order(dbent.Desc(supplierresourcerequest.FieldCreatedAt))
	if supplierID != nil {
		q.Where(supplierresourcerequest.SupplierID(*supplierID))
	}
	if status != "" {
		q.Where(supplierresourcerequest.StatusEQ(supplierresourcerequest.Status(status)))
	}
	return q.All(ctx)
}

func (s *SupplierService) ReviewResourceRequest(ctx context.Context, requestID, reviewerID int64, approved bool, note string) (*dbent.SupplierResourceRequest, error) {
	req, err := s.db.SupplierResourceRequest.Get(ctx, requestID)
	if err != nil || req.Status != supplierresourcerequest.StatusPending {
		return nil, fmt.Errorf("pending resource application not found")
	}
	if !approved {
		return req.Update().SetStatus(supplierresourcerequest.StatusRejected).SetReviewedBy(reviewerID).SetReviewedAt(time.Now()).SetReviewNote(note).Save(ctx)
	}
	key, err := s.encryptor.Decrypt(req.APIKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	g, err := tx.Group.Create().SetSupplierID(req.SupplierID).SetName(req.GroupName).SetDescription(req.RelayName).SetPlatform("openai").SetSubscriptionType("standard").SetRateMultiplier(1).SetStatus("active").SetIsExclusive(false).Save(ctx)
	if err != nil {
		return nil, err
	}
	a, err := tx.Account.Create().SetSupplierID(req.SupplierID).SetName(req.RelayName).SetPlatform("openai").SetType("api_key").SetCredentials(map[string]any{"api_key": key}).SetExtra(map[string]any{"base_url": req.RelayURL}).SetStatus("active").SetSchedulable(true).AddGroupIDs(g.ID).Save(ctx)
	if err != nil {
		return nil, err
	}
	m, err := tx.ChannelMonitor.Create().SetName(req.RelayName).SetProvider("openai").SetAPIMode("chat_completions").SetEndpoint(req.RelayURL).SetAPIKeyEncrypted(req.APIKeyEncrypted).SetPrimaryModel(req.Model).SetExtraModels([]string{}).SetGroupName(req.GroupName).SetGroupID(g.ID).SetEnabled(true).SetIntervalSeconds(60).SetJitterSeconds(5).SetCreatedBy(reviewerID).Save(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := tx.SupplierResourceRequest.UpdateOneID(req.ID).SetStatus(supplierresourcerequest.StatusApproved).SetReviewedBy(reviewerID).SetReviewedAt(time.Now()).SetReviewNote(note).SetGroupID(g.ID).SetAccountID(a.ID).SetMonitorID(m.ID).Save(ctx)
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
	CacheHitRate    *float64              `json:"cache_hit_rate"`
	TPS             *float64              `json:"tps"`
	Availability    *float64              `json:"availability"`
	Timeline        []SupplierMetricPoint `json:"timeline"`
}

type SupplierSettings struct {
	GlobalRateAdjustment float64 `json:"global_rate_adjustment"`
	MinimumWithdrawalUSD float64 `json:"minimum_withdrawal_usd"`
}

func (s *SupplierService) GetSettings(ctx context.Context) (SupplierSettings, error) {
	out := SupplierSettings{MinimumWithdrawalUSD: 100}
	rows, err := s.db.Setting.Query().Where(setting.KeyIn("supplier_global_rate_adjustment", "supplier_min_withdrawal_usd")).All(ctx)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		v, parseErr := strconv.ParseFloat(strings.TrimSpace(row.Value), 64)
		if parseErr != nil {
			continue
		}
		if row.Key == "supplier_global_rate_adjustment" {
			out.GlobalRateAdjustment = v
		} else if row.Key == "supplier_min_withdrawal_usd" {
			out.MinimumWithdrawalUSD = v
		}
	}
	return out, nil
}

func (s *SupplierService) UpdateSettings(ctx context.Context, in SupplierSettings) (SupplierSettings, error) {
	if math.IsNaN(in.GlobalRateAdjustment) || math.IsInf(in.GlobalRateAdjustment, 0) || in.MinimumWithdrawalUSD <= 0 || math.IsNaN(in.MinimumWithdrawalUSD) || math.IsInf(in.MinimumWithdrawalUSD, 0) {
		return SupplierSettings{}, fmt.Errorf("invalid supplier settings")
	}
	values := map[string]float64{"supplier_global_rate_adjustment": in.GlobalRateAdjustment, "supplier_min_withdrawal_usd": in.MinimumWithdrawalUSD}
	for key, value := range values {
		formatted := strconv.FormatFloat(value, 'f', -1, 64)
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
	return s.GetSettings(ctx)
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
	var success int64
	var duration, tokens, input, cache, ftSum, ftCount int64
	for _, r := range rows {
		out.RequestCount += r.RequestCount
		success += r.SuccessCount
		duration += r.DurationMsSum
		tokens += r.TotalTokens
		input += r.InputTokens
		cache += r.CacheReadTokens
		ftSum += r.FirstTokenMsSum
		ftCount += r.FirstTokenCount
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
		v := float64(success) / float64(out.RequestCount) * 100
		out.Availability = &v
		v = float64(duration) / float64(out.RequestCount)
		out.AvgLatencyMs = &v
	}
	if ftCount > 0 {
		v := float64(ftSum) / float64(ftCount)
		out.AvgFirstTokenMs = &v
	}
	if input+cache > 0 {
		v := float64(cache) / float64(input+cache) * 100
		out.CacheHitRate = &v
	}
	if duration > 0 {
		v := float64(tokens) * 1000 / float64(duration)
		out.TPS = &v
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
	relayURL, parseErr := url.Parse(in.RelayURL)
	if in.Name == "" || parseErr != nil || relayURL == nil || relayURL.Scheme != "https" || relayURL.Hostname() == "" || relayURL.User != nil {
		return nil, fmt.Errorf("name and relay_url are required")
	}
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

func (s *SupplierService) SetGroupModeration(ctx context.Context, groupID int64, adjustment *float64, clearAdjustment bool, forcedOffline *bool) (*dbent.Group, error) {
	g, err := s.db.Group.Get(ctx, groupID)
	if err != nil || g.SupplierID == nil {
		return nil, fmt.Errorf("supplier group not found")
	}
	u := s.db.Group.UpdateOne(g)
	if clearAdjustment {
		u.ClearSupplierAdminAdjustment()
	} else if adjustment != nil {
		u.SetSupplierAdminAdjustment(*adjustment)
	}
	if forcedOffline != nil {
		u.SetSupplierForcedOffline(*forcedOffline)
	}
	return u.Save(ctx)
}

func (s *SupplierService) Groups(ctx context.Context, supplierID int64) ([]*dbent.Group, error) {
	return s.db.Group.Query().Where(group.SupplierID(supplierID)).Order(dbent.Asc(group.FieldSortOrder), dbent.Asc(group.FieldID)).All(ctx)
}
func (s *SupplierService) CreateGroup(ctx context.Context, supplierID int64, in SupplierGroupInput) (*dbent.Group, error) {
	if in.Name == "" || in.Platform == "" || in.RateMultiplier < 0 {
		return nil, fmt.Errorf("invalid group")
	}
	if in.SubscriptionType == "" {
		in.SubscriptionType = "standard"
	}
	if in.Status != "active" && in.Status != "disabled" {
		in.Status = "disabled"
	}
	b := s.db.Group.Create().SetSupplierID(supplierID).SetName(in.Name).SetDescription(in.Description).SetPlatform(in.Platform).SetSubscriptionType(in.SubscriptionType).SetRateMultiplier(in.RateMultiplier).SetIsExclusive(in.IsExclusive).SetStatus(in.Status).SetSortOrder(in.SortOrder)
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
	if g.SupplierForcedOffline && in.Status == "active" {
		return nil, fmt.Errorf("group is forced offline by administrator")
	}
	if in.RateMultiplier < 0 {
		return nil, fmt.Errorf("invalid rate multiplier")
	}
	u := s.db.Group.UpdateOne(g).SetName(in.Name).SetDescription(in.Description).SetPlatform(in.Platform).SetSubscriptionType(in.SubscriptionType).SetRateMultiplier(in.RateMultiplier).SetIsExclusive(in.IsExclusive).SetStatus(in.Status).SetSortOrder(in.SortOrder)
	if len(in.SupportedModelScopes) > 0 {
		u.SetSupportedModelScopes(in.SupportedModelScopes)
	}
	return u.Save(ctx)
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
	return s.db.Group.Query().Where(group.StatusEQ("active"), group.SupplierForcedOffline(false), group.Or(group.SupplierIDIsNil(), group.HasSupplierWith(supplier.StatusEQ(supplier.StatusApproved)))).WithSupplier().Order(dbent.Asc(group.FieldSortOrder), dbent.Asc(group.FieldID)).All(ctx)
}

func (s *SupplierService) EffectiveRate(ctx context.Context, g *dbent.Group) (float64, float64, error) {
	if g.SupplierID == nil {
		return g.RateMultiplier, 0, nil
	}
	adjust := 0.0
	if g.SupplierAdminAdjustment != nil {
		adjust = *g.SupplierAdminAdjustment
	} else if settingRow, err := s.db.Setting.Query().Where(setting.KeyEQ("supplier_global_rate_adjustment")).Only(ctx); err == nil {
		_, _ = fmt.Sscanf(settingRow.Value, "%f", &adjust)
	}
	return g.RateMultiplier + adjust, adjust, nil
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
	if err = tx.SupplierLedger.Create().SetSupplierID(supplierID).SetGroupID(groupID).SetUsageLogID(usageID).SetEventKey(eventKey).SetEntryType(supplierledger.EntryTypeEarning).SetBucket(supplierledger.BucketPending).SetBaseRate(baseRate).SetAdminAdjustment(adminAdjustment).SetEffectiveRate(baseRate + adminAdjustment).SetModelCostUsd(modelCostUSD).SetRechargeRatio(rechargeRatio).SetEarningUsd(modelCostUSD * baseRate).SetAmountCny(amount).SetAvailableAt(time.Now().Add(7 * 24 * time.Hour)).Exec(ctx); err != nil {
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
		modelCost := log.TotalCost
		if log.RateMultiplier > 0 {
			modelCost = log.ActualCost / log.RateMultiplier
		}
		earning := modelCost * g.RateMultiplier * ratio
		if e = s.RecordEarning(ctx, fmt.Sprintf("usage:%d", log.ID), log.ID, *g.SupplierID, g.ID, modelCost, g.RateMultiplier, adjustment, ratio); e != nil {
			return count, e
		}
		if e = s.db.UsageLog.UpdateOneID(log.ID).SetSupplierID(*g.SupplierID).SetSupplierBaseRate(g.RateMultiplier).SetSupplierAdminAdjustment(adjustment).SetSupplierModelCostUsd(modelCost).SetSupplierRechargeRatio(ratio).SetSupplierEarningCny(earning).SetRateMultiplier(effective).Exec(ctx); e != nil {
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
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	minUSD, ratio := 100.0, 1.0
	if row, e := s.db.Setting.Query().Where(setting.KeyEQ("supplier_min_withdrawal_usd")).Only(ctx); e == nil {
		_, _ = fmt.Sscanf(row.Value, "%f", &minUSD)
	}
	if row, e := s.db.Setting.Query().Where(setting.KeyEQ(SettingBalanceRechargeMult)).Only(ctx); e == nil {
		_, _ = fmt.Sscanf(row.Value, "%f", &ratio)
	}
	if ratio <= 0 {
		ratio = 1
	}
	if amount < minUSD*ratio {
		return nil, fmt.Errorf("amount is below the minimum withdrawal threshold")
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
