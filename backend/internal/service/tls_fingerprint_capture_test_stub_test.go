package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

type tlsFingerprintCaptureRepoStub struct {
	mu            sync.Mutex
	nextTaskID    int64
	nextSampleID  int64
	nextSessionID int64
	nextEventID   int64
	tasks         []*TLSFingerprintCaptureTask
	samples       []*TLSFingerprintCaptureSample
	sessions      []*TLSFingerprintCaptureSession
	sessionEvents []*TLSFingerprintCaptureSessionEvent

	getRunningTaskByTokenErr error
	updateTaskErr            error
	deleteTaskErr            error
	deleteTaskCaptureDataErr error

	beforeCreateSampleLocked    func(*tlsFingerprintCaptureRepoStub, *TLSFingerprintCaptureSample)
	withTaskSubmissionLockCalls int
}

func newTLSFingerprintCaptureRepoStub() *tlsFingerprintCaptureRepoStub {
	return &tlsFingerprintCaptureRepoStub{nextTaskID: 1, nextSampleID: 1, nextSessionID: 1, nextEventID: 1}
}

func (r *tlsFingerprintCaptureRepoStub) CreateTask(_ context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	created := cloneTLSFingerprintCaptureTask(task)
	created.ID = r.nextTaskID
	r.nextTaskID++
	now := time.Now().UTC()
	created.CreatedAt = now
	created.UpdatedAt = now
	r.tasks = append(r.tasks, created)
	return cloneTLSFingerprintCaptureTask(created), nil
}

func (r *tlsFingerprintCaptureRepoStub) ListTasks(_ context.Context) ([]*TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*TLSFingerprintCaptureTask, 0, len(r.tasks))
	for _, task := range r.tasks {
		out = append(out, cloneTLSFingerprintCaptureTask(task))
	}
	return out, nil
}

