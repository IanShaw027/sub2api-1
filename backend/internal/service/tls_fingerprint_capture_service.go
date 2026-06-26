package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	utls "github.com/refraction-networking/utls"
)

const (
	TLSFingerprintCaptureStatusRunning   = "running"
	TLSFingerprintCaptureStatusCompleted = "completed"
	TLSFingerprintCaptureStatusStopped   = "stopped"
)

type TLSFingerprintCaptureTask struct {
	ID               int64          `json:"id"`
	Name             string         `json:"name"`
	Status           string         `json:"status"`
	Token            string         `json:"token,omitempty"`
	Targets          map[string]int `json:"targets"`
	Counts           map[string]int `json:"counts"`
	TransportTargets map[string]int `json:"transport_targets,omitempty"`
	TransportCounts  map[string]int `json:"transport_counts,omitempty"`
	UAKeywords       []string       `json:"ua_keywords"`
	CaptureBaseURL   string         `json:"capture_base_url,omitempty"`
	CaptureURL       string         `json:"capture_url,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	CompletedAt      *time.Time     `json:"completed_at,omitempty"`
}

type TLSFingerprintCaptureSample struct {
	ID              int64                        `json:"id"`
	TaskID          int64                        `json:"task_id"`
	Platform        string                       `json:"platform"`
	Transport       string                       `json:"transport,omitempty"`
	SessionID       string                       `json:"session_id,omitempty"`
	UserAgent       string                       `json:"user_agent"`
	Originator      string                       `json:"originator"`
	FingerprintHash string                       `json:"fingerprint_hash"`
	ReplayHash      string                       `json:"replay_hash,omitempty"`
	Profile         *model.TLSFingerprintProfile `json:"profile"`
	RawPayload      string                       `json:"raw_payload"`
	RawClientHello  []byte                       `json:"raw_client_hello,omitempty"`
	CreatedAt       time.Time                    `json:"created_at"`
}

type TLSFingerprintCaptureSession struct {
	ID         int64     `json:"id"`
	TaskID     int64     `json:"task_id"`
	SessionID  string    `json:"session_id"`
	Platform   string    `json:"platform,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	Originator string    `json:"originator,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TLSFingerprintCaptureSessionEvent struct {
	ID         int64     `json:"id"`
	TaskID     int64     `json:"task_id"`
	SessionRef int64     `json:"session_ref,omitempty"`
	SessionID  string    `json:"session_id,omitempty"`
	Platform   string    `json:"platform,omitempty"`
	Transport  string    `json:"transport,omitempty"`
	EventType  string    `json:"event_type"`
	Error      string    `json:"error,omitempty"`
	Replayable bool      `json:"replayable"`
	SampleID   *int64    `json:"sample_id,omitempty"`
	ReplayHash string    `json:"replay_hash,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type TLSFingerprintCaptureStartRequest struct {
	Name             string         `json:"name"`
	Targets          map[string]int `json:"targets"`
	TransportTargets map[string]int `json:"transport_targets"`
	UAKeywords       []string       `json:"ua_keywords"`
}

type TLSFingerprintCaptureNativeSubmitRequest struct {
	Token             string `json:"token"`
	Platform          string `json:"platform"`
	Transport         string `json:"transport,omitempty"`
	SessionID         string `json:"session_id,omitempty"`
	UserAgent         string `json:"user_agent"`
	Originator        string `json:"originator"`
	ClientHello       []byte `json:"client_hello"`
	Replayable        *bool  `json:"replayable,omitempty"`
	SessionEventType  string `json:"session_event_type,omitempty"`
	SessionEventError string `json:"session_event_error,omitempty"`
}

type TLSFingerprintCaptureTaskImportRequest struct {
	TaskID    int64   `json:"task_id"`
	SampleIDs []int64 `json:"sample_ids"`
}

type TLSFingerprintCaptureSubmitResult struct {
	Accepted        bool                               `json:"accepted"`
	Duplicate       bool                               `json:"duplicate"`
	IgnoredReason   string                             `json:"ignored_reason,omitempty"`
	FingerprintHash string                             `json:"fingerprint_hash,omitempty"`
	Task            *TLSFingerprintCaptureTask         `json:"task"`
	Sample          *TLSFingerprintCaptureSample       `json:"sample,omitempty"`
	Session         *TLSFingerprintCaptureSession      `json:"session,omitempty"`
	SessionEvent    *TLSFingerprintCaptureSessionEvent `json:"session_event,omitempty"`
	Counts          map[string]int                     `json:"counts"`
}

type tlsFingerprintCaptureSessionRepository interface {
	CreateSessionIfAbsent(ctx context.Context, session *TLSFingerprintCaptureSession) (*TLSFingerprintCaptureSession, bool, error)
	CreateSessionEvent(ctx context.Context, event *TLSFingerprintCaptureSessionEvent) (*TLSFingerprintCaptureSessionEvent, error)
}

type TLSFingerprintCaptureRepository interface {
	CreateTask(ctx context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error)
	ListTasks(ctx context.Context) ([]*TLSFingerprintCaptureTask, error)
	GetTaskByID(ctx context.Context, id int64) (*TLSFingerprintCaptureTask, error)
	GetRunningTaskByToken(ctx context.Context, token string) (*TLSFingerprintCaptureTask, error)
	UpdateTask(ctx context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error)
	DeleteTask(ctx context.Context, id int64) error
	DeleteSamplesByTask(ctx context.Context, taskID int64) error
	CreateSampleIfAbsent(ctx context.Context, sample *TLSFingerprintCaptureSample) (*TLSFingerprintCaptureSample, bool, error)
	GetSampleByTaskHash(ctx context.Context, taskID int64, fingerprintHash string) (*TLSFingerprintCaptureSample, error)
	ListSamplesByTask(ctx context.Context, taskID int64) ([]*TLSFingerprintCaptureSample, error)
}

type TLSFingerprintCaptureService struct {
	repo                       TLSFingerprintCaptureRepository
	profileSvc                 *TLSFingerprintProfileService
	nativeCapturePublicBaseURL string
}

func NewTLSFingerprintCaptureService(repo TLSFingerprintCaptureRepository, profileSvc *TLSFingerprintProfileService) *TLSFingerprintCaptureService {
	return &TLSFingerprintCaptureService{repo: repo, profileSvc: profileSvc}
}

func (s *TLSFingerprintCaptureService) SetNativeCapturePublicBaseURL(baseURL string) {
	if s == nil {
		return
	}
	s.nativeCapturePublicBaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func ProvideTLSFingerprintCaptureService(repo TLSFingerprintCaptureRepository, profileSvc *TLSFingerprintProfileService, cfg *config.Config) *TLSFingerprintCaptureService {
	svc := NewTLSFingerprintCaptureService(repo, profileSvc)
	if cfg != nil {
		svc.SetNativeCapturePublicBaseURL(cfg.TLSFingerprintCapture.PublicBaseURL)
	}
	return svc
}

func (s *TLSFingerprintCaptureService) StartTask(ctx context.Context, req TLSFingerprintCaptureStartRequest) (*TLSFingerprintCaptureTask, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("tls fingerprint capture repository is not configured")
	}
	targets := normalizeTLSCaptureTargets(req.Targets)
	if len(targets) == 0 {
		return nil, &model.ValidationError{Field: "targets", Message: "at least one platform target is required"}
	}
	token, err := newTLSFingerprintCaptureToken()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "TLS fingerprint capture"
	}
	task := &TLSFingerprintCaptureTask{
		Name:             name,
		Status:           TLSFingerprintCaptureStatusRunning,
		Token:            token,
		Targets:          targets,
		Counts:           make(map[string]int, len(targets)),
		TransportTargets: normalizeTLSCaptureTargets(req.TransportTargets),
		TransportCounts:  make(map[string]int, len(req.TransportTargets)),
		UAKeywords:       normalizeTLSCaptureKeywords(req.UAKeywords),
	}
	for platform := range targets {
		task.Counts[platform] = 0
	}
	for transport := range task.TransportTargets {
		task.TransportCounts[transport] = 0
	}
	created, err := s.repo.CreateTask(ctx, task)
	if err != nil {
		return nil, err
	}
	return s.decorateCaptureTask(created), nil
}

func (s *TLSFingerprintCaptureService) ListTasks(ctx context.Context) ([]*TLSFingerprintCaptureTask, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("tls fingerprint capture repository is not configured")
	}
	tasks, err := s.repo.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		tasks[i] = s.decorateCaptureTask(tasks[i])
	}
	return tasks, nil
}

func (s *TLSFingerprintCaptureService) GetTaskByID(ctx context.Context, id int64) (*TLSFingerprintCaptureTask, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("tls fingerprint capture repository is not configured")
	}
	if id <= 0 {
		return nil, &model.ValidationError{Field: "id", Message: "task ID is required"}
	}
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.decorateCaptureTask(task), nil
}

func (s *TLSFingerprintCaptureService) StopTask(ctx context.Context, id int64) (*TLSFingerprintCaptureTask, error) {
	task, err := s.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, &model.ValidationError{Field: "id", Message: "capture task not found"}
	}
	if task.Status != TLSFingerprintCaptureStatusRunning {
		return task, nil
	}
	now := time.Now().UTC()
	task.Status = TLSFingerprintCaptureStatusStopped
	task.CompletedAt = &now
	updated, err := s.repo.UpdateTask(ctx, task)
	if err != nil {
		return nil, err
	}
	return s.decorateCaptureTask(updated), nil
}

func (s *TLSFingerprintCaptureService) DeleteTask(ctx context.Context, id int64) error {
	task, err := s.GetTaskByID(ctx, id)
	if err != nil {
		return err
	}
	if task == nil {
		return &model.ValidationError{Field: "id", Message: "capture task not found"}
	}
	if task.Status == TLSFingerprintCaptureStatusRunning {
		return &model.ValidationError{Field: "status", Message: "cannot delete a running task; stop it first"}
	}
	if err := s.repo.DeleteSamplesByTask(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteTask(ctx, id)
}

func (s *TLSFingerprintCaptureService) RestartTask(ctx context.Context, id int64) (*TLSFingerprintCaptureTask, error) {
	task, err := s.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, &model.ValidationError{Field: "id", Message: "capture task not found"}
	}
	if task.Status == TLSFingerprintCaptureStatusRunning {
		return nil, &model.ValidationError{Field: "status", Message: "task is already running"}
	}
	token, err := newTLSFingerprintCaptureToken()
	if err != nil {
		return nil, err
	}
	task.Status = TLSFingerprintCaptureStatusRunning
	task.Token = token
	task.CompletedAt = nil
	for platform := range task.Counts {
		task.Counts[platform] = 0
	}
	for transport := range task.TransportCounts {
		task.TransportCounts[transport] = 0
	}
	updated, err := s.repo.UpdateTask(ctx, task)
	if err != nil {
		return nil, err
	}
	return s.decorateCaptureTask(updated), nil
}

func (s *TLSFingerprintCaptureService) ListSamplesByTask(ctx context.Context, taskID int64) ([]*TLSFingerprintCaptureSample, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("tls fingerprint capture repository is not configured")
	}
	if taskID <= 0 {
		return nil, &model.ValidationError{Field: "task_id", Message: "task ID is required"}
	}
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, &model.ValidationError{Field: "task_id", Message: "capture task not found"}
	}
	return s.repo.ListSamplesByTask(ctx, taskID)
}

func (s *TLSFingerprintCaptureService) ImportTaskSamples(ctx context.Context, req TLSFingerprintCaptureTaskImportRequest) (*TLSFingerprintCaptureImportResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("tls fingerprint capture repository is not configured")
	}
	if s.profileSvc == nil {
		return nil, fmt.Errorf("tls fingerprint profile service is not configured")
	}
	if req.TaskID <= 0 {
		return nil, &model.ValidationError{Field: "task_id", Message: "task ID is required"}
	}
	samples, err := s.ListSamplesByTask(ctx, req.TaskID)
	if err != nil {
		return nil, err
	}
	samples = filterTLSCaptureSamples(samples, req.SampleIDs)
	if len(samples) == 0 {
		return nil, &model.ValidationError{Field: "sample_ids", Message: "at least one captured sample is required"}
	}

	payloadsByPlatform := make(map[string][]string)
	platformOrder := make([]string, 0)
	seenPlatform := make(map[string]struct{})
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		payload, err := tlsCaptureSamplePayload(sample)
		if err != nil {
			return nil, err
		}
		platform := strings.TrimSpace(sample.Platform)
		if _, ok := seenPlatform[platform]; !ok {
			seenPlatform[platform] = struct{}{}
			platformOrder = append(platformOrder, platform)
		}
		payloadsByPlatform[platform] = append(payloadsByPlatform[platform], payload)
	}

	combined := &TLSFingerprintCaptureImportResult{}
	for _, platform := range platformOrder {
		result, err := s.profileSvc.ImportTLSFingerprintCaptures(ctx, TLSFingerprintCaptureImportRequest{
			Platform: platform,
			Profiles: payloadsByPlatform[platform],
		})
		if err != nil {
			return nil, err
		}
		combined.Imported += result.Imported
		combined.Duplicates += result.Duplicates
		combined.Profiles = append(combined.Profiles, result.Profiles...)
	}
	return combined, nil
}

func (s *TLSFingerprintCaptureService) SubmitNativeCapture(ctx context.Context, req TLSFingerprintCaptureNativeSubmitRequest) (*TLSFingerprintCaptureSubmitResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("tls fingerprint capture repository is not configured")
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return nil, &model.ValidationError{Field: "token", Message: "token is required"}
	}
	task, err := s.repo.GetRunningTaskByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, &model.ValidationError{Field: "token", Message: "running capture task not found"}
	}

	platform := strings.TrimSpace(req.Platform)
	if platform == "" && len(task.Targets) == 1 {
		for targetPlatform := range task.Targets {
			platform = targetPlatform
		}
	}
	if _, ok := task.Targets[platform]; !ok {
		return ignoredTLSCaptureResult(task, "platform_not_targeted"), nil
	}
	transport := normalizeTLSCaptureTransport(req.Transport)
	if len(task.TransportTargets) > 0 {
		if transport == "" {
			return ignoredTLSCaptureResult(task, "transport_required"), nil
		}
		if _, ok := task.TransportTargets[transport]; !ok {
			return ignoredTLSCaptureResult(task, "transport_not_targeted"), nil
		}
	}
	if !tlsCaptureUserAgentMatches(task.UAKeywords, req.UserAgent) {
		return ignoredTLSCaptureResult(task, "user_agent_not_matched"), nil
	}
	if len(req.ClientHello) == 0 {
		return ignoredTLSCaptureResult(task, "client_hello_required"), nil
	}

	session, sessionErr := s.ensureCaptureSession(ctx, task, req, platform)
	if sessionErr != nil {
		return nil, sessionErr
	}

	counts, transportCounts, err := s.countSamples(ctx, task)
	if err != nil {
		return nil, err
	}

	profile, err := parseTLSFingerprintClientHello(req.ClientHello)
	if err != nil {
		return nil, &model.ValidationError{Field: "client_hello", Message: err.Error()}
	}
	profile.Platform = platform
	hash, err := TLSFingerprintProfileReplayHash(profile)
	if err != nil {
		return nil, err
	}

	if replayable := tlsCaptureReplayable(req.Replayable); !replayable {
		event, eventErr := s.createCaptureSessionEvent(ctx, session, task, req, platform, transport, hash, nil, replayable)
		if eventErr != nil {
			return nil, eventErr
		}
		task, err = s.updateTaskProgress(ctx, task, counts, transportCounts)
		if err != nil {
			return nil, err
		}
		return ignoredTLSCaptureResultWithSession(task, "capture_not_replayable", session, event), nil
	}

	existing, err := s.findSampleByReplayKey(ctx, task, hash, transport)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		event, eventErr := s.createCaptureSessionEvent(ctx, session, task, req, platform, transport, hash, idPointer(existing.ID), true)
		if eventErr != nil {
			return nil, eventErr
		}
		task, err = s.updateTaskProgress(ctx, task, counts, transportCounts)
		if err != nil {
			return nil, err
		}
		return &TLSFingerprintCaptureSubmitResult{
			Accepted:        true,
			Duplicate:       true,
			FingerprintHash: existing.FingerprintHash,
			Task:            task,
			Sample:          existing,
			Session:         session,
			SessionEvent:    event,
			Counts:          copyStringIntMap(counts),
		}, nil
	}

	if existing, err = s.findSampleBySessionTransport(ctx, task, strings.TrimSpace(req.SessionID), transport); err != nil {
		return nil, err
	}
	if existing != nil {
		event, eventErr := s.createCaptureSessionEvent(ctx, session, task, req, platform, transport, existing.ReplayHash, idPointer(existing.ID), true)
		if eventErr != nil {
			return nil, eventErr
		}
		task, err = s.updateTaskProgress(ctx, task, counts, transportCounts)
		if err != nil {
			return nil, err
		}
		return &TLSFingerprintCaptureSubmitResult{
			Accepted:        true,
			Duplicate:       true,
			FingerprintHash: existing.FingerprintHash,
			Task:            task,
			Sample:          existing,
			Session:         session,
			SessionEvent:    event,
			Counts:          copyStringIntMap(counts),
		}, nil
	}

	if reached, reason := tlsCaptureSubmissionQuotaReached(task, counts, transportCounts, platform, transport); reached {
		task, err = s.updateTaskProgress(ctx, task, counts, transportCounts)
		if err != nil {
			return nil, err
		}
		return ignoredTLSCaptureResult(task, reason), nil
	}

	fingerprintHash := hash
	if transport != "" {
		fingerprintHash = hash + "::" + transport
	}
	rawClientHello := append([]byte(nil), req.ClientHello...)
	sample := &TLSFingerprintCaptureSample{
		TaskID:          task.ID,
		Platform:        platform,
		Transport:       transport,
		SessionID:       strings.TrimSpace(req.SessionID),
		UserAgent:       strings.TrimSpace(req.UserAgent),
		Originator:      strings.TrimSpace(req.Originator),
		FingerprintHash: fingerprintHash,
		ReplayHash:      hash,
		Profile:         profile,
		RawClientHello:  rawClientHello,
	}
	created, inserted, err := s.repo.CreateSampleIfAbsent(ctx, sample)
	if err != nil {
		return nil, err
	}

	counts, transportCounts, err = s.countSamples(ctx, task)
	if err != nil {
		return nil, err
	}
	event, eventErr := s.createCaptureSessionEvent(ctx, session, task, req, platform, transport, hash, idPointer(created.ID), true)
	if eventErr != nil {
		return nil, eventErr
	}
	task, err = s.updateTaskProgress(ctx, task, counts, transportCounts)
	if err != nil {
		return nil, err
	}

	return &TLSFingerprintCaptureSubmitResult{
		Accepted:        true,
		Duplicate:       !inserted,
		FingerprintHash: fingerprintHash,
		Task:            task,
		Sample:          created,
		Session:         session,
		SessionEvent:    event,
		Counts:          copyStringIntMap(counts),
	}, nil
}

func (s *TLSFingerprintCaptureService) updateTaskProgress(ctx context.Context, task *TLSFingerprintCaptureTask, counts, transportCounts map[string]int) (*TLSFingerprintCaptureTask, error) {
	latest, err := s.repo.GetTaskByID(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, &model.ValidationError{Field: "task_id", Message: "capture task not found"}
	}
	latest.Counts = counts
	latest.TransportCounts = transportCounts
	if latest.Status == TLSFingerprintCaptureStatusRunning && tlsCaptureTargetsReached(latest.Targets, counts) && tlsCaptureTargetsReached(latest.TransportTargets, transportCounts) {
		now := time.Now().UTC()
		latest.Status = TLSFingerprintCaptureStatusCompleted
		latest.CompletedAt = &now
	}
	updated, err := s.repo.UpdateTask(ctx, latest)
	if err != nil {
		return nil, err
	}
	return s.decorateCaptureTask(updated), nil
}

func (s *TLSFingerprintCaptureService) decorateCaptureTask(task *TLSFingerprintCaptureTask) *TLSFingerprintCaptureTask {
	if task == nil {
		return nil
	}
	baseURL := ""
	if s != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(s.nativeCapturePublicBaseURL), "/")
	}
	if baseURL == "" {
		return task
	}
	task.CaptureBaseURL = baseURL
	if strings.HasSuffix(baseURL, "/capture") {
		task.CaptureURL = baseURL
	} else {
		task.CaptureURL = baseURL + "/capture"
	}
	return task
}

func (s *TLSFingerprintCaptureService) countSamples(ctx context.Context, task *TLSFingerprintCaptureTask) (map[string]int, map[string]int, error) {
	counts := make(map[string]int, len(task.Targets))
	for platform := range task.Targets {
		counts[platform] = 0
	}
	transportCounts := make(map[string]int, len(task.TransportTargets))
	for transport := range task.TransportTargets {
		transportCounts[transport] = 0
	}
	samples, err := s.repo.ListSamplesByTask(ctx, task.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		if _, ok := task.Targets[sample.Platform]; ok {
			counts[sample.Platform]++
		}
		if _, ok := task.TransportTargets[sample.Transport]; ok {
			transportCounts[sample.Transport]++
		}
	}
	return counts, transportCounts, nil
}

func ignoredTLSCaptureResult(task *TLSFingerprintCaptureTask, reason string) *TLSFingerprintCaptureSubmitResult {
	return &TLSFingerprintCaptureSubmitResult{
		Accepted:      false,
		Duplicate:     false,
		IgnoredReason: reason,
		Task:          task,
		Counts:        copyStringIntMap(task.Counts),
	}
}

func ignoredTLSCaptureResultWithSession(task *TLSFingerprintCaptureTask, reason string, session *TLSFingerprintCaptureSession, event *TLSFingerprintCaptureSessionEvent) *TLSFingerprintCaptureSubmitResult {
	result := ignoredTLSCaptureResult(task, reason)
	result.Session = session
	result.SessionEvent = event
	return result
}

func tlsCaptureUserAgentMatches(keywords []string, userAgent string) bool {
	if len(keywords) == 0 {
		return true
	}
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(ua, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func tlsCaptureTargetsReached(targets, counts map[string]int) bool {
	for platform, target := range targets {
		if target > 0 && counts[platform] < target {
			return false
		}
	}
	return true
}

func tlsCapturePlatformTargetReached(targets, counts map[string]int, platform string) bool {
	target := targets[platform]
	return target > 0 && counts[platform] >= target
}

func tlsCaptureTransportTargetReached(targets, counts map[string]int, transport string) bool {
	target := targets[transport]
	return target > 0 && counts[transport] >= target
}

// tlsCaptureSubmissionQuotaReached reports whether a new sample for the given
// platform/transport should be rejected because its target quota is already
// satisfied. It returns a stable reason so callers can distinguish a
// per-platform stop from a per-transport stop.
func tlsCaptureSubmissionQuotaReached(task *TLSFingerprintCaptureTask, counts, transportCounts map[string]int, platform, transport string) (bool, string) {
	platformTargeted := task.Targets[platform] > 0
	platformReached := platformTargeted && counts[platform] >= task.Targets[platform]

	transportTargeted := len(task.TransportTargets) > 0 && task.TransportTargets[transport] > 0
	transportReached := transportTargeted && transportCounts[transport] >= task.TransportTargets[transport]

	// A submission is only ignored when every dimension it counts against is
	// already satisfied. If the platform is targeted but not yet reached, the
	// sample is still needed regardless of transport state, and vice versa.
	if platformTargeted && !platformReached {
		return false, ""
	}
	if transportTargeted && !transportReached {
		return false, ""
	}
	if platformReached {
		return true, "platform_target_reached"
	}
	if transportReached {
		return true, "transport_target_reached"
	}
	return false, ""
}

func normalizeTLSCaptureTargets(targets map[string]int) map[string]int {
	out := make(map[string]int, len(targets))
	for platform, target := range targets {
		platform = strings.TrimSpace(platform)
		if platform == "" || target <= 0 {
			continue
		}
		out[platform] = target
	}
	return out
}

func normalizeTLSCaptureTransport(transport string) string {
	return strings.ToLower(strings.TrimSpace(transport))
}

func normalizeTLSCaptureKeywords(keywords []string) []string {
	out := make([]string, 0, len(keywords))
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		key := strings.ToLower(keyword)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, keyword)
	}
	return out
}

func copyStringIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func tlsCaptureReplayable(replayable *bool) bool {
	if replayable == nil {
		return true
	}
	return *replayable
}

func idPointer(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	v := id
	return &v
}

func (s *TLSFingerprintCaptureService) ensureCaptureSession(ctx context.Context, task *TLSFingerprintCaptureTask, req TLSFingerprintCaptureNativeSubmitRequest, platform string) (*TLSFingerprintCaptureSession, error) {
	repo, ok := s.repo.(tlsFingerprintCaptureSessionRepository)
	if !ok {
		return nil, nil
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return nil, nil
	}
	session := &TLSFingerprintCaptureSession{
		TaskID:     task.ID,
		SessionID:  sessionID,
		Platform:   platform,
		UserAgent:  strings.TrimSpace(req.UserAgent),
		Originator: strings.TrimSpace(req.Originator),
	}
	created, _, err := repo.CreateSessionIfAbsent(ctx, session)
	return created, err
}

func (s *TLSFingerprintCaptureService) createCaptureSessionEvent(
	ctx context.Context,
	session *TLSFingerprintCaptureSession,
	task *TLSFingerprintCaptureTask,
	req TLSFingerprintCaptureNativeSubmitRequest,
	platform, transport, replayHash string,
	sampleID *int64,
	replayable bool,
) (*TLSFingerprintCaptureSessionEvent, error) {
	repo, ok := s.repo.(tlsFingerprintCaptureSessionRepository)
	if !ok {
		return nil, nil
	}
	eventType := strings.TrimSpace(req.SessionEventType)
	if eventType == "" {
		if sampleID != nil {
			eventType = "canonical_sample_recorded"
		} else {
			eventType = "session_observed"
		}
	}
	event := &TLSFingerprintCaptureSessionEvent{
		TaskID:     task.ID,
		SessionID:  strings.TrimSpace(req.SessionID),
		Platform:   platform,
		Transport:  transport,
		EventType:  eventType,
		Error:      strings.TrimSpace(req.SessionEventError),
		Replayable: replayable,
		SampleID:   sampleID,
		ReplayHash: replayHash,
	}
	if session != nil {
		event.SessionRef = session.ID
	}
	return repo.CreateSessionEvent(ctx, event)
}

func (s *TLSFingerprintCaptureService) findSampleByReplayKey(ctx context.Context, task *TLSFingerprintCaptureTask, replayHash, transport string) (*TLSFingerprintCaptureSample, error) {
	key := replayHash
	if transport != "" {
		key = replayHash + "::" + transport
	}
	sample, err := s.repo.GetSampleByTaskHash(ctx, task.ID, key)
	if err != nil || sample != nil {
		return sample, err
	}
	samples, err := s.repo.ListSamplesByTask(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	for _, existing := range samples {
		if existing == nil {
			continue
		}
		if strings.TrimSpace(existing.ReplayHash) == replayHash && normalizeTLSCaptureTransport(existing.Transport) == transport {
			return existing, nil
		}
	}
	return nil, nil
}

func (s *TLSFingerprintCaptureService) findSampleBySessionTransport(ctx context.Context, task *TLSFingerprintCaptureTask, sessionID, transport string) (*TLSFingerprintCaptureSample, error) {
	if strings.TrimSpace(sessionID) == "" || transport == "" {
		return nil, nil
	}
	samples, err := s.repo.ListSamplesByTask(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		if strings.TrimSpace(sample.SessionID) == sessionID && normalizeTLSCaptureTransport(sample.Transport) == transport {
			return sample, nil
		}
	}
	return nil, nil
}

func filterTLSCaptureSamples(samples []*TLSFingerprintCaptureSample, sampleIDs []int64) []*TLSFingerprintCaptureSample {
	if len(sampleIDs) == 0 {
		return samples
	}
	wanted := make(map[int64]struct{}, len(sampleIDs))
	for _, id := range sampleIDs {
		if id > 0 {
			wanted[id] = struct{}{}
		}
	}
	out := make([]*TLSFingerprintCaptureSample, 0, len(samples))
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		if _, ok := wanted[sample.ID]; ok {
			out = append(out, sample)
		}
	}
	return out
}

func tlsCaptureSamplePayload(sample *TLSFingerprintCaptureSample) (string, error) {
	if sample == nil {
		return "", fmt.Errorf("capture sample is required")
	}
	if raw := strings.TrimSpace(sample.RawPayload); raw != "" {
		return raw, nil
	}
	if sample.Profile == nil {
		return "", &model.ValidationError{Field: "profile", Message: "captured sample profile is required"}
	}
	encoded, err := json.Marshal(sample.Profile)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func parseTLSFingerprintClientHello(raw []byte) (*model.TLSFingerprintProfile, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("client hello is required")
	}
	spec, err := (&utls.Fingerprinter{AllowBluntMimicry: true}).RawClientHello(raw)
	if err != nil {
		return nil, fmt.Errorf("parse client hello: %w", err)
	}
	profile := tlsFingerprintProfileFromSpec(spec)
	if err := validateCompleteTLSFingerprintProfile(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func tlsFingerprintProfileFromSpec(spec *utls.ClientHelloSpec) *model.TLSFingerprintProfile {
	if spec == nil {
		return &model.TLSFingerprintProfile{Name: "native ClientHello capture"}
	}
	profile := &model.TLSFingerprintProfile{
		Name:         "native ClientHello capture",
		CipherSuites: append([]uint16(nil), spec.CipherSuites...),
	}
	for _, cipherSuite := range profile.CipherSuites {
		if tlsCaptureIsGREASEValue(cipherSuite) {
			profile.EnableGREASE = true
			break
		}
	}
	for _, ext := range spec.Extensions {
		extID, ok := tlsCaptureExtensionID(ext)
		if ok {
			profile.Extensions = append(profile.Extensions, extID)
			if tlsCaptureIsGREASEValue(extID) {
				profile.EnableGREASE = true
			}
		}
		switch typed := ext.(type) {
		case *utls.SupportedCurvesExtension:
			profile.Curves = tlsCaptureCurveIDsToUint16(typed.Curves)
		case *utls.SupportedPointsExtension:
			profile.PointFormats = tlsCaptureUint8sToUint16(typed.SupportedPoints)
		case *utls.SignatureAlgorithmsExtension:
			profile.SignatureAlgorithms = tlsCaptureSignatureSchemesToUint16(typed.SupportedSignatureAlgorithms)
		case *utls.ALPNExtension:
			profile.ALPNProtocols = append([]string(nil), typed.AlpnProtocols...)
		case *utls.UtlsCompressCertExtension:
			profile.CompressCertAlgos = tlsCaptureCertCompressionAlgosToUint16(typed.Algorithms)
		case *utls.FakeDelegatedCredentialsExtension:
			profile.DelegatedCredentialsAlgorithms = tlsCaptureSignatureSchemesToUint16(typed.SupportedSignatureAlgorithms)
		case *utls.SupportedVersionsExtension:
			profile.SupportedVersions = append([]uint16(nil), typed.Versions...)
		case *utls.KeyShareExtension:
			profile.KeyShareGroups = tlsCaptureKeyShareGroupsToUint16(typed.KeyShares)
		case *utls.PSKKeyExchangeModesExtension:
			profile.PSKModes = tlsCaptureUint8sToUint16(typed.Modes)
		case *utls.SignatureAlgorithmsCertExtension:
			if len(profile.SignatureAlgorithms) == 0 {
				profile.SignatureAlgorithms = tlsCaptureSignatureSchemesToUint16(typed.SupportedSignatureAlgorithms)
			}
		case *utls.ApplicationSettingsExtension:
			profile.ApplicationSettingsProtocols = append([]string(nil), typed.SupportedProtocols...)
		case *utls.ApplicationSettingsExtensionNew:
			profile.ApplicationSettingsProtocols = append([]string(nil), typed.SupportedProtocols...)
		}
	}
	if len(profile.SupportedVersions) == 0 && spec.TLSVersMax != 0 {
		profile.SupportedVersions = []uint16{spec.TLSVersMax}
		if spec.TLSVersMin != 0 && spec.TLSVersMin != spec.TLSVersMax {
			profile.SupportedVersions = append(profile.SupportedVersions, spec.TLSVersMin)
		}
	}
	return profile
}

func tlsCaptureExtensionID(ext utls.TLSExtension) (uint16, bool) {
	switch typed := ext.(type) {
	case *utls.SNIExtension:
		return 0, true
	case *utls.StatusRequestExtension:
		return 5, true
	case *utls.SupportedCurvesExtension:
		return 10, true
	case *utls.SupportedPointsExtension:
		return 11, true
	case *utls.SignatureAlgorithmsExtension:
		return 13, true
	case *utls.ALPNExtension:
		return 16, true
	case *utls.StatusRequestV2Extension:
		return 17, true
	case *utls.SCTExtension:
		return 18, true
	case *utls.UtlsPaddingExtension:
		return 21, true
	case *utls.ExtendedMasterSecretExtension:
		return 23, true
	case *utls.FakeTokenBindingExtension:
		return 24, true
	case *utls.UtlsCompressCertExtension:
		return 27, true
	case *utls.FakeRecordSizeLimitExtension:
		return 28, true
	case *utls.FakeDelegatedCredentialsExtension:
		return 34, true
	case *utls.SessionTicketExtension:
		return 35, true
	case utls.PreSharedKeyExtension:
		return 41, true
	case *utls.SupportedVersionsExtension:
		return 43, true
	case *utls.CookieExtension:
		return 44, true
	case *utls.PSKKeyExchangeModesExtension:
		return 45, true
	case *utls.SignatureAlgorithmsCertExtension:
		return 50, true
	case *utls.KeyShareExtension:
		return 51, true
	case *utls.QUICTransportParametersExtension:
		return 57, true
	case *utls.GenericExtension:
		return typed.Id, true
	case *utls.UtlsGREASEExtension:
		if typed.Value != 0 {
			return typed.Value, true
		}
		return utls.GREASE_PLACEHOLDER, true
	case *utls.GREASEEncryptedClientHelloExtension, *utls.UnimplementedECHExtension:
		return 0xfe0d, true
	case *utls.ApplicationSettingsExtension:
		return 17513, true
	case *utls.ApplicationSettingsExtensionNew:
		return 17613, true
	case *utls.NPNExtension:
		return 13172, true
	case *utls.FakeChannelIDExtension:
		if typed.OldExtensionID {
			return 30031, true
		}
		return 30032, true
	case *utls.RenegotiationInfoExtension:
		return 65281, true
	default:
		return 0, false
	}
}

func tlsCaptureCurveIDsToUint16(in []utls.CurveID) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, value := range in {
		out = append(out, uint16(value))
	}
	return out
}

func tlsCaptureUint8sToUint16(in []uint8) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, value := range in {
		out = append(out, uint16(value))
	}
	return out
}

func tlsCaptureSignatureSchemesToUint16(in []utls.SignatureScheme) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, value := range in {
		out = append(out, uint16(value))
	}
	return out
}

func tlsCaptureCertCompressionAlgosToUint16(in []utls.CertCompressionAlgo) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, value := range in {
		out = append(out, uint16(value))
	}
	return out
}

func tlsCaptureKeyShareGroupsToUint16(in []utls.KeyShare) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, value := range in {
		out = append(out, uint16(value.Group))
	}
	return out
}

func tlsCaptureIsGREASEValue(v uint16) bool {
	return v&0x0f0f == 0x0a0a && v>>8 == v&0xff
}

func newTLSFingerprintCaptureToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
