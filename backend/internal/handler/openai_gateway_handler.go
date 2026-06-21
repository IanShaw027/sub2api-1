package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// OpenAIGatewayHandler handles OpenAI API gateway requests
type OpenAIGatewayHandler struct {
	gatewayService           *service.OpenAIGatewayService
	billingCacheService      *service.BillingCacheService
	apiKeyService            *service.APIKeyService
	usageRecordWorkerPool    *service.UsageRecordWorkerPool
	errorPassthroughService  *service.ErrorPassthroughService
	contentModerationService *service.ContentModerationService
	concurrencyHelper        *ConcurrencyHelper
	imageLimiter             *imageConcurrencyLimiter
	maxAccountSwitches       int
	cfg                      *config.Config
}

const openAIStreamRetryReplayStateContextKey = "openai_stream_retry_replay_state"

var (
	errOpenAIWSLocalImageToggleUnavailable = errors.New("openai websocket local image-toggle unavailable")
	errOpenAIWSTurnAccountUnavailable      = errors.New("openai websocket turn account unavailable")
)

func (h *OpenAIGatewayHandler) handleOpenAIGroupModelUnsupportedError(c *gin.Context, err error, streamStarted bool) bool {
	var modelErr *service.GroupModelUnsupportedError
	if !errors.As(err, &modelErr) {
		return false
	}
	h.handleStreamingAwareError(c, http.StatusForbidden, "permission_error", modelErr.Error(), streamStarted)
	return true
}

type accountSlotAcquireStatus int

const (
	accountSlotAcquireFailed accountSlotAcquireStatus = iota
	accountSlotAcquireAcquired
	accountSlotAcquireRetry
)

func resolveOpenAIMessagesDispatchMappedModel(apiKey *service.APIKey, requestedModel string) string {
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return strings.TrimSpace(apiKey.Group.ResolveMessagesDispatchModel(requestedModel))
}

func resolveOpenAIMessagesDispatchForcedModel(apiKey *service.APIKey, requestedModel string) string {
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	mappedModel, explicit := apiKey.Group.ResolveMessagesDispatchModelWithSource(requestedModel)
	if !explicit {
		return ""
	}
	return strings.TrimSpace(mappedModel)
}

// NewOpenAIGatewayHandler creates a new OpenAIGatewayHandler
func NewOpenAIGatewayHandler(
	gatewayService *service.OpenAIGatewayService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	contentModerationService *service.ContentModerationService,
	cfg *config.Config,
) *OpenAIGatewayHandler {
	pingInterval := time.Duration(0)
	maxAccountSwitches := 3
	if cfg != nil {
		pingInterval = time.Duration(cfg.Concurrency.PingInterval) * time.Second
		if cfg.Gateway.MaxAccountSwitches > 0 {
			maxAccountSwitches = cfg.Gateway.MaxAccountSwitches
		}
	}
	return &OpenAIGatewayHandler{
		gatewayService:           gatewayService,
		billingCacheService:      billingCacheService,
		apiKeyService:            apiKeyService,
		usageRecordWorkerPool:    usageRecordWorkerPool,
		errorPassthroughService:  errorPassthroughService,
		contentModerationService: contentModerationService,
		concurrencyHelper:        NewConcurrencyHelper(concurrencyService, SSEPingFormatComment, pingInterval),
		imageLimiter:             &imageConcurrencyLimiter{},
		maxAccountSwitches:       maxAccountSwitches,
		cfg:                      cfg,
	}
}

