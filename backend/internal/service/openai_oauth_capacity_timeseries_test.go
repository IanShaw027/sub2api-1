package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthTimeseriesHourlyRateBlendsThreeSignals(t *testing.T) {
	// recent=30/3=10, prev=18/3=6, rpm=12
	// (10*0.5 + 6*0.3 + 12*0.2) / 1.0 = 9.2
	require.InDelta(t, 9.2, openAIOAuthTimeseriesHourlyRate(30, 18, 12), 0.0001)
	require.InDelta(t, 10, openAIOAuthTimeseriesHourlyRate(30, 0, 0), 0.0001)
	require.InDelta(t, 6, openAIOAuthTimeseriesHourlyRate(0, 18, 0), 0.0001)
	require.InDelta(t, 12, openAIOAuthTimeseriesHourlyRate(0, 0, 12), 0.0001)
	require.InDelta(t, 0, openAIOAuthTimeseriesHourlyRate(0, 0, 0), 0.0001)
}

func TestNormalizeOpenAIOAuthTimeseriesQueryDefaults(t *testing.T) {
	q, err := normalizeOpenAIOAuthTimeseriesQuery(OpenAIOAuthCapacityTimeseriesQuery{})
	require.NoError(t, err)
	require.Equal(t, "24h", q.Range)
	require.Equal(t, "5h", q.Window)
	require.Equal(t, 12, q.PastHours)
	require.Equal(t, 12, q.FutureHours)
}

func TestNormalizeOpenAIOAuthTimeseriesQueryRejectsBadWindow(t *testing.T) {
	_, err := normalizeOpenAIOAuthTimeseriesQuery(OpenAIOAuthCapacityTimeseriesQuery{Window: "1h"})
	require.Error(t, err)
}

func TestNormalizeOpenAIOAuthTimeseriesQueryTwelveHourIsDistinct(t *testing.T) {
	q, err := normalizeOpenAIOAuthTimeseriesQuery(OpenAIOAuthCapacityTimeseriesQuery{Range: "12h"})
	require.NoError(t, err)
	require.Equal(t, 6, q.PastHours)
	require.Equal(t, 6, q.FutureHours)
}

func TestNormalizeOpenAIOAuthTimeseriesQueryRejectsHugeCustomRange(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(30 * 24 * time.Hour)
	_, err := normalizeOpenAIOAuthTimeseriesQuery(OpenAIOAuthCapacityTimeseriesQuery{
		Range: "custom", From: &from, To: &to,
	})
	require.Error(t, err)
}

func TestOpenAIOAuthShapeFactorUsesPreviousDaySeasonality(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 30, 0, 0, time.UTC)
	current := now.Truncate(time.Hour)
	shape := map[time.Time]float64{
		current:                    10,
		current.Add(1 * time.Hour): 20,
		current.Add(2 * time.Hour): 30,
	}
	require.InDelta(t, 0.5, openAIOAuthShapeFactor(shape, current, 8), 0.0001)
	require.InDelta(t, 1.0, openAIOAuthShapeFactor(shape, current.Add(time.Hour), 8), 0.0001)
	require.InDelta(t, 1.5, openAIOAuthShapeFactor(shape, current.Add(2*time.Hour), 8), 0.0001)
	require.InDelta(t, 0.5, openAIOAuthShapeFactor(shape, current.Add(5*time.Hour), 8), 0.0001)
}

func TestOpenAIOAuthAvailableAtUsesRollingWindowSpend(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	hourly := map[time.Time]float64{
		now.Add(-5 * time.Hour): 10,
		now.Add(-4 * time.Hour): 10,
		now.Add(-3 * time.Hour): 10,
		now.Add(-2 * time.Hour): 10,
		now.Add(-1 * time.Hour): 10,
	}
	avail, used := openAIOAuthAvailableAt(100, hourly, now, 5*time.Hour, now, nil, 0, nil)
	require.InDelta(t, 50, avail, 0.01)
	require.InDelta(t, 50, used, 0.01)
}

