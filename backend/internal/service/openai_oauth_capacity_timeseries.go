package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

	"golang.org/x/sync/singleflight"
)

const (
	openAIOAuthTimeseriesRecentWeight      = 0.50
	openAIOAuthTimeseriesPreviousDayWeight = 0.30
	openAIOAuthTimeseriesRPMWeight         = 0.20
	openAIOAuthTimeseriesRPMWindowMinutes  = 15
	openAIOAuthTimeseriesMaxPoints         = 24 * 8 // 7d history + 1d forecast headroom
	openAIOAuthTimeseriesCacheTTL          = time.Minute
)

// OpenAIOAuthCapacityAllGroupsID is reserved for deduplicated global hourly
// facts. group_id=0 remains the real ungrouped request-time bucket.
const OpenAIOAuthCapacityAllGroupsID int64 = -1

var openAIOAuthTimeseriesCache = struct {
	sync.RWMutex
	entries map[string]openAIOAuthTimeseriesCacheEntry
}{entries: make(map[string]openAIOAuthTimeseriesCacheEntry)}

var openAIOAuthTimeseriesFlight singleflight.Group

type openAIOAuthTimeseriesCacheEntry struct {
	value     *OpenAIOAuthCapacityTimeseries
	updatedAt time.Time
}

// OpenAIOAuthHourlySpendRow is the repository projection for hourly USD spend.
type OpenAIOAuthHourlySpendRow struct {
	BucketStart time.Time
	GroupID     int64
	SpendUSD    float64
}

// OpenAIOAuthCapacityHourlyFact is a persisted hourly capacity fact.
// Optional capacity/available fields are nil when they could not be measured.
type OpenAIOAuthCapacityHourlyFact struct {
	BucketStart    time.Time
	GroupID        int64
	PlanType       string
	SpentUSD       float64
	Capacity5hUSD  *float64
	Available5hUSD *float64
	UsedPercent5h  *float64
	Capacity7dUSD  *float64
	Available7dUSD *float64
	UsedPercent7d  *float64
	Sealed         bool
	Source         string
}

type openAIOAuthCapacityTimeseriesRepository interface {
	openAIOAuthCapacityRepository
	GetOpenAIOAuthCapacityHourlySpend(ctx context.Context, from, to time.Time) ([]OpenAIOAuthHourlySpendRow, error)
	GetOpenAIOAuthCapacityHourlyFacts(ctx context.Context, from, to time.Time, groupID *int64, sealedOnly bool) ([]OpenAIOAuthCapacityHourlyFact, error)
	UpsertOpenAIOAuthCapacityHourlyFacts(ctx context.Context, facts []OpenAIOAuthCapacityHourlyFact) error
	GetOpenAIOAuthCapacityRecentSpend(ctx context.Context, from, to time.Time, groupID *int64) (float64, error)
}

// OpenAIOAuthCapacityTimeseriesQuery is the request contract for the timeseries endpoint.
type OpenAIOAuthCapacityTimeseriesQuery struct {
	Range       string // 12h | 24h | 48h | 7d | custom
	From        *time.Time
	To          *time.Time
	Window      string // 5h | 7d | both
	GroupID     *int64
	PlanType    string // empty = all
	Force       bool
	PastHours   int // optional override
	FutureHours int
}

type OpenAIOAuthCapacityTimeseriesMethod struct {
	Currency                string  `json:"currency"`
	Window                  string  `json:"window"`
	RecentWeight            float64 `json:"recent_weight"`
	PreviousDayWeight       float64 `json:"previous_day_weight"`
	RPMWeight               float64 `json:"rpm_weight"`
	RPMWindowMinutes        int     `json:"rpm_window_minutes"`
	MinimumInferencePercent float64 `json:"minimum_inference_percent"`
	Curve                   string  `json:"curve"`
	GroupAllocation         string  `json:"group_allocation"`
	PastHours               int     `json:"past_hours"`
	FutureHours             int     `json:"future_hours"`
}

type OpenAIOAuthCapacityTimeseriesPoint struct {
	BucketStart time.Time `json:"bucket_start"`
	Segment     string    `json:"segment"` // past | current | future
	// SpentUSD is set only when the hour has actual spend data (including sealed 0).
	// nil means no durable fact yet and no raw logs for that hour — UI must omit the point value.
	SpentUSD *float64 `json:"spent_usd"`
	// ForecastUSD is set only for current/future when a non-zero forecast model can run.
	ForecastUSD *float64 `json:"forecast_usd"`
	// DisplaySpendUSD is the chart y-value when present; nil means "do not plot".
	DisplaySpendUSD *float64 `json:"display_spend_usd"`
	// Window metrics are nil when capacity could not be inferred — never invent zeros.
	AvailableUSD     *float64 `json:"available_usd"`
	CapacityUSD      *float64 `json:"capacity_usd"`
	UsedPercent      *float64 `json:"used_percent"`
	Available7dUSD   *float64 `json:"available_7d_usd,omitempty"`
	Capacity7dUSD    *float64 `json:"capacity_7d_usd,omitempty"`
	UsedPercent7d    *float64 `json:"used_percent_7d,omitempty"`
	ShortfallRiskUSD *float64 `json:"shortfall_risk_usd"`
	Sealed           bool     `json:"sealed"`
	Source           string   `json:"source,omitempty"`
}

