//go:build integration

package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpsAggregation_HourlyAndDaily_AreCorrect_AndUpsertsIdempotent(t *testing.T) {
	db := opsITDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userID := insertUser(t, ctx, db)
	openAIGroup := insertGroup(t, ctx, db, "openai")
	anthropicGroup := insertGroup(t, ctx, db, "anthropic")
	openAIAccount := insertAccount(t, ctx, db, "openai")
	anthropicAccount := insertAccount(t, ctx, db, "anthropic")
	openAIKey := insertAPIKey(t, ctx, db, userID, openAIGroup)
	anthropicKey := insertAPIKey(t, ctx, db, userID, anthropicGroup)

	// Pick a stable hour that won't straddle a UTC day boundary for the next 2 hours.
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	hour0 := dayStart.Add(10 * time.Hour)
	if hour0.After(now) {
		hour0 = hour0.Add(-24 * time.Hour)
	}
	if hour0.Add(2 * time.Hour).After(now) {
		hour0 = hour0.Add(-24 * time.Hour)
	}
	start := hour0
	end := hour0.Add(2 * time.Hour)

	insertUsageN(t, ctx, db, userID, openAIKey, openAIAccount, openAIGroup, 100, 10, hour0.Add(10*time.Minute))
	insertUsageN(t, ctx, db, userID, openAIKey, openAIAccount, openAIGroup, 100, 20, hour0.Add(70*time.Minute))
	insertUsageN(t, ctx, db, userID, anthropicKey, anthropicAccount, anthropicGroup, 200, 5, hour0.Add(20*time.Minute))
	insertUsageN(t, ctx, db, userID, anthropicKey, anthropicAccount, anthropicGroup, 200, 5, hour0.Add(80*time.Minute))

	insertError(t, ctx, db, openAIAccount, openAIGroup, "timeout", 504, hour0.Add(30*time.Minute), "")
	insertError(t, ctx, db, openAIAccount, openAIGroup, "upstream_error", 500, hour0.Add(40*time.Minute), "")
	insertError(t, ctx, db, openAIAccount, openAIGroup, "rate_limit", 429, hour0.Add(90*time.Minute), "")
	insertError(t, ctx, db, anthropicAccount, anthropicGroup, "bad_request", 404, hour0.Add(15*time.Minute), "")
	insertError(t, ctx, db, anthropicAccount, anthropicGroup, "upstream_error", 500, hour0.Add(95*time.Minute), "")

	opsRepo := newOpsRepoWithPreagg(t, db)

	require.NoError(t, opsRepo.UpsertHourlyMetrics(ctx, start, end))
	gotHourly := fetchHourlyAgg(t, ctx, db, start, end)

	expectedHourly := map[hourlyKey]hourlyAgg{
		{bucketStart: hour0, platform: "openai"}: {
			requestCount:  12,
			successCount:  10,
			errorCount:    2,
			error4xxCount: 0,
			error5xxCount: 2,
			timeoutCount:  1,
			avgLatencyMs:  100,
			p99LatencyMs:  100,
			errorRate:     16.6667,
		},
		{bucketStart: hour0.Add(1 * time.Hour), platform: "openai"}: {
			requestCount:  21,
			successCount:  20,
			errorCount:    1,
			error4xxCount: 1,
			error5xxCount: 0,
			timeoutCount:  0,
			avgLatencyMs:  100,
			p99LatencyMs:  100,
			errorRate:     4.7619,
		},
		{bucketStart: hour0, platform: "anthropic"}: {
			requestCount:  6,
			successCount:  5,
			errorCount:    1,
			error4xxCount: 1,
			error5xxCount: 0,
			timeoutCount:  0,
			avgLatencyMs:  200,
			p99LatencyMs:  200,
			errorRate:     16.6667,
		},
		{bucketStart: hour0.Add(1 * time.Hour), platform: "anthropic"}: {
			requestCount:  6,
			successCount:  5,
			errorCount:    1,
			error4xxCount: 0,
			error5xxCount: 1,
			timeoutCount:  0,
			avgLatencyMs:  200,
			p99LatencyMs:  200,
			errorRate:     16.6667,
		},
	}
	assertHourlyAgg(t, expectedHourly, gotHourly)

	// Idempotency: running again should not change the aggregated values.
	require.NoError(t, opsRepo.UpsertHourlyMetrics(ctx, start, end))
	gotHourly2 := fetchHourlyAgg(t, ctx, db, start, end)
	assertHourlyAgg(t, expectedHourly, gotHourly2)

	require.NoError(t, opsRepo.UpsertDailyMetrics(ctx, start, end))
	gotDaily := fetchDailyAgg(t, ctx, db, hour0)

	expectedDaily := map[dailyKey]dailyAgg{
		{bucketDate: dayKey(hour0), platform: "openai"}: {
			requestCount:  33,
			successCount:  30,
			errorCount:    3,
			error4xxCount: 1,
			error5xxCount: 2,
			timeoutCount:  1,
			avgLatencyMs:  100,
			p99LatencyMs:  100,
			errorRate:     9.0909,
		},
		{bucketDate: dayKey(hour0), platform: "anthropic"}: {
			requestCount:  12,
			successCount:  10,
			errorCount:    2,
			error4xxCount: 1,
			error5xxCount: 1,
			timeoutCount:  0,
			avgLatencyMs:  200,
			p99LatencyMs:  200,
			errorRate:     16.6667,
		},
	}
	assertDailyAgg(t, expectedDaily, gotDaily)

	// Idempotency: running again should not change the aggregated values.
	require.NoError(t, opsRepo.UpsertDailyMetrics(ctx, start, end))
	gotDaily2 := fetchDailyAgg(t, ctx, db, hour0)
	assertDailyAgg(t, expectedDaily, gotDaily2)
}

