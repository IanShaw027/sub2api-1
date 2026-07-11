//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBillingSubscriptionGenerationFenceRejectsStaleFill(t *testing.T) {
	cache, _ := newMiniRedisCache(t)
	ctx := context.Background()
	const (
		userID  int64 = 81
		groupID int64 = 19
	)

	generation, err := cache.GetSubscriptionCacheGeneration(ctx, userID, groupID)
	require.NoError(t, err)
	require.Zero(t, generation)

	oldData := &service.SubscriptionCacheData{
		Status:    service.SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(time.Hour),
		Version:   1,
	}
	written, err := cache.SetSubscriptionCacheIfGeneration(ctx, userID, groupID, oldData, generation)
	require.NoError(t, err)
	require.True(t, written)

	require.NoError(t, cache.InvalidateSubscriptionCache(ctx, userID, groupID))
	newGeneration, err := cache.GetSubscriptionCacheGeneration(ctx, userID, groupID)
	require.NoError(t, err)
	require.Equal(t, generation+1, newGeneration)

	written, err = cache.SetSubscriptionCacheIfGeneration(ctx, userID, groupID, oldData, generation)
	require.NoError(t, err)
	require.False(t, written, "a pre-invalidation snapshot must not recreate the cache")
	_, err = cache.GetSubscriptionCache(ctx, userID, groupID)
	require.ErrorIs(t, err, redis.Nil)

	newData := &service.SubscriptionCacheData{
		Status:    service.SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(2 * time.Hour),
		Version:   2,
	}
	written, err = cache.SetSubscriptionCacheIfGeneration(ctx, userID, groupID, newData, newGeneration)
	require.NoError(t, err)
	require.True(t, written)
	got, err := cache.GetSubscriptionCache(ctx, userID, groupID)
	require.NoError(t, err)
	require.Equal(t, int64(2), got.Version)
}
