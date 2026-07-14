package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
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

func TestTLSCaptureSampleDedupesExactReplayableObservation(t *testing.T) {
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
		RequestPath: "/v1/responses",
		HTTPMethod:  "POST",
		RawPayload:  `{"model":"gpt-5.4","input":"same"}`,
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
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		RequestPath: "/v1/responses",
		HTTPMethod:  "POST",
		RawPayload:  `{"model":"gpt-5.4","input":"same"}`,
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

func TestTLSCaptureSamplePayloadMergesSampleDimensionsIntoProfile(t *testing.T) {
	payload, err := tlsCaptureSamplePayload(&TLSFingerprintCaptureSample{
		ClientType:        "codex-cli",
		HTTP2Fingerprint:  "1:4096|ph::method,:scheme,:authority,:path",
		StainlessMetadata: map[string]any{"os": "Linux"},
		Profile: &model.TLSFingerprintProfile{
			Name:      "captured",
			Platform:  "openai",
			Transport: "h2",
		},
	})

	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &got))
	require.Equal(t, "linux", got["os"])
	require.Equal(t, "codex-cli", got["client_type"])
	require.Equal(t, "1:4096|ph::method,:scheme,:authority,:path", got["http2_fingerprint"])
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

func TestTLSCaptureSessionSameTransportDistinctPayloadCreatesReplayableSamples(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:          map[string]int{"openai": 2},
		TransportTargets: map[string]int{string(tlsfpTransport.WebSocketH1): 2},
		UAKeywords:       []string{"codex"},
	})
	require.NoError(t, err)

	first, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:           task.Token,
		Platform:        "openai",
		Transport:       string(tlsfpTransport.WebSocketH1),
		SessionID:       "sess-sticky",
		UserAgent:       "codex-tui/0.140.0",
		Originator:      "codex_cli_rs",
		RequestPath:     "/v1/responses",
		HTTPMethod:      "POST",
		RawPayload:      `{"model":"gpt-5.4","input":"capture-1"}`,
		RequestSequence: 1,
		ClientHello:     captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.NotNil(t, first.Sample)

	second, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:           task.Token,
		Platform:        "openai",
		Transport:       string(tlsfpTransport.WebSocketH1),
		SessionID:       "sess-sticky",
		UserAgent:       "codex-tui/0.140.0",
		Originator:      "codex_cli_rs",
		RequestPath:     "/v1/responses",
		HTTPMethod:      "POST",
		RawPayload:      `{"model":"gpt-5.4","input":"capture-2"}`,
		RequestSequence: 2,
		ClientHello:     captureServiceTestClientHello(t, 1),
	})
	require.NoError(t, err)
	require.True(t, second.Accepted)
	require.False(t, second.Duplicate)
	require.NotNil(t, second.Session)
	require.NotNil(t, second.SessionEvent)
	require.NotNil(t, second.Sample)
	require.NotEqual(t, first.Sample.ID, second.Sample.ID)
	require.Equal(t, map[string]int{"openai": 2}, second.Counts)
	require.Equal(t, map[string]int{string(tlsfpTransport.WebSocketH1): 2}, second.Task.TransportCounts)
	require.Equal(t, TLSFingerprintCaptureStatusCompleted, second.Task.Status)
	require.Len(t, repo.samples, 2)
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
		name             string
		alpn             string
		isWebsocket      bool
		explicit         string
		http2Fingerprint string
		clientHelloKind  int
		wantTransport    string
	}{
		{name: "defaults to http1", wantTransport: string(tlsfpTransport.HTTP1)},
		{name: "defaults to h2 from ALPN", alpn: "h2", http2Fingerprint: "1:4096|ph::method,:scheme,:authority,:path", clientHelloKind: 1, wantTransport: string(tlsfpTransport.H2)},
		{name: "defaults to websocket-http1", isWebsocket: true, wantTransport: string(tlsfpTransport.WebSocketH1)},
		{name: "defaults to websocket-h2", alpn: "h2", isWebsocket: true, http2Fingerprint: "1:4096|ph::method,:scheme,:authority,:path,:protocol", clientHelloKind: 1, wantTransport: string(tlsfpTransport.WebSocketH2)},
		{name: "explicit transport wins", alpn: "h2", explicit: string(tlsfpTransport.HTTP1), clientHelloKind: 1, wantTransport: string(tlsfpTransport.HTTP1)},
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
				Token:            task.Token,
				Platform:         "openai",
				Transport:        tc.explicit,
				ALPNNegotiated:   tc.alpn,
				IsWebsocket:      tc.isWebsocket,
				UserAgent:        "codex-tui/0.140.0",
				Originator:       "codex_cli_rs",
				HTTP2Fingerprint: tc.http2Fingerprint,
				ClientHello:      captureServiceTestClientHello(t, tc.clientHelloKind),
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

func TestTLSFingerprintCaptureServiceKeepsDistinctUserAgentsAsSeparateSamples(t *testing.T) {
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
	require.False(t, second.Duplicate)
	require.Equal(t, map[string]int{"openai": 2}, second.Counts)
	require.Equal(t, TLSFingerprintCaptureStatusCompleted, second.Task.Status)
	require.Len(t, repo.samples, 2)
}

func TestTLSFingerprintCaptureServiceDoesNotDedupeAcrossPlatformsWithSameReplayHash(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:    "multi platform exact replay hash",
		Targets: map[string]int{"openai": 1, "kiro": 1},
	})
	require.NoError(t, err)

	first, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex_exec/0.140.0",
		Originator:  "codex_exec",
		RequestPath: "/v1/responses",
		HTTPMethod:  "POST",
		RawPayload:  `{"model":"gpt-5.4","input":"capture-platform-a"}`,
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.False(t, first.Duplicate)

	second, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "kiro",
		UserAgent:   "kiro/0.140.0",
		Originator:  "kiro",
		RequestPath: "/v1/responses",
		HTTPMethod:  "POST",
		RawPayload:  `{"model":"gpt-5.4","input":"capture-platform-b"}`,
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, second.Accepted)
	require.False(t, second.Duplicate)
	require.Equal(t, map[string]int{"openai": 1, "kiro": 1}, second.Counts)
	require.Equal(t, TLSFingerprintCaptureStatusCompleted, second.Task.Status)
	require.Len(t, repo.samples, 2)
	require.Equal(t, "openai", repo.samples[0].Platform)
	require.Equal(t, "kiro", repo.samples[1].Platform)
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

