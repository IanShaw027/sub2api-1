//go:build unit

package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSelectAndAcquireGrokRealtimeAccount_SwitchContinuesSelection(t *testing.T) {
	c, rec := newHelperTestContext(http.MethodGet, "/v1/realtime")
	attempts := 0
	selection, release, status := runOpenAISlotSwitchSelection(c, 4, func(failed map[int64]struct{}) (*service.AccountSelectionResult, func(), openAISlotAcquireResult) {
		attempts++
		if _, skipped := failed[1]; !skipped {
			return &service.AccountSelectionResult{
				Account: &service.Account{ID: 1, Platform: service.PlatformGrok, Concurrency: 1},
			}, nil, openAISlotAcquireSwitchAccount
		}
		return &service.AccountSelectionResult{
			Account: &service.Account{ID: 2, Platform: service.PlatformGrok, Concurrency: 1},
		}, func() {}, openAISlotAcquireOK
	})

	require.Equal(t, openAISlotAcquireOK, status, "pre-accept switch must continue selection instead of 503")
	require.Equal(t, 2, attempts)
	require.NotNil(t, selection)
	require.Equal(t, int64(2), selection.Account.ID)
	require.NotNil(t, release)
	require.NotEqual(t, http.StatusServiceUnavailable, rec.Code)
	require.True(t, service.PreserveStickyBindingFromContext(c.Request.Context()))
}

func TestSelectAndAcquireGrokRealtimeAccount_PreAcceptWaitDoesNotStreamPing(t *testing.T) {
	cache := &grokRealtimeSlotCache{allow: map[int64]bool{1: false, 2: true}}
	concurrency := service.NewConcurrencyService(cache)
	concurrency.SetSlotHeartbeatInterval(0)
	helper := NewConcurrencyHelper(concurrency, SSEPingFormatComment, 10*time.Millisecond)

	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.Scheduling.FallbackWaitTimeout = 80 * time.Millisecond
	cfg.Gateway.Scheduling.FallbackMaxWaiting = 4

	svc := service.NewOpenAIGatewayService(
		grokRealtimeAccountRepo{accounts: []service.Account{
			newGrokRealtimeTestAccount(1, 0),
			newGrokRealtimeTestAccount(2, 10),
		}},
		nil, nil, nil, nil, nil, nil,
		cfg,
		nil,
		concurrency,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	h := &OpenAIGatewayHandler{gatewayService: svc, concurrencyHelper: helper}

	c, rec := newHelperTestContext(http.MethodGet, "/v1/realtime")
	selection, release, status := h.selectAndAcquireGrokRealtimeAccount(c, nil, zap.NewNop())

	require.NotContains(t, rec.Body.String(), string(SSEPingFormatComment), "pre-accept wait must not write SSE pings")
	require.False(t, rec.Flushed, "pre-accept wait must not flush a started stream")
	require.False(t, c.Writer.Written(), "pre-accept wait must not commit the HTTP response")
	require.Equal(t, openAISlotAcquireOK, status, "deadline Burst miss must switch/continue, not abort")
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(2), selection.Account.ID, "first-account deadline miss must continue to the next account")
	require.NotNil(t, release)
	release()
	require.True(t, service.PreserveStickyBindingFromContext(c.Request.Context()))
	require.Greater(t, cache.accountWaitCalls, 0, "must exercise the real wait/ping path, not a mocked retry status")
}

func newGrokRealtimeTestAccount(id int64, priority int) service.Account {
	return service.Account{
		ID:          id,
		Platform:    service.PlatformGrok,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    priority,
	}
}

type grokRealtimeAccountRepo struct {
	service.AccountRepository
	accounts []service.Account
}

func (r grokRealtimeAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			acc := r.accounts[i]
			return &acc, nil
		}
	}
	return nil, context.Canceled
}

func (r grokRealtimeAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]service.Account, error) {
	return r.listByPlatform(platform), nil
}

func (r grokRealtimeAccountRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return r.listByPlatform(platform), nil
}

func (r grokRealtimeAccountRepo) ListSchedulableUngroupedByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	return r.listByPlatform(platform), nil
}

func (r grokRealtimeAccountRepo) listByPlatform(platform string) []service.Account {
	out := make([]service.Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			out = append(out, acc)
		}
	}
	return out
}

type grokRealtimeSlotCache struct {
	helperConcurrencyCacheStub
	allow map[int64]bool
}

func (s *grokRealtimeSlotCache) AcquireAccountSlot(_ context.Context, accountID int64, maxConcurrency int, _ string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountAcquireCalls++
	s.accountAcquireMaxes = append(s.accountAcquireMaxes, maxConcurrency)
	return s.allow[accountID], nil
}
