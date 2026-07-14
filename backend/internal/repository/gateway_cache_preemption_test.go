//go:build unit

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newGatewayCachePreemptionTest(t *testing.T) (*gatewayCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &gatewayCache{rdb: rdb}, mr
}

func TestGatewayCacheClaimOpenAIResponsesSessionWindowConcurrentClaimsAreLinearized(t *testing.T) {
	cache, _ := newGatewayCachePreemptionTest(t)
	const contenders = 32
	start := make(chan struct{})
	previous := make(chan string, contenders)
	errs := make(chan error, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		owner := fmt.Sprintf("owner-%02d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			prior, err := cache.ClaimOpenAIResponsesSessionWindow(context.Background(), 7, "claim-race", []byte(owner), time.Minute)
			errs <- err
			previous <- string(prior)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	close(previous)

	for err := range errs {
		require.NoError(t, err)
	}
	emptyCount := 0
	seenPrevious := make(map[string]struct{}, contenders)
	for prior := range previous {
		if prior == "" {
			emptyCount++
			continue
		}
		_, duplicate := seenPrevious[prior]
		require.False(t, duplicate, "an atomic claim chain cannot return the same previous owner twice")
		seenPrevious[prior] = struct{}{}
	}
	require.Equal(t, 1, emptyCount, "exactly one contender must observe an unowned key")
	require.Len(t, seenPrevious, contenders-1)

	current, err := cache.GetOpenAIResponsesSessionWindow(context.Background(), 7, "claim-race")
	require.NoError(t, err)
	require.NotEmpty(t, current)
	_, wasPrevious := seenPrevious[string(current)]
	require.False(t, wasPrevious, "the final owner is the only claim not returned as a previous owner")
}

func TestGatewayCachePreemptionOwnerLeaseHonorsOwnershipAndTTL(t *testing.T) {
	cache, mr := newGatewayCachePreemptionTest(t)
	ctx := context.Background()
	const groupID = int64(9)
	const sessionHash = "owner-lease"
	initialTTL := 30 * time.Second
	refreshTTL := 2 * time.Minute

	previous, err := cache.ClaimOpenAIResponsesSessionWindow(ctx, groupID, sessionHash, []byte("owner-a"), initialTTL)
	require.NoError(t, err)
	require.Empty(t, previous)
	key := buildOpenAIResponsesSessionWindowKey(groupID, sessionHash)
	require.Equal(t, initialTTL, mr.TTL(key))

	mr.FastForward(10 * time.Second)
	refreshed, err := cache.CompareAndRefreshOpenAIResponsesSessionWindow(ctx, groupID, sessionHash, []byte("stale-owner"), refreshTTL)
	require.NoError(t, err)
	require.False(t, refreshed)
	require.Equal(t, 20*time.Second, mr.TTL(key), "a stale owner must not extend the lease")

	refreshed, err = cache.CompareAndRefreshOpenAIResponsesSessionWindow(ctx, groupID, sessionHash, []byte("owner-a"), refreshTTL)
	require.NoError(t, err)
	require.True(t, refreshed)
	require.Equal(t, refreshTTL, mr.TTL(key))

	previous, err = cache.ClaimOpenAIResponsesSessionWindow(ctx, groupID, sessionHash, []byte("owner-b"), time.Minute)
	require.NoError(t, err)
	require.Equal(t, []byte("owner-a"), previous)
	require.Equal(t, time.Minute, mr.TTL(key), "a replacement claim must install its own fresh TTL")

	deleted, err := cache.CompareAndDeleteOpenAIResponsesSessionWindow(ctx, groupID, sessionHash, []byte("owner-a"))
	require.NoError(t, err)
	require.False(t, deleted, "a stale owner must not delete the replacement")
	current, err := cache.GetOpenAIResponsesSessionWindow(ctx, groupID, sessionHash)
	require.NoError(t, err)
	require.Equal(t, []byte("owner-b"), current)
}
