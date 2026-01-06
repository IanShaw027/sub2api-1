package service

import (
	"errors"
	"strings"
	"time"
)

func parseTimeRange(timeRange string) (time.Duration, error) {
	value := strings.TrimSpace(timeRange)
	if value == "" {
		return 0, errors.New("invalid time range")
	}

	// Support "7d" style day ranges for convenience.
	if strings.HasSuffix(value, "d") {
		numberPart := strings.TrimSuffix(value, "d")
		if numberPart == "" {
			return 0, errors.New("invalid time range")
		}
		days := 0
		for _, ch := range numberPart {
			if ch < '0' || ch > '9' {
				return 0, errors.New("invalid time range")
			}
			days = days*10 + int(ch-'0')
		}
		if days <= 0 {
			return 0, errors.New("invalid time range")
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	dur, err := time.ParseDuration(value)
	if err != nil || dur <= 0 {
		return 0, errors.New("invalid time range")
	}

	// Cap to avoid unbounded queries.
	const maxWindow = 30 * 24 * time.Hour
	if dur > maxWindow {
		dur = maxWindow
	}

	return dur, nil
}
