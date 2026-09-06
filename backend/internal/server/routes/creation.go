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
		gateway.Use(middleware.ClientRequestID())
		gateway.Use(creationHandler.GatewayContext)
		gateway.Use(compositeTargetPlatformMiddleware(compositeResolver))
		gateway.POST("/chat/completions", creationHandler.ChatCompletions)
		gateway.POST("/messages", creationHandler.Messages)
		gateway.POST("/images/generations", creationHandler.Images)
		gateway.POST("/images/generations/async", creationHandler.ImagesAsync)
		gateway.GET("/images/tasks/:task_id", creationHandler.ImageTask)
		gateway.GET("/models", creationHandler.Models)
		gateway.POST("/audio/speech", creationHandler.Speech)
		gateway.POST("/audio/transcriptions", creationHandler.Transcribe)
		gateway.POST("/audio/realtime-ticket", creationHandler.RealtimeTicket)
		gateway.POST("/local/images/generations/async", creationHandler.LocalImagesAsync)
		gateway.POST("/local/images/edits/async", creationHandler.LocalImagesAsync)
		gateway.GET("/local/images/tasks/:task_id", creationHandler.LocalImageTask)
		gateway.POST("/local/videos/generations", creationHandler.LocalVideoGeneration)
		gateway.POST("/local/videos/edits", creationHandler.LocalVideoEdit)
		gateway.POST("/local/videos/extensions", creationHandler.LocalVideoExtension)
		gateway.GET("/local/videos/:request_id", creationHandler.LocalVideoStatus)
		gateway.GET("/local/videos/:request_id/content", creationHandler.LocalVideoContent)

		creation.GET("/sessions", creationHandler.ListSessions)
		creation.POST("/sessions", creationHandler.CreateSession)
		creation.GET("/sessions/:id", creationHandler.GetSession)
		creation.PATCH("/sessions/:id", creationHandler.UpdateSession)
		creation.DELETE("/sessions/:id", creationHandler.DeleteSession)
		creation.GET("/sessions/:id/messages", creationHandler.ListSessionMessages)
		creation.POST("/sessions/:id/messages", creationHandler.CreateSessionMessage)
		creation.POST("/sessions/:id/exchanges", creationHandler.CreateSessionExchange)

		creation.GET("/images", creationHandler.ListImages)
		creation.GET("/images/:id", creationHandler.GetImage)
	}

	voice := v1.Group("/creation/audio")
	voice.Use(creationHandler.RealtimeVoiceAuth)
	voice.Use(middleware.BackendModeUserGuard(settingService))
	voice.Use(middleware.CreationFeatureGuard(settingService))
	voice.Use(panelRateLimiter.Global())
	voice.GET("/realtime", creationHandler.RealtimeVoice)
}
