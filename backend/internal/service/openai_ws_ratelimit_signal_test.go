package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIWSRateLimitSignalRepo struct {
	stubOpenAIAccountRepo
	rateLimitCalls []time.Time
	updateExtra    []map[string]any
	updateExtraErr error
	tempUnsched    []openAIWSTempUnschedCall
}

type openAIWSTempUnschedCacheRecorder struct {
	states []*TempUnschedState
}

type openAICodexSnapshotAsyncRepo struct {
	stubOpenAIAccountRepo
	updateExtraCh chan map[string]any
	rateLimitCh   chan time.Time
}

type openAIWSTempUnschedCall struct {
	id     int64
	until  time.Time
	reason string
}

type openAICodexExtraListRepo struct {
	stubOpenAIAccountRepo
	rateLimitCh chan time.Time
}

type openAIWSAuthTokenInvalidatorRecorder struct {
	accounts []*Account
}

type openAIWSAuth403CounterStub struct {
	counts []int64
}

type openAIWSAuthFailureDialer struct {
	statusCode int
	body       []byte
	dialCount  int
}

type openAIWSAuthFailureBodyError struct {
	err  error
	body []byte
}

func (r *openAIWSAuthTokenInvalidatorRecorder) InvalidateToken(_ context.Context, account *Account) error {
	r.accounts = append(r.accounts, account)
	return nil
}

func (d *openAIWSAuthFailureDialer) Dial(
	ctx context.Context,
	wsURL string,
	headers http.Header,
	proxyURL string,
	tlsProfile *tlsfingerprint.Profile,
) (openAIWSClientConn, int, http.Header, error) {
	_ = ctx
	_ = wsURL
	_ = headers
	_ = proxyURL
	_ = tlsProfile
	d.dialCount++
	return nil, d.statusCode, http.Header{"Server": []string{"unit-test"}}, &openAIWSAuthFailureBodyError{
		err:  errors.New("failed to WebSocket dial: expected handshake response status code 101"),
		body: d.body,
	}
}

func (e *openAIWSAuthFailureBodyError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *openAIWSAuthFailureBodyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *openAIWSAuthFailureBodyError) OpenAIWSHandshakeBody() []byte {
	if e == nil {
		return nil
	}
	return append([]byte(nil), e.body...)
}

func (s *openAIWSAuth403CounterStub) IncrementOpenAI403Count(_ context.Context, _ int64, _ int) (int64, error) {
	if len(s.counts) == 0 {
		return 1, nil
	}
	count := s.counts[0]
	s.counts = s.counts[1:]
	return count, nil
}

func (s *openAIWSAuth403CounterStub) ResetOpenAI403Count(_ context.Context, _ int64) error {
	return nil
}

func (r *openAIWSRateLimitSignalRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	r.rateLimitCalls = append(r.rateLimitCalls, resetAt)
	return nil
}

func (r *openAIWSRateLimitSignalRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	copied := make(map[string]any, len(updates))
	for k, v := range updates {
		copied[k] = v
	}
	r.updateExtra = append(r.updateExtra, copied)
	if r.updateExtraErr != nil {
		return r.updateExtraErr
	}
	return nil
}

func (r *openAIWSRateLimitSignalRepo) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.tempUnsched = append(r.tempUnsched, openAIWSTempUnschedCall{id: id, until: until, reason: reason})
	return nil
}

func (c *openAIWSTempUnschedCacheRecorder) SetTempUnsched(_ context.Context, _ int64, state *TempUnschedState) error {
	if state == nil {
		c.states = append(c.states, nil)
		return nil
	}
	cloned := *state
	c.states = append(c.states, &cloned)
	return nil
}

func (c *openAIWSTempUnschedCacheRecorder) GetTempUnsched(_ context.Context, _ int64) (*TempUnschedState, error) {
	return nil, nil
}

func (c *openAIWSTempUnschedCacheRecorder) DeleteTempUnsched(_ context.Context, _ int64) error {
	return nil
}

func (r *openAICodexSnapshotAsyncRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	if r.rateLimitCh != nil {
		r.rateLimitCh <- resetAt
	}
	return nil
}

func (r *openAICodexSnapshotAsyncRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.updateExtraCh != nil {
		copied := make(map[string]any, len(updates))
		for k, v := range updates {
			copied[k] = v
		}
		r.updateExtraCh <- copied
	}
	return nil
}

func (r *openAICodexExtraListRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	if r.rateLimitCh != nil {
		r.rateLimitCh <- resetAt
	}
	return nil
}

