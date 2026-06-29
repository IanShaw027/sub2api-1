//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

type tlsFingerprintProfileHandlerRepoStub struct {
	profiles []*model.TLSFingerprintProfile
	nextID   int64
}

func (r *tlsFingerprintProfileHandlerRepoStub) List(context.Context) ([]*model.TLSFingerprintProfile, error) {
	out := make([]*model.TLSFingerprintProfile, len(r.profiles))
	copy(out, r.profiles)
	return out, nil
}

func (r *tlsFingerprintProfileHandlerRepoStub) GetByID(_ context.Context, id int64) (*model.TLSFingerprintProfile, error) {
	for _, profile := range r.profiles {
		if profile.ID == id {
			clone := *profile
			return &clone, nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintProfileHandlerRepoStub) Create(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	created := *profile
	if r.nextID == 0 {
		r.nextID = 1
	}
	created.ID = r.nextID
	r.nextID++
	r.profiles = append(r.profiles, &created)
	return &created, nil
}

func (r *tlsFingerprintProfileHandlerRepoStub) Update(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	updated := *profile
	for i, existing := range r.profiles {
		if existing.ID == profile.ID {
			r.profiles[i] = &updated
			return &updated, nil
		}
	}
	r.profiles = append(r.profiles, &updated)
	return &updated, nil
}

func (r *tlsFingerprintProfileHandlerRepoStub) Delete(_ context.Context, id int64) error {
	next := r.profiles[:0]
	for _, profile := range r.profiles {
		if profile.ID != id {
			next = append(next, profile)
		}
	}
	r.profiles = next
	return nil
}

func TestTLSFingerprintProfileHandlerImportCaptures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &tlsFingerprintProfileHandlerRepoStub{}
	svc := service.NewTLSFingerprintProfileService(repo, nil)
	handler := NewTLSFingerprintProfileHandler(svc, service.NewTLSFingerprintCaptureService(newTLSFingerprintCaptureHandlerRepoStub(), svc))

	router := gin.New()
	router.POST("/api/v1/admin/tls-fingerprint-profiles/import-captures", handler.ImportCaptures)

	body := `{"profiles":[{"name":"Codex Desktop live capture","enable_grease":false,"cipher_suites":[4865,4866],"curves":[29,23],"point_formats":[0],"signature_algorithms":[1027],"alpn_protocols":["http/1.1"],"supported_versions":[772,771],"key_share_groups":[29],"psk_modes":[1],"extensions":[0,11,10,13,43,45,51]}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tls-fingerprint-profiles/import-captures", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Imported   int `json:"imported"`
			Duplicates int `json:"duplicates"`
			Profiles   []struct {
				Duplicate       bool   `json:"duplicate"`
				FingerprintHash string `json:"fingerprint_hash"`
				Profile         struct {
					ID   int64  `json:"id"`
					Name string `json:"name"`
				} `json:"profile"`
			} `json:"profiles"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)
	require.Equal(t, 1, envelope.Data.Imported)
	require.Equal(t, 0, envelope.Data.Duplicates)
	require.Len(t, envelope.Data.Profiles, 1)
	require.False(t, envelope.Data.Profiles[0].Duplicate)
	require.NotEmpty(t, envelope.Data.Profiles[0].FingerprintHash)
	require.Equal(t, int64(1), envelope.Data.Profiles[0].Profile.ID)
	require.Equal(t, "Codex Desktop live capture", envelope.Data.Profiles[0].Profile.Name)
}

