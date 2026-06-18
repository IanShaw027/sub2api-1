package service

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// badKiroProxyURL contains a control character so url.Parse (and therefore
// buildKiroOAuthTransport) fails, exercising the transport-error path.
const badKiroProxyURL = "http://\x7f-bad-proxy:8080"

// --- C1: proxy transport build errors must be hard errors, not silently
// swallowed (which would bypass the configured proxy and leak credentials to
// an unintended egress). ---

func TestExchangeKiroCodeForTokenFailsWhenProxyTransportInvalid(t *testing.T) {
	_, err := exchangeKiroCodeForToken(context.Background(), "code-1", "verifier-1", "http://localhost:3128/oauth/callback", badKiroProxyURL)
	if err == nil {
		t.Fatal("expected error when proxy transport cannot be built")
	}
	if !strings.Contains(err.Error(), "transport") {
		t.Fatalf("expected transport error, got %q", err.Error())
	}
}

func TestDoKiroIDCJSONRequestFailsWhenProxyTransportInvalid(t *testing.T) {
	err := doKiroIDCJSONRequest(context.Background(), badKiroProxyURL, "https://oidc.us-east-1.amazonaws.com/token", map[string]any{"k": "v"}, &map[string]any{}, "kiro idc test op")
	if err == nil {
		t.Fatal("expected error when proxy transport cannot be built")
	}
	if !strings.Contains(err.Error(), "transport") {
		t.Fatalf("expected transport error, got %q", err.Error())
	}
}

// --- C3-a: issuer_url / start_url / region from the untrusted callback must be
// validated against the AWS allowlist before they are used to register an IDC
// client. ---

func TestResolveKiroIDCContinuationConfigAllowsAWSDomains(t *testing.T) {
	cfg, err := resolveKiroIDCContinuationConfig(&KiroExchangeCallbackInput{
		IssuerURL: "https://oidc.eu-west-1.amazonaws.com",
		StartURL:  "https://my-org.awsapps.com/start",
		IDCRegion: "eu-west-1",
	}, url.Values{}, "awsidc", &KiroOAuthSession{})
	if err != nil {
		t.Fatalf("expected valid AWS config to be accepted, got error: %v", err)
	}
	if cfg.Region != "eu-west-1" {
		t.Fatalf("region = %q, want eu-west-1", cfg.Region)
	}
	if cfg.StartURL != "https://my-org.awsapps.com/start" {
		t.Fatalf("start_url = %q", cfg.StartURL)
	}
}

func TestResolveKiroIDCContinuationConfigRejectsForgedIssuer(t *testing.T) {
	query := url.Values{}
	query.Set("issuer_url", "https://oidc.us-east-1.attacker.example.com")
	query.Set("start_url", "https://view.awsapps.com/start")
	_, err := resolveKiroIDCContinuationConfig(&KiroExchangeCallbackInput{}, query, "awsidc", &KiroOAuthSession{})
	if err == nil {
		t.Fatal("expected forged issuer_url to be rejected")
	}
	if !strings.Contains(err.Error(), "issuer_url") {
		t.Fatalf("expected issuer_url error, got %q", err.Error())
	}
}

func TestResolveKiroIDCContinuationConfigRejectsIssuerWithExtraLabels(t *testing.T) {
	_, err := resolveKiroIDCContinuationConfig(&KiroExchangeCallbackInput{
		IssuerURL: "https://oidc.evil.us-east-1.amazonaws.com",
		StartURL:  "https://view.awsapps.com/start",
		IDCRegion: "us-east-1",
	}, url.Values{}, "awsidc", &KiroOAuthSession{})
	if err == nil {
		t.Fatal("expected issuer_url with extra labels to be rejected")
	}
}

func TestResolveKiroIDCContinuationConfigRejectsForgedStartURL(t *testing.T) {
	query := url.Values{}
	query.Set("start_url", "https://view.awsapps.com.attacker.example.com/start")
	_, err := resolveKiroIDCContinuationConfig(&KiroExchangeCallbackInput{}, query, "awsidc", &KiroOAuthSession{})
	if err == nil {
		t.Fatal("expected forged start_url to be rejected")
	}
	if !strings.Contains(err.Error(), "start_url") {
		t.Fatalf("expected start_url error, got %q", err.Error())
	}
}

func TestResolveKiroIDCContinuationConfigRejectsLookalikeStartURL(t *testing.T) {
	_, err := resolveKiroIDCContinuationConfig(&KiroExchangeCallbackInput{
		StartURL:  "https://evil-awsapps.com/start",
		IDCRegion: "us-east-1",
	}, url.Values{}, "awsidc", &KiroOAuthSession{})
	if err == nil {
		t.Fatal("expected lookalike start_url host to be rejected")
	}
}

