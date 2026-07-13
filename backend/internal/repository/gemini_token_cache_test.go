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

func TestGeminiTokenCache_DeleteAccessToken_RedisError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	cache := NewGeminiTokenCache(rdb)
	err := cache.DeleteAccessToken(context.Background(), "broken")
	require.Error(t, err)
}

func TestGeminiTokenCache_ReleaseRefreshLockRequiresLeaseOwner(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	ctx := context.Background()
	cache := NewGeminiTokenCache(rdb)

	firstLease, locked, err := cache.AcquireRefreshLock(ctx, "account-1", time.Second)
	require.NoError(t, err)
	require.True(t, locked)
	require.NotEmpty(t, firstLease)

	mr.FastForward(2 * time.Second)

	secondLease, locked, err := cache.AcquireRefreshLock(ctx, "account-1", time.Second)
	require.NoError(t, err)
	require.True(t, locked)
	require.NotEmpty(t, secondLease)
	require.NotEqual(t, firstLease, secondLease)

	require.NoError(t, cache.ReleaseRefreshLock(ctx, "account-1", firstLease))

	_, locked, err = cache.AcquireRefreshLock(ctx, "account-1", time.Second)
	require.NoError(t, err)
	require.False(t, locked, "stale lease owner must not delete a newer lock")

	require.NoError(t, cache.ReleaseRefreshLock(ctx, "account-1", secondLease))
	_, locked, err = cache.AcquireRefreshLock(ctx, "account-1", time.Second)
	require.NoError(t, err)
	require.True(t, locked)
}
