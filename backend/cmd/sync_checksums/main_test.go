package main

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSyncDatabaseChecksumsUsesTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("new-checksum", "001_init.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := syncDatabaseChecksums(
		context.Background(),
		db,
		[]migrationChecksum{
			{Filename: "001_init.sql", Checksum: "new-checksum"},
			{Filename: "002_same.sql", Checksum: "same-checksum"},
		},
		map[string]string{
			"001_init.sql": "old-checksum",
			"002_same.sql": "same-checksum",
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
		WithArgs("new-1", "001_init.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE schema_migrations SET checksum = \$1 WHERE filename = \$2`).
		WithArgs("new-2", "002_fail.sql").
		WillReturnError(errors.New("db update failed"))
	mock.ExpectRollback()

	updated, err := syncDatabaseChecksums(
		context.Background(),
		db,
		[]migrationChecksum{
			{Filename: "001_init.sql", Checksum: "new-1"},
			{Filename: "002_fail.sql", Checksum: "new-2"},
		},
		map[string]string{
			"001_init.sql": "old-1",
			"002_fail.sql": "old-2",
		},
	)

	require.ErrorContains(t, err, "update checksum for 002_fail.sql")
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