func (r *openAICodexExtraListRepo) ListWithFilters(_ context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	_ = platform
	_ = accountType
	_ = status
	_ = search
	_ = groupID
	_ = privacyMode
	return r.accounts, &pagination.PaginationResult{Total: int64(len(r.accounts)), Page: params.Page, PageSize: params.PageSize}, nil
}

func TestOpenAIGatewayService_PersistSoftRateLimitAdvisorySnapshotTempUnschedulesUntilReset(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resetAt := now.Add(7 * time.Hour)
	repo := &openAIWSRateLimitSignalRepo{}
	cache := &openAIWSTempUnschedCacheRecorder{}
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		rateLimitService: &RateLimitService{
			tempUnschedCache: cache,
		},
	}
	account := &Account{
		ID:          67230,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Name:        "soft-limit@example.com",
	}
	payload := []byte(`{"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":94,"window_minutes":10080,"reset_at":` + strconv.FormatInt(resetAt.Unix(), 10) + `},"secondary":{"used_percent":1,"window_minutes":300,"reset_at":` + strconv.FormatInt(now.Add(3*time.Hour).Unix(), 10) + `}},"credits":{"has_credits":false,"unlimited":false,"balance":null}}`)

	svc.persistOpenAIWSSoftRateLimitAdvisory(context.Background(), account, payload)

	require.Eventually(t, func() bool {
		return len(repo.updateExtra) == 1 && len(repo.tempUnsched) == 1 && len(cache.states) == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, 94.0, repo.updateExtra[0]["codex_7d_used_percent"])
	persistedResetAt, err := time.Parse(time.RFC3339, firstNonEmptyString(repo.updateExtra[0]["codex_7d_reset_at"]))
	require.NoError(t, err)
	require.WithinDuration(t, resetAt, persistedResetAt, 2*time.Second)
	require.Equal(t, 1.0, repo.updateExtra[0]["codex_5h_used_percent"])
	require.Equal(t, int64(67230), repo.tempUnsched[0].id)
	require.WithinDuration(t, resetAt, repo.tempUnsched[0].until, time.Second)
	reasonPayload, ok := parseTempUnschedReasonPayload(repo.tempUnsched[0].reason)
	require.True(t, ok)
	require.Equal(t, AccountSchedulingThresholdReasonSource, reasonPayload.Source)
	require.Equal(t, 94.0, reasonPayload.UsedPercent)
	require.Equal(t, 90, reasonPayload.ThresholdPercent)
	require.Equal(t, resetAt.Unix(), reasonPayload.UntilUnix)
	require.Equal(t, resetAt.Unix(), cache.states[0].UntilUnix)
	require.Contains(t, cache.states[0].ErrorMessage, "94.0% used")
}

