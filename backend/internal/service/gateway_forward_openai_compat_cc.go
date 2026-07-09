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
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func shouldAutoRouteOpenAICompatCCUpstream(account *Account) bool {
	return account != nil && account.IsOpenAICompatCCUpstream() && account.TextEndpointAutoRouteEnabled()
}

func (s *GatewayService) forwardChatCompletionsToOpenAICompatCC(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*ForwardResult, error) {
	startTime := time.Now()

	var ccReq apicompat.ChatCompletionsRequest
	if err := json.Unmarshal(body, &ccReq); err != nil {
		writeGatewayCCError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, fmt.Errorf("parse chat completions request: %w", err)
	}
	originalModel := strings.TrimSpace(ccReq.Model)
	if originalModel == "" {
		writeGatewayCCError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}
	clientStream := ccReq.Stream
	reasoningEffort := extractCCReasoningEffortFromBody(body)

	mappedModel, _ := resolveGatewayAnthropicForwardModel(ctx, s.settingService, account, originalModel)
	upstreamBody := body
	if mappedModel != "" && mappedModel != originalModel {
		upstreamBody = ReplaceModelInBody(upstreamBody, mappedModel)
	}
	var err error
	upstreamBody, err = normalizeRawChatCompletionsCompatBody(upstreamBody)
	if err != nil {
		return nil, fmt.Errorf("normalize raw chat compat body: %w", err)
	}
	if clientStream {
		upstreamBody, err = ensureOpenAIChatStreamUsage(upstreamBody)
		if err != nil {
			return nil, fmt.Errorf("enable stream usage: %w", err)
		}
	}

	resp, upstreamReq, err := s.sendOpenAICompatCCChatRequest(ctx, c, account, upstreamBody, mappedModel, clientStream)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return s.handleOpenAICompatCCChatError(ctx, c, account, resp, upstreamReq, mappedModel, writeGatewayCCError)
	}

	if clientStream {
		return s.streamOpenAICompatCCRawChat(c, resp, originalModel, mappedModel, reasoningEffort, startTime)
	}
	return s.bufferOpenAICompatCCRawChat(c, resp, originalModel, mappedModel, reasoningEffort, startTime)
}

func (s *GatewayService) forwardResponsesToOpenAICompatCC(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*ForwardResult, error) {
	startTime := time.Now()

	var responsesReq apicompat.ResponsesRequest
	if err := json.Unmarshal(body, &responsesReq); err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, fmt.Errorf("parse responses request: %w", err)
	}
	originalModel := strings.TrimSpace(responsesReq.Model)
	if originalModel == "" {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}
	clientStream := responsesReq.Stream
	reasoningEffort := ExtractResponsesReasoningEffortFromBody(body)

	chatReq, err := apicompat.ResponsesToChatCompletionsRequest(&responsesReq)
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, fmt.Errorf("convert responses to chat completions: %w", err)
	}
	mappedModel, _ := resolveGatewayAnthropicForwardModel(ctx, s.settingService, account, originalModel)
	if mappedModel == "" {
		mappedModel = originalModel
	}
	chatReq.Model = mappedModel
	if clientStream {
		chatReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}
	}
	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions request: %w", err)
	}

	resp, upstreamReq, err := s.sendOpenAICompatCCChatRequest(ctx, c, account, chatBody, mappedModel, clientStream)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return s.handleOpenAICompatCCResponsesError(ctx, c, account, resp, upstreamReq, mappedModel)
	}

	if clientStream {
		return s.streamOpenAICompatCCChatAsResponses(c, resp, originalModel, mappedModel, reasoningEffort, startTime)
	}
	return s.bufferOpenAICompatCCChatAsResponses(c, resp, originalModel, mappedModel, reasoningEffort, startTime)
}