// Responses handles OpenAI Responses API endpoint
// POST /openai/v1/responses
func (h *OpenAIGatewayHandler) Responses(c *gin.Context) {
	// 局部兜底：确保该 handler 内部任何 panic 都不会击穿到进程级。
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	compactStartedAt := time.Now()
	defer h.logOpenAIRemoteCompactOutcome(c, compactStartedAt)
	setOpenAIClientTransportHTTP(c)

	requestStart := time.Now()

	// Get apiKey and user from context (set by ApiKeyAuth middleware)
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
		"handler.openai_gateway.responses",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	// Read request body
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
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
	sessionHashBody := body
	if service.IsOpenAIResponsesCompactPathForTest(c) {
		if compactSeed := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()); compactSeed != "" {
			c.Set(service.OpenAICompactSessionSeedKeyForTest(), compactSeed)
		}
		normalizedCompactBody, normalizedCompact, compactErr := service.NormalizeOpenAICompactRequestBodyForTest(body)
		if compactErr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to normalize compact request body")
			return
		}
		if normalizedCompact {
			body = normalizedCompactBody
		}
	}

	// 校验请求体 JSON 合法性
	if !gjson.ValidBytes(body) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	// 使用 gjson 只读提取字段做校验，避免完整 Unmarshal
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()

	reqStream, ok := parseOpenAIStreamField(body)
	if !ok {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "invalid stream field type")
		return
	}
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	previousResponseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	previousResponseIDKind := ""
	if previousResponseID != "" {
		previousResponseIDKind = service.ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
		reqLog = reqLog.With(
			zap.Bool("has_previous_response_id", true),
			zap.String("previous_response_id_kind", previousResponseIDKind),
			zap.Int("previous_response_id_len", len(previousResponseID)),
		)
		if previousResponseIDKind == service.OpenAIPreviousResponseIDKindMessageID {
			reqLog.Warn("openai.request_validation_failed",
				zap.String("reason", "previous_response_id_looks_like_message_id"),
			)
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "previous_response_id must be a response.id (resp_*), not a message id")
			return
		}
	}
	h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
		Stage:          "request_received",
		EndpointKind:   "responses",
		RequestStart:   requestStart,
		APIKey:         apiKey,
		UserID:         subject.UserID,
		RequestedModel: reqModel,
		Stream:         reqStream,
		Fields: map[string]any{
			"inbound_endpoint":             GetInboundEndpoint(c),
			"request_body_bytes":           len(body),
			"previous_response_id_present": previousResponseID != "",
			"previous_response_id_kind":    previousResponseIDKind,
			"has_prompt_cache_key":         strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()) != "",
			"has_tools":                    gjson.GetBytes(body, "tools").Exists(),
		},
	})

	setOpsRequestContext(c, reqModel, reqStream, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))

	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, reqModel, body); decision != nil && decision.Blocked {
		h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return
	}

	allowImageGeneration := service.GroupAllowsImageGeneration(apiKey.Group)
	rawImageIntent, _ := classifyOpenAIResponsesImageRequest(allowImageGeneration, reqModel, body)
	if rawImageIntent && !allowImageGeneration {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}

	// 解析渠道级模型映射
	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)
	forwardPreviewBody, routedPreviewModel := h.applyOpenAIResponsesChannelMapping(body, reqModel, channelMapping)
	previewImageIntent, _ := classifyOpenAIResponsesImageRequest(allowImageGeneration, routedPreviewModel, forwardPreviewBody)
	if previewImageIntent && !allowImageGeneration {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}

	// 提前校验 function_call_output 是否具备可关联上下文，避免上游 400。
	if !h.validateFunctionCallOutputRequest(c, body, reqLog) {
		return
	}

	// 绑定错误透传服务，允许 service 层在非 failover 错误场景复用规则。
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	// Get subscription info (may be nil)
	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userSlotStart := time.Now()
	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted, reqLog)
	userSlotWaitMs := time.Since(userSlotStart).Milliseconds()
	if !acquired {
		return
	}
	// 确保请求取消时也会释放槽位，避免长连接被动中断造成泄漏
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	// 2. Re-check billing eligibility after wait
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.handleStreamingAwareError(c, status, code, message, streamStarted)
		return
	}
	if service.IsGroupContextValid(apiKey.Group) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, apiKey.Group))
	}

	// Generate session hash (header first; fallback to prompt_cache_key)
	sessionHash := h.gatewayService.GenerateSessionHash(c, sessionHashBody)
	requireCompact := isOpenAIRemoteCompactPath(c)

	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError
	var lastFailoverAccount *service.Account

	for {
		// Select account supporting the requested model
		reqLog.Debug("openai.account_selecting", zap.Int("excluded_account_count", len(failedAccountIDs)))
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForResponses(
			c.Request.Context(),
			apiKey.GroupID,
			apiKey.ID,
			previousResponseID,
			sessionHash,
			reqModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportAny,
			previewImageIntent,
			requireCompact,
		)
		if err != nil {
			reqLog.Warn("openai.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if lastFailoverErr == nil {
				if h.handleOpenAIGroupModelUnsupportedError(c, err, streamStarted) {
					return
				}
				if errors.Is(err, service.ErrNoAvailableCompactAccounts) {
					h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "compact_not_supported", "No available OpenAI accounts support /responses/compact", streamStarted)
					return
				}
				msg := buildOpenAISelectionFailureMessage(err, "Service temporarily unavailable")
				h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", msg, streamStarted)
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, lastFailoverAccount, streamStarted)
			} else {
				h.handleFailoverExhaustedSimple(c, 502, streamStarted)
			}
			return
		}
		if selection == nil || selection.Account == nil {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStarted)
			return
		}
		if previousResponseID != "" && selection != nil && selection.Account != nil {
			reqLog.Debug("openai.account_selected_with_previous_response_id", zap.Int64("account_id", selection.Account.ID))
		}
		reqLog.Debug("openai.account_schedule_decision",
			zap.String("layer", scheduleDecision.Layer),
			zap.Bool("sticky_previous_hit", scheduleDecision.StickyPreviousHit),
			zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
			zap.Int("candidate_count", scheduleDecision.CandidateCount),
			zap.Int("top_k", scheduleDecision.TopK),
			zap.Int64("latency_ms", scheduleDecision.LatencyMs),
			zap.Float64("load_skew", scheduleDecision.LoadSkew),
		)
		account := selection.Account
		sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
		reqLog.Debug("openai.account_selected", zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
		setOpsSelectedAccount(c, account.ID, account.Platform)
		h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
			Stage:          "account_selected",
			EndpointKind:   "responses",
			RequestStart:   requestStart,
			APIKey:         apiKey,
			Account:        account,
			UserID:         subject.UserID,
			RequestedModel: reqModel,
			Stream:         reqStream,
			SwitchCount:    switchCount,
			Fields: map[string]any{
				"inbound_endpoint":             GetInboundEndpoint(c),
				"previous_response_id_present": previousResponseID != "",
				"previous_response_id_kind":    previousResponseIDKind,
				"session_hash_present":         strings.TrimSpace(sessionHash) != "",
				"require_compact":              requireCompact,
				"scheduler_layer":              scheduleDecision.Layer,
				"sticky_previous_hit":          scheduleDecision.StickyPreviousHit,
				"sticky_session_hit":           scheduleDecision.StickySessionHit,
				"candidate_count":              scheduleDecision.CandidateCount,
				"top_k":                        scheduleDecision.TopK,
				"scheduler_latency_ms":         scheduleDecision.LatencyMs,
				"load_skew":                    scheduleDecision.LoadSkew,
			},
		})

		forwardBody := forwardPreviewBody
		effectiveModel := resolveOpenAIResponsesEffectiveModel(account, routedPreviewModel)
		effectiveImageIntent, shouldAcquireImageSlot := classifyOpenAIResponsesImageRequest(allowImageGeneration, effectiveModel, forwardBody)
		if effectiveImageIntent && !allowImageGeneration {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
			return
		}
		if !openAIResponsesAccountSupportsImageIntent(account, apiKey.Group, effectiveImageIntent) {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			_ = h.gatewayService.ClearStickySession(c.Request.Context(), apiKey.GroupID, sessionHash)
			_ = h.gatewayService.ClearPreviousResponseBinding(c.Request.Context(), apiKey.GroupID, apiKey.ID, previousResponseID)
			failedAccountIDs[account.ID] = struct{}{}
			continue
		}

		accountSlotStart := time.Now()
		accountReleaseFunc, acquireStatus := h.acquireResponsesAccountSlot(c, apiKey.GroupID, apiKey.ID, sessionHash, previousResponseID, selection, reqStream, &streamStarted, reqLog)
		accountSlotWaitMs := time.Since(accountSlotStart).Milliseconds()
		if acquireStatus == accountSlotAcquireRetry {
			failedAccountIDs[account.ID] = struct{}{}
			continue
		}
		if acquireStatus != accountSlotAcquireAcquired {
			return
		}
		slotFields := map[string]any{
			"inbound_endpoint":             GetInboundEndpoint(c),
			"user_slot_wait_ms":            userSlotWaitMs,
			"account_slot_wait_ms":         accountSlotWaitMs,
			"previous_response_id_present": previousResponseID != "",
			"session_hash_present":         strings.TrimSpace(sessionHash) != "",
		}
		if selection.WaitPlan != nil {
			slotFields["account_wait_timeout_ms"] = selection.WaitPlan.Timeout.Milliseconds()
			slotFields["account_max_concurrency"] = selection.WaitPlan.MaxConcurrency
			slotFields["account_max_waiting"] = selection.WaitPlan.MaxWaiting
		}
		h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
			Stage:          "concurrency_slots_acquired",
			EndpointKind:   "responses",
			RequestStart:   requestStart,
			APIKey:         apiKey,
			Account:        account,
			UserID:         subject.UserID,
			RequestedModel: reqModel,
			Stream:         reqStream,
			SwitchCount:    switchCount,
			Fields:         slotFields,
		})

		// Forward request
		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		var imageReleaseFunc func()
		if shouldAcquireImageSlot {
			var imageAcquired bool
			imageReleaseFunc, imageAcquired = h.acquireImageGenerationSlot(c, streamStarted)
			if !imageAcquired {
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
				return
			}
		}
		// 记录 Forward 之前已写出的字节数。若 Forward 在写出任意 SSE 帧
		// （含 response.created 等终态前置帧）后才返回 failover 错误，
		// 跨账号切换并重放会导致同一连接出现两份 response.created/failed 终态帧，
		// Codex CLI 会报 "response.created received twice"。
		// 注意：仅用于跨账号 switch 守卫；同账号 pool-mode 重试依赖
		// service 层 replay 去重，不能在此拦截。
		writerSizeBeforeForward := c.Writer.Size()
		result, err := h.gatewayService.Forward(c.Request.Context(), c, account, forwardBody)
		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		if imageReleaseFunc != nil {
			imageReleaseFunc()
		}
		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
		if err == nil && result != nil && result.FirstTokenMs != nil {
			service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
		}
		if err != nil {
			if result != nil && result.ImageCount > 0 {
				reqLog.Warn("openai.forward_partial_error_with_image_result",
					zap.Int64("account_id", account.ID),
					zap.Int("image_count", result.ImageCount),
					zap.Error(err),
				)
			} else {
				var failoverErr *service.UpstreamFailoverError
				if errors.As(err, &failoverErr) {
					h.reportOpenAIAccountScheduleFailure(account.ID, err)
					// 池模式：同账号重试
					if failoverErr.RetryableOnSameAccount {
						retryLimit := account.GetPoolModeRetryCount()
						if sameAccountRetryCount[account.ID] < retryLimit {
							sameAccountRetryCount[account.ID]++
							h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
								Stage:             "attempt_finished",
								EndpointKind:      "responses",
								RequestStart:      requestStart,
								APIKey:            apiKey,
								Account:           account,
								UserID:            subject.UserID,
								RequestedModel:    reqModel,
								Stream:            reqStream,
								SwitchCount:       switchCount,
								ForwardDurationMs: forwardDurationMs,
								OpenAIResult:      result,
								Err:               err,
								Fields: map[string]any{
									"outcome":                  "retry_same_account",
									"inbound_endpoint":         GetInboundEndpoint(c),
									"upstream_status":          failoverErr.StatusCode,
									"same_account_retry_count": sameAccountRetryCount[account.ID],
									"same_account_retry_limit": retryLimit,
								},
							})
							reqLog.Warn("openai.pool_mode_same_account_retry",
								zap.Int64("account_id", account.ID),
								zap.Int("upstream_status", failoverErr.StatusCode),
								zap.Int("retry_limit", retryLimit),
								zap.Int("retry_count", sameAccountRetryCount[account.ID]),
							)
							select {
							case <-c.Request.Context().Done():
								return
							case <-time.After(sameAccountRetryDelay):
							}
							continue
						}
					}
					h.gatewayService.RecordOpenAIAccountSwitch()
					failedAccountIDs[account.ID] = struct{}{}
					lastFailoverErr = failoverErr
					lastFailoverAccount = account
					h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
						Stage:             "attempt_finished",
						EndpointKind:      "responses",
						RequestStart:      requestStart,
						APIKey:            apiKey,
						Account:           account,
						UserID:            subject.UserID,
						RequestedModel:    reqModel,
						Stream:            reqStream,
						SwitchCount:       switchCount,
						ForwardDurationMs: forwardDurationMs,
						OpenAIResult:      result,
						Err:               err,
						Fields: map[string]any{
							"outcome":            "failover",
							"inbound_endpoint":   GetInboundEndpoint(c),
							"upstream_status":    failoverErr.StatusCode,
							"writer_size_before": writerSizeBeforeForward,
							"writer_size_after":  c.Writer.Size(),
						},
					})
					// 跨账号切换前的终态帧守卫：若本次 Forward 已向客户端写出
					// 任意 SSE 帧，切换到下一个账号重放会产生重复的
					// response.created/failed 终态帧（Codex CLI 报
					// "response.created received twice"）。此时不再 continue，
					// 直接按 failover 耗尽收口。同账号 pool-mode 重试在上面已经
					// continue，不会走到这里，因此其 replay 去重不受影响。
					if c.Writer.Size() != writerSizeBeforeForward {
						reqLog.Warn("openai.failover_blocked_after_stream_started",
							zap.Int64("account_id", account.ID),
							zap.Int("upstream_status", failoverErr.StatusCode),
						)
						h.handleFailoverExhausted(c, failoverErr, account, streamStarted)
						return
					}
					if switchCount >= maxAccountSwitches {
						h.handleFailoverExhausted(c, failoverErr, account, streamStarted)
						return
					}
					service.ClearOpenAICompatRequestState(c)
					switchCount++
					if h.gatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, switchCount) {
						h.handleFailoverExhausted(c, failoverErr, account, streamStarted)
						return
					}
					reqLog.Warn("openai.upstream_failover_switching",
						zap.Int64("account_id", account.ID),
						zap.Int("upstream_status", failoverErr.StatusCode),
						zap.Int("switch_count", switchCount),
						zap.Int("max_switches", maxAccountSwitches),
					)
					continue
				}
				h.reportOpenAIAccountScheduleFailure(account.ID, err)
				if shouldSuppressForwardErrorResponse(c, err) {
					return
				}
				upstreamErrorAlreadyCommunicated := c.Writer.Written()
				wroteFallback := h.ensureForwardErrorResponse(c, streamStarted, err)
				h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
					Stage:             "attempt_finished",
					EndpointKind:      "responses",
					RequestStart:      requestStart,
					APIKey:            apiKey,
					Account:           account,
					UserID:            subject.UserID,
					RequestedModel:    reqModel,
					Stream:            reqStream,
					SwitchCount:       switchCount,
					ForwardDurationMs: forwardDurationMs,
					OpenAIResult:      result,
					Err:               err,
					Fields: map[string]any{
						"outcome":                                  "error",
						"inbound_endpoint":                         GetInboundEndpoint(c),
						"fallback_error_response_written":          wroteFallback,
						"upstream_error_response_already_written":  upstreamErrorAlreadyCommunicated,
						"stream_started":                           streamStarted,
						"previous_response_id_present":             previousResponseID != "",
						"writer_size_changed_during_forward":       c.Writer.Size() != writerSizeBeforeForward,
						"writer_size_before_forward":               writerSizeBeforeForward,
						"writer_size_after_forward_error_response": c.Writer.Size(),
					},
				})
				fields := []zap.Field{
					zap.Int64("account_id", account.ID),
					zap.Bool("fallback_error_response_written", wroteFallback),
					zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
					zap.Error(err),
				}
				if shouldLogOpenAIForwardFailureAsWarn(c, wroteFallback) {
					reqLog.Warn("openai.forward_failed", fields...)
					return
				}
				reqLog.Error("openai.forward_failed", fields...)
				return
			}
		}
		if result != nil {
			if account.Type == service.AccountTypeOAuth {
				h.gatewayService.UpdateCodexUsageSnapshotFromHeaders(c.Request.Context(), account.ID, result.ResponseHeaders)
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
		} else {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
		}
		h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
			Stage:             "attempt_finished",
			EndpointKind:      "responses",
			RequestStart:      requestStart,
			APIKey:            apiKey,
			Account:           account,
			UserID:            subject.UserID,
			RequestedModel:    reqModel,
			Stream:            reqStream,
			SwitchCount:       switchCount,
			ForwardDurationMs: forwardDurationMs,
			OpenAIResult:      result,
			Err:               err,
			Fields: map[string]any{
				"outcome":                      "success",
				"inbound_endpoint":             GetInboundEndpoint(c),
				"previous_response_id_present": previousResponseID != "",
				"previous_response_id_kind":    previousResponseIDKind,
				"session_hash_present":         strings.TrimSpace(sessionHash) != "",
				"routed_model":                 routedPreviewModel,
				"effective_model":              effectiveModel,
			},
		})

		// 捕获请求信息（用于异步记录，避免在 goroutine 中访问 gin.Context）
		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(body)
		inboundEndpoint := GetInboundEndpoint(c)
		upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
		quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)

		// 使用量记录通过有界 worker 池提交，避免请求热路径创建无界 goroutine。
		h.submitOpenAIUsageRecordTask(c.Request.Context(), result, func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:             result,
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
				ChannelUsageFields: channelMapping.ToUsageFields(reqModel, result.UpstreamModel),
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.responses"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.Any("group_id", apiKey.GroupID),
					zap.String("model", reqModel),
					zap.Int64("account_id", account.ID),
				).Error("openai.record_usage_failed", zap.Error(err))
			}
		})
		reqLog.Debug("openai.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return
	}
}

