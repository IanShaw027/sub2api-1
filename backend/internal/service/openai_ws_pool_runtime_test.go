package service

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsOpenAIWSModelUnavailableEvent(t *testing.T) {
	cases := []struct {
		name    string
		code    string
		errType string
		msg     string
		want    bool
	}{
		{"exact code model_not_found", "model_not_found", "invalid_request_error", "", true},
		{"exact code unknown_model", "unknown_model", "", "", true},
		{"explicit phrase model not found", "invalid_request", "invalid_request_error", "the model not found", true},
		{"explicit phrase model is not supported", "invalid_request", "invalid_request_error", "the 'gpt-4o-mini' model is not supported", true},
		// 误判防护：普通 invalid_request 恰好提及 model + does not exist，不应判为 model_unavailable。
		{"field model.metadata does not exist", "invalid_request", "invalid_request_error", "the field model.metadata does not exist on this request.", false},
		{"generic does not exist", "invalid_request", "invalid_request_error", "the referenced model in tool does not exist", false},
		{"unrelated invalid_request", "invalid_request", "invalid_request_error", "missing required field input", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isOpenAIWSModelUnavailableEvent(tc.code, tc.errType, tc.msg)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestClassifyOpenAIWSReconnectReason_ConnectionLimitVsTTLEvict(t *testing.T) {
	// 账户级并发上限：非可重试，走账户级 failover。
	reason, retryable := classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("ws_connection_limit_reached", nil))
	require.Equal(t, "ws_connection_limit_reached", reason)
	require.False(t, retryable)

	// 60min TTL 到期：可重试（evict 后同账号重拨）。
	reason, retryable = classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("ws_conn_ttl_evict", nil))
	require.Equal(t, "ws_conn_ttl_evict", reason)
	require.True(t, retryable)

	// prewarm_ 前缀同样适用。
	reason, retryable = classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("prewarm_ws_conn_ttl_evict", nil))
	require.Equal(t, "prewarm_ws_conn_ttl_evict", reason)
	require.True(t, retryable)
}

func TestOpenAIWSPoolRuntimeSettings_AccessorPriority(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	cfg.Gateway.OpenAIWS.StickyReservePercent = 0
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	// 无运行时快照：min/max/sticky 均回退静态 cfg；0 是有效的静态配置。
	require.Equal(t, 0, pool.minIdlePerAccount())
	require.Equal(t, 4, pool.maxIdlePerAccount())
	require.Equal(t, 0, pool.stickyReservePercent())

	// 三参数 legacy 调用只配置 sticky reserve；idle min/max 回退静态 cfg。
	StoreOpenAIWSPoolRuntimeSettings(2, 6, 40)
	require.Equal(t, 0, pool.minIdlePerAccount())
	require.Equal(t, 4, pool.maxIdlePerAccount())
	require.Equal(t, 40, pool.stickyReservePercent())

	// 0 是有效的运行时设置，表示不为 sticky 会话预留 neutral 之外的容量。
	StoreOpenAIWSPoolRuntimeSettings(2, 6, 0)
	require.Equal(t, 0, pool.stickyReservePercent())

	// 不包含 idle/sticky 兼容快照时，sticky reserve 应回退静态 cfg，而不是硬编码默认值。
	cfg.Gateway.OpenAIWS.StickyReservePercent = 17
	StoreOpenAIWSPoolRuntimeSettings(25, 120)
	require.Equal(t, 17, pool.stickyReservePercent())
}

func TestOpenAIWSPoolRuntimeSettings_LegacyVariadicIdleAndSticky(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	StoreOpenAIWSPoolRuntimeSettings(20, 1000, 2, 6, 40)

	require.Equal(t, 2, pool.minIdlePerAccount())
	require.Equal(t, 6, pool.maxIdlePerAccount())
	require.Equal(t, 40, pool.stickyReservePercent())
	require.Equal(t, 1000*time.Second, pool.sessionIdleTTL())
}

