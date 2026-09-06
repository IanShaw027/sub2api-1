package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type creationVideoBillingLookup struct{ db *sql.DB }

func NewCreationVideoBillingLookup(db *sql.DB) service.CreationVideoBillingLookup {
	return &creationVideoBillingLookup{db: db}
}

func (r *creationVideoBillingLookup) Recorded(ctx context.Context, requestID string, apiKeyID int64) (bool, error) {
	var recorded bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
        SELECT 1 FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2
        UNION ALL
        SELECT 1 FROM usage_billing_dedup_archive WHERE request_id = $1 AND api_key_id = $2
    )`, requestID, apiKeyID).Scan(&recorded)
	return recorded, err
}