func isOpenAIRemoteCompactPath(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	normalizedPath := strings.TrimRight(strings.TrimSpace(c.Request.URL.Path), "/")
	return strings.Contains(normalizedPath, "/responses/compact")
}

func (h *OpenAIGatewayHandler) logOpenAIRemoteCompactOutcome(c *gin.Context, startedAt time.Time) {
	if !isOpenAIRemoteCompactPath(c) {
		return
	}

	var (
		ctx    = context.Background()
		path   string
		status int
	)
	if c != nil {
		if c.Request != nil {
			ctx = c.Request.Context()
			if c.Request.URL != nil {
				path = strings.TrimSpace(c.Request.URL.Path)
			}
		}
		if c.Writer != nil {
			status = c.Writer.Status()
		}
	}

	outcome := "failed"
	if status >= 200 && status < 300 {
		outcome = "succeeded"
	}
	latencyMs := time.Since(startedAt).Milliseconds()
	if latencyMs < 0 {
		latencyMs = 0
	}

	fields := []zap.Field{
		zap.String("component", "handler.openai_gateway.responses"),
		zap.Bool("remote_compact", true),
		zap.String("compact_outcome", outcome),
		zap.Int("status_code", status),
		zap.Int64("latency_ms", latencyMs),
		zap.String("path", path),
		zap.Bool("force_codex_cli", h != nil && h.cfg != nil && h.cfg.Gateway.ForceCodexCLI),
	}

	if c != nil {
		if userAgent := strings.TrimSpace(c.GetHeader("User-Agent")); userAgent != "" {
			fields = append(fields, zap.String("request_user_agent", userAgent))
		}
		if v, ok := c.Get(opsModelKey); ok {
			if model, ok := v.(string); ok && strings.TrimSpace(model) != "" {
				fields = append(fields, zap.String("request_model", strings.TrimSpace(model)))
			}
		}
		if v, ok := c.Get(opsAccountIDKey); ok {
			if accountID, ok := v.(int64); ok && accountID > 0 {
				fields = append(fields, zap.Int64("account_id", accountID))
			}
		}
		if c.Writer != nil {
			if upstreamRequestID := strings.TrimSpace(c.Writer.Header().Get("x-request-id")); upstreamRequestID != "" {
				fields = append(fields, zap.String("upstream_request_id", upstreamRequestID))
			} else if upstreamRequestID := strings.TrimSpace(c.Writer.Header().Get("X-Request-Id")); upstreamRequestID != "" {
				fields = append(fields, zap.String("upstream_request_id", upstreamRequestID))
			}
		}
	}

	log := logger.FromContext(ctx).With(fields...)
	if outcome == "succeeded" {
		log.Info("codex.remote_compact.succeeded")
		return
	}
	log.Warn("codex.remote_compact.failed")
}

// Messages handles Anthropic Messages API requests routed to OpenAI platform.
// POST /v1/messages (when group platform is OpenAI)
func (h *OpenAIGatewayHandler) Messages(c *gin.Context) {
	streamStarted := false
	defer h.recoverAnthropicMessagesPanic(c, &streamStarted)
	defer service.ClearOpenAICompatRequestState(c)

	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.openai_gateway.messages",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	// 检查分组是否允许 /v1/messages 调度
	if apiKey.Group != nil && !apiKey.Group.AllowMessagesDispatch {
		h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error",
			"This group does not allow /v1/messages dispatch")
		return
	}

	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.anthropicErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	if !gjson.ValidBytes(body) {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	routingModel := service.NormalizeOpenAICompatRequestedModel(reqModel)
	preferredMappedModel := resolveOpenAIMessagesDispatchMappedModel(apiKey, reqModel)
	forcedDispatchModel := resolveOpenAIMessagesDispatchForcedModel(apiKey, reqModel)
	service.SetOpenAIMessagesDispatchForcedModel(c, forcedDispatchModel)
	reqStream := gjson.GetBytes(body, "stream").Bool()

	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))

	setOpsRequestContext(c, reqModel, reqStream, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))

	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolAnthropicMessages, reqModel, body); decision != nil && decision.Blocked {
		h.anthropicErrorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return
	}

	// 解析渠道级模型映射
	channelMappingMsg, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)
	mappedBodyForMessages := newOpenAIModelMappedBodyCache(body, h.gatewayService.ReplaceModelInBody)

	// 绑定错误透传服务，允许 service 层在非 failover 错误场景复用规则。
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai_messages.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.anthropicStreamingAwareError(c, status, code, message, streamStarted)
		return
	}

	sessionHash := h.gatewayService.GenerateSessionHash(c, body)
	promptCacheKey := h.gatewayService.ExtractSessionID(c, body)
	sessionHash, promptCacheKey = resolveOpenAIMessagesMetadataSession(sessionHash, promptCacheKey, reqModel, body)

	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError
	effectiveMappedModel := preferredMappedModel

	for {
		currentRoutingModel := routingModel
		if effectiveMappedModel != "" {
			currentRoutingModel = effectiveMappedModel
		}
		reqLog.Debug("openai_messages.account_selecting", zap.Int("excluded_account_count", len(failedAccountIDs)))
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
			c.Request.Context(),
			apiKey.GroupID,
			"", // no previous_response_id
			sessionHash,
			currentRoutingModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportAny,
			service.OpenAIEndpointCapabilityAnthropicMessagesIngress,
			false,
		)
		if err != nil {
			reqLog.Warn("openai_messages.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if len(failedAccountIDs) == 0 {
				if err != nil {
					if h.handleOpenAIGroupModelUnsupportedError(c, err, streamStarted) {
						return
					}
					h.anthropicStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable", streamStarted)
					return
				}
			} else {
				if lastFailoverErr != nil {
					h.handleAnthropicFailoverExhausted(c, lastFailoverErr, streamStarted)
				} else {
					h.anthropicStreamingAwareError(c, http.StatusBadGateway, "api_error", "Upstream request failed", streamStarted)
				}
				return
			}
		}
		if selection == nil || selection.Account == nil {
			h.anthropicStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStarted)
			return
		}
		account := selection.Account
		sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
		reqLog.Debug("openai_messages.account_selected", zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
		_ = scheduleDecision
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc, acquireStatus := h.acquireResponsesAccountSlot(c, apiKey.GroupID, apiKey.ID, sessionHash, "", selection, reqStream, &streamStarted, reqLog)
		if acquireStatus == accountSlotAcquireRetry {
			failedAccountIDs[account.ID] = struct{}{}
			continue
		}
		if acquireStatus != accountSlotAcquireAcquired {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()

		defaultMappedModel := strings.TrimSpace(effectiveMappedModel)
		// 应用渠道模型映射到请求体
		forwardBody := mappedBodyForMessages(channelMappingMsg.Mapped, channelMappingMsg.MappedModel)
		writerSizeBeforeForward := c.Writer.Size()
		result, err := func() (*service.OpenAIForwardResult, error) {
			defer func() {
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
			}()
			return h.gatewayService.ForwardAsAnthropic(c.Request.Context(), c, account, forwardBody, promptCacheKey, defaultMappedModel)
		}()

		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
		if err == nil && result != nil && result.FirstTokenMs != nil {
			service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
		}
		if err != nil {
			if result != nil && result.ImageCount > 0 {
				reqLog.Warn("openai_messages.forward_partial_error_with_image_result",
					zap.Int64("account_id", account.ID),
					zap.Int("image_count", result.ImageCount),
					zap.Error(err),
				)
			} else {
				var failoverErr *service.UpstreamFailoverError
				if errors.As(err, &failoverErr) {
					if c.Writer.Size() != writerSizeBeforeForward {
						h.handleAnthropicFailoverExhausted(c, failoverErr, true)
						return
					}
					h.reportOpenAIAccountScheduleFailure(account.ID, err)
					// 池模式：同账号重试
					if failoverErr.RetryableOnSameAccount {
						retryLimit := account.GetPoolModeRetryCount()
						if sameAccountRetryCount[account.ID] < retryLimit {
							sameAccountRetryCount[account.ID]++
							reqLog.Warn("openai_messages.pool_mode_same_account_retry",
								zap.Int64("account_id", account.ID),
								zap.Int("upstream_status", failoverErr.StatusCode),
								zap.Int("retry_limit", retryLimit),
								zap.Int("retry_count", sameAccountRetryCount[account.ID]),
							)
							select {
							case <-c.Request.Context().Done():
								return
							case <-time.After(sameAccountRetryDelay):
							}
							continue
						}
					}
					h.gatewayService.RecordOpenAIAccountSwitch()
					failedAccountIDs[account.ID] = struct{}{}
					lastFailoverErr = failoverErr
					if switchCount >= maxAccountSwitches {
						h.handleAnthropicFailoverExhausted(c, failoverErr, streamStarted)
						return
					}
					service.ClearOpenAICompatRequestState(c)
					switchCount++
					if h.gatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, switchCount) {
						h.handleAnthropicFailoverExhausted(c, failoverErr, streamStarted)
						return
					}
					reqLog.Warn("openai_messages.upstream_failover_switching",
						zap.Int64("account_id", account.ID),
						zap.Int("upstream_status", failoverErr.StatusCode),
						zap.Int("switch_count", switchCount),
						zap.Int("max_switches", maxAccountSwitches),
					)
					continue
				}
				if result != nil && result.ClientDisconnect {
					reqLog.Info("openai_messages.client_disconnected",
						zap.Int64("account_id", account.ID),
						zap.Error(err),
					)
					return
				}
				h.reportOpenAIAccountScheduleFailure(account.ID, err)
				wroteFallback := h.ensureAnthropicErrorResponse(c, streamStarted)
				reqLog.Warn("openai_messages.forward_failed",
					zap.Int64("account_id", account.ID),
					zap.Bool("fallback_error_response_written", wroteFallback),
					zap.Error(err),
				)
				return
			}
		}
		if result != nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
		} else {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
		}

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(body)
		inboundEndpoint := GetInboundEndpoint(c)
		upstreamEndpoint := resolveOpenAIMessagesUpstreamEndpoint(c, account)
		quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)

		h.submitOpenAIUsageRecordTask(c.Request.Context(), result, func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:             result,
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
				ChannelUsageFields: channelMappingMsg.ToUsageFields(reqModel, result.UpstreamModel),
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.messages"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.Any("group_id", apiKey.GroupID),
					zap.String("model", reqModel),
					zap.Int64("account_id", account.ID),
				).Error("openai_messages.record_usage_failed", zap.Error(err))
			}
		})
		reqLog.Debug("openai_messages.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return
	}
}

