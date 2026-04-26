//go:build unit

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// userRPMCacheStub 记录每种计数器被调用的次数，并可注入返回值与错误。
type userRPMCacheStub struct {
	userGroupCalls int32
	userCalls      int32
	combinedCalls  int32
	forceCombined  bool

	userGroupCounts  []int // 依次返回的计数值
	userGroupErr     error
	userCounts       []int
	userErr          error
	combinedGroup    []int
	combinedUser     []int
	combinedErr      error
	combinedIncGroup []bool
	combinedIncUser  []bool
	combinedUserIDs  []int64
	combinedGroupIDs []int64
}

func (s *userRPMCacheStub) IncrementUserGroupRPM(_ context.Context, _, _ int64) (int, error) {
	idx := int(atomic.AddInt32(&s.userGroupCalls, 1)) - 1
	if s.userGroupErr != nil {
		return 0, s.userGroupErr
	}
	if idx < len(s.userGroupCounts) {
		return s.userGroupCounts[idx], nil
	}
	return 1, nil
}

func (s *userRPMCacheStub) IncrementUserRPM(_ context.Context, _ int64) (int, error) {
	idx := int(atomic.AddInt32(&s.userCalls, 1)) - 1
	if s.userErr != nil {
		return 0, s.userErr
	}
	if idx < len(s.userCounts) {
		return s.userCounts[idx], nil
	}
	return 1, nil
}

func (s *userRPMCacheStub) IncrementUserAndGroupRPM(_ context.Context, userID, groupID int64, incGroup, incUser bool) (int, int, error) {
	idx := int(atomic.AddInt32(&s.combinedCalls, 1)) - 1
	s.combinedIncGroup = append(s.combinedIncGroup, incGroup)
	s.combinedIncUser = append(s.combinedIncUser, incUser)
	s.combinedUserIDs = append(s.combinedUserIDs, userID)
	s.combinedGroupIDs = append(s.combinedGroupIDs, groupID)
	if s.combinedErr != nil {
		return 0, 0, s.combinedErr
	}
	if !s.forceCombined && len(s.combinedGroup) == 0 && len(s.combinedUser) == 0 {
		groupCount := 0
		userCount := 0
		var err error
		if incGroup {
			groupCount, err = s.IncrementUserGroupRPM(context.Background(), userID, groupID)
			if err != nil {
				return 0, 0, err
			}
		}
		if incUser {
			userCount, err = s.IncrementUserRPM(context.Background(), userID)
			if err != nil {
				return 0, 0, err
			}
		}
		return groupCount, userCount, nil
	}
	groupCount := 0
	userCount := 0
	if incGroup {
		if idx < len(s.combinedGroup) {
			groupCount = s.combinedGroup[idx]
		} else {
			groupCount = 1
		}
	}
	if incUser {
		if idx < len(s.combinedUser) {
			userCount = s.combinedUser[idx]
		} else {
			userCount = 1
		}
	}
	return groupCount, userCount, nil
}

func (s *userRPMCacheStub) GetUserGroupRPM(_ context.Context, _, _ int64) (int, error) {
	return 0, nil
}

func (s *userRPMCacheStub) GetUserRPM(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

type userRPMAtomicAdmitStub struct {
	userRPMCacheStub

	tryCalls       int32
	tryGroupCount  int
	tryUserCount   int
	tryGroupCounts []int
	tryUserCounts  []int
}

func (s *userRPMAtomicAdmitStub) TryIncrementUserAndGroupRPM(_ context.Context, _ int64, _ int64, groupLimit, userLimit int, incGroup, incUser bool) (int, int, bool, error) {
	atomic.AddInt32(&s.tryCalls, 1)
	nextGroup := s.tryGroupCount
	nextUser := s.tryUserCount
	if incGroup {
		nextGroup++
	}
	if incUser {
		nextUser++
	}
	if incGroup && groupLimit > 0 && nextGroup > groupLimit {
		return s.tryGroupCount, s.tryUserCount, false, nil
	}
	if incUser && userLimit > 0 && nextUser > userLimit {
		return s.tryGroupCount, s.tryUserCount, false, nil
	}
	s.tryGroupCount = nextGroup
	s.tryUserCount = nextUser
	s.tryGroupCounts = append(s.tryGroupCounts, s.tryGroupCount)
	s.tryUserCounts = append(s.tryUserCounts, s.tryUserCount)
	return s.tryGroupCount, s.tryUserCount, true, nil
}

// rpmOverrideRepoStub 专用于 checkRPM 分支测试，只实现必要方法。
type rpmOverrideRepoStub struct {
	UserGroupRateRepository

	override *int
	err      error
	calls    int32
}

func (s *rpmOverrideRepoStub) GetRPMOverrideByUserAndGroup(_ context.Context, _, _ int64) (*int, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.err != nil {
		return nil, s.err
	}
	return s.override, nil
}

func newBillingServiceForRPM(t *testing.T, cache UserRPMCache, rateRepo UserGroupRateRepository) *BillingCacheService {
	t.Helper()
	// 用 nil BillingCache 走 "无缓存" 分支，避免 CheckBillingEligibility 副作用。
	// 我们只直接测 checkRPM。
	svc := NewBillingCacheService(nil, nil, nil, nil, cache, rateRepo, &config.Config{})
	t.Cleanup(svc.Stop)
	return svc
}

func TestBillingCacheService_CheckRPM_OverrideTakesPrecedenceOverGroup(t *testing.T) {
	override := 2
	// user-group 计数: 1, 2, 3；user 计数: 默认返回 1（远小于 RPMLimit=100，不干扰）
	cache := &userRPMCacheStub{userGroupCounts: []int{1, 2, 3}}
	repo := &rpmOverrideRepoStub{override: &override}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 100} // 全局上限设高，不干扰 override 测试
	group := &Group{ID: 10, RPMLimit: 100}

	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrGroupRPMExceeded)

	require.EqualValues(t, 3, atomic.LoadInt32(&cache.userGroupCalls), "override 命中分支应走 user-group 计数")
	// combined 计数路径下，group/user 在每次请求内并行计数。
	require.EqualValues(t, 3, atomic.LoadInt32(&cache.userCalls), "combined 路径应每次都递增 user 计数器")
	require.EqualValues(t, 3, atomic.LoadInt32(&repo.calls))
}

