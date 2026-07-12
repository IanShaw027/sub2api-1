//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type kiroUsageAccountRepo struct {
	mockAccountRepoForGemini
	updateCalls      int
	updatedAcct      *Account
	updateExtraIDs   []int64
	updateExtraCalls []map[string]any
	updateExtraErr   error
	clearCalls       int
	account          *Account
}

func (r *kiroUsageAccountRepo) Update(_ context.Context, account *Account) error {
	r.updateCalls++
	r.updatedAcct = account
	return nil
}

func (r *kiroUsageAccountRepo) GetByID(_ context.Context, _ int64) (*Account, error) {
	if r.account != nil {
		return r.account, nil
	}
	return r.mockAccountRepoForGemini.GetByID(context.Background(), 0)
}

func (r *kiroUsageAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	r.updateExtraIDs = append(r.updateExtraIDs, id)
	cloned := make(map[string]any, len(updates))
	for k, v := range updates {
		cloned[k] = v
	}
	r.updateExtraCalls = append(r.updateExtraCalls, cloned)
	return r.updateExtraErr
}

func (r *kiroUsageAccountRepo) ClearError(_ context.Context, _ int64) error {
	r.clearCalls++
	return nil
}

type kiroUsageFetcherStub struct {
	upstream HTTPUpstream
}

func (s *kiroUsageFetcherStub) FetchUsage(context.Context, string, string) (*ClaudeUsageResponse, error) {
	return nil, nil
}

func (s *kiroUsageFetcherStub) FetchUsageWithOptions(context.Context, *ClaudeUsageFetchOptions) (*ClaudeUsageResponse, error) {
	return nil, nil
}

func (s *kiroUsageFetcherStub) HTTPUpstream() HTTPUpstream {
	return s.upstream
}

// newKiroUsageTestProvider 构造走「带锁统一刷新入口」的 KiroTokenProvider 供 usage 测试注入。
// tokenCache 传 nil → RefreshIfNeeded 降级为无分布式锁（进程内互斥仍在），但保留 DB 重读 +
// invalid_grant 竞争恢复 + credentials-only 持久化；executor 用测试 upstream，故 /refreshToken
// mock 仍生效。生产路径与此同源（ProvideKiroTokenProvider 也是同一 executor + transport）。
func newKiroUsageTestProvider(repo AccountRepository, proxyRepo ProxyRepository, upstream HTTPUpstream) *KiroTokenProvider {
	p := NewKiroTokenProvider(repo, nil)
	refreshAPI := NewOAuthRefreshAPI(repo, nil)
	executor := NewKiroTokenRefresher().
		WithTransport(upstream, nil).
		WithProxyRepo(proxyRepo)
	p.SetRefreshAPI(refreshAPI, executor)
	p.SetRefreshPolicy(KiroProviderRefreshPolicy())
	return p
}

type kiroWindowStatsRepoStub struct {
	geminiUsageLogRepoStub
	windowStats    *usagestats.AccountStats
	requestedStart time.Time
}

func (r *kiroWindowStatsRepoStub) GetAccountWindowStats(_ context.Context, _ int64, startTime time.Time) (*usagestats.AccountStats, error) {
	r.requestedStart = startTime
	return r.windowStats, nil
}

func TestAccountUsageService_PersistRefreshedKiroCredentials_InvalidatesTokenCache(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       88,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "",
			"refresh_token": "refresh-token",
			"expires_at":    time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &kiroUsageAccountRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{},
	}
	invalidator := &tokenCacheInvalidatorStub{}
	svc := &AccountUsageService{
		accountRepo:           repo,
		tokenCacheInvalidator: invalidator,
	}

	newCreds := map[string]any{
		"access_token":  "fresh-token",
		"refresh_token": "fresh-refresh-token",
		"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}

	err := svc.persistRefreshedKiroCredentials(context.Background(), account, newCreds)

	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.NotSame(t, account, repo.updatedAcct)
	require.Equal(t, newCreds, repo.updatedAcct.Credentials)
	require.Equal(t, "fresh-token", account.GetCredential("access_token"))
	require.Equal(t, 1, invalidator.calls)
}

