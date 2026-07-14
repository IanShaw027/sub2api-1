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
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// ForwardAsResponses accepts an OpenAI Responses API request body, converts it
// to Anthropic Messages format, forwards to the Anthropic upstream, and converts
// the response back to Responses format. This enables OpenAI Responses API
// clients to access Anthropic models through Anthropic platform groups.
//
// The method follows the same pattern as OpenAIGatewayService.ForwardAsAnthropic
// but in reverse direction: Responses → Anthropic upstream → Responses.
func (s *GatewayService) ForwardAsResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	clearClaudeResponseRewriteContext(c)

	if shouldAutoRouteOpenAICompatCCUpstream(account) {
		return s.forwardResponsesToOpenAICompatCC(ctx, c, account, body)
	}

	_ = parsed
	startTime := time.Now()

	ingressPlan, err := prepareResponsesAnthropicIngress(body)
	if err != nil {
		return nil, err
	}

	type responsesAnthropicAttempt struct {
		requestBody           []byte
		anthropicBody         []byte
		originalModel         string
		mappedModel           string
		clientStream          bool
		reasoningEffort       *string
		shouldMimicClaudeCode bool
	}

	buildAttempt := func(requestBody []byte) (*responsesAnthropicAttempt, error) {
		var responsesReq apicompat.ResponsesRequest
		if err := json.Unmarshal(requestBody, &responsesReq); err != nil {
			return nil, fmt.Errorf("parse responses request: %w", err)
		}

		anthropicReq, err := apicompat.ResponsesToAnthropicRequest(&responsesReq)
		if err != nil {
			return nil, fmt.Errorf("convert responses to anthropic: %w", err)
		}

		anthropicReq.Stream = true
		originalModel := responsesReq.Model
		mappedModel, _ := resolveGatewayAnthropicForwardModel(ctx, s.settingService, account, originalModel)
		anthropicReq.Model = mappedModel

		logger.L().Debug("gateway forward_as_responses: model mapping applied",
			zap.Int64("account_id", account.ID),
			zap.String("original_model", originalModel),
			zap.String("mapped_model", mappedModel),
			zap.Bool("client_stream", responsesReq.Stream),
		)

		anthropicBody, err := json.Marshal(anthropicReq)
		if err != nil {
			return nil, fmt.Errorf("marshal anthropic request: %w", err)
		}

		isClaudeCode := IsClaudeCodeClient(ctx)
		if !isClaudeCode && c != nil && c.Request != nil {
			isClaudeCode = IsClaudeCodeClient(c.Request.Context())
		}
		shouldMimicClaudeCode := s.shouldMimicClaudeCodeForAccount(ctx, account, isClaudeCode) && account.Platform != PlatformKiro
		if shouldMimicClaudeCode {
			anthropicBody = s.applyClaudeCodeOAuthMimicryToBody(ctx, c, account, anthropicBody, anthropicReq.System, mappedModel)
		}
		anthropicBody = enforceCacheControlLimit(anthropicBody)

		return &responsesAnthropicAttempt{
			requestBody:           requestBody,
			anthropicBody:         anthropicBody,
			originalModel:         originalModel,
			mappedModel:           mappedModel,
			clientStream:          responsesReq.Stream,
			reasoningEffort:       ExtractResponsesReasoningEffortFromBody(requestBody),
			shouldMimicClaudeCode: shouldMimicClaudeCode,
		}, nil
	}

	// 1. Get access token
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// 2. Get proxy URL
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	// 3. Build upstream request(s) and send, with at most one full-replay retry.
	reqStream := true
	currentBody := ingressPlan.PrimaryBody
	fullReplayRetried := false

	var (
		attempt     *responsesAnthropicAttempt
		resp        *http.Response
		upstreamReq *http.Request
	)

	for {
		attempt, err = buildAttempt(currentBody)
		if err != nil {
			return nil, err
		}

		if account != nil && account.Platform == PlatformKiro {
			kiroResp, kiroResult, err := s.forwardKiroAnthropicCapture(ctx, c, account, currentBody, attempt.anthropicBody, attempt.originalModel)
			if err != nil {
				var failoverErr *UpstreamFailoverError
				if errors.As(err, &failoverErr) {
					return nil, err
				}
				msg := sanitizeUpstreamErrorMessage(err.Error())
				if msg == "" {
					msg = "Kiro upstream request failed"
				}
				writeResponsesError(c, http.StatusBadGateway, "server_error", msg)
				return nil, err
			}
			if attempt.clientStream {
				return s.handleResponsesStreamingResponse(kiroResp, c, attempt.originalModel, kiroUpstreamModel(kiroResult, attempt.originalModel), attempt.reasoningEffort, startTime)
			}
			return s.handleResponsesBufferedStreamingResponse(kiroResp, c, attempt.originalModel, kiroUpstreamModel(kiroResult, attempt.originalModel), attempt.reasoningEffort, startTime)
		}

		upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, reqStream)
		var upstreamWireBody []byte
		upstreamReq, upstreamWireBody, err = s.buildUpstreamRequest(upstreamCtx, c, account, attempt.anthropicBody, token, tokenType, attempt.mappedModel, reqStream, attempt.shouldMimicClaudeCode)
		releaseUpstreamCtx()
		if err != nil {
			return nil, fmt.Errorf("build upstream request: %w", err)
		}
		setOpsUpstreamRequestBody(c, upstreamWireBody)

		upstreamStart := time.Now()
		resp, err = s.doGatewayRequestWithTLS(ctx, c, upstreamReq, account, proxyURL)
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		if err != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			safeErr := sanitizeUpstreamErrorMessage(err.Error())
			detail := recordDetailedUpstreamTransportError(c, err)
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: 0,
				UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
				Kind:               "request_error",
				Message:            safeErr,
			})
			body, _ := json.Marshal(gin.H{
				"error": gin.H{
					"type":    detail.ErrorType,
					"message": formatUpstreamRequestFailed(detail, "Upstream request failed"),
				},
			})
			return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway, ResponseBody: body}
		}

		if resp.StatusCode < 400 {
			break
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		failure := classifyResponsesAnthropicFailure(resp.StatusCode, respBody)
		if !fullReplayRetried && failure.RetryWithFullReplay && ingressPlan.CanRetryWithFullReplayForReason(failure.Reason) {
			fullReplayRetried = true
			currentBody = ingressPlan.FullReplayBody
			logger.L().Info("gateway forward_as_responses: retrying once with full replay input",
				zap.Int64("account_id", account.ID),
				zap.String("reason", failure.Reason),
				zap.String("source", ingressPlan.FullReplaySource),
			)
			continue
		}

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		upstreamDetail := ""
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
			if maxBytes <= 0 {
				maxBytes = 2048
			}
			upstreamDetail = truncateString(string(respBody), maxBytes)
		}

		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  upstreamRequestIDFromHeader(resp.Header),
				UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
				Kind:               "failover",
				Message:            upstreamMsg,
				Detail:             upstreamDetail,
			})
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, attempt.mappedModel)
			}
			return nil, &UpstreamFailoverError{
				StatusCode:   resp.StatusCode,
				ResponseBody: respBody,
			}
		}

		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  upstreamRequestIDFromHeader(resp.Header),
			UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		writeResponsesError(c, mapUpstreamStatusCode(resp.StatusCode), "server_error", upstreamMsg)
		return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
	}
	defer func() { _ = resp.Body.Close() }()

	// 4. Handle normal response (convert Anthropic → Responses)
	var result *ForwardResult
	var handleErr error
	if attempt.clientStream {
		result, handleErr = s.handleResponsesStreamingResponse(resp, c, attempt.originalModel, attempt.mappedModel, attempt.reasoningEffort, startTime)
	} else {
		result, handleErr = s.handleResponsesBufferedStreamingResponse(resp, c, attempt.originalModel, attempt.mappedModel, attempt.reasoningEffort, startTime)
	}

	return result, handleErr
}

