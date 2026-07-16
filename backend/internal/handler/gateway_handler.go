package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const gatewayCompatibilityMetricsLogInterval = 1024

var gatewayCompatibilityMetricsLogCounter atomic.Uint64

var channelMonitorProbeQuestionRegex = regexp.MustCompile(`(?m)Q:\s*(\d+)\s*([+-])\s*(\d+)\s*=\s*\?\s*\nA:\s*$`)

// GatewayHandler handles API gateway requests
type GatewayHandler struct {
	gatewayService            *service.GatewayService
	geminiCompatService       *service.GeminiMessagesCompatService
	antigravityGatewayService *service.AntigravityGatewayService
	userService               *service.UserService
	billingCacheService       *service.BillingCacheService
	usageService              *service.UsageService
	apiKeyService             *service.APIKeyService
	usageRecordWorkerPool     *service.UsageRecordWorkerPool
	errorPassthroughService   *service.ErrorPassthroughService
	contentModerationService  *service.ContentModerationService
	opsService                *service.OpsService
	concurrencyHelper         *ConcurrencyHelper
	userMsgQueueHelper        *UserMsgQueueHelper
	maxAccountSwitches        int
	maxAccountSwitchesGemini  int
	cfg                       *config.Config
	settingService            *service.SettingService
}

func (h *GatewayHandler) handleGroupModelUnsupportedError(c *gin.Context, err error, streamStarted bool) bool {
	var modelErr *service.GroupModelUnsupportedError
	if !errors.As(err, &modelErr) {
		return false
	}
	h.handleStreamingAwareError(c, http.StatusForbidden, "permission_error", modelErr.Error(), streamStarted)
	return true
}

// NewGatewayHandler creates a new GatewayHandler
func NewGatewayHandler(
	gatewayService *service.GatewayService,
	geminiCompatService *service.GeminiMessagesCompatService,
	antigravityGatewayService *service.AntigravityGatewayService,
	userService *service.UserService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	contentModerationService *service.ContentModerationService,
	opsService *service.OpsService,
	userMsgQueueService *service.UserMessageQueueService,
	cfg *config.Config,
	settingService *service.SettingService,
) *GatewayHandler {
	pingInterval := time.Duration(0)
	maxAccountSwitches := 10
	maxAccountSwitchesGemini := 3
	if cfg != nil {
		pingInterval = time.Duration(cfg.Concurrency.PingInterval) * time.Second
		if cfg.Gateway.MaxAccountSwitches > 0 {
			maxAccountSwitches = cfg.Gateway.MaxAccountSwitches
		}
		if cfg.Gateway.MaxAccountSwitchesGemini > 0 {
			maxAccountSwitchesGemini = cfg.Gateway.MaxAccountSwitchesGemini
		}
	}

	// 初始化用户消息串行队列 helper
	var umqHelper *UserMsgQueueHelper
	if userMsgQueueService != nil && cfg != nil {
		umqHelper = NewUserMsgQueueHelper(userMsgQueueService, SSEPingFormatClaude, pingInterval)
	}

	return &GatewayHandler{
		gatewayService:            gatewayService,
		geminiCompatService:       geminiCompatService,
		antigravityGatewayService: antigravityGatewayService,
		userService:               userService,
		billingCacheService:       billingCacheService,
		usageService:              usageService,
		apiKeyService:             apiKeyService,
		usageRecordWorkerPool:     usageRecordWorkerPool,
		errorPassthroughService:   errorPassthroughService,
		contentModerationService:  contentModerationService,
		opsService:                opsService,
		concurrencyHelper:         NewConcurrencyHelper(concurrencyService, SSEPingFormatClaude, pingInterval),
		userMsgQueueHelper:        umqHelper,
		maxAccountSwitches:        maxAccountSwitches,
		maxAccountSwitchesGemini:  maxAccountSwitchesGemini,
		cfg:                       cfg,
		settingService:            settingService,
	}
}

func cloneGatewayRequestBody(body []byte) []byte {
	if len(body) == 0 {
		return nil
	}
	cloned := make([]byte, len(body))
	copy(cloned, body)
	return cloned
}

