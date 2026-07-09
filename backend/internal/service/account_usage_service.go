package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	httppool "github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	openaipkg "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

type UsageLogRepository interface {
	// Create creates a usage log and returns whether it was actually inserted.
	// inserted is false when the insert was skipped due to conflict (idempotent retries).
	Create(ctx context.Context, log *UsageLog) (inserted bool, err error)
	RecordOpenAIStickyScheduleEvent(ctx context.Context, input *OpenAIStickyScheduleEventInput) error
	GetByID(ctx context.Context, id int64) (*UsageLog, error)
	Delete(ctx context.Context, id int64) error

	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error)
	ListByAPIKey(ctx context.Context, apiKeyID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error)
	ListByAccount(ctx context.Context, accountID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error)

	ListByUserAndTimeRange(ctx context.Context, userID int64, startTime, endTime time.Time) ([]UsageLog, *pagination.PaginationResult, error)
	ListByAPIKeyAndTimeRange(ctx context.Context, apiKeyID int64, startTime, endTime time.Time) ([]UsageLog, *pagination.PaginationResult, error)
	ListByAccountAndTimeRange(ctx context.Context, accountID int64, startTime, endTime time.Time) ([]UsageLog, *pagination.PaginationResult, error)
	ListByModelAndTimeRange(ctx context.Context, modelName string, startTime, endTime time.Time) ([]UsageLog, *pagination.PaginationResult, error)

	GetAccountWindowStats(ctx context.Context, accountID int64, startTime time.Time) (*usagestats.AccountStats, error)
	GetAccountTodayStats(ctx context.Context, accountID int64) (*usagestats.AccountStats, error)

	// Admin dashboard stats
	GetDashboardStats(ctx context.Context) (*usagestats.DashboardStats, error)
	GetUsageTrendWithFilters(ctx context.Context, startTime, endTime time.Time, granularity string, userID, apiKeyID, accountID, groupID int64, model string, requestType *int16, stream *bool, billingType *int8, billingMode string) ([]usagestats.TrendDataPoint, error)
	GetModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8, billingMode string) ([]usagestats.ModelStat, error)
	GetEndpointStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, model string, requestType *int16, stream *bool, billingType *int8) ([]usagestats.EndpointStat, error)
	GetUpstreamEndpointStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, model string, requestType *int16, stream *bool, billingType *int8) ([]usagestats.EndpointStat, error)
	GetGroupStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8, billingMode string) ([]usagestats.GroupStat, error)
	GetUserBreakdownStats(ctx context.Context, startTime, endTime time.Time, dim usagestats.UserBreakdownDimension, limit int) ([]usagestats.UserBreakdownItem, error)
	GetAllGroupUsageSummary(ctx context.Context, todayStart time.Time) ([]usagestats.GroupUsageSummary, error)
	GetAPIKeyUsageTrend(ctx context.Context, startTime, endTime time.Time, granularity string, limit int) ([]usagestats.APIKeyUsageTrendPoint, error)
	GetUserUsageTrend(ctx context.Context, startTime, endTime time.Time, granularity string, limit int) ([]usagestats.UserUsageTrendPoint, error)
	GetUserSpendingRanking(ctx context.Context, startTime, endTime time.Time, limit int) (*usagestats.UserSpendingRankingResponse, error)
	GetBatchUserUsageStats(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.BatchUserUsageStats, error)
	GetBatchAPIKeyUsageStats(ctx context.Context, apiKeyIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.BatchAPIKeyUsageStats, error)

	// User dashboard stats
	GetUserDashboardStats(ctx context.Context, userID int64) (*usagestats.UserDashboardStats, error)
	GetAPIKeyDashboardStats(ctx context.Context, apiKeyID int64) (*usagestats.UserDashboardStats, error)
	GetUserUsageTrendByUserID(ctx context.Context, userID int64, startTime, endTime time.Time, granularity string) ([]usagestats.TrendDataPoint, error)
	GetUserModelStats(ctx context.Context, userID int64, startTime, endTime time.Time) ([]usagestats.ModelStat, error)

	// Admin usage listing/stats
	ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]UsageLog, *pagination.PaginationResult, error)
	GetGlobalStats(ctx context.Context, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetStatsWithFilters(ctx context.Context, filters usagestats.UsageLogFilters) (*usagestats.UsageStats, error)

	// Account stats
	GetAccountUsageStats(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error)

	// Aggregated stats (optimized)
	GetUserStatsAggregated(ctx context.Context, userID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetAPIKeyStatsAggregated(ctx context.Context, apiKeyID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetAccountStatsAggregated(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetModelStatsAggregated(ctx context.Context, modelName string, startTime, endTime time.Time) (*usagestats.UsageStats, error)
	GetDailyStatsAggregated(ctx context.Context, userID int64, startTime, endTime time.Time) ([]map[string]any, error)
}

type accountWindowStatsBatchReader interface {
	GetAccountWindowStatsBatch(ctx context.Context, accountIDs []int64, startTime time.Time) (map[int64]*usagestats.AccountStats, error)
}

// apiUsageCache 缓存从 Anthropic API 获取的使用率数据（utilization, resets_at）
// 同时支持缓存错误响应（负缓存），防止 429 等错误导致的重试风暴
type apiUsageCache struct {
	response  *ClaudeUsageResponse
	err       error // 非 nil 表示缓存的错误（负缓存）
	timestamp time.Time
}

// windowStatsCache 缓存从本地数据库查询的窗口统计（requests, tokens, cost）
type windowStatsCache struct {
	stats     *WindowStats
	timestamp time.Time
}

// antigravityUsageCache 缓存 Antigravity 额度数据
type antigravityUsageCache struct {
	usageInfo *UsageInfo
	timestamp time.Time
}

type kiroUsageCache struct {
	usageInfo *UsageInfo
	timestamp time.Time
}

const (
	apiCacheTTL             = 3 * time.Minute
	apiErrorCacheTTL        = 1 * time.Minute        // 负缓存 TTL：429 等错误缓存 1 分钟
	antigravityErrorTTL     = 1 * time.Minute        // Antigravity 错误缓存 TTL（可恢复错误）
	apiQueryMaxJitter       = 800 * time.Millisecond // 用量查询最大随机延迟
	windowStatsCacheTTL     = 1 * time.Minute
	openAIProbeCacheTTL     = 10 * time.Minute
	openAICodexProbeVersion = "0.125.0"
)

// UsageCache 封装账户使用量相关的缓存
type UsageCache struct {
	apiCache          sync.Map           // accountID -> *apiUsageCache
	windowStatsCache  sync.Map           // accountID -> *windowStatsCache
	antigravityCache  sync.Map           // accountID -> *antigravityUsageCache
	kiroCache         sync.Map           // accountID -> *kiroUsageCache
	apiFlight         singleflight.Group // 防止同一账号的并发请求击穿缓存（Anthropic）
	antigravityFlight singleflight.Group // 防止同一 Antigravity 账号的并发请求击穿缓存
	kiroFlight        singleflight.Group // 防止同一 Kiro 账号的并发请求击穿缓存
	openAIProbeCache  sync.Map           // accountID -> time.Time
}

// NewUsageCache 创建 UsageCache 实例
func NewUsageCache() *UsageCache {
	return &UsageCache{}
}

// WindowStats 窗口期统计
//
// cost: 账号口径费用（total_cost * account_rate_multiplier）
// standard_cost: 标准费用（total_cost，不含倍率）
// user_cost: 用户/API Key 口径费用（actual_cost，受分组倍率影响）
type WindowStats struct {
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	StandardCost float64 `json:"standard_cost"`
	UserCost     float64 `json:"user_cost"`
}

// UsageProgress 使用量进度
type UsageProgress struct {
	Utilization      float64      `json:"utilization"`            // 使用率百分比 (0-100+，100表示100%)
	ResetsAt         *time.Time   `json:"resets_at"`              // 重置时间
	RemainingSeconds int          `json:"remaining_seconds"`      // 距重置剩余秒数
	WindowStats      *WindowStats `json:"window_stats,omitempty"` // 窗口期统计（从窗口开始到当前的使用量）
	UsedRequests     int64        `json:"used_requests,omitempty"`
	LimitRequests    int64        `json:"limit_requests,omitempty"`
}

// AntigravityModelQuota Antigravity 单个模型的配额信息
type AntigravityModelQuota struct {
	Utilization int    `json:"utilization"` // 使用率 0-100
	ResetTime   string `json:"reset_time"`  // 重置时间 ISO8601
}

// AntigravityModelDetail Antigravity 单个模型的详细能力信息
type AntigravityModelDetail struct {
	DisplayName        string          `json:"display_name,omitempty"`
	SupportsImages     *bool           `json:"supports_images,omitempty"`
	SupportsThinking   *bool           `json:"supports_thinking,omitempty"`
	ThinkingBudget     *int            `json:"thinking_budget,omitempty"`
	Recommended        *bool           `json:"recommended,omitempty"`
	MaxTokens          *int            `json:"max_tokens,omitempty"`
	MaxOutputTokens    *int            `json:"max_output_tokens,omitempty"`
	SupportedMimeTypes map[string]bool `json:"supported_mime_types,omitempty"`
}

// AICredit 表示 Antigravity 账号的 AI Credits 余额信息。
type AICredit struct {
	CreditType     string  `json:"credit_type,omitempty"`
	Amount         float64 `json:"amount,omitempty"`
	MinimumBalance float64 `json:"minimum_balance,omitempty"`
}

// KiroQuotaBreakdown 表示 Kiro 月度、赠送、试用或总额度的拆分信息。
type KiroQuotaBreakdown struct {
	CurrentUsage float64    `json:"current_usage"`
	UsageLimit   float64    `json:"usage_limit"`
	Remaining    float64    `json:"remaining"`
	Utilization  float64    `json:"utilization"`
	ResetsAt     *time.Time `json:"resets_at,omitempty"`
}

// UsageInfo 账号使用量信息
type UsageInfo struct {
	Source             string         `json:"source,omitempty"`               // "passive" or "active"
	UpdatedAt          *time.Time     `json:"updated_at,omitempty"`           // 更新时间
	FiveHour           *UsageProgress `json:"five_hour"`                      // 5小时窗口
	SevenDay           *UsageProgress `json:"seven_day,omitempty"`            // 7天窗口
	SevenDaySonnet     *UsageProgress `json:"seven_day_sonnet,omitempty"`     // 7天Sonnet窗口
	KiroQuota          *UsageProgress `json:"kiro_quota,omitempty"`           // Kiro 总额度进度
	GeminiSharedDaily  *UsageProgress `json:"gemini_shared_daily,omitempty"`  // Gemini shared pool RPD (Google One / Code Assist)
	GeminiProDaily     *UsageProgress `json:"gemini_pro_daily,omitempty"`     // Gemini Pro 日配额
	GeminiFlashDaily   *UsageProgress `json:"gemini_flash_daily,omitempty"`   // Gemini Flash 日配额
	GeminiSharedMinute *UsageProgress `json:"gemini_shared_minute,omitempty"` // Gemini shared pool RPM (Google One / Code Assist)
	GeminiProMinute    *UsageProgress `json:"gemini_pro_minute,omitempty"`    // Gemini Pro RPM
	GeminiFlashMinute  *UsageProgress `json:"gemini_flash_minute,omitempty"`  // Gemini Flash RPM

	// Antigravity 多模型配额
	AntigravityQuota map[string]*AntigravityModelQuota `json:"antigravity_quota,omitempty"`

	// Grok / xAI 被动额度快照
	GrokRequestQuota       *xai.QuotaWindow `json:"grok_request_quota,omitempty"`
	GrokTokenQuota         *xai.QuotaWindow `json:"grok_token_quota,omitempty"`
	GrokRetryAfterSeconds  *int             `json:"grok_retry_after_seconds,omitempty"`
	GrokEntitlementStatus  string           `json:"grok_entitlement_status,omitempty"`
	GrokQuotaSnapshotState string           `json:"grok_quota_snapshot_state,omitempty"`
	GrokLastQuotaProbeAt   string           `json:"grok_last_quota_probe_at,omitempty"`
	GrokLastHeadersSeenAt  string           `json:"grok_last_headers_seen_at,omitempty"`
	GrokLastStatusCode     int              `json:"grok_last_status_code,omitempty"`
	GrokLocalUsage         *WindowStats     `json:"grok_local_usage,omitempty"`

	// Antigravity 账号级信息
	SubscriptionTier    string `json:"subscription_tier,omitempty"`     // 归一化订阅等级: FREE/PRO/ULTRA/UNKNOWN
	SubscriptionTierRaw string `json:"subscription_tier_raw,omitempty"` // 上游原始订阅等级名称

	// Antigravity 模型详细能力信息（与 antigravity_quota 同 key）
	AntigravityQuotaDetails map[string]*AntigravityModelDetail `json:"antigravity_quota_details,omitempty"`

	// Antigravity AI Credits 余额
	AICredits []AICredit `json:"ai_credits,omitempty"`

	// Antigravity 废弃模型转发规则 (old_model_id -> new_model_id)
	ModelForwardingRules map[string]string `json:"model_forwarding_rules,omitempty"`

	// Kiro 账号额度信息
	KiroSubscriptionTitle string              `json:"kiro_subscription_title,omitempty"`
	KiroCurrentUsage      float64             `json:"kiro_current_usage,omitempty"`
	KiroUsageLimit        float64             `json:"kiro_usage_limit,omitempty"`
	KiroRemaining         float64             `json:"kiro_remaining,omitempty"`
	KiroMonthlyQuota      *KiroQuotaBreakdown `json:"kiro_monthly_quota,omitempty"`
	KiroBonusQuota        *KiroQuotaBreakdown `json:"kiro_bonus_quota,omitempty"`
	KiroFreeTrialQuota    *KiroQuotaBreakdown `json:"kiro_free_trial_quota,omitempty"`
	KiroTotalQuota        *KiroQuotaBreakdown `json:"kiro_total_quota,omitempty"`

	// Antigravity 账号是否被上游禁止 (HTTP 403)
	IsForbidden     bool   `json:"is_forbidden,omitempty"`
	ForbiddenReason string `json:"forbidden_reason,omitempty"`
	ForbiddenType   string `json:"forbidden_type,omitempty"` // "validation" / "violation" / "forbidden"
	ValidationURL   string `json:"validation_url,omitempty"` // 验证/申诉链接

	// 状态标记（从 ForbiddenType / HTTP 错误码推导）
	NeedsVerify bool `json:"needs_verify,omitempty"` // 需要人工验证（forbidden_type=validation）
	IsBanned    bool `json:"is_banned,omitempty"`    // 账号被封（forbidden_type=violation）
	NeedsReauth bool `json:"needs_reauth,omitempty"` // token 失效需重新授权（401）

	// 错误码（机器可读）：forbidden / unauthenticated / rate_limited / network_error
	ErrorCode string `json:"error_code,omitempty"`

	// OpenAI 图片生成路由状态
	OpenAIImageCodexSupported *bool          `json:"openai_image_codex_supported,omitempty"`
	OpenAIImageCodexReason    string         `json:"openai_image_codex_reason,omitempty"`
	OpenAIImagePlanType       string         `json:"openai_image_plan_type,omitempty"`
	OpenAIImageWorkspaceName  string         `json:"openai_image_workspace_name,omitempty"`
	OpenAIImageCodexFiveHour  *UsageProgress `json:"openai_image_codex_five_hour,omitempty"`
	OpenAIImageCodexSevenDay  *UsageProgress `json:"openai_image_codex_seven_day,omitempty"`

	// 获取 usage 时的错误信息（降级返回，而非 500）
	Error string `json:"error,omitempty"`
}

// ClaudeUsageResponse Anthropic API返回的usage结构
type ClaudeUsageResponse struct {
	FiveHour struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
	SevenDay struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
	SevenDaySonnet struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day_sonnet"`
}

// ClaudeUsageFetchOptions 包含获取 Claude 用量数据所需的所有选项
type ClaudeUsageFetchOptions struct {
	AccessToken string                  // OAuth access token
	ProxyURL    string                  // 代理 URL（可选）
	AccountID   int64                   // 账号 ID（用于连接池隔离）
	TLSProfile  *tlsfingerprint.Profile // TLS 指纹 Profile（nil 表示不启用）
	Fingerprint *Fingerprint            // 缓存的指纹信息（User-Agent 等）
}

// ClaudeUsageFetcher fetches usage data from Anthropic OAuth API
type ClaudeUsageFetcher interface {
	FetchUsage(ctx context.Context, accessToken, proxyURL string) (*ClaudeUsageResponse, error)
	// FetchUsageWithOptions 使用完整选项获取用量数据，支持 TLS 指纹和自定义 User-Agent
	FetchUsageWithOptions(ctx context.Context, opts *ClaudeUsageFetchOptions) (*ClaudeUsageResponse, error)
}

// AccountUsageService 账号使用量查询服务
type AccountUsageService struct {
	accountRepo             AccountRepository
	usageLogRepo            UsageLogRepository
	usageFetcher            ClaudeUsageFetcher
	geminiQuotaService      *GeminiQuotaService
	antigravityQuotaFetcher *AntigravityQuotaFetcher
	kiroTokenProvider       *KiroTokenProvider
	grokQuotaFetcher        *GrokQuotaFetcher
	openAIQuotaService      *OpenAIQuotaService
	cache                   *UsageCache
	identityCache           IdentityCache
	tokenCacheInvalidator   TokenCacheInvalidator
	proxyRepo               ProxyRepository
	tlsFPProfileService     *TLSFingerprintProfileService
	settingService          *SettingService
}

// NewAccountUsageService 创建AccountUsageService实例
func NewAccountUsageService(
	accountRepo AccountRepository,
	usageLogRepo UsageLogRepository,
	usageFetcher ClaudeUsageFetcher,
	geminiQuotaService *GeminiQuotaService,
	antigravityQuotaFetcher *AntigravityQuotaFetcher,
	kiroTokenProvider *KiroTokenProvider,
	grokQuotaFetcher *GrokQuotaFetcher,
	openAIQuotaService *OpenAIQuotaService,
	cache *UsageCache,
	identityCache IdentityCache,
	tokenCacheInvalidator TokenCacheInvalidator,
	proxyRepo ProxyRepository,
	tlsFPProfileService *TLSFingerprintProfileService,
	settingService *SettingService,
) *AccountUsageService {
	return &AccountUsageService{
		accountRepo:             accountRepo,
		usageLogRepo:            usageLogRepo,
		usageFetcher:            usageFetcher,
		geminiQuotaService:      geminiQuotaService,
		antigravityQuotaFetcher: antigravityQuotaFetcher,
		kiroTokenProvider:       kiroTokenProvider,
		grokQuotaFetcher:        grokQuotaFetcher,
		openAIQuotaService:      openAIQuotaService,
		cache:                   cache,
		identityCache:           identityCache,
		tokenCacheInvalidator:   tokenCacheInvalidator,
		proxyRepo:               proxyRepo,
		tlsFPProfileService:     tlsFPProfileService,
		settingService:          settingService,
	}
}

func supportsAnthropicPassiveUsage(account *Account) bool {
	if account == nil || account.Platform != PlatformAnthropic {
		return false
	}
	switch account.Type {
	case AccountTypeOAuth, AccountTypeSetupToken, AccountTypeServiceAccount:
		return true
	default:
		return false
	}
}

func batchUsageErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	switch infraerrors.Code(err) {
	case 400:
		return "bad request: " + infraerrors.Message(err)
	case 401:
		return "unauthorized: " + infraerrors.Message(err)
	case 403:
		return "forbidden: " + infraerrors.Message(err)
	case 404:
		return "not found: " + infraerrors.Message(err)
	case 429:
		return "too many requests: " + infraerrors.Message(err)
	default:
		return err.Error()
	}
}

func (s *AccountUsageService) getUsageForAccount(ctx context.Context, account *Account, forceProbe bool) (*UsageInfo, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	accountID := account.ID

	if account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth {
		usage, err := s.getOpenAIUsage(ctx, account, forceProbe)
		if err == nil {
			s.tryClearRecoverableAccountError(ctx, account)
		}
		return usage, err
	}

	if account.Platform == PlatformGemini {
		usage, err := s.getGeminiUsage(ctx, account)
		if err == nil {
			s.tryClearRecoverableAccountError(ctx, account)
		}
		return usage, err
	}

	// Antigravity 平台：使用 AntigravityQuotaFetcher 获取额度
	if account.Platform == PlatformAntigravity {
		usage, err := s.getAntigravityUsage(ctx, account)
		if err == nil {
			s.tryClearRecoverableAccountError(ctx, account)
		}
		return usage, err
	}

	if account.Platform == PlatformKiro && account.Type == AccountTypeOAuth {
		usage, err := s.getKiroUsage(ctx, account)
		if err == nil && usage != nil && usage.Error == "" && usage.ErrorCode == "" && !usage.IsForbidden && !usage.NeedsReauth {
			s.tryClearRecoverableAccountError(ctx, account)
		}
		return usage, err
	}

	if account.Platform == PlatformGrok {
		usage, err := s.getGrokUsage(ctx, account)
		if err == nil {
			s.tryClearRecoverableAccountError(ctx, account)
		}
		return usage, err
	}

	// 只有oauth类型账号可以通过API获取usage（有profile scope）
	if account.CanGetUsage() {
		var apiResp *ClaudeUsageResponse

		// 1. 检查缓存（成功响应 3 分钟 / 错误响应 1 分钟）
		if cached, ok := s.cache.apiCache.Load(accountID); ok {
			if cache, ok := cached.(*apiUsageCache); ok {
				age := time.Since(cache.timestamp)
				if cache.err != nil && age < apiErrorCacheTTL {
					// 负缓存命中：返回缓存的错误，避免重试风暴
					return nil, cache.err
				}
				if cache.response != nil && age < apiCacheTTL {
					apiResp = cache.response
				}
			}
		}

		// 2. 如果没有有效缓存，通过 singleflight 从 API 获取（防止并发击穿）
		if apiResp == nil {
			// 随机延迟：打散多账号并发请求，避免同一时刻大量相同 TLS 指纹请求
			// 触发上游反滥用检测。延迟范围 0~800ms，仅在缓存未命中时生效。
			jitter := time.Duration(rand.Int64N(int64(apiQueryMaxJitter)))
			select {
			case <-time.After(jitter):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			flightKey := fmt.Sprintf("usage:%d", accountID)
			result, flightErr, _ := s.cache.apiFlight.Do(flightKey, func() (any, error) {
				// 再次检查缓存（可能在等待 singleflight 期间被其他请求填充）
				if cached, ok := s.cache.apiCache.Load(accountID); ok {
					if cache, ok := cached.(*apiUsageCache); ok {
						age := time.Since(cache.timestamp)
						if cache.err != nil && age < apiErrorCacheTTL {
							return nil, cache.err
						}
						if cache.response != nil && age < apiCacheTTL {
							return cache.response, nil
						}
					}
				}
				resp, fetchErr := s.fetchOAuthUsageRaw(ctx, account)
				if fetchErr != nil {
					// 负缓存：缓存错误响应，防止后续请求重复触发 429
					s.cache.apiCache.Store(accountID, &apiUsageCache{
						err:       fetchErr,
						timestamp: time.Now(),
					})
					return nil, fetchErr
				}
				// 缓存成功响应
				s.cache.apiCache.Store(accountID, &apiUsageCache{
					response:  resp,
					timestamp: time.Now(),
				})
				return resp, nil
			})
			if flightErr != nil {
				return nil, flightErr
			}
			apiResp, _ = result.(*ClaudeUsageResponse)
		}

		// 3. 构建 UsageInfo（每次都重新计算 RemainingSeconds）
		now := time.Now()
		usage := s.buildUsageInfo(apiResp, &now)

		// 4. 添加窗口统计（有独立缓存，1 分钟）
		s.addWindowStats(ctx, account, usage)

		// 5. 将主动查询结果同步到被动缓存，下次 passive 加载即为最新值
		s.syncActiveToPassive(ctx, account.ID, usage)

		s.tryClearRecoverableAccountError(ctx, account)
		return usage, nil
	}

	// Setup Token账号：根据session_window推算（没有profile scope，无法调用usage API）
	if account.Type == AccountTypeSetupToken {
		usage := s.estimateSetupTokenUsage(account)
		// 添加窗口统计
		s.addWindowStats(ctx, account, usage)
		return usage, nil
	}

	// API Key账号不支持usage查询
	return nil, infraerrors.BadRequest(
		"ACCOUNT_USAGE_UNSUPPORTED",
		fmt.Sprintf("account type %s does not support usage query", account.Type),
	)
}

// GetUsage 获取账号使用量
// OAuth账号: 调用Anthropic API获取真实数据（需要profile scope），API响应缓存10分钟，窗口统计缓存1分钟
// Setup Token账号: 根据session_window推算5h窗口，7d数据不可用（没有profile scope）
// API Key账号: 不支持usage查询
func (s *AccountUsageService) GetUsage(ctx context.Context, accountID int64, force ...bool) (*UsageInfo, error) {
	forceProbe := len(force) > 0 && force[0]

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get account failed: %w", err)
	}

	return s.getUsageForAccount(ctx, account, forceProbe)
}

// GetUsageBatch 批量获取账号使用量。
// Anthropic OAuth/SetupToken/service_account 统一走 passive 链路，其他账号复用现有主动查询逻辑。
// 单个账号失败不会中断整批请求，错误会按账号返回。
func (s *AccountUsageService) GetUsageBatch(ctx context.Context, accountIDs []int64, force bool) (map[int64]*UsageInfo, map[int64]string, error) {
	uniqueIDs := make([]int64, 0, len(accountIDs))
	seen := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID <= 0 {
			continue
		}
		if _, ok := seen[accountID]; ok {
			continue
		}
		seen[accountID] = struct{}{}
		uniqueIDs = append(uniqueIDs, accountID)
	}

	usageByAccount := make(map[int64]*UsageInfo, len(uniqueIDs))
	errorsByAccount := make(map[int64]string)
	if len(uniqueIDs) == 0 {
		return usageByAccount, errorsByAccount, nil
	}

	accounts, err := s.accountRepo.GetByIDs(ctx, uniqueIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("get accounts failed: %w", err)
	}

	accountsByID := make(map[int64]*Account, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		accountsByID[account.ID] = account
	}

	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(6)

	for _, accountID := range uniqueIDs {
		id := accountID
		account := accountsByID[id]
		if account == nil {
			errorsByAccount[id] = ErrAccountNotFound.Error()
			continue
		}

		g.Go(func() error {
			var usage *UsageInfo
			var usageErr error
			if supportsAnthropicPassiveUsage(account) {
				usage, usageErr = s.getPassiveUsageForAccount(gctx, account)
			} else {
				usage, usageErr = s.getUsageForAccount(gctx, account, force)
			}

			mu.Lock()
			defer mu.Unlock()
			if usageErr != nil {
				errorsByAccount[id] = batchUsageErrorMessage(usageErr)
				return nil
			}
			usageByAccount[id] = usage
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	return usageByAccount, errorsByAccount, nil
}

func (s *AccountUsageService) getKiroUsage(ctx context.Context, account *Account) (*UsageInfo, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	if s.cache != nil {
		if cached, ok := s.cache.kiroCache.Load(account.ID); ok {
			if cache, ok := cached.(*kiroUsageCache); ok {
				ttl := kiroUsageCacheTTL(cache.usageInfo)
				if time.Since(cache.timestamp) < ttl {
					usage := cache.usageInfo
					recalcKiroRemainingSeconds(usage)
					return usage, nil
				}
			}
		}
	}
	var loadUsage func(fetchCtx context.Context, forceRefresh bool) (*UsageInfo, error)
	loadUsage = func(fetchCtx context.Context, forceRefresh bool) (*UsageInfo, error) {
		kiroHTTPUpstream := s.kiroHTTPUpstream()
		accessToken := ""
		if forceRefresh {
			// 走带分布式锁+DB 重读+invalid_grant 竞争恢复+credentials-only 持久化的统一刷新入口，
			// 不再裸调 refresher.Refresh + 整行 accountRepo.Update（会与网关侧带锁刷新并发 lost-update
			// 掉 token、把账号推上分叉的 refresh_token 链最终变砖）。RefreshAccount 用 kiroAdminRefreshSkew
			// (100 年)使 NeedsRefresh 恒真，保留“强制换新 token”语义。
			if err := s.invalidateKiroTokenCache(fetchCtx, account); err != nil {
				log.Printf("warning: failed to invalidate Kiro token cache before usage refresh for account %d: %v", account.ID, err)
			}
			if s.kiroTokenProvider == nil {
				return buildKiroDegradedUsage(fmt.Errorf("kiro token provider is not configured")), nil
			}
			refreshedAccount, err := s.kiroTokenProvider.RefreshAccount(fetchCtx, account)
			if err != nil {
				if shouldRetryKiroUsageWithForcedRefresh(err) {
					return buildKiroDegradedUsage(fmt.Errorf("kiro usage upstream returned 401: %w", err)), nil
				}
				return buildKiroDegradedUsage(fmt.Errorf("kiro usage refresh failed after auth retry: %w", err)), nil
			}
			copyKiroAccountRuntimeState(account, refreshedAccount)
			s.InvalidateKiroUsageCache(account.ID)
			accessToken = account.GetCredential("access_token")
		} else if s.kiroTokenProvider != nil {
			var err error
			accessToken, err = s.kiroTokenProvider.GetAccessToken(fetchCtx, account)
			if err != nil {
				return buildKiroDegradedUsage(err), nil
			}
		} else {
			accessToken = account.GetCredential("access_token")
		}
		if accessToken == "" {
			if s.kiroTokenProvider == nil {
				return buildKiroDegradedUsage(fmt.Errorf("kiro token provider is not configured")), nil
			}
			refreshedAccount, err := s.kiroTokenProvider.RefreshAccount(fetchCtx, account)
			if err != nil {
				return buildKiroDegradedUsage(err), nil
			}
			copyKiroAccountRuntimeState(account, refreshedAccount)
			s.InvalidateKiroUsageCache(account.ID)
			accessToken = account.GetCredential("access_token")
		}

		usageService := NewKiroUsageService().
			WithTransport(kiroHTTPUpstream, s.tlsFPProfileService).
			WithSettingService(s.settingService).
			WithProxyRepo(s.proxyRepo)
		limits, err := usageService.FetchUsageLimits(fetchCtx, account, accessToken)
		if err != nil {
			if !forceRefresh && shouldRetryKiroUsageWithForcedRefresh(err) {
				s.markKiroAccessTokenRejected(fetchCtx, account)
				if s.kiroTokenProvider == nil {
					// 首次请求拿旧 access token 命中 401 时，会尝试“强制刷新后重试”来恢复。
					// 但若当前实例根本未配置 refresh 能力（测试/裁剪部署），不能把原始 401
					// 降级语义改写成“provider 未配置”的网络错误，否则会丢掉 NeedsReauth/401
					// 信号并错误清洗掉账号的可恢复认证态。
					return buildKiroDegradedUsage(fmt.Errorf("kiro usage upstream returned 401: %w", err)), nil
				}
				return loadUsage(fetchCtx, true)
			}
			return buildKiroDegradedUsage(err), nil
		}

		currentUsage := limits.CurrentUsage()
		usageLimit := limits.UsageLimit()
		remaining := limits.Remaining()
		utilization := 0.0
		if usageLimit > 0 {
			utilization = (currentUsage / usageLimit) * 100
		}
		var resetAt *time.Time
		if t := limits.ResetAt(); t != nil {
			resetAt = t
		}
		now := time.Now()

		monthlyQuota := buildKiroQuotaBreakdown(limits.MonthlyCurrentUsage(), limits.MonthlyUsageLimit(), resetAt)
		bonusQuota := buildKiroQuotaBreakdown(limits.BonusCurrentUsage(), limits.BonusUsageLimit(), nil)
		freeTrialQuota := buildKiroQuotaBreakdown(limits.FreeTrialCurrentUsage(), limits.FreeTrialUsageLimit(), nil)
		totalQuota := buildKiroQuotaBreakdown(currentUsage, usageLimit, resetAt)
		windowStats := &WindowStats{}
		if s.usageLogRepo != nil {
			startTime := timezone.StartOfMonth(now)
			if stats, err := s.usageLogRepo.GetAccountWindowStats(fetchCtx, account.ID, startTime); err == nil && stats != nil {
				windowStats = windowStatsFromAccountStats(stats)
			} else if err != nil {
				log.Printf("Failed to get Kiro window stats for account %d: %v", account.ID, err)
			}
		}
		info := &UsageInfo{
			KiroSubscriptionTitle: limits.SubscriptionTitle(),
			KiroCurrentUsage:      currentUsage,
			KiroUsageLimit:        usageLimit,
			KiroRemaining:         remaining,
			KiroMonthlyQuota:      monthlyQuota,
			KiroBonusQuota:        bonusQuota,
			KiroFreeTrialQuota:    freeTrialQuota,
			KiroTotalQuota:        totalQuota,
			KiroQuota: &UsageProgress{
				Utilization: utilization,
				ResetsAt:    resetAt,
				WindowStats: windowStats,
			},
		}
		info.UpdatedAt = &now
		recalcKiroRemainingSeconds(info)
		s.persistKiroSchedulerSnapshot(fetchCtx, account, info)
		return info, nil
	}

	if s.cache == nil {
		return loadUsage(ctx, false)
	}

	flightKey := fmt.Sprintf("kiro-usage:%d", account.ID)
	result, flightErr, _ := s.cache.kiroFlight.Do(flightKey, func() (any, error) {
		if cached, ok := s.cache.kiroCache.Load(account.ID); ok {
			if cache, ok := cached.(*kiroUsageCache); ok {
				ttl := kiroUsageCacheTTL(cache.usageInfo)
				if time.Since(cache.timestamp) < ttl {
					usage := cache.usageInfo
					recalcKiroRemainingSeconds(usage)
					return usage, nil
				}
			}
		}

		fetchCtx, fetchCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer fetchCancel()

		usage, err := loadUsage(fetchCtx, false)
		if err != nil {
			return nil, err
		}
		s.cache.kiroCache.Store(account.ID, &kiroUsageCache{
			usageInfo: usage,
			timestamp: time.Now(),
		})
		return usage, nil
	})
	if flightErr != nil {
		return nil, flightErr
	}
	usage, ok := result.(*UsageInfo)
	if !ok || usage == nil {
		now := time.Now()
		return &UsageInfo{UpdatedAt: &now}, nil
	}
	return usage, nil
}

func shouldRetryKiroUsageWithForcedRefresh(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "kiro usage upstream returned 401") ||
		strings.Contains(errStr, "kiro refresh token is invalid") ||
		strings.Contains(errStr, "kiro_refresh_token_invalid") ||
		strings.Contains(errStr, "invalid_grant") ||
		strings.Contains(errStr, "invalid token") ||
		strings.Contains(errStr, "invalid_token") ||
		strings.Contains(errStr, "expired token") ||
		strings.Contains(errStr, "token expired") ||
		strings.Contains(errStr, "unauthorized")
}

func buildKiroDegradedUsage(err error) *UsageInfo {
	now := time.Now()
	info := &UsageInfo{
		UpdatedAt: &now,
		Error:     "Failed to fetch Kiro usage: " + err.Error(),
	}

	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "returned 401"),
		strings.Contains(errStr, "invalid_grant"),
		strings.Contains(errStr, "unauthenticated"),
		strings.Contains(errStr, "access_token not found"):
		info.ErrorCode = errorCodeUnauthenticated
		info.NeedsReauth = true
	case strings.Contains(errStr, "returned 403"),
		strings.Contains(errStr, "forbidden"):
		info.ErrorCode = errorCodeForbidden
		info.IsForbidden = true
		info.ForbiddenType = forbiddenTypeForbidden
		info.ForbiddenReason = err.Error()
	case strings.Contains(errStr, "returned 429"):
		info.ErrorCode = errorCodeRateLimited
	default:
		info.ErrorCode = errorCodeNetworkError
	}

	return info
}

func buildKiroQuotaBreakdown(currentUsage, usageLimit float64, resetAt *time.Time) *KiroQuotaBreakdown {
	if currentUsage <= 0 && usageLimit <= 0 {
		return nil
	}
	remaining := usageLimit - currentUsage
	if remaining < 0 {
		remaining = 0
	}
	utilization := 0.0
	if usageLimit > 0 {
		utilization = (currentUsage / usageLimit) * 100
	}
	return &KiroQuotaBreakdown{
		CurrentUsage: currentUsage,
		UsageLimit:   usageLimit,
		Remaining:    remaining,
		Utilization:  utilization,
		ResetsAt:     resetAt,
	}
}

func recalcKiroRemainingSeconds(info *UsageInfo) {
	if info == nil || info.KiroQuota == nil || info.KiroQuota.ResetsAt == nil {
		return
	}
	remaining := int(time.Until(*info.KiroQuota.ResetsAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	info.KiroQuota.RemainingSeconds = remaining
}

func kiroUsageCacheTTL(info *UsageInfo) time.Duration {
	if info == nil {
		return apiErrorCacheTTL
	}
	if info.ErrorCode != "" || info.Error != "" || info.NeedsReauth || info.IsForbidden {
		return apiErrorCacheTTL
	}
	return apiCacheTTL
}

func (s *AccountUsageService) InvalidateKiroUsageCache(accountID int64) {
	if s == nil || s.cache == nil || accountID <= 0 {
		return
	}
	s.cache.kiroCache.Delete(accountID)
	s.cache.kiroFlight.Forget(fmt.Sprintf("kiro-usage:%d", accountID))
}

func (s *AccountUsageService) kiroHTTPUpstream() HTTPUpstream {
	if s == nil || s.usageFetcher == nil {
		return nil
	}
	type httpUpstreamProvider interface {
		HTTPUpstream() HTTPUpstream
	}
	if provider, ok := s.usageFetcher.(httpUpstreamProvider); ok {
		return provider.HTTPUpstream()
	}
	return nil
}

func (s *AccountUsageService) invalidateKiroTokenCache(ctx context.Context, account *Account) error {
	if s.tokenCacheInvalidator != nil {
		if err := s.tokenCacheInvalidator.InvalidateToken(ctx, account); err != nil {
			log.Printf("warning: failed to invalidate Kiro token cache after usage refresh for account %d: %v", account.ID, err)
			return err
		}
	}
	return nil
}

func (s *AccountUsageService) markKiroAccessTokenRejected(ctx context.Context, account *Account) {
	if s == nil || account == nil {
		return
	}
	credentials := cloneCredentials(account.Credentials)
	if credentials == nil {
		credentials = make(map[string]any, 1)
	}
	credentials["expires_at"] = time.Now().Add(-kiroTokenRefreshSkew).UTC().Format(time.RFC3339)
	account.Credentials = credentials

	updater, ok := any(s.accountRepo).(accountCredentialsUpdater)
	if !ok || updater == nil {
		return
	}
	if err := updater.UpdateCredentials(ctx, account.ID, credentials); err != nil {
		log.Printf("warning: failed to persist rejected Kiro access token state for account %d: %v", account.ID, err)
	}
}

// persistRefreshedKiroCredentials keeps unit tests and legacy refresh paths on the
// same semantics after Kiro usage refresh was consolidated around RefreshAccount.
// It persists the refreshed credentials, updates the caller's in-memory account,
// invalidates the cached usage snapshot, and best-effort invalidates token cache.
func (s *AccountUsageService) persistRefreshedKiroCredentials(ctx context.Context, account *Account, newCreds map[string]any) error {
	if s == nil || account == nil {
		return nil
	}
	updated := *account
	updated.Credentials = cloneCredentials(newCreds)
	if err := s.accountRepo.Update(ctx, &updated); err != nil {
		return err
	}
	account.Credentials = cloneCredentials(newCreds)
	s.InvalidateKiroUsageCache(account.ID)
	_ = s.invalidateKiroTokenCache(ctx, account)
	return nil
}

// GetPassiveUsage 从 Account.Extra 中的被动采样数据构建 UsageInfo，不调用外部 API。
// 仅适用于 Anthropic OAuth / SetupToken / service_account 账号。
func (s *AccountUsageService) GetPassiveUsage(ctx context.Context, accountID int64) (*UsageInfo, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get account failed: %w", err)
	}

	return s.getPassiveUsageForAccount(ctx, account)
}

func (s *AccountUsageService) getPassiveUsageForAccount(ctx context.Context, account *Account) (*UsageInfo, error) {
	if !supportsAnthropicPassiveUsage(account) {
		return nil, fmt.Errorf("passive usage only supported for Anthropic OAuth/SetupToken/service_account accounts")
	}

	// 复用 estimateSetupTokenUsage 构建 5h 窗口（OAuth 和 SetupToken 逻辑一致）
	info := s.estimateSetupTokenUsage(account)
	info.Source = "passive"

	// 设置采样时间
	if raw, ok := account.Extra["passive_usage_sampled_at"]; ok {
		if str, ok := raw.(string); ok {
			if t, err := time.Parse(time.RFC3339, str); err == nil {
				info.UpdatedAt = &t
			}
		}
	}

	// 构建 7d 窗口（从被动采样数据）
	util7d := parseExtraFloat64(account.Extra["passive_usage_7d_utilization"])
	reset7dRaw := parseExtraFloat64(account.Extra["passive_usage_7d_reset"])
	if util7d > 0 || reset7dRaw > 0 {
		var resetAt *time.Time
		var remaining int
		if reset7dRaw > 0 {
			t := time.Unix(int64(reset7dRaw), 0)
			resetAt = &t
			remaining = int(time.Until(t).Seconds())
			if remaining < 0 {
				remaining = 0
			}
		}
		info.SevenDay = &UsageProgress{
			Utilization:      util7d * 100,
			ResetsAt:         resetAt,
			RemainingSeconds: remaining,
		}
	}

	// 添加窗口统计
	s.addWindowStats(ctx, account, info)

	return info, nil
}

// syncActiveToPassive 将主动查询的最新数据回写到 Extra 被动缓存，
// 这样下次被动加载时能看到最新值。
func (s *AccountUsageService) syncActiveToPassive(ctx context.Context, accountID int64, usage *UsageInfo) {
	extraUpdates := make(map[string]any, 4)

	if usage.FiveHour != nil {
		extraUpdates["session_window_utilization"] = usage.FiveHour.Utilization / 100
	}
	if usage.SevenDay != nil {
		extraUpdates["passive_usage_7d_utilization"] = usage.SevenDay.Utilization / 100
		if usage.SevenDay.ResetsAt != nil {
			extraUpdates["passive_usage_7d_reset"] = usage.SevenDay.ResetsAt.Unix()
		}
	}

	if len(extraUpdates) > 0 {
		extraUpdates["passive_usage_sampled_at"] = time.Now().UTC().Format(time.RFC3339)
		if err := s.accountRepo.UpdateExtra(ctx, accountID, extraUpdates); err != nil {
			slog.Warn("sync_active_to_passive_failed", "account_id", accountID, "error", err)
		}
	}

	// 5h ResetsAt 必须回写到 SessionWindowEnd column，estimateSetupTokenUsage
	// 读这个字段作为窗口结束时间；只塞 Extra 会让 UI 一直拿到上个窗口的过期时间。
	if usage.FiveHour != nil && usage.FiveHour.ResetsAt != nil {
		if err := s.accountRepo.UpdateSessionWindowEnd(ctx, accountID, *usage.FiveHour.ResetsAt); err != nil {
			slog.Warn("sync_active_to_passive_session_window_end_failed", "account_id", accountID, "error", err)
		}
	}
}

func (s *AccountUsageService) getOpenAIUsage(ctx context.Context, account *Account, force bool) (*UsageInfo, error) {
	now := time.Now()
	usage := &UsageInfo{UpdatedAt: &now}

	if account == nil {
		return usage, nil
	}
	populateOpenAIImageRouteUsageInfo(usage, account)

	if openAICodexSnapshotIdentityTrusted(account) {
		if progress := buildCodexUsageProgressFromExtra(account.Extra, "5h", now); progress != nil {
			usage.FiveHour = progress
		}
		if progress := buildCodexUsageProgressFromExtra(account.Extra, "7d", now); progress != nil {
			usage.SevenDay = progress
		}
	}

	if (force || shouldRefreshOpenAICodexSnapshot(account, usage, now)) && s.shouldProbeOpenAICodexSnapshot(account.ID, now, force) {
		if account.IsShadow() {
			// Spark shadow accounts fetch usage from /wham/usage (bengalfox channel)
			// via the shared OpenAIQuotaService, which resolves credentials from the
			// parent account.  The result is written to the shadow row's own codex_*
			// Extra keys and immediately reflected in the returned UsageInfo.
			if s.openAIQuotaService != nil {
				if quotaUsage, err := s.openAIQuotaService.QueryUsage(ctx, account.ID); err == nil {
					if updates := buildCodexSparkWindowExtraUpdates(quotaUsage, now); len(updates) > 0 {
						mergeAccountExtra(account, updates)
						s.persistOpenAICodexProbeSnapshot(account.ID, updates)
						if usage.UpdatedAt == nil {
							usage.UpdatedAt = &now
						}
						applyExtraToUsage(usage, account.Extra, now)
					}
				}
			}
		} else {
			if updates, err := s.probeOpenAICodexSnapshot(ctx, account); err == nil && len(updates) > 0 {
				mergeAccountExtra(account, updates)
				if usage.UpdatedAt == nil {
					usage.UpdatedAt = &now
				}
				applyExtraToUsage(usage, account.Extra, now)
			}
		}
	}

	if s.usageLogRepo == nil {
		return usage, nil
	}

	if stats, err := s.usageLogRepo.GetAccountWindowStats(ctx, account.ID, codexWindowStatsStart(usage.FiveHour, 5*time.Hour, now)); err == nil {
		if usage.FiveHour == nil {
			usage.FiveHour = &UsageProgress{Utilization: 0}
		}
		usage.FiveHour.WindowStats = windowStatsFromAccountStats(stats)
	}

	if stats, err := s.usageLogRepo.GetAccountWindowStats(ctx, account.ID, codexWindowStatsStart(usage.SevenDay, 7*24*time.Hour, now)); err == nil {
		if usage.SevenDay == nil {
			usage.SevenDay = &UsageProgress{Utilization: 0}
		}
		usage.SevenDay.WindowStats = windowStatsFromAccountStats(stats)
	}

	usage.OpenAIImageCodexFiveHour = buildOpenAIImageCodexRouteUsageProgressFromExtra(account.Extra, "5h", now)
	usage.OpenAIImageCodexSevenDay = buildOpenAIImageCodexRouteUsageProgressFromExtra(account.Extra, "7d", now)
	if usage.OpenAIImageCodexFiveHour != nil {
		if imageStats, err := s.getOpenAIImageWindowStatsByRequestType(ctx, account.ID, now.Add(-5*time.Hour), RequestTypeImage); err == nil && imageStats != nil {
			usage.OpenAIImageCodexFiveHour.WindowStats = imageStats
			usage.OpenAIImageCodexFiveHour.UsedRequests = imageStats.Requests
		}
	} else if imageStats, err := s.getOpenAIImageWindowStatsByRequestType(ctx, account.ID, now.Add(-5*time.Hour), RequestTypeImage); err == nil && imageStats != nil && imageStats.Requests > 0 {
		usage.OpenAIImageCodexFiveHour = &UsageProgress{
			Utilization:  0,
			WindowStats:  imageStats,
			UsedRequests: imageStats.Requests,
		}
	}
	if usage.OpenAIImageCodexSevenDay != nil {
		if imageStats, err := s.getOpenAIImageWindowStatsByRequestType(ctx, account.ID, now.Add(-7*24*time.Hour), RequestTypeImage); err == nil && imageStats != nil {
			usage.OpenAIImageCodexSevenDay.WindowStats = imageStats
			usage.OpenAIImageCodexSevenDay.UsedRequests = imageStats.Requests
		}
	} else if imageStats, err := s.getOpenAIImageWindowStatsByRequestType(ctx, account.ID, now.Add(-7*24*time.Hour), RequestTypeImage); err == nil && imageStats != nil && imageStats.Requests > 0 {
		usage.OpenAIImageCodexSevenDay = &UsageProgress{
			Utilization:  0,
			WindowStats:  imageStats,
			UsedRequests: imageStats.Requests,
		}
	}
	return usage, nil
}

func populateOpenAIImageRouteUsageInfo(usage *UsageInfo, account *Account) {
	if usage == nil || account == nil || !account.IsOpenAIOAuth() {
		return
	}
	codexSupported := account.SupportsOpenAIImageRoute(GroupImageGenerationRouteCodex)
	usage.OpenAIImageCodexSupported = &codexSupported
	usage.OpenAIImagePlanType = account.GetOpenAIPlanType()
	usage.OpenAIImageWorkspaceName = account.GetOpenAIWorkspaceName()
	if !codexSupported {
		if account.IsOpenAIFreePlan() {
			usage.OpenAIImageCodexReason = "free_plan_not_supported"
		} else {
			usage.OpenAIImageCodexReason = "unsupported_account_type"
		}
	}
}

func cloneUsageProgress(progress *UsageProgress) *UsageProgress {
	if progress == nil {
		return nil
	}
	cloned := *progress
	if progress.WindowStats != nil {
		statsCopy := *progress.WindowStats
		cloned.WindowStats = &statsCopy
	}
	return &cloned
}

func buildOpenAIImageRouteUsageProgressFromExtra(extra map[string]any, route string, now time.Time) *UsageProgress {
	prefix := openAIImageRouteExtraPrefix(route)
	if prefix == "" || len(extra) == 0 {
		return nil
	}
	progress := &UsageProgress{}
	hasData := false
	if resetAtRaw, ok := extra[prefix+"_rate_limit_reset_at"]; ok {
		if resetAt, err := parseTime(fmt.Sprint(resetAtRaw)); err == nil {
			progress.ResetsAt = &resetAt
			progress.RemainingSeconds = int(time.Until(resetAt).Seconds())
			if progress.RemainingSeconds < 0 {
				progress.RemainingSeconds = 0
			}
			if now.Before(resetAt) {
				progress.Utilization = 100
			}
			hasData = true
		}
	}
	if usedRequests := int64(parseExtraInt(extra[prefix+"_used_requests"])); usedRequests > 0 {
		progress.UsedRequests = usedRequests
		hasData = true
	}
	if !hasData {
		return nil
	}
	return progress
}

func buildOpenAIImageCodexRouteUsageProgressFromExtra(extra map[string]any, window string, now time.Time) *UsageProgress {
	routeLimited := buildOpenAIImageRouteUsageProgressFromExtra(extra, GroupImageGenerationRouteCodex, now)
	if routeLimited == nil {
		return nil
	}
	return cloneUsageProgress(routeLimited)
}

func (s *AccountUsageService) getOpenAIImageWindowStatsByRequestType(ctx context.Context, accountID int64, startTime time.Time, requestType RequestType) (*WindowStats, error) {
	if s == nil || s.usageLogRepo == nil || accountID <= 0 {
		return nil, nil
	}
	requestTypeValue := int16(requestType)
	stats, err := s.usageLogRepo.GetModelStatsWithFilters(ctx, startTime, time.Now(), 0, 0, accountID, 0, &requestTypeValue, nil, nil, "")
	if err != nil {
		return nil, err
	}
	windowStats := &WindowStats{}
	for _, item := range stats {
		windowStats.Requests += item.Requests
		windowStats.Tokens += item.TotalTokens
		windowStats.Cost += item.AccountCost
		windowStats.StandardCost += item.Cost
		windowStats.UserCost += item.ActualCost
	}
	return windowStats, nil
}

func shouldRefreshOpenAICodexSnapshot(account *Account, usage *UsageInfo, now time.Time) bool {
	if account == nil {
		return false
	}
	if usage == nil {
		return true
	}
	if usage.FiveHour == nil || usage.SevenDay == nil {
		return true
	}
	if account.IsRateLimited() {
		return true
	}
	return isOpenAICodexSnapshotStale(account, now)
}

func isOpenAICodexSnapshotStale(account *Account, now time.Time) bool {
	if account == nil || !account.IsOpenAIOAuth() {
		return false
	}
	// 普通账号的 codex 刷新走 probe(/responses 头),要求 WSv2;但 spark 影子走 QueryUsage
	// (/wham/usage body 的 codex_bengalfox),与 WSv2 无关——不能用 WSv2 门控其 staleness,否则首刷后
	// codex_5h/7d 已存在→staleness 恒 false→spark 窗口永久冻结(外审第9轮 P1)。影子改按
	// codex_usage_updated_at TTL 判定;实际查询频率仍由 shouldProbeOpenAICodexSnapshot 的缓存 TTL 节流。
	if !account.IsShadow() && !account.IsOpenAIResponsesWebSocketV2Enabled() {
		return false
	}
	if account.Extra == nil {
		return true
	}
	raw, ok := account.Extra["codex_usage_updated_at"]
	if !ok {
		return true
	}
	ts, err := parseTime(fmt.Sprint(raw))
	if err != nil {
		return true
	}
	return now.Sub(ts) >= openAIProbeCacheTTL
}

func (s *AccountUsageService) shouldProbeOpenAICodexSnapshot(accountID int64, now time.Time, force ...bool) bool {
	if s == nil || s.cache == nil || accountID <= 0 {
		return true
	}
	forceProbe := len(force) > 0 && force[0]
	if !forceProbe {
		if cached, ok := s.cache.openAIProbeCache.Load(accountID); ok {
			if ts, ok := cached.(time.Time); ok && now.Sub(ts) < openAIProbeCacheTTL {
				return false
			}
		}
	}
	s.cache.openAIProbeCache.Store(accountID, now)
	return true
}

func (s *AccountUsageService) probeOpenAICodexSnapshot(ctx context.Context, account *Account) (map[string]any, error) {
	if account == nil || !account.IsOAuth() {
		return nil, nil
	}
	accessToken := account.GetOpenAIAccessToken()
	if accessToken == "" {
		return nil, fmt.Errorf("no access token available")
	}
	modelID := openaipkg.DefaultTestModel
	payload := createOpenAITestPayload(modelID, true)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal openai probe payload: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, chatgptCodexURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("create openai probe request: %w", err)
	}
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("Originator", "codex_cli_rs")
	req.Header.Set("Version", openAICodexProbeVersion)
	req.Header.Set("User-Agent", codexCLIUserAgent)
	if s.identityCache != nil {
		if fp, fpErr := s.identityCache.GetFingerprint(reqCtx, account.ID); fpErr == nil && fp != nil && strings.TrimSpace(fp.UserAgent) != "" {
			req.Header.Set("User-Agent", strings.TrimSpace(fp.UserAgent))
		}
	}
	setOpenAIChatGPTAccountHeaders(req.Header, account)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	client, err := httppool.GetClient(httppool.Options{
		ProxyURL:              proxyURL,
		Timeout:               15 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("build openai probe client: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai codex probe request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	updates, err := extractOpenAICodexProbeUpdates(resp)
	if err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		s.persistOpenAICodexProbeSnapshot(account.ID, updates)
		return updates, nil
	}
	return nil, nil
}

func (s *AccountUsageService) persistOpenAICodexProbeSnapshot(accountID int64, updates map[string]any) {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return
	}
	if len(updates) == 0 {
		return
	}

	go func() {
		updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer updateCancel()
		_ = s.accountRepo.UpdateExtra(updateCtx, accountID, updates)
	}()
}

func extractOpenAICodexProbeUpdates(resp *http.Response) (map[string]any, error) {
	if resp == nil {
		return nil, nil
	}
	if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
		return buildCodexUsageExtraUpdates(snapshot, time.Now()), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai codex probe returned status %d", resp.StatusCode)
	}
	return nil, nil
}

func mergeAccountExtra(account *Account, updates map[string]any) {
	if account == nil || len(updates) == 0 {
		return
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any, len(updates))
	}
	for k, v := range updates {
		account.Extra[k] = v
	}
}

