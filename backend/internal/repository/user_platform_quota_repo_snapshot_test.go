//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func TestBatchSnapshotUsage_UsesStrictSnapshotFenceGuard(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	driver := sql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })

	fence := time.Date(2026, 7, 16, 10, 0, 0, 123456000, time.UTC)
	dailyStart := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	weeklyStart := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	monthlyStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectExec(`WHERE user_platform_quotas\.updated_at < EXCLUDED\.updated_at`).
		WithArgs(
			fence,
			int64(7), "openai",
			1.25, 2.5, 3.75,
			dailyStart, weeklyStart, monthlyStart,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewUserPlatformQuotaRepository(client)
	err = repo.BatchSnapshotUsage(context.Background(), []UserPlatformQuotaSnapshot{{
		UserID:             7,
		Platform:           "openai",
		DailyUsageUSD:      1.25,
		WeeklyUsageUSD:     2.5,
		MonthlyUsageUSD:    3.75,
		DailyWindowStart:   dailyStart,
		WeeklyWindowStart:  weeklyStart,
		MonthlyWindowStart: monthlyStart,
	}}, fence)
	if err != nil {
		t.Fatalf("BatchSnapshotUsage: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
