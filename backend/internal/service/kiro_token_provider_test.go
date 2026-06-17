//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKiroTokenProvider_RefreshAccount_UsesRefreshAPI(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       51,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-refresh",
			"machine_id":    "machine-1",
		},
	}
	repo := &refreshAPIAccountRepo{account: account}
	cache := &refreshAPICacheStub{lockResult: true}
	executor := &refreshAPIExecutorStub{
		needsRefresh: true,
		credentials: map[string]any{
			"access_token":  "fresh-access",
			"refresh_token": "fresh-refresh",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}

	provider := NewKiroTokenProvider(repo, cache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)

	refreshedAccount, err := provider.RefreshAccount(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, refreshedAccount)
	require.Equal(t, "fresh-access", refreshedAccount.GetCredential("access_token"))
	require.Equal(t, "fresh-refresh", refreshedAccount.GetCredential("refresh_token"))
	require.Equal(t, 1, executor.refreshCalls)
	require.Equal(t, 1, repo.updateCredentialsCalls)
}

func TestKiroTokenProvider_RefreshAccount_FallsBackToExecutor(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       52,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-refresh",
			"machine_id":    "machine-2",
		},
	}
	repo := &refreshAPIAccountRepo{account: account}
	executor := &refreshAPIExecutorStub{
		needsRefresh: true,
		credentials: map[string]any{
			"access_token":  "fallback-access",
			"refresh_token": "fallback-refresh",
			"expires_at":    time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
		},
	}

	provider := NewKiroTokenProvider(repo, nil)
	provider.SetRefreshAPI(nil, executor)

	refreshedAccount, err := provider.RefreshAccount(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, refreshedAccount)
	require.Equal(t, "fallback-access", refreshedAccount.GetCredential("access_token"))
	require.Equal(t, 1, executor.refreshCalls)
	require.Equal(t, 1, repo.updateCredentialsCalls)
	require.Equal(t, "fallback-refresh", repo.account.GetCredential("refresh_token"))
}

func TestKiroTokenProvider_RefreshAccount_WaitsForLockHolderToPersist(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       53,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "stale-access",
			"refresh_token": "stale-refresh",
			"machine_id":    "machine-3",
		},
	}
	repo := &refreshAPIAccountRepo{account: &Account{
		ID:          account.ID,
		Platform:    account.Platform,
		Type:        account.Type,
		Credentials: cloneCredentials(account.Credentials),
	}}
	cache := &refreshAPICacheStub{lockResult: false}
	provider := NewKiroTokenProvider(repo, cache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), &refreshAPIExecutorStub{needsRefresh: true})

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = repo.UpdateCredentials(context.Background(), account.ID, MergeCredentials(account.Credentials, map[string]any{
			"access_token":   "fresh-access",
			"_token_version": "123",
		}))
	}()

	refreshedAccount, err := provider.RefreshAccount(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, refreshedAccount)
	require.Equal(t, "fresh-access", refreshedAccount.GetCredential("access_token"))
}

func TestKiroTokenProvider_RefreshAccount_LockHeldWithoutUpdateReturnsError(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       54,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "stale-access",
			"refresh_token": "stale-refresh",
			"machine_id":    "machine-4",
		},
	}
	repo := &refreshAPIAccountRepo{account: account}
	cache := &refreshAPICacheStub{lockResult: false}
	provider := NewKiroTokenProvider(repo, cache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), &refreshAPIExecutorStub{needsRefresh: true})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	refreshedAccount, err := provider.RefreshAccount(ctx, account)
	require.Error(t, err)
	require.Nil(t, refreshedAccount)
}

func TestKiroTokenProvider_GetAccessTokenSyncsRefreshedAccountCredentials(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       55,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "stale-access",
			"refresh_token": "stale-refresh",
			"expires_at":    time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		},
	}
	repo := &refreshAPIAccountRepo{account: account}
	cache := &refreshAPICacheStub{lockResult: true}
	executor := &refreshAPIExecutorStub{
		needsRefresh: true,
		credentials: map[string]any{
			"access_token":  "fresh-access",
			"refresh_token": "fresh-refresh",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"profile_arn":   "arn:aws:bedrock:us-east-1:123456789012:inference-profile/fresh",
		},
	}

	provider := NewKiroTokenProvider(repo, cache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "fresh-access", token)
	require.Equal(t, "fresh-access", account.GetCredential("access_token"))
	require.Equal(t, "arn:aws:bedrock:us-east-1:123456789012:inference-profile/fresh", account.GetCredential("profile_arn"))
}

func TestKiroTokenProvider_GetAccessTokenReturnsRefreshError(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       56,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "stale-access",
			"refresh_token": "stale-refresh",
			"expires_at":    time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		},
	}
	repo := &refreshAPIAccountRepo{account: account}
	cache := &refreshAPICacheStub{lockResult: true}
	executor := &refreshAPIExecutorStub{
		needsRefresh: true,
		err:          errors.New("invalid_grant"),
	}

	provider := NewKiroTokenProvider(repo, cache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)

	token, err := provider.GetAccessToken(context.Background(), account)
	require.Error(t, err)
	require.Empty(t, token)
	require.Equal(t, "stale-access", account.GetCredential("access_token"))
}

func TestKiroTokenProvider_GetAccessToken_WaitsForLockHolderToPersist(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       57,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "stale-refresh",
			"expires_at":    time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		},
	}
	repo := &refreshAPIAccountRepo{account: &Account{
		ID:          account.ID,
		Platform:    account.Platform,
		Type:        account.Type,
		Credentials: cloneCredentials(account.Credentials),
	}}
	cache := &refreshAPICacheStub{lockResult: false}
	provider := NewKiroTokenProvider(repo, cache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), &refreshAPIExecutorStub{needsRefresh: true})

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = repo.UpdateCredentials(context.Background(), account.ID, MergeCredentials(account.Credentials, map[string]any{
			"access_token":   "fresh-access",
			"refresh_token":  "fresh-refresh",
			"_token_version": "123",
		}))
	}()

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "fresh-access", token)
	require.Equal(t, "fresh-access", account.GetCredential("access_token"))
	require.Equal(t, "fresh-refresh", account.GetCredential("refresh_token"))
}

func TestKiroTokenProvider_GetAccessToken_LockHeldWithoutUpdateReturnsError(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       58,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "stale-refresh",
			"expires_at":    time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		},
	}
	repo := &refreshAPIAccountRepo{account: account}
	cache := &refreshAPICacheStub{lockResult: false}
	provider := NewKiroTokenProvider(repo, cache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), &refreshAPIExecutorStub{needsRefresh: true})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	token, err := provider.GetAccessToken(ctx, account)
	require.Error(t, err)
	require.Empty(t, token)
}