type OpenAIOAuthCapacityTimeseriesSummary struct {
	SpentUSD float64 `json:"spent_usd"`
	// Available/Capacity/UsedPercent are nil when capacity could not be measured.
	AvailableUSD         *float64 `json:"available_usd"`
	CapacityUSD          *float64 `json:"capacity_usd"`
	UsedPercent          *float64 `json:"used_percent"`
	ForecastRemainingUSD float64  `json:"forecast_remaining_usd"`
	ProjectedCycleUSD    float64  `json:"projected_cycle_spend_usd"`
	// ProjectedShortfallUSD is nil when capacity is unmeasured (do not invent a zero gap).
	ProjectedShortfallUSD *float64 `json:"projected_shortfall_usd"`
	Confidence            string   `json:"confidence"`
	MeasuredAccounts      int      `json:"measured_accounts"`
	CapacityAccounts      int      `json:"capacity_accounts"`
}

type OpenAIOAuthCapacityTimeseriesForecast struct {
	RecentThreeHourUSD       float64 `json:"recent_three_hour_usd"`
	PreviousDaySamePeriodUSD float64 `json:"previous_day_same_period_usd"`
	RecentRPMWindowUSD       float64 `json:"recent_rpm_window_usd"`
	BlendedHourlyRateUSD     float64 `json:"blended_hourly_rate_usd"`
	RPMHourlyRateUSD         float64 `json:"rpm_hourly_rate_usd"`
}

type OpenAIOAuthCapacityTimeseries struct {
	GeneratedAt     time.Time                             `json:"generated_at"`
	Now             time.Time                             `json:"now"`
	Method          OpenAIOAuthCapacityTimeseriesMethod   `json:"method"`
	GroupID         *int64                                `json:"group_id,omitempty"`
	GroupName       string                                `json:"group_name"`
	Health          OpenAIOAuthPlanCount                  `json:"health"`
	RateLimits      OpenAIOAuthRateLimitBuckets           `json:"rate_limits"`
	PlanCounts      []OpenAIOAuthPlanCount                `json:"plan_counts"`
	Forecast        OpenAIOAuthCapacityTimeseriesForecast `json:"forecast"`
	Summary         OpenAIOAuthCapacityTimeseriesSummary  `json:"summary"`
	Windows         []OpenAIOAuthCapacityWindow           `json:"windows"`
	Points          []OpenAIOAuthCapacityTimeseriesPoint  `json:"points"`
	Recommendations []OpenAIOAuthCapacityRecommendation   `json:"recommendations"`
	Baselines       []OpenAIOAuthCapacityBaseline         `json:"baselines"`
}

func (s *AccountUsageService) GetOpenAIOAuthCapacityTimeseries(ctx context.Context, query OpenAIOAuthCapacityTimeseriesQuery) (*OpenAIOAuthCapacityTimeseries, error) {
	if s == nil || s.usageLogRepo == nil {
		return nil, fmt.Errorf("account usage service is not configured")
	}
	repo, ok := s.usageLogRepo.(openAIOAuthCapacityTimeseriesRepository)
	if !ok {
		return nil, fmt.Errorf("OpenAI OAuth capacity timeseries repository is not configured")
	}

	normalized, err := normalizeOpenAIOAuthTimeseriesQuery(query)
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_CAPACITY_TIMESERIES_QUERY", err.Error())
	}
	cacheKey := openAIOAuthTimeseriesCacheKey(normalized)
	if !normalized.Force {
		openAIOAuthTimeseriesCache.RLock()
		entry, exists := openAIOAuthTimeseriesCache.entries[cacheKey]
		openAIOAuthTimeseriesCache.RUnlock()
		if exists && time.Since(entry.updatedAt) < openAIOAuthTimeseriesCacheTTL {
			return entry.value, nil
		}
	}

	flightKey := cacheKey
	if normalized.Force {
		flightKey = "force:" + cacheKey
	}
	value, err, _ := openAIOAuthTimeseriesFlight.Do(flightKey, func() (any, error) {
		series, buildErr := s.buildOpenAIOAuthCapacityTimeseries(ctx, repo, normalized)
		if buildErr != nil {
			return nil, buildErr
		}
		openAIOAuthTimeseriesCache.Lock()
		openAIOAuthTimeseriesCache.entries[cacheKey] = openAIOAuthTimeseriesCacheEntry{
			value: series, updatedAt: time.Now(),
		}
		// Bound cache growth for multi-query shapes.
		if len(openAIOAuthTimeseriesCache.entries) > 64 {
			for key, item := range openAIOAuthTimeseriesCache.entries {
				if time.Since(item.updatedAt) > openAIOAuthTimeseriesCacheTTL {
					delete(openAIOAuthTimeseriesCache.entries, key)
				}
			}
		}
		openAIOAuthTimeseriesCache.Unlock()
		return series, nil
	})
	if err != nil {
		return nil, err
	}
	return value.(*OpenAIOAuthCapacityTimeseries), nil
}

