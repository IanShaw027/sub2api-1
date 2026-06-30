package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GroupHandler handles admin group management
type GroupHandler struct {
	adminService         service.AdminService
	dashboardService     *service.DashboardService
	groupCapacityService *service.GroupCapacityService
}

type optionalLimitField struct {
	set   bool
	value *float64
}

func (f *optionalLimitField) UnmarshalJSON(data []byte) error {
	f.set = true

	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		f.value = nil
		return nil
	}

	var number float64
	if err := json.Unmarshal(trimmed, &number); err == nil {
		f.value = &number
		return nil
	}

	var text string
	if err := json.Unmarshal(trimmed, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			f.value = nil
			return nil
		}
		number, err = strconv.ParseFloat(text, 64)
		if err != nil {
			return fmt.Errorf("invalid numeric limit value %q: %w", text, err)
		}
		f.value = &number
		return nil
	}

	return fmt.Errorf("invalid limit value: %s", string(trimmed))
}

func (f optionalLimitField) ToServiceInput() *float64 {
	if !f.set {
		return nil
	}
	if f.value != nil {
		return f.value
	}
	zero := 0.0
	return &zero
}

type optionalFloatField struct {
	set   bool
	value *float64
}

func (f *optionalFloatField) UnmarshalJSON(data []byte) error {
	f.set = true

	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		f.value = nil
		return nil
	}

	var number float64
	if err := json.Unmarshal(trimmed, &number); err != nil {
		return err
	}
	f.value = &number
	return nil
}

func (f optionalFloatField) Set() bool {
	return f.set
}

func (f optionalFloatField) Value() *float64 {
	if !f.set {
		return nil
	}
	return f.value
}

// NewGroupHandler creates a new admin group handler
func NewGroupHandler(adminService service.AdminService, dashboardService *service.DashboardService, groupCapacityService *service.GroupCapacityService) *GroupHandler {
	return &GroupHandler{
		adminService:         adminService,
		dashboardService:     dashboardService,
		groupCapacityService: groupCapacityService,
	}
}

