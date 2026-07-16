//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type authRefreshTokenCacheStub struct {
	mu                    sync.Mutex
	tokens                map[string]*service.RefreshTokenData
	used                  map[string]string
	usedAt                map[string]time.Time
	familyTokens          map[string]map[string]struct{}
	blockFamilyAddStarted chan struct{}
	releaseFamilyAdd      chan struct{}
	deleteFamilyCalls     int
}

func newAuthRefreshTokenCacheStub() *authRefreshTokenCacheStub {
	return &authRefreshTokenCacheStub{
		tokens:       make(map[string]*service.RefreshTokenData),
		used:         make(map[string]string),
		usedAt:       make(map[string]time.Time),
		familyTokens: make(map[string]map[string]struct{}),
	}
}

func (s *authRefreshTokenCacheStub) StoreRefreshToken(_ context.Context, tokenHash string, data *service.RefreshTokenData, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := *data
	s.tokens[tokenHash] = &cloned
	return nil
}

func (s *authRefreshTokenCacheStub) GetRefreshToken(_ context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.tokens[tokenHash]
	if !ok {
		return nil, service.ErrRefreshTokenNotFound
	}
	cloned := *data
	return &cloned, nil
}

func (s *authRefreshTokenCacheStub) ConsumeRefreshToken(_ context.Context, tokenHash, familyID string, _ time.Duration) (*service.RefreshTokenData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.tokens[tokenHash]
	if !ok {
		return nil, service.ErrRefreshTokenNotFound
	}
	cloned := *data
	delete(s.tokens, tokenHash)
	s.used[tokenHash] = familyID
	s.usedAt[tokenHash] = time.Now().UTC()
	return &cloned, nil
}

func (s *authRefreshTokenCacheStub) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, tokenHash)
	return nil
}

func (s *authRefreshTokenCacheStub) GetUsedRefreshTokenFamily(_ context.Context, tokenHash string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	familyID, ok := s.used[tokenHash]
	if !ok {
		return "", service.ErrRefreshTokenNotFound
	}
	return familyID, nil
}

func (s *authRefreshTokenCacheStub) GetUsedRefreshTokenMarker(_ context.Context, tokenHash string) (*service.UsedRefreshTokenMarker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	familyID, ok := s.used[tokenHash]
	if !ok {
		return nil, service.ErrRefreshTokenNotFound
	}
	return &service.UsedRefreshTokenMarker{
		FamilyID:   familyID,
		ConsumedAt: s.usedAt[tokenHash],
	}, nil
}

func (s *authRefreshTokenCacheStub) IsRefreshTokenReuseGraceActive(_ context.Context, tokenHash string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	consumedAt, ok := s.usedAt[tokenHash]
	if !ok {
		return false, nil
	}
	return time.Since(consumedAt) < service.RefreshTokenConcurrentReuseGrace, nil
}

func (s *authRefreshTokenCacheStub) DeleteUserRefreshTokens(context.Context, int64) error {
	return nil
}

func (s *authRefreshTokenCacheStub) DeleteTokenFamily(_ context.Context, familyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteFamilyCalls++
	for tokenHash := range s.familyTokens[familyID] {
		delete(s.tokens, tokenHash)
	}
	delete(s.familyTokens, familyID)
	return nil
}

func (s *authRefreshTokenCacheStub) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}