func TestOpenAIWSPoolRuntimeSettings_NeutralPrewarmPercentAndSessionTTLAccessors(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	require.Equal(t, defaultOpenAIWSNeutralPrewarmPercent, pool.neutralPrewarmPercent())
	require.Equal(t, time.Duration(defaultOpenAIWSSessionIdleTTLSeconds)*time.Second, pool.sessionIdleTTL())
	require.Equal(t, 70*time.Second, pool.neutralIdleTTL())

	StoreOpenAIWSPoolRuntimeSettings(25, 180)
	require.Equal(t, 25, pool.neutralPrewarmPercent())
	require.Equal(t, 180*time.Second, pool.sessionIdleTTL())
	require.Equal(t, 70*time.Second, pool.neutralIdleTTL())

	StoreOpenAIWSPoolRuntimeSettings(0, 60)
	require.Equal(t, 0, pool.neutralPrewarmPercent(), "0 disables proactive neutral prewarm")
	require.Equal(t, 60*time.Second, pool.sessionIdleTTL())
	require.Equal(t, pool.sessionIdleTTL(), pool.neutralIdleTTL())

	StoreOpenAIWSPoolRuntimeSettings(25, 1200)
	require.Equal(t, 1000*time.Second, pool.sessionIdleTTL(), "WS idle reuse must not exceed the configured upper bound")
	require.Equal(t, 70*time.Second, pool.neutralIdleTTL())
}

func TestOpenAIWSPool_NeutralAcquireStaleIdleAccessor(t *testing.T) {
	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	require.Equal(t, openAIWSNeutralAcquireStaleIdle, pool.neutralAcquireStaleIdle())

	cfg.Gateway.OpenAIWS.NeutralAcquireStaleIdleSeconds = 7
	require.Equal(t, 7*time.Second, pool.neutralAcquireStaleIdle())
}

func TestOpenAIWSPool_NeutralMaxConns_UsesPrewarmPercent(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	require.Equal(t, 2, pool.neutralMaxConns(10), "default 20 percent target")

	StoreOpenAIWSPoolRuntimeSettings(50, 120)
	require.Equal(t, 5, pool.neutralMaxConns(10))

	StoreOpenAIWSPoolRuntimeSettings(0, 120)
	require.Equal(t, 0, pool.neutralMaxConns(0))
	require.Equal(t, 0, pool.neutralMaxConns(10), "0 percent disables proactive neutral target")
}

func TestOpenAIWSPool_PrewarmNeutral_ColdAccount(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	account := &Account{ID: 4242, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 8}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}

	StoreOpenAIWSPoolRuntimeSettings(25, 120)
	// 冷账号按 8 * 25% 补足到 2 条 neutral 连接。
	pool.PrewarmNeutral(account.ID, req, 2)

	require.Equal(t, 2, dialer.DialCount())
	inflight, waiters, conns := pool.AccountPoolLoad(account.ID)
	require.Equal(t, 0, inflight)
	require.Equal(t, 0, waiters)
	require.Equal(t, 2, conns)

	// 已达 percent target 后再次预热不重复拨号。
	pool.PrewarmNeutral(account.ID, req, 2)
	require.Equal(t, 2, dialer.DialCount())
}

func TestOpenAIWSPool_NeutralAcquireDoesNotReuseDifferentHandshakeIdentity(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	account := &Account{ID: 4243, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 8}
	codexHeaders := http.Header{}
	codexHeaders.Set("originator", "codex_cli_rs")
	codexHeaders.Set("user-agent", codexCLIUserAgent)
	codexHeaders.Set("OpenAI-Beta", openAIWSBetaV2Value)
	nonCodexHeaders := codexHeaders.Clone()
	nonCodexHeaders.Set("originator", "opencode")

	baseReq := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}
	pool.PrewarmNeutral(account.ID, openAIWSAcquireRequest{
		Account: account,
		WSURL:   baseReq.WSURL,
		Headers: codexHeaders,
		Profile: openAIWSConnProfileNeutral,
	}, 1)
	require.Equal(t, 1, dialer.DialCount())
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(0, 120, 0, 4, 0)

	lease, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   baseReq.WSURL,
		Headers: nonCodexHeaders,
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	t.Cleanup(lease.Release)
	require.False(t, lease.Reused())
	require.Equal(t, 2, dialer.DialCount())
}

func TestOpenAIWSPool_NeutralAcquireDoesNotReuseDifferentAuthorization(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(0, 120, 0, 4, 0)

	account := &Account{ID: 4245, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 8}
	headersA := http.Header{}
	headersA.Set("authorization", "Bearer token_a")
	headersA.Set("originator", "codex_cli_rs")
	headersA.Set("user-agent", codexCLIUserAgent)
	headersA.Set("OpenAI-Beta", openAIWSBetaV2Value)
	headersB := headersA.Clone()
	headersB.Set("authorization", "Bearer token_b")

	reqA := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Headers: headersA,
		Profile: openAIWSConnProfileNeutral,
	}
	leaseA, err := pool.Acquire(context.Background(), reqA)
	require.NoError(t, err)
	leaseA.Release()
	require.Equal(t, 1, dialer.DialCount())

	leaseB, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   reqA.WSURL,
		Headers: headersB,
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	t.Cleanup(leaseB.Release)
	require.False(t, leaseB.Reused(), "neutral connections with different authorization headers must not be shared")
	require.Equal(t, 2, dialer.DialCount())
}

