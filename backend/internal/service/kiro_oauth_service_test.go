package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
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

	tokenInfo, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{
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
	if tokenInfo.AuthMethod != "idc" {
		t.Fatalf("auth_method = %q, want %q", tokenInfo.AuthMethod, "idc")
	}
}

func TestKiroOAuthServiceExchangeCallbackRejectsExternalIDPWhenKiroUsageForbidden(t *testing.T) {
	usageUpstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if req.Method != http.MethodGet || req.URL.Host != "q.us-east-1.amazonaws.com" || req.URL.Path != "/getUsageLimits" {
				t.Fatalf("unexpected validation request: %s %s", req.Method, req.URL.String())
			}
			if got := req.Header.Get("TokenType"); got != "EXTERNAL_IDP" {
				t.Fatalf("TokenType header = %q, want EXTERNAL_IDP", got)
			}
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader(`{"message":"User is not authorized to make this call."}`)),
				Header:     make(http.Header),
			}, nil
		},
	}
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, usageUpstream, &TLSFingerprintProfileService{}, nil)
	defer svc.Stop()

	const (
		sessionID    = "session-external-idp"
		state        = "state-external-idp"
		code         = "code-external-idp"
		codeVerifier = "verifier-external-idp"
		redirectURI  = "http://localhost:3128"
		callbackURL  = "http://localhost:3128/signin/callback?code=" + code + "&state=" + state + "&login_option=external_idp"
	)
	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    codeVerifier,
		RedirectURI:     redirectURI,
		CallbackBaseURL: redirectURI,
		CreatedAt:       time.Now(),
	})

	originalExchange := kiroCodeExchangeFunc
	kiroCodeExchangeFunc = func(ctx context.Context, gotCode, gotVerifier, gotRedirect, gotProxy string) (map[string]any, error) {
		if gotCode != code {
			t.Fatalf("unexpected code: got=%q want=%q", gotCode, code)
		}
		return map[string]any{
			"accessToken":   "new-ms-access-token",
			"refreshToken":  "new-ms-refresh-token",
			"expiresIn":     3600,
			"authMethod":    "external_idp",
			"provider":      "ExternalIdp",
			"clientId":      "microsoft-public-client",
			"tokenEndpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
			"issuerUrl":     "https://login.microsoftonline.com/tenant/v2.0",
			"profileArn":    "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
		}, nil
	}
	t.Cleanup(func() {
		kiroCodeExchangeFunc = originalExchange
	})

	result, err := svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   sessionID,
		CallbackURL: callbackURL,
	})

	if err == nil {
		t.Fatal("expected external_idp callback to fail when Kiro rejects the refreshed credentials")
	}
	if result != nil {
		t.Fatalf("expected no token result on validation failure, got %#v", result)
	}
	if !strings.Contains(err.Error(), "Kiro rejected") {
		t.Fatalf("error %q should mention Kiro rejection", err.Error())
	}
	if usageUpstream.calls != 1 {
		t.Fatalf("usage validation calls = %d, want 1", usageUpstream.calls)
	}
}

