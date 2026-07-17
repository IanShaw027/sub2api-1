package service

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyUpstreamTransportError_HTTP2InternalError(t *testing.T) {
	detail := classifyUpstreamTransportError(errors.New("stream error: stream ID 3; INTERNAL_ERROR; received from peer"))

	require.Equal(t, "upstream_http2_internal_error", detail.ErrorType)
	require.Equal(t, "Upstream HTTP/2 peer reset the stream with INTERNAL_ERROR", detail.Message)
	require.Equal(t, "stream error: stream ID 3; INTERNAL_ERROR; received from peer", detail.Detail)
}

func TestClassifyUpstreamTransportError_Timeout(t *testing.T) {
	detail := classifyUpstreamTransportError(context.DeadlineExceeded)

	require.Equal(t, "upstream_timeout_error", detail.ErrorType)
	require.Equal(t, "Upstream request timed out", detail.Message)
}

func TestClassifyUpstreamTransportError_Connect(t *testing.T) {
	detail := classifyUpstreamTransportError(&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED})

	require.Equal(t, "upstream_connect_error", detail.ErrorType)
	require.Equal(t, "Failed to connect to upstream service", detail.Message)
}

func TestFormatUpstreamRequestFailedAfterRetries(t *testing.T) {
	msg := formatUpstreamRequestFailedAfterRetries(upstreamTransportErrorDetail{
		Message: "Upstream request timed out",
	})

	require.Equal(t, "Upstream request timed out after retries", msg)
}

func TestClassifyUpstreamTransportError_ContextWindowNotTransport(t *testing.T) {
	detail := classifyUpstreamTransportError(errors.New("OpenAI upstream SSE context window exceeded"))
	require.Equal(t, "invalid_request_error", detail.ErrorType)
	require.Contains(t, detail.Message, "context window")
	require.NotEqual(t, "upstream_transport_error", detail.ErrorType)
}
