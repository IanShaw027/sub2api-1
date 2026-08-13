//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mustUTC(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return tm.UTC()
}

// ---------------------------------------------------------------------------
// ForecastFutureSpend
// ---------------------------------------------------------------------------

func TestForecastFutureSpendSamePeriodWeighting(t *testing.T) {
	current := mustUTC(t, "2026-07-23T10:00:00Z")
	bucket := current.Add(1 * time.Hour) // 11:00 today

	hourly := map[time.Time]float64{
		bucket.Add(-24 * time.Hour): 100, // yesterday 11:00
		bucket.Add(-48 * time.Hour): 50,  // day-before 11:00
	}

	result := ForecastFutureSpend(SpendForecastInput{
		CurrentBucket: current,
		Hourly:        hourly,
		FutureBuckets: []time.Time{bucket},
	})

	// No elapsed-today history -> ratio falls back to 1 (elapsed < 2h, since
	// CurrentBucket itself has no prior-today hours in the map).
	require.InDelta(t, 1.0, result.Ratio, 1e-9)
	// base = 0.6*100 + 0.4*50 = 80
	require.InDelta(t, 80.0, result.ForecastByBucket[bucket], 1e-9)
}

func TestForecastFutureSpendRatioClampHigh(t *testing.T) {
	current := mustUTC(t, "2026-07-23T10:00:00Z") // 10h elapsed today
	todayStart := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	bucket := current.Add(1 * time.Hour)

	hourly := map[time.Time]float64{
		bucket.Add(-24 * time.Hour): 10,
		bucket.Add(-48 * time.Hour): 10,
	}
	// Today's cumulative (00:00-10:00) spend is huge relative to the comparison
	// days, so the ratio should clamp at capacityRatioMax (4.0).
	for i := 0; i < 10; i++ {
		h := todayStart.Add(time.Duration(i) * time.Hour)
		hourly[h] = 1000
		hourly[h.Add(-24*time.Hour)] = 10
		hourly[h.Add(-48*time.Hour)] = 10
	}

	result := ForecastFutureSpend(SpendForecastInput{
		CurrentBucket: current,
		Hourly:        hourly,
		FutureBuckets: []time.Time{bucket},
	})
	require.InDelta(t, capacityRatioMax, result.Ratio, 1e-9)
}

func TestForecastFutureSpendRatioClampLow(t *testing.T) {
	current := mustUTC(t, "2026-07-23T10:00:00Z")
	todayStart := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	bucket := current.Add(1 * time.Hour)

	hourly := map[time.Time]float64{
		bucket.Add(-24 * time.Hour): 10,
		bucket.Add(-48 * time.Hour): 10,
	}
	for i := 0; i < 10; i++ {
		h := todayStart.Add(time.Duration(i) * time.Hour)
		hourly[h] = 0.1
		hourly[h.Add(-24*time.Hour)] = 100
		hourly[h.Add(-48*time.Hour)] = 100
	}

	result := ForecastFutureSpend(SpendForecastInput{
		CurrentBucket: current,
		Hourly:        hourly,
		FutureBuckets: []time.Time{bucket},
	})
	require.InDelta(t, capacityRatioMin, result.Ratio, 1e-9)
}

func TestForecastFutureSpendUnderTwoHoursElapsedRatioIsOne(t *testing.T) {
	// Only 1h elapsed today (00:00-01:00): ratio must be exactly 1 regardless
	// of how skewed the comparison days are.
	current := mustUTC(t, "2026-07-23T01:00:00Z")
	todayStart := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	bucket := current.Add(1 * time.Hour)

	hourly := map[time.Time]float64{
		todayStart:                      1000,
		todayStart.Add(-24 * time.Hour): 1,
		todayStart.Add(-48 * time.Hour): 1,
		bucket.Add(-24 * time.Hour):     20,
		bucket.Add(-48 * time.Hour):     30,
	}

	result := ForecastFutureSpend(SpendForecastInput{
		CurrentBucket: current,
		Hourly:        hourly,
		FutureBuckets: []time.Time{bucket},
	})
	require.InDelta(t, 1.0, result.Ratio, 1e-9)
	require.InDelta(t, 0.6*20+0.4*30, result.ForecastByBucket[bucket], 1e-9)
}

