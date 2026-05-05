package repository

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

const (
	aiSkillSelectColumns = `
id,
user_id,
skill_type,
title,
summary,
description,
category,
tags,
visibility,
source_visibility,
billing_mode,
price::double precision,
current_version_id,
published_version_id,
latest_approved_version_id,
latest_version,
like_count,
run_count,
total_income::double precision,
cover_asset_id,
metadata,
request_id,
usage_log_id,
api_key_id,
group_id,
created_at,
updated_at,
deleted_at`
	aiSkillVersionSelectColumns = `
id,
skill_id,
user_id,
version,
review_status,
content_format,
runtime,
source_content,
config,
input_schema,
output_schema,
change_note,
submitted_at,
reviewed_at,
reviewer_user_id,
review_note,
metadata,
request_id,
usage_log_id,
api_key_id,
group_id,
created_at,
updated_at,
deleted_at`
	aiSkillReviewSelectColumns = `
id,
skill_id,
version_id,
submitter_user_id,
reviewer_user_id,
status,
submit_note,
review_note,
snapshot,
metadata,
reviewed_at,
request_id,
usage_log_id,
api_key_id,
group_id,
created_at,
updated_at`
	aiSkillRunSelectColumns = `
id,
skill_id,
version_id,
user_id,
run_mode,
status,
billing_mode,
price::double precision,
input_payload,
output_payload,
error_message,
metadata,
request_id,
usage_log_id,
api_key_id,
group_id,
created_at,
updated_at`
	aiSkillSettlementSelectColumns = `
id,
skill_id,
version_id,
run_id,
owner_user_id,
buyer_user_id,
status,
billing_mode,
amount::double precision,
quota_amount::double precision,
quota_applied_at,
balance_transferred_at,
metadata,
request_id,
usage_log_id,
api_key_id,
group_id,
created_at,
updated_at`
)

type aiSkillRowScanner interface {
	Scan(dest ...any) error
}

func aiSkillTrimmedString(raw string) string {
	return strings.TrimSpace(raw)
}

func aiSkillPriceModeExpression() string {
	return `
COALESCE(
    NULLIF(LOWER(BTRIM(metadata->'pricing'->>'mode')), ''),
    NULLIF(LOWER(BTRIM(metadata->>'price_mode')), ''),
    CASE
        WHEN COALESCE(
            NULLIF(BTRIM(metadata->'pricing'->>'amount'), '')::double precision,
            NULLIF(BTRIM(metadata->>'price_amount'), '')::double precision,
            price,
            0
        ) > 0 THEN 'paid'
        ELSE 'free'
    END
)`
}

func aiSkillNillableString(raw string) any {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	return value
}

func aiSkillNillableInt64(raw *int64) any {
	if raw == nil {
		return nil
	}
	return *raw
}

func aiSkillNillableTime(raw *time.Time) any {
	if raw == nil {
		return nil
	}
	return *raw
}

func aiSkillJSONMapBytes(src map[string]any) ([]byte, error) {
	if src == nil {
		src = map[string]any{}
	}
	return json.Marshal(src)
}

func aiSkillJSONStringSliceBytes(src []string) ([]byte, error) {
	if len(src) == 0 {
		src = []string{}
	}
	return json.Marshal(src)
}

func aiSkillDecodeStringSlice(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}

func aiSkillDecodeMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

func aiSkillScanTrace(requestID sql.NullString, usageLogID, apiKeyID, groupID sql.NullInt64) domain.AITraceRef {
	trace := domain.AITraceRef{}
	if requestID.Valid {
		trace.RequestID = strings.TrimSpace(requestID.String)
	}
	if usageLogID.Valid {
		trace.UsageLogID = &usageLogID.Int64
	}
	if apiKeyID.Valid {
		trace.APIKeyID = &apiKeyID.Int64
	}
	if groupID.Valid {
		trace.GroupID = &groupID.Int64
	}
	return trace
}

