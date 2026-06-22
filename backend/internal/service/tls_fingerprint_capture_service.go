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
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Status         string         `json:"status"`
	Token          string         `json:"token,omitempty"`
	Targets        map[string]int `json:"targets"`
	Counts         map[string]int `json:"counts"`
	UAKeywords     []string       `json:"ua_keywords"`
	CaptureBaseURL string         `json:"capture_base_url,omitempty"`
	CaptureURL     string         `json:"capture_url,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
}

type TLSFingerprintCaptureSample struct {
	ID              int64                        `json:"id"`
	TaskID          int64                        `json:"task_id"`
	Platform        string                       `json:"platform"`
	UserAgent       string                       `json:"user_agent"`
	Originator      string                       `json:"originator"`
	FingerprintHash string                       `json:"fingerprint_hash"`
	Profile         *model.TLSFingerprintProfile `json:"profile"`
	RawPayload      string                       `json:"raw_payload"`
	RawClientHello  []byte                       `json:"raw_client_hello,omitempty"`
	CreatedAt       time.Time                    `json:"created_at"`
}

type TLSFingerprintCaptureStartRequest struct {
	Name       string         `json:"name"`
	Targets    map[string]int `json:"targets"`
	UAKeywords []string       `json:"ua_keywords"`
}

type TLSFingerprintCaptureNativeSubmitRequest struct {
	Token       string `json:"token"`
	Platform    string `json:"platform"`
	UserAgent   string `json:"user_agent"`
	Originator  string `json:"originator"`
	ClientHello []byte `json:"client_hello"`
}

type TLSFingerprintCaptureTaskImportRequest struct {
	TaskID    int64   `json:"task_id"`
	SampleIDs []int64 `json:"sample_ids"`
}

type TLSFingerprintCaptureSubmitResult struct {
	Accepted        bool                         `json:"accepted"`
	Duplicate       bool                         `json:"duplicate"`
	IgnoredReason   string                       `json:"ignored_reason,omitempty"`
	FingerprintHash string                       `json:"fingerprint_hash,omitempty"`
	Task            *TLSFingerprintCaptureTask   `json:"task"`
	Sample          *TLSFingerprintCaptureSample `json:"sample,omitempty"`
	Counts          map[string]int               `json:"counts"`
}

type TLSFingerprintCaptureRepository interface {
	CreateTask(ctx context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error)
	ListTasks(ctx context.Context) ([]*TLSFingerprintCaptureTask, error)
	GetTaskByID(ctx context.Context, id int64) (*TLSFingerprintCaptureTask, error)
	GetRunningTaskByToken(ctx context.Context, token string) (*TLSFingerprintCaptureTask, error)
	UpdateTask(ctx context.Context, task *TLSFingerprintCaptureTask) (*TLSFingerprintCaptureTask, error)
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
		Name:       name,
		Status:     TLSFingerprintCaptureStatusRunning,
		Token:      token,
		Targets:    targets,
		Counts:     make(map[string]int, len(targets)),
		UAKeywords: normalizeTLSCaptureKeywords(req.UAKeywords),
	}
	for platform := range targets {
		task.Counts[platform] = 0
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
	if !tlsCaptureUserAgentMatches(task.UAKeywords, req.UserAgent) {
		return ignoredTLSCaptureResult(task, "user_agent_not_matched"), nil
	}
	if len(req.ClientHello) == 0 {
		return ignoredTLSCaptureResult(task, "client_hello_required"), nil
	}

	counts, err := s.countSamples(ctx, task)
	if err != nil {
		return nil, err
	}
	if tlsCapturePlatformTargetReached(task.Targets, counts, platform) {
		task, err = s.updateTaskProgress(ctx, task, counts)
		if err != nil {
			return nil, err
		}
		return ignoredTLSCaptureResult(task, "platform_target_reached"), nil
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

	existing, err := s.repo.GetSampleByTaskHash(ctx, task.ID, hash)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		task, err = s.updateTaskProgress(ctx, task, counts)
		if err != nil {
			return nil, err
		}
		return &TLSFingerprintCaptureSubmitResult{
			Accepted:        true,
			Duplicate:       true,
			FingerprintHash: hash,
			Task:            task,
			Sample:          existing,
			Counts:          copyStringIntMap(counts),
		}, nil
	}

	rawClientHello := append([]byte(nil), req.ClientHello...)
	sample := &TLSFingerprintCaptureSample{
		TaskID:          task.ID,
		Platform:        platform,
		UserAgent:       strings.TrimSpace(req.UserAgent),
		Originator:      strings.TrimSpace(req.Originator),
		FingerprintHash: hash,
		Profile:         profile,
		RawClientHello:  rawClientHello,
	}
	created, inserted, err := s.repo.CreateSampleIfAbsent(ctx, sample)
	if err != nil {
		return nil, err
	}

	counts, err = s.countSamples(ctx, task)
	if err != nil {
		return nil, err
	}
	task, err = s.updateTaskProgress(ctx, task, counts)
	if err != nil {
		return nil, err
	}

	return &TLSFingerprintCaptureSubmitResult{
		Accepted:        true,
		Duplicate:       !inserted,
		FingerprintHash: hash,
		Task:            task,
		Sample:          created,
		Counts:          copyStringIntMap(counts),
	}, nil
}

func (s *TLSFingerprintCaptureService) updateTaskProgress(ctx context.Context, task *TLSFingerprintCaptureTask, counts map[string]int) (*TLSFingerprintCaptureTask, error) {
	latest, err := s.repo.GetTaskByID(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, &model.ValidationError{Field: "task_id", Message: "capture task not found"}
	}
	latest.Counts = counts
	if latest.Status == TLSFingerprintCaptureStatusRunning && tlsCaptureTargetsReached(latest.Targets, counts) {
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

func (s *TLSFingerprintCaptureService) countSamples(ctx context.Context, task *TLSFingerprintCaptureTask) (map[string]int, error) {
	counts := make(map[string]int, len(task.Targets))
	for platform := range task.Targets {
		counts[platform] = 0
	}
	samples, err := s.repo.ListSamplesByTask(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		if _, ok := task.Targets[sample.Platform]; ok {
			counts[sample.Platform]++
		}
	}
	return counts, nil
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
