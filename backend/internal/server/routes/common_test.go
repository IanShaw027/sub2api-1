package routes

import (
	"context"
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

type commonRoutesSettingRepoStub struct {
	values map[string]string
}

func (r *commonRoutesSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (r *commonRoutesSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if r.values == nil {
		return "", service.ErrSettingNotFound
	}
	v, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return v, nil
}
func (r *commonRoutesSettingRepoStub) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}
func (r *commonRoutesSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if r.values != nil {
			out[key] = r.values[key]
		}
	}
	return out, nil
}
func (r *commonRoutesSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	for k, v := range settings {
		r.values[k] = v
	}
	return nil
}
func (r *commonRoutesSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}
func (r *commonRoutesSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestCommonRoutesClaudeTelemetryDropModeDoesNotRequireAPIKeyAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authCalls := 0
	RegisterCommonRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}}, servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.AbortWithStatus(http.StatusUnauthorized)
	}), nil, nil)

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
	settingService := service.NewSettingService(&commonRoutesSettingRepoStub{values: map[string]string{service.SettingKeyClaudeTelemetryMode: service.ClaudeTelemetryModeForward}}, &config.Config{})
	RegisterCommonRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}}, servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformAnthropic}})
		c.Next()
	}), settingService, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", strings.NewReader(`{"events":[]}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, authCalls)
	require.JSONEq(t, `{}`, w.Body.String())
}

func TestCommonRoutesClaudeTelemetryForwardModeMissingAPIKeyStillReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authCalls := 0
	settingService := service.NewSettingService(&commonRoutesSettingRepoStub{values: map[string]string{service.SettingKeyClaudeTelemetryMode: service.ClaudeTelemetryModeForward}}, &config.Config{})
	RegisterCommonRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}}, servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.AbortWithStatus(http.StatusUnauthorized)
	}), settingService, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", strings.NewReader(`{"events":[]}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, authCalls)
	require.JSONEq(t, `{}`, w.Body.String())
}

func TestCommonRoutesClaudeTelemetryForwardModeBillingAbortStillReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authCalls := 0
	settingService := service.NewSettingService(&commonRoutesSettingRepoStub{values: map[string]string{service.SettingKeyClaudeTelemetryMode: service.ClaudeTelemetryModeForward}}, &config.Config{})
	RegisterCommonRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}}, servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "quota exhausted"})
	}), settingService, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", strings.NewReader(`{"events":[]}`))
	req.Header.Set("Authorization", "Bearer exhausted")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, authCalls)
	require.JSONEq(t, `{}`, w.Body.String())
}

func TestCommonRoutesClaudeTelemetryForwardModeMarksAPIKeyAuthBillingSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authSawSkipBilling := false
	groupID := int64(10)
	settingService := service.NewSettingService(&commonRoutesSettingRepoStub{values: map[string]string{service.SettingKeyClaudeTelemetryMode: service.ClaudeTelemetryModeForward}}, &config.Config{})
	RegisterCommonRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}}, servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		authSawSkipBilling = c.GetBool("skip_api_key_billing")
		if !authSawSkipBilling {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformAnthropic}})
		c.Next()
	}), settingService, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", strings.NewReader(`{"events":[]}`))
	req.Header.Set("Authorization", "Bearer exhausted")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, authSawSkipBilling)
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