func TestTLSFingerprintProfileHandlerCreateAcceptsReplayFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &tlsFingerprintProfileHandlerRepoStub{}
	svc := service.NewTLSFingerprintProfileService(repo, nil)
	handler := NewTLSFingerprintProfileHandler(svc, nil)
	router := gin.New()
	router.POST("/api/v1/admin/tls-fingerprint-profiles", handler.Create)

	body := `{"name":"captured","platform":"openai","extensions":[50,65037],"signature_algorithms_cert":[1027],"extension_payloads":{"65037":"AQID"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tls-fingerprint-profiles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, repo.profiles, 1)
	require.Equal(t, []uint16{1027}, repo.profiles[0].SignatureAlgorithmsCert)
	require.Equal(t, []byte{1, 2, 3}, repo.profiles[0].ExtensionPayloads[65037])
}

func TestTLSFingerprintProfileHandlerUpdatePreservesDimensionAndReplayFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &tlsFingerprintProfileHandlerRepoStub{profiles: []*model.TLSFingerprintProfile{
		{
			ID:                      1,
			Name:                    "captured",
			Platform:                "openai",
			Transport:               "h2",
			OS:                      "macos",
			ClientType:              "codex-cli",
			Extensions:              []uint16{50, 65037},
			SignatureAlgorithmsCert: []uint16{1027},
			ExtensionPayloads:       map[uint16][]byte{65037: {1, 2, 3}},
		},
	}}
	svc := service.NewTLSFingerprintProfileService(repo, nil)
	handler := NewTLSFingerprintProfileHandler(svc, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/tls-fingerprint-profiles/:id", handler.Update)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/tls-fingerprint-profiles/1", strings.NewReader(`{"name":"renamed"}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, repo.profiles, 1)
	updated := repo.profiles[0]
	require.Equal(t, "renamed", updated.Name)
	require.Equal(t, "macos", updated.OS)
	require.Equal(t, "codex-cli", updated.ClientType)
	require.Equal(t, []uint16{1027}, updated.SignatureAlgorithmsCert)
	require.Equal(t, []byte{1, 2, 3}, updated.ExtensionPayloads[65037])
}

