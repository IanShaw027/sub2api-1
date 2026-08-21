//go:build unit

package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminSensitiveRoutesUseStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminGroup := router.Group("/api/v1/admin")
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Account:        &adminhandler.AccountHandler{},
		Proxy:          &adminhandler.ProxyHandler{},
		Setting:        adminhandler.NewSettingHandler(nil, nil, nil, nil, nil, nil, nil),
		System:         &adminhandler.SystemHandler{},
		DataManagement: &adminhandler.DataManagementHandler{},
		Backup:         &adminhandler.BackupHandler{},
	}}
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) {
		servermiddleware.AbortWithError(c, http.StatusForbidden, "STEP_UP_REQUIRED", "step-up required")
	})

	registerAccountRoutes(adminGroup, handlers, stepUp)
	registerProxyRoutes(adminGroup, handlers, stepUp)
	registerSettingsRoutes(adminGroup, handlers, stepUp)
	registerDataManagementRoutes(adminGroup, handlers, stepUp)
	registerBackupRoutes(adminGroup, handlers, stepUp)
	registerSystemRoutes(adminGroup, handlers, stepUp)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/accounts/data"},
		{http.MethodGet, "/api/v1/admin/proxies/data"},
		{http.MethodPost, "/api/v1/admin/settings/admin-api-key/regenerate"},
		{http.MethodDelete, "/api/v1/admin/settings/admin-api-key"},
		{http.MethodPost, "/api/v1/admin/data-management/s3/profiles"},
		{http.MethodPut, "/api/v1/admin/data-management/s3/profiles/7"},
		{http.MethodDelete, "/api/v1/admin/data-management/s3/profiles/7"},
		{http.MethodPost, "/api/v1/admin/data-management/s3/profiles/7/activate"},
		{http.MethodPost, "/api/v1/admin/data-management/backups"},
		{http.MethodPut, "/api/v1/admin/backups/s3-config"},
		{http.MethodPut, "/api/v1/admin/backups/image-storage"},
		{http.MethodPost, "/api/v1/admin/backups"},
		{http.MethodDelete, "/api/v1/admin/backups/7"},
		{http.MethodGet, "/api/v1/admin/backups/7/download-url"},
		{http.MethodPost, "/api/v1/admin/backups/7/restore"},
		{http.MethodPost, "/api/v1/admin/system/update"},
		{http.MethodPost, "/api/v1/admin/system/rollback"},
		{http.MethodPost, "/api/v1/admin/system/restart"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Contains(t, recorder.Body.String(), "STEP_UP_REQUIRED")
		})
	}
}
