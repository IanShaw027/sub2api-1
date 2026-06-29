package repository

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// P4: ALPN 驱动的 h1/h2 出站分流单测。

func TestTLSFingerprintProfileWantsH2(t *testing.T) {
	require.False(t, tlsFingerprintProfileWantsH2(nil))
	require.False(t, tlsFingerprintProfileWantsH2(&tlsfingerprint.Profile{}))
	require.False(t, tlsFingerprintProfileWantsH2(&tlsfingerprint.Profile{ALPNProtocols: []string{"http/1.1"}}))
	require.True(t, tlsFingerprintProfileWantsH2(&tlsfingerprint.Profile{ALPNProtocols: []string{"h2", "http/1.1"}}))
	require.True(t, tlsFingerprintProfileWantsH2(&tlsfingerprint.Profile{ALPNProtocols: []string{" H2 "}}))
}

func TestBuildUpstreamRoundTripperWithTLSFingerprintNonH2UsesHTTP1(t *testing.T) {
	settings := poolSettings{maxIdleConns: 10, idleConnTimeout: time.Minute}
	profile := &tlsfingerprint.Profile{Name: "h1-only", ALPNProtocols: []string{"http/1.1"}}

	rt, h1, err := buildUpstreamRoundTripperWithTLSFingerprint(settings, nil, profile)
	require.NoError(t, err)
	tr, ok := rt.(*http.Transport)
	require.True(t, ok, "non-h2 profile should yield *http.Transport")
	require.NotNil(t, h1, "h1 transport handle must be returned for HTTP/1.x field tuning")
	require.Same(t, tr, h1)
	require.False(t, tr.ForceAttemptHTTP2)
}

func TestBuildUpstreamRoundTripperWithTLSFingerprintH2UsesHTTP2(t *testing.T) {
	settings := poolSettings{maxIdleConns: 10, idleConnTimeout: time.Minute}
	profile := &tlsfingerprint.Profile{Name: "h2", ALPNProtocols: []string{"h2", "http/1.1"}}

	for _, proxy := range []string{"", "http://user:pass@127.0.0.1:8080", "socks5://127.0.0.1:1080"} {
		var parsed *url.URL
		if proxy != "" {
			var err error
			parsed, err = url.Parse(proxy)
			require.NoError(t, err)
		}
		rt, h1, err := buildUpstreamRoundTripperWithTLSFingerprint(settings, parsed, profile)
		require.NoError(t, err, "proxy=%q", proxy)
		_, ok := rt.(*http2.Transport)
		require.True(t, ok, "h2 profile should yield *http2.Transport (proxy=%q)", proxy)
		require.Nil(t, h1, "h2 path has no HTTP/1.x transport handle (proxy=%q)", proxy)
	}
}

func TestBuildUpstreamRoundTripperWithTLSFingerprintH2UnknownProxyFallsBackToHTTP1(t *testing.T) {
	settings := poolSettings{maxIdleConns: 10, idleConnTimeout: time.Minute}
	profile := &tlsfingerprint.Profile{Name: "h2", ALPNProtocols: []string{"h2", "http/1.1"}}
	parsed, err := url.Parse("quic://127.0.0.1:9999")
	require.NoError(t, err)

	rt, h1, err := buildUpstreamRoundTripperWithTLSFingerprint(settings, parsed, profile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported proxy scheme")
	require.Nil(t, rt)
	require.Nil(t, h1)
}