func TestBillingCacheService_CheckRPM_UserLimitIsGlobalHardCap(t *testing.T) {
	override := 100 // override 很高
	// user-group 计数: 默认返回 1（远小于 override）；user 计数: 1, 2, 3
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3}}
	repo := &rpmOverrideRepoStub{override: &override}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 2} // 全局硬上限=2，应覆盖 override=100
	group := &Group{ID: 10, RPMLimit: 100}

	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrUserRPMExceeded, "user 全局硬上限应优先于 override")
}

func TestBillingCacheService_CheckRPM_OverrideZeroSkipsGroupButUserStillApplies(t *testing.T) {
	zero := 0
	// user 计数: 依次返回 1..6
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3, 4, 5, 6}}
	repo := &rpmOverrideRepoStub{override: &zero}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 5}
	group := &Group{ID: 10, RPMLimit: 100}

	// override=0 跳过分组计数，但 user.RPMLimit=5 仍生效
	for i := 0; i < 5; i++ {
		require.NoError(t, svc.checkRPM(context.Background(), user, group), "request %d should pass", i+1)
	}
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrUserRPMExceeded,
		"override=0 跳过分组但 user 全局上限仍应生效")
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls), "override=0 不应触发分组计数器")
	require.EqualValues(t, 6, atomic.LoadInt32(&cache.userCalls), "user 计数器应被调用")
}

func TestBillingCacheService_CheckRPM_OverrideZeroAndUserZeroIsFullyUnlimited(t *testing.T) {
	zero := 0
	cache := &userRPMCacheStub{}
	repo := &rpmOverrideRepoStub{override: &zero}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 0} // user 也不限
	group := &Group{ID: 10, RPMLimit: 100}

	for i := 0; i < 50; i++ {
		require.NoError(t, svc.checkRPM(context.Background(), user, group))
	}
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls), "override=0 不触发分组计数")
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userCalls), "user.RPMLimit=0 也不触发用户计数")
}

func TestBillingCacheService_CheckRPM_NilOverrideFallsThroughToGroup(t *testing.T) {
	// user-group 计数: 5, 6；user 计数: 默认 1（不干扰）
	cache := &userRPMCacheStub{userGroupCounts: []int{5, 6}}
	repo := &rpmOverrideRepoStub{override: nil}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 999} // 全局上限很高，group 先超
	group := &Group{ID: 10, RPMLimit: 5}

	require.NoError(t, svc.checkRPM(context.Background(), user, group))                      // ug=5, user=1, 都没超
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrGroupRPMExceeded) // ug=6 > 5

	require.EqualValues(t, 2, atomic.LoadInt32(&cache.userGroupCalls))
	require.EqualValues(t, 2, atomic.LoadInt32(&cache.userCalls), "combined 路径下 group 超限当次也会并行递增 user")
}

func TestBillingCacheService_CheckRPM_OverrideLookupErrorFallsThroughToGroup(t *testing.T) {
	cache := &userRPMCacheStub{userGroupCounts: []int{3}}
	repo := &rpmOverrideRepoStub{err: errors.New("db down")}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 0}
	group := &Group{ID: 10, RPMLimit: 10}

	// override 查询失败后应继续尝试 group 分支（不直接拒绝）
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.EqualValues(t, 1, atomic.LoadInt32(&cache.userGroupCalls))
	require.EqualValues(t, 1, atomic.LoadInt32(&repo.calls))
}

