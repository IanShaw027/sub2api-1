package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type claudeTelemetryAccountRepoStub struct {
	rateLimitAccountRepoStub
	byGroup        []Account
	global         []Account
	lastDeadline   time.Time
	hasDeadline    bool
	observedDone   <-chan struct{}
	blockUntilDone bool
}

func (r *claudeTelemetryAccountRepoStub) ListSchedulableByGroupIDAndPlatform(ctx context.Context, _ int64, platform string) ([]Account, error) {
	r.lastDeadline, r.hasDeadline = ctx.Deadline()
	if r.blockUntilDone {
		r.observedDone = ctx.Done()
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return filterClaudeTelemetryTestAccounts(r.byGroup, platform), nil
}

func (r *claudeTelemetryAccountRepoStub) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	r.lastDeadline, r.hasDeadline = ctx.Deadline()
	if r.blockUntilDone {
		r.observedDone = ctx.Done()
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return filterClaudeTelemetryTestAccounts(r.global, platform), nil
}

func filterClaudeTelemetryTestAccounts(accounts []Account, platform string) []Account {
	out := make([]Account, 0, len(accounts))
	for _, acc := range accounts {
		if acc.Platform == platform && acc.IsSchedulable() {
			out = append(out, acc)
		}
	}
	return out
}

type claudeTelemetryHTTPUpstreamRecorder struct {
	lastReq      *http.Request
	lastBody     []byte
	lastDeadline time.Time
	hasDeadline  bool
	lastProfile  *tlsfingerprint.Profile
}

func (u *claudeTelemetryHTTPUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *claudeTelemetryHTTPUpstreamRecorder) DoWithTLS(req *http.Request, _ string, _ int64, _ int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.lastReq = req
	u.lastProfile = profile
	if req != nil {
		u.lastDeadline, u.hasDeadline = req.Context().Deadline()
	}
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	return &http.Response{StatusCode: http.StatusAccepted, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`)))}, nil
}

func TestForwardClaudeTelemetryBatch_AppliesRequestAwareTLSRuntime(t *testing.T) {
	groupID := int64(12)
	repo := &claudeTelemetryAccountRepoStub{byGroup: []Account{{
		ID: 22, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-token"}, Status: StatusActive, Schedulable: true, Concurrency: 1,
		Extra: map[string]any{"enable_tls_fingerprint": true, "tls_fingerprint_router_id": float64(41)},
	}}}
	upstream := &claudeTelemetryHTTPUpstreamRecorder{}
	router := &model.TLSFingerprintRouter{
		ID: 41, Name: "telemetry", Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name: "claude", Enabled: true, MatchType: model.TLSFingerprintRouterMatchContains,
			Pattern: "claude-cli", TLSFingerprintProfileID: 91,
			UpstreamUserAgent: "claude-cli/telemetry", UpstreamOriginator: "claude-code",
		}},
	}
	svc := &GatewayService{
		accountRepo: repo, httpUpstream: upstream,
		tlsFPProfileService: &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
			91: {ID: 91, Name: "Telemetry Routed", Platform: PlatformAnthropic},
		}},
		tlsFPRouterService: &TLSFingerprintRouterService{localCache: map[int64]*model.TLSFingerprintRouter{41: router}},
	}
	ctx := WithTLSFingerprintInboundUserAgent(context.Background(), "claude-cli/2.0")

	status, err := svc.ForwardClaudeTelemetryBatch(ctx, &groupID, []byte(`{"events":[]}`))

	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, status)
	require.NotNil(t, upstream.lastProfile)
	require.Equal(t, "Telemetry Routed", upstream.lastProfile.Name)
	require.Equal(t, "claude-cli/telemetry", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "claude-code", upstream.lastReq.Header.Get("Originator"))
	require.Equal(t, "Bearer oauth-token", getHeaderRaw(upstream.lastReq.Header, "authorization"))
}

func TestForwardClaudeTelemetryBatch_UsesOnlyAnthropicOAuthAndSanitizes(t *testing.T) {
	groupID := int64(7)
	repo := &claudeTelemetryAccountRepoStub{byGroup: []Account{
		{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key"}, Status: StatusActive, Schedulable: true},
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "oauth-token"}, Extra: map[string]any{"account_uuid": "acc"}, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}}
	upstream := &claudeTelemetryHTTPUpstreamRecorder{}
	svc := &GatewayService{accountRepo: repo, httpUpstream: upstream}
	body := []byte(`{"baseUrl":"https://gateway.example.com","events":[{"event_data":{"device_id":"real","email":"real@example.com","base_url":"https://gateway.example.com","client_metadata":{"process":{"rss":1}},"process":{"rss":2}}}]}`)

	status, err := svc.ForwardClaudeTelemetryBatch(context.Background(), &groupID, body)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, status)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https", upstream.lastReq.URL.Scheme)
	require.Equal(t, "api.anthropic.com", upstream.lastReq.URL.Host)
	require.Equal(t, "/api/event_logging/batch", upstream.lastReq.URL.Path)
	require.Equal(t, "Bearer oauth-token", getHeaderRaw(upstream.lastReq.Header, "authorization"))
	require.Empty(t, upstream.lastReq.Header.Get("x-api-key"))
	require.False(t, gjson.GetBytes(upstream.lastBody, "baseUrl").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "events.0.event_data.base_url").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "events.0.event_data.client_metadata").Exists())
}

func TestForwardClaudeTelemetryBatch_UsesShortForwardDeadline(t *testing.T) {
	groupID := int64(7)
	repo := &claudeTelemetryAccountRepoStub{byGroup: []Account{
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "oauth-token"}, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}}
	upstream := &claudeTelemetryHTTPUpstreamRecorder{}
	svc := &GatewayService{accountRepo: repo, httpUpstream: upstream}

	status, err := svc.ForwardClaudeTelemetryBatch(context.Background(), &groupID, []byte(`{"events":[]}`))

	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, status)
	require.True(t, repo.hasDeadline, "telemetry account selection must also run under the short deadline")
	require.LessOrEqual(t, time.Until(repo.lastDeadline), 5*time.Second)
	require.Positive(t, time.Until(repo.lastDeadline))
	require.True(t, upstream.hasDeadline, "telemetry forwarding must not inherit an unbounded request context")
	require.LessOrEqual(t, time.Until(upstream.lastDeadline), 5*time.Second)
	require.Positive(t, time.Until(upstream.lastDeadline))
}

func TestForwardClaudeTelemetryBatch_AccountSelectionUsesShortTimeout(t *testing.T) {
	groupID := int64(7)
	repo := &claudeTelemetryAccountRepoStub{blockUntilDone: true}
	upstream := &claudeTelemetryHTTPUpstreamRecorder{}
	svc := &GatewayService{accountRepo: repo, httpUpstream: upstream}

	start := time.Now()
	status, err := svc.ForwardClaudeTelemetryBatch(context.Background(), &groupID, []byte(`{"events":[]}`))

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, http.StatusOK, status)
	require.Less(t, time.Since(start), 5*time.Second, "account selection should be bounded by telemetry forward timeout")
	require.True(t, repo.hasDeadline)
	require.NotNil(t, repo.observedDone)
	require.Nil(t, upstream.lastReq)
}

func TestForwardClaudeTelemetryBatch_UsesSetupTokenOAuthCredentialAndSkipsAPIKey(t *testing.T) {
	groupID := int64(9)
	repo := &claudeTelemetryAccountRepoStub{byGroup: []Account{
		{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key"}, Status: StatusActive, Schedulable: true},
		{ID: 3, Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Credentials: map[string]any{"access_token": "setup-token"}, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}}
	upstream := &claudeTelemetryHTTPUpstreamRecorder{}
	svc := &GatewayService{accountRepo: repo, httpUpstream: upstream}

	status, err := svc.ForwardClaudeTelemetryBatch(context.Background(), &groupID, []byte(`{"events":[]}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, status)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "Bearer setup-token", getHeaderRaw(upstream.lastReq.Header, "authorization"))
	require.Empty(t, upstream.lastReq.Header.Get("x-api-key"))
}

func TestForwardClaudeTelemetryBatch_ReturnsNoopWhenOnlyAPIKeyAccountsExist(t *testing.T) {
	groupID := int64(8)
	repo := &claudeTelemetryAccountRepoStub{byGroup: []Account{
		{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "key"}, Status: StatusActive, Schedulable: true},
	}}
	upstream := &claudeTelemetryHTTPUpstreamRecorder{}
	svc := &GatewayService{accountRepo: repo, httpUpstream: upstream}

	status, err := svc.ForwardClaudeTelemetryBatch(context.Background(), &groupID, []byte(`{"events":[]}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, upstream.lastReq)
}
