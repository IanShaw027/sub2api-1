//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alitto/pond/v2"
)

// stubMonitorSvc 实现 monitorRunnerSvc，用于隔离 runner 与真实 service/repo。
type stubMonitorSvc struct {
	enabled    []*ChannelMonitor
	runCount   atomic.Int64
	runCalled  chan int64 // 每次 RunCheck 触发时 push 一次（缓冲足够大避免阻塞）
	runDone    chan int64 // RunCheck 返回时 push 一次（用于断言取消是否生效）
	runErr     error
	listErr    error
	runHoldFor time.Duration // RunCheck 内额外阻塞的时长，用来测试 Stop 等待行为
	blockUntil <-chan struct{}
	lockErr    error
	locker     *monitorRunLockStub
	panicRun   bool
}

type monitorRunLockStub struct {
	mu   sync.Mutex
	held map[int64]struct{}
}

func newMonitorRunLockStub() *monitorRunLockStub {
	return &monitorRunLockStub{held: make(map[int64]struct{})}
}

func (s *monitorRunLockStub) Acquire(id int64) (func(), bool) {
	s.mu.Lock()
	if _, exists := s.held[id]; exists {
		s.mu.Unlock()
		return nil, false
	}
	s.held[id] = struct{}{}
	s.mu.Unlock()

	release := func() {
		s.mu.Lock()
		delete(s.held, id)
		s.mu.Unlock()
	}
	return release, true
}

func (s *stubMonitorSvc) ListEnabledMonitors(_ context.Context) ([]*ChannelMonitor, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.enabled, nil
}

func (s *stubMonitorSvc) RunCheck(ctx context.Context, id int64) ([]*CheckResult, error) {
	s.runCount.Add(1)
	if s.panicRun {
		panic("run check panic")
	}
	if s.runCalled != nil {
		select {
		case s.runCalled <- id:
		default:
		}
	}
	if s.runHoldFor > 0 {
		select {
		case <-time.After(s.runHoldFor):
		case <-ctx.Done():
		}
	}
	if s.blockUntil != nil {
		select {
		case <-s.blockUntil:
		case <-ctx.Done():
		}
	}
	if s.runDone != nil {
		select {
		case s.runDone <- id:
		default:
		}
	}
	return nil, s.runErr
}

func (s *stubMonitorSvc) AcquireChannelMonitorRunLock(_ context.Context, id int64) (func(), bool, error) {
	if s.lockErr != nil {
		return nil, false, s.lockErr
	}
	if s.locker == nil {
		return nil, true, nil
	}
	release, acquired := s.locker.Acquire(id)
	return release, acquired, nil
}

func newRunnerForTest(svc monitorRunnerSvc) *ChannelMonitorRunner {
	return newChannelMonitorRunner(svc, nil)
}

// 等待 condition 在 timeout 内变 true，否则 t.Fatalf。轮询 5ms 一次。
func waitFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("waitFor timed out: %s", msg)
	}
}

func runnerTaskCount(r *ChannelMonitorRunner) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks)
}

func runnerTaskPtr(r *ChannelMonitorRunner, id int64) *scheduledMonitor {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tasks[id]
}

// TestSchedule_AddsTaskAndFiresOnce 验证 Schedule 后立即触发一次首检测，并把任务记入 tasks 表。
func TestSchedule_AddsTaskAndFiresOnce(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4)}
	r := newRunnerForTest(svc)
	r.Start() // svc.enabled 为空，Start 立即完成

	r.Schedule(&ChannelMonitor{ID: 1, Name: "m1", Enabled: true, IntervalSeconds: 60})

	if got := runnerTaskCount(r); got != 1 {
		t.Fatalf("expected 1 scheduled task, got %d", got)
	}

	select {
	case id := <-svc.runCalled:
		if id != 1 {
			t.Fatalf("expected first fire for id=1, got %d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected immediate first fire within 2s")
	}

	r.Stop()
}

