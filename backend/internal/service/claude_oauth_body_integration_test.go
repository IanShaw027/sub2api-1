package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type claudeOAuthSanitizeUpstreamRecorder struct {
	lastReq  *http.Request
	lastBody []byte
}

func (u *claudeOAuthSanitizeUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *claudeOAuthSanitizeUpstreamRecorder) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.lastReq = req
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[],"usage":{"input_tokens":1,"output_tokens":1}}`)))}, nil
}

func newClaudeOAuthSanitizeTestService(upstream *claudeOAuthSanitizeUpstreamRecorder) *GatewayService {
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	return &GatewayService{
		cfg:                  cfg,
		responseHeaderFilter: compileResponseHeaderFilter(cfg),
		httpUpstream:         upstream,
	}
}

func enableClaudeAntiBanForSanitizeTest(t *testing.T, svc *GatewayService) {
	t.Helper()
	SetRuntimeAntiBanPlatforms(map[string]bool{PlatformAnthropic: true})
	t.Cleanup(func() {
		SetRuntimeAntiBanPlatforms(map[string]bool{})
	})
	svc.fingerprintNormalizer = NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{
		Enabled:           true,
		AntiBanEnabled:    true,
		EnabledByPlatform: map[string]bool{PlatformAnthropic: true},
	}, nil)
}

func newClaudeOAuthSanitizeTestContext() *gin.Context {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.0 (external, cli)")
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestBuildUpstreamRequest_SanitizesClaudeLeakFieldsForOAuthOnlyWhenAntiBanEnabled(t *testing.T) {
	SetRuntimeAntiBanPlatforms(map[string]bool{})
	t.Cleanup(func() {
		SetRuntimeAntiBanPlatforms(map[string]bool{})
	})
	upstream := &claudeOAuthSanitizeUpstreamRecorder{}
	svc := newClaudeOAuthSanitizeTestService(upstream)
	c := newClaudeOAuthSanitizeTestContext()
	body := []byte(`{"model":"claude-sonnet-4-5","baseUrl":"https://gateway.example.com","client_metadata":{"env":{"HOSTNAME":"real"}},"metadata":{"base_url":"https://gateway.example.com","keep":"ok"},"messages":[{"role":"user","content":"hello gateway text"}]}`)

	oauthAccount := &Account{ID: 701, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "tok"}, Status: StatusActive, Schedulable: true}
	_, disabledBody, err := svc.buildUpstreamRequest(context.Background(), c, oauthAccount, body, "tok", "oauth", "claude-sonnet-4-5", false, false)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(disabledBody, "baseUrl").Exists())
	require.True(t, gjson.GetBytes(disabledBody, "client_metadata").Exists())
	require.True(t, gjson.GetBytes(disabledBody, "metadata.base_url").Exists())

	enableClaudeAntiBanForSanitizeTest(t, svc)
	_, sanitized, err := svc.buildUpstreamRequest(context.Background(), c, oauthAccount, body, "tok", "oauth", "claude-sonnet-4-5", false, false)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(sanitized, "baseUrl").Exists())
	require.False(t, gjson.GetBytes(sanitized, "client_metadata").Exists())
	require.False(t, gjson.GetBytes(sanitized, "metadata.base_url").Exists())
	require.Equal(t, "ok", gjson.GetBytes(sanitized, "metadata.keep").String())
	require.Contains(t, string(sanitized), "hello gateway text")

	apiKeyAccount := &Account{ID: 702, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "key"}, Status: StatusActive, Schedulable: true}
	_, unsanitized, err := svc.buildUpstreamRequest(context.Background(), c, apiKeyAccount, body, "key", "apikey", "claude-sonnet-4-5", false, false)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(unsanitized, "baseUrl").Exists())
	require.True(t, gjson.GetBytes(unsanitized, "client_metadata").Exists())
	require.True(t, gjson.GetBytes(unsanitized, "metadata.base_url").Exists())
}

func TestBuildCountTokensRequest_SanitizesClaudeLeakFieldsForOAuthOnlyWhenAntiBanEnabled(t *testing.T) {
	SetRuntimeAntiBanPlatforms(map[string]bool{})
	t.Cleanup(func() {
		SetRuntimeAntiBanPlatforms(map[string]bool{})
	})
	upstream := &claudeOAuthSanitizeUpstreamRecorder{}
	svc := newClaudeOAuthSanitizeTestService(upstream)
	c := newClaudeOAuthSanitizeTestContext()
	body := []byte(`{"model":"claude-sonnet-4-5","base_url":"https://gateway.example.com","client_metadata":{"process":{"rss":1}},"messages":[{"role":"user","content":"hello"}]}`)

	oauthAccount := &Account{ID: 703, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "tok"}, Status: StatusActive, Schedulable: true}
	_, disabledBody, err := svc.buildCountTokensRequest(context.Background(), c, oauthAccount, body, "tok", "oauth", "claude-sonnet-4-5", false)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(disabledBody, "base_url").Exists())
	require.True(t, gjson.GetBytes(disabledBody, "client_metadata").Exists())

	enableClaudeAntiBanForSanitizeTest(t, svc)
	_, sanitized, err := svc.buildCountTokensRequest(context.Background(), c, oauthAccount, body, "tok", "oauth", "claude-sonnet-4-5", false)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(sanitized, "base_url").Exists())
	require.False(t, gjson.GetBytes(sanitized, "client_metadata").Exists())

	apiKeyAccount := &Account{ID: 704, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "key"}, Status: StatusActive, Schedulable: true}
	_, unsanitized, err := svc.buildCountTokensRequest(context.Background(), c, apiKeyAccount, body, "key", "apikey", "claude-sonnet-4-5", false)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(unsanitized, "base_url").Exists())
	require.True(t, gjson.GetBytes(unsanitized, "client_metadata").Exists())
}