func TestTLSFingerprintProfileHandlerCaptureTaskLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	profileRepo := &tlsFingerprintProfileHandlerRepoStub{}
	profileSvc := service.NewTLSFingerprintProfileService(profileRepo, nil)
	captureRepo := newTLSFingerprintCaptureHandlerRepoStub()
	captureSvc := service.NewTLSFingerprintCaptureService(captureRepo, profileSvc)
	handler := NewTLSFingerprintProfileHandler(profileSvc, captureSvc)

	router := gin.New()
	router.POST("/api/v1/admin/tls-fingerprint-profiles/capture-tasks", handler.StartCaptureTask)
	router.GET("/api/v1/admin/tls-fingerprint-profiles/capture-tasks", handler.ListCaptureTasks)
	router.GET("/api/v1/admin/tls-fingerprint-profiles/capture-tasks/:id/samples", handler.ListCaptureSamples)
	router.POST("/api/v1/admin/tls-fingerprint-profiles/capture-tasks/:id/import", handler.ImportCaptureTaskSamples)

	startBody := `{"name":"Codex live","targets":{"openai":2},"ua_keywords":["codex"]}`
	startRec := httptest.NewRecorder()
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tls-fingerprint-profiles/capture-tasks", strings.NewReader(startBody))
	startReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(startRec, startReq)
	require.Equal(t, http.StatusOK, startRec.Code, startRec.Body.String())

	var started struct {
		Data struct {
			ID     int64  `json:"id"`
			Token  string `json:"token"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(startRec.Body.Bytes(), &started))
	require.NotEmpty(t, started.Data.Token)
	require.Equal(t, service.TLSFingerprintCaptureStatusRunning, started.Data.Status)

	_, err := captureSvc.SubmitNativeCapture(context.Background(), service.TLSFingerprintCaptureNativeSubmitRequest{
		Token:       started.Data.Token,
		Platform:    "openai",
		UserAgent:   "codex-tui/0.140.0",
		Originator:  "codex_cli_rs",
		ClientHello: nativeCaptureHandlerClientHello(t),
	})
	require.NoError(t, err)

	samplesRec := httptest.NewRecorder()
	samplesReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tls-fingerprint-profiles/capture-tasks/1/samples", nil)
	router.ServeHTTP(samplesRec, samplesReq)
	require.Equal(t, http.StatusOK, samplesRec.Code, samplesRec.Body.String())

	var samplesEnvelope struct {
		Data []struct {
			ID              int64  `json:"id"`
			Platform        string `json:"platform"`
			FingerprintHash string `json:"fingerprint_hash"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(samplesRec.Body.Bytes(), &samplesEnvelope))
	require.Len(t, samplesEnvelope.Data, 1)
	require.Equal(t, "openai", samplesEnvelope.Data[0].Platform)
	require.NotEmpty(t, samplesEnvelope.Data[0].FingerprintHash)

	importRec := httptest.NewRecorder()
	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tls-fingerprint-profiles/capture-tasks/1/import", strings.NewReader(`{"sample_ids":[1]}`))
	importReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(importRec, importReq)
	require.Equal(t, http.StatusOK, importRec.Code, importRec.Body.String())

	var importEnvelope struct {
		Data struct {
			Imported int `json:"imported"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(importRec.Body.Bytes(), &importEnvelope))
	require.Equal(t, 1, importEnvelope.Data.Imported)
	require.Len(t, profileRepo.profiles, 1)
	require.Equal(t, "openai", profileRepo.profiles[0].Platform)

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tls-fingerprint-profiles/capture-tasks", nil)
	router.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code, listRec.Body.String())
}

type tlsFingerprintCaptureHandlerRepoStub struct {
	mu            sync.Mutex
	nextTaskID    int64
	nextSampleID  int64
	nextSessionID int64
	nextEventID   int64
	tasks         []*service.TLSFingerprintCaptureTask
	samples       []*service.TLSFingerprintCaptureSample
	sessions      []*service.TLSFingerprintCaptureSession
	sessionEvents []*service.TLSFingerprintCaptureSessionEvent
}

func newTLSFingerprintCaptureHandlerRepoStub() *tlsFingerprintCaptureHandlerRepoStub {
	return &tlsFingerprintCaptureHandlerRepoStub{nextTaskID: 1, nextSampleID: 1, nextSessionID: 1, nextEventID: 1}
}

func (r *tlsFingerprintCaptureHandlerRepoStub) CreateTask(_ context.Context, task *service.TLSFingerprintCaptureTask) (*service.TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	created := cloneTLSFingerprintCaptureHandlerTask(task)
	created.ID = r.nextTaskID
	r.nextTaskID++
	now := time.Now().UTC()
	created.CreatedAt = now
	created.UpdatedAt = now
	r.tasks = append(r.tasks, created)
	return cloneTLSFingerprintCaptureHandlerTask(created), nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) ListTasks(_ context.Context) ([]*service.TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*service.TLSFingerprintCaptureTask, 0, len(r.tasks))
	for _, task := range r.tasks {
		out = append(out, cloneTLSFingerprintCaptureHandlerTask(task))
	}
	return out, nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) GetTaskByID(_ context.Context, id int64) (*service.TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, task := range r.tasks {
		if task.ID == id {
			return cloneTLSFingerprintCaptureHandlerTask(task), nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) GetRunningTaskByToken(_ context.Context, token string) (*service.TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, task := range r.tasks {
		if task.Token == token && task.Status == service.TLSFingerprintCaptureStatusRunning {
			return cloneTLSFingerprintCaptureHandlerTask(task), nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) UpdateTask(_ context.Context, task *service.TLSFingerprintCaptureTask) (*service.TLSFingerprintCaptureTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	updated := cloneTLSFingerprintCaptureHandlerTask(task)
	updated.UpdatedAt = time.Now().UTC()
	for i, existing := range r.tasks {
		if existing.ID == task.ID {
			r.tasks[i] = updated
			return cloneTLSFingerprintCaptureHandlerTask(updated), nil
		}
	}
	r.tasks = append(r.tasks, updated)
	return cloneTLSFingerprintCaptureHandlerTask(updated), nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) DeleteTask(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.tasks[:0]
	for _, task := range r.tasks {
		if task.ID != id {
			next = append(next, task)
		}
	}
	r.tasks = next
	return nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) DeleteSamplesByTask(_ context.Context, taskID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.samples[:0]
	for _, sample := range r.samples {
		if sample.TaskID != taskID {
			next = append(next, sample)
		}
	}
	r.samples = next
	return nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) CreateSampleIfAbsent(_ context.Context, sample *service.TLSFingerprintCaptureSample) (*service.TLSFingerprintCaptureSample, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.samples {
		if existing.TaskID == sample.TaskID && existing.FingerprintHash == sample.FingerprintHash {
			return cloneTLSFingerprintCaptureHandlerSample(existing), false, nil
		}
	}
	created := cloneTLSFingerprintCaptureHandlerSample(sample)
	created.ID = r.nextSampleID
	r.nextSampleID++
	created.CreatedAt = time.Now().UTC()
	r.samples = append(r.samples, created)
	return cloneTLSFingerprintCaptureHandlerSample(created), true, nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) CreateSessionIfAbsent(_ context.Context, session *service.TLSFingerprintCaptureSession) (*service.TLSFingerprintCaptureSession, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.sessions {
		if existing.TaskID == session.TaskID && existing.SessionID == session.SessionID {
			return cloneTLSFingerprintCaptureHandlerSession(existing), false, nil
		}
	}
	created := cloneTLSFingerprintCaptureHandlerSession(session)
	created.ID = r.nextSessionID
	r.nextSessionID++
	now := time.Now().UTC()
	created.CreatedAt = now
	created.UpdatedAt = now
	r.sessions = append(r.sessions, created)
	return cloneTLSFingerprintCaptureHandlerSession(created), true, nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) CreateSessionEvent(_ context.Context, event *service.TLSFingerprintCaptureSessionEvent) (*service.TLSFingerprintCaptureSessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	created := cloneTLSFingerprintCaptureHandlerSessionEvent(event)
	created.ID = r.nextEventID
	r.nextEventID++
	created.CreatedAt = time.Now().UTC()
	r.sessionEvents = append(r.sessionEvents, created)
	return cloneTLSFingerprintCaptureHandlerSessionEvent(created), nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) GetSampleByTaskHash(_ context.Context, taskID int64, fingerprintHash string) (*service.TLSFingerprintCaptureSample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sample := range r.samples {
		if sample.TaskID == taskID && sample.FingerprintHash == fingerprintHash {
			return cloneTLSFingerprintCaptureHandlerSample(sample), nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintCaptureHandlerRepoStub) ListSamplesByTask(_ context.Context, taskID int64) ([]*service.TLSFingerprintCaptureSample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*service.TLSFingerprintCaptureSample, 0, len(r.samples))
	for _, sample := range r.samples {
		if sample.TaskID == taskID {
			out = append(out, cloneTLSFingerprintCaptureHandlerSample(sample))
		}
	}
	return out, nil
}

func cloneTLSFingerprintCaptureHandlerTask(task *service.TLSFingerprintCaptureTask) *service.TLSFingerprintCaptureTask {
	if task == nil {
		return nil
	}
	clone := *task
	clone.Targets = cloneStringIntMap(task.Targets)
	clone.Counts = cloneStringIntMap(task.Counts)
	clone.TransportTargets = cloneStringIntMap(task.TransportTargets)
	clone.TransportCounts = cloneStringIntMap(task.TransportCounts)
	clone.CaptureFilters = cloneStringAnyMapHandler(task.CaptureFilters)
	clone.TaskStats = cloneStringAnyMapHandler(task.TaskStats)
	clone.UAKeywords = append([]string(nil), task.UAKeywords...)
	clone.SampleSchemaVersion = task.SampleSchemaVersion
	if task.CompletedAt != nil {
		completedAt := *task.CompletedAt
		clone.CompletedAt = &completedAt
	}
	return &clone
}

func cloneTLSFingerprintCaptureHandlerSample(sample *service.TLSFingerprintCaptureSample) *service.TLSFingerprintCaptureSample {
	if sample == nil {
		return nil
	}
	clone := *sample
	if sample.Profile != nil {
		profile := *sample.Profile
		profile.CipherSuites = append([]uint16(nil), sample.Profile.CipherSuites...)
		profile.Curves = append([]uint16(nil), sample.Profile.Curves...)
		profile.PointFormats = append([]uint16(nil), sample.Profile.PointFormats...)
		profile.SignatureAlgorithms = append([]uint16(nil), sample.Profile.SignatureAlgorithms...)
		profile.SignatureAlgorithmsCert = append([]uint16(nil), sample.Profile.SignatureAlgorithmsCert...)
		profile.ALPNProtocols = append([]string(nil), sample.Profile.ALPNProtocols...)
		profile.SupportedVersions = append([]uint16(nil), sample.Profile.SupportedVersions...)
		profile.KeyShareGroups = append([]uint16(nil), sample.Profile.KeyShareGroups...)
		profile.PSKModes = append([]uint16(nil), sample.Profile.PSKModes...)
		profile.Extensions = append([]uint16(nil), sample.Profile.Extensions...)
		profile.ExtensionPayloads = cloneUint16BytesMapHandler(sample.Profile.ExtensionPayloads)
		profile.CompressCertAlgos = append([]uint16(nil), sample.Profile.CompressCertAlgos...)
		profile.DelegatedCredentialsAlgorithms = append([]uint16(nil), sample.Profile.DelegatedCredentialsAlgorithms...)
		profile.ApplicationSettingsProtocols = append([]string(nil), sample.Profile.ApplicationSettingsProtocols...)
		clone.Profile = &profile
	}
	clone.StainlessMetadata = cloneStringAnyMapHandler(sample.StainlessMetadata)
	clone.RawClientHello = append([]byte(nil), sample.RawClientHello...)
	return &clone
}

func cloneStringIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneTLSFingerprintCaptureHandlerSession(session *service.TLSFingerprintCaptureSession) *service.TLSFingerprintCaptureSession {
	if session == nil {
		return nil
	}
	clone := *session
	clone.RawClientHello = append([]byte(nil), session.RawClientHello...)
	clone.ObservedClientHello = cloneStringAnyMapHandler(session.ObservedClientHello)
	clone.ReplayProfile = cloneStringAnyMapHandler(session.ReplayProfile)
	clone.DerivedFingerprint = cloneStringAnyMapHandler(session.DerivedFingerprint)
	if session.ClosedAt != nil {
		closed := *session.ClosedAt
		clone.ClosedAt = &closed
	}
	return &clone
}

func cloneTLSFingerprintCaptureHandlerSessionEvent(event *service.TLSFingerprintCaptureSessionEvent) *service.TLSFingerprintCaptureSessionEvent {
	if event == nil {
		return nil
	}
	clone := *event
	clone.StainlessMetadata = cloneStringAnyMapHandler(event.StainlessMetadata)
	clone.HeadersSnapshot = cloneStringAnyMapHandler(event.HeadersSnapshot)
	if event.SampleID != nil {
		sampleID := *event.SampleID
		clone.SampleID = &sampleID
	}
	return &clone
}

func cloneStringAnyMapHandler(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneUint16BytesMapHandler(in map[uint16][]byte) map[uint16][]byte {
	if len(in) == 0 {
		return map[uint16][]byte{}
	}
	out := make(map[uint16][]byte, len(in))
	for k, v := range in {
		out[k] = append([]byte(nil), v...)
	}
	return out
}

func nativeCaptureHandlerClientHello(t *testing.T) []byte {
	t.Helper()

	uconn := utls.UClient(&net.TCPConn{}, &utls.Config{ServerName: "cloud.example"}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(&utls.ClientHelloSpec{
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
			&utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}},
			&utls.SupportedVersionsExtension{Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}},
		},
		TLSVersMin: utls.VersionTLS12,
		TLSVersMax: utls.VersionTLS13,
	}))
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
