package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

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
// - limit: int (optional; max 1000, default 300 for backward compatibility)
// Note: Maximum time range is 7 days
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
		if err != nil || parsed <= 0 || parsed > 1000 {
			response.BadRequest(c, "Invalid limit (must be 1-1000)")
			return
		}
		limit = parsed
		limitProvided = true
	}

	endTime := time.Now()
	startTime := time.Time{}
	parsedStart, parsedEnd, err := parseTimeRangeRFC3339(c)
	if err != nil {
		return
	}
	if err := validateTimeRangeOrderIfPresent(c, parsedStart, parsedEnd); err != nil {
		return
	}
	if !parsedStart.IsZero() {
		startTime = parsedStart
	}
	if !parsedEnd.IsZero() {
		endTime = parsedEnd
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
			limit = 1000
		}
	}

	// Enforce maximum time range of 7 days
	maxRange := 7 * 24 * time.Hour
	if endTime.Sub(startTime) > maxRange {
		response.BadRequest(c, "Time range exceeds maximum of 7 days")
		return
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