// Messages handles Claude API compatible messages endpoint
// POST /v1/messages
func (h *GatewayHandler) Messages(c *gin.Context) {
	requestStart := time.Now()
	// 从context获取apiKey和user（ApiKeyAuth中间件已设置）
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.gateway.messages",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	defer h.maybeLogCompatibilityFallbackMetrics(reqLog)

	// 读取请求体
	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false, body)

	bodyRef := service.NewRequestBodyRef(body)
	parsedReq, err := service.ParseGatewayRequest(bodyRef, domain.PlatformAnthropic)
	if err != nil {
		logRequestBodyParseFailure(reqLog, body, err)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}
	parsedReq.UserID = subject.UserID
	parsedReq.APIKeyID = apiKey.ID
	reqModel := parsedReq.Model
	reqStream := parsedReq.Stream
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))

	// 解析渠道级模型映射
	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)

	// 设置 max_tokens=1 + haiku 探测请求标识到 context 中
	// 必须在 SetClaudeCodeClientContext 之前设置，因为 ClaudeCodeValidator 需要读取此标识进行绕过判断
	if isMaxTokensOneHaikuRequest(reqModel, parsedReq.MaxTokens) {
		ctx := service.WithIsMaxTokensOneHaikuRequest(c.Request.Context(), true, h.metadataBridgeEnabled())
		c.Request = c.Request.WithContext(ctx)
	}

	// 检查是否为 Claude Code 客户端，设置到 context 中（复用已解析请求，避免二次反序列化）。
	SetClaudeCodeClientContext(c, body, parsedReq)
	isClaudeCodeClient := service.IsClaudeCodeClient(c.Request.Context())

	// 版本检查：仅对 Claude Code 客户端，拒绝低于最低版本的请求
	if !h.checkClaudeCodeVersion(c) {
		return
	}

	// 在请求上下文中记录 thinking 状态，供 Antigravity 最终模型 key 推导/模型维度限流使用
	c.Request = c.Request.WithContext(service.WithThinkingEnabled(c.Request.Context(), parsedReq.ThinkingEnabled, h.metadataBridgeEnabled()))
	c.Request = c.Request.WithContext(service.WithOutputEffort(c.Request.Context(), parsedReq.OutputEffort))

	setOpsRequestContext(c, reqModel, reqStream, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))
	timelinePlatform := ""
	if forcePlatform, ok := middleware2.GetForcePlatformFromContext(c); ok {
		timelinePlatform = forcePlatform
	} else if apiKey.Group != nil {
		timelinePlatform = apiKey.Group.Platform
	}
	h.emitGatewayDebugTimelineRequestReceived(c, timelinePlatform, "messages", requestStart, apiKey, subject.UserID, reqModel, reqStream, body)

	// 验证 model 必填
	if reqModel == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	if h.rejectIfCyberSessionBlocked(c, apiKey, body, reqModel, cyberBlockFormatAnthropic) {
		return
	}

	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolAnthropicMessages, reqModel, body); decision != nil && decision.Blocked {
		h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return
	}

	// Track if we've started streaming (for error handling)
	streamStarted := false

	// 绑定错误透传服务，允许 service 层在非 failover 错误场景复用规则。
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	// 获取订阅信息（可能为nil）- 提前获取用于后续检查
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	// 1. 首先获取用户并发槽位
	userSlotWaitStart := time.Now()
	userReleaseFunc, err := h.concurrencyHelper.AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted)
	userSlotWaitMs := time.Since(userSlotWaitStart).Milliseconds()
	if err != nil {
		reqLog.Warn("gateway.user_slot_acquire_failed", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", streamStarted)
		return
	}
	// 在请求结束或 Context 取消时确保释放槽位，避免客户端断开造成泄漏
	userReleaseFunc = wrapReleaseOnDone(c.Request.Context(), userReleaseFunc)
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	// 2. 【新增】Wait后二次检查余额/订阅
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("gateway.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.handleStreamingAwareError(c, status, code, message, streamStarted)
		return
	}

	// 获取平台：优先使用强制平台（/antigravity 路由，中间件已设置 request.Context），否则使用分组平台
	platform := ""
	if forcePlatform, ok := middleware2.GetForcePlatformFromContext(c); ok {
		platform = forcePlatform
	} else if apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}

	// 设置请求所属分组 ID（用于渠道级功能判断，如 WebSearch 模拟）
	parsedReq.GroupID = apiKey.GroupID

	if platform == service.PlatformKiro {
		hadMetadataUserID := strings.TrimSpace(parsedReq.MetadataUserID) != ""
		sessionSeed := service.KiroExplicitSessionSeed(
			body,
			c.GetHeader("X-Claude-Code-Session-Id"),
			c.GetHeader("session_id"),
			c.GetHeader("conversation_id"),
		)
		if updatedBody, metadataUserID, changed := service.EnsureKiroMetadataUserIDForSession(parsedReq.Body.Bytes(), parsedReq.MetadataUserID, sessionSeed); changed {
			body = updatedBody
			parsedReq.Body.Replace(updatedBody)
			parsedReq.MetadataUserID = metadataUserID
		}
		reqLog.Info("gateway.kiro_request_entry",
			zap.Bool("session_seed_present", strings.TrimSpace(sessionSeed) != ""),
			zap.Bool("metadata_user_id_present_before", hadMetadataUserID),
			zap.Bool("metadata_user_id_present", strings.TrimSpace(parsedReq.MetadataUserID) != ""),
			zap.Bool("thinking_enabled", parsedReq.ThinkingEnabled),
		)
	}

	// 计算粘性会话hash
	parsedReq.SessionContext = &service.SessionContext{
		ClientIP:  ip.GetClientIP(c),
		UserAgent: c.GetHeader("User-Agent"),
		APIKeyID:  apiKey.ID,
	}
	sessionHash := h.gatewayService.GenerateSessionHash(parsedReq)
	sessionKey := sessionHash
	if platform == service.PlatformGemini && sessionHash != "" {
		sessionKey = "gemini:" + sessionHash
	}

	// 查询粘性会话绑定的账号 ID
	var sessionBoundAccountID int64
	if sessionKey != "" {
		sessionBoundAccountID, _ = h.gatewayService.GetCachedSessionAccountID(c.Request.Context(), apiKey.GroupID, sessionKey)
		if sessionBoundAccountID > 0 {
			prefetchedGroupID := int64(0)
			if apiKey.GroupID != nil {
				prefetchedGroupID = *apiKey.GroupID
			}
			ctx := service.WithPrefetchedStickySession(c.Request.Context(), sessionBoundAccountID, prefetchedGroupID, h.metadataBridgeEnabled())
			c.Request = c.Request.WithContext(ctx)
		}
	}
	// 判断是否真的绑定了粘性会话：有 sessionKey 且已经绑定到某个账号
	hasBoundSession := sessionKey != "" && sessionBoundAccountID > 0

	if platform == service.PlatformGemini {
		fs := NewFailoverState(h.maxAccountSwitchesGemini, hasBoundSession)

		// 单账号分组提前设置 SingleAccountRetry 标记，让 Service 层首次 503 就不设模型限流标记。
		// 避免单账号分组收到 503 (MODEL_CAPACITY_EXHAUSTED) 时设 29s 限流，导致后续请求连续快速失败。
		if h.gatewayService.IsSingleAntigravityAccountGroup(c.Request.Context(), apiKey.GroupID) {
			ctx := service.WithSingleAccountRetry(c.Request.Context(), true, h.metadataBridgeEnabled())
			c.Request = c.Request.WithContext(ctx)
		}

		for {
			routingAttemptStart := time.Now()

			selection, err := h.gatewayService.SelectAccountWithLoadAwareness(c.Request.Context(), apiKey.GroupID, sessionKey, reqModel, fs.FailedAccountIDs, "", int64(0)) // Gemini 不使用会话限制
			if err != nil {
				if len(fs.FailedAccountIDs) == 0 {
					cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, reqModel, reqModel, service.PlatformGemini)
					if !cls.ModelNotFound {
						markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
					}
					reqLog.Warn("gateway.select_account_no_available",
						zap.String("model", reqModel),
						zap.Int64p("group_id", apiKey.GroupID),
						zap.String("platform", platform),
						zap.Bool("model_not_found", cls.ModelNotFound),
						zap.Error(err),
					)
					if h.handleGroupModelUnsupportedError(c, err, streamStarted) {
						return
					}
					message := cls.Message
					if !cls.ModelNotFound {
						message = "No available accounts: " + err.Error()
					}
					h.handleStreamingAwareError(c, cls.Status, cls.ErrType, message, streamStarted)
					return
				}
				action := fs.HandleSelectionExhausted(c.Request.Context())
				switch action {
				case FailoverContinue:
					ctx := service.WithSingleAccountRetry(c.Request.Context(), true, h.metadataBridgeEnabled())
					c.Request = c.Request.WithContext(ctx)
					continue
				case FailoverCanceled:
					return
				default: // FailoverExhausted
					if fs.LastFailoverErr != nil {
						h.handleFailoverExhausted(c, fs.LastFailoverErr, service.PlatformGemini, streamStarted)
					} else {
						h.handleFailoverExhaustedSimple(c, 502, streamStarted)
					}
					return
				}
			}
			account := selection.Account
			setOpsSelectedAccount(c, account.ID, account.Platform)
			h.emitGatewayDebugTimelineAccountSelected(c, platform, "messages", requestStart, apiKey, account, reqModel, reqStream, fs.SwitchCount)

			// 检查请求拦截（预热请求、SUGGESTION MODE等）
			if account.IsInterceptWarmupEnabled() {
				interceptType := detectInterceptType(body, reqModel, parsedReq.MaxTokens, isClaudeCodeClient)
				if interceptType != InterceptTypeNone {
					if selection.Acquired && selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
					}
					if reqStream {
						sendMockInterceptStream(c, reqModel, interceptType)
					} else {
						sendMockInterceptResponse(c, reqModel, interceptType)
					}
					return
				}
			}

			// 3. 获取账号并发槽位
			accountReleaseFunc := selection.ReleaseFunc
			accountSlotWaitMs := int64(0)
			if !selection.Acquired {
				if selection.WaitPlan == nil {
					reqLog.Warn("gateway.select_account_no_slot_no_wait_plan",
						zap.Int64("account_id", account.ID),
						zap.String("model", reqModel),
						zap.String("platform", platform),
					)
					h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStarted)
					return
				}
				accountWaitCounted := false
				canWait, err := h.concurrencyHelper.IncrementAccountWaitCount(c.Request.Context(), account.ID, selection.WaitPlan.MaxWaiting)
				if err != nil {
					reqLog.Warn("gateway.account_wait_counter_increment_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				} else if !canWait {
					reqLog.Info("gateway.account_wait_queue_full",
						zap.Int64("account_id", account.ID),
						zap.Int("max_waiting", selection.WaitPlan.MaxWaiting),
					)
					h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later", streamStarted)
					return
				}
				if err == nil && canWait {
					accountWaitCounted = true
				}
				releaseWait := func() {
					if accountWaitCounted {
						h.concurrencyHelper.DecrementAccountWaitCount(c.Request.Context(), account.ID)
						accountWaitCounted = false
					}
				}

				accountSlotWaitStart := time.Now()
				accountReleaseFunc, err = h.concurrencyHelper.AcquireAccountSlotWithWaitTimeoutForGroup(
					c,
					account.ID,
					apiKey.GroupID,
					selection.WaitPlan.MaxConcurrency,
					selection.WaitPlan.Timeout,
					reqStream,
					&streamStarted,
				)
				accountSlotWaitMs = time.Since(accountSlotWaitStart).Milliseconds()
				if err != nil {
					reqLog.Warn("gateway.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
					releaseWait()
					h.handleConcurrencyError(c, err, "account", streamStarted)
					return
				}
				// Slot acquired: no longer waiting in queue.
				releaseWait()
				if !h.gatewayService.RegisterSessionAfterAcquire(c.Request.Context(), account, sessionKey) {
					if accountReleaseFunc != nil {
						accountReleaseFunc()
					}
					reqLog.Warn("gateway.session_limit_exceeded_after_wait", zap.Int64("account_id", account.ID))
					h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Account session limit exceeded", streamStarted)
					return
				}
				if err := h.gatewayService.BindStickySession(c.Request.Context(), apiKey.GroupID, sessionKey, account.ID); err != nil {
					reqLog.Warn("gateway.bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				}
			}
			// 账号槽位/等待计数需要在超时或断开时安全回收
			accountReleaseFunc = wrapReleaseOnDone(c.Request.Context(), accountReleaseFunc)
			h.emitGatewayDebugTimelineSlotAcquired(c, platform, "messages", requestStart, apiKey, account, reqModel, reqStream, userSlotWaitMs, accountSlotWaitMs)

			if h.handleChannelMonitorProbe(c, body, reqModel, parsedReq.MaxTokens, reqStream) {
				recordOpsRoutingLatency(c, routingAttemptStart)
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
				return
			}

			// 转发请求 - 根据账号平台分流
			var result *service.ForwardResult
			recordOpsRoutingLatency(c, routingAttemptStart)
			forwardStart := time.Now()
			requestCtx := c.Request.Context()
			if fs.SwitchCount > 0 {
				requestCtx = service.WithAccountSwitchCount(requestCtx, fs.SwitchCount, h.metadataBridgeEnabled())
			}
			// 记录 Forward 前已写入字节数，Forward 后若增加则说明 SSE 内容已发，禁止 failover
			writerSizeBeforeForward := c.Writer.Size()
			if account.Platform == service.PlatformAntigravity {
				result, err = h.antigravityGatewayService.ForwardGemini(
					requestCtx,
					c,
					account,
					reqModel,
					"generateContent",
					reqStream,
					body,
					hasBoundSession,
					service.WithForwardGeminiSession(derefGroupID(apiKey.GroupID), sessionKey),
				)
			} else {
				result, err = h.geminiCompatService.Forward(requestCtx, c, account, body)
			}
			forwardDurationMs := time.Since(forwardStart).Milliseconds()
			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
			recordOpsForwardLatencies(c, forwardDurationMs, result, err)
			if err != nil {
				var failoverErr *service.UpstreamFailoverError
				if errors.As(err, &failoverErr) {
					h.emitGatewayDebugTimelineAttemptFinished(c, platform, "messages", "failover", requestStart, apiKey, account, reqModel, reqStream, fs.SwitchCount, forwardDurationMs, result, err)
					// 流式内容已写入客户端，无法撤销，禁止 failover 以防止流拼接腐化
					if gatewayFailoverStreamAlreadyWritten(c, writerSizeBeforeForward) {
						h.handleFailoverExhausted(c, failoverErr, service.PlatformGemini, true)
						return
					}
					action := fs.HandleFailoverError(c.Request.Context(), h.gatewayService, account.ID, account.Platform, account.GetPoolModeRetryCount(), failoverErr)
					switch action {
					case FailoverContinue:
						continue
					case FailoverExhausted:
						h.handleFailoverExhausted(c, fs.LastFailoverErr, service.PlatformGemini, streamStarted)
						return
					case FailoverCanceled:
						return
					}
				}
				h.emitGatewayDebugTimelineAttemptFinished(c, platform, "messages", "error", requestStart, apiKey, account, reqModel, reqStream, fs.SwitchCount, forwardDurationMs, result, err)
				// 终态错误但已产生部分输出：补记 partial，但 cyber 已记账则跳过。
				if result != nil {
					if service.GetOpsCyberPolicy(c) != nil {
						reqLog.Warn("gateway.forward_partial_error_result_cyber_billed",
							zap.Int64("account_id", account.ID),
							zap.Error(err),
						)
					} else {
						userAgent := c.GetHeader("User-Agent")
						clientIP := ip.GetClientIP(c)
						requestPayloadHash := service.HashUsageRequestPayload(body)
						inboundEndpoint := GetInboundEndpoint(c)
						upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
						forceCacheBilling := fs.ForceCacheBilling
						quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
						h.submitUsageRecordTask(c.Request.Context(), wrapUsageRecordTaskWithRequestContext(c, func(ctx context.Context) {
							if recErr := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
								Result:             result,
								QuotaPlatform:      quotaPlatform,
								APIKey:             apiKey,
								User:               apiKey.User,
								Account:            account,
								Subscription:       subscription,
								InboundEndpoint:    inboundEndpoint,
								UpstreamEndpoint:   upstreamEndpoint,
								UserAgent:          userAgent,
								IPAddress:          clientIP,
								RequestPayloadHash: requestPayloadHash,
								ForceCacheBilling:  forceCacheBilling,
								APIKeyService:      h.apiKeyService,
								ChannelUsageFields: channelMapping.ToUsageFields(reqModel, result.UpstreamModel),
							}); recErr != nil {
								logger.L().With(
									zap.String("component", "handler.gateway.messages"),
									zap.Int64("user_id", subject.UserID),
									zap.Int64("api_key_id", apiKey.ID),
									zap.Any("group_id", apiKey.GroupID),
									zap.String("model", reqModel),
									zap.Int64("account_id", account.ID),
								).Error("gateway.record_partial_usage_failed", zap.Error(recErr))
							}
						}))
						reqLog.Warn("gateway.forward_partial_error_result",
							zap.Int64("account_id", account.ID),
							zap.Error(err),
						)
					}
				}
				if shouldSuppressForwardErrorResponse(c, err) {
					return
				}
				upstreamErrorAlreadyCommunicated := c.Writer.Written()
				wroteFallback := h.ensureForwardErrorResponse(c, streamStarted, err)
				forwardFailedFields := []zap.Field{
					zap.Int64("account_id", account.ID),
					zap.String("account_name", account.Name),
					zap.String("account_platform", account.Platform),
					zap.Bool("fallback_error_response_written", wroteFallback),
					zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
					zap.Error(err),
				}
				if account.Proxy != nil {
					forwardFailedFields = append(forwardFailedFields,
						zap.Int64("proxy_id", account.Proxy.ID),
						zap.String("proxy_name", account.Proxy.Name),
						zap.String("proxy_host", account.Proxy.Host),
						zap.Int("proxy_port", account.Proxy.Port),
					)
				} else if account.ProxyID != nil {
					forwardFailedFields = append(forwardFailedFields, zap.Int64p("proxy_id", account.ProxyID))
				}
				reqLog.Error("gateway.forward_failed", forwardFailedFields...)
				return
			}
			h.emitGatewayDebugTimelineAttemptFinished(c, platform, "messages", "success", requestStart, apiKey, account, reqModel, reqStream, fs.SwitchCount, forwardDurationMs, result, nil)

			// RPM 计数递增（Forward 成功后）
			// 注意：TOCTOU 竞态是已知且可接受的设计权衡，与 WindowCost 一致的 soft-limit 模式。
			// 在高并发下可能短暂超出 RPM 限制，但不会导致请求失败。
			if account.IsAnthropicOAuthOrSetupToken() && account.GetBaseRPM() > 0 {
				if err := h.gatewayService.IncrementAccountRPM(c.Request.Context(), account.ID); err != nil {
					reqLog.Warn("gateway.rpm_increment_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				}
			}

			// 捕获请求信息（用于异步记录，避免在 goroutine 中访问 gin.Context）
			userAgent := c.GetHeader("User-Agent")
			clientIP := ip.GetClientIP(c)
			requestPayloadHash := service.HashUsageRequestPayload(body)
			inboundEndpoint := GetInboundEndpoint(c)
			upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)

			if result.ReasoningEffort == nil {
				result.ReasoningEffort = service.NormalizeClaudeOutputEffort(parsedReq.OutputEffort)
			}
			// 国产模型 thinking-enabled 默认 effort 填充：Kimi/GLM/MiniMax 这些不支持 effort 档位的
			// passback-required 上游，仅要 thinking 启用且 OutputEffort 未明确传递时，在 usage_log 写 "high"
			// 避免该字段长期为 NULL（详见 DefaultEffortForThinkingEnabled 文档）。
			if result.ReasoningEffort == nil && parsedReq.ThinkingEnabled {
				protocolModel := result.UpstreamModel
				if protocolModel == "" {
					protocolModel = result.Model
				}
				result.ReasoningEffort = service.DefaultEffortForThinkingEnabled(protocolModel)
			}

			// 使用量记录通过有界 worker 池提交，避免请求热路径创建无界 goroutine。
			// ForceCacheBilling 提前拍成标量，避免 worker 闭包保活 failover 状态里的响应体。
			forceCacheBilling := fs.ForceCacheBilling
			quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
			h.submitUsageRecordTask(c.Request.Context(), wrapUsageRecordTaskWithRequestContext(c, func(ctx context.Context) {
				if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
					Result:             result,
					QuotaPlatform:      quotaPlatform,
					APIKey:             apiKey,
					User:               apiKey.User,
					Account:            account,
					Subscription:       subscription,
					InboundEndpoint:    inboundEndpoint,
					UpstreamEndpoint:   upstreamEndpoint,
					UserAgent:          userAgent,
					IPAddress:          clientIP,
					RequestPayloadHash: requestPayloadHash,
					ForceCacheBilling:  forceCacheBilling,
					APIKeyService:      h.apiKeyService,
					ChannelUsageFields: channelMapping.ToUsageFields(reqModel, result.UpstreamModel),
				}); err != nil {
					logger.L().With(
						zap.String("component", "handler.gateway.messages"),
						zap.Int64("user_id", subject.UserID),
						zap.Int64("api_key_id", apiKey.ID),
						zap.Any("group_id", apiKey.GroupID),
						zap.String("model", reqModel),
						zap.Int64("account_id", account.ID),
					).Error("gateway.record_usage_failed", zap.Error(err))
				}
			}))
			return
		}
	}

	currentAPIKey := apiKey
	currentSubscription := subscription
	var fallbackGroupID *int64
	if apiKey.Group != nil {
		fallbackGroupID = apiKey.Group.FallbackGroupIDOnInvalidRequest
	}
	fallbackUsed := false

	// 单账号分组提前设置 SingleAccountRetry 标记，让 Service 层首次 503 就不设模型限流标记。
	// 避免单账号分组收到 503 (MODEL_CAPACITY_EXHAUSTED) 时设 29s 限流，导致后续请求连续快速失败。
	if h.gatewayService.IsSingleAntigravityAccountGroup(c.Request.Context(), currentAPIKey.GroupID) {
		ctx := service.WithSingleAccountRetry(c.Request.Context(), true, h.metadataBridgeEnabled())
		c.Request = c.Request.WithContext(ctx)
	}

	for {
		fs := NewFailoverState(h.maxAccountSwitches, hasBoundSession)
		retryWithFallback := false

		for {
			routingAttemptStart := time.Now()
			attemptParsedReq := *parsedReq
			attemptParsedReq.Body = service.NewRequestBodyRef(cloneGatewayRequestBody(parsedReq.Body.Bytes()))

			// 选择支持该模型的账号
			selection, err := h.gatewayService.SelectAccountWithLoadAwareness(c.Request.Context(), currentAPIKey.GroupID, sessionKey, reqModel, fs.FailedAccountIDs, parsedReq.MetadataUserID, subject.UserID)
			if err != nil {
				if len(fs.FailedAccountIDs) == 0 {
					cls := classifyNoAccountErrorFromGin(c, h.gatewayService, currentAPIKey, reqModel, reqModel, platform)
					if !cls.ModelNotFound {
						markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
					}
					reqLog.Warn("gateway.select_account_no_available",
						zap.String("model", reqModel),
						zap.Int64p("group_id", currentAPIKey.GroupID),
						zap.String("platform", platform),
						zap.Bool("fallback_used", fallbackUsed),
						zap.Bool("model_not_found", cls.ModelNotFound),
						zap.Error(err),
					)
					if h.handleGroupModelUnsupportedError(c, err, streamStarted) {
						return
					}
					message := cls.Message
					if !cls.ModelNotFound {
						message = "No available accounts: " + err.Error()
					}
					h.handleStreamingAwareError(c, cls.Status, cls.ErrType, message, streamStarted)
					return
				}
				action := fs.HandleSelectionExhausted(c.Request.Context())
				switch action {
				case FailoverContinue:
					ctx := service.WithSingleAccountRetry(c.Request.Context(), true, h.metadataBridgeEnabled())
					c.Request = c.Request.WithContext(ctx)
					continue
				case FailoverCanceled:
					return
				default: // FailoverExhausted
					if fs.LastFailoverErr != nil {
						h.handleFailoverExhausted(c, fs.LastFailoverErr, platform, streamStarted)
					} else {
						h.handleFailoverExhaustedSimple(c, 502, streamStarted)
					}
					return
				}
			}
			account := selection.Account
			setOpsSelectedAccount(c, account.ID, account.Platform)
			h.emitGatewayDebugTimelineAccountSelected(c, platform, "messages", requestStart, currentAPIKey, account, reqModel, reqStream, fs.SwitchCount)

			// 检查请求拦截（预热请求、SUGGESTION MODE等）
			if account.IsInterceptWarmupEnabled() {
				interceptType := detectInterceptType(body, reqModel, parsedReq.MaxTokens, isClaudeCodeClient)
				if interceptType != InterceptTypeNone {
					if selection.Acquired && selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
					}
					if reqStream {
						sendMockInterceptStream(c, reqModel, interceptType)
					} else {
						sendMockInterceptResponse(c, reqModel, interceptType)
					}
					return
				}
			}

			// 3. 获取账号并发槽位
			accountReleaseFunc := selection.ReleaseFunc
			accountSlotWaitMs := int64(0)
			if !selection.Acquired {
				if selection.WaitPlan == nil {
					reqLog.Warn("gateway.select_account_no_slot_no_wait_plan",
						zap.Int64("account_id", account.ID),
						zap.String("model", reqModel),
						zap.String("platform", platform),
					)
					h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStarted)
					return
				}
				accountWaitCounted := false
				canWait, err := h.concurrencyHelper.IncrementAccountWaitCount(c.Request.Context(), account.ID, selection.WaitPlan.MaxWaiting)
				if err != nil {
					reqLog.Warn("gateway.account_wait_counter_increment_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				} else if !canWait {
					reqLog.Info("gateway.account_wait_queue_full",
						zap.Int64("account_id", account.ID),
						zap.Int("max_waiting", selection.WaitPlan.MaxWaiting),
					)
					h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later", streamStarted)
					return
				}
				if err == nil && canWait {
					accountWaitCounted = true
				}
				releaseWait := func() {
					if accountWaitCounted {
						h.concurrencyHelper.DecrementAccountWaitCount(c.Request.Context(), account.ID)
						accountWaitCounted = false
					}
				}

				accountSlotWaitStart := time.Now()
				accountReleaseFunc, err = h.concurrencyHelper.AcquireAccountSlotWithWaitTimeoutForGroup(
					c,
					account.ID,
					currentAPIKey.GroupID,
					selection.WaitPlan.MaxConcurrency,
					selection.WaitPlan.Timeout,
					reqStream,
					&streamStarted,
				)
				accountSlotWaitMs = time.Since(accountSlotWaitStart).Milliseconds()
				if err != nil {
					reqLog.Warn("gateway.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
					releaseWait()
					h.handleConcurrencyError(c, err, "account", streamStarted)
					return
				}
				// Slot acquired: no longer waiting in queue.
				releaseWait()
				// Register session only after a real slot is held (WaitPlan no longer pre-registers).
				if !h.gatewayService.RegisterSessionAfterAcquire(c.Request.Context(), account, sessionKey) {
					if accountReleaseFunc != nil {
						accountReleaseFunc()
					}
					reqLog.Warn("gateway.session_limit_exceeded_after_wait", zap.Int64("account_id", account.ID))
					h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Account session limit exceeded", streamStarted)
					return
				}
				if err := h.gatewayService.BindStickySession(c.Request.Context(), currentAPIKey.GroupID, sessionKey, account.ID); err != nil {
					reqLog.Warn("gateway.bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				}
			}
			// 账号槽位/等待计数需要在超时或断开时安全回收
			accountReleaseFunc = wrapReleaseOnDone(c.Request.Context(), accountReleaseFunc)
			h.emitGatewayDebugTimelineSlotAcquired(c, platform, "messages", requestStart, currentAPIKey, account, reqModel, reqStream, userSlotWaitMs, accountSlotWaitMs)

			if h.handleChannelMonitorProbe(c, body, reqModel, parsedReq.MaxTokens, reqStream) {
				recordOpsRoutingLatency(c, routingAttemptStart)
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
				return
			}

			// ===== 用户消息串行队列 START =====
			var queueRelease func()
			umqMode := h.getUserMsgQueueMode(account, &attemptParsedReq)

			switch umqMode {
			case config.UMQModeSerialize:
				// 串行模式：获取锁 + RPM 延迟 + 释放（当前行为不变）
				baseRPM := account.GetBaseRPM()
				release, qErr := h.userMsgQueueHelper.AcquireWithWait(
					c, account.ID, baseRPM, reqStream, &streamStarted,
					h.cfg.Gateway.UserMessageQueue.WaitTimeout(),
					reqLog,
				)
				if qErr != nil {
					// fail-open: 记录 warn，不阻止请求
					reqLog.Warn("gateway.umq_acquire_failed",
						zap.Int64("account_id", account.ID),
						zap.Error(qErr),
					)
				} else {
					queueRelease = release
				}

			case config.UMQModeThrottle:
				// 软性限速：仅施加 RPM 自适应延迟，不阻塞并发
				baseRPM := account.GetBaseRPM()
				if tErr := h.userMsgQueueHelper.ThrottleWithPing(
					c, account.ID, baseRPM, reqStream, &streamStarted,
					h.cfg.Gateway.UserMessageQueue.WaitTimeout(),
					reqLog,
				); tErr != nil {
					reqLog.Warn("gateway.umq_throttle_failed",
						zap.Int64("account_id", account.ID),
						zap.Error(tErr),
					)
				}

			default:
				if umqMode != "" {
					reqLog.Warn("gateway.umq_unknown_mode",
						zap.String("mode", umqMode),
						zap.Int64("account_id", account.ID),
					)
				}
			}

			// 用 wrapReleaseOnDone 确保 context 取消时自动释放（仅 serialize 模式有 queueRelease）
			queueRelease = wrapReleaseOnDone(c.Request.Context(), queueRelease)
			// 注入回调到 ParsedRequest：使用外层 wrapper 以便提前清理 AfterFunc
			attemptParsedReq.OnUpstreamAccepted = queueRelease
			// ===== 用户消息串行队列 END =====

			// 应用渠道模型映射到请求
			forwardBody := cloneGatewayRequestBody(attemptParsedReq.Body.Bytes())
			forwardModel := attemptParsedReq.Model
			if channelMapping.Mapped {
				forwardModel = channelMapping.MappedModel
				forwardBody = h.gatewayService.ReplaceModelInBody(forwardBody, channelMapping.MappedModel)
			}
			// Bedrock CC 兼容：渠道模型映射后，清理 Anthropic API 专有字段、注入 Bedrock 必需字段。
			// 这里必须只改写本次 attempt 的 body，不能回写 parsedReq.Body，
			// 否则 Bedrock attempt 的兼容字段会污染后续非 Bedrock 重试。
			forwardBody = h.gatewayService.ApplyBedrockCCCompat(c, forwardBody, forwardModel, account, currentAPIKey.GroupID)
			forwardParsedReq := attemptParsedReq
			forwardParsedReq.Model = forwardModel
			forwardParsedReq.Body = service.NewRequestBodyRef(forwardBody)

			// 转发请求 - 根据账号平台分流
			c.Set("parsed_request", &attemptParsedReq)
			var result *service.ForwardResult
			recordOpsRoutingLatency(c, routingAttemptStart)
			forwardStart := time.Now()
			requestCtx := c.Request.Context()
			if fs.SwitchCount > 0 {
				requestCtx = service.WithAccountSwitchCount(requestCtx, fs.SwitchCount, h.metadataBridgeEnabled())
			}
			// 记录 Forward 前已写入字节数，Forward 后若增加则说明 SSE 内容已发，禁止 failover
			writerSizeBeforeForward := c.Writer.Size()
			if account.Platform == service.PlatformAntigravity && account.Type != service.AccountTypeAPIKey {
				result, err = h.antigravityGatewayService.Forward(requestCtx, c, account, forwardBody, hasBoundSession)
			} else {
				result, err = h.gatewayService.Forward(requestCtx, c, account, &forwardParsedReq)
			}
			forwardDurationMs := time.Since(forwardStart).Milliseconds()

			// 兜底释放串行锁（正常情况已通过回调提前释放）
			if queueRelease != nil {
				queueRelease()
			}
			// 清理回调引用，防止 failover 重试时旧回调被错误调用
			attemptParsedReq.OnUpstreamAccepted = nil

			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
			recordOpsForwardLatencies(c, forwardDurationMs, result, err)
			h.recordGatewayCyberPolicyIfMarked(c, currentAPIKey, account, currentSubscription, reqModel, err != nil, service.CyberSessionBlockKey(currentAPIKey.ID, c, body), channelMapping.ToUsageFields(reqModel, ""), service.ContentModerationProtocolAnthropicMessages, body)
			if err != nil {
				// Beta policy block: return 400 immediately, no failover
				var betaBlockedErr *service.BetaBlockedError
				if errors.As(err, &betaBlockedErr) {
					service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
					h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", betaBlockedErr.Message)
					return
				}

				var promptTooLongErr *service.PromptTooLongError
				if errors.As(err, &promptTooLongErr) {
					reqLog.Warn("gateway.prompt_too_long_from_antigravity",
						zap.Any("current_group_id", currentAPIKey.GroupID),
						zap.Any("fallback_group_id", fallbackGroupID),
						zap.Bool("fallback_used", fallbackUsed),
					)
					if !fallbackUsed && fallbackGroupID != nil && *fallbackGroupID > 0 {
						fallbackGroup, err := h.gatewayService.ResolveGroupByID(c.Request.Context(), *fallbackGroupID)
						if err != nil {
							reqLog.Warn("gateway.resolve_fallback_group_failed", zap.Int64("fallback_group_id", *fallbackGroupID), zap.Error(err))
							_ = h.antigravityGatewayService.WriteMappedClaudeError(c, account, promptTooLongErr.StatusCode, promptTooLongErr.RequestID, promptTooLongErr.Body)
							return
						}
						if fallbackGroup.Platform != service.PlatformAnthropic ||
							fallbackGroup.SubscriptionType == service.SubscriptionTypeSubscription ||
							fallbackGroup.FallbackGroupIDOnInvalidRequest != nil {
							reqLog.Warn("gateway.fallback_group_invalid",
								zap.Int64("fallback_group_id", fallbackGroup.ID),
								zap.String("fallback_platform", fallbackGroup.Platform),
								zap.String("fallback_subscription_type", fallbackGroup.SubscriptionType),
							)
							_ = h.antigravityGatewayService.WriteMappedClaudeError(c, account, promptTooLongErr.StatusCode, promptTooLongErr.RequestID, promptTooLongErr.Body)
							return
						}
						fallbackAPIKey := cloneAPIKeyWithGroup(apiKey, fallbackGroup)
						if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), fallbackAPIKey.User, fallbackAPIKey, fallbackGroup, nil, service.PlatformFromAPIKey(fallbackAPIKey)); err != nil {
							status, code, message, retryAfter := billingErrorDetails(err)
							if retryAfter > 0 {
								c.Header("Retry-After", strconv.Itoa(retryAfter))
							}
							h.handleStreamingAwareError(c, status, code, message, streamStarted)
							return
						}
						// 兜底重试按"直接请求兜底分组"处理：清除强制平台，允许按分组平台调度
						ctx := context.WithValue(c.Request.Context(), ctxkey.ForcePlatform, "")
						c.Request = c.Request.WithContext(ctx)
						currentAPIKey = fallbackAPIKey
						currentSubscription = nil
						fallbackUsed = true
						retryWithFallback = true
						break
					}
					_ = h.antigravityGatewayService.WriteMappedClaudeError(c, account, promptTooLongErr.StatusCode, promptTooLongErr.RequestID, promptTooLongErr.Body)
					return
				}
				var failoverErr *service.UpstreamFailoverError
				if errors.As(err, &failoverErr) {
					h.emitGatewayDebugTimelineAttemptFinished(c, platform, "messages", "failover", requestStart, currentAPIKey, account, reqModel, reqStream, fs.SwitchCount, forwardDurationMs, result, err)
					// 流式内容已写入客户端，无法撤销，禁止 failover 以防止流拼接腐化
					if gatewayFailoverStreamAlreadyWritten(c, writerSizeBeforeForward) {
						h.handleFailoverExhausted(c, failoverErr, account.Platform, true)
						return
					}
					prevRetryCount := fs.SameAccountRetryCount[account.ID]
					prevSwitchCount := fs.SwitchCount
					action := fs.HandleFailoverError(c.Request.Context(), h.gatewayService, account.ID, account.Platform, account.GetPoolModeRetryCount(), failoverErr)
					switch action {
					case FailoverContinue:
						ctx := pinSameAccountRetryContext(c.Request.Context(), fs, account.ID, currentAPIKey.GroupID, failoverErr, prevRetryCount, prevSwitchCount, h.metadataBridgeEnabled())
						c.Request = c.Request.WithContext(ctx)
						continue
					case FailoverExhausted:
						h.handleFailoverExhausted(c, fs.LastFailoverErr, account.Platform, streamStarted)
						return
					case FailoverCanceled:
						return
					}
				}
				h.emitGatewayDebugTimelineAttemptFinished(c, platform, "messages", "error", requestStart, currentAPIKey, account, reqModel, reqStream, fs.SwitchCount, forwardDurationMs, result, err)
				// 终态错误但已产生部分输出（流式中途断开 / 读错误 / 生成失败帧等）：
				// 上游已消耗账号配额且不会 failover（failover 错误在上面的分支已 return），
				// 补记这部分用量，避免客户端主动中断流来逃避计费。这是 Kiro/Claude 等走
				// gatewayService.Forward 的主路径，Kiro forwardStream 会在终态错误时返回
				// buildKiroPartialStreamResult() 作为非 nil result。
				// Cyber hits already billed via recordGatewayCyberPolicyIfMarked(forwardErrored=true);
				// do not also RecordUsage or the same request is double-charged (align OpenAI path).
				if result != nil {
					if service.GetOpsCyberPolicy(c) != nil {
						reqLog.Warn("gateway.forward_partial_error_result_cyber_billed",
							zap.Int64("account_id", account.ID),
							zap.Error(err),
						)
					} else {
						userAgent := c.GetHeader("User-Agent")
						clientIP := ip.GetClientIP(c)
						requestPayloadHash := service.HashUsageRequestPayload(attemptParsedReq.Body.Bytes())
						inboundEndpoint := GetInboundEndpoint(c)
						upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
						forceCacheBilling := fs.ForceCacheBilling
						quotaPlatform := service.QuotaPlatform(c.Request.Context(), currentAPIKey)
						h.submitUsageRecordTask(c.Request.Context(), wrapUsageRecordTaskWithRequestContext(c, func(ctx context.Context) {
							if recErr := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
								Result:             result,
								QuotaPlatform:      quotaPlatform,
								APIKey:             currentAPIKey,
								User:               currentAPIKey.User,
								Account:            account,
								Subscription:       currentSubscription,
								InboundEndpoint:    inboundEndpoint,
								UpstreamEndpoint:   upstreamEndpoint,
								UserAgent:          userAgent,
								IPAddress:          clientIP,
								RequestPayloadHash: requestPayloadHash,
								ForceCacheBilling:  forceCacheBilling,
								APIKeyService:      h.apiKeyService,
								ChannelUsageFields: channelMapping.ToUsageFields(reqModel, result.UpstreamModel),
							}); recErr != nil {
								logger.L().With(
									zap.String("component", "handler.gateway.messages"),
									zap.Int64("user_id", subject.UserID),
									zap.Int64("api_key_id", currentAPIKey.ID),
									zap.Any("group_id", currentAPIKey.GroupID),
									zap.String("model", reqModel),
									zap.Int64("account_id", account.ID),
								).Error("gateway.record_partial_usage_failed", zap.Error(recErr))
							}
						}))
						reqLog.Warn("gateway.forward_partial_error_result",
							zap.Int64("account_id", account.ID),
							zap.Error(err),
						)
					}
				}
				if shouldSuppressForwardErrorResponse(c, err) {
					return
				}
				upstreamErrorAlreadyCommunicated := c.Writer.Written()
				wroteFallback := h.ensureForwardErrorResponse(c, streamStarted, err)
				forwardFailedFields := []zap.Field{
					zap.Int64("account_id", account.ID),
					zap.String("account_name", account.Name),
					zap.String("account_platform", account.Platform),
					zap.Bool("fallback_error_response_written", wroteFallback),
					zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
					zap.Error(err),
				}
				if account.Proxy != nil {
					forwardFailedFields = append(forwardFailedFields,
						zap.Int64("proxy_id", account.Proxy.ID),
						zap.String("proxy_name", account.Proxy.Name),
						zap.String("proxy_host", account.Proxy.Host),
						zap.Int("proxy_port", account.Proxy.Port),
					)
				} else if account.ProxyID != nil {
					forwardFailedFields = append(forwardFailedFields, zap.Int64p("proxy_id", account.ProxyID))
				}
				reqLog.Error("gateway.forward_failed", forwardFailedFields...)
				return
			}
			h.emitGatewayDebugTimelineAttemptFinished(c, platform, "messages", "success", requestStart, currentAPIKey, account, reqModel, reqStream, fs.SwitchCount, forwardDurationMs, result, nil)

			// RPM 计数递增（Forward 成功后）
			// 注意：TOCTOU 竞态是已知且可接受的设计权衡，与 WindowCost 一致的 soft-limit 模式。
			// 在高并发下可能短暂超出 RPM 限制，但不会导致请求失败。
			if account.IsAnthropicOAuthOrSetupToken() && account.GetBaseRPM() > 0 {
				if err := h.gatewayService.IncrementAccountRPM(c.Request.Context(), account.ID); err != nil {
					reqLog.Warn("gateway.rpm_increment_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				}
			}

			// 捕获请求信息（用于异步记录，避免在 goroutine 中访问 gin.Context）
			userAgent := c.GetHeader("User-Agent")
			clientIP := ip.GetClientIP(c)
			// Forward 内部可能继续改写 body，usage 去重指纹必须使用最终上游接受的当前 body。
			requestPayloadHash := service.HashUsageRequestPayload(attemptParsedReq.Body.Bytes())
			inboundEndpoint := GetInboundEndpoint(c)
			upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)

			if result.ReasoningEffort == nil {
				result.ReasoningEffort = service.NormalizeClaudeOutputEffort(attemptParsedReq.OutputEffort)
			}
			// 同上（重试路径中的对称填充）。详见非重试路径同名注释。
			if result.ReasoningEffort == nil && attemptParsedReq.ThinkingEnabled {
				protocolModel := result.UpstreamModel
				if protocolModel == "" {
					protocolModel = result.Model
				}
				result.ReasoningEffort = service.DefaultEffortForThinkingEnabled(protocolModel)
			}

			// 使用量记录通过有界 worker 池提交，避免请求热路径创建无界 goroutine。
			// ForceCacheBilling 提前拍成标量，避免 worker 闭包保活 failover 状态里的响应体。
			forceCacheBilling := fs.ForceCacheBilling
			quotaPlatform := service.QuotaPlatform(c.Request.Context(), currentAPIKey)
			h.submitUsageRecordTask(c.Request.Context(), wrapUsageRecordTaskWithRequestContext(c, func(ctx context.Context) {
				if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
					Result:             result,
					QuotaPlatform:      quotaPlatform,
					APIKey:             currentAPIKey,
					User:               currentAPIKey.User,
					Account:            account,
					Subscription:       currentSubscription,
					InboundEndpoint:    inboundEndpoint,
					UpstreamEndpoint:   upstreamEndpoint,
					UserAgent:          userAgent,
					IPAddress:          clientIP,
					RequestPayloadHash: requestPayloadHash,
					ForceCacheBilling:  forceCacheBilling,
					APIKeyService:      h.apiKeyService,
					ChannelUsageFields: channelMapping.ToUsageFields(reqModel, result.UpstreamModel),
				}); err != nil {
					logger.L().With(
						zap.String("component", "handler.gateway.messages"),
						zap.Int64("user_id", subject.UserID),
						zap.Int64("api_key_id", currentAPIKey.ID),
						zap.Any("group_id", currentAPIKey.GroupID),
						zap.String("model", reqModel),
						zap.Int64("account_id", account.ID),
					).Error("gateway.record_usage_failed", zap.Error(err))
				}
			}))
			return
		}
		if !retryWithFallback {
			return
		}
	}
}

