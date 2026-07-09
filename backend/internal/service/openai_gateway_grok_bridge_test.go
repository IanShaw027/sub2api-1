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

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type grokBridgeAccountRepo struct {
	AccountRepository
	accountsByID          map[int64]*Account
	updates               map[int64]map[string]any
	tempUnschedCalls      int
	lastTempUnschedID     int64
	lastTempUnschedReason string
}

func (r *grokBridgeAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if r != nil && r.accountsByID != nil {
		if acc, ok := r.accountsByID[id]; ok {
			return acc, nil
		}
	}
	return nil, errors.New("account not found")
}

func (r *grokBridgeAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	if r.updates == nil {
		r.updates = make(map[int64]map[string]any)
	}
	r.updates[id] = updates
	return nil
}

func (r *grokBridgeAccountRepo) SetTempUnschedulable(_ context.Context, id int64, _ time.Time, reason string) error {
	r.tempUnschedCalls++
	r.lastTempUnschedID = id
	r.lastTempUnschedReason = reason
	return nil
}

type grokBridgeTLSFingerprintRouterRepoStub struct {
	routers []*model.TLSFingerprintRouter
}

func (r *grokBridgeTLSFingerprintRouterRepoStub) List(context.Context) ([]*model.TLSFingerprintRouter, error) {
	return r.routers, nil
}

func (r *grokBridgeTLSFingerprintRouterRepoStub) GetByID(_ context.Context, id int64) (*model.TLSFingerprintRouter, error) {
	for _, router := range r.routers {
		if router.ID == id {
			return router, nil
		}
	}
	return nil, nil
}

func (r *grokBridgeTLSFingerprintRouterRepoStub) Create(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	r.routers = append(r.routers, router)
	return router, nil
}

func (r *grokBridgeTLSFingerprintRouterRepoStub) Update(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	for i, existing := range r.routers {
		if existing.ID == router.ID {
			r.routers[i] = router
			return router, nil
		}
	}
	r.routers = append(r.routers, router)
	return router, nil
}

func (r *grokBridgeTLSFingerprintRouterRepoStub) Delete(_ context.Context, id int64) error {
	next := r.routers[:0]
	for _, router := range r.routers {
		if router.ID != id {
			next = append(next, router)
		}
	}
	r.routers = next
	return nil
}

type grokBridgeHTTPUpstreamRecorder struct {
	lastReq        *http.Request
	lastBody       []byte
	resp           *http.Response
	err            error
	tlsCalled      bool
	lastTLSProfile *tlsfingerprint.Profile
}

func (u *grokBridgeHTTPUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.lastReq = req
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	if u.err != nil {
		return nil, u.err
	}
	return u.resp, nil
}

func (u *grokBridgeHTTPUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.tlsCalled = true
	u.lastTLSProfile = profile
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestForwardAsChatCompletionsForGrokUsesCompatibleTLSRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "GrokDesktop/1.0")

	account := &Account{
		ID:          57,
		Name:        "grok-oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "grok-token",
			"base_url":     xai.DefaultCLIBaseURL,
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(31),
		},
	}
	router := &model.TLSFingerprintRouter{
		ID:      31,
		Name:    "grok-chat-router",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:                    "desktop-chat",
			Enabled:                 true,
			Transport:               model.TLSFingerprintRouterTransportHTTP,
			MatchType:               model.TLSFingerprintRouterMatchContains,
			Pattern:                 "GrokDesktop",
			TLSFingerprintProfileID: 92,
			UpstreamUserAgent:       "grok-chat-native/1.0",
			UpstreamOriginator:      "grok_chat",
		}},
	}
	upstream := &grokBridgeHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_grok_tls","object":"chat.completion","model":"grok-4.3","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		tlsFPRouterService: NewTLSFingerprintRouterService(&grokBridgeTLSFingerprintRouterRepoStub{
			routers: []*model.TLSFingerprintRouter{router},
		}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
			92: {ID: 92, Name: "Grok Chat Routed", Platform: "grok", Transport: "h2", UserAgent: "profile-ua", Originator: "profile-origin", ALPNProtocols: []string{"h2", "http/1.1"}, HTTP2Fingerprint: "1:4096|ph::method,:scheme,:authority,:path"},
		}},
	}

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.NoError(t, err)
	require.True(t, upstream.tlsCalled)
	require.NotNil(t, upstream.lastTLSProfile)
	require.Equal(t, "Grok Chat Routed", upstream.lastTLSProfile.Name)
	require.Equal(t, "grok-chat-native/1.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "grok_chat", upstream.lastReq.Header.Get("Originator"))
}

