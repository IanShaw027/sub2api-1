package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	SupportTicketCategoryConsult          = "consult"
	SupportTicketCategoryRefund           = "refund"
	SupportTicketCategoryConcurrencyApply = "concurrency_apply"
	SupportTicketCategoryRateApply        = "rate_apply"
	SupportTicketCategoryOther            = "other"
)

const (
	SupportTicketStatusSubmitted    = "submitted"
	SupportTicketStatusProcessing   = "processing"
	SupportTicketStatusWaitingUser  = "waiting_user"
	SupportTicketStatusWaitingAdmin = "waiting_admin"
	SupportTicketStatusResolved     = "resolved"
	SupportTicketStatusClosed       = "closed"
	SupportTicketStatusWithdrawn    = "withdrawn"
)

const (
	SupportTicketSenderRoleUser   = "user"
	SupportTicketSenderRoleAdmin  = "admin"
	SupportTicketSenderRoleSystem = "system"
)

const (
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
)

type SupportTicket struct {
	ID                 int64
	TicketNo           string
	UserID             int64
	UserName           string
	UserEmail          string
	UserAvatarURL      string
	Category           string
	Title              string
	Status             string
	CurrentFormPayload json.RawMessage
	CurrentRevisionNo  int
	LatestMessageAt    time.Time
	LastReplyRole      string
	UnreadByUser       bool
	UnreadByAdmin      bool
	SubmittedAt        *time.Time
	ClosedAt           *time.Time
	WithdrawnAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type SupportTicketMessage struct {
	ID                   int64
	TicketID             int64
	SenderRole           string
	SenderUserID         *int64
	SenderNameSnapshot   string
	SenderAvatarSnapshot string
	MessageType          string
	Content              string
	CreatedAt            time.Time
}

type SupportTicketRevision struct {
	ID          int64
	TicketID    int64
	RevisionNo  int
	Title       string
	FormPayload json.RawMessage
	SubmittedBy *int64
	SubmittedAt time.Time
	CreatedAt   time.Time
}

type SupportTicketListFilters struct {
	Status    string
	Category  string
	Search    string
	UserQuery string
	StartTime *time.Time
	EndTime   *time.Time
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
	UserID  int64
	Content string
}

type AdminSupportTicketStatusUpdateInput struct {
	AdminUserID int64
	Status      string
}

type SupportTicketRepository interface {
	CreateSubmitted(ctx context.Context, ticket *SupportTicket, revision *SupportTicketRevision, systemMessage *SupportTicketMessage) error
	GetByID(ctx context.Context, ticketID int64) (*SupportTicket, error)
	ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error)
	ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error)
	ListMessages(ctx context.Context, ticketID int64) ([]SupportTicketMessage, error)
	UpdateAfterUserWithdraw(ctx context.Context, ticketID int64, withdrawnAt time.Time, systemMessage *SupportTicketMessage) error
	UpdateEditableContent(ctx context.Context, ticketID int64, title string, formPayload json.RawMessage, expectedRevisionNo int) error
	Resubmit(ctx context.Context, ticketID int64, ticket *SupportTicket, revision *SupportTicketRevision, systemMessage *SupportTicketMessage, expectedRevisionNo int) error
	CloseByUser(ctx context.Context, ticketID int64, closedAt time.Time, systemMessage *SupportTicketMessage) error
	AddReply(ctx context.Context, ticketID int64, message *SupportTicketMessage, lastReplyRole string, unreadByUser, unreadByAdmin bool, nextStatus string) error
	UpdateStatusByAdmin(ctx context.Context, ticketID int64, status string, closedAt *time.Time, systemMessage *SupportTicketMessage) error
	MarkReadByUser(ctx context.Context, ticketID int64) error
	MarkReadByAdmin(ctx context.Context, ticketID int64) error
}

func NormalizeSupportTicketCategory(v string) string {
	switch strings.TrimSpace(v) {
	case SupportTicketCategoryConsult:
		return SupportTicketCategoryConsult
	case SupportTicketCategoryRefund:
		return SupportTicketCategoryRefund
	case SupportTicketCategoryConcurrencyApply:
		return SupportTicketCategoryConcurrencyApply
	case SupportTicketCategoryRateApply:
		return SupportTicketCategoryRateApply
	case SupportTicketCategoryOther:
		return SupportTicketCategoryOther
	default:
		return ""
	}
}

func NormalizeSupportTicketStatus(v string) string {
	switch strings.TrimSpace(v) {
	case SupportTicketStatusSubmitted:
		return SupportTicketStatusSubmitted
	case SupportTicketStatusProcessing:
		return SupportTicketStatusProcessing
	case SupportTicketStatusWaitingUser:
		return SupportTicketStatusWaitingUser
	case SupportTicketStatusWaitingAdmin:
		return SupportTicketStatusWaitingAdmin
	case SupportTicketStatusResolved:
		return SupportTicketStatusResolved
	case SupportTicketStatusClosed:
		return SupportTicketStatusClosed
	case SupportTicketStatusWithdrawn:
		return SupportTicketStatusWithdrawn
	default:
		return ""
	}
}
