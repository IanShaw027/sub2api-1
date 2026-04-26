//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelMonitorSettingRepoStub struct {
	values map[string]string
}

func (s *channelMonitorSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}
func (s *channelMonitorSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}
func (s *channelMonitorSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}
func (s *channelMonitorSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}
func (s *channelMonitorSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}
func (s *channelMonitorSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}
func (s *channelMonitorSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestChannelMonitorHandler_RejectsAdminAPIWhenFeatureDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := service.NewSettingService(&channelMonitorSettingRepoStub{values: map[string]string{
		service.SettingKeyChannelMonitorEnabled: "false",
	}}, &config.Config{})
	h := NewChannelMonitorHandler(nil, settings)

	router := gin.New()
	router.GET("/api/v1/admin/channel-monitors", h.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/channel-monitors", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "CHANNEL_MONITOR_DISABLED")
}
