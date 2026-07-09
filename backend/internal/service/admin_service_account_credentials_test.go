//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminServiceUpdateAccountMergesCredentialPatchAndPreservesSensitiveValues(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			42: {
				ID:       42,
				Name:     "openai-oauth",
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"access_token":          "old-access",
					"refresh_token":         "old-refresh",
					"client_id":             "old-client",
					"email":                 "user@example.com",
					"model_mapping":         map[string]any{"gpt-5.2": "gpt-5.2"},
					"compact_model_mapping": map[string]any{"gpt-5.4": "compact"},
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 42, &UpdateAccountInput{
		Credentials: map[string]any{
			"email":                 "user@example.com",
			"model_mapping":         map[string]any{"gpt-5.2": "gpt-5.2-live"},
			"compact_model_mapping": nil,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, repo.updatedAccounts, 1)
	require.Equal(t, "old-access", updated.GetCredential("access_token"))
	require.Equal(t, "old-refresh", updated.GetCredential("refresh_token"))
	require.Equal(t, "old-client", updated.GetCredential("client_id"))
	require.Equal(t, "user@example.com", updated.GetCredential("email"))
	require.Equal(t, map[string]any{"gpt-5.2": "gpt-5.2-live"}, updated.Credentials["model_mapping"])
	require.NotContains(t, updated.Credentials, "compact_model_mapping")
}

func TestAdminServiceUpdateAccountRejectsNilInput(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			42: {
				ID:       42,
				Name:     "openai-oauth",
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 42, nil)

	require.Nil(t, updated)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account input is required")
	require.Empty(t, repo.updatedAccounts)
}

