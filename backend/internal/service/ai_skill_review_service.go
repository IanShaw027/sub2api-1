package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AISkillReviewService struct {
	versionRepo AISkillVersionRepository
	reviewRepo  AISkillReviewRepository
	now         func() time.Time
}

func NewAISkillReviewService(versionRepo AISkillVersionRepository, reviewRepo AISkillReviewRepository) *AISkillReviewService {
	return &AISkillReviewService{
		versionRepo: versionRepo,
		reviewRepo:  reviewRepo,
		now:         time.Now,
	}
}

func (s *AISkillReviewService) ApproveVersion(ctx context.Context, reviewerUserID, versionID int64, input *AIReviewSkillVersionInput) (*AISkillVersion, error) {
	return s.applyDecision(ctx, reviewerUserID, versionID, AISkillVersionStatusApproved, AISkillReviewActionApproved, input)
}

func (s *AISkillReviewService) RejectVersion(ctx context.Context, reviewerUserID, versionID int64, input *AIReviewSkillVersionInput) (*AISkillVersion, error) {
	return s.applyDecision(ctx, reviewerUserID, versionID, AISkillVersionStatusRejected, AISkillReviewActionRejected, input)
}

func (s *AISkillReviewService) DisableVersion(ctx context.Context, reviewerUserID, versionID int64, input *AIReviewSkillVersionInput) (*AISkillVersion, error) {
	return s.applyDecision(ctx, reviewerUserID, versionID, AISkillVersionStatusDisabled, AISkillReviewActionDisabled, input)
}

func (s *AISkillReviewService) applyDecision(ctx context.Context, reviewerUserID, versionID int64, targetStatus, action string, input *AIReviewSkillVersionInput) (*AISkillVersion, error) {
	if s == nil || s.versionRepo == nil || s.reviewRepo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if reviewerUserID <= 0 {
		return nil, infraerrors.BadRequest("AI_SKILL_REVIEWER_INVALID", "ai skill reviewer is invalid")
	}
	version, err := s.versionRepo.GetVersionByID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	statusFrom := version.Status
	nextStatus, err := transitionAISkillVersionStatus(statusFrom, targetStatus)
	if err != nil {
		return nil, err
	}
	now := s.nowOrDefault()
	version.Status = nextStatus
	version.ReviewerUserID = &reviewerUserID
	version.ReviewedAt = &now
	version.Trace = normalizeAIWriteTrace(ctx, aiSkillReviewTrace(input))
	version.UpdatedAt = now
	version.ReviewComment = strings.TrimSpace(valueOrEmpty(aiSkillReviewComment(input)))

	switch nextStatus {
	case AISkillVersionStatusApproved:
		version.ApprovedAt = &now
		version.RejectedAt = nil
		version.DisabledAt = nil
	case AISkillVersionStatusRejected:
		version.RejectedAt = &now
		version.ApprovedAt = nil
		version.DisabledAt = nil
	case AISkillVersionStatusDisabled:
		version.DisabledAt = &now
	}

	if err := s.versionRepo.UpdateVersion(ctx, version); err != nil {
		return nil, err
	}
	if err := s.reviewRepo.CreateReview(ctx, &AISkillReview{
		SkillID:        version.SkillID,
		VersionID:      version.ID,
		OperatorUserID: reviewerUserID,
		Action:         action,
		StatusFrom:     statusFrom,
		StatusTo:       nextStatus,
		Comment:        version.ReviewComment,
		Metadata:       cloneAIMap(aiSkillReviewMetadata(input)),
		Trace:          version.Trace,
		CreatedAt:      now,
	}); err != nil {
		return nil, err
	}
	return version, nil
}

func (s *AISkillReviewService) nowOrDefault() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func aiSkillReviewComment(input *AIReviewSkillVersionInput) *string {
	if input == nil {
		return nil
	}
	return input.Comment
}

func aiSkillReviewMetadata(input *AIReviewSkillVersionInput) map[string]any {
	if input == nil {
		return nil
	}
	return input.Metadata
}

func aiSkillReviewTrace(input *AIReviewSkillVersionInput) AIWriteTrace {
	if input == nil {
		return AIWriteTrace{}
	}
	return input.Trace
}
