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
	"github.com/Wei-Shaw/sub2api/internal/model"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type kiroGatewayRateLimitRepoStub struct {
	mockAccountRepoForGemini
	rateLimitedIDs []int64
	rateLimitUntil []time.Time
	tempIDs        []int64
	tempUntil      []time.Time
	tempReasons    []string
	tempCtxErrs    []error
	rejectCanceled bool
	errorIDs       []int64
	errorMsgs      []string
}

func (r *kiroGatewayRateLimitRepoStub) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedIDs = append(r.rateLimitedIDs, id)
	r.rateLimitUntil = append(r.rateLimitUntil, resetAt)
	return nil
}

func (r *kiroGatewayRateLimitRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	return r.setTempUnschedulable(ctx, id, until, reason)
}

func (r *kiroGatewayRateLimitRepoStub) setTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	if ctx != nil {
		r.tempCtxErrs = append(r.tempCtxErrs, ctx.Err())
	} else {
		r.tempCtxErrs = append(r.tempCtxErrs, nil)
	}
	if r.rejectCanceled && ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	r.tempIDs = append(r.tempIDs, id)
	r.tempUntil = append(r.tempUntil, until)
	r.tempReasons = append(r.tempReasons, reason)
	return nil
}

type kiroTempUnschedCacheCtxRecorder struct {
	accountIDs     []int64
	states         []*TempUnschedState
	ctxErrs        []error
	rejectCanceled bool
}

func (c *kiroTempUnschedCacheCtxRecorder) SetTempUnsched(ctx context.Context, accountID int64, state *TempUnschedState) error {
	if ctx != nil {
		c.ctxErrs = append(c.ctxErrs, ctx.Err())
	} else {
		c.ctxErrs = append(c.ctxErrs, nil)
	}
	if c.rejectCanceled && ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	c.accountIDs = append(c.accountIDs, accountID)
	c.states = append(c.states, state)
	return nil
}

func (c *kiroTempUnschedCacheCtxRecorder) GetTempUnsched(context.Context, int64) (*TempUnschedState, error) {
	return nil, nil
}

func (c *kiroTempUnschedCacheCtxRecorder) DeleteTempUnsched(context.Context, int64) error {
	return nil
}

func TestGatewayService_TempUnscheduleRetryableError_Kiro429RetryExhaustedUsesShortCooldown(t *testing.T) {
	repo := &kiroGatewayRateLimitRepoStub{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{
				42: {
					ID:       42,
					Platform: PlatformKiro,
					Type:     AccountTypeOAuth,
				},
			},
		},
	}
	cache := &kiroTempUnschedCacheRecorder{}
	svc := &GatewayService{
		accountRepo:      repo,
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, cache),
	}
	before := time.Now()
	failoverErr := &UpstreamFailoverError{
		StatusCode:             http.StatusTooManyRequests,
		ResponseBody:           []byte(`{"message":"Too many requests, please wait before trying again.","reason":null}`),
		RetryableOnSameAccount: true,
		RetryExhaustedCooldown: time.Minute,
		RetryExhaustedReason:   "kiro_429_retry_exhausted",
	}

	svc.TempUnscheduleRetryableError(context.Background(), 42, failoverErr)

	require.Equal(t, []int64{42}, repo.tempIDs)
	require.Len(t, repo.tempUntil, 1)
	require.WithinDuration(t, before.Add(time.Minute), repo.tempUntil[0], 2*time.Second)
	require.Contains(t, repo.tempReasons[0], "Too many requests")
	require.Equal(t, []int64{42}, cache.accountIDs)
	require.Len(t, cache.states, 1)
	require.Equal(t, int64(http.StatusTooManyRequests), int64(cache.states[0].StatusCode))
	require.WithinDuration(t, before.Add(time.Minute), time.Unix(cache.states[0].UntilUnix, 0), 2*time.Second)
}

func (r *kiroGatewayRateLimitRepoStub) SetError(_ context.Context, id int64, errorMsg string) error {
	r.errorIDs = append(r.errorIDs, id)
	r.errorMsgs = append(r.errorMsgs, errorMsg)
	return nil
}

