package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type OpenAIMessagesDispatchModelConfig = domain.OpenAIMessagesDispatchModelConfig
type GroupModelsListConfig = domain.GroupModelsListConfig

const (
	GroupImageGenerationRouteCodex   = "codex"
	GroupImageGenerationRouteWeb2API = "web2api"
	GroupImageGenerationRouteNative  = "native"

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
	// 高峰时段倍率：peak_rate_enabled 为 true 且当前时刻处于 [PeakStart, PeakEnd) 时，
	// token 计费倍率额外乘以 PeakRateMultiplier。详见 PeakMultiplierAt。
	PeakRateEnabled    bool
	PeakStart          string
	PeakEnd            string
	PeakRateMultiplier float64
	IsExclusive        bool
	UserSelectable     bool
	Status             string
	Hydrated           bool // indicates the group was loaded from a trusted repository source

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

	// 视频生成计费配置（仅 Grok/xAI native videos）
	AllowVideoGeneration  bool
	VideoGenerationRoute  string
	VideoPrice480pPerSec  *float64
	VideoPrice720pPerSec  *float64
	VideoPrice1080pPerSec *float64
	VideoPrice4kPerSec    *float64

	// 搜索/工具 & 音频显式定价（分组级，不按文本 RateMultiplier，参考 OpenAI 图片定价）
	SearchPricePer1k             *float64
	AudioRealtimePricePerMin     *float64
	AudioTTSPricePerMillionChars *float64
	AudioSTTPricePerHour         *float64

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
	case GroupImageGenerationRouteNative:
		return GroupImageGenerationRouteNative
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

func groupPlatformSupportsVideoGeneration(platform string) bool {
	return strings.EqualFold(strings.TrimSpace(platform), PlatformGrok)
}

func SanitizeGroupVideoGenerationFields(group *Group) {
	if group == nil {
		return
	}
	if !groupPlatformSupportsVideoGeneration(group.Platform) {
		group.AllowVideoGeneration = false
		group.VideoGenerationRoute = GroupVideoGenerationRouteNative
		group.VideoPrice480pPerSec = nil
		group.VideoPrice720pPerSec = nil
		group.VideoPrice1080pPerSec = nil
		group.VideoPrice4kPerSec = nil
		return
	}
	group.VideoGenerationRoute = NormalizeGroupVideoGenerationRoute(group.VideoGenerationRoute)
}

func sanitizeGroupVideoGenerationFields(group *Group) {
	SanitizeGroupVideoGenerationFields(group)
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

// GetSearchPricePer1k returns explicit search/tool price per 1k calls if configured on group.
func (g *Group) GetSearchPricePer1k() *float64 {
	if g == nil {
		return nil
	}
	return g.SearchPricePer1k
}

type AudioPriceConfig struct {
	RealtimePerMin *float64
	TTSPerMChars   *float64
	STTPerHour     *float64
}

func (g *Group) GetAudioPriceConfig() *AudioPriceConfig {
	if g == nil {
		return nil
	}
	return &AudioPriceConfig{
		RealtimePerMin: g.AudioRealtimePricePerMin,
		TTSPerMChars:   g.AudioTTSPricePerMillionChars,
		STTPerHour:     g.AudioSTTPricePerHour,
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

// parseMinutes 把 "HH:MM" 解析为当日分钟数（0..1439），格式非法返回 (0,false)。
// 手工解析而非 time.Parse：本函数位于每请求的计费热路径（PeakMultiplierAt），
// 避免对静态配置字符串重复走 layout 解析与 time.Time 分配。
// 接受集与 time.Parse("15:04", s) 完全一致（存量数据按旧解析写入，不得收窄）：
// 小时 1–2 位数字（0..23，允许不补零如 "1:30"），分钟固定 2 位数字（00..59）。
func parseMinutes(hhmm string) (int, bool) {
	colon := strings.IndexByte(hhmm, ':')
	if (colon != 1 && colon != 2) || len(hhmm)-colon-1 != 2 {
		return 0, false
	}
	h := 0
	for i := 0; i < colon; i++ {
		d := hhmm[i] - '0'
		if d > 9 {
			return 0, false
		}
		h = h*10 + int(d)
	}
	m1, m2 := hhmm[colon+1]-'0', hhmm[colon+2]-'0'
	if m1 > 9 || m2 > 9 {
		return 0, false
	}
	m := int(m1)*10 + int(m2)
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// PeakMultiplierAt 返回指定时刻 now 的高峰因子。
//   - 未启用 / 未配置 / 配置非法（start>=end 或格式错误） / 非高峰时段 → 返回 1.0（安全降级）
//   - 区间为左闭右开 [PeakStart, PeakEnd)，仅支持当日区间，不支持跨天（如 22:00-次日02:00）
//   - 时刻基于全局系统时区（timezone.Location）判定
//
// 该方法是纯函数，不读取任何外部状态，便于单测。
func (g *Group) PeakMultiplierAt(now time.Time) float64 {
	if g == nil || !g.IsSubscriptionType() || !g.PeakRateEnabled || g.PeakStart == "" || g.PeakEnd == "" {
		return 1.0
	}
	start, ok1 := parseMinutes(g.PeakStart)
	end, ok2 := parseMinutes(g.PeakEnd)
	if !ok1 || !ok2 || start >= end {
		return 1.0
	}
	t := now.In(timezone.Location())
	cur := t.Hour()*60 + t.Minute()
	if cur >= start && cur < end {
		return g.PeakRateMultiplier
	}
	return 1.0
}

// ValidatePeakRateConfig 是高峰倍率配置的唯一校验来源，供 handler 与 service 层共用。
// enabled=true 时仅允许订阅类型分组；并要求 start/end 合法且 end>start（不支持跨天），multiplier>=0。
// multiplier=0 是允许的，表示高峰 token 请求按 0 倍计费，可用于折扣/免费策略。
// enabled=false 时放行（不关心类型）。subscriptionType 为空按 standard 处理。
func ValidatePeakRateConfig(subscriptionType string, enabled bool, start, end string, multiplier float64) error {
	if !enabled {
		return nil
	}
	if subscriptionType != SubscriptionTypeSubscription {
		return errors.New("高峰时段倍率仅支持订阅类型分组")
	}
	if start == "" || end == "" {
		return errors.New("peak_rate_enabled 为 true 时 peak_start 与 peak_end 必填")
	}
	st, okStart := parseMinutes(start)
	if !okStart {
		return fmt.Errorf("peak_start 格式应为 HH:MM，got %q", start)
	}
	en, okEnd := parseMinutes(end)
	if !okEnd {
		return fmt.Errorf("peak_end 格式应为 HH:MM，got %q", end)
	}
	if st >= en {
		return errors.New("peak_end 必须大于 peak_start（不支持跨天区间，如 22:00-02:00）")
	}
	if multiplier < 0 {
		return errors.New("peak_rate_multiplier 不能为负")
	}
	return nil
}

// NormalizePeakRateConfig 归一化最终落库的高峰配置，CreateGroup 与 UpdateGroup 两条写路径共用（唯一收口）：
//   - 非订阅类型分组不携带任何高峰配置，一律清空（enabled=false、窗口置空、倍率归 1.0）；
//   - 订阅分组关闭高峰时保留已配置的合法窗口（便于临时停用后再启用），
//     但清掉无法解析的脏字符串与负倍率，避免脏数据入库。
//
// 与 ValidatePeakRateConfig 的分工：enabled=true 时校验已保证各字段合法，本函数为无操作；
// enabled=false 时校验放行，由本函数兜底清洗。调用顺序为先归一化、后校验，
// 使"订阅转标准"这类更新能静默清空高峰配置而不是被校验拒绝。
func NormalizePeakRateConfig(subscriptionType string, enabled bool, start, end string, multiplier float64) (bool, string, string, float64) {
	if subscriptionType != SubscriptionTypeSubscription {
		return false, "", "", 1.0
	}
	if !enabled {
		if _, ok := parseMinutes(start); !ok {
			start = ""
		}
		if _, ok := parseMinutes(end); !ok {
			end = ""
		}
		if multiplier < 0 {
			multiplier = 1.0
		}
	}
	return enabled, start, end, multiplier
}

// computePeakAwareMultipliers 把"基础 token 倍率 base"（已含系统/分组/用户级倍率，但不含高峰）
// 拆分为最终 token 倍率与图片按次倍率：图片按次倍率基于 base 现算、不受高峰影响；token 倍率在 base 上叠加高峰因子。
// gateway_service.recordUsageCore 与 openai_gateway_service.RecordUsage 共用此函数，
// 锁死"高峰因子只乘入 token 倍率、图片按次倍率不受影响"这一叠加顺序——任何调换都会被 group_peak_rate_test 覆盖。
func computePeakAwareMultipliers(apiKey *APIKey, base float64, now time.Time) (text, image float64) {
	image = resolveImageRateMultiplier(apiKey, base)
	peak := 1.0
	if apiKey != nil && apiKey.Group != nil {
		peak = apiKey.Group.PeakMultiplierAt(now)
	}
	text = base * peak
	return
}
