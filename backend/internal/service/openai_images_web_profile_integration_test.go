package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIImagesWebProfileBackendHeadersPreserveBrowserProfileContract(t *testing.T) {
	account := &Account{
		ID:       1001,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "access-token",
			"chatgpt_account_id": "credential-account-id",
			"user_agent":         "Mozilla/5.0 CredentialUA Chrome/136.0.0.0 Safari/537.36",
		},
		Extra: map[string]any{
			"openai_device_id":  "device-from-account",
			"openai_session_id": "session-from-account",
			"web_profile": map[string]any{
				"user_agent":                 "Mozilla/5.0 WebProfileUA Chrome/136.0.0.0 Safari/537.36",
				"accept_language":            "en-US,en;q=0.9",
				"sec_ch_ua":                  `"Chromium";v="136", "Google Chrome";v="136"`,
				"sec_ch_ua_mobile":           "?0",
				"sec_ch_ua_platform":         `"macOS"`,
				"sec_ch_ua_arch":             `"arm"`,
				"sec_ch_ua_bitness":          `"64"`,
				"sec_ch_ua_full_version":     `"136.0.0.0"`,
				"sec_ch_ua_platform_version": `"15.4.0"`,
				"oai_device_id":              "device-from-profile",
				"oai_session_id":             "session-from-profile",
				"oai_language":               "zh-CN",
				"oai_client_version":         "prod-c9d58bd082f5fe5163759750852e4d690d489633",
				"oai_client_build_number":    "6445842",
				"x_oai_is":                   "ois1.initial",
				"chatgpt_account_id":         "profile-account-id",
				"cookies": []any{
					map[string]any{"name": "__Secure-next-auth.session-token", "value": "profile-session-secret", "domain": ".chatgpt.com", "path": "/"},
					map[string]any{"name": "cf_clearance", "value": "clearance-secret", "domain": ".chatgpt.com", "path": "/backend-api"},
					map[string]any{"name": "files_only", "value": "files-secret", "domain": ".chatgpt.com", "path": "/backend-api/files"},
					map[string]any{"name": "sentinel_only", "value": "sentinel-secret", "domain": ".chatgpt.com", "path": "/backend-api/sentinel"},
					map[string]any{"name": "async_only", "value": "async-secret", "domain": ".chatgpt.com", "path": "/backend-api/conversation/conv-123/async-status"},
					map[string]any{"name": "api_cookie", "value": "api-secret", "domain": ".openai.com", "path": "/"},
				},
			},
		},
	}

	baseHeaders, err := (&OpenAIGatewayService{}).buildOpenAIBackendAPIHeaders(account, "access-token")
	require.NoError(t, err)

	profile := ResolveOpenAIWebProfile(account)
	require.NotNil(t, profile)
	require.Equal(t, "Bearer access-token", baseHeaders.Get("Authorization"))
	require.Equal(t, "https://chatgpt.com", baseHeaders.Get("Origin"))
	require.Equal(t, "https://chatgpt.com/", baseHeaders.Get("Referer"))
	require.Equal(t, "Mozilla/5.0 WebProfileUA Chrome/136.0.0.0 Safari/537.36", baseHeaders.Get("User-Agent"))
	require.Equal(t, "en-US,en;q=0.9", baseHeaders.Get("Accept-Language"))
	require.Equal(t, `"Chromium";v="136", "Google Chrome";v="136"`, baseHeaders.Get("Sec-Ch-Ua"))
	require.Equal(t, "?0", baseHeaders.Get("Sec-Ch-Ua-Mobile"))
	require.Equal(t, `"macOS"`, baseHeaders.Get("Sec-Ch-Ua-Platform"))
	require.Equal(t, `"arm"`, baseHeaders.Get("Sec-Ch-Ua-Arch"))
	require.Equal(t, `"64"`, baseHeaders.Get("Sec-Ch-Ua-Bitness"))
	require.Equal(t, `"136.0.0.0"`, baseHeaders.Get("Sec-Ch-Ua-Full-Version"))
	require.Equal(t, `"15.4.0"`, baseHeaders.Get("Sec-Ch-Ua-Platform-Version"))
	require.Equal(t, "device-from-profile", baseHeaders.Get("oai-device-id"))
	require.Equal(t, "session-from-profile", baseHeaders.Get("oai-session-id"))
	require.Equal(t, "credential-account-id", baseHeaders.Get("chatgpt-account-id"))
	require.Equal(t, "zh-CN", baseHeaders.Get("OAI-Language"))
	require.Equal(t, "prod-c9d58bd082f5fe5163759750852e4d690d489633", baseHeaders.Get("OAI-Client-Version"))
	require.Equal(t, "6445842", baseHeaders.Get("OAI-Client-Build-Number"))
	require.Equal(t, "ois1.initial", baseHeaders.Get("X-OAI-IS"))
	require.Equal(t, "__Secure-next-auth.session-token=profile-session-secret; cf_clearance=clearance-secret; oai-did=device-from-profile", baseHeaders.Get("Cookie"))
	require.NotContains(t, baseHeaders.Get("Cookie"), "api-secret")

	filesHeaders := cloneHeaderForOpenAIImagesWebProfileTest(baseHeaders)
	setOpenAIBackendAPIRequestTarget(filesHeaders, openAIChatGPTFilesProcessUploadURL, "/backend-api/files/process_upload_stream")
	setOpenAIBackendAPIRequestCookieHeader(filesHeaders, profile, openAIChatGPTFilesProcessUploadURL)
	require.Equal(t, "/backend-api/files/process_upload_stream", filesHeaders.Get("X-OpenAI-Target-Path"))
	require.Contains(t, filesHeaders.Get("Cookie"), "files_only=files-secret")
	require.Contains(t, filesHeaders.Get("Cookie"), "cf_clearance=clearance-secret")
	require.NotContains(t, filesHeaders.Get("Cookie"), "api-secret")

	sentinelHeaders := cloneHeaderForOpenAIImagesWebProfileTest(baseHeaders)
	setOpenAIBackendAPIRequestTarget(sentinelHeaders, openAIChatGPTChatRequirementsPrepareURL, "/backend-api/sentinel/chat-requirements/prepare")
	setOpenAIBackendAPIRequestCookieHeader(sentinelHeaders, profile, openAIChatGPTChatRequirementsPrepareURL)
	require.Equal(t, "/backend-api/sentinel/chat-requirements/prepare", sentinelHeaders.Get("X-OpenAI-Target-Path"))
	require.Contains(t, sentinelHeaders.Get("Cookie"), "sentinel_only=sentinel-secret")
	require.Contains(t, sentinelHeaders.Get("Cookie"), "cf_clearance=clearance-secret")

	asyncURL, asyncPath, asyncRoute, _ := buildOpenAIConversationAsyncStatusRequestTarget("conv-123")
	asyncHeaders := cloneHeaderForOpenAIImagesWebProfileTest(baseHeaders)
	setOpenAIBackendAPIRequestTarget(asyncHeaders, asyncPath, asyncRoute)
	setOpenAIBackendAPIRequestCookieHeader(asyncHeaders, profile, asyncURL)
	require.Contains(t, asyncHeaders.Get("Cookie"), "async_only=async-secret")
	require.Contains(t, asyncHeaders.Get("Cookie"), "cf_clearance=clearance-secret")
}