func normalizeOpenAIOAuthTimeseriesQuery(query OpenAIOAuthCapacityTimeseriesQuery) (OpenAIOAuthCapacityTimeseriesQuery, error) {
	out := query
	out.Window = strings.ToLower(strings.TrimSpace(out.Window))
	if out.Window == "" {
		out.Window = "5h"
	}
	switch out.Window {
	case "5h", "7d", "both":
	default:
		return out, fmt.Errorf("invalid window %q (expected 5h, 7d, or both)", out.Window)
	}
	out.PlanType = normalizeOpenAIOAuthPlanType(out.PlanType)
	if out.PlanType == "unknown" && strings.TrimSpace(query.PlanType) == "" {
		out.PlanType = ""
	}

	rangeKey := strings.ToLower(strings.TrimSpace(out.Range))
	if rangeKey == "" {
		rangeKey = "24h"
	}
	out.Range = rangeKey

	switch rangeKey {
	case "12h":
		out.PastHours, out.FutureHours = 6, 6
	case "24h":
		out.PastHours, out.FutureHours = 12, 12
	case "48h":
		out.PastHours, out.FutureHours = 36, 12
	case "7d":
		out.PastHours, out.FutureHours = 24*6, 24
	case "custom":
		if out.From == nil || out.To == nil {
			return out, fmt.Errorf("custom range requires from and to")
		}
		if !out.To.After(*out.From) {
			return out, fmt.Errorf("to must be after from")
		}
		// Bound custom span so cold backfill cannot scan unbounded history.
		spanHours := int(math.Ceil(out.To.UTC().Sub(out.From.UTC()).Hours()))
		if spanHours < 1 {
			spanHours = 1
		}
		if spanHours > openAIOAuthTimeseriesMaxPoints {
			return out, fmt.Errorf("custom range spans %d hours; max is %d", spanHours, openAIOAuthTimeseriesMaxPoints)
		}
		// Past/future split is resolved against now in the builder; store span for the guard.
		out.PastHours, out.FutureHours = spanHours, 0
	default:
		return out, fmt.Errorf("invalid range %q (expected 12h, 24h, 48h, 7d, or custom)", out.Range)
	}
	if out.PastHours < 0 || out.FutureHours < 0 {
		return out, fmt.Errorf("past_hours and future_hours must be non-negative")
	}
	if out.PastHours+out.FutureHours > openAIOAuthTimeseriesMaxPoints {
		return out, fmt.Errorf("requested series exceeds %d hourly points", openAIOAuthTimeseriesMaxPoints)
	}
	// plan_type filter is not implemented end-to-end; ignore until scoped aggregation exists.
	out.PlanType = ""
	return out, nil
}

func openAIOAuthTimeseriesCacheKey(query OpenAIOAuthCapacityTimeseriesQuery) string {
	group := "total"
	if query.GroupID != nil {
		group = fmt.Sprintf("g:%d", *query.GroupID)
	}
	from, to := "", ""
	if query.From != nil {
		from = query.From.UTC().Format(time.RFC3339)
	}
	if query.To != nil {
		to = query.To.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%d|%d",
		query.Range, from, to, query.Window, group, query.PastHours, query.FutureHours)
}

