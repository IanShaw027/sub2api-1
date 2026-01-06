//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSensitiveRegexes_MatchExtendedCharset(t *testing.T) {
	// Base64-like secret (contains '+', '/', '=')
	require.True(t, apiKeyRegex.MatchString("sk-proj-abc123+def456/ghi789=="))

	// JWT-like token (contains '.', '-', '_')
	require.True(t, tokenRegex.MatchString("Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"))

	// URL-encoded parameter (contains '%')
	require.True(t, apiKeyRegex.MatchString("api_key=abc%2Bdef%3Dghi"))

	// Token variants that may include Base64 or URL-encoding characters
	require.True(t, tokenRegex.MatchString("Bearer abc+def/ghi=="))
	require.True(t, tokenRegex.MatchString("token=abc%2Bdef%3Dghi"))
}
