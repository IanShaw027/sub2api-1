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
	errorIDs       []int64
	errorMsgs      []string
}

func (r *kiroGatewayRateLimitRepoStub) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedIDs = append(r.rateLimitedIDs, id)
	r.rateLimitUntil = append(r.rateLimitUntil, resetAt)
	return nil
}

func (r *kiroGatewayRateLimitRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.tempIDs = append(r.tempIDs, id)
	r.tempUntil = append(r.tempUntil, until)
	r.tempReasons = append(r.tempReasons, reason)
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
	gin.SetMode(gin.TestMode)

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
		Body: []byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"tools":[{"type":"web_search_20250305"}],
			"messages":[{"role":"user","content":[{"type":"text","text":"query"}]}]
		}`),
	}))
	require.Zero(t, upstream.calls)
}

func TestKiroGatewayService_Forward_RetriesOnceAfterInvalidTokenResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
		Body: []byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`),
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.calls)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "hello after refresh")
}

func TestKiroGatewayService_Forward_InvalidTokenRetryFailureDoesNotLeakRetryError(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
		Body: []byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`),
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.NotContains(t, rec.Body.String(), "retry_error")
	require.NotContains(t, err.Error(), "retry_error")
}

func TestKiroGatewayService_Forward_Kiro402QuotaExhaustedTriggersFailoverRateLimitPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
		Body: []byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`),
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
	gin.SetMode(gin.TestMode)

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
		Body: []byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"tools":[{"type":"web_search_20250305"}],
			"messages":[{"role":"assistant","content":[{"type":"text","text":"query"}]}]
		}`),
		OnUpstreamAccepted: func() {
			accepted++
		},
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "no query found")
	require.Equal(t, 0, accepted)
}
