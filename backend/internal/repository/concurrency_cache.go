package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// 并发控制缓存常量定义
//
// 性能优化说明：
// 原实现使用 SCAN 命令遍历独立的槽位键（concurrency:account:{id}:{requestID}），
// 在高并发场景下 SCAN 需要多次往返，且遍历大量键时性能下降明显。
//
// 新实现改用 Redis 有序集合（Sorted Set）：
// 1. 每个账号/用户只有一个键，成员为 requestID，分数为时间戳
// 2. 使用 ZCARD 原子获取并发数，时间复杂度 O(1)
// 3. 使用 ZREMRANGEBYSCORE 清理过期槽位，避免手动管理 TTL
// 4. 单次 Redis 调用完成计数，减少网络往返
const (
	// 并发槽位键前缀（有序集合）
	// 格式: concurrency:account:{accountID}
	accountSlotKeyPrefix = "concurrency:account:"
	// 格式: concurrency:user:{userID}
	userSlotKeyPrefix = "concurrency:user:"

	// Wait queue keys (global structures)
	// - total: integer total queue depth across all users
	// - updated: sorted set of userID -> lastUpdateUnixSec (for TTL cleanup)
	// - counts: hash of userID -> current wait count
	waitQueueTotalKey   = "concurrency:wait:total"
	waitQueueUpdatedKey = "concurrency:wait:updated"
	waitQueueCountsKey  = "concurrency:wait:counts"

	// 默认槽位过期时间（分钟），可通过配置覆盖
	defaultSlotTTLMinutes = 15
)

