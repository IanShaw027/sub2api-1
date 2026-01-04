//go:build unit || opsalert_unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelectContiguousMetrics_Contiguous(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	metrics := []OpsMetrics{
		{UpdatedAt: now},
		{UpdatedAt: now.Add(-1 * time.Minute)},
		{UpdatedAt: now.Add(-2 * time.Minute)},
	}

	selected, ok := selectContiguousMetrics(metrics, 3, now)
	require.True(t, ok)
	require.Len(t, selected, 3)
}

func TestSelectContiguousMetrics_GapFails(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	metrics := []OpsMetrics{
		{UpdatedAt: now},
		// Missing the -1m sample (gap ~=2m).
		{UpdatedAt: now.Add(-2 * time.Minute)},
		{UpdatedAt: now.Add(-3 * time.Minute)},
	}

	_, ok := selectContiguousMetrics(metrics, 3, now)
	require.False(t, ok)
}

func TestSelectContiguousMetrics_StaleNewestFails(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)
	metrics := []OpsMetrics{
		{UpdatedAt: now.Add(-10 * time.Minute)},
		{UpdatedAt: now.Add(-11 * time.Minute)},
	}

	_, ok := selectContiguousMetrics(metrics, 2, now)
	require.False(t, ok)
}

func TestMetricValue_SuccessRate_NoTrafficIsNoData(t *testing.T) {
	metric := OpsMetrics{
		RequestCount: 0,
		SuccessRate:  0,
	}
	value, ok := metricValue(metric, OpsMetricSuccessRate)
	require.False(t, ok)
	require.Equal(t, 0.0, value)
}

func TestOpsAlertService_StopWithoutStart_NoPanic(t *testing.T) {
	s := NewOpsAlertService(nil, nil, nil, nil, nil)
	require.NotPanics(t, func() { s.Stop() })
}

func TestOpsAlertService_StartStop_Graceful(t *testing.T) {
	s := NewOpsAlertService(nil, nil, nil, nil, nil)
	s.interval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.StartWithContext(ctx)

	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("Stop did not return; background goroutine likely stuck")
	}

	require.NotPanics(t, func() { s.Stop() })
}

func TestRetryWithBackoff_SucceedsAfterRetries(t *testing.T) {
	oldSleep := opsAlertSleep
	t.Cleanup(func() { opsAlertSleep = oldSleep })

	var slept []time.Duration
	opsAlertSleep = func(ctx context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	attempts := 0
	err := retryWithBackoff(
		context.Background(),
		3,
		[]time.Duration{time.Second, 2 * time.Second, 4 * time.Second},
		func() error {
			attempts++
			if attempts <= 3 {
				return errors.New("send failed")
			}
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, 4, attempts)
	require.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, slept)
}

func TestRetryWithBackoff_ContextCanceledStopsRetries(t *testing.T) {
	oldSleep := opsAlertSleep
	t.Cleanup(func() { opsAlertSleep = oldSleep })

	var slept []time.Duration
	opsAlertSleep = func(ctx context.Context, d time.Duration) error {
		slept = append(slept, d)
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := retryWithBackoff(
		ctx,
		3,
		[]time.Duration{time.Second, 2 * time.Second, 4 * time.Second},
		func() error {
			attempts++
			return errors.New("send failed")
		},
		func(attempt int, total int, nextDelay time.Duration, err error) {
			if attempt == 1 {
				cancel()
			}
		},
	)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, attempts)
	require.Equal(t, []time.Duration{time.Second}, slept)
}
