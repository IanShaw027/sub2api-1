package admin

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// InvoiceHandler serves admin invoice review, issuance, and voiding.
type InvoiceHandler struct {
	invoiceService *service.InvoiceService
	mediaService   *service.MediaService
}

func NewInvoiceHandler(invoiceService *service.InvoiceService, mediaService *service.MediaService) *InvoiceHandler {
	return &InvoiceHandler{invoiceService: invoiceService, mediaService: mediaService}
}

func (h *InvoiceHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var userID int64
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			userID = v
		}
	}
	items, total, err := h.invoiceService.List(c.Request.Context(), service.InvoiceListParams{
		UserID:   userID,
		Status:   strings.TrimSpace(c.Query("status")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *InvoiceHandler) UnreadCount(c *gin.Context) {
	n, err := h.invoiceService.CountUnreadApplied(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"count": n})
}

func (h *InvoiceHandler) Get(c *gin.Context) {
	id, ok := parseAdminInvoiceID(c)
	if !ok {
		return
	}
	view, err := h.invoiceService.GetAndMarkRead(c.Request.Context(), id, 0, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *InvoiceHandler) Cancel(c *gin.Context) {
	id, ok := parseAdminInvoiceID(c)
	if !ok {
		return
	}
	view, err := h.invoiceService.Cancel(c.Request.Context(), id, 0, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

type issueInvoiceRequest struct {
	MediaID int64 `json:"media_id"`
}

func (h *InvoiceHandler) Issue(c *gin.Context) {
	id, ok := parseAdminInvoiceID(c)
	if !ok {
		return
	}
	mediaID := int64(0)
	if strings.Contains(strings.ToLower(c.ContentType()), "multipart/form-data") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxMediaUploadBytes+(1<<20))
		view, err := h.invoiceService.Get(c.Request.Context(), id, 0, true)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			response.BadRequest(c, "file is required")
			return
		}
		defer func() { _ = file.Close() }()
		data, err := io.ReadAll(io.LimitReader(file, service.MaxMediaUploadBytes+1))
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if int64(len(data)) > service.MaxMediaUploadBytes {
			response.ErrorFrom(c, service.ErrMediaTooLarge)
			return
		}
		filename := ""
		if header != nil {
			filename = header.Filename
		}
		asset, err := h.mediaService.Upload(c.Request.Context(), service.UploadMediaInput{
			OwnerUserID:  view.UserID,
			BizType:      service.MediaBizInvoice,
			BizID:        strconv.FormatInt(id, 10),
			Filename:     filename,
			Visibility:   service.MediaVisibilityPrivate,
			ActorIsAdmin: true,
			Data:         data,
		})
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		mediaID = asset.ID
	} else {
		var req issueInvoiceRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.MediaID <= 0 {
			response.BadRequest(c, "media_id is required")
			return
		}
		mediaID = req.MediaID
	}
	view, err := h.invoiceService.Issue(c.Request.Context(), id, mediaID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *InvoiceHandler) ResendEmail(c *gin.Context) {
	id, ok := parseAdminInvoiceID(c)
	if !ok {
		return
	}
	if err := h.invoiceService.ResendIssuedEmail(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "sent"})
}

func (h *InvoiceHandler) DownloadGrant(c *gin.Context) {
	id, ok := parseAdminInvoiceID(c)
	if !ok {
		return
	}
	view, err := h.invoiceService.Get(c.Request.Context(), id, 0, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if view.FileMediaID == nil {
		response.ErrorFrom(c, service.ErrInvoiceFileRequired)
		return
	}
	grant, err := h.mediaService.CreateDownloadGrant(c.Request.Context(), service.CreateDownloadGrantInput{
		AssetID:      *view.FileMediaID,
		ActorIsAdmin: true,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, grant)
}

func parseAdminInvoiceID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid invoice id")
		return 0, false
	}
	return id, true
}