// applyExtraToUsage rebuilds the codex 5h/7d windows in usage from the
// account's Extra map. Called after mergeAccountExtra to make the in-memory
// UsageInfo consistent with the just-persisted Extra values.
func applyExtraToUsage(usage *UsageInfo, extra map[string]any, now time.Time) {
	if usage == nil {
		return
	}
	if progress := buildCodexUsageProgressFromExtra(extra, "5h", now); progress != nil {
		usage.FiveHour = progress
	}
	if progress := buildCodexUsageProgressFromExtra(extra, "7d", now); progress != nil {
		usage.SevenDay = progress
	}
}

func buildKiroSchedulerSnapshotExtraUpdates(info *UsageInfo) map[string]any {
	if info == nil || info.KiroQuota == nil {
		return nil
	}
	if info.Error != "" || info.ErrorCode != "" || info.IsForbidden || info.NeedsReauth {
		return nil
	}

	updatedAt := time.Now().UTC()
	if info.UpdatedAt != nil {
		updatedAt = info.UpdatedAt.UTC()
	}

	updates := map[string]any{
		"kiro_sched_utilization":      info.KiroQuota.Utilization,
		"kiro_sched_usage_updated_at": updatedAt.Format(time.RFC3339),
	}
	if info.KiroQuota.ResetsAt != nil {
		updates["kiro_sched_reset_at"] = info.KiroQuota.ResetsAt.UTC().Format(time.RFC3339)
	}

	return updates
}

