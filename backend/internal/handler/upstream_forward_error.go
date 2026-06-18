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

type upstreamForwardErrorDetail struct {
	ErrorType string
	Message   string
	Detail    string
}

func classifyUpstreamForwardError(err error) upstreamForwardErrorDetail {
	if err == nil {
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_error",
			Message:   "Upstream request failed",
		}
	}

	detail := sanitizeUpstreamForwardErrorDetail(strings.TrimSpace(err.Error()))
	lowerDetail := strings.ToLower(detail)

	if errors.Is(err, context.DeadlineExceeded) {
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_timeout_error",
			Message:   "Upstream request timed out",
			Detail:    detail,
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_timeout_error",
			Message:   "Upstream request timed out",
			Detail:    detail,
		}
	}

	if code := parseUpstreamHTTP2StreamError(detail); code != "" {
		normalizedCode := strings.ToLower(code)
		normalizedCode = strings.TrimSuffix(normalizedCode, "_error")
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_http2_" + normalizedCode + "_error",
			Message:   "Upstream HTTP/2 peer reset the stream with " + code,
			Detail:    detail,
		}
	}

	switch {
	case strings.Contains(lowerDetail, "context canceled"):
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_canceled_error",
			Message:   "Upstream request was canceled",
			Detail:    detail,
		}
	case strings.Contains(lowerDetail, "no such host"),
		strings.Contains(lowerDetail, "server misbehaving"):
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_dns_error",
			Message:   "Upstream DNS lookup failed",
			Detail:    detail,
		}
	case strings.Contains(lowerDetail, "connection refused"),
		strings.Contains(lowerDetail, "dial tcp"):
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_connect_error",
			Message:   "Failed to connect to upstream service",
			Detail:    detail,
		}
	case strings.Contains(lowerDetail, "tls:"),
		strings.Contains(lowerDetail, "x509:"):
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_tls_error",
			Message:   "Upstream TLS handshake failed",
			Detail:    detail,
		}
	case strings.Contains(lowerDetail, "connection reset by peer"),
		strings.Contains(lowerDetail, "broken pipe"):
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_connection_reset_error",
			Message:   "Upstream connection was reset",
			Detail:    detail,
		}
	case detail == "EOF",
		strings.Contains(lowerDetail, "unexpected eof"),
		strings.Contains(lowerDetail, "client connection lost"):
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_connection_closed_error",
			Message:   "Upstream connection closed unexpectedly",
			Detail:    detail,
		}
	default:
		return upstreamForwardErrorDetail{
			ErrorType: "upstream_transport_error",
			Message:   "Upstream transport error",
			Detail:    detail,
		}
	}
}

func resolveUpstreamForwardErrorDetail(c *gin.Context, forwardErr error) upstreamForwardErrorDetail {
	if detail, ok := upstreamForwardErrorDetailFromContext(c); ok {
		return detail
	}
	return classifyUpstreamForwardError(forwardErr)
}

func shouldSuppressForwardErrorResponse(c *gin.Context, forwardErr error) bool {
	if service.IsResponseCommitted(c) {
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
	return detail, true
}

func isClientVisibleUpstreamForwardErrorType(errType string) bool {
	switch strings.TrimSpace(errType) {
	case "invalid_request_error", "not_found_error":
		return true
	default:
		return false
	}
}

func sanitizeUpstreamForwardErrorDetail(msg string) string {
	if msg == "" {
		return msg
	}
	return upstreamForwardSensitiveQueryParamPattern.ReplaceAllString(msg, `$1***`)
}

func parseUpstreamHTTP2StreamError(detail string) string {
	matches := upstreamHTTP2StreamErrorPattern.FindStringSubmatch(strings.TrimSpace(detail))
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}
