package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Cache key prefixes
	cachePrefixDashboard      = "ops:dashboard:"
	cachePrefixProviderHealth = "ops:provider_health:"

	// Default TTL for cache entries
	defaultCacheTTL = 10 * time.Second
)

// OpsCacheService handles Redis caching for ops monitoring data
type OpsCacheService struct {
	cache *redis.Client
}

// NewOpsCacheService creates a new cache service instance
func NewOpsCacheService(cache *redis.Client) *OpsCacheService {
	return &OpsCacheService{cache: cache}
}

// GetDashboardOverviewCache retrieves cached dashboard overview data
func (c *OpsCacheService) GetDashboardOverviewCache(ctx context.Context, timeRange string) (*DashboardOverviewData, error) {
	if c.cache == nil {
		return nil, fmt.Errorf("redis client is nil")
	}

	key := cachePrefixDashboard + timeRange
	data, err := c.cache.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		log.Printf("[OpsCache][WARN] Failed to get dashboard cache: %v", err)
		return nil, err
	}

	var result DashboardOverviewData
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		log.Printf("[OpsCache][WARN] Failed to unmarshal dashboard cache: %v", err)
		return nil, err
	}

	return &result, nil
}

// SetDashboardOverviewCache stores dashboard overview data in cache
func (c *OpsCacheService) SetDashboardOverviewCache(ctx context.Context, timeRange string, data *DashboardOverviewData, ttl time.Duration) error {
	if c.cache == nil {
		return fmt.Errorf("redis client is nil")
	}
	if data == nil {
		return fmt.Errorf("data is nil")
	}

	if ttl == 0 {
		ttl = defaultCacheTTL
	}

	key := cachePrefixDashboard + timeRange
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("[OpsCache][WARN] Failed to marshal dashboard data: %v", err)
		return err
	}

	if err := c.cache.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		log.Printf("[OpsCache][WARN] Failed to set dashboard cache: %v", err)
		return err
	}

	return nil
}

// GetProviderHealthCache retrieves cached provider health data
func (c *OpsCacheService) GetProviderHealthCache(ctx context.Context, timeRange string) ([]ProviderHealthData, error) {
	if c.cache == nil {
		return nil, fmt.Errorf("redis client is nil")
	}

	key := cachePrefixProviderHealth + timeRange
	data, err := c.cache.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		log.Printf("[OpsCache][WARN] Failed to get provider health cache: %v", err)
		return nil, err
	}

	var result []ProviderHealthData
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		log.Printf("[OpsCache][WARN] Failed to unmarshal provider health cache: %v", err)
		return nil, err
	}

	return result, nil
}

// SetProviderHealthCache stores provider health data in cache
func (c *OpsCacheService) SetProviderHealthCache(ctx context.Context, timeRange string, data []ProviderHealthData, ttl time.Duration) error {
	if c.cache == nil {
		return fmt.Errorf("redis client is nil")
	}
	if data == nil {
		return fmt.Errorf("data is nil")
	}

	if ttl == 0 {
		ttl = defaultCacheTTL
	}

	key := cachePrefixProviderHealth + timeRange
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("[OpsCache][WARN] Failed to marshal provider health data: %v", err)
		return err
	}

	if err := c.cache.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		log.Printf("[OpsCache][WARN] Failed to set provider health cache: %v", err)
		return err
	}

	return nil
}

// InvalidateDashboardCache removes dashboard cache for a specific time range
func (c *OpsCacheService) InvalidateDashboardCache(ctx context.Context, timeRange string) error {
	if c.cache == nil {
		return fmt.Errorf("redis client is nil")
	}

	key := cachePrefixDashboard + timeRange
	if err := c.cache.Del(ctx, key).Err(); err != nil {
		log.Printf("[OpsCache][WARN] Failed to invalidate dashboard cache: %v", err)
		return err
	}

	return nil
}

// InvalidateProviderHealthCache removes provider health cache for a specific time range
func (c *OpsCacheService) InvalidateProviderHealthCache(ctx context.Context, timeRange string) error {
	if c.cache == nil {
		return fmt.Errorf("redis client is nil")
	}

	key := cachePrefixProviderHealth + timeRange
	if err := c.cache.Del(ctx, key).Err(); err != nil {
		log.Printf("[OpsCache][WARN] Failed to invalidate provider health cache: %v", err)
		return err
	}

	return nil
}

// InvalidateAllOpsCache removes all ops-related cache entries
func (c *OpsCacheService) InvalidateAllOpsCache(ctx context.Context) error {
	if c.cache == nil {
		return fmt.Errorf("redis client is nil")
	}

	// Find all keys matching ops:* pattern
	iter := c.cache.Scan(ctx, 0, "ops:*", 0).Iterator()
	for iter.Next(ctx) {
		if err := c.cache.Del(ctx, iter.Val()).Err(); err != nil {
			log.Printf("[OpsCache][WARN] Failed to delete cache key %s: %v", iter.Val(), err)
		}
	}

	if err := iter.Err(); err != nil {
		log.Printf("[OpsCache][WARN] Failed to scan cache keys: %v", err)
		return err
	}

	return nil
}
