package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func shouldApplyAnthropicCompatFullReplayGuard(account *Account, previousResponseID string, continuationDisabled bool, compatReplayGuardEnabled bool) bool {
	if !compatReplayGuardEnabled || continuationDisabled {
		return false
	}
	if account == nil || account.Type == AccountTypeOAuth {
		return false
	}
	if strings.TrimSpace(previousResponseID) != "" {
		return false
	}
	return true
}

func ShouldForwardOpenAITextMessagesViaChatCompletions(account *Account) bool {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey || !account.TextEndpointAutoRouteEnabled() {
		return false
	}
	if !openai_compat.ShouldUseResponsesAPI(account.Extra) {
		return true
	}
	configured, found := account.openAIEndpointCapabilitySet()
	return found && configured[string(OpenAIEndpointCapabilityChatCompletions)] &&
		!configured[string(OpenAIEndpointCapabilityResponsesIngress)] &&
		!configured[string(OpenAIEndpointCapabilityAnthropicMessagesIngress)]
}

func ShouldForwardOpenAITextResponsesViaAnthropicMessages(account *Account) bool {
	if account == nil || !account.IsAnthropicMessagesUpstream() || !account.TextEndpointAutoRouteEnabled() {
		return false
	}
	return openai_compat.ResolveResponsesSupport(account.Extra) != openai_compat.ResponsesSupportNo
}

func (s *OpenAIGatewayService) forwardAnthropicMessagesViaRawChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*OpenAIForwardResult, error) {
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), "messages")
	if err != nil {
		return nil, err
	}
	gateway := s.bridgeGatewayService()
	result, err := gateway.forwardMessagesToChatCompletions(ctx, c, account, parsed)
	if result == nil {
		return nil, err
	}
	return openAIForwardResultFromGatewayResult(result), err
}

func (s *OpenAIGatewayService) forwardResponsesToAnthropicMessages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*OpenAIForwardResult, error) {
	gateway := s.bridgeGatewayService()
	result, err := gateway.ForwardAsResponses(ctx, c, account, body, nil)
	if result == nil {
		return nil, err
	}
	return openAIForwardResultFromGatewayResult(result), err
}

func (s *OpenAIGatewayService) bridgeGatewayService() *GatewayService {
	if s == nil {
		return &GatewayService{}
	}
	if s.gatewayService != nil {
		return s.gatewayService
	}
	return &GatewayService{
		accountRepo:           s.accountRepo,
		usageLogRepo:          s.usageLogRepo,
		usageBillingRepo:      s.usageBillingRepo,
		userRepo:              s.userRepo,
		userSubRepo:           s.userSubRepo,
		cache:                 s.cache,
		cfg:                   s.cfg,
		schedulerSnapshot:     s.schedulerSnapshot,
		billingService:        s.billingService,
		rateLimitService:      s.rateLimitService,
		billingCacheService:   s.billingCacheService,
		httpUpstream:          s.httpUpstream,
		concurrencyService:    s.concurrencyService,
		settingService:        s.settingService,
		responseHeaderFilter:  s.responseHeaderFilter,
		channelService:        s.channelService,
		resolver:              s.resolver,
		tlsFPProfileService:   s.tlsFPProfileService,
		balanceNotifyService:  s.balanceNotifyService,
		userPlatformQuotaRepo: s.userPlatformQuotaRepo,
		fingerprintNormalizer: s.fingerprintNormalizer,
	}
}

func openAIForwardResultFromGatewayResult(result *ForwardResult) *OpenAIForwardResult {
	if result == nil {
		return nil
	}
	return &OpenAIForwardResult{
		RequestID: result.RequestID,
		Usage: OpenAIUsage{
			InputTokens:              result.Usage.InputTokens,
			OutputTokens:             result.Usage.OutputTokens,
			CacheCreationInputTokens: result.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     result.Usage.CacheReadInputTokens,
			ImageOutputTokens:        result.Usage.ImageOutputTokens,
		},
		Model:              result.Model,
		UpstreamModel:      result.UpstreamModel,
		ReasoningEffort:    result.ReasoningEffort,
		Stream:             result.Stream,
		Duration:           result.Duration,
		FirstTokenMs:       result.FirstTokenMs,
		ClientDisconnected: result.ClientDisconnect,
		ClientDisconnect:   result.ClientDisconnect,
		ImageCount:         result.ImageCount,
		ImageSize:          result.ImageSize,
		ImageInputSize:     result.ImageInputSize,
		ImageOutputSize:    result.ImageOutputSize,
		ImageOutputSizes:   result.ImageOutputSizes,
		ImageSizeSource:    result.ImageSizeSource,
		ImageSizeBreakdown: result.ImageSizeBreakdown,
	}
}