func TestKiroOAuthServiceExchangeCallbackOrStartContinuationReturnsMicrosoftExternalIDPAuthURL(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	const (
		sessionID   = "session-ms-start"
		state       = "state-ms-start"
		redirectURI = "http://localhost:3128"
		issuerURL   = "https://login.microsoftonline.com/035247e5-0116-4d03-a92f-989b7476ca4f/v2.0"
		clientID    = "2d3c7e3b-bd70-4d35-b397-34ba184e7572"
		loginHint   = "jenspeter.bijelic@chinaq2.click"
	)
	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    "verifier-ms-start",
		RedirectURI:     redirectURI,
		CallbackBaseURL: redirectURI,
		CreatedAt:       time.Now(),
	})

	callbackURL := "http://localhost:3128/signin/callback?login_option=external_idp" +
		"&login_hint=" + url.QueryEscape(loginHint) +
		"&issuer_url=" + url.QueryEscape(issuerURL) +
		"&client_id=" + url.QueryEscape(clientID) +
		"&state=" + url.QueryEscape(state) +
		"&scopes=" + url.QueryEscape("api://"+clientID+"/codewhisperer:conversations api://"+clientID+"/codewhisperer:completions offline_access")

	progress, err := svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   sessionID,
		CallbackURL: callbackURL,
	})
	if err != nil {
		t.Fatalf("ExchangeCallbackOrStartContinuation returned error: %v", err)
	}
	if progress == nil || progress.ExternalIDP == nil {
		t.Fatal("expected external_idp authorization result")
	}
	if progress.TokenInfo != nil {
		t.Fatal("did not expect token info before Microsoft callback returns code")
	}

	authURL, err := url.Parse(progress.ExternalIDP.AuthURL)
	if err != nil {
		t.Fatalf("invalid external_idp auth_url: %v", err)
	}
	if authURL.String() == callbackURL {
		t.Fatal("external_idp continuation should return Microsoft authorize URL, not echo the Kiro callback")
	}
	if authURL.Scheme != "https" || authURL.Host != "login.microsoftonline.com" || authURL.Path != "/035247e5-0116-4d03-a92f-989b7476ca4f/oauth2/v2.0/authorize" {
		t.Fatalf("unexpected Microsoft authorize endpoint: %s", authURL.String())
	}
	query := authURL.Query()
	if query.Get("response_type") != "code" {
		t.Fatalf("response_type = %q, want code", query.Get("response_type"))
	}
	if query.Get("client_id") != clientID {
		t.Fatalf("client_id = %q, want %q", query.Get("client_id"), clientID)
	}
	if query.Get("redirect_uri") != "http://localhost/oauth/callback" {
		t.Fatalf("redirect_uri = %q", query.Get("redirect_uri"))
	}
	if strings.Contains(query.Get("redirect_uri"), "?") || strings.Contains(query.Get("redirect_uri"), "login_option=") || strings.Contains(query.Get("redirect_uri"), "login_hint=") || strings.Contains(query.Get("redirect_uri"), "issuer_url=") || strings.Contains(query.Get("redirect_uri"), "scopes=") {
		t.Fatalf("redirect_uri should be the Kiro Entra registered localhost callback without query params, got %q", query.Get("redirect_uri"))
	}
	if query.Get("state") != state {
		t.Fatalf("state = %q, want %q", query.Get("state"), state)
	}
	if query.Get("login_hint") != loginHint {
		t.Fatalf("login_hint = %q, want %q", query.Get("login_hint"), loginHint)
	}
	if !strings.Contains(query.Get("scope"), "offline_access") {
		t.Fatalf("scope should preserve requested scopes, got %q", query.Get("scope"))
	}

	stored, ok := svc.sessionStore.Get(sessionID)
	if !ok || stored.ExternalIDP == nil {
		t.Fatal("expected external_idp session material to be stored")
	}
	if stored.ExternalIDP.TokenEndpoint != "https://login.microsoftonline.com/035247e5-0116-4d03-a92f-989b7476ca4f/oauth2/v2.0/token" {
		t.Fatalf("token endpoint = %q", stored.ExternalIDP.TokenEndpoint)
	}
	if stored.ExternalIDP.RedirectURI != "http://localhost/oauth/callback" {
		t.Fatalf("stored external_idp redirect_uri = %q", stored.ExternalIDP.RedirectURI)
	}
}

