package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/lib/pq"
)

type AISkillRepository interface {
	CreateSkill(ctx context.Context, skill *domain.AISkill) error
	GetSkillByID(ctx context.Context, id int64) (*domain.AISkill, error)
	GetSkillByUserAndID(ctx context.Context, userID, id int64) (*domain.AISkill, error)
	ListSkills(ctx context.Context, viewerUserID int64, isAdmin bool, params pagination.PaginationParams, filter domain.AISkillListFilter) ([]domain.AISkill, *pagination.PaginationResult, error)
	SetSkillInstall(ctx context.Context, skillID, userID int64, installed bool) error
	HasSkillInstall(ctx context.Context, skillID, userID int64) (bool, error)
	GetSkillInstallStates(ctx context.Context, userID int64, skillIDs []int64) (map[int64]bool, error)
	GetSkillInstallCounts(ctx context.Context, skillIDs []int64) (map[int64]int, error)
	UpdateSkill(ctx context.Context, skill *domain.AISkill) error
	DeleteSkill(ctx context.Context, id int64) error

	CreateSkillVersion(ctx context.Context, version *domain.AISkillVersion) error
	GetSkillVersionByID(ctx context.Context, id int64) (*domain.AISkillVersion, error)
	GetSkillVersionBySkillAndVersion(ctx context.Context, skillID int64, version int) (*domain.AISkillVersion, error)
	ListSkillVersions(ctx context.Context, skillID int64) ([]domain.AISkillVersion, error)
	UpdateSkillVersion(ctx context.Context, version *domain.AISkillVersion) error
	DeleteSkillVersion(ctx context.Context, id int64) error

	SubmitSkillReview(ctx context.Context, input domain.AISkillReviewSubmitInput) (*domain.AISkillReview, error)
	ApplySkillReviewDecision(ctx context.Context, input domain.AISkillReviewDecisionInput) (*domain.AISkillReview, error)
	GetSkillReviewByID(ctx context.Context, id int64) (*domain.AISkillReview, error)

	CreateSkillRun(ctx context.Context, run *domain.AISkillRun) error
	UpdateSkillRun(ctx context.Context, run *domain.AISkillRun) error
	GetSkillRunByID(ctx context.Context, id int64) (*domain.AISkillRun, error)

	SetSkillLike(ctx context.Context, input domain.AISkillLikeInput) (bool, error)
	HasSkillLike(ctx context.Context, skillID, userID int64) (bool, error)

	CreateSkillSettlement(ctx context.Context, settlement *domain.AISkillSettlement) error
	UpdateSkillSettlement(ctx context.Context, settlement *domain.AISkillSettlement) error
	GetSkillSettlementByID(ctx context.Context, id int64) (*domain.AISkillSettlement, error)
}

type aiSkillQueryExecer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type aiSkillRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewAISkillRepository(client *dbent.Client, db *sql.DB) AISkillRepository {
	return &aiSkillRepository{client: client, db: db}
}

func (r *aiSkillRepository) CreateSkill(ctx context.Context, skill *domain.AISkill) error {
	if skill == nil {
		return nil
	}
	if err := aiSkillPrepareSkill(skill); err != nil {
		return err
	}
	q, err := r.execer(ctx)
	if err != nil {
		return err
	}
	tagsJSON, err := aiSkillJSONStringSliceBytes(skill.Tags)
	if err != nil {
		return err
	}
	metadataJSON, err := aiSkillJSONMapBytes(skill.Metadata)
	if err != nil {
		return err
	}
	rows, err := q.QueryContext(ctx, `
INSERT INTO ai_skills (
    user_id, skill_type, title, summary, description, category, tags,
    visibility, source_visibility, billing_mode, price,
    current_version_id, published_version_id, latest_approved_version_id,
    latest_version, like_count, run_count, total_income, cover_asset_id, metadata,
    request_id, usage_log_id, api_key_id, group_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11,
    NULL, NULL, NULL,
    0, 0, 0, 0, $12, $13,
    $14, $15, $16, $17
)
RETURNING `+aiSkillSelectColumns,
		skill.UserID,
		skill.Type,
		skill.Title,
		aiSkillNillableString(skill.Summary),
		aiSkillNillableString(skill.Description),
		aiSkillNillableString(skill.Category),
		tagsJSON,
		skill.Visibility,
		skill.SourceVisibility,
		skill.BillingMode,
		skill.Price,
		aiSkillNillableInt64(skill.CoverAssetID),
		metadataJSON,
		aiSkillNillableString(skill.Trace.RequestID),
		aiSkillNillableInt64(skill.Trace.UsageLogID),
		aiSkillNillableInt64(skill.Trace.APIKeyID),
		aiSkillNillableInt64(skill.Trace.GroupID),
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return domain.ErrAISkillNotFound
	}
	saved, err := aiSkillScanSkill(rows)
	if err != nil {
		return err
	}
	*skill = *saved
	return rows.Err()
}

func (r *aiSkillRepository) GetSkillByID(ctx context.Context, id int64) (*domain.AISkill, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := aiSkillLoadSkill(ctx, q, "id = $1 AND deleted_at IS NULL", "", id)
	if err != nil {
		return nil, translatePersistenceError(err, domain.ErrAISkillNotFound, nil)
	}
	return item, nil
}

func (r *aiSkillRepository) GetSkillByUserAndID(ctx context.Context, userID, id int64) (*domain.AISkill, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := aiSkillLoadSkill(ctx, q, "id = $1 AND user_id = $2 AND deleted_at IS NULL", "", id, userID)
	if err != nil {
		return nil, translatePersistenceError(err, domain.ErrAISkillNotFound, nil)
	}
	return item, nil
}

