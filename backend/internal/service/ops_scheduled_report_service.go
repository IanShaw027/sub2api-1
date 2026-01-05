package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type OpsScheduledReportService struct {
	opsService   *OpsService
	userService  *UserService
	emailService *EmailService
	httpClient   *http.Client
	redisClient  *redis.Client

	distributedLockOn   bool
	distributedLockWarn sync.Once
	skipLogMu           sync.Mutex
	skipLogAt           time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	stopCtx   context.Context
	stop      context.CancelFunc
	wg        sync.WaitGroup
}

type ScheduledReport struct {
	ID            int64
	Name          string
	ReportType    string
	TimeRange     string
	Schedule      string
	NotifyEmail   bool
	Enabled       bool
	LastRunAt     *time.Time
	NextRunAt     time.Time
}

func NewOpsScheduledReportService(opsService *OpsService, userService *UserService, emailService *EmailService, redisClient *redis.Client) *OpsScheduledReportService {
	return &OpsScheduledReportService{
		opsService:   opsService,
		userService:  userService,
		emailService: emailService,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		redisClient:  redisClient,

		distributedLockOn: true,
	}
}

func (s *OpsScheduledReportService) Start() {
	s.StartWithContext(context.Background())
}

func (s *OpsScheduledReportService) StartWithContext(ctx context.Context) {
	if s == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.startOnce.Do(func() {
		s.stopCtx, s.stop = context.WithCancel(ctx)
		s.wg.Add(1)
		go s.run()
	})
}

func (s *OpsScheduledReportService) Stop() {
	if s == nil {
		return
	}

	s.stopOnce.Do(func() {
		if s.stop != nil {
			s.stop()
		}
	})
	s.wg.Wait()
}

func (s *OpsScheduledReportService) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	s.checkAndRunReports()
	for {
		select {
		case <-ticker.C:
			s.checkAndRunReports()
		case <-s.stopCtx.Done():
			return
		}
	}
}

func (s *OpsScheduledReportService) checkAndRunReports() {
	ctx, cancel := context.WithTimeout(s.stopCtx, 30*time.Second)
	defer cancel()

	releaseLeaderLock, ok := s.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if releaseLeaderLock != nil {
		defer releaseLeaderLock()
	}

	reports, err := s.listScheduledReports(ctx)
	if err != nil {
		log.Printf("[ScheduledReport] failed to list reports: %v", err)
		return
	}

	now := time.Now()
	for _, report := range reports {
		if !report.Enabled {
			continue
		}
		if report.NextRunAt.After(now) {
			continue
		}

		s.runReport(ctx, &report, now)
	}
}

const (
	opsScheduledReportLeaderLockKeyDefault = "ops:scheduled_reports:leader"
	opsScheduledReportLeaderLockTTLDefault = 5 * time.Minute

	opsScheduledReportSkipLogMinInterval = 1 * time.Minute
)

var opsScheduledReportLeaderUnlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)

var opsScheduledReportLeaderRenewScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

func (s *OpsScheduledReportService) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil || !s.distributedLockOn {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if s.redisClient == nil {
		s.distributedLockWarn.Do(func() {
			log.Printf("[ScheduledReport] distributed lock enabled but redis client is nil; proceeding without leader lock (key=%q)", opsScheduledReportLeaderLockKeyDefault)
		})
		return nil, true
	}

	key := strings.TrimSpace(opsScheduledReportLeaderLockKeyDefault)
	ttl := opsScheduledReportLeaderLockTTLDefault
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	token := opsAlertLeaderToken()
	ok, err := s.redisClient.SetNX(lockCtx, key, token, ttl).Result()
	if err != nil {
		log.Printf("[ScheduledReport] failed to acquire leader lock (key=%q): %v", key, err)
		return nil, false
	}
	if !ok {
		s.logLeaderLockSkipped(key)
		return nil, false
	}

	renewCancel, renewDone := s.startLeaderLockRenewal(key, token, ttl)

	release := func() {
		if renewCancel != nil {
			renewCancel()
		}
		if renewDone != nil {
			select {
			case <-renewDone:
			case <-time.After(2 * time.Second):
				log.Printf("[ScheduledReport] leader lock renewal goroutine did not stop in time (key=%q)", key)
			}
		}

		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := opsScheduledReportLeaderUnlockScript.Run(releaseCtx, s.redisClient, []string{key}, token).Int(); err != nil {
			log.Printf("[ScheduledReport] failed to release leader lock (key=%q token=%s): %v", key, shortenLockToken(token), err)
		}
	}

	return release, true
}

