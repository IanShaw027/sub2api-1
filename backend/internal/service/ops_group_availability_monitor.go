package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

type OpsGroupAvailabilityMonitor struct {
	opsService   *OpsService
	accountRepo  AccountRepository
	groupRepo    GroupRepository
	emailService *EmailService
	userService  *UserService
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

var opsGroupAvailabilityMonitorInterval = 5 * time.Minute

func NewOpsGroupAvailabilityMonitor(
	opsService *OpsService,
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	emailService *EmailService,
	userService *UserService,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsGroupAvailabilityMonitor {
	lockOn := true
	lockKey := opsGroupAvailabilityLeaderLockKeyDefault
	lockTTL := opsGroupAvailabilityLeaderLockTTLDefault
	if cfg != nil {
		lockOn = cfg.Ops.Alert.DistributedLockEnabled
		if strings.TrimSpace(cfg.Ops.Alert.DistributedLockKey) != "" {
			lockKey = strings.TrimSpace(cfg.Ops.Alert.DistributedLockKey) + ":group_availability"
		}
		if cfg.Ops.Alert.DistributedLockTTL > 0 {
			lockTTL = cfg.Ops.Alert.DistributedLockTTL
		}
	}

	return &OpsGroupAvailabilityMonitor{
		opsService:   opsService,
		accountRepo:  accountRepo,
		groupRepo:    groupRepo,
		emailService: emailService,
		userService:  userService,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		interval:     opsGroupAvailabilityMonitorInterval,
		redisClient:  redisClient,

		distributedLockOn:  lockOn,
		distributedLockKey: lockKey,
		distributedLockTTL: lockTTL,
	}
}

func (s *OpsGroupAvailabilityMonitor) Start() {
	s.StartWithContext(context.Background())
}

func (s *OpsGroupAvailabilityMonitor) StartWithContext(ctx context.Context) {
	if s == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.startOnce.Do(func() {
		if s.interval <= 0 {
			s.interval = opsGroupAvailabilityMonitorInterval
		}

		s.stopCtx, s.stop = context.WithCancel(ctx)
		s.wg.Add(1)
		go s.run()
	})
}

func (s *OpsGroupAvailabilityMonitor) Stop() {
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

func (s *OpsGroupAvailabilityMonitor) run() {
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

func (s *OpsGroupAvailabilityMonitor) evaluateOnce() {
	ctx, cancel := context.WithTimeout(s.stopCtx, 45*time.Second)
	defer cancel()

	s.Evaluate(ctx, time.Now())
}

func (s *OpsGroupAvailabilityMonitor) Evaluate(ctx context.Context, now time.Time) {
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

	configs, err := s.opsService.ListGroupAvailabilityConfigs(ctx, true)
	if err != nil {
		log.Printf("[OpsGroupAvailability] failed to list configs: %v", err)
		return
	}
	if len(configs) == 0 {
		return
	}

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		available, total, err := s.opsService.CountAvailableAccountsByGroup(ctx, cfg.GroupID)
		if err != nil {
			log.Printf("[OpsGroupAvailability] failed to count accounts (group=%d): %v", cfg.GroupID, err)
			continue
		}

		breached := available < cfg.MinAvailableAccounts

		activeEvent, err := s.opsService.GetActiveGroupAvailabilityEvent(ctx, cfg.ID)
		if err != nil {
			log.Printf("[OpsGroupAvailability] failed to get active event (config=%d): %v", cfg.ID, err)
			continue
		}

		if breached {
			if activeEvent != nil {
				continue
			}

			lastEvent, err := s.opsService.GetLatestGroupAvailabilityEvent(ctx, cfg.ID)
			if err != nil {
				log.Printf("[OpsGroupAvailability] failed to get latest event (config=%d): %v", cfg.ID, err)
				continue
			}
			if lastEvent != nil && cfg.CooldownMinutes > 0 {
				cooldown := time.Duration(cfg.CooldownMinutes) * time.Minute
				if now.Sub(lastEvent.FiredAt) < cooldown {
					continue
				}
			}

			group, err := s.groupRepo.GetByID(ctx, cfg.GroupID)
			if err != nil || group == nil {
				log.Printf("[OpsGroupAvailability] failed to get group (id=%d): %v", cfg.GroupID, err)
				continue
			}

			event := &OpsGroupAvailabilityEvent{
				ConfigID:          cfg.ID,
				GroupID:           cfg.GroupID,
				Status:            OpsAlertStatusFiring,
				Severity:          cfg.Severity,
				Title:             fmt.Sprintf("[%s] 分组 %s 可用账号不足", cfg.Severity, group.Name),
				Description:       buildGroupAvailabilityDescription(group, available, cfg.MinAvailableAccounts, total),
				AvailableAccounts: available,
				ThresholdAccounts: cfg.MinAvailableAccounts,
				TotalAccounts:     total,
				FiredAt:           now,
				CreatedAt:         now,
			}

			if err := s.opsService.CreateGroupAvailabilityEvent(ctx, event); err != nil {
				log.Printf("[OpsGroupAvailability] failed to create event (config=%d): %v", cfg.ID, err)
				continue
			}

			emailSent := s.dispatchNotifications(ctx, cfg, event, group)
			if emailSent {
				if err := s.opsService.UpdateGroupAvailabilityEventNotifications(ctx, event.ID, emailSent); err != nil {
					log.Printf("[OpsGroupAvailability] failed to update notification flags (event=%d): %v", event.ID, err)
				}
			}
		} else if activeEvent != nil {
			resolvedAt := now
			if err := s.opsService.UpdateGroupAvailabilityEventStatus(ctx, activeEvent.ID, OpsAlertStatusResolved, &resolvedAt); err != nil {
				log.Printf("[OpsGroupAvailability] failed to resolve event (event=%d): %v", activeEvent.ID, err)
			}
		}
	}
}

const (
	opsGroupAvailabilityLeaderLockKeyDefault    = "ops:group_availability:leader"
	opsGroupAvailabilityLeaderLockTTLDefault    = 30 * time.Second
	opsGroupAvailabilityLeaderLockSkipLogMinInt = 1 * time.Minute
)

var opsGroupAvailabilityLeaderUnlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)

var opsGroupAvailabilityLeaderRenewScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

func (s *OpsGroupAvailabilityMonitor) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil || !s.distributedLockOn {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	key := strings.TrimSpace(s.distributedLockKey)
	if key == "" {
		key = opsGroupAvailabilityLeaderLockKeyDefault
	}
	ttl := s.distributedLockTTL
	if ttl <= 0 {
		ttl = opsGroupAvailabilityLeaderLockTTLDefault
	}

	if s.redisClient == nil {
		s.distributedLockWarn.Do(func() {
			log.Printf("[OpsGroupAvailability] distributed lock enabled but redis client is nil; proceeding without leader lock (key=%q)", key)
		})
		return nil, true
	}

	token := opsGroupAvailabilityLeaderToken()
	ok, err := s.redisClient.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		log.Printf("[OpsGroupAvailability] failed to acquire leader lock (key=%q): %v", key, err)
		return nil, false
	}
	if !ok {
		s.logLeaderLockSkipped(key)
		return nil, false
	}

	log.Printf("[OpsGroupAvailability] acquired leader lock (key=%q ttl=%s token=%s)", key, ttl, shortenLockToken(token))

	renewCancel, renewDone := s.startLeaderLockRenewal(key, token, ttl)

	release := func() {
		if renewCancel != nil {
			renewCancel()
		}
		if renewDone != nil {
			select {
			case <-renewDone:
			case <-time.After(2 * time.Second):
				log.Printf("[OpsGroupAvailability] leader lock renewal goroutine did not stop in time (key=%q)", key)
			}
		}

		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		res, err := opsGroupAvailabilityLeaderUnlockScript.Run(releaseCtx, s.redisClient, []string{key}, token).Int()
		if err != nil {
			log.Printf("[OpsGroupAvailability] failed to release leader lock (key=%q token=%s): %v", key, shortenLockToken(token), err)
			return
		}
		if res == 1 {
			log.Printf("[OpsGroupAvailability] released leader lock (key=%q token=%s)", key, shortenLockToken(token))
		}
	}

	return release, true
}

func (s *OpsGroupAvailabilityMonitor) startLeaderLockRenewal(key string, token string, ttl time.Duration) (context.CancelFunc, <-chan struct{}) {
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
				res, err := opsGroupAvailabilityLeaderRenewScript.Run(renewCtx, s.redisClient, []string{key}, token, ttlMillis).Int()
				renewCancel()
				if err != nil {
					log.Printf("[OpsGroupAvailability] leader lock renewal failed (key=%q token=%s): %v", key, shortenLockToken(token), err)
					continue
				}
				if res == 0 {
					log.Printf("[OpsGroupAvailability] leader lock no longer owned; stop renewing (key=%q token=%s)", key, shortenLockToken(token))
					return
				}
			}
		}
	}()

	return cancel, done
}

