//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIOAuthHandlerCodexPATStub struct{}

func (s *openAIOAuthHandlerCodexPATStub) GenerateAuthURL(context.Context, *int64, string, string) (*service.OpenAIAuthURLResult, error) {
	return nil, nil
}

func (s *openAIOAuthHandlerCodexPATStub) ExchangeCode(context.Context, *service.OpenAIExchangeCodeInput) (*service.OpenAITokenInfo, error) {
	return nil, nil
}

func (s *openAIOAuthHandlerCodexPATStub) RefreshTokenWithClientID(context.Context, string, string, string) (*service.OpenAITokenInfo, error) {
	return nil, nil
}

func (s *openAIOAuthHandlerCodexPATStub) RefreshAccountToken(context.Context, *service.Account) (*service.OpenAITokenInfo, error) {
	return nil, nil
}

func (s *openAIOAuthHandlerCodexPATStub) BuildAccountCredentials(tokenInfo *service.OpenAITokenInfo) map[string]any {
	return map[string]any{
		"access_token":       tokenInfo.AccessToken,
		"chatgpt_account_id": tokenInfo.ChatGPTAccountID,
		"chatgpt_user_id":    tokenInfo.ChatGPTUserID,
		"email":              tokenInfo.Email,
	}
}

func (s *openAIOAuthHandlerCodexPATStub) ValidateCodexPersonalAccessToken(context.Context, string, string) (*service.OpenAITokenInfo, error) {
	return &service.OpenAITokenInfo{
		AccessToken:      "at-test-token",
		Email:            "user@example.com",
		ChatGPTAccountID: "acct-1",
		ChatGPTUserID:    "user-1",
		PlanType:         "plus",
	}, nil
}

func TestCreateAccountFromCodexPATOmittedConcurrencyPassesZeroToCreateAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := &OpenAIOAuthHandler{
		openaiOAuthService: &openAIOAuthHandlerCodexPATStub{},
		adminService:       adminSvc,
	}
	router := gin.New()
	router.POST("/openai/create-from-codex-pat", handler.CreateAccountFromCodexPAT)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/openai/create-from-codex-pat", strings.NewReader(
		`{"access_token":"at-test-token","skip_default_group_bind":true}`,
	))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Equal(t, 0, adminSvc.createdAccounts[0].Concurrency)
}

func TestCreateAccountFromCodexPATNegativeConcurrencyDoesNotReturnBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := &OpenAIOAuthHandler{
		openaiOAuthService: &openAIOAuthHandlerCodexPATStub{},
		adminService:       adminSvc,
	}
	router := gin.New()
	router.POST("/openai/create-from-codex-pat", handler.CreateAccountFromCodexPAT)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/openai/create-from-codex-pat", strings.NewReader(
		`{"access_token":"at-test-token","concurrency":-1,"skip_default_group_bind":true}`,
	))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Equal(t, -1, adminSvc.createdAccounts[0].Concurrency)
}
