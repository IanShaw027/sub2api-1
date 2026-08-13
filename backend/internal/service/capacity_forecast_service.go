package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// This file wires the pure forecasting/simulation math (capacity_forecast_pure.go)
// and the generic CapacityProvider abstraction (capacity_provider.go) into the two
// public operations the admin API needs: GetTimeseries (read, Redis-cached) and
// Probe (write, batch upstream refresh). It also runs a lightweight periodic
// sealer so capacity_hourly history accumulates even if nobody opens the UI,
// mirroring dashboard_aggregation_service.go's timing-wheel + leader-lock pattern.

var (
	// ErrCapacityInvalidRange is returned for an unsupported range key.
	ErrCapacityInvalidRange = errors.New("capacity forecast: invalid range")
	// ErrCapacityProviderNotFound is returned when the platform has no registered CapacityProvider.
	ErrCapacityProviderNotFound = errors.New("capacity forecast: no provider registered for platform")
)

// capacityForecastValidRanges maps the API's accepted `range` query values to
// their total (past+future) span. Each range is split evenly into past/future
// hour buckets around the current hour.
var capacityForecastValidRanges = map[string]time.Duration{
	"12h": 12 * time.Hour,
	"24h": 24 * time.Hour,
	"48h": 48 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

const (
	capacityForecastCacheTTL       = 60 * time.Second
	capacityForecastCacheKeyPrefix = "capacity:forecast:timeseries:"

	// capacityForecastSealInterval/LockKey/LockTTL drive the background job that
	// guarantees capacity_hourly history exists even without any UI traffic.
	capacityForecastSealInterval = 10 * time.Minute
	capacityForecastSealLockKey  = "capacity:forecast:seal:leader"
	capacityForecastSealLockTTL  = 5 * time.Minute

	capacityForecastProbeConcurrency = 8
	capacityForecastProbeTimeout     = 110 * time.Second
)

// CapacityForecastService is the generic (multi-platform) capacity forecasting
// engine's service-layer glue: it fetches data via CapacityRepository/
// AccountRepository, delegates the actual math to the pure functions in
// capacity_forecast_pure.go, and refreshes upstream quota via the
// CapacityProviderRegistry.
type CapacityForecastService struct {
	repo        CapacityRepository
	accountRepo AccountRepository
	registry    *CapacityProviderRegistry
	redisClient *redis.Client

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	// nowFunc is overridable in tests; production always uses time.Now.
	nowFunc func() time.Time
}

// NewCapacityForecastService constructs the service. redisClient may be nil
// (caching is then a no-op); lockCache/db are injected separately via
// SetLeaderLock since they are only needed by the periodic sealer.
func NewCapacityForecastService(repo CapacityRepository, accountRepo AccountRepository, registry *CapacityProviderRegistry, redisClient *redis.Client) *CapacityForecastService {
	return &CapacityForecastService{
		repo:        repo,
		accountRepo: accountRepo,
		registry:    registry,
		redisClient: redisClient,
		instanceID:  uuid.NewString(),
		nowFunc:     time.Now,
	}
}

// SetLeaderLock injects the leader-lock cache and DB used to elect a single
// instance for the periodic seal job. When both are nil the job runs ungated
// (single-instance / test behavior) — see tryAcquireSingletonLeaderLock.
func (s *CapacityForecastService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// Start registers the periodic capacity_hourly sealer on the shared timing wheel.
func (s *CapacityForecastService) Start(timingWheel *TimingWheelService) {
	if s == nil || timingWheel == nil {
		return
	}
	timingWheel.ScheduleRecurring("capacity:seal", capacityForecastSealInterval, s.runScheduledSeal)
}

func (s *CapacityForecastService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

// ---------------------------------------------------------------------------
// GET .../capacity/timeseries
// ---------------------------------------------------------------------------

// GetTimeseries computes (or returns a cached copy of) the capacity forecast
// timeseries for the given platform/group/range. force=true bypasses the
// Redis cache (still repopulating it with the freshly computed result).
func (s *CapacityForecastService) GetTimeseries(ctx context.Context, platform string, groupID *int64, rangeKey string, force bool) (*CapacityTimeseries, error) {
	if s == nil {
		return nil, errors.New("capacity forecast service not configured")
	}
	rangeDuration, ok := capacityForecastValidRanges[rangeKey]
	if !ok {
		return nil, ErrCapacityInvalidRange
	}
	provider, ok := s.registry.Get(platform)
	if !ok {
		return nil, ErrCapacityProviderNotFound
	}

	cacheKey := capacityForecastCacheKey(platform, groupID, rangeKey)
	if !force {
		if cached, ok := s.readCache(ctx, cacheKey); ok {
			return cached, nil
		}
	}

	result, err := s.computeTimeseries(ctx, provider, platform, groupID, rangeKey, rangeDuration)
	if err != nil {
		return nil, err
	}
	s.writeCache(ctx, cacheKey, result)
	return result, nil
}

func (s *CapacityForecastService) computeTimeseries(ctx context.Context, provider CapacityProvider, platform string, groupID *int64, rangeKey string, rangeDuration time.Duration) (*CapacityTimeseries, error) {
	now := s.now().UTC()
	totalHours := int(rangeDuration / time.Hour)
	pastHours := totalHours / 2
	futureHours := totalHours / 2

	currentBucket := now.Truncate(time.Hour)
	rangeStart := currentBucket.Add(-time.Duration(pastHours) * time.Hour)
	rangeEnd := currentBucket.Add(time.Duration(futureHours+1) * time.Hour) // exclusive

	// Shape history needs at least the two prior days (for the same-hour
	// yesterday/day-before-yesterday base), regardless of how short the
	// requested past window is.
	shapeStart := rangeStart
	if s := currentBucket.Add(-49 * time.Hour); s.Before(shapeStart) {
		shapeStart = s
	}

	spendRows, err := s.repo.GetHourlySpend(ctx, platform, groupID, shapeStart, now)
	if err != nil {
		return nil, err
	}
	hourly := make(map[time.Time]float64, len(spendRows))
	for _, p := range spendRows {
		hourly[p.BucketStart.UTC()] = p.SpendUSD
	}

	factsRows, err := s.repo.GetHourlyFacts(ctx, platform, groupID, rangeStart, currentBucket)
	if err != nil {
		return nil, err
	}

	accounts, err := s.listOAuthAccounts(ctx, platform, groupID)
	if err != nil {
		return nil, err
	}
	states, err := s.buildAccountStates(ctx, provider, platform, accounts, now)
	if err != nil {
		return nil, err
	}

	var currentAvailable, currentCapacity float64
	for _, st := range states {
		if h, c, ok := accountHeadroomWithCapacity(st.Windows); ok {
			currentAvailable += h
			currentCapacity += c
		}
	}

	futureBuckets := make([]time.Time, 0, futureHours)
	for i := 1; i <= futureHours; i++ {
		futureBuckets = append(futureBuckets, currentBucket.Add(time.Duration(i)*time.Hour))
	}

	forecastInput := SpendForecastInput{
		CurrentBucket: currentBucket,
		Hourly:        hourly,
		// CurrentBucket is forecast too, so the simulation's rolling balance has
		// a "remaining current hour" spend figure to subtract before the first
		// real future bucket, without a separate calculation.
		FutureBuckets: append([]time.Time{currentBucket}, futureBuckets...),
	}
	forecastResult := ForecastFutureSpend(forecastInput)

	simInput := CapacitySimulationInput{
		CurrentBucket:    currentBucket,
		Buckets:          futureBuckets,
		RangeEnd:         rangeEnd,
		CurrentAvailable: currentAvailable,
		Accounts:         states,
		ForecastSpend:    forecastResult.ForecastByBucket,
	}
	simResult := SimulateFutureCapacity(simInput)

	points := make([]CapacityPoint, 0, totalHours+1)
	var shortfallInputs []CapacityShortfallPoint
	for t := rangeStart; t.Before(rangeEnd); t = t.Add(time.Hour) {
		switch {
		case t.Before(currentBucket):
			spend := hourly[t] // missing past buckets = 0, per spec
			point := CapacityPoint{BucketStart: t, Segment: "past", SpendUSD: &spend}
			if fact, ok := factsRows[t]; ok {
				point.AvailableUSD = fact.AvailableUSD
			}
			points = append(points, point)
		case t.Equal(currentBucket):
			spend := hourly[t]
			avail := currentAvailable
			points = append(points, CapacityPoint{
				BucketStart: t, Segment: "current",
				SpendUSD: &spend, AvailableUSD: &avail,
			})
		default:
			fs := forecastResult.ForecastByBucket[t]
			fa := simResult.AvailableByBucket[t]
			rec := simResult.RecoveredByBucket[t]
			points = append(points, CapacityPoint{
				BucketStart: t, Segment: "future",
				ForecastSpendUSD: &fs, ForecastAvailableUSD: &fa, RecoveredUSD: rec,
			})
			shortfallInputs = append(shortfallInputs, CapacityShortfallPoint{
				BucketStart: t, ForecastAvailable: fa, ForecastSpend: fs,
			})
		}
	}

	var events []CapacityEvent
	for _, rec := range simResult.RecoverEvents {
		events = append(events, CapacityEvent{
			At: rec.At, Type: "recover", WindowKind: rec.WindowKind,
			AccountCount: len(rec.Accounts), AmountUSD: rec.AmountUSD, Accounts: rec.Accounts,
		})
	}

	var allCapacities []float64
	for _, st := range states {
		for _, w := range st.Windows {
			if w.CapacityUSD > 0 {
				allCapacities = append(allCapacities, w.CapacityUSD)
			}
		}
	}
	medianCapacity := medianPositiveValues(allCapacities)
	shortfallEvents, firstShortfallAt, recommendations := ComputeShortfallEvents(shortfallInputs, medianCapacity)
	events = append(events, shortfallEvents...)
	sort.Slice(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })

	var futureForecastSpend float64
	for _, t := range futureBuckets {
		futureForecastSpend += forecastResult.ForecastByBucket[t]
	}
	suggestAccounts := 0
	if len(recommendations) > 0 {
		suggestAccounts = recommendations[0].SuggestAccounts
	}

	result := &CapacityTimeseries{
		GeneratedAt: time.Now().UTC(),
		Now:         now,
		Platform:    platform,
		GroupID:     groupID,
		Unit:        "usd",
		Range:       rangeKey,
		KPIs: CapacityKPIs{
			CurrentAvailableUSD:    currentAvailable,
			FutureForecastSpendUSD: futureForecastSpend,
			FirstShortfallAt:       firstShortfallAt,
			SuggestAccounts:        suggestAccounts,
		},
		Points:          points,
		Events:          events,
		Recommendations: recommendations,
	}

	// Seal the requested scope's current-hour bucket so history accumulates
	// even for ad-hoc group_id scopes the periodic sealer doesn't special-case.
	s.sealCurrentBucket(ctx, platform, groupID, currentBucket, hourly[currentBucket], currentAvailable, currentCapacity)
	if groupID != nil {
		// Also seal the platform-total (group_id=null) row so the platform-wide
		// view has history even when only group-scoped requests are ever made.
		s.sealPlatformTotal(ctx, provider, platform, currentBucket, now)
	}

	return result, nil
}

// listOAuthAccounts returns the platform's active OAuth accounts, optionally
// filtered to those bound to groupID. ListByPlatform's StatusActive-only
// filter is intentional here: a disabled/error account contributes no
// schedulable capacity and should not be counted.
func (s *CapacityForecastService) listOAuthAccounts(ctx context.Context, platform string, groupID *int64) ([]Account, error) {
	all, err := s.accountRepo.ListByPlatform(ctx, platform)
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(all))
	for _, acc := range all {
		if acc.Type != AccountTypeOAuth {
			continue
		}
		if groupID != nil && !containsInt64(acc.GroupIDs, *groupID) {
			continue
		}
		out = append(out, acc)
	}
	return out, nil
}

// buildAccountStates converts each account's live QuotaWindows into
// CapacityWindowStates with an inferred CapacityUSD, using the fallback chain:
// live inference (spend/used_ratio, when used_ratio >= capacityMinimumInferenceRatio)
// -> most recent archived period's inferred capacity -> median of the same
// window kind's live-inferred capacities within scope -> unconstrained (0).
func (s *CapacityForecastService) buildAccountStates(ctx context.Context, provider CapacityProvider, platform string, accounts []Account, now time.Time) ([]CapacityAccountState, error) {
	type pendingFallback struct {
		accountIdx int
		windowIdx  int
		kind       string
	}

	states := make([]CapacityAccountState, 0, len(accounts))
	rawByKind := make(map[string][]float64)
	var pendings []pendingFallback

	// First pass: extract windows and collect one batched spend request per
	// (account, window) instead of issuing an N+1 query inside the loop.
	var spendReqs []CapacityAccountWindowSpendRequest
	for i := range accounts {
		acc := accounts[i]
		windows := provider.ExtractWindows(&acc, now)
		if len(windows) == 0 {
			continue
		}
		state := CapacityAccountState{ID: acc.ID, Name: acc.Name, Windows: make([]CapacityWindowState, len(windows))}
		for wi, w := range windows {
			state.Windows[wi] = CapacityWindowState{Kind: w.Kind, UsedRatio: w.UsedRatio, ResetAt: w.ResetAt, Duration: w.Duration}
			spendReqs = append(spendReqs, CapacityAccountWindowSpendRequest{
				AccountID: acc.ID, WindowKind: w.Kind, From: w.ResetAt.Add(-w.Duration),
			})
		}
		states = append(states, state)
	}

	spends, err := s.repo.GetAccountWindowSpends(ctx, spendReqs, now)
	if err != nil {
		return nil, err
	}

	// Second pass: infer capacity from the batched spends; anything that can't
	// be inferred live joins the fallback chain.
	for accountIdx := range states {
		state := &states[accountIdx]
		for wi := range state.Windows {
			w := &state.Windows[wi]
			spend := spends[CapacityAccountWindowSpendKey{AccountID: state.ID, WindowKind: w.Kind}]
			if w.UsedRatio >= capacityMinimumInferenceRatio && spend > 0 {
				capacity := spend / w.UsedRatio
				w.CapacityUSD = capacity
				rawByKind[w.Kind] = append(rawByKind[w.Kind], capacity)
			} else {
				pendings = append(pendings, pendingFallback{accountIdx: accountIdx, windowIdx: wi, kind: w.Kind})
			}
		}
	}

	medianByKind := make(map[string]float64, len(rawByKind))
	for kind, vals := range rawByKind {
		medianByKind[kind] = medianPositiveValues(vals)
	}

	for _, p := range pendings {
		if fallback, err := s.repo.GetLatestQuotaPeriodCapacity(ctx, platform, states[p.accountIdx].ID, p.kind); err == nil && fallback != nil && *fallback > 0 {
			states[p.accountIdx].Windows[p.windowIdx].CapacityUSD = *fallback
			continue
		}
		if median := medianByKind[p.kind]; median > 0 {
			states[p.accountIdx].Windows[p.windowIdx].CapacityUSD = median
		}
		// Otherwise leave CapacityUSD == 0: the window is treated as
		// non-constraining, per spec ("否则视为该窗口不构成约束").
	}

	return states, nil
}

// ---------------------------------------------------------------------------
// Sealing (capacity_hourly)
// ---------------------------------------------------------------------------

func (s *CapacityForecastService) sealCurrentBucket(ctx context.Context, platform string, groupID *int64, bucket time.Time, spend, available, capacity float64) {
	if s == nil || s.repo == nil {
		return
	}
	availCopy, capCopy := available, capacity
	upsert := CapacityHourlyUpsert{
		Platform: platform, GroupID: groupID, BucketStart: bucket,
		SpendUSD: spend, AvailableUSD: &availCopy, CapacityUSD: &capCopy, Sealed: true,
	}
	if err := s.repo.UpsertHourlyFacts(ctx, []CapacityHourlyUpsert{upsert}); err != nil {
		logger.LegacyPrintf("service.capacity_forecast", "[CapacityForecast] seal failed platform=%s group_id=%v bucket=%s err=%v", platform, groupID, bucket, err)
	}
}

// sealPlatformTotal computes and seals the whole-platform (group_id=null)
// current-hour row. It is deliberately independent of any group-scoped
// timeseries request so the periodic background job can call it directly.
func (s *CapacityForecastService) sealPlatformTotal(ctx context.Context, provider CapacityProvider, platform string, currentBucket, now time.Time) {
	accounts, err := s.listOAuthAccounts(ctx, platform, nil)
	if err != nil {
		return
	}
	states, err := s.buildAccountStates(ctx, provider, platform, accounts, now)
	if err != nil {
		return
	}
	var available, capacity float64
	for _, st := range states {
		if h, c, ok := accountHeadroomWithCapacity(st.Windows); ok {
			available += h
			capacity += c
		}
	}
	spend := s.currentHourSpend(ctx, platform, nil, currentBucket, now)
	s.sealCurrentBucket(ctx, platform, nil, currentBucket, spend, available, capacity)
}

func (s *CapacityForecastService) currentHourSpend(ctx context.Context, platform string, groupID *int64, bucket, now time.Time) float64 {
	rows, err := s.repo.GetHourlySpend(ctx, platform, groupID, bucket, now)
	if err != nil || len(rows) == 0 {
		return 0
	}
	for _, r := range rows {
		if r.BucketStart.UTC().Equal(bucket) {
			return r.SpendUSD
		}
	}
	return rows[0].SpendUSD
}

// runScheduledSeal is the periodic (every capacityForecastSealInterval) job
// that guarantees capacity_hourly history exists for every platform with a
// registered CapacityProvider and at least one active OAuth account, even if
// nobody ever opens the admin UI.
func (s *CapacityForecastService) runScheduledSeal() {
	if s == nil || s.registry == nil {
		return
	}
	release, ok := tryAcquireSingletonLeaderLock(context.Background(), s.lockCache, s.db, capacityForecastSealLockKey, s.instanceID, capacityForecastSealLockTTL)
	if !ok {
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	now := s.now().UTC()
	currentBucket := now.Truncate(time.Hour)

	for _, platform := range s.registry.Platforms() {
		provider, ok := s.registry.Get(platform)
		if !ok {
			continue
		}
		accounts, err := s.listOAuthAccounts(ctx, platform, nil)
		if err != nil || len(accounts) == 0 {
			continue
		}
		s.sealPlatformTotal(ctx, provider, platform, currentBucket, now)
	}
}

// ---------------------------------------------------------------------------
// POST .../capacity/probe
// ---------------------------------------------------------------------------

// Probe refreshes upstream quota for a bounded batch of accounts (platform +
// type=oauth, plus status filtering per includeNormal) and invalidates the
// cache for the affected scope afterward.
func (s *CapacityForecastService) Probe(ctx context.Context, platform string, groupID *int64, includeNormal bool) (*CapacityProbeResult, error) {
	if s == nil {
		return nil, errors.New("capacity forecast service not configured")
	}
	provider, ok := s.registry.Get(platform)
	if !ok {
		return nil, ErrCapacityProviderNotFound
	}

	accounts, err := s.listOAuthAccounts(ctx, platform, groupID)
	if err != nil {
		return nil, err
	}

	targets := make([]Account, 0, len(accounts))
	for _, acc := range accounts {
		if includeNormal || acc.IsRateLimited() {
			targets = append(targets, acc)
		}
	}

	result := &CapacityProbeResult{Total: len(targets)}
	if len(targets) == 0 {
		return result, nil
	}

	probeCtx, cancel := context.WithTimeout(ctx, capacityForecastProbeTimeout)
	defer cancel()

	var mu sync.Mutex
	sem := make(chan struct{}, capacityForecastProbeConcurrency)
	var wg sync.WaitGroup
	start := time.Now()

	for i := range targets {
		acc := targets[i]
		wasRateLimited := acc.IsRateLimited()
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			probeErr := provider.Probe(probeCtx, &acc)

			mu.Lock()
			defer mu.Unlock()
			result.Probed++
			switch {
			case probeErr != nil:
				result.Failed++
				result.Failures = append(result.Failures, CapacityProbeFailure{AccountID: acc.ID, Name: acc.Name, Error: probeErr.Error()})
			case wasRateLimited:
				result.RateLimited++
			default:
				result.OK++
			}
		}()
	}
	wg.Wait()
	result.DurationMS = time.Since(start).Milliseconds()

	s.invalidateCache(ctx, platform, groupID)
	return result, nil
}

// ---------------------------------------------------------------------------
// Redis cache
// ---------------------------------------------------------------------------

func capacityForecastCacheKey(platform string, groupID *int64, rangeKey string) string {
	gid := "all"
	if groupID != nil {
		gid = strconv.FormatInt(*groupID, 10)
	}
	return capacityForecastCacheKeyPrefix + platform + ":" + gid + ":" + rangeKey
}

func (s *CapacityForecastService) readCache(ctx context.Context, key string) (*CapacityTimeseries, bool) {
	if s.redisClient == nil {
		return nil, false
	}
	raw, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, false
	}
	var result CapacityTimeseries
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, false
	}
	return &result, true
}

func (s *CapacityForecastService) writeCache(ctx context.Context, key string, result *CapacityTimeseries) {
	if s.redisClient == nil || result == nil {
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = s.redisClient.Set(ctx, key, raw, capacityForecastCacheTTL).Err()
}

// invalidateCache drops every range's cache entry for a platform/group scope
// (Probe doesn't know which ranges are currently cached, so it clears them
// all), plus the platform-total scope when a group-scoped probe was run.
func (s *CapacityForecastService) invalidateCache(ctx context.Context, platform string, groupID *int64) {
	if s.redisClient == nil {
		return
	}
	for rangeKey := range capacityForecastValidRanges {
		_ = s.redisClient.Del(ctx, capacityForecastCacheKey(platform, groupID, rangeKey)).Err()
	}
	if groupID != nil {
		for rangeKey := range capacityForecastValidRanges {
			_ = s.redisClient.Del(ctx, capacityForecastCacheKey(platform, nil, rangeKey)).Err()
		}
	}
}