func TestTLSFingerprintCaptureServiceSerializesQuotaCheckAgainstConcurrentDifferentFingerprintCreate(t *testing.T) {
	repo := &tlsFingerprintCaptureBlockingCreateRepoStub{
		tlsFingerprintCaptureRepoStub: newTLSFingerprintCaptureRepoStub(),
		createEntered:                 make(chan struct{}, 4),
		releaseCreate:                 make(chan struct{}),
		tokenValidated:                make(chan struct{}, 2),
	}
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 1},
	})
	require.NoError(t, err)

	type submitOutcome struct {
		result *TLSFingerprintCaptureSubmitResult
		err    error
	}

	firstDone := make(chan submitOutcome, 1)
	go func() {
		result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
			Token:       task.Token,
			Platform:    "openai",
			UserAgent:   "codex-tui/0.140.0",
			Originator:  "codex_cli_rs",
			ClientHello: captureServiceTestClientHello(t, 0),
		})
		firstDone <- submitOutcome{result: result, err: err}
	}()

	select {
	case <-repo.createEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first submission did not reach CreateSampleIfAbsent")
	}
	// Consume the first submission's validation notification. The second
	// submission must independently pass the initial token lookup before the
	// first one is released and completes the task.
	select {
	case <-repo.tokenValidated:
	case <-time.After(2 * time.Second):
		t.Fatal("first submission did not validate the running task token")
	}

	secondDone := make(chan submitOutcome, 1)
	go func() {
		result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
			Token:       task.Token,
			Platform:    "openai",
			UserAgent:   "Codex Desktop/0.140.0",
			Originator:  "codex_cli_rs",
			ClientHello: captureServiceTestClientHello(t, 1),
		})
		secondDone <- submitOutcome{result: result, err: err}
	}()

	select {
	case <-repo.tokenValidated:
	case <-time.After(2 * time.Second):
		t.Fatal("second submission did not validate the running task token")
	}

	select {
	case <-repo.createEntered:
		t.Fatal("second concurrent submission reached CreateSampleIfAbsent before first create finished")
	case <-time.After(150 * time.Millisecond):
	}

	close(repo.releaseCreate)

	first := <-firstDone
	require.NoError(t, first.err)
	require.NotNil(t, first.result)
	require.True(t, first.result.Accepted)
	require.False(t, first.result.Duplicate)

	second := <-secondDone
	require.NoError(t, second.err)
	require.NotNil(t, second.result)
	require.False(t, second.result.Accepted)
	require.Equal(t, "platform_target_reached", second.result.IgnoredReason)

	require.Len(t, repo.samples, 1)
	require.Equal(t, map[string]int{"openai": 1}, second.result.Counts)
}

