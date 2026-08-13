package service

import (
	"context"
	"fmt"
	"time"
)

const (
	antigravityCapacityWindowKind    = "quota"
	antigravityCapacityDefaultWindow = 5 * time.Hour
)

// AntigravityCapacityProvider is the CapacityProvider for
// PlatformAntigravity, backed by the antigravity_sched_* extra snapshot
// (antigravity_sched_utilization / antigravity_sched_reset_at /
// antigravity_sched_usage_updated_at) already written by
// buildAntigravitySchedulerSnapshotExtraUpdates
// (antigravity_quota_fetcher.go), which persistAntigravitySchedulerSnapshot
// calls from the existing dashboard usage poll.
//
// Deviation from the task brief: the brief assumed a dedicated pair of 5h/7d
// "exceeded" flags analogous to Anthropic's, plus an antigravity_sched_reset_at
// key. Grepping antigravity_credits_overages.go (the file named in the
// brief) shows it only tracks a single AICredits 429-cooldown key unrelated
// to this feature; there is no persisted is_5h_exceeded/is_7d_exceeded pair
// anywhere for Antigravity. The only real per-account quota signal is a
// single "tightest scope" window: whichever tracked model has the highest
// utilization right now, plus that model's own upstream-reported reset
// timestamp (buildAntigravitySchedulerSnapshotExtraUpdates already reduces
// Antigravity's per-model quota map down to this one tightest window). This
// provider surfaces exactly that single window under a generic "quota" Kind
// rather than inventing a fixed 5h/7d split that doesn't correspond to
// anything upstream actually reports.
type AntigravityCapacityProvider struct {
	accountRepo  AccountRepository
	quotaFetcher *AntigravityQuotaFetcher
	quotaRepo    CapacityRepository
}

// NewAntigravityCapacityProvider builds the Antigravity CapacityProvider.
func NewAntigravityCapacityProvider(accountRepo AccountRepository, quotaFetcher *AntigravityQuotaFetcher, quotaRepo CapacityRepository) *AntigravityCapacityProvider {
	return &AntigravityCapacityProvider{accountRepo: accountRepo, quotaFetcher: quotaFetcher, quotaRepo: quotaRepo}
}

// Platform implements CapacityProvider.
func (p *AntigravityCapacityProvider) Platform() string { return PlatformAntigravity }

// ExtractWindows implements CapacityProvider by reading the
// antigravity_sched_* extra snapshot. The window's Duration is derived from
// the gap between when the snapshot was captured
// (antigravity_sched_usage_updated_at) and its reset_at — Antigravity's
// upstream response reports an absolute reset timestamp per model but never
// a window length, so unlike Kiro's fixed-30d guess, here an actual duration
// can be computed directly from two real observed timestamps. Falls back to
// a 5h default (the shortest, most common Claude-family rolling window) only
// when antigravity_sched_usage_updated_at itself is missing (e.g. a snapshot
// written by an older code path).
func (p *AntigravityCapacityProvider) ExtractWindows(acc *Account, now time.Time) []QuotaWindow {
	if acc == nil || len(acc.Extra) == 0 {
		return nil
	}
	resetRaw, hasReset := acc.Extra["antigravity_sched_reset_at"]
	utilRaw, hasUtil := acc.Extra["antigravity_sched_utilization"]
	if !hasReset || !hasUtil {
		return nil
	}
	resetAt, err := parseTime(fmt.Sprint(resetRaw))
	if err != nil {
		return nil
	}
	usedRatio := parseExtraFloat64(utilRaw) / 100.0

	observedAt := now
	if raw, ok := acc.Extra["antigravity_sched_usage_updated_at"]; ok {
		if t, err := parseTime(fmt.Sprint(raw)); err == nil {
			observedAt = t
		}
	}
	duration := resetAt.Sub(observedAt)
	if duration <= 0 {
		duration = antigravityCapacityDefaultWindow
	}

	resetAt, usedRatio = recoverExpiredCapacityWindow(resetAt, usedRatio, duration, now)
	return []QuotaWindow{{Kind: antigravityCapacityWindowKind, UsedRatio: usedRatio, ResetAt: resetAt, Duration: duration}}
}

// Probe implements CapacityProvider by actively refreshing the account's
// Antigravity quota via AntigravityQuotaFetcher.FetchQuota — the same client
// the dashboard usage poll (getAntigravityUsage) uses — and reuses that same
// code's buildAntigravitySchedulerSnapshotExtraUpdates helper to convert the
// response into the antigravity_sched_* extra fields, so both code paths
// keep writing an identical shape.
func (p *AntigravityCapacityProvider) Probe(ctx context.Context, acc *Account) error {
	if p == nil || acc == nil {
		return fmt.Errorf("antigravity capacity provider: account is nil")
	}
	if p.quotaFetcher == nil {
		return fmt.Errorf("antigravity capacity provider: quota fetcher not configured")
	}
	if !p.quotaFetcher.CanFetch(acc) {
		return fmt.Errorf("antigravity capacity provider: account cannot fetch quota")
	}

	now := time.Now().UTC()
	prevWindows := p.ExtractWindows(acc, now)

	proxyURL := p.quotaFetcher.GetProxyURL(ctx, acc)
	result, err := p.quotaFetcher.FetchQuota(ctx, acc, proxyURL)
	if err != nil {
		return err
	}
	if result == nil || result.UsageInfo == nil {
		return fmt.Errorf("antigravity capacity provider: no usage info in quota response")
	}

	updates := buildAntigravitySchedulerSnapshotExtraUpdates(result.UsageInfo)
	if len(updates) == 0 {
		return fmt.Errorf("antigravity capacity provider: no quota windows in response")
	}
	if p.accountRepo != nil {
		if err := p.accountRepo.UpdateExtra(ctx, acc.ID, updates); err != nil {
			return err
		}
	}
	mergeAccountExtra(acc, updates)

	newWindows := p.ExtractWindows(acc, now)
	if err := upsertCapacityWindows(ctx, p.quotaRepo, PlatformAntigravity, acc.ID, newWindows, now); err != nil {
		return err
	}
	archiveClosedCapacityPeriods(ctx, p.quotaRepo, PlatformAntigravity, acc.ID, prevWindows, newWindows)
	return nil
}
