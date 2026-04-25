//go:build unit

package service

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func TestKiroOAuthServiceGenerateAuthURLUsesStoredRedirectURI(t *testing.T) {
	t.Parallel()

	svc := NewKiroOAuthService(&mockProxyRepoForOAuth{}, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	result, err := svc.GenerateAuthURL(context.Background(), nil)
	if err != nil {
		t.Fatalf("GenerateAuthURL returned error: %v", err)
	}

	session, ok := svc.sessionStore.Get(result.SessionID)
	if !ok {
		t.Fatal("expected kiro oauth session to be stored")
	}

	parsedAuthURL, err := url.Parse(result.AuthURL)
	if err != nil {
		t.Fatalf("failed to parse auth url: %v", err)
	}

	redirectURI := parsedAuthURL.Query().Get("redirect_uri")
	if redirectURI != result.CallbackURL {
		t.Fatalf("auth url redirect_uri mismatch: got=%q want=%q", redirectURI, result.CallbackURL)
	}
	if redirectURI != session.CallbackBaseURL {
		t.Fatalf("stored redirect_uri mismatch: got=%q want=%q", session.CallbackBaseURL, redirectURI)
	}
}

func TestKiroOAuthServiceExchangeCallbackReusesAuthorizeRedirectURI(t *testing.T) {
	t.Parallel()

	svc := NewKiroOAuthService(&mockProxyRepoForOAuth{}, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	const (
		sessionID    = "session-1"
		state        = "state-1"
		code         = "code-1"
		codeVerifier = "verifier-1"
		redirectURI  = kiroOAuthDefaultCallback
	)

	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    codeVerifier,
		CallbackBaseURL: redirectURI,
		CreatedAt:       time.Now(),
	})

	var gotRedirectURI string
	originalExchange := kiroCodeExchangeFunc
	kiroCodeExchangeFunc = func(ctx context.Context, gotCode, gotVerifier, gotRedirect, gotProxy string) (map[string]any, error) {
		if gotCode != code {
			t.Fatalf("unexpected code: got=%q want=%q", gotCode, code)
		}
		if gotVerifier != codeVerifier {
			t.Fatalf("unexpected code verifier: got=%q want=%q", gotVerifier, codeVerifier)
		}
		gotRedirectURI = gotRedirect
		return map[string]any{
			"refreshToken": "refresh-token",
		}, nil
	}
	t.Cleanup(func() {
		kiroCodeExchangeFunc = originalExchange
	})

	_, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   sessionID,
		CallbackURL: "http://localhost:1455/oauth/callback?code=" + code + "&state=" + state,
	})
	if err != nil {
		t.Fatalf("ExchangeCallback returned error: %v", err)
	}

	if gotRedirectURI != redirectURI {
		t.Fatalf("token exchange redirect_uri mismatch: got=%q want=%q", gotRedirectURI, redirectURI)
	}
}
