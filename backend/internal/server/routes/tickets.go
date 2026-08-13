package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterTicketRoutes(
	v1 *gin.RouterGroup,
	ticketHandler *handler.TicketHandler,
	adminTicketHandler *admin.TicketHandler,
	jwtAuth middleware.JWTAuthMiddleware,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	userTickets := v1.Group("/tickets")
	userTickets.Use(gin.HandlerFunc(jwtAuth))
	userTickets.Use(middleware.BackendModeUserGuard(settingService))
	userTickets.Use(panelRateLimiter.Global())
	{
		userTickets.POST("", ticketHandler.Create)
		userTickets.GET("", ticketHandler.ListMine)
		userTickets.GET("/unread-count", ticketHandler.UnreadCount)
		userTickets.GET("/rate-groups", ticketHandler.RateGroups)
		userTickets.GET("/:id", ticketHandler.GetMine)
		userTickets.GET("/:id/messages", ticketHandler.MessagesMine)
		userTickets.POST("/:id/withdraw", ticketHandler.Withdraw)
		userTickets.POST("/:id/update", ticketHandler.UpdateMine)
		userTickets.POST("/:id/resubmit", ticketHandler.Resubmit)
		userTickets.POST("/:id/close", ticketHandler.CloseMine)
		userTickets.POST("/:id/reply", ticketHandler.ReplyMine)
		userTickets.POST("/:id/attachments/:media_id/download-grant", ticketHandler.DownloadGrant)
	}

	adminTickets := v1.Group("/admin/tickets")
	adminTickets.Use(gin.HandlerFunc(adminAuth))
	adminTickets.Use(panelRateLimiter.Global())
	adminTickets.Use(gin.HandlerFunc(auditLog))
	{
		adminTickets.GET("", adminTicketHandler.List)
		adminTickets.GET("/unread-count", adminTicketHandler.UnreadCount)
		adminTickets.GET("/reply-templates", adminTicketHandler.ListTemplates)
		adminTickets.POST("/reply-templates", adminTicketHandler.CreateTemplate)
		adminTickets.PUT("/reply-templates/:template_id", adminTicketHandler.UpdateTemplate)
		adminTickets.DELETE("/reply-templates/:template_id", adminTicketHandler.DeleteTemplate)
		adminTickets.GET("/:id", adminTicketHandler.Get)
		adminTickets.GET("/:id/messages", adminTicketHandler.Messages)
		adminTickets.POST("/:id/reply", adminTicketHandler.Reply)
		adminTickets.POST("/:id/status", adminTicketHandler.UpdateStatus)
		adminTickets.POST("/:id/attachments/:media_id/download-grant", adminTicketHandler.DownloadGrant)
	}
}
