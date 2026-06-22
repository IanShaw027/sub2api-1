package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIImages_SelectionFailure_PreservesCompatibleAccountsMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"message":"No available compatible accounts"`)
}

func TestOpenAIImages_SelectionFailureStoresImageRequestType(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	rt, ok := c.Get(opsRequestTypeKey)
	require.True(t, ok)
	require.Equal(t, int16(service.RequestTypeImage), rt)
}

func TestOpenAIImages_SelectionFailureWritesDebugTimeline(t *testing.T) {
	resetHandlerGatewayDebugTimelineState(t)
	dir := filepath.Join(t.TempDir(), "timeline")
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)
	h.gatewayService = service.NewOpenAIGatewayService(
		openAISelectionErrorAccountRepoStub{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		h.cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		handlerGatewayDebugTimelineSettingService(dir),
		nil,
	)

	h.Images(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	content := readHandlerGatewayDebugTimelineLog(t, dir)
	require.Contains(t, content, `"stage":"request_received"`)
	require.Contains(t, content, `"endpoint_kind":"images"`)
	require.Contains(t, content, `"request_path":"/v1/images/generations"`)
	require.Contains(t, content, `"request_body_bytes":38`)
}

func TestOpenAIImages_GroupModelUnsupported_ReturnsPermissionError(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	h := newOpenAISelectionErrorTestHandler(t, []service.Account{unsupportedOpenAIImagesTestAccount()})

	h.Images(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"gpt-image-1\"`)
	require.Contains(t, rec.Body.String(), "gpt-5.4-mini")
}

func unsupportedOpenAIImagesTestAccount() service.Account {
	account := unsupportedOpenAITestAccount()
	account.Type = service.AccountTypeAPIKey
	return account
}

func TestOpenAIImages_PermissionFailureStoresImageRequestType(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	apiKey.Group.AllowImageGeneration = false
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rt, ok := c.Get(opsRequestTypeKey)
	require.True(t, ok)
	require.Equal(t, int16(service.RequestTypeImage), rt)
}

type handlerGatewayDebugTimelineSettingRepo struct {
	values map[string]string
}

func handlerGatewayDebugTimelineSettingService(dir string) *service.SettingService {
	repo := &handlerGatewayDebugTimelineSettingRepo{values: map[string]string{
		service.SettingKeyGatewayDebugTimelineEnabled:       "true",
		service.SettingKeyGatewayDebugTimelineDirectory:     dir,
		service.SettingKeyGatewayDebugTimelineRetentionDays: "7",
		service.SettingKeyGatewayDebugTimelineMaxSizeMB:     "1024",
	}}
	settingService := service.NewSettingService(repo, nil)
	if err := settingService.UpdateSettings(context.Background(), &service.SystemSettings{
		GatewayDebugTimelineEnabled:       true,
		GatewayDebugTimelineDirectory:     dir,
		GatewayDebugTimelineRetentionDays: 7,
		GatewayDebugTimelineMaxSizeMB:     1024,
		GatewayDebugTimelineBodyMaxKB:     32,
	}); err != nil {
		panic(err)
	}
	return settingService
}

func (r *handlerGatewayDebugTimelineSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *handlerGatewayDebugTimelineSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (r *handlerGatewayDebugTimelineSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *handlerGatewayDebugTimelineSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}

func (r *handlerGatewayDebugTimelineSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *handlerGatewayDebugTimelineSettingRepo) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *handlerGatewayDebugTimelineSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func resetHandlerGatewayDebugTimelineState(t *testing.T) {
	t.Helper()
	service.ResetGatewayDebugTimelineAutoStop()
}

func readHandlerGatewayDebugTimelineLog(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "gateway-timeline-"+time.Now().Format("2006-01-02")+".log")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}
