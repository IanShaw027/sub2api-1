package service

import (
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

const openAIContextWindowClientMessage = "Your input exceeds the context window of this model. Please adjust your input and try again."

const openAIUpstreamAccessUnavailableClientMessage = "Upstream access is temporarily unavailable, please retry later"

// ClientVisibleUpstreamError is a safe, user-facing classification of an
// upstream failure. Prefer this over opaque transport fallbacks when the
// failure is a known business/protocol condition (context overflow, quota,
// deactivated workspace, etc.).
type ClientVisibleUpstreamError struct {
	StatusCode int
	ErrorType  string
	Message    string
}

func openAIBillingLimitClientMessage() string {
	return "Upstream billing or quota limit reached, please retry later"
}

func isOpenAIBillingLimitError(statusCode int, upstreamMsg, codeRaw string) bool {
	lowerMsg := strings.ToLower(strings.TrimSpace(upstreamMsg))
	lowerCode := strings.ToLower(strings.TrimSpace(codeRaw))

	if strings.Contains(lowerCode, "billing_hard_limit") ||
		strings.Contains(lowerCode, "insufficient_quota") {
		return true
	}
	if strings.Contains(lowerMsg, "usage limit") && strings.Contains(lowerMsg, "reached") {
		return true
	}
	if strings.Contains(lowerMsg, "insufficient quota") {
		return true
	}
	if strings.Contains(lowerMsg, "billing hard limit") {
		return true
	}
	if strings.Contains(lowerMsg, "upgrade to plus") && strings.Contains(lowerMsg, "usage limit") {
		return true
	}
	return statusCode == http.StatusPaymentRequired
}

// ClassifyClientVisibleUpstreamError maps a known business/protocol upstream
// failure into a client-safe status/type/message. Returns ok=false when the
// failure should stay on the generic transport / mapUpstreamError path.
func ClassifyClientVisibleUpstreamError(message string, body []byte) (ClientVisibleUpstreamError, bool) {
	msg := sanitizeUpstreamErrorMessage(strings.TrimSpace(message))
	if msg == "" {
		msg = sanitizeUpstreamErrorMessage(ExtractUpstreamErrorMessage(body))
	}
	code := strings.TrimSpace(extractUpstreamErrorCode(body))
	if code == "" {
		code = strings.TrimSpace(gjson.GetBytes(body, "detail.code").String())
	}
	// Restrict pattern matching to structured error fields. Scanning the raw
	// response lets echoed prompts/model output accidentally trigger a business
	// classification (for example "your request was blocked").
	errorType := strings.TrimSpace(firstNonEmpty(
		gjson.GetBytes(body, "error.type").String(),
		gjson.GetBytes(body, "response.error.type").String(),
		gjson.GetBytes(body, "type").String(),
	))
	combinedLower := strings.ToLower(strings.TrimSpace(msg + " " + code + " " + errorType))

	// Billing / quota first: never mislabel usage-limit as context overflow.
	if isOpenAIBillingLimitError(0, msg, code) ||
		(strings.Contains(combinedLower, "usage limit") && strings.Contains(combinedLower, "reached")) ||
		strings.Contains(combinedLower, "insufficient_quota") ||
		strings.Contains(combinedLower, "billing_hard_limit") {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusTooManyRequests,
			ErrorType:  "rate_limit_error",
			Message:    openAIBillingLimitClientMessage(),
		}, true
	}

	if isOpenAIUpstreamAccessStateError(combinedLower, code) {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusBadGateway,
			ErrorType:  "upstream_error",
			Message:    openAIUpstreamAccessUnavailableClientMessage,
		}, true
	}

	if strings.Contains(combinedLower, "upstream compact response failed") {
		if detail := sanitizeUpstreamErrorMessage(ExtractUpstreamErrorMessage(body)); detail != "" {
			return ClientVisibleUpstreamError{
				StatusCode: http.StatusBadGateway,
				ErrorType:  "upstream_error",
				Message:    detail,
			}, true
		}
		if msg != "" && !strings.EqualFold(msg, "Upstream compact response failed") {
			return ClientVisibleUpstreamError{
				StatusCode: http.StatusBadGateway,
				ErrorType:  "upstream_error",
				Message:    msg,
			}, true
		}
	}

	if isOpenAIContextWindowError(msg, body) ||
		strings.Contains(combinedLower, "openai upstream sse context window exceeded") {
		clientMsg := msg
		if clientMsg == "" ||
			strings.Contains(strings.ToLower(clientMsg), "openai upstream sse context window") ||
			strings.EqualFold(clientMsg, "Upstream transport error") ||
			strings.EqualFold(clientMsg, "Upstream transport request failed") {
			clientMsg = openAIContextWindowClientMessage
		}
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusBadRequest,
			ErrorType:  "invalid_request_error",
			Message:    clientMsg,
		}, true
	}

	// Grok media field validation (aspect_ratio/resolution/…) must stay visible
	// even if a failover path exhausted before writing the upstream body.
	if isGrokClientParameterValidationErrorText(combinedLower) {
		clientMsg := resolveClientVisibleUpstreamMessage(msg, body, "Invalid request parameter")
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusBadRequest,
			ErrorType:  "invalid_request_error",
			Message:    clientMsg,
		}, true
	}

	// Recognizable business/protocol conditions that should reach the client
	// instead of opaque transport generics. Credential auth failures, CF/HTML
	// gateway pages, and account-access redactions stay on ok=false / earlier
	// sanitized branches above.
	if isClientVisibleMultiAgentChatCompletionsError(combinedLower) {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusBadRequest,
			ErrorType:  "invalid_request_error",
			Message:    resolveClientVisibleUpstreamMessage(msg, body, "Multi Agent requests are not allowed on chat completions"),
		}, true
	}

	if isClientVisibleGroupDeletedError(combinedLower, code) {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusForbidden,
			ErrorType:  "permission_error",
			Message:    resolveClientVisibleUpstreamMessage(msg, body, "API Key 所属分组已删除"),
		}, true
	}

	if strings.Contains(combinedLower, "image generation is not enabled for this group") {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusForbidden,
			ErrorType:  "permission_error",
			Message:    resolveClientVisibleUpstreamMessage(msg, body, "Image generation is not enabled for this group"),
		}, true
	}

	if isClientVisibleRequestBlockedError(combinedLower) {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusForbidden,
			ErrorType:  "invalid_request_error",
			Message:    resolveClientVisibleUpstreamMessage(msg, body, "Your request was blocked"),
		}, true
	}

	if strings.Contains(combinedLower, "no account is available, please try again later") {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusServiceUnavailable,
			ErrorType:  "upstream_error",
			Message:    resolveClientVisibleUpstreamMessage(msg, body, "no account is available, please try again later"),
		}, true
	}

	if isClientVisibleGrokCreditsError(combinedLower) {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusPaymentRequired,
			ErrorType:  "upstream_error",
			Message:    resolveClientVisibleUpstreamMessage(msg, body, "You have run out of credits or need a Grok subscription"),
		}, true
	}

	if isClientVisibleModelNotFoundError(combinedLower, code) {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusNotFound,
			ErrorType:  "model_not_found",
			Message:    resolveClientVisibleUpstreamMessage(msg, body, "model not found"),
		}, true
	}

	if isClientVisibleImageModelResponsesEndpointError(combinedLower) {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusBadRequest,
			ErrorType:  "invalid_request_error",
			Message:    resolveClientVisibleUpstreamMessage(msg, body, "image model is not available on the Responses endpoint; use /v1/images/generations instead"),
		}, true
	}

	return ClientVisibleUpstreamError{}, false
}

