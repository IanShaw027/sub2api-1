package handler

import (
	"context"
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
	mediaService  *service.MediaService
}

func NewTicketHandler(ticketService *service.TicketService, mediaService *service.MediaService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService, mediaService: mediaService}
}

type CreateTicketRequest struct {
	Category    string          `json:"category" binding:"required"`
	Title       string          `json:"title" binding:"required"`
	FormPayload json.RawMessage `json:"form_payload" binding:"required"`
}

type UpdateTicketRequest struct {
	Title              string          `json:"title" binding:"required"`
	FormPayload        json.RawMessage `json:"form_payload" binding:"required"`
	ExpectedRevisionNo int             `json:"expected_revision_no"`
}

type CreateTicketMessageRequest struct {
	Content     string                       `json:"content"`
	Attachments []TicketAttachmentRefRequest `json:"attachments,omitempty"`
}

type TicketAttachmentRefRequest struct {
	MediaID int64 `json:"media_id" binding:"required"`
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
	ctx := service.WithRequestBaseURL(c.Request.Context(), requestBaseURL(c))
	for i := range items {
		h.hydrateMessageAttachmentsForUser(ctx, subject.UserID, &items[i])
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
		UserID:             subject.UserID,
		Title:              req.Title,
		FormPayload:        req.FormPayload,
		ExpectedRevisionNo: expectedTicketRevisionNo(c, req.ExpectedRevisionNo),
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
		UserID:             subject.UserID,
		Title:              req.Title,
		FormPayload:        req.FormPayload,
		ExpectedRevisionNo: expectedTicketRevisionNo(c, req.ExpectedRevisionNo),
	}, ticketID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func expectedTicketRevisionNo(c *gin.Context, payloadRevisionNo int) int {
	if payloadRevisionNo > 0 {
		return payloadRevisionNo
	}
	headerValue := strings.TrimSpace(c.GetHeader("If-Match"))
	headerValue = strings.Trim(headerValue, `"`)
	if headerValue == "" {
		return 0
	}
	revisionNo, err := strconv.Atoi(headerValue)
	if err != nil || revisionNo <= 0 {
		return 0
	}
	return revisionNo
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
	content := strings.TrimSpace(req.Content)
	if content == "" && len(req.Attachments) == 0 {
		response.ErrorFrom(c, service.ErrTicketMessageRequired)
		return
	}
	executeUserIdempotentJSON(c, "tickets:reply", map[string]any{
		"ticket_id":    ticketID,
		"content":      content,
		"attachments":  req.Attachments,
		"expected_uid": subject.UserID,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		attachments, err := h.resolveAttachmentsForUser(ctx, subject.UserID, ticketID, req.Attachments)
		if err != nil {
			return nil, err
		}
		if err := h.ticketService.ReplyForUser(ctx, ticketID, service.CreateSupportTicketMessageInput{
			UserID:      subject.UserID,
			Content:     content,
			Attachments: attachments,
		}); err != nil {
			return nil, err
		}
		return gin.H{"message": "ok"}, nil
	})
}

func (h *TicketHandler) resolveAttachmentsForUser(ctx context.Context, userID, ticketID int64, refs []TicketAttachmentRefRequest) ([]service.TicketMessageAttachment, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	if h.mediaService == nil {
		return nil, service.ErrMediaStorageDisabled
	}
	attachments := make([]service.TicketMessageAttachment, 0, len(refs))
	for _, ref := range refs {
		asset, err := h.getTicketScopedMediaForUser(ctx, userID, ticketID, ref.MediaID)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, service.TicketMessageAttachment{
			MediaID:      asset.ID,
			URL:          h.mediaService.PublicURL(asset),
			ThumbnailURL: h.mediaService.ThumbnailPublicURL(asset),
			FileName:     asset.OriginalFileName,
			ContentType:  asset.MIMEType,
			SizeBytes:    asset.SizeBytes,
		})
	}
	return attachments, nil
}

func (h *TicketHandler) getTicketScopedMediaForUser(ctx context.Context, userID, ticketID, mediaID int64) (*service.MediaAsset, error) {
	if h.mediaService == nil {
		return nil, service.ErrMediaStorageDisabled
	}
	asset, err := h.mediaService.GetForUser(ctx, userID, mediaID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(asset.BizType) != "ticket" || strings.TrimSpace(asset.BizID) != strconv.FormatInt(ticketID, 10) {
		return nil, service.ErrMediaForbidden
	}
	return asset, nil
}

func (h *TicketHandler) hydrateMessageAttachmentsForUser(ctx context.Context, userID int64, message *service.SupportTicketMessage) {
	if h.mediaService == nil || message == nil || len(message.Attachments) == 0 {
		return
	}
	for i := range message.Attachments {
		attachment := &message.Attachments[i]
		if _, err := h.getTicketScopedMediaForUser(ctx, userID, message.TicketID, attachment.MediaID); err != nil {
			continue
		}
		download, err := h.mediaService.CreateDownloadURLForUser(ctx, userID, attachment.MediaID)
		if err == nil && download != nil {
			attachment.URL = download.URL
		}
		thumbnail, err := h.mediaService.CreateThumbnailDownloadURLForUser(ctx, userID, attachment.MediaID)
		if err == nil && thumbnail != nil {
			attachment.ThumbnailURL = thumbnail.URL
		}
	}
}
