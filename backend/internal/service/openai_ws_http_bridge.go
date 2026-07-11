package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openAIWSClientReadLimitBytesDefault     int64 = 64 * 1024 * 1024
	openAIWSHTTPBridgeThresholdBytesDefault int64 = 15 * 1024 * 1024
	openAIWSHTTPBridgeErrorBodyLimitBytes         = 64 * 1024
)

func ResolveOpenAIWSClientReadLimitBytes(cfg *config.Config) int64 {
	if cfg == nil || cfg.Gateway.OpenAIWS.ClientReadLimitBytes <= 0 {
		return openAIWSClientReadLimitBytesDefault
	}
	return cfg.Gateway.OpenAIWS.ClientReadLimitBytes
}

func (s *OpenAIGatewayService) openAIWSHTTPBridgeEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.HTTPBridgeEnabled
}

func (s *OpenAIGatewayService) openAIWSHTTPBridgeThresholdBytes() int64 {
	if s == nil || s.cfg == nil || s.cfg.Gateway.OpenAIWS.HTTPBridgeThresholdBytes <= 0 {
		return openAIWSHTTPBridgeThresholdBytesDefault
	}
	return s.cfg.Gateway.OpenAIWS.HTTPBridgeThresholdBytes
}

func (s *OpenAIGatewayService) shouldBridgeOpenAIWSHTTP(account *Account, payloadBytes int, previousResponseID string) bool {
	if account != nil && account.Platform == PlatformGrok {
		return true
	}
	if !s.openAIWSHTTPBridgeEnabled() {
		return false
	}
	if strings.TrimSpace(previousResponseID) != "" {
		return false
	}
	threshold := s.openAIWSHTTPBridgeThresholdBytes()
	return threshold > 0 && int64(payloadBytes) >= threshold
}

func prepareOpenAIWSHTTPBridgeBody(payload []byte) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	if body == nil {
		return nil, errors.New("response.create payload must be a JSON object")
	}
	keepGenerateFalse := false
	if generate, ok := body["generate"].(bool); ok && !generate {
		keepGenerateFalse = true
	}
	delete(body, "type")
	delete(body, "generate")
	delete(body, "previous_response_id")
	if keepGenerateFalse {
		body["generate"] = false
	}
	body["stream"] = true
	return json.Marshal(body)
}

type openAIWSToolCallReplayCollector struct {
	items []json.RawMessage
	seen  map[string]struct{}
}

func (c *openAIWSToolCallReplayCollector) AddEvent(eventType string, message []byte) {
	switch strings.TrimSpace(eventType) {
	case "response.output_item.done":
		c.addItem(gjson.GetBytes(message, "item"))
	case "response.completed", "response.done":
		output := gjson.GetBytes(message, "response.output")
		if !output.IsArray() {
			return
		}
		for _, item := range output.Array() {
			c.addItem(item)
		}
	}
}

func (c *openAIWSToolCallReplayCollector) Items() []json.RawMessage {
	return cloneOpenAIWSRawMessages(c.items)
}

func (c *openAIWSToolCallReplayCollector) addItem(item gjson.Result) {
	if !item.Exists() || item.Type != gjson.JSON {
		return
	}
	raw := strings.TrimSpace(item.Raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return
	}
	if !isCodexToolCallContextItemType(item.Get("type").String()) {
		return
	}
	key := strings.TrimSpace(item.Get("id").String())
	if key == "" {
		key = strings.TrimSpace(item.Get("call_id").String())
	}
	if key == "" {
		key = raw
	}
	if c.seen == nil {
		c.seen = make(map[string]struct{})
	}
	if _, ok := c.seen[key]; ok {
		return
	}
	c.seen[key] = struct{}{}
	c.items = append(c.items, json.RawMessage(raw))
}

func buildOpenAIWSHTTPBridgeErrorEvent(statusCode int, message string) []byte {
	message = strings.TrimSpace(message)
	if message == "" {
		message = http.StatusText(statusCode)
	}
	if message == "" {
		message = "upstream request failed"
	}
	event := map[string]any{
		"type":   "error",
		"status": statusCode,
		"error": map[string]any{
			"type":    "upstream_error",
			"message": message,
		},
	}
	body, err := json.Marshal(event)
	if err != nil {
		return []byte(`{"type":"error","error":{"type":"upstream_error","message":"upstream request failed"}}`)
	}
	return body
}

