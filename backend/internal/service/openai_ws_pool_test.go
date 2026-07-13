package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSConnPool_CleanupStaleAndTrimIdle(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	pool := newOpenAIWSConnPool(cfg)

	accountID := int64(10)
	ap := pool.getOrCreateAccountPool(accountID)

	stale := newOpenAIWSConn("stale", accountID, nil, nil)
	stale.createdAtNano.Store(time.Now().Add(-2 * time.Hour).UnixNano())
	stale.lastUsedNano.Store(time.Now().Add(-2 * time.Hour).UnixNano())

	idleOld := newOpenAIWSConn("idle_old", accountID, nil, nil)
	// 空闲超过 session idle TTL 上限（1000s），确保被 session_idle_ttl 驱逐。
	idleOld.lastUsedNano.Store(time.Now().Add(-20 * time.Minute).UnixNano())

	idleNew := newOpenAIWSConn("idle_new", accountID, nil, nil)
	idleNew.lastUsedNano.Store(time.Now().Add(-1 * time.Minute).UnixNano())

	ap.conns[stale.id] = stale
	ap.conns[idleOld.id] = idleOld
	ap.conns[idleNew.id] = idleNew

	evicted := pool.cleanupAccountLocked(ap, time.Now(), pool.maxConnsHardCap())
	closeOpenAIWSConns(evicted)

	require.Nil(t, ap.conns["stale"], "stale connection should be rotated")
	require.Nil(t, ap.conns["idle_old"], "idle beyond session idle TTL should be evicted")
	require.NotNil(t, ap.conns["idle_new"], "newer idle should be kept")
}

func TestOpenAIWSConnPool_CleanupNeutralSurplusPreservesSessionBeforeTTL(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)
	defer pool.Close()
	StoreOpenAIWSPoolRuntimeSettings(25, 120)

	accountID := int64(43)
	ap := pool.getOrCreateAccountPool(accountID)
	now := time.Now()
	session := newOpenAIWSConnWithProfile("session_recent", &openAIWSFakeConn{}, nil, openAIWSConnProfileSessionBound)
	session.lastUsedNano.Store(now.Add(-30 * time.Second).UnixNano())
	neutralOld := newOpenAIWSConnWithProfile("neutral_old", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	neutralOld.lastUsedNano.Store(now.Add(-20 * time.Second).UnixNano())
	neutralOld.markNeutralStock()
	neutralNew := newOpenAIWSConnWithProfile("neutral_new", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	neutralNew.lastUsedNano.Store(now.Add(-10 * time.Second).UnixNano())
	neutralNew.markNeutralStock()

	ap.conns[session.id] = session
	ap.conns[neutralOld.id] = neutralOld
	ap.conns[neutralNew.id] = neutralNew

	evicted := pool.cleanupAccountLocked(ap, now, 4)
	closeOpenAIWSConns(evicted)

	require.Len(t, evicted, 1)
	require.NotNil(t, ap.conns[session.id], "session_bound remains until session idle TTL")
	require.Nil(t, ap.conns[neutralOld.id], "oldest surplus neutral should be evicted down to target")
	require.NotNil(t, ap.conns[neutralNew.id])
}

func TestOpenAIWSConnPool_NextConnIDFormat(t *testing.T) {
	pool := newOpenAIWSConnPool(&config.Config{})
	id1 := pool.nextConnID(42)
	id2 := pool.nextConnID(42)

	require.True(t, strings.HasPrefix(id1, "oa_ws_42_"))
	require.True(t, strings.HasPrefix(id2, "oa_ws_42_"))
	require.NotEqual(t, id1, id2)
	require.Equal(t, "oa_ws_42_1", id1)
	require.Equal(t, "oa_ws_42_2", id2)
}

func TestOpenAIWSConnPool_CleanupIntervalsFollowSessionIdleTTL(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	pool := newOpenAIWSConnPool(&config.Config{})
	t.Cleanup(pool.Close)

	require.Equal(t, 3*time.Second, pool.acquireCleanupInterval())
	require.Equal(t, 30*time.Second, pool.backgroundSweepInterval())

	StoreOpenAIWSPoolRuntimeSettings(25, 1)
	require.Equal(t, time.Second, pool.acquireCleanupInterval(), "short session idle TTL should pull acquire cleanup down to the same 1s cadence")
	require.Equal(t, time.Second, pool.backgroundSweepInterval(), "short session idle TTL should also pull background cleanup down to the same 1s cadence")

	StoreOpenAIWSPoolRuntimeSettings(25, 120)
	require.Equal(t, 3*time.Second, pool.acquireCleanupInterval())
	require.Equal(t, 30*time.Second, pool.backgroundSweepInterval())
}

func TestOpenAIWSConnPool_BackgroundHealthCheckEvictsOnlyEligibleFailedConns(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	account := &Account{ID: 44, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 8}
	ap := pool.getOrCreateAccountPool(account.ID)
	now := time.Now()
	newConn := func(id string, probe *openAIWSHealthProbeConn) *openAIWSConn {
		conn := newOpenAIWSConnWithProfile(id, probe, nil, openAIWSConnProfileSessionBound)
		conn.lastUsedNano.Store(now.Add(-(openAIWSBackgroundPingIdle + time.Second)).UnixNano())
		return conn
	}

	failedProbe := &openAIWSHealthProbeConn{pingErr: errors.New("stale socket")}
	healthyProbe := &openAIWSHealthProbeConn{}
	leasedProbe := &openAIWSHealthProbeConn{pingErr: errors.New("must not ping leased")}
	pinnedProbe := &openAIWSHealthProbeConn{pingErr: errors.New("must not ping pinned")}
	waiterProbe := &openAIWSHealthProbeConn{pingErr: errors.New("must not ping waiter")}

	failed := newConn("failed", failedProbe)
	healthy := newConn("healthy", healthyProbe)
	leased := newConn("leased", leasedProbe)
	pinned := newConn("pinned", pinnedProbe)
	waiter := newConn("waiter", waiterProbe)
	require.True(t, leased.tryAcquire())
	t.Cleanup(leased.release)
	pinnedCount := 1
	waiter.waiters.Store(1)

	ap.mu.Lock()
	ap.lastAcquire = &openAIWSAcquireRequest{Account: account, Profile: openAIWSConnProfileSessionBound}
	for _, conn := range []*openAIWSConn{failed, healthy, leased, pinned, waiter} {
		ap.conns[conn.id] = conn
	}
	ap.pinnedConns[pinned.id] = pinnedCount
	ap.mu.Unlock()

	pool.runBackgroundCleanupSweep(now)

	ap.mu.Lock()
	_, failedKept := ap.conns[failed.id]
	_, healthyKept := ap.conns[healthy.id]
	_, leasedKept := ap.conns[leased.id]
	_, pinnedKept := ap.conns[pinned.id]
	_, waiterKept := ap.conns[waiter.id]
	ap.mu.Unlock()
	require.False(t, failedKept, "failed idle connection should be evicted")
	require.True(t, healthyKept)
	require.True(t, leasedKept)
	require.True(t, pinnedKept)
	require.True(t, waiterKept)
	require.Equal(t, int32(1), failedProbe.pings.Load())
	require.Equal(t, int32(1), healthyProbe.pings.Load())
	require.Equal(t, int32(0), leasedProbe.pings.Load())
	require.Equal(t, int32(0), pinnedProbe.pings.Load())
	require.Equal(t, int32(0), waiterProbe.pings.Load())
	require.False(t, healthy.isLeased(), "healthy probe must release its temporary lease")

	pool.runBackgroundCleanupSweep(now.Add(time.Second))
	require.Equal(t, int32(1), healthyProbe.pings.Load(), "healthy connection should not be probed again before the sweep interval")
}

func TestOpenAIWSRequestPathPingTimeoutIsBounded(t *testing.T) {
	require.Equal(t, 750*time.Millisecond, openAIWSRequestPathPingTO)
	require.Less(t, openAIWSRequestPathPingTO, openAIWSConnHealthCheckTO)
}

func TestOpenAIWSConnLease_WriteJSONAndGuards(t *testing.T) {
	conn := newOpenAIWSConn("lease_write", 1, &openAIWSFakeConn{}, nil)
	lease := &openAIWSConnLease{conn: conn}
	require.NoError(t, lease.WriteJSON(map[string]any{"type": "response.create"}, 0))

	var nilLease *openAIWSConnLease
	err := nilLease.WriteJSONWithContextTimeout(context.Background(), map[string]any{"type": "response.create"}, time.Second)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)

	err = (&openAIWSConnLease{}).WriteJSONWithContextTimeout(context.Background(), map[string]any{"type": "response.create"}, time.Second)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
}

func TestOpenAIWSConn_WriteJSONWithTimeout_NilParentContextUsesBackground(t *testing.T) {
	probe := &openAIWSContextProbeConn{}
	conn := newOpenAIWSConn("ctx_probe", 1, probe, nil)
	require.NoError(t, conn.writeJSONWithTimeout(context.Background(), map[string]any{"type": "response.create"}, 0))
	require.NotNil(t, probe.lastWriteCtx)
}

func TestOpenAIWSConnPool_TargetConnCountAdaptive(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 6
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.PoolTargetUtilization = 0.5

	pool := newOpenAIWSConnPool(cfg)
	ap := pool.getOrCreateAccountPool(88)

	conn1 := newOpenAIWSConn("c1", 88, nil, nil)
	conn2 := newOpenAIWSConn("c2", 88, nil, nil)
	require.True(t, conn1.tryAcquire())
	require.True(t, conn2.tryAcquire())
	conn1.waiters.Store(1)
	conn2.waiters.Store(1)

	ap.conns[conn1.id] = conn1
	ap.conns[conn2.id] = conn2

	target := pool.targetConnCountLocked(ap, pool.maxConnsHardCap())
	require.Equal(t, 6, target, "应按 inflight+waiters 与 target_utilization 自适应扩容到上限")

	conn1.release()
	conn2.release()
	conn1.waiters.Store(0)
	conn2.waiters.Store(0)
	target = pool.targetConnCountLocked(ap, pool.maxConnsHardCap())
	require.Equal(t, 1, target, "低负载时应缩回到最小空闲连接")
}

func TestOpenAIWSConnPool_TargetConnCountMinIdleZero(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.PoolTargetUtilization = 0.8

	pool := newOpenAIWSConnPool(cfg)
	ap := pool.getOrCreateAccountPool(66)

	target := pool.targetConnCountLocked(ap, pool.maxConnsHardCap())
	require.Equal(t, 0, target, "min_idle=0 且无负载时应允许缩容到 0")
}

func TestOpenAIWSConnPool_NeutralPrewarmTargetUsesAccountConcurrencyPercent(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	account := &Account{ID: 67, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 15}

	StoreOpenAIWSPoolRuntimeSettings(20, 120)
	require.Equal(t, 3, pool.neutralPrewarmTargetForAccount(account), "floor(15 * 20 / 100)")

	StoreOpenAIWSPoolRuntimeSettings(0, 120)
	require.Equal(t, 0, pool.neutralPrewarmTargetForAccount(account), "0 percent disables proactive neutral prewarm")

	StoreOpenAIWSPoolRuntimeSettings(100, 120)
	require.Equal(t, 15, pool.neutralPrewarmTargetForAccount(account), "100 percent matches account concurrency")
}

func TestOpenAIWSConnPool_AccountPoolSnapshotCapturesInventoryAndRequestMatches(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(60, 600, 1, 8, 0)

	account := &Account{ID: 680, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 5}
	req := openAIWSAcquireRequest{Account: account, WSURL: "wss://example.invalid/ws", Profile: openAIWSConnProfileNeutral}
	matching := newOpenAIWSConnWithProfileAndReuseKey("matching", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, openAIWSConnReuseKeyForAcquire(req))
	matching.markNeutralStock()
	mismatched := newOpenAIWSConnWithProfileAndReuseKey("mismatched", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, "different")
	session := newOpenAIWSConnWithProfile("session", &openAIWSFakeConn{}, nil, openAIWSConnProfileSessionBound)
	require.True(t, session.tryAcquire())
	t.Cleanup(session.release)

	ap := pool.getOrCreateAccountPool(account.ID)
	ap.mu.Lock()
	ap.conns[matching.id] = matching
	ap.conns[mismatched.id] = mismatched
	ap.conns[session.id] = session
	ap.creating = 1
	ap.creatingNeutral = 1
	ap.prewarmActive = true
	ap.prewarmFails = 2
	ap.mu.Unlock()

	snapshot := pool.AccountPoolSnapshot(account, req)
	require.Equal(t, 3, snapshot.TotalConns)
	require.Equal(t, 2, snapshot.NeutralConns)
	require.Equal(t, 1, snapshot.SessionBoundConns)
	require.Equal(t, 2, snapshot.IdleNeutralConns)
	require.Equal(t, 0, snapshot.IdleSessionBoundConns)
	require.Equal(t, 1, snapshot.NeutralStockConns)
	require.Equal(t, 1, snapshot.MatchingConns)
	require.Equal(t, 1, snapshot.MatchingIdleConns)
	require.Equal(t, 1, snapshot.LeasedConns)
	require.Equal(t, 1, snapshot.Creating)
	require.True(t, snapshot.PrewarmActive)
	require.Equal(t, 2, snapshot.PrewarmFailures)
	require.Equal(t, 3, snapshot.NeutralPrewarmTarget)
	require.Equal(t, "matching_idle_available", classifyOpenAIWSPoolAcquireState(snapshot, req))
	require.Equal(t, "force_new_conn", classifyOpenAIWSPoolAcquireState(snapshot, openAIWSAcquireRequest{ForceNewConn: true}))
	require.Equal(t, "no_matching_variant", classifyOpenAIWSPoolAcquireState(openAIWSAccountPoolSnapshot{TotalConns: 2}, req))
}