var (
	// acquireScript 使用有序集合计数并在未达上限时添加槽位
	// 使用 Redis TIME 命令获取服务器时间，避免多实例时钟不同步问题
	// KEYS[1] = 有序集合键 (concurrency:account:{id} / concurrency:user:{id})
	// ARGV[1] = maxConcurrency
	// ARGV[2] = TTL（秒）
	// ARGV[3] = requestID
	acquireScript = redis.NewScript(`
		local key = KEYS[1]
		local maxConcurrency = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		local requestID = ARGV[3]

		-- 使用 Redis 服务器时间，确保多实例时钟一致
		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl

		-- 清理过期槽位
		redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)

		-- 检查是否已存在（支持重试场景刷新时间戳）
		local exists = redis.call('ZSCORE', key, requestID)
		if exists ~= false then
			redis.call('ZADD', key, now, requestID)
			redis.call('EXPIRE', key, ttl)
			return 1
		end

		-- 检查是否达到并发上限
		local count = redis.call('ZCARD', key)
		if count < maxConcurrency then
			redis.call('ZADD', key, now, requestID)
			redis.call('EXPIRE', key, ttl)
			return 1
		end

		return 0
	`)

	// getCountScript 统计有序集合中的槽位数量并清理过期条目
	// 使用 Redis TIME 命令获取服务器时间
	// KEYS[1] = 有序集合键
	// ARGV[1] = TTL（秒）
	getCountScript = redis.NewScript(`
		local key = KEYS[1]
		local ttl = tonumber(ARGV[1])

		-- 使用 Redis 服务器时间
		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl

		redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)
		return redis.call('ZCARD', key)
	`)

	// incrementWaitScript - only sets TTL on first creation to avoid refreshing
	// KEYS[1] = total key
	// KEYS[2] = updated zset key
	// KEYS[3] = counts hash key
	// ARGV[1] = userID
	// ARGV[2] = maxWait
	// ARGV[3] = TTL in seconds
	// ARGV[4] = cleanup limit
	incrementWaitScript = redis.NewScript(`
		local totalKey = KEYS[1]
		local updatedKey = KEYS[2]
		local countsKey = KEYS[3]

		local userID = ARGV[1]
		local maxWait = tonumber(ARGV[2])
		local ttl = tonumber(ARGV[3])
		local cleanupLimit = tonumber(ARGV[4])

		redis.call('SETNX', totalKey, 0)

		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl

		-- Cleanup expired users (bounded)
		local expired = redis.call('ZRANGEBYSCORE', updatedKey, '-inf', expireBefore, 'LIMIT', 0, cleanupLimit)
		for _, uid in ipairs(expired) do
			local c = tonumber(redis.call('HGET', countsKey, uid) or '0')
			if c > 0 then
				redis.call('DECRBY', totalKey, c)
			end
			redis.call('HDEL', countsKey, uid)
			redis.call('ZREM', updatedKey, uid)
		end

		local current = tonumber(redis.call('HGET', countsKey, userID) or '0')
		if current >= maxWait then
			return 0
		end

		local newVal = current + 1
		redis.call('HSET', countsKey, userID, newVal)
		redis.call('ZADD', updatedKey, now, userID)
		redis.call('INCR', totalKey)

		-- Keep global structures from living forever in totally idle deployments.
		local ttlKeep = ttl * 2
		redis.call('EXPIRE', totalKey, ttlKeep)
		redis.call('EXPIRE', updatedKey, ttlKeep)
		redis.call('EXPIRE', countsKey, ttlKeep)

		return 1
	`)

	// decrementWaitScript - same as before
	decrementWaitScript = redis.NewScript(`
		local totalKey = KEYS[1]
		local updatedKey = KEYS[2]
		local countsKey = KEYS[3]

		local userID = ARGV[1]
		local ttl = tonumber(ARGV[2])
		local cleanupLimit = tonumber(ARGV[3])

		redis.call('SETNX', totalKey, 0)

		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl

		-- Cleanup expired users (bounded)
		local expired = redis.call('ZRANGEBYSCORE', updatedKey, '-inf', expireBefore, 'LIMIT', 0, cleanupLimit)
		for _, uid in ipairs(expired) do
			local c = tonumber(redis.call('HGET', countsKey, uid) or '0')
			if c > 0 then
				redis.call('DECRBY', totalKey, c)
			end
			redis.call('HDEL', countsKey, uid)
			redis.call('ZREM', updatedKey, uid)
		end

		local current = tonumber(redis.call('HGET', countsKey, userID) or '0')
		if current <= 0 then
			return 1
		end

		local newVal = current - 1
		if newVal <= 0 then
			redis.call('HDEL', countsKey, userID)
			redis.call('ZREM', updatedKey, userID)
		else
			redis.call('HSET', countsKey, userID, newVal)
			redis.call('ZADD', updatedKey, now, userID)
		end
		redis.call('DECR', totalKey)

		local ttlKeep = ttl * 2
		redis.call('EXPIRE', totalKey, ttlKeep)
		redis.call('EXPIRE', updatedKey, ttlKeep)
		redis.call('EXPIRE', countsKey, ttlKeep)

		return 1
	`)

	// getTotalWaitScript returns the global wait depth with TTL cleanup.
	// KEYS[1] = total key
	// KEYS[2] = updated zset key
	// KEYS[3] = counts hash key
	// ARGV[1] = TTL in seconds
	// ARGV[2] = cleanup limit
	getTotalWaitScript = redis.NewScript(`
		local totalKey = KEYS[1]
		local updatedKey = KEYS[2]
		local countsKey = KEYS[3]

		local ttl = tonumber(ARGV[1])
		local cleanupLimit = tonumber(ARGV[2])

		redis.call('SETNX', totalKey, 0)

		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl

		-- Cleanup expired users (bounded)
		local expired = redis.call('ZRANGEBYSCORE', updatedKey, '-inf', expireBefore, 'LIMIT', 0, cleanupLimit)
		for _, uid in ipairs(expired) do
			local c = tonumber(redis.call('HGET', countsKey, uid) or '0')
			if c > 0 then
				redis.call('DECRBY', totalKey, c)
			end
			redis.call('HDEL', countsKey, uid)
			redis.call('ZREM', updatedKey, uid)
		end

		-- If totalKey got lost but counts exist (e.g. Redis restart), recompute once.
		local total = redis.call('GET', totalKey)
		if total == false then
			total = 0
			local vals = redis.call('HVALS', countsKey)
			for _, v in ipairs(vals) do
				total = total + tonumber(v)
			end
			redis.call('SET', totalKey, total)
		end

		local ttlKeep = ttl * 2
		redis.call('EXPIRE', totalKey, ttlKeep)
		redis.call('EXPIRE', updatedKey, ttlKeep)
		redis.call('EXPIRE', countsKey, ttlKeep)

		local result = tonumber(redis.call('GET', totalKey) or '0')
		if result < 0 then
			result = 0
			redis.call('SET', totalKey, 0)
		end
		return result
	`)
)

