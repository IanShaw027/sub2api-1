//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type kiroUsageAccountRepo struct {
	mockAccountRepoForGemini
	updateCalls int
	updatedAcct *Account
	clearCalls  int
	account     *Account
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
