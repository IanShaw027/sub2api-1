package service

import (
	"fmt"
	"strings"
	"time"
)

const (
	BillingTypeBalance      int8 = 0 // 钱包余额
	BillingTypeSubscription int8 = 1 // 订阅套餐
)

type RequestType int16

const (
	RequestTypeUnknown        RequestType = 0
	RequestTypeSync           RequestType = 1
	RequestTypeStream         RequestType = 2
	RequestTypeWSV2           RequestType = 3
	RequestTypeCyberBlocked   RequestType = 4 // cyber_policy 命中（透传但被上游安全策略拒绝）；历史持久化值，禁止复用
	RequestTypeImageWebBridge RequestType = 5
	// RequestTypeCyberBlockedMoved 是曾短暂写入过的 cyber 值；读取/过滤时兼容，写入统一回 4。
	RequestTypeCyberBlockedMoved RequestType = 6
	RequestTypeVideo             RequestType = 7
	RequestTypeImage             RequestType = 8
)

func (t RequestType) IsValid() bool {
	switch t {
	case RequestTypeUnknown, RequestTypeSync, RequestTypeStream, RequestTypeWSV2, RequestTypeCyberBlocked, RequestTypeImageWebBridge, RequestTypeCyberBlockedMoved, RequestTypeVideo, RequestTypeImage:
		return true
	default:
		return false
	}
}

func (t RequestType) Normalize() RequestType {
	if t == RequestTypeCyberBlockedMoved {
		return RequestTypeCyberBlocked
	}
	if t.IsValid() {
		return t
	}
	return RequestTypeUnknown
}

func (t RequestType) String() string {
	switch t.Normalize() {
	case RequestTypeSync:
		return "sync"
	case RequestTypeStream:
		return "stream"
	case RequestTypeWSV2:
		return "ws_v2"
	case RequestTypeImage:
		return "image"
	case RequestTypeImageWebBridge:
		return "image_web_bridge"
	case RequestTypeCyberBlocked:
		return "cyber"
	case RequestTypeVideo:
		return "video"
	default:
		return "unknown"
	}
}

func RequestTypeFromInt16(v int16) RequestType {
	return RequestType(v).Normalize()
}

func ParseUsageRequestType(value string) (RequestType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "unknown":
		return RequestTypeUnknown, nil
	case "sync":
		return RequestTypeSync, nil
	case "stream":
		return RequestTypeStream, nil
	case "ws_v2":
		return RequestTypeWSV2, nil
	case "image":
		return RequestTypeImage, nil
	case "image_web_bridge":
		return RequestTypeImageWebBridge, nil
	case "cyber":
		return RequestTypeCyberBlocked, nil
	case "video":
		return RequestTypeVideo, nil
	default:
		return RequestTypeUnknown, fmt.Errorf("invalid request_type, allowed values: unknown, sync, stream, ws_v2, image, image_web_bridge, cyber, video")
	}
}

func RequestTypeFromLegacy(stream bool, openAIWSMode bool) RequestType {
	if openAIWSMode {
		return RequestTypeWSV2
	}
	if stream {
		return RequestTypeStream
	}
	return RequestTypeSync
}

func ApplyLegacyRequestFields(requestType RequestType, fallbackStream bool, fallbackOpenAIWSMode bool) (stream bool, openAIWSMode bool) {
	switch requestType.Normalize() {
	case RequestTypeSync:
		return false, false
	case RequestTypeStream:
		return true, false
	case RequestTypeWSV2:
		return true, true
	case RequestTypeImage, RequestTypeImageWebBridge, RequestTypeVideo:
		return false, false
	default:
		return fallbackStream, fallbackOpenAIWSMode
	}
}

