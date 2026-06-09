package service

import (
	"context"
	"errors"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

type openaiOAuthClientRedirectStub struct {
	exchangeCalled  int32
	lastRedirectURI string
}

func (s *openaiOAuthClientRedirectStub) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*openai.TokenResponse, error) {
	atomic.AddInt32(&s.exchangeCalled, 1)
	s.lastRedirectURI = redirectURI
	return &openai.TokenResponse{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresIn:    3600,
	}, nil
}

func (s *openaiOAuthClientRedirectStub) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *openaiOAuthClientRedirectStub) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxyURL string, clientID string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

// GenerateAuthURL must reject a malicious redirect_uri that is not on the
// server-side allowlist rather than reflecting it into the authorization URL.
func TestOpenAIOAuthService_GenerateAuthURL_RejectsUntrustedRedirectURI(t *testing.T) {
	svc := NewOpenAIOAuthService(nil, &openaiOAuthClientRedirectStub{})
	defer svc.Stop()

	_, err := svc.GenerateAuthURL(context.Background(), nil, "https://attacker.example.com/callback", PlatformOpenAI)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not allowed")
}

// An allowlisted redirect_uri is accepted and stored in the session.
func TestOpenAIOAuthService_GenerateAuthURL_AcceptsAllowlistedRedirectURI(t *testing.T) {
	svc := NewOpenAIOAuthService(nil, &openaiOAuthClientRedirectStub{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, openai.DefaultRedirectURI, PlatformOpenAI)
	require.NoError(t, err)

	parsed, err := url.Parse(result.AuthURL)
	require.NoError(t, err)
	require.Equal(t, openai.DefaultRedirectURI, parsed.Query().Get("redirect_uri"))

	session, ok := svc.sessionStore.Get(result.SessionID)
	require.True(t, ok)
	require.Equal(t, openai.DefaultRedirectURI, session.RedirectURI)
}

// An empty redirect_uri falls back to the default and is stored in the session.
func TestOpenAIOAuthService_GenerateAuthURL_EmptyRedirectURIDefaults(t *testing.T) {
	svc := NewOpenAIOAuthService(nil, &openaiOAuthClientRedirectStub{})
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil, "", PlatformOpenAI)
	require.NoError(t, err)

	session, ok := svc.sessionStore.Get(result.SessionID)
	require.True(t, ok)
	require.Equal(t, openai.DefaultRedirectURI, session.RedirectURI)
}

// ExchangeCode must use the redirect_uri stored in the session, never the
// caller-supplied input value, so an attacker cannot smuggle a different
// redirect_uri to the OpenAI token endpoint to hijack the authorization code.
func TestOpenAIOAuthService_ExchangeCode_IgnoresInputRedirectURI(t *testing.T) {
	client := &openaiOAuthClientRedirectStub{}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()

	svc.sessionStore.Set("sid", &openai.OAuthSession{
		State:        "expected-state",
		CodeVerifier: "verifier",
		RedirectURI:  openai.DefaultRedirectURI,
		CreatedAt:    time.Now(),
	})

	_, err := svc.ExchangeCode(context.Background(), &OpenAIExchangeCodeInput{
		SessionID:   "sid",
		Code:        "auth-code",
		State:       "expected-state",
		RedirectURI: "https://attacker.example.com/callback",
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&client.exchangeCalled))
	// The exchange must use the session value, not the malicious input.
	require.Equal(t, openai.DefaultRedirectURI, client.lastRedirectURI)
}
