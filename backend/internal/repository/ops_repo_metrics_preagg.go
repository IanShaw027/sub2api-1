package repository

import (
	"context"
	"database/sql"
	"time"
)

func (r *OpsRepository) UpsertHourlyMetrics(ctx context.Context, startTime, endTime time.Time) error {
	if endTime.IsZero() || startTime.IsZero() || !endTime.After(startTime) {
		return nil
	}

	query := `
		WITH
		usage AS (
			SELECT
				date_trunc('hour', u.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket_start,
				COALESCE(NULLIF(g.platform, ''), a.platform, '') AS platform,
				COUNT(*) AS success_count,
				AVG(u.duration_ms) FILTER (WHERE u.duration_ms IS NOT NULL) AS avg_latency_ms,
				percentile_cont(0.99) WITHIN GROUP (ORDER BY u.duration_ms)
					FILTER (WHERE u.duration_ms IS NOT NULL) AS p99_latency_ms
			FROM usage_logs u
			LEFT JOIN groups g ON u.group_id = g.id
			LEFT JOIN accounts a ON u.account_id = a.id
			WHERE u.created_at >= $1 AND u.created_at < $2
			GROUP BY 1, 2
		),
		errors AS (
			SELECT
				date_trunc('hour', o.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket_start,
				COALESCE(NULLIF(o.platform, ''), NULLIF(g.platform, ''), a.platform, '') AS platform,
				COUNT(*) AS error_count,
				COUNT(*) FILTER (WHERE o.status_code >= 400 AND o.status_code < 500) AS error_4xx_count,
				COUNT(*) FILTER (WHERE o.status_code >= 500) AS error_5xx_count,
				COUNT(*) FILTER (
					WHERE
						o.error_type IN ('timeout', 'timeout_error')
						OR o.error_message ILIKE '%timeout%'
						OR o.error_message ILIKE '%deadline exceeded%'
				) AS timeout_count
			FROM ops_error_logs o
			LEFT JOIN groups g ON o.group_id = g.id
			LEFT JOIN accounts a ON o.account_id = a.id
			WHERE o.created_at >= $1 AND o.created_at < $2
			GROUP BY 1, 2
		),
		combined AS (
			SELECT
				COALESCE(u.bucket_start, e.bucket_start) AS bucket_start,
				COALESCE(u.platform, e.platform) AS platform,
				COALESCE(u.success_count, 0) AS success_count,
				COALESCE(e.error_count, 0) AS error_count,
				COALESCE(e.error_4xx_count, 0) AS error_4xx_count,
				COALESCE(e.error_5xx_count, 0) AS error_5xx_count,
				COALESCE(e.timeout_count, 0) AS timeout_count,
				u.avg_latency_ms,
				u.p99_latency_ms
			FROM usage u
			FULL OUTER JOIN errors e
				ON u.bucket_start = e.bucket_start AND u.platform = e.platform
		)
		INSERT INTO ops_metrics_hourly (
			bucket_start,
			platform,
			request_count,
			success_count,
			error_count,
			error_4xx_count,
			error_5xx_count,
			timeout_count,
			avg_latency_ms,
			p99_latency_ms,
			error_rate,
			computed_at
		)
		SELECT
			bucket_start,
			platform,
			(success_count + error_count) AS request_count,
			success_count,
			error_count,
			error_4xx_count,
			error_5xx_count,
			timeout_count,
			avg_latency_ms,
			p99_latency_ms,
			CASE
				WHEN (success_count + error_count) = 0 THEN 0
				ELSE (error_count::double precision * 100.0) / (success_count + error_count)
			END AS error_rate,
			NOW()
		FROM combined
		WHERE platform <> ''
		ON CONFLICT (bucket_start, platform) DO UPDATE SET
			request_count = EXCLUDED.request_count,
			success_count = EXCLUDED.success_count,
			error_count = EXCLUDED.error_count,
			error_4xx_count = EXCLUDED.error_4xx_count,
			error_5xx_count = EXCLUDED.error_5xx_count,
			timeout_count = EXCLUDED.timeout_count,
			avg_latency_ms = CASE
				WHEN ops_metrics_hourly.success_count + EXCLUDED.success_count = 0 THEN NULL
				ELSE (
					COALESCE(ops_metrics_hourly.avg_latency_ms, 0) * ops_metrics_hourly.success_count +
					COALESCE(EXCLUDED.avg_latency_ms, 0) * EXCLUDED.success_count
				) / (ops_metrics_hourly.success_count + EXCLUDED.success_count)
			END,
			p99_latency_ms = CASE
				WHEN ops_metrics_hourly.success_count + EXCLUDED.success_count = 0 THEN NULL
				ELSE GREATEST(
					COALESCE(ops_metrics_hourly.p99_latency_ms, 0),
					COALESCE(EXCLUDED.p99_latency_ms, 0)
				)
			END,
			error_rate = EXCLUDED.error_rate,
			computed_at = NOW()
	`

	_, err := r.sql.ExecContext(ctx, query, startTime, endTime)
	return err
}

