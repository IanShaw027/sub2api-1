//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCacheUnlockBucketValidatesToken(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	bucket := service.SchedulerBucket{GroupID: 9, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	lockCtx1 := service.WithSchedulerBucketLockTokenCarrier(ctx)
	ok, err := cache.TryLockBucket(lockCtx1, bucket, 50*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)

	time.Sleep(80 * time.Millisecond)

	lockCtx2 := service.WithSchedulerBucketLockTokenCarrier(ctx)
	ok, err = cache.TryLockBucket(lockCtx2, bucket, time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, cache.UnlockBucket(lockCtx1, bucket))

	lockKey := schedulerBucketKey(schedulerLockPrefix, bucket)
	lockVal, err := rdb.Get(ctx, lockKey).Result()
	require.NoError(t, err)
	require.Equal(t, service.SchedulerBucketLockToken(lockCtx2), lockVal)

	lockCtx3 := service.WithSchedulerBucketLockTokenCarrier(ctx)
	ok, err = cache.TryLockBucket(lockCtx3, bucket, time.Second)
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, cache.UnlockBucket(lockCtx2, bucket))

	ok, err = cache.TryLockBucket(lockCtx3, bucket, time.Second)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestSchedulerCacheSetSnapshotLosingCASDoesNotPolluteSharedAccountCache(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	bucket := service.SchedulerBucket{GroupID: 7, Platform: service.PlatformAnthropic, Mode: service.SchedulerModeSingle}
	account := service.Account{
		ID:       501,
		Name:     "stale-rebuild",
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"api_key": "stale-key",
		},
	}
	current := &service.Account{
		ID:       account.ID,
		Name:     "current-cache",
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"api_key": "current-key",
		},
	}
	require.NoError(t, cache.SetAccount(ctx, current))

	activeKey := schedulerBucketKey(schedulerActivePrefix, bucket)
	versionKey := schedulerBucketKey(schedulerVersionPrefix, bucket)
	require.NoError(t, rdb.Set(ctx, activeKey, "5", 0).Err())
	require.NoError(t, rdb.Set(ctx, versionKey, "0", 0).Err())

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{account}))

	got, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "current-key", got.GetCredential("api_key"))

	version, err := rdb.Get(ctx, activeKey).Result()
	require.NoError(t, err)
	require.Equal(t, "5", version)

	exists, err := rdb.Exists(ctx, schedulerSnapshotKey(bucket, "1")).Result()
	require.NoError(t, err)
	require.Zero(t, exists)
}

func TestSchedulerCacheSetSnapshotChunkCASWindowDoesNotOverwriteNewerSharedAccountCache(t *testing.T) {
	ctx := context.Background()
	bucket := service.SchedulerBucket{GroupID: 17, Platform: service.PlatformAnthropic, Mode: service.SchedulerModeSingle}
	activeKey := schedulerBucketKey(schedulerActivePrefix, bucket)
	versionKey := schedulerBucketKey(schedulerVersionPrefix, bucket)

	staleAccounts := []service.Account{
		{
			ID:       701,
			Name:     "stale-first",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeAPIKey,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"api_key": "stale-first-key",
			},
		},
		{
			ID:       702,
			Name:     "stale-second",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeAPIKey,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"api_key": "stale-second-key",
			},
		},
	}
	currentAccounts := []service.Account{
		{
			ID:       701,
			Name:     "current-first",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeAPIKey,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"api_key": "current-first-key",
			},
		},
		{
			ID:       702,
			Name:     "current-second",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeAPIKey,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"api_key": "current-second-key",
			},
		},
	}

	var mutatorErr error
	rdb := testRedis(t)
	cache := newSchedulerCacheWithChunkSizes(rdb, defaultSchedulerSnapshotMGetChunkSize, 1)
	hookCalls := 0
	prevHook := schedulerBeforeAccountChunkWriteHook
	schedulerBeforeAccountChunkWriteHook = func(ctx context.Context, hookActiveKey, version string) {
		hookCalls++
		if hookCalls != 2 {
			return
		}
		pipe := rdb.Pipeline()
		pipe.Set(ctx, hookActiveKey, "2", 0)
		pipe.Set(ctx, versionKey, "2", 0)
		for _, account := range currentAccounts {
			fullPayload, err := json.Marshal(account)
			if err != nil {
				mutatorErr = err
				return
			}
			metaPayload, err := json.Marshal(buildSchedulerMetadataAccount(account))
			if err != nil {
				mutatorErr = err
				return
			}
			id := strconv.FormatInt(account.ID, 10)
			pipe.Set(ctx, schedulerAccountKey(id), fullPayload, 0)
			pipe.Set(ctx, schedulerAccountMetaKey(id), metaPayload, 0)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			mutatorErr = err
		}
	}
	t.Cleanup(func() {
		schedulerBeforeAccountChunkWriteHook = prevHook
	})

	require.NoError(t, rdb.Set(ctx, activeKey, "0", 0).Err())
	require.NoError(t, rdb.Set(ctx, versionKey, "0", 0).Err())
	require.NoError(t, cache.SetSnapshot(ctx, bucket, staleAccounts))
	require.NoError(t, mutatorErr)
	require.Equal(t, 2, hookCalls)

	activeVersion, err := rdb.Get(ctx, activeKey).Result()
	require.NoError(t, err)
	require.Equal(t, "2", activeVersion)

	first, err := cache.GetAccount(ctx, staleAccounts[0].ID)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, "current-first-key", first.GetCredential("api_key"))

	second, err := cache.GetAccount(ctx, staleAccounts[1].ID)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, "current-second-key", second.GetCredential("api_key"))
}

