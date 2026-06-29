package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCommonRoutesClaudeTelemetryDropModeDoesNotRequireAPIKeyAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authCalls := 0
	RegisterCommonRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}}, servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.AbortWithStatus(http.StatusUnauthorized)
	}), nil, &config.Config{Gateway: config.GatewayConfig{ClaudeTelemetryMode: config.ClaudeTelemetryModeDrop}})

	req := httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", strings.NewReader(`{"events":[]}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 0, authCalls)
	require.JSONEq(t, `{}`, w.Body.String())
}

func TestCommonRoutesClaudeTelemetryForwardModeUsesAPIKeyAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authCalls := 0
	groupID := int64(10)
	RegisterCommonRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}}, servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformAnthropic}})
		c.Next()
	}), nil, &config.Config{Gateway: config.GatewayConfig{ClaudeTelemetryMode: config.ClaudeTelemetryModeForward}})

	req := httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", strings.NewReader(`{"events":[]}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, authCalls)
	require.JSONEq(t, `{}`, w.Body.String())
}

func TestCommonRoutesClaudeAuxPolicyLimitsStub(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/claude_code/policy_limits", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"restrictions":{}}`, w.Body.String())
}
