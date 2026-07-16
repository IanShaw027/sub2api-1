package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	openAIOAuthRecentForecastWeight      = 0.65
	openAIOAuthPreviousDayForecastWeight = 0.35
	openAIOAuthMinimumInferencePercent   = 5.0
)

var openAIOAuthKnownPlanTypes = []string{"k12", "plus", "team", "pro"}

var openAIOAuthCapacityOverviewCache = struct {
	sync.RWMutex
	value     *OpenAIOAuthCapacityOverview
	updatedAt time.Time
}{}

var openAIOAuthCapacityOverviewFlight singleflight.Group

// OpenAIOAuthCapacityRawRow is the repository projection used by the capacity planner.
// Costs use the account-statistics USD pricing convention, not user billing cost.
type OpenAIOAuthCapacityRawRow struct {
	AccountID                 int64
	GroupID                   int64
	GroupName                 string
	PlanType                  string
	Status                    string
	Schedulable               bool
	RateLimitResetAt          *time.Time
	UsageUpdatedAt            *time.Time
	FiveHourUsedPercent       *float64
	FiveHourResetAt           *time.Time
	FiveHourWindowMinutes     int
	SevenDayUsedPercent       *float64
	SevenDayResetAt           *time.Time
	SevenDayWindowMinutes     int
	FiveHourAccountSpend      float64
	FiveHourGroupSpend        float64
	SevenDayAccountSpend      float64
	SevenDayGroupSpend        float64
	RecentThreeHourSpend      float64
	PreviousDayThreeHourSpend float64
}

type OpenAIOAuthCapacityBaselineRaw struct {
	PlanType          string
	Window            string
	MedianCapacityUSD float64
	Samples           int
}

type OpenAIOAuthCapacityRawData struct {
	Rows      []OpenAIOAuthCapacityRawRow
	Baselines []OpenAIOAuthCapacityBaselineRaw
}

type openAIOAuthCapacityRepository interface {
	GetOpenAIOAuthCapacityRawData(ctx context.Context, now time.Time) (*OpenAIOAuthCapacityRawData, error)
}

type OpenAIOAuthCapacityMethod struct {
	Currency                string  `json:"currency"`
	RecentHours             int     `json:"recent_hours"`
	RecentWeight            float64 `json:"recent_weight"`
	PreviousDayWeight       float64 `json:"previous_day_weight"`
	MinimumInferencePercent float64 `json:"minimum_inference_percent"`
	GroupAllocation         string  `json:"group_allocation"`
}

type OpenAIOAuthPlanCount struct {
	PlanType    string `json:"plan_type"`
	Total       int    `json:"total"`
	Schedulable int    `json:"schedulable"`
	Errors      int    `json:"errors"`
	RateLimited int    `json:"rate_limited"`
}

type OpenAIOAuthRateLimitBuckets struct {
	Total            int `json:"total"`
	UpTo10Minutes    int `json:"up_to_10m"`
	From10To30       int `json:"from_10m_to_30m"`
	From30To1Hour    int `json:"from_30m_to_1h"`
	From1To3Hours    int `json:"from_1h_to_3h"`
	From3To5Hours    int `json:"from_3h_to_5h"`
	From5HoursTo1Day int `json:"from_5h_to_1d"`
	From1To3Days     int `json:"from_1d_to_3d"`
	Over3Days        int `json:"over_3d"`
}

type OpenAIOAuthCapacityWindow struct {
	Window                 string   `json:"window"`
	CurrentSpendUSD        float64  `json:"current_spend_usd"`
	// Capacity/available/used/shortfall are nil when capacity could not be measured.
	EstimatedCapacityUSD   *float64 `json:"estimated_capacity_usd"`
	AvailableUSD           *float64 `json:"available_usd"`
	EstimatedUsedPercent   *float64 `json:"estimated_used_percent"`
	ForecastRemainingUSD   float64  `json:"forecast_remaining_usd"`
	ProjectedCycleSpendUSD float64  `json:"projected_cycle_spend_usd"`
	ProjectedShortfallUSD  *float64 `json:"projected_shortfall_usd"`
	MeasuredAccounts       int      `json:"measured_accounts"`
	CapacityAccounts       int      `json:"capacity_accounts"`
	Confidence             string   `json:"confidence"`
}

