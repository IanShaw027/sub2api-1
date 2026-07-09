//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type refreshTierGeminiOAuthClientStub struct{}

func (refreshTierGeminiOAuthClientStub) ExchangeCode(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
	return nil, nil
}

func (refreshTierGeminiOAuthClientStub) RefreshToken(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
	return &geminicli.TokenResponse{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}, nil
}

type refreshTierAccountAdminService struct {
	*stubAdminService
	accounts map[int64]service.Account
}

func (s *refreshTierAccountAdminService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if account, ok := s.accounts[id]; ok {
		acc := account
		return &acc, nil
	}
	return s.stubAdminService.GetAccount(context.Background(), id)
}

func (s *refreshTierAccountAdminService) GetAccountsByIDs(_ context.Context, ids []int64) ([]*service.Account, error) {
	out := make([]*service.Account, 0, len(ids))
	for _, id := range ids {
		if account, ok := s.accounts[id]; ok {
			acc := account
			out = append(out, &acc)
			continue
		}
		account := service.Account{ID: id, Name: "missing", Status: service.StatusActive}
		out = append(out, &account)
	}
	return out, nil
}

func (s *refreshTierAccountAdminService) UpdateAccount(_ context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	account, ok := s.accounts[id]
	if !ok {
		account = service.Account{ID: id, Name: "missing", Status: service.StatusActive}
	}
	if input.Name != "" {
		account.Name = input.Name
	}
	if input.Type != "" {
		account.Type = input.Type
	}
	if input.Credentials != nil {
		if account.Credentials == nil {
			account.Credentials = map[string]any{}
		}
		for k, v := range input.Credentials {
			account.Credentials[k] = v
		}
	}
	s.accounts[id] = account
	acc := account
	return &acc, nil
}

func setupRefreshTierRouter(svc service.AdminService, geminiSvc *service.GeminiOAuthService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(svc, nil, nil, geminiSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/:id/refresh-tier", handler.RefreshTier)
	router.POST("/api/v1/admin/accounts/batch-refresh-tier", handler.BatchRefreshTier)
	return router
}

func TestAccountHandlerRefreshTier_UsesBackwardCompatibleRefreshPath(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	adminSvc := &refreshTierAccountAdminService{
		stubAdminService: newStubAdminService(),
		accounts: map[int64]service.Account{
			42: {
				ID:       42,
				Name:     "gemini-oauth",
				Platform: service.PlatformGemini,
				Type:     service.AccountTypeOAuth,
				Status:   service.StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh-token",
					"oauth_type":    "google_one",
					"project_id":    "project-1",
					"tier_id":       "google_ai_pro",
				},
			},
		},
	}
	geminiSvc := service.NewGeminiOAuthService(
		nil,
		refreshTierGeminiOAuthClientStub{},
		geminiOAuthHandlerMockCodeAssist{},
		&config.Config{},
	)
	defer geminiSvc.Stop()

	router := setupRefreshTierRouter(adminSvc, geminiSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/refresh-tier", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(42), data["id"])
	require.Equal(t, "new-access-token", adminSvc.accounts[42].Credentials["access_token"])
	require.Equal(t, "new-refresh-token", adminSvc.accounts[42].Credentials["refresh_token"])
}

func TestAccountHandlerRefreshTier_RejectsNonGoogleOneAccount(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	adminSvc := &refreshTierAccountAdminService{
		stubAdminService: newStubAdminService(),
		accounts: map[int64]service.Account{
			42: {
				ID:       42,
				Name:     "gemini-code-assist",
				Platform: service.PlatformGemini,
				Type:     service.AccountTypeOAuth,
				Status:   service.StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh-token",
					"oauth_type":    "code_assist",
					"project_id":    "project-1",
					"tier_id":       "STANDARD",
				},
			},
		},
	}
	geminiSvc := service.NewGeminiOAuthService(
		nil,
		refreshTierGeminiOAuthClientStub{},
		geminiOAuthHandlerMockCodeAssist{},
		&config.Config{},
	)
	defer geminiSvc.Stop()

	router := setupRefreshTierRouter(adminSvc, geminiSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/refresh-tier", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "Only Gemini Google One OAuth accounts support tier refresh")
	require.Empty(t, adminSvc.accounts[42].Credentials["access_token"])
}

func TestAccountHandlerBatchRefreshTier_UsesBackwardCompatibleRefreshPath(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	adminSvc := &refreshTierAccountAdminService{
		stubAdminService: newStubAdminService(),
		accounts: map[int64]service.Account{
			42: {
				ID:       42,
				Name:     "gemini-oauth-1",
				Platform: service.PlatformGemini,
				Type:     service.AccountTypeOAuth,
				Status:   service.StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh-token-1",
					"oauth_type":    "google_one",
					"project_id":    "project-1",
					"tier_id":       "google_ai_pro",
				},
			},
			43: {
				ID:       43,
				Name:     "gemini-oauth-2",
				Platform: service.PlatformGemini,
				Type:     service.AccountTypeOAuth,
				Status:   service.StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh-token-2",
					"oauth_type":    "google_one",
					"project_id":    "project-2",
					"tier_id":       "google_ai_pro",
				},
			},
		},
	}
	geminiSvc := service.NewGeminiOAuthService(
		nil,
		refreshTierGeminiOAuthClientStub{},
		geminiOAuthHandlerMockCodeAssist{},
		&config.Config{},
	)
	defer geminiSvc.Stop()

	router := setupRefreshTierRouter(adminSvc, geminiSvc)

	body, err := json.Marshal(map[string]any{"account_ids": []int64{42, 43}})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-refresh-tier", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(2), data["success"])
	require.Equal(t, float64(0), data["failed"])
	require.Equal(t, "new-access-token", adminSvc.accounts[42].Credentials["access_token"])
	require.Equal(t, "new-access-token", adminSvc.accounts[43].Credentials["access_token"])
}

