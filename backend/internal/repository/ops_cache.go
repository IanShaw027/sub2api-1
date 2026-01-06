package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	opsLatestMetricsKey = "ops:metrics:latest"

	opsDashboardOverviewKeyPrefix = "ops:dashboard:overview:"

	opsLatestMetricsTTL = 10 * time.Second

	opsConcurrencyPlatformKey = "ops:concurrency:platform"
	opsConcurrencyGroupKey    = "ops:concurrency:group"
	opsConcurrencyCollectedAtKey = "ops:concurrency:collected_at_unix"
)

func (r *OpsRepository) GetCachedLatestSystemMetric(ctx context.Context) (*service.OpsMetrics, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.rdb == nil {
		return nil, nil
	}

	data, err := r.rdb.Get(ctx, opsLatestMetricsKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		recordRedisError(ctx, "OpsRepository.GetCachedLatestSystemMetric", err)
		return nil, fmt.Errorf("redis get cached latest system metric: %w", err)
	}

	var metric service.OpsMetrics
	if err := json.Unmarshal(data, &metric); err != nil {
		return nil, fmt.Errorf("unmarshal cached latest system metric: %w", err)
	}
	return &metric, nil
}

func (r *OpsRepository) SetCachedLatestSystemMetric(ctx context.Context, metric *service.OpsMetrics) error {
	if metric == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.rdb == nil {
		return nil
	}

	data, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("marshal cached latest system metric: %w", err)
	}
	if err := r.rdb.Set(ctx, opsLatestMetricsKey, data, opsLatestMetricsTTL).Err(); err != nil {
		recordRedisError(ctx, "OpsRepository.SetCachedLatestSystemMetric", err)
		return err
	}
	return nil
}

func (r *OpsRepository) GetCachedDashboardOverview(ctx context.Context, timeRange string) (*service.DashboardOverviewData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.rdb == nil {
		return nil, nil
	}
	rangeKey := strings.TrimSpace(timeRange)
	if rangeKey == "" {
		rangeKey = "1h"
	}

	key := opsDashboardOverviewKeyPrefix + rangeKey
	data, err := r.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		recordRedisError(ctx, "OpsRepository.GetCachedDashboardOverview", err)
		return nil, fmt.Errorf("redis get cached dashboard overview: %w", err)
	}

	var overview service.DashboardOverviewData
	if err := json.Unmarshal(data, &overview); err != nil {
		return nil, fmt.Errorf("unmarshal cached dashboard overview: %w", err)
	}
	return &overview, nil
}

func (r *OpsRepository) SetCachedDashboardOverview(ctx context.Context, timeRange string, data *service.DashboardOverviewData, ttl time.Duration) error {
	if data == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.rdb == nil {
		return nil
	}

	rangeKey := strings.TrimSpace(timeRange)
	if rangeKey == "" {
		rangeKey = "1h"
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal cached dashboard overview: %w", err)
	}
	key := opsDashboardOverviewKeyPrefix + rangeKey
	if err := r.rdb.Set(ctx, key, payload, ttl).Err(); err != nil {
		recordRedisError(ctx, "OpsRepository.SetCachedDashboardOverview", err)
		return err
	}
	return nil
}

func (r *OpsRepository) PingRedis(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.rdb == nil {
		return errors.New("redis client is nil")
	}
	if err := r.rdb.Ping(ctx).Err(); err != nil {
		recordRedisError(ctx, "OpsRepository.PingRedis", err)
		return err
	}
	return nil
}

func (r *OpsRepository) GetCachedPlatformConcurrency(ctx context.Context) (map[string]*service.PlatformConcurrencyInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.rdb == nil {
		return make(map[string]*service.PlatformConcurrencyInfo), nil
	}

	data, err := r.rdb.Get(ctx, opsConcurrencyPlatformKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return make(map[string]*service.PlatformConcurrencyInfo), nil
	}
	if err != nil {
		recordRedisError(ctx, "OpsRepository.GetCachedPlatformConcurrency", err)
		return make(map[string]*service.PlatformConcurrencyInfo), nil
	}

	var result map[string]*service.PlatformConcurrencyInfo
	if err := json.Unmarshal(data, &result); err != nil {
		return make(map[string]*service.PlatformConcurrencyInfo), nil
	}
	return result, nil
}

func (r *OpsRepository) GetCachedGroupConcurrency(ctx context.Context) (map[int64]*service.GroupConcurrencyInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.rdb == nil {
		return make(map[int64]*service.GroupConcurrencyInfo), nil
	}

	data, err := r.rdb.Get(ctx, opsConcurrencyGroupKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return make(map[int64]*service.GroupConcurrencyInfo), nil
	}
	if err != nil {
		recordRedisError(ctx, "OpsRepository.GetCachedGroupConcurrency", err)
		return make(map[int64]*service.GroupConcurrencyInfo), nil
	}

	var result map[int64]*service.GroupConcurrencyInfo
	if err := json.Unmarshal(data, &result); err != nil {
		return make(map[int64]*service.GroupConcurrencyInfo), nil
	}
	return result, nil
}

func (r *OpsRepository) GetCachedConcurrencyCollectedAt(ctx context.Context) (time.Time, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.rdb == nil {
		return time.Time{}, false, nil
	}

	val, err := r.rdb.Get(ctx, opsConcurrencyCollectedAtKey).Int64()
	if errors.Is(err, redis.Nil) {
		return time.Time{}, false, nil
	}
	if err != nil {
		recordRedisError(ctx, "OpsRepository.GetCachedConcurrencyCollectedAt", err)
		return time.Time{}, false, nil
	}
	if val <= 0 {
		return time.Time{}, false, nil
	}
	return time.Unix(val, 0).UTC(), true, nil
}
