// Package routes 提供 HTTP 路由注册和处理函数
package routes

import (
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// RegisterCommonRoutes 注册通用路由（健康检查、状态等）
func RegisterCommonRoutes(r *gin.Engine) {
	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Prometheus metrics (minimal exposition)
	r.GET("/metrics", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		queueDepth := handler.OpsErrorLogQueueLength()
		c.String(http.StatusOK, fmt.Sprintf(
			"# HELP ops_error_log_queue_depth Current ops error log async queue depth.\n"+
				"# TYPE ops_error_log_queue_depth gauge\n"+
				"ops_error_log_queue_depth %d\n"+
				"# HELP ops_error_log_queue_length Current ops error log async queue length.\n"+
				"# TYPE ops_error_log_queue_length gauge\n"+
				"ops_error_log_queue_length %d\n"+
				"# HELP usage_logs_failed_total Total number of failed usage log writes.\n"+
				"# TYPE usage_logs_failed_total counter\n"+
				"usage_logs_failed_total %d\n",
			queueDepth,
			queueDepth,
			metrics.UsageLogsFailedTotal(),
		))
	})

	// Claude Code 遥测日志（忽略，直接返回200）
	r.POST("/api/event_logging/batch", func(c *gin.Context) {
		c.Status(http.StatusOK)
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
}
