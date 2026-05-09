package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIWebProfileResolveReturnsNilForEmptyExtra(t *testing.T) {
	require.Nil(t, ResolveOpenAIWebProfile(nil))
	require.Nil(t, ResolveOpenAIWebProfile(&Account{}))
	require.Nil(t, ResolveOpenAIWebProfile(&Account{Extra: map[string]any{}}))
}

func TestOpenAIWebProfileResolveJSONBMapAndHeaders(t *testing.T) {
	account := &Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"version":                    "2026-04-29",
			"source":                     "capture",
			"captured_at":                "2026-04-29T12:34:56Z",
			"proxy_id":                   float64(42),
			"proxy_hash":                 "proxy-hash",
			"user_agent":                 "Mozilla/5.0 Profile",
			"impersonate":                "chrome136",
			"accept_language":            "en-US,en;q=0.9",
			"sec_ch_ua":                  `"Chromium";v="136"`,
			"sec_ch_ua_mobile":           "?0",
			"sec_ch_ua_platform":         `"macOS"`,
			"sec_ch_ua_arch":             `"arm"`,
			"sec_ch_ua_bitness":          `"64"`,
			"sec_ch_ua_full_version":     `"136.0.0.0"`,
			"sec_ch_ua_platform_version": `"15.4.0"`,
			"oai_language":               "zh-CN",
			"oai_client_version":         "prod-c9d58bd082f5fe5163759750852e4d690d489633",
			"oai_client_build_number":    "6445842",
			"x_oai_is":                   "ois1.initial",
			"timezone":                   "Asia/Shanghai",
			"viewport": map[string]any{
				"width":               float64(1440),
				"height":              float64(900),
				"device_scale_factor": float64(2),
			},
			"oai_device_id":  "device-123",
			"oai_session_id": "session-456",
			"cookies": []any{
				map[string]any{
					"name":      "__Host-next-auth.csrf-token",
					"value":     "csrf",
					"domain":    "chatgpt.com",
					"path":      "/",
					"secure":    true,
					"expires":   float64(1893456000),
					"httponly":  true,
					"same_site": "Lax",
				},
			},
		},
	}}

	profile := ResolveOpenAIWebProfile(account)
	require.NotNil(t, profile)
	require.Equal(t, "2026-04-29", profile.Version)
	require.Equal(t, "capture", profile.Source)
	require.Equal(t, "2026-04-29T12:34:56Z", profile.CapturedAt)
	require.Equal(t, int64(42), profile.ProxyID)
	require.Equal(t, "proxy-hash", profile.ProxyHash)
	require.Equal(t, "chrome136", profile.Impersonate)
	require.Equal(t, "zh-CN", profile.OAILanguage)
	require.Equal(t, "prod-c9d58bd082f5fe5163759750852e4d690d489633", profile.OAIClientVersion)
	require.Equal(t, "6445842", profile.OAIClientBuildNumber)
	require.Equal(t, "ois1.initial", profile.XOAIIS)
	require.Equal(t, "Asia/Shanghai", profile.Timezone)
	require.Equal(t, 1440, profile.Viewport.Width)
	require.Equal(t, 900, profile.Viewport.Height)
	require.Equal(t, 2.0, profile.Viewport.DeviceScaleFactor)
	require.Equal(t, "device-123", profile.OAIDeviceID)
	require.Equal(t, "session-456", profile.OAISessionID)
	require.Len(t, profile.Cookies, 1)
	require.True(t, profile.Cookies[0].HTTPOnly)
	require.Equal(t, "Lax", profile.Cookies[0].SameSite)

	headers := make(http.Header)
	ApplyOpenAIWebProfileHeaders(headers, profile)
	require.Equal(t, "Mozilla/5.0 Profile", headers.Get("User-Agent"))
	require.Equal(t, "en-US,en;q=0.9", headers.Get("Accept-Language"))
	require.Equal(t, `"Chromium";v="136"`, headers.Get("Sec-Ch-Ua"))
	require.Equal(t, "?0", headers.Get("Sec-Ch-Ua-Mobile"))
	require.Equal(t, `"macOS"`, headers.Get("Sec-Ch-Ua-Platform"))
	require.Equal(t, `"arm"`, headers.Get("Sec-Ch-Ua-Arch"))
	require.Equal(t, `"64"`, headers.Get("Sec-Ch-Ua-Bitness"))
	require.Equal(t, `"136.0.0.0"`, headers.Get("Sec-Ch-Ua-Full-Version"))
	require.Equal(t, `"15.4.0"`, headers.Get("Sec-Ch-Ua-Platform-Version"))
	require.Equal(t, "zh-CN", headers.Get("OAI-Language"))
	require.Equal(t, "prod-c9d58bd082f5fe5163759750852e4d690d489633", headers.Get("OAI-Client-Version"))
	require.Equal(t, "6445842", headers.Get("OAI-Client-Build-Number"))
	require.Equal(t, "ois1.initial", headers.Get("X-OAI-IS"))
}

