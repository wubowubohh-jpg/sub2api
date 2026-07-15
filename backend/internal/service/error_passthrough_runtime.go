package service

import "github.com/gin-gonic/gin"

const errorPassthroughServiceContextKey = "error_passthrough_service"

// BindErrorPassthroughService 将错误透传服务绑定到请求上下文，供 service 层在非 failover 场景下复用规则。
func BindErrorPassthroughService(c *gin.Context, svc *ErrorPassthroughService) {
	if c == nil || svc == nil {
		return
	}
	c.Set(errorPassthroughServiceContextKey, svc)
}

func getBoundErrorPassthroughService(c *gin.Context) *ErrorPassthroughService {
	if c == nil {
		return nil
	}
	v, ok := c.Get(errorPassthroughServiceContextKey)
	if !ok {
		return nil
	}
	svc, ok := v.(*ErrorPassthroughService)
	if !ok {
		return nil
	}
	return svc
}

// ApplyErrorPassthroughRule exposes the shared rule resolution to handlers that
// produce protocol-specific error envelopes after failover is exhausted.
func ApplyErrorPassthroughRule(
	c *gin.Context,
	platform string,
	upstreamStatus int,
	responseBody []byte,
	defaultStatus int,
	defaultErrType string,
	defaultErrMsg string,
) (status int, errType string, errMsg string, matched bool) {
	return applyErrorPassthroughRule(c, platform, upstreamStatus, responseBody, defaultStatus, defaultErrType, defaultErrMsg)
}

// applyErrorPassthroughRule 按规则改写错误响应；未命中时返回默认响应参数。
func applyErrorPassthroughRule(
	c *gin.Context,
	platform string,
	upstreamStatus int,
	responseBody []byte,
	defaultStatus int,
	defaultErrType string,
	defaultErrMsg string,
) (status int, errType string, errMsg string, matched bool) {
	status = defaultStatus
	errType = defaultErrType
	errMsg = defaultErrMsg

	svc := getBoundErrorPassthroughService(c)
	if svc == nil {
		return status, errType, errMsg, false
	}

	rule := svc.MatchRule(platform, upstreamStatus, responseBody)
	if rule == nil {
		return status, errType, errMsg, false
	}

	status = upstreamStatus
	if !rule.PassthroughCode && rule.ResponseCode != nil {
		status = *rule.ResponseCode
	}

	if rule.PassthroughBody {
		errMsg = ExtractUpstreamErrorMessage(responseBody)
	} else {
		// A filtered rule must never fall back to upstream content. Validation
		// requires a custom message, but keep the runtime safe for legacy or
		// malformed records as well.
		if rule.CustomMessage != nil && *rule.CustomMessage != "" {
			errMsg = *rule.CustomMessage
		}
	}

	// 命中 skip_monitoring 时在 context 中标记，供 ops_error_logger 跳过记录。
	if rule.SkipMonitoring {
		c.Set(OpsSkipPassthroughKey, true)
	}
	if !rule.PassthroughBody {
		sanitizeFilteredErrorResponseHeaders(c)
	}

	// 与现有 failover 场景保持一致：命中规则时统一返回 upstream_error。
	errType = "upstream_error"
	return status, errType, errMsg, true
}

func sanitizeFilteredErrorResponseHeaders(c *gin.Context) {
	if c == nil || c.Writer == nil {
		return
	}
	header := c.Writer.Header()
	for _, name := range []string{
		"Alt-Svc",
		"Content-Encoding",
		"Content-Length",
		"Content-Location",
		"ETag",
		"Last-Modified",
		"Link",
		"Location",
		"NEL",
		"OpenAI-Request-ID",
		"Proxy-Authenticate",
		"Refresh",
		"Report-To",
		"Reporting-Endpoints",
		"Request-ID",
		"Retry-After",
		"Server",
		"Server-Timing",
		"Set-Cookie",
		"Via",
		"WWW-Authenticate",
		"X-Powered-By",
		"X-Request-ID",
		"CF-Cache-Status",
		"CF-Ray",
	} {
		header.Del(name)
	}
	header.Set("Cache-Control", "no-store")
}