func (r *aiSkillRepository) ListSkills(ctx context.Context, viewerUserID int64, isAdmin bool, params pagination.PaginationParams, filter domain.AISkillListFilter) ([]domain.AISkill, *pagination.PaginationResult, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, nil, err
	}

	where := make([]string, 0, 12)
	args := make([]any, 0, 12)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	where = append(where, "deleted_at IS NULL")
	switch domain.NormalizeAISkillScope(filter.Scope) {
	case domain.AISkillScopeLibrary:
		where = append(where, "published_version_id IS NOT NULL")
		where = append(where, "visibility = "+addArg(domain.AIVisibilityPublic))
	case domain.AISkillScopeAll:
		if !isAdmin {
			where = append(where, "user_id = "+addArg(viewerUserID))
		}
	default:
		ownerUserID := viewerUserID
		if filter.OwnerUserID != nil {
			ownerUserID = *filter.OwnerUserID
		}
		where = append(where, "user_id = "+addArg(ownerUserID))
	}
	if filter.PublishedOnly {
		where = append(where, "published_version_id IS NOT NULL")
	}
	if skillType := strings.TrimSpace(filter.SkillType); skillType != "" {
		normalized := domain.NormalizeAISkillType(skillType)
		if normalized == "" {
			return nil, nil, domain.ErrAISkillTypeInvalid
		}
		where = append(where, "skill_type = "+addArg(normalized))
	}
	if visibility := strings.TrimSpace(filter.Visibility); visibility != "" {
		value, ok := aiSkillExplicitVisibility(visibility)
		if !ok {
			return nil, nil, domain.ErrAISkillVisibilityInvalid
		}
		where = append(where, "visibility = "+addArg(value))
	}
	if category := strings.TrimSpace(filter.Category); category != "" {
		where = append(where, "category = "+addArg(category))
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		where = append(where, `COALESCE(NULLIF(LOWER(metadata->>'status'), ''), CASE WHEN visibility = 'public' THEN 'published' ELSE 'draft' END) = `+addArg(strings.ToLower(status)))
	}
	if billingMode := strings.TrimSpace(filter.BillingMode); billingMode != "" {
		normalized := domain.NormalizeAISkillBillingMode(billingMode)
		if normalized == "" {
			return nil, nil, domain.ErrAISkillBillingModeInvalid
		}
		where = append(where, "billing_mode = "+addArg(normalized))
	}
	if priceMode := strings.TrimSpace(filter.PriceMode); priceMode != "" {
		normalized := domain.NormalizeAISkillPriceMode(priceMode)
		if normalized == "" {
			return nil, nil, domain.ErrAISkillPriceModeInvalid
		}
		where = append(where, "("+aiSkillPriceModeExpression()+") = "+addArg(normalized))
	}
	if filter.Installed != nil && viewerUserID > 0 {
		viewerPlaceholder := addArg(viewerUserID)
		if *filter.Installed {
			where = append(where, "EXISTS (SELECT 1 FROM ai_skill_installs si WHERE si.skill_id = ai_skills.id AND si.user_id = "+viewerPlaceholder+")")
		} else {
			where = append(where, "NOT EXISTS (SELECT 1 FROM ai_skill_installs si WHERE si.skill_id = ai_skills.id AND si.user_id = "+viewerPlaceholder+")")
		}
	}
	if filter.GroupID != nil {
		where = append(where, "group_id = "+addArg(*filter.GroupID))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		needle := "%" + search + "%"
		p1 := addArg(needle)
		p2 := addArg(needle)
		p3 := addArg(needle)
		p4 := addArg(needle)
		p5 := addArg(needle)
		where = append(where, fmt.Sprintf("(title ILIKE %s OR summary ILIKE %s OR description ILIKE %s OR category ILIKE %s OR tags::text ILIKE %s)", p1, p2, p3, p4, p5))
	}

	whereSQL := strings.Join(where, " AND ")
	total, err := aiSkillCount(ctx, q, "SELECT COUNT(*) FROM ai_skills WHERE "+whereSQL, args...)
	if err != nil {
		return nil, nil, err
	}

	listArgs := append([]any(nil), args...)
	listArgs = append(listArgs, params.Limit(), params.Offset())
	rows, err := q.QueryContext(ctx, `
SELECT `+aiSkillSelectColumns+`
FROM ai_skills
WHERE `+whereSQL+`
ORDER BY `+aiSkillListOrder(params)+`
LIMIT $`+fmt.Sprint(len(listArgs)-1)+` OFFSET $`+fmt.Sprint(len(listArgs)), listArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]domain.AISkill, 0)
	for rows.Next() {
		item, scanErr := aiSkillScanSkill(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *aiSkillRepository) SetSkillInstall(ctx context.Context, skillID, userID int64, installed bool) error {
	q, err := r.execer(ctx)
	if err != nil {
		return err
	}
	if installed {
		_, err = q.ExecContext(ctx, `
INSERT INTO ai_skill_installs (skill_id, user_id, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (skill_id, user_id)
DO UPDATE SET updated_at = EXCLUDED.updated_at
`, skillID, userID)
		return err
	}
	_, err = q.ExecContext(ctx, `DELETE FROM ai_skill_installs WHERE skill_id = $1 AND user_id = $2`, skillID, userID)
	return err
}

func (r *aiSkillRepository) HasSkillInstall(ctx context.Context, skillID, userID int64) (bool, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return false, err
	}
	rows, err := q.QueryContext(ctx, `
SELECT EXISTS(
	SELECT 1 FROM ai_skill_installs
	WHERE skill_id = $1 AND user_id = $2
)`, skillID, userID)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	var exists bool
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return false, err
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *aiSkillRepository) GetSkillInstallStates(ctx context.Context, userID int64, skillIDs []int64) (map[int64]bool, error) {
	out := make(map[int64]bool, len(skillIDs))
	if userID <= 0 || len(skillIDs) == 0 {
		return out, nil
	}
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `
SELECT skill_id
FROM ai_skill_installs
WHERE user_id = $1 AND skill_id = ANY($2::bigint[])
`, userID, pq.Array(skillIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var skillID int64
		if err := rows.Scan(&skillID); err != nil {
			return nil, err
		}
		out[skillID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *aiSkillRepository) GetSkillInstallCounts(ctx context.Context, skillIDs []int64) (map[int64]int, error) {
	out := make(map[int64]int, len(skillIDs))
	if len(skillIDs) == 0 {
		return out, nil
	}
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `
SELECT skill_id, COUNT(*)::bigint
FROM ai_skill_installs
WHERE skill_id = ANY($1::bigint[])
GROUP BY skill_id
`, pq.Array(skillIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			skillID int64
			count   int64
		)
		if err := rows.Scan(&skillID, &count); err != nil {
			return nil, err
		}
		out[skillID] = int(count)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *aiSkillRepository) UpdateSkill(ctx context.Context, skill *domain.AISkill) error {
	if skill == nil {
		return nil
	}
	if err := aiSkillPrepareSkill(skill); err != nil {
		return err
	}
	return r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		current, err := aiSkillLoadSkill(txCtx, q, "id = $1 AND deleted_at IS NULL", "FOR UPDATE", skill.ID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillNotFound, nil)
		}

		currentVersionID := skill.CurrentVersionID
		publishedVersionID := skill.PublishedVersionID
		latestApprovedVersionID := skill.LatestApprovedVersionID

		if currentVersionID != nil {
			if _, err := aiSkillLoadVersion(txCtx, q, "id = $1 AND skill_id = $2 AND deleted_at IS NULL", "", *currentVersionID, skill.ID); err != nil {
				return translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
			}
		}
		if publishedVersionID != nil {
			version, err := aiSkillLoadVersion(txCtx, q, "id = $1 AND skill_id = $2 AND deleted_at IS NULL", "", *publishedVersionID, skill.ID)
			if err != nil {
				return translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
			}
			if err := aiSkillValidateVersionForPublish(version); err != nil {
				return err
			}
			if latestApprovedVersionID == nil {
				latestApprovedVersionID = publishedVersionID
			}
		}
		if latestApprovedVersionID != nil {
			version, err := aiSkillLoadVersion(txCtx, q, "id = $1 AND skill_id = $2 AND deleted_at IS NULL", "", *latestApprovedVersionID, skill.ID)
			if err != nil {
				return translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
			}
			if err := aiSkillValidateVersionForPublish(version); err != nil {
				return err
			}
		}

		tagsJSON, err := aiSkillJSONStringSliceBytes(skill.Tags)
		if err != nil {
			return err
		}
		metadataJSON, err := aiSkillJSONMapBytes(skill.Metadata)
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(txCtx, `
UPDATE ai_skills
SET skill_type = $2,
    title = $3,
    summary = $4,
    description = $5,
    category = $6,
    tags = $7,
    visibility = $8,
    source_visibility = $9,
    billing_mode = $10,
    price = $11,
    current_version_id = $12,
    published_version_id = $13,
    latest_approved_version_id = $14,
    cover_asset_id = $15,
    metadata = $16,
    request_id = $17,
    usage_log_id = $18,
    api_key_id = $19,
    group_id = $20,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING `+aiSkillSelectColumns,
			skill.ID,
			skill.Type,
			skill.Title,
			aiSkillNillableString(skill.Summary),
			aiSkillNillableString(skill.Description),
			aiSkillNillableString(skill.Category),
			tagsJSON,
			skill.Visibility,
			skill.SourceVisibility,
			skill.BillingMode,
			skill.Price,
			aiSkillNillableInt64(currentVersionID),
			aiSkillNillableInt64(publishedVersionID),
			aiSkillNillableInt64(latestApprovedVersionID),
			aiSkillNillableInt64(skill.CoverAssetID),
			metadataJSON,
			aiSkillNillableString(skill.Trace.RequestID),
			aiSkillNillableInt64(skill.Trace.UsageLogID),
			aiSkillNillableInt64(skill.Trace.APIKeyID),
			aiSkillNillableInt64(skill.Trace.GroupID),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillNotFound
		}
		saved, err := aiSkillScanSkill(rows)
		if err != nil {
			return err
		}
		saved.UserID = current.UserID
		*skill = *saved
		return rows.Err()
	})
}

