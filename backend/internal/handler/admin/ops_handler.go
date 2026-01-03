package admin

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OpsHandler handles ops dashboard endpoints.
type OpsHandler struct {
	opsService *service.OpsService
}

// NewOpsHandler creates a new OpsHandler.
func NewOpsHandler(opsService *service.OpsService) *OpsHandler {
	return &OpsHandler{opsService: opsService}
}

// GetMetrics returns the latest ops metrics snapshot.
// GET /api/v1/admin/ops/metrics
func (h *OpsHandler) GetMetrics(c *gin.Context) {
	metrics, err := h.opsService.GetLatestMetrics(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get ops metrics")
		return
	}
	response.Success(c, metrics)
}

// ListMetricsHistory returns a time-range slice of metrics for charts.
// GET /api/v1/admin/ops/metrics/history
//
// Query params:
// - window_minutes: int (default 1)
// - minutes: int (lookback; optional)
// - start_time/end_time: RFC3339 timestamps (optional; overrides minutes when provided)
// - limit: int (optional; max 100, default 300 for backward compatibility)
func (h *OpsHandler) ListMetricsHistory(c *gin.Context) {
	windowMinutes := 1
	if v := c.Query("window_minutes"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			windowMinutes = parsed
		} else {
			response.BadRequest(c, "Invalid window_minutes")
			return
		}
	}
	validWindows := map[int]bool{1: true, 5: true, 60: true}
	if !validWindows[windowMinutes] {
		response.BadRequest(c, "Invalid window_minutes (supported: 1, 5, 60)")
		return
	}

	limit := 300
	limitProvided := false
	if v := c.Query("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 || parsed > 5000 {
			response.BadRequest(c, "Invalid limit (must be 1-5000)")
			return
		}
		limit = parsed
		limitProvided = true
	}

	endTime := time.Now()
	startTime := time.Time{}

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

	// If explicit range not provided, use lookback minutes.
	if startTime.IsZero() {
		if v := c.Query("minutes"); v != "" {
			minutes, err := strconv.Atoi(v)
			if err != nil || minutes <= 0 {
				response.BadRequest(c, "Invalid minutes")
				return
			}
			if minutes > 60*24*7 {
				minutes = 60 * 24 * 7
			}
			startTime = endTime.Add(-time.Duration(minutes) * time.Minute)
		}
	}

	// Default time range: last 24 hours.
	if startTime.IsZero() {
		startTime = endTime.Add(-24 * time.Hour)
		if !limitProvided {
			// Metrics are collected at 1-minute cadence; 24h requires ~1440 points.
			limit = 24 * 60
		}
	}

	if startTime.After(endTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	items, err := h.opsService.ListMetricsHistory(c.Request.Context(), windowMinutes, startTime, endTime, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list ops metrics history")
		return
	}
	response.Success(c, gin.H{"items": items})
}

// ListErrorLogs lists recent error logs with optional filters.
// GET /api/v1/admin/ops/error-logs
//
// Query params:
// - start_time/end_time: RFC3339 timestamps (optional)
// - platform: string (optional)
// - phase: string (optional)
// - severity: string (optional)
// - q: string (optional; fuzzy match)
// - limit: int (optional; default 100; max 500)
func (h *OpsHandler) ListErrorLogs(c *gin.Context) {
	var filters service.OpsErrorLogFilters

	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid start_time format (RFC3339)")
			return
		}
		filters.StartTime = &startTime
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid end_time format (RFC3339)")
			return
		}
		filters.EndTime = &endTime
	}

	if filters.StartTime != nil && filters.EndTime != nil && filters.StartTime.After(*filters.EndTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	filters.Platform = c.Query("platform")
	filters.Phase = c.Query("phase")
	filters.Severity = c.Query("severity")
	filters.Query = c.Query("q")

	filters.Limit = 100
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 500 {
			response.BadRequest(c, "Invalid limit (must be 1-500)")
			return
		}
		filters.Limit = limit
	}

	items, total, err := h.opsService.ListErrorLogs(c.Request.Context(), filters)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list error logs")
		return
	}

	response.Success(c, gin.H{
		"items": items,
		"total": total,
	})
}

// GetDashboardOverview returns realtime ops dashboard overview.
// GET /api/v1/admin/ops/dashboard/overview
//
// Query params:
// - time_range: string (optional; default "1h") one of: 5m, 30m, 1h, 6h, 24h
func (h *OpsHandler) GetDashboardOverview(c *gin.Context) {
	timeRange := c.Query("time_range")
	if timeRange == "" {
		timeRange = "1h"
	}

	switch timeRange {
	case "5m", "30m", "1h", "6h", "24h":
	default:
		response.BadRequest(c, "Invalid time_range (supported: 5m, 30m, 1h, 6h, 24h)")
		return
	}

	data, err := h.opsService.GetDashboardOverview(c.Request.Context(), timeRange)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get dashboard overview")
		return
	}
	response.Success(c, data)
}

