package main

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestShouldSyncMigrationChecksum(t *testing.T) {
	t.Parallel()

	require.True(t, shouldSyncMigrationChecksum(
		"195_add_invoice_order_active_unique_guard.sql",
		"a775587a040b17780ed37e5fe8efcf223de52263f1f75a5bb4b7c1f6af0b7a99",
		"ddeda6f9fcf3063d9e7fcefbd308e9bc8539a938baa3ca784a30970acbccbe92",
	))
	require.False(t, shouldSyncMigrationChecksum(
		"195_add_invoice_order_active_unique_guard.sql",
		"unknown-db-checksum",
		"ddeda6f9fcf3063d9e7fcefbd308e9bc8539a938baa3ca784a30970acbccbe92",
	))
	require.False(t, shouldSyncMigrationChecksum("001_init.sql", "old", "new"))
	require.False(t, shouldSyncMigrationChecksum("unknown_migration.sql", "old", "new"))
	require.False(t, shouldSyncMigrationChecksum("", "old", "new"))
}

func TestSyncDatabaseChecksumsUsesTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d", "054_drop_legacy_cache_columns.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := syncDatabaseChecksums(
		context.Background(),
		db,
		[]migrationChecksum{
			{Filename: "054_drop_legacy_cache_columns.sql", Checksum: "82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d"},
			{Filename: "002_same.sql", Checksum: "same-checksum"},
		},
		map[string]string{
			"054_drop_legacy_cache_columns.sql": "182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"002_same.sql":                      "same-checksum",
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncDatabaseChecksumsSkipsNonAllowlistedMismatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Only the exact known checksum pair should be rewritten; arbitrary
	// mismatches must be skipped even for a migration with compatibility rules.
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c", "061_add_usage_log_request_type.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := syncDatabaseChecksums(
		context.Background(),
		db,
		[]migrationChecksum{
			{Filename: "001_init.sql", Checksum: "new-init"},
			{Filename: "061_add_usage_log_request_type.sql", Checksum: "66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c"},
			{Filename: "195_add_invoice_order_active_unique_guard.sql", Checksum: "ddeda6f9fcf3063d9e7fcefbd308e9bc8539a938baa3ca784a30970acbccbe92"},
			{Filename: "999_unrelated.sql", Checksum: "new-unrelated"},
		},
		map[string]string{
			"001_init.sql":                                  "old-init",
			"061_add_usage_log_request_type.sql":            "08a248652cbab7cfde147fc6ef8cda464f2477674e20b718312faa252e0481c0",
			"195_add_invoice_order_active_unique_guard.sql": "unknown-old-checksum",
			"999_unrelated.sql":                             "old-unrelated",
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncDatabaseChecksumsRollsBackOnUpdateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d", "054_drop_legacy_cache_columns.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c", "061_add_usage_log_request_type.sql").
		WillReturnError(errors.New("db update failed"))
	mock.ExpectRollback()

	updated, err := syncDatabaseChecksums(
		context.Background(),
		db,
		[]migrationChecksum{
			{Filename: "054_drop_legacy_cache_columns.sql", Checksum: "82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d"},
			{Filename: "061_add_usage_log_request_type.sql", Checksum: "66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c"},
		},
		map[string]string{
			"054_drop_legacy_cache_columns.sql":  "182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"061_add_usage_log_request_type.sql": "08a248652cbab7cfde147fc6ef8cda464f2477674e20b718312faa252e0481c0",
		},
	)

	require.ErrorContains(t, err, "update checksum for 061_add_usage_log_request_type.sql")
	require.Equal(t, 1, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRestoreDatabaseChecksumsRollsBackOnUpdateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("old-1", "001_init.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("old-2", "002_fail.sql").
		WillReturnError(errors.New("db update failed"))
	mock.ExpectRollback()

	restored, err := restoreDatabaseChecksums(context.Background(), db, []migrationChecksum{
		{Filename: "001_init.sql", Checksum: "old-1"},
		{Filename: "002_fail.sql", Checksum: "old-2"},
	})

	require.ErrorContains(t, err, "update checksum for 002_fail.sql")
	require.Equal(t, 1, restored)
	require.NoError(t, mock.ExpectationsWereMet())
}
