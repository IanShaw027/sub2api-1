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
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type ticketHandlerSettingRepoStub struct {
	value    string
	setCount int
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

func TestTicketHandlerResolveAttachmentsForAdminSignsPrivateMediaURLs(t *testing.T) {
	mediaSvc := service.NewMediaService(&settingHandlerMediaRepoStub{
		assets: map[int64]*service.MediaAsset{
			321: {
				ID:                 321,
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
			PublicBaseURL:         "https://media.example.com",
			PresignExpiryMinutes:  10,
			DownloadSigningSecret: "secret",
		},
	})
	handler := NewTicketHandler(nil, nil, mediaSvc)

	attachments, err := handler.resolveAttachmentsForAdmin(context.Background(), []TicketAttachmentRefRequest{{MediaID: 321}})

	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.Contains(t, attachments[0].URL, "https://media.example.com/api/v1/media/download/321?expires=")
	require.Contains(t, attachments[0].ThumbnailURL, "https://media.example.com/api/v1/media/download/321/thumbnail?expires=")
}
