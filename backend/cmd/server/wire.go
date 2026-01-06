//go:build wireinject
// +build wireinject

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
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
	opsScheduledReport *service.OpsScheduledReportService,
	opsGroupAvailability *service.OpsGroupAvailabilityMonitor,
	opsCleanup *service.OpsCleanupService,
) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Cleanup steps in reverse dependency order
		cleanupSteps := make([]struct {
			name string
			fn   func() error
		}, 0, 16)

		if opsCleanup != nil {
			cleanupSteps = append(cleanupSteps, struct {
				name string
				fn   func() error
			}{
				name: "OpsCleanupService",
				fn: func() error {
					opsCleanup.Stop()
					return nil
				},
			})
		}

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "OpsErrorLogWorkers",
			fn: func() error {
				if ok := handler.StopOpsErrorLogWorkers(); !ok {
					return fmt.Errorf("timed out draining ops error log workers")
				}
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "OpsWSQPSCache",
			fn: func() error {
				admin.StopOpsWSQPSCache()
				return nil
			},
		})

		if opsGroupAvailability != nil {
			cleanupSteps = append(cleanupSteps, struct {
				name string
				fn   func() error
			}{
				name: "OpsGroupAvailabilityMonitor",
				fn: func() error {
					opsGroupAvailability.Stop()
					return nil
				},
			})
		}

		if opsScheduledReport != nil {
			cleanupSteps = append(cleanupSteps, struct {
				name string
				fn   func() error
			}{
				name: "OpsScheduledReportService",
				fn: func() error {
					opsScheduledReport.Stop()
					return nil
				},
			})
		}

		if opsAggregation != nil {
			cleanupSteps = append(cleanupSteps, struct {
				name string
				fn   func() error
			}{
				name: "OpsAggregationService",
				fn: func() error {
					opsAggregation.Stop()
					return nil
				},
			})
		}

		if opsMetricsCollector != nil {
			cleanupSteps = append(cleanupSteps, struct {
				name string
				fn   func() error
			}{
				name: "OpsMetricsCollector",
				fn: func() error {
					opsMetricsCollector.Stop()
					return nil
				},
			})
		}

		if opsAlertService != nil {
			cleanupSteps = append(cleanupSteps, struct {
				name string
				fn   func() error
			}{
				name: "OpsAlertService",
				fn: func() error {
					opsAlertService.Stop()
					return nil
				},
			})
		}

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "TokenRefreshService",
			fn: func() error {
				tokenRefresh.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "PricingService",
			fn: func() error {
				pricing.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "EmailQueueService",
			fn: func() error {
				emailQueue.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "BillingCacheService",
			fn: func() error {
				billingCache.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "OAuthService",
			fn: func() error {
				oauth.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "OpenAIOAuthService",
			fn: func() error {
				openaiOAuth.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "GeminiOAuthService",
			fn: func() error {
				geminiOAuth.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "AntigravityOAuthService",
			fn: func() error {
				antigravityOAuth.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "AntigravityQuotaRefresher",
			fn: func() error {
				antigravityQuota.Stop()
				return nil
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "Redis",
			fn: func() error {
				return rdb.Close()
			},
		})

		cleanupSteps = append(cleanupSteps, struct {
			name string
			fn   func() error
		}{
			name: "Ent",
			fn: func() error {
				return entClient.Close()
			},
		})

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
