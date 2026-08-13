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
//   - key 形式：rpm:ug:{uid}:{gid}:{minute}、rpm:u:{uid}:{minute}
//   - 时间来源：rdb.Time()（Redis 服务端时间），避免多实例时钟漂移。
//   - 单 key 递增：TxPipeline (MULTI/EXEC) 执行 INCR+EXPIRE。
//   - 双 key 准入：Lua 先检查再递增，避免先加后判的竞态超限。
//   - TTL：120s，覆盖当前分钟窗口 + 少量冗余。
//   - 返回值语义：超限判断由调用方（billing_cache_service.checkRPM）与 RPMLimit 比较完成。
const (
	userGroupRPMKeyPrefix = "rpm:ug:"
	userRPMKeyPrefix      = "rpm:u:"

	userRPMKeyTTL = 120 * time.Second
)

var userRPMAtomicAdmitScript = redis.NewScript(`
local ttl = tonumber(ARGV[1])
local inc_group = tonumber(ARGV[2]) == 1
local inc_user = tonumber(ARGV[3]) == 1
local group_limit = tonumber(ARGV[4]) or 0
local user_limit = tonumber(ARGV[5]) or 0
local group_count = 0
local user_count = 0

if inc_group then
  group_count = tonumber(redis.call("GET", KEYS[1]) or "0")
end
if inc_user then
  user_count = tonumber(redis.call("GET", KEYS[2]) or "0")
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

// atomicIncr 原子 INCR+EXPIRE。
func (c *userRPMCacheImpl) atomicIncr(ctx context.Context, key string) (int, error) {
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, userRPMKeyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("user rpm increment: %w", err)
	}
	return int(incr.Val()), nil
}

// IncrementUserGroupRPM 递增 (user, group) 分钟计数。
func (c *userRPMCacheImpl) IncrementUserGroupRPM(ctx context.Context, userID, groupID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("%s%d:%d:%d", userGroupRPMKeyPrefix, userID, groupID, minute)
	return c.atomicIncr(ctx, key)
}

// IncrementUserRPM 递增用户分钟计数。
func (c *userRPMCacheImpl) IncrementUserRPM(ctx context.Context, userID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("%s%d:%d", userRPMKeyPrefix, userID, minute)
	return c.atomicIncr(ctx, key)
}

// GetUserGroupRPM 获取 (user, group) 当前分钟已用 RPM（只读）。
func (c *userRPMCacheImpl) GetUserGroupRPM(ctx context.Context, userID, groupID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("%s%d:%d:%d", userGroupRPMKeyPrefix, userID, groupID, minute)
	val, err := c.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
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
	key := fmt.Sprintf("%s%d:%d", userRPMKeyPrefix, userID, minute)
	val, err := c.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("user rpm get: %w", err)
	}
	return val, nil
}

// TryIncrementUserAndGroupRPM atomically checks group/user limits then increments
// the selected counters. admitted=false means neither counter was increased.
func (c *userRPMCacheImpl) TryIncrementUserAndGroupRPM(ctx context.Context, userID, groupID int64, groupLimit, userLimit int, incGroup, incUser bool) (groupCount, userCount int, admitted bool, err error) {
	if !incGroup && !incUser {
		return 0, 0, true, nil
	}
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, 0, false, err
	}
	groupKey := fmt.Sprintf("%s%d:%d:%d", userGroupRPMKeyPrefix, userID, groupID, minute)
	userKey := fmt.Sprintf("%s%d:%d", userRPMKeyPrefix, userID, minute)
	incGroupArg, incUserArg := 0, 0
	if incGroup {
		incGroupArg = 1
	}
	if incUser {
		incUserArg = 1
	}
	vals, err := userRPMAtomicAdmitScript.Run(
		ctx,
		c.rdb,
		[]string{groupKey, userKey},
		int64(userRPMKeyTTL/time.Second),
		incGroupArg,
		incUserArg,
		groupLimit,
		userLimit,
	).Slice()
	if err != nil {
		return 0, 0, false, fmt.Errorf("user rpm atomic admit: %w", err)
	}
	if len(vals) != 3 {
		return 0, 0, false, fmt.Errorf("user rpm atomic admit returned %d values", len(vals))
	}
	return redisScriptInt(vals[0]), redisScriptInt(vals[1]), redisScriptInt(vals[2]) == 1, nil
}

func redisScriptInt(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	case string:
		parsed, _ := strconv.Atoi(n)
		return parsed
	default:
		return 0
	}
}
