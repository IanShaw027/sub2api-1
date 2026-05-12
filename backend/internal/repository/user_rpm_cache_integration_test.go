//go:build integration

package repository

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type localRedisServer struct {
	cmd *exec.Cmd
}

func startLocalRedisServer(t *testing.T) (*redis.Client, *localRedisServer) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	cmd := exec.Command("redis-server",
		"--port", fmt.Sprintf("%d", port),
		"--bind", "127.0.0.1",
		"--save", "",
		"--appendonly", "no",
	)
	require.NoError(t, cmd.Start())

	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("127.0.0.1:%d", port),
	})
	require.Eventually(t, func() bool {
		return client.Ping(context.Background()).Err() == nil
	}, 5*time.Second, 50*time.Millisecond)

	t.Cleanup(func() {
		_ = client.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	return client, &localRedisServer{cmd: cmd}
}

func newUserRPMCacheTest(t *testing.T) (*userRPMCacheImpl, *redis.Client) {
	t.Helper()
	rdb, _ := startLocalRedisServer(t)
	return NewUserRPMCache(rdb).(*userRPMCacheImpl), rdb
}

func TestUserRPMCache_AtomicAdmitConsidersLegacyCountersBeforeIncrement(t *testing.T) {
	cache, rdb := newUserRPMCacheTest(t)
	ctx := context.Background()

	minute, err := cache.minuteTS(ctx)
	require.NoError(t, err)

	legacyGroupKey := userGroupRPMLegacyKey(10, 20, minute)
	legacyUserKey := userRPMLegacyKey(10, minute)
	require.NoError(t, rdb.Set(ctx, legacyGroupKey, 2, userRPMKeyTTL).Err())
	require.NoError(t, rdb.Set(ctx, legacyUserKey, 2, userRPMKeyTTL).Err())

	groupCount, userCount, admitted, err := cache.TryIncrementUserAndGroupRPM(ctx, 10, 20, 2, 2, true, true)
	require.NoError(t, err)
	require.False(t, admitted)
	require.Equal(t, 2, groupCount)
	require.Equal(t, 2, userCount)

	clusterGroupCount, err := rdb.Get(ctx, userGroupRPMClusterSlotKey(10, 20, minute)).Int()
	require.ErrorIs(t, err, redis.Nil)
	require.Equal(t, 0, clusterGroupCount)
	clusterUserCount, err := rdb.Get(ctx, userRPMClusterSlotKey(10, minute)).Int()
	require.ErrorIs(t, err, redis.Nil)
	require.Equal(t, 0, clusterUserCount)
}

func TestUserRPMCache_AtomicAdmitWritesLegacyAndClusterWhenAdmitted(t *testing.T) {
	cache, rdb := newUserRPMCacheTest(t)
	ctx := context.Background()

	minute, err := cache.minuteTS(ctx)
	require.NoError(t, err)

	groupCount, userCount, admitted, err := cache.TryIncrementUserAndGroupRPM(ctx, 11, 21, 3, 3, true, true)
	require.NoError(t, err)
	require.True(t, admitted)
	require.Equal(t, 1, groupCount)
	require.Equal(t, 1, userCount)

	clusterGroupCount, err := rdb.Get(ctx, userGroupRPMClusterSlotKey(11, 21, minute)).Int()
	require.NoError(t, err)
	require.Equal(t, 1, clusterGroupCount)
	legacyGroupCount, err := rdb.Get(ctx, userGroupRPMLegacyKey(11, 21, minute)).Int()
	require.NoError(t, err)
	require.Equal(t, 1, legacyGroupCount)

	clusterUserCount, err := rdb.Get(ctx, userRPMClusterSlotKey(11, minute)).Int()
	require.NoError(t, err)
	require.Equal(t, 1, clusterUserCount)
	legacyUserCount, err := rdb.Get(ctx, userRPMLegacyKey(11, minute)).Int()
	require.NoError(t, err)
	require.Equal(t, 1, legacyUserCount)

	require.True(t, rdb.TTL(ctx, userGroupRPMClusterSlotKey(11, 21, minute)).Val() > 0)
}

func TestUserRPMCache_LegacyAdmitRejectsWithoutIncrementWhenAtLimit(t *testing.T) {
	cache, rdb := newUserRPMCacheTest(t)
	ctx := context.Background()

	minute, err := cache.minuteTS(ctx)
	require.NoError(t, err)

	legacyGroupKey := userGroupRPMLegacyKey(12, 22, minute)
	require.NoError(t, rdb.Set(ctx, legacyGroupKey, 2, userRPMKeyTTL).Err())

	count, admitted, err := cache.incrWithTTLAdmit(ctx, legacyGroupKey, 2)
	require.NoError(t, err)
	require.False(t, admitted)
	require.Equal(t, 2, count)
	legacyGroupCount, err := rdb.Get(ctx, legacyGroupKey).Int()
	require.NoError(t, err)
	require.Equal(t, 2, legacyGroupCount)
}

func TestUserRPMCache_AtomicAdmitRollsBackClusterWhenLegacyGroupRejects(t *testing.T) {
	cache, rdb := newUserRPMCacheTest(t)
	ctx := context.Background()

	minute, err := cache.minuteTS(ctx)
	require.NoError(t, err)

	legacyGroupKey := userGroupRPMLegacyKey(14, 24, minute)
	require.NoError(t, rdb.Set(ctx, legacyGroupKey, 2, userRPMKeyTTL).Err())

	groupCount, userCount, admitted, err := cache.TryIncrementUserAndGroupRPM(ctx, 14, 24, 2, 5, true, true)
	require.NoError(t, err)
	require.False(t, admitted)
	require.Equal(t, 2, groupCount)
	require.Equal(t, 0, userCount)

	clusterGroupCount, err := rdb.Get(ctx, userGroupRPMClusterSlotKey(14, 24, minute)).Int()
	require.ErrorIs(t, err, redis.Nil)
	require.Equal(t, 0, clusterGroupCount)
	clusterUserCount, err := rdb.Get(ctx, userRPMClusterSlotKey(14, minute)).Int()
	require.ErrorIs(t, err, redis.Nil)
	require.Equal(t, 0, clusterUserCount)
}

func TestUserRPMCache_AtomicAdmitRollsBackPriorLegacyIncrementWhenLegacyUserRejects(t *testing.T) {
	cache, rdb := newUserRPMCacheTest(t)
	ctx := context.Background()

	minute, err := cache.minuteTS(ctx)
	require.NoError(t, err)

	legacyUserKey := userRPMLegacyKey(15, minute)
	require.NoError(t, rdb.Set(ctx, legacyUserKey, 2, userRPMKeyTTL).Err())

	groupCount, userCount, admitted, err := cache.TryIncrementUserAndGroupRPM(ctx, 15, 25, 5, 2, true, true)
	require.NoError(t, err)
	require.False(t, admitted)
	require.Equal(t, 0, groupCount)
	require.Equal(t, 2, userCount)

	clusterGroupCount, err := rdb.Get(ctx, userGroupRPMClusterSlotKey(15, 25, minute)).Int()
	require.ErrorIs(t, err, redis.Nil)
	require.Equal(t, 0, clusterGroupCount)
	clusterUserCount, err := rdb.Get(ctx, userRPMClusterSlotKey(15, minute)).Int()
	require.ErrorIs(t, err, redis.Nil)
	require.Equal(t, 0, clusterUserCount)

	legacyGroupCount, err := rdb.Get(ctx, userGroupRPMLegacyKey(15, 25, minute)).Int()
	require.ErrorIs(t, err, redis.Nil)
	require.Equal(t, 0, legacyGroupCount)
	legacyUserCount, err := rdb.Get(ctx, legacyUserKey).Int()
	require.NoError(t, err)
	require.Equal(t, 2, legacyUserCount)
}
