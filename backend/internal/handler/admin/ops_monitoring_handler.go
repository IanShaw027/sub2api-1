package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OpsMonitoringHandler handles ops monitoring endpoints.
type OpsMonitoringHandler struct {
	opsService *service.OpsService
}

// NewOpsMonitoringHandler creates a new OpsMonitoringHandler.
func NewOpsMonitoringHandler(opsService *service.OpsService) *OpsMonitoringHandler {
	return &OpsMonitoringHandler{opsService: opsService}
}

// GetErrorLogs returns error logs with filters and pagination.
// GET /api/v1/admin/ops/error-logs
//
// Query params:
// - start_time: RFC3339 timestamp (optional)
// - end_time: RFC3339 timestamp (optional)
// - platform: string (optional)
// - error_source: string (optional)
// - error_type: string (optional)
// - limit: int (optional; default 20; max 100)
// - offset: int (optional; default 0)
func (h *OpsMonitoringHandler) GetErrorLogs(c *gin.Context) {
	filter := &service.ErrorLogFilter{}

	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid start_time format (RFC3339)")
			return
		}
		filter.StartTime = &startTime
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid end_time format (RFC3339)")
			return
		}
		filter.EndTime = &endTime
	}

	if filter.StartTime != nil && filter.EndTime != nil && filter.StartTime.After(*filter.EndTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	filter.Provider = c.Query("platform")

	page := 1
	pageSize := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 100 {
			response.BadRequest(c, "Invalid limit (must be 1-100)")
			return
		}
		pageSize = limit
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			response.BadRequest(c, "Invalid offset")
			return
		}
		page = (offset / pageSize) + 1
	}

	filter.Page = page
	filter.PageSize = pageSize

	result, err := h.opsService.GetErrorLogs(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get error logs")
		return
	}

	response.Success(c, gin.H{
		"items":  result.Errors,
		"total":  result.Total,
		"limit":  result.PageSize,
		"offset": (result.Page - 1) * result.PageSize,
	})
}

// GetErrorStats returns error statistics aggregated by dimensions.
// GET /api/v1/admin/ops/error-stats
//
// Query params:
// - start_time: RFC3339 timestamp (optional)
// - end_time: RFC3339 timestamp (optional)
// - group_by: string (optional; one of: platform, error_type, error_source)
func (h *OpsMonitoringHandler) GetErrorStats(c *gin.Context) {
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid start_time format (RFC3339)")
			return
		}
		startTime = parsed
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid end_time format (RFC3339)")
			return
		}
		endTime = parsed
	}

	if startTime.After(endTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	stats, err := h.opsService.GetWindowStats(c.Request.Context(), startTime, endTime)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get error stats")
		return
	}

	response.Success(c, gin.H{
		"success_count":   stats.SuccessCount,
		"error_count":     stats.ErrorCount,
		"error_4xx_count": stats.Error4xxCount,
		"error_5xx_count": stats.Error5xxCount,
		"timeout_count":   stats.TimeoutCount,
		"p50_latency_ms":  stats.P50LatencyMs,
		"p95_latency_ms":  stats.P95LatencyMs,
		"p99_latency_ms":  stats.P99LatencyMs,
		"avg_latency_ms":  stats.AvgLatencyMs,
	})
}

// GetAccountStatus returns account status information.
// GET /api/v1/admin/ops/account-status
//
// Query params:
// - account_id: int64 (optional; if not provided, returns all active accounts)
func (h *OpsMonitoringHandler) GetAccountStatus(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	if accountIDStr == "" {
		response.BadRequest(c, "account_id is required")
		return
	}

	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account_id")
		return
	}

	stats1h, err := h.opsService.GetAccountStats(c.Request.Context(), accountID, time.Hour)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get account stats")
		return
	}

	stats24h, err := h.opsService.GetAccountStats(c.Request.Context(), accountID, 24*time.Hour)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get account stats")
		return
	}

	response.Success(c, gin.H{
		"account_id": accountID,
		"stats_1h": gin.H{
			"error_count":      stats1h.ErrorCount,
			"success_count":    stats1h.SuccessCount,
			"timeout_count":    stats1h.TimeoutCount,
			"rate_limit_count": stats1h.RateLimitCount,
		},
		"stats_24h": gin.H{
			"error_count":      stats24h.ErrorCount,
			"success_count":    stats24h.SuccessCount,
			"timeout_count":    stats24h.TimeoutCount,
			"rate_limit_count": stats24h.RateLimitCount,
		},
	})
}

// GetErrorTimeseries returns time-series error data for charts.
// GET /api/v1/admin/ops/error-timeseries
//
// Query params:
// - start_time: RFC3339 timestamp (optional; default: 24h ago)
// - end_time: RFC3339 timestamp (optional; default: now)
// - interval: string (optional; one of: 1m, 5m, 1h; default: 5m)
func (h *OpsMonitoringHandler) GetErrorTimeseries(c *gin.Context) {
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid start_time format (RFC3339)")
			return
		}
		startTime = parsed
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid end_time format (RFC3339)")
			return
		}
		endTime = parsed
	}

	if startTime.After(endTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	windowMinutes := 5
	if intervalStr := c.Query("interval"); intervalStr != "" {
		switch intervalStr {
		case "1m":
			windowMinutes = 1
		case "5m":
			windowMinutes = 5
		case "1h":
			windowMinutes = 60
		default:
			response.BadRequest(c, "Invalid interval (supported: 1m, 5m, 1h)")
			return
		}
	}

	limit := int(endTime.Sub(startTime).Minutes()/float64(windowMinutes)) + 10
	if limit > 5000 {
		limit = 5000
	}

	items, err := h.opsService.ListMetricsHistory(c.Request.Context(), windowMinutes, startTime, endTime, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get error timeseries")
		return
	}

	response.Success(c, gin.H{"items": items})
}