func (s *authRefreshTokenCacheStub) AddToFamilyTokenSet(_ context.Context, familyID, tokenHash string, _ time.Duration) error {
	s.mu.Lock()
	if s.familyTokens[familyID] == nil {
		s.familyTokens[familyID] = make(map[string]struct{})
	}
	s.familyTokens[familyID][tokenHash] = struct{}{}
	started := s.blockFamilyAddStarted
	release := s.releaseFamilyAdd
	if started != nil {
		s.blockFamilyAddStarted = nil
		s.releaseFamilyAdd = nil
	}
	s.mu.Unlock()
	if started != nil {
		close(started)
		<-release
	}
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

func TestAuthHandlerRefreshToken_CookieModeRotatesHttpOnlyCookieWithoutJSONSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           32,
			Email:        "cookie-refresh@example.com",
			Username:     "cookie-refresh",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 1,
		},
	}
	cache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
		RefreshCookieSameSite:  "lax",
		RefreshCookiePath:      "/api/v1/auth",
	}}
	authService := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	initial, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)
	h := &AuthHandler{cfg: cfg, authService: authService}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set(refreshRequestHeader, "1")
	c.Request.Header.Set("Origin", "http://example.com")
	c.Request.Host = "example.com"
	c.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: initial.RefreshToken})

	h.RefreshToken(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.NotEmpty(t, resp.Data.AccessToken)
	require.Empty(t, resp.Data.RefreshToken)
	cookie := findCookie(recorder.Result().Cookies(), refreshTokenCookieName)
	require.NotNil(t, cookie)
	require.NotEqual(t, initial.RefreshToken, cookie.Value)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, "/api/v1/auth", cookie.Path)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestAuthHandlerRefreshToken_LegacyBodyOverridesStaleCookieDuringMigration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           33,
			Email:        "legacy-migration@example.com",
			Username:     "legacy-migration",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 1,
		},
	}
	cache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
	}}
	authService := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	initial, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)
	h := &AuthHandler{cfg: cfg, authService: authService}

	body := []byte(`{"refresh_token":"` + initial.RefreshToken + `"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set(refreshRequestHeader, "1")
	c.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: "sub2rt_stale-cookie-token"})

	h.RefreshToken(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	cookie := findCookie(recorder.Result().Cookies(), refreshTokenCookieName)
	require.NotNil(t, cookie)
	require.NotEqual(t, initial.RefreshToken, cookie.Value)
}

func TestAuthHandlerRefreshToken_CookieOnlyRejectsMissingCSRFHeader(t *testing.T) {
	h := &AuthHandler{}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: "sub2rt_cookie-token"})

	h.RefreshToken(c)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestAuthHandlerLogout_LegacyBodyOverridesStaleCookie(t *testing.T) {
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           36,
			Email:        "legacy-logout@example.com",
			Username:     "legacy-logout",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 1,
		},
	}
	cache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
	}}
	authService := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	initial, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)
	h := &AuthHandler{cfg: cfg, authService: authService}

	body := []byte(`{"refresh_token":"` + initial.RefreshToken + `"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: "sub2rt_stale-cookie-token"})

	h.Logout(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	_, err = authService.RefreshTokenPair(context.Background(), initial.RefreshToken)
	require.ErrorIs(t, err, service.ErrRefreshTokenInvalid)
	cleared := findCookie(recorder.Result().Cookies(), refreshTokenCookieName)
	require.NotNil(t, cleared)
	require.Equal(t, -1, cleared.MaxAge)
}

func TestAuthServiceRefreshTokenPairConcurrentRotationMintsOneSuccessor(t *testing.T) {
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           34,
			Email:        "concurrent-refresh@example.com",
			Username:     "concurrent-refresh",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 1,
		},
	}
	cache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
	}}
	authService := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	initial, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)

	const attempts = 16
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, refreshErr := authService.RefreshTokenPair(context.Background(), initial.RefreshToken)
			results <- refreshErr
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for refreshErr := range results {
		if refreshErr == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes, "only one concurrent refresh may mint a successor")
}

func TestAuthServiceRefreshTokenPairConcurrentLoserPreservesWinnerSuccessor(t *testing.T) {
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           35,
			Email:        "concurrent-successor@example.com",
			Username:     "concurrent-successor",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 1,
		},
	}
	cache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
	}}
	authService := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	initial, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)

	familyAddStarted := make(chan struct{})
	releaseFamilyAdd := make(chan struct{})
	cache.mu.Lock()
	cache.blockFamilyAddStarted = familyAddStarted
	cache.releaseFamilyAdd = releaseFamilyAdd
	cache.mu.Unlock()

	winnerResult := make(chan *service.TokenPairWithUser, 1)
	winnerErr := make(chan error, 1)
	go func() {
		result, refreshErr := authService.RefreshTokenPair(context.Background(), initial.RefreshToken)
		winnerResult <- result
		winnerErr <- refreshErr
	}()

	<-familyAddStarted
	_, loserErr := authService.RefreshTokenPair(context.Background(), initial.RefreshToken)
	require.ErrorIs(t, loserErr, service.ErrRefreshTokenConcurrent)
	cache.mu.Lock()
	require.Equal(t, 0, cache.deleteFamilyCalls, "ambiguous concurrent loser must not revoke the winner's family")
	cache.mu.Unlock()
	close(releaseFamilyAdd)

	winner := <-winnerResult
	require.NoError(t, <-winnerErr)
	require.NotNil(t, winner)
	_, err = authService.RefreshTokenPair(context.Background(), winner.RefreshToken)
	require.NoError(t, err, "the winner's successor must remain usable")
}

