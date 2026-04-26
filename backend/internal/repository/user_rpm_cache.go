package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// 用户/分组级 RPM 计数器 Redis 实现。
//
// 设计说明：
//   - 新 key（Cluster 同槽）：
//   - rpm:ug:{uid:minute}:{gid}
//   - rpm:u:{uid:minute}
//   - 旧 key（兼容迁移）：
//   - rpm:ug:uid:gid:minute
//   - rpm:u:uid:minute
//   - 时间来源：rdb.Time()（Redis 服务端时间），避免多实例时钟漂移。
//   - 原子操作：Lua 单 key执行 INCR+EXPIRE，避免多 key transaction 触发 CROSSSLOT。
//   - 兼容策略：双写新/旧 key，返回 max(new, legacy)；滚动升级期间与旧版本计数保持一致。
//   - TTL：120s，覆盖当前分钟窗口 + 少量冗余。
//   - 返回值语义：超限判断由调用方（billing_cache_service.checkRPM）与 RPMLimit 比较完成。
const (
	userGroupRPMLegacyKeyPrefix = "rpm:ug:"
	userRPMLegacyKeyPrefix      = "rpm:u:"
	userGroupRPMClusterKey      = "rpm:ug:"
	userRPMClusterKey           = "rpm:u:"

	userRPMKeyTTL = 120 * time.Second
)

var userRPMIncrExpireScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], ARGV[1])
return count
`)

var userRPMAtomicAdmitScript = redis.NewScript(`
local ttl = tonumber(ARGV[1])
local inc_group = tonumber(ARGV[2]) == 1
local inc_user = tonumber(ARGV[3]) == 1
local group_limit = tonumber(ARGV[4]) or 0
local user_limit = tonumber(ARGV[5]) or 0
local legacy_group_count = tonumber(ARGV[6]) or 0
local legacy_user_count = tonumber(ARGV[7]) or 0
local group_count = 0
local user_count = 0

if inc_group then
  group_count = tonumber(redis.call("GET", KEYS[1]) or "0")
  if legacy_group_count > group_count then
    group_count = legacy_group_count
  end
end
if inc_user then
  user_count = tonumber(redis.call("GET", KEYS[2]) or "0")
  if legacy_user_count > user_count then
    user_count = legacy_user_count
  end
end

if inc_group and group_limit > 0 and group_count + 1 > group_limit then
  return {group_count, user_count, 0}
end
if inc_user and user_limit > 0 and user_count + 1 > user_limit then
  return {group_count, user_count, 0}
end

if inc_group then
  group_count = redis.call("INCR", KEYS[1])
  redis.call("EXPIRE", KEYS[1], ttl)
end
if inc_user then
  user_count = redis.call("INCR", KEYS[2])
  redis.call("EXPIRE", KEYS[2], ttl)