// Models handles listing available models
// GET /v1/models
// Returns models based on account configurations (model_mapping whitelist)
// Falls back to default models if no whitelist is configured
func (h *GatewayHandler) Models(c *gin.Context) {
	apiKey, _ := middleware2.GetAPIKeyFromContext(c)

	var groupID *int64
	var platform string

	if apiKey != nil && apiKey.Group != nil {
		groupID = &apiKey.Group.ID
		platform = apiKey.Group.Platform
	}
	if forcedPlatform, ok := middleware2.GetForcePlatformFromContext(c); ok && strings.TrimSpace(forcedPlatform) != "" {
		platform = forcedPlatform
	}

	// Get available models from account configurations scoped to the current group platform.
	availableModels := h.gatewayService.GetAvailableModels(c.Request.Context(), groupID, platform)
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.CustomModelsListEnabled() {
		fallbackModels := defaultModelIDsForPlatform(platform)
		availableModels = filterModelsByCustomList(customModelsListSource(platform, availableModels, fallbackModels), fallbackModels, apiKey.Group.ModelsListConfig.Models)
		writeCustomModelsList(c, platform, availableModels)
		return
	}

	if len(availableModels) > 0 {
		writeModelsList(c, availableModels)
		return
	}

	// Fallback to default models
	if platform == "openai" {
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   openai.DefaultModels,
		})
		return
	}
	if platform == service.PlatformGemini {
		models := make([]claude.Model, 0, len(gemini.DefaultModels()))
		for _, model := range gemini.DefaultModels() {
			id := strings.TrimPrefix(model.Name, "models/")
			models = append(models, claude.Model{
				ID:          id,
				Type:        "model",
				DisplayName: id,
				CreatedAt:   "2024-01-01T00:00:00Z",
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   models,
		})
		return
	}
	if platform == service.PlatformKiro {
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   kiro.DefaultModels,
		})
		return
	}
	if platform == service.PlatformGrok {
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   xai.DefaultModels(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   claude.DefaultModels,
	})
}