func TestKiroGatewayService_Forward_TokenFailureReturnsFailoverAndTempUnsched(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	repo := &kiroGatewayRateLimitRepoStub{}
	cache := &kiroTempUnschedCacheRecorder{}
	svc := &KiroGatewayService{
		httpUpstream:     &kiroHTTPUpstreamRecorder{},
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, cache),
	}

	before := time.Now()
	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       58620,
		Name:     "revoked-refresh-token",
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr, "token acquisition failure should trigger account failover")
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "kiro token provider is not configured")
	require.Empty(t, rec.Body.String(), "service must not write a 502 body before handler failover is exhausted")
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, []int64{58620}, repo.tempIDs)
	require.Len(t, repo.tempUntil, 1)
	require.WithinDuration(t, before.Add(kiroTransportFailureCooldown), repo.tempUntil[0], 5*time.Second)
	require.Contains(t, repo.tempReasons[0], "kiro token provider is not configured")
	require.Equal(t, []int64{58620}, cache.accountIDs)
	require.Len(t, cache.states, 1)
	require.Equal(t, kiroTokenFailureReasonKeyword, cache.states[0].MatchedKeyword)
}

func TestKiroGatewayService_Forward_NonFailoverHTTPErrorAppliesKiroPassthroughRule(t *testing.T) {
	installDefaultKiroOutboundProfile(t)
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	ruleSvc := &ErrorPassthroughService{}
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{{
		ID:              1,
		Name:            "kiro-400-passthrough",
		Enabled:         true,
		Priority:        1,
		ErrorCodes:      []int{http.StatusBadRequest},
		Keywords:        []string{"kiro passthrough invalid field"},
		MatchMode:       model.MatchModeAll,
		Platforms:       []string{PlatformKiro},
		PassthroughCode: true,
		PassthroughBody: true,
	}})
	BindErrorPassthroughService(c, ruleSvc)

	upstreamBody := `{"error":{"type":"invalid_request_error","message":"kiro passthrough invalid field"}}`
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(upstreamBody)),
		},
	}
	svc := &KiroGatewayService{httpUpstream: upstream}

	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       58621,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "kiro passthrough invalid field")
	require.NotContains(t, rec.Body.String(), "api_error")
}

