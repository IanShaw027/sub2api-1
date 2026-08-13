//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeCapacityRepository is a minimal in-memory CapacityRepository stub used
// to exercise CapacityForecastService.buildAccountStates' capacity-inference
// fallback chain without a database.
type fakeCapacityRepository struct {
	// accountSpend maps accountID -> spend returned by GetAccountSpendBetween.
	accountSpend map[int64]float64
	// periodCapacity maps "accountID/windowKind" -> fallback capacity.
	periodCapacity map[string]float64
}

func (f *fakeCapacityRepository) GetHourlySpend(ctx context.Context, platform string, groupID *int64, from, to time.Time) ([]CapacityHourlySpendPoint, error) {
	return nil, nil
}

func (f *fakeCapacityRepository) GetAccountSpendBetween(ctx context.Context, accountID int64, from, to time.Time) (float64, error) {
	return f.accountSpend[accountID], nil
}

func (f *fakeCapacityRepository) GetAccountWindowSpends(ctx context.Context, reqs []CapacityAccountWindowSpendRequest, to time.Time) (map[CapacityAccountWindowSpendKey]float64, error) {
	out := make(map[CapacityAccountWindowSpendKey]float64, len(reqs))
	for _, req := range reqs {
		out[CapacityAccountWindowSpendKey{AccountID: req.AccountID, WindowKind: req.WindowKind}] = f.accountSpend[req.AccountID]
	}
	return out, nil
}

func (f *fakeCapacityRepository) GetHourlyFacts(ctx context.Context, platform string, groupID *int64, from, to time.Time) (map[time.Time]CapacityHourlyFact, error) {
	return nil, nil
}

func (f *fakeCapacityRepository) UpsertHourlyFacts(ctx context.Context, facts []CapacityHourlyUpsert) error {
	return nil
}

func (f *fakeCapacityRepository) UpsertQuotaSnapshot(ctx context.Context, snapshot AccountQuotaSnapshot) error {
	return nil
}

func (f *fakeCapacityRepository) InsertQuotaPeriod(ctx context.Context, period AccountQuotaPeriod) error {
	return nil
}

func (f *fakeCapacityRepository) GetLatestQuotaPeriodCapacity(ctx context.Context, platform string, accountID int64, windowKind string) (*float64, error) {
	key := windowKindKey(accountID, windowKind)
	if v, ok := f.periodCapacity[key]; ok {
		return &v, nil
	}
	return nil, nil
}

func windowKindKey(accountID int64, windowKind string) string {
	return strconv.FormatInt(accountID, 10) + "/" + windowKind
}

// fakeCapacityProvider returns pre-canned QuotaWindows per account, keyed by
// account ID, so buildAccountStates can be driven deterministically.
type fakeCapacityProvider struct {
	windowsByAccount map[int64][]QuotaWindow
}

func (f *fakeCapacityProvider) Platform() string { return PlatformOpenAI }
func (f *fakeCapacityProvider) ExtractWindows(acc *Account, now time.Time) []QuotaWindow {
	return f.windowsByAccount[acc.ID]
}
func (f *fakeCapacityProvider) Probe(ctx context.Context, acc *Account) error { return nil }

func TestBuildAccountStatesLiveInference(t *testing.T) {
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(3 * time.Hour)
	repo := &fakeCapacityRepository{accountSpend: map[int64]float64{1: 40}}
	provider := &fakeCapacityProvider{windowsByAccount: map[int64][]QuotaWindow{
		1: {{Kind: "5h", UsedRatio: 0.4, ResetAt: resetAt, Duration: 5 * time.Hour}},
	}}
	svc := &CapacityForecastService{repo: repo}

	states, err := svc.buildAccountStates(context.Background(), provider, PlatformOpenAI, []Account{{ID: 1, Name: "a"}}, now)
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.Len(t, states[0].Windows, 1)
	// live inference: spend/used_ratio = 40/0.4 = 100
	require.InDelta(t, 100.0, states[0].Windows[0].CapacityUSD, 1e-9)
}

