package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountRepoStubForAdminCreateValidation struct {
	kiroDefaultAccountRepoStub
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

func TestAdminServiceCreateAccount_AllowsKiroAPIKeyType(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForAdminCreateValidation{}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "kiro-key",
		Platform:             PlatformKiro,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.True(t, repo.createCalled)
	require.Equal(t, AccountTypeAPIKey, account.Type)
}

func TestAdminServiceCreateAccount_RejectsInvalidKiroCredentials(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{accountRepo: &kiroDefaultAccountRepoStub{}}

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

func TestAdminServiceUpdateAccount_AllowsSwitchToKiroAPIKeyType(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
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
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, "sk-test", account.GetCredential("api_key"))
}

func TestAdminServiceUpdateAccount_RejectsInvalidKiroIDCFields(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
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

func TestAdminServiceUpdateAccount_PreservesOmittedKiroOAuthSecrets(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			55: {
				ID:       55,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh",
					"access_token":  "old-access",
					"expires_at":    "1735689600",
					"client_id":     "old-client-id",
					"client_secret": "old-client-secret",
					"auth_method":   "idc",
					"profile_arn":   "arn:aws:kiro:old",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 55, &UpdateAccountInput{
		Credentials: map[string]any{
			"auth_method": "social",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "old-refresh", account.GetCredential("refresh_token"))
	require.Equal(t, "old-access", account.GetCredential("access_token"))
	require.Equal(t, "1735689600", account.GetCredential("expires_at"))
	require.Equal(t, "old-client-id", account.GetCredential("client_id"))
	require.Equal(t, "old-client-secret", account.GetCredential("client_secret"))
	require.Equal(t, "social", account.GetCredential("auth_method"))
	_, hasProfileArn := account.Credentials["profile_arn"]
	require.False(t, hasProfileArn)
}

func TestAdminServiceUpdateAccount_PreservesOmittedKiroAPIKeySecret(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			56: {
				ID:       56,
				Name:     "kiro-apikey",
				Platform: PlatformKiro,
				Type:     AccountTypeAPIKey,
				Status:   StatusActive,
				Credentials: map[string]any{
					"api_key": "old-api-key",
					"region":  "us-east-1",
					"model":   "legacy-model",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 56, &UpdateAccountInput{
		Credentials: map[string]any{
			"region": "eu-west-1",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "old-api-key", account.GetCredential("api_key"))
	require.Equal(t, "eu-west-1", account.GetCredential("region"))
	_, hasModel := account.Credentials["model"]
	require.False(t, hasModel)
}

func TestAdminServiceBulkUpdateAccounts_RejectsInvalidMergedKiroCredentials(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
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
		groupRepo:   &kiroDefaultGroupRepoStub{getErr: ErrGroupNotFound},
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

func TestValidateKiroCredentials_RequiresIDCSecretsForAllIDCRefreshMethods(t *testing.T) {
	t.Parallel()

	for _, authMethod := range []string{"idc", "builder-id", "iam"} {
		err := validateKiroCredentials(map[string]any{
			"refresh_token": "rt",
			"auth_method":   authMethod,
			"client_id":     "client-only",
		})
		require.Error(t, err, authMethod)
		require.Contains(t, err.Error(), "kiro idc client_id and client_secret are required", authMethod)
	}
}

func TestNormalizeKiroAuthMethod_InfersIDCFromClientCredentials(t *testing.T) {
	t.Parallel()

	got := NormalizeKiroAuthMethod(map[string]any{
		"refresh_token": "rt",
		"client_id":     "client-id",
		"client_secret": "client-secret",
	})
	require.Equal(t, "idc", got)
}

func TestNormalizeKiroAuthMethod_ExplicitSocialOverridesClientCredentials(t *testing.T) {
	t.Parallel()

	got := NormalizeKiroAuthMethod(map[string]any{
		"refresh_token": "rt",
		"auth_method":   "social",
		"client_id":     "client-id",
		"client_secret": "client-secret",
	})
	require.Equal(t, "social", got)
}
