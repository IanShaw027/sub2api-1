package service

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	opsAggHourlyInterval = 10 * time.Minute
	opsAggDailyInterval  = 1 * time.Hour

	// Keep in sync with ops_service.go parseTimeRange maxWindow (30d).
	opsAggBackfillWindow = 30 * 24 * time.Hour

	opsAggHourlyOverlap = 2 * time.Hour
	opsAggDailyOverlap  = 48 * time.Hour

	opsAggHourlyChunk = 24 * time.Hour
	opsAggDailyChunk  = 7 * 24 * time.Hour

	opsAggMaxQueryTimeout = 3 * time.Second
	opsAggHourlyTimeout   = 5 * time.Minute
	opsAggDailyTimeout    = 2 * time.Minute
)

type OpsAggregationService struct {
	repo  OpsRepository
	sqlDB *sql.DB
	rdb   *redis.Client

	hourlyInterval time.Duration
	dailyInterval  time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	wg        sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once

	hourlyMu sync.Mutex
	dailyMu  sync.Mutex

	distributedLockOn   bool
	distributedLockWarn sync.Once

	skipLogMu sync.Mutex
	skipLogAt time.Time
}

func NewOpsAggregationService(repo OpsRepository, sqlDB *sql.DB, rdb *redis.Client) *OpsAggregationService {
	ctx, cancel := context.WithCancel(context.Background())
	return &OpsAggregationService{
		repo:           repo,
		sqlDB:          sqlDB,
		rdb:            rdb,
		hourlyInterval: opsAggHourlyInterval,
		dailyInterval:  opsAggDailyInterval,
		ctx:            ctx,
		cancel:         cancel,

		distributedLockOn: true,
	}
}

func (s *OpsAggregationService) Start() {
	if s == nil {
		return
	}
	if s.repo == nil || s.sqlDB == nil {
		log.Printf("[OpsAggregation] not started (missing dependencies)")
		return
	}
	s.startOnce.Do(func() {
		s.wg.Add(2)
		go s.hourlyLoop()
		go s.dailyLoop()
		log.Printf("[OpsAggregation] started (hourly=%s, daily=%s)", s.hourlyInterval, s.dailyInterval)
	})
}

func (s *OpsAggregationService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
		log.Printf("[OpsAggregation] stopped")
	})
}

func (s *OpsAggregationService) hourlyLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.hourlyInterval)
	defer ticker.Stop()

	s.aggregateHourly()
	for {
		select {
		case <-ticker.C:
			s.aggregateHourly()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *OpsAggregationService) dailyLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.dailyInterval)
	defer ticker.Stop()

	s.aggregateDaily()
	for {
		select {
		case <-ticker.C:
			s.aggregateDaily()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *OpsAggregationService) aggregateHourly() {
	if s == nil || s.repo == nil || s.sqlDB == nil {
		return
	}

	releaseLeaderLock, ok := s.tryAcquireLeaderLock(s.ctx, opsAggHourlyLeaderLockKeyDefault, opsAggHourlyLeaderLockTTLDefault, "[OpsAggregation][hourly]")
	if !ok {
		return
	}
	if releaseLeaderLock != nil {
		defer releaseLeaderLock()
	}

	s.hourlyMu.Lock()
	defer s.hourlyMu.Unlock()

	end := utcFloorToHour(time.Now())
	if end.IsZero() {
		return
	}

	start := end.Add(-opsAggBackfillWindow)

	ctxMax, cancel := context.WithTimeout(s.ctx, opsAggMaxQueryTimeout)
	latest, ok, err := s.getLatestHourlyBucketStart(ctxMax)
	cancel()
	if err != nil {
		log.Printf("[OpsAggregation] hourly: failed to read latest bucket_start: %v", err)
	} else if ok {
		candidate := latest.Add(-opsAggHourlyOverlap)
		if candidate.After(start) {
			start = candidate
		}
	}

	start = utcFloorToHour(start)
	if !start.Before(end) {
		return
	}

	for cursor := start; cursor.Before(end); cursor = cursor.Add(opsAggHourlyChunk) {
		chunkEnd := minTime(cursor.Add(opsAggHourlyChunk), end)
		ctxRun, cancel := context.WithTimeout(s.ctx, opsAggHourlyTimeout)
		err := s.repo.UpsertHourlyMetrics(ctxRun, cursor, chunkEnd)
		cancel()
		if err != nil {
			log.Printf("[OpsAggregation] hourly: upsert failed (%s..%s): %v", cursor.Format(time.RFC3339), chunkEnd.Format(time.RFC3339), err)
			return
		}
	}
}

func (s *OpsAggregationService) aggregateDaily() {
	if s == nil || s.repo == nil || s.sqlDB == nil {
		return
	}

	releaseLeaderLock, ok := s.tryAcquireLeaderLock(s.ctx, opsAggDailyLeaderLockKeyDefault, opsAggDailyLeaderLockTTLDefault, "[OpsAggregation][daily]")
	if !ok {
		return
	}
	if releaseLeaderLock != nil {
		defer releaseLeaderLock()
	}

	s.dailyMu.Lock()
	defer s.dailyMu.Unlock()

	end := utcFloorToDay(time.Now())
	if end.IsZero() {
		return
	}

	start := end.Add(-opsAggBackfillWindow)

	ctxMax, cancel := context.WithTimeout(s.ctx, opsAggMaxQueryTimeout)
	latest, ok, err := s.getLatestDailyBucketDate(ctxMax)
	cancel()
	if err != nil {
		log.Printf("[OpsAggregation] daily: failed to read latest bucket_date: %v", err)
	} else if ok {
		candidate := utcFloorToDay(latest).Add(-opsAggDailyOverlap)
		if candidate.After(start) {
			start = candidate
		}
	}

	start = utcFloorToDay(start)
	if !start.Before(end) {
		return
	}

	for cursor := start; cursor.Before(end); cursor = cursor.Add(opsAggDailyChunk) {
		chunkEnd := minTime(cursor.Add(opsAggDailyChunk), end)
		ctxRun, cancel := context.WithTimeout(s.ctx, opsAggDailyTimeout)
		err := s.repo.UpsertDailyMetrics(ctxRun, cursor, chunkEnd)
		cancel()
		if err != nil {
			log.Printf("[OpsAggregation] daily: upsert failed (%s..%s): %v", cursor.Format("2006-01-02"), chunkEnd.Format("2006-01-02"), err)
			return
		}
	}
}

func (s *OpsAggregationService) getLatestHourlyBucketStart(ctx context.Context) (time.Time, bool, error) {
	var value sql.NullTime
	if err := s.sqlDB.QueryRowContext(ctx, `SELECT MAX(bucket_start) FROM ops_metrics_hourly`).Scan(&value); err != nil {
		return time.Time{}, false, err
	}
	if !value.Valid {
		return time.Time{}, false, nil
	}
	return value.Time.UTC(), true, nil
}

const (
	opsAggHourlyLeaderLockKeyDefault = "ops:aggregation:hourly:leader"
	opsAggDailyLeaderLockKeyDefault  = "ops:aggregation:daily:leader"

	// Hourly upsert can backfill multiple days; keep TTL comfortably above one run.
	opsAggHourlyLeaderLockTTLDefault = 15 * time.Minute
	opsAggDailyLeaderLockTTLDefault  = 10 * time.Minute

	opsAggLeaderLockSkipLogMinInterval = 1 * time.Minute
)

var opsAggLeaderUnlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)

var opsAggLeaderRenewScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

func (s *OpsAggregationService) tryAcquireLeaderLock(ctx context.Context, key string, ttl time.Duration, logPrefix string) (func(), bool) {
	if s == nil || !s.distributedLockOn {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return nil, true
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	if s.rdb == nil {
		s.distributedLockWarn.Do(func() {
			log.Printf("%s distributed lock enabled but redis client is nil; proceeding without leader lock (key=%q)", logPrefix, key)
		})
		return nil, true
	}

	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	token := opsAlertLeaderToken()
	ok, err := s.rdb.SetNX(lockCtx, key, token, ttl).Result()
	if err != nil {
		log.Printf("%s failed to acquire leader lock (key=%q): %v", logPrefix, key, err)
		return nil, false
	}
	if !ok {
		s.logLeaderLockSkipped(logPrefix, key)
		return nil, false
	}

	renewCancel, renewDone := s.startLeaderLockRenewal(key, token, ttl, logPrefix)

	release := func() {
		if renewCancel != nil {
			renewCancel()
		}
		if renewDone != nil {
			select {
			case <-renewDone:
			case <-time.After(2 * time.Second):
				log.Printf("%s leader lock renewal goroutine did not stop in time (key=%q)", logPrefix, key)
			}
		}

		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if _, err := opsAggLeaderUnlockScript.Run(releaseCtx, s.rdb, []string{key}, token).Int(); err != nil {
			log.Printf("%s failed to release leader lock (key=%q token=%s): %v", logPrefix, key, shortenLockToken(token), err)
		}
	}

	return release, true
}

func (s *OpsAggregationService) startLeaderLockRenewal(key string, token string, ttl time.Duration, logPrefix string) (context.CancelFunc, <-chan struct{}) {
	if s == nil || s.rdb == nil {
		return nil, nil
	}
	if strings.TrimSpace(key) == "" || token == "" || ttl <= 0 {
		return nil, nil
	}

	refreshEvery := ttl / 2
	if refreshEvery < 10*time.Second {
		refreshEvery = 10 * time.Second
	}
	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		ttlMillis = 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(refreshEvery)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				res, err := opsAggLeaderRenewScript.Run(context.Background(), s.rdb, []string{key}, token, ttlMillis).Int()
				if err != nil {
					log.Printf("%s leader lock renewal failed (key=%q token=%s): %v", logPrefix, key, shortenLockToken(token), err)
					continue
				}
				if res == 0 {
					log.Printf("%s leader lock no longer owned; stop renewing (key=%q token=%s)", logPrefix, key, shortenLockToken(token))
					return
				}
			}
		}
	}()

	return cancel, done
}

func (s *OpsAggregationService) logLeaderLockSkipped(prefix string, key string) {
	if s == nil {
		return
	}
	now := time.Now()

	s.skipLogMu.Lock()
	defer s.skipLogMu.Unlock()
	if !s.skipLogAt.IsZero() && now.Sub(s.skipLogAt) < opsAggLeaderLockSkipLogMinInterval {
		return
	}
	s.skipLogAt = now
	log.Printf("%s skipped; leader lock held by another instance (key=%q)", prefix, key)
}

func (s *OpsAggregationService) getLatestDailyBucketDate(ctx context.Context) (time.Time, bool, error) {
	var value sql.NullTime
	if err := s.sqlDB.QueryRowContext(ctx, `SELECT MAX(bucket_date) FROM ops_metrics_daily`).Scan(&value); err != nil {
		return time.Time{}, false, err
	}
	if !value.Valid {
		return time.Time{}, false, nil
	}
	t := value.Time
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true, nil
}

func utcFloorToHour(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}

func utcFloorToDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