func TestKiroOAuthServiceExchangeCallbackExchangesMicrosoftExternalIDPCode(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	const (
		sessionID     = "session-ms-code"
		state         = "state-ms-code"
		code          = "microsoft-code"
		clientID      = "microsoft-public-client"
		tokenEndpoint = "https://login.microsoftonline.com/tenant/oauth2/v2.0/token"
		issuerURL     = "https://login.microsoftonline.com/tenant/v2.0"
		redirectURI   = "http://localhost/oauth/callback"
	)
	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    "verifier-ms-code",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
		ExternalIDP: &KiroExternalIDPSession{
			ClientID:      clientID,
			IssuerURL:     issuerURL,
			TokenEndpoint: tokenEndpoint,
			RedirectURI:   redirectURI,
			Scopes:        []string{"scope-a", "offline_access"},
			LoginHint:     "dev@example.com",
			CreatedAt:     time.Now(),
		},
	})

	originalExchange := kiroExternalIDPCodeExchangeFunc
	kiroExternalIDPCodeExchangeFunc = func(ctx context.Context, input kiroExternalIDPCodeExchangeInput) (map[string]any, error) {
		if input.Code != code {
			t.Fatalf("code = %q, want %q", input.Code, code)
		}
		if input.ClientID != clientID {
			t.Fatalf("client_id = %q, want %q", input.ClientID, clientID)
		}
		if input.TokenEndpoint != tokenEndpoint {
			t.Fatalf("token_endpoint = %q, want %q", input.TokenEndpoint, tokenEndpoint)
		}
		if input.RedirectURI != redirectURI {
			t.Fatalf("redirect_uri = %q, want %q", input.RedirectURI, redirectURI)
		}
		return map[string]any{
			"access_token":  "ms-access-token",
			"refresh_token": "ms-refresh-token",
			"expires_in":    int64(3600),
			"profileArn":    "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
		}, nil
	}
	t.Cleanup(func() {
		kiroExternalIDPCodeExchangeFunc = originalExchange
	})

	progress, err := svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   sessionID,
		CallbackURL: "http://localhost/oauth/callback?code=" + code + "&state=" + state,
	})
	if err != nil {
		t.Fatalf("ExchangeCallbackOrStartContinuation returned error: %v", err)
	}
	if progress == nil || progress.TokenInfo == nil {
		t.Fatal("expected token info after Microsoft code exchange")
	}
	if progress.TokenInfo.AuthMethod != "external_idp" {
		t.Fatalf("auth_method = %q, want external_idp", progress.TokenInfo.AuthMethod)
	}
	if progress.TokenInfo.ClientID != clientID {
		t.Fatalf("client_id = %q, want %q", progress.TokenInfo.ClientID, clientID)
	}
	if progress.TokenInfo.TokenEndpoint != tokenEndpoint {
		t.Fatalf("token_endpoint = %q, want %q", progress.TokenInfo.TokenEndpoint, tokenEndpoint)
	}
	if progress.TokenInfo.IssuerURL != issuerURL {
		t.Fatalf("issuer_url = %q, want %q", progress.TokenInfo.IssuerURL, issuerURL)
	}
	if _, ok := svc.sessionStore.Get(sessionID); ok {
		t.Fatal("expected session to be deleted after successful Microsoft code exchange")
	}
}

