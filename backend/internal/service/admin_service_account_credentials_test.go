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
