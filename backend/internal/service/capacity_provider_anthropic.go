package service

import (
	"context"
	"fmt"
	"time"
)

const (
	anthropicCapacityWindow5h   = "5h"
	anthropicCapacityWindow7d   = "7d"
	anthropicCapacityDuration5h = 5 * time.Hour
	anthropicCapacityDuration7d = 7 * 24 * time.Hour
)

// AnthropicCapacityProvider is the CapacityProvider for PlatformAnthropic.
//
// Deviation from the original task brief: the brief assumed dedicated
// extra flags named anthropic_5h_window_exhausted / anthropic_7d_window_exhausted
// / anthropic_sched_reset_at existed. They do not — grepping the codebase
// shows the only persisted Anthropic quota state is:
//   - the Account.SessionWindowStart/SessionWindowEnd/SessionWindowStatus
//     columns (written by UpdateSessionWindow in ratelimit_service.go from the
//     anthropic-ratelimit-unified-5h-reset response header) plus the
//     session_window_utilization extra fraction (written by
//     AccountUsageService.syncActiveToPassive), for the 5h window; and
//   - the passive_usage_7d_utilization / passive_usage_7d_reset extra keys
//     (also written by syncActiveToPassive, read back by
//     AccountUsageService.buildPassiveUsageWindow) for the 7d window.
//
// This provider reads exactly those real fields instead.
type AnthropicCapacityProvider struct {
	usageService *AccountUsageService
	quotaRepo    CapacityRepository
}

// NewAnthropicCapacityProvider builds the Anthropic CapacityProvider.
func NewAnthropicCapacityProvider(usageService *AccountUsageService, quotaRepo CapacityRepository) *AnthropicCapacityProvider {
	return &AnthropicCapacityProvider{usageService: usageService, quotaRepo: quotaRepo}
}

// Platform implements CapacityProvider.
func (p *AnthropicCapacityProvider) Platform() string { return PlatformAnthropic }

// ExtractWindows implements CapacityProvider by reading the 5h session window
// (SessionWindowStart/End/Status columns + session_window_utilization extra)
// and the 7d passive-usage window (passive_usage_7d_utilization/_reset
// extra). Either may be absent independently (e.g. a brand-new account that
// has never been probed has neither); nil is returned only when both are
// unavailable.
func (p *AnthropicCapacityProvider) ExtractWindows(acc *Account, now time.Time) []QuotaWindow {
	if acc == nil {
		return nil
	}
	var windows []QuotaWindow
	if w := extractAnthropic5hWindow(acc, now); w != nil {
		windows = append(windows, *w)
	}
	if w := extractAnthropic7dWindow(acc, now); w != nil {
		windows = append(windows, *w)
	}
	if len(windows) == 0 {
		return nil
	}
	return windows
}

// extractAnthropic5hWindow mirrors estimateSetupTokenUsage's fallback ladder
// (account_usage_service.go): prefer the precise session_window_utilization
// fraction when present, else approximate from SessionWindowStatus
// ("rejected" => fully used, "allowed_warning" => 80% used), else, if we at
// least know the window's start/end, estimate purely from elapsed time.
func extractAnthropic5hWindow(acc *Account, now time.Time) *QuotaWindow {
	if acc.SessionWindowEnd == nil {
		return nil
	}
	resetAt := *acc.SessionWindowEnd
	duration := anthropicCapacityDuration5h
	if acc.SessionWindowStart != nil && acc.SessionWindowStart.Before(resetAt) {
		duration = resetAt.Sub(*acc.SessionWindowStart)
	}

	var usedRatio float64
	if raw, ok := acc.Extra["session_window_utilization"]; ok {
		usedRatio = parseExtraFloat64(raw)
	} else {
		switch acc.SessionWindowStatus {
		case "rejected":
			usedRatio = 1.0
		case "allowed_warning":
			usedRatio = 0.8
		default:
			if resetAt.After(now) && duration > 0 {
				elapsed := duration - resetAt.Sub(now)
				if elapsed > 0 {
					usedRatio = float64(elapsed) / float64(duration)
				}
			}
		}
	}

	resetAt, usedRatio = recoverExpiredCapacityWindow(resetAt, usedRatio, duration, now)
	return &QuotaWindow{Kind: anthropicCapacityWindow5h, UsedRatio: usedRatio, ResetAt: resetAt, Duration: duration}
}

