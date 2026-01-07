//go:build integration

package service

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This integration test protects the DI startup contract for OpsAlertService.
//
// Background:
//   - OpsMetricsCollector previously called alertService.Start()/Evaluate() directly.
//   - Those direct calls were removed, so OpsAlertService must now start via DI
//     (ProvideOpsAlertService in wire.go) and run its own evaluation ticker.
//
// What we validate here:
//  1. When we construct via the Wire provider functions (ProvideOpsAlertService +
//     ProvideOpsMetricsCollector), OpsAlertService starts automatically.
//  2. Its evaluation loop continues to tick even if OpsMetricsCollector is stopped,
//     proving the alert evaluator is independent.
//  3. The evaluation path can trigger alert logic (CreateAlertEvent called).
func TestOpsAlertService_StartedViaWireProviders_RunsIndependentTicker(t *testing.T) {
	oldInterval := opsAlertEvalInterval
	opsAlertEvalInterval = 25 * time.Millisecond
	t.Cleanup(func() { opsAlertEvalInterval = oldInterval })

	repo := newFakeOpsRepository()
	opsService := NewOpsService(repo, nil, nil, nil)

	// Start via the Wire provider function (the production DI path).
	alertService := ProvideOpsAlertService(opsService, nil, nil, nil, nil)
	t.Cleanup(alertService.Stop)

	// Construct via ProvideOpsMetricsCollector (wire.go). Stop immediately to ensure
	// the alert ticker keeps running without the metrics collector.
	collector := ProvideOpsMetricsCollector(opsService, NewConcurrencyService(nil), nil, nil, nil)
	collector.Stop()

	// Wait for at least one evaluation (run() calls evaluateOnce immediately).
	require.Eventually(t, func() bool {
		return repo.listRulesCalls.Load() >= 1
	}, 1*time.Second, 5*time.Millisecond)

	// Confirm the evaluation loop keeps ticking after the metrics collector is stopped.
	callsAfterCollectorStop := repo.listRulesCalls.Load()
	require.Eventually(t, func() bool {
		return repo.listRulesCalls.Load() >= callsAfterCollectorStop+2
	}, 1*time.Second, 5*time.Millisecond)

	// Confirm the evaluation logic actually fires an alert event at least once.
	select {
	case <-repo.eventCreatedCh:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatalf("expected OpsAlertService to create an alert event, but none was created (ListAlertRules calls=%d)", repo.listRulesCalls.Load())
	}
}

func newFakeOpsRepository() *fakeOpsRepository {
	return &fakeOpsRepository{
		eventCreatedCh: make(chan struct{}),
	}
}

type noopOpsRepository struct{}

func (noopOpsRepository) CreateAlertRule(ctx context.Context, rule *OpsAlertRule) error { return nil }
func (noopOpsRepository) UpdateAlertRule(ctx context.Context, rule *OpsAlertRule) error { return nil }
func (noopOpsRepository) DeleteAlertRule(ctx context.Context, id int64) error           { return nil }

func (noopOpsRepository) CreateGroupAvailabilityConfig(ctx context.Context, config *OpsGroupAvailabilityConfig) error {
	return nil
}
func (noopOpsRepository) UpdateGroupAvailabilityConfig(ctx context.Context, config *OpsGroupAvailabilityConfig) error {
	return nil
}
func (noopOpsRepository) DeleteGroupAvailabilityConfig(ctx context.Context, groupID int64) error {
	return nil
}
func (noopOpsRepository) CreateGroupAvailabilityEvent(ctx context.Context, event *OpsGroupAvailabilityEvent) error {
	return nil
}
func (noopOpsRepository) UpdateGroupAvailabilityEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	return nil
}
func (noopOpsRepository) UpdateGroupAvailabilityEventNotifications(ctx context.Context, eventID int64, emailSent bool) error {
	return nil
}