func resolveOpenAIMessagesMetadataSession(sessionHash, promptCacheKey, reqModel string, body []byte) (string, string) {
	// Anthropic metadata.user_id 只作为账号粘性信号。上游 GPT/Codex 缓存键
	// 交给 ForwardAsAnthropic 从 cache_control 或完整消息 digest 派生，避免
	// 固定 metadata key 压住后续 turn 的缓存滚动。
	if sessionHash != "" {
		return sessionHash, promptCacheKey
	}
	if userID := strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String()); userID != "" {
		seed := reqModel + "-" + userID
		sessionHash = service.DeriveSessionHashFromSeed(seed)
	}
	return sessionHash, promptCacheKey
}

// anthropicErrorResponse writes an error in Anthropic Messages API format.
func (h *OpenAIGatewayHandler) anthropicErrorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// anthropicStreamingAwareError handles errors that may occur during streaming,
// using Anthropic SSE error format.
func (h *OpenAIGatewayHandler) anthropicStreamingAwareError(c *gin.Context, status int, errType, message string, streamStarted bool) {
	if streamStarted {
		flusher, ok := c.Writer.(http.Flusher)
		if ok {
			errPayload, _ := json.Marshal(gin.H{
				"type": "error",
				"error": gin.H{
					"type":    errType,
					"message": message,
				},
			})
			fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", errPayload) //nolint:errcheck
			flusher.Flush()
		}
		return
	}
	h.anthropicErrorResponse(c, status, errType, message)
}

// handleAnthropicFailoverExhausted maps upstream failover errors to Anthropic format.
func (h *OpenAIGatewayHandler) handleAnthropicFailoverExhausted(c *gin.Context, failoverErr *service.UpstreamFailoverError, streamStarted bool) {
	copyFailoverResponseHeaders(c, failoverErr)
	status, errType, errMsg := h.mapUpstreamError(failoverErr.StatusCode)
	h.anthropicStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

// ensureAnthropicErrorResponse writes a fallback Anthropic error if no response was written.
func (h *OpenAIGatewayHandler) ensureAnthropicErrorResponse(c *gin.Context, streamStarted bool) bool {
	if c == nil || c.Writer == nil || c.Writer.Written() {
		return false
	}
	h.anthropicStreamingAwareError(c, http.StatusBadGateway, "api_error", "Upstream request failed", streamStarted)
	return true
}

func (h *OpenAIGatewayHandler) validateFunctionCallOutputRequest(c *gin.Context, body []byte, reqLog *zap.Logger) bool {
	if !service.HasToolContinuationOutputInRawPayload(body) {
		return true
	}

	validation := service.ValidateFunctionCallOutputContextBytes(body)
	if !validation.HasFunctionCallOutput {
		return true
	}

	if validation.HasToolCallContext {
		return true
	}

	if validation.HasFunctionCallOutputMissingCallID {
		reqLog.Warn("openai.request_validation_failed",
			zap.String("reason", "function_call_output_missing_call_id"),
		)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "function_call_output requires call_id on HTTP requests; continuation via previous_response_id is only supported on Responses WebSocket v2")
		return false
	}
	if validation.HasItemReferenceForAllCallIDs {
		return true
	}

	reqLog.Warn("openai.request_validation_failed",
		zap.String("reason", "function_call_output_missing_item_reference"),
	)
	h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "function_call_output requires item_reference ids matching each call_id on HTTP requests; continuation via previous_response_id is only supported on Responses WebSocket v2")
	return false
}

func newOpenAIModelMappedBodyCache(body []byte, replace func([]byte, string) []byte) func(bool, string) []byte {
	var cachedMapped bool
	var cachedModel string
	var cachedBody []byte
	return func(mapped bool, mappedModel string) []byte {
		if !mapped || replace == nil {
			return body
		}
		if !cachedMapped || cachedModel != mappedModel {
			cachedBody = replace(body, mappedModel)
			cachedModel = mappedModel
			cachedMapped = true
		}
		return cachedBody
	}
}