func TestOpenAIWSConnPool_NeutralPrewarmTargetHonorsMinIdleForLowConcurrency(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	StoreOpenAIWSPoolRuntimeSettingsWithIdle(20, 120, 1, 4, 30)

	lowConcurrency := &Account{ID: 671, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 2}
	require.Equal(t, 1, pool.neutralPrewarmTargetForAccount(lowConcurrency), "low concurrency accounts should still keep the configured min idle neutral connection")

	threeWayConcurrency := &Account{ID: 672, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 3}
	require.Equal(t, 1, pool.neutralPrewarmTargetForAccount(threeWayConcurrency), "configured min_idle should avoid a zero prewarm target")
}

func TestOpenAIWSConnPool_NeutralPrewarmTargetAccountsForStickyReserve(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)

	StoreOpenAIWSPoolRuntimeSettingsWithIdle(50, 120, 0, 10, 30)
	require.Equal(t, 3, pool.neutralPrewarmTargetForConcurrency(10), "neutral target should scale from fresh-session capacity after sticky reserve")

	StoreOpenAIWSPoolRuntimeSettingsWithIdle(50, 120, 0, 10, 50)
	require.Equal(t, 1, pool.neutralPrewarmTargetForConcurrency(4), "sticky reserve should reduce the neutral prewarm base before applying percent")
}

func TestOpenAIWSConnPool_CleanupEvictsSessionAfterTTLAndNeutralSurplus(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 64
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	StoreOpenAIWSPoolRuntimeSettings(25, 120)

	accountConcurrency := 8
	accountID := int64(68)
	ap := pool.getOrCreateAccountPool(accountID)
	now := time.Now()

	sessionExpired := newOpenAIWSConnWithProfile("session_expired", &openAIWSFakeConn{}, nil, openAIWSConnProfileSessionBound)
	sessionExpired.lastUsedNano.Store(now.Add(-121 * time.Second).UnixNano())
	sessionRecent := newOpenAIWSConnWithProfile("session_recent", &openAIWSFakeConn{}, nil, openAIWSConnProfileSessionBound)
	sessionRecent.lastUsedNano.Store(now.Add(-119 * time.Second).UnixNano())
	neutralOld := newOpenAIWSConnWithProfile("neutral_old", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	neutralOld.lastUsedNano.Store(now.Add(-30 * time.Second).UnixNano())
	neutralOld.markNeutralStock()
	neutralMid := newOpenAIWSConnWithProfile("neutral_mid", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	neutralMid.lastUsedNano.Store(now.Add(-20 * time.Second).UnixNano())
	neutralMid.markNeutralStock()
	neutralNew := newOpenAIWSConnWithProfile("neutral_new", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	neutralNew.lastUsedNano.Store(now.Add(-10 * time.Second).UnixNano())
	neutralNew.markNeutralStock()

	ap.conns[sessionExpired.id] = sessionExpired
	ap.conns[sessionRecent.id] = sessionRecent
	ap.conns[neutralOld.id] = neutralOld
	ap.conns[neutralMid.id] = neutralMid
	ap.conns[neutralNew.id] = neutralNew

	evicted := pool.cleanupAccountLocked(ap, now, accountConcurrency)
	closeOpenAIWSConns(evicted)

	require.Nil(t, ap.conns[sessionExpired.id], "session-bound idle conn should be evicted after TTL")
	require.NotNil(t, ap.conns[sessionRecent.id], "session-bound idle conn below TTL should remain")
	require.Nil(t, ap.conns[neutralOld.id], "oldest surplus neutral should be evicted down to target")
	require.NotNil(t, ap.conns[neutralMid.id])
	require.NotNil(t, ap.conns[neutralNew.id])
	require.Equal(t, 2, countConnsByProfileLocked(ap, openAIWSConnProfileNeutral), "25 percent of concurrency 8 keeps 2 neutral conns")
}

func TestOpenAIWSConnPool_CleanupEvictsNeutralAfterIdleTTL(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 64
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	StoreOpenAIWSPoolRuntimeSettings(100, 120)

	accountID := int64(69)
	ap := pool.getOrCreateAccountPool(accountID)
	now := time.Now()
	neutralIdleTTL := pool.neutralIdleTTL()

	neutralExpired := newOpenAIWSConnWithProfile("neutral_expired", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	neutralExpired.lastUsedNano.Store(now.Add(-(neutralIdleTTL + time.Second)).UnixNano())
	neutralRecent := newOpenAIWSConnWithProfile("neutral_recent", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	neutralRecent.lastUsedNano.Store(now.Add(-(neutralIdleTTL - time.Second)).UnixNano())

	ap.conns[neutralExpired.id] = neutralExpired
	ap.conns[neutralRecent.id] = neutralRecent

	evicted := pool.cleanupAccountLocked(ap, now, 4)
	closeOpenAIWSConns(evicted)

	require.Nil(t, ap.conns[neutralExpired.id], "neutral idle conn beyond TTL should be refreshed before serving real traffic")
	require.NotNil(t, ap.conns[neutralRecent.id], "neutral idle conn below TTL should remain reusable")
	require.Equal(t, 1, countConnsByProfileLocked(ap, openAIWSConnProfileNeutral))
}

func TestOpenAIWSConnPool_AcquireEvictsExpiredIdleCandidateBeforeReuse(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 64
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(0, 120, 0, 4, 0)

	account := &Account{ID: 701, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 4}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}
	reuseKey := openAIWSConnReuseKeyForAcquire(req)
	ap := pool.getOrCreateAccountPool(account.ID)
	now := time.Now()
	neutralIdleTTL := pool.neutralIdleTTL()
	expired := newOpenAIWSConnWithProfileAndReuseKey("neutral_expired_for_acquire", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, reuseKey)
	expired.lastUsedNano.Store(now.Add(-neutralIdleTTL).UnixNano())
	ap.mu.Lock()
	ap.lastCleanupAt = now
	ap.conns[expired.id] = expired
	ap.mu.Unlock()

	lease, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(lease.Release)
	require.False(t, lease.Reused(), "expired idle conn must not be reused before the next cleanup interval")
	require.Equal(t, 1, dialer.DialCount())

	ap.mu.Lock()
	_, stillPresent := ap.conns[expired.id]
	ap.mu.Unlock()
	require.False(t, stillPresent, "expired idle conn should be evicted synchronously during acquire")
}

func TestOpenAIWSConnPool_AcquireEvictsStaleNeutralBeforeTTL(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 64
	// 默认 stale-idle 阈值已抬高到与 session idle TTL 上限相同（1000s），
	// "stale 早于 TTL 被急切驱逐" 的场景需要显式配置一个低于 TTL 的阈值才能触发。
	cfg.Gateway.OpenAIWS.NeutralAcquireStaleIdleSeconds = 60
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	StoreOpenAIWSPoolRuntimeSettings(100, 600)

	account := &Account{ID: 702, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 4}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}
	reuseKey := openAIWSConnReuseKeyForAcquire(req)
	ap := pool.getOrCreateAccountPool(account.ID)
	now := time.Now()
	stale := newOpenAIWSConnWithProfileAndReuseKey("neutral_stale_for_acquire", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, reuseKey)
	stale.lastUsedNano.Store(now.Add(-(pool.neutralAcquireStaleIdle() + time.Second)).UnixNano())
	ap.mu.Lock()
	ap.lastCleanupAt = now
	ap.conns[stale.id] = stale
	ap.mu.Unlock()

	lease, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(lease.Release)
	require.False(t, lease.Reused(), "stale neutral idle conn should be redialed before real traffic hits it")
	require.GreaterOrEqual(t, dialer.DialCount(), 1)

	ap.mu.Lock()
	_, stillPresent := ap.conns[stale.id]
	ap.mu.Unlock()
	require.False(t, stillPresent, "stale neutral conn should be evicted synchronously during acquire")

	metrics := pool.SnapshotMetrics()
	require.Equal(t, int64(1), metrics.AcquireStaleEvictTotal)
}

func TestOpenAIWSConnPool_AcquireEvictsStaleNeutralUsingConfiguredThreshold(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 64
	cfg.Gateway.OpenAIWS.NeutralAcquireStaleIdleSeconds = 2
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	StoreOpenAIWSPoolRuntimeSettings(100, 600)

	account := &Account{ID: 703, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 4}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}
	reuseKey := openAIWSConnReuseKeyForAcquire(req)
	ap := pool.getOrCreateAccountPool(account.ID)
	now := time.Now()
	stale := newOpenAIWSConnWithProfileAndReuseKey("neutral_stale_configured", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, reuseKey)
	stale.lastUsedNano.Store(now.Add(-3 * time.Second).UnixNano())
	ap.mu.Lock()
	ap.lastCleanupAt = now
	ap.conns[stale.id] = stale
	ap.mu.Unlock()

	lease, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(lease.Release)
	require.False(t, lease.Reused(), "configured stale-idle threshold should trigger redial before reuse")
	require.GreaterOrEqual(t, dialer.DialCount(), 1)

	ap.mu.Lock()
	_, stillPresent := ap.conns[stale.id]
	ap.mu.Unlock()
	require.False(t, stillPresent, "stale neutral conn should be evicted synchronously during acquire")

	metrics := pool.SnapshotMetrics()
	require.Equal(t, int64(1), metrics.AcquireStaleEvictTotal)
}

func TestOpenAIWSConnPool_BackgroundCleanupRefreshesExpiredNeutralIdle(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	StoreOpenAIWSPoolRuntimeSettings(50, 120)

	account := &Account{ID: 70, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 4}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}
	reuseKey := openAIWSConnReuseKeyForAcquire(req)

	ap := pool.getOrCreateAccountPool(account.ID)
	now := time.Now()
	neutralIdleTTL := pool.neutralIdleTTL()
	expiredA := newOpenAIWSConnWithProfileAndReuseKey("neutral_expired_a", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, reuseKey)
	expiredA.lastUsedNano.Store(now.Add(-(neutralIdleTTL + time.Second)).UnixNano())
	expiredB := newOpenAIWSConnWithProfileAndReuseKey("neutral_expired_b", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, reuseKey)
	expiredB.lastUsedNano.Store(now.Add(-(neutralIdleTTL + time.Second)).UnixNano())
	ap.mu.Lock()
	ap.lastNeutralAcquire = cloneOpenAIWSAcquireRequestPtr(&req)
	ap.conns[expiredA.id] = expiredA
	ap.conns[expiredB.id] = expiredB
	ap.mu.Unlock()

	pool.runBackgroundCleanupSweep(now)

	require.Eventually(t, func() bool {
		ap.mu.Lock()
		defer ap.mu.Unlock()
		_, hasExpiredA := ap.conns[expiredA.id]
		_, hasExpiredB := ap.conns[expiredB.id]
		return !hasExpiredA &&
			!hasExpiredB &&
			countConnsByProfileAndReuseKeyLocked(ap, openAIWSConnProfileNeutral, reuseKey) == 2 &&
			dialer.DialCount() == 2
	}, time.Second, 10*time.Millisecond)
}

func TestOpenAIWSConnPool_SessionAcquireCanCreateBeyondAccountConcurrencyWithoutEvictingIdleSession(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	account := &Account{ID: 69, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileSessionBound,
	}

	first, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	firstConnID := first.ConnID()
	first.Release()

	second, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(second.Release)
	require.NotEqual(t, firstConnID, second.ConnID(), "new session request without affinity creates its own conn")
	require.Equal(t, 2, dialer.DialCount())

	ap, ok := pool.getAccountPool(account.ID)
	require.True(t, ok)
	ap.mu.Lock()
	_, firstStillCached := ap.conns[firstConnID]
	totalConns := len(ap.conns)
	ap.mu.Unlock()
	require.True(t, firstStillCached, "idle session conn remains available for a follow-up turn until TTL")
	require.Equal(t, 2, totalConns, "idle session WS cache can exceed account concurrency")
}

func TestOpenAIWSConnPool_EnsureTargetIdleAsync(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.PoolTargetUtilization = 0.8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 1

	pool := newOpenAIWSConnPool(cfg)
	StoreOpenAIWSPoolRuntimeSettings(50, 120)
	pool.setClientDialerForTest(&openAIWSFakeDialer{})

	accountID := int64(77)
	account := &Account{ID: accountID, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}
	ap := pool.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	ap.lastNeutralAcquire = &openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}
	ap.mu.Unlock()

	pool.ensureTargetIdleAsync(accountID)

	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(accountID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return countConnsByProfileLocked(ap, openAIWSConnProfileNeutral) >= 2
	}, 2*time.Second, 20*time.Millisecond)

	metrics := pool.SnapshotMetrics()
	require.GreaterOrEqual(t, metrics.ScaleUpTotal, int64(2))
}