func TestAccountUsageService_PersistRefreshedKiroCredentials_TokenCacheInvalidationFailureIsBestEffort(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       880,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "",
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{}
	invalidator := &tokenCacheInvalidatorStub{err: errors.New("cache unavailable")}
	svc := &AccountUsageService{
		accountRepo:           repo,
		tokenCacheInvalidator: invalidator,
	}

	err := svc.persistRefreshedKiroCredentials(context.Background(), account, map[string]any{
		"access_token":  "fresh-token",
		"refresh_token": "fresh-refresh-token",
	})

	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, 1, invalidator.calls)
	require.Equal(t, "fresh-token", account.GetCredential("access_token"))
}

func TestAccountUsageService_PersistRefreshedKiroCredentials_InvalidatesUsageCache(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       89,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "stale-token",
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
				"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100,"nextDateReset":4102444800}]
			}`)),
			Header: make(http.Header),
		},
	}
	cache := NewUsageCache()
	cache.kiroCache.Store(account.ID, &kiroUsageCache{
		usageInfo: &UsageInfo{
			KiroSubscriptionTitle: "stale",
			UpdatedAt:             ptr(time.Now().Add(-10 * time.Second)),
		},
		timestamp: time.Now(),
	})
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		cache: cache,
	}

	newCreds := map[string]any{
		"access_token":  "fresh-token",
		"refresh_token": "fresh-refresh-token",
		"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}

	require.NoError(t, svc.persistRefreshedKiroCredentials(context.Background(), account, newCreds))

	usage, err := svc.GetUsage(context.Background(), account.ID)
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 1, upstream.calls)
	require.Equal(t, "Kiro Pro", usage.KiroSubscriptionTitle)
}

func TestAccountUsageServiceGetUsageForAccountRejectsNilAccount(t *testing.T) {
	t.Parallel()

	svc := &AccountUsageService{}

	usage, err := svc.getUsageForAccount(context.Background(), nil, false)

	require.Nil(t, usage)
	require.EqualError(t, err, "account is required")
}

func TestAccountUsageServiceGetKiroUsageRejectsNilAccount(t *testing.T) {
	t.Parallel()

	svc := &AccountUsageService{}

	usage, err := svc.getKiroUsage(context.Background(), nil)

	require.Nil(t, usage)
	require.EqualError(t, err, "account is required")
}

func TestAccountUsageService_GetUsage_KiroFailureDoesNotClearRecoverableError(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:           66,
		Platform:     PlatformKiro,
		Type:         AccountTypeOAuth,
		Status:       StatusError,
		ErrorMessage: "token refresh failed",
		Credentials: map[string]any{
			"access_token":  "stale-access-token",
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"message":"expired"}`)),
			Header:     make(http.Header),
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.True(t, usage.NeedsReauth)
	require.Equal(t, errorCodeUnauthenticated, usage.ErrorCode)
	require.Equal(t, 0, repo.clearCalls)
}

func TestAccountUsageService_GetUsage_KiroTokenProviderFailureReturnsDegradedUsage(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       67,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	svc := &AccountUsageService{
		accountRepo:       repo,
		kiroTokenProvider: NewKiroTokenProvider(nil, nil),
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.True(t, usage.NeedsReauth)
	require.Equal(t, errorCodeUnauthenticated, usage.ErrorCode)
}

func TestAccountUsageService_GetUsage_KiroCachesSuccessfulUsage(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       68,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "cached-access-token",
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
				"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100,"nextDateReset":4102444800}]
			}`)),
			Header: make(http.Header),
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		cache: NewUsageCache(),
	}

	first, err := svc.GetUsage(context.Background(), account.ID)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := svc.GetUsage(context.Background(), account.ID)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, 1, upstream.calls)
	require.NotNil(t, second.KiroQuota)
	require.GreaterOrEqual(t, second.KiroQuota.RemainingSeconds, 0)
}

func TestAccountUsageService_GetUsage_KiroIncludesOverageCapabilityAndEmail(t *testing.T) {
	t.Parallel()

	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"userInfo":{"email":"kiro@example.com"},
					"overageConfiguration":{"enabled":true},
					"subscriptionInfo":{"subscriptionTitle":"Kiro Pro","overageCapability":"SUPPORTED"},
					"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100,"nextDateReset":4102444800}]
				}`)),
				Header: make(http.Header),
			}, nil
		},
	}
	account := &Account{
		ID:       991,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"profile_id":         "STALE-PROFILE",
			"login_provider":     "stale-provider",
			"kiro_status_reason": "STALE-REASON",
		},
		Credentials: map[string]any{
			"access_token":   "access-token",
			"refresh_token":  "refresh-token",
			"profile_id":     "PROFILE-123",
			"login_provider": "microsoft",
			"status_reason":  "FEATURE_NOT_SUPPORTED",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		cache: NewUsageCache(),
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, "kiro@example.com", usage.KiroEmail)
	require.Equal(t, "SUPPORTED", usage.KiroOverageCapability)
	require.NotNil(t, usage.KiroOverageEnabled)
	require.True(t, *usage.KiroOverageEnabled)
	require.Equal(t, "PROFILE-123", usage.KiroProfileID)
	require.Equal(t, "microsoft", usage.KiroLoginProvider)
	require.Equal(t, "FEATURE_NOT_SUPPORTED", usage.KiroStatusReason)
}

