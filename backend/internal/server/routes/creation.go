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
) {
	creation := v1.Group("/creation")
	creation.Use(gin.HandlerFunc(jwtAuth))
	creation.Use(middleware.BackendModeUserGuard(settingService))
	creation.Use(middleware.CreationFeatureGuard(settingService))
	creation.Use(panelRateLimiter.Global())
	{
		creation.POST("/chat/completions", creationHandler.ChatCompletions)
		creation.POST("/messages", creationHandler.Messages)
		creation.POST("/images/generations", creationHandler.Images)
		creation.POST("/images/generations/async", creationHandler.ImagesAsync)
		creation.GET("/images/tasks/:id", creationHandler.ImageTask)
		creation.GET("/models", creationHandler.Models)

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
