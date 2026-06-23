package service

import (
	"context"
	"net"
	"testing"

	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

func TestTLSCaptureTaskCompletionRequiresTargetsAndTransportTargets(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:             "transport aware",
		Targets:          map[string]int{"openai": 1},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH1): 1, string(tlsfpTransport.HTTP1): 1},
		UAKeywords:       []string{"codex"},
	})
	require.NoError(t, err)

	first, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		Transport:   string(tlsfpTransport.WebSocketH1),
		SessionID:   "sess-ws-1",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.NotNil(t, first.Sample)
	require.NotNil(t, first.Session)
	require.NotNil(t, first.SessionEvent)
	require.Equal(t, map[string]int{"openai": 1}, first.Counts)
	require.Equal(t, map[string]int{string(tlsfpTransport.HTTP1): 0, string(tlsfpTransport.WebSocketH1): 1}, first.Task.TransportCounts)
	require.Equal(t, TLSFingerprintCaptureStatusRunning, first.Task.Status)

	second, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		Transport:   string(tlsfpTransport.HTTP1),
		SessionID:   "sess-http-1",
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		ClientHello: captureServiceTestClientHello(t, 1),
	})
	require.NoError(t, err)
	require.True(t, second.Accepted)
	require.NotNil(t, second.Sample)
	require.NotNil(t, second.Session)
	require.NotNil(t, second.SessionEvent)
	require.Equal(t, map[string]int{"openai": 2}, second.Counts)
	require.Equal(t, map[string]int{string(tlsfpTransport.HTTP1): 1, string(tlsfpTransport.WebSocketH1): 1}, second.Task.TransportCounts)
	require.Equal(t, TLSFingerprintCaptureStatusCompleted, second.Task.Status)
	require.Len(t, repo.samples, 2)
	require.Len(t, repo.sessions, 2)
	require.Len(t, repo.sessionEvents, 2)
}

func TestTLSCaptureSampleDedupesByReplayHashAndTransport(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 2},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH1): 2},
		UAKeywords:       []string{"codex"},
	})
	require.NoError(t, err)

	first, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		Transport:   string(tlsfpTransport.WebSocketH1),
		SessionID:   "sess-a",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.False(t, first.Duplicate)
	require.NotNil(t, first.Sample)

	second, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		Transport:   string(tlsfpTransport.WebSocketH1),
		SessionID:   "sess-b",
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, second.Accepted)
	require.True(t, second.Duplicate)
	require.NotNil(t, second.Sample)
	require.Equal(t, first.Sample.ID, second.Sample.ID)
	require.Equal(t, first.Sample.ReplayHash, second.Sample.ReplayHash)
	require.Equal(t, map[string]int{"openai": 1}, second.Counts)
	require.Equal(t, map[string]int{string(tlsfpTransport.WebSocketH1): 1}, second.Task.TransportCounts)
	require.Len(t, repo.samples, 1)
	require.Len(t, repo.sessions, 2)
	require.Len(t, repo.sessionEvents, 2)
}

func TestTLSCaptureSessionReplayabilityFailureCreatesSessionEventOnly(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 1},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH1): 1},
		UAKeywords:       []string{"codex"},
	})
	require.NoError(t, err)

	replayable := false
	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:             task.Token,
		Platform:          "openai",
		Transport:         string(tlsfpTransport.WebSocketH1),
		SessionID:         "sess-replay-fail",
		UserAgent:         "codex-tui/0.140.0",
		Originator:        "codex_cli_rs",
		ClientHello:       captureServiceTestClientHello(t, 0),
		Replayable:        &replayable,
		SessionEventType:  "replayability_failed",
		SessionEventError: "missing resumption material",
	})
	require.NoError(t, err)
	require.False(t, result.Accepted)
	require.False(t, result.Duplicate)
	require.Equal(t, "capture_not_replayable", result.IgnoredReason)
	require.NotNil(t, result.Session)
	require.NotNil(t, result.SessionEvent)
	require.Nil(t, result.Sample)
	require.Equal(t, "replayability_failed", result.SessionEvent.EventType)
	require.Equal(t, "missing resumption material", result.SessionEvent.Error)
	require.False(t, result.SessionEvent.Replayable)
	require.Equal(t, map[string]int{"openai": 0}, result.Counts)
	require.Equal(t, map[string]int{string(tlsfpTransport.WebSocketH1): 0}, result.Task.TransportCounts)
	require.Len(t, repo.samples, 0)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 1)
}