func (noopOpsRepository) GetLatestHourlyBucketStart(ctx context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (noopOpsRepository) GetLatestDailyBucketDate(ctx context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (noopOpsRepository) DeleteOldErrorLogs(ctx context.Context, retentionDays int) (int64, error) {
	return 0, nil
}
func (noopOpsRepository) DeleteOldMetrics(ctx context.Context, windowMinutes int, retentionDays int) (int64, error) {
	return 0, nil
}

func (noopOpsRepository) GetErrorLogByID(ctx context.Context, id int64) (*OpsErrorLog, error) {
	return nil, nil
}
func (noopOpsRepository) ListRequestDetails(ctx context.Context, filter *OpsRequestDetailFilter) ([]*OpsRequestDetail, int64, error) {
	return nil, 0, nil
}
func (noopOpsRepository) GetTokenTPS(ctx context.Context, startTime, endTime time.Time) (current, peak, avg float64, err error) {
	return 0, 0, 0, nil
}
func (noopOpsRepository) ListAlertEvents(ctx context.Context, limit int) ([]OpsAlertEvent, error) {
	return nil, nil
}
func (noopOpsRepository) ListGroupAvailabilityConfigs(ctx context.Context, enabledOnly bool) ([]OpsGroupAvailabilityConfig, error) {
	return nil, nil
}
func (noopOpsRepository) GetGroupAvailabilityConfig(ctx context.Context, groupID int64) (*OpsGroupAvailabilityConfig, error) {
	return nil, nil
}
func (noopOpsRepository) GetActiveGroupAvailabilityEvent(ctx context.Context, configID int64) (*OpsGroupAvailabilityEvent, error) {
	return nil, nil
}
func (noopOpsRepository) GetLatestGroupAvailabilityEvent(ctx context.Context, configID int64) (*OpsGroupAvailabilityEvent, error) {
	return nil, nil
}
func (noopOpsRepository) ListGroupAvailabilityEvents(ctx context.Context, limit int, status string) ([]OpsGroupAvailabilityEvent, error) {
	return nil, nil
}
func (noopOpsRepository) GetAllActiveAccountStatus(ctx context.Context, platform string, groupID int64) ([]AccountStatusSummary, error) {
	return nil, nil
}
func (noopOpsRepository) GetErrorStatsByIP(ctx context.Context, startTime, endTime time.Time, limit int, sortBy, sortOrder string) ([]IPErrorStats, error) {
	return nil, nil
}
func (noopOpsRepository) GetErrorsByIP(ctx context.Context, ip string, startTime, endTime time.Time, page, pageSize int) ([]OpsErrorLog, int64, error) {
	return nil, 0, nil
}
func (noopOpsRepository) GetRetentionConfig(ctx context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}

// fakeOpsRepository is a lightweight in-memory stub of OpsRepository for integration tests.
// It avoids real DB/Redis usage and provides deterministic responses fast.
type fakeOpsRepository struct {
	noopOpsRepository

	listRulesCalls atomic.Int64

	mu             sync.Mutex
	activeEvent    *OpsAlertEvent
	latestEvent    *OpsAlertEvent
	nextEventID    int64
	eventCreatedCh chan struct{}
	eventOnce      sync.Once
}

func (r *fakeOpsRepository) CreateErrorLog(ctx context.Context, log *OpsErrorLog) error {
	return nil
}

func (r *fakeOpsRepository) ListErrorLogs(ctx context.Context, filter *ErrorLogFilter) ([]*OpsErrorLog, int64, error) {
	return nil, 0, nil
}

func (r *fakeOpsRepository) GetLatestSystemMetric(ctx context.Context) (*OpsMetrics, error) {
	return &OpsMetrics{WindowMinutes: 1}, sql.ErrNoRows
}

func (r *fakeOpsRepository) CreateSystemMetric(ctx context.Context, metric *OpsMetrics) error {
	return nil
}

func (r *fakeOpsRepository) GetWindowStats(ctx context.Context, startTime, endTime time.Time) (*OpsWindowStats, error) {
	return &OpsWindowStats{}, nil
}

func (r *fakeOpsRepository) GetWindowStatsGrouped(ctx context.Context, startTime, endTime time.Time, groupBy string) ([]*OpsWindowStatsGroupedItem, error) {
	return nil, nil
}

func (r *fakeOpsRepository) GetProviderStats(ctx context.Context, startTime, endTime time.Time) ([]*ProviderStats, error) {
	return nil, nil
}

func (r *fakeOpsRepository) GetLatencyHistogram(ctx context.Context, startTime, endTime time.Time) ([]*LatencyHistogramItem, error) {
	return nil, nil
}

func (r *fakeOpsRepository) GetErrorDistribution(ctx context.Context, startTime, endTime time.Time) ([]*ErrorDistributionItem, error) {
	return nil, nil
}

func (r *fakeOpsRepository) ListRecentSystemMetrics(ctx context.Context, windowMinutes, limit int) ([]OpsMetrics, error) {
	if limit <= 0 {
		limit = 1
	}
	now := time.Now()
	metrics := make([]OpsMetrics, 0, limit)
	for i := 0; i < limit; i++ {
		metrics = append(metrics, OpsMetrics{
			WindowMinutes:   windowMinutes,
			CPUUsagePercent: 99,
			UpdatedAt:       now.Add(-time.Duration(i) * opsMetricsInterval),
		})
	}
	return metrics, nil
}

func (r *fakeOpsRepository) ListSystemMetricsRange(ctx context.Context, windowMinutes int, startTime, endTime time.Time, limit int) ([]OpsMetrics, error) {
	return nil, nil
}

func (r *fakeOpsRepository) ListAlertRules(ctx context.Context) ([]OpsAlertRule, error) {
	call := r.listRulesCalls.Add(1)
	// Delay enabling rules slightly so the test can stop OpsMetricsCollector first,
	// then observe the alert evaluator ticking independently.
	if call < 5 {
		return nil, nil
	}
	return []OpsAlertRule{
		{
			ID:               1,
			Name:             "cpu too high (test)",
			Enabled:          true,
			MetricType:       OpsMetricCPUUsagePercent,
			Operator:         ">",
			Threshold:        0,
			WindowMinutes:    1,
			SustainedMinutes: 1,
			Severity:         "P1",
			NotifyEmail:      false,
			CooldownMinutes:  0,
		},
	}, nil
}

func (r *fakeOpsRepository) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeEvent == nil {
		return nil, nil
	}
	if r.activeEvent.RuleID != ruleID {
		return nil, nil
	}
	if r.activeEvent.Status != OpsAlertStatusFiring {
		return nil, nil
	}
	clone := *r.activeEvent
	return &clone, nil
}

func (r *fakeOpsRepository) GetLatestAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latestEvent == nil || r.latestEvent.RuleID != ruleID {
		return nil, nil
	}
	clone := *r.latestEvent
	return &clone, nil
}