func TestOpenAIWSConnPool_EnsureTargetIdleAsyncCooldown(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.PoolTargetUtilization = 0.8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 1
	cfg.Gateway.OpenAIWS.PrewarmCooldownMS = 500

	pool := newOpenAIWSConnPool(cfg)
	StoreOpenAIWSPoolRuntimeSettings(50, 120)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	accountID := int64(178)
	account := &Account{ID: accountID, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}
	ap := pool.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	ap.lastNeutralAcquire = &openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}
	ap.mu.Unlock()

	pool.ensureTargetIdleAsync(accountID)
	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(accountID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return countConnsByProfileLocked(ap, openAIWSConnProfileNeutral) >= 2 && !ap.prewarmActive
	}, 2*time.Second, 20*time.Millisecond)
	firstDialCount := dialer.DialCount()
	require.GreaterOrEqual(t, firstDialCount, 2)

	// 人工制造缺口触发新一轮预热需求。
	ap, ok := pool.getAccountPool(accountID)
	require.True(t, ok)
	require.NotNil(t, ap)
	ap.mu.Lock()
	for id := range ap.conns {
		delete(ap.conns, id)
		break
	}
	ap.mu.Unlock()

	pool.ensureTargetIdleAsync(accountID)
	time.Sleep(120 * time.Millisecond)
	require.Equal(t, firstDialCount, dialer.DialCount(), "cooldown 窗口内不应再次触发预热")

	time.Sleep(450 * time.Millisecond)
	pool.ensureTargetIdleAsync(accountID)
	require.Eventually(t, func() bool {
		return dialer.DialCount() > firstDialCount
	}, 2*time.Second, 20*time.Millisecond)
}

func TestOpenAIWSConnPool_EnsureTargetIdleAsyncFailureSuppress(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.PoolTargetUtilization = 0.8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 1
	cfg.Gateway.OpenAIWS.PrewarmCooldownMS = 0

	pool := newOpenAIWSConnPool(cfg)
	StoreOpenAIWSPoolRuntimeSettings(50, 120)
	dialer := &openAIWSAlwaysFailDialer{}
	pool.setClientDialerForTest(dialer)

	accountID := int64(279)
	account := &Account{ID: accountID, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 2}
	ap := pool.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	ap.lastNeutralAcquire = &openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}
	ap.mu.Unlock()

	pool.ensureTargetIdleAsync(accountID)
	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(accountID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return !ap.prewarmActive
	}, 2*time.Second, 20*time.Millisecond)

	pool.ensureTargetIdleAsync(accountID)
	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(accountID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return !ap.prewarmActive
	}, 2*time.Second, 20*time.Millisecond)
	require.Equal(t, 2, dialer.DialCount())

	// 连续失败达到阈值后，新的预热触发应被抑制，不再继续拨号。
	pool.ensureTargetIdleAsync(accountID)
	time.Sleep(120 * time.Millisecond)
	require.Equal(t, 2, dialer.DialCount())
}

func TestOpenAIWSConnPool_AcquireQueueWaitMetrics(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 4

	pool := newOpenAIWSConnPool(cfg)
	accountID := int64(99)
	account := &Account{ID: accountID, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}
	conn := newOpenAIWSConnWithProfileAndReuseKey("busy", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, openAIWSConnReuseKeyForAcquire(req))
	require.True(t, conn.tryAcquire()) // 占用连接，触发后续排队

	ap := pool.ensureAccountPoolLocked(accountID)
	ap.mu.Lock()
	ap.conns[conn.id] = conn
	ap.lastNeutralAcquire = cloneOpenAIWSAcquireRequestPtr(&req)
	ap.mu.Unlock()

	go func() {
		time.Sleep(60 * time.Millisecond)
		conn.release()
	}()

	req.PreferredConnID = conn.id
	req.ForcePreferredConn = true
	lease, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.True(t, lease.Reused())
	require.GreaterOrEqual(t, lease.QueueWaitDuration(), 50*time.Millisecond)
	lease.Release()

	metrics := pool.SnapshotMetrics()
	require.GreaterOrEqual(t, metrics.AcquireQueueWaitTotal, int64(1))
	require.Greater(t, metrics.AcquireQueueWaitMsTotal, int64(0))
	require.GreaterOrEqual(t, metrics.ConnPickTotal, int64(1))
}

func TestOpenAIWSConnPool_ForceNewConnSkipsReuse(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2

	pool := newOpenAIWSConnPool(cfg)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	account := &Account{ID: 123, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	lease1, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
	})
	require.NoError(t, err)
	require.NotNil(t, lease1)
	lease1.Release()

	lease2, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account:      account,
		WSURL:        "wss://example.com/v1/responses",
		ForceNewConn: true,
	})
	require.NoError(t, err)
	require.NotNil(t, lease2)
	lease2.Release()

	require.Equal(t, 2, dialer.DialCount(), "ForceNewConn=true 时应跳过空闲连接复用并新建连接")
}

func TestOpenAIWSConnPool_AcquireForcePreferredConnUnavailable(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2

	pool := newOpenAIWSConnPool(cfg)
	account := &Account{ID: 124, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	ap := pool.getOrCreateAccountPool(account.ID)
	otherConn := newOpenAIWSConn("other_conn", account.ID, &openAIWSFakeConn{}, nil)
	ap.mu.Lock()
	ap.conns[otherConn.id] = otherConn
	ap.mu.Unlock()

	_, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account:            account,
		WSURL:              "wss://example.com/v1/responses",
		ForcePreferredConn: true,
	})
	require.ErrorIs(t, err, errOpenAIWSPreferredConnUnavailable)

	_, err = pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account:            account,
		WSURL:              "wss://example.com/v1/responses",
		PreferredConnID:    "missing_conn",
		ForcePreferredConn: true,
	})
	require.ErrorIs(t, err, errOpenAIWSPreferredConnUnavailable)
}

func TestOpenAIWSConnPool_AcquireForcePreferredEvictsExpiredSessionBeforeReuse(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2

	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	StoreOpenAIWSPoolRuntimeSettings(100, 120)

	account := &Account{ID: 724, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 2}
	req := openAIWSAcquireRequest{
		Account:            account,
		WSURL:              "wss://example.com/v1/responses",
		Profile:            openAIWSConnProfileSessionBound,
		PreferredConnID:    "session_expired_preferred",
		ForcePreferredConn: true,
	}
	reuseKey := openAIWSConnReuseKeyForAcquire(req)
	ap := pool.getOrCreateAccountPool(account.ID)
	now := time.Now()
	expired := newOpenAIWSConnWithProfileAndReuseKey("session_expired_preferred", &openAIWSFakeConn{}, nil, openAIWSConnProfileSessionBound, reuseKey)
	expired.lastUsedNano.Store(now.Add(-pool.sessionIdleTTL()).UnixNano())
	ap.mu.Lock()
	ap.lastCleanupAt = now
	ap.conns[expired.id] = expired
	ap.mu.Unlock()

	_, err := pool.Acquire(context.Background(), req)
	require.ErrorIs(t, err, errOpenAIWSPreferredConnUnavailable)

	ap.mu.Lock()
	_, stillPresent := ap.conns[expired.id]
	ap.mu.Unlock()
	require.False(t, stillPresent, "expired session-bound preferred conn should be evicted before write")
}

func TestOpenAIWSConnPool_AcquireForcePreferredConnQueuesOnPreferredOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 4

	pool := newOpenAIWSConnPool(cfg)
	account := &Account{ID: 125, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	ap := pool.getOrCreateAccountPool(account.ID)
	preferredConn := newOpenAIWSConn("preferred_conn", account.ID, &openAIWSFakeConn{}, nil)
	otherConn := newOpenAIWSConn("other_conn_idle", account.ID, &openAIWSFakeConn{}, nil)
	require.True(t, preferredConn.tryAcquire(), "先占用 preferred 连接，触发排队获取")
	ap.mu.Lock()
	ap.conns[preferredConn.id] = preferredConn
	ap.conns[otherConn.id] = otherConn
	ap.lastCleanupAt = time.Now()
	ap.mu.Unlock()

	go func() {
		time.Sleep(60 * time.Millisecond)
		preferredConn.release()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	lease, err := pool.Acquire(ctx, openAIWSAcquireRequest{
		Account:            account,
		WSURL:              "wss://example.com/v1/responses",
		PreferredConnID:    preferredConn.id,
		ForcePreferredConn: true,
	})
	require.NoError(t, err)
	require.NotNil(t, lease)
	require.Equal(t, preferredConn.id, lease.ConnID(), "严格模式应只等待并复用 preferred 连接，不可漂移")
	require.GreaterOrEqual(t, lease.QueueWaitDuration(), 40*time.Millisecond)
	lease.Release()
	require.True(t, otherConn.tryAcquire(), "other 连接不应被严格模式抢占")
	otherConn.release()
}

func TestOpenAIWSConnPool_AcquireForcePreferredConnDirectAndQueueFull(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 1

	pool := newOpenAIWSConnPool(cfg)
	account := &Account{ID: 127, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	ap := pool.getOrCreateAccountPool(account.ID)
	preferredConn := newOpenAIWSConn("preferred_conn_direct", account.ID, &openAIWSFakeConn{}, nil)
	otherConn := newOpenAIWSConn("other_conn_direct", account.ID, &openAIWSFakeConn{}, nil)
	ap.mu.Lock()
	ap.conns[preferredConn.id] = preferredConn
	ap.conns[otherConn.id] = otherConn
	ap.lastCleanupAt = time.Now()
	ap.mu.Unlock()

	lease, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account:            account,
		WSURL:              "wss://example.com/v1/responses",
		PreferredConnID:    preferredConn.id,
		ForcePreferredConn: true,
	})
	require.NoError(t, err)
	require.Equal(t, preferredConn.id, lease.ConnID(), "preferred 空闲时应直接命中")
	lease.Release()

	require.True(t, preferredConn.tryAcquire())
	preferredConn.waiters.Store(1)
	_, err = pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account:            account,
		WSURL:              "wss://example.com/v1/responses",
		PreferredConnID:    preferredConn.id,
		ForcePreferredConn: true,
	})
	require.ErrorIs(t, err, errOpenAIWSConnQueueFull, "严格模式下队列满应直接失败，不得漂移")
	preferredConn.waiters.Store(0)
	preferredConn.release()
}

func TestOpenAIWSConnPool_CleanupSkipsPinnedConn(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 0

	pool := newOpenAIWSConnPool(cfg)
	accountID := int64(126)
	ap := pool.getOrCreateAccountPool(accountID)
	now := time.Now()
	pinnedConn := newOpenAIWSConn("pinned_conn", accountID, &openAIWSFakeConn{}, nil)
	pinnedConn.lastUsedNano.Store(now.Add(-3 * time.Minute).UnixNano())
	idleConn := newOpenAIWSConnWithProfile("idle_conn", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	idleConn.lastUsedNano.Store(now.Add(-3 * time.Minute).UnixNano())
	ap.mu.Lock()
	ap.conns[pinnedConn.id] = pinnedConn
	ap.conns[idleConn.id] = idleConn
	ap.mu.Unlock()

	require.True(t, pool.PinConn(accountID, pinnedConn.id))
	evicted := pool.cleanupAccountLocked(ap, now, 0)
	closeOpenAIWSConns(evicted)

	ap.mu.Lock()
	_, pinnedExists := ap.conns[pinnedConn.id]
	_, idleExists := ap.conns[idleConn.id]
	ap.mu.Unlock()
	require.True(t, pinnedExists, "被 active ingress 绑定的连接不应被 cleanup 回收")
	require.False(t, idleExists, "非绑定的空闲连接应被回收")

	pool.UnpinConn(accountID, pinnedConn.id)
	evicted = pool.cleanupAccountLocked(ap, now, 0)
	closeOpenAIWSConns(evicted)
	ap.mu.Lock()
	_, pinnedExists = ap.conns[pinnedConn.id]
	ap.mu.Unlock()
	require.False(t, pinnedExists, "解绑后连接应可被正常回收")
}

func TestOpenAIWSConnPool_PinUnpinConnBranches(t *testing.T) {
	var nilPool *openAIWSConnPool
	require.False(t, nilPool.PinConn(1, "x"))
	nilPool.UnpinConn(1, "x")

	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)
	accountID := int64(128)
	ap := &openAIWSAccountPool{
		conns: map[string]*openAIWSConn{},
	}
	pool.accounts.Store(accountID, ap)

	require.False(t, pool.PinConn(0, "x"))
	require.False(t, pool.PinConn(999, "x"))
	require.False(t, pool.PinConn(accountID, ""))
	require.False(t, pool.PinConn(accountID, "missing"))

	conn := newOpenAIWSConn("pin_refcount", accountID, &openAIWSFakeConn{}, nil)
	ap.mu.Lock()
	ap.conns[conn.id] = conn
	ap.mu.Unlock()
	require.True(t, pool.PinConn(accountID, conn.id))
	require.True(t, pool.PinConn(accountID, conn.id))

	ap.mu.Lock()
	require.Equal(t, 2, ap.pinnedConns[conn.id])
	ap.mu.Unlock()

	pool.UnpinConn(accountID, conn.id)
	ap.mu.Lock()
	require.Equal(t, 1, ap.pinnedConns[conn.id])
	ap.mu.Unlock()

	pool.UnpinConn(accountID, conn.id)
	ap.mu.Lock()
	_, exists := ap.pinnedConns[conn.id]
	ap.mu.Unlock()
	require.False(t, exists)

	pool.UnpinConn(accountID, conn.id)
	pool.UnpinConn(accountID, "")
	pool.UnpinConn(0, conn.id)
	pool.UnpinConn(999, conn.id)
}

