package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type OpenAIMessagesDispatchModelConfig = domain.OpenAIMessagesDispatchModelConfig
type GroupModelsListConfig = domain.GroupModelsListConfig

const (
	GroupImageGenerationRouteCodex   = "codex"
	GroupImageGenerationRouteWeb2API = "web2api"

	GroupVideoGenerationRouteNative = "native"
)

type Group struct {
	ID                   int64
	Name                 string
	DisplayName          string
	Description          string
	Platform             string
	RateMultiplier       float64
	RefundRateMultiplier float64
	IsExclusive          bool
	UserSelectable       bool
	Status               string
	Hydrated             bool // indicates the group was loaded from a trusted repository source

	SubscriptionType    string
	DailyLimitUSD       *float64
	WeeklyLimitUSD      *float64
	MonthlyLimitUSD     *float64
	DefaultValidityDays int

	// 图片生成计费配置（antigravity 和 gemini 平台使用）
	AllowImageGeneration bool
	ImageGenerationRoute string
	OpenAIImageMainModel string
	ImageRateIndependent bool
	ImageRateMultiplier  float64
	ImagePrice1K         *float64
	ImagePrice2K         *float64
	ImagePrice4K         *float64
	Images2APIPrice1K    *float64
	Images2APIPrice2K    *float64
	Images2APIPrice4K    *float64

	// 视频生成计费配置（按分辨率+秒数，与其他平台统一）
	AllowVideoGeneration  bool
	VideoGenerationRoute  string
	VideoPrice480pPerSec  *float64
	VideoPrice720pPerSec  *float64
	VideoPrice1080pPerSec *float64
	VideoPrice4kPerSec    *float64

	// Claude Code 客户端限制
	ClaudeCodeOnly  bool
	FallbackGroupID *int64
	// 无效请求兜底分组（仅 anthropic 平台使用）
	FallbackGroupIDOnInvalidRequest *int64

	// 模型路由配置
	// key: 模型匹配模式（支持 * 通配符，如 "claude-opus-*"）
	// value: 优先账号 ID 列表
	ModelRouting        map[string][]int64
	ModelRoutingEnabled bool

	// MCP XML 协议注入开关（仅 antigravity 平台使用）
	MCPXMLInject bool

	// 支持的模型系列（仅 antigravity 平台使用）
	// 可选值: claude, gemini_text, gemini_image
	SupportedModelScopes []string

	// 分组排序
	SortOrder int

	// OpenAI Messages 调度配置（仅 openai 平台使用）
	AllowMessagesDispatch       bool
	RequireOAuthOnly            bool // 仅允许非 apikey 类型账号关联（OpenAI/Antigravity/Anthropic/Gemini）
	RequirePrivacySet           bool // 调度时仅允许 privacy 已成功设置的账号（OpenAI/Antigravity/Anthropic/Gemini）
	DefaultMappedModel          string
	MessagesDispatchModelConfig OpenAIMessagesDispatchModelConfig
	ModelsListConfig            GroupModelsListConfig

	// RPMLimit 分组级每分钟请求数上限（0 = 不限制）。
	// 一旦设置即接管该分组用户的限流（覆盖用户级 rpm_limit），可被 user-group rpm_override 进一步覆盖。
	RPMLimit int

	CreatedAt time.Time
	UpdatedAt time.Time

	AccountGroups           []AccountGroup
	AccountCount            int64
	ActiveAccountCount      int64
	RateLimitedAccountCount int64
}

func (g *Group) IsActive() bool {
	return g.Status == StatusActive
}

func (g *Group) IsSubscriptionType() bool {
	return g.SubscriptionType == SubscriptionTypeSubscription
}

func normalizeRefundRateMultiplier(multiplier float64) float64 {
	if multiplier <= 0 {
		return 1
	}
	return multiplier
}

func NormalizeGroupImageGenerationRoute(route string) string {
	switch strings.ToLower(strings.TrimSpace(route)) {
	case "", GroupImageGenerationRouteCodex:
		return GroupImageGenerationRouteCodex
	case GroupImageGenerationRouteWeb2API:
		return GroupImageGenerationRouteWeb2API
	default:
		return GroupImageGenerationRouteCodex
	}
}

func NormalizeGroupVideoGenerationRoute(route string) string {
	switch strings.ToLower(strings.TrimSpace(route)) {
	case "", GroupVideoGenerationRouteNative, "openai", "codex":
		return GroupVideoGenerationRouteNative
	default:
		return GroupVideoGenerationRouteNative
	}
}

func (g *Group) EffectiveVideoGenerationRoute() string {
	if g == nil {
		return GroupVideoGenerationRouteNative
	}
	return NormalizeGroupVideoGenerationRoute(g.VideoGenerationRoute)
}

func (g *Group) EffectiveImageGenerationRoute() string {
	if g == nil {
		return GroupImageGenerationRouteCodex
	}
	return NormalizeGroupImageGenerationRoute(g.ImageGenerationRoute)
}

func NormalizeOpenAIImageMainModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return openAIImagesResponsesMainModel
	}
	return trimmed
}

func ValidateOpenAIImageMainModel(model string) error {
	normalized := NormalizeOpenAIImageMainModel(model)
	if isOpenAIImageGenerationModel(normalized) {
		return fmt.Errorf("openai_image_main_model must be a Responses-capable text model, got %q", normalized)
	}
	if isCodexSparkModel(normalized) {
		return fmt.Errorf("openai_image_main_model %q does not support image_generation", normalized)
	}
	return nil
}

