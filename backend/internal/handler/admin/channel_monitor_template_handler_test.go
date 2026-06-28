//go:build unit

package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelMonitorTemplateHandlerRepoStub struct {
	created []*service.ChannelMonitorRequestTemplate
}

func (r *channelMonitorTemplateHandlerRepoStub) Create(_ context.Context, t *service.ChannelMonitorRequestTemplate) error {
	t.ID = int64(len(r.created) + 1)
	r.created = append(r.created, t)
	return nil
}

func (r *channelMonitorTemplateHandlerRepoStub) GetByID(_ context.Context, id int64) (*service.ChannelMonitorRequestTemplate, error) {
	for _, tpl := range r.created {
		if tpl.ID == id {
			return tpl, nil
		}
	}
	return nil, service.ErrChannelMonitorTemplateNotFound
}

func (r *channelMonitorTemplateHandlerRepoStub) Update(_ context.Context, t *service.ChannelMonitorRequestTemplate) error {
	for i, tpl := range r.created {
		if tpl.ID == t.ID {
			r.created[i] = t
			return nil
		}
	}
	return service.ErrChannelMonitorTemplateNotFound
}

func (r *channelMonitorTemplateHandlerRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (r *channelMonitorTemplateHandlerRepoStub) List(context.Context, service.ChannelMonitorRequestTemplateListParams) ([]*service.ChannelMonitorRequestTemplate, error) {
	return r.created, nil
}

func (r *channelMonitorTemplateHandlerRepoStub) ApplyToMonitors(context.Context, int64, []int64) (int64, error) {
	return 0, nil
}

func (r *channelMonitorTemplateHandlerRepoStub) CountAssociatedMonitors(context.Context, int64) (int64, error) {
	return 0, nil
}

func (r *channelMonitorTemplateHandlerRepoStub) ListAssociatedMonitors(context.Context, int64) ([]*service.AssociatedMonitorBrief, error) {
	return nil, nil
}

func TestChannelMonitorTemplateHandlerCreateAcceptsKiroProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &channelMonitorTemplateHandlerRepoStub{}
	handler := NewChannelMonitorRequestTemplateHandler(service.NewChannelMonitorRequestTemplateService(repo))

	body := []byte(`{"name":"kiro-template","provider":"kiro","description":"Kiro template"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/channel-monitor-templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	handler.Create(c)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, repo.created, 1)
	require.Equal(t, service.MonitorProviderKiro, repo.created[0].Provider)
}

func TestChannelMonitorTemplateHandlerCreateAcceptsGrokProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &channelMonitorTemplateHandlerRepoStub{}
	handler := NewChannelMonitorRequestTemplateHandler(service.NewChannelMonitorRequestTemplateService(repo))

	body := []byte(`{"name":"grok-template","provider":"grok","description":"Grok template"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/channel-monitor-templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	handler.Create(c)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, repo.created, 1)
	require.Equal(t, service.MonitorProviderGrok, repo.created[0].Provider)
}