func writeModelsList(c *gin.Context, modelIDs []string) {
	models := make([]claude.Model, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		models = append(models, claude.Model{
			ID:          modelID,
			Type:        "model",
			DisplayName: modelID,
			CreatedAt:   "2024-01-01T00:00:00Z",
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   models,
	})
}

func writeCustomModelsList(c *gin.Context, platform string, modelIDs []string) {
	if platform == service.PlatformOpenAI {
		writeOpenAIModelsList(c, modelIDs)
		return
	}
	writeModelsList(c, modelIDs)
}

func writeOpenAIModelsList(c *gin.Context, modelIDs []string) {
	defaultsByID := make(map[string]openai.Model, len(openai.DefaultModels))
	for _, model := range openai.DefaultModels {
		defaultsByID[model.ID] = model
	}

	models := make([]openai.Model, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		if model, ok := defaultsByID[modelID]; ok {
			models = append(models, model)
			continue
		}
		models = append(models, openai.Model{
			ID:          modelID,
			Object:      "model",
			Created:     1704067200,
			OwnedBy:     "openai",
			Type:        "model",
			DisplayName: modelID,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   models,
	})
}

func customModelsListSource(platform string, availableModels, fallbackModels []string) []string {
	if platform == service.PlatformAnthropic && len(availableModels) > 0 {
		return mergeModelIDs(availableModels, fallbackModels)
	}
	return availableModels
}

func filterModelsByCustomList(availableModels, fallbackModels, selectedModels []string) []string {
	if len(selectedModels) == 0 {
		return availableModels
	}
	source := availableModels
	if len(source) == 0 {
		source = fallbackModels
	}
	if len(source) == 0 {
		return nil
	}

	allowed := make([]string, 0, len(source))
	for _, model := range source {
		model = strings.TrimSpace(model)
		if model != "" {
			allowed = append(allowed, model)
		}
	}

	seen := make(map[string]struct{}, len(selectedModels))
	filtered := make([]string, 0, len(selectedModels))
	for _, model := range selectedModels {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if strings.HasSuffix(model, "*") {
			continue
		}
		if !customModelsListAllowsModel(allowed, model) {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		filtered = append(filtered, model)
	}
	return filtered
}

func customModelsListAllowsModel(availablePatterns []string, model string) bool {
	for _, pattern := range availablePatterns {
		if pattern == model {
			return true
		}
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}

func defaultModelIDsForPlatform(platform string) []string {
	switch platform {
	case service.PlatformOpenAI:
		return openai.DefaultModelIDs()
	case service.PlatformGemini:
		ids := make([]string, 0, len(geminicli.DefaultModels))
		for _, model := range geminicli.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	case service.PlatformAntigravity:
		models := antigravity.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		return ids
	case service.PlatformAnthropic:
		ids := make([]string, 0, len(claude.DefaultModels)+len(antigravity.DefaultModels()))
		for _, model := range claude.DefaultModels {
			ids = append(ids, model.ID)
		}
		for _, model := range antigravity.DefaultModels() {
			ids = append(ids, model.ID)
		}
		return mergeModelIDs(ids, nil)
	case service.PlatformGrok:
		return xai.DefaultModelIDs()
	default:
		ids := make([]string, 0, len(claude.DefaultModels))
		for _, model := range claude.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	}
}

func mergeModelIDs(primary, secondary []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	merged := make([]string, 0, len(primary)+len(secondary))
	for _, models := range [][]string{primary, secondary} {
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			if _, ok := seen[model]; ok {
				continue
			}
			seen[model] = struct{}{}
			merged = append(merged, model)
		}
	}
	return merged
}

// AntigravityModels 返回 Antigravity 支持的全部模型
// GET /antigravity/models
func (h *GatewayHandler) AntigravityModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   antigravity.DefaultModels(),
	})
}