func TestOpenAIGatewayService_PersistSoftRateLimitAdvisoryUsesExceededWindowReset(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	exceededResetAt := now.Add(4 * time.Hour)
	laterBelowThresholdResetAt := now.Add(7 * 24 * time.Hour)
	repo := &openAIWSRateLimitSignalRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{
		ID:          73955,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Name:        "soft-limit-5h@example.com",
	}
	payload := []byte(`{"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":4,"window_minutes":10080,"reset_at":` + strconv.FormatInt(laterBelowThresholdResetAt.Unix(), 10) + `},"secondary":{"used_percent":95,"window_minutes":300,"reset_at":` + strconv.FormatInt(exceededResetAt.Unix(), 10) + `}},"credits":{"has_credits":false,"unlimited":false,"balance":null}}`)

	svc.persistOpenAIWSSoftRateLimitAdvisory(context.Background(), account, payload)

	require.Eventually(t, func() bool {
		return len(repo.updateExtra) == 1 && len(repo.tempUnsched) == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, 4.0, repo.updateExtra[0]["codex_7d_used_percent"])
	require.Equal(t, 95.0, repo.updateExtra[0]["codex_5h_used_percent"])
	require.WithinDuration(t, exceededResetAt, repo.tempUnsched[0].until, time.Second)
}

func TestOpenAIGatewayService_PersistOpenAIWSUpstreamErrorSignalMatchesHTTPStatePolicy(t *testing.T) {
	newAccount := func(id int64) *Account {
		return &Account{
			ID:          id,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"access_token":  "access-token",
				"refresh_token": "refresh-token",
			},
		}
	}

	t.Run("top-level token revoked sets permanent error", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := &OpenAIGatewayService{rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil)}
		account := newAccount(92001)
		payload := []byte(`{"type":"error","error":{"code":"token_revoked","message":"token revoked"}}`)

		svc.persistOpenAIWSUpstreamErrorSignal(context.Background(), account, nil, payload, "token_revoked", "", "token revoked")

		require.Equal(t, 1, repo.setErrorCalls)
		require.Contains(t, repo.lastErrorMsg, "Token revoked (401)")
	})

	t.Run("nested deactivated workspace sets permanent error", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := &OpenAIGatewayService{rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil)}
		account := newAccount(92002)
		payload := []byte(`{"type":"response.failed","response":{"error":{"code":"deactivated_workspace","message":"workspace deactivated"}}}`)

		svc.persistOpenAIWSUpstreamErrorSignal(context.Background(), account, nil, payload, "deactivated_workspace", "", "workspace deactivated")

		require.Equal(t, 1, repo.setErrorCalls)
		require.Contains(t, repo.lastErrorMsg, "Workspace deactivated (402)")
	})

	t.Run("permission error uses the OpenAI 403 cooldown", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		rateSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		rateSvc.SetOpenAI403CounterCache(&openAIWSAuth403CounterStub{counts: []int64{1}})
		svc := &OpenAIGatewayService{rateLimitService: rateSvc}
		account := newAccount(92003)
		payload := []byte(`{"type":"error","error":{"type":"permission_error","code":"forbidden","message":"forbidden"}}`)

		svc.persistOpenAIWSUpstreamErrorSignal(context.Background(), account, nil, payload, "forbidden", "permission_error", "forbidden")

		require.Equal(t, 0, repo.setErrorCalls)
		require.Equal(t, 1, repo.tempCalls)
		require.Contains(t, repo.lastTempReason, "OpenAI 403 temporary cooldown (1/3)")
	})

	t.Run("nested usage limit preserves reset time", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := &OpenAIGatewayService{rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil)}
		account := newAccount(92004)
		resetAt := time.Now().Add(2 * time.Hour).Truncate(time.Second)
		payload := []byte(`{"type":"response.failed","response":{"error":{"type":"usage_limit_reached","code":"rate_limit_exceeded","message":"limit reached","resets_at":` + strconv.FormatInt(resetAt.Unix(), 10) + `}}}`)

		svc.persistOpenAIWSUpstreamErrorSignal(context.Background(), account, nil, payload, "rate_limit_exceeded", "usage_limit_reached", "limit reached")

		require.GreaterOrEqual(t, repo.rateLimitedCalls, 1)
		require.WithinDuration(t, resetAt, repo.lastRateLimitedAt, time.Second)
	})
}

func TestOpenAIGatewayService_ResponseFailedSignalDefersAndFlushesOnce(t *testing.T) {
	setGinTestMode()

	repo := &rateLimitAccountRepoStub{}
	rateSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateSvc}
	account := &Account{
		ID:          92005,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusBadGateway)},
		},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	payload := []byte(`{"type":"response.failed","response":{"error":{"code":"server_error","type":"server_error","message":"temporary upstream failure"}}}`)

	for range 3 {
		svc.persistOrDeferOpenAIWSResponseFailedSignal(
			context.Background(), c, account, nil, payload,
			"server_error", "server_error", "temporary upstream failure", true,
		)
	}

	require.Equal(t, 0, repo.setErrorCalls, "internal retries must not advance account error policy")
	_, pending := pendingOpenAIWSAccountStateSignal(c)
	require.True(t, pending)

	svc.FlushOpenAIWSPendingAccountStateSignal(context.Background(), c, account)
	require.Equal(t, 1, repo.setErrorCalls)
	_, pending = pendingOpenAIWSAccountStateSignal(c)
	require.False(t, pending)

	svc.FlushOpenAIWSPendingAccountStateSignal(context.Background(), c, account)
	require.Equal(t, 1, repo.setErrorCalls, "flush must be idempotent")
}

