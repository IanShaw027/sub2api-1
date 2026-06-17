package skillkit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	metaKeySkillSlug             = "slug"
	metaKeySkillStatus           = "status"
	metaKeySkillTagline          = "tagline"
	metaKeySkillCoverImageURL    = "cover_image_url"
	metaKeySkillReadme           = "readme"
	metaKeySkillInstallNote      = "install_note"
	metaKeySkillSourceLocked     = "source_locked"
	metaKeySkillVariableSchema   = "variable_schema"
	metaKeySkillContent          = "content"
	metaKeySkillPricing          = "pricing"
	metaKeySkillVersionName      = "version_name"
	metaKeySkillBillingPolicy    = "billing_policy"
	metaKeySkillPresentation     = "presentation_status"
	metaKeySkillStatusOverride   = "service_status_override"
	metaValueSkillStatusDisabled = "disabled"
)

type serviceRepoAdapter struct {
	domainRepo      repository.AISkillRepository
	db              *sql.DB
	balanceCharger  service.AISkillBalanceCharger
	creatorCreditor service.AISkillCreatorEarningsCreditor
}

func (a *serviceRepoAdapter) CreateSkill(ctx context.Context, skill *service.AISkill) error {
	if a == nil || a.domainRepo == nil {
		return service.ErrAISkillServiceUnavailable
	}
	if skill == nil {
		return nil
	}
	entity := serviceSkillToDomain(skill)
	if err := a.domainRepo.CreateSkill(ctx, entity); err != nil {
		return err
	}
	*skill = *domainSkillToService(entity)
	return nil
}

func (a *serviceRepoAdapter) GetSkillByID(ctx context.Context, id int64) (*service.AISkill, error) {
	if a == nil || a.domainRepo == nil {
		return nil, service.ErrAISkillServiceUnavailable
	}
	entity, err := a.domainRepo.GetSkillByID(ctx, id)
	if err != nil {
		return nil, translateSkillError(err)
	}
	return domainSkillToService(entity), nil
}

func (a *serviceRepoAdapter) GetSkillByCreatorAndID(ctx context.Context, creatorUserID, id int64) (*service.AISkill, error) {
	if a == nil || a.domainRepo == nil {
		return nil, service.ErrAISkillServiceUnavailable
	}
	entity, err := a.domainRepo.GetSkillByUserAndID(ctx, creatorUserID, id)
	if err != nil {
		return nil, translateSkillError(err)
	}
	return domainSkillToService(entity), nil
}

func (a *serviceRepoAdapter) UpdateSkill(ctx context.Context, skill *service.AISkill) error {
	if a == nil || a.domainRepo == nil {
		return service.ErrAISkillServiceUnavailable
	}
	if skill == nil {
		return nil
	}
	entity := serviceSkillToDomain(skill)
	if err := a.domainRepo.UpdateSkill(ctx, entity); err != nil {
		return translateSkillError(err)
	}
	*skill = *domainSkillToService(entity)
	return nil
}

func (a *serviceRepoAdapter) CreateVersion(ctx context.Context, version *service.AISkillVersion) error {
	if a == nil || a.domainRepo == nil {
		return service.ErrAISkillServiceUnavailable
	}
	if version == nil {
		return nil
	}
	entity := serviceVersionToDomain(version)
	if err := a.domainRepo.CreateSkillVersion(ctx, entity); err != nil {
		return translateSkillError(err)
	}
	*version = *domainVersionToService(entity)
	return nil
}

func (a *serviceRepoAdapter) GetVersionByID(ctx context.Context, id int64) (*service.AISkillVersion, error) {
	entity, err := a.domainRepo.GetSkillVersionByID(ctx, id)
	if err != nil {
		return nil, translateSkillError(err)
	}
	return domainVersionToService(entity), nil
}

func (a *serviceRepoAdapter) GetVersionByCreatorAndID(ctx context.Context, creatorUserID, id int64) (*service.AISkillVersion, error) {
	entity, err := a.domainRepo.GetSkillVersionByID(ctx, id)
	if err != nil {
		return nil, translateSkillError(err)
	}
	if entity.UserID != creatorUserID {
		return nil, service.ErrAISkillVersionNotFound
	}
	return domainVersionToService(entity), nil
}

func (a *serviceRepoAdapter) GetLatestApprovedVersionBySkillID(ctx context.Context, skillID int64) (*service.AISkillVersion, error) {
	versions, err := a.domainRepo.ListSkillVersions(ctx, skillID)
	if err != nil {
		return nil, translateSkillError(err)
	}
	for i := range versions {
		item := versions[i]
		if item.ReviewStatus == domain.AISkillVersionReviewStatusApproved && readStatusOverride(item.Metadata) != metaValueSkillStatusDisabled {
			return domainVersionToService(&item), nil
		}
	}
	return nil, service.ErrAISkillVersionNotFound
}

