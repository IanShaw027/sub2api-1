//go:build unit

package repository

import (
	"context"
	"strconv"
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

func TestContentModerationHashCacheRecordDoesNotWriteLegacySet(t *testing.T) {
	cache, mr := newContentModerationHashCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "no-legacy-set", service.ContentModerationHashMeta{}))

	require.False(t, mr.Exists(contentModerationFlaggedHashSetKey))
	require.True(t, mr.Exists(contentModerationFlaggedHashKeyPrefix+"no-legacy-set"))
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

func TestContentModerationHashCacheWorksWithoutLegacySetIndex(t *testing.T) {
	cache, mr := newContentModerationHashCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "legacyless-a", service.ContentModerationHashMeta{}))
	require.NoError(t, cache.RecordFlaggedInputHash(ctx, "legacyless-b", service.ContentModerationHashMeta{}))
	mr.Del(contentModerationFlaggedHashSetKey)

	count, err := cache.CountFlaggedInputHashes(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)

	items, page, err := cache.ListFlaggedInputHashes(ctx, service.ContentModerationHashListFilter{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 20, SortBy: service.ContentModerationHashSortHits7D},
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	require.Len(t, items, 2)

	deleted, err := cache.DeleteFlaggedInputHash(ctx, "legacyless-a")
	require.NoError(t, err)
	require.True(t, deleted)

	deletedCount, err := cache.ClearFlaggedInputHashes(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, deletedCount)

	count, err = cache.CountFlaggedInputHashes(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
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
	// 把命中计数 key 预置为 string，令 HINCRBY 触发 WRONGTYPE，模拟命中计数写入失败：
	// 匹配结果不得因指标写入失败而降级。
	mr.Set(contentModerationFlaggedHashHitCountKeyPrefix+"metric-fail", "not-a-hash")

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

// TestContentModerationHashCacheBatchingSpansMultipleChunks 覆盖超过单次 pipeline 分片上限的场景：
// list 批量取项与批量删除都需跨多个 pipeline 分片正确工作。
func TestContentModerationHashCacheBatchingSpansMultipleChunks(t *testing.T) {
	cache, _ := newContentModerationHashCacheTest(t)
	ctx := context.Background()

	total := flaggedHashPipelineChunkSize + 120 // 跨 2 个分片
	all := make([]string, 0, total)
	for i := 0; i < total; i++ {
		h := "chunk-hash-" + strconv.Itoa(i)
		all = append(all, h)
		require.NoError(t, cache.RecordFlaggedInputHash(ctx, h, service.ContentModerationHashMeta{}))
	}

	// 用 hits 排序走全量扫描路径，覆盖 list 批量取项跨多个 pipeline 分片的场景
	// （created_at 排序走 ZSet 游标，每页仅取 pageSize 条不跨分片）。
	_, page, err := cache.ListFlaggedInputHashes(ctx, service.ContentModerationHashListFilter{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 20, SortBy: service.ContentModerationHashSortHits7D},
	})
	require.NoError(t, err)
	require.EqualValues(t, total, page.Total)

	deleted, err := cache.DeleteFlaggedInputHashes(ctx, all)
	require.NoError(t, err)
	require.EqualValues(t, total, deleted)

	count, err := cache.CountFlaggedInputHashes(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

// TestContentModerationHashCacheCreatedAtCursorPagination 覆盖 created_at 排序的 ZSet 游标分页：
// 按 created_at 逆序跨页取，且每页只拉当页详情、total 由 ZCard 得出。
func TestContentModerationHashCacheCreatedAtCursorPagination(t *testing.T) {
	cache, mr := newContentModerationHashCacheTest(t)
	ctx := context.Background()
	mr.SetTime(time.Unix(1_700_000_000, 0))

	// 依次记录 5 条，每条相隔 1 天，使 created_at 严格递增（h0 最早、h4 最新）。
	for i := 0; i < 5; i++ {
		require.NoError(t, cache.RecordFlaggedInputHash(ctx, "cursor-h"+strconv.Itoa(i), service.ContentModerationHashMeta{}))
		mr.FastForward(24 * time.Hour)
	}

	// 默认 created_at 逆序：第 1 页应为最新的 h4、h3。
	items, page, err := cache.ListFlaggedInputHashes(ctx, service.ContentModerationHashListFilter{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 2, SortOrder: pagination.SortOrderDesc},
	})
	require.NoError(t, err)
	require.EqualValues(t, 5, page.Total)
	require.EqualValues(t, 3, page.Pages)
	require.Len(t, items, 2)
	require.Equal(t, "cursor-h4", items[0].InputHash)
	require.Equal(t, "cursor-h3", items[1].InputHash)

	// 第 3 页只剩最早的 h0。
	items, _, err = cache.ListFlaggedInputHashes(ctx, service.ContentModerationHashListFilter{
		Pagination: pagination.PaginationParams{Page: 3, PageSize: 2, SortOrder: pagination.SortOrderDesc},
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "cursor-h0", items[0].InputHash)

	// 升序：第 1 页应为最早的 h0、h1。
	items, _, err = cache.ListFlaggedInputHashes(ctx, service.ContentModerationHashListFilter{
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 2, SortOrder: pagination.SortOrderAsc},
	})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "cursor-h0", items[0].InputHash)
	require.Equal(t, "cursor-h1", items[1].InputHash)
}
