package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterCreationMediaPricingRoutes(v1 *gin.RouterGroup, h *handler.CreationMediaPricingHandler, jwtAuth middleware.JWTAuthMiddleware, settings *service.SettingService, limiter *middleware.PanelRateLimiter) {
	if h == nil {
		return
	}
	group := v1.Group("/creation/local")
	group.Use(gin.HandlerFunc(jwtAuth))
	group.Use(middleware.BackendModeUserGuard(settings))
	group.Use(middleware.CreationFeatureGuard(settings))
	group.Use(limiter.Global())
	group.GET("/pricing", h.Estimate)
	group.GET("/billing", h.Receipt)
}
