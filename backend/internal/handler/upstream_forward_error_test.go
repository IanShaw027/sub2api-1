package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyUpstreamForwardError(t *testing.T) {
	t.Run("http2 internal error", func(t *testing.T) {
		detail := classifyUpstreamForwardError(errors.New("stream error: stream ID 3; INTERNAL_ERROR; received from peer"))
		require.Equal(t, "upstream_http2_internal_error", detail.ErrorType)
		require.Equal(t, "Upstream HTTP/2 peer reset the stream with INTERNAL_ERROR", detail.Message)
		require.Equal(t, "stream error: stream ID 3; INTERNAL_ERROR; received from peer", detail.Detail)
	})

	t.Run("timeout error", func(t *testing.T) {
		detail := classifyUpstreamForwardError(context.DeadlineExceeded)
		require.Equal(t, "upstream_timeout_error", detail.ErrorType)
		require.Equal(t, "Upstream request timed out", detail.Message)
	})

	t.Run("connection refused error", func(t *testing.T) {
		detail := classifyUpstreamForwardError(errors.New("dial tcp 127.0.0.1:443: connect: connection refused"))
		require.Equal(t, "upstream_connect_error", detail.ErrorType)
		require.Equal(t, "Failed to connect to upstream service", detail.Message)
	})
}
