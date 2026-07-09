package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNewDashboardCacheKeyPrefix(t *testing.T) {
	cache := NewDashboardCache(nil, &config.Config{
		Dashboard: config.DashboardCacheConfig{
			KeyPrefix: "prod",
		},
	})
	impl, ok := cache.(*dashboardCache)
	require.True(t, ok)
	require.Equal(t, "prod:", impl.keyPrefix)

	cache = NewDashboardCache(nil, &config.Config{
		Dashboard: config.DashboardCacheConfig{
			KeyPrefix: "staging:",
		},
	})
	impl, ok = cache.(*dashboardCache)
	require.True(t, ok)
	require.Equal(t, "staging:", impl.keyPrefix)
}

func TestDashboardCacheGetWithNilClientReturnsCacheMiss(t *testing.T) {
	cache := NewDashboardCache(nil, nil)

	value, err := cache.GetDashboardStats(context.Background())

	require.ErrorIs(t, err, service.ErrDashboardStatsCacheMiss)
	require.Empty(t, value)
}

func TestDashboardCacheSetWithNilClientIsNoop(t *testing.T) {
	cache := NewDashboardCache(nil, nil)

	err := cache.SetDashboardStats(context.Background(), `{"stats":{}}`, time.Minute)

	require.NoError(t, err)
}

func TestDashboardCacheDeleteWithNilClientIsNoop(t *testing.T) {
	cache := NewDashboardCache(nil, nil)

	err := cache.DeleteDashboardStats(context.Background())

	require.NoError(t, err)
}