func (a *serviceRepoAdapter) UpdateVersion(ctx context.Context, version *service.AISkillVersion) error {
	if a == nil || a.db == nil {
		return service.ErrAISkillServiceUnavailable
	}
	if version == nil {
		return nil
	}
	current, err := a.domainRepo.GetSkillVersionByID(ctx, version.ID)
	if err != nil {
		return translateSkillError(err)
	}
	entity := serviceVersionToDomain(version)
	entity.SkillID = current.SkillID
	entity.UserID = current.UserID
	entity.Version = current.Version
	entity.ReviewStatus = current.ReviewStatus
	entity.SubmittedAt = current.SubmittedAt
	entity.ReviewedAt = current.ReviewedAt
	entity.ReviewerUserID = current.ReviewerUserID
	entity.ReviewNote = current.ReviewNote

	override := ""
	switch version.Status {
	case service.AISkillVersionStatusDraft:
		if current.ReviewStatus == domain.AISkillVersionReviewStatusRejected {
			override = service.AISkillVersionStatusDraft
		}
	case service.AISkillVersionStatusDisabled:
		override = metaValueSkillStatusDisabled
	}
	applyStatusOverride(entity.Metadata, override)

	if shouldUseDomainVersionUpdate(version.Status, current.ReviewStatus) {
		if err := a.domainRepo.UpdateSkillVersion(ctx, entity); err != nil {
			return translateSkillError(err)
		}
	} else {
		if err := a.updateVersionSnapshot(ctx, entity); err != nil {
			return err
		}
	}
	*version = *domainVersionToService(entity)
	return nil
}

func (a *serviceRepoAdapter) CreateReview(ctx context.Context, review *service.AISkillReview) error {
	if a == nil || review == nil {
		return nil
	}
	switch review.Action {
	case service.AISkillReviewActionSubmitted:
		if a.domainRepo == nil {
			return nil
		}
		_, err := a.domainRepo.SubmitSkillReview(ctx, domain.AISkillReviewSubmitInput{
			SkillID:         review.SkillID,
			VersionID:       review.VersionID,
			SubmitterUserID: review.OperatorUserID,
			SubmitNote:      review.Comment,
			Metadata:        cloneMap(review.Metadata),
			Trace:           review.Trace,
		})
		return translateSkillError(err)
	case service.AISkillReviewActionApproved, service.AISkillReviewActionRejected:
		if a.domainRepo == nil {
			return nil
		}
		reviewID, err := a.lookupPendingReviewID(ctx, review.VersionID)
		if err != nil {
			return err
		}
		_, err = a.domainRepo.ApplySkillReviewDecision(ctx, domain.AISkillReviewDecisionInput{
			ReviewID:         reviewID,
			ReviewerUserID:   review.OperatorUserID,
			Approved:         review.Action == service.AISkillReviewActionApproved,
			ReviewNote:       review.Comment,
			PublishOnApprove: review.Action == service.AISkillReviewActionApproved,
			Metadata:         cloneMap(review.Metadata),
			Trace:            review.Trace,
		})
		return translateSkillError(err)
	case service.AISkillReviewActionDisabled:
		return a.insertDisabledReviewAudit(ctx, review)
	default:
		return nil
	}
}

