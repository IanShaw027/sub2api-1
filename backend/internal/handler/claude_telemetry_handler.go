package handler

import (
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ClaudeTelemetryBatch handles Claude Code telemetry in forward mode. The
// client always receives 200 so telemetry failures never affect normal CLI use.
// Forwarding is limited to Anthropic groups and the service only uses
// Anthropic OAuth/SetupToken accounts.
func (h *GatewayHandler) ClaudeTelemetryBatch(c *gin.Context) {
	var body []byte
	if c.Request != nil && c.Request.Body != nil {
		body, _ = io.ReadAll(c.Request.Body)
	}

	service.RecordGatewayDebugTimelineBody(h.settingService, c, "cc_aux_request", service.SanitizeClaudeTelemetryBatch(body, service.ClaudeTelemetrySanitizeOptions{}),
		c.GetHeader("Content-Type"), map[string]any{
			"component":     "cc_aux_endpoint",
			"endpoint_name": "event_logging_batch",
			"mode":          "forward",
		})

	apiKey, ok := servermiddleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformAnthropic {
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	if h == nil || h.gatewayService == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	status, err := h.gatewayService.ForwardClaudeTelemetryBatch(c.Request.Context(), apiKey.GroupID, body)
	if err != nil {
		logger.L().Warn("claude_telemetry_forward_failed", zap.Error(err))
	} else if status >= 400 {
		logger.L().Warn("claude_telemetry_forward_upstream_status", zap.Int("status", status))
	}
	c.JSON(http.StatusOK, gin.H{})
}