type OpenAIOAuthForecastInputs struct {
	RecentThreeHourUSD       float64 `json:"recent_three_hour_usd"`
	PreviousDaySamePeriodUSD float64 `json:"previous_day_same_period_usd"`
	BlendedHourlyRateUSD     float64 `json:"blended_hourly_rate_usd"`
}

type OpenAIOAuthPlanCapacity struct {
	PlanType string                      `json:"plan_type"`
	Accounts OpenAIOAuthPlanCount        `json:"accounts"`
	Windows  []OpenAIOAuthCapacityWindow `json:"windows"`
}

type OpenAIOAuthCapacityRecommendation struct {
	PlanType         string   `json:"plan_type"`
	Mode             string   `json:"mode,omitempty"` // "alternative" = mutually exclusive plan paths
	FiveHourAccounts *int     `json:"five_hour_accounts,omitempty"`
	SevenDayAccounts *int     `json:"seven_day_accounts,omitempty"`
	FiveHourUnitUSD  *float64 `json:"five_hour_unit_usd,omitempty"`
	SevenDayUnitUSD  *float64 `json:"seven_day_unit_usd,omitempty"`
	BaselineSource   string   `json:"baseline_source"`
	BaselineSamples  int      `json:"baseline_samples"`
}

type OpenAIOAuthCapacityScope struct {
	GroupID         *int64                              `json:"group_id,omitempty"`
	GroupName       string                              `json:"group_name"`
	Accounts        OpenAIOAuthPlanCount                `json:"accounts"`
	PlanCounts      []OpenAIOAuthPlanCount              `json:"plan_counts"`
	RateLimits      OpenAIOAuthRateLimitBuckets         `json:"rate_limits"`
	Forecast        OpenAIOAuthForecastInputs           `json:"forecast"`
	Windows         []OpenAIOAuthCapacityWindow         `json:"windows"`
	Plans           []OpenAIOAuthPlanCapacity           `json:"plans"`
	Recommendations []OpenAIOAuthCapacityRecommendation `json:"recommendations"`
}

type OpenAIOAuthCapacityBaseline struct {
	PlanType          string  `json:"plan_type"`
	Window            string  `json:"window"`
	MedianCapacityUSD float64 `json:"median_capacity_usd"`
	Samples           int     `json:"samples"`
	Source            string  `json:"source"`
}

type OpenAIOAuthCapacityOverview struct {
	GeneratedAt time.Time                     `json:"generated_at"`
	Method      OpenAIOAuthCapacityMethod     `json:"method"`
	Total       OpenAIOAuthCapacityScope      `json:"total"`
	Groups      []OpenAIOAuthCapacityScope    `json:"groups"`
	Baselines   []OpenAIOAuthCapacityBaseline `json:"baselines"`
}

type openAIOAuthAccountCapacity struct {
	row              OpenAIOAuthCapacityRawRow
	groupRows        []OpenAIOAuthCapacityRawRow
	fiveHourCap      float64
	sevenDayCap      float64
	fiveHourMeasured bool
	sevenDayMeasured bool
}

type openAIOAuthWindowAccumulator struct {
	currentSpend      float64
	capacity          float64
	forecastRemaining float64
	measuredAccounts  int
	capacityAccounts  int
}

type openAIOAuthScopeAccumulator struct {
	groupID       *int64
	groupName     string
	accounts      map[int64]struct{}
	accountCount  OpenAIOAuthPlanCount
	plans         map[string]*openAIOAuthPlanAccumulator
	rateLimits    OpenAIOAuthRateLimitBuckets
	recentSpend   float64
	previousSpend float64
	windows       map[string]*openAIOAuthWindowAccumulator
}

type openAIOAuthPlanAccumulator struct {
	count   OpenAIOAuthPlanCount
	windows map[string]*openAIOAuthWindowAccumulator
}