// TestSchedule_ReplaceCancelsOldTask 验证对同一 id 二次 Schedule 会替换旧 task 实例。
// （旧 goroutine 通过 ctx 取消退出；这里以 task 指针不同 + Stop 不超时作为证据。）
func TestSchedule_ReplaceCancelsOldTask(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 8)}
	r := newRunnerForTest(svc)
	r.Start()

	m := &ChannelMonitor{ID: 7, Name: "m7", Enabled: true, IntervalSeconds: 60}
	r.Schedule(m)
	first := runnerTaskPtr(r, 7)
	if first == nil {
		t.Fatal("first schedule did not register task")
	}

	r.Schedule(m)
	second := runnerTaskPtr(r, 7)
	if second == nil {
		t.Fatal("second schedule did not register task")
	}
	if first == second {
		t.Fatal("re-Schedule should create a new scheduledMonitor instance")
	}

	stoppedWithin(t, r, 3*time.Second)
}

// TestUnschedule_RemovesTask 验证 Unschedule 删除 task 并使对应 goroutine 退出。
func TestUnschedule_RemovesTask(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4)}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 3, Enabled: true, IntervalSeconds: 60})
	waitFor(t, time.Second, "task registered", func() bool { return runnerTaskCount(r) == 1 })

	r.Unschedule(3)
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected tasks empty after Unschedule, got %d", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

// TestSchedule_DisabledRedirectsToUnschedule 验证 Enabled=false 等同于 Unschedule。
func TestSchedule_DisabledRedirectsToUnschedule(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4)}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 9, Enabled: true, IntervalSeconds: 60})
	waitFor(t, time.Second, "task registered", func() bool { return runnerTaskCount(r) == 1 })

	r.Schedule(&ChannelMonitor{ID: 9, Enabled: false, IntervalSeconds: 60})
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected tasks empty after disabled re-Schedule, got %d", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

// TestSchedule_InvalidIntervalSkipped 验证 IntervalSeconds<=0 不会注册任务（防御性检查）。
func TestSchedule_InvalidIntervalSkipped(t *testing.T) {
	svc := &stubMonitorSvc{}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 1, Enabled: true, IntervalSeconds: 0})
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected no task for invalid interval, got %d", got)
	}
	r.Stop()
}

// TestSchedule_BeforeStartIsNoOp 验证 Start 之前调用 Schedule 不会注册任务。
func TestSchedule_BeforeStartIsNoOp(t *testing.T) {
	svc := &stubMonitorSvc{}
	r := newRunnerForTest(svc)
	// 故意不调用 Start

	r.Schedule(&ChannelMonitor{ID: 1, Enabled: true, IntervalSeconds: 60})
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected no task before Start, got %d", got)
	}
	r.Stop()
}

// TestStart_LoadsAllEnabledMonitors 验证 Start 会为 ListEnabledMonitors 返回的每条记录建立任务。
func TestStart_LoadsAllEnabledMonitors(t *testing.T) {
	svc := &stubMonitorSvc{
		enabled: []*ChannelMonitor{
			{ID: 1, Enabled: true, IntervalSeconds: 60},
			{ID: 2, Enabled: true, IntervalSeconds: 60},
			{ID: 3, Enabled: true, IntervalSeconds: 60},
		},
	}
	r := newRunnerForTest(svc)
	r.Start()
	waitFor(t, 2*time.Second, "all 3 tasks scheduled", func() bool { return runnerTaskCount(r) == 3 })

	stoppedWithin(t, r, 3*time.Second)
}

// TestStart_LoadFailureAllowsRetry 验证启动加载失败不会永久占用 started 状态，
// 后续 Start 能重新加载并调度 enabled monitor。
func TestStart_LoadFailureAllowsRetry(t *testing.T) {
	svc := &stubMonitorSvc{listErr: errors.New("database unavailable")}
	r := newRunnerForTest(svc)

	r.Start()
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected no tasks after failed startup load, got %d", got)
	}

	svc.listErr = nil
	svc.enabled = []*ChannelMonitor{{ID: 42, Enabled: true, IntervalSeconds: 60}}
	r.Start()
	waitFor(t, 2*time.Second, "task scheduled after retry", func() bool { return runnerTaskCount(r) == 1 })

	stoppedWithin(t, r, 3*time.Second)
}

