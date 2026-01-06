package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPlatformConcurrency(t *testing.T) {
	cache := &stubConcurrencyCache{}
	svc := NewConcurrencyService(cache)

	accounts := []AccountWithPlatform{
		{ID: 1, Platform: "anthropic", MaxConcurrency: 10},
		{ID: 2, Platform: "anthropic", MaxConcurrency: 5},
		{ID: 3, Platform: "openai", MaxConcurrency: 20},
	}

	cache.accountConcurrency = map[int64]int{1: 3, 2: 2, 3: 10}
	cache.accountWaiting = map[int64]int{1: 1, 2: 0, 3: 5}

	result, err := svc.GetPlatformConcurrency(context.Background(), accounts)
	require.NoError(t, err)
	require.Len(t, result, 2)

	anthropic := result["anthropic"]
	require.NotNil(t, anthropic)
	assert.Equal(t, int64(15), anthropic.MaxCapacity)
	assert.Equal(t, int64(5), anthropic.CurrentInUse)
	assert.Equal(t, int64(1), anthropic.WaitingInQueue)
	assert.InDelta(t, 33.33, anthropic.LoadPercentage, 0.01)

	openai := result["openai"]
	require.NotNil(t, openai)
	assert.Equal(t, int64(20), openai.MaxCapacity)
	assert.Equal(t, int64(10), openai.CurrentInUse)
	assert.Equal(t, int64(5), openai.WaitingInQueue)
	assert.Equal(t, 50.0, openai.LoadPercentage)
}

func TestGetGroupConcurrency(t *testing.T) {
	cache := &stubConcurrencyCache{}
	svc := NewConcurrencyService(cache)

	accounts := []AccountWithPlatform{
		{ID: 1, Platform: "anthropic", MaxConcurrency: 10, GroupID: 100, GroupName: "Group A"},
		{ID: 2, Platform: "anthropic", MaxConcurrency: 5, GroupID: 100, GroupName: "Group A"},
		{ID: 3, Platform: "openai", MaxConcurrency: 20, GroupID: 200, GroupName: "Group B"},
	}

	cache.accountConcurrency = map[int64]int{1: 8, 2: 3, 3: 15}
	cache.accountWaiting = map[int64]int{1: 2, 2: 1, 3: 0}

	result, err := svc.GetGroupConcurrency(context.Background(), accounts)
	require.NoError(t, err)
	require.Len(t, result, 2)

	groupA := result[100]
	require.NotNil(t, groupA)
	assert.Equal(t, int64(100), groupA.GroupID)
	assert.Equal(t, "Group A", groupA.GroupName)
	assert.Equal(t, int64(15), groupA.MaxCapacity)
	assert.Equal(t, int64(11), groupA.CurrentInUse)
	assert.Equal(t, int64(3), groupA.WaitingInQueue)
	assert.InDelta(t, 73.33, groupA.LoadPercentage, 0.01)

	groupB := result[200]
	require.NotNil(t, groupB)
	assert.Equal(t, int64(200), groupB.GroupID)
	assert.Equal(t, "Group B", groupB.GroupName)
	assert.Equal(t, int64(20), groupB.MaxCapacity)
	assert.Equal(t, int64(15), groupB.CurrentInUse)
	assert.Equal(t, int64(0), groupB.WaitingInQueue)
	assert.Equal(t, 75.0, groupB.LoadPercentage)
}

func TestHealthEvaluator_Healthy(t *testing.T) {
	evaluator := NewHealthEvaluator()
	metrics := &OpsMetrics{
		LatencyP95:            1000,
		ErrorRate:             0.5,
		ConcurrencyQueueDepth: 100,
	}
	health := evaluator.Evaluate(metrics, nil)
	assert.Equal(t, HealthStatusHealthy, health.Status)
	assert.True(t, health.Healthy)
	assert.Empty(t, health.Warnings)
}

