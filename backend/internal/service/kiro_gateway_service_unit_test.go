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

func TestKiroGatewayService_Forward_RetriesOnceAfterInvalidTokenResponse(t *testing.T) {
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
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.NotContains(t, rec.Body.String(), "retry_error")
	require.NotContains(t, err.Error(), "retry_error")
}

func TestKiroGatewayService_Forward_Kiro402QuotaExhaustedTriggersFailoverRateLimitPath(t *testing.T) {
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
