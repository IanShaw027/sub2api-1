package service

import (
	"context"
	"time"
)

type SchedulerOutboxEvent struct {
	ID         int64
	ClaimToken string
	EventType  string
	AccountID  *int64
	GroupID    *int64
	Payload    map[string]any
	CreatedAt  time.Time
}

// SchedulerOutboxRepository 提供调度 outbox 的读取接口。
type SchedulerOutboxRepository interface {
	// ClaimPending atomically leases the oldest available event. A lease that is
	// not acknowledged becomes available again after leaseDuration.
	ClaimPending(ctx context.Context, leaseDuration time.Duration) (*SchedulerOutboxEvent, error)
	AckClaim(ctx context.Context, eventID int64, claimToken string) (bool, error)
	ReleaseClaim(ctx context.Context, eventID int64, claimToken string) error
	OldestPendingCreatedAt(ctx context.Context) (time.Time, bool, error)
	PendingCount(ctx context.Context) (int64, error)
}
