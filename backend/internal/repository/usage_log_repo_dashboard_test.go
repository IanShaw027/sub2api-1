//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestFillDashboardPaymentStatsAllTimeAndToday(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	todayStart := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	stats := &usagestats.DashboardStats{}

	mock.ExpectQuery(`FROM payment_orders`).
		WithArgs(time.Unix(0, 0).UTC(), todayStart.Add(24*time.Hour), todayStart, todayStart.Add(24*time.Hour)).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_recharge_amount", "total_refund_amount", "today_recharge_amount", "today_refund_amount",
		}).AddRow(100.0, 20.0, 10.0, 2.0))

	require.NoError(t, repo.fillDashboardPaymentStats(context.Background(), stats, time.Time{}, time.Time{}, todayStart))
	require.Equal(t, 100.0, stats.TotalRechargeAmount)
	require.Equal(t, 20.0, stats.TotalRefundAmount)
	require.Equal(t, 10.0, stats.TodayRechargeAmount)
	require.Equal(t, 2.0, stats.TodayRefundAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFillDashboardPaymentStatsUsesRequestedRange(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	todayStart := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	stats := &usagestats.DashboardStats{}

	mock.ExpectQuery(`FROM payment_audit_logs`).
		WithArgs(start, end, todayStart, todayStart.Add(24*time.Hour)).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_recharge_amount", "total_refund_amount", "today_recharge_amount", "today_refund_amount",
		}).AddRow(40.0, 5.0, 0.0, 0.0))

	require.NoError(t, repo.fillDashboardPaymentStats(context.Background(), stats, start, end, todayStart))
	require.Equal(t, 40.0, stats.TotalRechargeAmount)
	require.Equal(t, 5.0, stats.TotalRefundAmount)
	require.NoError(t, mock.ExpectationsWereMet())
}