func TestKiroOAuthServiceExchangeCallbackExternalIDPCodeAllowsMissingProfileARN(t *testing.T) {
	usageUpstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if req.Method != http.MethodGet || req.URL.Host != "q.us-east-1.amazonaws.com" || req.URL.Path != "/getUsageLimits" {
				t.Fatalf("unexpected validation request: %s %s", req.Method, req.URL.String())
			}
			if got := req.Header.Get("TokenType"); got != "EXTERNAL_IDP" {
				t.Fatalf("TokenType header = %q, want EXTERNAL_IDP", got)
			}
			if got := req.URL.Query().Get("profileArn"); got != "" {
				t.Fatalf("profileArn query = %q, want empty when Microsoft token response omits profileArn", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
					"usageBreakdownList":[{"currentUsageWithPrecision":1,"usageLimitWithPrecision":100}]
				}`)),
				Header: make(http.Header),
			}, nil
		},
	}
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, usageUpstream, &TLSFingerprintProfileService{}, nil)
	defer svc.Stop()

	const (
		sessionID     = "session-ms-code-no-profile"
		state         = "state-ms-code-no-profile"
		code          = "microsoft-code-no-profile"
		clientID      = "microsoft-public-client"
		tokenEndpoint = "https://login.microsoftonline.com/tenant/oauth2/v2.0/token"
		issuerURL     = "https://login.microsoftonline.com/tenant/v2.0"
		redirectURI   = "http://localhost/oauth/callback"
	)
	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    "verifier-ms-code-no-profile",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
		ExternalIDP: &KiroExternalIDPSession{
			ClientID:      clientID,
			IssuerURL:     issuerURL,
			TokenEndpoint: tokenEndpoint,
			RedirectURI:   redirectURI,
			Scopes:        []string{"scope-a", "offline_access"},
			LoginHint:     "dev@example.com",
			CreatedAt:     time.Now(),
		},
	})

	originalExchange := kiroExternalIDPCodeExchangeFunc
	kiroExternalIDPCodeExchangeFunc = func(ctx context.Context, input kiroExternalIDPCodeExchangeInput) (map[string]any, error) {
		return map[string]any{
			"access_token":  "ms-access-token",
			"refresh_token": "ms-refresh-token",
			"expires_in":    int64(3600),
		}, nil
	}
	t.Cleanup(func() {
		kiroExternalIDPCodeExchangeFunc = originalExchange
	})

	progress, err := svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   sessionID,
		CallbackURL: "http://localhost/oauth/callback?code=" + code + "&state=" + state,
	})
	if err != nil {
		t.Fatalf("ExchangeCallbackOrStartContinuation returned error: %v", err)
	}
	if progress == nil || progress.TokenInfo == nil {
		t.Fatal("expected token info after Microsoft code exchange")
	}
	if progress.TokenInfo.ProfileARN != "" {
		t.Fatalf("profile_arn = %q, want empty when Microsoft token response omits it", progress.TokenInfo.ProfileARN)
	}
	if progress.TokenInfo.PlanName != "Kiro Pro" {
		t.Fatalf("plan_name = %q, want Kiro Pro", progress.TokenInfo.PlanName)
	}
}

func TestKiroOAuthServiceExchangeCallbackExternalIDPIgnoresUnsupportedUsageProbe(t *testing.T) {
	usageUpstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if got := req.Header.Get("TokenType"); got != "EXTERNAL_IDP" {
				t.Fatalf("TokenType header = %q, want EXTERNAL_IDP", got)
			}
			if req.Method == http.MethodGet && req.URL.Host == "q.us-east-1.amazonaws.com" && req.URL.Path == "/getUsageLimits" {
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Body:       io.NopCloser(strings.NewReader(`{"error":"FEATURE_NOT_SUPPORTED"}`)),
					Header:     make(http.Header),
				}, nil
			}
			if req.Method == http.MethodPost && req.URL.Path == "/" && req.Header.Get("x-amz-target") == "AmazonCodeWhispererService.ListAvailableProfiles" {
				switch req.URL.Host {
				case "q.us-east-1.amazonaws.com":
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"profiles":[{"arn":"arn:aws:codewhisperer:us-east-1:111111111111:profile/USEAST"}]}`)),
						Header:     make(http.Header),
					}, nil
				case "q.eu-central-1.amazonaws.com":
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"profiles":[{"arn":"arn:aws:codewhisperer:eu-central-1:222222222222:profile/EUCENTRAL"}]}`)),
						Header:     make(http.Header),
					}, nil
				}
			}
			if req.Method == http.MethodGet && req.URL.Path == "/getUsageLimits" && req.URL.Host == "q.eu-central-1.amazonaws.com" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
						"subscriptionInfo":{"subscriptionTitle":"KIRO POWER"},
						"usageBreakdownList":[{"currentUsageWithPrecision":0,"usageLimitWithPrecision":10000}]
					}`)),
					Header: make(http.Header),
				}, nil
			}
			t.Fatalf("unexpected validation request: %s %s", req.Method, req.URL.String())
			return nil, nil
		},
	}
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, usageUpstream, &TLSFingerprintProfileService{}, nil)
	defer svc.Stop()

	const (
		sessionID     = "session-ms-code-unsupported-usage"
		state         = "state-ms-code-unsupported-usage"
		code          = "microsoft-code-unsupported-usage"
		clientID      = "microsoft-public-client"
		tokenEndpoint = "https://login.microsoftonline.com/tenant/oauth2/v2.0/token"
		issuerURL     = "https://login.microsoftonline.com/tenant/v2.0"
		redirectURI   = "http://localhost/oauth/callback"
	)
	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    "verifier-ms-code-unsupported-usage",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
		ExternalIDP: &KiroExternalIDPSession{
			ClientID:      clientID,
			IssuerURL:     issuerURL,
			TokenEndpoint: tokenEndpoint,
			RedirectURI:   redirectURI,
			Scopes:        []string{"scope-a", "offline_access"},
			LoginHint:     "dev@example.com",
			CreatedAt:     time.Now(),
		},
	})

	originalExchange := kiroExternalIDPCodeExchangeFunc
	kiroExternalIDPCodeExchangeFunc = func(ctx context.Context, input kiroExternalIDPCodeExchangeInput) (map[string]any, error) {
		return map[string]any{
			"access_token":  "ms-access-token",
			"refresh_token": "ms-refresh-token",
			"expires_in":    int64(3600),
		}, nil
	}
	t.Cleanup(func() {
		kiroExternalIDPCodeExchangeFunc = originalExchange
	})

	progress, err := svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   sessionID,
		CallbackURL: "http://localhost/oauth/callback?code=" + code + "&state=" + state,
	})
	if err != nil {
		t.Fatalf("ExchangeCallbackOrStartContinuation returned error: %v", err)
	}
	if progress == nil || progress.TokenInfo == nil {
		t.Fatal("expected token info after Microsoft code exchange")
	}
	if progress.TokenInfo.AuthMethod != "external_idp" {
		t.Fatalf("auth_method = %q, want external_idp", progress.TokenInfo.AuthMethod)
	}
	if progress.TokenInfo.ProfileARN != "arn:aws:codewhisperer:eu-central-1:222222222222:profile/EUCENTRAL" {
		t.Fatalf("profile_arn = %q", progress.TokenInfo.ProfileARN)
	}
	if progress.TokenInfo.APIRegion != "eu-central-1" || progress.TokenInfo.Region != "eu-central-1" {
		t.Fatalf("regions = api:%q region:%q", progress.TokenInfo.APIRegion, progress.TokenInfo.Region)
	}
	if progress.TokenInfo.PlanName != "KIRO POWER" {
		t.Fatalf("plan_name = %q, want KIRO POWER", progress.TokenInfo.PlanName)
	}
}