func (s *AccountUsageService) persistKiroSchedulerSnapshot(ctx context.Context, account *Account, info *UsageInfo) {
	s.persistSchedulerSnapshotExtra(ctx, account, buildKiroSchedulerSnapshotExtraUpdates(info), "kiro")
}

func (s *AccountUsageService) persistAntigravitySchedulerSnapshot(ctx context.Context, account *Account, info *UsageInfo) {
	s.persistSchedulerSnapshotExtra(ctx, account, buildAntigravitySchedulerSnapshotExtraUpdates(info), "antigravity")
}

func (s *AccountUsageService) persistSchedulerSnapshotExtra(ctx context.Context, account *Account, updates map[string]any, platform string) {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 || len(updates) == 0 {
		return
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
		slog.Warn("persist_scheduler_snapshot_failed", "platform", platform, "account_id", account.ID, "error", err)
		return
	}
	mergeAccountExtra(account, updates)
}

func (s *AccountUsageService) getGeminiUsage(ctx context.Context, account *Account) (*UsageInfo, error) {
	now := time.Now()
	usage := &UsageInfo{
		UpdatedAt: &now,
	}

	enrichGeminiUsageWithStoredStatus(usage, account)

	if s.geminiQuotaService == nil || s.usageLogRepo == nil {
		return usage, nil
	}

	quota, ok := s.geminiQuotaService.QuotaForAccount(ctx, account)
	if !ok {
		return usage, nil
	}

	dayStart := geminiDailyWindowStart(now)
	stats, err := s.usageLogRepo.GetModelStatsWithFilters(ctx, dayStart, now, 0, 0, account.ID, 0, nil, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("get gemini usage stats failed: %w", err)
	}

	dayTotals := geminiAggregateUsage(stats)
	dailyResetAt := geminiDailyResetTime(now)

	// Daily window (RPD)
	if quota.SharedRPD > 0 {
		totalReq := dayTotals.ProRequests + dayTotals.FlashRequests
		totalTokens := dayTotals.ProTokens + dayTotals.FlashTokens
		totalCost := dayTotals.ProCost + dayTotals.FlashCost
		usage.GeminiSharedDaily = buildGeminiUsageProgress(totalReq, quota.SharedRPD, dailyResetAt, totalTokens, totalCost, now)
		usage.GeminiProDaily = buildGeminiUsageProgress(dayTotals.ProRequests, quota.SharedRPD, dailyResetAt, dayTotals.ProTokens, dayTotals.ProCost, now)
		usage.GeminiFlashDaily = buildGeminiUsageProgress(dayTotals.FlashRequests, quota.SharedRPD, dailyResetAt, dayTotals.FlashTokens, dayTotals.FlashCost, now)
	} else {
		usage.GeminiProDaily = buildGeminiUsageProgress(dayTotals.ProRequests, quota.ProRPD, dailyResetAt, dayTotals.ProTokens, dayTotals.ProCost, now)
		usage.GeminiFlashDaily = buildGeminiUsageProgress(dayTotals.FlashRequests, quota.FlashRPD, dailyResetAt, dayTotals.FlashTokens, dayTotals.FlashCost, now)
	}

	// Minute window (RPM) - fixed-window approximation: current minute [truncate(now), truncate(now)+1m)
	minuteStart := now.Truncate(time.Minute)
	minuteResetAt := minuteStart.Add(time.Minute)
	minuteStats, err := s.usageLogRepo.GetModelStatsWithFilters(ctx, minuteStart, now, 0, 0, account.ID, 0, nil, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("get gemini minute usage stats failed: %w", err)
	}
	minuteTotals := geminiAggregateUsage(minuteStats)

	if quota.SharedRPM > 0 {
		totalReq := minuteTotals.ProRequests + minuteTotals.FlashRequests
		totalTokens := minuteTotals.ProTokens + minuteTotals.FlashTokens
		totalCost := minuteTotals.ProCost + minuteTotals.FlashCost
		usage.GeminiSharedMinute = buildGeminiUsageProgress(totalReq, quota.SharedRPM, minuteResetAt, totalTokens, totalCost, now)
		usage.GeminiProMinute = buildGeminiUsageProgress(minuteTotals.ProRequests, quota.SharedRPM, minuteResetAt, minuteTotals.ProTokens, minuteTotals.ProCost, now)
		usage.GeminiFlashMinute = buildGeminiUsageProgress(minuteTotals.FlashRequests, quota.SharedRPM, minuteResetAt, minuteTotals.FlashTokens, minuteTotals.FlashCost, now)
	} else {
		usage.GeminiProMinute = buildGeminiUsageProgress(minuteTotals.ProRequests, quota.ProRPM, minuteResetAt, minuteTotals.ProTokens, minuteTotals.ProCost, now)
		usage.GeminiFlashMinute = buildGeminiUsageProgress(minuteTotals.FlashRequests, quota.FlashRPM, minuteResetAt, minuteTotals.FlashTokens, minuteTotals.FlashCost, now)
	}

	applyGeminiQuotaSnapshotFallback(usage, account, now)

	return usage, nil
}

