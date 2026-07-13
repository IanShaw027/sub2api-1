package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyDefaultGrokUpstreamHeadersUsesBrowserLikeUserAgent(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("x-grok-client-version", "none")

	applyDefaultGrokUpstreamHeaders(req)

	require.Equal(t, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36", req.Header.Get("User-Agent"))
	require.Equal(t, grokClientVersionHeader, req.Header.Get("x-grok-client-version"))
	require.Equal(t, grokClientIdentifierHeader, req.Header.Get("x-grok-client-identifier"))
}
