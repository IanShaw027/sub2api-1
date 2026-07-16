//go:build unit

package admin

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type grokImportProbeStub struct {
	mu           sync.Mutex
	calls        map[int64]int
	failures     map[int64]error
	active       int
	maxActive    int
	deadlineSeen bool
	block        <-chan struct{}
	started      chan int64
	done         chan int64
}

func newGrokImportProbeStub(buffer int) *grokImportProbeStub {
	return &grokImportProbeStub{
		calls:    make(map[int64]int),
		failures: make(map[int64]error),
		started:  make(chan int64, buffer),
		done:     make(chan int64, buffer),
	}
}

func (s *grokImportProbeStub) QueryQuota(ctx context.Context, accountID int64) (*service.GrokQuotaProbeResult, error) {
	_, deadlineSeen := ctx.Deadline()
	s.mu.Lock()
	s.calls[accountID]++
	s.active++
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	s.deadlineSeen = s.deadlineSeen || deadlineSeen
	s.mu.Unlock()

	s.started <- accountID
	var ctxErr error
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			ctxErr = ctx.Err()
		}
	}

	s.mu.Lock()
	s.active--
	failure := s.failures[accountID]
	s.mu.Unlock()
	s.done <- accountID
	if ctxErr != nil {
		return nil, ctxErr
	}
	if failure != nil {
		return nil, failure
	}
	return &service.GrokQuotaProbeResult{Model: "grok-4.5", StatusCode: 200}, nil
}

func awaitGrokProbeSignal(t *testing.T, signals <-chan int64) int64 {
	t.Helper()
	select {
	case id := <-signals:
		return id
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Grok import probe")
		return 0
	}
}

func TestGrokImportProbeSchedulerBoundsConcurrencyAndAddsDeadline(t *testing.T) {
	const taskCount = 20
	release := make(chan struct{})
	scheduler := newGrokImportProbeScheduler(3, time.Second)
	prober := newGrokImportProbeStub(taskCount)
	prober.block = release

	for id := int64(1); id <= taskCount; id++ {
		scheduler.schedule(prober, &service.Account{ID: id, Platform: service.PlatformGrok, Type: service.AccountTypeOAuth})
	}
	for i := 0; i < 3; i++ {
		awaitGrokProbeSignal(t, prober.started)
	}
	scheduler.mu.Lock()
	require.Equal(t, 17, len(scheduler.queue))
	require.Equal(t, 3, scheduler.workers)
	require.Equal(t, 3, scheduler.maxWorkers)
	scheduler.mu.Unlock()
	close(release)
	for i := 0; i < taskCount; i++ {
		awaitGrokProbeSignal(t, prober.done)
	}

	prober.mu.Lock()
	require.Len(t, prober.calls, taskCount)
	require.Equal(t, 3, prober.maxActive)
	require.True(t, prober.deadlineSeen)
	prober.mu.Unlock()
}

func TestGrokImportProbeSchedulerTimeoutCancelsProbe(t *testing.T) {
	scheduler := newGrokImportProbeScheduler(1, 20*time.Millisecond)
	prober := newGrokImportProbeStub(1)
	prober.block = make(chan struct{})
	scheduler.schedule(prober, &service.Account{ID: 21, Platform: service.PlatformGrok, Type: service.AccountTypeOAuth})
	require.Equal(t, int64(21), awaitGrokProbeSignal(t, prober.done))
}

func TestGrokImportProbeFailureLogIsRedacted(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)

	scheduler := newGrokImportProbeScheduler(1, time.Second)
	prober := newGrokImportProbeStub(1)
	prober.failures[31] = infraerrors.New(502, "GROK_TEST_PROBE_FAILED", "refresh-token-secret")
	scheduler.schedule(prober, &service.Account{ID: 31, Platform: service.PlatformGrok, Type: service.AccountTypeOAuth})
	awaitGrokProbeSignal(t, prober.done)

	require.Eventually(t, func() bool {
		return bytes.Contains(logs.Bytes(), []byte("grok_import_active_probe_failed"))
	}, time.Second, 10*time.Millisecond)
	require.Contains(t, logs.String(), "GROK_TEST_PROBE_FAILED")
	require.NotContains(t, logs.String(), "refresh-token-secret")
}

func TestGrokImportProbeSchedulerSkipsNonOAuthGrokAccounts(t *testing.T) {
	scheduler := newGrokImportProbeScheduler(1, time.Second)
	prober := newGrokImportProbeStub(1)
	scheduler.schedule(prober, &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth})
	scheduler.schedule(prober, &service.Account{ID: 42, Platform: service.PlatformGrok, Type: service.AccountTypeAPIKey})
	select {
	case id := <-prober.started:
		t.Fatalf("unexpected probe for account %d", id)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestGrokImportProbeSchedulerDeduplicatesPendingAndInFlightAccounts(t *testing.T) {
	scheduler := newGrokImportProbeScheduler(1, time.Second)
	prober := newGrokImportProbeStub(2)
	release := make(chan struct{})
	prober.block = release
	account := &service.Account{ID: 51, Platform: service.PlatformGrok, Type: service.AccountTypeOAuth}
	queued := &service.Account{ID: 52, Platform: service.PlatformGrok, Type: service.AccountTypeOAuth}

	scheduler.schedule(prober, account)
	require.Equal(t, int64(51), awaitGrokProbeSignal(t, prober.started))
	scheduler.schedule(prober, account)
	scheduler.schedule(prober, queued)
	scheduler.schedule(prober, queued)
	scheduler.mu.Lock()
	require.Len(t, scheduler.queue, 1)
	require.Contains(t, scheduler.inFlight, int64(51))
	require.Contains(t, scheduler.pending, int64(52))
	scheduler.mu.Unlock()

	close(release)
	require.Equal(t, int64(51), awaitGrokProbeSignal(t, prober.done))
	require.Equal(t, int64(52), awaitGrokProbeSignal(t, prober.done))
	prober.mu.Lock()
	require.Equal(t, 1, prober.calls[51])
	require.Equal(t, 1, prober.calls[52])
	prober.mu.Unlock()
}

func TestGrokImportProbeSchedulerBoundsPendingQueue(t *testing.T) {
	scheduler := newGrokImportProbeScheduler(1, time.Second)
	prober := newGrokImportProbeStub(grokImportProbeQueueLimit + 1)
	release := make(chan struct{})
	prober.block = release
	scheduler.schedule(prober, &service.Account{ID: 100, Platform: service.PlatformGrok, Type: service.AccountTypeOAuth})
	require.Equal(t, int64(100), awaitGrokProbeSignal(t, prober.started))
	for id := int64(101); id < 101+grokImportProbeQueueLimit+10; id++ {
		scheduler.schedule(prober, &service.Account{ID: id, Platform: service.PlatformGrok, Type: service.AccountTypeOAuth})
	}
	scheduler.mu.Lock()
	require.Len(t, scheduler.queue, grokImportProbeQueueLimit)
	scheduler.mu.Unlock()
	close(release)
	for i := 0; i < grokImportProbeQueueLimit+1; i++ {
		awaitGrokProbeSignal(t, prober.done)
	}
}
