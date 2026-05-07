//go:build unit

package skillkit

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestQueriesListSettlementsExcludesSkippedRowsFromPendingFilter(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	queries := NewQueries(db)
	params := pagination.PaginationParams{Page: 1, PageSize: 20}

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM ai_skill_settlements st.*WHERE 1 = 1.*st.status = 'pending'.*COALESCE\(st.metadata->>'service_status', ''\) <> 'skipped'`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(`(?s)SELECT.*COALESCE\(st.metadata->>'service_status', ''\) AS service_status.*FROM ai_skill_settlements st.*WHERE 1 = 1.*st.status = 'pending'.*COALESCE\(st.metadata->>'service_status', ''\) <> 'skipped'`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"skill_id",
			"title",
			"skill_slug",
			"author_name",
			"period_label",
			"status",
			"gross_amount",
			"platform_fee_amount",
			"payout_amount",
			"frozen_amount",
			"currency",
			"note",
			"service_status",
			"created_at",
			"updated_at",
		}))

	items, result, err := queries.ListSettlements(t.Context(), params, "pending", "")
	require.NoError(t, err)
	require.Empty(t, items)
	require.NotNil(t, result)
	require.EqualValues(t, 0, result.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}
