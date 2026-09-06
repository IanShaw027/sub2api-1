//go:build unit

package repository

import (
	"context"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestImageStorageRenewsURLWithoutUpload(t *testing.T) {
	ctx := context.Background()
	cfg := &config.ImageStorageConfig{
		Endpoint: "https://s3.example.test", Region: "auto", Bucket: "images",
		AccessKeyID: "test-key", SecretAccessKey: "test-secret", ForcePathStyle: true,
	}
	storage, err := NewS3ImageStorage(ctx, cfg)
	require.NoError(t, err)
	raw, err := storage.URL(ctx, "prefix/result.png")
	require.NoError(t, err)
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "/images/prefix/result.png", parsed.Path)
	require.NotEmpty(t, parsed.Query().Get("X-Amz-Signature"))
	require.Equal(t, "86400", parsed.Query().Get("X-Amz-Expires"))
	identity := storage.StorageID()
	cfg.Region = ""
	cfg.SecretAccessKey = "rotated-secret"
	cfg.PublicBaseURL = "https://cdn.example.test/images"
	rotated, err := NewS3ImageStorage(ctx, cfg)
	require.NoError(t, err)
	require.Equal(t, identity, rotated.StorageID(), "credential rotation does not change object identity")
	publicURL, err := rotated.URL(ctx, "prefix/result.png")
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.test/images/prefix/result.png", publicURL)
	cfg.Bucket = "other-images"
	other, err := NewS3ImageStorage(ctx, cfg)
	require.NoError(t, err)
	require.NotEqual(t, identity, other.StorageID())
}
