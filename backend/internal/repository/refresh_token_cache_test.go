//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newRefreshTokenCacheTest(t *testing.T) (*refreshTokenCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return &refreshTokenCache{rdb: client}, mr
}

func TestRefreshTokenCacheRotateIsAtomicAndIdempotent(t *testing.T) {
	cache, mr := newRefreshTokenCacheTest(t)
	ctx := context.Background()
	oldHash := "old-hash"
	newHash := "new-hash"
	oldData := &service.RefreshTokenData{
		UserID: 42, FamilyID: "family-1", AuthEpoch: "epoch-1",
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, cache.StoreRefreshToken(ctx, oldHash, oldData, time.Hour))

	newData := *oldData
	newData.CreatedAt = time.Now()
	result, err := cache.RotateRefreshToken(ctx, oldHash, newHash, &newData, time.Hour, 5*time.Second, `{"access_token":"same"}`)
	require.NoError(t, err)
	require.Equal(t, service.RefreshTokenRotationSucceeded, result.Status)

	_, err = cache.GetRefreshToken(ctx, oldHash)
	require.ErrorIs(t, err, service.ErrRefreshTokenNotFound)
	stored, err := cache.GetRefreshToken(ctx, newHash)
	require.NoError(t, err)
	require.Equal(t, "epoch-1", stored.AuthEpoch)
	userHashes, err := cache.GetUserTokenHashes(ctx, 42)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{newHash}, userHashes)
	familyHashes, err := cache.GetFamilyTokenHashes(ctx, "family-1")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{newHash}, familyHashes)

	retry, err := cache.RotateRefreshToken(ctx, oldHash, "unused", &newData, time.Hour, 5*time.Second, `{"access_token":"different"}`)
	require.NoError(t, err)
	require.Equal(t, service.RefreshTokenRotationRetry, retry.Status)
	require.Equal(t, result.Response, retry.Response)

	mr.FastForward(6 * time.Second)
	reused, err := cache.GetRefreshTokenRotationState(ctx, oldHash)
	require.NoError(t, err)
	require.Equal(t, service.RefreshTokenRotationReused, reused.Status)
	require.Equal(t, "family-1", reused.TokenData.FamilyID)
}

func TestRefreshTokenCacheRevokeUserSessionsSetsEpochAtomically(t *testing.T) {
	cache, mr := newRefreshTokenCacheTest(t)
	ctx := context.Background()
	data := &service.RefreshTokenData{
		UserID: 7, FamilyID: "family-7", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, cache.StoreRefreshToken(ctx, "token-7", data, time.Hour))
	require.NoError(t, cache.RevokeUserSessions(ctx, 7, "epoch-after-revoke", 2*time.Hour))

	_, err := cache.GetRefreshToken(ctx, "token-7")
	require.ErrorIs(t, err, service.ErrRefreshTokenNotFound)
	epoch, err := cache.GetUserAuthEpoch(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, "epoch-after-revoke", epoch)
	require.False(t, mr.Exists(userRefreshTokensKey(7)))
	require.Equal(t, 2*time.Hour, mr.TTL(userAuthEpochKey(7)))
}

func TestRefreshTokenCacheEnsureUserAuthEpochIsStable(t *testing.T) {
	cache, mr := newRefreshTokenCacheTest(t)
	ctx := context.Background()

	first, err := cache.EnsureUserAuthEpoch(ctx, 8, "epoch-first", time.Hour)
	require.NoError(t, err)
	require.Equal(t, "epoch-first", first)
	second, err := cache.EnsureUserAuthEpoch(ctx, 8, "epoch-second", 2*time.Hour)
	require.NoError(t, err)
	require.Equal(t, "epoch-first", second)
	require.Equal(t, 2*time.Hour, mr.TTL(userAuthEpochKey(8)))
}

func TestRefreshTokenCacheDeleteFamilyToleratesMalformedRecords(t *testing.T) {
	cache, mr := newRefreshTokenCacheTest(t)
	ctx := context.Background()
	require.NoError(t, mr.Set(refreshTokenKey("bad"), "not-json"))
	mr.SAdd(tokenFamilyKey("family-bad"), "bad")

	require.NoError(t, cache.DeleteTokenFamily(ctx, "family-bad"))
	require.False(t, mr.Exists(refreshTokenKey("bad")))
	require.False(t, mr.Exists(tokenFamilyKey("family-bad")))
}

func TestRefreshTokenCacheDeleteRefreshTokenCleansIndexes(t *testing.T) {
	cache, mr := newRefreshTokenCacheTest(t)
	ctx := context.Background()
	data := &service.RefreshTokenData{
		UserID: 9, FamilyID: "family-9", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, cache.StoreRefreshToken(ctx, "token-9", data, time.Hour))

	require.NoError(t, cache.DeleteRefreshToken(ctx, "token-9"))
	require.False(t, mr.Exists(userRefreshTokensKey(9)))
	require.False(t, mr.Exists(tokenFamilyKey("family-9")))
	require.False(t, mr.Exists(refreshTokenKey("token-9")))
}
