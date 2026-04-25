package admin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type TicketHandler struct {
	ticketService  *service.TicketService
	settingService *service.SettingService
}

func NewTicketHandler(ticketService *service.TicketService, settingService *service.SettingService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService, settingService: settingService}
}

type UpdateTicketStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type CreateTicketReplyRequest struct {
	Content string `json:"content" binding:"required"`
}

type ReplaceTicketReplyTemplatesRequest struct {
	Templates *[]service.AdminTicketReplyTemplate `json:"templates" binding:"required"`
}

func (h *TicketHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	startTime, endTime, err := parseTicketDateRange(c.Query("start_date"), c.Query("end_date"), c.Query("timezone"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, result, err := h.ticketService.ListForAdmin(c.Request.Context(), pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, service.SupportTicketListFilters{
		Status:    strings.TrimSpace(c.Query("status")),
		Category:  strings.TrimSpace(c.Query("category")),
		Search:    strings.TrimSpace(c.Query("search")),
		UserQuery: strings.TrimSpace(c.Query("user")),
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.SupportTicket, 0, len(items))
	for i := range items {
		out = append(out, *dto.SupportTicketFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

func (h *TicketHandler) ListReplyTemplates(c *gin.Context) {
	items, err := h.settingService.GetAdminTicketReplyTemplates(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *TicketHandler) ReplaceReplyTemplates(c *gin.Context) {
	var req ReplaceTicketReplyTemplatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.settingService.SetAdminTicketReplyTemplates(c.Request.Context(), *req.Templates); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *TicketHandler) GetByID(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	item, err := h.ticketService.GetForAdmin(c.Request.Context(), ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SupportTicketFromService(item))
}

func (h *TicketHandler) ListMessages(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	items, err := h.ticketService.ListMessagesForAdmin(c.Request.Context(), ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.SupportTicketMessage, 0, len(items))
	for i := range items {
		out = append(out, *dto.SupportTicketMessageFromService(&items[i]))
	}
	response.Success(c, out)
}

func (h *TicketHandler) Reply(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	var req CreateTicketReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.ticketService.ReplyForAdmin(c.Request.Context(), ticketID, service.CreateSupportTicketMessageInput{
		UserID:  subject.UserID,
		Content: req.Content,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func parseTicketDateRange(startDate, endDate, userTZ string) (*time.Time, *time.Time, error) {
	var startTime *time.Time
	var endTime *time.Time
	if strings.TrimSpace(startDate) != "" {
		parsed, err := timezone.ParseInUserLocation("2006-01-02", strings.TrimSpace(startDate), userTZ)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid start_date")
		}
		startTime = &parsed
	}
	if strings.TrimSpace(endDate) != "" {
		parsed, err := timezone.ParseInUserLocation("2006-01-02", strings.TrimSpace(endDate), userTZ)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid end_date")
		}
		upper := parsed.AddDate(0, 0, 1)
		endTime = &upper
	}
	if startTime != nil && endTime != nil && !endTime.After(*startTime) {
		return nil, nil, fmt.Errorf("invalid date range")
	}
	return startTime, endTime, nil
}

func (h *TicketHandler) UpdateStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	var req UpdateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.ticketService.UpdateStatusByAdmin(c.Request.Context(), ticketID, service.AdminSupportTicketStatusUpdateInput{
		AdminUserID: subject.UserID,
		Status:      req.Status,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}
