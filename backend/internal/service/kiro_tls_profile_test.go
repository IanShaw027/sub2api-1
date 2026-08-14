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

func TestApplyKiroTLSFingerprintRuntime_PrefersProfileUserAgent(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.test", nil)
	require.NoError(t, err)

	applyKiroTLSFingerprintRuntimeWithProfile(req, accountTLSFingerprintRuntime{
		UpstreamUserAgent:  "tls-runtime-ua/9.9.9",
		UpstreamOriginator: "kiro-ide",
	}, &AccountDeviceProfile{
		ProfilePayload: map[string]any{
			"user_agent": "profile-kiro-ua/1.0.0",
		},
	})

	require.Equal(t, "profile-kiro-ua/1.0.0", req.Header.Get("User-Agent"))
	require.Equal(t, "kiro-ide", getHeaderRaw(req.Header, "originator"))
}

func TestApplyKiroTLSFingerprintRuntime_KeepsCodeWhispererUAWhenTLSDoesNotStamp(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.test", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "aws-sdk-js/1.0.0 kiro/2.5.0")

	applyKiroTLSFingerprintRuntimeWithProfile(req, accountTLSFingerprintRuntime{}, &AccountDeviceProfile{
		ProfilePayload: map[string]any{
			"user_agent": "profile-kiro-ua/1.0.0",
		},
	})

	require.Equal(t, "aws-sdk-js/1.0.0 kiro/2.5.0", req.Header.Get("User-Agent"))
}