func TestAccountUsageService_GetUsage_KiroPersistsSchedulerSnapshotIntoExtra(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       680,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"existing": "value",
		},
		Credentials: map[string]any{
			"access_token":  "cached-access-token",
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
				"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100,"nextDateReset":4102444800}]
			}`)),
			Header: make(http.Header),
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		cache: NewUsageCache(),
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Len(t, repo.updateExtraCalls, 1)
	require.Equal(t, []int64{account.ID}, repo.updateExtraIDs)

	updates := repo.updateExtraCalls[0]
	require.Len(t, updates, 3)
	require.Equal(t, usage.KiroQuota.Utilization, updates["kiro_sched_utilization"])
	require.Equal(t, time.Unix(4102444800, 0).UTC().Format(time.RFC3339), updates["kiro_sched_reset_at"])
	require.Equal(t, usage.UpdatedAt.UTC().Format(time.RFC3339), updates["kiro_sched_usage_updated_at"])

	require.Equal(t, "value", account.Extra["existing"])
	require.Equal(t, updates["kiro_sched_utilization"], account.Extra["kiro_sched_utilization"])
	require.Equal(t, updates["kiro_sched_reset_at"], account.Extra["kiro_sched_reset_at"])
	require.Equal(t, updates["kiro_sched_usage_updated_at"], account.Extra["kiro_sched_usage_updated_at"])
}

func TestAccountUsageService_GetUsage_KiroDoesNotMergeSchedulerSnapshotWhenPersistenceFails(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       681,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"existing": "value",
		},
		Credentials: map[string]any{
			"access_token":  "cached-access-token",
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{
		account:        account,
		updateExtraErr: errors.New("write failed"),
	}
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
				"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100,"nextDateReset":4102444800}]
			}`)),
			Header: make(http.Header),
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		cache: NewUsageCache(),
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Len(t, repo.updateExtraCalls, 1)
	require.Equal(t, "value", account.Extra["existing"])
	_, hasUtilization := account.Extra["kiro_sched_utilization"]
	_, hasResetAt := account.Extra["kiro_sched_reset_at"]
	_, hasUpdatedAt := account.Extra["kiro_sched_usage_updated_at"]
	require.False(t, hasUtilization)
	require.False(t, hasResetAt)
	require.False(t, hasUpdatedAt)
}

