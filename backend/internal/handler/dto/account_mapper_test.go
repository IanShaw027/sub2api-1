package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountFromServiceRedactsSensitiveCredentials(t *testing.T) {
	account := &service.Account{Platform: service.PlatformKiro, Credentials: map[string]any{
		"access_token":  "access-secret",
		"Refresh_Token": "mixed-case-refresh-secret",
		"refresh_token": "refresh-secret",
		"api_key":       "sk-secret",
		"client_secret": "client-secret",
		"region":        "us-east-1",
		"auth_region":   "us-east-1",
		"api_region":    "us-east-1",
		"profile_arn":   "arn:aws:codewhisperer:us-east-1:123456789012:profile/PROFILEID",
		"machine_id":    "machine-secret",
		"model_whitelist": []any{
			"claude-sonnet-4.5",
		},
		"model_mapping": map[string]any{"claude-sonnet-4": "claude-sonnet-4"},
	}}

	out := AccountFromService(account)
	require.NotNil(t, out)
	require.NotContains(t, out.Credentials, "access_token")
	require.NotContains(t, out.Credentials, "Refresh_Token")
	require.NotContains(t, out.Credentials, "refresh_token")
	require.NotContains(t, out.Credentials, "api_key")
	require.NotContains(t, out.Credentials, "client_secret")
	require.NotContains(t, out.Credentials, "region")
	require.NotContains(t, out.Credentials, "auth_region")
	require.NotContains(t, out.Credentials, "api_region")
	require.NotContains(t, out.Credentials, "profile_arn")
	require.NotContains(t, out.Credentials, "machine_id")
	require.Contains(t, out.Credentials, "model_whitelist")
	require.Contains(t, out.Credentials, "model_mapping")
}

func TestAccountFromServiceDetailRedactsSensitiveCredentials(t *testing.T) {
	account := &service.Account{Platform: service.PlatformOpenAI, Credentials: map[string]any{
		"access_token":  "access-secret",
		"refresh_token": "refresh-secret",
		"api_key":       "sk-secret",
		"client_secret": "client-secret",
		"base_url":      "https://api.example.com",
	}}

	out := AccountFromServiceDetail(account)
	require.NotNil(t, out)
	require.Equal(t, "https://api.example.com", out.Credentials["base_url"])
	require.NotContains(t, out.Credentials, "access_token")
	require.NotContains(t, out.Credentials, "refresh_token")
	require.NotContains(t, out.Credentials, "api_key")
	require.NotContains(t, out.Credentials, "client_secret")
}

func TestAccountFromServiceExposesOpenAITLSFingerprintConfig(t *testing.T) {
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(42),
			"tls_fingerprint_router_id":  int64(7),
		},
	}

	out := AccountFromService(account)
	require.NotNil(t, out)
	require.NotNil(t, out.EnableTLSFingerprint)
	require.True(t, *out.EnableTLSFingerprint)
	require.NotNil(t, out.TLSFingerprintProfileID)
	require.Equal(t, int64(42), *out.TLSFingerprintProfileID)
	require.NotNil(t, out.TLSFingerprintRouterID)
	require.Equal(t, int64(7), *out.TLSFingerprintRouterID)
}

func TestAccountFromServiceRedactsOpenAIWebProfileCookieValues(t *testing.T) {
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			"web_profile": map[string]any{
				"user_agent": "Mozilla/5.0 Chrome/136.0.0.0",
				"cookies": []map[string]any{
					{"name": "oai-did", "value": "cookie-secret", "domain": ".chatgpt.com", "path": "/"},
				},
			},
		},
	}

	out := AccountFromService(account)
	require.NotNil(t, out)
	profile, ok := out.Extra["web_profile"].(map[string]any)
	require.True(t, ok)
	cookies, ok := profile["cookies"].([]map[string]any)
	require.True(t, ok)
	require.Equal(t, "oai-did", cookies[0]["name"])
	require.Equal(t, ".chatgpt.com", cookies[0]["domain"])
	require.NotContains(t, cookies[0], "value")
}

func TestAccountFromServiceDetailRedactsOpenAIWebProfileCookiesAndPreservesGroups(t *testing.T) {
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			"web_profile": map[string]any{
				"source": "capture",
				"cookies": []any{
					map[string]any{
						"name":   "oai-did",
						"value":  "cookie-secret",
						"domain": ".chatgpt.com",
						"path":   "/",
					},
				},
			},
		},
		Groups: []*service.Group{
			{
				ID:       17,
				Name:     "openai-group",
				Platform: service.PlatformOpenAI,
			},
		},
	}

	out := AccountFromServiceDetail(account)
	require.NotNil(t, out)

	profile, ok := out.Extra["web_profile"].(map[string]any)
	require.True(t, ok)
	cookies, ok := profile["cookies"].([]any)
	require.True(t, ok)
	cookie, ok := cookies[0].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, cookie, "value")

	require.Len(t, out.Groups, 1)
	require.Equal(t, int64(17), out.Groups[0].ID)
}

func TestAccountFromServiceKeepsGeminiCanonicalTierMetadata(t *testing.T) {
	account := &service.Account{
		Platform: service.PlatformGemini,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"tier_id":   "google_ai_pro",
			"plan_name": "Gemini Code Assist in Google One AI Pro",
		},
		Extra: map[string]any{
			"subscription_type":      "Gemini Code Assist in Google One AI Pro",
			"gemini_current_tier_id": "g1-pro-tier",
			"gemini_paid_tier_id":    "g1-pro-tier",
		},
	}

	out := AccountFromService(account)
	require.NotNil(t, out)
	require.Equal(t, "google_ai_pro", out.Credentials["tier_id"])
	require.Equal(t, "Gemini Code Assist in Google One AI Pro", out.Credentials["plan_name"])
	require.Equal(t, "Gemini Code Assist in Google One AI Pro", out.Extra["subscription_type"])
	require.Equal(t, "g1-pro-tier", out.Extra["gemini_current_tier_id"])
	require.Equal(t, "g1-pro-tier", out.Extra["gemini_paid_tier_id"])
}

func TestAccountFromServiceExposesOpenAIImageGenerationEnabledField(t *testing.T) {
	t.Run("missing key defaults to true", func(t *testing.T) {
		account := &service.Account{Platform: service.PlatformOpenAI}

		out := AccountFromService(account)
		require.NotNil(t, out)
		require.True(t, out.OpenAIImageGenerationEnabled)
	})

	t.Run("explicit false is exposed as false", func(t *testing.T) {
		account := &service.Account{
			Platform: service.PlatformOpenAI,
			Extra: map[string]any{
				"openai_image_generation_enabled": false,
			},
		}

		out := AccountFromService(account)
		require.NotNil(t, out)
		require.False(t, out.OpenAIImageGenerationEnabled)
	})
}