func TestOpenAIWebProfileCookieHeaderForHostMatchesDomains(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"cookies": []any{
				map[string]any{"name": "chatgpt_parent", "value": "a", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "chatgpt_exact", "value": "b", "domain": "chatgpt.com", "path": "/"},
				map[string]any{"name": "openai_parent", "value": "c", "domain": ".openai.com", "path": "/"},
				map[string]any{"name": "other", "value": "d", "domain": "example.com", "path": "/"},
				map[string]any{"name": "path_only", "value": "e", "domain": ".chatgpt.com", "path": "/backend-api"},
			},
		},
	}})
	require.NotNil(t, profile)

	require.Equal(t, "chatgpt_parent=a; chatgpt_exact=b", profile.CookieHeaderForHost("chatgpt.com"))
	require.Equal(t, "chatgpt_parent=a; chatgpt_exact=b", profile.CookieHeaderForHost("sub.chatgpt.com"))
	require.Equal(t, "openai_parent=c", profile.CookieHeaderForHost("api.openai.com"))
	require.Equal(t, "", profile.CookieHeaderForHost("unmatched.test"))
	require.Equal(t, "chatgpt_parent=a; chatgpt_exact=b; path_only=e", profile.CookieHeaderForHost("chatgpt.com/backend-api/conversation"))
}

func TestOpenAIWebProfileParseCookiesFiltersInvalidNameValueAndExpired(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"cookies": []any{
				map[string]any{"name": "valid", "value": "ok", "domain": ".chatgpt.com", "path": "/", "expires": time.Now().Add(time.Hour).Unix()},
				map[string]any{"name": "", "value": "empty-name", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "bad\nname", "value": "ctl-name", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "bad;name", "value": "semicolon-name", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "bad_value", "value": "bad\rvalue", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "bad_semicolon", "value": "bad;value", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "expired", "value": "old", "domain": ".chatgpt.com", "path": "/", "expires": time.Now().Add(-time.Hour).Unix()},
			},
		},
	}})
	require.NotNil(t, profile)
	require.Equal(t, "valid=ok", profile.CookieHeaderForHost("chatgpt.com"))
}

func TestOpenAIWebProfileParseCookiesFiltersUnrelatedDomains(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"cookies": []any{
				map[string]any{"name": "chatgpt", "value": "a", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "openai", "value": "b", "domain": ".openai.com", "path": "/"},
				map[string]any{"name": "chat_openai", "value": "c", "domain": "chat.openai.com", "path": "/"},
				map[string]any{"name": "unrelated", "value": "d", "domain": "example.com", "path": "/"},
				map[string]any{"name": "lookalike", "value": "e", "domain": "evilchatgpt.com", "path": "/"},
			},
		},
	}})
	require.NotNil(t, profile)
	require.Equal(t, "chatgpt=a", profile.CookieHeaderForHost("chatgpt.com"))
	require.Equal(t, "openai=b", profile.CookieHeaderForHost("api.openai.com"))
	require.Equal(t, "chatgpt=a; openai=b; chat_openai=c", profile.CookieHeaderForHost("chat.openai.com"))
	require.Empty(t, profile.CookieHeaderForHost("example.com"))
	require.Len(t, profile.Cookies, 3)
}

