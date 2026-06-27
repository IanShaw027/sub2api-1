package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Images handles OpenAI Images API requests.
// POST /v1/images/generations
// POST /v1/images/edits
func (h *OpenAIGatewayHandler) Images(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	requestStart := time.Now()

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
		"handler.openai_gateway.images",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

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

	if isMultipartImagesContentType(c.GetHeader("Content-Type")) {
		setOpsRequestContext(c, "", false, body)
	} else {
		setOpsRequestContext(c, "", false, body)
	}

	parsed, err := h.gatewayService.ParseOpenAIImagesRequest(c, body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	requestModel := parsed.Model

	reqLog = reqLog.With(
		zap.String("model", requestModel),
		zap.Bool("stream", parsed.Stream),
		zap.Bool("multipart", parsed.Multipart),
		zap.String("capability", string(parsed.RequiredCapability)),
	)
	setOpsEndpointContext(c, "", int16(service.RequestTypeImage))
	h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
		Stage:          "request_received",
		EndpointKind:   "images",
		RequestStart:   requestStart,
		APIKey:         apiKey,
		UserID:         subject.UserID,
		RequestedModel: requestModel,
		Stream:         parsed.Stream,
		Fields: map[string]any{
			"inbound_endpoint":   GetInboundEndpoint(c),
			"request_body_bytes": len(body),
			"image_endpoint":     parsed.Endpoint,
			"multipart":          parsed.Multipart,
			"capability":         string(parsed.RequiredCapability),
		},
	})

	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if service.IsGroupContextValid(apiKey.Group) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, apiKey.Group))
	}
	if !service.GroupAllowsOpenAIImagesCodex(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.OpenAIImagesCodexDisabledMessage())
		return
	}
	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, parsed.Model, parsed.ModerationBody()); decision != nil && decision.Blocked {
		h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return
	}
	imageReleaseFunc, acquired := h.acquireImageGenerationSlot(c, streamStarted)
	if !acquired {
		return
	}
	if imageReleaseFunc != nil {
		defer imageReleaseFunc()
	}

	if parsed.Multipart {
		setOpsRequestContext(c, requestModel, parsed.Stream, body)
	} else {
		setOpsRequestContext(c, requestModel, parsed.Stream, body)
	}

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, requestModel)

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userSlotStart := time.Now()
	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, parsed.Stream, &streamStarted, reqLog)
	userSlotWaitMs := time.Since(userSlotStart).Milliseconds()
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai.images.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.handleStreamingAwareError(c, status, code, message, streamStarted)
		return
	}

	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
	requestCtx := service.WithOpenAIImageGenerationIntent(c.Request.Context())

	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError
	var lastFailoverAccount *service.Account

	for {
		reqLog.Debug("openai.images.account_selecting", zap.Int("excluded_account_count", len(failedAccountIDs)))
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForImages(
			requestCtx,
			apiKey.GroupID,
			sessionHash,
			requestModel,
			failedAccountIDs,
			parsed.RequiredCapability,
			service.GroupImageGenerationRouteCodex,
			false,
		)
		if err != nil {
			reqLog.Warn("openai.images.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if len(failedAccountIDs) == 0 {
				if h.handleOpenAIGroupModelUnsupportedError(c, err, streamStarted) {
					return
				}
				cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, requestModel, requestModel, service.PlatformOpenAI)
				if !cls.ModelNotFound {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
				}
				message := cls.Message
				if !cls.ModelNotFound {
					message = "No available compatible accounts"
				}
				h.handleStreamingAwareError(c, cls.Status, cls.ErrType, message, streamStarted)
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, lastFailoverAccount, streamStarted)
			} else {
				markOpsRoutingCapacityLimited(c)
				msg := buildOpenAISelectionExhaustedMessage(err, "No available compatible accounts", len(failedAccountIDs) > 0)
				h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", msg, streamStarted)
			}
			return
		}
		if selection == nil || selection.Account == nil {
			cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, requestModel, requestModel, service.PlatformOpenAI)
			if !cls.ModelNotFound {
				markOpsRoutingCapacityLimited(c)
			}
			message := cls.Message
			if !cls.ModelNotFound {
				message = "No available compatible accounts"
			}
			h.handleStreamingAwareError(c, cls.Status, cls.ErrType, message, streamStarted)
			return
		}

		reqLog.Debug("openai.images.account_schedule_decision",
			zap.String("layer", scheduleDecision.Layer),
			zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
			zap.Int("candidate_count", scheduleDecision.CandidateCount),
			zap.Int("top_k", scheduleDecision.TopK),
			zap.Int64("latency_ms", scheduleDecision.LatencyMs),
			zap.Float64("load_skew", scheduleDecision.LoadSkew),
		)

		account := selection.Account
		sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
		reqLog.Debug("openai.images.account_selected", zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
		setOpsSelectedAccount(c, account.ID, account.Platform)
		h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
			Stage:          "account_selected",
			EndpointKind:   "images",
			RequestStart:   requestStart,
			APIKey:         apiKey,
			Account:        account,
			UserID:         subject.UserID,
			RequestedModel: requestModel,
			Stream:         parsed.Stream,
			SwitchCount:    switchCount,
			Fields: map[string]any{
				"inbound_endpoint":     GetInboundEndpoint(c),
				"image_endpoint":       parsed.Endpoint,
				"multipart":            parsed.Multipart,
				"capability":           string(parsed.RequiredCapability),
				"session_hash_present": strings.TrimSpace(sessionHash) != "",
				"scheduler_layer":      scheduleDecision.Layer,
				"sticky_session_hit":   scheduleDecision.StickySessionHit,
				"candidate_count":      scheduleDecision.CandidateCount,
				"top_k":                scheduleDecision.TopK,
				"scheduler_latency_ms": scheduleDecision.LatencyMs,
				"load_skew":            scheduleDecision.LoadSkew,
			},
		})

		accountSlotStart := time.Now()
		accountReleaseFunc, acquireStatus := h.acquireResponsesAccountSlot(c, apiKey.GroupID, apiKey.ID, sessionHash, "", selection, parsed.Stream, &streamStarted, reqLog)
		accountSlotWaitMs := time.Since(accountSlotStart).Milliseconds()
		if acquireStatus == accountSlotAcquireRetry {
			failedAccountIDs[account.ID] = struct{}{}
			continue
		}
		if acquireStatus != accountSlotAcquireAcquired {
			return
		}
		slotFields := map[string]any{
			"inbound_endpoint":     GetInboundEndpoint(c),
			"image_endpoint":       parsed.Endpoint,
			"user_slot_wait_ms":    userSlotWaitMs,
			"account_slot_wait_ms": accountSlotWaitMs,
			"session_hash_present": strings.TrimSpace(sessionHash) != "",
		}
		if selection.WaitPlan != nil {
			slotFields["account_wait_timeout_ms"] = selection.WaitPlan.Timeout.Milliseconds()
			slotFields["account_max_concurrency"] = selection.WaitPlan.MaxConcurrency
			slotFields["account_max_waiting"] = selection.WaitPlan.MaxWaiting
		}
		h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
			Stage:          "concurrency_slots_acquired",
			EndpointKind:   "images",
			RequestStart:   requestStart,
			APIKey:         apiKey,
			Account:        account,
			UserID:         subject.UserID,
			RequestedModel: requestModel,
			Stream:         parsed.Stream,
			SwitchCount:    switchCount,
			Fields:         slotFields,
		})

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		result, err := func() (*service.OpenAIForwardResult, error) {
			defer func() {
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
			}()
			return h.gatewayService.ForwardImages(requestCtx, c, account, body, parsed, channelMapping.MappedModel)
		}()
		requestPayloadHash := service.HashUsageRequestPayload(body)
		if parsed.Multipart {
			requestPayloadHash = service.HashUsageRequestPayload([]byte(parsed.StickySessionSeed()))
		}
		upstreamModel := ""
		if result != nil {
			upstreamModel = result.UpstreamModel
		}
		cyberBlockKeyImages := ""
		if service.GetOpsCyberPolicy(c) != nil {
			cyberBlockKeyImages = service.CyberSessionBlockKey(apiKey.ID, c, body)
		}
		h.recordCyberPolicyIfMarked(c, apiKey, account, subscription, requestModel, err != nil, cyberBlockKeyImages, channelMapping.ToUsageFields(parsed.Model, upstreamModel), requestPayloadHash, service.ContentModerationProtocolOpenAIImages, body)
		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
		if result != nil && result.FirstTokenMs != nil {
			service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
		}
		if err != nil {
			if result != nil && result.ImageCount > 0 {
				reqLog.Warn("openai.images.forward_partial_error_with_image_result",
					zap.Int64("account_id", account.ID),
					zap.Int("image_count", result.ImageCount),
					zap.Error(err),
				)
			} else {
				var failoverErr *service.UpstreamFailoverError
				if errors.As(err, &failoverErr) {
					h.reportOpenAIAccountScheduleFailure(c, account.ID, err)
					if failoverErr.RetryableOnSameAccount {
						retryLimit := account.GetPoolModeRetryCount()
						if sameAccountRetryCount[account.ID] < retryLimit {
							sameAccountRetryCount[account.ID]++
							h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
								Stage:             "attempt_finished",
								EndpointKind:      "images",
								RequestStart:      requestStart,
								APIKey:            apiKey,
								Account:           account,
								UserID:            subject.UserID,
								RequestedModel:    requestModel,
								Stream:            parsed.Stream,
								SwitchCount:       switchCount,
								ForwardDurationMs: forwardDurationMs,
								OpenAIResult:      result,
								Err:               err,
								Fields: map[string]any{
									"outcome":                  "retry_same_account",
									"inbound_endpoint":         GetInboundEndpoint(c),
									"image_endpoint":           parsed.Endpoint,
									"upstream_status":          failoverErr.StatusCode,
									"same_account_retry_count": sameAccountRetryCount[account.ID],
									"same_account_retry_limit": retryLimit,
								},
							})
							reqLog.Warn("openai.images.pool_mode_same_account_retry",
								zap.Int64("account_id", account.ID),
								zap.Int("upstream_status", failoverErr.StatusCode),
								zap.Int("retry_limit", retryLimit),
								zap.Int("retry_count", sameAccountRetryCount[account.ID]),
							)
							select {
							case <-requestCtx.Done():
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
						EndpointKind:      "images",
						RequestStart:      requestStart,
						APIKey:            apiKey,
						Account:           account,
						UserID:            subject.UserID,
						RequestedModel:    requestModel,
						Stream:            parsed.Stream,
						SwitchCount:       switchCount,
						ForwardDurationMs: forwardDurationMs,
						OpenAIResult:      result,
						Err:               err,
						Fields: map[string]any{
							"outcome":          "failover",
							"inbound_endpoint": GetInboundEndpoint(c),
							"image_endpoint":   parsed.Endpoint,
							"upstream_status":  failoverErr.StatusCode,
						},
					})
					if switchCount >= maxAccountSwitches {
						h.handleFailoverExhausted(c, failoverErr, account, streamStarted)
						return
					}
					switchCount++
					if h.gatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, switchCount) {
						h.handleFailoverExhausted(c, failoverErr, account, streamStarted)
						return
					}
					reqLog.Warn("openai.images.upstream_failover_switching",
						zap.Int64("account_id", account.ID),
						zap.Int("upstream_status", failoverErr.StatusCode),
						zap.Int("switch_count", switchCount),
						zap.Int("max_switches", maxAccountSwitches),
					)
					continue
				}
				h.reportOpenAIAccountScheduleFailure(c, account.ID, err)
				if shouldSuppressForwardErrorResponse(c, err) {
					return
				}
				upstreamErrorAlreadyCommunicated := c.Writer.Written()
				wroteFallback := h.ensureForwardErrorResponse(c, streamStarted, err)
				h.gatewayService.EmitOpenAIGatewayDebugTimelineEvent(c, service.OpenAIGatewayDebugTimelineEventInput{
					Stage:             "attempt_finished",
					EndpointKind:      "images",
					RequestStart:      requestStart,
					APIKey:            apiKey,
					Account:           account,
					UserID:            subject.UserID,
					RequestedModel:    requestModel,
					Stream:            parsed.Stream,
					SwitchCount:       switchCount,
					ForwardDurationMs: forwardDurationMs,
					OpenAIResult:      result,
					Err:               err,
					Fields: map[string]any{
						"outcome":                                 "error",
						"inbound_endpoint":                        GetInboundEndpoint(c),
						"image_endpoint":                          parsed.Endpoint,
						"fallback_error_response_written":         wroteFallback,
						"upstream_error_response_already_written": upstreamErrorAlreadyCommunicated,
						"stream_started":                          streamStarted,
					},
				})
				fields := []zap.Field{
					zap.Int64("account_id", account.ID),
					zap.Bool("fallback_error_response_written", wroteFallback),
					zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
					zap.Error(err),
				}
				if shouldLogOpenAIForwardFailureAsWarn(c, wroteFallback) {
					reqLog.Warn("openai.images.forward_failed", fields...)
					return
				}
				reqLog.Error("openai.images.forward_failed", fields...)
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
			EndpointKind:      "images",
			RequestStart:      requestStart,
			APIKey:            apiKey,
			Account:           account,
			UserID:            subject.UserID,
			RequestedModel:    requestModel,
			Stream:            parsed.Stream,
			SwitchCount:       switchCount,
			ForwardDurationMs: forwardDurationMs,
			OpenAIResult:      result,
			Err:               err,
			Fields: map[string]any{
				"outcome":              "success",
				"inbound_endpoint":     GetInboundEndpoint(c),
				"image_endpoint":       parsed.Endpoint,
				"session_hash_present": strings.TrimSpace(sessionHash) != "",
				"routed_model":         channelMapping.MappedModel,
			},
		})

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		inboundEndpoint := GetInboundEndpoint(c)
		upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
		quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)

		cyberBlocked := service.GetOpsCyberPolicy(c) != nil
		h.submitMandatoryUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
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
				ChannelUsageFields: channelMapping.ToUsageFields(parsed.Model, upstreamModel),
				CyberBlocked:       cyberBlocked,
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.images"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.Any("group_id", apiKey.GroupID),
					zap.String("model", requestModel),
					zap.Int64("account_id", account.ID),
				).Error("openai.images.record_usage_failed", zap.Error(err))
			}
		})

		reqLog.Debug("openai.images.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return
	}
}

func isMultipartImagesContentType(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "multipart/form-data")
}