func TestTLSCaptureSessionSameTransportProducesEventButSingleCanonicalSample(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 2},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH1): 2},
		UAKeywords:       []string{"codex"},
	})
	require.NoError(t, err)

	first, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		Transport:   string(tlsfpTransport.WebSocketH1),
		SessionID:   "sess-sticky",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.NotNil(t, first.Sample)

	second, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		Transport:   string(tlsfpTransport.WebSocketH1),
		SessionID:   "sess-sticky",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 1),
	})
	require.NoError(t, err)
	require.True(t, second.Accepted)
	require.True(t, second.Duplicate)
	require.NotNil(t, second.Session)
	require.NotNil(t, second.SessionEvent)
	require.NotNil(t, second.Sample)
	require.Equal(t, first.Sample.ID, second.Sample.ID)
	require.Equal(t, map[string]int{"openai": 1}, second.Counts)
	require.Equal(t, map[string]int{string(tlsfpTransport.WebSocketH1): 1}, second.Task.TransportCounts)
	require.Equal(t, TLSFingerprintCaptureStatusRunning, second.Task.Status)
	require.Len(t, repo.samples, 1)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 2)
}

func TestTLSCaptureMissingSessionStillPersistsGeneratedSession(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 1},
	})
	require.NoError(t, err)

	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, result.Accepted)
	require.NotNil(t, result.Session)
	require.NotEmpty(t, result.Session.SessionID)
	require.NotNil(t, result.Sample)
	require.Equal(t, result.Session.SessionID, result.Sample.SessionID)
	require.Len(t, repo.sessions, 1)
}

func TestTLSCaptureParseFailureCreatesSessionEventForInspection(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 1},
	})
	require.NoError(t, err)

	_, err = svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		SessionID:   "sess-parse-fail",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: []byte{0x16, 0x03, 0x01, 0x00, 0x01, 0x00},
	})
	require.Error(t, err)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 1)
	require.Equal(t, "client_hello_parse_failed", repo.sessionEvents[0].EventType)
	require.Equal(t, "error", repo.sessionEvents[0].EventStatus)
	require.False(t, repo.sessionEvents[0].Replayable)
	require.Contains(t, repo.sessionEvents[0].Error, "parse")
}

func TestTLSCaptureDefaultTransportInference(t *testing.T) {
	tests := []struct {
		name          string
		alpn          string
		isWebsocket   bool
		explicit      string
		wantTransport string
	}{
		{name: "defaults to http1", wantTransport: string(tlsfpTransport.HTTP1)},
		{name: "defaults to h2 from ALPN", alpn: "h2", wantTransport: string(tlsfpTransport.H2)},
		{name: "defaults to websocket-http1", isWebsocket: true, wantTransport: string(tlsfpTransport.WebSocketH1)},
		{name: "defaults to websocket-h2", alpn: "h2", isWebsocket: true, wantTransport: string(tlsfpTransport.WebSocketH2)},
		{name: "explicit transport wins", alpn: "h2", explicit: "custom-http", wantTransport: "custom-http"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTLSFingerprintCaptureRepoStub()
			svc := NewTLSFingerprintCaptureService(repo, nil)

			task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
				Targets:          map[string]int{"openai": 1},
				TransportTargets: map[string]int{tc.wantTransport: 1},
			})
			require.NoError(t, err)

			result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
				Token:          task.Token,
				Platform:       "openai",
				Transport:      tc.explicit,
				ALPNNegotiated: tc.alpn,
				IsWebsocket:    tc.isWebsocket,
				UserAgent:      "codex-tui/0.140.0",
				Originator:     "codex_cli_rs",
				ClientHello:    captureServiceTestClientHello(t, 0),
			})
			require.NoError(t, err)
			require.NotNil(t, result.Sample)
			require.Equal(t, tc.wantTransport, result.Sample.Transport)
			require.Equal(t, 1, result.Task.TransportCounts[tc.wantTransport])
		})
	}
}