// ExtractResponsesReasoningEffortFromBody reads Responses API reasoning.effort
// and normalizes it for usage logging.
func ExtractResponsesReasoningEffortFromBody(body []byte) *string {
	raw := strings.TrimSpace(gjson.GetBytes(body, "reasoning.effort").String())
	if raw == "" {
		return nil
	}
	normalized := normalizeOpenAIReasoningEffort(raw)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func mergeAnthropicUsage(dst *ClaudeUsage, src apicompat.AnthropicUsage) {
	if dst == nil {
		return
	}
	if src.InputTokens > 0 {
		dst.InputTokens = src.InputTokens
	}
	if src.OutputTokens > 0 {
		dst.OutputTokens = src.OutputTokens
	}
	if src.CacheReadInputTokens > 0 {
		dst.CacheReadInputTokens = src.CacheReadInputTokens
	}
	if src.CacheCreationInputTokens > 0 {
		dst.CacheCreationInputTokens = src.CacheCreationInputTokens
	}
}

// handleResponsesBufferedStreamingResponse reads all Anthropic SSE events from
// the upstream streaming response, assembles them into a complete Anthropic
// response, converts to Responses API JSON format, and writes it to the client.
func (s *GatewayService) handleResponsesBufferedStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	mappedModel string,
	reasoningEffort *string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	// Accumulate the final Anthropic response from streaming events
	var finalResp *apicompat.AnthropicResponse
	var usage ClaudeUsage
	sawMessageStop := false

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "event: ") {
			continue
		}
		eventType := strings.TrimPrefix(line, "event: ")

		// Read the data line
		if !scanner.Scan() {
			break
		}
		dataLine := scanner.Text()
		if !strings.HasPrefix(dataLine, "data: ") {
			continue
		}
		payload := dataLine[6:]

		var event apicompat.AnthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			logger.L().Warn("forward_as_responses buffered: failed to parse event",
				zap.Error(err),
				zap.String("request_id", requestID),
				zap.String("event_type", eventType),
			)
			continue
		}
		if event.Type == "message_stop" {
			sawMessageStop = true
		}

		// message_start carries the initial response structure
		if event.Type == "message_start" && event.Message != nil {
			finalResp = event.Message
			mergeAnthropicUsage(&usage, event.Message.Usage)
		}

		// message_delta carries final usage and stop_reason
		if event.Type == "message_delta" {
			if event.Usage != nil {
				mergeAnthropicUsage(&usage, *event.Usage)
			}
			if event.Delta != nil && event.Delta.StopReason != "" && finalResp != nil {
				finalResp.StopReason = event.Delta.StopReason
			}
		}

		// Accumulate content blocks
		if event.Type == "content_block_start" && event.ContentBlock != nil && finalResp != nil {
			finalResp.Content = append(finalResp.Content, *event.ContentBlock)
		}
		if event.Type == "content_block_delta" && event.Delta != nil && finalResp != nil && event.Index != nil {
			idx := *event.Index
			if idx < len(finalResp.Content) {
				switch event.Delta.Type {
				case "text_delta":
					finalResp.Content[idx].Text += event.Delta.Text
				case "thinking_delta":
					finalResp.Content[idx].Thinking += event.Delta.Thinking
				case "input_json_delta":
					finalResp.Content[idx].Input = appendRawJSON(finalResp.Content[idx].Input, event.Delta.PartialJSON)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("forward_as_responses buffered: read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			if !sawMessageStop {
				writeResponsesError(c, http.StatusBadGateway, "stream_read_error", "Upstream stream ended before a terminal event")
				return nil, fmt.Errorf("upstream stream read error before terminal event: %w", err)
			}
		}
	}

	if finalResp == nil {
		writeResponsesError(c, http.StatusBadGateway, "server_error", "Upstream stream ended without a response")
		return nil, fmt.Errorf("upstream stream ended without response")
	}
	if !sawMessageStop {
		writeResponsesError(c, http.StatusBadGateway, "stream_read_error", "Upstream stream ended before a terminal event")
		return nil, fmt.Errorf("upstream stream ended before terminal event")
	}

	// Update usage from accumulated delta
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		finalResp.Usage = apicompat.AnthropicUsage{
			InputTokens:              usage.InputTokens,
			OutputTokens:             usage.OutputTokens,
			CacheCreationInputTokens: usage.CacheCreationInputTokens,
			CacheReadInputTokens:     usage.CacheReadInputTokens,
		}
	}

	// Convert to Responses format
	responsesResp := apicompat.AnthropicToResponsesResponse(finalResp)
	responsesResp.Model = originalModel // Use original model name

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	// 非流式响应必须是 application/json。上游被强制流式后会返回
	// Content-Type: text/event-stream，经 WriteFilteredHeaders 透传后会污染
	// 响应头；而 c.Data/c.JSON 走 Gin 的 writeContentType（仅当头不存在时才设置），
	// 无法覆盖已存在的 SSE 头。这里显式 Set 强制改回 JSON，避免下游中间层
	// （如 new-api）按 Content-Type 误判为流式。
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if respBytes, err := json.Marshal(responsesResp); err == nil {
		respBytes = reverseToolNamesIfPresent(c, respBytes)
		respBytes = reverseWorkDirIfPresent(c, respBytes)
		c.Data(http.StatusOK, "application/json; charset=utf-8", respBytes)
	} else {
		c.JSON(http.StatusOK, responsesResp)
	}

	return &ForwardResult{
		RequestID:       requestID,
		Usage:           usage,
		Model:           originalModel,
		UpstreamModel:   mappedModel,
		ReasoningEffort: reasoningEffort,
		Stream:          false,
		Duration:        time.Since(startTime),
	}, nil
}