type openAIOAuthBaselineValue struct {
	value   float64
	samples int
	source  string
}

func (s *AccountUsageService) GetOpenAIOAuthCapacity(ctx context.Context, force ...bool) (*OpenAIOAuthCapacityOverview, error) {
	if s == nil || s.usageLogRepo == nil {
		return nil, fmt.Errorf("account usage service is not configured")
	}
	forced := len(force) > 0 && force[0]
	if !forced {
		openAIOAuthCapacityOverviewCache.RLock()
		cached, updatedAt := openAIOAuthCapacityOverviewCache.value, openAIOAuthCapacityOverviewCache.updatedAt
		openAIOAuthCapacityOverviewCache.RUnlock()
		if cached != nil && time.Since(updatedAt) < time.Minute {
			return cached, nil
		}
	}

	// Force uses a distinct singleflight key so it does not wait on a non-force rebuild
	// and return that shared (potentially stale) result. Both paths still update the same cache.
	// Note: force is best-effort under multi-replica (in-process cache only).
	flightKey := "overview"
	if forced {
		flightKey = "overview:force"
	}
	value, err, _ := openAIOAuthCapacityOverviewFlight.Do(flightKey, func() (any, error) {
		overview, buildErr := s.buildOpenAIOAuthCapacity(ctx)
		if buildErr != nil {
			return nil, buildErr
		}
		openAIOAuthCapacityOverviewCache.Lock()
		openAIOAuthCapacityOverviewCache.value = overview
		openAIOAuthCapacityOverviewCache.updatedAt = time.Now()
		openAIOAuthCapacityOverviewCache.Unlock()
		return overview, nil
	})
	if err != nil {
		return nil, err
	}
	return value.(*OpenAIOAuthCapacityOverview), nil
}

func (s *AccountUsageService) buildOpenAIOAuthCapacity(ctx context.Context) (*OpenAIOAuthCapacityOverview, error) {
	repo, ok := s.usageLogRepo.(openAIOAuthCapacityRepository)
	if !ok {
		return nil, fmt.Errorf("OpenAI OAuth capacity repository is not configured")
	}

	now := time.Now().UTC()
	raw, err := repo.GetOpenAIOAuthCapacityRawData(ctx, now)
	if err != nil {
		return nil, fmt.Errorf("get OpenAI OAuth capacity data: %w", err)
	}
	if raw == nil {
		raw = &OpenAIOAuthCapacityRawData{}
	}

	accounts := groupOpenAIOAuthCapacityRows(raw.Rows)
	baselines := buildOpenAIOAuthCapacityBaselines(accounts, raw.Baselines, now)
	total := newOpenAIOAuthScopeAccumulator(nil, "All groups")
	groups := make(map[int64]*openAIOAuthScopeAccumulator)

	for _, account := range accounts {
		addOpenAIOAuthAccountToScope(total, account, nil, now)
		for i := range account.groupRows {
			row := &account.groupRows[i]
			acc := groups[row.GroupID]
			if acc == nil {
				groupID := row.GroupID
				acc = newOpenAIOAuthScopeAccumulator(&groupID, row.GroupName)
				groups[row.GroupID] = acc
			}
			addOpenAIOAuthAccountToScope(acc, account, row, now)
		}
	}

	baselineList := make([]OpenAIOAuthCapacityBaseline, 0, len(baselines))
	for key, item := range baselines {
		parts := strings.SplitN(key, "\x00", 2)
		baselineList = append(baselineList, OpenAIOAuthCapacityBaseline{
			PlanType: parts[0], Window: parts[1], MedianCapacityUSD: roundUSD(item.value),
			Samples: item.samples, Source: item.source,
		})
	}
	sort.Slice(baselineList, func(i, j int) bool {
		if baselineList[i].PlanType == baselineList[j].PlanType {
			return baselineList[i].Window < baselineList[j].Window
		}
		return baselineList[i].PlanType < baselineList[j].PlanType
	})

	groupScopes := make([]OpenAIOAuthCapacityScope, 0, len(groups))
	for _, acc := range groups {
		groupScopes = append(groupScopes, finalizeOpenAIOAuthScope(acc, baselines))
	}
	sort.Slice(groupScopes, func(i, j int) bool {
		if groupScopes[i].Accounts.Total == groupScopes[j].Accounts.Total {
			return groupScopes[i].GroupName < groupScopes[j].GroupName
		}
		return groupScopes[i].Accounts.Total > groupScopes[j].Accounts.Total
	})

	return &OpenAIOAuthCapacityOverview{
		GeneratedAt: now,
		Method: OpenAIOAuthCapacityMethod{
			Currency: "USD", RecentHours: 3,
			RecentWeight:            openAIOAuthRecentForecastWeight,
			PreviousDayWeight:       openAIOAuthPreviousDayForecastWeight,
			MinimumInferencePercent: openAIOAuthMinimumInferencePercent,
			GroupAllocation:         "actual-window-spend-share; equal-share when no spend",
		},
		Total:     finalizeOpenAIOAuthScope(total, baselines),
		Groups:    groupScopes,
		Baselines: baselineList,
	}, nil
}

