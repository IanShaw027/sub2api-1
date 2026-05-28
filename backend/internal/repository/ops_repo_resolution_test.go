package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryUpdateErrorResolution_DoesNotWriteResolvedRetryID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &opsRepository{db: db}
	resolvedByUserID := int64(7)
	resolvedRetryID := int64(99)
	resolvedAt := time.Date(2026, time.May, 22, 10, 11, 12, 0, time.UTC)

	mock.ExpectExec(`UPDATE ops_error_logs\s+SET\s+resolved = \$2,\s+resolved_at = \$3,\s+resolved_by_user_id = \$4\s+WHERE id = \$1`).
		WithArgs(
			int64(42),
			true,
			sqlmock.AnyArg(),
			sql.NullInt64{Int64: resolvedByUserID, Valid: true},
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpdateErrorResolution(
		context.Background(),
		42,
		true,
		&resolvedByUserID,
		&resolvedRetryID,
		&resolvedAt,
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
