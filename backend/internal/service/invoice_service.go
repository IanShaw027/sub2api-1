package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoice"
	"github.com/Wei-Shaw/sub2api/ent/invoiceorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	InvoiceStatusApplied   = "APPLIED"
	InvoiceStatusIssued    = "ISSUED"
	InvoiceStatusCancelled = "CANCELLED"

	MaxInvoiceOrders = 100
)

type InvoiceMediaService interface {
	Upload(ctx context.Context, input UploadMediaInput) (*MediaAsset, error)
	CreateDownloadURLForUser(ctx context.Context, requesterUserID, id int64) (*MediaDownloadURL, error)
	CreateDownloadURLForAdmin(ctx context.Context, id int64) (*MediaDownloadURL, error)
}

type CreateInvoiceRequest struct {
	OrderIDs     []int64
	Title        string
	TaxNumber    string
	Email        string
	ContactName  string
	ContactPhone string
	RequestNote  *string
}

type InvoiceFileUploadInput struct {
	FileName    string
	ContentType string
	File        []byte
}

type InvoiceListParams struct {
	UserID   *int64
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

type InvoiceDetail struct {
	ID            int64              `json:"id"`
	UserID        int64              `json:"user_id"`
	UserEmail     string             `json:"user_email"`
	Status        string             `json:"status"`
	InvoiceAmount float64            `json:"invoice_amount"`
	OrderCount    int                `json:"order_count"`
	Title         string             `json:"title"`
	TaxNumber     string             `json:"tax_number"`
	Email         string             `json:"email"`
	ContactName   string             `json:"contact_name"`
	ContactPhone  string             `json:"contact_phone"`
	RequestNote   *string            `json:"request_note,omitempty"`
	FileMediaID   *int64             `json:"file_media_id,omitempty"`
	FileName      string             `json:"file_name,omitempty"`
	FileMIMEType  string             `json:"file_mime_type,omitempty"`
	FileSizeBytes int64              `json:"file_size_bytes,omitempty"`
	AppliedAt     *time.Time         `json:"applied_at,omitempty"`
	CancelledAt   *time.Time         `json:"cancelled_at,omitempty"`
	IssuedAt      *time.Time         `json:"issued_at,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	Orders        []InvoiceOrderItem `json:"orders,omitempty"`
}

type InvoiceOrderItem struct {
	OrderID           int64     `json:"order_id"`
	OutTradeNo        string    `json:"out_trade_no"`
	PayAmountSnapshot float64   `json:"pay_amount_snapshot"`
	PaymentType       string    `json:"payment_type"`
	CreatedAt         time.Time `json:"created_at"`
}

type OrderInvoiceLink struct {
	InvoiceID     int64
	InvoiceStatus string
	HasFile       bool
}

type InvoiceService struct {
	entClient  *dbent.Client
	paymentSvc *PaymentService
	mediaSvc   InvoiceMediaService
	emailSvc   *NotificationEmailService
}

func NewInvoiceService(entClient *dbent.Client, paymentSvc *PaymentService, mediaSvc InvoiceMediaService) *InvoiceService {
	return &InvoiceService{
		entClient:  entClient,
		paymentSvc: paymentSvc,
		mediaSvc:   mediaSvc,
	}
}

// SetNotificationEmailService wires the email service after construction. Optional;
// if nil, invoice issued emails are silently skipped.
func (s *InvoiceService) SetNotificationEmailService(svc *NotificationEmailService) {
	if s != nil {
		s.emailSvc = svc
	}
}

// 以下方法在后续任务里逐个 TDD 实现：
//   Create / GetForUser / GetForAdmin / List / Cancel / CancelByAdmin
//   UploadFile / CreateDownloadURLForUser / CreateDownloadURLForAdmin
//   GetActiveLinksByOrderIDs / GetInvoiceEligibleProviderInstanceIDs

func (s *InvoiceService) writeAuditLog(ctx context.Context, orderID int64, action, operator string, detail map[string]any) {
	if s.paymentSvc != nil {
		s.paymentSvc.writeAuditLog(ctx, orderID, action, operator, detail)
	}
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
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

func invoiceDetailFromEnt(inv *dbent.Invoice, orders []*dbent.InvoiceOrder) *InvoiceDetail {
	if inv == nil {
		return nil
	}
	d := &InvoiceDetail{
		ID:            inv.ID,
		UserID:        inv.UserID,
		UserEmail:     inv.UserEmail,
		Status:        inv.Status,
		InvoiceAmount: inv.InvoiceAmount,
		OrderCount:    inv.OrderCount,
		Title:         inv.Title,
		TaxNumber:     inv.TaxNumber,
		Email:         inv.Email,
		ContactName:   inv.ContactName,
		ContactPhone:  inv.ContactPhone,
		RequestNote:   inv.RequestNote,
		FileMediaID:   inv.FileMediaID,
		FileName:      inv.FileName,
		FileMIMEType:  inv.FileMimeType,
		FileSizeBytes: inv.FileSizeBytes,
		AppliedAt:     inv.AppliedAt,
		CancelledAt:   inv.CancelledAt,
		IssuedAt:      inv.IssuedAt,
		CreatedAt:     inv.CreatedAt,
		UpdatedAt:     inv.UpdatedAt,
	}
	if orders != nil {
		d.Orders = make([]InvoiceOrderItem, 0, len(orders))
		for _, o := range orders {
			d.Orders = append(d.Orders, InvoiceOrderItem{
				OrderID:           o.OrderID,
				OutTradeNo:        o.OutTradeNo,
				PayAmountSnapshot: o.PayAmountSnapshot,
				PaymentType:       o.PaymentType,
				CreatedAt:         o.CreatedAt,
			})
		}
	}
	return d
}

func (s *InvoiceService) Create(ctx context.Context, userID int64, req CreateInvoiceRequest) (result *InvoiceDetail, err error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if len(req.OrderIDs) == 0 {
		return nil, infraerrors.BadRequest("INVOICE_ORDER_IDS_REQUIRED", "order_ids is required")
	}
	if len(req.OrderIDs) > MaxInvoiceOrders {
		return nil, infraerrors.BadRequest("INVOICE_BATCH_TOO_MANY", fmt.Sprintf("at most %d orders per invoice", MaxInvoiceOrders))
	}

	title := strings.TrimSpace(req.Title)
	taxNumber := strings.TrimSpace(req.TaxNumber)
	email := strings.TrimSpace(req.Email)
	if title == "" {
		return nil, infraerrors.BadRequest("INVOICE_TITLE_REQUIRED", "invoice title is required")
	}
	if taxNumber == "" {
		return nil, infraerrors.BadRequest("INVOICE_TAX_NUMBER_REQUIRED", "tax number is required")
	}
	if email == "" {
		return nil, infraerrors.BadRequest("INVOICE_EMAIL_REQUIRED", "invoice email is required")
	}

	uniqueIDs := dedupeInt64(req.OrderIDs)

	tx, txErr := s.entClient.Tx(ctx)
	if txErr != nil {
		return nil, fmt.Errorf("invoice tx begin: %w", txErr)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	orderQuery := tx.PaymentOrder.Query().
		Where(paymentorder.IDIn(uniqueIDs...))
	if s.entClient.Driver().Dialect() != dialect.SQLite {
		orderQuery = orderQuery.ForUpdate()
	}
	orders, err := orderQuery.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock orders: %w", err)
	}
	if len(orders) != len(uniqueIDs) {
		return nil, infraerrors.NotFound("NOT_FOUND", "one or more orders not found")
	}
	for _, o := range orders {
		if o.UserID != userID {
			return nil, infraerrors.Forbidden("FORBIDDEN", "order does not belong to user")
		}
		if o.Status != OrderStatusCompleted {
			return nil, infraerrors.BadRequest("INVOICE_ORDER_INELIGIBLE", fmt.Sprintf("order %d not completed", o.ID))
		}
		inst, instErr := s.paymentSvc.getOrderProviderInstance(ctx, o)
		if instErr != nil || inst == nil || !inst.InvoiceEnabled {
			return nil, infraerrors.Forbidden("INVOICE_DISABLED", fmt.Sprintf("order %d invoice disabled", o.ID))
		}
	}

	links, err := s.queryActiveLinksTx(ctx, tx, uniqueIDs)
	if err != nil {
		return nil, err
	}
	if len(links) > 0 {
		busy := make([]int64, 0, len(links))
		for id := range links {
			busy = append(busy, id)
		}
		return nil, infraerrors.Conflict("INVOICE_ALREADY_INVOICED", fmt.Sprintf("orders already invoiced: %v", busy))
	}

	now := time.Now()
	requestNote := trimOptionalString(req.RequestNote)
	totalAmount := 0.0
	for _, o := range orders {
		totalAmount += o.PayAmount
	}
	totalAmount = roundMoney(totalAmount)

	user, err := tx.User.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	invoiceCreate := tx.Invoice.Create().
		SetUserID(userID).
		SetUserEmail(user.Email).
		SetStatus(InvoiceStatusApplied).
		SetInvoiceAmount(totalAmount).
		SetOrderCount(len(orders)).
		SetTitle(title).
		SetTaxNumber(taxNumber).
		SetEmail(email).
		SetContactName(strings.TrimSpace(req.ContactName)).
		SetContactPhone(strings.TrimSpace(req.ContactPhone)).
		SetAppliedAt(now)
	if requestNote != nil {
		invoiceCreate = invoiceCreate.SetNillableRequestNote(requestNote)
	}
	inv, err := invoiceCreate.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	linkRows := make([]*dbent.InvoiceOrder, 0, len(orders))
	for _, o := range orders {
		row, ioErr := tx.InvoiceOrder.Create().
			SetInvoiceID(inv.ID).
			SetOrderID(o.ID).
			SetPayAmountSnapshot(roundMoney(o.PayAmount)).
			SetOutTradeNo(o.OutTradeNo).
			SetPaymentType(o.PaymentType).
			Save(ctx)
		if ioErr != nil {
			err = fmt.Errorf("create invoice_order: %w", ioErr)
			return nil, err
		}
		linkRows = append(linkRows, row)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = commitErr
		return nil, fmt.Errorf("invoice tx commit: %w", commitErr)
	}

	for _, o := range orders {
		s.writeAuditLog(ctx, o.ID, "INVOICE_APPLIED", fmt.Sprintf("user:%d", userID), map[string]any{
			"invoice_id": inv.ID,
			"order_ids":  uniqueIDs,
		})
	}

	return invoiceDetailFromEnt(inv, linkRows), nil
}

func (s *InvoiceService) queryActiveLinksTx(ctx context.Context, tx *dbent.Tx, orderIDs []int64) (map[int64]OrderInvoiceLink, error) {
	if len(orderIDs) == 0 {
		return map[int64]OrderInvoiceLink{}, nil
	}
	rows, err := tx.InvoiceOrder.Query().
		Where(invoiceorder.OrderIDIn(orderIDs...)).
		WithInvoice().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query invoice_orders: %w", err)
	}
	out := map[int64]OrderInvoiceLink{}
	for _, row := range rows {
		inv := row.Edges.Invoice
		if inv == nil {
			continue
		}
		if inv.Status != InvoiceStatusApplied && inv.Status != InvoiceStatusIssued {
			continue
		}
		out[row.OrderID] = OrderInvoiceLink{
			InvoiceID:     inv.ID,
			InvoiceStatus: inv.Status,
			HasFile:       inv.FileMediaID != nil,
		}
	}
	return out, nil
}

func dedupeInt64(in []int64) []int64 {
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (s *InvoiceService) Cancel(ctx context.Context, invoiceID, userID int64) (*InvoiceDetail, error) {
	return s.cancelInternal(ctx, invoiceID, &userID, fmt.Sprintf("user:%d", userID))
}

func (s *InvoiceService) CancelByAdmin(ctx context.Context, invoiceID int64) (*InvoiceDetail, error) {
	return s.cancelInternal(ctx, invoiceID, nil, "admin")
}

func (s *InvoiceService) cancelInternal(ctx context.Context, invoiceID int64, userID *int64, operator string) (result *InvoiceDetail, err error) {
	tx, txErr := s.entClient.Tx(ctx)
	if txErr != nil {
		return nil, fmt.Errorf("cancel tx begin: %w", txErr)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	invQuery := tx.Invoice.Query().Where(invoice.IDEQ(invoiceID))
	if s.entClient.Driver().Dialect() != dialect.SQLite {
		invQuery = invQuery.ForUpdate()
	}
	inv, err := invQuery.Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice not found")
		}
		return nil, fmt.Errorf("lock invoice: %w", err)
	}
	if userID != nil && inv.UserID != *userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission for this invoice")
	}
	if inv.Status != InvoiceStatusApplied {
		return nil, infraerrors.BadRequest("INVOICE_CANNOT_CANCEL", "invoice cannot be cancelled")
	}

	now := time.Now()
	inv, err = tx.Invoice.UpdateOneID(inv.ID).
		SetStatus(InvoiceStatusCancelled).
		SetCancelledAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update invoice cancel: %w", err)
	}

	links, err := tx.InvoiceOrder.Query().Where(invoiceorder.InvoiceIDEQ(inv.ID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load invoice_orders: %w", err)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = commitErr
		return nil, fmt.Errorf("cancel tx commit: %w", commitErr)
	}

	for _, l := range links {
		s.writeAuditLog(ctx, l.OrderID, "INVOICE_CANCELLED", operator, map[string]any{"invoice_id": inv.ID})
	}

	return invoiceDetailFromEnt(inv, links), nil
}

func (s *InvoiceService) UploadFile(ctx context.Context, invoiceID int64, input InvoiceFileUploadInput) (result *InvoiceDetail, err error) {
	if s.mediaSvc == nil {
		return nil, infraerrors.ServiceUnavailable("INVOICE_FILE_STORAGE_UNAVAILABLE", "invoice file storage is unavailable")
	}
	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" || len(input.File) == 0 {
		return nil, infraerrors.BadRequest("INVOICE_FILE_REQUIRED", "invoice file is required")
	}

	tx, txErr := s.entClient.Tx(ctx)
	if txErr != nil {
		return nil, fmt.Errorf("upload tx begin: %w", txErr)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	invQuery := tx.Invoice.Query().Where(invoice.IDEQ(invoiceID))
	if s.entClient.Driver().Dialect() != dialect.SQLite {
		invQuery = invQuery.ForUpdate()
	}
	inv, err := invQuery.Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice not found")
		}
		return nil, fmt.Errorf("lock invoice: %w", err)
	}
	if inv.Status != InvoiceStatusApplied {
		return nil, infraerrors.BadRequest("INVOICE_CANNOT_UPLOAD", "invoice not in APPLIED state")
	}

	ownerUserID := inv.UserID
	asset, err := s.mediaSvc.Upload(ctx, UploadMediaInput{
		BizType:     "invoice",
		BizID:       fmt.Sprintf("invoice-%d", inv.ID),
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
	inv, err = tx.Invoice.UpdateOneID(inv.ID).
		SetStatus(InvoiceStatusIssued).
		SetFileMediaID(asset.ID).
		SetFileName(asset.OriginalFileName).
		SetFileMimeType(asset.MIMEType).
		SetFileSizeBytes(asset.SizeBytes).
		SetIssuedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark invoice issued: %w", err)
	}

	links, err := tx.InvoiceOrder.Query().Where(invoiceorder.InvoiceIDEQ(inv.ID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load invoice_orders: %w", err)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = commitErr
		return nil, fmt.Errorf("upload tx commit: %w", commitErr)
	}

	for _, l := range links {
		s.writeAuditLog(ctx, l.OrderID, "INVOICE_ISSUED", "admin", map[string]any{
			"invoice_id": inv.ID, "media_id": asset.ID, "file_name": asset.OriginalFileName,
		})
	}

	s.dispatchInvoiceIssuedEmail(inv.ID)

	return invoiceDetailFromEnt(inv, links), nil
}

func (s *InvoiceService) GetForUser(ctx context.Context, invoiceID, userID int64) (*InvoiceDetail, error) {
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice not found")
		}
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	if inv.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission for this invoice")
	}
	links, err := s.entClient.InvoiceOrder.Query().Where(invoiceorder.InvoiceIDEQ(inv.ID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load invoice_orders: %w", err)
	}
	return invoiceDetailFromEnt(inv, links), nil
}

func (s *InvoiceService) GetForAdmin(ctx context.Context, invoiceID int64) (*InvoiceDetail, error) {
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice not found")
		}
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	links, err := s.entClient.InvoiceOrder.Query().Where(invoiceorder.InvoiceIDEQ(inv.ID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load invoice_orders: %w", err)
	}
	return invoiceDetailFromEnt(inv, links), nil
}

func (s *InvoiceService) List(ctx context.Context, params InvoiceListParams) ([]*InvoiceDetail, int, error) {
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

	q := s.entClient.Invoice.Query()
	if params.UserID != nil {
		q = q.Where(invoice.UserIDEQ(*params.UserID))
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		q = q.Where(invoice.StatusEQ(status))
	}
	if keyword := strings.TrimSpace(params.Keyword); keyword != "" {
		matchedRows, err := s.entClient.InvoiceOrder.Query().
			Where(invoiceorder.OutTradeNoContainsFold(keyword)).
			All(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("keyword join lookup: %w", err)
		}
		matchedInvoiceIDs := make([]int64, 0, len(matchedRows))
		for _, r := range matchedRows {
			matchedInvoiceIDs = append(matchedInvoiceIDs, r.InvoiceID)
		}
		q = q.Where(invoice.Or(
			invoice.TitleContainsFold(keyword),
			invoice.EmailContainsFold(keyword),
			invoice.UserEmailContainsFold(keyword),
			invoice.TaxNumberContainsFold(keyword),
			invoice.IDIn(matchedInvoiceIDs...),
		))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count invoices: %w", err)
	}
	rows, err := q.
		Order(dbent.Desc(invoice.FieldCreatedAt)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoices: %w", err)
	}
	out := make([]*InvoiceDetail, 0, len(rows))
	for _, r := range rows {
		out = append(out, invoiceDetailFromEnt(r, nil))
	}
	return out, total, nil
}

func (s *InvoiceService) GetActiveLinksByOrderIDs(ctx context.Context, orderIDs []int64) (map[int64]OrderInvoiceLink, error) {
	if len(orderIDs) == 0 {
		return map[int64]OrderInvoiceLink{}, nil
	}
	rows, err := s.entClient.InvoiceOrder.Query().
		Where(invoiceorder.OrderIDIn(orderIDs...)).
		WithInvoice().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query invoice_orders: %w", err)
	}
	out := map[int64]OrderInvoiceLink{}
	for _, r := range rows {
		inv := r.Edges.Invoice
		if inv == nil {
			continue
		}
		if inv.Status != InvoiceStatusApplied && inv.Status != InvoiceStatusIssued {
			continue
		}
		out[r.OrderID] = OrderInvoiceLink{
			InvoiceID:     inv.ID,
			InvoiceStatus: inv.Status,
			HasFile:       inv.FileMediaID != nil,
		}
	}
	return out, nil
}

func (s *InvoiceService) CreateDownloadURLForUser(ctx context.Context, invoiceID, userID int64) (*MediaDownloadURL, error) {
	if s.mediaSvc == nil {
		return nil, infraerrors.ServiceUnavailable("INVOICE_FILE_STORAGE_UNAVAILABLE", "invoice file storage is unavailable")
	}
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice not found")
		}
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	if inv.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission for this invoice")
	}
	if inv.Status != InvoiceStatusIssued || inv.FileMediaID == nil {
		return nil, infraerrors.BadRequest("INVOICE_NOT_ISSUED", "invoice has not been issued")
	}
	return s.mediaSvc.CreateDownloadURLForUser(ctx, userID, *inv.FileMediaID)
}

func (s *InvoiceService) CreateDownloadURLForAdmin(ctx context.Context, invoiceID int64) (*MediaDownloadURL, error) {
	if s.mediaSvc == nil {
		return nil, infraerrors.ServiceUnavailable("INVOICE_FILE_STORAGE_UNAVAILABLE", "invoice file storage is unavailable")
	}
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice not found")
		}
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	if inv.Status != InvoiceStatusIssued || inv.FileMediaID == nil {
		return nil, infraerrors.BadRequest("INVOICE_NOT_ISSUED", "invoice has not been issued")
	}
	return s.mediaSvc.CreateDownloadURLForAdmin(ctx, *inv.FileMediaID)
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

// dispatchInvoiceIssuedEmail kicks off an async email send after a successful UploadFile.
// Failure is logged but never propagated — the invoice is already ISSUED and the user
// can always download from the in-app list/detail page.
func (s *InvoiceService) dispatchInvoiceIssuedEmail(invoiceID int64) {
	if s == nil || s.emailSvc == nil || invoiceID <= 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), emailSendTimeout)
		defer cancel()
		if err := s.sendInvoiceIssuedEmail(ctx, invoiceID, "auto"); err != nil {
			slog.Warn("invoice issued notification email failed", "invoice_id", invoiceID, "err", err.Error())
		}
	}()
}

// ResendIssuedEmail synchronously re-sends the issued-invoice email.
// Used by the admin "重发邮件" button. Returns BadRequest if invoice is not ISSUED.
func (s *InvoiceService) ResendIssuedEmail(ctx context.Context, invoiceID int64, operator string) error {
	if s == nil || s.emailSvc == nil {
		return infraerrors.ServiceUnavailable("INVOICE_EMAIL_DISABLED", "invoice email service is not configured")
	}
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice not found")
		}
		return fmt.Errorf("get invoice: %w", err)
	}
	if inv.Status != InvoiceStatusIssued || inv.FileMediaID == nil {
		return infraerrors.BadRequest("INVOICE_NOT_ISSUED", "invoice has not been issued")
	}
	op := strings.TrimSpace(operator)
	if op == "" {
		op = "manual"
	}
	return s.sendInvoiceIssuedEmail(ctx, invoiceID, op)
}

func (s *InvoiceService) sendInvoiceIssuedEmail(ctx context.Context, invoiceID int64, operator string) error {
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("get invoice: %w", err)
	}
	if inv.Status != InvoiceStatusIssued || inv.FileMediaID == nil {
		return infraerrors.BadRequest("INVOICE_NOT_ISSUED", "invoice has not been issued")
	}
	recipient := strings.TrimSpace(inv.Email)
	if recipient == "" {
		return infraerrors.BadRequest("INVOICE_EMAIL_REQUIRED", "invoice has no recipient email")
	}

	links, err := s.entClient.InvoiceOrder.Query().Where(invoiceorder.InvoiceIDEQ(inv.ID)).All(ctx)
	if err != nil {
		return fmt.Errorf("load invoice_orders: %w", err)
	}

	downloadURL := ""
	if s.mediaSvc != nil {
		dl, dlErr := s.mediaSvc.CreateDownloadURLForUser(ctx, inv.UserID, *inv.FileMediaID)
		if dlErr != nil {
			return fmt.Errorf("create download url: %w", dlErr)
		}
		if dl != nil {
			downloadURL = dl.URL
		}
	}
	if downloadURL == "" {
		return infraerrors.ServiceUnavailable("INVOICE_FILE_STORAGE_UNAVAILABLE", "invoice download url unavailable")
	}

	recipientName := strings.TrimSpace(inv.ContactName)
	if recipientName == "" {
		recipientName = recipient
	}

	variables := map[string]string{
		"invoice_id":           strconv.FormatInt(inv.ID, 10),
		"invoice_title":        inv.Title,
		"tax_number":           inv.TaxNumber,
		"invoice_amount":       fmt.Sprintf("%.2f", inv.InvoiceAmount),
		"order_count":          strconv.Itoa(inv.OrderCount),
		"invoice_download_url": downloadURL,
		"invoice_file_name":    inv.FileName,
	}
	rawHTML := map[string]string{
		"order_list_html": renderInvoiceOrderListHTML(links),
	}

	sendErr := s.emailSvc.Send(ctx, NotificationEmailSendInput{
		Event:            NotificationEmailEventInvoiceIssued,
		RecipientEmail:   recipient,
		RecipientName:    recipientName,
		UserID:           inv.UserID,
		SourceType:       "invoice",
		SourceID:         strconv.FormatInt(inv.ID, 10),
		Variables:        variables,
		RawHTMLVariables: rawHTML,
	})
	if sendErr != nil {
		s.writeAuditLog(ctx, 0, "INVOICE_EMAIL_FAILED", operator, map[string]any{
			"invoice_id": inv.ID, "recipient": recipient, "err": sendErr.Error(),
		})
		return sendErr
	}
	s.writeAuditLog(ctx, 0, "INVOICE_EMAIL_SENT", operator, map[string]any{
		"invoice_id": inv.ID, "recipient": recipient,
	})
	return nil
}

func renderInvoiceOrderListHTML(rows []*dbent.InvoiceOrder) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<table style="width:100%;border-collapse:collapse;margin:8px 0;font-size:13px;">`)
	b.WriteString(`<thead><tr style="background:#fafafa;color:#71717a;">`)
	b.WriteString(`<th style="text-align:left;padding:6px 8px;border-bottom:1px solid #e4e4e7;">Order No.</th>`)
	b.WriteString(`<th style="text-align:right;padding:6px 8px;border-bottom:1px solid #e4e4e7;">Amount</th>`)
	b.WriteString(`</tr></thead><tbody>`)
	for _, r := range rows {
		b.WriteString(`<tr><td style="padding:6px 8px;border-bottom:1px solid #f4f4f5;font-family:ui-monospace,monospace;">`)
		b.WriteString(html.EscapeString(r.OutTradeNo))
		b.WriteString(`</td><td style="padding:6px 8px;text-align:right;border-bottom:1px solid #f4f4f5;">¥`)
		b.WriteString(fmt.Sprintf("%.2f", r.PayAmountSnapshot))
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
