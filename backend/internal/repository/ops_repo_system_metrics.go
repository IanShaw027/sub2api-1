package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *OpsRepository) GetLatestSystemMetric(ctx context.Context) (*service.OpsMetrics, error) {
	query := `
		SELECT
			window_minutes,
			request_count,
			success_count,
			error_count,
			qps,
			tps,
			error_4xx_count,
			error_5xx_count,
			error_timeout_count,
			latency_p50,
			latency_p95,
			latency_p99,
			latency_avg,
			latency_max,
			upstream_latency_avg,
			success_rate,
			error_rate,
			cpu_usage_percent,
			memory_used_mb,
			memory_total_mb,
			memory_usage_percent,
			db_conn_active,
			db_conn_idle,
			db_conn_waiting,
			goroutine_count,
			token_consumed,
			token_rate,
			active_subscriptions,
			active_alerts,
			concurrency_queue_depth,
			created_at AS updated_at
		FROM ops_system_metrics
		WHERE window_minutes = $1
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`

	var windowMinutes sql.NullInt64
	var requestCount, successCount, errorCount sql.NullInt64
	var qps, tps sql.NullFloat64
	var error4xxCount, error5xxCount, errorTimeoutCount sql.NullInt64
	var latencyP50, latencyP95, latencyP99, latencyAvg, latencyMax, upstreamLatencyAvg sql.NullFloat64
	var successRate, errorRate sql.NullFloat64
	var cpuUsage, memoryUsage sql.NullFloat64
	var memoryUsed, memoryTotal sql.NullInt64
	var dbConnActive, dbConnIdle, dbConnWaiting sql.NullInt64
	var goroutineCount sql.NullInt64
	var tokenConsumed sql.NullInt64
	var tokenRate sql.NullFloat64
	var activeSubscriptions sql.NullInt64
	var activeAlerts sql.NullInt64
	var queueDepth sql.NullInt64
	var createdAt time.Time

	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{DefaultWindowMinutes},
		&windowMinutes,
		&requestCount,
		&successCount,
		&errorCount,
		&qps,
		&tps,
		&error4xxCount,
		&error5xxCount,
		&errorTimeoutCount,
		&latencyP50,
		&latencyP95,
		&latencyP99,
		&latencyAvg,
		&latencyMax,
		&upstreamLatencyAvg,
		&successRate,
		&errorRate,
		&cpuUsage,
		&memoryUsed,
		&memoryTotal,
		&memoryUsage,
		&dbConnActive,
		&dbConnIdle,
		&dbConnWaiting,
		&goroutineCount,
		&tokenConsumed,
		&tokenRate,
		&activeSubscriptions,
		&activeAlerts,
		&queueDepth,
		&createdAt,
	); err != nil {
		return nil, err
	}

	metric := &service.OpsMetrics{
		UpdatedAt: createdAt,
	}
	if windowMinutes.Valid {
		metric.WindowMinutes = int(windowMinutes.Int64)
	}
	if requestCount.Valid {
		metric.RequestCount = requestCount.Int64
	}
	if successCount.Valid {
		metric.SuccessCount = successCount.Int64
	}
	if errorCount.Valid {
		metric.ErrorCount = errorCount.Int64
	}
	if qps.Valid {
		metric.QPS = qps.Float64
	}
	if tps.Valid {
		metric.TPS = tps.Float64
	}
	if error4xxCount.Valid {
		metric.Error4xxCount = error4xxCount.Int64
	}
	if error5xxCount.Valid {
		metric.Error5xxCount = error5xxCount.Int64
	}
	if errorTimeoutCount.Valid {
		metric.ErrorTimeoutCount = errorTimeoutCount.Int64
	}
	if latencyP50.Valid {
		metric.LatencyP50 = latencyP50.Float64
	}
	if latencyP95.Valid {
		metric.LatencyP95 = latencyP95.Float64
	}
	if latencyP99.Valid {
		metric.LatencyP99 = latencyP99.Float64
	}
	if latencyAvg.Valid {
		metric.LatencyAvg = latencyAvg.Float64
	}
	if latencyMax.Valid {
		metric.LatencyMax = latencyMax.Float64
	}
	if upstreamLatencyAvg.Valid {
		metric.UpstreamLatencyAvg = upstreamLatencyAvg.Float64
	}
	if successRate.Valid {
		metric.SuccessRate = successRate.Float64
	}
	if errorRate.Valid {
		metric.ErrorRate = errorRate.Float64
	}
	if cpuUsage.Valid {
		metric.CPUUsagePercent = cpuUsage.Float64
	}
	if memoryUsed.Valid {
		metric.MemoryUsedMB = memoryUsed.Int64
	}
	if memoryTotal.Valid {
		metric.MemoryTotalMB = memoryTotal.Int64
	}
	if memoryUsage.Valid {
		metric.MemoryUsagePercent = memoryUsage.Float64
	}
	if dbConnActive.Valid {
		metric.DBConnActive = int(dbConnActive.Int64)
	}
	if dbConnIdle.Valid {
		metric.DBConnIdle = int(dbConnIdle.Int64)
	}
	if dbConnWaiting.Valid {
		metric.DBConnWaiting = int(dbConnWaiting.Int64)
	}
	if goroutineCount.Valid {
		metric.GoroutineCount = int(goroutineCount.Int64)
	}
	if tokenConsumed.Valid {
		metric.TokenConsumed = tokenConsumed.Int64
	}
	if tokenRate.Valid {
		metric.TokenRate = tokenRate.Float64
	}
	if activeSubscriptions.Valid {
		metric.ActiveSubscriptions = int(activeSubscriptions.Int64)
	}
	if activeAlerts.Valid {
		metric.ActiveAlerts = int(activeAlerts.Int64)
	}
	if queueDepth.Valid {
		metric.ConcurrencyQueueDepth = int(queueDepth.Int64)
	}
	return metric, nil
}

