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
		require.Equal(t, 502, detail.StatusCode)
	})

	t.Run("timeout error", func(t *testing.T) {
		detail := classifyUpstreamForwardError(context.DeadlineExceeded)
		require.Equal(t, "upstream_timeout_error", detail.ErrorType)
		require.Equal(t, "Upstream request timed out", detail.Message)
		require.Equal(t, 502, detail.StatusCode)
	})

	t.Run("connection refused error", func(t *testing.T) {
		detail := classifyUpstreamForwardError(errors.New("dial tcp 127.0.0.1:443: connect: connection refused"))
		require.Equal(t, "upstream_connect_error", detail.ErrorType)
		require.Equal(t, "Failed to connect to upstream service", detail.Message)
	})

	t.Run("context window is not transport", func(t *testing.T) {
		detail := classifyUpstreamForwardError(errors.New("OpenAI upstream SSE context window exceeded"))
		require.Equal(t, "invalid_request_error", detail.ErrorType)
		require.Equal(t, 400, detail.StatusCode)
		require.Contains(t, detail.Message, "context window")
		require.NotEqual(t, "upstream_transport_error", detail.ErrorType)
		require.NotEqual(t, "Upstream transport error", detail.Message)
	})

	t.Run("sanitizes sensitive query params", func(t *testing.T) {
		detail := classifyUpstreamForwardError(errors.New(`GET "https://api.example.com/v1?access_token=secret&refresh_token=abc&key=xyz"`))
		require.Equal(t, "upstream_transport_error", detail.ErrorType)
		require.NotContains(t, detail.Detail, "secret")
		require.NotContains(t, detail.Detail, "abc")
		require.NotContains(t, detail.Detail, "xyz")
		require.Contains(t, detail.Detail, "access_token=***")
		require.Contains(t, detail.Detail, "refresh_token=***")
		require.Contains(t, detail.Detail, "key=***")
	})

	t.Run("sanitizes bearer tokens", func(t *testing.T) {
		detail := classifyUpstreamForwardError(errors.New(`proxy failed Authorization: Bearer sk-secret-token, retrying`))
		require.Equal(t, "upstream_transport_error", detail.ErrorType)
		require.NotContains(t, detail.Detail, "sk-secret-token")
		require.Contains(t, detail.Detail, "Authorization: Bearer ***")
	})

	t.Run("sanitizes structured api keys and whitespace credentials", func(t *testing.T) {
		detail := classifyUpstreamForwardError(errors.New(`upstream failed {"api_key":"secret-api","access_token":"secret-access"} refresh_token=secret-refresh`))
		require.NotContains(t, detail.Detail, "secret-api")
		require.NotContains(t, detail.Detail, "secret-access")
		require.NotContains(t, detail.Detail, "secret-refresh")
	})
}