func TestBillingCacheService_CheckRPM_UserLevelFallbackWhenGroupUnlimited(t *testing.T) {
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3}}
	repo := &rpmOverrideRepoStub{override: nil}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 2}
	group := &Group{ID: 10, RPMLimit: 0} // 分组未设限

	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrUserRPMExceeded)

	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls), "group 未设限时不应 INCR user-group 键")
	require.EqualValues(t, 3, atomic.LoadInt32(&cache.userCalls))
}

func TestBillingCacheService_CheckRPM_NoLimitsConfiguredIsNoop(t *testing.T) {
	cache := &userRPMCacheStub{}
	repo := &rpmOverrideRepoStub{override: nil}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 0}
	group := &Group{ID: 10, RPMLimit: 0}

	for i := 0; i < 10; i++ {
		require.NoError(t, svc.checkRPM(context.Background(), user, group))
	}
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userCalls))
}

func TestBillingCacheService_CheckRPM_RedisErrorFailOpen(t *testing.T) {
	cache := &userRPMCacheStub{userGroupErr: errors.New("redis unavailable")}
	repo := &rpmOverrideRepoStub{override: nil}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 0}
	group := &Group{ID: 10, RPMLimit: 5}

	// Redis 故障时应 fail-open，不拒绝请求
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.EqualValues(t, 1, atomic.LoadInt32(&cache.userGroupCalls))
}

func TestBillingCacheService_CheckRPM_NoGroupUsesUserOnly(t *testing.T) {
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3}}
	repo := &rpmOverrideRepoStub{}
	svc := newBillingServiceForRPM(t, cache, repo)

	user := &User{ID: 1, RPMLimit: 2}

	// 无 group（纯用户级限流场景），不应查询 rpm_override。
	require.NoError(t, svc.checkRPM(context.Background(), user, nil))
	require.NoError(t, svc.checkRPM(context.Background(), user, nil))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, nil), ErrUserRPMExceeded)

	require.EqualValues(t, 0, atomic.LoadInt32(&repo.calls), "无 group 时不应查询 rpm_override")
	require.EqualValues(t, 3, atomic.LoadInt32(&cache.userCalls))
}

func TestBillingCacheService_CheckRPM_NilUserIsNoop(t *testing.T) {
	cache := &userRPMCacheStub{}
	repo := &rpmOverrideRepoStub{}
	svc := newBillingServiceForRPM(t, cache, repo)

	require.NoError(t, svc.checkRPM(context.Background(), nil, &Group{ID: 1, RPMLimit: 10}))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userCalls))
	require.EqualValues(t, 0, atomic.LoadInt32(&repo.calls))
}

func TestBillingCacheService_CheckRPM_UsesCombinedIncrementForGroupAndUser(t *testing.T) {
	cache := &userRPMCacheStub{
		forceCombined: true,
		combinedGroup: []int{1, 2, 3},
		combinedUser:  []int{1, 2, 3},
	}
	svc := newBillingServiceForRPM(t, cache, nil)

	user := &User{ID: 1, RPMLimit: 2}
	group := &Group{ID: 10, RPMLimit: 100}

	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrUserRPMExceeded)

	require.EqualValues(t, 3, atomic.LoadInt32(&cache.combinedCalls))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userCalls))
	require.Equal(t, []bool{true, true, true}, cache.combinedIncGroup)
	require.Equal(t, []bool{true, true, true}, cache.combinedIncUser)
	require.Equal(t, []int64{1, 1, 1}, cache.combinedUserIDs)
	require.Equal(t, []int64{10, 10, 10}, cache.combinedGroupIDs)
}

func TestBillingCacheService_CheckRPM_AtomicAdmitRejectsWithoutPollutingCounters(t *testing.T) {
	cache := &userRPMAtomicAdmitStub{}
	svc := newBillingServiceForRPM(t, cache, nil)

	user := &User{ID: 1, RPMLimit: 2}
	group := &Group{ID: 10, RPMLimit: 2}

	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrGroupRPMExceeded)

	require.EqualValues(t, 3, atomic.LoadInt32(&cache.tryCalls))
	require.Equal(t, []int{1, 2}, cache.tryGroupCounts)
	require.Equal(t, []int{1, 2}, cache.tryUserCounts)
	require.Equal(t, 2, cache.tryGroupCount)
	require.Equal(t, 2, cache.tryUserCount)
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.combinedCalls), "atomic admit path should avoid increment-then-check pollution")
}

func TestBillingCacheService_CheckRPM_SnapshotAbsentOverrideSkipsDBLookup(t *testing.T) {
	cache := &userRPMCacheStub{userGroupCounts: []int{1}}
	repo := &rpmOverrideRepoStub{override: nil}
	svc := newBillingServiceForRPM(t, cache, repo)

	absent := userGroupRPMOverrideAbsentSentinel
	user := &User{ID: 1, RPMLimit: 0, UserGroupRPMOverride: &absent}
	group := &Group{ID: 10, RPMLimit: 5}

	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.EqualValues(t, 0, atomic.LoadInt32(&repo.calls), "已知无 override 时不应回源查询")
	require.EqualValues(t, 1, atomic.LoadInt32(&cache.userGroupCalls))
}
