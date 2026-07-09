package admin

import (
	"context"
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
	mediaService   *service.MediaService
}

func NewTicketHandler(ticketService *service.TicketService, settingService *service.SettingService, mediaService *service.MediaService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService, settingService: settingService, mediaService: mediaService}
}

type UpdateTicketStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type CreateTicketReplyRequest struct {
	Content     string                       `json:"content"`
	Attachments []TicketAttachmentRefRequest `json:"attachments,omitempty"`
}

type TicketAttachmentRefRequest struct {
	MediaID int64 `json:"media_id" binding:"required"`
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
	ctx := service.WithRequestBaseURL(c.Request.Context(), requestBaseURL(c))
	for i := range items {
		h.hydrateMessageAttachmentsForAdmin(ctx, &items[i])
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
	content := strings.TrimSpace(req.Content)
	if content == "" && len(req.Attachments) == 0 {
		response.ErrorFrom(c, service.ErrTicketMessageRequired)
		return
	}
	executeAdminIdempotentJSON(c, "admin.tickets:reply", map[string]any{
		"ticket_id":    ticketID,
		"content":      content,
		"attachments":  req.Attachments,
		"expected_uid": subject.UserID,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		attachments, err := h.resolveAttachmentsForAdmin(ctx, ticketID, req.Attachments)
		if err != nil {
			return nil, err
		}
		if err := h.ticketService.ReplyForAdmin(ctx, ticketID, service.CreateSupportTicketMessageInput{
			UserID:      subject.UserID,
			Content:     content,
			Attachments: attachments,
		}); err != nil {
			return nil, err
		}
		return gin.H{"message": "ok"}, nil
	})
}

func (h *TicketHandler) resolveAttachmentsForAdmin(ctx context.Context, ticketID int64, refs []TicketAttachmentRefRequest) ([]service.TicketMessageAttachment, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	if h.mediaService == nil {
		return nil, service.ErrMediaStorageDisabled
	}
	attachments := make([]service.TicketMessageAttachment, 0, len(refs))
	for _, ref := range refs {
		asset, err := h.getTicketScopedMediaForAdmin(ctx, ticketID, ref.MediaID)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, service.TicketMessageAttachment{
			MediaID:     asset.ID,
			FileName:    asset.OriginalFileName,
			ContentType: asset.MIMEType,
			SizeBytes:   asset.SizeBytes,
		})
	}
	return attachments, nil
}

func (h *TicketHandler) getTicketScopedMediaForAdmin(ctx context.Context, ticketID, mediaID int64) (*service.MediaAsset, error) {
	if h.mediaService == nil {
		return nil, service.ErrMediaStorageDisabled
	}
	asset, err := h.mediaService.GetForAdmin(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	if asset == nil || strings.TrimSpace(asset.Status) != service.MediaStatusActive {
		return nil, service.ErrMediaNotFound
	}
	if strings.TrimSpace(asset.BizType) != "ticket" || strings.TrimSpace(asset.BizID) != strconv.FormatInt(ticketID, 10) {
		return nil, service.ErrMediaForbidden
	}
	return asset, nil
}

func (h *TicketHandler) hydrateMessageAttachmentsForAdmin(ctx context.Context, message *service.SupportTicketMessage) {
	if h.mediaService == nil || message == nil || len(message.Attachments) == 0 {
		return
	}
	for i := range message.Attachments {
		attachment := &message.Attachments[i]
		asset, err := h.getTicketScopedMediaForAdmin(ctx, message.TicketID, attachment.MediaID)
		if err != nil {
			continue
		}
		download, err := h.mediaService.CreateDownloadURLForAdmin(ctx, asset.ID)
		if err == nil && download != nil {
			attachment.URL = download.URL
		}
		if strings.TrimSpace(asset.ThumbnailObjectKey) == "" {
			attachment.ThumbnailURL = attachment.URL
			continue
		}
		thumbnail, err := h.mediaService.CreateThumbnailDownloadURLForAdmin(ctx, asset.ID)
		if err == nil && thumbnail != nil {
			attachment.ThumbnailURL = thumbnail.URL
		}
	}
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
