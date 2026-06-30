package service

import (
	"context"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrGroupNotFound = infraerrors.NotFound("GROUP_NOT_FOUND", "group not found")
	ErrGroupExists   = infraerrors.Conflict("GROUP_EXISTS", "group name already exists")
)

type GroupRepository interface {
	Create(ctx context.Context, group *Group) error
	GetByID(ctx context.Context, id int64) (*Group, error)
	GetByIDLite(ctx context.Context, id int64) (*Group, error)
	Update(ctx context.Context, group *Group) error
	Delete(ctx context.Context, id int64) error
	DeleteCascade(ctx context.Context, id int64) ([]int64, error)

	List(ctx context.Context, params pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error)
	ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, status, search string, isExclusive *bool) ([]Group, *pagination.PaginationResult, error)
	ListActive(ctx context.Context) ([]Group, error)
	ListActiveByPlatform(ctx context.Context, platform string) ([]Group, error)

	ExistsByName(ctx context.Context, name string) (bool, error)
	GetAccountCount(ctx context.Context, groupID int64) (total int64, active int64, err error)
	DeleteAccountGroupsByGroupID(ctx context.Context, groupID int64) (int64, error)
	// GetAccountIDsByGroupIDs 获取多个分组的所有账号 ID（去重）
	GetAccountIDsByGroupIDs(ctx context.Context, groupIDs []int64) ([]int64, error)
	// BindAccountsToGroup 将多个账号绑定到指定分组
	BindAccountsToGroup(ctx context.Context, groupID int64, accountIDs []int64) error
	// UpdateSortOrders 批量更新分组排序
	UpdateSortOrders(ctx context.Context, updates []GroupSortOrderUpdate) error
}

// GroupSortOrderUpdate 分组排序更新
type GroupSortOrderUpdate struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sort_order"`
}

// CreateGroupRequest 创建分组请求
type CreateGroupRequest struct {
	Name                 string  `json:"name"`
	Description          string  `json:"description"`
	RateMultiplier       float64 `json:"rate_multiplier"`
	RefundRateMultiplier float64 `json:"refund_rate_multiplier"`
	IsExclusive          bool    `json:"is_exclusive"`
	AllowImageGeneration bool    `json:"allow_image_generation"`
	ImageGenerationRoute string  `json:"image_generation_route"`

	AllowVideoGeneration  bool     `json:"allow_video_generation"`
	VideoGenerationRoute  string   `json:"video_generation_route"`
	VideoPrice480pPerSec  *float64 `json:"video_price_480p_per_sec"`
	VideoPrice720pPerSec  *float64 `json:"video_price_720p_per_sec"`
	VideoPrice1080pPerSec *float64 `json:"video_price_1080p_per_sec"`
	VideoPrice4kPerSec    *float64 `json:"video_price_4k_per_sec"`
	OpenAIImageMainModel  string   `json:"openai_image_main_model"`
	ImageRateIndependent  bool     `json:"image_rate_independent"`
	ImageRateMultiplier   *float64 `json:"image_rate_multiplier"`
}

// UpdateGroupRequest 更新分组请求
type UpdateGroupRequest struct {
	Name                 *string  `json:"name"`
	Description          *string  `json:"description"`
	RateMultiplier       *float64 `json:"rate_multiplier"`
	RefundRateMultiplier *float64 `json:"refund_rate_multiplier"`
	IsExclusive          *bool    `json:"is_exclusive"`
	Status               *string  `json:"status"`
	AllowImageGeneration *bool    `json:"allow_image_generation"`
	ImageGenerationRoute *string  `json:"image_generation_route"`

	AllowVideoGeneration  *bool    `json:"allow_video_generation"`
	VideoGenerationRoute  *string  `json:"video_generation_route"`
	VideoPrice480pPerSec  *float64 `json:"video_price_480p_per_sec"`
	VideoPrice720pPerSec  *float64 `json:"video_price_720p_per_sec"`
	VideoPrice1080pPerSec *float64 `json:"video_price_1080p_per_sec"`
	VideoPrice4kPerSec    *float64 `json:"video_price_4k_per_sec"`
	OpenAIImageMainModel  *string  `json:"openai_image_main_model"`
	ImageRateIndependent  *bool    `json:"image_rate_independent"`
	ImageRateMultiplier   *float64 `json:"image_rate_multiplier"`
}

