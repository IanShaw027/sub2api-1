package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_UpdateSettings_AllowsCustomMenuURLFragment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	const customPageURL = "https://mail-admin.example.com/#token=example-token"
	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled": true,
		"custom_menu_items": []map[string]any{
			{
				"id":         "mail-admin",
				"label":      "Mail Admin",
				"icon_svg":   "",
				"url":        customPageURL,
				"visibility": "admin",
				"sort_order": 0,
			},
		},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var items []dto.CustomMenuItem
	require.NoError(t, json.Unmarshal([]byte(repo.values[service.SettingKeyCustomMenuItems]), &items))
	require.Len(t, items, 1)
	require.Equal(t, customPageURL, items[0].URL)
}