func TestOpenAIWSConnPool_EffectiveMaxConnsByAccount(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.DynamicMaxConnsByAccountConcurrencyEnabled = true
	cfg.Gateway.OpenAIWS.OAuthMaxConnsFactor = 1.0
	cfg.Gateway.OpenAIWS.APIKeyMaxConnsFactor = 0.6

	pool := newOpenAIWSConnPool(cfg)

	oauthHigh := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 10}
	require.Equal(t, 8, pool.effectiveMaxConnsByAccount(oauthHigh), "应受全局硬上限约束")

	oauthLow := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 3}
	require.Equal(t, 3, pool.effectiveMaxConnsByAccount(oauthLow))

	apiKeyHigh := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 10}
	require.Equal(t, 6, pool.effectiveMaxConnsByAccount(apiKeyHigh), "API Key 应按系数缩放")

	apiKeyLow := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}
	require.Equal(t, 1, pool.effectiveMaxConnsByAccount(apiKeyLow), "最小值应保持为 1")

	unlimited := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 0}
	require.Equal(t, 8, pool.effectiveMaxConnsByAccount(unlimited), "无限并发应回退到全局硬上限")

	require.Equal(t, 8, pool.effectiveMaxConnsByAccount(nil), "缺少账号上下文应回退到全局硬上限")
}

func TestOpenAIWSConnPool_EffectiveMaxConnsDisabledFallbackHardCap(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.DynamicMaxConnsByAccountConcurrencyEnabled = false
	cfg.Gateway.OpenAIWS.OAuthMaxConnsFactor = 1.0
	cfg.Gateway.OpenAIWS.APIKeyMaxConnsFactor = 1.0

	pool := newOpenAIWSConnPool(cfg)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 2}
	require.Equal(t, 8, pool.effectiveMaxConnsByAccount(account), "关闭动态模式后应保持旧行为")
}

func TestOpenAIWSConnPool_EffectiveMaxConnsByAccount_ModeRouterV2UsesAccountConcurrency(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.DynamicMaxConnsByAccountConcurrencyEnabled = true
	cfg.Gateway.OpenAIWS.OAuthMaxConnsFactor = 0.3
	cfg.Gateway.OpenAIWS.APIKeyMaxConnsFactor = 0.6

	pool := newOpenAIWSConnPool(cfg)

	high := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 20}
	require.Equal(t, 20, pool.effectiveMaxConnsByAccount(high), "v2 路径应直接使用账号并发数作为池上限")

	nonPositive := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 0}
	require.Equal(t, 0, pool.effectiveMaxConnsByAccount(nonPositive), "并发数<=0 时应不可调度")
}

func TestOpenAIWSConnPool_AcquireDoesNotRejectWhenEffectiveMaxConnsIsZero(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	account := &Account{ID: 901, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 0}
	lease, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
	})
	require.NoError(t, err)
	require.NotNil(t, lease)
	lease.Release()
	require.Equal(t, 1, dialer.DialCount())
}

func TestOpenAIWSConnLease_ReadMessageWithContextTimeout_PerRead(t *testing.T) {
	conn := newOpenAIWSConn("timeout", 1, &openAIWSBlockingConn{readDelay: 80 * time.Millisecond}, nil)
	lease := &openAIWSConnLease{conn: conn}

	_, err := lease.ReadMessageWithContextTimeout(context.Background(), 20*time.Millisecond)
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	payload, err := lease.ReadMessageWithContextTimeout(context.Background(), 150*time.Millisecond)
	require.NoError(t, err)
	require.Contains(t, string(payload), "response.completed")

	parentCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = lease.ReadMessageWithContextTimeout(parentCtx, 150*time.Millisecond)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestOpenAIWSConnLease_WriteJSONWithContextTimeout_RespectsParentContext(t *testing.T) {
	conn := newOpenAIWSConn("write_timeout_ctx", 1, &openAIWSWriteBlockingConn{}, nil)
	lease := &openAIWSConnLease{conn: conn}

	parentCtx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := lease.WriteJSONWithContextTimeout(parentCtx, map[string]any{"type": "response.create"}, 2*time.Minute)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, elapsed, 200*time.Millisecond)
}

func TestOpenAIWSConnLease_PingWithTimeout(t *testing.T) {
	conn := newOpenAIWSConn("ping_ok", 1, &openAIWSFakeConn{}, nil)
	lease := &openAIWSConnLease{conn: conn}
	require.NoError(t, lease.PingWithTimeout(50*time.Millisecond))

	var nilLease *openAIWSConnLease
	err := nilLease.PingWithTimeout(50 * time.Millisecond)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
}

func TestOpenAIWSConn_ReadAndWriteCanProceedConcurrently(t *testing.T) {
	conn := newOpenAIWSConn("full_duplex", 1, &openAIWSBlockingConn{readDelay: 120 * time.Millisecond}, nil)

	readDone := make(chan error, 1)
	go func() {
		_, err := conn.readMessageWithContextTimeout(context.Background(), 200*time.Millisecond)
		readDone <- err
	}()

	// 让读取先占用 readMu。
	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	err := conn.pingWithTimeout(50 * time.Millisecond)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Less(t, elapsed, 80*time.Millisecond, "写路径不应被读锁长期阻塞")
	require.NoError(t, <-readDone)
}

func TestOpenAIWSConnPool_BackgroundCleanupSweep_WithoutAcquire(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	pool := newOpenAIWSConnPool(cfg)

	accountID := int64(302)
	ap := pool.getOrCreateAccountPool(accountID)
	stale := newOpenAIWSConn("stale_bg", accountID, &openAIWSFakeConn{}, nil)
	stale.createdAtNano.Store(time.Now().Add(-2 * time.Hour).UnixNano())
	stale.lastUsedNano.Store(time.Now().Add(-2 * time.Hour).UnixNano())
	ap.mu.Lock()
	ap.conns[stale.id] = stale
	ap.mu.Unlock()

	pool.runBackgroundCleanupSweep(time.Now())

	ap.mu.Lock()
	_, exists := ap.conns[stale.id]
	ap.mu.Unlock()
	require.False(t, exists, "后台清理应在无新 acquire 时也回收过期连接")
}

func TestOpenAIWSConnPool_BackgroundCleanupWorkerGuardBranches(t *testing.T) {
	var nilPool *openAIWSConnPool
	require.NotPanics(t, func() {
		nilPool.startBackgroundWorkers()
		nilPool.runBackgroundCleanupWorker()
		nilPool.runBackgroundCleanupSweep(time.Now())
	})

	poolNoStop := &openAIWSConnPool{}
	require.NotPanics(t, func() {
		poolNoStop.startBackgroundWorkers()
	})

	poolStopCleanup := &openAIWSConnPool{workerStopCh: make(chan struct{})}
	cleanupDone := make(chan struct{})
	go func() {
		poolStopCleanup.runBackgroundCleanupWorker()
		close(cleanupDone)
	}()
	close(poolStopCleanup.workerStopCh)
	select {
	case <-cleanupDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runBackgroundCleanupWorker 未在 stop 信号后退出")
	}
}

func TestOpenAIWSConnPool_RunBackgroundCleanupSweep_SkipsInvalidAndUsesAccountCap(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.DynamicMaxConnsByAccountConcurrencyEnabled = true

	pool := &openAIWSConnPool{cfg: cfg}
	pool.accounts.Store("bad-key", "bad-value")

	accountID := int64(2026)
	ap := &openAIWSAccountPool{
		conns: make(map[string]*openAIWSConn),
	}
	ap.conns["nil_conn"] = nil
	stale := newOpenAIWSConn("stale_bg_cleanup", accountID, &openAIWSFakeConn{}, nil)
	stale.createdAtNano.Store(time.Now().Add(-2 * time.Hour).UnixNano())
	stale.lastUsedNano.Store(time.Now().Add(-2 * time.Hour).UnixNano())
	ap.conns[stale.id] = stale
	ap.lastAcquire = &openAIWSAcquireRequest{
		Account: &Account{
			ID:          accountID,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Concurrency: 1,
		},
	}
	pool.accounts.Store(accountID, ap)

	now := time.Now()
	require.NotPanics(t, func() {
		pool.runBackgroundCleanupSweep(now)
	})

	ap.mu.Lock()
	_, nilConnExists := ap.conns["nil_conn"]
	_, exists := ap.conns[stale.id]
	lastCleanupAt := ap.lastCleanupAt
	ap.mu.Unlock()

	require.False(t, nilConnExists, "后台清理应移除无效 nil 连接条目")
	require.False(t, exists, "后台清理应清理过期连接")
	require.Equal(t, now, lastCleanupAt)
}

func TestOpenAIWSConnPool_RunBackgroundCleanupSweep_UsesNeutralSnapshotForNeutralShrink(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)
	StoreOpenAIWSPoolRuntimeSettings(50, 120)

	pool := newOpenAIWSConnPool(&config.Config{})
	t.Cleanup(pool.Close)

	accountID := int64(2027)
	ap := pool.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	for i := 0; i < 4; i++ {
		conn := newOpenAIWSConnWithProfile(
			fmt.Sprintf("neutral_snapshot_shrink_%d", i),
			&openAIWSFakeConn{},
			nil,
			openAIWSConnProfileNeutral,
		)
		conn.lastUsedNano.Store(time.Now().Add(-time.Duration(10+i) * time.Second).UnixNano())
		conn.markNeutralStock()
		ap.conns[conn.id] = conn
	}
	ap.lastAcquire = &openAIWSAcquireRequest{
		Account: &Account{
			ID:          accountID,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Concurrency: 8,
		},
		Profile: openAIWSConnProfileSessionBound,
	}
	ap.lastNeutralAcquire = &openAIWSAcquireRequest{
		Account: &Account{
			ID:          accountID,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Concurrency: 2,
		},
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}
	ap.mu.Unlock()

	pool.runBackgroundCleanupSweep(time.Now())

	ap.mu.Lock()
	neutralCount := countConnsByProfileLocked(ap, openAIWSConnProfileNeutral)
	ap.mu.Unlock()
	require.Equal(t, 1, neutralCount, "neutral shrink target must use the latest neutral account snapshot, not stale session concurrency")
}

func TestOpenAIWSConnPool_RunBackgroundCleanupSweep_UsesStickyAdjustedNeutralTarget(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(50, 120, 0, 10, 30)

	pool := newOpenAIWSConnPool(&config.Config{})
	t.Cleanup(pool.Close)

	accountID := int64(2028)
	ap := pool.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	for i := 0; i < 4; i++ {
		conn := newOpenAIWSConnWithProfile(
			fmt.Sprintf("neutral_sticky_shrink_%d", i),
			&openAIWSFakeConn{},
			nil,
			openAIWSConnProfileNeutral,
		)
		conn.lastUsedNano.Store(time.Now().Add(-time.Duration(10+i) * time.Second).UnixNano())
		conn.markNeutralStock()
		ap.conns[conn.id] = conn
	}
	ap.lastNeutralAcquire = &openAIWSAcquireRequest{
		Account: &Account{
			ID:          accountID,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Concurrency: 10,
		},
		WSURL:   "wss://example.invalid/ws",
		Profile: openAIWSConnProfileNeutral,
	}
	ap.mu.Unlock()

	pool.runBackgroundCleanupSweep(time.Now())

	ap.mu.Lock()
	neutralCount := countConnsByProfileLocked(ap, openAIWSConnProfileNeutral)
	ap.mu.Unlock()
	require.Equal(t, 3, neutralCount, "neutral shrink target should follow the sticky-adjusted fresh-session capacity")
}

func TestOpenAIWSConnPool_QueueLimitPerConn_DefaultAndConfigured(t *testing.T) {
	var nilPool *openAIWSConnPool
	require.Equal(t, 256, nilPool.queueLimitPerConn())

	pool := &openAIWSConnPool{cfg: &config.Config{}}
	require.Equal(t, 256, pool.queueLimitPerConn())

	pool.cfg.Gateway.OpenAIWS.QueueLimitPerConn = 9
	require.Equal(t, 9, pool.queueLimitPerConn())
}

