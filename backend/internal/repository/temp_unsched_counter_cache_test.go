//go:build unit

package repository

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTempUnschedCounterCacheTest(t *testing.T) (*tempUnschedCounterCache, *redis.Client) {
	t.Helper()

	cache, rdb, _ := newTempUnschedCounterCacheMiniRedisTest(t)
	return cache, rdb
}

func newTempUnschedCounterCacheMiniRedisTest(t *testing.T) (*tempUnschedCounterCache, *redis.Client, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	return &tempUnschedCounterCache{rdb: rdb}, rdb, mr
}

type redisCommandRecorder struct {
	names []string
}

func (h *redisCommandRecorder) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *redisCommandRecorder) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		h.names = append(h.names, strings.ToLower(cmd.Name()))
		return next(ctx, cmd)
	}
}

func (h *redisCommandRecorder) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			h.names = append(h.names, strings.ToLower(cmd.Name()))
		}
		return next(ctx, cmds)
	}
}

func (h *redisCommandRecorder) saw(command string) bool {
	command = strings.ToLower(command)
	for _, name := range h.names {
		if name == command {
			return true
		}
	}
	return false
}

func TestTempUnschedCounterCache_IncrementSetsTTLAndResetDeletes(t *testing.T) {
	cache, rdb := newTempUnschedCounterCacheTest(t)
	ctx := context.Background()

	count, err := cache.IncrementTempUnschedCount(ctx, 42, "status=502;keywords=overloaded;duration=10", 2)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	count, err = cache.IncrementTempUnschedCount(ctx, 42, "status=502;keywords=overloaded;duration=10", 2)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)

	key := cache.counterKey(42, "status=502;keywords=overloaded;duration=10")
	stored, err := rdb.Get(ctx, key).Int64()
	require.NoError(t, err)
	require.EqualValues(t, 2, stored)

	ttl, err := rdb.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.True(t, ttl > 0 && ttl <= 2*time.Minute, "ttl = %s", ttl)

	require.NoError(t, cache.ResetTempUnschedCount(ctx, 42, "status=502;keywords=overloaded;duration=10"))
	exists, err := rdb.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.EqualValues(t, 0, exists)
}

func TestTempUnschedCounterCache_IncrementRefreshesTTLForSlidingWindow(t *testing.T) {
	cache, _, mr := newTempUnschedCounterCacheMiniRedisTest(t)
	ctx := context.Background()
	fingerprint := "status=502;keywords=overloaded;duration=10"
	key := cache.counterKey(42, fingerprint)

	count, err := cache.IncrementTempUnschedCount(ctx, 42, fingerprint, 2)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	mr.FastForward(70 * time.Second)

	count, err = cache.IncrementTempUnschedCount(ctx, 42, fingerprint, 2)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)

	ttl, err := cache.rdb.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, 90*time.Second, "second hit should refresh the sliding window ttl, ttl = %s", ttl)
}

func TestTempUnschedCounterCache_CountsAreIndependentPerRule(t *testing.T) {
	cache, _ := newTempUnschedCounterCacheTest(t)
	ctx := context.Background()

	count, err := cache.IncrementTempUnschedCount(ctx, 42, "status=502;keywords=overloaded;duration=10", 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	count, err = cache.IncrementTempUnschedCount(ctx, 42, "status=503;keywords=overloaded;duration=10", 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}

func TestTempUnschedCounterCache_IncrementThresholdResetsAtomically(t *testing.T) {
	cache, rdb := newTempUnschedCounterCacheTest(t)
	ctx := context.Background()
	fingerprint := "status=502;keywords=overloaded;duration=10"
	key := cache.counterKey(42, fingerprint)

	count, reached, err := cache.IncrementTempUnschedThreshold(ctx, 42, fingerprint, 1, 3)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.False(t, reached)

	count, reached, err = cache.IncrementTempUnschedThreshold(ctx, 42, fingerprint, 1, 3)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)
	require.False(t, reached)

	count, reached, err = cache.IncrementTempUnschedThreshold(ctx, 42, fingerprint, 1, 3)
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
	require.True(t, reached)

	exists, err := rdb.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.EqualValues(t, 0, exists, "threshold hit should reset the counter in the same Redis script")

	count, reached, err = cache.IncrementTempUnschedThreshold(ctx, 42, fingerprint, 1, 3)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.False(t, reached)
}

func TestTempUnschedCounterCache_ResetAlsoDeletesLegacyIndexKey(t *testing.T) {
	cache, rdb := newTempUnschedCounterCacheTest(t)
	ctx := context.Background()

	legacyKey := "temp_unsched_count:account:42:rule:3"
	newKey := cache.counterKey(42, "status=502;keywords=overloaded;duration=10")
	require.NoError(t, rdb.Set(ctx, legacyKey, 1, time.Minute).Err())
	require.NoError(t, rdb.Set(ctx, newKey, 1, time.Minute).Err())

	require.NoError(t, cache.ResetTempUnschedCount(ctx, 42, "status=502;keywords=overloaded;duration=10"))

	exists, err := rdb.Exists(ctx, legacyKey, newKey).Result()
	require.NoError(t, err)
	require.EqualValues(t, 0, exists)
}

func TestTempUnschedCounterCache_ExactResetDoesNotScanLegacyKeys(t *testing.T) {
	cache, rdb := newTempUnschedCounterCacheTest(t)
	ctx := context.Background()
	recorder := &redisCommandRecorder{}
	rdb.AddHook(recorder)

	legacyKey := "temp_unsched_count:account:42:rule:3"
	newKey := cache.counterKey(42, "kiro:first_forwardable_event_timeout")
	require.NoError(t, rdb.Set(ctx, legacyKey, 1, time.Minute).Err())
	require.NoError(t, rdb.Set(ctx, newKey, 1, time.Minute).Err())

	require.NoError(t, cache.ResetTempUnschedFingerprint(ctx, 42, "kiro:first_forwardable_event_timeout"))

	require.False(t, recorder.saw("scan"))
	newExists, err := rdb.Exists(ctx, newKey).Result()
	require.NoError(t, err)
	require.Zero(t, newExists)
	legacyExists, err := rdb.Exists(ctx, legacyKey).Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, legacyExists)
}

func TestTempUnschedCounterCache_ResetUsesScanForLegacyKeys(t *testing.T) {
	cache, rdb := newTempUnschedCounterCacheTest(t)
	ctx := context.Background()
	recorder := &redisCommandRecorder{}
	rdb.AddHook(recorder)

	legacyKey := "temp_unsched_count:account:42:rule:3"
	newKey := cache.counterKey(42, "status=502;keywords=overloaded;duration=10")
	require.NoError(t, rdb.Set(ctx, legacyKey, 1, time.Minute).Err())
	require.NoError(t, rdb.Set(ctx, newKey, 1, time.Minute).Err())

	require.NoError(t, cache.ResetTempUnschedCount(ctx, 42, "status=502;keywords=overloaded;duration=10"))

	require.False(t, recorder.saw("keys"), "ResetTempUnschedCount must not issue Redis KEYS")
	require.True(t, recorder.saw("scan"), "ResetTempUnschedCount should discover legacy keys with SCAN")
	exists, err := rdb.Exists(ctx, legacyKey, newKey).Result()
	require.NoError(t, err)
	require.EqualValues(t, 0, exists)
}
