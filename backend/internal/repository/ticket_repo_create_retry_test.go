//go:build unit

package repository

import (
	"context"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTicketRepositoryCreateSubmittedRetriesOnTicketNoConflict(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}

	now := time.Now()
	ticket := &service.SupportTicket{
		UserID:             11,
		Category:           service.SupportTicketCategoryConsult,
		Title:              "retry-ticket-no",
		Status:             service.SupportTicketStatusSubmitted,
		CurrentFormPayload: []byte(`{"k":"v"}`),
		CurrentRevisionNo:  1,
		LatestMessageAt:    now,
		LastReplyRole:      service.SupportTicketSenderRoleSystem,
		UnreadByUser:       false,
		UnreadByAdmin:      true,
		SubmittedAt:        &now,
	}
	revision := &service.SupportTicketRevision{
		RevisionNo:  1,
		Title:       ticket.Title,
		FormPayload: []byte(`{"k":"v"}`),
		SubmittedAt: now,
	}
	systemMessage := &service.SupportTicketMessage{
		SenderRole:           service.SupportTicketSenderRoleSystem,
		SenderNameSnapshot:   "system",
		SenderAvatarSnapshot: "",
		MessageType:          service.SupportTicketMessageTypeSystem,
		Content:              "created",
		CreatedAt:            now,
	}

	uniqueErr := &pq.Error{Code: "23505", Constraint: "support_tickets_ticket_no_key"}

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO support_tickets").
		WithArgs(
			sqlmock.AnyArg(),
			ticket.UserID,
			ticket.Category,
			ticket.Title,
			ticket.Status,
			sqlmock.AnyArg(),
			ticket.CurrentRevisionNo,
			ticket.LatestMessageAt,
			ticket.LastReplyRole,
			ticket.UnreadByUser,
			ticket.UnreadByAdmin,
			ticket.SubmittedAt,
		).
		WillReturnError(uniqueErr)
	mock.ExpectRollback()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO support_tickets").
		WithArgs(
			sqlmock.AnyArg(),
			ticket.UserID,
			ticket.Category,
			ticket.Title,
			ticket.Status,
			sqlmock.AnyArg(),
			ticket.CurrentRevisionNo,
			ticket.LatestMessageAt,
			ticket.LastReplyRole,
			ticket.UnreadByUser,
			ticket.UnreadByAdmin,
			ticket.SubmittedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(101), now, now))
	mock.ExpectQuery("INSERT INTO support_ticket_revisions").
		WithArgs(
			int64(101),
			revision.RevisionNo,
			revision.Title,
			sqlmock.AnyArg(),
			revision.SubmittedBy,
			revision.SubmittedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(201), now))
	mock.ExpectQuery("INSERT INTO support_ticket_messages").
		WithArgs(
			int64(101),
			systemMessage.SenderRole,
			systemMessage.SenderUserID,
			systemMessage.SenderNameSnapshot,
			systemMessage.SenderAvatarSnapshot,
			systemMessage.MessageType,
			systemMessage.Content,
			systemMessage.CreatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(301)))
	mock.ExpectCommit()

	err := repo.CreateSubmitted(context.Background(), ticket, revision, systemMessage)
	require.NoError(t, err)
	require.Equal(t, int64(101), ticket.ID)
	require.NotEmpty(t, ticket.TicketNo)
	require.Equal(t, int64(101), revision.TicketID)
	require.Equal(t, int64(201), revision.ID)
	require.Equal(t, int64(101), systemMessage.TicketID)
	require.Equal(t, int64(301), systemMessage.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketRepositoryCreateSubmittedStopsAfterBoundedTicketNoRetries(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &ticketRepository{db: db}

	now := time.Now()
	ticket := &service.SupportTicket{
		UserID:             11,
		Category:           service.SupportTicketCategoryConsult,
		Title:              "retry-exhausted",
		Status:             service.SupportTicketStatusSubmitted,
		CurrentFormPayload: []byte(`{"k":"v"}`),
		CurrentRevisionNo:  1,
		LatestMessageAt:    now,
		LastReplyRole:      service.SupportTicketSenderRoleSystem,
		UnreadByUser:       false,
		UnreadByAdmin:      true,
		SubmittedAt:        &now,
	}
	revision := &service.SupportTicketRevision{
		RevisionNo:  1,
		Title:       ticket.Title,
		FormPayload: []byte(`{"k":"v"}`),
		SubmittedAt: now,
	}
	systemMessage := &service.SupportTicketMessage{
		SenderRole:           service.SupportTicketSenderRoleSystem,
		SenderNameSnapshot:   "system",
		SenderAvatarSnapshot: "",
		MessageType:          service.SupportTicketMessageTypeSystem,
		Content:              "created",
		CreatedAt:            now,
	}

	uniqueErr := &pq.Error{Code: "23505", Constraint: "support_tickets_ticket_no_key"}

	for i := 0; i < createSubmittedMaxTicketNoAttempts; i++ {
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT INTO support_tickets").
			WithArgs(
				sqlmock.AnyArg(),
				ticket.UserID,
				ticket.Category,
				ticket.Title,
				ticket.Status,
				sqlmock.AnyArg(),
				ticket.CurrentRevisionNo,
				ticket.LatestMessageAt,
				ticket.LastReplyRole,
				ticket.UnreadByUser,
				ticket.UnreadByAdmin,
				ticket.SubmittedAt,
			).
			WillReturnError(uniqueErr)
		mock.ExpectRollback()
	}

	err := repo.CreateSubmitted(context.Background(), ticket, revision, systemMessage)
	require.Error(t, err)
	var pgErr *pq.Error
	require.ErrorAs(t, err, &pgErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
