//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayTryAcquireAccountSlot_NilServiceFailClosed(t *testing.T) {
	svc := &GatewayService{}
	result, err := svc.tryAcquireAccountSlot(context.Background(), 1, nil, 5)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrConcurrencyUnavailable)
	require.NotNil(t, result)
	require.False(t, result.Acquired)
}

func TestOpenAITryAcquireAccountSlot_NilServiceFailClosed(t *testing.T) {
	svc := &OpenAIGatewayService{}
	result, err := svc.tryAcquireAccountSlot(context.Background(), 1, nil, 5)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrConcurrencyUnavailable)
	require.NotNil(t, result)
	require.False(t, result.Acquired)
}

func TestSelectAccountWithLoadAwareness_WaitPlanUsesEffectiveConcurrency(t *testing.T) {
	ctx := context.Background()
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Priority:    1,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 0,
			},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}

	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 1

	concurrencyCache := &mockConcurrencyCache{
		acquireResults: map[int64]bool{1: false},
		waitCounts:     map[int64]int{1: 0},
	}

	svc := &GatewayService{
		accountRepo:        repo,
		cache:              &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky": 1}},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sticky", "claude-3-5-sonnet-20241022", nil, "", int64(0))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.WaitPlan)
	require.Equal(t, 12, result.WaitPlan.MaxConcurrency, "WaitPlan must admit with EffectiveConcurrency, not raw 0")
}