func (s *AccountUsageService) buildOpenAIOAuthCapacityTimeseries(
	ctx context.Context,
	repo openAIOAuthCapacityTimeseriesRepository,
	query OpenAIOAuthCapacityTimeseriesQuery,
) (*OpenAIOAuthCapacityTimeseries, error) {
	now := time.Now().UTC().Truncate(time.Minute)
	overview, err := s.GetOpenAIOAuthCapacity(ctx, query.Force)
	if err != nil {
		return nil, err
	}

	scope := overview.Total
	if query.GroupID != nil {
		found := false
		for i := range overview.Groups {
			if overview.Groups[i].GroupID != nil && *overview.Groups[i].GroupID == *query.GroupID {
				scope = overview.Groups[i]
				found = true
				break
			}
		}
		if !found {
			return nil, infraerrors.BadRequest(
				"INVALID_CAPACITY_GROUP",
				fmt.Sprintf("group %d not found in OpenAI OAuth capacity overview", *query.GroupID),
			)
		}
	}

	pastHours, futureHours, rangeStart, rangeEnd := resolveOpenAIOAuthTimeseriesBounds(query, now)
	// Need previous-day shape + rolling window history for available reconstruction.
	historyFrom := rangeStart.Add(-7 * 24 * time.Hour)
	if historyFrom.After(now) {
		historyFrom = rangeStart.Add(-24 * time.Hour)
	}

	// Load sealed facts first; only unsealed/missing hours re-scan usage_logs and get persisted.
	facts, err := loadOpenAIOAuthHourlyFactsWithBackfill(ctx, repo, historyFrom, now, query.GroupID, query.Force)
	if err != nil {
		return nil, err
	}
	hourly := openAIOAuthHourlySpendFromFacts(facts)
	factByBucket := make(map[time.Time]OpenAIOAuthCapacityHourlyFact, len(facts))
	for _, fact := range facts {
		factByBucket[fact.BucketStart.UTC().Truncate(time.Hour)] = fact
	}

	rpmFrom := now.Add(-time.Duration(openAIOAuthTimeseriesRPMWindowMinutes) * time.Minute)
	rpmSpend, err := repo.GetOpenAIOAuthCapacityRecentSpend(ctx, rpmFrom, now, query.GroupID)
	if err != nil {
		return nil, fmt.Errorf("get OpenAI OAuth recent rpm spend: %w", err)
	}

	windowKey := query.Window
	if windowKey == "both" {
		windowKey = "5h"
	}
	primary := openAIOAuthWindowFromScope(scope, windowKey)
	secondary := openAIOAuthWindowFromScope(scope, "7d")
	// Only surface capacity/available when the overview actually measured them.
	primaryCapOK := openAIOAuthWindowCapacityOK(primary)
	secondaryCapOK := openAIOAuthWindowCapacityOK(secondary)

	recent3h := scope.Forecast.RecentThreeHourUSD
	prevDay3h := scope.Forecast.PreviousDaySamePeriodUSD
	rpmHourly := rpmSpend * (60.0 / float64(openAIOAuthTimeseriesRPMWindowMinutes))
	blended := openAIOAuthTimeseriesHourlyRate(recent3h, prevDay3h, rpmHourly)
	canForecast := blended > 0

	shape := openAIOAuthPreviousDayShape(hourly, now, futureHours)
	currentHourStart := now.Truncate(time.Hour)
	pointCapacity := int(rangeEnd.Sub(rangeStart).Hours())
	if pointCapacity < 0 {
		pointCapacity = 0
	}
	points := make([]OpenAIOAuthCapacityTimeseriesPoint, 0, pointCapacity)

	for bucketStart := rangeStart; bucketStart.Before(rangeEnd); bucketStart = bucketStart.Add(time.Hour) {
		bucketEnd := bucketStart.Add(time.Hour)
		point := OpenAIOAuthCapacityTimeseriesPoint{BucketStart: bucketStart}

		switch {
		case bucketEnd.Before(now) || bucketEnd.Equal(now):
			point.Segment = "past"
			if fact, ok := factByBucket[bucketStart]; ok {
				point.SpentUSD = floatPtr(roundUSD(fact.SpentUSD))
				point.DisplaySpendUSD = floatPtr(roundUSD(fact.SpentUSD))
				point.Sealed = fact.Sealed
				point.Source = fact.Source
				applySealedWindowMetrics(&point, fact, query.Window)
			} else if spent, ok := hourly[bucketStart]; ok {
				point.SpentUSD = floatPtr(roundUSD(spent))
				point.DisplaySpendUSD = floatPtr(roundUSD(spent))
				point.Source = "usage_logs"
			}
			// No inventing: if neither sealed fact nor raw spend exists, leave nils.

		case !bucketStart.After(now) && bucketEnd.After(now):
			point.Segment = "current"
			point.Source = "live"
			spentPartial, hasSpend := hourly[bucketStart]
			if hasSpend {
				point.SpentUSD = floatPtr(roundUSD(spentPartial))
			} else {
				spentPartial = 0
			}
			remaining := bucketEnd.Sub(now).Hours()
			if remaining < 0 {
				remaining = 0
			}
			if canForecast {
				shapeFactor := openAIOAuthShapeFactor(shape, bucketStart, blended)
				forecastPartial := blended * shapeFactor * remaining
				point.ForecastUSD = floatPtr(roundUSD(forecastPartial))
				display := spentPartial + forecastPartial
				point.DisplaySpendUSD = floatPtr(roundUSD(display))
				if primaryCapOK {
					primCap := openAIOAuthWindowCapacityUSD(primary)
					avail, used := openAIOAuthAvailableAt(
						primCap, hourly, now, windowDuration(windowKey), now,
						map[time.Time]float64{bucketStart: forecastPartial}, blended, shape,
					)
					point.CapacityUSD = floatPtr(roundUSD(primCap))
					point.AvailableUSD = floatPtr(avail)
					point.UsedPercent = floatPtr(used)
					if display > avail {
						risk := roundUSD(display - avail)
						point.ShortfallRiskUSD = &risk
					}
				}
				if query.Window == "both" && secondaryCapOK {
					secCap := openAIOAuthWindowCapacityUSD(secondary)
					avail7, used7 := openAIOAuthAvailableAt(
						secCap, hourly, now, windowDuration("7d"), now,
						map[time.Time]float64{bucketStart: forecastPartial}, blended, shape,
					)
					point.Available7dUSD = floatPtr(avail7)
					point.Capacity7dUSD = floatPtr(roundUSD(secCap))
					point.UsedPercent7d = floatPtr(used7)
				}
			} else if hasSpend {
				point.DisplaySpendUSD = floatPtr(roundUSD(spentPartial))
			}

		default:
			point.Segment = "future"
			point.Source = "forecast"
			if !canForecast {
				points = append(points, point)
				continue
			}
			shapeFactor := openAIOAuthShapeFactor(shape, bucketStart, blended)
			forecast := blended * shapeFactor
			point.ForecastUSD = floatPtr(roundUSD(forecast))
			point.DisplaySpendUSD = floatPtr(roundUSD(forecast))
			if primaryCapOK {
				primCap := openAIOAuthWindowCapacityUSD(primary)
				futureForecasts := openAIOAuthFutureForecastMap(hourly, currentHourStart, now, bucketEnd, blended, shape)
				avail, used := openAIOAuthAvailableAt(
					primCap, hourly, bucketEnd, windowDuration(windowKey), now, futureForecasts, blended, shape,
				)
				point.CapacityUSD = floatPtr(roundUSD(primCap))
				point.AvailableUSD = floatPtr(avail)
				point.UsedPercent = floatPtr(used)
				if forecast > avail {
					risk := roundUSD(forecast - avail)
					point.ShortfallRiskUSD = &risk
				}
			}
			if query.Window == "both" && secondaryCapOK {
				secCap := openAIOAuthWindowCapacityUSD(secondary)
				futureForecasts := openAIOAuthFutureForecastMap(hourly, currentHourStart, now, bucketEnd, blended, shape)
				avail7, used7 := openAIOAuthAvailableAt(
					secCap, hourly, bucketEnd, windowDuration("7d"), now, futureForecasts, blended, shape,
				)
				point.Available7dUSD = floatPtr(avail7)
				point.Capacity7dUSD = floatPtr(roundUSD(secCap))
				point.UsedPercent7d = floatPtr(used7)
			}
		}

		points = append(points, point)
	}

	// Summary: prefer live window snapshot from overview; enrich forecast remaining with timeseries blend.
	summaryWindow := primary
	forecastRemaining := 0.0
	for _, p := range points {
		if p.Segment == "future" && p.ForecastUSD != nil {
			forecastRemaining += *p.ForecastUSD
		}
		if p.Segment == "current" && p.ForecastUSD != nil {
			forecastRemaining += *p.ForecastUSD
		}
	}
	// Cap forecast remaining to window remaining semantics from overview when available.
	if summaryWindow.ForecastRemainingUSD > 0 {
		// Keep overview remaining as primary shortfall driver; timeseries forecast is for chart shape.
		forecastRemaining = summaryWindow.ForecastRemainingUSD
	}
	projected := summaryWindow.CurrentSpendUSD + forecastRemaining
	var (
		availPtr, capPtr, usedPtr, shortfallPtr *float64
	)
	if primaryCapOK && summaryWindow.EstimatedCapacityUSD != nil {
		capVal := *summaryWindow.EstimatedCapacityUSD
		if summaryWindow.AvailableUSD != nil {
			availPtr = floatPtr(*summaryWindow.AvailableUSD)
		} else {
			availPtr = floatPtr(roundUSD(math.Max(capVal-summaryWindow.CurrentSpendUSD, 0)))
		}
		capPtr = floatPtr(capVal)
		if summaryWindow.EstimatedUsedPercent != nil {
			usedPtr = floatPtr(*summaryWindow.EstimatedUsedPercent)
		}
		if summaryWindow.ProjectedShortfallUSD != nil {
			shortfallPtr = floatPtr(*summaryWindow.ProjectedShortfallUSD)
		} else {
			sf := roundUSD(math.Max(projected-capVal, 0))
			shortfallPtr = &sf
		}
	}

	return &OpenAIOAuthCapacityTimeseries{
		GeneratedAt: now,
		Now:         now,
		Method: OpenAIOAuthCapacityTimeseriesMethod{
			Currency:                "USD",
			Window:                  query.Window,
			RecentWeight:            openAIOAuthTimeseriesRecentWeight,
			PreviousDayWeight:       openAIOAuthTimeseriesPreviousDayWeight,
			RPMWeight:               openAIOAuthTimeseriesRPMWeight,
			RPMWindowMinutes:        openAIOAuthTimeseriesRPMWindowMinutes,
			MinimumInferencePercent: openAIOAuthMinimumInferencePercent,
			Curve:                   "seasonal_shape_v1",
			GroupAllocation:         overview.Method.GroupAllocation,
			PastHours:               pastHours,
			FutureHours:             futureHours,
		},
		GroupID:    scope.GroupID,
		GroupName:  scope.GroupName,
		Health:     scope.Accounts,
		RateLimits: scope.RateLimits,
		PlanCounts: scope.PlanCounts,
		Forecast: OpenAIOAuthCapacityTimeseriesForecast{
			RecentThreeHourUSD:       roundUSD(recent3h),
			PreviousDaySamePeriodUSD: roundUSD(prevDay3h),
			RecentRPMWindowUSD:       roundUSD(rpmSpend),
			BlendedHourlyRateUSD:     roundUSD(blended),
			RPMHourlyRateUSD:         roundUSD(rpmHourly),
		},
		Summary: OpenAIOAuthCapacityTimeseriesSummary{
			SpentUSD:              summaryWindow.CurrentSpendUSD,
			AvailableUSD:          availPtr,
			CapacityUSD:           capPtr,
			UsedPercent:           usedPtr,
			ForecastRemainingUSD:  roundUSD(forecastRemaining),
			ProjectedCycleUSD:     roundUSD(projected),
			ProjectedShortfallUSD: shortfallPtr,
			Confidence:            summaryWindow.Confidence,
			MeasuredAccounts:      summaryWindow.MeasuredAccounts,
			CapacityAccounts:      summaryWindow.CapacityAccounts,
		},
		Windows:         scope.Windows,
		Points:          points,
		Recommendations: scope.Recommendations,
		Baselines:       overview.Baselines,
	}, nil
}