func (s *OpsGroupAvailabilityMonitor) logLeaderLockSkipped(key string) {
	if s == nil {
		return
	}
	s.distributedSkipLogMu.Lock()
	defer s.distributedSkipLogMu.Unlock()

	now := time.Now()
	if !s.distributedSkipLogAt.IsZero() && now.Sub(s.distributedSkipLogAt) < opsGroupAvailabilityLeaderLockSkipLogMinInt {
		return
	}
	s.distributedSkipLogAt = now
	log.Printf("[OpsGroupAvailability] skipped evaluation; leader lock held by another instance (key=%q)", key)
}

func opsGroupAvailabilityLeaderToken() string {
	host, _ := os.Hostname()
	pid := os.Getpid()

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s:%d:%d", host, pid, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s:%d:%s", host, pid, hex.EncodeToString(buf))
}

func buildGroupAvailabilityDescription(group *Group, available, threshold, total int) string {
	return fmt.Sprintf(
		"分组可用性告警\n\n分组名称: %s\n平台: %s\n当前可用账号数: %d\n最低阈值: %d\n总账号数: %d",
		group.Name,
		group.Platform,
		available,
		threshold,
		total,
	)
}

func (s *OpsGroupAvailabilityMonitor) dispatchNotifications(ctx context.Context, cfg OpsGroupAvailabilityConfig, event *OpsGroupAvailabilityEvent, group *Group) bool {
	emailSent := false

	notifyCtx, cancel := s.notificationContext(ctx)
	defer cancel()

	if cfg.NotifyEmail {
		emailSent = s.sendEmailNotification(notifyCtx, cfg, event, group)
	}

	return emailSent
}

