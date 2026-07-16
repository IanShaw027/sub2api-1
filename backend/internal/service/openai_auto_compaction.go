package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

// buildOpenAIContextCompactionTriggerBody prepares the client-compatible
// remote-compaction request used only after an upstream context overflow. The
// original input is retained verbatim and the trigger is appended as a final
// item, so the upstream owns the actual summarization semantics.
func buildOpenAIContextCompactionTriggerBody(body []byte) ([]byte, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("invalid OpenAI request body")
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() || len(input.Array()) == 0 {
		return nil, fmt.Errorf("input must be a non-empty array")
	}
	if containsOpenAIContextCompactionTrigger(input) {
		return nil, fmt.Errorf("request is not safe for automatic compaction")
	}
	for _, item := range input.Array() {
		if !openAIContextCompactionInputItemSafe(item) {
			return nil, fmt.Errorf("request contains unsupported compaction input item")
		}
	}

	trigger := map[string]any{"type": "compaction_trigger"}
	triggerJSON, err := json.Marshal(trigger)
	if err != nil {
		return nil, err
	}
	nextInput := make([]json.RawMessage, 0, len(input.Array())+1)
	for _, item := range input.Array() {
		nextInput = append(nextInput, json.RawMessage(item.Raw))
	}
	nextInput = append(nextInput, triggerJSON)
	encodedInput, err := json.Marshal(nextInput)
	if err != nil {
		return nil, fmt.Errorf("marshal compaction input: %w", err)
	}
	updated, err := sjson.SetRawBytes(body, "input", encodedInput)
	if err != nil {
		return nil, fmt.Errorf("set compaction input: %w", err)
	}
	// The compact endpoint is unary upstream even when the client consumes SSE.
	updated, err = sjson.DeleteBytes(updated, "store")
	if err != nil {
		return nil, fmt.Errorf("remove compact store field: %w", err)
	}
	return sjson.DeleteBytes(updated, "previous_response_id")
}

func isOpenAIContextRemoteCompactionV2Request(c *gin.Context, _ []byte) bool {
	if c == nil || c.Request == nil {
		return false
	}
	for _, header := range c.Request.Header.Values("x-codex-beta-features") {
		for _, feature := range strings.Split(header, ",") {
			if strings.EqualFold(strings.TrimSpace(feature), "remote_compaction_v2") {
				return true
			}
		}
	}
	return false
}

func isOpenAIContextCompactionStatus(status int) bool {
	return status == http.StatusBadRequest
}

func containsOpenAIContextCompactionTrigger(input gjson.Result) bool {
	for _, item := range input.Array() {
		if strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "compaction_trigger") {
			return true
		}
	}
	return false
}

func openAIContextCompactionInputItemSafe(item gjson.Result) bool {
	if !item.IsObject() {
		return false
	}
	typeName := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
	if strings.Contains(typeName, "image") || strings.Contains(typeName, "file") {
		return false
	}
	content := item.Get("content")
	if content.IsArray() {
		for _, part := range content.Array() {
			partType := strings.ToLower(strings.TrimSpace(part.Get("type").String()))
			if strings.Contains(partType, "image") || strings.Contains(partType, "file") {
				return false
			}
		}
	}
	return true
}

type openAIContextCompactionSSEError struct {
	payload []byte
	message string
}

func (e *openAIContextCompactionSSEError) Error() string {
	if e == nil || strings.TrimSpace(e.message) == "" {
		return "OpenAI upstream SSE context window exceeded"
	}
	return e.message
}

func newOpenAIContextCompactionSSEError(payload []byte, message string) error {
	if !isOpenAIContextWindowError(message, payload) {
		return nil
	}
	return &openAIContextCompactionSSEError{
		payload: append([]byte(nil), payload...),
		message: sanitizeUpstreamErrorMessage(strings.TrimSpace(message)),
	}
}

func asOpenAIContextCompactionSSEError(err error) (*openAIContextCompactionSSEError, bool) {
	var target *openAIContextCompactionSSEError
	if !errors.As(err, &target) || target == nil {
		return nil, false
	}
	return target, true
}

func (s *OpenAIGatewayService) retryOpenAIContextCompactionSSE(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	originalBody []byte,
	reqModel string,
	clientStream bool,
	failure error,
) (*OpenAIForwardResult, error, bool) {
	overflow, ok := asOpenAIContextCompactionSSEError(failure)
	if !ok || isOpenAIResponsesCompactPath(c) ||
		!isOpenAIContextRemoteCompactionV2Request(c, originalBody) ||
		account == nil || !account.AllowsOpenAICompact() {
		return nil, nil, false
	}

	compactBody, triggerErr := buildOpenAIContextCompactionTriggerBody(originalBody)
	if triggerErr == nil {
		compactBody, _, triggerErr = normalizeOpenAICompactRequestBody(compactBody)
	}
	if triggerErr == nil {
		if account.Type == AccountTypeOAuth {
			compactBody, _, triggerErr = normalizeOpenAIPassthroughOAuthBody(compactBody, true)
			if triggerErr == nil {
				compactBody, _, triggerErr = ensureOpenAIPassthroughInstructions(c, reqModel, compactBody)
			}
		} else {
			compactBody, _, triggerErr = normalizeOpenAIPassthroughBaseBody(compactBody, true, shouldStripTopPForResponsesUpstream(account))
		}
	}
	if triggerErr != nil {
		logger.FromContext(ctx).Info("openai.context_overflow_remote_compaction_skipped",
			zap.Int64("account_id", account.ID),
			zap.String("reason", triggerErr.Error()),
			zap.String("transport", "http_sse"),
		)
		return nil, nil, false
	}

	compactUpstreamModel := resolveOpenAIAccountUpstreamModelForRequest(ctx, s.settingService, account, reqModel, true, s.openAICompactModel())
	if compactUpstreamModel != "" {
		compactBody = ReplaceModelInBody(compactBody, compactUpstreamModel)
	}
	setOpenAIResponsesUpstreamPathSuffixOverride(c, "/compact")
	setOpenAIFailoverRequestBody(c, compactBody)
	setOpsUpstreamRequestBody(c, compactBody)
	if clientStream {
		MarkOpenAICompactClientStream(c)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	logger.FromContext(ctx).Info("openai.context_overflow_remote_compaction_retry",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", reqModel),
		zap.String("compact_model", compactUpstreamModel),
		zap.String("upstream_code", extractUpstreamErrorCode(overflow.payload)),
		zap.String("transport", "http_sse"),
	)
	result, err := s.Forward(ctx, c, account, compactBody)
	return result, err, true
}