// getAntigravityUsage 获取 Antigravity 账户额度
func (s *AccountUsageService) getAntigravityUsage(ctx context.Context, account *Account) (*UsageInfo, error) {
	if s.antigravityQuotaFetcher == nil || !s.antigravityQuotaFetcher.CanFetch(account) {
		now := time.Now()
		return &UsageInfo{UpdatedAt: &now}, nil
	}

	// 1. 检查缓存
	if cached, ok := s.cache.antigravityCache.Load(account.ID); ok {
		if cache, ok := cached.(*antigravityUsageCache); ok {
			ttl := antigravityCacheTTL(cache.usageInfo)
			if time.Since(cache.timestamp) < ttl {
				usage := cache.usageInfo
				if usage.FiveHour != nil && usage.FiveHour.ResetsAt != nil {
					usage.FiveHour.RemainingSeconds = int(time.Until(*usage.FiveHour.ResetsAt).Seconds())
				}
				return usage, nil
			}
		}
	}

	// 2. singleflight 防止并发击穿
	flightKey := fmt.Sprintf("ag-usage:%d", account.ID)
	result, flightErr, _ := s.cache.antigravityFlight.Do(flightKey, func() (any, error) {
		// 再次检查缓存（等待期间可能已被填充）
		if cached, ok := s.cache.antigravityCache.Load(account.ID); ok {
			if cache, ok := cached.(*antigravityUsageCache); ok {
				ttl := antigravityCacheTTL(cache.usageInfo)
				if time.Since(cache.timestamp) < ttl {
					usage := cache.usageInfo
					// 重新计算 RemainingSeconds，避免返回过时的剩余秒数
					recalcAntigravityRemainingSeconds(usage)
					return usage, nil
				}
			}
		}

		// 使用独立 context，避免调用方 cancel 导致所有共享 flight 的请求失败
		fetchCtx, fetchCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer fetchCancel()

		proxyURL := s.antigravityQuotaFetcher.GetProxyURL(fetchCtx, account)
		fetchResult, err := s.antigravityQuotaFetcher.FetchQuota(fetchCtx, account, proxyURL)
		if err != nil {
			degraded := buildAntigravityDegradedUsage(err)
			enrichUsageWithAccountError(degraded, account)
			s.cache.antigravityCache.Store(account.ID, &antigravityUsageCache{
				usageInfo: degraded,
				timestamp: time.Now(),
			})
			return degraded, nil
		}

		enrichUsageWithAccountError(fetchResult.UsageInfo, account)
		s.persistAntigravitySchedulerSnapshot(fetchCtx, account, fetchResult.UsageInfo)
		s.cache.antigravityCache.Store(account.ID, &antigravityUsageCache{
			usageInfo: fetchResult.UsageInfo,
			timestamp: time.Now(),
		})
		return fetchResult.UsageInfo, nil
	})

	if flightErr != nil {
		return nil, flightErr
	}
	usage, ok := result.(*UsageInfo)
	if !ok || usage == nil {
		now := time.Now()
		return &UsageInfo{UpdatedAt: &now}, nil
	}
	return usage, nil
}