func TestHealthEvaluator_Degraded(t *testing.T) {
	evaluator := NewHealthEvaluator()
	metrics := &OpsMetrics{
		LatencyP95: 70000,
		ErrorRate:  0.5,
	}
	health := evaluator.Evaluate(metrics, nil)
	assert.Equal(t, HealthStatusDegraded, health.Status)
	assert.False(t, health.Healthy)
	assert.NotEmpty(t, health.Warnings)
}

func TestHealthEvaluator_Unhealthy(t *testing.T) {
	evaluator := NewHealthEvaluator()
	metrics := &OpsMetrics{
		ErrorRate: 6,
	}
	health := evaluator.Evaluate(metrics, nil)
	assert.Equal(t, HealthStatusUnhealthy, health.Status)
	assert.False(t, health.Healthy)
	assert.Contains(t, health.Warnings, "错误率超过5%")
}

type stubConcurrencyCache struct {
	accountConcurrency map[int64]int
	accountWaiting     map[int64]int
}

func (c *stubConcurrencyCache) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
	return true, nil
}

func (c *stubConcurrencyCache) ReleaseAccountSlot(ctx context.Context, accountID int64, requestID string) error {
	return nil
}

func (c *stubConcurrencyCache) GetAccountConcurrency(ctx context.Context, accountID int64) (int, error) {
	if count, ok := c.accountConcurrency[accountID]; ok {
		return count, nil
	}
	return 0, nil
}

func (c *stubConcurrencyCache) IncrementAccountWaitCount(ctx context.Context, accountID int64, maxWait int) (bool, error) {
	return true, nil
}

func (c *stubConcurrencyCache) DecrementAccountWaitCount(ctx context.Context, accountID int64) error {
	return nil
}

func (c *stubConcurrencyCache) GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error) {
	if count, ok := c.accountWaiting[accountID]; ok {
		return count, nil
	}
	return 0, nil
}

func (c *stubConcurrencyCache) AcquireUserSlot(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
	return true, nil
}

func (c *stubConcurrencyCache) ReleaseUserSlot(ctx context.Context, userID int64, requestID string) error {
	return nil
}

func (c *stubConcurrencyCache) GetUserConcurrency(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

func (c *stubConcurrencyCache) IncrementUserWaitCount(ctx context.Context, userID int64, maxWait int) (bool, error) {
	return true, nil
}

func (c *stubConcurrencyCache) DecrementUserWaitCount(ctx context.Context, userID int64) error {
	return nil
}

func (c *stubConcurrencyCache) GetUserWaitingCount(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

func (c *stubConcurrencyCache) CleanupExpiredAccountSlots(ctx context.Context, accountID int64) error {
	return nil
}

func (c *stubConcurrencyCache) IncrementWaitCount(ctx context.Context, userID int64, maxWait int) (bool, error) {
	return true, nil
}

func (c *stubConcurrencyCache) DecrementWaitCount(ctx context.Context, userID int64) error {
	return nil
}

func (c *stubConcurrencyCache) GetTotalWaitCount(ctx context.Context) (int, error) {
	return 0, nil
}

func (c *stubConcurrencyCache) GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	loadMap := make(map[int64]*AccountLoadInfo, len(accounts))
	for _, acc := range accounts {
		current := 0
		if c.accountConcurrency != nil {
			current = c.accountConcurrency[acc.ID]
		}
		waiting := 0
		if c.accountWaiting != nil {
			waiting = c.accountWaiting[acc.ID]
		}
		loadRate := 0
		if acc.MaxConcurrency > 0 {
			loadRate = int(float64(current) / float64(acc.MaxConcurrency) * 100)
		}
		loadMap[acc.ID] = &AccountLoadInfo{
			AccountID:          acc.ID,
			CurrentConcurrency: current,
			WaitingCount:       waiting,
			LoadRate:           loadRate,
		}
	}
	return loadMap, nil
}
