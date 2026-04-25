package service

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"
)

func TestKiroOAuthServiceGenerateAuthURLUsesStoredRedirectURI(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()
	originalFindCallbackBaseURL := kiroFindCallbackBaseURLFunc
	kiroFindCallbackBaseURLFunc = func() (string, error) {
		return "http://localhost:3128", nil
	}
	t.Cleanup(func() {
		kiroFindCallbackBaseURLFunc = originalFindCallbackBaseURL
	})

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
	if redirectURI != session.RedirectURI {
		t.Fatalf("session redirect_uri mismatch: got=%q want=%q", session.RedirectURI, redirectURI)
	}
}

func TestKiroOAuthServiceExchangeCallbackUsesCallbackPathAndLoginOptionForTokenExchange(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()
	originalFindCallbackBaseURL := kiroFindCallbackBaseURLFunc
	kiroFindCallbackBaseURLFunc = func() (string, error) {
		return "http://localhost:3128", nil
	}
	t.Cleanup(func() {
		kiroFindCallbackBaseURLFunc = originalFindCallbackBaseURL
	})

	const (
		code        = "code-1"
		callbackURL = "http://localhost:3128/signin/callback?code=" + code + "&state=%s&login_option=awsidc"
	)

	result, err := svc.GenerateAuthURL(context.Background(), nil)
	if err != nil {
		t.Fatalf("GenerateAuthURL returned error: %v", err)
	}

	parsedAuthURL, err := url.Parse(result.AuthURL)
	if err != nil {
		t.Fatalf("failed to parse auth url: %v", err)
	}
	authRedirectURI := parsedAuthURL.Query().Get("redirect_uri")
	if authRedirectURI == "" {
		t.Fatal("expected auth url redirect_uri to be set")
	}
	if result.CallbackURL != authRedirectURI {
		t.Fatalf("callback_url mismatch: got=%q want=%q", result.CallbackURL, authRedirectURI)
	}

	session, ok := svc.sessionStore.Get(result.SessionID)
	if !ok {
		t.Fatal("expected kiro oauth session to be stored")
	}

	var gotRedirectURI string
	originalExchange := kiroCodeExchangeFunc
	kiroCodeExchangeFunc = func(ctx context.Context, gotCode, gotVerifier, gotRedirect, gotProxy string) (map[string]any, error) {
		if gotCode != code {
			t.Fatalf("unexpected code: got=%q want=%q", gotCode, code)
		}
		if gotVerifier != session.CodeVerifier {
			t.Fatalf("unexpected code verifier: got=%q want=%q", gotVerifier, session.CodeVerifier)
		}
		gotRedirectURI = gotRedirect
		return map[string]any{
			"refreshToken": "refresh-token",
		}, nil
	}
	t.Cleanup(func() {
		kiroCodeExchangeFunc = originalExchange
	})

	_, err = svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   result.SessionID,
		CallbackURL: fmt.Sprintf(callbackURL, url.QueryEscape(session.State)),
	})
	if err != nil {
		t.Fatalf("ExchangeCallback returned error: %v", err)
	}

	expectedRedirectURI := "http://localhost:3128/signin/callback?login_option=awsidc"
	if gotRedirectURI != expectedRedirectURI {
		t.Fatalf("token exchange redirect_uri mismatch: got=%q want=%q", gotRedirectURI, expectedRedirectURI)
	}
	if gotRedirectURI == authRedirectURI {
		t.Fatalf("token exchange should not reuse bare authorize redirect_uri: %q", gotRedirectURI)
	}
}

func TestKiroOAuthServiceExchangeCallbackUsesCallbackPathForManualFullCallback(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	const (
		sessionID       = "session-1"
		state           = "state-1"
		code            = "code-1"
		codeVerifier    = "verifier-1"
		redirectURI     = "http://localhost:3128"
		callbackBaseURL = "http://localhost:3128/alternate"
		callbackURL     = "http://localhost:3128/oauth/callback?code=" + code + "&state=" + state + "&login_option=social"
	)

	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    codeVerifier,
		RedirectURI:     redirectURI,
		CallbackBaseURL: callbackBaseURL,
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
		CallbackURL: callbackURL,
	})
	if err != nil {
		t.Fatalf("ExchangeCallback returned error: %v", err)
	}

	expectedRedirectURI := "http://localhost:3128/oauth/callback?login_option=social"
	if gotRedirectURI != expectedRedirectURI {
		t.Fatalf("token exchange redirect_uri mismatch: got=%q want=%q", gotRedirectURI, expectedRedirectURI)
	}
	if gotRedirectURI == callbackBaseURL {
		t.Fatalf("token exchange used callback parsing base as redirect_uri: %q", gotRedirectURI)
	}
}
