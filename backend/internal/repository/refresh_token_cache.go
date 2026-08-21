package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	refreshTokenKeyPrefix   = "refresh_token:"
	userRefreshTokensPrefix = "user_refresh_tokens:"
	tokenFamilyPrefix       = "token_family:"
	consumedTokenKeyPrefix  = "consumed_refresh_token:"
	refreshRetryKeyPrefix   = "refresh_token_retry:"
	userAuthEpochKeyPrefix  = "user_auth_epoch:"
)

// refreshTokenKey generates the Redis key for a refresh token.
func refreshTokenKey(tokenHash string) string {
	return refreshTokenKeyPrefix + tokenHash
}

// userRefreshTokensKey generates the Redis key for user's token set.
func userRefreshTokensKey(userID int64) string {
	return fmt.Sprintf("%s%d", userRefreshTokensPrefix, userID)
}

// tokenFamilyKey generates the Redis key for token family set.
func tokenFamilyKey(familyID string) string {
	return tokenFamilyPrefix + familyID
}

func consumedTokenKey(tokenHash string) string { return consumedTokenKeyPrefix + tokenHash }
func refreshRetryKey(tokenHash string) string  { return refreshRetryKeyPrefix + tokenHash }
func userAuthEpochKey(userID int64) string {
	return fmt.Sprintf("%s%d", userAuthEpochKeyPrefix, userID)
}

type refreshTokenCache struct {
	rdb *redis.Client
}

// NewRefreshTokenCache creates a new RefreshTokenCache implementation.
func NewRefreshTokenCache(rdb *redis.Client) service.RefreshTokenCache {
	return &refreshTokenCache{rdb: rdb}
}

func (c *refreshTokenCache) StoreRefreshToken(ctx context.Context, tokenHash string, data *service.RefreshTokenData, ttl time.Duration) error {
	val, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal refresh token data: %w", err)
	}
	const script = `
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SADD', KEYS[2], ARGV[3])
redis.call('PEXPIRE', KEYS[2], ARGV[2])
redis.call('SADD', KEYS[3], ARGV[3])
redis.call('PEXPIRE', KEYS[3], ARGV[2])
return 1`
	return c.rdb.Eval(ctx, script, []string{
		refreshTokenKey(tokenHash),
		userRefreshTokensKey(data.UserID),
		tokenFamilyKey(data.FamilyID),
	}, string(val), ttl.Milliseconds(), tokenHash).Err()
}

func (c *refreshTokenCache) RotateRefreshToken(
	ctx context.Context,
	oldTokenHash string,
	newTokenHash string,
	newData *service.RefreshTokenData,
	newTTL time.Duration,
	retryTTL time.Duration,
	response string,
) (*service.RefreshTokenRotationResult, error) {
	newValue, err := json.Marshal(newData)
	if err != nil {
		return nil, fmt.Errorf("marshal refresh token data: %w", err)
	}
	const script = `
local old = redis.call('GET', KEYS[1])
if old then
  local old_ttl = redis.call('PTTL', KEYS[1])
  if old_ttl < 1 then old_ttl = tonumber(ARGV[2]) end
  redis.call('SET', KEYS[2], ARGV[1], 'PX', ARGV[2])
	redis.call('SREM', KEYS[3], ARGV[6])
  redis.call('SADD', KEYS[3], ARGV[3])
  redis.call('PEXPIRE', KEYS[3], ARGV[2])
	redis.call('SREM', KEYS[4], ARGV[6])
  redis.call('SADD', KEYS[4], ARGV[3])
  redis.call('PEXPIRE', KEYS[4], ARGV[2])
  redis.call('DEL', KEYS[1])
  redis.call('SET', KEYS[5], old, 'PX', old_ttl)
  redis.call('SET', KEYS[6], ARGV[4], 'PX', ARGV[5])
  return {1, ARGV[4]}
end
local retry = redis.call('GET', KEYS[6])
if retry then return {2, retry} end
local consumed = redis.call('GET', KEYS[5])
if consumed then return {3, consumed} end
return {0, ''}`
	result, err := c.rdb.Eval(ctx, script, []string{
		refreshTokenKey(oldTokenHash),
		refreshTokenKey(newTokenHash),
		userRefreshTokensKey(newData.UserID),
		tokenFamilyKey(newData.FamilyID),
		consumedTokenKey(oldTokenHash),
		refreshRetryKey(oldTokenHash),
	}, string(newValue), newTTL.Milliseconds(), newTokenHash, response, retryTTL.Milliseconds(), oldTokenHash).Slice()
	if err != nil {
		return nil, err
	}
	return decodeRotationResult(result)
}

func (c *refreshTokenCache) GetRefreshTokenRotationState(ctx context.Context, tokenHash string) (*service.RefreshTokenRotationResult, error) {
	const script = `
local retry = redis.call('GET', KEYS[1])
if retry then return {2, retry} end
local consumed = redis.call('GET', KEYS[2])
if consumed then return {3, consumed} end
return {0, ''}`
	result, err := c.rdb.Eval(ctx, script, []string{refreshRetryKey(tokenHash), consumedTokenKey(tokenHash)}).Slice()
	if err != nil {
		return nil, err
	}
	return decodeRotationResult(result)
}

