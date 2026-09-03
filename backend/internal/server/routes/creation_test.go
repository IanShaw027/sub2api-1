//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type creationRouteSettingRepoStub struct {
	values map[string]string
}

func (s *creationRouteSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *creationRouteSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *creationRouteSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *creationRouteSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *creationRouteSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *creationRouteSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *creationRouteSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func newCreationRouteSettings() *service.SettingService {
	return service.NewSettingService(&creationRouteSettingRepoStub{
		values: map[string]string{
			service.SettingKeyBackendModeEnabled:    "false",
			service.SettingKeyCreationCenterEnabled: "true",
		},
	}, nil)
}

func newCreationRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	settingService := newCreationRouteSettings()
	h := handler.NewCreationHandler(nil, nil, nil, nil, nil, nil)

	creation := v1.Group("/creation")
	creation.Use(gin.HandlerFunc(servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
		c.Next()
	})))
	creation.Use(servermiddleware.BackendModeUserGuard(settingService))
	creation.Use(servermiddleware.CreationFeatureGuard(settingService))
	{
		creation.POST("/chat/completions", h.ChatCompletions)
		creation.POST("/messages", h.Messages)
		creation.GET("/models", h.Models)
		creation.GET("/sessions", h.ListSessions)
		creation.POST("/sessions", h.CreateSession)
		creation.GET("/images", h.ListImages)
		creation.GET("/images/tasks/:task_id", h.ImageTask)
		creation.GET("/images/:id", h.GetImage)
	}
	return router
}

func TestCreationRoutesAreRegistered(t *testing.T) {
	router := newCreationRoutesTestRouter()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/creation/chat/completions"},
		{http.MethodPost, "/api/v1/creation/messages"},
		{http.MethodGet, "/api/v1/creation/models?group_id=1"},
		{http.MethodGet, "/api/v1/creation/sessions"},
		{http.MethodPost, "/api/v1/creation/sessions"},
		{http.MethodGet, "/api/v1/creation/images"},
		{http.MethodGet, "/api/v1/creation/images/tasks/imgtask_abc"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should be registered", tc.path)
	}
}

func TestCreationImageTaskRouteUsesTaskIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	settingService := newCreationRouteSettings()
	jwt := servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
		c.Next()
	})
	RegisterCreationRoutes(
		v1,
		handler.NewCreationHandler(nil, nil, nil, nil, nil, nil),
		jwt,
		settingService,
		servermiddleware.NewPanelRateLimiter(nil, settingService),
	)

	var foundTaskID bool
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/creation/images/tasks/:task_id" {
			foundTaskID = true
		}
		require.NotEqual(t, "/api/v1/creation/images/tasks/:id", route.Path)
	}
	require.True(t, foundTaskID, "creation image task route must use :task_id")
}

func TestCreationImageTaskRouteForwardsTaskIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	settingService := newCreationRouteSettings()
	jwt := servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
		c.Next()
	})
	creation := v1.Group("/creation")
	creation.Use(gin.HandlerFunc(jwt))
	creation.Use(servermiddleware.BackendModeUserGuard(settingService))
	creation.Use(servermiddleware.CreationFeatureGuard(settingService))
	creation.GET("/images/tasks/:task_id", func(c *gin.Context) {
		c.String(http.StatusOK, c.Param("task_id"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/creation/images/tasks/imgtask_forwarded", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "imgtask_forwarded", w.Body.String())
}
