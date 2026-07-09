package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type ticketRepository struct {
	db *sql.DB
}

const createSubmittedMaxTicketNoAttempts = 3
const supportTicketReplyDedupWindow = 10 * time.Second

func NewTicketRepository(db *sql.DB) service.SupportTicketRepository {
	return &ticketRepository{db: db}
}

func (r *ticketRepository) CreateSubmitted(ctx context.Context, ticket *service.SupportTicket, revision *service.SupportTicketRevision, systemMessage *service.SupportTicketMessage) error {
	for attempt := 1; attempt <= createSubmittedMaxTicketNoAttempts; attempt++ {
		if err := r.createSubmittedOnce(ctx, ticket, revision, systemMessage); err != nil {
			if isSupportTicketNoUniqueViolation(err) && attempt < createSubmittedMaxTicketNoAttempts {
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("create support ticket exhausted retries")
}

func (r *ticketRepository) createSubmittedOnce(ctx context.Context, ticket *service.SupportTicket, revision *service.SupportTicketRevision, systemMessage *service.SupportTicketMessage) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	ticketNo := generateTicketNo(time.Now())
	err = tx.QueryRowContext(ctx, `
		INSERT INTO support_tickets (
			ticket_no, user_id, category, title, status, current_form_payload,
			current_revision_no, latest_message_at, last_reply_role, unread_by_user,
			unread_by_admin, submitted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at
	`, ticketNo, ticket.UserID, ticket.Category, ticket.Title, ticket.Status, []byte(ticket.CurrentFormPayload), ticket.CurrentRevisionNo, ticket.LatestMessageAt, ticket.LastReplyRole, ticket.UnreadByUser, ticket.UnreadByAdmin, ticket.SubmittedAt).Scan(&ticket.ID, &ticket.CreatedAt, &ticket.UpdatedAt)
	if err != nil {
		return err
	}
	ticket.TicketNo = ticketNo

	revision.TicketID = ticket.ID
	err = tx.QueryRowContext(ctx, `
		INSERT INTO support_ticket_revisions (
			ticket_id, revision_no, title, form_payload, submitted_by, submitted_at
		) VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, created_at
	`, revision.TicketID, revision.RevisionNo, revision.Title, []byte(revision.FormPayload), revision.SubmittedBy, revision.SubmittedAt).Scan(&revision.ID, &revision.CreatedAt)
	if err != nil {
		return err
	}

	systemMessage.TicketID = ticket.ID
	err = tx.QueryRowContext(ctx, `
		INSERT INTO support_ticket_messages (
			ticket_id, sender_role, sender_user_id, sender_name_snapshot, sender_avatar_snapshot,
			message_type, content, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id
	`, ticket.ID, systemMessage.SenderRole, systemMessage.SenderUserID, systemMessage.SenderNameSnapshot, systemMessage.SenderAvatarSnapshot, systemMessage.MessageType, systemMessage.Content, systemMessage.CreatedAt).Scan(&systemMessage.ID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *ticketRepository) GetByID(ctx context.Context, ticketID int64) (*service.SupportTicket, error) {
	row := r.db.QueryRowContext(ctx, baseTicketSelect()+` WHERE t.id = $1`, ticketID)
	item, err := scanSupportTicket(row)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrTicketNotFound, nil)
	}
	return item, nil
}

func (r *ticketRepository) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	where := []string{"t.user_id = $1"}
	args := []any{userID}
	where, args = appendTicketFilters(where, args, filters)
	return r.list(ctx, params, where, args)
}

func (r *ticketRepository) ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	where, args := appendTicketFilters([]string{"1=1"}, nil, filters)
	return r.list(ctx, params, where, args)
}