func (a *serviceRepoAdapter) insertDisabledReviewAudit(ctx context.Context, review *service.AISkillReview) error {
	if a == nil || a.db == nil || review == nil {
		return service.ErrAISkillServiceUnavailable
	}

	snapshotJSON, err := json.Marshal(map[string]any{
		"skill_id":    review.SkillID,
		"version_id":  review.VersionID,
		"action":      review.Action,
		"status_from": review.StatusFrom,
		"status_to":   review.StatusTo,
	})
	if err != nil {
		return err
	}

	metadata := cloneMap(review.Metadata)
	metadata["action"] = review.Action
	if strings.TrimSpace(review.StatusFrom) != "" {
		metadata["status_from"] = review.StatusFrom
	}
	if strings.TrimSpace(review.StatusTo) != "" {
		metadata["status_to"] = review.StatusTo
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = a.db.ExecContext(ctx, `
INSERT INTO ai_skill_reviews (
    skill_id, version_id, submitter_user_id, reviewer_user_id, status,
    submit_note, review_note, snapshot, metadata, reviewed_at,
    request_id, usage_log_id, api_key_id, group_id
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14
)`,
		review.SkillID,
		review.VersionID,
		review.OperatorUserID,
		review.OperatorUserID,
		domain.AISkillReviewStatusCanceled,
		nil,
		nullableString(review.Comment),
		snapshotJSON,
		metadataJSON,
		time.Now().UTC(),
		nullableString(review.Trace.RequestID),
		nullableInt64(review.Trace.UsageLogID),
		nullableInt64(review.Trace.APIKeyID),
		nullableInt64(review.Trace.GroupID),
	)
	return err
}

func (a *serviceRepoAdapter) CreateRun(ctx context.Context, run *service.AISkillRun) error {
	if a == nil || a.domainRepo == nil || run == nil {
		return nil
	}
	entity := serviceRunToDomain(run)
	if err := a.domainRepo.CreateSkillRun(ctx, entity); err != nil {
		return translateSkillError(err)
	}
	*run = *domainRunToService(entity)
	return nil
}

func (a *serviceRepoAdapter) GetRunByID(ctx context.Context, id int64) (*service.AISkillRun, error) {
	entity, err := a.domainRepo.GetSkillRunByID(ctx, id)
	if err != nil {
		return nil, translateSkillError(err)
	}
	return domainRunToService(entity), nil
}

func (a *serviceRepoAdapter) UpdateRun(ctx context.Context, run *service.AISkillRun) error {
	if a == nil || a.domainRepo == nil || run == nil {
		return nil
	}
	entity := serviceRunToDomain(run)
	if err := a.domainRepo.UpdateSkillRun(ctx, entity); err != nil {
		return translateSkillError(err)
	}
	*run = *domainRunToService(entity)
	return nil
}

func (a *serviceRepoAdapter) CreateSettlement(ctx context.Context, settlement *service.AISkillSettlement) error {
	if a == nil || a.domainRepo == nil || settlement == nil {
		return nil
	}
	entity := serviceSettlementToDomain(settlement)
	if err := a.domainRepo.CreateSkillSettlement(ctx, entity); err != nil {
		return translateSkillError(err)
	}
	*settlement = *domainSettlementToService(entity)
	return nil
}

func (a *serviceRepoAdapter) GetSettlementByRunID(ctx context.Context, runID int64) (*service.AISkillSettlement, error) {
	if a == nil || a.db == nil {
		return nil, service.ErrAISkillSettlementUnavailable
	}
	row := a.db.QueryRowContext(ctx, `
SELECT id
FROM ai_skill_settlements
WHERE run_id = $1
ORDER BY id DESC
LIMIT 1`, runID)
	var settlementID int64
	if err := row.Scan(&settlementID); err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrAISkillSettlementNotFound
		}
		return nil, err
	}
	entity, err := a.domainRepo.GetSkillSettlementByID(ctx, settlementID)
	if err != nil {
		return nil, translateSkillError(err)
	}
	return domainSettlementToService(entity), nil
}

func (a *serviceRepoAdapter) UpdateSettlement(ctx context.Context, settlement *service.AISkillSettlement) error {
	if a == nil || a.domainRepo == nil || settlement == nil {
		return nil
	}
	entity := serviceSettlementToDomain(settlement)
	if err := a.domainRepo.UpdateSkillSettlement(ctx, entity); err != nil {
		return translateSkillError(err)
	}
	*settlement = *domainSettlementToService(entity)
	return nil
}

func (a *serviceRepoAdapter) lookupPendingReviewID(ctx context.Context, versionID int64) (int64, error) {
	row := a.db.QueryRowContext(ctx, `
SELECT id
FROM ai_skill_reviews
WHERE version_id = $1
  AND status = 'pending'
ORDER BY id DESC
LIMIT 1`, versionID)
	var reviewID int64
	if err := row.Scan(&reviewID); err != nil {
		if err == sql.ErrNoRows {
			return 0, service.ErrAISkillVersionNotFound
		}
		return 0, err
	}
	return reviewID, nil
}

