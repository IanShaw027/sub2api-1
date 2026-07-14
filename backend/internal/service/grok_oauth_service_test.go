//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

type grokOAuthClientStub struct {
	refreshResponse *xai.TokenResponse
	deviceCode      *GrokDeviceCodeResponse
	deviceToken     *xai.TokenResponse
	loginResult     *GrokPasswordLoginResult
	loginEmail      string
	loginPassword   string
	ssoResponse     *xai.TokenResponse
	exchangeCalls   int
	mu              sync.Mutex
	exchangeStarted chan struct{}
	releaseExchange chan struct{}
	onExchangeOnce  sync.Once
}

func (s *grokOAuthClientStub) ExchangeCode(context.Context, string, string, string, string, string) (*xai.TokenResponse, error) {
	s.mu.Lock()
	s.exchangeCalls++
	callNumber := s.exchangeCalls
	s.mu.Unlock()
	if callNumber == 1 && s.exchangeStarted != nil {
		s.onExchangeOnce.Do(func() { close(s.exchangeStarted) })
	}
	if callNumber == 1 && s.releaseExchange != nil {
		<-s.releaseExchange
	}
	return &xai.TokenResponse{AccessToken: "access-token", RefreshToken: "refresh-token", ExpiresIn: 3600}, nil
}

func (s *grokOAuthClientStub) RefreshToken(context.Context, string, string, string) (*xai.TokenResponse, error) {
	return s.refreshResponse, nil
}

func (s *grokOAuthClientStub) RequestDeviceCode(context.Context, string, string) (*GrokDeviceCodeResponse, error) {
	return s.deviceCode, nil
}

func (s *grokOAuthClientStub) AutoAuthorizeDeviceCode(context.Context, string, string, string) error {
	return nil
}

func (s *grokOAuthClientStub) PollDeviceToken(context.Context, string, int, int, string, string) (*xai.TokenResponse, error) {
	return s.deviceToken, nil
}

func (s *grokOAuthClientStub) LoginWithPassword(_ context.Context, email, password, _ string) (*GrokPasswordLoginResult, error) {
	s.loginEmail = email
	s.loginPassword = password
	return s.loginResult, nil
}

func (s *grokOAuthClientStub) ConvertSSOToBuild(context.Context, string, string) (*xai.TokenResponse, error) {
	return s.ssoResponse, nil
}

func TestGrokOAuthServiceRefreshTokenPreservesOriginalRefreshTokenWhenNotRotated(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		refreshResponse: &xai.TokenResponse{
			AccessToken: "new-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		},
	})
	defer svc.Stop()

	info, err := svc.RefreshToken(context.Background(), "original-refresh-token", "", "client-id")
	require.NoError(t, err)
	require.Equal(t, "new-access-token", info.AccessToken)
	require.Equal(t, "original-refresh-token", info.RefreshToken)
	require.Equal(t, "client-id", info.ClientID)
}

func TestGrokOAuthServiceRefreshTokenRejectsResponseWithoutAccessToken(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		refreshResponse: &xai.TokenResponse{
			RefreshToken: "new-refresh-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		},
	})
	defer svc.Stop()

	info, err := svc.RefreshToken(context.Background(), "original-refresh-token", "", "client-id")

	require.Nil(t, info)
	require.Error(t, err)
	require.Contains(t, err.Error(), "GROK_OAUTH_INVALID_TOKEN_RESPONSE")
}

