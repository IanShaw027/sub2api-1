package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type ticketRepoStub struct {
	ticket                   *SupportTicket
	createdTicket            *SupportTicket
	replyMessage             *SupportTicketMessage
	nextStatus               string
	listForUserFilters       SupportTicketListFilters
	listForAdminFilters      SupportTicketListFilters
	markReadByUserCount      int
	markReadByAdminCount     int
	listMessagesErr          error
	updateExpectedRevision   int
	resubmitExpectedRevision int
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

func (s *ticketRepoStub) ListForUser(_ context.Context, _ int64, _ pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	s.listForUserFilters = filters
	return nil, &pagination.PaginationResult{}, nil
}

func (s *ticketRepoStub) ListForAdmin(_ context.Context, _ pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	s.listForAdminFilters = filters
	return nil, &pagination.PaginationResult{}, nil
}

func (s *ticketRepoStub) ListMessages(context.Context, int64) ([]SupportTicketMessage, error) {
	if s.listMessagesErr != nil {
		return nil, s.listMessagesErr
	}
	return []SupportTicketMessage{{ID: 1}}, nil
}

func (*ticketRepoStub) UpdateAfterUserWithdraw(context.Context, int64, time.Time, *SupportTicketMessage) error {
	return nil
}

func (s *ticketRepoStub) UpdateEditableContent(_ context.Context, _ int64, _ string, _ json.RawMessage, expectedRevisionNo int) error {
	s.updateExpectedRevision = expectedRevisionNo
	return nil
}

func (s *ticketRepoStub) Resubmit(_ context.Context, _ int64, _ *SupportTicket, _ *SupportTicketRevision, _ *SupportTicketMessage, expectedRevisionNo int) error {
	s.resubmitExpectedRevision = expectedRevisionNo
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

func (s *ticketRepoStub) MarkReadByUser(context.Context, int64) error {
	s.markReadByUserCount++
	return nil
}

func (s *ticketRepoStub) MarkReadByAdmin(context.Context, int64) error {
	s.markReadByAdminCount++
	return nil
}

func TestTicketServiceListRejectsInvalidFilters(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{}, &announcementUserRepoStub{})

	_, _, err := svc.ListForUser(context.Background(), 1, pagination.PaginationParams{}, SupportTicketListFilters{Status: "invalid"})
	require.ErrorIs(t, err, ErrTicketInvalidStatus)

	_, _, err = svc.ListForAdmin(context.Background(), pagination.PaginationParams{}, SupportTicketListFilters{Category: "invalid"})
	require.ErrorIs(t, err, ErrTicketInvalidCategory)
}

func TestTicketServiceListAllowsEmptyAndNormalizesValidFilters(t *testing.T) {
	repo := &ticketRepoStub{}
	svc := NewTicketService(repo, &announcementUserRepoStub{})

	_, _, err := svc.ListForUser(context.Background(), 1, pagination.PaginationParams{}, SupportTicketListFilters{Status: " waiting_admin ", Category: " consult "})

	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusWaitingAdmin, repo.listForUserFilters.Status)
	require.Equal(t, SupportTicketCategoryConsult, repo.listForUserFilters.Category)
}

func TestTicketServiceGetDetailDoesNotMarkRead(t *testing.T) {
	repo := &ticketRepoStub{ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusSubmitted, UnreadByUser: true, UnreadByAdmin: true}}
	svc := NewTicketService(repo, &announcementUserRepoStub{})

	_, err := svc.GetForUser(context.Background(), 9, 1)
	require.NoError(t, err)
	_, err = svc.GetForAdmin(context.Background(), 1)
	require.NoError(t, err)

	require.Zero(t, repo.markReadByUserCount)
	require.Zero(t, repo.markReadByAdminCount)
}

func TestTicketServiceListMessagesMarksReadOnlyAfterSuccessfulList(t *testing.T) {
	repo := &ticketRepoStub{ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusSubmitted}, listMessagesErr: errors.New("list failed")}
	svc := NewTicketService(repo, &announcementUserRepoStub{})

	_, err := svc.ListMessagesForUser(context.Background(), 9, 1)
	require.Error(t, err)
	require.Zero(t, repo.markReadByUserCount)

	repo.listMessagesErr = nil
	_, err = svc.ListMessagesForUser(context.Background(), 9, 1)
	require.NoError(t, err)
	require.Equal(t, 1, repo.markReadByUserCount)
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

func TestTicketServiceCreateRejectsOversizedTitle(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{}, &announcementUserRepoStub{})

	_, err := svc.Create(context.Background(), CreateSupportTicketInput{
		UserID:      1,
		Category:    SupportTicketCategoryConsult,
		Title:       strings.Repeat("中", 81),
		FormPayload: json.RawMessage(`{"question":"hello"}`),
	})

	require.ErrorIs(t, err, ErrTicketInvalidTitle)
}

func TestTicketServiceCreateRejectsOversizedPayload(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{}, &announcementUserRepoStub{})

	_, err := svc.Create(context.Background(), CreateSupportTicketInput{
		UserID:      1,
		Category:    SupportTicketCategoryConsult,
		Title:       "need help",
		FormPayload: json.RawMessage(`{"question":"` + strings.Repeat("a", 20*1024) + `"}`),
	})

	require.ErrorIs(t, err, ErrTicketPayloadInvalid)
}