func TestForecastFutureSpendFarFutureBucketWalksBackToLatestSameHour(t *testing.T) {
	// 7d-range scenario: a bucket 60h ahead. Its -24h/-48h day offsets are still
	// in the future (no data possible); the walk-back must find the most recent
	// same-clock-hour values at -72h and -96h instead of collapsing to the flat
	// recent-average fallback.
	current := mustUTC(t, "2026-07-23T10:00:00Z")
	bucket := current.Add(60 * time.Hour) // +60h => -72h = current-12h, -96h = current-36h

	hourly := map[time.Time]float64{
		bucket.Add(-72 * time.Hour): 40, // most recent same-clock-hour with data
		bucket.Add(-96 * time.Hour): 10, // second most recent
		// recent 3h present so a fallback would be clearly distinguishable
		current.Add(-1 * time.Hour): 999,
		current.Add(-2 * time.Hour): 999,
		current.Add(-3 * time.Hour): 999,
	}

	result := ForecastFutureSpend(SpendForecastInput{
		CurrentBucket: current,
		Hourly:        hourly,
		FutureBuckets: []time.Time{bucket},
	})
	// base = 0.6*40 + 0.4*10 = 28, ratio = 1 (no elapsed-today history)
	require.InDelta(t, 28.0, result.ForecastByBucket[bucket], 1e-9)
}

func TestForecastFutureSpendNoHistoryFallsBackToRecentAverage(t *testing.T) {
	current := mustUTC(t, "2026-07-23T10:00:00Z")
	bucket := current.Add(1 * time.Hour)

	// No same-hour history at all for `bucket`, but the last 3 hours have data.
	hourly := map[time.Time]float64{
		current.Add(-1 * time.Hour): 10,
		current.Add(-2 * time.Hour): 20,
		current.Add(-3 * time.Hour): 30,
	}

	result := ForecastFutureSpend(SpendForecastInput{
		CurrentBucket: current,
		Hourly:        hourly,
		FutureBuckets: []time.Time{bucket},
	})
	// recent 3h average = (10+20+30)/3 = 20, ratio not reapplied on top of the
	// no-history fallback.
	require.InDelta(t, 20.0, result.ForecastByBucket[bucket], 1e-9)
}

// ---------------------------------------------------------------------------
// SimulateFutureCapacity
// ---------------------------------------------------------------------------

func TestSimulateFutureCapacityMultiWindowMin(t *testing.T) {
	current := mustUTC(t, "2026-07-23T00:00:00Z")
	accounts := []CapacityAccountState{
		{
			ID: 1, Name: "acct-1",
			Windows: []CapacityWindowState{
				{Kind: "5h", UsedRatio: 0.9, ResetAt: current.Add(10 * time.Hour), Duration: 5 * time.Hour, CapacityUSD: 100},
				{Kind: "7d", UsedRatio: 0.5, ResetAt: current.Add(72 * time.Hour), Duration: 7 * 24 * time.Hour, CapacityUSD: 1000},
			},
		},
	}
	// headroom = min(100*0.1, 1000*0.5) = min(10, 500) = 10
	h, ok := accountHeadroom(accounts[0].Windows)
	require.True(t, ok)
	require.InDelta(t, 10.0, h, 1e-9)
}

func TestSimulateFutureCapacity7dRecoveryStillConstrainedBy5h(t *testing.T) {
	current := mustUTC(t, "2026-07-23T00:00:00Z")
	buckets := []time.Time{
		current.Add(1 * time.Hour),
		current.Add(2 * time.Hour),
		current.Add(3 * time.Hour),
	}
	// 7d window resets at bucket[1] (used_ratio 0.9 -> 0); 5h window stays
	// exhausted (used_ratio 0.95, far-future reset) the whole time.
	accounts := []CapacityAccountState{
		{
			ID: 1, Name: "acct-1",
			Windows: []CapacityWindowState{
				{Kind: "5h", UsedRatio: 0.95, ResetAt: current.Add(100 * time.Hour), Duration: 5 * time.Hour, CapacityUSD: 100},
				{Kind: "7d", UsedRatio: 0.9, ResetAt: buckets[1], Duration: 7 * 24 * time.Hour, CapacityUSD: 1000},
			},
		},
	}
	// Initial headroom = min(100*0.05, 1000*0.1) = min(5, 100) = 5.
	initial, ok := accountHeadroom(accounts[0].Windows)
	require.True(t, ok)
	require.InDelta(t, 5.0, initial, 1e-9)

	forecastSpend := map[time.Time]float64{current: 0, buckets[0]: 0, buckets[1]: 0, buckets[2]: 0}
	result := SimulateFutureCapacity(CapacitySimulationInput{
		CurrentBucket: current, Buckets: buckets, RangeEnd: current.Add(200 * time.Hour),
		CurrentAvailable: initial, Accounts: accounts, ForecastSpend: forecastSpend,
	})

	// After the 7d window resets, headroom is still capped by the still-95%-used
	// 5h window: min(100*0.05, 1000*1.0) = 5. So no capacity should be
	// unlocked (delta == 0), meaning available stays at 5, not jumping to 100.
	require.InDelta(t, 5.0, result.AvailableByBucket[buckets[2]], 1e-9)
	require.Empty(t, result.RecoverEvents, "7d recovery should not unlock capacity while the 5h window is still exhausted")
}

