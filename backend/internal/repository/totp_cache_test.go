//go:build unit

package repository

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTotpCacheTest(t *testing.T) *TotpCache {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return NewTotpCache(client).(*TotpCache)
}

func TestTotpCacheReserveVerifyAttemptIsAtomic(t *testing.T) {
	cache := newTotpCacheTest(t)
	const requests = 32

	counts := make(chan int, requests)
	errs := make(chan error, requests)
	var wg sync.WaitGroup
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, err := cache.ReserveVerifyAttempt(context.Background(), 42, requests)
			counts <- count
			errs <- err
		}()
	}
	wg.Wait()
	close(counts)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	got := make([]int, 0, requests)
	for count := range counts {
		got = append(got, count)
	}
	sort.Ints(got)
	for i := range requests {
		require.Equal(t, i+1, got[i])
	}
}

func TestTotpCacheRejectedAttemptsDoNotKeepIncrementing(t *testing.T) {
	cache := newTotpCacheTest(t)
	ctx := context.Background()

	first, err := cache.ReserveVerifyAttempt(ctx, 43, 2)
	require.NoError(t, err)
	require.Equal(t, 1, first)
	second, err := cache.ReserveVerifyAttempt(ctx, 43, 2)
	require.NoError(t, err)
	require.Equal(t, 2, second)

	third, err := cache.ReserveVerifyAttempt(ctx, 43, 2)
	require.NoError(t, err)
	require.Equal(t, 3, third)
	fourth, err := cache.ReserveVerifyAttempt(ctx, 43, 2)
	require.NoError(t, err)
	require.Equal(t, 3, fourth)
}

func TestTotpCacheLoginSessionClaimOwnershipAndFinalize(t *testing.T) {
	cache := newTotpCacheTest(t)
	ctx := context.Background()
	const tempToken = "temporary-login-token"
	require.NoError(t, cache.SetLoginSession(ctx, tempToken, &service.TotpLoginSession{UserID: 7}, time.Minute))

	session, claimed, err := cache.ClaimLoginSession(ctx, tempToken, "claim-a", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)
	require.EqualValues(t, 7, session.UserID)

	_, claimed, err = cache.ClaimLoginSession(ctx, tempToken, "claim-b", time.Minute)
	require.NoError(t, err)
	require.False(t, claimed)
	renewed, err := cache.RenewLoginSessionClaim(ctx, tempToken, "claim-b", 2*time.Minute)
	require.NoError(t, err)
	require.False(t, renewed)
	renewed, err = cache.RenewLoginSessionClaim(ctx, tempToken, "claim-a", 2*time.Minute)
	require.NoError(t, err)
	require.True(t, renewed)

	require.NoError(t, cache.ReleaseLoginSessionClaim(ctx, tempToken, "claim-b"))
	finalized, err := cache.FinalizeLoginSessionClaim(ctx, tempToken, "claim-b")
	require.NoError(t, err)
	require.False(t, finalized)

	finalized, err = cache.FinalizeLoginSessionClaim(ctx, tempToken, "claim-a")
	require.NoError(t, err)
	require.True(t, finalized)
	session, err = cache.GetLoginSession(ctx, tempToken)
	require.NoError(t, err)
	require.Nil(t, session)
}

func TestTotpCacheReleaseClaimKeepsSessionRetryable(t *testing.T) {
	cache := newTotpCacheTest(t)
	ctx := context.Background()
	const tempToken = "retryable-login-token"
	require.NoError(t, cache.SetLoginSession(ctx, tempToken, &service.TotpLoginSession{UserID: 9}, time.Minute))

	_, claimed, err := cache.ClaimLoginSession(ctx, tempToken, "claim-a", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, cache.ReleaseLoginSessionClaim(ctx, tempToken, "claim-a"))

	session, claimed, err := cache.ClaimLoginSession(ctx, tempToken, "claim-b", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)
	require.EqualValues(t, 9, session.UserID)
}
