//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTempUnschedCounterCacheTest(t *testing.T) (*tempUnschedCounterCache, *redis.Client) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	return &tempUnschedCounterCache{rdb: rdb}, rdb
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
