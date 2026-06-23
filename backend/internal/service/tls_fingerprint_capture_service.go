package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	tlsfpParser "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/parser"
	tlsfpReplay "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/replay"
	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
	utls "github.com/refraction-networking/utls"
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
	UpdateTask(ctx context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error)
	DeleteTask(ctx context.Context, id int64) error
	DeleteSamplesByTask(ctx context.Context, taskID int64) error
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
		CaptureFilters:      map[string]any{},
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

	parsed, err := parseTLSFingerprintClientHello(req.ClientHello, platform, transport, strings.TrimSpace(req.UserAgent), strings.TrimSpace(req.Originator))
	if err != nil {
		req.SessionEventType = defaultString(strings.TrimSpace(req.SessionEventType), "client_hello_parse_failed")
		req.SessionEventError = defaultString(strings.TrimSpace(req.SessionEventError), err.Error())
		session, sessionErr := s.ensureCaptureSession(ctx, task, req, platform, nil, err.Error(), "parse_failed")
		if sessionErr != nil {
			return nil, sessionErr
		}
		if _, eventErr := s.createCaptureSessionEvent(ctx, session, task, req, platform, transport, "", nil, false, false); eventErr != nil {
			return nil, eventErr
		}
		return nil, &model.ValidationError{Field: "client_hello", Message: err.Error()}
	}

	session, sessionErr := s.ensureCaptureSession(ctx, task, req, platform, parsed, "", "observed")
	if sessionErr != nil {
		return nil, sessionErr
	}

	counts, transportCounts, err := s.countSamples(ctx, task)
	if err != nil {
		return nil, err
	}
	profile := parsed.Profile
	hash := parsed.Derived.ReplayHash
	sampleFingerprint := buildTLSCaptureSampleFingerprint(platform, transport, hash, req)

	if replayable := tlsCaptureReplayable(req.Replayable); !replayable {
		event, eventErr := s.createCaptureSessionEvent(ctx, session, task, req, platform, transport, hash, nil, false, replayable)
		if eventErr != nil {
			return nil, eventErr
		}
		task, counts, transportCounts, err = s.refreshTaskProgress(ctx, task)
		if err != nil {
			return nil, err
		}
		return ignoredTLSCaptureResultWithSession(task, "capture_not_replayable", session, event), nil
	}

	existing, err := s.repo.GetSampleByTaskHash(ctx, task.ID, sampleFingerprint)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		event, eventErr := s.createCaptureSessionEvent(ctx, session, task, req, platform, transport, hash, idPointer(existing.ID), false, true)
		if eventErr != nil {
			return nil, eventErr
		}
		task, counts, transportCounts, err = s.refreshTaskProgress(ctx, task)
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

	if tlsCaptureSubmissionQuotaReached(task, counts, transportCounts, platform, transport) {
		task, counts, transportCounts, err = s.refreshTaskProgress(ctx, task)
		if err != nil {
			return nil, err
		}
		reason := "platform_target_reached"
		if len(task.TransportTargets) > 0 && !tlsCapturePlatformTargetReached(task.Targets, counts, platform) && tlsCaptureTransportTargetReached(task.TransportTargets, transportCounts, transport) {
			reason = "transport_target_reached"
		}
		return ignoredTLSCaptureResult(task, reason), nil
	}

	rawClientHello := append([]byte(nil), req.ClientHello...)
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
		Profile:           profile,
		RawPayload:        req.RawPayload,
		RawClientHello:    rawClientHello,
		CapturedAt:        time.Now().UTC(),
	}
	created, inserted, err := s.repo.CreateSampleIfAbsent(ctx, sample)
	if err != nil {
		return nil, err
	}

	counts, transportCounts, err = s.countSamples(ctx, task)
	if err != nil {
		return nil, err
	}
	event, eventErr := s.createCaptureSessionEvent(ctx, session, task, req, platform, transport, hash, idPointer(created.ID), inserted, true)
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
		FingerprintHash: created.FingerprintHash,
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

func tlsCaptureSubmissionQuotaReached(task *TLSFingerprintCaptureTask, counts, transportCounts map[string]int, platform, transport string) bool {
	platformNeeded := true
	if target := task.Targets[platform]; target > 0 {
		platformNeeded = counts[platform] < target
	}
	transportNeeded := false
	if len(task.TransportTargets) > 0 {
		if target := task.TransportTargets[transport]; target > 0 {
			transportNeeded = transportCounts[transport] < target
		}
	}
	return !platformNeeded && !transportNeeded
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
		RawPayload:        req.RawPayload,
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
	encoded, err := json.Marshal(sample.Profile)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
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

func parseTLSFingerprintClientHello(raw []byte, platform, transport, userAgent, originator string) (*tlsFingerprintParsedCapture, error) {
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
