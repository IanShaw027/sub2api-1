//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
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

func TestMarkOpsCyberPolicy_SupplementalUpdateDoesNotMutateExistingPointer(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)

	MarkOpsCyberPolicy(c, CyberPolicyMark{
		Message: "blocked",
		Body:    "raw-body",
	})
	first := GetOpsCyberPolicy(c)
	require.NotNil(t, first)
	require.Equal(t, 0, first.UpstreamStatus)
	require.Equal(t, 0, first.UpstreamInTok)
	require.Equal(t, 0, first.UpstreamOutTok)

	MarkOpsCyberPolicy(c, CyberPolicyMark{
		UpstreamStatus: http.StatusForbidden,
		UpstreamInTok:  123,
		UpstreamOutTok: 45,
		Message:        "should not overwrite",
		Body:           "should not overwrite",
	})

	updated := GetOpsCyberPolicy(c)
	require.NotNil(t, updated)
	require.NotSame(t, first, updated)
	require.Equal(t, "blocked", updated.Message)
	require.Equal(t, "raw-body", updated.Body)
	require.Equal(t, http.StatusForbidden, updated.UpstreamStatus)
	require.Equal(t, 123, updated.UpstreamInTok)
	require.Equal(t, 45, updated.UpstreamOutTok)

	require.Equal(t, 0, first.UpstreamStatus)
	require.Equal(t, 0, first.UpstreamInTok)
	require.Equal(t, 0, first.UpstreamOutTok)
}
