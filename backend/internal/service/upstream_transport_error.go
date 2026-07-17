package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
)

type upstreamTransportErrorDetail struct {
	ErrorType string
	Message   string
	Detail    string
}

func recordDetailedUpstreamTransportError(c *gin.Context, err error) upstreamTransportErrorDetail {
	detail := classifyUpstreamTransportError(err)
	SetOpsUpstreamErrorWithType(c, detail.ErrorType, 0, detail.Message, detail.Detail)
	return detail
}

func classifyUpstreamTransportError(err error) upstreamTransportErrorDetail {
	if err == nil {
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_error",
			Message:   "Upstream request failed",
		}
	}

	// Business failures (context window, usage limit, deactivated workspace)
	// must not be reported as transport errors.
	if vis, ok := ClassifyClientVisibleUpstreamErrorFromErr(err); ok {
		return upstreamTransportErrorDetail{
			ErrorType: vis.ErrorType,
			Message:   vis.Message,
			Detail:    strings.TrimSpace(sanitizeUpstreamErrorMessage(err.Error())),
		}
	}

	detail := strings.TrimSpace(sanitizeUpstreamErrorMessage(err.Error()))
	lowerDetail := strings.ToLower(detail)

	switch {
	case strings.Contains(lowerDetail, "stream error: stream id") && strings.Contains(lowerDetail, "received from peer"):
		if code := extractHTTP2PeerErrorCode(detail); code != "" {
			return upstreamTransportErrorDetail{
				ErrorType: "upstream_http2_" + strings.ToLower(strings.TrimSuffix(code, "_ERROR")) + "_error",
				Message:   "Upstream HTTP/2 peer reset the stream with " + code,
				Detail:    detail,
			}
		}
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_http2_stream_error",
			Message:   "Upstream HTTP/2 peer reset the stream",
			Detail:    detail,
		}
	case errors.Is(err, context.DeadlineExceeded):
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_timeout_error",
			Message:   "Upstream request timed out",
			Detail:    detail,
		}
	case errors.Is(err, context.Canceled):
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_canceled_error",
			Message:   "Upstream request was canceled",
			Detail:    detail,
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_timeout_error",
			Message:   "Upstream request timed out",
			Detail:    detail,
		}
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_dns_error",
			Message:   "Failed to resolve upstream host",
			Detail:    detail,
		}
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		switch {
		case opErr.Op == "dial":
			return upstreamTransportErrorDetail{
				ErrorType: "upstream_connect_error",
				Message:   "Failed to connect to upstream service",
				Detail:    detail,
			}
		case strings.Contains(lowerDetail, "tls"):
			return upstreamTransportErrorDetail{
				ErrorType: "upstream_tls_error",
				Message:   "TLS handshake with upstream failed",
				Detail:    detail,
			}
		}
	}

	switch {
	case strings.Contains(lowerDetail, "tls"):
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_tls_error",
			Message:   "TLS handshake with upstream failed",
			Detail:    detail,
		}
	case strings.Contains(lowerDetail, "connection reset by peer"):
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_connection_reset_error",
			Message:   "Upstream connection was reset by peer",
			Detail:    detail,
		}
	case strings.Contains(lowerDetail, "use of closed network connection"),
		strings.Contains(lowerDetail, "unexpected eof"),
		strings.Contains(lowerDetail, "server closed idle connection"):
		return upstreamTransportErrorDetail{
			ErrorType: "upstream_connection_closed_error",
			Message:   "Upstream connection closed unexpectedly",
			Detail:    detail,
		}
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED:
			return upstreamTransportErrorDetail{
				ErrorType: "upstream_connect_error",
				Message:   "Failed to connect to upstream service",
				Detail:    detail,
			}
		case syscall.ECONNRESET:
			return upstreamTransportErrorDetail{
				ErrorType: "upstream_connection_reset_error",
				Message:   "Upstream connection was reset by peer",
				Detail:    detail,
			}
		}
	}

	return upstreamTransportErrorDetail{
		ErrorType: "upstream_transport_error",
		Message:   "Upstream transport request failed",
		Detail:    detail,
	}
}

func extractHTTP2PeerErrorCode(detail string) string {
	const marker = "; received from peer"
	if !strings.Contains(detail, marker) {
		return ""
	}
	before, _, ok := strings.Cut(detail, marker)
	if !ok {
		return ""
	}
	parts := strings.Split(before, ";")
	if len(parts) == 0 {
		return ""
	}
	code := strings.TrimSpace(parts[len(parts)-1])
	if code == "" || !strings.HasSuffix(code, "_ERROR") {
		return ""
	}
	return code
}

func formatUpstreamRequestFailed(detail upstreamTransportErrorDetail, fallback string) string {
	if detail.Message != "" {
		return detail.Message
	}
	return fallback
}

func formatUpstreamRequestFailedAfterRetries(detail upstreamTransportErrorDetail) string {
	if detail.Message == "" {
		return "Upstream request failed after retries"
	}
	return fmt.Sprintf("%s after retries", detail.Message)
}