// GetProviderHealth returns upstream provider health comparison data.
// GET /api/v1/admin/ops/dashboard/providers
//
// Query params:
// - time_range: string (optional; default "1h") one of: 5m, 30m, 1h, 6h, 24h
func (h *OpsHandler) GetProviderHealth(c *gin.Context) {
	timeRange := c.Query("time_range")
	if timeRange == "" {
		timeRange = "1h"
	}

	switch timeRange {
	case "5m", "30m", "1h", "6h", "24h":
	default:
		response.BadRequest(c, "Invalid time_range (supported: 5m, 30m, 1h, 6h, 24h)")
		return
	}

	providers, err := h.opsService.GetProviderHealth(c.Request.Context(), timeRange)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get provider health")
		return
	}

	var totalRequests int64
	var weightedSuccess float64
	var bestProvider string
	var worstProvider string
	var bestRate float64
	var worstRate float64
	hasRate := false

	for _, p := range providers {
		if p == nil {
			continue
		}
		totalRequests += p.RequestCount
		weightedSuccess += (p.SuccessRate / 100) * float64(p.RequestCount)

		if p.RequestCount <= 0 {
			continue
		}
		if !hasRate {
			bestProvider = p.Name
			worstProvider = p.Name
			bestRate = p.SuccessRate
			worstRate = p.SuccessRate
			hasRate = true
			continue
		}

		if p.SuccessRate > bestRate {
			bestProvider = p.Name
			bestRate = p.SuccessRate
		}
		if p.SuccessRate < worstRate {
			worstProvider = p.Name
			worstRate = p.SuccessRate
		}
	}

	avgSuccessRate := 0.0
	if totalRequests > 0 {
		avgSuccessRate = (weightedSuccess / float64(totalRequests)) * 100
		avgSuccessRate = math.Round(avgSuccessRate*100) / 100
	}

	response.Success(c, gin.H{
		"providers": providers,
		"summary": gin.H{
			"total_requests":   totalRequests,
			"avg_success_rate": avgSuccessRate,
			"best_provider":    bestProvider,
			"worst_provider":   worstProvider,
		},
	})
}

// GetErrorLogs returns a paginated error log list with multi-dimensional filters.
// GET /api/v1/admin/ops/errors
func (h *OpsHandler) GetErrorLogs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)

	filter := &service.ErrorLogFilter{
		Page:     page,
		PageSize: pageSize,
	}

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

	if errorCodeStr := c.Query("error_code"); errorCodeStr != "" {
		code, err := strconv.Atoi(errorCodeStr)
		if err != nil || code < 0 {
			response.BadRequest(c, "Invalid error_code")
			return
		}
		filter.ErrorCode = &code
	}

	// Query parameter uses "platform" (consistent with database field naming)
	filter.Provider = c.Query("platform")

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
		if err != nil || accountID <= 0 {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		filter.AccountID = &accountID
	}

	out, err := h.opsService.GetErrorLogs(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get error logs")
		return
	}

	response.Success(c, gin.H{
		"errors":    out.Errors,
		"total":     out.Total,
		"page":      out.Page,
		"page_size": out.PageSize,
	})
}

// GetLatencyHistogram returns the latency distribution histogram.
// GET /api/v1/admin/ops/dashboard/latency-histogram
func (h *OpsHandler) GetLatencyHistogram(c *gin.Context) {
	timeRange := c.Query("time_range")
	if timeRange == "" {
		timeRange = "1h"
	}

	buckets, err := h.opsService.GetLatencyHistogram(c.Request.Context(), timeRange)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get latency histogram")
		return
	}

	totalRequests := int64(0)
	for _, b := range buckets {
		totalRequests += b.Count
	}

	response.Success(c, gin.H{
		"buckets":                buckets,
		"total_requests":         totalRequests,
		"slow_request_threshold": 1000,
	})
}

// GetErrorDistribution returns the error distribution.
// GET /api/v1/admin/ops/dashboard/errors/distribution
func (h *OpsHandler) GetErrorDistribution(c *gin.Context) {
	timeRange := c.Query("time_range")
	if timeRange == "" {
		timeRange = "1h"
	}

	items, err := h.opsService.GetErrorDistribution(c.Request.Context(), timeRange)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get error distribution")
		return
	}

	response.Success(c, gin.H{
		"items": items,
	})
}

// GetErrorDetail returns detailed information for a specific error log by ID.
// GET /api/v1/admin/ops/errors/:id
func (h *OpsHandler) GetErrorDetail(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		response.BadRequest(c, "Error ID is required")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid error ID")
		return
	}

	errorLog, err := h.opsService.GetErrorLogByID(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(c, http.StatusNotFound, "Error log not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to get error detail")
		}
		return
	}

	response.Success(c, errorLog)
}