func TestOpenAIWebProfileHeadersSkipMissingValues(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"accept_language": "en-US,en;q=0.9",
		},
	}})
	require.NotNil(t, profile)

	headers := make(http.Header)
	ApplyOpenAIWebProfileHeaders(headers, profile)
	require.Empty(t, headers.Values("User-Agent"))
	require.Empty(t, headers.Values("Sec-Ch-Ua"))
	require.Equal(t, "en-US,en;q=0.9", headers.Get("Accept-Language"))
}

func TestOpenAIWebProfileMergeResponseCookies(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"source":      "capture",
			"captured_at": "2026-04-29T10:00:00Z",
			"cookies": []any{
				map[string]any{"name": "old", "value": "a", "domain": ".chatgpt.com", "path": "/"},
				map[string]any{"name": "session", "value": "old", "domain": ".chatgpt.com", "path": "/"},
			},
		},
	}})
	req, err := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/f/conversation", nil)
	require.NoError(t, err)
	resp := &http.Response{Request: req, Header: http.Header{}}
	resp.Header.Add("Set-Cookie", "session=new; Path=/; Domain=.chatgpt.com; Secure; HttpOnly; SameSite=Lax")
	resp.Header.Add("Set-Cookie", "scoped=value; Path=/backend-api; Secure")

	merged, changed := MergeOpenAIWebProfileResponseCookies(profile, resp)
	require.True(t, changed)
	require.Equal(t, "capture", merged.Source)
	require.Equal(t, "2026-04-29T10:00:00Z", merged.CapturedAt)
	require.Equal(t, "old=a; session=new; scoped=value", merged.CookieHeaderForHost("chatgpt.com/backend-api/f/conversation"))
	require.Equal(t, "old=a; session=new", merged.CookieHeaderForHost("chatgpt.com/"))
}

func TestOpenAIWebProfileMergeResponseCookiesDefaultsDomainAndPath(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"cookies": []any{
				map[string]any{"name": "old", "value": "a", "domain": ".chatgpt.com", "path": "/"},
			},
		},
	}})
	req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/files/process_upload_stream", nil)
	require.NoError(t, err)
	resp := &http.Response{Request: req, Header: http.Header{}}
	resp.Header.Add("Set-Cookie", "scoped=value; Secure; HttpOnly")

	merged, changed := MergeOpenAIWebProfileResponseCookies(profile, resp)
	require.True(t, changed)
	require.Equal(t, "old=a; scoped=value", merged.CookieHeaderForHost("chatgpt.com/backend-api/files/process_upload_stream"))
	require.Equal(t, "old=a", merged.CookieHeaderForHost("chatgpt.com/"))
}

func TestOpenAIWebProfileMergeResponseStateAppliesXOAIISUpdate(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"x_oai_is": "ois1.initial",
			"cookies": []any{
				map[string]any{"name": "old", "value": "a", "domain": ".chatgpt.com", "path": "/"},
			},
		},
	}})
	req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/f/conversation", nil)
	require.NoError(t, err)
	resp := &http.Response{Request: req, Header: http.Header{}}
	resp.Header.Set("x-oai-is-update", "ois1.updated")
	resp.Header.Add("Set-Cookie", "session=new; Path=/; Domain=.chatgpt.com; Secure; HttpOnly; SameSite=Lax")

	merged, changed := MergeOpenAIWebProfileResponseState(profile, resp)
	require.True(t, changed)
	require.Equal(t, "ois1.updated", merged.XOAIIS)
	require.Equal(t, "old=a; session=new", merged.CookieHeaderForHost("chatgpt.com"))
}

func TestOpenAIWebProfileMergeResponseStateAppliesClientVersionAndBuildNumber(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"oai_client_version":      "prod-c9d58bd082f5fe5163759750852e4d690d489633",
			"oai_client_build_number": "6445842",
		},
	}})
	req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/sentinel/chat-requirements/finalize", nil)
	require.NoError(t, err)
	resp := &http.Response{Request: req, Header: http.Header{}}
	resp.Header.Set("oai-client-version", "prod-next-version")
	resp.Header.Set("oai-client-build-number", "7000000")

	merged, changed := MergeOpenAIWebProfileResponseState(profile, resp)
	require.True(t, changed)
	require.Equal(t, "prod-next-version", merged.OAIClientVersion)
	require.Equal(t, "7000000", merged.OAIClientBuildNumber)
}

