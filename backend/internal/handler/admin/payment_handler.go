package admin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PaymentHandler handles admin payment management.
type PaymentHandler struct {
	paymentService *service.PaymentService
	configService  *service.PaymentConfigService
	invoiceService *service.InvoiceService
}

// NewPaymentHandler creates a new admin PaymentHandler.
func NewPaymentHandler(paymentService *service.PaymentService, configService *service.PaymentConfigService, invoiceService *service.InvoiceService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		configService:  configService,
		invoiceService: invoiceService,
	}
}

// --- Dashboard ---

// GetDashboard returns payment dashboard statistics.
// GET /api/v1/admin/payment/dashboard
func (h *PaymentHandler) GetDashboard(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	stats, err := h.paymentService.GetDashboardStats(c.Request.Context(), days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// --- Orders ---

// ListOrders returns a paginated list of all payment orders.
// GET /api/v1/admin/payment/orders
func (h *PaymentHandler) ListOrders(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	startTime, endTime, err := parseOrderDateRange(c.Query("start_date"), c.Query("end_date"), c.Query("timezone"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var userID int64
	if uid := c.Query("user_id"); uid != "" {
		if v, err := strconv.ParseInt(uid, 10, 64); err == nil {
			userID = v
		}
	}
	orders, total, err := h.paymentService.AdminListOrders(c.Request.Context(), userID, service.OrderListParams{
		Page:        page,
		PageSize:    pageSize,
		Status:      c.Query("status"),
		OrderType:   c.Query("order_type"),
		PaymentType: c.Query("payment_type"),
		Keyword:     c.Query("keyword"),
		StartTime:   startTime,
		EndTime:     endTime,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := sanitizeAdminPaymentOrdersForResponse(orders)
	items = enrichAdminOrdersWithInvoice(c.Request.Context(), h.invoiceService, items)
	response.Paginated(c, items, int64(total), page, pageSize)
}

func parseOrderDateRange(startDate, endDate, userTZ string) (*time.Time, *time.Time, error) {
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

// GetOrderDetail returns detailed information about a single order.
// GET /api/v1/admin/payment/orders/:id
func (h *PaymentHandler) GetOrderDetail(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	order, err := h.paymentService.GetOrderByID(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	auditLogs, _ := h.paymentService.GetOrderAuditLogs(c.Request.Context(), orderID)
	item := sanitizeAdminPaymentOrderForResponse(order)
	if item != nil {
		enriched := enrichAdminOrdersWithInvoice(c.Request.Context(), h.invoiceService, []AdminPaymentOrderResult{*item})
		if len(enriched) > 0 {
			item = &enriched[0]
		}
	}
	response.Success(c, gin.H{"order": item, "auditLogs": auditLogs})
}

// CancelOrder cancels a pending order (admin).
// POST /api/v1/admin/payment/orders/:id/cancel
func (h *PaymentHandler) CancelOrder(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	msg, err := h.paymentService.AdminCancelOrder(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": msg})
}

// RetryFulfillment retries fulfillment for a paid order.
// POST /api/v1/admin/payment/orders/:id/retry
func (h *PaymentHandler) RetryFulfillment(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.paymentService.RetryFulfillment(c.Request.Context(), orderID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "fulfillment retried"})
}

type AdminPaymentOrderResult struct {
	ID                    int64                  `json:"id"`
	UserID                int64                  `json:"user_id"`
	UserEmail             string                 `json:"user_email,omitempty"`
	UserName              string                 `json:"user_name,omitempty"`
	UserNotes             *string                `json:"user_notes,omitempty"`
	Amount                float64                `json:"amount"`
	PayAmount             float64                `json:"pay_amount"`
	FeeRate               float64                `json:"fee_rate"`
	Currency              string                 `json:"currency"`
	RechargeCode          string                 `json:"recharge_code,omitempty"`
	OutTradeNo            string                 `json:"out_trade_no"`
	PaymentType           string                 `json:"payment_type"`
	PaymentTradeNo        string                 `json:"payment_trade_no,omitempty"`
	PayURL                *string                `json:"pay_url,omitempty"`
	QrCode                *string                `json:"qr_code,omitempty"`
	QrCodeImg             *string                `json:"qr_code_img,omitempty"`
	OrderType             string                 `json:"order_type"`
	PlanID                *int64                 `json:"plan_id,omitempty"`
	SubscriptionGroupID   *int64                 `json:"subscription_group_id,omitempty"`
	SubscriptionDays      *int                   `json:"subscription_days,omitempty"`
	ProviderInstanceID    *string                `json:"provider_instance_id,omitempty"`
	ProviderKey           *string                `json:"provider_key,omitempty"`
	Status                string                 `json:"status"`
	RefundAmount          float64                `json:"refund_amount"`
	RefundReason          *string                `json:"refund_reason,omitempty"`
	RefundAt              *time.Time             `json:"refund_at,omitempty"`
	ForceRefund           bool                   `json:"force_refund,omitempty"`
	RefundRequestedAt     *time.Time             `json:"refund_requested_at,omitempty"`
	RefundRequestedAmount float64                `json:"refund_requested_amount"`
	RefundRequestReason   *string                `json:"refund_request_reason,omitempty"`
	RefundRequestedBy     *string                `json:"refund_requested_by,omitempty"`
	ExpiresAt             time.Time              `json:"expires_at"`
	PaidAt                *time.Time             `json:"paid_at,omitempty"`
	CompletedAt           *time.Time             `json:"completed_at,omitempty"`
	FailedAt              *time.Time             `json:"failed_at,omitempty"`
	FailedReason          *string                `json:"failed_reason,omitempty"`
	ClientIP              string                 `json:"client_ip,omitempty"`
	SrcHost               string                 `json:"src_host,omitempty"`
	SrcURL                *string                `json:"src_url,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	InvoiceStatus         string                 `json:"invoice_status,omitempty"`
	InvoiceID             *int64                 `json:"invoice_id,omitempty"`
	Edges                 map[string]interface{} `json:"edges,omitempty"`
}

func sanitizeAdminPaymentOrdersForResponse(orders []*dbent.PaymentOrder) []AdminPaymentOrderResult {
	if len(orders) == 0 {
		return nil
	}
	out := make([]AdminPaymentOrderResult, 0, len(orders))
	for _, order := range orders {
		if item := sanitizeAdminPaymentOrderForResponse(order); item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func sanitizeAdminPaymentOrderForResponse(order *dbent.PaymentOrder) *AdminPaymentOrderResult {
	if order == nil {
		return nil
	}
	return &AdminPaymentOrderResult{
		ID:                    order.ID,
		UserID:                order.UserID,
		UserEmail:             order.UserEmail,
		UserName:              order.UserName,
		UserNotes:             order.UserNotes,
		Amount:                order.Amount,
		PayAmount:             order.PayAmount,
		FeeRate:               order.FeeRate,
		Currency:              service.PaymentOrderCurrency(order),
		RechargeCode:          order.RechargeCode,
		OutTradeNo:            order.OutTradeNo,
		PaymentType:           order.PaymentType,
		PaymentTradeNo:        order.PaymentTradeNo,
		PayURL:                order.PayURL,
		QrCode:                order.QrCode,
		QrCodeImg:             order.QrCodeImg,
		OrderType:             order.OrderType,
		PlanID:                order.PlanID,
		SubscriptionGroupID:   order.SubscriptionGroupID,
		SubscriptionDays:      order.SubscriptionDays,
		ProviderInstanceID:    order.ProviderInstanceID,
		ProviderKey:           order.ProviderKey,
		Status:                order.Status,
		RefundAmount:          order.RefundAmount,
		RefundReason:          order.RefundReason,
		RefundAt:              order.RefundAt,
		ForceRefund:           order.ForceRefund,
		RefundRequestedAt:     order.RefundRequestedAt,
		RefundRequestedAmount: order.RefundRequestedAmount,
		RefundRequestReason:   order.RefundRequestReason,
		RefundRequestedBy:     order.RefundRequestedBy,
		ExpiresAt:             order.ExpiresAt,
		PaidAt:                order.PaidAt,
		CompletedAt:           order.CompletedAt,
		FailedAt:              order.FailedAt,
		FailedReason:          order.FailedReason,
		ClientIP:              order.ClientIP,
		SrcHost:               order.SrcHost,
		SrcURL:                order.SrcURL,
		CreatedAt:             order.CreatedAt,
		UpdatedAt:             order.UpdatedAt,
	}
}

func enrichAdminOrdersWithInvoice(ctx context.Context, invoiceSvc *service.InvoiceService, items []AdminPaymentOrderResult) []AdminPaymentOrderResult {
	if invoiceSvc == nil || len(items) == 0 {
		return items
	}
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	links, err := invoiceSvc.GetActiveLinksByOrderIDs(ctx, ids)
	if err != nil || len(links) == 0 {
		return items
	}
	for i := range items {
		if link, ok := links[items[i].ID]; ok {
			invoiceID := link.InvoiceID
			items[i].InvoiceID = &invoiceID
			items[i].InvoiceStatus = link.InvoiceStatus
		}
	}
	return items
}

// AdminProcessRefundRequest is the request body for admin refund processing.
type AdminProcessRefundRequest struct {
	Amount        float64 `json:"amount"`
	Reason        string  `json:"reason"`
	Force         bool    `json:"force"`
	DeductBalance bool    `json:"deduct_balance"`
}

// ProcessRefund processes a refund for an order (admin).
// POST /api/v1/admin/payment/orders/:id/refund
func (h *PaymentHandler) ProcessRefund(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req AdminProcessRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	plan, earlyResult, err := h.paymentService.PrepareRefund(c.Request.Context(), orderID, req.Amount, req.Reason, req.Force, req.DeductBalance)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if earlyResult != nil {
		response.Success(c, earlyResult)
		return
	}

	result, err := h.paymentService.ExecuteRefund(c.Request.Context(), plan)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GetRefundPreview returns refundable amount details for an order (admin).
// GET /api/v1/admin/payment/orders/:id/refund-preview
func (h *PaymentHandler) GetRefundPreview(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	preview, err := h.paymentService.GetAdminRefundPreview(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, preview)
}

// QueryAndFinalizeRefund queries the provider refund status and finalizes a pending refund.
// POST /api/v1/admin/payment/orders/:id/refund/query
func (h *PaymentHandler) QueryAndFinalizeRefund(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	result, err := h.paymentService.QueryAndFinalizeRefund(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ListInvoices returns a paginated list of invoice applications.
func (h *PaymentHandler) ListInvoices(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if h.invoiceService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("INVOICE_SERVICE_UNAVAILABLE", "invoice service unavailable"))
		return
	}
	items, total, err := h.invoiceService.List(c.Request.Context(), service.InvoiceListParams{
		Page:     page,
		PageSize: pageSize,
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, int64(total), page, pageSize)
}

// GetInvoiceDetail returns details for a single invoice application.
func (h *PaymentHandler) GetInvoiceDetail(c *gin.Context) {
	invoiceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if h.invoiceService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("INVOICE_SERVICE_UNAVAILABLE", "invoice service unavailable"))
		return
	}
	item, err := h.invoiceService.GetForAdmin(c.Request.Context(), invoiceID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// UploadInvoiceFile uploads the issued invoice file and marks the application as issued.
func (h *PaymentHandler) UploadInvoiceFile(c *gin.Context) {
	invoiceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if h.invoiceService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("INVOICE_SERVICE_UNAVAILABLE", "invoice service unavailable"))
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "invoice file is required")
		return
	}
	fileBytes, contentType, _, _, _, err := readAdminUploadedMedia(fileHeader, 64<<20)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.invoiceService.UploadFile(c.Request.Context(), invoiceID, service.InvoiceFileUploadInput{
		FileName:    fileHeader.Filename,
		ContentType: contentType,
		File:        fileBytes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// CancelInvoice cancels an APPLIED invoice on behalf of admin.
// POST /api/v1/admin/payment/invoices/:id/cancel
func (h *PaymentHandler) CancelInvoice(c *gin.Context) {
	invoiceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if h.invoiceService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("INVOICE_SERVICE_UNAVAILABLE", "invoice service unavailable"))
		return
	}
	inv, err := h.invoiceService.CancelByAdmin(c.Request.Context(), invoiceID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, inv)
}

// ResendInvoiceEmail re-sends the invoice issued email synchronously.
// POST /api/v1/admin/payment/invoices/:id/resend-email
func (h *PaymentHandler) ResendInvoiceEmail(c *gin.Context) {
	invoiceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if h.invoiceService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("INVOICE_SERVICE_UNAVAILABLE", "invoice service unavailable"))
		return
	}
	if err := h.invoiceService.ResendIssuedEmail(c.Request.Context(), invoiceID, "admin"); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "invoice email resent"})
}

// --- Subscription Plans ---

// ListPlans returns all subscription plans.
// GET /api/v1/admin/payment/plans
func (h *PaymentHandler) ListPlans(c *gin.Context) {
	plans, err := h.configService.ListPlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}

// CreatePlan creates a new subscription plan.
// POST /api/v1/admin/payment/plans
func (h *PaymentHandler) CreatePlan(c *gin.Context) {
	var req service.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.configService.CreatePlan(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, plan)
}

// UpdatePlan updates an existing subscription plan.
// PUT /api/v1/admin/payment/plans/:id
func (h *PaymentHandler) UpdatePlan(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.configService.UpdatePlan(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plan)
}

// DeletePlan deletes a subscription plan.
// DELETE /api/v1/admin/payment/plans/:id
func (h *PaymentHandler) DeletePlan(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.configService.DeletePlan(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

// --- Provider Instances ---

// ListProviders returns all payment provider instances.
// GET /api/v1/admin/payment/providers
func (h *PaymentHandler) ListProviders(c *gin.Context) {
	providers, err := h.configService.ListProviderInstancesWithConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, providers)
}

// CreateProvider creates a new payment provider instance.
// POST /api/v1/admin/payment/providers
func (h *PaymentHandler) CreateProvider(c *gin.Context) {
	var req service.CreateProviderInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	inst, err := h.configService.CreateProviderInstance(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Created(c, inst)
}

// UpdateProvider updates an existing payment provider instance.
// PUT /api/v1/admin/payment/providers/:id
func (h *PaymentHandler) UpdateProvider(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.UpdateProviderInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	inst, err := h.configService.UpdateProviderInstance(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Success(c, inst)
}

// DeleteProvider deletes a payment provider instance.
// DELETE /api/v1/admin/payment/providers/:id
func (h *PaymentHandler) DeleteProvider(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.configService.DeleteProviderInstance(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Success(c, gin.H{"message": "deleted"})
}

// parseIDParam parses an int64 path parameter.
// Returns the parsed ID and true on success; on failure it writes a BadRequest response and returns false.
func parseIDParam(c *gin.Context, paramName string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(paramName), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid "+paramName)
		return 0, false
	}
	return id, true
}

// --- Config ---

// GetConfig returns the payment configuration (admin view).
// GET /api/v1/admin/payment/config
func (h *PaymentHandler) GetConfig(c *gin.Context) {
	cfg, err := h.configService.GetPaymentConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfig updates the payment configuration.
// PUT /api/v1/admin/payment/config
func (h *PaymentHandler) UpdateConfig(c *gin.Context) {
	var req service.UpdatePaymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.configService.UpdatePaymentConfig(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "updated"})
}
