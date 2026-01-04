package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
)

type OpsAlertService struct {
	opsService   *OpsService
	userService  *UserService
	emailService *EmailService
	httpClient   *http.Client

	interval time.Duration

	redisClient          *redis.Client
	distributedLockOn    bool
	distributedLockKey   string
	distributedLockTTL   time.Duration
	distributedLockWarn  sync.Once
	distributedSkipLogMu sync.Mutex
	distributedSkipLogAt time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	stopCtx   context.Context
	stop      context.CancelFunc
	wg        sync.WaitGroup
}

// opsAlertEvalInterval defines how often OpsAlertService evaluates alert rules.
//
// Production uses opsMetricsInterval. Tests may override this variable to keep
// integration tests fast without changing production defaults.
var opsAlertEvalInterval = opsMetricsInterval

func NewOpsAlertService(
	opsService *OpsService,
	userService *UserService,
	emailService *EmailService,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsAlertService {
	lockOn := true
	lockKey := opsAlertLeaderLockKeyDefault
	lockTTL := opsAlertLeaderLockTTLDefault
	if cfg != nil {
		lockOn = cfg.Ops.Alert.DistributedLockEnabled
		if strings.TrimSpace(cfg.Ops.Alert.DistributedLockKey) != "" {
			lockKey = strings.TrimSpace(cfg.Ops.Alert.DistributedLockKey)
		}
		if cfg.Ops.Alert.DistributedLockTTL > 0 {
			lockTTL = cfg.Ops.Alert.DistributedLockTTL
		}
	}

	return &OpsAlertService{
		opsService:   opsService,
		userService:  userService,
		emailService: emailService,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		interval:     opsAlertEvalInterval,
		redisClient:  redisClient,

		distributedLockOn:  lockOn,
		distributedLockKey: lockKey,
		distributedLockTTL: lockTTL,
	}
}

// Start launches the background alert evaluation loop.
//
// Stop must be called during shutdown to ensure the goroutine exits.
func (s *OpsAlertService) Start() {
	s.StartWithContext(context.Background())
}

// StartWithContext is like Start but allows the caller to provide a parent context.
// When the parent context is canceled, the service stops automatically.
func (s *OpsAlertService) StartWithContext(ctx context.Context) {
	if s == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.startOnce.Do(func() {
		if s.interval <= 0 {
			s.interval = opsAlertEvalInterval
		}

		s.stopCtx, s.stop = context.WithCancel(ctx)
		s.wg.Add(1)
		go s.run()
	})
}

// Stop gracefully stops the background goroutine started by Start/StartWithContext.
// It is safe to call Stop multiple times.
func (s *OpsAlertService) Stop() {
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

func (s *OpsAlertService) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.evaluateOnce()
	for {
		select {
		case <-ticker.C:
			s.evaluateOnce()
		case <-s.stopCtx.Done():
			return
		}
	}
}

func (s *OpsAlertService) evaluateOnce() {
	ctx, cancel := context.WithTimeout(s.stopCtx, opsAlertEvaluateTimeout)
	defer cancel()

	s.Evaluate(ctx, time.Now())
}

func (s *OpsAlertService) Evaluate(ctx context.Context, now time.Time) {
	if s == nil || s.opsService == nil {
		return
	}

	releaseLeaderLock, ok := s.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if releaseLeaderLock != nil {
		defer releaseLeaderLock()
	}

	rules, err := s.opsService.ListAlertRules(ctx)
	if err != nil {
		log.Printf("[OpsAlert] failed to list rules: %v", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	maxSustainedByWindow := make(map[int]int)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		window := rule.WindowMinutes
		if window <= 0 {
			window = 1
		}
		sustained := rule.SustainedMinutes
		if sustained <= 0 {
			sustained = 1
		}
		if sustained > maxSustainedByWindow[window] {
			maxSustainedByWindow[window] = sustained
		}
	}

	metricsByWindow := make(map[int][]OpsMetrics)
	for window, limit := range maxSustainedByWindow {
		metrics, err := s.opsService.ListRecentSystemMetrics(ctx, window, limit)
		if err != nil {
			log.Printf("[OpsAlert] failed to load metrics window=%dm: %v", window, err)
			continue
		}
		metricsByWindow[window] = metrics
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		window := rule.WindowMinutes
		if window <= 0 {
			window = 1
		}
		sustained := rule.SustainedMinutes
		if sustained <= 0 {
			sustained = 1
		}

		metrics := metricsByWindow[window]
		selected, ok := selectContiguousMetrics(metrics, sustained, now)
		if !ok {
			continue
		}

		breached, latestValue, ok := evaluateRule(rule, selected)
		if !ok {
			continue
		}

		activeEvent, err := s.opsService.GetActiveAlertEvent(ctx, rule.ID)
		if err != nil {
			log.Printf("[OpsAlert] failed to get active event (rule=%d): %v", rule.ID, err)
			continue
		}

		if breached {
			if activeEvent != nil {
				continue
			}

			lastEvent, err := s.opsService.GetLatestAlertEvent(ctx, rule.ID)
			if err != nil {
				log.Printf("[OpsAlert] failed to get latest event (rule=%d): %v", rule.ID, err)
				continue
			}
			if lastEvent != nil && rule.CooldownMinutes > 0 {
				cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
				if now.Sub(lastEvent.FiredAt) < cooldown {
					continue
				}
			}

			event := &OpsAlertEvent{
				RuleID:         rule.ID,
				Severity:       rule.Severity,
				Status:         OpsAlertStatusFiring,
				Title:          fmt.Sprintf("%s: %s", rule.Severity, rule.Name),
				Description:    buildAlertDescription(rule, latestValue),
				MetricValue:    latestValue,
				ThresholdValue: rule.Threshold,
				FiredAt:        now,
				CreatedAt:      now,
			}

			if err := s.opsService.CreateAlertEvent(ctx, event); err != nil {
				log.Printf("[OpsAlert] failed to create event (rule=%d): %v", rule.ID, err)
				continue
			}

			emailSent := s.dispatchNotifications(ctx, rule, event)
			if emailSent {
				if err := s.opsService.UpdateAlertEventNotifications(ctx, event.ID, emailSent); err != nil {
					log.Printf("[OpsAlert] failed to update notification flags (event=%d): %v", event.ID, err)
				}
			}
		} else if activeEvent != nil {
			resolvedAt := now
			if err := s.opsService.UpdateAlertEventStatus(ctx, activeEvent.ID, OpsAlertStatusResolved, &resolvedAt); err != nil {
				log.Printf("[OpsAlert] failed to resolve event (event=%d): %v", activeEvent.ID, err)
			}
		}
	}
}

const (
	opsAlertLeaderLockKeyDefault = "ops:alert:leader"

	// opsAlertLeaderLockTTLDefault is the base TTL for the leader lock.
	// We renew periodically while evaluating, so this is mainly a safety net to
	// recover if the leader crashes mid-run.
	opsAlertLeaderLockTTLDefault = 30 * time.Second

	opsAlertLeaderLockSkipLogMinInterval = 1 * time.Minute
)

var opsAlertLeaderUnlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)

var opsAlertLeaderRenewScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

func (s *OpsAlertService) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil || !s.distributedLockOn {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	key := strings.TrimSpace(s.distributedLockKey)
	if key == "" {
		key = opsAlertLeaderLockKeyDefault
	}
	ttl := s.distributedLockTTL
	if ttl <= 0 {
		ttl = opsAlertLeaderLockTTLDefault
	}

	if s.redisClient == nil {
		s.distributedLockWarn.Do(func() {
			log.Printf("[OpsAlert] distributed lock enabled but redis client is nil; proceeding without leader lock (key=%q)", key)
		})
		return nil, true
	}

	token := opsAlertLeaderToken()
	ok, err := s.redisClient.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		log.Printf("[OpsAlert] failed to acquire leader lock (key=%q): %v", key, err)
		return nil, false
	}
	if !ok {
		s.logLeaderLockSkipped(key)
		return nil, false
	}

	log.Printf("[OpsAlert] acquired leader lock (key=%q ttl=%s token=%s)", key, ttl, shortenLockToken(token))

	renewCancel, renewDone := s.startLeaderLockRenewal(key, token, ttl)

	release := func() {
		if renewCancel != nil {
			renewCancel()
		}
		if renewDone != nil {
			select {
			case <-renewDone:
			case <-time.After(2 * time.Second):
				log.Printf("[OpsAlert] leader lock renewal goroutine did not stop in time (key=%q)", key)
			}
		}

		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		res, err := opsAlertLeaderUnlockScript.Run(releaseCtx, s.redisClient, []string{key}, token).Int()
		if err != nil {
			log.Printf("[OpsAlert] failed to release leader lock (key=%q token=%s): %v", key, shortenLockToken(token), err)
			return
		}
		if res == 1 {
			log.Printf("[OpsAlert] released leader lock (key=%q token=%s)", key, shortenLockToken(token))
		}
	}

	return release, true
}

