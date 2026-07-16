package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthForecastHourlyRateBlendsRecentAndPreviousDay(t *testing.T) {
	require.InDelta(t, 8.6, openAIOAuthForecastHourlyRate(30, 18), 0.0001)
	require.InDelta(t, 10, openAIOAuthForecastHourlyRate(30, 0), 0.0001)
	require.InDelta(t, 6, openAIOAuthForecastHourlyRate(0, 18), 0.0001)
}

func TestOpenAIOAuthRateLimitBucketsAreExclusive(t *testing.T) {
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	buckets := OpenAIOAuthRateLimitBuckets{}
	for _, remaining := range []time.Duration{
		5 * time.Minute,
		20 * time.Minute,
		45 * time.Minute,
		2 * time.Hour,
		4 * time.Hour,
		12 * time.Hour,
		48 * time.Hour,
		96 * time.Hour,
	} {
		reset := now.Add(remaining)
		addOpenAIOAuthRateLimitBucket(&buckets, &reset, now)
	}
	require.Equal(t, 8, buckets.Total)
	require.Equal(t, 1, buckets.UpTo10Minutes)
	require.Equal(t, 1, buckets.From10To30)
	require.Equal(t, 1, buckets.From30To1Hour)
	require.Equal(t, 1, buckets.From1To3Hours)
	require.Equal(t, 1, buckets.From3To5Hours)
	require.Equal(t, 1, buckets.From5HoursTo1Day)
	require.Equal(t, 1, buckets.From1To3Days)
	require.Equal(t, 1, buckets.Over3Days)
}

func TestOpenAIOAuthCapacityBaselinesStaySeparatedByPlan(t *testing.T) {
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	reset := now.Add(time.Hour)
	updated := now.Add(-time.Minute)
	k12Percent, proPercent := 50.0, 25.0
	accounts := []*openAIOAuthAccountCapacity{
		{row: OpenAIOAuthCapacityRawRow{PlanType: "k12", Status: StatusActive, Schedulable: true, FiveHourAccountSpend: 8, FiveHourUsedPercent: &k12Percent, FiveHourResetAt: &reset, UsageUpdatedAt: &updated}},
		{row: OpenAIOAuthCapacityRawRow{PlanType: "pro", Status: StatusActive, Schedulable: true, FiveHourAccountSpend: 100, FiveHourUsedPercent: &proPercent, FiveHourResetAt: &reset, UsageUpdatedAt: &updated}},
	}
	baselines := buildOpenAIOAuthCapacityBaselines(accounts, nil, now)
	require.InDelta(t, 16, baselines[openAIOAuthBaselineKey("k12", "5h")].value, 0.0001)
	require.InDelta(t, 400, baselines[openAIOAuthBaselineKey("pro", "5h")].value, 0.0001)
}

func TestOpenAIOAuthGroupShareDoesNotDuplicateSharedCapacity(t *testing.T) {
	require.InDelta(t, 0.75, openAIOAuthGroupShare(75, 100, 2), 0.0001)
	require.InDelta(t, 0.5, openAIOAuthGroupShare(0, 0, 2), 0.0001)
}

func TestNormalizeOpenAIOAuthPlanTypeKeepsBusinessWithTeam(t *testing.T) {
	require.Equal(t, "team", normalizeOpenAIOAuthPlanType("business"))
	require.Equal(t, "team", normalizeOpenAIOAuthPlanType("chatgpt_team"))
}

func TestOpenAIOAuthPlanCountsIncludeEveryUniqueAccount(t *testing.T) {
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	scope := newOpenAIOAuthScopeAccumulator(nil, "All groups")

	accounts := []*openAIOAuthAccountCapacity{
		{row: OpenAIOAuthCapacityRawRow{AccountID: 1, PlanType: "pro", Status: StatusActive, Schedulable: true}},
		{row: OpenAIOAuthCapacityRawRow{AccountID: 2, PlanType: "pro", Status: StatusActive, Schedulable: true}},
		{row: OpenAIOAuthCapacityRawRow{AccountID: 3, PlanType: "pro", Status: StatusError, Schedulable: false}},
		{row: OpenAIOAuthCapacityRawRow{AccountID: 4, PlanType: "plus", Status: StatusActive, Schedulable: true}},
	}
	for _, account := range accounts {
		addOpenAIOAuthAccountToScope(scope, account, nil, now)
		// Re-adding the same account (e.g. multi-group path) must not double-count.
		addOpenAIOAuthAccountToScope(scope, account, nil, now)
	}

	require.Equal(t, 4, scope.accountCount.Total)
	require.Equal(t, 3, scope.accountCount.Schedulable)
	require.Equal(t, 1, scope.accountCount.Errors)

	pro := scope.plans["pro"]
	require.NotNil(t, pro)
	require.Equal(t, 3, pro.count.Total)
	require.Equal(t, 2, pro.count.Schedulable)
	require.Equal(t, 1, pro.count.Errors)

	plus := scope.plans["plus"]
	require.NotNil(t, plus)
	require.Equal(t, 1, plus.count.Total)
	require.Equal(t, 1, plus.count.Schedulable)

	finalized := finalizeOpenAIOAuthScope(scope, nil)
	var proPlan *OpenAIOAuthPlanCount
	for i := range finalized.PlanCounts {
		if finalized.PlanCounts[i].PlanType == "pro" {
			proPlan = &finalized.PlanCounts[i]
			break
		}
	}
	require.NotNil(t, proPlan)
	require.Equal(t, 3, proPlan.Total)
	require.Equal(t, 2, proPlan.Schedulable)
}

