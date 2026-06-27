package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIWSStateStore_BindGetDeleteResponseAccount(t *testing.T) {
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	ctx := context.Background()
	groupID := int64(7)
	apiKeyID := int64(11)

	require.NoError(t, store.BindResponseAccount(ctx, groupID, apiKeyID, "resp_abc", 101, time.Minute))

	accountID, err := store.GetResponseAccount(ctx, groupID, apiKeyID, "resp_abc")
	require.NoError(t, err)
	require.Equal(t, int64(101), accountID)

	require.NoError(t, store.DeleteResponseAccount(ctx, groupID, apiKeyID, "resp_abc"))
	accountID, err = store.GetResponseAccount(ctx, groupID, apiKeyID, "resp_abc")
	require.NoError(t, err)
	require.Zero(t, accountID)
}

func TestOpenAIWSStateStore_ResponseAccountLocalCacheIsGroupIsolated(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	ctx := context.Background()

	require.NoError(t, store.BindResponseAccount(ctx, 7, 11, "resp_shared", 101, time.Minute))

	accountID, err := store.GetResponseAccount(ctx, 8, 11, "resp_shared")
	require.NoError(t, err)
	require.Zero(t, accountID, "local response account cache must not bypass group isolation")
}

func TestOpenAIWSStateStore_ResponseStateIsAPIKeyIsolated(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	ctx := context.Background()

	require.NoError(t, store.BindResponseAccount(ctx, 7, 11, "resp_shared", 101, time.Minute))
	accountID, err := store.GetResponseAccount(ctx, 7, 12, "resp_shared")
	require.NoError(t, err)
	require.Zero(t, accountID, "response account cache must not cross API keys in the same group")

	store.BindResponseConn(7, 11, "resp_shared", "conn_11", time.Minute)
	_, ok := store.GetResponseConn(7, 12, "resp_shared")
	require.False(t, ok, "response conn cache must not cross API keys in the same group")

	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	require.NoError(t, writer.BindResponseAccount(ctx, 7, 11, "resp_shared_redis", 201, time.Minute))
	reader := NewOpenAIWSStateStore(cache)
	accountID, err = reader.GetResponseAccount(ctx, 7, 12, "resp_shared_redis")
	require.NoError(t, err)
	require.Zero(t, accountID, "redis response account cache must not cross API keys in the same group")
	accountID, err = reader.GetResponseAccount(ctx, 7, 11, "resp_shared_redis")
	require.NoError(t, err)
	require.Equal(t, int64(201), accountID)
}

func TestOpenAIWSStateStore_ResponseAccountLegacyBindingIsNotVisibleToScopedAPIKey(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	ctx := context.Background()

	require.NoError(t, store.BindResponseAccount(ctx, 7, 0, "resp_legacy", 101, time.Minute))

	accountID, err := store.GetResponseAccount(ctx, 7, 11, "resp_legacy")
	require.NoError(t, err)
	require.Zero(t, accountID, "api-key scoped lookup must not fall back to legacy apiKeyID=0 bindings")

	require.NoError(t, store.DeleteResponseAccount(ctx, 7, 11, "resp_legacy"))
	accountID, err = store.GetResponseAccount(ctx, 7, 0, "resp_legacy")
	require.NoError(t, err)
	require.Equal(t, int64(101), accountID, "deleting scoped binding must not remove the legacy key")

	require.NoError(t, store.DeleteResponseAccount(ctx, 7, 0, "resp_legacy"))
	accountID, err = store.GetResponseAccount(ctx, 7, 0, "resp_legacy")
	require.NoError(t, err)
	require.Zero(t, accountID)

	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	require.NoError(t, writer.BindResponseAccount(ctx, 7, 0, "resp_legacy_redis", 202, time.Minute))

	reader := NewOpenAIWSStateStore(cache)
	accountID, err = reader.GetResponseAccount(ctx, 7, 11, "resp_legacy_redis")
	require.NoError(t, err)
	require.Zero(t, accountID, "api-key scoped redis lookup must not fall back to legacy apiKeyID=0 bindings")

	require.NoError(t, reader.DeleteResponseAccount(ctx, 7, 11, "resp_legacy_redis"))
	accountID, err = reader.GetResponseAccount(ctx, 7, 0, "resp_legacy_redis")
	require.NoError(t, err)
	require.Equal(t, int64(202), accountID, "deleting scoped redis binding must not remove the legacy key")
}

