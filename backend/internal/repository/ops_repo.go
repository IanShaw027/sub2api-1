package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultWindowMinutes = 1

	MaxErrorLogsLimit     = 500
	DefaultErrorLogsLimit = 200

	MaxRecentSystemMetricsLimit     = 500
	DefaultRecentSystemMetricsLimit = 60

	MaxMetricsLimit     = 5000
	DefaultMetricsLimit = 300
)

// opsMetricsPreaggFreshnessLag is the maximum "fresh" window we assume may not be
// covered by the hourly/daily aggregation tables.
//
// The pre-aggregation tables are intended to be populated by a background job; the
// newest hour is typically still being computed. For that most-recent slice we fall
// back to the legacy raw-log queries to keep real-time dashboards accurate.
const opsMetricsPreaggFreshnessLag = time.Hour

type OpsRepository struct {
	sql sqlExecutor
	rdb *redis.Client

	// Feature flag: prefer pre-aggregated ops tables (ops_metrics_hourly/daily) for
	// expensive dashboard queries when available, with safe fallbacks to legacy raw-log queries.
	usePreaggregatedTables bool
}

func NewOpsRepository(_ *dbent.Client, sqlDB *sql.DB, rdb *redis.Client, cfg *config.Config) service.OpsRepository {
	usePreagg := false
	if cfg != nil {
		usePreagg = cfg.Ops.UsePreaggregatedTables
	}
	return &OpsRepository{sql: sqlDB, rdb: rdb, usePreaggregatedTables: usePreagg}
}

func (r *OpsRepository) CreateErrorLog(ctx context.Context, log *service.OpsErrorLog) error {
	if log == nil {
		return nil
	}

	createdAt := log.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	query := `
		INSERT INTO ops_error_logs (
			request_id,
			user_id,
			api_key_id,
			account_id,
			group_id,
			client_ip,
			error_phase,
			error_type,
			severity,
			status_code,
			platform,
			model,
			request_path,
			stream,
			error_message,
			error_body,
			provider_error_code,
			provider_error_type,
			is_retryable,
			is_user_actionable,
			retry_count,
			completion_status,
			duration_ms,
			time_to_first_token_ms,
			auth_latency_ms,
			routing_latency_ms,
			upstream_latency_ms,
			response_latency_ms,
			request_body,
			user_agent,
			error_source,
			error_owner,
			account_status,
			upstream_status_code,
			upstream_error_message,
			network_error_type,
			retry_after_seconds,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25,
			$26, $27, $28, $29, $30,
			$31, $32, $33, $34, $35,
			$36, $37, $38
		)
		RETURNING id, created_at
	`

	requestID := nullString(log.RequestID)
	clientIP := nullString(log.ClientIP)
	platform := nullString(log.Platform)
	model := nullString(log.Model)
	requestPath := nullString(log.RequestPath)
	message := nullString(log.Message)
	errorBody := nullString(log.ErrorBody)
	providerErrorCode := nullString(log.ProviderErrorCode)
	providerErrorType := nullString(log.ProviderErrorType)
	completionStatus := nullString(log.CompletionStatus)
	userAgent := nullString(log.UserAgent)

	// For backward compatibility: use DurationMs if available, otherwise fall back to LatencyMs
	var durationMs sql.NullInt64
	if log.DurationMs != nil {
		durationMs = sql.NullInt64{Int64: int64(*log.DurationMs), Valid: true}
	} else if log.LatencyMs != nil {
		durationMs = sql.NullInt64{Int64: int64(*log.LatencyMs), Valid: true}
	}

	timeToFirstTokenMs := nullInt64Ptr(log.TimeToFirstTokenMs)
	authLatencyMs := nullInt64Ptr(log.AuthLatencyMs)
	routingLatencyMs := nullInt64Ptr(log.RoutingLatencyMs)
	upstreamLatencyMs := nullInt64Ptr(log.UpstreamLatencyMs)
	responseLatencyMs := nullInt64Ptr(log.ResponseLatencyMs)

	// Handle request_body as JSONB (can be nil)
	var requestBody sql.NullString
	if log.RequestBody != "" {
		requestBody = sql.NullString{String: log.RequestBody, Valid: true}
	}

	args := []any{
		requestID,
		nullInt64(log.UserID),
		nullInt64(log.APIKeyID),
		nullInt64(log.AccountID),
		nullInt64(log.GroupID),
		clientIP,
		log.Phase,
		log.Type,
		log.Severity,
		log.StatusCode,
		platform,
		model,
		requestPath,
		log.Stream,
		message,
		errorBody,
		providerErrorCode,
		providerErrorType,
		log.IsRetryable,
		log.IsUserActionable,
		log.RetryCount,
		completionStatus,
		durationMs,
		timeToFirstTokenMs,
		authLatencyMs,
		routingLatencyMs,
		upstreamLatencyMs,
		responseLatencyMs,
		requestBody,
		userAgent,
		nullString(log.ErrorSource),
		nullString(log.ErrorOwner),
		nullString(log.AccountStatus),
		nullInt64Ptr(log.UpstreamStatusCode),
		nullString(log.UpstreamErrorMessage),
		nullString(log.NetworkErrorType),
		nullInt64Ptr(log.RetryAfterSeconds),
		createdAt,
	}

	if err := scanSingleRow(ctx, r.sql, query, args, &log.ID, &log.CreatedAt); err != nil {
		return err
	}
	return nil
}

