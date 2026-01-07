package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

type OpsScheduledReportService struct {
	opsService   *OpsService
	userService  *UserService
	emailService *EmailService
	redisClient  *redis.Client

	distributedLockOn   bool
	distributedLockWarn sync.Once
	skipLogMu           sync.Mutex
	skipLogAt           time.Time

	opsDisabledWarn sync.Once

	startOnce sync.Once
	stopOnce  sync.Once
	stopCtx   context.Context
	stop      context.CancelFunc
	wg        sync.WaitGroup
}

type ScheduledReport struct {
	ID                              int64
	Name                            string
	ReportType                      string
	TimeRange                       string
	Schedule                        string
	NotifyEmail                     bool
	Recipients                      []string
	ErrorDigestMinCount             int
	AccountHealthErrorRateThreshold float64
	Enabled                         bool
	LastRunAt                       *time.Time
	NextRunAt                       time.Time
}

func NewOpsScheduledReportService(opsService *OpsService, userService *UserService, emailService *EmailService, redisClient *redis.Client, cfg *config.Config) *OpsScheduledReportService {
	lockOn := true
	if cfg != nil && cfg.RunMode == config.RunModeSimple {
		lockOn = false
	}
	return &OpsScheduledReportService{
		opsService:   opsService,
		userService:  userService,
		emailService: emailService,
		redisClient:  redisClient,

		distributedLockOn: lockOn,
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

	ticker := time.NewTicker(1 * time.Minute)
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

	if s.opsService != nil && !s.opsService.IsOpsMonitoringEnabled(ctx) {
		s.opsDisabledWarn.Do(func() {
			log.Printf("[ScheduledReport] ops monitoring disabled; skipping scheduled reports")
		})
		return
	}

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
	if len(reports) == 0 {
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

const (
	opsScheduledReportLastRunKeyPrefix = "ops:scheduled_reports:last_run:"
)

var opsScheduledReportCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func (s *OpsScheduledReportService) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil || !s.distributedLockOn {
		return nil, true
	}

	key := strings.TrimSpace(opsScheduledReportLeaderLockKeyDefault)
	ttl := opsScheduledReportLeaderLockTTLDefault
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	opts := RedisLeaderLockOptions{
		Enabled:         true,
		Redis:           s.redisClient,
		Key:             key,
		TTL:             ttl,
		LogPrefix:       "[ScheduledReport]",
		WarnNoRedisOnce: &s.distributedLockWarn,
		OnSkip: func() {
			s.logLeaderLockSkipped(key)
		},
		LogAcquired:      false,
		LogReleased:      false,
		MinRenewInterval: 15 * time.Second,
	}
	return TryAcquireRedisLeaderLock(ctx, opts)
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
	// Mark as "run" up-front so a misconfigured SMTP setup doesn't spam retries every minute.
	s.setLastRunAt(ctx, report.ReportType, now)

	content, err := s.generateReportContentForReport(ctx, report, now)
	if err != nil {
		log.Printf("[ScheduledReport] failed to generate report %s: %v", report.Name, err)
		return
	}

	if report.ReportType == "error_digest" && report.ErrorDigestMinCount > 0 {
		if total, ok := content["total_errors"].(int); ok && total < report.ErrorDigestMinCount {
			return
		}
	}

	emailSent := false
	if report.NotifyEmail {
		emailSent = s.sendReportEmail(ctx, report, content)
	}

	if emailSent {
		log.Printf("[ScheduledReport] report %s sent (email=%v)", report.Name, emailSent)
	}
}

func (s *OpsScheduledReportService) generateReportContentForReport(ctx context.Context, report *ScheduledReport, now time.Time) (map[string]any, error) {
	if s.opsService == nil {
		return nil, fmt.Errorf("ops service not initialized")
	}
	if report == nil {
		return nil, fmt.Errorf("report is nil")
	}

	duration, err := parseTimeRange(report.TimeRange)
	if err != nil {
		return nil, err
	}

	startTime := now.Add(-duration)

	switch report.ReportType {
	case "daily_summary":
		return s.generateDailySummary(ctx, startTime, now)
	case "weekly_summary":
		return s.generateWeeklySummary(ctx, startTime, now)
	case "error_digest":
		return s.generateErrorDigest(ctx, startTime, now)
	case "account_health":
		threshold := report.AccountHealthErrorRateThreshold
		if threshold <= 0 || threshold > 100 {
			threshold = 10
		}
		return s.generateAccountHealth(ctx, startTime, now, threshold)
	default:
		return nil, fmt.Errorf("unknown report type: %s", report.ReportType)
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
	filter := &ErrorLogFilter{
		StartTime: &startTime,
		EndTime:   &endTime,
		Page:      1,
		PageSize:  100,
	}

	out, err := s.opsService.GetErrorLogs(ctx, filter)
	if err != nil {
		return nil, err
	}
	logs := out.Errors

	errorsByType := make(map[string]int)
	for _, log := range logs {
		if log == nil {
			continue
		}
		errorsByType[log.Type]++
	}

	recent := logs
	if len(recent) > 10 {
		recent = recent[:10]
	}

	return map[string]any{
		"report_type":    "error_digest",
		"period_start":   startTime.Format(time.RFC3339),
		"period_end":     endTime.Format(time.RFC3339),
		"total_errors":   len(logs),
		"errors_by_type": errorsByType,
		"recent_errors":  recent,
	}, nil
}

func (s *OpsScheduledReportService) generateAccountHealth(ctx context.Context, startTime, endTime time.Time, errorRateThreshold float64) (map[string]any, error) {
	if s.opsService == nil {
		return nil, fmt.Errorf("ops service not initialized")
	}

	// Get all active account status (including 24h stats)
	accountStatuses, err := s.opsService.GetAllActiveAccountStatus(ctx, "", 0)
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
		if errorRate > errorRateThreshold {
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

	config, err := s.emailService.GetSMTPConfig(ctx)
	if err != nil {
		log.Printf("[ScheduledReport] email config load failed: %v", err)
		return false
	}

	recipients := normalizeEmails(report.Recipients)
	if len(recipients) == 0 {
		admin, err := s.userService.GetFirstAdmin(ctx)
		if err != nil || admin == nil || strings.TrimSpace(admin.Email) == "" {
			return false
		}
		recipients = []string{strings.TrimSpace(admin.Email)}
	}

	reportData := buildReportData(content)
	branding := s.emailService.GetBranding(ctx)
	actionURL := joinSiteURL(branding.SiteURL, "/admin/ops")

	templateData := EmailTemplateData{
		Type:      "report",
		Title:     report.Name,
		Year:      time.Now().Year(),
		LogoURL:   branding.LogoURL,
		SiteName:  branding.SiteName,
		SiteURL:   branding.SiteURL,
		ActionURL: actionURL,
		ActionText: func() string {
			if actionURL == "" {
				return ""
			}
			return "打开运维监控"
		}(),
		Report: reportData,
	}

	subject := fmt.Sprintf("[Scheduled Report] %s", report.Name)

	anySent := false
	for _, to := range recipients {
		if err := s.emailService.SendTemplatedEmail(config, to, subject, templateData); err != nil {
			log.Printf("[ScheduledReport] email send failed (to=%s): %v", to, err)
			continue
		}
		anySent = true
	}

	return anySent
}

func buildReportData(content map[string]any) *ReportData {
	reportData := &ReportData{
		Date:  time.Now().Format("2006-01-02"),
		Stats: []StatItem{},
	}

	if v, ok := content["total_requests"]; ok {
		switch vv := v.(type) {
		case int64:
			reportData.Stats = append(reportData.Stats, StatItem{Label: "总请求数", Value: fmt.Sprintf("%d", vv)})
		case int:
			reportData.Stats = append(reportData.Stats, StatItem{Label: "总请求数", Value: fmt.Sprintf("%d", vv)})
		case float64:
			reportData.Stats = append(reportData.Stats, StatItem{Label: "总请求数", Value: fmt.Sprintf("%.0f", vv)})
		}
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
	if s == nil || s.opsService == nil {
		return []ScheduledReport{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	emailCfg, err := s.opsService.GetEmailNotificationConfig(ctx)
	if err != nil {
		return nil, err
	}
	if emailCfg == nil || !emailCfg.Report.Enabled {
		return []ScheduledReport{}, nil
	}

	recipients, err := resolveOpsReportEmailRecipients(ctx, s.userService, emailCfg)
	if err != nil {
		log.Printf("[ScheduledReport] resolve recipients failed: %v", err)
	}

	type reportDef struct {
		enabled   bool
		name      string
		kind      string
		timeRange string
		schedule  string
	}

	defs := []reportDef{
		{enabled: emailCfg.Report.DailySummaryEnabled, name: "日报", kind: "daily_summary", timeRange: "24h", schedule: emailCfg.Report.DailySummarySchedule},
		{enabled: emailCfg.Report.WeeklySummaryEnabled, name: "周报", kind: "weekly_summary", timeRange: "7d", schedule: emailCfg.Report.WeeklySummarySchedule},
		{enabled: emailCfg.Report.ErrorDigestEnabled, name: "错误摘要", kind: "error_digest", timeRange: "24h", schedule: emailCfg.Report.ErrorDigestSchedule},
		{enabled: emailCfg.Report.AccountHealthEnabled, name: "账号健康", kind: "account_health", timeRange: "24h", schedule: emailCfg.Report.AccountHealthSchedule},
	}

	now := time.Now()
	reports := make([]ScheduledReport, 0, len(defs))

	for idx, d := range defs {
		if !d.enabled {
			continue
		}
		spec := strings.TrimSpace(d.schedule)
		if spec == "" {
			continue
		}
		sched, err := opsScheduledReportCronParser.Parse(spec)
		if err != nil {
			log.Printf("[ScheduledReport] invalid cron spec=%q for %s: %v", spec, d.kind, err)
			continue
		}

		lastRun := s.getLastRunAt(ctx, d.kind)
		base := lastRun
		if base.IsZero() {
			// Allow a schedule matching the current minute to trigger immediately after startup.
			base = now.Add(-1 * time.Minute)
		}

		next := sched.Next(base)
		if next.IsZero() {
			continue
		}

		var lastRunPtr *time.Time
		if !lastRun.IsZero() {
			lastRunPtr = &lastRun
		}

		reports = append(reports, ScheduledReport{
			ID:                              int64(idx + 1),
			Name:                            d.name,
			ReportType:                      d.kind,
			TimeRange:                       d.timeRange,
			Schedule:                        spec,
			NotifyEmail:                     true,
			Recipients:                      recipients,
			ErrorDigestMinCount:             emailCfg.Report.ErrorDigestMinCount,
			AccountHealthErrorRateThreshold: emailCfg.Report.AccountHealthErrorRateThreshold,
			Enabled:                         true,
			LastRunAt:                       lastRunPtr,
			NextRunAt:                       next,
		})
	}

	return reports, nil
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

func (s *OpsScheduledReportService) getLastRunAt(ctx context.Context, reportType string) time.Time {
	if s == nil || s.redisClient == nil {
		return time.Time{}
	}
	key := opsScheduledReportLastRunKeyPrefix + strings.TrimSpace(reportType)
	if strings.TrimSpace(reportType) == "" {
		return time.Time{}
	}
	raw, err := s.redisClient.Get(ctx, key).Result()
	if err != nil || strings.TrimSpace(raw) == "" {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

func (s *OpsScheduledReportService) setLastRunAt(ctx context.Context, reportType string, at time.Time) {
	if s == nil || s.redisClient == nil {
		return
	}
	rt := strings.TrimSpace(reportType)
	if rt == "" {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	key := opsScheduledReportLastRunKeyPrefix + rt
	// Keep for ~90 days; this is only a dedupe hint.
	_ = s.redisClient.Set(ctx, key, strconv.FormatInt(at.Unix(), 10), 90*24*time.Hour).Err()
}
