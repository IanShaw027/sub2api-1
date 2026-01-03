//go:build wireinject
// +build wireinject

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/cron"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

type Application struct {
	Server  *http.Server
	Cleanup func()
}

func initializeApplication(buildInfo handler.BuildInfo) (*Application, error) {
	wire.Build(
		// Infrastructure layer ProviderSets
		config.ProviderSet,

		// Business layer ProviderSets
		repository.ProviderSet,
		service.ProviderSet,
		middleware.ProviderSet,
		handler.ProviderSet,

		// Server layer ProviderSet
		server.ProviderSet,

		// BuildInfo provider
		provideServiceBuildInfo,

		// Cleanup function provider
		provideCleanup,

		// Application struct
		wire.Struct(new(Application), "Server", "Cleanup"),
	)
	return nil, nil
}

func provideServiceBuildInfo(buildInfo handler.BuildInfo) service.BuildInfo {
	return service.BuildInfo{
		Version:   buildInfo.Version,
		BuildType: buildInfo.BuildType,
	}
}

func provideCleanup(
	entClient *ent.Client,
	rdb *redis.Client,
	tokenRefresh *service.TokenRefreshService,
	pricing *service.PricingService,
	emailQueue *service.EmailQueueService,
	billingCache *service.BillingCacheService,
	oauth *service.OAuthService,
	openaiOAuth *service.OpenAIOAuthService,
	geminiOAuth *service.GeminiOAuthService,
	antigravityOAuth *service.AntigravityOAuthService,
	antigravityQuota *service.AntigravityQuotaRefresher,
	opsAggregation *service.OpsAggregationService,
	opsMetricsCollector *service.OpsMetricsCollector,
	opsAlertService *service.OpsAlertService,
	cfg *config.Config,
	opsRepo service.OpsRepository,
) func() {
	if opsAlertService != nil {
		opsAlertService.Start()
	}

	// Initialize cron manager if cleanup is enabled
	var cronManager *cron.Manager
	if cfg.Ops.Cleanup.Enabled || cfg.Ops.Aggregation.Enabled {
		ctx := context.Background()
		cronManager = cron.NewManager(ctx)

		// Register cleanup job
		if cfg.Ops.Cleanup.Enabled {
			cleanupConfig := cron.CleanupConfig{
				ErrorLogRetentionDays:      cfg.Ops.Cleanup.ErrorLogRetentionDays,
				MinuteMetricsRetentionDays: cfg.Ops.Cleanup.MinuteMetricsRetentionDays,
				HourlyMetricsRetentionDays: cfg.Ops.Cleanup.HourlyMetricsRetentionDays,
			}
			opsCleaner := cron.NewOpsCleaner(ctx, opsRepo, cleanupConfig)

			schedule := cfg.Ops.Cleanup.Schedule
			if schedule == "" {
				schedule = "0 2 * * *"
			}

			if err := cronManager.AddJob(schedule, opsCleaner.Run); err != nil {
				log.Printf("[CRON] Failed to add ops cleanup job: %v", err)
			} else {
				log.Printf("[CRON] Ops cleanup job scheduled: %s", schedule)
			}
		}

		// Register aggregation jobs
		if cfg.Ops.Aggregation.Enabled {
			opsAggregator := cron.NewOpsAggregator(ctx, opsRepo)

			// Hourly aggregation
			hourlySchedule := cfg.Ops.Aggregation.HourlySchedule
			if hourlySchedule == "" {
				hourlySchedule = "5 * * * *" // 5 minutes past every hour
			}
			if err := cronManager.AddJob(hourlySchedule, opsAggregator.RunHourly); err != nil {
				log.Printf("[CRON] Failed to add hourly aggregation job: %v", err)
			} else {
				log.Printf("[CRON] Hourly aggregation job scheduled: %s", hourlySchedule)
			}

			// Daily aggregation
			dailySchedule := cfg.Ops.Aggregation.DailySchedule
			if dailySchedule == "" {
				dailySchedule = "10 0 * * *" // 10 minutes past midnight
			}
			if err := cronManager.AddJob(dailySchedule, opsAggregator.RunDaily); err != nil {
				log.Printf("[CRON] Failed to add daily aggregation job: %v", err)
			} else {
				log.Printf("[CRON] Daily aggregation job scheduled: %s", dailySchedule)
			}
		}

		if cronManager != nil {
			cronManager.Start()
		}
	}

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Stop cron manager first
		if cronManager != nil {
			cronManager.Stop()
		}

		// Cleanup steps in reverse dependency order
		cleanupSteps := []struct {
			name string
			fn   func() error
		}{
			{"OpsAggregationService", func() error {
				opsAggregation.Stop()
				return nil
			}},
			{"OpsMetricsCollector", func() error {
				opsMetricsCollector.Stop()
				return nil
			}},
			{"OpsAlertService", func() error {
				opsAlertService.Stop()
				return nil
			}},
			{"TokenRefreshService", func() error {
				tokenRefresh.Stop()
				return nil
			}},
			{"PricingService", func() error {
				pricing.Stop()
				return nil
			}},
			{"EmailQueueService", func() error {
				emailQueue.Stop()
				return nil
			}},
			{"BillingCacheService", func() error {
				billingCache.Stop()
				return nil
			}},
			{"OAuthService", func() error {
				oauth.Stop()
				return nil
			}},
			{"OpenAIOAuthService", func() error {
				openaiOAuth.Stop()
				return nil
			}},
			{"GeminiOAuthService", func() error {
				geminiOAuth.Stop()
				return nil
			}},
			{"AntigravityOAuthService", func() error {
				antigravityOAuth.Stop()
				return nil
			}},
			{"AntigravityQuotaRefresher", func() error {
				antigravityQuota.Stop()
				return nil
			}},
			{"Redis", func() error {
				return rdb.Close()
			}},
			{"Ent", func() error {
				return entClient.Close()
			}},
		}

		for _, step := range cleanupSteps {
			if err := step.fn(); err != nil {
				log.Printf("[Cleanup] %s failed: %v", step.name, err)
				// Continue with remaining cleanup steps even if one fails
			} else {
				log.Printf("[Cleanup] %s succeeded", step.name)
			}
		}

		// Check if context timed out
		select {
		case <-ctx.Done():
			log.Printf("[Cleanup] Warning: cleanup timed out after 10 seconds")
		default:
			log.Printf("[Cleanup] All cleanup steps completed")
		}
	}
}
