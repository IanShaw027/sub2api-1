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
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func isAnthropicResponsesTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.completed", "response.done", "response.incomplete", "response.failed", "response.cancelled", "response.canceled":
		return true
	default:
		return false
	}
}

func shouldApplyAnthropicCompatFullReplayGuard(account *Account, previousResponseID string, continuationEnabled bool, continuationDisabled bool, compatReplayGuardEnabled bool) bool {
	if !compatReplayGuardEnabled || continuationDisabled {
		return false
	}
	if account == nil || account.Type == AccountTypeOAuth {
		return false
	}
	if strings.TrimSpace(previousResponseID) != "" {
		return false
	}
	if continuationEnabled {
		return true
	}
	return true
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
	startTime := time.Now()
	clearOpenAICodexCompatContext(c)

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

	isStream := true
	forcedDispatchModel := getOpenAIMessagesDispatchForcedModel(c)
	billingModel := resolveOpenAIForwardModel(account, normalizedModel, defaultMappedModel)
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
	if shouldApplyAnthropicCompatFullReplayGuard(account, previousResponseID, compatContinuationEnabled, compatContinuationDisabled, compatReplayGuardEnabled) {
		applyAnthropicCompatFullReplayGuard(&anthropicReq)
	}

	// 2. Convert Anthropic → Responses
	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("convert anthropic to responses: %w", err)
	}
	if IsClaudeCodeClient(c.Request.Context()) {
		apicompat.AugmentClaudeToolDescriptions(responsesReq.Tools)
	}

	// Upstream always uses streaming (upstream may not support sync mode).
	// The client's original preference determines the response format.
	responsesReq.Stream = true

	// 2b. Handle BetaFastMode → service_tier: "priority"
	if containsBetaToken(c.GetHeader("anthropic-beta"), claude.BetaFastMode) {
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
	if account.Type == AccountTypeOAuth {
		var reqBody map[string]any
		if err := json.Unmarshal(responsesBody, &reqBody); err != nil {
			return nil, fmt.Errorf("unmarshal for codex transform: %w", err)
		}
		codexResult := applyCodexOAuthTransformWithInputMode(reqBody, false, false, codexTransformInputModePreservePrefix)
		if c != nil {
			c.Set(openAICodexTransformObsKey, codexResult.Observability)
		}
		forcedTemplateText := ""
		if s.cfg != nil {
			forcedTemplateText = s.cfg.Gateway.ForcedCodexInstructionsTemplate
		}
		if IsClaudeCodeClient(c.Request.Context()) &&
			strings.TrimSpace(forcedTemplateText) == "" &&
			applyEmbeddedDefaultInstructions(reqBody) {
			codexResult.Modified = true
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

	// For API key accounts (including OpenAI-compatible upstream gateways),
	// ensure promptCacheKey is also propagated via the request body so that
	// upstreams using the Responses API can derive a stable session identifier
	// from prompt_cache_key. This makes our Anthropic /v1/messages compatibility
	// path behave more like a native Responses client.
	if account.Type == AccountTypeAPIKey {
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

	// 5. Get access token
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// 6. Build upstream request
	upstreamReq, err := s.buildUpstreamRequest(upstreamCtx, c, account, responsesBody, token, isStream, promptCacheKey, isOpenAICodexOfficialClientRequest(c))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}

	// Override session_id with a deterministic UUID derived from the isolated
	// session key, ensuring different API keys produce different upstream sessions.
	if account.Type == AccountTypeAPIKey && promptCacheKey != "" {
		apiKeyID := getAPIKeyIDFromContext(c)
		upstreamReq.Header.Set("session_id", generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey)))
	}
	if (oauthTurnStateBridge || oauthDerivedSessionBridge) && promptCacheKey != "" {
		apiKeyID := getAPIKeyIDFromContext(c)
		upstreamReq.Header.Set("session_id", generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey)))
	}
	if oauthTurnStateBridge && strings.TrimSpace(oauthTurnState) != "" && upstreamReq.Header.Get("x-codex-turn-state") == "" {
		upstreamReq.Header.Set("x-codex-turn-state", strings.TrimSpace(oauthTurnState))
	}

	// 7. Send request
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	httpCodexCompatRetryTried := false
	var resp *http.Response
	for {
		SetOpsLatencyMs(c, OpsOpenAIForwardPrepareLatencyMsKey, time.Since(startTime).Milliseconds())
		upstreamStart := time.Now()
		resp, err = s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
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
				return nil, &UpstreamFailoverError{
					StatusCode:             resp.StatusCode,
					ResponseBody:           respBody,
					RetryableOnSameAccount: account.IsPoolMode() && (isPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody)),
				}
			}
			// Non-failover error: return Anthropic-formatted error to client
			return s.handleAnthropicErrorResponse(resp, c, account)
		}
		break
	}

	// 9. Handle normal response
	// Upstream is always streaming; choose response format based on client preference.
	var result *OpenAIForwardResult
	var handleErr error
	if clientStream {
		result, handleErr = s.handleAnthropicStreamingResponse(resp, c, originalModel, billingModel, upstreamModel, toolNameMap, startTime)
	} else {
		// Client wants JSON: buffer the streaming response and assemble a JSON reply.
		result, handleErr = s.handleAnthropicBufferedStreamingResponse(resp, c, originalModel, billingModel, upstreamModel, toolNameMap, startTime)
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

	// Extract and save Codex usage snapshot from response headers (for OAuth accounts)
	if handleErr == nil && account.Type == AccountTypeOAuth {
		if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
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
) (*OpenAIForwardResult, error) {
	return s.handleCompatErrorResponse(resp, c, account, writeAnthropicError)
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
	originalModel string,
	billingModel string,
	upstreamModel string,
	toolNameMap map[string]string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var finalResponse *apicompat.ResponsesResponse
	var usage OpenAIUsage
	acc := apicompat.NewBufferedResponseAccumulator()
	processFrame := func(frame openAICompatSSEFrame) bool {
		payload := openAICompatPayloadWithEventType(frame.Data, frame.EventType)

		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			logger.L().Warn("openai messages buffered: failed to parse event",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			return false
		}

		// Accumulate delta content for fallback when terminal output is empty.
		acc.ProcessEvent(&event)

		// Terminal events carry the complete ResponsesResponse with output + usage.
		if isAnthropicResponsesTerminalEvent(event.Type) && event.Response != nil {
			finalResponse = event.Response
			if event.Response.Usage != nil {
				usage = OpenAIUsage{
					InputTokens:  event.Response.Usage.InputTokens,
					OutputTokens: event.Response.Usage.OutputTokens,
				}
				if event.Response.Usage.InputTokensDetails != nil {
					usage.CacheReadInputTokens = event.Response.Usage.InputTokensDetails.CachedTokens
				}
			}
			return true
		}
		return false
	}

	var parser openAICompatSSEFrameParser
	for scanner.Scan() {
		line := scanner.Text()
		if isOpenAICompatDoneSentinelLine(line) {
			continue
		}
		frame, ok := parser.AddLine(line)
		if !ok {
			continue
		}
		if processFrame(frame) {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("openai messages buffered: read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
		}
	}
	if finalResponse == nil {
		if frame, ok := parser.Finish(); ok {
			processFrame(frame)
		}
	}

	if finalResponse == nil {
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Upstream stream ended without a terminal response event")
		return nil, fmt.Errorf("stream usage incomplete: missing terminal event")
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
	c.Writer.WriteHeader(http.StatusOK)

	state := apicompat.NewResponsesEventToAnthropicState()
	state.Model = originalModel
	state.ToolNameMap = toolNameMap
	var usage OpenAIUsage
	var responseID string
	var firstTokenMs *int
	firstChunk := true
	sawTerminalEvent := false
	clientDisconnected := false

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	// resultWithUsage builds the final result snapshot.
	resultWithUsage := func() *OpenAIForwardResult {
		return &OpenAIForwardResult{
			RequestID:     requestID,
			ResponseID:    strings.TrimSpace(responseID),
			Usage:         usage,
			Model:         originalModel,
			BillingModel:  billingModel,
			UpstreamModel: upstreamModel,
			Stream:        true,
			Duration:      time.Since(startTime),
			FirstTokenMs:  firstTokenMs,
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

		// Extract usage from completion events
		if isAnthropicResponsesTerminalEvent(event.Type) && event.Response != nil && event.Response.Usage != nil {
			usage = OpenAIUsage{
				InputTokens:  event.Response.Usage.InputTokens,
				OutputTokens: event.Response.Usage.OutputTokens,
			}
			if event.Response.Usage.InputTokensDetails != nil {
				usage.CacheReadInputTokens = event.Response.Usage.InputTokensDetails.CachedTokens
			}
		}
		if isAnthropicResponsesTerminalEvent(event.Type) {
			if event.Response != nil {
				responseID = strings.TrimSpace(event.Response.ID)
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
			}
		}
		if len(events) > 0 && !clientDisconnected {
			c.Writer.Flush()
		}
		return sawTerminalEvent
	}

	// finalizeStream sends any remaining Anthropic events and returns the result.
	finalizeStream := func() (*OpenAIForwardResult, error) {
		if !sawTerminalEvent {
			writeAnthropicError(c, http.StatusBadGateway, "api_error", "Upstream stream ended without a terminal response event")
			return resultWithUsage(), fmt.Errorf("stream usage incomplete: missing terminal event")
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
				fmt.Fprint(c.Writer, sse) //nolint:errcheck
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
			if isOpenAICompatDoneSentinelLine(line) {
				continue
			}
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
			if isOpenAICompatDoneSentinelLine(line) {
				continue
			}
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