func TestAuthHandlerRefreshTokenConcurrentLoserDoesNotClearWinnerCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           37,
			Email:        "concurrent-cookie@example.com",
			Username:     "concurrent-cookie",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 1,
		},
	}
	cache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
		RefreshCookieSameSite:  "lax",
		RefreshCookiePath:      "/api/v1/auth",
	}}
	authService := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	initial, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)
	h := &AuthHandler{cfg: cfg, authService: authService}

	familyAddStarted := make(chan struct{})
	releaseFamilyAdd := make(chan struct{})
	cache.mu.Lock()
	cache.blockFamilyAddStarted = familyAddStarted
	cache.releaseFamilyAdd = releaseFamilyAdd
	cache.mu.Unlock()

	winnerRecorder := httptest.NewRecorder()
	winnerContext, _ := gin.CreateTestContext(winnerRecorder)
	winnerContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte(`{}`)))
	winnerContext.Request.Header.Set("Content-Type", "application/json")
	winnerContext.Request.Header.Set(refreshRequestHeader, "1")
	winnerContext.Request.Header.Set("Origin", "http://example.com")
	winnerContext.Request.Host = "example.com"
	winnerContext.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: initial.RefreshToken})
	winnerDone := make(chan struct{})
	go func() {
		defer close(winnerDone)
		h.RefreshToken(winnerContext)
	}()

	<-familyAddStarted
	loserRecorder := httptest.NewRecorder()
	loserContext, _ := gin.CreateTestContext(loserRecorder)
	loserContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte(`{}`)))
	loserContext.Request.Header.Set("Content-Type", "application/json")
	loserContext.Request.Header.Set(refreshRequestHeader, "1")
	loserContext.Request.Header.Set("Origin", "http://example.com")
	loserContext.Request.Host = "example.com"
	loserContext.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: initial.RefreshToken})
	h.RefreshToken(loserContext)

	close(releaseFamilyAdd)
	<-winnerDone

	require.Equal(t, http.StatusUnauthorized, loserRecorder.Code)
	for _, cookie := range loserRecorder.Result().Cookies() {
		require.NotEqual(t, refreshTokenCookieName, cookie.Name, "concurrent loser must not delete the shared winner cookie")
	}
	require.Equal(t, http.StatusOK, winnerRecorder.Code)
	winnerCookie := findCookie(winnerRecorder.Result().Cookies(), refreshTokenCookieName)
	require.NotNil(t, winnerCookie)
	require.NotEmpty(t, winnerCookie.Value)

	nextRecorder := httptest.NewRecorder()
	nextContext, _ := gin.CreateTestContext(nextRecorder)
	nextContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte(`{}`)))
	nextContext.Request.Header.Set("Content-Type", "application/json")
	nextContext.Request.Header.Set(refreshRequestHeader, "1")
	nextContext.Request.Header.Set("Origin", "http://example.com")
	nextContext.Request.Host = "example.com"
	nextContext.Request.AddCookie(winnerCookie)
	h.RefreshToken(nextContext)
	require.Equal(t, http.StatusOK, nextRecorder.Code, "winner cookie must remain usable after the loser response")
}