func TestForwardAsChatCompletionsForGrokUsesH2ProfileAndKeepsRouterHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "GrokDesktop/1.0")

	account := &Account{
		ID:          57,
		Name:        "grok-chat",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "grok-token",
			"base_url":     xai.DefaultCLIBaseURL,
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":      true,
			"enable_grok_tls_fingerprint": true,
			"tls_fingerprint_router_id":   int64(30),
		},
	}
	router := &model.TLSFingerprintRouter{
		ID:      30,
		Name:    "grok-router",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:                    "desktop",
			Enabled:                 true,
			Transport:               model.TLSFingerprintRouterTransportHTTP,
			MatchType:               model.TLSFingerprintRouterMatchContains,
			Pattern:                 "GrokDesktop",
			TLSFingerprintProfileID: 92,
			UpstreamUserAgent:       "grok-chat-native/1.0",
			UpstreamOriginator:      "grok_chat",
		}},
	}
	upstream := &grokBridgeHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_grok_tls","object":"chat.completion","model":"grok-4.3","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		tlsFPRouterService: NewTLSFingerprintRouterService(&grokBridgeTLSFingerprintRouterRepoStub{
			routers: []*model.TLSFingerprintRouter{router},
		}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
			92: {ID: 92, Name: "Grok Chat Routed", Platform: "grok", Transport: "h2", UserAgent: "profile-ua", Originator: "profile-origin", ALPNProtocols: []string{"h2", "http/1.1"}, HTTP2Fingerprint: "1:4096|ph::method,:scheme,:authority,:path"},
		}},
	}

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.NoError(t, err)
	require.True(t, upstream.tlsCalled)
	require.NotNil(t, upstream.lastTLSProfile)
	require.Equal(t, "Grok Chat Routed", upstream.lastTLSProfile.Name)
	require.Equal(t, "grok-chat-native/1.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "grok_chat", upstream.lastReq.Header.Get("Originator"))
}

func TestProxyOpenAIWSHTTPBridgeForGrokUsesTLSAndStoresQuotaHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "GrokDesktop/1.0")

	account := &Account{
		ID:          58,
		Name:        "grok-ws-bridge",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "grok-token",
			"base_url":     xai.DefaultCLIBaseURL,
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(32),
		},
	}
	repo := &grokBridgeAccountRepo{accountsByID: map[int64]*Account{58: account}}
	router := &model.TLSFingerprintRouter{
		ID:      32,
		Name:    "grok-ws-router",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:                    "desktop-ws",
			Enabled:                 true,
			Transport:               model.TLSFingerprintRouterTransportHTTP,
			MatchType:               model.TLSFingerprintRouterMatchContains,
			Pattern:                 "GrokDesktop",
			TLSFingerprintProfileID: 93,
			UpstreamUserAgent:       "grok-ws-native/1.0",
			UpstreamOriginator:      "grok_ws",
		}},
	}
	upstream := &grokBridgeHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                   []string{"text/event-stream"},
			"Xai-Request-Id":                 []string{"xai-ws-bridge"},
			"X-Ratelimit-Limit-Requests":     []string{"10"},
			"X-Ratelimit-Remaining-Requests": []string{"6"},
		},
		Body: io.NopCloser(strings.NewReader(
			`data: {"type":"response.output_text.delta","delta":"ok"}` + "\n\n" +
				`data: {"type":"response.completed","response":{"id":"resp_ws_bridge","model":"grok-4.3","usage":{"input_tokens":2,"output_tokens":1}}}` + "\n\n" +
				`data: [DONE]` + "\n\n",
		)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
		tlsFPRouterService: NewTLSFingerprintRouterService(&grokBridgeTLSFingerprintRouterRepoStub{
			routers: []*model.TLSFingerprintRouter{router},
		}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
			93: {ID: 93, Name: "Grok WS Routed", Platform: "grok", Transport: "h2", UserAgent: "profile-ua", Originator: "profile-origin", ALPNProtocols: []string{"h2", "http/1.1"}, HTTP2Fingerprint: "1:4096|ph::method,:scheme,:authority,:path"},
		}},
	}
	var downstream [][]byte
	payload := []byte(`{"type":"response.create","model":"grok","input":"hi"}`)

	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "grok-token", payload, len(payload), "grok", "", "", "", 1, func(msg []byte) error {
		downstream = append(downstream, append([]byte(nil), msg...))
		return nil
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, upstream.tlsCalled)
	require.NotNil(t, upstream.lastTLSProfile)
	require.Equal(t, "Grok WS Routed", upstream.lastTLSProfile.Name)
	require.Equal(t, "grok-ws-native/1.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "grok_ws", upstream.lastReq.Header.Get("Originator"))
	require.NotEmpty(t, downstream)
	require.NotNil(t, repo.updates[58][grokQuotaSnapshotExtraKey])
}

func TestProxyOpenAIWSHTTPBridgeForGrok429ReturnsFailoverBeforeClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)

	account := &Account{
		ID:          59,
		Name:        "grok-ws-bridge-429",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "grok-token",
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	repo := &grokBridgeAccountRepo{accountsByID: map[int64]*Account{59: account}}
	upstream := &grokBridgeHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Retry-After":  []string{"45"},
		},
		Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}
	var downstream [][]byte
	payload := []byte(`{"type":"response.create","model":"grok","input":"hi"}`)

	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "grok-token", payload, len(payload), "grok", "", "", "", 1, func(msg []byte) error {
		downstream = append(downstream, append([]byte(nil), msg...))
		return nil
	})

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr), "429 should fail over instead of writing a terminal client error")
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Equal(t, "45", failoverErr.ResponseHeaders.Get("Retry-After"))
	require.Empty(t, downstream)
	require.Equal(t, 1, repo.tempUnschedCalls)
	require.Equal(t, int64(59), repo.lastTempUnschedID)
}
