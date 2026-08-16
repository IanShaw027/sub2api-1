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
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
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

func leftoverGeminiTestService() *AccountTestService {
	return &AccountTestService{
		cfg:                 upstreamModelSyncTestConfig(),
		geminiTokenProvider: NewGeminiTokenProvider(nil, &geminiOutboundProfileTokenCache{token: "ya29.outbound-sa"}, nil),
	}
}

func TestBuildGeminiUpstreamModelsRequestStampsProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/5.5.5 (models-sync; leftover)"
	account := geminiOutboundAPIKeyAccount(1501)
	geminiOutboundInstallProfile(t, account, profileUA)

	req, err := leftoverGeminiTestService().buildGeminiUpstreamModelsRequest(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, profileUA, req.Header.Get("User-Agent"))
	require.NotEqual(t, geminicli.GeminiCLIUserAgent, req.Header.Get("User-Agent"))
}

func TestBuildGeminiUpstreamModelsRequestAbortsWhenProfileLoadFails(t *testing.T) {
	account := geminiOutboundAPIKeyAccount(1502)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	req, err := leftoverGeminiTestService().buildGeminiUpstreamModelsRequest(context.Background(), account)
	require.Error(t, err)
	require.Nil(t, req)
	require.ErrorContains(t, err, "identity_reject")
}

func TestBuildGeminiAPIKeyRequestStampsProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/5.5.4 (account-test; leftover)"
	account := geminiOutboundAPIKeyAccount(1511)
	geminiOutboundInstallProfile(t, account, profileUA)

	req, err := leftoverGeminiTestService().buildGeminiAPIKeyRequest(context.Background(), account, "gemini-2.5-flash", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.Equal(t, profileUA, req.Header.Get("User-Agent"))
}

func TestBuildGeminiAPIKeyRequestAbortsWhenProfileLoadFails(t *testing.T) {
	account := geminiOutboundAPIKeyAccount(1512)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	req, err := leftoverGeminiTestService().buildGeminiAPIKeyRequest(context.Background(), account, "gemini-2.5-flash", []byte(`{"contents":[]}`))
	require.Error(t, err)
	require.Nil(t, req)
	require.ErrorContains(t, err, "identity_reject")
}

func TestBuildGeminiOAuthAIStudioRequestStampsProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/5.5.3 (oauth-aistudio; leftover)"
	account := geminiOutboundOAuthAIStudioAccount(1521)
	geminiOutboundInstallProfile(t, account, profileUA)

	req, err := leftoverGeminiTestService().buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-flash", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.Equal(t, profileUA, req.Header.Get("User-Agent"))
}

func TestBuildGeminiServiceAccountRequestStampsProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/5.5.2 (service-account; leftover)"
	account := geminiOutboundServiceAccount(1531)
	geminiOutboundInstallProfile(t, account, profileUA)

	req, err := leftoverGeminiTestService().buildGeminiServiceAccountRequest(context.Background(), account, "gemini-2.5-flash", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.Equal(t, profileUA, req.Header.Get("User-Agent"))
}

func TestBuildCodeAssistRequestStampsProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/5.5.1 (code-assist; leftover)"
	account := leftoverGeminiOAuthAccount(1541)
	geminiOutboundInstallProfile(t, account, profileUA)

	req, err := leftoverGeminiTestService().buildCodeAssistRequest(context.Background(), account, "ya29.test", "project-1", "gemini-2.5-flash", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.Equal(t, profileUA, req.Header.Get("User-Agent"))
	require.NotEqual(t, geminicli.GeminiCLIUserAgent, req.Header.Get("User-Agent"))
}

func TestBuildCodeAssistRequestAbortsWhenProfileLoadFails(t *testing.T) {
	account := leftoverGeminiOAuthAccount(1542)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	req, err := leftoverGeminiTestService().buildCodeAssistRequest(context.Background(), account, "ya29.test", "project-1", "gemini-2.5-flash", []byte(`{"contents":[]}`))
	require.Error(t, err)
	require.Nil(t, req)
	require.ErrorContains(t, err, "identity_reject")
}

func TestGeminiBatchSubmitStampsProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/5.4.9 (batch-image; leftover)"
	account := geminiOutboundAPIKeyAccount(1551)
	geminiOutboundInstallProfile(t, account, profileUA)

	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/upload/"):
			_, _ = w.Write([]byte(`{"file":{"name":"files/input-jsonl"}}`))
		default:
			_, _ = w.Write([]byte(`{"name":"batches/job-123","state":"JOB_STATE_PENDING"}`))
		}
	}))
	t.Cleanup(server.Close)

	provider := NewGeminiAPIBatchImageProvider(NewGeminiBatchHTTPClient(server.URL, server.Client()))
	got, err := provider.Submit(context.Background(), nil, account, validGeminiBatchInput())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, profileUA, gotUA)
}