func (r *tlsFingerprintCaptureRepoStub) GetTaskByID(_ context.Context, id int64) (*TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, task := range r.tasks {
		if task.ID == id {
			return cloneTLSFingerprintCaptureTask(task), nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintCaptureRepoStub) GetRunningTaskByToken(_ context.Context, token string) (*TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getRunningTaskByTokenErr != nil {
		return nil, r.getRunningTaskByTokenErr
	}
	for _, task := range r.tasks {
		if task.Token == token && task.Status == TLSFingerprintCaptureStatusRunning {
			return cloneTLSFingerprintCaptureTask(task), nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintCaptureRepoStub) WithTaskSubmissionLock(ctx context.Context, _ int64, fn func(context.Context) error) error {
	r.mu.Lock()
	r.withTaskSubmissionLockCalls++
	tasks := cloneTLSFingerprintCaptureTasks(r.tasks)
	samples := cloneTLSFingerprintCaptureSamples(r.samples)
	sessions := cloneTLSFingerprintCaptureSessions(r.sessions)
	events := cloneTLSFingerprintCaptureSessionEvents(r.sessionEvents)
	r.mu.Unlock()
	if fn == nil {
		return nil
	}
	if err := fn(ctx); err != nil {
		r.mu.Lock()
		r.tasks = tasks
		r.samples = samples
		r.sessions = sessions
		r.sessionEvents = events
		r.mu.Unlock()
		return err
	}
	return nil
}

func (r *tlsFingerprintCaptureRepoStub) FailGetRunningTaskByToken(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		err = errors.New("get running task failed")
	}
	r.getRunningTaskByTokenErr = err
}

func (r *tlsFingerprintCaptureRepoStub) UpdateTask(_ context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateTaskErr != nil {
		return nil, r.updateTaskErr
	}
	updated := cloneTLSFingerprintCaptureTask(task)
	updated.UpdatedAt = time.Now().UTC()
	for i, existing := range r.tasks {
		if existing.ID == task.ID {
			r.tasks[i] = updated
			return cloneTLSFingerprintCaptureTask(updated), nil
		}
	}
	r.tasks = append(r.tasks, updated)
	return cloneTLSFingerprintCaptureTask(updated), nil
}

func (r *tlsFingerprintCaptureRepoStub) CreateSampleIfAbsent(_ context.Context, sample *TLSFingerprintCaptureSample) (*TLSFingerprintCaptureSample, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.beforeCreateSampleLocked != nil {
		r.beforeCreateSampleLocked(r, sample)
	}
	for _, existing := range r.samples {
		if existing.TaskID == sample.TaskID && existing.FingerprintHash == sample.FingerprintHash {
			return cloneTLSFingerprintCaptureSample(existing), false, nil
		}
	}
	created := cloneTLSFingerprintCaptureSample(sample)
	created.ID = r.nextSampleID
	r.nextSampleID++
	created.CreatedAt = time.Now().UTC()
	r.samples = append(r.samples, created)
	return cloneTLSFingerprintCaptureSample(created), true, nil
}

func (r *tlsFingerprintCaptureRepoStub) GetSampleByTaskHash(_ context.Context, taskID int64, fingerprintHash string) (*TLSFingerprintCaptureSample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sample := range r.samples {
		if sample.TaskID == taskID && sample.FingerprintHash == fingerprintHash {
			return cloneTLSFingerprintCaptureSample(sample), nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintCaptureRepoStub) ListSamplesByTask(_ context.Context, taskID int64) ([]*TLSFingerprintCaptureSample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*TLSFingerprintCaptureSample, 0, len(r.samples))
	for _, sample := range r.samples {
		if sample.TaskID == taskID {
			out = append(out, cloneTLSFingerprintCaptureSample(sample))
		}
	}
	return out, nil
}

func (r *tlsFingerprintCaptureRepoStub) DeleteTask(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleteTaskErr != nil {
		return r.deleteTaskErr
	}
	for i, task := range r.tasks {
		if task.ID == id {
			r.tasks = append(r.tasks[:i], r.tasks[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *tlsFingerprintCaptureRepoStub) DeleteTaskCaptureData(_ context.Context, taskID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleteTaskCaptureDataErr != nil {
		return r.deleteTaskCaptureDataErr
	}
	samples := r.samples[:0]
	for _, sample := range r.samples {
		if sample.TaskID != taskID {
			samples = append(samples, sample)
		}
	}
	r.samples = samples
	sessions := r.sessions[:0]
	for _, session := range r.sessions {
		if session.TaskID != taskID {
			sessions = append(sessions, session)
		}
	}
	r.sessions = sessions
	events := r.sessionEvents[:0]
	for _, event := range r.sessionEvents {
		if event.TaskID != taskID {
			events = append(events, event)
		}
	}
	r.sessionEvents = events
	return nil
}

func (r *tlsFingerprintCaptureRepoStub) CreateSessionIfAbsent(_ context.Context, session *TLSFingerprintCaptureSession) (*TLSFingerprintCaptureSession, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.sessions {
		if existing.TaskID == session.TaskID && existing.SessionID == session.SessionID {
			return cloneTLSFingerprintCaptureSession(existing), false, nil
		}
	}
	created := cloneTLSFingerprintCaptureSession(session)
	created.ID = r.nextSessionID
	r.nextSessionID++
	now := time.Now().UTC()
	created.CreatedAt = now
	created.UpdatedAt = now
	r.sessions = append(r.sessions, created)
	return cloneTLSFingerprintCaptureSession(created), true, nil
}

func (r *tlsFingerprintCaptureRepoStub) CreateSessionEvent(_ context.Context, event *TLSFingerprintCaptureSessionEvent) (*TLSFingerprintCaptureSessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	created := cloneTLSFingerprintCaptureSessionEvent(event)
	created.ID = r.nextEventID
	r.nextEventID++
	created.CreatedAt = time.Now().UTC()
	r.sessionEvents = append(r.sessionEvents, created)
	return cloneTLSFingerprintCaptureSessionEvent(created), nil
}

func cloneTLSFingerprintCaptureTask(task *TLSFingerprintCaptureTask) *TLSFingerprintCaptureTask {
	if task == nil {
		return nil
	}
	clone := *task
	clone.Targets = copyStringIntMap(task.Targets)
	clone.Counts = copyStringIntMap(task.Counts)
	clone.TransportTargets = copyStringIntMap(task.TransportTargets)
	clone.TransportCounts = copyStringIntMap(task.TransportCounts)
	clone.CaptureFilters = cloneStringAnyMap(task.CaptureFilters)
	clone.TaskStats = cloneStringAnyMap(task.TaskStats)
	clone.UAKeywords = append([]string(nil), task.UAKeywords...)
	if task.CompletedAt != nil {
		completedAt := *task.CompletedAt
		clone.CompletedAt = &completedAt
	}
	return &clone
}

func cloneTLSFingerprintCaptureSample(sample *TLSFingerprintCaptureSample) *TLSFingerprintCaptureSample {
	if sample == nil {
		return nil
	}
	clone := *sample
	clone.Profile = cloneTLSFingerprintProfile(sample.Profile)
	clone.RawClientHello = append([]byte(nil), sample.RawClientHello...)
	clone.StainlessMetadata = cloneStringAnyMap(sample.StainlessMetadata)
	return &clone
}

func cloneTLSFingerprintCaptureSession(session *TLSFingerprintCaptureSession) *TLSFingerprintCaptureSession {
	if session == nil {
		return nil
	}
	clone := *session
	clone.RawClientHello = append([]byte(nil), session.RawClientHello...)
	clone.ObservedClientHello = cloneStringAnyMap(session.ObservedClientHello)
	clone.ReplayProfile = cloneStringAnyMap(session.ReplayProfile)
	clone.DerivedFingerprint = cloneStringAnyMap(session.DerivedFingerprint)
	if session.ClosedAt != nil {
		closedAt := *session.ClosedAt
		clone.ClosedAt = &closedAt
	}
	return &clone
}

func cloneTLSFingerprintCaptureSessionEvent(event *TLSFingerprintCaptureSessionEvent) *TLSFingerprintCaptureSessionEvent {
	if event == nil {
		return nil
	}
	clone := *event
	if event.SampleID != nil {
		sampleID := *event.SampleID
		clone.SampleID = &sampleID
	}
	clone.StainlessMetadata = cloneStringAnyMap(event.StainlessMetadata)
	clone.HeadersSnapshot = cloneStringAnyMap(event.HeadersSnapshot)
	return &clone
}

func cloneTLSFingerprintCaptureTasks(tasks []*TLSFingerprintCaptureTask) []*TLSFingerprintCaptureTask {
	out := make([]*TLSFingerprintCaptureTask, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, cloneTLSFingerprintCaptureTask(task))
	}
	return out
}

func cloneTLSFingerprintCaptureSamples(samples []*TLSFingerprintCaptureSample) []*TLSFingerprintCaptureSample {
	out := make([]*TLSFingerprintCaptureSample, 0, len(samples))
	for _, sample := range samples {
		out = append(out, cloneTLSFingerprintCaptureSample(sample))
	}
	return out
}

func cloneTLSFingerprintCaptureSessions(sessions []*TLSFingerprintCaptureSession) []*TLSFingerprintCaptureSession {
	out := make([]*TLSFingerprintCaptureSession, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, cloneTLSFingerprintCaptureSession(session))
	}
	return out
}

func cloneTLSFingerprintCaptureSessionEvents(events []*TLSFingerprintCaptureSessionEvent) []*TLSFingerprintCaptureSessionEvent {
	out := make([]*TLSFingerprintCaptureSessionEvent, 0, len(events))
	for _, event := range events {
		out = append(out, cloneTLSFingerprintCaptureSessionEvent(event))
	}
	return out
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
