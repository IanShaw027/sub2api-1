package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyClientVisibleUpstreamError_ContextWindow(t *testing.T) {
	vis, ok := ClassifyClientVisibleUpstreamError("OpenAI upstream SSE context window exceeded", nil)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, vis.StatusCode)
	require.Equal(t, "invalid_request_error", vis.ErrorType)
	require.Equal(t, openAIContextWindowClientMessage, vis.Message)

	vis, ok = ClassifyClientVisibleUpstreamError(
		"Your input exceeds the context window of this model. Please adjust your input and try again.",
		[]byte(`{"error":{"code":"context_length_exceeded","message":"Your input exceeds the context window of this model. Please adjust your input and try again."}}`),
	)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, vis.StatusCode)
	require.Equal(t, "invalid_request_error", vis.ErrorType)
	require.Contains(t, strings.ToLower(vis.Message), "context window")
}

func TestClassifyClientVisibleUpstreamError_UsageLimitNotContext(t *testing.T) {
	body := []byte(`{"error":{"message":"The usage limit has been reached","plan_type":"k12"}}`)
	vis, ok := ClassifyClientVisibleUpstreamError("The usage limit has been reached", body)
	require.True(t, ok)
	require.Equal(t, http.StatusTooManyRequests, vis.StatusCode)
	require.Equal(t, "rate_limit_error", vis.ErrorType)
	require.Equal(t, openAIBillingLimitClientMessage(), vis.Message)
}

func TestClassifyClientVisibleUpstreamError_DeactivatedWorkspace(t *testing.T) {
	body := []byte(`{"detail":{"code":"deactivated_workspace"}}`)
	vis, ok := ClassifyClientVisibleUpstreamError("Upstream compact response failed", body)
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, vis.StatusCode)
	require.Equal(t, "upstream_error", vis.ErrorType)
	require.Equal(t, "Upstream workspace is deactivated", vis.Message)
}

func TestClassifyClientVisibleUpstreamErrorFromErr_ContextCompactionSSE(t *testing.T) {
	err := newOpenAIContextCompactionSSEError(
		[]byte(`{"type":"response.failed","response":{"error":{"code":"context_length_exceeded","message":"context window exceeded"}}}`),
		"context window exceeded",
	)
	require.NotNil(t, err)

	vis, ok := ClassifyClientVisibleUpstreamErrorFromErr(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, vis.StatusCode)
	require.Equal(t, "invalid_request_error", vis.ErrorType)
	require.Contains(t, strings.ToLower(vis.Message), "context")
}

func TestPreferClientVisibleUpstreamError_OpsDetailTransportFallback(t *testing.T) {
	vis, ok := PreferClientVisibleUpstreamError(
		nil,
		"upstream_transport_error",
		"Upstream transport error",
		"OpenAI upstream SSE context window exceeded",
	)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, vis.StatusCode)
	require.Equal(t, "invalid_request_error", vis.ErrorType)
}
