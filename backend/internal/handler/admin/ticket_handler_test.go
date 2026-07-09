package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type ticketHandlerSettingRepoStub struct {
	value    string
	setCount int
}

type adminTicketHandlerRepoStub struct {
	service.SupportTicketRepository

	ticket        *service.SupportTicket
	messages      []service.SupportTicketMessage
	replyMessage  *service.SupportTicketMessage
	addReplyCalls int
}

func (s *adminTicketHandlerRepoStub) GetByID(context.Context, int64) (*service.SupportTicket, error) {
	if s.ticket == nil {
		return nil, service.ErrTicketNotFound
	}
	return s.ticket, nil
}

func (s *adminTicketHandlerRepoStub) ListMessages(context.Context, int64) ([]service.SupportTicketMessage, error) {
	return s.messages, nil
}

func (s *adminTicketHandlerRepoStub) AddReply(_ context.Context, _ int64, message *service.SupportTicketMessage, _ string, _ bool, _ bool, _ string) error {
	s.addReplyCalls++
	if message != nil {
		stored := *message
		s.replyMessage = &stored
	}
	return nil
}

func (*adminTicketHandlerRepoStub) MarkReadByAdmin(context.Context, int64) error {
	return nil
}

type adminTicketHandlerUserRepoStub struct {
	service.UserRepository
}

func (*adminTicketHandlerUserRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Email: "admin@example.com", Username: "admin"}, nil
}

func (*adminTicketHandlerUserRepoStub) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

func (*ticketHandlerSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *ticketHandlerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if key != service.SettingKeyAdminTicketReplyTemplates {
		panic("unexpected GetValue call")
	}
	if s.value == "" {
		return "", service.ErrSettingNotFound
	}
	return s.value, nil
}

func (s *ticketHandlerSettingRepoStub) Set(_ context.Context, key, value string) error {
	if key != service.SettingKeyAdminTicketReplyTemplates {
		panic("unexpected Set call")
	}
	s.setCount++
	s.value = value
	return nil
}

func (*ticketHandlerSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (*ticketHandlerSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (*ticketHandlerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (*ticketHandlerSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestTicketHandlerReplaceReplyTemplatesRejectsMissingTemplatesField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &ticketHandlerSettingRepoStub{
		value: `[{"id":"existing","title":"Existing","content":"Saved"}]`,
	}
	handler := NewTicketHandler(nil, service.NewSettingService(repo, nil), nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/tickets/reply-templates", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ReplaceReplyTemplates(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, repo.setCount)
	require.Equal(t, `[{"id":"existing","title":"Existing","content":"Saved"}]`, repo.value)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestTicketHandlerResolveAttachmentsForAdminBuildsMetadataOnly(t *testing.T) {
	mediaSvc := service.NewMediaService(&settingHandlerMediaRepoStub{
		assets: map[int64]*service.MediaAsset{
			321: {
				ID:                 321,
				BizType:            "ticket",
				BizID:              "12",
				Visibility:         service.MediaVisibilityPrivate,
				Status:             service.MediaStatusActive,
				ThumbnailObjectKey: "thumbs/321.png",
				OriginalFileName:   "screen.png",
				MIMEType:           "image/png",
				SizeBytes:          42,
			},
		},
	}, &settingHandlerMediaStoreStub{}, &config.Config{
		Media: config.MediaConfig{
			Enabled:               true,
			Endpoint:              "https://s3.example.com",
			Bucket:                "media",
			AccessKeyID:           "test-ak",
			SecretAccessKey:       "test-sk",
			PublicBaseURL:         "https://media.example.com",
			PresignExpiryMinutes:  10,
			DownloadSigningSecret: "secret",
		},
	})
	handler := NewTicketHandler(nil, nil, mediaSvc)

	attachments, err := handler.resolveAttachmentsForAdmin(context.Background(), 12, []TicketAttachmentRefRequest{{MediaID: 321}})

	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.Equal(t, int64(321), attachments[0].MediaID)
	require.Equal(t, "screen.png", attachments[0].FileName)
	require.Equal(t, "image/png", attachments[0].ContentType)
	require.Equal(t, int64(42), attachments[0].SizeBytes)
	require.Empty(t, attachments[0].URL)
	require.Empty(t, attachments[0].ThumbnailURL)
}

func TestTicketHandlerReplyRejectsTicketScopedMediaMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &adminTicketHandlerRepoStub{
		ticket: &service.SupportTicket{ID: 12, UserID: 77, Status: service.SupportTicketStatusSubmitted},
	}
	mediaSvc := service.NewMediaService(&settingHandlerMediaRepoStub{
		assets: map[int64]*service.MediaAsset{
			321: {
				ID:               321,
				BizType:          "ticket",
				BizID:            "999",
				Visibility:       service.MediaVisibilityPrivate,
				Status:           service.MediaStatusActive,
				OriginalFileName: "screen.png",
				MIMEType:         "image/png",
				SizeBytes:        42,
			},
		},
	}, &settingHandlerMediaStoreStub{}, &config.Config{
		Media: config.MediaConfig{
			Enabled:               true,
			Endpoint:              "https://s3.example.com",
			Bucket:                "media",
			AccessKeyID:           "test-ak",
			SecretAccessKey:       "test-sk",
			PublicBaseURL:         "https://media.example.com",
			PresignExpiryMinutes:  10,
			DownloadSigningSecret: "secret",
		},
	})
	handler := NewTicketHandler(service.NewTicketService(repo, &adminTicketHandlerUserRepoStub{}), nil, mediaSvc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 88})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/tickets/12/messages", bytes.NewBufferString(`{"content":"reply","attachments":[{"media_id":321}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "12"}}

	handler.Reply(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Zero(t, repo.addReplyCalls)
}

func TestTicketHandlerReplyPersistsTicketAttachmentMetadataWithoutSignedURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &adminTicketHandlerRepoStub{
		ticket: &service.SupportTicket{ID: 12, UserID: 77, Status: service.SupportTicketStatusSubmitted},
	}
	mediaSvc := service.NewMediaService(&settingHandlerMediaRepoStub{
		assets: map[int64]*service.MediaAsset{
			321: {
				ID:                 321,
				BizType:            "ticket",
				BizID:              "12",
				Visibility:         service.MediaVisibilityPrivate,
				Status:             service.MediaStatusActive,
				ThumbnailObjectKey: "thumbs/321.png",
				OriginalFileName:   "screen.png",
				MIMEType:           "image/png",
				SizeBytes:          42,
			},
		},
	}, &settingHandlerMediaStoreStub{}, &config.Config{
		Media: config.MediaConfig{
			Enabled:               true,
			Endpoint:              "https://s3.example.com",
			Bucket:                "media",
			AccessKeyID:           "test-ak",
			SecretAccessKey:       "test-sk",
			PublicBaseURL:         "https://media.example.com",
			PresignExpiryMinutes:  10,
			DownloadSigningSecret: "secret",
		},
	})
	handler := NewTicketHandler(service.NewTicketService(repo, &adminTicketHandlerUserRepoStub{}), nil, mediaSvc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 88})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/tickets/12/messages", bytes.NewBufferString(`{"attachments":[{"media_id":321}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "12"}}

	handler.Reply(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, repo.addReplyCalls)
	require.NotNil(t, repo.replyMessage)
	require.Len(t, repo.replyMessage.Attachments, 1)
	require.Equal(t, int64(321), repo.replyMessage.Attachments[0].MediaID)
	require.Equal(t, "screen.png", repo.replyMessage.Attachments[0].FileName)
	require.Equal(t, "image/png", repo.replyMessage.Attachments[0].ContentType)
	require.Equal(t, int64(42), repo.replyMessage.Attachments[0].SizeBytes)
	require.Empty(t, repo.replyMessage.Attachments[0].URL)
	require.Empty(t, repo.replyMessage.Attachments[0].ThumbnailURL)
}

func TestTicketHandlerReplyIdempotencyReplaysWithoutDuplicateReply(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryIdempotencyRepoStub()
	cfg := service.DefaultIdempotencyConfig()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(repo, cfg))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(nil)
	})

	ticketRepo := &adminTicketHandlerRepoStub{
		ticket: &service.SupportTicket{ID: 12, UserID: 77, Status: service.SupportTicketStatusSubmitted},
	}
	handler := NewTicketHandler(service.NewTicketService(ticketRepo, &adminTicketHandlerUserRepoStub{}), nil, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 88})
		c.Next()
	})
	router.POST("/api/v1/admin/tickets/:id/messages", handler.Reply)

	call := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tickets/12/messages", bytes.NewBufferString(`{"content":"reply once"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "admin-ticket-reply-1")
		router.ServeHTTP(rec, req)
		return rec
	}

	first := call()
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, 1, ticketRepo.addReplyCalls)

	second := call()
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 1, ticketRepo.addReplyCalls)
}

