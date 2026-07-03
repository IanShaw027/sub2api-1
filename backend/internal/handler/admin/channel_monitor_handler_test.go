//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

type channelMonitorAdjustRepoStub struct {
	monitor   *service.ChannelMonitor
	requested float64
	createFn  func(context.Context, *service.ChannelMonitor) error
}

func (s *channelMonitorAdjustRepoStub) Create(ctx context.Context, monitor *service.ChannelMonitor) error {
	if s.createFn != nil {
		return s.createFn(ctx, monitor)
	}
	panic("unexpected Create call")
}
func (s *channelMonitorAdjustRepoStub) GetByID(context.Context, int64) (*service.ChannelMonitor, error) {
	return s.monitor, nil
}
func (s *channelMonitorAdjustRepoStub) Update(context.Context, *service.ChannelMonitor) error {
	panic("unexpected Update call")
}
func (s *channelMonitorAdjustRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *channelMonitorAdjustRepoStub) List(context.Context, service.ChannelMonitorListParams) ([]*service.ChannelMonitor, int64, error) {
	panic("unexpected List call")
}
func (s *channelMonitorAdjustRepoStub) AdjustAvailability7d(ctx context.Context, monitorID int64, model string, availabilityPct float64) (*service.ChannelMonitorAvailabilityAdjustResult, error) {
	s.requested = availabilityPct
	return &service.ChannelMonitorAvailabilityAdjustResult{
		MonitorID:                 monitorID,
		Model:                     model,
		TotalChecks:               10,
		PreviousOperationalChecks: 7,
		TargetOperationalChecks:   0,
		ActualOperationalChecks:   0,
		ChangedRows:               7,
		PreviousAvailabilityPct:   70,
		RequestedAvailabilityPct:  availabilityPct,
		ActualAvailabilityPct:     0,
	}, nil
}
func (s *channelMonitorAdjustRepoStub) ListEnabled(context.Context) ([]*service.ChannelMonitor, error) {
	panic("unexpected ListEnabled call")
}
func (s *channelMonitorAdjustRepoStub) MarkChecked(context.Context, int64, time.Time) error {
	panic("unexpected MarkChecked call")
}
func (s *channelMonitorAdjustRepoStub) InsertHistoryBatch(context.Context, []*service.ChannelMonitorHistoryRow) error {
	panic("unexpected InsertHistoryBatch call")
}
func (s *channelMonitorAdjustRepoStub) DeleteHistoryBefore(context.Context, time.Time) (int64, error) {
	panic("unexpected DeleteHistoryBefore call")
}
func (s *channelMonitorAdjustRepoStub) ListHistory(context.Context, int64, string, int) ([]*service.ChannelMonitorHistoryEntry, error) {
	panic("unexpected ListHistory call")
}
func (s *channelMonitorAdjustRepoStub) ListLatestPerModel(context.Context, int64) ([]*service.ChannelMonitorLatest, error) {
	panic("unexpected ListLatestPerModel call")
}
func (s *channelMonitorAdjustRepoStub) ComputeAvailability(context.Context, int64, int) ([]*service.ChannelMonitorAvailability, error) {
	panic("unexpected ComputeAvailability call")
}
func (s *channelMonitorAdjustRepoStub) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*service.ChannelMonitorLatest, error) {
	panic("unexpected ListLatestForMonitorIDs call")
}
func (s *channelMonitorAdjustRepoStub) ComputeAvailabilityForMonitors(context.Context, []int64, int) (map[int64][]*service.ChannelMonitorAvailability, error) {
	panic("unexpected ComputeAvailabilityForMonitors call")
}
func (s *channelMonitorAdjustRepoStub) ListRecentHistoryForMonitors(context.Context, []int64, map[int64]string, int) (map[int64][]*service.ChannelMonitorHistoryEntry, error) {
	panic("unexpected ListRecentHistoryForMonitors call")
}
func (s *channelMonitorAdjustRepoStub) UpsertDailyRollupsFor(context.Context, time.Time) (int64, error) {
	panic("unexpected UpsertDailyRollupsFor call")
}
func (s *channelMonitorAdjustRepoStub) DeleteRollupsBefore(context.Context, time.Time) (int64, error) {
	panic("unexpected DeleteRollupsBefore call")
}
func (s *channelMonitorAdjustRepoStub) LoadAggregationWatermark(context.Context) (*time.Time, error) {
	panic("unexpected LoadAggregationWatermark call")
}
func (s *channelMonitorAdjustRepoStub) UpdateAggregationWatermark(context.Context, time.Time) error {
	panic("unexpected UpdateAggregationWatermark call")
}
func (s *channelMonitorAdjustRepoStub) GetTemplateByID(context.Context, int64) (*service.ChannelMonitorRequestTemplate, error) {
	panic("unexpected GetTemplateByID call")
}

