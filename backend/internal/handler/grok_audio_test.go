//go:build unit

package handler

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
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