func TestOpenAIWSConnPool_Close(t *testing.T) {
	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)

	// Close 应该可以安全调用
	pool.Close()

	// workerStopCh 应已关闭
	select {
	case <-pool.workerStopCh:
		// 预期：channel 已关闭
	default:
		t.Fatal("Close 后 workerStopCh 应已关闭")
	}

	// 多次调用 Close 不应 panic
	pool.Close()

	// nil pool 调用 Close 不应 panic
	var nilPool *openAIWSConnPool
	nilPool.Close()
}

func TestOpenAIWSDialError_ErrorAndUnwrap(t *testing.T) {
	baseErr := errors.New("boom")
	dialErr := &openAIWSDialError{StatusCode: 502, Err: baseErr}
	require.Contains(t, dialErr.Error(), "status=502")
	require.ErrorIs(t, dialErr.Unwrap(), baseErr)

	noStatus := &openAIWSDialError{Err: baseErr}
	require.Contains(t, noStatus.Error(), "boom")

	var nilDialErr *openAIWSDialError
	require.Equal(t, "", nilDialErr.Error())
	require.NoError(t, nilDialErr.Unwrap())
}

func TestOpenAIWSConnLease_ReadWriteHelpersAndConnStats(t *testing.T) {
	conn := newOpenAIWSConn("helper_conn", 1, &openAIWSFakeConn{}, http.Header{
		"X-Test": []string{" value "},
	})
	lease := &openAIWSConnLease{conn: conn}

	require.NoError(t, lease.WriteJSONContext(context.Background(), map[string]any{"type": "response.create"}))
	payload, err := lease.ReadMessage(100 * time.Millisecond)
	require.NoError(t, err)
	require.Contains(t, string(payload), "response.completed")

	payload, err = lease.ReadMessageContext(context.Background())
	require.NoError(t, err)
	require.Contains(t, string(payload), "response.completed")

	payload, err = conn.readMessageWithTimeout(100 * time.Millisecond)
	require.NoError(t, err)
	require.Contains(t, string(payload), "response.completed")

	require.Equal(t, "value", conn.handshakeHeader(" X-Test "))
	require.NotZero(t, conn.createdAt())
	require.NotZero(t, conn.lastUsedAt())
	require.GreaterOrEqual(t, conn.age(time.Now()), time.Duration(0))
	require.GreaterOrEqual(t, conn.idleDuration(time.Now()), time.Duration(0))
	require.False(t, conn.isLeased())

	// 覆盖空上下文路径
	_, err = conn.readMessage(context.Background())
	require.NoError(t, err)

	// 覆盖 nil 保护分支
	var nilConn *openAIWSConn
	require.ErrorIs(t, nilConn.writeJSONWithTimeout(context.Background(), map[string]any{}, time.Second), errOpenAIWSConnClosed)
	_, err = nilConn.readMessageWithTimeout(10 * time.Millisecond)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
	_, err = nilConn.readMessageWithContextTimeout(context.Background(), 10*time.Millisecond)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
}

func TestOpenAIWSConnPool_PickOldestIdleAndAccountPoolLoad(t *testing.T) {
	pool := &openAIWSConnPool{}
	accountID := int64(404)
	ap := &openAIWSAccountPool{conns: map[string]*openAIWSConn{}}

	idleOld := newOpenAIWSConn("idle_old", accountID, &openAIWSFakeConn{}, nil)
	idleOld.lastUsedNano.Store(time.Now().Add(-10 * time.Minute).UnixNano())
	idleNew := newOpenAIWSConn("idle_new", accountID, &openAIWSFakeConn{}, nil)
	idleNew.lastUsedNano.Store(time.Now().Add(-1 * time.Minute).UnixNano())
	leased := newOpenAIWSConn("leased", accountID, &openAIWSFakeConn{}, nil)
	require.True(t, leased.tryAcquire())
	leased.waiters.Store(2)

	ap.conns[idleOld.id] = idleOld
	ap.conns[idleNew.id] = idleNew
	ap.conns[leased.id] = leased

	oldest := pool.pickOldestIdleConnLocked(ap)
	require.NotNil(t, oldest)
	require.Equal(t, idleOld.id, oldest.id)

	inflight, waiters := accountPoolLoadLocked(ap)
	require.Equal(t, 1, inflight)
	require.Equal(t, 2, waiters)

	pool.accounts.Store(accountID, ap)
	loadInflight, loadWaiters, conns := pool.AccountPoolLoad(accountID)
	require.Equal(t, 1, loadInflight)
	require.Equal(t, 2, loadWaiters)
	require.Equal(t, 3, conns)

	zeroInflight, zeroWaiters, zeroConns := pool.AccountPoolLoad(0)
	require.Equal(t, 0, zeroInflight)
	require.Equal(t, 0, zeroWaiters)
	require.Equal(t, 0, zeroConns)
}

func TestOpenAIWSConnPool_ConnSnapshotReportsReusableConnState(t *testing.T) {
	pool := &openAIWSConnPool{}
	accountID := int64(405)
	ap := &openAIWSAccountPool{conns: map[string]*openAIWSConn{}}
	conn := newOpenAIWSConnWithProfile("snap_conn", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	conn.createdAtNano.Store(time.Now().Add(-3 * time.Minute).UnixNano())
	conn.lastUsedNano.Store(time.Now().Add(-2 * time.Minute).UnixNano())
	conn.leaseCount.Store(3)
	conn.waiters.Store(2)
	require.True(t, conn.tryAcquire())
	ap.conns[conn.id] = conn
	pool.accounts.Store(accountID, ap)

	snapshot := pool.ConnSnapshot(accountID, conn.id)
	require.True(t, snapshot.Exists)
	require.Equal(t, openAIWSConnProfileNeutral, snapshot.Profile)
	require.Greater(t, snapshot.Age, time.Duration(0))
	require.GreaterOrEqual(t, snapshot.Idle, 2*time.Minute)
	require.EqualValues(t, 3, snapshot.LeaseCount)
	require.True(t, snapshot.Leased)
	require.EqualValues(t, 2, snapshot.Waiters)

	missing := pool.ConnSnapshot(accountID, "missing")
	require.False(t, missing.Exists)
}

func TestOpenAIWSConnPool_Close_WaitsWorkerGroupAndNilStopChannel(t *testing.T) {
	pool := &openAIWSConnPool{}
	release := make(chan struct{})
	pool.workerWg.Add(1)
	go func() {
		defer pool.workerWg.Done()
		<-release
	}()

	closed := make(chan struct{})
	go func() {
		pool.Close()
		close(closed)
	}()

	select {
	case <-closed:
		t.Fatal("Close 不应在 WaitGroup 未完成时提前返回")
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close 未等待 workerWg 完成")
	}
}

func TestOpenAIWSConnPool_Close_ClosesOnlyIdleConnections(t *testing.T) {
	pool := &openAIWSConnPool{
		workerStopCh: make(chan struct{}),
	}

	accountID := int64(606)
	ap := &openAIWSAccountPool{
		conns: map[string]*openAIWSConn{},
	}
	idle := newOpenAIWSConn("idle_conn", accountID, &openAIWSFakeConn{}, nil)
	leased := newOpenAIWSConn("leased_conn", accountID, &openAIWSFakeConn{}, nil)
	require.True(t, leased.tryAcquire())

	ap.conns[idle.id] = idle
	ap.conns[leased.id] = leased
	pool.accounts.Store(accountID, ap)
	pool.accounts.Store("invalid-key", "invalid-value")

	pool.Close()

	select {
	case <-idle.closedCh:
		// idle should be closed
	default:
		t.Fatal("空闲连接应在 Close 时被关闭")
	}

	select {
	case <-leased.closedCh:
		t.Fatal("已租赁连接不应在 Close 时被关闭")
	default:
	}

	leased.release()
	pool.Close()
}

func TestOpenAIWSConnLease_BasicGetterBranches(t *testing.T) {
	var nilLease *openAIWSConnLease
	require.Equal(t, "", nilLease.ConnID())
	require.Equal(t, time.Duration(0), nilLease.QueueWaitDuration())
	require.Equal(t, time.Duration(0), nilLease.ConnPickDuration())
	require.False(t, nilLease.Reused())
	require.Equal(t, "", nilLease.HandshakeHeader("x-test"))
	require.False(t, nilLease.IsPrewarmed())
	nilLease.MarkPrewarmed()
	nilLease.Release()

	conn := newOpenAIWSConn("getter_conn", 1, &openAIWSFakeConn{}, http.Header{"X-Test": []string{"ok"}})
	lease := &openAIWSConnLease{
		conn:      conn,
		queueWait: 3 * time.Millisecond,
		connPick:  4 * time.Millisecond,
		reused:    true,
	}
	require.Equal(t, "getter_conn", lease.ConnID())
	require.Equal(t, 3*time.Millisecond, lease.QueueWaitDuration())
	require.Equal(t, 4*time.Millisecond, lease.ConnPickDuration())
	require.True(t, lease.Reused())
	require.Equal(t, "ok", lease.HandshakeHeader("x-test"))
	require.False(t, lease.IsPrewarmed())
	lease.MarkPrewarmed()
	require.True(t, lease.IsPrewarmed())
	lease.Release()
}

func TestOpenAIWSConnPool_UtilityBranches(t *testing.T) {
	var nilPool *openAIWSConnPool
	require.Equal(t, OpenAIWSPoolMetricsSnapshot{}, nilPool.SnapshotMetrics())
	require.Equal(t, OpenAIWSTransportMetricsSnapshot{}, nilPool.SnapshotTransportMetrics())

	pool := &openAIWSConnPool{cfg: &config.Config{}}
	pool.metrics.acquireTotal.Store(7)
	pool.metrics.acquireReuseTotal.Store(3)
	metrics := pool.SnapshotMetrics()
	require.Equal(t, int64(7), metrics.AcquireTotal)
	require.Equal(t, int64(3), metrics.AcquireReuseTotal)

	// 非 transport metrics dialer 路径
	pool.clientDialer = &openAIWSFakeDialer{}
	require.Equal(t, OpenAIWSTransportMetricsSnapshot{}, pool.SnapshotTransportMetrics())
	pool.setClientDialerForTest(nil)
	require.NotNil(t, pool.clientDialer)

	require.Equal(t, 8, nilPool.maxConnsHardCap())
	require.False(t, nilPool.dynamicMaxConnsEnabled())
	require.Equal(t, 1.0, nilPool.maxConnsFactorByAccount(nil))
	require.Equal(t, 0, nilPool.minIdlePerAccount())
	require.Equal(t, 4, nilPool.maxIdlePerAccount())
	require.Equal(t, 256, nilPool.queueLimitPerConn())
	require.Equal(t, 0.7, nilPool.targetUtilization())
	require.Equal(t, time.Duration(0), nilPool.prewarmCooldown())
	require.Equal(t, 10*time.Second, nilPool.dialTimeout())

	// shouldSuppressPrewarmLocked 覆盖 3 条分支
	now := time.Now()
	apNilFail := &openAIWSAccountPool{prewarmFails: 1}
	require.False(t, pool.shouldSuppressPrewarmLocked(apNilFail, now))
	apZeroTime := &openAIWSAccountPool{prewarmFails: 2}
	require.False(t, pool.shouldSuppressPrewarmLocked(apZeroTime, now))
	require.Equal(t, 0, apZeroTime.prewarmFails)
	apOldFail := &openAIWSAccountPool{prewarmFails: 2, prewarmFailAt: now.Add(-openAIWSPrewarmFailureWindow - time.Second)}
	require.False(t, pool.shouldSuppressPrewarmLocked(apOldFail, now))
	apRecentFail := &openAIWSAccountPool{prewarmFails: openAIWSPrewarmFailureSuppress, prewarmFailAt: now}
	require.True(t, pool.shouldSuppressPrewarmLocked(apRecentFail, now))

	// recordConnPickDuration 的保护分支
	nilPool.recordConnPickDuration(10 * time.Millisecond)
	pool.recordConnPickDuration(-10 * time.Millisecond)
	require.Equal(t, int64(1), pool.metrics.connPickTotal.Load())

	// account pool 读写分支
	require.Nil(t, nilPool.getOrCreateAccountPool(1))
	require.Nil(t, pool.getOrCreateAccountPool(0))
	pool.accounts.Store(int64(7), "invalid")
	ap := pool.getOrCreateAccountPool(7)
	require.NotNil(t, ap)
	_, ok := pool.getAccountPool(0)
	require.False(t, ok)
	_, ok = pool.getAccountPool(12345)
	require.False(t, ok)
	pool.accounts.Store(int64(8), "bad-type")
	_, ok = pool.getAccountPool(8)
	require.False(t, ok)
}

func TestOpenAIWSConn_LeaseAndTimeHelpers_NilAndClosedBranches(t *testing.T) {
	var nilConn *openAIWSConn
	nilConn.touch()
	require.Equal(t, time.Time{}, nilConn.createdAt())
	require.Equal(t, time.Time{}, nilConn.lastUsedAt())
	require.Equal(t, time.Duration(0), nilConn.idleDuration(time.Now()))
	require.Equal(t, time.Duration(0), nilConn.age(time.Now()))
	require.False(t, nilConn.isLeased())
	require.False(t, nilConn.isPrewarmed())
	nilConn.markPrewarmed()

	conn := newOpenAIWSConn("lease_state", 1, &openAIWSFakeConn{}, nil)
	require.True(t, conn.tryAcquire())
	require.True(t, conn.isLeased())
	conn.release()
	require.False(t, conn.isLeased())
	conn.close()
	require.False(t, conn.tryAcquire())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := conn.acquire(ctx)
	require.Error(t, err)
}

func TestOpenAIWSConnLease_ReadWriteNilConnBranches(t *testing.T) {
	lease := &openAIWSConnLease{}
	require.ErrorIs(t, lease.WriteJSON(map[string]any{"k": "v"}, time.Second), errOpenAIWSConnClosed)
	require.ErrorIs(t, lease.WriteJSONContext(context.Background(), map[string]any{"k": "v"}), errOpenAIWSConnClosed)
	_, err := lease.ReadMessage(10 * time.Millisecond)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
	_, err = lease.ReadMessageContext(context.Background())
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
	_, err = lease.ReadMessageWithContextTimeout(context.Background(), 10*time.Millisecond)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
}

func TestOpenAIWSConnLease_ReleasedLeaseGuards(t *testing.T) {
	conn := newOpenAIWSConn("released_guard", 1, &openAIWSFakeConn{}, nil)
	lease := &openAIWSConnLease{conn: conn}

	require.NoError(t, lease.PingWithTimeout(50*time.Millisecond))

	lease.Release()
	lease.Release() // idempotent

	require.ErrorIs(t, lease.WriteJSON(map[string]any{"k": "v"}, time.Second), errOpenAIWSConnClosed)
	require.ErrorIs(t, lease.WriteJSONContext(context.Background(), map[string]any{"k": "v"}), errOpenAIWSConnClosed)
	require.ErrorIs(t, lease.WriteJSONWithContextTimeout(context.Background(), map[string]any{"k": "v"}, time.Second), errOpenAIWSConnClosed)

	_, err := lease.ReadMessage(10 * time.Millisecond)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
	_, err = lease.ReadMessageContext(context.Background())
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
	_, err = lease.ReadMessageWithContextTimeout(context.Background(), 10*time.Millisecond)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)

	require.ErrorIs(t, lease.PingWithTimeout(50*time.Millisecond), errOpenAIWSConnClosed)
}

