//go:build unit

package skillkit

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestQueriesGetSkillRevenueUsesCreatorPayoutAmounts(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	queries := NewQueries(db)

	creatorAmountExpr := `COALESCE\(\(st.metadata->>'creator_amount'\)::double precision, st.quota_amount, 0\)`
	mock.ExpectQuery(`(?s)SELECT.*SUM\(CASE WHEN st.status = 'transferred' THEN `+creatorAmountExpr+` ELSE 0 END\).*SUM\(CASE WHEN st.status = 'pending' THEN `+creatorAmountExpr+` ELSE 0 END\).*SUM\(CASE WHEN st.status = 'transferred' THEN `+creatorAmountExpr+` ELSE 0 END\).*FROM ai_skills s`).
		WithArgs(int64(42), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_revenue",
			"total_sales",
			"total_runs",
			"pending_amount",
			"settled_amount",
			"refunded_amount",
			"currency",
		}).AddRow(80.0, int64(2), int64(3), 20.0, 80.0, 0.0, "CNY"))
	mock.ExpectQuery(`(?s)SELECT.*SUM\(` + creatorAmountExpr + `\).*AS revenue.*FROM ai_skill_runs r`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"day_label",
			"revenue",
			"sales",
			"runs",
		}))
	mock.ExpectQuery(`(?s)SELECT.*` + creatorAmountExpr + `::double precision AS amount.*FROM ai_skill_settlements st`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"buyer_name",
			"version_name",
			"amount",
			"currency",
			"status",
			"created_at",
		}).AddRow(int64(9), "buyer", "v1", 80.0, "CNY", "transferred", time.Now()))

	detail, err := queries.GetSkillRevenue(t.Context(), 7, 42)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.Equal(t, 80.0, detail.Summary.TotalRevenue)
	require.Len(t, detail.Orders, 1)
	require.Equal(t, 80.0, detail.Orders[0].Amount)
	require.NoError(t, mock.ExpectationsWereMet())
}

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

func TestQueriesListSettlementsFrozenFilterUsesPendingCreatorPayout(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	queries := NewQueries(db)
	params := pagination.PaginationParams{Page: 1, PageSize: 20}
	creatorAmountExpr := `COALESCE\(\(st.metadata->>'creator_amount'\)::double precision, st.quota_amount, 0\)`
	frozenWhere := `st.status = 'pending'.*` + creatorAmountExpr + ` > 0.*COALESCE\(st.metadata->>'service_status', ''\) <> 'skipped'`

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM ai_skill_settlements st.*WHERE 1 = 1.*` + frozenWhere).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(`(?s)SELECT.*CASE WHEN st.status = 'pending' THEN ` + creatorAmountExpr + ` ELSE 0 END AS frozen_amount.*FROM ai_skill_settlements st.*WHERE 1 = 1.*` + frozenWhere).
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

	items, result, err := queries.ListSettlements(t.Context(), params, "frozen", "")
	require.NoError(t, err)
	require.Empty(t, items)
	require.NotNil(t, result)
	require.EqualValues(t, 0, result.Total)
	require.NoError(t, mock.ExpectationsWereMet())
}