func TestBuildAccountStatesFallsBackToArchivedPeriod(t *testing.T) {
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(3 * time.Hour)
	// used_ratio below capacityMinimumInferenceRatio -> live inference skipped.
	repo := &fakeCapacityRepository{
		accountSpend:   map[int64]float64{1: 1},
		periodCapacity: map[string]float64{windowKindKey(1, "5h"): 250},
	}
	provider := &fakeCapacityProvider{windowsByAccount: map[int64][]QuotaWindow{
		1: {{Kind: "5h", UsedRatio: 0.01, ResetAt: resetAt, Duration: 5 * time.Hour}},
	}}
	svc := &CapacityForecastService{repo: repo}

	states, err := svc.buildAccountStates(context.Background(), provider, PlatformOpenAI, []Account{{ID: 1, Name: "a"}}, now)
	require.NoError(t, err)
	require.InDelta(t, 250.0, states[0].Windows[0].CapacityUSD, 1e-9)
}

func TestBuildAccountStatesFallsBackToScopeMedian(t *testing.T) {
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(3 * time.Hour)
	repo := &fakeCapacityRepository{
		accountSpend: map[int64]float64{
			1: 100, // live-inferred: 100/0.5 = 200
			2: 300, // live-inferred: 300/0.5 = 600
			3: 1,   // used_ratio too low -> needs fallback, no archived period
		},
		periodCapacity: map[string]float64{},
	}
	provider := &fakeCapacityProvider{windowsByAccount: map[int64][]QuotaWindow{
		1: {{Kind: "5h", UsedRatio: 0.5, ResetAt: resetAt, Duration: 5 * time.Hour}},
		2: {{Kind: "5h", UsedRatio: 0.5, ResetAt: resetAt, Duration: 5 * time.Hour}},
		3: {{Kind: "5h", UsedRatio: 0.01, ResetAt: resetAt, Duration: 5 * time.Hour}},
	}}
	svc := &CapacityForecastService{repo: repo}

	states, err := svc.buildAccountStates(context.Background(), provider, PlatformOpenAI, []Account{
		{ID: 1, Name: "a"}, {ID: 2, Name: "b"}, {ID: 3, Name: "c"},
	}, now)
	require.NoError(t, err)
	require.Len(t, states, 3)

	byID := map[int64]CapacityAccountState{}
	for _, st := range states {
		byID[st.ID] = st
	}
	require.InDelta(t, 200.0, byID[1].Windows[0].CapacityUSD, 1e-9)
	require.InDelta(t, 600.0, byID[2].Windows[0].CapacityUSD, 1e-9)
	// median(200, 600) = 400
	require.InDelta(t, 400.0, byID[3].Windows[0].CapacityUSD, 1e-9)
}

func TestBuildAccountStatesUnconstrainedWhenNoFallbackAvailable(t *testing.T) {
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(3 * time.Hour)
	// Single account, no archived period, no cohort (median of nothing) -> the
	// window must be left non-constraining (CapacityUSD == 0).
	repo := &fakeCapacityRepository{accountSpend: map[int64]float64{1: 1}}
	provider := &fakeCapacityProvider{windowsByAccount: map[int64][]QuotaWindow{
		1: {{Kind: "5h", UsedRatio: 0.01, ResetAt: resetAt, Duration: 5 * time.Hour}},
	}}
	svc := &CapacityForecastService{repo: repo}

	states, err := svc.buildAccountStates(context.Background(), provider, PlatformOpenAI, []Account{{ID: 1, Name: "a"}}, now)
	require.NoError(t, err)
	require.InDelta(t, 0.0, states[0].Windows[0].CapacityUSD, 1e-9)

	_, ok := accountHeadroom(states[0].Windows)
	require.False(t, ok, "an account with no constrained window must not contribute headroom")
}
