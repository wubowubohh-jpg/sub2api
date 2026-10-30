package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type groupPromptRouteAdmin struct{ service.AdminService }

func (*groupPromptRouteAdmin) GetGroup(context.Context, int64) (*service.Group, error) {
	return &service.Group{ID: 7}, nil
}

type groupPromptRouteRepo struct {
	service.SettingRepository
	value string
}

func (r *groupPromptRouteRepo) GetValue(context.Context, string) (string, error) {
	return r.value, nil
}
func (r *groupPromptRouteRepo) Set(_ context.Context, _, value string) error {
	r.value = value
	return nil
}

func TestGroupPromptRoutesInheritAdminMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	audited := 0
	admin := r.Group("/api/v1/admin", func(c *gin.Context) {
		if c.GetHeader("X-Test-Admin") == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if c.GetHeader("X-Test-Admin") == "scoped" {
			c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{}))
		}
	}, func(c *gin.Context) { audited++ })
	h := &handler.Handlers{Admin: &handler.AdminHandlers{Group: adminhandler.NewGroupHandler(&groupPromptRouteAdmin{}, nil, nil)}}
	repo := &groupPromptRouteRepo{}
	// Existing group registration keeps its original signature and routes.
	registerGroupRoutes(admin, h)
	registerGroupPromptRoutes(admin, h, service.NewSettingService(repo, nil))
	request := func(method, credential, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/admin/groups/7/prepend-prompt", strings.NewReader(body))
		req.Header.Set("X-Test-Admin", credential)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	payload := `{"enabled":true,"content":"group","model_prompts":{"model-a":"model prompt"}}`
	require.Equal(t, http.StatusUnauthorized, request(http.MethodPut, "", payload).Code)
	require.Empty(t, repo.value)
	require.Equal(t, http.StatusOK, request(http.MethodPut, "session", payload).Code)
	w := request(http.MethodGet, "session", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "model prompt")
	stored := repo.value
	require.Equal(t, http.StatusForbidden, request(http.MethodPut, "scoped", `{"enabled":false}`).Code)
	require.Equal(t, stored, repo.value)
	require.Equal(t, 3, audited)
}
