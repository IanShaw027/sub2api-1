//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandlerListIncludesActivityFieldsAndSortParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastLoginAt := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(30 * time.Minute)
	lastUsedAt := lastLoginAt.Add(90 * time.Minute)

	adminSvc := newStubAdminService()
	adminSvc.users = []service.User{
		{
			ID:           7,
			Email:        "activity@example.com",
			Username:     "activity-user",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			LastLoginAt:  &lastLoginAt,
			LastActiveAt: &lastActiveAt,
			LastUsedAt:   &lastUsedAt,
			CreatedAt:    lastLoginAt.Add(-24 * time.Hour),
			UpdatedAt:    lastLoginAt,
		},
	}
	handler := NewUserHandler(adminSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/users?sort_by=last_used_at&sort_order=asc&search=activity",
		nil,
	)

	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "last_used_at", adminSvc.lastListUsers.sortBy)
	require.Equal(t, "asc", adminSvc.lastListUsers.sortOrder)
	require.Equal(t, "activity", adminSvc.lastListUsers.filters.Search)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				LastLoginAt  *time.Time `json:"last_login_at"`
				LastActiveAt *time.Time `json:"last_active_at"`
				LastUsedAt   *time.Time `json:"last_used_at"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Items, 1)
	require.WithinDuration(t, lastLoginAt, *resp.Data.Items[0].LastLoginAt, time.Second)
	require.WithinDuration(t, lastActiveAt, *resp.Data.Items[0].LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *resp.Data.Items[0].LastUsedAt, time.Second)
}

func TestUserHandlerGetByIDIncludesActivityFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastLoginAt := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(30 * time.Minute)
	lastUsedAt := lastLoginAt.Add(90 * time.Minute)

	adminSvc := newStubAdminService()
	adminSvc.users = []service.User{
		{
			ID:           8,
			Email:        "detail@example.com",
			Username:     "detail-user",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			LastLoginAt:  &lastLoginAt,
			LastActiveAt: &lastActiveAt,
			LastUsedAt:   &lastUsedAt,
			CreatedAt:    lastLoginAt.Add(-24 * time.Hour),
			UpdatedAt:    lastLoginAt,
		},
	}
	handler := NewUserHandler(adminSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "8"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/8", nil)

	handler.GetByID(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			LastLoginAt  *time.Time `json:"last_login_at"`
			LastActiveAt *time.Time `json:"last_active_at"`
			LastUsedAt   *time.Time `json:"last_used_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.WithinDuration(t, lastLoginAt, *resp.Data.LastLoginAt, time.Second)
	require.WithinDuration(t, lastActiveAt, *resp.Data.LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *resp.Data.LastUsedAt, time.Second)
}

func TestUserHandlerGetByIDIncludesAvatarAndIdentityBindings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
	adminSvc := newStubAdminService()
	adminSvc.users = []service.User{
		{
			ID:        9,
			Email:     "profile@example.com",
			Username:  "profile-user",
			AvatarURL: "https://cdn.example.com/user-avatar.png",
			Role:      service.RoleUser,
			Status:    service.StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	adminSvc.identitySummaries = map[int64]service.UserIdentitySummarySet{
		9: {
			LinuxDo: service.UserIdentitySummary{
				Provider:    "linuxdo",
				Bound:       true,
				DisplayName: "LinuxDo Nick",
				AvatarURL:   "https://cdn.example.com/linuxdo-avatar.png",
				SubjectHint: "linuxdo-user",
			},
		},
	}
	handler := NewUserHandler(adminSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "9"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/9", nil)

	handler.GetByID(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			AvatarURL    string `json:"avatar_url"`
			AuthBindings map[string]struct {
				Provider    string `json:"provider"`
				Bound       bool   `json:"bound"`
				DisplayName string `json:"display_name"`
				AvatarURL   string `json:"avatar_url"`
				SubjectHint string `json:"subject_hint"`
			} `json:"auth_bindings"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "https://cdn.example.com/user-avatar.png", resp.Data.AvatarURL)
	require.True(t, resp.Data.AuthBindings["linuxdo"].Bound)
	require.Equal(t, "LinuxDo Nick", resp.Data.AuthBindings["linuxdo"].DisplayName)
	require.Equal(t, "https://cdn.example.com/linuxdo-avatar.png", resp.Data.AuthBindings["linuxdo"].AvatarURL)
	require.Equal(t, "linuxdo-user", resp.Data.AuthBindings["linuxdo"].SubjectHint)
}