func (g *Group) EffectiveOpenAIImageMainModel() string {
	if g == nil {
		return NormalizeOpenAIImageMainModel("")
	}
	if g.EffectiveImageGenerationRoute() != GroupImageGenerationRouteCodex {
		return NormalizeOpenAIImageMainModel("")
	}
	return NormalizeOpenAIImageMainModel(g.OpenAIImageMainModel)
}

func (g *Group) OpenAIImageCodexEnabled() bool {
	if g == nil {
		return true
	}
	if !g.AllowImageGeneration {
		return false
	}
	return NormalizeGroupImageGenerationRoute(g.ImageGenerationRoute) == GroupImageGenerationRouteCodex
}

func (g *Group) OpenAIImageWeb2APIEnabled() bool {
	return false
}

func (g *Group) HasDailyLimit() bool {
	return g.DailyLimitUSD != nil && *g.DailyLimitUSD > 0
}

func (g *Group) HasWeeklyLimit() bool {
	return g.WeeklyLimitUSD != nil && *g.WeeklyLimitUSD > 0
}

func (g *Group) HasMonthlyLimit() bool {
	return g.MonthlyLimitUSD != nil && *g.MonthlyLimitUSD > 0
}

// GetImagePrice 根据 image_size 返回对应的图片生成价格
// 如果分组未配置价格，返回 nil（调用方应使用默认值）
func (g *Group) GetImagePrice(imageSize string) *float64 {
	return g.GetImagePriceForRequestType(imageSize, RequestTypeImage)
}

func (g *Group) GetImagePriceConfigForRequestType(requestType RequestType) *ImagePriceConfig {
	if g == nil {
		return nil
	}
	if requestType == RequestTypeImageWebBridge {
		return &ImagePriceConfig{
			Price1K: g.Images2APIPrice1K,
			Price2K: g.Images2APIPrice2K,
			Price4K: g.Images2APIPrice4K,
		}
	}
	return &ImagePriceConfig{
		Price1K: g.ImagePrice1K,
		Price2K: g.ImagePrice2K,
		Price4K: g.ImagePrice4K,
	}
}

func (g *Group) GetImagePriceForRequestType(imageSize string, requestType RequestType) *float64 {
	if requestType == RequestTypeImageWebBridge {
		switch imageSize {
		case "1K":
			return g.Images2APIPrice1K
		case "2K":
			return g.Images2APIPrice2K
		case "4K":
			return g.Images2APIPrice4K
		default:
			return g.Images2APIPrice2K
		}
	}
	switch imageSize {
	case "1K":
		return g.ImagePrice1K
	case "2K":
		return g.ImagePrice2K
	case "4K":
		return g.ImagePrice4K
	default:
		// 未知尺寸默认按 2K 计费
		return g.ImagePrice2K
	}
}

type VideoPriceConfig struct {
	Price480p  *float64
	Price720p  *float64
	Price1080p *float64
	Price4K    *float64
}

func (g *Group) GetVideoPriceConfig() *VideoPriceConfig {
	if g == nil {
		return nil
	}
	return &VideoPriceConfig{
		Price480p:  g.VideoPrice480pPerSec,
		Price720p:  g.VideoPrice720pPerSec,
		Price1080p: g.VideoPrice1080pPerSec,
		Price4K:    g.VideoPrice4kPerSec,
	}
}

func (g *Group) GetVideoPricePerSecond(videoSize string) *float64 {
	if g == nil {
		return nil
	}
	switch NormalizeVideoBillingTierOrDefault(videoSize) {
	case VideoBillingTier480p:
		return g.VideoPrice480pPerSec
	case VideoBillingTier720p:
		return g.VideoPrice720pPerSec
	case VideoBillingTier1080p:
		return g.VideoPrice1080pPerSec
	case VideoBillingTier4K:
		return g.VideoPrice4kPerSec
	default:
		return g.VideoPrice720pPerSec
	}
}

func (g *Group) DisplayLabel() string {
	if g == nil {
		return ""
	}
	if label := strings.TrimSpace(g.DisplayName); label != "" {
		return label
	}
	return strings.TrimSpace(g.Name)
}

func (g *Group) CanBeSelectedByUser() bool {
	return g != nil && g.IsActive() && g.UserSelectable
}

// IsGroupContextValid reports whether a group from context has the fields required for routing decisions.
func IsGroupContextValid(group *Group) bool {
	if group == nil {
		return false
	}
	if group.ID <= 0 {
		return false
	}
	if !group.Hydrated {
		return false
	}
	if group.Platform == "" || group.Status == "" {
		return false
	}
	return true
}

// GetRoutingAccountIDs 根据请求模型获取路由账号 ID 列表
// 返回匹配的优先账号 ID 列表，如果没有匹配规则则返回 nil
func (g *Group) GetRoutingAccountIDs(requestedModel string) []int64 {
	if !g.ModelRoutingEnabled || len(g.ModelRouting) == 0 || requestedModel == "" {
		return nil
	}

	// 1. 精确匹配优先
	if accountIDs, ok := g.ModelRouting[requestedModel]; ok && len(accountIDs) > 0 {
		return accountIDs
	}

	// 2. 通配符匹配（前缀匹配）
	for pattern, accountIDs := range g.ModelRouting {
		if matchModelPattern(pattern, requestedModel) && len(accountIDs) > 0 {
			return accountIDs
		}
	}

	return nil
}

// matchModelPattern 检查模型是否匹配模式
// 支持 * 通配符，如 "claude-opus-*" 匹配 "claude-opus-4-20250514"
func matchModelPattern(pattern, model string) bool {
	if pattern == model {
		return true
	}

	// 处理 * 通配符（仅支持末尾通配符）
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}

	return false
}
