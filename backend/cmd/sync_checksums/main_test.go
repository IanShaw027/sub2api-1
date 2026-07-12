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

	require.True(t, shouldSyncMigrationChecksum("054_drop_legacy_cache_columns.sql"))
	require.True(t, shouldSyncMigrationChecksum("199_batch_image_idempotency_unique.sql"))
	require.False(t, shouldSyncMigrationChecksum("001_init.sql"))
	require.False(t, shouldSyncMigrationChecksum("unknown_migration.sql"))
	require.False(t, shouldSyncMigrationChecksum(""))
}

func TestSyncDatabaseChecksumsUsesTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("new-checksum", "054_drop_legacy_cache_columns.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := syncDatabaseChecksums(
		context.Background(),
		db,
		[]migrationChecksum{
			{Filename: "054_drop_legacy_cache_columns.sql", Checksum: "new-checksum"},
			{Filename: "002_same.sql", Checksum: "same-checksum"},
		},
		map[string]string{
			"054_drop_legacy_cache_columns.sql": "old-checksum",
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

	// Only the allowlisted migration should be rewritten; arbitrary mismatches
	// must be skipped even when DEPLOY_SYNC_SCHEMA_CHECKSUMS is opted in.
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("new-allowlisted", "061_add_usage_log_request_type.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := syncDatabaseChecksums(
		context.Background(),
		db,
		[]migrationChecksum{
			{Filename: "001_init.sql", Checksum: "new-init"},
			{Filename: "061_add_usage_log_request_type.sql", Checksum: "new-allowlisted"},
			{Filename: "999_unrelated.sql", Checksum: "new-unrelated"},
		},
		map[string]string{
			"001_init.sql":                     "old-init",
			"061_add_usage_log_request_type.sql": "old-allowlisted",
			"999_unrelated.sql":                "old-unrelated",
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
		WithArgs("new-1", "054_drop_legacy_cache_columns.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("new-2", "061_add_usage_log_request_type.sql").
		WillReturnError(errors.New("db update failed"))
	mock.ExpectRollback()

	updated, err := syncDatabaseChecksums(
		context.Background(),
		db,
		[]migrationChecksum{
			{Filename: "054_drop_legacy_cache_columns.sql", Checksum: "new-1"},
			{Filename: "061_add_usage_log_request_type.sql", Checksum: "new-2"},
		},
		map[string]string{
			"054_drop_legacy_cache_columns.sql": "old-1",
			"061_add_usage_log_request_type.sql": "old-2",
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
