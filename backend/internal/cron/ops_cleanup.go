package cron

import (
	"context"
	"log"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type CleanupConfig struct {
	ErrorLogRetentionDays      int
	MinuteMetricsRetentionDays int
	HourlyMetricsRetentionDays int
}

type OpsCleaner struct {
	repo   service.OpsRepository
	config CleanupConfig
	ctx    context.Context
}

func NewOpsCleaner(ctx context.Context, repo service.OpsRepository, config CleanupConfig) *OpsCleaner {
	if ctx == nil {
		ctx = context.Background()
	}
	return &OpsCleaner{
		repo:   repo,
		config: config,
		ctx:    ctx,
	}
}

func (c *OpsCleaner) Run() {
	log.Println("[CRON] Starting ops data cleanup...")

	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Minute)
	defer cancel()

	// Clean error logs
	if c.config.ErrorLogRetentionDays > 0 {
		deleted, err := c.repo.DeleteOldErrorLogs(ctx, c.config.ErrorLogRetentionDays)
		if err != nil {
			log.Printf("[CRON] Failed to clean error logs: %v", err)
		} else {
			log.Printf("[CRON] Cleaned %d error logs older than %d days", deleted, c.config.ErrorLogRetentionDays)
		}
	}

	// Clean minute-level metrics
	if err := ctx.Err(); err != nil {
		log.Printf("[CRON] Ops data cleanup canceled: %v", err)
		return
	}
	if c.config.MinuteMetricsRetentionDays > 0 {
		deleted, err := c.repo.DeleteOldMetrics(ctx, 1, c.config.MinuteMetricsRetentionDays)
		if err != nil {
			log.Printf("[CRON] Failed to clean minute metrics: %v", err)
		} else {
			log.Printf("[CRON] Cleaned %d minute metrics older than %d days", deleted, c.config.MinuteMetricsRetentionDays)
		}
	}

	// Clean hourly metrics
	if err := ctx.Err(); err != nil {
		log.Printf("[CRON] Ops data cleanup canceled: %v", err)
		return
	}
	if c.config.HourlyMetricsRetentionDays > 0 {
		deleted, err := c.repo.DeleteOldMetrics(ctx, 60, c.config.HourlyMetricsRetentionDays)
		if err != nil {
			log.Printf("[CRON] Failed to clean hourly metrics: %v", err)
		} else {
			log.Printf("[CRON] Cleaned %d hourly metrics older than %d days", deleted, c.config.HourlyMetricsRetentionDays)
		}
	}

	log.Println("[CRON] Ops data cleanup completed")
}