func groupOpenAIOAuthCapacityRows(rows []OpenAIOAuthCapacityRawRow) []*openAIOAuthAccountCapacity {
	byID := make(map[int64]*openAIOAuthAccountCapacity)
	order := make([]int64, 0)
	for _, row := range rows {
		row.PlanType = normalizeOpenAIOAuthPlanType(row.PlanType)
		row.GroupName = strings.TrimSpace(row.GroupName)
		account := byID[row.AccountID]
		if account == nil {
			account = &openAIOAuthAccountCapacity{row: row}
			byID[row.AccountID] = account
			order = append(order, row.AccountID)
		}
		account.groupRows = append(account.groupRows, row)
	}
	result := make([]*openAIOAuthAccountCapacity, 0, len(order))
	for _, id := range order {
		result = append(result, byID[id])
	}
	return result
}

func buildOpenAIOAuthCapacityBaselines(accounts []*openAIOAuthAccountCapacity, historical []OpenAIOAuthCapacityBaselineRaw, now time.Time) map[string]openAIOAuthBaselineValue {
	baselines := make(map[string]openAIOAuthBaselineValue)
	for _, raw := range historical {
		if raw.MedianCapacityUSD <= 0 || raw.Samples <= 0 {
			continue
		}
		key := openAIOAuthBaselineKey(raw.PlanType, raw.Window)
		baselines[key] = openAIOAuthBaselineValue{value: raw.MedianCapacityUSD, samples: raw.Samples, source: "completed_periods"}
	}

	current := make(map[string][]float64)
	for _, account := range accounts {
		row := &account.row
		account.fiveHourCap, account.fiveHourMeasured = inferOpenAIOAuthCapacity(row.FiveHourAccountSpend, row.FiveHourUsedPercent, row.FiveHourResetAt, row.UsageUpdatedAt, now)
		account.sevenDayCap, account.sevenDayMeasured = inferOpenAIOAuthCapacity(row.SevenDayAccountSpend, row.SevenDayUsedPercent, row.SevenDayResetAt, row.UsageUpdatedAt, now)
		eligible := row.Status == StatusActive && row.Schedulable
		if eligible && account.fiveHourMeasured {
			current[openAIOAuthBaselineKey(row.PlanType, "5h")] = append(current[openAIOAuthBaselineKey(row.PlanType, "5h")], account.fiveHourCap)
		}
		if eligible && account.sevenDayMeasured {
			current[openAIOAuthBaselineKey(row.PlanType, "7d")] = append(current[openAIOAuthBaselineKey(row.PlanType, "7d")], account.sevenDayCap)
		}
	}
	for key, values := range current {
		if _, exists := baselines[key]; exists {
			continue
		}
		if median := medianPositive(values); median > 0 {
			baselines[key] = openAIOAuthBaselineValue{value: median, samples: len(values), source: "current_cohort"}
		}
	}

	for _, account := range accounts {
		if !account.fiveHourMeasured {
			if baseline := baselines[openAIOAuthBaselineKey(account.row.PlanType, "5h")]; baseline.value > 0 {
				account.fiveHourCap = baseline.value
			}
		}
		if !account.sevenDayMeasured {
			if baseline := baselines[openAIOAuthBaselineKey(account.row.PlanType, "7d")]; baseline.value > 0 {
				account.sevenDayCap = baseline.value
			}
		}
	}
	return baselines
}

