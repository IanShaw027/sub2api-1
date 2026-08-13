package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	geminiCapacityWindowKind           = "day"
	geminiCapacityDayUsedRatioExtraKey = "gemini_capacity_day_used_ratio"
	geminiCapacityDayResetAtExtraKey   = "gemini_capacity_day_reset_at"
)

// GeminiCapacityProvider is the CapacityProvider for PlatformGemini.
//
// Deviation from the task brief: Gemini's RPD/RPM limits (GeminiQuotaService,
// gemini_quota.go) are policy-configured numbers enforced against Sub2API's
// own usage_log — not a value the upstream ever reports back — so unlike
// every other platform there is no pre-existing "quota snapshot" extra key
// to read for OAuth/Code-Assist accounts. This provider introduces its own
// capacity-only extra keys (gemini_capacity_day_used_ratio /
// gemini_capacity_day_reset_at), written by Probe from a fresh
// GetModelStatsWithFilters aggregate, and read back by ExtractWindows —
// matching every other provider's "write in Probe, read in ExtractWindows"
// shape even though the underlying computation is a local aggregate instead
// of a network probe. As a secondary source (mainly useful right after
// account creation, before this provider's first Probe has ever run),
// ExtractWindows also falls back to the existing gemini_usage_raw snapshot
// (parseGeminiQuotaSnapshot) used by the passive Gemini usage display, when
// present.
type GeminiCapacityProvider struct {
	accountRepo  AccountRepository
	quotaService *GeminiQuotaService
	usageLogRepo UsageLogRepository
	quotaRepo    CapacityRepository
}

// NewGeminiCapacityProvider builds the Gemini CapacityProvider.
func NewGeminiCapacityProvider(accountRepo AccountRepository, quotaService *GeminiQuotaService, usageLogRepo UsageLogRepository, quotaRepo CapacityRepository) *GeminiCapacityProvider {
	return &GeminiCapacityProvider{accountRepo: accountRepo, quotaService: quotaService, usageLogRepo: usageLogRepo, quotaRepo: quotaRepo}
}

// Platform implements CapacityProvider.
func (p *GeminiCapacityProvider) Platform() string { return PlatformGemini }

// ExtractWindows implements CapacityProvider. See the type doc for the
// two-source design.
func (p *GeminiCapacityProvider) ExtractWindows(acc *Account, now time.Time) []QuotaWindow {
	if acc == nil {
		return nil
	}
	if w := extractGeminiProbedDayWindow(acc, now); w != nil {
		return []QuotaWindow{*w}
	}
	if w := extractGeminiUpstreamSnapshotWindow(acc, now); w != nil {
		return []QuotaWindow{*w}
	}
	return nil
}

func extractGeminiProbedDayWindow(acc *Account, now time.Time) *QuotaWindow {
	resetRaw, hasReset := acc.Extra[geminiCapacityDayResetAtExtraKey]
	utilRaw, hasUtil := acc.Extra[geminiCapacityDayUsedRatioExtraKey]
	if !hasReset || !hasUtil {
		return nil
	}
	resetAt, err := parseTime(fmt.Sprint(resetRaw))
	if err != nil {
		return nil
	}
	usedRatio := parseExtraFloat64(utilRaw)
	duration := 24 * time.Hour

	resetAt, usedRatio = recoverExpiredCapacityWindow(resetAt, usedRatio, duration, now)
	return &QuotaWindow{Kind: geminiCapacityWindowKind, UsedRatio: usedRatio, ResetAt: resetAt, Duration: duration}
}

// extractGeminiUpstreamSnapshotWindow falls back to the raw gemini_usage_raw
// snapshot (Google One / Code Assist OAuth accounts only — AI Studio API-key
// accounts never populate it), picking whichever of pro/flash is currently
// more utilized, mirroring getGeminiUsage's own tightest-scope selection.
func extractGeminiUpstreamSnapshotWindow(acc *Account, now time.Time) *QuotaWindow {
	snapshot := parseGeminiQuotaSnapshot(acc)
	if snapshot == nil {
		return nil
	}
	tightest := snapshot.pro
	if snapshot.flash != nil && (tightest == nil || snapshot.flash.utilization > tightest.utilization) {
		tightest = snapshot.flash
	}
	if tightest == nil {
		return nil
	}

	usedRatio := tightest.utilization / 100
	duration := 24 * time.Hour
	resetAt := geminiDailyResetTime(now)
	if tightest.resetAt != nil {
		resetAt = *tightest.resetAt
	}

	resetAt, usedRatio = recoverExpiredCapacityWindow(resetAt, usedRatio, duration, now)
	return &QuotaWindow{Kind: geminiCapacityWindowKind, UsedRatio: usedRatio, ResetAt: resetAt, Duration: duration}
}

