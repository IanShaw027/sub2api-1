package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// AuditRetentionRepository 定义审计表按 created_at 批量删除旧行的仓储能力。
type AuditRetentionRepository interface {
	DeleteAIAuditLogsOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	DeleteAISkillRunsOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	DeleteAISkillSettlementsOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	DeleteCodexInviteResetHistoryOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	DeleteDeletedAPIKeyAuditsOlderThan(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}

// AuditRetentionService 周期性清理只增审计表中超过保留期的旧行，防止无界增长。
type AuditRetentionService struct {
	repo      AuditRetentionRepository
	enabled   bool
	retention time.Duration
	interval  time.Duration
	batch     int

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

func NewAuditRetentionService(repo AuditRetentionRepository, cfg *config.Config) *AuditRetentionService {
	enabled := false
	retention := 180 * 24 * time.Hour
	interval := 3600 * time.Second
	batch := 1000
	if cfg != nil {
		enabled = cfg.AuditRetention.Enabled
		if cfg.AuditRetention.RetentionDays > 0 {
			retention = time.Duration(cfg.AuditRetention.RetentionDays) * 24 * time.Hour
		}
		if cfg.AuditRetention.CleanupIntervalSeconds > 0 {
			interval = time.Duration(cfg.AuditRetention.CleanupIntervalSeconds) * time.Second
		}
		if cfg.AuditRetention.BatchSize > 0 {
			batch = cfg.AuditRetention.BatchSize
		}
	}
	return &AuditRetentionService{
		repo:      repo,
		enabled:   enabled,
		retention: retention,
		interval:  interval,
		batch:     batch,
		stopCh:    make(chan struct{}),
	}
}

func (s *AuditRetentionService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	if !s.enabled {
		logger.LegacyPrintf("service.audit_retention", "[AuditRetention] disabled by config, skip start")
		return
	}
	s.startOnce.Do(func() {
		logger.LegacyPrintf("service.audit_retention", "[AuditRetention] started retention=%s interval=%s batch=%d", s.retention, s.interval, s.batch)
		go s.runLoop()
	})
}

func (s *AuditRetentionService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
		logger.LegacyPrintf("service.audit_retention", "[AuditRetention] stopped")
	})
}

func (s *AuditRetentionService) runLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 启动后先清理一轮，防止重启后积压。
	s.cleanupOnce()

	for {
		select {
		case <-ticker.C:
			s.cleanupOnce()
		case <-s.stopCh:
			return
		}
	}
}

func (s *AuditRetentionService) cleanupOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cutoff := time.Now().Add(-s.retention)

	// 逐表清理：审计清理非关键，单表失败记录日志后继续删其余表，不中断。
	tasks := []struct {
		name string
		fn   func(context.Context, time.Time, int) (int64, error)
	}{
		{"ai_audit_logs", s.repo.DeleteAIAuditLogsOlderThan},
		{"ai_skill_runs", s.repo.DeleteAISkillRunsOlderThan},
		{"ai_skill_settlements", s.repo.DeleteAISkillSettlementsOlderThan},
		{"codex_invite_reset_history", s.repo.DeleteCodexInviteResetHistoryOlderThan},
		{"deleted_api_key_audits", s.repo.DeleteDeletedAPIKeyAuditsOlderThan},
	}
	for _, task := range tasks {
		if ctx.Err() != nil {
			// 本轮预算(10s)已耗尽，余下表留待下一 tick，避免固定只削一批导致高写入表积压永久增长。
			break
		}
		var tableTotal int64
		for {
			deleted, err := task.fn(ctx, cutoff, s.batch)
			if err != nil {
				logger.LegacyPrintf("service.audit_retention", "[AuditRetention] cleanup failed table=%s err=%v", task.name, err)
				break
			}
			tableTotal += deleted
			// 单批仍受 batch 上限约束(短事务、走 created_at 索引)；循环删至本表 backlog 清空或预算耗尽。
			if deleted < int64(s.batch) || ctx.Err() != nil {
				break
			}
		}
		if tableTotal > 0 {
			logger.LegacyPrintf("service.audit_retention", "[AuditRetention] cleaned table=%s count=%d", task.name, tableTotal)
		}
	}
}