func (a *serviceRepoAdapter) updateVersionSnapshot(ctx context.Context, version *domain.AISkillVersion) error {
	configJSON, err := json.Marshal(version.Config)
	if err != nil {
		return err
	}
	inputSchemaJSON, err := json.Marshal(version.InputSchema)
	if err != nil {
		return err
	}
	outputSchemaJSON, err := json.Marshal(version.OutputSchema)
	if err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(version.Metadata)
	if err != nil {
		return err
	}
	row := a.db.QueryRowContext(ctx, `
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
    approved_artifact_digest = $14,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING skill_id, user_id, version, review_status, submitted_at, reviewed_at, reviewer_user_id, review_note, created_at, updated_at`,
		version.ID,
		nullableString(version.ContentFormat),
		nullableString(version.Runtime),
		version.SourceContent,
		configJSON,
		inputSchemaJSON,
		outputSchemaJSON,
		nullableString(version.ChangeNote),
		metadataJSON,
		nullableString(version.Trace.RequestID),
		nullableInt64(version.Trace.UsageLogID),
		nullableInt64(version.Trace.APIKeyID),
		nullableInt64(version.Trace.GroupID),
		nullableString(version.ApprovedArtifactDigest),
	)
	var (
		submittedAt sql.NullTime
		reviewedAt  sql.NullTime
		reviewerID  sql.NullInt64
		reviewNote  sql.NullString
	)
	if err := row.Scan(
		&version.SkillID,
		&version.UserID,
		&version.Version,
		&version.ReviewStatus,
		&submittedAt,
		&reviewedAt,
		&reviewerID,
		&reviewNote,
		&version.CreatedAt,
		&version.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return service.ErrAISkillVersionNotFound
		}
		return err
	}
	version.SubmittedAt = nullTimePtr(submittedAt)
	version.ReviewedAt = nullTimePtr(reviewedAt)
	version.ReviewerUserID = nullInt64Ptr(reviewerID)
	version.ReviewNote = strings.TrimSpace(reviewNote.String)
	return nil
}

func shouldUseDomainVersionUpdate(serviceStatus, currentReviewStatus string) bool {
	if serviceStatus != service.AISkillVersionStatusDraft {
		return false
	}
	switch currentReviewStatus {
	case domain.AISkillVersionReviewStatusDraft, domain.AISkillVersionReviewStatusRejected:
		return true
	default:
		return false
	}
}

func translateSkillError(err error) error {
	switch err {
	case nil:
		return nil
	case domain.ErrAISkillNotFound:
		return service.ErrAISkillNotFound
	case domain.ErrAISkillVersionNotFound:
		return service.ErrAISkillVersionNotFound
	case domain.ErrAISkillRunNotFound:
		return service.ErrAISkillRunNotFound
	case domain.ErrAISkillSettlementNotFound:
		return service.ErrAISkillSettlementNotFound
	case domain.ErrAISkillTypeInvalid:
		return service.ErrAISkillTypeInvalid
	case domain.ErrAISkillTitleRequired:
		return service.ErrAISkillNameRequired
	case domain.ErrAISkillVersionStatusInvalid, domain.ErrAISkillVersionTransitionInvalid:
		return service.ErrAISkillVersionTransitionInvalid
	case domain.ErrAISkillVersionNotApproved:
		return service.ErrAISkillVersionNotApproved
	case domain.ErrAISkillRunModeInvalid:
		return service.ErrAISkillRunModeInvalid
	default:
		return err
	}
}

func serviceSkillToDomain(skill *service.AISkill) *domain.AISkill {
	if skill == nil {
		return nil
	}
	meta := cloneMap(skill.Metadata)
	pricing := readPricing(meta)
	visibility := domain.NormalizeAIVisibility(readString(meta, "visibility"))
	if visibility == "" {
		visibility = domain.AIVisibilityPrivate
	}
	sourceVisibility := domain.AISkillSourceVisibilityPublic
	if readBool(meta, metaKeySkillSourceLocked, pricing.Amount > 0) {
		sourceVisibility = domain.AISkillSourceVisibilityHidden
	}
	return &domain.AISkill{
		ID:               skill.ID,
		UserID:           skill.CreatorUserID,
		Type:             skill.Type,
		Title:            skill.Name,
		Summary:          readString(meta, metaKeySkillTagline),
		Description:      skill.Description,
		Category:         readString(meta, "category"),
		Tags:             readStringSlice(meta, "tags"),
		Visibility:       visibility,
		SourceVisibility: sourceVisibility,
		BillingMode:      domain.AISkillBillingModePerRequest,
		Price:            pricing.Amount,
		LatestVersion:    skill.LatestVersion,
		Metadata:         meta,
		Trace:            skill.Trace,
		CreatedAt:        skill.CreatedAt,
		UpdatedAt:        skill.UpdatedAt,
	}
}

func domainSkillToService(skill *domain.AISkill) *service.AISkill {
	if skill == nil {
		return nil
	}
	return &service.AISkill{
		ID:            skill.ID,
		CreatorUserID: skill.UserID,
		Name:          skill.Title,
		Description:   skill.Description,
		Type:          skill.Type,
		LatestVersion: skill.LatestVersion,
		Metadata:      cloneMap(skill.Metadata),
		Trace:         skill.Trace,
		CreatedAt:     skill.CreatedAt,
		UpdatedAt:     skill.UpdatedAt,
	}
}

