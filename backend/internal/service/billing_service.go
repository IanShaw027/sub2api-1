package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// APIKeyRateLimitCacheData holds rate limit usage data cached in Redis.
type APIKeyRateLimitCacheData struct {
	Usage5h  float64 `json:"usage_5h"`
	Usage1d  float64 `json:"usage_1d"`
	Usage7d  float64 `json:"usage_7d"`
	Window5h int64   `json:"window_5h"` // unix timestamp, 0 = not started
	Window1d int64   `json:"window_1d"`
	Window7d int64   `json:"window_7d"`
}

// UserPlatformQuotaKey 标识一个 user×platform，用于脏集出入与批量读。
type UserPlatformQuotaKey struct {
	UserID   int64
	Platform string
}

// UserPlatformQuotaCacheEntry Redis hash 反序列化结果。
//
// SchemaVersion 用于向后兼容：
//   - 0（旧 entry，无 SchemaVersion 字段）→ 视为 cache MISS，强制 refresh
//   - 1（当前版本）→ 包含 limits 和 window_start，可免 DB 查询
//
// limit 字段为 nil 表示"无限额"（DB 中对应列为 NULL）。
const UserPlatformQuotaCacheSchemaV1 = int64(1)

type UserPlatformQuotaCacheEntry struct {
	DailyUsageUSD   float64
	WeeklyUsageUSD  float64
	MonthlyUsageUSD float64
	Version         int64
	SchemaVersion   int64

	// 以下字段仅在 SchemaVersion >= 1 时有效
	DailyLimitUSD   *float64
	WeeklyLimitUSD  *float64
	MonthlyLimitUSD *float64

	DailyWindowStart   *time.Time
	WeeklyWindowStart  *time.Time
	MonthlyWindowStart *time.Time
}

// BillingCache defines cache operations for billing service
type BillingCache interface {
	// Balance operations
	GetUserBalance(ctx context.Context, userID int64) (float64, error)
	SetUserBalance(ctx context.Context, userID int64, balance float64) error
	DeductUserBalance(ctx context.Context, userID int64, amount float64) error
	InvalidateUserBalance(ctx context.Context, userID int64) error

	// Subscription operations
	GetSubscriptionCache(ctx context.Context, userID, groupID int64) (*SubscriptionCacheData, error)
	SetSubscriptionCache(ctx context.Context, userID, groupID int64, data *SubscriptionCacheData) error
	UpdateSubscriptionUsage(ctx context.Context, userID, groupID int64, cost float64) error
	InvalidateSubscriptionCache(ctx context.Context, userID, groupID int64) error

	// API Key rate limit operations
	GetAPIKeyRateLimit(ctx context.Context, keyID int64) (*APIKeyRateLimitCacheData, error)
	SetAPIKeyRateLimit(ctx context.Context, keyID int64, data *APIKeyRateLimitCacheData) error
	UpdateAPIKeyRateLimitUsage(ctx context.Context, keyID int64, cost float64) error
	InvalidateAPIKeyRateLimit(ctx context.Context, keyID int64) error

	// user × platform quota 缓存
	GetUserPlatformQuotaCache(ctx context.Context, userID int64, platform string) (*UserPlatformQuotaCacheEntry, bool, error)
	SetUserPlatformQuotaCache(ctx context.Context, userID int64, platform string, entry *UserPlatformQuotaCacheEntry, ttl time.Duration) error
	DeleteUserPlatformQuotaCache(ctx context.Context, userID int64, platform string) error
	// IncrUserPlatformQuotaUsageCache 在缓存命中时累加用量；缓存未命中（key 不存在）静默返回 nil。
	// markDirty=true 时将该 key 的 member 写入 Redis 脏集，供 flusher 批量回写 DB。
	IncrUserPlatformQuotaUsageCache(ctx context.Context, userID int64, platform string, cost float64, ttl time.Duration, markDirty bool) error

	// 脏集读写，供 flusher 使用。
	PopDirtyUserPlatformQuotaKeys(ctx context.Context, n int) ([]UserPlatformQuotaKey, error)
	ReaddDirtyUserPlatformQuotaKeys(ctx context.Context, keys []UserPlatformQuotaKey) error
	BatchGetUserPlatformQuotaCache(ctx context.Context, keys []UserPlatformQuotaKey) ([]*UserPlatformQuotaCacheEntry, error)
}

// ModelPricing 模型价格配置（per-token价格，与LiteLLM格式一致）
type ModelPricing struct {
	InputPricePerToken                 float64 // 每token输入价格 (USD)
	InputPricePerTokenPriority         float64 // priority service tier 下每token输入价格 (USD)
	ImageInputPricePerToken            float64 // 图片输入 token 价格 (USD)，用于多模态 embedding 等图文不同价场景；为 0 时回退到 InputPricePerToken
	OutputPricePerToken                float64 // 每token输出价格 (USD)
	OutputPricePerTokenPriority        float64 // priority service tier 下每token输出价格 (USD)
	CacheCreationPricePerToken         float64 // 缓存创建每token价格 (USD)
	CacheCreationPricePerTokenPriority float64 // priority service tier 下缓存创建每token价格 (USD)
	CacheCreationPriceExplicit         bool    // 是否由渠道/区间定价显式设定（为 true 时即使 == 0 也不回退）
	CacheReadPricePerToken             float64 // 缓存读取每token价格 (USD)
	CacheReadPricePerTokenPriority     float64 // priority service tier 下缓存读取每token价格 (USD)
	CacheCreation5mPrice               float64 // 5分钟缓存创建每token价格 (USD)
	CacheCreation1hPrice               float64 // 1小时缓存创建每token价格 (USD)
	SupportsCacheBreakdown             bool    // 是否支持详细的缓存分类
	LongContextInputThreshold          int     // 超过阈值后按整次会话提升输入价格
	LongContextInputMultiplier         float64 // 长上下文整次会话输入倍率
	LongContextOutputMultiplier        float64 // 长上下文整次会话输出倍率
	ImageOutputPricePerToken           float64 // 图片输出 token 价格 (USD)
	ImageOutputPriceExplicit           bool    // 是否由渠道定价显式设定（为 true 时即使 == 0 也不回退）
}

func cloneModelPricing(pricing *ModelPricing) *ModelPricing {
	if pricing == nil {
		return nil
	}
	cloned := *pricing
	return &cloned
}

const (
	openAIGPT54LongContextInputThreshold   = 272000
	openAIGPT54LongContextInputMultiplier  = 2.0
	openAIGPT54LongContextOutputMultiplier = 1.5
)

func normalizeBillingServiceTier(serviceTier string) string {
	return strings.ToLower(strings.TrimSpace(serviceTier))
}

func usePriorityServiceTierPricing(serviceTier string, pricing *ModelPricing) bool {
	if pricing == nil || normalizeBillingServiceTier(serviceTier) != "priority" {
		return false
	}
	return pricing.InputPricePerTokenPriority > 0 || pricing.OutputPricePerTokenPriority > 0 ||
		pricing.CacheCreationPricePerTokenPriority > 0 || pricing.CacheReadPricePerTokenPriority > 0
}

func clearPriorityServiceTierPricing(pricing *ModelPricing) {
	if pricing == nil {
		return
	}
	// Channel pricing expresses the normal resolved unit price. Until channels
	// grow explicit priority-tier fields, leaving/copying priority prices here
	// would suppress serviceTierCostMultiplier("priority") and under-bill fast.
	pricing.InputPricePerTokenPriority = 0
	pricing.OutputPricePerTokenPriority = 0
	pricing.CacheCreationPricePerTokenPriority = 0
	pricing.CacheReadPricePerTokenPriority = 0
}

func serviceTierCostMultiplier(serviceTier string) float64 {
	switch normalizeBillingServiceTier(serviceTier) {
	case "priority":
		return 2.0
	case "flex":
		return 0.5
	default:
		return 1.0
	}
}

// UsageTokens 使用的token数量
type UsageTokens struct {
	InputTokens           int
	ImageInputTokens      int
	OutputTokens          int
	CacheCreationTokens   int
	CacheReadTokens       int
	CacheCreation5mTokens int
	CacheCreation1hTokens int
	ImageOutputTokens     int
}

// CostBreakdown 费用明细
type CostBreakdown struct {
	InputCost         float64
	OutputCost        float64
	ImageOutputCost   float64
	CacheCreationCost float64
	CacheReadCost     float64
	TotalCost         float64
	ActualCost        float64 // 应用倍率后的实际费用
	BillingMode       string  // 计费模式（"token"/"per_request"/"image"），由 CalculateCostUnified 填充
}

// ErrModelPricingUnavailable indicates that none of the configured pricing
// sources can price the requested model.
var ErrModelPricingUnavailable = errors.New("pricing not found")

// BillingService 计费服务
type BillingService struct {
	cfg            *config.Config
	pricingService *PricingService
	fallbackPrices map[string]*ModelPricing // 硬编码回退价格

	// fallbackWarnSeen 记录已打过 fallback 警告日志的(已小写化)模型名,
	// 让 "[Billing] Using fallback pricing" 每个模型每进程最多打一条,
	// 避免热路径上每请求刷屏(issue #3394)。零值即可用,无需在构造函数初始化。
	fallbackWarnSeen sync.Map
}

// NewBillingService 创建计费服务实例
func NewBillingService(cfg *config.Config, pricingService *PricingService) *BillingService {
	s := &BillingService{
		cfg:            cfg,
		pricingService: pricingService,
		fallbackPrices: make(map[string]*ModelPricing),
	}

	// 初始化硬编码回退价格（当动态价格不可用时使用）
	s.initFallbackPricing()

	return s
}