func (r *OpsRepository) CreateSystemMetric(ctx context.Context, metric *service.OpsMetrics) error {
	if metric == nil {
		return nil
	}
	createdAt := metric.UpdatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	windowMinutes := metric.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = DefaultWindowMinutes
	}

	query := `
		INSERT INTO ops_system_metrics (
			window_minutes,
			request_count,
			success_count,
			error_count,
			qps,
			tps,
			error_4xx_count,
			error_5xx_count,
			error_timeout_count,
			latency_p50,
			latency_p95,
			latency_p99,
			latency_avg,
			latency_max,
			upstream_latency_avg,
			goroutine_count,
			db_conn_active,
			db_conn_idle,
			db_conn_waiting,
			token_consumed,
			token_rate,
			active_subscriptions,
			success_rate,
			error_rate,
			active_alerts,
			cpu_usage_percent,
			memory_used_mb,
			memory_total_mb,
			memory_usage_percent,
			concurrency_queue_depth,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28,
			$29, $30, $31
		)
	`
	_, err := r.sql.ExecContext(ctx, query,
		windowMinutes,
		metric.RequestCount,
		metric.SuccessCount,
		metric.ErrorCount,
		metric.QPS,
		metric.TPS,
		metric.Error4xxCount,
		metric.Error5xxCount,
		metric.ErrorTimeoutCount,
		metric.LatencyP50,
		metric.LatencyP95,
		metric.LatencyP99,
		metric.LatencyAvg,
		metric.LatencyMax,
		metric.UpstreamLatencyAvg,
		metric.GoroutineCount,
		metric.DBConnActive,
		metric.DBConnIdle,
		metric.DBConnWaiting,
		metric.TokenConsumed,
		metric.TokenRate,
		metric.ActiveSubscriptions,
		metric.SuccessRate,
		metric.ErrorRate,
		metric.ActiveAlerts,
		metric.CPUUsagePercent,
		metric.MemoryUsedMB,
		metric.MemoryTotalMB,
		metric.MemoryUsagePercent,
		metric.ConcurrencyQueueDepth,
		createdAt,
	)
	return err
}