// extractAnthropic7dWindow reads the passive_usage_7d_* extra pair written by
// syncActiveToPassive. passive_usage_7d_reset is stored as unix seconds
// (matching buildPassiveUsageWindow's own reading convention), not RFC3339.
func extractAnthropic7dWindow(acc *Account, now time.Time) *QuotaWindow {
	resetRaw, hasReset := acc.Extra["passive_usage_7d_reset"]
	utilRaw, hasUtil := acc.Extra["passive_usage_7d_utilization"]
	if !hasReset || !hasUtil {
		return nil
	}
	resetSeconds := parseExtraFloat64(resetRaw)
	if resetSeconds <= 0 {
		return nil
	}
	resetAt := time.Unix(int64(resetSeconds), 0).UTC()
	usedRatio := parseExtraFloat64(utilRaw)
	duration := anthropicCapacityDuration7d

	resetAt, usedRatio = recoverExpiredCapacityWindow(resetAt, usedRatio, duration, now)
	return &QuotaWindow{Kind: anthropicCapacityWindow7d, UsedRatio: usedRatio, ResetAt: resetAt, Duration: duration}
}

// Probe implements CapacityProvider by forcing a fresh upstream usage fetch
// through the existing AccountUsageService.GetUsage(force=true) path — the
// same call the admin "refresh usage" action uses for OAuth/setup-token
// Anthropic accounts. GetUsage already persists SessionWindowEnd and the
// session_window_utilization/passive_usage_7d_* extra fields internally
// (via syncActiveToPassive), so Probe only needs to mirror the resulting
// account state before recomputing windows.
//
// Note: only Account.Type == OAuth actually reaches the live usage API
// (Account.CanGetUsage()); ServiceAccount/APIKey-type Anthropic accounts will
// return an "unsupported" error here, same as calling GetUsage on them
// directly. SetupToken-type accounts are estimated locally with no network
// call (estimateSetupTokenUsage) and will still produce a usable 5h window
// from whatever SessionWindowEnd/Status is already on record, even though
// Probe itself made no live request.
func (p *AnthropicCapacityProvider) Probe(ctx context.Context, acc *Account) error {
	if p == nil || acc == nil {
		return fmt.Errorf("anthropic capacity provider: account is nil")
	}
	if p.usageService == nil {
		return fmt.Errorf("anthropic capacity provider: usage service not configured")
	}

	now := time.Now().UTC()
	prevWindows := p.ExtractWindows(acc, now)

	usage, err := p.usageService.GetUsage(ctx, acc.ID, true)
	if err != nil {
		return err
	}
	if usage == nil || (usage.FiveHour == nil && usage.SevenDay == nil) {
		return fmt.Errorf("anthropic capacity provider: no usage windows in quota response")
	}

	// GetUsage already persisted the underlying account row (SessionWindowEnd
	// via UpdateSessionWindowEnd, extra via UpdateExtra) as a side effect of
	// syncActiveToPassive; refresh the in-memory copy so ExtractWindows below
	// sees the same state without a second DB write.
	refreshed, err := p.usageService.accountRepo.GetByID(ctx, acc.ID)
	if err == nil && refreshed != nil {
		acc.SessionWindowStart = refreshed.SessionWindowStart
		acc.SessionWindowEnd = refreshed.SessionWindowEnd
		acc.SessionWindowStatus = refreshed.SessionWindowStatus
		mergeAccountExtra(acc, refreshed.Extra)
	} else {
		if usage.FiveHour != nil {
			acc.SessionWindowEnd = usage.FiveHour.ResetsAt
			mergeAccountExtra(acc, map[string]any{"session_window_utilization": usage.FiveHour.Utilization / 100})
		}
		if usage.SevenDay != nil {
			updates := map[string]any{"passive_usage_7d_utilization": usage.SevenDay.Utilization / 100}
			if usage.SevenDay.ResetsAt != nil {
				updates["passive_usage_7d_reset"] = float64(usage.SevenDay.ResetsAt.Unix())
			}
			mergeAccountExtra(acc, updates)
		}
	}

	newWindows := p.ExtractWindows(acc, now)
	if err := upsertCapacityWindows(ctx, p.quotaRepo, PlatformAnthropic, acc.ID, newWindows, now); err != nil {
		return err
	}
	archiveClosedCapacityPeriods(ctx, p.quotaRepo, PlatformAnthropic, acc.ID, prevWindows, newWindows)
	return nil
}
