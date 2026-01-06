package service

import (
	"log"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// BuildInfo contains build information
type BuildInfo struct {
	Version   string
	BuildType string
}

// ProvidePricingService creates and initializes PricingService
func ProvidePricingService(cfg *config.Config, remoteClient PricingRemoteClient) (*PricingService, error) {
	svc := NewPricingService(cfg, remoteClient)
	if err := svc.Initialize(); err != nil {
		// Pricing service initialization failure should not block startup, use fallback prices
		println("[Service] Warning: Pricing service initialization failed:", err.Error())
	}
	return svc, nil
}

// ProvideUpdateService creates UpdateService with BuildInfo
func ProvideUpdateService(cache UpdateCache, githubClient GitHubReleaseClient, buildInfo BuildInfo) *UpdateService {
	return NewUpdateService(cache, githubClient, buildInfo.Version, buildInfo.BuildType)
}

// ProvideEmailQueueService creates EmailQueueService with default worker count
func ProvideEmailQueueService(emailService *EmailService) *EmailQueueService {
	return NewEmailQueueService(emailService, 3)
}

// ProvideTokenRefreshService creates and starts TokenRefreshService
func ProvideTokenRefreshService(
	accountRepo AccountRepository,
	oauthService *OAuthService,
	openaiOAuthService *OpenAIOAuthService,
	geminiOAuthService *GeminiOAuthService,
	antigravityOAuthService *AntigravityOAuthService,
	cfg *config.Config,
) *TokenRefreshService {
	svc := NewTokenRefreshService(accountRepo, oauthService, openaiOAuthService, geminiOAuthService, antigravityOAuthService, cfg)
	svc.Start()
	return svc
}

// ProvideTimingWheelService creates and starts TimingWheelService
func ProvideTimingWheelService() *TimingWheelService {
	svc := NewTimingWheelService()
	svc.Start()
	return svc
}

// ProvideDeferredService creates and starts DeferredService
func ProvideDeferredService(accountRepo AccountRepository, timingWheel *TimingWheelService) *DeferredService {
	svc := NewDeferredService(accountRepo, timingWheel, 10*time.Second)
	svc.Start()
	return svc
}

// ProvideOpsMetricsCollector creates and starts OpsMetricsCollector.
func ProvideOpsMetricsCollector(
	opsService *OpsService,
	concurrencyService *ConcurrencyService,
	accountRepo AccountRepository,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsMetricsCollector {
	if cfg != nil && !cfg.Ops.Enabled {
		return nil
	}
	svc := NewOpsMetricsCollector(opsService, concurrencyService, accountRepo, redisClient, cfg)
	svc.Start()
	return svc
}

// ProvideOpsAlertService creates and starts OpsAlertService.
func ProvideOpsAlertService(
	opsService *OpsService,
	userService *UserService,
	emailService *EmailService,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsAlertService {
	if cfg != nil && !cfg.Ops.Enabled {
		return nil
	}
	svc := NewOpsAlertService(opsService, userService, emailService, redisClient, cfg)
	svc.Start()
	return svc
}

// ProvideOpsAggregationService creates and starts OpsAggregationService.
func ProvideOpsAggregationService(opsService *OpsService, repo OpsRepository, redisClient *redis.Client, cfg *config.Config) *OpsAggregationService {
	if cfg != nil && !cfg.Ops.Enabled {
		return nil
	}
	svc := NewOpsAggregationService(repo, redisClient)
	svc.ops = opsService
	if cfg != nil && cfg.RunMode == config.RunModeSimple {
		svc.distributedLockOn = false
	}
	if cfg != nil && cfg.Ops.Aggregation.Enabled {
		if !cfg.Ops.UsePreaggregatedTables {
			log.Printf("[OpsAggregation] aggregation enabled but ops.use_preaggregated_tables=false; skipping background pre-aggregation")
			return svc
		}
		svc.Start()
	}
	return svc
}

// ProvideOpsScheduledReportService creates and starts OpsScheduledReportService.
func ProvideOpsScheduledReportService(opsService *OpsService, userService *UserService, emailService *EmailService, redisClient *redis.Client, cfg *config.Config) *OpsScheduledReportService {
	if cfg != nil && !cfg.Ops.Enabled {
		return nil
	}
	svc := NewOpsScheduledReportService(opsService, userService, emailService, redisClient, cfg)
	svc.Start()
	return svc
}

// ProvideOpsGroupAvailabilityMonitor creates and starts OpsGroupAvailabilityMonitor.
func ProvideOpsGroupAvailabilityMonitor(
	opsService *OpsService,
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	emailService *EmailService,
	userService *UserService,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsGroupAvailabilityMonitor {
	if cfg != nil && !cfg.Ops.Enabled {
		return nil
	}
	svc := NewOpsGroupAvailabilityMonitor(opsService, accountRepo, groupRepo, emailService, userService, redisClient, cfg)
	svc.Start()
	return svc
}

// ProvideOpsCleanupService creates and starts OpsCleanupService.
func ProvideOpsCleanupService(repo OpsRepository, redisClient *redis.Client, cfg *config.Config) *OpsCleanupService {
	if cfg != nil && !cfg.Ops.Enabled {
		return nil
	}
	svc := NewOpsCleanupService(repo, redisClient, cfg)
	svc.Start()
	return svc
}

// ProvideConcurrencyService creates ConcurrencyService and starts slot cleanup worker.
func ProvideConcurrencyService(cache ConcurrencyCache, accountRepo AccountRepository, cfg *config.Config) *ConcurrencyService {
	svc := NewConcurrencyService(cache)
	if cfg != nil {
		svc.StartSlotCleanupWorker(accountRepo, cfg.Gateway.Scheduling.SlotCleanupInterval)
	}
	return svc
}

// ProviderSet is the Wire provider set for all services
var ProviderSet = wire.NewSet(
	// Core services
	NewAuthService,
	NewUserService,
	NewAPIKeyService,
	NewGroupService,
	NewAccountService,
	NewProxyService,
	NewRedeemService,
	NewUsageService,
	NewDashboardService,
	ProvidePricingService,
	NewBillingService,
	NewBillingCacheService,
	NewAdminService,
	NewGatewayService,
	NewOpenAIGatewayService,
	NewOAuthService,
	NewOpenAIOAuthService,
	NewGeminiOAuthService,
	NewGeminiQuotaService,
	NewAntigravityOAuthService,
	NewGeminiTokenProvider,
	NewGeminiMessagesCompatService,
	NewAntigravityTokenProvider,
	NewAntigravityGatewayService,
	NewRateLimitService,
	NewAccountUsageService,
	NewAccountTestService,
	NewSettingService,
	NewOpsService,
	NewEmailService,
	ProvideEmailQueueService,
	NewTurnstileService,
	NewSubscriptionService,
	ProvideConcurrencyService,
	NewIdentityService,
	NewCRSSyncService,
	ProvideUpdateService,
	ProvideTokenRefreshService,
	ProvideTimingWheelService,
	ProvideDeferredService,
	NewAntigravityQuotaFetcher,
	ProvideOpsMetricsCollector,
	ProvideOpsAlertService,
	ProvideOpsAggregationService,
	ProvideOpsScheduledReportService,
	ProvideOpsGroupAvailabilityMonitor,
	ProvideOpsCleanupService,
	NewUserAttributeService,
	NewUsageCache,
)
