package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	grokComposerImageBridgeVisionModel     = "grok-build-0.1"
	grokComposerImageBridgeMaxOutputTokens = 512
)

func (s *OpenAIGatewayService) forwardGrokResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
	reqStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	return s.forwardGrokResponsesWithPromptCacheKey(ctx, c, account, body, originalModel, reqStream, startTime, "")
}

func (s *OpenAIGatewayService) forwardGrokResponsesWithPromptCacheKey(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
	reqStream bool,
	startTime time.Time,
	promptCacheKey string,
) (*OpenAIForwardResult, error) {
	// Prefer platform-default + account mapping (same path as OpenAI groups).
	upstreamModel := resolveOpenAIForwardModelWithSettings(ctx, s.settingService, account, originalModel, "")
	upstreamModel = xai.ResolveDefaultTextModel(upstreamModel)

	// Never silently drop image_generation: xAI Responses does not support it.
	// Fail closed so clients use POST /v1/images/* instead of getting plain text.
	if err := rejectGrokUnsupportedImageGenerationTools(body); err != nil {
		if c != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"type":    "invalid_request_error",
					"message": err.Error(),
					"code":    "unsupported_tool",
				},
			})
		}
		return nil, err
	}

	patchedBody, err := patchGrokResponsesBody(body, upstreamModel)
	if err != nil {
		return nil, err
	}
	// Inject prompt_cache_key into body when provided so xAI can sticky-route.
	if trimmed := strings.TrimSpace(promptCacheKey); trimmed != "" {
		if existing := strings.TrimSpace(gjson.GetBytes(patchedBody, "prompt_cache_key").String()); existing == "" {
			if updated, setErr := sjson.SetBytes(patchedBody, "prompt_cache_key", trimmed); setErr == nil {
				patchedBody = updated
			}
		} else {
			promptCacheKey = existing
		}
	} else {
		promptCacheKey = strings.TrimSpace(gjson.GetBytes(patchedBody, "prompt_cache_key").String())
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := buildGrokResponsesRequest(upstreamCtx, c, account, patchedBody, token)
	if err != nil {
		return nil, err
	}
	// Isolate session_id from prompt_cache_key so multi-tenant Grok traffic
	// does not share xAI conversation state (parity with OpenAI path).
	if trimmed := strings.TrimSpace(promptCacheKey); trimmed != "" && upstreamReq.Header.Get("session_id") == "" {
		apiKeyID := getAPIKeyIDFromContext(c)
		sessionID := generateSessionUUID(isolateOpenAISessionID(apiKeyID, trimmed))
		upstreamReq.Header.Set("session_id", sessionID)
		upstreamReq.Header.Set("x-grok-conv-id", sessionID)
	}
	tlsRuntime := s.resolveGrokTLSFingerprintRuntime(ctx, c, account, "http")
	applyGrokRuntimeHeaders(upstreamReq, tlsRuntime)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	compactionRetryTried := false
	var resp *http.Response
	for {
		upstreamStart := time.Now()
		resp, err = s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		if err != nil {
			return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
		}

		if resp.StatusCode < 400 {
			break
		}

		respBody := s.readUpstreamErrorBody(resp)
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		if !compactionRetryTried && isGrokCompactionBlobDecodeError(resp.StatusCode, respBody) {
			retryBody, retryable, retryErr := sanitizeGrokCompactionReplayBody(patchedBody)
			if retryErr != nil {
				_ = resp.Body.Close()
				return nil, retryErr
			}
			if retryable {
				_ = resp.Body.Close()
				compactionRetryTried = true
				patchedBody = retryBody
				promptCacheKey = strings.TrimSpace(gjson.GetBytes(patchedBody, "prompt_cache_key").String())
				if promptCacheKey == "" {
					promptCacheKey = strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
				}
				setOpsUpstreamRequestBody(c, patchedBody)
				upstreamReq, err = buildGrokResponsesRequest(upstreamCtx, c, account, patchedBody, token)
				if err != nil {
					return nil, err
				}
				if trimmed := strings.TrimSpace(promptCacheKey); trimmed != "" && upstreamReq.Header.Get("session_id") == "" {
					apiKeyID := getAPIKeyIDFromContext(c)
					sessionID := generateSessionUUID(isolateOpenAISessionID(apiKeyID, trimmed))
					upstreamReq.Header.Set("session_id", sessionID)
					upstreamReq.Header.Set("x-grok-conv-id", sessionID)
				}
				applyGrokRuntimeHeaders(upstreamReq, tlsRuntime)
				continue
			}
		}
		defer func() { _ = resp.Body.Close() }()

		s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		if upstreamMsg == "" {
			upstreamMsg = fmt.Sprintf("xAI upstream returned status %d", resp.StatusCode)
		}
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
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		return s.handleErrorResponse(ctx, resp, c, account, patchedBody, upstreamModel)
	}
	defer func() { _ = resp.Body.Close() }()

	s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))

	var usage *OpenAIUsage
	var firstTokenMs *int
	responseID := ""
	searchCount := 0
	if reqStream {
		streamResult, err := s.handleStreamingResponse(ctx, resp, c, account, startTime, originalModel, upstreamModel)
		if err != nil {
			// Terminal streaming errors (client disconnect / mid-stream read error)
			// already consumed upstream quota and cannot be retried, so bill the
			// partial usage. Failover errors return nil to allow retry (avoids
			// double-billing). Mirrors the OpenAI Responses path for consistency.
			if streamResult != nil && openaiStreamingErrorBillsPartial(err) {
				return buildOpenAIStreamingPartialForwardResult(resp, patchedBody, originalModel, upstreamModel, streamResult, time.Since(startTime)), err
			}
			return nil, err
		}
		usage = streamResult.usage
		firstTokenMs = streamResult.firstTokenMs
		responseID = strings.TrimSpace(streamResult.responseID)
		searchCount = streamResult.searchCount
	} else {
		nonStreamResult, err := s.handleNonStreamingResponse(ctx, resp, c, account, originalModel, upstreamModel)
		if err != nil {
			// Non-streaming terminal errors (e.g. SSE `response.failed` carrying
			// usage) already consumed upstream quota; bill the partial. Failover
			// errors return a nil result, so this guard is safe. Mirrors OpenAI.
			if nonStreamResult != nil {
				return buildOpenAIPartialForwardResult(resp, patchedBody, originalModel, upstreamModel, nonStreamResult.usage, nonStreamResult.responseID, nonStreamResult.imageCount, nonStreamResult.searchCount, time.Since(startTime)), err
			}
			return nil, err
		}
		usage = nonStreamResult.usage
		responseID = strings.TrimSpace(nonStreamResult.responseID)
		searchCount = nonStreamResult.searchCount
	}

	if usage == nil {
		usage = &OpenAIUsage{}
	}
	// Bind response_id → account so previous_response_id can stick to the same Grok account.
	if responseID != "" {
		s.bindHTTPResponseAccount(ctx, c, account, responseID)
	}
	return &OpenAIForwardResult{
		RequestID:       firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
		ResponseID:      responseID,
		Usage:           *usage,
		Model:           originalModel,
		BillingModel:    upstreamModel,
		UpstreamModel:   upstreamModel,
		ReasoningEffort: extractOpenAIReasoningEffortFromBody(patchedBody, originalModel, upstreamModel),
		Stream:          reqStream,
		OpenAIWSMode:    false,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
		// Bill actual search calls observed in the response, not merely requested tools.
		SearchCount: searchCount,
	}, nil
}

