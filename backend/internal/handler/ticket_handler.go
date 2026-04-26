package handler

import (
	"encoding/json"
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
	ticketService *service.TicketService
}

func NewTicketHandler(ticketService *service.TicketService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService}
}

type CreateTicketRequest struct {
	Category    string          `json:"category" binding:"required"`
	Title       string          `json:"title" binding:"required"`
	FormPayload json.RawMessage `json:"form_payload" binding:"required"`
}

type UpdateTicketRequest struct {
	Title       string          `json:"title" binding:"required"`
	FormPayload json.RawMessage `json:"form_payload" binding:"required"`
}

type CreateTicketMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

func (h *TicketHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	startTime, endTime, err := parseTicketDateRange(c.Query("start_date"), c.Query("end_date"), c.Query("timezone"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, result, err := h.ticketService.ListForUser(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, service.SupportTicketListFilters{
		Status:    strings.TrimSpace(c.Query("status")),
		Category:  strings.TrimSpace(c.Query("category")),
		Search:    strings.TrimSpace(c.Query("search")),
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.UserSupportTicket, 0, len(items))
	for i := range items {
		out = append(out, *dto.UserSupportTicketFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

func (h *TicketHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.ticketService.Create(c.Request.Context(), service.CreateSupportTicketInput{
		UserID:      subject.UserID,
		Category:    req.Category,
		Title:       req.Title,
		FormPayload: req.FormPayload,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UserSupportTicketFromService(item))
}

func (h *TicketHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	item, err := h.ticketService.GetForUser(c.Request.Context(), subject.UserID, ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UserSupportTicketFromService(item))
}

func (h *TicketHandler) ListMessages(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	items, err := h.ticketService.ListMessagesForUser(c.Request.Context(), subject.UserID, ticketID)
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

func (h *TicketHandler) Withdraw(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	if err := h.ticketService.Withdraw(c.Request.Context(), subject.UserID, ticketID); err != nil {
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

func (h *TicketHandler) Update(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	var req UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.ticketService.UpdateEditable(c.Request.Context(), service.UpdateSupportTicketInput{
		UserID:      subject.UserID,
		Title:       req.Title,
		FormPayload: req.FormPayload,
	}, ticketID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *TicketHandler) Resubmit(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	var req UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.ticketService.Resubmit(c.Request.Context(), service.UpdateSupportTicketInput{
		UserID:      subject.UserID,
		Title:       req.Title,
		FormPayload: req.FormPayload,
	}, ticketID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *TicketHandler) Close(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	if err := h.ticketService.CloseForUser(c.Request.Context(), subject.UserID, ticketID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *TicketHandler) Reply(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}
	var req CreateTicketMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.ticketService.ReplyForUser(c.Request.Context(), ticketID, service.CreateSupportTicketMessageInput{
		UserID:  subject.UserID,
		Content: req.Content,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}
