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

type geminiOAuthHandlerMockClient struct{}

func (geminiOAuthHandlerMockClient) ExchangeCode(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error) {
	return &geminicli.TokenResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}, nil
}

func (geminiOAuthHandlerMockClient) RefreshToken(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error) {
	return nil, nil
}

type geminiOAuthHandlerMockCodeAssist struct{}

func (geminiOAuthHandlerMockCodeAssist) LoadCodeAssist(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
	return &geminicli.LoadCodeAssistResponse{
		CloudAICompanionProject: req.CloudAICompanionProject,
	}, nil
}

func (geminiOAuthHandlerMockCodeAssist) OnboardUser(ctx context.Context, accessToken, proxyURL string, req *geminicli.OnboardUserRequest) (*geminicli.OnboardUserResponse, error) {
	return nil, nil
}

func (geminiOAuthHandlerMockCodeAssist) GetOperation(ctx context.Context, accessToken, proxyURL, name string) (*geminicli.OnboardUserResponse, error) {
	return nil, nil
}

func (geminiOAuthHandlerMockCodeAssist) RetrieveUserQuota(ctx context.Context, accessToken, proxyURL string, req *geminicli.RetrieveUserQuotaRequest) (*geminicli.RetrieveUserQuotaResponse, error) {
	return &geminicli.RetrieveUserQuotaResponse{}, nil
}

func TestGeminiOAuthHandler_ExchangeCode_PrefersDetectedTierOverSessionFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	svc := service.NewGeminiOAuthService(
		nil,
		geminiOAuthHandlerMockClient{},
		geminiOAuthHandlerMockCodeAssist{},
		nil,
		&config.Config{},
	)
	defer svc.Stop()

	handler := NewGeminiOAuthHandler(svc)
	router := gin.New()
	router.POST("/api/v1/admin/gemini/oauth/auth-url", handler.GenerateAuthURL)
	router.POST("/api/v1/admin/gemini/oauth/exchange-code", handler.ExchangeCode)

	authReqBody := []byte(`{"project_id":"project-1","oauth_type":"code_assist","tier_id":"gcp_enterprise"}`)
	authReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/gemini/oauth/auth-url", bytes.NewReader(authReqBody))
	authReq.Header.Set("Content-Type", "application/json")
	authRec := httptest.NewRecorder()
	router.ServeHTTP(authRec, authReq)
	require.Equal(t, http.StatusOK, authRec.Code, authRec.Body.String())

	var authResp response.Response
	require.NoError(t, json.Unmarshal(authRec.Body.Bytes(), &authResp))
	authData, ok := authResp.Data.(map[string]any)
	require.True(t, ok)

	exchangePayload := map[string]any{
		"session_id": authData["session_id"],
		"state":      authData["state"],
		"code":       "code-1",
		"oauth_type": "code_assist",
	}
	exchangeBody, err := json.Marshal(exchangePayload)
	require.NoError(t, err)

	exchangeReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/gemini/oauth/exchange-code", bytes.NewReader(exchangeBody))
	exchangeReq.Header.Set("Content-Type", "application/json")
	exchangeRec := httptest.NewRecorder()
	router.ServeHTTP(exchangeRec, exchangeReq)
	require.Equal(t, http.StatusOK, exchangeRec.Code, exchangeRec.Body.String())

	var exchangeResp response.Response
	require.NoError(t, json.Unmarshal(exchangeRec.Body.Bytes(), &exchangeResp))
	exchangeData, ok := exchangeResp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "LEGACY", exchangeData["tier_id"])
	require.Equal(t, "project-1", exchangeData["project_id"])
}

func TestGeminiOAuthHandler_GenerateAuthURL_AcceptsProjectIDHint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	svc := service.NewGeminiOAuthService(
		nil,
		geminiOAuthHandlerMockClient{},
		geminiOAuthHandlerMockCodeAssist{},
		nil,
		&config.Config{},
	)
	defer svc.Stop()

	handler := NewGeminiOAuthHandler(svc)
	router := gin.New()
	router.POST("/api/v1/admin/gemini/oauth/auth-url", handler.GenerateAuthURL)

	authReqBody := []byte(`{"project_id_hint":"project-hint-1","oauth_type":"code_assist"}`)
	authReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/gemini/oauth/auth-url", bytes.NewReader(authReqBody))
	authReq.Header.Set("Content-Type", "application/json")
	authRec := httptest.NewRecorder()
	router.ServeHTTP(authRec, authReq)
	require.Equal(t, http.StatusOK, authRec.Code, authRec.Body.String())

	var authResp response.Response
	require.NoError(t, json.Unmarshal(authRec.Body.Bytes(), &authResp))
	authData, ok := authResp.Data.(map[string]any)
	require.True(t, ok)
	authURL, ok := authData["auth_url"].(string)
	require.True(t, ok)
	require.Contains(t, authURL, "project_id=project-hint-1")
}
