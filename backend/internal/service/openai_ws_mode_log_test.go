package service

import (
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"

	"github.com/stretchr/testify/require"
)

func TestOpenAIWSTempDiagRateLimiterCapsAndReportsPreviousWindow(t *testing.T) {
	limiter := &openAIWSTempDiagRateLimiter{}
	start := time.Unix(120, 0)

	allowed, suppressed := limiter.allow(start, 2)
	require.True(t, allowed)
	require.Zero(t, suppressed)
	allowed, suppressed = limiter.allow(start.Add(time.Second), 2)
	require.True(t, allowed)
	require.Zero(t, suppressed)
	allowed, _ = limiter.allow(start.Add(2*time.Second), 2)
	require.False(t, allowed)

	allowed, suppressed = limiter.allow(start.Add(time.Minute), 2)
	require.True(t, allowed)
	require.EqualValues(t, 1, suppressed)
}

func TestOpenAIWSTemporaryAnomalyEmitsAtInfoWithRPMCap(t *testing.T) {
	resetOpenAIWSDeltaRuntimeSettingsForTest()
	t.Cleanup(resetOpenAIWSDeltaRuntimeSettingsForTest)
	StoreOpenAIWSDeltaRuntimeSettings(true, true, true)
	StoreOpenAIWSTemporaryDiagnosticLogsRPM(2)

	err := logger.Init(logger.InitOptions{
		Level:       "info",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: true,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)

	sink := &openAIWSModeLogTestSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	logOpenAIWSTemporaryAnomaly("attempt_failed", "account_id=%d", 1)
	logOpenAIWSTemporaryAnomaly("http_fallback", "account_id=%d", 2)
	logOpenAIWSTemporaryAnomaly("read_fail", "account_id=%d", 3)

	events := sink.snapshot()
	require.Len(t, events, 2)
	require.Equal(t, "info", events[0].Level)
	require.Contains(t, events[0].Message, "event=attempt_failed")
	require.Contains(t, events[0].Message, "rpm_limit=2")
}

func TestLogOpenAIWSSlowCompletionOnlyLogsAboveThreshold(t *testing.T) {
	resetOpenAIWSDeltaRuntimeSettingsForTest()
	t.Cleanup(resetOpenAIWSDeltaRuntimeSettingsForTest)
	StoreOpenAIWSDeltaRuntimeSettings(true, true, true)
	StoreOpenAIWSTemporaryDiagnosticLogsRPM(10)

	err := logger.Init(logger.InitOptions{
		Level:       "info",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output:      logger.OutputOptions{ToStdout: true},
		Sampling:    logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)
	sink := &openAIWSModeLogTestSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	logOpenAIWSSlowCompletion(openAIWSDiagnosticCompletedLog{FirstTokenMs: openAIWSTempDiagSlowTTFTMs})
	require.Empty(t, sink.snapshot())
	logOpenAIWSSlowCompletion(openAIWSDiagnosticCompletedLog{
		AccountID:           42,
		FirstTokenMs:        openAIWSTempDiagSlowTTFTMs + 1,
		DeltaFallbackReason: "no_session_context",
		PoolAcquireState:    "no_matching_variant",
		PoolSnapshot: openAIWSAccountPoolSnapshot{
			TotalConns:        4,
			MatchingIdleConns: 2,
		},
	})
	require.Len(t, sink.snapshot(), 1)
	require.Contains(t, sink.snapshot()[0].Message, "event=slow_first_token")
	require.Contains(t, sink.snapshot()[0].Message, "delta_fallback_reason=no_session_context")
	require.Contains(t, sink.snapshot()[0].Message, "pool_total=4")
	require.Contains(t, sink.snapshot()[0].Message, "pool_acquire_state=no_matching_variant")
	require.Contains(t, sink.snapshot()[0].Message, "pool_matching_idle=2")
}

type openAIWSModeLogTestSink struct {
	mu     sync.Mutex
	events []*logger.LogEvent
}

func (s *openAIWSModeLogTestSink) WriteLogEvent(event *logger.LogEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *openAIWSModeLogTestSink) snapshot() []*logger.LogEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*logger.LogEvent, len(s.events))
	copy(out, s.events)
	return out
}

func TestLogOpenAIWSModeInfoEmitsDebugLevel(t *testing.T) {
	err := logger.Init(logger.InitOptions{
		Level:       "debug",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: true,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)

	sink := &openAIWSModeLogTestSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	logOpenAIWSModeInfo("test_marker account_id=%d session_id=%s", 123, "sess_abc")

	events := sink.snapshot()
	require.Len(t, events, 1)
	require.Equal(t, "debug", events[0].Level)
	require.Contains(t, events[0].Message, "[OpenAI WS Mode][openai_ws_mode=true] test_marker")
}

func TestLogOpenAIWSModeInfoSuppressesTemporaryDiagnosticsByDefault(t *testing.T) {
	err := logger.Init(logger.InitOptions{
		Level:       "debug",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: true,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)

	t.Setenv("OPENAI_WS_TEMP_DIAG_LOGS", "")
	sink := &openAIWSModeLogTestSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	logOpenAIWSModeInfo("sample_diag temporary_diag=sample account_id=%d", 123)
	logOpenAIWSModeInfoDirect("sample_diag_direct temporary_diag=sample account_id=%d", 123)

	require.Empty(t, sink.snapshot())
}

func TestLogOpenAIWSModeInfoAllowsTemporaryDiagnosticsWhenExplicitlyEnabled(t *testing.T) {
	err := logger.Init(logger.InitOptions{
		Level:       "debug",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: true,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)

	t.Setenv("OPENAI_WS_TEMP_DIAG_LOGS", "1")
	sink := &openAIWSModeLogTestSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	logOpenAIWSModeInfo("sample_diag temporary_diag=sample account_id=%d", 123)

	events := sink.snapshot()
	require.Len(t, events, 1)
	require.Contains(t, events[0].Message, "sample_diag")
}

func TestOpenAIWSTemporaryDiagnosticLogSwitchCanChangeAtRuntime(t *testing.T) {
	t.Setenv("OPENAI_WS_TEMP_DIAG_LOGS", "")
	resetOpenAIWSDeltaRuntimeSettingsForTest()
	t.Cleanup(resetOpenAIWSDeltaRuntimeSettingsForTest)

	err := logger.Init(logger.InitOptions{
		Level:       "debug",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: true,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)

	sink := &openAIWSModeLogTestSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	logOpenAIWSModeInfo("sample_diag temporary_diag=sample account_id=%d", 123)
	require.Empty(t, sink.snapshot())

	StoreOpenAIWSDeltaRuntimeSettings(true, true, true)
	logOpenAIWSModeInfo("sample_diag temporary_diag=sample account_id=%d", 456)
	require.Len(t, sink.snapshot(), 1)

	StoreOpenAIWSDeltaRuntimeSettings(true, true, false)
	logOpenAIWSModeInfo("sample_diag temporary_diag=sample account_id=%d", 789)
	require.Len(t, sink.snapshot(), 1, "runtime switch should suppress new temporary diagnostics without restart")
}

func TestLogOpenAIWSModeInfoSilentWhenDebugDisabled(t *testing.T) {
	err := logger.Init(logger.InitOptions{
		Level:       "info",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: true,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)

	sink := &openAIWSModeLogTestSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	logOpenAIWSModeInfo("test_marker account_id=%d session_id=%s", 123, "sess_abc")
	logOpenAIWSModeInfoDirect("test_marker account_id=%d session_id=%s", 123, "sess_abc")

	require.Empty(t, sink.snapshot())
}
