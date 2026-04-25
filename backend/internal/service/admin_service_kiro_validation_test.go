//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountRepoStubForAdminCreateValidation struct {
	mockAccountRepoForGemini
	createCalled     bool
	bindGroupsCalled bool
}

func (s *accountRepoStubForAdminCreateValidation) Create(_ context.Context, account *Account) error {
	s.createCalled = true
	account.ID = 101
	return nil
}

func (s *accountRepoStubForAdminCreateValidation) BindGroups(_ context.Context, _ int64, _ []int64) error {
	s.bindGroupsCalled = true
	return nil
}

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

func TestAdminServiceCreateAccount_RejectsInvalidKiroCredentials(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{accountRepo: &mockAccountRepoForGemini{}}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "kiro-oauth",
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"auth_method": "social"},
	})

	require.Nil(t, account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro refresh_token is required")
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

func TestAdminServiceUpdateAccount_RejectsInvalidKiroIDCFields(t *testing.T) {
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
		Credentials: map[string]any{
			"refresh_token": "rt",
			"auth_method":   "idc",
			"client_id":     "client-only",
		},
	})

	require.Nil(t, account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro idc client_id and client_secret are required")
}

func TestAdminServiceBulkUpdateAccounts_RejectsInvalidMergedKiroCredentials(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		getByIDsAccounts: []*Account{
			{
				ID:       77,
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"refresh_token": "rt",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs: []int64{77},
		Credentials: map[string]any{
			"auth_method": "idc",
			"client_id":   "client-only",
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro idc client_id and client_secret are required")
	require.Empty(t, repo.bulkUpdateIDs)
}

func TestAdminServiceCreateAccount_ValidatesGroupsBeforePersist(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForAdminCreateValidation{}
	svc := &adminServiceImpl{
		accountRepo: repo,
		groupRepo:   &groupRepoStubForAdmin{getErr: ErrGroupNotFound},
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "kiro-oauth",
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"refresh_token": "rt"},
		GroupIDs:    []int64{404},
	})

	require.Nil(t, account)
	require.Error(t, err)
	require.False(t, repo.createCalled)
	require.False(t, repo.bindGroupsCalled)
}

func TestValidateAccountPlatform_RejectsUnknownPlatform(t *testing.T) {
	t.Parallel()

	err := validateAccountPlatform("not-a-real-platform")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported platform")
}

func TestValidateKiroCredentials_AcceptsUnixSecondsExpiresAt(t *testing.T) {
	t.Parallel()

	err := validateKiroCredentials(map[string]any{
		"refresh_token": "rt",
		"expires_at":    time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
}