func TestTLSFingerprintCaptureServiceCountsDifferentFingerprintsForSameUserAgent(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:       "codex live",
		Targets:    map[string]int{"openai": 2},
		UAKeywords: []string{"Codex Desktop"},
	})
	require.NoError(t, err)

	first, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "Codex Desktop/0.140.0-alpha.2 (Windows 11; x86_64)",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.False(t, first.Duplicate)
	require.Equal(t, map[string]int{"openai": 1}, first.Counts)

	second, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "Codex Desktop/0.140.0-alpha.2 (Windows 11; x86_64)",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 1),
	})
	require.NoError(t, err)
	require.True(t, second.Accepted)
	require.False(t, second.Duplicate)
	require.Equal(t, map[string]int{"openai": 2}, second.Counts)
	require.Equal(t, TLSFingerprintCaptureStatusCompleted, second.Task.Status)
}

func TestTLSFingerprintCaptureServiceDedupesSameFingerprintAcrossUserAgents(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:       "codex live",
		Targets:    map[string]int{"openai": 2},
		UAKeywords: []string{"codex"},
	})
	require.NoError(t, err)

	first, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex-tui/0.140.0 (Debian GNU/Linux 12; x86_64)",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.False(t, first.Duplicate)

	second, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex_exec/0.140.0 (macOS 15.5; arm64)",
		Originator:  "codex_exec",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, second.Accepted)
	require.True(t, second.Duplicate)
	require.Equal(t, map[string]int{"openai": 1}, second.Counts)
	require.Equal(t, TLSFingerprintCaptureStatusRunning, second.Task.Status)
	require.Len(t, repo.samples, 1)
}

func TestTLSFingerprintCaptureServiceStopsCollectingPlatformAfterTargetReached(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:    "multi platform",
		Targets: map[string]int{"openai": 1, "kiro": 1},
	})
	require.NoError(t, err)

	openaiFirst, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, openaiFirst.Accepted)
	require.Equal(t, map[string]int{"openai": 1, "kiro": 0}, openaiFirst.Counts)
	require.Equal(t, TLSFingerprintCaptureStatusRunning, openaiFirst.Task.Status)

	openaiSecond, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		ClientHello: captureServiceTestClientHello(t, 1),
	})
	require.NoError(t, err)
	require.False(t, openaiSecond.Accepted)
	require.Equal(t, "platform_target_reached", openaiSecond.IgnoredReason)
	require.Equal(t, map[string]int{"openai": 1, "kiro": 0}, openaiSecond.Counts)
	require.Len(t, repo.samples, 1)

	kiroFirst, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "kiro",
		UserAgent:   "Kiro/0.1",
		Originator:  "kiro",
		ClientHello: captureServiceTestClientHello(t, 1),
	})
	require.NoError(t, err)
	require.True(t, kiroFirst.Accepted)
	require.Equal(t, map[string]int{"openai": 1, "kiro": 1}, kiroFirst.Counts)
	require.Equal(t, TLSFingerprintCaptureStatusCompleted, kiroFirst.Task.Status)
}

func TestTLSFingerprintCaptureServiceTreatsConcurrentDuplicateCreateAsDuplicate(t *testing.T) {
	repo := &tlsFingerprintCaptureDuplicateCreateRepoStub{
		tlsFingerprintCaptureRepoStub: *newTLSFingerprintCaptureRepoStub(),
	}
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 2},
	})
	require.NoError(t, err)

	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, result.Accepted)
	require.True(t, result.Duplicate)
	require.Equal(t, map[string]int{"openai": 1}, result.Counts)
	require.Len(t, repo.samples, 1)
}

func TestTLSFingerprintCaptureServiceRecountsAfterConcurrentDifferentFingerprintCreate(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	repo.beforeCreateSampleLocked = func(repo *tlsFingerprintCaptureRepoStub, sample *TLSFingerprintCaptureSample) {
		repo.samples = append(repo.samples, &TLSFingerprintCaptureSample{
			ID:              repo.nextSampleID,
			TaskID:          sample.TaskID,
			Platform:        sample.Platform,
			UserAgent:       "codex_exec/0.140.0",
			FingerprintHash: sample.FingerprintHash + "-concurrent",
			Profile:         cloneTLSFingerprintProfile(sample.Profile),
			RawClientHello:  append([]byte(nil), sample.RawClientHello...),
			CreatedAt:       sample.CreatedAt,
		})
		repo.nextSampleID++
		repo.beforeCreateSampleLocked = nil
	}
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 2},
	})
	require.NoError(t, err)

	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.False(t, result.Duplicate)
	require.Equal(t, map[string]int{"openai": 2}, result.Counts)
	require.Equal(t, TLSFingerprintCaptureStatusCompleted, result.Task.Status)
	require.Len(t, repo.samples, 2)
}

