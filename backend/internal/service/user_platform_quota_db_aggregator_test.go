package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type quotaBatchCapableRepoStub struct {
	mu             sync.Mutex
	incrementCalls []UserPlatformQuotaUsageDelta
	batchCalls     [][]UserPlatformQuotaUsageDelta
	incrementErr   error
	batchErr       error
}

func (s *quotaBatchCapableRepoStub) GetByUserPlatform(context.Context, int64, string) (*UserPlatformQuotaRecord, error) {
	panic("unexpected GetByUserPlatform call")
}

func (s *quotaBatchCapableRepoStub) BulkInsertInitial(context.Context, []UserPlatformQuotaRecord) error {
	panic("unexpected BulkInsertInitial call")
}

func (s *quotaBatchCapableRepoStub) IncrementUsageWithReset(_ context.Context, userID int64, platform string, cost float64, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incrementCalls = append(s.incrementCalls, UserPlatformQuotaUsageDelta{
		UserID:   userID,
		Platform: platform,
		Cost:     cost,
	})
	return s.incrementErr
}

func (s *quotaBatchCapableRepoStub) ListByUser(context.Context, int64) ([]UserPlatformQuotaRecord, error) {
	panic("unexpected ListByUser call")
}

func (s *quotaBatchCapableRepoStub) UpsertForUser(context.Context, int64, []UserPlatformQuotaRecord) error {
	panic("unexpected UpsertForUser call")
}

func (s *quotaBatchCapableRepoStub) ResetExpiredWindow(context.Context, int64, string, string, time.Time) error {
	panic("unexpected ResetExpiredWindow call")
}

func (s *quotaBatchCapableRepoStub) BatchSnapshotUsage(context.Context, []UserPlatformQuotaSnapshot, time.Time) error {
	panic("unexpected BatchSnapshotUsage call")
}

func (s *quotaBatchCapableRepoStub) BatchIncrementUsageWithReset(_ context.Context, deltas []UserPlatformQuotaUsageDelta, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := append([]UserPlatformQuotaUsageDelta(nil), deltas...)
	s.batchCalls = append(s.batchCalls, copied)
	return s.batchErr
}

func (s *quotaBatchCapableRepoStub) snapshotIncrementCalls() []UserPlatformQuotaUsageDelta {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]UserPlatformQuotaUsageDelta(nil), s.incrementCalls...)
}

func (s *quotaBatchCapableRepoStub) snapshotBatchCalls() [][]UserPlatformQuotaUsageDelta {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]UserPlatformQuotaUsageDelta, 0, len(s.batchCalls))
	for _, call := range s.batchCalls {
		out = append(out, append([]UserPlatformQuotaUsageDelta(nil), call...))
	}
	return out
}

func TestUserPlatformQuotaDBAggregator_PrefersBatchIncrementWhenSupported(t *testing.T) {
	t.Parallel()

	repo := &quotaBatchCapableRepoStub{}
	agg := newUserPlatformQuotaDBAggregator(time.Hour)

	agg.enqueue(repo, 11, PlatformOpenAI, 1.25)
	agg.enqueue(repo, 12, PlatformGemini, 2.5)
	agg.flushPending()

	require.Empty(t, repo.snapshotIncrementCalls(), "batch-capable repo should not fall back to per-row increments")
	batchCalls := repo.snapshotBatchCalls()
	require.Len(t, batchCalls, 1)
	require.ElementsMatch(t, []UserPlatformQuotaUsageDelta{
		{UserID: 11, Platform: PlatformOpenAI, Cost: 1.25},
		{UserID: 12, Platform: PlatformGemini, Cost: 2.5},
	}, batchCalls[0])
}

func TestUserPlatformQuotaDBAggregator_StopFlushesPending(t *testing.T) {
	t.Parallel()

	repo := &quotaBatchCapableRepoStub{}
	agg := newUserPlatformQuotaDBAggregator(time.Hour)

	agg.enqueue(repo, 42, PlatformOpenAI, 3.5)
	agg.Stop()

	require.Empty(t, repo.snapshotIncrementCalls(), "batch-capable repo should flush via batch path on stop")
	batchCalls := repo.snapshotBatchCalls()
	require.Len(t, batchCalls, 1)
	require.ElementsMatch(t, []UserPlatformQuotaUsageDelta{
		{UserID: 42, Platform: PlatformOpenAI, Cost: 3.5},
	}, batchCalls[0])
}