func TestOpenAIWebProfileMergeResponseCookiesRejectsNonChatGPTRequestHost(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"cookies": []any{
				map[string]any{"name": "old", "value": "a", "domain": ".chatgpt.com", "path": "/"},
			},
		},
	}})
	req, err := http.NewRequest(http.MethodGet, "https://example.com/backend-api/f/conversation", nil)
	require.NoError(t, err)
	resp := &http.Response{Request: req, Header: http.Header{}}
	resp.Header.Add("Set-Cookie", "session=new; Path=/; Domain=.chatgpt.com; Secure; HttpOnly")

	merged, changed := MergeOpenAIWebProfileResponseCookies(profile, resp)
	require.False(t, changed)
	require.Same(t, profile, merged)
	require.Equal(t, "old=a", merged.CookieHeaderForHost("chatgpt.com"))
}

func TestOpenAIWebProfileMergeResponseCookiesRejectsMismatchedDomain(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"cookies": []any{
				map[string]any{"name": "old", "value": "a", "domain": ".chatgpt.com", "path": "/"},
			},
		},
	}})
	req, err := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/f/conversation", nil)
	require.NoError(t, err)
	resp := &http.Response{Request: req, Header: http.Header{}}
	resp.Header.Add("Set-Cookie", "other=new; Path=/; Domain=.example.com; Secure; HttpOnly")
	resp.Header.Add("Set-Cookie", "parent=new; Path=/; Domain=.openai.com; Secure; HttpOnly")

	merged, changed := MergeOpenAIWebProfileResponseCookies(profile, resp)
	require.False(t, changed)
	require.Same(t, profile, merged)
	require.Equal(t, "old=a", merged.CookieHeaderForHost("chatgpt.com"))
}

func TestOpenAIWebProfileMergeResponseCookiesRejectsExpiredCookies(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"cookies": []any{
				map[string]any{"name": "old", "value": "a", "domain": ".chatgpt.com", "path": "/"},
			},
		},
	}})
	req, err := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/f/conversation", nil)
	require.NoError(t, err)
	resp := &http.Response{Request: req, Header: http.Header{}}
	resp.Header.Add("Set-Cookie", "session=deleted; Path=/; Domain=.chatgpt.com; Max-Age=0")
	resp.Header.Add("Set-Cookie", "expired=old; Path=/; Domain=.chatgpt.com; Expires=Thu, 01 Jan 1970 00:00:00 GMT")

	merged, changed := MergeOpenAIWebProfileResponseCookies(profile, resp)
	require.False(t, changed)
	require.Same(t, profile, merged)
	require.Equal(t, "old=a", merged.CookieHeaderForHost("chatgpt.com"))
}

func TestOpenAIWebProfileToExtraMapRedactsNothingButPreservesShape(t *testing.T) {
	profile := &OpenAIWebProfile{
		Version:      "1",
		Source:       "test",
		CapturedAt:   "2026-04-29T10:00:00Z",
		UserAgent:    "Mozilla/5.0",
		OAIDeviceID:  "device",
		OAISessionID: "session",
		Cookies: []OpenAIWebProfileCookie{
			{Name: "cookie", Value: "value", Domain: ".chatgpt.com", Path: "/", Secure: true, HTTPOnly: true, SameSite: "Lax"},
		},
	}

	extra := profile.ToExtraMap()
	require.Equal(t, "1", extra["version"])
	require.Equal(t, "Mozilla/5.0", extra["user_agent"])
	require.Equal(t, "device", extra["oai_device_id"])
	require.Equal(t, "session", extra["oai_session_id"])
	require.Equal(t, "prod-c9d58bd082f5fe5163759750852e4d690d489633", extra["oai_client_version"])
	require.Equal(t, "6445842", extra["oai_client_build_number"])
	require.Equal(t, "ois1.initial", extra["x_oai_is"])
	cookies, ok := extra["cookies"].([]map[string]any)
	require.True(t, ok)
	require.Equal(t, "cookie", cookies[0]["name"])
	require.Equal(t, "value", cookies[0]["value"])
	require.Equal(t, true, cookies[0]["secure"])
}