func cloneAPIKeyWithGroup(apiKey *service.APIKey, group *service.Group) *service.APIKey {
	if apiKey == nil || group == nil {
		return apiKey
	}
	cloned := *apiKey
	groupID := group.ID
	cloned.GroupID = &groupID
	cloned.Group = group
	return &cloned
}

// Usage handles getting account balance and usage statistics for CC Switch integration
// GET /v1/usage
//
// Two modes:
//   - quota_limited: API Key has quota or rate limits configured. Returns key-level limits/usage.
//   - unrestricted:  No key-level limits. Returns subscription or wallet balance info.
func (h *GatewayHandler) Usage(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	ctx := c.Request.Context()

	// 解析可选的日期范围参数（用于 model_stats 查询）
	startTime, endTime := h.parseUsageDateRange(c)

	// Best-effort: 获取用量统计（按当前 API Key 过滤），失败不影响基础响应
	usageData := h.buildUsageData(ctx, apiKey.ID)

	// Best-effort: 获取模型统计
	var modelStats any
	if h.usageService != nil {
		if stats, err := h.usageService.GetAPIKeyModelStats(ctx, apiKey.ID, startTime, endTime); err == nil && len(stats) > 0 {
			modelStats = stats
		}
	}

	// 判断模式: key 有总额度或速率限制 → quota_limited，否则 → unrestricted
	isQuotaLimited := apiKey.Quota > 0 || apiKey.HasRateLimits()

	if isQuotaLimited {
		h.usageQuotaLimited(c, ctx, apiKey, usageData, modelStats)
		return
	}

	h.usageUnrestricted(c, ctx, apiKey, subject, usageData, modelStats)
}

// parseUsageDateRange 解析 start_date / end_date query params，默认返回近 30 天范围
func (h *GatewayHandler) parseUsageDateRange(c *gin.Context) (time.Time, time.Time) {
	now := timezone.Now()
	endTime := now
	startTime := now.AddDate(0, 0, -30)

	if s := c.Query("start_date"); s != "" {
		if t, err := timezone.ParseInLocation("2006-01-02", s); err == nil {
			startTime = t
		}
	}
	if s := c.Query("end_date"); s != "" {
		if t, err := timezone.ParseInLocation("2006-01-02", s); err == nil {
			endTime = t.AddDate(0, 0, 1) // half-open range upper bound
		}
	}
	return startTime, endTime
}

// buildUsageData 构建 today/total 用量摘要
func (h *GatewayHandler) buildUsageData(ctx context.Context, apiKeyID int64) gin.H {
	if h.usageService == nil {
		return nil
	}
	dashStats, err := h.usageService.GetAPIKeyDashboardStats(ctx, apiKeyID)
	if err != nil || dashStats == nil {
		return nil
	}
	return gin.H{
		"today": gin.H{
			"requests":              dashStats.TodayRequests,
			"input_tokens":          dashStats.TodayInputTokens,
			"output_tokens":         dashStats.TodayOutputTokens,
			"cache_creation_tokens": dashStats.TodayCacheCreationTokens,
			"cache_read_tokens":     dashStats.TodayCacheReadTokens,
			"total_tokens":          dashStats.TodayTokens,
			"cost":                  dashStats.TodayCost,
			"actual_cost":           dashStats.TodayActualCost,
		},
		"total": gin.H{
			"requests":              dashStats.TotalRequests,
			"input_tokens":          dashStats.TotalInputTokens,
			"output_tokens":         dashStats.TotalOutputTokens,
			"cache_creation_tokens": dashStats.TotalCacheCreationTokens,
			"cache_read_tokens":     dashStats.TotalCacheReadTokens,
			"total_tokens":          dashStats.TotalTokens,
			"cost":                  dashStats.TotalCost,
			"actual_cost":           dashStats.TotalActualCost,
		},
		"average_duration_ms": dashStats.AverageDurationMs,
		"rpm":                 dashStats.Rpm,
		"tpm":                 dashStats.Tpm,
	}
}

// usageQuotaLimited 处理 quota_limited 模式的响应
func (h *GatewayHandler) usageQuotaLimited(c *gin.Context, ctx context.Context, apiKey *service.APIKey, usageData gin.H, modelStats any) {
	resp := gin.H{
		"mode":    "quota_limited",
		"isValid": apiKey.Status == service.StatusAPIKeyActive || apiKey.Status == service.StatusAPIKeyQuotaExhausted || apiKey.Status == service.StatusAPIKeyExpired,
		"status":  apiKey.Status,
	}

	// 总额度信息
	if apiKey.Quota > 0 {
		remaining := apiKey.GetQuotaRemaining()
		resp["quota"] = gin.H{
			"limit":     apiKey.Quota,
			"used":      apiKey.QuotaUsed,
			"remaining": remaining,
			"unit":      "USD",
		}
		resp["remaining"] = remaining
		resp["unit"] = "USD"
	}

	// 速率限制信息（从 DB 获取实时用量）
	if apiKey.HasRateLimits() && h.apiKeyService != nil {
		rateLimitData, err := h.apiKeyService.GetRateLimitData(ctx, apiKey.ID)
		if err == nil && rateLimitData != nil {
			var rateLimits []gin.H
			if apiKey.RateLimit5h > 0 {
				used := rateLimitData.EffectiveUsage5h()
				entry := gin.H{
					"window":       "5h",
					"limit":        apiKey.RateLimit5h,
					"used":         used,
					"remaining":    max(0, apiKey.RateLimit5h-used),
					"window_start": rateLimitData.Window5hStart,
				}
				if rateLimitData.Window5hStart != nil && !service.IsWindowExpired(rateLimitData.Window5hStart, service.RateLimitWindow5h) {
					entry["reset_at"] = rateLimitData.Window5hStart.Add(service.RateLimitWindow5h)
				}
				rateLimits = append(rateLimits, entry)
			}
			if apiKey.RateLimit1d > 0 {
				used := rateLimitData.EffectiveUsage1d()
				entry := gin.H{
					"window":       "1d",
					"limit":        apiKey.RateLimit1d,
					"used":         used,
					"remaining":    max(0, apiKey.RateLimit1d-used),
					"window_start": rateLimitData.Window1dStart,
				}
				if rateLimitData.Window1dStart != nil && !service.IsWindowExpired(rateLimitData.Window1dStart, service.RateLimitWindow1d) {
					entry["reset_at"] = rateLimitData.Window1dStart.Add(service.RateLimitWindow1d)
				}
				rateLimits = append(rateLimits, entry)
			}
			if apiKey.RateLimit7d > 0 {
				used := rateLimitData.EffectiveUsage7d()
				entry := gin.H{
					"window":       "7d",
					"limit":        apiKey.RateLimit7d,
					"used":         used,
					"remaining":    max(0, apiKey.RateLimit7d-used),
					"window_start": rateLimitData.Window7dStart,
				}
				if rateLimitData.Window7dStart != nil && !service.IsWindowExpired(rateLimitData.Window7dStart, service.RateLimitWindow7d) {
					entry["reset_at"] = rateLimitData.Window7dStart.Add(service.RateLimitWindow7d)
				}
				rateLimits = append(rateLimits, entry)
			}
			if len(rateLimits) > 0 {
				resp["rate_limits"] = rateLimits
			}
		}
	}

	// 过期时间
	if apiKey.ExpiresAt != nil {
		resp["expires_at"] = apiKey.ExpiresAt
		resp["days_until_expiry"] = apiKey.GetDaysUntilExpiry()
	}

	if usageData != nil {
		resp["usage"] = usageData
	}
	if modelStats != nil {
		resp["model_stats"] = modelStats
	}

	c.JSON(http.StatusOK, resp)
}

// usageUnrestricted 处理 unrestricted 模式的响应（向后兼容）
func (h *GatewayHandler) usageUnrestricted(c *gin.Context, ctx context.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, usageData gin.H, modelStats any) {
	// 订阅模式
	if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() {
		resp := gin.H{
			"mode":     "unrestricted",
			"isValid":  true,
			"planName": apiKey.Group.Name,
			"unit":     "USD",
		}

		// 订阅信息可能不在 context 中（/v1/usage 路径跳过了中间件的计费检查）
		subscription, ok := middleware2.GetSubscriptionFromContext(c)
		if ok {
			remaining := h.calculateSubscriptionRemaining(apiKey.Group, subscription)
			resp["remaining"] = remaining
			resp["subscription"] = gin.H{
				"daily_usage_usd":     subscription.DailyUsageUSD,
				"weekly_usage_usd":    subscription.WeeklyUsageUSD,
				"monthly_usage_usd":   subscription.MonthlyUsageUSD,
				"weekly_window_start": subscription.WeeklyWindowStart,
				"daily_limit_usd":     apiKey.Group.DailyLimitUSD,
				"weekly_limit_usd":    apiKey.Group.WeeklyLimitUSD,
				"monthly_limit_usd":   apiKey.Group.MonthlyLimitUSD,
				"expires_at":          subscription.ExpiresAt,
			}
		}

		if usageData != nil {
			resp["usage"] = usageData
		}
		if modelStats != nil {
			resp["model_stats"] = modelStats
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// 余额模式
	latestUser, err := h.userService.GetByID(ctx, subject.UserID)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Failed to get user info")
		return
	}

	resp := gin.H{
		"mode":      "unrestricted",
		"isValid":   true,
		"planName":  "钱包余额",
		"remaining": latestUser.Balance,
		"unit":      "USD",
		"balance":   latestUser.Balance,
	}
	if usageData != nil {
		resp["usage"] = usageData
	}
	if modelStats != nil {
		resp["model_stats"] = modelStats
	}
	c.JSON(http.StatusOK, resp)
}

// WebSearch provides a dedicated web search API using configured providers (e.g. Tavily/Brave).
// Can be used by other agents via direct API calls or wrapped as MCP tool.
// For Grok native search, use /v1/responses with grok model + web_search tool (if emulation disabled for the group).
func (h *GatewayHandler) WebSearch(c *gin.Context) {
	type webSearchReq struct {
		Query      string `json:"query" binding:"required"`
		MaxResults int    `json:"max_results"`
	}

	var req webSearchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"type":    "invalid_request_error",
			"message": err.Error(),
		}})
		return
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 5
	}

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{
			"type":    "authentication_error",
			"message": "API key required",
		}})
		return
	}

	if apiKey.Group == nil || apiKey.Group.Platform != "grok" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"type":    "invalid_request_error",
			"message": "web search is only supported for grok groups",
		}})
		return
	}

	// Billing eligibility (same as other requests)
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		c.JSON(status, gin.H{"error": gin.H{"type": code, "message": message}})
		return
	}

	// Use exactly the same scheduling as other requests (SelectAccountWithLoadAwareness handles load, rate limit, sticky, etc.)
	groupID := apiKey.GroupID
	if groupID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
			"type":    "invalid_request_error",
			"message": "group required",
		}})
		return
	}

	selected, err := h.gatewayService.SelectAccountWithLoadAwareness(c.Request.Context(), groupID, "", xai.DefaultTextModel, nil, "", 0)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
			"type":    "scheduling_error",
			"message": err.Error(),
		}})
		return
	}
	if selected == nil || selected.Account == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
			"type":    "scheduling_error",
			"message": "No available accounts",
		}})
		return
	}
	account := selected.Account
	accountReleaseFunc := selected.ReleaseFunc
	if !selected.Acquired {
		if selected.WaitPlan == nil || h.concurrencyHelper == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
				"type":    "scheduling_error",
				"message": "No available accounts",
			}})
			return
		}
		accountWaitCounted := false
		canWait, waitErr := h.concurrencyHelper.IncrementAccountWaitCount(c.Request.Context(), account.ID, selected.WaitPlan.MaxWaiting)
		if waitErr != nil {
			logger.L().Warn("gateway.web_search.account_wait_counter_increment_failed",
				zap.Int64("account_id", account.ID),
				zap.Error(waitErr),
			)
		} else if !canWait {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
				"type":    "rate_limit_error",
				"message": "Too many pending requests, please retry later",
			}})
			return
		} else {
			accountWaitCounted = true
		}
		releaseWait := func() {
			if accountWaitCounted {
				h.concurrencyHelper.DecrementAccountWaitCount(c.Request.Context(), account.ID)
				accountWaitCounted = false
			}
		}
		streamStarted := false
		release, acquireErr := h.concurrencyHelper.AcquireAccountSlotWithWaitTimeoutForGroup(
			c,
			account.ID,
			apiKey.GroupID,
			selected.WaitPlan.MaxConcurrency,
			selected.WaitPlan.Timeout,
			false,
			&streamStarted,
		)
		releaseWait()
		if acquireErr != nil {
			h.handleConcurrencyError(c, acquireErr, "account", streamStarted)
			return
		}
		accountReleaseFunc = release
	}
	if accountReleaseFunc != nil {
		defer accountReleaseFunc()
	}

	// Scheduling is 100% the same as other requests:
	// SelectAccountWithLoadAwareness handles load balancing, rate limits, failover, sticky sessions, concurrency, proxies etc.
	// Downstream rate limiting, billing etc. can be wired the same way.

	// Use Grok *native* web search via the selected Grok account + responses API + web_search tool.
	// This ensures results come from Grok's own search (not third-party emulation like Tavily/Brave).
	// Output is normalized to the same unified format for clients/agents/MCP.

	nativeResp, providerName, err := h.doGrokNativeWebSearch(c.Request.Context(), c, account, req.Query, req.MaxResults)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{
			"type":    "web_search_error",
			"message": err.Error(),
		}})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
	requestPayloadHash := service.HashUsageRequestPayload([]byte(req.Query))
	quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
	h.submitUsageRecordTask(c.Request.Context(), wrapUsageRecordTaskWithRequestContext(c, func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
			Result: &service.ForwardResult{
				RequestID:   "web_search:" + service.HashUsageRequestPayload([]byte(req.Query)),
				Model:       "grok-web-search",
				SearchCount: 1,
				Duration:    0,
			},
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    inboundEndpoint,
			UpstreamEndpoint:   upstreamEndpoint,
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			RequestPayloadHash: requestPayloadHash,
			APIKeyService:      h.apiKeyService,
			QuotaPlatform:      quotaPlatform,
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.gateway.web_search"),
				zap.Int64("user_id", apiKey.User.ID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Int64("account_id", account.ID),
			).Error("gateway.web_search.record_usage_failed", zap.Error(err))
		}
	}))

	c.JSON(http.StatusOK, gin.H{
		"query":       req.Query,
		"results":     nativeResp.Results,
		"provider":    providerName,
		"max_results": req.MaxResults,
	})
}

