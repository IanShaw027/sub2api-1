package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterMediaRoutes registers public media GET/HMAC download plus authenticated upload/presign.
func RegisterMediaRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	if h == nil || h.Media == nil {
		return
	}

	public := v1.Group("/media")
	public.Use(panelRateLimiter.PublicIP())
	{
		public.GET("/public/:id", h.Media.PublicGet)
		public.GET("/download/:id", h.Media.SignedDownload)
	}

	authenticated := v1.Group("/media")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	authenticated.Use(panelRateLimiter.Global())
	{
		authenticated.POST("/upload", h.Media.Upload)
		authenticated.GET("/:id", h.Media.Get)
		authenticated.DELETE("/:id", h.Media.Delete)
		authenticated.POST("/:id/visibility", h.Media.UpdateVisibility)
		authenticated.POST("/:id/presign-download", h.Media.PresignDownload)
	}

	admin := v1.Group("/admin/media")
	admin.Use(gin.HandlerFunc(adminAuth))
	admin.Use(panelRateLimiter.Global())
	admin.Use(gin.HandlerFunc(auditLog))
	admin.Use(middleware.AdminComplianceGuard(settingService))
	{
		admin.POST("/upload", h.Media.AdminUpload)
		admin.GET("/:id", h.Media.AdminGet)
		admin.DELETE("/:id", h.Media.AdminDelete)
		admin.POST("/:id/visibility", h.Media.AdminUpdateVisibility)
		admin.POST("/:id/presign-download", h.Media.AdminPresignDownload)
	}
}