func (s *AccountUsageService) getGrokUsage(ctx context.Context, account *Account) (*UsageInfo, error) {
	if s.grokQuotaFetcher == nil {
		now := time.Now()
		return &UsageInfo{UpdatedAt: &now}, nil
	}
	usage := s.grokQuotaFetcher.BuildUsageInfo(account)
	if usage.GrokQuotaSnapshotState == "" {
		if usage.ErrorCode == "quota_unknown" {
			usage.GrokQuotaSnapshotState = "unknown_until_first_response"
		} else {
			usage.GrokQuotaSnapshotState = "observed"
		}
	}

	if s.usageLogRepo != nil && account != nil {
		if stats, err := s.usageLogRepo.GetAccountWindowStats(ctx, account.ID, time.Now().Add(-7*24*time.Hour)); err == nil && stats != nil {
			if usage.SevenDay == nil {
				usage.SevenDay = &UsageProgress{Utilization: 0}
			}
			usage.SevenDay.WindowStats = windowStatsFromAccountStats(stats)
		}
	}
	if account != nil {
		if snapshot, err := grokQuotaSnapshotFromExtra(account.Extra); err == nil {
			if progress := grokQuotaSnapshotSevenDayProgress(snapshot); progress != nil {
				if usage.SevenDay == nil {
					usage.SevenDay = progress
				} else {
					usage.SevenDay.Utilization = progress.Utilization
					usage.SevenDay.ResetsAt = progress.ResetsAt
					usage.SevenDay.RemainingSeconds = progress.RemainingSeconds
				}
			}
		}
	}

	enrichUsageWithAccountError(usage, account)
	return usage, nil
}

