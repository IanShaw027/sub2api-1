//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type usageBillingRepoApplyStub struct {
	result *UsageBillingApplyResult
	err    error
	cmd    *UsageBillingCommand
}

func (s *usageBillingRepoApplyStub) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	s.cmd = cmd
	return s.result, s.err
}

func (s *usageBillingRepoApplyStub) ReserveBatchImageBalance(_ context.Context, _ *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

func (s *usageBillingRepoApplyStub) CaptureBatchImageBalance(_ context.Context, _ *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

func (s *usageBillingRepoApplyStub) ReleaseBatchImageBalance(_ context.Context, _ *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	return &BatchImageBalanceHoldResult{}, nil
}

func TestApplyUsageBilling_NotAppliedWithoutDeferredServiceDoesNotPanic(t *testing.T) {
	repo := &usageBillingRepoApplyStub{result: &UsageBillingApplyResult{Applied: false}}
	p := &postUsageBillingParams{
		Cost:    &CostBreakdown{},
		User:    &User{ID: 1},
		APIKey:  &APIKey{ID: 2},
		Account: &Account{ID: 3},
	}

	require.NotPanics(t, func() {
		applied, err := applyUsageBilling(context.Background(), "req-nil-deferred", nil, p, &billingDeps{}, repo)
		require.NoError(t, err)
		require.False(t, applied)
	})
}

func TestApplyUsageBillingPassesMinimumBalanceReserveToRepository(t *testing.T) {
	repo := &usageBillingRepoApplyStub{result: &UsageBillingApplyResult{Applied: false}}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	p := &postUsageBillingParams{
		Cost:    &CostBreakdown{ActualCost: 0.5},
		User:    &User{ID: 1},
		APIKey:  &APIKey{ID: 2},
		Account: &Account{ID: 3},
	}

	_, err := applyUsageBilling(context.Background(), "req-reserve", nil, p, &billingDeps{cfg: cfg}, repo)

	require.NoError(t, err)
	require.NotNil(t, repo.cmd)
	require.Equal(t, 0.01, repo.cmd.MinimumBalanceReserve)
}

func TestApplyUsageBillingBuildsAPIKeyQuotaWithoutLegacyUpdater(t *testing.T) {
	repo := &usageBillingRepoApplyStub{result: &UsageBillingApplyResult{Applied: false}}
	p := &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.5},
		User: &User{ID: 1},
		APIKey: &APIKey{
			ID:          2,
			Quota:       10,
			RateLimit5h: 5,
		},
		Account:  &Account{ID: 3},
		UsageLog: &UsageLog{RequestType: RequestTypeStream},
		// APIKeyService intentionally nil: the unified repository owns these
		// atomic counters; this is the AI Skill/internal-runtime path.
	}

	_, err := applyUsageBilling(context.Background(), "req-key-quota", p.UsageLog, p, &billingDeps{}, repo)

	require.NoError(t, err)
	require.NotNil(t, repo.cmd)
	require.Equal(t, 0.5, repo.cmd.APIKeyQuotaCost)
	require.Equal(t, 0.5, repo.cmd.APIKeyRateLimitCost)
}

func TestFinalizePostUsageBilling_WithoutDeferredServiceDoesNotPanic(t *testing.T) {
	p := &postUsageBillingParams{
		Cost:    &CostBreakdown{},
		User:    &User{ID: 1},
		APIKey:  &APIKey{ID: 2},
		Account: &Account{ID: 3},
	}

	require.NotPanics(t, func() {
		finalizePostUsageBilling(context.Background(), p, &billingDeps{}, &UsageBillingApplyResult{Applied: true})
	})
}

func TestFinalizePostUsageBilling_WithoutBillingCacheServiceSkipsPlatformQuotaSync(t *testing.T) {
	p := &postUsageBillingParams{
		Cost:     &CostBreakdown{ActualCost: 0.5},
		User:     &User{ID: 1},
		APIKey:   &APIKey{ID: 2},
		Account:  &Account{ID: 3},
		Platform: "openai",
	}
	deps := &billingDeps{
		deferredService:       &DeferredService{},
		userPlatformQuotaRepo: &fakeQuotaRepo{},
	}

	require.NotPanics(t, func() {
		finalizePostUsageBilling(context.Background(), p, deps, &UsageBillingApplyResult{Applied: true})
	})
}

func TestPostUsageBilling_WithoutBillingCacheServiceSkipsPlatformQuotaSync(t *testing.T) {
	userRepo := &openAIRecordUsageUserRepoStub{}
	p := &postUsageBillingParams{
		Cost:     &CostBreakdown{ActualCost: 0.5},
		User:     &User{ID: 1},
		APIKey:   &APIKey{ID: 2},
		Account:  &Account{ID: 3},
		Platform: "openai",
	}
	deps := &billingDeps{
		userRepo:              userRepo,
		userPlatformQuotaRepo: &fakeQuotaRepo{},
	}

	require.NotPanics(t, func() {
		postUsageBilling(context.Background(), p, deps)
	})
	require.Equal(t, 1, userRepo.deductCalls, "legacy balance deduction should still run")
}
