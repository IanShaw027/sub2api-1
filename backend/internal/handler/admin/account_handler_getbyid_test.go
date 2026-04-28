package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type getByIDAccountAdminService struct {
	*stubAdminService
	account service.Account
}

func (s *getByIDAccountAdminService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if s.account.ID == id {
		acc := s.account
		return &acc, nil
	}
	return s.stubAdminService.GetAccount(context.Background(), id)
}

func setupAccountGetByIDRouter(adminSvc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts/:id", handler.GetByID)
	return router
}

func TestAccountHandlerGetByID_PreservesSensitiveCredentialsForDetail(t *testing.T) {
	svc := &getByIDAccountAdminService{
		stubAdminService: newStubAdminService(),
		account: service.Account{
			ID:       42,
			Name:     "openai-apikey",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeAPIKey,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"api_key":       "sk-secret",
				"refresh_token": "refresh-secret",
				"base_url":      "https://api.example.com",
			},
		},
	}
	router := setupAccountGetByIDRouter(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/42", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data struct {
			Credentials map[string]any `json:"credentials"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "sk-secret", resp.Data.Credentials["api_key"])
	require.Equal(t, "refresh-secret", resp.Data.Credentials["refresh_token"])
	require.Equal(t, "https://api.example.com", resp.Data.Credentials["base_url"])
}
