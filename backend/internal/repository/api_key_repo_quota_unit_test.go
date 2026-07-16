//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIncrementQuotaUsedAndGetStateAcceptsNullLookupHash(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`UPDATE api_keys`).
		WithArgs(1.25, service.StatusAPIKeyQuotaExhausted, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"quota_used", "quota", "lookup_hash", "status"}).
			AddRow(5.5, 5.0, nil, service.StatusAPIKeyQuotaExhausted))

	repo := &apiKeyRepository{sql: db}
	state, err := repo.IncrementQuotaUsedAndGetState(context.Background(), 42, 1.25)

	require.NoError(t, err)
	require.NotNil(t, state)
	require.Empty(t, state.LookupHash)
	require.Equal(t, 5.5, state.QuotaUsed)
	require.Equal(t, service.StatusAPIKeyQuotaExhausted, state.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}