func (h *OpenAIGatewayHandler) acquireResponsesUserSlot(
	c *gin.Context,
	userID int64,
	userConcurrency int,
	reqStream bool,
	streamStarted *bool,
	reqLog *zap.Logger,
) (func(), bool) {
	ctx := c.Request.Context()
	userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, userID, userConcurrency)
	if err != nil {
		reqLog.Warn("openai.user_slot_acquire_failed", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", *streamStarted)
		return nil, false
	}
	if userAcquired {
		return wrapReleaseOnDone(ctx, userReleaseFunc), true
	}

	maxWait := service.CalculateMaxWait(userConcurrency)
	canWait, waitErr := h.concurrencyHelper.IncrementWaitCount(ctx, userID, maxWait)
	if waitErr != nil {
		reqLog.Warn("openai.user_wait_counter_increment_failed", zap.Error(waitErr))
		// 按现有降级语义：等待计数异常时放行后续抢槽流程
	} else if !canWait {
		reqLog.Info("openai.user_wait_queue_full", zap.Int("max_wait", maxWait))
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later")
		return nil, false
	}

	waitCounted := waitErr == nil && canWait
	defer func() {
		if waitCounted {
			h.concurrencyHelper.DecrementWaitCount(ctx, userID)
		}
	}()

	userReleaseFunc, err = h.concurrencyHelper.AcquireUserSlotWithWait(c, userID, userConcurrency, reqStream, streamStarted)
	if err != nil {
		reqLog.Warn("openai.user_slot_acquire_failed_after_wait", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", *streamStarted)
		return nil, false
	}

	// 槽位获取成功后，立刻退出等待计数。
	if waitCounted {
		h.concurrencyHelper.DecrementWaitCount(ctx, userID)
		waitCounted = false
	}
	return wrapReleaseOnDone(ctx, userReleaseFunc), true
}

func (h *OpenAIGatewayHandler) acquireResponsesAccountSlot(
	c *gin.Context,
	groupID *int64,
	apiKeyID int64,
	sessionHash string,
	previousResponseID string,
	selection *service.AccountSelectionResult,
	reqStream bool,
	streamStarted *bool,
	reqLog *zap.Logger,
) (func(), accountSlotAcquireStatus) {
	if selection == nil || selection.Account == nil {
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", *streamStarted)
		return nil, accountSlotAcquireFailed
	}

	ctx := c.Request.Context()
	clearStickyBindings := func() {
		_ = h.gatewayService.ClearStickySession(ctx, groupID, sessionHash)
		_ = h.gatewayService.ClearPreviousResponseBinding(ctx, groupID, apiKeyID, previousResponseID)
	}
	account := selection.Account
	if selection.Acquired {
		return wrapReleaseOnDone(ctx, selection.ReleaseFunc), accountSlotAcquireAcquired
	}
	if selection.WaitPlan == nil {
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", *streamStarted)
		return nil, accountSlotAcquireFailed
	}

	remainingTimeout := selection.WaitPlan.Timeout
	if selection.WaitPlan.NotBefore != nil {
		if remainingTimeout <= 0 || time.Until(*selection.WaitPlan.NotBefore) > remainingTimeout {
			reqLog.Info("openai.account_wait_deadline_exceeded_before_ready",
				zap.Int64("account_id", account.ID),
				zap.Duration("wait_timeout", selection.WaitPlan.Timeout),
				zap.Timep("not_before", selection.WaitPlan.NotBefore),
			)
			clearStickyBindings()
			return nil, accountSlotAcquireRetry
		}
		waitStart := time.Now()
		if err := h.concurrencyHelper.WaitUntil(c, *selection.WaitPlan.NotBefore, reqStream, streamStarted); err != nil {
			reqLog.Warn("openai.account_wait_until_ready_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			h.handleConcurrencyError(c, err, "account", *streamStarted)
			return nil, accountSlotAcquireFailed
		}
		remainingTimeout -= time.Since(waitStart)
		if remainingTimeout < 0 {
			remainingTimeout = 0
		}
	}

	fastReleaseFunc, fastAcquired, err := h.concurrencyHelper.TryAcquireAccountSlotForGroup(
		ctx,
		account.ID,
		groupID,
		selection.WaitPlan.MaxConcurrency,
	)
	if err != nil {
		reqLog.Warn("openai.account_slot_quick_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		h.handleConcurrencyError(c, err, "account", *streamStarted)
		return nil, accountSlotAcquireFailed
	}
	if fastAcquired {
		if err := h.gatewayService.BindStickySession(ctx, groupID, sessionHash, account.ID); err != nil {
			reqLog.Warn("openai.bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}
		return wrapReleaseOnDone(ctx, fastReleaseFunc), accountSlotAcquireAcquired
	}

	canWait, waitErr := h.concurrencyHelper.IncrementAccountWaitCount(ctx, account.ID, selection.WaitPlan.MaxWaiting)
	if waitErr != nil {
		reqLog.Warn("openai.account_wait_counter_increment_failed", zap.Int64("account_id", account.ID), zap.Error(waitErr))
	} else if !canWait {
		reqLog.Info("openai.account_wait_queue_full",
			zap.Int64("account_id", account.ID),
			zap.Int("max_waiting", selection.WaitPlan.MaxWaiting),
		)
		h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later", *streamStarted)
		return nil, accountSlotAcquireFailed
	}

	accountWaitCounted := waitErr == nil && canWait
	releaseWait := func() {
		if accountWaitCounted {
			h.concurrencyHelper.DecrementAccountWaitCount(ctx, account.ID)
			accountWaitCounted = false
		}
	}
	defer releaseWait()

	accountReleaseFunc, err := h.concurrencyHelper.AcquireAccountSlotWithWaitTimeoutForGroup(
		c,
		account.ID,
		groupID,
		selection.WaitPlan.MaxConcurrency,
		remainingTimeout,
		reqStream,
		streamStarted,
	)
	if err != nil {
		var concurrencyErr *ConcurrencyError
		if errors.As(err, &concurrencyErr) && concurrencyErr.IsTimeout {
			reqLog.Info("openai.account_slot_wait_timed_out",
				zap.Int64("account_id", account.ID),
				zap.Duration("wait_timeout", selection.WaitPlan.Timeout),
			)
			clearStickyBindings()
			return nil, accountSlotAcquireRetry
		}
		reqLog.Warn("openai.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		h.handleConcurrencyError(c, err, "account", *streamStarted)
		return nil, accountSlotAcquireFailed
	}

	// Slot acquired: no longer waiting in queue.
	releaseWait()
	if err := h.gatewayService.BindStickySession(ctx, groupID, sessionHash, account.ID); err != nil {
		reqLog.Warn("openai.bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
	}
	return wrapReleaseOnDone(ctx, accountReleaseFunc), accountSlotAcquireAcquired
}

// ResponsesWebSocket handles OpenAI Responses API WebSocket ingress endpoint
// GET /openai/v1/responses (Upgrade: websocket)
func (h *OpenAIGatewayHandler) ResponsesWebSocket(c *gin.Context) {
	if !isOpenAIWSUpgradeRequest(c.Request) {
		h.errorResponse(c, http.StatusUpgradeRequired, "invalid_request_error", "WebSocket upgrade required (Upgrade: websocket)")
		return
	}
	setOpenAIClientTransportWS(c)

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
		"handler.openai_gateway.responses_ws",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.Bool("openai_ws_mode", true),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}
	reqLog.Info("openai.websocket_ingress_started")
	clientIP := ip.GetClientIP(c)
	userAgent := strings.TrimSpace(c.GetHeader("User-Agent"))

	wsConn, err := coderws.Accept(c.Writer, c.Request, &coderws.AcceptOptions{
		CompressionMode: coderws.CompressionContextTakeover,
	})
	if err != nil {
		reqLog.Warn("openai.websocket_accept_failed",
			zap.Error(err),
			zap.String("client_ip", clientIP),
			zap.String("request_user_agent", userAgent),
			zap.String("upgrade_header", strings.TrimSpace(c.GetHeader("Upgrade"))),
			zap.String("connection_header", strings.TrimSpace(c.GetHeader("Connection"))),
			zap.String("sec_websocket_version", strings.TrimSpace(c.GetHeader("Sec-WebSocket-Version"))),
			zap.Bool("has_sec_websocket_key", strings.TrimSpace(c.GetHeader("Sec-WebSocket-Key")) != ""),
		)
		return
	}
	defer func() {
		_ = wsConn.CloseNow()
	}()
	wsConn.SetReadLimit(service.ResolveOpenAIWSClientReadLimitBytes(h.cfg))

	ctx := c.Request.Context()
	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	msgType, firstMessage, err := wsConn.Read(readCtx)
	cancel()
	if err != nil {
		closeStatus, closeReason := summarizeWSCloseErrorForLog(err)
		reqLog.Warn("openai.websocket_read_first_message_failed",
			zap.Error(err),
			zap.String("client_ip", clientIP),
			zap.String("close_status", closeStatus),
			zap.String("close_reason", closeReason),
			zap.Duration("read_timeout", 30*time.Second),
		)
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "missing first response.create message")
		return
	}
	if msgType != coderws.MessageText && msgType != coderws.MessageBinary {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "unsupported websocket message type")
		return
	}
	if !gjson.ValidBytes(firstMessage) {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "invalid JSON payload")
		return
	}

	reqModel := strings.TrimSpace(gjson.GetBytes(firstMessage, "model").String())
	if reqModel == "" {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "model is required in first response.create payload")
		return
	}
	previousResponseID := strings.TrimSpace(gjson.GetBytes(firstMessage, "previous_response_id").String())
	previousResponseIDKind := service.ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
	if previousResponseID != "" && previousResponseIDKind == service.OpenAIPreviousResponseIDKindMessageID {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "previous_response_id must be a response.id (resp_*), not a message id")
		return
	}
	reqLog = reqLog.With(
		zap.Bool("ws_ingress", true),
		zap.String("model", reqModel),
		zap.Bool("has_previous_response_id", previousResponseID != ""),
		zap.String("previous_response_id_kind", previousResponseIDKind),
	)
	setOpsRequestContext(c, reqModel, true, firstMessage)
	setOpsEndpointContext(c, "", int16(service.RequestTypeWSV2))

	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, reqModel, firstMessage); decision != nil && decision.Blocked {
		writeContentModerationWSError(ctx, wsConn, decision)
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, decision.Message)
		return
	}

	allowImageGeneration := service.GroupAllowsImageGeneration(apiKey.Group)
	rawWSImageIntent, _ := classifyOpenAIResponsesImageRequest(allowImageGeneration, reqModel, firstMessage)
	if rawWSImageIntent && !allowImageGeneration {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, service.ImageGenerationPermissionMessage())
		return
	}

	// 解析渠道级模型映射
	channelMappingWS, _ := h.gatewayService.ResolveChannelMappingAndRestrict(ctx, apiKey.GroupID, reqModel)
	wsFirstMessage, wsFirstModel := h.applyOpenAIResponsesChannelMapping(firstMessage, reqModel, channelMappingWS)
	wsPreviewImageIntent, _ := classifyOpenAIResponsesImageRequest(allowImageGeneration, wsFirstModel, wsFirstMessage)
	if wsPreviewImageIntent && !allowImageGeneration {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, service.ImageGenerationPermissionMessage())
		return
	}

	var currentUserRelease func()
	var currentAccountRelease func()
	var currentImageRelease func()
	currentTurnNeedsImageSlot := false
	releaseAccountScopedSlots := func() {
		if currentImageRelease != nil {
			currentImageRelease()
			currentImageRelease = nil
		}
		if currentAccountRelease != nil {
			currentAccountRelease()
			currentAccountRelease = nil
		}
	}
	releaseTurnSlots := func() {
		releaseAccountScopedSlots()
		if currentUserRelease != nil {
			currentUserRelease()
			currentUserRelease = nil
		}
	}
	// 必须尽早注册，确保任何 early return 都能释放已获取的并发槽位。
	defer releaseTurnSlots()

	userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
	if err != nil {
		reqLog.Warn("openai.websocket_user_slot_acquire_failed", zap.Error(err))
		closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire user concurrency slot")
		return
	}
	if !userAcquired {
		closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "too many concurrent requests, please retry later")
		return
	}
	currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
	ensureUserSlotHeld := func() bool {
		if currentUserRelease != nil {
			return true
		}
		userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
		if err != nil {
			reqLog.Warn("openai.websocket_user_slot_reacquire_failed", zap.Error(err))
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire user concurrency slot")
			return false
		}
		if !userAcquired {
			closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "too many concurrent requests, please retry later")
			return false
		}
		currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
		return true
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(ctx, apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai.websocket_billing_eligibility_check_failed", zap.Error(err))
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "billing check failed")
		return
	}

	sessionHash := h.gatewayService.GenerateSessionHashWithFallback(
		c,
		firstMessage,
		openAIWSIngressFallbackSessionSeed(subject.UserID, apiKey.ID, apiKey.GroupID),
	)
	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	var lastFailoverErr *service.UpstreamFailoverError

	for {
		reqLog.Debug("openai.websocket_account_selecting", zap.Int("excluded_account_count", len(failedAccountIDs)))
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForResponses(
			ctx,
			apiKey.GroupID,
			apiKey.ID,
			previousResponseID,
			sessionHash,
			reqModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportResponsesWebsocketV2,
			wsPreviewImageIntent,
			false,
		)
		if err != nil {
			reqLog.Warn("openai.websocket_account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if lastFailoverErr != nil {
				closeOpenAIWSFailoverExhausted(wsConn, lastFailoverErr)
			} else {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "no available account")
			}
			return
		}
		if selection == nil || selection.Account == nil {
			if lastFailoverErr != nil {
				closeOpenAIWSFailoverExhausted(wsConn, lastFailoverErr)
			} else {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "no available account")
			}
			return
		}

		account := selection.Account
		effectiveWSFirstModel := resolveOpenAIResponsesEffectiveModel(account, wsFirstModel)
		firstTurnImageIntent, firstTurnNeedsImageSlot := classifyOpenAIResponsesImageRequest(allowImageGeneration, effectiveWSFirstModel, wsFirstMessage)
		if firstTurnImageIntent && !allowImageGeneration {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, service.ImageGenerationPermissionMessage())
			return
		}
		if !openAIResponsesAccountSupportsImageIntent(account, apiKey.Group, firstTurnImageIntent) {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			_ = h.gatewayService.ClearStickySession(ctx, apiKey.GroupID, sessionHash)
			_ = h.gatewayService.ClearPreviousResponseBinding(ctx, apiKey.GroupID, apiKey.ID, previousResponseID)
			failedAccountIDs[account.ID] = struct{}{}
			continue
		}

		accountMaxConcurrency := account.Concurrency
		if selection.WaitPlan != nil && selection.WaitPlan.MaxConcurrency > 0 {
			accountMaxConcurrency = selection.WaitPlan.MaxConcurrency
		}
		accountReleaseFunc := selection.ReleaseFunc
		if !selection.Acquired {
			if selection.WaitPlan == nil {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "account is busy, please retry later")
				return
			}
			fastReleaseFunc, fastAcquired, err := h.concurrencyHelper.TryAcquireAccountSlotForGroup(
				ctx,
				account.ID,
				apiKey.GroupID,
				selection.WaitPlan.MaxConcurrency,
			)
			if err != nil {
				reqLog.Warn("openai.websocket_account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire account concurrency slot")
				return
			}
			if !fastAcquired {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "account is busy, please retry later")
				return
			}
			accountReleaseFunc = fastReleaseFunc
		}
		currentAccountRelease = wrapReleaseOnDone(ctx, accountReleaseFunc)
		if err := h.gatewayService.BindStickySession(ctx, apiKey.GroupID, sessionHash, account.ID); err != nil {
			reqLog.Warn("openai.websocket_bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}

		token, _, err := h.gatewayService.GetAccessToken(ctx, account)
		if err != nil {
			reqLog.Warn("openai.websocket_get_access_token_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to get access token")
			return
		}

		reqLog.Debug("openai.websocket_account_selected",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.String("schedule_layer", scheduleDecision.Layer),
			zap.Int("candidate_count", scheduleDecision.CandidateCount),
		)

		currentTurnNeedsImageSlot = firstTurnNeedsImageSlot

		hooks := &service.OpenAIWSIngressHooks{
			InitialRequestModel: reqModel,
			SessionHash:         sessionHash,
			BeforeRequest: func(turn int, payload []byte, originalModel string) error {
				if turn == 1 {
					return nil
				}
				if !gjson.ValidBytes(payload) {
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", errors.New("invalid json"))
				}
				model := strings.TrimSpace(originalModel)
				if model == "" {
					model = strings.TrimSpace(gjson.GetBytes(payload, "model").String())
				}
				if model == "" {
					model = reqModel
				}
				if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, model, payload); decision != nil && decision.Blocked {
					writeContentModerationWSError(ctx, wsConn, decision)
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, decision.Message, nil)
				}
				refreshedAccount := h.gatewayService.RefreshOpenAIResponsesTurnAccount(ctx, account, model)
				if refreshedAccount == nil {
					_ = h.gatewayService.ClearStickySession(ctx, apiKey.GroupID, sessionHash)
					_ = h.gatewayService.ClearPreviousResponseBinding(ctx, apiKey.GroupID, apiKey.ID, previousResponseID)
					return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "no available account", errOpenAIWSTurnAccountUnavailable)
				}
				account = refreshedAccount
				effectiveModel := resolveOpenAIResponsesEffectiveModel(account, model)
				imageIntent, needsImageSlot := classifyOpenAIResponsesImageRequest(allowImageGeneration, effectiveModel, payload)
				if imageIntent && !allowImageGeneration {
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, service.ImageGenerationPermissionMessage(), nil)
				}
				if needsImageSlot && !account.OpenAIImageGenerationAllowed() {
					return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "no available account", errOpenAIWSLocalImageToggleUnavailable)
				}
				if !openAIResponsesAccountSupportsImageIntent(account, apiKey.Group, imageIntent || needsImageSlot) {
					_ = h.gatewayService.ClearStickySession(ctx, apiKey.GroupID, sessionHash)
					_ = h.gatewayService.ClearPreviousResponseBinding(ctx, apiKey.GroupID, apiKey.ID, previousResponseID)
					return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "no available account", errOpenAIWSTurnAccountUnavailable)
				}
				currentTurnNeedsImageSlot = needsImageSlot
				return nil
			},
			BeforeTurn: func(turn int) error {
				if turn > 1 {
					// 防御式清理：避免异常路径下旧槽位覆盖导致泄漏。
					releaseTurnSlots()
					// 非首轮 turn 需要重新抢占并发槽位，避免长连接空闲占槽。
					userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
					if err != nil {
						return service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "failed to acquire user concurrency slot", err)
					}
					if !userAcquired {
						return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "too many concurrent requests, please retry later", nil)
					}
					turnAccountMaxConcurrency := accountMaxConcurrency
					if account != nil {
						turnAccountMaxConcurrency = account.Concurrency
						if turnAccountMaxConcurrency <= 0 {
							turnAccountMaxConcurrency = 1
						}
					}
					accountReleaseFunc, accountAcquired, err := h.concurrencyHelper.TryAcquireAccountSlotForGroup(ctx, account.ID, apiKey.GroupID, turnAccountMaxConcurrency)
					if err != nil {
						if userReleaseFunc != nil {
							userReleaseFunc()
						}
						return service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "failed to acquire account concurrency slot", err)
					}
					if !accountAcquired {
						if userReleaseFunc != nil {
							userReleaseFunc()
						}
						return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "account is busy, please retry later", nil)
					}
					currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
					currentAccountRelease = wrapReleaseOnDone(ctx, accountReleaseFunc)
				}
				if currentTurnNeedsImageSlot {
					imageReleaseFunc, err := h.acquireImageGenerationWSSlot(ctx)
					if err != nil {
						releaseTurnSlots()
						return err
					}
					currentImageRelease = imageReleaseFunc
				} else {
					currentImageRelease = nil
				}
				return nil
			},
			AfterTurn: func(turn int, turnPayload []byte, result *service.OpenAIForwardResult, turnErr error) {
				releaseTurnSlots()
				if turnErr != nil {
					if result == nil || result.ImageCount <= 0 {
						return
					}
					reqLog.Warn("openai.websocket_partial_error_with_image_result",
						zap.Int64("account_id", account.ID),
						zap.Int("image_count", result.ImageCount),
						zap.Error(turnErr),
					)
				}
				if result == nil {
					return
				}
				if account.Type == service.AccountTypeOAuth {
					h.gatewayService.UpdateCodexUsageSnapshotFromHeaders(ctx, account.ID, result.ResponseHeaders)
				}
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
				inboundEndpoint := GetInboundEndpoint(c)
				upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
				quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
				requestPayloadHash := service.HashUsageRequestPayload(turnPayload)
				h.submitOpenAIUsageRecordTask(c.Request.Context(), result, func(taskCtx context.Context) {
					if err := h.gatewayService.RecordUsage(taskCtx, &service.OpenAIRecordUsageInput{
						Result:             result,
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
						ChannelUsageFields: channelMappingWS.ToUsageFields(reqModel, result.UpstreamModel),
					}); err != nil {
						reqLog.Error("openai.websocket_record_usage_failed",
							zap.Int64("account_id", account.ID),
							zap.String("request_id", result.RequestID),
							zap.Error(err),
						)
					}
				})
			},
		}

		if err := h.gatewayService.ProxyResponsesWebSocketFromClient(ctx, c, wsConn, account, token, wsFirstMessage, hooks); err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				h.reportOpenAIAccountScheduleFailure(account.ID, err)
				releaseAccountScopedSlots()
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					closeOpenAIWSFailoverExhausted(wsConn, failoverErr)
					return
				}
				switchCount++
				if h.gatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, switchCount) {
					closeOpenAIWSFailoverExhausted(wsConn, failoverErr)
					return
				}
				h.gatewayService.RecordOpenAIAccountSwitch()
				reqLog.Warn("openai.websocket_upstream_failover_switching",
					zap.Int64("account_id", account.ID),
					zap.Int("upstream_status", failoverErr.StatusCode),
					zap.Int("switch_count", switchCount),
					zap.Int("max_switches", maxAccountSwitches),
				)
				if !ensureUserSlotHeld() {
					return
				}
				continue
			}

			var closeErr *service.OpenAIWSClientCloseError
			if errors.As(err, &closeErr) {
				if !shouldReportOpenAIWebSocketAccountFailure(err) {
					closeStatus, closeReason := summarizeWSCloseErrorForLog(err)
					reqLog.Warn("openai.websocket_proxy_closed_without_scheduler_failure",
						zap.Int64("account_id", account.ID),
						zap.Error(err),
						zap.String("close_status", closeStatus),
						zap.String("close_reason", closeReason),
					)
					closeOpenAIClientWS(wsConn, closeErr.StatusCode(), closeErr.Reason())
					return
				}
			}

			h.reportOpenAIAccountScheduleFailure(account.ID, err)
			closeStatus, closeReason := summarizeWSCloseErrorForLog(err)
			reqLog.Warn("openai.websocket_proxy_failed",
				zap.Int64("account_id", account.ID),
				zap.Error(err),
				zap.String("close_status", closeStatus),
				zap.String("close_reason", closeReason),
			)
			if errors.As(err, &closeErr) {
				closeOpenAIClientWS(wsConn, closeErr.StatusCode(), closeErr.Reason())
				return
			}
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "upstream websocket proxy failed")
			return
		}
		reqLog.Info("openai.websocket_ingress_closed", zap.Int64("account_id", account.ID))
		return
	}
}

