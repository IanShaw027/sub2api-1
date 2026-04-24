package admin

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountHandlerRefreshSingleAccount_KiroUsesKiroRefresher(t *testing.T) {
	t.Parallel()

	adminSvc := newStubAdminService()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	var refreshed bool
	handler.kiroRefresh = func(_ context.Context, account *service.Account) (map[string]any, error) {
		refreshed = true
		require.Equal(t, service.PlatformKiro, account.Platform)
		return map[string]any{
			"access_token":  "fresh-access-token",
			"refresh_token": "fresh-refresh-token",
			"expires_at":    "2030-01-01T00:00:00Z",
			"machine_id":    "machine-1",
		}, nil
	}

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
	require.True(t, refreshed)
	require.NotNil(t, updated)
}
