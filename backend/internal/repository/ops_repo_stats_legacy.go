package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetWindowStatsLegacy keeps the original raw-log implementation (usage_logs + ops_error_logs).
// It is intentionally preserved for backward compatibility and as a safe fallback when
// pre-aggregation tables are not available.
func (r *OpsRepository) GetWindowStatsLegacy(ctx context.Context, startTime, endTime time.Time) (*service.OpsWindowStats, error) {
	query := `
		WITH
		usage_agg AS (
			SELECT
				COUNT(*) AS success_count,
				-- Ordered-set aggregates can be expensive at high QPS; computing multiple
				-- percentiles via the array form avoids redundant sorts within the same query.
				percentile_cont(ARRAY[0.50, 0.95, 0.99, 0.999]) WITHIN GROUP (ORDER BY duration_ms)
					FILTER (WHERE duration_ms IS NOT NULL) AS pcts,
				AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS avg_latency,
				MAX(duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS max_latency,
				COALESCE(
					SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens),
					0
				) AS token_consumed
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2
		),
		error_agg AS (
			SELECT
				COUNT(*) AS error_count,
				COUNT(*) FILTER (
					WHERE
						error_type = 'network_error'
						OR error_message ILIKE '%http2%'
						OR error_message ILIKE '%http/2%'
				) AS http2_errors
				,
				COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS error_4xx_count,
				COUNT(*) FILTER (WHERE status_code >= 500) AS error_5xx_count,
				COUNT(*) FILTER (
					WHERE
						error_type IN ('timeout', 'timeout_error')
						OR error_message ILIKE '%timeout%'
						OR error_message ILIKE '%deadline exceeded%'
				) AS error_timeout_count
			FROM ops_error_logs
			WHERE created_at >= $1 AND created_at < $2
		)
		SELECT
			usage_agg.success_count,
			error_agg.error_count,
			(usage_agg.pcts)[1] AS p50,
			(usage_agg.pcts)[2] AS p95,
			(usage_agg.pcts)[3] AS p99,
			(usage_agg.pcts)[4] AS p999,
			usage_agg.avg_latency,
			usage_agg.max_latency,
			error_agg.http2_errors,
			error_agg.error_4xx_count,
			error_agg.error_5xx_count,
			error_agg.error_timeout_count,
			usage_agg.token_consumed
		FROM usage_agg
		CROSS JOIN error_agg
	`

	var stats service.OpsWindowStats
	var p50Latency, p95Latency, p99Latency, p999Latency, avgLatency, maxLatency sql.NullFloat64
	var http2Errors int64
	var error4xxCount, error5xxCount, errorTimeoutCount int64
	var tokenConsumed int64
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{startTime, endTime},
		&stats.SuccessCount,
		&stats.ErrorCount,
		&p50Latency,
		&p95Latency,
		&p99Latency,
		&p999Latency,
		&avgLatency,
		&maxLatency,
		&http2Errors,
		&error4xxCount,
		&error5xxCount,
		&errorTimeoutCount,
		&tokenConsumed,
	); err != nil {
		return nil, err
	}

	stats.Error4xxCount = error4xxCount
	stats.Error5xxCount = error5xxCount
	stats.TimeoutCount = errorTimeoutCount
	stats.TokenConsumed = tokenConsumed
	if p50Latency.Valid {
		stats.P50LatencyMs = int(math.Round(p50Latency.Float64))
	}
	if p95Latency.Valid {
		stats.P95LatencyMs = int(math.Round(p95Latency.Float64))
	}
	if p99Latency.Valid {
		stats.P99LatencyMs = int(math.Round(p99Latency.Float64))
	}
	if avgLatency.Valid {
		stats.AvgLatencyMs = int(math.Round(avgLatency.Float64))
	}
	if maxLatency.Valid {
		stats.MaxLatencyMs = int(math.Round(maxLatency.Float64))
	}

	return &stats, nil
}

