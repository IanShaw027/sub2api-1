//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClearRefreshTokenCookiePreservesSecureBehindMultipleProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "http://api.example.com/api/v1/auth/logout", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https, http")

	(&AuthHandler{}).clearRefreshTokenCookie(c)

	cookie := findCookie(recorder.Result().Cookies(), refreshTokenCookieName)
	require.NotNil(t, cookie)
	require.True(t, cookie.Secure)
	require.Equal(t, -1, cookie.MaxAge)
}

func TestValidateRefreshCookieRequestAllowsValidOriginWhenCORSIsUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	c.Request.Host = ""
	c.Request.Header.Set(refreshRequestHeader, "1")
	c.Request.Header.Set("Origin", "https://frontend.example.com")

	h := &AuthHandler{cfg: &config.Config{}}
	require.True(t, h.validateRefreshCookieRequest(c))
}

func TestValidateRefreshCookieRequestRejectsOriginOutsideConfiguredCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "http://api.example.com/api/v1/auth/refresh", nil)
	c.Request.Header.Set(refreshRequestHeader, "1")
	c.Request.Header.Set("Origin", "https://unexpected.example.com")

	h := &AuthHandler{cfg: &config.Config{
		CORS: config.CORSConfig{AllowedOrigins: []string{"https://frontend.example.com"}},
	}}
	require.False(t, h.validateRefreshCookieRequest(c))
}
