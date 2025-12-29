package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	apiKeyRateLimitKeyPrefix = "apikey:ratelimit:"
	apiKeyRateLimitDuration  = 24 * time.Hour
	apiKeyDataKeyPrefix      = "apikey:data:"
	apiKeyDataTTL            = 60 * time.Second
)

// apiKeyRateLimitKey generates the Redis key for API key creation rate limiting.
func apiKeyRateLimitKey(userID int64) string {
	return fmt.Sprintf("%s%d", apiKeyRateLimitKeyPrefix, userID)
}

func apiKeyDataKey(key string) string {
	return fmt.Sprintf("%s%s", apiKeyDataKeyPrefix, key)
}

type apiKeyCache struct {
	rdb *redis.Client
}

func NewApiKeyCache(rdb *redis.Client) service.ApiKeyCache {
	return &apiKeyCache{rdb: rdb}
}

func (c *apiKeyCache) GetCreateAttemptCount(ctx context.Context, userID int64) (int, error) {
	key := apiKeyRateLimitKey(userID)
	count, err := c.rdb.Get(ctx, key).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return count, err
}

func (c *apiKeyCache) IncrementCreateAttemptCount(ctx context.Context, userID int64) error {
	key := apiKeyRateLimitKey(userID)
	pipe := c.rdb.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, apiKeyRateLimitDuration)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *apiKeyCache) DeleteCreateAttemptCount(ctx context.Context, userID int64) error {
	key := apiKeyRateLimitKey(userID)
	return c.rdb.Del(ctx, key).Err()
}

func (c *apiKeyCache) IncrementDailyUsage(ctx context.Context, apiKey string) error {
	return c.rdb.Incr(ctx, apiKey).Err()
}

func (c *apiKeyCache) SetDailyUsageExpiry(ctx context.Context, apiKey string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, apiKey, ttl).Err()
}

func (c *apiKeyCache) GetByKey(ctx context.Context, key string) (*service.ApiKey, error) {
	cacheKey := apiKeyDataKey(key)
	val, err := c.rdb.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, err
	}
	var apiKey service.ApiKey
	if err := json.Unmarshal([]byte(val), &apiKey); err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (c *apiKeyCache) SetByKey(ctx context.Context, key string, apiKey *service.ApiKey) error {
	if apiKey == nil {
		return errors.New("api key is nil")
	}
	cacheKey := apiKeyDataKey(key)
	val, err := json.Marshal(apiKey)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, cacheKey, val, apiKeyDataTTL).Err()
}

func (c *apiKeyCache) DeleteByKey(ctx context.Context, key string) error {
	cacheKey := apiKeyDataKey(key)
	return c.rdb.Del(ctx, cacheKey).Err()
}