func TestSimulateFutureCapacityOnly30dConstraintUnlocksFully(t *testing.T) {
	current := mustUTC(t, "2026-07-23T00:00:00Z")
	buckets := []time.Time{current.Add(1 * time.Hour), current.Add(2 * time.Hour)}
	resetAt := buckets[0]

	accounts := []CapacityAccountState{
		{
			ID: 1, Name: "acct-1",
			Windows: []CapacityWindowState{
				// Only the 30d window is constraining (CapacityUSD > 0); no other window.
				{Kind: "30d", UsedRatio: 0.8, ResetAt: resetAt, Duration: 30 * 24 * time.Hour, CapacityUSD: 500},
			},
		},
	}
	initial, ok := accountHeadroom(accounts[0].Windows)
	require.True(t, ok)
	require.InDelta(t, 100.0, initial, 1e-9) // 500*0.2

	forecastSpend := map[time.Time]float64{current: 0, buckets[0]: 0, buckets[1]: 0}
	result := SimulateFutureCapacity(CapacitySimulationInput{
		CurrentBucket: current, Buckets: buckets, RangeEnd: current.Add(48 * time.Hour),
		CurrentAvailable: initial, Accounts: accounts, ForecastSpend: forecastSpend,
	})

	// Reset makes used_ratio=0 -> full capacity (500) becomes available.
	require.InDelta(t, 500.0, result.AvailableByBucket[buckets[1]], 1e-9)
	require.Len(t, result.RecoverEvents, 1)
	require.InDelta(t, 400.0, result.RecoverEvents[0].AmountUSD, 1e-9) // 500 - 100
}

func TestSimulateFutureCapacityShortWindowPeriodicExtrapolationCapped(t *testing.T) {
	current := mustUTC(t, "2026-07-23T00:00:00Z")
	// A 24h range with a 5h window resetting every 5h: expect resets at +5h,
	// +10h, +15h, +20h (all within RangeEnd), each capped at CapacityUSD.
	var buckets []time.Time
	for i := 1; i <= 24; i++ {
		buckets = append(buckets, current.Add(time.Duration(i)*time.Hour))
	}
	accounts := []CapacityAccountState{
		{
			ID: 1, Name: "acct-1",
			Windows: []CapacityWindowState{
				{Kind: "5h", UsedRatio: 1.0, ResetAt: current.Add(5 * time.Hour), Duration: 5 * time.Hour, CapacityUSD: 100},
			},
		},
	}
	forecastSpend := make(map[time.Time]float64)
	forecastSpend[current] = 0
	for _, b := range buckets {
		forecastSpend[b] = 0
	}

	result := SimulateFutureCapacity(CapacitySimulationInput{
		CurrentBucket: current, Buckets: buckets, RangeEnd: current.Add(24 * time.Hour),
		CurrentAvailable: 0, Accounts: accounts, ForecastSpend: forecastSpend,
	})

	// Only the first reset actually changes headroom: nothing in this pure
	// model re-exhausts a window's used_ratio between synthetic periodic
	// resets, so the later extrapolated occurrences at +10h/+15h/+20h are
	// legitimate no-ops (delta <= 0, filtered out) rather than fabricated
	// repeated top-ups. The net effect is exactly the desired cap: available
	// rises to CapacityUSD (100) on the first reset and never exceeds it.
	require.InDelta(t, 100.0, result.AvailableByBucket[current.Add(5*time.Hour)], 1e-9)
	require.InDelta(t, 100.0, result.AvailableByBucket[current.Add(10*time.Hour)], 1e-9)
	require.InDelta(t, 100.0, result.AvailableByBucket[current.Add(20*time.Hour)], 1e-9)
	require.Len(t, result.RecoverEvents, 1)
	require.InDelta(t, 100.0, result.RecoverEvents[0].AmountUSD, 1e-9)
}