end
return {group_count, user_count, 1}
`)

type userRPMCacheImpl struct {
	rdb *redis.Client
}

// NewUserRPMCache 创建用户/分组级 RPM 计数器。
func NewUserRPMCache(rdb *redis.Client) service.UserRPMCache {
	return &userRPMCacheImpl{rdb: rdb}
}

// minuteTS 获取当前 Redis 服务端分钟时间戳。
func (c *userRPMCacheImpl) minuteTS(ctx context.Context) (int64, error) {
	t, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return 0, fmt.Errorf("redis TIME: %w", err)
	}
	return t.Unix() / 60, nil
}

func userGroupRPMClusterSlotKey(userID, groupID, minute int64) string {
	return fmt.Sprintf("%s{%d:%d}:%d", userGroupRPMClusterKey, userID, minute, groupID)
}

func userRPMClusterSlotKey(userID, minute int64) string {
	return fmt.Sprintf("%s{%d:%d}", userRPMClusterKey, userID, minute)
}

func userGroupRPMLegacyKey(userID, groupID, minute int64) string {
	return fmt.Sprintf("%s%d:%d:%d", userGroupRPMLegacyKeyPrefix, userID, groupID, minute)
}

func userRPMLegacyKey(userID, minute int64) string {
	return fmt.Sprintf("%s%d:%d", userRPMLegacyKeyPrefix, userID, minute)
}

// incrWithTTL 对单 key 执行原子 INCR+EXPIRE。
func (c *userRPMCacheImpl) incrWithTTL(ctx context.Context, key string) (int, error) {
	count, err := userRPMIncrExpireScript.Run(
		ctx,
		c.rdb,
		[]string{key},
		int64(userRPMKeyTTL/time.Second),
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("user rpm increment key=%s: %w", key, err)
	}
	return int(count), nil
}

// incrWithLegacyCompat 双写新/旧 key，返回两者较大值，保证滚动升级期间计数连续。
func (c *userRPMCacheImpl) incrWithLegacyCompat(ctx context.Context, clusterKey, legacyKey string) (int, error) {
	clusterCount, err := c.incrWithTTL(ctx, clusterKey)
	if err != nil {
		return 0, err
	}
	if legacyKey == "" || legacyKey == clusterKey {
		return clusterCount, nil
	}
	legacyCount, err := c.incrWithTTL(ctx, legacyKey)
	if err != nil {
		return 0, err
	}
	if legacyCount > clusterCount {
		return legacyCount, nil
	}
	return clusterCount, nil
}

func (c *userRPMCacheImpl) getRPMValue(ctx context.Context, key string) (int, error) {
	val, err := c.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("user rpm get key=%s: %w", key, err)
	}
	return val, nil
}

func (c *userRPMCacheImpl) getWithLegacyCompat(ctx context.Context, clusterKey, legacyKey string) (int, error) {
	clusterVal, err := c.getRPMValue(ctx, clusterKey)
	if err != nil {
		return 0, err
	}
	if legacyKey == "" || legacyKey == clusterKey {
		return clusterVal, nil
	}
	legacyVal, err := c.getRPMValue(ctx, legacyKey)
	if err != nil {
		return 0, err
	}
	if legacyVal > clusterVal {
		return legacyVal, nil
	}
	return clusterVal, nil
}

// IncrementUserAndGroupRPM 在同一 Redis 分钟时间源下递增 user-group 与 user 计数器。
// 仅在需要同时递增两个计数器时使用；调用方通过 incGroup/incUser 控制分支。
func (c *userRPMCacheImpl) IncrementUserAndGroupRPM(ctx context.Context, userID, groupID int64, incGroup, incUser bool) (groupCount, userCount int, err error) {
	if !incGroup && !incUser {
		return 0, 0, nil
	}

	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, 0, err
	}

	if incGroup {
		groupCount, err = c.incrWithLegacyCompat(
			ctx,
			userGroupRPMClusterSlotKey(userID, groupID, minute),
			userGroupRPMLegacyKey(userID, groupID, minute),
		)
		if err != nil {
			return 0, 0, err
		}
	}
	if incUser {
		userCount, err = c.incrWithLegacyCompat(
			ctx,
			userRPMClusterSlotKey(userID, minute),
			userRPMLegacyKey(userID, minute),
		)
		if err != nil {
			return 0, 0, err
		}
	}
	return groupCount, userCount, nil
}

// TryIncrementUserAndGroupRPM 原子检查并递增 user-group 与 user 计数。
func (c *userRPMCacheImpl) TryIncrementUserAndGroupRPM(ctx context.Context, userID, groupID int64, groupLimit, userLimit int, incGroup, incUser bool) (groupCount, userCount int, admitted bool, err error) {
	if !incGroup && !incUser {
		return 0, 0, true, nil
	}

	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, 0, false, err
	}

	clusterGroupKey := userGroupRPMClusterSlotKey(userID, groupID, minute)
	clusterUserKey := userRPMClusterSlotKey(userID, minute)
	legacyGroupCount := 0
	if incGroup {
		legacyGroupCount, err = c.getRPMValue(ctx, userGroupRPMLegacyKey(userID, groupID, minute))
		if err != nil {
			return 0, 0, false, err
		}
	}
	legacyUserCount := 0
	if incUser {
		legacyUserCount, err = c.getRPMValue(ctx, userRPMLegacyKey(userID, minute))
		if err != nil {
			return 0, 0, false, err
		}
	}
	incGroupArg := 0
	if incGroup {
		incGroupArg = 1
	}
	incUserArg := 0
	if incUser {
		incUserArg = 1
	}

	vals, err := userRPMAtomicAdmitScript.Run(
		ctx,
		c.rdb,
		[]string{clusterGroupKey, clusterUserKey},
		int64(userRPMKeyTTL/time.Second),
		incGroupArg,
		incUserArg,
		groupLimit,
		userLimit,
		legacyGroupCount,
		legacyUserCount,
	).Slice()
	if err != nil {
		return 0, 0, false, fmt.Errorf("user rpm atomic admit: %w", err)
	}
	if len(vals) != 3 {
		return 0, 0, false, fmt.Errorf("user rpm atomic admit returned %d values", len(vals))
	}

	groupCount = redisInt(vals[0])
	userCount = redisInt(vals[1])
	admitted = redisInt(vals[2]) == 1
	if !admitted {
		return groupCount, userCount, false, nil
	}

	if incGroup {
		legacyCount, err := c.incrWithTTL(ctx, userGroupRPMLegacyKey(userID, groupID, minute))
		if err != nil {
			return 0, 0, false, err
		}
		if legacyCount > groupCount {
			groupCount = legacyCount
		}
	}
	if incUser {
		legacyCount, err := c.incrWithTTL(ctx, userRPMLegacyKey(userID, minute))
		if err != nil {
			return 0, 0, false, err
		}
		if legacyCount > userCount {
			userCount = legacyCount
		}
	}
	return groupCount, userCount, true, nil
}

func redisInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case string:
		parsed, _ := strconv.Atoi(n)
		return parsed
	case []byte:
		parsed, _ := strconv.Atoi(string(n))
		return parsed
	default:
		return 0
	}
}

// IncrementUserGroupRPM 递增 (user, group) 分钟计数。
func (c *userRPMCacheImpl) IncrementUserGroupRPM(ctx context.Context, userID, groupID int64) (int, error) {
	groupCount, _, err := c.IncrementUserAndGroupRPM(ctx, userID, groupID, true, false)
	return groupCount, err
}

// IncrementUserRPM 递增用户分钟计数。
func (c *userRPMCacheImpl) IncrementUserRPM(ctx context.Context, userID int64) (int, error) {
	_, userCount, err := c.IncrementUserAndGroupRPM(ctx, userID, 0, false, true)
	return userCount, err
}

// GetUserGroupRPM 获取 (user, group) 当前分钟已用 RPM（只读）。
func (c *userRPMCacheImpl) GetUserGroupRPM(ctx context.Context, userID, groupID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	val, err := c.getWithLegacyCompat(
		ctx,
		userGroupRPMClusterSlotKey(userID, groupID, minute),
		userGroupRPMLegacyKey(userID, groupID, minute),
	)
	if err != nil {
		return 0, fmt.Errorf("user group rpm get: %w", err)
	}
	return val, nil
}

// GetUserRPM 获取用户当前分钟已用 RPM（只读）。
func (c *userRPMCacheImpl) GetUserRPM(ctx context.Context, userID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	val, err := c.getWithLegacyCompat(
		ctx,
		userRPMClusterSlotKey(userID, minute),
		userRPMLegacyKey(userID, minute),
	)
	if err != nil {
		return 0, fmt.Errorf("user rpm get: %w", err)
	}
	return val, nil
}