// TestStop_DrainsAllGoroutines 验证 Stop 会等待所有调度 goroutine 退出（无游离）。
func TestStop_DrainsAllGoroutines(t *testing.T) {
	svc := &stubMonitorSvc{}
	r := newRunnerForTest(svc)
	r.Start()

	for id := int64(1); id <= 5; id++ {
		r.Schedule(&ChannelMonitor{ID: id, Enabled: true, IntervalSeconds: 60})
	}
	waitFor(t, 2*time.Second, "5 tasks scheduled", func() bool { return runnerTaskCount(r) == 5 })

	stoppedWithin(t, r, 3*time.Second)
}

// TestStop_CancelsAndWaitsForInFlightCheck 验证 Stop 会取消并等待正在执行的 RunCheck 退出（pool.StopAndWait）。
func TestStop_CancelsAndWaitsForInFlightCheck(t *testing.T) {
	svc := &stubMonitorSvc{
		runCalled:  make(chan int64, 1),
		runDone:    make(chan int64, 1),
		blockUntil: make(chan struct{}),
	}
	r := newRunnerForTest(svc)
	r.Start()
	r.Schedule(&ChannelMonitor{ID: 1, Enabled: true, IntervalSeconds: 60})

	select {
	case <-svc.runCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("first fire never happened")
	}

	stoppedWithin(t, r, 3*time.Second)

	select {
	case <-svc.runDone:
	default:
		t.Fatal("Stop returned before in-flight check observed cancellation and exited")
	}
	if !r.tryAcquireInFlight(1) {
		t.Fatal("Stop returned before inFlight slot was released")
	}
	r.releaseInFlight(1)
}

// TestUnschedule_CancelsInFlightCheck 验证 Unschedule 会取消 in-flight RunCheck 的 ctx。
func TestUnschedule_CancelsInFlightCheck(t *testing.T) {
	svc := &stubMonitorSvc{
		runCalled: make(chan int64, 1),
		runDone:   make(chan int64, 1),
		// 永不主动释放；只能依赖 ctx.Done() 返回。
		blockUntil: make(chan struct{}),
	}
	r := newRunnerForTest(svc)
	r.Start()
	r.Schedule(&ChannelMonitor{ID: 11, Enabled: true, IntervalSeconds: 60})

	select {
	case <-svc.runCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("first fire never happened")
	}

	r.Unschedule(11)

	select {
	case id := <-svc.runDone:
		if id != 11 {
			t.Fatalf("expected canceled run for id=11, got %d", id)
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("unschedule did not cancel in-flight RunCheck within 1.5s")
	}

	stoppedWithin(t, r, 3*time.Second)
}

// TestInFlight_PoolFullReleasesSlot 直接驱动 fire 路径，模拟 pool.TrySubmit 失败时 inFlight 必须释放。
// 用一个小型 stub pool 替换 r.pool 不便（pond.Pool 是接口但 mock 麻烦），
// 改为：占满 inFlight 后直接 fire，验证不会在 inFlight 空槽时永久卡住。
func TestInFlight_AcquireReleaseSymmetric(t *testing.T) {
	svc := &stubMonitorSvc{}
	r := newRunnerForTest(svc)

	if !r.tryAcquireInFlight(42) {
		t.Fatal("first acquire should succeed")
	}
	if r.tryAcquireInFlight(42) {
		t.Fatal("second acquire (no release) must fail")
	}
	r.releaseInFlight(42)
	if !r.tryAcquireInFlight(42) {
		t.Fatal("acquire after release should succeed")
	}
	r.releaseInFlight(42)
}

// TestFire_PoolFullRunsSynchronously 验证 worker 池满时不会丢弃检测，而是同步执行本次任务。
func TestFire_PoolFullRunsSynchronously(t *testing.T) {
	block := make(chan struct{})
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4), blockUntil: block}
	r := newRunnerForTest(svc)
	r.pool = pond.NewPool(1, pond.WithQueueSize(0))
	r.Start()

	r.fire(context.Background(), &scheduledMonitor{id: 1, name: "m1"})
	select {
	case <-svc.runCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("task 1 never started")
	}

	done := make(chan struct{})
	go func() {
		r.fire(context.Background(), &scheduledMonitor{id: 2, name: "m2"})
		close(done)
	}()
	select {
	case id := <-svc.runCalled:
		if id != 2 {
			t.Fatalf("expected synchronous run for id=2, got %d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pool-full fire did not execute synchronously")
	}
	select {
	case <-done:
		t.Fatal("synchronous fallback should stay blocked until RunCheck returns")
	case <-time.After(50 * time.Millisecond):
	}

	close(block)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("synchronous fallback did not return after RunCheck completed")
	}
	waitFor(t, time.Second, "task 2 in-flight released", func() bool {
		if !r.tryAcquireInFlight(2) {
			return false
		}
		r.releaseInFlight(2)
		return true
	})
	stoppedWithin(t, r, 3*time.Second)
}