func (r *ticketRepository) list(ctx context.Context, params pagination.PaginationParams, where []string, args []any) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	whereSQL := strings.Join(where, " AND ")
	countQuery := `SELECT COUNT(*) FROM support_tickets t LEFT JOIN users u ON u.id = t.user_id WHERE ` + whereSQL
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	orderBy := ticketListOrder(params)
	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, baseTicketSelect()+` WHERE `+whereSQL+` ORDER BY `+orderBy+` LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.SupportTicket, 0)
	for rows.Next() {
		item, scanErr := scanSupportTicket(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return items, paginationResultFromTotal(total, params), nil
}

func (r *ticketRepository) ListMessages(ctx context.Context, ticketID int64) ([]service.SupportTicketMessage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, ticket_id, sender_role, sender_user_id, sender_name_snapshot, sender_avatar_snapshot, message_type, content, COALESCE(attachments, '[]'::jsonb), created_at
		FROM support_ticket_messages
		WHERE ticket_id = $1
		ORDER BY created_at ASC, id ASC
	`, ticketID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.SupportTicketMessage, 0)
	for rows.Next() {
		item, scanErr := scanSupportTicketMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *ticketRepository) UpdateAfterUserWithdraw(ctx context.Context, ticketID int64, withdrawnAt time.Time, systemMessage *service.SupportTicketMessage) error {
	return r.withTicketUpdateTx(ctx, ticketID, func(tx *sql.Tx, locked *lockedTicketState) error {
		if locked.status != service.SupportTicketStatusSubmitted &&
			locked.status != service.SupportTicketStatusProcessing &&
			locked.status != service.SupportTicketStatusWaitingAdmin {
			return service.ErrTicketCannotWithdraw
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE support_tickets
			SET status = $2, withdrawn_at = $3, latest_message_at = $3, last_reply_role = $4,
				unread_by_user = FALSE, unread_by_admin = TRUE, closed_at = NULL, updated_at = NOW()
			WHERE id = $1
		`, ticketID, service.SupportTicketStatusWithdrawn, withdrawnAt, service.SupportTicketSenderRoleSystem); err != nil {
			return err
		}
		return insertTicketMessage(ctx, tx, ticketID, systemMessage)
	})
}

func (r *ticketRepository) UpdateEditableContent(ctx context.Context, ticketID int64, title string, formPayload json.RawMessage, expectedRevisionNo int) error {
	return r.withTicketUpdateTx(ctx, ticketID, func(tx *sql.Tx, locked *lockedTicketState) error {
		if locked.status != service.SupportTicketStatusWithdrawn {
			return service.ErrTicketNotEditable
		}
		if locked.currentRevisionNo != expectedRevisionNo {
			return service.ErrTicketRevisionConflict
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE support_tickets
			SET title = $2, current_form_payload = $3, updated_at = NOW()
			WHERE id = $1
		`, ticketID, title, []byte(formPayload))
		return err
	})
}

func (r *ticketRepository) Resubmit(ctx context.Context, ticketID int64, ticket *service.SupportTicket, revision *service.SupportTicketRevision, systemMessage *service.SupportTicketMessage, expectedRevisionNo int) error {
	return r.withTicketUpdateTx(ctx, ticketID, func(tx *sql.Tx, locked *lockedTicketState) error {
		if locked.status != service.SupportTicketStatusWithdrawn {
			return service.ErrTicketNotEditable
		}
		if locked.currentRevisionNo != expectedRevisionNo {
			return service.ErrTicketRevisionConflict
		}
		nextRevision := locked.currentRevisionNo + 1
		ticket.CurrentRevisionNo = nextRevision
		revision.RevisionNo = nextRevision
		if _, err := tx.ExecContext(ctx, `
			UPDATE support_tickets
			SET title = $2, status = $3, current_form_payload = $4, current_revision_no = $5,
				latest_message_at = $6, last_reply_role = $7, unread_by_user = $8, unread_by_admin = $9,
				submitted_at = $10, withdrawn_at = NULL, closed_at = NULL, updated_at = NOW()
			WHERE id = $1
		`, ticketID, ticket.Title, ticket.Status, []byte(ticket.CurrentFormPayload), ticket.CurrentRevisionNo, ticket.LatestMessageAt, ticket.LastReplyRole, ticket.UnreadByUser, ticket.UnreadByAdmin, ticket.SubmittedAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO support_ticket_revisions (ticket_id, revision_no, title, form_payload, submitted_by, submitted_at)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, ticketID, revision.RevisionNo, revision.Title, []byte(revision.FormPayload), revision.SubmittedBy, revision.SubmittedAt); err != nil {
			return err
		}
		return insertTicketMessage(ctx, tx, ticketID, systemMessage)
	})
}

