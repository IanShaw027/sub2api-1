package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
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
	doFunc             func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error)
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
	if r.doFunc != nil {
		return r.doFunc(req, proxyURL, accountID, accountConcurrency, profile)
	}
	return r.resp, r.err
}

type kiroRefresherSettingRepoStub struct {
	values map[string]string
}

func (s *kiroRefresherSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *kiroRefresherSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *kiroRefresherSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *kiroRefresherSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if s.values != nil {
			result[key] = s.values[key]
		}
	}
	return result, nil
}

func (s *kiroRefresherSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *kiroRefresherSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *kiroRefresherSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
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

func TestKiroUsageService_FetchUsageLimits_ExternalIDPSetsTokenTypeHeader(t *testing.T) {
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
		ID:       44,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_method": "external_idp",
		},
	}

	_, err := service.FetchUsageLimits(context.Background(), account, "access-token")

	require.NoError(t, err)
	require.Equal(t, "EXTERNAL_IDP", upstream.req.Header.Get("TokenType"))
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

func TestKiroTokenRefresher_Refresh_DecodeWrappedDataPayload(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"data":{
					"accessToken":"new-access-token",
					"refreshToken":"new-refresh-token",
					"profileArn":"arn:aws:kiro:wrapped",
					"expiresIn":3600
				}
			}`)),
			Header: make(http.Header),
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:       70,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "kiro-refresh-token",
		},
	}

	creds, err := refresher.Refresh(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, "new-access-token", creds["access_token"])
	require.Equal(t, "new-refresh-token", creds["refresh_token"])
	require.Equal(t, "arn:aws:kiro:wrapped", creds["profile_arn"])
}

func TestKiroTokenRefresher_Refresh_KeepsExistingRefreshTokenWhenUpstreamReturnsTruncatedValue(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"accessToken":"new-access-token",
				"refreshToken":"kiro-refresh-token...",
				"profileArn":"arn:aws:kiro:test",
				"expiresIn":3600
			}`)),
			Header: make(http.Header),
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:       71,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "kiro-refresh-token",
		},
	}

	creds, err := refresher.Refresh(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, "new-access-token", creds["access_token"])
	require.Equal(t, "kiro-refresh-token", creds["refresh_token"])
	require.Equal(t, "arn:aws:kiro:test", creds["profile_arn"])
}

func TestKiroTokenRefresher_Refresh_RejectsShortRefreshTokenBeforeRequest(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:       72,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "short",
		},
	}

	_, err := refresher.Refresh(context.Background(), account)

	require.Error(t, err)
	require.ErrorContains(t, err, "kiro refresh_token is too short")
	require.Equal(t, 0, upstream.calls)
}

func TestKiroTokenRefresher_Refresh_IDCUsesRuntimeHeaders(t *testing.T) {
	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"accessToken":"new-access-token",
				"refreshToken":"new-refresh-token",
				"expiresIn":3600
			}`)),
			Header: make(http.Header),
		},
	}
	settings := NewSettingService(&kiroRefresherSettingRepoStub{
		values: map[string]string{
			SettingKeyKiroDefaultVersion:       "0.12.0",
			SettingKeyKiroDefaultCommit:        "commit-runtime",
			SettingKeyKiroDefaultSystemVersion: "linux#6.9.0",
			SettingKeyKiroDefaultNodeVersion:   "23.1.0",
		},
	}, &config.Config{})
	refresher := NewKiroTokenRefresher().
		WithTransport(upstream, &TLSFingerprintProfileService{}).
		WithSettingService(settings)
	account := &Account{
		ID:          9,
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Concurrency: 2,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"machine_id":    "machine-3",
			"auth_method":   "builder-id",
			"client_id":     "client-id",
			"client_secret": "client-secret",
		},
	}

	creds, err := refresher.Refresh(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "new-access-token", creds["access_token"])
	require.NotNil(t, upstream.req)
	require.Contains(t, upstream.req.URL.String(), "oidc.")
	require.Contains(t, upstream.req.Header.Get("x-amz-user-agent"), "KiroIDE-0.12.0-")
	require.Contains(t, upstream.req.Header.Get("User-Agent"), "os/linux#6.9.0")
	require.Contains(t, upstream.req.Header.Get("User-Agent"), "md/nodejs#23.1.0")
	require.Contains(t, upstream.req.Header.Get("User-Agent"), "api/sso-oidc#3.738.0")
	require.Equal(t, "commit-runtime", upstream.req.Header.Get("x-amzn-kiro-commit"))
}

func TestKiroTokenRefresher_Refresh_IDCUsesJSONTokenPayload(t *testing.T) {
	var requestBody string
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			requestBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"accessToken":"new-access-token",
					"refreshToken":"new-refresh-token",
					"expiresIn":3600
				}`)),
				Header: make(http.Header),
			}, nil
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:       12,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"auth_method":   "idc",
			"client_id":     "client-id",
			"client_secret": "client-secret",
		},
	}

	_, err := refresher.Refresh(context.Background(), account)

	require.NoError(t, err)
	require.NotNil(t, upstream.req)
	require.Contains(t, upstream.req.URL.String(), "oidc.")
	require.Equal(t, "application/json", upstream.req.Header.Get("Content-Type"))

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(requestBody), &payload))
	require.Equal(t, map[string]any{
		"clientId":     "client-id",
		"clientSecret": "client-secret",
		"grantType":    "refresh_token",
		"refreshToken": "refresh-token",
	}, payload)
}

