//go:build unit

package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func publicationSQLRows(request string) *sqlmock.Rows {
	columns := strings.Split(publicationColumns, ",")
	for i := range columns {
		columns[i] = strings.TrimSpace(columns[i])
	}
	return sqlmock.NewRows(columns).AddRow(int64(3), int64(7), request, "Title", "Prompt", "model", "image", "image/png", int64(8), "private/key", "backup", "digest", time.Now(), nil, "pending")
}

func TestCreationPublicationRepositoryReplayKeepsExistingRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewCreationPublicationRepository(db)
	request := uuid.NewString()
	mock.ExpectQuery(`(?s)INSERT INTO creation_publications.*ON CONFLICT \(owner_user_id, request_id\) DO NOTHING`).
		WithArgs(int64(7), request, "Title", "Prompt", "model", "image", "image/png", int64(8), "new/key", "backup", "digest").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT .* FROM creation_publications WHERE owner_user_id = \$1 AND request_id = \$2`).
		WithArgs(int64(7), request).WillReturnRows(publicationSQLRows(request))
	p, created, err := repo.CreateIfAbsent(context.Background(), &service.CreationPublication{
		OwnerUserID: 7, RequestID: request, Title: "Title", Prompt: "Prompt", Model: "model", Kind: "image", MIME: "image/png", Size: 8, StorageKey: "new/key", StorageProfileID: "backup", SHA256: "digest",
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, int64(3), p.ID)
	require.Equal(t, "private/key", p.StorageKey)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreationPublicationRepositoryOwnerAndPublicPredicates(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewCreationPublicationRepository(db)
	mock.ExpectQuery(`(?s)UPDATE creation_publications.*WHERE id = \$1 AND owner_user_id = \$2`).
		WithArgs(int64(3), int64(8)).WillReturnError(sql.ErrNoRows)
	_, err = repo.Withdraw(context.Background(), 8, 3)
	require.ErrorIs(t, err, service.ErrCreationPublicationNotFound)
	mock.ExpectQuery(`(?s)SELECT .*WHERE id = \$1 AND withdrawn_at IS NULL AND status = 'published'`).WithArgs(int64(3)).WillReturnError(sql.ErrNoRows)
	_, err = repo.GetPublic(context.Background(), 3)
	require.ErrorIs(t, err, service.ErrCreationPublicationNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
