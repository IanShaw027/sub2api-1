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

// tempUnschedCounterIncrScript 原子地增加计数并在首次写入时设置过期时间。
var tempUnschedCounterIncrScript = redis.NewScript(`
	local key = KEYS[1]
	local ttl = tonumber(ARGV[1])

	local count = redis.call('INCR', key)
	if count == 1 then
		redis.call('EXPIRE', key, ttl)
	end

	return count
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

// ResetTempUnschedCount 重置某账户某条规则的命中计数。
func (c *tempUnschedCounterCache) ResetTempUnschedCount(ctx context.Context, accountID int64, ruleFingerprint string) error {
	keys := []string{c.counterKey(accountID, ruleFingerprint)}
	if legacyKeys, err := c.rdb.Keys(ctx, c.legacyCounterKeyPattern(accountID)).Result(); err == nil {
		keys = append(keys, legacyKeys...)
	}
	return c.rdb.Del(ctx, keys...).Err()
}