func TestAccountUsageService_GetUsage_KiroPopulatesWindowStatsFromLocalUsageLogs(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       69,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	usageRepo := &kiroWindowStatsRepoStub{
		windowStats: &usagestats.AccountStats{
			Requests:     27,
			Tokens:       45123,
			Cost:         99.9,
			StandardCost: 88.8,
			UserCost:     77.7,
		},
	}
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
				"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100,"nextDateReset":4102444800}]
			}`)),
			Header: make(http.Header),
		},
	}
	svc := &AccountUsageService{
		accountRepo:  repo,
		usageLogRepo: usageRepo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.NotNil(t, usage.KiroQuota)
	require.NotNil(t, usage.KiroQuota.WindowStats)
	require.Equal(t, int64(27), usage.KiroQuota.WindowStats.Requests)
	require.Equal(t, int64(45123), usage.KiroQuota.WindowStats.Tokens)
	require.Equal(t, 99.9, usage.KiroQuota.WindowStats.Cost)
	require.Equal(t, 77.7, usage.KiroQuota.WindowStats.UserCost)
	require.Equal(t, 88.8, usage.KiroQuota.WindowStats.StandardCost)
	require.True(t, usageRepo.requestedStart.Equal(timezone.StartOfMonth(time.Now())), "window start = %v", usageRepo.requestedStart)
}

func TestAccountUsageService_GetUsage_KiroSeparatesMonthlyBonusAndFreeTrialQuota(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       70,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
				"usageBreakdownList":[{
					"currentUsageWithPrecision":12.5,
					"usageLimitWithPrecision":100,
					"nextDateReset":4102444800,
					"bonuses":[
						{"currentUsage":2,"usageLimit":20,"status":"ACTIVE"},
						{"currentUsage":99,"usageLimit":99,"status":"EXPIRED"}
					],
					"freeTrialInfo":{
						"currentUsageWithPrecision":1.5,
						"usageLimitWithPrecision":10,
						"freeTrialStatus":"ACTIVE"
					}
				}]
			}`)),
			Header: make(http.Header),
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 16.0, usage.KiroCurrentUsage)
	require.Equal(t, 130.0, usage.KiroUsageLimit)
	require.Equal(t, 114.0, usage.KiroRemaining)
	require.NotNil(t, usage.KiroMonthlyQuota)
	require.Equal(t, 12.5, usage.KiroMonthlyQuota.CurrentUsage)
	require.Equal(t, 100.0, usage.KiroMonthlyQuota.UsageLimit)
	require.NotNil(t, usage.KiroBonusQuota)
	require.Equal(t, 2.0, usage.KiroBonusQuota.CurrentUsage)
	require.Equal(t, 20.0, usage.KiroBonusQuota.UsageLimit)
	require.NotNil(t, usage.KiroFreeTrialQuota)
	require.Equal(t, 1.5, usage.KiroFreeTrialQuota.CurrentUsage)
	require.Equal(t, 10.0, usage.KiroFreeTrialQuota.UsageLimit)
	require.NotNil(t, usage.KiroTotalQuota)
	require.Equal(t, 16.0, usage.KiroTotalQuota.CurrentUsage)
	require.Equal(t, 130.0, usage.KiroTotalQuota.UsageLimit)
}

func TestAccountUsageService_GetUsage_KiroRefreshUsesAssignedProxy(t *testing.T) {
	t.Parallel()

	proxyID := int64(501)
	account := &Account{
		ID:       69,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		ProxyID:  &proxyID,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	var refreshProxyURL string
	var usageProxyURL string
	var usageRawQuery string
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			switch {
			case strings.Contains(req.URL.String(), "/refreshToken"):
				refreshProxyURL = proxyURL
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
						"accessToken":"fresh-access-token",
						"refreshToken":"fresh-refresh-token",
						"profileArn":"arn:aws:kiro:us-east-1:123456789012:profile/test",
						"expiresIn":3600
					}`)),
					Header: make(http.Header),
				}, nil
			default:
				usageProxyURL = proxyURL
				usageRawQuery = req.URL.RawQuery
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
						"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
						"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100,"nextDateReset":4102444800}]
					}`)),
					Header: make(http.Header),
				}, nil
			}
		},
	}
	proxyRepo := &kiroDefaultProxyRepoStub{
		getByIDFunc: func(ctx context.Context, id int64) (*Proxy, error) {
			require.Equal(t, proxyID, id)
			return &Proxy{
				Protocol: "http",
				Host:     "127.0.0.1",
				Port:     8089,
			}, nil
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		kiroTokenProvider: newKiroUsageTestProvider(repo, proxyRepo, upstream),
		proxyRepo:         proxyRepo,
		cache:             NewUsageCache(),
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, "http://127.0.0.1:8089", refreshProxyURL)
	require.Equal(t, "http://127.0.0.1:8089", usageProxyURL)
	require.Contains(t, usageRawQuery, "profileArn=")
	require.Equal(t, "fresh-access-token", account.GetCredential("access_token"))
	require.Equal(t, "arn:aws:kiro:us-east-1:123456789012:profile/test", account.GetCredential("profile_arn"))
}

