//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type grokQuotaAccountRepo struct {
	*mockAccountRepoForPlatform
	updates               map[int64]map[string]any
	tempUnschedCalls      int
	lastTempUnschedID     int64
	lastTempUnschedUntil  time.Time
	lastTempUnschedReason string
	rateLimitedCalls      int
	lastRateLimitedID     int64
	lastRateLimitedUntil  time.Time
	setErrorCalls         int
	lastSetErrorID        int64
	lastSetErrorMsg       string
}

func (r *grokQuotaAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	if r.updates == nil {
		r.updates = make(map[int64]map[string]any)
	}
	r.updates[id] = updates
	return nil
}

func (r *grokQuotaAccountRepo) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.tempUnschedCalls++
	r.lastTempUnschedID = id
	r.lastTempUnschedUntil = until
	r.lastTempUnschedReason = reason
	return nil
}

func (r *grokQuotaAccountRepo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedCalls++
	r.lastRateLimitedID = id
	r.lastRateLimitedUntil = resetAt
	return nil
}

func (r *grokQuotaAccountRepo) SetError(_ context.Context, id int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastSetErrorID = id
	r.lastSetErrorMsg = errorMsg
	return nil
}

type grokQuotaProxyRepo struct {
	proxyRepoStub
	proxies map[int64]*Proxy
	calls   int
}

type grokBillingRouteResponse struct {
	status int
	body   string
}

type grokBillingRouteUpstream struct {
	mu        sync.Mutex
	routes    map[string]grokBillingRouteResponse
	calls     map[string]int
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
}

func (u *grokBillingRouteUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *grokBillingRouteUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	key := req.URL.RequestURI()
	u.mu.Lock()
	if u.calls == nil {
		u.calls = make(map[string]int)
	}
	u.calls[key]++
	route, ok := u.routes[key]
	u.mu.Unlock()

	if u.started != nil && strings.HasSuffix(key, xai.BillingPathCredits) {
		u.startOnce.Do(func() { close(u.started) })
		<-u.release
	}
	if !ok {
		route = grokBillingRouteResponse{status: http.StatusNotFound, body: `{"error":"not found"}`}
	}
	return &http.Response{
		StatusCode: route.status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(route.body)),
	}, nil
}

func (u *grokBillingRouteUpstream) callCount(suffix string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	total := 0
	for path, count := range u.calls {
		if strings.HasSuffix(path, suffix) {
			total += count
		}
	}
	return total
}

func newGrokBillingTestAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
}

func (r *grokQuotaProxyRepo) GetByID(_ context.Context, id int64) (*Proxy, error) {
	r.calls++
	return r.proxies[id], nil
}

func TestGrokQuotaServiceProbeUsageStoresHeaders(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:          42,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{42: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"X-Ratelimit-Limit-Requests":     []string{"10"},
			"X-Ratelimit-Remaining-Requests": []string{"7"},
			"X-Ratelimit-Reset-Requests":     []string{"2000000000"},
			"X-Ratelimit-Limit-Tokens":       []string{"1000"},
			"X-Ratelimit-Remaining-Tokens":   []string{"900"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"resp_probe"}`)),
	}}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream, nil)

	result, err := svc.ProbeUsage(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.StatusCode)
	require.True(t, result.HeadersObserved)
	require.NotNil(t, result.Snapshot)
	require.True(t, result.Snapshot.HeadersObserved)
	require.Equal(t, "active_probe", result.Snapshot.ObservationSource)
	require.NotEmpty(t, result.Snapshot.LastProbeAt)
	require.NotEmpty(t, result.Snapshot.LastHeadersSeenAt)
	require.NotNil(t, result.Snapshot.Requests)
	require.EqualValues(t, 10, *result.Snapshot.Requests.Limit)
	require.EqualValues(t, 7, *result.Snapshot.Requests.Remaining)
	require.Equal(t, "https://cli-chat-proxy.grok.com/v1/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Contains(t, string(upstream.lastBody), `"max_output_tokens":1`)
	require.Contains(t, string(upstream.lastBody), `"store":false`)
	require.NotNil(t, repo.updates[42][grokQuotaSnapshotExtraKey])
}

func TestBuildGrokQuotaProbeBodyUsesMappedTextResponsesModel(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"grok": "grok-4.3",
			},
		},
	}

	body, err := buildGrokQuotaProbeBody(account)
	require.NoError(t, err)
	require.Equal(t, "grok-4.3", gjson.GetBytes(body, "model").String())
	require.Equal(t, grokQuotaProbeInput, gjson.GetBytes(body, "input").String())
	require.EqualValues(t, 1, gjson.GetBytes(body, "max_output_tokens").Int())
	require.False(t, gjson.GetBytes(body, "store").Bool())
}

func TestGrokQuotaServiceProbeUsageUsesProfileHeadersWhenTLSEnabled(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:          47,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(91),
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{47: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_probe"}`)),
	}}
	svc := NewGrokQuotaService(
		repo,
		nil,
		NewGrokTokenProvider(repo, nil),
		upstream,
		&TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
			91: {ID: 91, Name: "Grok Routed", Platform: "grok", Transport: "http", UserAgent: "grok-native/1.0", Originator: "grok_desktop"},
		}},
	)

	_, err := svc.ProbeUsage(context.Background(), 47)
	require.NoError(t, err)
	require.True(t, upstream.tlsCalled)
	require.Equal(t, defaultGrokUpstreamUserAgent(), upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "grok_desktop", upstream.lastReq.Header.Get("Originator"))
}