func TestFire_PoolQueueBuffersBurst(t *testing.T) {
	block := make(chan struct{})
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4), blockUntil: block}
	r := newRunnerForTest(svc)
	r.pool = pond.NewPool(1, pond.WithQueueSize(1))
	r.Start()

	r.fire(context.Background(), &scheduledMonitor{id: 1, name: "m1"})
	select {
	case <-svc.runCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("task 1 never started")
	}

	r.fire(context.Background(), &scheduledMonitor{id: 2, name: "m2"})
	if r.tryAcquireInFlight(2) {
		r.releaseInFlight(2)
		t.Fatal("queued task should keep inFlight slot until it runs")
	}

	close(block)
	select {
	case id := <-svc.runCalled:
		if id != 2 {
			t.Fatalf("expected queued task id=2, got %d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queued task did not run")
	}
	stoppedWithin(t, r, 3*time.Second)
}

// TestRunOne_ReleasesInFlightOnAllExitPaths 验证 runOne 的所有退出路径都会释放 inFlight。
func TestRunOne_ReleasesInFlightOnAllExitPaths(t *testing.T) {
	tests := []struct {
		name string
		svc  *stubMonitorSvc
		hold func(*testing.T, *stubMonitorSvc, int64) func()
	}{
		{
			name: "distributed lock error",
			svc:  &stubMonitorSvc{lockErr: errors.New("lock unavailable")},
		},
		{
			name: "distributed lock busy",
			svc:  &stubMonitorSvc{locker: newMonitorRunLockStub()},
			hold: func(t *testing.T, svc *stubMonitorSvc, id int64) func() {
				t.Helper()
				release, acquired := svc.locker.Acquire(id)
				if !acquired {
					t.Fatal("failed to pre-acquire distributed lock")
				}
				return release
			},
		},
		{
			name: "panic",
			svc:  &stubMonitorSvc{panicRun: true},
		},
		{
			name: "normal completion",
			svc:  &stubMonitorSvc{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const id int64 = 123
			if tt.hold != nil {
				release := tt.hold(t, tt.svc, id)
				defer release()
			}

			r := newRunnerForTest(tt.svc)
			if !r.tryAcquireInFlight(id) {
				t.Fatal("failed to pre-acquire inFlight slot")
			}

			r.runOne(context.Background(), id, "m123")

			if !r.tryAcquireInFlight(id) {
				t.Fatal("runOne did not release inFlight slot")
			}
			r.releaseInFlight(id)
			r.Stop()
		})
	}
}

// TestRunOne_DistributedLockContentionSkipsDuplicate 验证多 runner 竞争同一 monitor 锁时只有一方执行 RunCheck。
func TestRunOne_DistributedLockContentionSkipsDuplicate(t *testing.T) {
	block := make(chan struct{})
	svc := &stubMonitorSvc{
		runCalled:  make(chan int64, 4),
		blockUntil: block,
		locker:     newMonitorRunLockStub(),
	}
	r1 := newRunnerForTest(svc)
	r2 := newRunnerForTest(svc)
	r1.Start()
	r2.Start()

	task := &scheduledMonitor{id: 88, name: "m88"}
	r1.fire(context.Background(), task)
	select {
	case <-svc.runCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("runner #1 did not start RunCheck")
	}

	r2.fire(context.Background(), task)
	// r1 仍持有锁时，r2 应直接跳过，不会触发第二次 RunCheck。
	time.Sleep(100 * time.Millisecond)
	if got := svc.runCount.Load(); got != 1 {
		t.Fatalf("expected only one RunCheck under lock contention, got %d", got)
	}

	close(block)
	stoppedWithin(t, r1, 3*time.Second)
	stoppedWithin(t, r2, 3*time.Second)
}

