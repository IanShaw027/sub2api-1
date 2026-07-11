package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type slotCleanupCache struct {
	ConcurrencyCache
	calls atomic.Int64
}

func (c *slotCleanupCache) CleanupExpiredAccountSlotKeys(context.Context) error {
	c.calls.Add(1)
	return nil
}

type slotCleanupAccountRepo struct {
	AccountRepository
	calls atomic.Int64
}

func (r *slotCleanupAccountRepo) ListSchedulable(context.Context) ([]Account, error) {
	r.calls.Add(1)
	return nil, nil
}

func TestStartSlotCleanupWorker_UsesCacheWideCleanupWithoutAccountRepo(t *testing.T) {
	cache := &slotCleanupCache{}
	svc := NewConcurrencyService(cache)

	svc.StartSlotCleanupWorker(nil, time.Hour)

	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if cache.calls.Load() > 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("cleanup worker did not call cache-wide account slot cleanup")
		case <-ticker.C:
		}
	}
}

func TestStartSlotCleanupWorker_UsesCacheWideCleanupWithAccountRepo(t *testing.T) {
	cache := &slotCleanupCache{}
	repo := &slotCleanupAccountRepo{}
	svc := NewConcurrencyService(cache)

	svc.StartSlotCleanupWorker(repo, time.Hour)

	require.Eventually(t, func() bool {
		return cache.calls.Load() > 0 && repo.calls.Load() > 0
	}, time.Second, 10*time.Millisecond)
}
