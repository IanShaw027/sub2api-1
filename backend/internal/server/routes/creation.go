package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterCreationRoutes(
	v1 *gin.RouterGroup,
	creationHandler *handler.CreationHandler,
	jwtAuth middleware.JWTAuthMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
	compositeResolver *service.CompositeRouteResolver,
) {
	creation := v1.Group("/creation")
	creation.Use(gin.HandlerFunc(jwtAuth))
	creation.Use(middleware.BackendModeUserGuard(settingService))
	creation.Use(middleware.CreationFeatureGuard(settingService))
	creation.Use(panelRateLimiter.Global())
	{
		gateway := creation.Group("")
		gateway.Use(creationHandler.GatewayContext)
		gateway.Use(compositeTargetPlatformMiddleware(compositeResolver))
		gateway.POST("/chat/completions", creationHandler.ChatCompletions)
		gateway.POST("/messages", creationHandler.Messages)
		gateway.POST("/images/generations", creationHandler.Images)
		gateway.POST("/images/generations/async", creationHandler.ImagesAsync)
		gateway.GET("/images/tasks/:task_id", creationHandler.ImageTask)
		gateway.GET("/models", creationHandler.Models)

		creation.GET("/sessions", creationHandler.ListSessions)
		creation.POST("/sessions", creationHandler.CreateSession)
		creation.GET("/sessions/:id", creationHandler.GetSession)
		creation.PATCH("/sessions/:id", creationHandler.UpdateSession)
		creation.DELETE("/sessions/:id", creationHandler.DeleteSession)
		creation.GET("/sessions/:id/messages", creationHandler.ListSessionMessages)
		creation.POST("/sessions/:id/messages", creationHandler.CreateSessionMessage)

		creation.GET("/images", creationHandler.ListImages)
		creation.GET("/images/:id", creationHandler.GetImage)
	}
}
