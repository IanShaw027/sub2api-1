package service

import (
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

const openAIContextWindowClientMessage = "Your input exceeds the context window of this model. Please adjust your input and try again."

// ClientVisibleUpstreamError is a safe, user-facing classification of an
// upstream failure. Prefer this over opaque transport fallbacks when the
// failure is a known business/protocol condition (context overflow, quota,
// deactivated workspace, etc.).
type ClientVisibleUpstreamError struct {
	StatusCode int
	ErrorType  string
	Message    string
}

// IsOpenAIContextWindowError reports whether the upstream failure is a
// context-window / context-length overflow. Exported for handler-layer
// fallback classification.
func IsOpenAIContextWindowError(upstreamMsg string, upstreamBody []byte) bool {
	return isOpenAIContextWindowError(upstreamMsg, upstreamBody)
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
	combinedLower := strings.ToLower(strings.TrimSpace(msg + " " + code + " " + string(body)))

	// Billing / quota first: never mislabel usage-limit as context overflow.
	// Do NOT pass a synthetic PaymentRequired status into isOpenAIBillingLimitError —
	// that helper treats status 402 alone as billing even when the message is unrelated.
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

	if strings.EqualFold(code, "deactivated_workspace") ||
		strings.Contains(combinedLower, "deactivated_workspace") ||
		strings.Contains(combinedLower, "workspace deactivated") ||
		strings.Contains(combinedLower, "workspace is deactivated") {
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusForbidden,
			ErrorType:  "upstream_error",
			Message:    "Upstream workspace is deactivated",
		}, true
	}

	// Compact path often surfaces only detail.code; if we already handled
	// deactivated_workspace above, remaining compact failures with a concrete
	// message should surface that message rather than a generic transport line.
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

	return ClientVisibleUpstreamError{}, false
}

// ClassifyClientVisibleUpstreamErrorFromErr unwraps known typed upstream
// failures (e.g. SSE context-window compaction errors) before string matching.
func ClassifyClientVisibleUpstreamErrorFromErr(err error) (ClientVisibleUpstreamError, bool) {
	if err == nil {
		return ClientVisibleUpstreamError{}, false
	}
	if overflow, ok := asOpenAIContextCompactionSSEError(err); ok {
		msg := strings.TrimSpace(overflow.message)
		if msg == "" {
			msg = overflow.Error()
		}
		if vis, ok := ClassifyClientVisibleUpstreamError(msg, overflow.payload); ok {
			return vis, true
		}
		return ClientVisibleUpstreamError{
			StatusCode: http.StatusBadRequest,
			ErrorType:  "invalid_request_error",
			Message:    openAIContextWindowClientMessage,
		}, true
	}
	return ClassifyClientVisibleUpstreamError(err.Error(), nil)
}

// PreferClientVisibleUpstreamError prefers a concrete business classification
// over a generic transport fallback when both are available.
func PreferClientVisibleUpstreamError(err error, fallbackType, fallbackMessage, fallbackDetail string) (ClientVisibleUpstreamError, bool) {
	if vis, ok := ClassifyClientVisibleUpstreamErrorFromErr(err); ok {
		return vis, true
	}
	// Ops may already hold a more specific detail than the generic client message.
	if vis, ok := ClassifyClientVisibleUpstreamError(fallbackMessage, []byte(fallbackDetail)); ok {
		return vis, true
	}
	if vis, ok := ClassifyClientVisibleUpstreamError(fallbackDetail, nil); ok {
		return vis, true
	}
	return ClientVisibleUpstreamError{}, false
}
