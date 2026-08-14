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
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiOutboundProfileTokenCache struct {
	token string
}

func (c *geminiOutboundProfileTokenCache) GetAccessToken(context.Context, string) (string, error) {
	return c.token, nil
}

func (c *geminiOutboundProfileTokenCache) SetAccessToken(context.Context, string, string, time.Duration) error {
	return nil
}

func (c *geminiOutboundProfileTokenCache) DeleteAccessToken(context.Context, string) error {
	return nil
}

func (c *geminiOutboundProfileTokenCache) AcquireRefreshLock(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}

func (c *geminiOutboundProfileTokenCache) ReleaseRefreshLock(context.Context, string) error {
	return nil
}

func geminiOutboundCompatService() *GeminiMessagesCompatService {
	return &GeminiMessagesCompatService{
		tokenProvider: NewGeminiTokenProvider(nil, &geminiOutboundProfileTokenCache{token: "ya29.outbound-sa"}, nil),
		cfg:           &config.Config{},
	}
}

func geminiOutboundAPIKeyAccount(id int64) *Account {
	return &Account{
		ID:       id,
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test-gemini-api-key",
		},
		Concurrency: 1,
	}
}

func geminiOutboundOAuthAIStudioAccount(id int64) *Account {
	return &Account{
		ID:       id,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "ya29.test-token",
			"expires_at":   "2099-01-01T00:00:00Z",
		},
		Concurrency: 1,
	}
}

func geminiOutboundServiceAccount(id int64) *Account {
	return &Account{
		ID:       id,
		Platform: PlatformGemini,
		Type:     AccountTypeServiceAccount,
		Credentials: map[string]any{
			"project_id":           "vertex-project",
			"location":             "us-central1",
			"service_account_json": `{"client_email":"sa@test.iam.gserviceaccount.com","private_key":"dummy-key","project_id":"vertex-project"}`,
		},
		Concurrency: 1,
	}
}

func geminiOutboundOKResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"gemini-ua"}, "Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`)),
	}
}

func geminiOutboundInstallProfile(t *testing.T, account *Account, ua string) {
	t.Helper()
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformGemini, ClientFamilyGeminiCLI, ua, "mid-gemini-ua"))
}

func TestGeminiChatCompletionsBuildersStampProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/9.9.9 (chat-completions; leftover)"
	cases := []struct {
		name    string
		account *Account
	}{
		{name: "api_key", account: geminiOutboundAPIKeyAccount(1401)},
		{name: "oauth_ai_studio", account: geminiOutboundOAuthAIStudioAccount(1402)},
		{name: "service_account", account: geminiOutboundServiceAccount(1403)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			geminiOutboundInstallProfile(t, tc.account, profileUA)
			build, _ := geminiOutboundCompatService().buildGeminiChatCompletionsUpstreamRequestFunc(
				tc.account, "gemini-2.5-flash", []byte(`{"contents":[]}`), false, false,
			)
			req, _, err := build(context.Background())
			require.NoError(t, err)
			require.NotNil(t, req)
			require.Equal(t, profileUA, req.Header.Get("User-Agent"))
		})
	}
}

func TestGeminiChatCompletionsBuildersAbortWhenProfileLoadFails(t *testing.T) {
	cases := []struct {
		name    string
		account *Account
	}{
		{name: "api_key", account: geminiOutboundAPIKeyAccount(1411)},
		{name: "oauth_ai_studio", account: geminiOutboundOAuthAIStudioAccount(1412)},
		{name: "service_account", account: geminiOutboundServiceAccount(1413)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installLeftoverOutboundProfileError(t, tc.account.ID, errors.New("identity_reject: profile store down"))
			build, _ := geminiOutboundCompatService().buildGeminiChatCompletionsUpstreamRequestFunc(
				tc.account, "gemini-2.5-flash", []byte(`{"contents":[]}`), false, false,
			)
			req, _, err := build(context.Background())
			require.Error(t, err)
			require.Nil(t, req)
			require.ErrorContains(t, err, "identity_reject")
		})
	}
}

func TestGeminiMessagesCompatAccountTypesStampProfileUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const profileUA = "GeminiCLI/8.8.8 (messages; leftover)"
	cases := []struct {
		name    string
		account *Account
	}{
		{name: "api_key", account: geminiOutboundAPIKeyAccount(1421)},
		{name: "oauth_ai_studio", account: geminiOutboundOAuthAIStudioAccount(1422)},
		{name: "service_account", account: geminiOutboundServiceAccount(1423)},
	}
	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			geminiOutboundInstallProfile(t, tc.account, profileUA)
			httpStub := &geminiCompatHTTPUpstreamStub{response: geminiOutboundOKResponse()}
			svc := geminiOutboundCompatService()
			svc.httpUpstream = httpStub

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

			result, err := svc.Forward(context.Background(), c, tc.account, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, httpStub.lastReq)
			require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))
		})
	}
}

