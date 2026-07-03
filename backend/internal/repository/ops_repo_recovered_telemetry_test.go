package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryGetErrorTrendSeparatesRecoveredTelemetry(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	repo := NewOpsRepository(db).(*opsRepository)
	start := time.Date(2026, 7, 2, 18, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	bucket := start

	mock.ExpectQuery(`COALESCE\(status_code, 0\) > 0 AND COALESCE\(status_code, 0\) < 400 AND error_phase = 'upstream'[\s\S]*AS recovered_telemetry`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{
			"bucket", "error_total", "business_limited", "error_sla", "upstream_excl", "upstream_429", "upstream_529", "recovered_telemetry",
		}).AddRow(bucket, int64(3), int64(1), int64(2), int64(2), int64(0), int64(0), int64(7)))

	resp, err := repo.GetErrorTrend(context.Background(), &service.OpsDashboardFilter{StartTime: start, EndTime: end}, 3600)
	require.NoError(t, err)
	require.Len(t, resp.Points, 1)
	require.Equal(t, int64(3), resp.Points[0].ErrorCountTotal)
	require.Equal(t, int64(7), resp.Points[0].RecoveredTelemetryCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsRepositoryGetErrorDistributionSeparatesRecoveredTelemetryTotal(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	repo := NewOpsRepository(db).(*opsRepository)
	start := time.Date(2026, 7, 2, 18, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	mock.ExpectQuery(`GROUP BY 1\s+ORDER BY total DESC`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{"status_code", "total", "sla", "business_limited"}).
			AddRow(503, int64(4), int64(4), int64(0)))
	mock.ExpectQuery(`GROUP BY 1\s+ORDER BY total DESC`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{"owner", "total", "sla", "business_limited"}).
			AddRow("provider", int64(4), int64(4), int64(0)))
	mock.ExpectQuery(`COALESCE\(status_code, 0\) > 0 AND COALESCE\(status_code, 0\) < 400 AND error_phase = 'upstream'[\s\S]*recovered_telemetry_total`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{"recovered_telemetry_total"}).AddRow(int64(9)))

	resp, err := repo.GetErrorDistribution(context.Background(), &service.OpsDashboardFilter{StartTime: start, EndTime: end})
	require.NoError(t, err)
	require.Equal(t, int64(4), resp.Total)
	require.Equal(t, int64(9), resp.RecoveredTelemetryTotal)
	require.NoError(t, mock.ExpectationsWereMet())
}
