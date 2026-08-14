package handler

import (
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type SlotDecision int

const (
	SlotAcquiredNormal SlotDecision = iota
	SlotAcquiredBurst
	SlotWaitThenRetry
	SlotSwitchAccountPreserveBinding
	SlotAbortRequest
	SlotInfrastructureError
)

type SlotAcquireResult struct {
	Decision    SlotDecision
	ReleaseFunc func()
	Err         error
}

type AccountSlotLadderParams struct {
	AccountID     int64
	NormalLimit   int
	BurstLimit    int
	Timeout       time.Duration
	MaxWaiting    int
	IsStream      bool
	StreamStarted *bool
	ImmediateOnly bool
}

func (h *ConcurrencyHelper) AcquireAccountSlotLadder(c *gin.Context, p AccountSlotLadderParams) SlotAcquireResult {
	if h == nil || h.concurrencyService == nil {
		return SlotAcquireResult{Decision: SlotInfrastructureError, Err: fmt.Errorf("concurrency service is unavailable")}
	}
	if c == nil || c.Request == nil {
		return SlotAcquireResult{Decision: SlotInfrastructureError, Err: fmt.Errorf("request context is unavailable")}
	}
	if p.NormalLimit <= 0 {
		return SlotAcquireResult{Decision: SlotInfrastructureError, Err: service.ErrInvalidConcurrency}
	}
	if p.BurstLimit < p.NormalLimit {
		p.BurstLimit = p.NormalLimit
	}
	if p.Timeout <= 0 {
		p.Timeout = maxConcurrencyWait
	}
	if p.StreamStarted == nil {
		started := false
		p.StreamStarted = &started
	}

	ctx := c.Request.Context()
	groupID := h.accountGroupIDFromGin(c)

	release, acquired, err := h.TryAcquireAccountSlotForGroup(ctx, p.AccountID, groupID, p.NormalLimit)
	if err != nil {
		return SlotAcquireResult{Decision: SlotInfrastructureError, Err: err}
	}
	if acquired {
		return SlotAcquireResult{Decision: SlotAcquiredNormal, ReleaseFunc: release}
	}
	if p.ImmediateOnly {
		return slotSwitchOrAbort(p.StreamStarted, nil)
	}

	maxWaiting := p.MaxWaiting
	if maxWaiting < 1 {
		maxWaiting = 3
	}
	canWait, waitErr := h.IncrementAccountWaitCount(ctx, p.AccountID, maxWaiting)
	if waitErr == nil && !canWait {
		return slotSwitchOrAbort(p.StreamStarted, nil)
	}
	accountWaitCounted := waitErr == nil && canWait
	defer func() {
		if accountWaitCounted {
			h.DecrementAccountWaitCount(ctx, p.AccountID)
		}
	}()

	release, err = h.waitForSlotWithPingTimeout(c, "account", p.AccountID, p.NormalLimit, p.Timeout, p.IsStream, p.StreamStarted, false)
	if err == nil && release != nil {
		return SlotAcquireResult{Decision: SlotAcquiredNormal, ReleaseFunc: release}
	}
	if err != nil {
		if parentErr := c.Request.Context().Err(); parentErr != nil {
			return SlotAcquireResult{Decision: SlotInfrastructureError, Err: parentErr}
		}
		var timeoutErr *ConcurrencyError
		if !asConcurrencyTimeout(err, &timeoutErr) {
			return SlotAcquireResult{Decision: SlotInfrastructureError, Err: err}
		}
	}

	release, acquired, err = h.TryAcquireAccountSlotForGroup(c.Request.Context(), p.AccountID, groupID, p.NormalLimit)
	if err != nil {
		return SlotAcquireResult{Decision: SlotInfrastructureError, Err: err}
	}
	if acquired {
		return SlotAcquireResult{Decision: SlotAcquiredNormal, ReleaseFunc: release}
	}
	release, acquired, err = h.TryAcquireAccountSlotForGroup(c.Request.Context(), p.AccountID, groupID, p.BurstLimit)
	if err != nil {
		return SlotAcquireResult{Decision: SlotInfrastructureError, Err: err}
	}
	if acquired {
		return SlotAcquireResult{Decision: SlotAcquiredBurst, ReleaseFunc: release}
	}
	return slotSwitchOrAbort(p.StreamStarted, &ConcurrencyError{SlotType: "account", IsTimeout: true})
}

func asConcurrencyTimeout(err error, dest **ConcurrencyError) bool {
	if err == nil || dest == nil {
		return false
	}
	cErr, ok := err.(*ConcurrencyError)
	if !ok || cErr == nil || !cErr.IsTimeout {
		return false
	}
	*dest = cErr
	return true
}

func slotSwitchOrAbort(streamStarted *bool, timeoutErr error) SlotAcquireResult {
	if streamStarted != nil && *streamStarted {
		if timeoutErr == nil {
			timeoutErr = &ConcurrencyError{SlotType: "account", IsTimeout: true}
		}
		return SlotAcquireResult{Decision: SlotAbortRequest, Err: timeoutErr}
	}
	return SlotAcquireResult{Decision: SlotSwitchAccountPreserveBinding}
}

func applyGatewaySlotLadder(
	c *gin.Context,
	fs *FailoverState,
	accountID int64,
	slot SlotAcquireResult,
	streamStarted bool,
	handleAbort func(err error),
) (release func(), cont bool, done bool) {
	switch slot.Decision {
	case SlotAcquiredNormal, SlotAcquiredBurst:
		return slot.ReleaseFunc, false, false
	case SlotSwitchAccountPreserveBinding:
		if streamStarted {
			if handleAbort != nil {
				handleAbort(slot.Err)
			}
			return nil, false, true
		}
		if continueAfterSlotSwitch(c, fs, accountID) == FailoverExhausted {
			if handleAbort != nil {
				handleAbort(slot.Err)
			}
			return nil, false, true
		}
		return nil, true, false
	default:
		if handleAbort != nil {
			handleAbort(slot.Err)
		}
		return nil, false, true
	}
}

func continueAfterSlotSwitch(c *gin.Context, fs *FailoverState, accountID int64) FailoverAction {
	if c != nil && c.Request != nil {
		c.Request = c.Request.WithContext(service.WithPreserveStickyBinding(c.Request.Context()))
	}
	if fs == nil {
		return FailoverExhausted
	}
	return fs.RecordConcurrencyTimeout(accountID)
}

func runAccountSlotLadder(
	c *gin.Context,
	helper *ConcurrencyHelper,
	account *service.Account,
	waitPlan *service.AccountWaitPlan,
	isStream bool,
	streamStarted *bool,
) SlotAcquireResult {
	if helper == nil {
		return SlotAcquireResult{Decision: SlotInfrastructureError, Err: fmt.Errorf("concurrency helper is unavailable")}
	}
	if account == nil {
		return SlotAcquireResult{Decision: SlotInfrastructureError, Err: fmt.Errorf("account is unavailable")}
	}
	return helper.AcquireAccountSlotLadder(c, accountSlotLadderParams(c, account, waitPlan, isStream, streamStarted))
}

func accountSlotLadderParams(
	c *gin.Context,
	account *service.Account,
	waitPlan *service.AccountWaitPlan,
	isStream bool,
	streamStarted *bool,
) AccountSlotLadderParams {
	params := AccountSlotLadderParams{
		AccountID:     account.ID,
		NormalLimit:   account.EffectiveConcurrency(),
		BurstLimit:    account.BurstConcurrency(),
		Timeout:       maxConcurrencyWait,
		MaxWaiting:    slotLadderMaxWaiting(3, account),
		IsStream:      isStream,
		StreamStarted: streamStarted,
	}
	if c != nil && c.Request != nil {
		params.ImmediateOnly = service.PreserveStickyBindingFromContext(c.Request.Context())
	}
	if waitPlan != nil {
		if waitPlan.MaxConcurrency > 0 {
			params.NormalLimit = waitPlan.MaxConcurrency
		}
		if waitPlan.Timeout > 0 {
			params.Timeout = clampLadderWaitTimeout(waitPlan.Timeout)
		}
		if waitPlan.MaxWaiting > 0 {
			params.MaxWaiting = waitPlan.MaxWaiting
		}
	}
	return params
}

func clampLadderWaitTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 || timeout > maxConcurrencyWait {
		return maxConcurrencyWait
	}
	return timeout
}

func slotLadderMaxWaiting(configured int, account *service.Account) int {
	return serviceSlotLadderMaxWaiting(configured, account)
}

func serviceSlotLadderMaxWaiting(configured int, account *service.Account) int {
	overflow := 1
	if account != nil {
		overflow = account.OverflowConcurrency()
	}
	minWait := overflow + 2
	if minWait < 3 {
		minWait = 3
	}
	if configured > minWait {
		return configured
	}
	return minWait
}
