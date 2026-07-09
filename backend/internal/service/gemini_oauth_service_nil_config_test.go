//go:build unit

package service

import (
	"context"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/stretchr/testify/require"
)

func TestGeminiOAuthServiceGenerateAuthURL_NilConfigUsesBuiltInOAuthDefaults(t *testing.T) {
	t.Setenv(geminicli.GeminiCLIOAuthClientSecretEnv, "test-built-in-secret")

	svc := NewGeminiOAuthService(nil, nil, nil, nil)
	defer svc.Stop()

	var result *GeminiAuthURLResult
	require.NotPanics(t, func() {
		var err error
		result, err = svc.GenerateAuthURL(
			context.Background(),
			nil,
			"https://example.com/auth/callback",
			"project-1",
			"code_assist",
			"",
		)
		require.NoError(t, err)
	})

	require.NotNil(t, result)
	parsed, err := url.Parse(result.AuthURL)
	require.NoError(t, err)
	query := parsed.Query()
	require.Equal(t, geminicli.GeminiCLIOAuthClientID, query.Get("client_id"))
	require.Equal(t, geminicli.GeminiCLIRedirectURI, query.Get("redirect_uri"))
	require.Equal(t, geminicli.DefaultCodeAssistScopes, query.Get("scope"))
	require.Equal(t, result.State, query.Get("state"))
	require.NotEmpty(t, result.SessionID)
}