func TestOpenAIWSPool_PrewarmNeutralDoesNotCountDifferentAuthorization(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(25, 120, 0, 4, 0)

	account := &Account{ID: 4246, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 8}
	headersA := http.Header{}
	headersA.Set("authorization", "Bearer token_a")
	headersA.Set("originator", "codex_cli_rs")
	headersA.Set("user-agent", codexCLIUserAgent)
	headersA.Set("OpenAI-Beta", openAIWSBetaV2Value)
	headersB := headersA.Clone()
	headersB.Set("authorization", "Bearer token_b")

	baseReq := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}
	pool.PrewarmNeutral(account.ID, openAIWSAcquireRequest{
		Account: account,
		WSURL:   baseReq.WSURL,
		Headers: headersA,
		Profile: openAIWSConnProfileNeutral,
	}, 1)
	require.Equal(t, 1, dialer.DialCount())

	pool.PrewarmNeutral(account.ID, openAIWSAcquireRequest{
		Account: account,
		WSURL:   baseReq.WSURL,
		Headers: headersB,
		Profile: openAIWSConnProfileNeutral,
	}, 2)
	require.Equal(t, 2, dialer.DialCount(), "prewarm must create a separate neutral connection for a different authorization identity when account target has room")
}

func TestOpenAIWSPool_PrewarmNeutralClearsCreatingAfterDial(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	pool.setClientDialerForTest(&openAIWSCountingDialer{})

	account := &Account{ID: 4244, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 8}
	pool.PrewarmNeutral(account.ID, openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}, 2)

	ap, ok := pool.getAccountPool(account.ID)
	require.True(t, ok)
	ap.mu.Lock()
	defer ap.mu.Unlock()
	require.Equal(t, 0, ap.creating)
}

func TestOpenAIWSPool_ReconcileShrink_RespectsNeutralPrewarmTarget(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	account := &Account{ID: 4343, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 8}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}

	// 先在 100% target 下预热 5 条 neutral 空闲连接。
	StoreOpenAIWSPoolRuntimeSettings(100, 120)
	pool.PrewarmNeutral(account.ID, req, 5)
	_, _, conns := pool.AccountPoolLoad(account.ID)
	require.Equal(t, 5, conns)

	// 下调到 25% 后 target=2，reconcile 收缩多余 neutral。
	StoreOpenAIWSPoolRuntimeSettings(25, 120)
	pool.ReconcileShrink()

	// 给收缩中关闭连接留出极短时间（cleanup 同步删表，close 同步执行）。
	require.Eventually(t, func() bool {
		_, _, c := pool.AccountPoolLoad(account.ID)
		return c == 2
	}, time.Second, 10*time.Millisecond)
}

type openAIWSReconcileBlockingAccountRepo struct {
	AccountRepository
	calls        atomic.Int32
	firstEntered chan struct{}
	release      chan struct{}
}

func (r *openAIWSReconcileBlockingAccountRepo) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	_ = platform
	call := r.calls.Add(1)
	if call == 1 {
		close(r.firstEntered)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-r.release:
		}
	}
	return nil, nil
}

func TestOpenAIWSPoolReconcile_RerunsPendingTriggerAfterCurrentRound(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	repo := &openAIWSReconcileBlockingAccountRepo{
		firstEntered: make(chan struct{}),
		release:      make(chan struct{}),
	}
	svc := &OpenAIGatewayService{
		cfg:          cfg,
		openaiWSPool: pool,
		accountRepo:  repo,
	}

	done := make(chan struct{})
	go func() {
		svc.ReconcileOpenAIWSPool(context.Background())
		close(done)
	}()

	select {
	case <-repo.firstEntered:
	case <-time.After(time.Second):
		t.Fatal("first reconcile did not enter account listing")
	}

	svc.ReconcileOpenAIWSPool(context.Background())
	close(repo.release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first reconcile did not finish")
	}
	require.Eventually(t, func() bool {
		return repo.calls.Load() == 2
	}, time.Second, 10*time.Millisecond, "concurrent reconcile trigger should be replayed after the current round")
}
