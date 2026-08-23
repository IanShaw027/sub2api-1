//go:build unit

package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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

func TestSlotLadder_WaitPlanTimeoutClampedTo30s(t *testing.T) {
	require.Equal(t, 30*time.Second, clampLadderWaitTimeout(45*time.Second), "45s sticky default must two-shot at 30s")
	require.Equal(t, 30*time.Second, clampLadderWaitTimeout(120*time.Second), "120s StickySessionWaitTimeout must two-shot at 30s")
	require.Equal(t, 10*time.Second, clampLadderWaitTimeout(10*time.Second), "shorter plans must be kept")
	require.Equal(t, 30*time.Second, clampLadderWaitTimeout(0))

	acc := &service.Account{ID: 1, Concurrency: 12}
	c, _ := newHelperTestContext(http.MethodPost, "/v1/messages")
	started := false
	params := accountSlotLadderParams(c, acc, &service.AccountWaitPlan{
		Timeout:        45 * time.Second,
		MaxWaiting:     3,
		MaxConcurrency: 12,
	}, false, &started)
	require.Equal(t, 30*time.Second, params.Timeout)

	params = accountSlotLadderParams(c, acc, &service.AccountWaitPlan{Timeout: 120 * time.Second}, false, &started)
	require.Equal(t, 30*time.Second, params.Timeout)
}

func TestAcquireResponsesAccountSlot_PostSwitchAdmissionBindPreservesOriginal(t *testing.T) {
	const sessionHash = "sess"
	cache := &handlerStickyCache{bindings: map[string]int64{"openai:" + sessionHash: 11}}
	svc := newOpenAIGatewayServiceForHandlerTest(cache)
	helper := newSlotLadderHelper(&helperConcurrencyCacheStub{accountSeq: []bool{true}})
	h := &OpenAIGatewayHandler{gatewayService: svc, concurrencyHelper: helper}

	c, _ := newHelperTestContext(http.MethodPost, "/v1/responses")
	c.Request = c.Request.WithContext(service.WithPreserveStickyBinding(c.Request.Context()))
	streamStarted := false
	groupID := int64(1)
	selection := &service.AccountSelectionResult{
		Account: &service.Account{
			ID:          22,
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Concurrency: 12,
			Status:      service.StatusActive,
			Schedulable: true,
		},
		Acquired: false,
	}

	release, status := h.acquireResponsesAccountSlot(c, &groupID, sessionHash, selection, false, &streamStarted, zap.NewNop())
	require.Equal(t, openAISlotAcquireOK, status)
	require.NotNil(t, release)
	release()
	require.Equal(t, int64(11), cache.bindings["openai:"+sessionHash], "switch → immediate acquire → admission-bind must keep the original sticky account")
}

func TestAcquireWebSearchAccountSlot_SwitchSetsPreserveAndImmediateNOnly(t *testing.T) {
	cache := &helperConcurrencyCacheStub{
		accountSeq:         []bool{false},
		accountWaitAllowed: slotLadderBoolPtr(false),
	}
	h := &GatewayHandler{concurrencyHelper: newSlotLadderHelper(cache)}
	c, _ := newHelperTestContext(http.MethodPost, "/v1/ws/search")
	selected := &service.AccountSelectionResult{
		Account: &service.Account{ID: 101, Concurrency: 12},
		WaitPlan: &service.AccountWaitPlan{
			AccountID:      101,
			MaxConcurrency: 12,
			Timeout:        40 * time.Millisecond,
			MaxWaiting:     4,
		},
	}

	release, ok, err := h.acquireWebSearchAccountSlot(c, selected)
	require.False(t, ok)
	require.Nil(t, release)
	require.NoError(t, err)
	require.True(t, service.PreserveStickyBindingFromContext(c.Request.Context()), "web-search switch must set Preserve")

	post := &helperConcurrencyCacheStub{accountSeq: []bool{false}}
	h.concurrencyHelper = newSlotLadderHelper(post)
	next := &service.AccountSelectionResult{
		Account: &service.Account{ID: 202, Concurrency: 12},
		WaitPlan: &service.AccountWaitPlan{
			AccountID:      202,
			MaxConcurrency: 12,
			Timeout:        30 * time.Second,
			MaxWaiting:     4,
		},
	}
	started := time.Now()
	_, ok, _ = h.acquireWebSearchAccountSlot(c, next)
	require.False(t, ok)
	require.Less(t, time.Since(started), 500*time.Millisecond, "post-switch web-search must not wait")
	require.Equal(t, []int{12}, post.accountAcquireMaxes)
	require.Equal(t, 0, post.accountWaitCalls)
}

func newOpenAIGatewayServiceForHandlerTest(cache service.GatewayCache) *service.OpenAIGatewayService {
	return service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil,
		cache,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

type handlerStickyCache struct {
	bindings map[string]int64
}

func (c *handlerStickyCache) GetSessionAccountID(_ context.Context, _ int64, sessionHash string) (int64, error) {
	if id, ok := c.bindings[sessionHash]; ok {
		return id, nil
	}
	return 0, service.ErrStickySessionNotFound
}

func (c *handlerStickyCache) SetSessionAccountID(_ context.Context, _ int64, sessionHash string, accountID int64, _ time.Duration) error {
	if c.bindings == nil {
		c.bindings = make(map[string]int64)
	}
	c.bindings[sessionHash] = accountID
	return nil
}

func (c *handlerStickyCache) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}

func (c *handlerStickyCache) DeleteSessionAccountID(_ context.Context, _ int64, sessionHash string) error {
	delete(c.bindings, sessionHash)
	return nil
}

func (c *handlerStickyCache) SetGrokVideoPendingBilling(context.Context, string, []byte, time.Duration) error {
	return nil
}
func (c *handlerStickyCache) GetGrokVideoPendingBilling(context.Context, string) ([]byte, error) {
	return nil, nil
}
func (c *handlerStickyCache) ClaimGrokVideoBilled(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}
func (c *handlerStickyCache) ReleaseGrokVideoBilled(context.Context, string) error {
	return nil
}
func (c *handlerStickyCache) SetReasoningContent(context.Context, string, string, time.Duration) error {
	return nil
}
func (c *handlerStickyCache) GetReasoningContent(context.Context, string) (string, error) {
	return "", service.ErrReasoningContentNotFound
}