// GroupService 分组管理服务
type GroupService struct {
	groupRepo            GroupRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
}

// NewGroupService 创建分组服务实例
func NewGroupService(groupRepo GroupRepository, authCacheInvalidator APIKeyAuthCacheInvalidator) *GroupService {
	return &GroupService{
		groupRepo:            groupRepo,
		authCacheInvalidator: authCacheInvalidator,
	}
}

// Create 创建分组
func (s *GroupService) Create(ctx context.Context, req CreateGroupRequest) (*Group, error) {
	imageRateMultiplier := 1.0
	if req.ImageRateMultiplier != nil {
		if *req.ImageRateMultiplier < 0 {
			return nil, fmt.Errorf("image_rate_multiplier must be >= 0")
		}
		imageRateMultiplier = *req.ImageRateMultiplier
	}
	openAIImageMainModel := NormalizeOpenAIImageMainModel(req.OpenAIImageMainModel)
	if err := ValidateOpenAIImageMainModel(openAIImageMainModel); err != nil {
		return nil, err
	}
	videoPrice480p := normalizePrice(req.VideoPrice480pPerSec)
	videoPrice720p := normalizePrice(req.VideoPrice720pPerSec)
	videoPrice1080p := normalizePrice(req.VideoPrice1080pPerSec)
	videoPrice4k := normalizePrice(req.VideoPrice4kPerSec)
	// 检查名称是否已存在
	exists, err := s.groupRepo.ExistsByName(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("check group exists: %w", err)
	}
	if exists {
		return nil, ErrGroupExists
	}

	// 创建分组
	group := &Group{
		Name:                 req.Name,
		Description:          req.Description,
		Platform:             PlatformAnthropic,
		RateMultiplier:       req.RateMultiplier,
		RefundRateMultiplier: normalizeRefundRateMultiplier(req.RefundRateMultiplier),
		IsExclusive:          req.IsExclusive,
		Status:               StatusActive,
		SubscriptionType:     SubscriptionTypeStandard,
		AllowImageGeneration: req.AllowImageGeneration,
		ImageGenerationRoute: NormalizeGroupImageGenerationRoute(req.ImageGenerationRoute),

		AllowVideoGeneration:  req.AllowVideoGeneration,
		VideoGenerationRoute:  NormalizeGroupVideoGenerationRoute(req.VideoGenerationRoute),
		VideoPrice480pPerSec:  videoPrice480p,
		VideoPrice720pPerSec:  videoPrice720p,
		VideoPrice1080pPerSec: videoPrice1080p,
		VideoPrice4kPerSec:    videoPrice4k,
		OpenAIImageMainModel:  openAIImageMainModel,
		ImageRateIndependent:  req.ImageRateIndependent,
		ImageRateMultiplier:   imageRateMultiplier,
	}
	sanitizeGroupVideoGenerationFields(group)

	if err := s.groupRepo.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	return group, nil
}

// GetByID 根据ID获取分组
func (s *GroupService) GetByID(ctx context.Context, id int64) (*Group, error) {
	group, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	return group, nil
}

// List 获取分组列表
func (s *GroupService) List(ctx context.Context, params pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	groups, pagination, err := s.groupRepo.List(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list groups: %w", err)
	}
	return groups, pagination, nil
}

// ListActive 获取活跃分组列表
func (s *GroupService) ListActive(ctx context.Context) ([]Group, error) {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}
	return groups, nil
}