func TestGrokQuotaServiceProbeUsageRedactsUpstreamErrorBodyFromErrorAndLogs(t *testing.T) {
	const upstreamSecret = "upstream-secret-refresh-token"
	account := &Account{
		ID:          49,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{49: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(
			`{"error":"` + upstreamSecret + `","detail":"credential rejected"}`,
		)),
	}}
	svc := NewGrokQuotaService(
		repo,
		nil,
		NewGrokTokenProvider(repo, nil),
		upstream,
		nil,
	)

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)

	_, err := svc.ProbeUsage(context.Background(), account.ID)
	require.Error(t, err)
	require.Equal(t, "GROK_QUOTA_PROBE_UPSTREAM_ERROR", infraerrors.Reason(err))
	require.Contains(t, infraerrors.Message(err), `probe model "grok-4.5"`)
	require.NotContains(t, err.Error(), upstreamSecret)
	require.NotContains(t, infraerrors.Message(err), upstreamSecret)
	require.Contains(t, logs.String(), "GROK_QUOTA_PROBE_UPSTREAM_ERROR")
	require.NotContains(t, logs.String(), upstreamSecret)
	require.NotContains(t, logs.String(), "credential rejected")
	require.Equal(t, xai.DefaultCLIBaseURL+"/responses", upstream.lastReq.URL.String())
}

func TestGrokQuotaServiceProbeUsageLoadsProxyWhenAccountEdgeMissing(t *testing.T) {
	t.Parallel()

	proxyID := int64(7)
	account := &Account{
		ID:          46,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		ProxyID:     &proxyID,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{46: account},
		},
	}
	proxyRepo := &grokQuotaProxyRepo{
		proxies: map[int64]*Proxy{
			proxyID: {
				ID:       proxyID,
				Protocol: "http",
				Host:     "proxy.test",
				Port:     3128,
			},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_probe"}`)),
	}}
	svc := NewGrokQuotaService(repo, proxyRepo, NewGrokTokenProvider(repo, nil), upstream, nil)

	_, err := svc.ProbeUsage(context.Background(), 46)
	require.NoError(t, err)
	require.Equal(t, 1, proxyRepo.calls)
	require.Equal(t, "http://proxy.test:3128", upstream.lastProxyURL)
}

func TestGrokQuotaServiceProbeUsageStoresNoHeadersState(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:          45,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{45: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_probe"}`)),
	}}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream, nil)

	result, err := svc.ProbeUsage(context.Background(), 45)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.StatusCode)
	require.False(t, result.HeadersObserved)
	require.NotNil(t, result.Snapshot)
	require.False(t, result.Snapshot.HeadersObserved)
	require.Equal(t, "active_probe", result.Snapshot.ObservationSource)
	require.NotEmpty(t, result.Snapshot.LastProbeAt)
	require.Empty(t, result.Snapshot.LastHeadersSeenAt)

	stored, ok := repo.updates[45][grokQuotaSnapshotExtraKey].(*xai.QuotaSnapshot)
	require.True(t, ok)
	require.False(t, stored.HeadersObserved)
	require.Equal(t, http.StatusOK, stored.StatusCode)
}

func TestGrokQuotaServiceProbeUsageReturnsRateLimitedSnapshot(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       43,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{43: account},
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"45"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
	}}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream, nil)

	result, err := svc.ProbeUsage(context.Background(), 43)
	require.NoError(t, err)
	require.Equal(t, http.StatusTooManyRequests, result.StatusCode)
	require.NotNil(t, result.Snapshot)
	require.NotNil(t, result.Snapshot.RetryAfterSeconds)
	require.Equal(t, 45, *result.Snapshot.RetryAfterSeconds)
}