func TestRunManual_RespectsFeatureSwitch(t *testing.T) {
	svc := &stubMonitorSvc{}
	settings := NewSettingService(&settingPublicRepoStub{values: map[string]string{
		SettingKeyChannelMonitorEnabled: "false",
	}}, nil)
	r := newChannelMonitorRunner(svc, settings)
	r.Start()

	_, err := r.RunManual(context.Background(), 12)
	if !errors.Is(err, ErrChannelMonitorDisabled) {
		t.Fatalf("expected ErrChannelMonitorDisabled, got %v", err)
	}
	if got := svc.runCount.Load(); got != 0 {
		t.Fatalf("RunCheck should not be called when feature is disabled, got %d", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

func TestRunManual_RejectsConcurrentRun(t *testing.T) {
	block := make(chan struct{})
	svc := &stubMonitorSvc{runCalled: make(chan int64, 1), blockUntil: block}
	r := newRunnerForTest(svc)
	r.Start()

	done := make(chan error, 1)
	go func() {
		_, err := r.RunManual(context.Background(), 33)
		done <- err
	}()
	select {
	case <-svc.runCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("first manual run did not start")
	}

	_, err := r.RunManual(context.Background(), 33)
	if !errors.Is(err, ErrChannelMonitorRunInFlight) {
		t.Fatalf("expected ErrChannelMonitorRunInFlight, got %v", err)
	}

	close(block)
	if err := <-done; err != nil {
		t.Fatalf("first manual run returned error: %v", err)
	}
	stoppedWithin(t, r, 3*time.Second)
}

func TestRunManual_RespectsDistributedLock(t *testing.T) {
	svc := &stubMonitorSvc{locker: newMonitorRunLockStub()}
	release, acquired := svc.locker.Acquire(44)
	if !acquired {
		t.Fatal("failed to pre-acquire distributed lock")
	}
	defer release()

	r := newRunnerForTest(svc)
	r.Start()
	_, err := r.RunManual(context.Background(), 44)
	if !errors.Is(err, ErrChannelMonitorRunInFlight) {
		t.Fatalf("expected ErrChannelMonitorRunInFlight under lock contention, got %v", err)
	}
	if got := svc.runCount.Load(); got != 0 {
		t.Fatalf("RunCheck should not be called when distributed lock is busy, got %d", got)
	}
	stoppedWithin(t, r, 3*time.Second)
}

func TestRunner_FeatureDisableQuiescesAndReenableReloadsTasks(t *testing.T) {
	repo := &settingPublicRepoStub{values: map[string]string{
		SettingKeyChannelMonitorEnabled: "true",
	}}
	settings := NewSettingService(repo, nil)
	svc := &stubMonitorSvc{
		enabled: []*ChannelMonitor{
			{ID: 1, Name: "m1", Enabled: true, IntervalSeconds: 1},
			{ID: 2, Name: "m2", Enabled: true, IntervalSeconds: 1},
		},
		runCalled: make(chan int64, 8),
	}
	r := newChannelMonitorRunner(svc, settings)
	r.Start()

	waitFor(t, 2*time.Second, "all tasks scheduled at startup", func() bool {
		return runnerTaskCount(r) == 2
	})

	repo.setValue(SettingKeyChannelMonitorEnabled, "false")
	waitFor(t, 2*time.Second, "feature disable should quiesce all tasks", func() bool {
		return runnerTaskCount(r) == 0
	})

	runCountAfterDisable := svc.runCount.Load()
	time.Sleep(1200 * time.Millisecond)
	if got := svc.runCount.Load(); got != runCountAfterDisable {
		t.Fatalf("expected no additional runs after quiesce, got %d -> %d", runCountAfterDisable, got)
	}

	repo.setValue(SettingKeyChannelMonitorEnabled, "true")
	waitFor(t, 2*time.Second, "feature re-enable should reload enabled monitors", func() bool {
		return runnerTaskCount(r) == 2
	})

	stoppedWithin(t, r, 3*time.Second)
}

// stoppedWithin 在 timeout 内并行调用 Stop，超时则 Fatal。验证 Stop 不会阻塞。
func stoppedWithin(t *testing.T, r *ChannelMonitorRunner, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	var once sync.Once
	go func() {
		r.Stop()
		once.Do(func() { close(done) })
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("Stop did not return within %s — leaked goroutine?", timeout)
	}
}