func TestOpenAIGatewayService_ResponseFailedImmediateSignalsBypassDeferral(t *testing.T) {
	for _, tt := range []struct {
		name    string
		code    string
		errType string
	}{
		{name: "revoked token", code: "token_revoked"},
		{name: "deactivated workspace", code: "deactivated_workspace"},
		{name: "forbidden", code: "forbidden", errType: "permission_error"},
		{name: "rate limit", code: "rate_limit_exceeded", errType: "rate_limit_error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, shouldPersistOpenAIWSAccountStateSignalImmediately(tt.code, tt.errType))
		})
	}
	require.False(t, shouldPersistOpenAIWSAccountStateSignalImmediately("server_error", "server_error"))

	repo := &rateLimitAccountRepoStub{}
	svc := &OpenAIGatewayService{rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil)}
	account := &Account{
		ID:          92006,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	payload := []byte(`{"type":"response.failed","response":{"error":{"code":"deactivated_workspace","message":"workspace deactivated"}}}`)

	for range 2 {
		svc.persistOrDeferOpenAIWSResponseFailedSignal(
			context.Background(), c, account, nil, payload,
			"deactivated_workspace", "", "workspace deactivated", true,
		)
	}

	require.Equal(t, 1, repo.setErrorCalls, "immediate signal must apply on the first event but remain request-deduplicated")
	svc.persistOrDeferOpenAIWSResponseFailedSignal(
		context.Background(), c, account, nil,
		[]byte(`{"type":"response.failed","response":{"error":{"code":"server_error","type":"server_error","message":"later transient"}}}`),
		"server_error", "server_error", "later transient", false,
	)
	require.Equal(t, 1, repo.setErrorCalls, "a later response.failed must not advance account state twice in one request")
	_, pending := pendingOpenAIWSAccountStateSignal(c)
	require.False(t, pending)
}

func TestOpenAIGatewayService_Forward_WSv2ErrorEventUsageLimitPersistsRateLimit(t *testing.T) {
	setGinTestMode()

	resetAt := time.Now().Add(2 * time.Hour).Unix()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket failed: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		var req map[string]any
		if err := conn.ReadJSON(&req); err != nil {
			t.Errorf("read ws request failed: %v", err)
			return
		}
		_ = conn.WriteJSON(map[string]any{
			"type": "error",
			"error": map[string]any{
				"code":      "rate_limit_exceeded",
				"type":      "usage_limit_reached",
				"message":   "The usage limit has been reached",
				"resets_at": resetAt,
			},
		})
	}))
	defer wsServer.Close()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "unit-test-agent/1.0")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_http_should_not_run"}`)),
		},
	}

	cfg := newOpenAIWSV2TestConfig()
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true

	account := Account{
		ID:          501,
		Name:        "openai-ws-rate-limit-event",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": wsServer.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}
	repo := &openAIWSRateLimitSignalRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}}}
	rateSvc := &RateLimitService{accountRepo: repo}
	svc := &OpenAIGatewayService{
		accountRepo:      repo,
		rateLimitService: rateSvc,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		cfg:              cfg,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, &account, body)
	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Nil(t, upstream.lastReq, "WS 限流 error event 不应回退到同账号 HTTP")
	require.Len(t, repo.rateLimitCalls, 1)
	require.WithinDuration(t, time.Unix(resetAt, 0), repo.rateLimitCalls[0], 2*time.Second)
}

func TestOpenAIGatewayService_PersistSoftRateLimitAdvisoryContinuesTempUnschedWhenSnapshotPersistFails(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resetAt := now.Add(7 * time.Hour)
	repo := &openAIWSRateLimitSignalRepo{updateExtraErr: errors.New("update extra failed")}
	cache := &openAIWSTempUnschedCacheRecorder{}
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		rateLimitService: &RateLimitService{
			tempUnschedCache: cache,
		},
	}
	account := &Account{
		ID:          84211,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Name:        "soft-limit-failed-extra@example.com",
	}
	payload := []byte(`{"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":94,"window_minutes":10080,"reset_at":` + strconv.FormatInt(resetAt.Unix(), 10) + `},"secondary":{"used_percent":1,"window_minutes":300,"reset_at":` + strconv.FormatInt(now.Add(3*time.Hour).Unix(), 10) + `}},"credits":{"has_credits":false,"unlimited":false,"balance":null}}`)

	svc.persistOpenAIWSSoftRateLimitAdvisory(context.Background(), account, payload)

	require.Eventually(t, func() bool {
		return len(repo.updateExtra) == 1 && len(repo.tempUnsched) == 1 && len(cache.states) == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, int64(84211), repo.tempUnsched[0].id)
	require.WithinDuration(t, resetAt, repo.tempUnsched[0].until, time.Second)
	require.Equal(t, resetAt.Unix(), cache.states[0].UntilUnix)
}