func TestKiroTokenRefresher_Refresh_ExternalIDPUsesFormTokenEndpoint(t *testing.T) {
	var requestBody string
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			requestBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"access_token":"new-ms-access-token",
					"refresh_token":"new-ms-refresh-token",
					"expires_in":3600,
					"token_type":"Bearer",
					"scope":"scope-a scope-b"
				}`)),
				Header: make(http.Header),
			}, nil
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:       13,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token":  "external-refresh-token",
			"auth_method":    "ExternalIdp",
			"client_id":      "microsoft-public-client",
			"token_endpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
			"scopes":         "scope-a scope-b",
			"profile_arn":    "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
		},
	}

	creds, err := refresher.Refresh(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, "new-ms-access-token", creds["access_token"])
	require.Equal(t, "new-ms-refresh-token", creds["refresh_token"])
	require.Equal(t, "external_idp", creds["auth_method"])
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD", creds["profile_arn"])
	require.NotNil(t, upstream.req)
	require.Equal(t, "https", upstream.req.URL.Scheme)
	require.Equal(t, "login.microsoftonline.com", upstream.req.URL.Host)
	require.Equal(t, "/tenant/oauth2/v2.0/token", upstream.req.URL.Path)
	require.Equal(t, "application/x-www-form-urlencoded", upstream.req.Header.Get("Content-Type"))
	require.NotContains(t, upstream.req.URL.String(), "oidc.")

	form, err := url.ParseQuery(requestBody)
	require.NoError(t, err)
	require.Equal(t, "refresh_token", form.Get("grant_type"))
	require.Equal(t, "microsoft-public-client", form.Get("client_id"))
	require.Equal(t, "external-refresh-token", form.Get("refresh_token"))
	require.Equal(t, "scope-a scope-b", form.Get("scope"))
}

func TestKiroTokenRefresher_Refresh_ExternalIDPJoinsScopeArray(t *testing.T) {
	var requestBody string
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			requestBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"access_token":"new-ms-access-token",
					"refresh_token":"new-ms-refresh-token",
					"expires_in":3600,
					"token_type":"Bearer"
				}`)),
				Header: make(http.Header),
			}, nil
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:       14,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token":  "external-refresh-token",
			"auth_method":    "external_idp",
			"client_id":      "microsoft-public-client",
			"token_endpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
			"scopes":         []any{"scope-a", "scope-b"},
			"profile_arn":    "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
		},
	}

	_, err := refresher.Refresh(context.Background(), account)

	require.NoError(t, err)
	form, err := url.ParseQuery(requestBody)
	require.NoError(t, err)
	require.Equal(t, "scope-a scope-b", form.Get("scope"))
	require.NotEqual(t, "[scope-a scope-b]", form.Get("scope"))
}

func TestKiroTokenRefresher_Refresh_IDCDoesNotFallbackToSocial(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if strings.Contains(req.URL.Host, "oidc.") {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error":"temporarily_unavailable"}`)),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"accessToken":"fallback-access-token",
					"refreshToken":"fallback-refresh-token",
					"expiresIn":3600
				}`)),
				Header: make(http.Header),
			}, nil
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:       10,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"auth_method":   "awsidc",
			"client_id":     "client-id",
			"client_secret": "client-secret",
		},
	}

	_, err := refresher.Refresh(context.Background(), account)

	require.Error(t, err)
	require.ErrorContains(t, err, "kiro idc refresh failed")
	require.Equal(t, 1, upstream.calls)
	require.NotNil(t, upstream.req)
	require.Contains(t, upstream.req.URL.String(), "oidc.")
}

func TestKiroTokenRefresher_Refresh_InvalidGrantReturnsTerminalError(t *testing.T) {
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant","error_description":"Invalid refresh token provided"}`)),
			Header:     make(http.Header),
		},
	}
	refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
	account := &Account{
		ID:       11,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "revoked-refresh-token",
		},
	}

	_, err := refresher.Refresh(context.Background(), account)

	require.Error(t, err)
	require.True(t, infraerrors.IsUnauthorized(err))
	require.Equal(t, kiroRefreshTokenInvalidReason, infraerrors.Reason(err))
	require.ErrorContains(t, err, "invalid_grant")
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