func TestOpenAIWSConnLease_MarkBrokenAfterRelease_NoEviction(t *testing.T) {
	conn := newOpenAIWSConn("released_markbroken", 7, &openAIWSFakeConn{}, nil)
	ap := &openAIWSAccountPool{
		conns: map[string]*openAIWSConn{
			conn.id: conn,
		},
	}
	pool := &openAIWSConnPool{}
	pool.accounts.Store(int64(7), ap)

	lease := &openAIWSConnLease{
		pool:      pool,
		accountID: 7,
		conn:      conn,
	}

	lease.Release()
	lease.MarkBroken()

	ap.mu.Lock()
	_, exists := ap.conns[conn.id]
	ap.mu.Unlock()
	require.True(t, exists, "released lease should not evict active pool connection")
}

func TestOpenAIWSConn_AdditionalGuardBranches(t *testing.T) {
	var nilConn *openAIWSConn
	require.False(t, nilConn.tryAcquire())
	require.ErrorIs(t, nilConn.acquire(context.Background()), errOpenAIWSConnClosed)
	nilConn.release()
	nilConn.close()
	require.Equal(t, "", nilConn.handshakeHeader("x-test"))

	connBusy := newOpenAIWSConn("busy_ctx", 1, &openAIWSFakeConn{}, nil)
	require.True(t, connBusy.tryAcquire())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, connBusy.acquire(ctx), context.Canceled)
	connBusy.release()

	connClosed := newOpenAIWSConn("closed_guard", 1, &openAIWSFakeConn{}, nil)
	connClosed.close()
	require.ErrorIs(
		t,
		connClosed.writeJSONWithTimeout(context.Background(), map[string]any{"k": "v"}, time.Second),
		errOpenAIWSConnClosed,
	)
	_, err := connClosed.readMessageWithContextTimeout(context.Background(), time.Second)
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
	require.ErrorIs(t, connClosed.pingWithTimeout(time.Second), errOpenAIWSConnClosed)

	connNoWS := newOpenAIWSConn("no_ws", 1, nil, nil)
	require.ErrorIs(t, connNoWS.writeJSON(map[string]any{"k": "v"}, context.Background()), errOpenAIWSConnClosed)
	_, err = connNoWS.readMessage(context.Background())
	require.ErrorIs(t, err, errOpenAIWSConnClosed)
	require.ErrorIs(t, connNoWS.pingWithTimeout(time.Second), errOpenAIWSConnClosed)
	require.Equal(t, "", connNoWS.handshakeHeader("x-test"))

	connOK := newOpenAIWSConn("ok", 1, &openAIWSFakeConn{}, nil)
	require.NoError(t, connOK.writeJSON(map[string]any{"k": "v"}, nil))
	_, err = connOK.readMessageWithContextTimeout(context.Background(), 0)
	require.NoError(t, err)
	require.NoError(t, connOK.pingWithTimeout(0))

	connZero := newOpenAIWSConn("zero_ts", 1, &openAIWSFakeConn{}, nil)
	connZero.createdAtNano.Store(0)
	connZero.lastUsedNano.Store(0)
	require.True(t, connZero.createdAt().IsZero())
	require.True(t, connZero.lastUsedAt().IsZero())
	require.Equal(t, time.Duration(0), connZero.idleDuration(time.Now()))
	require.Equal(t, time.Duration(0), connZero.age(time.Now()))

	require.Nil(t, cloneOpenAIWSAcquireRequestPtr(nil))
	copied := cloneHeader(http.Header{
		"X-Empty": []string{},
		"X-Test":  []string{"v1"},
	})
	require.Contains(t, copied, "X-Empty")
	require.Nil(t, copied["X-Empty"])
	require.Equal(t, "v1", copied.Get("X-Test"))

	closeOpenAIWSConns([]*openAIWSConn{nil, connOK})
}

func TestOpenAIWSConnLease_MarkBrokenEvictsConn(t *testing.T) {
	pool := newOpenAIWSConnPool(&config.Config{})
	accountID := int64(5001)
	conn := newOpenAIWSConn("broken_me", accountID, &openAIWSFakeConn{}, nil)
	ap := pool.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	ap.conns[conn.id] = conn
	ap.mu.Unlock()

	lease := &openAIWSConnLease{
		pool:      pool,
		accountID: accountID,
		conn:      conn,
	}
	lease.MarkBroken()

	ap.mu.Lock()
	_, exists := ap.conns[conn.id]
	ap.mu.Unlock()
	require.False(t, exists)
	require.False(t, conn.tryAcquire(), "被标记为 broken 的连接应被关闭")
}

func TestOpenAIWSConnLease_MarkBrokenForPropagatesEvictReason(t *testing.T) {
	pool := newOpenAIWSConnPool(&config.Config{})
	accountID := int64(5002)
	conn := newOpenAIWSConn("broken_for_reason", accountID, &openAIWSFakeConn{}, nil)
	ap := pool.getOrCreateAccountPool(accountID)
	ap.mu.Lock()
	ap.conns[conn.id] = conn
	ap.mu.Unlock()

	var gotConnID string
	var gotReason string
	RegisterOpenAIWSConnEvictHook(func(connID string, reason string) {
		gotConnID = connID
		gotReason = reason
	})
	t.Cleanup(func() { RegisterOpenAIWSConnEvictHook(nil) })

	lease := &openAIWSConnLease{
		pool:      pool,
		accountID: accountID,
		conn:      conn,
	}
	lease.MarkBrokenFor("read_fail")

	require.Equal(t, conn.id, gotConnID)
	require.Equal(t, "read_fail", gotReason)
}

func TestOpenAIWSConnPool_TargetConnCountAndPrewarmBranches(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	pool := newOpenAIWSConnPool(cfg)

	require.Equal(t, 0, pool.targetConnCountLocked(nil, 1))
	ap := &openAIWSAccountPool{conns: map[string]*openAIWSConn{}}
	require.Equal(t, 0, pool.targetConnCountLocked(ap, 0))

	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 3
	require.Equal(t, 1, pool.targetConnCountLocked(ap, 1), "minIdle 应被 maxConns 截断")

	// 覆盖 waiters>0 且 target 需要至少 len(conns)+1 的分支
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.PoolTargetUtilization = 0.9
	busy := newOpenAIWSConn("busy_target", 2, &openAIWSFakeConn{}, nil)
	require.True(t, busy.tryAcquire())
	busy.waiters.Store(1)
	ap.conns[busy.id] = busy
	target := pool.targetConnCountLocked(ap, 4)
	require.GreaterOrEqual(t, target, len(ap.conns)+1)

	// prewarm: account pool 缺失时，拨号后的连接应被关闭并提前返回
	req := openAIWSAcquireRequest{
		Account: &Account{ID: 999, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		WSURL:   "wss://example.com/v1/responses",
	}
	pool.prewarmConns(999, req, 1)

	// prewarm: 拨号失败分支（prewarmFails 累加）
	accountID := int64(1000)
	failPool := newOpenAIWSConnPool(cfg)
	failPool.setClientDialerForTest(&openAIWSAlwaysFailDialer{})
	apFail := failPool.getOrCreateAccountPool(accountID)
	apFail.mu.Lock()
	apFail.creating = 1
	apFail.mu.Unlock()
	req.Account.ID = accountID
	failPool.prewarmConns(accountID, req, 1)
	apFail.mu.Lock()
	require.GreaterOrEqual(t, apFail.prewarmFails, 1)
	apFail.mu.Unlock()
}

func TestOpenAIWSConnPool_Acquire_ErrorBranches(t *testing.T) {
	var nilPool *openAIWSConnPool
	_, err := nilPool.Acquire(context.Background(), openAIWSAcquireRequest{})
	require.Error(t, err)

	pool := newOpenAIWSConnPool(&config.Config{})
	_, err = pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: &Account{ID: 1},
		WSURL:   "   ",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ws url is empty")

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 1
	fullPool := newOpenAIWSConnPool(cfg)

	// queue full 分支：waiters 达上限
	account2 := &Account{ID: 2002, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	ap2 := fullPool.getOrCreateAccountPool(account2.ID)
	req2 := openAIWSAcquireRequest{
		Account: account2,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}
	conn := newOpenAIWSConnWithProfileAndReuseKey("queue_full", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, openAIWSConnReuseKeyForAcquire(req2))
	require.True(t, conn.tryAcquire())
	conn.waiters.Store(1)
	ap2.mu.Lock()
	ap2.conns[conn.id] = conn
	ap2.lastCleanupAt = time.Now()
	ap2.mu.Unlock()
	req2.PreferredConnID = conn.id
	req2.ForcePreferredConn = true
	_, err = fullPool.Acquire(context.Background(), req2)
	require.ErrorIs(t, err, errOpenAIWSConnQueueFull)
}

type openAIWSFakeDialer struct{}

func (d *openAIWSFakeDialer) Dial(
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
	return &openAIWSFakeConn{}, 0, nil, nil
}

type openAIWSCountingDialer struct {
	mu        sync.Mutex
	dialCount int
}

type openAIWSBlockingDialer struct {
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
	dialCount atomic.Int32
}

type openAIWSAlwaysFailDialer struct {
	mu        sync.Mutex
	dialCount int
}

func (d *openAIWSCountingDialer) Dial(
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
	d.mu.Lock()
	d.dialCount++
	d.mu.Unlock()
	return &openAIWSFakeConn{}, 0, nil, nil
}

func (d *openAIWSCountingDialer) DialCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dialCount
}

func (d *openAIWSBlockingDialer) Dial(
	ctx context.Context,
	wsURL string,
	headers http.Header,
	proxyURL string,
	tlsProfile *tlsfingerprint.Profile,
) (openAIWSClientConn, int, http.Header, error) {
	_ = wsURL
	_ = headers
	_ = proxyURL
	_ = tlsProfile
	d.dialCount.Add(1)
	d.startOnce.Do(func() { close(d.started) })
	select {
	case <-ctx.Done():
		return nil, 0, nil, ctx.Err()
	case <-d.release:
		return &openAIWSFakeConn{}, 0, nil, nil
	}
}

func (d *openAIWSBlockingDialer) DialCount() int {
	return int(d.dialCount.Load())
}

func (d *openAIWSAlwaysFailDialer) Dial(
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
	d.mu.Lock()
	d.dialCount++
	d.mu.Unlock()
	return nil, 503, nil, errors.New("dial failed")
}

func (d *openAIWSAlwaysFailDialer) DialCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dialCount
}

type openAIWSFakeConn struct {
	mu      sync.Mutex
	closed  bool
	payload [][]byte
}

type openAIWSHealthProbeConn struct {
	pings   atomic.Int32
	pingErr error
}

