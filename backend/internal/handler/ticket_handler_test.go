package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTicketHandlerGetByIDOmitsUserIdentityFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	repo := &ticketHandlerRepoStub{
		ticket: &service.SupportTicket{
			ID:                 12,
			TicketNo:           "TK-12",
			UserID:             99,
			UserName:           "private-user",
			UserEmail:          "private@example.com",
			UserAvatarURL:      "https://example.com/avatar.png",
			Category:           service.SupportTicketCategoryConsult,
			Title:              "help",
			Status:             service.SupportTicketStatusSubmitted,
			CurrentFormPayload: json.RawMessage(`{"question":"hello"}`),
			CurrentRevisionNo:  1,
			LatestMessageAt:    now,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
	}
	h := NewTicketHandler(service.NewTicketService(repo, &ticketHandlerUserRepoStub{}))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "12"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 99})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/tickets/12", nil)

	h.GetByID(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "TK-12", data["ticket_no"])
	require.NotContains(t, data, "user_id")
	require.NotContains(t, data, "user_name")
	require.NotContains(t, data, "user_email")
	require.NotContains(t, data, "user_avatar_url")
}

func TestTicketHandlerListOmitsUserIdentityFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	repo := &ticketHandlerRepoStub{
		listItems: []service.SupportTicket{{
			ID:                 12,
			TicketNo:           "TK-12",
			UserID:             99,
			UserName:           "private-user",
			UserEmail:          "private@example.com",
			UserAvatarURL:      "https://example.com/avatar.png",
			Category:           service.SupportTicketCategoryConsult,
			Title:              "help",
			Status:             service.SupportTicketStatusSubmitted,
			CurrentFormPayload: json.RawMessage(`{"question":"hello"}`),
			CurrentRevisionNo:  1,
			LatestMessageAt:    now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}},
	}
	h := NewTicketHandler(service.NewTicketService(repo, &ticketHandlerUserRepoStub{}))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 99})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/tickets", nil)

	h.List(c)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "TK-12")
	require.NotContains(t, body, "user_id")
	require.NotContains(t, body, "user_name")
	require.NotContains(t, body, "user_email")
	require.NotContains(t, body, "user_avatar_url")
}

func TestTicketHandlerReplyRejectsOversizedBodyBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &ticketHandlerRepoStub{ticket: &service.SupportTicket{ID: 12, UserID: 99, Status: service.SupportTicketStatusSubmitted}}
	h := NewTicketHandler(service.NewTicketService(repo, &ticketHandlerUserRepoStub{}))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "12"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 99})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/tickets/12/messages", strings.NewReader(`{"content":"`+strings.Repeat("a", 20*1024)+`"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Reply(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, repo.addReplyCalls)
}

type ticketHandlerRepoStub struct {
	ticket        *service.SupportTicket
	listItems     []service.SupportTicket
	addReplyCalls int
}

func (s *ticketHandlerRepoStub) CreateSubmitted(_ context.Context, ticket *service.SupportTicket, _ *service.SupportTicketRevision, _ *service.SupportTicketMessage) error {
	if ticket != nil {
		stored := *ticket
		if stored.ID == 0 {
			stored.ID = 1
		}
		s.ticket = &stored
	}
	return nil
}

func (s *ticketHandlerRepoStub) GetByID(context.Context, int64) (*service.SupportTicket, error) {
	if s.ticket == nil {
		return nil, service.ErrTicketNotFound
	}
	return s.ticket, nil
}

func (s *ticketHandlerRepoStub) ListForUser(context.Context, int64, pagination.PaginationParams, service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	return s.listItems, &pagination.PaginationResult{Total: int64(len(s.listItems)), Page: 1, PageSize: 20}, nil
}

func (*ticketHandlerRepoStub) ListForAdmin(context.Context, pagination.PaginationParams, service.SupportTicketListFilters) ([]service.SupportTicket, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*ticketHandlerRepoStub) ListMessages(context.Context, int64) ([]service.SupportTicketMessage, error) {
	return nil, nil
}

func (*ticketHandlerRepoStub) UpdateAfterUserWithdraw(context.Context, int64, time.Time, *service.SupportTicketMessage) error {
	return nil
}

func (*ticketHandlerRepoStub) UpdateEditableContent(context.Context, int64, string, json.RawMessage) error {
	return nil
}

func (*ticketHandlerRepoStub) Resubmit(context.Context, int64, *service.SupportTicket, *service.SupportTicketRevision, *service.SupportTicketMessage) error {
	return nil
}

func (*ticketHandlerRepoStub) CloseByUser(context.Context, int64, time.Time, *service.SupportTicketMessage) error {
	return nil
}

func (s *ticketHandlerRepoStub) AddReply(context.Context, int64, *service.SupportTicketMessage, string, bool, bool, string) error {
	s.addReplyCalls++
	return nil
}

func (*ticketHandlerRepoStub) UpdateStatusByAdmin(context.Context, int64, string, *time.Time, *service.SupportTicketMessage) error {
	return nil
}

func (*ticketHandlerRepoStub) MarkReadByUser(context.Context, int64) error  { return nil }
func (*ticketHandlerRepoStub) MarkReadByAdmin(context.Context, int64) error { return nil }

type ticketHandlerUserRepoStub struct{}

func (*ticketHandlerUserRepoStub) Create(context.Context, *service.User) error { return nil }
func (*ticketHandlerUserRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Email: "user@example.com", Username: "user"}, nil
}
func (*ticketHandlerUserRepoStub) GetByEmail(context.Context, string) (*service.User, error) {
	return nil, nil
}
func (*ticketHandlerUserRepoStub) GetFirstAdmin(context.Context) (*service.User, error) {
	return nil, nil
}
func (*ticketHandlerUserRepoStub) Update(context.Context, *service.User) error { return nil }
func (*ticketHandlerUserRepoStub) Delete(context.Context, int64) error         { return nil }
func (*ticketHandlerUserRepoStub) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}
func (*ticketHandlerUserRepoStub) UpsertUserAvatar(context.Context, int64, service.UpsertUserAvatarInput) (*service.UserAvatar, error) {
	return nil, nil
}
func (*ticketHandlerUserRepoStub) DeleteUserAvatar(context.Context, int64) error { return nil }
func (*ticketHandlerUserRepoStub) List(context.Context, pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (*ticketHandlerUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (*ticketHandlerUserRepoStub) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	return nil, nil
}
func (*ticketHandlerUserRepoStub) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func (*ticketHandlerUserRepoStub) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	return nil
}
func (*ticketHandlerUserRepoStub) UpdateBalance(context.Context, int64, float64) error { return nil }
func (*ticketHandlerUserRepoStub) DeductBalance(context.Context, int64, float64) error { return nil }
func (*ticketHandlerUserRepoStub) UpdateConcurrency(context.Context, int64, int) error { return nil }
func (*ticketHandlerUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	return false, nil
}
func (*ticketHandlerUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	return 0, nil
}
func (*ticketHandlerUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (*ticketHandlerUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (*ticketHandlerUserRepoStub) ListUserAuthIdentities(context.Context, int64) ([]service.UserAuthIdentityRecord, error) {
	return nil, nil
}
func (*ticketHandlerUserRepoStub) UnbindUserAuthProvider(context.Context, int64, string) error {
	return nil
}
func (*ticketHandlerUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error { return nil }
func (*ticketHandlerUserRepoStub) EnableTotp(context.Context, int64) error                { return nil }
func (*ticketHandlerUserRepoStub) DisableTotp(context.Context, int64) error               { return nil }