func inferOpenAIOAuthCapacity(spend float64, usedPercent *float64, resetAt, updatedAt *time.Time, now time.Time) (float64, bool) {
	if spend <= 0 || usedPercent == nil || *usedPercent < openAIOAuthMinimumInferencePercent || *usedPercent > 150 || resetAt == nil || !resetAt.After(now) {
		return 0, false
	}
	if updatedAt != nil && now.Sub(*updatedAt) > 6*time.Hour {
		return 0, false
	}
	return spend * 100 / *usedPercent, true
}

func newOpenAIOAuthScopeAccumulator(groupID *int64, name string) *openAIOAuthScopeAccumulator {
	return &openAIOAuthScopeAccumulator{
		groupID: groupID, groupName: name, accounts: make(map[int64]struct{}),
		plans:   make(map[string]*openAIOAuthPlanAccumulator),
		windows: map[string]*openAIOAuthWindowAccumulator{"5h": {}, "7d": {}},
	}
}

func addOpenAIOAuthAccountToScope(scope *openAIOAuthScopeAccumulator, account *openAIOAuthAccountCapacity, groupRow *OpenAIOAuthCapacityRawRow, now time.Time) {
	row := &account.row
	plan := scope.plans[row.PlanType]
	if plan == nil {
		plan = &openAIOAuthPlanAccumulator{count: OpenAIOAuthPlanCount{PlanType: row.PlanType}, windows: map[string]*openAIOAuthWindowAccumulator{"5h": {}, "7d": {}}}
		scope.plans[row.PlanType] = plan
	}
	// Deduplicate per account within the scope (total or single group). Each unique
	// account contributes once to total and plan counts; windows/spend still accumulate
	// on every call for group-scoped share allocation.
	if _, exists := scope.accounts[row.AccountID]; !exists {
		scope.accounts[row.AccountID] = struct{}{}
		incrementOpenAIOAuthCount(&scope.accountCount, row, now)
		incrementOpenAIOAuthCount(&plan.count, row, now)
		// Rate-limit buckets are per-account; only count the first sighting in this scope.
		addOpenAIOAuthRateLimitBucket(&scope.rateLimits, row.RateLimitResetAt, now)
	}

	membershipCount := len(account.groupRows)
	if membershipCount == 0 {
		membershipCount = 1
	}
	var spend5, spend7, recent, previous float64
	share5, share7 := 1.0, 1.0
	if groupRow != nil {
		spend5, spend7 = groupRow.FiveHourGroupSpend, groupRow.SevenDayGroupSpend
		recent, previous = groupRow.RecentThreeHourSpend, groupRow.PreviousDayThreeHourSpend
		share5 = openAIOAuthGroupShare(spend5, row.FiveHourAccountSpend, membershipCount)
		share7 = openAIOAuthGroupShare(spend7, row.SevenDayAccountSpend, membershipCount)
	} else {
		spend5, spend7 = row.FiveHourAccountSpend, row.SevenDayAccountSpend
		for _, item := range account.groupRows {
			recent += item.RecentThreeHourSpend
			previous += item.PreviousDayThreeHourSpend
		}
	}
	scope.recentSpend += recent
	scope.previousSpend += previous

	rate := openAIOAuthForecastHourlyRate(recent, previous)
	operational := row.Status == StatusActive && row.Schedulable
	addOpenAIOAuthWindow(scope.windows["5h"], spend5, account.fiveHourCap*share5, account.fiveHourMeasured, operational, rate, row.FiveHourResetAt, row.FiveHourWindowMinutes, now)
	addOpenAIOAuthWindow(scope.windows["7d"], spend7, account.sevenDayCap*share7, account.sevenDayMeasured, operational, rate, row.SevenDayResetAt, row.SevenDayWindowMinutes, now)
	addOpenAIOAuthWindow(plan.windows["5h"], spend5, account.fiveHourCap*share5, account.fiveHourMeasured, operational, rate, row.FiveHourResetAt, row.FiveHourWindowMinutes, now)
	addOpenAIOAuthWindow(plan.windows["7d"], spend7, account.sevenDayCap*share7, account.sevenDayMeasured, operational, rate, row.SevenDayResetAt, row.SevenDayWindowMinutes, now)
}