func (s *OpenAIGatewayService) proxyOpenAIWSHTTPBridgeTurn(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	payload []byte,
	payloadBytes int,
	originalModel string,
	imageBillingModel string,
	imageSizeTier string,
	imageInputSize string,
	turn int,
	writeClientMessage func([]byte) error,
) (*OpenAIForwardResult, error) {
	if s == nil {
		return nil, errors.New("service is required")
	}
	if s.httpUpstream == nil {
		return nil, errors.New("openai http upstream is required")
	}
	if account == nil {
		return nil, errors.New("account is required")
	}
	if writeClientMessage == nil {
		return nil, errors.New("client websocket writer is required")
	}

	body, err := prepareOpenAIWSHTTPBridgeBody(payload)
	if err != nil {
		return nil, fmt.Errorf("prepare http bridge body: %w", err)
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	var upstreamReq *http.Request
	if account.Platform == PlatformGrok {
		// Match HTTP Grok Responses: never silently drop image_generation tools.
		if err := rejectGrokUnsupportedImageGenerationTools(body); err != nil {
			releaseUpstreamCtx()
			_ = writeClientMessage(buildOpenAIWSHTTPBridgeErrorEvent(http.StatusBadRequest, err.Error()))
			return nil, err
		}
		upstreamModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
		if originalModel != "" {
			if mappedModel := normalizeOpenAIModelForUpstream(account, account.GetMappedModel(originalModel)); mappedModel != "" {
				upstreamModel = mappedModel
			}
		}
		upstreamModel = xai.ResolveDefaultTextModel(upstreamModel)
		body, err = patchGrokResponsesBody(body, upstreamModel)
		if err != nil {
			releaseUpstreamCtx()
			return nil, err
		}
		upstreamReq, err = buildGrokResponsesRequest(upstreamCtx, c, account, body, token)
	} else {
		upstreamReq, err = s.buildUpstreamRequestOpenAIPassthrough(upstreamCtx, c, account, body, token, "")
	}
	releaseUpstreamCtx()
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	if c != nil {
		c.Set("openai_passthrough", true)
		c.Set("openai_ws_http_bridge", true)
	}

	turnStart := time.Now()
	var resp *http.Response
	if account.Platform == PlatformGrok {
		tlsRuntime := s.resolveOpenAICompatibleTLSFingerprintRuntime(ctx, c, account, "http")
		applyOpenAITLSFingerprintRuntime(upstreamReq, tlsRuntime)
		resp, err = s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	} else {
		resp, err = s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	}
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		_ = writeClientMessage(buildOpenAIWSHTTPBridgeErrorEvent(http.StatusBadGateway, "Upstream request failed"))
		return nil, fmt.Errorf("upstream http bridge request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, openAIWSHTTPBridgeErrorBodyLimitBytes))
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if upstreamMsg == "" {
			upstreamMsg = http.StatusText(resp.StatusCode)
		}
		if account.Platform == PlatformGrok {
			s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			if s.shouldFailoverUpstreamError(resp.StatusCode) {
				return nil, &UpstreamFailoverError{
					StatusCode:             resp.StatusCode,
					ResponseBody:           respBody,
					ResponseHeaders:        resp.Header.Clone(),
					RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
				}
			}
		}
		_ = writeClientMessage(buildOpenAIWSHTTPBridgeErrorEvent(resp.StatusCode, upstreamMsg))
		return nil, fmt.Errorf("upstream http bridge error: status=%d message=%s", resp.StatusCode, upstreamMsg)
	}

	if account.Platform == PlatformGrok {
		s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
	}

	responseID := ""
	usage := OpenAIUsage{}
	imageCounter := newOpenAIImageOutputCounter()
	var firstTokenMs *int
	reqStream := openAIWSPayloadBoolFromRaw(body, "stream", true)
	generateFalseWarmup := gjson.GetBytes(body, "generate").Exists() && !gjson.GetBytes(body, "generate").Bool()
	eventCount := 0
	tokenEventCount := 0
	terminalEventCount := 0
	replayCollector := &openAIWSToolCallReplayCollector{}
	firstEventType := ""
	lastEventType := ""
	sawDone := false
	wroteDownstream := false
	clientDisconnected := false
	mappedModel := ""
	needModelReplace := false
	var mappedModelBytes []byte
	if originalModel != "" {
		mappedModel = normalizeOpenAIModelForUpstream(account, account.GetMappedModel(originalModel))
		needModelReplace = mappedModel != "" && mappedModel != originalModel
		if needModelReplace {
			mappedModelBytes = []byte(mappedModel)
		}
	}

	resultWithUsage := func() *OpenAIForwardResult {
		imageCount := imageCounter.Count()
		result := &OpenAIForwardResult{
			RequestID:       responseID,
			Usage:           usage,
			Model:           originalModel,
			UpstreamModel:   mappedModel,
			ServiceTier:     extractOpenAIServiceTierFromBody(body),
			ReasoningEffort: ApplyThinkingEnabledFallback(extractOpenAIReasoningEffortFromBody(body, mappedModel, originalModel), body, mappedModel),
			Stream:          reqStream,
			OpenAIWSMode:    true,
			ResponseHeaders: cloneHeader(resp.Header),
			Duration:        time.Since(turnStart),
			FirstTokenMs:    firstTokenMs,
		}
		if replayInput := replayCollector.Items(); len(replayInput) > 0 {
			result.wsReplayInput = replayInput
			result.wsReplayInputExists = true
		}
		if imageCount > 0 {
			result.ImageCount = imageCount
			result.ImageSize = imageSizeTier
			result.ImageInputSize = imageInputSize
			result.ImageOutputSizes = imageCounter.Sizes()
			result.BillingModel = imageBillingModel
		}
		return result
	}

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanBuf := getSSEScannerBuf64K()
	scanner.Buffer(scanBuf[:0], maxLineSize)
	defer putSSEScannerBuf64K(scanBuf)

	for scanner.Scan() {
		line := scanner.Text()
		data, ok := extractOpenAISSEDataLine(line)
		if !ok {
			continue
		}
		trimmedData := strings.TrimSpace(data)
		if trimmedData == "" {
			continue
		}
		if trimmedData == "[DONE]" {
			sawDone = true
			continue
		}

		upstreamMessages := [][]byte{[]byte(trimmedData)}
		if account.Platform == PlatformGrok {
			frames, _ := normalizeGrokReasoningSSEFrame(openAICompatSSEFrame{Data: trimmedData})
			if len(frames) > 0 {
				upstreamMessages = upstreamMessages[:0]
				for _, frame := range frames {
					if data := strings.TrimSpace(frame.Data); data != "" {
						upstreamMessages = append(upstreamMessages, []byte(data))
					}
				}
			}
		}

		for _, upstreamMessage := range upstreamMessages {
			eventType, eventResponseID, _ := parseOpenAIWSEventEnvelope(upstreamMessage)
			if eventType == "" && gjson.GetBytes(upstreamMessage, "error").Exists() {
				eventType = "error"
			}
			if responseID == "" && eventResponseID != "" {
				responseID = eventResponseID
			}
			if eventType != "" {
				eventCount++
				if firstEventType == "" {
					firstEventType = eventType
				}
				lastEventType = eventType
			}
			if isOpenAIWSTokenEvent(eventType) {
				tokenEventCount++
				if firstTokenMs == nil {
					ms := int(time.Since(turnStart).Milliseconds())
					firstTokenMs = &ms
				}
			}
			if openAIWSEventShouldParseUsage(eventType) {
				parseOpenAIWSResponseUsageFromCompletedEvent(upstreamMessage, &usage)
			}
			imageCounter.AddSSEData(upstreamMessage)

			if needModelReplace && len(mappedModelBytes) > 0 && openAIWSEventMayContainModel(eventType) && strings.Contains(string(upstreamMessage), mappedModel) {
				upstreamMessage = replaceOpenAIWSMessageModel(upstreamMessage, mappedModel, originalModel)
			}
			if s.toolCorrector != nil && openAIWSEventMayContainToolCalls(eventType) && openAIWSMessageLikelyContainsToolCalls(upstreamMessage) {
				if corrected, changed := s.toolCorrector.CorrectToolCallsInSSEBytes(upstreamMessage); changed {
					upstreamMessage = corrected
				}
			}
			replayCollector.AddEvent(eventType, upstreamMessage)

			if !clientDisconnected {
				if err := writeClientMessage(upstreamMessage); err != nil {
					if isOpenAIWSClientDisconnectError(err) {
						clientDisconnected = true
						closeStatus, closeReason := summarizeOpenAIWSReadCloseError(err)
						logOpenAIWSModeInfo(
							"ingress_ws_http_bridge_client_disconnected_drain account_id=%d turn=%d close_status=%s close_reason=%s",
							account.ID,
							turn,
							closeStatus,
							truncateOpenAIWSLogValue(closeReason, openAIWSHeaderValueMaxLen),
						)
					} else {
						return nil, wrapOpenAIWSIngressTurnError(
							"write_client",
							fmt.Errorf("write client websocket event: %w", err),
							wroteDownstream,
						)
					}
				} else {
					wroteDownstream = true
				}
			}

			if eventType == "error" {
				errCodeRaw, errTypeRaw, errMsgRaw := parseOpenAIWSErrorEventFields(upstreamMessage)
				s.persistOpenAIWSRateLimitSignal(ctx, account, resp.Header, upstreamMessage, errCodeRaw, errTypeRaw, errMsgRaw)
				errMessage := strings.TrimSpace(errMsgRaw)
				if errMessage == "" {
					errMessage = "upstream error event"
				}
				return resultWithUsage(), errors.New(errMessage)
			}
			if isOpenAIWSTerminalEvent(eventType) {
				terminalEventCount++
				firstTokenMsValue := -1
				if firstTokenMs != nil {
					firstTokenMsValue = *firstTokenMs
				}
				logOpenAIWSModeInfo(
					"ingress_ws_http_bridge_turn_completed account_id=%d turn=%d response_id=%s payload_bytes=%d duration_ms=%d events=%d token_events=%d terminal_events=%d first_event=%s last_event=%s first_token_ms=%d client_disconnected=%v",
					account.ID,
					turn,
					truncateOpenAIWSLogValue(responseID, openAIWSIDValueMaxLen),
					payloadBytes,
					time.Since(turnStart).Milliseconds(),
					eventCount,
					tokenEventCount,
					terminalEventCount,
					truncateOpenAIWSLogValue(firstEventType, openAIWSLogValueMaxLen),
					truncateOpenAIWSLogValue(lastEventType, openAIWSLogValueMaxLen),
					firstTokenMsValue,
					clientDisconnected,
				)
				return resultWithUsage(), nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return resultWithUsage(), fmt.Errorf("read upstream http bridge stream: %w", err)
	}
	if generateFalseWarmup && eventCount > 0 && responseID != "" {
		completed := buildOpenAIWSHTTPBridgeGenerateFalseCompletedEvent(responseID)
		if !clientDisconnected {
			if err := writeClientMessage(completed); err != nil {
				return nil, wrapOpenAIWSIngressTurnError(
					"write_client",
					fmt.Errorf("write client websocket event: %w", err),
					wroteDownstream,
				)
			}
		}
		return resultWithUsage(), nil
	}
	if sawDone && eventCount > 0 {
		return resultWithUsage(), nil
	}
	return resultWithUsage(), errors.New("upstream http bridge stream ended before terminal event")
}

func buildOpenAIWSHTTPBridgeGenerateFalseCompletedEvent(responseID string) []byte {
	event := map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id":     strings.TrimSpace(responseID),
			"object": "response",
			"status": "completed",
			"output": []any{},
			"usage": map[string]any{
				"input_tokens":  0,
				"output_tokens": 0,
				"total_tokens":  0,
			},
		},
	}
	body, err := json.Marshal(event)
	if err != nil {
		return []byte(`{"type":"response.completed","response":{"status":"completed","output":[]}}`)
	}
	return body
}