func (r *ticketRepository) CloseByUser(ctx context.Context, ticketID int64, closedAt time.Time, systemMessage *service.SupportTicketMessage) error {
	return r.withTicketUpdateTx(ctx, ticketID, func(tx *sql.Tx, locked *lockedTicketState) error {
		if locked.status == service.SupportTicketStatusClosed || locked.status == service.SupportTicketStatusWithdrawn {
			return service.ErrTicketCannotClose
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE support_tickets
			SET status = $2, closed_at = $3, latest_message_at = $3, last_reply_role = $4,
				unread_by_user = FALSE, unread_by_admin = TRUE, withdrawn_at = NULL, updated_at = NOW()
			WHERE id = $1
		`, ticketID, service.SupportTicketStatusClosed, closedAt, service.SupportTicketSenderRoleSystem); err != nil {
			return err
		}
		return insertTicketMessage(ctx, tx, ticketID, systemMessage)
	})
}

func (r *ticketRepository) AddReply(ctx context.Context, ticketID int64, message *service.SupportTicketMessage, lastReplyRole string, unreadByUser, unreadByAdmin bool, nextStatus string) error {
	return r.withTicketUpdateTx(ctx, ticketID, func(tx *sql.Tx, locked *lockedTicketState) error {
		if locked.status == service.SupportTicketStatusResolved ||
			locked.status == service.SupportTicketStatusClosed ||
			locked.status == service.SupportTicketStatusWithdrawn {
			return service.ErrTicketReplyLocked
		}
		duplicate, err := r.isRecentDuplicateReply(ctx, tx, ticketID, message)
		if err != nil {
			return err
		}
		if duplicate {
			return nil
		}
		if err := insertTicketMessage(ctx, tx, ticketID, message); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE support_tickets
			SET latest_message_at = $2, last_reply_role = $3, unread_by_user = $4, unread_by_admin = $5,
				status = COALESCE(NULLIF($6, ''), status), updated_at = NOW()
			WHERE id = $1
		`, ticketID, message.CreatedAt, lastReplyRole, unreadByUser, unreadByAdmin, nextStatus)
		return err
	})
}

func (r *ticketRepository) isRecentDuplicateReply(ctx context.Context, tx *sql.Tx, ticketID int64, message *service.SupportTicketMessage) (bool, error) {
	if tx == nil || message == nil {
		return false, nil
	}
	var (
		senderRole   string
		senderUserID sql.NullInt64
		messageType  string
		content      string
		attachments  []byte
		createdAt    time.Time
	)
	err := tx.QueryRowContext(ctx, `
		SELECT sender_role, sender_user_id, message_type, content, COALESCE(attachments, '[]'::jsonb), created_at
		FROM support_ticket_messages
		WHERE ticket_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, ticketID).Scan(&senderRole, &senderUserID, &messageType, &content, &attachments, &createdAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	delta := message.CreatedAt.Sub(createdAt)
	if delta < 0 || delta > supportTicketReplyDedupWindow {
		return false, nil
	}
	if senderRole != message.SenderRole || messageType != message.MessageType || content != message.Content {
		return false, nil
	}
	if !equalNullableInt64(senderUserID, message.SenderUserID) {
		return false, nil
	}
	return equalTicketMessageAttachmentsJSON(attachments, message.Attachments), nil
}

func equalNullableInt64(left sql.NullInt64, right *int64) bool {
	if right == nil {
		return !left.Valid
	}
	return left.Valid && left.Int64 == *right
}

func equalTicketMessageAttachmentsJSON(raw []byte, attachments []service.TicketMessageAttachment) bool {
	if len(raw) == 0 {
		raw = []byte("[]")
	}
	var existing []service.TicketMessageAttachment
	if err := json.Unmarshal(raw, &existing); err != nil {
		return false
	}
	if len(existing) == 0 && len(attachments) == 0 {
		return true
	}
	return reflect.DeepEqual(existing, attachments)
}

func (r *ticketRepository) UpdateStatusByAdmin(ctx context.Context, ticketID int64, status string, closedAt *time.Time, systemMessage *service.SupportTicketMessage) error {
	return r.withTicketUpdateTx(ctx, ticketID, func(tx *sql.Tx, locked *lockedTicketState) error {
		if locked.status == service.SupportTicketStatusClosed && status != service.SupportTicketStatusClosed {
			return service.ErrTicketStatusLocked
		}
		if locked.status == service.SupportTicketStatusResolved && status != service.SupportTicketStatusResolved && status != service.SupportTicketStatusClosed {
			return service.ErrTicketStatusLocked
		}
		if locked.status == service.SupportTicketStatusWithdrawn {
			return service.ErrTicketStatusLocked
		}
		if !isAdminManualStatusTransitionAllowed(locked.status, locked.lastReplyRole, status) {
			return service.ErrTicketStatusInvalidTransition
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE support_tickets
			SET status = $2, closed_at = $3, latest_message_at = $4, last_reply_role = $5,
				unread_by_user = TRUE, unread_by_admin = FALSE, withdrawn_at = NULL, updated_at = NOW()
			WHERE id = $1
		`, ticketID, status, closedAt, systemMessage.CreatedAt, service.SupportTicketSenderRoleSystem); err != nil {
			return err
		}
		return insertTicketMessage(ctx, tx, ticketID, systemMessage)
	})
}

