package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func readGatewayDebugTimelineLog(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, gatewayDebugTimelineFilenamePrefix+time.Now().Format("2006-01-02")+".log")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read timeline log: %v", err)
	}
	return string(content)
}