func TestOpenAIGatewayService_Forward_WSv2Handshake429PersistsRateLimit(t *testing.T) {
	setGinTestMode()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-codex-primary-used-percent", "100")
		w.Header().Set("x-codex-primary-reset-after-seconds", "7200")
		w.Header().Set("x-codex-primary-window-minutes", "10080")
		w.Header().Set("x-codex-secondary-used-percent", "3")
		w.Header().Set("x-codex-secondary-reset-after-seconds", "1800")
		w.Header().Set("x-codex-secondary-window-minutes", "300")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_exceeded","message":"rate limited"}}`))
	}))
	defer server.Close()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "unit-test-agent/1.0")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_http_should_not_run"}`)),
		},
	}

	cfg := newOpenAIWSV2TestConfig()
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true

	account := Account{
		ID:          502,
		Name:        "openai-ws-rate-limit-handshake",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": server.URL,
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}
	repo := &openAIWSRateLimitSignalRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}}}
	rateSvc := &RateLimitService{accountRepo: repo}
	svc := &OpenAIGatewayService{
		accountRepo:      repo,
		rateLimitService: rateSvc,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		cfg:              cfg,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"input":[{"type":"input_text","text":"hello"}]}`)
	result, err := svc.Forward(context.Background(), c, &account, body)
	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Nil(t, upstream.lastReq, "WS 握手 429 不应回退到同账号 HTTP")
	require.Len(t, repo.rateLimitCalls, 1)
	require.NotEmpty(t, repo.updateExtra, "握手 429 的 x-codex 头应立即落库")
	require.Contains(t, repo.updateExtra[0], "codex_usage_updated_at")
}

func TestOpenAIGatewayService_Forward_WSv2Handshake401TokenRevokedSetsError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &rateLimitAccountRepoStub{}
	svc, account := newOpenAIWSAuthFailureTestGateway(
		t,
		repo,
		http.StatusUnauthorized,
		[]byte(`{"error":{"code":"token_revoked","message":"Encountered invalidated oauth token for user, failing request"}}`),
	)

	_, _ = svc.Forward(context.Background(), newOpenAIWSAuthFailureTestContext(), account, []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"input_text","text":"hello"}]}`))

	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, 0, repo.tempCalls)
	require.Contains(t, repo.lastErrorMsg, "Token revoked (401)")
	require.Contains(t, repo.lastErrorMsg, "Encountered invalidated oauth token")
}

func TestOpenAIGatewayService_Forward_WSv2Handshake401AppliesOAuthCooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &rateLimitAccountRepoStub{}
	svc, account := newOpenAIWSAuthFailureTestGateway(t, repo, http.StatusUnauthorized, nil)
	invalidator := &openAIWSAuthTokenInvalidatorRecorder{}
	svc.rateLimitService.SetTokenCacheInvalidator(invalidator)

	_, _ = svc.Forward(context.Background(), newOpenAIWSAuthFailureTestContext(), account, []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"input_text","text":"hello"}]}`))

	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.Contains(t, repo.lastTempReason, "Authentication failed (401)")
	require.Len(t, invalidator.accounts, 1)
}

func TestOpenAIGatewayService_Forward_WSv2Handshake403AppliesOpenAI403Cooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &rateLimitAccountRepoStub{}
	svc, account := newOpenAIWSAuthFailureTestGateway(t, repo, http.StatusForbidden, nil)
	svc.rateLimitService.SetOpenAI403CounterCache(&openAIWSAuth403CounterStub{counts: []int64{1}})

	_, _ = svc.Forward(context.Background(), newOpenAIWSAuthFailureTestContext(), account, []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"input_text","text":"hello"}]}`))

	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.Contains(t, repo.lastTempReason, "OpenAI 403 temporary cooldown (1/3)")
}

func TestOpenAIGatewayService_Forward_WSv2Handshake402DeactivatedWorkspaceSetsError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &rateLimitAccountRepoStub{}
	svc, account := newOpenAIWSAuthFailureTestGateway(
		t,
		repo,
		http.StatusPaymentRequired,
		[]byte(`{"detail":{"code":"deactivated_workspace"}}`),
	)

	_, _ = svc.Forward(context.Background(), newOpenAIWSAuthFailureTestContext(), account, []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"input_text","text":"hello"}]}`))

	require.Equal(t, 1, repo.setErrorCalls)
	require.Contains(t, repo.lastErrorMsg, "Workspace deactivated (402)")
}

func TestOpenAIGatewayService_Forward_WSv2Handshake529PersistsOverload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &rateLimitAccountRepoStub{}
	svc, account := newOpenAIWSAuthFailureTestGateway(t, repo, 529, []byte(`{"error":{"message":"overloaded"}}`))

	_, _ = svc.Forward(context.Background(), newOpenAIWSAuthFailureTestContext(), account, []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"input_text","text":"hello"}]}`))

	require.Equal(t, 1, repo.overloadedCalls)
	require.True(t, repo.lastOverloadedUntil.After(time.Now()))
}

