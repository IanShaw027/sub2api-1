package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	aiSkillBalanceOperationCharge = "charge"
	aiSkillBalanceOperationRefund = "refund"
)

type aiSkillBalanceLedgerRepository struct {
	db *sql.DB
}

func NewAISkillBalanceLedgerRepository(db *sql.DB) service.AISkillBalanceLedgerRepository {
	return &aiSkillBalanceLedgerRepository{db: db}
}

func (r *aiSkillBalanceLedgerRepository) ApplyAISkillBalanceCharge(ctx context.Context, input service.AISkillBalanceChargeInput) (*service.AISkillBalanceLedgerResult, error) {
	if err := validateAISkillBalanceMutation(input.UserID, input.Amount, input.Reference); err != nil {
		return nil, err
	}
	if r == nil || r.db == nil {
		return nil, service.ErrAISkillBalanceServiceUnavailable
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal ai skill balance charge metadata: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin ai skill balance charge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	inserted, err := insertAISkillBalanceClaim(ctx, tx, aiSkillBalanceOperationCharge, input.UserID, input.Amount, input.Reference, metadata, input.Trace)
	if err != nil {
		return nil, err
	}
	if !inserted {
		result, loadErr := loadAISkillBalanceLedgerResult(ctx, tx, aiSkillBalanceOperationCharge, input.UserID, input.Amount, input.Reference)
		if loadErr != nil {
			return nil, loadErr
		}
		if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM ai_skill_balance_ledger
    WHERE reference = $1 AND operation = $2
)`, input.Reference, aiSkillBalanceOperationRefund).Scan(&result.Refunded); err != nil {
			return nil, fmt.Errorf("check ai skill balance refund state: %w", err)
		}
		result.Duplicate = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit duplicate ai skill balance charge: %w", err)
		}
		return result, nil
	}

	var balanceAfter float64
	err = tx.QueryRowContext(ctx, `
UPDATE users
SET balance = balance - $1, updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
RETURNING balance::double precision`, input.Amount, input.UserID).Scan(&balanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if existsErr := tx.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL
)`, input.UserID).Scan(&exists); existsErr != nil {
			return nil, fmt.Errorf("check ai skill buyer balance owner: %w", existsErr)
		}
		if !exists {
			return nil, service.ErrUserNotFound
		}
		return nil, service.ErrInsufficientBalance
	}
	if err != nil {
		return nil, fmt.Errorf("deduct ai skill buyer balance: %w", err)
	}
	balanceBefore := balanceAfter + input.Amount
	if _, err := tx.ExecContext(ctx, `
INSERT INTO balance_cache_outbox (user_id)
VALUES ($1)`, input.UserID); err != nil {
		return nil, fmt.Errorf("enqueue ai skill balance cache invalidation: %w", err)
	}
	if err := finishAISkillBalanceClaim(ctx, tx, aiSkillBalanceOperationCharge, input.Reference, balanceBefore, balanceAfter); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit ai skill balance charge: %w", err)
	}
	return &service.AISkillBalanceLedgerResult{
		Amount:        input.Amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
	}, nil
}