func (s *OpsGroupAvailabilityMonitor) notificationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	parent := ctx
	if s != nil && s.stopCtx != nil {
		parent = s.stopCtx
	}
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, 30*time.Second)
}

func (s *OpsGroupAvailabilityMonitor) sendEmailNotification(ctx context.Context, cfg OpsGroupAvailabilityConfig, event *OpsGroupAvailabilityEvent, group *Group) bool {
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
		log.Printf("[OpsGroupAvailability] email config load failed: %v", err)
		return false
	}

	templateData := EmailTemplateData{
		Type:      "alert",
		Title:     fmt.Sprintf("分组 %s 可用账号不足", group.Name),
		Message:   fmt.Sprintf("分组 %s 当前可用账号数 %d，低于阈值 %d", group.Name, event.AvailableAccounts, event.ThresholdAccounts),
		LogoURL:   "https://your-site.com/logo.png",
		SiteName:  "Sub2API",
		SiteURL:   "https://your-site.com",
		Year:      time.Now().Year(),
		ActionURL: fmt.Sprintf("https://your-site.com/admin/ops/groups/%d", cfg.GroupID),
		Alert: &AlertData{
			Status:    "",
			Level:     cfg.Severity,
			Metric:    "可用账号数",
			Value:     fmt.Sprintf("%d", event.AvailableAccounts),
			Threshold: fmt.Sprintf(">= %d", event.ThresholdAccounts),
			Duration:  fmt.Sprintf("总账号数: %d", event.TotalAccounts),
			Time:      event.FiredAt.Format("2006-01-02 15:04:05"),
			Labels: map[string]string{
				"分组名称": group.Name,
				"平台":   group.Platform,
			},
		},
	}

	subject := fmt.Sprintf("[Ops Alert][%s] 分组 %s 可用账号不足", cfg.Severity, group.Name)

	if err := retryWithBackoff(
		ctx,
		3,
		[]time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second},
		func() error {
			return s.emailService.SendTemplatedEmail(config, admin.Email, subject, templateData)
		},
		func(attempt int, total int, nextDelay time.Duration, err error) {
			if attempt < total {
				log.Printf("[OpsGroupAvailability] email send failed (attempt=%d/%d), retrying in %s: %v", attempt, total, nextDelay, err)
				return
			}
			log.Printf("[OpsGroupAvailability] email send failed (attempt=%d/%d), giving up: %v", attempt, total, err)
		},
	); err != nil {
		return false
	}
	return true
}