func grokQuotaSnapshotSevenDayProgress(snapshot *xai.QuotaSnapshot) *UsageProgress {
	utilization, resetAt, ok := grokSnapshotUtilization(snapshot)
	if !ok {
		return nil
	}
	progress := &UsageProgress{Utilization: utilization}
	if resetAt != nil {
		progress.ResetsAt = resetAt
		if seconds := int(time.Until(*resetAt).Seconds()); seconds > 0 {
			progress.RemainingSeconds = seconds
		}
	}
	return progress
}

// recalcAntigravityRemainingSeconds 重新计算 Antigravity UsageInfo 中各窗口的 RemainingSeconds
// 用于从缓存取出时更新倒计时，避免返回过时的剩余秒数
func recalcAntigravityRemainingSeconds(info *UsageInfo) {
	if info == nil {
		return
	}
	if info.FiveHour != nil && info.FiveHour.ResetsAt != nil {
		remaining := int(time.Until(*info.FiveHour.ResetsAt).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		info.FiveHour.RemainingSeconds = remaining
	}
}

// antigravityCacheTTL 根据 UsageInfo 内容决定缓存 TTL
// 403 forbidden 状态稳定，缓存与成功相同（3 分钟）；
// 其他错误（401/网络）可能快速恢复，缓存 1 分钟。
func antigravityCacheTTL(info *UsageInfo) time.Duration {
	if info == nil {
		return antigravityErrorTTL
	}
	if info.IsForbidden {
		return apiCacheTTL // 封号/验证状态不会很快变
	}
	if info.ErrorCode != "" || info.Error != "" {
		return antigravityErrorTTL
	}
	return apiCacheTTL
}

// buildAntigravityDegradedUsage 从 FetchQuota 错误构建降级 UsageInfo
func buildAntigravityDegradedUsage(err error) *UsageInfo {
	now := time.Now()
	errMsg := fmt.Sprintf("usage API error: %v", err)
	slog.Warn("antigravity usage fetch failed, returning degraded response", "error", err)

	info := &UsageInfo{
		UpdatedAt: &now,
		Error:     errMsg,
	}

	// 从错误信息推断 error_code 和状态标记
	// 错误格式来自 antigravity/client.go: "fetchAvailableModels 失败 (HTTP %d): ..."
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "HTTP 401") ||
		strings.Contains(errStr, "UNAUTHENTICATED") ||
		strings.Contains(errStr, "invalid_grant"):
		info.ErrorCode = errorCodeUnauthenticated
		info.NeedsReauth = true
	case strings.Contains(errStr, "HTTP 429"):
		info.ErrorCode = errorCodeRateLimited
	default:
		info.ErrorCode = errorCodeNetworkError
	}

	return info
}