// CreateGroupRequest represents create group request
type CreateGroupRequest struct {
	Name                 string             `json:"name" binding:"required"`
	DisplayName          *string            `json:"display_name"`
	Description          string             `json:"description"`
	Platform             string             `json:"platform" binding:"omitempty,oneof=anthropic openai gemini antigravity sora kiro grok"`
	RateMultiplier       float64            `json:"rate_multiplier"`
	RefundRateMultiplier float64            `json:"refund_rate_multiplier"`
	IsExclusive          bool               `json:"is_exclusive"`
	UserSelectable       *bool              `json:"user_selectable"`
	SubscriptionType     string             `json:"subscription_type" binding:"omitempty,oneof=standard subscription"`
	DailyLimitUSD        optionalLimitField `json:"daily_limit_usd"`
	WeeklyLimitUSD       optionalLimitField `json:"weekly_limit_usd"`
	MonthlyLimitUSD      optionalLimitField `json:"monthly_limit_usd"`
	// 图片生成计费配置（antigravity 和 gemini 平台使用，负数表示清除配置）
	AllowImageGeneration  bool     `json:"allow_image_generation"`
	ImageGenerationRoute  string   `json:"image_generation_route"`
	OpenAIImageMainModel  string   `json:"openai_image_main_model"`
	ImageRateIndependent  bool     `json:"image_rate_independent"`
	ImageRateMultiplier   *float64 `json:"image_rate_multiplier"`
	ImagePrice1K          *float64 `json:"image_price_1k"`
	ImagePrice2K          *float64 `json:"image_price_2k"`
	ImagePrice4K          *float64 `json:"image_price_4k"`
	Images2APIPrice1K     *float64 `json:"images2api_price_1k"`
	Images2APIPrice2K     *float64 `json:"images2api_price_2k"`
	Images2APIPrice4K     *float64 `json:"images2api_price_4k"`
	AllowVideoGeneration  bool     `json:"allow_video_generation"`
	VideoGenerationRoute  string   `json:"video_generation_route"`
	VideoPrice480pPerSec  *float64 `json:"video_price_480p_per_sec"`
	VideoPrice720pPerSec  *float64 `json:"video_price_720p_per_sec"`
	VideoPrice1080pPerSec *float64 `json:"video_price_1080p_per_sec"`
	VideoPrice4kPerSec    *float64 `json:"video_price_4k_per_sec"`

	SearchPricePer1k             *float64 `json:"search_price_per_1k"`
	AudioRealtimePricePerMin     *float64 `json:"audio_realtime_price_per_min"`
	AudioTtsPricePerMillionChars *float64 `json:"audio_tts_price_per_million_chars"`
	AudioSttPricePerHour         *float64 `json:"audio_stt_price_per_hour"`

	ClaudeCodeOnly                  bool   `json:"claude_code_only"`
	FallbackGroupID                 *int64 `json:"fallback_group_id"`
	FallbackGroupIDOnInvalidRequest *int64 `json:"fallback_group_id_on_invalid_request"`
	// 模型路由配置（仅 anthropic 平台使用）
	ModelRouting        map[string][]int64 `json:"model_routing"`
	ModelRoutingEnabled bool               `json:"model_routing_enabled"`
	MCPXMLInject        *bool              `json:"mcp_xml_inject"`
	// 支持的模型系列（仅 antigravity 平台使用）
	SupportedModelScopes []string `json:"supported_model_scopes"`
	// OpenAI Messages 调度配置（仅 openai 平台使用）
	AllowMessagesDispatch       bool                                      `json:"allow_messages_dispatch"`
	RequireOAuthOnly            bool                                      `json:"require_oauth_only"`
	RequirePrivacySet           bool                                      `json:"require_privacy_set"`
	DefaultMappedModel          string                                    `json:"default_mapped_model"`
	MessagesDispatchModelConfig service.OpenAIMessagesDispatchModelConfig `json:"messages_dispatch_model_config"`
	ModelsListConfig            service.GroupModelsListConfig             `json:"models_list_config"`
	// 分组 RPM 上限（0 = 不限制）
	RPMLimit int `json:"rpm_limit"`
	// 从指定分组复制账号（创建后自动绑定）
	CopyAccountsFromGroupIDs []int64 `json:"copy_accounts_from_group_ids"`
}

