//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newProxyLatencyCacheTest(t *testing.T) (*proxyLatencyCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &proxyLatencyCache{rdb: rdb}, mr
}

func TestProxyLatencyCacheSetAppliesTTL(t *testing.T) {
	cache, mr := newProxyLatencyCacheTest(t)
	ctx := context.Background()
	latency := int64(123)

	require.NoError(t, cache.SetProxyLatency(ctx, 7, &service.ProxyLatencyInfo{
		Success:   true,
		LatencyMs: &latency,
		UpdatedAt: time.Unix(1_700_000_000, 0),
	}))

	ttl := mr.TTL(proxyLatencyKey(7))
	require.Greater(t, ttl, time.Duration(0), "proxy latency cache key should expire")
}

func TestProxyLatencyCacheSetPreservesExistingQualityFieldsWhenLatencyOnlyUpdateArrives(t *testing.T) {
	cache, _ := newProxyLatencyCacheTest(t)
	ctx := context.Background()

	firstLatency := int64(120)
	qualityScore := 88
	qualityCheckedAt := int64(1_700_000_123)
	require.NoError(t, cache.SetProxyLatency(ctx, 7, &service.ProxyLatencyInfo{
		Success:          true,
		LatencyMs:        &firstLatency,
		IPAddress:        "1.2.3.4",
		Country:          "US",
		CountryCode:      "US",
		QualityStatus:    "good",
		QualityScore:     &qualityScore,
		QualityGrade:     "A",
		QualitySummary:   "stable",
		QualityCheckedAt: &qualityCheckedAt,
		QualityCFRay:     "ray-1",
		UpdatedAt:        time.Unix(1_700_000_000, 0),
	}))

	secondLatency := int64(80)
	require.NoError(t, cache.SetProxyLatency(ctx, 7, &service.ProxyLatencyInfo{
		Success:   true,
		LatencyMs: &secondLatency,
		UpdatedAt: time.Unix(1_700_000_100, 0),
	}))

	latencies, err := cache.GetProxyLatencies(ctx, []int64{7})
	require.NoError(t, err)
	info := latencies[7]
	require.NotNil(t, info)
	require.NotNil(t, info.LatencyMs)
	require.Equal(t, secondLatency, *info.LatencyMs)
	require.Equal(t, "good", info.QualityStatus)
	require.NotNil(t, info.QualityScore)
	require.Equal(t, qualityScore, *info.QualityScore)
	require.Equal(t, "A", info.QualityGrade)
	require.Equal(t, "stable", info.QualitySummary)
	require.NotNil(t, info.QualityCheckedAt)
	require.Equal(t, qualityCheckedAt, *info.QualityCheckedAt)
	require.Equal(t, "ray-1", info.QualityCFRay)
}

func TestProxyLatencyCacheGetWithNilClientReturnsEmptyResult(t *testing.T) {
	cache := &proxyLatencyCache{}

	latencies, err := cache.GetProxyLatencies(context.Background(), []int64{7, 8})

	require.NoError(t, err)
	require.Empty(t, latencies)
}

func TestProxyLatencyCacheSetWithNilClientIsNoop(t *testing.T) {
	cache := &proxyLatencyCache{}
	latency := int64(42)

	err := cache.SetProxyLatency(context.Background(), 7, &service.ProxyLatencyInfo{
		Success:   true,
		LatencyMs: &latency,
		UpdatedAt: time.Unix(1_700_000_000, 0),
	})

	require.NoError(t, err)
}
