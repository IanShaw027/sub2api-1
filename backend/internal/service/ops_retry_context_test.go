package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewOpsRetryContext_SetsHTTPTransportAndRequestHeaders(t *testing.T) {
	errorLog := &OpsErrorLogDetail{
		OpsErrorLog: OpsErrorLog{
			RequestPath: "/openai/v1/responses",
		},
		UserAgent: "ops-retry-agent/1.0",
		RequestHeaders: `{
			"anthropic-beta":"beta-v1",
			"ANTHROPIC-VERSION":"2023-06-01",
			"authorization":"Bearer should-not-forward"
		}`,
	}

	c, w := newOpsRetryContext(context.Background(), errorLog)
	require.NotNil(t, c)
	require.NotNil(t, w)
	require.NotNil(t, c.Request)

	require.Equal(t, "/openai/v1/responses", c.Request.URL.Path)
	require.Equal(t, "application/json", c.Request.Header.Get("Content-Type"))
	require.Equal(t, "ops-retry-agent/1.0", c.Request.Header.Get("User-Agent"))
	require.Equal(t, "beta-v1", c.Request.Header.Get("anthropic-beta"))
	require.Equal(t, "2023-06-01", c.Request.Header.Get("anthropic-version"))
	require.Empty(t, c.Request.Header.Get("authorization"), "未在白名单内的敏感头不应被重放")
	require.Equal(t, OpenAIClientTransportHTTP, GetOpenAIClientTransport(c))
}

func TestNewOpsRetryContext_InvalidHeadersJSONStillSetsHTTPTransport(t *testing.T) {
	errorLog := &OpsErrorLogDetail{
		RequestHeaders: "{invalid-json",
	}

	c, _ := newOpsRetryContext(context.Background(), errorLog)
	require.NotNil(t, c)
	require.NotNil(t, c.Request)
	require.Equal(t, "/", c.Request.URL.Path)
	require.Equal(t, OpenAIClientTransportHTTP, GetOpenAIClientTransport(c))
}

func TestExtractResponsePreview_SummarizesKiroEventStream(t *testing.T) {
	w := newLimitedResponseWriter(opsRetryCaptureBytesLimit)
	_, err := w.Write(append(
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "你好"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "contextUsageEvent",
		}, map[string]any{"contextUsagePercentage": 0.5})...,
	))
	require.NoError(t, err)

	preview, truncated := extractResponsePreview(w)
	require.False(t, truncated)
	require.Contains(t, preview, "Kiro eventstream 2 frame(s)")
	require.Contains(t, preview, "assistantResponseEvent")
	require.Contains(t, preview, `assistant_text="你好"`)
}

func TestExtractResponsePreview_HidesGenericBinaryBodies(t *testing.T) {
	w := newLimitedResponseWriter(opsRetryCaptureBytesLimit)
	_, err := w.Write([]byte{0x00, 0x01, 0x02, 0x03})
	require.NoError(t, err)

	preview, truncated := extractResponsePreview(w)
	require.False(t, truncated)
	require.True(t, strings.HasPrefix(preview, "[binary response body: "))
}
