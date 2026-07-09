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

func TestOpenAIWSStateStore_DeleteConnScopedStateEvictDeletesMatchingSessionContext(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindSessionContext(7, 11, "sess_evict", openAIWSSessionContextValue{
		accountID:      101,
		connID:         "conn_evict",
		lastResponseID: "resp_evict",
	}, time.Minute)
	store.BindConnLastResponse("conn_evict", "resp_evict", time.Minute)

	store.DeleteConnScopedState("conn_evict", "evict")

	_, ok := store.GetSessionContext(7, 11, "sess_evict")
	require.False(t, ok, "generic broken-connection evict must clear session context to avoid stale conn_mismatch retry")
	_, ok = store.GetConnLastResponse("conn_evict")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_DeleteConnScopedStateReadFailDeletesMatchingSessionContext(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindSessionContext(7, 11, "sess_read_fail", openAIWSSessionContextValue{
		accountID:      101,
		connID:         "conn_read_fail",
		lastResponseID: "resp_read_fail",
	}, time.Minute)
	store.BindConnLastResponse("conn_read_fail", "resp_read_fail", time.Minute)

	store.DeleteConnScopedState("conn_read_fail", "read_fail")

	_, ok := store.GetSessionContext(7, 11, "sess_read_fail")
	require.False(t, ok, "read-failed upstream conn must not leave stale session context")
	_, ok = store.GetConnLastResponse("conn_read_fail")
	require.False(t, ok)

	reason, age, ok := store.GetConnLastEvict("conn_read_fail")
	require.True(t, ok)
	require.Equal(t, "read_fail", reason)
	require.GreaterOrEqual(t, age, time.Duration(0))
}

func TestOpenAIWSStateStore_DeleteConnScopedStateWriteFailKeepsSessionContextForReanchor(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindSessionContext(7, 11, "sess_write_fail", openAIWSSessionContextValue{
		accountID:      101,
		connID:         "conn_write_fail",
		lastResponseID: "resp_write_fail",
	}, time.Minute)
	store.BindConnLastResponse("conn_write_fail", "resp_write_fail", time.Minute)

	store.DeleteConnScopedState("conn_write_fail", "write_request_fail")

	cached, ok := store.GetSessionContext(7, 11, "sess_write_fail")
	require.True(t, ok, "write-failed request can safely reanchor cached previous_response_id on a replacement conn")
	require.Equal(t, "resp_write_fail", cached.lastResponseID)
	_, ok = store.GetConnLastResponse("conn_write_fail")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_RestoresSessionContextFromSharedHashCacheWithoutConnTrust(t *testing.T) {
	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	reader := NewOpenAIWSStateStore(cache)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	output1 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"two"}]}`
	payload := []byte(`{"model":"gpt-5.5","store":false,"input":[` + input1 + `,` + output1 + `]}`)
	nonInputHash, _, nonInputFields := openAIWSNonInputFingerprint(payload)

	writer.BindSessionContext(7, 11, "sess_shared_ctx", openAIWSSessionContextValue{
		accountID:               101,
		connID:                  "oa_ws_101_old_process",
		lastResponseID:          "resp_shared_ctx_1",
		materializedHashes:      [][32]byte{mustItemHash(t, input1), mustItemHash(t, output1)},
		materializedShapes:      []string{"type=message;paths=$.content[]", "type=message;paths=$.content[]"},
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInputHash,
		nonInputFields:          nonInputFields,
		rawVsClientVisibleEqual: true,
	}, time.Hour)

	restored, ok := reader.GetSessionContext(7, 11, "sess_shared_ctx")
	require.True(t, ok)
	require.Equal(t, int64(101), restored.accountID)
	require.Empty(t, restored.connID, "shared hash context must not trust stale in-process conn_id")
	require.Equal(t, "resp_shared_ctx_1", restored.lastResponseID)
	require.Equal(t, 2, restored.materializedCount)
	require.Equal(t, 1, restored.inputCount)
	require.Len(t, restored.materializedHashes, 2)
	require.Equal(t, mustItemHash(t, input1), restored.materializedHashes[0])
	require.Equal(t, mustItemHash(t, output1), restored.materializedHashes[1])
	require.Empty(t, restored.materializedShapes, "durable context should not persist diagnostic shapes")
	require.Equal(t, nonInputHash, restored.nonInputHash)
	require.Contains(t, restored.nonInputFields, "model")
	require.True(t, restored.rawVsClientVisibleEqual)
}

func TestOpenAIWSStateStore_RestoredSharedHashContextPreservesHTTPTransportMarker(t *testing.T) {
	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	reader := NewOpenAIWSStateStore(cache)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	payload := []byte(`{"model":"gpt-5.5","store":false,"input":[` + input1 + `]}`)
	nonInputHash, _, nonInputFields := openAIWSNonInputFingerprint(payload)

	writer.BindSessionContext(7, 11, "sess_http_shared_ctx", openAIWSSessionContextValue{
		accountID:               101,
		connID:                  "http",
		lastResponseID:          "resp_http_shared_ctx",
		materializedHashes:      [][32]byte{mustItemHash(t, input1)},
		materializedCount:       1,
		inputCount:              1,
		inputOnlyContext:        true,
		nonInputHash:            nonInputHash,
		nonInputFields:          nonInputFields,
		rawVsClientVisibleEqual: true,
	}, time.Hour)

	restored, ok := reader.GetSessionContext(7, 11, "sess_http_shared_ctx")
	require.True(t, ok)
	require.Equal(t, "http", restored.connID, "HTTP transport marker is not a stale WS conn_id and must survive durable restore")
	require.Equal(t, "resp_http_shared_ctx", restored.lastResponseID)
	require.True(t, restored.inputOnlyContext)
}

func TestOpenAIWSStateStore_RestoredSharedHashContextDoesNotOutliveExplicitSharedDelete(t *testing.T) {
	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	reader := NewOpenAIWSStateStore(cache)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	writer.BindSessionContext(7, 11, "sess_restore_then_delete", openAIWSSessionContextValue{
		accountID:               101,
		lastResponseID:          "resp_restore_then_delete",
		materializedHashes:      [][32]byte{mustItemHash(t, input1)},
		materializedCount:       1,
		inputCount:              1,
		rawVsClientVisibleEqual: true,
	}, time.Hour)

	_, ok := reader.GetSessionContext(7, 11, "sess_restore_then_delete")
	require.True(t, ok)

	writer.DeleteSessionContext(7, 11, "sess_restore_then_delete")

	_, ok = reader.GetSessionContext(7, 11, "sess_restore_then_delete")
	require.False(t, ok, "a process that restored from shared cache must not keep using stale local durable context after explicit shared delete")
}

func TestOpenAIWSStateStore_LocalSessionContextDoesNotOutliveExplicitSharedDelete(t *testing.T) {
	cache := &stubGatewayCache{}
	owner := NewOpenAIWSStateStore(cache)
	deleter := NewOpenAIWSStateStore(cache)

	owner.BindSessionContext(7, 11, "sess_local_delete", openAIWSSessionContextValue{
		accountID:               101,
		connID:                  "oa_ws_local",
		lastResponseID:          "resp_local_delete",
		materializedHashes:      [][32]byte{mustItemHash(t, `{"type":"message","role":"user","content":"one"}`)},
		materializedCount:       1,
		inputCount:              1,
		rawVsClientVisibleEqual: true,
	}, time.Hour)

	cached, ok := owner.GetSessionContext(7, 11, "sess_local_delete")
	require.True(t, ok)
	require.Equal(t, "oa_ws_local", cached.connID)

	deleter.DeleteSessionContext(7, 11, "sess_local_delete")

	_, ok = owner.GetSessionContext(7, 11, "sess_local_delete")
	require.False(t, ok, "local durable context must be invalidated when another process deletes the shared session context")
}

func TestOpenAIWSStateStore_DeleteSessionContextDeletesSharedHashCache(t *testing.T) {
	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	reader := NewOpenAIWSStateStore(cache)

	writer.BindSessionContext(7, 11, "sess_delete_shared_ctx", openAIWSSessionContextValue{
		accountID:               101,
		lastResponseID:          "resp_delete_shared_ctx",
		materializedHashes:      [][32]byte{mustItemHash(t, `{"type":"message","role":"user","content":"one"}`)},
		materializedCount:       1,
		inputCount:              1,
		rawVsClientVisibleEqual: true,
	}, time.Hour)

	writer.DeleteSessionContext(7, 11, "sess_delete_shared_ctx")

	_, ok := reader.GetSessionContext(7, 11, "sess_delete_shared_ctx")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_SessionIdleEvictKeepsSharedHashContextForReanchor(t *testing.T) {
	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	reader := NewOpenAIWSStateStore(cache)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	payload := []byte(`{"model":"gpt-5.5","store":false,"input":[` + input1 + `]}`)
	nonInputHash, _, nonInputFields := openAIWSNonInputFingerprint(payload)

	writer.BindSessionContext(7, 11, "sess_idle_shared_ctx", openAIWSSessionContextValue{
		accountID:               101,
		connID:                  "oa_ws_idle_old",
		lastResponseID:          "resp_idle_shared_ctx",
		materializedHashes:      [][32]byte{mustItemHash(t, input1)},
		materializedShapes:      []string{"type=message;paths=$.content[]"},
		materializedCount:       1,
		inputCount:              1,
		nonInputHash:            nonInputHash,
		nonInputFields:          nonInputFields,
		rawVsClientVisibleEqual: true,
	}, time.Hour)

	writer.DeleteConnScopedState("oa_ws_idle_old", "session_idle_ttl")

	restored, ok := reader.GetSessionContext(7, 11, "sess_idle_shared_ctx")
	require.True(t, ok, "idle WS destruction should keep hash-only context so the next turn can reanchor on last_response_id")
	require.Empty(t, restored.connID, "restored durable context must not trust the evicted conn_id")
	require.Equal(t, "resp_idle_shared_ctx", restored.lastResponseID)
	require.Equal(t, nonInputHash, restored.nonInputHash)
	require.Contains(t, restored.nonInputFields, "model")
}

func TestOpenAIWSStateStore_DeleteConnScopedStateWriteFailNoReanchorDeletesSessionContext(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindSessionContext(7, 11, "sess_write_fail_no_reanchor", openAIWSSessionContextValue{
		accountID:      101,
		connID:         "conn_write_fail_no_reanchor",
		lastResponseID: "resp_write_fail_no_reanchor",
	}, time.Minute)
	store.BindConnLastResponse("conn_write_fail_no_reanchor", "resp_write_fail_no_reanchor", time.Minute)

	store.DeleteConnScopedState("conn_write_fail_no_reanchor", "write_request_fail_no_reanchor")

	_, ok := store.GetSessionContext(7, 11, "sess_write_fail_no_reanchor")
	require.False(t, ok, "write-failed request that cannot reanchor must not leave stale session context")
	_, ok = store.GetConnLastResponse("conn_write_fail_no_reanchor")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_DeleteConnScopedStateErrEventDeletesMatchingSessionContext(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindSessionContext(7, 11, "sess_err_event", openAIWSSessionContextValue{
		accountID:      101,
		connID:         "conn_err_event",
		lastResponseID: "resp_err_event",
	}, time.Minute)
	store.BindConnLastResponse("conn_err_event", "resp_err_event", time.Minute)

	store.DeleteConnScopedState("conn_err_event", "err_event")

	_, ok := store.GetSessionContext(7, 11, "sess_err_event")
	require.False(t, ok, "error-event conn eviction must not leave stale session context")
	_, ok = store.GetConnLastResponse("conn_err_event")
	require.False(t, ok)
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

func TestOpenAIWSStateStore_MaybeCleanupExpiresSessionInFlight(t *testing.T) {
	raw := NewOpenAIWSStateStore(nil)
	store, ok := raw.(*defaultOpenAIWSStateStore)
	require.True(t, ok)

	key := openAIWSSessionContextKey(7, 11, "sess_stale_inflight")
	require.NotEmpty(t, key)

	store.sessionInFlightMu.Lock()
	store.sessionInFlight[key] = openAIWSSessionInFlightBinding{
		expiresAt: time.Now().Add(-time.Minute),
	}
	store.sessionInFlightMu.Unlock()

	require.True(t, store.TrySessionInFlight(7, 11, "sess_stale_inflight"), "过期 owner 不应继续挡住新请求")
	store.EndSessionInFlight(7, 11, "sess_stale_inflight")

	store.sessionInFlightMu.Lock()
	store.sessionInFlight[key] = openAIWSSessionInFlightBinding{
		expiresAt: time.Now().Add(-time.Minute),
	}
	store.sessionInFlightMu.Unlock()

	store.lastCleanupUnixNano.Store(time.Now().Add(-2 * openAIWSStateStoreCleanupInterval).UnixNano())
	store.maybeCleanup()

	store.sessionInFlightMu.Lock()
	_, exists := store.sessionInFlight[key]
	store.sessionInFlightMu.Unlock()
	require.False(t, exists, "cleanup 后应清掉陈旧 in-flight owner")
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
