package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/redis/go-redis/v9"
)

const (
	accountTodayStatsKeyPrefix = "account:stats:today:"
	accountTodayStatsTTL       = 90 * time.Second
)

func accountTodayStatsKey(accountID int64) string {
	return fmt.Sprintf("%s%d", accountTodayStatsKeyPrefix, accountID)
}

// UsageLogCache defines cache operations for usage log statistics.
type UsageLogCache interface {
	GetAccountTodayStats(ctx context.Context, accountID int64) (*usagestats.AccountStats, error)
	SetAccountTodayStats(ctx context.Context, accountID int64, stats *usagestats.AccountStats) error
}

type usageLogCache struct {
	rdb *redis.Client
}

func NewUsageLogCache(rdb *redis.Client) UsageLogCache {
	return &usageLogCache{rdb: rdb}
}

func (c *usageLogCache) GetAccountTodayStats(ctx context.Context, accountID int64) (*usagestats.AccountStats, error) {
	key := accountTodayStatsKey(accountID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var stats usagestats.AccountStats
	if err := json.Unmarshal([]byte(val), &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (c *usageLogCache) SetAccountTodayStats(ctx context.Context, accountID int64, stats *usagestats.AccountStats) error {
	if stats == nil {
		return errors.New("stats is nil")
	}
	key := accountTodayStatsKey(accountID)
	val, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, accountTodayStatsTTL).Err()
}
