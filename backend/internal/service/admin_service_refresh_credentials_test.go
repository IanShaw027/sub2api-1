//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type adminRefreshAccountRepoStub struct {
	mockAccountRepoForGemini
	account                *Account
	updateCredentialsCalls int
}

func (r *adminRefreshAccountRepoStub) GetByID(_ context.Context, _ int64) (*Account, error) {
	if r.account == nil {
		return nil, ErrAccountNotFound
	}
	return r.account, nil
}

func (r *adminRefreshAccountRepoStub) UpdateCredentials(_ context.Context, id int64, credentials map[string]any) error {
	r.updateCredentialsCalls++
	if r.account == nil || r.account.ID != id {
		return errors.New("unexpected account id")
	}
	r.account.Credentials = cloneCredentials(credentials)
	return nil
}

type adminRefreshExecutorStub struct {
	platform     string
	credentials  map[string]any
	refreshCalls int
}

func (e *adminRefreshExecutorStub) CanRefresh(account *Account) bool {
	return account != nil && account.Platform == e.platform && account.Type == AccountTypeOAuth
}

func (e *adminRefreshExecutorStub) NeedsRefresh(_ *Account, _ time.Duration) bool { return true }

func (e *adminRefreshExecutorStub) Refresh(_ context.Context, _ *Account) (map[string]any, error) {
	e.refreshCalls++
	return e.credentials, nil
}

func (e *adminRefreshExecutorStub) CacheKey(account *Account) string {
	return "admin-refresh:" + account.Platform
}

func TestAdminServiceRefreshAccountCredentialsRejectsNonOAuthAccount(t *testing.T) {
	repo := &adminRefreshAccountRepoStub{account: &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.RefreshAccountCredentials(context.Background(), 1)

	require.Nil(t, updated)
	require.Error(t, err)
	require.Equal(t, "NOT_OAUTH", infraerrors.Reason(err))
	require.Equal(t, 0, repo.updateCredentialsCalls)
}

func TestAdminServiceRefreshAccountCredentialsRejectsMissingRefreshToken(t *testing.T) {
	repo := &adminRefreshAccountRepoStub{account: &Account{
		ID:          2,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "old-access"},
	}}
	svc := &adminServiceImpl{accountRepo: repo, oauthRefreshExecutors: []OAuthRefreshExecutor{
		&adminRefreshExecutorStub{platform: PlatformOpenAI, credentials: map[string]any{"access_token": "new-access"}},
	}}

	updated, err := svc.RefreshAccountCredentials(context.Background(), 2)

	require.Nil(t, updated)
	require.Error(t, err)
	require.Equal(t, "MISSING_REFRESH_TOKEN", infraerrors.Reason(err))
	require.Equal(t, 0, repo.updateCredentialsCalls)
}

func TestAdminServiceRefreshAccountCredentialsRejectsUnsupportedPlatform(t *testing.T) {
	repo := &adminRefreshAccountRepoStub{account: &Account{
		ID:       3,
		Platform: PlatformSora,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-refresh",
		},
	}}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.RefreshAccountCredentials(context.Background(), 3)

	require.Nil(t, updated)
	require.Error(t, err)
	require.Equal(t, "UNSUPPORTED_REFRESH_PLATFORM", infraerrors.Reason(err))
	require.Equal(t, 0, repo.updateCredentialsCalls)
}

func TestAdminServiceRefreshAccountCredentialsCallsExecutorPersistsAndReturnsUpdatedAccount(t *testing.T) {
	repo := &adminRefreshAccountRepoStub{account: &Account{
		ID:       4,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "old-access",
			"refresh_token": "old-refresh",
			"custom":        "preserved",
		},
	}}
	executor := &adminRefreshExecutorStub{
		platform: PlatformOpenAI,
		credentials: map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"custom":        "preserved",
		},
	}
	svc := &adminServiceImpl{
		accountRepo:           repo,
		oauthRefreshAPI:       NewOAuthRefreshAPI(repo, nil),
		oauthRefreshExecutors: []OAuthRefreshExecutor{executor},
	}

	updated, err := svc.RefreshAccountCredentials(context.Background(), 4)

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 1, executor.refreshCalls)
	require.Equal(t, 1, repo.updateCredentialsCalls)
	require.Equal(t, "new-access", updated.Credentials["access_token"])
	require.Equal(t, "new-refresh", updated.Credentials["refresh_token"])
	require.Equal(t, "preserved", updated.Credentials["custom"])
	require.NotNil(t, updated.Credentials["_token_version"])
}
