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
	combinedLower := strings.ToLower(strings.TrimSpace(msg + " " + code + " " + string(body)))

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