// Probe implements CapacityProvider by recomputing today's RPD usage against
// the account's policy-configured daily limit (GeminiQuotaService), the same
// way the passive dashboard usage endpoint (getGeminiUsage) does. There is no
// upstream network call: Gemini's RPD/RPM quota is enforced against
// Sub2API's own usage_log, so "probing" here means re-aggregating it. The
// tighter of the pro/flash (or shared) ratios is persisted so ExtractWindows
// can read it back without re-querying usage_log on every call.
func (p *GeminiCapacityProvider) Probe(ctx context.Context, acc *Account) error {
	if p == nil || acc == nil {
		return fmt.Errorf("gemini capacity provider: account is nil")
	}
	if p.quotaService == nil || p.usageLogRepo == nil {
		return fmt.Errorf("gemini capacity provider: quota service not configured")
	}

	quota, ok := p.quotaService.QuotaForAccount(ctx, acc)
	if !ok {
		return fmt.Errorf("gemini capacity provider: no quota policy for this account's tier")
	}

	now := time.Now().UTC()
	prevWindows := p.ExtractWindows(acc, now)

	dayStart := geminiDailyWindowStart(now)
	stats, err := p.usageLogRepo.GetModelStatsWithFilters(ctx, dayStart, now, 0, 0, acc.ID, 0, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("gemini capacity provider: get usage stats: %w", err)
	}
	totals := geminiAggregateUsage(stats)
	dailyResetAt := geminiDailyResetTime(now)

	usedRatio, ok := geminiTightestDayUsedRatio(quota, totals)
	if !ok {
		return fmt.Errorf("gemini capacity provider: account tier has no bounded daily quota")
	}

	updates := map[string]any{
		geminiCapacityDayUsedRatioExtraKey: usedRatio,
		geminiCapacityDayResetAtExtraKey:   dailyResetAt.UTC().Format(time.RFC3339),
	}
	if p.accountRepo != nil {
		if err := p.accountRepo.UpdateExtra(ctx, acc.ID, updates); err != nil {
			return err
		}
	}
	mergeAccountExtra(acc, updates)

	newWindows := p.ExtractWindows(acc, now)
	if err := upsertCapacityWindows(ctx, p.quotaRepo, PlatformGemini, acc.ID, newWindows, now); err != nil {
		return err
	}
	archiveClosedCapacityPeriods(ctx, p.quotaRepo, PlatformGemini, acc.ID, prevWindows, newWindows)
	return nil
}

// geminiTightestDayUsedRatio returns the higher (more constrained) of the
// pro/flash (or shared) daily used ratios, mirroring getGeminiUsage's
// SharedRPD-vs-per-model branching. ok is false when the account's tier has
// no bounded RPD at all (limit <= 0, i.e. unset or "-1 = unlimited").
func geminiTightestDayUsedRatio(quota GeminiQuota, totals GeminiUsageTotals) (float64, bool) {
	if quota.SharedRPD > 0 {
		used := totals.ProRequests + totals.FlashRequests
		return float64(used) / float64(quota.SharedRPD), true
	}
	var best float64
	var found bool
	if quota.ProRPD > 0 {
		best = float64(totals.ProRequests) / float64(quota.ProRPD)
		found = true
	}
	if quota.FlashRPD > 0 {
		ratio := float64(totals.FlashRequests) / float64(quota.FlashRPD)
		if !found || ratio > best {
			best = ratio
		}
		found = true
	}
	return best, found
}

// geminiQuotaSnapshotWindow is one model-family window parsed from
// gemini_usage_raw (Code Assist / Google One OAuth snapshots).
type geminiQuotaSnapshotWindow struct {
	utilization float64
	resetAt     *time.Time
}

type geminiQuotaSnapshotUsage struct {
	pro   *geminiQuotaSnapshotWindow
	flash *geminiQuotaSnapshotWindow
}

// parseGeminiQuotaSnapshot reads gemini_usage_raw from credentials or extra.
// personal-main does not persist this snapshot in account_usage_service, so
// this is a best-effort fallback for accounts that still carry the raw blob.
func parseGeminiQuotaSnapshot(account *Account) *geminiQuotaSnapshotUsage {
	if account == nil {
		return nil
	}
	raw, ok := account.Credentials["gemini_usage_raw"]
	if !ok {
		raw = account.Extra["gemini_usage_raw"]
	}

	root, ok := raw.(map[string]any)
	if !ok || len(root) == 0 {
		return nil
	}
	buckets, ok := root["buckets"].([]any)
	if !ok || len(buckets) == 0 {
		return nil
	}

	snapshot := &geminiQuotaSnapshotUsage{}
	for _, item := range buckets {
		bucket, ok := item.(map[string]any)
		if !ok {
			continue
		}

		modelID, _ := bucket["modelId"].(string)
		modelID = strings.ToLower(strings.TrimSpace(modelID))
		if modelID == "" {
			continue
		}

		remainingFraction, ok := parseGeminiSnapshotFloat(bucket["remainingFraction"])
		if !ok {
			continue
		}

		utilization := 100 - (remainingFraction * 100)
		if utilization < 0 {
			utilization = 0
		}
		if utilization > 100 {
			utilization = 100
		}

		window := &geminiQuotaSnapshotWindow{
			utilization: utilization,
			resetAt:     parseGeminiSnapshotTime(bucket["resetTime"]),
		}

		switch {
		case strings.Contains(modelID, "flash"):
			snapshot.flash = pickHigherUtilizationSnapshot(snapshot.flash, window)
		case strings.Contains(modelID, "pro"):
			snapshot.pro = pickHigherUtilizationSnapshot(snapshot.pro, window)
		}
	}

	if snapshot.pro == nil && snapshot.flash == nil {
		return nil
	}
	return snapshot
}

func parseGeminiSnapshotFloat(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func parseGeminiSnapshotTime(raw any) *time.Time {
	switch v := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil
		}
		if ts, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return &ts
		}
	case float64:
		ts := time.Unix(int64(v), 0)
		return &ts
	case int64:
		ts := time.Unix(v, 0)
		return &ts
	}
	return nil
}

func pickHigherUtilizationSnapshot(current, next *geminiQuotaSnapshotWindow) *geminiQuotaSnapshotWindow {
	if next == nil {
		return current
	}
	if current == nil || next.utilization > current.utilization {
		return next
	}
	if next.utilization < current.utilization {
		return current
	}
	if current.resetAt == nil {
		return next
	}
	if next.resetAt == nil {
		return current
	}
	if next.resetAt.After(*current.resetAt) {
		return next
	}
	return current
}
