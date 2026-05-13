package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthSchedulableForSync(t *testing.T) {
	t.Parallel()

	require.False(t, openAIOAuthSchedulableForSync(true, map[string]any{
		"refresh_token": "rt-only",
	}))
	require.False(t, openAIOAuthSchedulableForSync(false, map[string]any{
		"access_token": "at",
	}))
	require.True(t, openAIOAuthSchedulableForSync(true, map[string]any{
		"access_token":  "at",
		"refresh_token": "rt",
	}))
}

func TestCRSSyncServiceFinalizeOpenAIOAuthSyncStateRestoresHealthyAccount(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{},
	}
	svc := &CRSSyncService{accountRepo: repo}
	account := &Account{
		ID:          501,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Schedulable: false,
		Credentials: map[string]any{
			"access_token":  "restored-at",
			"refresh_token": "rt",
		},
		Status:       StatusError,
		ErrorMessage: "previous error",
	}

	svc.finalizeOpenAIOAuthSyncState(context.Background(), account, StatusActive, true)

	require.True(t, account.Schedulable)
	require.Equal(t, StatusActive, account.Status)
	require.Empty(t, account.ErrorMessage)
	require.Len(t, repo.updatedAccounts, 1)
	require.True(t, repo.updatedAccounts[0].Schedulable)
}

func TestCRSSyncServiceFinalizeOpenAIOAuthSyncStateMarksBrokenAccountError(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{},
	}
	svc := &CRSSyncService{accountRepo: repo}
	account := &Account{
		ID:          502,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Schedulable: false,
		Credentials: map[string]any{
			"refresh_token": "rt-only",
		},
	}

	svc.finalizeOpenAIOAuthSyncState(context.Background(), account, StatusActive, true)

	require.False(t, account.Schedulable)
	require.Equal(t, StatusError, account.Status)
	require.Contains(t, account.ErrorMessage, "missing access_token")
	require.Len(t, repo.updatedAccounts, 1)
}
