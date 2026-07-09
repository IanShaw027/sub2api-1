package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryGetThroughputTrendIncludesStickyOriginalUnavailableCounts(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	repo := NewOpsRepository(db).(*opsRepository)
	start := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	bucket := start
	groupID := int64(9)

	mock.ExpectQuery(`sticky_buckets[\s\S]*sticky_original_unavailable_count`).
		WithArgs(start, end, groupID, start, end, groupID, start, end, groupID).
		WillReturnRows(sqlmock.NewRows([]string{
			"bucket",
			"request_count",
			"token_consumed",
			"switch_count",
			"sticky_original_bound_count",
			"sticky_original_unavailable_count",
		}).AddRow(bucket, int64(10), int64(2500), int64(2), int64(4), int64(1)))

	resp, err := repo.GetThroughputTrend(context.Background(), &service.OpsDashboardFilter{StartTime: start, EndTime: end, GroupID: &groupID}, 3600)
	require.NoError(t, err)
	require.Len(t, resp.Points, 1)
	require.Equal(t, int64(2), resp.Points[0].SwitchCount)
	require.Equal(t, int64(4), resp.Points[0].StickyOriginalBoundCount)
	require.Equal(t, int64(1), resp.Points[0].StickyOriginalUnavailableCount)
	require.NoError(t, mock.ExpectationsWereMet())
}