func TestKiroOAuthServiceExchangeCallbackExternalIDPRequiresMicrosoftClientIDAndIssuer(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	svc.sessionStore.Set("session-ms-missing", &KiroOAuthSession{
		State:           "state-ms-missing",
		CodeVerifier:    "verifier-ms-missing",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
	})

	_, err := svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   "session-ms-missing",
		CallbackURL: "http://localhost:3128/signin/callback?state=state-ms-missing&login_option=external_idp&issuer_url=https%3A%2F%2Flogin.microsoftonline.com%2Ftenant%2Fv2.0",
	})
	if err == nil {
		t.Fatal("expected missing client_id to fail")
	}
	if !strings.Contains(err.Error(), "client_id") {
		t.Fatalf("error %q should mention client_id", err.Error())
	}
}

func TestKiroTokenRefresherRejectsRefreshResponseWithoutAccessToken(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if req.Method != http.MethodPost || req.URL.Path != "/refreshToken" {
				t.Fatalf("unexpected refresh request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"refreshToken":"new-refresh-token","expiresIn":3600}`)),
				Header:     make(http.Header),
			}, nil
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})

	credentials, err := refresher.Refresh(context.Background(), &Account{
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"refresh_token": "original-refresh-token"},
		Concurrency: 1,
	})

	if err == nil {
		t.Fatal("expected malformed Kiro refresh response to fail")
	}
	if credentials != nil {
		t.Fatalf("expected no credentials on malformed response, got %#v", credentials)
	}
	if !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("error %q should mention missing access_token", err.Error())
	}
}

func TestKiroTokenRefresherPersistsStableMachineIDBeforeRefreshTokenRotates(t *testing.T) {
	oldRefresh := "original-refresh-token"
	newRefresh := "rotated-refresh-token"
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if req.Method != http.MethodPost || req.URL.Path != "/refreshToken" {
				t.Fatalf("unexpected refresh request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"accessToken":"fresh-access","refreshToken":"` + newRefresh + `","expiresIn":3600}`)),
				Header:     make(http.Header),
			}, nil
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})

	credentials, err := refresher.Refresh(context.Background(), &Account{
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"refresh_token": oldRefresh},
		Concurrency: 1,
	})

	if err != nil {
		t.Fatalf("refresh returned error: %v", err)
	}
	oldMachineID := kiropkg.GenerateMachineID("", oldRefresh)
	newMachineID := kiropkg.GenerateMachineID("", newRefresh)
	if got := strings.TrimSpace(stringCredential(credentials, "machine_id")); got != oldMachineID {
		t.Fatalf("machine_id = %q, want stable old-token machine id %q", got, oldMachineID)
	}
	if stringCredential(credentials, "machine_id") == newMachineID {
		t.Fatalf("machine_id should not be derived from rotated refresh token")
	}
}

