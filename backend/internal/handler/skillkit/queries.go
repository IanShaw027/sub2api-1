package skillkit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type Queries struct {
	db *sql.DB
}

func NewQueries(db *sql.DB) *Queries {
	return &Queries{db: db}
}

type RunRecord struct {
	ID            int64
	SkillID       int64
	SkillName     string
	VersionID     *int64
	VersionName   string
	Status        string
	Trigger       string
	InputPreview  string
	OutputPreview string
	ErrorMessage  string
	DurationMS    *float64
	Cost          float64
	Currency      string
	CreatedAt     time.Time
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

type RevenuePoint struct {
	Date    string
	Revenue float64
	Sales   int64
	Runs    int64
}

type RevenueOrder struct {
	ID        int64
	BuyerName string
	Version   string
	Amount    float64
	Currency  string
	Status    string
	CreatedAt time.Time
}

type RevenueSummary struct {
	TotalRevenue   float64
	TotalSales     int64
	TotalRuns      int64
	PendingAmount  float64
	SettledAmount  float64
	RefundedAmount float64
	Currency       string
}

type RevenueDetail struct {
	Summary RevenueSummary
	Trend   []RevenuePoint
	Orders  []RevenueOrder
}

type ReviewRecord struct {
	ID                     int64
	SkillID                int64
	SkillName              string
	SkillSlug              string
	VersionID              int64
	VersionName            string
	LatestPublishedVersion string
	ReviewStatus           string
	Visibility             string
	RiskLevel              string
	Category               string
	AuthorName             string
	Summary                string
	Changelog              string
	ReviewNote             string
	RejectionReason        string
	ReviewerName           string
	Tags                   []string
	Requests24H            int64
	Revenue30D             float64
	SubmittedAt            time.Time
	ReviewedAt             *time.Time
	UpdatedAt              time.Time
}

type GovernanceRecord struct {
	ID                     int64
	SkillID                int64
	SkillName              string
	SkillSlug              string
	CurrentVersion         string
	LatestPublishedVersion string
	GovernanceStatus       string
	LatestReviewStatus     string
	Visibility             string
	Category               string
	AuthorName             string
	ReviewNote             string
	Tags                   []string
	Requests24H            int64
	SuccessRate            float64
	Revenue30D             float64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type RuntimeRecord struct {
	ID             int64
	SkillID        int64
	SkillName      string
	SkillSlug      string
	CurrentVersion string
	HealthStatus   string
	Requests24H    int64
	SuccessRate    float64
	AvgLatencyMS   float64
	P95LatencyMS   float64
	ErrorRate      float64
	QueueDepth     int64
	LastError      string
	LastRunAt      *time.Time
	LastAlertAt    *time.Time
}

type RuntimeEvent struct {
	ID          int64
	SkillID     *int64
	SkillName   string
	Level       string
	Message     string
	MetricName  string
	MetricValue *float64
	CreatedAt   time.Time
}

type SettlementRecord struct {
	ID                int64
	SkillID           int64
	SkillName         string
	SkillSlug         string
	AuthorName        string
	PeriodLabel       string
	SettlementStatus  string
	ServiceStatus     string
	GrossAmount       float64
	PlatformFeeAmount float64
	PayoutAmount      float64
	FrozenAmount      float64
	Currency          string
	Note              string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (q *Queries) ListSkillRuns(ctx context.Context, userID, skillID int64, params pagination.PaginationParams, versionID *int64, status, search string) ([]RunRecord, *pagination.PaginationResult, error) {
	if q == nil || q.db == nil {
		return nil, nil, fmt.Errorf("skill run query db unavailable")
	}
	where := []string{"r.user_id = $1", "r.skill_id = $2"}
	args := []any{userID, skillID}
	if versionID != nil && *versionID > 0 {
		args = append(args, *versionID)
		where = append(where, fmt.Sprintf("r.version_id = $%d", len(args)))
	}
	if trimmed := strings.TrimSpace(status); trimmed != "" && trimmed != "all" {
		args = append(args, strings.ToLower(trimmed))
		where = append(where, fmt.Sprintf("LOWER(r.status) = $%d", len(args)))
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		args = append(args, "%"+trimmed+"%", "%"+trimmed+"%")
		where = append(where, fmt.Sprintf("(s.title ILIKE $%d OR COALESCE(v.metadata->>'version_name', 'v' || v.version::text) ILIKE $%d)", len(args)-1, len(args)))
	}
	whereSQL := strings.Join(where, " AND ")
	total, err := countRows(ctx, q.db, "SELECT COUNT(*) FROM ai_skill_runs r JOIN ai_skills s ON s.id = r.skill_id JOIN ai_skill_versions v ON v.id = r.version_id WHERE "+whereSQL, args...)
	if err != nil {
		return nil, nil, err
	}
	listArgs := append([]any(nil), args...)
	listArgs = append(listArgs, params.Limit(), params.Offset())
	rows, err := q.db.QueryContext(ctx, `
SELECT
  r.id,
  r.skill_id,
  s.title,
  r.version_id,
  COALESCE(v.metadata->>'version_name', 'v' || v.version::text) AS version_name,
  r.status,
  COALESCE(r.metadata->>'trigger', r.run_mode, '') AS trigger_name,
  COALESCE(r.input_payload::text, '') AS input_preview,
  COALESCE(r.output_payload::text, '') AS output_preview,
  COALESCE(r.error_message, '') AS error_message,
  NULL::double precision AS duration_ms,
  r.price::double precision AS cost,
  COALESCE(st.metadata->>'currency', s.metadata->>'currency', 'CNY') AS currency,
  r.created_at,
  NULL::timestamp,
  NULL::timestamp
FROM ai_skill_runs r
JOIN ai_skills s ON s.id = r.skill_id
JOIN ai_skill_versions v ON v.id = r.version_id
LEFT JOIN ai_skill_settlements st ON st.run_id = r.id
WHERE `+whereSQL+`
ORDER BY r.created_at DESC, r.id DESC
LIMIT $`+fmt.Sprint(len(listArgs)-1)+` OFFSET $`+fmt.Sprint(len(listArgs)), listArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	records := make([]RunRecord, 0)
	for rows.Next() {
		var (
			record         RunRecord
			versionIDValue sql.NullInt64
			durationMS     sql.NullFloat64
			startedAt      sql.NullTime
			finishedAt     sql.NullTime
		)
		if err := rows.Scan(
			&record.ID,
			&record.SkillID,
			&record.SkillName,
			&versionIDValue,
			&record.VersionName,
			&record.Status,
			&record.Trigger,
			&record.InputPreview,
			&record.OutputPreview,
			&record.ErrorMessage,
			&durationMS,
			&record.Cost,
			&record.Currency,
			&record.CreatedAt,
			&startedAt,
			&finishedAt,
		); err != nil {
			return nil, nil, err
		}
		record.VersionID = nullInt64Ptr(versionIDValue)
		if durationMS.Valid {
			value := durationMS.Float64
			record.DurationMS = &value
		}
		record.StartedAt = nullTimePtr(startedAt)
		record.FinishedAt = nullTimePtr(finishedAt)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return records, paginationResult(total, params), nil
}

func (q *Queries) GetSkillRevenue(ctx context.Context, ownerUserID, skillID int64) (*RevenueDetail, error) {
	if q == nil || q.db == nil {
		return nil, fmt.Errorf("skill revenue query db unavailable")
	}
	summary := RevenueSummary{Currency: "CNY"}
	row := q.db.QueryRowContext(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN st.status = 'transferred' THEN COALESCE((st.metadata->>'creator_amount')::double precision, st.quota_amount, 0) ELSE 0 END), 0)::double precision AS total_revenue,
  COUNT(st.id) AS total_sales,
  COALESCE(COUNT(r.id), 0) AS total_runs,
  COALESCE(SUM(CASE WHEN st.status = 'pending' THEN COALESCE((st.metadata->>'creator_amount')::double precision, st.quota_amount, 0) ELSE 0 END), 0)::double precision AS pending_amount,
  COALESCE(SUM(CASE WHEN st.status = 'transferred' THEN COALESCE((st.metadata->>'creator_amount')::double precision, st.quota_amount, 0) ELSE 0 END), 0)::double precision AS settled_amount,
  0::double precision AS refunded_amount,
  COALESCE(MAX(st.metadata->>'currency'), 'CNY') AS currency
FROM ai_skills s
LEFT JOIN ai_skill_runs r ON r.skill_id = s.id
LEFT JOIN ai_skill_settlements st ON st.run_id = r.id
WHERE s.id = $1
  AND s.user_id = $2`, skillID, ownerUserID)
	if err := row.Scan(&summary.TotalRevenue, &summary.TotalSales, &summary.TotalRuns, &summary.PendingAmount, &summary.SettledAmount, &summary.RefundedAmount, &summary.Currency); err != nil {
		return nil, err
	}

	trendRows, err := q.db.QueryContext(ctx, `
SELECT
  TO_CHAR(DATE_TRUNC('day', COALESCE(st.created_at, r.created_at)), 'YYYY-MM-DD') AS day_label,
  COALESCE(SUM(COALESCE((st.metadata->>'creator_amount')::double precision, st.quota_amount, 0)), 0)::double precision AS revenue,
  COUNT(DISTINCT st.id) AS sales,
  COUNT(DISTINCT r.id) AS runs
FROM ai_skill_runs r
LEFT JOIN ai_skill_settlements st ON st.run_id = r.id
WHERE r.skill_id = $1
  AND r.created_at >= NOW() - INTERVAL '30 days'
GROUP BY 1
ORDER BY 1 ASC`, skillID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = trendRows.Close() }()
	trend := make([]RevenuePoint, 0)
	for trendRows.Next() {
		var point RevenuePoint
		if err := trendRows.Scan(&point.Date, &point.Revenue, &point.Sales, &point.Runs); err != nil {
			return nil, err
		}
		trend = append(trend, point)
	}
	if err := trendRows.Err(); err != nil {
		return nil, err
	}

	orderRows, err := q.db.QueryContext(ctx, `
SELECT
  st.id,
  COALESCE(u.username, '') AS buyer_name,
  COALESCE(v.metadata->>'version_name', 'v' || v.version::text) AS version_name,
  COALESCE((st.metadata->>'creator_amount')::double precision, st.quota_amount, 0)::double precision AS amount,
  COALESCE(st.metadata->>'currency', 'CNY') AS currency,
  st.status,
  st.created_at
FROM ai_skill_settlements st
JOIN ai_skill_versions v ON v.id = st.version_id
LEFT JOIN users u ON u.id = st.buyer_user_id
WHERE st.skill_id = $1
ORDER BY st.created_at DESC, st.id DESC
LIMIT 20`, skillID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = orderRows.Close() }()
	orders := make([]RevenueOrder, 0)
	for orderRows.Next() {
		var item RevenueOrder
		if err := orderRows.Scan(&item.ID, &item.BuyerName, &item.Version, &item.Amount, &item.Currency, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, item)
	}
	if err := orderRows.Err(); err != nil {
		return nil, err
	}

	return &RevenueDetail{Summary: summary, Trend: trend, Orders: orders}, nil
}

func (q *Queries) ListReviews(ctx context.Context, params pagination.PaginationParams, reviewStatus, visibility, riskLevel, search string) ([]ReviewRecord, *pagination.PaginationResult, error) {
	if q == nil || q.db == nil {
		return nil, nil, fmt.Errorf("skill review query db unavailable")
	}
	where := []string{"1 = 1"}
	args := make([]any, 0, 8)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if trimmed := strings.TrimSpace(reviewStatus); trimmed != "" && trimmed != "all" {
		where = append(where, "r.status = "+addArg(trimmed))
	}
	if trimmed := strings.TrimSpace(visibility); trimmed != "" && trimmed != "all" {
		if trimmed == "force_private" {
			where = append(where, "(s.visibility = 'private' AND COALESCE(s.metadata->>'governance_status', '') = 'force_private')")
		} else {
			where = append(where, "s.visibility = "+addArg(trimmed))
		}
	}
	if trimmed := strings.TrimSpace(riskLevel); trimmed != "" && trimmed != "all" {
		switch trimmed {
		case "high":
			where = append(where, "s.skill_type = "+addArg(domain.AISkillTypeScript))
		case "medium":
			where = append(where, "s.skill_type = "+addArg(domain.AISkillTypePromptImage))
		case "low":
			where = append(where, "s.skill_type = "+addArg(domain.AISkillTypePromptChat))
		}
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		needle := "%" + trimmed + "%"
		where = append(where, "(s.title ILIKE "+addArg(needle)+" OR COALESCE(s.metadata->>'slug','') ILIKE "+addArg(needle)+" OR COALESCE(v.metadata->>'version_name','') ILIKE "+addArg(needle)+" OR COALESCE(author.username,'') ILIKE "+addArg(needle)+")")
	}
	whereSQL := strings.Join(where, " AND ")
	total, err := countRows(ctx, q.db, `
SELECT COUNT(*)
FROM ai_skill_reviews r
JOIN ai_skills s ON s.id = r.skill_id
JOIN ai_skill_versions v ON v.id = r.version_id
LEFT JOIN users author ON author.id = r.submitter_user_id
WHERE `+whereSQL, args...)
	if err != nil {
		return nil, nil, err
	}

	listArgs := append([]any(nil), args...)
	listArgs = append(listArgs, params.Limit(), params.Offset())
	rows, err := q.db.QueryContext(ctx, `
SELECT
  r.id,
  s.id,
  s.title,
  COALESCE(s.metadata->>'slug', 'skill-' || s.id::text) AS skill_slug,
  v.id,
  COALESCE(v.metadata->>'version_name', 'v' || v.version::text) AS version_name,
  COALESCE(pv.metadata->>'version_name', '') AS latest_published_version,
  r.status,
  s.visibility,
  s.skill_type,
  COALESCE(s.category, '') AS category,
  COALESCE(author.username, '') AS author_name,
  COALESCE(s.summary, '') AS summary,
  COALESCE(v.change_note, '') AS changelog,
  COALESCE(r.review_note, '') AS review_note,
  COALESCE(reviewer.username, '') AS reviewer_name,
  s.tags,
  s.run_count,
  s.total_income::double precision,
  COALESCE(v.submitted_at, r.created_at) AS submitted_at,
  r.reviewed_at,
  r.updated_at
FROM ai_skill_reviews r
JOIN ai_skills s ON s.id = r.skill_id
JOIN ai_skill_versions v ON v.id = r.version_id
LEFT JOIN ai_skill_versions pv ON pv.id = s.published_version_id
LEFT JOIN users author ON author.id = r.submitter_user_id
LEFT JOIN users reviewer ON reviewer.id = r.reviewer_user_id
WHERE `+whereSQL+`
ORDER BY r.created_at DESC, r.id DESC
LIMIT $`+fmt.Sprint(len(listArgs)-1)+` OFFSET $`+fmt.Sprint(len(listArgs)), listArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]ReviewRecord, 0)
	for rows.Next() {
		var (
			item       ReviewRecord
			skillType  string
			tagsJSON   []byte
			reviewedAt sql.NullTime
		)
		if err := rows.Scan(
			&item.ID,
			&item.SkillID,
			&item.SkillName,
			&item.SkillSlug,
			&item.VersionID,
			&item.VersionName,
			&item.LatestPublishedVersion,
			&item.ReviewStatus,
			&item.Visibility,
			&skillType,
			&item.Category,
			&item.AuthorName,
			&item.Summary,
			&item.Changelog,
			&item.ReviewNote,
			&item.ReviewerName,
			&tagsJSON,
			&item.Requests24H,
			&item.Revenue30D,
			&item.SubmittedAt,
			&reviewedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		item.ReviewedAt = nullTimePtr(reviewedAt)
		item.Tags = decodeStringArray(tagsJSON)
		item.RiskLevel = riskLevelForSkillType(skillType)
		if item.ReviewStatus == domain.AISkillReviewStatusRejected {
			item.RejectionReason = item.ReviewNote
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResult(total, params), nil
}

func (q *Queries) ListGovernance(ctx context.Context, params pagination.PaginationParams, governanceStatus, reviewStatus, visibility, search string) ([]GovernanceRecord, *pagination.PaginationResult, error) {
	if q == nil || q.db == nil {
		return nil, nil, fmt.Errorf("skill governance query db unavailable")
	}
	where := []string{"s.deleted_at IS NULL"}
	args := make([]any, 0, 8)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if trimmed := strings.TrimSpace(reviewStatus); trimmed != "" && trimmed != "all" {
		where = append(where, "COALESCE(last_review.status, 'pending') = "+addArg(trimmed))
	}
	if trimmed := strings.TrimSpace(visibility); trimmed != "" && trimmed != "all" {
		if trimmed == "force_private" {
			where = append(where, "(s.visibility = 'private' AND COALESCE(s.metadata->>'governance_status', '') = 'force_private')")
		} else {
			where = append(where, "s.visibility = "+addArg(trimmed))
		}
	}
	if trimmed := strings.TrimSpace(governanceStatus); trimmed != "" && trimmed != "all" {
		where = append(where, "COALESCE(s.metadata->>'governance_status', CASE WHEN s.visibility = 'public' THEN 'online' ELSE 'draft' END) = "+addArg(trimmed))
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		needle := "%" + trimmed + "%"
		where = append(where, "(s.title ILIKE "+addArg(needle)+" OR COALESCE(s.metadata->>'slug','') ILIKE "+addArg(needle)+")")
	}
	whereSQL := strings.Join(where, " AND ")
	total, err := countRows(ctx, q.db, `
SELECT COUNT(*)
FROM ai_skills s
LEFT JOIN LATERAL (
  SELECT r.status
  FROM ai_skill_reviews r
  WHERE r.skill_id = s.id
  ORDER BY r.created_at DESC, r.id DESC
  LIMIT 1
) last_review ON true
WHERE `+whereSQL, args...)
	if err != nil {
		return nil, nil, err
	}

	listArgs := append([]any(nil), args...)
	listArgs = append(listArgs, params.Limit(), params.Offset())
	rows, err := q.db.QueryContext(ctx, `
SELECT
  s.id,
  s.id,
  s.title,
  COALESCE(s.metadata->>'slug', 'skill-' || s.id::text) AS skill_slug,
  COALESCE(cv.metadata->>'version_name', '') AS current_version,
  COALESCE(pv.metadata->>'version_name', '') AS latest_published_version,
  COALESCE(s.metadata->>'governance_status', CASE WHEN s.visibility = 'public' THEN 'online' ELSE 'draft' END) AS governance_status,
  COALESCE(last_review.status, 'pending') AS latest_review_status,
  s.visibility,
  COALESCE(s.category, '') AS category,
  COALESCE(owner.username, '') AS author_name,
  COALESCE(last_review.review_note, '') AS review_note,
  s.tags,
  s.run_count,
  CASE WHEN stats.total_runs > 0 THEN stats.success_runs::double precision / stats.total_runs::double precision ELSE 0 END AS success_rate,
  s.total_income::double precision,
  s.created_at,
  s.updated_at
FROM ai_skills s
LEFT JOIN ai_skill_versions cv ON cv.id = s.current_version_id
LEFT JOIN ai_skill_versions pv ON pv.id = s.published_version_id
LEFT JOIN users owner ON owner.id = s.user_id
LEFT JOIN LATERAL (
  SELECT r.status, COALESCE(r.review_note, '') AS review_note
  FROM ai_skill_reviews r
  WHERE r.skill_id = s.id
  ORDER BY r.created_at DESC, r.id DESC
  LIMIT 1
) last_review ON true
LEFT JOIN LATERAL (
  SELECT
    COUNT(*) AS total_runs,
    COUNT(*) FILTER (WHERE status = 'succeeded') AS success_runs
  FROM ai_skill_runs
  WHERE skill_id = s.id
) stats ON true
WHERE `+whereSQL+`
ORDER BY s.updated_at DESC, s.id DESC
LIMIT $`+fmt.Sprint(len(listArgs)-1)+` OFFSET $`+fmt.Sprint(len(listArgs)), listArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]GovernanceRecord, 0)
	for rows.Next() {
		var (
			item     GovernanceRecord
			tagsJSON []byte
		)
		if err := rows.Scan(
			&item.ID,
			&item.SkillID,
			&item.SkillName,
			&item.SkillSlug,
			&item.CurrentVersion,
			&item.LatestPublishedVersion,
			&item.GovernanceStatus,
			&item.LatestReviewStatus,
			&item.Visibility,
			&item.Category,
			&item.AuthorName,
			&item.ReviewNote,
			&tagsJSON,
			&item.Requests24H,
			&item.SuccessRate,
			&item.Revenue30D,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		item.Tags = decodeStringArray(tagsJSON)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResult(total, params), nil
}

func (q *Queries) ListRuntime(ctx context.Context, params pagination.PaginationParams, healthStatus, search string) ([]RuntimeRecord, *pagination.PaginationResult, error) {
	if q == nil || q.db == nil {
		return nil, nil, fmt.Errorf("skill runtime query db unavailable")
	}
	where := []string{"s.deleted_at IS NULL"}
	args := make([]any, 0, 4)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if trimmed := strings.TrimSpace(healthStatus); trimmed != "" && trimmed != "all" {
		switch trimmed {
		case "critical":
			where = append(where, runtimeHealthCondition("stats", trimmed))
		case "warning":
			where = append(where, runtimeHealthCondition("stats", trimmed))
		case "healthy":
			where = append(where, runtimeHealthCondition("stats", trimmed))
		}
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		needle := "%" + trimmed + "%"
		where = append(where, "(s.title ILIKE "+addArg(needle)+" OR COALESCE(s.metadata->>'slug','') ILIKE "+addArg(needle)+")")
	}
	whereSQL := strings.Join(where, " AND ")
	total, err := countRows(ctx, q.db, `
SELECT COUNT(*)
FROM ai_skills s
LEFT JOIN LATERAL (
  SELECT
    CASE WHEN COUNT(*) > 0 THEN COUNT(*) FILTER (WHERE r.status = 'failed')::double precision / COUNT(*)::double precision ELSE 0 END AS error_rate,
    COUNT(*) FILTER (WHERE r.status IN ('queued', 'running')) AS queue_depth
  FROM ai_skill_runs r
  WHERE r.skill_id = s.id
) stats ON true
WHERE `+whereSQL, args...)
	if err != nil {
		return nil, nil, err
	}
	listArgs := append([]any(nil), args...)
	listArgs = append(listArgs, params.Limit(), params.Offset())
	rows, err := q.db.QueryContext(ctx, `
SELECT
  s.id,
  s.id,
  s.title,
  COALESCE(s.metadata->>'slug', 'skill-' || s.id::text) AS skill_slug,
  COALESCE(cv.metadata->>'version_name', '') AS current_version,
  stats.requests_24h,
  stats.success_rate,
  stats.avg_latency_ms,
  stats.p95_latency_ms,
  stats.error_rate,
  stats.queue_depth,
  stats.last_error,
  stats.last_run_at
FROM ai_skills s
LEFT JOIN ai_skill_versions cv ON cv.id = s.current_version_id
LEFT JOIN LATERAL (
  SELECT
    COUNT(*) FILTER (WHERE r.created_at >= NOW() - INTERVAL '24 hours') AS requests_24h,
    CASE WHEN COUNT(*) > 0 THEN COUNT(*) FILTER (WHERE r.status = 'succeeded')::double precision / COUNT(*)::double precision ELSE 0 END AS success_rate,
    0::double precision AS avg_latency_ms,
    0::double precision AS p95_latency_ms,
    CASE WHEN COUNT(*) > 0 THEN COUNT(*) FILTER (WHERE r.status = 'failed')::double precision / COUNT(*)::double precision ELSE 0 END AS error_rate,
    COUNT(*) FILTER (WHERE r.status IN ('queued', 'running')) AS queue_depth,
    MAX(CASE WHEN r.status = 'failed' THEN COALESCE(r.error_message, '') ELSE '' END) AS last_error,
    MAX(r.created_at) AS last_run_at
  FROM ai_skill_runs r
  WHERE r.skill_id = s.id
) stats ON true
WHERE `+whereSQL+`
ORDER BY s.updated_at DESC, s.id DESC
LIMIT $`+fmt.Sprint(len(listArgs)-1)+` OFFSET $`+fmt.Sprint(len(listArgs)), listArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]RuntimeRecord, 0)
	for rows.Next() {
		var (
			item      RuntimeRecord
			lastRunAt sql.NullTime
		)
		if err := rows.Scan(
			&item.ID,
			&item.SkillID,
			&item.SkillName,
			&item.SkillSlug,
			&item.CurrentVersion,
			&item.Requests24H,
			&item.SuccessRate,
			&item.AvgLatencyMS,
			&item.P95LatencyMS,
			&item.ErrorRate,
			&item.QueueDepth,
			&item.LastError,
			&lastRunAt,
		); err != nil {
			return nil, nil, err
		}
		item.LastRunAt = nullTimePtr(lastRunAt)
		item.HealthStatus = classifyRuntimeHealth(item)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResult(total, params), nil
}

func (q *Queries) ListRuntimeEvents(ctx context.Context, limit int) ([]RuntimeEvent, error) {
	if q == nil || q.db == nil {
		return nil, fmt.Errorf("skill runtime query db unavailable")
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := q.db.QueryContext(ctx, `
SELECT
  r.id,
  r.skill_id,
  s.title,
  CASE WHEN r.status = 'failed' THEN 'critical' ELSE 'info' END AS level,
  CASE WHEN r.status = 'failed' THEN COALESCE(r.error_message, 'skill run failed') ELSE 'skill run completed' END AS message,
  'run_status' AS metric_name,
  NULL::double precision AS metric_value,
  r.updated_at
FROM ai_skill_runs r
JOIN ai_skills s ON s.id = r.skill_id
ORDER BY r.updated_at DESC, r.id DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	events := make([]RuntimeEvent, 0)
	for rows.Next() {
		var (
			item      RuntimeEvent
			skillID   sql.NullInt64
			metricVal sql.NullFloat64
		)
		if err := rows.Scan(&item.ID, &skillID, &item.SkillName, &item.Level, &item.Message, &item.MetricName, &metricVal, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.SkillID = nullInt64Ptr(skillID)
		if metricVal.Valid {
			value := metricVal.Float64
			item.MetricValue = &value
		}
		events = append(events, item)
	}
	return events, rows.Err()
}

func (q *Queries) ListSettlements(ctx context.Context, params pagination.PaginationParams, settlementStatus, search string) ([]SettlementRecord, *pagination.PaginationResult, error) {
	if q == nil || q.db == nil {
		return nil, nil, fmt.Errorf("skill settlement query db unavailable")
	}
	where := []string{"1 = 1"}
	args := make([]any, 0, 4)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		needle := "%" + trimmed + "%"
		where = append(where, "(s.title ILIKE "+addArg(needle)+" OR COALESCE(owner.username,'') ILIKE "+addArg(needle)+")")
	}
	if trimmed := strings.TrimSpace(settlementStatus); trimmed != "" && trimmed != "all" {
		switch trimmed {
		case "settled":
			where = append(where, "st.status = 'transferred'")
		case "skipped":
			where = append(where, "COALESCE(st.metadata->>'service_status', '') = 'skipped'")
		case "pending", "ready":
			where = append(where, "st.status = 'pending'")
			where = append(where, "COALESCE(st.metadata->>'service_status', '') <> 'skipped'")
		case "frozen":
			where = append(where, "st.status = 'pending'")
			where = append(where, "COALESCE((st.metadata->>'creator_amount')::double precision, st.quota_amount, 0) > 0")
			where = append(where, "COALESCE(st.metadata->>'service_status', '') <> 'skipped'")
		case "rejected":
			where = append(where, "st.status = 'canceled'")
		}
	}
	whereSQL := strings.Join(where, " AND ")
	total, err := countRows(ctx, q.db, `
SELECT COUNT(*)
FROM ai_skill_settlements st
JOIN ai_skills s ON s.id = st.skill_id
LEFT JOIN users owner ON owner.id = s.user_id
WHERE `+whereSQL, args...)
	if err != nil {
		return nil, nil, err
	}
	listArgs := append([]any(nil), args...)
	listArgs = append(listArgs, params.Limit(), params.Offset())
	rows, err := q.db.QueryContext(ctx, `
SELECT
  st.id,
  st.skill_id,
  s.title,
  COALESCE(s.metadata->>'slug', 'skill-' || s.id::text) AS skill_slug,
  COALESCE(owner.username, '') AS author_name,
  TO_CHAR(DATE_TRUNC('month', st.created_at), 'YYYY-MM') AS period_label,
  st.status,
  st.amount::double precision AS gross_amount,
  COALESCE((st.metadata->>'platform_amount')::double precision, 0)::double precision AS platform_fee_amount,
  COALESCE((st.metadata->>'creator_amount')::double precision, st.quota_amount, 0)::double precision AS payout_amount,
	  CASE WHEN st.status = 'pending' THEN COALESCE((st.metadata->>'creator_amount')::double precision, st.quota_amount, 0) ELSE 0 END AS frozen_amount,
	  COALESCE(st.metadata->>'currency', 'CNY') AS currency,
	  COALESCE(st.metadata->>'failure_reason', '') AS note,
	  COALESCE(st.metadata->>'service_status', '') AS service_status,
	  st.created_at,
	  st.updated_at
	FROM ai_skill_settlements st
JOIN ai_skills s ON s.id = st.skill_id
LEFT JOIN users owner ON owner.id = s.user_id
WHERE `+whereSQL+`
ORDER BY st.updated_at DESC, st.id DESC
LIMIT $`+fmt.Sprint(len(listArgs)-1)+` OFFSET $`+fmt.Sprint(len(listArgs)), listArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]SettlementRecord, 0)
	for rows.Next() {
		var item SettlementRecord
		if err := rows.Scan(
			&item.ID,
			&item.SkillID,
			&item.SkillName,
			&item.SkillSlug,
			&item.AuthorName,
			&item.PeriodLabel,
			&item.SettlementStatus,
			&item.GrossAmount,
			&item.PlatformFeeAmount,
			&item.PayoutAmount,
			&item.FrozenAmount,
			&item.Currency,
			&item.Note,
			&item.ServiceStatus,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		item.SettlementStatus = mapSettlementStatus(item.SettlementStatus, item.ServiceStatus)
		items = append(items, item)
	}
	return items, paginationResult(total, params), rows.Err()
}

func countRows(ctx context.Context, db *sql.DB, query string, args ...any) (int64, error) {
	row := db.QueryRowContext(ctx, query, args...)
	var total int64
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func paginationResult(total int64, params pagination.PaginationParams) *pagination.PaginationResult {
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages <= 0 {
		pages = 1
	}
	return &pagination.PaginationResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}
}

func decodeStringArray(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}

func riskLevelForSkillType(skillType string) string {
	switch strings.TrimSpace(skillType) {
	case domain.AISkillTypeScript:
		return "high"
	case domain.AISkillTypePromptImage:
		return "medium"
	default:
		return "low"
	}
}

func classifyRuntimeHealth(item RuntimeRecord) string {
	if item.ErrorRate >= 0.5 {
		return "critical"
	}
	if item.ErrorRate >= 0.1 || item.QueueDepth > 0 {
		return "warning"
	}
	return "healthy"
}

func mapSettlementStatus(raw string, serviceStatus string) string {
	if strings.TrimSpace(serviceStatus) == "skipped" {
		return "skipped"
	}
	switch raw {
	case "transferred":
		return "settled"
	case "canceled":
		return "rejected"
	default:
		return "pending"
	}
}

func runtimeHealthCondition(alias, healthStatus string) string {
	switch strings.TrimSpace(healthStatus) {
	case "critical":
		return fmt.Sprintf("CASE WHEN %s.error_rate >= 0.5 THEN 'critical' WHEN %s.error_rate >= 0.1 OR %s.queue_depth > 0 THEN 'warning' ELSE 'healthy' END = 'critical'", alias, alias, alias)
	case "warning":
		return fmt.Sprintf("CASE WHEN %s.error_rate >= 0.5 THEN 'critical' WHEN %s.error_rate >= 0.1 OR %s.queue_depth > 0 THEN 'warning' ELSE 'healthy' END = 'warning'", alias, alias, alias)
	case "healthy":
		return fmt.Sprintf("CASE WHEN %s.error_rate >= 0.5 THEN 'critical' WHEN %s.error_rate >= 0.1 OR %s.queue_depth > 0 THEN 'warning' ELSE 'healthy' END = 'healthy'", alias, alias, alias)
	default:
		return "1 = 1"
	}
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	out := value.Time
	return &out
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}
