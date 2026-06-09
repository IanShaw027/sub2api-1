package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminAIHandlerGetAssetUsesInboundRequestBaseURLForPrivateMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := int64(42)
	mediaSvc := service.NewMediaService(&settingHandlerMediaRepoStub{
		assets: map[int64]*service.MediaAsset{
			321: &service.MediaAsset{
				ID:          321,
				Visibility:  service.MediaVisibilityPrivate,
				Status:      service.MediaStatusActive,
				OwnerUserID: &ownerID,
			},
		},
	}, &settingHandlerMediaStoreStub{}, adminAIMediaBaseURLTestConfig())
	aiSvc := service.NewAICenterService(&adminAIRequestBaseURLRepoStub{
		asset: &service.AIAsset{
			ID:         99,
			UserID:     ownerID,
			Visibility: service.AIVisibilityPrivate,
			Status:     service.AIAssetStatusReady,
			Metadata:   map[string]any{"media_asset_id": int64(321)},
		},
	}, mediaSvc)
	h := NewAIHandler(aiSvc, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 88})
	c.Request = httptest.NewRequest(http.MethodGet, "https://admin.caller.example/api/v1/admin/ai/assets/99", nil)
	c.Params = gin.Params{{Key: "id", Value: "99"}}

	h.GetAsset(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	payload, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	sourceURL, ok := payload["source_url"].(string)
	require.True(t, ok)
	require.Contains(t, sourceURL, "https://admin.caller.example/api/v1/media/download/321?")
	require.NotContains(t, sourceURL, "https://media.example.com/api/v1/media/download/321?")
}

type adminAIRequestBaseURLRepoStub struct {
	service.AICenterRepository
	asset *service.AIAsset
}

func (s *adminAIRequestBaseURLRepoStub) GetAssetByID(_ context.Context, id int64) (*service.AIAsset, error) {
	if s.asset != nil && s.asset.ID == id {
		cloned := *s.asset
		cloned.Metadata = cloneAdminAIMap(s.asset.Metadata)
		return &cloned, nil
	}
	return nil, service.ErrAIAssetNotFound
}

func (s *adminAIRequestBaseURLRepoStub) ListAssets(_ context.Context, _ int64, _ bool, _ pagination.PaginationParams, _ service.AIListAssetsFilter) ([]service.AIAsset, *pagination.PaginationResult, error) {
	if s.asset == nil {
		return nil, &pagination.PaginationResult{Total: 0, Page: 1, PageSize: 20}, nil
	}
	cloned := *s.asset
	cloned.Metadata = cloneAdminAIMap(s.asset.Metadata)
	return []service.AIAsset{cloned}, &pagination.PaginationResult{Total: 1, Page: 1, PageSize: 20}, nil
}

func adminAIMediaBaseURLTestConfig() *config.Config {
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