func TestKiroGatewayService_Forward_EmulatesWebSearchBeforeKiroUpstream(t *testing.T) {
	setGinTestMode()

	manager := websearch.NewManager([]websearch.ProviderConfig{{Type: websearch.ProviderTypeBrave, APIKey: "key"}}, nil)
	SetWebSearchManager(manager)
	defer SetWebSearchManager(nil)
	setGlobalWebSearchConfig(&WebSearchEmulationConfig{
		Enabled:   true,
		Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "key"}},
	})
	defer clearGlobalWebSearchConfig()

	upstream := &kiroHTTPUpstreamRecorder{}
	channelSvc := newChannelServiceWithCache(77, &Channel{
		ID:     9,
		Status: StatusActive,
		FeaturesConfig: map[string]any{
			featureKeyWebSearchEmulation: map[string]any{
				PlatformKiro: true,
			},
		},
	})
	svc := &KiroGatewayService{
		httpUpstream:   upstream,
		settingService: newSettingServiceForWebSearchTest(true),
		channelService: channelSvc,
	}
	groupID := int64(77)

	require.True(t, svc.shouldEmulateWebSearch(context.Background(), &Account{
		ID:       501,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{featureKeyWebSearchEmulation: WebSearchModeDefault},
	}, &ParsedRequest{
		GroupID: &groupID,
		Model:   "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"tools":[{"type":"web_search_20250305"}],
			"messages":[{"role":"user","content":[{"type":"text","text":"query"}]}]
		}`)),
	}))
	require.Zero(t, upstream.calls)
}

func TestKiroGatewayService_Forward_HonorsDeviceTLSProfileID(t *testing.T) {
	pinID := int64(12)
	device := validKiroOutboundProfile(620, kiroPinnedOutboundMachineID)
	device.TLSProfileID = &pinID
	installKiroOutboundDeviceProfile(t, device, nil)
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "curl/8.0")

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "assistantResponseEvent",
			}, map[string]any{"content": "pinned tls"}))),
		},
	}
	svc := &KiroGatewayService{
		httpUpstream: upstream,
		tlsFPProfileSvc: testTLSProfileService(
			&model.TLSFingerprintProfile{ID: 12, Name: "pin:kiro-ide:macos:h1"},
			&model.TLSFingerprintProfile{ID: 23, Name: "router-unique"},
		),
		tlsFPRouterSvc: testTLSRouterService(&model.TLSFingerprintRouter{
			ID:      3,
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{{
				Name:                    "unique",
				Enabled:                 true,
				TLSFingerprintProfileID: 23,
			}},
		}),
	}

	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       620,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(23),
			"tls_fingerprint_router_id":  int64(3),
		},
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.profile)
	require.Equal(t, "pin:kiro-ide:macos:h1", upstream.profile.Name)
	require.NotEqual(t, "router-unique", upstream.profile.Name)
}

func TestKiroGatewayService_Forward_RetriesOnceAfterInvalidTokenResponse(t *testing.T) {
	installDefaultKiroOutboundProfile(t)
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			auth := req.Header.Get("Authorization")
			if auth == "Bearer stale-access" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"message":"invalid token"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
					":message-type": "event",
					":event-type":   "assistantResponseEvent",
				}, map[string]any{"content": "hello after refresh"}))),
			}, nil
		},
	}
	repo := &refreshAPIAccountRepo{
		account: &Account{
			ID:       610,
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"access_token":  "fresh-access",
				"refresh_token": "fresh-refresh",
				"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			},
		},
	}
	provider := NewKiroTokenProvider(repo, nil)
	provider.executor = &refreshAPIExecutorStub{
		needsRefresh: true,
		credentials: map[string]any{
			"access_token":  "fresh-access",
			"refresh_token": "fresh-refresh",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	svc := &KiroGatewayService{
		httpUpstream:  upstream,
		tokenProvider: provider,
	}

	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       610,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "stale-access",
			"refresh_token": "stale-refresh",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.calls)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "hello after refresh")
}

func TestKiroGatewayService_Forward_InvalidTokenRetryFailureDoesNotLeakRetryError(t *testing.T) {
	installDefaultKiroOutboundProfile(t)
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"invalid token"}`)),
		},
	}
	svc := &KiroGatewayService{
		httpUpstream: upstream,
	}

	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       611,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "stale-access",
			"refresh_token": "stale-refresh",
		},
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Empty(t, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "retry_error")
	require.NotContains(t, err.Error(), "retry_error")
}

func TestKiroGatewayService_Forward_Kiro402QuotaExhaustedTriggersFailoverRateLimitPath(t *testing.T) {
	installDefaultKiroOutboundProfile(t)
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	repo := &kiroGatewayRateLimitRepoStub{}
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusPaymentRequired,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"message":"MONTHLY_REQUEST_COUNT exceeded for this subscription"
			}`)),
		},
	}
	svc := &KiroGatewayService{
		httpUpstream:     upstream,
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil),
	}

	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       612,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-key",
		},
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Empty(t, rec.Body.String())
	require.Equal(t, []int64{612}, repo.rateLimitedIDs)
	require.Len(t, repo.rateLimitUntil, 1)
	require.Empty(t, repo.tempIDs)
	require.Empty(t, repo.errorIDs)
}

func TestKiroGatewayService_Forward_WebSearchMalformedRequestDoesNotAcceptEarly(t *testing.T) {
	setGinTestMode()

	manager := websearch.NewManager([]websearch.ProviderConfig{{Type: websearch.ProviderTypeBrave, APIKey: "key"}}, nil)
	SetWebSearchManager(manager)
	defer SetWebSearchManager(nil)
	setGlobalWebSearchConfig(&WebSearchEmulationConfig{
		Enabled:   true,
		Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "key"}},
	})
	defer clearGlobalWebSearchConfig()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	channelSvc := newChannelServiceWithCache(77, &Channel{
		ID:     9,
		Status: StatusActive,
		FeaturesConfig: map[string]any{
			featureKeyWebSearchEmulation: map[string]any{
				PlatformKiro: true,
			},
		},
	})
	svc := &KiroGatewayService{
		settingService: newSettingServiceForWebSearchTest(true),
		channelService: channelSvc,
	}
	groupID := int64(77)
	accepted := 0

	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       612,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{featureKeyWebSearchEmulation: WebSearchModeDefault},
	}, &ParsedRequest{
		GroupID: &groupID,
		Model:   "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"tools":[{"type":"web_search_20250305"}],
			"messages":[{"role":"assistant","content":[{"type":"text","text":"query"}]}]
		}`)),
		OnUpstreamAccepted: func() {
			accepted++
		},
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "no query found")
	require.Equal(t, 0, accepted)
}

