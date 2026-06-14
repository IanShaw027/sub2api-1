package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAICodexIdentityExtraDropsMismatchedIdentityAndLegacyWebProfile(t *testing.T) {
	extra := NormalizeOpenAICodexIdentityExtra(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		"email":              "current@example.com",
		"chatgpt_account_id": "acct-current",
	}, map[string]any{
		"email":                   "old@example.com",
		"chatgpt_account_id":      "acct-old",
		"web_profile":             map[string]any{"user_agent": "UA"},
		"cookies":                 []any{map[string]any{"name": "session", "value": "secret"}},
		"oai_device_id":           "device-old",
		"codex_5h_used_percent":   91,
		"codex_7d_used_percent":   72,
		"codex_usage_updated_at":  "2026-06-14T00:00:00Z",
		"unrelated_configuration": true,
	})

	require.NotContains(t, extra, "email")
	require.NotContains(t, extra, "chatgpt_account_id")
	require.NotContains(t, extra, "web_profile")
	require.NotContains(t, extra, "cookies")
	require.NotContains(t, extra, "oai_device_id")
	require.NotContains(t, extra, "codex_5h_used_percent")
	require.NotContains(t, extra, "codex_7d_used_percent")
	require.NotContains(t, extra, "codex_usage_updated_at")
	require.Equal(t, true, extra["unrelated_configuration"])
}

func TestNormalizeOpenAICodexIdentityExtraPreservesTrustedCodexSnapshots(t *testing.T) {
	extra := NormalizeOpenAICodexIdentityExtra(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		"email":              "current@example.com",
		"chatgpt_account_id": "acct-current",
	}, map[string]any{
		"email":                 "current@example.com",
		"chatgpt_account_id":    "acct-current",
		"codex_5h_used_percent": 91,
	})

	require.Equal(t, "current@example.com", extra["email"])
	require.Equal(t, "acct-current", extra["chatgpt_account_id"])
	require.Equal(t, 91, extra["codex_5h_used_percent"])
}