type UsageLog struct {
	ID        int64
	UserID    int64
	APIKeyID  int64
	AccountID int64
	// Provider snapshots the account platform at request time for stable usage analytics.
	Provider  string
	RequestID string
	Model     string
	// RequestedModel is the client-requested model name recorded for stable user/admin display.
	// Empty should be treated as Model for backward compatibility with historical rows.
	RequestedModel string
	// UpstreamModel is the actual model sent to the upstream provider after mapping.
	// Nil means no mapping was applied (requested model was used as-is).
	UpstreamModel *string
	// ChannelID 渠道 ID
	ChannelID *int64
	// ModelMappingChain 模型映射链，如 "a→b→c"
	ModelMappingChain *string
	// BillingTier 计费层级标签（per_request/image 模式）
	BillingTier *string
	// BillingMode 计费模式：token/image
	BillingMode *string
	// ServiceTier records the OpenAI service tier used for billing, e.g. "priority" / "flex".
	ServiceTier *string
	// ReasoningEffort is the request's reasoning effort level.
	// OpenAI: "low" / "medium" / "high" / "xhigh"; Claude: "low" / "medium" / "high" / "max".
	// Nil means not provided / not applicable.
	ReasoningEffort *string
	// InboundEndpoint is the client-facing API endpoint path, e.g. /v1/chat/completions.
	InboundEndpoint *string
	// UpstreamEndpoint is the normalized upstream endpoint path, e.g. /v1/responses.
	UpstreamEndpoint *string

	GroupID        *int64
	SubscriptionID *int64

	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int

	CacheCreation5mTokens int `gorm:"column:cache_creation_5m_tokens"`
	CacheCreation1hTokens int `gorm:"column:cache_creation_1h_tokens"`

	ImageOutputTokens int
	ImageOutputCost   float64

	InputCost         float64
	OutputCost        float64
	CacheCreationCost float64
	CacheReadCost     float64
	TotalCost         float64
	ActualCost        float64
	RateMultiplier    float64
	// BilledByHigherPricedUpstream marks records where billing compared requested
	// and upstream models and charged using the more expensive upstream price.
	BilledByHigherPricedUpstream bool
	// AccountRateMultiplier 账号计费倍率快照（nil 表示历史数据，按 1.0 处理）
	AccountRateMultiplier *float64
	// AccountStatsCost 账号统计定价预计算费用（nil = 使用默认公式 total_cost × account_rate_multiplier）
	AccountStatsCost *float64

	BillingType  int8
	RequestType  RequestType
	Stream       bool
	OpenAIWSMode bool
	// OpenAIWSProfile 标识上游 WS 连接类别："neutral"/"session_bound"，空表示非 WS 路径。
	OpenAIWSProfile string
	// OpenAIWSConnReused 标识本次请求是否复用了已建立的上游 WS 连接（省去握手）。
	OpenAIWSConnReused bool
	DurationMs         *int
	FirstTokenMs       *int
	UserAgent          *string
	IPAddress          *string

	// Cache TTL Override 标记（管理员强制替换了缓存 TTL 计费）
	CacheTTLOverridden bool

	// 图片生成字段
	ImageCount         int
	ImageSize          *string
	ImageInputSize     *string
	ImageOutputSize    *string
	ImageSizeSource    *string
	ImageSizeBreakdown map[string]int
	MediaType          *string

	CreatedAt time.Time

	User         *User
	APIKey       *APIKey
	Account      *Account
	Group        *Group
	Subscription *UserSubscription
}

func (u *UsageLog) TotalTokens() int {
	return u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens
}

func (u *UsageLog) EffectiveRequestType() RequestType {
	if u == nil {
		return RequestTypeUnknown
	}
	if u.RequestType == RequestTypeCyberBlocked && u.hasImageRequestEvidence() {
		return RequestTypeImage
	}
	if normalized := u.RequestType.Normalize(); normalized != RequestTypeUnknown {
		return normalized
	}
	return RequestTypeFromLegacy(u.Stream, u.OpenAIWSMode)
}

func (u *UsageLog) hasImageRequestEvidence() bool {
	if u == nil {
		return false
	}
	if u.ImageCount > 0 || u.ImageOutputTokens > 0 {
		return true
	}
	if u.BillingMode != nil && strings.EqualFold(strings.TrimSpace(*u.BillingMode), string(BillingModeImage)) {
		return true
	}
	return endpointLooksLikeImageRequest(u.InboundEndpoint) || endpointLooksLikeImageRequest(u.UpstreamEndpoint)
}

func endpointLooksLikeImageRequest(endpoint *string) bool {
	if endpoint == nil {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(*endpoint))
	return strings.Contains(normalized, "/images/") || strings.Contains(normalized, "/images2api/")
}

func (u *UsageLog) SyncRequestTypeAndLegacyFields() {
	if u == nil {
		return
	}
	requestType := u.EffectiveRequestType()
	u.RequestType = requestType
	u.Stream, u.OpenAIWSMode = ApplyLegacyRequestFields(requestType, u.Stream, u.OpenAIWSMode)
}
