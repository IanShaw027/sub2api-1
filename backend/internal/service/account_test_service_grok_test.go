//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type grokAccountTestRouteAwareUpstream struct {
	lastReq      *http.Request
	lastBody     []byte
	lastProxyURL string
}

func (u *grokAccountTestRouteAwareUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.lastReq = req
	u.lastProxyURL = proxyURL
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	switch {
	case req != nil && req.URL != nil && req.URL.Path == "/v1/images/generations":
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"created":1710000008,"data":[{"b64_json":"aGVsbG8=","revised_prompt":"draw a cat"}]}`)),
		}, nil
	case req != nil && req.URL != nil && req.URL.Path == "/v1/videos/generations":
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"video_req_123","model":"grok-imagine-video","video":{"duration":2,"resolution":"720p"}}`)),
		}, nil
	default:
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
					"data: {\"type\":\"response.completed\"}\n\n" +
					"data: [DONE]\n\n",
			)),
		}, nil
	}
}

func (u *grokAccountTestRouteAwareUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

type grokTestAccountRepo struct {
	stubOpenAIAccountRepo
	updatedExtra          map[string]any
	tempUnschedCalls      int
	lastTempUnschedID     int64
	lastTempUnschedUntil  time.Time
	lastTempUnschedReason string
	setRateLimitedCalls   int
}

func (r *grokTestAccountRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if updates != nil {
		r.updatedExtra = make(map[string]any, len(updates))
		for k, v := range updates {
			r.updatedExtra[k] = v
		}
	}
	return nil
}

func (r *grokTestAccountRepo) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.tempUnschedCalls++
	r.lastTempUnschedID = id
	r.lastTempUnschedUntil = until
	r.lastTempUnschedReason = reason
	return nil
}

func (r *grokTestAccountRepo) SetRateLimited(_ context.Context, _ int64, _ time.Time) error {
	r.setRateLimitedCalls++
	return nil
}

func TestAccountTestService_TestAccountConnection_GrokOAuthUsesXAIResponses(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73977,
		Name:        "grok-oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n" +
				"data: [DONE]\n\n",
		)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73977/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-4.3", "hello grok", "")
	require.NoError(t, err)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, xai.DefaultBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer grok-access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json, text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, defaultGrokUpstreamUserAgent, upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
	require.Equal(t, "grok-4.3", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hello grok", gjson.GetBytes(upstream.lastBody, "input.0.content.0.text").String())
	require.Empty(t, upstream.lastReq.Header.Get("X-Account-Test-Prompt"))
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
	require.Contains(t, rec.Body.String(), `"text":"ok"`)
	require.Contains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokOAuthUsesProfileUserAgentWhenTLSEnabled(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73979,
		Name:        "grok-oauth-tls",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(91),
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n" +
				"data: [DONE]\n\n",
		)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
		tlsFPProfileService: &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
			91: {ID: 91, Name: "Grok Routed", Platform: "grok", Transport: "http", UserAgent: "grok-native/1.0", Originator: "grok_desktop"},
		}},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73979/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-4.3", "hello grok", "")
	require.NoError(t, err)

	require.True(t, upstream.tlsCalled)
	require.NotNil(t, upstream.lastTLSProfile)
	require.Equal(t, "grok-native/1.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "grok_desktop", upstream.lastReq.Header.Get("Originator"))
}

