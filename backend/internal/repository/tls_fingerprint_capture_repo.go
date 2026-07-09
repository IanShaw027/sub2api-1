package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintcapturetask"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tlsFingerprintCaptureRepository struct {
	taskRepo         *tlsFingerprintCaptureTaskRepoV2
	sampleRepo       *tlsFingerprintCaptureSampleRepoV2
	sessionRepo      *tlsFingerprintCaptureSessionRepo
	sessionEventRepo *tlsFingerprintCaptureSessionEventRepo
}

func NewTLSFingerprintCaptureRepository(client *ent.Client) service.TLSFingerprintCaptureRepository {
	return &tlsFingerprintCaptureRepository{
		taskRepo:         NewTLSFingerprintCaptureTaskRepoV2(client),
		sampleRepo:       NewTLSFingerprintCaptureSampleRepoV2(client),
		sessionRepo:      NewTLSFingerprintCaptureSessionRepo(client),
		sessionEventRepo: NewTLSFingerprintCaptureSessionEventRepo(client),
	}
}

func (r *tlsFingerprintCaptureRepository) CreateTask(ctx context.Context, task *service.TLSFingerprintCaptureTask) (*service.TLSFingerprintCaptureTask, error) {
	return r.taskRepo.CreateTask(ctx, task)
}

func (r *tlsFingerprintCaptureRepository) ListTasks(ctx context.Context) ([]*service.TLSFingerprintCaptureTask, error) {
	return r.taskRepo.ListTasks(ctx)
}

func (r *tlsFingerprintCaptureRepository) GetTaskByID(ctx context.Context, id int64) (*service.TLSFingerprintCaptureTask, error) {
	return r.taskRepo.GetTaskByID(ctx, id)
}

func (r *tlsFingerprintCaptureRepository) GetRunningTaskByToken(ctx context.Context, token string) (*service.TLSFingerprintCaptureTask, error) {
	return r.taskRepo.GetRunningTaskByToken(ctx, token)
}

