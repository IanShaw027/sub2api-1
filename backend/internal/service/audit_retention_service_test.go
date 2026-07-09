package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeAuditRetentionRepo struct {
	mu        sync.Mutex
	calls     map[string]int
	cutoffs   []time.Time
	failFor   string
	lastBatch int
}

func newFakeAuditRetentionRepo() *fakeAuditRetentionRepo {
	return &fakeAuditRetentionRepo{calls: map[string]int{}}
}

func (f *fakeAuditRetentionRepo) record(name string, cutoff time.Time, limit int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[name]++
	f.cutoffs = append(f.cutoffs, cutoff)
	f.lastBatch = limit
	if f.failFor == name {
		return 0, errors.New("boom")
	}
	return 1, nil
}

func (f *fakeAuditRetentionRepo) DeleteAIAuditLogsOlderThan(_ context.Context, c time.Time, l int) (int64, error) {
	return f.record("ai_audit_logs", c, l)
}
func (f *fakeAuditRetentionRepo) DeleteAISkillRunsOlderThan(_ context.Context, c time.Time, l int) (int64, error) {
	return f.record("ai_skill_runs", c, l)
}
func (f *fakeAuditRetentionRepo) DeleteAISkillSettlementsOlderThan(_ context.Context, c time.Time, l int) (int64, error) {
	return f.record("ai_skill_settlements", c, l)
}
func (f *fakeAuditRetentionRepo) DeleteCodexInviteResetHistoryOlderThan(_ context.Context, c time.Time, l int) (int64, error) {
	return f.record("codex_invite_reset_history", c, l)
}
func (f *fakeAuditRetentionRepo) DeleteDeletedAPIKeyAuditsOlderThan(_ context.Context, c time.Time, l int) (int64, error) {
	return f.record("deleted_api_key_audits", c, l)
}

// TestAuditRetentionCleanupOnceCoversAllTables cleanupOnce 覆盖全部 5 表、cutoff≈now-retention、batch 透传。
func TestAuditRetentionCleanupOnceCoversAllTables(t *testing.T) {
	repo := newFakeAuditRetentionRepo()
	s := &AuditRetentionService{repo: repo, enabled: true, retention: 180 * 24 * time.Hour, interval: time.Hour, batch: 777, stopCh: make(chan struct{})}

	before := time.Now().Add(-180 * 24 * time.Hour)
	s.cleanupOnce()
	after := time.Now().Add(-180 * 24 * time.Hour)

	for _, table := range []string{"ai_audit_logs", "ai_skill_runs", "ai_skill_settlements", "codex_invite_reset_history", "deleted_api_key_audits"} {
		require.Equal(t, 1, repo.calls[table], "table %s should be cleaned once", table)
	}
	require.Equal(t, 777, repo.lastBatch)
	require.Len(t, repo.cutoffs, 5)
	for _, c := range repo.cutoffs {
		require.False(t, c.Before(before.Add(-time.Second)))
		require.False(t, c.After(after.Add(time.Second)))
	}
}

// TestAuditRetentionCleanupContinuesOnTableError 单表出错不中断其余表清理。
func TestAuditRetentionCleanupContinuesOnTableError(t *testing.T) {
	repo := newFakeAuditRetentionRepo()
	repo.failFor = "ai_skill_runs"
	s := &AuditRetentionService{repo: repo, enabled: true, retention: 24 * time.Hour, interval: time.Hour, batch: 100, stopCh: make(chan struct{})}

	s.cleanupOnce()

	require.Equal(t, 5, len(repo.calls), "全部 5 表都应被尝试")
}

// drainRepo 模拟单表 backlog：前若干批返回满批(=batch)，随后返回不足一批表示删空。
type drainRepo struct {
	remaining int
	batch     int
	calls     int
}

func (d *drainRepo) del(limit int) (int64, error) {
	d.calls++
	if d.remaining <= 0 {
		return 0, nil
	}
	n := limit
	if d.remaining < n {
		n = d.remaining
	}
	d.remaining -= n
	return int64(n), nil
}

func (d *drainRepo) DeleteAIAuditLogsOlderThan(_ context.Context, _ time.Time, l int) (int64, error) {
	return d.del(l)
}
func (d *drainRepo) DeleteAISkillRunsOlderThan(_ context.Context, _ time.Time, l int) (int64, error) {
	return 0, nil
}
func (d *drainRepo) DeleteAISkillSettlementsOlderThan(_ context.Context, _ time.Time, l int) (int64, error) {
	return 0, nil
}
func (d *drainRepo) DeleteCodexInviteResetHistoryOlderThan(_ context.Context, _ time.Time, l int) (int64, error) {
	return 0, nil
}
func (d *drainRepo) DeleteDeletedAPIKeyAuditsOlderThan(_ context.Context, _ time.Time, l int) (int64, error) {
	return 0, nil
}

// TestAuditRetentionCleanupDrainsBacklog 单表有多批 backlog 时，一轮 cleanup 应循环删至清空(单批 <batch)。
func TestAuditRetentionCleanupDrainsBacklog(t *testing.T) {
	repo := &drainRepo{remaining: 2500, batch: 1000}
	s := &AuditRetentionService{repo: repo, enabled: true, retention: time.Hour, interval: time.Hour, batch: 1000, stopCh: make(chan struct{})}

	s.cleanupOnce()

	// 2500 行 / 1000 批：1000 + 1000 + 500(<batch，停) = 3 次调用删空。
	require.Equal(t, 3, repo.calls)
	require.Equal(t, 0, repo.remaining)
}

// TestAuditRetentionStartDisabledDoesNotRun 未启用时 Start 不启动后台清理。
func TestAuditRetentionStartDisabledDoesNotRun(t *testing.T) {
	repo := newFakeAuditRetentionRepo()
	s := &AuditRetentionService{repo: repo, enabled: false, retention: time.Hour, interval: time.Hour, batch: 100, stopCh: make(chan struct{})}
	s.Start()
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, repo.calls)
}
