package admin

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetErrorLogs returns a paginated error log list with multi-dimensional filters.
// GET /api/v1/admin/ops/errors
func (h *OpsHandler) GetErrorLogs(c *gin.Context) {
	page := 1
	if p := strings.TrimSpace(c.Query("page")); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 20
	if ps := strings.TrimSpace(c.Query("page_size")); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 500 {
			pageSize = parsed
		} else {
			response.BadRequest(c, "Invalid page_size (must be 1-500)")
			return
		}
	} else if l := strings.TrimSpace(c.Query("limit")); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			pageSize = parsed
		} else {
			response.BadRequest(c, "Invalid limit (must be 1-500)")
			return
		}
	}

	filter := &service.ErrorLogFilter{
		Page:     page,
		PageSize: pageSize,
	}

	startTime, endTime, err := parseTimeRangeRFC3339(c)
	if err != nil {
		return
	}
	if err := validateTimeRangeOrderIfPresent(c, startTime, endTime); err != nil {
		return
	}
	if !startTime.IsZero() {
		filter.StartTime = &startTime
	}
	if !endTime.IsZero() {
		filter.EndTime = &endTime
	}

	if errorCodeStr := c.Query("error_code"); errorCodeStr != "" {
		code, err := strconv.Atoi(errorCodeStr)
		if err != nil || code < 0 {
			response.BadRequest(c, "Invalid error_code")
			return
		}
		filter.ErrorCode = &code
	}

	// Query parameter uses "platform" or "platforms" (consistent with database field naming).
	// Backwards compatibility: older clients used "provider" for the same filter.
	// Support both single-value "platform" and multi-value "platforms" (comma-separated).
	platforms := c.Query("platforms")
	if platforms == "" {
		platform := c.Query("platform")
		if platform == "" {
			platform = c.Query("provider")
		}
		if platform != "" {
			// Single-value backwards compatibility: add to Platforms array
			filter.Platforms = []string{platform}
			filter.Provider = platform
		}
	} else {
		// Multi-value support: parse comma-separated string
		platformList := strings.Split(strings.TrimSpace(platforms), ",")
		filter.Platforms = make([]string, 0, len(platformList))
		for _, p := range platformList {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				filter.Platforms = append(filter.Platforms, trimmed)
			}
		}
	}

	// Parse status_codes: comma-separated integers (e.g., "500,502,503")
	if statusCodesStr := c.Query("status_codes"); statusCodesStr != "" {
		codeStrs := strings.Split(strings.TrimSpace(statusCodesStr), ",")
		filter.StatusCodes = make([]int, 0, len(codeStrs))
		for _, codeStr := range codeStrs {
			if trimmed := strings.TrimSpace(codeStr); trimmed != "" {
				code, err := strconv.Atoi(trimmed)
				if err != nil || code < 0 {
					response.BadRequest(c, fmt.Sprintf("Invalid status_codes: %q is not a valid integer", trimmed))
					return
				}
				filter.StatusCodes = append(filter.StatusCodes, code)
			}
		}
	}

	// Parse client_ip: single IP address string
	if clientIP := strings.TrimSpace(c.Query("client_ip")); clientIP != "" {
		filter.ClientIP = clientIP
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
		if err != nil || accountID <= 0 {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		filter.AccountID = &accountID
	}

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil || groupID <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		filter.GroupID = &groupID
	}

	if phase := strings.TrimSpace(c.Query("phase")); phase != "" {
		filter.Phase = phase
	}
	if severity := strings.TrimSpace(c.Query("severity")); severity != "" {
		filter.Severity = severity
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		filter.Query = q
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

// ListRequestDetails returns a request-level list (success + error) for metric drill-down.
// GET /api/v1/admin/ops/requests
//
// Query params:
// - start_time/end_time: RFC3339 timestamps (optional; default last 1h)
// - time_range: string (optional; one of: 5m, 30m, 1h, 6h, 24h; used when start/end not provided)
// - kind: string (optional; one of: success, error, all)
// - platform/platforms: string (optional; platforms is comma-separated)
// - user_id/api_key_id/account_id/group_id: int64 (optional)
// - model: string (optional; exact match)
// - request_id: string (optional; exact match)
// - q: string (optional; fuzzy match against request_id/model/message)
// - min_duration_ms/max_duration_ms: int (optional)
// - sort: string (optional; one of: created_at_desc, duration_desc)
// - page/page_size: pagination (page_size max 100)
func (h *OpsHandler) ListRequestDetails(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)

	filter := &service.OpsRequestDetailFilter{
		Page:     page,
		PageSize: pageSize,
	}

	var hasExplicitRange bool
	startTime, endTime, err := parseTimeRangeRFC3339(c)
	if err != nil {
		return
	}
	if err := validateTimeRangeOrderIfPresent(c, startTime, endTime); err != nil {
		return
	}
	if !startTime.IsZero() {
		filter.StartTime = &startTime
		hasExplicitRange = true
	}
	if !endTime.IsZero() {
		filter.EndTime = &endTime
		hasExplicitRange = true
	}

	if !hasExplicitRange {
		timeRange, dur, err := parseDashboardTimeRangeParam(c, "1h")
		if err != nil {
			return
		}
		_ = timeRange

		end := time.Now()
		start := end.Add(-dur)
		filter.StartTime = &start
		filter.EndTime = &end
	}

	if filter.StartTime != nil && filter.EndTime != nil && filter.StartTime.After(*filter.EndTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	filter.Kind = strings.TrimSpace(c.Query("kind"))
	filter.Model = strings.TrimSpace(c.Query("model"))
	filter.RequestID = strings.TrimSpace(c.Query("request_id"))
	filter.Query = strings.TrimSpace(c.Query("q"))
	filter.Sort = strings.TrimSpace(c.Query("sort"))

	// Platforms (platform/platforms)
	platforms := strings.TrimSpace(c.Query("platforms"))
	if platforms == "" {
		if p := strings.TrimSpace(c.Query("platform")); p != "" {
			filter.Platforms = []string{p}
		}
	} else {
		parts := strings.Split(platforms, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		filter.Platforms = out
	}

	// IDs
	if v := strings.TrimSpace(c.Query("user_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		filter.UserID = &id
	}
	if v := strings.TrimSpace(c.Query("api_key_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		filter.APIKeyID = &id
	}
	if v := strings.TrimSpace(c.Query("account_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		filter.AccountID = &id
	}
	if v := strings.TrimSpace(c.Query("group_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		filter.GroupID = &id
	}

	// Duration filters
	if v := strings.TrimSpace(c.Query("min_duration_ms")); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			response.BadRequest(c, "Invalid min_duration_ms")
			return
		}
		filter.MinDurationMs = &parsed
	}
	if v := strings.TrimSpace(c.Query("max_duration_ms")); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			response.BadRequest(c, "Invalid max_duration_ms")
			return
		}
		filter.MaxDurationMs = &parsed
	}

	out, err := h.opsService.ListRequestDetails(c.Request.Context(), filter)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid") {
			response.BadRequest(c, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to list request details")
		return
	}

	response.Paginated(c, out.Items, out.Total, out.Page, out.PageSize)
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
// - group_by: string (optional; one of: platform, phase, severity; default: global summary)
func (h *OpsHandler) GetErrorStats(c *gin.Context) {
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	parsedStart, parsedEnd, err := parseTimeRangeRFC3339(c)
	if err != nil {
		return
	}
	if !parsedStart.IsZero() {
		startTime = parsedStart
	}
	if !parsedEnd.IsZero() {
		endTime = parsedEnd
	}

	if startTime.After(endTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return
	}

	groupBy := strings.TrimSpace(strings.ToLower(c.Query("group_by")))
	switch groupBy {
	case "", "platform", "phase", "severity":
		// ok
	default:
		response.BadRequest(c, "Invalid group_by (supported: platform, phase, severity)")
		return
	}

	if groupBy != "" {
		items, err := h.opsService.GetWindowStatsGrouped(c.Request.Context(), startTime, endTime, groupBy)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to get error stats")
			return
		}

		response.Success(c, gin.H{
			"group_by": groupBy,
			"items":    items,
		})
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
	accountIDStr := strings.TrimSpace(c.Query("account_id"))
	if accountIDStr == "" {
		items, err := h.opsService.GetAllActiveAccountStatus(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to get account stats")
			return
		}

		response.Success(c, gin.H{
			"accounts": items,
			"total":    len(items),
		})
		return
	}

	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account_id")
		return
	}

	// Get all active accounts and find the requested one
	items, err := h.opsService.GetAllActiveAccountStatus(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get account stats")
		return
	}

	var found *service.AccountStatusSummary
	for i := range items {
		if items[i].AccountID == accountID {
			found = &items[i]
			break
		}
	}

	if found == nil {
		// Return empty stats for accounts with no recent activity
		response.Success(c, gin.H{
			"account_id": accountID,
			"stats_1h": gin.H{
				"error_count":      0,
				"success_count":    0,
				"timeout_count":    0,
				"rate_limit_count": 0,
			},
			"stats_24h": gin.H{
				"error_count":      0,
				"success_count":    0,
				"timeout_count":    0,
				"rate_limit_count": 0,
			},
		})
		return
	}

	response.Success(c, gin.H{
		"account_id": accountID,
		"stats_1h": gin.H{
			"error_count":      found.Stats1h.ErrorCount,
			"success_count":    found.Stats1h.SuccessCount,
			"timeout_count":    found.Stats1h.TimeoutCount,
			"rate_limit_count": found.Stats1h.RateLimitCount,
		},
		"stats_24h": gin.H{
			"error_count":      found.Stats24h.ErrorCount,
			"success_count":    found.Stats24h.SuccessCount,
			"timeout_count":    found.Stats24h.TimeoutCount,
			"rate_limit_count": found.Stats24h.RateLimitCount,
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

	parsedStart, parsedEnd, err := parseTimeRangeRFC3339(c)
	if err != nil {
		return
	}
	if !parsedStart.IsZero() {
		startTime = parsedStart
	}
	if !parsedEnd.IsZero() {
		endTime = parsedEnd
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
	startTime, endTime, err := requireTimeRangeRFC3339(c)
	if err != nil {
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

	startTime, endTime, err := requireTimeRangeRFC3339(c)
	if err != nil {
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

// RetryErrorRequest retries a failed request to verify if the issue persists.
// POST /api/v1/admin/ops/errors/:id/retry
func (h *OpsHandler) RetryErrorRequest(c *gin.Context) {
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

	// Get error log details
	errorLog, err := h.opsService.GetErrorLogByID(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(c, http.StatusNotFound, "Error log not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to get error detail")
		}
		return
	}

	// Check if we have request body to retry
	if errorLog.RequestBody == "" {
		response.BadRequest(c, "No request body found to retry")
		return
	}

	// Return retry information for now
	// In a full implementation, this would actually retry the request
	response.Success(c, gin.H{
		"can_retry":    true,
		"request_id":   errorLog.RequestID,
		"platform":     errorLog.Platform,
		"model":        errorLog.Model,
		"request_body": errorLog.RequestBody,
		"message":      "Retry information retrieved successfully. Please use the client to retry the request manually.",
	})
}
