package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAdminRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()

	userHandler := NewUserHandler(adminSvc, nil, nil, nil)
	groupHandler := NewGroupHandler(adminSvc, nil, nil)
	proxyHandler := NewProxyHandler(adminSvc)
	redeemHandler := NewRedeemHandler(adminSvc, nil)

	router.GET("/api/v1/admin/users", userHandler.List)
	router.GET("/api/v1/admin/users/:id", userHandler.GetByID)
	router.POST("/api/v1/admin/users/:id/auth-identities", userHandler.BindAuthIdentity)
	router.POST("/api/v1/admin/users", userHandler.Create)
	router.PUT("/api/v1/admin/users/:id", userHandler.Update)
	router.DELETE("/api/v1/admin/users/:id", userHandler.Delete)
	router.POST("/api/v1/admin/users/:id/balance", userHandler.UpdateBalance)
	router.GET("/api/v1/admin/users/:id/api-keys", userHandler.GetUserAPIKeys)
	router.GET("/api/v1/admin/users/:id/usage", userHandler.GetUserUsage)

	router.GET("/api/v1/admin/groups", groupHandler.List)
	router.GET("/api/v1/admin/groups/all", groupHandler.GetAll)
	router.GET("/api/v1/admin/groups/:id/models-list-candidates", groupHandler.GetModelsListCandidates)
	router.GET("/api/v1/admin/groups/:id", groupHandler.GetByID)
	router.POST("/api/v1/admin/groups", groupHandler.Create)
	router.PUT("/api/v1/admin/groups/:id", groupHandler.Update)
	router.DELETE("/api/v1/admin/groups/:id", groupHandler.Delete)
	router.GET("/api/v1/admin/groups/:id/stats", groupHandler.GetStats)
	router.GET("/api/v1/admin/groups/:id/api-keys", groupHandler.GetGroupAPIKeys)

	router.GET("/api/v1/admin/proxies", proxyHandler.List)
	router.GET("/api/v1/admin/proxies/all", proxyHandler.GetAll)
	router.GET("/api/v1/admin/proxies/:id", proxyHandler.GetByID)
	router.POST("/api/v1/admin/proxies", proxyHandler.Create)
	router.PUT("/api/v1/admin/proxies/:id", proxyHandler.Update)
	router.DELETE("/api/v1/admin/proxies/:id", proxyHandler.Delete)
	router.POST("/api/v1/admin/proxies/batch-delete", proxyHandler.BatchDelete)
	router.POST("/api/v1/admin/proxies/:id/test", proxyHandler.Test)
	router.POST("/api/v1/admin/proxies/:id/quality-check", proxyHandler.CheckQuality)
	router.GET("/api/v1/admin/proxies/:id/stats", proxyHandler.GetStats)
	router.GET("/api/v1/admin/proxies/:id/accounts", proxyHandler.GetProxyAccounts)

	router.GET("/api/v1/admin/redeem-codes", redeemHandler.List)
	router.GET("/api/v1/admin/redeem-codes/:id", redeemHandler.GetByID)
	router.POST("/api/v1/admin/redeem-codes", redeemHandler.Generate)
	router.DELETE("/api/v1/admin/redeem-codes/:id", redeemHandler.Delete)
	router.POST("/api/v1/admin/redeem-codes/batch-delete", redeemHandler.BatchDelete)
	router.POST("/api/v1/admin/redeem-codes/:id/expire", redeemHandler.Expire)
	router.GET("/api/v1/admin/redeem-codes/:id/stats", redeemHandler.GetStats)

	return router, adminSvc
}