type concurrencyCache struct {
	rdb            *redis.Client
	slotTTLSeconds int // 槽位过期时间（秒）
}

// NewConcurrencyCache 创建并发控制缓存
// slotTTLMinutes: 槽位过期时间（分钟），0 或负数使用默认值 15 分钟
func NewConcurrencyCache(rdb *redis.Client, slotTTLMinutes int) service.ConcurrencyCache {
	if slotTTLMinutes <= 0 {
		slotTTLMinutes = defaultSlotTTLMinutes
	}
	return &concurrencyCache{
		rdb:            rdb,
		slotTTLSeconds: slotTTLMinutes * 60,
	}
}

// Helper functions for key generation
func accountSlotKey(accountID int64) string {
	return fmt.Sprintf("%s%d", accountSlotKeyPrefix, accountID)
}

func userSlotKey(userID int64) string {
	return fmt.Sprintf("%s%d", userSlotKeyPrefix, userID)
}

func waitQueueKey(userID int64) string {
	// Historical: per-user string keys were used.
	// Now we use global structures keyed by userID string.
	return strconv.FormatInt(userID, 10)
}

// Account slot operations

func (c *concurrencyCache) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
	key := accountSlotKey(accountID)
	// 时间戳在 Lua 脚本内使用 Redis TIME 命令获取，确保多实例时钟一致
	result, err := acquireScript.Run(ctx, c.rdb, []string{key}, maxConcurrency, c.slotTTLSeconds, requestID).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *concurrencyCache) ReleaseAccountSlot(ctx context.Context, accountID int64, requestID string) error {
	key := accountSlotKey(accountID)
	return c.rdb.ZRem(ctx, key, requestID).Err()
}

func (c *concurrencyCache) GetAccountConcurrency(ctx context.Context, accountID int64) (int, error) {
	key := accountSlotKey(accountID)
	// 时间戳在 Lua 脚本内使用 Redis TIME 命令获取
	result, err := getCountScript.Run(ctx, c.rdb, []string{key}, c.slotTTLSeconds).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// User slot operations

func (c *concurrencyCache) AcquireUserSlot(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
	key := userSlotKey(userID)
	// 时间戳在 Lua 脚本内使用 Redis TIME 命令获取，确保多实例时钟一致
	result, err := acquireScript.Run(ctx, c.rdb, []string{key}, maxConcurrency, c.slotTTLSeconds, requestID).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *concurrencyCache) ReleaseUserSlot(ctx context.Context, userID int64, requestID string) error {
	key := userSlotKey(userID)
	return c.rdb.ZRem(ctx, key, requestID).Err()
}

func (c *concurrencyCache) GetUserConcurrency(ctx context.Context, userID int64) (int, error) {
	key := userSlotKey(userID)
	// 时间戳在 Lua 脚本内使用 Redis TIME 命令获取
	result, err := getCountScript.Run(ctx, c.rdb, []string{key}, c.slotTTLSeconds).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// Wait queue operations

func (c *concurrencyCache) IncrementWaitCount(ctx context.Context, userID int64, maxWait int) (bool, error) {
	userKey := waitQueueKey(userID)
	result, err := incrementWaitScript.Run(
		ctx,
		c.rdb,
		[]string{waitQueueTotalKey, waitQueueUpdatedKey, waitQueueCountsKey},
		userKey,
		maxWait,
		c.slotTTLSeconds,
		200, // cleanup limit per call
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *concurrencyCache) DecrementWaitCount(ctx context.Context, userID int64) error {
	userKey := waitQueueKey(userID)
	_, err := decrementWaitScript.Run(
		ctx,
		c.rdb,
		[]string{waitQueueTotalKey, waitQueueUpdatedKey, waitQueueCountsKey},
		userKey,
		c.slotTTLSeconds,
		200, // cleanup limit per call
	).Result()
	return err
}

func (c *concurrencyCache) GetTotalWaitCount(ctx context.Context) (int, error) {
	if c.rdb == nil {
		return 0, nil
	}
	total, err := getTotalWaitScript.Run(
		ctx,
		c.rdb,
		[]string{waitQueueTotalKey, waitQueueUpdatedKey, waitQueueCountsKey},
		c.slotTTLSeconds,
		500, // cleanup limit per query (rare)
	).Int64()
	if err != nil {
		return 0, err
	}
	return int(total), nil
}
