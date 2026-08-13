package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TicketHandler struct {
	ticketService *service.TicketService
	mediaService  *service.MediaService
}

func NewTicketHandler(ticketService *service.TicketService, mediaService *service.MediaService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService, mediaService: mediaService}
}

type adminTicketReplyRequest struct {
	Content  string  `json:"content"`
	MediaIDs []int64 `json:"media_ids"`
}

type adminTicketStatusRequest struct {
	Status string `json:"status"`
}

type ticketTemplateRequest struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	SortOrder int    `json:"sort_order"`
}

func (h *TicketHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.ticketService.ListForAdmin(c.Request.Context(), service.SupportTicketListFilters{
		Status:     strings.TrimSpace(c.Query("status")),
		Category:   strings.TrimSpace(c.Query("category")),
		Search:     strings.TrimSpace(c.Query("keyword")),
		UnreadOnly: c.Query("unread_only") == "1" || c.Query("unread_only") == "true",
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *TicketHandler) UnreadCount(c *gin.Context) {
	n, err := h.ticketService.CountUnreadForAdmin(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"count": n})
}

func (h *TicketHandler) Get(c *gin.Context) {
	id, ok := parseAdminTicketID(c)
	if !ok {
		return
	}
	view, err := h.ticketService.GetForAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *TicketHandler) Messages(c *gin.Context) {
	id, ok := parseAdminTicketID(c)
	if !ok {
		return
	}
	items, err := h.ticketService.ListMessagesForAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *TicketHandler) Reply(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseAdminTicketID(c)
	if !ok {
		return
	}
	var req adminTicketReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if err := h.ticketService.ReplyForAdmin(c.Request.Context(), id, service.CreateSupportTicketMessageInput{
		UserID:   subject.UserID,
		Content:  req.Content,
		MediaIDs: req.MediaIDs,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *TicketHandler) UpdateStatus(c *gin.Context) {
	id, ok := parseAdminTicketID(c)
	if !ok {
		return
	}
	var req adminTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if err := h.ticketService.UpdateStatusByAdmin(c.Request.Context(), id, req.Status); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *TicketHandler) ListTemplates(c *gin.Context) {
	items, err := h.ticketService.ListReplyTemplates(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *TicketHandler) CreateTemplate(c *gin.Context) {
	var req ticketTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	view, err := h.ticketService.CreateReplyTemplate(c.Request.Context(), req.Title, req.Content, req.SortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *TicketHandler) UpdateTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("template_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid template id")
		return
	}
	var req ticketTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	view, err := h.ticketService.UpdateReplyTemplate(c.Request.Context(), id, req.Title, req.Content, req.SortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *TicketHandler) DeleteTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("template_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid template id")
		return
	}
	if err := h.ticketService.DeleteReplyTemplate(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *TicketHandler) DownloadGrant(c *gin.Context) {
	id, ok := parseAdminTicketID(c)
	if !ok {
		return
	}
	mediaID, err := strconv.ParseInt(c.Param("media_id"), 10, 64)
	if err != nil || mediaID <= 0 {
		response.BadRequest(c, "invalid media id")
		return
	}
	if err := h.ticketService.EnsureTicketAttachmentAccess(c.Request.Context(), id, mediaID, 0, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	grant, err := h.mediaService.CreateDownloadGrant(c.Request.Context(), service.CreateDownloadGrantInput{
		AssetID:        mediaID,
		ActorIsAdmin:   true,
		VerifiedAccess: true,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, grant)
}

func parseAdminTicketID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid ticket id")
		return 0, false
	}
	return id, true
}
