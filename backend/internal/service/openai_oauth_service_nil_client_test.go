package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthService_ExchangeCode_NilOAuthClientReturnsError(t *testing.T) {
	svc := NewOpenAIOAuthService(nil, nil)
	defer svc.Stop()

	svc.sessionStore.Set("sid", &openai.OAuthSession{
		State:        "expected-state",
		CodeVerifier: "verifier",
		RedirectURI:  openai.DefaultRedirectURI,
		CreatedAt:    time.Now(),
	})

	_, err := svc.ExchangeCode(context.Background(), &OpenAIExchangeCodeInput{
		SessionID: "sid",
		Code:      "auth-code",
		State:     "expected-state",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth client is not configured")
}

func TestOpenAIOAuthService_RefreshTokenWithClientID_NilOAuthClientReturnsError(t *testing.T) {
	svc := NewOpenAIOAuthService(nil, nil)
	defer svc.Stop()

	_, err := svc.RefreshTokenWithClientID(context.Background(), "refresh-token", "", "client-id")
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth client is not configured")
}
