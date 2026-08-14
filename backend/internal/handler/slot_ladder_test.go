//go:build unit

package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func slotLadderBoolPtr(v bool) *bool { return &v }

func newSlotLadderHelper(cache *helperConcurrencyCacheStub) *ConcurrencyHelper {
	concurrency := service.NewConcurrencyService(cache)
	concurrency.SetSlotHeartbeatInterval(0)
	return NewConcurrencyHelper(concurrency, SSEPingFormatNone, 5*time.Millisecond)
}

func TestSlotLadder_ThirteenthRequestWaitsDoesNotBurstBeforeDeadline(t *testing.T) {
	const n, burst = 12, 14
	cache := &helperConcurrencyCacheStub{
		accountSeq: []bool{false, true},
	}
	helper := newSlotLadderHelper(cache)
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	streamStarted := false

	result := helper.AcquireAccountSlotLadder(c, AccountSlotLadderParams{
		AccountID:     101,
		NormalLimit:   n,
		BurstLimit:    burst,
		Timeout:       time.Second,
		MaxWaiting:    4,
		StreamStarted: &streamStarted,
	})
	require.Equal(t, SlotAcquiredNormal, result.Decision)
	require.NoError(t, result.Err)
	require.NotNil(t, result.ReleaseFunc)
	result.ReleaseFunc()

	require.GreaterOrEqual(t, cache.accountAcquireCalls, 2)
	for i, max := range cache.accountAcquireMaxes {
		require.Equal(t, n, max, "acquire[%d] must use N before deadline, got %d", i, max)
	}
}

func TestSlotLadder_WaitIntervalNeverAcquiresBurst(t *testing.T) {
	const n, burst = 12, 14
	cache := &helperConcurrencyCacheStub{
		accountSeq: []bool{false, false, true},
	}
	helper := newSlotLadderHelper(cache)
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	streamStarted := false

	result := helper.AcquireAccountSlotLadder(c, AccountSlotLadderParams{
		AccountID:     101,
		NormalLimit:   n,
		BurstLimit:    burst,
		Timeout:       time.Second,
		MaxWaiting:    4,
		StreamStarted: &streamStarted,
	})
	require.Equal(t, SlotAcquiredNormal, result.Decision)
	require.NotNil(t, result.ReleaseFunc)
	result.ReleaseFunc()

	for i, max := range cache.accountAcquireMaxes {
		require.NotEqual(t, burst, max, "wait-interval acquire[%d] must not use Burst", i)
		require.Equal(t, n, max)
	}
}

func TestSlotLadder_DeadlineTwoShotNThenBurst(t *testing.T) {
	const n, burst = 12, 14
	cache := &helperConcurrencyCacheStub{}
	helper := newSlotLadderHelper(cache)
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	streamStarted := false

	result := helper.AcquireAccountSlotLadder(c, AccountSlotLadderParams{
		AccountID:     101,
		NormalLimit:   n,
		BurstLimit:    burst,
		Timeout:       40 * time.Millisecond,
		MaxWaiting:    4,
		StreamStarted: &streamStarted,
	})
	require.Equal(t, SlotSwitchAccountPreserveBinding, result.Decision)
	require.Nil(t, result.ReleaseFunc)

	require.GreaterOrEqual(t, len(cache.accountAcquireMaxes), 2)
	last := cache.accountAcquireMaxes[len(cache.accountAcquireMaxes)-2:]
	require.Equal(t, []int{n, burst}, last, "deadline two-shot must be N then Burst")
	for i, max := range cache.accountAcquireMaxes[:len(cache.accountAcquireMaxes)-1] {
		require.Equal(t, n, max, "pre-burst acquire[%d] must be N", i)
	}
}

func TestSlotLadder_BothFailSwitchPreservesBinding(t *testing.T) {
	const n, burst = 12, 14
	cache := &helperConcurrencyCacheStub{}
	helper := newSlotLadderHelper(cache)
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	streamStarted := false

	result := helper.AcquireAccountSlotLadder(c, AccountSlotLadderParams{
		AccountID:     101,
		NormalLimit:   n,
		BurstLimit:    burst,
		Timeout:       40 * time.Millisecond,
		MaxWaiting:    4,
		StreamStarted: &streamStarted,
	})
	require.Equal(t, SlotSwitchAccountPreserveBinding, result.Decision)

	fs := NewFailoverState(3, true)
	action := continueAfterSlotSwitch(c, fs, 101)
	require.Equal(t, FailoverContinue, action)
	_, failed := fs.FailedAccountIDs[101]
	require.True(t, failed)
	require.True(t, service.PreserveStickyBindingFromContext(c.Request.Context()))
}