func TestResolveKiroIDCContinuationConfigRejectsUnknownRegion(t *testing.T) {
	query := url.Values{}
	query.Set("idc_region", "moon-base-1")
	query.Set("start_url", "https://view.awsapps.com/start")
	_, err := resolveKiroIDCContinuationConfig(&KiroExchangeCallbackInput{}, query, "awsidc", &KiroOAuthSession{})
	if err == nil {
		t.Fatal("expected unknown region to be rejected")
	}
	if !strings.Contains(err.Error(), "region") {
		t.Fatalf("expected region error, got %q", err.Error())
	}
}

// startOrResumeIDCContinuation must not register an IDC client when the
// continuation config fails allowlist validation.
func TestStartContinuationRejectsForgedIssuerWithoutRegistering(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalFindCallbackBaseURL := kiroFindCallbackBaseURLFunc
	originalRegister := kiroIDCRegisterClientFunc
	originalStart := kiroIDCStartDeviceAuthorizationFunc
	kiroFindCallbackBaseURLFunc = func() (string, error) { return "http://localhost:3128", nil }
	registerCalls := int32(0)
	kiroIDCRegisterClientFunc = func(ctx context.Context, input kiroIDCRegisterClientInput) (*kiroIDCRegisterClientResult, error) {
		atomic.AddInt32(&registerCalls, 1)
		return &kiroIDCRegisterClientResult{ClientID: "c", ClientSecret: "s"}, nil
	}
	kiroIDCStartDeviceAuthorizationFunc = func(ctx context.Context, input kiroIDCStartDeviceAuthorizationInput) (*kiroIDCStartDeviceAuthorizationResult, error) {
		return &kiroIDCStartDeviceAuthorizationResult{DeviceCode: "d"}, nil
	}
	t.Cleanup(func() {
		kiroFindCallbackBaseURLFunc = originalFindCallbackBaseURL
		kiroIDCRegisterClientFunc = originalRegister
		kiroIDCStartDeviceAuthorizationFunc = originalStart
	})

	result, err := svc.GenerateAuthURL(context.Background(), nil)
	if err != nil {
		t.Fatalf("GenerateAuthURL error: %v", err)
	}
	session, _ := svc.sessionStore.Get(result.SessionID)

	callbackURL := "http://localhost:3128/signin/callback?state=" + url.QueryEscape(session.State) +
		"&login_option=awsidc&issuer_url=" + url.QueryEscape("https://oidc.us-east-1.attacker.example.com")
	_, err = svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   result.SessionID,
		CallbackURL: callbackURL,
	})
	if err == nil {
		t.Fatal("expected forged issuer to be rejected")
	}
	if atomic.LoadInt32(&registerCalls) != 0 {
		t.Fatalf("expected no IDC client registration on rejected issuer, got %d calls", registerCalls)
	}
}

// --- C4-a: a session's single-use authorization code must not be redeemable
// twice, even under concurrent callbacks. ---

func TestExchangeCallbackRejectsReplayedSession(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalExchange := kiroCodeExchangeFunc
	exchangeCalls := int32(0)
	kiroCodeExchangeFunc = func(ctx context.Context, code, verifier, redirect, proxy string) (map[string]any, error) {
		atomic.AddInt32(&exchangeCalls, 1)
		return map[string]any{"refreshToken": "refresh"}, nil
	}
	t.Cleanup(func() { kiroCodeExchangeFunc = originalExchange })

	const (
		sessionID = "session-replay"
		state     = "state-replay"
	)
	session := &KiroOAuthSession{
		State:           state,
		CodeVerifier:    "verifier",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
	}
	svc.sessionStore.Set(sessionID, session)

	callbackURL := "http://localhost:3128/oauth/callback?code=code-1&state=" + state + "&login_option=social"
	if _, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{SessionID: sessionID, CallbackURL: callbackURL}); err != nil {
		t.Fatalf("first ExchangeCallback error: %v", err)
	}
	if _, ok := svc.sessionStore.Get(sessionID); ok {
		t.Fatal("session should have been deleted after successful exchange")
	}

	// Simulate a leaked-but-still-present session (e.g. delete lost / replayed
	// before cleanup): re-add the same consumed pointer and replay it.
	svc.sessionStore.Set(sessionID, session)
	_, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{SessionID: sessionID, CallbackURL: callbackURL})
	if err == nil {
		t.Fatal("expected replay of a consumed session to be rejected")
	}
	if got := atomic.LoadInt32(&exchangeCalls); got != 1 {
		t.Fatalf("expected exactly one code exchange across replay, got %d", got)
	}
}

