package routes

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newAuthRoutesTestRouter(redisClient *redis.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterAuthRoutes(
		v1,
		&handler.Handlers{
			Auth:    &handler.AuthHandler{},
			Setting: &handler.SettingHandler{},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		redisClient,
		nil,
		nil,
	)

	return router
}

func TestAuthRoutesRateLimitFailCloseWhenRedisUnavailable(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	router := newAuthRoutesTestRouter(rdb)
	paths := []string{
		"/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/auth/login/2fa",
		"/api/v1/auth/send-verify-code",
		"/api/v1/auth/oauth/pending/send-verify-code",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.10:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusTooManyRequests, w.Code, "path=%s", path)
		require.Contains(t, w.Body.String(), "rate limit exceeded", "path=%s", path)
		require.Equal(t, "60", w.Header().Get("Retry-After"), "path=%s", path)
	}
}

func TestOAuthStartMethodsShareProviderRateLimitBucket(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	router := newAuthRoutesTestRouter(rdb)

	providers := []struct {
		name     string
		requests []struct {
			method string
			path   string
		}
	}{
		{name: "linuxdo", requests: []struct{ method, path string }{
			{http.MethodGet, "/api/v1/auth/oauth/linuxdo/start"},
			{http.MethodPost, "/api/v1/auth/oauth/linuxdo/start"},
			{http.MethodGet, "/api/v1/auth/oauth/linuxdo/bind/start"},
		}},
		{name: "github", requests: []struct{ method, path string }{
			{http.MethodGet, "/api/v1/auth/oauth/github/start"},
			{http.MethodPost, "/api/v1/auth/oauth/github/start"},
		}},
		{name: "google", requests: []struct{ method, path string }{
			{http.MethodGet, "/api/v1/auth/oauth/google/start"},
			{http.MethodPost, "/api/v1/auth/oauth/google/start"},
		}},
		{name: "wechat", requests: []struct{ method, path string }{
			{http.MethodGet, "/api/v1/auth/oauth/wechat/start"},
			{http.MethodPost, "/api/v1/auth/oauth/wechat/start"},
			{http.MethodGet, "/api/v1/auth/oauth/wechat/bind/start"},
		}},
		{name: "oidc", requests: []struct{ method, path string }{
			{http.MethodGet, "/api/v1/auth/oauth/oidc/start"},
			{http.MethodPost, "/api/v1/auth/oauth/oidc/start"},
			{http.MethodGet, "/api/v1/auth/oauth/oidc/bind/start"},
		}},
		{name: "dingtalk", requests: []struct{ method, path string }{
			{http.MethodGet, "/api/v1/auth/oauth/dingtalk/start"},
			{http.MethodPost, "/api/v1/auth/oauth/dingtalk/start"},
			{http.MethodGet, "/api/v1/auth/oauth/dingtalk/bind/start"},
		}},
	}
	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			redisKey := "rate_limit:oauth-" + provider.name + "-start:203.0.113.10"
			mr.Set(redisKey, "20")
			for _, request := range provider.requests {
				req := httptest.NewRequest(request.method, request.path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.RemoteAddr = "203.0.113.10:12345"
				w := httptest.NewRecorder()

				router.ServeHTTP(w, req)

				require.Equal(t, http.StatusTooManyRequests, w.Code, "%s %s", request.method, request.path)
			}
			count, err := mr.Get(redisKey)
			require.NoError(t, err)
			require.Equal(t, strconv.Itoa(20+len(provider.requests)), count)
		})
	}
}
