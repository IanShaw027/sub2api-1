//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCreationVideoBillingLookupUsesCommittedDedupAndArchive(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`(?s)SELECT EXISTS .*FROM usage_billing_dedup WHERE request_id = \$1 AND api_key_id = \$2.*UNION ALL.*FROM usage_billing_dedup_archive`).
		WithArgs("grok-video:task-1", int64(9)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	recorded, err := NewCreationVideoBillingLookup(db).Recorded(context.Background(), "grok-video:task-1", 9)
	require.NoError(t, err)
	require.True(t, recorded)
	require.NoError(t, mock.ExpectationsWereMet())
}