func serviceVersionToDomain(version *service.AISkillVersion) *domain.AISkillVersion {
	if version == nil {
		return nil
	}
	meta := cloneMap(version.Metadata)
	billingPolicyMeta := cloneMap(mustMap(version.BillingPolicy))
	if mode := serviceBillingModeToDomainBillingPolicy(readString(billingPolicyMeta, "mode")); mode != "" {
		billingPolicyMeta["mode"] = mode
	}
	meta[metaKeySkillBillingPolicy] = billingPolicyMeta
	applyStatusOverride(meta, "")
	contentFormat := version.Type
	runtime := ""
	sourceContent := ""
	config := map[string]any{
		"execution_spec": mustMap(version.ExecutionSpec),
	}
	inputSchema := map[string]any{}
	outputSchema := map[string]any{}
	switch version.Type {
	case service.AISkillTypePromptChat:
		if version.ExecutionSpec.PromptChat != nil {
			sourceContent = version.ExecutionSpec.PromptChat.UserPromptTemplate
			inputSchema["variables"] = cloneSlice(version.ExecutionSpec.PromptChat.Variables)
			config["response_format"] = cloneMap(version.ExecutionSpec.PromptChat.ResponseFormat)
		}
	case service.AISkillTypePromptImage:
		if version.ExecutionSpec.PromptImage != nil {
			sourceContent = version.ExecutionSpec.PromptImage.PromptTemplate
			inputSchema["variables"] = cloneSlice(version.ExecutionSpec.PromptImage.Variables)
		}
	case service.AISkillTypeScript:
		if version.ExecutionSpec.Script != nil {
			runtime = version.ExecutionSpec.Script.Runtime
			sourceContent = readScriptSourceContent(meta)
			config["environment"] = cloneStringMap(version.ExecutionSpec.Script.Environment)
			config["arguments"] = cloneSlice(version.ExecutionSpec.Script.Arguments)
		}
	}
	return &domain.AISkillVersion{
		ID:                     version.ID,
		SkillID:                version.SkillID,
		UserID:                 version.CreatorUserID,
		Version:                version.Version,
		ReviewStatus:           domainReviewStatus(version.Status),
		ApprovedArtifactDigest: strings.TrimSpace(version.ApprovedArtifactDigest),
		ContentFormat:          contentFormat,
		Runtime:                runtime,
		SourceContent:          sourceContent,
		Config:                 config,
		InputSchema:            inputSchema,
		OutputSchema:           outputSchema,
		ChangeNote:             version.ChangeNote,
		SubmittedAt:            version.SubmittedAt,
		ReviewedAt:             version.ReviewedAt,
		ReviewerUserID:         version.ReviewerUserID,
		ReviewNote:             version.ReviewComment,
		Metadata:               meta,
		Trace:                  version.Trace,
		CreatedAt:              version.CreatedAt,
		UpdatedAt:              version.UpdatedAt,
	}
}

func domainVersionToService(version *domain.AISkillVersion) *service.AISkillVersion {
	if version == nil {
		return nil
	}
	meta := cloneMap(version.Metadata)
	if sourceContent := version.SourceContent; strings.TrimSpace(sourceContent) != "" {
		if content, ok := meta[metaKeySkillContent].(map[string]any); ok {
			cloned := cloneMap(content)
			if readString(cloned, "source_code") == "" {
				cloned["source_code"] = sourceContent
			}
			meta[metaKeySkillContent] = cloned
		}
		if readString(meta, "source_code") == "" {
			meta["source_code"] = sourceContent
		}
	}
	spec := decodeExecutionSpec(version.ContentFormat, version.Config)
	billing := decodeBillingPolicy(meta)
	billing.Mode = domainBillingModeToServiceBillingPolicy(billing.Mode)
	status := serviceVersionStatus(version.ReviewStatus, meta)
	return &service.AISkillVersion{
		ID:                     version.ID,
		SkillID:                version.SkillID,
		CreatorUserID:          version.UserID,
		Version:                version.Version,
		Type:                   version.ContentFormat,
		Status:                 status,
		ApprovedArtifactDigest: strings.TrimSpace(version.ApprovedArtifactDigest),
		ExecutionSpec:          spec,
		BillingPolicy:          billing,
		ChangeNote:             version.ChangeNote,
		ReviewComment:          version.ReviewNote,
		Metadata:               meta,
		SubmittedAt:            version.SubmittedAt,
		ReviewedAt:             version.ReviewedAt,
		ApprovedAt:             approvedAtForStatus(status, version.ReviewedAt),
		RejectedAt:             rejectedAtForStatus(status, version.ReviewedAt),
		DisabledAt:             disabledAtForStatus(status, version.ReviewedAt),
		ReviewerUserID:         version.ReviewerUserID,
		Trace:                  version.Trace,
		CreatedAt:              version.CreatedAt,
		UpdatedAt:              version.UpdatedAt,
	}
}

