package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestWriteGatewayDebugTimelineEventRecreatesDeletedDirectory(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)

	dir := filepath.Join(t.TempDir(), "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)

	WriteGatewayDebugTimelineEvent(settingService, nil, "before_delete", map[string]any{"component": "test"})
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove timeline dir: %v", err)
	}

	WriteGatewayDebugTimelineEvent(settingService, nil, "after_delete", map[string]any{"component": "test"})

	content := readGatewayDebugTimelineLog(t, dir)
	if !strings.Contains(content, `"stage":"after_delete"`) {
		t.Fatalf("expected recreated log to contain stage, got %s", content)
	}
}

func TestWriteGatewayDebugTimelineEventRecoversAfterInitFailure(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)

	tempDir := t.TempDir()
	blockingFile := filepath.Join(tempDir, "not-a-dir")
	if err := os.WriteFile(blockingFile, []byte("block"), 0o644); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}
	dir := filepath.Join(blockingFile, "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)
	WriteGatewayDebugTimelineEvent(settingService, nil, "blocked", map[string]any{"component": "test"})
	if err := os.Remove(blockingFile); err != nil {
		t.Fatalf("remove blocking file: %v", err)
	}

	WriteGatewayDebugTimelineEvent(settingService, nil, "after_recover", map[string]any{"component": "test"})

	content := readGatewayDebugTimelineLog(t, dir)
	if !strings.Contains(content, `"stage":"after_recover"`) {
		t.Fatalf("expected recovered log to contain stage, got %s", content)
	}
}