func TestTicketServiceCreateRejectsPayloadWithTooManyFields(t *testing.T) {
	fields := []string{`"question":"hello"`}
	for i := range 40 {
		fields = append(fields, `"extra_`+strconv.Itoa(i)+`":"x"`)
	}
	svc := NewTicketService(&ticketRepoStub{}, &announcementUserRepoStub{})

	_, err := svc.Create(context.Background(), CreateSupportTicketInput{
		UserID:      1,
		Category:    SupportTicketCategoryConsult,
		Title:       "need help",
		FormPayload: json.RawMessage(`{` + strings.Join(fields, ",") + `}`),
	})

	require.ErrorIs(t, err, ErrTicketPayloadInvalid)
}

func TestNormalizeTicketPayload_PreservesLargeIntegerLexemes(t *testing.T) {
	payload := json.RawMessage(`{"order_no":1234567890123456789,"reason":"refund"}`)

	normalized := normalizeTicketPayload(payload)
	require.NotNil(t, normalized)
	require.Contains(t, string(normalized), "1234567890123456789")
	require.NotContains(t, string(normalized), "e+")
}

func TestTicketServiceReplyForUserRejectsOversizedContent(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusSubmitted},
	}, &announcementUserRepoStub{})

	err := svc.ReplyForUser(context.Background(), 1, CreateSupportTicketMessageInput{
		UserID:  9,
		Content: strings.Repeat("a", 10*1024),
	})

	require.ErrorIs(t, err, ErrTicketMessageTooLarge)
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

func TestTicketServiceReplyForUserRejectsWithdrawnTicket(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusWithdrawn},
	}, &announcementUserRepoStub{})

	err := svc.ReplyForUser(context.Background(), 1, CreateSupportTicketMessageInput{
		UserID:  9,
		Content: "hello",
	})

	require.ErrorIs(t, err, ErrTicketReplyLocked)
}

func TestTicketServiceReplyForAdminRejectsWithdrawnTicket(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusWithdrawn},
	}, &announcementUserRepoStub{})

	err := svc.ReplyForAdmin(context.Background(), 1, CreateSupportTicketMessageInput{
		UserID:  1,
		Content: "done",
	})

	require.ErrorIs(t, err, ErrTicketReplyLocked)
}

func TestTicketServiceCloseForUserRejectsWithdrawnTicket(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusWithdrawn},
	}, &announcementUserRepoStub{})

	err := svc.CloseForUser(context.Background(), 9, 1)
	require.ErrorIs(t, err, ErrTicketCannotClose)
}

func TestTicketServiceAdminStatusRejectsWaitingUserWithoutAdminReply(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusSubmitted, LastReplyRole: SupportTicketSenderRoleSystem},
	}, &announcementUserRepoStub{})

	err := svc.UpdateStatusByAdmin(context.Background(), 1, AdminSupportTicketStatusUpdateInput{AdminUserID: 7, Status: SupportTicketStatusWaitingUser})
	require.ErrorIs(t, err, ErrTicketStatusInvalidTransition)
}

func TestTicketServiceAdminStatusRejectsResolvedWithoutAdminReply(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusWaitingAdmin, LastReplyRole: SupportTicketSenderRoleUser},
	}, &announcementUserRepoStub{})

	err := svc.UpdateStatusByAdmin(context.Background(), 1, AdminSupportTicketStatusUpdateInput{AdminUserID: 7, Status: SupportTicketStatusResolved})
	require.ErrorIs(t, err, ErrTicketStatusInvalidTransition)
}

func TestTicketServiceAdminStatusAllowsResolvedAfterAdminReply(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusWaitingUser, LastReplyRole: SupportTicketSenderRoleAdmin},
	}, &announcementUserRepoStub{})

	err := svc.UpdateStatusByAdmin(context.Background(), 1, AdminSupportTicketStatusUpdateInput{AdminUserID: 7, Status: SupportTicketStatusResolved})
	require.NoError(t, err)
}

func TestTicketServiceUpdateEditableRequiresRevision(t *testing.T) {
	svc := NewTicketService(&ticketRepoStub{
		ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusWithdrawn, Category: SupportTicketCategoryConsult},
	}, &announcementUserRepoStub{})

	err := svc.UpdateEditable(context.Background(), UpdateSupportTicketInput{
		UserID:      9,
		Title:       "need help",
		FormPayload: json.RawMessage(`{"question":"hello"}`),
	}, 1)
	require.ErrorIs(t, err, ErrTicketRevisionRequired)
}

func TestTicketServiceResubmitPassesExpectedRevision(t *testing.T) {
	repo := &ticketRepoStub{ticket: &SupportTicket{ID: 1, UserID: 9, Status: SupportTicketStatusWithdrawn, Category: SupportTicketCategoryConsult, CurrentRevisionNo: 3}}
	svc := NewTicketService(repo, &announcementUserRepoStub{})

	err := svc.Resubmit(context.Background(), UpdateSupportTicketInput{
		UserID:             9,
		Title:              "need help",
		FormPayload:        json.RawMessage(`{"question":"hello"}`),
		ExpectedRevisionNo: 3,
	}, 1)

	require.NoError(t, err)
	require.Equal(t, 3, repo.resubmitExpectedRevision)
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
