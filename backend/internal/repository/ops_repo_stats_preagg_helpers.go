package repository

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var errOpsPreaggregatedNotPopulated = errors.New("ops pre-aggregated tables not populated")

const opsMetricsPreaggHourlyOnlyMaxRange = 24 * time.Hour

type opsAggSummary struct {
	requestCount int64
	successCount int64
	errorCount   int64

	error4xxCount int64
	error5xxCount int64
	timeoutCount  int64

	avgLatencyWeightedSum float64
	avgLatencyWeight      int64
	p99LatencyMax         float64
}

type providerStatsAgg struct {
	requestCount int64
	successCount int64
	errorCount   int64

	error4xxCount int64
	error5xxCount int64
	timeoutCount  int64

	avgLatencyWeightedSum float64
	avgLatencyWeight      int64
	p99LatencyMax         float64
}

func normalizeTimeRange(startTime, endTime time.Time) (time.Time, time.Time) {
	if startTime.After(endTime) {
		return endTime, startTime
	}
	return startTime, endTime
}

func preaggUseHourlyOnly(startTime, endTime time.Time) bool {
	return endTime.Sub(startTime) <= opsMetricsPreaggHourlyOnlyMaxRange
}

func utcCeilToHour(t time.Time) time.Time {
	u := t.UTC()
	f := u.Truncate(time.Hour)
	if f.Equal(u) {
		return f
	}
	return f.Add(time.Hour)
}

func utcFloorToHour(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}

func utcCeilToDay(t time.Time) time.Time {
	u := t.UTC()
	y, m, d := u.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if day.Equal(u) {
		return day
	}
	return day.Add(24 * time.Hour)
}

func utcFloorToDay(t time.Time) time.Time {
	u := t.UTC()
	y, m, d := u.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (r *OpsRepository) preaggSafeEnd(endTime time.Time) time.Time {
	now := time.Now().UTC()
	cutoff := now.Add(-opsMetricsPreaggFreshnessLag)
	if endTime.After(cutoff) {
		return cutoff
	}
	return endTime
}

func (r *OpsRepository) rawOpsDataExists(ctx context.Context, startTime, endTime time.Time) (bool, error) {
	var exists bool
	err := scanSingleRow(
		ctx,
		r.sql,
		`
			SELECT
				EXISTS (SELECT 1 FROM usage_logs WHERE created_at >= $1 AND created_at < $2)
				OR EXISTS (SELECT 1 FROM ops_error_logs WHERE created_at >= $1 AND created_at < $2)
		`,
		[]any{startTime, endTime},
		&exists,
	)
	return exists, err
}

func sortProviderStats(items []*service.ProviderStats) {
	if len(items) <= 1 {
		return
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		return items[i].Platform < items[j].Platform
	})
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
