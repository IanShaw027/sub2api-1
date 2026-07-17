package handler

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

var upstreamHTTP2StreamErrorPattern = regexp.MustCompile(`stream error: stream ID \d+; ([A-Z_]+)(?:;|$)`)
var upstreamForwardSensitiveQueryParamPattern = regexp.MustCompile(`(?i)([?&](?:key|client_secret|access_token|refresh_token)=)[^&"\s]+`)
var upstreamForwardSensitiveBearerPattern = regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^,"\s]+`)

type upstreamForwardErrorDetail struct {
	StatusCode int
	ErrorType  string
	Message    string
	Detail     string
}

func classifyUpstreamForwardError(err error) upstreamForwardErrorDetail {
	if err == nil {
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_error",
			Message:    "Upstream request failed",
		}
	}

	// Business / protocol failures first so they never fall into the opaque
	// transport bucket (e.g. SSE context-window overflow).
	if vis, ok := service.ClassifyClientVisibleUpstreamErrorFromErr(err); ok {
		return upstreamForwardErrorDetail{
			StatusCode: vis.StatusCode,
			ErrorType:  vis.ErrorType,
			Message:    vis.Message,
			Detail:     sanitizeUpstreamForwardErrorDetail(strings.TrimSpace(err.Error())),
		}
	}

	detail := sanitizeUpstreamForwardErrorDetail(strings.TrimSpace(err.Error()))
	lowerDetail := strings.ToLower(detail)

	if errors.Is(err, context.DeadlineExceeded) {
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_timeout_error",
			Message:    "Upstream request timed out",
			Detail:     detail,
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_timeout_error",
			Message:    "Upstream request timed out",
			Detail:     detail,
		}
	}

	if code := parseUpstreamHTTP2StreamError(detail); code != "" {
		normalizedCode := strings.ToLower(code)
		normalizedCode = strings.TrimSuffix(normalizedCode, "_error")
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_http2_" + normalizedCode + "_error",
			Message:    "Upstream HTTP/2 peer reset the stream with " + code,
			Detail:     detail,
		}
	}

	switch {
	case strings.Contains(lowerDetail, "context canceled"):
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_canceled_error",
			Message:    "Upstream request was canceled",
			Detail:     detail,
		}
	case strings.Contains(lowerDetail, "no such host"),
		strings.Contains(lowerDetail, "server misbehaving"):
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_dns_error",
			Message:    "Upstream DNS lookup failed",
			Detail:     detail,
		}
	case strings.Contains(lowerDetail, "connection refused"),
		strings.Contains(lowerDetail, "dial tcp"):
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_connect_error",
			Message:    "Failed to connect to upstream service",
			Detail:     detail,
		}
	case strings.Contains(lowerDetail, "tls:"),
		strings.Contains(lowerDetail, "x509:"):
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_tls_error",
			Message:    "Upstream TLS handshake failed",
			Detail:     detail,
		}
	case strings.Contains(lowerDetail, "connection reset by peer"),
		strings.Contains(lowerDetail, "broken pipe"):
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_connection_reset_error",
			Message:    "Upstream connection was reset",
			Detail:     detail,
		}
	case detail == "EOF",
		strings.Contains(lowerDetail, "unexpected eof"),
		strings.Contains(lowerDetail, "client connection lost"):
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_connection_closed_error",
			Message:    "Upstream connection closed unexpectedly",
			Detail:     detail,
		}
	default:
		return upstreamForwardErrorDetail{
			StatusCode: 502,
			ErrorType:  "upstream_transport_error",
			Message:    "Upstream transport error",
			Detail:     detail,
		}
	}
}

