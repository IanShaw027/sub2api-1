package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// forwardMessagesToChatCompletions converts an incoming Anthropic Messages
// request to OpenAI Chat Completions format and forwards it to a CC-compatible
// upstream (e.g. opencode.ai GLM/Kimi/DeepSeek/MiMo endpoints).
// The upstream CC response is converted back to Anthropic Messages SSE events.
func (s *GatewayService) forwardMessagesToChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	startTime := time.Now()

	// 1. Parse the Anthropic Messages request
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(parsed.Body.Bytes(), &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	originalModel := anthropicReq.Model
	clientStream := anthropicReq.Stream

	// 2. Apply model mapping
	mappedModel, _ := resolveGatewayAnthropicForwardModel(ctx, s.settingService, account, originalModel)
	anthropicReq.Model = mappedModel

	// 3. Convert Anthropic Messages → Chat Completions
	anthropicReq.Stream = true // always stream to upstream for TTFT
	ccReq, err := apicompat.AnthropicRequestToChatCompletions(&anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("convert anthropic to chat completions: %w", err)
	}
	ccReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}

	ccBody, err := json.Marshal(ccReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions request: %w", err)
	}

	logger.L().Debug("gateway forward_messages_to_cc: converting and forwarding",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("mapped_model", mappedModel),
		zap.Bool("client_stream", clientStream),
	)

	// 4. Build upstream request
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	if tokenType != "apikey" {
		return nil, fmt.Errorf("openai_compat_cc_upstream requires apikey, got: %s", tokenType)
	}

	baseURL := account.GetBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("account %d missing base_url for CC upstream", account.ID)
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	targetURL := buildOpenAIChatCompletionsURL(validatedURL)

	upstreamCtx, releaseCtx := detachStreamUpstreamContext(ctx, true)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(ccBody))
	releaseCtx()
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	upstreamReq.Header.Set("Accept", "text/event-stream")

	// 5. Send request
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:    account.Platform,
			AccountID:   account.ID,
			AccountName: account.Name,
			Kind:        "request_error",
			Message:     safeErr,
		})
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	// 6. Handle error responses
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			return nil, &UpstreamFailoverError{
				StatusCode:   resp.StatusCode,
				ResponseBody: respBody,
			}
		}
		return s.handleErrorResponse(ctx, resp, c, account, originalModel)
	}

	// 7. Stream CC response back as Anthropic SSE events
	usage, firstTokenMs, clientDisconnect, err := s.streamCCResponseAsAnthropic(ctx, c, resp, mappedModel, clientStream, startTime)
	if err != nil {
		return nil, err
	}

	return &ForwardResult{
		RequestID:        resp.Header.Get("x-request-id"),
		Usage:            usage,
		Model:            originalModel,
		UpstreamModel:    mappedModel,
		Stream:           clientStream,
		Duration:         time.Since(startTime),
		FirstTokenMs:     firstTokenMs,
		ClientDisconnect: clientDisconnect,
	}, nil
}

// streamCCResponseAsAnthropic reads an upstream CC SSE stream and converts it to
// Anthropic Messages SSE events (streaming) or a single JSON response (non-streaming).
func (s *GatewayService) streamCCResponseAsAnthropic(
	ctx context.Context,
	c *gin.Context,
	resp *http.Response,
	model string,
	clientStream bool,
	startTime time.Time,
) (ClaudeUsage, *int, bool, error) {
	state := apicompat.NewChatChunkToAnthropicState(model)

	if clientStream {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
	}

	var firstTokenMs *int
	var lastStopReason string
	var finalUsage *apicompat.AnthropicUsage
	clientDisconnect := false
	var textBuf strings.Builder
	var thinkingBuf strings.Builder
	var toolUseBlocks []apicompat.AnthropicContentBlock

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		if ctx.Err() != nil {
			clientDisconnect = true
			break
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk apicompat.ChatCompletionsChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			finalUsage = apicompat.CcUsageToAnthropic(chunk.Usage)
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != nil {
				textBuf.WriteString(*choice.Delta.Content)
			}
			if reasoning := choice.Delta.EffectiveReasoningContent(); reasoning != "" {
				thinkingBuf.WriteString(reasoning)
			}
			if choice.FinishReason != nil {
				lastStopReason = *choice.FinishReason
			}
			for _, tc := range choice.Delta.ToolCalls {
				apicompat.AccumulateToolCall(state, &tc)
			}
		}

		if clientStream {
			events := apicompat.ChatChunkToAnthropicEvents(&chunk, state)
			for _, evt := range events {
				if firstTokenMs == nil && evt.Event == "content_block_delta" {
					ms := int(time.Since(startTime).Milliseconds())
					firstTokenMs = &ms
				}
				writeAnthropicSSE(c.Writer, evt.Event, evt.Data)
			}
			c.Writer.Flush()
		}
	}
	if scanErr := scanner.Err(); scanErr != nil && !clientDisconnect {
		return ClaudeUsage{}, firstTokenMs, false, fmt.Errorf("upstream stream read: %w", scanErr)
	}

	if clientStream {
		if lastStopReason == "" && !clientDisconnect {
			finishEvents := apicompat.FinalizeAnthropicStream(state, "end_turn")
			for _, evt := range finishEvents {
				writeAnthropicSSE(c.Writer, evt.Event, evt.Data)
			}
			c.Writer.Flush()
		}
	} else {
		toolUseBlocks = apicompat.BuildToolUseBlocks(state)
		stopReason := apicompat.CcFinishReasonToAnthropicStop(lastStopReason)
		anthropicResp := buildNonStreamingAnthropicResponse(state.MessageID, model, stopReason, textBuf.String(), thinkingBuf.String(), toolUseBlocks, finalUsage)
		c.JSON(http.StatusOK, anthropicResp)
	}

	usage := ClaudeUsage{}
	if finalUsage != nil {
		usage.InputTokens = finalUsage.InputTokens
		usage.OutputTokens = finalUsage.OutputTokens
		usage.CacheReadInputTokens = finalUsage.CacheReadInputTokens
		usage.CacheCreationInputTokens = finalUsage.CacheCreationInputTokens
	}

	return usage, firstTokenMs, clientDisconnect, nil
}

func buildNonStreamingAnthropicResponse(id, model, stopReason, text, thinking string, toolUseBlocks []apicompat.AnthropicContentBlock, usage *apicompat.AnthropicUsage) apicompat.AnthropicResponse {
	var content []apicompat.AnthropicContentBlock
	// Use text content if available; fall back to reasoning when content is empty
	// (model spent all tokens on reasoning without producing final output).
	effectiveText := text
	if effectiveText == "" && thinking != "" {
		effectiveText = thinking
	}
	if effectiveText != "" {
		content = append(content, apicompat.AnthropicContentBlock{Type: "text", Text: effectiveText})
	}
	content = append(content, toolUseBlocks...)

	resp := apicompat.AnthropicResponse{
		ID:         id,
		Type:       "message",
		Role:       "assistant",
		Content:    content,
		Model:      model,
		StopReason: stopReason,
	}
	if usage != nil {
		resp.Usage = *usage
	}
	return resp
}

func writeAnthropicSSE(w gin.ResponseWriter, event string, data []byte) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
}
