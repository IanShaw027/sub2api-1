//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type usageBillingRepoApplyStub struct {
	result *UsageBillingApplyResult
	err    error
}

func (s *usageBillingRepoApplyStub) Apply(_ context.Context, _ *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	return s.result, s.err
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
