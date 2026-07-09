package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeMaterializedHashes(n int) [][32]byte {
	out := make([][32]byte, n)
	for i := range out {
		out[i][0] = byte(i)
		out[i][1] = byte(i >> 8)
	}
	return out
}

// TestOpenAIWSStateStore_OversizedSessionContextDropsDeltaFingerprint 超过 materialized 上限的会话上下文
// 落盘时丢弃 delta 指纹、置 deltaFingerprintOmitted，但保留 account 亲和；封顶后 Redis/内存值有界。
func TestOpenAIWSStateStore_OversizedSessionContextDropsDeltaFingerprint(t *testing.T) {
	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	reader := NewOpenAIWSStateStore(cache)

	oversized := openAIWSSessionContextMaxMaterializedItems + 1
	writer.BindSessionContext(7, 11, "sess_oversized", openAIWSSessionContextValue{
		accountID:               101,
		connID:                  "http",
		lastResponseID:          "resp_oversized_1",
		materializedHashes:      makeMaterializedHashes(oversized),
		materializedShapes:      make([]string, oversized),
		materializedCount:       oversized,
		inputCount:              oversized,
		inputOnlyContext:        true,
		rawVsClientVisibleEqual: true,
	}, time.Hour)

	// 本进程内读取：亲和保留、指纹已丢弃并置标志。
	local, ok := writer.GetSessionContext(7, 11, "sess_oversized")
	require.True(t, ok)
	require.Equal(t, int64(101), local.accountID)
	require.Equal(t, "resp_oversized_1", local.lastResponseID)
	require.True(t, local.deltaFingerprintOmitted)
	require.Empty(t, local.materializedHashes)
	require.Zero(t, local.materializedCount)

	// 经 Redis 往返(另一进程)读取：标志与亲和持久化，wsctx value 不含大数组。
	restored, ok := reader.GetSessionContext(7, 11, "sess_oversized")
	require.True(t, ok)
	require.Equal(t, int64(101), restored.accountID)
	require.Equal(t, "resp_oversized_1", restored.lastResponseID)
	require.True(t, restored.deltaFingerprintOmitted)
	require.Empty(t, restored.materializedHashes)
}

// TestOpenAIWSStateStore_UnderCapSessionContextKeepsFingerprint 未超限的会话上下文保留 delta 指纹，标志为假。
func TestOpenAIWSStateStore_UnderCapSessionContextKeepsFingerprint(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindSessionContext(7, 11, "sess_small", openAIWSSessionContextValue{
		accountID:          101,
		connID:             "oa_ws_101",
		lastResponseID:     "resp_small_1",
		materializedHashes: makeMaterializedHashes(3),
		materializedCount:  3,
		inputCount:         3,
	}, time.Hour)

	got, ok := store.GetSessionContext(7, 11, "sess_small")
	require.True(t, ok)
	require.False(t, got.deltaFingerprintOmitted)
	require.Len(t, got.materializedHashes, 3)
	require.Equal(t, 3, got.materializedCount)
}

// TestBoundOpenAIWSSessionContextValue_Boundary 恰好等于上限不丢弃、超过一项即丢弃。
func TestBoundOpenAIWSSessionContextValue_Boundary(t *testing.T) {
	atCap := openAIWSSessionContextValue{materializedHashes: makeMaterializedHashes(openAIWSSessionContextMaxMaterializedItems), materializedCount: openAIWSSessionContextMaxMaterializedItems}
	kept := boundOpenAIWSSessionContextValue(atCap)
	require.False(t, kept.deltaFingerprintOmitted)
	require.Len(t, kept.materializedHashes, openAIWSSessionContextMaxMaterializedItems)

	over := openAIWSSessionContextValue{materializedHashes: makeMaterializedHashes(openAIWSSessionContextMaxMaterializedItems + 1), materializedCount: openAIWSSessionContextMaxMaterializedItems + 1}
	dropped := boundOpenAIWSSessionContextValue(over)
	require.True(t, dropped.deltaFingerprintOmitted)
	require.Empty(t, dropped.materializedHashes)
	require.Zero(t, dropped.materializedCount)
}
