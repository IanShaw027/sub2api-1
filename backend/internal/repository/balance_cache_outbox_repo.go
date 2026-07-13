package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type balanceCacheOutboxRepository struct {
	db *sql.DB
}

func NewBalanceCacheOutboxRepository(db *sql.DB) service.BalanceCacheOutboxRepository {
	return &balanceCacheOutboxRepository{db: db}
}

func (r *balanceCacheOutboxRepository) Claim(ctx context.Context, limit int, lease time.Duration) ([]service.BalanceCacheOutboxEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH picked AS (
			SELECT id
			FROM balance_cache_outbox
			WHERE available_at <= NOW()
			  AND (lease_until IS NULL OR lease_until <= NOW())
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE balance_cache_outbox o
		SET lease_until = NOW() + ($2 * INTERVAL '1 second'),
		    attempts = attempts + 1
		FROM picked
		WHERE o.id = picked.id
		RETURNING o.id, o.user_id
	`, limit, max(int(lease/time.Second), 1))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]service.BalanceCacheOutboxEvent, 0, limit)
	for rows.Next() {
		var event service.BalanceCacheOutboxEvent
		if err := rows.Scan(&event.ID, &event.UserID); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *balanceCacheOutboxRepository) Ack(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM balance_cache_outbox WHERE id = ANY($1)`, pq.Array(ids))
	return err
}

func (r *balanceCacheOutboxRepository) Nack(ctx context.Context, ids []int64, retryAfter time.Duration, cause string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE balance_cache_outbox
		SET lease_until = NULL,
		    available_at = NOW() + ($2 * INTERVAL '1 second'),
		    last_error = LEFT($3, 2000)
		WHERE id = ANY($1)
	`, pq.Array(ids), max(int(retryAfter/time.Second), 1), cause)
	return err
}
