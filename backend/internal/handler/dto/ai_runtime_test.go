package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAIRuntimeFromMediaRuntimeIncludesChatAndThumbnailTemplate(t *testing.T) {
	runtime := AIRuntimeFromMediaRuntime(service.MediaRuntimeInfo{
		Enabled:                   true,
		Bucket:                    "internal-media-bucket",
		PublicBaseURL:             "https://source.qazwc.com",
		PublicEndpointTemplate:    "https://source.qazwc.com/{object_key}",
		ThumbnailEndpointTemplate: "https://source.qazwc.com/{thumbnail_object_key}",
		DownloadEndpointTemplate:  "/api/v1/media/download/{id}",
		ThumbnailDownloadTemplate: "/api/v1/media/download/{id}/thumbnail",
	})

	require.Equal(t, []string{"responses", "chat_completions"}, runtime.Chat.SupportedEntries)
	require.Equal(t, "https://source.qazwc.com/{object_key}", runtime.Media.PublicEndpointTemplate)
	require.Equal(t, "https://source.qazwc.com/{thumbnail_object_key}", runtime.Media.ThumbnailEndpointTemplate)
	require.Equal(t, "/api/v1/media/download/{id}/thumbnail", runtime.Media.ThumbnailDownloadTemplate)

	body, err := json.Marshal(runtime)
	require.NoError(t, err)
	require.Contains(t, string(body), `"chat":{"supported_entries":["responses","chat_completions"]}`)
	require.Contains(t, string(body), `"public_endpoint_template":"https://source.qazwc.com/{object_key}"`)
	require.Contains(t, string(body), `"thumbnail_endpoint_template":"https://source.qazwc.com/{thumbnail_object_key}"`)
	require.Contains(t, string(body), `"thumbnail_download_template":"/api/v1/media/download/{id}/thumbnail"`)
	require.NotContains(t, string(body), `"/api/v1/media/public/{id}"`)
	require.NotContains(t, string(body), `"bucket":`)
}
