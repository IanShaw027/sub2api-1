//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectOpenAICyberPolicy_ErrorCode(t *testing.T) {
	ok, code, msg := detectOpenAICyberPolicy([]byte(`{"error":{"code":"cyber_policy","message":"blocked by cyber safety policy"}}`))

	require.True(t, ok)
	require.Equal(t, "cyber_policy", code)
	require.Equal(t, "blocked by cyber safety policy", msg)
}

func TestDetectOpenAICyberPolicy_ResponseErrorCodeCaseInsensitive(t *testing.T) {
	ok, code, msg := detectOpenAICyberPolicy([]byte(`{"response":{"error":{"code":"CYBER_POLICY","message":"denied"}}}`))

	require.True(t, ok)
	require.Equal(t, "CYBER_POLICY", code)
	require.Equal(t, "denied", msg)
}

func TestDetectOpenAICyberPolicy_IgnoresOtherErrors(t *testing.T) {
	ok, code, msg := detectOpenAICyberPolicy([]byte(`{"error":{"code":"rate_limit_exceeded","message":"try later"}}`))

	require.False(t, ok)
	require.Empty(t, code)
	require.Empty(t, msg)
}

func TestDetectOpenAICyberPolicy_SSEBody(t *testing.T) {
	body := []byte("event: response.failed\n" +
		`data: {"type":"response.failed","response":{"error":{"code":"cyber_policy","message":"policy denied"}}}` +
		"\n\n")

	ok, code, msg := detectOpenAICyberPolicy(body)

	require.True(t, ok)
	require.Equal(t, "cyber_policy", code)
	require.Equal(t, "policy denied", msg)
}

func TestShouldFailoverOpenAIUpstreamResponse_CyberPolicyIsNonRetryable(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{"error":{"code":"cyber_policy","message":"blocked by policy"}}`)

	require.False(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusForbidden, "blocked by policy", body))
	require.False(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusTooManyRequests, "blocked by policy", body))
	require.False(t, svc.shouldFailoverOpenAIUpstreamResponse(http.StatusInternalServerError, "blocked by policy", body))
}

func TestShouldFailoverOpenAIPassthroughResponse_CyberPolicyIsNonRetryable(t *testing.T) {
	body := []byte(`{"response":{"error":{"code":"cyber_policy","message":"blocked by policy"}}}`)

	require.False(t, shouldFailoverOpenAIPassthroughResponse(nil, http.StatusTooManyRequests, body))
	require.False(t, shouldFailoverOpenAIPassthroughResponse(nil, http.StatusInternalServerError, body))
}