func TestWriteGatewayDebugTimelineEventCleansOldFilesWhenSizeLimitReached(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)

	dir := filepath.Join(t.TempDir(), "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)
	repo := settingService.settingRepo
	if repo == nil {
		t.Fatal("expected setting repo")
	}
	if err := repo.SetMultiple(context.TODO(), map[string]string{
		SettingKeyGatewayDebugTimelineMaxSizeMB: "1",
	}); err != nil {
		t.Fatalf("seed gateway debug timeline size setting: %v", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir timeline dir: %v", err)
	}
	oldestGzipName := gatewayDebugTimelineFilenamePrefix + time.Now().AddDate(0, 0, -3).Format("2006-01-02") + ".log.gz"
	olderLogName := gatewayDebugTimelineFilenamePrefix + time.Now().AddDate(0, 0, -2).Format("2006-01-02") + ".log"
	newerLogName := gatewayDebugTimelineFilenamePrefix + time.Now().AddDate(0, 0, -1).Format("2006-01-02") + ".log"
	writeGatewayDebugTimelineSizedFile(t, dir, oldestGzipName, 200*1024)
	writeGatewayDebugTimelineSizedFile(t, dir, olderLogName, 700*1024)
	writeGatewayDebugTimelineSizedFile(t, dir, newerLogName, 400*1024)

	WriteGatewayDebugTimelineEvent(settingService, nil, "after_cleanup", map[string]any{"component": "test"})
	WriteGatewayDebugTimelineEvent(settingService, nil, "follow_up", map[string]any{"component": "test"})

	content := readGatewayDebugTimelineLog(t, dir)
	if !strings.Contains(content, `"stage":"after_cleanup"`) {
		t.Fatalf("expected size-pressure cleanup to preserve current write, got %s", content)
	}
	if !strings.Contains(content, `"stage":"follow_up"`) {
		t.Fatalf("expected timeline to remain writable after cleanup, got %s", content)
	}
	if _, err := os.Stat(filepath.Join(dir, oldestGzipName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected oldest gzip file to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, olderLogName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected older log file to be removed after gzip cleanup was insufficient, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, newerLogName)); err != nil {
		t.Fatalf("expected newer file to remain after cleanup, got err=%v", err)
	}

	gatewayDebugTimelineState.Lock()
	disabled := gatewayDebugTimelineState.disabled
	gatewayDebugTimelineState.Unlock()
	if disabled {
		t.Fatal("expected timeline writer to stay enabled after cleanup")
	}
}

func TestRecordGatewayDebugTimelineBody_RedactsOversizedSensitiveJSON(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)

	dir := filepath.Join(t.TempDir(), "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)
	repo := settingService.settingRepo
	if repo == nil {
		t.Fatal("expected setting repo")
	}
	if err := repo.SetMultiple(context.TODO(), map[string]string{
		SettingKeyGatewayDebugTimelineIncludeBody: "true",
		SettingKeyGatewayDebugTimelineBodyMaxKB:   "1",
	}); err != nil {
		t.Fatalf("seed gateway debug timeline body settings: %v", err)
	}
	gatewayDebugTimelineSettingsSF.Forget("gateway_debug_timeline")
	gatewayDebugTimelineSettingsCache.Store(&cachedGatewayDebugTimelineSettings{
		settings:  DefaultGatewayDebugTimelineSettings(),
		expiresAt: 0,
	})

	body := []byte(`{"access_token":"secret-token","payload":"` + strings.Repeat("x", 4096) + `"}`)
	RecordGatewayDebugTimelineBody(settingService, nil, "request_received", body, "application/json", map[string]any{
		"component": "test",
	})

	content := readGatewayDebugTimelineLog(t, dir)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 {
		t.Fatal("expected timeline log line")
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
		t.Fatalf("unmarshal timeline event: %v", err)
	}
	bodyValue, _ := event["body"].(string)
	if strings.Contains(bodyValue, "secret-token") {
		t.Fatalf("expected redacted body, got %q", bodyValue)
	}
	if !strings.Contains(bodyValue, "[REDACTED]") {
		t.Fatalf("expected redacted marker in body, got %q", bodyValue)
	}
}

func TestOpenAIGatewayServiceEmitDebugTimelineEvent_WritesOpenAIMetadata(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)

	dir := filepath.Join(t.TempDir(), "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)
	svc := &OpenAIGatewayService{settingService: settingService}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`))

	groupID := int64(22)
	apiKey := &APIKey{ID: 200, GroupID: &groupID}
	account := &Account{
		ID:          67225,
		Name:        "MasonDobies01@outlook.com",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 64,
	}
	firstTokenMs := 1234
	result := &ForwardResult{
		RequestID:     "resp_timeline_1",
		UpstreamModel: "gpt-5.4",
		FirstTokenMs:  &firstTokenMs,
		Usage: ClaudeUsage{
			InputTokens:          100,
			CacheReadInputTokens: 90,
			OutputTokens:         10,
		},
	}

	svc.EmitOpenAIGatewayDebugTimelineEvent(c, OpenAIGatewayDebugTimelineEventInput{
		Stage:             "attempt_finished",
		EndpointKind:      "responses",
		RequestStart:      time.Now().Add(-1500 * time.Millisecond),
		APIKey:            apiKey,
		Account:           account,
		RequestedModel:    "gpt-5.4",
		Stream:            true,
		SwitchCount:       2,
		ForwardDurationMs: 1400,
		Result:            result,
		Fields: map[string]any{
			"openai_ws_mode":        true,
			"openai_ws_profile":     "session_bound",
			"openai_ws_conn_reused": true,
			"transport_path":        "session_ws_incremental_delta",
			"store_mode":            "incremental",
			"delta_active":          true,
		},
	})

	content := readGatewayDebugTimelineLog(t, dir)
	if !strings.Contains(content, `"platform":"openai"`) {
		t.Fatalf("expected openai platform timeline event, got %s", content)
	}
	if !strings.Contains(content, `"stage":"attempt_finished"`) {
		t.Fatalf("expected attempt_finished stage, got %s", content)
	}
	if !strings.Contains(content, `"account_id":67225`) {
		t.Fatalf("expected account metadata, got %s", content)
	}
	if !strings.Contains(content, `"openai_ws_profile":"session_bound"`) {
		t.Fatalf("expected ws metadata, got %s", content)
	}
	if !strings.Contains(content, `"transport_path":"session_ws_incremental_delta"`) {
		t.Fatalf("expected transport path metadata, got %s", content)
	}
	if strings.Contains(content, `"body":`) {
		t.Fatalf("OpenAI timeline should remain metadata-only, got %s", content)
	}
}

func TestOpenAIGatewayServiceEmitDebugTimelineEvent_WritesOpenAIWSResultBreakdown(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)

	dir := filepath.Join(t.TempDir(), "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)
	svc := &OpenAIGatewayService{settingService: settingService}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`))

	account := &Account{ID: 67238, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	result := &OpenAIForwardResult{
		RequestID:            "resp_ws_timeline",
		UpstreamModel:        "gpt-5.4",
		OpenAIWSMode:         true,
		OpenAIWSProfile:      "session_bound",
		OpenAIWSConnReused:   true,
		OpenAIWSStoreMode:    "incremental",
		OpenAIWSDeltaActive:  true,
		OpenAIWSPayloadBytes: 512,
		OpenAIWSDeltaItems:   1,
		OpenAIWSDeltaBytes:   128,
		OpenAIWSFullItems:    42,
		OpenAIWSFullBytes:    8192,
		OpenAIWSConnPickMs:   3,
		OpenAIWSQueueWaitMs:  7,
	}

	svc.EmitOpenAIGatewayDebugTimelineEvent(c, OpenAIGatewayDebugTimelineEventInput{
		Stage:          "attempt_finished",
		EndpointKind:   "responses",
		RequestStart:   time.Now().Add(-1500 * time.Millisecond),
		Account:        account,
		RequestedModel: "gpt-5.4",
		Stream:         true,
		OpenAIResult:   result,
	})

	content := readGatewayDebugTimelineLog(t, dir)
	for _, expected := range []string{
		`"store_mode":"incremental"`,
		`"delta_active":true`,
		`"payload_bytes":512`,
		`"delta_items":1`,
		`"delta_bytes":128`,
		`"full_items":42`,
		`"full_bytes":8192`,
		`"conn_pick_ms":3`,
		`"queue_wait_ms":7`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected %s in timeline event, got %s", expected, content)
		}
	}
}

