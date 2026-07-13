//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminServiceCreateAccount_AllowsGrokOAuthHighConcurrency(t *testing.T) {
	repo := &kiroDefaultAccountRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "grok-oauth",
		Platform:             PlatformGrok,
		Type:                 AccountTypeOAuth,
		Concurrency:          10,
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, 10, account.Concurrency)
	require.Len(t, repo.createdAccounts, 1)
}

func TestAdminServiceUpdateAccount_AllowsGrokOAuthHighConcurrency(t *testing.T) {
	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			42: {
				ID:          42,
				Name:        "grok-oauth",
				Platform:    PlatformGrok,
				Type:        AccountTypeOAuth,
				Concurrency: 1,
				Status:      StatusActive,
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	concurrency := 10

	account, err := svc.UpdateAccount(context.Background(), 42, &UpdateAccountInput{
		Concurrency: &concurrency,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, 10, account.Concurrency)
	require.Len(t, repo.updatedAccounts, 1)
}