func resolveOpenAIOAuthTimeseriesBounds(query OpenAIOAuthCapacityTimeseriesQuery, now time.Time) (pastHours, futureHours int, rangeStart, rangeEnd time.Time) {
	currentHour := now.Truncate(time.Hour)
	if query.Range == "custom" && query.From != nil && query.To != nil {
		from := query.From.UTC().Truncate(time.Hour)
		toRaw := query.To.UTC()
		to := toRaw.Truncate(time.Hour)
		if to.Before(toRaw) {
			to = to.Add(time.Hour)
		}
		// Sub-hour custom ranges collapse to the same truncated hour; expand to one bucket.
		if !to.After(from) {
			to = from.Add(time.Hour)
		}
		if to.Before(now) || to.Equal(now) {
			// Fully historical custom range (rangeEnd exclusive in the point loop).
			pastHours = int(to.Sub(from).Hours())
			if pastHours < 1 {
				pastHours = 1
				to = from.Add(time.Hour)
			}
			futureHours = 0
			return pastHours, futureHours, from, to
		}
		if from.After(now) {
			// Fully future custom range.
			futureHours = int(to.Sub(from).Hours())
			if futureHours < 1 {
				futureHours = 1
				to = from.Add(time.Hour)
			}
			return 0, futureHours, from, to
		}
		pastHours = int(currentHour.Sub(from).Hours())
		if pastHours < 0 {
			pastHours = 0
		}
		futureHours = int(to.Sub(currentHour).Hours()) - 1
		if futureHours < 0 {
			futureHours = 0
		}
		return pastHours, futureHours, from, to
	}

	pastHours = query.PastHours
	futureHours = query.FutureHours
	if pastHours < 1 {
		pastHours = 12
	}
	rangeStart = currentHour.Add(-time.Duration(pastHours) * time.Hour)
	rangeEnd = currentHour.Add(time.Duration(futureHours+1) * time.Hour)
	return pastHours, futureHours, rangeStart, rangeEnd
}

