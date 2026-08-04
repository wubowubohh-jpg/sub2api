package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type SupplierHandler struct {
	svc                *service.SupplierService
	accountTestService *service.AccountTestService
}

type supplierHallItem struct {
	ID            int64                        `json:"id"`
	Name          string                       `json:"name"`
	Platform      string                       `json:"platform"`
	EffectiveRate float64                      `json:"effective_rate"`
	Status        string                       `json:"status"`
	IsExclusive   bool                         `json:"is_exclusive"`
	Metrics       service.SupplierGroupMetrics `json:"metrics"`
}

func NewSupplierHandler(svc *service.SupplierService, accountTestService *service.AccountTestService) *SupplierHandler {
	return &SupplierHandler{svc: svc, accountTestService: accountTestService}
}
func subject(c *gin.Context) (int64, bool) {
	s, ok := middleware.GetAuthSubjectFromContext(c)
	return s.UserID, ok
}

func (h *SupplierHandler) Apply(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	var in service.SupplierApplication
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application"})
		return
	}
	sp, err := h.svc.Apply(c, uid, in)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sp)
}
func (h *SupplierHandler) Me(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	_, _ = h.svc.ReconcileUsage(c, 200)
	_, _ = h.svc.ReleaseDue(c, time.Now())
	sp, err := h.svc.GetByUser(c, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "supplier profile not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                    sp.ID,
		"user_id":               sp.UserID,
		"name":                  sp.Name,
		"relay_url":             sp.RelayURL,
		"application_note":      sp.ApplicationNote,
		"status":                sp.Status,
		"review_note":           sp.ReviewNote,
		"freeze_reason":         sp.FreezeReason,
		"pending_balance_cny":   sp.PendingBalanceCny,
		"available_balance_cny": sp.AvailableBalanceCny,
		"frozen_balance_cny":    sp.FrozenBalanceCny,
		"group_name_prefix":     fmt.Sprintf("A%04d-", sp.ID),
		"supplier_code":         fmt.Sprintf("A%04d", sp.ID),
		"created_at":            sp.CreatedAt,
	})
}
func (h *SupplierHandler) MyGroups(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	groups, err := h.svc.Groups(c, sp.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (h *SupplierHandler) CreateResourceRequest(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	var in service.SupplierResourceApplication
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource application"})
		return
	}
	created, err := h.svc.CreateResourceRequest(c, sp.ID, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.ResourceRequestForSupplier(c, sp.ID, created.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *SupplierHandler) MyResourceRequests(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	items, err := h.svc.ResourceRequestsForSupplier(c, sp.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *SupplierHandler) UpdateResourceRequestAPIKey(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if parseErr != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource application id"})
		return
	}
	var in struct {
		APIKey string `json:"api_key"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key payload"})
		return
	}
	_, err = h.svc.UpdateResourceRequestAPIKey(c, sp.ID, id, in.APIKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.ResourceRequestForSupplier(c, sp.ID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *SupplierHandler) UpdateResourceRequestProbe(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if parseErr != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource application id"})
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if c.ShouldBindJSON(&in) != nil || in.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabled is required"})
		return
	}
	_, err = h.svc.UpdateResourceRequestProbe(c, sp.ID, id, *in.Enabled)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.ResourceRequestForSupplier(c, sp.ID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *SupplierHandler) UpdateResourceRequestRate(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if parseErr != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource application id"})
		return
	}
	var in struct {
		RateMultiplier *float64 `json:"rate_multiplier"`
	}
	if c.ShouldBindJSON(&in) != nil || in.RateMultiplier == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rate_multiplier is required"})
		return
	}
	if _, err = h.svc.UpdateResourceRequestRate(c, sp.ID, id, *in.RateMultiplier); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.ResourceRequestForSupplier(c, sp.ID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *SupplierHandler) MyBills(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	items, err := h.svc.Bills(c, sp.ID, c.Query("status"), 100)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (h *SupplierHandler) UploadDocument(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.GetByUser(c, uid)
	if err != nil || (sp.Status != "pending" && sp.Status != "approved") {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier application required"})
		return
	}
	f, err := c.FormFile("file")
	if err != nil || f.Size <= 0 || f.Size > 10<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file must be between 1 byte and 10 MB"})
		return
	}
	content := strings.ToLower(f.Header.Get("Content-Type"))
	allowed := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
	if !allowed[content] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only JPEG, PNG and WebP are supported"})
		return
	}
	if err := os.MkdirAll(filepath.Join("data", "supplier-documents"), 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	key := fmt.Sprintf("%d-%d-%s", sp.ID, time.Now().UnixNano(), filepath.Base(f.Filename))
	path := filepath.Join("data", "supplier-documents", key)
	if err := c.SaveUploadedFile(f, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	doc, err := h.svc.AddDocument(c, sp.ID, path, filepath.Base(f.Filename), content, f.Size)
	if err != nil {
		_ = os.Remove(path)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, doc)
}
func (h *SupplierHandler) CreateGroup(c *gin.Context) {
	uid, _ := subject(c)
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	var in service.SupplierGroupInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group"})
		return
	}
	g, err := h.svc.CreateGroup(c, sp.ID, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, g)
}
func (h *SupplierHandler) UpdateGroup(c *gin.Context) {
	uid, _ := subject(c)
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in service.SupplierGroupInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group"})
		return
	}
	g, err := h.svc.UpdateGroup(c, sp.ID, id, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, g)
}
func (h *SupplierHandler) MyAccounts(c *gin.Context) {
	uid, _ := subject(c)
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	items, err := h.svc.Accounts(c, sp.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, a := range items {
		a.Credentials = map[string]any{"configured": len(a.Credentials) > 0}
	}
	c.JSON(http.StatusOK, items)
}
func (h *SupplierHandler) CreateAccount(c *gin.Context) {
	uid, _ := subject(c)
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	var in service.SupplierAccountInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account"})
		return
	}
	a, err := h.svc.CreateAccount(c, sp.ID, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.Credentials = map[string]any{"configured": true}
	c.JSON(http.StatusCreated, a)
}
func (h *SupplierHandler) UpdateAccount(c *gin.Context) {
	uid, _ := subject(c)
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in service.SupplierAccountInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account"})
		return
	}
	a, err := h.svc.UpdateAccount(c, sp.ID, id, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.Credentials = map[string]any{"configured": true}
	c.JSON(http.StatusOK, a)
}
func (h *SupplierHandler) Hall(c *gin.Context) {
	groups, err := h.svc.PublicGroups(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]supplierHallItem, 0, len(groups))
	for _, g := range groups {
		rate, _, _ := h.svc.EffectiveRate(c, g)
		window := 6 * time.Hour
		switch c.Query("window") {
		case "24h":
			window = 24 * time.Hour
		case "7d":
			window = 7 * 24 * time.Hour
		case "30d":
			window = 30 * 24 * time.Hour
		}
		metrics, _ := h.svc.GroupMetrics(c, g.ID, time.Now().Add(-window))
		out = append(out, supplierHallItem{
			ID:            g.ID,
			Name:          g.Name,
			Platform:      g.Platform,
			EffectiveRate: rate,
			Status:        g.Status,
			IsExclusive:   g.IsExclusive,
			Metrics:       metrics,
		})
	}
	sortSupplierHallItems(out)
	c.JSON(http.StatusOK, gin.H{"groups": out})
}

func sortSupplierHallItems(items []supplierHallItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return supplierHallStatusRank(items[i].Metrics.MonitorStatus) > supplierHallStatusRank(items[j].Metrics.MonitorStatus)
	})
}

func supplierHallStatusRank(status string) int {
	switch status {
	case "operational", "degraded":
		return 1
	default:
		return 0
	}
}
func (h *SupplierHandler) Withdraw(c *gin.Context) {
	uid, ok := subject(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	sp, err := h.svc.UserIsSupplier(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier approval required"})
		return
	}
	var in struct {
		AmountCNY float64        `json:"amount_cny"`
		Method    string         `json:"method"`
		Profile   map[string]any `json:"profile"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid withdrawal"})
		return
	}
	w, err := h.svc.Withdraw(c, sp.ID, in.AmountCNY, in.Method, in.Profile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, w)
}

func (h *SupplierHandler) AdminList(c *gin.Context) {
	items, err := h.svc.List(c, c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (h *SupplierHandler) AdminReview(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, _ := subject(c)
	var in struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid review"})
		return
	}
	sp, err := h.svc.Review(c, id, uid, in.Status, in.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sp)
}
func (h *SupplierHandler) AdminFreeze(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, _ := subject(c)
	var in struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&in)
	sp, err := h.svc.Freeze(c, id, uid, in.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sp)
}

func (h *SupplierHandler) AdminUnfreeze(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, _ := subject(c)
	sp, err := h.svc.Unfreeze(c, id, uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sp)
}

func (h *SupplierHandler) AdminModerateGroup(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("group_id"), 10, 64)
	var in struct {
		Adjustment      *float64 `json:"adjustment"`
		ClearAdjustment bool     `json:"clear_adjustment"`
		ForcedOffline   *bool    `json:"forced_offline"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid moderation"})
		return
	}
	g, err := h.svc.SetGroupModeration(c, id, in.Adjustment, in.ClearAdjustment, in.ForcedOffline)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *SupplierHandler) AdminSettings(c *gin.Context) {
	settings, err := h.svc.GetSettings(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *SupplierHandler) AdminUpdateSettings(c *gin.Context) {
	var in service.SupplierSettings
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings"})
		return
	}
	settings, err := h.svc.UpdateSettings(c, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *SupplierHandler) AdminReconcile(c *gin.Context) {
	count, err := h.svc.ReconcileUsage(c, 1000)
	if err == nil {
		_, err = h.svc.ReleaseDue(c, time.Now())
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reconciled": count})
}

func (h *SupplierHandler) AdminResourceRequests(c *gin.Context) {
	items, err := h.svc.ResourceRequests(c, nil, c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// AdminTestResourceRequest tests a supplier resource application without creating
// an account or returning the decrypted credential to the client.
func (h *SupplierHandler) AdminTestResourceRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("request_id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource application id"})
		return
	}
	if h.accountTestService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "account test service unavailable"})
		return
	}

	var in struct {
		ModelID string `json:"model_id"`
		Prompt  string `json:"prompt"`
		Mode    string `json:"mode"`
	}
	// Keep parity with the account test endpoint: an empty body is valid.
	_ = c.ShouldBindJSON(&in)

	account, defaultModel, err := h.svc.BuildResourceRequestTestAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(in.ModelID) == "" {
		in.ModelID = defaultModel
	}
	_ = h.accountTestService.TestTransientAccountConnection(c, account, in.ModelID, in.Prompt, in.Mode)
}

func (h *SupplierHandler) AdminReviewResourceRequest(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("request_id"), 10, 64)
	uid, _ := subject(c)
	var in struct {
		Approved bool   `json:"approved"`
		Note     string `json:"note"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid review"})
		return
	}
	item, err := h.svc.ReviewResourceRequest(c, id, uid, in.Approved, in.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *SupplierHandler) AdminUpdateResourceRequestRate(c *gin.Context) {
	id, parseErr := strconv.ParseInt(c.Param("request_id"), 10, 64)
	if parseErr != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource application id"})
		return
	}
	var in struct {
		RateMultiplier      *float64 `json:"rate_multiplier"`
		AdminRateAdjustment *float64 `json:"admin_rate_adjustment"`
	}
	if c.ShouldBindJSON(&in) != nil || (in.RateMultiplier == nil && in.AdminRateAdjustment == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rate update is required"})
		return
	}
	item, err := h.svc.AdminUpdateResourceRequestRate(
		c, id, in.RateMultiplier, in.AdminRateAdjustment,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *SupplierHandler) AdminUpdateResourceRequest(c *gin.Context) {
	id, parseErr := strconv.ParseInt(c.Param("request_id"), 10, 64)
	if parseErr != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource application id"})
		return
	}
	var in service.SupplierResourceAdminUpdate
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource update"})
		return
	}
	item, err := h.svc.AdminUpdateResourceRequest(c, id, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *SupplierHandler) MyWithdrawals(c *gin.Context) {
	uid, _ := subject(c)
	sp, err := h.svc.GetByUser(c, uid)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "supplier required"})
		return
	}
	items, err := h.svc.ListWithdrawals(c, &sp.ID, c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (h *SupplierHandler) AdminWithdrawals(c *gin.Context) {
	items, err := h.svc.ListWithdrawals(c, nil, c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (h *SupplierHandler) AdminReviewWithdrawal(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("withdrawal_id"), 10, 64)
	uid, _ := subject(c)
	var in struct {
		Status          string `json:"status"`
		Note            string `json:"note"`
		PaymentProofKey string `json:"payment_proof_key"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid review"})
		return
	}
	w, err := h.svc.ReviewWithdrawal(c, id, uid, in.Status, in.Note, in.PaymentProofKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, w)
}
