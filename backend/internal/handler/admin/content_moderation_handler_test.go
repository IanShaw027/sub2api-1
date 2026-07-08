package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type contentModerationHandlerSettingRepo struct {
	mu     sync.Mutex
	values map[string]string
}

func (r *contentModerationHandlerSettingRepo) Get(ctx context.Context, key string) (*service.Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if value, ok := r.values[key]; ok {
		return &service.Setting{Key: key, Value: value}, nil
	}
	return nil, service.ErrSettingNotFound
}

func (r *contentModerationHandlerSettingRepo) GetValue(ctx context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (r *contentModerationHandlerSettingRepo) Set(ctx context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *contentModerationHandlerSettingRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *contentModerationHandlerSettingRepo) SetMultiple(ctx context.Context, settings map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = map[string]string{}
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *contentModerationHandlerSettingRepo) GetAll(ctx context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]string{}
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *contentModerationHandlerSettingRepo) Delete(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, key)
	return nil
}

func TestContentModerationHandler_UpdateConfigPersistsDefaultProximityWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &contentModerationHandlerSettingRepo{values: map[string]string{}}
	svc := service.NewContentModerationService(repo, nil, nil, nil, nil, nil, nil)
	handler := NewContentModerationHandler(svc)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/content-moderation/config",
		strings.NewReader(`{"default_proximity_window":40}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateConfig(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var saved service.ContentModerationConfig
	require.NoError(t, json.Unmarshal([]byte(repo.values[service.SettingKeyContentModerationConfig]), &saved))
	require.Equal(t, 40, saved.DefaultProximityWindow)
}
