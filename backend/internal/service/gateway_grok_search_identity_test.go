//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoGrokNativeResponsesJSONAPIKeyDoesNotFailClosedOnProfileError(t *testing.T) {
	injectOutboundGrokProfileError(t, errors.New("identity_reject: profile load failed"))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_search"}`)),
	}}
	svc := &GatewayService{httpUpstream: upstream}
	account := &Account{
		ID:       900099,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-key",
		},
	}

	body, err := svc.DoGrokNativeResponsesJSON(context.Background(), account, []byte(`{"input":"query"}`))
	require.NoError(t, err)
	require.Equal(t, `{"id":"resp_search"}`, string(body))
	require.Equal(t, "Bearer xai-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, defaultGrokUpstreamUserAgent(), upstream.lastReq.Header.Get("User-Agent"))
}

func TestDoGrokNativeResponsesJSONOAuthProfileErrorFailovers(t *testing.T) {
	injectOutboundGrokProfileError(t, errors.New("identity_reject: profile load failed"))

	svc := &GatewayService{httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_search"}`)),
	}}}
	account := &Account{
		ID:       900098,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "oauth-access-token",
		},
	}

	_, err := svc.DoGrokNativeResponsesJSON(context.Background(), account, []byte(`{"input":"query"}`))
	require.Error(t, err)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, GatewayFailureReason("grok_search_identity"), failover.Reason)
	require.Equal(t, http.StatusBadGateway, failover.StatusCode)
}