func TestValidateAndEnrichRefreshedCredentialsForAccountPreservesMachineID(t *testing.T) {
	oldRefresh := "original-refresh-token"
	newRefresh := "rotated-refresh-token"
	oldMachineID := kiropkg.GenerateMachineID("", oldRefresh)
	newMachineID := kiropkg.GenerateMachineID("", newRefresh)

	svc := &KiroOAuthService{}
	credentials, err := svc.ValidateAndEnrichRefreshedCredentialsForAccount(context.Background(), &Account{
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": oldRefresh,
			"machine_id":    oldMachineID,
		},
	}, map[string]any{
		"refresh_token": newRefresh,
		"machine_id":    oldMachineID,
	})
	if err != nil {
		t.Fatalf("ValidateAndEnrichRefreshedCredentialsForAccount returned error: %v", err)
	}
	if got := strings.TrimSpace(stringCredential(credentials, "machine_id")); got != oldMachineID {
		t.Fatalf("machine_id = %q, want preserved %q", got, oldMachineID)
	}
	if stringCredential(credentials, "machine_id") == newMachineID {
		t.Fatalf("machine_id should not be derived from rotated refresh token")
	}
}

func TestBuildKiroTokenInfoExtractsProfileIDFromARN(t *testing.T) {
	tokenInfo := buildKiroTokenInfo(map[string]any{
		"profileArn": "arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK",
	}, url.Values{}, "social")

	if tokenInfo.ProfileID != "EHGA3GRVQMUK" {
		t.Fatalf("profile_id mismatch: got=%q", tokenInfo.ProfileID)
	}
	if tokenInfo.UserID != "EHGA3GRVQMUK" {
		t.Fatalf("user_id should use profile_id fallback, got=%q", tokenInfo.UserID)
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

func TestKiroOAuthServiceExchangeCallbackUsesSigninPathForManualQueryOnlyIDCInput(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	const (
		sessionID    = "session-query-idc"
		state        = "state-query-idc"
		code         = "code-query-idc"
		codeVerifier = "verifier-query-idc"
		redirectURI  = "http://localhost:3128"
		callbackURL  = "?code=" + code + "&state=" + state + "&login_option=awsidc"
	)

	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    codeVerifier,
		RedirectURI:     redirectURI,
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
		CallbackURL: callbackURL,
	})
	if err != nil {
		t.Fatalf("ExchangeCallback returned error: %v", err)
	}

	expectedRedirectURI := "http://localhost:3128/signin/callback?login_option=awsidc"
	if gotRedirectURI != expectedRedirectURI {
		t.Fatalf("token exchange redirect_uri mismatch: got=%q want=%q", gotRedirectURI, expectedRedirectURI)
	}
}

func TestKiroOAuthServiceExchangeCallbackOrStartContinuationStartsIDCFlowWithoutCode(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalFindCallbackBaseURL := kiroFindCallbackBaseURLFunc
	kiroFindCallbackBaseURLFunc = func() (string, error) {
		return "http://localhost:3128", nil
	}
	originalRegister := kiroIDCRegisterClientFunc
	originalStart := kiroIDCStartDeviceAuthorizationFunc
	t.Cleanup(func() {
		kiroFindCallbackBaseURLFunc = originalFindCallbackBaseURL
		kiroIDCRegisterClientFunc = originalRegister
		kiroIDCStartDeviceAuthorizationFunc = originalStart
	})

	kiroIDCRegisterClientFunc = func(ctx context.Context, input kiroIDCRegisterClientInput) (*kiroIDCRegisterClientResult, error) {
		if input.Region != "us-east-1" {
			t.Fatalf("unexpected register region: %q", input.Region)
		}
		return &kiroIDCRegisterClientResult{
			ClientID:     "client-1",
			ClientSecret: "secret-1",
		}, nil
	}
	kiroIDCStartDeviceAuthorizationFunc = func(ctx context.Context, input kiroIDCStartDeviceAuthorizationInput) (*kiroIDCStartDeviceAuthorizationResult, error) {
		if input.ClientID != "client-1" || input.ClientSecret != "secret-1" {
			t.Fatalf("unexpected client credentials: %q %q", input.ClientID, input.ClientSecret)
		}
		return &kiroIDCStartDeviceAuthorizationResult{
			DeviceCode:              "device-code-1",
			UserCode:                "USER-CODE",
			VerificationURI:         "https://device.example.com",
			VerificationURIComplete: "https://device.example.com/complete",
			ExpiresIn:               900,
			Interval:                5,
		}, nil
	}

	result, err := svc.GenerateAuthURL(context.Background(), nil)
	if err != nil {
		t.Fatalf("GenerateAuthURL returned error: %v", err)
	}
	session, ok := svc.sessionStore.Get(result.SessionID)
	if !ok {
		t.Fatal("expected kiro oauth session to be stored")
	}

	progress, err := svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   result.SessionID,
		CallbackURL: "http://localhost:3128/signin/callback?state=" + url.QueryEscape(session.State) + "&login_option=awsidc",
	})
	if err != nil {
		t.Fatalf("ExchangeCallbackOrStartContinuation returned error: %v", err)
	}
	if progress == nil || progress.Continuation == nil {
		t.Fatal("expected continuation result")
	}
	if progress.TokenInfo != nil {
		t.Fatal("did not expect token info during continuation start")
	}
	if progress.Continuation.AuthMethod != "idc" {
		t.Fatalf("continuation auth_method = %q, want idc", progress.Continuation.AuthMethod)
	}
	if progress.Continuation.Status != "authorization_pending" {
		t.Fatalf("continuation status = %q, want authorization_pending", progress.Continuation.Status)
	}

	stored, ok := svc.sessionStore.Get(result.SessionID)
	if !ok || stored.IDCContinuation == nil {
		t.Fatal("expected continuation session to be stored")
	}
	if stored.IDCContinuation.ClientID != "client-1" || stored.IDCContinuation.ClientSecret != "secret-1" {
		t.Fatalf("stored continuation client credentials mismatch: %#v", stored.IDCContinuation)
	}
}

