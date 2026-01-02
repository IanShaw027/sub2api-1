package service

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"
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

	hourlyInterval time.Duration
	dailyInterval  time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	wg        sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once

	hourlyMu sync.Mutex
	dailyMu  sync.Mutex
}

func NewOpsAggregationService(repo OpsRepository, sqlDB *sql.DB) *OpsAggregationService {
	ctx, cancel := context.WithCancel(context.Background())
	return &OpsAggregationService{
		repo:           repo,
		sqlDB:          sqlDB,
		hourlyInterval: opsAggHourlyInterval,
		dailyInterval:  opsAggDailyInterval,
		ctx:            ctx,
		cancel:         cancel,
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