func TestGrokOAuthServiceExchangeCodeRequiresStateForCallbackURLAndConsumesSession(t *testing.T) {
	client := &grokOAuthClientStub{}
	svc := NewGrokOAuthService(nil, client)
	defer svc.Stop()

	auth, err := svc.GenerateAuthURL(context.Background(), nil, "")
	require.NoError(t, err)

	_, err = svc.ExchangeCode(context.Background(), &GrokExchangeCodeInput{
		SessionID: auth.SessionID,
		Code:      "http://127.0.0.1:56121/callback?code=code-without-state",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GROK_OAUTH_STATE_REQUIRED")
	require.Zero(t, client.exchangeCalls)

	_, err = svc.ExchangeCode(context.Background(), &GrokExchangeCodeInput{
		SessionID: auth.SessionID,
		Code:      "code-with-state",
		State:     auth.State,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GROK_OAUTH_SESSION_NOT_FOUND")
	require.Zero(t, client.exchangeCalls)
}

func TestGrokOAuthServiceExchangeCodeRejectsConcurrentSessionReuse(t *testing.T) {
	client := &grokOAuthClientStub{
		exchangeStarted: make(chan struct{}),
		releaseExchange: make(chan struct{}),
	}
	released := false
	defer func() {
		if !released {
			close(client.releaseExchange)
		}
	}()
	svc := NewGrokOAuthService(nil, client)
	defer svc.Stop()

	auth, err := svc.GenerateAuthURL(context.Background(), nil, "")
	require.NoError(t, err)

	firstErr := make(chan error, 1)
	go func() {
		_, err := svc.ExchangeCode(context.Background(), &GrokExchangeCodeInput{
			SessionID: auth.SessionID,
			Code:      "first-code",
			State:     auth.State,
		})
		firstErr <- err
	}()
	<-client.exchangeStarted

	_, err = svc.ExchangeCode(context.Background(), &GrokExchangeCodeInput{
		SessionID: auth.SessionID,
		Code:      "second-code",
		State:     auth.State,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GROK_OAUTH_SESSION_ALREADY_USED")
	require.Equal(t, 1, client.exchangeCalls, "second concurrent callback must not redeem the same session")

	close(client.releaseExchange)
	released = true
	require.NoError(t, <-firstErr)
}

func TestGrokOAuthServiceValidateSSOTokenReturnsOAuthTokensWithoutPersistingSSO(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		ssoResponse: &xai.TokenResponse{
			AccessToken:  "access-from-sso",
			RefreshToken: "refresh-from-sso",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		},
	})
	defer svc.Stop()

	info, err := svc.ValidateSSOToken(context.Background(), "sso-token", nil)
	require.NoError(t, err)
	require.Equal(t, "access-from-sso", info.AccessToken)
	require.Equal(t, "refresh-from-sso", info.RefreshToken)
	require.Empty(t, info.SSOToken)

	creds := svc.BuildAccountCredentials(info)
	require.NotContains(t, creds, "sso_token")
}

func TestGrokOAuthServiceAuthorizePasswordUsesLoginThenSSOAuthorize(t *testing.T) {
	client := &grokOAuthClientStub{
		loginResult: &GrokPasswordLoginResult{
			Email:    "user@example.com",
			SSOToken: "password-derived-sso",
		},
		ssoResponse: &xai.TokenResponse{
			AccessToken:  "access-from-password",
			RefreshToken: "refresh-from-password",
			ExpiresIn:    3600,
		},
	}
	svc := NewGrokOAuthService(nil, client)
	defer svc.Stop()

	info, err := svc.AuthorizePassword(context.Background(), " user@example.com ", "  super-secret  ", nil)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", info.Email)
	require.Empty(t, info.SSOToken)

	creds := svc.BuildAccountCredentials(info)
	require.NotContains(t, creds, "password")
	require.NotContains(t, creds, "sso_token")
	require.Equal(t, "user@example.com", client.loginEmail)
	require.Equal(t, "  super-secret  ", client.loginPassword, "password bytes must be preserved")
}

func TestGrokOAuthServiceConvertFromSSOExtractsBuildClaims(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		ssoResponse: &xai.TokenResponse{
			AccessToken:  makeGrokOAuthJWT(map[string]any{"sub": "user-sub", "team_id": "team-1"}),
			RefreshToken: "refresh-token",
			IDToken:      makeGrokOAuthJWT(map[string]any{"email": "user@example.com"}),
			ExpiresIn:    3600,
		},
	})
	defer svc.Stop()

	info, err := svc.ConvertFromSSO(context.Background(), "sso-token", nil)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", info.Email)
	require.Equal(t, "user-sub", info.Subject)
	require.Equal(t, "team-1", info.TeamID)

	credentials := svc.BuildAccountCredentials(info)
	require.Equal(t, "user@example.com", credentials["email"])
	require.Equal(t, "user-sub", credentials["sub"])
	require.Equal(t, "team-1", credentials["team_id"])
}

func TestMergeGrokAccountCredentialsRemovesLegacySSOCookie(t *testing.T) {
	merged := MergeGrokAccountCredentials(
		map[string]any{"sso_token": "legacy-cookie", "base_url": "https://api.x.ai/v1"},
		map[string]any{"access_token": "new-access-token"},
	)

	require.NotContains(t, merged, "sso_token")
	require.Equal(t, "https://api.x.ai/v1", merged["base_url"])
	require.Equal(t, "new-access-token", merged["access_token"])
}

func makeGrokOAuthJWT(claims map[string]any) string {
	payload, _ := json.Marshal(claims)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
