package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *OpsRepository) ListRequestDetails(ctx context.Context, filter *service.OpsRequestDetailFilter) ([]*service.OpsRequestDetail, int64, error) {
	page, pageSize, startTime, endTime := filter.Normalize()
	offset := (page - 1) * pageSize

	conditions := make([]string, 0)
	args := make([]any, 0, 16)

	// Placeholders $1/$2 are reserved for time window inside the CTE.
	args = append(args, startTime, endTime)

	addCondition := func(condition string, values ...any) {
		conditions = append(conditions, condition)
		args = append(args, values...)
	}

	if filter != nil {
		if kind := strings.TrimSpace(strings.ToLower(filter.Kind)); kind != "" && kind != "all" {
			if kind != string(service.OpsRequestKindSuccess) && kind != string(service.OpsRequestKindError) {
				return nil, 0, fmt.Errorf("invalid kind")
			}
			addCondition(fmt.Sprintf("kind = $%d", len(args)+1), kind)
		}

		if len(filter.Platforms) > 0 {
			placeholders := make([]string, 0, len(filter.Platforms))
			for _, p := range filter.Platforms {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
					args = append(args, trimmed)
				}
			}
			if len(placeholders) > 0 {
				conditions = append(conditions, fmt.Sprintf("platform = ANY(ARRAY[%s])", strings.Join(placeholders, ", ")))
			}
		}

		if filter.UserID != nil {
			addCondition(fmt.Sprintf("user_id = $%d", len(args)+1), *filter.UserID)
		}
		if filter.APIKeyID != nil {
			addCondition(fmt.Sprintf("api_key_id = $%d", len(args)+1), *filter.APIKeyID)
		}
		if filter.AccountID != nil {
			addCondition(fmt.Sprintf("account_id = $%d", len(args)+1), *filter.AccountID)
		}
		if filter.GroupID != nil {
			addCondition(fmt.Sprintf("group_id = $%d", len(args)+1), *filter.GroupID)
		}

		if model := strings.TrimSpace(filter.Model); model != "" {
			addCondition(fmt.Sprintf("model = $%d", len(args)+1), model)
		}
		if requestID := strings.TrimSpace(filter.RequestID); requestID != "" {
			addCondition(fmt.Sprintf("request_id = $%d", len(args)+1), requestID)
		}
		if q := strings.TrimSpace(filter.Query); q != "" {
			like := "%" + strings.ToLower(q) + "%"
			startIdx := len(args) + 1
			addCondition(
				fmt.Sprintf("(LOWER(request_id) LIKE $%d OR LOWER(model) LIKE $%d OR LOWER(message) LIKE $%d)",
					startIdx, startIdx+1, startIdx+2,
				),
				like, like, like,
			)
		}

		if filter.MinDurationMs != nil {
			addCondition(fmt.Sprintf("duration_ms >= $%d", len(args)+1), *filter.MinDurationMs)
		}
		if filter.MaxDurationMs != nil {
			addCondition(fmt.Sprintf("duration_ms <= $%d", len(args)+1), *filter.MaxDurationMs)
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	cte := `
		WITH combined AS (
			SELECT
				'success'::TEXT AS kind,
				u.created_at AS created_at,
				u.request_id AS request_id,
				COALESCE(NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
				u.model AS model,
				u.duration_ms AS duration_ms,
				NULL::INT AS status_code,
				NULL::BIGINT AS error_id,
				NULL::TEXT AS phase,
				NULL::TEXT AS severity,
				NULL::TEXT AS message,
				u.user_id AS user_id,
				u.api_key_id AS api_key_id,
				u.account_id AS account_id,
				u.group_id AS group_id,
				u.stream AS stream
			FROM usage_logs u
			LEFT JOIN groups g ON g.id = u.group_id
			LEFT JOIN accounts a ON a.id = u.account_id
			WHERE u.created_at >= $1 AND u.created_at < $2

			UNION ALL

			SELECT
				'error'::TEXT AS kind,
				o.created_at AS created_at,
				o.request_id AS request_id,
				COALESCE(NULLIF(o.platform, ''), NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
				o.model AS model,
				o.duration_ms AS duration_ms,
				o.status_code AS status_code,
				o.id AS error_id,
				o.error_phase AS phase,
				o.severity AS severity,
				o.error_message AS message,
				o.user_id AS user_id,
				o.api_key_id AS api_key_id,
				o.account_id AS account_id,
				o.group_id AS group_id,
				o.stream AS stream
			FROM ops_error_logs o
			LEFT JOIN groups g ON g.id = o.group_id
			LEFT JOIN accounts a ON a.id = o.account_id
			WHERE o.created_at >= $1 AND o.created_at < $2
		)
	`

	countQuery := fmt.Sprintf(`%s SELECT COUNT(1) FROM combined %s`, cte, where)
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		if err == sql.ErrNoRows {
			total = 0
		} else {
			return nil, 0, err
		}
	}

	sort := "ORDER BY created_at DESC"
	if filter != nil {
		switch strings.TrimSpace(strings.ToLower(filter.Sort)) {
		case "", "created_at_desc":
			// default
		case "duration_desc":
			sort = "ORDER BY duration_ms DESC NULLS LAST, created_at DESC"
		default:
			return nil, 0, fmt.Errorf("invalid sort")
		}
	}

	listQuery := fmt.Sprintf(`
		%s
		SELECT
			kind,
			created_at,
			request_id,
			platform,
			model,
			duration_ms,
			status_code,
			error_id,
			phase,
			severity,
			message,
			user_id,
			api_key_id,
			account_id,
			group_id,
			stream
		FROM combined
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, cte, where, sort, len(args)+1, len(args)+2)

	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := r.sql.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	toIntPtr := func(v sql.NullInt64) *int {
		if !v.Valid {
			return nil
		}
		i := int(v.Int64)
		return &i
	}
	toInt64Ptr := func(v sql.NullInt64) *int64 {
		if !v.Valid {
			return nil
		}
		i := v.Int64
		return &i
	}

	out := make([]*service.OpsRequestDetail, 0, pageSize)
	for rows.Next() {
		var (
			kind      string
			createdAt sql.NullTime
			requestID sql.NullString
			platform  sql.NullString
			model     sql.NullString

			durationMs sql.NullInt64
			statusCode sql.NullInt64
			errorID    sql.NullInt64

			phase    sql.NullString
			severity sql.NullString
			message  sql.NullString

			userID    sql.NullInt64
			apiKeyID  sql.NullInt64
			accountID sql.NullInt64
			groupID   sql.NullInt64

			stream bool
		)

		if err := rows.Scan(
			&kind,
			&createdAt,
			&requestID,
			&platform,
			&model,
			&durationMs,
			&statusCode,
			&errorID,
			&phase,
			&severity,
			&message,
			&userID,
			&apiKeyID,
			&accountID,
			&groupID,
			&stream,
		); err != nil {
			return nil, 0, err
		}

		item := &service.OpsRequestDetail{
			Kind:      service.OpsRequestKind(kind),
			CreatedAt: createdAt.Time,
			RequestID: requestID.String,
			Platform:  platform.String,
			Model:     model.String,
			DurationMs: toIntPtr(durationMs),
			StatusCode: toIntPtr(statusCode),
			ErrorID:    toInt64Ptr(errorID),
			Phase:      phase.String,
			Severity:   severity.String,
			Message:    message.String,
			UserID:     toInt64Ptr(userID),
			APIKeyID:   toInt64Ptr(apiKeyID),
			AccountID:  toInt64Ptr(accountID),
			GroupID:    toInt64Ptr(groupID),
			Stream:     stream,
		}

		if item.Platform == "" {
			item.Platform = "unknown"
		}

		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}