func TestOpenAIOAuthAvailableAtProjectsFutureForecast(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 30, 0, 0, time.UTC)
	currentHour := now.Truncate(time.Hour)
	hourly := map[time.Time]float64{
		currentHour: 5,
	}
	forecast := map[time.Time]float64{
		currentHour:                5,
		currentHour.Add(time.Hour): 10,
	}
	at := currentHour.Add(2 * time.Hour)
	avail, _ := openAIOAuthAvailableAt(100, hourly, at, 5*time.Hour, now, forecast, 10, nil)
	require.InDelta(t, 80, avail, 0.5)
}

func TestAggregateOpenAIOAuthHourlySpendFiltersGroup(t *testing.T) {
	rows := []OpenAIOAuthHourlySpendRow{
		{BucketStart: time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC), GroupID: 1, SpendUSD: 3},
		{BucketStart: time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC), GroupID: 2, SpendUSD: 7},
		{BucketStart: time.Date(2026, 7, 15, 11, 0, 0, 0, time.UTC), GroupID: 1, SpendUSD: 4},
	}
	groupID := int64(1)
	filtered := aggregateOpenAIOAuthHourlySpend(rows, &groupID)
	require.InDelta(t, 3, filtered[time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)], 0.0001)
	require.InDelta(t, 4, filtered[time.Date(2026, 7, 15, 11, 0, 0, 0, time.UTC)], 0.0001)

	all := aggregateOpenAIOAuthHourlySpend(rows, nil)
	require.InDelta(t, 10, all[time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)], 0.0001)
}

func TestResolveOpenAIOAuthTimeseriesBoundsDefault(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 42, 0, 0, time.UTC)
	q := OpenAIOAuthCapacityTimeseriesQuery{Range: "24h", PastHours: 12, FutureHours: 12}
	past, future, start, end := resolveOpenAIOAuthTimeseriesBounds(q, now)
	require.Equal(t, 12, past)
	require.Equal(t, 12, future)
	require.Equal(t, time.Date(2026, 7, 15, 6, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, 7, 16, 7, 0, 0, 0, time.UTC), end)
	require.Equal(t, past+future+1, int(end.Sub(start).Hours()))
}

func TestResolveOpenAIOAuthTimeseriesBoundsCustomIncludesPartialEndHour(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 42, 0, 0, time.UTC)
	from := time.Date(2026, 7, 15, 10, 15, 0, 0, time.UTC)
	to := time.Date(2026, 7, 15, 12, 45, 0, 0, time.UTC)
	q := OpenAIOAuthCapacityTimeseriesQuery{Range: "custom", From: &from, To: &to}
	past, future, start, end := resolveOpenAIOAuthTimeseriesBounds(q, now)
	require.Equal(t, 3, past)
	require.Equal(t, 0, future)
	require.Equal(t, time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC), end)
}

func TestResolveOpenAIOAuthTimeseriesBoundsCustomSubHourExpands(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 42, 0, 0, time.UTC)
	from := time.Date(2026, 7, 15, 10, 15, 0, 0, time.UTC)
	to := time.Date(2026, 7, 15, 10, 45, 0, 0, time.UTC)
	q := OpenAIOAuthCapacityTimeseriesQuery{Range: "custom", From: &from, To: &to}
	past, future, start, end := resolveOpenAIOAuthTimeseriesBounds(q, now)
	require.Equal(t, 1, past)
	require.Equal(t, 0, future)
	require.Equal(t, time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, 7, 15, 11, 0, 0, 0, time.UTC), end)
}

func TestOpenAIOAuthHourlySpendFromFacts(t *testing.T) {
	bucket := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	facts := []OpenAIOAuthCapacityHourlyFact{
		{BucketStart: bucket, SpentUSD: 12.5, Sealed: true},
		{BucketStart: bucket.Add(time.Hour), SpentUSD: 0, Sealed: true},
	}
	hourly := openAIOAuthHourlySpendFromFacts(facts)
	require.InDelta(t, 12.5, hourly[bucket], 0.0001)
	require.InDelta(t, 0, hourly[bucket.Add(time.Hour)], 0.0001)
}