func TestExchangeCallbackRejectsAbsoluteCallbackFromDifferentHost(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalExchange := kiroCodeExchangeFunc
	exchangeCalls := int32(0)
	kiroCodeExchangeFunc = func(ctx context.Context, code, verifier, redirect, proxy string) (map[string]any, error) {
		atomic.AddInt32(&exchangeCalls, 1)
		return map[string]any{"refreshToken": "refresh"}, nil
	}
	t.Cleanup(func() { kiroCodeExchangeFunc = originalExchange })

	const (
		sessionID = "session-host-bind"
		state     = "state-host-bind"
	)
	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    "verifier",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
	})

	callbackURL := "https://attacker.example/oauth/callback?code=code-1&state=" + state + "&login_option=social"
	_, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{SessionID: sessionID, CallbackURL: callbackURL})
	if err == nil {
		t.Fatal("expected absolute callback from a different host to be rejected")
	}
	if got := atomic.LoadInt32(&exchangeCalls); got != 0 {
		t.Fatalf("expected rejected callback not to exchange code, got %d calls", got)
	}
}

func TestExchangeCallbackConcurrentRedeemConsumesOnce(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalExchange := kiroCodeExchangeFunc
	exchangeCalls := int32(0)
	kiroCodeExchangeFunc = func(ctx context.Context, code, verifier, redirect, proxy string) (map[string]any, error) {
		atomic.AddInt32(&exchangeCalls, 1)
		time.Sleep(5 * time.Millisecond)
		return map[string]any{"refreshToken": "refresh"}, nil
	}
	t.Cleanup(func() { kiroCodeExchangeFunc = originalExchange })

	const (
		sessionID = "session-concurrent"
		state     = "state-concurrent"
	)
	svc.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    "verifier",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
	})

	callbackURL := "http://localhost:3128/oauth/callback?code=code-1&state=" + state + "&login_option=social"

	var wg sync.WaitGroup
	successes := int32(0)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{SessionID: sessionID, CallbackURL: callbackURL})
			if err == nil {
				atomic.AddInt32(&successes, 1)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&exchangeCalls); got != 1 {
		t.Fatalf("expected exactly one code exchange, got %d", got)
	}
	if got := atomic.LoadInt32(&successes); got != 1 {
		t.Fatalf("expected exactly one successful callback, got %d", got)
	}
}

// --- C4-b: concurrent IDC continuation starts on the same session must
// register exactly one client (no lost update / orphan client). ---

func TestStartContinuationConcurrentRegistersOnce(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalFindCallbackBaseURL := kiroFindCallbackBaseURLFunc
	originalRegister := kiroIDCRegisterClientFunc
	originalStart := kiroIDCStartDeviceAuthorizationFunc
	kiroFindCallbackBaseURLFunc = func() (string, error) { return "http://localhost:3128", nil }
	registerCalls := int32(0)
	kiroIDCRegisterClientFunc = func(ctx context.Context, input kiroIDCRegisterClientInput) (*kiroIDCRegisterClientResult, error) {
		n := atomic.AddInt32(&registerCalls, 1)
		time.Sleep(5 * time.Millisecond)
		return &kiroIDCRegisterClientResult{
			ClientID:     "client-" + string(rune('0'+n)),
			ClientSecret: "secret",
		}, nil
	}
	kiroIDCStartDeviceAuthorizationFunc = func(ctx context.Context, input kiroIDCStartDeviceAuthorizationInput) (*kiroIDCStartDeviceAuthorizationResult, error) {
		return &kiroIDCStartDeviceAuthorizationResult{
			DeviceCode: "device-code",
			ExpiresIn:  900,
			Interval:   5,
		}, nil
	}
	t.Cleanup(func() {
		kiroFindCallbackBaseURLFunc = originalFindCallbackBaseURL
		kiroIDCRegisterClientFunc = originalRegister
		kiroIDCStartDeviceAuthorizationFunc = originalStart
	})

	result, err := svc.GenerateAuthURL(context.Background(), nil)
	if err != nil {
		t.Fatalf("GenerateAuthURL error: %v", err)
	}
	session, _ := svc.sessionStore.Get(result.SessionID)
	callbackURL := "http://localhost:3128/signin/callback?state=" + url.QueryEscape(session.State) + "&login_option=awsidc"

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.ExchangeCallbackOrStartContinuation(context.Background(), &KiroExchangeCallbackInput{
				SessionID:   result.SessionID,
				CallbackURL: callbackURL,
			})
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&registerCalls); got != 1 {
		t.Fatalf("expected exactly one IDC client registration, got %d", got)
	}

	stored, ok := svc.sessionStore.Get(result.SessionID)
	if !ok || stored.IDCContinuation == nil {
		t.Fatal("expected a stored continuation")
	}
	if stored.IDCContinuation.ClientID != "client-1" {
		t.Fatalf("stored continuation should reference the single registered client, got %q", stored.IDCContinuation.ClientID)
	}
}