// handleResponsesStreamingResponse reads Anthropic SSE events from upstream,
// converts each to Responses SSE events, and writes them to the client.
func (s *GatewayService) handleResponsesStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	mappedModel string,
	reasoningEffort *string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	state := apicompat.NewAnthropicEventToResponsesState()
	state.Model = originalModel
	var usage ClaudeUsage
	var firstTokenMs *int
	firstChunk := true
	sawMessageStop := false

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	resultWithUsage := func() *ForwardResult {
		return &ForwardResult{
			RequestID:       requestID,
			Usage:           usage,
			Model:           originalModel,
			UpstreamModel:   mappedModel,
			ReasoningEffort: reasoningEffort,
			Stream:          true,
			Duration:        time.Since(startTime),
			FirstTokenMs:    firstTokenMs,
		}
	}

	// processEvent handles a single parsed Anthropic SSE event.
	processEvent := func(event *apicompat.AnthropicStreamEvent) bool {
		if firstChunk {
			firstChunk = false
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		// Extract usage from message_delta
		if event.Type == "message_delta" && event.Usage != nil {
			mergeAnthropicUsage(&usage, *event.Usage)
		}
		// Also capture usage from message_start
		if event.Type == "message_start" && event.Message != nil {
			mergeAnthropicUsage(&usage, event.Message.Usage)
		}
		if event.Type == "message_stop" {
			sawMessageStop = true
		}

		// Convert to Responses events
		events := apicompat.AnthropicEventToResponsesEvents(event, state)
		for _, evt := range events {
			sse, err := apicompat.ResponsesEventToSSE(evt)
			if err != nil {
				logger.L().Warn("forward_as_responses stream: failed to marshal event",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				continue
			}
			outBytes := reverseToolNamesIfPresent(c, []byte(sse))
			outBytes = reverseWorkDirIfPresent(c, outBytes)
			out := string(outBytes)
			if _, err := fmt.Fprint(c.Writer, out); err != nil {
				logger.L().Info("forward_as_responses stream: client disconnected",
					zap.String("request_id", requestID),
				)
				return true // client disconnected
			}
		}
		if len(events) > 0 {
			c.Writer.Flush()
		}
		return false
	}

	finalizeStream := func() (*ForwardResult, error) {
		if finalEvents := apicompat.FinalizeAnthropicResponsesStream(state); len(finalEvents) > 0 {
			for _, evt := range finalEvents {
				sse, err := apicompat.ResponsesEventToSSE(evt)
				if err != nil {
					continue
				}
				outBytes := reverseToolNamesIfPresent(c, []byte(sse))
				outBytes = reverseWorkDirIfPresent(c, outBytes)
				out := string(outBytes)
				fmt.Fprint(c.Writer, out) //nolint:errcheck
			}
			c.Writer.Flush()
		}
		return resultWithUsage(), nil
	}

	// Read Anthropic SSE events
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "event: ") {
			continue
		}
		eventType := strings.TrimPrefix(line, "event: ")

		// Read data line
		if !scanner.Scan() {
			break
		}
		dataLine := scanner.Text()
		if !strings.HasPrefix(dataLine, "data: ") {
			continue
		}
		payload := dataLine[6:]

		var event apicompat.AnthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			logger.L().Warn("forward_as_responses stream: failed to parse event",
				zap.Error(err),
				zap.String("request_id", requestID),
				zap.String("event_type", eventType),
			)
			continue
		}

		if processEvent(&event) {
			return resultWithUsage(), nil
		}
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("forward_as_responses stream: read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			if !sawMessageStop {
				if writeErr := writeOpenAIResponsesFailedSSE(c.Writer, state.ResponseID, originalModel, "stream_read_error", "Upstream stream ended before a terminal event"); writeErr != nil {
					return nil, fmt.Errorf("upstream stream read error before terminal event: %w; failed to write response.failed: %v", err, writeErr)
				}
				c.Writer.Flush()
				MarkResponseCommitted(c)
				return nil, fmt.Errorf("upstream stream read error before terminal event: %w", err)
			}
		}
	}
	if !sawMessageStop {
		if writeErr := writeOpenAIResponsesFailedSSE(c.Writer, state.ResponseID, originalModel, "stream_read_error", "Upstream stream ended before a terminal event"); writeErr != nil {
			return nil, fmt.Errorf("upstream stream ended before terminal event; failed to write response.failed: %v", writeErr)
		}
		c.Writer.Flush()
		MarkResponseCommitted(c)
		return nil, fmt.Errorf("upstream stream ended before terminal event")
	}

	return finalizeStream()
}

// appendRawJSON appends a JSON fragment string to existing raw JSON.
func appendRawJSON(existing json.RawMessage, fragment string) json.RawMessage {
	trimmed := strings.TrimSpace(string(existing))
	if len(existing) == 0 || trimmed == "{}" {
		return json.RawMessage(fragment)
	}
	return json.RawMessage(string(existing) + fragment)
}

// writeResponsesError writes an error response in OpenAI Responses API format.
func writeResponsesError(c *gin.Context, statusCode int, code, message string) {
	MarkResponseCommitted(c)
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}

// mapUpstreamStatusCode maps upstream HTTP status codes to appropriate client-facing codes.
func mapUpstreamStatusCode(code int) int {
	if code >= 500 {
		return http.StatusBadGateway
	}
	return code
}
