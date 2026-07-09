package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTicketRepositoryAddReplyRejectsWithdrawnTicket(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusWithdrawn, 1, service.SupportTicketSenderRoleSystem))
	mock.ExpectRollback()

	err := repo.AddReply(context.Background(), 1, &service.SupportTicketMessage{
		SenderRole:  service.SupportTicketSenderRoleUser,
		MessageType: service.SupportTicketMessageTypeMessage,
		Content:     "hello",
		CreatedAt:   time.Now(),
	}, service.SupportTicketSenderRoleUser, false, true, service.SupportTicketStatusWaitingAdmin)
	require.ErrorIs(t, err, service.ErrTicketReplyLocked)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryAddReplyDeduplicatesRecentIdenticalReply(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusSubmitted, 1, service.SupportTicketSenderRoleUser))
	mock.ExpectQuery("SELECT sender_role, sender_user_id, message_type, content, COALESCE\\(attachments, '\\[\\]'::jsonb\\), created_at\\s+FROM support_ticket_messages\\s+WHERE ticket_id = \\$1\\s+ORDER BY created_at DESC, id DESC\\s+LIMIT 1").
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"sender_role", "sender_user_id", "message_type", "content", "attachments", "created_at"}).
			AddRow(service.SupportTicketSenderRoleUser, int64(9), service.SupportTicketMessageTypeMessage, "hello", []byte("[]"), now.Add(-2*time.Second)))
	mock.ExpectCommit()

	err := repo.AddReply(context.Background(), 11, &service.SupportTicketMessage{
		SenderRole: service.SupportTicketSenderRoleUser,
		SenderUserID: func() *int64 {
			id := int64(9)
			return &id
		}(),
		MessageType: service.SupportTicketMessageTypeMessage,
		Content:     "hello",
		CreatedAt:   now,
	}, service.SupportTicketSenderRoleUser, false, true, service.SupportTicketStatusWaitingAdmin)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryAddReplyDoesNotDeduplicateOldIdenticalReply(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusSubmitted, 1, service.SupportTicketSenderRoleUser))
	mock.ExpectQuery("SELECT sender_role, sender_user_id, message_type, content, COALESCE\\(attachments, '\\[\\]'::jsonb\\), created_at\\s+FROM support_ticket_messages\\s+WHERE ticket_id = \\$1\\s+ORDER BY created_at DESC, id DESC\\s+LIMIT 1").
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"sender_role", "sender_user_id", "message_type", "content", "attachments", "created_at"}).
			AddRow(service.SupportTicketSenderRoleUser, int64(9), service.SupportTicketMessageTypeMessage, "hello", []byte("[]"), now.Add(-supportTicketReplyDedupWindow-time.Second)))
	mock.ExpectQuery("INSERT INTO support_ticket_messages").
		WithArgs(
			int64(12),
			service.SupportTicketSenderRoleUser,
			sqlmock.AnyArg(),
			"",
			"",
			service.SupportTicketMessageTypeMessage,
			"hello",
			[]byte("[]"),
			now,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
	mock.ExpectExec("UPDATE support_tickets\\s+SET latest_message_at = \\$2, last_reply_role = \\$3, unread_by_user = \\$4, unread_by_admin = \\$5,\\s+status = COALESCE\\(NULLIF\\(\\$6, ''\\), status\\), updated_at = NOW\\(\\)\\s+WHERE id = \\$1").
		WithArgs(int64(12), now, service.SupportTicketSenderRoleUser, false, true, service.SupportTicketStatusWaitingAdmin).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.AddReply(context.Background(), 12, &service.SupportTicketMessage{
		SenderRole: service.SupportTicketSenderRoleUser,
		SenderUserID: func() *int64 {
			id := int64(9)
			return &id
		}(),
		MessageType: service.SupportTicketMessageTypeMessage,
		Content:     "hello",
		CreatedAt:   now,
	}, service.SupportTicketSenderRoleUser, false, true, service.SupportTicketStatusWaitingAdmin)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryCloseByUserRejectsWithdrawnTicket(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusWithdrawn, 1, service.SupportTicketSenderRoleSystem))
	mock.ExpectRollback()

	err := repo.CloseByUser(context.Background(), 9, time.Now(), repoSystemMessage("close", time.Now()))
	require.ErrorIs(t, err, service.ErrTicketCannotClose)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryWithdrawRejectsDisallowedStatus(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusWaitingUser, 1, service.SupportTicketSenderRoleUser))
	mock.ExpectRollback()

	err := repo.UpdateAfterUserWithdraw(context.Background(), 3, time.Now(), repoSystemMessage("withdraw", time.Now()))
	require.ErrorIs(t, err, service.ErrTicketCannotWithdraw)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryCloseByUserUsesLockedTransition(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}
	closedAt := time.Now()
	systemMessage := repoSystemMessage("close", closedAt)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusProcessing, 1, service.SupportTicketSenderRoleSystem))
	mock.ExpectExec("UPDATE support_tickets\\s+SET status = \\$2, closed_at = \\$3, latest_message_at = \\$3, last_reply_role = \\$4,\\s+unread_by_user = FALSE, unread_by_admin = TRUE, withdrawn_at = NULL, updated_at = NOW\\(\\)\\s+WHERE id = \\$1").
		WithArgs(int64(5), service.SupportTicketStatusClosed, closedAt, service.SupportTicketSenderRoleSystem).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO support_ticket_messages").
		WithArgs(
			int64(5),
			systemMessage.SenderRole,
			systemMessage.SenderUserID,
			systemMessage.SenderNameSnapshot,
			systemMessage.SenderAvatarSnapshot,
			systemMessage.MessageType,
			systemMessage.Content,
			[]byte("[]"),
			systemMessage.CreatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	err := repo.CloseByUser(context.Background(), 5, closedAt, systemMessage)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryResubmitUsesLockedRevision(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}
	now := time.Now()
	ticket := &service.SupportTicket{
		Title:              "updated",
		Status:             service.SupportTicketStatusSubmitted,
		CurrentFormPayload: []byte(`{"question":"updated"}`),
		CurrentRevisionNo:  1,
		LatestMessageAt:    now,
		LastReplyRole:      service.SupportTicketSenderRoleSystem,
		UnreadByUser:       false,
		UnreadByAdmin:      true,
		SubmittedAt:        &now,
	}
	revision := &service.SupportTicketRevision{
		RevisionNo:  1,
		Title:       "updated",
		FormPayload: []byte(`{"question":"updated"}`),
		SubmittedAt: now,
	}
	systemMessage := repoSystemMessage("resubmit", now)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusWithdrawn, 4, service.SupportTicketSenderRoleSystem))
	mock.ExpectExec("UPDATE support_tickets").
		WithArgs(int64(8), ticket.Title, ticket.Status, []byte(ticket.CurrentFormPayload), 5, ticket.LatestMessageAt, ticket.LastReplyRole, ticket.UnreadByUser, ticket.UnreadByAdmin, ticket.SubmittedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO support_ticket_revisions").
		WithArgs(int64(8), 5, revision.Title, []byte(revision.FormPayload), revision.SubmittedBy, revision.SubmittedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO support_ticket_messages").
		WithArgs(
			int64(8),
			systemMessage.SenderRole,
			systemMessage.SenderUserID,
			systemMessage.SenderNameSnapshot,
			systemMessage.SenderAvatarSnapshot,
			systemMessage.MessageType,
			systemMessage.Content,
			[]byte("[]"),
			systemMessage.CreatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	err := repo.Resubmit(context.Background(), 8, ticket, revision, systemMessage, 4)
	require.NoError(t, err)
	require.Equal(t, 5, ticket.CurrentRevisionNo)
	require.Equal(t, 5, revision.RevisionNo)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryResubmitRejectsStaleRevisionInsideLock(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusWithdrawn, 5, service.SupportTicketSenderRoleSystem))
	mock.ExpectRollback()

	err := repo.Resubmit(context.Background(), 8, &service.SupportTicket{}, &service.SupportTicketRevision{}, repoSystemMessage("resubmit", now), 4)
	require.ErrorIs(t, err, service.ErrTicketRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryAdminStatusRejectsWaitingUserWithoutAdminReplyInsideLock(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, current_revision_no, last_reply_role\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "current_revision_no", "last_reply_role"}).
			AddRow(service.SupportTicketStatusSubmitted, 1, service.SupportTicketSenderRoleSystem))
	mock.ExpectRollback()

	err := repo.UpdateStatusByAdmin(context.Background(), 9, service.SupportTicketStatusWaitingUser, nil, repoSystemMessage("waiting user", now))
	require.ErrorIs(t, err, service.ErrTicketStatusInvalidTransition)
	require.NoError(t, mock.ExpectationsWereMet())
}

func repoSystemMessage(content string, createdAt time.Time) *service.SupportTicketMessage {
	return &service.SupportTicketMessage{
		SenderRole:           service.SupportTicketSenderRoleSystem,
		SenderNameSnapshot:   "system",
		SenderAvatarSnapshot: "",
		MessageType:          service.SupportTicketMessageTypeSystem,
		Content:              content,
		CreatedAt:            createdAt,
	}
}