// GetOverviewStatsLegacy keeps the original raw-log implementation.
func (r *OpsRepository) GetOverviewStatsLegacy(ctx context.Context, startTime, endTime time.Time) (*service.OverviewStats, error) {
	query := `
		WITH
		usage_stats AS (
			SELECT
				COUNT(*) AS request_count,
				COUNT(*) FILTER (WHERE duration_ms IS NOT NULL) AS success_count,
				percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS p50,
				percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS p95,
				percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS p99,
				percentile_cont(0.999) WITHIN GROUP (ORDER BY duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS p999,
				AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS avg_latency,
				MAX(duration_ms) FILTER (WHERE duration_ms IS NOT NULL) AS max_latency
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2
		),
		error_stats AS (
			SELECT
				COUNT(*) AS error_count,
				COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS error_4xx,
				COUNT(*) FILTER (WHERE status_code >= 500) AS error_5xx,
				COUNT(*) FILTER (
					WHERE
						error_type IN ('timeout', 'timeout_error')
						OR error_message ILIKE '%timeout%'
						OR error_message ILIKE '%deadline exceeded%'
				) AS timeout_count
			FROM ops_error_logs
			WHERE created_at >= $1 AND created_at < $2
		),
		top_error AS (
			SELECT
				COALESCE(status_code::text, 'unknown') AS error_code,
				error_message,
				COUNT(*) AS error_count
			FROM ops_error_logs
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY status_code, error_message
			ORDER BY error_count DESC
			LIMIT 1
		),
		latest_metrics AS (
			SELECT
				cpu_usage_percent,
				memory_usage_percent,
				memory_used_mb,
				memory_total_mb,
				concurrency_queue_depth
			FROM ops_system_metrics
			ORDER BY created_at DESC
			LIMIT 1
		)
		SELECT
			COALESCE(usage_stats.request_count, 0) + COALESCE(error_stats.error_count, 0) AS request_count,
			COALESCE(usage_stats.success_count, 0),
			COALESCE(error_stats.error_count, 0),
			COALESCE(error_stats.error_4xx, 0),
			COALESCE(error_stats.error_5xx, 0),
			COALESCE(error_stats.timeout_count, 0),
			COALESCE(usage_stats.p50, 0),
			COALESCE(usage_stats.p95, 0),
			COALESCE(usage_stats.p99, 0),
			COALESCE(usage_stats.p999, 0),
			COALESCE(usage_stats.avg_latency, 0),
			COALESCE(usage_stats.max_latency, 0),
			COALESCE(top_error.error_code, ''),
			COALESCE(top_error.error_message, ''),
			COALESCE(top_error.error_count, 0),
			COALESCE(latest_metrics.cpu_usage_percent, 0),
			COALESCE(latest_metrics.memory_usage_percent, 0),
			COALESCE(latest_metrics.memory_used_mb, 0),
			COALESCE(latest_metrics.memory_total_mb, 0),
			COALESCE(latest_metrics.concurrency_queue_depth, 0)
		FROM usage_stats
		CROSS JOIN error_stats
		LEFT JOIN top_error ON true
		LEFT JOIN latest_metrics ON true
	`

	var stats service.OverviewStats
	var p50, p95, p99, p999, avgLatency, maxLatency sql.NullFloat64

	err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{startTime, endTime},
		&stats.RequestCount,
		&stats.SuccessCount,
		&stats.ErrorCount,
		&stats.Error4xxCount,
		&stats.Error5xxCount,
		&stats.TimeoutCount,
		&p50,
		&p95,
		&p99,
		&p999,
		&avgLatency,
		&maxLatency,
		&stats.TopErrorCode,
		&stats.TopErrorMsg,
		&stats.TopErrorCount,
		&stats.CPUUsage,
		&stats.MemoryUsage,
		&stats.MemoryUsedMB,
		&stats.MemoryTotalMB,
		&stats.ConcurrencyQueueDepth,
	)
	if err != nil {
		return nil, err
	}

	if p50.Valid {
		stats.LatencyP50 = int(p50.Float64)
	}
	if p95.Valid {
		stats.LatencyP95 = int(p95.Float64)
	}
	if p99.Valid {
		stats.LatencyP99 = int(p99.Float64)
	}
	if avgLatency.Valid {
		stats.LatencyAvg = int(avgLatency.Float64)
	}
	if maxLatency.Valid {
		stats.LatencyMax = int(maxLatency.Float64)
	}

	return &stats, nil
}

