//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type generationBalanceCacheStub struct {
	*billingCacheMissStub
	mu         sync.Mutex
	generation int64
	values     []float64
}

func newGenerationBalanceCacheStub() *generationBalanceCacheStub {
	return &generationBalanceCacheStub{billingCacheMissStub: &billingCacheMissStub{}}
}

func (s *generationBalanceCacheStub) GetBalanceCacheGeneration(context.Context, int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation, nil
}

func (s *generationBalanceCacheStub) SetUserBalanceIfGeneration(_ context.Context, _ int64, balance float64, expected int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generation != expected {
		return false, nil
	}
	s.values = append(s.values, balance)
	return true, nil
}

func (s *generationBalanceCacheStub) InvalidateUserBalance(context.Context, int64) error {
	s.mu.Lock()
	s.generation++
	s.mu.Unlock()
	return nil
}

func (s *generationBalanceCacheStub) cachedValues() []float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]float64(nil), s.values...)
}

type racingBalanceUserRepo struct {
	mockUserRepo
	mu           sync.Mutex
	balance      float64
	calls        int
	firstStarted chan struct{}
	releaseFirst chan struct{}
}

type balanceOutboxRepoStub struct {
	mu      sync.Mutex
	claimed bool
	acked   []int64
}

func (r *balanceOutboxRepoStub) Claim(context.Context, int, time.Duration) ([]BalanceCacheOutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed {
		return nil, nil
	}
	r.claimed = true
	return []BalanceCacheOutboxEvent{{ID: 11, UserID: 7}}, nil
}

func (r *balanceOutboxRepoStub) Ack(_ context.Context, ids []int64) error {
	r.mu.Lock()
	r.acked = append(r.acked, ids...)
	r.mu.Unlock()
	return nil
}

func (r *balanceOutboxRepoStub) Nack(context.Context, []int64, time.Duration, string) error {
	return nil
}

func (r *balanceOutboxRepoStub) ackedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.acked)
}

func (r *racingBalanceUserRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	r.mu.Lock()
	r.calls++
	call := r.calls
	captured := r.balance
	r.mu.Unlock()
	if call == 1 {
		close(r.firstStarted)
		select {
		case <-r.releaseFirst:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &User{ID: id, Balance: captured}, nil
}

func (r *racingBalanceUserRepo) setBalance(balance float64) {
	r.mu.Lock()
	r.balance = balance
	r.mu.Unlock()
}

type balanceEligibilityCacheStub struct {
	billingCacheWorkerStub

	balance                  float64
	cacheMissAfterInvalidate bool
	invalidated              atomic.Bool
	deductCalls              atomic.Int64
	invalidateCalls          atomic.Int64
}

func (s *balanceEligibilityCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	if s.cacheMissAfterInvalidate && s.invalidated.Load() {
		return 0, errors.New("cache miss")
	}
	return s.balance, nil
}

func (s *balanceEligibilityCacheStub) DeductUserBalance(context.Context, int64, float64) error {
	s.deductCalls.Add(1)
	return nil
}

func (s *balanceEligibilityCacheStub) InvalidateUserBalance(context.Context, int64) error {
	s.invalidateCalls.Add(1)
	s.invalidated.Store(true)
	return nil
}

func TestCheckBillingEligibility_RejectsBalanceBelowMinimumReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.005}
	userRepo := &balanceLoadUserRepoStub{balance: 0.005}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrInsufficientBalance)
}

func TestCheckBillingEligibility_AllowsBalanceAtMinimumReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.01}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.NoError(t, err)
}

func TestCheckBillingEligibility_ConfirmsCachedExhaustionAgainstDatabase(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0}
	userRepo := &balanceLoadUserRepoStub{balance: 25}
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")

	require.NoError(t, err)
	require.Equal(t, int64(1), userRepo.calls.Load())
}

func TestGetUserBalance_GenerationFenceRejectsPreRechargeRefill(t *testing.T) {
	cache := newGenerationBalanceCacheStub()
	userRepo := &racingBalanceUserRepo{
		balance:      0,
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	result := make(chan float64, 1)
	errCh := make(chan error, 1)
	go func() {
		balance, err := svc.GetUserBalance(context.Background(), 7)
		result <- balance
		errCh <- err
	}()

	<-userRepo.firstStarted
	userRepo.setBalance(10)
	require.NoError(t, cache.InvalidateUserBalance(context.Background(), 7))
	close(userRepo.releaseFirst)

	require.NoError(t, <-errCh)
	require.Equal(t, 10.0, <-result)
	require.Eventually(t, func() bool {
		return len(cache.cachedValues()) == 1
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, []float64{10}, cache.cachedValues())
}

func TestBalanceCacheOutboxConsumerInvalidatesAndAcknowledges(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 1}
	outbox := &balanceOutboxRepoStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	svc.SetBalanceCacheOutboxRepository(outbox)

	require.Eventually(t, func() bool {
		return cache.invalidateCalls.Load() == 1 && outbox.ackedCount() == 1
	}, time.Second, 10*time.Millisecond)
}

func TestSyncBalanceCacheAfterDeduction_InvalidatesExhaustedBalance(t *testing.T) {
	cache := &balanceEligibilityCacheStub{
		balance:                  0.50,
		cacheMissAfterInvalidate: true,
	}
	userRepo := &balanceLoadUserRepoStub{balance: -0.25}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := -0.25
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.75},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{
		NewBalance:         &newBalance,
		BalanceOverdrafted: true,
	})

	require.Equal(t, int64(1), cache.invalidateCalls.Load())
	require.Equal(t, int64(0), cache.deductCalls.Load())

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Equal(t, int64(2), userRepo.calls.Load())
}

func TestSyncBalanceCacheAfterDeduction_InvalidatesWhenBalanceFallsBelowReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.50}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := 0.005
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.495},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{NewBalance: &newBalance})

	require.Equal(t, int64(1), cache.invalidateCalls.Load())
	require.Equal(t, int64(0), cache.deductCalls.Load())
}

func TestSyncBalanceCacheAfterDeduction_QueuesDeductWhenBalanceStillEligible(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 1}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := 0.75
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.25},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{NewBalance: &newBalance})

	require.Equal(t, int64(0), cache.invalidateCalls.Load())
	require.Eventually(t, func() bool {
		return cache.deductCalls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)
}

func TestGetUserBalance_NilUserRepoReturnsError(t *testing.T) {
	svc := NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	_, err := svc.GetUserBalance(context.Background(), 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "user repository is unavailable")
}

func TestGetSubscriptionStatus_NilSubscriptionRepoReturnsError(t *testing.T) {
	svc := NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	_, err := svc.GetSubscriptionStatus(context.Background(), 1, 2)

	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription repository is unavailable")
}