func TestGeminiMessagesCompatAccountTypesAbortWhenProfileLoadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name    string
		account *Account
	}{
		{name: "api_key", account: geminiOutboundAPIKeyAccount(1431)},
		{name: "oauth_ai_studio", account: geminiOutboundOAuthAIStudioAccount(1432)},
		{name: "service_account", account: geminiOutboundServiceAccount(1433)},
	}
	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installLeftoverOutboundProfileError(t, tc.account.ID, errors.New("identity_reject: profile store down"))
			httpStub := &geminiCompatHTTPUpstreamStub{response: geminiOutboundOKResponse()}
			svc := geminiOutboundCompatService()
			svc.httpUpstream = httpStub

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

			result, err := svc.Forward(context.Background(), c, tc.account, body)
			require.Error(t, err)
			require.Nil(t, result)
			require.Zero(t, httpStub.calls)
		})
	}
}

func TestGeminiNativeAccountTypesStampProfileUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const profileUA = "GeminiCLI/7.7.7 (native; leftover)"
	cases := []struct {
		name    string
		account *Account
	}{
		{name: "api_key", account: geminiOutboundAPIKeyAccount(1441)},
		{name: "oauth_ai_studio", account: geminiOutboundOAuthAIStudioAccount(1442)},
		{name: "service_account", account: geminiOutboundServiceAccount(1443)},
	}
	body := []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			geminiOutboundInstallProfile(t, tc.account, profileUA)
			httpStub := &geminiCompatHTTPUpstreamStub{response: geminiOutboundOKResponse()}
			svc := geminiOutboundCompatService()
			svc.httpUpstream = httpStub

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", bytes.NewReader(body))

			result, err := svc.ForwardNative(context.Background(), c, tc.account, "gemini-2.5-flash", "generateContent", false, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, httpStub.lastReq)
			require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))
		})
	}
}

func TestGeminiNativeAccountTypesAbortWhenProfileLoadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name    string
		account *Account
	}{
		{name: "api_key", account: geminiOutboundAPIKeyAccount(1451)},
		{name: "oauth_ai_studio", account: geminiOutboundOAuthAIStudioAccount(1452)},
		{name: "service_account", account: geminiOutboundServiceAccount(1453)},
	}
	body := []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installLeftoverOutboundProfileError(t, tc.account.ID, errors.New("identity_reject: profile store down"))
			httpStub := &geminiCompatHTTPUpstreamStub{response: geminiOutboundOKResponse()}
			svc := geminiOutboundCompatService()
			svc.httpUpstream = httpStub

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", bytes.NewReader(body))

			result, err := svc.ForwardNative(context.Background(), c, tc.account, "gemini-2.5-flash", "generateContent", false, body)
			require.Error(t, err)
			require.Nil(t, result)
			require.Zero(t, httpStub.calls)
		})
	}
}

func TestForwardAIStudioGETStampsProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/6.6.6 (models-get; leftover)"
	cases := []struct {
		name    string
		account *Account
	}{
		{name: "api_key", account: geminiOutboundAPIKeyAccount(1461)},
		{name: "oauth", account: geminiOutboundOAuthAIStudioAccount(1462)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			geminiOutboundInstallProfile(t, tc.account, profileUA)
			httpStub := &geminiCompatHTTPUpstreamStub{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"models":[]}`)),
				},
			}
			svc := geminiOutboundCompatService()
			svc.httpUpstream = httpStub

			result, err := svc.ForwardAIStudioGET(context.Background(), tc.account, "/v1beta/models")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, httpStub.lastReq)
			require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))
		})
	}
}

func TestForwardAIStudioGETAbortsWhenProfileLoadFails(t *testing.T) {
	cases := []struct {
		name    string
		account *Account
	}{
		{name: "api_key", account: geminiOutboundAPIKeyAccount(1471)},
		{name: "oauth", account: geminiOutboundOAuthAIStudioAccount(1472)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installLeftoverOutboundProfileError(t, tc.account.ID, errors.New("identity_reject: profile store down"))
			httpStub := &geminiCompatHTTPUpstreamStub{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"models":[]}`)),
				},
			}
			svc := geminiOutboundCompatService()
			svc.httpUpstream = httpStub

			result, err := svc.ForwardAIStudioGET(context.Background(), tc.account, "/v1beta/models")
			require.Error(t, err)
			require.Nil(t, result)
			require.Zero(t, httpStub.calls)
			require.ErrorContains(t, err, "identity_reject")
		})
	}
}