func TestAdminServiceUpdateAccountIgnoresEmptySensitiveCredentialPatch(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			43: {
				ID:       43,
				Name:     "openai-oauth",
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"access_token":  "old-access",
					"refresh_token": "old-refresh",
					"api_key":       "old-api-key",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 43, &UpdateAccountInput{
		Credentials: map[string]any{
			"access_token":  "",
			"refresh_token": nil,
			"api_key":       " ",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, "old-access", updated.GetCredential("access_token"))
	require.Equal(t, "old-refresh", updated.GetCredential("refresh_token"))
	require.Equal(t, "old-api-key", updated.GetCredential("api_key"))
}

func TestAdminServiceUpdateAccountAllowsExplicitSensitiveCredentialReplacement(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			44: {
				ID:       44,
				Name:     "openai-oauth",
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"access_token":  "old-access",
					"refresh_token": "old-refresh",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 44, &UpdateAccountInput{
		Credentials: map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, "new-access", updated.GetCredential("access_token"))
	require.Equal(t, "new-refresh", updated.GetCredential("refresh_token"))
}

func TestAdminServiceUpdateKiroAccountDeletesLegacyModelWhitelistPatch(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			45: {
				ID:       45,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token":   "old-refresh",
					"access_token":    "old-access",
					"model_whitelist": []any{"claude-sonnet-4.5"},
					"model_mapping":   map[string]any{"claude-sonnet-4.5": "claude-sonnet-4.5"},
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 45, &UpdateAccountInput{
		Credentials: map[string]any{
			"model_whitelist": nil,
			"model_mapping":   map[string]any{"claude-sonnet-*": "claude-sonnet-4.5"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, "old-refresh", updated.GetCredential("refresh_token"))
	require.Equal(t, "old-access", updated.GetCredential("access_token"))
	require.NotContains(t, updated.Credentials, "model_whitelist")
	require.Equal(t, map[string]any{"claude-sonnet-*": "claude-sonnet-4.5"}, updated.Credentials["model_mapping"])
}

func TestAdminServiceUpdateKiroOAuthIgnoresSensitiveCredentialsWithoutTrustedFlag(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			46: {
				ID:       46,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh",
					"access_token":  "old-access",
					"auth_method":   "social",
					"model_mapping": map[string]any{"claude-sonnet-*": "claude-sonnet-4.5"},
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 46, &UpdateAccountInput{
		Credentials: map[string]any{
			"refresh_token": "new-refresh",
			"access_token":  "new-access",
			"auth_method":   "idc",
			"model_mapping": nil,
		},
	})

	require.NoError(t, err)
	require.Equal(t, "old-refresh", updated.GetCredential("refresh_token"))
	require.Equal(t, "old-access", updated.GetCredential("access_token"))
	require.Equal(t, "social", updated.GetCredential("auth_method"))
	require.NotContains(t, updated.Credentials, "model_mapping")
}

func TestAdminServiceUpdateKiroOAuthAllowsTrustedSensitiveCredentialReplacement(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			47: {
				ID:       47,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh",
					"access_token":  "old-access",
					"auth_method":   "idc",
					"client_id":     "old-client",
					"client_secret": "old-secret",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 47, &UpdateAccountInput{
		Credentials: map[string]any{
			"refresh_token": "new-refresh",
			"access_token":  "new-access",
			"auth_method":   "social",
		},
		AllowSensitiveCredentials: true,
	})

	require.NoError(t, err)
	require.Equal(t, "new-refresh", updated.GetCredential("refresh_token"))
	require.Equal(t, "new-access", updated.GetCredential("access_token"))
	require.Equal(t, "social", updated.GetCredential("auth_method"))
	require.NotContains(t, updated.Credentials, "client_id")
	require.NotContains(t, updated.Credentials, "client_secret")
}

func TestAdminServiceUpdateOAuthTrustedOverwriteClearsOmittedSensitiveCredentials(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			49: {
				ID:       49,
				Name:     "openai-oauth",
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "old-refresh",
					"access_token":  "old-access",
					"client_id":     "old-client",
					"client_secret": "old-secret",
					"email":         "user@example.com",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 49, &UpdateAccountInput{
		Credentials: map[string]any{
			"email": "user@example.com",
		},
		AllowSensitiveCredentials: true,
	})

	require.NoError(t, err)
	require.Equal(t, "user@example.com", updated.GetCredential("email"))
	require.NotContains(t, updated.Credentials, "refresh_token")
	require.NotContains(t, updated.Credentials, "access_token")
	require.NotContains(t, updated.Credentials, "client_id")
	require.NotContains(t, updated.Credentials, "client_secret")
}

func TestAdminServiceBulkUpdateKiroCredentialsUsesKiroMerge(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       48,
		Name:     "kiro-oauth",
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Credentials: map[string]any{
			"refresh_token":   "old-refresh",
			"access_token":    "old-access",
			"auth_method":     "social",
			"model_whitelist": []any{"claude-sonnet-4.5"},
		},
	}
	repo := &kiroDefaultAccountRepoStub{
		getByIDsAccounts: []*Account{account},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs: []int64{48},
		Credentials: map[string]any{
			"refresh_token":   "new-refresh",
			"access_token":    "new-access",
			"model_whitelist": nil,
			"model_mapping":   map[string]any{"claude-haiku-*": "claude-haiku-4.5"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Success)
	require.Len(t, repo.updatedAccounts, 1)
	require.Equal(t, "old-refresh", repo.updatedAccounts[0].GetCredential("refresh_token"))
	require.Equal(t, "old-access", repo.updatedAccounts[0].GetCredential("access_token"))
	require.NotContains(t, repo.updatedAccounts[0].Credentials, "model_whitelist")
	require.Equal(t, map[string]any{"claude-haiku-*": "claude-haiku-4.5"}, repo.updatedAccounts[0].Credentials["model_mapping"])
	require.Empty(t, repo.bulkUpdateCreds)
}

func TestAdminServiceUpdateAccountProxyChangeClearsFallbackOrigin(t *testing.T) {
	t.Parallel()

	originID := int64(10)
	oldProxyID := int64(11)
	newProxyID := int64(12)
	repo := &kiroDefaultAccountRepoStub{accountsByID: map[int64]*Account{
		50: {
			ID:                    50,
			Name:                  "fallback-account",
			Platform:              PlatformOpenAI,
			Type:                  AccountTypeOAuth,
			Status:                StatusActive,
			ProxyID:               &oldProxyID,
			ProxyFallbackOriginID: &originID,
			Credentials:           map[string]any{},
		},
	}}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 50, &UpdateAccountInput{ProxyID: &newProxyID})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, repo.updatedAccounts, 1)
	require.Nil(t, repo.updatedAccounts[0].ProxyFallbackOriginID)
	require.Nil(t, updated.ProxyFallbackOriginID)
}