func serviceRunToDomain(run *service.AISkillRun) *domain.AISkillRun {
	if run == nil {
		return nil
	}
	meta := cloneMap(run.Metadata)
	meta["attachments"] = cloneAttachments(run.Attachments)
	return &domain.AISkillRun{
		ID:            run.ID,
		SkillID:       run.SkillID,
		VersionID:     run.VersionID,
		UserID:        run.UserID,
		RunMode:       domain.NormalizeAISkillRunMode(run.Mode),
		Status:        domainRunStatus(run.Status),
		BillingMode:   domain.AISkillBillingModePerRequest,
		Price:         run.ChargeAmount,
		InputPayload:  cloneMap(run.Parameters),
		OutputPayload: cloneMap(run.Output),
		ErrorMessage:  run.ErrorMessage,
		Metadata:      meta,
		Trace:         run.Trace,
		CreatedAt:     run.CreatedAt,
		UpdatedAt:     run.UpdatedAt,
	}
}

func domainRunToService(run *domain.AISkillRun) *service.AISkillRun {
	if run == nil {
		return nil
	}
	meta := cloneMap(run.Metadata)
	return &service.AISkillRun{
		ID:           run.ID,
		SkillID:      run.SkillID,
		VersionID:    run.VersionID,
		UserID:       run.UserID,
		Mode:         run.RunMode,
		Type:         readString(meta, "type"),
		Status:       serviceRunStatus(run.Status),
		BillingMode:  service.AISkillBillingModePerRun,
		ChargeAmount: run.Price,
		Currency:     readString(meta, "currency"),
		Parameters:   cloneMap(run.InputPayload),
		Attachments:  decodeAttachments(meta["attachments"]),
		Metadata:     meta,
		Output:       cloneMap(run.OutputPayload),
		ErrorMessage: run.ErrorMessage,
		Trace:        run.Trace,
		CreatedAt:    run.CreatedAt,
		UpdatedAt:    run.UpdatedAt,
	}
}

func serviceSettlementToDomain(settlement *service.AISkillSettlement) *domain.AISkillSettlement {
	if settlement == nil {
		return nil
	}
	meta := cloneMap(settlement.Metadata)
	meta["currency"] = settlement.Currency
	meta["platform_amount"] = settlement.PlatformAmount
	meta["creator_amount"] = settlement.CreatorAmount
	meta["creator_credited_amount"] = settlement.CreatorCreditedAmount
	meta["platform_commission_rate"] = settlement.PlatformCommissionRate
	meta["failure_reason"] = settlement.FailureReason
	meta["service_status"] = settlement.Status
	return &domain.AISkillSettlement{
		ID:          settlement.ID,
		SkillID:     settlement.SkillID,
		VersionID:   settlement.VersionID,
		RunID:       settlement.RunID,
		OwnerUserID: settlement.CreatorUserID,
		BuyerUserID: settlement.BuyerUserID,
		Status:      domainSettlementStatus(settlement.Status),
		BillingMode: domain.AISkillBillingModePerRequest,
		Amount:      settlement.TotalAmount,
		QuotaAmount: settlement.CreatorAmount,
		Metadata:    meta,
		Trace:       settlement.Trace,
		CreatedAt:   settlement.CreatedAt,
		UpdatedAt:   settlement.UpdatedAt,
	}
}

func domainSettlementToService(settlement *domain.AISkillSettlement) *service.AISkillSettlement {
	if settlement == nil {
		return nil
	}
	meta := cloneMap(settlement.Metadata)
	balanceAfter := readOptionalFloat(meta, "balance_after")
	return &service.AISkillSettlement{
		ID:                     settlement.ID,
		RunID:                  settlement.RunID,
		SkillID:                settlement.SkillID,
		VersionID:              settlement.VersionID,
		BuyerUserID:            settlement.BuyerUserID,
		CreatorUserID:          settlement.OwnerUserID,
		BillingMode:            service.AISkillBillingModePerRun,
		Currency:               readString(meta, "currency"),
		TotalAmount:            settlement.Amount,
		PlatformAmount:         readFloat(meta, "platform_amount"),
		CreatorAmount:          readFloat(meta, "creator_amount"),
		CreatorCreditedAmount:  readFloat(meta, "creator_credited_amount"),
		PlatformCommissionRate: readFloat(meta, "platform_commission_rate"),
		Status:                 serviceSettlementStatus(settlement.Status, meta),
		BalanceAfter:           balanceAfter,
		FailureReason:          readString(meta, "failure_reason"),
		Metadata:               meta,
		Trace:                  settlement.Trace,
		CreatedAt:              settlement.CreatedAt,
		UpdatedAt:              settlement.UpdatedAt,
	}
}