func (c *openAIWSHealthProbeConn) WriteJSON(context.Context, any) error { return nil }

func (c *openAIWSHealthProbeConn) ReadMessage(context.Context) ([]byte, error) {
	return []byte(`{"type":"response.completed"}`), nil
}

func (c *openAIWSHealthProbeConn) Ping(context.Context) error {
	c.pings.Add(1)
	return c.pingErr
}

func (c *openAIWSHealthProbeConn) Close() error { return nil }

func (c *openAIWSFakeConn) WriteJSON(ctx context.Context, value any) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("closed")
	}
	c.payload = append(c.payload, []byte("ok"))
	_ = value
	return nil
}

func (c *openAIWSFakeConn) ReadMessage(ctx context.Context) ([]byte, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("closed")
	}
	return []byte(`{"type":"response.completed","response":{"id":"resp_fake"}}`), nil
}

func (c *openAIWSFakeConn) Ping(ctx context.Context) error {
	_ = ctx
	return nil
}

func (c *openAIWSFakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

type openAIWSBlockingConn struct {
	readDelay time.Duration
}

func (c *openAIWSBlockingConn) WriteJSON(ctx context.Context, value any) error {
	_ = ctx
	_ = value
	return nil
}

func (c *openAIWSBlockingConn) ReadMessage(ctx context.Context) ([]byte, error) {
	delay := c.readDelay
	if delay <= 0 {
		delay = 10 * time.Millisecond
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return []byte(`{"type":"response.completed","response":{"id":"resp_blocking"}}`), nil
	}
}

func (c *openAIWSBlockingConn) Ping(ctx context.Context) error {
	_ = ctx
	return nil
}

func (c *openAIWSBlockingConn) Close() error {
	return nil
}

type openAIWSWriteBlockingConn struct{}

func (c *openAIWSWriteBlockingConn) WriteJSON(ctx context.Context, _ any) error {
	<-ctx.Done()
	return ctx.Err()
}

func (c *openAIWSWriteBlockingConn) ReadMessage(context.Context) ([]byte, error) {
	return []byte(`{"type":"response.completed","response":{"id":"resp_write_block"}}`), nil
}

func (c *openAIWSWriteBlockingConn) Ping(context.Context) error {
	return nil
}

func (c *openAIWSWriteBlockingConn) Close() error {
	return nil
}

type openAIWSContextProbeConn struct {
	lastWriteCtx context.Context
}

func (c *openAIWSContextProbeConn) WriteJSON(ctx context.Context, _ any) error {
	c.lastWriteCtx = ctx
	return nil
}

func (c *openAIWSContextProbeConn) ReadMessage(context.Context) ([]byte, error) {
	return []byte(`{"type":"response.completed","response":{"id":"resp_ctx_probe"}}`), nil
}

func (c *openAIWSContextProbeConn) Ping(context.Context) error {
	return nil
}

func (c *openAIWSContextProbeConn) Close() error {
	return nil
}

type openAIWSNilConnDialer struct{}

func (d *openAIWSNilConnDialer) Dial(
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
	return nil, 200, nil, nil
}

func TestOpenAIWSConnPool_DialConnNilConnection(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 1

	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(&openAIWSNilConnDialer{})
	account := &Account{ID: 91, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	_, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil connection")
}

func TestOpenAIWSConnPool_SnapshotTransportMetrics(t *testing.T) {
	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)

	dialer, ok := pool.clientDialer.(*coderOpenAIWSClientDialer)
	require.True(t, ok)

	_, err := dialer.proxyHTTPClient("http://127.0.0.1:28080")
	require.NoError(t, err)
	_, err = dialer.proxyHTTPClient("http://127.0.0.1:28080")
	require.NoError(t, err)
	_, err = dialer.proxyHTTPClient("http://127.0.0.1:28081")
	require.NoError(t, err)

	snapshot := pool.SnapshotTransportMetrics()
	require.Equal(t, int64(1), snapshot.ProxyClientCacheHits)
	require.Equal(t, int64(2), snapshot.ProxyClientCacheMisses)
	require.InDelta(t, 1.0/3.0, snapshot.TransportReuseRatio, 0.0001)
}

func TestOpenAIWSConnPool_ProfileIsolation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	cfg.Gateway.OpenAIWS.StickyReservePercent = 50

	pool := newOpenAIWSConnPool(cfg)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 900, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	// 先建立一个 session_bound 空闲连接。
	leaseSession, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
	})
	require.NoError(t, err)
	leaseSession.Release()
	require.Equal(t, 1, dialer.DialCount())

	// neutral 请求不得借用 session_bound 空闲连接，应新建。
	leaseNeutral, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	require.Equal(t, openAIWSConnProfileNeutral, leaseNeutral.conn.profile)
	require.False(t, leaseNeutral.Reused(), "neutral 不应复用 session_bound 连接")
	leaseNeutral.Release()
	require.Equal(t, 2, dialer.DialCount())

	// neutral 再次请求应复用上一条 neutral 空闲连接。
	leaseNeutral2, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	require.True(t, leaseNeutral2.Reused(), "neutral 应复用已建立的 neutral 连接")
	leaseNeutral2.Release()
	require.Equal(t, 2, dialer.DialCount())

	// session_bound 请求没有明确 affinity 时不得泛复用带会话握手头的空闲连接。
	leaseSession2, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
	})
	require.NoError(t, err)
	require.False(t, leaseSession2.Reused(), "session_bound without affinity must not reuse another session-bound connection")
	require.Equal(t, openAIWSConnProfileSessionBound, leaseSession2.conn.profile)
	leaseSession2.Release()
	require.Equal(t, 3, dialer.DialCount())
}

func TestOpenAIWSConnPool_SeparatesConnectionsByHandshakeIdentity(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4

	pool := newOpenAIWSConnPool(cfg)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 902, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	headersA := http.Header{}
	headersA.Set("user-agent", "routed-a/1.0")
	headersA.Set("originator", "routed-a")
	leaseA, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Headers: headersA,
	})
	require.NoError(t, err)
	leaseA.Release()
	require.Equal(t, 1, dialer.DialCount())

	headersB := http.Header{}
	headersB.Set("user-agent", "routed-b/1.0")
	headersB.Set("originator", "routed-b")
	leaseB, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Headers: headersB,
	})
	require.NoError(t, err)
	require.False(t, leaseB.Reused(), "different routed upstream headers must not reuse an existing WS connection")
	leaseB.Release()
	require.Equal(t, 2, dialer.DialCount())
}

func TestOpenAIWSConnPool_SeparatesConnectionsByTLSProfileIdentity(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4

	pool := newOpenAIWSConnPool(cfg)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 903, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	leaseA, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		TLSProfile: &tlsfingerprint.Profile{
			Name:          "Chrome A",
			ALPNProtocols: []string{"h2", "http/1.1"},
			CipherSuites:  []uint16{0x1301},
		},
	})
	require.NoError(t, err)
	leaseA.Release()
	require.Equal(t, 1, dialer.DialCount())

	leaseB, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		TLSProfile: &tlsfingerprint.Profile{
			Name:          "Chrome B",
			ALPNProtocols: []string{"http/1.1"},
			CipherSuites:  []uint16{0x1302},
		},
	})
	require.NoError(t, err)
	require.False(t, leaseB.Reused(), "different routed TLS profiles must not reuse an existing WS connection")
	leaseB.Release()
	require.Equal(t, 2, dialer.DialCount())
}

func TestOpenAIWSConnPool_TLSProfileIdentityIncludesAuthHeaders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4

	pool := newOpenAIWSConnPool(cfg)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 904, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	profile := &tlsfingerprint.Profile{
		Name:          "Chrome Routed",
		ALPNProtocols: []string{"h2", "http/1.1"},
		CipherSuites:  []uint16{0x1301},
	}

	headersA := http.Header{}
	headersA.Set("authorization", "Bearer token-a")
	headersA.Set("chatgpt-account-id", "acct-a")
	leaseA, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account:    account,
		WSURL:      "wss://example.com/v1/responses",
		Headers:    headersA,
		TLSProfile: profile,
		Profile:    openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	leaseA.Release()
	require.Equal(t, 1, dialer.DialCount())

	headersB := http.Header{}
	headersB.Set("authorization", "Bearer token-b")
	headersB.Set("chatgpt-account-id", "acct-b")
	leaseB, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account:    account,
		WSURL:      "wss://example.com/v1/responses",
		Headers:    headersB,
		TLSProfile: profile,
		Profile:    openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	require.False(t, leaseB.Reused(), "TLS-routed neutral WS connections must not cross auth or ChatGPT account identities")
	leaseB.Release()
	require.Equal(t, 2, dialer.DialCount())
}

func TestOpenAIWSConnPool_SessionDoesNotEvictIdleNeutralWhenConcurrencyFull(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.StickyReservePercent = 50

	pool := newOpenAIWSConnPool(cfg)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 901, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}

	// 即使账号并发为 1，idle WS cache 也不作为请求容量占用。
	leaseSession, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
	})
	require.NoError(t, err)
	leaseNeutral, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	leaseNeutral.Release() // neutral 变空闲
	require.Equal(t, 2, dialer.DialCount())

	// 等待可能的异步预热 settle，确保 creating 归零，使后续断言不受预热竞态影响。
	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(account.ID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return ap.creating == 0
	}, time.Second, 5*time.Millisecond)

	// 第二个 session_bound 请求应直接新建，不为了“腾容量”删除空闲 neutral。
	dialBefore := dialer.DialCount()
	leaseSession2, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
	})
	require.NoError(t, err)
	require.Equal(t, openAIWSConnProfileSessionBound, leaseSession2.conn.profile)
	require.False(t, leaseSession2.Reused(), "应新建 session_bound 而非复用")
	require.Equal(t, dialBefore+1, dialer.DialCount(), "应新建一条 session_bound")

	// 空闲 neutral 保持等待后续请求，由 TTL/cleanup 周期统一回收。
	ap, ok := pool.getAccountPool(account.ID)
	require.True(t, ok)
	ap.mu.Lock()
	require.Equal(t, 1, countConnsByProfileLocked(ap, openAIWSConnProfileNeutral), "空闲 neutral 不应被 session 建连驱逐")
	require.Equal(t, 3, len(ap.conns), "idle WS cache can exceed account concurrency")
	ap.mu.Unlock()

	leaseSession.Release()
	leaseSession2.Release()
}

func TestOpenAIWSConnPool_NeutralIdentityMismatchKeepsIdleUntilCleanup(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.StickyReservePercent = 50

	pool := newOpenAIWSConnPool(cfg)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 903, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	headersA := http.Header{}
	headersA.Set("originator", "codex_cli_rs")
	headersA.Set("user-agent", codexCLIUserAgent)
	headersA.Set("OpenAI-Beta", openAIWSBetaV2Value)
	headersB := headersA.Clone()
	headersB.Set("originator", "opencode")

	leaseA, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Headers: headersA,
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	firstConnID := leaseA.ConnID()
	leaseA.Release()
	require.Equal(t, 1, dialer.DialCount())

	leaseB, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Headers: headersB,
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	require.False(t, leaseB.Reused(), "neutral identity mismatch must create a fresh connection")
	require.NotEqual(t, firstConnID, leaseB.ConnID())
	require.Equal(t, 2, dialer.DialCount())
	leaseB.Release()

	ap, ok := pool.getAccountPool(account.ID)
	require.True(t, ok)
	ap.mu.Lock()
	_, oldExists := ap.conns[firstConnID]
	neutralCount := countConnsByProfileLocked(ap, openAIWSConnProfileNeutral)
	ap.mu.Unlock()
	require.True(t, oldExists, "idle neutral with the previous identity remains until TTL/target cleanup")
	require.Equal(t, 2, neutralCount)
}

func TestOpenAIWSConnPool_NeutralPrewarmIgnoresSessionCreating(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	cfg.Gateway.OpenAIWS.StickyReservePercent = 50

	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(25, 120, 0, 4, 0)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 904, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}
	ap := pool.getOrCreateAccountPool(account.ID)
	ap.mu.Lock()
	ap.creating = 1 // simulate a session_bound dial already in progress
	ap.mu.Unlock()

	pool.PrewarmNeutral(account.ID, openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}, 1)

	require.Equal(t, 1, dialer.DialCount(), "session_bound creating must not consume neutral prewarm capacity")
}

