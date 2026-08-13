package service

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// This file contains the pure (no I/O, no DB) forecasting/simulation math for
// the generic capacity engine. Keeping it side-effect-free is what makes it
// unit-testable without a database — see capacity_forecast_pure_test.go.

const (
	capacityRatioMin           = 0.25
	capacityRatioMax           = 4.0
	capacityMinElapsedForRatio = 2 * time.Hour
	// capacityShortWindowMaxDuration marks windows (e.g. Codex's 5h window) as
	// "short" — within a long display range they recur many times and are
	// extrapolated periodically, unlike a 7d/30d window which resets at most
	// once within the range.
	capacityShortWindowMaxDuration = 6 * time.Hour
	// capacityMaxExtrapolatedResets bounds the extrapolation loop so a
	// misconfigured (near-zero) Duration can never spin forever.
	capacityMaxExtrapolatedResets = 500
	// capacityShapeLookbackDays bounds how many whole days back
	// forecastBaseForBucket walks to find same-clock-hour history. The 7d range
	// needs up to 4 skipped (still-future) days plus 2 data days; 8 leaves margin.
	capacityShapeLookbackDays = 8
)

// ---------------------------------------------------------------------------
// Future consumption forecast
// ---------------------------------------------------------------------------

// SpendForecastInput is the pure input to ForecastFutureSpend.
type SpendForecastInput struct {
	// CurrentBucket is "now" truncated down to the hour (UTC).
	CurrentBucket time.Time
	// Hourly holds every known actual-spend bucket (UTC hour-aligned), at
	// least covering the two prior days plus today-so-far. Missing keys mean
	// "no data", not zero.
	Hourly map[time.Time]float64
	// FutureBuckets are the hour-aligned bucket starts to forecast, in order.
	FutureBuckets []time.Time
}

// SpendForecastResult is ForecastFutureSpend's output.
type SpendForecastResult struct {
	ForecastByBucket map[time.Time]float64
	// Ratio is the today-vs-recent-days pace multiplier actually applied
	// (exposed for tests/observability).
	Ratio float64
}

// ForecastFutureSpend predicts per-hour spend for FutureBuckets using a
// same-hour-yesterday / day-before-yesterday weighted shape (0.6/0.4,
// renormalized when one day is missing), scaled by how today's pace compares
// to the same two days (clamped to [0.25, 4]). Falls back to the recent
// 3-hour average rate for any bucket with no shape history at all.
func ForecastFutureSpend(input SpendForecastInput) SpendForecastResult {
	hourly := input.Hourly
	if hourly == nil {
		hourly = map[time.Time]float64{}
	}
	result := SpendForecastResult{ForecastByBucket: make(map[time.Time]float64, len(input.FutureBuckets))}

	recentAvgRate := recentAverageRate(hourly, input.CurrentBucket)
	ratio := computeSpendRatio(hourly, input.CurrentBucket)
	result.Ratio = ratio

	for _, bucket := range input.FutureBuckets {
		base, hasHistory := forecastBaseForBucket(hourly, bucket, input.CurrentBucket, recentAvgRate)
		if hasHistory {
			result.ForecastByBucket[bucket] = base * ratio
		} else {
			// No same-hour history for this bucket at all: the recent-rate
			// fallback already reflects current pace, so ratio is not
			// reapplied on top of it.
			result.ForecastByBucket[bucket] = base
		}
	}
	return result
}

// forecastBaseForBucket builds the same-clock-hour shape base for a future
// bucket. For buckets more than 24h/48h ahead (7d range), "yesterday"/"day
// before" are themselves still in the future with no possible data — so it
// walks back in whole days from the bucket, skipping day offsets that land
// after CurrentBucket, and weights the two most recent same-clock-hour values
// it finds 0.6/0.4 (a single value is used as-is).
func forecastBaseForBucket(hourly map[time.Time]float64, bucket, currentBucket time.Time, recentAvgRate float64) (float64, bool) {
	var vals []float64
	for day := 1; day <= capacityShapeLookbackDays && len(vals) < 2; day++ {
		t := bucket.Add(-time.Duration(day) * 24 * time.Hour)
		if t.After(currentBucket) {
			continue // that day back is still in the future — no data possible
		}
		if v, ok := hourly[t]; ok {
			vals = append(vals, v)
		}
	}
	switch len(vals) {
	case 2:
		return 0.6*vals[0] + 0.4*vals[1], true
	case 1:
		return vals[0], true
	default:
		return recentAvgRate, false
	}
}