func decodeRotationResult(values []any) (*service.RefreshTokenRotationResult, error) {
	if len(values) != 2 {
		return nil, fmt.Errorf("unexpected refresh rotation result length: %d", len(values))
	}
	statusValue, ok := values[0].(int64)
	if !ok {
		return nil, fmt.Errorf("unexpected refresh rotation status type %T", values[0])
	}
	payload, _ := values[1].(string)
	result := &service.RefreshTokenRotationResult{Status: service.RefreshTokenRotationStatus(statusValue)}
	switch result.Status {
	case service.RefreshTokenRotationSucceeded, service.RefreshTokenRotationRetry:
		result.Response = payload
	case service.RefreshTokenRotationReused:
		var data service.RefreshTokenData
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			return nil, fmt.Errorf("unmarshal consumed refresh token data: %w", err)
		}
		result.TokenData = &data
	case service.RefreshTokenRotationMissing:
	default:
		return nil, fmt.Errorf("unexpected refresh rotation status: %d", statusValue)
	}
	return result, nil
}

func (c *refreshTokenCache) GetUserAuthEpoch(ctx context.Context, userID int64) (string, error) {
	epoch, err := c.rdb.Get(ctx, userAuthEpochKey(userID)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return epoch, err
}

func (c *refreshTokenCache) EnsureUserAuthEpoch(ctx context.Context, userID int64, candidate string, ttl time.Duration) (string, error) {
	const script = `
local current = redis.call('GET', KEYS[1])
if current then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
  return current
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2], 'NX')
return redis.call('GET', KEYS[1])`
	return c.rdb.Eval(ctx, script, []string{userAuthEpochKey(userID)}, candidate, ttl.Milliseconds()).Text()
}

func (c *refreshTokenCache) RevokeUserSessions(ctx context.Context, userID int64, authEpoch string, authEpochTTL time.Duration) error {
	const script = `
local hashes = redis.call('SMEMBERS', KEYS[1])
for _, hash in ipairs(hashes) do
  redis.call('DEL', ARGV[3] .. hash, ARGV[4] .. hash, ARGV[5] .. hash)
end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], ARGV[1], 'PX', ARGV[2])
return #hashes`
	return c.rdb.Eval(ctx, script, []string{
		userRefreshTokensKey(userID),
		userAuthEpochKey(userID),
	}, authEpoch, authEpochTTL.Milliseconds(), refreshTokenKeyPrefix, consumedTokenKeyPrefix, refreshRetryKeyPrefix).Err()
}

func (c *refreshTokenCache) GetRefreshToken(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	key := refreshTokenKey(tokenHash)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrRefreshTokenNotFound
		}
		return nil, err
	}
	var data service.RefreshTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("unmarshal refresh token data: %w", err)
	}
	return &data, nil
}

func (c *refreshTokenCache) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	const script = `
local data = redis.call('GET', KEYS[1])
if not data then data = redis.call('GET', KEYS[2]) end
if data then
  local ok, decoded = pcall(cjson.decode, data)
  if ok and type(decoded) == 'table' and decoded.user_id then
    redis.call('SREM', ARGV[1] .. decoded.user_id, ARGV[3])
  end
  if ok and type(decoded) == 'table' and decoded.family_id then
    redis.call('SREM', ARGV[2] .. decoded.family_id, ARGV[3])
  end
end
redis.call('DEL', KEYS[1], KEYS[2], KEYS[3])
return 1`
	return c.rdb.Eval(ctx, script, []string{
		refreshTokenKey(tokenHash),
		consumedTokenKey(tokenHash),
		refreshRetryKey(tokenHash),
	}, userRefreshTokensPrefix, tokenFamilyPrefix, tokenHash).Err()
}

func (c *refreshTokenCache) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	const script = `
local hashes = redis.call('SMEMBERS', KEYS[1])
for _, hash in ipairs(hashes) do
  redis.call('DEL', ARGV[1] .. hash, ARGV[2] .. hash, ARGV[3] .. hash)
end
redis.call('DEL', KEYS[1])
return #hashes`
	return c.rdb.Eval(ctx, script, []string{userRefreshTokensKey(userID)},
		refreshTokenKeyPrefix, consumedTokenKeyPrefix, refreshRetryKeyPrefix).Err()
}

func (c *refreshTokenCache) DeleteTokenFamily(ctx context.Context, familyID string) error {
	const script = `
local hashes = redis.call('SMEMBERS', KEYS[1])
for _, hash in ipairs(hashes) do
	  local data = redis.call('GET', ARGV[1] .. hash)
	  if not data then data = redis.call('GET', ARGV[2] .. hash) end
	  if data then
	    local ok, decoded = pcall(cjson.decode, data)
	    if ok and type(decoded) == 'table' and decoded.user_id then
	      redis.call('SREM', ARGV[5] .. decoded.user_id, hash)
	    end
	  end
  redis.call('DEL', ARGV[1] .. hash, ARGV[2] .. hash, ARGV[3] .. hash)
end
redis.call('DEL', KEYS[1])
return #hashes`
	return c.rdb.Eval(ctx, script, []string{tokenFamilyKey(familyID)},
		refreshTokenKeyPrefix, consumedTokenKeyPrefix, refreshRetryKeyPrefix, tokenFamilyPrefix, userRefreshTokensPrefix).Err()
}

func (c *refreshTokenCache) AddToUserTokenSet(ctx context.Context, userID int64, tokenHash string, ttl time.Duration) error {
	key := userRefreshTokensKey(userID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) AddToFamilyTokenSet(ctx context.Context, familyID string, tokenHash string, ttl time.Duration) error {
	key := tokenFamilyKey(familyID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) GetUserTokenHashes(ctx context.Context, userID int64) ([]string, error) {
	key := userRefreshTokensKey(userID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) GetFamilyTokenHashes(ctx context.Context, familyID string) ([]string, error) {
	key := tokenFamilyKey(familyID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) IsTokenInFamily(ctx context.Context, familyID string, tokenHash string) (bool, error) {
	key := tokenFamilyKey(familyID)
	return c.rdb.SIsMember(ctx, key, tokenHash).Result()
}
