package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type ticketRepoStub struct {
	ticket        *SupportTicket
	createdTicket *SupportTicket
	replyMessage  *SupportTicketMessage
	nextStatus    string
}

func (s *ticketRepoStub) CreateSubmitted(_ context.Context, ticket *SupportTicket, _ *SupportTicketRevision, _ *SupportTicketMessage) error {
	if ticket != nil {
		stored := *ticket
		if stored.ID == 0 {
			stored.ID = 1
		}
		s.createdTicket = &stored
		s.ticket = &stored
	}
	return nil
}

func (s *ticketRepoStub) GetByID(context.Context, int64) (*SupportTicket, error) {
	if s.ticket == nil {
		return nil, ErrTicketNotFound
	}
	return s.ticket, nil
}

func (*ticketRepoStub) ListForUser(context.Context, int64, pagination.PaginationParams, SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*ticketRepoStub) ListForAdmin(context.Context, pagination.PaginationParams, SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*ticketRepoStub) ListMessages(context.Context, int64) ([]SupportTicketMessage, error) {
	return nil, nil
}

func (*ticketRepoStub) UpdateAfterUserWithdraw(context.Context, int64, time.Time, *SupportTicketMessage) error {
	return nil
}

func (*ticketRepoStub) UpdateEditableContent(context.Context, int64, string, json.RawMessage) error {
	return nil
}

func (*ticketRepoStub) Resubmit(context.Context, int64, *SupportTicket, *SupportTicketRevision, *SupportTicketMessage) error {
	return nil
}

func (*ticketRepoStub) CloseByUser(context.Context, int64, time.Time, *SupportTicketMessage) error {
	return nil
}

func (s *ticketRepoStub) AddReply(_ context.Context, _ int64, message *SupportTicketMessage, _ string, _ bool, _ bool, nextStatus string) error {
	if message != nil {
		stored := *message
		s.replyMessage = &stored
	}
	s.nextStatus = nextStatus
	return nil
}

func (*ticketRepoStub) UpdateStatusByAdmin(context.Context, int64, string, *time.Time, *SupportTicketMessage) error {
	return nil
}

func (*ticketRepoStub) MarkReadByUser(context.Context, int64) error {
	return nil
}

func (*ticketRepoStub) MarkReadByAdmin(context.Context, int64) error {
	return nil
}

func TestTicketServiceCreateRejectsCategorySpecificIncompletePayload(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{}, &announcementUserRepoStub{})

	_, err := svc.Create(context.Background(), CreateSupportTicketInput{
		UserID:      1,
		Category:    SupportTicketCategoryRefund,
		Title:       "refund request",
		FormPayload: json.RawMessage(`{"reason":"need refund"}`),
	})

	require.ErrorIs(t, err, ErrTicketPayloadInvalid)
}

func TestTicketServiceReplyForUserRejectsLockedTicket(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusResolved},
	}, &announcementUserRepoStub{})

	err := svc.ReplyForUser(context.Background(), 1, CreateSupportTicketMessageInput{
		UserID:  9,
		Content: "hello",
	})

	require.ErrorIs(t, err, ErrTicketReplyLocked)
}

func TestTicketServiceReplyForAdminRejectsLockedTicket(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusClosed},
	}, &announcementUserRepoStub{})

	err := svc.ReplyForAdmin(context.Background(), 1, CreateSupportTicketMessageInput{
		UserID:  1,
		Content: "done",
	})

	require.ErrorIs(t, err, ErrTicketReplyLocked)
}

type ticketUserRepoStub struct {
	announcementUserRepoStub
	user       *User
	getByIDErr error
	avatar     *UserAvatar
	avatarErr  error
}

func (s *ticketUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	if s.user != nil {
		return s.user, nil
	}
	return &User{ID: id, Email: "user@example.com", Username: "user"}, nil
}

func (s *ticketUserRepoStub) GetUserAvatar(_ context.Context, _ int64) (*UserAvatar, error) {
	if s.avatarErr != nil {
		return nil, s.avatarErr
	}
	return s.avatar, nil
}

func TestTicketServiceCreateIgnoresAvatarLookupError(t *testing.T) {
	repo := &ticketRepoStub{}
	svc := NewTicketService(repo, &ticketUserRepoStub{
		user:      &User{ID: 1, Email: "user@example.com", Username: "ticket-user"},
		avatarErr: errors.New("avatar lookup failed"),
	})

	ticket, err := svc.Create(context.Background(), CreateSupportTicketInput{
		UserID:      1,
		Category:    SupportTicketCategoryConsult,
		Title:       "need help",
		FormPayload: json.RawMessage(`{"question":"hello"}`),
	})

	require.NoError(t, err)
	require.NotNil(t, ticket)
	require.NotNil(t, repo.createdTicket)
	require.Equal(t, "", repo.createdTicket.UserAvatarURL)
	require.Equal(t, "", ticket.UserAvatarURL)
}

func TestTicketServiceReplyForUserIgnoresAvatarLookupError(t *testing.T) {
	repo := &ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusSubmitted},
	}
	svc := NewTicketService(repo, &ticketUserRepoStub{
		user:      &User{ID: 9, Email: "user@example.com", Username: "ticket-user"},
		avatarErr: errors.New("avatar lookup failed"),
	})

	err := svc.ReplyForUser(context.Background(), 1, CreateSupportTicketMessageInput{
		UserID:  9,
		Content: "hello",
	})

	require.NoError(t, err)
	require.NotNil(t, repo.replyMessage)
	require.Equal(t, "", repo.replyMessage.SenderAvatarSnapshot)
	require.Equal(t, SupportTicketStatusWaitingAdmin, repo.nextStatus)
}

func TestTicketServiceReplyForAdminIgnoresAvatarLookupError(t *testing.T) {
	repo := &ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusSubmitted},
	}
	svc := NewTicketService(repo, &ticketUserRepoStub{
		user:      &User{ID: 7, Email: "admin@example.com", Username: "ticket-admin"},
		avatarErr: errors.New("avatar lookup failed"),
	})

	err := svc.ReplyForAdmin(context.Background(), 1, CreateSupportTicketMessageInput{
		UserID:  7,
		Content: "resolved",
	})

	require.NoError(t, err)
	require.NotNil(t, repo.replyMessage)
	require.Equal(t, "", repo.replyMessage.SenderAvatarSnapshot)
	require.Equal(t, SupportTicketStatusWaitingUser, repo.nextStatus)
}
