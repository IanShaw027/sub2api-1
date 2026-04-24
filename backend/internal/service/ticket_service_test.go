package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type ticketRepoStub struct {
	ticket *SupportTicket
}

func (*ticketRepoStub) CreateSubmitted(context.Context, *SupportTicket, *SupportTicketRevision, *SupportTicketMessage) error {
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

func (*ticketRepoStub) AddReply(context.Context, int64, *SupportTicketMessage, string, bool, bool) error {
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