func (r *fakeOpsRepository) CreateAlertEvent(ctx context.Context, event *OpsAlertEvent) error {
	if event == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextEventID++
	event.ID = r.nextEventID

	clone := *event
	r.latestEvent = &clone
	if clone.Status == OpsAlertStatusFiring {
		r.activeEvent = &clone
	}

	r.eventOnce.Do(func() { close(r.eventCreatedCh) })
	return nil
}

func (r *fakeOpsRepository) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeEvent != nil && r.activeEvent.ID == eventID {
		r.activeEvent.Status = status
		r.activeEvent.ResolvedAt = resolvedAt
	}
	if r.latestEvent != nil && r.latestEvent.ID == eventID {
		r.latestEvent.Status = status
		r.latestEvent.ResolvedAt = resolvedAt
	}
	return nil
}

func (r *fakeOpsRepository) UpdateAlertEventNotifications(ctx context.Context, eventID int64, emailSent bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeEvent != nil && r.activeEvent.ID == eventID {
		r.activeEvent.EmailSent = emailSent
	}
	if r.latestEvent != nil && r.latestEvent.ID == eventID {
		r.latestEvent.EmailSent = emailSent
	}
	return nil
}

func (r *fakeOpsRepository) CountActiveAlerts(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeEvent == nil {
		return 0, nil
	}
	return 1, nil
}

func (r *fakeOpsRepository) GetOverviewStats(ctx context.Context, startTime, endTime time.Time) (*OverviewStats, error) {
	return &OverviewStats{}, nil
}

func (r *fakeOpsRepository) UpsertHourlyMetrics(ctx context.Context, startTime, endTime time.Time) error {
	return nil
}

func (r *fakeOpsRepository) UpsertDailyMetrics(ctx context.Context, startTime, endTime time.Time) error {
	return nil
}

func (r *fakeOpsRepository) GetCachedLatestSystemMetric(ctx context.Context) (*OpsMetrics, error) {
	return nil, nil
}

func (r *fakeOpsRepository) SetCachedLatestSystemMetric(ctx context.Context, metric *OpsMetrics) error {
	return nil
}

func (r *fakeOpsRepository) GetCachedDashboardOverview(ctx context.Context, timeRange string) (*DashboardOverviewData, error) {
	return nil, nil
}

func (r *fakeOpsRepository) SetCachedDashboardOverview(ctx context.Context, timeRange string, data *DashboardOverviewData, ttl time.Duration) error {
	return nil
}

func (r *fakeOpsRepository) PingRedis(ctx context.Context) error {
	return nil
}

func (r *fakeOpsRepository) CountAvailableAccountsByGroup(ctx context.Context, groupID int64) (available, total int, err error) {
	return 0, 0, nil
}

func (r *fakeOpsRepository) GetCachedPlatformConcurrency(ctx context.Context) (map[string]*PlatformConcurrencyInfo, error) {
	return map[string]*PlatformConcurrencyInfo{}, nil
}

func (r *fakeOpsRepository) GetCachedGroupConcurrency(ctx context.Context) (map[int64]*GroupConcurrencyInfo, error) {
	return map[int64]*GroupConcurrencyInfo{}, nil
}

func (r *fakeOpsRepository) GetCachedConcurrencyCollectedAt(ctx context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