func TestTLSFingerprintCaptureServiceAllowsDifferentTasksToSubmitConcurrently(t *testing.T) {
	repo := &tlsFingerprintCaptureTaskSelectiveBlockingRepoStub{
		tlsFingerprintCaptureRepoStub: newTLSFingerprintCaptureRepoStub(),
		createEntered:                 make(chan int64, 4),
		releaseCreate:                 make(chan struct{}),
	}
	svc := NewTLSFingerprintCaptureService(repo, nil)

	firstTask, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 1},
	})
	require.NoError(t, err)
	secondTask, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 1},
	})
	require.NoError(t, err)
	repo.blockedTaskID = firstTask.ID

	type submitOutcome struct {
		result *TLSFingerprintCaptureSubmitResult
		err    error
	}

	firstDone := make(chan submitOutcome, 1)
	go func() {
		result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
			Token:       firstTask.Token,
			Platform:    "openai",
			UserAgent:   "codex-tui/0.140.0",
			Originator:  "codex_cli_rs",
			ClientHello: captureServiceTestClientHello(t, 0),
		})
		firstDone <- submitOutcome{result: result, err: err}
	}()

	select {
	case taskID := <-repo.createEntered:
		require.Equal(t, firstTask.ID, taskID)
	case <-time.After(2 * time.Second):
		t.Fatal("first task submission did not reach CreateSampleIfAbsent")
	}

	secondDone := make(chan submitOutcome, 1)
	go func() {
		result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
			Token:       secondTask.Token,
			Platform:    "openai",
			UserAgent:   "Codex Desktop/0.140.0",
			Originator:  "codex_cli_rs",
			ClientHello: captureServiceTestClientHello(t, 1),
		})
		secondDone <- submitOutcome{result: result, err: err}
	}()

	select {
	case taskID := <-repo.createEntered:
		require.Equal(t, secondTask.ID, taskID)
	case <-time.After(150 * time.Millisecond):
		t.Fatal("second task submission was blocked by unrelated task")
	}

	close(repo.releaseCreate)

	first := <-firstDone
	require.NoError(t, first.err)
	require.NotNil(t, first.result)
	require.True(t, first.result.Accepted)

	second := <-secondDone
	require.NoError(t, second.err)
	require.NotNil(t, second.result)
	require.True(t, second.result.Accepted)

	require.Len(t, repo.samples, 2)
}

func TestTLSFingerprintCaptureServiceUsesRepositoryTaskSubmissionLock(t *testing.T) {
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
	require.NotNil(t, result)
	require.Equal(t, 1, repo.withTaskSubmissionLockCalls)
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
	require.NotEmpty(t, task.Token)

	tasks, err := svc.ListTasks(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, task.ID, tasks[0].ID)
	require.Equal(t, TLSFingerprintCaptureStatusRunning, tasks[0].Status)
	require.Empty(t, tasks[0].Token, "list responses must redact capture tokens")

	// Single-task fetch still returns the token for operator workflows.
	running, err := svc.GetTaskByID(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, task.Token, running.Token)

	stopped, err := svc.StopTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, TLSFingerprintCaptureStatusStopped, stopped.Status)
	require.NotNil(t, stopped.CompletedAt)

	fetched, err := svc.GetTaskByID(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, TLSFingerprintCaptureStatusStopped, fetched.Status)
}

func TestTLSFingerprintCaptureServiceRestartClearsStaleSamplesFromQuota(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	// Target > 1 so the first sample does not auto-complete the task; we stop explicitly.
	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:    "codex capture",
		Targets: map[string]int{"openai": 2},
	})
	require.NoError(t, err)

	accepted, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		SessionID:   "generation-session",
		UserAgent:   "codex-cli/1.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, accepted.Accepted)
	require.Equal(t, 1, accepted.Task.Counts["openai"])
	require.Equal(t, TLSFingerprintCaptureStatusRunning, accepted.Task.Status)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 1)

	stopped, err := svc.StopTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, TLSFingerprintCaptureStatusStopped, stopped.Status)
	stopped.TaskStats = map[string]any{"generation": "old"}
	stopped.Counts = map[string]int{"stale-target": 9}
	_, err = repo.UpdateTask(context.Background(), stopped)
	require.NoError(t, err)

	restarted, err := svc.RestartTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, TLSFingerprintCaptureStatusRunning, restarted.Status)
	require.NotEmpty(t, restarted.Token)
	require.NotEqual(t, task.Token, restarted.Token)
	require.Equal(t, map[string]int{"openai": 0}, restarted.Counts)
	require.Empty(t, restarted.TaskStats)

	samples, err := svc.ListSamplesByTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Empty(t, samples, "restart must drop prior samples so quota starts clean")
	require.Empty(t, repo.sessions, "restart must drop prior sessions so session IDs can be reused")
	require.Empty(t, repo.sessionEvents, "restart must drop events that reference prior samples and sessions")

	// Same ClientHello can be accepted again under the new generation.
	again, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       restarted.Token,
		Platform:    "openai",
		SessionID:   "generation-session",
		UserAgent:   "codex-cli/1.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, again.Accepted)
	require.Equal(t, 1, again.Task.Counts["openai"])
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 1)
}

