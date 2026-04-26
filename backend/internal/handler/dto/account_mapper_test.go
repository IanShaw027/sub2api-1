package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountFromServiceRedactsSensitiveCredentials(t *testing.T) {
	account := &service.Account{Credentials: map[string]any{
		"access_token":  "access-secret",
		"refresh_token": "refresh-secret",
		"api_key":       "sk-secret",
		"client_secret": "client-secret",
		"model_mapping": map[string]any{"claude-sonnet-4": "claude-sonnet-4"},
	}}

	out := AccountFromService(account)
	require.NotNil(t, out)
	require.NotContains(t, out.Credentials, "access_token")
	require.NotContains(t, out.Credentials, "refresh_token")
	require.NotContains(t, out.Credentials, "api_key")
	require.NotContains(t, out.Credentials, "client_secret")
	require.Contains(t, out.Credentials, "model_mapping")
}
