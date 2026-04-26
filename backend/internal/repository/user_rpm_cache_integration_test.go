//go:build integration

package repository

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UserRPMCacheSuite struct {
	IntegrationRedisSuite
	cache *userRPMCacheImpl
}

func (s *UserRPMCacheSuite) SetupTest() {
	s.IntegrationRedisSuite.SetupTest()
	s.cache = NewUserRPMCache(s.rdb).(*userRPMCacheImpl)
}

func (s *UserRPMCacheSuite) TestAtomicAdmitConsidersLegacyCountersBeforeIncrement() {
	minute, err := s.cache.minuteTS(s.ctx)
	require.NoError(s.T(), err)

	legacyGroupKey := userGroupRPMLegacyKey(10, 20, minute)
	legacyUserKey := userRPMLegacyKey(10, minute)
	require.NoError(s.T(), s.rdb.Set(s.ctx, legacyGroupKey, 2, userRPMKeyTTL).Err())
	require.NoError(s.T(), s.rdb.Set(s.ctx, legacyUserKey, 2, userRPMKeyTTL).Err())

	groupCount, userCount, admitted, err := s.cache.TryIncrementUserAndGroupRPM(s.ctx, 10, 20, 2, 2, true, true)
	require.NoError(s.T(), err)
	require.False(s.T(), admitted)
	require.Equal(s.T(), 2, groupCount)
	require.Equal(s.T(), 2, userCount)

	clusterGroupCount, err := s.rdb.Get(s.ctx, userGroupRPMClusterSlotKey(10, 20, minute)).Int()
	require.ErrorIs(s.T(), err, redis.Nil)
	require.Equal(s.T(), 0, clusterGroupCount)
	clusterUserCount, err := s.rdb.Get(s.ctx, userRPMClusterSlotKey(10, minute)).Int()
	require.ErrorIs(s.T(), err, redis.Nil)
	require.Equal(s.T(), 0, clusterUserCount)
}

func (s *UserRPMCacheSuite) TestAtomicAdmitWritesLegacyAndClusterWhenAdmitted() {
	minute, err := s.cache.minuteTS(s.ctx)
	require.NoError(s.T(), err)

	groupCount, userCount, admitted, err := s.cache.TryIncrementUserAndGroupRPM(s.ctx, 11, 21, 3, 3, true, true)
	require.NoError(s.T(), err)
	require.True(s.T(), admitted)
	require.Equal(s.T(), 1, groupCount)
	require.Equal(s.T(), 1, userCount)

	clusterGroupCount, err := s.rdb.Get(s.ctx, userGroupRPMClusterSlotKey(11, 21, minute)).Int()
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, clusterGroupCount)
	legacyGroupCount, err := s.rdb.Get(s.ctx, userGroupRPMLegacyKey(11, 21, minute)).Int()
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, legacyGroupCount)

	clusterUserCount, err := s.rdb.Get(s.ctx, userRPMClusterSlotKey(11, minute)).Int()
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, clusterUserCount)
	legacyUserCount, err := s.rdb.Get(s.ctx, userRPMLegacyKey(11, minute)).Int()
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, legacyUserCount)

	s.AssertTTLWithin(s.rdb.TTL(s.ctx, userGroupRPMClusterSlotKey(11, 21, minute)).Val(), time.Second, userRPMKeyTTL)
}

func TestUserRPMCacheSuite(t *testing.T) {
	suite.Run(t, new(UserRPMCacheSuite))
}
