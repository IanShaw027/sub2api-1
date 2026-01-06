package admin

import (
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const supportedDashboardTimeRangesMessage = "Invalid time_range (supported: 5m, 30m, 1h, 6h, 24h)"

var dashboardTimeRangeDurations = map[string]time.Duration{
	"5m":  5 * time.Minute,
	"30m": 30 * time.Minute,
	"1h":  1 * time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
}

func parseDashboardTimeRangeParam(c *gin.Context, defaultValue string) (string, time.Duration, error) {
	value := strings.TrimSpace(c.Query("time_range"))
	if value == "" {
		value = strings.TrimSpace(defaultValue)
	}
	if value == "" {
		value = "1h"
	}

	dur, ok := dashboardTimeRangeDurations[value]
	if !ok {
		response.BadRequest(c, supportedDashboardTimeRangesMessage)
		return "", 0, errors.New("invalid time_range")
	}
	return value, dur, nil
}

func validateTimeRangeOrderIfPresent(c *gin.Context, startTime, endTime time.Time) error {
	if startTime.IsZero() || endTime.IsZero() {
		return nil
	}
	if startTime.After(endTime) {
		response.BadRequest(c, "Invalid time range: start_time must be <= end_time")
		return errors.New("invalid time range")
	}
	return nil
}

// parseTimeRangeRFC3339 parses start_time and end_time query parameters from gin.Context.
// Returns (startTime, endTime, error). If error is not nil, an HTTP error response
// has already been sent to the client.
func parseTimeRangeRFC3339(c *gin.Context) (time.Time, time.Time, error) {
	var startTime, endTime time.Time

	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid start_time format (RFC3339)")
			return time.Time{}, time.Time{}, err
		}
		startTime = parsed
	}

	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			response.BadRequest(c, "Invalid end_time format (RFC3339)")
			return time.Time{}, time.Time{}, err
		}
		endTime = parsed
	}

	return startTime, endTime, nil
}

func requireTimeRangeRFC3339(c *gin.Context) (time.Time, time.Time, error) {
	startTimeStr := strings.TrimSpace(c.Query("start_time"))
	endTimeStr := strings.TrimSpace(c.Query("end_time"))
	if startTimeStr == "" || endTimeStr == "" {
		response.BadRequest(c, "start_time and end_time are required")
		return time.Time{}, time.Time{}, errors.New("start_time and end_time are required")
	}

	startTime, endTime, err := parseTimeRangeRFC3339(c)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if startTime.IsZero() || endTime.IsZero() {
		response.BadRequest(c, "start_time and end_time are required")
		return time.Time{}, time.Time{}, errors.New("start_time and end_time are required")
	}

	if err := validateTimeRangeOrderIfPresent(c, startTime, endTime); err != nil {
		return time.Time{}, time.Time{}, err
	}
	return startTime, endTime, nil
}