// ForwardAsAnthropic accepts an Anthropic Messages request body, converts it
// to OpenAI Responses API format, forwards to the OpenAI upstream, and converts
// the response back to Anthropic Messages format. This enables Claude Code
// clients to access OpenAI models through the standard /v1/messages endpoint.
func (s *OpenAIGatewayService) ForwardAsAnthropic(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	promptCacheKey string,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	if account == nil {
		return nil, errors.New("account is required")
	}
	startTime := time.Now()
	clearOpenAICodexCompatContext(c)
	if ShouldForwardOpenAITextMessagesViaChatCompletions(account) {
		return s.forwardAnthropicMessagesViaRawChatCompletions(ctx, c, account, body)
	}

	// 1. Parse Anthropic request
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	originalModel := anthropicReq.Model
	applyOpenAICompatModelNormalization(&anthropicReq)
	normalizedModel := anthropicReq.Model
	clientStream := anthropicReq.Stream // client's original stream preference
	toolNameMap := anthropicToolNameMap(anthropicReq.Tools)
	stripAnthropicBillingHeaderFromSystem(&anthropicReq)
	claudeCodeClient := c != nil && c.Request != nil && IsClaudeCodeClient(c.Request.Context())
	anthropicBetaHeader := ""
	if c != nil {
		anthropicBetaHeader = c.GetHeader("anthropic-beta")
	}

	isStream := true
	forcedDispatchModel := getOpenAIMessagesDispatchForcedModel(c)
	billingModel := resolveOpenAIForwardModelWithSettings(ctx, s.settingService, account, normalizedModel, defaultMappedModel)
	if forcedDispatchModel != "" {
		billingModel = forcedDispatchModel
	}
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	explicitPromptCacheKey := strings.TrimSpace(promptCacheKey)
	promptCacheKey = strings.TrimSpace(promptCacheKey)
	derivedPromptCacheKey := false
	if promptCacheKey == "" && shouldAutoInjectPromptCacheKeyForCompat(upstreamModel) {
		if metadataKey := promptCacheKeyFromAnthropicMetadataSession(&anthropicReq); metadataKey != "" {
			promptCacheKey = metadataKey
		} else {
			promptCacheKey = deriveAnthropicCompatPromptCacheKey(&anthropicReq, upstreamModel)
		}
		derivedPromptCacheKey = promptCacheKey != ""
	}
	compatReplayGuardEnabled := shouldAutoInjectPromptCacheKeyForCompat(upstreamModel)
	compatContinuationEnabled := openAICompatContinuationEnabled(account, upstreamModel)
	compatContinuationDisabled := compatContinuationEnabled &&
		s.isOpenAICompatSessionContinuationDisabled(ctx, c, account, promptCacheKey)
	previousResponseID := ""
	if compatContinuationEnabled {
		previousResponseID = s.getOpenAICompatSessionResponseID(ctx, c, account, promptCacheKey)
	}
	oauthTurnStateBridge := account.Type == AccountTypeOAuth && explicitPromptCacheKey != ""
	oauthDerivedSessionBridge := account.Type == AccountTypeOAuth &&
		derivedPromptCacheKey &&
		isOpenAICompatMessagesBridgePromptCacheKey(promptCacheKey)
	if oauthTurnStateBridge {
		setOpenAICompatMessagesBridgeContext(c, true)
	}
	if shouldApplyAnthropicCompatFullReplayGuard(account, previousResponseID, compatContinuationDisabled, compatReplayGuardEnabled) {
		applyAnthropicCompatFullReplayGuard(&anthropicReq)
	}

	// 2. Convert Anthropic → Responses
	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("convert anthropic to responses: %w", err)
	}
	if claudeCodeClient {
		apicompat.AugmentClaudeToolDescriptions(responsesReq.Tools)
	}

	// Upstream always uses streaming (upstream may not support sync mode).
	// The client's original preference determines the response format.
	responsesReq.Stream = true

	// 2b. Handle BetaFastMode → service_tier: "priority"
	if containsBetaToken(anthropicBetaHeader, claude.BetaFastMode) {
		responsesReq.ServiceTier = "priority"
	}

	// 3. Model mapping
	responsesReq.Model = upstreamModel
	if forcedDispatchModel != "" {
		if forcedEffort := deriveOpenAIReasoningEffortFromModel(forcedDispatchModel); forcedEffort != "" {
			if responsesReq.Reasoning == nil {
				responsesReq.Reasoning = &apicompat.ResponsesReasoning{Summary: "auto"}
			}
			responsesReq.Reasoning.Effort = forcedEffort
			if strings.TrimSpace(responsesReq.Reasoning.Summary) == "" {
				responsesReq.Reasoning.Summary = "auto"
			}
		}
	}

	logger.L().Debug("openai messages: model mapping applied",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("normalized_model", normalizedModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", isStream),
	)

	// 4. Marshal Responses request body, then apply OAuth codex transform
	if compatContinuationEnabled {
		if previousResponseID != "" {
			responsesReq.PreviousResponseID = previousResponseID
			trimAnthropicCompatResponsesInputToLatestTurn(responsesReq)
		}
		appendOpenAICompatClaudeCodeTodoGuard(responsesReq)
	}
	oauthTurnState := ""
	if oauthTurnStateBridge {
		oauthTurnState = s.getOpenAICompatSessionTurnState(ctx, c, account, promptCacheKey)
	}
	responsesBody, err := json.Marshal(responsesReq)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	probeRequestBody := append([]byte(nil), responsesBody...)
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()

	var oauthReqBody map[string]any
	if account.Type == AccountTypeOAuth && account.Platform != PlatformGrok {
		var reqBody map[string]any
		if err := json.Unmarshal(responsesBody, &reqBody); err != nil {
			return nil, fmt.Errorf("unmarshal for codex transform: %w", err)
		}
		forcedTemplateText := ""
		if s.cfg != nil {
			forcedTemplateText = s.cfg.Gateway.ForcedCodexInstructionsTemplate
		}
		embedDefaultInstructions := claudeCodeClient &&
			strings.TrimSpace(forcedTemplateText) == ""
		codexResult := applyCodexOAuthTransformWithInputModeAndOptions(
			reqBody,
			false,
			false,
			codexTransformInputModePreservePrefix,
			codexOAuthTransformOptions{
				SkipDefaultInstructions: !embedDefaultInstructions,
			},
		)
		if c != nil {
			c.Set(openAICodexTransformObsKey, codexResult.Observability)
		}
		templateUpstreamModel := upstreamModel
		if codexResult.NormalizedModel != "" {
			templateUpstreamModel = codexResult.NormalizedModel
		}
		existingInstructions, _ := reqBody["instructions"].(string)
		if _, err := applyForcedCodexInstructionsTemplate(reqBody, forcedTemplateText, forcedCodexInstructionsTemplateData{
			ExistingInstructions: strings.TrimSpace(existingInstructions),
			OriginalModel:        originalModel,
			NormalizedModel:      normalizedModel,
			BillingModel:         billingModel,
			UpstreamModel:        templateUpstreamModel,
		}); err != nil {
			return nil, err
		}
		if codexResult.NormalizedModel != "" {
			upstreamModel = codexResult.NormalizedModel
		}
		if codexResult.PromptCacheKey != "" {
			promptCacheKey = codexResult.PromptCacheKey
		}
		if promptCacheKey != "" && (!oauthTurnStateBridge || strings.TrimSpace(oauthTurnState) == "") {
			reqBody["prompt_cache_key"] = promptCacheKey
		}
		// OAuth codex transform forces stream=true upstream, so always use
		// the streaming response handler regardless of what the client asked.
		isStream = true
		responsesBody, err = marshalOpenAIResponsesRequestBodyOrdered(reqBody)
		if err != nil {
			return nil, fmt.Errorf("remarshal after codex transform: %w", err)
		}
		oauthReqBody = reqBody
	}

	// For API key accounts (including OpenAI-compatible upstream gateways) and
	// Grok OAuth subscription accounts, ensure promptCacheKey is also propagated
	// via the request body so upstream Responses can derive a stable session id.
	// This keeps Anthropic /v1/messages (Claude Code) multi-turn sticky on Grok.
	if account.Type == AccountTypeAPIKey || account.Platform == PlatformGrok {
		if trimmedKey := strings.TrimSpace(promptCacheKey); trimmedKey != "" {
			var reqBody map[string]any
			if err := json.Unmarshal(responsesBody, &reqBody); err != nil {
				return nil, fmt.Errorf("unmarshal for prompt cache key injection: %w", err)
			}
			if existing, ok := reqBody["prompt_cache_key"].(string); !ok || strings.TrimSpace(existing) == "" {
				reqBody["prompt_cache_key"] = trimmedKey
				recordOpenAICompatPromptCacheInjected()
				updated, err := marshalOpenAIResponsesRequestBodyOrdered(reqBody)
				if err != nil {
					return nil, fmt.Errorf("remarshal after prompt cache key injection: %w", err)
				}
				responsesBody = updated
			}
		}
	}

	updatedBody, policyErr := s.applyOpenAIFastPolicyToBody(ctx, account, upstreamModel, responsesBody)
	if policyErr != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(policyErr, &blocked) {
			MarkOpsClientBusinessLimited(c, OpsClientBusinessLimitedReasonLocalPolicyDenied)
			writeAnthropicError(c, http.StatusForbidden, "permission_error", blocked.Message)
		}
		return nil, policyErr
	}
	responsesBody = updatedBody
	if account.Platform == PlatformGrok {
		patchedBody, patchErr := patchGrokResponsesBody(responsesBody, upstreamModel)
		if patchErr != nil {
			return nil, patchErr
		}
		responsesBody = patchedBody
	}

	// 5. Get access token
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// 6. Build upstream request
	if account.Type == AccountTypeOAuth && account.Platform != PlatformGrok {
		// Every Anthropic Messages conversion is a compatibility bridge, even
		// when a non-Codex target model produces no prompt-cache marker. Mark it
		// before the shared builder so its intentionally minimal identity shape
		// (non-browser client UA preserved unless an explicit runtime override is
		// configured, no originator) cannot be normalized as a regular OAuth
		// Responses request.
		setOpenAICompatMessagesBridgeContext(c, true)
	}
	var upstreamReq *http.Request
	if account.Platform == PlatformGrok {
		upstreamReq, err = buildGrokResponsesRequest(upstreamCtx, c, account, responsesBody, token)
	} else {
		upstreamReq, err = s.buildUpstreamRequest(upstreamCtx, c, account, responsesBody, token, isStream, promptCacheKey, isOpenAICodexOfficialClientRequest(c))
	}
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}

	// Override session_id with a deterministic UUID derived from the isolated
	// session key, ensuring different API keys produce different upstream sessions.
	// Grok OAuth Claude Code traffic also needs isolation so concurrent keys do
	// not share xAI session state.
	if (account.Type == AccountTypeAPIKey || account.Platform == PlatformGrok) && promptCacheKey != "" {
		apiKeyID := getAPIKeyIDFromContext(c)
		upstreamReq.Header.Set("session_id", generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey)))
	}
	if (oauthTurnStateBridge || oauthDerivedSessionBridge) && promptCacheKey != "" {
		apiKeyID := getAPIKeyIDFromContext(c)
		upstreamReq.Header.Set("session_id", generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey)))
	}
	if account.Type == AccountTypeOAuth && account.Platform != PlatformGrok {
		// Anthropic Messages compatibility uses the ChatGPT Codex SSE endpoint.
		// Match airgate-openai's request shape: the SSE endpoint does not need
		// the Responses experimental beta header, and forcing originator can make
		// ChatGPT select a different internal continuation path.
		upstreamReq.Header.Del("OpenAI-Beta")
		upstreamReq.Header.Del("originator")
	}
	if oauthTurnStateBridge && strings.TrimSpace(oauthTurnState) != "" && upstreamReq.Header.Get("x-codex-turn-state") == "" {
		upstreamReq.Header.Set("x-codex-turn-state", strings.TrimSpace(oauthTurnState))
	}

	// 7. Send request
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	tlsRuntime := s.resolveOpenAICompatibleTLSFingerprintRuntime(ctx, c, account, "http")
	httpCodexCompatRetryTried := false
	var resp *http.Response
	for {
		applyOpenAICodexTLSFingerprintRuntime(upstreamReq, tlsRuntime, account, true)
		upstreamReq = withOpenAIHTTP1RawHeaderReplay(upstreamReq, account, tlsRuntime.Profile)
		SetOpsLatencyMs(c, OpsOpenAIForwardPrepareLatencyMsKey, time.Since(startTime).Milliseconds())
		upstreamStart := time.Now()
		resp, err = s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		if err != nil {
			safeErr := sanitizeUpstreamErrorMessage(err.Error())
			detail := recordDetailedUpstreamTransportError(c, err)
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: 0,
				Kind:               "request_error",
				Message:            safeErr,
			})
			writeAnthropicError(c, http.StatusBadGateway, detail.ErrorType, formatUpstreamRequestFailed(detail, "Upstream request failed"))
			return nil, fmt.Errorf("upstream request failed: %s", safeErr)
		}
		defer func() { _ = resp.Body.Close() }()

		// 8. Handle error response with failover
		if resp.StatusCode >= 400 {
			recordOpenAICompatUpstreamStatus(upstreamModel, resp.StatusCode)
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			_ = resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
			if account.Platform == PlatformGrok {
				s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
				s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			}

			upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
			upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
			upstreamCode := extractUpstreamErrorCode(respBody)
			continuationPreviousResponseMissing := compatContinuationEnabled &&
				!httpCodexCompatRetryTried &&
				isOpenAICompatPreviousResponseNotFound(resp.StatusCode, upstreamMsg, respBody)
			continuationRequiresWebSocketV2 := compatContinuationEnabled &&
				!httpCodexCompatRetryTried &&
				isOpenAICompatPreviousResponseUnsupported(resp.StatusCode, upstreamMsg, respBody)
			if continuationPreviousResponseMissing || continuationRequiresWebSocketV2 {
				httpCodexCompatRetryTried = true
				if continuationRequiresWebSocketV2 {
					s.disableOpenAICompatSessionContinuation(ctx, c, account, promptCacheKey)
				} else {
					s.deleteOpenAICompatSessionResponseID(ctx, c, account, promptCacheKey)
				}
				var replayReq apicompat.AnthropicRequest
				if err := json.Unmarshal(body, &replayReq); err != nil {
					return nil, fmt.Errorf("reparse anthropic request for compat replay: %w", err)
				}
				applyOpenAICompatModelNormalization(&replayReq)
				stripAnthropicBillingHeaderFromSystem(&replayReq)
				applyAnthropicCompatFullReplayGuard(&replayReq)
				replayResponsesReq, convErr := apicompat.AnthropicToResponses(&replayReq)
				if convErr != nil {
					return nil, fmt.Errorf("rebuild anthropic compat replay request: %w", convErr)
				}
				replayResponsesReq.Model = upstreamModel
				replayResponsesReq.Stream = true
				appendOpenAICompatClaudeCodeTodoGuard(replayResponsesReq)
				if containsBetaToken(c.GetHeader("anthropic-beta"), claude.BetaFastMode) {
					replayResponsesReq.ServiceTier = "priority"
				}
				if promptCacheKey != "" {
					replayBodyMap := map[string]any{}
					replayBodyBytes, marshalErr := json.Marshal(replayResponsesReq)
					if marshalErr != nil {
						return nil, fmt.Errorf("marshal anthropic compat replay request: %w", marshalErr)
					}
					if err := json.Unmarshal(replayBodyBytes, &replayBodyMap); err != nil {
						return nil, fmt.Errorf("unmarshal anthropic compat replay body: %w", err)
					}
					replayBodyMap["prompt_cache_key"] = promptCacheKey
					replayBodyBytes, marshalErr = marshalOpenAIResponsesRequestBodyOrdered(replayBodyMap)
					if marshalErr != nil {
						return nil, fmt.Errorf("remarshal anthropic compat replay body: %w", marshalErr)
					}
					responsesBody = replayBodyBytes
				} else {
					var marshalErr error
					responsesBody, marshalErr = json.Marshal(replayResponsesReq)
					if marshalErr != nil {
						return nil, fmt.Errorf("marshal anthropic compat replay request without key: %w", marshalErr)
					}
				}
				upstreamReq, err = s.buildUpstreamRequest(upstreamCtx, c, account, responsesBody, token, isStream, promptCacheKey, isOpenAICodexOfficialClientRequest(c))
				if err != nil {
					return nil, fmt.Errorf("build upstream request after anthropic compat replay: %w", err)
				}
				if account.Type == AccountTypeAPIKey && promptCacheKey != "" {
					apiKeyID := getAPIKeyIDFromContext(c)
					upstreamReq.Header.Set("session_id", generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey)))
				}
				continue
			}
			if !httpCodexCompatRetryTried && account.Type == AccountTypeOAuth && oauthReqBody != nil {
				if fallbackReason := classifyOpenAICodexCompatFallback(resp.StatusCode, upstreamCode, upstreamMsg, respBody); fallbackReason != "" {
					updatedBody, codexResult, updatedPromptCacheKey, remarshalErr := remarshalOpenAIOAuthCompatFallbackBody(oauthReqBody, promptCacheKey, fallbackReason)
					if remarshalErr != nil {
						return nil, fmt.Errorf("remarshal messages compat fallback body: %w", remarshalErr)
					}
					httpCodexCompatRetryTried = true
					if c != nil {
						c.Set(openAICodexTransformObsKey, codexResult.Observability)
					}
					if codexResult.NormalizedModel != "" {
						upstreamModel = codexResult.NormalizedModel
					}
					if codexResult.Modified {
						responsesBody = updatedBody
						promptCacheKey = updatedPromptCacheKey
						upstreamReq, err = s.buildUpstreamRequest(upstreamCtx, c, account, responsesBody, token, isStream, promptCacheKey, isOpenAICodexOfficialClientRequest(c))
						if err != nil {
							return nil, fmt.Errorf("build upstream request after messages compat fallback: %w", err)
						}
						if account.Type == AccountTypeAPIKey && promptCacheKey != "" {
							apiKeyID := getAPIKeyIDFromContext(c)
							upstreamReq.Header.Set("session_id", generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey)))
						}
						logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Retrying compat messages request once with Codex compat fallback (account: %s, reason: %s)", account.Name, fallbackReason)
						continue
					}
				}
			}
			if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
				upstreamDetail := ""
				if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
					maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
					if maxBytes <= 0 {
						maxBytes = 2048
					}
					upstreamDetail = truncateString(string(respBody), maxBytes)
				}
				appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
					Platform:           account.Platform,
					AccountID:          account.ID,
					AccountName:        account.Name,
					UpstreamStatusCode: resp.StatusCode,
					UpstreamRequestID:  resp.Header.Get("x-request-id"),
					Kind:               "failover",
					Message:            upstreamMsg,
					Detail:             upstreamDetail,
				})
				if s.rateLimitService != nil {
					s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
				}
				s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, billingModel)
				return nil, &UpstreamFailoverError{
					StatusCode:             resp.StatusCode,
					ResponseBody:           respBody,
					RetryableOnSameAccount: account.IsPoolMode() && (account.IsPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody)),
				}
			}
			// Non-failover error: return Anthropic-formatted error to client
			return s.handleAnthropicErrorResponse(resp, c, account, billingModel)
		}
		break
	}

	// 9. Handle normal response
	// Upstream is always streaming; choose response format based on client preference.
	var result *OpenAIForwardResult
	var handleErr error
	if clientStream {
		result, handleErr = s.handleAnthropicStreamingResponse(resp, c, account, originalModel, billingModel, upstreamModel, toolNameMap, startTime)
	} else {
		// Client wants JSON: buffer the streaming response and assemble a JSON reply.
		result, handleErr = s.handleAnthropicBufferedStreamingResponse(resp, c, account, originalModel, billingModel, upstreamModel, toolNameMap, startTime)
	}

	// cyber_policy：标记已设、error 已按 Anthropic 格式发给客户端。丢弃 result、返回哨兵，
	// 使 handler 经 RecordCyberPolicyUsageLog 按真实 token 计费（而非走正常 RecordUsage 二次计费），
	// 且不计入 failover。
	if GetOpsCyberPolicy(c) != nil {
		if handleErr == nil {
			handleErr = errOpenAICyberPolicyForwarded
		}
		return nil, handleErr
	}

	// Propagate ServiceTier and ReasoningEffort to result for billing
	if handleErr == nil && result != nil {
		if responsesReq.ServiceTier != "" {
			st := responsesReq.ServiceTier
			result.ServiceTier = &st
		}
		if responsesReq.Reasoning != nil && responsesReq.Reasoning.Effort != "" {
			re := responsesReq.Reasoning.Effort
			result.ReasoningEffort = &re
		}
	}
	if handleErr == nil && result != nil {
		if compatContinuationEnabled {
			if strings.TrimSpace(result.ResponseID) != "" {
				s.bindOpenAICompatSessionResponseID(ctx, c, account, promptCacheKey, result.ResponseID)
			}
			if resp != nil {
				s.bindOpenAICompatSessionTurnState(ctx, c, account, promptCacheKey, resp.Header.Get("x-codex-turn-state"))
			}
		}
		if oauthTurnStateBridge && resp != nil {
			s.bindOpenAICompatSessionTurnState(ctx, c, account, promptCacheKey, resp.Header.Get("x-codex-turn-state"))
		}
		emitOpenAICacheProbeEvent(ctx, c, account, probeRequestBody, responsesBody, result, promptCacheKey, false)
	}

	// Extract and save Codex usage snapshot from response headers (for OAuth accounts).
	// 排除 spark 影子:其 codex_* 仅由 QueryUsage(/wham/usage bengalfox)更新(外审第7轮 P1)。
	if handleErr == nil && account.Type == AccountTypeOAuth && !account.IsShadow() {
		if account.Platform == PlatformGrok {
			s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
		} else if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
			s.updateCodexUsageSnapshot(ctx, account.ID, snapshot)
		}
	}

	return result, handleErr
}