func (r *tlsFingerprintCaptureRepository) WithTaskSubmissionLock(ctx context.Context, taskID int64, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	if taskID <= 0 {
		return fmt.Errorf("task id is required")
	}
	if tx := ent.TxFromContext(ctx); tx != nil {
		if _, err := tx.TLSFingerprintCaptureTask.Query().Where(tlsfingerprintcapturetask.ID(taskID)).ForUpdate().Only(ctx); err != nil {
			return err
		}
		return fn(ctx)
	}

	tx, err := r.taskRepo.client.Tx(ctx)
	if err != nil {
		return err
	}
	txCtx := ent.NewTxContext(ctx, tx)
	if _, err := tx.TLSFingerprintCaptureTask.Query().Where(tlsfingerprintcapturetask.ID(taskID)).ForUpdate().Only(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *tlsFingerprintCaptureRepository) UpdateTask(ctx context.Context, task *service.TLSFingerprintCaptureTask) (*service.TLSFingerprintCaptureTask, error) {
	return r.taskRepo.UpdateTask(ctx, task)
}

func (r *tlsFingerprintCaptureRepository) DeleteTask(ctx context.Context, id int64) error {
	return r.taskRepo.DeleteTask(ctx, id)
}

func (r *tlsFingerprintCaptureRepository) DeleteSamplesByTask(ctx context.Context, taskID int64) error {
	return r.sampleRepo.DeleteSamplesByTask(ctx, taskID)
}

func (r *tlsFingerprintCaptureRepository) CreateSampleIfAbsent(ctx context.Context, sample *service.TLSFingerprintCaptureSample) (*service.TLSFingerprintCaptureSample, bool, error) {
	return r.sampleRepo.CreateSampleIfAbsent(ctx, sample)
}

func (r *tlsFingerprintCaptureRepository) GetSampleByTaskHash(ctx context.Context, taskID int64, fingerprintHash string) (*service.TLSFingerprintCaptureSample, error) {
	return r.sampleRepo.GetSampleByTaskHash(ctx, taskID, fingerprintHash)
}

func (r *tlsFingerprintCaptureRepository) ListSamplesByTask(ctx context.Context, taskID int64) ([]*service.TLSFingerprintCaptureSample, error) {
	return r.sampleRepo.ListSamplesByTask(ctx, taskID)
}

func (r *tlsFingerprintCaptureRepository) CreateSessionIfAbsent(ctx context.Context, session *service.TLSFingerprintCaptureSession) (*service.TLSFingerprintCaptureSession, bool, error) {
	return r.sessionRepo.CreateSessionIfAbsent(ctx, session)
}

func (r *tlsFingerprintCaptureRepository) CreateSessionEvent(ctx context.Context, event *service.TLSFingerprintCaptureSessionEvent) (*service.TLSFingerprintCaptureSessionEvent, error) {
	return r.sessionEventRepo.CreateSessionEvent(ctx, event)
}

func tlsCaptureTaskToService(task *ent.TLSFingerprintCaptureTask) *service.TLSFingerprintCaptureTask {
	if task == nil {
		return nil
	}
	out := &service.TLSFingerprintCaptureTask{
		ID:                  task.ID,
		Name:                task.Name,
		Status:              task.Status,
		Token:               task.Token,
		Targets:             copyStringIntMapOrEmpty(task.Targets),
		Counts:              copyStringIntMapOrEmpty(task.Counts),
		TransportTargets:    copyStringIntMapOrEmpty(task.TransportTargets),
		TransportCounts:     copyStringIntMapOrEmpty(task.TransportCounts),
		CaptureFilters:      copyStringAnyMapOrEmpty(task.CaptureFilters),
		SampleSchemaVersion: defaultInt(task.SampleSchemaVersion, 2),
		TaskStats:           copyStringAnyMapOrEmpty(task.TaskStats),
		UAKeywords:          copyStringSliceOrEmpty(task.UaKeywords),
		CreatedAt:           task.CreatedAt,
		UpdatedAt:           task.UpdatedAt,
		CompletedAt:         task.CompletedAt,
	}
	return out
}

func tlsCaptureSampleToService(sample *ent.TLSFingerprintCaptureSample) *service.TLSFingerprintCaptureSample {
	if sample == nil {
		return nil
	}
	profile := sample.ReplayProfile
	if profile == nil {
		profile = &model.TLSFingerprintProfile{}
	}
	return &service.TLSFingerprintCaptureSample{
		ID:                sample.ID,
		TaskID:            sample.TaskID,
		Platform:          strings.TrimSpace(sample.Platform),
		Transport:         strings.TrimSpace(sample.Transport),
		SessionID:         strings.TrimSpace(sample.SessionID),
		UserAgent:         sample.UserAgent,
		Originator:        strings.TrimSpace(sample.Originator),
		FingerprintHash:   strings.TrimSpace(sample.FingerprintHash),
		ReplayHash:        normalizeTLSCaptureStoredReplayHash(sample.ReplayHash, sample.FingerprintHash),
		JA3Raw:            strings.TrimSpace(sample.Ja3Raw),
		JA3Hash:           strings.TrimSpace(sample.Ja3Hash),
		JA4:               strings.TrimSpace(sample.Ja4),
		RequestPath:       sample.RequestPath,
		HTTPMethod:        sample.HTTPMethod,
		IsWebsocket:       sample.IsWebsocket,
		WebsocketProtocol: sample.WebsocketProtocol,
		ClientType:        sample.ClientType,
		Model:             sample.Model,
		RequestKind:       sample.RequestKind,
		Streaming:         sample.Streaming,
		ResponseMode:      sample.ResponseMode,
		HTTP2Fingerprint:  sample.Http2Fingerprint,
		StainlessMetadata: copyStringAnyMapOrEmpty(sample.StainlessMetadata),
		Profile:           profile,
		RawPayload:        sample.RawPayload,
		RawClientHello:    cloneBytesPtr(sample.RawClientHello),
		CapturedAt:        coalesceTime(sample.CapturedAt, sample.CreatedAt),
		CreatedAt:         sample.CreatedAt,
	}
}

func tlsCaptureSessionToService(session *ent.TLSFingerprintCaptureSession) *service.TLSFingerprintCaptureSession {
	if session == nil {
		return nil
	}
	return &service.TLSFingerprintCaptureSession{
		ID:                  session.ID,
		TaskID:              session.TaskID,
		SessionID:           strings.TrimSpace(session.SessionID),
		ClientIP:            strings.TrimSpace(session.ClientIP),
		Platform:            strings.TrimSpace(session.Platform),
		UserAgent:           session.UserAgent,
		Originator:          strings.TrimSpace(session.Originator),
		ALPNNegotiated:      strings.TrimSpace(session.AlpnNegotiated),
		RawClientHello:      cloneBytesPtr(session.RawClientHello),
		ObservedClientHello: copyStringAnyMapOrEmpty(session.ObservedClientHello),
		ReplayProfile:       copyStringAnyMapOrEmpty(session.ReplayProfile),
		DerivedFingerprint:  copyStringAnyMapOrEmpty(session.DerivedFingerprint),
		SessionStatus:       strings.TrimSpace(session.SessionStatus),
		ErrorSummary:        session.ErrorSummary,
		OpenedAt:            session.OpenedAt,
		ClosedAt:            cloneTimePtr(session.ClosedAt),
		CreatedAt:           session.CreatedAt,
		UpdatedAt:           session.UpdatedAt,
	}
}

func tlsCaptureSessionEventToService(event *ent.TLSFingerprintCaptureSessionEvent) *service.TLSFingerprintCaptureSessionEvent {
	if event == nil {
		return nil
	}
	return &service.TLSFingerprintCaptureSessionEvent{
		ID:                event.ID,
		TaskID:            event.TaskID,
		SessionRef:        valueOrZero(event.SessionRef),
		SessionID:         strings.TrimSpace(event.SessionID),
		EventID:           strings.TrimSpace(event.EventID),
		Platform:          strings.TrimSpace(event.Platform),
		Transport:         strings.TrimSpace(event.Transport),
		EventType:         strings.TrimSpace(event.EventType),
		RequestSequence:   event.RequestSequence,
		StreamID:          strings.TrimSpace(event.StreamID),
		RequestPath:       event.RequestPath,
		HTTPMethod:        event.HTTPMethod,
		IsWebsocket:       event.IsWebsocket,
		WebsocketProtocol: strings.TrimSpace(event.WebsocketProtocol),
		ClientType:        strings.TrimSpace(event.ClientType),
		Model:             event.Model,
		RequestKind:       strings.TrimSpace(event.RequestKind),
		Streaming:         event.Streaming,
		ResponseMode:      strings.TrimSpace(event.ResponseMode),
		UserAgent:         event.UserAgent,
		Originator:        strings.TrimSpace(event.Originator),
		StainlessMetadata: copyStringAnyMapOrEmpty(event.StainlessMetadata),
		HeadersSnapshot:   copyStringAnyMapOrEmpty(event.HeadersSnapshot),
		BodySummary:       event.BodySummary,
		RawPayload:        event.RawPayload,
		EventStatus:       strings.TrimSpace(event.EventStatus),
		Error:             event.EventError,
		Replayable:        event.Replayable,
		SampleID:          cloneInt64Ptr(event.SampleID),
		ReplayHash:        strings.TrimSpace(event.ReplayHash),
		CreatedAt:         event.CreatedAt,
	}
}

func normalizeTLSCaptureStoredReplayHash(replayHash, fingerprintHash string) string {
	replayHash = strings.TrimSpace(replayHash)
	if replayHash != "" {
		return replayHash
	}
	fingerprintHash = strings.TrimSpace(fingerprintHash)
	if idx := strings.Index(fingerprintHash, "::"); idx >= 0 {
		return fingerprintHash[:idx]
	}
	return fingerprintHash
}

func copyStringIntMapOrEmpty(in map[string]int) map[string]int {
	if len(in) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyStringSliceOrEmpty(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	return append([]string(nil), in...)
}

func cloneBytesPtr(in *[]byte) []byte {
	if in == nil {
		return nil
	}
	return append([]byte(nil), (*in)...)
}

func cloneInt64Ptr(in *int64) *int64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func cloneTimePtr(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func coalesceTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func valueOrZero(in *int64) int64 {
	if in == nil {
		return 0
	}
	return *in
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func copyStringAnyMapOrEmpty(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
