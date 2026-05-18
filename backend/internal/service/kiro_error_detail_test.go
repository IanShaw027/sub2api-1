package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildKiroOAuthTokenExchangeErrorIncludesUpstreamDetail(t *testing.T) {
	err := buildKiroOAuthTokenExchangeError(http.StatusBadRequest, []byte(`{
		"error":"invalid_grant",
		"error_description":"authorization code expired"
	}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "重新生成授权链接")
	require.ErrorContains(t, err, "authorization code expired")
}

func TestKiroHTTPStatusErrorMessageIncludesParsedJSONDetail(t *testing.T) {
	msg := kiroHTTPStatusErrorMessage("Kiro API", http.StatusBadRequest, []byte(`{
		"error":"invalid_request",
		"message":"selected model is not available for this account"
	}`))
	require.Contains(t, msg, "Kiro API returned 400")
	require.Contains(t, msg, "invalid_request")
	require.Contains(t, msg, "selected model is not available for this account")
}

func TestKiroHTTPStatusErrorMessageRecognizesQuotaExhausted402(t *testing.T) {
	msg := kiroHTTPStatusErrorMessage("Kiro API", http.StatusPaymentRequired, []byte(`{
		"message":"MONTHLY_REQUEST_COUNT exceeded for this subscription"
	}`))
	require.Contains(t, msg, "Kiro API returned 402")
	require.Contains(t, msg, "monthly quota or request count exhausted")
	require.Contains(t, msg, "MONTHLY_REQUEST_COUNT exceeded for this subscription")
}

func TestClassifyKiroHTTPErrorSemanticLeavesGeneric402Unchanged(t *testing.T) {
	semantic := classifyKiroHTTPErrorSemantic(http.StatusPaymentRequired, []byte(`{
		"message":"billing profile incomplete"
	}`))
	require.Equal(t, kiroHTTPErrorSemanticUnknown, semantic)
}