// initFallbackPricing 初始化硬编码回退价格（当动态价格不可用时使用）
// 价格单位：USD per token（与LiteLLM格式一致）
func (s *BillingService) initFallbackPricing() {
	// Claude 4.5 Opus
	s.fallbackPrices["claude-opus-4.5"] = &ModelPricing{
		InputPricePerToken:         5e-6,    // $5 per MTok
		OutputPricePerToken:        25e-6,   // $25 per MTok
		CacheCreationPricePerToken: 6.25e-6, // $6.25 per MTok
		CacheReadPricePerToken:     0.5e-6,  // $0.50 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 4 Sonnet
	s.fallbackPrices["claude-sonnet-4"] = &ModelPricing{
		InputPricePerToken:         3e-6,    // $3 per MTok
		OutputPricePerToken:        15e-6,   // $15 per MTok
		CacheCreationPricePerToken: 3.75e-6, // $3.75 per MTok
		CacheReadPricePerToken:     0.3e-6,  // $0.30 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 3.5 Sonnet
	s.fallbackPrices["claude-3-5-sonnet"] = &ModelPricing{
		InputPricePerToken:         3e-6,    // $3 per MTok
		OutputPricePerToken:        15e-6,   // $15 per MTok
		CacheCreationPricePerToken: 3.75e-6, // $3.75 per MTok
		CacheReadPricePerToken:     0.3e-6,  // $0.30 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 3.5 Haiku
	s.fallbackPrices["claude-3-5-haiku"] = &ModelPricing{
		InputPricePerToken:         1e-6,    // $1 per MTok
		OutputPricePerToken:        5e-6,    // $5 per MTok
		CacheCreationPricePerToken: 1.25e-6, // $1.25 per MTok
		CacheReadPricePerToken:     0.1e-6,  // $0.10 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 3 Opus
	s.fallbackPrices["claude-3-opus"] = &ModelPricing{
		InputPricePerToken:         15e-6,    // $15 per MTok
		OutputPricePerToken:        75e-6,    // $75 per MTok
		CacheCreationPricePerToken: 18.75e-6, // $18.75 per MTok
		CacheReadPricePerToken:     1.5e-6,   // $1.50 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 3 Haiku
	s.fallbackPrices["claude-3-haiku"] = &ModelPricing{
		InputPricePerToken:         0.25e-6, // $0.25 per MTok
		OutputPricePerToken:        1.25e-6, // $1.25 per MTok
		CacheCreationPricePerToken: 0.3e-6,  // $0.30 per MTok
		CacheReadPricePerToken:     0.03e-6, // $0.03 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 4.6 Opus (与4.5同价)
	s.fallbackPrices["claude-opus-4.6"] = s.fallbackPrices["claude-opus-4.5"]

	// Claude 4.7 Opus (暂与4.6同价，待官方定价更新)
	s.fallbackPrices["claude-opus-4.7"] = s.fallbackPrices["claude-opus-4.6"]

	// Gemini 3.1 Pro
	s.fallbackPrices["gemini-3.1-pro"] = &ModelPricing{
		InputPricePerToken:         2e-6,   // $2 per MTok
		OutputPricePerToken:        12e-6,  // $12 per MTok
		CacheCreationPricePerToken: 2e-6,   // $2 per MTok
		CacheReadPricePerToken:     0.2e-6, // $0.20 per MTok
		SupportsCacheBreakdown:     false,
	}

	// OpenAI GPT-5.4（业务指定价格）
	s.fallbackPrices["gpt-5.4"] = &ModelPricing{
		InputPricePerToken:             2.5e-6,  // $2.5 per MTok
		InputPricePerTokenPriority:     5e-6,    // $5 per MTok
		OutputPricePerToken:            15e-6,   // $15 per MTok
		OutputPricePerTokenPriority:    30e-6,   // $30 per MTok
		CacheCreationPricePerToken:     2.5e-6,  // $2.5 per MTok
		CacheReadPricePerToken:         0.25e-6, // $0.25 per MTok
		CacheReadPricePerTokenPriority: 0.5e-6,  // $0.5 per MTok
		SupportsCacheBreakdown:         false,
		LongContextInputThreshold:      openAIGPT54LongContextInputThreshold,
		LongContextInputMultiplier:     openAIGPT54LongContextInputMultiplier,
		LongContextOutputMultiplier:    openAIGPT54LongContextOutputMultiplier,
	}
	// GPT-5.5 / GPT-5.5 Pro 暂无独立定价，回退到 GPT-5.4。
	s.fallbackPrices["gpt-5.5"] = s.fallbackPrices["gpt-5.4"]
	s.fallbackPrices["gpt-5.5-pro"] = s.fallbackPrices["gpt-5.4"]
	s.fallbackPrices["gpt-5.6-sol"] = &ModelPricing{
		InputPricePerToken:                 5e-6,
		InputPricePerTokenPriority:         10e-6,
		OutputPricePerToken:                30e-6,
		OutputPricePerTokenPriority:        60e-6,
		CacheCreationPricePerToken:         6.25e-6,
		CacheCreationPricePerTokenPriority: 12.5e-6,
		CacheReadPricePerToken:             0.5e-6,
		CacheReadPricePerTokenPriority:     1e-6,
		SupportsCacheBreakdown:             false,
		LongContextInputThreshold:          openAIGPT54LongContextInputThreshold,
		LongContextInputMultiplier:         openAIGPT54LongContextInputMultiplier,
		LongContextOutputMultiplier:        openAIGPT54LongContextOutputMultiplier,
	}
	s.fallbackPrices["gpt-5.6-terra"] = &ModelPricing{
		InputPricePerToken:                 2.5e-6,
		InputPricePerTokenPriority:         5e-6,
		OutputPricePerToken:                15e-6,
		OutputPricePerTokenPriority:        30e-6,
		CacheCreationPricePerToken:         3.125e-6,
		CacheCreationPricePerTokenPriority: 6.25e-6,
		CacheReadPricePerToken:             0.25e-6,
		CacheReadPricePerTokenPriority:     0.5e-6,
		SupportsCacheBreakdown:             false,
		LongContextInputThreshold:          openAIGPT54LongContextInputThreshold,
		LongContextInputMultiplier:         openAIGPT54LongContextInputMultiplier,
		LongContextOutputMultiplier:        openAIGPT54LongContextOutputMultiplier,
	}
	s.fallbackPrices["gpt-5.6-luna"] = &ModelPricing{
		InputPricePerToken:                 1e-6,
		InputPricePerTokenPriority:         2e-6,
		OutputPricePerToken:                6e-6,
		OutputPricePerTokenPriority:        12e-6,
		CacheCreationPricePerToken:         1.25e-6,
		CacheCreationPricePerTokenPriority: 2.5e-6,
		CacheReadPricePerToken:             0.1e-6,
		CacheReadPricePerTokenPriority:     0.2e-6,
		SupportsCacheBreakdown:             false,
		LongContextInputThreshold:          openAIGPT54LongContextInputThreshold,
		LongContextInputMultiplier:         openAIGPT54LongContextInputMultiplier,
		LongContextOutputMultiplier:        openAIGPT54LongContextOutputMultiplier,
	}

	s.fallbackPrices["gpt-5.4-mini"] = &ModelPricing{
		InputPricePerToken:     7.5e-7,
		OutputPricePerToken:    4.5e-6,
		CacheReadPricePerToken: 7.5e-8,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["gpt-5.4-nano"] = &ModelPricing{
		InputPricePerToken:     2e-7,
		OutputPricePerToken:    1.25e-6,
		CacheReadPricePerToken: 2e-8,
		SupportsCacheBreakdown: false,
	}
	// OpenAI GPT-5.2（本地兜底）
	s.fallbackPrices["gpt-5.2"] = &ModelPricing{
		InputPricePerToken:             1.75e-6,
		InputPricePerTokenPriority:     3.5e-6,
		OutputPricePerToken:            14e-6,
		OutputPricePerTokenPriority:    28e-6,
		CacheCreationPricePerToken:     1.75e-6,
		CacheReadPricePerToken:         0.175e-6,
		CacheReadPricePerTokenPriority: 0.35e-6,
		SupportsCacheBreakdown:         false,
	}
	// OpenAI GPT-5.2 Codex（官方模型页与 GPT-5.2 同价，单独建键避免误映射到其它价位）
	s.fallbackPrices["gpt-5.2-codex"] = s.fallbackPrices["gpt-5.2"]
	// OpenAI GPT-5.1
	s.fallbackPrices["gpt-5.1"] = &ModelPricing{
		InputPricePerToken:             1.25e-6,
		InputPricePerTokenPriority:     2.5e-6,
		OutputPricePerToken:            10e-6,
		OutputPricePerTokenPriority:    20e-6,
		CacheCreationPricePerToken:     1.25e-6,
		CacheReadPricePerToken:         0.125e-6,
		CacheReadPricePerTokenPriority: 0.25e-6,
		SupportsCacheBreakdown:         false,
	}
	// OpenAI GPT-5.1 Codex（官方模型页与 GPT-5.1 同价）
	s.fallbackPrices["gpt-5.1-codex"] = s.fallbackPrices["gpt-5.1"]
	// OpenAI GPT-5.1 Codex Max（官方模型页与 GPT-5.1 同价）
	s.fallbackPrices["gpt-5.1-codex-max"] = s.fallbackPrices["gpt-5.1"]
	// OpenAI GPT-5.1 Codex Mini
	s.fallbackPrices["gpt-5.1-codex-mini"] = &ModelPricing{
		InputPricePerToken:             0.25e-6,
		InputPricePerTokenPriority:     0.5e-6,
		OutputPricePerToken:            2e-6,
		OutputPricePerTokenPriority:    4e-6,
		CacheCreationPricePerToken:     0.25e-6,
		CacheReadPricePerToken:         0.025e-6,
		CacheReadPricePerTokenPriority: 0.05e-6,
		SupportsCacheBreakdown:         false,
	}
	s.fallbackPrices["gpt-5.3-codex-spark"] = &ModelPricing{
		InputPricePerToken:             1.75e-6,
		InputPricePerTokenPriority:     3.5e-6,
		OutputPricePerToken:            14e-6,
		OutputPricePerTokenPriority:    28e-6,
		CacheReadPricePerToken:         0.175e-6,
		CacheReadPricePerTokenPriority: 0.35e-6,
		SupportsCacheBreakdown:         false,
	}
	// Codex 族兜底统一按 GPT-5.3 Codex 价格计费
	s.fallbackPrices["gpt-5.3-codex"] = &ModelPricing{
		InputPricePerToken:             1.5e-6, // $1.5 per MTok
		InputPricePerTokenPriority:     3e-6,   // $3 per MTok
		OutputPricePerToken:            12e-6,  // $12 per MTok
		OutputPricePerTokenPriority:    24e-6,  // $24 per MTok
		CacheCreationPricePerToken:     1.5e-6, // $1.5 per MTok
		CacheReadPricePerToken:         0.15e-6,
		CacheReadPricePerTokenPriority: 0.3e-6,
		SupportsCacheBreakdown:         false,
	}
	s.fallbackPrices["codex-auto-review"] = s.fallbackPrices["gpt-5.3-codex"]

	// ============================================================
	// 国产 LLM 兜底定价（数据源：各家官方定价页/USD 口径）
	// 顺序：DeepSeek → 智谱 GLM → 月之暗面 Kimi → MiniMax
	// 覆盖逻辑见同文件 getFallbackPricing()
	// ============================================================

	// ---- DeepSeek V4 系列 ----
	// Source: https://api-docs.deepseek.com/quick_start/pricing
	// （deepseek-chat / deepseek-reasoner 为 deepseek-v4-flash 的兼容别名，2026/07/24 弃用）
	s.fallbackPrices["deepseek-v4-pro"] = &ModelPricing{
		InputPricePerToken:     4.35e-7,  // $0.435 per MTok (cache miss)
		OutputPricePerToken:    8.7e-7,   // $0.87 per MTok
		CacheReadPricePerToken: 3.625e-9, // $0.003625 per MTok (cache hit)
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["deepseek-v4-flash"] = &ModelPricing{
		InputPricePerToken:     1.4e-7, // $0.14 per MTok (cache miss)
		OutputPricePerToken:    2.8e-7, // $0.28 per MTok
		CacheReadPricePerToken: 2.8e-9, // $0.0028 per MTok (cache hit)
		SupportsCacheBreakdown: false,
	}

	// ---- 智谱 GLM（Z.AI）----
	// Source: https://docs.z.ai/guides/overview/pricing (USD per 1M tokens)
	// 注意：CacheReadPricePerToken 即"缓存命中"价格，CacheCreationPricePerToken 留空（智谱未公开写入价，按 0 处理）。
	// GLM-4.6 与 GLM-4.5 在 z.ai 国际版上定价一致；GLM-4.5 国内按 ¥0.8/¥2，汇率换算后约 $0.112/$0.28，与国际版 $0.6/$2.2 不同，本分支采用国际版 USD 口径与现有 Claude/GPT 一致。
	s.fallbackPrices["glm-5.1"] = &ModelPricing{
		InputPricePerToken:     1.4e-6, // $1.40 per MTok
		OutputPricePerToken:    4.4e-6, // $4.40 per MTok
		CacheReadPricePerToken: 0.26e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-5"] = &ModelPricing{
		InputPricePerToken:     1e-6, // $1.00 per MTok
		OutputPricePerToken:    3.2e-6,
		CacheReadPricePerToken: 0.2e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-5-turbo"] = &ModelPricing{
		InputPricePerToken:     1.2e-6,
		OutputPricePerToken:    4e-6,
		CacheReadPricePerToken: 0.24e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.7"] = &ModelPricing{
		InputPricePerToken:     0.6e-6, // $0.60 per MTok
		OutputPricePerToken:    2.2e-6,
		CacheReadPricePerToken: 0.11e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.7-flashx"] = &ModelPricing{
		InputPricePerToken:     0.07e-6, // $0.07 per MTok
		OutputPricePerToken:    0.4e-6,
		CacheReadPricePerToken: 0.01e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.6"] = &ModelPricing{
		InputPricePerToken:     0.6e-6, // $0.60 per MTok
		OutputPricePerToken:    2.2e-6,
		CacheReadPricePerToken: 0.11e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.5"] = &ModelPricing{
		InputPricePerToken:     0.6e-6, // $0.60 per MTok
		OutputPricePerToken:    2.2e-6,
		CacheReadPricePerToken: 0.11e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.5-x"] = &ModelPricing{
		InputPricePerToken:     2.2e-6, // $2.20 per MTok
		OutputPricePerToken:    8.9e-6,
		CacheReadPricePerToken: 0.45e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.5-air"] = &ModelPricing{
		InputPricePerToken:     0.2e-6, // $0.20 per MTok
		OutputPricePerToken:    1.1e-6,
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.5-airx"] = &ModelPricing{
		InputPricePerToken:     1.1e-6,
		OutputPricePerToken:    4.5e-6,
		CacheReadPricePerToken: 0.22e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4-32b-0414-128k"] = &ModelPricing{
		InputPricePerToken:     0.1e-6, // $0.10 per MTok
		OutputPricePerToken:    0.1e-6,
		SupportsCacheBreakdown: false,
	}
	// GLM-4.5-Flash / GLM-4.7-Flash 在 z.ai 上为 Free，保留 zero-cost entry 防止未知 alias 误计费。
	s.fallbackPrices["glm-4.5-flash"] = &ModelPricing{
		InputPricePerToken:     0,
		OutputPricePerToken:    0,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.7-flash"] = &ModelPricing{
		InputPricePerToken:     0,
		OutputPricePerToken:    0,
		SupportsCacheBreakdown: false,
	}

	// ---- 月之暗面 Kimi（K 系列）----
	// Source: https://platform.moonshot.cn/docs/pricing/overview (元/百万 tokens 口径)
	//       交叉验证：https://www.tmtpost.com/7961404.html (USD 口径)
	// Moonshot V1 (¥2/¥5/¥10 多 tier) 公开页未直接标注 USD 价，本分支不覆盖，避免误计价。
	// K2-0905 / K2-0711 官方页面未保留定价，不覆盖。
	s.fallbackPrices["kimi-k2.6"] = &ModelPricing{
		InputPricePerToken:     0.95e-6, // $0.95 per MTok (cache miss)
		OutputPricePerToken:    4e-6,    // $4.00 per MTok
		CacheReadPricePerToken: 0.15e-6, // $0.15 per MTok (cache hit, ¥1.10)
		SupportsCacheBreakdown: false,
	}
	// kimi-for-coding 走 Kimi Coding endpoint，按当前 K2.6 coding 档位兜底计费。
	s.fallbackPrices["kimi-for-coding"] = &ModelPricing{
		InputPricePerToken:     0.95e-6,
		OutputPricePerToken:    4e-6,
		CacheReadPricePerToken: 0.15e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["kimi-k2.5"] = &ModelPricing{
		InputPricePerToken:     0.60e-6, // $0.60 per MTok
		OutputPricePerToken:    3e-6,    // $3.00 per MTok
		CacheReadPricePerToken: 0.098e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["kimi-k2-thinking"] = &ModelPricing{
		InputPricePerToken:     0.56e-6, // ¥4/百万 ≈ $0.56
		OutputPricePerToken:    2.24e-6, // ¥16/百万
		CacheReadPricePerToken: 0.14e-6, // ¥1/百万
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["kimi-k2"] = &ModelPricing{
		InputPricePerToken:     0.56e-6, // ¥4/百万
		OutputPricePerToken:    2.24e-6, // ¥16/百万
		CacheReadPricePerToken: 0.14e-6, // ¥1/百万
		SupportsCacheBreakdown: false,
	}

	// ---- MiniMax M 系列 ----
	// Source: https://platform.minimax.io/docs/guides/pricing-paygo
	// 注意：MiniMax M3 在 >512K context 时价格翻倍，本兜底采用 ≤512K 标准 tier（保守口径，对用户有利）。
	// 如需支持长上下文 multiplier，可后续参考 GPT-5.4 模式扩展 LongContextXxx 字段。
	s.fallbackPrices["minimax-m3"] = &ModelPricing{
		InputPricePerToken:     0.60e-6, // $0.60 per MTok (≤512K standard tier, 含 50% 永久折扣前原价 $1.20)
		OutputPricePerToken:    2.40e-6,
		CacheReadPricePerToken: 0.12e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2.7"] = &ModelPricing{
		InputPricePerToken:     0.30e-6, // $0.30 per MTok
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.06e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2.7-highspeed"] = &ModelPricing{
		InputPricePerToken:     0.60e-6,
		OutputPricePerToken:    2.40e-6,
		CacheReadPricePerToken: 0.06e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2.5"] = &ModelPricing{
		InputPricePerToken:     0.30e-6,
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2.1"] = &ModelPricing{
		InputPricePerToken:     0.30e-6,
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2"] = &ModelPricing{
		InputPricePerToken:     0.30e-6,
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}

	// ---- 火山方舟 豆包 Embedding（多模态向量化）----
	// doubao-embedding-vision 图文向量化：上游 usage 回传 prompt_tokens_details.{text_tokens,image_tokens}，
	// 按量付费官方价 文本 ¥0.7/MTok、图片 ¥1.8/MTok；汇率口径 ÷7.14（与本表其他国产模型一致，¥1≈$0.14）。
	// embedding 无 output，OutputPricePerToken 置 0。
	s.fallbackPrices["doubao-embedding-vision"] = &ModelPricing{
		InputPricePerToken:      0.098e-6, // ¥0.7/MTok ≈ $0.098（文本输入）
		ImageInputPricePerToken: 0.252e-6, // ¥1.8/MTok ≈ $0.252（图片输入）
		OutputPricePerToken:     0,
		SupportsCacheBreakdown:  false,
	}

	// xAI Grok 4.5 (per https://docs.x.ai/docs/models : $2.00/$6.00 per M tokens, cached $0.50, 500k context)
	s.fallbackPrices["grok-4.5"] = &ModelPricing{
		InputPricePerToken:         2e-6,   // $2.00 per MTok
		OutputPricePerToken:        6e-6,   // $6.00 per MTok
		CacheReadPricePerToken:     0.5e-6, // $0.50 per MTok (cached input)
		SupportsCacheBreakdown:     false,
		LongContextInputThreshold:  500000,
		LongContextInputMultiplier: 1,
	}

	// xAI Grok 4.3 / 4.20 series (per https://docs.x.ai/developers/models : $1.25/$2.50, cached $0.20)
	s.fallbackPrices["grok-4.3"] = &ModelPricing{
		InputPricePerToken:         1.25e-6, // $1.25 per MTok
		OutputPricePerToken:        2.5e-6,  // $2.50 per MTok
		CacheReadPricePerToken:     0.2e-6,  // $0.20 per MTok (cached input)
		SupportsCacheBreakdown:     false,
		LongContextInputThreshold:  1000000,
		LongContextInputMultiplier: 1,
	}
	// All 4.20 variants share the same pricing
	s.fallbackPrices["grok-4.20-0309-reasoning"] = s.fallbackPrices["grok-4.3"]
	s.fallbackPrices["grok-4.20-0309-non-reasoning"] = s.fallbackPrices["grok-4.3"]
	s.fallbackPrices["grok-4.20-multi-agent-0309"] = s.fallbackPrices["grok-4.3"]

	// xAI Grok Build 0.1 (per official: $1.00 / $2.00, cached $0.20)
	s.fallbackPrices["grok-build-0.1"] = &ModelPricing{
		InputPricePerToken:     1e-6,   // $1.00 per MTok
		OutputPricePerToken:    2e-6,   // $2.00 per MTok
		CacheReadPricePerToken: 0.2e-6, // $0.20 per MTok (cached input)
		SupportsCacheBreakdown: false,
	}
}

// getFallbackPricing 根据模型系列获取回退价格
func (s *BillingService) getFallbackPricing(model string) *ModelPricing {
	modelLower := strings.ToLower(model)

	// 按模型系列匹配
	if strings.Contains(modelLower, "opus") {
		if strings.Contains(modelLower, "4.7") || strings.Contains(modelLower, "4-7") {
			return s.fallbackPrices["claude-opus-4.7"]
		}
		if strings.Contains(modelLower, "4.6") || strings.Contains(modelLower, "4-6") {
			return s.fallbackPrices["claude-opus-4.6"]
		}
		if strings.Contains(modelLower, "4.5") || strings.Contains(modelLower, "4-5") {
			return s.fallbackPrices["claude-opus-4.5"]
		}
		return s.fallbackPrices["claude-3-opus"]
	}
	if strings.Contains(modelLower, "sonnet") {
		if strings.Contains(modelLower, "4") && !strings.Contains(modelLower, "3") {
			return s.fallbackPrices["claude-sonnet-4"]
		}
		return s.fallbackPrices["claude-3-5-sonnet"]
	}
	if strings.Contains(modelLower, "haiku") {
		if strings.Contains(modelLower, "3-5") || strings.Contains(modelLower, "3.5") {
			return s.fallbackPrices["claude-3-5-haiku"]
		}
		return s.fallbackPrices["claude-3-haiku"]
	}
	// Claude 未知型号统一回退到 Sonnet，避免计费中断。
	if strings.Contains(modelLower, "claude") {
		return s.fallbackPrices["claude-sonnet-4"]
	}
	if strings.Contains(modelLower, "gemini-3.1-pro") || strings.Contains(modelLower, "gemini-3-1-pro") {
		return s.fallbackPrices["gemini-3.1-pro"]
	}

	// DeepSeek V4 系列：仅匹配已知 V4 Pro/Flash 与官方兼容别名
	// （deepseek-chat / deepseek-reasoner → V4 Flash），未知 deepseek-* 型号不回退，避免误计价。
	if strings.Contains(modelLower, "deepseek-v4-flash") {
		return s.fallbackPrices["deepseek-v4-flash"]
	}
	if strings.Contains(modelLower, "deepseek-v4-pro") {
		return s.fallbackPrices["deepseek-v4-pro"]
	}
	if strings.Contains(modelLower, "deepseek-chat") || strings.Contains(modelLower, "deepseek-reasoner") {
		return s.fallbackPrices["deepseek-v4-flash"]
	}

	// ---- 国产 LLM 兜底匹配 ----
	// 匹配策略：长 key 优先（具体模型 → 系列 / 厂商），未知型号不回退以避免误计价。
	// 与 DeepSeek 一样采用"白名单"语义：未在本表命中的国产模型 alias 一律不返回兜底价。

	// 智谱 GLM（z.ai 公开 SKU：glm-5.1 / glm-5 / glm-5-turbo / glm-4.7 / glm-4.6 / glm-4.5 等）
	// 匹配顺序：先判别最高 tier，再依次降级。
	if strings.Contains(modelLower, "glm-5.1") {
		return s.fallbackPrices["glm-5.1"]
	}
	if strings.Contains(modelLower, "glm-5-turbo") || strings.Contains(modelLower, "glm-5turbo") {
		return s.fallbackPrices["glm-5-turbo"]
	}
	if strings.Contains(modelLower, "glm-5") {
		return s.fallbackPrices["glm-5"]
	}
	if strings.Contains(modelLower, "glm-4.7-flashx") {
		return s.fallbackPrices["glm-4.7-flashx"]
	}
	if strings.Contains(modelLower, "glm-4.7-flash") {
		return s.fallbackPrices["glm-4.7-flash"]
	}
	if strings.Contains(modelLower, "glm-4.7") {
		return s.fallbackPrices["glm-4.7"]
	}
	if strings.Contains(modelLower, "glm-4.6") {
		return s.fallbackPrices["glm-4.6"]
	}
	if strings.Contains(modelLower, "glm-4.5-flash") {
		return s.fallbackPrices["glm-4.5-flash"]
	}
	if strings.Contains(modelLower, "glm-4.5-x") || strings.Contains(modelLower, "glm-4.5x") {
		return s.fallbackPrices["glm-4.5-x"]
	}
	if strings.Contains(modelLower, "glm-4.5-airx") || strings.Contains(modelLower, "glm-4.5airx") {
		return s.fallbackPrices["glm-4.5-airx"]
	}
	if strings.Contains(modelLower, "glm-4.5-air") || strings.Contains(modelLower, "glm-4.5air") {
		return s.fallbackPrices["glm-4.5-air"]
	}
	if strings.Contains(modelLower, "glm-4.5") {
		return s.fallbackPrices["glm-4.5"]
	}
	if strings.Contains(modelLower, "glm-4-32b") {
		return s.fallbackPrices["glm-4-32b-0414-128k"]
	}

	// 月之暗面 Kimi（kimi-k2.6 / kimi-for-coding / kimi-k2.5 / kimi-k2-thinking / kimi-k2）
	// K2-0905 / K2-0711 官方未保留定价，不进入 fallback。
	if strings.Contains(modelLower, "kimi-for-coding") {
		return s.fallbackPrices["kimi-for-coding"]
	}
	if strings.Contains(modelLower, "kimi-k2.6") || strings.Contains(modelLower, "kimi-k2-6") {
		return s.fallbackPrices["kimi-k2.6"]
	}
	if strings.Contains(modelLower, "kimi-k2.5") || strings.Contains(modelLower, "kimi-k2-5") {
		return s.fallbackPrices["kimi-k2.5"]
	}
	if strings.Contains(modelLower, "kimi-k2-thinking") || strings.Contains(modelLower, "kimi-k2-thinking-") {
		return s.fallbackPrices["kimi-k2-thinking"]
	}
	if strings.Contains(modelLower, "kimi-k2") || strings.Contains(modelLower, "kimi/k2") {
		return s.fallbackPrices["kimi-k2"]
	}

	// MiniMax M 系列（M3 / M2.7 / M2.5 / M2.1 / M2；含 highspeed 变体）
	if strings.Contains(modelLower, "minimax-m3") {
		return s.fallbackPrices["minimax-m3"]
	}
	if strings.Contains(modelLower, "minimax-m2.7-highspeed") || strings.Contains(modelLower, "minimax-m2-7-highspeed") {
		return s.fallbackPrices["minimax-m2.7-highspeed"]
	}
	if strings.Contains(modelLower, "minimax-m2.7") || strings.Contains(modelLower, "minimax-m2-7") {
		return s.fallbackPrices["minimax-m2.7"]
	}
	if strings.Contains(modelLower, "minimax-m2.5") || strings.Contains(modelLower, "minimax-m2-5") {
		return s.fallbackPrices["minimax-m2.5"]
	}
	if strings.Contains(modelLower, "minimax-m2.1") || strings.Contains(modelLower, "minimax-m2-1") {
		return s.fallbackPrices["minimax-m2.1"]
	}
	if strings.Contains(modelLower, "minimax-m2") || strings.Contains(modelLower, "minimax-m-2") {
		return s.fallbackPrices["minimax-m2"]
	}

	// 火山方舟 豆包 Embedding（多模态向量化）。
	// most-specific-first：放在未来任何 doubao-embedding / doubao 宽匹配之前。
	// 覆盖带版本后缀的别名（如 doubao-embedding-vision-251215）。
	if strings.Contains(modelLower, "doubao-embedding-vision") {
		return s.fallbackPrices["doubao-embedding-vision"]
	}

	// OpenAI（GPT-5 / Codex 族）：仅匹配已知型号，避免未知 OpenAI 型号误计价。
	if normalized := normalizeOpenAIPricingFallbackModel(modelLower); normalized != "" {
		switch normalized {
		case "gpt-5.6-sol":
			return s.fallbackPrices["gpt-5.6-sol"]
		case "gpt-5.6-terra":
			return s.fallbackPrices["gpt-5.6-terra"]
		case "gpt-5.6-luna":
			return s.fallbackPrices["gpt-5.6-luna"]
		case "gpt-5.5-pro":
			return s.fallbackPrices["gpt-5.5-pro"]
		case "gpt-5.5":
			return s.fallbackPrices["gpt-5.5"]
		case "gpt-5.4-mini":
			return s.fallbackPrices["gpt-5.4-mini"]
		case "gpt-5.4-nano":
			return s.fallbackPrices["gpt-5.4-nano"]
		case "gpt-5.4":
			return s.fallbackPrices["gpt-5.4"]
		case "gpt-5.1":
			return s.fallbackPrices["gpt-5.1"]
		case "gpt-5.2":
			return s.fallbackPrices["gpt-5.2"]
		case "gpt-5.2-codex":
			return s.fallbackPrices["gpt-5.2-codex"]
		case "gpt-5.1-codex":
			return s.fallbackPrices["gpt-5.1-codex"]
		case "gpt-5.1-codex-mini":
			return s.fallbackPrices["gpt-5.1-codex-mini"]
		case "gpt-5.1-codex-max":
			return s.fallbackPrices["gpt-5.1-codex-max"]
		case "gpt-5.3-codex-spark":
			return s.fallbackPrices["gpt-5.3-codex-spark"]
		case "gpt-5.3-codex":
			return s.fallbackPrices["gpt-5.3-codex"]
		case "codex-auto-review":
			return s.fallbackPrices["codex-auto-review"]
		}
	}

	// xAI Grok text models: whitelist only known families / aliases.
	// Do NOT catch-all grok* — Imagine media models must not inherit text token rates.
	switch modelLower {
	case "grok", "grok-latest", "grok-4.5", "grok-4.5-latest":
		return s.fallbackPrices["grok-4.5"]
	case "grok-4.3", "grok-4.3-latest":
		return s.fallbackPrices["grok-4.3"]
	case "grok-build", "grok-build-0.1", "grok-code-fast", "grok-code-fast-1", "grok-code-fast-1-0825",
		"grok-composer", "grok-composer-2.5-fast", "composer-2.5":
		return s.fallbackPrices["grok-build-0.1"]
	case "grok-4.20-0309-reasoning", "grok-4.20-reasoning":
		return s.fallbackPrices["grok-4.20-0309-reasoning"]
	case "grok-4.20-0309-non-reasoning", "grok-4.20-non-reasoning":
		return s.fallbackPrices["grok-4.20-0309-non-reasoning"]
	case "grok-4.20-multi-agent-0309":
		return s.fallbackPrices["grok-4.20-multi-agent-0309"]
	}

	// Grok 4.5 family (including future dated/aliased variants)
	if strings.Contains(modelLower, "grok-4.5") {
		return s.fallbackPrices["grok-4.5"]
	}
	// Grok 4.3 family (dated variants)
	if strings.Contains(modelLower, "grok-4.3") {
		return s.fallbackPrices["grok-4.3"]
	}
	// Grok 4.20 series (all share grok-4.3 pricing per official)
	if strings.Contains(modelLower, "grok-4.20") {
		return s.fallbackPrices["grok-4.3"]
	}
	// Grok Build / code-fast family
	if strings.Contains(modelLower, "grok-build") || strings.Contains(modelLower, "grok-code-fast") || strings.Contains(modelLower, "grok-composer") {
		return s.fallbackPrices["grok-build-0.1"]
	}
	// grok-imagine-* and any other unknown grok-* → nil so media/token callers
	// use image-video pricing paths instead of silent text overcharge.

	return nil
}

func normalizeOpenAIPricingFallbackModel(model string) string {
	canonical := canonicalizeOpenAIModelAliasSpelling(model)
	if canonical == "" {
		return ""
	}
	if strings.Contains(canonical, "codex-auto-review") {
		return "codex-auto-review"
	}

	// 定价回退保留账单族群，不复用上游路由归一化里的跨版本折叠。
	if mapped := normalizeGPT56ModelAlias(canonical); mapped != "" {
		return mapped
	}
	if strings.HasPrefix(canonical, "gpt-5.6") {
		return ""
	}
	switch {
	case strings.HasPrefix(canonical, "gpt-5.5"):
		return "gpt-5.5"
	case strings.HasPrefix(canonical, "gpt-5.4-mini"):
		return "gpt-5.4-mini"
	case strings.HasPrefix(canonical, "gpt-5.4-nano"):
		return "gpt-5.4-nano"
	case strings.HasPrefix(canonical, "gpt-5.4"):
		return "gpt-5.4"
	case strings.HasPrefix(canonical, "gpt-5.3-codex-spark"):
		return "gpt-5.3-codex-spark"
	case strings.HasPrefix(canonical, "gpt-5.3-codex"):
		return "gpt-5.3-codex"
	case strings.HasPrefix(canonical, "gpt-5.2-codex"):
		return "gpt-5.2-codex"
	case strings.HasPrefix(canonical, "gpt-5.2"):
		return "gpt-5.2"
	case strings.HasPrefix(canonical, "gpt-5.1-codex-mini"),
		strings.HasPrefix(canonical, "codex-mini-latest"),
		strings.HasPrefix(canonical, "gpt-5-codex-mini"):
		return "gpt-5.1-codex-mini"
	case strings.HasPrefix(canonical, "gpt-5.1-codex-max"):
		return "gpt-5.1-codex-max"
	case strings.HasPrefix(canonical, "gpt-5.1-codex"):
		return "gpt-5.1-codex"
	case strings.HasPrefix(canonical, "gpt-5.1"):
		return "gpt-5.1"
	default:
		return normalizeKnownOpenAICodexModel(canonical)
	}
}

// GetModelPricing 获取模型价格配置
func (s *BillingService) GetModelPricing(model string) (*ModelPricing, error) {
	// 标准化模型名称（转小写）
	model = strings.ToLower(model)

	// 1. 优先从动态价格服务获取
	if s.pricingService != nil {
		litellmPricing := s.pricingService.GetModelPricing(model)
		if litellmPricing != nil {
			// 启用 5m/1h 分类计费的条件：
			// 1. 存在 1h 价格
			// 2. 1h 价格 > 5m 价格（防止 LiteLLM 数据错误导致少收费）
			price5m := litellmPricing.CacheCreationInputTokenCost
			price1h := litellmPricing.CacheCreationInputTokenCostAbove1hr
			enableBreakdown := price1h > 0 && price1h > price5m
			return s.applyModelSpecificPricingPolicy(model, &ModelPricing{
				InputPricePerToken:                 litellmPricing.InputCostPerToken,
				InputPricePerTokenPriority:         litellmPricing.InputCostPerTokenPriority,
				OutputPricePerToken:                litellmPricing.OutputCostPerToken,
				OutputPricePerTokenPriority:        litellmPricing.OutputCostPerTokenPriority,
				CacheCreationPricePerToken:         litellmPricing.CacheCreationInputTokenCost,
				CacheCreationPricePerTokenPriority: litellmPricing.CacheCreationInputTokenCostPriority,
				CacheReadPricePerToken:             litellmPricing.CacheReadInputTokenCost,
				CacheReadPricePerTokenPriority:     litellmPricing.CacheReadInputTokenCostPriority,
				CacheCreation5mPrice:               price5m,
				CacheCreation1hPrice:               price1h,
				SupportsCacheBreakdown:             enableBreakdown,
				LongContextInputThreshold:          litellmPricing.LongContextInputTokenThreshold,
				LongContextInputMultiplier:         litellmPricing.LongContextInputCostMultiplier,
				LongContextOutputMultiplier:        litellmPricing.LongContextOutputCostMultiplier,
				ImageOutputPricePerToken:           litellmPricing.OutputCostPerImageToken,
			}), nil
		}
	}

	// 2. 使用硬编码回退价格
	fallback := s.getFallbackPricing(model)
	if fallback != nil {
		// 按模型名去重:每个模型每进程最多打一条 warn,避免热路径每请求刷屏（issue #3394）。
		// model 在函数入口已 ToLower,故 GLM-5.2 / glm-5.2 视为同一条目。
		if _, seen := s.fallbackWarnSeen.LoadOrStore(model, struct{}{}); !seen {
			log.Printf("[Billing] Using fallback pricing for model: %s", model)
		}
		return s.applyModelSpecificPricingPolicy(model, cloneModelPricing(fallback)), nil
	}

	return nil, fmt.Errorf("%w for model: %s", ErrModelPricingUnavailable, model)
}

// GetModelPricingWithChannel 获取模型定价，渠道配置的价格覆盖默认值
// 渠道存在时，未配置的图片输出价格归零（不回退到 LiteLLM）
func (s *BillingService) GetModelPricingWithChannel(model string, channelPricing *ChannelModelPricing) (*ModelPricing, error) {
	pricing, err := s.GetModelPricing(model)
	if err != nil {
		return nil, err
	}
	if channelPricing == nil {
		return pricing, nil
	}
	if channelPricing.InputPrice != nil {
		pricing.InputPricePerToken = *channelPricing.InputPrice
	}
	if channelPricing.OutputPrice != nil {
		pricing.OutputPricePerToken = *channelPricing.OutputPrice
	}
	if channelPricing.CacheWritePrice != nil {
		pricing.CacheCreationPricePerToken = *channelPricing.CacheWritePrice
		pricing.CacheCreationPriceExplicit = true
		pricing.CacheCreation5mPrice = *channelPricing.CacheWritePrice
		pricing.CacheCreation1hPrice = *channelPricing.CacheWritePrice
	}
	if channelPricing.CacheReadPrice != nil {
		pricing.CacheReadPricePerToken = *channelPricing.CacheReadPrice
	}
	clearPriorityServiceTierPricing(pricing)
	if channelPricing.ImageOutputPrice != nil {
		pricing.ImageOutputPricePerToken = *channelPricing.ImageOutputPrice
	} else {
		pricing.ImageOutputPricePerToken = 0
	}
	pricing.ImageOutputPriceExplicit = true
	return pricing, nil
}

// --- 统一计费入口 ---

// CostInput 统一计费输入
type CostInput struct {
	Ctx            context.Context
	Model          string
	GroupID        *int64 // 用于渠道定价查找
	Tokens         UsageTokens
	RequestCount   int    // 按次计费时使用
	SizeTier       string // 按次/图片模式的层级标签（"1K","2K","4K","HD" 等）
	RateMultiplier float64
	ServiceTier    string                // "priority","flex","" 等
	Resolver       *ModelPricingResolver // 定价解析器
	Resolved       *ResolvedPricing      // 可选：预解析的定价结果（避免重复 Resolve 调用）
}

// CalculateCostUnified 统一计费入口，支持三种计费模式。
// 使用 ModelPricingResolver 解析定价，然后根据 BillingMode 分发计算。
func (s *BillingService) CalculateCostUnified(input CostInput) (*CostBreakdown, error) {
	if input.Resolver == nil {
		// 无 Resolver，回退到旧路径
		return s.calculateCostInternal(input.Model, input.Tokens, input.RateMultiplier, input.ServiceTier, nil)
	}

	// 优先使用预解析结果，避免重复 Resolve 调用
	resolved := input.Resolved
	if resolved == nil {
		resolved = input.Resolver.Resolve(input.Ctx, PricingInput{
			Model:   input.Model,
			GroupID: input.GroupID,
		})
	}

	// 保存时强制 > 0；若仍有负数泄漏（缓存/迁移残留），按 0 处理避免按 1x 误扣。
	if input.RateMultiplier < 0 {
		input.RateMultiplier = 0
	}

	var breakdown *CostBreakdown
	var err error
	switch resolved.Mode {
	case BillingModePerRequest, BillingModeImage:
		breakdown, err = s.calculatePerRequestCost(resolved, input)
	default: // BillingModeToken
		breakdown, err = s.calculateTokenCost(resolved, input)
	}
	if err == nil && breakdown != nil {
		breakdown.BillingMode = string(resolved.Mode)
		if breakdown.BillingMode == "" {
			breakdown.BillingMode = string(BillingModeToken)
		}
	}
	return breakdown, err
}

// calculateTokenCost 按 token 区间计费
func (s *BillingService) calculateTokenCost(resolved *ResolvedPricing, input CostInput) (*CostBreakdown, error) {
	totalContext := input.Tokens.InputTokens + input.Tokens.CacheCreationTokens + input.Tokens.CacheReadTokens

	pricing := input.Resolver.GetIntervalPricing(resolved, totalContext)
	if pricing == nil {
		return nil, fmt.Errorf("no pricing available for model: %s: %w", input.Model, ErrModelPricingUnavailable)
	}

	pricing = s.applyModelSpecificPricingPolicy(input.Model, pricing)

	// 长上下文定价仅在无区间定价时应用（区间定价已包含上下文分层）
	applyLongCtx := len(resolved.Intervals) == 0

	return s.computeTokenBreakdown(pricing, input.Tokens, input.RateMultiplier, input.ServiceTier, applyLongCtx), nil
}

// computeTokenBreakdown 是 token 计费的核心逻辑，由 calculateTokenCost 和 calculateCostInternal 共用。
// applyLongCtx 控制是否检查长上下文定价（区间定价已自含上下文分层，不需要额外应用）。
func (s *BillingService) computeTokenBreakdown(
	pricing *ModelPricing, tokens UsageTokens,
	rateMultiplier float64, serviceTier string,
	applyLongCtx bool,
) *CostBreakdown {
	// 保存时强制 > 0；若仍有负数泄漏，按 0 处理避免按 1x 误扣。
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}

	inputPrice := pricing.InputPricePerToken
	outputPrice := pricing.OutputPricePerToken
	cacheReadPrice := pricing.CacheReadPricePerToken
	cacheCreationPrice := pricing.CacheCreationPricePerToken
	cacheCreationMultiplier := 1.0
	tierMultiplier := 1.0
	uncoveredPriorityMultiplier := 1.0

	if usePriorityServiceTierPricing(serviceTier, pricing) {
		if pricing.InputPricePerTokenPriority > 0 {
			inputPrice = pricing.InputPricePerTokenPriority
		} else {
			inputPrice *= serviceTierCostMultiplier(serviceTier)
		}
		if pricing.OutputPricePerTokenPriority > 0 {
			outputPrice = pricing.OutputPricePerTokenPriority
		} else {
			outputPrice *= serviceTierCostMultiplier(serviceTier)
		}
		if pricing.CacheReadPricePerTokenPriority > 0 {
			cacheReadPrice = pricing.CacheReadPricePerTokenPriority
		} else {
			cacheReadPrice *= serviceTierCostMultiplier(serviceTier)
		}
		if pricing.CacheCreationPricePerTokenPriority > 0 && !pricing.CacheCreationPriceExplicit && !pricing.SupportsCacheBreakdown {
			cacheCreationPrice = pricing.CacheCreationPricePerTokenPriority
		} else {
			cacheCreationMultiplier *= serviceTierCostMultiplier(serviceTier)
		}
		uncoveredPriorityMultiplier = serviceTierCostMultiplier(serviceTier)
	} else {
		tierMultiplier = serviceTierCostMultiplier(serviceTier)
	}

	if applyLongCtx && s.shouldApplySessionLongContextPricing(tokens, pricing) {
		inputPrice *= pricing.LongContextInputMultiplier
		outputPrice *= pricing.LongContextOutputMultiplier
		// 缓存读取本质上是输入侧的复用，应与 input 一同应用长上下文倍率；
		// 否则 cache hit 越多，少计的费用越多（见 #2293）。
		cacheReadPrice *= pricing.LongContextInputMultiplier
		// 缓存创建（cache_write）也是输入侧操作，三档价格（标准 / 5m / 1h）
		// 都通过 computeCacheCreationCost 直接读取 pricing.*，不会经过这里
		// 的倍率修改，因此显式向下传一个倍率，避免长上下文场景下被漏乘。
		cacheCreationMultiplier *= pricing.LongContextInputMultiplier
	}
	bd := &CostBreakdown{}
	// 分离图片输入 token 与文本输入 token（多模态 embedding 等图文不同价场景）。
	// ImageInputTokens 为 0 时（绝大多数 chat/vision 流量）走原始单价路径，行为不变。
	if tokens.ImageInputTokens > 0 {
		imageInputTokens := tokens.ImageInputTokens
		textInputTokens := tokens.InputTokens - imageInputTokens
		if textInputTokens < 0 {
			textInputTokens = 0
			imageInputTokens = tokens.InputTokens
		}
		imageInputPrice := pricing.ImageInputPricePerToken
		if imageInputPrice == 0 {
			// 未配置图片输入档时回退到文本 input 价（已含 priority / 长上下文调整）
			imageInputPrice = inputPrice
		} else {
			imageInputPrice *= uncoveredPriorityMultiplier
		}
		bd.InputCost = float64(textInputTokens)*inputPrice + float64(imageInputTokens)*imageInputPrice
	} else {
		bd.InputCost = float64(tokens.InputTokens) * inputPrice
	}

	// 分离图片输出 token 与文本输出 token
	textOutputTokens := tokens.OutputTokens - tokens.ImageOutputTokens
	if textOutputTokens < 0 {
		textOutputTokens = 0
	}
	bd.OutputCost = float64(textOutputTokens) * outputPrice

	// 图片输出 token 费用（独立费率）
	if tokens.ImageOutputTokens > 0 {
		imgPrice := pricing.ImageOutputPricePerToken
		if imgPrice == 0 && !pricing.ImageOutputPriceExplicit {
			imgPrice = outputPrice
		} else {
			imgPrice *= uncoveredPriorityMultiplier
		}
		bd.ImageOutputCost = float64(tokens.ImageOutputTokens) * imgPrice
	}

	// 缓存创建费用
	bd.CacheCreationCost = s.computeCacheCreationCost(pricing, tokens, cacheCreationPrice, cacheCreationMultiplier)

	bd.CacheReadCost = float64(tokens.CacheReadTokens) * cacheReadPrice

	if tierMultiplier != 1.0 {
		bd.InputCost *= tierMultiplier
		bd.OutputCost *= tierMultiplier
		bd.ImageOutputCost *= tierMultiplier
		bd.CacheCreationCost *= tierMultiplier
		bd.CacheReadCost *= tierMultiplier
	}

	bd.TotalCost = bd.InputCost + bd.OutputCost + bd.ImageOutputCost +
		bd.CacheCreationCost + bd.CacheReadCost
	bd.ActualCost = bd.TotalCost * rateMultiplier

	return bd
}

// computeCacheCreationCost 计算缓存创建费用（支持 5m/1h 分类或标准计费）。
// multiplier 用于长上下文等场景下的整体价格缩放（普通调用传 1.0 即可）。
func (s *BillingService) computeCacheCreationCost(pricing *ModelPricing, tokens UsageTokens, price, multiplier float64) float64 {
	if pricing.SupportsCacheBreakdown && (pricing.CacheCreation5mPrice > 0 || pricing.CacheCreation1hPrice > 0) {
		if tokens.CacheCreation5mTokens == 0 && tokens.CacheCreation1hTokens == 0 && tokens.CacheCreationTokens > 0 {
			// API 未返回 ephemeral 明细，回退到全部按 5m 单价计费
			return float64(tokens.CacheCreationTokens) * pricing.CacheCreation5mPrice * multiplier
		}
		return float64(tokens.CacheCreation5mTokens)*pricing.CacheCreation5mPrice*multiplier +
			float64(tokens.CacheCreation1hTokens)*pricing.CacheCreation1hPrice*multiplier
	}
	return float64(tokens.CacheCreationTokens) * price * multiplier
}

// calculatePerRequestCost 按次/图片计费
func (s *BillingService) calculatePerRequestCost(resolved *ResolvedPricing, input CostInput) (*CostBreakdown, error) {
	count := input.RequestCount
	if count <= 0 {
		count = 1
	}

	var unitPrice float64

	if input.SizeTier != "" {
		unitPrice = input.Resolver.GetRequestTierPrice(resolved, input.SizeTier)
	}

	if unitPrice == 0 {
		totalContext := input.Tokens.InputTokens + input.Tokens.CacheCreationTokens + input.Tokens.CacheReadTokens
		unitPrice = input.Resolver.GetRequestTierPriceByContext(resolved, totalContext)
	}

	// 回退到默认按次价格
	if unitPrice == 0 {
		unitPrice = resolved.DefaultPerRequestPrice
	}

	totalCost := unitPrice * float64(count)
	actualCost := totalCost * input.RateMultiplier

	return &CostBreakdown{
		TotalCost:  totalCost,
		ActualCost: actualCost,
	}, nil
}

// CalculateCost 计算使用费用
func (s *BillingService) CalculateCost(model string, tokens UsageTokens, rateMultiplier float64) (*CostBreakdown, error) {
	return s.calculateCostInternal(model, tokens, rateMultiplier, "", nil)
}

func (s *BillingService) CalculateCostWithServiceTier(model string, tokens UsageTokens, rateMultiplier float64, serviceTier string) (*CostBreakdown, error) {
	return s.calculateCostInternal(model, tokens, rateMultiplier, serviceTier, nil)
}

func (s *BillingService) calculateCostInternal(model string, tokens UsageTokens, rateMultiplier float64, serviceTier string, channelPricing *ChannelModelPricing) (*CostBreakdown, error) {
	var pricing *ModelPricing
	var err error
	if channelPricing != nil {
		pricing, err = s.GetModelPricingWithChannel(model, channelPricing)
	} else {
		pricing, err = s.GetModelPricing(model)
	}
	if err != nil {
		return nil, err
	}

	// 旧路径始终检查长上下文定价（无区间定价概念）
	return s.computeTokenBreakdown(pricing, tokens, rateMultiplier, serviceTier, true), nil
}

func (s *BillingService) applyModelSpecificPricingPolicy(model string, pricing *ModelPricing) *ModelPricing {
	if pricing == nil {
		return nil
	}
	normalized := normalizeOpenAIPricingFallbackModel(model)
	isGPT56 := strings.HasPrefix(normalized, "gpt-5.6-")
	if !isGPT56 && !isOpenAIGPT54Model(model) {
		return pricing
	}
	needsLongContextPolicy := pricing.LongContextInputThreshold <= 0 || pricing.LongContextInputMultiplier <= 0 || pricing.LongContextOutputMultiplier <= 0
	needsCacheCreationPolicy := isGPT56 && !pricing.CacheCreationPriceExplicit && (pricing.CacheCreationPricePerToken <= 0 ||
		(pricing.InputPricePerTokenPriority > 0 && pricing.CacheCreationPricePerTokenPriority <= 0))
	if !needsLongContextPolicy && !needsCacheCreationPolicy {
		return pricing
	}
	cloned := *pricing
	if isGPT56 && !cloned.CacheCreationPriceExplicit {
		if cloned.CacheCreationPricePerToken <= 0 {
			cloned.CacheCreationPricePerToken = cloned.InputPricePerToken * 1.25
		}
		if cloned.CacheCreationPricePerTokenPriority <= 0 {
			cloned.CacheCreationPricePerTokenPriority = cloned.InputPricePerTokenPriority * 1.25
		}
	}
	if cloned.LongContextInputThreshold <= 0 {
		cloned.LongContextInputThreshold = openAIGPT54LongContextInputThreshold
	}
	if cloned.LongContextInputMultiplier <= 0 {
		cloned.LongContextInputMultiplier = openAIGPT54LongContextInputMultiplier
	}
	if cloned.LongContextOutputMultiplier <= 0 {
		cloned.LongContextOutputMultiplier = openAIGPT54LongContextOutputMultiplier
	}
	return &cloned
}

func (s *BillingService) shouldApplySessionLongContextPricing(tokens UsageTokens, pricing *ModelPricing) bool {
	if pricing == nil || pricing.LongContextInputThreshold <= 0 {
		return false
	}
	if pricing.LongContextInputMultiplier <= 1 && pricing.LongContextOutputMultiplier <= 1 {
		return false
	}
	totalInputTokens := tokens.InputTokens + tokens.CacheCreationTokens + tokens.CacheReadTokens
	return totalInputTokens > pricing.LongContextInputThreshold
}

func isOpenAIGPT54Model(model string) bool {
	// 仅当模型字符串实际属于已知 GPT-5/Codex 族时才做归一判定，避免
	// normalizeCodexModel 的默认兜底把非 OpenAI 模型（claude-*、gemini-*、gpt-4o）
	// 误识别为 gpt-5.4。
	normalized := normalizeOpenAIPricingFallbackModel(model)
	return normalized == "gpt-5.4" || normalized == "gpt-5.5" || strings.HasPrefix(normalized, "gpt-5.6-")
}

// CalculateCostWithConfig 使用配置中的默认倍率计算费用
func (s *BillingService) CalculateCostWithConfig(model string, tokens UsageTokens) (*CostBreakdown, error) {
	multiplier := 1.0
	if s != nil && s.cfg != nil {
		multiplier = s.cfg.Default.RateMultiplier
		if multiplier <= 0 {
			multiplier = 1.0
		}
	}
	return s.CalculateCost(model, tokens, multiplier)
}

// CalculateCostWithLongContext 计算费用，支持长上下文双倍计费
// threshold: 阈值（如 200000），超过此值的部分按 extraMultiplier 倍计费
// extraMultiplier: 超出部分的倍率（如 2.0 表示双倍）
//
// 示例：缓存 210k + 输入 10k = 220k，阈值 200k，倍率 2.0
// 拆分为：范围内 (200k, 0) + 范围外 (10k, 10k)
// 范围内正常计费，范围外 × 2 计费
func (s *BillingService) CalculateCostWithLongContext(model string, tokens UsageTokens, rateMultiplier float64, threshold int, extraMultiplier float64) (*CostBreakdown, error) {
	// 未启用长上下文计费，直接走正常计费
	if threshold <= 0 || extraMultiplier <= 1 {
		return s.CalculateCost(model, tokens, rateMultiplier)
	}

	// 计算总输入 token（缓存读取 + 缓存写入 + 新输入）
	total := tokens.CacheReadTokens + tokens.CacheCreationTokens + tokens.InputTokens
	if total <= threshold {
		return s.CalculateCost(model, tokens, rateMultiplier)
	}

	// 拆分成范围内和范围外
	remaining := threshold
	takeInRange := func(n int) (int, int) {
		if n <= 0 {
			return 0, 0
		}
		if remaining <= 0 {
			return 0, n
		}
		in := min(n, remaining)
		remaining -= in
		return in, n - in
	}
	inRangeCacheTokens, outRangeCacheTokens := takeInRange(tokens.CacheReadTokens)
	inRangeCacheCreation5mTokens, outRangeCacheCreation5mTokens := takeInRange(tokens.CacheCreation5mTokens)
	inRangeCacheCreation1hTokens, outRangeCacheCreation1hTokens := takeInRange(tokens.CacheCreation1hTokens)
	breakdownCacheCreationTokens := tokens.CacheCreation5mTokens + tokens.CacheCreation1hTokens
	inRangeCacheCreationTokens := inRangeCacheCreation5mTokens + inRangeCacheCreation1hTokens
	outRangeCacheCreationTokens := outRangeCacheCreation5mTokens + outRangeCacheCreation1hTokens
	if breakdownCacheCreationTokens == 0 {
		inRangeCacheCreationTokens, outRangeCacheCreationTokens = takeInRange(tokens.CacheCreationTokens)
	}
	inRangeInputTokens, outRangeInputTokens := takeInRange(tokens.InputTokens)

	// 范围内部分：正常计费
	inRangeTokens := UsageTokens{
		InputTokens:           inRangeInputTokens,
		OutputTokens:          tokens.OutputTokens, // 输出只算一次
		CacheCreationTokens:   inRangeCacheCreationTokens,
		CacheReadTokens:       inRangeCacheTokens,
		CacheCreation5mTokens: inRangeCacheCreation5mTokens,
		CacheCreation1hTokens: inRangeCacheCreation1hTokens,
		ImageOutputTokens:     tokens.ImageOutputTokens,
	}
	inRangeCost, err := s.CalculateCost(model, inRangeTokens, rateMultiplier)
	if err != nil {
		return nil, err
	}

	// 范围外部分：× extraMultiplier 计费
	outRangeTokens := UsageTokens{
		InputTokens:           outRangeInputTokens,
		CacheCreationTokens:   outRangeCacheCreationTokens,
		CacheReadTokens:       outRangeCacheTokens,
		CacheCreation5mTokens: outRangeCacheCreation5mTokens,
		CacheCreation1hTokens: outRangeCacheCreation1hTokens,
	}
	outRangeCost, err := s.CalculateCost(model, outRangeTokens, rateMultiplier*extraMultiplier)
	if err != nil {
		return inRangeCost, fmt.Errorf("out-range cost: %w", err)
	}

	// 合并成本
	return &CostBreakdown{
		InputCost:         inRangeCost.InputCost + outRangeCost.InputCost,
		OutputCost:        inRangeCost.OutputCost,
		ImageOutputCost:   inRangeCost.ImageOutputCost,
		CacheCreationCost: inRangeCost.CacheCreationCost + outRangeCost.CacheCreationCost,
		CacheReadCost:     inRangeCost.CacheReadCost + outRangeCost.CacheReadCost,
		TotalCost:         inRangeCost.TotalCost + outRangeCost.TotalCost,
		ActualCost:        inRangeCost.ActualCost + outRangeCost.ActualCost,
	}, nil
}

// ListSupportedModels 列出所有支持的模型（现在总是返回true，因为有模糊匹配）
func (s *BillingService) ListSupportedModels() []string {
	models := make([]string, 0)
	// 返回回退价格支持的模型系列
	for model := range s.fallbackPrices {
		models = append(models, model)
	}
	return models
}

// IsModelSupported 检查模型是否支持（现在总是返回true，因为有模糊匹配回退）
func (s *BillingService) IsModelSupported(model string) bool {
	// 所有Claude模型都有回退价格支持
	modelLower := strings.ToLower(model)
	return strings.Contains(modelLower, "claude") ||
		strings.Contains(modelLower, "opus") ||
		strings.Contains(modelLower, "sonnet") ||
		strings.Contains(modelLower, "haiku")
}

// GetEstimatedCost 估算费用（用于前端展示）
func (s *BillingService) GetEstimatedCost(model string, estimatedInputTokens, estimatedOutputTokens int) (float64, error) {
	tokens := UsageTokens{
		InputTokens:  estimatedInputTokens,
		OutputTokens: estimatedOutputTokens,
	}

	breakdown, err := s.CalculateCostWithConfig(model, tokens)
	if err != nil {
		return 0, err
	}

	return breakdown.ActualCost, nil
}

// GetPricingServiceStatus 获取价格服务状态
func (s *BillingService) GetPricingServiceStatus() map[string]any {
	if s.pricingService != nil {
		return s.pricingService.GetStatus()
	}
	return map[string]any{
		"model_count":  len(s.fallbackPrices),
		"last_updated": "using fallback",
		"local_hash":   "N/A",
	}
}

// ForceUpdatePricing 强制更新价格数据
func (s *BillingService) ForceUpdatePricing() error {
	if s.pricingService != nil {
		return s.pricingService.ForceUpdate()
	}
	return fmt.Errorf("pricing service not initialized")
}

// ImagePriceConfig 图片计费配置
type ImagePriceConfig struct {
	Price1K *float64 // 1K 尺寸价格（nil 表示使用默认值）
	Price2K *float64 // 2K 尺寸价格（nil 表示使用默认值）
	Price4K *float64 // 4K 尺寸价格（nil 表示使用默认值）
}

const defaultWebSearchPricePerCall = 0.01

func (s *BillingService) CalculateWebSearchCost(callCount int, groupPrice *float64, rateMultiplier float64) *CostBreakdown {
	if callCount <= 0 {
		return &CostBreakdown{}
	}
	unitPrice := defaultWebSearchPricePerCall
	if groupPrice != nil && *groupPrice >= 0 {
		unitPrice = *groupPrice
	}
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	totalCost := unitPrice * float64(callCount)
	return &CostBreakdown{
		TotalCost:   totalCost,
		ActualCost:  totalCost * rateMultiplier,
		BillingMode: string(BillingModePerRequest),
	}
}

// CalculateImageCost 计算图片生成费用
// model: 请求的模型名称（用于获取 LiteLLM 默认价格）
// imageSize: 图片尺寸 "1K", "2K", "4K"
// imageCount: 生成的图片数量
// groupConfig: 分组配置的价格（可能为 nil，表示使用默认值）
// rateMultiplier: 费率倍数
func (s *BillingService) CalculateImageCost(model string, imageSize string, imageCount int, groupConfig *ImagePriceConfig, rateMultiplier float64) *CostBreakdown {
	if imageCount <= 0 {
		return &CostBreakdown{}
	}
	imageSize = NormalizeImageBillingTierOrDefault(imageSize)

	// 获取单价
	unitPrice := s.getImageUnitPrice(model, imageSize, groupConfig)

	// 计算总费用
	totalCost := unitPrice * float64(imageCount)

	// 应用倍率（保存时强制 > 0；负数按 0 处理避免按 1x 误扣）
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	actualCost := totalCost * rateMultiplier

	return &CostBreakdown{
		TotalCost:   totalCost,
		ActualCost:  actualCost,
		BillingMode: string(BillingModeImage),
	}
}

// getImageUnitPrice 获取图片单价
func (s *BillingService) getImageUnitPrice(model string, imageSize string, groupConfig *ImagePriceConfig) float64 {
	// 优先使用分组配置的价格
	if groupConfig != nil {
		switch imageSize {
		case "1K":
			if groupConfig.Price1K != nil {
				return *groupConfig.Price1K
			}
		case "2K":
			if groupConfig.Price2K != nil {
				return *groupConfig.Price2K
			}
		case "4K":
			if groupConfig.Price4K != nil {
				return *groupConfig.Price4K
			}
		}
	}

	// 回退到 LiteLLM 默认价格
	return s.getDefaultImagePrice(model, imageSize)
}

// Official xAI Imagine list prices (https://docs.x.ai/docs/models).
const (
	grokImagineImageQualityPricePerImage  = 0.05 // grok-imagine-image-quality
	grokImagineImageFastPricePerImage     = 0.02 // grok-imagine-image
	grokImagineVideoPrice480pPerSecond    = 0.05 // grok-imagine-video 480p
	grokImagineVideoPrice720pPerSecond    = 0.07 // grok-imagine-video 720p/1080p
	grokImagineVideo15Price480pPerSecond  = 0.08 // grok-imagine-video-1.5 480p
	grokImagineVideo15Price720pPerSecond  = 0.14 // grok-imagine-video-1.5 720p
	grokImagineVideo15Price1080pPerSecond = 0.25 // grok-imagine-video-1.5 1080p/4k
)

// getDefaultImagePrice 获取 LiteLLM 默认图片价格
func (s *BillingService) getDefaultImagePrice(model string, imageSize string) float64 {
	basePrice := 0.0

	// 从 PricingService 获取 output_cost_per_image
	if s.pricingService != nil {
		pricing := s.pricingService.GetModelPricing(model)
		if pricing != nil && pricing.OutputCostPerImage > 0 {
			basePrice = pricing.OutputCostPerImage
		}
	}

	// Grok Imagine: official flat per-image rates (prefer over Gemini default).
	if basePrice <= 0 {
		if p := getGrokImagineDefaultImagePrice(model); p > 0 {
			basePrice = p
		}
	}

	// 如果没有找到价格，使用硬编码默认值（$0.134，来自 gemini-3-pro-image-preview）
	if basePrice <= 0 {
		basePrice = 0.134
	}

	// Official Grok Imagine is flat per image regardless of resolution tier.
	if isGrokImagineImageModel(model) {
		return basePrice
	}

	// 2K 尺寸 1.5 倍，4K 尺寸翻倍
	if imageSize == "2K" {
		return basePrice * 1.5
	}
	if imageSize == "4K" {
		return basePrice * 2
	}

	return basePrice
}

func isGrokImagineImageModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	// Aliases (grok-imagine, grok-imagine-1, grok-imagine-edit) bill as quality.
	if strings.HasPrefix(m, "grok-imagine-image") ||
		m == "grok-imagine" || m == "grok-imagine-1" || m == "grok-imagine-edit" {
		return true
	}
	return false
}

// getGrokImagineDefaultImagePrice returns official flat per-image price, or 0 if not an Imagine image model.
// Official list: quality $0.05/image, fast (grok-imagine-image) $0.02/image; aliases bill as quality.
func getGrokImagineDefaultImagePrice(model string) float64 {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" || strings.Contains(m, "video") {
		return 0
	}
	switch {
	case m == "grok-imagine-image":
		return grokImagineImageFastPricePerImage
	case strings.HasPrefix(m, "grok-imagine-image-quality"),
		m == "grok-imagine", m == "grok-imagine-1", m == "grok-imagine-edit",
		strings.HasPrefix(m, "grok-imagine-image"),
		strings.HasPrefix(m, "grok-imagine"):
		return grokImagineImageQualityPricePerImage
	default:
		return 0
	}
}

// CalculateVideoCost calculates video generation cost from group per-second pricing.
//
// It accepts both the local call shape:
//
//	CalculateVideoCost(videoSize, seconds, videoCount, groupConfig, rateMultiplier[, model])
//
// and the upstream call shape kept for merge compatibility:
//
//	CalculateVideoCost(model, resolution, videoCount, durationSeconds, groupConfig, rateMultiplier)
func (s *BillingService) CalculateVideoCost(first string, args ...any) *CostBreakdown {
	videoSize, seconds, videoCount, groupConfig, rateMultiplier, model, defaultMissingDuration := parseCalculateVideoCostArgs(first, args)
	if defaultMissingDuration {
		seconds = NormalizeVideoBillingDurationSecondsOrDefault(seconds)
	}
	if seconds <= 0 || videoCount <= 0 {
		return &CostBreakdown{BillingMode: string(BillingModeVideo)}
	}
	sizeTier := NormalizeVideoBillingTierOrDefault(videoSize)
	unitPrice := s.getVideoUnitPrice(model, sizeTier, groupConfig)
	totalCost := unitPrice * float64(seconds) * float64(videoCount)
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	actualCost := totalCost * rateMultiplier
	return &CostBreakdown{
		TotalCost:   totalCost,
		ActualCost:  actualCost,
		BillingMode: string(BillingModeVideo),
	}
}

func parseCalculateVideoCostArgs(first string, args []any) (videoSize string, seconds int, videoCount int, groupConfig *VideoPriceConfig, rateMultiplier float64, model string, defaultMissingDuration bool) {
	videoSize = first
	rateMultiplier = 1
	if len(args) == 0 {
		return videoSize, seconds, videoCount, groupConfig, rateMultiplier, model, false
	}
	if resolution, ok := args[0].(string); ok {
		// Upstream-compatible shape: model, resolution, count, duration, config, multiplier.
		model = first
		videoSize = resolution
		videoCount = videoCostArgInt(args, 1)
		seconds = videoCostArgInt(args, 2)
		groupConfig = videoCostArgConfig(args, 3)
		rateMultiplier = videoCostArgFloat(args, 4, 1)
		return videoSize, seconds, videoCount, groupConfig, rateMultiplier, model, true
	}
	// Local shape: size, seconds, count, config, multiplier[, model].
	seconds = videoCostArgInt(args, 0)
	videoCount = videoCostArgInt(args, 1)
	groupConfig = videoCostArgConfig(args, 2)
	rateMultiplier = videoCostArgFloat(args, 3, 1)
	model = videoCostArgString(args, 4)
	return videoSize, seconds, videoCount, groupConfig, rateMultiplier, model, false
}

func videoCostArgInt(args []any, idx int) int {
	if idx < 0 || idx >= len(args) {
		return 0
	}
	switch v := args[idx].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}

func videoCostArgFloat(args []any, idx int, fallback float64) float64 {
	if idx < 0 || idx >= len(args) {
		return fallback
	}
	switch v := args[idx].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return fallback
	}
}

func videoCostArgConfig(args []any, idx int) *VideoPriceConfig {
	if idx < 0 || idx >= len(args) || args[idx] == nil {
		return nil
	}
	cfg, _ := args[idx].(*VideoPriceConfig)
	return cfg
}

func videoCostArgString(args []any, idx int) string {
	if idx < 0 || idx >= len(args) {
		return ""
	}
	value, _ := args[idx].(string)
	return strings.TrimSpace(value)
}

func (s *BillingService) GetVideoUnitPrice(model string, sizeTier string, groupConfig *VideoPriceConfig) float64 {
	return s.getVideoUnitPrice(model, sizeTier, groupConfig)
}

func (s *BillingService) getVideoUnitPrice(model string, sizeTier string, groupConfig *VideoPriceConfig) float64 {
	if groupConfig != nil {
		var price *float64
		switch NormalizeVideoBillingTierOrDefault(sizeTier) {
		case VideoBillingTier480p:
			price = firstFloatPtr(groupConfig.Price480p, groupConfig.Price480P)
		case VideoBillingTier720p:
			price = firstFloatPtr(groupConfig.Price720p, groupConfig.Price720P)
		case VideoBillingTier1080p:
			price = firstFloatPtr(groupConfig.Price1080p, groupConfig.Price1080P)
		case VideoBillingTier4K:
			price = groupConfig.Price4K
		default:
			price = firstFloatPtr(groupConfig.Price720p, groupConfig.Price720P)
		}
		if price == nil {
			// Fallback to highest non-nil defined price (avoid 0 for 4K or missing tier per report).
			for _, p := range []*float64{
				groupConfig.Price4K,
				firstFloatPtr(groupConfig.Price1080p, groupConfig.Price1080P),
				firstFloatPtr(groupConfig.Price720p, groupConfig.Price720P),
				firstFloatPtr(groupConfig.Price480p, groupConfig.Price480P),
			} {
				if p != nil {
					price = p
					log.Printf("[Billing] video price tier %s not configured, falling back to highest configured tier", sizeTier)
					break
				}
			}
		}
		if price != nil {
			return *price
		}
	}
	return s.getDefaultVideoPricePerSecond(model, sizeTier)
}

// getDefaultVideoPricePerSecond resolves catalog / hard-coded Grok Imagine per-second rates.
func (s *BillingService) getDefaultVideoPricePerSecond(model string, sizeTier string) float64 {
	if price := getGrokImagineDefaultVideoPricePerSecond(model, sizeTier); price > 0 {
		return price
	}
	if s.pricingService != nil {
		if pricing := s.pricingService.GetModelPricing(model); pricing != nil && pricing.OutputCostPerSecond > 0 {
			return pricing.OutputCostPerSecond
		}
	}
	return 0
}

func getGrokImagineDefaultVideoPricePerSecond(model string, sizeTierOptional ...string) float64 {
	m := strings.ToLower(strings.TrimSpace(model))
	sizeTier := VideoBillingTier480p
	if len(sizeTierOptional) > 0 {
		sizeTier = NormalizeVideoBillingTierOrDefault(sizeTierOptional[0])
	}
	switch {
	case m == "grok-imagine-video-1.5" || strings.Contains(m, "video-1.5") || m == "grok-video-1.5":
		switch sizeTier {
		case VideoBillingTier480p:
			return grokImagineVideo15Price480pPerSecond
		case VideoBillingTier720p:
			return grokImagineVideo15Price720pPerSecond
		case VideoBillingTier1080p, VideoBillingTier4K:
			return grokImagineVideo15Price1080pPerSecond
		default:
			return grokImagineVideo15Price480pPerSecond
		}
	case m == "grok-imagine-video" || m == "grok-video" || m == "grok-video-latest" ||
		strings.HasPrefix(m, "grok-imagine-video") || strings.HasPrefix(m, "grok-video"):
		switch sizeTier {
		case VideoBillingTier480p:
			return grokImagineVideoPrice480pPerSecond
		case VideoBillingTier720p, VideoBillingTier1080p, VideoBillingTier4K:
			return grokImagineVideoPrice720pPerSecond
		default:
			return grokImagineVideoPrice480pPerSecond
		}
	default:
		return 0
	}
}

// --- Grok / explicit non-text pricing (image/audio/search) ---
// These use explicit group prices (not derived from token pricing), and callers
// typically pass rateMultiplier=1 for platforms like OpenAI/Grok to avoid applying text rate.

type SearchPriceConfig struct {
	PricePer1k *float64
}

// CalculateSearchCost bills search/tool invocations (e.g. web_search, x_search) per 1k calls.
// numCalls is the count of tool invocations in the request/response.
// Uses explicit group price if set; otherwise 0 (caller may fall back).
func (s *BillingService) CalculateSearchCost(numCalls int, groupPricePer1k *float64, rateMultiplier float64) *CostBreakdown {
	if numCalls <= 0 {
		return &CostBreakdown{}
	}
	if groupPricePer1k == nil || *groupPricePer1k <= 0 {
		return &CostBreakdown{}
	}
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	unit := *groupPricePer1k / 1000.0 // per call
	total := unit * float64(numCalls)
	return &CostBreakdown{
		TotalCost:   total,
		ActualCost:  total * rateMultiplier,
		BillingMode: string(BillingModeSearch), // reuse or add if needed; for now use a mode
	}
}

type audioPriceConfig struct {
	RealtimePerMin *float64
	TTSPerMChars   *float64
	STTPerHour     *float64
}

// CalculateAudioCost supports realtime (per min), tts (per M chars), stt (per hr).
// durationOrUnits: minutes for realtime, millions chars for tts, hours for stt.
// mode: "realtime" | "tts" | "stt"
func (s *BillingService) CalculateAudioCost(mode string, durationOrUnits float64, groupConfig *audioPriceConfig, rateMultiplier float64) *CostBreakdown {
	if durationOrUnits <= 0 {
		return &CostBreakdown{}
	}
	var unitPrice float64
	switch strings.ToLower(mode) {
	case "realtime":
		if groupConfig != nil && groupConfig.RealtimePerMin != nil {
			unitPrice = *groupConfig.RealtimePerMin
		}
	case "tts":
		if groupConfig != nil && groupConfig.TTSPerMChars != nil {
			unitPrice = *groupConfig.TTSPerMChars
		}
	case "stt":
		if groupConfig != nil && groupConfig.STTPerHour != nil {
			unitPrice = *groupConfig.STTPerHour
		}
	default:
		return &CostBreakdown{}
	}
	if unitPrice <= 0 {
		return &CostBreakdown{}
	}
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	total := unitPrice * durationOrUnits
	return &CostBreakdown{
		TotalCost:   total,
		ActualCost:  total * rateMultiplier,
		BillingMode: string(BillingModeAudio),
	}
}
