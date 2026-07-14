//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOAuthService_GenerateAuthURL_NilProxyRepoReturnsError(t *testing.T) {
	t.Parallel()

	svc := NewOAuthService(nil, &mockClaudeOAuthClient{})
	defer svc.Stop()

	proxyID := int64(42)
	_, err := svc.GenerateAuthURL(context.Background(), &proxyID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy repository is unavailable")
}

func TestOAuthService_ExchangeCode_NilOAuthClientReturnsError(t *testing.T) {
	t.Parallel()

	svc := NewOAuthService(&mockProxyRepoForOAuth{}, nil)
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil)
	require.NoError(t, err)

	_, err = svc.ExchangeCode(context.Background(), &ExchangeCodeInput{
		SessionID: result.SessionID,
		Code:      "auth-code",
		State:     result.State,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth client is not configured")
}

func TestOAuthService_RefreshAccountToken_NilOAuthClientReturnsError(t *testing.T) {
	t.Parallel()

	svc := NewOAuthService(&mockProxyRepoForOAuth{}, nil)
	defer svc.Stop()

	account := &Account{
		Type: AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth client is not configured")
}