func (s *GatewayService) sendOpenAICompatCCChatRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	mappedModel string,
	stream bool,
) (*http.Response, *http.Request, error) {
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, nil, fmt.Errorf("get access token: %w", err)
	}
	if tokenType != "apikey" {
		return nil, nil, fmt.Errorf("openai_compat_cc_upstream requires apikey, got: %s", tokenType)
	}

	baseURL := account.GetBaseURL()
	if strings.TrimSpace(baseURL) == "" {
		return nil, nil, fmt.Errorf("account %d missing base_url for CC upstream", account.ID)
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid base_url: %w", err)
	}
	targetURL := buildOpenAIChatCompletionsURL(validatedURL)

	upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, stream)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	releaseUpstreamCtx()
	if err != nil {
		return nil, nil, fmt.Errorf("build upstream request: %w", err)
	}
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	if stream {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			if openaiCCRawAllowedHeaders[strings.ToLower(key)] {
				for _, value := range values {
					upstreamReq.Header.Add(key, value)
				}
			}
		}
	}
	setOpsUpstreamRequestBody(c, body)
	tlsProfile := s.resolveGatewayTLSProfile(account)
	upstreamReq = withOpenAIHTTP1RawHeaderReplay(upstreamReq, account, tlsProfile)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
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
		return nil, upstreamReq, &UpstreamFailoverError{StatusCode: http.StatusBadGateway, ResponseBody: body}
	}
	_ = mappedModel
	return resp, upstreamReq, nil
}

func (s *GatewayService) resolveGatewayTLSProfile(account *Account) *tlsfingerprint.Profile {
	if s == nil || s.tlsFPProfileService == nil {
		return nil
	}
	return s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http")
}

func (s *GatewayService) handleOpenAICompatCCChatError(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	upstreamReq *http.Request,
	mappedModel string,
	writeErr func(*gin.Context, int, string, string),
) (*ForwardResult, error) {
	respBody, _ := s.readUpstreamErrorBody(resp)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(respBody), maxBytes)
	}
	if markOpsCyberPolicyIfDetected(c, respBody, resp.StatusCode, 0, 0) {
		if upstreamMsg == "" {
			upstreamMsg = "Request blocked by upstream cyber-security policy"
		}
		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  upstreamRequestIDFromHeader(resp.Header),
			UpstreamURL:        safeHTTPRequestURL(upstreamReq),
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		writeErr(c, http.StatusBadRequest, "invalid_request_error", upstreamMsg)
		return nil, fmt.Errorf("openai cyber_policy: %s", upstreamMsg)
	}
	if s.shouldFailoverUpstreamError(resp.StatusCode) {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  upstreamRequestIDFromHeader(resp.Header),
			UpstreamURL:        safeHTTPRequestURL(upstreamReq),
			Kind:               "failover",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		if s.rateLimitService != nil {
			s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, mappedModel)
		}
		return nil, &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: respBody}
	}

	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  upstreamRequestIDFromHeader(resp.Header),
		UpstreamURL:        safeHTTPRequestURL(upstreamReq),
		Kind:               "http_error",
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})
	writeErr(c, mapUpstreamStatusCode(resp.StatusCode), "server_error", upstreamMsg)
	return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
}

func (s *GatewayService) handleOpenAICompatCCResponsesError(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	upstreamReq *http.Request,
	mappedModel string,
) (*ForwardResult, error) {
	return s.handleOpenAICompatCCChatError(ctx, c, account, resp, upstreamReq, mappedModel, writeResponsesError)
}

func (s *GatewayService) bufferOpenAICompatCCRawChat(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	mappedModel string,
	reasoningEffort *string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeGatewayCCError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, fmt.Errorf("read upstream body: %w", err)
	}

	usage := claudeUsageFromChatCompletionBody(respBody)
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Writer.Header().Set("Content-Type", ct)
	} else {
		c.Writer.Header().Set("Content-Type", "application/json")
	}
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(respBody)

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

func (s *GatewayService) bufferOpenAICompatCCChatAsResponses(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	mappedModel string,
	reasoningEffort *string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeResponsesError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, fmt.Errorf("read upstream body: %w", err)
	}

	var ccResp apicompat.ChatCompletionsResponse
	if err := json.Unmarshal(respBody, &ccResp); err != nil {
		writeResponsesError(c, http.StatusBadGateway, "api_error", "Failed to parse upstream response")
		return nil, fmt.Errorf("parse chat completions response: %w", err)
	}
	responsesResp := apicompat.ChatCompletionsResponseToResponses(&ccResp, originalModel)
	usage := claudeUsageFromChatUsage(ccResp.Usage)

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.JSON(http.StatusOK, responsesResp)

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