func TestApplySealedWindowMetricsOmitsUnknown(t *testing.T) {
	cap5 := 100.0
	avail5 := 40.0
	point := OpenAIOAuthCapacityTimeseriesPoint{}
	applySealedWindowMetrics(&point, OpenAIOAuthCapacityHourlyFact{
		Capacity5hUSD:  &cap5,
		Available5hUSD: &avail5,
	}, "both")
	require.NotNil(t, point.CapacityUSD)
	require.NotNil(t, point.AvailableUSD)
	require.Nil(t, point.Capacity7dUSD)
	require.Nil(t, point.Available7dUSD)
}

type fakeCapacityTimeseriesRepo struct {
	facts      map[string]OpenAIOAuthCapacityHourlyFact
	spendCalls int
	spend      map[time.Time]float64
	upserts    []OpenAIOAuthCapacityHourlyFact
}

func (f *fakeCapacityTimeseriesRepo) GetOpenAIOAuthCapacityRawData(ctx context.Context, now time.Time) (*OpenAIOAuthCapacityRawData, error) {
	return &OpenAIOAuthCapacityRawData{}, nil
}

func (f *fakeCapacityTimeseriesRepo) GetOpenAIOAuthCapacityHourlySpend(ctx context.Context, from, to time.Time) ([]OpenAIOAuthHourlySpendRow, error) {
	f.spendCalls++
	rows := make([]OpenAIOAuthHourlySpendRow, 0)
	for bucket, spend := range f.spend {
		if !bucket.Before(from) && bucket.Before(to) {
			rows = append(rows, OpenAIOAuthHourlySpendRow{BucketStart: bucket, GroupID: 0, SpendUSD: spend})
		}
	}
	return rows, nil
}

func (f *fakeCapacityTimeseriesRepo) GetOpenAIOAuthCapacityHourlyFacts(ctx context.Context, from, to time.Time, groupID *int64, sealedOnly bool) ([]OpenAIOAuthCapacityHourlyFact, error) {
	gid := OpenAIOAuthCapacityAllGroupsID
	if groupID != nil {
		gid = *groupID
	}
	end := to.UTC().Truncate(time.Hour).Add(time.Hour)
	out := make([]OpenAIOAuthCapacityHourlyFact, 0)
	for _, fact := range f.facts {
		if fact.GroupID != gid {
			continue
		}
		b := fact.BucketStart.UTC().Truncate(time.Hour)
		if b.Before(from.UTC().Truncate(time.Hour)) || !b.Before(end) {
			continue
		}
		if sealedOnly && !fact.Sealed {
			continue
		}
		out = append(out, fact)
	}
	return out, nil
}

func (f *fakeCapacityTimeseriesRepo) UpsertOpenAIOAuthCapacityHourlyFacts(ctx context.Context, facts []OpenAIOAuthCapacityHourlyFact) error {
	if f.facts == nil {
		f.facts = make(map[string]OpenAIOAuthCapacityHourlyFact)
	}
	for _, fact := range facts {
		k := fmt.Sprintf("%s|%d", fact.BucketStart.UTC().Format(time.RFC3339), fact.GroupID)
		existing, ok := f.facts[k]
		if ok && existing.Sealed {
			if existing.Capacity5hUSD == nil {
				existing.Capacity5hUSD = fact.Capacity5hUSD
			}
			if existing.Available5hUSD == nil {
				existing.Available5hUSD = fact.Available5hUSD
			}
			if existing.UsedPercent5h == nil {
				existing.UsedPercent5h = fact.UsedPercent5h
			}
			if existing.Capacity7dUSD == nil {
				existing.Capacity7dUSD = fact.Capacity7dUSD
			}
			if existing.Available7dUSD == nil {
				existing.Available7dUSD = fact.Available7dUSD
			}
			if existing.UsedPercent7d == nil {
				existing.UsedPercent7d = fact.UsedPercent7d
			}
			existing.Sealed = true
			f.facts[k] = existing
		} else {
			f.facts[k] = fact
		}
		f.upserts = append(f.upserts, fact)
	}
	return nil
}

func (f *fakeCapacityTimeseriesRepo) GetOpenAIOAuthCapacityRecentSpend(ctx context.Context, from, to time.Time, groupID *int64) (float64, error) {
	return 0, nil
}