func TestGrokQuotaServiceResetQuotaUnsupported(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       44,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
	}
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{44: account},
		},
	}
	svc := NewGrokQuotaService(repo, nil, nil, nil, nil)

	_, err := svc.ResetQuota(context.Background(), 44)
	require.Error(t, err)
	require.Equal(t, http.StatusNotImplemented, infraerrors.Code(err))
	require.Equal(t, "GROK_QUOTA_RESET_UNSUPPORTED", infraerrors.Reason(err))
}

func TestGrokQuotaServiceQueryQuotaUsesBillingWithoutInference(t *testing.T) {
	t.Parallel()

	account := newGrokBillingTestAccount(48)
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
	upstream := &grokBillingRouteUpstream{routes: map[string]grokBillingRouteResponse{
		"/v1/billing?format=credits": {status: http.StatusOK, body: `{"config":{"creditUsagePercent":25}}`},
		"/v1/billing":                {status: http.StatusOK, body: `{"config":{"monthlyLimit":{"val":100},"used":{"val":20}}}`},
		"/v1/user?include=subscription": {
			status: http.StatusOK,
			body:   `{"email":"user@example.com","subscriptionTier":"supergrok","hasGrokCodeAccess":true}`,
		},
	}}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream, nil)

	result, err := svc.QueryQuota(context.Background(), account.ID)

	require.NoError(t, err)
	require.Equal(t, "active_billing", result.Source)
	require.NotNil(t, result.Billing)
	require.InDelta(t, 25, result.Billing.Credits.CreditUsagePercent, 0.001)
	require.InDelta(t, 20, xai.Money(result.Billing.Monthly.Used), 0.001)
	require.Equal(t, "supergrok", result.Billing.SubscriptionTier)
	require.Zero(t, upstream.callCount("/responses"), "authoritative billing must not consume inference quota")
}

func TestGrokQuotaServiceFetchBillingRetainsFailedWindowAndSubscription(t *testing.T) {
	t.Parallel()

	account := newGrokBillingTestAccount(49)
	account.Extra = map[string]any{
		grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
			UpdatedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			Credits:   &xai.CreditsBillingConfig{CreditUsagePercent: 10},
			Monthly: &xai.MonthlyBillingConfig{
				MonthlyLimit: &xai.MoneyVal{Val: 100},
				Used:         &xai.MoneyVal{Val: 30},
			},
			SubscriptionTier:  "previous-tier",
			Email:             "previous@example.com",
			HasGrokCodeAccess: true,
		},
	}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
	upstream := &grokBillingRouteUpstream{routes: map[string]grokBillingRouteResponse{
		"/v1/billing?format=credits": {status: http.StatusOK, body: `{"config":{"creditUsagePercent":45}}`},
		"/v1/billing":                {status: http.StatusBadGateway, body: `{"error":"temporary"}`},
		"/v1/user?include=subscription": {
			status: http.StatusOK,
			body:   `{invalid`,
		},
	}}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream, nil)

	snapshot, err := svc.FetchBilling(context.Background(), account.ID)

	require.NoError(t, err, "a retained authoritative window keeps the partial snapshot usable")
	require.InDelta(t, 45, snapshot.Credits.CreditUsagePercent, 0.001)
	require.Same(t, account.Extra[grokBillingSnapshotExtraKey].(*xai.BillingSnapshot).Monthly, snapshot.Monthly)
	require.InDelta(t, 30, xai.Money(snapshot.Monthly.Used), 0.001)
	require.Equal(t, "previous-tier", snapshot.SubscriptionTier)
	require.Equal(t, "previous@example.com", snapshot.Email)
	require.True(t, snapshot.HasGrokCodeAccess)
	require.NotEmpty(t, snapshot.FetchError)
	require.Same(t, snapshot, repo.updates[account.ID][grokBillingSnapshotExtraKey])
}