func TestTLSFingerprintCaptureServiceIgnoresNonMatchingUserAgent(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:    map[string]int{"openai": 1},
		UAKeywords: []string{"Codex Desktop"},
	})
	require.NoError(t, err)

	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "curl/8.9.1",
		Originator:  "curl",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.False(t, result.Accepted)
	require.Equal(t, "user_agent_not_matched", result.IgnoredReason)
	require.Len(t, repo.samples, 0)
}

func TestTLSFingerprintCaptureServiceListsAndStopsTask(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:    "codex capture",
		Targets: map[string]int{"openai": 1},
	})
	require.NoError(t, err)

	tasks, err := svc.ListTasks(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, task.ID, tasks[0].ID)
	require.Equal(t, TLSFingerprintCaptureStatusRunning, tasks[0].Status)

	stopped, err := svc.StopTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, TLSFingerprintCaptureStatusStopped, stopped.Status)
	require.NotNil(t, stopped.CompletedAt)

	fetched, err := svc.GetTaskByID(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, TLSFingerprintCaptureStatusStopped, fetched.Status)
}

func TestTLSFingerprintCaptureServiceImportsTaskSamplesToProfiles(t *testing.T) {
	captureRepo := newTLSFingerprintCaptureRepoStub()
	profileRepo := &tlsFingerprintProfileImportRepoStub{}
	profileSvc := NewTLSFingerprintProfileService(profileRepo, nil)
	svc := NewTLSFingerprintCaptureService(captureRepo, profileSvc)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:    "codex capture",
		Targets: map[string]int{"openai": 2},
	})
	require.NoError(t, err)

	first, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	second, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "Codex Desktop/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 1),
	})
	require.NoError(t, err)

	result, err := svc.ImportTaskSamples(context.Background(), TLSFingerprintCaptureTaskImportRequest{
		TaskID:    task.ID,
		SampleIDs: []int64{first.Sample.ID, second.Sample.ID},
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.Imported)
	require.Equal(t, 0, result.Duplicates)
	require.Len(t, profileRepo.profiles, 2)
	require.Equal(t, "openai", profileRepo.profiles[0].Platform)
	require.Equal(t, "openai", profileRepo.profiles[1].Platform)
}

func captureServiceTestClientHello(t *testing.T, variant int) []byte {
	t.Helper()

	alpnProtocols := []string{"http/1.1"}
	if variant > 0 {
		alpnProtocols = []string{"h2", "http/1.1"}
	}

	spec := &utls.ClientHelloSpec{
		CipherSuites: []uint16{
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		CompressionMethods: []uint8{0},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519, utls.CurveP256}},
			&utls.SupportedPointsExtension{SupportedPoints: []uint8{0}},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{0x0403, 0x0804, 0x0401}},
			&utls.ALPNExtension{AlpnProtocols: alpnProtocols},
			&utls.SupportedVersionsExtension{Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}},
		},
		TLSVersMin: utls.VersionTLS12,
		TLSVersMax: utls.VersionTLS13,
	}
	uconn := utls.UClient(&net.TCPConn{}, &utls.Config{ServerName: "cloud.example"}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(spec))
	require.NoError(t, uconn.MarshalClientHello())
	require.NotEmpty(t, uconn.HandshakeState.Hello.Raw)

	handshake := uconn.HandshakeState.Hello.Raw
	record := make([]byte, 5+len(handshake))
	recordVersion := uint16(utls.VersionTLS12)
	record[0] = 22
	record[1] = byte(recordVersion >> 8)
	record[2] = byte(recordVersion)
	record[3] = byte(len(handshake) >> 8)
	record[4] = byte(len(handshake))
	copy(record[5:], handshake)
	return record
}

type tlsFingerprintCaptureDuplicateCreateRepoStub struct {
	tlsFingerprintCaptureRepoStub
}

func (r *tlsFingerprintCaptureDuplicateCreateRepoStub) CreateSampleIfAbsent(_ context.Context, sample *TLSFingerprintCaptureSample) (*TLSFingerprintCaptureSample, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	created := cloneTLSFingerprintCaptureSample(sample)
	created.ID = r.nextSampleID
	r.nextSampleID++
	r.samples = append(r.samples, created)
	return cloneTLSFingerprintCaptureSample(created), false, nil
}
