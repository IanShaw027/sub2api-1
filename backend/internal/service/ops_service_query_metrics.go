package service

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *OpsQueryService) GetWindowStats(ctx context.Context, startTime, endTime time.Time) (*OpsWindowStats, error) {
	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	return s.repo.GetWindowStats(ctxDB, startTime, endTime)
}

func (s *OpsQueryService) GetWindowStatsGrouped(ctx context.Context, startTime, endTime time.Time, groupBy string) ([]*OpsWindowStatsGroupedItem, error) {
	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	return s.repo.GetWindowStatsGrouped(ctxDB, startTime, endTime, groupBy)
}

func (s *OpsQueryService) GetLatestMetrics(ctx context.Context) (*OpsMetrics, error) {
	// Cache first (best-effort): cache errors should not break the dashboard.
	if s != nil {
		if repo := s.repo; repo != nil {
			if cached, err := repo.GetCachedLatestSystemMetric(ctx); err == nil && cached != nil {
				if cached.WindowMinutes == 0 {
					cached.WindowMinutes = 1
				}
				return cached, nil
			}
		}
	}

	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	metric, err := s.repo.GetLatestSystemMetric(ctxDB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &OpsMetrics{WindowMinutes: 1}, nil
		}
		return nil, err
	}
	if metric == nil {
		return &OpsMetrics{WindowMinutes: 1}, nil
	}
	if metric.WindowMinutes == 0 {
		metric.WindowMinutes = 1
	}

	// Backfill cache (best-effort).
	if s != nil {
		if repo := s.repo; repo != nil {
			_ = repo.SetCachedLatestSystemMetric(ctx, metric)
		}
	}
	return metric, nil
}

func (s *OpsQueryService) ListMetricsHistory(ctx context.Context, windowMinutes int, startTime, endTime time.Time, limit int) ([]OpsMetrics, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	if windowMinutes <= 0 {
		windowMinutes = 1
	}
	if limit <= 0 || limit > 5000 {
		limit = 300
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.IsZero() {
		startTime = endTime.Add(-time.Duration(limit) * opsMetricsInterval)
	}
	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}

	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	return s.repo.ListSystemMetricsRange(ctxDB, windowMinutes, startTime, endTime, limit)
}