func TestKiroOAuthServiceCompleteDeviceAuthorizationReturnsFinalIDCTokenInfo(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalComplete := kiroIDCCompleteDeviceAuthorizationFunc
	t.Cleanup(func() {
		kiroIDCCompleteDeviceAuthorizationFunc = originalComplete
	})

	kiroIDCCompleteDeviceAuthorizationFunc = func(ctx context.Context, input kiroIDCCompleteDeviceAuthorizationInput) (map[string]any, error) {
		if input.ClientID != "client-1" || input.ClientSecret != "secret-1" || input.DeviceCode != "device-code-1" {
			t.Fatalf("unexpected completion input: %#v", input)
		}
		return map[string]any{
			"accessToken":  "access-1",
			"refreshToken": "refresh-1",
			"expiresIn":    int64(3600),
		}, nil
	}

	svc.sessionStore.Set("session-device-complete", &KiroOAuthSession{
		State:           "state-1",
		CodeVerifier:    "verifier-1",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
		IDCContinuation: &KiroIDCContinuationSession{
			LoginOption:     "awsidc",
			ClientID:        "client-1",
			ClientSecret:    "secret-1",
			DeviceCode:      "device-code-1",
			Region:          "us-east-1",
			IssuerURL:       "https://oidc.us-east-1.amazonaws.com",
			Scopes:          []string{"openid", "profile"},
			LoginHint:       "dev@example.com",
			IntervalSeconds: 5,
			ExpiresAt:       time.Now().Add(10 * time.Minute),
			CreatedAt:       time.Now(),
		},
	})

	progress, err := svc.CompleteDeviceAuthorization(context.Background(), &KiroDeviceCompleteInput{
		SessionID: "session-device-complete",
	})
	if err != nil {
		t.Fatalf("CompleteDeviceAuthorization returned error: %v", err)
	}
	if progress == nil || progress.TokenInfo == nil {
		t.Fatal("expected token info result")
	}
	if progress.Continuation != nil {
		t.Fatal("did not expect continuation result after successful completion")
	}
	if progress.TokenInfo.AuthMethod != "idc" {
		t.Fatalf("token auth_method = %q, want idc", progress.TokenInfo.AuthMethod)
	}
	if progress.TokenInfo.ClientID != "client-1" || progress.TokenInfo.ClientSecret != "secret-1" {
		t.Fatalf("token client credentials mismatch: %#v", progress.TokenInfo)
	}
	if progress.TokenInfo.IssuerURL != "https://oidc.us-east-1.amazonaws.com" {
		t.Fatalf("issuer_url mismatch: %q", progress.TokenInfo.IssuerURL)
	}
	if progress.TokenInfo.LoginHint != "dev@example.com" {
		t.Fatalf("login_hint mismatch: %q", progress.TokenInfo.LoginHint)
	}
	if _, ok := svc.sessionStore.Get("session-device-complete"); ok {
		t.Fatal("expected session to be deleted after successful device completion")
	}
}