func (s *OpsAlertService) startLeaderLockRenewal(key string, token string, ttl time.Duration) (context.CancelFunc, <-chan struct{}) {
	if s == nil || s.redisClient == nil {
		return nil, nil
	}
	if strings.TrimSpace(key) == "" || token == "" || ttl <= 0 {
		return nil, nil
	}

	refreshEvery := ttl / 2
	if refreshEvery < 5*time.Second {
		refreshEvery = 5 * time.Second
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
				renewCtx, renewCancel := context.WithTimeout(context.Background(), 2*time.Second)
				res, err := opsAlertLeaderRenewScript.Run(renewCtx, s.redisClient, []string{key}, token, ttlMillis).Int()
				renewCancel()
				if err != nil {
					log.Printf("[OpsAlert] leader lock renewal failed (key=%q token=%s): %v", key, shortenLockToken(token), err)
					continue
				}
				if res == 0 {
					log.Printf("[OpsAlert] leader lock no longer owned; stop renewing (key=%q token=%s)", key, shortenLockToken(token))
					return
				}
			}
		}
	}()

	return cancel, done
}

func (s *OpsAlertService) logLeaderLockSkipped(key string) {
	if s == nil {
		return
	}
	s.distributedSkipLogMu.Lock()
	defer s.distributedSkipLogMu.Unlock()

	now := time.Now()
	if !s.distributedSkipLogAt.IsZero() && now.Sub(s.distributedSkipLogAt) < opsAlertLeaderLockSkipLogMinInterval {
		return
	}
	s.distributedSkipLogAt = now
	log.Printf("[OpsAlert] skipped evaluation; leader lock held by another instance (key=%q)", key)
}

