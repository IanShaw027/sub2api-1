package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const stickySessionPrefix = "sticky_session:"

type gatewayCache struct {
	rdb *redis.Client
}

func NewGatewayCache(rdb *redis.Client) service.GatewayCache {
	return &gatewayCache{rdb: rdb}
}

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, sessionHash string) (int64, error) {
	key := stickySessionPrefix + sessionHash
	val, err := c.rdb.Get(ctx, key).Int64()
	if err != nil {
		recordRedisError(ctx, "GatewayCache.GetSessionAccountID", err)
	}
	return val, err
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, sessionHash string, accountID int64, ttl time.Duration) error {
	key := stickySessionPrefix + sessionHash
	if err := c.rdb.Set(ctx, key, accountID, ttl).Err(); err != nil {
		recordRedisError(ctx, "GatewayCache.SetSessionAccountID", err)
		return err
	}
	return nil
}

func (c *gatewayCache) RefreshSessionTTL(ctx context.Context, sessionHash string, ttl time.Duration) error {
	key := stickySessionPrefix + sessionHash
	if err := c.rdb.Expire(ctx, key, ttl).Err(); err != nil {
		recordRedisError(ctx, "GatewayCache.RefreshSessionTTL", err)
		return err
	}
	return nil
}
