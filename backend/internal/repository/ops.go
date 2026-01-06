package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ListErrorLogs queries ops_error_logs with optional filters and pagination.
// It returns the list items and the total count of matching rows.
func (r *OpsRepository) ListErrorLogs(ctx context.Context, filter *service.ErrorLogFilter) ([]*service.OpsErrorLog, int64, error) {
	page := 1
	pageSize := 20
	if filter != nil {
		if filter.Page > 0 {
			page = filter.Page
		}
		if filter.PageSize > 0 {
			pageSize = filter.PageSize
		}
	}
	// Keep consistent with OpsService filter normalization and handler validation.
	if pageSize > 500 {
		pageSize = 500
	}
	offset := (page - 1) * pageSize

	conditions := make([]string, 0)
	args := make([]any, 0)

	addCondition := func(condition string, values ...any) {
		conditions = append(conditions, condition)
		args = append(args, values...)
	}

	if filter != nil {
		// 默认查询最近 24 小时
		if filter.StartTime == nil && filter.EndTime == nil {
			defaultStart := time.Now().Add(-24 * time.Hour)
			filter.StartTime = &defaultStart
		}

		if filter.StartTime != nil {
			addCondition(fmt.Sprintf("created_at >= $%d", len(args)+1), *filter.StartTime)
		}
		if filter.EndTime != nil {
			addCondition(fmt.Sprintf("created_at < $%d", len(args)+1), *filter.EndTime)
		}
		if filter.ErrorCode != nil {
			addCondition(fmt.Sprintf("status_code = $%d", len(args)+1), *filter.ErrorCode)
		}

		// 支持单值 Provider（向后兼容）和多值 Platforms
		if len(filter.Platforms) > 0 {
			// 多值平台过滤
			placeholders := make([]string, len(filter.Platforms))
			for i, p := range filter.Platforms {
				placeholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
				args = append(args, p)
			}
			conditions = append(conditions, fmt.Sprintf("platform = ANY(ARRAY[%s])", strings.Join(placeholders, ", ")))
		} else if provider := strings.TrimSpace(filter.Provider); provider != "" {
			// 单值平台过滤（向后兼容）
			addCondition(fmt.Sprintf("platform = $%d", len(args)+1), provider)
		}

		if phase := strings.TrimSpace(filter.Phase); phase != "" {
			addCondition(fmt.Sprintf("error_phase = $%d", len(args)+1), phase)
		}
		if severity := strings.TrimSpace(filter.Severity); severity != "" {
			addCondition(fmt.Sprintf("severity = $%d", len(args)+1), severity)
		}
		if q := strings.TrimSpace(filter.Query); q != "" {
			like := "%" + strings.ToLower(q) + "%"
			startIdx := len(args) + 1
			addCondition(
				fmt.Sprintf("(LOWER(request_id) LIKE $%d OR LOWER(model) LIKE $%d OR LOWER(error_message) LIKE $%d OR LOWER(error_type) LIKE $%d)",
					startIdx, startIdx+1, startIdx+2, startIdx+3,
				),
				like, like, like, like,
			)
		}

		// 支持多值状态码过滤
		if len(filter.StatusCodes) > 0 {
			placeholders := make([]string, len(filter.StatusCodes))
			for i, code := range filter.StatusCodes {
				placeholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
				args = append(args, code)
			}
			conditions = append(conditions, fmt.Sprintf("status_code = ANY(ARRAY[%s])", strings.Join(placeholders, ", ")))
		}

		// 支持客户端 IP 过滤
		if clientIP := strings.TrimSpace(filter.ClientIP); clientIP != "" {
			addCondition(fmt.Sprintf("client_ip = $%d", len(args)+1), clientIP)
		}

		if filter.AccountID != nil {
			addCondition(fmt.Sprintf("account_id = $%d", len(args)+1), *filter.AccountID)
		}

		if filter.GroupID != nil {
			addCondition(fmt.Sprintf("group_id = $%d", len(args)+1), *filter.GroupID)
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM ops_error_logs %s`, where)
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		if err == sql.ErrNoRows {
			total = 0
		} else {
			return nil, 0, err
		}
	}

	// NOTE: Keep columns aligned with scanOpsErrorLog() in ops_repo_error_logs.go.
	listQuery := fmt.Sprintf(`
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
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := r.sql.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]*service.OpsErrorLog, 0)
	for rows.Next() {
		entry, err := scanOpsErrorLog(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// GetErrorLogByID retrieves a single error log by its ID with all details.
func (r *OpsRepository) GetErrorLogByID(ctx context.Context, id int64) (*service.OpsErrorLog, error) {
	query := `
		SELECT
			id,
			created_at,
			error_phase,
			error_type,
			severity,
			status_code,
			platform,
			model,
			duration_ms,
			request_id,
			error_message,
			user_id,
			api_key_id,
			account_id,
			group_id,
			client_ip,
			request_path,
			stream,
			provider_error_code,
			provider_error_type,
			is_retryable,
			is_user_actionable,
			retry_count,
			completion_status,
			upstream_status_code,
			upstream_error_message,
			upstream_error_detail,
			network_error_type,
			retry_after_seconds,
			auth_latency_ms,
			routing_latency_ms,
			upstream_latency_ms,
			response_latency_ms,
			time_to_first_token_ms,
			request_body,
			error_body,
			user_agent
		FROM ops_error_logs
		WHERE id = $1
	`

	var (
		errorLog                    service.OpsErrorLog
		userID, apiKeyID, accountID sql.NullInt64
		groupID                     sql.NullInt64
		clientIP, requestPath       sql.NullString
		stream                      sql.NullBool
		statusCode                  sql.NullInt64
		platform, model             sql.NullString
		durationMs                  sql.NullInt64
		requestID                   sql.NullString
		message                     sql.NullString
		providerErrorCode           sql.NullString
		providerErrorType           sql.NullString
		isRetryable                 sql.NullBool
		isUserActionable            sql.NullBool
		retryCount                  sql.NullInt64
		completionStatus            sql.NullString
		upstreamStatusCode          sql.NullInt64
		upstreamErrorMessage        sql.NullString
		upstreamErrorDetail         sql.NullString
		networkErrorType            sql.NullString
		retryAfterSeconds           sql.NullInt64
		authLatencyMs               sql.NullInt64
		routingLatencyMs            sql.NullInt64
		upstreamLatencyMs           sql.NullInt64
		responseLatencyMs           sql.NullInt64
		timeToFirstTokenMs          sql.NullInt64
		requestBody, errorBody      sql.NullString
		userAgent                   sql.NullString
	)

	err := r.sql.QueryRowContext(ctx, query, id).Scan(
		&errorLog.ID,
		&errorLog.CreatedAt,
		&errorLog.Phase,
		&errorLog.Type,
		&errorLog.Severity,
		&statusCode,
		&platform,
		&model,
		&durationMs,
		&requestID,
		&message,
		&userID,
		&apiKeyID,
		&accountID,
		&groupID,
		&clientIP,
		&requestPath,
		&stream,
		&providerErrorCode,
		&providerErrorType,
		&isRetryable,
		&isUserActionable,
		&retryCount,
		&completionStatus,
		&upstreamStatusCode,
		&upstreamErrorMessage,
		&upstreamErrorDetail,
		&networkErrorType,
		&retryAfterSeconds,
		&authLatencyMs,
		&routingLatencyMs,
		&upstreamLatencyMs,
		&responseLatencyMs,
		&timeToFirstTokenMs,
		&requestBody,
		&errorBody,
		&userAgent,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("error log not found: id=%d", id)
		}
		return nil, err
	}

	// Set nullable fields
	if statusCode.Valid {
		errorLog.StatusCode = int(statusCode.Int64)
	}
	if platform.Valid {
		errorLog.Platform = platform.String
	}
	if model.Valid {
		errorLog.Model = model.String
	}
	if durationMs.Valid {
		v := int(durationMs.Int64)
		errorLog.LatencyMs = &v
		errorLog.DurationMs = &v
	}
	if requestID.Valid {
		errorLog.RequestID = requestID.String
	}
	if message.Valid {
		errorLog.Message = message.String
	}
	if userID.Valid {
		v := userID.Int64
		errorLog.UserID = &v
	}
	if apiKeyID.Valid {
		v := apiKeyID.Int64
		errorLog.APIKeyID = &v
	}
	if accountID.Valid {
		v := accountID.Int64
		errorLog.AccountID = &v
	}
	if groupID.Valid {
		v := groupID.Int64
		errorLog.GroupID = &v
	}
	if clientIP.Valid {
		errorLog.ClientIP = clientIP.String
	}
	if requestPath.Valid {
		errorLog.RequestPath = requestPath.String
	}
	if stream.Valid {
		errorLog.Stream = stream.Bool
	}
	if providerErrorCode.Valid {
		errorLog.ProviderErrorCode = providerErrorCode.String
	}
	if providerErrorType.Valid {
		errorLog.ProviderErrorType = providerErrorType.String
	}
	if isRetryable.Valid {
		errorLog.IsRetryable = isRetryable.Bool
	}
	if isUserActionable.Valid {
		errorLog.IsUserActionable = isUserActionable.Bool
	}
	if retryCount.Valid {
		errorLog.RetryCount = int(retryCount.Int64)
	}
	if completionStatus.Valid {
		errorLog.CompletionStatus = completionStatus.String
	}
	if upstreamStatusCode.Valid {
		v := int(upstreamStatusCode.Int64)
		errorLog.UpstreamStatusCode = &v
	}
	if upstreamErrorMessage.Valid {
		errorLog.UpstreamErrorMessage = upstreamErrorMessage.String
	}
	if upstreamErrorDetail.Valid {
		detail := upstreamErrorDetail.String
		errorLog.UpstreamErrorDetail = &detail
	}
	if networkErrorType.Valid {
		errorLog.NetworkErrorType = networkErrorType.String
	}
	if retryAfterSeconds.Valid {
		v := int(retryAfterSeconds.Int64)
		errorLog.RetryAfterSeconds = &v
	}
	if authLatencyMs.Valid {
		v := int(authLatencyMs.Int64)
		errorLog.AuthLatencyMs = &v
	}
	if routingLatencyMs.Valid {
		v := int(routingLatencyMs.Int64)
		errorLog.RoutingLatencyMs = &v
	}
	if upstreamLatencyMs.Valid {
		v := int(upstreamLatencyMs.Int64)
		errorLog.UpstreamLatencyMs = &v
	}
	if responseLatencyMs.Valid {
		v := int(responseLatencyMs.Int64)
		errorLog.ResponseLatencyMs = &v
	}
	if timeToFirstTokenMs.Valid {
		v := int(timeToFirstTokenMs.Int64)
		errorLog.TimeToFirstTokenMs = &v
	}
	if requestBody.Valid {
		errorLog.RequestBody = requestBody.String
	}
	if errorBody.Valid {
		errorLog.ErrorBody = errorBody.String
	}
	if userAgent.Valid {
		errorLog.UserAgent = userAgent.String
	}

	return &errorLog, nil
}

func levelFromSeverity(severity string) string {
	sev := strings.ToUpper(strings.TrimSpace(severity))
	switch sev {
	case "P0", "P1":
		return "CRITICAL"
	case "P2":
		return "ERROR"
	case "P3":
		return "WARN"
	default:
		return "ERROR"
	}
}
