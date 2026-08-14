//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func leftoverGeminiOAuthAccount(id int64) *Account {
	return &Account{
		ID:       id,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "ya29.test-token",
			"project_id":   "project-1",
			"expires_at":   "2099-01-01T00:00:00Z",
		},
		Concurrency: 1,
	}
}

func leftoverGeminiCompatService() *GeminiMessagesCompatService {
	return &GeminiMessagesCompatService{
		tokenProvider: NewGeminiTokenProvider(nil, nil, nil),
		cfg:           &config.Config{},
	}
}

func TestOutboundProfileUserAgentUsesPayloadWhenSet(t *testing.T) {
	p := leftoverValidProfile(1, PlatformGemini, ClientFamilyGeminiCLI, "GeminiCLI/9.9.9 (profile; test)", "mid")
	require.Equal(t, "GeminiCLI/9.9.9 (profile; test)", outboundProfileUserAgent(p))
}

func TestOutboundProfileUserAgentFallsBackToFamilyDefault(t *testing.T) {
	gemini := leftoverValidProfile(1, PlatformGemini, ClientFamilyGeminiCLI, "", "mid")
	gemini.ProfilePayload = map[string]any{}
	require.Equal(t, geminicli.GeminiCLIUserAgent, outboundProfileUserAgent(gemini))

	ag := leftoverValidProfile(2, PlatformAntigravity, ClientFamilyAntigravity, "", "mid")
	ag.ProfilePayload = map[string]any{}
	ag.ClientVersion = antigravity.DefaultUserAgentVersion
	require.Equal(t, antigravity.BuildUserAgent(antigravity.DefaultUserAgentVersion), outboundProfileUserAgent(ag))
}

func TestGeminiChatCompletionsOAuthUsesProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/9.9.9 (profile; test)"
	account := leftoverGeminiOAuthAccount(1301)
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformGemini, ClientFamilyGeminiCLI, profileUA, "mid-gemini"))

	build, _ := leftoverGeminiCompatService().buildGeminiChatCompletionsUpstreamRequestFunc(
		account, "gemini-2.5-pro", []byte(`{"contents":[]}`), false, false,
	)
	req, _, err := build(context.Background())
	require.NoError(t, err)
	require.Equal(t, profileUA, req.Header.Get("User-Agent"))
	require.NotEqual(t, geminicli.GeminiCLIUserAgent, req.Header.Get("User-Agent"))
}

func TestGeminiChatCompletionsOAuthAbortsWhenProfileLoadFails(t *testing.T) {
	account := leftoverGeminiOAuthAccount(1302)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	build, _ := leftoverGeminiCompatService().buildGeminiChatCompletionsUpstreamRequestFunc(
		account, "gemini-2.5-pro", []byte(`{"contents":[]}`), false, false,
	)
	req, _, err := build(context.Background())
	require.Error(t, err)
	require.Nil(t, req)
	require.NotContains(t, err.Error(), geminicli.GeminiCLIUserAgent)
}

func TestGeminiMessagesCompatOAuthUsesProfileUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const profileUA = "GeminiCLI/8.8.8 (messages; profile)"
	account := leftoverGeminiOAuthAccount(1303)
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformGemini, ClientFamilyGeminiCLI, profileUA, "mid-messages"))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"x-request-id": []string{"gemini-req-13"}},
			Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`)),
		},
	}
	svc := leftoverGeminiCompatService()
	svc.httpUpstream = httpStub

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("User-Agent", "inbound-should-not-win")

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, httpStub.lastReq)
	require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, geminicli.GeminiCLIUserAgent, httpStub.lastReq.Header.Get("User-Agent"))
}

func TestGeminiMessagesCompatOAuthAbortsWhenProfileLoadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverGeminiOAuthAccount(1304)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"candidates":[]}`)),
		},
	}
	svc := leftoverGeminiCompatService()
	svc.httpUpstream = httpStub

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Zero(t, httpStub.calls)
}

func TestGeminiNativeOAuthUsesProfileUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const profileUA = "GeminiCLI/7.7.7 (native; profile)"
	account := leftoverGeminiOAuthAccount(1305)
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformGemini, ClientFamilyGeminiCLI, profileUA, "mid-native"))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`)),
		},
	}
	svc := leftoverGeminiCompatService()
	svc.httpUpstream = httpStub

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", bytes.NewReader(body))

	result, err := svc.ForwardNative(context.Background(), c, account, "gemini-2.5-flash", "generateContent", false, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, httpStub.lastReq)
	require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))
}

func TestAntigravityUpstreamUsesProfileUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const profileUA = "antigravity/9.9.9 windows/amd64"
	account := &Account{
		ID:       1310,
		Platform: PlatformAntigravity,
		Type:     AccountTypeUpstream,
		Credentials: map[string]any{
			"base_url": "https://upstream.example.test",
			"api_key":  "sk-test",
		},
		Concurrency: 1,
	}
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformAntigravity, ClientFamilyAntigravity, profileUA, "mid-ag"))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":1}}`)),
		},
	}
	svc := &AntigravityGatewayService{httpUpstream: httpStub}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("User-Agent", "inbound-should-not-win")

	result, err := svc.ForwardUpstream(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, httpStub.lastReq)
	require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, "inbound-should-not-win", httpStub.lastReq.Header.Get("User-Agent"))
}

