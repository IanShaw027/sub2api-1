package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

const (
	openAIWSBetaV1Value = "responses_websockets=2026-02-04"
	openAIWSBetaV2Value = "responses_websockets=2026-02-06"

	openAIWSTurnStateHeader    = "x-codex-turn-state"
	openAIWSTurnMetadataHeader = "x-codex-turn-metadata"
	openAIWSWindowIDHeader     = "x-codex-window-id"

	openAIWSLogValueMaxLen      = 160
	openAIWSHeaderValueMaxLen   = 120
	openAIWSIDValueMaxLen       = 64
	openAIWSEventLogHeadLimit   = 20
	openAIWSEventLogEveryN      = 50
	openAIWSBufferLogHeadLimit  = 8
	openAIWSBufferLogEveryN     = 20
	openAIWSPrewarmEventLogHead = 10
	openAIWSPayloadKeySizeTopN  = 6

	openAIWSPayloadSizeEstimateDepth    = 3
	openAIWSPayloadSizeEstimateMaxBytes = 64 * 1024
	openAIWSPayloadSizeEstimateMaxItems = 16

	openAIWSEventFlushBatchSizeDefault = 4
	openAIWSEventFlushIntervalDefault  = 25 * time.Millisecond
	openAIWSPayloadLogSampleDefault    = 0.2

	openAIWSStoreDisabledConnModeStrict   = "strict"
	openAIWSStoreDisabledConnModeAdaptive = "adaptive"
	openAIWSStoreDisabledConnModeOff      = "off"
	openAIWSStoreModeFull                 = "full"
	openAIWSStoreModeIncremental          = "incremental"

	openAIWSIngressStagePreviousResponseNotFound = "previous_response_not_found"
	openAIWSMaxPrevResponseIDDeletePasses        = 8

	openAIWSSoftRateLimitAdvisoryMessage      = "Approaching upstream rate limits; switch account and retry"
	openAIWSSoftRateLimitAdvisoryThreshold    = 90.0
	openAIWSSoftRateLimitAdvisoryDefaultLimit = "codex"
)

var openAIWSLogValueReplacer = strings.NewReplacer(
	"error", "err",
	"fallback", "fb",
	"warning", "warnx",
	"failed", "fail",
)

const defaultOpenAIWSIngressPreflightPingIdle = 20 * time.Second

type openAIWSTransportTimeouts struct {
	Dial            time.Duration
	Acquire         time.Duration
	Read            time.Duration
	Write           time.Duration
	PassthroughIdle time.Duration
}

// openAIWSFallbackError 表示可安全回退到 HTTP 的 WS 错误（尚未写下游）。
type openAIWSFallbackError struct {
	Reason             string
	Err                error
	PreviousResponseID string
	ActiveDelta        bool
}

func (e *openAIWSFallbackError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("openai ws fallback: %s", strings.TrimSpace(e.Reason))
	}
	return fmt.Sprintf("openai ws fallback: %s: %v", strings.TrimSpace(e.Reason), e.Err)
}

func (e *openAIWSFallbackError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func wrapOpenAIWSFallback(reason string, err error) error {
	return &openAIWSFallbackError{Reason: strings.TrimSpace(reason), Err: err}
}

func wrapOpenAIWSFallbackWithPayloadState(reason string, err error, previousResponseID string, activeDelta bool) error {
	return &openAIWSFallbackError{
		Reason:             strings.TrimSpace(reason),
		Err:                err,
		PreviousResponseID: strings.TrimSpace(previousResponseID),
		ActiveDelta:        activeDelta,
	}
}

type openAIWSResponseFailedError struct {
	code      string
	errType   string
	message   string
	retryable bool
}

func (e *openAIWSResponseFailedError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.message)
}

// OpenAIWSClientCloseError 表示应以指定 WebSocket close code 主动关闭客户端连接的错误。
type OpenAIWSClientCloseError struct {
	statusCode coderws.StatusCode
	reason     string
	err        error
}

type openAIWSIngressTurnError struct {
	stage           string
	cause           error
	wroteDownstream bool
}

func (e *openAIWSIngressTurnError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause == nil {
		return strings.TrimSpace(e.stage)
	}
	return e.cause.Error()
}

func (e *openAIWSIngressTurnError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func wrapOpenAIWSIngressTurnError(stage string, cause error, wroteDownstream bool) error {
	if cause == nil {
		return nil
	}
	return &openAIWSIngressTurnError{
		stage:           strings.TrimSpace(stage),
		cause:           cause,
		wroteDownstream: wroteDownstream,
	}
}

func isOpenAIWSIngressTurnRetryable(err error) bool {
	var turnErr *openAIWSIngressTurnError
	if !errors.As(err, &turnErr) || turnErr == nil {
		return false
	}
	if errors.Is(turnErr.cause, context.Canceled) || errors.Is(turnErr.cause, context.DeadlineExceeded) {
		return false
	}
	if turnErr.wroteDownstream {
		return false
	}
	switch turnErr.stage {
	case "write_upstream", "read_upstream":
		return true
	default:
		return false
	}
}

func openAIWSIngressTurnRetryReason(err error) string {
	var turnErr *openAIWSIngressTurnError
	if !errors.As(err, &turnErr) || turnErr == nil {
		return "unknown"
	}
	if turnErr.stage == "" {
		return "unknown"
	}
	return turnErr.stage
}

func isOpenAIWSIngressPreviousResponseNotFound(err error) bool {
	var turnErr *openAIWSIngressTurnError
	if !errors.As(err, &turnErr) || turnErr == nil {
		return false
	}
	if strings.TrimSpace(turnErr.stage) != openAIWSIngressStagePreviousResponseNotFound {
		return false
	}
	return !turnErr.wroteDownstream
}

// NewOpenAIWSClientCloseError 创建一个客户端 WS 关闭错误。
func NewOpenAIWSClientCloseError(statusCode coderws.StatusCode, reason string, err error) error {
	return &OpenAIWSClientCloseError{
		statusCode: statusCode,
		reason:     strings.TrimSpace(reason),
		err:        err,
	}
}

func (e *OpenAIWSClientCloseError) Error() string {
	if e == nil {
		return ""
	}
	if e.err == nil {
		return fmt.Sprintf("openai ws client close: %d %s", int(e.statusCode), strings.TrimSpace(e.reason))
	}
	return fmt.Sprintf("openai ws client close: %d %s: %v", int(e.statusCode), strings.TrimSpace(e.reason), e.err)
}

func (e *OpenAIWSClientCloseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *OpenAIWSClientCloseError) StatusCode() coderws.StatusCode {
	if e == nil {
		return coderws.StatusInternalError
	}
	return e.statusCode
}

func (e *OpenAIWSClientCloseError) Reason() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.reason)
}

// OpenAIWSIngressHooks 定义入站 WS 每个 turn 的生命周期回调。
type OpenAIWSIngressHooks struct {
	// InitialRequestModel 是首帧渠道映射前的请求模型，只用于 usage metadata
	// 的 reasoning effort 后缀推导，禁止用于上游请求或计费模型。
	InitialRequestModel string
	SessionHash         string
	BeforeTurn          func(turn int) error
	BeforeRequest       func(turn int, payload []byte, originalModel string) error
	AfterTurn           func(turn int, payload []byte, result *OpenAIForwardResult, turnErr error)
}

func normalizeOpenAIWSLogValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return openAIWSLogValueReplacer.Replace(trimmed)
}

func normalizeOpenAIWSLogValueNoReplace(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

func truncateOpenAIWSLogValue(value string, maxLen int) string {
	normalized := normalizeOpenAIWSLogValue(value)
	if normalized == "-" || maxLen <= 0 {
		return normalized
	}
	if len(normalized) <= maxLen {
		return normalized
	}
	return normalized[:maxLen] + "..."
}

func truncateOpenAIWSLogValueNoReplace(value string, maxLen int) string {
	normalized := normalizeOpenAIWSLogValueNoReplace(value)
	if normalized == "-" || maxLen <= 0 {
		return normalized
	}
	if len(normalized) <= maxLen {
		return normalized
	}
	return normalized[:maxLen] + "..."
}

func compactOpenAIWSLogValue(value string, maxLen int) string {
	compact := truncateOpenAIWSLogValue(value, maxLen)
	compact = strings.ReplaceAll(compact, " ", "_")
	compact = strings.ReplaceAll(compact, "\t", "_")
	compact = strings.ReplaceAll(compact, "\n", "_")
	compact = strings.ReplaceAll(compact, "\r", "_")
	return compact
}

func openAIWSHeaderValueForLog(headers http.Header, key string) string {
	if headers == nil {
		return "-"
	}
	return truncateOpenAIWSLogValue(headers.Get(key), openAIWSHeaderValueMaxLen)
}

func hasOpenAIWSHeader(headers http.Header, key string) bool {
	if headers == nil {
		return false
	}
	return strings.TrimSpace(headers.Get(key)) != ""
}

type openAIWSSessionHeaderResolution struct {
	SessionID          string
	ConversationID     string
	SessionSource      string
	ConversationSource string
}

func isolateOpenAIWSWindowID(apiKeyID int64, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	sessionPart := raw
	suffix := ""
	if idx := strings.Index(raw, ":"); idx >= 0 {
		sessionPart = raw[:idx]
		suffix = raw[idx:]
	}
	return isolateOpenAISessionID(apiKeyID, sessionPart) + suffix
}

func resolveOpenAIWSSessionHeaders(c *gin.Context, promptCacheKey string) openAIWSSessionHeaderResolution {
	resolution := openAIWSSessionHeaderResolution{
		SessionSource:      "none",
		ConversationSource: "none",
	}
	if c != nil && c.Request != nil {
		if sessionID := strings.TrimSpace(c.Request.Header.Get("session_id")); sessionID != "" {
			resolution.SessionID = sessionID
			resolution.SessionSource = "header_session_id"
		}
		if conversationID := strings.TrimSpace(c.Request.Header.Get("conversation_id")); conversationID != "" {
			resolution.ConversationID = conversationID
			resolution.ConversationSource = "header_conversation_id"
			if resolution.SessionID == "" {
				resolution.SessionID = conversationID
				resolution.SessionSource = "header_conversation_id"
			}
		}
	}

	cacheKey := strings.TrimSpace(promptCacheKey)
	if cacheKey != "" {
		if resolution.SessionID == "" {
			resolution.SessionID = cacheKey
			resolution.SessionSource = "prompt_cache_key"
		}
	}
	return resolution
}

func shouldLogOpenAIWSEvent(idx int, eventType string) bool {
	if idx <= openAIWSEventLogHeadLimit {
		return true
	}
	if openAIWSEventLogEveryN > 0 && idx%openAIWSEventLogEveryN == 0 {
		return true
	}
	if eventType == "error" || isOpenAIWSTerminalEvent(eventType) {
		return true
	}
	return false
}

func shouldLogOpenAIWSBufferedEvent(idx int) bool {
	if idx <= openAIWSBufferLogHeadLimit {
		return true
	}
	if openAIWSBufferLogEveryN > 0 && idx%openAIWSBufferLogEveryN == 0 {
		return true
	}
	return false
}

func openAIWSEventMayContainModel(eventType string) bool {
	switch eventType {
	case "response.created",
		"response.in_progress",
		"response.completed",
		"response.done",
		"response.failed",
		"response.incomplete",
		"response.cancelled",
		"response.canceled":
		return true
	default:
		trimmed := strings.TrimSpace(eventType)
		if trimmed == eventType {
			return false
		}
		switch trimmed {
		case "response.created",
			"response.in_progress",
			"response.completed",
			"response.done",
			"response.failed",
			"response.incomplete",
			"response.cancelled",
			"response.canceled":
			return true
		default:
			return false
		}
	}
}

func openAIWSEventMayContainToolCalls(eventType string) bool {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return false
	}
	if strings.Contains(eventType, "function_call") || strings.Contains(eventType, "tool_call") {
		return true
	}
	switch eventType {
	case "response.output_item.added", "response.output_item.done", "response.completed", "response.done":
		return true
	default:
		return false
	}
}

func openAIWSEventShouldParseUsage(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
		return true
	default:
		return false
	}
}

func parseOpenAIWSEventEnvelope(message []byte) (eventType string, responseID string, response gjson.Result) {
	if len(message) == 0 {
		return "", "", gjson.Result{}
	}
	values := gjson.GetManyBytes(message, "type", "response.id", "id", "response")
	eventType = strings.TrimSpace(values[0].String())
	if id := strings.TrimSpace(values[1].String()); id != "" {
		responseID = id
	} else {
		responseID = strings.TrimSpace(values[2].String())
	}
	return eventType, responseID, values[3]
}

func parseOpenAIWSResponseFailedErrorFields(message []byte) (code string, errType string, errMessage string) {
	if len(message) == 0 {
		return "", "", ""
	}
	code = strings.TrimSpace(gjson.GetBytes(message, "response.error.code").String())
	errType = strings.TrimSpace(gjson.GetBytes(message, "response.error.type").String())
	errMessage = strings.TrimSpace(gjson.GetBytes(message, "response.error.message").String())
	return code, errType, errMessage
}

func openAIWSMessageLikelyContainsToolCalls(message []byte) bool {
	if len(message) == 0 {
		return false
	}
	return bytes.Contains(message, []byte(`"tool_calls"`)) ||
		bytes.Contains(message, []byte(`"tool_call"`)) ||
		bytes.Contains(message, []byte(`"function_call"`))
}

func parseOpenAIWSResponseUsageFromCompletedEvent(message []byte, usage *OpenAIUsage) {
	if usage == nil || len(message) == 0 {
		return
	}
	if parsedUsage, ok := extractOpenAIUsageFromJSONBytes(message); ok {
		*usage = parsedUsage
	}
}

func parseOpenAIWSErrorEventFields(message []byte) (code string, errType string, errMessage string) {
	if len(message) == 0 {
		return "", "", ""
	}
	values := gjson.GetManyBytes(message, "error.code", "error.type", "error.message")
	return strings.TrimSpace(values[0].String()), strings.TrimSpace(values[1].String()), strings.TrimSpace(values[2].String())
}

func summarizeOpenAIWSErrorEventFieldsFromRaw(codeRaw, errTypeRaw, errMessageRaw string) (code string, errType string, errMessage string) {
	code = truncateOpenAIWSLogValue(codeRaw, openAIWSLogValueMaxLen)
	errType = truncateOpenAIWSLogValue(errTypeRaw, openAIWSLogValueMaxLen)
	errMessage = truncateOpenAIWSLogValue(errMessageRaw, openAIWSLogValueMaxLen)
	return code, errType, errMessage
}

func summarizeOpenAIWSErrorEventFields(message []byte) (code string, errType string, errMessage string) {
	if len(message) == 0 {
		return "-", "-", "-"
	}
	return summarizeOpenAIWSErrorEventFieldsFromRaw(parseOpenAIWSErrorEventFields(message))
}

func summarizeOpenAIWSPayloadKeySizes(payload map[string]any, topN int) string {
	if len(payload) == 0 {
		return "-"
	}
	type keySize struct {
		Key  string
		Size int
	}
	sizes := make([]keySize, 0, len(payload))
	for key, value := range payload {
		size := estimateOpenAIWSPayloadValueSize(value, openAIWSPayloadSizeEstimateDepth)
		sizes = append(sizes, keySize{Key: key, Size: size})
	}
	sort.Slice(sizes, func(i, j int) bool {
		if sizes[i].Size == sizes[j].Size {
			return sizes[i].Key < sizes[j].Key
		}
		return sizes[i].Size > sizes[j].Size
	})

	if topN <= 0 || topN > len(sizes) {
		topN = len(sizes)
	}
	parts := make([]string, 0, topN)
	for idx := 0; idx < topN; idx++ {
		item := sizes[idx]
		parts = append(parts, fmt.Sprintf("%s:%d", item.Key, item.Size))
	}
	return strings.Join(parts, ",")
}

func estimateOpenAIWSPayloadValueSize(value any, depth int) int {
	if depth <= 0 {
		return -1
	}
	switch v := value.(type) {
	case nil:
		return 0
	case string:
		return len(v)
	case []byte:
		return len(v)
	case bool:
		return 1
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return 8
	case float32, float64:
		return 8
	case map[string]any:
		if len(v) == 0 {
			return 2
		}
		total := 2
		count := 0
		for key, item := range v {
			count++
			if count > openAIWSPayloadSizeEstimateMaxItems {
				return -1
			}
			itemSize := estimateOpenAIWSPayloadValueSize(item, depth-1)
			if itemSize < 0 {
				return -1
			}
			total += len(key) + itemSize + 3
			if total > openAIWSPayloadSizeEstimateMaxBytes {
				return -1
			}
		}
		return total
	case []any:
		if len(v) == 0 {
			return 2
		}
		total := 2
		limit := len(v)
		if limit > openAIWSPayloadSizeEstimateMaxItems {
			return -1
		}
		for i := 0; i < limit; i++ {
			itemSize := estimateOpenAIWSPayloadValueSize(v[i], depth-1)
			if itemSize < 0 {
				return -1
			}
			total += itemSize + 1
			if total > openAIWSPayloadSizeEstimateMaxBytes {
				return -1
			}
		}
		return total
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return -1
		}
		if len(raw) > openAIWSPayloadSizeEstimateMaxBytes {
			return -1
		}
		return len(raw)
	}
}

func openAIWSPayloadString(payload map[string]any, key string) string {
	if len(payload) == 0 {
		return ""
	}
	raw, ok := payload[key]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	default:
		return ""
	}
}

func openAIWSPayloadStringFromRaw(payload []byte, key string) string {
	if len(payload) == 0 || strings.TrimSpace(key) == "" {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(payload, key).String())
}

func openAIWSPayloadBoolFromRaw(payload []byte, key string, defaultValue bool) bool {
	if len(payload) == 0 || strings.TrimSpace(key) == "" {
		return defaultValue
	}
	value := gjson.GetBytes(payload, key)
	if !value.Exists() {
		return defaultValue
	}
	if value.Type != gjson.True && value.Type != gjson.False {
		return defaultValue
	}
	return value.Bool()
}

func extractOpenAIWSImageURL(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		if raw, ok := v["url"].(string); ok {
			return strings.TrimSpace(raw)
		}
	}
	return ""
}

func summarizeOpenAIWSInput(input any) string {
	items, ok := input.([]any)
	if !ok || len(items) == 0 {
		return "-"
	}

	itemCount := len(items)
	textChars := 0
	imageDataURLs := 0
	imageDataURLChars := 0
	imageRemoteURLs := 0

	handleContentItem := func(contentItem map[string]any) {
		contentType, _ := contentItem["type"].(string)
		switch strings.TrimSpace(contentType) {
		case "input_text", "output_text", "text":
			if text, ok := contentItem["text"].(string); ok {
				textChars += len(text)
			}
		case "input_image":
			imageURL := extractOpenAIWSImageURL(contentItem["image_url"])
			if imageURL == "" {
				return
			}
			if strings.HasPrefix(strings.ToLower(imageURL), "data:image/") {
				imageDataURLs++
				imageDataURLChars += len(imageURL)
				return
			}
			imageRemoteURLs++
		}
	}

	handleInputItem := func(inputItem map[string]any) {
		if content, ok := inputItem["content"].([]any); ok {
			for _, rawContent := range content {
				contentItem, ok := rawContent.(map[string]any)
				if !ok {
					continue
				}
				handleContentItem(contentItem)
			}
			return
		}

		itemType, _ := inputItem["type"].(string)
		switch strings.TrimSpace(itemType) {
		case "input_text", "output_text", "text":
			if text, ok := inputItem["text"].(string); ok {
				textChars += len(text)
			}
		case "input_image":
			imageURL := extractOpenAIWSImageURL(inputItem["image_url"])
			if imageURL == "" {
				return
			}
			if strings.HasPrefix(strings.ToLower(imageURL), "data:image/") {
				imageDataURLs++
				imageDataURLChars += len(imageURL)
				return
			}
			imageRemoteURLs++
		}
	}

	for _, rawItem := range items {
		inputItem, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		handleInputItem(inputItem)
	}

	return fmt.Sprintf(
		"items=%d,text_chars=%d,image_data_urls=%d,image_data_url_chars=%d,image_remote_urls=%d",
		itemCount,
		textChars,
		imageDataURLs,
		imageDataURLChars,
		imageRemoteURLs,
	)
}

func dropOpenAIWSPayloadKey(payload map[string]any, key string, removed *[]string) {
	if len(payload) == 0 || strings.TrimSpace(key) == "" {
		return
	}
	if _, exists := payload[key]; !exists {
		return
	}
	delete(payload, key)
	*removed = append(*removed, key)
}

// applyOpenAIWSRetryPayloadStrategy 在 WS 连续失败时仅移除无语义字段，
// 避免重试成功却改变原始请求语义。
// 注意：prompt_cache_key 不应在重试中移除；它常用于会话稳定标识（session_id 兜底）。
func applyOpenAIWSRetryPayloadStrategy(payload map[string]any, attempt int) (strategy string, removedKeys []string) {
	if len(payload) == 0 {
		return "empty", nil
	}
	if attempt <= 1 {
		return "full", nil
	}

	removed := make([]string, 0, 2)
	if attempt >= 2 {
		dropOpenAIWSPayloadKey(payload, "include", &removed)
	}

	if len(removed) == 0 {
		return "full", nil
	}
	sort.Strings(removed)
	return "trim_optional_fields", removed
}

func stripOpenAIWSCreatePayloadUnsupportedFields(payload map[string]any) {
	if len(payload) == 0 {
		return
	}
	for _, field := range openAIResponsesUnsupportedFields {
		delete(payload, field)
	}
}

func logOpenAIWSModeInfo(format string, args ...any) {
	logger.LegacyPrintf("service.openai_gateway", "[OpenAI WS Mode][openai_ws_mode=true] "+format, args...)
}

func logOpenAIWSModeInfoDirect(format string, args ...any) {
	msg := fmt.Sprintf("[OpenAI WS Mode][openai_ws_mode=true] "+format, args...)
	logger.L().
		With(zap.String("component", "service.openai_gateway")).
		WithOptions(zap.AddCallerSkip(1)).
		Info(msg, zap.Bool("legacy_printf", true))
}

func isOpenAIWSModeDebugEnabled() bool {
	return logger.L().Core().Enabled(zap.DebugLevel)
}

func logOpenAIWSModeDebug(format string, args ...any) {
	if !isOpenAIWSModeDebugEnabled() {
		return
	}
	logger.LegacyPrintf("service.openai_gateway", "[debug] [OpenAI WS Mode][openai_ws_mode=true] "+format, args...)
}

type openAIWSContinuationProbeLog struct {
	AccountID                 int64
	AccountType               string
	ConnID                    string
	PreviousResponseID        string
	PreviousResponseIDKind    string
	PreviousResponseIDSource  string
	OriginalPreviousIDPresent bool
	PreferredConnID           string
	ConnReused                bool
	StoreDisabled             bool
	StoreMode                 string
	StoreEnabled              bool
	StickyAccountID           int64
	StickyAccountHit          bool
	StickyAccountMismatch     bool
	ConnAffinityHit           bool
	FallbackReason            string
	PayloadBytes              int
	ConnPickMs                int64
	QueueWaitMs               int64
	ConnAgeMs                 int64
	ConnIdleMs                int64
	ConnLeaseCount            int64
	SessionHash               string
	HeaderSessionID           string
	HeaderConversationID      string
	SessionIDSource           string
	ConversationIDSource      string
	HasTurnState              bool
	TurnStateLen              int
	HasPromptCacheKey         bool
}

func openAIWSContinuationProbeLogMessage(v openAIWSContinuationProbeLog) string {
	return fmt.Sprintf(
		"continuation_probe account_id=%d account_type=%s conn_id=%s previous_response_id=%s previous_response_id_kind=%s previous_response_id_source=%s original_previous_response_id_present=%v preferred_conn_id=%s conn_reused=%v store_disabled=%v store_mode=%s store_enabled=%v sticky_account_id=%d sticky_account_hit=%v sticky_account_conflict=%v conn_affinity_hit=%v fallback_reason=%s payload_bytes=%d conn_pick_ms=%d queue_wait_ms=%d conn_age_ms=%d conn_idle_ms=%d conn_lease_count=%d session_hash=%s header_session_id=%s header_conversation_id=%s session_id_source=%s conversation_id_source=%s has_turn_state=%v turn_state_len=%d has_prompt_cache_key=%v",
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		truncateOpenAIWSLogValue(v.PreviousResponseID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDKind),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDSource),
		v.OriginalPreviousIDPresent,
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		v.StoreDisabled,
		normalizeOpenAIWSLogValue(v.StoreMode),
		v.StoreEnabled,
		v.StickyAccountID,
		v.StickyAccountHit,
		v.StickyAccountMismatch,
		v.ConnAffinityHit,
		normalizeOpenAIWSLogValue(v.FallbackReason),
		v.PayloadBytes,
		v.ConnPickMs,
		v.QueueWaitMs,
		v.ConnAgeMs,
		v.ConnIdleMs,
		v.ConnLeaseCount,
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		normalizeOpenAIWSLogValue(v.HeaderSessionID),
		normalizeOpenAIWSLogValue(v.HeaderConversationID),
		normalizeOpenAIWSLogValue(v.SessionIDSource),
		normalizeOpenAIWSLogValue(v.ConversationIDSource),
		v.HasTurnState,
		v.TurnStateLen,
		v.HasPromptCacheKey,
	)
}

func logOpenAIWSContinuationProbe(v openAIWSContinuationProbeLog) {
	logOpenAIWSModeInfo("%s", openAIWSContinuationProbeLogMessage(v))
}

// TEMP_DIAG(ctx_pool_store_true): remove these INFO diagnostics after store=true coverage debugging.
type openAIWSDiagnosticStartLog struct {
	RequestID                 string
	ClientRequestID           string
	AccountID                 int64
	AccountType               string
	ConnProfile               openAIWSConnProfile
	ConnID                    string
	ConnReused                bool
	Transport                 string
	Model                     string
	Stream                    string
	PayloadEventType          string
	PayloadBytes              int
	PayloadKeys               string
	InputSummary              string
	PreviousResponseID        string
	PreviousResponseIDKind    string
	PreviousResponseIDSource  string
	OriginalPreviousIDPresent bool
	PreferredConnID           string
	StoreMode                 string
	StoreEnabled              bool
	StoreDisabled             bool
	StickyAccountID           int64
	StickyAccountHit          bool
	StickyAccountMismatch     bool
	ConnAffinityHit           bool
	FallbackReason            string
	DroppedPreviousResponseID bool
	UnsafeToolContinuation    bool
	DeltaActive               bool
	DeltaItems                int
	DeltaBytes                int
	FullItems                 int
	FullBytes                 int
	ConnPickMs                int64
	QueueWaitMs               int64
	ConnAgeMs                 int64
	ConnIdleMs                int64
	ConnLeaseCount            int64
	SessionHash               string
	HeaderSessionID           string
	HeaderConversationID      string
	SessionIDSource           string
	ConversationIDSource      string
	HasTurnState              bool
	TurnStateLen              int
	HasPromptCacheKey         bool
	HasTools                  bool
	HTTPIngressWSOneShot      bool
	ForceNewConn              bool
	ForcePreferredConn        bool
	AffinityOnlyReuse         bool
	StoreDisabledConnMode     string
	ProxyEnabled              bool
}

func openAIWSDiagnosticStartLogMessage(v openAIWSDiagnosticStartLog) string {
	return fmt.Sprintf(
		"openai_ws_diag_start temporary_diag=ctx_pool_store_true remove_after_debug=true request_id=%s client_request_id=%s account_id=%d account_type=%s conn_profile=%s conn_id=%s conn_reused=%v transport=%s model=%s stream=%s payload_event=%s payload_bytes=%d payload_keys=%s input_summary=%s previous_response_id=%s previous_response_id_kind=%s previous_response_id_source=%s original_previous_response_id_present=%v preferred_conn_id=%s store_mode=%s store_enabled=%v store_disabled=%v sticky_account_id=%d sticky_account_hit=%v sticky_account_conflict=%v conn_affinity_hit=%v fallback_reason=%s dropped_previous_response_id=%v unsafe_tool_continuation=%v delta_active=%v delta_items=%d delta_bytes=%d full_items=%d full_bytes=%d conn_pick_ms=%d queue_wait_ms=%d conn_age_ms=%d conn_idle_ms=%d conn_lease_count=%d session_hash=%s header_session_id=%s header_conversation_id=%s session_id_source=%s conversation_id_source=%s has_turn_state=%v turn_state_len=%d has_prompt_cache_key=%v has_tools=%v http_ingress_ws_one_shot=%v force_new_conn=%v force_preferred_conn=%v affinity_only_reuse=%v store_disabled_conn_mode=%s proxy_enabled=%v",
		normalizeOpenAIWSLogValue(v.RequestID),
		normalizeOpenAIWSLogValue(v.ClientRequestID),
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		normalizeOpenAIWSLogValue(v.Transport),
		normalizeOpenAIWSLogValue(v.Model),
		normalizeOpenAIWSLogValue(v.Stream),
		normalizeOpenAIWSLogValue(v.PayloadEventType),
		v.PayloadBytes,
		normalizeOpenAIWSLogValue(v.PayloadKeys),
		normalizeOpenAIWSLogValue(v.InputSummary),
		truncateOpenAIWSLogValue(v.PreviousResponseID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDKind),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDSource),
		v.OriginalPreviousIDPresent,
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.StoreMode),
		v.StoreEnabled,
		v.StoreDisabled,
		v.StickyAccountID,
		v.StickyAccountHit,
		v.StickyAccountMismatch,
		v.ConnAffinityHit,
		normalizeOpenAIWSLogValue(v.FallbackReason),
		v.DroppedPreviousResponseID,
		v.UnsafeToolContinuation,
		v.DeltaActive,
		v.DeltaItems,
		v.DeltaBytes,
		v.FullItems,
		v.FullBytes,
		v.ConnPickMs,
		v.QueueWaitMs,
		v.ConnAgeMs,
		v.ConnIdleMs,
		v.ConnLeaseCount,
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		normalizeOpenAIWSLogValue(v.HeaderSessionID),
		normalizeOpenAIWSLogValue(v.HeaderConversationID),
		normalizeOpenAIWSLogValue(v.SessionIDSource),
		normalizeOpenAIWSLogValue(v.ConversationIDSource),
		v.HasTurnState,
		v.TurnStateLen,
		v.HasPromptCacheKey,
		v.HasTools,
		v.HTTPIngressWSOneShot,
		v.ForceNewConn,
		v.ForcePreferredConn,
		v.AffinityOnlyReuse,
		normalizeOpenAIWSLogValue(v.StoreDisabledConnMode),
		v.ProxyEnabled,
	)
}

func logOpenAIWSDiagnosticStart(v openAIWSDiagnosticStartLog) {
	logOpenAIWSModeInfo("%s", openAIWSDiagnosticStartLogMessage(v))
}

type openAIWSAcquirePreferredSnapshotLog struct {
	RequestID                     string
	GroupID                       int64
	APIKeyID                      int64
	AccountID                     int64
	AccountType                   string
	SessionHash                   string
	PreferredConnID               string
	PreferredConnInPool           bool
	PreferredConnProfile          openAIWSConnProfile
	PreferredConnAgeMS            int64
	PreferredConnIdleMS           int64
	PreferredConnLeaseCount       int64
	PreferredConnLeased           bool
	PreferredConnWaiters          int32
	PreferredConnMatchesAcquire   bool
	PreferredConnProfileMatches   bool
	PreferredConnIdentityRequired bool
	PreferredConnIdentityMatches  bool
	PreferredConnReuseKeyMatches  bool
	ConnProfile                   openAIWSConnProfile
	ForcePreferredConn            bool
	ForceNewConn                  bool
	AffinityOnlyReuse             bool
	HasPreviousResponseID         bool
	StoreMode                     string
	StoreDisabled                 bool
	StoreFallbackReason           string
}

func openAIWSAcquirePreferredSnapshotLogMessage(v openAIWSAcquirePreferredSnapshotLog) string {
	return fmt.Sprintf(
		"openai_ws_acquire_preferred_snapshot temporary_diag=sticky_select remove_after_debug=true request_id=%s group_id=%d api_key_id=%d account_id=%d account_type=%s session_hash=%s preferred_conn_id=%s preferred_conn_in_pool=%v preferred_conn_profile=%s preferred_conn_age_ms=%d preferred_conn_idle_ms=%d preferred_conn_lease_count=%d preferred_conn_leased=%v preferred_conn_waiters=%d preferred_conn_matches_acquire=%v preferred_conn_profile_matches=%v preferred_conn_identity_required=%v preferred_conn_identity_matches=%v preferred_conn_reuse_key_matches=%v conn_profile=%s force_preferred_conn=%v force_new_conn=%v affinity_only_reuse=%v has_previous_response_id=%v store_mode=%s store_disabled=%v store_fallback_reason=%s",
		normalizeOpenAIWSLogValue(v.RequestID),
		v.GroupID,
		v.APIKeyID,
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		v.PreferredConnInPool,
		openAIWSConnProfileSnapshotLogValue(v.PreferredConnProfile, v.PreferredConnInPool),
		v.PreferredConnAgeMS,
		v.PreferredConnIdleMS,
		v.PreferredConnLeaseCount,
		v.PreferredConnLeased,
		v.PreferredConnWaiters,
		v.PreferredConnMatchesAcquire,
		v.PreferredConnProfileMatches,
		v.PreferredConnIdentityRequired,
		v.PreferredConnIdentityMatches,
		v.PreferredConnReuseKeyMatches,
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		v.ForcePreferredConn,
		v.ForceNewConn,
		v.AffinityOnlyReuse,
		v.HasPreviousResponseID,
		normalizeOpenAIWSLogValue(v.StoreMode),
		v.StoreDisabled,
		normalizeOpenAIWSLogValue(v.StoreFallbackReason),
	)
}

type openAIWSPreferredConnDriftLog struct {
	RequestID                   string
	GroupID                     int64
	APIKeyID                    int64
	AccountID                   int64
	AccountType                 string
	SessionHash                 string
	PreferredConnID             string
	LeaseConnID                 string
	PreferredConnInPool         bool
	PreferredConnProfile        openAIWSConnProfile
	PreferredConnLeased         bool
	PreferredConnWaiters        int32
	PreferredConnMatchesAcquire bool
	ConnProfile                 openAIWSConnProfile
	ForcePreferredConn          bool
	ForceNewConn                bool
	AffinityOnlyReuse           bool
	StoreFallbackReason         string
}

func openAIWSPreferredConnDriftLogMessage(v openAIWSPreferredConnDriftLog) string {
	return fmt.Sprintf(
		"openai_ws_preferred_conn_drift temporary_diag=sticky_select remove_after_debug=true request_id=%s group_id=%d api_key_id=%d account_id=%d account_type=%s session_hash=%s preferred_conn_id=%s lease_conn_id=%s preferred_conn_in_pool=%v preferred_conn_profile=%s preferred_conn_leased=%v preferred_conn_waiters=%d preferred_conn_matches_acquire=%v conn_profile=%s force_preferred_conn=%v force_new_conn=%v affinity_only_reuse=%v store_fallback_reason=%s",
		normalizeOpenAIWSLogValue(v.RequestID),
		v.GroupID,
		v.APIKeyID,
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		truncateOpenAIWSLogValue(v.LeaseConnID, openAIWSIDValueMaxLen),
		v.PreferredConnInPool,
		openAIWSConnProfileSnapshotLogValue(v.PreferredConnProfile, v.PreferredConnInPool),
		v.PreferredConnLeased,
		v.PreferredConnWaiters,
		v.PreferredConnMatchesAcquire,
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		v.ForcePreferredConn,
		v.ForceNewConn,
		v.AffinityOnlyReuse,
		normalizeOpenAIWSLogValue(v.StoreFallbackReason),
	)
}

func shouldForceOpenAIWSPreferredConn(account *Account, preferredConnID string, snapshot openAIWSConnAcquireSnapshot, connProfile openAIWSConnProfile, forceNewConn, httpIngressWSOneShot bool) bool {
	return account != nil &&
		account.Type == AccountTypeOAuth &&
		strings.TrimSpace(preferredConnID) != "" &&
		connProfile == openAIWSConnProfileSessionBound &&
		!forceNewConn &&
		!httpIngressWSOneShot &&
		snapshot.Exists &&
		snapshot.MatchesAcquire &&
		!snapshot.Leased &&
		snapshot.Waiters == 0
}

func canFallbackOpenAIWSPreferredConnUnavailable(storeDisabled bool, previousResponseID string) bool {
	return storeDisabled && strings.TrimSpace(previousResponseID) == ""
}

func openAIWSPreviousResponseIDSource(originalPreviousID, currentPreviousID string, activeDelta, dropped bool) string {
	originalPreviousID = strings.TrimSpace(originalPreviousID)
	currentPreviousID = strings.TrimSpace(currentPreviousID)
	if currentPreviousID == "" {
		if originalPreviousID != "" || dropped {
			return "dropped"
		}
		return "none"
	}
	if originalPreviousID != "" {
		if currentPreviousID == originalPreviousID {
			return "client"
		}
		return "rewritten"
	}
	if activeDelta {
		return "session_context"
	}
	return "inferred"
}

type openAIWSDiagnosticCompletedLog struct {
	RequestID                   string
	ClientRequestID             string
	AccountID                   int64
	AccountType                 string
	ConnProfile                 openAIWSConnProfile
	ConnID                      string
	ConnReused                  bool
	ResponseID                  string
	Model                       string
	UpstreamModel               string
	Stream                      bool
	StoreMode                   string
	StoreEnabled                bool
	StoreDisabled               bool
	HasPreviousResponseID       bool
	PreviousResponseIDKind      string
	PreviousResponseIDSource    string
	OriginalPreviousIDPresent   bool
	PayloadBytes                int
	ConnAgeMs                   int64
	ConnIdleMs                  int64
	ConnLeaseCount              int64
	WriteSentMs                 int
	DurationMs                  int64
	FirstTokenMs                int
	FirstEventMs                int
	FirstTokenEvent             string
	FirstTokenAfterFirstEventMs int
	Events                      int
	TokenEvents                 int
	TerminalEvents              int
	BufferedEvents              int
	BufferedFlushed             int
	FirstEvent                  string
	LastEvent                   string
	WroteDownstream             bool
	ClientDisconnected          bool
	HTTPIngressWSOneShot        bool
	InputTokens                 int
	CacheReadTokens             int
	CacheCreationTokens         int
	OutputTokens                int
	DeltaActive                 bool
	DeltaItems                  int
	DeltaBytes                  int
	FullItems                   int
	FullBytes                   int
}

func openAIWSDiagnosticCompletedLogMessage(v openAIWSDiagnosticCompletedLog) string {
	return fmt.Sprintf(
		"openai_ws_diag_completed temporary_diag=ctx_pool_store_true remove_after_debug=true request_id=%s client_request_id=%s account_id=%d account_type=%s conn_profile=%s conn_id=%s conn_reused=%v response_id=%s model=%s upstream_model=%s stream=%v store_mode=%s store_enabled=%v store_disabled=%v has_previous_response_id=%v previous_response_id_kind=%s previous_response_id_source=%s original_previous_response_id_present=%v payload_bytes=%d conn_age_ms=%d conn_idle_ms=%d conn_lease_count=%d write_sent_ms=%d duration_ms=%d first_token_ms=%d first_event_ms=%d first_token_event=%s first_token_after_first_event_ms=%d events=%d token_events=%d terminal_events=%d buffered_events=%d buffered_flushed=%d first_event=%s last_event=%s wrote_downstream=%v client_disconnected=%v http_ingress_ws_one_shot=%v input_tokens=%d cache_read_tokens=%d cache_creation_tokens=%d output_tokens=%d delta_active=%v delta_items=%d delta_bytes=%d full_items=%d full_bytes=%d",
		normalizeOpenAIWSLogValue(v.RequestID),
		normalizeOpenAIWSLogValue(v.ClientRequestID),
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		truncateOpenAIWSLogValue(v.ResponseID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.Model),
		normalizeOpenAIWSLogValue(v.UpstreamModel),
		v.Stream,
		normalizeOpenAIWSLogValue(v.StoreMode),
		v.StoreEnabled,
		v.StoreDisabled,
		v.HasPreviousResponseID,
		normalizeOpenAIWSLogValue(v.PreviousResponseIDKind),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDSource),
		v.OriginalPreviousIDPresent,
		v.PayloadBytes,
		v.ConnAgeMs,
		v.ConnIdleMs,
		v.ConnLeaseCount,
		v.WriteSentMs,
		v.DurationMs,
		v.FirstTokenMs,
		v.FirstEventMs,
		truncateOpenAIWSLogValue(v.FirstTokenEvent, openAIWSLogValueMaxLen),
		v.FirstTokenAfterFirstEventMs,
		v.Events,
		v.TokenEvents,
		v.TerminalEvents,
		v.BufferedEvents,
		v.BufferedFlushed,
		truncateOpenAIWSLogValue(v.FirstEvent, openAIWSLogValueMaxLen),
		truncateOpenAIWSLogValue(v.LastEvent, openAIWSLogValueMaxLen),
		v.WroteDownstream,
		v.ClientDisconnected,
		v.HTTPIngressWSOneShot,
		v.InputTokens,
		v.CacheReadTokens,
		v.CacheCreationTokens,
		v.OutputTokens,
		v.DeltaActive,
		v.DeltaItems,
		v.DeltaBytes,
		v.FullItems,
		v.FullBytes,
	)
}

func logOpenAIWSDiagnosticCompleted(v openAIWSDiagnosticCompletedLog) {
	logOpenAIWSModeInfo("%s", openAIWSDiagnosticCompletedLogMessage(v))
}

func openAIWSTransportPathFromStart(v openAIWSDiagnosticStartLog) string {
	profile := openAIWSProfileUsageString(v.ConnProfile)
	if profile == "" {
		profile = "unknown"
	}
	reuse := "new"
	if v.ConnReused {
		reuse = "reused"
	}
	mode := "full_or_non_delta"
	if v.DeltaActive {
		mode = "incremental_delta"
	}
	return profile + "_ws_" + reuse + "_" + mode
}

type openAIWSReadFailLog struct {
	RequestID                string
	ClientRequestID          string
	AccountID                int64
	AccountType              string
	Model                    string
	UpstreamModel            string
	ConnProfile              openAIWSConnProfile
	ConnID                   string
	ConnReused               bool
	Transport                string
	Attempt                  int
	ConnPickMs               int64
	QueueWaitMs              int64
	ConnAgeMs                int64
	ConnIdleMs               int64
	ConnLeaseCount           int64
	PayloadBytes             int
	PreviousResponseID       string
	PreviousResponseIDKind   string
	PreviousResponseIDSource string
	StoreMode                string
	StoreEnabled             bool
	StoreDisabled            bool
	StickyAccountID          int64
	StickyAccountHit         bool
	ConnAffinityHit          bool
	PreferredConnID          string
	StoreFallbackReason      string
	SessionHash              string
	HasPromptCacheKey        bool
	HasTurnState             bool
	TurnStateLen             int
	ProxyEnabled             bool
	ProxyID                  int64
	WroteDownstream          bool
	CloseStatus              string
	CloseReason              string
	Cause                    string
	Events                   int
	TokenEvents              int
	TerminalEvents           int
	FirstEventMs             int
	FirstTokenMs             int
	FirstTokenEvent          string
	BufferedPending          int
	BufferedFlushed          int
	FirstEvent               string
	LastEvent                string
}

type openAIWSWriteRequestFailLog struct {
	RequestID                string
	ClientRequestID          string
	AccountID                int64
	AccountType              string
	Model                    string
	UpstreamModel            string
	ConnProfile              openAIWSConnProfile
	ConnID                   string
	ConnReused               bool
	Transport                string
	Attempt                  int
	ConnPickMs               int64
	QueueWaitMs              int64
	ConnAgeMs                int64
	ConnIdleMs               int64
	ConnLeaseCount           int64
	SessionIdleTTLMS         int64
	PrewritePingThresholdMS  int64
	PrewritePingAttempted    bool
	PrewritePingResult       string
	PayloadBytes             int
	PreviousResponseID       string
	PreviousResponseIDKind   string
	PreviousResponseIDSource string
	StoreMode                string
	StoreEnabled             bool
	StoreDisabled            bool
	StickyAccountID          int64
	StickyAccountHit         bool
	ConnAffinityHit          bool
	PreferredConnID          string
	StoreFallbackReason      string
	CleanupReason            string
	AllowDeltaConnReanchor   bool
	ConnReanchorBlockers     string
	SessionHash              string
	HasPromptCacheKey        bool
	HasTurnState             bool
	TurnStateLen             int
	HTTPIngressWSOneShot     bool
	ForceNewConn             bool
	AffinityOnlyReuse        bool
	HasFunctionCallOutput    bool
	ActiveDelta              bool
	DeltaItems               int
	DeltaBytes               int
	FullItems                int
	FullBytes                int
	ProxyEnabled             bool
	ProxyID                  int64
	Cause                    string
}

type openAIWSPrewritePingLog struct {
	RequestID                string
	ClientRequestID          string
	AccountID                int64
	AccountType              string
	ConnProfile              openAIWSConnProfile
	ConnID                   string
	ConnReused               bool
	ConnAgeMS                int64
	ConnIdleMS               int64
	ConnLeaseCount           int64
	SessionIdleTTLMS         int64
	PrewritePingThresholdMS  int64
	PreviousResponseID       string
	PreviousResponseIDSource string
	StoreMode                string
	StoreFallbackReason      string
	ActiveDelta              bool
	HasFunctionCallOutput    bool
	SessionHash              string
	StickyAccountID          int64
	StickyAccountHit         bool
	ConnAffinityHit          bool
	Result                   string
	Cause                    string
}

type openAIWSErrorEventLog struct {
	RequestID                 string
	ClientRequestID           string
	AccountID                 int64
	AccountType               string
	ConnProfile               openAIWSConnProfile
	ConnID                    string
	ConnReused                bool
	Transport                 string
	Attempt                   int
	ConnAgeMs                 int64
	ConnIdleMs                int64
	ConnLeaseCount            int64
	PayloadBytes              int
	ResponseID                string
	PreviousResponseID        string
	PreviousResponseIDKind    string
	PreviousResponseIDSource  string
	OriginalPreviousIDPresent bool
	StoreMode                 string
	StoreDisabled             bool
	StickyAccountID           int64
	StickyAccountHit          bool
	ConnAffinityHit           bool
	PreferredConnID           string
	StoreFallbackReason       string
	CleanupReason             string
	ActiveDelta               bool
	DeltaItems                int
	FullItems                 int
	EventIndex                int
	FallbackReason            string
	CanFallback               bool
	ErrorCode                 string
	ErrorType                 string
	ErrorMessage              string
	SessionHash               string
	HeaderSessionID           string
	HeaderConversationID      string
	SessionIDSource           string
	ConversationIDSource      string
	HTTPIngressWSOneShot      bool
	HasPromptCacheKey         bool
	HasTurnState              bool
	TurnStateLen              int
}

func openAIWSWriteRequestFailLogMessage(v openAIWSWriteRequestFailLog) string {
	return fmt.Sprintf(
		"write_request_fail request_id=%s client_request_id=%s account_id=%d account_type=%s model=%s upstream_model=%s conn_profile=%s conn_id=%s conn_reused=%v transport=%s attempt=%d conn_pick_ms=%d queue_wait_ms=%d conn_age_ms=%d conn_idle_ms=%d conn_lease_count=%d session_idle_ttl_ms=%d prewrite_ping_threshold_ms=%d prewrite_ping_attempted=%v prewrite_ping_result=%s payload_bytes=%d previous_response_id=%s previous_response_id_kind=%s previous_response_id_source=%s store_mode=%s store_enabled=%v store_disabled=%v sticky_account_id=%d sticky_account_hit=%v conn_affinity_hit=%v preferred_conn_id=%s store_fallback_reason=%s cleanup_reason=%s allow_delta_conn_reanchor=%v conn_reanchor_blockers=%s session_hash=%s has_prompt_cache_key=%v has_turn_state=%v turn_state_len=%d http_ingress_ws_one_shot=%v force_new_conn=%v affinity_only_reuse=%v has_function_call_output=%v active_delta=%v delta_items=%d delta_bytes=%d full_items=%d full_bytes=%d proxy_enabled=%v proxy_id=%d cause=%s",
		normalizeOpenAIWSLogValue(v.RequestID),
		normalizeOpenAIWSLogValue(v.ClientRequestID),
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		normalizeOpenAIWSLogValue(v.Model),
		normalizeOpenAIWSLogValue(v.UpstreamModel),
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		normalizeOpenAIWSLogValue(v.Transport),
		v.Attempt,
		v.ConnPickMs,
		v.QueueWaitMs,
		v.ConnAgeMs,
		v.ConnIdleMs,
		v.ConnLeaseCount,
		v.SessionIdleTTLMS,
		v.PrewritePingThresholdMS,
		v.PrewritePingAttempted,
		normalizeOpenAIWSLogValue(v.PrewritePingResult),
		v.PayloadBytes,
		truncateOpenAIWSLogValue(v.PreviousResponseID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDKind),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDSource),
		normalizeOpenAIWSLogValue(v.StoreMode),
		v.StoreEnabled,
		v.StoreDisabled,
		v.StickyAccountID,
		v.StickyAccountHit,
		v.ConnAffinityHit,
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.StoreFallbackReason),
		normalizeOpenAIWSLogValueNoReplace(v.CleanupReason),
		v.AllowDeltaConnReanchor,
		normalizeOpenAIWSLogValue(v.ConnReanchorBlockers),
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		v.HasPromptCacheKey,
		v.HasTurnState,
		v.TurnStateLen,
		v.HTTPIngressWSOneShot,
		v.ForceNewConn,
		v.AffinityOnlyReuse,
		v.HasFunctionCallOutput,
		v.ActiveDelta,
		v.DeltaItems,
		v.DeltaBytes,
		v.FullItems,
		v.FullBytes,
		v.ProxyEnabled,
		v.ProxyID,
		compactOpenAIWSLogValue(v.Cause, openAIWSLogValueMaxLen),
	)
}

func openAIWSPrewritePingLogMessage(v openAIWSPrewritePingLog) string {
	return fmt.Sprintf(
		"prewrite_ping request_id=%s client_request_id=%s account_id=%d account_type=%s conn_profile=%s conn_id=%s conn_reused=%v conn_age_ms=%d conn_idle_ms=%d conn_lease_count=%d session_idle_ttl_ms=%d prewrite_ping_threshold_ms=%d previous_response_id=%s previous_response_id_source=%s store_mode=%s store_fallback_reason=%s active_delta=%v has_function_call_output=%v session_hash=%s sticky_account_id=%d sticky_account_hit=%v conn_affinity_hit=%v result=%s cause=%s",
		normalizeOpenAIWSLogValue(v.RequestID),
		normalizeOpenAIWSLogValue(v.ClientRequestID),
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		v.ConnAgeMS,
		v.ConnIdleMS,
		v.ConnLeaseCount,
		v.SessionIdleTTLMS,
		v.PrewritePingThresholdMS,
		truncateOpenAIWSLogValue(v.PreviousResponseID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDSource),
		normalizeOpenAIWSLogValue(v.StoreMode),
		normalizeOpenAIWSLogValue(v.StoreFallbackReason),
		v.ActiveDelta,
		v.HasFunctionCallOutput,
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		v.StickyAccountID,
		v.StickyAccountHit,
		v.ConnAffinityHit,
		normalizeOpenAIWSLogValue(v.Result),
		compactOpenAIWSLogValue(v.Cause, openAIWSLogValueMaxLen),
	)
}

func openAIWSErrorEventLogMessage(v openAIWSErrorEventLog) string {
	return fmt.Sprintf(
		"error_event request_id=%s client_request_id=%s account_id=%d account_type=%s conn_profile=%s conn_id=%s conn_reused=%v transport=%s attempt=%d conn_age_ms=%d conn_idle_ms=%d conn_lease_count=%d payload_bytes=%d response_id=%s previous_response_id=%s previous_response_id_kind=%s previous_response_id_source=%s original_previous_response_id_present=%v store_mode=%s store_disabled=%v sticky_account_id=%d sticky_account_hit=%v conn_affinity_hit=%v preferred_conn_id=%s store_fallback_reason=%s cleanup_reason=%s active_delta=%v delta_items=%d full_items=%d idx=%d fallback_reason=%s can_fallback=%v err_code=%s err_type=%s err_message=%s session_hash=%s header_session_id=%s header_conversation_id=%s session_id_source=%s conversation_id_source=%s http_ingress_ws_one_shot=%v has_prompt_cache_key=%v has_turn_state=%v turn_state_len=%d",
		normalizeOpenAIWSLogValue(v.RequestID),
		normalizeOpenAIWSLogValue(v.ClientRequestID),
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		normalizeOpenAIWSLogValue(v.Transport),
		v.Attempt,
		v.ConnAgeMs,
		v.ConnIdleMs,
		v.ConnLeaseCount,
		v.PayloadBytes,
		truncateOpenAIWSLogValue(v.ResponseID, openAIWSIDValueMaxLen),
		truncateOpenAIWSLogValue(v.PreviousResponseID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDKind),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDSource),
		v.OriginalPreviousIDPresent,
		normalizeOpenAIWSLogValue(v.StoreMode),
		v.StoreDisabled,
		v.StickyAccountID,
		v.StickyAccountHit,
		v.ConnAffinityHit,
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.StoreFallbackReason),
		normalizeOpenAIWSLogValueNoReplace(v.CleanupReason),
		v.ActiveDelta,
		v.DeltaItems,
		v.FullItems,
		v.EventIndex,
		truncateOpenAIWSLogValue(v.FallbackReason, openAIWSLogValueMaxLen),
		v.CanFallback,
		normalizeOpenAIWSLogValue(v.ErrorCode),
		normalizeOpenAIWSLogValue(v.ErrorType),
		truncateOpenAIWSLogValue(v.ErrorMessage, openAIWSLogValueMaxLen),
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		normalizeOpenAIWSLogValue(v.HeaderSessionID),
		normalizeOpenAIWSLogValue(v.HeaderConversationID),
		normalizeOpenAIWSLogValue(v.SessionIDSource),
		normalizeOpenAIWSLogValue(v.ConversationIDSource),
		v.HTTPIngressWSOneShot,
		v.HasPromptCacheKey,
		v.HasTurnState,
		v.TurnStateLen,
	)
}

type openAIWSBindingSnapshotLog struct {
	Trigger                           string
	RequestID                         string
	GroupID                           int64
	APIKeyID                          int64
	AccountID                         int64
	AccountType                       string
	ConnID                            string
	PreviousResponseID                string
	OriginalPreviousIDPresent         bool
	PreviousBoundAccountID            int64
	PreviousBoundAccountHit           bool
	PreviousBoundAccountMatches       bool
	PreviousBoundConnID               string
	PreviousBoundConnHit              bool
	PreviousBoundConnMatchesLease     bool
	PreviousBoundConnMatchesPreferred bool
	PreviousBoundConnLastEvictReason  string
	PreviousBoundConnLastEvictAgeMS   int64
	PreferredConnID                   string
	PreferredConnMatchesLease         bool
	PreferredConnInPool               bool
	PreferredConnProfile              openAIWSConnProfile
	PreferredConnAgeMS                int64
	PreferredConnIdleMS               int64
	PreferredConnLeaseCount           int64
	PreferredConnLeased               bool
	PreferredConnWaiters              int32
	PreferredConnLastEvictReason      string
	PreferredConnLastEvictAgeMS       int64
	LeaseConnLastResponseID           string
	LeaseConnLastResponseHit          bool
	LeaseConnLastEvictReason          string
	LeaseConnLastEvictAgeMS           int64
	SessionHash                       string
	SessionContextFound               bool
	SessionContextAccountID           int64
	SessionContextAccountMatch        bool
	SessionContextConnID              string
	SessionContextLastResponseID      string
	SessionContextLastResponseMatch   bool
	SessionConnCachedAccountID        int64
	SessionConnCachedID               string
	SessionConnCachedHit              bool
	SessionConnCachedMatch            bool
	SessionConnCurrentID              string
	SessionConnCurrentHit             bool
	SessionConnCurrentMatch           bool
	CachedConnInPool                  bool
	CachedConnProfile                 openAIWSConnProfile
	CachedConnAgeMS                   int64
	CachedConnIdleMS                  int64
	CachedConnLeaseCount              int64
	CachedConnLeased                  bool
	CachedConnWaiters                 int32
	CachedConnLastResponseID          string
	CachedConnLastResponseHit         bool
	CachedConnLastEvictReason         string
	CachedConnLastEvictAgeMS          int64
	StickyAccountID                   int64
	StickyAccountHit                  bool
	StickyAccountMismatch             bool
	ConnAffinityHit                   bool
	StoreFallbackReason               string
	FallbackReason                    string
	ErrorCode                         string
	ErrorType                         string
	ErrorMessage                      string
}

func openAIWSBindingSnapshotLogMessage(v openAIWSBindingSnapshotLog) string {
	return fmt.Sprintf(
		"openai_ws_binding_snapshot temporary_diag=sticky_select remove_after_debug=true trigger=%s request_id=%s group_id=%d api_key_id=%d account_id=%d account_type=%s conn_id=%s previous_response_id=%s original_previous_response_id_present=%v previous_bound_account_id=%d previous_bound_account_hit=%v previous_bound_account_matches=%v previous_bound_conn_id=%s previous_bound_conn_hit=%v previous_bound_conn_matches_lease=%v previous_bound_conn_matches_preferred=%v previous_bound_conn_last_evict_reason=%s previous_bound_conn_last_evict_age_ms=%d preferred_conn_id=%s preferred_conn_matches_lease=%v preferred_conn_in_pool=%v preferred_conn_profile=%s preferred_conn_age_ms=%d preferred_conn_idle_ms=%d preferred_conn_lease_count=%d preferred_conn_leased=%v preferred_conn_waiters=%d preferred_conn_last_evict_reason=%s preferred_conn_last_evict_age_ms=%d lease_conn_last_response_id=%s lease_conn_last_response_hit=%v lease_conn_last_evict_reason=%s lease_conn_last_evict_age_ms=%d session_hash=%s session_context_found=%v session_context_account_id=%d session_context_account_match=%v session_context_conn_id=%s session_context_last_response_id=%s session_context_last_response_match=%v session_conn_cached_account_id=%d session_conn_cached_id=%s session_conn_cached_hit=%v session_conn_cached_match=%v session_conn_current_id=%s session_conn_current_hit=%v session_conn_current_match=%v cached_conn_in_pool=%v cached_conn_profile=%s cached_conn_age_ms=%d cached_conn_idle_ms=%d cached_conn_lease_count=%d cached_conn_leased=%v cached_conn_waiters=%d cached_conn_last_response_id=%s cached_conn_last_response_hit=%v cached_conn_last_evict_reason=%s cached_conn_last_evict_age_ms=%d sticky_account_id=%d sticky_account_hit=%v sticky_account_conflict=%v conn_affinity_hit=%v store_fallback_reason=%s fallback_reason=%s err_code=%s err_type=%s err_message=%s",
		truncateOpenAIWSLogValueNoReplace(v.Trigger, openAIWSLogValueMaxLen),
		normalizeOpenAIWSLogValue(v.RequestID),
		v.GroupID,
		v.APIKeyID,
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		truncateOpenAIWSLogValue(v.PreviousResponseID, openAIWSIDValueMaxLen),
		v.OriginalPreviousIDPresent,
		v.PreviousBoundAccountID,
		v.PreviousBoundAccountHit,
		v.PreviousBoundAccountMatches,
		truncateOpenAIWSLogValue(v.PreviousBoundConnID, openAIWSIDValueMaxLen),
		v.PreviousBoundConnHit,
		v.PreviousBoundConnMatchesLease,
		v.PreviousBoundConnMatchesPreferred,
		normalizeOpenAIWSLogValue(v.PreviousBoundConnLastEvictReason),
		v.PreviousBoundConnLastEvictAgeMS,
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		v.PreferredConnMatchesLease,
		v.PreferredConnInPool,
		openAIWSConnProfileSnapshotLogValue(v.PreferredConnProfile, v.PreferredConnInPool),
		v.PreferredConnAgeMS,
		v.PreferredConnIdleMS,
		v.PreferredConnLeaseCount,
		v.PreferredConnLeased,
		v.PreferredConnWaiters,
		normalizeOpenAIWSLogValue(v.PreferredConnLastEvictReason),
		v.PreferredConnLastEvictAgeMS,
		truncateOpenAIWSLogValue(v.LeaseConnLastResponseID, openAIWSIDValueMaxLen),
		v.LeaseConnLastResponseHit,
		normalizeOpenAIWSLogValue(v.LeaseConnLastEvictReason),
		v.LeaseConnLastEvictAgeMS,
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		v.SessionContextFound,
		v.SessionContextAccountID,
		v.SessionContextAccountMatch,
		truncateOpenAIWSLogValue(v.SessionContextConnID, openAIWSIDValueMaxLen),
		truncateOpenAIWSLogValue(v.SessionContextLastResponseID, openAIWSIDValueMaxLen),
		v.SessionContextLastResponseMatch,
		v.SessionConnCachedAccountID,
		truncateOpenAIWSLogValue(v.SessionConnCachedID, openAIWSIDValueMaxLen),
		v.SessionConnCachedHit,
		v.SessionConnCachedMatch,
		truncateOpenAIWSLogValue(v.SessionConnCurrentID, openAIWSIDValueMaxLen),
		v.SessionConnCurrentHit,
		v.SessionConnCurrentMatch,
		v.CachedConnInPool,
		openAIWSConnProfileSnapshotLogValue(v.CachedConnProfile, v.CachedConnInPool),
		v.CachedConnAgeMS,
		v.CachedConnIdleMS,
		v.CachedConnLeaseCount,
		v.CachedConnLeased,
		v.CachedConnWaiters,
		truncateOpenAIWSLogValue(v.CachedConnLastResponseID, openAIWSIDValueMaxLen),
		v.CachedConnLastResponseHit,
		normalizeOpenAIWSLogValue(v.CachedConnLastEvictReason),
		v.CachedConnLastEvictAgeMS,
		v.StickyAccountID,
		v.StickyAccountHit,
		v.StickyAccountMismatch,
		v.ConnAffinityHit,
		normalizeOpenAIWSLogValue(v.StoreFallbackReason),
		normalizeOpenAIWSLogValue(v.FallbackReason),
		normalizeOpenAIWSLogValue(v.ErrorCode),
		normalizeOpenAIWSLogValue(v.ErrorType),
		truncateOpenAIWSLogValue(strings.ReplaceAll(v.ErrorMessage, " ", "_"), openAIWSLogValueMaxLen),
	)
}

func openAIWSConnLastEvictForLog(stateStore OpenAIWSStateStore, connID string) (string, int64) {
	if stateStore == nil || strings.TrimSpace(connID) == "" {
		return "", -1
	}
	reason, age, ok := stateStore.GetConnLastEvict(connID)
	if !ok {
		return "", -1
	}
	return reason, age.Milliseconds()
}

func shouldLogOpenAIWSContinuationBindingSnapshot(decision openAIWSContinuationStoreDecision) bool {
	reason := strings.TrimSpace(decision.FallbackReason)
	if decision.StickyAccountMismatch {
		return true
	}
	switch reason {
	case "sticky_account_error", "sticky_account_miss", "sticky_account_mismatch", "conn_affinity_miss", "session_context_conn_reanchor":
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) logOpenAIWSBindingSnapshot(
	ctx context.Context,
	trigger string,
	requestID string,
	account *Account,
	stateStore OpenAIWSStateStore,
	pool *openAIWSConnPool,
	groupID int64,
	apiKeyID int64,
	sessionHash string,
	previousResponseID string,
	originalPreviousIDPresent bool,
	leaseConnID string,
	preferredConnID string,
	decision openAIWSContinuationStoreDecision,
	fallbackReason string,
	errCode string,
	errType string,
	errMessage string,
) {
	if account == nil || stateStore == nil || account.Type != AccountTypeOAuth {
		return
	}
	prevID := strings.TrimSpace(previousResponseID)
	sess := strings.TrimSpace(sessionHash)
	leaseConn := strings.TrimSpace(leaseConnID)
	preferredConn := strings.TrimSpace(preferredConnID)
	if prevID == "" && sess == "" && preferredConn == "" && leaseConn == "" {
		return
	}
	log := openAIWSBindingSnapshotLog{
		Trigger:                   trigger,
		RequestID:                 requestID,
		GroupID:                   groupID,
		APIKeyID:                  apiKeyID,
		AccountID:                 account.ID,
		AccountType:               account.Type,
		ConnID:                    leaseConn,
		PreviousResponseID:        prevID,
		OriginalPreviousIDPresent: originalPreviousIDPresent,
		PreferredConnID:           preferredConn,
		PreferredConnMatchesLease: preferredConn != "" && leaseConn != "" && preferredConn == leaseConn,
		SessionHash:               sess,
		StickyAccountID:           decision.StickyAccountID,
		StickyAccountHit:          decision.StickyAccountHit,
		StickyAccountMismatch:     decision.StickyAccountMismatch,
		ConnAffinityHit:           decision.ConnAffinityHit,
		StoreFallbackReason:       decision.FallbackReason,
		FallbackReason:            fallbackReason,
		ErrorCode:                 errCode,
		ErrorType:                 errType,
		ErrorMessage:              errMessage,
	}
	if prevID != "" {
		if accountID, err := stateStore.GetResponseAccount(ctx, groupID, apiKeyID, prevID); err == nil && accountID > 0 {
			log.PreviousBoundAccountID = accountID
			log.PreviousBoundAccountHit = true
			log.PreviousBoundAccountMatches = accountID == account.ID
		}
		if connID, ok := stateStore.GetResponseConn(groupID, apiKeyID, prevID); ok {
			connID = strings.TrimSpace(connID)
			log.PreviousBoundConnID = connID
			log.PreviousBoundConnHit = true
			log.PreviousBoundConnMatchesLease = leaseConn != "" && connID == leaseConn
			log.PreviousBoundConnMatchesPreferred = preferredConn != "" && connID == preferredConn
			log.PreviousBoundConnLastEvictReason, log.PreviousBoundConnLastEvictAgeMS = openAIWSConnLastEvictForLog(stateStore, connID)
		}
	}
	if leaseConn != "" {
		log.LeaseConnLastResponseID, log.LeaseConnLastResponseHit = stateStore.GetConnLastResponse(leaseConn)
		log.LeaseConnLastEvictReason, log.LeaseConnLastEvictAgeMS = openAIWSConnLastEvictForLog(stateStore, leaseConn)
	}
	if sess != "" {
		if cached, ok := stateStore.GetSessionContext(groupID, apiKeyID, sess); ok {
			log.SessionContextFound = true
			log.SessionContextAccountID = cached.accountID
			log.SessionContextAccountMatch = cached.accountID == account.ID
			log.SessionContextConnID = cached.connID
			log.SessionContextLastResponseID = cached.lastResponseID
			log.SessionContextLastResponseMatch = strings.TrimSpace(cached.lastResponseID) != "" &&
				strings.TrimSpace(cached.lastResponseID) == prevID
			if connID, hit := stateStore.GetSessionConn(groupID, apiKeyID, cached.accountID, sess); hit {
				log.SessionConnCachedAccountID = cached.accountID
				log.SessionConnCachedID = connID
				log.SessionConnCachedHit = true
				log.SessionConnCachedMatch = strings.TrimSpace(connID) == strings.TrimSpace(cached.connID)
			}
			if cached.connID != "" {
				log.CachedConnLastResponseID, log.CachedConnLastResponseHit = stateStore.GetConnLastResponse(cached.connID)
				log.CachedConnLastEvictReason, log.CachedConnLastEvictAgeMS = openAIWSConnLastEvictForLog(stateStore, cached.connID)
				if pool != nil {
					snapshot := pool.ConnSnapshot(cached.accountID, cached.connID)
					log.CachedConnInPool = snapshot.Exists
					log.CachedConnProfile = snapshot.Profile
					log.CachedConnAgeMS = snapshot.Age.Milliseconds()
					log.CachedConnIdleMS = snapshot.Idle.Milliseconds()
					log.CachedConnLeaseCount = snapshot.LeaseCount
					log.CachedConnLeased = snapshot.Leased
					log.CachedConnWaiters = snapshot.Waiters
				}
			}
		}
		if connID, hit := stateStore.GetSessionConn(groupID, apiKeyID, account.ID, sess); hit {
			log.SessionConnCurrentID = connID
			log.SessionConnCurrentHit = true
			log.SessionConnCurrentMatch = strings.TrimSpace(connID) == leaseConn
		}
	}
	if preferredConn != "" && pool != nil {
		snapshot := pool.ConnSnapshot(account.ID, preferredConn)
		log.PreferredConnInPool = snapshot.Exists
		log.PreferredConnProfile = snapshot.Profile
		log.PreferredConnAgeMS = snapshot.Age.Milliseconds()
		log.PreferredConnIdleMS = snapshot.Idle.Milliseconds()
		log.PreferredConnLeaseCount = snapshot.LeaseCount
		log.PreferredConnLeased = snapshot.Leased
		log.PreferredConnWaiters = snapshot.Waiters
	}
	if preferredConn != "" {
		log.PreferredConnLastEvictReason, log.PreferredConnLastEvictAgeMS = openAIWSConnLastEvictForLog(stateStore, preferredConn)
	}
	logOpenAIWSModeInfo("%s", openAIWSBindingSnapshotLogMessage(log))
}

type openAIWSMismatchErrorProbeLog struct {
	RequestID                             string
	ClientRequestID                       string
	GroupID                               int64
	APIKeyID                              int64
	AccountID                             int64
	AccountType                           string
	ConnProfile                           openAIWSConnProfile
	ConnID                                string
	ConnReused                            bool
	PreviousResponseID                    string
	PreviousResponseIDSource              string
	OriginalPreviousIDPresent             bool
	ResponseID                            string
	PreferredConnID                       string
	PreferredConnInPool                   bool
	PreferredConnAgeMS                    int64
	PreferredConnIdleMS                   int64
	PreferredConnLeaseCount               int64
	PreferredConnLeased                   bool
	PreferredConnWaiters                  int32
	PreferredConnMatchesLease             bool
	PreferredConnLastEvictReason          string
	PreferredConnLastEvictAgeMS           int64
	PreviousBoundAccountID                int64
	PreviousBoundAccountHit               bool
	PreviousBoundAccountMatches           bool
	PreviousBoundConnID                   string
	PreviousBoundConnHit                  bool
	PreviousBoundConnMatches              bool
	PreviousBoundConnLastEvictReason      string
	PreviousBoundConnLastEvictAgeMS       int64
	LeaseConnLastResponseID               string
	LeaseConnLastResponseHit              bool
	LeaseConnLastResponseMatchesPrev      bool
	LeaseConnLastEvictReason              string
	LeaseConnLastEvictAgeMS               int64
	SessionHash                           string
	SessionContextFound                   bool
	SessionContextAccountID               int64
	SessionContextAccountMatches          bool
	SessionContextConnID                  string
	SessionContextConnMatches             bool
	SessionContextLastResponseID          string
	SessionContextLastResponseMatchesPrev bool
	SessionConnCurrentID                  string
	SessionConnCurrentHit                 bool
	SessionConnCurrentMatches             bool
	CachedConnInPool                      bool
	CachedConnProfile                     openAIWSConnProfile
	CachedConnAgeMS                       int64
	CachedConnIdleMS                      int64
	CachedConnLeaseCount                  int64
	CachedConnLeased                      bool
	CachedConnWaiters                     int32
	CachedConnLastEvictReason             string
	CachedConnLastEvictAgeMS              int64
	StickyAccountID                       int64
	StickyAccountHit                      bool
	StickyAccountMismatch                 bool
	ConnAffinityHit                       bool
	StoreMode                             string
	StoreFallbackReason                   string
	ActiveDelta                           bool
	DeltaItems                            int
	FullItems                             int
	FallbackReason                        string
	CanFallback                           bool
	CanSafeFallback                       bool
	SafeFallbackReason                    string
	ErrorCode                             string
	ErrorType                             string
	ErrorMessage                          string
	WroteDownstream                       bool
}

func openAIWSMismatchErrorProbeLogMessage(v openAIWSMismatchErrorProbeLog) string {
	return fmt.Sprintf(
		"openai_ws_mismatch_error_probe temporary_diag=sticky_select remove_after_debug=true request_id=%s client_request_id=%s group_id=%d api_key_id=%d account_id=%d account_type=%s conn_profile=%s conn_id=%s conn_reused=%v previous_response_id=%s previous_response_id_source=%s original_previous_response_id_present=%v response_id=%s preferred_conn_id=%s preferred_conn_in_pool=%v preferred_conn_age_ms=%d preferred_conn_idle_ms=%d preferred_conn_lease_count=%d preferred_conn_leased=%v preferred_conn_waiters=%d preferred_conn_matches_lease=%v preferred_conn_last_evict_reason=%s preferred_conn_last_evict_age_ms=%d previous_bound_account_id=%d previous_bound_account_hit=%v previous_bound_account_matches=%v previous_bound_conn_id=%s previous_bound_conn_hit=%v previous_bound_conn_matches=%v previous_bound_conn_last_evict_reason=%s previous_bound_conn_last_evict_age_ms=%d lease_conn_last_response_id=%s lease_conn_last_response_hit=%v lease_conn_last_response_matches_previous=%v lease_conn_last_evict_reason=%s lease_conn_last_evict_age_ms=%d session_hash=%s session_context_found=%v session_context_account_id=%d session_context_account_matches=%v session_context_conn_id=%s session_context_conn_matches=%v session_context_last_response_id=%s session_context_last_response_matches_previous=%v session_conn_current_id=%s session_conn_current_hit=%v session_conn_current_matches=%v cached_conn_in_pool=%v cached_conn_profile=%s cached_conn_age_ms=%d cached_conn_idle_ms=%d cached_conn_lease_count=%d cached_conn_leased=%v cached_conn_waiters=%d cached_conn_last_evict_reason=%s cached_conn_last_evict_age_ms=%d sticky_account_id=%d sticky_account_hit=%v sticky_account_conflict=%v conn_affinity_hit=%v store_mode=%s store_fallback_reason=%s active_delta=%v delta_items=%d full_items=%d fallback_reason=%s can_fallback=%v can_safe_fallback=%v safe_fallback_reason=%s err_code=%s err_type=%s err_message=%s wrote_downstream=%v",
		normalizeOpenAIWSLogValue(v.RequestID),
		normalizeOpenAIWSLogValue(v.ClientRequestID),
		v.GroupID,
		v.APIKeyID,
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		truncateOpenAIWSLogValue(v.PreviousResponseID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDSource),
		v.OriginalPreviousIDPresent,
		truncateOpenAIWSLogValue(v.ResponseID, openAIWSIDValueMaxLen),
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		v.PreferredConnInPool,
		v.PreferredConnAgeMS,
		v.PreferredConnIdleMS,
		v.PreferredConnLeaseCount,
		v.PreferredConnLeased,
		v.PreferredConnWaiters,
		v.PreferredConnMatchesLease,
		normalizeOpenAIWSLogValue(v.PreferredConnLastEvictReason),
		v.PreferredConnLastEvictAgeMS,
		v.PreviousBoundAccountID,
		v.PreviousBoundAccountHit,
		v.PreviousBoundAccountMatches,
		truncateOpenAIWSLogValue(v.PreviousBoundConnID, openAIWSIDValueMaxLen),
		v.PreviousBoundConnHit,
		v.PreviousBoundConnMatches,
		normalizeOpenAIWSLogValue(v.PreviousBoundConnLastEvictReason),
		v.PreviousBoundConnLastEvictAgeMS,
		truncateOpenAIWSLogValue(v.LeaseConnLastResponseID, openAIWSIDValueMaxLen),
		v.LeaseConnLastResponseHit,
		v.LeaseConnLastResponseMatchesPrev,
		normalizeOpenAIWSLogValue(v.LeaseConnLastEvictReason),
		v.LeaseConnLastEvictAgeMS,
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		v.SessionContextFound,
		v.SessionContextAccountID,
		v.SessionContextAccountMatches,
		truncateOpenAIWSLogValue(v.SessionContextConnID, openAIWSIDValueMaxLen),
		v.SessionContextConnMatches,
		truncateOpenAIWSLogValue(v.SessionContextLastResponseID, openAIWSIDValueMaxLen),
		v.SessionContextLastResponseMatchesPrev,
		truncateOpenAIWSLogValue(v.SessionConnCurrentID, openAIWSIDValueMaxLen),
		v.SessionConnCurrentHit,
		v.SessionConnCurrentMatches,
		v.CachedConnInPool,
		openAIWSConnProfileSnapshotLogValue(v.CachedConnProfile, v.CachedConnInPool),
		v.CachedConnAgeMS,
		v.CachedConnIdleMS,
		v.CachedConnLeaseCount,
		v.CachedConnLeased,
		v.CachedConnWaiters,
		normalizeOpenAIWSLogValue(v.CachedConnLastEvictReason),
		v.CachedConnLastEvictAgeMS,
		v.StickyAccountID,
		v.StickyAccountHit,
		v.StickyAccountMismatch,
		v.ConnAffinityHit,
		normalizeOpenAIWSLogValue(v.StoreMode),
		normalizeOpenAIWSLogValue(v.StoreFallbackReason),
		v.ActiveDelta,
		v.DeltaItems,
		v.FullItems,
		truncateOpenAIWSLogValue(v.FallbackReason, openAIWSLogValueMaxLen),
		v.CanFallback,
		v.CanSafeFallback,
		normalizeOpenAIWSLogValue(v.SafeFallbackReason),
		normalizeOpenAIWSLogValue(v.ErrorCode),
		normalizeOpenAIWSLogValue(v.ErrorType),
		truncateOpenAIWSLogValue(strings.ReplaceAll(v.ErrorMessage, " ", "_"), openAIWSLogValueMaxLen),
		v.WroteDownstream,
	)
}

func isOpenAIWSMismatchFallbackReason(reason string) bool {
	switch strings.TrimSpace(reason) {
	case "conn_mismatch", "account_mismatch":
		return true
	default:
		return false
	}
}

func openAIWSMismatchSafeFallbackState(canFallback, wroteDownstream bool) (bool, string) {
	if !canFallback {
		return false, "fallback_not_allowed"
	}
	if wroteDownstream {
		return false, "downstream_already_written"
	}
	return true, "not_written_downstream"
}

func (s *OpenAIGatewayService) logOpenAIWSMismatchErrorProbe(
	ctx context.Context,
	requestID string,
	clientRequestID string,
	account *Account,
	stateStore OpenAIWSStateStore,
	pool *openAIWSConnPool,
	groupID int64,
	apiKeyID int64,
	sessionHash string,
	previousResponseID string,
	previousResponseIDSource string,
	originalPreviousIDPresent bool,
	responseID string,
	connProfile openAIWSConnProfile,
	connID string,
	connReused bool,
	preferredConnID string,
	storeDecision openAIWSContinuationStoreDecision,
	storeMode string,
	activeDelta bool,
	deltaItems int,
	fullItems int,
	fallbackReason string,
	canFallback bool,
	wroteDownstream bool,
	errCode string,
	errType string,
	errMessage string,
) {
	if account == nil || stateStore == nil || account.Type != AccountTypeOAuth || !isOpenAIWSMismatchFallbackReason(fallbackReason) {
		return
	}
	prevID := strings.TrimSpace(previousResponseID)
	leaseConn := strings.TrimSpace(connID)
	preferredConn := strings.TrimSpace(preferredConnID)
	sess := strings.TrimSpace(sessionHash)
	canSafeFallback, safeFallbackReason := openAIWSMismatchSafeFallbackState(canFallback, wroteDownstream)
	log := openAIWSMismatchErrorProbeLog{
		RequestID:                 requestID,
		ClientRequestID:           clientRequestID,
		GroupID:                   groupID,
		APIKeyID:                  apiKeyID,
		AccountID:                 account.ID,
		AccountType:               account.Type,
		ConnProfile:               connProfile,
		ConnID:                    leaseConn,
		ConnReused:                connReused,
		PreviousResponseID:        prevID,
		PreviousResponseIDSource:  previousResponseIDSource,
		OriginalPreviousIDPresent: originalPreviousIDPresent,
		ResponseID:                responseID,
		PreferredConnID:           preferredConn,
		PreferredConnMatchesLease: preferredConn != "" && preferredConn == leaseConn,
		SessionHash:               sess,
		StickyAccountID:           storeDecision.StickyAccountID,
		StickyAccountHit:          storeDecision.StickyAccountHit,
		StickyAccountMismatch:     storeDecision.StickyAccountMismatch,
		ConnAffinityHit:           storeDecision.ConnAffinityHit,
		StoreMode:                 storeMode,
		StoreFallbackReason:       storeDecision.FallbackReason,
		ActiveDelta:               activeDelta,
		DeltaItems:                deltaItems,
		FullItems:                 fullItems,
		FallbackReason:            fallbackReason,
		CanFallback:               canFallback,
		CanSafeFallback:           canSafeFallback,
		SafeFallbackReason:        safeFallbackReason,
		ErrorCode:                 errCode,
		ErrorType:                 errType,
		ErrorMessage:              errMessage,
		WroteDownstream:           wroteDownstream,
	}
	if prevID != "" {
		if accountID, err := stateStore.GetResponseAccount(ctx, groupID, apiKeyID, prevID); err == nil && accountID > 0 {
			log.PreviousBoundAccountID = accountID
			log.PreviousBoundAccountHit = true
			log.PreviousBoundAccountMatches = accountID == account.ID
		}
		if boundConnID, ok := stateStore.GetResponseConn(groupID, apiKeyID, prevID); ok {
			log.PreviousBoundConnID = boundConnID
			log.PreviousBoundConnHit = true
			log.PreviousBoundConnMatches = strings.TrimSpace(boundConnID) == leaseConn
			log.PreviousBoundConnLastEvictReason, log.PreviousBoundConnLastEvictAgeMS = openAIWSConnLastEvictForLog(stateStore, boundConnID)
		}
	}
	if leaseConn != "" {
		if lastResponseID, ok := stateStore.GetConnLastResponse(leaseConn); ok {
			log.LeaseConnLastResponseID = lastResponseID
			log.LeaseConnLastResponseHit = true
			log.LeaseConnLastResponseMatchesPrev = prevID != "" && strings.TrimSpace(lastResponseID) == prevID
		}
		log.LeaseConnLastEvictReason, log.LeaseConnLastEvictAgeMS = openAIWSConnLastEvictForLog(stateStore, leaseConn)
	}
	if sess != "" {
		if cached, ok := stateStore.GetSessionContext(groupID, apiKeyID, sess); ok {
			log.SessionContextFound = true
			log.SessionContextAccountID = cached.accountID
			log.SessionContextAccountMatches = cached.accountID == account.ID
			log.SessionContextConnID = cached.connID
			log.SessionContextConnMatches = strings.TrimSpace(cached.connID) == leaseConn
			log.SessionContextLastResponseID = cached.lastResponseID
			log.SessionContextLastResponseMatchesPrev = prevID != "" && strings.TrimSpace(cached.lastResponseID) == prevID
			log.CachedConnLastEvictReason, log.CachedConnLastEvictAgeMS = openAIWSConnLastEvictForLog(stateStore, cached.connID)
			if cached.accountID > 0 {
				if currentConnID, hit := stateStore.GetSessionConn(groupID, apiKeyID, cached.accountID, sess); hit {
					log.SessionConnCurrentID = currentConnID
					log.SessionConnCurrentHit = true
					log.SessionConnCurrentMatches = strings.TrimSpace(currentConnID) == leaseConn
				}
			}
			if pool != nil && cached.accountID > 0 && strings.TrimSpace(cached.connID) != "" {
				snapshot := pool.ConnSnapshot(cached.accountID, cached.connID)
				log.CachedConnInPool = snapshot.Exists
				log.CachedConnProfile = snapshot.Profile
				log.CachedConnAgeMS = snapshot.Age.Milliseconds()
				log.CachedConnIdleMS = snapshot.Idle.Milliseconds()
				log.CachedConnLeaseCount = snapshot.LeaseCount
				log.CachedConnLeased = snapshot.Leased
				log.CachedConnWaiters = snapshot.Waiters
			}
		}
		if log.SessionConnCurrentID == "" {
			if currentConnID, hit := stateStore.GetSessionConn(groupID, apiKeyID, account.ID, sess); hit {
				log.SessionConnCurrentID = currentConnID
				log.SessionConnCurrentHit = true
				log.SessionConnCurrentMatches = strings.TrimSpace(currentConnID) == leaseConn
			}
		}
	}
	if preferredConn != "" && pool != nil {
		snapshot := pool.ConnSnapshot(account.ID, preferredConn)
		log.PreferredConnInPool = snapshot.Exists
		log.PreferredConnAgeMS = snapshot.Age.Milliseconds()
		log.PreferredConnIdleMS = snapshot.Idle.Milliseconds()
		log.PreferredConnLeaseCount = snapshot.LeaseCount
		log.PreferredConnLeased = snapshot.Leased
		log.PreferredConnWaiters = snapshot.Waiters
	}
	if preferredConn != "" {
		log.PreferredConnLastEvictReason, log.PreferredConnLastEvictAgeMS = openAIWSConnLastEvictForLog(stateStore, preferredConn)
	}
	logOpenAIWSModeInfo("%s", openAIWSMismatchErrorProbeLogMessage(log))
}

func openAIWSReadFailLogMessage(v openAIWSReadFailLog) string {
	return fmt.Sprintf(
		"read_fail request_id=%s client_request_id=%s account_id=%d account_type=%s model=%s upstream_model=%s conn_profile=%s conn_id=%s conn_reused=%v transport=%s attempt=%d conn_pick_ms=%d queue_wait_ms=%d conn_age_ms=%d conn_idle_ms=%d conn_lease_count=%d payload_bytes=%d previous_response_id=%s previous_response_id_kind=%s previous_response_id_source=%s store_mode=%s store_enabled=%v store_disabled=%v sticky_account_id=%d sticky_account_hit=%v conn_affinity_hit=%v preferred_conn_id=%s store_fallback_reason=%s session_hash=%s has_prompt_cache_key=%v has_turn_state=%v turn_state_len=%d proxy_enabled=%v proxy_id=%d wrote_downstream=%v close_status=%s close_reason=%s cause=%s events=%d token_events=%d terminal_events=%d first_event_ms=%d first_token_ms=%d first_token_event=%s buffered_pending=%d buffered_flushed=%d first_event=%s last_event=%s",
		normalizeOpenAIWSLogValue(v.RequestID),
		normalizeOpenAIWSLogValue(v.ClientRequestID),
		v.AccountID,
		normalizeOpenAIWSLogValue(v.AccountType),
		normalizeOpenAIWSLogValue(v.Model),
		normalizeOpenAIWSLogValue(v.UpstreamModel),
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		normalizeOpenAIWSLogValue(v.Transport),
		v.Attempt,
		v.ConnPickMs,
		v.QueueWaitMs,
		v.ConnAgeMs,
		v.ConnIdleMs,
		v.ConnLeaseCount,
		v.PayloadBytes,
		truncateOpenAIWSLogValue(v.PreviousResponseID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDKind),
		normalizeOpenAIWSLogValue(v.PreviousResponseIDSource),
		normalizeOpenAIWSLogValue(v.StoreMode),
		v.StoreEnabled,
		v.StoreDisabled,
		v.StickyAccountID,
		v.StickyAccountHit,
		v.ConnAffinityHit,
		truncateOpenAIWSLogValue(v.PreferredConnID, openAIWSIDValueMaxLen),
		normalizeOpenAIWSLogValue(v.StoreFallbackReason),
		truncateOpenAIWSLogValue(v.SessionHash, 12),
		v.HasPromptCacheKey,
		v.HasTurnState,
		v.TurnStateLen,
		v.ProxyEnabled,
		v.ProxyID,
		v.WroteDownstream,
		normalizeOpenAIWSLogValue(v.CloseStatus),
		truncateOpenAIWSLogValue(strings.ReplaceAll(v.CloseReason, " ", "_"), openAIWSHeaderValueMaxLen),
		truncateOpenAIWSLogValue(v.Cause, openAIWSLogValueMaxLen),
		v.Events,
		v.TokenEvents,
		v.TerminalEvents,
		v.FirstEventMs,
		v.FirstTokenMs,
		truncateOpenAIWSLogValue(v.FirstTokenEvent, openAIWSLogValueMaxLen),
		v.BufferedPending,
		v.BufferedFlushed,
		truncateOpenAIWSLogValue(v.FirstEvent, openAIWSLogValueMaxLen),
		truncateOpenAIWSLogValue(v.LastEvent, openAIWSLogValueMaxLen),
	)
}

func logOpenAIWSReadFail(v openAIWSReadFailLog) {
	logOpenAIWSModeInfo("%s", openAIWSReadFailLogMessage(v))
}

func buildOpenAIWSSyntheticReadFailureEvent(responseID, model, message string, usage *OpenAIUsage) []byte {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		responseID = "resp_synthetic_ws_eof"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "upstream websocket read failed after downstream stream started"
	}
	response := map[string]any{
		"id":     responseID,
		"object": "response",
		"status": "failed",
		"output": []any{},
		"error": map[string]any{
			"type":    "upstream_error",
			"code":    "upstream_websocket_read_failed",
			"message": message,
		},
	}
	if model = strings.TrimSpace(model); model != "" {
		response["model"] = model
	}
	if usage != nil && (usage.InputTokens != 0 || usage.OutputTokens != 0 || usage.CacheCreationInputTokens != 0 || usage.CacheReadInputTokens != 0 || usage.ImageOutputTokens != 0) {
		usageBody := map[string]any{
			"input_tokens":  usage.InputTokens,
			"output_tokens": usage.OutputTokens,
			"total_tokens":  usage.InputTokens + usage.OutputTokens,
		}
		if usage.CacheReadInputTokens != 0 || usage.CacheCreationInputTokens != 0 {
			usageBody["input_tokens_details"] = map[string]any{
				"cached_tokens": usage.CacheReadInputTokens,
			}
		}
		if usage.ImageOutputTokens != 0 {
			usageBody["output_tokens_details"] = map[string]any{
				"image_tokens": usage.ImageOutputTokens,
			}
		}
		response["usage"] = usageBody
	}
	payload, err := json.Marshal(map[string]any{
		"type":     "response.failed",
		"response": response,
	})
	if err != nil {
		return []byte(`{"type":"response.failed","response":{"id":"` + responseID + `","object":"response","status":"failed","output":[],"error":{"type":"upstream_error","code":"upstream_websocket_read_failed","message":"upstream websocket read failed after downstream stream started"}}}`)
	}
	return payload
}

func openAIWSRequestLogIDs(c *gin.Context) (string, string) {
	if c == nil || c.Request == nil {
		return "", ""
	}
	requestID := ""
	clientRequestID := ""
	if c.Request.Context() != nil {
		if value, _ := c.Request.Context().Value(ctxkey.RequestID).(string); strings.TrimSpace(value) != "" {
			requestID = strings.TrimSpace(value)
		}
		if value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(value) != "" {
			clientRequestID = strings.TrimSpace(value)
		}
	}
	if clientRequestID == "" {
		clientRequestID = strings.TrimSpace(c.GetHeader("X-Client-Request-Id"))
	}
	return requestID, clientRequestID
}

func logOpenAIWSBindResponseAccountWarn(groupID, accountID int64, responseID string, err error) {
	if err == nil {
		return
	}
	logger.L().Warn(
		"openai.ws_bind_response_account_failed",
		zap.Int64("group_id", groupID),
		zap.Int64("account_id", accountID),
		zap.String("response_id", truncateOpenAIWSLogValue(responseID, openAIWSIDValueMaxLen)),
		zap.Error(err),
	)
}

func summarizeOpenAIWSReadCloseError(err error) (status string, reason string) {
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
		reasonText := strings.TrimSpace(closeErr.Reason)
		if reasonText != "" {
			closeReason = normalizeOpenAIWSLogValue(reasonText)
		}
	}
	return normalizeOpenAIWSLogValue(closeStatus), closeReason
}

func unwrapOpenAIWSDialBaseError(err error) error {
	if err == nil {
		return nil
	}
	var dialErr *openAIWSDialError
	if errors.As(err, &dialErr) && dialErr != nil && dialErr.Err != nil {
		return dialErr.Err
	}
	return err
}

func openAIWSDialRespHeaderForLog(err error, key string) string {
	var dialErr *openAIWSDialError
	if !errors.As(err, &dialErr) || dialErr == nil || dialErr.ResponseHeaders == nil {
		return "-"
	}
	return truncateOpenAIWSLogValue(dialErr.ResponseHeaders.Get(key), openAIWSHeaderValueMaxLen)
}

func classifyOpenAIWSDialError(err error) string {
	if err == nil {
		return "-"
	}
	baseErr := unwrapOpenAIWSDialBaseError(err)
	if baseErr == nil {
		return "-"
	}
	if errors.Is(baseErr, context.DeadlineExceeded) {
		return "ctx_deadline_exceeded"
	}
	if errors.Is(baseErr, context.Canceled) {
		return "ctx_canceled"
	}
	var netErr net.Error
	if errors.As(baseErr, &netErr) && netErr.Timeout() {
		return "net_timeout"
	}
	if status := coderws.CloseStatus(baseErr); status != -1 {
		return normalizeOpenAIWSLogValue(fmt.Sprintf("ws_close_%d", int(status)))
	}
	message := strings.ToLower(strings.TrimSpace(baseErr.Error()))
	switch {
	case strings.Contains(message, "handshake not finished"):
		return "handshake_not_finished"
	case strings.Contains(message, "bad handshake"):
		return "bad_handshake"
	case strings.Contains(message, "connection refused"):
		return "connection_refused"
	case strings.Contains(message, "no such host"):
		return "dns_not_found"
	case strings.Contains(message, "tls"):
		return "tls_error"
	case strings.Contains(message, "i/o timeout"):
		return "io_timeout"
	case strings.Contains(message, "context deadline exceeded"):
		return "ctx_deadline_exceeded"
	default:
		return "dial_error"
	}
}

func summarizeOpenAIWSDialError(err error) (
	statusCode int,
	dialClass string,
	closeStatus string,
	closeReason string,
	respServer string,
	respVia string,
	respCFRay string,
	respRequestID string,
) {
	dialClass = "-"
	closeStatus = "-"
	closeReason = "-"
	respServer = "-"
	respVia = "-"
	respCFRay = "-"
	respRequestID = "-"
	if err == nil {
		return
	}
	var dialErr *openAIWSDialError
	if errors.As(err, &dialErr) && dialErr != nil {
		statusCode = dialErr.StatusCode
		respServer = openAIWSDialRespHeaderForLog(err, "server")
		respVia = openAIWSDialRespHeaderForLog(err, "via")
		respCFRay = openAIWSDialRespHeaderForLog(err, "cf-ray")
		respRequestID = openAIWSDialRespHeaderForLog(err, "x-request-id")
	}
	dialClass = normalizeOpenAIWSLogValue(classifyOpenAIWSDialError(err))
	closeStatus, closeReason = summarizeOpenAIWSReadCloseError(unwrapOpenAIWSDialBaseError(err))
	return
}

func isOpenAIWSClientDisconnectError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return true
	}
	switch coderws.CloseStatus(err) {
	case coderws.StatusNormalClosure, coderws.StatusGoingAway, coderws.StatusNoStatusRcvd, coderws.StatusAbnormalClosure:
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return false
	}
	return strings.Contains(message, "failed to read frame header: eof") ||
		strings.Contains(message, "unexpected eof") ||
		strings.Contains(message, "use of closed network connection") ||
		strings.Contains(message, "connection reset by peer") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "an established connection was aborted")
}

func classifyOpenAIWSReadFallbackReason(err error) string {
	if err == nil {
		return "read_event"
	}
	switch coderws.CloseStatus(err) {
	case coderws.StatusPolicyViolation:
		return "policy_violation"
	case coderws.StatusMessageTooBig:
		return "message_too_big"
	default:
		return "read_event"
	}
}

func sortedKeys(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *OpenAIGatewayService) getOpenAIWSConnPool() *openAIWSConnPool {
	if s == nil {
		return nil
	}
	s.openaiWSPoolOnce.Do(func() {
		if s.openaiWSPool == nil {
			s.openaiWSPool = newOpenAIWSConnPool(s.cfg)
		}
	})
	return s.openaiWSPool
}

func (s *OpenAIGatewayService) getOpenAIWSPassthroughDialer() openAIWSClientDialer {
	if s == nil {
		return nil
	}
	s.openaiWSPassthroughDialerOnce.Do(func() {
		if s.openaiWSPassthroughDialer == nil {
			s.openaiWSPassthroughDialer = newDefaultOpenAIWSClientDialer()
		}
	})
	return s.openaiWSPassthroughDialer
}

func (s *OpenAIGatewayService) SnapshotOpenAIWSPoolMetrics() OpenAIWSPoolMetricsSnapshot {
	pool := s.getOpenAIWSConnPool()
	if pool == nil {
		return OpenAIWSPoolMetricsSnapshot{}
	}
	return pool.SnapshotMetrics()
}

type OpenAIWSPerformanceMetricsSnapshot struct {
	Pool      OpenAIWSPoolMetricsSnapshot      `json:"pool"`
	Retry     OpenAIWSRetryMetricsSnapshot     `json:"retry"`
	Transport OpenAIWSTransportMetricsSnapshot `json:"transport"`
}

func (s *OpenAIGatewayService) SnapshotOpenAIWSPerformanceMetrics() OpenAIWSPerformanceMetricsSnapshot {
	pool := s.getOpenAIWSConnPool()
	snapshot := OpenAIWSPerformanceMetricsSnapshot{
		Retry: s.SnapshotOpenAIWSRetryMetrics(),
	}
	if pool == nil {
		return snapshot
	}
	snapshot.Pool = pool.SnapshotMetrics()
	snapshot.Transport = pool.SnapshotTransportMetrics()
	return snapshot
}

func (s *OpenAIGatewayService) getOpenAIWSStateStore() OpenAIWSStateStore {
	if s == nil {
		return nil
	}
	s.openaiWSStateStoreOnce.Do(func() {
		if s.openaiWSStateStore == nil {
			s.openaiWSStateStore = NewOpenAIWSStateStore(s.cache)
		}
		// 连接淘汰时同步失效 conn 作用域 session/delta 状态，避免后续请求命中 stale affinity。
		store := s.openaiWSStateStore
		RegisterOpenAIWSConnEvictHook(func(connID string, reason string) {
			store.DeleteConnScopedState(connID, reason)
		})
	})
	return s.openaiWSStateStore
}

func (s *OpenAIGatewayService) openAIWSResponseStickyTTL() time.Duration {
	if s != nil && s.cfg != nil {
		seconds := s.cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return time.Hour
}

func (s *OpenAIGatewayService) openAIWSIngressPreviousResponseRecoveryEnabled() bool {
	if s != nil && s.cfg != nil {
		return s.cfg.Gateway.OpenAIWS.IngressPreviousResponseRecoveryEnabled
	}
	return true
}

func (s *OpenAIGatewayService) openAIWSReadTimeout() time.Duration {
	return s.openAIWSTransportTimeouts().Read
}

func (s *OpenAIGatewayService) openAIWSPassthroughIdleTimeout() time.Duration {
	return s.openAIWSTransportTimeouts().PassthroughIdle
}

func (s *OpenAIGatewayService) openAIWSWriteTimeout() time.Duration {
	return s.openAIWSTransportTimeouts().Write
}

func openAIWSSessionPrewritePingIdleThreshold(sessionTTL time.Duration) time.Duration {
	if sessionTTL <= 0 {
		return 0
	}
	threshold := sessionTTL * 3 / 4
	maxThreshold := 60 * time.Second
	if threshold > maxThreshold {
		return maxThreshold
	}
	return threshold
}

func shouldOpenAIWSSessionPrewritePing(account *Account, connProfile openAIWSConnProfile, lease *openAIWSConnLease, httpIngressWSOneShot bool, threshold time.Duration) bool {
	if account == nil || account.Type != AccountTypeOAuth {
		return false
	}
	if lease == nil || !lease.Reused() {
		return false
	}
	if httpIngressWSOneShot || connProfile != openAIWSConnProfileSessionBound || threshold <= 0 {
		return false
	}
	return lease.ConnIdleDuration() >= threshold
}

func (s *OpenAIGatewayService) openAIWSDialTimeout() time.Duration {
	return s.openAIWSTransportTimeouts().Dial
}

func (s *OpenAIGatewayService) openAIWSAcquireTimeout() time.Duration {
	return s.openAIWSTransportTimeouts().Acquire
}

func (s *OpenAIGatewayService) openAIWSIngressPreflightPingIdle() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.IngressPreflightPingIdleSeconds >= 0 {
		return time.Duration(s.cfg.Gateway.OpenAIWS.IngressPreflightPingIdleSeconds) * time.Second
	}
	return defaultOpenAIWSIngressPreflightPingIdle
}

func (s *OpenAIGatewayService) openAIWSTransportTimeouts() openAIWSTransportTimeouts {
	dial := defaultOpenAIWSDialTimeout
	read := defaultOpenAIWSReadTimeout
	write := defaultOpenAIWSWriteTimeout
	acquireExtra := openAIWSAcquireTimeoutExtra
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.ReadTimeoutSeconds > 0 {
		read = time.Duration(s.cfg.Gateway.OpenAIWS.ReadTimeoutSeconds) * time.Second
	}
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.WriteTimeoutSeconds > 0 {
		write = time.Duration(s.cfg.Gateway.OpenAIWS.WriteTimeoutSeconds) * time.Second
	}
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.DialTimeoutSeconds > 0 {
		dial = time.Duration(s.cfg.Gateway.OpenAIWS.DialTimeoutSeconds) * time.Second
	}
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.AcquireTimeoutExtraMS > 0 {
		acquireExtra = time.Duration(s.cfg.Gateway.OpenAIWS.AcquireTimeoutExtraMS) * time.Millisecond
	}
	return openAIWSTransportTimeouts{
		Dial:            dial,
		Acquire:         dial + acquireExtra,
		Read:            read,
		Write:           write,
		PassthroughIdle: read,
	}
}

func (s *OpenAIGatewayService) openAIWSEventFlushBatchSize() int {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.EventFlushBatchSize > 0 {
		return s.cfg.Gateway.OpenAIWS.EventFlushBatchSize
	}
	return openAIWSEventFlushBatchSizeDefault
}

func (s *OpenAIGatewayService) openAIWSEventFlushInterval() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.EventFlushIntervalMS >= 0 {
		if s.cfg.Gateway.OpenAIWS.EventFlushIntervalMS == 0 {
			return 0
		}
		return time.Duration(s.cfg.Gateway.OpenAIWS.EventFlushIntervalMS) * time.Millisecond
	}
	return openAIWSEventFlushIntervalDefault
}

func (s *OpenAIGatewayService) openAIWSPayloadLogSampleRate() float64 {
	if s != nil && s.cfg != nil {
		rate := s.cfg.Gateway.OpenAIWS.PayloadLogSampleRate
		if rate < 0 {
			return 0
		}
		if rate > 1 {
			return 1
		}
		return rate
	}
	return openAIWSPayloadLogSampleDefault
}

func (s *OpenAIGatewayService) shouldLogOpenAIWSPayloadSchema(attempt int) bool {
	// 首次尝试保留一条完整 payload_schema 便于排障。
	if attempt <= 1 {
		return true
	}
	rate := s.openAIWSPayloadLogSampleRate()
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	return rand.Float64() < rate
}

func (s *OpenAIGatewayService) shouldEmitOpenAIWSPayloadSchema(attempt int) bool {
	if !s.shouldLogOpenAIWSPayloadSchema(attempt) {
		return false
	}
	return logger.L().Core().Enabled(zap.DebugLevel)
}

func (s *OpenAIGatewayService) buildOpenAIResponsesWSURL(account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if s != nil && s.openaiWSURLBuilder != nil {
		return s.openaiWSURLBuilder(account)
	}
	var targetURL string
	switch account.Type {
	case AccountTypeOAuth:
		targetURL = chatgptCodexURL
	case AccountTypeAPIKey:
		baseURL := account.GetOpenAIBaseURL()
		if baseURL == "" {
			targetURL = openaiPlatformAPIURL
		} else {
			validatedURL, err := s.validateUpstreamBaseURL(baseURL)
			if err != nil {
				return "", err
			}
			targetURL = buildOpenAIResponsesURL(validatedURL)
		}
	default:
		targetURL = openaiPlatformAPIURL
	}

	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil {
		return "", fmt.Errorf("invalid target url: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	case "wss", "ws":
		// 保持不变
	default:
		return "", fmt.Errorf("unsupported scheme for ws: %s", parsed.Scheme)
	}
	return parsed.String(), nil
}

func (s *OpenAIGatewayService) buildOpenAIWSHeaders(
	c *gin.Context,
	account *Account,
	token string,
	decision OpenAIWSProtocolDecision,
	isCodexCLI bool,
	turnState string,
	turnMetadata string,
	promptCacheKey string,
) (http.Header, openAIWSSessionHeaderResolution) {
	headers := make(http.Header)
	headers.Set("authorization", "Bearer "+token)

	sessionResolution := resolveOpenAIWSSessionHeaders(c, promptCacheKey)
	if c != nil && c.Request != nil {
		if v := strings.TrimSpace(c.Request.Header.Get("accept-language")); v != "" {
			headers.Set("accept-language", v)
		}
	}
	// OAuth 账号：将 apiKeyID 混入 session 标识符，防止跨用户会话碰撞。
	if account != nil && account.Type == AccountTypeOAuth {
		apiKeyID := getAPIKeyIDFromContext(c)
		if sessionResolution.SessionID != "" {
			headers.Set("session_id", isolateOpenAISessionID(apiKeyID, sessionResolution.SessionID))
		}
		if sessionResolution.ConversationID != "" {
			headers.Set("conversation_id", isolateOpenAISessionID(apiKeyID, sessionResolution.ConversationID))
		}
	} else {
		if sessionResolution.SessionID != "" {
			headers.Set("session_id", sessionResolution.SessionID)
		}
		if sessionResolution.ConversationID != "" {
			headers.Set("conversation_id", sessionResolution.ConversationID)
		}
	}

	if account != nil && account.Type == AccountTypeOAuth {
		apiKeyID := getAPIKeyIDFromContext(c)
		if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
			headers.Set("chatgpt-account-id", chatgptAccountID)
		}
		setOpenAIChatGPTAccountHeaders(headers, account)
		headers.Set("originator", resolveOpenAIUpstreamOriginator(c, isCodexCLI))
		if c != nil {
			if windowID := strings.TrimSpace(c.GetHeader(openAIWSWindowIDHeader)); windowID != "" {
				headers.Set(openAIWSWindowIDHeader, isolateOpenAIWSWindowID(apiKeyID, windowID))
			}
			if betaFeatures := strings.TrimSpace(c.GetHeader("x-codex-beta-features")); betaFeatures != "" {
				headers.Set("x-codex-beta-features", betaFeatures)
			}
			if clientRequestID := strings.TrimSpace(c.GetHeader("x-client-request-id")); clientRequestID != "" {
				headers.Set("x-client-request-id", clientRequestID)
			}
			if installationID := strings.TrimSpace(c.GetHeader("x-codex-installation-id")); installationID != "" {
				headers.Set("x-codex-installation-id", installationID)
			}
		}
	}
	if state := strings.TrimSpace(turnState); state != "" {
		headers.Set(openAIWSTurnStateHeader, state)
	}
	if metadata := strings.TrimSpace(turnMetadata); metadata != "" {
		headers.Set(openAIWSTurnMetadataHeader, metadata)
	}

	betaValue := openAIWSBetaV2Value
	if decision.Transport == OpenAIUpstreamTransportResponsesWebsocket {
		betaValue = openAIWSBetaV1Value
	}
	headers.Set("OpenAI-Beta", betaValue)

	customUA := ""
	if account != nil {
		customUA = account.GetOpenAIUserAgent()
	}
	if strings.TrimSpace(customUA) != "" {
		headers.Set("user-agent", customUA)
	} else if c != nil {
		if ua := strings.TrimSpace(c.GetHeader("User-Agent")); ua != "" {
			headers.Set("user-agent", ua)
		}
	}
	if s != nil && s.cfg != nil && s.cfg.Gateway.ForceCodexCLI {
		headers.Set("user-agent", codexCLIUserAgent)
	}

	return headers, sessionResolution
}

// buildOpenAIWSNeutralHeaders 构造账号级中性握手头：不携带任何请求级身份
// （session_id/conversation_id/turn_state/window_id/x-client-request-id 等），
// 使连接可在无会话请求间安全复用；请求级 tracing 字段经消息层 client_metadata 传递。
func (s *OpenAIGatewayService) buildOpenAIWSNeutralHeaders(
	account *Account,
	token string,
	decision OpenAIWSProtocolDecision,
	isCodexCLI bool,
) http.Header {
	headers := make(http.Header)
	headers.Set("authorization", "Bearer "+token)

	if account != nil && account.Type == AccountTypeOAuth {
		if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
			headers.Set("chatgpt-account-id", chatgptAccountID)
		}
		headers.Set("originator", resolveOpenAIUpstreamOriginator(nil, isCodexCLI))
	}

	betaValue := openAIWSBetaV2Value
	if decision.Transport == OpenAIUpstreamTransportResponsesWebsocket {
		betaValue = openAIWSBetaV1Value
	}
	headers.Set("OpenAI-Beta", betaValue)

	customUA := ""
	if account != nil {
		customUA = strings.TrimSpace(account.GetOpenAIUserAgent())
	}
	if customUA != "" {
		headers.Set("user-agent", customUA)
	} else {
		headers.Set("user-agent", codexCLIUserAgent)
	}
	if s != nil && s.cfg != nil && s.cfg.Gateway.ForceCodexCLI {
		headers.Set("user-agent", codexCLIUserAgent)
	}

	return headers
}

func applyOpenAIWSFingerprintRuntimeHeaders(headers http.Header, runtime openAITLSFingerprintRuntime) {
	if headers == nil {
		return
	}
	if runtime.UpstreamUserAgent != "" {
		headers.Set("user-agent", runtime.UpstreamUserAgent)
	}
	if runtime.UpstreamOriginator != "" {
		headers.Set("originator", runtime.UpstreamOriginator)
	}
}

func (s *OpenAIGatewayService) buildOpenAIWSCreatePayload(reqBody map[string]any, account *Account) map[string]any {
	// OpenAI WS Mode 协议：response.create 字段与 HTTP /responses 基本一致。
	// 保留 stream 字段（与 Codex CLI 一致），仅移除 background。
	payload := make(map[string]any, len(reqBody)+1)
	for k, v := range reqBody {
		payload[k] = v
	}

	delete(payload, "background")
	stripOpenAIWSCreatePayloadUnsupportedFields(payload)
	extractSystemMessagesFromInput(payload)
	if _, exists := payload["stream"]; !exists {
		payload["stream"] = true
	}
	payload["type"] = "response.create"

	trimOpenAIStoreFalseReasoningItems(payload)
	return payload
}

func setOpenAIWSClientMetadataField(payload map[string]any, key string, value string) {
	if len(payload) == 0 {
		return
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}

	switch existing := payload["client_metadata"].(type) {
	case map[string]any:
		existing[key] = value
		payload["client_metadata"] = existing
	case map[string]string:
		next := make(map[string]any, len(existing)+1)
		for k, v := range existing {
			next[k] = v
		}
		next[key] = value
		payload["client_metadata"] = next
	default:
		payload["client_metadata"] = map[string]any{
			key: value,
		}
	}
}

func setOpenAIWSTurnMetadata(payload map[string]any, turnMetadata string) {
	setOpenAIWSClientMetadataField(payload, openAIWSTurnMetadataHeader, turnMetadata)
}

func (s *OpenAIGatewayService) attachOpenAIWSOAuthClientMetadata(payload map[string]any, c *gin.Context) {
	if len(payload) == 0 || c == nil {
		return
	}
	setOpenAIWSClientMetadataField(payload, "x-codex-installation-id", c.GetHeader("x-codex-installation-id"))
	setOpenAIWSClientMetadataField(payload, "x-codex-beta-features", c.GetHeader("x-codex-beta-features"))
	setOpenAIWSClientMetadataField(payload, "x-client-request-id", c.GetHeader("x-client-request-id"))
	if windowID := strings.TrimSpace(c.GetHeader(openAIWSWindowIDHeader)); windowID != "" {
		setOpenAIWSClientMetadataField(payload, openAIWSWindowIDHeader, isolateOpenAIWSWindowID(getAPIKeyIDFromContext(c), windowID))
	}
}

type openAIWSContinuationStoreDecision struct {
	StoreMode                  string
	StoreEnabled               bool
	StoreDisabled              bool
	StickyAccountID            int64
	StickyAccountHit           bool
	StickyAccountMismatch      bool
	ConnAffinityHit            bool
	DroppedPreviousResponseID  bool
	OriginalPreviousResponseID string
	PreferredConnID            string
	FallbackReason             string
	UnsafeToolContinuation     bool
}

func (d openAIWSContinuationStoreDecision) unsafeToolContinuationError() error {
	if !d.UnsafeToolContinuation {
		return nil
	}
	reason := strings.TrimSpace(d.FallbackReason)
	if reason == "" {
		reason = "continuation_binding_unavailable"
	}
	return fmt.Errorf("tool continuation requires previous_response_id binding: %s", reason)
}

func openAIWSActiveDeltaContextPayloadRaw(payload map[string]any, decision openAIWSContinuationStoreDecision) []byte {
	if len(payload) == 0 {
		return payloadAsJSONBytes(payload)
	}
	if decision.StoreMode != openAIWSStoreModeIncremental || !decision.StickyAccountHit || !decision.ConnAffinityHit {
		return payloadAsJSONBytes(payload)
	}
	contextPayload := make(map[string]any, len(payload))
	for key, value := range payload {
		contextPayload[key] = value
	}
	contextPayload["store"] = false
	return payloadAsJSONBytes(contextPayload)
}

func openAIWSPayloadStoreEnabled(payload map[string]any) bool {
	if len(payload) == 0 {
		return false
	}
	rawStore, ok := payload["store"]
	if !ok {
		return false
	}
	storeEnabled, ok := rawStore.(bool)
	return ok && storeEnabled
}

func openAIWSPayloadStoreDisabled(payload map[string]any) bool {
	if len(payload) == 0 {
		return false
	}
	rawStore, ok := payload["store"]
	if !ok {
		return false
	}
	storeEnabled, ok := rawStore.(bool)
	return ok && !storeEnabled
}

func (s *OpenAIGatewayService) resolveOpenAIWSContinuationStoreDecision(
	ctx context.Context,
	payload map[string]any,
	account *Account,
	stateStore OpenAIWSStateStore,
	groupID int64,
	apiKeyID int64,
) openAIWSContinuationStoreDecision {
	previousResponseID := openAIWSPayloadString(payload, "previous_response_id")
	decision := openAIWSContinuationStoreDecision{
		StoreMode:                  openAIWSStoreModeFull,
		StoreEnabled:               openAIWSPayloadStoreEnabled(payload),
		StoreDisabled:              openAIWSPayloadStoreDisabled(payload),
		OriginalPreviousResponseID: previousResponseID,
	}

	if account == nil || account.Type != AccountTypeOAuth {
		if previousResponseID != "" && stateStore != nil {
			if connID, ok := stateStore.GetResponseConn(groupID, apiKeyID, previousResponseID); ok {
				decision.PreferredConnID = connID
				decision.ConnAffinityHit = true
			}
		}
		if decision.StoreEnabled && previousResponseID != "" {
			decision.StoreMode = openAIWSStoreModeIncremental
		}
		return decision
	}

	if previousResponseID == "" {
		payload["store"] = false
		decision.StoreEnabled = false
		decision.StoreDisabled = true
		decision.FallbackReason = "missing_previous_response_id"
		return decision
	}

	dropToFullCreate := func(reason string) openAIWSContinuationStoreDecision {
		if HasFunctionCallOutput(payload) {
			decision.UnsafeToolContinuation = true
			decision.FallbackReason = reason
			return decision
		}
		delete(payload, "previous_response_id")
		payload["store"] = false
		decision.StoreMode = openAIWSStoreModeFull
		decision.StoreEnabled = false
		decision.StoreDisabled = true
		decision.DroppedPreviousResponseID = true
		decision.PreferredConnID = ""
		decision.ConnAffinityHit = false
		decision.FallbackReason = reason
		return decision
	}
	if stateStore == nil {
		return dropToFullCreate("state_store_missing")
	}

	stickyAccountID, err := stateStore.GetResponseAccount(ctx, groupID, apiKeyID, previousResponseID)
	decision.StickyAccountID = stickyAccountID
	if err != nil {
		return dropToFullCreate("sticky_account_error")
	}
	if stickyAccountID <= 0 {
		return dropToFullCreate("sticky_account_miss")
	}
	if stickyAccountID != account.ID {
		decision.StickyAccountMismatch = true
		return dropToFullCreate("sticky_account_mismatch")
	}
	decision.StickyAccountHit = true

	if connID, ok := stateStore.GetResponseConn(groupID, apiKeyID, previousResponseID); ok {
		decision.PreferredConnID = connID
		decision.ConnAffinityHit = true
	} else {
		payload["store"] = false
		decision.StoreMode = openAIWSStoreModeIncremental
		decision.StoreEnabled = false
		decision.StoreDisabled = true
		decision.FallbackReason = "conn_affinity_miss"
		return decision
	}

	// OAuth Responses continuation can remain incremental while still forcing
	// store=false. previous_response_id + sticky conn affinity are sufficient
	// for this path; re-enabling store trips upstream validation.
	payload["store"] = false
	decision.StoreMode = openAIWSStoreModeIncremental
	decision.StoreEnabled = false
	decision.StoreDisabled = true
	decision.StickyAccountHit = true
	return decision
}

func setOpenAIWSRawPayloadStore(payload []byte, store bool) ([]byte, error) {
	if len(payload) == 0 {
		return payload, nil
	}
	updated, err := sjson.SetBytes(payload, "store", store)
	if err == nil {
		return updated, nil
	}

	reqBody := make(map[string]any)
	if unmarshalErr := json.Unmarshal(payload, &reqBody); unmarshalErr != nil {
		return nil, err
	}
	reqBody["store"] = store
	rebuilt, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		return nil, marshalErr
	}
	return rebuilt, nil
}

func forceOpenAIWSRawPayloadFullCreate(payload []byte) ([]byte, bool, error) {
	updated, removed, err := dropPreviousResponseIDFromRawPayload(payload)
	if err != nil {
		return payload, false, err
	}
	if gjson.GetBytes(updated, "previous_response_id").Exists() {
		return payload, false, errors.New("previous_response_id was not removed")
	}
	updated, err = setOpenAIWSRawPayloadStore(updated, false)
	if err != nil {
		return payload, false, err
	}
	return updated, removed, nil
}

func (s *OpenAIGatewayService) resolveOpenAIWSContinuationStoreDecisionRaw(
	ctx context.Context,
	payload []byte,
	account *Account,
	stateStore OpenAIWSStateStore,
	groupID int64,
	apiKeyID int64,
) ([]byte, openAIWSContinuationStoreDecision, error) {
	return s.resolveOpenAIWSContinuationStoreDecisionRawWithOptions(ctx, payload, account, stateStore, groupID, apiKeyID, openAIWSContinuationStoreDecisionOptions{})
}

type openAIWSContinuationStoreDecisionOptions struct {
	AllowLiveRelayContinuation bool
}

func (s *OpenAIGatewayService) resolveOpenAIWSContinuationStoreDecisionRawWithOptions(
	ctx context.Context,
	payload []byte,
	account *Account,
	stateStore OpenAIWSStateStore,
	groupID int64,
	apiKeyID int64,
	options openAIWSContinuationStoreDecisionOptions,
) ([]byte, openAIWSContinuationStoreDecision, error) {
	previousResponseID := openAIWSPayloadStringFromRaw(payload, "previous_response_id")
	rawStore := gjson.GetBytes(payload, "store")
	decision := openAIWSContinuationStoreDecision{
		StoreMode:                  openAIWSStoreModeFull,
		StoreEnabled:               rawStore.Exists() && rawStore.Type == gjson.True,
		StoreDisabled:              rawStore.Exists() && rawStore.Type == gjson.False,
		OriginalPreviousResponseID: previousResponseID,
	}

	if account == nil || account.Type != AccountTypeOAuth {
		if previousResponseID != "" && stateStore != nil {
			if connID, ok := stateStore.GetResponseConn(groupID, apiKeyID, previousResponseID); ok {
				decision.PreferredConnID = connID
				decision.ConnAffinityHit = true
			}
		}
		if decision.StoreEnabled && previousResponseID != "" {
			decision.StoreMode = openAIWSStoreModeIncremental
		}
		return payload, decision, nil
	}

	if previousResponseID == "" {
		updated, err := setOpenAIWSRawPayloadStore(payload, false)
		if err != nil {
			return payload, decision, err
		}
		decision.StoreEnabled = false
		decision.StoreDisabled = true
		decision.FallbackReason = "missing_previous_response_id"
		return updated, decision, nil
	}

	if options.AllowLiveRelayContinuation {
		updated, err := setOpenAIWSRawPayloadStore(payload, false)
		if err != nil {
			return payload, decision, err
		}
		decision.StoreMode = openAIWSStoreModeIncremental
		decision.StoreEnabled = false
		decision.StoreDisabled = true
		if account != nil {
			decision.StickyAccountID = account.ID
		}
		decision.StickyAccountHit = true
		decision.ConnAffinityHit = true
		decision.FallbackReason = ""
		return updated, decision, nil
	}

	dropToFullCreate := func(reason string) ([]byte, openAIWSContinuationStoreDecision, error) {
		if HasToolContinuationOutputInRawPayload(payload) {
			decision.UnsafeToolContinuation = true
			decision.FallbackReason = reason
			return payload, decision, nil
		}
		updated, removed, err := forceOpenAIWSRawPayloadFullCreate(payload)
		if err != nil {
			return payload, decision, err
		}
		decision.StoreMode = openAIWSStoreModeFull
		decision.StoreEnabled = false
		decision.StoreDisabled = true
		decision.DroppedPreviousResponseID = removed
		decision.PreferredConnID = ""
		decision.ConnAffinityHit = false
		decision.FallbackReason = reason
		return updated, decision, nil
	}
	if stateStore == nil {
		return dropToFullCreate("state_store_missing")
	}

	stickyAccountID, err := stateStore.GetResponseAccount(ctx, groupID, apiKeyID, previousResponseID)
	decision.StickyAccountID = stickyAccountID
	if err != nil {
		return dropToFullCreate("sticky_account_error")
	}
	if stickyAccountID <= 0 {
		return dropToFullCreate("sticky_account_miss")
	}
	if stickyAccountID != account.ID {
		decision.StickyAccountMismatch = true
		return dropToFullCreate("sticky_account_mismatch")
	}
	decision.StickyAccountHit = true

	if connID, ok := stateStore.GetResponseConn(groupID, apiKeyID, previousResponseID); ok {
		decision.PreferredConnID = connID
		decision.ConnAffinityHit = true
	} else {
		updated, err := setOpenAIWSRawPayloadStore(payload, false)
		if err != nil {
			return payload, decision, err
		}
		decision.StoreMode = openAIWSStoreModeIncremental
		decision.StoreEnabled = false
		decision.StoreDisabled = true
		decision.FallbackReason = "conn_affinity_miss"
		return updated, decision, nil
	}

	updated, err := setOpenAIWSRawPayloadStore(payload, false)
	if err != nil {
		return payload, decision, err
	}
	decision.StoreMode = openAIWSStoreModeIncremental
	decision.StoreEnabled = false
	decision.StoreDisabled = true
	decision.StickyAccountHit = true
	decision.FallbackReason = ""
	return updated, decision, nil
}

func (s *OpenAIGatewayService) shouldUseOpenAIHTTPIngressWSOneShot(c *gin.Context, payload map[string]any, previousResponseID, promptCacheKey string) bool {
	if !s.openAIHTTPIngressUpstreamWSEnabled() || GetOpenAIClientTransport(c) != OpenAIClientTransportHTTP {
		return false
	}
	if c != nil && c.Request != nil {
		if strings.TrimSpace(c.GetHeader("session_id")) != "" || strings.TrimSpace(c.GetHeader("conversation_id")) != "" {
			return false
		}
	}
	if strings.TrimSpace(previousResponseID) != "" || strings.TrimSpace(promptCacheKey) != "" {
		return false
	}
	return strings.TrimSpace(openAIWSPayloadString(payload, "response_id")) == ""
}

func (s *OpenAIGatewayService) isOpenAIWSStoreDisabledInRequestRaw(reqBody []byte, account *Account) bool {
	if len(reqBody) == 0 {
		return false
	}
	storeValue := gjson.GetBytes(reqBody, "store")
	if !storeValue.Exists() {
		return false
	}
	if storeValue.Type != gjson.True && storeValue.Type != gjson.False {
		return false
	}
	return !storeValue.Bool()
}

func (s *OpenAIGatewayService) openAIWSStoreDisabledConnMode() string {
	if s == nil || s.cfg == nil {
		return openAIWSStoreDisabledConnModeStrict
	}
	mode := strings.ToLower(strings.TrimSpace(s.cfg.Gateway.OpenAIWS.StoreDisabledConnMode))
	switch mode {
	case openAIWSStoreDisabledConnModeStrict, openAIWSStoreDisabledConnModeAdaptive, openAIWSStoreDisabledConnModeOff:
		return mode
	case "":
		// 兼容旧配置：仅配置了布尔开关时按旧语义推导。
		if s.cfg.Gateway.OpenAIWS.StoreDisabledForceNewConn {
			return openAIWSStoreDisabledConnModeStrict
		}
		return openAIWSStoreDisabledConnModeOff
	default:
		return openAIWSStoreDisabledConnModeStrict
	}
}

func shouldForceNewConnOnStoreDisabled(mode, lastFailureReason string) bool {
	switch mode {
	case openAIWSStoreDisabledConnModeOff:
		return false
	case openAIWSStoreDisabledConnModeAdaptive:
		reason := strings.TrimPrefix(strings.TrimSpace(lastFailureReason), "prewarm_")
		switch reason {
		case "policy_violation", "message_too_big", "auth_failed", "write_request", "write":
			return true
		default:
			return false
		}
	default:
		return true
	}
}

func shouldForceNewConnOnRecoveredFullReplay(lastFailureReason string) bool {
	reason := strings.TrimPrefix(strings.TrimSpace(lastFailureReason), "prewarm_")
	return reason == "previous_response_not_found"
}

// openAIWSHasDeltaReanchorTarget reports whether a strict-delta connection
// reanchor can actually engage for this session: it requires a cached session
// context bound to the same account with a prior response to anchor onto.
// Cold sessions (no cached context) have nothing to reanchor, so they must fall
// through to neutral idle-connection reuse instead of forcing a new
// session-bound connection.
func openAIWSHasDeltaReanchorTarget(
	stateStore OpenAIWSStateStore,
	groupID int64,
	apiKeyID int64,
	sessionHash string,
	accountID int64,
) bool {
	if stateStore == nil || sessionHash == "" {
		return false
	}
	cached, ok := stateStore.GetSessionContext(groupID, apiKeyID, sessionHash)
	if !ok {
		return false
	}
	return cached.accountID == accountID && strings.TrimSpace(cached.lastResponseID) != ""
}

func openAIWSDeltaConnReanchorBlockers(
	sessionPreemptedPrevious bool,
	attempt int,
	lastFailureReason string,
	httpIngressWSOneShot bool,
	account *Account,
	stateStore OpenAIWSStateStore,
	storeDisabled bool,
	previousResponseID string,
	sessionHash string,
	hasFunctionCallOutput bool,
	hasReanchorTarget bool,
) string {
	blockers := make([]string, 0, 8)
	if sessionPreemptedPrevious {
		blockers = append(blockers, "session_preempted")
	}
	if attempt > 2 {
		blockers = append(blockers, "attempt_gt_2")
	}
	reason := strings.TrimSpace(lastFailureReason)
	if reason != "" && reason != "write_request" && reason != "write" {
		blockers = append(blockers, "last_failure_"+sanitizeOpenAIWSLogToken(reason))
	}
	if httpIngressWSOneShot {
		blockers = append(blockers, "http_ingress_one_shot")
	}
	if account == nil || account.Type != AccountTypeOAuth {
		blockers = append(blockers, "account_not_oauth")
	}
	if stateStore == nil {
		blockers = append(blockers, "state_store_missing")
	}
	if !storeDisabled {
		blockers = append(blockers, "store_not_disabled")
	}
	if strings.TrimSpace(previousResponseID) != "" {
		blockers = append(blockers, "has_previous_response_id")
	}
	if strings.TrimSpace(sessionHash) == "" {
		blockers = append(blockers, "missing_session_hash")
	}
	if hasFunctionCallOutput {
		blockers = append(blockers, "has_function_call_output")
	}
	if len(blockers) == 0 && !hasReanchorTarget {
		blockers = append(blockers, "missing_reanchor_target")
	}
	if len(blockers) == 0 {
		return "-"
	}
	return strings.Join(blockers, ",")
}

func populateOpenAIWSDeltaShadowBindingDiagnostics(
	input *openAIWSDeltaShadowInput,
	stateStore OpenAIWSStateStore,
	pool *openAIWSConnPool,
	groupID int64,
	apiKeyID int64,
	sessionHash string,
	currentAccountID int64,
	cached openAIWSSessionContextValue,
	cachedFound bool,
) {
	if input == nil {
		return
	}
	input.GroupID = groupID
	input.APIKeyID = apiKeyID
	input.SessionHash = sessionHash
	if stateStore == nil || strings.TrimSpace(sessionHash) == "" || !cachedFound {
		return
	}
	if cached.accountID > 0 {
		if connID, ok := stateStore.GetSessionConn(groupID, apiKeyID, cached.accountID, sessionHash); ok {
			input.CachedSessionConnID = connID
		}
	}
	if currentAccountID > 0 {
		if connID, ok := stateStore.GetSessionConn(groupID, apiKeyID, currentAccountID, sessionHash); ok {
			input.CurrentSessionConnID = connID
		}
	}
	cachedConnID := strings.TrimSpace(cached.connID)
	if cachedConnID == "" {
		return
	}
	if responseID, ok := stateStore.GetConnLastResponse(cachedConnID); ok {
		input.CachedConnLastResponseID = responseID
	}
	if pool == nil || cached.accountID <= 0 {
		return
	}
	snapshot := pool.ConnSnapshot(cached.accountID, cachedConnID)
	input.CachedConnInPool = snapshot.Exists
	input.CachedConnProfile = snapshot.Profile
	input.CachedConnAgeMS = snapshot.Age.Milliseconds()
	input.CachedConnIdleMS = snapshot.Idle.Milliseconds()
	input.CachedConnLeaseCount = snapshot.LeaseCount
	input.CachedConnLeased = snapshot.Leased
	input.CachedConnWaiters = snapshot.Waiters
}

func logOpenAIWSResponseStickyBind(groupID int64, apiKeyID int64, accountID int64, accountType string, responseID string, connID string, ttl time.Duration, accountErr error) {
	errText := "-"
	if accountErr != nil {
		errText = compactOpenAIWSLogValue(accountErr.Error(), openAIWSLogValueMaxLen)
	}
	logOpenAIWSModeInfoDirect(
		"response_sticky_bind temporary_diag=sticky_bind remove_after_debug=true group_id=%d api_key_id=%d account_id=%d account_type=%s response_id=%s conn_id=%s conn_bound=%v ttl_seconds=%d account_bind_error=%s",
		groupID,
		apiKeyID,
		accountID,
		normalizeOpenAIWSLogValue(accountType),
		truncateOpenAIWSLogValue(responseID, openAIWSIDValueMaxLen),
		truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
		strings.TrimSpace(connID) != "",
		int64(ttl.Seconds()),
		normalizeOpenAIWSLogValue(errText),
	)
}

func logOpenAIWSSessionConnBind(groupID int64, apiKeyID int64, accountID int64, accountType string, sessionHash string, connID string, ttl time.Duration) {
	logOpenAIWSModeInfoDirect(
		"session_conn_bind temporary_diag=sticky_bind remove_after_debug=true group_id=%d api_key_id=%d account_id=%d account_type=%s session=%s conn_id=%s ttl_seconds=%d",
		groupID,
		apiKeyID,
		accountID,
		normalizeOpenAIWSLogValue(accountType),
		truncateOpenAIWSLogValue(sessionHash, 12),
		truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
		int64(ttl.Seconds()),
	)
}

func logOpenAIWSSessionContextBind(
	groupID int64,
	apiKeyID int64,
	accountID int64,
	accountType string,
	sessionHash string,
	connID string,
	responseID string,
	ttl time.Duration,
	inputCount int,
	materializedCount int,
	outputCaptured bool,
	rawClientEquivalent bool,
) {
	logOpenAIWSModeInfoDirect(
		"session_context_bind temporary_diag=sticky_bind remove_after_debug=true group_id=%d api_key_id=%d account_id=%d account_type=%s session=%s conn_id=%s response_id=%s ttl_seconds=%d input_count=%d materialized_count=%d output_captured=%v raw_client_equiv=%v",
		groupID,
		apiKeyID,
		accountID,
		normalizeOpenAIWSLogValue(accountType),
		truncateOpenAIWSLogValue(sessionHash, 12),
		truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
		truncateOpenAIWSLogValue(responseID, openAIWSIDValueMaxLen),
		int64(ttl.Seconds()),
		inputCount,
		materializedCount,
		outputCaptured,
		rawClientEquivalent,
	)
}

func sanitizeOpenAIWSLogToken(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "-"
	}
	var b strings.Builder
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := b.String()
	if out == "" {
		return "-"
	}
	return out
}

func shouldUseOpenAIWSNeutralForColdSession(
	account *Account,
	httpIngressWSOneShot bool,
	storeDisabled bool,
	previousResponseID string,
	sessionHash string,
	turnState string,
	turnMetadata string,
	payload map[string]any,
) bool {
	if account == nil || account.Type != AccountTypeOAuth {
		return false
	}
	if httpIngressWSOneShot || !storeDisabled {
		return false
	}
	if strings.TrimSpace(previousResponseID) != "" || strings.TrimSpace(sessionHash) == "" {
		return false
	}
	if strings.TrimSpace(turnState) != "" || strings.TrimSpace(turnMetadata) != "" {
		return false
	}
	signals := AnalyzeToolContinuationSignals(payload)
	if !signals.HasFunctionCallOutput {
		return true
	}
	return signals.HasToolCallContext
}

func shouldForceNewConnOnHTTPIngressWSOneShotRetry(lastFailureReason string) bool {
	reason := strings.TrimSpace(lastFailureReason)
	if reason == "" || strings.HasPrefix(reason, "prewarm_") {
		return false
	}
	switch reason {
	case "read_event", "write_request", "write":
		return true
	default:
		return false
	}
}

func dropPreviousResponseIDFromRawPayload(payload []byte) ([]byte, bool, error) {
	return dropPreviousResponseIDFromRawPayloadWithDeleteFn(payload, sjson.DeleteBytes)
}

func dropPreviousResponseIDFromRawPayloadWithDeleteFn(
	payload []byte,
	deleteFn func([]byte, string) ([]byte, error),
) ([]byte, bool, error) {
	if len(payload) == 0 {
		return payload, false, nil
	}
	if !gjson.GetBytes(payload, "previous_response_id").Exists() {
		return payload, false, nil
	}
	if deleteFn == nil {
		deleteFn = sjson.DeleteBytes
	}

	updated := payload
	for i := 0; i < openAIWSMaxPrevResponseIDDeletePasses &&
		gjson.GetBytes(updated, "previous_response_id").Exists(); i++ {
		next, err := deleteFn(updated, "previous_response_id")
		if err != nil {
			return payload, false, err
		}
		updated = next
	}
	return updated, !gjson.GetBytes(updated, "previous_response_id").Exists(), nil
}

func setPreviousResponseIDToRawPayload(payload []byte, previousResponseID string) ([]byte, error) {
	normalizedPrevID := strings.TrimSpace(previousResponseID)
	if len(payload) == 0 || normalizedPrevID == "" {
		return payload, nil
	}
	updated, err := sjson.SetBytes(payload, "previous_response_id", normalizedPrevID)
	if err == nil {
		return updated, nil
	}

	var reqBody map[string]any
	if unmarshalErr := json.Unmarshal(payload, &reqBody); unmarshalErr != nil {
		return nil, err
	}
	reqBody["previous_response_id"] = normalizedPrevID
	rebuilt, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		return nil, marshalErr
	}
	return rebuilt, nil
}

func shouldInferIngressFunctionCallOutputPreviousResponseID(
	storeDisabled bool,
	turn int,
	signals ToolContinuationSignals,
	currentPreviousResponseID string,
	expectedPreviousResponseID string,
) bool {
	if !storeDisabled || turn <= 1 || !signals.HasFunctionCallOutput {
		return false
	}
	if strings.TrimSpace(currentPreviousResponseID) != "" {
		return false
	}
	if signals.HasFunctionCallOutputMissingCallID {
		return false
	}
	// If the client already sent the actual tool-call context, treat this as
	// a full replay / self-contained continuation payload rather than
	// downgrading it into an inferred delta continuation. item_reference alone
	// is not enough on the store=false WS path: it still needs a valid prior
	// response anchor so upstream can resolve the referenced function_call.
	if signals.HasToolCallContext {
		return false
	}
	return strings.TrimSpace(expectedPreviousResponseID) != ""
}

func alignStoreDisabledPreviousResponseID(
	payload []byte,
	expectedPreviousResponseID string,
) ([]byte, bool, error) {
	if len(payload) == 0 {
		return payload, false, nil
	}
	expected := strings.TrimSpace(expectedPreviousResponseID)
	if expected == "" {
		return payload, false, nil
	}
	current := openAIWSPayloadStringFromRaw(payload, "previous_response_id")
	if current == "" || current == expected {
		return payload, false, nil
	}

	withoutPrev, removed, dropErr := dropPreviousResponseIDFromRawPayload(payload)
	if dropErr != nil {
		return payload, false, dropErr
	}
	if !removed {
		return payload, false, nil
	}
	updated, setErr := setPreviousResponseIDToRawPayload(withoutPrev, expected)
	if setErr != nil {
		return payload, false, setErr
	}
	return updated, true, nil
}

func cloneOpenAIWSPayloadBytes(payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	cloned := make([]byte, len(payload))
	copy(cloned, payload)
	return cloned
}

func cloneOpenAIWSRawMessages(items []json.RawMessage) []json.RawMessage {
	if items == nil {
		return nil
	}
	cloned := make([]json.RawMessage, 0, len(items))
	for idx := range items {
		cloned = append(cloned, json.RawMessage(cloneOpenAIWSPayloadBytes(items[idx])))
	}
	return cloned
}

func normalizeOpenAIWSJSONForCompare(raw []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("json is empty")
	}
	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return nil, err
	}
	return json.Marshal(decoded)
}

func normalizeOpenAIWSJSONForCompareOrRaw(raw []byte) []byte {
	normalized, err := normalizeOpenAIWSJSONForCompare(raw)
	if err != nil {
		return bytes.TrimSpace(raw)
	}
	return normalized
}

func normalizeOpenAIWSPayloadWithoutInputAndPreviousResponseID(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("payload is empty")
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	delete(decoded, "input")
	delete(decoded, "previous_response_id")
	return json.Marshal(decoded)
}

func openAIWSExtractNormalizedInputSequence(payload []byte) ([]json.RawMessage, bool, error) {
	if len(payload) == 0 {
		return nil, false, nil
	}
	inputValue := gjson.GetBytes(payload, "input")
	if !inputValue.Exists() {
		return nil, false, nil
	}
	if inputValue.Type == gjson.JSON {
		raw := strings.TrimSpace(inputValue.Raw)
		if strings.HasPrefix(raw, "[") {
			var items []json.RawMessage
			if err := json.Unmarshal([]byte(raw), &items); err != nil {
				return nil, true, err
			}
			return items, true, nil
		}
		return []json.RawMessage{json.RawMessage(raw)}, true, nil
	}
	if inputValue.Type == gjson.String {
		encoded, _ := json.Marshal(inputValue.String())
		return []json.RawMessage{encoded}, true, nil
	}
	return []json.RawMessage{json.RawMessage(inputValue.Raw)}, true, nil
}

func openAIWSInputIsPrefixExtended(previousPayload, currentPayload []byte) (bool, error) {
	previousItems, previousExists, prevErr := openAIWSExtractNormalizedInputSequence(previousPayload)
	if prevErr != nil {
		return false, prevErr
	}
	currentItems, currentExists, currentErr := openAIWSExtractNormalizedInputSequence(currentPayload)
	if currentErr != nil {
		return false, currentErr
	}
	if !previousExists && !currentExists {
		return true, nil
	}
	if !previousExists {
		return len(currentItems) == 0, nil
	}
	if !currentExists {
		return len(previousItems) == 0, nil
	}
	if len(currentItems) < len(previousItems) {
		return false, nil
	}

	for idx := range previousItems {
		previousNormalized := normalizeOpenAIWSJSONForCompareOrRaw(previousItems[idx])
		currentNormalized := normalizeOpenAIWSJSONForCompareOrRaw(currentItems[idx])
		if !bytes.Equal(previousNormalized, currentNormalized) {
			return false, nil
		}
	}
	return true, nil
}

func openAIWSRawItemsHasPrefix(items []json.RawMessage, prefix []json.RawMessage) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(items) < len(prefix) {
		return false
	}
	for idx := range prefix {
		previousNormalized := normalizeOpenAIWSJSONForCompareOrRaw(prefix[idx])
		currentNormalized := normalizeOpenAIWSJSONForCompareOrRaw(items[idx])
		if !bytes.Equal(previousNormalized, currentNormalized) {
			return false
		}
	}
	return true
}

func openAIWSRawItemsHasFunctionCallOutput(items []json.RawMessage) bool {
	for _, item := range items {
		if isToolContinuationOutputItemType(gjson.GetBytes(item, "type").String()) {
			return true
		}
	}
	return false
}

func openAIWSRawItemsHaveToolCallContextForOutputs(items []json.RawMessage) bool {
	if len(items) == 0 {
		return false
	}
	contextCallIDs := make(map[string]struct{})
	outputCallIDs := make(map[string]struct{})
	for _, item := range items {
		parsed := parseRawJSONView(item)
		itemType := parsed.Get("type").String()
		switch {
		case isCodexToolCallContextItemType(itemType):
			if callID := toolCallContextIDFromRaw(parsed); callID != "" {
				contextCallIDs[callID] = struct{}{}
			}
		case isCodexToolCallOutputItemType(itemType):
			callID := strings.TrimSpace(parsed.Get("call_id").String())
			if callID == "" {
				return false
			}
			outputCallIDs[callID] = struct{}{}
		}
	}
	if len(outputCallIDs) == 0 || len(contextCallIDs) == 0 {
		return false
	}
	for callID := range outputCallIDs {
		if _, ok := contextCallIDs[callID]; !ok {
			return false
		}
	}
	return true
}

func openAIWSRawPayloadHasToolCallOutput(payload []byte) bool {
	return HasToolContinuationOutputInRawPayload(payload)
}

func analyzeToolContinuationSignalsFromRawPayload(payload []byte) ToolContinuationSignals {
	signals := ToolContinuationSignals{
		HasFunctionCallOutput: openAIWSRawPayloadHasToolCallOutput(payload),
	}
	if !signals.HasFunctionCallOutput {
		return signals
	}
	var reqBody map[string]any
	if err := json.Unmarshal(payload, &reqBody); err != nil {
		return signals
	}
	return AnalyzeToolContinuationSignals(reqBody)
}

func buildOpenAIWSReplayInputSequence(
	previousFullInput []json.RawMessage,
	previousFullInputExists bool,
	currentPayload []byte,
	hasPreviousResponseID bool,
) ([]json.RawMessage, bool, error) {
	currentItems, currentExists, currentErr := openAIWSExtractNormalizedInputSequence(currentPayload)
	if currentErr != nil {
		return nil, false, currentErr
	}
	if !hasPreviousResponseID {
		return cloneOpenAIWSRawMessages(currentItems), currentExists, nil
	}
	if !previousFullInputExists {
		return cloneOpenAIWSRawMessages(currentItems), currentExists, nil
	}
	if !currentExists || len(currentItems) == 0 {
		return cloneOpenAIWSRawMessages(previousFullInput), true, nil
	}
	if openAIWSRawItemsHasPrefix(currentItems, previousFullInput) {
		return cloneOpenAIWSRawMessages(currentItems), true, nil
	}
	merged := make([]json.RawMessage, 0, len(previousFullInput)+len(currentItems))
	merged = append(merged, cloneOpenAIWSRawMessages(previousFullInput)...)
	merged = append(merged, cloneOpenAIWSRawMessages(currentItems)...)
	return merged, true, nil
}

func setOpenAIWSPayloadInputSequence(
	payload []byte,
	fullInput []json.RawMessage,
	fullInputExists bool,
) ([]byte, error) {
	if !fullInputExists {
		return payload, nil
	}
	// Preserve [] vs null semantics when input exists but is empty.
	inputForMarshal := fullInput
	if inputForMarshal == nil {
		inputForMarshal = []json.RawMessage{}
	}
	inputRaw, marshalErr := json.Marshal(inputForMarshal)
	if marshalErr != nil {
		return nil, marshalErr
	}
	return sjson.SetRawBytes(payload, "input", inputRaw)
}

func shouldKeepIngressPreviousResponseID(
	previousPayload []byte,
	currentPayload []byte,
	lastTurnResponseID string,
	hasFunctionCallOutput bool,
) (bool, string, error) {
	if hasFunctionCallOutput {
		return true, "has_function_call_output", nil
	}
	currentPreviousResponseID := strings.TrimSpace(openAIWSPayloadStringFromRaw(currentPayload, "previous_response_id"))
	if currentPreviousResponseID == "" {
		return false, "missing_previous_response_id", nil
	}
	expectedPreviousResponseID := strings.TrimSpace(lastTurnResponseID)
	if expectedPreviousResponseID == "" {
		return false, "missing_last_turn_response_id", nil
	}
	if currentPreviousResponseID != expectedPreviousResponseID {
		return false, "previous_response_id_mismatch", nil
	}
	if len(previousPayload) == 0 {
		return false, "missing_previous_turn_payload", nil
	}

	previousComparable, previousComparableErr := normalizeOpenAIWSPayloadWithoutInputAndPreviousResponseID(previousPayload)
	if previousComparableErr != nil {
		return false, "non_input_compare_error", previousComparableErr
	}
	currentComparable, currentComparableErr := normalizeOpenAIWSPayloadWithoutInputAndPreviousResponseID(currentPayload)
	if currentComparableErr != nil {
		return false, "non_input_compare_error", currentComparableErr
	}
	if !bytes.Equal(previousComparable, currentComparable) {
		return false, "non_input_changed", nil
	}
	return true, "strict_incremental_ok", nil
}

type openAIWSIngressPreviousTurnStrictState struct {
	nonInputComparable []byte
}

func buildOpenAIWSIngressPreviousTurnStrictState(payload []byte) (*openAIWSIngressPreviousTurnStrictState, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	nonInputComparable, nonInputErr := normalizeOpenAIWSPayloadWithoutInputAndPreviousResponseID(payload)
	if nonInputErr != nil {
		return nil, nonInputErr
	}
	return &openAIWSIngressPreviousTurnStrictState{
		nonInputComparable: nonInputComparable,
	}, nil
}

func shouldKeepIngressPreviousResponseIDWithStrictState(
	previousState *openAIWSIngressPreviousTurnStrictState,
	currentPayload []byte,
	lastTurnResponseID string,
	hasFunctionCallOutput bool,
) (bool, string, error) {
	if hasFunctionCallOutput {
		return true, "has_function_call_output", nil
	}
	currentPreviousResponseID := strings.TrimSpace(openAIWSPayloadStringFromRaw(currentPayload, "previous_response_id"))
	if currentPreviousResponseID == "" {
		return false, "missing_previous_response_id", nil
	}
	expectedPreviousResponseID := strings.TrimSpace(lastTurnResponseID)
	if expectedPreviousResponseID == "" {
		return false, "missing_last_turn_response_id", nil
	}
	if currentPreviousResponseID != expectedPreviousResponseID {
		return false, "previous_response_id_mismatch", nil
	}
	if previousState == nil {
		return false, "missing_previous_turn_payload", nil
	}

	currentComparable, currentComparableErr := normalizeOpenAIWSPayloadWithoutInputAndPreviousResponseID(currentPayload)
	if currentComparableErr != nil {
		return false, "non_input_compare_error", currentComparableErr
	}
	if !bytes.Equal(previousState.nonInputComparable, currentComparable) {
		return false, "non_input_changed", nil
	}
	return true, "strict_incremental_ok", nil
}

func shouldBranchOpenAIWSIngressFollowupTurn(
	turn int,
	originalClientPayload []byte,
	currentPayload []byte,
	storeDisabled bool,
	previousPayload []byte,
	previousState *openAIWSIngressPreviousTurnStrictState,
	lastTurnResponseID string,
) (bool, string, error) {
	if turn <= 1 {
		return false, "", nil
	}
	signals := analyzeToolContinuationSignalsFromRawPayload(currentPayload)
	currentPreviousResponseID := strings.TrimSpace(openAIWSPayloadStringFromRaw(currentPayload, "previous_response_id"))
	clientPreviousResponseID := strings.TrimSpace(openAIWSPayloadStringFromRaw(originalClientPayload, "previous_response_id"))
	if shouldInferIngressFunctionCallOutputPreviousResponseID(
		storeDisabled,
		turn,
		signals,
		currentPreviousResponseID,
		lastTurnResponseID,
	) {
		return false, "tool_continuation_prev_infer", nil
	}
	if currentPreviousResponseID != "" {
		// 显式 previous_response_id 的 follow-up 继续沿用既有 strict/full-create/
		// recovery 规则；这里只处理“同 session 但未显式续链”的 branch 场景。
		return false, "explicit_previous_response_id", nil
	}
	if clientPreviousResponseID != "" {
		// 规范化过程中可能因为 unsafe/unbound continuation 已经删掉了
		// previous_response_id；这种 follow-up 仍属于“客户端显式请求续链”，
		// 继续交给既有 full-create/recovery 逻辑处理，而不是当作历史编辑分叉。
		return false, "normalized_previous_response_id_removed", nil
	}
	if currentPreviousResponseID == "" {
		if signals.HasFunctionCallOutput {
			if signals.HasToolCallContext || strings.TrimSpace(lastTurnResponseID) == "" {
				return false, "self_contained_tool_replay", nil
			}
		}
		return true, "missing_previous_response_id", nil
	}
	_ = previousPayload
	_ = previousState
	return false, "", nil
}

func (s *OpenAIGatewayService) forwardOpenAIWSV2(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	reqBody map[string]any,
	token string,
	decision OpenAIWSProtocolDecision,
	isCodexCLI bool,
	reqStream bool,
	originalModel string,
	mappedModel string,
	startTime time.Time,
	attempt int,
	lastFailureReason string,
) (*OpenAIForwardResult, error) {
	if s == nil || account == nil {
		return nil, wrapOpenAIWSFallback("invalid_state", errors.New("service or account is nil"))
	}

	wsURL, err := s.buildOpenAIResponsesWSURL(account)
	if err != nil {
		return nil, wrapOpenAIWSFallback("build_ws_url", err)
	}
	wsHost := "-"
	wsPath := "-"
	accountID := int64(0)
	accountType := ""
	if account != nil {
		accountID = account.ID
		accountType = account.Type
	}
	if parsed, parseErr := url.Parse(wsURL); parseErr == nil && parsed != nil {
		if h := strings.TrimSpace(parsed.Host); h != "" {
			wsHost = normalizeOpenAIWSLogValue(h)
		}
		if p := strings.TrimSpace(parsed.Path); p != "" {
			wsPath = normalizeOpenAIWSLogValue(p)
		}
	}
	logOpenAIWSModeDebug(
		"dial_target account_id=%d account_type=%s ws_host=%s ws_path=%s",
		accountID,
		accountType,
		wsHost,
		wsPath,
	)

	payload := s.buildOpenAIWSCreatePayload(reqBody, account)
	if account != nil && account.Type == AccountTypeOAuth {
		s.attachOpenAIWSOAuthClientMetadata(payload, c)
	}
	payloadStrategy, removedKeys := applyOpenAIWSRetryPayloadStrategy(payload, attempt)
	stateStore := s.getOpenAIWSStateStore()
	groupID := getOpenAIGroupIDFromContext(c)
	apiKeyID := getAPIKeyIDFromContext(c)
	storeDecision := s.resolveOpenAIWSContinuationStoreDecision(ctx, payload, account, stateStore, groupID, apiKeyID)
	if err := storeDecision.unsafeToolContinuationError(); err != nil {
		return nil, wrapOpenAIWSFallback("unsafe_tool_continuation", err)
	}
	trimOpenAIStoreFalseReasoningItems(payload)
	previousResponseID := openAIWSPayloadString(payload, "previous_response_id")
	previousResponseIDKind := ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
	promptCacheKey := openAIWSPayloadString(payload, "prompt_cache_key")
	if promptCacheKey == "" {
		promptCacheKey = getOpenAIRoutingPromptCacheKey(c)
	}
	_, hasTools := payload["tools"]
	debugEnabled := isOpenAIWSModeDebugEnabled()
	payloadBytes := -1
	resolvePayloadBytes := func() int {
		if payloadBytes >= 0 {
			return payloadBytes
		}
		payloadBytes = len(payloadAsJSONBytes(payload))
		return payloadBytes
	}
	streamValue := "-"
	if raw, ok := payload["stream"]; ok {
		streamValue = normalizeOpenAIWSLogValue(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	}
	turnState := ""
	turnMetadata := ""
	if c != nil && c.Request != nil {
		turnState = strings.TrimSpace(c.GetHeader(openAIWSTurnStateHeader))
		turnMetadata = strings.TrimSpace(c.GetHeader(openAIWSTurnMetadataHeader))
	}
	setOpenAIWSTurnMetadata(payload, turnMetadata)
	payloadEventType := openAIWSPayloadString(payload, "type")
	if payloadEventType == "" {
		payloadEventType = "response.create"
	}
	if s.shouldEmitOpenAIWSPayloadSchema(attempt) {
		logOpenAIWSModeInfo(
			"[debug] payload_schema account_id=%d attempt=%d event=%s payload_keys=%s payload_bytes=%d payload_key_sizes=%s input_summary=%s stream=%s payload_strategy=%s removed_keys=%s store_mode=%s store_enabled=%v sticky_account_hit=%v conn_affinity_hit=%v fallback_reason=%s has_previous_response_id=%v has_prompt_cache_key=%v has_tools=%v",
			account.ID,
			attempt,
			payloadEventType,
			normalizeOpenAIWSLogValue(strings.Join(sortedKeys(payload), ",")),
			resolvePayloadBytes(),
			normalizeOpenAIWSLogValue(summarizeOpenAIWSPayloadKeySizes(payload, openAIWSPayloadKeySizeTopN)),
			normalizeOpenAIWSLogValue(summarizeOpenAIWSInput(payload["input"])),
			streamValue,
			normalizeOpenAIWSLogValue(payloadStrategy),
			normalizeOpenAIWSLogValue(strings.Join(removedKeys, ",")),
			normalizeOpenAIWSLogValue(storeDecision.StoreMode),
			storeDecision.StoreEnabled,
			storeDecision.StickyAccountHit,
			storeDecision.ConnAffinityHit,
			normalizeOpenAIWSLogValue(storeDecision.FallbackReason),
			previousResponseID != "",
			promptCacheKey != "",
			hasTools,
		)
	}

	httpIngressWSOneShot := s.shouldUseOpenAIHTTPIngressWSOneShot(c, payload, previousResponseID, promptCacheKey)
	if httpIngressWSOneShot && c != nil {
		c.Set("openai_http_ingress_ws_one_shot", true)
	}
	sessionHash := s.GenerateSessionHash(c, nil)
	if sessionHash == "" {
		var legacySessionHash string
		sessionHash, legacySessionHash = deriveOpenAIRequestScopedSessionHashes(c, promptCacheKey)
		attachOpenAILegacySessionHashToGin(c, legacySessionHash)
	}
	if httpIngressWSOneShot {
		sessionHash = ""
	}
	requestID, clientRequestID := openAIWSRequestLogIDs(c)
	sessionPreemptedPrevious := false
	if preemptCtx, cleanupPreempt, preemptEnabled, preemptedPrevious := s.beginOpenAIWSSessionPreemptContext(
		ctx,
		account,
		groupID,
		apiKeyID,
		sessionHash,
		httpIngressWSOneShot,
		requestID,
	); preemptEnabled {
		ctx = preemptCtx
		sessionPreemptedPrevious = preemptedPrevious
		defer cleanupPreempt()
		logOpenAIWSModeInfo(
			"session_preempt_register account_id=%d account_type=%s request_id=%s session_hash=%s preempted_previous=%v",
			account.ID,
			account.Type,
			truncateOpenAIWSLogValue(requestID, openAIWSIDValueMaxLen),
			truncateOpenAIWSLogValue(sessionHash, 12),
			sessionPreemptedPrevious,
		)
	}
	if sessionPreemptedPrevious && stateStore != nil && sessionHash != "" {
		stateStore.DeleteSessionTurnState(groupID, sessionHash)
		stateStore.DeleteSessionConn(groupID, apiKeyID, account.ID, sessionHash)
		stateStore.DeleteSessionContext(groupID, apiKeyID, sessionHash)
		storeDecision.PreferredConnID = ""
		storeDecision.ConnAffinityHit = false
		storeDecision.StickyAccountHit = false
		storeDecision.StickyAccountID = 0
		storeDecision.StickyAccountMismatch = false
		storeDecision.FallbackReason = "session_preempted_full_replay"
		delete(payload, "previous_response_id")
		payload["store"] = false
		previousResponseID = ""
		previousResponseIDKind = ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
		turnState = ""
	}
	if turnState == "" && !sessionPreemptedPrevious && stateStore != nil && sessionHash != "" {
		if savedTurnState, ok := stateStore.GetSessionTurnState(groupID, sessionHash); ok {
			turnState = savedTurnState
		}
	}
	preferredConnID := ""
	connAffinityHit := false
	if !httpIngressWSOneShot {
		preferredConnID = storeDecision.PreferredConnID
		connAffinityHit = storeDecision.ConnAffinityHit
	}
	storeDisabled := openAIWSPayloadStoreDisabled(payload)
	storeEnabled := openAIWSPayloadStoreEnabled(payload)
	forceNewConnForRecoveredFullReplay := shouldForceNewConnOnRecoveredFullReplay(lastFailureReason)
	if !sessionPreemptedPrevious && !forceNewConnForRecoveredFullReplay && !httpIngressWSOneShot && stateStore != nil && storeDisabled && previousResponseID == "" && sessionHash != "" {
		if connID, ok := stateStore.GetSessionConn(groupID, apiKeyID, account.ID, sessionHash); ok {
			preferredConnID = connID
			connAffinityHit = true
		}
	}
	storeDisabledConnMode := s.openAIWSStoreDisabledConnMode()
	forceNewConnByPolicy := shouldForceNewConnOnStoreDisabled(storeDisabledConnMode, lastFailureReason)
	forceNewConn := forceNewConnByPolicy && storeDisabled && previousResponseID == "" && sessionHash != "" && preferredConnID == ""
	sameAccountContinuationWithoutConn := account.Type == AccountTypeOAuth &&
		previousResponseID != "" &&
		storeDecision.StickyAccountHit &&
		!storeDecision.ConnAffinityHit &&
		strings.TrimSpace(storeDecision.FallbackReason) == "conn_affinity_miss"
	sameAccountContinuationReanchoredSessionConn := false
	if sameAccountContinuationWithoutConn && stateStore != nil && sessionHash != "" {
		if cached, ok := stateStore.GetSessionContext(groupID, apiKeyID, sessionHash); ok &&
			cached.accountID == account.ID &&
			strings.TrimSpace(cached.lastResponseID) == previousResponseID &&
			strings.TrimSpace(cached.connID) != "" {
			preferredConnID = strings.TrimSpace(cached.connID)
			connAffinityHit = true
			storeDecision.PreferredConnID = preferredConnID
			storeDecision.ConnAffinityHit = true
			storeDecision.FallbackReason = "session_context_conn_reanchor"
			sameAccountContinuationReanchoredSessionConn = true
		}
	}
	if forceNewConnForRecoveredFullReplay && storeDisabled && previousResponseID == "" {
		preferredConnID = ""
		connAffinityHit = false
		forceNewConn = true
	}
	if sameAccountContinuationWithoutConn && !sameAccountContinuationReanchoredSessionConn {
		preferredConnID = ""
		connAffinityHit = false
		forceNewConn = true
	}
	lastFailureAllowsDeltaConnReanchor := strings.TrimSpace(lastFailureReason) == "" ||
		strings.TrimSpace(lastFailureReason) == "write_request" ||
		strings.TrimSpace(lastFailureReason) == "write"
	hasFunctionCallOutputForReanchor := HasFunctionCallOutput(payload)
	deltaConnReanchorTarget := false
	if !sessionPreemptedPrevious &&
		attempt <= 2 &&
		lastFailureAllowsDeltaConnReanchor &&
		!httpIngressWSOneShot &&
		account.Type == AccountTypeOAuth &&
		stateStore != nil &&
		storeDisabled &&
		previousResponseID == "" &&
		sessionHash != "" &&
		!hasFunctionCallOutputForReanchor {
		deltaConnReanchorTarget = openAIWSHasDeltaReanchorTarget(stateStore, groupID, apiKeyID, sessionHash, account.ID)
	}
	deltaConnReanchorBlockers := openAIWSDeltaConnReanchorBlockers(
		sessionPreemptedPrevious,
		attempt,
		lastFailureReason,
		httpIngressWSOneShot,
		account,
		stateStore,
		storeDisabled,
		previousResponseID,
		sessionHash,
		hasFunctionCallOutputForReanchor,
		deltaConnReanchorTarget,
	)
	allowDeltaConnReanchor := !sessionPreemptedPrevious &&
		attempt <= 2 &&
		lastFailureAllowsDeltaConnReanchor &&
		!httpIngressWSOneShot &&
		account.Type == AccountTypeOAuth &&
		stateStore != nil &&
		storeDisabled &&
		previousResponseID == "" &&
		sessionHash != "" &&
		!hasFunctionCallOutputForReanchor &&
		deltaConnReanchorTarget
	if sessionPreemptedPrevious {
		preferredConnID = ""
		connAffinityHit = false
		allowDeltaConnReanchor = false
		forceNewConn = true
	}
	connProfile := openAIWSConnProfileSessionBound
	promoteNeutralConnToSessionBound := false
	pool := s.getOpenAIWSConnPool()
	tlsFPRuntime := s.resolveOpenAITLSFingerprintRuntime(ctx, c, account, "websocket")
	wsHeaders, sessionResolution := s.buildOpenAIWSHeaders(c, account, token, decision, isCodexCLI, turnState, turnMetadata, promptCacheKey)
	applyOpenAIWSFingerprintRuntimeHeaders(wsHeaders, tlsFPRuntime)
	if httpIngressWSOneShot {
		// 无会话 one-shot：使用账号级中性握手头，不绑定 session/response。
		connProfile = openAIWSConnProfileNeutral
		if shouldForceNewConnOnHTTPIngressWSOneShotRetry(lastFailureReason) {
			forceNewConn = true
		}
		wsHeaders = s.buildOpenAIWSNeutralHeaders(account, token, decision, isCodexCLI)
		applyOpenAIWSFingerprintRuntimeHeaders(wsHeaders, tlsFPRuntime)
	}
	if shouldUseOpenAIWSNeutralForColdSession(account, httpIngressWSOneShot, storeDisabled, previousResponseID, sessionHash, turnState, turnMetadata, payload) {
		useNeutral := preferredConnID == ""
		if preferredConnID != "" {
			if profile, ok := pool.ConnProfile(account.ID, preferredConnID); ok {
				useNeutral = profile == openAIWSConnProfileNeutral
			} else {
				preferredConnID = ""
				connAffinityHit = false
				useNeutral = true
			}
		}
		if useNeutral && !allowDeltaConnReanchor {
			connProfile = openAIWSConnProfileNeutral
			if !forceNewConnForRecoveredFullReplay {
				forceNewConn = false
			}
			wsHeaders = s.buildOpenAIWSNeutralHeaders(account, token, decision, isCodexCLI)
			applyOpenAIWSFingerprintRuntimeHeaders(wsHeaders, tlsFPRuntime)
			promoteNeutralConnToSessionBound = true
		}
	}
	affinityOnlyReuse := !httpIngressWSOneShot &&
		connProfile == openAIWSConnProfileSessionBound &&
		account.Type == AccountTypeOAuth &&
		storeEnabled &&
		previousResponseID != ""
	if !httpIngressWSOneShot &&
		connProfile == openAIWSConnProfileSessionBound &&
		account.Type == AccountTypeOAuth &&
		preferredConnID == "" &&
		((storeEnabled && previousResponseID != "") || storeDecision.DroppedPreviousResponseID || sameAccountContinuationWithoutConn) {
		forceNewConn = true
	}
	if shouldLogOpenAIWSContinuationBindingSnapshot(storeDecision) {
		snapshotPreviousResponseID := previousResponseID
		if strings.TrimSpace(snapshotPreviousResponseID) == "" {
			snapshotPreviousResponseID = storeDecision.OriginalPreviousResponseID
		}
		s.logOpenAIWSBindingSnapshot(
			ctx,
			"continuation_binding",
			requestID,
			account,
			stateStore,
			pool,
			groupID,
			apiKeyID,
			sessionHash,
			snapshotPreviousResponseID,
			strings.TrimSpace(storeDecision.OriginalPreviousResponseID) != "",
			"",
			preferredConnID,
			storeDecision,
			storeDecision.FallbackReason,
			"",
			"",
			"",
		)
	}
	logOpenAIWSModeDebug(
		"acquire_start account_id=%d account_type=%s transport=%s preferred_conn_id=%s has_previous_response_id=%v session_hash=%s has_turn_state=%v turn_state_len=%d has_turn_metadata=%v turn_metadata_len=%d store_mode=%s store_enabled=%v store_disabled=%v sticky_account_hit=%v conn_affinity_hit=%v fallback_reason=%s store_disabled_conn_mode=%s retry_last_reason=%s force_new_conn=%v affinity_only_reuse=%v http_ingress_ws_one_shot=%v header_user_agent=%s header_openai_beta=%s header_originator=%s header_accept_language=%s header_session_id=%s header_conversation_id=%s session_id_source=%s conversation_id_source=%s has_prompt_cache_key=%v has_chatgpt_account_id=%v has_authorization=%v has_session_id=%v has_conversation_id=%v proxy_enabled=%v",
		account.ID,
		account.Type,
		normalizeOpenAIWSLogValue(string(decision.Transport)),
		truncateOpenAIWSLogValue(preferredConnID, openAIWSIDValueMaxLen),
		previousResponseID != "",
		truncateOpenAIWSLogValue(sessionHash, 12),
		turnState != "",
		len(turnState),
		turnMetadata != "",
		len(turnMetadata),
		normalizeOpenAIWSLogValue(storeDecision.StoreMode),
		storeEnabled,
		storeDisabled,
		storeDecision.StickyAccountHit,
		connAffinityHit,
		normalizeOpenAIWSLogValue(storeDecision.FallbackReason),
		normalizeOpenAIWSLogValue(storeDisabledConnMode),
		truncateOpenAIWSLogValue(lastFailureReason, openAIWSLogValueMaxLen),
		forceNewConn,
		affinityOnlyReuse,
		httpIngressWSOneShot,
		openAIWSHeaderValueForLog(wsHeaders, "user-agent"),
		openAIWSHeaderValueForLog(wsHeaders, "openai-beta"),
		openAIWSHeaderValueForLog(wsHeaders, "originator"),
		openAIWSHeaderValueForLog(wsHeaders, "accept-language"),
		openAIWSHeaderValueForLog(wsHeaders, "session_id"),
		openAIWSHeaderValueForLog(wsHeaders, "conversation_id"),
		normalizeOpenAIWSLogValue(sessionResolution.SessionSource),
		normalizeOpenAIWSLogValue(sessionResolution.ConversationSource),
		promptCacheKey != "",
		hasOpenAIWSHeader(wsHeaders, "chatgpt-account-id"),
		hasOpenAIWSHeader(wsHeaders, "authorization"),
		hasOpenAIWSHeader(wsHeaders, "session_id"),
		hasOpenAIWSHeader(wsHeaders, "conversation_id"),
		account.ProxyID != nil && account.Proxy != nil,
	)

	acquireCtx, acquireCancel := context.WithTimeout(ctx, s.openAIWSAcquireTimeout())
	defer acquireCancel()

	acquireReq := openAIWSAcquireRequest{
		Account:           account,
		WSURL:             wsURL,
		Headers:           wsHeaders,
		TLSProfile:        tlsFPRuntime.Profile,
		PreferredConnID:   preferredConnID,
		ForceNewConn:      forceNewConn,
		AffinityOnlyReuse: affinityOnlyReuse,
		Profile:           connProfile,
		ProxyURL: func() string {
			if account.ProxyID != nil && account.Proxy != nil {
				return account.Proxy.URL()
			}
			return ""
		}(),
	}
	originalPreferredConnID := preferredConnID
	preferredAcquireSnapshot := openAIWSConnAcquireSnapshot{}
	forcePreferredConn := false
	if strings.TrimSpace(preferredConnID) != "" && pool != nil {
		preferredAcquireSnapshot = pool.ConnAcquireSnapshot(account.ID, preferredConnID, acquireReq)
		forcePreferredConn = shouldForceOpenAIWSPreferredConn(account, preferredConnID, preferredAcquireSnapshot, connProfile, forceNewConn, httpIngressWSOneShot)
		acquireReq.ForcePreferredConn = forcePreferredConn
		logOpenAIWSModeInfo("%s", openAIWSAcquirePreferredSnapshotLogMessage(openAIWSAcquirePreferredSnapshotLog{
			RequestID:                     requestID,
			GroupID:                       groupID,
			APIKeyID:                      apiKeyID,
			AccountID:                     account.ID,
			AccountType:                   account.Type,
			SessionHash:                   sessionHash,
			PreferredConnID:               preferredConnID,
			PreferredConnInPool:           preferredAcquireSnapshot.Exists,
			PreferredConnProfile:          preferredAcquireSnapshot.Profile,
			PreferredConnAgeMS:            preferredAcquireSnapshot.Age.Milliseconds(),
			PreferredConnIdleMS:           preferredAcquireSnapshot.Idle.Milliseconds(),
			PreferredConnLeaseCount:       preferredAcquireSnapshot.LeaseCount,
			PreferredConnLeased:           preferredAcquireSnapshot.Leased,
			PreferredConnWaiters:          preferredAcquireSnapshot.Waiters,
			PreferredConnMatchesAcquire:   preferredAcquireSnapshot.MatchesAcquire,
			PreferredConnProfileMatches:   preferredAcquireSnapshot.ProfileMatches,
			PreferredConnIdentityRequired: preferredAcquireSnapshot.IdentityRequired,
			PreferredConnIdentityMatches:  preferredAcquireSnapshot.IdentityMatches,
			PreferredConnReuseKeyMatches:  preferredAcquireSnapshot.ReuseKeyMatches,
			ConnProfile:                   connProfile,
			ForcePreferredConn:            forcePreferredConn,
			ForceNewConn:                  forceNewConn,
			AffinityOnlyReuse:             affinityOnlyReuse,
			HasPreviousResponseID:         previousResponseID != "",
			StoreMode:                     storeDecision.StoreMode,
			StoreDisabled:                 storeDisabled,
			StoreFallbackReason:           storeDecision.FallbackReason,
		}))
	}
	lease, err := pool.Acquire(acquireCtx, acquireReq)
	if err != nil && forcePreferredConn && errors.Is(err, errOpenAIWSPreferredConnUnavailable) &&
		canFallbackOpenAIWSPreferredConnUnavailable(storeDisabled, previousResponseID) {
		logOpenAIWSModeInfo(
			"openai_ws_preferred_conn_unavailable_safe_fallback temporary_diag=sticky_select remove_after_debug=true request_id=%s group_id=%d api_key_id=%d account_id=%d account_type=%s session_hash=%s preferred_conn_id=%s reason=preferred_conn_unavailable_full_replay",
			normalizeOpenAIWSLogValue(requestID),
			groupID,
			apiKeyID,
			account.ID,
			normalizeOpenAIWSLogValue(account.Type),
			truncateOpenAIWSLogValue(sessionHash, 12),
			truncateOpenAIWSLogValue(originalPreferredConnID, openAIWSIDValueMaxLen),
		)
		forcePreferredConn = false
		preferredConnID = ""
		connAffinityHit = false
		storeDecision.ConnAffinityHit = false
		storeDecision.FallbackReason = "preferred_conn_unavailable_full_replay"
		acquireReq.PreferredConnID = ""
		acquireReq.ForcePreferredConn = false
		lease, err = pool.Acquire(acquireCtx, acquireReq)
	}
	if err != nil {
		if isOpenAIWSSessionPreempted(ctx) {
			logOpenAIWSModeInfo(
				"session_preempted account_id=%d account_type=%s request_id=%s stage=acquire preferred_conn_id=%s",
				account.ID,
				account.Type,
				truncateOpenAIWSLogValue(requestID, openAIWSIDValueMaxLen),
				truncateOpenAIWSLogValue(preferredConnID, openAIWSIDValueMaxLen),
			)
			return nil, wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted)
		}
		dialStatus, dialClass, dialCloseStatus, dialCloseReason, dialRespServer, dialRespVia, dialRespCFRay, dialRespReqID := summarizeOpenAIWSDialError(err)
		logOpenAIWSModeInfo(
			"acquire_fail account_id=%d account_type=%s transport=%s reason=%s dial_status=%d dial_class=%s dial_close_status=%s dial_close_reason=%s dial_resp_server=%s dial_resp_via=%s dial_resp_cf_ray=%s dial_resp_x_request_id=%s cause=%s preferred_conn_id=%s force_new_conn=%v ws_host=%s ws_path=%s proxy_enabled=%v",
			account.ID,
			account.Type,
			normalizeOpenAIWSLogValue(string(decision.Transport)),
			normalizeOpenAIWSLogValue(classifyOpenAIWSAcquireError(err)),
			dialStatus,
			dialClass,
			dialCloseStatus,
			truncateOpenAIWSLogValue(dialCloseReason, openAIWSHeaderValueMaxLen),
			dialRespServer,
			dialRespVia,
			dialRespCFRay,
			dialRespReqID,
			truncateOpenAIWSLogValue(err.Error(), openAIWSLogValueMaxLen),
			truncateOpenAIWSLogValue(preferredConnID, openAIWSIDValueMaxLen),
			forceNewConn,
			wsHost,
			wsPath,
			account.ProxyID != nil && account.Proxy != nil,
		)
		s.persistOpenAIWSDialFailureSignal(ctx, account, err)
		return nil, wrapOpenAIWSFallback(classifyOpenAIWSAcquireError(err), err)
	}
	// cleanExit 标记正常终端事件退出，此时上游不会再发送帧，连接可安全归还复用。
	// 所有异常路径（读写错误、error 事件等）已在各自分支中提前调用 MarkBroken，
	// 因此 defer 中只需处理正常退出时不 MarkBroken 即可。
	cleanExit := false
	clientDisconnected := false
	defer func() {
		if !cleanExit || clientDisconnected {
			reason := "unclean_exit"
			if clientDisconnected {
				reason = "client_disconnected"
			}
			lease.MarkBrokenFor(reason)
		}
		lease.Release()
	}()
	connID := strings.TrimSpace(lease.ConnID())
	if strings.TrimSpace(originalPreferredConnID) != "" && connID != strings.TrimSpace(originalPreferredConnID) {
		logOpenAIWSModeInfo("%s", openAIWSPreferredConnDriftLogMessage(openAIWSPreferredConnDriftLog{
			RequestID:                   requestID,
			GroupID:                     groupID,
			APIKeyID:                    apiKeyID,
			AccountID:                   account.ID,
			AccountType:                 account.Type,
			SessionHash:                 sessionHash,
			PreferredConnID:             originalPreferredConnID,
			LeaseConnID:                 connID,
			PreferredConnInPool:         preferredAcquireSnapshot.Exists,
			PreferredConnProfile:        preferredAcquireSnapshot.Profile,
			PreferredConnLeased:         preferredAcquireSnapshot.Leased,
			PreferredConnWaiters:        preferredAcquireSnapshot.Waiters,
			PreferredConnMatchesAcquire: preferredAcquireSnapshot.MatchesAcquire,
			ConnProfile:                 connProfile,
			ForcePreferredConn:          forcePreferredConn,
			ForceNewConn:                forceNewConn,
			AffinityOnlyReuse:           affinityOnlyReuse,
			StoreFallbackReason:         storeDecision.FallbackReason,
		}))
	}
	if promoteNeutralConnToSessionBound {
		pool.PromoteNeutralConnToSessionBound(account.ID, connID)
	}
	logOpenAIWSModeDebug(
		"connected account_id=%d account_type=%s transport=%s conn_id=%s conn_reused=%v conn_pick_ms=%d queue_wait_ms=%d has_previous_response_id=%v",
		account.ID,
		account.Type,
		normalizeOpenAIWSLogValue(string(decision.Transport)),
		connID,
		lease.Reused(),
		lease.ConnPickDuration().Milliseconds(),
		lease.QueueWaitDuration().Milliseconds(),
		previousResponseID != "",
	)

	// active delta uses the full payload only as a verified source-of-truth for
	// prefix matching and later context binding. The upstream write may be
	// reduced to just the trailing input items.
	contextPayloadRaw := openAIWSActiveDeltaContextPayloadRaw(payload, storeDecision)
	deltaShadowEnabled := openAIWSDeltaShadowEnabled()
	activeDeltaLog := openAIWSDeltaShadowLog{}
	activeDeltaApplied := false
	shadowOwner := false
	if deltaShadowEnabled && !sessionPreemptedPrevious && !httpIngressWSOneShot && stateStore != nil && sessionHash != "" {
		shadowOwner = stateStore.TrySessionInFlight(groupID, apiKeyID, sessionHash)
		if !shadowOwner {
			activeDeltaLog = openAIWSDeltaShadowLog{
				RequestID:      requestID,
				AccountID:      account.ID,
				ConnID:         connID,
				Candidate:      false,
				FallbackReason: "same_session_in_flight",
			}
		} else {
			cached, found := stateStore.GetSessionContext(groupID, apiKeyID, sessionHash)
			connMostRecent, _ := stateStore.GetConnLastResponse(connID)
			shadowInput := openAIWSDeltaShadowInput{
				RequestID:                requestID,
				AccountID:                account.ID,
				LeaseConnID:              connID,
				ConnMostRecentResponseID: connMostRecent,
				CurrentPayload:           contextPayloadRaw,
				HasFunctionCallOutput:    HasToolContinuationOutputInRawPayload(contextPayloadRaw),
				AllowConnReanchor:        allowDeltaConnReanchor,
				StickyAccountID:          storeDecision.StickyAccountID,
				StickyAccountHit:         storeDecision.StickyAccountHit,
				StickyAccountMismatch:    storeDecision.StickyAccountMismatch,
				ConnAffinityHit:          storeDecision.ConnAffinityHit,
				PreferredConnID:          storeDecision.PreferredConnID,
				StoreFallbackReason:      storeDecision.FallbackReason,
				ConnReanchorBlockers:     deltaConnReanchorBlockers,
				Cached:                   cached,
				CachedFound:              found,
			}
			populateOpenAIWSDeltaShadowBindingDiagnostics(&shadowInput, stateStore, pool, groupID, apiKeyID, sessionHash, account.ID, cached, found)
			if openAIWSActiveDeltaEnabled() {
				if deltaPayload, deltaLog, applied, buildErr := buildOpenAIWSActiveDeltaPayload(shadowInput); buildErr == nil && applied {
					payload = deltaPayload
					payloadBytes = -1
					previousResponseID = openAIWSPayloadString(payload, "previous_response_id")
					previousResponseIDKind = ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
					storeEnabled = openAIWSPayloadStoreEnabled(payload)
					storeDisabled = openAIWSPayloadStoreDisabled(payload)
					storeDecision.StoreMode = openAIWSStoreModeIncremental
					storeDecision.StoreEnabled = storeEnabled
					storeDecision.StoreDisabled = storeDisabled
					storeDecision.StickyAccountID = account.ID
					storeDecision.StickyAccountHit = true
					storeDecision.StickyAccountMismatch = false
					storeDecision.ConnAffinityHit = true
					storeDecision.FallbackReason = "active_delta"
					connAffinityHit = true
					activeDeltaLog = deltaLog
					activeDeltaApplied = true
				} else {
					activeDeltaLog = deltaLog
					if buildErr != nil {
						activeDeltaLog.Candidate = false
						activeDeltaLog.FallbackReason = "active_delta_build_error"
					}
				}
			} else {
				activeDeltaLog = evaluateOpenAIWSDeltaShadowCandidate(shadowInput)
			}
		}
		logOpenAIWSDeltaShadow(activeDeltaLog)
	}
	defer func() {
		if shadowOwner {
			stateStore.EndSessionInFlight(groupID, apiKeyID, sessionHash)
		}
	}()

	previousResponseIDSource := openAIWSPreviousResponseIDSource(
		storeDecision.OriginalPreviousResponseID,
		previousResponseID,
		activeDeltaApplied,
		storeDecision.DroppedPreviousResponseID,
	)
	originalPreviousResponseIDPresent := strings.TrimSpace(storeDecision.OriginalPreviousResponseID) != ""
	connAgeMs := lease.ConnAge().Milliseconds()
	connIdleMs := lease.ConnIdleDuration().Milliseconds()
	connLeaseCount := lease.ConnLeaseCount()
	diagnosticStart := openAIWSDiagnosticStartLog{
		RequestID:                 requestID,
		ClientRequestID:           clientRequestID,
		AccountID:                 account.ID,
		AccountType:               account.Type,
		ConnProfile:               connProfile,
		ConnID:                    connID,
		ConnReused:                lease.Reused(),
		Transport:                 string(decision.Transport),
		Model:                     originalModel,
		Stream:                    streamValue,
		PayloadEventType:          payloadEventType,
		PayloadBytes:              resolvePayloadBytes(),
		PayloadKeys:               strings.Join(sortedKeys(payload), ","),
		InputSummary:              summarizeOpenAIWSInput(payload["input"]),
		PreviousResponseID:        previousResponseID,
		PreviousResponseIDKind:    previousResponseIDKind,
		PreviousResponseIDSource:  previousResponseIDSource,
		OriginalPreviousIDPresent: originalPreviousResponseIDPresent,
		PreferredConnID:           preferredConnID,
		StoreMode:                 storeDecision.StoreMode,
		StoreEnabled:              storeEnabled,
		StoreDisabled:             storeDisabled,
		StickyAccountID:           storeDecision.StickyAccountID,
		StickyAccountHit:          storeDecision.StickyAccountHit,
		StickyAccountMismatch:     storeDecision.StickyAccountMismatch,
		ConnAffinityHit:           connAffinityHit,
		FallbackReason:            storeDecision.FallbackReason,
		DroppedPreviousResponseID: storeDecision.DroppedPreviousResponseID,
		UnsafeToolContinuation:    storeDecision.UnsafeToolContinuation,
		DeltaActive:               activeDeltaApplied,
		DeltaItems:                activeDeltaLog.DeltaItems,
		DeltaBytes:                activeDeltaLog.DeltaBytes,
		FullItems:                 activeDeltaLog.FullItems,
		FullBytes:                 activeDeltaLog.FullBytes,
		ConnPickMs:                lease.ConnPickDuration().Milliseconds(),
		QueueWaitMs:               lease.QueueWaitDuration().Milliseconds(),
		ConnAgeMs:                 connAgeMs,
		ConnIdleMs:                connIdleMs,
		ConnLeaseCount:            connLeaseCount,
		SessionHash:               sessionHash,
		HeaderSessionID:           openAIWSHeaderValueForLog(wsHeaders, "session_id"),
		HeaderConversationID:      openAIWSHeaderValueForLog(wsHeaders, "conversation_id"),
		SessionIDSource:           sessionResolution.SessionSource,
		ConversationIDSource:      sessionResolution.ConversationSource,
		HasTurnState:              turnState != "",
		TurnStateLen:              len(turnState),
		HasPromptCacheKey:         promptCacheKey != "",
		HasTools:                  hasTools,
		HTTPIngressWSOneShot:      httpIngressWSOneShot,
		ForceNewConn:              forceNewConn,
		ForcePreferredConn:        forcePreferredConn,
		AffinityOnlyReuse:         affinityOnlyReuse,
		StoreDisabledConnMode:     storeDisabledConnMode,
		ProxyEnabled:              account.ProxyID != nil && account.Proxy != nil,
	}
	logOpenAIWSDiagnosticStart(diagnosticStart)
	SetOpsOpenAIWSTransportPath(c, openAIWSTransportPathFromStart(diagnosticStart))
	s.EmitOpenAIGatewayDebugTimelineEvent(c, OpenAIGatewayDebugTimelineEventInput{
		Stage:          "openai_ws_start",
		EndpointKind:   "responses",
		RequestStart:   startTime,
		Account:        account,
		RequestedModel: originalModel,
		Stream:         reqStream,
		Fields:         openAIWSDiagnosticStartTimelineFields(groupID, apiKeyID, diagnosticStart),
	})
	if previousResponseID != "" {
		logOpenAIWSContinuationProbe(openAIWSContinuationProbeLog{
			AccountID:                 account.ID,
			AccountType:               account.Type,
			ConnID:                    connID,
			PreviousResponseID:        previousResponseID,
			PreviousResponseIDKind:    previousResponseIDKind,
			PreviousResponseIDSource:  previousResponseIDSource,
			OriginalPreviousIDPresent: originalPreviousResponseIDPresent,
			PreferredConnID:           preferredConnID,
			ConnReused:                lease.Reused(),
			StoreDisabled:             storeDisabled,
			StoreMode:                 storeDecision.StoreMode,
			StoreEnabled:              storeEnabled,
			StickyAccountID:           storeDecision.StickyAccountID,
			StickyAccountHit:          storeDecision.StickyAccountHit,
			StickyAccountMismatch:     storeDecision.StickyAccountMismatch,
			ConnAffinityHit:           connAffinityHit,
			FallbackReason:            storeDecision.FallbackReason,
			PayloadBytes:              resolvePayloadBytes(),
			ConnPickMs:                lease.ConnPickDuration().Milliseconds(),
			QueueWaitMs:               lease.QueueWaitDuration().Milliseconds(),
			ConnAgeMs:                 connAgeMs,
			ConnIdleMs:                connIdleMs,
			ConnLeaseCount:            connLeaseCount,
			SessionHash:               sessionHash,
			HeaderSessionID:           openAIWSHeaderValueForLog(wsHeaders, "session_id"),
			HeaderConversationID:      openAIWSHeaderValueForLog(wsHeaders, "conversation_id"),
			SessionIDSource:           sessionResolution.SessionSource,
			ConversationIDSource:      sessionResolution.ConversationSource,
			HasTurnState:              turnState != "",
			TurnStateLen:              len(turnState),
			HasPromptCacheKey:         promptCacheKey != "",
		})
	}
	if c != nil {
		SetOpsLatencyMs(c, OpsOpenAIWSConnPickMsKey, lease.ConnPickDuration().Milliseconds())
		SetOpsLatencyMs(c, OpsOpenAIWSQueueWaitMsKey, lease.QueueWaitDuration().Milliseconds())
		c.Set(OpsOpenAIWSConnReusedKey, lease.Reused())
		if connID != "" {
			c.Set(OpsOpenAIWSConnIDKey, connID)
		}
	}

	handshakeTurnState := strings.TrimSpace(lease.HandshakeHeader(openAIWSTurnStateHeader))
	logOpenAIWSModeDebug(
		"handshake account_id=%d conn_id=%s has_turn_state=%v turn_state_len=%d",
		account.ID,
		connID,
		handshakeTurnState != "",
		len(handshakeTurnState),
	)
	if handshakeTurnState != "" {
		if stateStore != nil && sessionHash != "" {
			stateStore.BindSessionTurnState(groupID, sessionHash, handshakeTurnState, s.openAIWSSessionStickyTTL())
		}
		if c != nil {
			c.Header(http.CanonicalHeaderKey(openAIWSTurnStateHeader), handshakeTurnState)
		}
	}

	if !httpIngressWSOneShot {
		if err := s.performOpenAIWSGeneratePrewarm(
			ctx,
			lease,
			decision,
			payload,
			previousResponseID,
			reqBody,
			account,
			stateStore,
			groupID,
			apiKeyID,
		); err != nil {
			return nil, err
		}
	}

	sessionIdleTTL := time.Duration(0)
	if pool != nil {
		sessionIdleTTL = pool.sessionIdleTTL()
	}
	prewritePingThreshold := openAIWSSessionPrewritePingIdleThreshold(sessionIdleTTL)
	prewritePingAttempted := false
	prewritePingResult := "not_required"
	if shouldOpenAIWSSessionPrewritePing(account, connProfile, lease, httpIngressWSOneShot, prewritePingThreshold) {
		prewritePingAttempted = true
		prewritePingResult = "ok"
		pingLog := openAIWSPrewritePingLog{
			RequestID:                requestID,
			ClientRequestID:          clientRequestID,
			AccountID:                account.ID,
			AccountType:              account.Type,
			ConnProfile:              connProfile,
			ConnID:                   connID,
			ConnReused:               lease.Reused(),
			ConnAgeMS:                lease.ConnAge().Milliseconds(),
			ConnIdleMS:               lease.ConnIdleDuration().Milliseconds(),
			ConnLeaseCount:           lease.ConnLeaseCount(),
			SessionIdleTTLMS:         sessionIdleTTL.Milliseconds(),
			PrewritePingThresholdMS:  prewritePingThreshold.Milliseconds(),
			PreviousResponseID:       previousResponseID,
			PreviousResponseIDSource: previousResponseIDSource,
			StoreMode:                storeDecision.StoreMode,
			StoreFallbackReason:      storeDecision.FallbackReason,
			ActiveDelta:              activeDeltaApplied,
			HasFunctionCallOutput:    hasFunctionCallOutputForReanchor,
			SessionHash:              sessionHash,
			StickyAccountID:          storeDecision.StickyAccountID,
			StickyAccountHit:         storeDecision.StickyAccountHit,
			ConnAffinityHit:          connAffinityHit,
			Result:                   "ok",
		}
		if pingErr := lease.PingWithTimeout(openAIWSConnHealthCheckTO); pingErr != nil {
			prewritePingResult = "fail"
			pingLog.Result = "fail"
			pingLog.Cause = pingErr.Error()
			logOpenAIWSModeInfo("%s", openAIWSPrewritePingLogMessage(pingLog))
			s.logOpenAIWSBindingSnapshot(
				ctx,
				"prewrite_ping_fail",
				requestID,
				account,
				stateStore,
				pool,
				groupID,
				apiKeyID,
				sessionHash,
				previousResponseID,
				originalPreviousResponseIDPresent,
				connID,
				preferredConnID,
				storeDecision,
				"prewrite_ping_fail",
				"prewrite_ping_fail",
				"transport_write",
				pingErr.Error(),
			)
			lease.MarkBrokenFor("prewrite_ping_fail")
			return nil, wrapOpenAIWSFallback("write_request", pingErr)
		}
		logOpenAIWSModeInfo("%s", openAIWSPrewritePingLogMessage(pingLog))
	}

	writeSentMs := -1
	if err := lease.WriteJSONWithContextTimeout(ctx, payload, s.openAIWSWriteTimeout()); err != nil {
		connAgeMs := lease.ConnAge().Milliseconds()
		connIdleMs := lease.ConnIdleDuration().Milliseconds()
		connLeaseCount := lease.ConnLeaseCount()
		if isOpenAIWSSessionPreempted(ctx) {
			lease.MarkBrokenFor("session_preempted_write")
			logOpenAIWSModeInfo(
				"session_preempted account_id=%d account_type=%s request_id=%s conn_id=%s stage=write",
				account.ID,
				account.Type,
				truncateOpenAIWSLogValue(requestID, openAIWSIDValueMaxLen),
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
			)
			return nil, wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted)
		}
		cleanupReason := "write_request_fail"
		if !allowDeltaConnReanchor {
			cleanupReason = "write_request_fail_no_reanchor"
		}
		s.logOpenAIWSBindingSnapshot(
			ctx,
			"write_request_fail",
			requestID,
			account,
			stateStore,
			pool,
			groupID,
			apiKeyID,
			sessionHash,
			previousResponseID,
			originalPreviousResponseIDPresent,
			connID,
			preferredConnID,
			storeDecision,
			"write_request_fail",
			cleanupReason,
			"transport_write",
			err.Error(),
		)
		lease.MarkBrokenFor(cleanupReason)
		var proxyID int64
		if account.ProxyID != nil {
			proxyID = *account.ProxyID
		}
		logOpenAIWSModeInfo("%s", openAIWSWriteRequestFailLogMessage(openAIWSWriteRequestFailLog{
			RequestID:                requestID,
			ClientRequestID:          clientRequestID,
			AccountID:                account.ID,
			AccountType:              account.Type,
			Model:                    originalModel,
			UpstreamModel:            mappedModel,
			ConnProfile:              connProfile,
			ConnID:                   connID,
			ConnReused:               lease.Reused(),
			Transport:                string(decision.Transport),
			Attempt:                  attempt,
			ConnPickMs:               lease.ConnPickDuration().Milliseconds(),
			QueueWaitMs:              lease.QueueWaitDuration().Milliseconds(),
			ConnAgeMs:                connAgeMs,
			ConnIdleMs:               connIdleMs,
			ConnLeaseCount:           connLeaseCount,
			SessionIdleTTLMS:         sessionIdleTTL.Milliseconds(),
			PrewritePingThresholdMS:  prewritePingThreshold.Milliseconds(),
			PrewritePingAttempted:    prewritePingAttempted,
			PrewritePingResult:       prewritePingResult,
			PayloadBytes:             resolvePayloadBytes(),
			PreviousResponseID:       previousResponseID,
			PreviousResponseIDKind:   previousResponseIDKind,
			PreviousResponseIDSource: previousResponseIDSource,
			StoreMode:                storeDecision.StoreMode,
			StoreEnabled:             storeEnabled,
			StoreDisabled:            storeDisabled,
			StickyAccountID:          storeDecision.StickyAccountID,
			StickyAccountHit:         storeDecision.StickyAccountHit,
			ConnAffinityHit:          connAffinityHit,
			PreferredConnID:          preferredConnID,
			StoreFallbackReason:      storeDecision.FallbackReason,
			CleanupReason:            cleanupReason,
			AllowDeltaConnReanchor:   allowDeltaConnReanchor,
			ConnReanchorBlockers:     deltaConnReanchorBlockers,
			SessionHash:              sessionHash,
			HasPromptCacheKey:        promptCacheKey != "",
			HasTurnState:             turnState != "",
			TurnStateLen:             len(turnState),
			HTTPIngressWSOneShot:     httpIngressWSOneShot,
			ForceNewConn:             forceNewConn,
			AffinityOnlyReuse:        affinityOnlyReuse,
			HasFunctionCallOutput:    hasFunctionCallOutputForReanchor,
			ActiveDelta:              activeDeltaApplied,
			DeltaItems:               activeDeltaLog.DeltaItems,
			DeltaBytes:               activeDeltaLog.DeltaBytes,
			FullItems:                activeDeltaLog.FullItems,
			FullBytes:                activeDeltaLog.FullBytes,
			ProxyEnabled:             account.ProxyID != nil && account.Proxy != nil,
			ProxyID:                  proxyID,
			Cause:                    err.Error(),
		}))
		return nil, wrapOpenAIWSFallback("write_request", err)
	}
	writeSentMs = int(time.Since(startTime).Milliseconds())
	if debugEnabled {
		logOpenAIWSModeDebug(
			"write_request_sent account_id=%d conn_id=%s stream=%v payload_bytes=%d previous_response_id=%s",
			account.ID,
			connID,
			reqStream,
			resolvePayloadBytes(),
			truncateOpenAIWSLogValue(previousResponseID, openAIWSIDValueMaxLen),
		)
	}

	usage := &OpenAIUsage{}
	imageCounter := newOpenAIImageOutputCounter()
	var firstTokenMs *int
	var firstEventMs *int
	firstTokenEventType := ""
	responseID := ""
	var finalResponse []byte
	wroteDownstream := false
	needModelReplace := originalModel != mappedModel
	var mappedModelBytes []byte
	if needModelReplace && mappedModel != "" {
		mappedModelBytes = []byte(mappedModel)
	}
	bufferedStreamEvents := make([][]byte, 0, 4)
	eventCount := 0
	tokenEventCount := 0
	terminalEventCount := 0
	bufferedEventCount := 0
	flushedBufferedEventCount := 0
	recoveredUpstreamTransportEOF := false
	firstEventType := ""
	lastEventType := ""
	var deltaShadowRawOutputHashes [][32]byte
	var deltaShadowRawOutputShapes []string
	deltaShadowOutputCaptured := false
	deltaShadowRawClientEquiv := true
	deltaShadowRawDoneItems := map[int]json.RawMessage{}
	deltaShadowVisibleDoneItems := map[int]json.RawMessage{}
	nonStreamOutputDoneItems := map[int]json.RawMessage{}

	var flusher http.Flusher
	if reqStream {
		if s.responseHeaderFilter != nil {
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), http.Header{}, s.responseHeaderFilter)
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		f, ok := c.Writer.(http.Flusher)
		if !ok {
			lease.MarkBrokenFor("streaming_not_supported")
			return nil, wrapOpenAIWSFallback("streaming_not_supported", errors.New("streaming not supported"))
		}
		flusher = f
	}

	flushBatchSize := s.openAIWSEventFlushBatchSize()
	flushInterval := s.openAIWSEventFlushInterval()
	pendingFlushEvents := 0
	lastFlushAt := time.Now()
	flushStreamWriter := func(force bool) {
		if clientDisconnected || flusher == nil || pendingFlushEvents <= 0 {
			return
		}
		if !force && flushBatchSize > 1 && pendingFlushEvents < flushBatchSize {
			if flushInterval <= 0 || time.Since(lastFlushAt) < flushInterval {
				return
			}
		}
		flusher.Flush()
		pendingFlushEvents = 0
		lastFlushAt = time.Now()
	}
	emitStreamMessage := func(message []byte, forceFlush bool) {
		if clientDisconnected {
			return
		}
		frame := make([]byte, 0, len(message)+8)
		frame = append(frame, "data: "...)
		frame = append(frame, message...)
		frame = append(frame, '\n', '\n')
		_, wErr := c.Writer.Write(frame)
		if wErr == nil {
			wroteDownstream = true
			pendingFlushEvents++
			flushStreamWriter(forceFlush)
			return
		}
		clientDisconnected = true
		logger.LegacyPrintf("service.openai_gateway", "[OpenAI WS Mode] client disconnected, continue draining upstream: account=%d", account.ID)
	}
	flushBufferedStreamEvents := func(reason string) {
		if len(bufferedStreamEvents) == 0 {
			return
		}
		flushed := len(bufferedStreamEvents)
		for _, buffered := range bufferedStreamEvents {
			emitStreamMessage(buffered, false)
		}
		bufferedStreamEvents = bufferedStreamEvents[:0]
		flushStreamWriter(true)
		flushedBufferedEventCount += flushed
		if debugEnabled {
			logOpenAIWSModeDebug(
				"buffer_flush account_id=%d conn_id=%s reason=%s flushed=%d total_flushed=%d client_disconnected=%v",
				account.ID,
				connID,
				truncateOpenAIWSLogValue(reason, openAIWSLogValueMaxLen),
				flushed,
				flushedBufferedEventCount,
				clientDisconnected,
			)
		}
	}

	readTimeout := s.openAIWSReadTimeout()

	for {
		message, readErr := lease.ReadMessageWithContextTimeout(ctx, readTimeout)
		if readErr != nil {
			if isOpenAIWSSessionPreempted(ctx) {
				lease.MarkBrokenFor("session_preempted_read")
				logOpenAIWSModeInfo(
					"session_preempted account_id=%d account_type=%s request_id=%s conn_id=%s stage=read wrote_downstream=%v events=%d token_events=%d terminal_events=%d",
					account.ID,
					account.Type,
					truncateOpenAIWSLogValue(requestID, openAIWSIDValueMaxLen),
					truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
					wroteDownstream,
					eventCount,
					tokenEventCount,
					terminalEventCount,
				)
				return nil, wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted)
			}
			lease.MarkBrokenFor("read_fail")
			closeStatus, closeReason := summarizeOpenAIWSReadCloseError(readErr)
			proxyID := int64(0)
			if account.ProxyID != nil {
				proxyID = *account.ProxyID
			}
			firstEventMsValue := -1
			if firstEventMs != nil {
				firstEventMsValue = *firstEventMs
			}
			firstTokenMsValue := -1
			if firstTokenMs != nil {
				firstTokenMsValue = *firstTokenMs
			}
			logOpenAIWSReadFail(openAIWSReadFailLog{
				RequestID:                requestID,
				ClientRequestID:          clientRequestID,
				AccountID:                account.ID,
				AccountType:              account.Type,
				Model:                    originalModel,
				UpstreamModel:            mappedModel,
				ConnProfile:              connProfile,
				ConnID:                   connID,
				ConnReused:               lease.Reused(),
				Transport:                string(decision.Transport),
				Attempt:                  attempt,
				ConnPickMs:               lease.ConnPickDuration().Milliseconds(),
				QueueWaitMs:              lease.QueueWaitDuration().Milliseconds(),
				ConnAgeMs:                lease.ConnAge().Milliseconds(),
				ConnIdleMs:               lease.ConnIdleDuration().Milliseconds(),
				ConnLeaseCount:           lease.ConnLeaseCount(),
				PayloadBytes:             resolvePayloadBytes(),
				PreviousResponseID:       previousResponseID,
				PreviousResponseIDKind:   previousResponseIDKind,
				PreviousResponseIDSource: previousResponseIDSource,
				StoreMode:                storeDecision.StoreMode,
				StoreEnabled:             storeEnabled,
				StoreDisabled:            storeDisabled,
				StickyAccountID:          storeDecision.StickyAccountID,
				StickyAccountHit:         storeDecision.StickyAccountHit,
				ConnAffinityHit:          connAffinityHit,
				PreferredConnID:          preferredConnID,
				StoreFallbackReason:      storeDecision.FallbackReason,
				SessionHash:              sessionHash,
				HasPromptCacheKey:        promptCacheKey != "",
				HasTurnState:             turnState != "",
				TurnStateLen:             len(turnState),
				ProxyEnabled:             account.ProxyID != nil && account.Proxy != nil,
				ProxyID:                  proxyID,
				WroteDownstream:          wroteDownstream,
				CloseStatus:              closeStatus,
				CloseReason:              closeReason,
				Cause:                    readErr.Error(),
				Events:                   eventCount,
				TokenEvents:              tokenEventCount,
				TerminalEvents:           terminalEventCount,
				FirstEventMs:             firstEventMsValue,
				FirstTokenMs:             firstTokenMsValue,
				FirstTokenEvent:          firstTokenEventType,
				BufferedPending:          len(bufferedStreamEvents),
				BufferedFlushed:          flushedBufferedEventCount,
				FirstEvent:               firstEventType,
				LastEvent:                lastEventType,
			})
			if !wroteDownstream {
				return nil, wrapOpenAIWSFallback(classifyOpenAIWSReadFallbackReason(readErr), readErr)
			}
			if clientDisconnected {
				break
			}
			detail := recordDetailedUpstreamTransportError(c, readErr)
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:    account.Platform,
				AccountID:   account.ID,
				AccountName: account.Name,
				Kind:        "request_error",
				Message:     detail.Message,
				Detail:      "upstream websocket read failed after downstream stream started: " + detail.Detail,
			})
			errMessage := "upstream websocket read failed after downstream stream started"
			if detail.Message != "" {
				errMessage = detail.Message
			}
			syntheticFailed := buildOpenAIWSSyntheticReadFailureEvent(responseID, originalModel, errMessage, usage)
			emitStreamMessage(syntheticFailed, true)
			recoveredUpstreamTransportEOF = true
			terminalEventCount++
			lastEventType = "response.failed"
			cleanExit = false
			return nil, fmt.Errorf("openai ws read event after downstream stream started: %w", readErr)
		}

		eventType, eventResponseID, responseField := parseOpenAIWSEventEnvelope(message)
		if eventType == "" {
			continue
		}
		eventCount++
		if firstEventType == "" {
			firstEventType = eventType
			ms := int(time.Since(startTime).Milliseconds())
			firstEventMs = &ms
		}
		lastEventType = eventType

		if responseID == "" && eventResponseID != "" {
			responseID = eventResponseID
		}
		if deltaShadowEnabled && !deltaShadowOutputCaptured && responseField.Exists() {
			if outItems, ok := openAIWSExtractResponseOutputItems([]byte(responseField.Raw)); ok {
				if hashes, hok := openAIWSCanonicalItemHashes(outItems); hok {
					if shapes, sok := openAIWSItemShapes(outItems); sok {
						deltaShadowRawOutputHashes = hashes
						deltaShadowRawOutputShapes = shapes
						deltaShadowOutputCaptured = true
					}
				}
			}
		}
		if outputIndex, item, ok := openAIWSExtractOutputItemDoneItem(message); ok {
			if !reqStream {
				nonStreamOutputDoneItems[outputIndex] = item
			}
			if deltaShadowEnabled {
				deltaShadowRawDoneItems[outputIndex] = item
			}
		}

		isTokenEvent := isOpenAIWSTokenEvent(eventType)
		if isTokenEvent {
			tokenEventCount++
		}
		isTerminalEvent := isOpenAIWSTerminalEvent(eventType)
		if isTerminalEvent {
			terminalEventCount++
		}
		if firstTokenMs == nil && isTokenEvent {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
			firstTokenEventType = eventType
		}
		if debugEnabled && shouldLogOpenAIWSEvent(eventCount, eventType) {
			logOpenAIWSModeDebug(
				"event_received account_id=%d conn_id=%s idx=%d type=%s bytes=%d token=%v terminal=%v buffered_pending=%d",
				account.ID,
				connID,
				eventCount,
				truncateOpenAIWSLogValue(eventType, openAIWSLogValueMaxLen),
				len(message),
				isTokenEvent,
				isTerminalEvent,
				len(bufferedStreamEvents),
			)
		}

		if !clientDisconnected {
			if needModelReplace && len(mappedModelBytes) > 0 && openAIWSEventMayContainModel(eventType) && bytes.Contains(message, mappedModelBytes) {
				message = replaceOpenAIWSMessageModel(message, mappedModel, originalModel)
			}
			if openAIWSEventMayContainToolCalls(eventType) && openAIWSMessageLikelyContainsToolCalls(message) {
				if corrected, changed := s.toolCorrector.CorrectToolCallsInSSEBytes(message); changed {
					message = corrected
				}
			}
		}
		if deltaShadowEnabled && !clientDisconnected {
			if outputIndex, item, ok := openAIWSExtractOutputItemDoneItem(message); ok {
				deltaShadowVisibleDoneItems[outputIndex] = item
			}
		}
		if reqStream && isTerminalEvent && deltaShadowEnabled && deltaShadowOutputCaptured && !clientDisconnected {
			responseRaw := gjson.GetBytes(message, "response")
			if responseRaw.Exists() {
				if cvItems, ok := openAIWSExtractResponseOutputItems([]byte(responseRaw.Raw)); ok {
					if cvHashes, hok := openAIWSCanonicalItemHashes(cvItems); hok {
						deltaShadowRawClientEquiv = openAIWSHashSlicesEqual(deltaShadowRawOutputHashes, cvHashes)
					}
				}
			}
		}
		if reqStream && isTerminalEvent && deltaShadowEnabled && !deltaShadowOutputCaptured && len(deltaShadowRawDoneItems) > 0 {
			rawItems := openAIWSOrderedOutputDoneItems(deltaShadowRawDoneItems)
			if hashes, hok := openAIWSCanonicalItemHashes(rawItems); hok {
				if shapes, sok := openAIWSItemShapes(rawItems); sok {
					deltaShadowRawOutputHashes = hashes
					deltaShadowRawOutputShapes = shapes
					deltaShadowOutputCaptured = true
					visibleItems := openAIWSOrderedOutputDoneItems(deltaShadowVisibleDoneItems)
					if cvHashes, vok := openAIWSCanonicalItemHashes(visibleItems); vok {
						deltaShadowRawClientEquiv = openAIWSHashSlicesEqual(deltaShadowRawOutputHashes, cvHashes)
					} else {
						deltaShadowRawClientEquiv = false
					}
				}
			}
		}
		if openAIWSEventShouldParseUsage(eventType) {
			parseOpenAIWSResponseUsageFromCompletedEvent(message, usage)
		}
		imageCounter.AddSSEData(message)
		if advisoryMsg, ok := classifyOpenAIWSSoftRateLimitAdvisory(message); ok {
			s.persistOpenAIWSRateLimitSignal(ctx, account, lease.HandshakeHeaders(), nil, "rate_limit_exceeded", "rate_limit_error", advisoryMsg)
			logOpenAIWSModeInfo(
				"soft_rate_limit_advisory account_id=%d conn_id=%s idx=%d event_type=%s message=%s",
				account.ID,
				connID,
				eventCount,
				truncateOpenAIWSLogValue(eventType, openAIWSLogValueMaxLen),
				truncateOpenAIWSLogValue(advisoryMsg, openAIWSLogValueMaxLen),
			)
			lease.MarkBrokenFor("soft_rate_limit_advisory")
			if !wroteDownstream {
				return nil, wrapOpenAIWSFallback("upstream_rate_limited", errors.New(advisoryMsg))
			}
			setOpsUpstreamError(c, http.StatusTooManyRequests, advisoryMsg, "")
			return nil, fmt.Errorf("openai ws soft rate limit advisory: %s", advisoryMsg)
		}

		if eventType == "response.failed" {
			if hit, code, msg := detectOpenAICyberPolicy(message); hit {
				MarkOpsCyberPolicy(c, CyberPolicyMark{
					Code:           code,
					Message:        msg,
					Body:           truncateString(string(message), 4096),
					UpstreamStatus: http.StatusOK,
					UpstreamInTok:  usage.InputTokens,
					UpstreamOutTok: usage.OutputTokens,
				})
			}
		}

		if eventType == "error" {
			errCodeRaw, errTypeRaw, errMsgRaw := parseOpenAIWSErrorEventFields(message)
			_ = markOpsCyberPolicyIfDetected(c, message, http.StatusOK, usage.InputTokens, usage.OutputTokens)
			s.persistOpenAIWSRateLimitSignal(ctx, account, lease.HandshakeHeaders(), message, errCodeRaw, errTypeRaw, errMsgRaw)
			errMsg := strings.TrimSpace(errMsgRaw)
			if errMsg == "" {
				errMsg = "Upstream websocket error"
			}
			fallbackReason, canFallback := classifyOpenAIWSErrorEventFromRaw(errCodeRaw, errTypeRaw, errMsgRaw)
			if fallbackReason == "ws_connection_limit_reached" && lease.ConnAge() >= openAIWSConnLimitTTLEvictThreshold {
				fallbackReason = "ws_conn_ttl_evict"
			}
			errCode, errType, errMessage := summarizeOpenAIWSErrorEventFieldsFromRaw(errCodeRaw, errTypeRaw, errMsgRaw)
			s.logOpenAIWSBindingSnapshot(
				ctx,
				"error_event",
				requestID,
				account,
				stateStore,
				pool,
				groupID,
				apiKeyID,
				sessionHash,
				previousResponseID,
				originalPreviousResponseIDPresent,
				connID,
				preferredConnID,
				storeDecision,
				fallbackReason,
				errCode,
				errType,
				errMessage,
			)
			s.logOpenAIWSMismatchErrorProbe(
				ctx,
				requestID,
				clientRequestID,
				account,
				stateStore,
				pool,
				groupID,
				apiKeyID,
				sessionHash,
				previousResponseID,
				previousResponseIDSource,
				originalPreviousResponseIDPresent,
				responseID,
				connProfile,
				connID,
				lease.Reused(),
				preferredConnID,
				storeDecision,
				storeDecision.StoreMode,
				activeDeltaApplied,
				activeDeltaLog.DeltaItems,
				activeDeltaLog.FullItems,
				fallbackReason,
				canFallback,
				wroteDownstream,
				errCode,
				errType,
				errMessage,
			)
			logOpenAIWSModeInfo("%s", openAIWSErrorEventLogMessage(openAIWSErrorEventLog{
				RequestID:                 requestID,
				ClientRequestID:           clientRequestID,
				AccountID:                 account.ID,
				AccountType:               account.Type,
				ConnProfile:               connProfile,
				ConnID:                    connID,
				ConnReused:                lease.Reused(),
				Transport:                 string(decision.Transport),
				Attempt:                   attempt,
				ConnAgeMs:                 lease.ConnAge().Milliseconds(),
				ConnIdleMs:                lease.ConnIdleDuration().Milliseconds(),
				ConnLeaseCount:            lease.ConnLeaseCount(),
				PayloadBytes:              resolvePayloadBytes(),
				ResponseID:                responseID,
				PreviousResponseID:        previousResponseID,
				PreviousResponseIDKind:    previousResponseIDKind,
				PreviousResponseIDSource:  previousResponseIDSource,
				OriginalPreviousIDPresent: originalPreviousResponseIDPresent,
				StoreMode:                 storeDecision.StoreMode,
				StoreDisabled:             storeDisabled,
				StickyAccountID:           storeDecision.StickyAccountID,
				StickyAccountHit:          storeDecision.StickyAccountHit,
				ConnAffinityHit:           connAffinityHit,
				PreferredConnID:           preferredConnID,
				StoreFallbackReason:       storeDecision.FallbackReason,
				CleanupReason:             "error_event",
				ActiveDelta:               activeDeltaApplied,
				DeltaItems:                activeDeltaLog.DeltaItems,
				FullItems:                 activeDeltaLog.FullItems,
				EventIndex:                eventCount,
				FallbackReason:            fallbackReason,
				CanFallback:               canFallback,
				ErrorCode:                 errCode,
				ErrorType:                 errType,
				ErrorMessage:              errMessage,
				SessionHash:               sessionHash,
				HeaderSessionID:           openAIWSHeaderValueForLog(wsHeaders, "session_id"),
				HeaderConversationID:      openAIWSHeaderValueForLog(wsHeaders, "conversation_id"),
				SessionIDSource:           sessionResolution.SessionSource,
				ConversationIDSource:      sessionResolution.ConversationSource,
				HTTPIngressWSOneShot:      httpIngressWSOneShot,
				HasPromptCacheKey:         promptCacheKey != "",
				HasTurnState:              turnState != "",
				TurnStateLen:              len(turnState),
			}))
			if fallbackReason == "previous_response_not_found" {
				logOpenAIWSModeInfo(
					"previous_response_not_found_diag account_id=%d account_type=%s conn_id=%s previous_response_id=%s previous_response_id_kind=%s response_id=%s event_idx=%d req_stream=%v store_disabled=%v conn_reused=%v session_hash=%s header_session_id=%s header_conversation_id=%s session_id_source=%s conversation_id_source=%s has_turn_state=%v turn_state_len=%d has_prompt_cache_key=%v err_code=%s err_type=%s err_message=%s",
					account.ID,
					account.Type,
					connID,
					truncateOpenAIWSLogValue(previousResponseID, openAIWSIDValueMaxLen),
					normalizeOpenAIWSLogValue(previousResponseIDKind),
					truncateOpenAIWSLogValue(responseID, openAIWSIDValueMaxLen),
					eventCount,
					reqStream,
					storeDisabled,
					lease.Reused(),
					truncateOpenAIWSLogValue(sessionHash, 12),
					openAIWSHeaderValueForLog(wsHeaders, "session_id"),
					openAIWSHeaderValueForLog(wsHeaders, "conversation_id"),
					normalizeOpenAIWSLogValue(sessionResolution.SessionSource),
					normalizeOpenAIWSLogValue(sessionResolution.ConversationSource),
					turnState != "",
					len(turnState),
					promptCacheKey != "",
					errCode,
					errType,
					errMessage,
				)
			}
			// error 事件后连接不再可复用，避免回池后污染下一请求。
			lease.MarkBrokenFor("error_event")
			if !wroteDownstream && canFallback {
				return nil, wrapOpenAIWSFallbackWithPayloadState(fallbackReason, errors.New(errMsg), previousResponseID, activeDeltaApplied)
			}
			statusCode := openAIWSErrorHTTPStatusFromRaw(errCodeRaw, errTypeRaw)
			setOpsUpstreamError(c, statusCode, errMsg, "")
			if reqStream && !clientDisconnected {
				flushBufferedStreamEvents("error_event")
				emitStreamMessage(message, true)
			}
			if !reqStream {
				c.JSON(statusCode, gin.H{
					"error": gin.H{
						"type":    "upstream_error",
						"message": errMsg,
					},
				})
			}
			return nil, fmt.Errorf("openai ws error event: %s", errMsg)
		}
		if eventType == "response.failed" {
			errCodeRaw, errTypeRaw, errMsgRaw := parseOpenAIWSResponseFailedErrorFields(message)
			errMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(errMsgRaw))
			if errMsg == "" {
				errMsg = "Upstream response failed"
			}
			failedErr := &openAIWSResponseFailedError{
				code:      errCodeRaw,
				errType:   errTypeRaw,
				message:   errMsg,
				retryable: openAIStreamFailedEventShouldFailover(message, errMsg),
			}
			lease.MarkBrokenFor("response_failed")
			if !wroteDownstream {
				return nil, wrapOpenAIWSFallback("response_failed", failedErr)
			}
			statusCode := openAIWSErrorHTTPStatusFromRaw(errCodeRaw, errTypeRaw)
			setOpsUpstreamError(c, statusCode, errMsg, "")
			if reqStream && !clientDisconnected {
				flushBufferedStreamEvents("response_failed")
				emitStreamMessage(message, true)
			}
			return nil, fmt.Errorf("upstream response failed: %s", errMsg)
		}

		if reqStream {
			// 在首个 token 前先缓冲事件（如 response.created），
			// 以便上游早期断连时仍可安全回退到 HTTP，不给下游发送半截流。
			shouldBuffer := firstTokenMs == nil && !isTokenEvent && !isTerminalEvent
			if shouldBuffer {
				buffered := make([]byte, len(message))
				copy(buffered, message)
				bufferedStreamEvents = append(bufferedStreamEvents, buffered)
				bufferedEventCount++
				if debugEnabled && shouldLogOpenAIWSBufferedEvent(bufferedEventCount) {
					logOpenAIWSModeDebug(
						"buffer_enqueue account_id=%d conn_id=%s idx=%d event_idx=%d event_type=%s buffer_size=%d",
						account.ID,
						connID,
						bufferedEventCount,
						eventCount,
						truncateOpenAIWSLogValue(eventType, openAIWSLogValueMaxLen),
						len(bufferedStreamEvents),
					)
				}
			} else {
				flushBufferedStreamEvents(eventType)
				emitStreamMessage(message, isTerminalEvent)
			}
		} else {
			if responseField.Exists() && responseField.Type == gjson.JSON {
				finalResponse = []byte(responseField.Raw)
			}
		}

		if isTerminalEvent {
			cleanExit = true
			break
		}
	}

	if !reqStream {
		if len(finalResponse) == 0 {
			logOpenAIWSModeInfo(
				"missing_final_response account_id=%d conn_id=%s events=%d token_events=%d terminal_events=%d wrote_downstream=%v",
				account.ID,
				connID,
				eventCount,
				tokenEventCount,
				terminalEventCount,
				wroteDownstream,
			)
			if !wroteDownstream {
				return nil, wrapOpenAIWSFallback("missing_final_response", errors.New("no terminal response payload"))
			}
			return nil, errors.New("ws finished without final response")
		}

		if needModelReplace {
			finalResponse = s.replaceModelInResponseBody(finalResponse, mappedModel, originalModel)
		}
		finalResponse = openAIWSMaterializeResponseOutputFromDoneItems(finalResponse, nonStreamOutputDoneItems)
		finalResponse = s.correctToolCallsInResponseBody(finalResponse)
		if deltaShadowEnabled && deltaShadowOutputCaptured && !clientDisconnected {
			if cvItems, ok := openAIWSExtractResponseOutputItems(finalResponse); ok {
				if cvHashes, hok := openAIWSCanonicalItemHashes(cvItems); hok {
					deltaShadowRawClientEquiv = openAIWSHashSlicesEqual(deltaShadowRawOutputHashes, cvHashes)
				}
			}
		}
		populateOpenAIUsageFromResponseJSON(finalResponse, usage)
		if responseID == "" {
			responseID = strings.TrimSpace(gjson.GetBytes(finalResponse, "id").String())
		}

		c.Data(http.StatusOK, "application/json", finalResponse)
	} else {
		flushStreamWriter(true)
	}

	if responseID != "" && stateStore != nil && !clientDisconnected && !recoveredUpstreamTransportEOF {
		ttl := s.openAIWSResponseStickyTTL()
		bindAccountErr := stateStore.BindResponseAccount(ctx, groupID, apiKeyID, responseID, account.ID, ttl)
		logOpenAIWSBindResponseAccountWarn(groupID, account.ID, responseID, bindAccountErr)
		boundConnID := ""
		if !httpIngressWSOneShot {
			boundConnID = lease.ConnID()
			stateStore.BindResponseConn(groupID, apiKeyID, responseID, boundConnID, ttl)
		}
		logOpenAIWSResponseStickyBind(groupID, apiKeyID, account.ID, account.Type, responseID, boundConnID, ttl, bindAccountErr)
	}
	if !httpIngressWSOneShot && stateStore != nil && storeDisabled && sessionHash != "" && !clientDisconnected && !recoveredUpstreamTransportEOF {
		ttl := s.openAIWSSessionStickyTTL()
		stateStore.BindSessionConn(groupID, apiKeyID, account.ID, sessionHash, lease.ConnID(), ttl)
		logOpenAIWSSessionConnBind(groupID, apiKeyID, account.ID, account.Type, sessionHash, lease.ConnID(), ttl)
	}
	if shadowOwner && deltaShadowEnabled && stateStore != nil && responseID != "" && connID != "" &&
		!clientDisconnected && !recoveredUpstreamTransportEOF {
		if inputItems, _, ierr := openAIWSExtractNormalizedInputSequence(contextPayloadRaw); ierr == nil {
			if inputHashes, hok := openAIWSCanonicalItemHashes(inputItems); hok {
				var materializedShapes []string
				if inputShapes, sok := openAIWSItemShapes(inputItems); sok && len(deltaShadowRawOutputShapes) == len(deltaShadowRawOutputHashes) {
					materializedShapes = make([]string, 0, len(inputShapes)+len(deltaShadowRawOutputShapes))
					materializedShapes = append(materializedShapes, inputShapes...)
					materializedShapes = append(materializedShapes, deltaShadowRawOutputShapes...)
				}
				materialized := make([][32]byte, 0, len(inputHashes)+len(deltaShadowRawOutputHashes))
				materialized = append(materialized, inputHashes...)
				materialized = append(materialized, deltaShadowRawOutputHashes...)
				nonInputHash, _, nonInputFields := openAIWSNonInputFingerprint(contextPayloadRaw)
				ttl := s.openAIWSSessionStickyTTL()
				stateStore.BindConnLastResponse(connID, responseID, ttl)
				sessionContextValue := openAIWSSessionContextValue{
					accountID:               account.ID,
					connID:                  connID,
					lastResponseID:          responseID,
					materializedHashes:      materialized,
					materializedShapes:      materializedShapes,
					materializedCount:       len(materialized),
					inputCount:              len(inputHashes),
					inputOnlyContext:        !deltaShadowOutputCaptured,
					nonInputHash:            nonInputHash,
					nonInputFields:          nonInputFields,
					rawVsClientVisibleEqual: deltaShadowRawClientEquiv,
				}
				stateStore.BindSessionContext(groupID, apiKeyID, sessionHash, sessionContextValue, ttl)
				logOpenAIWSSessionContextBind(
					groupID,
					apiKeyID,
					account.ID,
					account.Type,
					sessionHash,
					connID,
					responseID,
					ttl,
					len(inputHashes),
					len(materialized),
					deltaShadowOutputCaptured,
					deltaShadowRawClientEquiv,
				)
			}
		}
	}
	firstTokenMsValue := -1
	if firstTokenMs != nil {
		firstTokenMsValue = *firstTokenMs
	}
	firstEventMsValue := -1
	if firstEventMs != nil {
		firstEventMsValue = *firstEventMs
	}
	firstTokenAfterFirstEventMs := -1
	if firstTokenMsValue >= 0 && firstEventMsValue >= 0 {
		firstTokenAfterFirstEventMs = firstTokenMsValue - firstEventMsValue
	}
	durationMs := time.Since(startTime).Milliseconds()
	diagnosticCompleted := openAIWSDiagnosticCompletedLog{
		RequestID:                   requestID,
		ClientRequestID:             clientRequestID,
		AccountID:                   account.ID,
		AccountType:                 account.Type,
		ConnProfile:                 connProfile,
		ConnID:                      connID,
		ConnReused:                  lease.Reused(),
		ResponseID:                  strings.TrimSpace(responseID),
		Model:                       originalModel,
		UpstreamModel:               mappedModel,
		Stream:                      reqStream,
		StoreMode:                   storeDecision.StoreMode,
		StoreEnabled:                storeEnabled,
		StoreDisabled:               storeDisabled,
		HasPreviousResponseID:       previousResponseID != "",
		PreviousResponseIDKind:      previousResponseIDKind,
		PreviousResponseIDSource:    previousResponseIDSource,
		OriginalPreviousIDPresent:   originalPreviousResponseIDPresent,
		PayloadBytes:                resolvePayloadBytes(),
		ConnAgeMs:                   lease.ConnAge().Milliseconds(),
		ConnIdleMs:                  lease.ConnIdleDuration().Milliseconds(),
		ConnLeaseCount:              lease.ConnLeaseCount(),
		WriteSentMs:                 writeSentMs,
		DurationMs:                  durationMs,
		FirstTokenMs:                firstTokenMsValue,
		FirstEventMs:                firstEventMsValue,
		FirstTokenEvent:             firstTokenEventType,
		FirstTokenAfterFirstEventMs: firstTokenAfterFirstEventMs,
		Events:                      eventCount,
		TokenEvents:                 tokenEventCount,
		TerminalEvents:              terminalEventCount,
		BufferedEvents:              bufferedEventCount,
		BufferedFlushed:             flushedBufferedEventCount,
		FirstEvent:                  firstEventType,
		LastEvent:                   lastEventType,
		WroteDownstream:             wroteDownstream,
		ClientDisconnected:          clientDisconnected,
		HTTPIngressWSOneShot:        httpIngressWSOneShot,
		InputTokens:                 usage.InputTokens,
		CacheReadTokens:             usage.CacheReadInputTokens,
		CacheCreationTokens:         usage.CacheCreationInputTokens,
		OutputTokens:                usage.OutputTokens,
		DeltaActive:                 activeDeltaApplied,
		DeltaItems:                  activeDeltaLog.DeltaItems,
		DeltaBytes:                  activeDeltaLog.DeltaBytes,
		FullItems:                   activeDeltaLog.FullItems,
		FullBytes:                   activeDeltaLog.FullBytes,
	}
	logOpenAIWSDiagnosticCompleted(diagnosticCompleted)
	s.EmitOpenAIGatewayDebugTimelineEvent(c, OpenAIGatewayDebugTimelineEventInput{
		Stage:          "openai_ws_completed",
		EndpointKind:   "responses",
		RequestStart:   startTime,
		Account:        account,
		RequestedModel: originalModel,
		Stream:         reqStream,
		Fields:         openAIWSDiagnosticCompletedTimelineFields(groupID, apiKeyID, diagnosticCompleted),
	})
	logOpenAIWSModeDebug(
		"completed account_id=%d conn_id=%s response_id=%s stream=%v duration_ms=%d events=%d token_events=%d terminal_events=%d buffered_events=%d buffered_flushed=%d first_event=%s last_event=%s first_token_ms=%d wrote_downstream=%v client_disconnected=%v http_ingress_ws_one_shot=%v",
		account.ID,
		connID,
		truncateOpenAIWSLogValue(strings.TrimSpace(responseID), openAIWSIDValueMaxLen),
		reqStream,
		durationMs,
		eventCount,
		tokenEventCount,
		terminalEventCount,
		bufferedEventCount,
		flushedBufferedEventCount,
		truncateOpenAIWSLogValue(firstEventType, openAIWSLogValueMaxLen),
		truncateOpenAIWSLogValue(lastEventType, openAIWSLogValueMaxLen),
		firstTokenMsValue,
		wroteDownstream,
		clientDisconnected,
		httpIngressWSOneShot,
	)

	imageCount := imageCounter.Count()
	imageBillingModel := ""
	imageSizeTier := ""
	if imageCount > 0 {
		if resolvedBillingModel, resolvedSizeTier, resolveErr := resolveOpenAIResponsesImageBillingConfig(reqBody, originalModel); resolveErr == nil {
			imageBillingModel = resolvedBillingModel
			imageSizeTier = resolvedSizeTier
		}
	}

	return &OpenAIForwardResult{
		RequestID:            responseID,
		Usage:                *usage,
		Model:                originalModel,
		UpstreamModel:        mappedModel,
		ImageCount:           imageCount,
		ImageSize:            imageSizeTier,
		BillingModel:         imageBillingModel,
		ServiceTier:          extractOpenAIServiceTier(reqBody),
		ReasoningEffort:      extractOpenAIReasoningEffort(reqBody, originalModel),
		Stream:               reqStream,
		OpenAIWSMode:         true,
		OpenAIWSProfile:      openAIWSProfileUsageString(connProfile),
		OpenAIWSConnReused:   lease.Reused(),
		OpenAIWSStoreMode:    diagnosticCompleted.StoreMode,
		OpenAIWSDeltaActive:  diagnosticCompleted.DeltaActive,
		OpenAIWSPayloadBytes: diagnosticCompleted.PayloadBytes,
		OpenAIWSDeltaItems:   diagnosticCompleted.DeltaItems,
		OpenAIWSDeltaBytes:   diagnosticCompleted.DeltaBytes,
		OpenAIWSFullItems:    diagnosticCompleted.FullItems,
		OpenAIWSFullBytes:    diagnosticCompleted.FullBytes,
		OpenAIWSConnPickMs:   lease.ConnPickDuration().Milliseconds(),
		OpenAIWSQueueWaitMs:  lease.QueueWaitDuration().Milliseconds(),
		OpenAIWSOneShot:      diagnosticCompleted.HTTPIngressWSOneShot,
		ClientDisconnected:   clientDisconnected,
		ResponseHeaders:      lease.HandshakeHeaders(),
		Duration:             time.Since(startTime),
		FirstTokenMs:         firstTokenMs,

		DeltaShadowOutputHashes:   deltaShadowRawOutputHashes,
		DeltaShadowOutputShapes:   deltaShadowRawOutputShapes,
		DeltaShadowOutputCaptured: deltaShadowOutputCaptured,
		DeltaShadowRawClientEquiv: deltaShadowRawClientEquiv,
	}, nil
}

// ProxyResponsesWebSocketFromClient 处理客户端入站 WebSocket（OpenAI Responses WS Mode）并转发到上游。
// 当前实现按“单请求 -> 终止事件 -> 下一请求”的顺序代理，适配 Codex CLI 的 turn 模式。
// stripCodexSparkImageGenerationToolFromRawPayload removes the image_generation
// tool from a raw /responses payload when the upstream model is gpt-5.3-codex-spark.
// Spark rejects that tool upstream with HTTP 400 (invalid_request_error, param=tools);
// Codex clients advertise it by default. Returns the (possibly unchanged) payload,
// whether it changed, and any JSON decode error.
func stripCodexSparkImageGenerationToolFromRawPayload(payload []byte, model string) ([]byte, bool, error) {
	if !isCodexSparkModel(model) || !openAIRequestBodyHasImageGenerationTool(payload) {
		return payload, false, nil
	}
	payloadMap := make(map[string]any)
	if err := json.Unmarshal(payload, &payloadMap); err != nil {
		return payload, false, err
	}
	if !stripCodexSparkImageGenerationTools(payloadMap) {
		return payload, false, nil
	}
	rebuilt, err := json.Marshal(payloadMap)
	if err != nil {
		return payload, false, err
	}
	return rebuilt, true, nil
}

func (s *OpenAIGatewayService) ProxyResponsesWebSocketFromClient(
	ctx context.Context,
	c *gin.Context,
	clientConn *coderws.Conn,
	account *Account,
	token string,
	firstClientMessage []byte,
	hooks *OpenAIWSIngressHooks,
) error {
	if s == nil {
		return errors.New("service is nil")
	}
	if c == nil {
		return errors.New("gin context is nil")
	}
	if clientConn == nil {
		return errors.New("client websocket is nil")
	}
	if account == nil {
		return errors.New("account is nil")
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("token is empty")
	}
	ingressRequestID, ingressClientRequestID := openAIWSRequestLogIDs(c)

	// 预取一次 OpenAI Fast Policy settings，绑定到 ctx，让该 WS session
	// 内所有帧的 evaluateOpenAIFastPolicy 调用复用同一份快照，避免每帧
	// 进入 DB / settingRepo。Trade-off 见 withOpenAIFastPolicyContext 注释。
	if s.settingService != nil {
		if settings, err := s.settingService.GetOpenAIFastPolicySettings(ctx); err == nil && settings != nil {
			ctx = withOpenAIFastPolicyContext(ctx, settings)
		}
	}

	wsDecision := s.getOpenAIWSProtocolResolver().Resolve(account)
	modeRouterV2Enabled := s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.ModeRouterV2Enabled
	ingressMode := OpenAIWSIngressModeCtxPool
	if modeRouterV2Enabled {
		ingressMode = account.ResolveOpenAIResponsesWebSocketV2Mode(s.cfg.Gateway.OpenAIWS.IngressModeDefault)
		if ingressMode == OpenAIWSIngressModeOff {
			return NewOpenAIWSClientCloseError(
				coderws.StatusPolicyViolation,
				"websocket mode is disabled for this account",
				nil,
			)
		}
		switch ingressMode {
		case OpenAIWSIngressModePassthrough:
			if wsDecision.Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
				return fmt.Errorf("websocket ingress requires ws_v2 transport, got=%s", wsDecision.Transport)
			}
			return s.proxyResponsesWebSocketV2Passthrough(
				ctx,
				c,
				clientConn,
				account,
				token,
				firstClientMessage,
				hooks,
				wsDecision,
			)
		case OpenAIWSIngressModeCtxPool, OpenAIWSIngressModeShared, OpenAIWSIngressModeDedicated:
			// continue
		default:
			return NewOpenAIWSClientCloseError(
				coderws.StatusPolicyViolation,
				"websocket mode only supports ctx_pool/passthrough",
				nil,
			)
		}
	}
	if wsDecision.Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
		return fmt.Errorf("websocket ingress requires ws_v2 transport, got=%s", wsDecision.Transport)
	}
	dedicatedMode := modeRouterV2Enabled && ingressMode == OpenAIWSIngressModeDedicated

	wsURL, err := s.buildOpenAIResponsesWSURL(account)
	if err != nil {
		return fmt.Errorf("build ws url: %w", err)
	}
	wsHost := "-"
	wsPath := "-"
	if parsedURL, parseErr := url.Parse(wsURL); parseErr == nil && parsedURL != nil {
		wsHost = normalizeOpenAIWSLogValue(parsedURL.Host)
		wsPath = normalizeOpenAIWSLogValue(parsedURL.Path)
	}
	debugEnabled := isOpenAIWSModeDebugEnabled()
	isCodexCLI := openai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator")) || (s.cfg != nil && s.cfg.Gateway.ForceCodexCLI)
	allowImageGeneration := GroupAllowsImageGeneration(apiKeyGroup(getAPIKeyFromContext(c)))
	stateStore := s.getOpenAIWSStateStore()
	groupID := getOpenAIGroupIDFromContext(c)
	apiKeyID := getAPIKeyIDFromContext(c)

	type openAIWSClientPayload struct {
		payloadRaw         []byte
		rawForHash         []byte
		promptCacheKey     string
		previousResponseID string
		preferredConnID    string
		originalModel      string
		imageBillingModel  string
		imageSizeTier      string
		imageInputSize     string
		payloadBytes       int
	}
	ingressSessionOriginalModel := ""

	applyPayloadMutation := func(current []byte, path string, value any) ([]byte, error) {
		next, err := sjson.SetBytes(current, path, value)
		if err == nil {
			return next, nil
		}

		// 仅在确实需要修改 payload 且 sjson 失败时，退回 map 路径确保兼容性。
		payload := make(map[string]any)
		if unmarshalErr := json.Unmarshal(current, &payload); unmarshalErr != nil {
			return nil, err
		}
		switch path {
		case "type", "model":
			payload[path] = value
		case "client_metadata." + openAIWSTurnMetadataHeader:
			setOpenAIWSTurnMetadata(payload, fmt.Sprintf("%v", value))
		default:
			return nil, err
		}
		rebuilt, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return nil, marshalErr
		}
		return rebuilt, nil
	}

	parseClientPayload := func(raw []byte) (openAIWSClientPayload, error) {
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 {
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "empty websocket request payload", nil)
		}
		if !gjson.ValidBytes(trimmed) {
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", errors.New("invalid json"))
		}

		values := gjson.GetManyBytes(trimmed, "type", "model", "prompt_cache_key", "previous_response_id")
		eventType := strings.TrimSpace(values[0].String())
		normalized := trimmed
		switch eventType {
		case "":
			eventType = "response.create"
			next, setErr := applyPayloadMutation(normalized, "type", eventType)
			if setErr != nil {
				return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", setErr)
			}
			normalized = next
		case "response.create":
		case "response.append":
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(
				coderws.StatusPolicyViolation,
				"response.append is not supported in ws v2; use response.create with previous_response_id",
				nil,
			)
		default:
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(
				coderws.StatusPolicyViolation,
				fmt.Sprintf("unsupported websocket request type: %s", eventType),
				nil,
			)
		}

		originalModel := strings.TrimSpace(values[1].String())
		modelMissing := originalModel == ""
		if originalModel == "" {
			// 入站 WS 长会话里，部分客户端只在第一轮 response.create 上声明
			// model，后续 turn 复用同一 session-level model。为避免因省略
			// model 直接断开用户连接，这里回落到上一轮已通过校验的客户端模型，
			// 并在下方写回上游 payload，保证账号模型映射/fast policy/图片权限
			// 仍按同一模型执行。
			originalModel = ingressSessionOriginalModel
			if originalModel == "" {
				return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(
					coderws.StatusPolicyViolation,
					"model is required in response.create payload",
					nil,
				)
			}
		}
		promptCacheKey := strings.TrimSpace(values[2].String())
		previousResponseID := strings.TrimSpace(values[3].String())
		previousResponseIDKind := ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
		if previousResponseID != "" && previousResponseIDKind == OpenAIPreviousResponseIDKindMessageID {
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(
				coderws.StatusPolicyViolation,
				"previous_response_id must be a response.id (resp_*), not a message id",
				nil,
			)
		}
		if turnMetadata := strings.TrimSpace(c.GetHeader(openAIWSTurnMetadataHeader)); turnMetadata != "" {
			next, setErr := applyPayloadMutation(normalized, "client_metadata."+openAIWSTurnMetadataHeader, turnMetadata)
			if setErr != nil {
				return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", setErr)
			}
			normalized = next
		}
		apiKey := getAPIKeyFromContext(c)
		imageGenerationAllowed := GroupAllowsImageGeneration(apiKeyGroup(apiKey))
		codexBridgeEnabled := isCodexCLI && imageGenerationAllowed && s.isCodexImageGenerationBridgeEnabled(ctx, account, apiKey)
		if codexBridgeEnabled {
			payloadMap := make(map[string]any)
			if err := json.Unmarshal(normalized, &payloadMap); err != nil {
				return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", err)
			}
			bridgeModified := false
			if ensureOpenAIResponsesImageGenerationTool(payloadMap) {
				bridgeModified = true
				logOpenAIWSModeInfo("ingress_ws_codex_image_tool_injected account_id=%d", account.ID)
			}
			if normalizeOpenAIResponsesImageGenerationTools(payloadMap) {
				bridgeModified = true
			}
			if applyCodexImageGenerationBridgeInstructions(payloadMap) {
				bridgeModified = true
				logOpenAIWSModeInfo("ingress_ws_codex_image_bridge_instructions_added account_id=%d", account.ID)
			}
			if bridgeModified {
				rebuilt, marshalErr := json.Marshal(payloadMap)
				if marshalErr != nil {
					return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", marshalErr)
				}
				normalized = rebuilt
			}
		}
		upstreamModel := normalizeOpenAIModelForUpstream(account, ResolveEffectiveMappedModel(ctx, s.settingService, account, originalModel, false))
		if modelMissing || upstreamModel != originalModel {
			next, setErr := applyPayloadMutation(normalized, "model", upstreamModel)
			if setErr != nil {
				return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", setErr)
			}
			normalized = next
		}
		var normalizedReqBody map[string]any
		if err := json.Unmarshal(normalized, &normalizedReqBody); err == nil {
			if normalizeOpenAIResponsesInputToolRolesWithOptions(normalizedReqBody, false) {
				rebuilt, marshalErr := marshalOpenAIResponsesRequestBodyOrdered(normalizedReqBody)
				if marshalErr != nil {
					return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", marshalErr)
				}
				normalized = rebuilt
			}
		}
		imageToolCapability := hasOpenAIImageGenerationTool(normalizedReqBody)
		if stripped, changed, stripErr := stripCodexSparkImageGenerationToolFromRawPayload(normalized, upstreamModel); stripErr != nil {
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", stripErr)
		} else if changed {
			normalized = stripped
			logOpenAIWSModeInfo("ingress_ws_codex_spark_image_tool_stripped account_id=%d", account.ID)
		}
		imageIntent := IsImageGenerationIntent(openAIResponsesEndpoint, originalModel, normalized)
		if imageIntent && !allowImageGeneration {
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, ImageGenerationPermissionMessage(), nil)
		}
		imageToolStripped := false
		if !imageIntent && imageToolCapability &&
			(!allowImageGeneration || accountShouldStripDeclaredImageGenerationTool(account, imageIntent)) &&
			stripOpenAIImageGenerationTools(normalizedReqBody) {
			imageToolStripped = true
			rebuilt, marshalErr := marshalOpenAIResponsesRequestBodyOrdered(normalizedReqBody)
			if marshalErr != nil {
				return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", marshalErr)
			}
			normalized = rebuilt
			stripReason := "group_image_generation_disabled"
			if allowImageGeneration {
				stripReason = "account_image_generation_disabled"
			}
			logOpenAIWSModeInfo(
				"ingress_ws_strip_image_generation_tool account_id=%d model=%s reason=%s",
				account.ID,
				normalizeOpenAIWSLogValue(originalModel),
				stripReason,
			)
		}
		imageBillingModel := ""
		imageSizeTier := ""
		imageInputSize := ""
		if imageIntent || (allowImageGeneration && imageToolCapability && !imageToolStripped) {
			var imageCfgErr error
			imageCfg, imageCfgErr := resolveOpenAIResponsesImageBillingConfigDetailedFromBody(normalized, originalModel)
			if imageCfgErr != nil {
				return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, imageCfgErr.Error(), imageCfgErr)
			}
			imageBillingModel = imageCfg.Model
			imageSizeTier = imageCfg.SizeTier
			imageInputSize = imageCfg.InputSize
		}

		// Apply OpenAI Fast Policy on the response.create frame using the same
		// evaluator/normalize/scope rules as the HTTP entrypoints. This is the
		// single integration point for all WS ingress turns (first + follow-up
		// frames flow through here).
		//
		// Model fallback: first turn still requires model at the handler layer；
		// follow-up response.create frames may omit it and then reuse
		// ingressSessionOriginalModel. We always write a concrete upstream model
		// before evaluating policy, so whitelist / filter behavior remains stable.
		policyApplied, blocked, policyErr := s.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, upstreamModel, normalized)
		if policyErr != nil {
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", policyErr)
		}
		if blocked != nil {
			MarkOpsClientBusinessLimited(c, OpsClientBusinessLimitedReasonLocalPolicyDenied)
			// Send a Realtime-style error event to the client first, then
			// signal the handler to close the connection with PolicyViolation.
			// We intentionally do NOT forward this frame upstream.
			//
			// coder/websocket@v1.8.14 Conn.Write is synchronous and flushes
			// the underlying bufio writer before returning (write.go:42 →
			// 307-311), and the subsequent close handshake re-acquires the
			// same writeFrameMu, so the error event is guaranteed to reach
			// the kernel send buffer before any close frame is queued.
			eventBytes := buildOpenAIFastPolicyBlockedWSEvent(blocked)
			if eventBytes != nil {
				writeCtx, cancel := context.WithTimeout(ctx, s.openAIWSWriteTimeout())
				_ = clientConn.Write(writeCtx, coderws.MessageText, eventBytes)
				cancel()
			}
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(
				coderws.StatusPolicyViolation,
				blocked.Message,
				blocked,
			)
		}
		normalized = policyApplied
		ingressSessionOriginalModel = originalModel

		storeDecisionPayload, storeDecision, storeDecisionErr := s.resolveOpenAIWSContinuationStoreDecisionRaw(
			ctx,
			normalized,
			account,
			stateStore,
			groupID,
			apiKeyID,
		)
		if storeDecisionErr != nil {
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", storeDecisionErr)
		}
		if err := storeDecision.unsafeToolContinuationError(); err != nil {
			return openAIWSClientPayload{}, NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "unsafe websocket tool continuation", err)
		}
		normalized = storeDecisionPayload
		previousResponseID = openAIWSPayloadStringFromRaw(normalized, "previous_response_id")

		return openAIWSClientPayload{
			payloadRaw:         normalized,
			rawForHash:         trimmed,
			promptCacheKey:     promptCacheKey,
			previousResponseID: previousResponseID,
			preferredConnID:    storeDecision.PreferredConnID,
			originalModel:      originalModel,
			imageBillingModel:  imageBillingModel,
			imageSizeTier:      imageSizeTier,
			imageInputSize:     imageInputSize,
			payloadBytes:       len(normalized),
		}, nil
	}

	writeClientMessage := func(message []byte) error {
		writeCtx, cancel := context.WithTimeout(ctx, s.openAIWSWriteTimeout())
		defer cancel()
		return clientConn.Write(writeCtx, coderws.MessageText, message)
	}

	readClientMessage := func() ([]byte, error) {
		msgType, payload, readErr := clientConn.Read(ctx)
		if readErr != nil {
			return nil, readErr
		}
		if msgType != coderws.MessageText && msgType != coderws.MessageBinary {
			return nil, NewOpenAIWSClientCloseError(
				coderws.StatusPolicyViolation,
				fmt.Sprintf("unsupported websocket client message type: %s", msgType.String()),
				nil,
			)
		}
		return payload, nil
	}

	firstPayload, err := parseClientPayload(firstClientMessage)
	if err != nil {
		return err
	}

	turnState := strings.TrimSpace(c.GetHeader(openAIWSTurnStateHeader))
	storeDisabledConnMode := s.openAIWSStoreDisabledConnMode()
	sessionHash := ""
	preferredConnID := ""
	storeDisabled := false
	refreshIngressRouteState := func(payload openAIWSClientPayload) {
		sessionHash = ""
		if hooks != nil {
			sessionHash = strings.TrimSpace(hooks.SessionHash)
		}
		if sessionHash == "" {
			sessionHash = s.GenerateSessionHash(c, payload.rawForHash)
		}
		if turnState == "" && stateStore != nil && sessionHash != "" {
			if savedTurnState, ok := stateStore.GetSessionTurnState(groupID, sessionHash); ok {
				turnState = savedTurnState
			}
		}

		preferredConnID = payload.preferredConnID
		if stateStore != nil && payload.previousResponseID != "" {
			if connID, ok := stateStore.GetResponseConn(groupID, apiKeyID, payload.previousResponseID); ok {
				preferredConnID = connID
			}
		}

		storeDisabled = s.isOpenAIWSStoreDisabledInRequestRaw(payload.payloadRaw, account)
		if stateStore != nil && storeDisabled && payload.previousResponseID == "" && sessionHash != "" {
			if connID, ok := stateStore.GetSessionConn(groupID, apiKeyID, account.ID, sessionHash); ok {
				preferredConnID = connID
			}
		}
	}
	refreshIngressRouteState(firstPayload)

	if s.shouldBridgeOpenAIWSHTTP(firstPayload.payloadBytes, firstPayload.previousResponseID) {
		logOpenAIWSModeInfo(
			"ingress_ws_http_bridge_start account_id=%d account_type=%s payload_bytes=%d threshold_bytes=%d has_session_hash=%v store_disabled=%v",
			account.ID,
			account.Type,
			firstPayload.payloadBytes,
			s.openAIWSHTTPBridgeThresholdBytes(),
			sessionHash != "",
			storeDisabled,
		)
		currentBridgePayload := firstPayload
		var bridgeReplayInput []json.RawMessage
		bridgeReplayInputExists := false
		bridgeLastResponseID := ""
		bridgeLastPayload := []byte(nil)
		var bridgeLastStrictState *openAIWSIngressPreviousTurnStrictState
		for turn := 1; ; turn++ {
			if turn > 1 && hooks != nil && hooks.BeforeRequest != nil {
				if err := hooks.BeforeRequest(turn, currentBridgePayload.payloadRaw, currentBridgePayload.originalModel); err != nil {
					return err
				}
			}
			if hooks != nil && hooks.BeforeTurn != nil {
				if err := hooks.BeforeTurn(turn); err != nil {
					return err
				}
			}
			if turnState != "" && c != nil && c.Request != nil {
				c.Request.Header.Set(openAIWSTurnStateHeader, turnState)
			}
			bridgePayloadRaw := currentBridgePayload.payloadRaw
			bridgePayloadBytes := currentBridgePayload.payloadBytes
			needsBridgeReplay := currentBridgePayload.previousResponseID != "" || openAIWSRawPayloadHasToolCallOutput(currentBridgePayload.payloadRaw)
			turnReplayInput, turnReplayInputExists, replayInputErr := buildOpenAIWSReplayInputSequence(
				bridgeReplayInput,
				bridgeReplayInputExists,
				currentBridgePayload.payloadRaw,
				needsBridgeReplay,
			)
			if replayInputErr != nil {
				return fmt.Errorf("build websocket http bridge replay input: %w", replayInputErr)
			}
			if needsBridgeReplay && turnReplayInputExists {
				updatedPayload, setInputErr := setOpenAIWSPayloadInputSequence(
					currentBridgePayload.payloadRaw,
					turnReplayInput,
					true,
				)
				if setInputErr != nil {
					return fmt.Errorf("set websocket http bridge replay input: %w", setInputErr)
				}
				bridgePayloadRaw = updatedPayload
				bridgePayloadBytes = len(updatedPayload)
				logOpenAIWSModeInfo(
					"ingress_ws_http_bridge_replay_input account_id=%d turn=%d input_items=%d previous_response_id_present=%v has_tool_output=%v",
					account.ID,
					turn,
					len(turnReplayInput),
					currentBridgePayload.previousResponseID != "",
					openAIWSRawPayloadHasToolCallOutput(currentBridgePayload.payloadRaw),
				)
			}
			result, bridgeErr := s.proxyOpenAIWSHTTPBridgeTurn(
				ctx,
				c,
				account,
				token,
				bridgePayloadRaw,
				bridgePayloadBytes,
				currentBridgePayload.originalModel,
				currentBridgePayload.imageBillingModel,
				currentBridgePayload.imageSizeTier,
				currentBridgePayload.imageInputSize,
				turn,
				writeClientMessage,
			)
			if hooks != nil && hooks.AfterTurn != nil {
				hooks.AfterTurn(turn, cloneOpenAIWSPayloadBytes(bridgePayloadRaw), result, bridgeErr)
			}
			if bridgeErr != nil {
				return bridgeErr
			}
			if result == nil {
				return errors.New("websocket http bridge turn result is nil")
			}
			bridgeReplayInput = cloneOpenAIWSRawMessages(turnReplayInput)
			bridgeReplayInputExists = turnReplayInputExists
			if result.wsReplayInputExists {
				bridgeReplayInput = append(bridgeReplayInput, cloneOpenAIWSRawMessages(result.wsReplayInput)...)
				bridgeReplayInputExists = true
			}
			if bridgeTurnState := strings.TrimSpace(result.ResponseHeaders.Get(openAIWSTurnStateHeader)); bridgeTurnState != "" {
				turnState = bridgeTurnState
				if stateStore != nil && sessionHash != "" {
					stateStore.BindSessionTurnState(groupID, sessionHash, bridgeTurnState, s.openAIWSSessionStickyTTL())
				}
			}
			responseID := strings.TrimSpace(result.RequestID)
			bridgeLastResponseID = responseID
			bridgeLastPayload = cloneOpenAIWSPayloadBytes(currentBridgePayload.payloadRaw)
			nextStrictState, strictStateErr := buildOpenAIWSIngressPreviousTurnStrictState(currentBridgePayload.payloadRaw)
			if strictStateErr != nil {
				bridgeLastStrictState = nil
				logOpenAIWSModeInfo(
					"ingress_ws_http_bridge_strict_state_skip account_id=%d turn=%d reason=build_error cause=%s",
					account.ID,
					turn,
					truncateOpenAIWSLogValue(strictStateErr.Error(), openAIWSLogValueMaxLen),
				)
			} else {
				bridgeLastStrictState = nextStrictState
			}
			if responseID != "" && stateStore != nil {
				ttl := s.openAIWSResponseStickyTTL()
				logOpenAIWSBindResponseAccountWarn(groupID, account.ID, responseID, stateStore.BindResponseAccount(ctx, groupID, apiKeyID, responseID, account.ID, ttl))
			}
			nextClientMessage, readErr := readClientMessage()
			if readErr != nil {
				if isOpenAIWSClientDisconnectError(readErr) {
					closeStatus, closeReason := summarizeOpenAIWSReadCloseError(readErr)
					logOpenAIWSModeInfo(
						"ingress_ws_http_bridge_client_closed account_id=%d close_status=%s close_reason=%s",
						account.ID,
						closeStatus,
						truncateOpenAIWSLogValue(closeReason, openAIWSHeaderValueMaxLen),
					)
					return nil
				}
				return fmt.Errorf("read client websocket request: %w", readErr)
			}
			nextPayload, parseErr := parseClientPayload(nextClientMessage)
			if parseErr != nil {
				return parseErr
			}
			nextStoreDisabled := s.isOpenAIWSStoreDisabledInRequestRaw(nextPayload.payloadRaw, account)
			shouldBranchTurn, branchReason, branchErr := shouldBranchOpenAIWSIngressFollowupTurn(
				turn+1,
				nextPayload.rawForHash,
				nextPayload.payloadRaw,
				nextStoreDisabled,
				bridgeLastPayload,
				bridgeLastStrictState,
				bridgeLastResponseID,
			)
			if shouldBranchTurn {
				if stateStore != nil && sessionHash != "" {
					stateStore.DeleteSessionTurnState(groupID, sessionHash)
					stateStore.DeleteSessionConn(groupID, apiKeyID, account.ID, sessionHash)
					stateStore.DeleteSessionContext(groupID, apiKeyID, sessionHash)
				}
				updatedPayload, removedPrev, dropErr := dropPreviousResponseIDFromRawPayload(nextPayload.payloadRaw)
				if dropErr != nil {
					return fmt.Errorf("drop previous_response_id for ingress http bridge branch replay: %w", dropErr)
				}
				if removedPrev {
					nextPayload.payloadRaw = updatedPayload
					nextPayload.payloadBytes = len(updatedPayload)
					nextPayload.previousResponseID = ""
				}
				logMsg := "ingress_ws_http_bridge_branch_replay account_id=%d turn=%d next_turn=%d reason=%s removed_previous_response_id=%v"
				logArgs := []any{
					account.ID,
					turn,
					turn + 1,
					normalizeOpenAIWSLogValue(branchReason),
					removedPrev,
				}
				if branchErr != nil {
					logMsg += " cause=%s"
					logArgs = append(logArgs, truncateOpenAIWSLogValue(branchErr.Error(), openAIWSLogValueMaxLen))
				}
				logOpenAIWSModeInfo(logMsg, logArgs...)
				turnState = ""
				bridgeReplayInput = nil
				bridgeReplayInputExists = false
				bridgeLastResponseID = ""
				bridgeLastPayload = nil
				bridgeLastStrictState = nil
			}
			currentBridgePayload = nextPayload
		}
	}

	isCodexCLI = openai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator")) || (s.cfg != nil && s.cfg.Gateway.ForceCodexCLI)
	tlsFPRuntime := s.resolveOpenAITLSFingerprintRuntime(ctx, c, account, "websocket")
	wsHeaders, _ := s.buildOpenAIWSHeaders(c, account, token, wsDecision, isCodexCLI, turnState, strings.TrimSpace(c.GetHeader(openAIWSTurnMetadataHeader)), firstPayload.promptCacheKey)
	applyOpenAIWSFingerprintRuntimeHeaders(wsHeaders, tlsFPRuntime)
	baseAcquireReq := openAIWSAcquireRequest{
		Account:    account,
		WSURL:      wsURL,
		Headers:    wsHeaders,
		TLSProfile: tlsFPRuntime.Profile,
		ProxyURL: func() string {
			if account.ProxyID != nil && account.Proxy != nil {
				return account.Proxy.URL()
			}
			return ""
		}(),
		ForceNewConn: false,
	}
	pool := s.getOpenAIWSConnPool()
	if pool == nil {
		return errors.New("openai ws conn pool is nil")
	}

	logOpenAIWSModeInfo(
		"ingress_ws_protocol_confirm account_id=%d account_type=%s transport=%s ws_host=%s ws_path=%s ws_mode=%s store_disabled=%v has_session_hash=%v has_previous_response_id=%v",
		account.ID,
		account.Type,
		normalizeOpenAIWSLogValue(string(wsDecision.Transport)),
		wsHost,
		wsPath,
		normalizeOpenAIWSLogValue(ingressMode),
		storeDisabled,
		sessionHash != "",
		firstPayload.previousResponseID != "",
	)

	if debugEnabled {
		logOpenAIWSModeDebug(
			"ingress_ws_start account_id=%d account_type=%s transport=%s ws_host=%s preferred_conn_id=%s has_session_hash=%v has_previous_response_id=%v store_disabled=%v",
			account.ID,
			account.Type,
			normalizeOpenAIWSLogValue(string(wsDecision.Transport)),
			wsHost,
			truncateOpenAIWSLogValue(preferredConnID, openAIWSIDValueMaxLen),
			sessionHash != "",
			firstPayload.previousResponseID != "",
			storeDisabled,
		)
	}
	if firstPayload.previousResponseID != "" {
		firstPreviousResponseIDKind := ClassifyOpenAIPreviousResponseIDKind(firstPayload.previousResponseID)
		logOpenAIWSModeInfo(
			"ingress_ws_continuation_probe account_id=%d turn=%d previous_response_id=%s previous_response_id_kind=%s preferred_conn_id=%s session_hash=%s header_session_id=%s header_conversation_id=%s has_turn_state=%v turn_state_len=%d has_prompt_cache_key=%v store_disabled=%v",
			account.ID,
			1,
			truncateOpenAIWSLogValue(firstPayload.previousResponseID, openAIWSIDValueMaxLen),
			normalizeOpenAIWSLogValue(firstPreviousResponseIDKind),
			truncateOpenAIWSLogValue(preferredConnID, openAIWSIDValueMaxLen),
			truncateOpenAIWSLogValue(sessionHash, 12),
			openAIWSHeaderValueForLog(baseAcquireReq.Headers, "session_id"),
			openAIWSHeaderValueForLog(baseAcquireReq.Headers, "conversation_id"),
			turnState != "",
			len(turnState),
			firstPayload.promptCacheKey != "",
			storeDisabled,
		)
	}

	acquireTimeout := s.openAIWSAcquireTimeout()
	if acquireTimeout <= 0 {
		acquireTimeout = 30 * time.Second
	}

	acquireTurnLease := func(turn int, preferred string, forcePreferredConn bool) (*openAIWSConnLease, error) {
		req := cloneOpenAIWSAcquireRequest(baseAcquireReq)
		req.PreferredConnID = strings.TrimSpace(preferred)
		forcePreferredWithConn := forcePreferredConn && req.PreferredConnID != ""
		req.ForcePreferredConn = forcePreferredWithConn
		// dedicated 模式下每次获取均新建连接，避免跨会话复用残留上下文。
		req.ForceNewConn = dedicatedMode ||
			(forcePreferredConn && req.PreferredConnID == "") ||
			(shouldForceNewConnOnStoreDisabled(storeDisabledConnMode, "") &&
				storeDisabled &&
				sessionHash != "" &&
				req.PreferredConnID == "" &&
				!forcePreferredWithConn)
		acquireCtx, acquireCancel := context.WithTimeout(ctx, acquireTimeout)
		lease, acquireErr := pool.Acquire(acquireCtx, req)
		acquireCancel()
		if acquireErr != nil {
			dialStatus, dialClass, dialCloseStatus, dialCloseReason, dialRespServer, dialRespVia, dialRespCFRay, dialRespReqID := summarizeOpenAIWSDialError(acquireErr)
			logOpenAIWSModeInfo(
				"ingress_ws_upstream_acquire_fail account_id=%d turn=%d reason=%s dial_status=%d dial_class=%s dial_close_status=%s dial_close_reason=%s dial_resp_server=%s dial_resp_via=%s dial_resp_cf_ray=%s dial_resp_x_request_id=%s cause=%s preferred_conn_id=%s force_preferred_conn=%v ws_host=%s ws_path=%s proxy_enabled=%v",
				account.ID,
				turn,
				normalizeOpenAIWSLogValue(classifyOpenAIWSAcquireError(acquireErr)),
				dialStatus,
				dialClass,
				dialCloseStatus,
				truncateOpenAIWSLogValue(dialCloseReason, openAIWSHeaderValueMaxLen),
				dialRespServer,
				dialRespVia,
				dialRespCFRay,
				dialRespReqID,
				truncateOpenAIWSLogValue(acquireErr.Error(), openAIWSLogValueMaxLen),
				truncateOpenAIWSLogValue(preferred, openAIWSIDValueMaxLen),
				forcePreferredConn,
				wsHost,
				wsPath,
				account.ProxyID != nil && account.Proxy != nil,
			)
			s.persistOpenAIWSDialFailureSignal(ctx, account, acquireErr)
			var dialErr *openAIWSDialError
			if errors.As(acquireErr, &dialErr) && dialErr != nil && dialErr.StatusCode == http.StatusTooManyRequests {
				return nil, &UpstreamFailoverError{
					StatusCode:      http.StatusTooManyRequests,
					ResponseHeaders: cloneHeader(dialErr.ResponseHeaders),
				}
			}
			if errors.Is(acquireErr, errOpenAIWSPreferredConnUnavailable) {
				return nil, NewOpenAIWSClientCloseError(
					coderws.StatusPolicyViolation,
					"upstream continuation connection is unavailable; please restart the conversation",
					acquireErr,
				)
			}
			if errors.Is(acquireErr, context.DeadlineExceeded) || errors.Is(acquireErr, errOpenAIWSConnQueueFull) {
				return nil, NewOpenAIWSClientCloseError(
					coderws.StatusTryAgainLater,
					"upstream websocket is busy, please retry later",
					acquireErr,
				)
			}
			return nil, acquireErr
		}
		connID := strings.TrimSpace(lease.ConnID())
		if handshakeTurnState := strings.TrimSpace(lease.HandshakeHeader(openAIWSTurnStateHeader)); handshakeTurnState != "" {
			turnState = handshakeTurnState
			if stateStore != nil && sessionHash != "" {
				stateStore.BindSessionTurnState(groupID, sessionHash, handshakeTurnState, s.openAIWSSessionStickyTTL())
			}
			updatedHeaders := cloneHeader(baseAcquireReq.Headers)
			if updatedHeaders == nil {
				updatedHeaders = make(http.Header)
			}
			updatedHeaders.Set(openAIWSTurnStateHeader, handshakeTurnState)
			baseAcquireReq.Headers = updatedHeaders
		}
		logOpenAIWSModeInfo(
			"ingress_ws_upstream_connected account_id=%d turn=%d conn_id=%s conn_reused=%v conn_pick_ms=%d queue_wait_ms=%d preferred_conn_id=%s",
			account.ID,
			turn,
			truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
			lease.Reused(),
			lease.ConnPickDuration().Milliseconds(),
			lease.QueueWaitDuration().Milliseconds(),
			truncateOpenAIWSLogValue(preferred, openAIWSIDValueMaxLen),
		)
		return lease, nil
	}

	sendAndRelay := func(turn int, lease *openAIWSConnLease, payload []byte, payloadBytes int, originalModel string, imageBillingModel string, imageSizeTier string, imageInputSize string) (*OpenAIForwardResult, error) {
		if lease == nil {
			return nil, errors.New("upstream websocket lease is nil")
		}
		turnStart := time.Now()
		wroteDownstream := false
		if err := lease.WriteJSONWithContextTimeout(ctx, json.RawMessage(payload), s.openAIWSWriteTimeout()); err != nil {
			return nil, wrapOpenAIWSIngressTurnError(
				"write_upstream",
				fmt.Errorf("write upstream websocket request: %w", err),
				false,
			)
		}
		if debugEnabled {
			logOpenAIWSModeDebug(
				"ingress_ws_turn_request_sent account_id=%d turn=%d conn_id=%s payload_bytes=%d",
				account.ID,
				turn,
				truncateOpenAIWSLogValue(lease.ConnID(), openAIWSIDValueMaxLen),
				payloadBytes,
			)
		}

		responseID := ""
		usage := OpenAIUsage{}
		imageCounter := newOpenAIImageOutputCounter()
		var firstTokenMs *int
		reqStream := openAIWSPayloadBoolFromRaw(payload, "stream", true)
		turnPreviousResponseID := openAIWSPayloadStringFromRaw(payload, "previous_response_id")
		turnPreviousResponseIDKind := ClassifyOpenAIPreviousResponseIDKind(turnPreviousResponseID)
		turnPromptCacheKey := openAIWSPayloadStringFromRaw(payload, "prompt_cache_key")
		turnStoreDisabled := s.isOpenAIWSStoreDisabledInRequestRaw(payload, account)
		turnHasFunctionCallOutput := HasToolContinuationOutputInRawPayload(payload)
		eventCount := 0
		tokenEventCount := 0
		terminalEventCount := 0
		replayCollector := &openAIWSToolCallReplayCollector{}
		firstEventType := ""
		lastEventType := ""
		needModelReplace := false
		clientDisconnected := false
		mappedModel := ""
		var mappedModelBytes []byte
		if originalModel != "" {
			mappedModel = normalizeOpenAIModelForUpstream(account, ResolveEffectiveMappedModel(ctx, s.settingService, account, originalModel, false))
			needModelReplace = mappedModel != "" && mappedModel != originalModel
			if needModelReplace {
				mappedModelBytes = []byte(mappedModel)
			}
		}
		// TEMP_DIAG(openai_ws_delta_shadow) remove_after_debug=true：捕获 raw upstream output 的
		// canonical 哈希，并对比改写前后(raw vs client-visible)是否等价；仅度量，不改 payload。
		deltaShadowEnabled := openAIWSDeltaShadowEnabled()
		var deltaShadowRawOutputHashes [][32]byte
		var deltaShadowRawOutputShapes []string
		deltaShadowOutputCaptured := false
		deltaShadowRawClientEquiv := true
		deltaShadowRawDoneItems := map[int]json.RawMessage{}
		deltaShadowVisibleDoneItems := map[int]json.RawMessage{}
		for {
			upstreamMessage, readErr := lease.ReadMessageWithContextTimeout(ctx, s.openAIWSReadTimeout())
			if readErr != nil {
				lease.MarkBrokenFor("ingress_read_upstream")
				return nil, wrapOpenAIWSIngressTurnError(
					"read_upstream",
					fmt.Errorf("read upstream websocket event: %w", readErr),
					wroteDownstream,
				)
			}

			eventType, eventResponseID, responseField := parseOpenAIWSEventEnvelope(upstreamMessage)
			if responseID == "" && eventResponseID != "" {
				responseID = eventResponseID
			}
			// raw upstream output 在改写(:3928 model/tool 改写)之前捕获，仅终端 completed 事件携带 output 数组。
			if deltaShadowEnabled && !deltaShadowOutputCaptured && responseField.Exists() {
				if outItems, ok := openAIWSExtractResponseOutputItems([]byte(responseField.Raw)); ok {
					if hashes, hok := openAIWSCanonicalItemHashes(outItems); hok {
						if shapes, sok := openAIWSItemShapes(outItems); sok {
							deltaShadowRawOutputHashes = hashes
							deltaShadowRawOutputShapes = shapes
							deltaShadowOutputCaptured = true
						}
					}
				}
			}
			if deltaShadowEnabled {
				if outputIndex, item, ok := openAIWSExtractOutputItemDoneItem(upstreamMessage); ok {
					deltaShadowRawDoneItems[outputIndex] = item
				}
			}
			if eventType != "" {
				eventCount++
				if firstEventType == "" {
					firstEventType = eventType
				}
				lastEventType = eventType
			}
			if eventType == "error" {
				errCodeRaw, errTypeRaw, errMsgRaw := parseOpenAIWSErrorEventFields(upstreamMessage)
				_ = markOpsCyberPolicyIfDetected(c, upstreamMessage, http.StatusOK, usage.InputTokens, usage.OutputTokens)
				s.persistOpenAIWSRateLimitSignal(ctx, account, lease.HandshakeHeaders(), upstreamMessage, errCodeRaw, errTypeRaw, errMsgRaw)
				fallbackReason, _ := classifyOpenAIWSErrorEventFromRaw(errCodeRaw, errTypeRaw, errMsgRaw)
				errCode, errType, errMessage := summarizeOpenAIWSErrorEventFieldsFromRaw(errCodeRaw, errTypeRaw, errMsgRaw)
				snapshotDecision := openAIWSContinuationStoreDecision{
					StoreMode:                  openAIWSStoreModeFull,
					StoreDisabled:              turnStoreDisabled,
					OriginalPreviousResponseID: turnPreviousResponseID,
					PreferredConnID:            preferredConnID,
					FallbackReason:             fallbackReason,
				}
				if strings.TrimSpace(turnPreviousResponseID) != "" {
					snapshotDecision.StoreMode = openAIWSStoreModeIncremental
				}
				if strings.TrimSpace(preferredConnID) != "" {
					snapshotDecision.ConnAffinityHit = strings.TrimSpace(preferredConnID) == strings.TrimSpace(lease.ConnID())
				}
				if stateStore != nil && strings.TrimSpace(turnPreviousResponseID) != "" {
					if boundAccountID, getErr := stateStore.GetResponseAccount(ctx, groupID, apiKeyID, turnPreviousResponseID); getErr == nil && boundAccountID > 0 {
						snapshotDecision.StickyAccountID = boundAccountID
						snapshotDecision.StickyAccountHit = boundAccountID == account.ID
						snapshotDecision.StickyAccountMismatch = boundAccountID != account.ID
					}
				}
				s.logOpenAIWSBindingSnapshot(
					ctx,
					"ingress_error_event",
					ingressRequestID,
					account,
					stateStore,
					pool,
					groupID,
					apiKeyID,
					sessionHash,
					turnPreviousResponseID,
					strings.TrimSpace(turnPreviousResponseID) != "",
					lease.ConnID(),
					preferredConnID,
					snapshotDecision,
					fallbackReason,
					errCode,
					errType,
					errMessage,
				)
				s.logOpenAIWSMismatchErrorProbe(
					ctx,
					ingressRequestID,
					ingressClientRequestID,
					account,
					stateStore,
					pool,
					groupID,
					apiKeyID,
					sessionHash,
					turnPreviousResponseID,
					openAIWSPreviousResponseIDSource(turnPreviousResponseID, turnPreviousResponseID, false, false),
					strings.TrimSpace(turnPreviousResponseID) != "",
					responseID,
					openAIWSConnProfileSessionBound,
					lease.ConnID(),
					lease.Reused(),
					preferredConnID,
					snapshotDecision,
					snapshotDecision.StoreMode,
					false,
					0,
					0,
					fallbackReason,
					false,
					wroteDownstream,
					errCode,
					errType,
					errMessage,
				)
				recoverablePrevNotFound := fallbackReason == openAIWSIngressStagePreviousResponseNotFound &&
					turnPreviousResponseID != "" &&
					!turnHasFunctionCallOutput &&
					s.openAIWSIngressPreviousResponseRecoveryEnabled() &&
					!wroteDownstream
				if recoverablePrevNotFound {
					// 可恢复场景使用非 error 关键字日志，避免被 LegacyPrintf 误判为 ERROR 级别。
					logOpenAIWSModeInfo(
						"ingress_ws_prev_response_recoverable account_id=%d turn=%d conn_id=%s idx=%d reason=%s code=%s type=%s message=%s previous_response_id=%s previous_response_id_kind=%s response_id=%s store_disabled=%v has_prompt_cache_key=%v",
						account.ID,
						turn,
						truncateOpenAIWSLogValue(lease.ConnID(), openAIWSIDValueMaxLen),
						eventCount,
						truncateOpenAIWSLogValue(fallbackReason, openAIWSLogValueMaxLen),
						errCode,
						errType,
						errMessage,
						truncateOpenAIWSLogValue(turnPreviousResponseID, openAIWSIDValueMaxLen),
						normalizeOpenAIWSLogValue(turnPreviousResponseIDKind),
						truncateOpenAIWSLogValue(responseID, openAIWSIDValueMaxLen),
						turnStoreDisabled,
						turnPromptCacheKey != "",
					)
				} else {
					logOpenAIWSModeInfo(
						"ingress_ws_error_event account_id=%d turn=%d conn_id=%s idx=%d fallback_reason=%s err_code=%s err_type=%s err_message=%s previous_response_id=%s previous_response_id_kind=%s response_id=%s store_disabled=%v has_prompt_cache_key=%v",
						account.ID,
						turn,
						truncateOpenAIWSLogValue(lease.ConnID(), openAIWSIDValueMaxLen),
						eventCount,
						truncateOpenAIWSLogValue(fallbackReason, openAIWSLogValueMaxLen),
						errCode,
						errType,
						errMessage,
						truncateOpenAIWSLogValue(turnPreviousResponseID, openAIWSIDValueMaxLen),
						normalizeOpenAIWSLogValue(turnPreviousResponseIDKind),
						truncateOpenAIWSLogValue(responseID, openAIWSIDValueMaxLen),
						turnStoreDisabled,
						turnPromptCacheKey != "",
					)
				}
				// previous_response_not_found 在 ingress 模式支持单次恢复重试：
				// 不把该 error 直接下发客户端，而是由上层去掉 previous_response_id 后重放当前 turn。
				if recoverablePrevNotFound {
					lease.MarkBrokenFor("ingress_previous_response_not_found")
					errMsg := strings.TrimSpace(errMsgRaw)
					if errMsg == "" {
						errMsg = "previous response not found"
					}
					return nil, wrapOpenAIWSIngressTurnError(
						openAIWSIngressStagePreviousResponseNotFound,
						errors.New(errMsg),
						false,
					)
				}
				if !wroteDownstream && isOpenAIWSRateLimitError(errCodeRaw, errTypeRaw, errMsgRaw) {
					lease.MarkBrokenFor("ingress_rate_limit")
					return nil, &UpstreamFailoverError{
						StatusCode:      http.StatusTooManyRequests,
						ResponseBody:    append([]byte(nil), upstreamMessage...),
						ResponseHeaders: cloneHeader(lease.HandshakeHeaders()),
					}
				}
			}
			isTokenEvent := isOpenAIWSTokenEvent(eventType)
			if isTokenEvent {
				tokenEventCount++
			}
			isTerminalEvent := isOpenAIWSTerminalEvent(eventType)
			if isTerminalEvent {
				terminalEventCount++
			}
			if firstTokenMs == nil && isTokenEvent {
				ms := int(time.Since(turnStart).Milliseconds())
				firstTokenMs = &ms
			}
			if openAIWSEventShouldParseUsage(eventType) {
				parseOpenAIWSResponseUsageFromCompletedEvent(upstreamMessage, &usage)
			}
			imageCounter.AddSSEData(upstreamMessage)

			if eventType == "response.failed" {
				if hit, code, msg := detectOpenAICyberPolicy(upstreamMessage); hit {
					MarkOpsCyberPolicy(c, CyberPolicyMark{
						Code:           code,
						Message:        msg,
						Body:           truncateString(string(upstreamMessage), 4096),
						UpstreamStatus: http.StatusOK,
						UpstreamInTok:  usage.InputTokens,
						UpstreamOutTok: usage.OutputTokens,
					})
				}
			}

			if !clientDisconnected {
				if needModelReplace && len(mappedModelBytes) > 0 && openAIWSEventMayContainModel(eventType) && bytes.Contains(upstreamMessage, mappedModelBytes) {
					upstreamMessage = replaceOpenAIWSMessageModel(upstreamMessage, mappedModel, originalModel)
				}
				if openAIWSEventMayContainToolCalls(eventType) && openAIWSMessageLikelyContainsToolCalls(upstreamMessage) {
					if corrected, changed := s.toolCorrector.CorrectToolCallsInSSEBytes(upstreamMessage); changed {
						upstreamMessage = corrected
					}
				}
				replayCollector.AddEvent(eventType, upstreamMessage)
				if deltaShadowEnabled {
					if outputIndex, item, ok := openAIWSExtractOutputItemDoneItem(upstreamMessage); ok {
						deltaShadowVisibleDoneItems[outputIndex] = item
					}
				}
				if err := writeClientMessage(upstreamMessage); err != nil {
					if isOpenAIWSClientDisconnectError(err) {
						clientDisconnected = true
						closeStatus, closeReason := summarizeOpenAIWSReadCloseError(err)
						logOpenAIWSModeInfo(
							"ingress_ws_client_disconnected_drain account_id=%d turn=%d conn_id=%s close_status=%s close_reason=%s",
							account.ID,
							turn,
							truncateOpenAIWSLogValue(lease.ConnID(), openAIWSIDValueMaxLen),
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
			if isTerminalEvent {
				// 客户端已断连时，上游连接的 session 状态不可信，标记 broken 避免回池复用。
				if clientDisconnected {
					lease.MarkBrokenFor("ingress_client_disconnected")
				}
				// TEMP_DIAG(openai_ws_delta_shadow): 对比 client-visible(改写后)与 raw output 是否等价。
				if deltaShadowEnabled && deltaShadowOutputCaptured && !clientDisconnected {
					responseRaw := gjson.GetBytes(upstreamMessage, "response")
					if responseRaw.Exists() {
						if cvItems, ok := openAIWSExtractResponseOutputItems([]byte(responseRaw.Raw)); ok {
							if cvHashes, hok := openAIWSCanonicalItemHashes(cvItems); hok {
								deltaShadowRawClientEquiv = openAIWSHashSlicesEqual(deltaShadowRawOutputHashes, cvHashes)
							}
						}
					}
				}
				if deltaShadowEnabled && !deltaShadowOutputCaptured && len(deltaShadowRawDoneItems) > 0 {
					rawItems := openAIWSOrderedOutputDoneItems(deltaShadowRawDoneItems)
					if hashes, hok := openAIWSCanonicalItemHashes(rawItems); hok {
						if shapes, sok := openAIWSItemShapes(rawItems); sok {
							deltaShadowRawOutputHashes = hashes
							deltaShadowRawOutputShapes = shapes
							deltaShadowOutputCaptured = true
							visibleItems := openAIWSOrderedOutputDoneItems(deltaShadowVisibleDoneItems)
							if cvHashes, vok := openAIWSCanonicalItemHashes(visibleItems); vok {
								deltaShadowRawClientEquiv = openAIWSHashSlicesEqual(deltaShadowRawOutputHashes, cvHashes)
							} else {
								deltaShadowRawClientEquiv = false
							}
						}
					}
				}
				firstTokenMsValue := -1
				if firstTokenMs != nil {
					firstTokenMsValue = *firstTokenMs
				}
				if debugEnabled {
					logOpenAIWSModeDebug(
						"ingress_ws_turn_completed account_id=%d turn=%d conn_id=%s response_id=%s duration_ms=%d events=%d token_events=%d terminal_events=%d first_event=%s last_event=%s first_token_ms=%d client_disconnected=%v",
						account.ID,
						turn,
						truncateOpenAIWSLogValue(lease.ConnID(), openAIWSIDValueMaxLen),
						truncateOpenAIWSLogValue(responseID, openAIWSIDValueMaxLen),
						time.Since(turnStart).Milliseconds(),
						eventCount,
						tokenEventCount,
						terminalEventCount,
						truncateOpenAIWSLogValue(firstEventType, openAIWSLogValueMaxLen),
						truncateOpenAIWSLogValue(lastEventType, openAIWSLogValueMaxLen),
						firstTokenMsValue,
						clientDisconnected,
					)
				}
				imageCount := imageCounter.Count()
				resolvedImageBillingModel := imageBillingModel
				resolvedImageSizeTier := imageSizeTier
				if imageCount > 0 && (strings.TrimSpace(resolvedImageBillingModel) == "" || strings.TrimSpace(resolvedImageSizeTier) == "") {
					fallbackBillingModel, fallbackSizeTier, fallbackErr := resolveOpenAIResponsesImageBillingConfigFromBody(payload, originalModel)
					if fallbackErr == nil {
						if strings.TrimSpace(resolvedImageBillingModel) == "" {
							resolvedImageBillingModel = fallbackBillingModel
						}
						if strings.TrimSpace(resolvedImageSizeTier) == "" {
							resolvedImageSizeTier = fallbackSizeTier
						}
					}
				}
				result := &OpenAIForwardResult{
					RequestID:          responseID,
					Usage:              usage,
					Model:              originalModel,
					UpstreamModel:      mappedModel,
					ServiceTier:        extractOpenAIServiceTierFromBody(payload),
					ReasoningEffort:    ApplyThinkingEnabledFallback(extractOpenAIReasoningEffortFromBody(payload, originalModel), payload, mappedModel),
					Stream:             reqStream,
					OpenAIWSMode:       true,
					OpenAIWSProfile:    openAIWSProfileUsageString(openAIWSConnProfileSessionBound),
					OpenAIWSConnReused: lease.Reused(),
					ResponseHeaders:    lease.HandshakeHeaders(),
					Duration:           time.Since(turnStart),
					FirstTokenMs:       firstTokenMs,
					ClientDisconnected: clientDisconnected,

					DeltaShadowOutputHashes:   deltaShadowRawOutputHashes,
					DeltaShadowOutputShapes:   deltaShadowRawOutputShapes,
					DeltaShadowOutputCaptured: deltaShadowOutputCaptured,
					DeltaShadowRawClientEquiv: deltaShadowRawClientEquiv,
				}
				if replayInput := replayCollector.Items(); len(replayInput) > 0 {
					result.wsReplayInput = replayInput
					result.wsReplayInputExists = true
				}
				if imageCount > 0 {
					result.ImageCount = imageCount
					result.ImageSize = resolvedImageSizeTier
					result.ImageInputSize = imageInputSize
					result.BillingModel = resolvedImageBillingModel
				}
				return result, nil
			}
		}
	}

	currentPayload := firstPayload.payloadRaw
	currentOriginalModel := firstPayload.originalModel
	currentImageBillingModel := firstPayload.imageBillingModel
	currentImageSizeTier := firstPayload.imageSizeTier
	currentImageInputSize := firstPayload.imageInputSize
	currentPayloadBytes := firstPayload.payloadBytes
	isStrictAffinityTurn := func(payload []byte) bool {
		if !storeDisabled {
			return false
		}
		return strings.TrimSpace(openAIWSPayloadStringFromRaw(payload, "previous_response_id")) != ""
	}
	var sessionLease *openAIWSConnLease
	sessionConnID := ""
	pinnedSessionConnID := ""
	unpinSessionConn := func(connID string) {
		connID = strings.TrimSpace(connID)
		if connID == "" || pinnedSessionConnID != connID {
			return
		}
		pool.UnpinConn(account.ID, connID)
		pinnedSessionConnID = ""
	}
	pinSessionConn := func(connID string) {
		if !storeDisabled {
			return
		}
		connID = strings.TrimSpace(connID)
		if connID == "" || pinnedSessionConnID == connID {
			return
		}
		if pinnedSessionConnID != "" {
			pool.UnpinConn(account.ID, pinnedSessionConnID)
			pinnedSessionConnID = ""
		}
		if pool.PinConn(account.ID, connID) {
			pinnedSessionConnID = connID
		}
	}
	// lastTurnClean 标记最后一轮 sendAndRelay 是否正常完成（收到终端事件且客户端未断连）。
	// 所有异常路径（读写错误、error 事件、客户端断连）已在各自分支或上层（L3403）中 MarkBroken，
	// 因此 releaseSessionLease 中只需在非正常结束时 MarkBroken。
	lastTurnClean := false
	releaseSessionLease := func() {
		if sessionLease == nil {
			return
		}
		if !lastTurnClean {
			sessionLease.MarkBrokenFor("ingress_unclean_turn")
		}
		unpinSessionConn(sessionConnID)
		sessionLease.Release()
		if debugEnabled {
			logOpenAIWSModeDebug(
				"ingress_ws_upstream_released account_id=%d conn_id=%s",
				account.ID,
				truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
			)
		}
	}
	defer releaseSessionLease()

	turn := 1
	turnRetry := 0
	turnPrevRecoveryTried := false
	lastTurnFinishedAt := time.Time{}
	lastTurnResponseID := ""
	lastTurnPayload := []byte(nil)
	var lastTurnStrictState *openAIWSIngressPreviousTurnStrictState
	lastTurnReplayInput := []json.RawMessage(nil)
	lastTurnReplayInputExists := false
	currentTurnReplayInput := []json.RawMessage(nil)
	currentTurnReplayInputExists := false
	skipBeforeTurn := false
	hasCurrentOrReplayFunctionCallOutput := func(payload []byte) bool {
		if openAIWSRawPayloadHasToolCallOutput(payload) {
			return true
		}
		return currentTurnReplayInputExists && openAIWSRawItemsHasFunctionCallOutput(currentTurnReplayInput)
	}
	resetSessionLease := func(markBroken bool) {
		if sessionLease == nil {
			return
		}
		if markBroken {
			sessionLease.MarkBrokenFor("ingress_reset")
		}
		releaseSessionLease()
		sessionLease = nil
		sessionConnID = ""
		preferredConnID = ""
	}
	recoverIngressPrevResponseNotFound := func(relayErr error, turn int, connID string) bool {
		if !isOpenAIWSIngressPreviousResponseNotFound(relayErr) {
			return false
		}
		if turnPrevRecoveryTried || !s.openAIWSIngressPreviousResponseRecoveryEnabled() {
			return false
		}
		// 携带 function_call_output 的请求不能丢弃 previous_response_id：
		// 上游 API 需要 response chain 来匹配 tool_result 与之前的 tool_use，
		// 丢弃后会导致 "No tool call found for function call output" 400 错误。
		if hasCurrentOrReplayFunctionCallOutput(currentPayload) {
			return false
		}
		if isStrictAffinityTurn(currentPayload) {
			// Layer 2：严格亲和链路命中 previous_response_not_found 时，降级为“去掉 previous_response_id 后重放一次”。
			// 该错误说明续链锚点已失效，继续 strict fail-close 只会直接中断本轮请求。
			logOpenAIWSModeInfo(
				"ingress_ws_prev_response_recovery_layer2 account_id=%d turn=%d conn_id=%s store_disabled_conn_mode=%s action=drop_previous_response_id_retry",
				account.ID,
				turn,
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
				normalizeOpenAIWSLogValue(storeDisabledConnMode),
			)
		}
		turnPrevRecoveryTried = true
		updatedPayload, removed, dropErr := forceOpenAIWSRawPayloadFullCreate(currentPayload)
		if dropErr != nil || !removed {
			reason := "not_removed"
			if dropErr != nil {
				reason = "drop_error"
			}
			logOpenAIWSModeInfo(
				"ingress_ws_prev_response_recovery_skip account_id=%d turn=%d conn_id=%s reason=%s",
				account.ID,
				turn,
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
				normalizeOpenAIWSLogValue(reason),
			)
			return false
		}
		updatedWithInput, setInputErr := setOpenAIWSPayloadInputSequence(
			updatedPayload,
			currentTurnReplayInput,
			currentTurnReplayInputExists,
		)
		if setInputErr != nil {
			logOpenAIWSModeInfo(
				"ingress_ws_prev_response_recovery_skip account_id=%d turn=%d conn_id=%s reason=set_full_input_error cause=%s",
				account.ID,
				turn,
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
				truncateOpenAIWSLogValue(setInputErr.Error(), openAIWSLogValueMaxLen),
			)
			return false
		}
		logOpenAIWSModeInfo(
			"ingress_ws_prev_response_recovery account_id=%d turn=%d conn_id=%s action=drop_previous_response_id retry=1",
			account.ID,
			turn,
			truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
		)
		s.RecordOpenAIAccountRecoveryReason(account.ID, "previous_response_not_found")
		currentPayload = updatedWithInput
		currentPayloadBytes = len(updatedWithInput)
		storeDisabled = s.isOpenAIWSStoreDisabledInRequestRaw(currentPayload, account)
		resetSessionLease(true)
		skipBeforeTurn = true
		return true
	}
	retryIngressTurn := func(relayErr error, turn int, connID string) bool {
		if !isOpenAIWSIngressTurnRetryable(relayErr) || turnRetry >= 1 {
			return false
		}
		if isStrictAffinityTurn(currentPayload) {
			logOpenAIWSModeInfo(
				"ingress_ws_turn_retry_skip account_id=%d turn=%d conn_id=%s reason=strict_affinity",
				account.ID,
				turn,
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
			)
			return false
		}
		turnRetry++
		logOpenAIWSModeInfo(
			"ingress_ws_turn_retry account_id=%d turn=%d retry=%d reason=%s conn_id=%s",
			account.ID,
			turn,
			turnRetry,
			truncateOpenAIWSLogValue(openAIWSIngressTurnRetryReason(relayErr), openAIWSLogValueMaxLen),
			truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
		)
		resetSessionLease(true)
		skipBeforeTurn = true
		return true
	}
	// TEMP_DIAG(openai_ws_delta_shadow) remove_after_debug=true：strict-delta shadow 度量（不改 payload）。
	deltaShadowEnabled := openAIWSDeltaShadowEnabled()
	deltaShadowReqLogID, _ := openAIWSRequestLogIDs(c)
	for {
		if turn > 1 && !skipBeforeTurn && hooks != nil && hooks.BeforeRequest != nil {
			if err := hooks.BeforeRequest(turn, currentPayload, currentOriginalModel); err != nil {
				return err
			}
		}
		if !skipBeforeTurn && hooks != nil && hooks.BeforeTurn != nil {
			if err := hooks.BeforeTurn(turn); err != nil {
				return err
			}
		}
		skipBeforeTurn = false
		currentPreviousResponseID := openAIWSPayloadStringFromRaw(currentPayload, "previous_response_id")
		expectedPrev := strings.TrimSpace(lastTurnResponseID)
		toolSignals := analyzeToolContinuationSignalsFromRawPayload(currentPayload)
		hasFunctionCallOutput := toolSignals.HasFunctionCallOutput
		// store=false + function_call_output 场景必须有续链锚点。
		// 若客户端未传 previous_response_id，优先回填上一轮响应 ID，避免上游报 call_id 无法关联。
		if shouldInferIngressFunctionCallOutputPreviousResponseID(
			storeDisabled,
			turn,
			toolSignals,
			currentPreviousResponseID,
			expectedPrev,
		) {
			updatedPayload, setPrevErr := setPreviousResponseIDToRawPayload(currentPayload, expectedPrev)
			if setPrevErr != nil {
				logOpenAIWSModeInfo(
					"ingress_ws_function_call_output_prev_infer_skip account_id=%d turn=%d conn_id=%s reason=set_previous_response_id_error cause=%s expected_previous_response_id=%s",
					account.ID,
					turn,
					truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
					truncateOpenAIWSLogValue(setPrevErr.Error(), openAIWSLogValueMaxLen),
					truncateOpenAIWSLogValue(expectedPrev, openAIWSIDValueMaxLen),
				)
			} else {
				currentPayload = updatedPayload
				currentPayloadBytes = len(updatedPayload)
				currentPreviousResponseID = expectedPrev
				logOpenAIWSModeInfo(
					"ingress_ws_function_call_output_prev_infer account_id=%d turn=%d conn_id=%s action=set_previous_response_id previous_response_id=%s",
					account.ID,
					turn,
					truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
					truncateOpenAIWSLogValue(expectedPrev, openAIWSIDValueMaxLen),
				)
			}
		}
		nextReplayInput, nextReplayInputExists, replayInputErr := buildOpenAIWSReplayInputSequence(
			lastTurnReplayInput,
			lastTurnReplayInputExists,
			currentPayload,
			currentPreviousResponseID != "",
		)
		if replayInputErr != nil {
			logOpenAIWSModeInfo(
				"ingress_ws_replay_input_skip account_id=%d turn=%d conn_id=%s reason=build_error cause=%s",
				account.ID,
				turn,
				truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
				truncateOpenAIWSLogValue(replayInputErr.Error(), openAIWSLogValueMaxLen),
			)
			currentTurnReplayInput = nil
			currentTurnReplayInputExists = false
		} else {
			currentTurnReplayInput = nextReplayInput
			currentTurnReplayInputExists = nextReplayInputExists
		}
		replayHasFunctionCallOutput := currentTurnReplayInputExists &&
			openAIWSRawItemsHasFunctionCallOutput(currentTurnReplayInput)
		hasFunctionCallOutput = hasFunctionCallOutput || replayHasFunctionCallOutput
		if storeDisabled && turn > 1 && currentPreviousResponseID != "" {
			shouldKeepPreviousResponseID := false
			strictReason := ""
			var strictErr error
			if lastTurnStrictState != nil {
				shouldKeepPreviousResponseID, strictReason, strictErr = shouldKeepIngressPreviousResponseIDWithStrictState(
					lastTurnStrictState,
					currentPayload,
					lastTurnResponseID,
					hasFunctionCallOutput,
				)
			} else {
				shouldKeepPreviousResponseID, strictReason, strictErr = shouldKeepIngressPreviousResponseID(
					lastTurnPayload,
					currentPayload,
					lastTurnResponseID,
					hasFunctionCallOutput,
				)
			}
			if strictErr != nil {
				logOpenAIWSModeInfo(
					"ingress_ws_prev_response_strict_eval account_id=%d turn=%d conn_id=%s action=keep_previous_response_id reason=%s cause=%s previous_response_id=%s expected_previous_response_id=%s has_function_call_output=%v",
					account.ID,
					turn,
					truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
					normalizeOpenAIWSLogValue(strictReason),
					truncateOpenAIWSLogValue(strictErr.Error(), openAIWSLogValueMaxLen),
					truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
					truncateOpenAIWSLogValue(expectedPrev, openAIWSIDValueMaxLen),
					hasFunctionCallOutput,
				)
			} else if !shouldKeepPreviousResponseID {
				updatedPayload, removed, dropErr := forceOpenAIWSRawPayloadFullCreate(currentPayload)
				if dropErr != nil || !removed {
					dropReason := "not_removed"
					if dropErr != nil {
						dropReason = "drop_error"
					}
					logOpenAIWSModeInfo(
						"ingress_ws_prev_response_strict_eval account_id=%d turn=%d conn_id=%s action=keep_previous_response_id reason=%s drop_reason=%s previous_response_id=%s expected_previous_response_id=%s has_function_call_output=%v",
						account.ID,
						turn,
						truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
						normalizeOpenAIWSLogValue(strictReason),
						normalizeOpenAIWSLogValue(dropReason),
						truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
						truncateOpenAIWSLogValue(expectedPrev, openAIWSIDValueMaxLen),
						hasFunctionCallOutput,
					)
				} else {
					updatedWithInput, setInputErr := setOpenAIWSPayloadInputSequence(
						updatedPayload,
						currentTurnReplayInput,
						currentTurnReplayInputExists,
					)
					if setInputErr != nil {
						logOpenAIWSModeInfo(
							"ingress_ws_prev_response_strict_eval account_id=%d turn=%d conn_id=%s action=keep_previous_response_id reason=%s drop_reason=set_full_input_error previous_response_id=%s expected_previous_response_id=%s cause=%s has_function_call_output=%v",
							account.ID,
							turn,
							truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
							normalizeOpenAIWSLogValue(strictReason),
							truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
							truncateOpenAIWSLogValue(expectedPrev, openAIWSIDValueMaxLen),
							truncateOpenAIWSLogValue(setInputErr.Error(), openAIWSLogValueMaxLen),
							hasFunctionCallOutput,
						)
					} else {
						currentPayload = updatedWithInput
						currentPayloadBytes = len(updatedWithInput)
						storeDisabled = s.isOpenAIWSStoreDisabledInRequestRaw(currentPayload, account)
						logOpenAIWSModeInfo(
							"ingress_ws_prev_response_strict_eval account_id=%d turn=%d conn_id=%s action=drop_previous_response_id_full_create reason=%s previous_response_id=%s expected_previous_response_id=%s has_function_call_output=%v",
							account.ID,
							turn,
							truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
							normalizeOpenAIWSLogValue(strictReason),
							truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
							truncateOpenAIWSLogValue(expectedPrev, openAIWSIDValueMaxLen),
							hasFunctionCallOutput,
						)
						currentPreviousResponseID = ""
					}
				}
			}
		}
		forcePreferredConn := isStrictAffinityTurn(currentPayload)
		if sessionLease == nil {
			acquiredLease, acquireErr := acquireTurnLease(turn, preferredConnID, forcePreferredConn)
			if acquireErr != nil {
				return fmt.Errorf("acquire upstream websocket: %w", acquireErr)
			}
			sessionLease = acquiredLease
			sessionConnID = strings.TrimSpace(sessionLease.ConnID())
			if storeDisabled {
				pinSessionConn(sessionConnID)
			} else {
				unpinSessionConn(sessionConnID)
			}
		}
		shouldPreflightPing := turn > 1 && sessionLease != nil && turnRetry == 0
		preflightPingIdle := s.openAIWSIngressPreflightPingIdle()
		if shouldPreflightPing && preflightPingIdle > 0 && !lastTurnFinishedAt.IsZero() {
			if time.Since(lastTurnFinishedAt) < preflightPingIdle {
				shouldPreflightPing = false
			}
		}
		if shouldPreflightPing {
			if pingErr := sessionLease.PingWithTimeout(openAIWSConnHealthCheckTO); pingErr != nil {
				logOpenAIWSModeInfo(
					"ingress_ws_upstream_preflight_ping_fail account_id=%d turn=%d conn_id=%s cause=%s",
					account.ID,
					turn,
					truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
					truncateOpenAIWSLogValue(pingErr.Error(), openAIWSLogValueMaxLen),
				)
				if forcePreferredConn {
					// 携带 function_call_output 的请求不能丢弃 previous_response_id：
					// 上游 API 需要 response chain 来匹配 tool_result 与之前的 tool_use，
					// 除非 replay input 已经包含与每个 tool_result 匹配的 tool_use 上下文。
					hasFCOutput := hasFunctionCallOutput
					hasReplayToolContext := hasFCOutput &&
						currentTurnReplayInputExists &&
						openAIWSRawItemsHaveToolCallContextForOutputs(currentTurnReplayInput)
					if !turnPrevRecoveryTried && currentPreviousResponseID != "" && (!hasFCOutput || hasReplayToolContext) {
						updatedPayload, removed, dropErr := forceOpenAIWSRawPayloadFullCreate(currentPayload)
						if dropErr != nil || !removed {
							reason := "not_removed"
							if dropErr != nil {
								reason = "drop_error"
							}
							logOpenAIWSModeInfo(
								"ingress_ws_preflight_ping_recovery_skip account_id=%d turn=%d conn_id=%s reason=%s previous_response_id=%s",
								account.ID,
								turn,
								truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
								normalizeOpenAIWSLogValue(reason),
								truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
							)
						} else {
							updatedWithInput, setInputErr := setOpenAIWSPayloadInputSequence(
								updatedPayload,
								currentTurnReplayInput,
								currentTurnReplayInputExists,
							)
							if setInputErr != nil {
								logOpenAIWSModeInfo(
									"ingress_ws_preflight_ping_recovery_skip account_id=%d turn=%d conn_id=%s reason=set_full_input_error previous_response_id=%s cause=%s",
									account.ID,
									turn,
									truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
									truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
									truncateOpenAIWSLogValue(setInputErr.Error(), openAIWSLogValueMaxLen),
								)
							} else {
								logOpenAIWSModeInfo(
									"ingress_ws_preflight_ping_recovery account_id=%d turn=%d conn_id=%s action=drop_previous_response_id_retry previous_response_id=%s has_function_call_output=%v has_replay_tool_context=%v",
									account.ID,
									turn,
									truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
									truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
									hasFCOutput,
									hasReplayToolContext,
								)
								turnPrevRecoveryTried = true
								currentPayload = updatedWithInput
								currentPayloadBytes = len(updatedWithInput)
								storeDisabled = s.isOpenAIWSStoreDisabledInRequestRaw(currentPayload, account)
								resetSessionLease(true)
								skipBeforeTurn = true
								continue
							}
						}
					}
					if hasFCOutput && currentPreviousResponseID != "" {
						reason := "function_call_output_missing_replay_context"
						if hasReplayToolContext {
							reason = "function_call_output_replay_not_applied"
						}
						logOpenAIWSModeInfo(
							"ingress_ws_preflight_ping_recovery_skip account_id=%d turn=%d conn_id=%s reason=%s action=fail_close previous_response_id=%s has_replay_tool_context=%v",
							account.ID,
							turn,
							truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
							reason,
							truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
							hasReplayToolContext,
						)
					}
					resetSessionLease(true)
					return NewOpenAIWSClientCloseError(
						coderws.StatusPolicyViolation,
						"upstream continuation connection is unavailable; please restart the conversation",
						pingErr,
					)
				}
				resetSessionLease(true)

				acquiredLease, acquireErr := acquireTurnLease(turn, preferredConnID, forcePreferredConn)
				if acquireErr != nil {
					return fmt.Errorf("acquire upstream websocket after preflight ping fail: %w", acquireErr)
				}
				sessionLease = acquiredLease
				sessionConnID = strings.TrimSpace(sessionLease.ConnID())
				if storeDisabled {
					pinSessionConn(sessionConnID)
				}
			}
		}
		connID := sessionConnID
		if currentPreviousResponseID != "" {
			chainedFromLast := expectedPrev != "" && currentPreviousResponseID == expectedPrev
			currentPreviousResponseIDKind := ClassifyOpenAIPreviousResponseIDKind(currentPreviousResponseID)
			logOpenAIWSModeInfo(
				"ingress_ws_turn_chain account_id=%d turn=%d conn_id=%s previous_response_id=%s previous_response_id_kind=%s last_turn_response_id=%s chained_from_last=%v preferred_conn_id=%s header_session_id=%s header_conversation_id=%s has_turn_state=%v turn_state_len=%d has_prompt_cache_key=%v store_disabled=%v",
				account.ID,
				turn,
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
				truncateOpenAIWSLogValue(currentPreviousResponseID, openAIWSIDValueMaxLen),
				normalizeOpenAIWSLogValue(currentPreviousResponseIDKind),
				truncateOpenAIWSLogValue(expectedPrev, openAIWSIDValueMaxLen),
				chainedFromLast,
				truncateOpenAIWSLogValue(preferredConnID, openAIWSIDValueMaxLen),
				openAIWSHeaderValueForLog(baseAcquireReq.Headers, "session_id"),
				openAIWSHeaderValueForLog(baseAcquireReq.Headers, "conversation_id"),
				turnState != "",
				len(turnState),
				openAIWSPayloadStringFromRaw(currentPayload, "prompt_cache_key") != "",
				storeDisabled,
			)
		}

		// TEMP_DIAG(openai_ws_delta_shadow): owner 在发送前评估 strict-delta 候选（只读），
		// owner 生命周期 = TryBegin → 评估 → (完成后)写状态 → End；non-owner 仅记录 same_session_in_flight。
		shadowOwner := false
		if deltaShadowEnabled && stateStore != nil && sessionHash != "" {
			shadowOwner = stateStore.TrySessionInFlight(groupID, apiKeyID, sessionHash)
			if !shadowOwner {
				logOpenAIWSDeltaShadow(openAIWSDeltaShadowLog{
					GroupID:        groupID,
					APIKeyID:       apiKeyID,
					SessionHash:    sessionHash,
					RequestID:      deltaShadowReqLogID,
					AccountID:      account.ID,
					ConnID:         connID,
					Candidate:      false,
					FallbackReason: "same_session_in_flight",
				})
			} else if turn > 1 {
				cached, found := stateStore.GetSessionContext(groupID, apiKeyID, sessionHash)
				connMostRecent, _ := stateStore.GetConnLastResponse(connID)
				shadowInput := openAIWSDeltaShadowInput{
					RequestID:                deltaShadowReqLogID,
					AccountID:                account.ID,
					LeaseConnID:              connID,
					ConnMostRecentResponseID: connMostRecent,
					CurrentPayload:           currentPayload,
					HasFunctionCallOutput:    hasFunctionCallOutput,
					StickyAccountID:          account.ID,
					StickyAccountHit:         true,
					StickyAccountMismatch:    false,
					ConnAffinityHit:          true,
					PreferredConnID:          connID,
					StoreFallbackReason:      "",
					Cached:                   cached,
					CachedFound:              found,
				}
				populateOpenAIWSDeltaShadowBindingDiagnostics(&shadowInput, stateStore, pool, groupID, apiKeyID, sessionHash, account.ID, cached, found)
				logOpenAIWSDeltaShadow(evaluateOpenAIWSDeltaShadowCandidate(shadowInput))
			}
		}

		result, relayErr := sendAndRelay(turn, sessionLease, currentPayload, currentPayloadBytes, currentOriginalModel, currentImageBillingModel, currentImageSizeTier, currentImageInputSize)
		if relayErr != nil {
			if shadowOwner {
				stateStore.EndSessionInFlight(groupID, apiKeyID, sessionHash)
				shadowOwner = false
			}
			lastTurnClean = false
			if recoverIngressPrevResponseNotFound(relayErr, turn, connID) {
				continue
			}
			if retryIngressTurn(relayErr, turn, connID) {
				continue
			}
			finalErr := relayErr
			if unwrapped := errors.Unwrap(relayErr); unwrapped != nil {
				finalErr = unwrapped
			}
			if hooks != nil && hooks.AfterTurn != nil {
				hooks.AfterTurn(turn, cloneOpenAIWSPayloadBytes(currentPayload), nil, finalErr)
			}
			sessionLease.MarkBrokenFor("ingress_relay_error")
			return finalErr
		}
		if result == nil {
			if shadowOwner {
				stateStore.EndSessionInFlight(groupID, apiKeyID, sessionHash)
				shadowOwner = false
			}
			return errors.New("websocket turn result is nil")
		}
		turnRetry = 0
		turnPrevRecoveryTried = false
		lastTurnFinishedAt = time.Now()
		lastTurnClean = !result.ClientDisconnected
		if hooks != nil && hooks.AfterTurn != nil {
			hooks.AfterTurn(turn, cloneOpenAIWSPayloadBytes(currentPayload), result, nil)
		}
		responseID := strings.TrimSpace(result.RequestID)
		lastTurnResponseID = responseID
		lastTurnPayload = cloneOpenAIWSPayloadBytes(currentPayload)
		lastTurnReplayInput = cloneOpenAIWSRawMessages(currentTurnReplayInput)
		lastTurnReplayInputExists = currentTurnReplayInputExists
		if result.wsReplayInputExists {
			lastTurnReplayInput = append(lastTurnReplayInput, cloneOpenAIWSRawMessages(result.wsReplayInput)...)
			lastTurnReplayInputExists = true
		}
		nextStrictState, strictStateErr := buildOpenAIWSIngressPreviousTurnStrictState(currentPayload)
		if strictStateErr != nil {
			lastTurnStrictState = nil
			logOpenAIWSModeInfo(
				"ingress_ws_prev_response_strict_state_skip account_id=%d turn=%d conn_id=%s reason=build_error cause=%s",
				account.ID,
				turn,
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
				truncateOpenAIWSLogValue(strictStateErr.Error(), openAIWSLogValueMaxLen),
			)
		} else {
			lastTurnStrictState = nextStrictState
		}

		bindCleanTurnResponseAccount := func(boundConnID string) {
			if responseID != "" && stateStore != nil && !result.ClientDisconnected {
				ttl := s.openAIWSResponseStickyTTL()
				bindAccountErr := stateStore.BindResponseAccount(ctx, groupID, apiKeyID, responseID, account.ID, ttl)
				logOpenAIWSBindResponseAccountWarn(groupID, account.ID, responseID, bindAccountErr)
				logOpenAIWSResponseStickyBind(groupID, apiKeyID, account.ID, account.Type, responseID, boundConnID, ttl, bindAccountErr)
			}
		}
		bindCleanTurnState := func() {
			boundConnID := ""
			if responseID != "" && stateStore != nil && !result.ClientDisconnected {
				boundConnID = connID
				stateStore.BindResponseConn(groupID, apiKeyID, responseID, boundConnID, s.openAIWSResponseStickyTTL())
			}
			bindCleanTurnResponseAccount(boundConnID)
			if stateStore != nil && storeDisabled && sessionHash != "" && !result.ClientDisconnected {
				ttl := s.openAIWSSessionStickyTTL()
				stateStore.BindSessionConn(groupID, apiKeyID, account.ID, sessionHash, connID, ttl)
				logOpenAIWSSessionConnBind(groupID, apiKeyID, account.ID, account.Type, sessionHash, connID, ttl)
			}
			// TEMP_DIAG(openai_ws_delta_shadow): owner 在干净完成后写会话上下文指纹；
			// 优先用 input_N(本轮实际发送的 full input) ++ raw output_N，缺少 raw output 时降级为 input-only context。
			if shadowOwner {
				if deltaShadowEnabled && stateStore != nil && responseID != "" && connID != "" &&
					!result.ClientDisconnected {
					if inputItems, _, ierr := openAIWSExtractNormalizedInputSequence(currentPayload); ierr == nil {
						if inputHashes, hok := openAIWSCanonicalItemHashes(inputItems); hok {
							var materializedShapes []string
							if inputShapes, sok := openAIWSItemShapes(inputItems); sok && len(result.DeltaShadowOutputShapes) == len(result.DeltaShadowOutputHashes) {
								materializedShapes = make([]string, 0, len(inputShapes)+len(result.DeltaShadowOutputShapes))
								materializedShapes = append(materializedShapes, inputShapes...)
								materializedShapes = append(materializedShapes, result.DeltaShadowOutputShapes...)
							}
							materialized := make([][32]byte, 0, len(inputHashes)+len(result.DeltaShadowOutputHashes))
							materialized = append(materialized, inputHashes...)
							materialized = append(materialized, result.DeltaShadowOutputHashes...)
							nonInputHash, _, nonInputFields := openAIWSNonInputFingerprint(currentPayload)
							ttl := s.openAIWSSessionStickyTTL()
							stateStore.BindConnLastResponse(connID, responseID, ttl)
							sessionContextValue := openAIWSSessionContextValue{
								accountID:               account.ID,
								connID:                  connID,
								lastResponseID:          responseID,
								materializedHashes:      materialized,
								materializedShapes:      materializedShapes,
								materializedCount:       len(materialized),
								inputCount:              len(inputHashes),
								inputOnlyContext:        !result.DeltaShadowOutputCaptured,
								nonInputHash:            nonInputHash,
								nonInputFields:          nonInputFields,
								rawVsClientVisibleEqual: result.DeltaShadowRawClientEquiv,
							}
							stateStore.BindSessionContext(groupID, apiKeyID, sessionHash, sessionContextValue, ttl)
							logOpenAIWSSessionContextBind(
								groupID,
								apiKeyID,
								account.ID,
								account.Type,
								sessionHash,
								connID,
								responseID,
								ttl,
								len(inputHashes),
								len(materialized),
								result.DeltaShadowOutputCaptured,
								result.DeltaShadowRawClientEquiv,
							)
						}
					}
				}
				stateStore.EndSessionInFlight(groupID, apiKeyID, sessionHash)
				shadowOwner = false
			}
			if connID != "" {
				preferredConnID = connID
			}
		}

		nextClientMessage, readErr := readClientMessage()
		if readErr != nil {
			if isOpenAIWSClientDisconnectError(readErr) {
				if coderws.CloseStatus(readErr) == coderws.StatusNormalClosure && !result.ClientDisconnected {
					bindCleanTurnState()
				} else {
					bindCleanTurnResponseAccount("")
					lastTurnClean = false
					if sessionLease != nil {
						sessionLease.MarkBrokenFor("ingress_client_closed")
					}
					if stateStore != nil {
						if responseID != "" {
							stateStore.DeleteResponseConn(groupID, apiKeyID, responseID)
							if result.ClientDisconnected {
								_ = stateStore.DeleteResponseAccount(ctx, groupID, apiKeyID, responseID)
							}
						}
						if storeDisabled && sessionHash != "" {
							stateStore.DeleteSessionConn(groupID, apiKeyID, account.ID, sessionHash)
						}
					}
				}
				closeStatus, closeReason := summarizeOpenAIWSReadCloseError(readErr)
				logOpenAIWSModeInfo(
					"ingress_ws_client_closed account_id=%d conn_id=%s close_status=%s close_reason=%s",
					account.ID,
					truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
					closeStatus,
					truncateOpenAIWSLogValue(closeReason, openAIWSHeaderValueMaxLen),
				)
				return nil
			}
			return fmt.Errorf("read client websocket request: %w", readErr)
		}

		bindCleanTurnState()
		nextPayload, parseErr := parseClientPayload(nextClientMessage)
		if parseErr != nil {
			return parseErr
		}
		nextStoreDisabled := s.isOpenAIWSStoreDisabledInRequestRaw(nextPayload.payloadRaw, account)
		shouldBranchTurn, branchReason, branchErr := shouldBranchOpenAIWSIngressFollowupTurn(
			turn+1,
			nextPayload.rawForHash,
			nextPayload.payloadRaw,
			nextStoreDisabled,
			lastTurnPayload,
			lastTurnStrictState,
			lastTurnResponseID,
		)
		if shouldBranchTurn {
			if stateStore != nil && sessionHash != "" {
				stateStore.DeleteSessionTurnState(groupID, sessionHash)
				stateStore.DeleteSessionConn(groupID, apiKeyID, account.ID, sessionHash)
				stateStore.DeleteSessionContext(groupID, apiKeyID, sessionHash)
			}
			updatedPayload, removedPrev, dropErr := dropPreviousResponseIDFromRawPayload(nextPayload.payloadRaw)
			if dropErr != nil {
				return fmt.Errorf("drop previous_response_id for ingress branch replay: %w", dropErr)
			}
			if removedPrev {
				nextPayload.payloadRaw = updatedPayload
				nextPayload.payloadBytes = len(updatedPayload)
				nextPayload.previousResponseID = ""
			}
			logMsg := "ingress_ws_branch_replay account_id=%d turn=%d next_turn=%d conn_id=%s reason=%s removed_previous_response_id=%v"
			logArgs := []any{
				account.ID,
				turn,
				turn + 1,
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
				normalizeOpenAIWSLogValue(branchReason),
				removedPrev,
			}
			if branchErr != nil {
				logMsg += " cause=%s"
				logArgs = append(logArgs, truncateOpenAIWSLogValue(branchErr.Error(), openAIWSLogValueMaxLen))
			}
			logOpenAIWSModeInfo(logMsg, logArgs...)
			resetSessionLease(true)
			preferredConnID = ""
			turnState = ""
			lastTurnFinishedAt = time.Time{}
			lastTurnResponseID = ""
			lastTurnPayload = nil
			lastTurnStrictState = nil
			lastTurnReplayInput = nil
			lastTurnReplayInputExists = false
			currentTurnReplayInput = nil
			currentTurnReplayInputExists = false
		}
		if !shouldBranchTurn && connID != "" {
			preferredConnID = connID
		}
		if nextPayload.promptCacheKey != "" {
			// ingress 会话在整个客户端 WS 生命周期内复用同一上游连接；
			// prompt_cache_key 对握手头的更新仅在未来需要重新建连时生效。
			updatedHeaders, _ := s.buildOpenAIWSHeaders(c, account, token, wsDecision, isCodexCLI, turnState, strings.TrimSpace(c.GetHeader(openAIWSTurnMetadataHeader)), nextPayload.promptCacheKey)
			applyOpenAIWSFingerprintRuntimeHeaders(updatedHeaders, tlsFPRuntime)
			baseAcquireReq.Headers = updatedHeaders
		}
		if nextPayload.previousResponseID != "" {
			expectedPrev := strings.TrimSpace(lastTurnResponseID)
			chainedFromLast := expectedPrev != "" && nextPayload.previousResponseID == expectedPrev
			nextPreviousResponseIDKind := ClassifyOpenAIPreviousResponseIDKind(nextPayload.previousResponseID)
			logOpenAIWSModeInfo(
				"ingress_ws_next_turn_chain account_id=%d turn=%d next_turn=%d conn_id=%s previous_response_id=%s previous_response_id_kind=%s last_turn_response_id=%s chained_from_last=%v has_prompt_cache_key=%v store_disabled=%v",
				account.ID,
				turn,
				turn+1,
				truncateOpenAIWSLogValue(connID, openAIWSIDValueMaxLen),
				truncateOpenAIWSLogValue(nextPayload.previousResponseID, openAIWSIDValueMaxLen),
				normalizeOpenAIWSLogValue(nextPreviousResponseIDKind),
				truncateOpenAIWSLogValue(expectedPrev, openAIWSIDValueMaxLen),
				chainedFromLast,
				nextPayload.promptCacheKey != "",
				storeDisabled,
			)
		}
		if stickyConnID := strings.TrimSpace(nextPayload.preferredConnID); stickyConnID != "" {
			if sessionConnID != "" && stickyConnID != sessionConnID {
				logOpenAIWSModeInfo(
					"ingress_ws_keep_session_conn account_id=%d turn=%d conn_id=%s sticky_conn_id=%s previous_response_id=%s",
					account.ID,
					turn,
					truncateOpenAIWSLogValue(sessionConnID, openAIWSIDValueMaxLen),
					truncateOpenAIWSLogValue(stickyConnID, openAIWSIDValueMaxLen),
					truncateOpenAIWSLogValue(nextPayload.previousResponseID, openAIWSIDValueMaxLen),
				)
			} else {
				preferredConnID = stickyConnID
			}
		}
		currentPayload = nextPayload.payloadRaw
		currentOriginalModel = nextPayload.originalModel
		currentImageBillingModel = nextPayload.imageBillingModel
		currentImageSizeTier = nextPayload.imageSizeTier
		currentImageInputSize = nextPayload.imageInputSize
		currentPayloadBytes = nextPayload.payloadBytes
		storeDisabled = s.isOpenAIWSStoreDisabledInRequestRaw(currentPayload, account)
		if !storeDisabled {
			unpinSessionConn(sessionConnID)
		}
		turn++
	}
}

func (s *OpenAIGatewayService) isOpenAIWSGeneratePrewarmEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.PrewarmGenerateEnabled
}

// performOpenAIWSGeneratePrewarm 在 WSv2 下执行可选的 generate=false 预热。
// 预热默认关闭，仅在配置开启后生效；失败时按可恢复错误回退到 HTTP。
func (s *OpenAIGatewayService) performOpenAIWSGeneratePrewarm(
	ctx context.Context,
	lease *openAIWSConnLease,
	decision OpenAIWSProtocolDecision,
	payload map[string]any,
	previousResponseID string,
	reqBody map[string]any,
	account *Account,
	stateStore OpenAIWSStateStore,
	groupID int64,
	apiKeyID int64,
) error {
	if s == nil {
		return nil
	}
	if lease == nil || account == nil {
		logOpenAIWSModeInfo("prewarm_skip reason=invalid_state has_lease=%v has_account=%v", lease != nil, account != nil)
		return nil
	}
	connID := strings.TrimSpace(lease.ConnID())
	if !s.isOpenAIWSGeneratePrewarmEnabled() {
		return nil
	}
	if decision.Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
		logOpenAIWSModeInfo(
			"prewarm_skip account_id=%d conn_id=%s reason=transport_not_v2 transport=%s",
			account.ID,
			connID,
			normalizeOpenAIWSLogValue(string(decision.Transport)),
		)
		return nil
	}
	if strings.TrimSpace(previousResponseID) != "" {
		logOpenAIWSModeInfo(
			"prewarm_skip account_id=%d conn_id=%s reason=has_previous_response_id previous_response_id=%s",
			account.ID,
			connID,
			truncateOpenAIWSLogValue(previousResponseID, openAIWSIDValueMaxLen),
		)
		return nil
	}
	if lease.IsPrewarmed() {
		logOpenAIWSModeInfo("prewarm_skip account_id=%d conn_id=%s reason=already_prewarmed", account.ID, connID)
		return nil
	}
	if NeedsToolContinuation(reqBody) {
		logOpenAIWSModeInfo("prewarm_skip account_id=%d conn_id=%s reason=tool_continuation", account.ID, connID)
		return nil
	}
	prewarmStart := time.Now()
	logOpenAIWSModeInfo("prewarm_start account_id=%d conn_id=%s", account.ID, connID)

	prewarmPayload := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		prewarmPayload[k] = v
	}
	prewarmPayload["generate"] = false
	prewarmPayloadJSON := payloadAsJSONBytes(prewarmPayload)

	if err := lease.WriteJSONWithContextTimeout(ctx, prewarmPayload, s.openAIWSWriteTimeout()); err != nil {
		lease.MarkBrokenFor("prewarm_write_fail")
		logOpenAIWSModeInfo(
			"prewarm_write_fail account_id=%d conn_id=%s cause=%s",
			account.ID,
			connID,
			truncateOpenAIWSLogValue(err.Error(), openAIWSLogValueMaxLen),
		)
		return wrapOpenAIWSFallback("prewarm_write", err)
	}
	logOpenAIWSModeInfo("prewarm_write_sent account_id=%d conn_id=%s payload_bytes=%d", account.ID, connID, len(prewarmPayloadJSON))

	prewarmResponseID := ""
	prewarmEventCount := 0
	prewarmTerminalCount := 0
	for {
		message, readErr := lease.ReadMessageWithContextTimeout(ctx, s.openAIWSReadTimeout())
		if readErr != nil {
			lease.MarkBrokenFor("prewarm_read_fail")
			closeStatus, closeReason := summarizeOpenAIWSReadCloseError(readErr)
			logOpenAIWSModeInfo(
				"prewarm_read_fail account_id=%d conn_id=%s close_status=%s close_reason=%s cause=%s events=%d",
				account.ID,
				connID,
				closeStatus,
				closeReason,
				truncateOpenAIWSLogValue(readErr.Error(), openAIWSLogValueMaxLen),
				prewarmEventCount,
			)
			return wrapOpenAIWSFallback("prewarm_"+classifyOpenAIWSReadFallbackReason(readErr), readErr)
		}

		eventType, eventResponseID, _ := parseOpenAIWSEventEnvelope(message)
		if eventType == "" {
			continue
		}
		prewarmEventCount++
		if prewarmResponseID == "" && eventResponseID != "" {
			prewarmResponseID = eventResponseID
		}
		if prewarmEventCount <= openAIWSPrewarmEventLogHead || eventType == "error" || isOpenAIWSTerminalEvent(eventType) {
			logOpenAIWSModeInfo(
				"prewarm_event account_id=%d conn_id=%s idx=%d type=%s bytes=%d",
				account.ID,
				connID,
				prewarmEventCount,
				truncateOpenAIWSLogValue(eventType, openAIWSLogValueMaxLen),
				len(message),
			)
		}

		if eventType == "error" {
			errCodeRaw, errTypeRaw, errMsgRaw := parseOpenAIWSErrorEventFields(message)
			s.persistOpenAIWSRateLimitSignal(ctx, account, lease.HandshakeHeaders(), message, errCodeRaw, errTypeRaw, errMsgRaw)
			errMsg := strings.TrimSpace(errMsgRaw)
			if errMsg == "" {
				errMsg = "OpenAI websocket prewarm error"
			}
			fallbackReason, canFallback := classifyOpenAIWSErrorEventFromRaw(errCodeRaw, errTypeRaw, errMsgRaw)
			if fallbackReason == "ws_connection_limit_reached" && lease.ConnAge() >= openAIWSConnLimitTTLEvictThreshold {
				fallbackReason = "ws_conn_ttl_evict"
			}
			errCode, errType, errMessage := summarizeOpenAIWSErrorEventFieldsFromRaw(errCodeRaw, errTypeRaw, errMsgRaw)
			logOpenAIWSModeInfo(
				"prewarm_error_event account_id=%d conn_id=%s idx=%d fallback_reason=%s can_fallback=%v err_code=%s err_type=%s err_message=%s",
				account.ID,
				connID,
				prewarmEventCount,
				truncateOpenAIWSLogValue(fallbackReason, openAIWSLogValueMaxLen),
				canFallback,
				errCode,
				errType,
				errMessage,
			)
			lease.MarkBrokenFor("prewarm_error_event")
			if canFallback {
				return wrapOpenAIWSFallback("prewarm_"+fallbackReason, errors.New(errMsg))
			}
			return wrapOpenAIWSFallback("prewarm_error_event", errors.New(errMsg))
		}

		if isOpenAIWSTerminalEvent(eventType) {
			prewarmTerminalCount++
			break
		}
	}

	lease.MarkPrewarmed()
	if prewarmResponseID != "" && stateStore != nil {
		ttl := s.openAIWSResponseStickyTTL()
		logOpenAIWSBindResponseAccountWarn(groupID, account.ID, prewarmResponseID, stateStore.BindResponseAccount(ctx, groupID, apiKeyID, prewarmResponseID, account.ID, ttl))
		stateStore.BindResponseConn(groupID, apiKeyID, prewarmResponseID, lease.ConnID(), ttl)
	}
	logOpenAIWSModeInfo(
		"prewarm_done account_id=%d conn_id=%s response_id=%s events=%d terminal_events=%d duration_ms=%d",
		account.ID,
		connID,
		truncateOpenAIWSLogValue(prewarmResponseID, openAIWSIDValueMaxLen),
		prewarmEventCount,
		prewarmTerminalCount,
		time.Since(prewarmStart).Milliseconds(),
	)
	return nil
}

func payloadAsJSON(payload map[string]any) string {
	return string(payloadAsJSONBytes(payload))
}

func payloadAsJSONBytes(payload map[string]any) []byte {
	if len(payload) == 0 {
		return []byte("{}")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return []byte("{}")
	}
	return body
}

func openAIWSMaterializeResponseOutputFromDoneItems(response []byte, doneItems map[int]json.RawMessage) []byte {
	if len(response) == 0 || len(doneItems) == 0 {
		return response
	}
	output := gjson.GetBytes(response, "output")
	if output.Exists() && (!output.IsArray() || len(output.Array()) > 0) {
		return response
	}
	orderedItems := openAIWSOrderedOutputDoneItems(doneItems)
	if len(orderedItems) == 0 {
		return response
	}
	rawItems, err := json.Marshal(orderedItems)
	if err != nil {
		return response
	}
	next, err := sjson.SetRawBytes(response, "output", rawItems)
	if err != nil {
		return response
	}
	return next
}

func isOpenAIWSTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
		return true
	default:
		return false
	}
}

func isOpenAIWSTokenEvent(eventType string) bool {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return false
	}
	switch eventType {
	case "response.created", "response.in_progress", "response.output_item.done":
		return false
	}
	if strings.Contains(eventType, ".delta") {
		return true
	}
	if strings.HasPrefix(eventType, "response.output_text") {
		return true
	}
	if strings.HasPrefix(eventType, "response.output") {
		return true
	}
	// 终止事件（response.completed/done/failed/...）由 isOpenAIWSTerminalEvent 单独处理。
	// 不能把它们当作 token event，否则当上游没有可识别的 delta 时，
	// firstTokenMs 会被填到终止时刻，等于把"总耗时"误报为"首 token 延迟"。
	return false
}

func replaceOpenAIWSMessageModel(message []byte, fromModel, toModel string) []byte {
	if len(message) == 0 {
		return message
	}
	if strings.TrimSpace(fromModel) == "" || strings.TrimSpace(toModel) == "" || fromModel == toModel {
		return message
	}
	if !bytes.Contains(message, []byte(`"model"`)) || !bytes.Contains(message, []byte(fromModel)) {
		return message
	}
	modelValues := gjson.GetManyBytes(message, "model", "response.model")
	replaceModel := modelValues[0].Exists() && modelValues[0].Str == fromModel
	replaceResponseModel := modelValues[1].Exists() && modelValues[1].Str == fromModel
	if !replaceModel && !replaceResponseModel {
		return message
	}
	updated := message
	if replaceModel {
		if next, err := sjson.SetBytes(updated, "model", toModel); err == nil {
			updated = next
		}
	}
	if replaceResponseModel {
		if next, err := sjson.SetBytes(updated, "response.model", toModel); err == nil {
			updated = next
		}
	}
	return updated
}

func populateOpenAIUsageFromResponseJSON(body []byte, usage *OpenAIUsage) {
	if usage == nil || len(body) == 0 {
		return
	}
	values := gjson.GetManyBytes(
		body,
		"usage.input_tokens",
		"usage.output_tokens",
		"usage.input_tokens_details.cached_tokens",
	)
	usage.InputTokens = int(values[0].Int())
	usage.OutputTokens = int(values[1].Int())
	usage.CacheReadInputTokens = int(values[2].Int())
}

func getOpenAIGroupIDFromContext(c *gin.Context) int64 {
	if c == nil {
		return 0
	}
	value, exists := c.Get("api_key")
	if !exists {
		return 0
	}
	apiKey, ok := value.(*APIKey)
	if !ok || apiKey == nil || apiKey.GroupID == nil {
		return 0
	}
	return *apiKey.GroupID
}

// SelectAccountByPreviousResponseID 按 previous_response_id 命中账号粘连。
// 未命中或账号不可用时返回 (nil, nil)，由调用方继续走常规调度。
func (s *OpenAIGatewayService) SelectAccountByPreviousResponseID(
	ctx context.Context,
	groupID *int64,
	apiKeyID int64,
	previousResponseID string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requireCompact bool,
) (*AccountSelectionResult, error) {
	return s.selectAccountByPreviousResponseIDForCapability(ctx, groupID, apiKeyID, previousResponseID, requestedModel, excludedIDs, "", OpenAIUpstreamTransportAny, requireCompact)
}

func (s *OpenAIGatewayService) selectAccountByPreviousResponseIDForCapability(
	ctx context.Context,
	groupID *int64,
	apiKeyID int64,
	previousResponseID string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredCapability OpenAIEndpointCapability,
	requiredTransport OpenAIUpstreamTransport,
	requireCompact bool,
) (*AccountSelectionResult, error) {
	if s == nil {
		return nil, nil
	}
	responseID := strings.TrimSpace(previousResponseID)
	if responseID == "" {
		return nil, nil
	}
	group := derefGroupID(groupID)
	responseConnID := ""
	responseConnHit := false
	responseConnInPool := false
	responseConnProfile := openAIWSConnProfile("")
	responseConnAgeMS := int64(0)
	responseConnIdleMS := int64(0)
	responseConnLeaseCount := int64(0)
	logDiag := func(reason string, action string, account *Account, accountID int64, deletedBinding bool, selectionHit bool) {
		accountType := ""
		if account != nil {
			accountType = account.Type
			if accountID <= 0 {
				accountID = account.ID
			}
		}
		s.logOpenAIWSPreviousResponseStickyDiag(openAIWSPreviousResponseStickyDiagLog{
			GroupID:                derefGroupID(groupID),
			APIKeyID:               apiKeyID,
			PreviousResponseID:     responseID,
			RequestedModel:         requestedModel,
			RequiredTransport:      requiredTransport,
			RequiredCapability:     requiredCapability,
			AccountID:              accountID,
			AccountType:            accountType,
			Reason:                 reason,
			Action:                 action,
			DeletedBinding:         deletedBinding,
			SelectionHit:           selectionHit,
			ExcludedCount:          len(excludedIDs),
			ResponseConnID:         responseConnID,
			ResponseConnHit:        responseConnHit,
			ResponseConnInPool:     responseConnInPool,
			ResponseConnProfile:    responseConnProfile,
			ResponseConnAgeMS:      responseConnAgeMS,
			ResponseConnIdleMS:     responseConnIdleMS,
			ResponseConnLeaseCount: responseConnLeaseCount,
		})
	}
	store := s.getOpenAIWSStateStore()
	if store == nil {
		logDiag("state_store_missing", "load_balance_fallback", nil, 0, false, false)
		return nil, nil
	}

	accountID, err := store.GetResponseAccount(ctx, group, apiKeyID, responseID)
	if err != nil {
		logDiag("binding_error", "load_balance_fallback", nil, 0, false, false)
		return nil, nil
	}
	responseConnID, responseConnHit = store.GetResponseConn(group, apiKeyID, responseID)
	if responseConnHit && accountID > 0 {
		if pool := s.getOpenAIWSConnPool(); pool != nil {
			snapshot := pool.ConnSnapshot(accountID, responseConnID)
			responseConnInPool = snapshot.Exists
			responseConnProfile = snapshot.Profile
			responseConnAgeMS = snapshot.Age.Milliseconds()
			responseConnIdleMS = snapshot.Idle.Milliseconds()
			responseConnLeaseCount = snapshot.LeaseCount
		}
	}
	if accountID <= 0 {
		logDiag("binding_miss", "load_balance_fallback", nil, 0, false, false)
		return nil, nil
	}
	if excludedIDs != nil {
		if _, excluded := excludedIDs[accountID]; excluded {
			account, _ := s.getSchedulableAccount(ctx, accountID)
			logDiag("excluded", "load_balance_fallback", account, accountID, false, false)
			return nil, nil
		}
	}

	account, err := s.getSchedulableAccount(ctx, accountID)
	if err != nil || account == nil {
		logDiag("account_not_found", "delete_binding", account, accountID, true, false)
		_ = store.DeleteResponseAccount(ctx, group, apiKeyID, responseID)
		return nil, nil
	}
	// 非 WSv2 场景默认不使用 previous_response_id 粘连，避免伪造 HTTP continuation。
	// 仅当显式 durable HTTP lane 打开时，才允许 OAuth 账号在 HTTP SSE 上复用 sticky account。
	if transport := s.getOpenAIWSProtocolResolver().Resolve(account).Transport; transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
		if transport != OpenAIUpstreamTransportHTTPSSE || !s.allowOpenAIDurableHTTPSticky(account) {
			logDiag("transport_incompatible", "load_balance_fallback", account, accountID, false, false)
			return nil, nil
		}
	}
	stickyWaitTimeout := s.openAIStickyWaitTimeout(ctx)
	if shouldClearOpenAIStickyAccount(account, requestedModel, "", stickyWaitTimeout) || !isOpenAIStickyCandidateCompatible(ctx, s.settingService, account, requestedModel, requireCompact, "", false, false) {
		logDiag("sticky_clear_policy", "delete_binding", account, accountID, true, false)
		_ = store.DeleteResponseAccount(ctx, group, apiKeyID, responseID)
		return nil, nil
	}
	account = s.recheckSelectedStickyOpenAIAccountFromDB(ctx, account, requestedModel, requireCompact, "", "", false, false)
	if account == nil {
		logDiag("recheck_failed", "delete_binding", nil, accountID, true, false)
		_ = store.DeleteResponseAccount(ctx, group, apiKeyID, responseID)
		return nil, nil
	}
	if !accountSupportsOpenAICapabilities(account, requiredCapability, "") {
		logDiag("capability_mismatch", "load_balance_fallback", account, accountID, false, false)
		return nil, nil
	}
	// Quota auto-pause must also gate the previous_response_id sticky path; otherwise an
	// account over its 5h/7d threshold keeps serving the same response chain even though
	// normal scheduling skips it. Pause is transient, so fall through to normal scheduling
	// without deleting the binding (the window may reset before the next turn).
	if paused, _ := shouldAutoPauseOpenAIAccountByQuota(ctx, account); paused {
		logDiag("quota_paused", "load_balance_fallback", account, accountID, false, false)
		return nil, nil
	}
	if s.schedulerSnapshot != nil && s.accountRepo != nil {
		latest, latestErr := s.accountRepo.GetByID(ctx, account.ID)
		if latestErr != nil || latest == nil {
			logDiag("latest_missing", "delete_binding", account, accountID, true, false)
			_ = store.DeleteResponseAccount(ctx, group, apiKeyID, responseID)
			return nil, nil
		}
		if shouldClearStickySession(latest, requestedModel) || !latest.IsOpenAI() || !latest.IsSchedulable() {
			logDiag("latest_unschedulable", "delete_binding", latest, accountID, true, false)
			_ = store.DeleteResponseAccount(ctx, group, apiKeyID, responseID)
			return nil, nil
		}
		if requestedModel != "" && !latest.IsModelSupported(requestedModel) {
			logDiag("model_unsupported", "load_balance_fallback", latest, accountID, false, false)
			return nil, nil
		}
		if !latest.SupportsOpenAIEndpointCapability(requiredCapability) {
			logDiag("latest_capability_mismatch", "load_balance_fallback", latest, accountID, false, false)
			return nil, nil
		}
		if paused, _ := shouldAutoPauseOpenAIAccountByQuota(ctx, latest); paused {
			logDiag("latest_quota_paused", "load_balance_fallback", latest, accountID, false, false)
			return nil, nil
		}
		if s.isOpenAIAccountRuntimeBlocked(latest) {
			logDiag("runtime_blocked", "delete_binding", latest, accountID, true, false)
			_ = store.DeleteResponseAccount(ctx, group, apiKeyID, responseID)
			return nil, nil
		}
		account = latest
	}
	if requireCompact && openAICompactSupportTier(account) == 0 {
		logDiag("compact_unsupported", "delete_binding", account, accountID, true, false)
		_ = store.DeleteResponseAccount(ctx, group, apiKeyID, responseID)
		return nil, nil
	}

	result, acquireErr := s.tryAcquireAccountSlot(ctx, accountID, groupID, account.Concurrency)
	if acquireErr == nil && result.Acquired {
		logOpenAIWSBindResponseAccountWarn(
			group,
			accountID,
			responseID,
			store.BindResponseAccount(ctx, group, apiKeyID, responseID, accountID, s.openAIWSResponseStickyTTL()),
		)
		logDiag("slot_acquired", "sticky_previous_selected", account, accountID, false, true)
		return s.newAcquiredSelectionResult(ctx, account, result.ReleaseFunc)
	}

	cfg := s.schedulingConfig()
	if waitPlan := buildOpenAIAccountWaitPlan(account, requestedModel, "", account.Concurrency, stickyWaitTimeout, cfg.StickySessionMaxWaiting); waitPlan != nil {
		logDiag("wait_plan", "sticky_previous_wait", account, accountID, false, true)
		return s.newSelectionResult(ctx, account, false, nil, waitPlan)
	}
	if s.concurrencyService != nil {
		logDiag("slot_busy_wait", "sticky_previous_wait", account, accountID, false, true)
		return s.newSelectionResult(ctx, account, false, nil, &AccountWaitPlan{
			AccountID:      accountID,
			MaxConcurrency: account.Concurrency,
			Timeout:        stickyWaitTimeout,
			MaxWaiting:     cfg.StickySessionMaxWaiting,
		})
	}
	logDiag("slot_unavailable", "load_balance_fallback", account, accountID, false, false)
	return nil, nil
}

func classifyOpenAIWSAcquireError(err error) string {
	if err == nil {
		return "acquire_conn"
	}
	var dialErr *openAIWSDialError
	if errors.As(err, &dialErr) {
		switch dialErr.StatusCode {
		case 426:
			return "upgrade_required"
		case 401, 403:
			return "auth_failed"
		case 429:
			return "upstream_rate_limited"
		}
		if dialErr.StatusCode >= 500 {
			return "upstream_5xx"
		}
		return "dial_failed"
	}
	if errors.Is(err, errOpenAIWSConnQueueFull) {
		return "conn_queue_full"
	}
	if errors.Is(err, errOpenAIWSPreferredConnUnavailable) {
		return "preferred_conn_unavailable"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "acquire_timeout"
	}
	return "acquire_conn"
}

func isOpenAIWSRateLimitError(codeRaw, errTypeRaw, msgRaw string) bool {
	code := strings.ToLower(strings.TrimSpace(codeRaw))
	errType := strings.ToLower(strings.TrimSpace(errTypeRaw))
	msg := strings.ToLower(strings.TrimSpace(msgRaw))

	if strings.Contains(errType, "rate_limit") || strings.Contains(errType, "usage_limit") {
		return true
	}
	if strings.Contains(code, "rate_limit") ||
		strings.Contains(code, "usage_limit") ||
		strings.Contains(code, "insufficient_quota") ||
		strings.Contains(code, "billing_hard_limit") {
		return true
	}
	if strings.Contains(msg, "usage limit") && strings.Contains(msg, "reached") {
		return true
	}
	if strings.Contains(msg, "upgrade to plus") && strings.Contains(msg, "usage limit") {
		return true
	}
	if strings.Contains(msg, "rate limit") && (strings.Contains(msg, "reached") || strings.Contains(msg, "exceeded")) {
		return true
	}
	return false
}

func classifyOpenAIWSSoftRateLimitAdvisory(message []byte) (string, bool) {
	if len(message) == 0 {
		return "", false
	}
	eventType := strings.TrimSpace(gjson.GetBytes(message, "type").String())
	if eventType == "codex.rate_limits" {
		if !gjson.GetBytes(message, "rate_limits").IsObject() {
			return "", false
		}
		allowed := gjson.GetBytes(message, "rate_limits.allowed")
		limitReached := gjson.GetBytes(message, "rate_limits.limit_reached")
		if !allowed.Exists() || !allowed.Bool() || !limitReached.Exists() || limitReached.Bool() {
			return "", false
		}
		limitID := strings.TrimSpace(gjson.GetBytes(message, "metered_limit_name").String())
		if limitID == "" {
			limitID = strings.TrimSpace(gjson.GetBytes(message, "limit_name").String())
		}
		if limitID == "" {
			limitID = openAIWSSoftRateLimitAdvisoryDefaultLimit
		}
		normalizedLimitID := strings.ReplaceAll(strings.ToLower(limitID), "-", "_")
		if normalizedLimitID != openAIWSSoftRateLimitAdvisoryDefaultLimit {
			return "", false
		}
		if gjson.GetBytes(message, "credits.has_credits").Bool() ||
			gjson.GetBytes(message, "credits.unlimited").Bool() {
			return "", false
		}
		if !openAIWSRateLimitWindowApproaching(message, "rate_limits.primary.used_percent") &&
			!openAIWSRateLimitWindowApproaching(message, "rate_limits.secondary.used_percent") {
			return "", false
		}
		return openAIWSSoftRateLimitAdvisoryMessage, true
	}

	return "", false
}

func openAIWSRateLimitWindowApproaching(message []byte, path string) bool {
	value := gjson.GetBytes(message, path)
	return value.Exists() && value.Float() >= openAIWSSoftRateLimitAdvisoryThreshold
}

func (s *OpenAIGatewayService) persistOpenAIWSRateLimitSignal(ctx context.Context, account *Account, headers http.Header, responseBody []byte, codeRaw, errTypeRaw, msgRaw string) {
	if s == nil || s.rateLimitService == nil || account == nil || account.Platform != PlatformOpenAI {
		return
	}
	if !isOpenAIWSRateLimitError(codeRaw, errTypeRaw, msgRaw) {
		return
	}
	s.handleOpenAIAccountUpstreamError(ctx, account, http.StatusTooManyRequests, headers, responseBody)
}

func (s *OpenAIGatewayService) persistOpenAIWSDialFailureSignal(ctx context.Context, account *Account, err error) {
	if s == nil || s.rateLimitService == nil || account == nil || account.Platform != PlatformOpenAI || err == nil {
		return
	}

	var dialErr *openAIWSDialError
	if !errors.As(err, &dialErr) || dialErr == nil {
		return
	}

	responseBody := dialErr.ResponseBody
	if len(responseBody) == 0 {
		responseBody = openAIWSHandshakeBodyFromError(dialErr.Err)
	}
	switch dialErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		s.handleOpenAIAccountUpstreamError(ctx, account, dialErr.StatusCode, dialErr.ResponseHeaders, responseBody)
	case http.StatusTooManyRequests:
		s.persistOpenAIWSRateLimitSignal(ctx, account, dialErr.ResponseHeaders, responseBody, "rate_limit_exceeded", "rate_limit_error", strings.TrimSpace(err.Error()))
	}
}

// isOpenAIWSModelUnavailableEvent 精确判定 WS error event 是否为“模型不可用/不存在”。
// 仅匹配明确的错误码或无歧义短语，避免把普通 invalid_request（恰好提及 model）误判为模型不可用。
// 入参均已 lower+trim。
func isOpenAIWSModelUnavailableEvent(code, errType, msg string) bool {
	switch code {
	case "model_not_found", "unknown_model", "unsupported_model", "model_not_supported":
		return true
	}
	// type 仅在配合明确短语时才算（invalid_request_error 太宽泛，不能单独凭 type 判定）。
	explicitPhrases := []string{
		"model not found",
		"unknown model",
		"unsupported model",
		"model is not supported",
		"invalid model",
		"unrecognized model",
		"no such model",
	}
	for _, phrase := range explicitPhrases {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	_ = errType
	return false
}

func classifyOpenAIWSErrorEventFromRaw(codeRaw, errTypeRaw, msgRaw string) (string, bool) {
	code := strings.ToLower(strings.TrimSpace(codeRaw))
	errType := strings.ToLower(strings.TrimSpace(errTypeRaw))
	msg := strings.ToLower(strings.TrimSpace(msgRaw))

	switch code {
	case "upgrade_required":
		return "upgrade_required", true
	case "websocket_not_supported", "websocket_unsupported":
		return "ws_unsupported", true
	case "websocket_connection_limit_reached":
		return "ws_connection_limit_reached", true
	case "invalid_encrypted_content":
		return "invalid_encrypted_content", true
	case "previous_response_not_found":
		return "previous_response_not_found", true
	case "conn_mismatch", "connection_mismatch":
		return "conn_mismatch", true
	case "account_mismatch":
		return "account_mismatch", true
	}
	if isOpenAIWSRateLimitError(codeRaw, errTypeRaw, msgRaw) {
		return "upstream_rate_limited", true
	}
	if isOpenAIWSModelUnavailableEvent(code, errType, msg) {
		return "model_unavailable", true
	}
	if strings.Contains(msg, "upgrade required") || strings.Contains(msg, "status 426") {
		return "upgrade_required", true
	}
	if strings.Contains(errType, "upgrade") {
		return "upgrade_required", true
	}
	if strings.Contains(msg, "websocket") && strings.Contains(msg, "unsupported") {
		return "ws_unsupported", true
	}
	if strings.Contains(msg, "connection limit") && strings.Contains(msg, "websocket") {
		return "ws_connection_limit_reached", true
	}
	if strings.Contains(msg, "invalid_encrypted_content") ||
		(strings.Contains(msg, "encrypted content") && strings.Contains(msg, "could not be verified")) {
		return "invalid_encrypted_content", true
	}
	if strings.Contains(msg, "previous_response_not_found") ||
		(strings.Contains(msg, "previous response") && strings.Contains(msg, "not found")) {
		return "previous_response_not_found", true
	}
	if strings.Contains(msg, "conn_mismatch") ||
		(strings.Contains(msg, "connection") && strings.Contains(msg, "mismatch")) {
		return "conn_mismatch", true
	}
	if strings.Contains(msg, "account_mismatch") ||
		(strings.Contains(msg, "account") && strings.Contains(msg, "mismatch")) {
		return "account_mismatch", true
	}
	if strings.Contains(errType, "invalid_request") || strings.Contains(code, "invalid_request") {
		if fallbackReason := classifyOpenAICodexCompatFallbackMessage(msg); fallbackReason != "" {
			return fallbackReason, true
		}
	}
	if strings.Contains(errType, "server_error") || strings.Contains(code, "server_error") {
		return "upstream_error_event", true
	}
	return "event_error", false
}

func classifyOpenAIWSErrorEvent(message []byte) (string, bool) {
	if len(message) == 0 {
		return "event_error", false
	}
	return classifyOpenAIWSErrorEventFromRaw(parseOpenAIWSErrorEventFields(message))
}

func openAIWSErrorHTTPStatusFromRaw(codeRaw, errTypeRaw string) int {
	code := strings.ToLower(strings.TrimSpace(codeRaw))
	errType := strings.ToLower(strings.TrimSpace(errTypeRaw))
	switch {
	case strings.Contains(errType, "invalid_request"),
		strings.Contains(code, "invalid_request"),
		strings.Contains(code, "bad_request"),
		code == "invalid_encrypted_content",
		code == "previous_response_not_found":
		return http.StatusBadRequest
	case strings.Contains(errType, "authentication"),
		strings.Contains(code, "invalid_api_key"),
		strings.Contains(code, "unauthorized"):
		return http.StatusUnauthorized
	case strings.Contains(errType, "permission"),
		strings.Contains(code, "forbidden"):
		return http.StatusForbidden
	case isOpenAIWSRateLimitError(codeRaw, errTypeRaw, ""):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadGateway
	}
}

func openAIWSErrorHTTPStatus(message []byte) int {
	if len(message) == 0 {
		return http.StatusBadGateway
	}
	codeRaw, errTypeRaw, _ := parseOpenAIWSErrorEventFields(message)
	return openAIWSErrorHTTPStatusFromRaw(codeRaw, errTypeRaw)
}

func (s *OpenAIGatewayService) openAIWSFallbackCooldown() time.Duration {
	if s == nil || s.cfg == nil {
		return 30 * time.Second
	}
	seconds := s.cfg.Gateway.OpenAIWS.FallbackCooldownSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func (s *OpenAIGatewayService) isOpenAIWSFallbackCooling(accountID int64) bool {
	if s == nil || accountID <= 0 {
		return false
	}
	cooldown := s.openAIWSFallbackCooldown()
	if cooldown <= 0 {
		return false
	}
	rawUntil, ok := s.openaiWSFallbackUntil.Load(accountID)
	if !ok || rawUntil == nil {
		return false
	}
	until, ok := rawUntil.(time.Time)
	if !ok || until.IsZero() {
		s.openaiWSFallbackUntil.Delete(accountID)
		return false
	}
	if time.Now().Before(until) {
		return true
	}
	s.openaiWSFallbackUntil.Delete(accountID)
	return false
}

func (s *OpenAIGatewayService) markOpenAIWSFallbackCooling(accountID int64, _ string) {
	if s == nil || accountID <= 0 {
		return
	}
	cooldown := s.openAIWSFallbackCooldown()
	if cooldown <= 0 {
		return
	}
	s.openaiWSFallbackUntil.Store(accountID, time.Now().Add(cooldown))
}

func (s *OpenAIGatewayService) clearOpenAIWSFallbackCooling(accountID int64) {
	if s == nil || accountID <= 0 {
		return
	}
	s.openaiWSFallbackUntil.Delete(accountID)
}
