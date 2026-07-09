//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type authRefreshTokenCacheStub struct {
	tokens map[string]*service.RefreshTokenData
}

func newAuthRefreshTokenCacheStub() *authRefreshTokenCacheStub {
	return &authRefreshTokenCacheStub{
		tokens: make(map[string]*service.RefreshTokenData),
	}
}

func (s *authRefreshTokenCacheStub) StoreRefreshToken(_ context.Context, tokenHash string, data *service.RefreshTokenData, _ time.Duration) error {
	cloned := *data
	s.tokens[tokenHash] = &cloned
	return nil
}

func (s *authRefreshTokenCacheStub) GetRefreshToken(_ context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	data, ok := s.tokens[tokenHash]
	if !ok {
		return nil, service.ErrRefreshTokenNotFound
	}
	cloned := *data
	return &cloned, nil
}

func (s *authRefreshTokenCacheStub) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	delete(s.tokens, tokenHash)
	return nil
}

func (s *authRefreshTokenCacheStub) DeleteUserRefreshTokens(context.Context, int64) error {
	return nil
}

func (s *authRefreshTokenCacheStub) DeleteTokenFamily(context.Context, string) error {
	return nil
}

func (s *authRefreshTokenCacheStub) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}

func (s *authRefreshTokenCacheStub) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}

func (s *authRefreshTokenCacheStub) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}

func (s *authRefreshTokenCacheStub) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *authRefreshTokenCacheStub) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestAuthHandlerRefreshToken_NilSettingServiceStillSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           31,
			Email:        "refresh@example.com",
			Username:     "refresh-user",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 3,
		},
	}
	refreshTokenCache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-secret",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
	}
	authService := service.NewAuthService(nil, repo, nil, refreshTokenCache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	tokenPair, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)

	h := &AuthHandler{authService: authService}

	body := []byte(`{"refresh_token":"` + tokenPair.RefreshToken + `"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RefreshToken(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			TokenType    string `json:"token_type"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.NotEmpty(t, resp.Data.AccessToken)
	require.NotEmpty(t, resp.Data.RefreshToken)
	require.Equal(t, "Bearer", resp.Data.TokenType)
	require.Greater(t, resp.Data.ExpiresIn, 0)
}

func TestAuthHandlerForgotPassword_NilSettingServiceReturnsInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       32,
			Email:    "forgot@example.com",
			Username: "forgot-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
	}
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			ExpireHour: 1,
		},
	}
	authService := service.NewAuthService(nil, repo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &AuthHandler{authService: authService}

	body := []byte(`{"email":"forgot@example.com","turnstile_token":""}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ForgotPassword(c)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Equal(t, "Password reset is not configured", resp.Message)
}

func TestAuthHandlerLogin_NilSettingServiceDoesNotTriggerTotpBranch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &service.User{
		ID:           33,
		Email:        "login@example.com",
		Username:     "login-user",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		TokenVersion: 2,
		TotpEnabled:  true,
	}
	require.NoError(t, user.SetPassword("correct-password"))

	repo := &userHandlerRepoStub{user: user}
	refreshTokenCache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-secret",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
	}
	authService := service.NewAuthService(nil, repo, nil, refreshTokenCache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &AuthHandler{
		authService: authService,
		totpService: &service.TotpService{},
	}

	body := []byte(`{"email":"login@example.com","password":"correct-password","turnstile_token":""}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			TokenType    string `json:"token_type"`
			Requires2FA  bool   `json:"requires_2fa"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.NotEmpty(t, resp.Data.AccessToken)
	require.NotEmpty(t, resp.Data.RefreshToken)
	require.Equal(t, "Bearer", resp.Data.TokenType)
	require.False(t, resp.Data.Requires2FA)
}