func (s *GatewayService) streamOpenAICompatCCRawChat(
	c *gin.Context,
	resp *http.Response,
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

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), s.gatewayMaxLineSize())
	var usage ClaudeUsage
	var firstTokenMs *int
	clientDisconnected := false

	for scanner.Scan() {
		line := scanner.Text()
		if payload, ok := extractOpenAISSEDataLine(line); ok {
			trimmed := strings.TrimSpace(payload)
			if trimmed != "" && trimmed != "[DONE]" {
				if u := extractCCStreamUsage(payload); u != nil {
					usage = claudeUsageFromOpenAIUsage(*u)
					_ = markOpsCyberPolicyIfDetected(c, []byte(payload), http.StatusOK, u.InputTokens, u.OutputTokens)
				} else {
					_ = markOpsCyberPolicyIfDetected(c, []byte(payload), http.StatusOK, 0, 0)
				}
				if firstTokenMs == nil && !isOpenAIChatUsageOnlyStreamChunk(payload) {
					elapsed := int(time.Since(startTime).Milliseconds())
					firstTokenMs = &elapsed
				}
			}
		}
		if !clientDisconnected {
			if _, err := c.Writer.WriteString(line + "\n"); err != nil {
				clientDisconnected = true
			}
			if line == "" {
				c.Writer.Flush()
			}
		}
	}

	return &ForwardResult{
		RequestID:        requestID,
		Usage:            usage,
		Model:            originalModel,
		UpstreamModel:    mappedModel,
		ReasoningEffort:  reasoningEffort,
		Stream:           true,
		Duration:         time.Since(startTime),
		FirstTokenMs:     firstTokenMs,
		ClientDisconnect: clientDisconnected,
	}, scanner.Err()
}