func (h *OpenAIGatewayHandler) recoverResponsesPanic(c *gin.Context, streamStarted *bool) {
	recovered := recover()
	if recovered == nil {
		return
	}

	started := false
	if streamStarted != nil {
		started = *streamStarted
	}
	wroteFallback := h.ensureForwardErrorResponse(c, started, nil)
	requestLogger(c, "handler.openai_gateway.responses").Error(
		"openai.responses_panic_recovered",
		zap.Bool("fallback_error_response_written", wroteFallback),
		zap.Any("panic", recovered),
		zap.ByteString("stack", debug.Stack()),
	)
}

// recoverAnthropicMessagesPanic recovers from panics in the Anthropic Messages
// handler and returns an Anthropic-formatted error response.
func (h *OpenAIGatewayHandler) recoverAnthropicMessagesPanic(c *gin.Context, streamStarted *bool) {
	recovered := recover()
	if recovered == nil {
		return
	}

	started := streamStarted != nil && *streamStarted
	requestLogger(c, "handler.openai_gateway.messages").Error(
		"openai.messages_panic_recovered",
		zap.Bool("stream_started", started),
		zap.Any("panic", recovered),
		zap.ByteString("stack", debug.Stack()),
	)
	if !started {
		h.anthropicErrorResponse(c, http.StatusInternalServerError, "api_error", "Internal server error")
	}
}

