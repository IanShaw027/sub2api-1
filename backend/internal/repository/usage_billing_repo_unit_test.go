//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	conditionalBalanceDeductSQL = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND balance >= \$1\s+RETURNING balance`
	overdraftBalanceDeductSQL   = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL\s+RETURNING balance`
)

func TestDeductUsageBillingBalance_UsesSufficientBalanceGuard(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(7.5))
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 2.5)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 7.5, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_RecordsOverdraftWhenGuardMisses(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 10)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -5.0, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_FlagsBalanceOverdraft(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -5.0, *result.NewBalance, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_EnqueuesInvalidationWhenBalanceCrossesReserve(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(0.01, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0.005))
	mock.ExpectExec(`INSERT INTO balance_cache_outbox \(user_id\) VALUES \(\$1\)`).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:                42,
		BalanceCost:           0.01,
		MinimumBalanceReserve: 0.01,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 0.005, *result.NewBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_RollsBackWhenReserveInvalidationCannotBeEnqueued(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(0.01, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0.005))
	mock.ExpectExec(`INSERT INTO balance_cache_outbox \(user_id\) VALUES \(\$1\)`).
		WithArgs(int64(42)).
		WillReturnError(errors.New("outbox unavailable"))
	mock.ExpectRollback()

	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:                42,
		BalanceCost:           0.01,
		MinimumBalanceReserve: 0.01,
	}, &service.UsageBillingApplyResult{Applied: true})

	require.ErrorContains(t, err, "enqueue reserve-crossing balance cache invalidation")
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnqueueBalanceInvalidationOnReserveCrossingSkipsNonCrossings(t *testing.T) {
	tests := []struct {
		name       string
		newBalance float64
		cost       float64
		reserve    float64
	}{
		{name: "still eligible", newBalance: 0.02, cost: 0.01, reserve: 0.01},
		{name: "already below reserve", newBalance: 0.005, cost: 0.001, reserve: 0.01},
		{name: "exhausted covered by trigger", newBalance: 0, cost: 0.01, reserve: 0.01},
		{name: "reserve disabled", newBalance: 0.005, cost: 0.01, reserve: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			mock.ExpectBegin()
			tx, err := db.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			mock.ExpectRollback()

			err = enqueueBalanceInvalidationOnReserveCrossing(context.Background(), tx, &service.UsageBillingCommand{
				UserID:                42,
				BalanceCost:           tt.cost,
				MinimumBalanceReserve: tt.reserve,
			}, tt.newBalance)

			require.NoError(t, err)
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDeductUsageBillingBalance_ReturnsUserNotFoundWhenNoUserUpdated(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, _, err = deductUsageBillingBalance(ctx, tx, 42, 10)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_RequiresDB(t *testing.T) {
	result, err := (*usageBillingRepository)(nil).Apply(context.Background(), &service.UsageBillingCommand{})
	require.Nil(t, result)
	require.EqualError(t, err, "usage billing repository db is required")
}
