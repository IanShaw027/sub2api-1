//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type accountRepoStubForOAuthOnlyGroup struct {
	accountRepoStub
	createErr       error
	updateErr       error
	getByIDAccount  *Account
	createdAccount  *Account
	updatedAccount  *Account
	bindGroupsCalls [][]int64
}

func (s *accountRepoStubForOAuthOnlyGroup) Create(_ context.Context, account *Account) error {
	if s.createErr != nil {
		return s.createErr
	}
	clone := *account
	clone.ID = 101
	s.createdAccount = &clone
	account.ID = 101
	return nil
}

func (s *accountRepoStubForOAuthOnlyGroup) GetByID(_ context.Context, _ int64) (*Account, error) {
	if s.getByIDAccount == nil {
		return nil, errors.New("account not found")
	}
	clone := *s.getByIDAccount
	return &clone, nil
}

func (s *accountRepoStubForOAuthOnlyGroup) Update(_ context.Context, account *Account) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	clone := *account
	s.updatedAccount = &clone
	return nil
}

func (s *accountRepoStubForOAuthOnlyGroup) BindGroups(_ context.Context, _ int64, groupIDs []int64) error {
	s.bindGroupsCalls = append(s.bindGroupsCalls, append([]int64(nil), groupIDs...))
	return nil
}

type groupRepoStubForOAuthOnlyGroup struct {
	groupRepoStubForAdmin
	groups map[int64]*Group
}

func (s *groupRepoStubForOAuthOnlyGroup) GetByID(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	clone := *group
	return &clone, nil
}

func TestAccountService_Create_RejectsAPIKeyForKiroOAuthOnlyGroup(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{}
	groupRepo := &groupRepoStubForOAuthOnlyGroup{
		groups: map[int64]*Group{
			10: {ID: 10, Name: "kiro-oauth", Platform: PlatformKiro, RequireOAuthOnly: true},
		},
	}
	svc := &AccountService{accountRepo: accountRepo, groupRepo: groupRepo}

	_, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:        "kiro-api-key",
		Platform:    PlatformKiro,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
		GroupIDs:    []int64{10},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "仅允许 OAuth 账号")
	require.Empty(t, accountRepo.bindGroupsCalls)
}

func TestAccountService_Update_RejectsAPIKeyForKiroOAuthOnlyGroup(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{
		getByIDAccount: &Account{
			ID:       7,
			Name:     "kiro-api-key",
			Platform: PlatformKiro,
			Type:     AccountTypeAPIKey,
			Status:   StatusActive,
		},
	}
	groupRepo := &groupRepoStubForOAuthOnlyGroup{
		groups: map[int64]*Group{
			10: {ID: 10, Name: "kiro-oauth", Platform: PlatformKiro, RequireOAuthOnly: true},
		},
	}
	svc := &AccountService{accountRepo: accountRepo, groupRepo: groupRepo}

	groupIDs := []int64{10}
	_, err := svc.Update(context.Background(), 7, UpdateAccountRequest{
		GroupIDs: &groupIDs,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "仅允许 OAuth 账号")
	require.Empty(t, accountRepo.bindGroupsCalls)
}

func TestAccountService_Create_RejectsInvalidKiroCredentials(t *testing.T) {
	t.Parallel()

	svc := &AccountService{
		accountRepo: &accountRepoStubForOAuthOnlyGroup{},
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:        "kiro-oauth",
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"auth_method": "social"},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "kiro refresh_token is required")
}