// doGrokNativeWebSearch executes web search using the Grok account's native capability
// by calling the responses endpoint with web_search tool, then normalizes sources to unified format.
func (h *GatewayHandler) doGrokNativeWebSearch(ctx context.Context, c *gin.Context, account *service.Account, query string, maxResults int) (*websearch.SearchResponse, string, error) {
	// Build a minimal responses request that triggers Grok web search tool.
	// Grok will perform the search using its backend and return sources in web_search_call or annotations.
	searchBody := map[string]any{
		"model":  xai.DefaultTextModel,
		"input":  query,
		"tools":  []map[string]any{{"type": "web_search"}},
		"store":  false,
		"stream": false,
	}
	bodyBytes, _ := json.Marshal(searchBody)

	respBytes, err := h.gatewayService.DoGrokNativeResponsesJSON(ctx, c, account, bodyBytes)
	if err != nil {
		return nil, "", err
	}

	// Extract sources from Grok responses output.
	// Prefer web_search_call.action.sources (standardized), fallback to annotations or text links.
	results := extractGrokWebSearchSources(respBytes)

	providerName := "grok-native"
	if account.Name != "" {
		providerName = "grok:" + account.Name
	}

	return &websearch.SearchResponse{
		Results: results,
		Query:   query,
	}, providerName, nil
}

// extractGrokWebSearchSources pulls url/title (and optional snippet) from a Grok responses body.
func extractGrokWebSearchSources(body []byte) []websearch.SearchResult {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil
	}
	var out []websearch.SearchResult
	seen := map[string]bool{}

	// 1. Look for web_search_call items
	output := gjson.GetBytes(body, "output")
	output.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "web_search_call" {
			sources := item.Get("action.sources")
			if sources.IsArray() {
				sources.ForEach(func(_, src gjson.Result) bool {
					u := strings.TrimSpace(src.Get("url").String())
					if u == "" || seen[u] {
						return true
					}
					seen[u] = true
					out = append(out, websearch.SearchResult{
						URL:     u,
						Title:   strings.TrimSpace(src.Get("title").String()),
						Snippet: "", // Grok sources typically provide url+title; snippet can be augmented later
					})
					return true
				})
			}
		}
		return true
	})

	// 2. Fallback: annotations on output_text (common in responses with web citations)
	if len(out) == 0 {
		gjson.GetBytes(body, "output").ForEach(func(_, item gjson.Result) bool {
			if item.Get("type").String() == "message" {
				item.Get("content").ForEach(func(_, part gjson.Result) bool {
					if part.Get("type").String() == "output_text" {
						part.Get("annotations").ForEach(func(_, ann gjson.Result) bool {
							if ann.Get("type").String() == "url_citation" || ann.Get("type").String() == "web" {
								u := strings.TrimSpace(ann.Get("url").String())
								if u != "" && !seen[u] {
									seen[u] = true
									out = append(out, websearch.SearchResult{
										URL:     u,
										Title:   strings.TrimSpace(ann.Get("title").String()),
										Snippet: "",
									})
								}
							}
							return true
						})
					}
					return true
				})
			}
			return true
		})
	}

	// limit to max if needed is done by caller; return what Grok gave (usually top results)
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// calculateSubscriptionRemaining 计算订阅剩余可用额度
// 逻辑：
// 1. 如果日/周/月任一限额达到100%，返回0
// 2. 否则返回所有已配置周期中剩余额度的最小值
func (h *GatewayHandler) calculateSubscriptionRemaining(group *service.Group, sub *service.UserSubscription) float64 {
	var remainingValues []float64

	// 检查日限额
	if group.HasDailyLimit() {
		remaining := *group.DailyLimitUSD - sub.DailyUsageUSD
		if remaining <= 0 {
			return 0
		}
		remainingValues = append(remainingValues, remaining)
	}

	// 检查周限额
	if group.HasWeeklyLimit() {
		remaining := *group.WeeklyLimitUSD - sub.WeeklyUsageUSD
		if remaining <= 0 {
			return 0
		}
		remainingValues = append(remainingValues, remaining)
	}

	// 检查月限额
	if group.HasMonthlyLimit() {
		remaining := *group.MonthlyLimitUSD - sub.MonthlyUsageUSD
		if remaining <= 0 {
			return 0
		}
		remainingValues = append(remainingValues, remaining)
	}

	// 如果没有配置任何限额，返回-1表示无限制
	if len(remainingValues) == 0 {
		return -1
	}

	// 返回最小值
	min := remainingValues[0]
	for _, v := range remainingValues[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// handleConcurrencyError handles concurrency-related acquire errors.
func (h *GatewayHandler) handleConcurrencyError(c *gin.Context, err error, slotType string, streamStarted bool) {
	status, errType, message := concurrencyErrorResponse(err, slotType)
	h.handleStreamingAwareError(c, status, errType, message, streamStarted)
}

func gatewayFailoverStreamAlreadyWritten(c *gin.Context, writerSizeBeforeForward int) bool {
	if c == nil || c.Writer == nil {
		return false
	}
	return c.Writer.Size() != writerSizeBeforeForward
}

func (h *GatewayHandler) handleFailoverExhausted(c *gin.Context, failoverErr *service.UpstreamFailoverError, platform string, streamStarted bool) {
	statusCode := failoverErr.StatusCode
	responseBody := failoverErr.ResponseBody
	copyFailoverResponseHeaders(c, failoverErr)

	// 先检查透传规则
	if h.errorPassthroughService != nil && len(responseBody) > 0 {
		if rule := h.errorPassthroughService.MatchRule(platform, statusCode, responseBody); rule != nil {
			// 确定响应状态码
			respCode := statusCode
			if !rule.PassthroughCode && rule.ResponseCode != nil {
				respCode = *rule.ResponseCode
			}

			// 确定响应消息
			msg := service.SanitizeUpstreamErrorMessage(service.ExtractUpstreamErrorMessage(responseBody))
			if !rule.PassthroughBody && rule.CustomMessage != nil {
				msg = *rule.CustomMessage
			}

			if rule.SkipMonitoring {
				c.Set(service.OpsSkipPassthroughKey, true)
			}

			h.handleStreamingAwareError(c, respCode, "upstream_error", msg, streamStarted)
			return
		}
	}

	// 记录原始上游状态码，以便 ops 错误日志捕获真实的上游错误
	upstreamMsg := service.ExtractUpstreamErrorMessage(responseBody)
	service.SetOpsUpstreamError(c, statusCode, upstreamMsg, "")

	// 使用默认的错误映射
	status, errType, errMsg := h.mapUpstreamError(statusCode)
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

// handleFailoverExhaustedSimple 简化版本，用于没有响应体的情况
func (h *GatewayHandler) handleFailoverExhaustedSimple(c *gin.Context, statusCode int, streamStarted bool) {
	status, errType, errMsg := h.mapUpstreamError(statusCode)
	service.SetOpsUpstreamError(c, statusCode, errMsg, "")
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

func (h *GatewayHandler) mapUpstreamError(statusCode int) (int, string, string) {
	switch statusCode {
	case 401:
		return http.StatusBadGateway, "upstream_error", "Upstream authentication failed, please contact administrator"
	case 403:
		return http.StatusBadGateway, "upstream_error", "Upstream access forbidden, please contact administrator"
	case 429:
		return http.StatusTooManyRequests, "rate_limit_error", "Upstream rate limit exceeded, please retry later"
	case 529:
		return http.StatusServiceUnavailable, "overloaded_error", "Upstream service overloaded, please retry later"
	case 500, 502, 503, 504:
		return http.StatusBadGateway, "upstream_error", "Upstream service temporarily unavailable"
	default:
		return http.StatusBadGateway, "upstream_error", "Upstream request failed"
	}
}

// handleStreamingAwareError handles errors that may occur after streaming has started
func (h *GatewayHandler) handleStreamingAwareError(c *gin.Context, status int, errType, message string, streamStarted bool) {
	if streamStarted {
		// 响应状态码已固化为 200（ping/部分数据已 flush），错误只能就地以 SSE 帧回传。
		// 标记本次流内错误，供 ops_error_logger 补记——否则该中间件按 status>=400 采集，
		// 这类挂在 200 流上的失败（如并发限流回退）不会进错误看板。
		service.MarkOpsStreamError(c, errType, message, status)

		// /v1/responses 的严格 SDK（Codex CLI）要求终止事件必须属于
		// response.completed/failed/incomplete/cancelled 集合。
		// Anthropic-backed Responses 路径同样会因为通用 error 帧被拒。
		if inboundIsResponses(c) {
			if writeResponsesFailedSSE(c, errType, message) {
				return
			}
		}
		// Stream already started, send error as SSE event then close
		flusher, ok := c.Writer.(http.Flusher)
		if ok {
			// SSE 错误事件固定 schema，使用 Quote 直拼可避免额外 Marshal 分配。
			errorEvent := `data: {"type":"error","error":{"type":` + strconv.Quote(errType) + `,"message":` + strconv.Quote(message) + `}}` + "\n\n"
			if _, err := fmt.Fprint(c.Writer, errorEvent); err != nil {
				_ = c.Error(err)
			}
			if inboundIsChatCompletions(c) {
				if _, err := fmt.Fprint(c.Writer, "data: [DONE]\n\n"); err != nil {
					_ = c.Error(err)
				}
			}
			flusher.Flush()
		}
		return
	}

	// Normal case: return JSON response with proper status code
	h.errorResponse(c, status, errType, message)
}

// ensureForwardErrorResponse 在 Forward 返回错误但尚未写响应时补写统一错误响应。
// Writer 已写过 SSE 时强制走 streamStarted 分支，这样 /responses 可以回写
// response.failed 而不是 silent EOF；普通 JSON 响应已写出时不能再追加 SSE。
func (h *GatewayHandler) ensureForwardErrorResponse(c *gin.Context, streamStarted bool, forwardErr error) bool {
	if c == nil || c.Writer == nil {
		return false
	}
	if shouldSuppressForwardErrorResponse(c, forwardErr) {
		return false
	}
	if c.Writer.Written() {
		if !openAIStreamingResponseStarted(c, streamStarted) {
			return false
		}
		streamStarted = true
	}
	detail := resolveUpstreamForwardErrorDetail(c, forwardErr)
	service.SetOpsUpstreamErrorWithType(c, detail.ErrorType, 0, detail.Message, detail.Detail)
	h.handleStreamingAwareError(c, http.StatusBadGateway, detail.ErrorType, detail.Message, streamStarted)
	return true
}

// checkClaudeCodeVersion 检查 Claude Code 客户端版本是否满足版本要求
// 仅对已识别的 Claude Code 客户端执行，count_tokens 路径除外
func (h *GatewayHandler) checkClaudeCodeVersion(c *gin.Context) bool {
	ctx := c.Request.Context()
	if !service.IsClaudeCodeClient(ctx) {
		return true
	}

	// 排除 count_tokens 子路径
	if strings.HasSuffix(c.Request.URL.Path, "/count_tokens") {
		return true
	}

	minVersion, maxVersion := h.settingService.GetClaudeCodeVersionBounds(ctx)
	if minVersion == "" && maxVersion == "" {
		return true // 未设置，不检查
	}

	clientVersion := service.GetClaudeCodeVersion(ctx)
	if clientVersion == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error",
			"Unable to determine Claude Code version. Please update Claude Code: npm update -g @anthropic-ai/claude-code")
		return false
	}

	if minVersion != "" && service.CompareVersions(clientVersion, minVersion) < 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("Your Claude Code version (%s) is below the minimum required version (%s). Please update: npm update -g @anthropic-ai/claude-code",
				clientVersion, minVersion))
		return false
	}

	if maxVersion != "" && service.CompareVersions(clientVersion, maxVersion) > 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("Your Claude Code version (%s) exceeds the maximum allowed version (%s). "+
				"Please downgrade: npm install -g @anthropic-ai/claude-code@%s && "+
				"set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 to prevent auto-upgrade",
				clientVersion, maxVersion, maxVersion))
		return false
	}

	return true
}

