package service

import (
	"context"
	"time"
)

// This file holds helpers shared across all per-platform CapacityProvider
// implementations (capacity_provider_<platform>.go). They were originally
// written inline in capacity_provider_openai.go (the reference
// implementation); once a second platform needed the same "expired window ->
// recovered" extrapolation and upsert/archive plumbing, they were promoted
// here so every provider stays consistent instead of re-implementing them.

// recoverExpiredCapacityWindow treats an already-elapsed reset_at as
// "recovered": used_ratio resets to 0 and reset_at is extrapolated forward by
// whole `duration` periods until it is back in the future. This mirrors
// extractOpenAICapacityWindow's original inline logic exactly, so a stale
// snapshot (e.g. an account that hasn't been probed since its window closed)
// still yields a forward-looking window instead of a permanently-expired one.
// resetAt/duration are returned unchanged when resetAt is already in the
// future or duration <= 0 (nothing to extrapolate).
func recoverExpiredCapacityWindow(resetAt time.Time, usedRatio float64, duration time.Duration, now time.Time) (time.Time, float64) {
	if duration <= 0 || resetAt.After(now) {
		return resetAt, usedRatio
	}
	elapsed := now.Sub(resetAt)
	periods := int64(elapsed/duration) + 1
	return resetAt.Add(time.Duration(periods) * duration), 0
}

// upsertCapacityWindows persists the latest known state of each window to
// account_quota_snapshots. A nil quotaRepo is a no-op (used by tests that
// only exercise ExtractWindows).
func upsertCapacityWindows(ctx context.Context, quotaRepo CapacityRepository, platform string, accountID int64, windows []QuotaWindow, now time.Time) error {
	if quotaRepo == nil {
		return nil
	}
	for _, w := range windows {
		snapshot := AccountQuotaSnapshot{
			Platform:      platform,
			AccountID:     accountID,
			WindowKind:    w.Kind,
			UsedRatio:     w.UsedRatio,
			ResetAt:       w.ResetAt,
			WindowMinutes: int(w.Duration.Minutes()),
			ObservedAt:    now,
		}
		if err := quotaRepo.UpsertQuotaSnapshot(ctx, snapshot); err != nil {
			return err
		}
	}
	return nil
}

// archiveClosedCapacityPeriods detects, per window kind, whether reset_at
// moved forward past the previous snapshot's reset_at (i.e. the window
// actually closed since the last probe rather than just being re-observed)
// and, if so, archives the closed period's spend/used_ratio/inferred
// capacity. A nil quotaRepo is a no-op. Failures archiving an individual
// period are swallowed (best-effort, matching the original OpenAI provider's
// behavior) since a failed archive must not fail the whole Probe.
func archiveClosedCapacityPeriods(ctx context.Context, quotaRepo CapacityRepository, platform string, accountID int64, prevWindows, newWindows []QuotaWindow) {
	if quotaRepo == nil {
		return
	}
	for _, prev := range prevWindows {
		if prev.ResetAt.IsZero() || prev.Duration <= 0 {
			continue
		}
		var cur *QuotaWindow
		for i := range newWindows {
			if newWindows[i].Kind == prev.Kind {
				cur = &newWindows[i]
				break
			}
		}
		if cur == nil || !cur.ResetAt.After(prev.ResetAt) {
			continue
		}

		periodStart := prev.ResetAt.Add(-prev.Duration)
		periodEnd := prev.ResetAt
		spend, err := quotaRepo.GetAccountSpendBetween(ctx, accountID, periodStart, periodEnd)
		if err != nil {
			continue
		}
		var capacity *float64
		if prev.UsedRatio >= capacityMinimumInferenceRatio && spend > 0 {
			c := spend / prev.UsedRatio
			capacity = &c
		}
		finalRatio := prev.UsedRatio
		period := AccountQuotaPeriod{
			Platform:            platform,
			AccountID:           accountID,
			WindowKind:          prev.Kind,
			PeriodStart:         periodStart,
			PeriodEnd:           periodEnd,
			FinalUsedRatio:      &finalRatio,
			SpendUSD:            spend,
			InferredCapacityUSD: capacity,
			SampleCount:         1,
		}
		_ = quotaRepo.InsertQuotaPeriod(ctx, period)
	}
}
