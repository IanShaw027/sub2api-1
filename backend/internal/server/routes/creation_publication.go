package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterCreationPublicationRoutes(v1 *gin.RouterGroup, h *handler.CreationPublicationHandler, jwtAuth middleware.JWTAuthMiddleware, settings *service.SettingService, limiter *middleware.PanelRateLimiter) {
	if h == nil {
		return
	}
	public := v1.Group("/creation/gallery")
	public.Use(middleware.CreationFeatureGuard(settings))
	public.Use(limiter.PublicIP())
	public.GET("/:id/media", h.Media)

	authenticated := v1.Group("/creation")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settings))
	authenticated.Use(middleware.CreationFeatureGuard(settings))
	authenticated.Use(limiter.Global())
	authenticated.GET("/gallery", h.List)
	authenticated.POST("/publications", h.Upload)
	authenticated.GET("/publications/requests/:request_id", h.Status)
	authenticated.DELETE("/publications/:id", h.Delete)
}