func rejectGrokUnsupportedImageGenerationTools(body []byte) error {
	if len(body) == 0 {
		return nil
	}
	for _, tool := range gjson.GetBytes(body, "tools").Array() {
		t := strings.TrimSpace(tool.Get("type").String())
		if t == "image_generation" || t == "image_generation_call" {
			return fmt.Errorf("tool type %q is not supported on Grok Responses; use POST /v1/images/generations or /v1/images/edits instead", t)
		}
	}
	return nil
}

func isGrokCompactionBlobDecodeError(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest || len(body) == 0 {
		return false
	}

	code := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		gjson.GetBytes(body, "code").String(),
		gjson.GetBytes(body, "error.code").String(),
	)))
	msg := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		gjson.GetBytes(body, "error").String(),
		gjson.GetBytes(body, "error.message").String(),
		gjson.GetBytes(body, "message").String(),
		gjson.GetBytes(body, "detail").String(),
		string(body),
	)))
	if strings.Contains(msg, "compaction blob") &&
		(strings.Contains(msg, "could not decode") || strings.Contains(msg, "decode")) {
		return true
	}
	return isOpenAIInvalidEncryptedContentError(code, msg, body)
}

func sanitizeGrokCompactionReplayBody(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return nil, false, nil
	}
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return nil, false, fmt.Errorf("parse grok compaction retry body: %w", err)
	}

	removedReasoningItems := trimOpenAIEncryptedReasoningItems(reqBody)
	previousResponseID := openAIWSPayloadString(reqBody, "previous_response_id")
	hasFunctionCallOutput := HasFunctionCallOutput(reqBody)
	droppedPreviousResponseID := false
	if previousResponseID != "" && !hasFunctionCallOutput {
		delete(reqBody, "previous_response_id")
		droppedPreviousResponseID = true
	}
	if droppedPreviousResponseID {
		if _, ok := reqBody["store"]; !ok {
			reqBody["store"] = false
		}
	}
	if !removedReasoningItems && !droppedPreviousResponseID {
		return nil, false, nil
	}
	retryBody, err := marshalOpenAIResponsesRequestBodyOrdered(reqBody)
	if err != nil {
		return nil, false, fmt.Errorf("serialize grok compaction retry body: %w", err)
	}
	return retryBody, true, nil
}

func countOpenAISearchToolsInRequestBody(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	// Dedupe by tool type: alias expansion may leave multiple web_search entries
	// (e.g. google_search + web_search_preview → two web_search tools) that should
	// bill once per distinct search capability, not once per alias.
	seen := make(map[string]struct{}, 2)
	for _, tool := range gjson.GetBytes(body, "tools").Array() {
		t := strings.TrimSpace(tool.Get("type").String())
		// Only count tools that survive Grok sanitization / are actually executed.
		if t == "web_search" || t == "x_search" {
			seen[t] = struct{}{}
		}
	}
	return len(seen)
}