type hourlyKey struct {
	bucketStart time.Time
	platform    string
}

type hourlyAgg struct {
	requestCount  int64
	successCount  int64
	errorCount    int64
	error4xxCount int64
	error5xxCount int64
	timeoutCount  int64

	avgLatencyMs float64
	p99LatencyMs float64
	errorRate    float64
}

func fetchHourlyAgg(t *testing.T, ctx context.Context, db *sql.DB, start, end time.Time) map[hourlyKey]hourlyAgg {
	t.Helper()

	rows, err := db.QueryContext(ctx, `
		SELECT
			bucket_start,
			platform,
			request_count,
			success_count,
			error_count,
			error_4xx_count,
			error_5xx_count,
			timeout_count,
			COALESCE(avg_latency_ms, 0),
			COALESCE(p99_latency_ms, 0),
			error_rate
		FROM ops_metrics_hourly
		WHERE bucket_start >= $1 AND bucket_start < $2
		ORDER BY bucket_start ASC, platform ASC
	`, start, end)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	out := map[hourlyKey]hourlyAgg{}
	for rows.Next() {
		var (
			bucketStart time.Time
			platform    string
			row         hourlyAgg
		)
		require.NoError(t, rows.Scan(
			&bucketStart,
			&platform,
			&row.requestCount,
			&row.successCount,
			&row.errorCount,
			&row.error4xxCount,
			&row.error5xxCount,
			&row.timeoutCount,
			&row.avgLatencyMs,
			&row.p99LatencyMs,
			&row.errorRate,
		))
		out[hourlyKey{bucketStart: bucketStart.UTC(), platform: platform}] = row
	}
	require.NoError(t, rows.Err())
	return out
}

func assertHourlyAgg(t *testing.T, expected, got map[hourlyKey]hourlyAgg) {
	t.Helper()
	require.Len(t, got, len(expected))
	for k, exp := range expected {
		act, ok := got[k]
		require.True(t, ok, "missing hourly agg row: bucket_start=%s platform=%s", k.bucketStart.Format(time.RFC3339), k.platform)
		require.Equal(t, exp.requestCount, act.requestCount)
		require.Equal(t, exp.successCount, act.successCount)
		require.Equal(t, exp.errorCount, act.errorCount)
		require.Equal(t, exp.error4xxCount, act.error4xxCount)
		require.Equal(t, exp.error5xxCount, act.error5xxCount)
		require.Equal(t, exp.timeoutCount, act.timeoutCount)
		require.InDelta(t, exp.avgLatencyMs, act.avgLatencyMs, 0.0001)
		require.InDelta(t, exp.p99LatencyMs, act.p99LatencyMs, 0.0001)
		require.InDelta(t, exp.errorRate, act.errorRate, 0.01)
	}
}

type dailyKey struct {
	bucketDate string
	platform   string
}

type dailyAgg struct {
	requestCount  int64
	successCount  int64
	errorCount    int64
	error4xxCount int64
	error5xxCount int64
	timeoutCount  int64

	avgLatencyMs float64
	p99LatencyMs float64
	errorRate    float64
}

func dayKey(t time.Time) string {
	u := t.UTC()
	return u.Format("2006-01-02")
}

func fetchDailyAgg(t *testing.T, ctx context.Context, db *sql.DB, anchor time.Time) map[dailyKey]dailyAgg {
	t.Helper()

	date := dayKey(anchor)
	rows, err := db.QueryContext(ctx, `
		SELECT
			bucket_date::text,
			platform,
			request_count,
			success_count,
			error_count,
			error_4xx_count,
			error_5xx_count,
			timeout_count,
			COALESCE(avg_latency_ms, 0),
			COALESCE(p99_latency_ms, 0),
			error_rate
		FROM ops_metrics_daily
		WHERE bucket_date = $1::date
		ORDER BY platform ASC
	`, date)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	out := map[dailyKey]dailyAgg{}
	for rows.Next() {
		var (
			bucketDate string
			platform   string
			row        dailyAgg
		)
		require.NoError(t, rows.Scan(
			&bucketDate,
			&platform,
			&row.requestCount,
			&row.successCount,
			&row.errorCount,
			&row.error4xxCount,
			&row.error5xxCount,
			&row.timeoutCount,
			&row.avgLatencyMs,
			&row.p99LatencyMs,
			&row.errorRate,
		))
		out[dailyKey{bucketDate: bucketDate, platform: platform}] = row
	}
	require.NoError(t, rows.Err())
	return out
}

func assertDailyAgg(t *testing.T, expected, got map[dailyKey]dailyAgg) {
	t.Helper()
	require.Len(t, got, len(expected))
	for k, exp := range expected {
		act, ok := got[k]
		require.True(t, ok, "missing daily agg row: bucket_date=%s platform=%s", k.bucketDate, k.platform)
		require.Equal(t, exp.requestCount, act.requestCount)
		require.Equal(t, exp.successCount, act.successCount)
		require.Equal(t, exp.errorCount, act.errorCount)
		require.Equal(t, exp.error4xxCount, act.error4xxCount)
		require.Equal(t, exp.error5xxCount, act.error5xxCount)
		require.Equal(t, exp.timeoutCount, act.timeoutCount)
		require.InDelta(t, exp.avgLatencyMs, act.avgLatencyMs, 0.0001)
		require.InDelta(t, exp.p99LatencyMs, act.p99LatencyMs, 0.0001)
		require.InDelta(t, exp.errorRate, act.errorRate, 0.01)
	}
}
