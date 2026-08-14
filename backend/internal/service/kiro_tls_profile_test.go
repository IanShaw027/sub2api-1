package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyKiroTLSFingerprintRuntime_UsesLowercaseOriginator(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.test", nil)
	require.NoError(t, err)

	applyKiroTLSFingerprintRuntime(req, accountTLSFingerprintRuntime{
		UpstreamUserAgent:  "kiro/1.2.3",
		UpstreamOriginator: "kiro-ide",
	})

	require.Equal(t, "kiro/1.2.3", req.Header.Get("User-Agent"))
	require.Equal(t, "kiro-ide", getHeaderRaw(req.Header, "originator"))
	require.Contains(t, req.Header, "originator")
	require.Empty(t, req.Header.Get("X-Originator"))
	_, hasXOriginator := req.Header["X-Originator"]
	require.False(t, hasXOriginator)
}
