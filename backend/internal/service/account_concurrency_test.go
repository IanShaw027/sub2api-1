//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEffectiveConcurrency_UsesStoredWhenInRange(t *testing.T) {
	t.Parallel()

	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 15}

	require.Equal(t, 15, account.EffectiveConcurrency())
}

func TestEffectiveConcurrency_Fallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		platform string
		typ      string
		conc     int
		want     int
	}{
		{name: "anthropic oauth defaults to twelve", platform: PlatformAnthropic, typ: AccountTypeOAuth, conc: 0, want: 12},
		{name: "anthropic setup token defaults to twelve", platform: PlatformAnthropic, typ: AccountTypeSetupToken, conc: -1, want: 12},
		{name: "openai oauth caps invalid high values", platform: PlatformOpenAI, typ: AccountTypeOAuth, conc: 99, want: 12},
		{name: "grok oauth defaults to one", platform: PlatformGrok, typ: AccountTypeOAuth, conc: 0, want: 1},
		{name: "gemini oauth keeps generic default", platform: PlatformGemini, typ: AccountTypeOAuth, conc: 0, want: 3},
		{name: "anthropic apikey keeps generic default", platform: PlatformAnthropic, typ: AccountTypeAPIKey, conc: 0, want: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			account := &Account{Platform: tc.platform, Type: tc.typ, Concurrency: tc.conc}
			require.Equal(t, tc.want, account.EffectiveConcurrency())
		})
	}
}

func TestBurstConcurrency(t *testing.T) {
	t.Parallel()

	require.Equal(t, 2, (&Account{Concurrency: 10}).OverflowConcurrency())
	require.Equal(t, 12, (&Account{Concurrency: 10}).BurstConcurrency())
	require.Equal(t, 14, (&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 0}).BurstConcurrency())
	require.Equal(t, 3, (&Account{Concurrency: 15}).OverflowConcurrency())
	require.Equal(t, 18, (&Account{Concurrency: 15}).BurstConcurrency())
}

func TestAccountServiceCreate_WritesPlatformDefaultWhenConcurrencyOmitted(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{}
	svc := &AccountService{
		accountRepo: accountRepo,
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	account, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "test"},
		Concurrency: 0,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, 12, account.Concurrency)
	require.NotNil(t, accountRepo.createdAccount)
	require.Equal(t, 12, accountRepo.createdAccount.Concurrency)
}