func aiSkillScanSkill(scanner aiSkillRowScanner) (*domain.AISkill, error) {
	var (
		item                    domain.AISkill
		summary                 sql.NullString
		description             sql.NullString
		category                sql.NullString
		tagsJSON                []byte
		currentVersionID        sql.NullInt64
		publishedVersionID      sql.NullInt64
		latestApprovedVersionID sql.NullInt64
		coverAssetID            sql.NullInt64
		metadataJSON            []byte
		requestID               sql.NullString
		usageLogID              sql.NullInt64
		apiKeyID                sql.NullInt64
		groupID                 sql.NullInt64
		deletedAt               sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.Type,
		&item.Title,
		&summary,
		&description,
		&category,
		&tagsJSON,
		&item.Visibility,
		&item.SourceVisibility,
		&item.BillingMode,
		&item.Price,
		&currentVersionID,
		&publishedVersionID,
		&latestApprovedVersionID,
		&item.LatestVersion,
		&item.LikeCount,
		&item.RunCount,
		&item.TotalIncome,
		&coverAssetID,
		&metadataJSON,
		&requestID,
		&usageLogID,
		&apiKeyID,
		&groupID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	item.Summary = strings.TrimSpace(summary.String)
	item.Description = strings.TrimSpace(description.String)
	item.Category = strings.TrimSpace(category.String)
	tags, err := aiSkillDecodeStringSlice(tagsJSON)
	if err != nil {
		return nil, err
	}
	item.Tags = tags
	metadata, err := aiSkillDecodeMap(metadataJSON)
	if err != nil {
		return nil, err
	}
	item.Metadata = metadata
	item.Trace = aiSkillScanTrace(requestID, usageLogID, apiKeyID, groupID)
	if currentVersionID.Valid {
		item.CurrentVersionID = &currentVersionID.Int64
	}
	if publishedVersionID.Valid {
		item.PublishedVersionID = &publishedVersionID.Int64
	}
	if latestApprovedVersionID.Valid {
		item.LatestApprovedVersionID = &latestApprovedVersionID.Int64
	}
	if coverAssetID.Valid {
		item.CoverAssetID = &coverAssetID.Int64
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}

func aiSkillScanVersion(scanner aiSkillRowScanner) (*domain.AISkillVersion, error) {
	var (
		item           domain.AISkillVersion
		contentFormat  sql.NullString
		runtime        sql.NullString
		changeNote     sql.NullString
		submittedAt    sql.NullTime
		reviewedAt     sql.NullTime
		reviewerUserID sql.NullInt64
		reviewNote     sql.NullString
		configJSON     []byte
		inputSchema    []byte
		outputSchema   []byte
		metadataJSON   []byte
		requestID      sql.NullString
		usageLogID     sql.NullInt64
		apiKeyID       sql.NullInt64
		groupID        sql.NullInt64
		deletedAt      sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.SkillID,
		&item.UserID,
		&item.Version,
		&item.ReviewStatus,
		&contentFormat,
		&runtime,
		&item.SourceContent,
		&configJSON,
		&inputSchema,
		&outputSchema,
		&changeNote,
		&submittedAt,
		&reviewedAt,
		&reviewerUserID,
		&reviewNote,
		&metadataJSON,
		&requestID,
		&usageLogID,
		&apiKeyID,
		&groupID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	item.ContentFormat = strings.TrimSpace(contentFormat.String)
	item.Runtime = strings.TrimSpace(runtime.String)
	item.ChangeNote = strings.TrimSpace(changeNote.String)
	item.ReviewNote = strings.TrimSpace(reviewNote.String)
	if submittedAt.Valid {
		item.SubmittedAt = &submittedAt.Time
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	if reviewerUserID.Valid {
		item.ReviewerUserID = &reviewerUserID.Int64
	}
	config, err := aiSkillDecodeMap(configJSON)
	if err != nil {
		return nil, err
	}
	item.Config = config
	inSchema, err := aiSkillDecodeMap(inputSchema)
	if err != nil {
		return nil, err
	}
	item.InputSchema = inSchema
	outSchema, err := aiSkillDecodeMap(outputSchema)
	if err != nil {
		return nil, err
	}
	item.OutputSchema = outSchema
	metadata, err := aiSkillDecodeMap(metadataJSON)
	if err != nil {
		return nil, err
	}
	item.Metadata = metadata
	item.Trace = aiSkillScanTrace(requestID, usageLogID, apiKeyID, groupID)
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}

func aiSkillScanReview(scanner aiSkillRowScanner) (*domain.AISkillReview, error) {
	var (
		item           domain.AISkillReview
		reviewerUserID sql.NullInt64
		submitNote     sql.NullString
		reviewNote     sql.NullString
		snapshotJSON   []byte
		metadataJSON   []byte
		reviewedAt     sql.NullTime
		requestID      sql.NullString
		usageLogID     sql.NullInt64
		apiKeyID       sql.NullInt64
		groupID        sql.NullInt64
	)
	if err := scanner.Scan(
		&item.ID,
		&item.SkillID,
		&item.VersionID,
		&item.SubmitterUserID,
		&reviewerUserID,
		&item.Status,
		&submitNote,
		&reviewNote,
		&snapshotJSON,
		&metadataJSON,
		&reviewedAt,
		&requestID,
		&usageLogID,
		&apiKeyID,
		&groupID,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if reviewerUserID.Valid {
		item.ReviewerUserID = &reviewerUserID.Int64
	}
	item.SubmitNote = strings.TrimSpace(submitNote.String)
	item.ReviewNote = strings.TrimSpace(reviewNote.String)
	snapshot, err := aiSkillDecodeMap(snapshotJSON)
	if err != nil {
		return nil, err
	}
	item.Snapshot = snapshot
	metadata, err := aiSkillDecodeMap(metadataJSON)
	if err != nil {
		return nil, err
	}
	item.Metadata = metadata
	item.Trace = aiSkillScanTrace(requestID, usageLogID, apiKeyID, groupID)
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	return &item, nil
}

func aiSkillScanRun(scanner aiSkillRowScanner) (*domain.AISkillRun, error) {
	var (
		item         domain.AISkillRun
		inputJSON    []byte
		outputJSON   []byte
		errorMessage sql.NullString
		metadataJSON []byte
		requestID    sql.NullString
		usageLogID   sql.NullInt64
		apiKeyID     sql.NullInt64
		groupID      sql.NullInt64
	)
	if err := scanner.Scan(
		&item.ID,
		&item.SkillID,
		&item.VersionID,
		&item.UserID,
		&item.RunMode,
		&item.Status,
		&item.BillingMode,
		&item.Price,
		&inputJSON,
		&outputJSON,
		&errorMessage,
		&metadataJSON,
		&requestID,
		&usageLogID,
		&apiKeyID,
		&groupID,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	inputPayload, err := aiSkillDecodeMap(inputJSON)
	if err != nil {
		return nil, err
	}
	item.InputPayload = inputPayload
	outputPayload, err := aiSkillDecodeMap(outputJSON)
	if err != nil {
		return nil, err
	}
	item.OutputPayload = outputPayload
	item.ErrorMessage = strings.TrimSpace(errorMessage.String)
	metadata, err := aiSkillDecodeMap(metadataJSON)
	if err != nil {
		return nil, err
	}
	item.Metadata = metadata
	item.Trace = aiSkillScanTrace(requestID, usageLogID, apiKeyID, groupID)
	return &item, nil
}

func aiSkillScanSettlement(scanner aiSkillRowScanner) (*domain.AISkillSettlement, error) {
	var (
		item                 domain.AISkillSettlement
		quotaAppliedAt       sql.NullTime
		balanceTransferredAt sql.NullTime
		metadataJSON         []byte
		requestID            sql.NullString
		usageLogID           sql.NullInt64
		apiKeyID             sql.NullInt64
		groupID              sql.NullInt64
	)
	if err := scanner.Scan(
		&item.ID,
		&item.SkillID,
		&item.VersionID,
		&item.RunID,
		&item.OwnerUserID,
		&item.BuyerUserID,
		&item.Status,
		&item.BillingMode,
		&item.Amount,
		&item.QuotaAmount,
		&quotaAppliedAt,
		&balanceTransferredAt,
		&metadataJSON,
		&requestID,
		&usageLogID,
		&apiKeyID,
		&groupID,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if quotaAppliedAt.Valid {
		item.QuotaAppliedAt = &quotaAppliedAt.Time
	}
	if balanceTransferredAt.Valid {
		item.BalanceTransferredAt = &balanceTransferredAt.Time
	}
	metadata, err := aiSkillDecodeMap(metadataJSON)
	if err != nil {
		return nil, err
	}
	item.Metadata = metadata
	item.Trace = aiSkillScanTrace(requestID, usageLogID, apiKeyID, groupID)
	return &item, nil
}

func aiSkillLoadSkill(ctx context.Context, q aiSkillQueryExecer, where, suffix string, args ...any) (*domain.AISkill, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+aiSkillSelectColumns+" FROM ai_skills WHERE "+where+" "+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	item, err := aiSkillScanSkill(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return item, nil
}

func aiSkillLoadVersion(ctx context.Context, q aiSkillQueryExecer, where, suffix string, args ...any) (*domain.AISkillVersion, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+aiSkillVersionSelectColumns+" FROM ai_skill_versions WHERE "+where+" "+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	item, err := aiSkillScanVersion(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return item, nil
}

func aiSkillLoadReview(ctx context.Context, q aiSkillQueryExecer, where, suffix string, args ...any) (*domain.AISkillReview, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+aiSkillReviewSelectColumns+" FROM ai_skill_reviews WHERE "+where+" "+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	item, err := aiSkillScanReview(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return item, nil
}

func aiSkillLoadRun(ctx context.Context, q aiSkillQueryExecer, where, suffix string, args ...any) (*domain.AISkillRun, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+aiSkillRunSelectColumns+" FROM ai_skill_runs WHERE "+where+" "+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	item, err := aiSkillScanRun(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return item, nil
}

func aiSkillLoadSettlement(ctx context.Context, q aiSkillQueryExecer, where, suffix string, args ...any) (*domain.AISkillSettlement, error) {
	rows, err := q.QueryContext(ctx, "SELECT "+aiSkillSettlementSelectColumns+" FROM ai_skill_settlements WHERE "+where+" "+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	item, err := aiSkillScanSettlement(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return item, nil
}

func aiSkillRefreshCounters(ctx context.Context, q aiSkillQueryExecer, skillID int64) error {
	_, err := q.ExecContext(ctx, `
UPDATE ai_skills
SET like_count = COALESCE((SELECT COUNT(*) FROM ai_skill_likes WHERE skill_id = $1), 0),
    run_count = COALESCE((SELECT COUNT(*) FROM ai_skill_runs WHERE skill_id = $1), 0),
    total_income = COALESCE((
        SELECT SUM(amount)
        FROM ai_skill_settlements
        WHERE skill_id = $1
          AND status IN ('quota_accrued', 'transferred')
    ), 0),
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL`, skillID)
	return err
}

func aiSkillVersionInUse(ctx context.Context, q aiSkillQueryExecer, skillID, versionID int64) (bool, error) {
	rows, err := q.QueryContext(ctx, `
SELECT EXISTS(
    SELECT 1
    FROM ai_skills
    WHERE id = $1
      AND deleted_at IS NULL
      AND (
          current_version_id = $2
          OR published_version_id = $2
          OR latest_approved_version_id = $2
      )
) OR EXISTS(
    SELECT 1 FROM ai_skill_runs WHERE version_id = $2
) OR EXISTS(
    SELECT 1 FROM ai_skill_reviews WHERE version_id = $2
) OR EXISTS(
    SELECT 1 FROM ai_skill_settlements WHERE version_id = $2
)`, skillID, versionID)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var inUse bool
	if err := rows.Scan(&inUse); err != nil {
		return false, err
	}
	return inUse, rows.Err()
}

func aiSkillPendingReviewExists(ctx context.Context, q aiSkillQueryExecer, versionID int64) (bool, error) {
	rows, err := q.QueryContext(ctx, `
SELECT EXISTS(
    SELECT 1
    FROM ai_skill_reviews
    WHERE version_id = $1
      AND status = 'pending'
)`, versionID)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var exists bool
	if err := rows.Scan(&exists); err != nil {
		return false, err
	}
	return exists, rows.Err()
}

func aiSkillLikeExists(ctx context.Context, q aiSkillQueryExecer, skillID, userID int64) (bool, error) {
	rows, err := q.QueryContext(ctx, `
SELECT EXISTS(
    SELECT 1
    FROM ai_skill_likes l
    JOIN ai_skills s ON s.id = l.skill_id
    WHERE l.skill_id = $1
      AND l.user_id = $2
      AND s.deleted_at IS NULL
)`, skillID, userID)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var exists bool
	if err := rows.Scan(&exists); err != nil {
		return false, err
	}
	return exists, rows.Err()
}

func aiSkillBuildReviewSnapshot(version *domain.AISkillVersion) map[string]any {
	if version == nil {
		return map[string]any{}
	}
	return map[string]any{
		"version":        version.Version,
		"review_status":  version.ReviewStatus,
		"content_format": version.ContentFormat,
		"runtime":        version.Runtime,
		"config":         aiSkillCloneMap(version.Config),
		"input_schema":   aiSkillCloneMap(version.InputSchema),
		"output_schema":  aiSkillCloneMap(version.OutputSchema),
		"metadata":       aiSkillCloneMap(version.Metadata),
	}
}

func aiSkillCloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func aiSkillCloneTags(src []string) []string {
	if len(src) == 0 {
		return []string{}
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func aiSkillMergeMaps(base, patch map[string]any) map[string]any {
	if len(base) == 0 && len(patch) == 0 {
		return map[string]any{}
	}
	out := aiSkillCloneMap(base)
	for k, v := range patch {
		out[k] = v
	}
	return out
}

func aiSkillListOrder(params pagination.PaginationParams) string {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	switch sortBy {
	case "created_at", "price", "like_count", "run_count":
	default:
		sortBy = "updated_at"
	}
	order := params.NormalizedSortOrder("desc")
	return sortBy + " " + order + ", id " + order
}

func aiSkillExplicitVisibility(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case domain.AIVisibilityPrivate:
		return domain.AIVisibilityPrivate, true
	case domain.AIVisibilityUnlisted:
		return domain.AIVisibilityUnlisted, true
	case domain.AIVisibilityPublic:
		return domain.AIVisibilityPublic, true
	default:
		return "", false
	}
}

func aiSkillApplySettlementTimestamps(settlement *domain.AISkillSettlement) {
	if settlement == nil {
		return
	}
	now := time.Now()
	switch settlement.Status {
	case domain.AISkillSettlementStatusQuotaAccrued:
		if settlement.QuotaAppliedAt == nil {
			settlement.QuotaAppliedAt = &now
		}
		settlement.BalanceTransferredAt = nil
	case domain.AISkillSettlementStatusTransferred:
		if settlement.QuotaAppliedAt == nil {
			settlement.QuotaAppliedAt = &now
		}
		if settlement.BalanceTransferredAt == nil {
			settlement.BalanceTransferredAt = &now
		}
	default:
		settlement.QuotaAppliedAt = nil
		settlement.BalanceTransferredAt = nil
	}
}

func aiSkillRowsAffected(result sql.Result) (int64, error) {
	if result == nil {
		return 0, nil
	}
	return result.RowsAffected()
}

func aiSkillPrepareSkill(skill *domain.AISkill) error {
	if skill == nil {
		return nil
	}
	skill.Type = domain.NormalizeAISkillType(skill.Type)
	if skill.Type == "" {
		return domain.ErrAISkillTypeInvalid
	}
	skill.Title = aiSkillTrimmedString(skill.Title)
	if skill.Title == "" {
		return domain.ErrAISkillTitleRequired
	}
	skill.Visibility = domain.NormalizeAIVisibility(skill.Visibility)
	skill.BillingMode = domain.NormalizeAISkillBillingMode(skill.BillingMode)
	if skill.BillingMode == "" {
		return domain.ErrAISkillBillingModeInvalid
	}
	if skill.Price < 0 {
		return domain.ErrAISkillPriceInvalid
	}
	skill.SourceVisibility = domain.NormalizeAISkillSourceVisibility(skill.SourceVisibility, skill.Visibility, skill.Price)
	if !domain.IsValidAISkillSourceVisibility(skill.SourceVisibility) {
		return domain.ErrAISkillSourceVisibilityInvalid
	}
	skill.Tags = aiSkillCloneTags(skill.Tags)
	skill.Metadata = aiSkillCloneMap(skill.Metadata)
	return nil
}

func aiSkillPrepareVersion(version *domain.AISkillVersion) error {
	if version == nil {
		return nil
	}
	if version.SkillID <= 0 {
		return domain.ErrAISkillNotFound
	}
	if version.Version <= 0 {
		return domain.ErrAISkillVersionNumberInvalid
	}
	version.ReviewStatus = domain.NormalizeAISkillVersionReviewStatus(version.ReviewStatus)
	if version.ReviewStatus == "" {
		return domain.ErrAISkillVersionStatusInvalid
	}
	version.ContentFormat = aiSkillTrimmedString(version.ContentFormat)
	version.Runtime = aiSkillTrimmedString(version.Runtime)
	version.ChangeNote = aiSkillTrimmedString(version.ChangeNote)
	version.ReviewNote = aiSkillTrimmedString(version.ReviewNote)
	version.Config = aiSkillCloneMap(version.Config)
	version.InputSchema = aiSkillCloneMap(version.InputSchema)
	version.OutputSchema = aiSkillCloneMap(version.OutputSchema)
	version.Metadata = aiSkillCloneMap(version.Metadata)
	return nil
}

func aiSkillPrepareRun(run *domain.AISkillRun) error {
	if run == nil {
		return nil
	}
	run.RunMode = domain.NormalizeAISkillRunMode(run.RunMode)
	if run.RunMode == "" {
		return domain.ErrAISkillRunModeInvalid
	}
	run.Status = domain.NormalizeAISkillRunStatus(run.Status)
	if run.Status == "" {
		return domain.ErrAISkillRunStatusInvalid
	}
	run.BillingMode = domain.NormalizeAISkillBillingMode(run.BillingMode)
	if run.BillingMode == "" {
		return domain.ErrAISkillBillingModeInvalid
	}
	if run.Price < 0 {
		return domain.ErrAISkillPriceInvalid
	}
	run.InputPayload = aiSkillCloneMap(run.InputPayload)
	run.OutputPayload = aiSkillCloneMap(run.OutputPayload)
	run.Metadata = aiSkillCloneMap(run.Metadata)
	run.ErrorMessage = aiSkillTrimmedString(run.ErrorMessage)
	return nil
}

func aiSkillPrepareSettlement(settlement *domain.AISkillSettlement) error {
	if settlement == nil {
		return nil
	}
	settlement.Status = domain.NormalizeAISkillSettlementStatus(settlement.Status)
	if settlement.Status == "" {
		return domain.ErrAISkillSettlementStatusInvalid
	}
	settlement.BillingMode = domain.NormalizeAISkillBillingMode(settlement.BillingMode)
	if settlement.BillingMode == "" {
		return domain.ErrAISkillBillingModeInvalid
	}
	if settlement.Amount < 0 || settlement.QuotaAmount < 0 {
		return domain.ErrAISkillPriceInvalid
	}
	settlement.Metadata = aiSkillCloneMap(settlement.Metadata)
	aiSkillApplySettlementTimestamps(settlement)
	return nil
}

func aiSkillCount(ctx context.Context, q aiSkillQueryExecer, query string, args ...any) (int64, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var total int64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func aiSkillValidateVersionForPublish(version *domain.AISkillVersion) error {
	if version == nil {
		return domain.ErrAISkillVersionNotFound
	}
	if !domain.CanPublishAISkillVersion(version.ReviewStatus) {
		return domain.ErrAISkillVersionNotApproved
	}
	return nil
}

func aiSkillConfiguredError() error {
	return fmt.Errorf("ai skill repository is not configured")
}
