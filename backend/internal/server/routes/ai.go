package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterAIRoutes is a compatibility wrapper for isolated route registration.
// Production routing is owned by RegisterUserRoutes and RegisterAdminRoutes.
func RegisterAIRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	adminAuth middleware.AdminAuthMiddleware,
	settingServices ...*service.SettingService,
) {
	settingService := firstAIStudioSettingService(settingServices)

	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	registerUserAIRoutes(authenticated, h, settingService)

	admin := v1.Group("/admin")
	admin.Use(gin.HandlerFunc(adminAuth))
	registerAdminAIRoutes(admin, h, settingService)
}

func firstAIStudioSettingService(settingServices []*service.SettingService) *service.SettingService {
	if len(settingServices) == 0 {
		return nil
	}
	return settingServices[0]
}

func aiStudioFeatureGuard(settingService *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if settingService != nil && settingService.GetAIStudioRuntime(c.Request.Context()).Enabled {
			c.Next()
			return
		}
		response.ErrorFrom(c, service.ErrAIStudioDisabled)
		c.Abort()
	}
}

func registerUserAIRoutes(authenticated *gin.RouterGroup, h *handler.Handlers, settingService *service.SettingService) {
	userAI := authenticated.Group("/user/ai")
	userAI.Use(aiStudioFeatureGuard(settingService))
	{
		userAI.GET("/runtime", h.AI.GetRuntime)
		userAI.GET("/sessions", h.AI.ListSessions)
		userAI.POST("/sessions", h.AI.CreateSession)
		userAI.GET("/sessions/:id", h.AI.GetSession)
		userAI.PUT("/sessions/:id", h.AI.UpdateSession)
		userAI.DELETE("/sessions/:id", h.AI.DeleteSession)
		userAI.GET("/sessions/:id/messages", h.AI.ListSessionMessages)
		userAI.POST("/sessions/:id/messages", h.AI.SendMessage)

		userAI.GET("/prompt-templates", h.AI.ListPromptTemplates)
		userAI.POST("/prompt-templates", h.AI.CreatePromptTemplate)
		userAI.GET("/prompt-templates/:id", h.AI.GetPromptTemplate)
		userAI.PUT("/prompt-templates/:id", h.AI.UpdatePromptTemplate)
		userAI.DELETE("/prompt-templates/:id", h.AI.DeletePromptTemplate)
		userAI.GET("/prompts", h.AI.ListPrompts)
		userAI.POST("/prompts", h.AI.CreatePrompt)
		userAI.PUT("/prompts/:id", h.AI.UpdatePrompt)
		userAI.DELETE("/prompts/:id", h.AI.DeletePrompt)
		userAI.POST("/prompts/:id/clone", h.AI.ClonePrompt)

		userAI.GET("/generation-jobs", h.AI.ListGenerationJobs)
		userAI.POST("/generation-jobs", h.AI.CreateGenerationJob)
		userAI.GET("/generation-jobs/:id", h.AI.GetGenerationJob)

		userAI.POST("/chat", h.AI.Chat)

		userAI.GET("/artworks", h.AI.ListGallery)
		userAI.POST("/artworks", h.AI.CreateArtwork)
		userAI.POST("/artworks/edit", h.AI.EditArtwork)
		userAI.PUT("/artworks/:id", h.AI.UpdateArtwork)
		userAI.DELETE("/artworks/:id", h.AI.DeleteArtwork)

		userAI.GET("/gallery", h.AI.ListGallery)
		userAI.GET("/assets/:id", h.AI.GetAsset)
	}
}
