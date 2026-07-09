//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAntigravityOAuthService_ValidateRefreshToken_NilProxyRepoReturnsError(t *testing.T) {
	svc := NewAntigravityOAuthService(nil)

	proxyID := int64(42)
	_, err := svc.ValidateRefreshToken(context.Background(), "refresh-token", &proxyID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy repository is unavailable")
}

func TestAntigravityOAuthService_RefreshAccountToken_NilProxyRepoReturnsError(t *testing.T) {
	svc := NewAntigravityOAuthService(nil)

	proxyID := int64(42)
	account := &Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeOAuth,
		ProxyID:  &proxyID,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy repository is unavailable")
}

func TestAntigravityOAuthService_FillProjectID_NilProxyRepoReturnsError(t *testing.T) {
	svc := NewAntigravityOAuthService(nil)

	proxyID := int64(42)
	account := &Account{
		Platform: PlatformAntigravity,
		Type:     AccountTypeOAuth,
		ProxyID:  &proxyID,
	}

	projectID, err := svc.FillProjectID(context.Background(), account, "access-token")
	require.Error(t, err)
	require.Empty(t, projectID)
	require.Contains(t, err.Error(), "proxy repository is unavailable")
}
