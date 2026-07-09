package service

import (
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"

	"github.com/stretchr/testify/require"
)

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