func domainReviewStatus(status string) string {
	switch status {
	case service.AISkillVersionStatusSubmitted:
		return domain.AISkillVersionReviewStatusPending
	case service.AISkillVersionStatusApproved:
		return domain.AISkillVersionReviewStatusApproved
	case service.AISkillVersionStatusRejected:
		return domain.AISkillVersionReviewStatusRejected
	case service.AISkillVersionStatusDisabled:
		return domain.AISkillVersionReviewStatusApproved
	default:
		return domain.AISkillVersionReviewStatusDraft
	}
}

func serviceVersionStatus(reviewStatus string, meta map[string]any) string {
	if override := readStatusOverride(meta); override != "" {
		switch override {
		case service.AISkillVersionStatusDraft:
			return service.AISkillVersionStatusDraft
		case metaValueSkillStatusDisabled:
			return service.AISkillVersionStatusDisabled
		}
	}
	switch reviewStatus {
	case domain.AISkillVersionReviewStatusPending:
		return service.AISkillVersionStatusSubmitted
	case domain.AISkillVersionReviewStatusApproved:
		return service.AISkillVersionStatusApproved
	case domain.AISkillVersionReviewStatusRejected:
		return service.AISkillVersionStatusRejected
	default:
		return service.AISkillVersionStatusDraft
	}
}

func domainRunStatus(status string) string {
	switch status {
	case service.AISkillRunStatusSucceeded:
		return domain.AISkillRunStatusSucceeded
	case service.AISkillRunStatusFailed:
		return domain.AISkillRunStatusFailed
	case service.AISkillRunStatusDispatched:
		return domain.AISkillRunStatusRunning
	default:
		return domain.AISkillRunStatusQueued
	}
}

func serviceRunStatus(status string) string {
	switch status {
	case domain.AISkillRunStatusRunning:
		return service.AISkillRunStatusDispatched
	case domain.AISkillRunStatusSucceeded:
		return service.AISkillRunStatusSucceeded
	case domain.AISkillRunStatusFailed:
		return service.AISkillRunStatusFailed
	default:
		return service.AISkillRunStatusPrepared
	}
}

func domainSettlementStatus(status string) string {
	switch status {
	case service.AISkillSettlementStatusSettled:
		return domain.AISkillSettlementStatusTransferred
	case service.AISkillSettlementStatusFailed:
		return domain.AISkillSettlementStatusCanceled
	default:
		return domain.AISkillSettlementStatusPending
	}
}

func serviceSettlementStatus(status string, meta map[string]any) string {
	if value := readString(meta, "service_status"); value != "" {
		return value
	}
	switch status {
	case domain.AISkillSettlementStatusTransferred:
		return service.AISkillSettlementStatusSettled
	case domain.AISkillSettlementStatusCanceled:
		return service.AISkillSettlementStatusFailed
	default:
		return service.AISkillSettlementStatusPending
	}
}

func serviceBillingModeToDomainBillingPolicy(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case service.AISkillBillingModePerRun:
		return domain.AISkillBillingModePerRequest
	case service.AISkillBillingModeFree:
		return service.AISkillBillingModeFree
	default:
		return strings.TrimSpace(mode)
	}
}

func domainBillingModeToServiceBillingPolicy(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case domain.AISkillBillingModePerRequest:
		return service.AISkillBillingModePerRun
	case service.AISkillBillingModeFree:
		return service.AISkillBillingModeFree
	default:
		return strings.TrimSpace(mode)
	}
}

func approvedAtForStatus(status string, reviewedAt *time.Time) *time.Time {
	if status == service.AISkillVersionStatusApproved {
		return reviewedAt
	}
	return nil
}

func rejectedAtForStatus(status string, reviewedAt *time.Time) *time.Time {
	if status == service.AISkillVersionStatusRejected {
		return reviewedAt
	}
	return nil
}

func disabledAtForStatus(status string, reviewedAt *time.Time) *time.Time {
	if status == service.AISkillVersionStatusDisabled {
		return reviewedAt
	}
	return nil
}

func decodeExecutionSpec(skillType string, config map[string]any) service.AISkillExecutionSpec {
	if raw, ok := config["execution_spec"]; ok {
		var spec service.AISkillExecutionSpec
		if decodeViaJSON(raw, &spec) == nil {
			return spec
		}
	}
	return service.AISkillExecutionSpec{Type: skillType}
}