func (s *GatewayService) streamOpenAICompatCCChatAsResponses(
	c *gin.Context,
	resp *http.Response,
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

	state := apicompat.NewChatCompletionsToResponsesStreamState(originalModel)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), s.gatewayMaxLineSize())
	var usage ClaudeUsage
	var firstTokenMs *int
	clientDisconnected := false
	// sawTerminal 记录是否见到真正的终止信号（finish_reason 或 [DONE]）。上游无终止信号即 EOF 属截断，
	// 不能合成 response.completed+[DONE] 当成正常结束——否则客户端把截断响应当成功并按已收 token 计费。
	sawTerminal := false
	var upstreamStreamErr error

	writeEvents := func(events []apicompat.ResponsesStreamEvent) {
		if clientDisconnected {
			return
		}
		for _, event := range events {
			sse, err := apicompat.ResponsesEventToSSE(event)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				clientDisconnected = true
				return
			}
		}
		if len(events) > 0 {
			c.Writer.Flush()
		}
	}
	emitFailed := func(err error) {
		writeEvents([]apicompat.ResponsesStreamEvent{{
			Type: "response.failed",
			Response: &apicompat.ResponsesResponse{
				Model:  originalModel,
				Status: "failed",
				Error: &apicompat.ResponsesError{
					Code:    "upstream_error",
					Message: sanitizeUpstreamErrorMessage(err.Error()),
				},
			},
		}})
	}

	for scanner.Scan() {
		payload, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(payload)
		if trimmed == "" {
			continue
		}
		if trimmed == "[DONE]" {
			sawTerminal = true
			break
		}
		// 中途上游错误：CC/OpenAI 流式错误为 data: {"error":{...}}，反序列化成空 Choices 的 chunk 会被
		// 静默吞掉。显式探测并中断，透传上游错误、不再合成正常结束。
		if strings.HasPrefix(trimmed, "{") && gjson.Get(trimmed, "error").Exists() {
			upstreamStreamErr = fmt.Errorf("upstream stream error: %s", sanitizeUpstreamErrorMessage(gjson.Get(trimmed, "error.message").String()))
			break
		}
		if u := extractCCStreamUsage(payload); u != nil {
			usage = claudeUsageFromOpenAIUsage(*u)
			_ = markOpsCyberPolicyIfDetected(c, []byte(payload), http.StatusOK, u.InputTokens, u.OutputTokens)
		} else {
			_ = markOpsCyberPolicyIfDetected(c, []byte(payload), http.StatusOK, int(usage.InputTokens), int(usage.OutputTokens))
		}
		var chunk apicompat.ChatCompletionsChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			usage = claudeUsageFromChatUsage(chunk.Usage)
		}
		for _, choice := range chunk.Choices {
			if choice.FinishReason != nil {
				sawTerminal = true
			}
		}
		if firstTokenMs == nil && !isOpenAIChatUsageOnlyStreamChunk(payload) {
			elapsed := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &elapsed
		}
		writeEvents(apicompat.ChatCompletionsChunkToResponsesEvents(&chunk, state))
	}

	// 读错误 / 中途上游错误 / 无终止信号截断：都发 response.failed 事件并返回 error，
	// 绝不合成 response.completed+[DONE]（避免出错/截断响应被当成功计费）。
	if scanErr := scanner.Err(); scanErr != nil && !clientDisconnected {
		wrapped := fmt.Errorf("upstream stream read: %w", scanErr)
		emitFailed(wrapped)
		return nil, wrapped
	}
	if upstreamStreamErr != nil {
		emitFailed(upstreamStreamErr)
		return nil, upstreamStreamErr
	}
	if !sawTerminal && !clientDisconnected {
		err := errors.New("upstream stream ended before a terminal event")
		emitFailed(err)
		return nil, err
	}

	writeEvents(apicompat.FinalizeChatCompletionsResponsesStream(state))
	if !clientDisconnected {
		fmt.Fprint(c.Writer, "data: [DONE]\n\n") //nolint:errcheck
		c.Writer.Flush()
	}

	return &ForwardResult{
		RequestID:        requestID,
		Usage:            usage,
		Model:            originalModel,
		UpstreamModel:    mappedModel,
		ReasoningEffort:  reasoningEffort,
		Stream:           true,
		Duration:         time.Since(startTime),
		FirstTokenMs:     firstTokenMs,
		ClientDisconnect: clientDisconnected,
	}, nil
}

func (s *GatewayService) gatewayMaxLineSize() int {
	maxLineSize := defaultMaxLineSize
	if s != nil && s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	return maxLineSize
}

func claudeUsageFromChatCompletionBody(body []byte) ClaudeUsage {
	var ccResp apicompat.ChatCompletionsResponse
	if err := json.Unmarshal(body, &ccResp); err != nil {
		return ClaudeUsage{}
	}
	return claudeUsageFromChatUsage(ccResp.Usage)
}

func claudeUsageFromChatUsage(usage *apicompat.ChatUsage) ClaudeUsage {
	if usage == nil {
		return ClaudeUsage{}
	}
	cacheReadInputTokens := 0
	if usage.PromptTokensDetails != nil {
		cacheReadInputTokens = usage.PromptTokensDetails.CachedTokens
	}
	inputTokens := usage.PromptTokens - cacheReadInputTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	out := ClaudeUsage{
		InputTokens:          inputTokens,
		OutputTokens:         usage.CompletionTokens,
		CacheReadInputTokens: cacheReadInputTokens,
	}
	return out
}

func claudeUsageFromOpenAIUsage(usage OpenAIUsage) ClaudeUsage {
	inputTokens := usage.InputTokens - usage.CacheReadInputTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	return ClaudeUsage{
		InputTokens:          inputTokens,
		OutputTokens:         usage.OutputTokens,
		CacheReadInputTokens: usage.CacheReadInputTokens,
	}
}

func safeHTTPRequestURL(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	return safeUpstreamURL(req.URL.String())
}
