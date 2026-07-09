//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestGrokOAuthService_ExchangeCode_NilOAuthClientReturnsError(t *testing.T) {
	svc := NewGrokOAuthService(nil, nil)
	defer svc.Stop()

	auth, err := svc.GenerateAuthURL(context.Background(), nil, "")
	require.NoError(t, err)

	_, err = svc.ExchangeCode(context.Background(), &GrokExchangeCodeInput{
		SessionID: auth.SessionID,
		Code:      "auth-code",
		State:     auth.State,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth client is not configured")
}

func TestGrokOAuthService_RefreshAccountToken_NilOAuthClientReturnsError(t *testing.T) {
	svc := NewGrokOAuthService(nil, nil)
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"client_id":     xai.EffectiveClientID(),
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth client is not configured")
}
