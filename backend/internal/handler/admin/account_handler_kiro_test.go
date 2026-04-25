package admin

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type kiroHandlerRefreshExecutorStub struct {
	t            *testing.T
	credentials  map[string]any
	refreshCalls int
}

func (s *kiroHandlerRefreshExecutorStub) CanRefresh(account *service.Account) bool {
	return account != nil && account.Platform == service.PlatformKiro
}

func (s *kiroHandlerRefreshExecutorStub) NeedsRefresh(_ *service.Account, _ time.Duration) bool {
	return true
}

func (s *kiroHandlerRefreshExecutorStub) Refresh(_ context.Context, account *service.Account) (map[string]any, error) {
	s.refreshCalls++
	require.Equal(s.t, service.PlatformKiro, account.Platform)
	return s.credentials, nil
}

func (s *kiroHandlerRefreshExecutorStub) CacheKey(account *service.Account) string {
	return service.KiroTokenCacheKey(account)
}

func TestAccountHandlerRefreshSingleAccount_KiroUsesTokenProvider(t *testing.T) {
	t.Parallel()

	adminSvc := newStubAdminService()

	provider := service.NewKiroTokenProvider(nil, nil)
	executor := &kiroHandlerRefreshExecutorStub{
		t: t,
		credentials: map[string]any{
			"access_token":  "fresh-access-token",
			"refresh_token": "fresh-refresh-token",
			"expires_at":    "2030-01-01T00:00:00Z",
			"machine_id":    "machine-1",
		},
	}
	provider.SetRefreshAPI(nil, executor)
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, provider, nil, nil, nil, nil, nil, nil, nil, nil)

	account := &service.Account{
		ID:       101,
		Platform: service.PlatformKiro,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "old-refresh-token",
			"machine_id":    "machine-1",
		},
	}

	updated, warning, err := handler.refreshSingleAccount(context.Background(), account)

	require.NoError(t, err)
	require.Empty(t, warning)
	require.Equal(t, 1, executor.refreshCalls)
	require.NotNil(t, updated)
}
