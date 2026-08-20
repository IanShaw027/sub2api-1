//go:build unit

package service

import (
	"context"
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
	require.Equal(t, 1, normalizeAccountConcurrency(PlatformGrok, AccountTypeOAuth, 200))
	require.Equal(t, 12, normalizeAccountConcurrency(PlatformOpenAI, AccountTypeOAuth, 200))
	require.Equal(t, 12, normalizeAccountConcurrency(PlatformAnthropic, AccountTypeSetupToken, 200))
}

func TestNormalizeAccountConcurrencyPreservesHighAPIKeyConcurrency(t *testing.T) {
	require.Equal(t, 200, normalizeAccountConcurrency(PlatformOpenAI, AccountTypeAPIKey, 200))
}

func TestUpdateAccount_NormalizesOutOfRangeConcurrencyByPlatform(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		platform string
		typ      string
		input    int
		want     int
	}{
		{name: "grok oauth zero falls back to one", platform: PlatformGrok, typ: AccountTypeOAuth, input: 0, want: 1},
		{name: "anthropic oauth zero falls back to twelve", platform: PlatformAnthropic, typ: AccountTypeOAuth, input: 0, want: 12},
		{name: "explicit in-range value is preserved", platform: PlatformAnthropic, typ: AccountTypeOAuth, input: 3, want: 3},
		{name: "api key 200 is preserved", platform: PlatformOpenAI, typ: AccountTypeAPIKey, input: 200, want: 200},
		{name: "oauth 200 falls back to twelve", platform: PlatformOpenAI, typ: AccountTypeOAuth, input: 200, want: 12},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &accountRepoStubForOAuthOnlyGroup{
				getByIDAccount: &Account{
					ID:          55,
					Name:        "before",
					Platform:    tc.platform,
					Type:        tc.typ,
					Status:      StatusActive,
					Concurrency: 7,
				},
			}
			svc := &adminServiceImpl{accountRepo: repo}

			updated, err := svc.UpdateAccount(context.Background(), 55, &UpdateAccountInput{
				Concurrency: &tc.input,
			})

			require.NoError(t, err)
			require.NotNil(t, updated)
			require.NotNil(t, repo.updatedAccount)
			require.Equal(t, tc.want, repo.updatedAccount.Concurrency)
		})
	}
}