// TestKiroGatewayService_Forward_TransportErr_UpstreamCancel_FailoverAndTempUnsched
// 验证：上游 transport 故障（非客户端断开）时，handleKiroTransportError 应当返回
// *UpstreamFailoverError 触发 handler 的账号 failover，并把当前账号置临时不可调度。
func TestKiroGatewayService_Forward_TransportErr_UpstreamCancel_FailoverAndTempUnsched(t *testing.T) {
	installDefaultKiroOutboundProfile(t)
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(*http.Request, string, int64, int, *tlsfingerprint.Profile) (*http.Response, error) {
			return nil, errors.New(`Post "https://q.us-east-1.amazonaws.com/generateAssistantResponse": context canceled`)
		},
	}
	repo := &kiroGatewayRateLimitRepoStub{}
	cache := &kiroTempUnschedCacheRecorder{}
	tokenRepo := &refreshAPIAccountRepo{
		account: &Account{
			ID:       58609,
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"access_token":  "fresh",
				"refresh_token": "fresh",
				"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			},
		},
	}
	provider := NewKiroTokenProvider(tokenRepo, nil)
	svc := &KiroGatewayService{
		httpUpstream:     upstream,
		tokenProvider:    provider,
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, cache),
	}

	before := time.Now()
	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       58609,
		Name:     "hikmanalie@protzy.tech",
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "fresh",
			"refresh_token": "fresh",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr, "应当返回 UpstreamFailoverError 触发账号 failover")
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "context canceled")

	// handler 主循环负责写最终响应；service 内部不再 c.JSON
	require.Empty(t, rec.Body.String())

	// 上游故障 → 该账号临时不可调度
	require.Equal(t, []int64{58609}, repo.tempIDs)
	require.Len(t, repo.tempUntil, 1)
	require.WithinDuration(t, before.Add(kiroTransportFailureCooldown), repo.tempUntil[0], 5*time.Second)
	require.Contains(t, repo.tempReasons[0], "context canceled")

	// cache 同步落盘
	require.Equal(t, []int64{58609}, cache.accountIDs)
	require.Len(t, cache.states, 1)
	require.Equal(t, kiroTransportFailureReasonKeyword, cache.states[0].MatchedKeyword)

	// transport err 不应误标全局限流
	require.Empty(t, repo.rateLimitedIDs)
	require.Empty(t, repo.errorIDs)
}

func TestKiroGatewayService_HandleKiroTransportError_ExpiredCtxStillMarksTempUnsched(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	repo := &kiroGatewayRateLimitRepoStub{rejectCanceled: true}
	cache := &kiroTempUnschedCacheCtxRecorder{rejectCanceled: true}
	svc := &KiroGatewayService{
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, cache),
	}
	account := &Account{
		ID:       58611,
		Name:     "ctx-expired-account",
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}
	expiredCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	before := time.Now()

	err := svc.handleKiroTransportError(
		expiredCtx,
		c,
		account,
		"https://q.us-east-1.amazonaws.com/generateAssistantResponse",
		errors.New(`Post "https://q.us-east-1.amazonaws.com/generateAssistantResponse": context deadline exceeded`),
	)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Equal(t, []int64{58611}, repo.tempIDs)
	require.Len(t, repo.tempUntil, 1)
	require.WithinDuration(t, before.Add(kiroTransportFailureCooldown), repo.tempUntil[0], 5*time.Second)
	require.Len(t, repo.tempCtxErrs, 1)
	require.NoError(t, repo.tempCtxErrs[0], "账号短冷却写入应使用脱离请求取消的 state context")
	require.Equal(t, []int64{58611}, cache.accountIDs)
	require.Len(t, cache.states, 1)
	require.Len(t, cache.ctxErrs, 1)
	require.NoError(t, cache.ctxErrs[0], "cache 同步也应使用脱离请求取消的 state context")
}