// GetProviderStatsLegacy keeps the original raw-log implementation.
func (r *OpsRepository) GetProviderStatsLegacy(ctx context.Context, startTime, endTime time.Time) ([]*service.ProviderStats, error) {
	if startTime.IsZero() || endTime.IsZero() {
		return nil, nil
	}
	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}

	query := `
		WITH combined AS (
			SELECT
				COALESCE(g.platform, a.platform, '') AS platform,
				u.duration_ms AS duration_ms,
				1 AS is_success,
				0 AS is_error,
				NULL::INT AS status_code,
				NULL::TEXT AS error_type,
				NULL::TEXT AS error_message
			FROM usage_logs u
			LEFT JOIN groups g ON g.id = u.group_id
			LEFT JOIN accounts a ON a.id = u.account_id
			WHERE u.created_at >= $1 AND u.created_at < $2

			UNION ALL

			SELECT
				COALESCE(NULLIF(o.platform, ''), g.platform, a.platform, '') AS platform,
				o.duration_ms AS duration_ms,
				0 AS is_success,
				1 AS is_error,
				o.status_code AS status_code,
				o.error_type AS error_type,
				o.error_message AS error_message
			FROM ops_error_logs o
			LEFT JOIN groups g ON g.id = o.group_id
			LEFT JOIN accounts a ON a.id = o.account_id
			WHERE o.created_at >= $1 AND o.created_at < $2
		)
		SELECT
			platform,
			COUNT(*) AS request_count,
			COALESCE(SUM(is_success), 0) AS success_count,
			COALESCE(SUM(is_error), 0) AS error_count,
			COALESCE(AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL), 0) AS avg_latency_ms,
			percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms)
				FILTER (WHERE duration_ms IS NOT NULL) AS p99_latency_ms,
			COUNT(*) FILTER (WHERE is_error = 1 AND status_code >= 400 AND status_code < 500) AS error_4xx,
			COUNT(*) FILTER (WHERE is_error = 1 AND status_code >= 500 AND status_code < 600) AS error_5xx,
			COUNT(*) FILTER (
				WHERE
					is_error = 1
					AND (
						status_code = 504
						OR error_type ILIKE '%timeout%'
						OR error_message ILIKE '%timeout%'
					)
			) AS timeout_count
		FROM combined
		WHERE platform <> ''
		GROUP BY platform
		ORDER BY request_count DESC, platform ASC
	`

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]*service.ProviderStats, 0)
	for rows.Next() {
		var item service.ProviderStats
		var avgLatency sql.NullFloat64
		var p99Latency sql.NullFloat64
		if err := rows.Scan(
			&item.Platform,
			&item.RequestCount,
			&item.SuccessCount,
			&item.ErrorCount,
			&avgLatency,
			&p99Latency,
			&item.Error4xxCount,
			&item.Error5xxCount,
			&item.TimeoutCount,
		); err != nil {
			return nil, err
		}

		if avgLatency.Valid {
			item.AvgLatencyMs = int(math.Round(avgLatency.Float64))
		}
		if p99Latency.Valid {
			item.P99LatencyMs = int(math.Round(p99Latency.Float64))
		}

		results = append(results, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetLatencyHistogramLegacy keeps the original raw-log implementation.
func (r *OpsRepository) GetLatencyHistogramLegacy(ctx context.Context, startTime, endTime time.Time) ([]*service.LatencyHistogramItem, error) {
	query := fmt.Sprintf(`
		WITH buckets AS (
			SELECT
				%[1]s AS range_name,
				%[2]s AS range_order,
				COUNT(*) AS count
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2 AND duration_ms IS NOT NULL
			GROUP BY 1, 2
		),
		total AS (
			SELECT SUM(count) AS total_count FROM buckets
		)
		SELECT
			b.range_name,
			b.count,
			ROUND((b.count::numeric / t.total_count) * 100, 2) AS percentage
		FROM buckets b
		CROSS JOIN total t
		ORDER BY b.range_order ASC
	`,
		latencyHistogramRangeCaseExpr("duration_ms"),
		latencyHistogramRangeOrderCaseExpr("duration_ms"),
	)

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]*service.LatencyHistogramItem, 0)
	for rows.Next() {
		var item service.LatencyHistogramItem
		if err := rows.Scan(&item.Range, &item.Count, &item.Percentage); err != nil {
			return nil, err
		}
		results = append(results, &item)
	}
	return results, nil
}