func TestOpenAIWSStateStore_ResponseConnTTL(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindResponseConn(7, 11, "resp_conn", "conn_1", 30*time.Millisecond)

	connID, ok := store.GetResponseConn(7, 11, "resp_conn")
	require.True(t, ok)
	require.Equal(t, "conn_1", connID)

	_, ok = store.GetResponseConn(8, 11, "resp_conn")
	require.False(t, ok, "local response conn cache must not bypass group isolation")

	time.Sleep(60 * time.Millisecond)
	_, ok = store.GetResponseConn(7, 11, "resp_conn")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_SessionTurnStateTTL(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindSessionTurnState(9, "session_hash_1", "turn_state_1", 30*time.Millisecond)

	state, ok := store.GetSessionTurnState(9, "session_hash_1")
	require.True(t, ok)
	require.Equal(t, "turn_state_1", state)

	// group 隔离
	_, ok = store.GetSessionTurnState(10, "session_hash_1")
	require.False(t, ok)

	time.Sleep(60 * time.Millisecond)
	_, ok = store.GetSessionTurnState(9, "session_hash_1")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_SessionConnTTL(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindSessionConn(9, 101, 201, "session_hash_conn_1", "conn_1", 30*time.Millisecond)

	connID, ok := store.GetSessionConn(9, 101, 201, "session_hash_conn_1")
	require.True(t, ok)
	require.Equal(t, "conn_1", connID)

	// group / api key / account isolation.
	_, ok = store.GetSessionConn(10, 101, 201, "session_hash_conn_1")
	require.False(t, ok)
	_, ok = store.GetSessionConn(9, 102, 201, "session_hash_conn_1")
	require.False(t, ok)
	_, ok = store.GetSessionConn(9, 101, 202, "session_hash_conn_1")
	require.False(t, ok)

	time.Sleep(60 * time.Millisecond)
	_, ok = store.GetSessionConn(9, 101, 201, "session_hash_conn_1")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_DeleteConnScopedStateDeletesMatchingSessionContext(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindSessionConn(7, 11, 101, "sess_a", "conn_a", time.Minute)
	store.BindSessionConn(7, 11, 102, "sess_b", "conn_b", time.Minute)
	store.BindSessionContext(7, 11, "sess_a", openAIWSSessionContextValue{
		accountID:      101,
		connID:         "conn_a",
		lastResponseID: "resp_a",
	}, time.Minute)
	store.BindSessionContext(7, 11, "sess_b", openAIWSSessionContextValue{
		accountID:      102,
		connID:         "conn_b",
		lastResponseID: "resp_b",
	}, time.Minute)
	store.BindConnLastResponse("conn_a", "resp_a", time.Minute)
	store.BindConnLastResponse("conn_b", "resp_b", time.Minute)

	store.DeleteConnScopedState("conn_a", "session_idle_ttl")

	_, ok := store.GetSessionConn(7, 11, 101, "sess_a")
	require.False(t, ok, "session -> conn binding for evicted conn must be cleared")
	connID, ok := store.GetSessionConn(7, 11, 102, "sess_b")
	require.True(t, ok)
	require.Equal(t, "conn_b", connID)

	_, ok = store.GetSessionContext(7, 11, "sess_a")
	require.False(t, ok, "session context tied to evicted conn must be cleared to avoid stale conn_mismatch full replay")
	ctx, ok := store.GetSessionContext(7, 11, "sess_b")
	require.True(t, ok)
	require.Equal(t, "conn_b", ctx.connID)

	_, ok = store.GetConnLastResponse("conn_a")
	require.False(t, ok, "conn last response for evicted conn must be cleared")
	respID, ok := store.GetConnLastResponse("conn_b")
	require.True(t, ok)
	require.Equal(t, "resp_b", respID)
}

func TestOpenAIWSStateStore_GetResponseAccount_NoStaleAfterCacheMiss(t *testing.T) {
	cache := &stubGatewayCache{sessionBindings: map[string]int64{}}
	store := NewOpenAIWSStateStore(cache)
	ctx := context.Background()
	groupID := int64(17)
	responseID := "resp_cache_stale"
	cacheKey := openAIWSResponseAccountCacheKey(11, responseID)

	cache.sessionBindings[cacheKey] = 501
	accountID, err := store.GetResponseAccount(ctx, groupID, 11, responseID)
	require.NoError(t, err)
	require.Equal(t, int64(501), accountID)

	delete(cache.sessionBindings, cacheKey)
	accountID, err = store.GetResponseAccount(ctx, groupID, 11, responseID)
	require.NoError(t, err)
	require.Zero(t, accountID, "上游缓存失效后不应继续命中本地陈旧映射")
}

func TestOpenAIWSStateStore_MaybeCleanupRemovesExpiredIncrementally(t *testing.T) {
	raw := NewOpenAIWSStateStore(nil)
	store, ok := raw.(*defaultOpenAIWSStateStore)
	require.True(t, ok)

	expiredAt := time.Now().Add(-time.Minute)
	total := 2048
	store.responseToConnMu.Lock()
	for i := 0; i < total; i++ {
		store.responseToConn[fmt.Sprintf("resp_%d", i)] = openAIWSConnBinding{
			connID:    "conn_incremental",
			expiresAt: expiredAt,
		}
	}
	store.responseToConnMu.Unlock()

	store.lastCleanupUnixNano.Store(time.Now().Add(-2 * openAIWSStateStoreCleanupInterval).UnixNano())
	store.maybeCleanup()

	store.responseToConnMu.RLock()
	remainingAfterFirst := len(store.responseToConn)
	store.responseToConnMu.RUnlock()
	require.Less(t, remainingAfterFirst, total, "单轮 cleanup 应至少有进展")
	require.Greater(t, remainingAfterFirst, 0, "增量清理不要求单轮清空全部键")

	for i := 0; i < 8; i++ {
		store.lastCleanupUnixNano.Store(time.Now().Add(-2 * openAIWSStateStoreCleanupInterval).UnixNano())
		store.maybeCleanup()
	}

	store.responseToConnMu.RLock()
	remaining := len(store.responseToConn)
	store.responseToConnMu.RUnlock()
	require.Zero(t, remaining, "多轮 cleanup 后应逐步清空全部过期键")
}

func TestEnsureBindingCapacity_EvictsOneWhenMapIsFull(t *testing.T) {
	bindings := map[string]int{
		"a": 1,
		"b": 2,
	}

	ensureBindingCapacity(bindings, "c", 2)
	bindings["c"] = 3

	require.Len(t, bindings, 2)
	require.Equal(t, 3, bindings["c"])
}

func TestEnsureBindingCapacity_DoesNotEvictWhenUpdatingExistingKey(t *testing.T) {
	bindings := map[string]int{
		"a": 1,
		"b": 2,
	}

	ensureBindingCapacity(bindings, "a", 2)
	bindings["a"] = 9

	require.Len(t, bindings, 2)
	require.Equal(t, 9, bindings["a"])
}

type openAIWSStateStoreTimeoutProbeCache struct {
	setHasDeadline    bool
	getHasDeadline    bool
	deleteHasDeadline bool
	setDeadlineDelta  time.Duration
	getDeadlineDelta  time.Duration
	delDeadlineDelta  time.Duration
}

func (c *openAIWSStateStoreTimeoutProbeCache) GetSessionAccountID(ctx context.Context, _ int64, _ string) (int64, error) {
	if deadline, ok := ctx.Deadline(); ok {
		c.getHasDeadline = true
		c.getDeadlineDelta = time.Until(deadline)
	}
	return 123, nil
}

func (c *openAIWSStateStoreTimeoutProbeCache) SetSessionAccountID(ctx context.Context, _ int64, _ string, _ int64, _ time.Duration) error {
	if deadline, ok := ctx.Deadline(); ok {
		c.setHasDeadline = true
		c.setDeadlineDelta = time.Until(deadline)
	}
	return errors.New("set failed")
}

func (c *openAIWSStateStoreTimeoutProbeCache) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}

func (c *openAIWSStateStoreTimeoutProbeCache) DeleteSessionAccountID(ctx context.Context, _ int64, _ string) error {
	if deadline, ok := ctx.Deadline(); ok {
		c.deleteHasDeadline = true
		c.delDeadlineDelta = time.Until(deadline)
	}
	return nil
}

func (c *openAIWSStateStoreTimeoutProbeCache) GetOpenAIResponsesSessionWindow(ctx context.Context, _ int64, _ string) ([]byte, error) {
	if deadline, ok := ctx.Deadline(); ok {
		c.getHasDeadline = true
		c.getDeadlineDelta = time.Until(deadline)
	}
	return nil, errors.New("not found")
}

func (c *openAIWSStateStoreTimeoutProbeCache) SetOpenAIResponsesSessionWindow(ctx context.Context, _ int64, _ string, _ []byte, _ time.Duration) error {
	if deadline, ok := ctx.Deadline(); ok {
		c.setHasDeadline = true
		c.setDeadlineDelta = time.Until(deadline)
	}
	return errors.New("set failed")
}

func (c *openAIWSStateStoreTimeoutProbeCache) DeleteOpenAIResponsesSessionWindow(ctx context.Context, _ int64, _ string) error {
	if deadline, ok := ctx.Deadline(); ok {
		c.deleteHasDeadline = true
		c.delDeadlineDelta = time.Until(deadline)
	}
	return nil
}

func TestOpenAIWSStateStore_RedisOpsUseShortTimeout(t *testing.T) {
	probe := &openAIWSStateStoreTimeoutProbeCache{}
	store := NewOpenAIWSStateStore(probe)
	ctx := context.Background()
	groupID := int64(5)

	err := store.BindResponseAccount(ctx, groupID, 0, "resp_timeout_probe", 11, time.Minute)
	require.Error(t, err)

	accountID, getErr := store.GetResponseAccount(ctx, groupID, 0, "resp_timeout_probe")
	require.NoError(t, getErr)
	require.Equal(t, int64(11), accountID, "本地缓存命中应优先返回已绑定账号")

	require.NoError(t, store.DeleteResponseAccount(ctx, groupID, 0, "resp_timeout_probe"))

	require.True(t, probe.setHasDeadline, "SetSessionAccountID 应携带独立超时上下文")
	require.True(t, probe.deleteHasDeadline, "DeleteSessionAccountID 应携带独立超时上下文")
	require.False(t, probe.getHasDeadline, "GetSessionAccountID 本用例应由本地缓存命中，不触发 Redis 读取")
	require.Greater(t, probe.setDeadlineDelta, 2*time.Second)
	require.LessOrEqual(t, probe.setDeadlineDelta, 3*time.Second)
	require.Greater(t, probe.delDeadlineDelta, 2*time.Second)
	require.LessOrEqual(t, probe.delDeadlineDelta, 3*time.Second)

	probe2 := &openAIWSStateStoreTimeoutProbeCache{}
	store2 := NewOpenAIWSStateStore(probe2)
	accountID2, err2 := store2.GetResponseAccount(ctx, groupID, 0, "resp_cache_only")
	require.NoError(t, err2)
	require.Equal(t, int64(123), accountID2)
	require.True(t, probe2.getHasDeadline, "GetSessionAccountID 在缓存未命中时应携带独立超时上下文")
	require.Greater(t, probe2.getDeadlineDelta, 2*time.Second)
	require.LessOrEqual(t, probe2.getDeadlineDelta, 3*time.Second)
}

func TestWithOpenAIWSStateStoreRedisTimeout_WithParentContext(t *testing.T) {
	ctx, cancel := withOpenAIWSStateStoreRedisTimeout(context.Background())
	defer cancel()
	require.NotNil(t, ctx)
	_, ok := ctx.Deadline()
	require.True(t, ok, "应附加短超时")
}
