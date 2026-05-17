package handler

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"
)

var upstreamHTTP2StreamErrorPattern = regexp.MustCompile(`stream error: stream ID \d+; ([A-Z_]+)(?:;|$)`)

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

	detail := strings.TrimSpace(err.Error())
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

func parseUpstreamHTTP2StreamError(detail string) string {
	matches := upstreamHTTP2StreamErrorPattern.FindStringSubmatch(strings.TrimSpace(detail))
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}
