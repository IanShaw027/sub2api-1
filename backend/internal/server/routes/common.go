package routes

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const ccAuxMaxBodyBytes int64 = 1 << 20

// RegisterCommonRoutes 注册通用路由（健康检查、状态等）+ CC 辅助端点 stub
func RegisterCommonRoutes(r *gin.Engine, h *handler.Handlers, apiKeyAuth middleware.APIKeyAuthMiddleware, settingService *service.SettingService, _ any) {
	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Setup status endpoint (always returns needs_setup: false in normal mode)
	// This is used by the frontend to detect when the service has restarted after setup
	r.GET("/setup/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"needs_setup": false,
				"step":        "completed",
			},
		})
	})

	// -----------------------------------------------------------------------
	// CC CLI 辅助端点 — stub 响应 + Debug Timeline 日志
	//
	// 真实 CC CLI 会向 api.anthropic.com 发送以下非 Messages API 请求。
	// 当客户端流量通过 DNS 拦截打到 sub2api 时，需要：
	// 1. 返回适当的 stub 响应（避免客户端报错）
	// 2. 在 Gateway Debug Timeline 中记录完整请求头和请求体（便于后续分析）
	// -----------------------------------------------------------------------

	// 一方遥测事件上报：默认 drop；forward 模式下先鉴权，再只通过 Anthropic OAuth 账号清洗代发。
	r.POST("/api/event_logging/batch", claudeTelemetryModeHandler(h, apiKeyAuth, settingService))

	// 启动引导配置
	r.GET("/api/claude_cli/bootstrap",
		ccAuxHandler(settingService, "bootstrap", http.StatusOK, gin.H{}))

	// 组织策略限制
	r.GET("/api/claude_code/policy_limits",
		ccAuxHandler(settingService, "policy_limits", http.StatusOK, gin.H{"restrictions": gin.H{}}))

	// 设置同步
	r.GET("/api/claude_code/user_settings",
		ccAuxHandler(settingService, "user_settings_get", http.StatusOK, gin.H{"entries": []any{}}))
	r.PUT("/api/claude_code/user_settings",
		ccAuxHandler(settingService, "user_settings_put", http.StatusOK, gin.H{}))

	// OAuth 用户资料
	r.GET("/api/claude_cli_profile",
		ccAuxHandler(settingService, "cli_profile", http.StatusOK, gin.H{}))
	r.GET("/api/oauth/profile",
		ccAuxHandler(settingService, "oauth_profile", http.StatusOK, gin.H{}))

	// Fast Mode 切换
	r.POST("/api/claude_code_penguin_mode",
		ccAuxHandler(settingService, "penguin_mode", http.StatusOK, gin.H{}))

	// 用量查询
	r.GET("/api/oauth/usage",
		ccAuxHandler(settingService, "oauth_usage", http.StatusOK, gin.H{}))
}

func claudeTelemetryModeHandler(h *handler.Handlers, apiKeyAuth middleware.APIKeyAuthMiddleware, settingService *service.SettingService) gin.HandlerFunc {
	forwardHandler := gin.HandlerFunc(func(c *gin.Context) {
		if h == nil || h.Gateway == nil {
			c.JSON(http.StatusOK, gin.H{})
			return
		}
		h.Gateway.ClaudeTelemetryBatch(c)
	})
	dropHandler := claudeTelemetryDropHandler(settingService)
	return func(c *gin.Context) {
		if !limitCCAuxRequestBody(c) {
			return
		}
		if settingService == nil || settingService.GetClaudeTelemetryMode(c.Request.Context()) != service.ClaudeTelemetryModeForward || apiKeyAuth == nil {
			dropHandler(c)
			return
		}
		if !runClaudeTelemetrySoftAPIKeyAuth(c, apiKeyAuth) {
			dropHandler(c)
			return
		}
		forwardHandler(c)
	}
}

type claudeTelemetryDiscardResponseWriter struct {
	gin.ResponseWriter
	header  http.Header
	status  int
	size    int
	written bool
}

func newClaudeTelemetryDiscardResponseWriter(base gin.ResponseWriter) *claudeTelemetryDiscardResponseWriter {
	return &claudeTelemetryDiscardResponseWriter{
		ResponseWriter: base,
		header:         http.Header{},
		status:         http.StatusOK,
		size:           -1,
	}
}

func (w *claudeTelemetryDiscardResponseWriter) Header() http.Header {
	return w.header
}

func (w *claudeTelemetryDiscardResponseWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.status = code
	w.written = true
}