func aggregateOpenAIOAuthHourlySpend(rows []OpenAIOAuthHourlySpendRow, groupID *int64) map[time.Time]float64 {
	out := make(map[time.Time]float64)
	for _, row := range rows {
		if groupID != nil && row.GroupID != *groupID {
			continue
		}
		bucket := row.BucketStart.UTC().Truncate(time.Hour)
		out[bucket] += row.SpendUSD
	}
	return out
}

func openAIOAuthWindowFromScope(scope OpenAIOAuthCapacityScope, window string) OpenAIOAuthCapacityWindow {
	for _, item := range scope.Windows {
		if strings.EqualFold(item.Window, window) {
			return item
		}
	}
	return OpenAIOAuthCapacityWindow{Window: window}
}

func windowDuration(window string) time.Duration {
	switch strings.ToLower(window) {
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return 5 * time.Hour
	}
}

func openAIOAuthTimeseriesHourlyRate(recent3h, prevDay3h, rpmHourly float64) float64 {
	recentRate := recent3h / 3
	prevRate := prevDay3h / 3
	var totalWeight, weighted float64
	if recent3h > 0 {
		weighted += recentRate * openAIOAuthTimeseriesRecentWeight
		totalWeight += openAIOAuthTimeseriesRecentWeight
	}
	if prevDay3h > 0 {
		weighted += prevRate * openAIOAuthTimeseriesPreviousDayWeight
		totalWeight += openAIOAuthTimeseriesPreviousDayWeight
	}
	if rpmHourly > 0 {
		weighted += rpmHourly * openAIOAuthTimeseriesRPMWeight
		totalWeight += openAIOAuthTimeseriesRPMWeight
	}
	if totalWeight <= 0 {
		return 0
	}
	return weighted / totalWeight
}

// openAIOAuthPreviousDayShape returns spend per future hour-start using the previous day's same clock hour.
func openAIOAuthPreviousDayShape(hourly map[time.Time]float64, now time.Time, futureHours int) map[time.Time]float64 {
	currentHour := now.Truncate(time.Hour)
	shape := make(map[time.Time]float64, futureHours+1)
	for i := 0; i <= futureHours; i++ {
		bucket := currentHour.Add(time.Duration(i) * time.Hour)
		prev := bucket.Add(-24 * time.Hour)
		shape[bucket] = hourly[prev]
	}
	return shape
}

