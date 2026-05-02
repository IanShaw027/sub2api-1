package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMediaAssetPublicFromServiceOmitsInternalStorageFields(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	asset := &service.MediaAsset{
		ID:                 5,
		BizType:            "ai_image",
		BizID:              "123",
		Bucket:             "private-media-bucket",
		ObjectKey:          "artworks/object.png",
		ThumbnailObjectKey: "artworks/thumb.png",
		Visibility:         service.MediaVisibilityPrivate,
		MIMEType:           "image/png",
		SizeBytes:          1024,
		Status:             service.MediaStatusActive,
		OriginalFileName:   "object.png",
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	out := MediaAssetPublicFromService(asset, "https://signed.example.com/image", "https://signed.example.com/thumb")
	require.NotNil(t, out)

	body, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(body), `"bucket"`)
	require.NotContains(t, string(body), `"object_key"`)
	require.NotContains(t, string(body), `"thumbnail_object_key"`)
}
