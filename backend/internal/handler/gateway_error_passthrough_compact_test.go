//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const cloudflareCompactErrorBody = `{"cloudflare_error":true,"detail":"The origin www.xiaobaishu.org did not respond to Cloudflare","instance":"a1ba23f3789bc146","ray_id":"a1ba23f3789bc146","status":504,"type":"https://developers.cloudflare.com/support/error-504/","zone":"www.xiaobaishu.org"}`

type staticErrorPassthroughRuleRepo struct {
	service.ErrorPassthroughRepository
	rules []*model.ErrorPassthroughRule
}

func (r *staticErrorPassthroughRuleRepo) List(context.Context) ([]*model.ErrorPassthroughRule, error) {
	return r.rules, nil
}

func newCloudflareFilterService(platforms []string) *service.ErrorPassthroughService {
	responseCode := http.StatusBadGateway
	message := "Upstream service temporarily unavailable"
	return service.NewErrorPassthroughService(&staticErrorPassthroughRuleRepo{
		rules: []*model.ErrorPassthroughRule{
			{
				ID:              1,
				Name:            "cloudflare-filter",
				Enabled:         true,
				Priority:        0,
				Keywords:        []string{"cloudflare"},
				MatchMode:       model.MatchModeAny,
				Platforms:       platforms,
				PassthroughCode: false,
				ResponseCode:    &responseCode,
				PassthroughBody: false,
				CustomMessage:   &message,
			},
		},
	}, nil)
}

type cloudflareCompactUpstream struct {
	service.HTTPUpstream
}

func (u *cloudflareCompactUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusGatewayTimeout,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Location":     []string{"https://www.xiaobaishu.org/cdn-cgi/error"},
			"CF-Ray":       []string{"a1ba23f3789bc146"},
			"Server":       []string{"cloudflare"},
		},
		Body: io.NopCloser(strings.NewReader(cloudflareCompactErrorBody)),
	}, nil
}

func requireFilteredCloudflareResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Equal(t, "Upstream service temporarily unavailable", gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
	response := strings.ToLower(rec.Body.String())
	for _, secret := range []string{
		"cloudflare",
		"xiaobaishu.org",
		"ray_id",
		"a1ba23f3789bc146",
		"instance",
		"detail",
		"zone",
		"developers.cloudflare.com",
	} {
		require.NotContains(t, response, secret)
	}
	require.Empty(t, rec.Header().Get("Location"))
	require.Empty(t, rec.Header().Get("CF-Ray"))
	require.Empty(t, rec.Header().Get("Server"))
}

func TestOpenAIResponsesCompactAppliesConfiguredCloudflareFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newOpenAIResponsesFailoverTestHandler(t, &cloudflareCompactUpstream{})
	handler.errorPassthroughService = newCloudflareFilterService([]string{service.PlatformOpenAI})
	c, rec := newOpenAIResponsesFailoverTestContext(t, nil)
	c.Request.URL.Path = "/v1/responses/compact"
	c.Request.RequestURI = "/v1/responses/compact"

	handler.Responses(c)

	require.Equal(t, "upstream_error", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
	requireFilteredCloudflareResponse(t, rec)
}

func TestGatewayResponsesFailoverAppliesConfiguredCloudflareFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(nil))
	c.Header("Location", "https://www.xiaobaishu.org/cdn-cgi/error")
	c.Header("CF-Ray", "a1ba23f3789bc146")
	c.Header("Server", "cloudflare")
	groupID := int64(77)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      9,
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAnthropic,
		},
	})
	handler := &GatewayHandler{
		errorPassthroughService: newCloudflareFilterService([]string{service.PlatformAnthropic}),
	}

	handler.handleResponsesFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusGatewayTimeout,
		ResponseBody: []byte(cloudflareCompactErrorBody),
	}, false)

	require.Equal(t, "upstream_error", gjson.GetBytes(rec.Body.Bytes(), "error.code").String())
	requireFilteredCloudflareResponse(t, rec)
}