func TestOpenAIOAuthRecommendationsAreAlternativeAndOmitZeroCounts(t *testing.T) {
	// Shortfall present: recommendations are mutually exclusive alternatives with positive counts.
	accWithShortfall := newOpenAIOAuthScopeAccumulator(nil, "All groups")
	accWithShortfall.accountCount.Schedulable = 1
	accWithShortfall.windows["5h"] = &openAIOAuthWindowAccumulator{
		currentSpend: 100, capacity: 50, forecastRemaining: 50,
		measuredAccounts: 1, capacityAccounts: 1,
	}
	accWithShortfall.windows["7d"] = &openAIOAuthWindowAccumulator{
		currentSpend: 200, capacity: 100, forecastRemaining: 100,
		measuredAccounts: 1, capacityAccounts: 1,
	}
	baselines := map[string]openAIOAuthBaselineValue{
		openAIOAuthBaselineKey("pro", "5h"): {value: 50, samples: 4, source: "completed_periods"},
		openAIOAuthBaselineKey("pro", "7d"): {value: 100, samples: 4, source: "completed_periods"},
		openAIOAuthBaselineKey("plus", "5h"): {value: 25, samples: 2, source: "current_cohort"},
	}
	scope := finalizeOpenAIOAuthScope(accWithShortfall, baselines)
	require.Greater(t, len(scope.Recommendations), 0)
	for _, rec := range scope.Recommendations {
		require.Equal(t, "alternative", rec.Mode)
		if rec.FiveHourAccounts != nil {
			require.Greater(t, *rec.FiveHourAccounts, 0)
		}
		if rec.SevenDayAccounts != nil {
			require.Greater(t, *rec.SevenDayAccounts, 0)
		}
		require.True(t, rec.FiveHourAccounts != nil || rec.SevenDayAccounts != nil)
	}
	// pro 5h shortfall = max((100+50)-50,0)=100 → ceil(100/50)=2
	// pro 7d shortfall = max((200+100)-100,0)=200 → ceil(200/100)=2
	var proRec *OpenAIOAuthCapacityRecommendation
	for i := range scope.Recommendations {
		if scope.Recommendations[i].PlanType == "pro" {
			proRec = &scope.Recommendations[i]
			break
		}
	}
	require.NotNil(t, proRec)
	require.NotNil(t, proRec.FiveHourAccounts)
	require.Equal(t, 2, *proRec.FiveHourAccounts)
	require.NotNil(t, proRec.SevenDayAccounts)
	require.Equal(t, 2, *proRec.SevenDayAccounts)

	// No shortfall: do not emit +0 account recommendations.
	accNoShortfall := newOpenAIOAuthScopeAccumulator(nil, "All groups")
	accNoShortfall.accountCount.Schedulable = 2
	accNoShortfall.windows["5h"] = &openAIOAuthWindowAccumulator{
		currentSpend: 10, capacity: 100, forecastRemaining: 5,
		measuredAccounts: 2, capacityAccounts: 2,
	}
	accNoShortfall.windows["7d"] = &openAIOAuthWindowAccumulator{
		currentSpend: 20, capacity: 400, forecastRemaining: 10,
		measuredAccounts: 2, capacityAccounts: 2,
	}
	noShortfall := finalizeOpenAIOAuthScope(accNoShortfall, baselines)
	require.Empty(t, noShortfall.Recommendations)
	require.NotNil(t, noShortfall.Windows[0].ProjectedShortfallUSD)
	require.NotNil(t, noShortfall.Windows[1].ProjectedShortfallUSD)
	require.InDelta(t, 0, *noShortfall.Windows[0].ProjectedShortfallUSD, 0.0001)
	require.InDelta(t, 0, *noShortfall.Windows[1].ProjectedShortfallUSD, 0.0001)
}