func TestGeminiBatchSubmitAbortsWhenProfileLoadFails(t *testing.T) {
	account := geminiOutboundAPIKeyAccount(1552)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	client := &fakeGeminiBatchClient{
		uploaded: &GeminiUploadedFile{Name: "files/input-jsonl"},
		created:  &GeminiBatchJob{Name: "batches/job-123", State: "JOB_STATE_PENDING"},
	}
	provider := NewGeminiAPIBatchImageProvider(client)
	got, err := provider.Submit(context.Background(), nil, account, validGeminiBatchInput())
	require.Error(t, err)
	require.Nil(t, got)
	require.ErrorContains(t, err, "identity_reject")
	require.Empty(t, client.calls)
}

func TestFetchProjectIDUsesAccountProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/9.1.1 (code-assist-fetch; leftover)"
	account := leftoverGeminiOAuthAccount(1561)
	geminiOutboundInstallProfile(t, account, profileUA)

	var gotUA string
	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(_ context.Context, _, _, userAgent string, _ *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			gotUA = userAgent
			return &geminicli.LoadCodeAssistResponse{
				CloudAICompanionProject: "auto-project-123",
				CurrentTier:             &geminicli.TierInfo{ID: "STANDARD"},
			}, nil
		},
	}
	svc := NewGeminiOAuthService(nil, nil, codeAssist, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	_, _, err := svc.fetchProjectID(context.Background(), "ya29.test", "", account)
	require.NoError(t, err)
	require.Equal(t, profileUA, gotUA)
	require.NotEqual(t, geminicli.GeminiCLIUserAgent, gotUA)
}

func TestFetchProjectIDWithoutAccountUsesDefaultUserAgent(t *testing.T) {
	var gotUA string
	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(_ context.Context, _, _, userAgent string, _ *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			gotUA = userAgent
			return &geminicli.LoadCodeAssistResponse{
				CloudAICompanionProject: "auto-project-123",
				CurrentTier:             &geminicli.TierInfo{ID: "STANDARD"},
			}, nil
		},
	}
	svc := NewGeminiOAuthService(nil, nil, codeAssist, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	_, _, err := svc.fetchProjectID(context.Background(), "ya29.test", "", nil)
	require.NoError(t, err)
	require.Equal(t, geminicli.GeminiCLIUserAgent, gotUA)
}

func TestFetchProjectIDResourceManagerFallbackUsesAccountProfileUserAgent(t *testing.T) {
	const profileUA = "GeminiCLI/9.1.1 (crm-fallback; leftover)"
	account := leftoverGeminiOAuthAccount(1571)
	geminiOutboundInstallProfile(t, account, profileUA)

	var gotUA string
	leftoverGeminiResourceManagerRoundTrip = func(req *http.Request) (*http.Response, error) {
		gotUA = req.Header.Get("User-Agent")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"projects":[{"projectId":"crm-project-1","name":"default","lifecycleState":"ACTIVE"}]}`)),
		}, nil
	}
	t.Cleanup(func() { leftoverGeminiResourceManagerRoundTrip = nil })

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(_ context.Context, _, _, _ string, _ *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				CurrentTier: &geminicli.TierInfo{ID: "STANDARD"},
			}, nil
		},
	}
	svc := NewGeminiOAuthService(nil, nil, codeAssist, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	project, _, err := svc.fetchProjectID(context.Background(), "ya29.test", "", account)
	require.NoError(t, err)
	require.Equal(t, "crm-project-1", project)
	require.Equal(t, profileUA, gotUA)
	require.NotEqual(t, geminicli.GeminiCLIUserAgent, gotUA)
}

func TestFetchProjectIDResourceManagerFallbackWithoutAccountUsesDefaultUserAgent(t *testing.T) {
	var gotUA string
	leftoverGeminiResourceManagerRoundTrip = func(req *http.Request) (*http.Response, error) {
		gotUA = req.Header.Get("User-Agent")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"projects":[{"projectId":"crm-project-1","name":"default","lifecycleState":"ACTIVE"}]}`)),
		}, nil
	}
	t.Cleanup(func() { leftoverGeminiResourceManagerRoundTrip = nil })

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(_ context.Context, _, _, _ string, _ *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			return &geminicli.LoadCodeAssistResponse{
				CurrentTier: &geminicli.TierInfo{ID: "STANDARD"},
			}, nil
		},
	}
	svc := NewGeminiOAuthService(nil, nil, codeAssist, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	project, _, err := svc.fetchProjectID(context.Background(), "ya29.test", "", nil)
	require.NoError(t, err)
	require.Equal(t, "crm-project-1", project)
	require.Equal(t, geminicli.GeminiCLIUserAgent, gotUA)
}

func TestFetchProjectIDAbortsWhenAccountProfileLoadFails(t *testing.T) {
	account := leftoverGeminiOAuthAccount(1562)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	codeAssist := &mockGeminiCodeAssistClient{
		loadCodeAssistFunc: func(_ context.Context, _, _, _ string, _ *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
			t.Fatal("LoadCodeAssist should not run when profile load fails")
			return nil, nil
		},
	}
	svc := NewGeminiOAuthService(nil, nil, codeAssist, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	_, _, err := svc.fetchProjectID(context.Background(), "ya29.test", "", account)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
}