func TestOpenAIWSConnPool_NeutralCreateCanTemporarilyExceedTargetBeforeCleanup(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 2
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.StickyReservePercent = 50

	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSBlockingDialer{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 905, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}

	type acquireResult struct {
		lease *openAIWSConnLease
		err   error
	}
	resultCh := make(chan acquireResult, 1)
	go func() {
		lease, err := pool.Acquire(context.Background(), req)
		resultCh <- acquireResult{lease: lease, err: err}
	}()

	select {
	case <-dialer.started:
	case <-time.After(time.Second):
		t.Fatal("neutral dial did not start")
	}

	ap := pool.getOrCreateAccountPool(account.ID)
	reuseKey := openAIWSConnReuseKeyForAcquire(req)
	existing := newOpenAIWSConnWithProfileAndReuseKey("existing_neutral", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral, reuseKey)
	ap.mu.Lock()
	ap.conns[existing.id] = existing
	ap.mu.Unlock()

	close(dialer.release)

	var result acquireResult
	select {
	case result = <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("neutral acquire did not finish")
	}
	require.NoError(t, result.err)
	t.Cleanup(result.lease.Release)
	require.False(t, result.lease.Reused(), "raced neutral dial is kept as short-lived surplus cache")

	ap.mu.Lock()
	neutralCount := countConnsByProfileLocked(ap, openAIWSConnProfileNeutral)
	ap.mu.Unlock()
	require.Equal(t, 2, neutralCount, "surplus neutral conns are reclaimed by cleanup, not by acquire hard caps")
	require.Equal(t, 1, dialer.DialCount())
}

func TestOpenAIWSConnPool_NeutralMaxConns(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	pool := newOpenAIWSConnPool(cfg)
	require.Equal(t, 2, pool.neutralMaxConns(10), "default 20 percent target")
	require.Equal(t, 0, pool.neutralMaxConns(1), "floor(1 * 20 / 100) keeps no proactive neutral when min_idle is not configured")

	StoreOpenAIWSPoolRuntimeSettings(50, 120)
	require.Equal(t, 5, pool.neutralMaxConns(10))
	require.Equal(t, 0, pool.neutralMaxConns(0))
}

func TestOpenAIWSConnPool_EnsureTargetIdleNeutral(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 8
	cfg.Gateway.OpenAIWS.StickyReservePercent = 50
	cfg.Gateway.OpenAIWS.DynamicMaxConnsByAccountConcurrencyEnabled = false

	pool := newOpenAIWSConnPool(cfg)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(50, 120, 2, 8, 0)
	dialer := &openAIWSFakeDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 902, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}

	// 一次 neutral 请求建立快照后释放，触发异步中性预热补足到 percent target。
	lease, err := pool.Acquire(context.Background(), openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	})
	require.NoError(t, err)
	lease.Release()

	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(account.ID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return countConnsByProfileLocked(ap, openAIWSConnProfileNeutral) >= 2
	}, 2*time.Second, 10*time.Millisecond, "中性预热应补足到 percent target")
}

func TestOpenAIWSConnPool_EnsureTargetIdlePrewarmsBoundedNeutralVariants(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.StickyReservePercent = 0
	cfg.Gateway.OpenAIWS.PrewarmCooldownMS = 0

	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 908, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}

	headersA := http.Header{}
	headersA.Set("authorization", "Bearer same-token")
	headersA.Set("originator", "codex_cli_rs")
	headersA.Set("user-agent", codexCLIUserAgent)
	headersA.Set("OpenAI-Beta", openAIWSBetaV2Value)
	headersB := headersA.Clone()
	headersB.Set("originator", "opencode")
	headersB.Set("user-agent", "opencode/1.0")

	reqA := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Headers: headersA,
		Profile: openAIWSConnProfileNeutral,
	}
	reqB := openAIWSAcquireRequest{
		Account: account,
		WSURL:   reqA.WSURL,
		Headers: headersB,
		Profile: openAIWSConnProfileNeutral,
	}
	reuseKeyA := openAIWSConnReuseKeyForAcquire(reqA)
	reuseKeyB := openAIWSConnReuseKeyForAcquire(reqB)

	StoreOpenAIWSPoolRuntimeSettingsWithIdle(0, 120, 0, 2, 0)
	leaseA, err := pool.Acquire(context.Background(), reqA)
	require.NoError(t, err)
	require.False(t, leaseA.Reused())
	leaseA.Release()

	leaseB, err := pool.Acquire(context.Background(), reqB)
	require.NoError(t, err)
	require.False(t, leaseB.Reused())
	leaseB.Release()
	require.Equal(t, 2, dialer.DialCount())

	StoreOpenAIWSPoolRuntimeSettingsWithIdle(50, 120, 0, 2, 0)
	pool.ensureTargetIdleAsync(account.ID)

	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(account.ID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return !ap.prewarmActive &&
			countNeutralStockConnsLocked(ap) == 2 &&
			countNeutralStockConnsByReuseKeyLocked(ap, reuseKeyA) == 1 &&
			countNeutralStockConnsByReuseKeyLocked(ap, reuseKeyB) == 1
	}, 2*time.Second, 10*time.Millisecond, "neutral refill should spread account target across remembered variants without exceeding target")

	leaseA2, err := pool.Acquire(context.Background(), reqA)
	require.NoError(t, err)
	require.True(t, leaseA2.Reused(), "variant A should reuse its prewarmed neutral stock")
	leaseA2.Release()

	leaseB2, err := pool.Acquire(context.Background(), reqB)
	require.NoError(t, err)
	require.True(t, leaseB2.Reused(), "variant B should reuse its prewarmed neutral stock")
	leaseB2.Release()
}

func TestOpenAIWSConnPool_EnsureTargetIdleEvictsNeutralVariantsOverCap(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 8
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	cfg.Gateway.OpenAIWS.StickyReservePercent = 0
	cfg.Gateway.OpenAIWS.PrewarmCooldownMS = 0

	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 909, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}

	reqs := make([]openAIWSAcquireRequest, 0, 5)
	reuseKeys := make([]string, 0, 5)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(0, 120, 0, 4, 0)
	for i := 0; i < 5; i++ {
		headers := http.Header{}
		headers.Set("authorization", "Bearer same-token")
		headers.Set("originator", fmt.Sprintf("client-%d", i))
		headers.Set("user-agent", fmt.Sprintf("client/%d", i))
		headers.Set("OpenAI-Beta", openAIWSBetaV2Value)
		req := openAIWSAcquireRequest{
			Account: account,
			WSURL:   "wss://example.com/v1/responses",
			Headers: headers,
			Profile: openAIWSConnProfileNeutral,
		}
		lease, err := pool.Acquire(context.Background(), req)
		require.NoError(t, err)
		require.False(t, lease.Reused())
		lease.Release()
		reqs = append(reqs, req)
		reuseKeys = append(reuseKeys, openAIWSConnReuseKeyForAcquire(req))
	}
	require.Equal(t, 5, dialer.DialCount())

	StoreOpenAIWSPoolRuntimeSettingsWithIdle(100, 120, 0, 4, 0)
	pool.ensureTargetIdleAsync(account.ID)

	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(account.ID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		if ap.prewarmActive || countNeutralStockConnsLocked(ap) != 4 {
			return false
		}
		if countNeutralStockConnsByReuseKeyLocked(ap, reuseKeys[0]) != 0 {
			return false
		}
		for _, reuseKey := range reuseKeys[1:] {
			if countNeutralStockConnsByReuseKeyLocked(ap, reuseKey) != 1 {
				return false
			}
		}
		return true
	}, 2*time.Second, 10*time.Millisecond, "catalog should keep the four most recent variants and prewarm one stock conn for each")

	for _, req := range reqs[1:] {
		lease, err := pool.Acquire(context.Background(), req)
		require.NoError(t, err)
		require.True(t, lease.Reused())
		lease.Release()
	}
}

func TestOpenAIWSConnPool_EnsureTargetIdleDropsExpiredNeutralVariants(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 2
	cfg.Gateway.OpenAIWS.StickyReservePercent = 0
	cfg.Gateway.OpenAIWS.PrewarmCooldownMS = 0

	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)
	account := &Account{ID: 910, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}
	headers := http.Header{}
	headers.Set("authorization", "Bearer same-token")
	headers.Set("originator", "codex_cli_rs")
	headers.Set("user-agent", codexCLIUserAgent)
	headers.Set("OpenAI-Beta", openAIWSBetaV2Value)
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Headers: headers,
		Profile: openAIWSConnProfileNeutral,
	}

	StoreOpenAIWSPoolRuntimeSettingsWithIdle(0, 1, 0, 2, 0)
	lease, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	require.False(t, lease.Reused())
	lease.Release()
	require.Equal(t, 1, dialer.DialCount())

	time.Sleep(1100 * time.Millisecond)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(50, 1, 0, 2, 0)
	pool.ensureTargetIdleAsync(account.ID)
	time.Sleep(150 * time.Millisecond)

	ap, ok := pool.getAccountPool(account.ID)
	require.True(t, ok)
	ap.mu.Lock()
	stock := countNeutralStockConnsLocked(ap)
	prewarmActive := ap.prewarmActive
	ap.mu.Unlock()
	require.False(t, prewarmActive)
	require.Equal(t, 0, stock, "expired neutral variants should be pruned instead of used as prewarm templates")
	require.Equal(t, 1, dialer.DialCount(), "expired variant should not trigger a fresh neutral prewarm dial")
}

func TestOpenAIWSConnPool_UsedNeutralDoesNotBlockInventoryReplenish(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	cfg.Gateway.OpenAIWS.StickyReservePercent = 50

	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(50, 120, 0, 4, 0)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	account := &Account{ID: 906, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}
	req := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}

	pool.PrewarmNeutral(account.ID, req, 2)
	require.Equal(t, 2, dialer.DialCount(), "应先补足 2 条 neutral 预热库存")

	lease, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	require.True(t, lease.Reused(), "真实请求应复用已有 neutral 连接")
	lease.Release()

	require.Eventually(t, func() bool {
		ap, ok := pool.getAccountPool(account.ID)
		if !ok || ap == nil {
			return false
		}
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return len(ap.conns) >= 3 && ap.creating == 0
	}, 2*time.Second, 10*time.Millisecond, "已被真实请求用过的 neutral 不应继续占用库存名额，连接池应补回新的库存")
}

func TestOpenAIWSConnPool_PromoteNeutralConnToSessionBoundClearsNeutralStock(t *testing.T) {
	pool := newOpenAIWSConnPool(&config.Config{})

	accountID := int64(9061)
	ap := pool.getOrCreateAccountPool(accountID)
	conn := newOpenAIWSConnWithProfile("neutral_promote", &openAIWSFakeConn{}, nil, openAIWSConnProfileNeutral)
	conn.markNeutralStock()

	ap.mu.Lock()
	ap.conns[conn.id] = conn
	ap.mu.Unlock()

	require.True(t, pool.PromoteNeutralConnToSessionBound(accountID, conn.id))

	ap.mu.Lock()
	defer ap.mu.Unlock()
	require.Equal(t, openAIWSConnProfileSessionBound, conn.profile)
	require.False(t, conn.isNeutralStock(), "neutral 晋升为 session_bound 后不应继续占用 neutral 库存")
}

func TestOpenAIWSConnPool_CleanupNeutralOverTargetKeepsReusableNeutral(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 4
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 4
	cfg.Gateway.OpenAIWS.StickyReservePercent = 50

	pool := newOpenAIWSConnPool(cfg)
	t.Cleanup(pool.Close)
	StoreOpenAIWSPoolRuntimeSettingsWithIdle(25, 120, 0, 4, 0)
	dialer := &openAIWSCountingDialer{}
	pool.setClientDialerForTest(dialer)

	account := &Account{ID: 907, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 4}
	baseReq := openAIWSAcquireRequest{
		Account: account,
		WSURL:   "wss://example.com/v1/responses",
		Profile: openAIWSConnProfileNeutral,
	}

	pool.PrewarmNeutral(account.ID, baseReq, 1)
	require.Equal(t, 1, dialer.DialCount(), "应先建立 1 条 neutral 预热库存")

	reusableHeaders := http.Header{}
	reusableHeaders.Set("originator", "opencode")
	reusableReq := openAIWSAcquireRequest{
		Account: account,
		WSURL:   baseReq.WSURL,
		Headers: reusableHeaders,
		Profile: openAIWSConnProfileNeutral,
	}
	lease, err := pool.Acquire(context.Background(), reusableReq)
	require.NoError(t, err)
	require.False(t, lease.Reused(), "不同 reuse key 的 neutral 请求应新建连接")
	reusableConnID := lease.ConnID()
	lease.Release()
	require.Equal(t, 2, dialer.DialCount())

	ap, ok := pool.getAccountPool(account.ID)
	require.True(t, ok)
	require.NotNil(t, ap)

	var evicted []*openAIWSConn
	ap.mu.Lock()
	for _, conn := range ap.conns {
		if conn == nil {
			continue
		}
		if conn.id == reusableConnID {
			conn.lastUsedNano.Store(time.Now().Add(-60 * time.Second).UnixNano())
			continue
		}
		conn.lastUsedNano.Store(time.Now().Add(-30 * time.Second).UnixNano())
	}
	evicted = pool.cleanupAccountLocked(ap, time.Now(), account.Concurrency)
	_, reusableStillExists := ap.conns[reusableConnID]
	neutralCount := countConnsByProfileLocked(ap, openAIWSConnProfileNeutral)
	ap.mu.Unlock()
	closeOpenAIWSConns(evicted)

	require.True(t, reusableStillExists, "neutral_over_target 不应优先驱逐已存在的 reusable neutral")
	require.Equal(t, 2, neutralCount, "当库存 neutral 已在 target 内时，cleanup 不应因为 reusable neutral 存在而缩容")
}