func TestSchedulerCacheGetSnapshotReflectsUpdateLastUsedHotPath(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	bucket := service.SchedulerBucket{GroupID: 11, Platform: service.PlatformGemini, Mode: service.SchedulerModeSingle}
	account := service.Account{
		ID:          808,
		Name:        "snapshot-account",
		Platform:    service.PlatformGemini,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Schedulable: true,
	}

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{account}))
	usedAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, cache.UpdateLastUsed(ctx, map[int64]time.Time{account.ID: usedAt}))

	snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, snapshot, 1)
	require.Equal(t, "snapshot-account", snapshot[0].Name)
	require.NotNil(t, snapshot[0].LastUsedAt)
	require.WithinDuration(t, usedAt, *snapshot[0].LastUsedAt, time.Second)
}

func TestSchedulerCacheGetAccountsReflectsUpdateLastUsedHotPath(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb).(*schedulerCache)

	stale := time.Now().Add(-3 * time.Hour).UTC().Truncate(time.Second)
	fresh := time.Now().Add(-5 * time.Minute).UTC().Truncate(time.Second)
	account := service.Account{
		ID:          1808,
		Name:        "batch-account",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		LastUsedAt:  &stale,
	}

	require.NoError(t, cache.SetAccount(ctx, &account))
	require.NoError(t, cache.UpdateLastUsed(ctx, map[int64]time.Time{account.ID: fresh}))

	got, err := cache.GetAccounts(ctx, []int64{account.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[account.ID])
	require.NotNil(t, got[account.ID].LastUsedAt)
	require.True(t, got[account.ID].LastUsedAt.Equal(fresh))
}

func TestSchedulerCacheUpdateLastUsedUsesOverlayWithoutRewritingAccountState(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	oldUsedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	account := &service.Account{
		ID:          809,
		Name:        "overlay-account",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		LastUsedAt:  &oldUsedAt,
		Credentials: map[string]any{
			"api_key": "state-key",
		},
	}
	require.NoError(t, cache.SetAccount(ctx, account))

	id := strconv.FormatInt(account.ID, 10)
	fullBefore, err := rdb.Get(ctx, schedulerAccountKey(id)).Result()
	require.NoError(t, err)
	metaBefore, err := rdb.Get(ctx, schedulerAccountMetaKey(id)).Result()
	require.NoError(t, err)

	usedAt := oldUsedAt.Add(30 * time.Minute)
	require.NoError(t, cache.UpdateLastUsed(ctx, map[int64]time.Time{account.ID: usedAt}))

	fullAfter, err := rdb.Get(ctx, schedulerAccountKey(id)).Result()
	require.NoError(t, err)
	metaAfter, err := rdb.Get(ctx, schedulerAccountMetaKey(id)).Result()
	require.NoError(t, err)
	require.Equal(t, fullBefore, fullAfter)
	require.Equal(t, metaBefore, metaAfter)

	overlayValue, err := rdb.HGet(ctx, schedulerLastUsedOverlayKey, id).Result()
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(usedAt.UnixNano(), 10), overlayValue)
	lastUsedReader := cache.(interface {
		GetLastUsed(context.Context, []int64) (map[int64]time.Time, error)
	})
	lastUsed, err := lastUsedReader.GetLastUsed(ctx, []int64{account.ID})
	require.NoError(t, err)
	require.Equal(t, usedAt.UnixNano(), lastUsed[account.ID].UnixNano())

	changed := *account
	changed.Status = service.StatusDisabled
	changed.Schedulable = false
	changed.Credentials = map[string]any{"api_key": "changed-key"}
	require.NoError(t, cache.SetAccount(ctx, &changed))

	got, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, service.StatusDisabled, got.Status)
	require.False(t, got.Schedulable)
	require.Equal(t, "changed-key", got.GetCredential("api_key"))
	require.NotNil(t, got.LastUsedAt)
	require.Equal(t, usedAt.UnixNano(), got.LastUsedAt.UnixNano())
}