func TestGrokQuotaServiceFetchBillingDoesNotRefreshTimestampWhenAllWindowsFail(t *testing.T) {
	t.Parallel()

	previousUpdatedAt := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	account := newGrokBillingTestAccount(51)
	account.Extra = map[string]any{
		grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
			UpdatedAt: previousUpdatedAt,
			Credits:   &xai.CreditsBillingConfig{CreditUsagePercent: 10},
			Monthly: &xai.MonthlyBillingConfig{
				MonthlyLimit: &xai.MoneyVal{Val: 100},
				Used:         &xai.MoneyVal{Val: 30},
			},
		},
	}
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
	upstream := &grokBillingRouteUpstream{routes: map[string]grokBillingRouteResponse{
		"/v1/billing?format=credits": {status: http.StatusBadGateway, body: `{"error":"temporary"}`},
		"/v1/billing":                {status: http.StatusBadGateway, body: `{"error":"temporary"}`},
		"/v1/user?include=subscription": {
			status: http.StatusBadGateway,
			body:   `{"error":"temporary"}`,
		},
	}}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream, nil)

	snapshot, err := svc.FetchBilling(context.Background(), account.ID)

	require.Error(t, err)
	require.Equal(t, previousUpdatedAt, snapshot.UpdatedAt)
	require.NotNil(t, snapshot.Credits)
	require.NotNil(t, snapshot.Monthly)
	require.Same(t, snapshot, repo.updates[account.ID][grokBillingSnapshotExtraKey])
}

func TestGrokQuotaServiceFetchBillingSingleflight(t *testing.T) {
	account := newGrokBillingTestAccount(50)
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
	started := make(chan struct{})
	release := make(chan struct{})
	upstream := &grokBillingRouteUpstream{
		started: started,
		release: release,
		routes: map[string]grokBillingRouteResponse{
			"/v1/billing?format=credits": {status: http.StatusOK, body: `{"config":{"creditUsagePercent":25}}`},
			"/v1/billing":                {status: http.StatusOK, body: `{"config":{"monthlyLimit":{"val":100},"used":{"val":20}}}`},
			"/v1/user?include=subscription": {
				status: http.StatusOK,
				body:   `{}`,
			},
		},
	}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream, nil)

	type fetchResult struct {
		snapshot *xai.BillingSnapshot
		err      error
	}
	results := make(chan fetchResult, 2)
	go func() {
		snapshot, err := svc.FetchBilling(context.Background(), account.ID)
		results <- fetchResult{snapshot: snapshot, err: err}
	}()
	<-started
	secondStarted := make(chan struct{})
	go func() {
		close(secondStarted)
		snapshot, err := svc.FetchBilling(context.Background(), account.ID)
		results <- fetchResult{snapshot: snapshot, err: err}
	}()
	<-secondStarted
	time.Sleep(20 * time.Millisecond)
	close(release)

	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Same(t, first.snapshot, second.snapshot)
	require.Equal(t, 1, upstream.callCount(xai.BillingPathCredits))
	require.Equal(t, 1, upstream.callCount(xai.BillingPathMonthly))
	require.Equal(t, 1, upstream.callCount(xai.UserPathSubscription))
}

func TestShouldAutoPauseGrokAccountByQuota(t *testing.T) {
	t.Parallel()

	zero := int64(0)
	limit := int64(10)
	resetFuture := time.Now().Add(time.Minute).Unix()
	retryAfter := 30
	tests := []struct {
		name     string
		snapshot xai.QuotaSnapshot
		want     bool
	}{
		{
			name: "remaining requests exhausted",
			snapshot: xai.QuotaSnapshot{
				Requests:  &xai.QuotaWindow{Limit: &limit, Remaining: &zero, ResetUnix: &resetFuture},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "retry after active",
			snapshot: xai.QuotaSnapshot{
				RetryAfterSeconds: &retryAfter,
				UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "retry after expired",
			snapshot: xai.QuotaSnapshot{
				RetryAfterSeconds: &retryAfter,
				UpdatedAt:         time.Now().Add(-time.Duration(retryAfter+1) * time.Second).UTC().Format(time.RFC3339),
			},
			want: false,
		},
		{
			name: "stale snapshot ignored",
			snapshot: xai.QuotaSnapshot{
				Requests:  &xai.QuotaWindow{Limit: &limit, Remaining: &zero, ResetUnix: &resetFuture},
				UpdatedAt: time.Now().Add(-3 * time.Hour).UTC().Format(time.RFC3339),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			account := &Account{
				Platform: PlatformGrok,
				Type:     AccountTypeOAuth,
				Extra: map[string]any{
					grokQuotaSnapshotExtraKey: tt.snapshot,
				},
			}
			got, _ := shouldAutoPauseGrokAccountByQuota(account)
			require.Equal(t, tt.want, got)
		})
	}
}
