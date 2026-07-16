//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

const (
	aiSkillLedgerInsertSQL  = `(?s)INSERT INTO ai_skill_balance_ledger.*ON CONFLICT \(reference, operation\) DO NOTHING.*RETURNING id`
	aiSkillLedgerLoadSQL    = `(?s)SELECT user_id,.*FROM ai_skill_balance_ledger.*WHERE reference = \$1 AND operation = \$2.*FOR UPDATE`
	aiSkillLedgerFinishSQL  = `(?s)UPDATE ai_skill_balance_ledger.*SET balance_before = \$1, balance_after = \$2.*WHERE reference = \$3 AND operation = \$4`
	aiSkillBalanceDeductSQL = `(?s)UPDATE users.*SET balance = balance - \$1, updated_at = NOW\(\).*WHERE id = \$2 AND deleted_at IS NULL AND balance >= \$1.*RETURNING balance::double precision`
	aiSkillBalanceRefundSQL = `(?s)UPDATE users.*SET balance = balance \+ \$1, updated_at = NOW\(\).*WHERE id = \$2 AND deleted_at IS NULL.*RETURNING balance::double precision`
)

func TestAISkillBalanceLedgerRepository_ChargeClaimAndBalanceMutationCommitTogether(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAISkillBalanceLedgerRepository(db)
	input := service.AISkillBalanceChargeInput{UserID: 42, Amount: 12.5, Reference: "ai_skill_run:99"}

	mock.ExpectBegin()
	mock.ExpectQuery(aiSkillLedgerInsertSQL).
		WithArgs(input.Reference, aiSkillBalanceOperationCharge, input.UserID, input.Amount, "null", "", nil, nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(aiSkillBalanceDeductSQL).
		WithArgs(input.Amount, input.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(87.5))
	mock.ExpectExec(`(?s)INSERT INTO balance_cache_outbox.*VALUES \(\$1\)`).
		WithArgs(input.UserID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(aiSkillLedgerFinishSQL).
		WithArgs(100.0, 87.5, input.Reference, aiSkillBalanceOperationCharge).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.ApplyAISkillBalanceCharge(context.Background(), input)
	require.NoError(t, err)
	require.False(t, result.Duplicate)
	require.InDelta(t, 100, result.BalanceBefore, 1e-9)
	require.InDelta(t, 87.5, result.BalanceAfter, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAISkillBalanceLedgerRepository_DuplicateChargeReturnsOriginalResult(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAISkillBalanceLedgerRepository(db)
	input := service.AISkillBalanceChargeInput{UserID: 42, Amount: 12.5, Reference: "ai_skill_run:99"}

	mock.ExpectBegin()
	mock.ExpectQuery(aiSkillLedgerInsertSQL).
		WithArgs(input.Reference, aiSkillBalanceOperationCharge, input.UserID, input.Amount, "null", "", nil, nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(aiSkillLedgerLoadSQL).
		WithArgs(input.Reference, aiSkillBalanceOperationCharge).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "amount", "balance_before", "balance_after"}).
			AddRow(input.UserID, input.Amount, 100.0, 87.5))
	mock.ExpectQuery(`(?s)SELECT EXISTS.*WHERE reference = \$1 AND operation = \$2`).
		WithArgs(input.Reference, aiSkillBalanceOperationRefund).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectCommit()

	result, err := repo.ApplyAISkillBalanceCharge(context.Background(), input)
	require.NoError(t, err)
	require.True(t, result.Duplicate)
	require.False(t, result.Refunded)
	require.InDelta(t, 87.5, result.BalanceAfter, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAISkillBalanceLedgerRepository_ChargeRejectsInsufficientBalance(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAISkillBalanceLedgerRepository(db)
	input := service.AISkillBalanceChargeInput{UserID: 42, Amount: 12.5, Reference: "ai_skill_run:99"}

	mock.ExpectBegin()
	mock.ExpectQuery(aiSkillLedgerInsertSQL).
		WithArgs(input.Reference, aiSkillBalanceOperationCharge, input.UserID, input.Amount, "null", "", nil, nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(aiSkillBalanceDeductSQL).
		WithArgs(input.Amount, input.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}))
	mock.ExpectQuery(`(?s)SELECT EXISTS.*FROM users WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs(input.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	result, err := repo.ApplyAISkillBalanceCharge(context.Background(), input)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	require.Nil(t, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAISkillBalanceLedgerRepository_RefundLocksChargeAndCommitsMutation(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAISkillBalanceLedgerRepository(db)
	input := service.AISkillBalanceRefundInput{UserID: 42, Amount: 12.5, Reference: "ai_skill_run:99"}

	mock.ExpectBegin()
	mock.ExpectQuery(aiSkillLedgerLoadSQL).
		WithArgs(input.Reference, aiSkillBalanceOperationCharge).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "amount", "balance_before", "balance_after"}).
			AddRow(input.UserID, input.Amount, 100.0, 87.5))
	mock.ExpectQuery(aiSkillLedgerInsertSQL).
		WithArgs(input.Reference, aiSkillBalanceOperationRefund, input.UserID, input.Amount, "null", "", nil, nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
	mock.ExpectQuery(aiSkillBalanceRefundSQL).
		WithArgs(input.Amount, input.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(100.0))
	mock.ExpectExec(aiSkillLedgerFinishSQL).
		WithArgs(87.5, 100.0, input.Reference, aiSkillBalanceOperationRefund).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.ApplyAISkillBalanceRefund(context.Background(), input)
	require.NoError(t, err)
	require.False(t, result.Duplicate)
	require.InDelta(t, 87.5, result.BalanceBefore, 1e-9)
	require.InDelta(t, 100, result.BalanceAfter, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}
