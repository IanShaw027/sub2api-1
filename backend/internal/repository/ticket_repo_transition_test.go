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
	mock.ExpectQuery("SELECT status\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).
			AddRow(service.SupportTicketStatusWithdrawn))
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

func TestTicketRepositoryCloseByUserRejectsWithdrawnTicket(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).
			AddRow(service.SupportTicketStatusWithdrawn))
	mock.ExpectRollback()

	err := repo.CloseByUser(context.Background(), 9, time.Now(), repoSystemMessage("close", time.Now()))
	require.ErrorIs(t, err, service.ErrTicketCannotClose)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryWithdrawRejectsDisallowedStatus(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).
			AddRow(service.SupportTicketStatusWaitingUser))
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
	mock.ExpectQuery("SELECT status\\s+FROM support_tickets\\s+WHERE id = \\$1\\s+FOR UPDATE").
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).
			AddRow(service.SupportTicketStatusProcessing))
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
			systemMessage.CreatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	err := repo.CloseByUser(context.Background(), 5, closedAt, systemMessage)
	require.NoError(t, err)
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
