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
	userCacheKeyPrefix = "user:"
	userCacheTTL       = 60 * time.Second
)

// userCacheKey generates the Redis key for user cache.
func userCacheKey(userID int64) string {
	return fmt.Sprintf("%s%d", userCacheKeyPrefix, userID)
}

// UserCache defines cache operations for users.
type UserCache interface {
	Get(ctx context.Context, userID int64) (*service.User, error)
	Set(ctx context.Context, user *service.User) error
	Delete(ctx context.Context, userID int64) error
}

type userCache struct {
	rdb *redis.Client
}

func NewUserCache(rdb *redis.Client) UserCache {
	return &userCache{rdb: rdb}
}

func (c *userCache) Get(ctx context.Context, userID int64) (*service.User, error) {
	key := userCacheKey(userID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var user service.User
	if err := json.Unmarshal([]byte(val), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *userCache) Set(ctx context.Context, user *service.User) error {
	if user == nil {
		return errors.New("user is nil")
	}
	key := userCacheKey(user.ID)
	val, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, userCacheTTL).Err()
}

func (c *userCache) Delete(ctx context.Context, userID int64) error {
	key := userCacheKey(userID)
	return c.rdb.Del(ctx, key).Err()
}
