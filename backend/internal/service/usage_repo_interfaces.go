package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// UsageLogWriter is the minimal dependency needed by gateway forwarding paths.
// Keeping this interface small prevents unrelated stats/report methods from leaking into hot paths and tests.
type UsageLogWriter interface {
	Create(ctx context.Context, log *UsageLog) error
}

// DashboardUsageRepository is the minimal set of read operations required by the admin dashboard service.
type DashboardUsageRepository interface {
	GetDashboardStats(ctx context.Context) (*usagestats.DashboardStats, error)
	GetUsageTrendWithFilters(ctx context.Context, startTime, endTime time.Time, granularity string, userID, apiKeyID int64) ([]usagestats.TrendDataPoint, error)
	GetModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID int64) ([]usagestats.ModelStat, error)
	GetAPIKeyUsageTrend(ctx context.Context, startTime, endTime time.Time, granularity string, limit int) ([]usagestats.APIKeyUsageTrendPoint, error)
	GetUserUsageTrend(ctx context.Context, startTime, endTime time.Time, granularity string, limit int) ([]usagestats.UserUsageTrendPoint, error)
	GetBatchUserUsageStats(ctx context.Context, userIDs []int64) (map[int64]*usagestats.BatchUserUsageStats, error)
	GetBatchAPIKeyUsageStats(ctx context.Context, apiKeyIDs []int64) (map[int64]*usagestats.BatchAPIKeyUsageStats, error)
}

// RateLimitUsageRepository is the minimal read surface required for proactive quota checks.
type RateLimitUsageRepository interface {
	GetModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID int64) ([]usagestats.ModelStat, error)
}

// AccountUsageRepository is the minimal set of reads required by account usage inspection endpoints.
type AccountUsageRepository interface {
	GetModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID int64) ([]usagestats.ModelStat, error)
	GetAccountWindowStats(ctx context.Context, accountID int64, startTime time.Time) (*usagestats.AccountStats, error)
	GetAccountTodayStats(ctx context.Context, accountID int64) (*usagestats.AccountStats, error)
	GetAccountUsageStats(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.AccountUsageStatsResponse, error)
}
