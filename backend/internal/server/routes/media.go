package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterMediaRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	adminAuth middleware.AdminAuthMiddleware,
	settingService *service.SettingService,
) {
	public := v1.Group("/media")
	{
		public.GET("/public/:id", h.Media.ServePublic)
		public.GET("/public/:id/thumbnail", h.Media.ServePublicThumbnail)
		public.GET("/download/:id", h.Media.ServeSignedDownload)
		public.GET("/download/:id/thumbnail", h.Media.ServeSignedThumbnailDownload)
	}

	authenticated := v1.Group("/media")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	{
		authenticated.POST("/upload", h.Media.Upload)
		authenticated.GET("/:id", h.Media.GetByID)
		authenticated.DELETE("/:id", h.Media.Delete)
		authenticated.POST("/:id/visibility", h.Media.UpdateVisibility)
		authenticated.POST("/:id/presign-download", h.Media.PresignDownload)
		authenticated.POST("/:id/presign-thumbnail-download", h.Media.PresignThumbnailDownload)
	}

	adminGroup := v1.Group("/admin/media")
	adminGroup.Use(gin.HandlerFunc(adminAuth))
	{
		adminGroup.GET("", h.Admin.Media.List)
		adminGroup.POST("/upload", h.Admin.Media.Upload)
		adminGroup.GET("/:id", h.Admin.Media.GetByID)
		adminGroup.DELETE("/:id", h.Admin.Media.Delete)
		adminGroup.POST("/:id/visibility", h.Admin.Media.UpdateVisibility)
		adminGroup.POST("/:id/presign-download", h.Admin.Media.PresignDownload)
		adminGroup.POST("/:id/presign-thumbnail-download", h.Admin.Media.PresignThumbnailDownload)
	}
}