func (r *aiSkillBalanceLedgerRepository) ApplyAISkillBalanceRefund(ctx context.Context, input service.AISkillBalanceRefundInput) (*service.AISkillBalanceLedgerResult, error) {
	if err := validateAISkillBalanceMutation(input.UserID, input.Amount, input.Reference); err != nil {
		return nil, err
	}
	if r == nil || r.db == nil {
		return nil, service.ErrAISkillBalanceServiceUnavailable
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal ai skill balance refund metadata: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin ai skill balance refund: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Locking the original charge serializes a concurrent duplicate charge with
	// its compensation and prevents a refund from being applied before a charge.
	if _, err := loadAISkillBalanceLedgerResult(ctx, tx, aiSkillBalanceOperationCharge, input.UserID, input.Amount, input.Reference); err != nil {
		return nil, err
	}
	inserted, err := insertAISkillBalanceClaim(ctx, tx, aiSkillBalanceOperationRefund, input.UserID, input.Amount, input.Reference, metadata, input.Trace)
	if err != nil {
		return nil, err
	}
	if !inserted {
		result, loadErr := loadAISkillBalanceLedgerResult(ctx, tx, aiSkillBalanceOperationRefund, input.UserID, input.Amount, input.Reference)
		if loadErr != nil {
			return nil, loadErr
		}
		result.Duplicate = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit duplicate ai skill balance refund: %w", err)
		}
		return result, nil
	}

	var balanceAfter float64
	err = tx.QueryRowContext(ctx, `
UPDATE users
SET balance = balance + $1, updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL
RETURNING balance::double precision`, input.Amount, input.UserID).Scan(&balanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("refund ai skill buyer balance: %w", err)
	}
	balanceBefore := balanceAfter - input.Amount
	if err := finishAISkillBalanceClaim(ctx, tx, aiSkillBalanceOperationRefund, input.Reference, balanceBefore, balanceAfter); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit ai skill balance refund: %w", err)
	}
	return &service.AISkillBalanceLedgerResult{
		Amount:        input.Amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
	}, nil
}

func validateAISkillBalanceMutation(userID int64, amount float64, reference string) error {
	if userID <= 0 || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return service.ErrAISkillBalanceServiceUnavailable
	}
	if strings.TrimSpace(reference) == "" {
		return service.ErrAISkillBalanceReferenceInvalid
	}
	return nil
}

func insertAISkillBalanceClaim(ctx context.Context, tx *sql.Tx, operation string, userID int64, amount float64, reference string, metadata []byte, trace service.AIWriteTrace) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
INSERT INTO ai_skill_balance_ledger (
    reference, operation, user_id, amount, metadata,
    request_id, usage_log_id, api_key_id, group_id
)
VALUES ($1, $2, $3, $4, $5::jsonb, NULLIF($6, ''), $7, $8, $9)
ON CONFLICT (reference, operation) DO NOTHING
RETURNING id`, reference, operation, userID, amount, string(metadata), trace.RequestID, trace.UsageLogID, trace.APIKeyID, trace.GroupID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim ai skill balance %s reference: %w", operation, err)
	}
	return true, nil
}

func loadAISkillBalanceLedgerResult(ctx context.Context, tx *sql.Tx, operation string, userID int64, amount float64, reference string) (*service.AISkillBalanceLedgerResult, error) {
	var storedUserID int64
	result := &service.AISkillBalanceLedgerResult{}
	err := tx.QueryRowContext(ctx, `
SELECT user_id,
       amount::double precision,
       balance_before::double precision,
       balance_after::double precision
FROM ai_skill_balance_ledger
WHERE reference = $1 AND operation = $2
FOR UPDATE`, reference, operation).Scan(&storedUserID, &result.Amount, &result.BalanceBefore, &result.BalanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAISkillBalanceReferenceConflict
	}
	if err != nil {
		return nil, fmt.Errorf("load ai skill balance %s reference: %w", operation, err)
	}
	if storedUserID != userID || !sameAISkillLedgerAmount(result.Amount, amount) {
		return nil, service.ErrAISkillBalanceReferenceConflict
	}
	return result, nil
}

func finishAISkillBalanceClaim(ctx context.Context, tx *sql.Tx, operation, reference string, balanceBefore, balanceAfter float64) error {
	result, err := tx.ExecContext(ctx, `
UPDATE ai_skill_balance_ledger
SET balance_before = $1, balance_after = $2, updated_at = NOW()
WHERE reference = $3 AND operation = $4`, balanceBefore, balanceAfter, reference, operation)
	if err != nil {
		return fmt.Errorf("finish ai skill balance %s reference: %w", operation, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count ai skill balance %s claim update: %w", operation, err)
	}
	if affected != 1 {
		return fmt.Errorf("finish ai skill balance %s reference: expected one ledger row, updated %d", operation, affected)
	}
	return nil
}

func sameAISkillLedgerAmount(left, right float64) bool {
	return math.Abs(left-right) < 0.000000005
}