func TestAntigravityUpstreamAbortsWhenProfileLoadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{
		ID:       1311,
		Platform: PlatformAntigravity,
		Type:     AccountTypeUpstream,
		Credentials: map[string]any{
			"base_url": "https://upstream.example.test",
			"api_key":  "sk-test",
		},
	}
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"usage":{}}`)),
		},
	}
	svc := &AntigravityGatewayService{httpUpstream: httpStub}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

	result, err := svc.ForwardUpstream(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Zero(t, httpStub.calls)
}

func TestBuildKiroMCPRequestUsesProfileMachineID(t *testing.T) {
	const machineID = "profile-machine-id-13"
	account := &Account{
		ID:       1320,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":       "test-token",
			"refresh_token": "should-not-derive-machine-id",
		},
	}
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformKiro, ClientFamilyKiroIDE, "", machineID))

	req, err := (&KiroGatewayService{}).buildKiroMCPRequest(
		context.Background(), account, kiroMCPToolSearch, map[string]any{"query": "hi"}, "api-key-token", DefaultKiroRuntimeSettings(),
	)
	require.NoError(t, err)
	require.Contains(t, req.Header.Get("User-Agent"), machineID)
	require.NotContains(t, req.Header.Get("User-Agent"), kiropkg.GenerateMachineID("", "should-not-derive-machine-id"))
}

func TestBuildKiroMCPRequestAbortsWhenProfileLoadFails(t *testing.T) {
	account := &Account{ID: 1321, Platform: PlatformKiro, Type: AccountTypeAPIKey}
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	req, err := (&KiroGatewayService{}).buildKiroMCPRequest(
		context.Background(), account, kiroMCPToolSearch, map[string]any{"query": "hi"}, "api-key-token", DefaultKiroRuntimeSettings(),
	)
	require.Error(t, err)
	require.Nil(t, req)
}

func TestKiroTokenRefresherUsesProfileMachineID(t *testing.T) {
	const machineID = "profile-refresh-machine-13"
	account := &Account{
		ID:       1322,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "kiro-refresh-token",
		},
		Concurrency: 1,
	}
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformKiro, ClientFamilyKiroIDE, "", machineID))

	var gotUA string
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"accessToken":"new-access-token","refreshToken":"new-refresh-token","expiresIn":3600}`)),
				Header:     make(http.Header),
			}, nil
		},
	}
	_, err := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{}).Refresh(context.Background(), account)
	require.NoError(t, err)
	require.Contains(t, gotUA, machineID)
	require.NotContains(t, gotUA, kiropkg.GenerateMachineID("", "kiro-refresh-token"))
}

func TestKiroTokenRefresherAbortsWhenProfileLoadFails(t *testing.T) {
	account := &Account{
		ID:          1323,
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"refresh_token": "kiro-refresh-token"},
		Concurrency: 1,
	}
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			t.Fatalf("should not send mixed identity: ua=%q", req.Header.Get("User-Agent"))
			return nil, errors.New("unreachable")
		},
	}
	_, err := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{}).Refresh(context.Background(), account)
	require.Error(t, err)
	require.Zero(t, upstream.calls)
}

func TestKiroUsageUsesProfileMachineID(t *testing.T) {
	const machineID = "profile-usage-machine-13"
	account := &Account{
		ID:       1324,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "access-token",
			"refresh_token": "kiro-refresh-token",
		},
	}
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformKiro, ClientFamilyKiroIDE, "", machineID))

	var gotUA string
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			gotUA = req.Header.Get("User-Agent")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"usageBreakdownList":[]}`)),
				Header:     make(http.Header),
			}, nil
		},
	}
	_, err := NewKiroUsageService().WithTransport(upstream, &TLSFingerprintProfileService{}).
		fetchUsageLimitsInRegion(context.Background(), account, "access-token", "us-east-1")
	require.NoError(t, err)
	require.Contains(t, gotUA, machineID)
	require.NotContains(t, gotUA, kiropkg.GenerateMachineID("", "kiro-refresh-token"))
}

func TestKiroUsageAbortsWhenProfileLoadFails(t *testing.T) {
	account := &Account{ID: 1325, Platform: PlatformKiro, Type: AccountTypeOAuth}
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			t.Fatalf("should not send mixed identity: ua=%q", req.Header.Get("User-Agent"))
			return nil, errors.New("unreachable")
		},
	}
	_, err := NewKiroUsageService().WithTransport(upstream, &TLSFingerprintProfileService{}).
		fetchUsageLimitsInRegion(context.Background(), account, "access-token", "us-east-1")
	require.Error(t, err)
	require.Zero(t, upstream.calls)
}
