package service

import (
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SupportTicketCategoryConsult          = "consult"
	SupportTicketCategoryRefund           = "refund"
	SupportTicketCategoryConcurrencyApply = "concurrency_apply"
	SupportTicketCategoryRateApply        = "rate_apply"
	SupportTicketCategoryOther            = "other"

	SupportTicketStatusSubmitted    = "submitted"
	SupportTicketStatusProcessing   = "processing"
	SupportTicketStatusWaitingUser  = "waiting_user"
	SupportTicketStatusWaitingAdmin = "waiting_admin"
	SupportTicketStatusResolved     = "resolved"
	SupportTicketStatusClosed       = "closed"
	SupportTicketStatusWithdrawn    = "withdrawn"

	SupportTicketSenderRoleUser   = "user"
	SupportTicketSenderRoleAdmin  = "admin"
	SupportTicketSenderRoleSystem = "system"

	SupportTicketMessageTypeMessage = "message"
	SupportTicketMessageTypeSystem  = "system"
)

var (
	ErrTicketNotFound                = infraerrors.NotFound("TICKET_NOT_FOUND", "ticket not found")
	ErrTicketInvalidCategory         = infraerrors.BadRequest("TICKET_CATEGORY_INVALID", "ticket category is invalid")
	ErrTicketInvalidTitle            = infraerrors.BadRequest("TICKET_TITLE_INVALID", "ticket title is invalid")
	ErrTicketPayloadRequired         = infraerrors.BadRequest("TICKET_PAYLOAD_REQUIRED", "ticket form payload is required")
	ErrTicketPayloadInvalid          = infraerrors.BadRequest("TICKET_PAYLOAD_INVALID", "ticket form payload is invalid")
	ErrTicketMessageRequired         = infraerrors.BadRequest("TICKET_MESSAGE_REQUIRED", "ticket message is required")
	ErrTicketMessageTooLarge         = infraerrors.BadRequest("TICKET_MESSAGE_TOO_LARGE", "ticket message is too large")
	ErrTicketNotEditable             = infraerrors.BadRequest("TICKET_NOT_EDITABLE", "ticket is not editable in current status")
	ErrTicketCannotWithdraw          = infraerrors.BadRequest("TICKET_WITHDRAW_INVALID", "ticket cannot be withdrawn in current status")
	ErrTicketCannotClose             = infraerrors.BadRequest("TICKET_CLOSE_INVALID", "ticket cannot be closed in current status")
	ErrTicketInvalidStatus           = infraerrors.BadRequest("TICKET_STATUS_INVALID", "ticket status is invalid")
	ErrTicketReplyLocked             = infraerrors.BadRequest("TICKET_REPLY_LOCKED", "ticket cannot receive replies in current status")
	ErrTicketForbidden               = infraerrors.Forbidden("TICKET_FORBIDDEN", "ticket is not accessible")
	ErrTicketStatusLocked            = infraerrors.BadRequest("TICKET_STATUS_LOCKED", "ticket status can no longer be changed")
	ErrTicketStatusInvalidTransition = infraerrors.BadRequest("TICKET_STATUS_TRANSITION_INVALID", "ticket status transition is invalid")
	ErrTicketRevisionRequired        = infraerrors.BadRequest("TICKET_REVISION_REQUIRED", "ticket revision is required")
	ErrTicketRevisionConflict        = infraerrors.Conflict("TICKET_REVISION_CONFLICT", "ticket revision has changed")
	ErrTicketAttachmentInvalid       = infraerrors.BadRequest("TICKET_ATTACHMENT_INVALID", "ticket attachment is not a valid private ticket media asset")
	ErrTicketAttachmentLimit         = infraerrors.BadRequest("TICKET_ATTACHMENT_LIMIT", "ticket attachment count exceeds the limit")
	ErrTicketTemplateInvalid         = infraerrors.BadRequest("TICKET_TEMPLATE_INVALID", "ticket reply template is invalid")
)