func TestAccountHandlerBatchRefreshTier_FailsNonGoogleOneAccountsWithoutBlockingOthers(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	adminSvc := &refreshTierAccountAdminService{
		stubAdminService: newStubAdminService(),
		accounts: map[int64]service.Account{
			42: {
				ID:       42,
				Name:     "gemini-google-one",
				Platform: service.PlatformGemini,
				Type:     service.AccountTypeOAuth,
				Status:   service.StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh-token-1",
					"oauth_type":    "google_one",
					"project_id":    "project-1",
					"tier_id":       "google_ai_pro",
				},
			},
			43: {
				ID:       43,
				Name:     "gemini-code-assist",
				Platform: service.PlatformGemini,
				Type:     service.AccountTypeOAuth,
				Status:   service.StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh-token-2",
					"oauth_type":    "code_assist",
					"project_id":    "project-2",
					"tier_id":       "STANDARD",
				},
			},
		},
	}
	geminiSvc := service.NewGeminiOAuthService(
		nil,
		refreshTierGeminiOAuthClientStub{},
		geminiOAuthHandlerMockCodeAssist{},
		&config.Config{},
	)
	defer geminiSvc.Stop()

	router := setupRefreshTierRouter(adminSvc, geminiSvc)

	body, err := json.Marshal(map[string]any{"account_ids": []int64{42, 43}})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-refresh-tier", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(1), data["success"])
	require.Equal(t, float64(1), data["failed"])
	require.Equal(t, "new-access-token", adminSvc.accounts[42].Credentials["access_token"])
	require.Empty(t, adminSvc.accounts[43].Credentials["access_token"])

	errorsOut, ok := data["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errorsOut, 1)
	errItem, ok := errorsOut[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(43), errItem["account_id"])
	require.Contains(t, errItem["error"], "Only Gemini Google One OAuth accounts support tier refresh")
}