func TestKiroGatewayService_HandleKiroTransportError_CanceledRequestWithConnectionResetMarksTempUnsched(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(canceledCtx)

	repo := &kiroGatewayRateLimitRepoStub{}
	cache := &kiroTempUnschedCacheRecorder{}
	svc := &KiroGatewayService{
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, cache),
	}
	account := &Account{
		ID:       58612,
		Name:     "connection-reset-account",
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}
	before := time.Now()

	err := svc.handleKiroTransportError(
		context.Background(),
		c,
		account,
		"https://q.us-east-1.amazonaws.com/generateAssistantResponse",
		errors.New(`Post "https://q.us-east-1.amazonaws.com/generateAssistantResponse": read tcp 10.0.0.2:52564->10.0.0.3:443: read: connection reset by peer`),
	)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "connection reset by peer")
	require.Equal(t, []int64{58612}, repo.tempIDs)
	require.Len(t, repo.tempUntil, 1)
	require.WithinDuration(t, before.Add(kiroTransportFailureCooldown), repo.tempUntil[0], 5*time.Second)
	require.Equal(t, []int64{58612}, cache.accountIDs)
	require.Len(t, cache.states, 1)
	require.Equal(t, kiroTransportFailureReasonKeyword, cache.states[0].MatchedKeyword)
}

// TestKiroGatewayService_Forward_TransportErr_ClientDisconnect_NoFailoverNoMark
// 验证：客户端真正断开（gin.Request.Context() 已 Canceled）时，
// 不返回 UpstreamFailoverError、不标记账号临时不可调度。
func TestKiroGatewayService_Forward_TransportErr_ClientDisconnect_NoFailoverNoMark(t *testing.T) {
	installDefaultKiroOutboundProfile(t)
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // 模拟客户端在请求过程中断开
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(canceledCtx)

	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(*http.Request, string, int64, int, *tlsfingerprint.Profile) (*http.Response, error) {
			return nil, context.Canceled
		},
	}
	repo := &kiroGatewayRateLimitRepoStub{}
	cache := &kiroTempUnschedCacheRecorder{}
	tokenRepo := &refreshAPIAccountRepo{
		account: &Account{
			ID:       58610,
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"access_token":  "fresh",
				"refresh_token": "fresh",
				"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			},
		},
	}
	provider := NewKiroTokenProvider(tokenRepo, nil)
	svc := &KiroGatewayService{
		httpUpstream:     upstream,
		tokenProvider:    provider,
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, cache),
	}

	result, err := svc.Forward(context.Background(), c, &Account{
		ID:       58610,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "fresh",
			"refresh_token": "fresh",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "客户端断开不应当触发账号 failover")

	// 不写客户端响应（客户端已经走了）、不标记账号
	require.Empty(t, repo.tempIDs)
	require.Empty(t, cache.accountIDs)
	require.Empty(t, repo.rateLimitedIDs)
	require.Empty(t, repo.errorIDs)
}

func TestKiroResponseTelemetryFlagsMixedCompletedAndPartialToolUse(t *testing.T) {
	t.Parallel()

	telemetry := &kiroResponseTelemetry{
		CompletedToolUseCount: 1,
		PartialToolUseCount:   1,
	}

	require.Contains(t, telemetry.anomalyKinds("tool_use"), "incomplete_tool_use_completed")
}

const kiroPinnedOutboundMachineID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