func (h *OpenAIGatewayHandler) ensureResponsesDependencies(c *gin.Context, reqLog *zap.Logger) bool {
	missing := h.missingResponsesDependencies()
	if len(missing) == 0 {
		return true
	}

	if reqLog == nil {
		reqLog = requestLogger(c, "handler.openai_gateway.responses")
	}
	reqLog.Error("openai.handler_dependencies_missing", zap.Strings("missing_dependencies", missing))

	if c != nil && c.Writer != nil && !c.Writer.Written() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"type":    "api_error",
				"message": "Service temporarily unavailable",
			},
		})
	}
	return false
}

func (h *OpenAIGatewayHandler) missingResponsesDependencies() []string {
	missing := make([]string, 0, 5)
	if h == nil {
		return append(missing, "handler")
	}
	if h.gatewayService == nil {
		missing = append(missing, "gatewayService")
	}
	if h.billingCacheService == nil {
		missing = append(missing, "billingCacheService")
	}
	if h.apiKeyService == nil {
		missing = append(missing, "apiKeyService")
	}
	if h.concurrencyHelper == nil || h.concurrencyHelper.concurrencyService == nil {
		missing = append(missing, "concurrencyHelper")
	}
	return missing
}

func shouldReportOpenAIWebSocketAccountFailure(err error) bool {
	return !errors.Is(err, errOpenAIWSLocalImageToggleUnavailable) &&
		!errors.Is(err, errOpenAIWSTurnAccountUnavailable)
}

func parseOpenAIStreamField(body []byte) (bool, bool) {
	streamResult := gjson.GetBytes(body, "stream")
	if streamResult.Exists() && streamResult.Type != gjson.True && streamResult.Type != gjson.False {
		return false, false
	}
	return streamResult.Bool(), true
}

func getContextInt64(c *gin.Context, key string) (int64, bool) {
	if c == nil || key == "" {
		return 0, false
	}
	v, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}