func opsAlertLeaderToken() string {
	host, _ := os.Hostname()
	pid := os.Getpid()

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s:%d:%d", host, pid, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s:%d:%s", host, pid, hex.EncodeToString(buf))
}

func shortenLockToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	const maxLen = 10
	if len(token) <= maxLen {
		return token
	}
	return token[:maxLen]
}

const opsMetricsContinuityTolerance = 20 * time.Second

// selectContiguousMetrics picks the newest N metrics and verifies they are continuous.
//
// This prevents a sustained rule from triggering when metrics sampling has gaps
// (e.g. collector downtime) and avoids evaluating "stale" data.
//
// Assumptions:
// - Metrics are ordered by UpdatedAt DESC (newest first).
// - Metrics are expected to be collected at opsMetricsInterval cadence.
func selectContiguousMetrics(metrics []OpsMetrics, needed int, now time.Time) ([]OpsMetrics, bool) {
	if needed <= 0 {
		return nil, false
	}
	if len(metrics) < needed {
		return nil, false
	}
	newest := metrics[0].UpdatedAt
	if newest.IsZero() {
		return nil, false
	}
	if now.Sub(newest) > opsMetricsInterval+opsMetricsContinuityTolerance {
		return nil, false
	}

	selected := metrics[:needed]
	for i := 0; i < len(selected)-1; i++ {
		a := selected[i].UpdatedAt
		b := selected[i+1].UpdatedAt
		if a.IsZero() || b.IsZero() {
			return nil, false
		}
		gap := a.Sub(b)
		if gap < opsMetricsInterval-opsMetricsContinuityTolerance || gap > opsMetricsInterval+opsMetricsContinuityTolerance {
			return nil, false
		}
	}
	return selected, true
}