func incrementOpenAIOAuthCount(count *OpenAIOAuthPlanCount, row *OpenAIOAuthCapacityRawRow, now time.Time) {
	count.Total++
	if row.Status == StatusActive && row.Schedulable {
		count.Schedulable++
	}
	if row.Status == StatusError {
		count.Errors++
	}
	if row.RateLimitResetAt != nil && row.RateLimitResetAt.After(now) {
		count.RateLimited++
	}
}

func addOpenAIOAuthWindow(acc *openAIOAuthWindowAccumulator, spend, capacity float64, measured, operational bool, hourlyRate float64, resetAt *time.Time, windowMinutes int, now time.Time) {
	acc.currentSpend += spend
	if operational && capacity > 0 {
		acc.capacity += capacity
		acc.capacityAccounts++
		if measured {
			acc.measuredAccounts++
		}
	}
	remainingHours := openAIOAuthRemainingWindowHours(resetAt, windowMinutes, now)
	acc.forecastRemaining += hourlyRate * remainingHours
}

func openAIOAuthRemainingWindowHours(resetAt *time.Time, windowMinutes int, now time.Time) float64 {
	if resetAt == nil || !resetAt.After(now) {
		return 0
	}
	remaining := resetAt.Sub(now).Hours()
	if windowMinutes > 0 {
		remaining = math.Min(remaining, float64(windowMinutes)/60)
	}
	return math.Max(remaining, 0)
}

func openAIOAuthGroupShare(groupSpend, accountSpend float64, membershipCount int) float64 {
	if accountSpend > 0 {
		return math.Max(0, math.Min(1, groupSpend/accountSpend))
	}
	if membershipCount <= 0 {
		return 1
	}
	return 1 / float64(membershipCount)
}

func openAIOAuthForecastHourlyRate(recentThreeHour, previousDayThreeHour float64) float64 {
	recentRate := recentThreeHour / 3
	previousRate := previousDayThreeHour / 3
	switch {
	case recentThreeHour > 0 && previousDayThreeHour > 0:
		return recentRate*openAIOAuthRecentForecastWeight + previousRate*openAIOAuthPreviousDayForecastWeight
	case recentThreeHour > 0:
		return recentRate
	case previousDayThreeHour > 0:
		return previousRate
	default:
		return 0
	}
}

func addOpenAIOAuthRateLimitBucket(buckets *OpenAIOAuthRateLimitBuckets, resetAt *time.Time, now time.Time) {
	if resetAt == nil || !resetAt.After(now) {
		return
	}
	buckets.Total++
	remaining := resetAt.Sub(now)
	switch {
	case remaining <= 10*time.Minute:
		buckets.UpTo10Minutes++
	case remaining <= 30*time.Minute:
		buckets.From10To30++
	case remaining <= time.Hour:
		buckets.From30To1Hour++
	case remaining <= 3*time.Hour:
		buckets.From1To3Hours++
	case remaining <= 5*time.Hour:
		buckets.From3To5Hours++
	case remaining <= 24*time.Hour:
		buckets.From5HoursTo1Day++
	case remaining <= 72*time.Hour:
		buckets.From1To3Days++
	default:
		buckets.Over3Days++
	}
}

