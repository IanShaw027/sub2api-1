package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// AISkillDomainService is the application boundary for domain-shaped skill
// queries that have not yet been migrated to the public AISkill DTO services.
type AISkillDomainService struct {
	repo AISkillDomainRepository
}

func NewAISkillDomainService(repo AISkillDomainRepository) *AISkillDomainService {
	return &AISkillDomainService{repo: repo}
}

func (s *AISkillDomainService) repository() (AISkillDomainRepository, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	return s.repo, nil
}

func (s *AISkillDomainService) GetSkillByID(ctx context.Context, id int64) (*domain.AISkill, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, err
	}
	return repo.GetSkillByID(ctx, id)
}

func (s *AISkillDomainService) GetSkillByUserAndID(ctx context.Context, userID, id int64) (*domain.AISkill, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, err
	}
	return repo.GetSkillByUserAndID(ctx, userID, id)
}

func (s *AISkillDomainService) ListSkills(ctx context.Context, viewerUserID int64, isAdmin bool, params pagination.PaginationParams, filter domain.AISkillListFilter) ([]domain.AISkill, *pagination.PaginationResult, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, nil, err
	}
	return repo.ListSkills(ctx, viewerUserID, isAdmin, params, filter)
}

func (s *AISkillDomainService) SetSkillInstall(ctx context.Context, skillID, userID int64, installed bool) error {
	repo, err := s.repository()
	if err != nil {
		return err
	}
	return repo.SetSkillInstall(ctx, skillID, userID, installed)
}

func (s *AISkillDomainService) HasSkillInstall(ctx context.Context, skillID, userID int64) (bool, error) {
	repo, err := s.repository()
	if err != nil {
		return false, err
	}
	return repo.HasSkillInstall(ctx, skillID, userID)
}

func (s *AISkillDomainService) GetSkillInstallStates(ctx context.Context, userID int64, skillIDs []int64) (map[int64]bool, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, err
	}
	return repo.GetSkillInstallStates(ctx, userID, skillIDs)
}

func (s *AISkillDomainService) GetSkillInstallCounts(ctx context.Context, skillIDs []int64) (map[int64]int, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, err
	}
	return repo.GetSkillInstallCounts(ctx, skillIDs)
}

func (s *AISkillDomainService) UpdateSkill(ctx context.Context, skill *domain.AISkill) error {
	repo, err := s.repository()
	if err != nil {
		return err
	}
	return repo.UpdateSkill(ctx, skill)
}

func (s *AISkillDomainService) GetSkillVersionByID(ctx context.Context, id int64) (*domain.AISkillVersion, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, err
	}
	return repo.GetSkillVersionByID(ctx, id)
}

func (s *AISkillDomainService) ListSkillVersions(ctx context.Context, skillID int64) ([]domain.AISkillVersion, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, err
	}
	return repo.ListSkillVersions(ctx, skillID)
}

func (s *AISkillDomainService) GetSkillReviewByID(ctx context.Context, id int64) (*domain.AISkillReview, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, err
	}
	return repo.GetSkillReviewByID(ctx, id)
}

func (s *AISkillDomainService) GetSkillSettlementByID(ctx context.Context, id int64) (*domain.AISkillSettlement, error) {
	repo, err := s.repository()
	if err != nil {
		return nil, err
	}
	return repo.GetSkillSettlementByID(ctx, id)
}