func (r *OpsRepository) UpsertDailyMetrics(ctx context.Context, startTime, endTime time.Time) error {
	if endTime.IsZero() || startTime.IsZero() || !endTime.After(startTime) {
		return nil
	}

	query := `
		INSERT INTO ops_metrics_daily (
			bucket_date,
			platform,
			request_count,
			success_count,
			error_count,
			error_4xx_count,
			error_5xx_count,
			timeout_count,
			avg_latency_ms,
			p99_latency_ms,
			error_rate,
			computed_at
		)
		SELECT
			(h.bucket_start AT TIME ZONE 'UTC')::date AS bucket_date,
			h.platform,
			SUM(h.request_count) AS request_count,
			SUM(h.success_count) AS success_count,
			SUM(h.error_count) AS error_count,
			SUM(h.error_4xx_count) AS error_4xx_count,
			SUM(h.error_5xx_count) AS error_5xx_count,
			SUM(h.timeout_count) AS timeout_count,
			(
				SUM(h.avg_latency_ms * h.success_count) FILTER (WHERE h.avg_latency_ms IS NOT NULL)
				/ NULLIF(SUM(h.success_count) FILTER (WHERE h.avg_latency_ms IS NOT NULL), 0)
			) AS avg_latency_ms,
			MAX(h.p99_latency_ms) AS p99_latency_ms,
			CASE
				WHEN SUM(h.request_count) = 0 THEN 0
				ELSE (SUM(h.error_count)::double precision * 100.0) / SUM(h.request_count)
			END AS error_rate,
			NOW()
		FROM ops_metrics_hourly h
		WHERE h.bucket_start >= $1 AND h.bucket_start < $2 AND h.platform <> ''
		GROUP BY 1, 2
		ON CONFLICT (bucket_date, platform) DO UPDATE SET
			request_count = EXCLUDED.request_count,
			success_count = EXCLUDED.success_count,
			error_count = EXCLUDED.error_count,
			error_4xx_count = EXCLUDED.error_4xx_count,
			error_5xx_count = EXCLUDED.error_5xx_count,
			timeout_count = EXCLUDED.timeout_count,
			avg_latency_ms = CASE
				WHEN ops_metrics_daily.success_count + EXCLUDED.success_count = 0 THEN NULL
				ELSE (
					COALESCE(ops_metrics_daily.avg_latency_ms, 0) * ops_metrics_daily.success_count +
					COALESCE(EXCLUDED.avg_latency_ms, 0) * EXCLUDED.success_count
				) / (ops_metrics_daily.success_count + EXCLUDED.success_count)
			END,
			p99_latency_ms = GREATEST(
				COALESCE(ops_metrics_daily.p99_latency_ms, 0),
				COALESCE(EXCLUDED.p99_latency_ms, 0)
			),
			error_rate = EXCLUDED.error_rate,
			computed_at = NOW()
	`

	_, err := r.sql.ExecContext(ctx, query, startTime, endTime)
	return err
}

func (r *OpsRepository) GetLatestHourlyBucketStart(ctx context.Context) (time.Time, bool, error) {
	var value sql.NullTime
	if err := r.sql.QueryRowContext(ctx, `SELECT MAX(bucket_start) FROM ops_metrics_hourly`).Scan(&value); err != nil {
		return time.Time{}, false, err
	}
	if !value.Valid {
		return time.Time{}, false, nil
	}
	return value.Time.UTC(), true, nil
}

func (r *OpsRepository) GetLatestDailyBucketDate(ctx context.Context) (time.Time, bool, error) {
	var value sql.NullTime
	if err := r.sql.QueryRowContext(ctx, `SELECT MAX(bucket_date) FROM ops_metrics_daily`).Scan(&value); err != nil {
		return time.Time{}, false, err
	}
	if !value.Valid {
		return time.Time{}, false, nil
	}
	t := value.Time
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true, nil
}