func TestTLSFingerprintCaptureServiceRestartRollsBackGenerationCleanupWhenUpdateFails(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 2},
	})
	require.NoError(t, err)
	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		SessionID:   "rollback-session",
		UserAgent:   "codex-cli/1.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, result.Accepted)
	stopped, err := svc.StopTask(context.Background(), task.ID)
	require.NoError(t, err)

	repo.updateTaskErr = errors.New("update task failed")
	_, err = svc.RestartTask(context.Background(), task.ID)
	require.EqualError(t, err, "update task failed")

	stored, err := repo.GetTaskByID(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, stopped.Status, stored.Status)
	require.Equal(t, stopped.Token, stored.Token)
	require.Equal(t, stopped.Counts, stored.Counts)
	require.Len(t, repo.samples, 1)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 1)
}

func TestTLSFingerprintCaptureServiceDeleteIsAtomicAndCleansGenerationData(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 2},
	})
	require.NoError(t, err)
	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		SessionID:   "delete-session",
		UserAgent:   "codex-cli/1.0",
		Originator:  "codex_cli_rs",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, result.Accepted)
	_, err = svc.StopTask(context.Background(), task.ID)
	require.NoError(t, err)

	repo.deleteTaskErr = errors.New("delete task failed")
	err = svc.DeleteTask(context.Background(), task.ID)
	require.EqualError(t, err, "delete task failed")
	require.Len(t, repo.tasks, 1)
	require.Len(t, repo.samples, 1)
	require.Len(t, repo.sessions, 1)
	require.Len(t, repo.sessionEvents, 1)

	repo.deleteTaskErr = nil
	require.NoError(t, svc.DeleteTask(context.Background(), task.ID))
	require.Empty(t, repo.tasks)
	require.Empty(t, repo.samples)
	require.Empty(t, repo.sessions)
	require.Empty(t, repo.sessionEvents)
}

func TestTLSFingerprintCaptureServiceRejectsOldTokenAfterWaitingForTaskLock(t *testing.T) {
	repo := &tlsFingerprintCaptureBlockingLockRepoStub{
		tlsFingerprintCaptureRepoStub: newTLSFingerprintCaptureRepoStub(),
		lockEntered:                   make(chan struct{}, 1),
		releaseLock:                   make(chan struct{}),
	}
	svc := NewTLSFingerprintCaptureService(repo, nil)
	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets: map[string]int{"openai": 2},
	})
	require.NoError(t, err)
	clientHello := captureServiceTestClientHello(t, 0)

	done := make(chan error, 1)
	go func() {
		_, submitErr := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
			Token:       task.Token,
			Platform:    "openai",
			SessionID:   "stale-token-session",
			UserAgent:   "codex-cli/1.0",
			Originator:  "codex_cli_rs",
			ClientHello: clientHello,
		})
		done <- submitErr
	}()

	select {
	case <-repo.lockEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("submission did not reach the task lock")
	}
	// Simulate another process committing a restart while this process waits for
	// the database row lock. The locked re-read must reject the old generation.
	repo.mu.Lock()
	repo.tasks[0].Token = "new-generation-token"
	repo.mu.Unlock()
	close(repo.releaseLock)

	err = <-done
	require.Error(t, err)
	require.Contains(t, err.Error(), "running capture task not found")
	require.Empty(t, repo.samples)
	require.Empty(t, repo.sessions)
	require.Empty(t, repo.sessionEvents)
}