// GetErrorStats returns error statistics for a time window.
// GET /api/v1/admin/ops/error-stats
//
// Query params:
// - start_time: RFC3339 timestamp (optional; default: 24h ago)
// - end_time: RFC3339 timestamp (optional; default: now)
func (h *OpsHandler) GetErrorStats(c *gin.Context) {
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
func (h *OpsHandler) GetAccountStatus(c *gin.Context) {
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
func (h *OpsHandler) GetErrorTimeseries(c *gin.Context) {
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

// GetErrorStatsByIP returns error statistics aggregated by client IP.
// GET /api/v1/admin/ops/errors/by-ip
//
// Query params:
// - start_time: RFC3339 timestamp (required)
// - end_time: RFC3339 timestamp (required)
// - limit: int (optional; default 50; max 200)
// - sort_by: string (optional; error_count or last_error_time; default error_count)
// - sort_order: string (optional; asc or desc; default desc)
func (h *OpsHandler) GetErrorStatsByIP(c *gin.Context) {
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")
	if startTimeStr == "" || endTimeStr == "" {
		response.BadRequest(c, "start_time and end_time are required")
		return
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		response.BadRequest(c, "Invalid start_time format (RFC3339)")
		return
	}
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		response.BadRequest(c, "Invalid end_time format (RFC3339)")
		return
	}

	if startTime.After(endTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l <= 0 || l > 200 {
			response.BadRequest(c, "Invalid limit (must be 1-200)")
			return
		}
		limit = l
	}

	sortBy := c.DefaultQuery("sort_by", "error_count")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	stats, err := h.opsService.GetErrorStatsByIP(c.Request.Context(), startTime, endTime, limit, sortBy, sortOrder)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get IP error statistics")
		return
	}

	response.Success(c, gin.H{
		"total": len(stats),
		"data":  stats,
	})
}

// GetErrorsByIP returns error details for a specific IP.
// GET /api/v1/admin/ops/errors/by-ip/:ip
//
// Query params:
// - start_time: RFC3339 timestamp (required)
// - end_time: RFC3339 timestamp (required)
// - page: int (optional; default 1)
// - page_size: int (optional; default 50; max 100)
func (h *OpsHandler) GetErrorsByIP(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		response.BadRequest(c, "IP address is required")
		return
	}
	if net.ParseIP(ip) == nil {
		response.BadRequest(c, "Invalid IP address format")
		return
	}

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")
	if startTimeStr == "" || endTimeStr == "" {
		response.BadRequest(c, "start_time and end_time are required")
		return
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		response.BadRequest(c, "Invalid start_time format (RFC3339)")
		return
	}
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		response.BadRequest(c, "Invalid end_time format (RFC3339)")
		return
	}

	if startTime.After(endTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p <= 0 {
			response.BadRequest(c, "Invalid page")
			return
		}
		page = p
	}

	pageSize := 50
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		ps, err := strconv.Atoi(pageSizeStr)
		if err != nil || ps <= 0 || ps > 100 {
			response.BadRequest(c, "Invalid page_size (must be 1-100)")
			return
		}
		pageSize = ps
	}

	errors, total, err := h.opsService.GetErrorsByIP(c.Request.Context(), ip, startTime, endTime, page, pageSize)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get errors by IP")
		return
	}

	response.Success(c, gin.H{
		"ip":        ip,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"errors":    errors,
	})
}

// ListAlertRules returns all alert rules.
// GET /api/v1/admin/ops/alert-rules
func (h *OpsHandler) ListAlertRules(c *gin.Context) {
	rules, err := h.opsService.ListAlertRules(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list alert rules")
		return
	}
	response.Success(c, rules)
}

// CreateAlertRule creates a new alert rule.
// POST /api/v1/admin/ops/alert-rules
func (h *OpsHandler) CreateAlertRule(c *gin.Context) {
	var rule service.OpsAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.opsService.CreateAlertRule(c.Request.Context(), &rule); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create alert rule")
		return
	}
	response.Success(c, rule)
}

// UpdateAlertRule updates an existing alert rule.
// PUT /api/v1/admin/ops/alert-rules/:id
func (h *OpsHandler) UpdateAlertRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid rule ID")
		return
	}
	var rule service.OpsAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	rule.ID = id
	if err := h.opsService.UpdateAlertRule(c.Request.Context(), &rule); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update alert rule")
		return
	}
	response.Success(c, rule)
}

// DeleteAlertRule deletes an alert rule.
// DELETE /api/v1/admin/ops/alert-rules/:id
func (h *OpsHandler) DeleteAlertRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid rule ID")
		return
	}
	if err := h.opsService.DeleteAlertRule(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete alert rule")
		return
	}
	response.Success(c, nil)
}

// ListAlertEvents returns alert event history.
// GET /api/v1/admin/ops/alert-events
func (h *OpsHandler) ListAlertEvents(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	events, err := h.opsService.ListAlertEvents(c.Request.Context(), limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list alert events")
		return
	}
	response.Success(c, events)
}
