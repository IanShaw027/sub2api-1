package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

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

// 占位以避免 import "fmt" / paymentproviderinstance 等未使用；后续任务会用到。
var _ = fmt.Errorf
var _ = invoice.FieldID
var _ = invoiceorder.FieldID
var _ = paymentorder.FieldID
var _ = paymentproviderinstance.FieldID
var _ infraerrors.Error