// handleAnthropicErrorResponse reads an upstream error and returns it in
// Anthropic error format.
func (s *OpenAIGatewayService) handleAnthropicErrorResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestedModel ...string,
) (*OpenAIForwardResult, error) {
	return s.handleCompatErrorResponse(resp, c, account, writeAnthropicError, requestedModel...)
}

// handleAnthropicBufferedStreamingResponse reads all Responses SSE events from
// the upstream streaming response, finds the terminal event (response.completed
// / response.incomplete / response.failed), converts the complete response to
// Anthropic Messages JSON format, and writes it to the client.
// This is used when the client requested stream=false but the upstream is always
// streaming.
func (s *OpenAIGatewayService) handleAnthropicBufferedStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	originalModel string,
	billingModel string,
	upstreamModel string,
	toolNameMap map[string]string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	finalResponse, usage, acc, err := s.readOpenAICompatBufferedTerminal(c.Request.Context(), resp, "openai messages buffered", requestID, account)
	if err != nil {
		return nil, err
	}

	if finalResponse == nil {
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Upstream stream ended without a terminal response event")
		return nil, fmt.Errorf("upstream stream ended without terminal event")
	}

	// cyber_policy：上游硬阻断（response.failed）。anthropic buffered 原对 failed 无特殊分支，
	// 此处仅为 cyber 增加：以 Anthropic 错误格式回写，打请求级标记供 handler 事后写风控/邮件/
	// 按真实 token 计费，且绝不 failover/换号。
	if strings.TrimSpace(finalResponse.Status) == "failed" {
		payload, _ := json.Marshal(gin.H{"type": "response.failed", "response": finalResponse})
		errMessage := extractResponsesFailureMessage(finalResponse, payload)
		if errMessage == "" {
			errMessage = "Upstream response failed"
		}
		cyberHit, cyberCode, cyberMsg := detectOpenAICyberPolicy(payload)
		if cyberHit {
			MarkOpsCyberPolicy(c, CyberPolicyMark{
				Code:           cyberCode,
				Message:        cyberMsg,
				Body:           truncateString(string(payload), 4096),
				UpstreamStatus: http.StatusOK,
				UpstreamInTok:  usage.InputTokens,
				UpstreamOutTok: usage.OutputTokens,
			})
		}
		if err, matched := s.tryWriteOpenAIResponseFailedPassthrough(resp, c, account, payload, errMessage); matched {
			return nil, err
		}
		if cyberHit {
			clientMsg := cyberMsg
			if clientMsg == "" {
				clientMsg = "Request blocked by upstream cyber-security policy"
			}
			writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", clientMsg)
			return nil, fmt.Errorf("openai cyber_policy: %s", cyberMsg)
		}
		writeAnthropicError(c, http.StatusBadGateway, "upstream_error", errMessage)
		return nil, fmt.Errorf("upstream response failed: %s", errMessage)
	}

	// When the terminal event has an empty output array, reconstruct from
	// accumulated delta events so the client receives the full content.
	acc.SupplementResponseOutput(finalResponse)

	anthropicResp := apicompat.ResponsesToAnthropic(finalResponse, originalModel, toolNameMap)

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.JSON(http.StatusOK, anthropicResp)

	return &OpenAIForwardResult{
		RequestID:     requestID,
		ResponseID:    strings.TrimSpace(finalResponse.ID),
		Usage:         usage,
		Model:         originalModel,
		BillingModel:  billingModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

// handleAnthropicStreamingResponse reads Responses SSE events from upstream,
// converts each to Anthropic SSE events, and writes them to the client.
// When StreamKeepaliveInterval is configured, it uses a goroutine + channel
// pattern to send Anthropic ping events during periods of upstream silence,
// preventing proxy/client timeout disconnections.
func (s *OpenAIGatewayService) handleAnthropicStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	originalModel string,
	billingModel string,
	upstreamModel string,
	toolNameMap map[string]string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	state := apicompat.NewResponsesEventToAnthropicState()
	state.Model = originalModel
	state.ToolNameMap = toolNameMap
	var usage OpenAIUsage
	var responseID string
	var firstTokenMs *int
	firstChunk := true
	sawTerminalEvent := false
	clientDisconnected := false
	clientOutputStarted := false
	streamFailedErr := error(nil)

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	// resultWithUsage builds the final result snapshot.
	resultWithUsage := func() *OpenAIForwardResult {
		return &OpenAIForwardResult{
			RequestID:          requestID,
			ResponseID:         strings.TrimSpace(responseID),
			Usage:              usage,
			Model:              originalModel,
			BillingModel:       billingModel,
			UpstreamModel:      upstreamModel,
			Stream:             true,
			Duration:           time.Since(startTime),
			FirstTokenMs:       firstTokenMs,
			ClientDisconnected: clientDisconnected,
			ClientDisconnect:   clientDisconnected,
		}
	}

	// processDataLine handles a single "data: ..." SSE line from upstream.
	// Returns true once a terminal event has been processed.
	processDataLine := func(payload string) bool {
		if firstChunk {
			firstChunk = false
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			logger.L().Warn("openai messages stream: failed to parse event",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			return false
		}
		if event.Type == "response.failed" {
			payloadBytes := []byte(payload)
			errMessage := extractResponsesFailureMessage(event.Response, payloadBytes)
			if errMessage == "" {
				errMessage = "Upstream response failed"
			}
			cyberHit, cyberCode, cyberMsg := detectOpenAICyberPolicy(payloadBytes)
			if cyberHit {
				if event.Response != nil && event.Response.Usage != nil {
					usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
				}
				if event.Usage != nil {
					usage = copyOpenAIUsageFromResponsesUsage(event.Usage)
				}
				MarkOpsCyberPolicy(c, CyberPolicyMark{
					Code:           cyberCode,
					Message:        cyberMsg,
					Body:           truncateString(payload, 4096),
					UpstreamStatus: http.StatusOK,
					UpstreamInTok:  usage.InputTokens,
					UpstreamOutTok: usage.OutputTokens,
				})
			}
			if !clientOutputStarted && !c.Writer.Written() {
				if err, matched := s.tryWriteOpenAIResponseFailedPassthrough(resp, c, account, payloadBytes, errMessage); matched {
					clientDisconnected = true
					streamFailedErr = err
					sawTerminalEvent = true
					return true
				}
			}
			// cyber_policy 致命且不可重试：绝不 failover/换号。先解析 response.failed
			// 自带的真实 usage 再打请求级标记（供 handler 事后审计/按真实 token 计费），
			// 以 Anthropic SSE error 事件回写让客户端停止重试，丢弃后续转换输出。
			if cyberHit {
				if !clientDisconnected {
					clientMsg := cyberMsg
					if clientMsg == "" {
						clientMsg = "Request blocked by upstream cyber-security policy"
					}
					errBody := fmt.Sprintf(`{"type":"error","error":{"type":%q,"message":%q}}`, "invalid_request_error", clientMsg)
					if _, werr := fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", errBody); werr == nil {
						c.Writer.Flush()
					}
					clientDisconnected = true
				}
				sawTerminalEvent = true
				return true
			}
			if !clientOutputStarted && !c.Writer.Written() {
				writeAnthropicError(c, http.StatusBadGateway, "upstream_error", errMessage)
			} else if !clientDisconnected {
				errBody := fmt.Sprintf(`{"type":"error","error":{"type":%q,"message":%q}}`, "upstream_error", errMessage)
				if _, werr := fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", errBody); werr == nil {
					c.Writer.Flush()
				} else {
					clientDisconnected = true
				}
			}
			streamFailedErr = fmt.Errorf("upstream response failed: %s", errMessage)
			clientDisconnected = true
			sawTerminalEvent = true
			return true
		}

		isTerminalEvent := isOpenAICompatResponsesTerminalEvent(event.Type)
		if isTerminalEvent {
			if event.Response != nil {
				if id := strings.TrimSpace(event.Response.ID); id != "" {
					responseID = id
				}
				if event.Response.Usage != nil {
					usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
				}
			}
			if event.Usage != nil {
				usage = copyOpenAIUsageFromResponsesUsage(event.Usage)
			}
			sawTerminalEvent = true
		}

		// Convert to Anthropic events
		events := apicompat.ResponsesEventToAnthropicEvents(&event, state)
		for _, evt := range events {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(evt)
			if err != nil {
				logger.L().Warn("openai messages stream: failed to marshal event",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				continue
			}
			if clientDisconnected {
				continue
			}
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				clientDisconnected = true
				logger.L().Info("openai messages stream: client disconnected",
					zap.String("request_id", requestID),
				)
			} else {
				clientOutputStarted = true
			}
		}
		if len(events) > 0 && !clientDisconnected {
			c.Writer.Flush()
		}
		return isTerminalEvent
	}

	// finalizeStream sends any remaining Anthropic events and returns the result.
	finalizeStream := func() (*OpenAIForwardResult, error) {
		if !sawTerminalEvent {
			message := "OpenAI messages stream ended before a terminal event"
			if clientDisconnected {
				return resultWithUsage(), fmt.Errorf("stream usage incomplete: missing terminal event")
			}
			if !clientOutputStarted {
				return resultWithUsage(), s.newOpenAIStreamFailoverError(c, account, true, requestID, nil, message)
			}
			s.recordOpenAIMessagesStreamUpstreamError(c, account, requestID, "stream_missing_terminal", message)
			return resultWithUsage(), fmt.Errorf("stream usage incomplete: missing terminal event")
		}
		if streamFailedErr != nil {
			return resultWithUsage(), streamFailedErr
		}
		if clientDisconnected {
			return resultWithUsage(), nil
		}
		if finalEvents := apicompat.FinalizeResponsesAnthropicStream(state); len(finalEvents) > 0 {
			for _, evt := range finalEvents {
				sse, err := apicompat.ResponsesAnthropicEventToSSE(evt)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprint(c.Writer, sse); err == nil {
					clientOutputStarted = true
				}
			}
			c.Writer.Flush()
		}
		return resultWithUsage(), nil
	}

	// handleScanErr logs scanner errors if meaningful.
	handleScanErr := func(err error) {
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("openai messages stream: read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
		}
	}
	processFrame := func(frame openAICompatSSEFrame) bool {
		payload := openAICompatPayloadWithEventType(frame.Data, frame.EventType)
		if strings.TrimSpace(payload) == "[DONE]" {
			return false
		}
		return processDataLine(payload)
	}

	// ── Determine keepalive interval ──
	keepaliveInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamKeepaliveInterval > 0 {
		keepaliveInterval = time.Duration(s.cfg.Gateway.StreamKeepaliveInterval) * time.Second
	}

	// ── No keepalive: fast synchronous path (no goroutine overhead) ──
	if keepaliveInterval <= 0 {
		var parser openAICompatSSEFrameParser
		for scanner.Scan() {
			line := scanner.Text()
			frame, ok := parser.AddLine(line)
			if !ok {
				continue
			}
			if processFrame(frame) {
				return finalizeStream()
			}
		}
		if frame, ok := parser.Finish(); ok {
			if processFrame(frame) {
				return finalizeStream()
			}
		}
		handleScanErr(scanner.Err())
		return finalizeStream()
	}

	// ── With keepalive: goroutine + channel + select ──
	type scanEvent struct {
		line string
		err  error
	}
	events := make(chan scanEvent, 16)
	done := make(chan struct{})
	sendEvent := func(ev scanEvent) bool {
		select {
		case events <- ev:
			return true
		case <-done:
			return false
		}
	}
	go func() {
		defer close(events)
		for scanner.Scan() {
			if !sendEvent(scanEvent{line: scanner.Text()}) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = sendEvent(scanEvent{err: err})
		}
	}()
	defer close(done)

	keepaliveTicker := time.NewTicker(keepaliveInterval)
	defer keepaliveTicker.Stop()
	lastDataAt := time.Now()
	var parser openAICompatSSEFrameParser

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				// Upstream closed
				if frame, ok := parser.Finish(); ok {
					if processFrame(frame) {
						return finalizeStream()
					}
				}
				return finalizeStream()
			}
			if ev.err != nil {
				handleScanErr(ev.err)
				return finalizeStream()
			}
			lastDataAt = time.Now()
			line := ev.line
			frame, ok := parser.AddLine(line)
			if !ok {
				continue
			}
			if processFrame(frame) {
				return finalizeStream()
			}

		case <-keepaliveTicker.C:
			if time.Since(lastDataAt) < keepaliveInterval {
				continue
			}
			// Send Anthropic-format ping event
			if clientDisconnected {
				continue
			}
			if _, err := fmt.Fprint(c.Writer, "event: ping\ndata: {\"type\":\"ping\"}\n\n"); err != nil {
				clientDisconnected = true
				// Client disconnected
				logger.L().Info("openai messages stream: client disconnected during keepalive",
					zap.String("request_id", requestID),
				)
			}
			if !clientDisconnected {
				c.Writer.Flush()
			}
		}
	}
}

func isOpenAICompatResponsesTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.completed", "response.done", "response.incomplete", "response.failed", "response.cancelled", "response.canceled":
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) recordOpenAIMessagesStreamUpstreamError(c *gin.Context, account *Account, upstreamRequestID, kind, message string) {
	if c == nil {
		return
	}
	message = sanitizeUpstreamErrorMessage(message)
	setOpsUpstreamError(c, http.StatusBadGateway, message, "")
	event := OpsUpstreamErrorEvent{
		Platform:           PlatformOpenAI,
		UpstreamStatusCode: http.StatusBadGateway,
		UpstreamRequestID:  strings.TrimSpace(upstreamRequestID),
		Kind:               kind,
		Message:            message,
	}
	if account != nil {
		event.Platform = account.Platform
		event.AccountID = account.ID
		event.AccountName = account.Name
	}
	appendOpsUpstreamError(c, event)
}

func isOpenAICompatDoneSentinelLine(line string) bool {
	payload, ok := extractOpenAISSEDataLine(line)
	return ok && strings.TrimSpace(payload) == "[DONE]"
}

func (s *OpenAIGatewayService) readOpenAICompatBufferedTerminal(
	ctx context.Context,
	resp *http.Response,
	logPrefix string,
	requestID string,
	account *Account,
) (*apicompat.ResponsesResponse, OpenAIUsage, *apicompat.BufferedResponseAccumulator, error) {
	acc := apicompat.NewBufferedResponseAccumulator()
	var usage OpenAIUsage
	if resp == nil || resp.Body == nil {
		return nil, usage, acc, errors.New("upstream response body is required")
	}

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	streamInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamDataIntervalTimeout > 0 {
		streamInterval = time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout) * time.Second
	}
	var timeoutCh <-chan time.Time
	var timeoutTimer *time.Timer
	resetTimeout := func() {
		if streamInterval <= 0 {
			return
		}
		if timeoutTimer == nil {
			timeoutTimer = time.NewTimer(streamInterval)
			timeoutCh = timeoutTimer.C
			return
		}
		if !timeoutTimer.Stop() {
			select {
			case <-timeoutTimer.C:
			default:
			}
		}
		timeoutTimer.Reset(streamInterval)
	}
	stopTimeout := func() {
		if timeoutTimer == nil {
			return
		}
		if !timeoutTimer.Stop() {
			select {
			case <-timeoutTimer.C:
			default:
			}
		}
	}
	resetTimeout()
	defer stopTimeout()

	type scanEvent struct {
		line string
		err  error
	}
	events := make(chan scanEvent, 16)
	done := make(chan struct{})
	go func() {
		defer close(events)
		for scanner.Scan() {
			select {
			case events <- scanEvent{line: scanner.Text()}:
			case <-done:
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case events <- scanEvent{err: err}:
			case <-done:
			}
		}
	}()
	defer close(done)

	var parser openAICompatSSEFrameParser
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				if frame, ok := parser.Finish(); ok {
					payload := openAICompatPayloadWithEventType(frame.Data, frame.EventType)
					var event apicompat.ResponsesStreamEvent
					if err := json.Unmarshal([]byte(payload), &event); err == nil {
						acc.ProcessEvent(&event)
						if isOpenAICompatResponsesTerminalEvent(event.Type) && event.Response != nil {
							if event.Usage != nil {
								usage = copyOpenAIUsageFromResponsesUsage(event.Usage)
								if event.Response.Usage == nil {
									event.Response.Usage = event.Usage
								}
							}
							if event.Response.Usage != nil {
								usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
							}
							return event.Response, usage, acc, nil
						}
					}
				}
				return nil, usage, acc, nil
			}
			resetTimeout()
			if ev.err != nil {
				if !errors.Is(ev.err, context.Canceled) && !errors.Is(ev.err, context.DeadlineExceeded) {
					logger.L().Warn(logPrefix+": read error",
						zap.Error(ev.err),
						zap.String("request_id", requestID),
					)
				}
				return nil, usage, acc, ev.err
			}
			if isOpenAICompatDoneSentinelLine(ev.line) {
				return nil, usage, acc, nil
			}
			frame, ok := parser.AddLine(ev.line)
			if !ok {
				continue
			}
			payload := openAICompatPayloadWithEventType(frame.Data, frame.EventType)

			var event apicompat.ResponsesStreamEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				logger.L().Warn(logPrefix+": failed to parse event",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				continue
			}

			acc.ProcessEvent(&event)
			if isOpenAICompatResponsesTerminalEvent(event.Type) && event.Response != nil {
				if event.Usage != nil {
					usage = copyOpenAIUsageFromResponsesUsage(event.Usage)
					if event.Response.Usage == nil {
						event.Response.Usage = event.Usage
					}
				}
				if event.Response.Usage != nil {
					usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
				}
				return event.Response, usage, acc, nil
			}

		case <-timeoutCh:
			_ = resp.Body.Close()
			logger.L().Warn(logPrefix+": data interval timeout",
				zap.String("request_id", requestID),
				zap.Duration("interval", streamInterval),
			)
			return nil, usage, acc, fmt.Errorf("stream data interval timeout")
		}
	}
}