// Update 更新分组
func (s *GroupService) Update(ctx context.Context, id int64, req UpdateGroupRequest) (*Group, error) {
	group, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}

	// 更新字段
	if req.Name != nil && *req.Name != group.Name {
		// 检查新名称是否已存在
		exists, err := s.groupRepo.ExistsByName(ctx, *req.Name)
		if err != nil {
			return nil, fmt.Errorf("check group exists: %w", err)
		}
		if exists {
			return nil, ErrGroupExists
		}
		group.Name = *req.Name
	}

	if req.Description != nil {
		group.Description = *req.Description
	}

	if req.RateMultiplier != nil {
		group.RateMultiplier = *req.RateMultiplier
	}
	if req.RefundRateMultiplier != nil {
		group.RefundRateMultiplier = normalizeRefundRateMultiplier(*req.RefundRateMultiplier)
	}

	if req.IsExclusive != nil {
		group.IsExclusive = *req.IsExclusive
	}

	if req.Status != nil {
		group.Status = *req.Status
	}
	if req.AllowImageGeneration != nil {
		group.AllowImageGeneration = *req.AllowImageGeneration
	}
	if req.ImageGenerationRoute != nil {
		group.ImageGenerationRoute = NormalizeGroupImageGenerationRoute(*req.ImageGenerationRoute)
	}
	if req.AllowVideoGeneration != nil {
		group.AllowVideoGeneration = *req.AllowVideoGeneration
	}
	if req.VideoGenerationRoute != nil {
		group.VideoGenerationRoute = NormalizeGroupVideoGenerationRoute(*req.VideoGenerationRoute)
	}
	if req.VideoPrice480pPerSec != nil {
		group.VideoPrice480pPerSec = normalizePrice(req.VideoPrice480pPerSec)
	}
	if req.VideoPrice720pPerSec != nil {
		group.VideoPrice720pPerSec = normalizePrice(req.VideoPrice720pPerSec)
	}
	if req.VideoPrice1080pPerSec != nil {
		group.VideoPrice1080pPerSec = normalizePrice(req.VideoPrice1080pPerSec)
	}
	if req.VideoPrice4kPerSec != nil {
		group.VideoPrice4kPerSec = normalizePrice(req.VideoPrice4kPerSec)
	}
	if req.OpenAIImageMainModel != nil {
		model := NormalizeOpenAIImageMainModel(*req.OpenAIImageMainModel)
		if err := ValidateOpenAIImageMainModel(model); err != nil {
			return nil, err
		}
		group.OpenAIImageMainModel = model
	}
	if req.ImageRateIndependent != nil {
		group.ImageRateIndependent = *req.ImageRateIndependent
	}
	if req.ImageRateMultiplier != nil {
		if *req.ImageRateMultiplier < 0 {
			return nil, fmt.Errorf("image_rate_multiplier must be >= 0")
		}
		group.ImageRateMultiplier = *req.ImageRateMultiplier
	}
	sanitizeGroupVideoGenerationFields(group)

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, fmt.Errorf("update group: %w", err)
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, id)
	}

	return group, nil
}

// Delete 删除分组
func (s *GroupService) Delete(ctx context.Context, id int64) error {
	// 检查分组是否存在
	_, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get group: %w", err)
	}

	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, id)
	}
	if err := s.groupRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	return nil
}

// GetStats 获取分组统计信息
func (s *GroupService) GetStats(ctx context.Context, id int64) (map[string]any, error) {
	group, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}

	// 获取账号数量
	accountCount, _, err := s.groupRepo.GetAccountCount(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get account count: %w", err)
	}

	stats := map[string]any{
		"id":              group.ID,
		"name":            group.Name,
		"rate_multiplier": group.RateMultiplier,
		"is_exclusive":    group.IsExclusive,
		"status":          group.Status,
		"account_count":   accountCount,
	}

	return stats, nil
}