func TestAuthServiceRefreshTokenPairReplayAfterGraceRevokesSuccessor(t *testing.T) {
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           36,
			Email:        "refresh-replay@example.com",
			Username:     "refresh-replay",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 1,
		},
	}
	cache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
	}}
	authService := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	initial, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)
	successor, err := authService.RefreshTokenPair(context.Background(), initial.RefreshToken)
	require.NoError(t, err)

	cache.mu.Lock()
	for tokenHash := range cache.used {
		cache.usedAt[tokenHash] = time.Now().Add(-time.Minute)
	}
	cache.mu.Unlock()

	_, replayErr := authService.RefreshTokenPair(context.Background(), initial.RefreshToken)
	require.ErrorIs(t, replayErr, service.ErrRefreshTokenReused)
	cache.mu.Lock()
	require.Equal(t, 1, cache.deleteFamilyCalls)
	cache.mu.Unlock()

	_, successorErr := authService.RefreshTokenPair(context.Background(), successor.RefreshToken)
	require.ErrorIs(t, successorErr, service.ErrRefreshTokenInvalid)
}

func TestAuthHandlerRefreshTokenReplayAfterGraceClearsCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           38,
			Email:        "replay-cookie@example.com",
			Username:     "replay-cookie",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			TokenVersion: 1,
		},
	}
	cache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
		RefreshCookieSameSite:  "lax",
		RefreshCookiePath:      "/api/v1/auth",
	}}
	authService := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	initial, err := authService.GenerateTokenPair(context.Background(), repo.user, "")
	require.NoError(t, err)
	_, err = authService.RefreshTokenPair(context.Background(), initial.RefreshToken)
	require.NoError(t, err)
	cache.mu.Lock()
	for tokenHash := range cache.used {
		cache.usedAt[tokenHash] = time.Now().Add(-time.Minute)
	}
	cache.mu.Unlock()

	h := &AuthHandler{cfg: cfg, authService: authService}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set(refreshRequestHeader, "1")
	c.Request.Header.Set("Origin", "http://example.com")
	c.Request.Host = "example.com"
	c.Request.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: initial.RefreshToken})
	h.RefreshToken(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	cleared := findCookie(recorder.Result().Cookies(), refreshTokenCookieName)
	require.NotNil(t, cleared)
	require.Equal(t, -1, cleared.MaxAge)
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

func TestAuthHandlerLoginHydratesAvatarFromSeparateRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &service.User{
		ID:           34,
		Email:        "avatar-login@example.com",
		Username:     "avatar-login",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		TokenVersion: 1,
	}
	require.NoError(t, user.SetPassword("correct-password"))
	repo := &userHandlerRepoStub{
		user: user,
		avatar: &service.UserAvatar{
			StorageProvider: "inline",
			URL:             "data:image/png;base64,YXZhdGFy",
			ContentType:     "image/png",
		},
	}
	refreshTokenCache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
	}}
	authService := service.NewAuthService(nil, repo, nil, refreshTokenCache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &AuthHandler{
		authService: authService,
		userService: service.NewUserService(repo, nil, nil, nil),
	}

	body := []byte(`{"email":"avatar-login@example.com","password":"correct-password","turnstile_token":""}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			User struct {
				AvatarURL string `json:"avatar_url"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, repo.avatar.URL, resp.Data.User.AvatarURL)
	require.Equal(t, 1, repo.getAvatar)
}

func TestAuthHandlerLoginSkipsHydrationWhenAvatarAlreadyPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &service.User{
		ID:           35,
		Email:        "avatar-present-login@example.com",
		Username:     "avatar-present-login",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		TokenVersion: 1,
		AvatarURL:    "https://cdn.example.com/avatar.png",
	}
	require.NoError(t, user.SetPassword("correct-password"))
	repo := &userHandlerRepoStub{user: user}
	refreshTokenCache := newAuthRefreshTokenCacheStub()
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                 "test-secret",
		ExpireHour:             1,
		RefreshTokenExpireDays: 7,
	}}
	authService := service.NewAuthService(nil, repo, nil, refreshTokenCache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &AuthHandler{
		authService: authService,
		userService: service.NewUserService(repo, nil, nil, nil),
	}

	body := []byte(`{"email":"avatar-present-login@example.com","password":"correct-password","turnstile_token":""}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			User struct {
				AvatarURL string `json:"avatar_url"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "https://cdn.example.com/avatar.png", resp.Data.User.AvatarURL)
	require.Zero(t, repo.getAvatar)
}