func (w *claudeTelemetryDiscardResponseWriter) WriteHeaderNow() {
	if !w.written {
		w.WriteHeader(w.status)
	}
}

func (w *claudeTelemetryDiscardResponseWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	if w.size < 0 {
		w.size = 0
	}
	w.size += len(data)
	return len(data), nil
}

func (w *claudeTelemetryDiscardResponseWriter) WriteString(data string) (int, error) {
	w.WriteHeaderNow()
	if w.size < 0 {
		w.size = 0
	}
	w.size += len(data)
	return len(data), nil
}

func (w *claudeTelemetryDiscardResponseWriter) Status() int {
	return w.status
}

func (w *claudeTelemetryDiscardResponseWriter) Size() int {
	return w.size
}

func (w *claudeTelemetryDiscardResponseWriter) Written() bool {
	return w.written
}

func runClaudeTelemetrySoftAPIKeyAuth(c *gin.Context, apiKeyAuth middleware.APIKeyAuthMiddleware) bool {
	if c == nil || apiKeyAuth == nil {
		return false
	}
	skipBillingKey := string(middleware.ContextKeySkipAPIKeyBilling)
	previousSkipBilling, hadPreviousSkipBilling := c.Get(skipBillingKey)
	c.Set(skipBillingKey, true)
	originalWriter := c.Writer
	c.Writer = newClaudeTelemetryDiscardResponseWriter(originalWriter)
	gin.HandlerFunc(apiKeyAuth)(c)
	c.Writer = originalWriter
	if hadPreviousSkipBilling {
		c.Set(skipBillingKey, previousSkipBilling)
	} else if c.Keys != nil {
		delete(c.Keys, skipBillingKey)
	}

	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	return ok && apiKey != nil
}

func claudeTelemetryDropHandler(settingService *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, ok := readCCAuxRequestBody(c)
		if !ok {
			return
		}
		// ok=false 时 sanitizedTelemetry 为 nil：fail-closed，不把未脱敏原文记入 debug timeline。
		sanitizedTelemetry, _ := service.SanitizeClaudeTelemetryBatch(body, service.ClaudeTelemetrySanitizeOptions{})
		service.RecordGatewayDebugTimelineBody(settingService, c, "cc_aux_request", sanitizedTelemetry,
			c.GetHeader("Content-Type"), map[string]any{
				"component":     "cc_aux_endpoint",
				"endpoint_name": "event_logging_batch",
				"mode":          "drop",
			})
		c.JSON(http.StatusOK, gin.H{})
	}
}

// ccAuxHandler 为 CC CLI 辅助端点生成通用 handler：
// 读取完整请求头+请求体 → 写入 Gateway Debug Timeline → 返回 stub 响应。
func ccAuxHandler(settingService *service.SettingService, endpointName string, statusCode int, response any) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limitCCAuxRequestBody(c) {
			return
		}
		// 1. 读取请求体
		body, ok := readCCAuxRequestBody(c)
		if !ok {
			return
		}

		// 2. 收集完整请求头（敏感值脱敏）
		headers := make(map[string]string, len(c.Request.Header))
		for k, vs := range c.Request.Header {
			headers[k] = redactSensitiveHeader(k, strings.Join(vs, ", "))
		}

		// 3. 写入 Gateway Debug Timeline
		service.RecordGatewayDebugTimelineBody(settingService, c, "cc_aux_request", body,
			c.GetHeader("Content-Type"), map[string]any{
				"component":     "cc_aux_endpoint",
				"endpoint_name": endpointName,
				"headers":       headers,
			})

		// 4. 返回 stub 响应
		c.JSON(statusCode, response)
	}
}

func limitCCAuxRequestBody(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return true
	}
	if c.Request.ContentLength > ccAuxMaxBodyBytes {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ccAuxMaxBodyBytes)
	return true
}

func readCCAuxRequestBody(c *gin.Context) ([]byte, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, true
	}
	body, err := io.ReadAll(c.Request.Body)
	if err == nil {
		return body, true
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
		return nil, false
	}
	return nil, true
}

// redactSensitiveHeader 对 authorization / x-api-key 等敏感请求头值进行脱敏。
func redactSensitiveHeader(key, value string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "authorization":
		if strings.HasPrefix(strings.ToLower(value), "bearer ") {
			return "Bearer [redacted]"
		}
		return "[redacted]"
	case "x-api-key":
		return "[redacted]"
	default:
		return value
	}
}