func resolveClientVisibleUpstreamMessage(msg string, body []byte, fallback string) string {
	if !isOpaqueUpstreamClientMessage(msg) {
		return msg
	}
	if detail := sanitizeUpstreamErrorMessage(ExtractUpstreamErrorMessage(body)); detail != "" {
		return detail
	}
	if text, _, _ := grokUpstreamErrorCorpus(0, body); strings.TrimSpace(text) != "" {
		if cleaned := sanitizeUpstreamErrorMessage(strings.TrimSpace(text)); cleaned != "" {
			return cleaned
		}
	}
	if cleaned := sanitizeUpstreamErrorMessage(strings.TrimSpace(string(body))); cleaned != "" && !looksLikeHTMLUpstreamErrorBody(cleaned) {
		return cleaned
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "Upstream request failed"
}

func isOpaqueUpstreamClientMessage(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	return strings.EqualFold(trimmed, "Upstream request failed") ||
		strings.EqualFold(trimmed, "Upstream transport error") ||
		strings.EqualFold(trimmed, "Upstream transport request failed") ||
		strings.HasPrefix(lower, "xai upstream returned status")
}

func looksLikeHTMLUpstreamErrorBody(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<!doctype html") ||
		strings.Contains(lower, "error code 1010") ||
		strings.Contains(lower, "cf-error") ||
		strings.Contains(lower, "<title>502") ||
		strings.Contains(lower, "<title>504")
}

func isClientVisibleMultiAgentChatCompletionsError(combinedLower string) bool {
	if !(strings.Contains(combinedLower, "multi agent") || strings.Contains(combinedLower, "multi_agent")) {
		return false
	}
	return strings.Contains(combinedLower, "not allowed") &&
		strings.Contains(combinedLower, "chat completions")
}

func isClientVisibleGroupDeletedError(combinedLower, code string) bool {
	if strings.EqualFold(strings.TrimSpace(code), "GROUP_DELETED") ||
		strings.Contains(combinedLower, "group_deleted") {
		return true
	}
	return strings.Contains(combinedLower, "api key 所属分组已删除") ||
		strings.Contains(combinedLower, "所属分组已删除")
}

func isClientVisibleRequestBlockedError(combinedLower string) bool {
	// Exact/short policy blocks only. Account suspensions are handled by the
	// access-state branch; credential auth failures must stay opaque.
	if strings.Contains(combinedLower, "account suspended") ||
		strings.Contains(combinedLower, "incorrect api key") ||
		strings.Contains(combinedLower, "invalid_api_key") ||
		strings.Contains(combinedLower, "invalid api key") {
		return false
	}
	if looksLikeHTMLUpstreamErrorBody(combinedLower) {
		return false
	}
	return strings.Contains(combinedLower, "your request was blocked")
}

func isClientVisibleGrokCreditsError(combinedLower string) bool {
	return strings.Contains(combinedLower, "run out of credits") ||
		strings.Contains(combinedLower, "need a grok subscription") ||
		strings.Contains(combinedLower, "need grok subscription") ||
		strings.Contains(combinedLower, "add credits at grok.com")
}

func isClientVisibleModelNotFoundError(combinedLower, code string) bool {
	lowerCode := strings.ToLower(strings.TrimSpace(code))
	if lowerCode == "model_not_found" ||
		lowerCode == "model_not_available" ||
		lowerCode == "unsupported_model" ||
		lowerCode == "invalid_model" {
		return true
	}
	if strings.Contains(combinedLower, "not supported by any configured account") {
		return true
	}
	return strings.Contains(combinedLower, "\"type\":\"model_not_found\"") ||
		strings.Contains(combinedLower, "\"code\":\"model_not_found\"") ||
		strings.Contains(combinedLower, "model_not_found")
}

func isClientVisibleImageModelResponsesEndpointError(combinedLower string) bool {
	if strings.Contains(combinedLower, "not available on the responses endpoint") {
		return true
	}
	return strings.Contains(combinedLower, "image model") &&
		strings.Contains(combinedLower, "/v1/images/generations")
}

// isOpenAIUpstreamAccessStateError identifies provider-side account/workspace
// state failures without exposing the provider's internal status terminology.
func isOpenAIUpstreamAccessStateError(combinedLower, code string) bool {
	if strings.EqualFold(code, "deactivated_workspace") ||
		strings.Contains(combinedLower, "deactivated_workspace") {
		return true
	}
	for _, phrase := range []string{
		"workspace is deactivated",
		"workspace deactivated",
		"workspace is disabled",
		"workspace disabled",
		"account is deactivated",
		"account deactivated",
		"account is disabled",
		"account disabled",
		"organization is deactivated",
		"organization deactivated",
		"organization is disabled",
		"organization disabled",
		"account is suspended",
		"account suspended",
		"workspace is suspended",
		"workspace suspended",
	} {
		if strings.Contains(combinedLower, phrase) {
			return true
		}
	}
	return false
}

// ClassifyClientVisibleUpstreamErrorFromErr unwraps known typed upstream
// failures before string matching.
func ClassifyClientVisibleUpstreamErrorFromErr(err error) (ClientVisibleUpstreamError, bool) {
	if err == nil {
		return ClientVisibleUpstreamError{}, false
	}
	return ClassifyClientVisibleUpstreamError(err.Error(), nil)
}

// PreferClientVisibleUpstreamError prefers a concrete business classification
// over a generic transport fallback when both are available.
func PreferClientVisibleUpstreamError(err error, fallbackType, fallbackMessage, fallbackDetail string) (ClientVisibleUpstreamError, bool) {
	if vis, ok := ClassifyClientVisibleUpstreamErrorFromErr(err); ok {
		return vis, true
	}
	if vis, ok := ClassifyClientVisibleUpstreamError(fallbackMessage, []byte(fallbackDetail)); ok {
		return vis, true
	}
	if vis, ok := ClassifyClientVisibleUpstreamError(fallbackDetail, nil); ok {
		return vis, true
	}
	return ClientVisibleUpstreamError{}, false
}
