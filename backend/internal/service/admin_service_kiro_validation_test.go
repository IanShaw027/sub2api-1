//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminServiceCreateAccount_RejectsNonOAuthKiroType(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{accountRepo: &mockAccountRepoForGemini{}}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "kiro-key",
		Platform:    PlatformKiro,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
	})

	require.Nil(t, account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro accounts only support oauth type")
}

func TestAdminServiceUpdateAccount_RejectsNonOAuthKiroType(t *testing.T) {
	t.Parallel()

	repo := &mockAccountRepoForGemini{
		accountsByID: map[int64]*Account{
			55: {
				ID:       55,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "rt",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 55, &UpdateAccountInput{
		Type: AccountTypeAPIKey,
	})

	require.Nil(t, account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro accounts only support oauth type")
}