func TestOpenAIWebProfileReturnsOAIDeviceAndSessionIDs(t *testing.T) {
	profile := ResolveOpenAIWebProfile(&Account{Extra: map[string]any{
		"web_profile": map[string]any{
			"oai_device_id":  "device-id",
			"oai_session_id": "session-id",
		},
	}})
	require.NotNil(t, profile)
	require.Equal(t, "device-id", profile.OAIDeviceID)
	require.Equal(t, "session-id", profile.OAISessionID)
}

func TestNormalizeOpenAIWebProfileExtraBuildsFromCredentialsAndCookies(t *testing.T) {
	extra := NormalizeOpenAIWebProfileExtra(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		"user_agent":         "Mozilla/5.0 OAuth",
		"accept_language":    "en-US,en;q=0.9",
		"sec_ch_ua":          `"Chromium";v="136"`,
		"chatgpt_account_id": "credential-account-id",
	}, map[string]any{
		"cookies": []any{
			map[string]any{"name": "oai-did", "value": "device-cookie", "domain": ".chatgpt.com", "path": "/"},
		},
	})

	profile := ResolveOpenAIWebProfile(&Account{Extra: extra})
	require.NotNil(t, profile)
	require.Equal(t, "Mozilla/5.0 OAuth", profile.UserAgent)
	require.Equal(t, "en-US,en;q=0.9", profile.AcceptLanguage)
	require.Equal(t, `"Chromium";v="136"`, profile.SecCHUA)
	require.Equal(t, "device-cookie", profile.OAIDeviceID)
	require.Len(t, profile.Cookies, 1)
	webProfile, ok := extra["web_profile"].(map[string]any)
	require.True(t, ok)
	_, hasAccountID := webProfile["chatgpt_account_id"]
	require.False(t, hasAccountID)
}

func TestNormalizeOpenAIWebProfileExtraUsesStorageStateCookies(t *testing.T) {
	extra := NormalizeOpenAIWebProfileExtra(PlatformOpenAI, AccountTypeOAuth, nil, map[string]any{
		"storage_state": map[string]any{
			"cookies": []any{
				map[string]any{"name": "oai-did", "value": "device-storage", "domain": ".chatgpt.com", "path": "/"},
			},
		},
	})

	profile := ResolveOpenAIWebProfile(&Account{Extra: extra})
	require.NotNil(t, profile)
	require.Equal(t, "device-storage", profile.OAIDeviceID)
	require.Len(t, profile.Cookies, 1)
}

func TestNormalizeOpenAIWebProfileExtraPreservesUnknownWebProfileFields(t *testing.T) {
	extra := NormalizeOpenAIWebProfileExtra(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		"user_agent": "Mozilla/5.0 Fresh",
	}, map[string]any{
		"web_profile": map[string]any{
			"version":             "1",
			"user_agent":          "Mozilla/5.0 Old",
			"custom_fingerprint":  "fp-123",
			"chatgpt_account_id":  "legacy-chatgpt-account",
			"account_id":          "legacy-account",
			"storage_state":       map[string]any{"origins": []any{map[string]any{"origin": "https://chatgpt.com"}}},
			"experimental_config": map[string]any{"enabled": true},
		},
	})

	webProfile, ok := extra["web_profile"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fp-123", webProfile["custom_fingerprint"])
	require.Equal(t, map[string]any{"origins": []any{map[string]any{"origin": "https://chatgpt.com"}}}, webProfile["storage_state"])
	require.Equal(t, map[string]any{"enabled": true}, webProfile["experimental_config"])
	require.Equal(t, "Mozilla/5.0 Old", webProfile["user_agent"])
	require.NotContains(t, webProfile, "chatgpt_account_id")
	require.NotContains(t, webProfile, "account_id")
}
