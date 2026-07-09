package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthService_GenerateAuthURL_NilProxyRepoReturnsError(t *testing.T) {
	svc := NewOpenAIOAuthService(nil, &openaiOAuthClientRedirectStub{})
	defer svc.Stop()

	proxyID := int64(42)
	_, err := svc.GenerateAuthURL(context.Background(), &proxyID, openai.DefaultRedirectURI, PlatformOpenAI)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy repository is unavailable")
}

func TestOpenAIOAuthService_ExchangeCode_NilProxyRepoReturnsError(t *testing.T) {
	client := &openaiOAuthClientStateStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()

	svc.sessionStore.Set("sid", &openai.OAuthSession{
		State:        "expected-state",
		CodeVerifier: "verifier",
		RedirectURI:  openai.DefaultRedirectURI,
		CreatedAt:    time.Now(),
	})

	proxyID := int64(42)
	_, err := svc.ExchangeCode(context.Background(), &OpenAIExchangeCodeInput{
		SessionID: "sid",
		Code:      "auth-code",
		State:     "expected-state",
		ProxyID:   &proxyID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy repository is unavailable")
	require.Equal(t, int32(0), atomic.LoadInt32(&client.exchangeCalled))
}