func (s *OpsScheduledReportService) startLeaderLockRenewal(key string, token string, ttl time.Duration) (context.CancelFunc, <-chan struct{}) {
	if s == nil || s.redisClient == nil {
		return nil, nil
	}
	if strings.TrimSpace(key) == "" || token == "" || ttl <= 0 {
		return nil, nil
	}

	refreshEvery := ttl / 2
	if refreshEvery < 15*time.Second {
		refreshEvery = 15 * time.Second
	}
	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		ttlMillis = 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(refreshEvery)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				res, err := opsScheduledReportLeaderRenewScript.Run(context.Background(), s.redisClient, []string{key}, token, ttlMillis).Int()
				if err != nil {
					log.Printf("[ScheduledReport] leader lock renewal failed (key=%q token=%s): %v", key, shortenLockToken(token), err)
					continue
				}
				if res == 0 {
					log.Printf("[ScheduledReport] leader lock no longer owned; stop renewing (key=%q token=%s)", key, shortenLockToken(token))
					return
				}
			}
		}
	}()

	return cancel, done
}

func (s *OpsScheduledReportService) logLeaderLockSkipped(key string) {
	if s == nil {
		return
	}
	now := time.Now()
	s.skipLogMu.Lock()
	defer s.skipLogMu.Unlock()
	if !s.skipLogAt.IsZero() && now.Sub(s.skipLogAt) < opsScheduledReportSkipLogMinInterval {
		return
	}
	s.skipLogAt = now
	log.Printf("[ScheduledReport] skipped run; leader lock held by another instance (key=%q)", key)
}

func (s *OpsScheduledReportService) runReport(ctx context.Context, report *ScheduledReport, now time.Time) {
	content, err := s.generateReportContent(ctx, report.ReportType, report.TimeRange)
	if err != nil {
		log.Printf("[ScheduledReport] failed to generate report %s: %v", report.Name, err)
		return
	}

	emailSent := false
	if report.NotifyEmail {
		emailSent = s.sendReportEmail(ctx, report, content)
	}

	if emailSent {
		log.Printf("[ScheduledReport] report %s sent (email=%v)", report.Name, emailSent)
	}
}

func (s *OpsScheduledReportService) generateReportContent(ctx context.Context, reportType string, timeRange string) (map[string]any, error) {
	if s.opsService == nil {
		return nil, fmt.Errorf("ops service not initialized")
	}

	duration, err := parseTimeRange(timeRange)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	startTime := now.Add(-duration)

	switch reportType {
	case "daily_summary":
		return s.generateDailySummary(ctx, startTime, now)
	case "weekly_summary":
		return s.generateWeeklySummary(ctx, startTime, now)
	case "error_digest":
		return s.generateErrorDigest(ctx, startTime, now)
	case "account_health":
		return s.generateAccountHealth(ctx, startTime, now)
	default:
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}
}

