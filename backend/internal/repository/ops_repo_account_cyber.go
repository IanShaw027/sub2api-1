package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func accountCyberUsageCondition(paramIndex int) (string, any) {
	condition, args := buildRequestTypeFilterConditionWithAlias(paramIndex, int16(service.RequestTypeCyberBlocked), "u")
	return condition, args[0]
}

func (r *opsRepository) GetAccountCyberSummaries(ctx context.Context, accountIDs []int64) (map[int64]service.AccountCyberSummary, error) {
	out := make(map[int64]service.AccountCyberSummary, len(accountIDs))
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if len(accountIDs) == 0 {
		return out, nil
	}

	cyberCondition, cyberArg := accountCyberUsageCondition(2)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
WITH cyber_events AS (
  SELECT
    u.account_id,
    COALESCE(NULLIF(u.request_id, ''), 'usage:' || u.id::text) AS event_key,
    MAX(u.created_at) AS created_at
  FROM usage_logs u
  WHERE u.account_id = ANY($1)
    AND %s
  GROUP BY u.account_id, COALESCE(NULLIF(u.request_id, ''), 'usage:' || u.id::text)
)
SELECT account_id, COUNT(*), MAX(created_at)
FROM cyber_events
GROUP BY account_id`, cyberCondition), pq.Array(accountIDs), cyberArg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var accountID int64
		var summary service.AccountCyberSummary
		var latestAt sql.NullTime
		if err := rows.Scan(&accountID, &summary.Count, &latestAt); err != nil {
			return nil, err
		}
		if latestAt.Valid {
			value := latestAt.Time
			summary.LatestAt = &value
		}
		out[accountID] = summary
	}
	return out, rows.Err()
}

func (r *opsRepository) ListAccountCyberEvents(ctx context.Context, accountID int64, page, pageSize int) (*service.AccountCyberEventList, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if accountID <= 0 {
		return nil, fmt.Errorf("invalid account id")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	cyberCondition, cyberArg := accountCyberUsageCondition(2)
	countSQL := fmt.Sprintf(`
SELECT COUNT(*)
FROM (
  SELECT COALESCE(NULLIF(u.request_id, ''), 'usage:' || u.id::text)
  FROM usage_logs u
  WHERE u.account_id = $1 AND %s
  GROUP BY COALESCE(NULLIF(u.request_id, ''), 'usage:' || u.id::text)
) cyber_events`, cyberCondition)
	result := &service.AccountCyberEventList{
		Events:   make([]*service.AccountCyberEvent, 0, pageSize),
		Page:     page,
		PageSize: pageSize,
	}
	if err := r.db.QueryRowContext(ctx, countSQL, accountID, cyberArg).Scan(&result.Total); err != nil {
		return nil, err
	}
	if result.Total == 0 || (page-1)*pageSize >= result.Total {
		return result, nil
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
WITH ranked AS (
  SELECT
    u.id,
    u.created_at,
    COALESCE(u.request_id, '') AS request_id,
    COALESCE(NULLIF(u.requested_model, ''), NULLIF(u.model, ''), '') AS model,
    ROW_NUMBER() OVER (
      PARTITION BY COALESCE(NULLIF(u.request_id, ''), 'usage:' || u.id::text)
      ORDER BY u.created_at DESC, u.id DESC
    ) AS row_num
  FROM usage_logs u
  WHERE u.account_id = $1 AND %s
), deduplicated AS (
  SELECT id, created_at, request_id, model
  FROM ranked
  WHERE row_num = 1
), paged AS (
  SELECT id, created_at, request_id, model
  FROM deduplicated
  ORDER BY created_at DESC, id DESC
  LIMIT $3 OFFSET $4
), moderation AS (
  SELECT DISTINCT ON (m.request_id) m.request_id, m.error
  FROM content_moderation_logs m
  JOIN paged p ON p.request_id <> '' AND p.request_id = m.request_id
  WHERE m.action = 'cyber_policy'
  ORDER BY m.request_id, m.created_at DESC, m.id DESC
)
SELECT
  o.id,
  d.created_at,
  d.request_id,
  d.model,
  COALESCE(o.upstream_status_code, o.status_code),
  COALESCE(
    NULLIF(o.upstream_error_message, ''),
    NULLIF(o.upstream_error_detail, ''),
    NULLIF(o.error_message, ''),
    NULLIF(m.error, ''),
    'cyber_policy'
  )
FROM paged d
LEFT JOIN LATERAL (
  SELECT o.id, o.upstream_status_code, o.status_code,
         o.upstream_error_message, o.upstream_error_detail, o.error_message
  FROM ops_error_logs o
  WHERE d.request_id <> ''
    AND o.account_id = $1
    AND o.error_type = 'cyber_policy'
    AND o.request_id = d.request_id
  ORDER BY o.created_at DESC, o.id DESC
  LIMIT 1
) o ON true
LEFT JOIN moderation m ON m.request_id = d.request_id
ORDER BY d.created_at DESC, d.id DESC`, cyberCondition), accountID, cyberArg, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		item := &service.AccountCyberEvent{}
		var errorID sql.NullInt64
		var statusCode sql.NullInt64
		if err := rows.Scan(&errorID, &item.CreatedAt, &item.RequestID, &item.Model, &statusCode, &item.Message); err != nil {
			return nil, err
		}
		if errorID.Valid {
			value := errorID.Int64
			item.ErrorID = &value
		}
		if statusCode.Valid {
			value := int(statusCode.Int64)
			item.StatusCode = &value
		}
		result.Events = append(result.Events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
