package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type kiroHTTPUpstreamRecorder struct {
	req                *http.Request
	proxyURL           string
	accountID          int64
	accountConcurrency int
	calls              int
	profile            *tlsfingerprint.Profile
	resp               *http.Response
	err                error
}

func (r *kiroHTTPUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return r.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (r *kiroHTTPUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	r.calls++
	r.req = req
	r.proxyURL = proxyURL
	r.accountID = accountID
	r.accountConcurrency = accountConcurrency
	r.profile = profile
	return r.resp, r.err
}

func TestKiroUsageService_FetchUsageLimits_UsesHTTPUpstreamTransport(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
				"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100}]
			}`)),
			Header: make(http.Header),
		},
	}
	service := NewKiroUsageService().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:          42,
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Concurrency: 7,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"machine_id":    "machine-1",
		},
		Extra: map[string]any{
			"enable_tls_fingerprint": true,
		},
		Proxy: &Proxy{
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     8080,
		},
	}

	limits, err := service.FetchUsageLimits(context.Background(), account, "access-token")
	if err != nil {
		t.Fatalf("FetchUsageLimits() error = %v", err)
	}
	if limits.SubscriptionTitle() != "Kiro Pro" {
		t.Fatalf("SubscriptionTitle() = %q, want %q", limits.SubscriptionTitle(), "Kiro Pro")
	}
	if upstream.req == nil {
		t.Fatal("expected upstream request to be recorded")
	}
	if upstream.req.URL == nil || !strings.Contains(upstream.req.URL.String(), "getUsageLimits") {
		t.Fatalf("request URL = %v, want getUsageLimits endpoint", upstream.req.URL)
	}
	if upstream.proxyURL != "http://127.0.0.1:8080" {
		t.Fatalf("proxyURL = %q, want %q", upstream.proxyURL, "http://127.0.0.1:8080")
	}
	if upstream.accountID != 42 {
		t.Fatalf("accountID = %d, want 42", upstream.accountID)
	}
	if upstream.accountConcurrency != 7 {
		t.Fatalf("accountConcurrency = %d, want 7", upstream.accountConcurrency)
	}
	if values := upstream.req.Header.Values("Connection"); len(values) != 0 {
		t.Fatalf("Connection header = %v, want absent", values)
	}
	if upstream.profile == nil {
		t.Fatal("expected non-nil TLS profile when Kiro TLS fingerprint is enabled")
	}
}

func TestKiroUsageService_FetchUsageLimits_EncodesProfileARNQuery(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
				"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100}]
			}`)),
			Header: make(http.Header),
		},
	}
	service := NewKiroUsageService().WithTransport(upstream, nil)
	account := &Account{
		ID:       43,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"profile_arn": "arn:aws:kiro:us-east-1:123456789012:profile/dev:alpha",
		},
	}

	_, err := service.FetchUsageLimits(context.Background(), account, "access-token")
	if err != nil {
		t.Fatalf("FetchUsageLimits() error = %v", err)
	}
	if got := upstream.req.URL.Query().Get("profileArn"); got != "arn:aws:kiro:us-east-1:123456789012:profile/dev:alpha" {
		t.Fatalf("profileArn query = %q", got)
	}
}

func TestKiroTokenRefresher_Refresh_UsesHTTPUpstreamTransport(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"accessToken":"new-access-token",
				"refreshToken":"new-refresh-token",
				"profileArn":"arn:aws:kiro:test",
				"expiresIn":3600
			}`)),
			Header: make(http.Header),
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:          7,
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Concurrency: 3,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"machine_id":    "machine-2",
		},
		Extra: map[string]any{
			"enable_tls_fingerprint": true,
		},
		Proxy: &Proxy{
			Protocol: "http",
			Host:     "127.0.0.1",
			Port:     8081,
		},
	}

	creds, err := refresher.Refresh(context.Background(), account)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if got := creds["access_token"]; got != "new-access-token" {
		t.Fatalf("access_token = %v, want %q", got, "new-access-token")
	}
	if got := creds["refresh_token"]; got != "new-refresh-token" {
		t.Fatalf("refresh_token = %v, want %q", got, "new-refresh-token")
	}
	if got := creds["profile_arn"]; got != "arn:aws:kiro:test" {
		t.Fatalf("profile_arn = %v, want %q", got, "arn:aws:kiro:test")
	}
	if upstream.req == nil {
		t.Fatal("expected upstream request to be recorded")
	}
	if upstream.req.URL == nil || !strings.Contains(upstream.req.URL.String(), "/refreshToken") {
		t.Fatalf("request URL = %v, want refreshToken endpoint", upstream.req.URL)
	}
	if upstream.proxyURL != "http://127.0.0.1:8081" {
		t.Fatalf("proxyURL = %q, want %q", upstream.proxyURL, "http://127.0.0.1:8081")
	}
	if upstream.accountID != 7 {
		t.Fatalf("accountID = %d, want 7", upstream.accountID)
	}
	if upstream.accountConcurrency != 3 {
		t.Fatalf("accountConcurrency = %d, want 3", upstream.accountConcurrency)
	}
	if values := upstream.req.Header.Values("Connection"); len(values) != 0 {
		t.Fatalf("Connection header = %v, want absent", values)
	}
	if upstream.profile == nil {
		t.Fatal("expected non-nil TLS profile when Kiro TLS fingerprint is enabled")
	}
}

func TestNewKiroSidecarHTTPClient_UsesTLSFingerprintTransportWhenEnabled(t *testing.T) {
	account := &Account{
		ID:          8,
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Concurrency: 4,
		Extra: map[string]any{
			"enable_tls_fingerprint": true,
		},
	}

	client, err := newKiroSidecarHTTPClient(account, &TLSFingerprintProfileService{}, 5*time.Second)
	if err != nil {
		t.Fatalf("newKiroSidecarHTTPClient() error = %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.DialTLSContext == nil {
		t.Fatal("expected DialTLSContext to be configured for TLS fingerprint fallback transport")
	}
	if transport.MaxConnsPerHost != 4 {
		t.Fatalf("MaxConnsPerHost = %d, want 4", transport.MaxConnsPerHost)
	}
}