func finalizeOpenAIOAuthScope(acc *openAIOAuthScopeAccumulator, baselines map[string]openAIOAuthBaselineValue) OpenAIOAuthCapacityScope {
	windows := []OpenAIOAuthCapacityWindow{
		finalizeOpenAIOAuthWindow("5h", acc.windows["5h"], acc.accountCount.Schedulable, acc.recentSpend, acc.previousSpend),
		finalizeOpenAIOAuthWindow("7d", acc.windows["7d"], acc.accountCount.Schedulable, acc.recentSpend, acc.previousSpend),
	}
	plans := make([]OpenAIOAuthPlanCapacity, 0, len(acc.plans))
	planCounts := make([]OpenAIOAuthPlanCount, 0, len(acc.plans))
	planTypes := openAIOAuthScopePlanTypes(acc, baselines)
	for _, planType := range planTypes {
		plan := acc.plans[planType]
		if plan == nil {
			plan = &openAIOAuthPlanAccumulator{count: OpenAIOAuthPlanCount{PlanType: planType}, windows: map[string]*openAIOAuthWindowAccumulator{"5h": {}, "7d": {}}}
		}
		planCounts = append(planCounts, plan.count)
		plans = append(plans, OpenAIOAuthPlanCapacity{
			PlanType: plan.count.PlanType,
			Accounts: plan.count,
			Windows: []OpenAIOAuthCapacityWindow{
				finalizeOpenAIOAuthWindow("5h", plan.windows["5h"], plan.count.Schedulable, 0, 0),
				finalizeOpenAIOAuthWindow("7d", plan.windows["7d"], plan.count.Schedulable, 0, 0),
			},
		})
	}
	sort.Slice(planCounts, func(i, j int) bool { return planCounts[i].PlanType < planCounts[j].PlanType })
	sort.Slice(plans, func(i, j int) bool { return plans[i].PlanType < plans[j].PlanType })

	// Recommendations are mutually exclusive plan paths: each plan independently sizes
	// accounts against the full shortfall (Mode "alternative"). Clients should treat
	// them as alternatives, not sum them. Zero shortfall leaves account pointers nil
	// and omits the recommendation entirely when no window needs capacity.
	recommendations := make([]OpenAIOAuthCapacityRecommendation, 0)
	for _, planType := range planTypes {
		rec := OpenAIOAuthCapacityRecommendation{PlanType: planType, Mode: "alternative"}
		if base := baselines[openAIOAuthBaselineKey(planType, "5h")]; base.value > 0 && windows[0].ProjectedShortfallUSD != nil && *windows[0].ProjectedShortfallUSD > 0 {
			value := roundUSD(base.value)
			count := int(math.Ceil(*windows[0].ProjectedShortfallUSD / base.value))
			if count > 0 {
				rec.FiveHourUnitUSD, rec.FiveHourAccounts = &value, &count
				rec.BaselineSource, rec.BaselineSamples = base.source, base.samples
			}
		}
		if base := baselines[openAIOAuthBaselineKey(planType, "7d")]; base.value > 0 && windows[1].ProjectedShortfallUSD != nil && *windows[1].ProjectedShortfallUSD > 0 {
			value := roundUSD(base.value)
			count := int(math.Ceil(*windows[1].ProjectedShortfallUSD / base.value))
			if count > 0 {
				rec.SevenDayUnitUSD, rec.SevenDayAccounts = &value, &count
				if rec.BaselineSource == "" || base.source == "completed_periods" {
					rec.BaselineSource, rec.BaselineSamples = base.source, base.samples
				}
			}
		}
		if rec.FiveHourAccounts != nil || rec.SevenDayAccounts != nil {
			recommendations = append(recommendations, rec)
		}
	}
	sort.Slice(recommendations, func(i, j int) bool { return recommendations[i].PlanType < recommendations[j].PlanType })

	return OpenAIOAuthCapacityScope{
		GroupID: acc.groupID, GroupName: acc.groupName, Accounts: acc.accountCount,
		PlanCounts: planCounts, RateLimits: acc.rateLimits,
		Forecast: OpenAIOAuthForecastInputs{
			RecentThreeHourUSD:       roundUSD(acc.recentSpend),
			PreviousDaySamePeriodUSD: roundUSD(acc.previousSpend),
			BlendedHourlyRateUSD:     roundUSD(openAIOAuthForecastHourlyRate(acc.recentSpend, acc.previousSpend)),
		},
		Windows: windows, Plans: plans, Recommendations: recommendations,
	}
}