func TestSchedulerCacheUpdateLastUsedOverlayKeepsNewestAcrossConcurrentUpdates(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	bucket := service.SchedulerBucket{GroupID: 12, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	account := service.Account{
		ID:          810,
		Name:        "concurrent-overlay-account",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
	}
	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{account}))

	base := time.Now().UTC().Truncate(time.Second)
	newest := base.Add(63 * time.Second)
	var wg sync.WaitGroup
	errCh := make(chan error, 64)
	for i := range 64 {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- cache.UpdateLastUsed(ctx, map[int64]time.Time{
				account.ID: base.Add(time.Duration(i) * time.Second),
			})
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	require.NoError(t, cache.UpdateLastUsed(ctx, map[int64]time.Time{account.ID: base.Add(2 * time.Second)}))

	got, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.LastUsedAt)
	require.Equal(t, newest.UnixNano(), got.LastUsedAt.UnixNano())

	snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, snapshot, 1)
	require.NotNil(t, snapshot[0].LastUsedAt)
	require.Equal(t, newest.UnixNano(), snapshot[0].LastUsedAt.UnixNano())
}

func TestSchedulerCacheSnapshotUsesSlimMetadataButKeepsFullAccount(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	bucket := service.SchedulerBucket{GroupID: 2, Platform: service.PlatformGemini, Mode: service.SchedulerModeSingle}
	now := time.Now().UTC().Truncate(time.Second)
	limitReset := now.Add(10 * time.Minute)
	overloadUntil := now.Add(2 * time.Minute)
	tempUnschedUntil := now.Add(3 * time.Minute)
	windowEnd := now.Add(5 * time.Hour)

	account := service.Account{
		ID:          101,
		Name:        "gemini-heavy",
		Platform:    service.PlatformGemini,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 3,
		Priority:    7,
		LastUsedAt:  &now,
		Credentials: map[string]any{
			"api_key":       "gemini-api-key",
			"access_token":  "secret-access-token",
			"project_id":    "proj-1",
			"oauth_type":    "ai_studio",
			"model_mapping": map[string]any{"gemini-2.5-pro": "gemini-2.5-pro"},
			"huge_blob":     strings.Repeat("x", 4096),
		},
		Extra: map[string]any{
			"mixed_scheduling":             true,
			"window_cost_limit":            12.5,
			"window_cost_sticky_reserve":   8.0,
			"max_sessions":                 4,
			"session_idle_timeout_minutes": 11,
			"unused_large_field":           strings.Repeat("y", 4096),
		},
		RateLimitResetAt:       &limitReset,
		OverloadUntil:          &overloadUntil,
		TempUnschedulableUntil: &tempUnschedUntil,
		SessionWindowStart:     &now,
		SessionWindowEnd:       &windowEnd,
		SessionWindowStatus:    "active",
		GroupIDs:               []int64{bucket.GroupID},
		AccountGroups: []service.AccountGroup{
			{
				AccountID: 101,
				GroupID:   bucket.GroupID,
				Priority:  5,
				Group:     &service.Group{ID: bucket.GroupID, Name: "gemini-group"},
			},
		},
	}

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{account}))

	snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, snapshot, 1)

	got := snapshot[0]
	require.NotNil(t, got)
	require.Equal(t, "gemini-api-key", got.GetCredential("api_key"))
	require.Equal(t, "proj-1", got.GetCredential("project_id"))
	require.Equal(t, "ai_studio", got.GetCredential("oauth_type"))
	require.NotEmpty(t, got.GetModelMapping())
	require.Empty(t, got.GetCredential("access_token"))
	require.Empty(t, got.GetCredential("huge_blob"))
	require.Equal(t, true, got.Extra["mixed_scheduling"])
	require.Equal(t, 12.5, got.GetWindowCostLimit())
	require.Equal(t, 8.0, got.GetWindowCostStickyReserve())
	require.Equal(t, 4, got.GetMaxSessions())
	require.Equal(t, 11, got.GetSessionIdleTimeoutMinutes())
	require.Nil(t, got.Extra["unused_large_field"])
	require.Equal(t, []int64{bucket.GroupID}, got.GroupIDs)
	require.Len(t, got.AccountGroups, 1)
	require.Equal(t, account.ID, got.AccountGroups[0].AccountID)
	require.Equal(t, bucket.GroupID, got.AccountGroups[0].GroupID)
	require.Nil(t, got.AccountGroups[0].Group)

	full, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, full)
	require.Equal(t, "secret-access-token", full.GetCredential("access_token"))
	require.Equal(t, strings.Repeat("x", 4096), full.GetCredential("huge_blob"))
	require.Len(t, full.AccountGroups, 1)
	require.NotNil(t, full.AccountGroups[0].Group)
}
