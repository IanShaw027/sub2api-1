//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/require"
)

type blockingSubscriptionLoadRepo struct {
	userSubRepoNoop
	started chan struct{}
	release chan struct{}
	calls   atomic.Int64
	stale   *UserSubscription
	fresh   *UserSubscription
}

func (r *blockingSubscriptionLoadRepo) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	if r.calls.Add(1) == 1 {
		close(r.started)
		<-r.release
		return r.stale, nil
	}
	return r.fresh, nil
}

func TestGetActiveSubscriptionDoesNotRepopulateAfterConcurrentInvalidation(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config{NumCounters: 1_000, MaxCost: 100, BufferItems: 64})
	require.NoError(t, err)
	t.Cleanup(cache.Close)

	repo := &blockingSubscriptionLoadRepo{
		started: make(chan struct{}),
		release: make(chan struct{}),
		stale:   &UserSubscription{ID: 1, UserID: 501, GroupID: 7, Status: SubscriptionStatusActive},
		fresh:   &UserSubscription{ID: 2, UserID: 501, GroupID: 7, Status: SubscriptionStatusActive},
	}
	svc := &SubscriptionService{
		userSubRepo: repo,
		subCacheL1:  cache,
		subCacheTTL: time.Minute,
	}

	result := make(chan *UserSubscription, 1)
	errCh := make(chan error, 1)
	go func() {
		sub, loadErr := svc.GetActiveSubscription(context.Background(), 501, 7)
		result <- sub
		errCh <- loadErr
	}()

	<-repo.started
	svc.InvalidateSubCacheSync(501, 7)
	close(repo.release)

	require.NoError(t, <-errCh)
	got := <-result
	require.Equal(t, int64(2), got.ID, "the load overlapping invalidation must retry the DB")
	require.Equal(t, int64(2), repo.calls.Load())

	cached, ok := cache.Get(subCacheKey(501, 7))
	require.True(t, ok)
	require.Equal(t, int64(2), cached.(*UserSubscription).ID)
}

type fencedBillingCacheStub struct {
	billingCacheWorkerStub
	mu          sync.Mutex
	generation  int64
	setAttempts int
	setAccepted int
}

func (c *fencedBillingCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	return nil, errors.New("cache miss")
}

func (c *fencedBillingCacheStub) GetSubscriptionCacheGeneration(context.Context, int64, int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation, nil
}

func (c *fencedBillingCacheStub) SetSubscriptionCacheIfGeneration(_ context.Context, _, _ int64, _ *SubscriptionCacheData, expectedGeneration int64) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setAttempts++
	if expectedGeneration != c.generation {
		return false, nil
	}
	c.setAccepted++
	return true, nil
}

func (c *fencedBillingCacheStub) InvalidateSubscriptionCache(context.Context, int64, int64) error {
	c.mu.Lock()
	c.generation++
	c.mu.Unlock()
	return nil
}

type blockingBillingSubscriptionRepo struct {
	userSubRepoNoop
	started chan struct{}
	release chan struct{}
	calls   atomic.Int64
	stale   *UserSubscription
	fresh   *UserSubscription
}

func (r *blockingBillingSubscriptionRepo) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	if r.calls.Add(1) == 1 {
		close(r.started)
		<-r.release
		return r.stale, nil
	}
	return r.fresh, nil
}

func TestGetSubscriptionStatusFencesAsyncRedisFill(t *testing.T) {
	cache := &fencedBillingCacheStub{}
	repo := &blockingBillingSubscriptionRepo{
		started: make(chan struct{}),
		release: make(chan struct{}),
		stale: &UserSubscription{
			ID:        10,
			UserID:    601,
			GroupID:   8,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(time.Hour),
			UpdatedAt: time.Unix(10, 0),
		},
		fresh: &UserSubscription{
			ID:        11,
			UserID:    601,
			GroupID:   8,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(2 * time.Hour),
			UpdatedAt: time.Unix(11, 0),
		},
	}
	svc := NewBillingCacheService(cache, nil, repo, nil, nil, nil, nil, nil)

	resultCh := make(chan *subscriptionCacheData, 1)
	errCh := make(chan error, 1)
	go func() {
		data, loadErr := svc.GetSubscriptionStatus(context.Background(), 601, 8)
		resultCh <- data
		errCh <- loadErr
	}()
	<-repo.started
	require.NoError(t, cache.InvalidateSubscriptionCache(context.Background(), 601, 8))
	close(repo.release)
	require.NoError(t, <-errCh)
	require.Equal(t, int64(11), (<-resultCh).Version, "the current request must retry after overlapping invalidation")
	require.Equal(t, int64(2), repo.calls.Load())

	// Stop drains the cache worker queue before assertions.
	svc.Stop()
	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Equal(t, 1, cache.setAttempts)
	require.Equal(t, 1, cache.setAccepted, "the retried snapshot should populate under the new generation")
}