func (r *aiSkillRepository) DeleteSkill(ctx context.Context, id int64) error {
	q, err := r.execer(ctx)
	if err != nil {
		return err
	}
	result, err := q.ExecContext(ctx, `
UPDATE ai_skills
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	affected, err := aiSkillRowsAffected(result)
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrAISkillNotFound
	}
	return nil
}

func (r *aiSkillRepository) CreateSkillVersion(ctx context.Context, version *domain.AISkillVersion) error {
	if version == nil {
		return nil
	}
	if err := aiSkillPrepareVersion(version); err != nil {
		return err
	}
	version.ReviewStatus = domain.AISkillVersionReviewStatusDraft
	version.SubmittedAt = nil
	version.ReviewedAt = nil
	version.ReviewerUserID = nil
	version.ReviewNote = ""

	return r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		skill, err := aiSkillLoadSkill(txCtx, q, "id = $1 AND deleted_at IS NULL", "FOR UPDATE", version.SkillID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillNotFound, nil)
		}
		if version.UserID == 0 {
			version.UserID = skill.UserID
		}
		configJSON, err := aiSkillJSONMapBytes(version.Config)
		if err != nil {
			return err
		}
		inputSchemaJSON, err := aiSkillJSONMapBytes(version.InputSchema)
		if err != nil {
			return err
		}
		outputSchemaJSON, err := aiSkillJSONMapBytes(version.OutputSchema)
		if err != nil {
			return err
		}
		metadataJSON, err := aiSkillJSONMapBytes(version.Metadata)
		if err != nil {
			return err
		}

		rows, err := q.QueryContext(txCtx, `
INSERT INTO ai_skill_versions (
    skill_id, user_id, version, review_status,
    content_format, runtime, source_content, config, input_schema, output_schema,
    change_note, submitted_at, reviewed_at, reviewer_user_id, review_note,
    metadata, request_id, usage_log_id, api_key_id, group_id
) VALUES (
    $1, $2, $3, 'draft',
    $4, $5, $6, $7, $8, $9,
    $10, NULL, NULL, NULL, NULL,
    $11, $12, $13, $14, $15
)
RETURNING `+aiSkillVersionSelectColumns,
			version.SkillID,
			version.UserID,
			version.Version,
			aiSkillNillableString(version.ContentFormat),
			aiSkillNillableString(version.Runtime),
			version.SourceContent,
			configJSON,
			inputSchemaJSON,
			outputSchemaJSON,
			aiSkillNillableString(version.ChangeNote),
			metadataJSON,
			aiSkillNillableString(version.Trace.RequestID),
			aiSkillNillableInt64(version.Trace.UsageLogID),
			aiSkillNillableInt64(version.Trace.APIKeyID),
			aiSkillNillableInt64(version.Trace.GroupID),
		)
		if err != nil {
			return translatePersistenceError(err, nil, domain.ErrAISkillVersionAlreadyExists)
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillVersionNotFound
		}
		saved, err := aiSkillScanVersion(rows)
		if err != nil {
			return err
		}
		*version = *saved
		if err := rows.Err(); err != nil {
			return err
		}

		_, err = q.ExecContext(txCtx, `
UPDATE ai_skills
SET current_version_id = CASE
        WHEN current_version_id IS NULL OR latest_version <= $2 THEN $1
        ELSE current_version_id
    END,
    latest_version = CASE
        WHEN latest_version < $2 THEN $2
        ELSE latest_version
    END,
    updated_at = NOW()
WHERE id = $3
  AND deleted_at IS NULL`, saved.ID, saved.Version, saved.SkillID)
		return err
	})
}

func (r *aiSkillRepository) GetSkillVersionByID(ctx context.Context, id int64) (*domain.AISkillVersion, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := aiSkillLoadVersion(ctx, q, "id = $1 AND deleted_at IS NULL", "", id)
	if err != nil {
		return nil, translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
	}
	return item, nil
}

func (r *aiSkillRepository) GetSkillVersionBySkillAndVersion(ctx context.Context, skillID int64, version int) (*domain.AISkillVersion, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := aiSkillLoadVersion(ctx, q, "skill_id = $1 AND version = $2 AND deleted_at IS NULL", "", skillID, version)
	if err != nil {
		return nil, translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
	}
	return item, nil
}

func (r *aiSkillRepository) ListSkillVersions(ctx context.Context, skillID int64) ([]domain.AISkillVersion, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `
SELECT `+aiSkillVersionSelectColumns+`
FROM ai_skill_versions
WHERE skill_id = $1
  AND deleted_at IS NULL
ORDER BY version DESC, id DESC`, skillID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]domain.AISkillVersion, 0)
	for rows.Next() {
		item, scanErr := aiSkillScanVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *aiSkillRepository) UpdateSkillVersion(ctx context.Context, version *domain.AISkillVersion) error {
	if version == nil {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		current, err := aiSkillLoadVersion(txCtx, q, "id = $1 AND deleted_at IS NULL", "FOR UPDATE", version.ID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
		}
		if !domain.CanSubmitAISkillVersionForReview(current.ReviewStatus) {
			return domain.ErrAISkillVersionTransitionInvalid
		}
		version.SkillID = current.SkillID
		version.UserID = current.UserID
		version.Version = current.Version
		version.ReviewStatus = current.ReviewStatus
		version.SubmittedAt = current.SubmittedAt
		version.ReviewedAt = current.ReviewedAt
		version.ReviewerUserID = current.ReviewerUserID
		if err := aiSkillPrepareVersion(version); err != nil {
			return err
		}

		configJSON, err := aiSkillJSONMapBytes(version.Config)
		if err != nil {
			return err
		}
		inputSchemaJSON, err := aiSkillJSONMapBytes(version.InputSchema)
		if err != nil {
			return err
		}
		outputSchemaJSON, err := aiSkillJSONMapBytes(version.OutputSchema)
		if err != nil {
			return err
		}
		metadataJSON, err := aiSkillJSONMapBytes(version.Metadata)
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(txCtx, `
UPDATE ai_skill_versions
SET content_format = $2,
    runtime = $3,
    source_content = $4,
    config = $5,
    input_schema = $6,
    output_schema = $7,
    change_note = $8,
    metadata = $9,
    request_id = $10,
    usage_log_id = $11,
    api_key_id = $12,
    group_id = $13,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING `+aiSkillVersionSelectColumns,
			version.ID,
			aiSkillNillableString(version.ContentFormat),
			aiSkillNillableString(version.Runtime),
			version.SourceContent,
			configJSON,
			inputSchemaJSON,
			outputSchemaJSON,
			aiSkillNillableString(version.ChangeNote),
			metadataJSON,
			aiSkillNillableString(version.Trace.RequestID),
			aiSkillNillableInt64(version.Trace.UsageLogID),
			aiSkillNillableInt64(version.Trace.APIKeyID),
			aiSkillNillableInt64(version.Trace.GroupID),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillVersionNotFound
		}
		saved, err := aiSkillScanVersion(rows)
		if err != nil {
			return err
		}
		*version = *saved
		return rows.Err()
	})
}

func (r *aiSkillRepository) DeleteSkillVersion(ctx context.Context, id int64) error {
	return r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		version, err := aiSkillLoadVersion(txCtx, q, "id = $1 AND deleted_at IS NULL", "FOR UPDATE", id)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
		}
		inUse, err := aiSkillVersionInUse(txCtx, q, version.SkillID, version.ID)
		if err != nil {
			return err
		}
		if inUse {
			return domain.ErrAISkillVersionInUse
		}
		result, err := q.ExecContext(txCtx, `
UPDATE ai_skill_versions
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL`, id)
		if err != nil {
			return err
		}
		affected, err := aiSkillRowsAffected(result)
		if err != nil {
			return err
		}
		if affected == 0 {
			return domain.ErrAISkillVersionNotFound
		}
		return nil
	})
}

func (r *aiSkillRepository) SubmitSkillReview(ctx context.Context, input domain.AISkillReviewSubmitInput) (*domain.AISkillReview, error) {
	if input.VersionID <= 0 {
		return nil, domain.ErrAISkillVersionNotFound
	}
	input.Metadata = aiSkillCloneMap(input.Metadata)

	var created *domain.AISkillReview
	err := r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		version, err := aiSkillLoadVersion(txCtx, q, "id = $1 AND deleted_at IS NULL", "FOR UPDATE", input.VersionID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
		}
		if input.SkillID > 0 && input.SkillID != version.SkillID {
			return domain.ErrAISkillVersionNotFound
		}
		if !domain.CanSubmitAISkillVersionForReview(version.ReviewStatus) {
			return domain.ErrAISkillVersionTransitionInvalid
		}
		pending, err := aiSkillPendingReviewExists(txCtx, q, version.ID)
		if err != nil {
			return err
		}
		if pending {
			return domain.ErrAISkillReviewAlreadyPending
		}

		submitterUserID := input.SubmitterUserID
		if submitterUserID == 0 {
			submitterUserID = version.UserID
		}
		snapshotJSON, err := aiSkillJSONMapBytes(aiSkillBuildReviewSnapshot(version))
		if err != nil {
			return err
		}
		metadataJSON, err := aiSkillJSONMapBytes(input.Metadata)
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(txCtx, `
INSERT INTO ai_skill_reviews (
    skill_id, version_id, submitter_user_id, reviewer_user_id, status,
    submit_note, review_note, snapshot, metadata, reviewed_at,
    request_id, usage_log_id, api_key_id, group_id
) VALUES (
    $1, $2, $3, NULL, 'pending',
    $4, NULL, $5, $6, NULL,
    $7, $8, $9, $10
)
RETURNING `+aiSkillReviewSelectColumns,
			version.SkillID,
			version.ID,
			submitterUserID,
			aiSkillNillableString(input.SubmitNote),
			snapshotJSON,
			metadataJSON,
			aiSkillNillableString(input.Trace.RequestID),
			aiSkillNillableInt64(input.Trace.UsageLogID),
			aiSkillNillableInt64(input.Trace.APIKeyID),
			aiSkillNillableInt64(input.Trace.GroupID),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillReviewNotFound
		}
		saved, err := aiSkillScanReview(rows)
		if err != nil {
			return err
		}
		if err := rows.Err(); err != nil {
			return err
		}
		created = saved
		_, err = q.ExecContext(txCtx, `
UPDATE ai_skill_versions
SET review_status = 'pending',
    submitted_at = NOW(),
    reviewed_at = NULL,
    reviewer_user_id = NULL,
    review_note = NULL,
    request_id = $2,
    usage_log_id = $3,
    api_key_id = $4,
    group_id = $5,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL`,
			version.ID,
			aiSkillNillableString(input.Trace.RequestID),
			aiSkillNillableInt64(input.Trace.UsageLogID),
			aiSkillNillableInt64(input.Trace.APIKeyID),
			aiSkillNillableInt64(input.Trace.GroupID),
		)
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *aiSkillRepository) ApplySkillReviewDecision(ctx context.Context, input domain.AISkillReviewDecisionInput) (*domain.AISkillReview, error) {
	if input.ReviewID <= 0 {
		return nil, domain.ErrAISkillReviewNotFound
	}
	input.Metadata = aiSkillCloneMap(input.Metadata)

	var updated *domain.AISkillReview
	err := r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		review, err := aiSkillLoadReview(txCtx, q, "id = $1", "FOR UPDATE", input.ReviewID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillReviewNotFound, nil)
		}
		if !domain.CanApplyAISkillReview(review.Status) {
			return domain.ErrAISkillReviewTransitionInvalid
		}
		version, err := aiSkillLoadVersion(txCtx, q, "id = $1 AND deleted_at IS NULL", "FOR UPDATE", review.VersionID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
		}
		nextReviewStatus, err := domain.NextAISkillReviewStatus(review.Status, input.Approved)
		if err != nil {
			return err
		}
		nextVersionStatus, err := domain.NextAISkillVersionReviewStatus(version.ReviewStatus, input.Approved)
		if err != nil {
			return err
		}

		metadataJSON, err := aiSkillJSONMapBytes(aiSkillMergeMaps(review.Metadata, input.Metadata))
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(txCtx, `
UPDATE ai_skill_reviews
SET reviewer_user_id = $2,
    status = $3,
    review_note = $4,
    metadata = $5,
    reviewed_at = NOW(),
    request_id = $6,
    usage_log_id = $7,
    api_key_id = $8,
    group_id = $9,
    updated_at = NOW()
WHERE id = $1
RETURNING `+aiSkillReviewSelectColumns,
			review.ID,
			input.ReviewerUserID,
			nextReviewStatus,
			aiSkillNillableString(input.ReviewNote),
			metadataJSON,
			aiSkillNillableString(input.Trace.RequestID),
			aiSkillNillableInt64(input.Trace.UsageLogID),
			aiSkillNillableInt64(input.Trace.APIKeyID),
			aiSkillNillableInt64(input.Trace.GroupID),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillReviewNotFound
		}
		savedReview, err := aiSkillScanReview(rows)
		if err != nil {
			return err
		}
		if err := rows.Err(); err != nil {
			return err
		}
		updated = savedReview

		_, err = q.ExecContext(txCtx, `
UPDATE ai_skill_versions
SET review_status = $2,
    reviewed_at = NOW(),
    reviewer_user_id = $3,
    review_note = $4,
    request_id = $5,
    usage_log_id = $6,
    api_key_id = $7,
    group_id = $8,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL`,
			version.ID,
			nextVersionStatus,
			input.ReviewerUserID,
			aiSkillNillableString(input.ReviewNote),
			aiSkillNillableString(input.Trace.RequestID),
			aiSkillNillableInt64(input.Trace.UsageLogID),
			aiSkillNillableInt64(input.Trace.APIKeyID),
			aiSkillNillableInt64(input.Trace.GroupID),
		)
		if err != nil {
			return err
		}
		if input.Approved {
			if input.PublishOnApprove {
				_, err = q.ExecContext(txCtx, `
UPDATE ai_skills
SET latest_approved_version_id = $1,
    published_version_id = $1,
    current_version_id = $1,
    updated_at = NOW()
WHERE id = $2
  AND deleted_at IS NULL`, version.ID, version.SkillID)
			} else {
				_, err = q.ExecContext(txCtx, `
UPDATE ai_skills
SET latest_approved_version_id = $1,
    updated_at = NOW()
WHERE id = $2
  AND deleted_at IS NULL`, version.ID, version.SkillID)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *aiSkillRepository) GetSkillReviewByID(ctx context.Context, id int64) (*domain.AISkillReview, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := aiSkillLoadReview(ctx, q, "id = $1", "", id)
	if err != nil {
		return nil, translatePersistenceError(err, domain.ErrAISkillReviewNotFound, nil)
	}
	return item, nil
}

func (r *aiSkillRepository) CreateSkillRun(ctx context.Context, run *domain.AISkillRun) error {
	if run == nil {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		version, err := aiSkillLoadVersion(txCtx, q, "id = $1 AND deleted_at IS NULL", "", run.VersionID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillVersionNotFound, nil)
		}
		if !domain.CanUseAISkillVersion(version.ReviewStatus) {
			return domain.ErrAISkillVersionNotApproved
		}
		skill, err := aiSkillLoadSkill(txCtx, q, "id = $1 AND deleted_at IS NULL", "", version.SkillID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillNotFound, nil)
		}
		run.SkillID = version.SkillID
		run.VersionID = version.ID
		if run.BillingMode == "" {
			run.BillingMode = skill.BillingMode
		}
		if run.Price == 0 && skill.Price > 0 {
			run.Price = skill.Price
		}
		if err := aiSkillPrepareRun(run); err != nil {
			return err
		}
		inputPayloadJSON, err := aiSkillJSONMapBytes(run.InputPayload)
		if err != nil {
			return err
		}
		outputPayloadJSON, err := aiSkillJSONMapBytes(run.OutputPayload)
		if err != nil {
			return err
		}
		metadataJSON, err := aiSkillJSONMapBytes(run.Metadata)
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(txCtx, `
INSERT INTO ai_skill_runs (
    skill_id, version_id, user_id, run_mode, status, billing_mode, price,
    input_payload, output_payload, error_message, metadata,
    request_id, usage_log_id, api_key_id, group_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11,
    $12, $13, $14, $15
)
RETURNING `+aiSkillRunSelectColumns,
			run.SkillID,
			run.VersionID,
			run.UserID,
			run.RunMode,
			run.Status,
			run.BillingMode,
			run.Price,
			inputPayloadJSON,
			outputPayloadJSON,
			aiSkillNillableString(run.ErrorMessage),
			metadataJSON,
			aiSkillNillableString(run.Trace.RequestID),
			aiSkillNillableInt64(run.Trace.UsageLogID),
			aiSkillNillableInt64(run.Trace.APIKeyID),
			aiSkillNillableInt64(run.Trace.GroupID),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillRunNotFound
		}
		saved, err := aiSkillScanRun(rows)
		if err != nil {
			return err
		}
		*run = *saved
		if err := rows.Err(); err != nil {
			return err
		}
		return aiSkillRefreshCounters(txCtx, q, run.SkillID)
	})
}

func (r *aiSkillRepository) UpdateSkillRun(ctx context.Context, run *domain.AISkillRun) error {
	if run == nil {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		current, err := aiSkillLoadRun(txCtx, q, "id = $1", "FOR UPDATE", run.ID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillRunNotFound, nil)
		}
		run.SkillID = current.SkillID
		run.VersionID = current.VersionID
		run.UserID = current.UserID
		if run.BillingMode == "" {
			run.BillingMode = current.BillingMode
		}
		if err := aiSkillPrepareRun(run); err != nil {
			return err
		}
		inputPayloadJSON, err := aiSkillJSONMapBytes(run.InputPayload)
		if err != nil {
			return err
		}
		outputPayloadJSON, err := aiSkillJSONMapBytes(run.OutputPayload)
		if err != nil {
			return err
		}
		metadataJSON, err := aiSkillJSONMapBytes(run.Metadata)
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(txCtx, `
UPDATE ai_skill_runs
SET run_mode = $2,
    status = $3,
    billing_mode = $4,
    price = $5,
    input_payload = $6,
    output_payload = $7,
    error_message = $8,
    metadata = $9,
    request_id = $10,
    usage_log_id = $11,
    api_key_id = $12,
    group_id = $13,
    updated_at = NOW()
WHERE id = $1
RETURNING `+aiSkillRunSelectColumns,
			run.ID,
			run.RunMode,
			run.Status,
			run.BillingMode,
			run.Price,
			inputPayloadJSON,
			outputPayloadJSON,
			aiSkillNillableString(run.ErrorMessage),
			metadataJSON,
			aiSkillNillableString(run.Trace.RequestID),
			aiSkillNillableInt64(run.Trace.UsageLogID),
			aiSkillNillableInt64(run.Trace.APIKeyID),
			aiSkillNillableInt64(run.Trace.GroupID),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillRunNotFound
		}
		saved, err := aiSkillScanRun(rows)
		if err != nil {
			return err
		}
		*run = *saved
		return rows.Err()
	})
}

func (r *aiSkillRepository) GetSkillRunByID(ctx context.Context, id int64) (*domain.AISkillRun, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := aiSkillLoadRun(ctx, q, "id = $1", "", id)
	if err != nil {
		return nil, translatePersistenceError(err, domain.ErrAISkillRunNotFound, nil)
	}
	return item, nil
}

func (r *aiSkillRepository) SetSkillLike(ctx context.Context, input domain.AISkillLikeInput) (bool, error) {
	if input.SkillID <= 0 {
		return false, domain.ErrAISkillNotFound
	}
	var changed bool
	err := r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		if _, err := aiSkillLoadSkill(txCtx, q, "id = $1 AND deleted_at IS NULL", "", input.SkillID); err != nil {
			return translatePersistenceError(err, domain.ErrAISkillNotFound, nil)
		}
		if input.Liked {
			result, err := q.ExecContext(txCtx, `
INSERT INTO ai_skill_likes (
    skill_id, user_id, request_id, usage_log_id, api_key_id, group_id, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (skill_id, user_id) DO NOTHING`,
				input.SkillID,
				input.UserID,
				aiSkillNillableString(input.Trace.RequestID),
				aiSkillNillableInt64(input.Trace.UsageLogID),
				aiSkillNillableInt64(input.Trace.APIKeyID),
				aiSkillNillableInt64(input.Trace.GroupID),
			)
			if err != nil {
				return err
			}
			affected, err := aiSkillRowsAffected(result)
			if err != nil {
				return err
			}
			changed = affected > 0
		} else {
			result, err := q.ExecContext(txCtx, `
DELETE FROM ai_skill_likes
WHERE skill_id = $1
  AND user_id = $2`, input.SkillID, input.UserID)
			if err != nil {
				return err
			}
			affected, err := aiSkillRowsAffected(result)
			if err != nil {
				return err
			}
			changed = affected > 0
		}
		if changed {
			return aiSkillRefreshCounters(txCtx, q, input.SkillID)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (r *aiSkillRepository) HasSkillLike(ctx context.Context, skillID, userID int64) (bool, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return false, err
	}
	return aiSkillLikeExists(ctx, q, skillID, userID)
}

func (r *aiSkillRepository) CreateSkillSettlement(ctx context.Context, settlement *domain.AISkillSettlement) error {
	if settlement == nil {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		run, err := aiSkillLoadRun(txCtx, q, "id = $1", "FOR UPDATE", settlement.RunID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillRunNotFound, nil)
		}
		skill, err := aiSkillLoadSkill(txCtx, q, "id = $1 AND deleted_at IS NULL", "", run.SkillID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillNotFound, nil)
		}
		settlement.SkillID = run.SkillID
		settlement.VersionID = run.VersionID
		settlement.OwnerUserID = skill.UserID
		if settlement.BillingMode == "" {
			settlement.BillingMode = run.BillingMode
		}
		if settlement.Amount == 0 {
			settlement.Amount = run.Price
		}
		if settlement.QuotaAmount == 0 {
			settlement.QuotaAmount = settlement.Amount
		}
		if err := aiSkillPrepareSettlement(settlement); err != nil {
			return err
		}
		metadataJSON, err := aiSkillJSONMapBytes(settlement.Metadata)
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(txCtx, `
INSERT INTO ai_skill_settlements (
    skill_id, version_id, run_id, owner_user_id, buyer_user_id,
    status, billing_mode, amount, quota_amount,
    quota_applied_at, balance_transferred_at, metadata,
    request_id, usage_log_id, api_key_id, group_id
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $12,
    $13, $14, $15, $16
)
RETURNING `+aiSkillSettlementSelectColumns,
			settlement.SkillID,
			settlement.VersionID,
			settlement.RunID,
			settlement.OwnerUserID,
			settlement.BuyerUserID,
			settlement.Status,
			settlement.BillingMode,
			settlement.Amount,
			settlement.QuotaAmount,
			aiSkillNillableTime(settlement.QuotaAppliedAt),
			aiSkillNillableTime(settlement.BalanceTransferredAt),
			metadataJSON,
			aiSkillNillableString(settlement.Trace.RequestID),
			aiSkillNillableInt64(settlement.Trace.UsageLogID),
			aiSkillNillableInt64(settlement.Trace.APIKeyID),
			aiSkillNillableInt64(settlement.Trace.GroupID),
		)
		if err != nil {
			return translatePersistenceError(err, nil, domain.ErrAISkillSettlementAlreadyExists)
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillSettlementNotFound
		}
		saved, err := aiSkillScanSettlement(rows)
		if err != nil {
			return err
		}
		*settlement = *saved
		if err := rows.Err(); err != nil {
			return err
		}
		return aiSkillRefreshCounters(txCtx, q, settlement.SkillID)
	})
}

func (r *aiSkillRepository) UpdateSkillSettlement(ctx context.Context, settlement *domain.AISkillSettlement) error {
	if settlement == nil {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, q aiSkillQueryExecer) error {
		current, err := aiSkillLoadSettlement(txCtx, q, "id = $1", "FOR UPDATE", settlement.ID)
		if err != nil {
			return translatePersistenceError(err, domain.ErrAISkillSettlementNotFound, nil)
		}
		settlement.SkillID = current.SkillID
		settlement.VersionID = current.VersionID
		settlement.RunID = current.RunID
		settlement.OwnerUserID = current.OwnerUserID
		settlement.BuyerUserID = current.BuyerUserID
		if settlement.BillingMode == "" {
			settlement.BillingMode = current.BillingMode
		}
		if settlement.QuotaAmount == 0 {
			settlement.QuotaAmount = settlement.Amount
		}
		if err := aiSkillPrepareSettlement(settlement); err != nil {
			return err
		}
		metadataJSON, err := aiSkillJSONMapBytes(settlement.Metadata)
		if err != nil {
			return err
		}
		rows, err := q.QueryContext(txCtx, `
UPDATE ai_skill_settlements
SET status = $2,
    billing_mode = $3,
    amount = $4,
    quota_amount = $5,
    quota_applied_at = $6,
    balance_transferred_at = $7,
    metadata = $8,
    request_id = $9,
    usage_log_id = $10,
    api_key_id = $11,
    group_id = $12,
    updated_at = NOW()
WHERE id = $1
RETURNING `+aiSkillSettlementSelectColumns,
			settlement.ID,
			settlement.Status,
			settlement.BillingMode,
			settlement.Amount,
			settlement.QuotaAmount,
			aiSkillNillableTime(settlement.QuotaAppliedAt),
			aiSkillNillableTime(settlement.BalanceTransferredAt),
			metadataJSON,
			aiSkillNillableString(settlement.Trace.RequestID),
			aiSkillNillableInt64(settlement.Trace.UsageLogID),
			aiSkillNillableInt64(settlement.Trace.APIKeyID),
			aiSkillNillableInt64(settlement.Trace.GroupID),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return domain.ErrAISkillSettlementNotFound
		}
		saved, err := aiSkillScanSettlement(rows)
		if err != nil {
			return err
		}
		*settlement = *saved
		if err := rows.Err(); err != nil {
			return err
		}
		return aiSkillRefreshCounters(txCtx, q, settlement.SkillID)
	})
}

func (r *aiSkillRepository) GetSkillSettlementByID(ctx context.Context, id int64) (*domain.AISkillSettlement, error) {
	q, err := r.execer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := aiSkillLoadSettlement(ctx, q, "id = $1", "", id)
	if err != nil {
		return nil, translatePersistenceError(err, domain.ErrAISkillSettlementNotFound, nil)
	}
	return item, nil
}

func (r *aiSkillRepository) execer(ctx context.Context) (aiSkillQueryExecer, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client(), nil
	}
	if r.db != nil {
		return r.db, nil
	}
	if r.client != nil {
		return r.client, nil
	}
	return nil, aiSkillConfiguredError()
}

func (r *aiSkillRepository) withTx(ctx context.Context, fn func(txCtx context.Context, q aiSkillQueryExecer) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	if r.client != nil {
		tx, err := r.client.Tx(ctx)
		if err != nil {
			return fmt.Errorf("begin ai skill transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		txCtx := dbent.NewTxContext(ctx, tx)
		if err := fn(txCtx, tx.Client()); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit ai skill transaction: %w", err)
		}
		return nil
	}
	if r.db == nil {
		return aiSkillConfiguredError()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ai skill transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ai skill transaction: %w", err)
	}
	return nil
}
