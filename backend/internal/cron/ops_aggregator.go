package cron

import (
	"context"
	"log"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type OpsAggregator struct {
	repo service.OpsRepository
	ctx  context.Context
}

func NewOpsAggregator(ctx context.Context, repo service.OpsRepository) *OpsAggregator {
	if ctx == nil {
		ctx = context.Background()
	}
	return &OpsAggregator{
		repo: repo,
		ctx:  ctx,
	}
}

// RunHourly aggregates the previous hour's data into ops_metrics_hourly
func (a *OpsAggregator) RunHourly() {
	log.Println("[CRON] Starting hourly metrics aggregation...")

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()

	// Aggregate the previous hour (e.g., if now is 14:30, aggregate 13:00-14:00)
	now := time.Now().UTC()
	endTime := now.Truncate(time.Hour)
	startTime := endTime.Add(-1 * time.Hour)

	log.Printf("[CRON] Aggregating hourly metrics for [%s, %s)", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	if err := a.repo.UpsertHourlyMetrics(ctx, startTime, endTime); err != nil {
		log.Printf("[CRON] Failed to aggregate hourly metrics: %v", err)
		return
	}

	log.Printf("[CRON] Hourly metrics aggregation completed for [%s, %s)", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
}

// RunDaily aggregates the previous day's data into ops_metrics_daily
func (a *OpsAggregator) RunDaily() {
	log.Println("[CRON] Starting daily metrics aggregation...")

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()

	// Aggregate the previous day (e.g., if now is 2024-01-03 01:00, aggregate 2024-01-02 00:00-2024-01-03 00:00)
	now := time.Now().UTC()
	endTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	startTime := endTime.AddDate(0, 0, -1)

	log.Printf("[CRON] Aggregating daily metrics for [%s, %s)", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	if err := a.repo.UpsertDailyMetrics(ctx, startTime, endTime); err != nil {
		log.Printf("[CRON] Failed to aggregate daily metrics: %v", err)
		return
	}

	log.Printf("[CRON] Daily metrics aggregation completed for [%s, %s)", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
}
