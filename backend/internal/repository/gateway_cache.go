package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const stickySessionPrefix = "sticky_session:"
const openAIResponsesSessionWindowPrefix = "openai_responses_session_window:"

type gatewayCache struct {
	rdb *redis.Client
}

func NewGatewayCache(rdb *redis.Client) service.GatewayCache {
	return &gatewayCache{rdb: rdb}
}

// buildSessionKey 构建 session key，包含 groupID 实现分组隔离
// 格式: sticky_session:{groupID}:{sessionHash}
func buildSessionKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", stickySessionPrefix, groupID, sessionHash)
}

func buildOpenAIResponsesSessionWindowKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", openAIResponsesSessionWindowPrefix, groupID, sessionHash)
}

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	key := buildSessionKey(groupID, sessionHash)
	accountID, err := c.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, service.ErrGatewayCacheMiss
	}
	return accountID, err
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Set(ctx, key, accountID, ttl).Err()
}

func (c *gatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *gatewayCache) GetSessionAccountTTL(ctx context.Context, groupID int64, sessionHash string) (time.Duration, error) {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.PTTL(ctx, key).Result()
}

// DeleteSessionAccountID 删除粘性会话与账号的绑定关系。
// 当检测到绑定的账号不可用（如状态错误、禁用、不可调度等）时调用，
// 以便下次请求能够重新选择可用账号。
//
// DeleteSessionAccountID removes the sticky session binding for the given session.
// Called when the bound account becomes unavailable (e.g., error status, disabled,
// or unschedulable), allowing subsequent requests to select a new available account.
func (c *gatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Del(ctx, key).Err()
}

// Compile-time assertion: gatewayCache must implement CyberSessionBlockStore.
var _ service.CyberSessionBlockStore = (*gatewayCache)(nil)

const cyberSessionBlockPrefix = "cyber_session_block:"

// SetCyberSessionBlocked 把被 cyber_policy 命中的会话写入屏蔽表（TTL 自动过期）。
// 存储值 "1" 作为存在标记（IsCyberSessionBlocked 只检查 key 是否存在，不读值）。
func (c *gatewayCache) SetCyberSessionBlocked(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Set(ctx, cyberSessionBlockPrefix+key, "1", ttl).Err()
}

// IsCyberSessionBlocked 查询会话是否在屏蔽表中。
func (c *gatewayCache) IsCyberSessionBlocked(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, cyberSessionBlockPrefix+key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (c *gatewayCache) GetOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string) ([]byte, error) {
	key := buildOpenAIResponsesSessionWindowKey(groupID, sessionHash)
	payload, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, service.ErrGatewayCacheMiss
	}
	return payload, err
}

func (c *gatewayCache) SetOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string, payload []byte, ttl time.Duration) error {
	key := buildOpenAIResponsesSessionWindowKey(groupID, sessionHash)
	return c.rdb.Set(ctx, key, payload, ttl).Err()
}

func (c *gatewayCache) DeleteOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildOpenAIResponsesSessionWindowKey(groupID, sessionHash)
	return c.rdb.Del(ctx, key).Err()
}

// compareAndDeleteSessionWindowScript deletes the key only when its current
// value exactly matches ARGV[1]. Returns 1 on delete, 0 when missing/mismatched.
var compareAndDeleteSessionWindowScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current == false then
  return 0
end
if current == ARGV[1] then
  redis.call('DEL', KEYS[1])
  return 1
end
return 0
`)

// claimSessionWindowScript atomically returns the previous value and installs
// the new owner with a bounded lease. GET followed by SET is not sufficient:
// two contenders can otherwise overwrite each other in the opposite order.
var claimSessionWindowScript = redis.NewScript(`
local previous = redis.call('GET', KEYS[1])
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
return previous
`)

// compareAndRefreshSessionWindowScript refreshes the lease only while the
// caller still owns it. A stale turn must never extend a replacement owner's
// lease.
var compareAndRefreshSessionWindowScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current == false or current ~= ARGV[1] then
  return 0
end
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1
`)

// ClaimOpenAIResponsesSessionWindow atomically replaces the current WS
// preemption owner, returns the previous owner, and starts a fresh lease.
func (c *gatewayCache) ClaimOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string, owner []byte, ttl time.Duration) ([]byte, error) {
	if c == nil || c.rdb == nil {
		return nil, fmt.Errorf("gateway cache is unavailable")
	}
	if len(owner) == 0 {
		return nil, fmt.Errorf("session window claim owner is required")
	}
	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		return nil, fmt.Errorf("session window claim ttl must be positive")
	}
	key := buildOpenAIResponsesSessionWindowKey(groupID, sessionHash)
	result, err := claimSessionWindowScript.Run(ctx, c.rdb, []string{key}, owner, ttlMillis).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	switch value := result.(type) {
	case nil:
		return nil, nil
	case string:
		return []byte(value), nil
	case []byte:
		return append([]byte(nil), value...), nil
	default:
		return nil, fmt.Errorf("unexpected session window claim result %T", result)
	}
}

// CompareAndRefreshOpenAIResponsesSessionWindow extends a WS preemption lease
// only when expected is still the current owner.
func (c *gatewayCache) CompareAndRefreshOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string, expected []byte, ttl time.Duration) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, fmt.Errorf("gateway cache is unavailable")
	}
	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		return false, fmt.Errorf("session window refresh ttl must be positive")
	}
	key := buildOpenAIResponsesSessionWindowKey(groupID, sessionHash)
	n, err := compareAndRefreshSessionWindowScript.Run(ctx, c.rdb, []string{key}, expected, ttlMillis).Int()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// CompareAndDeleteOpenAIResponsesSessionWindow atomically releases a WS
// preemption owner token without racing a newer claim.
func (c *gatewayCache) CompareAndDeleteOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string, expected []byte) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, nil
	}
	key := buildOpenAIResponsesSessionWindowKey(groupID, sessionHash)
	n, err := compareAndDeleteSessionWindowScript.Run(ctx, c.rdb, []string{key}, expected).Int()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