// UpdateGroupRequest represents update group request
type UpdateGroupRequest struct {
	Name                 string             `json:"name"`
	DisplayName          *string            `json:"display_name"`
	Description          *string            `json:"description"`
	Platform             string             `json:"platform" binding:"omitempty,oneof=anthropic openai gemini antigravity sora kiro grok"`
	RateMultiplier       *float64           `json:"rate_multiplier"`
	RefundRateMultiplier *float64           `json:"refund_rate_multiplier"`
	IsExclusive          *bool              `json:"is_exclusive"`
	UserSelectable       *bool              `json:"user_selectable"`
	Status               string             `json:"status" binding:"omitempty,oneof=active inactive"`
	SubscriptionType     string             `json:"subscription_type" binding:"omitempty,oneof=standard subscription"`
	DailyLimitUSD        optionalLimitField `json:"daily_limit_usd"`
	WeeklyLimitUSD       optionalLimitField `json:"weekly_limit_usd"`
	MonthlyLimitUSD      optionalLimitField `json:"monthly_limit_usd"`
	// 图片生成计费配置（antigravity 和 gemini 平台使用，负数表示清除配置）
	AllowImageGeneration  *bool              `json:"allow_image_generation"`
	ImageGenerationRoute  *string            `json:"image_generation_route"`
	OpenAIImageMainModel  *string            `json:"openai_image_main_model"`
	ImageRateIndependent  *bool              `json:"image_rate_independent"`
	ImageRateMultiplier   *float64           `json:"image_rate_multiplier"`
	ImagePrice1K          *float64           `json:"image_price_1k"`
	ImagePrice2K          *float64           `json:"image_price_2k"`
	ImagePrice4K          *float64           `json:"image_price_4k"`
	Images2APIPrice1K     *float64           `json:"images2api_price_1k"`
	Images2APIPrice2K     *float64           `json:"images2api_price_2k"`
	Images2APIPrice4K     *float64           `json:"images2api_price_4k"`
	AllowVideoGeneration  *bool              `json:"allow_video_generation"`
	VideoGenerationRoute  *string            `json:"video_generation_route"`
	VideoPrice480pPerSec  optionalFloatField `json:"video_price_480p_per_sec"`
	VideoPrice720pPerSec  optionalFloatField `json:"video_price_720p_per_sec"`
	VideoPrice1080pPerSec optionalFloatField `json:"video_price_1080p_per_sec"`
	VideoPrice4kPerSec    optionalFloatField `json:"video_price_4k_per_sec"`

	// 搜索与音频显式定价（Grok 图片/音频/search 支持，不按文本倍率）
	SearchPricePer1k             optionalFloatField `json:"search_price_per_1k"`
	AudioRealtimePricePerMin     optionalFloatField `json:"audio_realtime_price_per_min"`
	AudioTtsPricePerMillionChars optionalFloatField `json:"audio_tts_price_per_million_chars"`
	AudioSttPricePerHour         optionalFloatField `json:"audio_stt_price_per_hour"`

	ClaudeCodeOnly                  *bool  `json:"claude_code_only"`
	FallbackGroupID                 *int64 `json:"fallback_group_id"`
	FallbackGroupIDOnInvalidRequest *int64 `json:"fallback_group_id_on_invalid_request"`
	// 模型路由配置（仅 anthropic 平台使用）
	ModelRouting        map[string][]int64 `json:"model_routing"`
	ModelRoutingEnabled *bool              `json:"model_routing_enabled"`
	MCPXMLInject        *bool              `json:"mcp_xml_inject"`
	// 支持的模型系列（仅 antigravity 平台使用）
	SupportedModelScopes *[]string `json:"supported_model_scopes"`
	// OpenAI Messages 调度配置（仅 openai 平台使用）
	AllowMessagesDispatch       *bool                                      `json:"allow_messages_dispatch"`
	RequireOAuthOnly            *bool                                      `json:"require_oauth_only"`
	RequirePrivacySet           *bool                                      `json:"require_privacy_set"`
	DefaultMappedModel          *string                                    `json:"default_mapped_model"`
	MessagesDispatchModelConfig *service.OpenAIMessagesDispatchModelConfig `json:"messages_dispatch_model_config"`
	ModelsListConfig            *service.GroupModelsListConfig             `json:"models_list_config"`
	// 分组 RPM 上限（0 = 不限制）；nil 表示未提供不改动
	RPMLimit *int `json:"rpm_limit"`
	// 从指定分组复制账号（同步操作：先清空当前分组的账号绑定，再绑定源分组的账号）
	CopyAccountsFromGroupIDs []int64 `json:"copy_accounts_from_group_ids"`
}

