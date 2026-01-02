package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const updateCacheKey = "update:latest"

type updateCache struct {
	rdb *redis.Client
}

func NewUpdateCache(rdb *redis.Client) service.UpdateCache {
	return &updateCache{rdb: rdb}
}

func (c *updateCache) GetUpdateInfo(ctx context.Context) (string, error) {
	val, err := c.rdb.Get(ctx, updateCacheKey).Result()
	if err != nil {
		recordRedisError(ctx, "UpdateCache.GetUpdateInfo", err)
	}
	return val, err
}

func (c *updateCache) SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error {
	if err := c.rdb.Set(ctx, updateCacheKey, data, ttl).Err(); err != nil {
		recordRedisError(ctx, "UpdateCache.SetUpdateInfo", err)
		return err
	}
	return nil
}