func finalizeOpenAIOAuthWindow(window string, acc *openAIOAuthWindowAccumulator, eligibleAccounts int, recent, previous float64) OpenAIOAuthCapacityWindow {
	projected := acc.currentSpend + acc.forecastRemaining
	shortfall := math.Max(projected-acc.capacity, 0)
	usedPercent := 0.0
	if acc.capacity > 0 {
		usedPercent = acc.currentSpend / acc.capacity * 100
	}
	confidence := "low"
	coverage := 0.0
	if eligibleAccounts > 0 {
		coverage = float64(acc.measuredAccounts) / float64(eligibleAccounts)
	}
	if coverage >= 0.8 && recent > 0 && previous > 0 {
		confidence = "high"
	} else if coverage >= 0.5 || (recent > 0 && previous > 0) {
		confidence = "medium"
	}
	var capacityUSD, availableUSD, usedOut, shortfallOut *float64
	if acc.capacityAccounts > 0 && acc.capacity > 0 {
		cap := roundUSD(acc.capacity)
		avail := roundUSD(math.Max(acc.capacity-acc.currentSpend, 0))
		used := math.Round(usedPercent*100) / 100
		sf := roundUSD(shortfall)
		capacityUSD, availableUSD, usedOut, shortfallOut = &cap, &avail, &used, &sf
	}
	return OpenAIOAuthCapacityWindow{
		Window:                 window,
		CurrentSpendUSD:        roundUSD(acc.currentSpend),
		EstimatedCapacityUSD:   capacityUSD,
		AvailableUSD:           availableUSD,
		EstimatedUsedPercent:   usedOut,
		ForecastRemainingUSD:   roundUSD(acc.forecastRemaining),
		ProjectedCycleSpendUSD: roundUSD(projected),
		ProjectedShortfallUSD:  shortfallOut,
		MeasuredAccounts:       acc.measuredAccounts,
		CapacityAccounts:       acc.capacityAccounts,
		Confidence:             confidence,
	}
}

func normalizeOpenAIOAuthPlanType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "":
		return "unknown"
	case "business", "chatgpt_business", "chatgpt_team":
		return "team"
	case "chatgpt_plus":
		return "plus"
	case "chatgpt_pro":
		return "pro"
	}
	return value
}

func openAIOAuthScopePlanTypes(acc *openAIOAuthScopeAccumulator, baselines map[string]openAIOAuthBaselineValue) []string {
	set := make(map[string]struct{}, len(openAIOAuthKnownPlanTypes)+len(acc.plans))
	for _, planType := range openAIOAuthKnownPlanTypes {
		set[planType] = struct{}{}
	}
	for planType := range acc.plans {
		set[planType] = struct{}{}
	}
	for key := range baselines {
		parts := strings.SplitN(key, "\x00", 2)
		if len(parts) == 2 {
			set[parts[0]] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for planType := range set {
		result = append(result, planType)
	}
	sort.Strings(result)
	return result
}

func openAIOAuthBaselineKey(planType, window string) string {
	return normalizeOpenAIOAuthPlanType(planType) + "\x00" + strings.ToLower(strings.TrimSpace(window))
}

func medianPositive(values []float64) float64 {
	filtered := make([]float64, 0, len(values))
	for _, value := range values {
		if value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0) {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return 0
	}
	sort.Float64s(filtered)
	middle := len(filtered) / 2
	if len(filtered)%2 == 1 {
		return filtered[middle]
	}
	return (filtered[middle-1] + filtered[middle]) / 2
}

func roundUSD(value float64) float64 {
	return math.Round(value*100) / 100
}
