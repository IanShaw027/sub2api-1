package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserFromServiceAdmin_MapsActivityTimestamps(t *testing.T) {
	t.Parallel()

	lastLoginAt := time.Date(2026, time.April, 20, 10, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(15 * time.Minute)
	lastUsedAt := lastLoginAt.Add(45 * time.Minute)

	out := UserFromServiceAdmin(&service.User{
		ID:           42,
		Email:        "admin@example.com",
		Username:     "admin",
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
		LastActiveAt: &lastActiveAt,
		LastUsedAt:   &lastUsedAt,
	})

	require.NotNil(t, out)
	require.NotNil(t, out.LastActiveAt)
	require.NotNil(t, out.LastUsedAt)
	require.WithinDuration(t, lastActiveAt, *out.LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *out.LastUsedAt, time.Second)
}

func TestUserFromServiceAdmin_MapsUsageStats(t *testing.T) {
	t.Parallel()

	out := UserFromServiceAdmin(&service.User{
		ID:                          42,
		Email:                       "admin@example.com",
		Username:                    "admin",
		Role:                        service.RoleAdmin,
		Status:                      service.StatusActive,
		TodayActualCost:             0.17,
		TodayBalanceActualCost:      0.12,
		TodaySubscriptionActualCost: 0.05,
		TotalActualCost:             1.23,
	})

	require.NotNil(t, out)
	require.Equal(t, 0.17, out.TodayActualCost)
	require.Equal(t, 0.12, out.TodayBalanceActualCost)
	require.Equal(t, 0.05, out.TodaySubscriptionActualCost)
	require.Equal(t, 1.23, out.TotalActualCost)
}
