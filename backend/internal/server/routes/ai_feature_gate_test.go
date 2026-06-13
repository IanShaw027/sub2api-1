package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type aiStudioRouteSettingRepoStub struct {
	values map[string]string
}

func (s *aiStudioRouteSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *aiStudioRouteSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *aiStudioRouteSettingRepoStub) Set(_ context.Context, key string, value string) error {
	if key != "admin_compliance_acknowledgement" && !strings.HasPrefix(key, "admin_compliance_acknowledgement:") {
		panic("unexpected Set call: " + key)
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *aiStudioRouteSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *aiStudioRouteSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *aiStudioRouteSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *aiStudioRouteSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func newAIStudioRouteSettings(enabled bool) *service.SettingService {
	value := "false"
	if enabled {
		value = "true"
	}
	return service.NewSettingService(&aiStudioRouteSettingRepoStub{
		values: map[string]string{
			service.SettingKeyAIStudioEnabled:    value,
			service.SettingKeyBackendModeEnabled: "false",
		},
	}, &config.Config{})
}

func newAIStudioRouteSettingsForCompliantAdmin(t *testing.T, enabled bool, adminUserID int64) *service.SettingService {
	t.Helper()

	settingService := newAIStudioRouteSettings(enabled)
	_, err := settingService.AcceptAdminCompliance(context.Background(), service.AdminComplianceAcceptInput{
		AdminUserID: adminUserID,
		Language:    "zh",
		Phrase:      service.AdminComplianceAckPhraseZH,
	})
	require.NoError(t, err)

	return settingService
}

func compliantAdminAuth(adminUserID int64) middleware.AdminAuthMiddleware {
	return middleware.AdminAuthMiddleware(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: adminUserID})
		c.Next()
	})
}

func TestRegisterUserRoutesRejectsAIWhenFeatureDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterUserRoutes(
		v1,
		&handler.Handlers{AI: &handler.AIHandler{}},
		middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		newAIStudioRouteSettings(false),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/ai/runtime", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AI_STUDIO_DISABLED")
}

func TestRegisterUserSkillRoutesRejectsWhenFeatureDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterUserRoutes(
		v1,
		&handler.Handlers{AI: &handler.AIHandler{}},
		middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		newAIStudioRouteSettings(false),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/skills", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AI_STUDIO_DISABLED")
}

func TestRegisterAdminRoutesRejectsAIWhenFeatureDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const adminUserID int64 = 1

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterAdminRoutes(
		v1,
		&handler.Handlers{
			Admin: &handler.AdminHandlers{
				AI: &admin.AIHandler{},
			},
		},
		compliantAdminAuth(adminUserID),
		newAIStudioRouteSettingsForCompliantAdmin(t, false, adminUserID),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai/prompts", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AI_STUDIO_DISABLED")
}

func TestRegisterAdminSkillRoutesRejectsWhenFeatureDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const adminUserID int64 = 1

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterAdminRoutes(
		v1,
		&handler.Handlers{
			Admin: &handler.AdminHandlers{
				AI: &admin.AIHandler{},
			},
		},
		compliantAdminAuth(adminUserID),
		newAIStudioRouteSettingsForCompliantAdmin(t, false, adminUserID),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/skills/governance", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AI_STUDIO_DISABLED")
}
