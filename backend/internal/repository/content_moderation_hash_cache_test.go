//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newContentModerationHashCacheTest(t *testing.T) (*contentModerationHashCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &contentModerationHashCache{rdb: rdb}, mr
}

func TestContentModerationHashCacheDeleteRemovesSetMemberAndPerHashKey(t *testing.T) {
	cache, mr := newContentModerationHashCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "hash-delete"))
	require.True(t, mr.Exists(contentModerationFlaggedHashKeyPrefix+"hash-delete"))

	deleted, err := cache.DeleteFlaggedInputHash(ctx, "hash-delete")
	require.NoError(t, err)
	require.True(t, deleted)

	matched, err := cache.HasFlaggedInputHash(ctx, "hash-delete")
	require.NoError(t, err)
	require.False(t, matched)
	require.False(t, mr.Exists(contentModerationFlaggedHashKeyPrefix+"hash-delete"))
}

func TestContentModerationHashCacheClearRemovesPerHashKeys(t *testing.T) {
	cache, mr := newContentModerationHashCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "hash-clear-a"))
	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "hash-clear-b"))
	require.True(t, mr.Exists(contentModerationFlaggedHashKeyPrefix+"hash-clear-a"))
	require.True(t, mr.Exists(contentModerationFlaggedHashKeyPrefix+"hash-clear-b"))

	deleted, err := cache.ClearFlaggedInputHashes(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)

	for _, inputHash := range []string{"hash-clear-a", "hash-clear-b"} {
		matched, err := cache.HasFlaggedInputHash(ctx, inputHash)
		require.NoError(t, err)
		require.False(t, matched)
		require.False(t, mr.Exists(contentModerationFlaggedHashKeyPrefix+inputHash))
	}
}