func TestTicketHandlerListMessagesFallsBackToImageURLWhenThumbnailMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &adminTicketHandlerRepoStub{
		ticket: &service.SupportTicket{ID: 12, UserID: 77, Status: service.SupportTicketStatusSubmitted},
		messages: []service.SupportTicketMessage{
			{
				ID:                 1,
				TicketID:           12,
				SenderRole:         service.SupportTicketSenderRoleAdmin,
				SenderNameSnapshot: "Admin",
				MessageType:        service.SupportTicketMessageTypeMessage,
				Content:            "",
				Attachments: []service.TicketMessageAttachment{
					{
						MediaID:     321,
						FileName:    "screen.png",
						ContentType: "image/png",
						SizeBytes:   42,
					},
				},
			},
		},
	}
	mediaSvc := service.NewMediaService(&settingHandlerMediaRepoStub{
		assets: map[int64]*service.MediaAsset{
			321: {
				ID:               321,
				BizType:          "ticket",
				BizID:            "12",
				Visibility:       service.MediaVisibilityPrivate,
				Status:           service.MediaStatusActive,
				OriginalFileName: "screen.png",
				MIMEType:         "image/png",
				SizeBytes:        42,
			},
		},
	}, &settingHandlerMediaStoreStub{}, &config.Config{
		Media: config.MediaConfig{
			Enabled:               true,
			Endpoint:              "https://s3.example.com",
			Bucket:                "media",
			AccessKeyID:           "test-ak",
			SecretAccessKey:       "test-sk",
			PublicBaseURL:         "https://media.example.com",
			PresignExpiryMinutes:  10,
			DownloadSigningSecret: "secret",
		},
	})
	handler := NewTicketHandler(service.NewTicketService(repo, &adminTicketHandlerUserRepoStub{}), nil, mediaSvc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 88})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/tickets/12/messages", nil)
	c.Params = gin.Params{{Key: "id", Value: "12"}}

	handler.ListMessages(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)

	payload, ok := resp.Data.([]any)
	require.True(t, ok)
	require.Len(t, payload, 1)

	message, ok := payload[0].(map[string]any)
	require.True(t, ok)
	attachments, ok := message["attachments"].([]any)
	require.True(t, ok)
	require.Len(t, attachments, 1)

	attachment, ok := attachments[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, attachment["url"], attachment["thumbnail_url"])
	url, ok := attachment["url"].(string)
	require.True(t, ok)
	require.Contains(t, url, "http://example.com/api/v1/media/download/321?")
}
