package routes

import (
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterCommonRoutes 注册通用路由（健康检查、状态等）+ CC 辅助端点 stub
func RegisterCommonRoutes(r *gin.Engine, settingService *service.SettingService) {
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

	// 一方遥测事件上报
	r.POST("/api/event_logging/batch",
		ccAuxHandler(settingService, "event_logging_batch", http.StatusOK, gin.H{}))

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

// ccAuxHandler 为 CC CLI 辅助端点生成通用 handler：
// 读取完整请求头+请求体 → 写入 Gateway Debug Timeline → 返回 stub 响应。
func ccAuxHandler(settingService *service.SettingService, endpointName string, statusCode int, response any) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 读取请求体
		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(c.Request.Body)
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
