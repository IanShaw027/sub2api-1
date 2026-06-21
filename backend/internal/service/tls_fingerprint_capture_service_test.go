package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSFingerprintCaptureServiceCountsDifferentFingerprintsForSameUserAgent(t *testing.T) {
	repo := newTLSFingerprintCaptureRepoStub()
	svc := NewTLSFingerprintCaptureService(repo, nil)

	task, err := svc.StartTask(context.Background(), TLSFingerprintCaptureStartRequest{
		Name:       "codex live",
		Targets:    map[string]int{"openai": 2},
		UAKeywords: []string{"Codex Desktop"},
	})
	require.NoError(t, err)

	first, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "Codex Desktop/0.140.0-alpha.2 (Windows 11; x86_64)",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51]"),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.False(t, first.Duplicate)
	require.Equal(t, map[string]int{"openai": 1}, first.Counts)

	second, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "Codex Desktop/0.140.0-alpha.2 (Windows 11; x86_64)",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51,65037]"),
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

	first, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "codex-tui/0.140.0 (Debian GNU/Linux 12; x86_64)",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51]"),
	})
	require.NoError(t, err)
	require.True(t, first.Accepted)
	require.False(t, first.Duplicate)

	second, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "codex_exec/0.140.0 (macOS 15.5; arm64)",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51]"),
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

	openaiFirst, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "codex-tui/0.140.0",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51]"),
	})
	require.NoError(t, err)
	require.True(t, openaiFirst.Accepted)
	require.Equal(t, map[string]int{"openai": 1, "kiro": 0}, openaiFirst.Counts)
	require.Equal(t, TLSFingerprintCaptureStatusRunning, openaiFirst.Task.Status)

	openaiSecond, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "codex_exec/0.140.0",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51,65037]"),
	})
	require.NoError(t, err)
	require.False(t, openaiSecond.Accepted)
	require.Equal(t, "platform_target_reached", openaiSecond.IgnoredReason)
	require.Equal(t, map[string]int{"openai": 1, "kiro": 0}, openaiSecond.Counts)
	require.Len(t, repo.samples, 1)

	kiroFirst, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "kiro",
		UserAgent: "Kiro/0.1",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51,65037]"),
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

	result, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "codex-tui/0.140.0",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51]"),
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
			RawPayload:      sample.RawPayload,
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

	result, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "codex-tui/0.140.0",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51]"),
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

	result, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "curl/8.9.1",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51]"),
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

	first, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "codex-tui/0.140.0",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51]"),
	})
	require.NoError(t, err)
	second, err := svc.SubmitCapture(context.Background(), TLSFingerprintCaptureSubmitRequest{
		Token:     task.Token,
		Platform:  "openai",
		UserAgent: "Codex Desktop/0.140.0",
		Payload:   capturePayloadWithExtensions("[0,11,10,13,43,45,51,65037]"),
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

func capturePayloadWithExtensions(extensions string) string {
	return `{
  "name": "captured codex",
  "enable_grease": false,
  "cipher_suites": [4865,4866,4867,49195],
  "curves": [29,23,24],
  "point_formats": [0],
  "signature_algorithms": [1027,2052,1025],
  "alpn_protocols": ["http/1.1"],
  "supported_versions": [772,771],
  "key_share_groups": [29],
  "psk_modes": [1],
  "extensions": ` + extensions + `
	}`
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
