package service

import (
	"context"
	"fmt"
	"time"
)

const (
	openAICapacityWindow5h = "5h"
	openAICapacityWindow7d = "7d"
)

var openAICapacityDefaultWindowMinutes = map[string]int{
	openAICapacityWindow5h: 300,
	openAICapacityWindow7d: 10080,
}

// OpenAICapacityProvider is the CapacityProvider for PlatformOpenAI, backed by
// the Codex quota snapshot fields (codex_5h_*/codex_7d_*) already written into
// Account.Extra by the existing OpenAI usage/probe code paths.
type OpenAICapacityProvider struct {
	accountRepo  AccountRepository
	quotaService *OpenAIQuotaService
	quotaRepo    CapacityRepository
}

// NewOpenAICapacityProvider builds the OpenAI CapacityProvider.
func NewOpenAICapacityProvider(accountRepo AccountRepository, quotaService *OpenAIQuotaService, quotaRepo CapacityRepository) *OpenAICapacityProvider {
	return &OpenAICapacityProvider{accountRepo: accountRepo, quotaService: quotaService, quotaRepo: quotaRepo}
}

// Platform implements CapacityProvider.
func (p *OpenAICapacityProvider) Platform() string { return PlatformOpenAI }

// ExtractWindows implements CapacityProvider by parsing the codex_5h_*/codex_7d_*
// keys already present in Account.Extra. Returns nil when the account has no
// Codex quota snapshot at all.
func (p *OpenAICapacityProvider) ExtractWindows(acc *Account, now time.Time) []QuotaWindow {
	if acc == nil || len(acc.Extra) == 0 {
		return nil
	}
	var windows []QuotaWindow
	if w := extractOpenAICapacityWindow(acc.Extra, openAICapacityWindow5h, now); w != nil {
		windows = append(windows, *w)
	}
	if w := extractOpenAICapacityWindow(acc.Extra, openAICapacityWindow7d, now); w != nil {
		windows = append(windows, *w)
	}
	return windows
}

// extractOpenAICapacityWindow reads one codex_<kind>_* window from extra. An
// already-elapsed reset_at is treated as "recovered" (used_ratio reset to 0)
// with the next reset extrapolated by whole periods, so ExtractWindows always
// hands the forecasting engine a future-facing window rather than a stale one.
func extractOpenAICapacityWindow(extra map[string]any, kind string, now time.Time) *QuotaWindow {
	usedRaw, ok := extra["codex_"+kind+"_used_percent"]
	if !ok {
		return nil
	}
	resetRaw, ok := extra["codex_"+kind+"_reset_at"]
	if !ok {
		return nil
	}
	resetAt, err := parseTime(fmt.Sprint(resetRaw))
	if err != nil {
		return nil
	}

	duration := time.Duration(openAICapacityDefaultWindowMinutes[kind]) * time.Minute
	if wm := parseExtraInt(extra["codex_"+kind+"_window_minutes"]); wm > 0 {
		duration = time.Duration(wm) * time.Minute
	}
	if duration <= 0 {
		return nil
	}

	usedRatio := parseExtraFloat64(usedRaw) / 100.0
	resetAt, usedRatio = recoverExpiredCapacityWindow(resetAt, usedRatio, duration, now)

	return &QuotaWindow{Kind: kind, UsedRatio: usedRatio, ResetAt: resetAt, Duration: duration}
}

// Probe implements CapacityProvider by refreshing the account's Codex quota
// from upstream. It deliberately reuses OpenAIQuotaService.QueryUsage plus
// buildCodexSparkWindowExtraUpdates (openai_quota_service.go) — the same
// function shadow/spark accounts already use to translate a QueryUsage result
// into codex_5h_*/codex_7d_* extra fields — instead of re-implementing that
// parsing. After the refresh it upserts the resulting windows into
// account_quota_snapshots and, when a window's reset_at has rolled forward
// past its previous value, archives the closed period into
// account_quota_periods.
func (p *OpenAICapacityProvider) Probe(ctx context.Context, acc *Account) error {
	if p == nil || acc == nil {
		return fmt.Errorf("openai capacity provider: account is nil")
	}
	if p.quotaService == nil {
		return fmt.Errorf("openai capacity provider: quota service not configured")
	}

	now := time.Now().UTC()
	prevWindows := p.ExtractWindows(acc, now)

	usage, err := p.quotaService.QueryUsage(ctx, acc.ID)
	if err != nil {
		return err
	}
	updates := buildCodexSparkWindowExtraUpdates(usage, now)
	if len(updates) == 0 {
		return fmt.Errorf("openai capacity provider: no codex usage windows in quota response")
	}
	if p.accountRepo != nil {
		if err := p.accountRepo.UpdateExtra(ctx, acc.ID, updates); err != nil {
			return err
		}
	}
	mergeAccountExtra(acc, updates)

	newWindows := p.ExtractWindows(acc, now)
	if err := upsertCapacityWindows(ctx, p.quotaRepo, PlatformOpenAI, acc.ID, newWindows, now); err != nil {
		return err
	}
	archiveClosedCapacityPeriods(ctx, p.quotaRepo, PlatformOpenAI, acc.ID, prevWindows, newWindows)
	return nil
}
