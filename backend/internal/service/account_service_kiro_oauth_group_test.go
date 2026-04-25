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

	for _, authMethod := range []string{"idc", "IDC", "builder-id", "iam"} {
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

	for _, authMethod := range []string{"IDC", "builder-id", "iam"} {
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