type staticKiroDeviceProfileRepo struct {
	profile       *AccountDeviceProfile
	err           error
	getCalls      int
	insertCalls   int
	failAfterGets int
}

func (r *staticKiroDeviceProfileRepo) GetByAccountID(_ context.Context, accountID int64) (*AccountDeviceProfile, error) {
	r.getCalls++
	if r.failAfterGets > 0 {
		if r.getCalls > r.failAfterGets {
			if r.err != nil {
				return nil, r.err
			}
			return nil, errors.New("identity_reject: subsequent profile load failed")
		}
	} else if r.err != nil {
		return nil, r.err
	}
	if r.profile == nil {
		return nil, nil
	}
	cloned := *r.profile
	cloned.AccountID = accountID
	if cloned.ProfilePayload != nil {
		payload := make(map[string]any, len(cloned.ProfilePayload))
		for k, v := range cloned.ProfilePayload {
			payload[k] = v
		}
		cloned.ProfilePayload = payload
	}
	return &cloned, nil
}

func (r *staticKiroDeviceProfileRepo) InsertBaseline(_ context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	r.insertCalls++
	if r.err != nil {
		return nil, r.err
	}
	return p, nil
}

func (r *staticKiroDeviceProfileRepo) UpdateCAS(context.Context, int64, int64, *AccountDeviceProfile) (bool, error) {
	return false, nil
}

func (r *staticKiroDeviceProfileRepo) DeleteByAccountID(context.Context, int64) error {
	return nil
}

func validKiroOutboundProfile(accountID int64, machineID string) *AccountDeviceProfile {
	return &AccountDeviceProfile{
		AccountID:          accountID,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           PlatformKiro,
		ClientFamily:       ClientFamilyKiroIDE,
		InstallationID:     "11111111-1111-4111-8111-111111111111",
		DeviceID:           "22222222-2222-4222-8222-222222222222",
		MachineID:          machineID,
		GatewayAccountUUID: "55555555-5555-4555-8555-555555555555",
		SessionNamespace:   "0123456789abcdef0123456789abcdef",
		OSFamily:           "macos",
		Arch:               "arm64",
		Runtime:            "node",
		RuntimeVersion:     defaultKiroNodeVersion,
		ClientVersion:      defaultKiroVersion,
		TransportFamily:    TransportH1,
		ProfilePayload: map[string]any{
			"kiro_system_version": defaultKiroSystemVersion,
			"kiro_node_version":   defaultKiroNodeVersion,
		},
		LearnedFrom: LearnedFromBaseline,
	}
}

func installKiroOutboundDeviceProfile(t *testing.T, profile *AccountDeviceProfile, loadErr error) {
	t.Helper()
	prev := OutboundDeviceProfileService()
	if loadErr != nil || profile != nil {
		SetOutboundDeviceProfileService(NewAccountDeviceService(&staticKiroDeviceProfileRepo{
			profile: profile,
			err:     loadErr,
		}))
	} else {
		SetOutboundDeviceProfileService(nil)
	}
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
}

func installDefaultKiroOutboundProfile(t *testing.T) {
	t.Helper()
	installKiroOutboundDeviceProfile(t, validKiroOutboundProfile(0, kiroPinnedOutboundMachineID), nil)
}

