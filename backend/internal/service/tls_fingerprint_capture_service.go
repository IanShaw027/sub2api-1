package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	tlsfpParser "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/parser"
	tlsfpReplay "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/replay"
	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
)

const (
	TLSFingerprintCaptureStatusRunning   = "running"
	TLSFingerprintCaptureStatusCompleted = "completed"
	TLSFingerprintCaptureStatusStopped   = "stopped"
)

type TLSFingerprintCaptureTask struct {
	ID                  int64          `json:"id"`
	Name                string         `json:"name"`
	Status              string         `json:"status"`
	Token               string         `json:"token,omitempty"`
	Targets             map[string]int `json:"targets"`
	Counts              map[string]int `json:"counts"`
	TransportTargets    map[string]int `json:"transport_targets,omitempty"`
	TransportCounts     map[string]int `json:"transport_counts,omitempty"`
	CaptureFilters      map[string]any `json:"capture_filters,omitempty"`
	SampleSchemaVersion int            `json:"sample_schema_version,omitempty"`
	TaskStats           map[string]any `json:"task_stats,omitempty"`
	UAKeywords          []string       `json:"ua_keywords"`
	CaptureBaseURL      string         `json:"capture_base_url,omitempty"`
	CaptureURL          string         `json:"capture_url,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	CompletedAt         *time.Time     `json:"completed_at,omitempty"`
}

type TLSFingerprintCaptureSample struct {
	ID                int64                        `json:"id"`
	TaskID            int64                        `json:"task_id"`
	Platform          string                       `json:"platform"`
	Transport         string                       `json:"transport,omitempty"`
	SessionID         string                       `json:"session_id,omitempty"`
	UserAgent         string                       `json:"user_agent"`
	Originator        string                       `json:"originator"`
	FingerprintHash   string                       `json:"fingerprint_hash"`
	ReplayHash        string                       `json:"replay_hash,omitempty"`
	JA3Raw            string                       `json:"ja3_raw,omitempty"`
	JA3Hash           string                       `json:"ja3_hash,omitempty"`
	JA4               string                       `json:"ja4,omitempty"`
	RequestPath       string                       `json:"request_path,omitempty"`
	HTTPMethod        string                       `json:"http_method,omitempty"`
	IsWebsocket       bool                         `json:"is_websocket,omitempty"`
	WebsocketProtocol string                       `json:"websocket_protocol,omitempty"`
	ClientType        string                       `json:"client_type,omitempty"`
	Model             string                       `json:"model,omitempty"`
	RequestKind       string                       `json:"request_kind,omitempty"`
	Streaming         bool                         `json:"streaming,omitempty"`
	ResponseMode      string                       `json:"response_mode,omitempty"`
	HTTP2Fingerprint  string                       `json:"http2_fingerprint,omitempty"`
	StainlessMetadata map[string]any               `json:"stainless_metadata,omitempty"`
	Profile           *model.TLSFingerprintProfile `json:"replay_profile"`
	RawPayload        string                       `json:"raw_payload"`
	RawClientHello    []byte                       `json:"raw_client_hello,omitempty"`
	CapturedAt        time.Time                    `json:"captured_at,omitempty"`
	CreatedAt         time.Time                    `json:"created_at"`
}

type TLSFingerprintCaptureSession struct {
	ID                  int64          `json:"id"`
	TaskID              int64          `json:"task_id"`
	SessionID           string         `json:"session_id"`
	ClientIP            string         `json:"client_ip,omitempty"`
	Platform            string         `json:"platform,omitempty"`
	UserAgent           string         `json:"user_agent,omitempty"`
	Originator          string         `json:"originator,omitempty"`
	ALPNNegotiated      string         `json:"alpn_negotiated,omitempty"`
	RawClientHello      []byte         `json:"raw_client_hello,omitempty"`
	ObservedClientHello map[string]any `json:"observed_client_hello,omitempty"`
	ReplayProfile       map[string]any `json:"replay_profile,omitempty"`
	DerivedFingerprint  map[string]any `json:"derived_fingerprint,omitempty"`
	SessionStatus       string         `json:"session_status,omitempty"`
	ErrorSummary        string         `json:"error_summary,omitempty"`
	OpenedAt            time.Time      `json:"opened_at,omitempty"`
	ClosedAt            *time.Time     `json:"closed_at,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type TLSFingerprintCaptureSessionEvent struct {
	ID                int64          `json:"id"`
	TaskID            int64          `json:"task_id"`
	SessionRef        int64          `json:"session_ref,omitempty"`
	SessionID         string         `json:"session_id,omitempty"`
	EventID           string         `json:"event_id,omitempty"`
	Platform          string         `json:"platform,omitempty"`
	Transport         string         `json:"transport,omitempty"`
	EventType         string         `json:"event_type"`
	RequestSequence   int            `json:"request_sequence,omitempty"`
	StreamID          string         `json:"stream_id,omitempty"`
	RequestPath       string         `json:"request_path,omitempty"`
	HTTPMethod        string         `json:"http_method,omitempty"`
	IsWebsocket       bool           `json:"is_websocket,omitempty"`
	WebsocketProtocol string         `json:"websocket_protocol,omitempty"`
	ClientType        string         `json:"client_type,omitempty"`
	Model             string         `json:"model,omitempty"`
	RequestKind       string         `json:"request_kind,omitempty"`
	Streaming         bool           `json:"streaming,omitempty"`
	ResponseMode      string         `json:"response_mode,omitempty"`
	UserAgent         string         `json:"user_agent,omitempty"`
	Originator        string         `json:"originator,omitempty"`
	StainlessMetadata map[string]any `json:"stainless_metadata,omitempty"`
	HeadersSnapshot   map[string]any `json:"headers_snapshot,omitempty"`
	BodySummary       string         `json:"body_summary,omitempty"`
	RawPayload        string         `json:"raw_payload,omitempty"`
	EventStatus       string         `json:"event_status,omitempty"`
	Error             string         `json:"error,omitempty"`
	Replayable        bool           `json:"replayable"`
	SampleID          *int64         `json:"sample_id,omitempty"`
	ReplayHash        string         `json:"replay_hash,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

type TLSFingerprintCaptureStartRequest struct {
	Name             string         `json:"name"`
	Targets          map[string]int `json:"targets"`
	TransportTargets map[string]int `json:"transport_targets"`
	CaptureFilters   map[string]any `json:"capture_filters"`
	UAKeywords       []string       `json:"ua_keywords"`
}

type TLSFingerprintCaptureNativeSubmitRequest struct {
	Token             string         `json:"token"`
	Platform          string         `json:"platform"`
	Transport         string         `json:"transport,omitempty"`
	SessionID         string         `json:"session_id,omitempty"`
	ClientIP          string         `json:"client_ip,omitempty"`
	ALPNNegotiated    string         `json:"alpn_negotiated,omitempty"`
	UserAgent         string         `json:"user_agent"`
	Originator        string         `json:"originator"`
	RequestPath       string         `json:"request_path,omitempty"`
	HTTPMethod        string         `json:"http_method,omitempty"`
	IsWebsocket       bool           `json:"is_websocket,omitempty"`
	WebsocketProtocol string         `json:"websocket_protocol,omitempty"`
	ClientType        string         `json:"client_type,omitempty"`
	Model             string         `json:"model,omitempty"`
	RequestKind       string         `json:"request_kind,omitempty"`
	Streaming         bool           `json:"streaming,omitempty"`
	ResponseMode      string         `json:"response_mode,omitempty"`
	HTTP2Fingerprint  string         `json:"http2_fingerprint,omitempty"`
	StainlessMetadata map[string]any `json:"stainless_metadata,omitempty"`
	HeadersSnapshot   map[string]any `json:"headers_snapshot,omitempty"`
	BodySummary       string         `json:"body_summary,omitempty"`
	RawPayload        string         `json:"raw_payload,omitempty"`
	EventID           string         `json:"event_id,omitempty"`
	RequestSequence   int            `json:"request_sequence,omitempty"`
	StreamID          string         `json:"stream_id,omitempty"`
	ClientHello       []byte         `json:"client_hello"`
	Replayable        *bool          `json:"replayable,omitempty"`
	SessionEventType  string         `json:"session_event_type,omitempty"`
	SessionEventError string         `json:"session_event_error,omitempty"`
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

type tlsFingerprintParsedCapture struct {
	Observed      *tlsfpParser.ObservedClientHello
	ReplayProfile *tlsfpReplay.ReplayProfile
	Derived       *tlsfpParser.DerivedFingerprint
	Profile       *model.TLSFingerprintProfile
}

type TLSFingerprintCaptureRepository interface {
	CreateTask(ctx context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error)
	ListTasks(ctx context.Context) ([]*TLSFingerprintCaptureTask, error)
	GetTaskByID(ctx context.Context, id int64) (*TLSFingerprintCaptureTask, error)
	GetRunningTaskByToken(ctx context.Context, token string) (*TLSFingerprintCaptureTask, error)
	WithTaskSubmissionLock(ctx context.Context, taskID int64, fn func(context.Context) error) error
	UpdateTask(ctx context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error)
	DeleteTask(ctx context.Context, id int64) error
	DeleteTaskCaptureData(ctx context.Context, taskID int64) error
	CreateSampleIfAbsent(ctx context.Context, sample *TLSFingerprintCaptureSample) (*TLSFingerprintCaptureSample, bool, error)
	GetSampleByTaskHash(ctx context.Context, taskID int64, fingerprintHash string) (*TLSFingerprintCaptureSample, error)
	ListSamplesByTask(ctx context.Context, taskID int64) ([]*TLSFingerprintCaptureSample, error)
	CreateSessionIfAbsent(ctx context.Context, session *TLSFingerprintCaptureSession) (*TLSFingerprintCaptureSession, bool, error)
	CreateSessionEvent(ctx context.Context, event *TLSFingerprintCaptureSessionEvent) (*TLSFingerprintCaptureSessionEvent, error)
}

type TLSFingerprintCaptureService struct {
	repo                       TLSFingerprintCaptureRepository
	profileSvc                 *TLSFingerprintProfileService
	nativeCapturePublicBaseURL string
	// capture 配额判断依赖 count -> quota check -> insert -> recount 这一整段顺序；仓库层目前仅保证
	// sample 去重插入原子，不保证“未超配再插入”这一复合条件原子。仓库层已按 task 行加锁，
	// 这里仅在 service 内补同 task 的单进程序列化，避免不同 task 之间被全局锁误串行。
	submitMu     sync.Mutex
	taskSubmitMu map[int64]*tlsFingerprintCaptureTaskMutex
}

type tlsFingerprintCaptureTaskMutex struct {
	mu   sync.Mutex
	refs int
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
		Name:                name,
		Status:              TLSFingerprintCaptureStatusRunning,
		Token:               token,
		Targets:             targets,
		Counts:              make(map[string]int, len(targets)),
		TransportTargets:    normalizeTLSCaptureTargets(req.TransportTargets),
		TransportCounts:     make(map[string]int, len(req.TransportTargets)),
		CaptureFilters:      copyStringAnyMap(req.CaptureFilters),
		SampleSchemaVersion: 2,
		TaskStats:           map[string]any{},
		UAKeywords:          normalizeTLSCaptureKeywords(req.UAKeywords),
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
		// List responses must not expose capture tokens by default (ops safety).
		// Operators fetch a single task (GetTaskByID / Start / Restart) when they need the token.
		tasks[i] = s.decorateCaptureTask(redactTLSCaptureTaskToken(tasks[i]))
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
	var updated *TLSFingerprintCaptureTask
	unlock := s.lockTaskSubmission(id)
	defer unlock()
	if err := s.repo.WithTaskSubmissionLock(ctx, id, func(lockCtx context.Context) error {
		lockedTask, err := s.repo.GetTaskByID(lockCtx, id)
		if err != nil {
			return err
		}
		if lockedTask == nil {
			return &model.ValidationError{Field: "id", Message: "capture task not found"}
		}
		if lockedTask.Status != TLSFingerprintCaptureStatusRunning {
			updated = lockedTask
			return nil
		}
		now := time.Now().UTC()
		lockedTask.Status = TLSFingerprintCaptureStatusStopped
		lockedTask.CompletedAt = &now
		updated, err = s.repo.UpdateTask(lockCtx, lockedTask)
		return err
	}); err != nil {
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
	unlock := s.lockTaskSubmission(id)
	defer unlock()
	return s.repo.WithTaskSubmissionLock(ctx, id, func(lockCtx context.Context) error {
		// Re-read under the row lock so a concurrent stop/restart cannot invalidate
		// the status checked before entering the transaction.
		lockedTask, err := s.repo.GetTaskByID(lockCtx, id)
		if err != nil {
			return err
		}
		if lockedTask == nil {
			return &model.ValidationError{Field: "id", Message: "capture task not found"}
		}
		if lockedTask.Status == TLSFingerprintCaptureStatusRunning {
			return &model.ValidationError{Field: "status", Message: "cannot delete a running task; stop it first"}
		}
		if err := s.repo.DeleteTaskCaptureData(lockCtx, id); err != nil {
			return err
		}
		return s.repo.DeleteTask(lockCtx, id)
	})
}

func (s *TLSFingerprintCaptureService) RestartTask(ctx context.Context, id int64) (*TLSFingerprintCaptureTask, error) {
	task, err := s.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, &model.ValidationError{Field: "id", Message: "capture task not found"}
	}
	token, err := newTLSFingerprintCaptureToken()
	if err != nil {
		return nil, err
	}
	var updated *TLSFingerprintCaptureTask
	unlock := s.lockTaskSubmission(id)
	defer unlock()
	if err := s.repo.WithTaskSubmissionLock(ctx, id, func(lockCtx context.Context) error {
		// The task can change while waiting for the row lock. Use only the locked
		// copy when deciding whether and how to start the next generation.
		lockedTask, err := s.repo.GetTaskByID(lockCtx, id)
		if err != nil {
			return err
		}
		if lockedTask == nil {
			return &model.ValidationError{Field: "id", Message: "capture task not found"}
		}
		if lockedTask.Status == TLSFingerprintCaptureStatusRunning {
			return &model.ValidationError{Field: "status", Message: "task is already running"}
		}

		// All generation-owned rows and the task update share this transaction.
		// If any step fails, the repository rolls the complete restart back.
		if err := s.repo.DeleteTaskCaptureData(lockCtx, id); err != nil {
			return err
		}
		lockedTask.Status = TLSFingerprintCaptureStatusRunning
		lockedTask.Token = token
		lockedTask.CompletedAt = nil
		lockedTask.TaskStats = map[string]any{}
		lockedTask.Counts = make(map[string]int, len(lockedTask.Targets))
		for platform := range lockedTask.Targets {
			lockedTask.Counts[platform] = 0
		}
		lockedTask.TransportCounts = make(map[string]int, len(lockedTask.TransportTargets))
		for transport := range lockedTask.TransportTargets {
			lockedTask.TransportCounts[transport] = 0
		}
		updated, err = s.repo.UpdateTask(lockCtx, lockedTask)
		return err
	}); err != nil {
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
	task, err := s.ValidateRunningTaskToken(ctx, req.Token)
	if err != nil {
		return nil, err
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
	transport := defaultTLSCaptureTransport(req)
	req.Transport = transport
	sessionID, err := ensureTLSCaptureSessionID(req.SessionID)
	if err != nil {
		return nil, err
	}
	req.SessionID = sessionID
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

	parsed, err := parseTLSFingerprintClientHello(
		req.ClientHello,
		platform,
		transport,
		strings.TrimSpace(req.UserAgent),
		strings.TrimSpace(req.Originator),
		strings.TrimSpace(req.HTTP2Fingerprint),
	)
	unlock := s.lockTaskSubmission(task.ID)
	defer unlock()
	if err != nil {
		req.SessionEventType = defaultString(strings.TrimSpace(req.SessionEventType), "client_hello_parse_failed")
		req.SessionEventError = defaultString(strings.TrimSpace(req.SessionEventError), err.Error())
		parseErr := err
		if lockErr := s.repo.WithTaskSubmissionLock(ctx, task.ID, func(lockCtx context.Context) error {
			lockedTask, lockErr := s.revalidateCaptureTaskUnderLock(lockCtx, task.ID, req.Token)
			if lockErr != nil {
				return lockErr
			}
			task = lockedTask
			session, sessionErr := s.ensureCaptureSession(lockCtx, task, req, platform, nil, parseErr.Error(), "parse_failed")
			if sessionErr != nil {
				return sessionErr
			}
			_, eventErr := s.createCaptureSessionEvent(lockCtx, session, task, req, platform, transport, "", nil, false, false)
			return eventErr
		}); lockErr != nil {
			return nil, lockErr
		}
		return nil, &model.ValidationError{Field: "client_hello", Message: parseErr.Error()}
	}

	var session *TLSFingerprintCaptureSession
	var result *TLSFingerprintCaptureSubmitResult
	if err := s.repo.WithTaskSubmissionLock(ctx, task.ID, func(lockCtx context.Context) error {
		lockedTask, err := s.revalidateCaptureTaskUnderLock(lockCtx, task.ID, req.Token)
		if err != nil {
			return err
		}
		task = lockedTask
		session, err = s.ensureCaptureSession(lockCtx, task, req, platform, parsed, "", "observed")
		if err != nil {
			return err
		}

		counts, transportCounts, err := s.countSamples(lockCtx, task)
		if err != nil {
			return err
		}
		profile := parsed.Profile
		hash := parsed.Derived.ReplayHash
		sampleFingerprint := buildTLSCaptureSampleFingerprint(platform, transport, hash, req)

		if replayable := tlsCaptureReplayable(req.Replayable); !replayable {
			event, eventErr := s.createCaptureSessionEvent(lockCtx, session, task, req, platform, transport, hash, nil, false, replayable)
			if eventErr != nil {
				return eventErr
			}
			task, _, _, err = s.refreshTaskProgress(lockCtx, task)
			if err != nil {
				return err
			}
			result = ignoredTLSCaptureResultWithSession(task, "capture_not_replayable", session, event)
			return nil
		}

		existing, err := s.repo.GetSampleByTaskHash(lockCtx, task.ID, sampleFingerprint)
		if err != nil {
			return err
		}
		if existing != nil {
			event, eventErr := s.createCaptureSessionEvent(lockCtx, session, task, req, platform, transport, hash, idPointer(existing.ID), false, true)
			if eventErr != nil {
				return eventErr
			}
			task, counts, _, err = s.refreshTaskProgress(lockCtx, task)
			if err != nil {
				return err
			}
			result = &TLSFingerprintCaptureSubmitResult{
				Accepted:        true,
				Duplicate:       true,
				FingerprintHash: existing.FingerprintHash,
				Task:            task,
				Sample:          existing,
				Session:         session,
				SessionEvent:    event,
				Counts:          copyStringIntMap(counts),
			}
			return nil
		}

		if reached, reason := tlsCaptureSubmissionQuotaReached(task, counts, transportCounts, platform, transport); reached {
			task, _, _, err = s.refreshTaskProgress(lockCtx, task)
			if err != nil {
				return err
			}
			result = ignoredTLSCaptureResult(task, reason)
			return nil
		}

		rawClientHello := append([]byte(nil), req.ClientHello...)
		sampleProfile := cloneTLSFingerprintCaptureProfile(profile)
		mergeTLSCaptureSampleDimensionsIntoProfile(sampleProfile, strings.TrimSpace(req.ClientType), req.UserAgent, req.StainlessMetadata)
		// Body storage is opt-in (capture_filters.store_body=true). Default keeps summary only.
		storedPayload := ""
		if tlsCaptureTaskStoreBodyEnabled(task) {
			storedPayload = req.RawPayload
		}
		sample := &TLSFingerprintCaptureSample{
			TaskID:            task.ID,
			Platform:          platform,
			Transport:         transport,
			SessionID:         strings.TrimSpace(req.SessionID),
			UserAgent:         strings.TrimSpace(req.UserAgent),
			Originator:        strings.TrimSpace(req.Originator),
			FingerprintHash:   sampleFingerprint,
			ReplayHash:        hash,
			JA3Raw:            strings.TrimSpace(parsed.Derived.JA3Raw),
			JA3Hash:           strings.TrimSpace(parsed.Derived.JA3Hash),
			JA4:               strings.TrimSpace(parsed.Derived.JA4),
			RequestPath:       req.RequestPath,
			HTTPMethod:        req.HTTPMethod,
			IsWebsocket:       req.IsWebsocket,
			WebsocketProtocol: strings.TrimSpace(req.WebsocketProtocol),
			ClientType:        strings.TrimSpace(req.ClientType),
			Model:             req.Model,
			RequestKind:       strings.TrimSpace(req.RequestKind),
			Streaming:         req.Streaming,
			ResponseMode:      strings.TrimSpace(req.ResponseMode),
			HTTP2Fingerprint:  defaultString(strings.TrimSpace(req.HTTP2Fingerprint), strings.TrimSpace(parsed.Derived.Http2Fingerprint)),
			StainlessMetadata: copyStringAnyMap(req.StainlessMetadata),
			Profile:           sampleProfile,
			RawPayload:        storedPayload,
			RawClientHello:    rawClientHello,
			CapturedAt:        time.Now().UTC(),
		}
		created, inserted, err := s.repo.CreateSampleIfAbsent(lockCtx, sample)
		if err != nil {
			return err
		}

		counts, transportCounts, err = s.countSamples(lockCtx, task)
		if err != nil {
			return err
		}
		event, eventErr := s.createCaptureSessionEvent(lockCtx, session, task, req, platform, transport, hash, idPointer(created.ID), inserted, true)
		if eventErr != nil {
			return eventErr
		}
		task, err = s.updateTaskProgress(lockCtx, task, counts, transportCounts)
		if err != nil {
			return err
		}

		result = &TLSFingerprintCaptureSubmitResult{
			Accepted:        true,
			Duplicate:       !inserted,
			FingerprintHash: created.FingerprintHash,
			Task:            task,
			Sample:          created,
			Session:         session,
			SessionEvent:    event,
			Counts:          copyStringIntMap(counts),
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *TLSFingerprintCaptureService) revalidateCaptureTaskUnderLock(ctx context.Context, taskID int64, token string) (*TLSFingerprintCaptureTask, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil || task.Token != strings.TrimSpace(token) ||
		(task.Status != TLSFingerprintCaptureStatusRunning && task.Status != TLSFingerprintCaptureStatusCompleted) {
		return nil, &model.ValidationError{Field: "token", Message: "running capture task not found"}
	}
	return task, nil
}

func (s *TLSFingerprintCaptureService) lockTaskSubmission(taskID int64) func() {
	if s == nil || taskID <= 0 {
		return func() {}
	}

	s.submitMu.Lock()
	if s.taskSubmitMu == nil {
		s.taskSubmitMu = make(map[int64]*tlsFingerprintCaptureTaskMutex)
	}
	taskMu := s.taskSubmitMu[taskID]
	if taskMu == nil {
		taskMu = &tlsFingerprintCaptureTaskMutex{}
		s.taskSubmitMu[taskID] = taskMu
	}
	taskMu.refs++
	s.submitMu.Unlock()

	taskMu.mu.Lock()
	return func() {
		taskMu.mu.Unlock()

		s.submitMu.Lock()
		taskMu.refs--
		if taskMu.refs == 0 {
			delete(s.taskSubmitMu, taskID)
		}
		s.submitMu.Unlock()
	}
}

func (s *TLSFingerprintCaptureService) ValidateRunningTaskToken(ctx context.Context, token string) (*TLSFingerprintCaptureTask, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("tls fingerprint capture repository is not configured")
	}
	token = strings.TrimSpace(token)
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
	return task, nil
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

func (s *TLSFingerprintCaptureService) refreshTaskProgress(ctx context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, map[string]int, map[string]int, error) {
	counts, transportCounts, err := s.countSamples(ctx, task)
	if err != nil {
		return nil, nil, nil, err
	}
	updated, err := s.updateTaskProgress(ctx, task, counts, transportCounts)
	if err != nil {
		return nil, nil, nil, err
	}
	return updated, counts, transportCounts, nil
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

func redactTLSCaptureTaskToken(task *TLSFingerprintCaptureTask) *TLSFingerprintCaptureTask {
	if task == nil {
		return nil
	}
	// decorateCaptureTask mutates in place; clone first so repo-owned objects stay intact.
	clone := *task
	clone.Targets = copyStringIntMap(task.Targets)
	clone.Counts = copyStringIntMap(task.Counts)
	clone.TransportTargets = copyStringIntMap(task.TransportTargets)
	clone.TransportCounts = copyStringIntMap(task.TransportCounts)
	clone.CaptureFilters = copyStringAnyMap(task.CaptureFilters)
	clone.TaskStats = copyStringAnyMap(task.TaskStats)
	if task.UAKeywords != nil {
		clone.UAKeywords = append([]string(nil), task.UAKeywords...)
	}
	if task.CompletedAt != nil {
		completed := *task.CompletedAt
		clone.CompletedAt = &completed
	}
	clone.Token = ""
	return &clone
}

func tlsCaptureTaskStoreBodyEnabled(task *TLSFingerprintCaptureTask) bool {
	if task == nil || len(task.CaptureFilters) == 0 {
		return false
	}
	raw, ok := task.CaptureFilters["store_body"]
	if !ok {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func tlsCaptureStoredRawPayload(task *TLSFingerprintCaptureTask, raw string) string {
	if !tlsCaptureTaskStoreBodyEnabled(task) {
		return ""
	}
	return raw
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

func (s *TLSFingerprintCaptureService) ensureCaptureSession(
	ctx context.Context,
	task *TLSFingerprintCaptureTask,
	req TLSFingerprintCaptureNativeSubmitRequest,
	platform string,
	parsed *tlsFingerprintParsedCapture,
	errorSummary, sessionStatus string,
) (*TLSFingerprintCaptureSession, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	session := &TLSFingerprintCaptureSession{
		TaskID:         task.ID,
		SessionID:      sessionID,
		ClientIP:       strings.TrimSpace(req.ClientIP),
		Platform:       platform,
		UserAgent:      strings.TrimSpace(req.UserAgent),
		Originator:     strings.TrimSpace(req.Originator),
		ALPNNegotiated: strings.TrimSpace(req.ALPNNegotiated),
		RawClientHello: append([]byte(nil), req.ClientHello...),
		SessionStatus:  defaultString(strings.TrimSpace(sessionStatus), "observed"),
		ErrorSummary:   strings.TrimSpace(errorSummary),
		OpenedAt:       time.Now().UTC(),
	}
	if parsed != nil {
		session.ObservedClientHello = structToStringAnyMap(parsed.Observed)
		session.ReplayProfile = structToStringAnyMap(parsed.ReplayProfile)
		session.DerivedFingerprint = structToStringAnyMap(parsed.Derived)
	}
	created, _, err := s.repo.CreateSessionIfAbsent(ctx, session)
	return created, err
}

func (s *TLSFingerprintCaptureService) createCaptureSessionEvent(
	ctx context.Context,
	session *TLSFingerprintCaptureSession,
	task *TLSFingerprintCaptureTask,
	req TLSFingerprintCaptureNativeSubmitRequest,
	platform, transport, replayHash string,
	sampleID *int64,
	sampleRecorded bool,
	replayable bool,
) (*TLSFingerprintCaptureSessionEvent, error) {
	eventType := strings.TrimSpace(req.SessionEventType)
	if eventType == "" {
		if sampleRecorded {
			eventType = "replayable_sample_recorded"
		} else {
			eventType = "session_observed"
		}
	}
	event := &TLSFingerprintCaptureSessionEvent{
		TaskID:            task.ID,
		SessionID:         strings.TrimSpace(req.SessionID),
		EventID:           strings.TrimSpace(req.EventID),
		Platform:          platform,
		Transport:         transport,
		EventType:         eventType,
		RequestSequence:   req.RequestSequence,
		StreamID:          strings.TrimSpace(req.StreamID),
		RequestPath:       req.RequestPath,
		HTTPMethod:        req.HTTPMethod,
		IsWebsocket:       req.IsWebsocket,
		WebsocketProtocol: strings.TrimSpace(req.WebsocketProtocol),
		ClientType:        strings.TrimSpace(req.ClientType),
		Model:             req.Model,
		RequestKind:       strings.TrimSpace(req.RequestKind),
		Streaming:         req.Streaming,
		ResponseMode:      strings.TrimSpace(req.ResponseMode),
		UserAgent:         strings.TrimSpace(req.UserAgent),
		Originator:        strings.TrimSpace(req.Originator),
		StainlessMetadata: copyStringAnyMap(req.StainlessMetadata),
		HeadersSnapshot:   copyStringAnyMap(req.HeadersSnapshot),
		BodySummary:       req.BodySummary,
		RawPayload:        tlsCaptureStoredRawPayload(task, req.RawPayload),
		EventStatus:       defaultTLSCaptureEventStatus(req, sampleRecorded, replayable),
		Error:             strings.TrimSpace(req.SessionEventError),
		Replayable:        replayable,
		SampleID:          sampleID,
		ReplayHash:        replayHash,
	}
	if session != nil {
		event.SessionRef = session.ID
	}
	return s.repo.CreateSessionEvent(ctx, event)
}

func normalizeTLSCaptureTransport(transport string) string {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "", string(tlsfpTransport.HTTP1), string(tlsfpTransport.H2), string(tlsfpTransport.WebSocketH1), string(tlsfpTransport.WebSocketH2):
		return strings.ToLower(strings.TrimSpace(transport))
	case "http", "https", "http/1.1", "h1":
		return string(tlsfpTransport.HTTP1)
	case "ws", "wss", "websocket":
		return string(tlsfpTransport.WebSocketH1)
	default:
		return strings.ToLower(strings.TrimSpace(transport))
	}
}

func defaultTLSCaptureTransport(req TLSFingerprintCaptureNativeSubmitRequest) string {
	if transport := normalizeTLSCaptureTransport(req.Transport); transport != "" {
		return transport
	}
	if strings.EqualFold(strings.TrimSpace(req.ALPNNegotiated), "h2") {
		if req.IsWebsocket {
			return string(tlsfpTransport.WebSocketH2)
		}
		return string(tlsfpTransport.H2)
	}
	if req.IsWebsocket {
		return string(tlsfpTransport.WebSocketH1)
	}
	return string(tlsfpTransport.HTTP1)
}

func defaultTLSCaptureEventStatus(req TLSFingerprintCaptureNativeSubmitRequest, sampleRecorded bool, replayable bool) string {
	if status := strings.TrimSpace(req.SessionEventError); status != "" {
		return "error"
	}
	if sampleRecorded {
		return "recorded"
	}
	if !replayable {
		return "ignored"
	}
	return "observed"
}

func ensureTLSCaptureSessionID(sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		return sessionID, nil
	}
	token, err := newTLSFingerprintCaptureToken()
	if err != nil {
		return "", err
	}
	return "capture-" + token, nil
}

func copyStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func structToStringAnyMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil
	}
	return out
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
	if sample.Profile == nil {
		if sample.RawPayload != "" {
			return sample.RawPayload, nil
		}
		return "", &model.ValidationError{Field: "replay_profile", Message: "captured sample replay profile is required"}
	}
	profile := cloneTLSFingerprintCaptureProfile(sample.Profile)
	mergeTLSCaptureSampleDimensionsIntoProfile(profile, strings.TrimSpace(sample.ClientType), sample.UserAgent, sample.StainlessMetadata)
	if profile != nil && strings.TrimSpace(profile.HTTP2Fingerprint) == "" {
		profile.HTTP2Fingerprint = strings.TrimSpace(sample.HTTP2Fingerprint)
	}
	encoded, err := json.Marshal(profile)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func cloneTLSFingerprintCaptureProfile(profile *model.TLSFingerprintProfile) *model.TLSFingerprintProfile {
	if profile == nil {
		return nil
	}
	clone := *profile
	if profile.Description != nil {
		desc := *profile.Description
		clone.Description = &desc
	}
	clone.CipherSuites = append([]uint16(nil), profile.CipherSuites...)
	clone.Curves = append([]uint16(nil), profile.Curves...)
	clone.PointFormats = append([]uint16(nil), profile.PointFormats...)
	clone.SignatureAlgorithms = append([]uint16(nil), profile.SignatureAlgorithms...)
	clone.SignatureAlgorithmsCert = append([]uint16(nil), profile.SignatureAlgorithmsCert...)
	clone.ALPNProtocols = append([]string(nil), profile.ALPNProtocols...)
	clone.SupportedVersions = append([]uint16(nil), profile.SupportedVersions...)
	clone.KeyShareGroups = append([]uint16(nil), profile.KeyShareGroups...)
	clone.PSKModes = append([]uint16(nil), profile.PSKModes...)
	clone.Extensions = append([]uint16(nil), profile.Extensions...)
	clone.ExtensionPayloads = cloneExtensionPayloads(profile.ExtensionPayloads)
	clone.CompressCertAlgos = append([]uint16(nil), profile.CompressCertAlgos...)
	clone.DelegatedCredentialsAlgorithms = append([]uint16(nil), profile.DelegatedCredentialsAlgorithms...)
	clone.ApplicationSettingsProtocols = append([]string(nil), profile.ApplicationSettingsProtocols...)
	return &clone
}

func mergeTLSCaptureSampleDimensionsIntoProfile(profile *model.TLSFingerprintProfile, clientType, userAgent string, stainlessMetadata map[string]any) {
	if profile == nil {
		return
	}
	if strings.TrimSpace(profile.ClientType) == "" {
		profile.ClientType = normalizeTLSCaptureClientType(clientType)
	}
	if strings.TrimSpace(profile.OS) == "" {
		profile.OS = normalizeTLSCaptureProfileOS(stringFromAny(stainlessMetadata["os"]))
		if profile.OS == "" {
			profile.OS = inferTLSFingerprintOS(userAgent)
		}
	}
}

func normalizeTLSCaptureProfileOS(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "macos", "mac", "darwin", "osx", "mac os", "mac os x":
		return "macos"
	case "linux":
		return "linux"
	case "windows", "win", "win32":
		return "windows"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
}

func buildTLSCaptureSampleFingerprint(platform, transport, replayHash string, req TLSFingerprintCaptureNativeSubmitRequest) string {
	stainlessJSON, _ := json.Marshal(copyStringAnyMap(req.StainlessMetadata))
	material := strings.Join([]string{
		fingerprintComponent(strings.TrimSpace(platform)),
		fingerprintComponent(normalizeTLSCaptureTransport(transport)),
		fingerprintComponent(strings.TrimSpace(replayHash)),
		fingerprintComponent(strings.TrimSpace(req.UserAgent)),
		fingerprintComponent(strings.TrimSpace(req.Originator)),
		fingerprintComponent(req.RequestPath),
		fingerprintComponent(strings.TrimSpace(req.HTTPMethod)),
		fingerprintComponent(boolFingerprintComponent(req.IsWebsocket)),
		fingerprintComponent(strings.TrimSpace(req.WebsocketProtocol)),
		fingerprintComponent(strings.TrimSpace(req.ClientType)),
		fingerprintComponent(strings.TrimSpace(req.Model)),
		fingerprintComponent(strings.TrimSpace(req.RequestKind)),
		fingerprintComponent(boolFingerprintComponent(req.Streaming)),
		fingerprintComponent(strings.TrimSpace(req.ResponseMode)),
		fingerprintComponent(strings.TrimSpace(req.HTTP2Fingerprint)),
		fingerprintComponent(string(stainlessJSON)),
		fingerprintComponent(req.RawPayload),
	}, "|")
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:])
}

func fingerprintComponent(value string) string {
	return fmt.Sprintf("%d:%s", len(value), value)
}

func boolFingerprintComponent(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func parseTLSFingerprintClientHello(raw []byte, platform, transport, userAgent, originator, http2Fingerprint string) (*tlsFingerprintParsedCapture, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("client hello is required")
	}
	observed, err := tlsfpParser.ParseObservedClientHello(raw)
	if err != nil {
		return nil, fmt.Errorf("parse client hello: %w", err)
	}
	replayProfile, err := tlsfpReplay.ReplayProfileFromObserved(observed)
	if err != nil {
		return nil, err
	}
	derived, err := tlsfpParser.DeriveFingerprints(observed)
	if err != nil {
		return nil, err
	}
	profile := tlsFingerprintProfileFromReplayProfile(replayProfile)
	profile.Name = "native ClientHello capture"
	profile.Platform = platform
	profile.Transport = transport
	profile.UserAgent = userAgent
	profile.Originator = originator
	profile.HTTP2Fingerprint = http2Fingerprint
	if err := validateCompleteTLSFingerprintProfile(profile); err != nil {
		return nil, err
	}
	return &tlsFingerprintParsedCapture{
		Observed:      observed,
		ReplayProfile: replayProfile,
		Derived:       derived,
		Profile:       profile,
	}, nil
}

func tlsFingerprintProfileFromReplayProfile(profile *tlsfpReplay.ReplayProfile) *model.TLSFingerprintProfile {
	if profile == nil {
		return &model.TLSFingerprintProfile{Name: "native ClientHello capture"}
	}
	return &model.TLSFingerprintProfile{
		Name:                           "native ClientHello capture",
		EnableGREASE:                   profile.EnableGREASE,
		CipherSuites:                   append([]uint16(nil), profile.CipherSuites...),
		Curves:                         append([]uint16(nil), profile.Curves...),
		PointFormats:                   append([]uint16(nil), profile.PointFormats...),
		SignatureAlgorithms:            append([]uint16(nil), profile.SignatureAlgorithms...),
		SignatureAlgorithmsCert:        append([]uint16(nil), profile.SignatureAlgorithmsCert...),
		ALPNProtocols:                  append([]string(nil), profile.ALPNProtocols...),
		SupportedVersions:              append([]uint16(nil), profile.SupportedVersions...),
		KeyShareGroups:                 append([]uint16(nil), profile.KeyShareGroups...),
		PSKModes:                       append([]uint16(nil), profile.PSKModes...),
		Extensions:                     append([]uint16(nil), profile.Extensions...),
		ExtensionPayloads:              cloneUint16BytesMap(profile.ExtensionPayloads),
		CompressCertAlgos:              append([]uint16(nil), profile.CompressCertAlgos...),
		DelegatedCredentialsAlgorithms: append([]uint16(nil), profile.DelegatedCredentialsAlgorithms...),
		ApplicationSettingsProtocols:   append([]string(nil), profile.ApplicationSettingsProtocols...),
	}
}

func cloneUint16BytesMap(in map[uint16][]byte) map[uint16][]byte {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint16][]byte, len(in))
	for key, value := range in {
		out[key] = append([]byte(nil), value...)
	}
	return out
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func newTLSFingerprintCaptureToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