func (h *OpenAIGatewayHandler) submitUsageRecordTask(parent context.Context, task service.UsageRecordTask) {
	if task == nil {
		return
	}
	task = wrapUsageRecordTaskContext(parent, task)
	if h.usageRecordWorkerPool != nil {
		h.usageRecordWorkerPool.Submit(task)
		return
	}
	// 回退路径：worker 池未注入时同步执行，避免退回到无界 goroutine 模式。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.responses"),
				zap.Any("panic", recovered),
			).Error("openai.usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}

func (h *OpenAIGatewayHandler) submitOpenAIUsageRecordTask(parent context.Context, result *service.OpenAIForwardResult, task service.UsageRecordTask) {
	h.submitMandatoryUsageRecordTask(parent, task)
}

func (h *OpenAIGatewayHandler) submitMandatoryUsageRecordTask(parent context.Context, task service.UsageRecordTask) {
	if task == nil {
		return
	}
	task = wrapUsageRecordTaskContext(parent, task)
	if h.usageRecordWorkerPool != nil {
		if mode := h.usageRecordWorkerPool.Submit(task); mode != service.UsageRecordSubmitModeDropped {
			return
		}
		logger.L().With(
			zap.String("component", "handler.openai_gateway.usage"),
		).Warn("openai.usage_record_task_mandatory_sync_fallback")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.usage"),
				zap.Any("panic", recovered),
			).Error("openai.usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}

func (h *OpenAIGatewayHandler) acquireImageGenerationSlot(c *gin.Context, streamStarted bool) (func(), bool) {
	release, acquired := h.tryAcquireImageGenerationSlot(c.Request.Context())
	if acquired {
		return release, true
	}
	h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Image generation concurrency limit exceeded, please retry later", streamStarted)
	return nil, false
}

func (h *OpenAIGatewayHandler) tryAcquireImageGenerationSlot(ctx context.Context) (func(), bool) {
	if h == nil || h.cfg == nil || h.imageLimiter == nil {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	imageConcurrency := h.cfg.Gateway.ImageConcurrency
	wait := strings.TrimSpace(imageConcurrency.OverflowMode) == config.ImageConcurrencyOverflowModeWait
	return h.imageLimiter.Acquire(
		ctx,
		imageConcurrency.Enabled,
		imageConcurrency.MaxConcurrentRequests,
		wait,
		time.Duration(imageConcurrency.WaitTimeoutSeconds)*time.Second,
		imageConcurrency.MaxWaitingRequests,
	)
}

func (h *OpenAIGatewayHandler) acquireImageGenerationWSSlot(ctx context.Context) (func(), error) {
	release, acquired := h.tryAcquireImageGenerationSlot(ctx)
	if acquired {
		return release, nil
	}
	return nil, service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "Image generation concurrency limit exceeded, please retry later", nil)
}

func (h *OpenAIGatewayHandler) applyOpenAIResponsesChannelMapping(body []byte, reqModel string, channelMapping service.ChannelMappingResult) ([]byte, string) {
	routedModel := strings.TrimSpace(reqModel)
	if !channelMapping.Mapped {
		return body, routedModel
	}
	routedModel = strings.TrimSpace(channelMapping.MappedModel)
	if h == nil || h.gatewayService == nil {
		return body, routedModel
	}
	return h.gatewayService.ReplaceModelInBody(body, routedModel), routedModel
}

func resolveOpenAIResponsesEffectiveModel(account *service.Account, requestModel string) string {
	model := strings.TrimSpace(requestModel)
	if account == nil {
		return model
	}
	if mappedModel := strings.TrimSpace(account.GetMappedModel(model)); mappedModel != "" {
		return mappedModel
	}
	return model
}

func resolveOpenAIResponsesRequiredImageRoute(group *service.Group, imageIntent bool) string {
	if !imageIntent {
		return ""
	}
	if group == nil {
		return service.GroupImageGenerationRouteCodex
	}
	return group.EffectiveImageGenerationRoute()
}

func openAIResponsesAccountSupportsImageIntent(account *service.Account, group *service.Group, imageIntent bool) bool {
	if imageIntent && (account == nil || !account.OpenAIImageGenerationAllowed()) {
		return false
	}
	requiredRoute := resolveOpenAIResponsesRequiredImageRoute(group, imageIntent)
	if requiredRoute == "" {
		return true
	}
	if account == nil || !account.SupportsOpenAIImageRoute(requiredRoute) {
		return false
	}
	if service.NormalizeGroupImageGenerationRoute(requiredRoute) == service.GroupImageGenerationRouteWeb2API && !account.HasOpenAIImageWeb2APIProfile() {
		return false
	}
	return true
}

func classifyOpenAIResponsesImageRequest(allowImageGeneration bool, requestModel string, body []byte) (bool, bool) {
	imageIntent := service.IsImageGenerationIntent("/v1/responses", requestModel, body)
	return imageIntent, shouldAcquireOpenAIResponsesImageSlot(allowImageGeneration, imageIntent, body)
}

func shouldAcquireOpenAIResponsesImageSlot(allowImageGeneration bool, imageIntent bool, body []byte) bool {
	if imageIntent {
		return true
	}
	return allowImageGeneration && service.HasOpenAIImageGenerationToolCapability(body)
}

// handleConcurrencyError handles concurrency-related acquire errors.
func (h *OpenAIGatewayHandler) handleConcurrencyError(c *gin.Context, err error, slotType string, streamStarted bool) {
	status, errType, message := concurrencyErrorResponse(err, slotType)
	h.handleStreamingAwareError(c, status, errType, message, streamStarted)
}

func (h *OpenAIGatewayHandler) handleFailoverExhausted(c *gin.Context, failoverErr *service.UpstreamFailoverError, account *service.Account, streamStarted bool) {
	statusCode := failoverErr.StatusCode
	responseBody := failoverErr.ResponseBody
	copyFailoverResponseHeaders(c, failoverErr)

	if isOpenAIRetryableOverloadFailover(failoverErr) {
		upstreamMsg := strings.TrimSpace(service.ExtractUpstreamErrorMessage(responseBody))
		if upstreamMsg == "" {
			upstreamMsg = "Upstream service overloaded, please retry later"
		}
		service.SetOpsUpstreamErrorWithType(c, "overloaded_error", statusCode, upstreamMsg, "")
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "overloaded_error", "Upstream service overloaded, please retry later", streamStarted)
		return
	}

	upstreamMsg := service.ExtractUpstreamErrorMessage(responseBody)
	service.SetOpsUpstreamError(c, statusCode, upstreamMsg, "")
	status, errType, errMsg := h.mapUpstreamError(statusCode)

	if rewritten, matched := h.rewriteOpenAIUpstreamErrorForAccount(account, statusCode, responseBody); matched {
		h.handleStreamingAwareError(c, status, errType, rewritten, streamStarted)
		return
	}

	// 先检查透传规则
	if h.errorPassthroughService != nil && len(responseBody) > 0 {
		if rule := h.errorPassthroughService.MatchRule("openai", statusCode, responseBody); rule != nil {
			// 确定响应状态码
			respCode := statusCode
			if !rule.PassthroughCode && rule.ResponseCode != nil {
				respCode = *rule.ResponseCode
			}

			// 确定响应消息
			msg := service.ExtractUpstreamErrorMessage(responseBody)
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

	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

func isOpenAIRetryableOverloadFailover(failoverErr *service.UpstreamFailoverError) bool {
	if failoverErr == nil || failoverErr.StatusCode != http.StatusServiceUnavailable || !failoverErr.RetryableOnSameAccount {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(service.ExtractUpstreamErrorMessage(failoverErr.ResponseBody)))
	if message == "" {
		return true
	}
	return strings.Contains(message, "overloaded") || strings.Contains(message, "capacity")
}

// handleFailoverExhaustedSimple 简化版本，用于没有响应体的情况
func (h *OpenAIGatewayHandler) handleFailoverExhaustedSimple(c *gin.Context, statusCode int, streamStarted bool) {
	status, errType, errMsg := h.mapUpstreamError(statusCode)
	service.SetOpsUpstreamError(c, statusCode, errMsg, "")
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

func (h *OpenAIGatewayHandler) mapUpstreamError(statusCode int) (int, string, string) {
	switch statusCode {
	case 401:
		return http.StatusBadGateway, "upstream_error", "Upstream authentication failed, please contact administrator"
	case 403:
		return http.StatusBadGateway, "upstream_error", "Upstream access forbidden, please contact administrator"
	case 429:
		return http.StatusTooManyRequests, "rate_limit_error", "Upstream rate limit exceeded, please retry later"
	case 529:
		return http.StatusServiceUnavailable, "upstream_error", "Upstream service overloaded, please retry later"
	case 500, 502, 503, 504:
		return http.StatusBadGateway, "upstream_error", "Upstream service temporarily unavailable"
	default:
		return http.StatusBadGateway, "upstream_error", "Upstream request failed"
	}
}

// handleStreamingAwareError handles errors that may occur after streaming has started
func (h *OpenAIGatewayHandler) handleStreamingAwareError(c *gin.Context, status int, errType, message string, streamStarted bool) {
	if openAIStreamingResponseStarted(c, streamStarted) {
		// /v1/responses 的严格 SDK（Codex CLI）要求终止事件必须属于
		// response.completed/failed/incomplete/cancelled 集合。
		// 通用 `event: error` 帧不被识别为终止事件，会导致
		// "stream closed before response.completed"。
		if inboundIsResponses(c) {
			if writeResponsesFailedSSE(c, errType, message) {
				return
			}
		}
		// Stream already started, send error as SSE event then close
		flusher, ok := c.Writer.(http.Flusher)
		if ok {
			// SSE 错误事件固定 schema，使用 Quote 直拼可避免额外 Marshal 分配。
			errorEvent := "event: error\ndata: " + `{"error":{"type":` + strconv.Quote(errType) + `,"message":` + strconv.Quote(message) + `}}` + "\n\n"
			if _, err := fmt.Fprint(c.Writer, errorEvent); err != nil {
				_ = c.Error(err)
			}
			// 给 chat completions / 通用 SSE 客户端补一个 [DONE] 终止符。
			// 不少 SDK（含 openai-python、openai-js）依赖 data: [DONE] 关闭流，
			// 仅有 event: error 会让客户端等到读取超时才放弃。
			// /v1/responses 严格 SDK（Codex CLI / openai-python responses 客户端）
			// 把 [DONE] 当作非法终止帧，因此 Responses 路径不附加 [DONE]。
			// 正常情况下 Responses 路径已在上面 writeResponsesFailedSSE 成功后 return，
			// 这里仅作为 flusher 不可用等兜底分支的防御。
			if !inboundIsResponses(c) {
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

func clearOpenAIStreamRetryReplayState(c *gin.Context) {
	if c == nil || c.Keys == nil {
		return
	}
	delete(c.Keys, openAIStreamRetryReplayStateContextKey)
}

func openAIStreamingResponseStarted(c *gin.Context, streamStarted bool) bool {
	if streamStarted {
		return true
	}
	if c == nil || c.Writer == nil || !c.Writer.Written() {
		return false
	}
	contentType := strings.ToLower(strings.TrimSpace(c.Writer.Header().Get("Content-Type")))
	return strings.Contains(contentType, "text/event-stream")
}

func openAIRequestWantsStream(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if v, ok := c.Get(opsStreamKey); ok {
		if stream, ok := v.(bool); ok {
			return stream
		}
	}
	return false
}

// ensureForwardErrorResponse 在 Forward 返回错误但尚未写响应时补写统一错误响应。
func (h *OpenAIGatewayHandler) ensureForwardErrorResponse(c *gin.Context, streamStarted bool, forwardErr error) bool {
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
	if service.IsOpenAIWSSessionPreemptedError(forwardErr) {
		if inboundIsResponses(c) && (streamStarted || openAIRequestWantsStream(c)) {
			return writeResponsesCancelledSSE(c)
		}
		if streamStarted {
			h.handleStreamingAwareError(c, 499, "request_canceled", "Superseded by a newer request in the same session", true)
			return true
		}
		h.errorResponse(c, 499, "request_canceled", "Superseded by a newer request in the same session")
		return true
	}
	detail := resolveUpstreamForwardErrorDetail(c, forwardErr)
	service.SetOpsUpstreamErrorWithType(c, detail.ErrorType, 0, detail.Message, detail.Detail)
	h.handleStreamingAwareError(c, http.StatusBadGateway, detail.ErrorType, detail.Message, streamStarted)
	return true
}

func shouldLogOpenAIForwardFailureAsWarn(c *gin.Context, wroteFallback bool) bool {
	if wroteFallback {
		return false
	}
	if c == nil || c.Writer == nil {
		return false
	}
	return c.Writer.Written()
}

// errorResponse returns OpenAI API format error response
func (h *OpenAIGatewayHandler) errorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

func setOpenAIClientTransportHTTP(c *gin.Context) {
	service.SetOpenAIClientTransport(c, service.OpenAIClientTransportHTTP)
}

func setOpenAIClientTransportWS(c *gin.Context) {
	service.SetOpenAIClientTransport(c, service.OpenAIClientTransportWS)
}

func ensureOpenAIPoolModeSessionHash(sessionHash string, account *service.Account) string {
	if sessionHash != "" || account == nil || !account.IsPoolMode() {
		return sessionHash
	}
	// 为当前请求生成一次性粘性会话键，确保同账号重试不会重新负载均衡到其他账号。
	return "openai-pool-retry-" + uuid.NewString()
}

func openAIWSIngressFallbackSessionSeed(userID, apiKeyID int64, groupID *int64) string {
	gid := int64(0)
	if groupID != nil {
		gid = *groupID
	}
	return fmt.Sprintf("openai_ws_ingress_conn:%d:%d:%d:%s", gid, userID, apiKeyID, uuid.NewString())
}

func isOpenAIWSUpgradeRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Connection"))), "upgrade")
}

func closeOpenAIClientWS(conn *coderws.Conn, status coderws.StatusCode, reason string) {
	if conn == nil {
		return
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 120 {
		reason = reason[:120]
	}
	_ = conn.Close(status, reason)
	_ = conn.CloseNow()
}

func closeOpenAIWSFailoverExhausted(conn *coderws.Conn, failoverErr *service.UpstreamFailoverError) {
	if failoverErr == nil {
		closeOpenAIClientWS(conn, coderws.StatusInternalError, "upstream websocket proxy failed")
		return
	}
	switch failoverErr.StatusCode {
	case http.StatusTooManyRequests:
		reason := "upstream rate limit exceeded, please retry later"
		if isOpenAIRetryableOverloadFailover(failoverErr) {
			reason = "upstream overloaded, retry later"
		}
		closeOpenAIClientWS(conn, coderws.StatusTryAgainLater, reason)
	case 529, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		reason := "upstream service temporarily unavailable"
		if msg := strings.TrimSpace(service.ExtractUpstreamErrorMessage(failoverErr.ResponseBody)); msg != "" {
			reason = msg
		}
		closeOpenAIClientWS(conn, coderws.StatusTryAgainLater, reason)
	case http.StatusUnauthorized, http.StatusForbidden:
		closeOpenAIClientWS(conn, coderws.StatusPolicyViolation, "upstream websocket authentication failed")
	default:
		reason := "upstream websocket proxy failed"
		if msg := strings.TrimSpace(service.ExtractUpstreamErrorMessage(failoverErr.ResponseBody)); msg != "" {
			reason = msg
		}
		closeOpenAIClientWS(conn, coderws.StatusInternalError, reason)
	}
}

func writeContentModerationWSError(ctx context.Context, conn *coderws.Conn, decision *service.ContentModerationDecision) {
	if conn == nil || decision == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	message := strings.TrimSpace(decision.Message)
	if message == "" {
		message = "content moderation blocked this request"
	}
	payload, err := json.Marshal(gin.H{
		"event_id": "evt_content_moderation_blocked",
		"type":     "error",
		"error": gin.H{
			"type":    "invalid_request_error",
			"code":    contentModerationErrorCode(decision),
			"message": message,
		},
	})
	if err != nil {
		payload = []byte(`{"event_id":"evt_content_moderation_blocked","type":"error","error":{"type":"invalid_request_error","code":"content_policy_violation","message":"content moderation blocked this request"}}`)
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = conn.Write(writeCtx, coderws.MessageText, payload)
}

func summarizeWSCloseErrorForLog(err error) (string, string) {
	if err == nil {
		return "-", "-"
	}
	statusCode := coderws.CloseStatus(err)
	if statusCode == -1 {
		return "-", "-"
	}
	closeStatus := fmt.Sprintf("%d(%s)", int(statusCode), statusCode.String())
	closeReason := "-"
	var closeErr coderws.CloseError
	if errors.As(err, &closeErr) {
		reason := strings.TrimSpace(closeErr.Reason)
		if reason != "" {
			closeReason = reason
		}
	}
	return closeStatus, closeReason
}