func (r *OpsRepository) ListErrorLogsLegacy(ctx context.Context, filters service.OpsErrorLogFilters) ([]service.OpsErrorLog, error) {
	conditions := make([]string, 0)
	args := make([]any, 0)

	addCondition := func(condition string, values ...any) {
		conditions = append(conditions, condition)
		args = append(args, values...)
	}

	if filters.StartTime != nil {
		addCondition(fmt.Sprintf("created_at >= $%d", len(args)+1), *filters.StartTime)
	}
	if filters.EndTime != nil {
		addCondition(fmt.Sprintf("created_at <= $%d", len(args)+1), *filters.EndTime)
	}
	if filters.Platform != "" {
		addCondition(fmt.Sprintf("platform = $%d", len(args)+1), filters.Platform)
	}
	if filters.Phase != "" {
		addCondition(fmt.Sprintf("error_phase = $%d", len(args)+1), filters.Phase)
	}
	if filters.Severity != "" {
		addCondition(fmt.Sprintf("severity = $%d", len(args)+1), filters.Severity)
	}
	if filters.Query != "" {
		like := "%" + strings.ToLower(filters.Query) + "%"
		startIdx := len(args) + 1
		addCondition(
			fmt.Sprintf("(LOWER(request_id) LIKE $%d OR LOWER(model) LIKE $%d OR LOWER(error_message) LIKE $%d OR LOWER(error_type) LIKE $%d)",
				startIdx, startIdx+1, startIdx+2, startIdx+3,
			),
			like, like, like, like,
		)
	}

	limit := filters.Limit
	if limit <= 0 || limit > MaxErrorLogsLimit {
		limit = DefaultErrorLogsLimit
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			id,
			created_at,
			user_id,
			api_key_id,
			account_id,
			group_id,
			client_ip,
			error_phase,
			error_type,
			severity,
			status_code,
			platform,
			model,
			request_path,
			stream,
			duration_ms,
			request_id,
			error_message,
			error_body,
			provider_error_code,
			provider_error_type,
			is_retryable,
			is_user_actionable,
			retry_count,
			completion_status
		FROM ops_error_logs
		%s
		ORDER BY created_at DESC
		LIMIT $%d
	`, where, len(args)+1)

	args = append(args, limit)

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]service.OpsErrorLog, 0)
	for rows.Next() {
		logEntry, err := scanOpsErrorLog(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *logEntry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

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
			latency_p999,
			latency_avg,
			latency_max,
			upstream_latency_avg,
			disk_used,
			disk_total,
			disk_iops,
			network_in_bytes,
			network_out_bytes,
			goroutine_count,
			db_conn_active,
			db_conn_idle,
			db_conn_waiting,
			token_consumed,
			token_rate,
			active_subscriptions,
			tags,
			success_rate,
			error_rate,
			p95_latency_ms,
			p99_latency_ms,
			http2_errors,
			active_alerts,
			cpu_usage_percent,
			memory_used_mb,
			memory_total_mb,
			memory_usage_percent,
			heap_alloc_mb,
			gc_pause_ms,
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
	var latencyP50, latencyP999, latencyAvg, latencyMax, upstreamLatencyAvg sql.NullFloat64
	var diskUsed, diskTotal, diskIOPS sql.NullInt64
	var networkInBytes, networkOutBytes sql.NullInt64
	var goroutineCount, dbConnActive, dbConnIdle, dbConnWaiting sql.NullInt64
	var tokenConsumed sql.NullInt64
	var tokenRate sql.NullFloat64
	var activeSubscriptions sql.NullInt64
	var tags []byte
	var successRate, errorRate sql.NullFloat64
	var p95Latency, p99Latency, http2Errors, activeAlerts sql.NullInt64
	var cpuUsage, memoryUsage, gcPause sql.NullFloat64
	var memoryUsed, memoryTotal, heapAlloc, queueDepth sql.NullInt64
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
		&latencyP999,
		&latencyAvg,
		&latencyMax,
		&upstreamLatencyAvg,
		&diskUsed,
		&diskTotal,
		&diskIOPS,
		&networkInBytes,
		&networkOutBytes,
		&goroutineCount,
		&dbConnActive,
		&dbConnIdle,
		&dbConnWaiting,
		&tokenConsumed,
		&tokenRate,
		&activeSubscriptions,
		&tags,
		&successRate,
		&errorRate,
		&p95Latency,
		&p99Latency,
		&http2Errors,
		&activeAlerts,
		&cpuUsage,
		&memoryUsed,
		&memoryTotal,
		&memoryUsage,
		&heapAlloc,
		&gcPause,
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
	if latencyP999.Valid {
		metric.LatencyP999 = latencyP999.Float64
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
	if diskUsed.Valid {
		metric.DiskUsed = diskUsed.Int64
	}
	if diskTotal.Valid {
		metric.DiskTotal = diskTotal.Int64
	}
	if diskIOPS.Valid {
		metric.DiskIOPS = diskIOPS.Int64
	}
	if networkInBytes.Valid {
		metric.NetworkInBytes = networkInBytes.Int64
	}
	if networkOutBytes.Valid {
		metric.NetworkOutBytes = networkOutBytes.Int64
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
	if len(tags) > 0 {
		_ = json.Unmarshal(tags, &metric.Tags)
	}
	if successRate.Valid {
		metric.SuccessRate = successRate.Float64
	}
	if errorRate.Valid {
		metric.ErrorRate = errorRate.Float64
	}
	if p95Latency.Valid {
		metric.P95LatencyMs = int(p95Latency.Int64)
	}
	if p99Latency.Valid {
		metric.P99LatencyMs = int(p99Latency.Int64)
	}
	if http2Errors.Valid {
		metric.HTTP2Errors = int(http2Errors.Int64)
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
	if heapAlloc.Valid {
		metric.HeapAllocMB = heapAlloc.Int64
	}
	if gcPause.Valid {
		metric.GCPauseMs = gcPause.Float64
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
			latency_p999,
			latency_avg,
			latency_max,
			upstream_latency_avg,
			disk_used,
			disk_total,
			disk_iops,
			network_in_bytes,
			network_out_bytes,
			goroutine_count,
			db_conn_active,
			db_conn_idle,
			db_conn_waiting,
			token_consumed,
			token_rate,
			active_subscriptions,
			tags,
			success_rate,
			error_rate,
			p95_latency_ms,
			p99_latency_ms,
			http2_errors,
			active_alerts,
			cpu_usage_percent,
			memory_used_mb,
			memory_total_mb,
			memory_usage_percent,
			heap_alloc_mb,
			gc_pause_ms,
			concurrency_queue_depth,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28::jsonb,
			$29, $30, $31, $32, $33, $34, $35, $36,
			$37, $38, $39, $40, $41, $42, $43, $44
		)
	`
	tagsJSON := "{}"
	if metric.Tags != nil {
		if raw, err := json.Marshal(metric.Tags); err == nil && len(raw) > 0 {
			tagsJSON = string(raw)
		}
	}
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
		metric.LatencyP999,
		metric.LatencyAvg,
		metric.LatencyMax,
		metric.UpstreamLatencyAvg,
		metric.DiskUsed,
		metric.DiskTotal,
		metric.DiskIOPS,
		metric.NetworkInBytes,
		metric.NetworkOutBytes,
		metric.GoroutineCount,
		metric.DBConnActive,
		metric.DBConnIdle,
		metric.DBConnWaiting,
		metric.TokenConsumed,
		metric.TokenRate,
		metric.ActiveSubscriptions,
		tagsJSON,
		metric.SuccessRate,
		metric.ErrorRate,
		metric.P95LatencyMs,
		metric.P99LatencyMs,
		metric.HTTP2Errors,
		metric.ActiveAlerts,
		metric.CPUUsagePercent,
		metric.MemoryUsedMB,
		metric.MemoryTotalMB,
		metric.MemoryUsagePercent,
		metric.HeapAllocMB,
		metric.GCPauseMs,
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
			latency_p999,
			latency_avg,
			latency_max,
			upstream_latency_avg,
			disk_used,
			disk_total,
			disk_iops,
			network_in_bytes,
			network_out_bytes,
			goroutine_count,
			db_conn_active,
			db_conn_idle,
			db_conn_waiting,
			token_consumed,
			token_rate,
			active_subscriptions,
			tags,
			success_rate,
			error_rate,
			p95_latency_ms,
			p99_latency_ms,
			http2_errors,
			active_alerts,
			cpu_usage_percent,
			memory_used_mb,
			memory_total_mb,
			memory_usage_percent,
			heap_alloc_mb,
			gc_pause_ms,
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
			latency_p999,
			latency_avg,
			latency_max,
			upstream_latency_avg,
			disk_used,
			disk_total,
			disk_iops,
			network_in_bytes,
			network_out_bytes,
			goroutine_count,
			db_conn_active,
			db_conn_idle,
			db_conn_waiting,
			token_consumed,
			token_rate,
			active_subscriptions,
			tags,
			success_rate,
			error_rate,
			p95_latency_ms,
			p99_latency_ms,
			http2_errors,
			active_alerts,
			cpu_usage_percent,
			memory_used_mb,
			memory_total_mb,
			memory_usage_percent,
			heap_alloc_mb,
			gc_pause_ms,
			concurrency_queue_depth,
			created_at
		FROM ops_system_metrics
		WHERE window_minutes = $1
		  AND created_at >= $2
		  AND created_at <= $3
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

func (r *OpsRepository) ListAlertRules(ctx context.Context) ([]service.OpsAlertRule, error) {
	query := `
		SELECT
			id,
			name,
			description,
			enabled,
			metric_type,
			operator,
			threshold,
			window_minutes,
			sustained_minutes,
			severity,
			notify_email,
			notify_webhook,
			webhook_url,
			cooldown_minutes,
			dimension_filters,
			notify_channels,
			notify_config,
			created_at,
			updated_at
		FROM ops_alert_rules
		ORDER BY id ASC
	`

	rows, err := r.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	rules := make([]service.OpsAlertRule, 0)
	for rows.Next() {
		var rule service.OpsAlertRule
		var description sql.NullString
		var webhookURL sql.NullString
		var dimensionFilters, notifyChannels, notifyConfig []byte
		if err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&description,
			&rule.Enabled,
			&rule.MetricType,
			&rule.Operator,
			&rule.Threshold,
			&rule.WindowMinutes,
			&rule.SustainedMinutes,
			&rule.Severity,
			&rule.NotifyEmail,
			&rule.NotifyWebhook,
			&webhookURL,
			&rule.CooldownMinutes,
			&dimensionFilters,
			&notifyChannels,
			&notifyConfig,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if description.Valid {
			rule.Description = description.String
		}
		if webhookURL.Valid {
			rule.WebhookURL = webhookURL.String
		}
		if len(dimensionFilters) > 0 {
			_ = json.Unmarshal(dimensionFilters, &rule.DimensionFilters)
		}
		if len(notifyChannels) > 0 {
			_ = json.Unmarshal(notifyChannels, &rule.NotifyChannels)
		}
		if len(notifyConfig) > 0 {
			_ = json.Unmarshal(notifyConfig, &rule.NotifyConfig)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *OpsRepository) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*service.OpsAlertEvent, error) {
	return r.getAlertEvent(ctx, `WHERE rule_id = $1 AND status = $2`, []any{ruleID, service.OpsAlertStatusFiring})
}

func (r *OpsRepository) GetLatestAlertEvent(ctx context.Context, ruleID int64) (*service.OpsAlertEvent, error) {
	return r.getAlertEvent(ctx, `WHERE rule_id = $1`, []any{ruleID})
}

func (r *OpsRepository) CreateAlertEvent(ctx context.Context, event *service.OpsAlertEvent) error {
	if event == nil {
		return nil
	}
	if event.FiredAt.IsZero() {
		event.FiredAt = time.Now()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = event.FiredAt
	}
	if event.Status == "" {
		event.Status = service.OpsAlertStatusFiring
	}

	query := `
		INSERT INTO ops_alert_events (
			rule_id,
			severity,
			status,
			title,
			description,
			metric_value,
			threshold_value,
			fired_at,
			resolved_at,
			email_sent,
			webhook_sent,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12
		)
		RETURNING id, created_at
	`

	var resolvedAt sql.NullTime
	if event.ResolvedAt != nil {
		resolvedAt = sql.NullTime{Time: *event.ResolvedAt, Valid: true}
	}

	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{
			event.RuleID,
			event.Severity,
			event.Status,
			event.Title,
			event.Description,
			event.MetricValue,
			event.ThresholdValue,
			event.FiredAt,
			resolvedAt,
			event.EmailSent,
			event.WebhookSent,
			event.CreatedAt,
		},
		&event.ID,
		&event.CreatedAt,
	); err != nil {
		return err
	}
	return nil
}

func (r *OpsRepository) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	var resolved sql.NullTime
	if resolvedAt != nil {
		resolved = sql.NullTime{Time: *resolvedAt, Valid: true}
	}
	_, err := r.sql.ExecContext(ctx, `
		UPDATE ops_alert_events
		SET status = $2, resolved_at = $3
		WHERE id = $1
	`, eventID, status, resolved)
	return err
}

func (r *OpsRepository) UpdateAlertEventNotifications(ctx context.Context, eventID int64, emailSent, webhookSent bool) error {
	_, err := r.sql.ExecContext(ctx, `
		UPDATE ops_alert_events
		SET email_sent = $2, webhook_sent = $3
		WHERE id = $1
	`, eventID, emailSent, webhookSent)
	return err
}

func (r *OpsRepository) CountActiveAlerts(ctx context.Context) (int, error) {
	var count int64
	if err := scanSingleRow(
		ctx,
		r.sql,
		`SELECT COUNT(*) FROM ops_alert_events WHERE status = $1`,
		[]any{service.OpsAlertStatusFiring},
		&count,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return int(count), nil
}

// GetWindowStats is the primary API used by the ops dashboard/collector.
//
// When enabled via config (`ops.use_preaggregated_tables`), it prefers the
// pre-aggregated tables for data old enough to be stable (hourly/daily buckets),
// and falls back to the legacy raw-log query for the most recent <1h slice and
// for any periods where aggregates are not yet populated.
func (r *OpsRepository) GetWindowStats(ctx context.Context, startTime, endTime time.Time) (*service.OpsWindowStats, error) {
	if !r.usePreaggregatedTables {
		return r.GetWindowStatsLegacy(ctx, startTime, endTime)
	}
	stats, err := r.getWindowStatsPreaggregated(ctx, startTime, endTime)
	if err == nil {
		return stats, nil
	}
	return r.GetWindowStatsLegacy(ctx, startTime, endTime)
}

// GetWindowStatsLegacy keeps the original raw-log implementation (usage_logs + ops_error_logs).
// It is intentionally preserved for backward compatibility and as a safe fallback when
// pre-aggregation tables are not available.
func (r *OpsRepository) GetWindowStatsLegacy(ctx context.Context, startTime, endTime time.Time) (*service.OpsWindowStats, error) {
	query := `
		WITH
		usage_agg AS (
			SELECT
				COUNT(*) AS success_count,
				percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms)
					FILTER (WHERE duration_ms IS NOT NULL) AS p50,
				percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms)
					FILTER (WHERE duration_ms IS NOT NULL) AS p95,
				percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms)
					FILTER (WHERE duration_ms IS NOT NULL) AS p99
				,
				percentile_cont(0.999) WITHIN GROUP (ORDER BY duration_ms)
					FILTER (WHERE duration_ms IS NOT NULL) AS p999,
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
			usage_agg.p50,
			usage_agg.p95,
			usage_agg.p99,
			usage_agg.p999,
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

	stats.HTTP2Errors = int(http2Errors)
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
	if p999Latency.Valid {
		stats.P999LatencyMs = int(math.Round(p999Latency.Float64))
	}
	if avgLatency.Valid {
		stats.AvgLatencyMs = int(math.Round(avgLatency.Float64))
	}
	if maxLatency.Valid {
		stats.MaxLatencyMs = int(math.Round(maxLatency.Float64))
	}

	return &stats, nil
}

// GetOverviewStats powers the ops "dashboard overview" endpoint.
//
// The legacy implementation runs percentile queries on raw logs. When the feature
// flag is enabled, this method prefers the pre-aggregation tables for older data
// (full buckets only) and uses the raw-log query only for the newest <1h slice.
func (r *OpsRepository) GetOverviewStats(ctx context.Context, startTime, endTime time.Time) (*service.OverviewStats, error) {
	if !r.usePreaggregatedTables {
		return r.GetOverviewStatsLegacy(ctx, startTime, endTime)
	}
	stats, err := r.getOverviewStatsPreaggregated(ctx, startTime, endTime)
	if err == nil {
		return stats, nil
	}
	return r.GetOverviewStatsLegacy(ctx, startTime, endTime)
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
	if p999.Valid {
		stats.LatencyP999 = int(p999.Float64)
	}
	if avgLatency.Valid {
		stats.LatencyAvg = int(avgLatency.Float64)
	}
	if maxLatency.Valid {
		stats.LatencyMax = int(maxLatency.Float64)
	}

	return &stats, nil
}

// GetProviderStats backs the "provider health" dashboard view.
//
// With `ops.use_preaggregated_tables=true`, it sums ops_metrics_hourly/ops_metrics_daily
// for full buckets and uses the legacy raw-log query only for the newest <1h slice
// (and for small boundary fragments that don't align to full hour buckets).
func (r *OpsRepository) GetProviderStats(ctx context.Context, startTime, endTime time.Time) ([]*service.ProviderStats, error) {
	if !r.usePreaggregatedTables {
		return r.GetProviderStatsLegacy(ctx, startTime, endTime)
	}
	stats, err := r.getProviderStatsPreaggregated(ctx, startTime, endTime)
	if err == nil {
		return stats, nil
	}
	return r.GetProviderStatsLegacy(ctx, startTime, endTime)
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

// GetLatencyHistogram returns a coarse latency histogram used by the ops UI.
//
// Note: ops_metrics_hourly/daily do not store per-request latency distributions; when
// pre-aggregation is enabled this method approximates older data by bucketing each
// aggregated row by its avg_latency_ms (weighted by success_count), and uses raw logs
// for the newest <1h slice.
func (r *OpsRepository) GetLatencyHistogram(ctx context.Context, startTime, endTime time.Time) ([]*service.LatencyHistogramItem, error) {
	if !r.usePreaggregatedTables {
		return r.GetLatencyHistogramLegacy(ctx, startTime, endTime)
	}
	items, err := r.getLatencyHistogramPreaggregated(ctx, startTime, endTime)
	if err == nil {
		return items, nil
	}
	return r.GetLatencyHistogramLegacy(ctx, startTime, endTime)
}

// GetLatencyHistogramLegacy keeps the original raw-log implementation.
func (r *OpsRepository) GetLatencyHistogramLegacy(ctx context.Context, startTime, endTime time.Time) ([]*service.LatencyHistogramItem, error) {
	query := `
		WITH buckets AS (
			SELECT
				CASE
					WHEN duration_ms < 200 THEN '<200ms'
					WHEN duration_ms < 500 THEN '200-500ms'
					WHEN duration_ms < 1000 THEN '500-1000ms'
					WHEN duration_ms < 3000 THEN '1000-3000ms'
					ELSE '>3000ms'
				END AS range_name,
				CASE
					WHEN duration_ms < 200 THEN 1
					WHEN duration_ms < 500 THEN 2
					WHEN duration_ms < 1000 THEN 3
					WHEN duration_ms < 3000 THEN 4
					ELSE 5
				END AS range_order,
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
	`

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

func (r *OpsRepository) getWindowStatsPreaggregated(ctx context.Context, startTime, endTime time.Time) (*service.OpsWindowStats, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return r.GetWindowStatsLegacy(ctx, startTime, endTime)
	}

	aggSafeEnd := r.preaggSafeEnd(endTime)
	aggFullStart := utcCeilToHour(startTime)
	aggFullEnd := utcFloorToHour(aggSafeEnd)

	// If there are no stable full-hour buckets, keep the raw-log path (real-time windows).
	if !aggFullStart.Before(aggFullEnd) {
		return r.GetWindowStatsLegacy(ctx, startTime, endTime)
	}

	agg, aggErr := r.queryOpsAggSummary(ctx, aggFullStart, aggFullEnd)
	if aggErr != nil {
		return nil, aggErr
	}

	// If aggregates returned no data but raw logs do have rows, treat it as "not populated yet"
	// and fall back to the legacy query for correctness.
	if agg.requestCount == 0 && agg.successCount == 0 && agg.errorCount == 0 {
		if exists, err := r.rawOpsDataExists(ctx, aggFullStart, aggFullEnd); err == nil && exists {
			return nil, errors.New("ops pre-aggregated tables not populated")
		}
	}

	// Build a conservative approximation for the portion served by ops_metrics_*.
	out := &service.OpsWindowStats{
		SuccessCount:  agg.successCount,
		ErrorCount:    agg.errorCount,
		Error4xxCount: agg.error4xxCount,
		Error5xxCount: agg.error5xxCount,
		TimeoutCount:  agg.timeoutCount,
	}
	if agg.avgLatencyWeight > 0 {
		out.AvgLatencyMs = int(math.Round(agg.avgLatencyWeightedSum / float64(agg.avgLatencyWeight)))
	}
	out.P99LatencyMs = int(math.Round(agg.p99LatencyMax))
	out.P50LatencyMs = out.P99LatencyMs
	out.P95LatencyMs = out.P99LatencyMs
	out.P999LatencyMs = out.P99LatencyMs
	out.MaxLatencyMs = out.P99LatencyMs

	// Raw-log tail/head fragments:
	// - Head: [startTime, aggFullStart)
	// - Tail: [aggFullEnd, endTime) (includes the newest <1h slice)
	if startTime.Before(aggFullStart) {
		part, err := r.GetWindowStatsLegacy(ctx, startTime, minTime(endTime, aggFullStart))
		if err != nil {
			return nil, err
		}
		mergeWindowStats(out, part)
	}
	if aggFullEnd.Before(endTime) {
		part, err := r.GetWindowStatsLegacy(ctx, maxTime(startTime, aggFullEnd), endTime)
		if err != nil {
			return nil, err
		}
		mergeWindowStats(out, part)
	}
	return out, nil
}

func (r *OpsRepository) getOverviewStatsPreaggregated(ctx context.Context, startTime, endTime time.Time) (*service.OverviewStats, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return r.GetOverviewStatsLegacy(ctx, startTime, endTime)
	}

	aggSafeEnd := r.preaggSafeEnd(endTime)
	aggFullStart := utcCeilToHour(startTime)
	aggFullEnd := utcFloorToHour(aggSafeEnd)

	// No stable full-hour buckets => use legacy (typically the default "1h" dashboard view).
	if !aggFullStart.Before(aggFullEnd) {
		return r.GetOverviewStatsLegacy(ctx, startTime, endTime)
	}

	agg, aggErr := r.queryOpsAggSummary(ctx, aggFullStart, aggFullEnd)
	if aggErr != nil {
		return nil, aggErr
	}
	if agg.requestCount == 0 && agg.successCount == 0 && agg.errorCount == 0 {
		if exists, err := r.rawOpsDataExists(ctx, aggFullStart, aggFullEnd); err == nil && exists {
			return nil, errors.New("ops pre-aggregated tables not populated")
		}
	}

	out := &service.OverviewStats{
		RequestCount:  agg.requestCount,
		SuccessCount:  agg.successCount,
		ErrorCount:    agg.errorCount,
		Error4xxCount: agg.error4xxCount,
		Error5xxCount: agg.error5xxCount,
		TimeoutCount:  agg.timeoutCount,
	}
	if agg.avgLatencyWeight > 0 {
		out.LatencyAvg = int(math.Round(agg.avgLatencyWeightedSum / float64(agg.avgLatencyWeight)))
	}
	out.LatencyP99 = int(math.Round(agg.p99LatencyMax))
	out.LatencyP50 = out.LatencyP99
	out.LatencyP95 = out.LatencyP99
	out.LatencyP999 = out.LatencyP99
	out.LatencyMax = out.LatencyP99

	// Bring in raw-log boundary fragments (small) and let them "win" for percentiles/top error.
	if startTime.Before(aggFullStart) {
		part, err := r.GetOverviewStatsLegacy(ctx, startTime, minTime(endTime, aggFullStart))
		if err != nil {
			return nil, err
		}
		mergeOverviewStats(out, part)
	}
	if aggFullEnd.Before(endTime) {
		part, err := r.GetOverviewStatsLegacy(ctx, maxTime(startTime, aggFullEnd), endTime)
		if err != nil {
			return nil, err
		}
		mergeOverviewStats(out, part)
	}

	// Always attach latest system snapshot (independent of the requested time window).
	if snap, err := r.getLatestSystemSnapshot(ctx); err == nil && snap != nil {
		out.CPUUsage = snap.CPUUsage
		out.MemoryUsage = snap.MemoryUsage
		out.MemoryUsedMB = snap.MemoryUsedMB
		out.MemoryTotalMB = snap.MemoryTotalMB
		out.ConcurrencyQueueDepth = snap.ConcurrencyQueueDepth
	}
	return out, nil
}

func (r *OpsRepository) getProviderStatsPreaggregated(ctx context.Context, startTime, endTime time.Time) ([]*service.ProviderStats, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return r.GetProviderStatsLegacy(ctx, startTime, endTime)
	}

	aggSafeEnd := r.preaggSafeEnd(endTime)
	aggFullStart := utcCeilToHour(startTime)
	aggFullEnd := utcFloorToHour(aggSafeEnd)
	if !aggFullStart.Before(aggFullEnd) {
		return r.GetProviderStatsLegacy(ctx, startTime, endTime)
	}

	aggByPlatform, err := r.queryProviderAgg(ctx, aggFullStart, aggFullEnd)
	if err != nil {
		return nil, err
	}
	if len(aggByPlatform) == 0 {
		if exists, err := r.rawOpsDataExists(ctx, aggFullStart, aggFullEnd); err == nil && exists {
			return nil, errors.New("ops pre-aggregated tables not populated")
		}
	}

	// Merge in raw head/tail fragments.
	if startTime.Before(aggFullStart) {
		items, err := r.GetProviderStatsLegacy(ctx, startTime, minTime(endTime, aggFullStart))
		if err != nil {
			return nil, err
		}
		mergeProviderStatsAgg(aggByPlatform, items)
	}
	if aggFullEnd.Before(endTime) {
		items, err := r.GetProviderStatsLegacy(ctx, maxTime(startTime, aggFullEnd), endTime)
		if err != nil {
			return nil, err
		}
		mergeProviderStatsAgg(aggByPlatform, items)
	}

	results := make([]*service.ProviderStats, 0, len(aggByPlatform))
	for platform, acc := range aggByPlatform {
		if strings.TrimSpace(platform) == "" {
			continue
		}
		item := &service.ProviderStats{
			Platform:      platform,
			RequestCount:  acc.requestCount,
			SuccessCount:  acc.successCount,
			ErrorCount:    acc.errorCount,
			Error4xxCount: acc.error4xxCount,
			Error5xxCount: acc.error5xxCount,
			TimeoutCount:  acc.timeoutCount,
		}
		if acc.avgLatencyWeight > 0 {
			item.AvgLatencyMs = int(math.Round(acc.avgLatencyWeightedSum / float64(acc.avgLatencyWeight)))
		}
		item.P99LatencyMs = int(math.Round(acc.p99LatencyMax))
		results = append(results, item)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].RequestCount == results[j].RequestCount {
			return results[i].Platform < results[j].Platform
		}
		return results[i].RequestCount > results[j].RequestCount
	})
	return results, nil
}

func (r *OpsRepository) getLatencyHistogramPreaggregated(ctx context.Context, startTime, endTime time.Time) ([]*service.LatencyHistogramItem, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return r.GetLatencyHistogramLegacy(ctx, startTime, endTime)
	}

	aggSafeEnd := r.preaggSafeEnd(endTime)
	aggFullStart := utcCeilToHour(startTime)
	aggFullEnd := utcFloorToHour(aggSafeEnd)
	if !aggFullStart.Before(aggFullEnd) {
		return r.GetLatencyHistogramLegacy(ctx, startTime, endTime)
	}

	counts, err := r.queryLatencyHistogramCounts(ctx, aggFullStart, aggFullEnd)
	if err != nil {
		return nil, err
	}
	if len(counts) == 0 {
		if exists, err := r.rawOpsDataExists(ctx, aggFullStart, aggFullEnd); err == nil && exists {
			return nil, errors.New("ops pre-aggregated tables not populated")
		}
	}

	// Merge in raw head/tail fragments.
	if startTime.Before(aggFullStart) {
		items, err := r.GetLatencyHistogramLegacy(ctx, startTime, minTime(endTime, aggFullStart))
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if it != nil {
				counts[it.Range] += it.Count
			}
		}
	}
	if aggFullEnd.Before(endTime) {
		items, err := r.GetLatencyHistogramLegacy(ctx, maxTime(startTime, aggFullEnd), endTime)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if it != nil {
				counts[it.Range] += it.Count
			}
		}
	}

	total := int64(0)
	for _, c := range counts {
		total += c
	}
	if total <= 0 {
		return []*service.LatencyHistogramItem{}, nil
	}

	type orderedRange struct {
		name  string
		order int
	}
	ordered := []orderedRange{
		{name: "<200ms", order: 1},
		{name: "200-500ms", order: 2},
		{name: "500-1000ms", order: 3},
		{name: "1000-3000ms", order: 4},
		{name: ">3000ms", order: 5},
	}

	out := make([]*service.LatencyHistogramItem, 0, len(ordered))
	for _, r := range ordered {
		count := counts[r.name]
		if count <= 0 {
			continue
		}
		out = append(out, &service.LatencyHistogramItem{
			Range:      r.name,
			Count:      count,
			Percentage: math.Round((float64(count)/float64(total))*10000) / 100,
		})
	}
	return out, nil
}

func (r *OpsRepository) queryOpsAggSummary(ctx context.Context, startTime, endTime time.Time) (opsAggSummary, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if !startTime.Before(endTime) {
		return opsAggSummary{}, nil
	}

	// Optimization:
	// - For short time ranges (<= 24h), use the hourly table for precision without scanning many daily buckets.
	// - For longer ranges (> 24h), use daily buckets for the full-day middle segment, and hourly buckets for the
	//   remaining partial-day hours at the edges (to preserve exact semantics without falling back to raw logs).
	if endTime.Sub(startTime) <= 24*time.Hour {
		return r.queryOpsAggSummaryHourly(ctx, startTime, endTime)
	}

	var out opsAggSummary

	// Prefer daily for full-day buckets (if any), then fill the remaining hours from hourly.
	dayStart := utcCeilToDay(startTime)
	dayEnd := utcFloorToDay(endTime)

	// 1) Hourly head segment: [startTime, min(dayStart, endTime))
	headEnd := minTime(dayStart, endTime)
	if startTime.Before(headEnd) {
		hStart := utcCeilToHour(startTime)
		hEnd := utcFloorToHour(headEnd)
		if hStart.Before(hEnd) {
			part, err := r.queryOpsAggSummaryHourly(ctx, hStart, hEnd)
			if err != nil {
				return opsAggSummary{}, err
			}
			mergeOpsAggSummary(&out, &part)
		}
	}

	// 2) Daily middle segment: [dayStart, dayEnd)
	if dayStart.Before(dayEnd) {
		part, err := r.queryOpsAggSummaryDaily(ctx, dayStart, dayEnd)
		if err != nil {
			return opsAggSummary{}, err
		}

		// If daily isn't populated yet, fall back to raw-log queries at the method level.
		if part.requestCount == 0 && part.successCount == 0 && part.errorCount == 0 {
			if exists, err := r.rawOpsDataExists(ctx, dayStart, dayEnd); err == nil && exists {
				return opsAggSummary{}, errors.New("ops pre-aggregated tables not populated")
			}
		}
		mergeOpsAggSummary(&out, &part)
	}

	// 3) Hourly tail segment: [max(dayEnd, startTime), endTime)
	tailStart := maxTime(dayEnd, startTime)
	if tailStart.Before(endTime) {
		hStart := utcCeilToHour(tailStart)
		hEnd := utcFloorToHour(endTime)
		if hStart.Before(hEnd) {
			part, err := r.queryOpsAggSummaryHourly(ctx, hStart, hEnd)
			if err != nil {
				return opsAggSummary{}, err
			}
			mergeOpsAggSummary(&out, &part)
		}
	}

	return out, nil
}

func (r *OpsRepository) queryOpsAggSummaryHourly(ctx context.Context, startTime, endTime time.Time) (opsAggSummary, error) {
	var out opsAggSummary
	var avgWeightedSum sql.NullFloat64
	var avgWeight sql.NullInt64
	var p99 sql.NullFloat64

	err := scanSingleRow(
		ctx,
		r.sql,
		`
			SELECT
				COALESCE(SUM(request_count), 0) AS request_count,
				COALESCE(SUM(success_count), 0) AS success_count,
				COALESCE(SUM(error_count), 0) AS error_count,
				COALESCE(SUM(error_4xx_count), 0) AS error_4xx_count,
				COALESCE(SUM(error_5xx_count), 0) AS error_5xx_count,
				COALESCE(SUM(timeout_count), 0) AS timeout_count,
				SUM(avg_latency_ms * success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weighted_sum,
				SUM(success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weight,
				MAX(p99_latency_ms) AS p99_latency_max
			FROM ops_metrics_hourly
			WHERE bucket_start >= $1 AND bucket_start < $2
		`,
		[]any{startTime, endTime},
		&out.requestCount,
		&out.successCount,
		&out.errorCount,
		&out.error4xxCount,
		&out.error5xxCount,
		&out.timeoutCount,
		&avgWeightedSum,
		&avgWeight,
		&p99,
	)
	if err != nil {
		return opsAggSummary{}, err
	}
	if avgWeightedSum.Valid {
		out.avgLatencyWeightedSum = avgWeightedSum.Float64
	}
	if avgWeight.Valid {
		out.avgLatencyWeight = avgWeight.Int64
	}
	if p99.Valid {
		out.p99LatencyMax = p99.Float64
	}
	return out, nil
}

func (r *OpsRepository) queryOpsAggSummaryDaily(ctx context.Context, startTime, endTime time.Time) (opsAggSummary, error) {
	var out opsAggSummary
	var avgWeightedSum sql.NullFloat64
	var avgWeight sql.NullInt64
	var p99 sql.NullFloat64

	err := scanSingleRow(
		ctx,
		r.sql,
		`
			SELECT
				COALESCE(SUM(request_count), 0) AS request_count,
				COALESCE(SUM(success_count), 0) AS success_count,
				COALESCE(SUM(error_count), 0) AS error_count,
				COALESCE(SUM(error_4xx_count), 0) AS error_4xx_count,
				COALESCE(SUM(error_5xx_count), 0) AS error_5xx_count,
				COALESCE(SUM(timeout_count), 0) AS timeout_count,
				SUM(avg_latency_ms * success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weighted_sum,
				SUM(success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weight,
				MAX(p99_latency_ms) AS p99_latency_max
			FROM ops_metrics_daily
			WHERE bucket_date >= $1::date AND bucket_date < $2::date
		`,
		[]any{startTime, endTime},
		&out.requestCount,
		&out.successCount,
		&out.errorCount,
		&out.error4xxCount,
		&out.error5xxCount,
		&out.timeoutCount,
		&avgWeightedSum,
		&avgWeight,
		&p99,
	)
	if err != nil {
		return opsAggSummary{}, err
	}
	if avgWeightedSum.Valid {
		out.avgLatencyWeightedSum = avgWeightedSum.Float64
	}
	if avgWeight.Valid {
		out.avgLatencyWeight = avgWeight.Int64
	}
	if p99.Valid {
		out.p99LatencyMax = p99.Float64
	}
	return out, nil
}

func (r *OpsRepository) queryProviderAgg(ctx context.Context, startTime, endTime time.Time) (map[string]*providerStatsAgg, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if !startTime.Before(endTime) {
		return map[string]*providerStatsAgg{}, nil
	}

	out := make(map[string]*providerStatsAgg)

	// Same optimization rule as queryOpsAggSummary:
	// - <=24h: hourly only
	// - >24h: daily full days + hourly edges
	if endTime.Sub(startTime) <= 24*time.Hour {
		if err := r.mergeProviderAggHourly(ctx, out, startTime, endTime); err != nil {
			return nil, err
		}
		return out, nil
	}

	dayStart := utcCeilToDay(startTime)
	dayEnd := utcFloorToDay(endTime)

	// Hourly head segment.
	headEnd := minTime(dayStart, endTime)
	if startTime.Before(headEnd) {
		hStart := utcCeilToHour(startTime)
		hEnd := utcFloorToHour(headEnd)
		if hStart.Before(hEnd) {
			if err := r.mergeProviderAggHourly(ctx, out, hStart, hEnd); err != nil {
				return nil, err
			}
		}
	}

	// Daily middle segment (fall back to raw-log queries if daily not populated).
	if dayStart.Before(dayEnd) {
		dailyRows, err := r.queryProviderAggDaily(ctx, dayStart, dayEnd)
		if err != nil {
			return nil, err
		}
		if len(dailyRows) == 0 {
			if exists, err := r.rawOpsDataExists(ctx, dayStart, dayEnd); err == nil && exists {
				return nil, errors.New("ops pre-aggregated tables not populated")
			}
			// No data in raw logs either -> nothing to merge.
		} else {
			mergeProviderAggMap(out, dailyRows)
		}
	}

	// Hourly tail segment.
	tailStart := maxTime(dayEnd, startTime)
	if tailStart.Before(endTime) {
		hStart := utcCeilToHour(tailStart)
		hEnd := utcFloorToHour(endTime)
		if hStart.Before(hEnd) {
			if err := r.mergeProviderAggHourly(ctx, out, hStart, hEnd); err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

func (r *OpsRepository) mergeProviderAggHourly(ctx context.Context, acc map[string]*providerStatsAgg, startTime, endTime time.Time) error {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			platform,
			COALESCE(SUM(request_count), 0) AS request_count,
			COALESCE(SUM(success_count), 0) AS success_count,
			COALESCE(SUM(error_count), 0) AS error_count,
			COALESCE(SUM(error_4xx_count), 0) AS error_4xx_count,
			COALESCE(SUM(error_5xx_count), 0) AS error_5xx_count,
			COALESCE(SUM(timeout_count), 0) AS timeout_count,
			SUM(avg_latency_ms * success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weighted_sum,
			SUM(success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weight,
			MAX(p99_latency_ms) AS p99_latency_max
		FROM ops_metrics_hourly
		WHERE bucket_start >= $1 AND bucket_start < $2
		GROUP BY platform
	`, startTime, endTime)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var platform string
		var row providerStatsAgg
		var avgWeightedSum sql.NullFloat64
		var avgWeight sql.NullInt64
		var p99 sql.NullFloat64
		if err := rows.Scan(
			&platform,
			&row.requestCount,
			&row.successCount,
			&row.errorCount,
			&row.error4xxCount,
			&row.error5xxCount,
			&row.timeoutCount,
			&avgWeightedSum,
			&avgWeight,
			&p99,
		); err != nil {
			return err
		}
		if avgWeightedSum.Valid {
			row.avgLatencyWeightedSum = avgWeightedSum.Float64
		}
		if avgWeight.Valid {
			row.avgLatencyWeight = avgWeight.Int64
		}
		if p99.Valid {
			row.p99LatencyMax = p99.Float64
		}

		existing := acc[platform]
		if existing == nil {
			existing = &providerStatsAgg{}
			acc[platform] = existing
		}
		mergeProviderAgg(existing, &row)
	}
	return rows.Err()
}

func (r *OpsRepository) queryProviderAggDaily(ctx context.Context, startTime, endTime time.Time) (map[string]*providerStatsAgg, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			platform,
			COALESCE(SUM(request_count), 0) AS request_count,
			COALESCE(SUM(success_count), 0) AS success_count,
			COALESCE(SUM(error_count), 0) AS error_count,
			COALESCE(SUM(error_4xx_count), 0) AS error_4xx_count,
			COALESCE(SUM(error_5xx_count), 0) AS error_5xx_count,
			COALESCE(SUM(timeout_count), 0) AS timeout_count,
			SUM(avg_latency_ms * success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weighted_sum,
			SUM(success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weight,
			MAX(p99_latency_ms) AS p99_latency_max
		FROM ops_metrics_daily
		WHERE bucket_date >= $1::date AND bucket_date < $2::date
		GROUP BY platform
	`, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]*providerStatsAgg)
	for rows.Next() {
		var platform string
		var row providerStatsAgg
		var avgWeightedSum sql.NullFloat64
		var avgWeight sql.NullInt64
		var p99 sql.NullFloat64
		if err := rows.Scan(
			&platform,
			&row.requestCount,
			&row.successCount,
			&row.errorCount,
			&row.error4xxCount,
			&row.error5xxCount,
			&row.timeoutCount,
			&avgWeightedSum,
			&avgWeight,
			&p99,
		); err != nil {
			return nil, err
		}
		if avgWeightedSum.Valid {
			row.avgLatencyWeightedSum = avgWeightedSum.Float64
		}
		if avgWeight.Valid {
			row.avgLatencyWeight = avgWeight.Int64
		}
		if p99.Valid {
			row.p99LatencyMax = p99.Float64
		}
		out[platform] = &row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *OpsRepository) queryLatencyHistogramCounts(ctx context.Context, startTime, endTime time.Time) (map[string]int64, error) {
	counts := make(map[string]int64)

	// Keep behavior consistent with other ops_* pre-aggregation queries:
	// - <=24h: hourly only
	// - >24h: daily full days + hourly edges
	if endTime.Sub(startTime) <= 24*time.Hour {
		part, err := r.queryLatencyHistogramCountsHourly(ctx, startTime, endTime)
		if err != nil {
			return nil, err
		}
		mergeHistogramCounts(counts, part)
		return counts, nil
	}

	dayStart := utcCeilToDay(startTime)
	dayEnd := utcFloorToDay(endTime)

	// Hourly head segment.
	headEnd := minTime(dayStart, endTime)
	if startTime.Before(headEnd) {
		hStart := utcCeilToHour(startTime)
		hEnd := utcFloorToHour(headEnd)
		if hStart.Before(hEnd) {
			part, err := r.queryLatencyHistogramCountsHourly(ctx, hStart, hEnd)
			if err != nil {
				return nil, err
			}
			mergeHistogramCounts(counts, part)
		}
	}

	// Daily middle segment (fall back to raw-log queries if daily not populated).
	if dayStart.Before(dayEnd) {
		part, err := r.queryLatencyHistogramCountsDaily(ctx, dayStart, dayEnd)
		if err != nil {
			return nil, err
		}
		if len(part) == 0 {
			if exists, err := r.rawOpsDataExists(ctx, dayStart, dayEnd); err == nil && exists {
				return nil, errors.New("ops pre-aggregated tables not populated")
			}
		}
		mergeHistogramCounts(counts, part)
	}

	// Hourly tail segment.
	tailStart := maxTime(dayEnd, startTime)
	if tailStart.Before(endTime) {
		hStart := utcCeilToHour(tailStart)
		hEnd := utcFloorToHour(endTime)
		if hStart.Before(hEnd) {
			part, err := r.queryLatencyHistogramCountsHourly(ctx, hStart, hEnd)
			if err != nil {
				return nil, err
			}
			mergeHistogramCounts(counts, part)
		}
	}

	return counts, nil
}

func (r *OpsRepository) queryLatencyHistogramCountsHourly(ctx context.Context, startTime, endTime time.Time) (map[string]int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			CASE
				WHEN avg_latency_ms < 200 THEN '<200ms'
				WHEN avg_latency_ms < 500 THEN '200-500ms'
				WHEN avg_latency_ms < 1000 THEN '500-1000ms'
				WHEN avg_latency_ms < 3000 THEN '1000-3000ms'
				ELSE '>3000ms'
			END AS range_name,
			COALESCE(SUM(success_count), 0) AS count
		FROM ops_metrics_hourly
		WHERE bucket_start >= $1 AND bucket_start < $2 AND avg_latency_ms IS NOT NULL
		GROUP BY 1
	`, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]int64)
	for rows.Next() {
		var name string
		var c int64
		if err := rows.Scan(&name, &c); err != nil {
			return nil, err
		}
		out[name] += c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *OpsRepository) queryLatencyHistogramCountsDaily(ctx context.Context, startTime, endTime time.Time) (map[string]int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			CASE
				WHEN avg_latency_ms < 200 THEN '<200ms'
				WHEN avg_latency_ms < 500 THEN '200-500ms'
				WHEN avg_latency_ms < 1000 THEN '500-1000ms'
				WHEN avg_latency_ms < 3000 THEN '1000-3000ms'
				ELSE '>3000ms'
			END AS range_name,
			COALESCE(SUM(success_count), 0) AS count
		FROM ops_metrics_daily
		WHERE bucket_date >= $1::date AND bucket_date < $2::date AND avg_latency_ms IS NOT NULL
		GROUP BY 1
	`, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]int64)
	for rows.Next() {
		var name string
		var c int64
		if err := rows.Scan(&name, &c); err != nil {
			return nil, err
		}
		out[name] += c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func mergeOpsAggSummary(dst, src *opsAggSummary) {
	dst.requestCount += src.requestCount
	dst.successCount += src.successCount
	dst.errorCount += src.errorCount
	dst.error4xxCount += src.error4xxCount
	dst.error5xxCount += src.error5xxCount
	dst.timeoutCount += src.timeoutCount
	dst.avgLatencyWeightedSum += src.avgLatencyWeightedSum
	dst.avgLatencyWeight += src.avgLatencyWeight
	if src.p99LatencyMax > dst.p99LatencyMax {
		dst.p99LatencyMax = src.p99LatencyMax
	}
}

func mergeProviderAgg(dst, src *providerStatsAgg) {
	dst.requestCount += src.requestCount
	dst.successCount += src.successCount
	dst.errorCount += src.errorCount
	dst.error4xxCount += src.error4xxCount
	dst.error5xxCount += src.error5xxCount
	dst.timeoutCount += src.timeoutCount
	dst.avgLatencyWeightedSum += src.avgLatencyWeightedSum
	dst.avgLatencyWeight += src.avgLatencyWeight
	if src.p99LatencyMax > dst.p99LatencyMax {
		dst.p99LatencyMax = src.p99LatencyMax
	}
}

func mergeProviderAggMap(dst map[string]*providerStatsAgg, src map[string]*providerStatsAgg) {
	for platform, row := range src {
		if row == nil {
			continue
		}
		existing := dst[platform]
		if existing == nil {
			existing = &providerStatsAgg{}
			dst[platform] = existing
		}
		mergeProviderAgg(existing, row)
	}
}

func mergeProviderStatsAgg(dst map[string]*providerStatsAgg, raw []*service.ProviderStats) {
	for _, it := range raw {
		if it == nil {
			continue
		}
		existing := dst[it.Platform]
		if existing == nil {
			existing = &providerStatsAgg{}
			dst[it.Platform] = existing
		}
		existing.requestCount += it.RequestCount
		existing.successCount += it.SuccessCount
		existing.errorCount += it.ErrorCount
		existing.error4xxCount += it.Error4xxCount
		existing.error5xxCount += it.Error5xxCount
		existing.timeoutCount += it.TimeoutCount
		if it.SuccessCount > 0 && it.AvgLatencyMs > 0 {
			existing.avgLatencyWeightedSum += float64(it.AvgLatencyMs) * float64(it.SuccessCount)
			existing.avgLatencyWeight += it.SuccessCount
		}
		if float64(it.P99LatencyMs) > existing.p99LatencyMax {
			existing.p99LatencyMax = float64(it.P99LatencyMs)
		}
	}
}

func mergeHistogramCounts(dst map[string]int64, src map[string]int64) {
	for k, v := range src {
		dst[k] += v
	}
}

func mergeWindowStats(dst *service.OpsWindowStats, src *service.OpsWindowStats) {
	if dst == nil || src == nil {
		return
	}
	dst.SuccessCount += src.SuccessCount
	dst.ErrorCount += src.ErrorCount
	dst.Error4xxCount += src.Error4xxCount
	dst.Error5xxCount += src.Error5xxCount
	dst.TimeoutCount += src.TimeoutCount
	dst.HTTP2Errors += src.HTTP2Errors
	dst.TokenConsumed += src.TokenConsumed

	// Conservative merge for latency percentiles: keep the worst/highest observed.
	if src.P50LatencyMs > dst.P50LatencyMs {
		dst.P50LatencyMs = src.P50LatencyMs
	}
	if src.P95LatencyMs > dst.P95LatencyMs {
		dst.P95LatencyMs = src.P95LatencyMs
	}
	if src.P99LatencyMs > dst.P99LatencyMs {
		dst.P99LatencyMs = src.P99LatencyMs
	}
	if src.P999LatencyMs > dst.P999LatencyMs {
		dst.P999LatencyMs = src.P999LatencyMs
	}
	if src.MaxLatencyMs > dst.MaxLatencyMs {
		dst.MaxLatencyMs = src.MaxLatencyMs
	}

	// Average latency is weighted by success_count (best available proxy).
	weightDst := dst.SuccessCount - src.SuccessCount
	weightSrc := src.SuccessCount
	if weightDst > 0 && weightSrc > 0 && dst.AvgLatencyMs > 0 && src.AvgLatencyMs > 0 {
		dst.AvgLatencyMs = int(math.Round(
			(float64(dst.AvgLatencyMs)*float64(weightDst) + float64(src.AvgLatencyMs)*float64(weightSrc)) /
				float64(weightDst+weightSrc),
		))
	} else if dst.AvgLatencyMs == 0 && src.AvgLatencyMs > 0 {
		dst.AvgLatencyMs = src.AvgLatencyMs
	}
}

func mergeOverviewStats(dst *service.OverviewStats, src *service.OverviewStats) {
	if dst == nil || src == nil {
		return
	}
	dst.RequestCount += src.RequestCount
	dst.SuccessCount += src.SuccessCount
	dst.ErrorCount += src.ErrorCount
	dst.Error4xxCount += src.Error4xxCount
	dst.Error5xxCount += src.Error5xxCount
	dst.TimeoutCount += src.TimeoutCount

	if src.TopErrorCount > dst.TopErrorCount {
		dst.TopErrorCode = src.TopErrorCode
		dst.TopErrorMsg = src.TopErrorMsg
		dst.TopErrorCount = src.TopErrorCount
	}

	if src.LatencyP50 > dst.LatencyP50 {
		dst.LatencyP50 = src.LatencyP50
	}
	if src.LatencyP95 > dst.LatencyP95 {
		dst.LatencyP95 = src.LatencyP95
	}
	if src.LatencyP99 > dst.LatencyP99 {
		dst.LatencyP99 = src.LatencyP99
	}
	if src.LatencyP999 > dst.LatencyP999 {
		dst.LatencyP999 = src.LatencyP999
	}
	if src.LatencyMax > dst.LatencyMax {
		dst.LatencyMax = src.LatencyMax
	}

	weightDst := dst.SuccessCount - src.SuccessCount
	weightSrc := src.SuccessCount
	if weightDst > 0 && weightSrc > 0 && dst.LatencyAvg > 0 && src.LatencyAvg > 0 {
		dst.LatencyAvg = int(math.Round(
			(float64(dst.LatencyAvg)*float64(weightDst) + float64(src.LatencyAvg)*float64(weightSrc)) /
				float64(weightDst+weightSrc),
		))
	} else if dst.LatencyAvg == 0 && src.LatencyAvg > 0 {
		dst.LatencyAvg = src.LatencyAvg
	}
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

func (r *OpsRepository) GetErrorDistribution(ctx context.Context, startTime, endTime time.Time) ([]*service.ErrorDistributionItem, error) {
	query := `
		WITH errors AS (
			SELECT
				COALESCE(status_code::text, 'unknown') AS code,
				COALESCE(error_message, 'Unknown error') AS message,
				COUNT(*) AS count
			FROM ops_error_logs
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY 1, 2
		),
		total AS (
			SELECT SUM(count) AS total_count FROM errors
		)
		SELECT
			e.code,
			e.message,
			e.count,
			ROUND((e.count::numeric / t.total_count) * 100, 2) AS percentage
		FROM errors e
		CROSS JOIN total t
		ORDER BY e.count DESC
		LIMIT 20
	`

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]*service.ErrorDistributionItem, 0)
	for rows.Next() {
		var item service.ErrorDistributionItem
		if err := rows.Scan(&item.Code, &item.Message, &item.Count, &item.Percentage); err != nil {
			return nil, err
		}
		results = append(results, &item)
	}
	return results, nil
}

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
			avg_latency_ms = EXCLUDED.avg_latency_ms,
			p99_latency_ms = EXCLUDED.p99_latency_ms,
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
			percentile_cont(0.99) WITHIN GROUP (ORDER BY h.p99_latency_ms)
				FILTER (WHERE h.p99_latency_ms IS NOT NULL) AS p99_latency_ms,
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
			avg_latency_ms = EXCLUDED.avg_latency_ms,
			p99_latency_ms = EXCLUDED.p99_latency_ms,
			error_rate = EXCLUDED.error_rate,
			computed_at = NOW()
	`

	_, err := r.sql.ExecContext(ctx, query, startTime, endTime)
	return err
}

func (r *OpsRepository) getAlertEvent(ctx context.Context, whereClause string, args []any) (*service.OpsAlertEvent, error) {
	query := fmt.Sprintf(`
		SELECT
			id,
			rule_id,
			severity,
			status,
			title,
			description,
			metric_value,
			threshold_value,
			fired_at,
			resolved_at,
			email_sent,
			webhook_sent,
			created_at
		FROM ops_alert_events
		%s
		ORDER BY fired_at DESC
		LIMIT 1
	`, whereClause)

	var event service.OpsAlertEvent
	var resolvedAt sql.NullTime
	var metricValue sql.NullFloat64
	var thresholdValue sql.NullFloat64
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		args,
		&event.ID,
		&event.RuleID,
		&event.Severity,
		&event.Status,
		&event.Title,
		&event.Description,
		&metricValue,
		&thresholdValue,
		&event.FiredAt,
		&resolvedAt,
		&event.EmailSent,
		&event.WebhookSent,
		&event.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if metricValue.Valid {
		event.MetricValue = metricValue.Float64
	}
	if thresholdValue.Valid {
		event.ThresholdValue = thresholdValue.Float64
	}
	if resolvedAt.Valid {
		event.ResolvedAt = &resolvedAt.Time
	}
	return &event, nil
}

func scanOpsSystemMetric(rows *sql.Rows) (*service.OpsMetrics, error) {
	var metric service.OpsMetrics
	var windowMinutes sql.NullInt64
	var requestCount, successCount, errorCount sql.NullInt64
	var qps, tps sql.NullFloat64
	var error4xxCount, error5xxCount, errorTimeoutCount sql.NullInt64
	var latencyP50, latencyP999, latencyAvg, latencyMax, upstreamLatencyAvg sql.NullFloat64
	var diskUsed, diskTotal, diskIOPS sql.NullInt64
	var networkInBytes, networkOutBytes sql.NullInt64
	var goroutineCount, dbConnActive, dbConnIdle, dbConnWaiting sql.NullInt64
	var tokenConsumed sql.NullInt64
	var tokenRate sql.NullFloat64
	var activeSubscriptions sql.NullInt64
	var tags []byte
	var successRate, errorRate sql.NullFloat64
	var p95Latency, p99Latency, http2Errors, activeAlerts sql.NullInt64
	var cpuUsage, memoryUsage, gcPause sql.NullFloat64
	var memoryUsed, memoryTotal, heapAlloc, queueDepth sql.NullInt64

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
		&latencyP999,
		&latencyAvg,
		&latencyMax,
		&upstreamLatencyAvg,
		&diskUsed,
		&diskTotal,
		&diskIOPS,
		&networkInBytes,
		&networkOutBytes,
		&goroutineCount,
		&dbConnActive,
		&dbConnIdle,
		&dbConnWaiting,
		&tokenConsumed,
		&tokenRate,
		&activeSubscriptions,
		&tags,
		&successRate,
		&errorRate,
		&p95Latency,
		&p99Latency,
		&http2Errors,
		&activeAlerts,
		&cpuUsage,
		&memoryUsed,
		&memoryTotal,
		&memoryUsage,
		&heapAlloc,
		&gcPause,
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
	if latencyP999.Valid {
		metric.LatencyP999 = latencyP999.Float64
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
	if diskUsed.Valid {
		metric.DiskUsed = diskUsed.Int64
	}
	if diskTotal.Valid {
		metric.DiskTotal = diskTotal.Int64
	}
	if diskIOPS.Valid {
		metric.DiskIOPS = diskIOPS.Int64
	}
	if networkInBytes.Valid {
		metric.NetworkInBytes = networkInBytes.Int64
	}
	if networkOutBytes.Valid {
		metric.NetworkOutBytes = networkOutBytes.Int64
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
	if len(tags) > 0 {
		_ = json.Unmarshal(tags, &metric.Tags)
	}
	if successRate.Valid {
		metric.SuccessRate = successRate.Float64
	}
	if errorRate.Valid {
		metric.ErrorRate = errorRate.Float64
	}
	if p95Latency.Valid {
		metric.P95LatencyMs = int(p95Latency.Int64)
	}
	if p99Latency.Valid {
		metric.P99LatencyMs = int(p99Latency.Int64)
	}
	if http2Errors.Valid {
		metric.HTTP2Errors = int(http2Errors.Int64)
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
	if heapAlloc.Valid {
		metric.HeapAllocMB = heapAlloc.Int64
	}
	if gcPause.Valid {
		metric.GCPauseMs = gcPause.Float64
	}
	if queueDepth.Valid {
		metric.ConcurrencyQueueDepth = int(queueDepth.Int64)
	}

	return &metric, nil
}

func scanOpsErrorLog(rows *sql.Rows) (*service.OpsErrorLog, error) {
	var entry service.OpsErrorLog
	var userID, apiKeyID, accountID, groupID sql.NullInt64
	var clientIP sql.NullString
	var statusCode sql.NullInt64
	var platform sql.NullString
	var model sql.NullString
	var requestPath sql.NullString
	var stream sql.NullBool
	var durationMs sql.NullInt64
	var requestID sql.NullString
	var message sql.NullString
	var errorBody sql.NullString
	var providerErrorCode sql.NullString
	var providerErrorType sql.NullString
	var isRetryable sql.NullBool
	var isUserActionable sql.NullBool
	var retryCount sql.NullInt64
	var completionStatus sql.NullString

	if err := rows.Scan(
		&entry.ID,
		&entry.CreatedAt,
		&userID,
		&apiKeyID,
		&accountID,
		&groupID,
		&clientIP,
		&entry.Phase,
		&entry.Type,
		&entry.Severity,
		&statusCode,
		&platform,
		&model,
		&requestPath,
		&stream,
		&durationMs,
		&requestID,
		&message,
		&errorBody,
		&providerErrorCode,
		&providerErrorType,
		&isRetryable,
		&isUserActionable,
		&retryCount,
		&completionStatus,
	); err != nil {
		return nil, err
	}

	if userID.Valid {
		v := userID.Int64
		entry.UserID = &v
	}
	if apiKeyID.Valid {
		v := apiKeyID.Int64
		entry.APIKeyID = &v
	}
	if accountID.Valid {
		v := accountID.Int64
		entry.AccountID = &v
	}
	if groupID.Valid {
		v := groupID.Int64
		entry.GroupID = &v
	}
	if clientIP.Valid {
		entry.ClientIP = clientIP.String
	}
	if statusCode.Valid {
		entry.StatusCode = int(statusCode.Int64)
	}
	if platform.Valid {
		entry.Platform = platform.String
	}
	if model.Valid {
		entry.Model = model.String
	}
	if requestPath.Valid {
		entry.RequestPath = requestPath.String
	}
	if stream.Valid {
		entry.Stream = stream.Bool
	}
	if durationMs.Valid {
		value := int(durationMs.Int64)
		entry.DurationMs = &value
		// For backward compatibility, also set LatencyMs
		entry.LatencyMs = &value
	}
	if requestID.Valid {
		entry.RequestID = requestID.String
	}
	if message.Valid {
		entry.Message = message.String
	}
	if errorBody.Valid {
		entry.ErrorBody = errorBody.String
	}
	if providerErrorCode.Valid {
		entry.ProviderErrorCode = providerErrorCode.String
	}
	if providerErrorType.Valid {
		entry.ProviderErrorType = providerErrorType.String
	}
	if isRetryable.Valid {
		entry.IsRetryable = isRetryable.Bool
	}
	if isUserActionable.Valid {
		entry.IsUserActionable = isUserActionable.Bool
	}
	if retryCount.Valid {
		entry.RetryCount = int(retryCount.Int64)
	}
	if completionStatus.Valid {
		entry.CompletionStatus = completionStatus.String
	}

	return &entry, nil
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullInt64Ptr(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func formatPostgresInterval(duration time.Duration) (string, error) {
	if duration < 0 {
		return "", fmt.Errorf("duration must be non-negative: %s", duration)
	}
	if duration == 0 {
		return "0 seconds", nil
	}

	remaining := duration
	hours := remaining / time.Hour
	remaining -= hours * time.Hour
	minutes := remaining / time.Minute
	remaining -= minutes * time.Minute
	seconds := remaining / time.Second
	remaining -= seconds * time.Second

	var parts []string
	if hours != 0 {
		unit := "hours"
		if hours == 1 {
			unit = "hour"
		}
		parts = append(parts, fmt.Sprintf("%d %s", hours, unit))
	}
	if minutes != 0 {
		unit := "minutes"
		if minutes == 1 {
			unit = "minute"
		}
		parts = append(parts, fmt.Sprintf("%d %s", minutes, unit))
	}

	secondsValue := float64(seconds)
	if remaining != 0 {
		secondsValue += float64(remaining) / float64(time.Second)
	}
	if secondsValue != 0 || len(parts) == 0 {
		secondsStr := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.9f", secondsValue), "0"), ".")
		unit := "seconds"
		if secondsStr == "1" {
			unit = "second"
		}
		parts = append(parts, fmt.Sprintf("%s %s", secondsStr, unit))
	}

	return strings.Join(parts, " "), nil
}

// GetAccountStats 获取账号统计数据
func (r *OpsRepository) GetAccountStats(ctx context.Context, accountID int64, duration time.Duration) (*service.AccountStats, error) {
	interval, err := formatPostgresInterval(duration)
	if err != nil {
		return nil, err
	}

	query := `
		WITH
		error_stats AS (
			SELECT
				COUNT(*) FILTER (WHERE status_code >= 400) AS error_count,
				COUNT(*) FILTER (WHERE error_type = 'timeout_error') AS timeout_count,
				COUNT(*) FILTER (WHERE error_type = 'rate_limit_error') AS rate_limit_count
			FROM ops_error_logs
			WHERE account_id = $1 AND created_at >= NOW() - $2::interval
		),
		usage_stats AS (
			SELECT
				COUNT(*) AS success_count
			FROM usage_logs
			WHERE account_id = $1 AND created_at >= NOW() - $2::interval
		)
		SELECT
			error_stats.error_count,
			usage_stats.success_count,
			error_stats.timeout_count,
			error_stats.rate_limit_count
		FROM error_stats
		CROSS JOIN usage_stats
	`

	var stats service.AccountStats
	err = r.sql.QueryRowContext(ctx, query, accountID, interval).Scan(
		&stats.ErrorCount,
		&stats.SuccessCount,
		&stats.TimeoutCount,
		&stats.RateLimitCount,
	)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetLastAccountError 获取账号最近一次错误
func (r *OpsRepository) GetLastAccountError(ctx context.Context, accountID int64) (*service.OpsErrorLog, error) {
	query := `
		SELECT id, created_at, error_type, error_message, platform, account_status
		FROM ops_error_logs
		WHERE account_id = $1 AND status_code >= 400
		ORDER BY created_at DESC
		LIMIT 1
	`

	var log service.OpsErrorLog
	err := r.sql.QueryRowContext(ctx, query, accountID).Scan(
		&log.ID,
		&log.CreatedAt,
		&log.Type,
		&log.Message,
		&log.Platform,
		&log.AccountStatus,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// UpsertAccountStatus 更新或插入账号状态
func (r *OpsRepository) UpsertAccountStatus(ctx context.Context, status *service.OpsAccountStatus) error {
	query := `
		INSERT INTO ops_account_status (
			account_id, platform, status, last_error_type, last_error_message,
			last_error_time, error_count_1h, success_count_1h, timeout_count_1h,
			rate_limit_count_1h, error_count_24h, success_count_24h,
			timeout_count_24h, rate_limit_count_24h, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (account_id, platform) DO UPDATE SET
			status = EXCLUDED.status,
			last_error_type = EXCLUDED.last_error_type,
			last_error_message = EXCLUDED.last_error_message,
			last_error_time = EXCLUDED.last_error_time,
			error_count_1h = EXCLUDED.error_count_1h,
			success_count_1h = EXCLUDED.success_count_1h,
			timeout_count_1h = EXCLUDED.timeout_count_1h,
			rate_limit_count_1h = EXCLUDED.rate_limit_count_1h,
			error_count_24h = EXCLUDED.error_count_24h,
			success_count_24h = EXCLUDED.success_count_24h,
			timeout_count_24h = EXCLUDED.timeout_count_24h,
			rate_limit_count_24h = EXCLUDED.rate_limit_count_24h,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.sql.ExecContext(ctx, query,
		status.AccountID,
		status.Platform,
		status.Status,
		status.LastErrorType,
		status.LastErrorMessage,
		status.LastErrorTime,
		status.ErrorCount1h,
		status.SuccessCount1h,
		status.TimeoutCount1h,
		status.RateLimitCount1h,
		status.ErrorCount24h,
		status.SuccessCount24h,
		status.TimeoutCount24h,
		status.RateLimitCount24h,
		status.UpdatedAt,
	)
	return err
}

// GetActiveAccounts 获取所有活跃账号ID
func (r *OpsRepository) GetActiveAccounts(ctx context.Context) ([]int64, error) {
	query := `
		SELECT DISTINCT account_id
		FROM ops_error_logs
		WHERE created_at >= NOW() - INTERVAL '24 hours'
		  AND account_id IS NOT NULL
	`

	rows, err := r.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []int64
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return nil, err
		}
		accounts = append(accounts, accountID)
	}
	return accounts, rows.Err()
}

// GetErrorStatsByIP 获取IP错误统计
func (r *OpsRepository) GetErrorStatsByIP(ctx context.Context, startTime, endTime time.Time, limit int, sortBy, sortOrder string) ([]service.IPErrorStats, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if sortBy != "error_count" && sortBy != "last_error_time" {
		sortBy = "error_count"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	query := fmt.Sprintf(`
		WITH error_type_counts AS (
			SELECT
				client_ip,
				error_type,
				COUNT(*) as type_count,
				MIN(created_at) as first_error,
				MAX(created_at) as last_error
			FROM ops_error_logs
			WHERE created_at >= $1 AND created_at < $2
				AND client_ip IS NOT NULL AND client_ip != ''
			GROUP BY client_ip, error_type
		)
		SELECT
			client_ip,
			SUM(type_count) as error_count,
			MIN(first_error) as first_error_time,
			MAX(last_error) as last_error_time,
			jsonb_object_agg(error_type, type_count) as error_types
		FROM error_type_counts
		GROUP BY client_ip
		ORDER BY %s %s
		LIMIT $3
	`, sortBy, sortOrder)

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []service.IPErrorStats
	for rows.Next() {
		var stat service.IPErrorStats
		var errorTypesJSON []byte
		if err := rows.Scan(&stat.ClientIP, &stat.ErrorCount, &stat.FirstErrorTime, &stat.LastErrorTime, &errorTypesJSON); err != nil {
			return nil, err
		}
		if len(errorTypesJSON) > 0 {
			if err := json.Unmarshal(errorTypesJSON, &stat.ErrorTypes); err != nil {
				return nil, err
			}
		}
		results = append(results, stat)
	}
	return results, rows.Err()
}

// GetErrorsByIP 获取特定IP的错误详情
func (r *OpsRepository) GetErrorsByIP(ctx context.Context, ip string, startTime, endTime time.Time, page, pageSize int) ([]service.OpsErrorLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	countQuery := `
		SELECT COUNT(*)
		FROM ops_error_logs
		WHERE client_ip = $1 AND created_at >= $2 AND created_at < $3
	`
	var total int64
	if err := r.sql.QueryRowContext(ctx, countQuery, ip, startTime, endTime).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			id, created_at, error_type, error_message, request_path,
			user_id, duration_ms, status_code, platform, model
		FROM ops_error_logs
		WHERE client_ip = $1 AND created_at >= $2 AND created_at < $3
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`

	offset := (page - 1) * pageSize
	rows, err := r.sql.QueryContext(ctx, query, ip, startTime, endTime, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []service.OpsErrorLog
	for rows.Next() {
		var log service.OpsErrorLog
		var userID sql.NullInt64
		var durationMs sql.NullInt64
		if err := rows.Scan(&log.ID, &log.CreatedAt, &log.Type, &log.Message, &log.RequestPath, &userID, &durationMs, &log.StatusCode, &log.Platform, &log.Model); err != nil {
			return nil, 0, err
		}
		if userID.Valid {
			uid := userID.Int64
			log.UserID = &uid
		}
		if durationMs.Valid {
			dm := int(durationMs.Int64)
			log.DurationMs = &dm
		}
		results = append(results, log)
	}
	return results, total, rows.Err()
}
