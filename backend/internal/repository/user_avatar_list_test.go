//go:build unit

package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPopulateUserAvatarsLoadsPersistedAvatarsInBatch(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)SELECT user_id, storage_provider, storage_key, url, content_type, byte_size, sha256\s+FROM user_avatars\s+WHERE user_id = ANY\(\$1\)`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "storage_provider", "storage_key", "url", "content_type", "byte_size", "sha256",
		}).AddRow(
			int64(11), "inline", "", "data:image/webp;base64,YXZhdGFy", "image/webp", 6, "digest",
		))

	users := []service.User{{ID: 11}, {ID: 12}}
	userMap := map[int64]*service.User{11: &users[0], 12: &users[1]}
	repo := newUserRepositoryWithSQL(nil, db)

	require.NoError(t, repo.populateUserAvatars(context.Background(), userMap))
	require.Equal(t, "data:image/webp;base64,YXZhdGFy", users[0].AvatarURL)
	require.Equal(t, "inline", users[0].AvatarSource)
	require.Equal(t, "image/webp", users[0].AvatarMIME)
	require.Equal(t, 6, users[0].AvatarByteSize)
	require.Equal(t, "digest", users[0].AvatarSHA256)
	require.Empty(t, users[1].AvatarURL)
	require.NoError(t, mock.ExpectationsWereMet())
}