func TestKiroOAuthServiceCompleteDeviceAuthorizationReturnsPendingContinuation(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalComplete := kiroIDCCompleteDeviceAuthorizationFunc
	t.Cleanup(func() {
		kiroIDCCompleteDeviceAuthorizationFunc = originalComplete
	})

	kiroIDCCompleteDeviceAuthorizationFunc = func(ctx context.Context, input kiroIDCCompleteDeviceAuthorizationInput) (map[string]any, error) {
		return nil, &kiroIDCAPIError{
			Operation:        "kiro idc create token",
			StatusCode:       http.StatusBadRequest,
			ErrorCode:        "authorization_pending",
			ErrorDescription: "still pending",
		}
	}

	svc.sessionStore.Set("session-device-pending", &KiroOAuthSession{
		State:           "state-1",
		CodeVerifier:    "verifier-1",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
		IDCContinuation: &KiroIDCContinuationSession{
			LoginOption:     "builderid",
			ClientID:        "client-1",
			ClientSecret:    "secret-1",
			DeviceCode:      "device-code-1",
			Region:          "us-east-1",
			UserCode:        "USER-CODE",
			VerificationURI: "https://device.example.com",
			ExpiresAt:       time.Now().Add(10 * time.Minute),
			CreatedAt:       time.Now(),
		},
	})

	progress, err := svc.CompleteDeviceAuthorization(context.Background(), &KiroDeviceCompleteInput{
		SessionID: "session-device-pending",
	})
	if err != nil {
		t.Fatalf("CompleteDeviceAuthorization returned error: %v", err)
	}
	if progress == nil || progress.Continuation == nil {
		t.Fatal("expected continuation result")
	}
	if progress.Continuation.AuthMethod != "idc" {
		t.Fatalf("continuation auth_method = %q, want idc", progress.Continuation.AuthMethod)
	}
	if progress.Continuation.Status != "authorization_pending" {
		t.Fatalf("continuation status = %q, want authorization_pending", progress.Continuation.Status)
	}
}

func TestResolveKiroIDCContinuationConfig_DoesNotUseIssuerAsStartURL(t *testing.T) {
	cfg, err := resolveKiroIDCContinuationConfig(&KiroExchangeCallbackInput{
		IssuerURL: "https://oidc.us-east-1.amazonaws.com",
	}, url.Values{}, "awsidc", &KiroOAuthSession{})
	if err != nil {
		t.Fatalf("resolveKiroIDCContinuationConfig returned error: %v", err)
	}

	if cfg.StartURL != kiroIDCDefaultStartURL {
		t.Fatalf("start_url = %q, want %q", cfg.StartURL, kiroIDCDefaultStartURL)
	}
}

func TestKiroOAuthServiceEnrichRefreshedCredentialsUsesAccountProxy(t *testing.T) {
	proxyID := int64(901)
	var usageProxyURL string
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			usageProxyURL = proxyURL
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
					"usageBreakdownList":[{"currentUsageWithPrecision":1,"usageLimitWithPrecision":10,"nextDateReset":4102444800}]
				}`)),
				Header: make(http.Header),
			}, nil
		},
	}
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{
		getByIDFunc: func(ctx context.Context, id int64) (*Proxy, error) {
			if id != proxyID {
				t.Fatalf("proxy id mismatch: got=%d want=%d", id, proxyID)
			}
			return &Proxy{Protocol: "http", Host: "127.0.0.1", Port: 8091}, nil
		},
	}, upstream, nil, nil)
	defer svc.Stop()

	account := &Account{
		ID:          902,
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		ProxyID:     &proxyID,
		Concurrency: 1,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"profile_arn":   "arn:aws:kiro:us-east-1:123456789012:profile/test",
			"region":        "us-east-1",
		},
	}
	credentials := MergeCredentials(account.Credentials, map[string]any{
		"access_token": "fresh-access",
	})

	enriched := svc.EnrichRefreshedCredentialsForAccount(context.Background(), account, credentials)

	if usageProxyURL != "http://127.0.0.1:8091" {
		t.Fatalf("usage proxy URL mismatch: got=%q", usageProxyURL)
	}
	if enriched["subscription_type"] != "Kiro Pro" {
		t.Fatalf("subscription_type mismatch: got=%v", enriched["subscription_type"])
	}
}
