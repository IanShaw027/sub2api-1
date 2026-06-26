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

func TestSettingHandler_UpdateSettings_PreservesOmittedFieldsOnGatewayDebugTimelinePatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyLinuxDoConnectEnabled:           "true",
			service.SettingKeyLinuxDoConnectClientID:          "linuxdo-client",
			service.SettingKeyLinuxDoConnectClientSecret:      "linuxdo-secret",
			service.SettingKeyLinuxDoConnectRedirectURL:       "https://example.com/auth/linuxdo/callback",
			service.SettingKeyEnableModelFallback:             "true",
			service.SettingKeyFallbackModelAnthropic:          "claude-sonnet-custom",
			service.SettingKeyFallbackModelOpenAI:             "gpt-custom",
			service.SettingKeyFallbackModelGemini:             "gemini-custom",
			service.SettingKeyFallbackModelAntigravity:        "anti-custom",
			service.SettingKeySiteName:                        "Custom Site",
			service.SettingKeySiteSubtitle:                    "Custom Subtitle",
			service.SettingKeyOIDCConnectEnabled:              "true",
			service.SettingKeyOIDCConnectProviderName:         "AcmeID",
			service.SettingKeyOIDCConnectClientID:             "oidc-client",
			service.SettingKeyOIDCConnectClientSecret:         "oidc-secret",
			service.SettingKeyOIDCConnectIssuerURL:            "https://issuer.example.com",
			service.SettingKeyOIDCConnectRedirectURL:          "https://example.com/api/v1/auth/oauth/oidc/callback",
			service.SettingKeyOIDCConnectScopes:               "openid profile groups",
			service.SettingKeyOIDCConnectFrontendRedirectURL:  "/auth/oidc/custom-callback",
			service.SettingKeyOIDCConnectTokenAuthMethod:      "client_secret_basic",
			service.SettingKeyOIDCConnectAllowedSigningAlgs:   "ES256",
			service.SettingKeyGatewayDebugTimelineIncludeBody: "true",
			service.SettingKeyGatewayDebugTimelineBodyMaxKB:   "128",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"gateway_debug_timeline_include_body": false,
		"gateway_debug_timeline_body_max_kb":  512,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyLinuxDoConnectEnabled])
	require.Equal(t, "linuxdo-client", repo.values[service.SettingKeyLinuxDoConnectClientID])
	require.Equal(t, "linuxdo-secret", repo.values[service.SettingKeyLinuxDoConnectClientSecret])
	require.Equal(t, "https://example.com/auth/linuxdo/callback", repo.values[service.SettingKeyLinuxDoConnectRedirectURL])
	require.Equal(t, "true", repo.values[service.SettingKeyEnableModelFallback])
	require.Equal(t, "claude-sonnet-custom", repo.values[service.SettingKeyFallbackModelAnthropic])
	require.Equal(t, "gpt-custom", repo.values[service.SettingKeyFallbackModelOpenAI])
	require.Equal(t, "gemini-custom", repo.values[service.SettingKeyFallbackModelGemini])
	require.Equal(t, "anti-custom", repo.values[service.SettingKeyFallbackModelAntigravity])
	require.Equal(t, "Custom Site", repo.values[service.SettingKeySiteName])
	require.Equal(t, "Custom Subtitle", repo.values[service.SettingKeySiteSubtitle])
	require.Equal(t, "true", repo.values[service.SettingKeyOIDCConnectEnabled])
	require.Equal(t, "AcmeID", repo.values[service.SettingKeyOIDCConnectProviderName])
	require.Equal(t, "oidc-client", repo.values[service.SettingKeyOIDCConnectClientID])
	require.Equal(t, "oidc-secret", repo.values[service.SettingKeyOIDCConnectClientSecret])
	require.Equal(t, "openid profile groups", repo.values[service.SettingKeyOIDCConnectScopes])
	require.Equal(t, "/auth/oidc/custom-callback", repo.values[service.SettingKeyOIDCConnectFrontendRedirectURL])
	require.Equal(t, "client_secret_basic", repo.values[service.SettingKeyOIDCConnectTokenAuthMethod])
	require.Equal(t, "ES256", repo.values[service.SettingKeyOIDCConnectAllowedSigningAlgs])
	require.Equal(t, "false", repo.values[service.SettingKeyGatewayDebugTimelineIncludeBody])
	require.Equal(t, "512", repo.values[service.SettingKeyGatewayDebugTimelineBodyMaxKB])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["linuxdo_connect_enabled"])
	require.Equal(t, "linuxdo-client", data["linuxdo_connect_client_id"])
	require.Equal(t, true, data["enable_model_fallback"])
	require.Equal(t, "claude-sonnet-custom", data["fallback_model_anthropic"])
	require.Equal(t, "gpt-custom", data["fallback_model_openai"])
	require.Equal(t, "gemini-custom", data["fallback_model_gemini"])
	require.Equal(t, "anti-custom", data["fallback_model_antigravity"])
	require.Equal(t, "Custom Site", data["site_name"])
	require.Equal(t, "Custom Subtitle", data["site_subtitle"])
	require.Equal(t, true, data["oidc_connect_enabled"])
	require.Equal(t, "AcmeID", data["oidc_connect_provider_name"])
	require.Equal(t, "openid profile groups", data["oidc_connect_scopes"])
	require.Equal(t, "/auth/oidc/custom-callback", data["oidc_connect_frontend_redirect_url"])
	require.Equal(t, "client_secret_basic", data["oidc_connect_token_auth_method"])
	require.Equal(t, "ES256", data["oidc_connect_allowed_signing_algs"])
	require.Equal(t, false, data["gateway_debug_timeline_include_body"])
	require.Equal(t, float64(512), data["gateway_debug_timeline_body_max_kb"])
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