func TestSimulateFutureCapacityRollingBalance(t *testing.T) {
	current := mustUTC(t, "2026-07-23T00:00:00Z")
	buckets := []time.Time{current.Add(1 * time.Hour), current.Add(2 * time.Hour), current.Add(3 * time.Hour)}
	// No windows/events at all -> pure drain-by-forecast-spend rolling balance.
	forecastSpend := map[time.Time]float64{
		current: 10, buckets[0]: 20, buckets[1]: 5, buckets[2]: 0,
	}
	result := SimulateFutureCapacity(CapacitySimulationInput{
		CurrentBucket: current, Buckets: buckets, RangeEnd: current.Add(10 * time.Hour),
		CurrentAvailable: 100, Accounts: nil, ForecastSpend: forecastSpend,
	})
	require.InDelta(t, 90.0, result.AvailableByBucket[buckets[0]], 1e-9) // 100-10
	require.InDelta(t, 70.0, result.AvailableByBucket[buckets[1]], 1e-9) // 90-20
	require.InDelta(t, 65.0, result.AvailableByBucket[buckets[2]], 1e-9) // 70-5
}

func TestSimulateFutureCapacityBalanceNeverGoesNegative(t *testing.T) {
	current := mustUTC(t, "2026-07-23T00:00:00Z")
	buckets := []time.Time{current.Add(1 * time.Hour)}
	forecastSpend := map[time.Time]float64{current: 1000, buckets[0]: 0}
	result := SimulateFutureCapacity(CapacitySimulationInput{
		CurrentBucket: current, Buckets: buckets, RangeEnd: current.Add(10 * time.Hour),
		CurrentAvailable: 10, Accounts: nil, ForecastSpend: forecastSpend,
	})
	require.InDelta(t, 0.0, result.AvailableByBucket[buckets[0]], 1e-9)
}

// ---------------------------------------------------------------------------
// ComputeShortfallEvents
// ---------------------------------------------------------------------------

func TestComputeShortfallEventsAndRecommendation(t *testing.T) {
	b1 := mustUTC(t, "2026-07-23T01:00:00Z")
	b2 := mustUTC(t, "2026-07-23T02:00:00Z")
	points := []CapacityShortfallPoint{
		{BucketStart: b1, ForecastAvailable: 50, ForecastSpend: 80},  // gap 30
		{BucketStart: b2, ForecastAvailable: 10, ForecastSpend: 210}, // gap 200 (max)
	}
	events, firstAt, recs := ComputeShortfallEvents(points, 100)
	require.Len(t, events, 2)
	require.NotNil(t, firstAt)
	require.Equal(t, b1, *firstAt)
	require.Len(t, recs, 1)
	require.Equal(t, b2, recs[0].At)
	require.InDelta(t, 200.0, recs[0].GapUSD, 1e-9)
	require.Equal(t, 2, recs[0].SuggestAccounts) // ceil(200/100)
}

func TestComputeShortfallEventsNoGapNoRecommendation(t *testing.T) {
	points := []CapacityShortfallPoint{{BucketStart: mustUTC(t, "2026-07-23T01:00:00Z"), ForecastAvailable: 100, ForecastSpend: 10}}
	events, firstAt, recs := ComputeShortfallEvents(points, 50)
	require.Empty(t, events)
	require.Nil(t, firstAt)
	require.Nil(t, recs)
}

// ---------------------------------------------------------------------------
// Capacity-inference fallback chain (via buildAccountStates' logic, exercised
// directly through the pure accountHeadroom/median helpers since the fallback
// chain itself lives in capacity_forecast_service.go and needs a repository —
// see capacity_forecast_service_test.go for the full-chain test).
// ---------------------------------------------------------------------------

func TestMedianPositiveValues(t *testing.T) {
	require.InDelta(t, 0.0, medianPositiveValues(nil), 1e-9)
	require.InDelta(t, 0.0, medianPositiveValues([]float64{-1, 0}), 1e-9)
	require.InDelta(t, 20.0, medianPositiveValues([]float64{10, 20, 30}), 1e-9)
	// -5 is filtered out before the median is taken, so this reduces to the
	// same 3-element case as above (median of [10, 20, 30] = 20).
	require.InDelta(t, 20.0, medianPositiveValues([]float64{10, 20, 30, -5}), 1e-9)
}
