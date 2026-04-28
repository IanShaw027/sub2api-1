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

func TestAccountFromServiceDetailKeepsSensitiveCredentials(t *testing.T) {
	account := &service.Account{Platform: service.PlatformOpenAI, Credentials: map[string]any{
		"access_token":  "access-secret",
		"refresh_token": "refresh-secret",
		"api_key":       "sk-secret",
		"client_secret": "client-secret",
		"base_url":      "https://api.example.com",
	}}

	out := AccountFromServiceDetail(account)
	require.NotNil(t, out)
	require.Equal(t, "access-secret", out.Credentials["access_token"])
	require.Equal(t, "refresh-secret", out.Credentials["refresh_token"])
	require.Equal(t, "sk-secret", out.Credentials["api_key"])
	require.Equal(t, "client-secret", out.Credentials["client_secret"])
	require.Equal(t, "https://api.example.com", out.Credentials["base_url"])
}
