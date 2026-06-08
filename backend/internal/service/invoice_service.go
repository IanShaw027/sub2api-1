package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoiceorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
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
}

func NewInvoiceService(entClient *dbent.Client, paymentSvc *PaymentService, mediaSvc InvoiceMediaService) *InvoiceService {
	return &InvoiceService{
		entClient:  entClient,
		paymentSvc: paymentSvc,
		mediaSvc:   mediaSvc,
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

func (s *InvoiceService) Create(ctx context.Context, userID int64, req CreateInvoiceRequest) (*InvoiceDetail, error) {
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

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("invoice tx begin: %w", err)
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