type channelMonitorAdjustEncryptorStub struct{}

func (channelMonitorAdjustEncryptorStub) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}
func (channelMonitorAdjustEncryptorStub) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
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

func TestChannelMonitorHandler_CreateAcceptsKiroProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &channelMonitorAdjustRepoStub{}
	repo.createFn = func(ctx context.Context, monitor *service.ChannelMonitor) error {
		require.Equal(t, service.MonitorProviderKiro, monitor.Provider)
		require.Equal(t, "kiro(pro号池)", monitor.GroupName)
		monitor.ID = 25
		monitor.CreatedAt = time.Now()
		monitor.UpdatedAt = monitor.CreatedAt
		return nil
	}
	svc := service.NewChannelMonitorService(repo, channelMonitorAdjustEncryptorStub{})
	h := NewChannelMonitorHandler(svc)

	router := gin.New()
	router.POST("/api/v1/admin/channel-monitors", h.Create)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/channel-monitors",
		bytes.NewBufferString(`{
			"name":"kiro(pro号池)",
			"provider":"kiro",
			"endpoint":"https://8.8.8.8",
			"api_key":"sk-test-monitor-key",
			"primary_model":"claude-haiku-4-5",
			"group_name":"kiro(pro号池)",
			"enabled":true,
			"interval_seconds":300
		}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		Data channelMonitorResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, service.MonitorProviderKiro, body.Data.Provider)
	require.Equal(t, "kiro(pro号池)", body.Data.GroupName)
}

func TestChannelMonitorHandler_CreateAcceptsGrokProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &channelMonitorAdjustRepoStub{}
	repo.createFn = func(ctx context.Context, monitor *service.ChannelMonitor) error {
		require.Equal(t, service.MonitorProviderGrok, monitor.Provider)
		require.Equal(t, "grok(supergrok号池)", monitor.GroupName)
		monitor.ID = 26
		monitor.CreatedAt = time.Now()
		monitor.UpdatedAt = monitor.CreatedAt
		return nil
	}
	svc := service.NewChannelMonitorService(repo, channelMonitorAdjustEncryptorStub{})
	h := NewChannelMonitorHandler(svc)

	router := gin.New()
	router.POST("/api/v1/admin/channel-monitors", h.Create)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/channel-monitors",
		bytes.NewBufferString(`{
			"name":"grok(supergrok号池)",
			"provider":"grok",
			"endpoint":"https://8.8.8.8",
			"api_key":"xai-test-monitor-key",
			"primary_model":"grok-4.3",
			"group_name":"grok(supergrok号池)",
			"enabled":true,
			"interval_seconds":300
		}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body struct {
		Data channelMonitorResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, service.MonitorProviderGrok, body.Data.Provider)
	require.Equal(t, "grok(supergrok号池)", body.Data.GroupName)
}

func TestChannelMonitorHandler_AdjustAvailability7dAcceptsZero(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &channelMonitorAdjustRepoStub{
		monitor: &service.ChannelMonitor{
			ID:           42,
			PrimaryModel: "claude-sonnet-4",
		},
	}
	svc := service.NewChannelMonitorService(repo, channelMonitorAdjustEncryptorStub{})
	h := NewChannelMonitorHandler(svc)

	router := gin.New()
	router.POST("/api/v1/admin/channel-monitors/:id/availability-7d", h.AdjustAvailability7d)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/channel-monitors/42/availability-7d",
		bytes.NewBufferString(`{"availability_pct":0}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 0.0, repo.requested)

	var body struct {
		Data service.ChannelMonitorAvailabilityAdjustResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0.0, body.Data.RequestedAvailabilityPct)
	require.Equal(t, 7, body.Data.ChangedRows)
}
