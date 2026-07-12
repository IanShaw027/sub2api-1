package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
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
			BatchImage:    &handler.BatchImageHandler{},
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

func assertGatewayRouteRegistered(t *testing.T, router *gin.Engine, method, path string) {
	t.Helper()
	for _, route := range router.Routes() {
		if route.Method != method {
			continue
		}
		if route.Path == path {
			return
		}
		if strings.HasSuffix(route.Path, "/*subpath") {
			prefix := strings.TrimSuffix(route.Path, "/*subpath")
			if strings.HasPrefix(path, prefix+"/") {
				return
			}
		}
	}
	require.Failf(t, "route not registered", "method=%s path=%s", method, path)
}

func TestGatewayRoutesBatchImagePathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()
	registered := make(map[string]struct{}, len(router.Routes()))
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}

	for _, route := range []string{
		http.MethodPost + " /v1/images/batches",
		http.MethodGet + " /v1/images/batches",
		http.MethodGet + " /v1/images/batches/models",
		http.MethodGet + " /v1/images/batches/:id",
		http.MethodGet + " /v1/images/batches/:id/items",
		http.MethodGet + " /v1/images/batches/:id/items/:custom_id/content",
		http.MethodGet + " /v1/images/batches/:id/download",
		http.MethodPost + " /v1/images/batches/:id/cancel",
		http.MethodDelete + " /v1/images/batches/:id",
		http.MethodDelete + " /v1/images/batches/:id/outputs",
	} {
		_, ok := registered[route]
		require.True(t, ok, "route %s should be registered", route)
	}
}

func TestGatewayImageRouteKindForPlatform(t *testing.T) {
	require.Equal(t, imageGatewayRouteOpenAI, imageGatewayRouteKindForPlatform(service.PlatformOpenAI))
	require.Equal(t, imageGatewayRouteGrok, imageGatewayRouteKindForPlatform(service.PlatformGrok))
	require.Equal(t, imageGatewayRouteUnsupported, imageGatewayRouteKindForPlatform(service.PlatformAnthropic))
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
		assertGatewayRouteRegistered(t, router, http.MethodPost, path)
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
			assertGatewayRouteRegistered(t, router, http.MethodPost, path)
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
		assertGatewayRouteRegistered(t, router, http.MethodPost, path)
		require.NotContains(t, w.Body.String(), "Images API is not supported for this platform")
	}
}

func TestGatewayRoutesOpenAIVideosPathsRejectOpenAIGroups(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformOpenAI)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/videos"},
		{http.MethodPost, "/v1/videos/generations"},
		{http.MethodPost, "/v1/videos/edits"},
		{http.MethodPost, "/v1/videos/extensions"},
		{http.MethodGet, "/v1/videos"},
		{http.MethodGet, "/v1/videos/video-job"},
		{http.MethodPost, "/videos"},
		{http.MethodPost, "/videos/generations"},
		{http.MethodPost, "/videos/edits"},
		{http.MethodPost, "/videos/extensions"},
		{http.MethodGet, "/videos"},
		{http.MethodGet, "/videos/video-job"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"grok-video","prompt":"a cat playing piano","seconds":"8","size":"1280x720"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, "method=%s path=%s should reject videos for OpenAI groups", tc.method, tc.path)
		require.Contains(t, w.Body.String(), "Videos API is not supported for this platform")
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
		assertGatewayRouteRegistered(t, router, tc.method, tc.path)
		require.NotContains(t, w.Body.String(), "Videos API is not supported for this platform")
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

func TestGatewayRoutesGrokImagesAndVideosPathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformGrok)

	for _, path := range []string{
		"/v1/images/generations",
		"/v1/images/edits",
		"/images/generations",
		"/images/edits",
		"/v1/videos/generations",
		"/videos/generations",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"grok-imagine","prompt":"draw a cat"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assertGatewayRouteRegistered(t, router, http.MethodPost, path)
		require.NotContains(t, w.Body.String(), "not supported for this platform")
	}

	for _, path := range []string{
		"/v1/videos/request-123",
		"/videos/request-123",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assertGatewayRouteRegistered(t, router, http.MethodGet, path)
		require.NotContains(t, w.Body.String(), "not supported for this platform")
	}
}

func TestGatewayRoutesNonGrokVideosAreRejectedAtPlatformGate(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformOpenAI)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/videos/generations", `{"model":"grok-imagine-video-1.5","prompt":"waves"}`},
		{http.MethodPost, "/videos/generations", `{"model":"grok-imagine-video-1.5","prompt":"waves"}`},
		{http.MethodGet, "/v1/videos/request-123", ""},
		{http.MethodGet, "/videos/request-123", ""},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, "method=%s path=%s", tc.method, tc.path)
		require.Contains(t, w.Body.String(), "Videos API is not supported for this platform")
	}
}

func TestGatewayRoutesGrokAllowsCLICompatibilityEntrypoints(t *testing.T) {
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
		assertGatewayRouteRegistered(t, router, tc.method, tc.path)
		require.NotContains(t, w.Body.String(), "not supported for Grok groups")
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(`{"model":"grok","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	// Grok groups fail closed for count_tokens (no xAI input_tokens endpoint).
	// Must be intentional not_found, not a misleading capacity/503 selection error.
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not_found_error")
	require.Contains(t, w.Body.String(), "Token counting is not supported for Grok groups")
	require.NotContains(t, strings.ToLower(w.Body.String()), "no available")
	require.NotContains(t, strings.ToLower(w.Body.String()), "capacity")

	for _, path := range []string{
		"/v1/responses",
		"/responses",
		"/backend-api/codex/responses",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"grok","input":"hi"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assertGatewayRouteRegistered(t, router, http.MethodPost, path)
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

func TestGatewayRoutesGrokCountTokensDocsMatchUnsupportedBehavior(t *testing.T) {
	readme, err := os.ReadFile("../../../../README.md")
	require.NoError(t, err)
	readmeCN, err := os.ReadFile("../../../../README_CN.md")
	require.NoError(t, err)

	require.Contains(t, string(readme), "Grok groups do not support `/v1/messages/count_tokens`")
	require.NotContains(t, string(readme), "Public Claude-compatible targets: `/v1/messages` and `/v1/messages/count_tokens`")
	require.Contains(t, string(readmeCN), "Grok 分组不支持 `count_tokens`")
	require.NotContains(t, string(readmeCN), "`/v1/messages`（含 `count_tokens`）")
}

func TestGatewayRoutesOpenAICountTokensPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformOpenAI)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assertGatewayRouteRegistered(t, router, http.MethodPost, "/v1/messages/count_tokens")
}
