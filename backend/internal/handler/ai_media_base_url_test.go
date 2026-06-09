package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAIHandlerGetAssetUsesInboundRequestBaseURLForPrivateMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := int64(42)
	mediaSvc := service.NewMediaService(&ticketHandlerMediaRepoStub{
		asset: &service.MediaAsset{
			ID:          321,
			Visibility:  service.MediaVisibilityPrivate,
			Status:      service.MediaStatusActive,
			OwnerUserID: &ownerID,
		},
	}, &ticketHandlerMediaStoreStub{}, aiMediaBaseURLTestConfig())
	aiSvc := service.NewAICenterService(&aiGatewayBridgeRepoStub{
		asset: &service.AIAsset{
			ID:         99,
			UserID:     ownerID,
			Visibility: service.AIVisibilityPrivate,
			Status:     service.AIAssetStatusReady,
			Metadata:   map[string]any{"media_asset_id": int64(321)},
		},
	}, mediaSvc)
	h := &AIHandler{aiService: aiSvc}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: ownerID})
	c.Request = httptest.NewRequest(http.MethodGet, "https://api.caller.example/api/v1/ai/assets/99", nil)
	c.Params = gin.Params{{Key: "id", Value: "99"}}

	h.GetAsset(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	payload, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	sourceURL, ok := payload["source_url"].(string)
	require.True(t, ok)
	require.Contains(t, sourceURL, "https://api.caller.example/api/v1/media/download/321?")
	require.NotContains(t, sourceURL, "https://media.example.com/api/v1/media/download/321?")
}

func aiMediaBaseURLTestConfig() *config.Config {
	return &config.Config{
		Media: config.MediaConfig{
			Enabled:               true,
			Endpoint:              "https://s3.example.com",
			Bucket:                "media",
			AccessKeyID:           "test-ak",
			SecretAccessKey:       "test-sk",
			PublicBaseURL:         "https://media.example.com",
			PresignExpiryMinutes:  10,
			DownloadSigningSecret: "secret",
		},
	}
}