func newOpenAIWSAuthFailureTestContext() *gin.Context {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "unit-test-agent/1.0")
	return c
}

func newOpenAIWSAuthFailureTestGateway(t *testing.T, repo *rateLimitAccountRepoStub, statusCode int, handshakeBody []byte) (*OpenAIGatewayService, *Account) {
	t.Helper()

	cfg := newOpenAIWSV2TestConfig()
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(&openAIWSAuthFailureDialer{
		statusCode: statusCode,
		body:       handshakeBody,
	})

	rateSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_http_fallback","object":"response","output":[]}`)),
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:      repo,
		rateLimitService: rateSvc,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		cfg:              cfg,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	rateSvc.SetAccountRuntimeBlocker(svc)

	account := &Account{
		ID:          68005,
		Name:        "openai-ws-auth-failure",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}
	return svc, account
}

func TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_ErrorEventUsageLimitPersistsRateLimit(t *testing.T) {
	setGinTestMode()

	cfg := newOpenAIWSV2TestConfig()
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	resetAt := time.Now().Add(90 * time.Minute).Unix()
	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"error","error":{"code":"rate_limit_exceeded","type":"usage_limit_reached","message":"The usage limit has been reached","resets_at":PLACEHOLDER}}`),
		},
	}
	captureConn.events[0] = []byte(strings.ReplaceAll(string(captureConn.events[0]), "PLACEHOLDER", strconv.FormatInt(resetAt, 10)))
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	account := Account{
		ID:          503,
		Name:        "openai-ingress-rate-limit",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Extra: map[string]any{
			"responses_websockets_v2_enabled": true,
		},
	}
	repo := &openAIWSRateLimitSignalRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}}}
	rateSvc := &RateLimitService{accountRepo: repo}
	svc := &OpenAIGatewayService{
		accountRepo:      repo,
		rateLimitService: rateSvc,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		cfg:              cfg,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}

	serverErrCh := make(chan error, 1)
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			serverErrCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()

		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		req := r.Clone(r.Context())
		req.Header = req.Header.Clone()
		req.Header.Set("User-Agent", "unit-test-agent/1.0")
		ginCtx.Request = req

		readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		msgType, firstMessage, readErr := conn.Read(readCtx)
		cancel()
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		if msgType != coderws.MessageText && msgType != coderws.MessageBinary {
			serverErrCh <- io.ErrUnexpectedEOF
			return
		}

		serverErrCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, &account, "sk-test", firstMessage, nil)
	}))
	defer wsServer.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	cancelDial()
	require.NoError(t, err)
	defer func() { _ = clientConn.CloseNow() }()

	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
	err = clientConn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","stream":false}`))
	cancelWrite()
	require.NoError(t, err)

	select {
	case serverErr := <-serverErrCh:
		require.Error(t, serverErr)
		var failoverErr *UpstreamFailoverError
		require.ErrorAs(t, serverErr, &failoverErr)
		require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
		require.Len(t, repo.rateLimitCalls, 1)
		require.WithinDuration(t, time.Unix(resetAt, 0), repo.rateLimitCalls[0], 2*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("等待 ingress websocket 结束超时")
	}
}

func TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_ResponseFailedMarksTurnFailedAndKeepsClientConnection(t *testing.T) {
	setGinTestMode()

	cfg := newOpenAIWSV2TestConfig()
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	failedConn := &openAIWSCaptureConn{events: [][]byte{
		[]byte(`{"type":"response.failed","response":{"id":"resp_failed","status":"failed","error":{"code":"server_error","type":"server_error","message":"temporary upstream failure"},"usage":{"input_tokens":3,"output_tokens":0,"total_tokens":3}}}`),
	}}
	successConn := &openAIWSCaptureConn{events: [][]byte{
		[]byte(`{"type":"response.completed","response":{"id":"resp_ok","status":"completed","output":[],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`),
	}}
	dialer := &openAIWSSequentialCaptureDialer{conns: []*openAIWSCaptureConn{failedConn, successConn}}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)

	account := Account{
		ID:          504,
		Name:        "openai-ingress-response-failed",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
		},
		Extra: map[string]any{"responses_websockets_v2_enabled": true},
	}
	repo := &rateLimitAccountRepoStub{}
	rateSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{
		accountRepo:      repo,
		rateLimitService: rateSvc,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		cfg:              cfg,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	rateSvc.SetAccountRuntimeBlocker(svc)

	serverErrCh := make(chan error, 1)
	turnErrCh := make(chan error, 2)
	hooks := &OpenAIWSIngressHooks{AfterTurn: func(_ int, _ []byte, _ *OpenAIForwardResult, turnErr error) {
		turnErrCh <- turnErr
	}}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			serverErrCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()

		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		ginCtx.Request = r.Clone(r.Context())
		ginCtx.Request.Header = r.Header.Clone()
		ginCtx.Request.Header.Set("User-Agent", "unit-test-agent/1.0")

		readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		_, firstMessage, readErr := conn.Read(readCtx)
		cancel()
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		serverErrCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, &account, "access-token", firstMessage, hooks)
	}))
	defer wsServer.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	cancelDial()
	require.NoError(t, err)
	defer func() { _ = clientConn.CloseNow() }()

	writeTurn := func(payload string) {
		writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		require.NoError(t, clientConn.Write(writeCtx, coderws.MessageText, []byte(payload)))
		cancel()
	}
	readTurn := func() []byte {
		readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, payload, readErr := clientConn.Read(readCtx)
		cancel()
		require.NoError(t, readErr)
		return payload
	}

	writeTurn(`{"type":"response.create","model":"gpt-5.4","stream":true,"input":[{"type":"input_text","text":"first"}]}`)
	require.Equal(t, "response.failed", gjson.GetBytes(readTurn(), "type").String())
	writeTurn(`{"type":"response.create","model":"gpt-5.4","stream":true,"input":[{"type":"input_text","text":"second"}]}`)
	require.Equal(t, "response.completed", gjson.GetBytes(readTurn(), "type").String())

	require.NoError(t, clientConn.Close(coderws.StatusNormalClosure, "done"))

	firstTurnErr := <-turnErrCh
	secondTurnErr := <-turnErrCh
	require.Error(t, firstTurnErr)
	require.Contains(t, firstTurnErr.Error(), "temporary upstream failure")
	require.NoError(t, secondTurnErr)
	require.Equal(t, 0, repo.setErrorCalls)
	require.GreaterOrEqual(t, dialer.DialCount(), 2)

	select {
	case serverErr := <-serverErrCh:
		require.NoError(t, serverErr)
	case <-time.After(5 * time.Second):
		t.Fatal("等待 ingress websocket 正常结束超时")
	}
}

func TestOpenAIGatewayService_UpdateCodexUsageSnapshot_ExhaustedSnapshotDoesNotSetRateLimit(t *testing.T) {
	repo := &openAICodexSnapshotAsyncRepo{
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &OpenAIGatewayService{accountRepo: repo}
	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:         ptrFloat64WS(100),
		PrimaryResetAfterSeconds:   ptrIntWS(3600),
		PrimaryWindowMinutes:       ptrIntWS(10080),
		SecondaryUsedPercent:       ptrFloat64WS(12),
		SecondaryResetAfterSeconds: ptrIntWS(1200),
		SecondaryWindowMinutes:     ptrIntWS(300),
	}
	svc.updateCodexUsageSnapshot(context.Background(), 601, snapshot)

	select {
	case updates := <-repo.updateExtraCh:
		require.Equal(t, 100.0, updates["codex_7d_used_percent"])
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 快照落库超时")
	}

	select {
	case resetAt := <-repo.rateLimitCh:
		t.Fatalf("不应因仅写入快照而生成运行时限流时间: %v", resetAt)
	case <-time.After(2 * time.Second):
	}
}

