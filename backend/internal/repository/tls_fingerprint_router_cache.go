package repository

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	tlsFPRouterCacheKey  = "tls_fingerprint_routers"
	tlsFPRouterPubSubKey = "tls_fingerprint_routers_updated"
	tlsFPRouterCacheTTL  = 24 * time.Hour
)

type tlsFingerprintRouterCache struct {
	rdb        *redis.Client
	localCache []*model.TLSFingerprintRouter
	localMu    sync.RWMutex
}

// NewTLSFingerprintRouterCache 创建 TLS 指纹路由缓存。
func NewTLSFingerprintRouterCache(rdb *redis.Client) service.TLSFingerprintRouterCache {
	return &tlsFingerprintRouterCache{rdb: rdb}
}

func (c *tlsFingerprintRouterCache) Get(ctx context.Context) ([]*model.TLSFingerprintRouter, bool) {
	c.localMu.RLock()
	if c.localCache != nil {
		routers := c.localCache
		c.localMu.RUnlock()
		return routers, true
	}
	c.localMu.RUnlock()

	data, err := c.rdb.Get(ctx, tlsFPRouterCacheKey).Bytes()
	if err != nil {
		if err != redis.Nil {
			slog.Warn("tls_fp_router_cache_get_failed", "error", err)
		}
		return nil, false
	}

	var routers []*model.TLSFingerprintRouter
	if err := json.Unmarshal(data, &routers); err != nil {
		slog.Warn("tls_fp_router_cache_unmarshal_failed", "error", err)
		return nil, false
	}

	c.localMu.Lock()
	c.localCache = routers
	c.localMu.Unlock()
	return routers, true
}

func (c *tlsFingerprintRouterCache) Set(ctx context.Context, routers []*model.TLSFingerprintRouter) error {
	if routers == nil {
		routers = []*model.TLSFingerprintRouter{}
	}
	c.localMu.Lock()
	c.localCache = routers
	c.localMu.Unlock()

	data, err := json.Marshal(routers)
	if err != nil {
		return err
	}
	if err := c.rdb.Set(ctx, tlsFPRouterCacheKey, data, tlsFPRouterCacheTTL).Err(); err != nil {
		return err
	}
	return nil
}

func (c *tlsFingerprintRouterCache) Invalidate(ctx context.Context) error {
	c.localMu.Lock()
	c.localCache = nil
	c.localMu.Unlock()
	return c.rdb.Del(ctx, tlsFPRouterCacheKey).Err()
}

func (c *tlsFingerprintRouterCache) NotifyUpdate(ctx context.Context) error {
	return c.rdb.Publish(ctx, tlsFPRouterPubSubKey, "refresh").Err()
}

func (c *tlsFingerprintRouterCache) SubscribeUpdates(ctx context.Context, handler func()) {
	go func() {
		sub := c.rdb.Subscribe(ctx, tlsFPRouterPubSubKey)
		defer func() { _ = sub.Close() }()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-ch:
				if msg == nil {
					return
				}
				c.localMu.Lock()
				c.localCache = nil
				c.localMu.Unlock()
				handler()
			}
		}
	}()
}