func TestBuildKiroGenerateAssistantRequest_UsesProfileMachineIDNotRefreshToken(t *testing.T) {
	installKiroOutboundDeviceProfile(t, validKiroOutboundProfile(12, kiroPinnedOutboundMachineID), nil)

	body := []byte(`{"conversationState":{}}`)
	accountA := &Account{
		ID:       12,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "kiro-refresh-token-alpha",
		},
	}
	accountB := &Account{
		ID:       12,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "kiro-refresh-token-bravo",
		},
	}

	reqA, err := buildKiroGenerateAssistantRequest(context.Background(), accountA, body, "access-a", nil)
	require.NoError(t, err)
	require.NotNil(t, reqA)
	reqB, err := buildKiroGenerateAssistantRequest(context.Background(), accountB, body, "access-b", nil)
	require.NoError(t, err)
	require.NotNil(t, reqB)

	tokenMachineA := kiropkg.GenerateMachineID("", accountA.GetCredential("refresh_token"))
	tokenMachineB := kiropkg.GenerateMachineID("", accountB.GetCredential("refresh_token"))
	require.NotEmpty(t, tokenMachineA)
	require.NotEmpty(t, tokenMachineB)
	require.NotEqual(t, tokenMachineA, tokenMachineB)

	uaA := reqA.Header.Get("x-amz-user-agent")
	uaB := reqB.Header.Get("x-amz-user-agent")
	require.Equal(t, uaA, uaB)
	require.Contains(t, uaA, kiroPinnedOutboundMachineID)
	require.Contains(t, reqA.Header.Get("User-Agent"), kiroPinnedOutboundMachineID)
	require.NotContains(t, uaA, tokenMachineA)
	require.NotContains(t, uaA, tokenMachineB)
	require.NotContains(t, reqA.Header.Get("User-Agent"), tokenMachineA)
	require.NotContains(t, reqB.Header.Get("User-Agent"), tokenMachineB)
}

func TestBuildKiroGenerateAssistantRequest_LoadFailureDoesNotUseTokenMachineID(t *testing.T) {
	installKiroOutboundDeviceProfile(t, nil, errors.New("identity_reject: device profile unavailable"))

	account := &Account{
		ID:       13,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "kiro-refresh-token-should-not-mint-machine-id",
		},
	}

	req, err := buildKiroGenerateAssistantRequest(context.Background(), account, []byte(`{}`), "access", nil)
	require.Error(t, err)
	require.Nil(t, req)
	require.NotContains(t, err.Error(), kiropkg.GenerateMachineID("", account.GetCredential("refresh_token")))
}

func TestBuildKiroGenerateAssistantRequest_ProfilePayloadOverridesRuntimeSettings(t *testing.T) {
	profile := validKiroOutboundProfile(14, kiroPinnedOutboundMachineID)
	profile.ProfilePayload = map[string]any{
		"kiro_system_version": "linux#6.8.0",
		"kiro_node_version":   "20.19.0",
		"kiro_commit":         "abc1234",
	}
	installKiroOutboundDeviceProfile(t, profile, nil)

	req, err := buildKiroGenerateAssistantRequest(context.Background(), &Account{
		ID:       14,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, []byte(`{}`), "access", &KiroRuntimeSettings{
		KiroVersion:   defaultKiroVersion,
		SystemVersion: defaultKiroSystemVersion,
		NodeVersion:   defaultKiroNodeVersion,
		KiroCommit:    "runtime-commit",
	})
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Contains(t, req.Header.Get("User-Agent"), "os/linux#6.8.0")
	require.Contains(t, req.Header.Get("User-Agent"), "md/nodejs#20.19.0")
	require.Equal(t, "abc1234", req.Header.Get("x-amzn-kiro-commit"))
	require.NotEqual(t, "runtime-commit", req.Header.Get("x-amzn-kiro-commit"))
}

func TestKiroBuildRequestAndUpstream_LoadsDeviceProfileOnce(t *testing.T) {
	repo := &staticKiroDeviceProfileRepo{
		profile:       validKiroOutboundProfile(21, kiroPinnedOutboundMachineID),
		failAfterGets: 1,
		err:           errors.New("identity_reject: subsequent profile load failed"),
	}
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}
	svc := &KiroGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:       21,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}

	req, deviceProfile, err := svc.buildRequest(context.Background(), account, []byte(`{}`), "access", nil)
	require.NoError(t, err)
	require.NotNil(t, req)
	resp, err := svc.doKiroUpstream(context.Background(), nil, account, req, deviceProfile)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 1, repo.getCalls)
}

func TestLoadOutboundDeviceProfile_UnconfiguredWithoutPerTestInstall(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	p, err := LoadOutboundDeviceProfile(context.Background(), &Account{ID: 99, Platform: PlatformAnthropic})
	require.Error(t, err)
	require.Nil(t, p)
	require.ErrorContains(t, err, "not configured")
}