func TestOpenAIGatewayService_UpdateCodexUsageSnapshot_NonExhaustedSnapshotDoesNotSetRateLimit(t *testing.T) {
	repo := &openAICodexSnapshotAsyncRepo{
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &OpenAIGatewayService{accountRepo: repo}
	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:         ptrFloat64WS(94),
		PrimaryResetAfterSeconds:   ptrIntWS(3600),
		PrimaryWindowMinutes:       ptrIntWS(10080),
		SecondaryUsedPercent:       ptrFloat64WS(22),
		SecondaryResetAfterSeconds: ptrIntWS(1200),
		SecondaryWindowMinutes:     ptrIntWS(300),
	}
	svc.updateCodexUsageSnapshot(context.Background(), 602, snapshot)

	select {
	case <-repo.updateExtraCh:
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 快照落库超时")
	}

	select {
	case resetAt := <-repo.rateLimitCh:
		t.Fatalf("不应写入运行时限流时间: %v", resetAt)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestOpenAIGatewayService_RecordCodexRateLimitEventUpdatesSnapshotWithoutRateLimit(t *testing.T) {
	repo := &openAICodexSnapshotAsyncRepo{
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 603, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	resetAt := time.Now().Add(90 * time.Minute).Unix()
	payload := []byte(`{"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":95,"window_minutes":300,"reset_at":` + strconv.FormatInt(resetAt, 10) + `},"secondary":null},"credits":{"has_credits":false,"unlimited":false,"balance":null}}`)

	svc.recordOpenAIWSCodexRateLimitSnapshot(context.Background(), account, payload)

	select {
	case updates := <-repo.updateExtraCh:
		require.Equal(t, 95.0, updates["codex_5h_used_percent"])
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex rate_limits 快照落库超时")
	}

	select {
	case resetAt := <-repo.rateLimitCh:
		t.Fatalf("advisory 快照不应写入运行时限流: %v", resetAt)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestOpenAIGatewayService_UpdateCodexUsageSnapshot_ThrottlesExtraWrites(t *testing.T) {
	repo := &openAICodexSnapshotAsyncRepo{
		updateExtraCh: make(chan map[string]any, 2),
	}
	svc := &OpenAIGatewayService{
		accountRepo:           repo,
		codexSnapshotThrottle: newAccountWriteThrottle(time.Hour),
	}
	snapshot := &OpenAICodexUsageSnapshot{
		PrimaryUsedPercent:         ptrFloat64WS(94),
		PrimaryResetAfterSeconds:   ptrIntWS(3600),
		PrimaryWindowMinutes:       ptrIntWS(10080),
		SecondaryUsedPercent:       ptrFloat64WS(22),
		SecondaryResetAfterSeconds: ptrIntWS(1200),
		SecondaryWindowMinutes:     ptrIntWS(300),
	}

	svc.updateCodexUsageSnapshot(context.Background(), 777, snapshot)
	svc.updateCodexUsageSnapshot(context.Background(), 777, snapshot)

	select {
	case <-repo.updateExtraCh:
	case <-time.After(2 * time.Second):
		t.Fatal("等待第一次 codex 快照落库超时")
	}

	select {
	case updates := <-repo.updateExtraCh:
		t.Fatalf("unexpected second codex snapshot write: %v", updates)
	case <-time.After(200 * time.Millisecond):
	}
}

func ptrFloat64WS(v float64) *float64 { return &v }
func ptrIntWS(v int) *int             { return &v }

func TestOpenAIGatewayService_GetSchedulableAccount_ExhaustedCodexExtraDoesNotSetRateLimit(t *testing.T) {
	resetAt := time.Now().Add(6 * 24 * time.Hour)
	account := Account{
		ID:          701,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Extra: map[string]any{
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.UTC().Format(time.RFC3339),
		},
	}
	repo := &openAICodexExtraListRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}}, rateLimitCh: make(chan time.Time, 1)}
	svc := &OpenAIGatewayService{accountRepo: repo}

	fresh, err := svc.getSchedulableAccount(context.Background(), account.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh)
	require.Nil(t, fresh.RateLimitResetAt)
	select {
	case persisted := <-repo.rateLimitCh:
		t.Fatalf("不应将已耗尽的 codex extra 提升为运行时限流状态: %v", persisted)
	case <-time.After(2 * time.Second):
	}
}

func TestAdminService_ListAccounts_ExhaustedCodexExtraDoesNotSetRateLimit(t *testing.T) {
	resetAt := time.Now().Add(4 * 24 * time.Hour)
	repo := &openAICodexExtraListRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:          702,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Extra: map[string]any{
				"codex_7d_used_percent": 100.0,
				"codex_7d_reset_at":     resetAt.UTC().Format(time.RFC3339),
			},
		}}},
		rateLimitCh: make(chan time.Time, 1),
	}
	svc := &adminServiceImpl{accountRepo: repo}

	accounts, total, err := svc.ListAccounts(context.Background(), 1, 20, PlatformOpenAI, AccountTypeOAuth, "", "", 0, "", "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, accounts, 1)
	require.Nil(t, accounts[0].RateLimitResetAt)
	select {
	case persisted := <-repo.rateLimitCh:
		t.Fatalf("不应在账号列表查询时将 codex extra 持久化为运行时限流状态: %v", persisted)
	case <-time.After(2 * time.Second):
	}
}

func TestOpenAIWSErrorHTTPStatusFromRaw_UsageLimitReachedIs429(t *testing.T) {
	require.Equal(t, http.StatusTooManyRequests, openAIWSErrorHTTPStatusFromRaw("", "usage_limit_reached"))
	require.Equal(t, http.StatusTooManyRequests, openAIWSErrorHTTPStatusFromRaw("rate_limit_exceeded", ""))
}