func (r *ticketRepository) MarkReadByUser(ctx context.Context, ticketID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE support_tickets SET unread_by_user = FALSE, updated_at = updated_at WHERE id = $1`, ticketID)
	return err
}

func (r *ticketRepository) MarkReadByAdmin(ctx context.Context, ticketID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE support_tickets SET unread_by_admin = FALSE, updated_at = updated_at WHERE id = $1`, ticketID)
	return err
}

type lockedTicketState struct {
	status            string
	currentRevisionNo int
	lastReplyRole     string
}

func (r *ticketRepository) withTicketUpdateTx(ctx context.Context, ticketID int64, fn func(tx *sql.Tx, locked *lockedTicketState) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	locked := &lockedTicketState{}
	if err := tx.QueryRowContext(ctx, `
		SELECT status, current_revision_no, last_reply_role
			FROM support_tickets
			WHERE id = $1
			FOR UPDATE
		`, ticketID).Scan(&locked.status, &locked.currentRevisionNo, &locked.lastReplyRole); err != nil {
		if err == sql.ErrNoRows {
			return service.ErrTicketNotFound
		}
		return err
	}
	if err := fn(tx, locked); err != nil {
		return err
	}
	return tx.Commit()
}

func isAdminManualStatusTransitionAllowed(currentStatus, lastReplyRole, nextStatus string) bool {
	if currentStatus == nextStatus {
		return true
	}
	if nextStatus == service.SupportTicketStatusWaitingUser || nextStatus == service.SupportTicketStatusResolved {
		return lastReplyRole == service.SupportTicketSenderRoleAdmin
	}
	return true
}

func insertTicketMessage(ctx context.Context, tx *sql.Tx, ticketID int64, message *service.SupportTicketMessage) error {
	attachmentsJSON := []byte("[]")
	if len(message.Attachments) > 0 {
		var err error
		attachmentsJSON, err = json.Marshal(message.Attachments)
		if err != nil {
			return fmt.Errorf("marshal attachments: %w", err)
		}
	}
	return tx.QueryRowContext(ctx, `
		INSERT INTO support_ticket_messages (
			ticket_id, sender_role, sender_user_id, sender_name_snapshot, sender_avatar_snapshot,
			message_type, content, attachments, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id
	`, ticketID, message.SenderRole, message.SenderUserID, message.SenderNameSnapshot, message.SenderAvatarSnapshot, message.MessageType, message.Content, attachmentsJSON, message.CreatedAt).Scan(&message.ID)
}

func baseTicketSelect() string {
	return `
		SELECT
			t.id, t.ticket_no, t.user_id, COALESCE(NULLIF(u.username, ''), u.email) AS user_name, u.email, COALESCE(ua.url, '') AS user_avatar_url,
			t.category, t.title, t.status, t.current_form_payload, t.current_revision_no, t.latest_message_at, t.last_reply_role,
			t.unread_by_user, t.unread_by_admin, t.submitted_at, t.closed_at, t.withdrawn_at, t.created_at, t.updated_at
		FROM support_tickets t
		LEFT JOIN users u ON u.id = t.user_id
		LEFT JOIN user_avatars ua ON ua.user_id = t.user_id
	`
}

