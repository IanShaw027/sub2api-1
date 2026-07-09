package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type kiroRetryAccountRepo struct {
	stubOpenAIAccountRepo
	updateCalls       int
	updateCredentials int
}

func (r *kiroRetryAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	account, err := r.stubOpenAIAccountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cloned := *account
	cloned.Credentials = cloneCredentials(account.Credentials)
	cloned.Extra = cloneCredentials(account.Extra)
	return &cloned, nil
}

func (r *kiroRetryAccountRepo) Update(_ context.Context, account *Account) error {
	r.updateCalls++
	for i := range r.accounts {
		if r.accounts[i].ID == account.ID {
			r.accounts[i] = *account
			r.accounts[i].Credentials = cloneCredentials(account.Credentials)
			r.accounts[i].Extra = cloneCredentials(account.Extra)
			return nil
		}
	}
	return nil
}

func (r *kiroRetryAccountRepo) UpdateCredentials(_ context.Context, id int64, credentials map[string]any) error {
	r.updateCredentials++
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			r.accounts[i].Credentials = cloneCredentials(credentials)
			return nil
		}
	}
	return nil
}

type kiroRetryTokenCacheInvalidatorStub struct {
	calls int
}

func (s *kiroRetryTokenCacheInvalidatorStub) InvalidateToken(context.Context, *Account) error {
	s.calls++
	return nil
}

type kiroRetryUsageFetcherStub struct {
	upstream HTTPUpstream
}

func (s *kiroRetryUsageFetcherStub) FetchUsage(context.Context, string, string) (*ClaudeUsageResponse, error) {
	return nil, nil
}

func (s *kiroRetryUsageFetcherStub) FetchUsageWithOptions(context.Context, *ClaudeUsageFetchOptions) (*ClaudeUsageResponse, error) {
	return nil, nil
}

func (s *kiroRetryUsageFetcherStub) HTTPUpstream() HTTPUpstream {
	return s.upstream
}

func newKiroUsageRetryTestProvider(repo AccountRepository, upstream HTTPUpstream) *KiroTokenProvider {
	provider := NewKiroTokenProvider(repo, nil)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, nil), NewKiroTokenRefresher().WithTransport(upstream, nil))
	provider.SetRefreshPolicy(KiroProviderRefreshPolicy())
	return provider
}

func TestAccountUsageService_GetUsage_KiroForcedRefreshFailureDoesNotReuseRejectedTokenOnNextAttempt(t *testing.T) {
	t.Parallel()

	repo := &kiroRetryAccountRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{
			accounts: []Account{
				{
					ID:       74,
					Platform: PlatformKiro,
					Type:     AccountTypeOAuth,
					Credentials: map[string]any{
						"access_token":  "cached-stale-token",
						"refresh_token": "refresh-token",
						"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}
	invalidator := &kiroRetryTokenCacheInvalidatorStub{}

	var staleAuthCalls int
	var refreshCalls int
	upstream := &kiroHTTPUpstreamRecorder{
		doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "/refreshToken") {
				refreshCalls++
				return nil, errors.New("refresh failed")
			}

			if req.Header.Get("Authorization") == "Bearer cached-stale-token" {
				staleAuthCalls++
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
		usageFetcher: &kiroRetryUsageFetcherStub{
			upstream: upstream,
		},
		kiroTokenProvider:     newKiroUsageRetryTestProvider(repo, upstream),
		cache:                 NewUsageCache(),
		tokenCacheInvalidator: invalidator,
	}

	first, err := svc.GetUsage(context.Background(), 74)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, errorCodeNetworkError, first.ErrorCode)
	require.Equal(t, 1, staleAuthCalls)
	require.Equal(t, 1, refreshCalls)

	svc.InvalidateKiroUsageCache(74)

	second, err := svc.GetUsage(context.Background(), 74)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, errorCodeNetworkError, second.ErrorCode)
	require.Equal(t, 1, staleAuthCalls, "known-invalid token should not be retried after forced refresh failure")
	require.Equal(t, 2, refreshCalls)
}
