package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AISkillService struct {
	repo AISkillRepository
	now  func() time.Time
}

func NewAISkillService(repo AISkillRepository) *AISkillService {
	return &AISkillService{
		repo: repo,
		now:  time.Now,
	}
}

func (s *AISkillService) CreateSkill(ctx context.Context, creatorUserID int64, input *AICreateSkillInput) (*AISkill, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if creatorUserID <= 0 || input == nil {
		return nil, infraerrors.BadRequest("AI_SKILL_INPUT_REQUIRED", "ai skill input is required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrAISkillNameRequired
	}
	skillType := normalizeAISkillType(input.Type)
	if skillType == "" {
		return nil, ErrAISkillTypeInvalid
	}
	now := s.nowOrDefault()
	skill := &AISkill{
		CreatorUserID: creatorUserID,
		Name:          name,
		Description:   strings.TrimSpace(valueOrEmpty(input.Description)),
		Type:          skillType,
		LatestVersion: 0,
		Metadata:      cloneAIMap(input.Metadata),
		Trace:         normalizeAIWriteTrace(ctx, input.Trace),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateSkill(ctx, skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *AISkillService) GetSkill(ctx context.Context, skillID int64) (*AISkill, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if skillID <= 0 {
		return nil, ErrAISkillNotFound
	}
	return s.repo.GetSkillByID(ctx, skillID)
}

func (s *AISkillService) UpdateSkill(ctx context.Context, creatorUserID, skillID int64, input *AIUpdateSkillInput) (*AISkill, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if creatorUserID <= 0 || input == nil {
		return nil, infraerrors.BadRequest("AI_SKILL_INPUT_REQUIRED", "ai skill input is required")
	}
	skill, err := s.repo.GetSkillByCreatorAndID(ctx, creatorUserID, skillID)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrAISkillNameRequired
		}
		skill.Name = name
	}
	if input.Description != nil {
		skill.Description = strings.TrimSpace(valueOrEmpty(normalizeAIStringPtr(input.Description)))
	}
	if input.Metadata != nil {
		skill.Metadata = cloneAIMap(*input.Metadata)
	}
	skill.Trace = normalizeAIWriteTrace(ctx, input.Trace)
	skill.UpdatedAt = s.nowOrDefault()
	if err := s.repo.UpdateSkill(ctx, skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *AISkillService) nowOrDefault() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

type AISkillVersionService struct {
	skillRepo   AISkillRepository
	versionRepo AISkillVersionRepository
	reviewRepo  AISkillReviewRepository
	now         func() time.Time
}

func NewAISkillVersionService(skillRepo AISkillRepository, versionRepo AISkillVersionRepository, reviewRepo AISkillReviewRepository) *AISkillVersionService {
	return &AISkillVersionService{
		skillRepo:   skillRepo,
		versionRepo: versionRepo,
		reviewRepo:  reviewRepo,
		now:         time.Now,
	}
}

type aiSkillAtomicVersionCreator interface {
	CreateVersionWithSkill(ctx context.Context, skill *AISkill, version *AISkillVersion) error
}

func (s *AISkillVersionService) CreateVersion(ctx context.Context, creatorUserID, skillID int64, input *AICreateSkillVersionInput) (*AISkillVersion, error) {
	if s == nil || s.skillRepo == nil || s.versionRepo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if creatorUserID <= 0 || input == nil {
		return nil, infraerrors.BadRequest("AI_SKILL_VERSION_INPUT_REQUIRED", "ai skill version input is required")
	}
	skill, err := s.skillRepo.GetSkillByCreatorAndID(ctx, creatorUserID, skillID)
	if err != nil {
		return nil, err
	}
	spec, err := normalizeAISkillExecutionSpec(skill.Type, input.ExecutionSpec)
	if err != nil {
		return nil, err
	}
	billing, err := normalizeAISkillBillingPolicy(input.BillingPolicy)
	if err != nil {
		return nil, err
	}
	now := s.nowOrDefault()
	version := &AISkillVersion{
		SkillID:       skill.ID,
		CreatorUserID: creatorUserID,
		Version:       skill.LatestVersion + 1,
		Type:          skill.Type,
		Status:        AISkillVersionStatusDraft,
		ExecutionSpec: cloneAISkillExecutionSpec(spec),
		BillingPolicy: billing,
		ChangeNote:    strings.TrimSpace(valueOrEmpty(input.ChangeNote)),
		Metadata:      cloneAIMap(input.Metadata),
		Trace:         normalizeAIWriteTrace(ctx, input.Trace),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	skill.LatestVersion = version.Version
	skill.Trace = version.Trace
	skill.UpdatedAt = now
	if writer, ok := s.versionRepo.(aiSkillAtomicVersionCreator); ok {
		if err := writer.CreateVersionWithSkill(ctx, skill, version); err != nil {
			return nil, err
		}
		return version, nil
	}
	if err := s.versionRepo.CreateVersion(ctx, version); err != nil {
		return nil, err
	}
	if err := s.skillRepo.UpdateSkill(ctx, skill); err != nil {
		return nil, err
	}
	return version, nil
}

func (s *AISkillVersionService) GetVersion(ctx context.Context, versionID int64) (*AISkillVersion, error) {
	if s == nil || s.versionRepo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if versionID <= 0 {
		return nil, ErrAISkillVersionNotFound
	}
	return s.versionRepo.GetVersionByID(ctx, versionID)
}

func (s *AISkillVersionService) UpdateVersionDraft(ctx context.Context, creatorUserID, versionID int64, input *AIUpdateSkillVersionInput) (*AISkillVersion, error) {
	if s == nil || s.versionRepo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if creatorUserID <= 0 || input == nil {
		return nil, infraerrors.BadRequest("AI_SKILL_VERSION_INPUT_REQUIRED", "ai skill version input is required")
	}
	version, err := s.versionRepo.GetVersionByCreatorAndID(ctx, creatorUserID, versionID)
	if err != nil {
		return nil, err
	}
	if !canAISkillVersionEdit(version.Status) {
		return nil, ErrAISkillVersionNotEditable
	}
	spec := cloneAISkillExecutionSpec(version.ExecutionSpec)
	if input.ExecutionSpec != nil {
		spec = *input.ExecutionSpec
	}
	spec, err = normalizeAISkillExecutionSpec(version.Type, spec)
	if err != nil {
		return nil, err
	}
	billing := version.BillingPolicy
	if input.BillingPolicy != nil {
		billing = *input.BillingPolicy
	}
	billing, err = normalizeAISkillBillingPolicy(billing)
	if err != nil {
		return nil, err
	}
	version.ExecutionSpec = cloneAISkillExecutionSpec(spec)
	version.BillingPolicy = billing
	if input.ChangeNote != nil {
		version.ChangeNote = strings.TrimSpace(valueOrEmpty(normalizeAIStringPtr(input.ChangeNote)))
	}
	if input.Metadata != nil {
		version.Metadata = cloneAIMap(*input.Metadata)
	}
	if normalizeAISkillVersionStatus(version.Status) == AISkillVersionStatusRejected {
		version.Status = AISkillVersionStatusDraft
		version.SubmittedAt = nil
		resetAISkillReviewState(version)
	}
	version.Trace = normalizeAIWriteTrace(ctx, input.Trace)
	version.UpdatedAt = s.nowOrDefault()
	if err := s.versionRepo.UpdateVersion(ctx, version); err != nil {
		return nil, err
	}
	return version, nil
}

func (s *AISkillVersionService) SubmitVersion(ctx context.Context, creatorUserID, versionID int64, input *AISubmitSkillVersionInput) (*AISkillVersion, error) {
	if s == nil || s.versionRepo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if creatorUserID <= 0 {
		return nil, infraerrors.BadRequest("AI_SKILL_VERSION_SUBMITTER_INVALID", "ai skill submitter is invalid")
	}
	version, err := s.versionRepo.GetVersionByCreatorAndID(ctx, creatorUserID, versionID)
	if err != nil {
		return nil, err
	}
	statusFrom := version.Status
	nextStatus, err := transitionAISkillVersionStatus(statusFrom, AISkillVersionStatusSubmitted)
	if err != nil {
		return nil, err
	}
	spec, err := normalizeAISkillExecutionSpec(version.Type, version.ExecutionSpec)
	if err != nil {
		return nil, err
	}
	billing, err := normalizeAISkillBillingPolicy(version.BillingPolicy)
	if err != nil {
		return nil, err
	}
	now := s.nowOrDefault()
	version.Status = nextStatus
	version.ExecutionSpec = cloneAISkillExecutionSpec(spec)
	version.BillingPolicy = billing
	version.SubmittedAt = &now
	resetAISkillReviewState(version)
	version.Trace = normalizeAIWriteTrace(ctx, aiSkillSubmitTrace(input))
	version.UpdatedAt = now
	if err := s.versionRepo.UpdateVersion(ctx, version); err != nil {
		return nil, err
	}
	if s.reviewRepo != nil {
		if err := s.reviewRepo.CreateReview(ctx, &AISkillReview{
			SkillID:        version.SkillID,
			VersionID:      version.ID,
			OperatorUserID: creatorUserID,
			Action:         AISkillReviewActionSubmitted,
			StatusFrom:     statusFrom,
			StatusTo:       nextStatus,
			Comment:        strings.TrimSpace(valueOrEmpty(aiSkillSubmitComment(input))),
			Metadata:       cloneAIMap(aiSkillSubmitMetadata(input)),
			Trace:          version.Trace,
			CreatedAt:      now,
		}); err != nil {
			return nil, err
		}
	}
	return version, nil
}

func (s *AISkillVersionService) nowOrDefault() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func aiSkillSubmitComment(input *AISubmitSkillVersionInput) *string {
	if input == nil {
		return nil
	}
	return input.Comment
}

func aiSkillSubmitMetadata(input *AISubmitSkillVersionInput) map[string]any {
	if input == nil {
		return nil
	}
	return input.Metadata
}

func aiSkillSubmitTrace(input *AISubmitSkillVersionInput) AIWriteTrace {
	if input == nil {
		return AIWriteTrace{}
	}
	return input.Trace
}