func openAIOAuthShapeFactor(shape map[time.Time]float64, bucket time.Time, blended float64) float64 {
	if blended <= 0 {
		return 0
	}
	if len(shape) == 0 {
		return 1
	}
	values := make([]float64, 0, len(shape))
	for _, v := range shape {
		if v > 0 {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return 1
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	if mean <= 0 {
		return 1
	}
	value := shape[bucket.UTC().Truncate(time.Hour)]
	if value <= 0 {
		// Keep a floor so zero-history hours still project mild demand.
		return 0.5
	}
	factor := value / mean
	// Bound extreme spikes from single noisy hours.
	return math.Max(0.25, math.Min(factor, 3.0))
}

func openAIOAuthFutureForecastMap(
	hourly map[time.Time]float64,
	currentHourStart, now, until time.Time,
	blended float64,
	shape map[time.Time]float64,
) map[time.Time]float64 {
	out := make(map[time.Time]float64)
	// Remaining portion of current hour.
	if until.After(now) {
		remaining := currentHourStart.Add(time.Hour).Sub(now).Hours()
		if remaining < 0 {
			remaining = 0
		}
		// Only the remaining forecast, not the already-spent portion.
		out[currentHourStart] = blended * openAIOAuthShapeFactor(shape, currentHourStart, blended) * remaining
	}
	for bucket := currentHourStart.Add(time.Hour); bucket.Before(until); bucket = bucket.Add(time.Hour) {
		out[bucket] = blended * openAIOAuthShapeFactor(shape, bucket, blended)
	}
	return out
}

// openAIOAuthAvailableAt estimates available capacity at time `at` for a rolling window.
// Historical hours use only actual hourly spend. Future projection adds forecastSpendByHour.
func openAIOAuthAvailableAt(
	capacity float64,
	hourly map[time.Time]float64,
	at time.Time,
	window time.Duration,
	now time.Time,
	forecastByHour map[time.Time]float64,
	blended float64,
	shape map[time.Time]float64,
) (available float64, usedPercent float64) {
	if capacity <= 0 {
		return 0, 0
	}
	windowStart := at.Add(-window)
	var spend float64

	// Walk hour buckets that overlap (windowStart, at].
	hour := windowStart.Truncate(time.Hour)
	if hour.Before(windowStart) {
		// partial first hour handled below by fraction.
	}
	for ; !hour.After(at); hour = hour.Add(time.Hour) {
		hourEnd := hour.Add(time.Hour)
		// Overlap of [hour, hourEnd) with (windowStart, at]
		overlapStart := hour
		if overlapStart.Before(windowStart) {
			overlapStart = windowStart
		}
		overlapEnd := hourEnd
		if overlapEnd.After(at) {
			overlapEnd = at
		}
		if !overlapEnd.After(overlapStart) {
			continue
		}
		fraction := overlapEnd.Sub(overlapStart).Hours()
		if fraction <= 0 {
			continue
		}

		actual := hourly[hour]
		// For the current incomplete hour, hourly already reflects spend until `now` only when
		// the repository upper-bound is now. Scale only when we need a sub-hour slice of a
		// completed hour (historical partial edges).
		if hourEnd.Before(now) || hourEnd.Equal(now) {
			// Fully completed hour in the DB sense for hours entirely before now.
			// If we only need a fraction of the hour (window edge), prorate uniformly.
			if fraction < 1 {
				spend += actual * fraction
			} else {
				spend += actual
			}
			continue
		}

		// Current or future hour relative to now.
		if !hour.After(now.Truncate(time.Hour)) && hourEnd.After(now) {
			// Current hour: actual is spend from hour start to now.
			// If at <= now: use full actual (already partial).
			// If at > now: actual + forecast remainder up to at.
			if !at.After(now) {
				// Need fraction of the partial actual if windowStart cuts into the hour.
				elapsed := now.Sub(hour).Hours()
				if elapsed <= 0 {
					continue
				}
				needed := overlapEnd.Sub(overlapStart).Hours()
				// overlap is within [hour, now]
				if needed >= elapsed {
					spend += actual
				} else {
					spend += actual * (needed / elapsed)
				}
			} else {
				spend += actual
				if forecastByHour != nil {
					// forecastByHour for current hour is remaining-to-hour-end amount; scale to `at`.
					remToEnd := hourEnd.Sub(now).Hours()
					if remToEnd > 0 {
						remToAt := at.Sub(now).Hours()
						if remToAt > remToEnd {
							remToAt = remToEnd
						}
						spend += forecastByHour[hour] * (remToAt / remToEnd)
					}
				}
			}
			continue
		}

		// Pure future hour.
		if forecastByHour != nil {
			if fraction < 1 {
				spend += forecastByHour[hour] * fraction
			} else {
				spend += forecastByHour[hour]
			}
		} else if blended > 0 {
			factor := openAIOAuthShapeFactor(shape, hour, blended)
			spend += blended * factor * fraction
		}
	}

	available = math.Max(capacity-spend, 0)
	usedPercent = math.Round((spend/capacity)*10000) / 100
	return roundUSD(available), usedPercent
}

// loadOpenAIOAuthHourlyFactsWithBackfill returns hourly facts for [from, to).
// Sealed hours are served from openai_oauth_capacity_hourly only.
// Missing completed hours are computed once from usage_logs and sealed.
// The current (incomplete) hour is computed live and not sealed.
func loadOpenAIOAuthHourlyFactsWithBackfill(
	ctx context.Context,
	repo openAIOAuthCapacityTimeseriesRepository,
	from, to time.Time,
	groupID *int64,
	force bool,
) ([]OpenAIOAuthCapacityHourlyFact, error) {
	from = from.UTC().Truncate(time.Hour)
	to = to.UTC()
	currentHour := to.Truncate(time.Hour)

	existing, err := repo.GetOpenAIOAuthCapacityHourlyFacts(ctx, from, to, groupID, false)
	if err != nil {
		return nil, fmt.Errorf("get OpenAI OAuth hourly facts: %w", err)
	}
	byBucket := make(map[time.Time]OpenAIOAuthCapacityHourlyFact, len(existing))
	for _, fact := range existing {
		byBucket[fact.BucketStart.UTC().Truncate(time.Hour)] = fact
	}

	// Identify completed hours missing sealed facts (or force-rebuild unsealed).
	needFrom := time.Time{}
	needTo := time.Time{}
	missing := make([]time.Time, 0)
	for bucket := from; bucket.Before(currentHour); bucket = bucket.Add(time.Hour) {
		fact, ok := byBucket[bucket]
		if ok && fact.Sealed && !force {
			continue
		}
		// Force only re-scans unsealed rows; sealed stays frozen.
		if ok && fact.Sealed {
			continue
		}
		missing = append(missing, bucket)
		if needFrom.IsZero() || bucket.Before(needFrom) {
			needFrom = bucket
		}
		needTo = bucket.Add(time.Hour)
	}
	// Always refresh the current incomplete hour from raw logs (not sealed).
	liveFrom := currentHour
	liveTo := to
	if liveTo.After(liveFrom) {
		if needFrom.IsZero() || liveFrom.Before(needFrom) {
			needFrom = liveFrom
		}
		if needTo.IsZero() || liveTo.After(needTo) {
			needTo = liveTo
		}
	}

	if !needFrom.IsZero() && needTo.After(needFrom) {
		rawRows, err := repo.GetOpenAIOAuthCapacityHourlySpend(ctx, needFrom, needTo)
		if err != nil {
			return nil, fmt.Errorf("backfill OpenAI OAuth hourly spend: %w", err)
		}
		raw := aggregateOpenAIOAuthHourlySpend(rawRows, groupID)

		// Seal only buckets returned by the aggregation. Missing buckets remain
		// retryable so late logs cannot be frozen permanently as zero spend.
		toUpsert := make([]OpenAIOAuthCapacityHourlyFact, 0, len(missing)+1)
		gid := OpenAIOAuthCapacityAllGroupsID
		if groupID != nil {
			gid = *groupID
		}
		for _, bucket := range missing {
			spent, found := raw[bucket]
			if !found {
				continue
			}
			fact := OpenAIOAuthCapacityHourlyFact{
				BucketStart: bucket,
				GroupID:     gid,
				PlanType:    "all",
				SpentUSD:    roundUSD(spent),
				Sealed:      true,
				Source:      "usage_logs",
			}
			toUpsert = append(toUpsert, fact)
			byBucket[bucket] = fact
		}
		// Current hour: store unsealed live snapshot so UI can show partial spend without sealing.
		if liveTo.After(liveFrom) {
			spent := raw[currentHour]
			fact := OpenAIOAuthCapacityHourlyFact{
				BucketStart: currentHour,
				GroupID:     gid,
				PlanType:    "all",
				SpentUSD:    roundUSD(spent),
				Sealed:      false,
				Source:      "live",
			}
			toUpsert = append(toUpsert, fact)
			byBucket[currentHour] = fact
		}
		if len(toUpsert) > 0 {
			if err := repo.UpsertOpenAIOAuthCapacityHourlyFacts(ctx, toUpsert); err != nil {
				return nil, fmt.Errorf("persist OpenAI OAuth hourly facts: %w", err)
			}
		}
	}

	// Materialize ordered result covering [from, to) known facts only.
	result := make([]OpenAIOAuthCapacityHourlyFact, 0, len(byBucket))
	for bucket := from; !bucket.After(currentHour); bucket = bucket.Add(time.Hour) {
		if fact, ok := byBucket[bucket]; ok {
			result = append(result, fact)
		}
	}
	return result, nil
}

func openAIOAuthHourlySpendFromFacts(facts []OpenAIOAuthCapacityHourlyFact) map[time.Time]float64 {
	out := make(map[time.Time]float64, len(facts))
	for _, fact := range facts {
		out[fact.BucketStart.UTC().Truncate(time.Hour)] = fact.SpentUSD
	}
	return out
}

func applySealedWindowMetrics(point *OpenAIOAuthCapacityTimeseriesPoint, fact OpenAIOAuthCapacityHourlyFact, window string) {
	if point == nil {
		return
	}
	// Primary window metrics.
	use7d := strings.EqualFold(window, "7d")
	if use7d {
		point.CapacityUSD = fact.Capacity7dUSD
		point.AvailableUSD = fact.Available7dUSD
		point.UsedPercent = fact.UsedPercent7d
	} else {
		point.CapacityUSD = fact.Capacity5hUSD
		point.AvailableUSD = fact.Available5hUSD
		point.UsedPercent = fact.UsedPercent5h
	}
	if strings.EqualFold(window, "both") {
		point.Capacity7dUSD = fact.Capacity7dUSD
		point.Available7dUSD = fact.Available7dUSD
		point.UsedPercent7d = fact.UsedPercent7d
		// Primary stays 5h for "both".
		point.CapacityUSD = fact.Capacity5hUSD
		point.AvailableUSD = fact.Available5hUSD
		point.UsedPercent = fact.UsedPercent5h
	}
}

func openAIOAuthWindowCapacityOK(window OpenAIOAuthCapacityWindow) bool {
	return window.CapacityAccounts > 0 && window.EstimatedCapacityUSD != nil && *window.EstimatedCapacityUSD > 0
}

func openAIOAuthWindowCapacityUSD(window OpenAIOAuthCapacityWindow) float64 {
	if window.EstimatedCapacityUSD == nil {
		return 0
	}
	return *window.EstimatedCapacityUSD
}

func floatPtr(value float64) *float64 {
	v := value
	return &v
}