func TestOpenAIImagesWebProfileTelemetryRecordsChallengeAndRedactsCookieValues(t *testing.T) {
	c := newOpenAIImagesTelemetryTestContext()
	proxyID := int64(9)
	account := &Account{
		ProxyID: &proxyID,
		Proxy:   &Proxy{ID: proxyID, Protocol: "http", Host: "example.test", Port: 8080, Username: "proxy-user", Password: "proxy-pass"},
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(44),
		},
	}
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"source":      "images2api-capture",
			"captured_at": "2026-04-29T10:00:00Z",
			"proxy_id":    9,
			"user_agent":  "Mozilla/5.0 Chrome/136.0.0.0 Safari/537.36",
			"sec_ch_ua":   `"Chromium";v="136"`,
			"proxy_hash":  "proxy-hash",
			"cookies": []any{
				map[string]any{"name": "__Secure-next-auth.session-token", "value": "raw-session-secret", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "cf_clearance", "value": "raw-clearance-secret", "domain": ".chatgpt.com", "path": "/"},
			},
		},
	}})
	require.NotNil(t, profile)

	SetOpenAIImagesTelemetryChallenge(c, OpenAIImagesTelemetryChallengeState{
		ArkoseRequired:           true,
		TurnstileRequired:        true,
		PoWRequired:              true,
		RequirementsTokenPresent: true,
		ProofTokenPresent:        true,
	})
	(&OpenAIGatewayService{}).recordOpenAIImagesLegacyBridgeTelemetry(c, account, profile, account.Proxy.URL())

	telemetry := GetOpenAIImagesTelemetry(c)
	require.NotNil(t, telemetry)
	require.True(t, telemetry.Challenge.ArkoseRequired)
	require.True(t, telemetry.Challenge.TurnstileRequired)
	require.True(t, telemetry.Challenge.PoWRequired)
	require.True(t, telemetry.Challenge.RequirementsTokenPresent)
	require.True(t, telemetry.Challenge.ProofTokenPresent)
	require.True(t, telemetry.Profile.HasWebProfile)
	require.Equal(t, "images2api-capture", telemetry.Profile.ProfileSource)
	require.GreaterOrEqual(t, telemetry.Profile.ProfileAgeHours, int64(0))
	require.True(t, telemetry.Profile.HasCookieJar)
	require.True(t, telemetry.Profile.ProxyMatch)
	require.Equal(t, 136, telemetry.Profile.UAMajor)
	require.True(t, telemetry.Profile.SecCHUAPresent)
	require.NotEmpty(t, telemetry.Profile.CookieNamesDigest)
	require.Equal(t, telemetry.Profile.CookieNamesHash, telemetry.Profile.CookieNamesDigest)
	require.NotContains(t, telemetry.Profile.CookieNamesDigest, "raw-session-secret")
	require.NotContains(t, telemetry.Profile.CookieNamesDigest, "raw-clearance-secret")
	require.NotContains(t, telemetry.Profile.CookieNamesDigest, "__Secure-next-auth.session-token")
	require.NotContains(t, telemetry.Profile.CookieNamesDigest, "cf_clearance")
	require.Equal(t, "req_impersonate", telemetry.Network.TransportKind)
	require.Equal(t, "44", telemetry.Network.TLSProfileID)
	require.Equal(t, "not_applied", telemetry.Network.TLSProfileSource)
	summary := FormatOpenAIImagesTelemetrySummary(telemetry)
	require.Contains(t, summary, "transport_kind=req_impersonate")
	require.Contains(t, summary, "tls_profile_source=not_applied")
	require.NotContains(t, summary, "proxy-user")
	require.NotContains(t, summary, "proxy-pass")
}

func cloneHeaderForOpenAIImagesWebProfileTest(headers http.Header) http.Header {
	cloned := make(http.Header, len(headers))
	for key, values := range headers {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}