func patchGrokResponsesBody(body []byte, upstreamModel string) ([]byte, error) {
	if !json.Valid(body) {
		return nil, fmt.Errorf("invalid json request body")
	}
	upstreamModel = xai.ResolveDefaultTextModel(upstreamModel)
	baseModel, suffixEffort := parseGrokThinkingSuffix(upstreamModel)
	upstreamModel = xai.StripGrokProviderPrefix(baseModel)
	out, err := sjson.SetBytes(body, "model", upstreamModel)
	if err != nil {
		return nil, err
	}
	if suffixEffort != "" && strings.TrimSpace(gjson.GetBytes(out, "reasoning.effort").String()) == "" {
		out, err = sjson.SetBytes(out, "reasoning.effort", suffixEffort)
		if err != nil {
			return nil, err
		}
	}
	if !grokSupportsReasoningEffort(upstreamModel) && gjson.GetBytes(out, "reasoning.effort").Exists() {
		out, err = sjson.DeleteBytes(out, "reasoning.effort")
		if err != nil {
			return nil, err
		}
		if reasoning := gjson.GetBytes(out, "reasoning"); reasoning.Exists() && reasoning.IsObject() && len(reasoning.Map()) == 0 {
			out, err = sjson.DeleteBytes(out, "reasoning")
			if err != nil {
				return nil, err
			}
		}
	}
	// Do not force stream=true when the client omitted/false'd it. The handler
	// branches on pre-patch reqStream; forcing stream here desyncs body vs path
	// (SSE upstream + non-stream client handling). Keep stream aligned with the client.
	// Prefer not storing conversation payloads server-side unless the client
	// explicitly opts in (Codex/Claude bridges re-send full turns).
	// Exception: multi-turn sticky (previous_response_id present) must not force
	// store=false, or continuation can break against xAI response storage.
	if !gjson.GetBytes(out, "store").Exists() {
		prevID := strings.TrimSpace(gjson.GetBytes(out, "previous_response_id").String())
		if prevID == "" {
			out, err = sjson.SetBytes(out, "store", false)
			if err != nil {
				return nil, err
			}
		}
	}
	for _, unsupportedField := range []string{"prompt_cache_retention", "safety_identifier"} {
		if gjson.GetBytes(out, unsupportedField).Exists() {
			out, err = sjson.DeleteBytes(out, unsupportedField)
			if err != nil {
				return nil, err
			}
		}
	}
	if strings.EqualFold(upstreamModel, "grok-4.5") {
		for _, unsupportedField := range []string{"presence_penalty", "presencePenalty", "frequency_penalty", "frequencyPenalty", "stop"} {
			if gjson.GetBytes(out, unsupportedField).Exists() {
				out, err = sjson.DeleteBytes(out, unsupportedField)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	out, err = sanitizeGrokResponsesUnsupportedFields(out)
	if err != nil {
		return nil, err
	}
	out = sanitizeGrokResponsesReasoningState(out)
	out, err = sanitizeGrokResponsesTools(out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func grokSupportsReasoningEffort(model string) bool {
	normalized := strings.ToLower(xai.StripGrokProviderPrefix(model))
	switch normalized {
	case xai.DefaultTextModel,
		"grok-4.5-latest",
		"grok-4.3",
		"grok-4.3-latest",
		"grok-3-mini",
		"grok-3-mini-fast",
		"grok-4.20-0309-reasoning",
		"grok-4.20-reasoning",
		"grok-4.20-multi-agent-0309":
		return true
	default:
		return false
	}
}

func sanitizeGrokResponsesReasoningState(body []byte) []byte {
	body = removeGrokEncryptedReasoningInclude(body)
	body = sanitizeGrokInputEncryptedContent(body)
	return body
}

func removeGrokEncryptedReasoningInclude(body []byte) []byte {
	include := gjson.GetBytes(body, "include")
	if !include.Exists() || !include.IsArray() {
		return body
	}
	kept := make([]string, 0, len(include.Array()))
	changed := false
	for _, item := range include.Array() {
		value := strings.TrimSpace(item.String())
		if value == "" || value == "reasoning.encrypted_content" {
			changed = true
			continue
		}
		kept = append(kept, value)
	}
	if !changed {
		return body
	}
	updated, err := sjson.SetBytes(body, "include", kept)
	if err != nil {
		return body
	}
	return updated
}

func sanitizeGrokInputEncryptedContent(body []byte) []byte {
	input := gjson.GetBytes(body, "input")
	if !input.Exists() || !input.IsArray() {
		return body
	}
	items := input.Array()
	filtered := make([]json.RawMessage, 0, len(items))
	changed := false
	for _, item := range items {
		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType != "reasoning" && itemType != "compaction" {
			filtered = append(filtered, json.RawMessage(item.Raw))
			continue
		}
		raw := []byte(item.Raw)
		if itemType == "reasoning" {
			if content := item.Get("content"); content.Exists() && content.Type == gjson.Null {
				if next, err := sjson.DeleteBytes(raw, "content"); err == nil {
					raw = next
					changed = true
				}
			}
		}
		encrypted := gjson.GetBytes(raw, "encrypted_content")
		if !encrypted.Exists() {
			filtered = append(filtered, json.RawMessage(raw))
			continue
		}
		if encrypted.Type == gjson.String && isLikelyValidGrokEncryptedContent(encrypted.String()) {
			filtered = append(filtered, json.RawMessage(raw))
			continue
		}
		changed = true
		if itemType == "compaction" {
			continue
		}
		if next, err := sjson.DeleteBytes(raw, "encrypted_content"); err == nil {
			raw = next
		}
		filtered = append(filtered, json.RawMessage(raw))
	}
	if !changed {
		return body
	}
	rawInput, err := json.Marshal(filtered)
	if err != nil {
		return body
	}
	updated, err := sjson.SetRawBytes(body, "input", rawInput)
	if err != nil {
		return body
	}
	return mergeAdjacentGrokInputReasoningSummaries(updated)
}

func mergeAdjacentGrokInputReasoningSummaries(body []byte) []byte {
	input := gjson.GetBytes(body, "input")
	if !input.Exists() || !input.IsArray() {
		return body
	}
	changed := false
	items := make([]json.RawMessage, 0, len(input.Array()))
	for _, item := range input.Array() {
		if len(items) > 0 && canMergeGrokReasoningSummary(items[len(items)-1], item) {
			merged, ok := appendGrokReasoningSummary(items[len(items)-1], item.Get("summary").Array())
			if ok {
				items[len(items)-1] = json.RawMessage(merged)
				changed = true
				continue
			}
		}
		items = append(items, json.RawMessage(item.Raw))
	}
	if !changed {
		return body
	}
	rawInput, err := json.Marshal(items)
	if err != nil {
		return body
	}
	updated, err := sjson.SetRawBytes(body, "input", rawInput)
	if err != nil {
		return body
	}
	return updated
}

func canMergeGrokReasoningSummary(previous json.RawMessage, current gjson.Result) bool {
	previousItem := gjson.ParseBytes(previous)
	if previousItem.Get("type").String() != "reasoning" || current.Get("type").String() != "reasoning" {
		return false
	}
	if !previousItem.Get("summary").IsArray() || !current.Get("summary").IsArray() {
		return false
	}
	if len(current.Get("summary").Array()) == 0 {
		return false
	}
	for name := range current.Map() {
		if name != "type" && name != "summary" {
			return false
		}
	}
	return true
}

func appendGrokReasoningSummary(previous json.RawMessage, currentSummary []gjson.Result) ([]byte, bool) {
	updated := []byte(previous)
	summary := gjson.GetBytes(updated, "summary")
	if !summary.IsArray() {
		return previous, false
	}
	nextIndex := len(summary.Array())
	for i, item := range currentSummary {
		next, err := sjson.SetRawBytes(updated, fmt.Sprintf("summary.%d", nextIndex+i), []byte(item.Raw))
		if err != nil {
			return previous, false
		}
		updated = next
	}
	return updated, true
}

func isLikelyValidGrokEncryptedContent(raw string) bool {
	_, err := inspectGrokEncryptedContent(raw)
	return err == nil
}

func inspectGrokEncryptedContent(raw string) (int, error) {
	sig := strings.TrimSpace(raw)
	if sig == "" {
		return 0, fmt.Errorf("empty Grok encrypted_content")
	}
	if len(sig) > 8*1024*1024 {
		return 0, fmt.Errorf("Grok encrypted_content exceeds maximum length")
	}
	if sig != raw {
		return 0, fmt.Errorf("Grok encrypted_content has leading or trailing whitespace")
	}
	if strings.HasPrefix(sig, "gAAAA") {
		return 0, fmt.Errorf("Grok encrypted_content looks like GPT/Codex reasoning signature")
	}
	if strings.Contains(sig, "=") {
		return 0, fmt.Errorf("invalid Grok encrypted_content: expected unpadded standard base64")
	}
	for _, r := range sig {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '+' || r == '/':
		default:
			return 0, fmt.Errorf("invalid Grok encrypted_content: contains non-base64 character")
		}
	}
	if isDecodableClaudeThinkingSignature(sig) {
		return 0, fmt.Errorf("Grok encrypted_content looks like Claude thinking signature")
	}
	if isKnownGeminiThoughtSignatureEnvelope(sig) {
		return 0, fmt.Errorf("Grok encrypted_content looks like Gemini thoughtSignature")
	}
	decoded, err := base64.RawStdEncoding.DecodeString(sig)
	if err != nil {
		return 0, fmt.Errorf("invalid Grok encrypted_content: base64 decode failed: %w", err)
	}
	if len(decoded) < 50 {
		return 0, fmt.Errorf("invalid Grok encrypted_content: decoded payload too short")
	}
	if entropyRatio := byteEntropyRatio(decoded); entropyRatio < 0.85 {
		return 0, fmt.Errorf("invalid Grok encrypted_content: decoded payload entropy ratio %.3f below %.3f", entropyRatio, 0.85)
	}
	return len(decoded), nil
}

func isDecodableClaudeThinkingSignature(raw string) bool {
	sig := strings.TrimSpace(raw)
	if idx := strings.IndexByte(sig, '#'); idx >= 0 {
		sig = strings.TrimSpace(sig[idx+1:])
	}
	if sig == "" || len(sig) > 8*1024*1024 {
		return false
	}
	switch sig[0] {
	case 'E':
		decoded, err := base64.StdEncoding.DecodeString(sig)
		return err == nil && len(decoded) > 0
	case 'R':
		decoded, err := base64.StdEncoding.DecodeString(sig)
		if err != nil || len(decoded) == 0 || decoded[0] != 'E' {
			return false
		}
		innerDecoded, err := base64.StdEncoding.DecodeString(string(decoded))
		return err == nil && len(innerDecoded) > 0
	default:
		return false
	}
}

func isKnownGeminiThoughtSignatureEnvelope(raw string) bool {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(decoded) == 0 {
		return false
	}
	envelope := classifyGeminiThoughtSignatureEnvelope(decoded)
	return envelope != ""
}

func classifyGeminiThoughtSignatureEnvelope(decoded []byte) string {
	switch {
	case isGeminiField1Envelope(decoded):
		return "protobuf_field_1"
	case isGeminiField2Envelope(decoded):
		return "protobuf_field_2"
	default:
		return ""
	}
}

func isGeminiField1Envelope(decoded []byte) bool {
	offset := 0
	recordCount := 0
	for offset < len(decoded) {
		num, typ, n := protowire.ConsumeTag(decoded[offset:])
		if n < 0 || num != 1 || typ != protowire.BytesType {
			return false
		}
		offset += n
		value, n := protowire.ConsumeBytes(decoded[offset:])
		if n < 0 || !isLikelyGeminiOpaquePayload(value) {
			return false
		}
		recordCount++
		offset += n
	}
	return offset == len(decoded) && recordCount > 0
}

func isGeminiField2Envelope(decoded []byte) bool {
	value, ok := consumeGeminiField2Field1Value(decoded)
	return ok && isLikelyGeminiOpaquePayload(value)
}

func consumeGeminiField2Field1Value(decoded []byte) ([]byte, bool) {
	num, typ, n := protowire.ConsumeTag(decoded)
	if n < 0 || num != 2 || typ != protowire.BytesType {
		return nil, false
	}
	offset := n
	container, n := protowire.ConsumeBytes(decoded[offset:])
	if n < 0 {
		return nil, false
	}
	offset += n
	if offset != len(decoded) {
		return nil, false
	}
	num, typ, n = protowire.ConsumeTag(container)
	if n < 0 || num != 1 || typ != protowire.BytesType {
		return nil, false
	}
	containerOffset := n
	value, n := protowire.ConsumeBytes(container[containerOffset:])
	if n < 0 {
		return nil, false
	}
	containerOffset += n
	if containerOffset != len(container) {
		return nil, false
	}
	return value, true
}

func isLikelyGeminiOpaquePayload(value []byte) bool {
	return len(value) > 0 && value[0] == 0x01
}

func byteEntropyRatio(buf []byte) float64 {
	if len(buf) == 0 {
		return 0
	}
	var counts [256]int
	for _, b := range buf {
		counts[b]++
	}
	n := float64(len(buf))
	entropy := 0.0
	for _, count := range counts {
		if count == 0 {
			continue
		}
		p := float64(count) / n
		entropy -= p * math.Log2(p)
	}
	maxSymbols := len(buf)
	if maxSymbols > 256 {
		maxSymbols = 256
	}
	if maxSymbols <= 1 {
		return 0
	}
	return entropy / math.Log2(float64(maxSymbols))
}

func parseGrokThinkingSuffix(model string) (baseModel string, effort string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return model, ""
	}
	if !strings.HasSuffix(model, ")") {
		return model, ""
	}
	open := strings.LastIndex(model, "(")
	if open <= 0 {
		return model, ""
	}
	base := strings.TrimSpace(model[:open])
	suffix := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(model[open+1:], ")")))
	switch suffix {
	case "low", "medium", "high":
		if base != "" {
			return base, suffix
		}
	}
	return model, ""
}

var grokResponsesUnsupportedRecursiveFields = map[string]struct{}{
	"external_web_access": {},
}

func sanitizeGrokResponsesUnsupportedFields(body []byte) ([]byte, error) {
	if !bytes.Contains(body, []byte(`"external_web_access"`)) {
		return body, nil
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if !deleteJSONFields(payload, grokResponsesUnsupportedRecursiveFields) {
		return body, nil
	}
	return json.Marshal(payload)
}

func deleteJSONFields(value any, fields map[string]struct{}) bool {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		for field := range fields {
			if _, ok := typed[field]; ok {
				delete(typed, field)
				changed = true
			}
		}
		for _, child := range typed {
			if deleteJSONFields(child, fields) {
				changed = true
			}
		}
		return changed
	case []any:
		changed := false
		for _, child := range typed {
			if deleteJSONFields(child, fields) {
				changed = true
			}
		}
		return changed
	default:
		return false
	}
}

var grokResponsesSupportedToolTypes = map[string]struct{}{
	"code_execution":     {},
	"code_interpreter":   {},
	"collections_search": {},
	"file_search":        {},
	"function":           {},
	"mcp":                {},
	"shell":              {},
	"web_search":         {},
	"x_search":           {},
}

// normalizeGrokResponsesToolType maps client tool aliases onto types xAI accepts
// so they are executed instead of silently dropped by sanitizeGrokResponsesTools.
//
// tool_search is intentionally NOT remapped to web_search: OpenAI's tool_search is
// a deferred tool-loading mechanism, not web search. Without deferred loading
// support, tool_search is dropped by sanitizeGrokResponsesTools (not in the
// supported set) rather than billed/executed as a web search.
func normalizeGrokResponsesToolType(toolType string) string {
	trimmed := strings.TrimSpace(toolType)
	switch {
	case trimmed == "web_search_20250305", trimmed == "google_search":
		return "web_search"
	case trimmed == "web_search_preview" || strings.HasPrefix(trimmed, "web_search_preview_"):
		// OpenAI preview tool type / dated variants → xAI web_search.
		return "web_search"
	// Codex CLI historically advertises local_shell; xAI Responses uses shell.
	case trimmed == "local_shell":
		return "shell"
	default:
		return trimmed
	}
}

func sanitizeGrokResponsesTools(body []byte) ([]byte, error) {
	tools := gjson.GetBytes(body, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return dropGrokToolChoiceWithoutTools(body)
	}

	rawTools := tools.Array()
	filteredTools := make([]json.RawMessage, 0, len(rawTools))
	for _, tool := range rawTools {
		toolType := normalizeGrokResponsesToolType(strings.TrimSpace(tool.Get("type").String()))
		if _, ok := grokResponsesSupportedToolTypes[toolType]; !ok {
			continue
		}
		raw := json.RawMessage(tool.Raw)
		if original := strings.TrimSpace(tool.Get("type").String()); original != toolType {
			if rewritten, err := sjson.SetBytes([]byte(tool.Raw), "type", toolType); err == nil {
				raw = json.RawMessage(rewritten)
			}
		}
		filteredTools = append(filteredTools, raw)
	}

	var err error
	// Always rewrite tools slice so alias remaps (same count) still land in body.
	if len(filteredTools) == 0 {
		if len(rawTools) > 0 {
			body, err = sjson.DeleteBytes(body, "tools")
			if err != nil {
				return nil, err
			}
		}
		body, err = dropGrokToolChoiceWithoutTools(body)
		if err != nil {
			return nil, err
		}
	} else {
		encoded, marshalErr := json.Marshal(filteredTools)
		if marshalErr != nil {
			return nil, marshalErr
		}
		body, err = sjson.SetRawBytes(body, "tools", encoded)
		if err != nil {
			return nil, err
		}
	}

	toolChoice := gjson.GetBytes(body, "tool_choice")
	if !toolChoice.Exists() {
		return body, nil
	}
	if shouldDropGrokToolChoice(toolChoice, filteredTools) {
		body, err = sjson.DeleteBytes(body, "tool_choice")
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	// Rewrite tool_choice.type aliases (e.g. local_shell → shell) for xAI.
	if toolChoice.IsObject() {
		original := strings.TrimSpace(toolChoice.Get("type").String())
		if normalized := normalizeGrokResponsesToolType(original); original != "" && normalized != original {
			if rewritten, setErr := sjson.SetBytes(body, "tool_choice.type", normalized); setErr == nil {
				body = rewritten
			}
		}
	}
	return body, nil
}

func dropGrokToolChoiceWithoutTools(body []byte) ([]byte, error) {
	var err error
	if gjson.GetBytes(body, "tool_choice").Exists() {
		body, err = sjson.DeleteBytes(body, "tool_choice")
		if err != nil {
			return nil, err
		}
	}
	if gjson.GetBytes(body, "parallel_tool_calls").Exists() {
		body, err = sjson.DeleteBytes(body, "parallel_tool_calls")
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func shouldDropGrokToolChoice(toolChoice gjson.Result, tools []json.RawMessage) bool {
	if len(tools) == 0 {
		return true
	}
	if !toolChoice.IsObject() {
		return false
	}
	choiceType := normalizeGrokResponsesToolType(strings.TrimSpace(toolChoice.Get("type").String()))
	if choiceType == "" {
		return false
	}
	if _, ok := grokResponsesSupportedToolTypes[choiceType]; !ok {
		return true
	}
	if choiceType == "function" {
		choiceName := strings.TrimSpace(toolChoice.Get("name").String())
		if choiceName == "" {
			choiceName = strings.TrimSpace(toolChoice.Get("function.name").String())
		}
		if choiceName == "" {
			return false
		}
		for _, tool := range tools {
			var item struct {
				Type     string `json:"type"`
				Name     string `json:"name"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if err := json.Unmarshal(tool, &item); err != nil {
				continue
			}
			name := strings.TrimSpace(item.Name)
			if name == "" {
				name = strings.TrimSpace(item.Function.Name)
			}
			if strings.TrimSpace(item.Type) == "function" && name == choiceName {
				return false
			}
		}
		return true
	}
	return false
}

func (s *OpenAIGatewayService) bridgeGrokComposerImageInputs(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
) ([]byte, OpenAIUsage, bool, error) {
	if !shouldBridgeGrokComposerImageInputs(body) {
		return body, OpenAIUsage{}, false, nil
	}

	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return body, OpenAIUsage{}, false, fmt.Errorf("parse grok composer image bridge request: %w", err)
	}

	imageURLs := collectGrokComposerImageURLs(reqBody)
	if len(imageURLs) == 0 {
		return body, OpenAIUsage{}, false, nil
	}

	descriptions := make([]string, 0, len(imageURLs))
	var bridgeUsage OpenAIUsage
	for index, imageURL := range imageURLs {
		description, usage, err := s.describeGrokComposerImage(ctx, c, account, token, imageURL, index+1)
		if err != nil {
			return body, bridgeUsage, false, err
		}
		descriptions = append(descriptions, description)
		addOpenAIUsage(&bridgeUsage, usage)
	}

	if !rewriteGrokComposerImagesAsText(reqBody, descriptions) {
		return body, bridgeUsage, false, nil
	}
	bridgedBody, err := marshalOpenAIUpstreamJSON(reqBody)
	if err != nil {
		return body, bridgeUsage, false, fmt.Errorf("serialize grok composer image bridge request: %w", err)
	}
	return bridgedBody, bridgeUsage, true, nil
}

func shouldBridgeGrokComposerImageInputs(body []byte) bool {
	if len(body) == 0 || !isGrokComposerModel(gjson.GetBytes(body, "model").String()) {
		return false
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() {
		return false
	}
	return openAIJSONValueMayContainImageInput(messages)
}

func isGrokComposerModel(model string) bool {
	model = strings.TrimSpace(strings.ToLower(model))
	if model == "" {
		return false
	}
	if strings.Contains(model, "/") {
		parts := strings.Split(model, "/")
		model = strings.TrimSpace(parts[len(parts)-1])
	}
	return strings.Contains(model, "composer")
}

func collectGrokComposerImageURLs(reqBody map[string]any) []string {
	messages, ok := reqBody["messages"].([]any)
	if !ok {
		return nil
	}

	var imageURLs []string
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := msgMap["content"].([]any)
		if !ok {
			continue
		}
		for _, part := range parts {
			if imageURL := grokComposerImageURLFromPart(part); imageURL != "" {
				imageURLs = append(imageURLs, imageURL)
			}
		}
	}
	return imageURLs
}

func grokComposerImageURLFromPart(part any) string {
	partMap, ok := part.(map[string]any)
	if !ok {
		return ""
	}
	if strings.TrimSpace(strings.ToLower(fmt.Sprint(partMap["type"]))) != "image_url" {
		return ""
	}
	switch imageURL := partMap["image_url"].(type) {
	case string:
		return normalizeGrokComposerImageURL(imageURL)
	case map[string]any:
		raw, _ := imageURL["url"].(string)
		return normalizeGrokComposerImageURL(raw)
	default:
		return ""
	}
}

func normalizeGrokComposerImageURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || isEmptyBase64DataURI(trimmed) {
		return ""
	}
	return trimmed
}

func (s *OpenAIGatewayService) describeGrokComposerImage(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	imageURL string,
	index int,
) (string, OpenAIUsage, error) {
	body, err := buildGrokComposerImageDescriptionBody(imageURL, index)
	if err != nil {
		return "", OpenAIUsage{}, err
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	upstreamReq, err := buildGrokResponsesRequest(upstreamCtx, c, account, body, token)
	releaseUpstreamCtx()
	if err != nil {
		return "", OpenAIUsage{}, fmt.Errorf("build grok composer image bridge request: %w", err)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	// Match main Grok Responses/media paths: same TLS fingerprint profile + UA/originator.
	tlsRuntime := s.resolveGrokTLSFingerprintRuntime(ctx, c, account, "http")
	applyGrokRuntimeHeaders(upstreamReq, tlsRuntime)

	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	if err != nil {
		return "", OpenAIUsage{}, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody := s.readUpstreamErrorBody(resp)
		s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		if upstreamMsg == "" {
			upstreamMsg = fmt.Sprintf("xAI image bridge upstream returned status %d", resp.StatusCode)
		}
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
			return "", OpenAIUsage{}, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		return "", OpenAIUsage{}, fmt.Errorf("grok composer image bridge upstream error: %s", upstreamMsg)
	}

	s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, nil)
	if err != nil {
		return "", OpenAIUsage{}, fmt.Errorf("read grok composer image bridge response: %w", err)
	}

	var parsed apicompat.ResponsesResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", OpenAIUsage{}, fmt.Errorf("parse grok composer image bridge response: %w", err)
	}
	description := strings.TrimSpace(grokResponsesOutputText(&parsed))
	if description == "" {
		return "", copyOpenAIUsageFromResponsesUsage(parsed.Usage), fmt.Errorf("grok composer image bridge returned empty description")
	}
	return description, copyOpenAIUsageFromResponsesUsage(parsed.Usage), nil
}

func buildGrokComposerImageDescriptionBody(imageURL string, index int) ([]byte, error) {
	prompt := fmt.Sprintf("Describe image %d in concise, factual text for a downstream coding/composer model. Include visible text, UI elements, diagrams, errors, and spatial relationships. Do not mention that you are an image analysis bridge.", index)
	req := map[string]any{
		"model":             grokComposerImageBridgeVisionModel,
		"stream":            false,
		"store":             false,
		"max_output_tokens": grokComposerImageBridgeMaxOutputTokens,
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": prompt},
					map[string]any{"type": "input_image", "image_url": imageURL},
				},
			},
		},
	}
	return marshalOpenAIUpstreamJSON(req)
}

func grokResponsesOutputText(resp *apicompat.ResponsesResponse) string {
	if resp == nil {
		return ""
	}
	var parts []string
	for _, output := range resp.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" || content.Type == "text" || content.Type == "input_text" {
				if text := strings.TrimSpace(content.Text); text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func rewriteGrokComposerImagesAsText(reqBody map[string]any, descriptions []string) bool {
	messages, ok := reqBody["messages"].([]any)
	if !ok {
		return false
	}

	imageIndex := 0
	changed := false
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := msgMap["content"].([]any)
		if !ok {
			continue
		}
		var textParts []string
		messageChanged := false
		for _, part := range parts {
			if imageURL := grokComposerImageURLFromPart(part); imageURL != "" {
				if imageIndex < len(descriptions) {
					textParts = append(textParts, fmt.Sprintf("Image %d description: %s", imageIndex+1, strings.TrimSpace(descriptions[imageIndex])))
				}
				imageIndex++
				messageChanged = true
				continue
			}
			if text := grokComposerTextFromPart(part); text != "" {
				textParts = append(textParts, text)
			}
		}
		if messageChanged {
			msgMap["content"] = strings.Join(textParts, "\n\n")
			changed = true
		}
	}
	return changed
}

func grokComposerTextFromPart(part any) string {
	partMap, ok := part.(map[string]any)
	if !ok {
		return ""
	}
	partType := strings.TrimSpace(strings.ToLower(fmt.Sprint(partMap["type"])))
	switch partType {
	case "text", "input_text":
		text, _ := partMap["text"].(string)
		return strings.TrimSpace(text)
	default:
		return ""
	}
}

func buildGrokResponsesRequest(ctx context.Context, c *gin.Context, account *Account, body []byte, token string) (*http.Request, error) {
	targetURL, err := xai.BuildResponsesURL(account.GetGrokBaseURL())
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	// Align with OpenAI Responses clients: accept both JSON and SSE so streaming
	// Claude Code / Codex traffic works through the same Grok Responses upstream.
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Connection", "Keep-Alive")
	req.Header.Set("OpenAI-Beta", "responses=experimental")

	req.Header.Set("User-Agent", resolveGrokUpstreamUserAgent(c))
	if c != nil {
		if v := strings.TrimSpace(c.GetHeader("OpenAI-Beta")); v != "" {
			req.Header.Set("OpenAI-Beta", v)
		}
		if convID := strings.TrimSpace(c.GetHeader("x-grok-conv-id")); convID != "" {
			req.Header.Set("x-grok-conv-id", isolateOpenAISessionID(getAPIKeyIDFromContext(c), convID))
		}
		// Preserve client session affinity when present so multi-turn Codex /
		// Claude Code conversations stay sticky on Grok upstream.
		if sessionID := strings.TrimSpace(c.GetHeader("session_id")); sessionID != "" {
			apiKeyID := getAPIKeyIDFromContext(c)
			if apiKeyID > 0 {
				sessionID = isolateOpenAISessionID(apiKeyID, sessionID)
			}
			req.Header.Set("session_id", sessionID)
			req.Header.Set("x-grok-conv-id", sessionID)
		}
	}
	if req.Header.Get("x-grok-conv-id") == "" {
		if promptCacheKey := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()); promptCacheKey != "" {
			apiKeyID := getAPIKeyIDFromContext(c)
			sessionID := generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey))
			req.Header.Set("x-grok-conv-id", sessionID)
		}
	}
	_ = account
	return req, nil
}

func (s *OpenAIGatewayService) updateGrokUsageSnapshot(ctx context.Context, accountID int64, snapshot *xai.QuotaSnapshot) {
	if s == nil || s.accountRepo == nil || accountID <= 0 || snapshot == nil {
		return
	}
	if s.codexSnapshotThrottle != nil && !s.codexSnapshotThrottle.Allow(accountID, time.Now()) {
		return
	}
	updates := map[string]any{
		grokQuotaSnapshotExtraKey: snapshot,
	}
	// Also derive the scheduling-threshold extras (grok_sched_*) the evaluator
	// reads in grokThresholdCandidates. Without this writer the admin-configured
	// Grok auto-pause threshold could never fire (the read side was dead config).
	for k, v := range buildGrokSchedulerExtraUpdates(snapshot) {
		updates[k] = v
	}
	_ = s.accountRepo.UpdateExtra(ctx, accountID, updates)
}

// buildGrokSchedulerExtraUpdates derives the grok_sched_* scheduling snapshot
// (utilization percent + reset time) consumed by EvaluateAccountSchedulingThreshold.
// Utilization is the most-constrained of the requests/tokens windows.
func buildGrokSchedulerExtraUpdates(snapshot *xai.QuotaSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}
	util, reset, ok := grokSnapshotUtilization(snapshot)
	if !ok {
		return nil
	}
	updates := map[string]any{
		"grok_sched_utilization":      util,
		"grok_sched_usage_updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if reset != nil {
		// 防御：调度阈值暂停时长由 grok_sched_reset_at 决定。若上游返回脏的
		// reset 头（例如把相对毫秒 "6000" 误当相对秒解析出 ~33h 的未来时刻），
		// 不设上限会把耗尽账号长时间锁死。xAI 配额窗口不会超过一天，因此对
		// 未来时刻做 grokMaxSchedulingResetHorizon 钳制；过去/无效值直接不写。
		now := time.Now()
		if reset.After(now) {
			capped := *reset
			if horizon := now.Add(grokMaxSchedulingResetHorizon); capped.After(horizon) {
				capped = horizon
			}
			updates["grok_sched_reset_at"] = capped.UTC().Format(time.RFC3339)
		}
	}
	return updates
}

// grokSnapshotUtilization returns the highest window utilization (0-100) across
// the requests/tokens quota windows and the reset time of that window.
func grokSnapshotUtilization(snapshot *xai.QuotaSnapshot) (float64, *time.Time, bool) {
	if snapshot == nil {
		return 0, nil, false
	}
	best := -1.0
	var bestReset *time.Time
	consider := func(window *xai.QuotaWindow) {
		if window == nil || window.Limit == nil || *window.Limit <= 0 || window.Remaining == nil {
			return
		}
		remaining := *window.Remaining
		if remaining < 0 {
			remaining = 0
		}
		util := (1 - float64(remaining)/float64(*window.Limit)) * 100
		if util < 0 {
			util = 0
		}
		if util > 100 {
			util = 100
		}
		if util > best {
			best = util
			if window.ResetUnix != nil {
				t := time.Unix(*window.ResetUnix, 0).UTC()
				bestReset = &t
			} else {
				bestReset = nil
			}
		}
	}
	consider(snapshot.Requests)
	consider(snapshot.Tokens)
	if best < 0 {
		return 0, nil, false
	}
	return best, bestReset, true
}

// grokMaxUpstreamCooldown caps how long a single upstream error response can
// remove a Grok account from scheduling. Prevents a malformed/hostile
// `retry-after: 86400` (or a bogus reset header) from parking an account for a
// day off one bad response.
const grokMaxUpstreamCooldown = 30 * time.Minute

// grokMaxSchedulingResetHorizon bounds how far into the future a Grok
// scheduling-threshold pause (grok_sched_reset_at) may be set, so a malformed
// upstream reset header can't park an over-threshold account for days. xAI quota
// windows do not exceed ~a day.
const grokMaxSchedulingResetHorizon = 25 * time.Hour

func (s *OpenAIGatewayService) handleGrokAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte) {
	if s == nil || account == nil {
		return
	}
	// Refresh the quota snapshot from every error response so quota-based
	// auto-pause and the admin quota view see the latest reset window, including
	// on 429 (previously the reset headers here were ignored entirely).
	if snapshot := xai.ParseQuotaHeaders(headers, statusCode); snapshot != nil {
		s.updateGrokUsageSnapshot(ctx, account.ID, snapshot)
	}

	switch statusCode {
	case http.StatusUnauthorized, http.StatusTooManyRequests:
		// Route 401/429 through the unified pipeline so Grok gets the same
		// treatment as every other platform: 401 invalidates the token cache,
		// permanently disables accounts missing a refresh_token, and honors
		// OAuth401CooldownMinutes; 429 uses the parsed x-ratelimit-reset-* window
		// (capped) and respects pool-mode / custom-error-code rules.
		if s.rateLimitService != nil {
			if s.rateLimitService.HandleUpstreamError(ctx, account, statusCode, headers, responseBody) {
				s.BlockAccountScheduling(account, time.Time{}, "upstream_disable")
			}
			return
		}
		// Defensive fallback if the rate-limit service is unwired (tests).
		s.tempUnscheduleGrok(ctx, account, grokUpstreamCooldownFor(statusCode, headers), "grok upstream error")
	case http.StatusForbidden:
		// Keep 403 recoverable with a bounded cooldown rather than the generic
		// permanent SetError — a transient entitlement/tier 403 should not kill
		// the account outright.
		s.tempUnscheduleGrok(ctx, account, grokMaxUpstreamCooldown, "grok entitlement or subscription tier denied")
	default:
		if statusCode >= 500 {
			s.tempUnscheduleGrok(ctx, account, 2*time.Minute, "grok upstream temporary error")
		}
	}
}

// grokUpstreamCooldownFor derives a capped cooldown from the upstream reset /
// retry-after headers, used only on the defensive fallback path.
func grokUpstreamCooldownFor(statusCode int, headers http.Header) time.Duration {
	cooldown := 2 * time.Minute
	if snapshot := xai.ParseQuotaHeaders(headers, statusCode); snapshot != nil && snapshot.RetryAfterSeconds != nil && *snapshot.RetryAfterSeconds > 0 {
		cooldown = time.Duration(*snapshot.RetryAfterSeconds) * time.Second
	}
	if cooldown > grokMaxUpstreamCooldown {
		cooldown = grokMaxUpstreamCooldown
	}
	return cooldown
}

// grokRateLimitResetTime returns the soonest capped reset time from Grok/xAI
// rate-limit headers (x-ratelimit-reset-requests/tokens or retry-after), or nil
// when none are present. The result is clamped to grokMaxUpstreamCooldown from
// now so a hostile/oversized value can't park the account indefinitely.
func grokRateLimitResetTime(headers http.Header) *time.Time {
	snapshot := xai.ParseQuotaHeaders(headers, http.StatusTooManyRequests)
	if snapshot == nil {
		return nil
	}
	now := time.Now()
	maxUntil := now.Add(grokMaxUpstreamCooldown)
	var reset *time.Time
	consider := func(candidate time.Time) {
		if !candidate.After(now) {
			return
		}
		if candidate.After(maxUntil) {
			candidate = maxUntil
		}
		if reset == nil || candidate.Before(*reset) {
			c := candidate
			reset = &c
		}
	}
	if snapshot.RetryAfterSeconds != nil && *snapshot.RetryAfterSeconds > 0 {
		consider(now.Add(time.Duration(*snapshot.RetryAfterSeconds) * time.Second))
	}
	for _, window := range []*xai.QuotaWindow{snapshot.Requests, snapshot.Tokens} {
		if window != nil && window.ResetUnix != nil {
			consider(time.Unix(*window.ResetUnix, 0))
		}
	}
	return reset
}

func (s *OpenAIGatewayService) tempUnscheduleGrok(ctx context.Context, account *Account, cooldown time.Duration, reason string) {
	if s == nil || account == nil {
		return
	}
	until := time.Now().Add(cooldown)
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(until) {
		until = *account.TempUnschedulableUntil
	}
	s.BlockAccountScheduling(account, until, reason)
	if s.accountRepo != nil {
		stateCtx, cancel := openAIAccountStateContext(ctx)
		defer cancel()
		_ = s.accountRepo.SetTempUnschedulable(stateCtx, account.ID, until, reason)
	}
}

func ptrStringOrNil(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func normalizeGrokReasoningSSEFrame(frame openAICompatSSEFrame) ([]openAICompatSSEFrame, bool) {
	data := strings.TrimSpace(openAICompatPayloadWithEventType(frame.Data, frame.EventType))
	if data == "" || data == "[DONE]" || !gjson.Valid(data) {
		return []openAICompatSSEFrame{frame}, false
	}
	normalizedDataEvents := normalizeGrokReasoningSummaryDataEvents([]byte(data))
	if len(normalizedDataEvents) == 0 {
		return []openAICompatSSEFrame{frame}, false
	}
	frames := make([]openAICompatSSEFrame, 0, len(normalizedDataEvents))
	changed := len(normalizedDataEvents) != 1
	for _, eventData := range normalizedDataEvents {
		eventType := strings.TrimSpace(gjson.GetBytes(eventData, "type").String())
		if eventType == "" {
			eventType = normalizeGrokReasoningSummaryEventName(frame.EventType)
		}
		next := frame
		if eventType != "" {
			setOpenAICompatSSEFrameEventType(&next, eventType)
		}
		openAICompatSetSSEFrameData(&next, string(eventData))
		if next.EventType != frame.EventType || next.Data != frame.Data {
			changed = true
		}
		frames = append(frames, next)
	}
	return frames, changed
}

func setOpenAICompatSSEFrameEventType(frame *openAICompatSSEFrame, eventType string) {
	if frame == nil {
		return
	}
	eventType = strings.TrimSpace(eventType)
	frame.EventType = eventType
	if len(frame.Fields) == 0 {
		return
	}
	updated := false
	for i := range frame.Fields {
		if frame.Fields[i].Name == "event" {
			frame.Fields[i].Value = eventType
			frame.Fields[i].Raw = "event: " + eventType
			updated = true
		}
	}
	if !updated && eventType != "" {
		fields := make([]openAICompatSSEField, 0, len(frame.Fields)+1)
		fields = append(fields, openAICompatSSEField{Name: "event", Value: eventType, Raw: "event: " + eventType})
		fields = append(fields, frame.Fields...)
		frame.Fields = fields
	}
}

func normalizeGrokReasoningSummaryEventName(eventName string) string {
	switch strings.TrimSpace(eventName) {
	case "response.reasoning_text.delta":
		return "response.reasoning_summary_text.delta"
	case "response.reasoning_text.done":
		return "response.reasoning_summary_part.done"
	default:
		return strings.TrimSpace(eventName)
	}
}

func normalizeGrokReasoningSummaryDataEvents(eventData []byte) [][]byte {
	if len(eventData) == 0 || !gjson.ValidBytes(eventData) {
		return [][]byte{eventData}
	}
	if gjson.GetBytes(eventData, "type").String() != "response.reasoning_text.done" {
		return [][]byte{normalizeGrokReasoningSummaryData(eventData)}
	}

	textDone, _ := sjson.SetBytes(eventData, "type", "response.reasoning_summary_text.done")
	textDone = normalizeGrokReasoningSummaryIndex(textDone)
	partDone := normalizeGrokReasoningSummaryData(eventData)
	return [][]byte{textDone, partDone}
}

func normalizeGrokReasoningSummaryData(eventData []byte) []byte {
	if len(eventData) == 0 || !gjson.ValidBytes(eventData) {
		return eventData
	}

	normalized := eventData
	switch gjson.GetBytes(normalized, "type").String() {
	case "response.reasoning_text.delta":
		normalized, _ = sjson.SetBytes(normalized, "type", "response.reasoning_summary_text.delta")
		normalized = normalizeGrokReasoningSummaryIndex(normalized)
	case "response.reasoning_text.done":
		normalized, _ = sjson.SetBytes(normalized, "type", "response.reasoning_summary_part.done")
		normalized, _ = sjson.SetBytes(normalized, "part.type", "summary_text")
		if text := gjson.GetBytes(normalized, "text"); text.Exists() {
			normalized, _ = sjson.SetBytes(normalized, "part.text", text.String())
		}
		normalized, _ = sjson.DeleteBytes(normalized, "text")
		normalized = normalizeGrokReasoningSummaryIndex(normalized)
	case "response.content_part.added":
		if gjson.GetBytes(normalized, "part.type").String() == "reasoning_text" {
			normalized, _ = sjson.SetBytes(normalized, "type", "response.reasoning_summary_part.added")
			normalized, _ = sjson.SetBytes(normalized, "part.type", "summary_text")
			normalized = normalizeGrokReasoningSummaryIndex(normalized)
		}
	case "response.content_part.done":
		if gjson.GetBytes(normalized, "part.type").String() == "reasoning_text" {
			normalized, _ = sjson.SetBytes(normalized, "type", "response.reasoning_summary_part.done")
			normalized, _ = sjson.SetBytes(normalized, "part.type", "summary_text")
			normalized = normalizeGrokReasoningSummaryIndex(normalized)
		}
	}

	if item := gjson.GetBytes(normalized, "item"); item.Exists() && item.Type == gjson.JSON {
		updatedItem := normalizeGrokReasoningOutputItem([]byte(item.Raw))
		if !bytes.Equal(updatedItem, []byte(item.Raw)) {
			normalized, _ = sjson.SetRawBytes(normalized, "item", updatedItem)
		}
	}
	if output := gjson.GetBytes(normalized, "response.output"); output.IsArray() {
		updatedOutput, changed := normalizeGrokReasoningOutputItems(output.Array())
		if changed {
			normalized, _ = sjson.SetRawBytes(normalized, "response.output", updatedOutput)
		}
	}

	return normalized
}

func normalizeGrokReasoningSummaryIndex(eventData []byte) []byte {
	contentIndex := gjson.GetBytes(eventData, "content_index")
	if contentIndex.Exists() && contentIndex.Raw != "" && !gjson.GetBytes(eventData, "summary_index").Exists() {
		eventData, _ = sjson.SetRawBytes(eventData, "summary_index", []byte(contentIndex.Raw))
	}
	eventData, _ = sjson.DeleteBytes(eventData, "content_index")
	return eventData
}

func normalizeGrokReasoningResponseBody(body []byte) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}
	out := body
	if output := gjson.GetBytes(out, "response.output"); output.IsArray() {
		updatedOutput, changed := normalizeGrokReasoningOutputItems(output.Array())
		if changed {
			out, _ = sjson.SetRawBytes(out, "response.output", updatedOutput)
		}
	}
	if output := gjson.GetBytes(out, "output"); output.IsArray() {
		updatedOutput, changed := normalizeGrokReasoningOutputItems(output.Array())
		if changed {
			out, _ = sjson.SetRawBytes(out, "output", updatedOutput)
		}
	}
	return out
}

func normalizeGrokReasoningOutputItems(items []gjson.Result) ([]byte, bool) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	changed := false
	for i, item := range items {
		if i > 0 {
			buf.WriteByte(',')
		}
		updatedItem := normalizeGrokReasoningOutputItem([]byte(item.Raw))
		if !bytes.Equal(updatedItem, []byte(item.Raw)) {
			changed = true
		}
		buf.Write(updatedItem)
	}
	buf.WriteByte(']')
	return buf.Bytes(), changed
}

func normalizeGrokReasoningOutputItem(item []byte) []byte {
	if !gjson.ValidBytes(item) || gjson.GetBytes(item, "type").String() != "reasoning" {
		return item
	}

	normalized := item
	if summary := gjson.GetBytes(normalized, "summary"); summary.IsArray() {
		updatedSummary, changed := normalizeGrokReasoningSummaryItems(summary.Array())
		if changed {
			normalized, _ = sjson.SetRawBytes(normalized, "summary", updatedSummary)
		}
	}

	content := gjson.GetBytes(normalized, "content")
	if !content.IsArray() {
		return normalized
	}

	summaryItems := make([]gjson.Result, 0, len(content.Array()))
	for _, part := range content.Array() {
		if part.Get("type").String() == "reasoning_text" {
			summaryItems = append(summaryItems, part)
		}
	}
	if len(summaryItems) == 0 {
		return normalized
	}

	updatedSummary, _ := normalizeGrokReasoningSummaryItems(summaryItems)
	normalized, _ = sjson.SetRawBytes(normalized, "summary", updatedSummary)
	normalized, _ = sjson.DeleteBytes(normalized, "content")
	return normalized
}

func normalizeGrokReasoningSummaryItems(items []gjson.Result) ([]byte, bool) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	changed := false
	for i, item := range items {
		if i > 0 {
			buf.WriteByte(',')
		}
		itemRaw := []byte(item.Raw)
		if item.Get("type").String() == "reasoning_text" {
			var errSet error
			itemRaw, errSet = sjson.SetBytes(itemRaw, "type", "summary_text")
			if errSet == nil {
				changed = true
			}
		}
		buf.Write(itemRaw)
	}
	buf.WriteByte(']')
	return buf.Bytes(), changed
}