func appendTicketFilters(where []string, args []any, filters service.SupportTicketListFilters) ([]string, []any) {
	if filters.Status != "" {
		args = append(args, filters.Status)
		where = append(where, `t.status = $`+fmt.Sprint(len(args)))
	}
	if filters.Category != "" {
		args = append(args, filters.Category)
		where = append(where, `t.category = $`+fmt.Sprint(len(args)))
	}
	if search := strings.TrimSpace(filters.Search); search != "" {
		args = append(args, "%"+search+"%")
		p := fmt.Sprint(len(args))
		where = append(where, `(t.title ILIKE $`+p+` OR t.ticket_no ILIKE $`+p+` OR COALESCE(u.username, '') ILIKE $`+p+` OR COALESCE(u.email, '') ILIKE $`+p+`)`)
	}
	if userQuery := strings.TrimSpace(filters.UserQuery); userQuery != "" {
		args = append(args, "%"+userQuery+"%")
		p := fmt.Sprint(len(args))
		where = append(where, `(COALESCE(u.username, '') ILIKE $`+p+` OR COALESCE(u.email, '') ILIKE $`+p+`)`)
	}
	if filters.StartTime != nil {
		args = append(args, *filters.StartTime)
		where = append(where, `t.created_at >= $`+fmt.Sprint(len(args)))
	}
	if filters.EndTime != nil {
		args = append(args, *filters.EndTime)
		where = append(where, `t.created_at < $`+fmt.Sprint(len(args)))
	}
	return where, args
}

func ticketListOrder(params pagination.PaginationParams) string {
	switch strings.ToLower(strings.TrimSpace(params.SortBy)) {
	case "updated_at":
		if params.NormalizedSortOrder(pagination.SortOrderDesc) == pagination.SortOrderAsc {
			return "t.updated_at ASC, t.id ASC"
		}
		return "t.updated_at DESC, t.id DESC"
	case "created_at":
		if params.NormalizedSortOrder(pagination.SortOrderDesc) == pagination.SortOrderAsc {
			return "t.created_at ASC, t.id ASC"
		}
	}
	return "t.created_at DESC, t.id DESC"
}

type ticketRowScanner interface {
	Scan(dest ...any) error
}

func scanSupportTicket(row ticketRowScanner) (*service.SupportTicket, error) {
	item := &service.SupportTicket{}
	var payload []byte
	var submittedAt, closedAt, withdrawnAt sql.NullTime
	var userAvatar sql.NullString
	if err := row.Scan(
		&item.ID,
		&item.TicketNo,
		&item.UserID,
		&item.UserName,
		&item.UserEmail,
		&userAvatar,
		&item.Category,
		&item.Title,
		&item.Status,
		&payload,
		&item.CurrentRevisionNo,
		&item.LatestMessageAt,
		&item.LastReplyRole,
		&item.UnreadByUser,
		&item.UnreadByAdmin,
		&submittedAt,
		&closedAt,
		&withdrawnAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.CurrentFormPayload = payload
	if userAvatar.Valid {
		item.UserAvatarURL = userAvatar.String
	}
	if submittedAt.Valid {
		t := submittedAt.Time
		item.SubmittedAt = &t
	}
	if closedAt.Valid {
		t := closedAt.Time
		item.ClosedAt = &t
	}
	if withdrawnAt.Valid {
		t := withdrawnAt.Time
		item.WithdrawnAt = &t
	}
	return item, nil
}

func scanSupportTicketMessage(row ticketRowScanner) (*service.SupportTicketMessage, error) {
	item := &service.SupportTicketMessage{}
	var senderUserID sql.NullInt64
	var avatar sql.NullString
	var attachmentsRaw []byte
	if err := row.Scan(&item.ID, &item.TicketID, &item.SenderRole, &senderUserID, &item.SenderNameSnapshot, &avatar, &item.MessageType, &item.Content, &attachmentsRaw, &item.CreatedAt); err != nil {
		return nil, err
	}
	if senderUserID.Valid {
		v := senderUserID.Int64
		item.SenderUserID = &v
	}
	if avatar.Valid {
		item.SenderAvatarSnapshot = avatar.String
	}
	if len(attachmentsRaw) > 0 {
		_ = json.Unmarshal(attachmentsRaw, &item.Attachments)
	}
	return item, nil
}

func generateTicketNo(now time.Time) string {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err == nil {
		return fmt.Sprintf("TK%s%s", now.Format("20060102150405"), hex.EncodeToString(suffix[:]))
	}
	return fmt.Sprintf("TK%s%016x", now.Format("20060102150405"), uint64(now.UnixNano()))
}

func isSupportTicketNoUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr != nil {
		if string(pqErr.Code) != "23505" {
			return false
		}
		constraint := strings.ToLower(strings.TrimSpace(pqErr.Constraint))
		if constraint != "" {
			return constraint == "support_tickets_ticket_no_key"
		}
		msg := strings.ToLower(pqErr.Message)
		detail := strings.ToLower(pqErr.Detail)
		return strings.Contains(msg, "ticket_no") || strings.Contains(detail, "ticket_no")
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "support_tickets.ticket_no")
}
