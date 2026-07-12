//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAccountConcurrencyDefaultsInvalidGrokOAuthToOne(t *testing.T) {
	require.Equal(t, 1, normalizeAccountConcurrency(PlatformGrok, AccountTypeOAuth, 0))
	require.Equal(t, 1, normalizeAccountConcurrency(PlatformGrok, AccountTypeOAuth, -5))
}

func TestNormalizeAccountConcurrencyPreservesExplicitValues(t *testing.T) {
	require.Equal(t, 50, normalizeAccountConcurrency(PlatformGrok, AccountTypeOAuth, 50))
	require.Equal(t, 2, normalizeAccountConcurrency(PlatformOpenAI, AccountTypeOAuth, 2))
	require.Equal(t, 2, normalizeAccountConcurrency(PlatformGrok, AccountTypeAPIKey, 2))
}

func TestValidateGrokOAuthConcurrencyRequiresUnsafeGate(t *testing.T) {
	t.Setenv("XAI_GROK_UNSAFE_ALLOW_CONCURRENCY_GT_ONE", "")
	require.Error(t, validateGrokOAuthConcurrency(PlatformGrok, AccountTypeOAuth, 10))
	require.NoError(t, validateGrokOAuthConcurrency(PlatformGrok, AccountTypeOAuth, 1))
	require.NoError(t, validateGrokOAuthConcurrency(PlatformOpenAI, AccountTypeOAuth, 10))

	t.Setenv("XAI_GROK_UNSAFE_ALLOW_CONCURRENCY_GT_ONE", "1")
	require.NoError(t, validateGrokOAuthConcurrency(PlatformGrok, AccountTypeOAuth, 10))
}