// errorResponse 返回Claude API格式的错误响应
func (h *GatewayHandler) errorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// CountTokens handles token counting endpoint
// POST /v1/messages/count_tokens
// 特点：校验订阅/余额，但不计算并发、不记录使用量
func (h *GatewayHandler) CountTokens(c *gin.Context) {
	requestStart := time.Now()

	// 从context获取apiKey和user（ApiKeyAuth中间件已设置）
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	_, ok = middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.gateway.count_tokens",
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	defer h.maybeLogCompatibilityFallbackMetrics(reqLog)

	// 读取请求体
	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false, body)

	bodyRef := service.NewRequestBodyRef(body)
	parsedReq, err := service.ParseGatewayRequest(bodyRef, domain.PlatformAnthropic)
	if err != nil {
		logRequestBodyParseFailure(reqLog, body, err)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}
	// count_tokens 走 messages 严格校验时，复用已解析请求，避免二次反序列化。
	SetClaudeCodeClientContext(c, body, parsedReq)
	reqLog = reqLog.With(zap.String("model", parsedReq.Model), zap.Bool("stream", parsedReq.Stream))
	// 在请求上下文中记录 thinking 状态，供 Antigravity 最终模型 key 推导/模型维度限流使用
	c.Request = c.Request.WithContext(service.WithThinkingEnabled(c.Request.Context(), parsedReq.ThinkingEnabled, h.metadataBridgeEnabled()))
	c.Request = c.Request.WithContext(service.WithOutputEffort(c.Request.Context(), parsedReq.OutputEffort))

	// 验证 model 必填
	if parsedReq.Model == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	setOpsRequestContext(c, parsedReq.Model, parsedReq.Stream, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(parsedReq.Stream, false)))

	// 获取订阅信息（可能为nil）
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	// 校验 billing eligibility（订阅/余额）
	// 【注意】不计算并发，但需要校验订阅/余额
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}

	// 计算粘性会话 hash
	parsedReq.SessionContext = &service.SessionContext{
		ClientIP:  ip.GetClientIP(c),
		UserAgent: c.GetHeader("User-Agent"),
		APIKeyID:  apiKey.ID,
	}
	sessionHash := h.gatewayService.GenerateSessionHash(parsedReq)

	// 选择支持该模型的账号
	account, err := h.gatewayService.SelectAccountForModel(c.Request.Context(), apiKey.GroupID, sessionHash, parsedReq.Model)
	if err != nil {
		reqLog.Warn("gateway.count_tokens_select_account_failed", zap.Error(err))
		if h.handleGroupModelUnsupportedError(c, err, false) {
			return
		}
		cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, parsedReq.Model, parsedReq.Model, service.PlatformAnthropic)
		if !cls.ModelNotFound {
			markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
		}
		h.errorResponse(c, cls.Status, cls.ErrType, cls.Message)
		return
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)

	// 转发请求（不记录使用量）
	service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
	forwardStart := time.Now()
	err = h.gatewayService.ForwardCountTokens(c.Request.Context(), c, account, parsedReq)
	recordOpsForwardLatencies(c, time.Since(forwardStart).Milliseconds(), nil, err)
	if err != nil {
		reqLog.Error("gateway.count_tokens_forward_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		// 错误响应已在 ForwardCountTokens 中处理
		return
	}
}

// InterceptType 表示请求拦截类型
type InterceptType int

const (
	InterceptTypeNone              InterceptType = iota
	InterceptTypeWarmup                          // 预热请求（返回 "New Conversation"）
	InterceptTypeSuggestionMode                  // SUGGESTION MODE（返回空字符串）
	InterceptTypeMaxTokensOneHaiku               // max_tokens=1 + haiku 探测请求（返回 "#"）
)

// isHaikuModel 检查模型名称是否包含 "haiku"（大小写不敏感）
func isHaikuModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "haiku")
}

// isMaxTokensOneHaikuRequest 检查是否为 max_tokens=1 + haiku 模型的探测请求
// 这类请求用于 Claude Code 验证 API 连通性（流式/非流式均会出现，如 cc-switch v3.9.0 起的健康检查探测为流式）
// 条件：max_tokens == 1 且 model 包含 "haiku"
func isMaxTokensOneHaikuRequest(model string, maxTokens int) bool {
	return maxTokens == 1 && isHaikuModel(model)
}

func (h *GatewayHandler) handleChannelMonitorProbe(c *gin.Context, body []byte, model string, maxTokens int, isStream bool) bool {
	answer, ok := channelMonitorProbeAnswer(c, body, maxTokens, isStream)
	if !ok {
		return false
	}
	c.JSON(http.StatusOK, gin.H{
		"model":         model,
		"id":            "msg_channel_monitor_probe",
		"type":          "message",
		"role":          "assistant",
		"content":       []gin.H{{"type": "text", "text": answer}},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": gin.H{
			"input_tokens":                1,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens":     0,
			"output_tokens":               1,
			"total_tokens":                2,
		},
	})
	return true
}

func channelMonitorProbeAnswer(c *gin.Context, body []byte, maxTokens int, isStream bool) (string, bool) {
	if !service.IsChannelMonitorProbeContext(c.Request.Context()) ||
		c.GetHeader(service.ChannelMonitorProbeHeaderName) != service.ChannelMonitorProbeHeaderValue ||
		isStream ||
		maxTokens != service.ChannelMonitorProbeMaxTokens {
		return "", false
	}

	prompt, ok := extractDefaultChannelMonitorPrompt(body)
	if !ok {
		return "", false
	}
	if !strings.Contains(prompt, "Calculate and respond with ONLY the number, nothing else.") ||
		!strings.Contains(prompt, "Q: 3 + 5 = ?") ||
		!strings.Contains(prompt, "Q: 12 - 7 = ?") {
		return "", false
	}

	matches := channelMonitorProbeQuestionRegex.FindStringSubmatch(prompt)
	if len(matches) != 4 {
		return "", false
	}
	left, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", false
	}
	right, err := strconv.Atoi(matches[3])
	if err != nil {
		return "", false
	}
	switch matches[2] {
	case "+":
		return strconv.Itoa(left + right), true
	case "-":
		return strconv.Itoa(left - right), true
	default:
		return "", false
	}
}

func extractDefaultChannelMonitorPrompt(body []byte) (string, bool) {
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", false
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
		return "", false
	}
	prompt := strings.TrimSpace(req.Messages[0].Content)
	if prompt == "" {
		return "", false
	}
	return prompt, true
}

// detectInterceptType 检测请求是否需要拦截，返回拦截类型
// 参数说明：
//   - body: 请求体字节
//   - model: 请求的模型名称
//   - maxTokens: max_tokens 值
//   - isClaudeCodeClient: 是否已通过 Claude Code 客户端校验
func detectInterceptType(body []byte, model string, maxTokens int, isClaudeCodeClient bool) InterceptType {
	// 优先检查 max_tokens=1 + haiku 探测请求（流式/非流式均适用）
	if isClaudeCodeClient && isMaxTokensOneHaikuRequest(model, maxTokens) {
		return InterceptTypeMaxTokensOneHaiku
	}

	// 快速检查：如果不包含任何关键字，直接返回
	bodyStr := string(body)
	hasSuggestionMode := strings.Contains(bodyStr, "[SUGGESTION MODE:")
	hasWarmupKeyword := strings.Contains(bodyStr, "title") || strings.Contains(bodyStr, "Warmup")

	if !hasSuggestionMode && !hasWarmupKeyword {
		return InterceptTypeNone
	}

	// 解析请求（只解析一次）
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
		System []struct {
			Text string `json:"text"`
		} `json:"system"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return InterceptTypeNone
	}

	// 检查 SUGGESTION MODE（最后一条 user 消息）
	if hasSuggestionMode && len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1]
		if lastMsg.Role == "user" && len(lastMsg.Content) > 0 &&
			lastMsg.Content[0].Type == "text" &&
			strings.HasPrefix(lastMsg.Content[0].Text, "[SUGGESTION MODE:") {
			return InterceptTypeSuggestionMode
		}
	}

	// 检查 Warmup 请求
	if hasWarmupKeyword {
		// 检查 messages 中的标题提示模式
		for _, msg := range req.Messages {
			for _, content := range msg.Content {
				if content.Type == "text" {
					if strings.Contains(content.Text, "Please write a 5-10 word title for the following conversation:") ||
						content.Text == "Warmup" {
						return InterceptTypeWarmup
					}
				}
			}
		}
		// 检查 system 中的标题提取模式
		for _, sys := range req.System {
			if strings.Contains(sys.Text, "nalyze if this message indicates a new conversation topic. If it does, extract a 2-3 word title") {
				return InterceptTypeWarmup
			}
		}
	}

	return InterceptTypeNone
}

// sendMockInterceptStream 发送流式 mock 响应（用于请求拦截）
func sendMockInterceptStream(c *gin.Context, model string, interceptType InterceptType) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 根据拦截类型决定响应内容
	var msgID string
	var outputTokens int
	var textDeltas []string

	switch interceptType {
	case InterceptTypeSuggestionMode:
		msgID = "msg_mock_suggestion"
		outputTokens = 1
		textDeltas = []string{""} // 空内容
	default: // InterceptTypeWarmup
		msgID = "msg_mock_warmup"
		outputTokens = 2
		textDeltas = []string{"New", " Conversation"}
	}

	// Build message_start event with fixed schema.
	messageStartJSON := `{"type":"message_start","message":{"id":` + strconv.Quote(msgID) + `,"type":"message","role":"assistant","model":` + strconv.Quote(model) + `,"content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":0}}}`

	// Build events
	events := []string{
		`event: message_start` + "\n" + `data: ` + string(messageStartJSON),
		`event: content_block_start` + "\n" + `data: {"content_block":{"text":"","type":"text"},"index":0,"type":"content_block_start"}`,
	}

	// Add text deltas
	for _, text := range textDeltas {
		deltaJSON := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":` + strconv.Quote(text) + `}}`
		events = append(events, `event: content_block_delta`+"\n"+`data: `+string(deltaJSON))
	}

	// Add final events
	messageDeltaJSON := `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":10,"output_tokens":` + strconv.Itoa(outputTokens) + `}}`

	events = append(events,
		`event: content_block_stop`+"\n"+`data: {"index":0,"type":"content_block_stop"}`,
		`event: message_delta`+"\n"+`data: `+string(messageDeltaJSON),
		`event: message_stop`+"\n"+`data: {"type":"message_stop"}`,
	)

	for _, event := range events {
		_, _ = c.Writer.WriteString(event + "\n\n")
		c.Writer.Flush()
		time.Sleep(20 * time.Millisecond)
	}
}

// generateRealisticMsgID 生成仿真的消息 ID（msg_bdrk_XXXXXXX 格式）
// 格式与 Claude API 真实响应一致，24 位随机字母数字
func generateRealisticMsgID() string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const idLen = 24
	randomBytes := make([]byte, idLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("msg_bdrk_%d", time.Now().UnixNano())
	}
	b := make([]byte, idLen)
	for i := range b {
		b[i] = charset[int(randomBytes[i])%len(charset)]
	}
	return "msg_bdrk_" + string(b)
}

// sendMockInterceptResponse 发送非流式 mock 响应（用于请求拦截）
func sendMockInterceptResponse(c *gin.Context, model string, interceptType InterceptType) {
	var msgID, text, stopReason string
	var outputTokens int

	switch interceptType {
	case InterceptTypeSuggestionMode:
		msgID = "msg_mock_suggestion"
		text = ""
		outputTokens = 1
		stopReason = "end_turn"
	case InterceptTypeMaxTokensOneHaiku:
		msgID = generateRealisticMsgID()
		text = "#"
		outputTokens = 1
		stopReason = "max_tokens" // max_tokens=1 探测请求的 stop_reason 应为 max_tokens
	default: // InterceptTypeWarmup
		msgID = "msg_mock_warmup"
		text = "New Conversation"
		outputTokens = 2
		stopReason = "end_turn"
	}

	// 构建完整的响应格式（与 Claude API 响应格式一致）
	response := gin.H{
		"model":         model,
		"id":            msgID,
		"type":          "message",
		"role":          "assistant",
		"content":       []gin.H{{"type": "text", "text": text}},
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage": gin.H{
			"input_tokens":                10,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens":     0,
			"cache_creation": gin.H{
				"ephemeral_5m_input_tokens": 0,
				"ephemeral_1h_input_tokens": 0,
			},
			"output_tokens": outputTokens,
			"total_tokens":  10 + outputTokens,
		},
	}

	c.JSON(http.StatusOK, response)
}