func TestAccountTestService_TestAccountConnection_GrokTextMappingToMediaStillUsesResponses(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73985,
		Name:        "grok-oauth-text-mapped-media",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
			"model_mapping": map[string]any{
				"grok-4.3": "grok-imagine-video",
			},
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &grokAccountTestRouteAwareUpstream{}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73985/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-4.3", "hello grok", "")
	require.NoError(t, err)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, xai.DefaultBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, "grok-4.3", gjson.GetBytes(upstream.lastBody, "model").String())
	require.NotContains(t, upstream.lastReq.URL.String(), "/videos/generations")
	require.Contains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokImagineUsesNativeImagesAPI(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73979,
		Name:        "grok-oauth-image",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &grokAccountTestRouteAwareUpstream{}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73979/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-imagine-1", "draw a cat", "")
	require.NoError(t, err)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, xai.DefaultBaseURL+"/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer grok-access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, defaultGrokUpstreamUserAgent, upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "grok-imagine-image-quality", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "draw a cat", gjson.GetBytes(upstream.lastBody, "prompt").String())
	require.NotContains(t, upstream.lastReq.URL.String(), "/responses")
	require.Contains(t, rec.Body.String(), `"type":"image"`)
	require.Contains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokImageMappingToTextStillUsesNativeImagesAPI(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73986,
		Name:        "grok-oauth-image-mapped-text",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
			"model_mapping": map[string]any{
				"grok-imagine-1": "grok-4.3",
			},
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &grokAccountTestRouteAwareUpstream{}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73986/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-imagine-1", "draw a cat", "")
	require.NoError(t, err)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, xai.DefaultBaseURL+"/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, "draw a cat", gjson.GetBytes(upstream.lastBody, "prompt").String())
	require.NotContains(t, upstream.lastReq.URL.String(), "/responses")
	require.Contains(t, rec.Body.String(), `"type":"image"`)
	require.Contains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokImagineVideoUsesNativeVideosAPI(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73980,
		Name:        "grok-oauth-video",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &grokAccountTestRouteAwareUpstream{}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73980/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-imagine-video", "make a cat video", "")
	require.NoError(t, err)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, xai.DefaultBaseURL+"/videos/generations", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer grok-access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, defaultGrokUpstreamUserAgent, upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "grok-imagine-video", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "make a cat video", gjson.GetBytes(upstream.lastBody, "prompt").String())
	require.NotContains(t, upstream.lastReq.URL.String(), "/responses")
	require.Contains(t, rec.Body.String(), `video_req_123`)
	require.Contains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokAPIKeyImagineImageRejected(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73981,
		Name:        "grok-apikey-image",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "grok-api-key",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &grokAccountTestRouteAwareUpstream{}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73981/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-imagine-1", "draw a cat", "")
	require.Error(t, err)
	require.Nil(t, upstream.lastReq)
	require.Contains(t, err.Error(), "OAuth")
	require.Contains(t, rec.Body.String(), "OAuth")
	require.NotContains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokAPIKeyImagineVideoRejected(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73982,
		Name:        "grok-apikey-video",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "grok-api-key",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &grokAccountTestRouteAwareUpstream{}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73982/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-imagine-video", "make a cat video", "")
	require.Error(t, err)
	require.Nil(t, upstream.lastReq)
	require.Contains(t, err.Error(), "OAuth")
	require.Contains(t, rec.Body.String(), "OAuth")
	require.NotContains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokVideoEmptyBodyWithoutIDFails(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73983,
		Name:        "grok-oauth-video-empty",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	// HTTP 200 with no request/job id must not be treated as success
	// (VideoCount is still inflated to ≥1 by usage metadata).
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"model":"grok-imagine-video","video":{"duration":2,"resolution":"720p"}}`)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73983/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-imagine-video", "make a cat video", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "request/job id")
	require.Contains(t, rec.Body.String(), "request/job id")
	require.NotContains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokImageBodyWithoutBytesFails(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73984,
		Name:        "grok-oauth-image-empty",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	// HTTP 200 with revised_prompt only (no url/b64_json) must not succeed.
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1710000008,"data":[{"revised_prompt":"draw a cat"}]}`)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73984/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-imagine-1", "draw a cat", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "url/b64_json")
	require.Contains(t, rec.Body.String(), "url/b64_json")
	require.NotContains(t, rec.Body.String(), `"success":true`)
}

func TestAccountSupportsOpenAIEndpointCapability_GrokAPIKeyTextOnlyNoImagine(t *testing.T) {
	oauth := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth}
	apiKey := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}

	require.True(t, oauth.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos))
	require.True(t, oauth.SupportsOpenAIImageCapability(OpenAIImagesCapabilityNative))

	require.False(t, apiKey.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos))
	require.False(t, apiKey.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic))
	require.False(t, apiKey.SupportsOpenAIImageCapability(OpenAIImagesCapabilityNative))
	require.True(t, apiKey.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.True(t, apiKey.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponsesIngress))
}

func TestAccountTestService_TestAccountConnection_GrokOAuth429UsesGrokReconcile(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73978,
		Name:        "grok-oauth-429",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := &grokTestAccountRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header: http.Header{
			"Content-Type":                   []string{"application/json"},
			"x-ratelimit-reset-requests":     []string{"30"},
			"x-ratelimit-limit-requests":     []string{"10"},
			"x-ratelimit-remaining-requests": []string{"0"},
		},
		Body: io.NopCloser(strings.NewReader(`{"error":"rate limited"}`)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73978/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-4.3", "hello grok", "")
	require.Error(t, err)
	require.Equal(t, 1, repo.tempUnschedCalls)
	require.Equal(t, account.ID, repo.lastTempUnschedID)
	require.Equal(t, "grok rate limited", repo.lastTempUnschedReason)
	require.NotZero(t, repo.lastTempUnschedUntil)
	require.Zero(t, repo.setRateLimitedCalls)
	require.Contains(t, rec.Body.String(), "429")
}
