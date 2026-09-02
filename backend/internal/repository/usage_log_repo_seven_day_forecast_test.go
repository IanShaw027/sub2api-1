package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestRecordAccountSevenDayForecastObservation(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	resetAt := time.Date(2026, 8, 30, 12, 0, 0, 900, time.UTC)
	observedAt := resetAt.Add(-3 * 24 * time.Hour)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO account_seven_day_forecast_snapshots")).
		WithArgs(int64(42), resetAt.Round(time.Minute).Add(-7*24*time.Hour), 30, 37.42, observedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.RecordAccountSevenDayForecastObservation(context.Background(), 42, 37.42, resetAt, observedAt))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAccountSevenDayForecastPointsPreservesMissingFirstInterval(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	windowStart := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	firstObserved := windowStart.Add(24 * time.Hour)
	secondObserved := firstObserved.Add(12 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"bucket", "utilization", "window_start", "observed_at", "used_cost", "rate_limit_429", "sessions",
	}).
		AddRow(10, 10.4, windowStart, firstObserved, 4.0, nil, nil).
		AddRow(20, 20.2, windowStart, secondObserved, 8.5, int64(2), int64(3))
	mock.ExpectQuery("jsonb_array_elements").WithArgs(int64(42)).WillReturnRows(rows)

	points, err := repo.GetAccountSevenDayForecastPoints(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, points, 2)
	require.Nil(t, points[0].RateLimit429)
	require.Nil(t, points[0].Sessions)
	require.InDelta(t, 4.0*100/10.4, points[0].PredictedTotalCost, 1e-9)
	require.NotNil(t, points[1].RateLimit429)
	require.EqualValues(t, 2, *points[1].RateLimit429)
	require.NotNil(t, points[1].Sessions)
	require.EqualValues(t, 3, *points[1].Sessions)
	require.NoError(t, mock.ExpectationsWereMet())
}
