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