func TestOpenAIGatewayServiceEmitDebugTimelineEvent_RedactsErrorMetadata(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)

	dir := filepath.Join(t.TempDir(), "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)
	svc := &OpenAIGatewayService{settingService: settingService}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`))

	svc.EmitOpenAIGatewayDebugTimelineEvent(c, OpenAIGatewayDebugTimelineEventInput{
		Stage:        "attempt_finished",
		EndpointKind: "responses",
		Err: errors.New(
			`upstream https://example.invalid/callback?access_token=sk-secret-token&refresh_token=rt-secret failed with Authorization: Bearer sk-proj-secret`,
		),
	})

	content := readGatewayDebugTimelineLog(t, dir)
	if strings.Contains(content, "sk-secret-token") || strings.Contains(content, "rt-secret") || strings.Contains(content, "sk-proj-secret") {
		t.Fatalf("timeline error metadata leaked secret: %s", content)
	}
	if !strings.Contains(content, "[REDACTED]") {
		t.Fatalf("expected redacted marker in error metadata, got %s", content)
	}
}

func testGatewayDebugTimelineSettingService(t *testing.T, dir string) *SettingService {
	t.Helper()
	repo := &gatewayDebugTimelineSettingRepo{values: map[string]string{}}
	if err := repo.SetMultiple(context.TODO(), map[string]string{
		SettingKeyGatewayDebugTimelineEnabled:       "true",
		SettingKeyGatewayDebugTimelineDirectory:     dir,
		SettingKeyGatewayDebugTimelineRetentionDays: "7",
		SettingKeyGatewayDebugTimelineMaxSizeMB:     "1024",
	}); err != nil {
		t.Fatalf("seed gateway debug timeline settings: %v", err)
	}
	return NewSettingService(repo, nil)
}

type gatewayDebugTimelineSettingRepo struct {
	values map[string]string
}

func (r *gatewayDebugTimelineSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *gatewayDebugTimelineSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (r *gatewayDebugTimelineSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *gatewayDebugTimelineSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}

func (r *gatewayDebugTimelineSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *gatewayDebugTimelineSettingRepo) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *gatewayDebugTimelineSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func resetGatewayDebugTimelineStateForTest(t *testing.T) {
	t.Helper()

	gatewayDebugTimelineState.Lock()
	gatewayDebugTimelineState.lastCleanup = time.Time{}
	gatewayDebugTimelineState.disabled = false
	gatewayDebugTimelineState.warnedFull = false
	gatewayDebugTimelineState.initializedDir = ""
	gatewayDebugTimelineState.lastCreateDirErrorLog = time.Time{}
	gatewayDebugTimelineState.Unlock()
	gatewayDebugTimelineSettingsSF.Forget("gateway_debug_timeline")
	gatewayDebugTimelineSettingsCache.Store(&cachedGatewayDebugTimelineSettings{
		settings:  DefaultGatewayDebugTimelineSettings(),
		expiresAt: 0,
	})
}

func writeGatewayDebugTimelineSizedFile(t *testing.T, dir, name string, size int) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
		t.Fatalf("write sized timeline file %s: %v", name, err)
	}
}

func readGatewayDebugTimelineLog(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, gatewayDebugTimelineFilenamePrefix+time.Now().Format("2006-01-02")+".log")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read timeline log: %v", err)
	}
	return string(content)
}
