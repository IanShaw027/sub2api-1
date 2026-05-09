package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoiceapplication"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	InvoiceStatusApplied   = "APPLIED"
	InvoiceStatusIssued    = "ISSUED"
	InvoiceStatusCancelled = "CANCELLED"
)

type InvoiceMediaService interface {
	Upload(ctx context.Context, input UploadMediaInput) (*MediaAsset, error)
	CreateDownloadURLForUser(ctx context.Context, requesterUserID, id int64) (*MediaDownloadURL, error)
	CreateDownloadURLForAdmin(ctx context.Context, id int64) (*MediaDownloadURL, error)
}

type ApplyInvoiceRequest struct {
	Title        string  `json:"title"`
	TaxNumber    string  `json:"tax_number"`
	Email        string  `json:"email"`
	ContactName  string  `json:"contact_name"`
	ContactPhone string  `json:"contact_phone"`
	RequestNote  *string `json:"request_note,omitempty"`
}

type InvoiceFileUploadInput struct {
	FileName    string
	ContentType string
	File        []byte
}

type InvoiceApplicationDetail struct {
	ID                int64      `json:"id"`
	OrderID           int64      `json:"order_id"`
	UserID            int64      `json:"user_id"`
	UserEmail         string     `json:"user_email"`
	OrderOutTradeNo   string     `json:"order_out_trade_no"`
	PaymentType       string     `json:"payment_type"`
	ProviderInstanceID string    `json:"provider_instance_id"`
	ProviderKey       string     `json:"provider_key"`
	Status            string     `json:"status"`
	InvoiceAmount     float64    `json:"invoice_amount"`
	Title             string     `json:"title"`
	TaxNumber         string     `json:"tax_number"`
	Email             string     `json:"email"`
	ContactName       string     `json:"contact_name"`
	ContactPhone      string     `json:"contact_phone"`
	RequestNote       *string    `json:"request_note,omitempty"`
	FileMediaID       *int64     `json:"file_media_id,omitempty"`
	FileName          string     `json:"file_name,omitempty"`
	FileMIMEType      string     `json:"file_mime_type,omitempty"`
	FileSizeBytes     int64      `json:"file_size_bytes,omitempty"`
	AppliedAt         *time.Time `json:"applied_at,omitempty"`
	CancelledAt       *time.Time `json:"cancelled_at,omitempty"`
	IssuedAt          *time.Time `json:"issued_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type InvoiceListParams struct {
	Page     int
	PageSize int
	Status   string
	Keyword  string
}

type InvoiceService struct {
	entClient  *dbent.Client
	paymentSvc *PaymentService
	mediaSvc   InvoiceMediaService
}

func NewInvoiceService(entClient *dbent.Client, paymentSvc *PaymentService, mediaSvc InvoiceMediaService) *InvoiceService {
	return &InvoiceService{
		entClient:  entClient,
		paymentSvc: paymentSvc,
		mediaSvc:   mediaSvc,
	}
}

func (s *InvoiceService) Apply(ctx context.Context, orderID, userID int64, req ApplyInvoiceRequest) (*InvoiceApplicationDetail, error) {
	order, err := s.validateOrderForInvoice(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	taxNumber := strings.TrimSpace(req.TaxNumber)
	email := strings.TrimSpace(req.Email)
	contactName := strings.TrimSpace(req.ContactName)
	contactPhone := strings.TrimSpace(req.ContactPhone)
	requestNote := trimOptionalString(req.RequestNote)

	if title == "" {
		return nil, infraerrors.BadRequest("INVOICE_TITLE_REQUIRED", "invoice title is required")
	}
	if taxNumber == "" {
		return nil, infraerrors.BadRequest("INVOICE_TAX_NUMBER_REQUIRED", "tax number is required")
	}
	if email == "" {
		return nil, infraerrors.BadRequest("INVOICE_EMAIL_REQUIRED", "invoice email is required")
	}

	now := time.Now()
	app, err := s.entClient.InvoiceApplication.Query().
		Where(invoiceapplication.OrderIDEQ(orderID)).
		Only(ctx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, fmt.Errorf("query invoice application: %w", err)
	}

	invoiceAmount := roundMoney(order.PayAmount)
	if app == nil || dbent.IsNotFound(err) {
		app, err = s.entClient.InvoiceApplication.Create().
			SetOrderID(order.ID).
			SetUserID(order.UserID).
			SetUserEmail(order.UserEmail).
			SetOrderOutTradeNo(order.OutTradeNo).
			SetPaymentType(order.PaymentType).
			SetProviderInstanceID(strings.TrimSpace(psStringValue(order.ProviderInstanceID))).
			SetProviderKey(strings.TrimSpace(psStringValue(order.ProviderKey))).
			SetInvoiceStatus(InvoiceStatusApplied).
			SetInvoiceAmount(invoiceAmount).
			SetInvoiceTitle(title).
			SetTaxNumber(taxNumber).
			SetEmail(email).
			SetContactName(contactName).
			SetContactPhone(contactPhone).
			SetNillableRequestNote(requestNote).
			SetAppliedAt(now).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("create invoice application: %w", err)
		}
	} else {
		switch app.InvoiceStatus {
		case InvoiceStatusApplied:
			return nil, infraerrors.Conflict("INVOICE_ALREADY_APPLIED", "invoice has already been applied")
		case InvoiceStatusIssued:
			return nil, infraerrors.Conflict("INVOICE_ALREADY_ISSUED", "invoice has already been issued")
		case InvoiceStatusCancelled:
			app, err = s.entClient.InvoiceApplication.UpdateOneID(app.ID).
				SetInvoiceStatus(InvoiceStatusApplied).
				SetInvoiceAmount(invoiceAmount).
				SetInvoiceTitle(title).
				SetTaxNumber(taxNumber).
				SetEmail(email).
				SetContactName(contactName).
				SetContactPhone(contactPhone).
				SetNillableRequestNote(requestNote).
				SetAppliedAt(now).
				ClearCancelledAt().
				ClearIssuedAt().
				ClearFileMediaID().
				SetFileName("").
				SetFileMimeType("").
				SetFileSizeBytes(0).
				Save(ctx)
			if err != nil {
				return nil, fmt.Errorf("reapply invoice application: %w", err)
			}
		default:
			return nil, infraerrors.Conflict("INVOICE_STATUS_INVALID", "invoice application status is invalid")
		}
	}

	if _, err := s.entClient.PaymentOrder.UpdateOneID(order.ID).
		SetInvoiceStatus(InvoiceStatusApplied).
		ClearInvoiceFileMediaID().
		Save(ctx); err != nil {
		return nil, fmt.Errorf("update payment order invoice status: %w", err)
	}
	s.writeAuditLog(ctx, order.ID, "INVOICE_APPLIED", fmt.Sprintf("user:%d", userID), map[string]any{
		"title": title,
		"email": email,
	})

	return invoiceApplicationDetailFromEnt(app), nil
}

func (s *InvoiceService) GetByOrderForUser(ctx context.Context, orderID, userID int64) (*InvoiceApplicationDetail, error) {
	order, err := s.getOwnedOrder(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	app, err := s.entClient.InvoiceApplication.Query().
		Where(invoiceapplication.OrderIDEQ(order.ID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
		}
		return nil, fmt.Errorf("query invoice application: %w", err)
	}
	return invoiceApplicationDetailFromEnt(app), nil
}

func (s *InvoiceService) CancelByOrderForUser(ctx context.Context, orderID, userID int64) (*InvoiceApplicationDetail, error) {
	order, err := s.getOwnedOrder(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	app, err := s.entClient.InvoiceApplication.Query().
		Where(invoiceapplication.OrderIDEQ(order.ID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
		}
		return nil, fmt.Errorf("query invoice application: %w", err)
	}
	if app.InvoiceStatus != InvoiceStatusApplied {
		return nil, infraerrors.BadRequest("INVOICE_CANNOT_CANCEL", "invoice application cannot be cancelled")
	}

	now := time.Now()
	app, err = s.entClient.InvoiceApplication.UpdateOneID(app.ID).
		SetInvoiceStatus(InvoiceStatusCancelled).
		SetCancelledAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("cancel invoice application: %w", err)
	}
	if _, err := s.entClient.PaymentOrder.UpdateOneID(order.ID).
		SetInvoiceStatus(InvoiceStatusCancelled).
		ClearInvoiceFileMediaID().
		Save(ctx); err != nil {
		return nil, fmt.Errorf("update payment order invoice cancellation: %w", err)
	}
	s.writeAuditLog(ctx, order.ID, "INVOICE_CANCELLED", fmt.Sprintf("user:%d", userID), nil)

	return invoiceApplicationDetailFromEnt(app), nil
}

func (s *InvoiceService) CreateDownloadURLForUser(ctx context.Context, orderID, userID int64) (*MediaDownloadURL, error) {
	if s.mediaSvc == nil {
		return nil, infraerrors.ServiceUnavailable("INVOICE_FILE_STORAGE_UNAVAILABLE", "invoice file storage is unavailable")
	}
	order, err := s.getOwnedOrder(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	app, err := s.entClient.InvoiceApplication.Query().
		Where(invoiceapplication.OrderIDEQ(order.ID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
		}
		return nil, fmt.Errorf("query invoice application: %w", err)
	}
	if app.InvoiceStatus != InvoiceStatusIssued || app.FileMediaID == nil {
		return nil, infraerrors.BadRequest("INVOICE_NOT_ISSUED", "invoice has not been issued")
	}
	return s.mediaSvc.CreateDownloadURLForUser(ctx, userID, *app.FileMediaID)
}

func (s *InvoiceService) List(ctx context.Context, params InvoiceListParams) ([]*InvoiceApplicationDetail, int, error) {
	if s == nil || s.entClient == nil {
		return nil, 0, fmt.Errorf("invoice service is not initialized")
	}

	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := s.entClient.InvoiceApplication.Query()
	if status := strings.TrimSpace(params.Status); status != "" {
		query = query.Where(invoiceapplication.InvoiceStatusEQ(status))
	}
	if keyword := strings.TrimSpace(params.Keyword); keyword != "" {
		query = query.Where(
			invoiceapplication.Or(
				invoiceapplication.InvoiceTitleContainsFold(keyword),
				invoiceapplication.EmailContainsFold(keyword),
				invoiceapplication.UserEmailContainsFold(keyword),
				invoiceapplication.OrderOutTradeNoContainsFold(keyword),
				invoiceapplication.TaxNumberContainsFold(keyword),
			),
		)
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count invoice applications: %w", err)
	}
	rows, err := query.
		Order(dbent.Desc(invoiceapplication.FieldCreatedAt)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoice applications: %w", err)
	}

	items := make([]*InvoiceApplicationDetail, 0, len(rows))
	for _, row := range rows {
		items = append(items, invoiceApplicationDetailFromEnt(row))
	}
	return items, total, nil
}

func (s *InvoiceService) GetByIDForAdmin(ctx context.Context, invoiceID int64) (*InvoiceApplicationDetail, error) {
	app, err := s.entClient.InvoiceApplication.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
		}
		return nil, fmt.Errorf("get invoice application: %w", err)
	}
	return invoiceApplicationDetailFromEnt(app), nil
}

func (s *InvoiceService) UploadFile(ctx context.Context, invoiceID int64, input InvoiceFileUploadInput) (*InvoiceApplicationDetail, error) {
	if s.mediaSvc == nil {
		return nil, infraerrors.ServiceUnavailable("INVOICE_FILE_STORAGE_UNAVAILABLE", "invoice file storage is unavailable")
	}
	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" || len(input.File) == 0 {
		return nil, infraerrors.BadRequest("INVOICE_FILE_REQUIRED", "invoice file is required")
	}

	app, err := s.entClient.InvoiceApplication.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
		}
		return nil, fmt.Errorf("get invoice application: %w", err)
	}
	if app.InvoiceStatus != InvoiceStatusApplied {
		return nil, infraerrors.BadRequest("INVOICE_UPLOAD_INVALID_STATUS", "invoice application is not pending upload")
	}

	ownerUserID := app.UserID
	asset, err := s.mediaSvc.Upload(ctx, UploadMediaInput{
		BizType:     "invoice",
		BizID:       fmt.Sprintf("order-%d", app.OrderID),
		Visibility:  MediaVisibilityPrivate,
		OwnerUserID: &ownerUserID,
		FileName:    fileName,
		ContentType: strings.TrimSpace(input.ContentType),
		SizeBytes:   int64(len(input.File)),
		File:        input.File,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	app, err = s.entClient.InvoiceApplication.UpdateOneID(app.ID).
		SetInvoiceStatus(InvoiceStatusIssued).
		SetFileMediaID(asset.ID).
		SetFileName(asset.OriginalFileName).
		SetFileMimeType(asset.MIMEType).
		SetFileSizeBytes(asset.SizeBytes).
		SetIssuedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update issued invoice application: %w", err)
	}
	if _, err := s.entClient.PaymentOrder.UpdateOneID(app.OrderID).
		SetInvoiceStatus(InvoiceStatusIssued).
		SetInvoiceFileMediaID(asset.ID).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("update payment order invoice issue state: %w", err)
	}
	s.writeAuditLog(ctx, app.OrderID, "INVOICE_ISSUED", "admin", map[string]any{
		"file_name": asset.OriginalFileName,
		"media_id":  asset.ID,
	})

	return invoiceApplicationDetailFromEnt(app), nil
}

func (s *InvoiceService) GetInvoiceEligibleProviderInstanceIDs(ctx context.Context) ([]string, error) {
	instances, err := s.entClient.PaymentProviderInstance.Query().
		Where(paymentproviderinstance.InvoiceEnabledEQ(true)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query invoice-enabled provider instances: %w", err)
	}
	ids := make([]string, 0, len(instances))
	for _, inst := range instances {
		ids = append(ids, fmt.Sprintf("%d", inst.ID))
	}
	return ids, nil
}

func (s *InvoiceService) getOwnedOrder(ctx context.Context, orderID, userID int64) (*dbent.PaymentOrder, error) {
	order, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
		}
		return nil, fmt.Errorf("get payment order: %w", err)
	}
	if order.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	return order, nil
}

func (s *InvoiceService) validateOrderForInvoice(ctx context.Context, orderID, userID int64) (*dbent.PaymentOrder, error) {
	order, err := s.getOwnedOrder(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	if order.Status != OrderStatusCompleted {
		return nil, infraerrors.BadRequest("INVOICE_INVALID_ORDER_STATUS", "only completed orders can apply for invoice")
	}
	inst, err := s.resolveInvoiceProviderInstance(ctx, order)
	if err != nil || inst == nil {
		return nil, infraerrors.Forbidden("INVOICE_DISABLED", "invoice is not available for this order")
	}
	if !inst.InvoiceEnabled {
		return nil, infraerrors.Forbidden("INVOICE_DISABLED", "invoice is not enabled for this provider")
	}
	return order, nil
}

func (s *InvoiceService) resolveInvoiceProviderInstance(ctx context.Context, order *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s.paymentSvc != nil {
		return s.paymentSvc.getOrderProviderInstance(ctx, order)
	}
	return nil, nil
}

func (s *InvoiceService) writeAuditLog(ctx context.Context, orderID int64, action, operator string, detail map[string]any) {
	if s.paymentSvc != nil {
		s.paymentSvc.writeAuditLog(ctx, orderID, action, operator, detail)
	}
}

func invoiceApplicationDetailFromEnt(app *dbent.InvoiceApplication) *InvoiceApplicationDetail {
	if app == nil {
		return nil
	}
	return &InvoiceApplicationDetail{
		ID:                 app.ID,
		OrderID:            app.OrderID,
		UserID:             app.UserID,
		UserEmail:          app.UserEmail,
		OrderOutTradeNo:    app.OrderOutTradeNo,
		PaymentType:        app.PaymentType,
		ProviderInstanceID: app.ProviderInstanceID,
		ProviderKey:        app.ProviderKey,
		Status:             app.InvoiceStatus,
		InvoiceAmount:      app.InvoiceAmount,
		Title:              app.InvoiceTitle,
		TaxNumber:          app.TaxNumber,
		Email:              app.Email,
		ContactName:        app.ContactName,
		ContactPhone:       app.ContactPhone,
		RequestNote:        app.RequestNote,
		FileMediaID:        app.FileMediaID,
		FileName:           app.FileName,
		FileMIMEType:       app.FileMimeType,
		FileSizeBytes:      app.FileSizeBytes,
		AppliedAt:          app.AppliedAt,
		CancelledAt:        app.CancelledAt,
		IssuedAt:           app.IssuedAt,
		CreatedAt:          app.CreatedAt,
		UpdatedAt:          app.UpdatedAt,
	}
}

func trimOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
