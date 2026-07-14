package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type outboxClaimCache struct {
	updateErr       error
	lockAvailable   bool
	buckets         []SchedulerBucket
	listBucketCalls int
}

func (c *outboxClaimCache) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}
func (c *outboxClaimCache) SetSnapshot(context.Context, SchedulerBucket, []Account) error { return nil }
func (c *outboxClaimCache) GetAccount(context.Context, int64) (*Account, error)           { return nil, nil }
func (c *outboxClaimCache) SetAccount(context.Context, *Account) error                    { return nil }
func (c *outboxClaimCache) DeleteAccount(context.Context, int64) error                    { return nil }
func (c *outboxClaimCache) UpdateLastUsed(context.Context, map[int64]time.Time) error {
	return c.updateErr
}
func (c *outboxClaimCache) TryLockBucket(context.Context, SchedulerBucket, time.Duration) (bool, error) {
	return c.lockAvailable, nil
}
func (c *outboxClaimCache) UnlockBucket(context.Context, SchedulerBucket) error { return nil }
func (c *outboxClaimCache) ListBuckets(context.Context) ([]SchedulerBucket, error) {
	c.listBucketCalls++
	return c.buckets, nil
}
func (c *outboxClaimCache) GetOutboxWatermark(context.Context) (int64, error) { return 0, nil }
func (c *outboxClaimCache) SetOutboxWatermark(context.Context, int64) error   { return nil }

type outboxClaimRepo struct {
	events             []SchedulerOutboxEvent
	acked              []int64
	released           []int64
	claimLease         []time.Duration
	oldestPendingCalls int
}

func (r *outboxClaimRepo) ClaimPending(_ context.Context, lease time.Duration) (*SchedulerOutboxEvent, error) {
	r.claimLease = append(r.claimLease, lease)
	if len(r.events) == 0 {
		return nil, nil
	}
	event := r.events[0]
	r.events = r.events[1:]
	if event.ClaimToken == "" {
		event.ClaimToken = "claim-token"
	}
	return &event, nil
}

func (r *outboxClaimRepo) AckClaim(_ context.Context, eventID int64, claimToken string) (bool, error) {
	if claimToken == "" {
		return false, nil
	}
	r.acked = append(r.acked, eventID)
	return true, nil
}

func (r *outboxClaimRepo) ReleaseClaim(_ context.Context, eventID int64, _ string) error {
	r.released = append(r.released, eventID)
	return nil
}

func (r *outboxClaimRepo) OldestPendingCreatedAt(context.Context) (time.Time, bool, error) {
	r.oldestPendingCalls++
	if len(r.events) == 0 {
		return time.Time{}, false, nil
	}
	return r.events[0].CreatedAt, true, nil
}

func (r *outboxClaimRepo) PendingCount(context.Context) (int64, error) {
	return int64(len(r.events)), nil
}

func TestSchedulerSnapshotServicePollOutboxAcknowledgesProcessedClaims(t *testing.T) {
	cache := &outboxClaimCache{lockAvailable: true}
	repo := &outboxClaimRepo{events: []SchedulerOutboxEvent{
		{ID: 2, EventType: SchedulerOutboxEventAccountLastUsed},
		{ID: 1, EventType: SchedulerOutboxEventAccountLastUsed},
	}}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil)

	svc.pollOutbox()

	require.Equal(t, []int64{2, 1}, repo.acked,
		"claims must be acknowledged independently of sequence order")
	require.Empty(t, repo.released)
	require.NotEmpty(t, repo.claimLease)
	for _, lease := range repo.claimLease[:len(repo.claimLease)-1] {
		require.Equal(t, outboxClaimLease, lease)
	}
}

func TestSchedulerSnapshotServicePollOutboxReleasesClaimOnHandleFailure(t *testing.T) {
	cache := &outboxClaimCache{updateErr: errors.New("cache update failed"), lockAvailable: true}
	repo := &outboxClaimRepo{events: []SchedulerOutboxEvent{{
		ID:        5,
		EventType: SchedulerOutboxEventAccountLastUsed,
		Payload: map[string]any{
			"last_used": map[string]any{"101": float64(123)},
		},
	}}}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil)

	svc.pollOutbox()

	require.Empty(t, repo.acked)
	require.Equal(t, []int64{5}, repo.released)
}

func TestSchedulerSnapshotServicePollOutboxReleasesClaimOnBucketLockMiss(t *testing.T) {
	bucket := SchedulerBucket{GroupID: 10, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}
	cache := &outboxClaimCache{lockAvailable: false, buckets: []SchedulerBucket{bucket}}
	repo := &outboxClaimRepo{events: []SchedulerOutboxEvent{{
		ID:        8,
		EventType: SchedulerOutboxEventFullRebuild,
	}}}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil)

	svc.pollOutbox()

	require.Empty(t, repo.acked, "a lock miss must not acknowledge the event")
	require.Equal(t, []int64{8}, repo.released)
}

func TestSchedulerSnapshotServicePollOutboxLagIgnoresAcknowledgedEvent(t *testing.T) {
	cache := &outboxClaimCache{lockAvailable: true}
	repo := &outboxClaimRepo{events: []SchedulerOutboxEvent{{
		ID:        7,
		EventType: SchedulerOutboxEventAccountLastUsed,
		CreatedAt: time.Now().Add(-time.Hour),
	}}}
	cfg := &config.Config{Gateway: config.GatewayConfig{Scheduling: config.GatewaySchedulingConfig{
		OutboxLagWarnSeconds:     1,
		OutboxLagRebuildSeconds:  1,
		OutboxLagRebuildFailures: 1,
	}}}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, cfg)

	svc.pollOutbox()

	require.Equal(t, []int64{7}, repo.acked)
	require.Equal(t, 1, repo.oldestPendingCalls)
	require.Zero(t, cache.listBucketCalls)
	require.Zero(t, svc.lagFailures)
}

func TestSchedulerSnapshotServiceRebuildBucketReturnsRetryableLockError(t *testing.T) {
	cache := &outboxClaimCache{lockAvailable: false}
	svc := NewSchedulerSnapshotService(cache, nil, nil, nil, nil)

	err := svc.rebuildBucket(context.Background(), SchedulerBucket{
		GroupID: 1, Platform: PlatformOpenAI, Mode: SchedulerModeSingle,
	}, "test")

	require.ErrorIs(t, err, ErrSchedulerBucketLocked)
}
