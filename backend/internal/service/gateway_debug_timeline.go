package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

const gatewayDebugTimelineFilenamePrefix = "gateway-timeline-"

var gatewayDebugTimelineState struct {
	sync.Mutex
	lastCleanup           time.Time
	disabled              bool
	warnedFull            bool
	initializedDir        string
	lastCreateDirErrorLog time.Time
}

func ResetGatewayDebugTimelineAutoStop() {
	gatewayDebugTimelineState.Lock()
	gatewayDebugTimelineState.disabled = false
	gatewayDebugTimelineState.warnedFull = false
	gatewayDebugTimelineState.Unlock()
}

func GatewayDebugTimelineEnabled(ctx context.Context, settingService *SettingService) bool {
	return ResolveGatewayDebugTimelineSettings(ctx, settingService).Enabled
}

func ResolveGatewayDebugTimelineSettings(ctx context.Context, settingService *SettingService) GatewayDebugTimelineSettings {
	if settingService == nil {
		return DefaultGatewayDebugTimelineSettings()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return settingService.GetGatewayDebugTimelineSettings(ctx)
}

func WriteGatewayDebugTimelineEvent(settingService *SettingService, c *gin.Context, stage string, fields map[string]any) {
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	settings := ResolveGatewayDebugTimelineSettings(ctx, settingService)
	if !settings.Enabled {
		return
	}
	stage = strings.TrimSpace(stage)
	if stage == "" {
		return
	}

	dir := gatewayDebugTimelineDir(settings.Directory)
	retentionDays := settings.RetentionDays
	maxSizeBytes := settings.MaxSizeMB * 1024 * 1024

	now := time.Now()
	event := make(map[string]any, len(fields)+12)
	for k, v := range fields {
		event[k] = v
	}
	event["ts"] = now.Format(time.RFC3339Nano)
	event["event_unix_ms"] = now.UnixMilli()
	event["stage"] = stage

	if c != nil {
		if c.Request != nil {
			if requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
				event["request_id"] = strings.TrimSpace(requestID)
			}
			if clientRequestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
				event["client_request_id"] = strings.TrimSpace(clientRequestID)
			}
			if c.Request.URL != nil {
				event["request_path"] = strings.TrimSpace(c.Request.URL.Path)
			}
			event["request_method"] = strings.TrimSpace(c.Request.Method)
		}
		if c.Request != nil {
			if _, ok := event["client_request_id"]; !ok {
				if id := strings.TrimSpace(c.GetHeader("X-Client-Request-Id")); id != "" {
					event["client_request_id"] = id
				}
			}
			event["request_user_agent"] = strings.TrimSpace(c.GetHeader("User-Agent"))
		}
	}

	line, err := json.Marshal(event)
	if err != nil {
		logger.LegacyPrintf("service.gateway_debug_timeline", "marshal event failed: %v", err)
		return
	}

	gatewayDebugTimelineState.Lock()
	defer gatewayDebugTimelineState.Unlock()

	if gatewayDebugTimelineState.disabled {
		return
	}

	if err := ensureGatewayDebugTimelineDirLocked(dir); err != nil {
		if gatewayDebugTimelineState.lastCreateDirErrorLog.IsZero() || now.Sub(gatewayDebugTimelineState.lastCreateDirErrorLog) >= time.Minute {
			gatewayDebugTimelineState.lastCreateDirErrorLog = now
			logger.LegacyPrintf("service.gateway_debug_timeline", "create log dir failed: %v", err)
		}
		return
	}
	cleanupGatewayDebugTimelineLocked(dir, retentionDays, now)
	if maxSizeBytes > 0 {
		sizeBytes := gatewayDebugTimelineDirSize(dir)
		if sizeBytes >= maxSizeBytes {
			gatewayDebugTimelineState.disabled = true
			if !gatewayDebugTimelineState.warnedFull {
				gatewayDebugTimelineState.warnedFull = true
				logger.LegacyPrintf(
					"service.gateway_debug_timeline",
					"Gateway debug timeline disabled: directory size limit reached size_bytes=%d max_size_bytes=%d dir=%s",
					sizeBytes,
					maxSizeBytes,
					dir,
				)
			}
			return
		}
	}

	filename := gatewayDebugTimelineFilenamePrefix + now.Format("2006-01-02") + ".log"
	path := filepath.Join(dir, filename)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logger.LegacyPrintf("service.gateway_debug_timeline", "open log file failed: %v", err)
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(line, '\n'))
}

func gatewayDebugTimelineDir(configured string) string {
	dir := strings.TrimSpace(configured)
	if dir == "" {
		dir = defaultGatewayDebugTimelineDirectory
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return absDir
}

func ensureGatewayDebugTimelineDirLocked(dir string) error {
	if gatewayDebugTimelineState.initializedDir == dir {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return nil
		}
		gatewayDebugTimelineState.initializedDir = ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	gatewayDebugTimelineState.initializedDir = dir
	gatewayDebugTimelineState.lastCreateDirErrorLog = time.Time{}
	return nil
}

func gatewayDebugTimelineDirSize(dir string) int64 {
	var total int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, gatewayDebugTimelineFilenamePrefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		total += info.Size()
	}
	return total
}

func cleanupGatewayDebugTimelineLocked(dir string, retentionDays int, now time.Time) {
	if retentionDays <= 0 {
		return
	}
	if !gatewayDebugTimelineState.lastCleanup.IsZero() && now.Sub(gatewayDebugTimelineState.lastCleanup) < time.Hour {
		return
	}
	gatewayDebugTimelineState.lastCleanup = now

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := now.AddDate(0, 0, -retentionDays)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, gatewayDebugTimelineFilenamePrefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		datePart := strings.TrimSuffix(strings.TrimPrefix(name, gatewayDebugTimelineFilenamePrefix), ".log")
		day, err := time.ParseInLocation("2006-01-02", datePart, now.Location())
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}
