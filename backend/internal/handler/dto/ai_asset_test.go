package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAIAssetPublicFromServiceOmitsStorageFieldsAndSanitizesMetadata(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	asset := &service.AIAsset{
		ID:              7,
		UserID:          11,
		AssetType:       service.AIAssetTypeImage,
		Status:          service.AIAssetStatusReady,
		Visibility:      service.AIVisibilityPrivate,
		ModerationState: service.AIModerationStateNormal,
		StorageKind:     "media",
		StoragePath:     "artworks/internal/path.png",
		SourceURL:       "https://cdn.example.com/a.png",
		MIMEType:        "image/png",
		Metadata: map[string]any{
			"title":                      "Artwork",
			"media_bucket":               "private-bucket",
			"media_object_key":           "artworks/object.png",
			"media_thumbnail_object_key": "artworks/thumb.png",
			"storage_path":               "nested/internal/path.png",
			"object_key":                 "nested-object-key",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	out := AIAssetPublicFromService(asset)
	require.NotNil(t, out)
	require.Equal(t, "Artwork", out.Metadata["title"])
	require.NotContains(t, out.Metadata, "media_bucket")
	require.NotContains(t, out.Metadata, "media_object_key")
	require.NotContains(t, out.Metadata, "media_thumbnail_object_key")
	require.NotContains(t, out.Metadata, "storage_path")
	require.NotContains(t, out.Metadata, "object_key")

	body, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(body), `"storage_kind"`)
	require.NotContains(t, string(body), `"storage_path"`)
	require.NotContains(t, string(body), `"media_bucket"`)
	require.NotContains(t, string(body), `"media_object_key"`)
	require.NotContains(t, string(body), `"media_thumbnail_object_key"`)
	require.NotContains(t, string(body), `"object_key"`)
}

func TestAIArtworkFromServiceSanitizesStorageMetadata(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	asset := &service.AIAsset{
		ID:              9,
		UserID:          12,
		AssetType:       service.AIAssetTypeImage,
		Status:          service.AIAssetStatusReady,
		Visibility:      service.AIVisibilityPublic,
		ModerationState: service.AIModerationStateNormal,
		SourceURL:       "https://cdn.example.com/render.png",
		Metadata: map[string]any{
			"title":                      "Rendered",
			"prompt":                     "draw a lighthouse",
			"thumbnail_url":              "https://cdn.example.com/render-thumb.png",
			"media_bucket":               "private-bucket",
			"media_object_key":           "artworks/object.png",
			"media_thumbnail_object_key": "artworks/thumb.png",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	out := AIArtworkFromService(asset, asset.UserID)
	require.NotNil(t, out)
	require.Equal(t, "Rendered", out.Title)
	require.NotContains(t, out.Metadata, "media_bucket")
	require.NotContains(t, out.Metadata, "media_object_key")
	require.NotContains(t, out.Metadata, "media_thumbnail_object_key")

	body, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(body), `"media_bucket"`)
	require.NotContains(t, string(body), `"media_object_key"`)
	require.NotContains(t, string(body), `"media_thumbnail_object_key"`)
}