func decodeBillingPolicy(meta map[string]any) service.AISkillBillingPolicy {
	raw, ok := meta[metaKeySkillBillingPolicy]
	if !ok {
		return service.AISkillBillingPolicy{Mode: service.AISkillBillingModeFree}
	}
	var policy service.AISkillBillingPolicy
	if decodeViaJSON(raw, &policy) == nil {
		if policy.Mode == "" {
			policy.Mode = service.AISkillBillingModeFree
		}
		return policy
	}
	return service.AISkillBillingPolicy{Mode: service.AISkillBillingModeFree}
}

func decodeViaJSON(src any, out any) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func mustMap(value any) map[string]any {
	out := map[string]any{}
	_ = decodeViaJSON(value, &out)
	return out
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneSlice(src []map[string]any) []map[string]any {
	if len(src) == 0 {
		return []map[string]any{}
	}
	dst := make([]map[string]any, 0, len(src))
	for _, item := range src {
		dst = append(dst, cloneMap(item))
	}
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneAttachments(src []service.AISkillRunAttachment) []map[string]any {
	if len(src) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(src))
	for _, item := range src {
		entry := map[string]any{
			"url":       item.URL,
			"purpose":   item.Purpose,
			"file_name": item.FileName,
		}
		if item.AssetID != nil {
			entry["asset_id"] = *item.AssetID
		}
		if item.MediaID != nil {
			entry["media_id"] = *item.MediaID
		}
		out = append(out, entry)
	}
	return out
}

func decodeAttachments(raw any) []service.AISkillRunAttachment {
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]service.AISkillRunAttachment, 0, len(entries))
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		attachment := service.AISkillRunAttachment{
			URL:      readString(item, "url"),
			Purpose:  readString(item, "purpose"),
			FileName: readString(item, "file_name"),
		}
		if value, ok := readInt64(item, "asset_id"); ok {
			attachment.AssetID = &value
		}
		if value, ok := readInt64(item, "media_id"); ok {
			attachment.MediaID = &value
		}
		out = append(out, attachment)
	}
	return out
}

type pricingMeta struct {
	Mode   string
	Amount float64
}

func readPricing(meta map[string]any) pricingMeta {
	if raw, ok := meta[metaKeySkillPricing]; ok {
		record, ok := raw.(map[string]any)
		if ok {
			mode := strings.TrimSpace(readString(record, "mode"))
			if mode == "paid" {
				return pricingMeta{Mode: "paid", Amount: readFloat(record, "amount")}
			}
		}
	}
	if readString(meta, "price_mode") == "paid" {
		return pricingMeta{Mode: "paid", Amount: readFloat(meta, "price_amount")}
	}
	return pricingMeta{Mode: "free", Amount: 0}
}

func applyStatusOverride(meta map[string]any, value string) {
	if meta == nil {
		return
	}
	if strings.TrimSpace(value) == "" {
		delete(meta, metaKeySkillStatusOverride)
		return
	}
	meta[metaKeySkillStatusOverride] = value
}

func readStatusOverride(meta map[string]any) string {
	return strings.TrimSpace(readString(meta, metaKeySkillStatusOverride))
}

func readScriptSourceContent(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	if raw, ok := meta[metaKeySkillContent]; ok {
		if content, ok := raw.(map[string]any); ok {
			if source := readRawString(content, "source_code"); source != "" {
				return source
			}
			if source := readRawString(content, "code"); source != "" {
				return source
			}
			if source := readRawString(content, "script"); source != "" {
				return source
			}
		}
	}
	if source := readRawString(meta, "source_code"); source != "" {
		return source
	}
	if source := readRawString(meta, "code"); source != "" {
		return source
	}
	return readRawString(meta, "script")
}

func readString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	value, ok := meta[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func readRawString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	value, ok := meta[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		return text
	}
	return ""
}

func readBool(meta map[string]any, key string, fallback bool) bool {
	if meta == nil {
		return fallback
	}
	value, ok := meta[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return fallback
}

func readFloat(meta map[string]any, key string) float64 {
	if meta == nil {
		return 0
	}
	value, ok := meta[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		out, _ := v.Float64()
		return out
	case string:
		var out float64
		if _, err := fmt.Sscan(strings.TrimSpace(v), &out); err == nil {
			return out
		}
		return 0
	default:
		return 0
	}
}

func readOptionalFloat(meta map[string]any, key string) *float64 {
	if meta == nil {
		return nil
	}
	value, ok := meta[key]
	if !ok {
		return nil
	}
	out := readFloat(map[string]any{key: value}, key)
	return &out
}

func readStringSlice(meta map[string]any, key string) []string {
	if meta == nil {
		return nil
	}
	value, ok := meta[key]
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func readInt64(meta map[string]any, key string) (int64, bool) {
	if meta == nil {
		return 0, false
	}
	value, ok := meta[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case json.Number:
		out, err := v.Int64()
		return out, err == nil
	default:
		return 0, false
	}
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
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