func resolveUpstreamForwardErrorDetail(c *gin.Context, forwardErr error) upstreamForwardErrorDetail {
	// Prefer typed/string business classification from the forward error itself.
	// Context may still hold a stale generic "Upstream transport error" from an
	// earlier transport path, which must not mask context-window / compact causes.
	if vis, ok := service.ClassifyClientVisibleUpstreamErrorFromErr(forwardErr); ok {
		detail := sanitizeUpstreamForwardErrorDetail(strings.TrimSpace(errString(forwardErr)))
		return upstreamForwardErrorDetail{
			StatusCode: vis.StatusCode,
			ErrorType:  vis.ErrorType,
			Message:    vis.Message,
			Detail:     detail,
		}
	}
	if detail, ok := upstreamForwardErrorDetailFromContext(c); ok {
		// Re-classify generic transport fallbacks when ops detail already holds
		// a more specific business cause (common for streamed Responses).
		if vis, ok := service.PreferClientVisibleUpstreamError(forwardErr, detail.ErrorType, detail.Message, detail.Detail); ok {
			return upstreamForwardErrorDetail{
				StatusCode: vis.StatusCode,
				ErrorType:  vis.ErrorType,
				Message:    vis.Message,
				Detail:     firstNonEmptyForwardDetail(detail.Detail, sanitizeUpstreamForwardErrorDetail(errString(forwardErr))),
			}
		}
		if detail.StatusCode == 0 {
			detail.StatusCode = statusForUpstreamForwardErrorType(detail.ErrorType)
		}
		return detail
	}
	return classifyUpstreamForwardError(forwardErr)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func firstNonEmptyForwardDetail(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func statusForUpstreamForwardErrorType(errType string) int {
	switch strings.TrimSpace(errType) {
	case "invalid_request_error", "not_found_error":
		return 400
	case "rate_limit_error":
		return 429
	case "overloaded_error":
		return 503
	default:
		return 502
	}
}

func shouldSuppressForwardErrorResponse(c *gin.Context, forwardErr error) bool {
	if service.IsResponseCommitted(c) {
		return true
	}
	if service.IsOpenAIWSSessionPreemptedError(forwardErr) {
		return true
	}
	if c == nil || c.Request == nil {
		return false
	}
	reqCtx := c.Request.Context()
	if reqCtx == nil || reqCtx.Err() == nil {
		return false
	}
	return errors.Is(forwardErr, context.Canceled) || errors.Is(reqCtx.Err(), context.Canceled)
}

func upstreamForwardErrorDetailFromContext(c *gin.Context) (upstreamForwardErrorDetail, bool) {
	if c == nil {
		return upstreamForwardErrorDetail{}, false
	}

	rawType, ok := c.Get(service.OpsUpstreamErrorTypeKey)
	if !ok {
		return upstreamForwardErrorDetail{}, false
	}
	errType, ok := rawType.(string)
	if !ok {
		return upstreamForwardErrorDetail{}, false
	}
	errType = strings.TrimSpace(errType)
	if !isDetailedUpstreamOpsErrorType(errType) && !isClientVisibleUpstreamForwardErrorType(errType) {
		return upstreamForwardErrorDetail{}, false
	}

	detail := upstreamForwardErrorDetail{
		ErrorType: errType,
		Message:   "Upstream request failed",
	}
	if rawMessage, ok := c.Get(service.OpsUpstreamErrorMessageKey); ok {
		if message, ok := rawMessage.(string); ok {
			if message = strings.TrimSpace(message); message != "" {
				detail.Message = message
			}
		}
	}
	if rawDetail, ok := c.Get(service.OpsUpstreamErrorDetailKey); ok {
		if s, ok := rawDetail.(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				detail.Detail = sanitizeUpstreamForwardErrorDetail(s)
			}
		}
	}
	if rawStatus, ok := c.Get(service.OpsUpstreamStatusCodeKey); ok {
		switch v := rawStatus.(type) {
		case int:
			if v > 0 {
				detail.StatusCode = v
			}
		case int64:
			if v > 0 {
				detail.StatusCode = int(v)
			}
		}
	}
	if detail.StatusCode == 0 {
		detail.StatusCode = statusForUpstreamForwardErrorType(detail.ErrorType)
	}
	return detail, true
}

func isClientVisibleUpstreamForwardErrorType(errType string) bool {
	switch strings.TrimSpace(errType) {
	case "invalid_request_error", "not_found_error", "rate_limit_error", "overloaded_error":
		return true
	default:
		return false
	}
}

func sanitizeUpstreamForwardErrorDetail(msg string) string {
	if msg == "" {
		return msg
	}
	msg = upstreamForwardSensitiveQueryParamPattern.ReplaceAllString(msg, `$1***`)
	return upstreamForwardSensitiveBearerPattern.ReplaceAllString(msg, `${1}***`)
}

func parseUpstreamHTTP2StreamError(detail string) string {
	matches := upstreamHTTP2StreamErrorPattern.FindStringSubmatch(strings.TrimSpace(detail))
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}
