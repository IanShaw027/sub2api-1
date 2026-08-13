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

func TestAccountService_Create_RejectsShortKiroRefreshToken(t *testing.T) {
	t.Parallel()

	svc := &AccountService{
		accountRepo: &accountRepoStubForOAuthOnlyGroup{},
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:        "kiro-oauth",
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"refresh_token": "short"},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "kiro refresh_token is too short")
}

func TestAccountService_Create_RejectsTruncatedKiroRefreshToken(t *testing.T) {
	t.Parallel()

	svc := &AccountService{
		accountRepo: &accountRepoStubForOAuthOnlyGroup{},
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Create(context.Background(), CreateAccountRequest{
		Name:        "kiro-oauth",
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"refresh_token": "kiro-refresh-token..."},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "kiro refresh_token appears truncated")
}

func TestAccountService_Update_IgnoresInvalidKiroIDCAuthPatches(t *testing.T) {
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
			require.NoError(t, err)
			require.NotNil(t, accountRepo.updatedAccount)
			require.Equal(t, "refresh-token", accountRepo.updatedAccount.GetCredential("refresh_token"))
			require.Empty(t, accountRepo.updatedAccount.GetCredential("auth_method"))
			require.Empty(t, accountRepo.updatedAccount.GetCredential("client_id"))
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

func TestNormalizeKiroAuthMethod_CanonicalizesExternalIDPAliases(t *testing.T) {
	t.Parallel()

	for _, authMethod := range []string{"external_idp", "external-idp", "ExternalIdp"} {
		authMethod := authMethod
		t.Run(authMethod, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, "external_idp", NormalizeKiroAuthMethod(map[string]any{
				"auth_method": authMethod,
			}))
		})
	}
}