// List handles listing all groups with pagination
// GET /api/v1/admin/groups
func (h *GroupHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	platform := c.Query("platform")
	status := c.Query("status")
	search := c.Query("search")
	// 标准化和验证 search 参数
	search = strings.TrimSpace(search)
	if len(search) > 100 {
		search = search[:100]
	}
	isExclusiveStr := c.Query("is_exclusive")
	sortBy := c.DefaultQuery("sort_by", "sort_order")
	sortOrder := c.DefaultQuery("sort_order", "asc")

	var isExclusive *bool
	if isExclusiveStr != "" {
		val := isExclusiveStr == "true"
		isExclusive = &val
	}

	groups, total, err := h.adminService.ListGroups(c.Request.Context(), page, pageSize, platform, status, search, isExclusive, sortBy, sortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outGroups := make([]dto.AdminGroup, 0, len(groups))
	for i := range groups {
		outGroups = append(outGroups, *dto.GroupFromServiceAdmin(&groups[i]))
	}
	response.Paginated(c, outGroups, total, page, pageSize)
}

// GetAll handles getting all active groups without pagination.
// Pass ?include_inactive=true to also include disabled groups (used by the
// API Key group filter, which needs to surface groups that still have API keys
// bound to them even after the group is disabled).
// GET /api/v1/admin/groups/all
func (h *GroupHandler) GetAll(c *gin.Context) {
	platform := c.Query("platform")
	includeInactive := c.Query("include_inactive") == "true"

	var groups []service.Group
	var err error

	if includeInactive {
		groups, err = h.adminService.GetAllGroupsIncludingInactive(c.Request.Context())
	} else if platform != "" {
		groups, err = h.adminService.GetAllGroupsByPlatform(c.Request.Context(), platform)
	} else {
		groups, err = h.adminService.GetAllGroups(c.Request.Context())
	}

	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outGroups := make([]dto.AdminGroup, 0, len(groups))
	for i := range groups {
		outGroups = append(outGroups, *dto.GroupFromServiceAdmin(&groups[i]))
	}
	response.Success(c, outGroups)
}

// GetByID handles getting a group by ID
// GET /api/v1/admin/groups/:id
func (h *GroupHandler) GetByID(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	group, err := h.adminService.GetGroup(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.GroupFromServiceAdmin(group))
}

// GetModelsListCandidates handles getting candidate model IDs for custom /v1/models list.
// GET /api/v1/admin/groups/:id/models-list-candidates
func (h *GroupHandler) GetModelsListCandidates(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID < 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	models, err := h.adminService.GetGroupModelsListCandidates(
		c.Request.Context(),
		groupID,
		c.Query("platform"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"models": models})
}

// Create handles creating a new group
// POST /api/v1/admin/groups
func (h *GroupHandler) Create(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	group, err := h.adminService.CreateGroup(c.Request.Context(), &service.CreateGroupInput{
		Name:                  req.Name,
		DisplayName:           req.DisplayName,
		Description:           req.Description,
		Platform:              req.Platform,
		RateMultiplier:        req.RateMultiplier,
		RefundRateMultiplier:  req.RefundRateMultiplier,
		IsExclusive:           req.IsExclusive,
		UserSelectable:        req.UserSelectable,
		SubscriptionType:      req.SubscriptionType,
		DailyLimitUSD:         req.DailyLimitUSD.ToServiceInput(),
		WeeklyLimitUSD:        req.WeeklyLimitUSD.ToServiceInput(),
		MonthlyLimitUSD:       req.MonthlyLimitUSD.ToServiceInput(),
		AllowImageGeneration:  req.AllowImageGeneration,
		ImageGenerationRoute:  req.ImageGenerationRoute,
		OpenAIImageMainModel:  req.OpenAIImageMainModel,
		ImageRateIndependent:  req.ImageRateIndependent,
		ImageRateMultiplier:   req.ImageRateMultiplier,
		ImagePrice1K:          req.ImagePrice1K,
		ImagePrice2K:          req.ImagePrice2K,
		ImagePrice4K:          req.ImagePrice4K,
		Images2APIPrice1K:     req.Images2APIPrice1K,
		Images2APIPrice2K:     req.Images2APIPrice2K,
		Images2APIPrice4K:     req.Images2APIPrice4K,
		AllowVideoGeneration:  req.AllowVideoGeneration,
		VideoGenerationRoute:  req.VideoGenerationRoute,
		VideoPrice480pPerSec:  req.VideoPrice480pPerSec,
		VideoPrice720pPerSec:  req.VideoPrice720pPerSec,
		VideoPrice1080pPerSec: req.VideoPrice1080pPerSec,
		VideoPrice4kPerSec:    req.VideoPrice4kPerSec,

		SearchPricePer1k:             req.SearchPricePer1k,
		AudioRealtimePricePerMin:     req.AudioRealtimePricePerMin,
		AudioTtsPricePerMillionChars: req.AudioTtsPricePerMillionChars,
		AudioSttPricePerHour:         req.AudioSttPricePerHour,

		ClaudeCodeOnly:                  req.ClaudeCodeOnly,
		FallbackGroupID:                 req.FallbackGroupID,
		FallbackGroupIDOnInvalidRequest: req.FallbackGroupIDOnInvalidRequest,
		ModelRouting:                    req.ModelRouting,
		ModelRoutingEnabled:             req.ModelRoutingEnabled,
		MCPXMLInject:                    req.MCPXMLInject,
		SupportedModelScopes:            req.SupportedModelScopes,
		AllowMessagesDispatch:           req.AllowMessagesDispatch,
		RequireOAuthOnly:                req.RequireOAuthOnly,
		RequirePrivacySet:               req.RequirePrivacySet,
		DefaultMappedModel:              req.DefaultMappedModel,
		MessagesDispatchModelConfig:     req.MessagesDispatchModelConfig,
		ModelsListConfig:                req.ModelsListConfig,
		RPMLimit:                        req.RPMLimit,
		CopyAccountsFromGroupIDs:        req.CopyAccountsFromGroupIDs,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.GroupFromServiceAdmin(group))
}

// Update handles updating a group
// PUT /api/v1/admin/groups/:id
func (h *GroupHandler) Update(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	group, err := h.adminService.UpdateGroup(c.Request.Context(), groupID, &service.UpdateGroupInput{
		Name:                     req.Name,
		DisplayName:              req.DisplayName,
		Description:              req.Description,
		Platform:                 req.Platform,
		RateMultiplier:           req.RateMultiplier,
		RefundRateMultiplier:     req.RefundRateMultiplier,
		IsExclusive:              req.IsExclusive,
		UserSelectable:           req.UserSelectable,
		Status:                   req.Status,
		SubscriptionType:         req.SubscriptionType,
		DailyLimitUSD:            req.DailyLimitUSD.ToServiceInput(),
		WeeklyLimitUSD:           req.WeeklyLimitUSD.ToServiceInput(),
		MonthlyLimitUSD:          req.MonthlyLimitUSD.ToServiceInput(),
		AllowImageGeneration:     req.AllowImageGeneration,
		ImageGenerationRoute:     req.ImageGenerationRoute,
		OpenAIImageMainModel:     req.OpenAIImageMainModel,
		ImageRateIndependent:     req.ImageRateIndependent,
		ImageRateMultiplier:      req.ImageRateMultiplier,
		ImagePrice1K:             req.ImagePrice1K,
		ImagePrice2K:             req.ImagePrice2K,
		ImagePrice4K:             req.ImagePrice4K,
		Images2APIPrice1K:        req.Images2APIPrice1K,
		Images2APIPrice2K:        req.Images2APIPrice2K,
		Images2APIPrice4K:        req.Images2APIPrice4K,
		AllowVideoGeneration:     req.AllowVideoGeneration,
		VideoGenerationRoute:     req.VideoGenerationRoute,
		VideoPrice480pPerSec:     req.VideoPrice480pPerSec.Value(),
		VideoPrice480pPerSecSet:  req.VideoPrice480pPerSec.Set(),
		VideoPrice720pPerSec:     req.VideoPrice720pPerSec.Value(),
		VideoPrice720pPerSecSet:  req.VideoPrice720pPerSec.Set(),
		VideoPrice1080pPerSec:    req.VideoPrice1080pPerSec.Value(),
		VideoPrice1080pPerSecSet: req.VideoPrice1080pPerSec.Set(),
		VideoPrice4kPerSec:       req.VideoPrice4kPerSec.Value(),
		VideoPrice4kPerSecSet:    req.VideoPrice4kPerSec.Set(),

		SearchPricePer1k:                req.SearchPricePer1k.Value(),
		SearchPricePer1kSet:             req.SearchPricePer1k.Set(),
		AudioRealtimePricePerMin:        req.AudioRealtimePricePerMin.Value(),
		AudioRealtimePricePerMinSet:     req.AudioRealtimePricePerMin.Set(),
		AudioTtsPricePerMillionChars:    req.AudioTtsPricePerMillionChars.Value(),
		AudioTtsPricePerMillionCharsSet: req.AudioTtsPricePerMillionChars.Set(),
		AudioSttPricePerHour:            req.AudioSttPricePerHour.Value(),
		AudioSttPricePerHourSet:         req.AudioSttPricePerHour.Set(),

		ClaudeCodeOnly:                  req.ClaudeCodeOnly,
		FallbackGroupID:                 req.FallbackGroupID,
		FallbackGroupIDOnInvalidRequest: req.FallbackGroupIDOnInvalidRequest,
		ModelRouting:                    req.ModelRouting,
		ModelRoutingEnabled:             req.ModelRoutingEnabled,
		MCPXMLInject:                    req.MCPXMLInject,
		SupportedModelScopes:            req.SupportedModelScopes,
		AllowMessagesDispatch:           req.AllowMessagesDispatch,
		RequireOAuthOnly:                req.RequireOAuthOnly,
		RequirePrivacySet:               req.RequirePrivacySet,
		DefaultMappedModel:              req.DefaultMappedModel,
		MessagesDispatchModelConfig:     req.MessagesDispatchModelConfig,
		ModelsListConfig:                req.ModelsListConfig,
		RPMLimit:                        req.RPMLimit,
		CopyAccountsFromGroupIDs:        req.CopyAccountsFromGroupIDs,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.GroupFromServiceAdmin(group))
}

// Delete handles deleting a group
// DELETE /api/v1/admin/groups/:id
func (h *GroupHandler) Delete(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	err = h.adminService.DeleteGroup(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Group deleted successfully"})
}

// GetStats handles getting group statistics
// GET /api/v1/admin/groups/:id/stats
func (h *GroupHandler) GetStats(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	stats, err := h.adminService.GetGroupStats(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if h.dashboardService != nil {
		usageStats, err := h.dashboardService.GetGroupStatsWithFilters(c.Request.Context(), time.Time{}, time.Now(), 0, 0, 0, groupID, nil, nil, nil, "")
		if err != nil {
			response.Error(c, 500, "Failed to get group usage stats")
			return
		}
		for _, usage := range usageStats {
			if usage.GroupID == groupID {
				stats.TotalRequests += usage.Requests
				stats.TotalCost += usage.Cost
			}
		}
	}

	response.Success(c, stats)
}

// GetUsageSummary returns today's and cumulative cost for all groups.
// GET /api/v1/admin/groups/usage-summary?timezone=Asia/Shanghai
func (h *GroupHandler) GetUsageSummary(c *gin.Context) {
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	todayStart := timezone.StartOfDayInUserLocation(now, userTZ)

	groupIDs, err := parsePositiveInt64ListQuery(c.Query("ids"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var results []usagestats.GroupUsageSummary
	if len(groupIDs) > 0 {
		results, err = h.dashboardService.GetGroupUsageSummaryByIDs(c.Request.Context(), groupIDs, todayStart)
	} else {
		results, err = h.dashboardService.GetGroupUsageSummary(c.Request.Context(), todayStart)
	}
	if err != nil {
		response.Error(c, 500, "Failed to get group usage summary")
		return
	}

	response.Success(c, results)
}

func parsePositiveInt64ListQuery(raw string) ([]int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("Invalid IDs query")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GetCapacitySummary returns aggregated capacity (concurrency/sessions/RPM) for all active groups.
// GET /api/v1/admin/groups/capacity-summary
func (h *GroupHandler) GetCapacitySummary(c *gin.Context) {
	results, err := h.groupCapacityService.GetAllGroupCapacity(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "Failed to get group capacity summary")
		return
	}
	response.Success(c, results)
}

// GetGroupAPIKeys handles getting API keys in a group
// GET /api/v1/admin/groups/:id/api-keys
func (h *GroupHandler) GetGroupAPIKeys(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	page, pageSize := response.ParsePagination(c)

	keys, total, err := h.adminService.GetGroupAPIKeys(c.Request.Context(), groupID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outKeys := make([]dto.APIKey, 0, len(keys))
	for i := range keys {
		outKeys = append(outKeys, *dto.APIKeyFromService(&keys[i]))
	}
	response.Paginated(c, outKeys, total, page, pageSize)
}

// GetGroupRateMultipliers handles getting rate multipliers for users in a group
// GET /api/v1/admin/groups/:id/rate-multipliers
func (h *GroupHandler) GetGroupRateMultipliers(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	entries, err := h.adminService.GetGroupRateMultipliers(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if entries == nil {
		entries = []service.UserGroupRateEntry{}
	}
	response.Success(c, entries)
}

// ClearGroupRateMultipliers handles clearing all rate multipliers for a group
// DELETE /api/v1/admin/groups/:id/rate-multipliers
func (h *GroupHandler) ClearGroupRateMultipliers(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	if err := h.adminService.ClearGroupRateMultipliers(c.Request.Context(), groupID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Rate multipliers cleared successfully"})
}

// BatchSetGroupRateMultipliersRequest represents batch set rate multipliers request
type BatchSetGroupRateMultipliersRequest struct {
	Entries []service.GroupRateMultiplierInput `json:"entries" binding:"required"`
}

// BatchSetGroupRateMultipliers handles batch setting rate multipliers for a group
// PUT /api/v1/admin/groups/:id/rate-multipliers
func (h *GroupHandler) BatchSetGroupRateMultipliers(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	var req BatchSetGroupRateMultipliersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.adminService.BatchSetGroupRateMultipliers(c.Request.Context(), groupID, req.Entries); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Rate multipliers updated successfully"})
}

// BatchSetGroupRPMOverridesRequest represents batch set rpm_override request
type BatchSetGroupRPMOverridesRequest struct {
	Entries []service.GroupRPMOverrideInput `json:"entries" binding:"required"`
}

// BatchSetGroupRPMOverrides handles batch setting rpm_override for users in a group
// PUT /api/v1/admin/groups/:id/rpm-overrides
func (h *GroupHandler) BatchSetGroupRPMOverrides(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	var req BatchSetGroupRPMOverridesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.adminService.BatchSetGroupRPMOverrides(c.Request.Context(), groupID, req.Entries); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "RPM overrides updated successfully"})
}

// ClearGroupRPMOverrides handles clearing all rpm_override for a group
// DELETE /api/v1/admin/groups/:id/rpm-overrides
func (h *GroupHandler) ClearGroupRPMOverrides(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	if err := h.adminService.ClearGroupRPMOverrides(c.Request.Context(), groupID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "RPM overrides cleared successfully"})
}

// UpdateSortOrderRequest represents the request to update group sort orders
type UpdateSortOrderRequest struct {
	Updates []struct {
		ID        int64 `json:"id" binding:"required"`
		SortOrder int   `json:"sort_order"`
	} `json:"updates" binding:"required,min=1"`
}

// UpdateSortOrder handles updating group sort orders
// PUT /api/v1/admin/groups/sort-order
func (h *GroupHandler) UpdateSortOrder(c *gin.Context) {
	var req UpdateSortOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updates := make([]service.GroupSortOrderUpdate, 0, len(req.Updates))
	for _, u := range req.Updates {
		updates = append(updates, service.GroupSortOrderUpdate{
			ID:        u.ID,
			SortOrder: u.SortOrder,
		})
	}

	if err := h.adminService.UpdateGroupSortOrders(c.Request.Context(), updates); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Sort order updated successfully"})
}