type TicketMessageAttachment struct {
	MediaID     int64  `json:"media_id"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type SupportTicket struct {
	ID                 int64           `json:"id"`
	TicketNo           string          `json:"ticket_no"`
	UserID             int64           `json:"user_id"`
	UserName           string          `json:"user_name,omitempty"`
	UserEmail          string          `json:"user_email,omitempty"`
	Category           string          `json:"category"`
	Title              string          `json:"title"`
	Status             string          `json:"status"`
	CurrentFormPayload json.RawMessage `json:"current_form_payload"`
	CurrentRevisionNo  int             `json:"current_revision_no"`
	LatestMessageAt    time.Time       `json:"latest_message_at"`
	LastReplyRole      string          `json:"last_reply_role"`
	UnreadByUser       bool            `json:"unread_by_user"`
	UnreadByAdmin      bool            `json:"unread_by_admin"`
	SubmittedAt        *time.Time      `json:"submitted_at,omitempty"`
	ClosedAt           *time.Time      `json:"closed_at,omitempty"`
	WithdrawnAt        *time.Time      `json:"withdrawn_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type SupportTicketMessage struct {
	ID                   int64                     `json:"id"`
	TicketID             int64                     `json:"ticket_id"`
	SenderRole           string                    `json:"sender_role"`
	SenderUserID         *int64                    `json:"sender_user_id,omitempty"`
	SenderNameSnapshot   string                    `json:"sender_name_snapshot"`
	SenderAvatarSnapshot string                    `json:"sender_avatar_snapshot,omitempty"`
	MessageType          string                    `json:"message_type"`
	Content              string                    `json:"content"`
	Attachments          []TicketMessageAttachment `json:"attachments,omitempty"`
	CreatedAt            time.Time                 `json:"created_at"`
}

type SupportTicketRevision struct {
	ID          int64           `json:"id"`
	TicketID    int64           `json:"ticket_id"`
	RevisionNo  int             `json:"revision_no"`
	Title       string          `json:"title"`
	FormPayload json.RawMessage `json:"form_payload"`
	SubmittedBy *int64          `json:"submitted_by,omitempty"`
	SubmittedAt time.Time       `json:"submitted_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

type TicketReplyTemplate struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TicketRateGroupOption struct {
	GroupID            int64    `json:"group_id"`
	Name               string   `json:"name"`
	BaseRateMultiplier float64  `json:"base_rate_multiplier"`
	UserRateMultiplier *float64 `json:"user_rate_multiplier,omitempty"`
	EffectiveRate      float64  `json:"effective_rate"`
}

type SupportTicketListFilters struct {
	UserID     int64
	Status     string
	Category   string
	Search     string
	UnreadOnly bool
	Page       int
	PageSize   int
}

type CreateSupportTicketInput struct {
	UserID      int64
	Category    string
	Title       string
	FormPayload json.RawMessage
}

type UpdateSupportTicketInput struct {
	UserID             int64
	Title              string
	FormPayload        json.RawMessage
	ExpectedRevisionNo int
}

type CreateSupportTicketMessageInput struct {
	UserID      int64
	Content     string
	MediaIDs    []int64
	Attachments []TicketMessageAttachment
}

func NormalizeSupportTicketCategory(v string) string {
	switch strings.TrimSpace(v) {
	case SupportTicketCategoryConsult, SupportTicketCategoryRefund, SupportTicketCategoryConcurrencyApply, SupportTicketCategoryRateApply, SupportTicketCategoryOther:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func NormalizeSupportTicketStatus(v string) string {
	switch strings.TrimSpace(v) {
	case SupportTicketStatusSubmitted, SupportTicketStatusProcessing, SupportTicketStatusWaitingUser, SupportTicketStatusWaitingAdmin, SupportTicketStatusResolved, SupportTicketStatusClosed, SupportTicketStatusWithdrawn:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}
