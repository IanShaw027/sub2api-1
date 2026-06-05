package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMediaRepositoryCreateUsesVisibilityBeforeThumbnailMIMEType(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &mediaRepository{db: db}
	asset := &service.MediaAsset{
		BizType:            "ai_image",
		BizID:              "123",
		Bucket:             "media-bucket",
		ObjectKey:          "artworks/image.png",
		ThumbnailObjectKey: "artworks/thumb.png",
		Visibility:         service.MediaVisibilityPrivate,
		ThumbnailMIMEType:  "image/webp",
		MIMEType:           "image/png",
		SizeBytes:          1024,
		SHA256:             "deadbeef",
		Status:             service.MediaStatusActive,
		OriginalFileName:   "image.png",
	}
	now := time.Now().UTC()

	mock.ExpectQuery("INSERT INTO media_assets").
		WithArgs(
			asset.BizType,
			asset.BizID,
			asset.StorageProfileID,
			asset.Bucket,
			asset.ObjectKey,
			asset.ThumbnailObjectKey,
			asset.Visibility,
			asset.ThumbnailMIMEType,
			asset.MIMEType,
			asset.SizeBytes,
			nil,
			nil,
			asset.SHA256,
			nil,
			asset.Status,
			asset.OriginalFileName,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(9), now, now))

	err = repo.Create(context.Background(), asset)
	require.NoError(t, err)
	require.Equal(t, int64(9), asset.ID)
	require.Equal(t, now, asset.CreatedAt)
	require.Equal(t, now, asset.UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMediaRepositoryGetByObjectKey_MatchesThumbnailObjectKey(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &mediaRepository{db: db}
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM media_assets\s+WHERE \(object_key = \$1 OR thumbnail_object_key = \$1\) AND status = \$2 AND bucket = \$3\s+ORDER BY CASE WHEN object_key = \$1 THEN 0 ELSE 1 END, id DESC LIMIT 1`).
		WithArgs("media/avatar_thumbnail/user-42/thumb.png", service.MediaStatusActive, "old-bucket").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "biz_type", "biz_id", "storage_profile_id", "bucket", "object_key", "thumbnail_object_key", "thumbnail_mime_type", "visibility",
			"mime_type", "size_bytes", "width", "height", "sha256", "owner_user_id", "status",
			"original_file_name", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			int64(42), "avatar", "user-42", "old", "old-bucket", "media/avatar/user-42/original.png",
			"media/avatar_thumbnail/user-42/thumb.png", "image/webp", service.MediaVisibilityPublic,
			"image/png", int64(2048), nil, nil, "deadbeef", nil, service.MediaStatusActive,
			"avatar.png", now, now, nil,
		))

	asset, err := repo.GetByObjectKey(context.Background(), "old-bucket", "media/avatar_thumbnail/user-42/thumb.png")
	require.NoError(t, err)
	require.NotNil(t, asset)
	require.Equal(t, int64(42), asset.ID)
	require.Equal(t, "media/avatar_thumbnail/user-42/thumb.png", asset.ThumbnailObjectKey)
	require.NoError(t, mock.ExpectationsWereMet())
}