func evaluateRule(rule OpsAlertRule, metrics []OpsMetrics) (bool, float64, bool) {
	if len(metrics) == 0 {
		return false, 0, false
	}

	// 如果没有 alert_category，使用传统逻辑（向后兼容）
	if rule.AlertCategory == "" {
		return evaluateRuleLegacy(rule, metrics)
	}

	// 根据 alert_category 选择评估逻辑
	switch rule.AlertCategory {
	case "error_rate", "error_count", "latency", "availability":
		return evaluateRuleLegacy(rule, metrics)
	case "account_status":
		// account_status 类型暂时使用传统逻辑，后续可扩展
		return evaluateRuleLegacy(rule, metrics)
	default:
		return evaluateRuleLegacy(rule, metrics)
	}
}

func evaluateRuleLegacy(rule OpsAlertRule, metrics []OpsMetrics) (bool, float64, bool) {
	latestValue, ok := metricValue(metrics[0], rule.MetricType)
	if !ok {
		return false, 0, false
	}

	for _, metric := range metrics {
		value, ok := metricValue(metric, rule.MetricType)
		if !ok || !compareMetric(value, rule.Operator, rule.Threshold) {
			return false, latestValue, true
		}
	}

	return true, latestValue, true
}

func metricValue(metric OpsMetrics, metricType string) (float64, bool) {
	switch metricType {
	case OpsMetricSuccessRate:
		if metric.RequestCount == 0 {
			return 0, false
		}
		return metric.SuccessRate, true
	case OpsMetricErrorRate:
		if metric.RequestCount == 0 {
			return 0, false
		}
		return metric.ErrorRate, true
	case OpsMetricP95LatencyMs:
		return metric.LatencyP95, true
	case OpsMetricP99LatencyMs:
		return metric.LatencyP99, true
	case OpsMetricHTTP2Errors:
		return 0, false // HTTP2Errors 已删除
	case OpsMetricCPUUsagePercent:
		return metric.CPUUsagePercent, true
	case OpsMetricMemoryUsagePercent:
		return metric.MemoryUsagePercent, true
	case OpsMetricQueueDepth:
		return float64(metric.ConcurrencyQueueDepth), true
	default:
		return 0, false
	}
}

func compareMetric(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func buildAlertDescription(rule OpsAlertRule, value float64) string {
	window := rule.WindowMinutes
	if window <= 0 {
		window = 1
	}
	return fmt.Sprintf("Rule %s triggered: %s %s %.2f (current %.2f) over last %dm",
		rule.Name,
		rule.MetricType,
		rule.Operator,
		rule.Threshold,
		value,
		window,
	)
}

func (s *OpsAlertService) dispatchNotifications(ctx context.Context, rule OpsAlertRule, event *OpsAlertEvent) bool {
	emailSent := false

	notifyCtx, cancel := s.notificationContext(ctx)
	defer cancel()

	if rule.NotifyEmail {
		emailSent = s.sendEmailNotification(notifyCtx, rule, event)
	}

	return emailSent
}

const (
	opsAlertEvaluateTimeout     = 45 * time.Second
	opsAlertNotificationTimeout = 30 * time.Second
	opsAlertEmailMaxRetries     = 3
)

var opsAlertEmailBackoff = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
}

