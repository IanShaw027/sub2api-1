package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ListErrorLogs queries ops_error_logs with optional filters and pagination.
// It returns the list items and the total count of matching rows.
func (r *OpsRepository) ListErrorLogs(ctx context.Context, filter *service.ErrorLogFilter) ([]*service.ErrorLog, int64, error) {
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
	if pageSize > 100 {
		pageSize = 100
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
		if provider := strings.TrimSpace(filter.Provider); provider != "" {
			addCondition(fmt.Sprintf("platform = $%d", len(args)+1), provider)
		}
		if filter.AccountID != nil {
			addCondition(fmt.Sprintf("account_id = $%d", len(args)+1), *filter.AccountID)
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

	listQuery := fmt.Sprintf(`
		SELECT
			id,
			created_at,
			severity,
			request_id,
			account_id,
			request_path,
			platform,
			model,
			status_code,
			error_message,
			duration_ms,
			retry_count,
			stream
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

	results := make([]*service.ErrorLog, 0)
	for rows.Next() {
		var (
			id         int64
			createdAt  time.Time
			severity   sql.NullString
			requestID  sql.NullString
			accountID  sql.NullInt64
			requestURI sql.NullString
			platform   sql.NullString
			model      sql.NullString
			statusCode sql.NullInt64
			message    sql.NullString
			durationMs sql.NullInt64
			retryCount sql.NullInt64
			stream     sql.NullBool
		)

		if err := rows.Scan(
			&id,
			&createdAt,
			&severity,
			&requestID,
			&accountID,
			&requestURI,
			&platform,
			&model,
			&statusCode,
			&message,
			&durationMs,
			&retryCount,
			&stream,
		); err != nil {
			return nil, 0, err
		}

		entry := &service.ErrorLog{
			ID:        id,
			Timestamp: createdAt,
			Level:     levelFromSeverity(severity.String),
			RequestID: requestID.String,
			APIPath:   requestURI.String,
			Provider:  platform.String,
			Model:     model.String,
			HTTPCode:  int(statusCode.Int64),
			Stream:    stream.Bool,
		}
		if accountID.Valid {
			entry.AccountID = strconv.FormatInt(accountID.Int64, 10)
		}
		if message.Valid {
			entry.ErrorMessage = message.String
		}
		if durationMs.Valid {
			v := int(durationMs.Int64)
			entry.DurationMs = &v
		}
		if retryCount.Valid {
			v := int(retryCount.Int64)
			entry.RetryCount = &v
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
		errorLog                      service.OpsErrorLog
		userID, apiKeyID, accountID   sql.NullInt64
		groupID                       sql.NullInt64
		clientIP, requestPath         sql.NullString
		stream                        sql.NullBool
		durationMs                    sql.NullInt64
		authLatencyMs                 sql.NullInt64
		routingLatencyMs              sql.NullInt64
		upstreamLatencyMs             sql.NullInt64
		responseLatencyMs             sql.NullInt64
		timeToFirstTokenMs            sql.NullInt64
		requestBody, errorBody        sql.NullString
		userAgent                     sql.NullString
	)

	err := r.sql.QueryRowContext(ctx, query, id).Scan(
		&errorLog.ID,
		&errorLog.CreatedAt,
		&errorLog.Phase,
		&errorLog.Type,
		&errorLog.Severity,
		&errorLog.StatusCode,
		&errorLog.Platform,
		&errorLog.Model,
		&durationMs,
		&errorLog.RequestID,
		&errorLog.Message,
		&userID,
		&apiKeyID,
		&accountID,
		&groupID,
		&clientIP,
		&requestPath,
		&stream,
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
	if durationMs.Valid {
		v := int(durationMs.Int64)
		errorLog.LatencyMs = &v
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
