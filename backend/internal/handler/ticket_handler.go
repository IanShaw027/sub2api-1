package handler

import (
	"encoding/json"
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

type createTicketRequest struct {
	Category    string          `json:"category"`
	Title       string          `json:"title"`
	FormPayload json.RawMessage `json:"form_payload"`
}

type updateTicketRequest struct {
	Title              string          `json:"title"`
	FormPayload        json.RawMessage `json:"form_payload"`
	ExpectedRevisionNo int             `json:"expected_revision_no"`
}

type ticketReplyRequest struct {
	Content  string  `json:"content"`
	MediaIDs []int64 `json:"media_ids"`
}

func (h *TicketHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	view, err := h.ticketService.Create(c.Request.Context(), service.CreateSupportTicketInput{
		UserID:      subject.UserID,
		Category:    req.Category,
		Title:       req.Title,
		FormPayload: req.FormPayload,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *TicketHandler) ListMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	startAt, endAt, err := service.ParseSupportTicketDateRange(c.Query("start_date"), c.Query("end_date"), c.Query("timezone"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items, total, err := h.ticketService.ListForUser(c.Request.Context(), subject.UserID, service.SupportTicketListFilters{
		Status:     strings.TrimSpace(c.Query("status")),
		Category:   strings.TrimSpace(c.Query("category")),
		Search:     strings.TrimSpace(c.Query("keyword")),
		UnreadOnly: c.Query("unread_only") == "1" || c.Query("unread_only") == "true",
		StartAt:    startAt,
		EndAt:      endAt,
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
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	n, err := h.ticketService.CountUnreadForUser(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"count": n})
}

func (h *TicketHandler) GetMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseTicketID(c)
	if !ok {
		return
	}
	view, err := h.ticketService.GetForUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *TicketHandler) MessagesMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseTicketID(c)
	if !ok {
		return
	}
	items, err := h.ticketService.ListMessagesForUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *TicketHandler) Withdraw(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseTicketID(c)
	if !ok {
		return
	}
	if err := h.ticketService.Withdraw(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *TicketHandler) UpdateMine(c *gin.Context) {
	h.updateOrResubmit(c, false)
}

func (h *TicketHandler) Resubmit(c *gin.Context) {
	h.updateOrResubmit(c, true)
}

func (h *TicketHandler) updateOrResubmit(c *gin.Context, resubmit bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseTicketID(c)
	if !ok {
		return
	}
	var req updateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	in := service.UpdateSupportTicketInput{
		UserID:             subject.UserID,
		Title:              req.Title,
		FormPayload:        req.FormPayload,
		ExpectedRevisionNo: req.ExpectedRevisionNo,
	}
	var err error
	if resubmit {
		err = h.ticketService.Resubmit(c.Request.Context(), id, in)
	} else {
		err = h.ticketService.UpdateEditable(c.Request.Context(), id, in)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *TicketHandler) CloseMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseTicketID(c)
	if !ok {
		return
	}
	if err := h.ticketService.CloseForUser(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *TicketHandler) ReplyMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseTicketID(c)
	if !ok {
		return
	}
	var req ticketReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if err := h.ticketService.ReplyForUser(c.Request.Context(), id, service.CreateSupportTicketMessageInput{
		UserID:   subject.UserID,
		Content:  req.Content,
		MediaIDs: req.MediaIDs,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *TicketHandler) RateGroups(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.ticketService.ListRateApplyGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *TicketHandler) DownloadGrant(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseTicketID(c)
	if !ok {
		return
	}
	mediaID, err := strconv.ParseInt(c.Param("media_id"), 10, 64)
	if err != nil || mediaID <= 0 {
		response.BadRequest(c, "invalid media id")
		return
	}
	if err := h.ticketService.EnsureTicketAttachmentAccess(c.Request.Context(), id, mediaID, subject.UserID, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	grant, err := h.mediaService.CreateDownloadGrant(c.Request.Context(), service.CreateDownloadGrantInput{
		AssetID:        mediaID,
		ActorUserID:    subject.UserID,
		VerifiedAccess: true,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, grant)
}

func parseTicketID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid ticket id")
		return 0, false
	}
	return id, true
}