// extractQuotaResetSeconds 从 quota 错误的 metadata 中提取 window_resets_at 并计算
// 距重置剩余秒数。fallback 路径必须返回 ≥1 秒，避免客户端立即重试无限循环。
func extractQuotaResetSeconds(err error) int {
	const fallback = 60
	appErr := pkgerrors.FromError(err)
	if appErr == nil {
		return fallback
	}
	raw, ok := appErr.Metadata["window_resets_at"]
	if !ok || raw == "" {
		return fallback
	}
	resetAt, parseErr := time.Parse(time.RFC3339, raw)
	if parseErr != nil {
		logger.L().With(
			zap.String("component", "handler.gateway.billing"),
			zap.String("raw", raw),
			zap.Error(parseErr),
		).Warn("quota.invalid_window_resets_at_format")
		return fallback
	}
	secs := time.Until(resetAt).Seconds()
	if secs <= 0 {
		// reset 时间已过：cache 与 DB 应该正在自愈，返回 fallback 让客户端按常规节奏退避，
		// 避免返回 1 秒导致客户端立即重试仍触发限额的退避循环。
		return fallback
	}
	return int(math.Ceil(secs))
}

func billingErrorDetails(err error) (status int, code, message string, retryAfter int) {
	if errors.Is(err, service.ErrBillingServiceUnavailable) {
		msg := pkgerrors.Message(err)
		if msg == "" {
			msg = "Billing service temporarily unavailable. Please retry later."
		}
		return http.StatusServiceUnavailable, "billing_service_error", msg, 0
	}
	if errors.Is(err, service.ErrAPIKeyRateLimit5hExceeded) {
		msg := pkgerrors.Message(err)
		return http.StatusTooManyRequests, "rate_limit_exceeded", msg, 0
	}
	if errors.Is(err, service.ErrAPIKeyRateLimit1dExceeded) {
		msg := pkgerrors.Message(err)
		return http.StatusTooManyRequests, "rate_limit_exceeded", msg, 0
	}
	if errors.Is(err, service.ErrAPIKeyRateLimit7dExceeded) {
		msg := pkgerrors.Message(err)
		return http.StatusTooManyRequests, "rate_limit_exceeded", msg, 0
	}
	// 用户/分组 RPM 超限统一映射为 HTTP 429；保留与其它 rate_limit 一致的错误码便于客户端分类。
	// 返回 Retry-After 秒数（当前分钟剩余秒数），让 SDK 自动退避。
	if errors.Is(err, service.ErrGroupRPMExceeded) || errors.Is(err, service.ErrUserRPMExceeded) {
		msg := pkgerrors.Message(err)
		retrySeconds := 60 - int(time.Now().Unix()%60)
		return http.StatusTooManyRequests, "rate_limit_exceeded", msg, retrySeconds
	}
	if errors.Is(err, service.ErrUserPlatformDailyQuotaExhausted) ||
		errors.Is(err, service.ErrUserPlatformWeeklyQuotaExhausted) ||
		errors.Is(err, service.ErrUserPlatformMonthlyQuotaExhausted) {
		// 与 RPM 超限一致映射 429 + Retry-After，让 SDK 自动退避（而非 403 直接失败）。
		// 错误码用 rate_limit_exceeded 与 OpenAI 兼容客户端一致；细分类型由 ErrCode + window_resets_at metadata 区分。
		msg := pkgerrors.Message(err)
		return http.StatusTooManyRequests, "rate_limit_exceeded", msg, extractQuotaResetSeconds(err)
	}
	msg := pkgerrors.Message(err)
	if msg == "" {
		logger.L().With(
			zap.String("component", "handler.gateway.billing"),
			zap.Error(err),
		).Warn("gateway.billing_error_missing_message")
		msg = "Billing error"
	}
	return http.StatusForbidden, "billing_error", msg, 0
}

func (h *GatewayHandler) metadataBridgeEnabled() bool {
	if h == nil || h.cfg == nil {
		return true
	}
	return h.cfg.Gateway.OpenAIWS.MetadataBridgeEnabled
}

func (h *GatewayHandler) maybeLogCompatibilityFallbackMetrics(reqLog *zap.Logger) {
	if reqLog == nil {
		return
	}
	if gatewayCompatibilityMetricsLogCounter.Add(1)%gatewayCompatibilityMetricsLogInterval != 0 {
		return
	}
	metrics := service.SnapshotOpenAICompatibilityFallbackMetrics()
	reqLog.Info("gateway.compatibility_fallback_metrics",
		zap.Int64("session_hash_legacy_read_fallback_total", metrics.SessionHashLegacyReadFallbackTotal),
		zap.Int64("session_hash_legacy_read_fallback_hit", metrics.SessionHashLegacyReadFallbackHit),
		zap.Int64("session_hash_legacy_dual_write_total", metrics.SessionHashLegacyDualWriteTotal),
		zap.Float64("session_hash_legacy_read_hit_rate", metrics.SessionHashLegacyReadHitRate),
		zap.Int64("metadata_legacy_fallback_total", metrics.MetadataLegacyFallbackTotal),
	)
}

func (h *GatewayHandler) submitUsageRecordTask(parent context.Context, task service.UsageRecordTask) {
	if task == nil {
		return
	}
	task = wrapUsageRecordTaskContext(parent, task)
	if h.usageRecordWorkerPool != nil {
		if mode := h.usageRecordWorkerPool.Submit(task); mode != service.UsageRecordSubmitModeDropped {
			return
		}
		logger.L().With(
			zap.String("component", "handler.gateway.messages"),
		).Warn("gateway.usage_record_task_mandatory_sync_fallback")
	}
	// 回退路径：worker 池未注入时同步执行，避免退回到无界 goroutine 模式。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", "handler.gateway.messages"),
				zap.Any("panic", recovered),
			).Error("gateway.usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}

func (h *GatewayHandler) emitGatewayDebugTimelineRequestReceived(c *gin.Context, platform, endpointKind string, requestStart time.Time, apiKey *service.APIKey, userID int64, requestedModel string, stream bool, body []byte) {
	if c == nil || c.Request == nil || !service.GatewayDebugTimelineEnabled(c.Request.Context(), h.settingService) {
		return
	}
	if !shouldCaptureGatewayDebugTimelineBody(platform) {
		fields := gatewayDebugTimelineFields(c, apiKey, nil)
		fields["component"] = "gateway_debug_timeline"
		fields["platform"] = strings.TrimSpace(platform)
		fields["endpoint_kind"] = strings.TrimSpace(endpointKind)
		fields["user_id"] = userID
		fields["requested_model"] = strings.TrimSpace(requestedModel)
		fields["stream"] = stream
		fields["request_body_bytes"] = len(body)
		fields["request_start_unix_ms"] = requestStart.UnixMilli()
		fields["request_elapsed_ms"] = time.Since(requestStart).Milliseconds()
		fields["inbound_endpoint"] = GetInboundEndpoint(c)
		service.WriteGatewayDebugTimelineEvent(h.settingService, c, "request_received", fields)
		return
	}
	fields := gatewayDebugTimelineFields(c, apiKey, nil)
	fields["component"] = "gateway_debug_timeline"
	fields["platform"] = strings.TrimSpace(platform)
	fields["endpoint_kind"] = strings.TrimSpace(endpointKind)
	fields["user_id"] = userID
	fields["requested_model"] = strings.TrimSpace(requestedModel)
	fields["stream"] = stream
	fields["request_body_bytes"] = len(body)
	fields["request_start_unix_ms"] = requestStart.UnixMilli()
	fields["request_elapsed_ms"] = time.Since(requestStart).Milliseconds()
	fields["inbound_endpoint"] = GetInboundEndpoint(c)
	contentType := ""
	if c.Request != nil {
		contentType = c.Request.Header.Get("Content-Type")
	}
	service.RecordGatewayDebugTimelineBody(h.settingService, c, "request_received", body, contentType, fields)
}

// shouldCaptureGatewayDebugTimelineBody limits body capture to the platforms
// whose end-to-end traces actually need it (Claude + Kiro). For other
// platforms the timeline keeps its existing metadata-only entry.
func shouldCaptureGatewayDebugTimelineBody(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "anthropic", "kiro":
		return true
	default:
		return false
	}
}

func (h *GatewayHandler) emitGatewayDebugTimelineAccountSelected(c *gin.Context, platform, endpointKind string, requestStart time.Time, apiKey *service.APIKey, account *service.Account, requestedModel string, stream bool, switchCount int) {
	if c == nil || c.Request == nil || !service.GatewayDebugTimelineEnabled(c.Request.Context(), h.settingService) {
		return
	}
	fields := gatewayDebugTimelineFields(c, apiKey, account)
	fields["component"] = "gateway_debug_timeline"
	fields["platform"] = strings.TrimSpace(platform)
	fields["endpoint_kind"] = strings.TrimSpace(endpointKind)
	fields["requested_model"] = strings.TrimSpace(requestedModel)
	fields["stream"] = stream
	fields["switch_count_before_attempt"] = switchCount
	fields["request_elapsed_ms"] = time.Since(requestStart).Milliseconds()
	service.WriteGatewayDebugTimelineEvent(h.settingService, c, "account_selected", fields)
}

func (h *GatewayHandler) emitGatewayDebugTimelineSlotAcquired(c *gin.Context, platform, endpointKind string, requestStart time.Time, apiKey *service.APIKey, account *service.Account, requestedModel string, stream bool, userSlotWaitMs int64, accountSlotWaitMs int64) {
	if c == nil || c.Request == nil || !service.GatewayDebugTimelineEnabled(c.Request.Context(), h.settingService) {
		return
	}
	fields := gatewayDebugTimelineFields(c, apiKey, account)
	fields["component"] = "gateway_debug_timeline"
	fields["platform"] = strings.TrimSpace(platform)
	fields["endpoint_kind"] = strings.TrimSpace(endpointKind)
	fields["requested_model"] = strings.TrimSpace(requestedModel)
	fields["stream"] = stream
	fields["user_slot_wait_ms"] = userSlotWaitMs
	fields["account_slot_wait_ms"] = accountSlotWaitMs
	fields["request_elapsed_ms"] = time.Since(requestStart).Milliseconds()
	service.WriteGatewayDebugTimelineEvent(h.settingService, c, "concurrency_slots_acquired", fields)
}

func (h *GatewayHandler) emitGatewayDebugTimelineAttemptFinished(c *gin.Context, platform, endpointKind, outcome string, requestStart time.Time, apiKey *service.APIKey, account *service.Account, requestedModel string, stream bool, switchCount int, forwardDurationMs int64, result *service.ForwardResult, err error) {
	if c == nil || c.Request == nil || !service.GatewayDebugTimelineEnabled(c.Request.Context(), h.settingService) {
		return
	}
	fields := gatewayDebugTimelineFields(c, apiKey, account)
	fields["component"] = "gateway_debug_timeline"
	fields["platform"] = strings.TrimSpace(platform)
	fields["endpoint_kind"] = strings.TrimSpace(endpointKind)
	fields["outcome"] = strings.TrimSpace(outcome)
	fields["requested_model"] = strings.TrimSpace(requestedModel)
	fields["stream"] = stream
	fields["switch_count_before_attempt"] = switchCount
	fields["forward_duration_ms"] = forwardDurationMs
	fields["request_elapsed_ms"] = time.Since(requestStart).Milliseconds()
	if result != nil {
		fields["upstream_request_id"] = strings.TrimSpace(result.RequestID)
		fields["upstream_model"] = strings.TrimSpace(result.UpstreamModel)
		fields["duration_ms"] = result.Duration.Milliseconds()
		if result.FirstTokenMs != nil {
			fields["first_forwardable_event_ms"] = *result.FirstTokenMs
		}
		fields["usage_input_tokens"] = result.Usage.InputTokens
		fields["usage_cache_read_tokens"] = result.Usage.CacheReadInputTokens
		fields["usage_output_tokens"] = result.Usage.OutputTokens
	}
	if err != nil {
		fields["error"] = truncateString(sanitizeUpstreamForwardErrorDetail(err.Error()), 512)
	}
	service.WriteGatewayDebugTimelineEvent(h.settingService, c, "attempt_finished", fields)
}

func gatewayDebugTimelineFields(c *gin.Context, apiKey *service.APIKey, account *service.Account) map[string]any {
	fields := make(map[string]any, 12)
	if apiKey != nil {
		fields["api_key_id"] = apiKey.ID
		if apiKey.GroupID != nil {
			fields["group_id"] = *apiKey.GroupID
		}
	}
	if account != nil {
		fields["account_id"] = account.ID
		fields["account_name"] = strings.TrimSpace(account.Name)
		fields["account_type"] = strings.TrimSpace(string(account.Type))
		fields["account_platform"] = strings.TrimSpace(account.Platform)
		fields["account_concurrency"] = account.Concurrency
		fields["upstream_endpoint"] = GetUpstreamEndpoint(c, account.Platform)
	}
	return fields
}

// getUserMsgQueueMode 获取当前请求的 UMQ 模式
// 返回 "serialize" | "throttle" | ""
func (h *GatewayHandler) getUserMsgQueueMode(account *service.Account, parsed *service.ParsedRequest) string {
	if h.userMsgQueueHelper == nil {
		return ""
	}
	// 仅适用于 Anthropic OAuth/SetupToken 账号
	if !account.IsAnthropicOAuthOrSetupToken() {
		return ""
	}
	if !service.IsRealUserMessage(parsed) {
		return ""
	}
	// 账号级模式优先，fallback 到全局配置
	mode := account.GetUserMsgQueueMode()
	if mode == "" {
		mode = h.cfg.Gateway.UserMessageQueue.GetEffectiveMode()
	}
	return mode
}