func TestAccountService_Update_IgnoresKiroOAuthAuthCredentialPatches(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{
		getByIDAccount: &Account{
			ID:       12,
			Name:     "kiro-oauth",
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Credentials: map[string]any{
				"refresh_token":                  "old-refresh",
				"access_token":                   "old-access",
				"expires_at":                     "1735689600",
				"client_id":                      "old-client-id",
				"client_secret":                  "old-client-secret",
				"auth_method":                    "idc",
				"profile_arn":                    "arn:aws:kiro:old",
				"auth_region":                    "us-west-2",
				"api_region":                     "us-east-2",
				"machine_id":                     "machine-1",
				"model_mapping":                  map[string]any{"claude-sonnet-4": "claude-sonnet-4"},
				"temp_unschedulable_enabled":     true,
				"temp_unschedulable_rules":       []any{map[string]any{"error_code": float64(429), "duration_minutes": float64(10)}},
				"intercept_warmup_requests":      true,
				"custom_unrelated_runtime_field": "keep-me",
			},
		},
	}
	svc := &AccountService{
		accountRepo: accountRepo,
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Update(context.Background(), 12, UpdateAccountRequest{
		Credentials: &map[string]any{
			"refresh_token": "new-refresh",
			"access_token":  "new-access",
			"expires_at":    nil,
			"client_id":     "new-client-id",
			"client_secret": "new-client-secret",
			"auth_method":   "social",
			"profile_arn":   "arn:aws:kiro:new",
			"auth_region":   "eu-west-1",
			"api_region":    "eu-central-1",
			"machine_id":    "machine-2",
			"model_mapping": map[string]any{"claude-sonnet-4.6": "claude-sonnet-4.6"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, accountRepo.updatedAccount)
	require.Equal(t, "old-refresh", accountRepo.updatedAccount.GetCredential("refresh_token"))
	require.Equal(t, "old-access", accountRepo.updatedAccount.GetCredential("access_token"))
	require.Equal(t, "1735689600", accountRepo.updatedAccount.GetCredential("expires_at"))
	require.Equal(t, "old-client-id", accountRepo.updatedAccount.GetCredential("client_id"))
	require.Equal(t, "old-client-secret", accountRepo.updatedAccount.GetCredential("client_secret"))
	require.Equal(t, "idc", accountRepo.updatedAccount.GetCredential("auth_method"))
	require.Equal(t, "arn:aws:kiro:new", accountRepo.updatedAccount.GetCredential("profile_arn"))
	require.Equal(t, "eu-west-1", accountRepo.updatedAccount.GetCredential("auth_region"))
	require.Equal(t, "eu-central-1", accountRepo.updatedAccount.GetCredential("api_region"))
	require.Equal(t, "machine-2", accountRepo.updatedAccount.GetCredential("machine_id"))
	require.Equal(t, map[string]any{"claude-sonnet-4.6": "claude-sonnet-4.6"}, accountRepo.updatedAccount.Credentials["model_mapping"])
	require.Equal(t, true, accountRepo.updatedAccount.Credentials["temp_unschedulable_enabled"])
	require.Equal(t, []any{map[string]any{"error_code": float64(429), "duration_minutes": float64(10)}}, accountRepo.updatedAccount.Credentials["temp_unschedulable_rules"])
	require.Equal(t, true, accountRepo.updatedAccount.Credentials["intercept_warmup_requests"])
	require.Equal(t, "keep-me", accountRepo.updatedAccount.GetCredential("custom_unrelated_runtime_field"))
}

func TestAccountService_Update_PreservesOmittedKiroAPIKeyCredentials(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{
		getByIDAccount: &Account{
			ID:       13,
			Name:     "kiro-apikey",
			Platform: PlatformKiro,
			Type:     AccountTypeAPIKey,
			Status:   StatusActive,
			Credentials: map[string]any{
				"api_key":       "old-api-key",
				"region":        "us-east-1",
				"model":         "legacy-model",
				"profile_arn":   "arn:aws:kiro:old",
				"auth_region":   "us-west-2",
				"api_region":    "us-east-2",
				"machine_id":    "machine-1",
				"model_mapping": map[string]any{"claude-sonnet-4": "claude-sonnet-4"},
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
	require.Equal(t, "legacy-model", accountRepo.updatedAccount.GetCredential("model"))
	require.Equal(t, "arn:aws:kiro:old", accountRepo.updatedAccount.GetCredential("profile_arn"))
	require.Equal(t, "us-west-2", accountRepo.updatedAccount.GetCredential("auth_region"))
	require.Equal(t, "us-east-2", accountRepo.updatedAccount.GetCredential("api_region"))
	require.Equal(t, "machine-1", accountRepo.updatedAccount.GetCredential("machine_id"))
	require.Equal(t, map[string]any{"claude-sonnet-4": "claude-sonnet-4"}, accountRepo.updatedAccount.Credentials["model_mapping"])
}

func TestAccountService_Update_IgnoresKiroOAuthExpiresAtEmptyStringPatch(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{
		getByIDAccount: &Account{
			ID:       14,
			Name:     "kiro-oauth",
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Credentials: map[string]any{
				"refresh_token": "kiro-refresh-token",
				"expires_at":    "1735689600",
			},
		},
	}
	svc := &AccountService{
		accountRepo: accountRepo,
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Update(context.Background(), 14, UpdateAccountRequest{
		Credentials: &map[string]any{
			"expires_at": "",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, accountRepo.updatedAccount)
	require.Equal(t, "kiro-refresh-token", accountRepo.updatedAccount.GetCredential("refresh_token"))
	require.Equal(t, "1735689600", accountRepo.updatedAccount.GetCredential("expires_at"))
}

func TestAccountService_Update_IgnoresKiroOAuthExpiresAtNullPatch(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{
		getByIDAccount: &Account{
			ID:       15,
			Name:     "kiro-oauth",
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Credentials: map[string]any{
				"refresh_token": "kiro-refresh-token",
				"expires_at":    "1735689600",
			},
		},
	}
	svc := &AccountService{
		accountRepo: accountRepo,
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Update(context.Background(), 15, UpdateAccountRequest{
		Credentials: &map[string]any{
			"expires_at": nil,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, accountRepo.updatedAccount)
	require.Equal(t, "kiro-refresh-token", accountRepo.updatedAccount.GetCredential("refresh_token"))
	require.Equal(t, "1735689600", accountRepo.updatedAccount.GetCredential("expires_at"))
}

func TestAccountService_Update_ClearsKiroModelMappingWithEmptyObject(t *testing.T) {
	t.Parallel()

	accountRepo := &accountRepoStubForOAuthOnlyGroup{
		getByIDAccount: &Account{
			ID:       16,
			Name:     "kiro-oauth",
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Credentials: map[string]any{
				"refresh_token": "kiro-refresh-token",
				"model_mapping": map[string]any{"claude-sonnet-4": "claude-sonnet-4"},
			},
		},
	}
	svc := &AccountService{
		accountRepo: accountRepo,
		groupRepo:   &groupRepoStubForOAuthOnlyGroup{},
	}

	_, err := svc.Update(context.Background(), 16, UpdateAccountRequest{
		Credentials: &map[string]any{
			"model_mapping": map[string]any{},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, accountRepo.updatedAccount)
	require.Equal(t, "kiro-refresh-token", accountRepo.updatedAccount.GetCredential("refresh_token"))
	require.Equal(t, map[string]any{}, accountRepo.updatedAccount.Credentials["model_mapping"])
}