func TestLoadOpenAIOAuthHourlyFactsWithBackfillSealsOnce(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 30, 0, 0, time.UTC)
	current := now.Truncate(time.Hour)
	past1 := current.Add(-time.Hour)
	past2 := current.Add(-2 * time.Hour)
	repo := &fakeCapacityTimeseriesRepo{
		facts: make(map[string]OpenAIOAuthCapacityHourlyFact),
		spend: map[time.Time]float64{
			past2:   5,
			past1:   7,
			current: 3,
		},
	}
	from := past2
	facts, err := loadOpenAIOAuthHourlyFactsWithBackfill(context.Background(), repo, from, now, nil, false)
	require.NoError(t, err)
	require.Equal(t, 1, repo.spendCalls)
	require.GreaterOrEqual(t, len(facts), 2)

	var sealedPast int
	for _, f := range facts {
		if f.BucketStart.Before(current) {
			require.True(t, f.Sealed, "past hour must be sealed")
			require.Nil(t, f.Capacity5hUSD, "historical capacity must not be inferred at read time")
			require.Nil(t, f.Available5hUSD, "historical availability must remain unknown")
			sealedPast++
		}
		if f.BucketStart.Equal(current) {
			require.False(t, f.Sealed, "current hour must stay unsealed")
		}
	}
	require.Equal(t, 2, sealedPast)

	repo.spendCalls = 0
	facts2, err := loadOpenAIOAuthHourlyFactsWithBackfill(context.Background(), repo, from, now, nil, false)
	require.NoError(t, err)
	require.LessOrEqual(t, repo.spendCalls, 1)
	require.NotEmpty(t, facts2)

	k := fmt.Sprintf("%s|%d", past1.UTC().Format(time.RFC3339), OpenAIOAuthCapacityAllGroupsID)
	before := repo.facts[k]
	repo.spend[past1] = 999
	_, err = loadOpenAIOAuthHourlyFactsWithBackfill(context.Background(), repo, from, now, nil, true)
	require.NoError(t, err)
	after := repo.facts[k]
	require.InDelta(t, before.SpentUSD, after.SpentUSD, 0.0001)
	require.True(t, after.Sealed)
}

func TestLoadOpenAIOAuthHourlyFactsSeparatesGlobalAndUngrouped(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 30, 0, 0, time.UTC)
	past := now.Truncate(time.Hour).Add(-time.Hour)
	repo := &fakeCapacityTimeseriesRepo{
		facts: make(map[string]OpenAIOAuthCapacityHourlyFact),
		spend: map[time.Time]float64{past: 5},
	}

	_, err := loadOpenAIOAuthHourlyFactsWithBackfill(context.Background(), repo, past, now, nil, false)
	require.NoError(t, err)
	ungrouped := int64(0)
	_, err = loadOpenAIOAuthHourlyFactsWithBackfill(context.Background(), repo, past, now, &ungrouped, false)
	require.NoError(t, err)

	prefix := past.UTC().Format(time.RFC3339)
	require.Contains(t, repo.facts, fmt.Sprintf("%s|%d", prefix, OpenAIOAuthCapacityAllGroupsID))
	require.Contains(t, repo.facts, prefix+"|0")
}

func TestLoadOpenAIOAuthHourlyFactsDoesNotSealMissingPastBucket(t *testing.T) {
	now := time.Date(2026, 7, 15, 18, 30, 0, 0, time.UTC)
	past := now.Truncate(time.Hour).Add(-time.Hour)
	repo := &fakeCapacityTimeseriesRepo{
		facts: make(map[string]OpenAIOAuthCapacityHourlyFact),
		spend: make(map[time.Time]float64),
	}

	_, err := loadOpenAIOAuthHourlyFactsWithBackfill(context.Background(), repo, past, now, nil, false)
	require.NoError(t, err)
	require.NotContains(t, repo.facts, fmt.Sprintf("%s|%d", past.UTC().Format(time.RFC3339), OpenAIOAuthCapacityAllGroupsID))
}
