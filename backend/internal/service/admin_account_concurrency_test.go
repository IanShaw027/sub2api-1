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

func TestNormalizeAccountConcurrencyDefaultsAnthropicAndOpenAIOAuthToTwelve(t *testing.T) {
	require.Equal(t, 12, normalizeAccountConcurrency(PlatformAnthropic, AccountTypeOAuth, 0))
	require.Equal(t, 12, normalizeAccountConcurrency(PlatformAnthropic, AccountTypeSetupToken, -5))
	require.Equal(t, 12, normalizeAccountConcurrency(PlatformOpenAI, AccountTypeOAuth, 0))
}

func TestNormalizeAccountConcurrencyPreservesExplicitValues(t *testing.T) {
	require.Equal(t, 15, normalizeAccountConcurrency(PlatformGrok, AccountTypeOAuth, 15))
	require.Equal(t, 2, normalizeAccountConcurrency(PlatformOpenAI, AccountTypeOAuth, 2))
	require.Equal(t, 2, normalizeAccountConcurrency(PlatformGrok, AccountTypeAPIKey, 2))
}

func TestNormalizeAccountConcurrencyFallsBackForOutOfRangeValues(t *testing.T) {
	require.Equal(t, 1, normalizeAccountConcurrency(PlatformGrok, AccountTypeOAuth, 50))
	require.Equal(t, 12, normalizeAccountConcurrency(PlatformOpenAI, AccountTypeOAuth, 99))
}