func (s *OpsScheduledReportService) generateDailySummary(ctx context.Context, startTime, endTime time.Time) (map[string]any, error) {
	stats, err := s.opsService.GetWindowStats(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	totalReqs := stats.SuccessCount + stats.ErrorCount
	successRate := 0.0
	if totalReqs > 0 {
		successRate = float64(stats.SuccessCount) / float64(totalReqs) * 100
	}

	return map[string]any{
		"report_type":    "daily_summary",
		"period_start":   startTime.Format(time.RFC3339),
		"period_end":     endTime.Format(time.RFC3339),
		"total_requests": totalReqs,
		"success_count":  stats.SuccessCount,
		"error_count":    stats.ErrorCount,
		"success_rate":   fmt.Sprintf("%.2f%%", successRate),
		"p50_latency_ms": stats.P50LatencyMs,
		"p99_latency_ms": stats.P99LatencyMs,
		"timeout_count":  stats.TimeoutCount,
	}, nil
}

func (s *OpsScheduledReportService) generateWeeklySummary(ctx context.Context, startTime, endTime time.Time) (map[string]any, error) {
	stats, err := s.opsService.GetWindowStats(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	totalReqs := stats.SuccessCount + stats.ErrorCount
	successRate := 0.0
	if totalReqs > 0 {
		successRate = float64(stats.SuccessCount) / float64(totalReqs) * 100
	}

	return map[string]any{
		"report_type":    "weekly_summary",
		"period_start":   startTime.Format(time.RFC3339),
		"period_end":     endTime.Format(time.RFC3339),
		"total_requests": totalReqs,
		"success_rate":   fmt.Sprintf("%.2f%%", successRate),
		"error_count":    stats.ErrorCount,
		"avg_latency_ms": stats.AvgLatencyMs,
		"p99_latency_ms": stats.P99LatencyMs,
	}, nil
}

func (s *OpsScheduledReportService) generateErrorDigest(ctx context.Context, startTime, endTime time.Time) (map[string]any, error) {
	filters := OpsErrorLogFilters{
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     100,
	}

	logs, _, err := s.opsService.ListErrorLogs(ctx, filters)
	if err != nil {
		return nil, err
	}

	errorsByType := make(map[string]int)
	for _, log := range logs {
		errorsByType[log.Type]++
	}

	return map[string]any{
		"report_type":    "error_digest",
		"period_start":   startTime.Format(time.RFC3339),
		"period_end":     endTime.Format(time.RFC3339),
		"total_errors":   len(logs),
		"errors_by_type": errorsByType,
		"recent_errors":  logs[:min(10, len(logs))],
	}, nil
}

func (s *OpsScheduledReportService) generateAccountHealth(ctx context.Context, startTime, endTime time.Time) (map[string]any, error) {
	if s.opsService == nil {
		return nil, fmt.Errorf("ops service not initialized")
	}

	// Get all active account status (including 24h stats)
	accountStatuses, err := s.opsService.GetAllActiveAccountStatus(ctx)
	if err != nil {
		return nil, err
	}

	healthyCount := 0
	unhealthyAccounts := []map[string]any{}

	for _, status := range accountStatuses {
		stats := status.Stats24h
		totalReqs := stats.SuccessCount + stats.ErrorCount
		if totalReqs == 0 {
			continue
		}

		errorRate := float64(stats.ErrorCount) / float64(totalReqs) * 100
		if errorRate > 10 {
			unhealthyAccounts = append(unhealthyAccounts, map[string]any{
				"account_id":  status.AccountID,
				"error_rate":  fmt.Sprintf("%.2f%%", errorRate),
				"error_count": stats.ErrorCount,
			})
		} else {
			healthyCount++
		}
	}

	return map[string]any{
		"report_type":        "account_health",
		"period_start":       startTime.Format(time.RFC3339),
		"period_end":         endTime.Format(time.RFC3339),
		"total_accounts":     len(accountStatuses),
		"healthy_accounts":   healthyCount,
		"unhealthy_accounts": unhealthyAccounts,
	}, nil
}

func (s *OpsScheduledReportService) sendReportEmail(ctx context.Context, report *ScheduledReport, content map[string]any) bool {
	if s.emailService == nil || s.userService == nil {
		return false
	}

	admin, err := s.userService.GetFirstAdmin(ctx)
	if err != nil || admin == nil || admin.Email == "" {
		return false
	}

	config, err := s.emailService.GetSMTPConfig(ctx)
	if err != nil {
		log.Printf("[ScheduledReport] email config load failed: %v", err)
		return false
	}

	reportData := buildReportData(content)
	templateData := EmailTemplateData{
		Type:      "report",
		Title:     report.Name,
		LogoURL:   "https://your-site.com/logo.png",
		SiteName:  "Sub2API",
		SiteURL:   "https://your-site.com",
		Year:      time.Now().Year(),
		ActionURL: "https://your-site.com/admin/ops/reports",
		Report:    reportData,
	}

	subject := fmt.Sprintf("[Scheduled Report] %s", report.Name)

	if err := s.emailService.SendTemplatedEmail(config, admin.Email, subject, templateData); err != nil {
		log.Printf("[ScheduledReport] email send failed: %v", err)
		return false
	}

	return true
}

func buildReportData(content map[string]any) *ReportData {
	reportData := &ReportData{
		Date:  time.Now().Format("2006-01-02"),
		Stats: []StatItem{},
	}

	if totalReqs, ok := content["total_requests"].(int); ok {
		reportData.Stats = append(reportData.Stats, StatItem{
			Label: "总请求数",
			Value: fmt.Sprintf("%d", totalReqs),
		})
	}
	if successRate, ok := content["success_rate"].(string); ok {
		reportData.Stats = append(reportData.Stats, StatItem{
			Label: "成功率",
			Value: successRate,
		})
	}
	if p99, ok := content["p99_latency_ms"].(float64); ok {
		reportData.Stats = append(reportData.Stats, StatItem{
			Label: "P99延迟",
			Value: fmt.Sprintf("%.0fms", p99),
		})
	}

	return reportData
}

func (s *OpsScheduledReportService) listScheduledReports(ctx context.Context) ([]ScheduledReport, error) {
	// TODO: 从数据库读取配置
	// 目前返回空列表，等待数据库表创建
	return []ScheduledReport{}, nil
}

func formatReportEmail(content map[string]any) string {
	body := "Scheduled Report\n\n"
	for k, v := range content {
		body += fmt.Sprintf("%s: %v\n", k, v)
	}
	return body
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
