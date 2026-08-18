package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// InvoiceHandler serves user invoice applications and signed downloads.
type InvoiceHandler struct {
	invoiceService *service.InvoiceService
	mediaService   *service.MediaService
	configService  *service.PaymentConfigService
}

func NewInvoiceHandler(invoiceService *service.InvoiceService, mediaService *service.MediaService, configService *service.PaymentConfigService) *InvoiceHandler {
	return &InvoiceHandler{
		invoiceService: invoiceService,
		mediaService:   mediaService,
		configService:  configService,
	}
}

type applyInvoiceRequest struct {
	OrderIDs     []int64 `json:"order_ids"`
	Title        string  `json:"title"`
	TaxNumber    string  `json:"tax_number"`
	Email        string  `json:"email"`
	ContactName  string  `json:"contact_name"`
	ContactPhone string  `json:"contact_phone"`
	RequestNote  string  `json:"request_note"`
}

func (h *InvoiceHandler) Apply(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req applyInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	view, err := h.invoiceService.Apply(c.Request.Context(), service.ApplyInvoiceInput{
		UserID:       subject.UserID,
		OrderIDs:     req.OrderIDs,
		Title:        req.Title,
		TaxNumber:    req.TaxNumber,
		Email:        req.Email,
		ContactName:  req.ContactName,
		ContactPhone: req.ContactPhone,
		RequestNote:  req.RequestNote,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *InvoiceHandler) ListMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.invoiceService.List(c.Request.Context(), service.InvoiceListParams{
		UserID:   subject.UserID,
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

func (h *InvoiceHandler) GetMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid invoice id")
		return
	}
	view, err := h.invoiceService.Get(c.Request.Context(), id, subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *InvoiceHandler) CancelMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid invoice id")
		return
	}
	view, err := h.invoiceService.Cancel(c.Request.Context(), id, subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *InvoiceHandler) DownloadGrant(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid invoice id")
		return
	}
	view, err := h.invoiceService.Get(c.Request.Context(), id, subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if view.Status != service.InvoiceStatusIssued || view.FileMediaID == nil {
		response.ErrorFrom(c, service.ErrInvoiceInvalidStatus)
		return
	}
	grant, err := h.mediaService.CreateDownloadGrant(c.Request.Context(), service.CreateDownloadGrantInput{
		AssetID:     *view.FileMediaID,
		ActorUserID: subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, grant)
}

func (h *InvoiceHandler) DownloadFile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid invoice id")
		return
	}
	view, err := h.invoiceService.Get(c.Request.Context(), id, subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if view.Status != service.InvoiceStatusIssued || view.FileMediaID == nil {
		response.ErrorFrom(c, service.ErrInvoiceInvalidStatus)
		return
	}
	asset, rc, err := h.mediaService.OpenForActor(c.Request.Context(), service.OpenForActorInput{
		AssetID:        *view.FileMediaID,
		ActorUserID:    subject.UserID,
		VerifiedAccess: true,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = rc.Close() }()
	filename := view.FileName
	if filename == "" {
		filename = asset.Filename
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", service.MediaContentDisposition(filename))
	c.DataFromReader(http.StatusOK, asset.Size, asset.MIME, rc, nil)
}

func (h *InvoiceHandler) EligibleProviders(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ids, err := h.configService.GetInvoiceEligibleInstanceIDs(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"provider_instance_ids": ids})
}
