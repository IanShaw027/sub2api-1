package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type webProfileAdminService struct {
	*stubAdminService
	account      service.Account
	updateID     int64
	updateExtra  map[string]any
	updateCalled bool
}

func (s *webProfileAdminService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if s.account.ID == id {
		account := s.account
		return &account, nil
	}
	return s.stubAdminService.GetAccount(context.Background(), id)
}

func (s *webProfileAdminService) UpdateAccountExtra(_ context.Context, id int64, updates map[string]any) error {
	s.updateCalled = true
	s.updateID = id
	s.updateExtra = updates
	return nil
}

func setupAccountWebProfileImportRouter(svc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/:id/openai-web-profile/import", handler.ImportOpenAIWebProfile)
	return router
}

func TestAccountHandlerImportOpenAIWebProfile_Success(t *testing.T) {
	svc := &webProfileAdminService{
		stubAdminService: newStubAdminService(),
		account: service.Account{
			ID:       42,
			Name:     "openai-oauth",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
		},
	}
	router := setupAccountWebProfileImportRouter(svc)

	body := map[string]any{
		"web_profile": map[string]any{
			"source":        "capture",
			"captured_at":   "2026-04-29T10:00:00Z",
			"proxy_id":      7,
			"proxy_hash":    "proxy-hash",
			"user_agent":    "Mozilla/5.0 Chrome/136.0.0.0",
			"oai_device_id": "device-id",
			"cookies": []any{
				map[string]any{"name": "oai-did", "value": "cookie-secret", "domain": ".chatgpt.com", "path": "/", "httponly": true},
			},
		},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/openai-web-profile/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, svc.updateCalled)
	require.Equal(t, int64(42), svc.updateID)

	stored, ok := svc.updateExtra["web_profile"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Mozilla/5.0 Chrome/136.0.0.0", stored["user_agent"])
	cookies, ok := stored["cookies"].([]map[string]any)
	require.True(t, ok)
	require.Equal(t, "cookie-secret", cookies[0]["value"])

	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, true, resp.Data["has_web_profile"])
	require.Equal(t, "capture", resp.Data["source"])
	require.Equal(t, "2026-04-29T10:00:00Z", resp.Data["captured_at"])
	require.Equal(t, float64(136), resp.Data["ua_major"])
	require.Equal(t, true, resp.Data["has_cookie_jar"])
	require.NotEmpty(t, resp.Data["cookie_names_digest"])
	require.Equal(t, float64(7), resp.Data["proxy_id"])
	require.Equal(t, "proxy-hash", resp.Data["proxy_hash"])
	account, ok := resp.Data["account"].(map[string]any)
	require.True(t, ok)
	accountExtra, ok := account["extra"].(map[string]any)
	require.True(t, ok)
	accountProfile, ok := accountExtra["web_profile"].(map[string]any)
	require.True(t, ok)
	accountCookies, ok := accountProfile["cookies"].([]any)
	require.True(t, ok)
	accountCookie, ok := accountCookies[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "oai-did", accountCookie["name"])
	require.NotContains(t, accountCookie, "value")
}

func TestAccountHandlerImportOpenAIWebProfile_AcceptsContentJSON(t *testing.T) {
	svc := &webProfileAdminService{
		stubAdminService: newStubAdminService(),
		account: service.Account{
			ID:       42,
			Name:     "openai-oauth",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
		},
	}
	router := setupAccountWebProfileImportRouter(svc)

	content := `{"web_profile":{"user_agent":"Mozilla/5.0 Chrome/136.0.0.0","cookies":[{"name":"oai-did","value":"cookie-secret","domain":".chatgpt.com","path":"/"}]}}`
	body, err := json.Marshal(map[string]any{"content": content})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/openai-web-profile/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, svc.updateCalled)
	stored, ok := svc.updateExtra["web_profile"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Mozilla/5.0 Chrome/136.0.0.0", stored["user_agent"])
	require.NotContains(t, rec.Body.String(), "cookie-secret")
}

func TestAccountHandlerImportOpenAIWebProfile_RejectsNonOpenAI(t *testing.T) {
	svc := &webProfileAdminService{
		stubAdminService: newStubAdminService(),
		account:          service.Account{ID: 42, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Status: service.StatusActive},
	}
	router := setupAccountWebProfileImportRouter(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/openai-web-profile/import", bytes.NewBufferString(`{"web_profile":{"user_agent":"Mozilla/5.0 Chrome/136.0.0.0"}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, svc.updateCalled)
}

func TestAccountHandlerImportOpenAIWebProfile_RejectsEmptyProfile(t *testing.T) {
	svc := &webProfileAdminService{
		stubAdminService: newStubAdminService(),
		account:          service.Account{ID: 42, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive},
	}
	router := setupAccountWebProfileImportRouter(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/openai-web-profile/import", bytes.NewBufferString(`{"web_profile":{"source":"capture"}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, svc.updateCalled)
}

func TestAccountHandlerImportOpenAIWebProfile_ResponseDoesNotLeakCookieValue(t *testing.T) {
	svc := &webProfileAdminService{
		stubAdminService: newStubAdminService(),
		account:          service.Account{ID: 42, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive},
	}
	router := setupAccountWebProfileImportRouter(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/openai-web-profile/import", bytes.NewBufferString(`{"web_profile":{"cookies":[{"name":"oai-did","value":"cookie-secret","domain":".chatgpt.com","path":"/"}]}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cookie-secret")
	require.Contains(t, rec.Body.String(), "cookie_names_digest")
}