func TestAccountUsageService_GetUsage_KiroRetriesInvalidTokenWithForcedRefresh(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       71,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "cached-stale-token",
			// future expires_at：让 GetAccessToken 不主动预刷新，保留“用现有 token 发请求→上游 401→
			// 强制刷新重试”这一被测场景。
			"refresh_token": "refresh-token",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	invalidator := &tokenCacheInvalidatorStub{}
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			switch {
			case strings.Contains(req.URL.String(), "/refreshToken"):
				require.Equal(t, "refresh-token", account.GetCredential("refresh_token"))
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
						"accessToken":"fresh-access-token",
						"refreshToken":"fresh-refresh-token",
						"profileArn":"arn:aws:kiro:us-east-1:123456789012:profile/test",
						"expiresIn":3600
					}`)),
					Header: make(http.Header),
				}, nil
			default:
				auth := req.Header.Get("Authorization")
				if auth == "Bearer cached-stale-token" {
					return &http.Response{
						StatusCode: http.StatusUnauthorized,
						Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_token","message":"The access token expired"}`)),
						Header:     make(http.Header),
					}, nil
				}
				require.Equal(t, "Bearer fresh-access-token", auth)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{
						"subscriptionInfo":{"subscriptionTitle":"Kiro Pro"},
						"usageBreakdownList":[{"currentUsageWithPrecision":12.5,"usageLimitWithPrecision":100,"nextDateReset":4102444800}]
					}`)),
					Header: make(http.Header),
				}, nil
			}
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		kiroTokenProvider:     newKiroUsageTestProvider(repo, nil, upstream),
		cache:                 NewUsageCache(),
		tokenCacheInvalidator: invalidator,
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, "Kiro Pro", usage.KiroSubscriptionTitle)
	require.Equal(t, "fresh-access-token", account.GetCredential("access_token"))
	require.Equal(t, 3, upstream.calls)
	// 强制刷新前置失效一次；credentials 持久化经 persistAccountCredentials 不再调 invalidator。
	require.Equal(t, 1, invalidator.calls)
	require.Equal(t, 1, repo.updateCalls)
}

func TestAccountUsageService_GetUsage_KiroForcedRefreshFailureReturnsDegradedUsage(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       72,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "cached-stale-token",
			"refresh_token": "refresh-token",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	invalidator := &tokenCacheInvalidatorStub{}
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "/refreshToken") {
				return nil, errors.New("refresh failed")
			}
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_token","message":"The access token expired"}`)),
				Header:     make(http.Header),
			}, nil
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		kiroTokenProvider:     newKiroUsageTestProvider(repo, nil, upstream),
		cache:                 NewUsageCache(),
		tokenCacheInvalidator: invalidator,
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.False(t, usage.NeedsReauth)
	require.Equal(t, errorCodeNetworkError, usage.ErrorCode)
	require.Contains(t, usage.Error, "kiro usage refresh failed after auth retry")
	require.Contains(t, usage.Error, "refresh failed")
	require.Equal(t, 2, upstream.calls)
	require.Equal(t, 1, invalidator.calls)
	require.Equal(t, 0, repo.updateCalls)
}

func TestAccountUsageService_GetUsage_KiroForcedRefreshAuthFailureReturnsReauthUsage(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       73,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "cached-stale-token",
			"refresh_token": "refresh-token",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &kiroUsageAccountRepo{account: account}
	invalidator := &tokenCacheInvalidatorStub{}
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "/refreshToken") {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant","message":"refresh token invalid"}`)),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_token","message":"The access token expired"}`)),
				Header:     make(http.Header),
			}, nil
		},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		usageFetcher: &kiroUsageFetcherStub{
			upstream: upstream,
		},
		kiroTokenProvider:     newKiroUsageTestProvider(repo, nil, upstream),
		cache:                 NewUsageCache(),
		tokenCacheInvalidator: invalidator,
	}

	usage, err := svc.GetUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.True(t, usage.NeedsReauth)
	require.Equal(t, errorCodeUnauthenticated, usage.ErrorCode)
	require.Equal(t, 2, upstream.calls)
	require.Equal(t, 1, invalidator.calls)
	require.Equal(t, 0, repo.updateCalls)
}