// writeAnthropicError writes an error response in Anthropic Messages API format.
func writeAnthropicError(c *gin.Context, statusCode int, errType, message string) {
	c.JSON(statusCode, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

func copyOpenAIUsageFromResponsesUsage(usage *apicompat.ResponsesUsage) OpenAIUsage {
	if usage == nil {
		return OpenAIUsage{}
	}
	result := OpenAIUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
	}
	if usage.InputTokensDetails != nil {
		result.CacheReadInputTokens = usage.InputTokensDetails.CachedTokens
	}
	return result
}

func anthropicToolNameMap(tools []apicompat.AnthropicTool) map[string]string {
	nameMap := make(map[string]string, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		canonical := strings.ToLower(strings.TrimLeft(strings.TrimSpace(tool.Name), "_"))
		if canonical == "" {
			continue
		}
		if _, exists := nameMap[canonical]; exists {
			continue
		}
		nameMap[canonical] = tool.Name
	}
	return nameMap
}

func stripAnthropicBillingHeaderFromSystem(req *apicompat.AnthropicRequest) {
	if req == nil || len(req.System) == 0 {
		return
	}

	if cleaned, ok := stripAnthropicBillingHeaderFromRawSystem(req.System); ok {
		req.System = cleaned
	}
}

func stripAnthropicBillingHeaderFromRawSystem(raw json.RawMessage) (json.RawMessage, bool) {
	var systemText string
	if err := json.Unmarshal(raw, &systemText); err == nil {
		cleaned, changed := stripAnthropicBillingHeaderText(systemText)
		if !changed {
			return raw, false
		}
		encoded, err := json.Marshal(cleaned)
		if err != nil {
			return raw, false
		}
		return encoded, true
	}

	var blocks []apicompat.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return raw, false
	}

	filtered := make([]apicompat.AnthropicContentBlock, 0, len(blocks))
	changed := false
	for _, block := range blocks {
		if block.Type == "text" {
			cleaned, textChanged := stripAnthropicBillingHeaderText(block.Text)
			if textChanged {
				changed = true
				block.Text = cleaned
			}
			if textChanged && strings.TrimSpace(block.Text) == "" {
				continue
			}
		}
		filtered = append(filtered, block)
	}
	if !changed {
		return raw, false
	}

	encoded, err := json.Marshal(filtered)
	if err != nil {
		return raw, false
	}
	return encoded, true
}

func stripAnthropicBillingHeaderText(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "x-anthropic-billing-header:") {
			changed = true
			continue
		}
		filtered = append(filtered, line)
	}
	if !changed {
		return text, false
	}
	return strings.TrimSpace(strings.Join(filtered, "\n")), true
}
