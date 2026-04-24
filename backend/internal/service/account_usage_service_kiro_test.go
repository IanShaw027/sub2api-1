//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type kiroUsageAccountRepo struct {
	mockAccountRepoForGemini
	updateCalls int
	updatedAcct *Account
}

func (r *kiroUsageAccountRepo) Update(_ context.Context, account *Account) error {
	r.updateCalls++
	r.updatedAcct = account
	return nil
}

func TestAccountUsageService_PersistRefreshedKiroCredentials_InvalidatesTokenCache(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       88,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "",
			"refresh_token": "refresh-token",
			"expires_at":    time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		},
	}
	repo := &kiroUsageAccountRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{},
	}
	invalidator := &tokenCacheInvalidatorStub{}
	svc := &AccountUsageService{
		accountRepo:           repo,
		tokenCacheInvalidator: invalidator,
	}

	newCreds := map[string]any{
		"access_token":  "fresh-token",
		"refresh_token": "fresh-refresh-token",
		"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}

	err := svc.persistRefreshedKiroCredentials(context.Background(), account, newCreds)

	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.Same(t, account, repo.updatedAcct)
	require.Equal(t, "fresh-token", account.GetCredential("access_token"))
	require.Equal(t, 1, invalidator.calls)
}