func (s *OpsAlertService) notificationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	parent := ctx
	if s != nil && s.stopCtx != nil {
		parent = s.stopCtx
	}
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, opsAlertNotificationTimeout)
}

var opsAlertSleep = sleepWithContext

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	if ctx == nil {
		time.Sleep(d)
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryWithBackoff(
	ctx context.Context,
	maxRetries int,
	backoff []time.Duration,
	fn func() error,
	onError func(attempt int, total int, nextDelay time.Duration, err error),
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	totalAttempts := maxRetries + 1

	var lastErr error
	for attempt := 1; attempt <= totalAttempts; attempt++ {
		if attempt > 1 {
			backoffIdx := attempt - 2
			if backoffIdx < len(backoff) {
				if err := opsAlertSleep(ctx, backoff[backoffIdx]); err != nil {
					return err
				}
			}
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if err := fn(); err != nil {
			lastErr = err
			nextDelay := time.Duration(0)
			if attempt < totalAttempts {
				nextIdx := attempt - 1
				if nextIdx < len(backoff) {
					nextDelay = backoff[nextIdx]
				}
			}
			if onError != nil {
				onError(attempt, totalAttempts, nextDelay, err)
			}
			continue
		}
		return nil
	}

	return lastErr
}

func (s *OpsAlertService) sendEmailNotification(ctx context.Context, rule OpsAlertRule, event *OpsAlertEvent) bool {
	if s.emailService == nil || s.userService == nil {
		return false
	}

	if ctx == nil {
		ctx = context.Background()
	}

	admin, err := s.userService.GetFirstAdmin(ctx)
	if err != nil || admin == nil || admin.Email == "" {
		return false
	}

	config, err := s.emailService.GetSMTPConfig(ctx)
	if err != nil {
		log.Printf("[OpsAlert] email config load failed: %v", err)
		return false
	}

	templateData := EmailTemplateData{
		Type:      "alert",
		Title:     rule.Name,
		Message:   fmt.Sprintf("告警规则 %s 已触发", rule.Name),
		LogoURL:   "https://your-site.com/logo.png",
		SiteName:  "Sub2API",
		SiteURL:   "https://your-site.com",
		Year:      time.Now().Year(),
		ActionURL: fmt.Sprintf("https://your-site.com/admin/ops/alerts/%d", rule.ID),
		Alert: &AlertData{
			Status:    event.Status,
			Level:     rule.Severity,
			Metric:    rule.MetricType,
			Value:     fmt.Sprintf("%.2f", event.MetricValue),
			Threshold: fmt.Sprintf("%.2f", rule.Threshold),
			Duration:  fmt.Sprintf("%d 分钟", rule.WindowMinutes),
			Time:      event.FiredAt.Format("2006-01-02 15:04:05"),
			Labels:    map[string]string{},
		},
	}

	subject := fmt.Sprintf("[Ops Alert][%s] %s", rule.Severity, rule.Name)

	if err := retryWithBackoff(
		ctx,
		opsAlertEmailMaxRetries,
		opsAlertEmailBackoff,
		func() error {
			return s.emailService.SendTemplatedEmail(config, admin.Email, subject, templateData)
		},
		func(attempt int, total int, nextDelay time.Duration, err error) {
			if attempt < total {
				log.Printf("[OpsAlert] email send failed (attempt=%d/%d), retrying in %s: %v", attempt, total, nextDelay, err)
				return
			}
			log.Printf("[OpsAlert] email send failed (attempt=%d/%d), giving up: %v", attempt, total, err)
		},
	); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			log.Printf("[OpsAlert] email send canceled: %v", err)
		}
		return false
	}
	return true
}
