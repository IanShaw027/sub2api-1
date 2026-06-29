package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type claudeTelemetryAccountRepoStub struct {
	rateLimitAccountRepoStub
	byGroup []Account
	global  []Account
}

func (r *claudeTelemetryAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]Account, error) {
	return filterClaudeTelemetryTestAccounts(r.byGroup, platform), nil
}

func (r *claudeTelemetryAccountRepoStub) ListSchedulableByPlatform(_ context.Context, platform string) ([]Account, error) {
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
	lastReq  *http.Request
	lastBody []byte
}

func (u *claudeTelemetryHTTPUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *claudeTelemetryHTTPUpstreamRecorder) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.lastReq = req
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	return &http.Response{StatusCode: http.StatusAccepted, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`)))}, nil
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