// enrichUsageWithAccountError 结合账号错误状态修正 UsageInfo
// 场景 1（成功路径）：FetchAvailableModels 正常返回，但账号已因 403 被标记为 error，
//
//	需要在正常 usage 数据上附加 forbidden/validation 信息。
//
// 场景 2（降级路径）：被封号的账号 OAuth token 失效，FetchAvailableModels 返回 401，
//
//	降级逻辑设置了 needs_reauth，但账号实际是 403 封号/需验证，需覆盖为正确状态。
func enrichUsageWithAccountError(info *UsageInfo, account *Account) {
	if info == nil || account == nil || account.Status != StatusError {
		return
	}
	msg := strings.ToLower(account.ErrorMessage)
	if !strings.Contains(msg, "403") && !strings.Contains(msg, "forbidden") &&
		!strings.Contains(msg, "violation") && !strings.Contains(msg, "validation") {
		return
	}
	fbType := classifyForbiddenType(account.ErrorMessage)
	info.IsForbidden = true
	info.ForbiddenType = fbType
	info.ForbiddenReason = account.ErrorMessage
	info.NeedsVerify = fbType == forbiddenTypeValidation
	info.IsBanned = fbType == forbiddenTypeViolation
	info.ValidationURL = extractValidationURL(account.ErrorMessage)
	info.ErrorCode = errorCodeForbidden
	info.NeedsReauth = false
}

// addWindowStats 为 usage 数据添加窗口期统计
// 使用独立缓存（1 分钟），与 API 缓存分离
func (s *AccountUsageService) addWindowStats(ctx context.Context, account *Account, usage *UsageInfo) {
	// 修复：即使 FiveHour 为 nil，也要尝试获取统计数据
	// 因为 SevenDay/SevenDaySonnet 可能需要
	if usage.FiveHour == nil && usage.SevenDay == nil && usage.SevenDaySonnet == nil {
		return
	}

	// 检查窗口统计缓存（1 分钟）
	var windowStats *WindowStats
	if cached, ok := s.cache.windowStatsCache.Load(account.ID); ok {
		if cache, ok := cached.(*windowStatsCache); ok && time.Since(cache.timestamp) < windowStatsCacheTTL {
			windowStats = cache.stats
		}
	}

	// 如果没有缓存，从数据库查询
	if windowStats == nil {
		// 使用统一的窗口开始时间计算逻辑（考虑窗口过期情况）
		startTime := account.GetCurrentWindowStartTime()

		stats, err := s.usageLogRepo.GetAccountWindowStats(ctx, account.ID, startTime)
		if err != nil {
			log.Printf("Failed to get window stats for account %d: %v", account.ID, err)
			return
		}
		if stats == nil {
			stats = &usagestats.AccountStats{}
		}

		windowStats = &WindowStats{
			Requests:     stats.Requests,
			Tokens:       stats.Tokens,
			Cost:         stats.Cost,
			StandardCost: stats.StandardCost,
			UserCost:     stats.UserCost,
		}

		// 缓存窗口统计（1 分钟）
		s.cache.windowStatsCache.Store(account.ID, &windowStatsCache{
			stats:     windowStats,
			timestamp: time.Now(),
		})
	}

	// 为 FiveHour 添加 WindowStats（5h 窗口统计）
	if usage.FiveHour != nil {
		usage.FiveHour.WindowStats = windowStats
	}
}

// GetTodayStats 获取账号今日统计
func (s *AccountUsageService) GetTodayStats(ctx context.Context, accountID int64) (*WindowStats, error) {
	stats, err := s.usageLogRepo.GetAccountTodayStats(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get today stats failed: %w", err)
	}

	return &WindowStats{
		Requests:     stats.Requests,
		Tokens:       stats.Tokens,
		Cost:         stats.Cost,
		StandardCost: stats.StandardCost,
		UserCost:     stats.UserCost,
	}, nil
}

// GetTodayStatsBatch 批量获取账号今日统计，优先走批量 SQL，失败时回退单账号查询。
func (s *AccountUsageService) GetTodayStatsBatch(ctx context.Context, accountIDs []int64) (map[int64]*WindowStats, error) {
	uniqueIDs := make([]int64, 0, len(accountIDs))
	seen := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID <= 0 {
			continue
		}
		if _, exists := seen[accountID]; exists {
			continue
		}
		seen[accountID] = struct{}{}
		uniqueIDs = append(uniqueIDs, accountID)
	}

	result := make(map[int64]*WindowStats, len(uniqueIDs))
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	startTime := timezone.Today()
	if batchReader, ok := s.usageLogRepo.(accountWindowStatsBatchReader); ok {
		statsByAccount, err := batchReader.GetAccountWindowStatsBatch(ctx, uniqueIDs, startTime)
		if err == nil {
			for _, accountID := range uniqueIDs {
				result[accountID] = windowStatsFromAccountStats(statsByAccount[accountID])
			}
			return result, nil
		}
	}

	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)

	for _, accountID := range uniqueIDs {
		id := accountID
		g.Go(func() error {
			stats, err := s.usageLogRepo.GetAccountWindowStats(gctx, id, startTime)
			if err != nil {
				return nil
			}
			mu.Lock()
			result[id] = windowStatsFromAccountStats(stats)
			mu.Unlock()
			return nil
		})
	}

	_ = g.Wait()

	for _, accountID := range uniqueIDs {
		if _, ok := result[accountID]; !ok {
			result[accountID] = &WindowStats{}
		}
	}
	return result, nil
}

func windowStatsFromAccountStats(stats *usagestats.AccountStats) *WindowStats {
	if stats == nil {
		return &WindowStats{}
	}
	return &WindowStats{
		Requests:     stats.Requests,
		Tokens:       stats.Tokens,
		Cost:         stats.Cost,
		StandardCost: stats.StandardCost,
		UserCost:     stats.UserCost,
	}
}

func buildCodexUsageProgressFromExtra(extra map[string]any, window string, now time.Time) *UsageProgress {
	if len(extra) == 0 {
		return nil
	}

	var (
		usedPercentKey string
		resetAfterKey  string
		resetAtKey     string
	)

	switch window {
	case "5h":
		usedPercentKey = "codex_5h_used_percent"
		resetAfterKey = "codex_5h_reset_after_seconds"
		resetAtKey = "codex_5h_reset_at"
	case "7d":
		usedPercentKey = "codex_7d_used_percent"
		resetAfterKey = "codex_7d_reset_after_seconds"
		resetAtKey = "codex_7d_reset_at"
	default:
		return nil
	}

	usedRaw, ok := extra[usedPercentKey]
	if !ok {
		return nil
	}

	progress := &UsageProgress{Utilization: parseExtraFloat64(usedRaw)}
	if resetAtRaw, ok := extra[resetAtKey]; ok {
		if resetAt, err := parseTime(fmt.Sprint(resetAtRaw)); err == nil {
			progress.ResetsAt = &resetAt
			progress.RemainingSeconds = int(time.Until(resetAt).Seconds())
			if progress.RemainingSeconds < 0 {
				progress.RemainingSeconds = 0
			}
		}
	}
	if progress.ResetsAt == nil {
		if resetAfterSeconds := parseExtraInt(extra[resetAfterKey]); resetAfterSeconds > 0 {
			base := now
			if updatedAtRaw, ok := extra["codex_usage_updated_at"]; ok {
				if updatedAt, err := parseTime(fmt.Sprint(updatedAtRaw)); err == nil {
					base = updatedAt
				}
			}
			resetAt := base.Add(time.Duration(resetAfterSeconds) * time.Second)
			progress.ResetsAt = &resetAt
			progress.RemainingSeconds = int(time.Until(resetAt).Seconds())
			if progress.RemainingSeconds < 0 {
				progress.RemainingSeconds = 0
			}
		}
	}

	// 窗口已过期（resetAt 在 now 之前）→ 额度已重置，归零
	if progress.ResetsAt != nil && !now.Before(*progress.ResetsAt) {
		progress.Utilization = 0
	}

	return progress
}

func codexWindowStatsStart(progress *UsageProgress, fallbackWindow time.Duration, now time.Time) time.Time {
	if progress != nil && progress.ResetsAt != nil && now.Before(*progress.ResetsAt) {
		return progress.ResetsAt.Add(-fallbackWindow)
	}
	return now.Add(-fallbackWindow)
}

func (s *AccountUsageService) GetAccountUsageStats(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error) {
	stats, err := s.usageLogRepo.GetAccountUsageStats(ctx, accountID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get account usage stats failed: %w", err)
	}
	return stats, nil
}

// fetchOAuthUsageRaw 从 Anthropic API 获取原始响应（不构建 UsageInfo）
// 如果账号开启了 TLS 指纹，则使用 TLS 指纹伪装
// 如果有缓存的 Fingerprint，则使用缓存的 User-Agent 等信息
func (s *AccountUsageService) fetchOAuthUsageRaw(ctx context.Context, account *Account) (*ClaudeUsageResponse, error) {
	accessToken := account.GetCredential("access_token")
	if accessToken == "" {
		return nil, fmt.Errorf("no access token available")
	}

	var proxyURL string
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	// 构建完整的选项
	opts := &ClaudeUsageFetchOptions{
		AccessToken: accessToken,
		ProxyURL:    proxyURL,
		AccountID:   account.ID,
		TLSProfile:  s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"),
	}

	// 尝试获取缓存的 Fingerprint（包含 User-Agent 等信息）
	if s.identityCache != nil {
		if fp, err := s.identityCache.GetFingerprint(ctx, account.ID); err == nil && fp != nil {
			opts.Fingerprint = fp
		}
	}

	return s.usageFetcher.FetchUsageWithOptions(ctx, opts)
}

// parseTime 尝试多种格式解析时间
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

func (s *AccountUsageService) tryClearRecoverableAccountError(ctx context.Context, account *Account) {
	if account == nil || account.Status != StatusError {
		return
	}

	msg := strings.ToLower(strings.TrimSpace(account.ErrorMessage))
	if msg == "" {
		return
	}

	isGeminiProjectConfigError := account.Platform == PlatformGemini && IsGeminiProjectConfigurationErrorMessage(msg)
	if isGeminiProjectConfigError {
		return
	}

	if !strings.Contains(msg, "token refresh failed") &&
		!strings.Contains(msg, "invalid_client") &&
		!strings.Contains(msg, "missing_project_id") &&
		!strings.Contains(msg, "unauthenticated") {
		return
	}

	if err := s.accountRepo.ClearError(ctx, account.ID); err != nil {
		log.Printf("[usage] failed to clear recoverable account error for account %d: %v", account.ID, err)
		return
	}

	account.Status = StatusActive
	account.ErrorMessage = ""
}

