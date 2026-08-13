package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const (
	grokCapacityKindMinute = "minute"
	grokCapacityKindHour   = "hour"
	grokCapacityKindDay    = "day"
	grokCapacityKindWeek   = "week"
)

// GrokCapacityProvider is the CapacityProvider for PlatformGrok, backed by
// the xai.QuotaSnapshot already stored in Account.Extra[grok_usage_snapshot]
// (written by GrokQuotaService's active probe and by passive header
// observation on live gateway requests).
//
// Deviation from the task brief: xAI's rate-limit headers (x-ratelimit-*)
// only ever expose limit/remaining/reset_at per dimension (requests, tokens)
// — there is no header indicating the window's length, unlike OpenAI's
// codex_*_window_minutes. So Duration here is *inferred* as the gap between
// when the snapshot was captured and its reset_at, and Kind is a coarse
// label bucketed off that inferred duration rather than a fixed, known
// window name.
type GrokCapacityProvider struct {
	quotaService *GrokQuotaService
	quotaRepo    CapacityRepository
}

// NewGrokCapacityProvider builds the Grok CapacityProvider.
func NewGrokCapacityProvider(quotaService *GrokQuotaService, quotaRepo CapacityRepository) *GrokCapacityProvider {
	return &GrokCapacityProvider{quotaService: quotaService, quotaRepo: quotaRepo}
}

// Platform implements CapacityProvider.
func (p *GrokCapacityProvider) Platform() string { return PlatformGrok }

// ExtractWindows implements CapacityProvider by decoding the
// grok_usage_snapshot extra value (via the existing
// grokQuotaSnapshotFromExtra helper) and converting its Requests/Tokens
// dimensions into QuotaWindows. When both dimensions fall into the same Kind
// bucket, the tighter (higher used_ratio) of the two wins, since that is the
// one that will actually throttle the account first.
func (p *GrokCapacityProvider) ExtractWindows(acc *Account, now time.Time) []QuotaWindow {
	if acc == nil || len(acc.Extra) == 0 {
		return nil
	}
	snapshot, err := grokQuotaSnapshotFromExtra(acc.Extra)
	if err != nil || snapshot == nil {
		return nil
	}

	observedAt := now
	if t, err := time.Parse(time.RFC3339, snapshot.UpdatedAt); err == nil {
		observedAt = t
	}

	byKind := make(map[string]QuotaWindow, 2)
	for _, w := range []*xai.QuotaWindow{snapshot.Requests, snapshot.Tokens} {
		window := extractGrokWindow(w, observedAt, now)
		if window == nil {
			continue
		}
		if existing, ok := byKind[window.Kind]; !ok || window.UsedRatio > existing.UsedRatio {
			byKind[window.Kind] = *window
		}
	}
	if len(byKind) == 0 {
		return nil
	}
	windows := make([]QuotaWindow, 0, len(byKind))
	for _, w := range byKind {
		windows = append(windows, w)
	}
	return windows
}

func extractGrokWindow(w *xai.QuotaWindow, observedAt, now time.Time) *QuotaWindow {
	if w == nil || w.Limit == nil || *w.Limit <= 0 || w.Remaining == nil || w.ResetUnix == nil {
		return nil
	}
	resetAt := time.Unix(*w.ResetUnix, 0).UTC()
	duration := resetAt.Sub(observedAt)
	if duration <= 0 {
		// The snapshot's own reset was already in the past relative to when
		// it was captured (clock skew or a stale/replayed probe) — fall back
		// to a 1h guess so classification still degrades gracefully instead
		// of the window being dropped entirely.
		duration = time.Hour
	}

	remaining := *w.Remaining
	if remaining < 0 {
		remaining = 0
	}
	usedRatio := 1 - float64(remaining)/float64(*w.Limit)
	if usedRatio < 0 {
		usedRatio = 0
	}
	if usedRatio > 1 {
		usedRatio = 1
	}

	resetAt, usedRatio = recoverExpiredCapacityWindow(resetAt, usedRatio, duration, now)
	return &QuotaWindow{Kind: grokCapacityWindowKind(duration), UsedRatio: usedRatio, ResetAt: resetAt, Duration: duration}
}

// grokCapacityWindowKind buckets an inferred window length into a small,
// stable set of labels since xAI's actual limiter granularity per model/tier
// isn't exposed anywhere in the response.
func grokCapacityWindowKind(duration time.Duration) string {
	switch {
	case duration <= 2*time.Minute:
		return grokCapacityKindMinute
	case duration <= 2*time.Hour:
		return grokCapacityKindHour
	case duration <= 36*time.Hour:
		return grokCapacityKindDay
	default:
		return grokCapacityKindWeek
	}
}

// Probe implements CapacityProvider by actively refreshing the account's Grok
// quota via GrokQuotaService.ProbeUsage — the same inference-probe entry
// point the admin "test account" flow uses. ProbeUsage already persists the
// resulting xai.QuotaSnapshot into Account.Extra[grok_usage_snapshot] via
// AccountRepository.UpdateExtra itself, so Probe only needs to mirror that
// write into the in-memory account before recomputing windows.
func (p *GrokCapacityProvider) Probe(ctx context.Context, acc *Account) error {
	if p == nil || acc == nil {
		return fmt.Errorf("grok capacity provider: account is nil")
	}
	if p.quotaService == nil {
		return fmt.Errorf("grok capacity provider: quota service not configured")
	}

	now := time.Now().UTC()
	prevWindows := p.ExtractWindows(acc, now)

	result, err := p.quotaService.ProbeUsage(ctx, acc.ID)
	if err != nil {
		return err
	}
	if result == nil || result.Snapshot == nil {
		return fmt.Errorf("grok capacity provider: no quota snapshot in probe response")
	}
	mergeAccountExtra(acc, map[string]any{grokQuotaSnapshotExtraKey: result.Snapshot})

	newWindows := p.ExtractWindows(acc, now)
	if len(newWindows) == 0 {
		return fmt.Errorf("grok capacity provider: no usable rate-limit windows observed")
	}
	if err := upsertCapacityWindows(ctx, p.quotaRepo, PlatformGrok, acc.ID, newWindows, now); err != nil {
		return err
	}
	archiveClosedCapacityPeriods(ctx, p.quotaRepo, PlatformGrok, acc.ID, prevWindows, newWindows)
	return nil
}
