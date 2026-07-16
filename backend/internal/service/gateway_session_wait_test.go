//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sessionWaitLimitCacheStub struct {
	SessionLimitCache
	active        bool
	activeCount   int
	registerCalls int
	allowRegister bool
}

func (s *sessionWaitLimitCacheStub) RegisterSession(context.Context, int64, string, int, time.Duration) (bool, error) {
	s.registerCalls++
	return s.allowRegister, nil
}

func (s *sessionWaitLimitCacheStub) IsSessionActive(context.Context, int64, string) (bool, error) {
	return s.active, nil
}

func (s *sessionWaitLimitCacheStub) GetActiveSessionCount(context.Context, int64) (int, error) {
	return s.activeCount, nil
}

func TestGatewaySessionWaitChecksWithoutRegisteringUntilSlotAcquire(t *testing.T) {
	cache := &sessionWaitLimitCacheStub{activeCount: 1, allowRegister: true}
	svc := &GatewayService{sessionLimitCache: cache}
	account := &Account{
		ID:       77,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"max_sessions": 2},
	}

	require.True(t, svc.sessionQuotaAllows(context.Background(), account, "session-new"))
	require.Zero(t, cache.registerCalls, "building a WaitPlan must not consume a session")

	require.True(t, svc.RegisterSessionAfterAcquire(context.Background(), account, "session-new"))
	require.Equal(t, 1, cache.registerCalls)
}

func TestGatewaySessionRegisterAfterWaitCanRejectConcurrentQuotaRace(t *testing.T) {
	cache := &sessionWaitLimitCacheStub{activeCount: 1, allowRegister: false}
	svc := &GatewayService{sessionLimitCache: cache}
	account := &Account{
		ID:       78,
		Platform: PlatformAnthropic,
		Type:     AccountTypeSetupToken,
		Extra:    map[string]any{"max_sessions": 2},
	}

	require.True(t, svc.sessionQuotaAllows(context.Background(), account, "session-race"))
	require.False(t, svc.RegisterSessionAfterAcquire(context.Background(), account, "session-race"))
	require.Equal(t, 1, cache.registerCalls)
}