// buildUsageInfo 构建UsageInfo
func (s *AccountUsageService) buildUsageInfo(resp *ClaudeUsageResponse, updatedAt *time.Time) *UsageInfo {
	info := &UsageInfo{
		UpdatedAt: updatedAt,
	}

	// 5小时窗口 - 始终创建对象（即使 ResetsAt 为空）
	info.FiveHour = &UsageProgress{
		Utilization: resp.FiveHour.Utilization,
	}
	if resp.FiveHour.ResetsAt != "" {
		if fiveHourReset, err := parseTime(resp.FiveHour.ResetsAt); err == nil {
			info.FiveHour.ResetsAt = &fiveHourReset
			info.FiveHour.RemainingSeconds = int(time.Until(fiveHourReset).Seconds())
		} else {
			log.Printf("Failed to parse FiveHour.ResetsAt: %s, error: %v", resp.FiveHour.ResetsAt, err)
		}
	}

	// 7天窗口
	if resp.SevenDay.ResetsAt != "" {
		if sevenDayReset, err := parseTime(resp.SevenDay.ResetsAt); err == nil {
			info.SevenDay = &UsageProgress{
				Utilization:      resp.SevenDay.Utilization,
				ResetsAt:         &sevenDayReset,
				RemainingSeconds: int(time.Until(sevenDayReset).Seconds()),
			}
		} else {
			log.Printf("Failed to parse SevenDay.ResetsAt: %s, error: %v", resp.SevenDay.ResetsAt, err)
			info.SevenDay = &UsageProgress{
				Utilization: resp.SevenDay.Utilization,
			}
		}
	}

	// 7天Sonnet窗口
	if resp.SevenDaySonnet.ResetsAt != "" {
		if sonnetReset, err := parseTime(resp.SevenDaySonnet.ResetsAt); err == nil {
			info.SevenDaySonnet = &UsageProgress{
				Utilization:      resp.SevenDaySonnet.Utilization,
				ResetsAt:         &sonnetReset,
				RemainingSeconds: int(time.Until(sonnetReset).Seconds()),
			}
		} else {
			log.Printf("Failed to parse SevenDaySonnet.ResetsAt: %s, error: %v", resp.SevenDaySonnet.ResetsAt, err)
			info.SevenDaySonnet = &UsageProgress{
				Utilization: resp.SevenDaySonnet.Utilization,
			}
		}
	}

	return info
}

// estimateSetupTokenUsage 根据session_window推算Setup Token账号的使用量
func (s *AccountUsageService) estimateSetupTokenUsage(account *Account) *UsageInfo {
	info := &UsageInfo{}

	// 如果有session_window信息
	if account.SessionWindowEnd != nil {
		remaining := int(time.Until(*account.SessionWindowEnd).Seconds())
		if remaining < 0 {
			remaining = 0
		}

		// 优先使用响应头中存储的真实 utilization 值（0-1 小数，转为 0-100 百分比）
		var utilization float64
		var found bool
		if stored, ok := account.Extra["session_window_utilization"]; ok {
			switch v := stored.(type) {
			case float64:
				utilization = v * 100
				found = true
			case json.Number:
				if f, err := v.Float64(); err == nil {
					utilization = f * 100
					found = true
				}
			}
		}

		// 如果没有存储的 utilization，回退到状态估算
		if !found {
			switch account.SessionWindowStatus {
			case "rejected":
				utilization = 100.0
			case "allowed_warning":
				utilization = 80.0
			}
		}

		info.FiveHour = &UsageProgress{
			Utilization:      utilization,
			ResetsAt:         account.SessionWindowEnd,
			RemainingSeconds: remaining,
		}

		// 窗口已过期（resetAt 在 now 之前）→ 额度已重置，归零；
		// 与 Codex 分支 buildCodexUsageProgressFromExtra 保持一致，避免
		// UI 在 active poll 没回写 SessionWindowEnd 时渲染矛盾状态。
		if info.FiveHour.ResetsAt != nil && !time.Now().Before(*info.FiveHour.ResetsAt) {
			info.FiveHour.Utilization = 0
			info.FiveHour.ResetsAt = nil
			info.FiveHour.RemainingSeconds = 0
		}
	} else {
		// 没有窗口信息，返回空数据
		info.FiveHour = &UsageProgress{
			Utilization:      0,
			RemainingSeconds: 0,
		}
	}

	// Setup Token无法获取7d数据
	return info
}

func buildGeminiUsageProgress(used, limit int64, resetAt time.Time, tokens int64, cost float64, now time.Time) *UsageProgress {
	// limit <= 0 means "no local quota window" (unknown or unlimited).
	if limit <= 0 {
		return nil
	}
	utilization := (float64(used) / float64(limit)) * 100
	remainingSeconds := int(resetAt.Sub(now).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}
	resetCopy := resetAt
	return &UsageProgress{
		Utilization:      utilization,
		ResetsAt:         &resetCopy,
		RemainingSeconds: remainingSeconds,
		UsedRequests:     used,
		LimitRequests:    limit,
		WindowStats: &WindowStats{
			Requests: used,
			Tokens:   tokens,
			Cost:     cost,
		},
	}
}

func applyGeminiQuotaSnapshotFallback(usage *UsageInfo, account *Account, now time.Time) {
	if usage == nil || account == nil {
		return
	}

	snapshot := parseGeminiQuotaSnapshot(account)
	if snapshot == nil {
		return
	}

	mergeGeminiUsageProgressWithSnapshot(&usage.GeminiProDaily, snapshot.pro, now)
	mergeGeminiUsageProgressWithSnapshot(&usage.GeminiFlashDaily, snapshot.flash, now)

	if shared := higherUtilizationProgress(usage.GeminiProDaily, usage.GeminiFlashDaily); shared != nil {
		if usage.GeminiSharedDaily == nil || shared.Utilization > usage.GeminiSharedDaily.Utilization {
			usage.GeminiSharedDaily = cloneUsageProgress(shared)
		}
	}
}

type geminiQuotaSnapshotWindow struct {
	utilization float64
	resetAt     *time.Time
}

type geminiQuotaSnapshotUsage struct {
	pro   *geminiQuotaSnapshotWindow
	flash *geminiQuotaSnapshotWindow
}

func parseGeminiQuotaSnapshot(account *Account) *geminiQuotaSnapshotUsage {
	if account == nil {
		return nil
	}
	raw, ok := account.Credentials["gemini_usage_raw"]
	if !ok {
		raw = account.Extra["gemini_usage_raw"]
	}

	root, ok := raw.(map[string]any)
	if !ok || len(root) == 0 {
		return nil
	}
	buckets, ok := root["buckets"].([]any)
	if !ok || len(buckets) == 0 {
		return nil
	}

	snapshot := &geminiQuotaSnapshotUsage{}
	for _, item := range buckets {
		bucket, ok := item.(map[string]any)
		if !ok {
			continue
		}

		modelID, _ := bucket["modelId"].(string)
		modelID = strings.ToLower(strings.TrimSpace(modelID))
		if modelID == "" {
			continue
		}

		remainingFraction, ok := parseGeminiSnapshotFloat(bucket["remainingFraction"])
		if !ok {
			continue
		}

		utilization := 100 - (remainingFraction * 100)
		if utilization < 0 {
			utilization = 0
		}
		if utilization > 100 {
			utilization = 100
		}

		window := &geminiQuotaSnapshotWindow{
			utilization: utilization,
			resetAt:     parseGeminiSnapshotTime(bucket["resetTime"]),
		}

		switch {
		case strings.Contains(modelID, "flash"):
			snapshot.flash = pickHigherUtilizationSnapshot(snapshot.flash, window)
		case strings.Contains(modelID, "pro"):
			snapshot.pro = pickHigherUtilizationSnapshot(snapshot.pro, window)
		}
	}

	if snapshot.pro == nil && snapshot.flash == nil {
		return nil
	}
	return snapshot
}

func parseGeminiSnapshotFloat(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func parseGeminiSnapshotTime(raw any) *time.Time {
	switch v := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil
		}
		if ts, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return &ts
		}
	case float64:
		ts := time.Unix(int64(v), 0)
		return &ts
	case int64:
		ts := time.Unix(v, 0)
		return &ts
	}
	return nil
}

func pickHigherUtilizationSnapshot(current, next *geminiQuotaSnapshotWindow) *geminiQuotaSnapshotWindow {
	if next == nil {
		return current
	}
	if current == nil || next.utilization > current.utilization {
		return next
	}
	if next.utilization < current.utilization {
		return current
	}
	if current.resetAt == nil {
		return next
	}
	if next.resetAt == nil {
		return current
	}
	if next.resetAt.Before(*current.resetAt) {
		return next
	}
	return current
}

func mergeGeminiUsageProgressWithSnapshot(target **UsageProgress, snapshot *geminiQuotaSnapshotWindow, now time.Time) {
	if snapshot == nil {
		return
	}
	if *target == nil {
		*target = &UsageProgress{}
	}
	(*target).Utilization = snapshot.utilization
	(*target).ResetsAt = snapshot.resetAt
	if snapshot.resetAt != nil {
		remainingSeconds := int(snapshot.resetAt.Sub(now).Seconds())
		if remainingSeconds < 0 {
			remainingSeconds = 0
		}
		(*target).RemainingSeconds = remainingSeconds
	} else {
		(*target).RemainingSeconds = 0
	}
}

func higherUtilizationProgress(items ...*UsageProgress) *UsageProgress {
	var picked *UsageProgress
	for _, item := range items {
		if item == nil {
			continue
		}
		if picked == nil || item.Utilization > picked.Utilization {
			picked = item
			continue
		}
		if item.Utilization == picked.Utilization {
			if picked.ResetsAt == nil {
				picked = item
				continue
			}
			if item.ResetsAt != nil && item.ResetsAt.Before(*picked.ResetsAt) {
				picked = item
			}
		}
	}
	return picked
}

func enrichGeminiUsageWithStoredStatus(usage *UsageInfo, account *Account) {
	if usage == nil || account == nil {
		return
	}

	status := strings.ToLower(strings.TrimSpace(geminiStoredUsageString(account, "gemini_status")))
	statusReason := strings.TrimSpace(geminiStoredUsageString(account, "gemini_status_reason"))
	if statusReason == "" {
		statusReason = strings.TrimSpace(account.ErrorMessage)
	}

	quotaErr := strings.TrimSpace(geminiStoredUsageString(account, "quota_query_last_error"))
	if quotaErr != "" && usage.Error == "" {
		usage.Error = quotaErr
	}

	reason := statusReason
	if reason == "" {
		reason = usage.Error
	}

	forbiddenType := ""
	switch status {
	case "forbidden":
		forbiddenType = classifyForbiddenType(reason)
	case "validation", "validation_required", "needs_verify":
		forbiddenType = forbiddenTypeValidation
	}
	if forbiddenType == "" && isGeminiStoredForbiddenReason(reason) {
		forbiddenType = classifyForbiddenType(reason)
	}
	if forbiddenType == "" {
		return
	}
	if forbiddenType == forbiddenTypeForbidden && status == "validation_required" {
		forbiddenType = forbiddenTypeValidation
	}
	if reason == "" {
		reason = "Gemini access forbidden"
	}

	usage.IsForbidden = true
	usage.ForbiddenType = forbiddenType
	usage.ForbiddenReason = reason
	usage.ErrorCode = errorCodeForbidden
	usage.NeedsVerify = forbiddenType == forbiddenTypeValidation
	usage.IsBanned = forbiddenType == forbiddenTypeViolation
	if validationURL := extractValidationURL(reason); validationURL != "" {
		usage.ValidationURL = validationURL
	} else if usage.Error != "" {
		usage.ValidationURL = extractValidationURL(usage.Error)
	}
}

func geminiStoredUsageString(account *Account, key string) string {
	if account == nil {
		return ""
	}
	if value := strings.TrimSpace(account.GetCredential(key)); value != "" {
		return value
	}
	return strings.TrimSpace(account.GetExtraString(key))
}

func isGeminiStoredForbiddenReason(reason string) bool {
	lower := strings.ToLower(strings.TrimSpace(reason))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "status 403") ||
		strings.Contains(lower, "permission_denied") ||
		strings.Contains(lower, "forbidden") ||
		strings.Contains(lower, "validation_required") ||
		strings.Contains(lower, "verify your account") ||
		strings.Contains(lower, "validation_url")
}

// GetAccountWindowStats 获取账号在指定时间窗口内的使用统计
// 用于账号列表页面显示当前窗口费用
func (s *AccountUsageService) GetAccountWindowStats(ctx context.Context, accountID int64, startTime time.Time) (*usagestats.AccountStats, error) {
	return s.usageLogRepo.GetAccountWindowStats(ctx, accountID, startTime)
}
