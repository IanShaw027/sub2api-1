package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildContentModerationLogWhere_BlockedIncludesAllBlockActions(t *testing.T) {
	where, args := buildContentModerationLogWhere(service.ContentModerationLogFilter{Result: "blocked"})

	require.Empty(t, args)
	sql := strings.Join(where, " AND ")
	require.Contains(t, sql, "l.action IN ('block', 'keyword_block', 'hash_block', 'cyber_policy')")
	require.NotContains(t, sql, "l.action = 'block'")
}

func TestBuildContentModerationLogWhere_SupportsExactResultTypes(t *testing.T) {
	tests := []struct {
		name     string
		result   string
		contains string
	}{
		{name: "api block", result: "block", contains: "l.action = 'block'"},
		{name: "keyword block", result: "keyword_block", contains: "l.action = 'keyword_block'"},
		{name: "hash block", result: "hash_block", contains: "l.action = 'hash_block'"},
		{name: "hash observe", result: "hash_observe", contains: "l.action = 'hash_observe'"},
		{name: "cyber policy", result: "cyber_policy", contains: "l.action = 'cyber_policy'"},
		{name: "allow", result: "allow", contains: "l.action = 'allow' AND l.flagged = FALSE AND l.error = ''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			where, args := buildContentModerationLogWhere(service.ContentModerationLogFilter{Result: tt.result})

			require.Empty(t, args)
			require.Contains(t, strings.Join(where, " AND "), tt.contains)
		})
	}
}

func TestContentModerationRepositoryCountFlaggedByUserSince_ExcludesHashActions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewContentModerationRepository(db)
	since := time.Now().Add(-time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta("AND action NOT IN ('hash_block', 'hash_observe')")).
		WithArgs(int64(1001), since, false).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	count, err := repo.CountFlaggedByUserSince(context.Background(), 1001, since, false)

	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContentModerationRepositoryCountFlaggedByUserSince_ExcludesCyberPolicyWhenRequested(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewContentModerationRepository(db)
	since := time.Now().Add(-time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta("AND ($3::bool IS FALSE OR action <> 'cyber_policy')")).
		WithArgs(int64(1001), since, true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	count, err := repo.CountFlaggedByUserSince(context.Background(), 1001, since, true)

	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContentModerationRepositoryUpdateLogAutoBanned(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewContentModerationRepository(db)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE content_moderation_logs SET auto_banned = $1 WHERE id = $2")).
		WithArgs(true, int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.UpdateLogAutoBanned(context.Background(), 77, true))
	require.NoError(t, mock.ExpectationsWereMet())
}