func recentAverageRate(hourly map[time.Time]float64, currentBucket time.Time) float64 {
	var sum float64
	var count int
	for i := 1; i <= 3; i++ {
		if v, ok := hourly[currentBucket.Add(-time.Duration(i)*time.Hour)]; ok {
			sum += v
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// computeSpendRatio compares today's cumulative spend (midnight through the
// last fully-elapsed hour before currentBucket) against the mean of
// yesterday's/day-before's cumulative spend over the same elapsed clock-time
// window. Returns 1 (no scaling) when the elapsed window is under 2h or there
// is no comparable denominator.
func computeSpendRatio(hourly map[time.Time]float64, currentBucket time.Time) float64 {
	todayStart := time.Date(currentBucket.Year(), currentBucket.Month(), currentBucket.Day(), 0, 0, 0, 0, currentBucket.Location())
	elapsed := currentBucket.Sub(todayStart)
	if elapsed < capacityMinElapsedForRatio {
		return 1
	}
	elapsedHours := int(elapsed / time.Hour)

	todayCumulative := sumRange(hourly, todayStart, elapsedHours)

	var denomSum float64
	var denomCount int
	if v, ok := sumRangeIfAny(hourly, todayStart.Add(-24*time.Hour), elapsedHours); ok {
		denomSum += v
		denomCount++
	}
	if v, ok := sumRangeIfAny(hourly, todayStart.Add(-48*time.Hour), elapsedHours); ok {
		denomSum += v
		denomCount++
	}
	if denomCount == 0 {
		return 1
	}
	denom := denomSum / float64(denomCount)
	if denom <= 0 {
		return 1
	}

	ratio := todayCumulative / denom
	if ratio < capacityRatioMin {
		ratio = capacityRatioMin
	}
	if ratio > capacityRatioMax {
		ratio = capacityRatioMax
	}
	return ratio
}

func sumRange(hourly map[time.Time]float64, start time.Time, hours int) float64 {
	var sum float64
	for i := 0; i < hours; i++ {
		sum += hourly[start.Add(time.Duration(i)*time.Hour)]
	}
	return sum
}

// sumRangeIfAny sums like sumRange but reports ok=false when none of the
// hours in range have data at all, so a fully-missing comparison day is
// excluded from the denominator rather than counted as a real zero-spend day.
func sumRangeIfAny(hourly map[time.Time]float64, start time.Time, hours int) (float64, bool) {
	var sum float64
	var found bool
	for i := 0; i < hours; i++ {
		if v, ok := hourly[start.Add(time.Duration(i)*time.Hour)]; ok {
			sum += v
			found = true
		}
	}
	return sum, found
}

// ---------------------------------------------------------------------------
// Future capacity simulation
// ---------------------------------------------------------------------------

// CapacityWindowState is one account window's state at simulation start.
// CapacityUSD <= 0 means "not constraining" (inferred capacity unknown).
type CapacityWindowState struct {
	Kind        string
	UsedRatio   float64
	ResetAt     time.Time
	Duration    time.Duration
	CapacityUSD float64
}

// CapacityAccountState is one account's full window set at simulation start.
type CapacityAccountState struct {
	ID      int64
	Name    string
	Windows []CapacityWindowState
}

// CapacitySimulationInput is the pure input to SimulateFutureCapacity.
type CapacitySimulationInput struct {
	// CurrentBucket is "now" truncated down to the hour (UTC).
	CurrentBucket time.Time
	// Buckets are the future hour-aligned bucket starts, in chronological order.
	Buckets []time.Time
	// RangeEnd is the exclusive end of the simulated range, used to bound
	// short-window reset extrapolation.
	RangeEnd time.Time
	// CurrentAvailable is the live headroom at CurrentBucket (sum of
	// per-account min-over-windows headroom).
	CurrentAvailable float64
	Accounts         []CapacityAccountState
	// ForecastSpend must contain an entry for CurrentBucket (remaining-hour
	// forecast) and every bucket in Buckets except the last (whose forecast
	// is never consumed by the rolling balance).
	ForecastSpend map[time.Time]float64
}

// CapacityRecoverEvent groups same-bucket, same-window-kind recoveries across
// accounts into a single event (matches the API's events[] shape).
type CapacityRecoverEvent struct {
	At         time.Time
	WindowKind string
	AmountUSD  float64
	Accounts   []CapacityEventAccountRef
}

// CapacitySimulationResult is SimulateFutureCapacity's output.
type CapacitySimulationResult struct {
	// AvailableByBucket includes CurrentBucket plus every future bucket.
	AvailableByBucket map[time.Time]float64
	RecoveredByBucket map[time.Time]float64
	RecoverEvents     []CapacityRecoverEvent
}

// accountHeadroom returns min(capacity_w * (1 - used_ratio_w)) over an
// account's constrained windows (CapacityUSD > 0). ok=false when the account
// has no constrained window at all, in which case it contributes nothing to
// the scope's headroom (never fabricated).
func accountHeadroom(windows []CapacityWindowState) (float64, bool) {
	best := math.Inf(1)
	constrained := false
	for _, w := range windows {
		if w.CapacityUSD <= 0 {
			continue
		}
		constrained = true
		h := w.CapacityUSD * (1 - w.UsedRatio)
		if h < 0 {
			h = 0
		}
		if h < best {
			best = h
		}
	}
	if !constrained {
		return 0, false
	}
	return best, true
}

// accountHeadroomWithCapacity is like accountHeadroom but also returns the
// CapacityUSD of the constraining (argmin-headroom) window, used when sealing
// capacity_hourly's capacity_usd column (the "size" of the resource the
// current headroom is a fraction of).
func accountHeadroomWithCapacity(windows []CapacityWindowState) (headroom, capacity float64, ok bool) {
	best := math.Inf(1)
	var bestCapacity float64
	constrained := false
	for _, w := range windows {
		if w.CapacityUSD <= 0 {
			continue
		}
		constrained = true
		h := w.CapacityUSD * (1 - w.UsedRatio)
		if h < 0 {
			h = 0
		}
		if h < best {
			best = h
			bestCapacity = w.CapacityUSD
		}
	}
	if !constrained {
		return 0, 0, false
	}
	return best, bestCapacity, true
}

type capacityResetEvent struct {
	at         time.Time
	accountIdx int
	windowIdx  int
}

// SimulateFutureCapacity rolls the available-capacity balance forward:
// available(b) = max(0, available(b-1) - forecastSpend(b-1)) + recovered(b),
// where recovered(b) sums each account's headroom delta at every window reset
// falling in bucket b. A window reset recomputes that account's headroom with
// its used_ratio set to 0; the delta (>= 0) is what actually becomes newly
// available — naturally expressing that e.g. a 7d reset stays capped by a
// still-exhausted 5h window, while an account constrained only by a 30d
// window unlocks its full capacity on that window's reset. Short windows
// (Duration <= 6h) are extrapolated periodically across the whole range, each
// occurrence's release capped at that window's own capacity.
func SimulateFutureCapacity(input CapacitySimulationInput) CapacitySimulationResult {
	result := CapacitySimulationResult{
		AvailableByBucket: make(map[time.Time]float64, len(input.Buckets)+1),
		RecoveredByBucket: make(map[time.Time]float64, len(input.Buckets)),
	}

	// Deep-copy so we can mutate used_ratio as events fire without touching
	// the caller's slice.
	accounts := make([]CapacityAccountState, len(input.Accounts))
	for i, acc := range input.Accounts {
		accounts[i] = acc
		accounts[i].Windows = append([]CapacityWindowState(nil), acc.Windows...)
	}

	var events []capacityResetEvent
	for ai, acc := range accounts {
		for wi, w := range acc.Windows {
			if w.Duration <= 0 || w.ResetAt.IsZero() {
				continue
			}
			resetAt := w.ResetAt
			for iterations := 0; !resetAt.After(input.RangeEnd) && iterations < capacityMaxExtrapolatedResets; iterations++ {
				if resetAt.After(input.CurrentBucket) {
					events = append(events, capacityResetEvent{at: resetAt, accountIdx: ai, windowIdx: wi})
				}
				if w.Duration > capacityShortWindowMaxDuration {
					break
				}
				resetAt = resetAt.Add(w.Duration)
			}
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })

	type recoverAgg struct {
		windowKind string
		amount     float64
		accounts   []CapacityEventAccountRef
	}
	recoverByBucketWindow := make(map[time.Time]map[string]*recoverAgg)

	available := input.CurrentAvailable
	result.AvailableByBucket[input.CurrentBucket] = available

	eventIdx := 0
	prevBucket := input.CurrentBucket
	for _, bucket := range input.Buckets {
		available = math.Max(0, available-input.ForecastSpend[prevBucket])

		for eventIdx < len(events) && !events[eventIdx].at.After(bucket) {
			ev := events[eventIdx]
			eventIdx++
			acc := &accounts[ev.accountIdx]
			w := &acc.Windows[ev.windowIdx]

			oldHeadroom, _ := accountHeadroom(acc.Windows)
			w.UsedRatio = 0
			newHeadroom, _ := accountHeadroom(acc.Windows)

			delta := newHeadroom - oldHeadroom
			if delta <= 0 {
				continue
			}
			if w.CapacityUSD > 0 && delta > w.CapacityUSD {
				delta = w.CapacityUSD
			}
			available += delta

			bucketKey := ev.at.Truncate(time.Hour)
			if recoverByBucketWindow[bucketKey] == nil {
				recoverByBucketWindow[bucketKey] = make(map[string]*recoverAgg)
			}
			agg := recoverByBucketWindow[bucketKey][w.Kind]
			if agg == nil {
				agg = &recoverAgg{windowKind: w.Kind}
				recoverByBucketWindow[bucketKey][w.Kind] = agg
			}
			agg.amount += delta
			agg.accounts = append(agg.accounts, CapacityEventAccountRef{ID: acc.ID, Name: acc.Name})
			result.RecoveredByBucket[bucket] += delta
		}

		result.AvailableByBucket[bucket] = available
		prevBucket = bucket
	}

	for bucket, byWindow := range recoverByBucketWindow {
		for _, agg := range byWindow {
			result.RecoverEvents = append(result.RecoverEvents, CapacityRecoverEvent{
				At: bucket, WindowKind: agg.windowKind, AmountUSD: agg.amount, Accounts: agg.accounts,
			})
		}
	}
	sort.Slice(result.RecoverEvents, func(i, j int) bool { return result.RecoverEvents[i].At.Before(result.RecoverEvents[j].At) })

	return result
}

// ---------------------------------------------------------------------------
// Gap detection & recommendations
// ---------------------------------------------------------------------------

// CapacityShortfallPoint is one future bucket's forecast supply/demand pair.
type CapacityShortfallPoint struct {
	BucketStart       time.Time
	ForecastAvailable float64
	ForecastSpend     float64
}

// ComputeShortfallEvents finds every future bucket where forecast demand
// exceeds forecast supply, and — if any exist — sizes a single "add accounts"
// recommendation off the largest gap divided by the scope's median inferred
// account capacity.
func ComputeShortfallEvents(points []CapacityShortfallPoint, medianCapacity float64) ([]CapacityEvent, *time.Time, []CapacityRecommendation) {
	var events []CapacityEvent
	var firstShortfallAt *time.Time
	var maxGap float64
	var maxGapAt time.Time

	for _, p := range points {
		gap := p.ForecastSpend - p.ForecastAvailable
		if gap <= 0 {
			continue
		}
		events = append(events, CapacityEvent{
			At:           p.BucketStart,
			Type:         "shortfall",
			WindowKind:   "",
			AccountCount: 0,
			AmountUSD:    gap,
			Accounts:     []CapacityEventAccountRef{},
		})
		if firstShortfallAt == nil {
			t := p.BucketStart
			firstShortfallAt = &t
		}
		if gap > maxGap {
			maxGap = gap
			maxGapAt = p.BucketStart
		}
	}

	if maxGap <= 0 {
		return events, firstShortfallAt, nil
	}

	suggested := 0
	basis := fmt.Sprintf("max gap %.4f USD found, but no positive median account capacity in scope to size a recommendation", maxGap)
	if medianCapacity > 0 {
		suggested = int(math.Ceil(maxGap / medianCapacity))
		basis = fmt.Sprintf("ceil(max gap %.4f USD / median inferred account capacity %.4f USD)", maxGap, medianCapacity)
	}
	recommendations := []CapacityRecommendation{{
		At:              maxGapAt,
		GapUSD:          maxGap,
		SuggestAccounts: suggested,
		Basis:           basis,
	}}
	return events, firstShortfallAt, recommendations
}

// medianPositiveValues returns the median of the positive values in vs, or 0
// if there are none.
func medianPositiveValues(vs []float64) float64 {
	positives := make([]float64, 0, len(vs))
	for _, v := range vs {
		if v > 0 {
			positives = append(positives, v)
		}
	}
	if len(positives) == 0 {
		return 0
	}
	sort.Float64s(positives)
	n := len(positives)
	if n%2 == 1 {
		return positives[n/2]
	}
	return (positives[n/2-1] + positives[n/2]) / 2
}
