package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type schedulerOutboxRepository struct {
	db *sql.DB
}

func NewSchedulerOutboxRepository(db *sql.DB) service.SchedulerOutboxRepository {
	return &schedulerOutboxRepository{db: db}
}

func (r *schedulerOutboxRepository) ClaimPending(ctx context.Context, leaseDuration time.Duration) (*service.SchedulerOutboxEvent, error) {
	if leaseDuration <= 0 {
		return nil, fmt.Errorf("scheduler outbox lease duration must be positive")
	}
	claimToken, err := newSchedulerOutboxClaimToken()
	if err != nil {
		return nil, err
	}

	row := r.db.QueryRowContext(ctx, `
		WITH selected AS MATERIALIZED (
			SELECT id
			FROM scheduler_outbox
			WHERE claimed_at IS NULL
				OR claimed_at < NOW() - make_interval(secs => $1::double precision)
			ORDER BY created_at ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		), claimed AS (
			UPDATE scheduler_outbox AS o
			SET claimed_at = NOW(),
				claim_token = $2,
				dedup_key = NULL
			FROM selected AS s
			WHERE o.id = s.id
			RETURNING o.id, o.event_type, o.account_id, o.group_id, o.payload, o.created_at, o.claim_token
		)
		SELECT id, event_type, account_id, group_id, payload, created_at, claim_token
		FROM claimed
	`, int64(leaseDuration/time.Second), claimToken)

	var (
		payloadRaw []byte
		accountID  sql.NullInt64
		groupID    sql.NullInt64
		event      service.SchedulerOutboxEvent
	)
	if err := row.Scan(&event.ID, &event.EventType, &accountID, &groupID, &payloadRaw, &event.CreatedAt, &event.ClaimToken); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if accountID.Valid {
		v := accountID.Int64
		event.AccountID = &v
	}
	if groupID.Valid {
		v := groupID.Int64
		event.GroupID = &v
	}
	if len(payloadRaw) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(payloadRaw, &payload); err != nil {
			return nil, err
		}
		event.Payload = payload
	}
	return &event, nil
}

func (r *schedulerOutboxRepository) AckClaim(ctx context.Context, eventID int64, claimToken string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM scheduler_outbox
		WHERE id = $1 AND claim_token = $2
	`, eventID, claimToken)
	if err != nil {
		return false, err
	}
	deleted, err := result.RowsAffected()
	return deleted == 1, err
}

func (r *schedulerOutboxRepository) ReleaseClaim(ctx context.Context, eventID int64, claimToken string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduler_outbox
		SET claimed_at = NULL, claim_token = NULL
		WHERE id = $1 AND claim_token = $2
	`, eventID, claimToken)
	return err
}

func (r *schedulerOutboxRepository) OldestPendingCreatedAt(ctx context.Context) (time.Time, bool, error) {
	var createdAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT MIN(created_at) FROM scheduler_outbox
	`).Scan(&createdAt)
	if err != nil {
		return time.Time{}, false, err
	}
	if !createdAt.Valid {
		return time.Time{}, false, nil
	}
	return createdAt.Time, true, nil
}

func (r *schedulerOutboxRepository) PendingCount(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func newSchedulerOutboxClaimToken() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate scheduler outbox claim token: %w", err)
	}
	return hex.EncodeToString(token[:]), nil
}

func enqueueSchedulerOutbox(ctx context.Context, exec sqlExecutor, eventType string, accountID *int64, groupID *int64, payload any) error {
	if exec == nil {
		return nil
	}
	var payloadArg any
	var payloadJSON []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadArg = encoded
		payloadJSON = encoded
	}
	query := `
		INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
		VALUES ($1, $2, $3, $4)
	`
	args := []any{eventType, accountID, groupID, payloadArg}
	if schedulerOutboxEventSupportsDedup(eventType) {
		dedupKey := schedulerOutboxDedupKey(eventType, accountID, groupID, payloadJSON)
		query = `
			INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload, dedup_key)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING
		`
		args = append(args, dedupKey)
	}
	_, err := exec.ExecContext(ctx, query, args...)
	return err
}

func schedulerOutboxDedupKey(eventType string, accountID *int64, groupID *int64, payloadJSON []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(eventType))
	_, _ = h.Write([]byte{0})
	if accountID != nil {
		_, _ = h.Write([]byte(strconv.FormatInt(*accountID, 10)))
	}
	_, _ = h.Write([]byte{0})
	if groupID != nil {
		_, _ = h.Write([]byte(strconv.FormatInt(*groupID, 10)))
	}
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(payloadJSON)
	return fmt.Sprintf("scheduler_outbox:%s", hex.EncodeToString(h.Sum(nil)))
}

func schedulerOutboxEventSupportsDedup(eventType string) bool {
	switch eventType {
	case service.SchedulerOutboxEventAccountChanged,
		service.SchedulerOutboxEventAccountBulkChanged,
		service.SchedulerOutboxEventGroupChanged,
		service.SchedulerOutboxEventFullRebuild:
		return true
	default:
		return false
	}
}
