package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/redis/go-redis/v9"
)

const (
	geminiTokenKeyPrefix       = "gemini:token:"
	geminiRefreshLockKeyPrefix = "gemini:refresh_lock:"
)

type geminiTokenCache struct {
	rdb *redis.Client
}

func NewGeminiTokenCache(rdb *redis.Client) service.GeminiTokenCache {
	return &geminiTokenCache{rdb: rdb}
}

func (c *geminiTokenCache) GetAccessToken(ctx context.Context, cacheKey string) (string, error) {
	key := fmt.Sprintf("%s%s", geminiTokenKeyPrefix, cacheKey)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		recordRedisError(ctx, "GeminiTokenCache.GetAccessToken", err)
	}
	return val, err
}

func (c *geminiTokenCache) SetAccessToken(ctx context.Context, cacheKey string, token string, ttl time.Duration) error {
	key := fmt.Sprintf("%s%s", geminiTokenKeyPrefix, cacheKey)
	if err := c.rdb.Set(ctx, key, token, ttl).Err(); err != nil {
		recordRedisError(ctx, "GeminiTokenCache.SetAccessToken", err)
		return err
	}
	return nil
}

func (c *geminiTokenCache) AcquireRefreshLock(ctx context.Context, cacheKey string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("%s%s", geminiRefreshLockKeyPrefix, cacheKey)
	ok, err := c.rdb.SetNX(ctx, key, 1, ttl).Result()
	if err != nil {
		recordRedisError(ctx, "GeminiTokenCache.AcquireRefreshLock", err)
	}
	return ok, err
}

func (c *geminiTokenCache) ReleaseRefreshLock(ctx context.Context, cacheKey string) error {
	key := fmt.Sprintf("%s%s", geminiRefreshLockKeyPrefix, cacheKey)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		recordRedisError(ctx, "GeminiTokenCache.ReleaseRefreshLock", err)
		return err
	}
	return nil
}
