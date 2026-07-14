package service

import (
	"context"
	"time"
)

type BalanceCacheOutboxEvent struct {
	ID     int64
	UserID int64
}

type BalanceCacheOutboxRepository interface {
	Claim(ctx context.Context, limit int, lease time.Duration) ([]BalanceCacheOutboxEvent, error)
	Ack(ctx context.Context, ids []int64) error
	Nack(ctx context.Context, ids []int64, retryAfter time.Duration, cause string) error
}
