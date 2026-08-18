package service

import (
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	InvoiceStatusApplied   = "APPLIED"
	InvoiceStatusIssued    = "ISSUED"
	InvoiceStatusCancelled = "CANCELLED"

	MaxInvoiceOrders = 100
)

var (
	ErrInvoiceNotFound               = infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice not found")
	ErrInvoiceForbidden              = infraerrors.Forbidden("INVOICE_FORBIDDEN", "not allowed to access this invoice")
	ErrInvoiceInvalidStatus          = infraerrors.BadRequest("INVOICE_INVALID_STATUS", "invoice status does not allow this action")
	ErrInvoiceOrderNotEligible       = infraerrors.BadRequest("INVOICE_ORDER_NOT_ELIGIBLE", "order is not eligible for invoicing")
	ErrInvoiceChannelDisabled        = infraerrors.BadRequest("INVOICE_CHANNEL_DISABLED", "invoicing is not enabled for this payment channel")
	ErrInvoiceOrderOccupied          = infraerrors.Conflict("INVOICE_ORDER_OCCUPIED", "order is already covered by an active invoice")
	ErrInvoiceTooManyOrders          = infraerrors.BadRequest("INVOICE_TOO_MANY_ORDERS", "at most 100 orders can be merged into one invoice")
	ErrInvoiceNoOrders               = infraerrors.BadRequest("INVOICE_NO_ORDERS", "at least one order is required")
	ErrInvoiceIssuedRefundBlock      = infraerrors.Forbidden("INVOICE_ISSUED", "issued invoices block refund unless an administrator forces it")
	ErrInvoiceMixedCurrency          = infraerrors.BadRequest("INVOICE_MIXED_CURRENCY", "orders in one invoice must share the same currency")
	ErrInvoiceTitleTooLong           = infraerrors.BadRequest("INVOICE_TITLE_TOO_LONG", "invoice title is too long")
	ErrInvoiceFieldTooLong           = infraerrors.BadRequest("INVOICE_FIELD_TOO_LONG", "invoice field exceeds the allowed length")
	ErrInvoiceFileRequired           = infraerrors.BadRequest("INVOICE_FILE_REQUIRED", "invoice file is required to issue")
	ErrInvoiceFileInvalid            = infraerrors.BadRequest("INVOICE_FILE_INVALID", "invoice file is not a valid private invoice media asset")
	ErrInvoiceTitleRequired          = infraerrors.BadRequest("INVOICE_TITLE_REQUIRED", "invoice title is required")
	ErrInvoiceEmailRequired          = infraerrors.BadRequest("INVOICE_EMAIL_REQUIRED", "invoice email is required")
	ErrInvoiceTaxNumberRequired      = infraerrors.BadRequest("INVOICE_TAX_NUMBER_REQUIRED", "invoice tax number is required")
	ErrInvoiceDownloadURLUnavailable = infraerrors.ServiceUnavailable("INVOICE_FILE_STORAGE_UNAVAILABLE", "invoice download url unavailable")
)

// ApplyInvoiceInput is a user request to invoice one or more completed orders.
type ApplyInvoiceInput struct {
	UserID       int64
	UserEmail    string
	OrderIDs     []int64
	Title        string
	TaxNumber    string
	Email        string
	ContactName  string
	ContactPhone string
	RequestNote  string
}

// InvoiceOrderItem is one order covered by an invoice.
type InvoiceOrderItem struct {
	OrderID           int64     `json:"order_id"`
	PayAmountSnapshot float64   `json:"pay_amount_snapshot"`
	Currency          string    `json:"currency,omitempty"`
	OutTradeNo        string    `json:"out_trade_no"`
	PaymentType       string    `json:"payment_type"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
}

// InvoiceView is the API representation of an invoice.
type InvoiceView struct {
	ID            int64              `json:"id"`
	UserID        int64              `json:"user_id"`
	UserEmail     string             `json:"user_email"`
	Status        string             `json:"status"`
	UnreadByAdmin bool               `json:"unread_by_admin"`
	InvoiceAmount float64            `json:"invoice_amount"`
	Currency      string             `json:"currency,omitempty"`
	OrderCount    int                `json:"order_count"`
	Title         string             `json:"title"`
	TaxNumber     string             `json:"tax_number"`
	Email         string             `json:"email"`
	ContactName   string             `json:"contact_name"`
	ContactPhone  string             `json:"contact_phone"`
	RequestNote   string             `json:"request_note"`
	FileMediaID   *int64             `json:"file_media_id,omitempty"`
	FileName      string             `json:"file_name,omitempty"`
	FileMimeType  string             `json:"file_mime_type,omitempty"`
	FileSizeBytes int64              `json:"file_size_bytes,omitempty"`
	HasFile       bool               `json:"has_file"`
	AppliedAt     time.Time          `json:"applied_at"`
	CancelledAt   *time.Time         `json:"cancelled_at,omitempty"`
	IssuedAt      *time.Time         `json:"issued_at,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	Orders        []InvoiceOrderItem `json:"orders,omitempty"`
}

// InvoiceListParams filters invoice lists.
type InvoiceListParams struct {
	UserID   int64
	Status   string
	Keyword  string
	Page     int
	PageSize int
}