func TestSlotLadder_WaitQueueFullSwitchesNot429(t *testing.T) {
	cache := &helperConcurrencyCacheStub{
		accountSeq:         []bool{false},
		accountWaitAllowed: slotLadderBoolPtr(false),
	}
	helper := newSlotLadderHelper(cache)
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	streamStarted := false

	result := helper.AcquireAccountSlotLadder(c, AccountSlotLadderParams{
		AccountID:     101,
		NormalLimit:   12,
		BurstLimit:    14,
		Timeout:       200 * time.Millisecond,
		MaxWaiting:    4,
		StreamStarted: &streamStarted,
	})
	require.Equal(t, SlotSwitchAccountPreserveBinding, result.Decision)
	require.Nil(t, result.ReleaseFunc)
	var waitErr *WaitQueueFullError
	require.False(t, result.Err != nil && waitErr != nil)
	require.Equal(t, 1, cache.accountWaitCalls)
	require.Equal(t, 1, cache.accountAcquireCalls, "wait-queue-full must not enter the 30s poll")
}

func TestSlotLadder_PostSwitchImmediateNOnly(t *testing.T) {
	const n, burst = 12, 14
	cache := &helperConcurrencyCacheStub{
		accountSeq: []bool{false},
	}
	helper := newSlotLadderHelper(cache)
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	streamStarted := false

	started := time.Now()
	result := helper.AcquireAccountSlotLadder(c, AccountSlotLadderParams{
		AccountID:     202,
		NormalLimit:   n,
		BurstLimit:    burst,
		Timeout:       30 * time.Second,
		MaxWaiting:    4,
		StreamStarted: &streamStarted,
		ImmediateOnly: true,
	})
	elapsed := time.Since(started)
	require.Less(t, elapsed, 500*time.Millisecond, "post-switch must not wait")
	require.Equal(t, SlotSwitchAccountPreserveBinding, result.Decision)
	require.Equal(t, []int{n}, cache.accountAcquireMaxes)
	require.Equal(t, 0, cache.accountWaitCalls)
}

func TestSlotLadder_StreamStartedNoSwitch(t *testing.T) {
	const n, burst = 12, 14
	cache := &helperConcurrencyCacheStub{}
	helper := newSlotLadderHelper(cache)
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	streamStarted := true

	result := helper.AcquireAccountSlotLadder(c, AccountSlotLadderParams{
		AccountID:     101,
		NormalLimit:   n,
		BurstLimit:    burst,
		Timeout:       40 * time.Millisecond,
		MaxWaiting:    4,
		StreamStarted: &streamStarted,
	})
	require.Equal(t, SlotAbortRequest, result.Decision)
	require.NotEqual(t, SlotSwitchAccountPreserveBinding, result.Decision)
	var cErr *ConcurrencyError
	require.ErrorAs(t, result.Err, &cErr)
	require.True(t, cErr.IsTimeout)
}

func TestSlotLadder_DeadlineBurstSucceedsOnlyAtTwoShot(t *testing.T) {
	const n, burst = 12, 14
	cache := &helperConcurrencyCacheStub{
		acquireByMax: map[int]bool{n: false, burst: true},
	}
	helper := newSlotLadderHelper(cache)
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	streamStarted := false

	result := helper.AcquireAccountSlotLadder(c, AccountSlotLadderParams{
		AccountID:     101,
		NormalLimit:   n,
		BurstLimit:    burst,
		Timeout:       40 * time.Millisecond,
		MaxWaiting:    4,
		StreamStarted: &streamStarted,
	})
	require.Equal(t, SlotAcquiredBurst, result.Decision)
	require.NotNil(t, result.ReleaseFunc)
	result.ReleaseFunc()
	require.Equal(t, burst, cache.accountAcquireMaxes[len(cache.accountAcquireMaxes)-1])
	for i, max := range cache.accountAcquireMaxes[:len(cache.accountAcquireMaxes)-1] {
		require.Equal(t, n, max, "pre-deadline acquire[%d] must be N", i)
	}
}

func TestFailoverState_RecordConcurrencyTimeout(t *testing.T) {
	fs := NewFailoverState(1, true)
	require.Equal(t, FailoverContinue, fs.RecordConcurrencyTimeout(7))
	_, ok := fs.FailedAccountIDs[7]
	require.True(t, ok)
	require.Equal(t, 1, fs.SwitchCount)
	require.Equal(t, FailoverExhausted, fs.RecordConcurrencyTimeout(8))
	_, ok = fs.FailedAccountIDs[8]
	require.True(t, ok)
}