func TestAccountService_Update_RejectsInvalidKiroIDCFields(t *testing.T) {
	t.Parallel()

	for _, authMethod := range []string{"idc", "IDC", "builder-id", "builderid", "awsidc", "iam"} {
		authMethod := authMethod
		t.Run(authMethod, func(t *testing.T) {
			t.Parallel()

			accountRepo := &accountRepoStubForOAuthOnlyGroup{
				getByIDAccount: &Account{
					ID:       11,
					Name:     "kiro-oauth",
					Platform: PlatformKiro,
					Type:     AccountTypeOAuth,
					Status:   StatusActive,
					Credentials: map[string]any{
						"refresh_token": "refresh-token",
					},
				},
			}
			svc := &AccountService{
				accountRepo: accountRepo,
				groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
			}

			_, err := svc.Update(context.Background(), 11, UpdateAccountRequest{
				Credentials: &map[string]any{
					"refresh_token": "refresh-token",
					"auth_method":   authMethod,
					"client_id":     "client-id",
				},
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "kiro idc client_id and client_secret are required")
		})
	}
}

func TestAccountService_Create_RejectsInvalidKiroIDCRefreshAliases(t *testing.T) {
	t.Parallel()

	for _, authMethod := range []string{"IDC", "builder-id", "builderid", "awsidc", "iam"} {
		authMethod := authMethod
		t.Run(authMethod, func(t *testing.T) {
			t.Parallel()

			svc := &AccountService{
				accountRepo: &accountRepoStubForOAuthOnlyGroup{},
				groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
			}

			_, err := svc.Create(context.Background(), CreateAccountRequest{
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"refresh_token": "refresh-token",
					"auth_method":   authMethod,
				},
			})

			require.Error(t, err)
			require.ErrorContains(t, err, "kiro idc client_id and client_secret are required")
		})
	}
}

func TestAccountService_Create_AllowsKiroAPIKeyWithoutOAuthCredentials(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{}
	svc := &AccountService{
		accountRepo: accountRepo,
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	account, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:        "kiro-api-key",
		Platform:    PlatformKiro,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, "sk-test", account.GetCredential("api_key"))
}

func TestNormalizeKiroAuthMethod_CanonicalizesIDCAliases(t *testing.T) {
	t.Parallel()

	for _, authMethod := range []string{"IDC", "builder-id", "builderid", "awsidc", "aws-idc", "iam"} {
		authMethod := authMethod
		t.Run(authMethod, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, "idc", NormalizeKiroAuthMethod(map[string]any{
				"auth_method": authMethod,
			}))
		})
	}
}

func TestAccountService_Update_PreservesOmittedKiroOAuthSecrets(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{
		getByIDAccount: &Account{
			ID:       12,
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
	}
	svc := &AccountService{
		accountRepo: accountRepo,
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Update(context.Background(), 12, UpdateAccountRequest{
		Credentials: &map[string]any{
			"auth_method": "social",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, accountRepo.updatedAccount)
	require.Equal(t, "old-refresh", accountRepo.updatedAccount.GetCredential("refresh_token"))
	require.Equal(t, "old-access", accountRepo.updatedAccount.GetCredential("access_token"))
	require.Equal(t, "1735689600", accountRepo.updatedAccount.GetCredential("expires_at"))
	require.Equal(t, "old-client-id", accountRepo.updatedAccount.GetCredential("client_id"))
	require.Equal(t, "old-client-secret", accountRepo.updatedAccount.GetCredential("client_secret"))
	require.Equal(t, "social", accountRepo.updatedAccount.GetCredential("auth_method"))
	_, hasProfileArn := accountRepo.updatedAccount.Credentials["profile_arn"]
	require.False(t, hasProfileArn)
}

func TestAccountService_Update_PreservesOmittedKiroAPIKeySecret(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{
		getByIDAccount: &Account{
			ID:       13,
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
	}
	svc := &AccountService{
		accountRepo: accountRepo,
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Update(context.Background(), 13, UpdateAccountRequest{
		Credentials: &map[string]any{
			"region": "eu-west-1",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, accountRepo.updatedAccount)
	require.Equal(t, "old-api-key", accountRepo.updatedAccount.GetCredential("api_key"))
	require.Equal(t, "eu-west-1", accountRepo.updatedAccount.GetCredential("region"))
	_, hasModel := accountRepo.updatedAccount.Credentials["model"]
	require.False(t, hasModel)
}