func TestUserHandlerEndpoints(t *testing.T) {
	router, _ := setupAdminRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&page_size=20", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/1", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	bindBody := map[string]any{
		"provider_type":    "wechat",
		"provider_key":     "wechat-main",
		"provider_subject": "union-123",
		"metadata":         map[string]any{"source": "admin-repair"},
		"channel": map[string]any{
			"channel":         "open",
			"channel_app_id":  "wx-open",
			"channel_subject": "openid-123",
		},
	}
	body, _ := json.Marshal(bindBody)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/1/auth-identities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	createBody := map[string]any{"email": "new@example.com", "password": "pass123", "balance": 1, "concurrency": 2}
	body, _ = json.Marshal(createBody)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	updateBody := map[string]any{"email": "updated@example.com"}
	body, _ = json.Marshal(updateBody)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/1/balance", bytes.NewBufferString(`{"balance":1,"operation":"add"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/1/api-keys", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/1/usage?period=today", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestUserHandlerBindAuthIdentityMapsRequest(t *testing.T) {
	router, adminSvc := setupAdminRouter()

	body, err := json.Marshal(map[string]any{
		"provider_type":    "oidc",
		"provider_key":     "https://issuer.example",
		"provider_subject": "subject-123",
		"issuer":           "https://issuer.example",
		"metadata":         map[string]any{"report_id": 12},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/9/auth-identities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(9), adminSvc.boundAuthIdentityFor)
	require.NotNil(t, adminSvc.boundAuthIdentity)
	require.Equal(t, "oidc", adminSvc.boundAuthIdentity.ProviderType)
	require.Equal(t, "https://issuer.example", adminSvc.boundAuthIdentity.ProviderKey)
	require.Equal(t, "subject-123", adminSvc.boundAuthIdentity.ProviderSubject)
	require.Nil(t, adminSvc.boundAuthIdentity.Channel)
	require.Equal(t, float64(12), adminSvc.boundAuthIdentity.Metadata["report_id"])
}

func TestGroupHandlerEndpoints(t *testing.T) {
	router, _ := setupAdminRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/all", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/2", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/0/models-list-candidates?platform=openai", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "gpt-5.5")

	body, _ := json.Marshal(map[string]any{"name": "new", "platform": "anthropic", "subscription_type": "standard"})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body, _ = json.Marshal(map[string]any{"name": "update"})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/groups/2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/groups/2", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/2/stats", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/2/api-keys", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestGroupHandlerMapsVideoGenerationFields(t *testing.T) {
	router, adminSvc := setupAdminRouter()

	body, err := json.Marshal(map[string]any{
		"name":                      "video-create",
		"platform":                  "grok",
		"subscription_type":         "standard",
		"allow_video_generation":    true,
		"video_generation_route":    "native",
		"video_price_480p_per_sec":  0.01,
		"video_price_720p_per_sec":  0.02,
		"video_price_1080p_per_sec": 0.03,
		"video_price_4k_per_sec":    0.04,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdGroups, 1)
	created := adminSvc.createdGroups[0]
	require.True(t, created.AllowVideoGeneration)
	require.Equal(t, "native", created.VideoGenerationRoute)
	require.NotNil(t, created.VideoPrice480pPerSec)
	require.InDelta(t, 0.01, *created.VideoPrice480pPerSec, 0.000001)
	require.NotNil(t, created.VideoPrice720pPerSec)
	require.InDelta(t, 0.02, *created.VideoPrice720pPerSec, 0.000001)
	require.NotNil(t, created.VideoPrice1080pPerSec)
	require.InDelta(t, 0.03, *created.VideoPrice1080pPerSec, 0.000001)
	require.NotNil(t, created.VideoPrice4kPerSec)
	require.InDelta(t, 0.04, *created.VideoPrice4kPerSec, 0.000001)

	updateBody, err := json.Marshal(map[string]any{
		"allow_video_generation":    false,
		"video_generation_route":    "native",
		"video_price_480p_per_sec":  0.11,
		"video_price_720p_per_sec":  0.12,
		"video_price_1080p_per_sec": 0.13,
		"video_price_4k_per_sec":    0.14,
	})
	require.NoError(t, err)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/groups/2", bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{2}, adminSvc.updatedGroupIDs)
	require.Len(t, adminSvc.updatedGroups, 1)
	updated := adminSvc.updatedGroups[0]
	require.NotNil(t, updated.AllowVideoGeneration)
	require.False(t, *updated.AllowVideoGeneration)
	require.NotNil(t, updated.VideoGenerationRoute)
	require.Equal(t, "native", *updated.VideoGenerationRoute)
	require.NotNil(t, updated.VideoPrice480pPerSec)
	require.InDelta(t, 0.11, *updated.VideoPrice480pPerSec, 0.000001)
	require.NotNil(t, updated.VideoPrice720pPerSec)
	require.InDelta(t, 0.12, *updated.VideoPrice720pPerSec, 0.000001)
	require.NotNil(t, updated.VideoPrice1080pPerSec)
	require.InDelta(t, 0.13, *updated.VideoPrice1080pPerSec, 0.000001)
	require.NotNil(t, updated.VideoPrice4kPerSec)
	require.InDelta(t, 0.14, *updated.VideoPrice4kPerSec, 0.000001)

	nullBody := []byte(`{
		"video_price_480p_per_sec": null,
		"video_price_720p_per_sec": null,
		"video_price_1080p_per_sec": null,
		"video_price_4k_per_sec": null
	}`)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/groups/2", bytes.NewReader(nullBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.updatedGroups, 2)
	cleared := adminSvc.updatedGroups[1]
	require.True(t, cleared.VideoPrice480pPerSecSet)
	require.Nil(t, cleared.VideoPrice480pPerSec)
	require.True(t, cleared.VideoPrice720pPerSecSet)
	require.Nil(t, cleared.VideoPrice720pPerSec)
	require.True(t, cleared.VideoPrice1080pPerSecSet)
	require.Nil(t, cleared.VideoPrice1080pPerSec)
	require.True(t, cleared.VideoPrice4kPerSecSet)
	require.Nil(t, cleared.VideoPrice4kPerSec)
}

func TestGroupHandlerMapsImages2APIPriceFields(t *testing.T) {
	router, adminSvc := setupAdminRouter()

	body, err := json.Marshal(map[string]any{
		"name":                "images2api-create",
		"platform":            "openai",
		"subscription_type":   "standard",
		"images2api_price_1k": 0.21,
		"images2api_price_2k": 0.42,
		"images2api_price_4k": 0.84,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdGroups, 1)
	created := adminSvc.createdGroups[0]
	require.NotNil(t, created.Images2APIPrice1K)
	require.InDelta(t, 0.21, *created.Images2APIPrice1K, 0.000001)
	require.NotNil(t, created.Images2APIPrice2K)
	require.InDelta(t, 0.42, *created.Images2APIPrice2K, 0.000001)
	require.NotNil(t, created.Images2APIPrice4K)
	require.InDelta(t, 0.84, *created.Images2APIPrice4K, 0.000001)

	updateBody, err := json.Marshal(map[string]any{
		"images2api_price_1k": 0.31,
		"images2api_price_2k": -1,
		"images2api_price_4k": 0,
	})
	require.NoError(t, err)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/groups/2", bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{2}, adminSvc.updatedGroupIDs)
	require.Len(t, adminSvc.updatedGroups, 1)
	updated := adminSvc.updatedGroups[0]
	require.NotNil(t, updated.Images2APIPrice1K)
	require.InDelta(t, 0.31, *updated.Images2APIPrice1K, 0.000001)
	require.NotNil(t, updated.Images2APIPrice2K)
	require.InDelta(t, -1, *updated.Images2APIPrice2K, 0.000001)
	require.NotNil(t, updated.Images2APIPrice4K)
	require.InDelta(t, 0, *updated.Images2APIPrice4K, 0.000001)
}

func TestProxyHandlerEndpoints(t *testing.T) {
	router, _ := setupAdminRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/proxies", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/proxies/all", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/proxies/4", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body, _ := json.Marshal(map[string]any{"name": "proxy", "protocol": "http", "host": "localhost", "port": 8080})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body, _ = json.Marshal(map[string]any{"name": "proxy2"})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/proxies/4", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/proxies/4", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/batch-delete", bytes.NewBufferString(`{"ids":[1,2]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/4/test", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/4/quality-check", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/proxies/4/stats", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/proxies/4/accounts", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRedeemHandlerEndpoints(t *testing.T) {
	router, _ := setupAdminRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/redeem-codes", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/redeem-codes/5", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body, _ := json.Marshal(map[string]any{"count": 1, "type": "balance", "value": 10})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/redeem-codes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/redeem-codes/5", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/redeem-codes/batch-delete", bytes.NewBufferString(`{"ids":[1,2]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/redeem-codes/5/expire", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/redeem-codes/5/stats", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}
