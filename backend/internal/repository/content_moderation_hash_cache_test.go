//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
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

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "hash-delete", service.ContentModerationHashMeta{}))
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

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "hash-clear-a", service.ContentModerationHashMeta{}))
	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "hash-clear-b", service.ContentModerationHashMeta{}))
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

func TestContentModerationHashCacheListTracksHitCountsAndSorts(t *testing.T) {
	cache, mr := newContentModerationHashCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "hash-a", service.ContentModerationHashMeta{}))
	mr.FastForward(24 * time.Hour)
	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "hash-b", service.ContentModerationHashMeta{}))

	matched, err := cache.HasFlaggedInputHash(ctx, "hash-a")
	require.NoError(t, err)
	require.True(t, matched)
	matched, err = cache.HasFlaggedInputHash(ctx, "hash-a")
	require.NoError(t, err)
	require.True(t, matched)
	matched, err = cache.HasFlaggedInputHash(ctx, "hash-b")
	require.NoError(t, err)
	require.True(t, matched)

	items, page, err := cache.ListFlaggedInputHashes(ctx, service.ContentModerationHashListFilter{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 1, SortBy: service.ContentModerationHashSortHits7D, SortOrder: pagination.SortOrderDesc},
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	require.Equal(t, 1, page.Page)
	require.Len(t, items, 1)
	require.Equal(t, "hash-a", items[0].InputHash)
	require.EqualValues(t, 2, items[0].HitCount7D)
	require.EqualValues(t, 2, items[0].HitCount30D)
	require.False(t, items[0].CreatedAt.IsZero())
	require.True(t, items[0].ExpiresAt.After(items[0].CreatedAt))

	items, page, err = cache.ListFlaggedInputHashes(ctx, service.ContentModerationHashListFilter{
		Pagination: pagination.PaginationParams{Page: 2, PageSize: 1, SortBy: service.ContentModerationHashSortHits7D, SortOrder: pagination.SortOrderDesc},
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	require.Len(t, items, 1)
	require.Equal(t, "hash-b", items[0].InputHash)
	require.EqualValues(t, 1, items[0].HitCount7D)
	require.EqualValues(t, 1, items[0].HitCount30D)
}

func TestSortFlaggedHashItemsUsesStrictTieBreakerForDescendingHitSorts(t *testing.T) {
	created := time.Unix(1_700_000_000, 0)
	items := []service.ContentModerationHashItem{
		{InputHash: "hash-c", CreatedAt: created, HitCount7D: 0, HitCount30D: 0},
		{InputHash: "hash-a", CreatedAt: created, HitCount7D: 0, HitCount30D: 0},
		{InputHash: "hash-b", CreatedAt: created, HitCount7D: 0, HitCount30D: 0},
	}

	sortFlaggedHashItems(items, service.ContentModerationHashSortHits7D, pagination.SortOrderDesc)

	require.Equal(t, []string{"hash-c", "hash-b", "hash-a"}, []string{
		items[0].InputHash,
		items[1].InputHash,
		items[2].InputHash,
	})
}

func TestContentModerationHashCacheExpiresAfter90DaysAndPrunesStaleSetMember(t *testing.T) {
	cache, mr := newContentModerationHashCacheTest(t)
	ctx := context.Background()
	base := time.Unix(1_700_000_000, 0)
	mr.SetTime(base)

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "expires", service.ContentModerationHashMeta{}))
	require.True(t, mr.Exists(contentModerationFlaggedHashKeyPrefix+"expires"))

	mr.FastForward(90*24*time.Hour + time.Second)

	matched, err := cache.HasFlaggedInputHash(ctx, "expires")
	require.NoError(t, err)
	require.False(t, matched)

	count, err := cache.CountFlaggedInputHashes(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestContentModerationHashCacheHitMetricFailureDoesNotDowngradeMatch(t *testing.T) {
	cache, mr := newContentModerationHashCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "metric-fail", service.ContentModerationHashMeta{}))
	mr.Set(contentModerationFlaggedHashHitsKeyPrefix+"metric-fail", "not-a-zset")

	matched, err := cache.HasFlaggedInputHash(ctx, "metric-fail")
	require.NoError(t, err)
	require.True(t, matched)
}

func TestContentModerationHashCacheBatchDeleteRemovesMetadataAndHits(t *testing.T) {
	cache, _ := newContentModerationHashCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "delete-a", service.ContentModerationHashMeta{}))
	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "delete-b", service.ContentModerationHashMeta{}))
	matched, err := cache.HasFlaggedInputHash(ctx, "delete-a")
	require.NoError(t, err)
	require.True(t, matched)

	deleted, err := cache.DeleteFlaggedInputHashes(ctx, []string{"delete-a", "delete-b", "missing"})
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)

	items, page, err := cache.ListFlaggedInputHashes(ctx, service.ContentModerationHashListFilter{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 20},
	})
	require.NoError(t, err)
	require.EqualValues(t, 0, page.Total)
	require.Empty(t, items)
}
