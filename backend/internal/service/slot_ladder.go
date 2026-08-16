package service

import (
	"context"
	"time"
)

type preserveStickyBindingContextKey struct{}

// WithPreserveStickyBinding marks this request as a sticky-preserving failover.
// Selection must not overwrite the original session binding, and the failover
// account only gets an immediate normal-slot acquire (no 30s wait, no burst).
func WithPreserveStickyBinding(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, preserveStickyBindingContextKey{}, true)
}

func PreserveStickyBindingFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(preserveStickyBindingContextKey{}).(bool)
	return v
}

func accountIsSlotCandidate(acc *Account, loadInfo *AccountLoadInfo, stickyAccountID int64) bool {
	if acc == nil {
		return false
	}
	if stickyAccountID > 0 && acc.ID == stickyAccountID {
		return true
	}
	if loadInfo == nil {
		return true
	}
	return loadInfo.CurrentConcurrency < acc.BurstConcurrency()
}

func preferInstantFanoutAccounts(available []accountWithLoad) []accountWithLoad {
	if len(available) == 0 {
		return available
	}
	free := make([]accountWithLoad, 0, len(available))
	for _, item := range available {
		if item.account == nil || item.loadInfo == nil {
			free = append(free, item)
			continue
		}
		if item.loadInfo.CurrentConcurrency < item.account.EffectiveConcurrency() {
			free = append(free, item)
		}
	}
	if len(free) > 0 {
		return free
	}
	return available
}

func slotLadderMaxWaiting(configured int, account *Account) int {
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

func newAccountWaitPlan(account *Account, timeout time.Duration, configuredMaxWaiting int) *AccountWaitPlan {
	if account == nil {
		return nil
	}
	return &AccountWaitPlan{
		AccountID:      account.ID,
		MaxConcurrency: account.EffectiveConcurrency(),
		Timeout:        timeout,
		MaxWaiting:     slotLadderMaxWaiting(configuredMaxWaiting, account),
	}
}

func waitPlanUnlessPostSwitch(ctx context.Context, account *Account, timeout time.Duration, configuredMaxWaiting int) *AccountWaitPlan {
	if PreserveStickyBindingFromContext(ctx) {
		return nil
	}
	return newAccountWaitPlan(account, timeout, configuredMaxWaiting)
}
