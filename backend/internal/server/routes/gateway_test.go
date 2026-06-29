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

func newGatewayRoutesTestRouter(platform ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	groupPlatform := service.PlatformOpenAI
	if len(platform) > 0 && platform[0] != "" {
		groupPlatform = platform[0]
	}

	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: &handler.OpenAIGatewayHandler{},
		},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: groupPlatform},
			})
			c.Next()
		}),
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
	)

	return router
}

func TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/responses/compact",
		"/responses/compact",
		"/backend-api/codex/responses",
		"/backend-api/codex/responses/compact",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should hit OpenAI responses handler", path)
	}
}

func TestGatewayRoutesOpenAIImagesPathsAreRegistered(t *testing.T) {
	for _, plat := range []string{"", service.PlatformOpenAI} {
		router := newGatewayRoutesTestRouter(plat)
		for _, path := range []string{
			"/v1/images/generations",
			"/v1/images/edits",
			"/images/generations",
			"/images/edits",
		} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)
			require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should be available for platform %s", path, plat)
		}
	}
}

func TestGatewayRoutesGrokImagesPathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformGrok)

	for _, path := range []string{
		"/v1/images/generations",
		"/v1/images/edits",
		"/images/generations",
		"/images/edits",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"grok-imagine-image","prompt":"draw a cat"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should expose images for Grok groups (uses subscription OAuth)", path)
	}
}

func TestGatewayRoutesOpenAIVideosPathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformOpenAI)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/videos"},
		{http.MethodPost, "/v1/videos/generations"},
		{http.MethodPost, "/v1/videos/edits"},
		{http.MethodPost, "/v1/videos/extensions"},
		{http.MethodPost, "/videos"},
		{http.MethodPost, "/videos/generations"},
		{http.MethodPost, "/videos/edits"},
		{http.MethodPost, "/videos/extensions"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"sora-2","prompt":"a cat playing piano","seconds":"8","size":"1280x720"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "method=%s path=%s should hit OpenAI-compatible videos handler", tc.method, tc.path)
	}
}

func TestGatewayRoutesGrokVideosPathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformGrok)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/videos"},
		{http.MethodPost, "/v1/videos/generations"},
		{http.MethodPost, "/v1/videos/edits"},
		{http.MethodPost, "/v1/videos/extensions"},
		{http.MethodPost, "/videos"},
		{http.MethodPost, "/videos/generations"},
		{http.MethodPost, "/videos/edits"},
		{http.MethodPost, "/videos/extensions"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"grok","prompt":"a cat playing piano","seconds":"8","size":"1280x720"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "method=%s path=%s should hit videos handler for Grok groups", tc.method, tc.path)
	}
}

func TestGatewayRoutesOpenAIImages2APIPathsAreRemoved(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/images2api/generations",
		"/v1/images2api/edits",
		"/images2api/generations",
		"/images2api/edits",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, "path=%s should not be registered", path)
	}
}

func TestGatewayRoutesGrokOnlyAllowsResponsesHTTP(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformGrok)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/messages"},
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodPost, "/chat/completions"},
		{http.MethodGet, "/v1/responses"},
		{http.MethodGet, "/responses"},
		{http.MethodGet, "/backend-api/codex/responses"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"grok"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, "method=%s path=%s", tc.method, tc.path)
		require.Contains(t, w.Body.String(), "not supported for Grok groups")
	}

	for _, path := range []string{
		"/v1/responses",
		"/responses",
		"/backend-api/codex/responses",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"grok","input":"hi"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should still reach Responses handler", path)
	}
}

func TestGatewayRoutesVideoAndWebSearchRejectUnsupportedPlatforms(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformAnthropic)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/videos"},
		{http.MethodPost, "/v1/videos/generations"},
		{http.MethodGet, "/v1/videos"},
		{http.MethodPost, "/videos"},
		{http.MethodPost, "/videos/generations"},
		{http.MethodGet, "/videos"},
		{http.MethodPost, "/v1/web_search"},
		{http.MethodPost, "/web_search"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"query":"news"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, "method=%s path=%s should be platform-gated", tc.method, tc.path)
	}
}