func TestTLSFingerprintCaptureServiceDoesNotStoreBodyByDefault(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:    "codex capture",
		Targets: map[string]int{"openai": 2},
	})
	require.NoError(t, err)

	result, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex-cli/1.0",
		Originator:  "codex_cli_rs",
		RawPayload:  `{"secret":"should-not-persist"}`,
		BodySummary: "summary-only",
		ClientHello: captureServiceTestClientHello(t, 0),
	})
	require.NoError(t, err)
	require.True(t, result.Accepted)
	require.NotNil(t, result.Sample)
	require.Empty(t, result.Sample.RawPayload)
	require.NotNil(t, result.SessionEvent)
	require.Empty(t, result.SessionEvent.RawPayload)

	// Opt-in store_body keeps the payload for debugging.
	task.CaptureFilters = map[string]any{"store_body": true}
	_, err = repo.UpdateTask(context.Background(), task)
	require.NoError(t, err)

	stored, err := svc.SubmitNativeCapture(context.Background(), TLSFingerprintCaptureNativeSubmitRequest{
		Token:       task.Token,
		Platform:    "openai",
		UserAgent:   "codex-cli/1.1",
		Originator:  "codex_cli_rs",
		RawPayload:  `{"ok":true}`,
		ClientHello: captureServiceTestClientHello(t, 1),
	})
	require.NoError(t, err)
	require.True(t, stored.Accepted)
	require.Equal(t, `{"ok":true}`, stored.Sample.RawPayload)
}

func TestTLSFingerprintCaptureServiceStartTaskPersistsCaptureFilters(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Targets:        map[string]int{"openai": 1},
		CaptureFilters: map[string]any{"store_body": true},
	})
	require.NoError(t, err)
	require.Equal(t, true, task.CaptureFilters["store_body"])
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
		Token:            task.Token,
		Platform:         "openai",
		Transport:        "h2",
		UserAgent:        "codex-tui/0.140.0",
		Originator:       "codex_cli_rs",
		HTTP2Fingerprint: "4:65535,1:4096,3:100|wu:12345|ph::method,:scheme,:authority,:path",
		ClientHello:      captureServiceTestClientHello(t, 1),
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
	require.Equal(t, "4:65535,1:4096,3:100|wu:12345|ph::method,:scheme,:authority,:path", profileRepo.profiles[0].HTTP2Fingerprint)
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

type tlsFingerprintCaptureBlockingCreateRepoStub struct {
	*tlsFingerprintCaptureRepoStub
	createEntered  chan struct{}
	releaseCreate  chan struct{}
	tokenValidated chan struct{}
}

type tlsFingerprintCaptureBlockingLockRepoStub struct {
	*tlsFingerprintCaptureRepoStub
	lockEntered chan struct{}
	releaseLock chan struct{}
}

func (r *tlsFingerprintCaptureBlockingLockRepoStub) WithTaskSubmissionLock(ctx context.Context, taskID int64, fn func(context.Context) error) error {
	if r.lockEntered != nil {
		r.lockEntered <- struct{}{}
	}
	if r.releaseLock != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.releaseLock:
		}
	}
	return r.tlsFingerprintCaptureRepoStub.WithTaskSubmissionLock(ctx, taskID, fn)
}

func (r *tlsFingerprintCaptureBlockingCreateRepoStub) GetRunningTaskByToken(ctx context.Context, token string) (*TLSFingerprintCaptureTask, error) {
	task, err := r.tlsFingerprintCaptureRepoStub.GetRunningTaskByToken(ctx, token)
	if err == nil && task != nil && r.tokenValidated != nil {
		r.tokenValidated <- struct{}{}
	}
	return task, err
}

func (r *tlsFingerprintCaptureBlockingCreateRepoStub) CreateSampleIfAbsent(ctx context.Context, sample *TLSFingerprintCaptureSample) (*TLSFingerprintCaptureSample, bool, error) {
	if r.createEntered != nil {
		r.createEntered <- struct{}{}
	}
	if r.releaseCreate != nil {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-r.releaseCreate:
		}
	}
	return r.tlsFingerprintCaptureRepoStub.CreateSampleIfAbsent(ctx, sample)
}

type tlsFingerprintCaptureTaskSelectiveBlockingRepoStub struct {
	*tlsFingerprintCaptureRepoStub
	blockedTaskID int64
	createEntered chan int64
	releaseCreate chan struct{}
}

func (r *tlsFingerprintCaptureTaskSelectiveBlockingRepoStub) CreateSampleIfAbsent(ctx context.Context, sample *TLSFingerprintCaptureSample) (*TLSFingerprintCaptureSample, bool, error) {
	if r.createEntered != nil {
		r.createEntered <- sample.TaskID
	}
	if r.releaseCreate != nil && sample.TaskID == r.blockedTaskID {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-r.releaseCreate:
		}
	}
	return r.tlsFingerprintCaptureRepoStub.CreateSampleIfAbsent(ctx, sample)
}
