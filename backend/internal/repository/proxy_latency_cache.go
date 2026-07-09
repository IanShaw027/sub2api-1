package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const proxyLatencyKeyPrefix = "proxy:latency:"

// 保留最近一次代理探测/质量检查结果的短期缓存，避免永久键无限增长。
const proxyLatencyKeyTTL = 15 * time.Minute

func proxyLatencyKey(proxyID int64) string {
	return fmt.Sprintf("%s%d", proxyLatencyKeyPrefix, proxyID)
}

type proxyLatencyCache struct {
	rdb *redis.Client
}

func NewProxyLatencyCache(rdb *redis.Client) service.ProxyLatencyCache {
	return &proxyLatencyCache{rdb: rdb}
}

func (c *proxyLatencyCache) GetProxyLatencies(ctx context.Context, proxyIDs []int64) (map[int64]*service.ProxyLatencyInfo, error) {
	results := make(map[int64]*service.ProxyLatencyInfo)
	if c == nil || c.rdb == nil || len(proxyIDs) == 0 {
		return results, nil
	}

	keys := make([]string, 0, len(proxyIDs))
	for _, id := range proxyIDs {
		keys = append(keys, proxyLatencyKey(id))
	}

	values, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return results, err
	}

	for i, raw := range values {
		if raw == nil {
			continue
		}
		var payload []byte
		switch v := raw.(type) {
		case string:
			payload = []byte(v)
		case []byte:
			payload = v
		default:
			continue
		}
		var info service.ProxyLatencyInfo
		if err := json.Unmarshal(payload, &info); err != nil {
			continue
		}
		results[proxyIDs[i]] = &info
	}

	return results, nil
}

func (c *proxyLatencyCache) SetProxyLatency(ctx context.Context, proxyID int64, info *service.ProxyLatencyInfo) error {
	if c == nil || c.rdb == nil || info == nil {
		return nil
	}
	key := proxyLatencyKey(proxyID)
	var lastErr error
	for range 5 {
		err := c.rdb.Watch(ctx, func(tx *redis.Tx) error {
			merged := *info

			raw, err := tx.Get(ctx, key).Bytes()
			switch err {
			case nil:
				var existing service.ProxyLatencyInfo
				if jsonErr := json.Unmarshal(raw, &existing); jsonErr == nil {
					mergeProxyLatencyInfo(&merged, &existing)
				}
			case redis.Nil:
			default:
				return err
			}

			payload, err := json.Marshal(&merged)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, payload, proxyLatencyKeyTTL)
				return nil
			})
			return err
		}, key)
		if err == nil {
			return nil
		}
		if err != redis.TxFailedErr {
			return err
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = redis.TxFailedErr
	}
	return lastErr
}

func mergeProxyLatencyInfo(next *service.ProxyLatencyInfo, existing *service.ProxyLatencyInfo) {
	if next == nil || existing == nil || proxyLatencyInfoHasQualitySnapshot(next) {
		return
	}
	next.QualityStatus = existing.QualityStatus
	next.QualityScore = existing.QualityScore
	next.QualityGrade = existing.QualityGrade
	next.QualitySummary = existing.QualitySummary
	next.QualityCheckedAt = existing.QualityCheckedAt
	next.QualityCFRay = existing.QualityCFRay
}

func proxyLatencyInfoHasQualitySnapshot(info *service.ProxyLatencyInfo) bool {
	if info == nil {
		return false
	}
	return info.QualityCheckedAt != nil ||
		info.QualityScore != nil ||
		strings.TrimSpace(info.QualityGrade) != "" ||
		strings.TrimSpace(info.QualityStatus) != "" ||
		strings.TrimSpace(info.QualitySummary) != "" ||
		strings.TrimSpace(info.QualityCFRay) != ""
}
