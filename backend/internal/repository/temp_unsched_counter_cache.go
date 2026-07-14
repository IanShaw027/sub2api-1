package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const tempUnschedCounterPrefix = "temp_unsched_count:account:"

// tempUnschedCounterIncrScript 原子地增加计数并刷新过期时间，形成滑动窗口。
var tempUnschedCounterIncrScript = redis.NewScript(`
	local key = KEYS[1]
	local ttl = tonumber(ARGV[1])

	local count = redis.call('INCR', key)
	redis.call('EXPIRE', key, ttl)

	return count
`)

// tempUnschedCounterThresholdScript 原子地增加计数，达阈值时清零并返回触发标记。
var tempUnschedCounterThresholdScript = redis.NewScript(`
	local key = KEYS[1]
	local ttl = tonumber(ARGV[1])
	local threshold = tonumber(ARGV[2])

	local count = redis.call('INCR', key)
	redis.call('EXPIRE', key, ttl)
	if count >= threshold then
		redis.call('DEL', key)
		return {count, 1}
	end

	return {count, 0}
`)

type tempUnschedCounterCache struct {
	rdb *redis.Client
}

// NewTempUnschedCounterCache 创建临时不可调度规则窗口计数缓存实例。
func NewTempUnschedCounterCache(rdb *redis.Client) service.TempUnschedCounterCache {
	return &tempUnschedCounterCache{rdb: rdb}
}

func (c *tempUnschedCounterCache) counterKey(accountID int64, ruleFingerprint string) string {
	sum := sha256.Sum256([]byte(ruleFingerprint))
	return fmt.Sprintf("%s%d:rule_fingerprint:%s", tempUnschedCounterPrefix, accountID, hex.EncodeToString(sum[:]))
}

func (c *tempUnschedCounterCache) legacyCounterKeyPattern(accountID int64) string {
	return fmt.Sprintf("%s%d:rule:*", tempUnschedCounterPrefix, accountID)
}

// IncrementTempUnschedCount 增加某账户某条规则的命中计数，返回当前计数值。
func (c *tempUnschedCounterCache) IncrementTempUnschedCount(ctx context.Context, accountID int64, ruleFingerprint string, windowMinutes int) (int64, error) {
	key := c.counterKey(accountID, ruleFingerprint)

	ttlSeconds := windowMinutes * 60
	if ttlSeconds < 60 {
		ttlSeconds = 60 // 最小1分钟
	}

	result, err := tempUnschedCounterIncrScript.Run(ctx, c.rdb, []string{key}, ttlSeconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment temp unsched count: %w", err)
	}

	return result, nil
}

// IncrementTempUnschedThreshold 增加某账户某条规则的命中计数，并在达阈值时原子清零。
func (c *tempUnschedCounterCache) IncrementTempUnschedThreshold(ctx context.Context, accountID int64, ruleFingerprint string, windowMinutes int, thresholdCount int) (int64, bool, error) {
	key := c.counterKey(accountID, ruleFingerprint)

	ttlSeconds := windowMinutes * 60
	if ttlSeconds < 60 {
		ttlSeconds = 60 // 最小1分钟
	}
	if thresholdCount < 1 {
		thresholdCount = 1
	}

	result, err := tempUnschedCounterThresholdScript.Run(ctx, c.rdb, []string{key}, ttlSeconds, thresholdCount).Slice()
	if err != nil {
		return 0, false, fmt.Errorf("increment temp unsched threshold: %w", err)
	}
	if len(result) != 2 {
		return 0, false, fmt.Errorf("increment temp unsched threshold: unexpected result length %d", len(result))
	}
	count, ok := result[0].(int64)
	if !ok {
		return 0, false, fmt.Errorf("increment temp unsched threshold: unexpected count type %T", result[0])
	}
	reached, ok := result[1].(int64)
	if !ok {
		return 0, false, fmt.Errorf("increment temp unsched threshold: unexpected reached type %T", result[1])
	}

	return count, reached == 1, nil
}

func (c *tempUnschedCounterCache) ResetTempUnschedFingerprint(ctx context.Context, accountID int64, ruleFingerprint string) error {
	return c.rdb.Del(ctx, c.counterKey(accountID, ruleFingerprint)).Err()
}

// ResetTempUnschedCount 重置某账户某条规则的命中计数。
func (c *tempUnschedCounterCache) ResetTempUnschedCount(ctx context.Context, accountID int64, ruleFingerprint string) error {
	if err := c.ResetTempUnschedFingerprint(ctx, accountID, ruleFingerprint); err != nil {
		return err
	}

	var cursor uint64
	for {
		keys, nextCursor, err := c.rdb.Scan(ctx, cursor, c.legacyCounterKeyPattern(accountID), 100).Result()
		if err != nil {
			return fmt.Errorf("scan legacy temp unsched count keys: %w", err)
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("delete legacy temp unsched count keys: %w", err)
			}
		}
		if nextCursor == 0 {
			return nil
		}
		cursor = nextCursor
	}
}
