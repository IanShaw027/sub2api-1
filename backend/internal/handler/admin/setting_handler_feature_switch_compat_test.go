package admin

import (
	"bytes"
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

func TestSettingHandler_UpdateSettings_PreservesOmittedAffiliateAndTicketSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:            "true",
			service.SettingKeyAffiliateEnabled:            "true",
			service.SettingKeyAffiliateRebateRate:         "37.50000000",
			service.SettingKeyAffiliateRebateCap:          "120.50000000",
			service.SettingKeyAffiliateRebateInviteeLimit: "9",
			service.SettingKeyAffiliateSignupBonus:        "18.80000000",
			service.SettingKeyTicketEnabled:               "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled": false,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyAffiliateEnabled])
	require.Equal(t, "37.50000000", repo.values[service.SettingKeyAffiliateRebateRate])
	require.Equal(t, "120.50000000", repo.values[service.SettingKeyAffiliateRebateCap])
	require.Equal(t, "9", repo.values[service.SettingKeyAffiliateRebateInviteeLimit])
	require.Equal(t, "18.80000000", repo.values[service.SettingKeyAffiliateSignupBonus])
	require.Equal(t, "true", repo.values[service.SettingKeyTicketEnabled])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["affiliate_enabled"])
	require.Equal(t, 37.5, data["affiliate_rebate_rate"])
	require.Equal(t, 120.5, data["affiliate_rebate_cap"])
	require.Equal(t, float64(9), data["affiliate_rebate_invitee_limit"])
	require.Equal(t, 18.8, data["affiliate_signup_bonus"])
	require.Equal(t, true, data["ticket_enabled"])
}

func TestSettingHandler_UpdateSettings_ClampsAffiliatePolicyFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:            "true",
			service.SettingKeyAffiliateEnabled:            "true",
			service.SettingKeyAffiliateRebateRate:         "25.00000000",
			service.SettingKeyAffiliateRebateCap:          "50.00000000",
			service.SettingKeyAffiliateRebateInviteeLimit: "7",
			service.SettingKeyAffiliateSignupBonus:        "5.00000000",
			service.SettingKeyTicketEnabled:               "false",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled":             true,
		"affiliate_rebate_rate":          999,
		"affiliate_rebate_cap":           -12.5,
		"affiliate_rebate_invitee_limit": -3,
		"affiliate_signup_bonus":         -8.75,
		"affiliate_enabled":              false,
		"ticket_enabled":                 true,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "false", repo.values[service.SettingKeyAffiliateEnabled])
	require.Equal(t, "100.00000000", repo.values[service.SettingKeyAffiliateRebateRate])
	require.Equal(t, "0.00000000", repo.values[service.SettingKeyAffiliateRebateCap])
	require.Equal(t, "0", repo.values[service.SettingKeyAffiliateRebateInviteeLimit])
	require.Equal(t, "0.00000000", repo.values[service.SettingKeyAffiliateSignupBonus])
	require.Equal(t, "true", repo.values[service.SettingKeyTicketEnabled])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, data["affiliate_enabled"])
	require.Equal(t, 100.0, data["affiliate_rebate_rate"])
	require.Equal(t, 0.0, data["affiliate_rebate_cap"])
	require.Equal(t, float64(0), data["affiliate_rebate_invitee_limit"])
	require.Equal(t, 0.0, data["affiliate_signup_bonus"])
	require.Equal(t, true, data["ticket_enabled"])
}
