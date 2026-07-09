//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/stretchr/testify/require"
)

func TestGeminiOAuthService_ExchangeCode_NilOAuthClientReturnsError(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	svc := NewGeminiOAuthService(nil, nil, nil, &config.Config{})
	defer svc.Stop()

	svc.sessionStore.Set("test-session", &geminicli.OAuthSession{
		State:        "expected-state",
		CodeVerifier: "verifier",
		OAuthType:    "code_assist",
		CreatedAt:    time.Now(),
	})

	_, err := svc.ExchangeCode(context.Background(), &GeminiExchangeCodeInput{
		SessionID: "test-session",
		State:     "expected-state",
		Code:      "auth-code",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth client is not configured")
}

func TestGeminiOAuthService_RefreshAccountToken_NilOAuthClientReturnsError(t *testing.T) {
	svc := NewGeminiOAuthService(&mockGeminiProxyRepo{}, nil, nil, &config.Config{})
	defer svc.Stop()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"oauth_type":    "google_one",
			"tier_id":       GeminiTierGoogleAIPro,
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "oauth client is not configured")
}

func TestGeminiOAuthService_RefreshAccountToken_NilProxyRepoReturnsError(t *testing.T) {
	client := &mockGeminiOAuthClient{
		refreshTokenFunc: func(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
			return &geminicli.TokenResponse{
				AccessToken: "refreshed",
				ExpiresIn:   3600,
			}, nil
		},
	}

	svc := NewGeminiOAuthService(nil, client, nil, &config.Config{})
	defer svc.Stop()

	proxyID := int64(42)
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		ProxyID:  &proxyID,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"oauth_type":    "google_one",
			"tier_id":       GeminiTierGoogleAIPro,
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy repository is unavailable")
}
