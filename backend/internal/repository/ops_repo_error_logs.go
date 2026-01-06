package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

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

func (r *OpsRepository) GetErrorDistribution(ctx context.Context, startTime, endTime time.Time) ([]*service.ErrorDistributionItem, error) {
	query := `
		WITH errors AS (
			SELECT
				COALESCE(status_code::text, 'unknown') AS code,
				COALESCE(error_type, 'unknown') AS message,
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

// GetAllActiveAccountStatus returns recent stats for all "active" accounts (as defined by ops_error_logs activity in last 24h).
func (r *OpsRepository) GetAllActiveAccountStatus(ctx context.Context) ([]service.AccountStatusSummary, error) {
	query := `
		WITH
		active_accounts AS (
			SELECT DISTINCT account_id
			FROM ops_error_logs
			WHERE created_at >= NOW() - INTERVAL '24 hours'
			  AND account_id IS NOT NULL
		),
		error_1h AS (
			SELECT
				account_id,
				COUNT(*) FILTER (WHERE status_code >= 400) AS error_count,
				COUNT(*) FILTER (WHERE error_type = 'timeout_error') AS timeout_count,
				COUNT(*) FILTER (WHERE error_type = 'rate_limit_error') AS rate_limit_count
			FROM ops_error_logs
			WHERE created_at >= NOW() - INTERVAL '1 hour'
			  AND account_id IS NOT NULL
			GROUP BY account_id
		),
		error_24h AS (
			SELECT
				account_id,
				COUNT(*) FILTER (WHERE status_code >= 400) AS error_count,
				COUNT(*) FILTER (WHERE error_type = 'timeout_error') AS timeout_count,
				COUNT(*) FILTER (WHERE error_type = 'rate_limit_error') AS rate_limit_count
			FROM ops_error_logs
			WHERE created_at >= NOW() - INTERVAL '24 hours'
			  AND account_id IS NOT NULL
			GROUP BY account_id
		),
		usage_1h AS (
			SELECT account_id, COUNT(*) AS success_count
			FROM usage_logs
			WHERE created_at >= NOW() - INTERVAL '1 hour'
			  AND account_id IS NOT NULL
			GROUP BY account_id
		),
		usage_24h AS (
			SELECT account_id, COUNT(*) AS success_count
			FROM usage_logs
			WHERE created_at >= NOW() - INTERVAL '24 hours'
			  AND account_id IS NOT NULL
			GROUP BY account_id
		)
		SELECT
			a.account_id,
			COALESCE(e1.error_count, 0) AS error_count_1h,
			COALESCE(u1.success_count, 0) AS success_count_1h,
			COALESCE(e1.timeout_count, 0) AS timeout_count_1h,
			COALESCE(e1.rate_limit_count, 0) AS rate_limit_count_1h,
			COALESCE(e24.error_count, 0) AS error_count_24h,
			COALESCE(u24.success_count, 0) AS success_count_24h,
			COALESCE(e24.timeout_count, 0) AS timeout_count_24h,
			COALESCE(e24.rate_limit_count, 0) AS rate_limit_count_24h
		FROM active_accounts a
		LEFT JOIN error_1h e1 ON e1.account_id = a.account_id
		LEFT JOIN error_24h e24 ON e24.account_id = a.account_id
		LEFT JOIN usage_1h u1 ON u1.account_id = a.account_id
		LEFT JOIN usage_24h u24 ON u24.account_id = a.account_id
		ORDER BY a.account_id
	`

	rows, err := r.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.AccountStatusSummary, 0, 64)
	for rows.Next() {
		var item service.AccountStatusSummary
		if err := rows.Scan(
			&item.AccountID,
			&item.Stats1h.ErrorCount,
			&item.Stats1h.SuccessCount,
			&item.Stats1h.TimeoutCount,
			&item.Stats1h.RateLimitCount,
			&item.Stats24h.ErrorCount,
			&item.Stats24h.SuccessCount,
			&item.Stats24h.TimeoutCount,
			&item.Stats24h.RateLimitCount,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *OpsRepository) GetErrorStatsByIP(ctx context.Context, startTime, endTime time.Time, limit int, sortBy, sortOrder string) ([]service.IPErrorStats, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	sortColumns := map[string]string{
		"error_count":     "error_count",
		"last_error_time": "last_error_time",
	}
	sortDirections := map[string]string{
		"asc":  "ASC",
		"desc": "DESC",
	}

	sortCol, ok := sortColumns[sortBy]
	if !ok {
		sortCol = "error_count"
	}
	sortDir, ok := sortDirections[sortOrder]
	if !ok {
		sortDir = "DESC"
	}

	// Performance notes (large datasets: million+ ops_error_logs rows):
	// - This query scans all rows in the [startTime, endTime) window, then aggregates twice:
	//   1) (client_ip, error_type) to get per-type counts and min/max timestamps
	//   2) (client_ip) to get total counts + jsonb_object_agg(error_type -> count)
	// - Validate real-world performance with a representative window and:
	//   EXPLAIN (ANALYZE, BUFFERS) <query>
	// - Index recommendations:
	//   - Per-IP drilldown queries (e.g. GetErrorsByIP) benefit from:
	//     CREATE INDEX idx_ops_error_logs_ip_time ON ops_error_logs(client_ip, created_at);
	//   - This aggregate query is primarily time-range driven; if EXPLAIN shows a Seq Scan / heavy heap fetches
	//     on large windows, consider an index that starts with created_at, e.g. (created_at, client_ip, error_type)
	//     or (created_at, client_ip) plus INCLUDE(error_type) (Postgres), depending on write/space tradeoffs.
	// - For very wide windows, consider time partitioning or pre-aggregation tables to avoid scanning raw logs.
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
				AND client_ip IS NOT NULL
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
	`, sortCol, sortDir)

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

// DeleteOldErrorLogs deletes error logs older than retentionDays in batches to avoid database overload.
// Returns the total count of deleted rows.
func (r *OpsRepository) DeleteOldErrorLogs(ctx context.Context, retentionDays int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	totalDeleted := int64(0)
	batchSize := 10000

	for {
		// Delete in batches to avoid WAL explosion and table lock issues
		result, err := r.sql.ExecContext(ctx, `
			DELETE FROM ops_error_logs
			WHERE id IN (
				SELECT id FROM ops_error_logs
				WHERE created_at < $1
				ORDER BY created_at ASC
				LIMIT $2
			)
		`, cutoffTime, batchSize)

		if err != nil {
			return totalDeleted, err
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			break
		}

		totalDeleted += rowsAffected

		// Pause between batches to avoid overwhelming the database
		time.Sleep(100 * time.Millisecond)
	}

	return totalDeleted, nil
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