func (r *OpsRepository) ListRecentSystemMetrics(ctx context.Context, windowMinutes, limit int) ([]service.OpsMetrics, error) {
	if windowMinutes <= 0 {
		windowMinutes = DefaultWindowMinutes
	}
	if limit <= 0 || limit > MaxRecentSystemMetricsLimit {
		limit = DefaultRecentSystemMetricsLimit
	}

	query := `
		SELECT
			window_minutes,
			request_count,
			success_count,
			error_count,
			qps,
			tps,
			error_4xx_count,
			error_5xx_count,
			error_timeout_count,
			latency_p50,
			latency_p95,
			latency_p99,
			latency_avg,
			latency_max,
			upstream_latency_avg,
			goroutine_count,
			db_conn_active,
			db_conn_idle,
			db_conn_waiting,
			token_consumed,
			token_rate,
			active_subscriptions,
			success_rate,
			error_rate,
			active_alerts,
			cpu_usage_percent,
			memory_used_mb,
			memory_total_mb,
			memory_usage_percent,
			concurrency_queue_depth,
			created_at AS updated_at
		FROM ops_system_metrics
		WHERE window_minutes = $1
		ORDER BY updated_at DESC, id DESC
		LIMIT $2
	`

	rows, err := r.sql.QueryContext(ctx, query, windowMinutes, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]service.OpsMetrics, 0)
	for rows.Next() {
		metric, err := scanOpsSystemMetric(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *metric)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *OpsRepository) ListSystemMetricsRange(ctx context.Context, windowMinutes int, startTime, endTime time.Time, limit int) ([]service.OpsMetrics, error) {
	if windowMinutes <= 0 {
		windowMinutes = DefaultWindowMinutes
	}
	if limit <= 0 || limit > MaxMetricsLimit {
		limit = DefaultMetricsLimit
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-time.Duration(limit) * time.Minute)
	}
	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}

	query := `
		SELECT
			window_minutes,
			request_count,
			success_count,
			error_count,
			qps,
			tps,
			error_4xx_count,
			error_5xx_count,
			error_timeout_count,
			latency_p50,
			latency_p95,
			latency_p99,
			latency_avg,
			latency_max,
			upstream_latency_avg,
			goroutine_count,
			db_conn_active,
			db_conn_idle,
			db_conn_waiting,
			token_consumed,
			token_rate,
			active_subscriptions,
			success_rate,
			error_rate,
			active_alerts,
			cpu_usage_percent,
			memory_used_mb,
			memory_total_mb,
			memory_usage_percent,
			concurrency_queue_depth,
			created_at
		FROM ops_system_metrics
		WHERE window_minutes = $1
		  AND created_at >= $2
		  AND created_at < $3
		ORDER BY created_at ASC
		LIMIT $4
	`

	rows, err := r.sql.QueryContext(ctx, query, windowMinutes, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]service.OpsMetrics, 0)
	for rows.Next() {
		metric, err := scanOpsSystemMetric(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *metric)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

type systemSnapshot struct {
	CPUUsage              float64
	MemoryUsage           float64
	MemoryUsedMB          int64
	MemoryTotalMB         int64
	ConcurrencyQueueDepth int
}

func (r *OpsRepository) getLatestSystemSnapshot(ctx context.Context) (*systemSnapshot, error) {
	var snap systemSnapshot
	if err := scanSingleRow(
		ctx,
		r.sql,
		`
			SELECT
				COALESCE(cpu_usage_percent, 0),
				COALESCE(memory_usage_percent, 0),
				COALESCE(memory_used_mb, 0),
				COALESCE(memory_total_mb, 0),
				COALESCE(concurrency_queue_depth, 0)
			FROM ops_system_metrics
			ORDER BY created_at DESC
			LIMIT 1
		`,
		nil,
		&snap.CPUUsage,
		&snap.MemoryUsage,
		&snap.MemoryUsedMB,
		&snap.MemoryTotalMB,
		&snap.ConcurrencyQueueDepth,
	); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (r *OpsRepository) GetTokenTPS(ctx context.Context, startTime, endTime time.Time) (current, peak, avg float64, err error) {
	var currentTPS sql.NullFloat64
	{
		row := r.sql.QueryRowContext(ctx, `
			SELECT token_rate
			FROM ops_system_metrics
			WHERE window_minutes = 1
			ORDER BY created_at DESC
			LIMIT 1
		`)
		scanErr := row.Scan(&currentTPS)
		if scanErr != nil && scanErr != sql.ErrNoRows {
			return 0, 0, 0, scanErr
		}
	}

	var peakTPS, avgTPS sql.NullFloat64
	{
		row := r.sql.QueryRowContext(ctx, `
			SELECT
				MAX(token_rate) as peak_tps,
				AVG(token_rate) as avg_tps
			FROM ops_system_metrics
			WHERE window_minutes = 1
			  AND created_at >= $1
			  AND created_at < $2
		`, startTime, endTime)
		scanErr := row.Scan(&peakTPS, &avgTPS)
		if scanErr != nil && scanErr != sql.ErrNoRows {
			return 0, 0, 0, scanErr
		}
	}

	current = 0
	if currentTPS.Valid {
		current = currentTPS.Float64
	}

	peak = 0
	if peakTPS.Valid {
		peak = peakTPS.Float64
	}

	avg = 0
	if avgTPS.Valid {
		avg = avgTPS.Float64
	}

	return current, peak, avg, nil
}

func scanOpsSystemMetric(rows *sql.Rows) (*service.OpsMetrics, error) {
	var metric service.OpsMetrics
	var windowMinutes sql.NullInt64
	var requestCount, successCount, errorCount sql.NullInt64
	var qps, tps sql.NullFloat64
	var error4xxCount, error5xxCount, errorTimeoutCount sql.NullInt64
	var latencyP50, latencyP95, latencyP99, latencyAvg, latencyMax, upstreamLatencyAvg sql.NullFloat64
	var goroutineCount, dbConnActive, dbConnIdle, dbConnWaiting sql.NullInt64
	var tokenConsumed sql.NullInt64
	var tokenRate sql.NullFloat64
	var activeSubscriptions sql.NullInt64
	var successRate, errorRate sql.NullFloat64
	var activeAlerts sql.NullInt64
	var cpuUsage sql.NullFloat64
	var memoryUsed, memoryTotal sql.NullInt64
	var memoryUsage sql.NullFloat64
	var queueDepth sql.NullInt64

	if err := rows.Scan(
		&windowMinutes,
		&requestCount,
		&successCount,
		&errorCount,
		&qps,
		&tps,
		&error4xxCount,
		&error5xxCount,
		&errorTimeoutCount,
		&latencyP50,
		&latencyP95,
		&latencyP99,
		&latencyAvg,
		&latencyMax,
		&upstreamLatencyAvg,
		&goroutineCount,
		&dbConnActive,
		&dbConnIdle,
		&dbConnWaiting,
		&tokenConsumed,
		&tokenRate,
		&activeSubscriptions,
		&successRate,
		&errorRate,
		&activeAlerts,
		&cpuUsage,
		&memoryUsed,
		&memoryTotal,
		&memoryUsage,
		&queueDepth,
		&metric.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if windowMinutes.Valid {
		metric.WindowMinutes = int(windowMinutes.Int64)
	}
	if requestCount.Valid {
		metric.RequestCount = requestCount.Int64
	}
	if successCount.Valid {
		metric.SuccessCount = successCount.Int64
	}
	if errorCount.Valid {
		metric.ErrorCount = errorCount.Int64
	}
	if qps.Valid {
		metric.QPS = qps.Float64
	}
	if tps.Valid {
		metric.TPS = tps.Float64
	}
	if error4xxCount.Valid {
		metric.Error4xxCount = error4xxCount.Int64
	}
	if error5xxCount.Valid {
		metric.Error5xxCount = error5xxCount.Int64
	}
	if errorTimeoutCount.Valid {
		metric.ErrorTimeoutCount = errorTimeoutCount.Int64
	}
	if latencyP50.Valid {
		metric.LatencyP50 = latencyP50.Float64
	}
	if latencyP95.Valid {
		metric.LatencyP95 = latencyP95.Float64
	}
	if latencyP99.Valid {
		metric.LatencyP99 = latencyP99.Float64
	}
	if latencyAvg.Valid {
		metric.LatencyAvg = latencyAvg.Float64
	}
	if latencyMax.Valid {
		metric.LatencyMax = latencyMax.Float64
	}
	if upstreamLatencyAvg.Valid {
		metric.UpstreamLatencyAvg = upstreamLatencyAvg.Float64
	}
	if goroutineCount.Valid {
		metric.GoroutineCount = int(goroutineCount.Int64)
	}
	if dbConnActive.Valid {
		metric.DBConnActive = int(dbConnActive.Int64)
	}
	if dbConnIdle.Valid {
		metric.DBConnIdle = int(dbConnIdle.Int64)
	}
	if dbConnWaiting.Valid {
		metric.DBConnWaiting = int(dbConnWaiting.Int64)
	}
	if tokenConsumed.Valid {
		metric.TokenConsumed = tokenConsumed.Int64
	}
	if tokenRate.Valid {
		metric.TokenRate = tokenRate.Float64
	}
	if activeSubscriptions.Valid {
		metric.ActiveSubscriptions = int(activeSubscriptions.Int64)
	}
	if successRate.Valid {
		metric.SuccessRate = successRate.Float64
	}
	if errorRate.Valid {
		metric.ErrorRate = errorRate.Float64
	}
	if activeAlerts.Valid {
		metric.ActiveAlerts = int(activeAlerts.Int64)
	}
	if cpuUsage.Valid {
		metric.CPUUsagePercent = cpuUsage.Float64
	}
	if memoryUsed.Valid {
		metric.MemoryUsedMB = memoryUsed.Int64
	}
	if memoryTotal.Valid {
		metric.MemoryTotalMB = memoryTotal.Int64
	}
	if memoryUsage.Valid {
		metric.MemoryUsagePercent = memoryUsage.Float64
	}
	if queueDepth.Valid {
		metric.ConcurrencyQueueDepth = int(queueDepth.Int64)
	}

	return &metric, nil
}
