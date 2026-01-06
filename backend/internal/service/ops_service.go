package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"golang.org/x/sync/singleflight"
)

type OpsService struct {
	// Core dependencies used by ops-related sub-services and other ops modules.
	// Split responsibilities:
	// - OpsIngestService: write/collection/ingestion paths.
	// - OpsQueryService: dashboard/query paths.
	// - OpsSettingsService: settings persistence (runtime settings, notifications).
	*OpsIngestService
	*OpsQueryService
	*OpsSettingsService
}

const opsDBQueryTimeout = 5 * time.Second
const opsDashboardLocalCacheTTL = 10 * time.Second
const opsMonitoringEnabledCacheTTL = 5 * time.Second

type OpsIngestService struct {
	repo         OpsIngestRepository
	sqlDB        *sql.DB
	isOpsEnabled func(context.Context) bool
}

type OpsQueryService struct {
	repo         OpsQueryRepository
	sqlDB        *sql.DB
	isOpsEnabled func(context.Context) bool

	redisNilWarnOnce sync.Once
	dbNilWarnOnce    sync.Once

	dashboardSF singleflight.Group

	dashboardOverviewCacheMu sync.Mutex
	dashboardOverviewCache   map[string]dashboardOverviewCacheEntry

	providerHealthCacheMu sync.Mutex
	providerHealthCache   map[string]providerHealthCacheEntry

	latencyHistogramCacheMu sync.Mutex
	latencyHistogramCache   map[string]latencyHistogramCacheEntry

	errorDistributionCacheMu sync.Mutex
	errorDistributionCache   map[string]errorDistributionCacheEntry
}

type OpsSettingsService struct {
	settingRepo SettingRepository

	opsEnabledMu        sync.Mutex
	opsEnabledExpiresAt time.Time
	opsEnabledCached    bool
	opsEnabledWarnOnce  sync.Once

	realtimeEnabledMu        sync.Mutex
	realtimeEnabledExpiresAt time.Time
	realtimeEnabledCached    bool
	realtimeEnabledWarnOnce  sync.Once
}

func (s *OpsService) IsOpsMonitoringEnabled(ctx context.Context) bool {
	if s == nil {
		return false
	}
	if s.OpsSettingsService == nil {
		// Preserve "default open" behavior when settings are not available.
		return true
	}
	return s.OpsSettingsService.IsOpsMonitoringEnabled(ctx)
}

func (s *OpsSettingsService) IsOpsMonitoringEnabled(ctx context.Context) bool {
	if s == nil {
		return false
	}

	// When we cannot consult settings (e.g., during tests or early bootstrap),
	// default to enabled to preserve existing behavior ("default open").
	if s.settingRepo == nil {
		return true
	}

	now := time.Now()
	s.opsEnabledMu.Lock()
	if now.Before(s.opsEnabledExpiresAt) {
		enabled := s.opsEnabledCached
		s.opsEnabledMu.Unlock()
		return enabled
	}
	s.opsEnabledMu.Unlock()

	enabled := true
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOpsMonitoringEnabled)
	if err == nil {
		enabled = strings.TrimSpace(value) != "false"
	} else if errors.Is(err, ErrSettingNotFound) {
		enabled = true
	} else {
		s.opsEnabledWarnOnce.Do(func() {
			log.Printf("[OpsService][WARN] failed to read %q from settings; defaulting to enabled: %v", SettingKeyOpsMonitoringEnabled, err)
		})
	}

	s.opsEnabledMu.Lock()
	s.opsEnabledCached = enabled
	s.opsEnabledExpiresAt = now.Add(opsMonitoringEnabledCacheTTL)
	s.opsEnabledMu.Unlock()

	return enabled
}

func (s *OpsSettingsService) IsRealtimeMonitoringEnabled(ctx context.Context) bool {
	if s == nil {
		return false
	}

	if s.settingRepo == nil {
		return true
	}

	now := time.Now()
	s.realtimeEnabledMu.Lock()
	if now.Before(s.realtimeEnabledExpiresAt) {
		enabled := s.realtimeEnabledCached
		s.realtimeEnabledMu.Unlock()
		return enabled
	}
	s.realtimeEnabledMu.Unlock()

	enabled := true
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOpsRealtimeMonitoringEnabled)
	if err == nil {
		enabled = strings.TrimSpace(value) != "false"
	} else if errors.Is(err, ErrSettingNotFound) {
		enabled = true
	} else {
		s.realtimeEnabledWarnOnce.Do(func() {
			log.Printf("[OpsService][WARN] failed to read %q from settings; defaulting to enabled: %v", SettingKeyOpsRealtimeMonitoringEnabled, err)
		})
	}

	s.realtimeEnabledMu.Lock()
	s.realtimeEnabledCached = enabled
	s.realtimeEnabledExpiresAt = now.Add(opsMonitoringEnabledCacheTTL)
	s.realtimeEnabledMu.Unlock()

	return enabled
}

func NewOpsService(repo OpsRepository, sqlDB *sql.DB, cfg *config.Config, settingRepo SettingRepository) *OpsService {
	_ = cfg

	svc := &OpsService{}
	svc.OpsSettingsService = &OpsSettingsService{
		settingRepo: settingRepo,
	}
	svc.OpsIngestService = &OpsIngestService{
		repo:         repo,
		sqlDB:        sqlDB,
		isOpsEnabled: svc.IsOpsMonitoringEnabled,
	}
	svc.OpsQueryService = &OpsQueryService{
		repo:         repo,
		sqlDB:        sqlDB,
		isOpsEnabled: svc.IsOpsMonitoringEnabled,

		dashboardOverviewCache: make(map[string]dashboardOverviewCacheEntry),
		providerHealthCache:    make(map[string]providerHealthCacheEntry),
		latencyHistogramCache:  make(map[string]latencyHistogramCacheEntry),
		errorDistributionCache: make(map[string]errorDistributionCacheEntry),
	}
	// Best-effort startup health checks: log warnings if Redis/DB is unavailable,
	// but never fail service startup (graceful degradation).
	log.Printf("[OpsService] Performing startup health checks...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	redisStatus := svc.OpsQueryService.checkRedisHealth(ctx)
	dbStatus := svc.OpsQueryService.checkDatabaseHealth(ctx)

	log.Printf("[OpsService] Startup health check complete: Redis=%s, Database=%s", redisStatus, dbStatus)
	if redisStatus == "critical" || dbStatus == "critical" {
		log.Printf("[OpsService][WARN] Service starting with degraded dependencies - some features may be unavailable")
	}

	return svc
}
