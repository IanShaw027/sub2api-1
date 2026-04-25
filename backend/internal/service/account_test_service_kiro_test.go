//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type kiroAccountTestRepo struct {
	mockAccountRepoForGemini
	updateErr   error
	updateCalls int
	updatedAcct *Account
}

func (r *kiroAccountTestRepo) Update(_ context.Context, account *Account) error {
	r.updateCalls++
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updatedAcct = account
	return nil
}

func TestAccountTestService_PersistRefreshedKiroCredentials_InvalidatesTokenCache(t *testing.T) {
	t.Parallel()

	cache := newOpenAITokenCacheStub()
	account := &Account{
		ID:       42,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "stale-token",
		},
	}
	cacheKey := KiroTokenCacheKey(account)
	cache.tokens[cacheKey] = "stale-token"

	repo := &kiroAccountTestRepo{}
	svc := &AccountTestService{
		accountRepo:         repo,
		geminiTokenProvider: &GeminiTokenProvider{tokenCache: cache},
	}

	newCreds := map[string]any{
		"access_token":  "fresh-token",
		"refresh_token": "fresh-refresh-token",
		"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}

	err := svc.persistRefreshedKiroCredentials(context.Background(), account, newCreds)
	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.NotSame(t, account, repo.updatedAcct)
	require.Equal(t, newCreds, repo.updatedAcct.Credentials)
	require.Equal(t, "fresh-token", account.GetCredential("access_token"))
	require.Empty(t, cache.tokens[cacheKey])
}

func TestAccountTestService_PersistRefreshedKiroCredentials_UpdateErrorKeepsTokenCache(t *testing.T) {
	t.Parallel()

	cache := newOpenAITokenCacheStub()
	account := &Account{
		ID:       43,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "stale-token",
		},
	}
	cacheKey := KiroTokenCacheKey(account)
	cache.tokens[cacheKey] = "stale-token"

	repo := &kiroAccountTestRepo{updateErr: errors.New("update failed")}
	svc := &AccountTestService{
		accountRepo:         repo,
		geminiTokenProvider: &GeminiTokenProvider{tokenCache: cache},
	}

	newCreds := map[string]any{
		"access_token":  "fresh-token",
		"refresh_token": "fresh-refresh-token",
		"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}

	err := svc.persistRefreshedKiroCredentials(context.Background(), account, newCreds)
	require.EqualError(t, err, "update failed")
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, "stale-token", account.GetCredential("access_token"))
	require.Equal(t, "stale-token", cache.tokens[cacheKey])
}
