//go:build unit

package service

import (
	"context"
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

func TestGrokOAuthServiceValidateSSOTokenReturnsOAuthTokensAndPersistsSSO(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthClientStub{
		deviceCode: &GrokDeviceCodeResponse{
			DeviceCode: "device-code",
			UserCode:   "user-code",
			ExpiresIn:  900,
			Interval:   5,
		},
		deviceToken: &xai.TokenResponse{
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
	require.Equal(t, "sso-token", info.SSOToken)

	creds := svc.BuildAccountCredentials(info)
	require.Equal(t, "sso-token", creds["sso_token"])
}

func TestGrokOAuthServiceAuthorizePasswordUsesLoginThenSSOAuthorize(t *testing.T) {
	client := &grokOAuthClientStub{
		loginResult: &GrokPasswordLoginResult{
			Email:    "user@example.com",
			SSOToken: "password-derived-sso",
		},
		deviceCode: &GrokDeviceCodeResponse{
			DeviceCode: "device-code",
			UserCode:   "user-code",
			ExpiresIn:  900,
			Interval:   5,
		},
		deviceToken: &xai.TokenResponse{
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
	require.Equal(t, "password-derived-sso", info.SSOToken)

	creds := svc.BuildAccountCredentials(info)
	require.NotContains(t, creds, "password")
	require.Equal(t, "password-derived-sso", creds["sso_token"])
	require.Equal(t, "user@example.com", client.loginEmail)
	require.Equal(t, "  super-secret  ", client.loginPassword, "password bytes must be preserved")
}
