//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestUserRPMCache(t *testing.T) *userRPMCacheImpl {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &userRPMCacheImpl{rdb: rdb}
}

func TestTryIncrementUserAndGroupRPM_AdmitsThenRejects(t *testing.T) {
	cache := newTestUserRPMCache(t)
	ctx := context.Background()

	groupCount, userCount, admitted, err := cache.TryIncrementUserAndGroupRPM(ctx, 10, 20, 2, 5, true, true)
	require.NoError(t, err)
	require.True(t, admitted)
	require.Equal(t, 1, groupCount)
	require.Equal(t, 1, userCount)

	groupCount, userCount, admitted, err = cache.TryIncrementUserAndGroupRPM(ctx, 10, 20, 2, 5, true, true)
	require.NoError(t, err)
	require.True(t, admitted)
	require.Equal(t, 2, groupCount)
	require.Equal(t, 2, userCount)

	groupCount, userCount, admitted, err = cache.TryIncrementUserAndGroupRPM(ctx, 10, 20, 2, 5, true, true)
	require.NoError(t, err)
	require.False(t, admitted)
	require.Equal(t, 2, groupCount)
	require.Equal(t, 2, userCount)
}

func TestTryIncrementUserAndGroupRPM_ParallelRespectsLimit(t *testing.T) {
	cache := newTestUserRPMCache(t)
	ctx := context.Background()
	const workers = 20
	const limit = 5
	admitted := make(chan bool, workers)
	for i := 0; i < workers; i++ {
		go func() {
			_, _, ok, err := cache.TryIncrementUserAndGroupRPM(ctx, 30, 40, limit, limit, true, true)
			if err != nil {
				admitted <- false
				return
			}
			admitted <- ok
		}()
	}
	accepted := 0
	for i := 0; i < workers; i++ {
		if <-admitted {
			accepted++
		}
	}
	require.Equal(t, limit, accepted)
}

func TestTryIncrementUserAndGroupRPM_UserLimitRejectsWithoutGroupIncr(t *testing.T) {
	cache := newTestUserRPMCache(t)
	ctx := context.Background()

	_, _, admitted, err := cache.TryIncrementUserAndGroupRPM(ctx, 11, 21, 5, 1, true, true)
	require.NoError(t, err)
	require.True(t, admitted)

	groupCount, userCount, admitted, err := cache.TryIncrementUserAndGroupRPM(ctx, 11, 21, 5, 1, true, true)
	require.NoError(t, err)
	require.False(t, admitted)
	require.Equal(t, 1, groupCount)
	require.Equal(t, 1, userCount)
}
