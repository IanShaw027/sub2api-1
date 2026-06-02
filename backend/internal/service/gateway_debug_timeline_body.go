package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// gatewayDebugTimelineRedactedKeys lists JSON object keys whose values must be
// scrubbed before a body is written to the debug timeline. Matching is
// case-insensitive on the key string.
var gatewayDebugTimelineRedactedKeys = map[string]struct{}{
	"access_token":    {},
	"accesstoken":     {},
	"refresh_token":   {},
	"refreshtoken":    {},
	"api_key":         {},
	"apikey":          {},
	"client_secret":   {},
	"clientsecret":    {},
	"session_id":      {},
	"sessionid":       {},
	"session_key":     {},
	"sessionkey":      {},
	"authorization":   {},
	"x-api-key":       {},
	"cookie":          {},
	"set-cookie":      {},
	"password":        {},
	"secret":          {},
	"code":            {},
	"code_verifier":   {},
	"code_challenge":  {},
	"x-amzn-kiro-jwt": {},
}

// RecordGatewayDebugTimelineBody emits a debug-timeline event that includes a
// captured request/response body. Honours the IncludeBody / BodyMaxKB settings;
// when capture is disabled (or the body is empty) it still records a
// metadata-only event so the timeline preserves an entry for that stage.
func RecordGatewayDebugTimelineBody(
	settingService *SettingService,
	c *gin.Context,
	stage string,
	body []byte,
	contentType string,
	extra map[string]any,
) {
	settings := resolveGatewayDebugTimelineSettingsFromContext(c, settingService)
	if !settings.Enabled {
		return
	}

	fields := make(map[string]any, len(extra)+6)
	for k, v := range extra {
		fields[k] = v
	}
	fields["body_bytes"] = len(body)
	if trimmed := strings.TrimSpace(contentType); trimmed != "" {
		fields["content_type"] = trimmed
	}

	if settings.IncludeBody && len(body) > 0 {
		redacted := redactGatewayDebugTimelineBody(string(body))
		captured, truncated := truncateGatewayDebugTimelineBody([]byte(redacted), settings.BodyMaxKB)
		fields["body"] = captured
		if truncated {
			fields["body_truncated"] = true
		}
	} else if !settings.IncludeBody {
		fields["body_capture"] = "disabled"
	}

	WriteGatewayDebugTimelineEvent(settingService, c, stage, fields)
}

func resolveGatewayDebugTimelineSettingsFromContext(c *gin.Context, settingService *SettingService) GatewayDebugTimelineSettings {
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	return ResolveGatewayDebugTimelineSettings(ctx, settingService)
}

// truncateGatewayDebugTimelineBody returns a UTF-8 safe prefix bounded by
// maxKB kilobytes (0 disables truncation) and a flag indicating whether the
// content was cut.
func truncateGatewayDebugTimelineBody(body []byte, maxKB int) (string, bool) {
	if maxKB <= 0 {
		return string(body), false
	}
	limit := maxKB * 1024
	if len(body) <= limit {
		return string(body), false
	}
	return utf8SafePrefix(string(body), limit), true
}

// redactGatewayDebugTimelineBody scrubs sensitive values inside a JSON-shaped
// body. If the body is not valid JSON it is returned unchanged so the operator
// can still inspect non-JSON payloads.
func redactGatewayDebugTimelineBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return body
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return body
	}
	redactGatewayDebugTimelineValue(parsed)
	rendered, err := json.Marshal(parsed)
	if err != nil {
		return body
	}
	return string(rendered)
}

func redactGatewayDebugTimelineValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, redact := gatewayDebugTimelineRedactedKeys[strings.ToLower(strings.TrimSpace(key))]; redact {
				typed[key] = "[REDACTED]"
				continue
			}
			redactGatewayDebugTimelineValue(child)
		}
	case []any:
		for _, child := range typed {
			redactGatewayDebugTimelineValue(child)
		}
	}
}

// KiroFrameAggregator buffers reconstructed Kiro frame summaries so a single
// upstream_response_body event can be emitted at the end of the streaming
// request. When debug capture is disabled the aggregator is a no-op.
type KiroFrameAggregator struct {
	mu             sync.Mutex
	settingService *SettingService
	settings       GatewayDebugTimelineSettings
	enabled        bool
	c              *gin.Context
	maxFrames      int
	maxBodyBytes   int
	frames         []map[string]any
	textBuilder    strings.Builder
	textBytes      int
	textTruncated  bool
	frameCount     int
	overflowFrames int
}

// BeginKiroFrameAggregator inspects the timeline settings and returns an
// aggregator. Callers can always invoke methods safely even when capture is
// disabled (they become no-ops).
func BeginKiroFrameAggregator(settingService *SettingService, c *gin.Context) *KiroFrameAggregator {
	settings := resolveGatewayDebugTimelineSettingsFromContext(c, settingService)
	enabled := settings.Enabled && settings.IncludeBody
	maxFrames := 256
	maxBodyBytes := settings.BodyMaxKB * 1024
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultGatewayDebugTimelineBodyMaxKB * 1024
	}
	return &KiroFrameAggregator{
		settingService: settingService,
		settings:       settings,
		enabled:        enabled,
		c:              c,
		maxFrames:      maxFrames,
		maxBodyBytes:   maxBodyBytes,
	}
}

// Append records a single Kiro frame. Only EventType + payload key list are
// retained; payload values are intentionally dropped to avoid leaking content
// outside the bounded text builder.
func (agg *KiroFrameAggregator) Append(frame *kiroFrame, assistantText string) {
	if agg == nil || !agg.enabled || frame == nil {
		return
	}
	agg.mu.Lock()
	defer agg.mu.Unlock()
	agg.frameCount++
	if len(agg.frames) >= agg.maxFrames {
		agg.overflowFrames++
		return
	}
	keys := make([]string, 0, len(frame.Payload))
	for key := range frame.Payload {
		keys = append(keys, key)
	}
	entry := map[string]any{
		"event_type":   frame.EventType,
		"message_type": frame.MessageType,
		"payload_keys": keys,
	}
	agg.frames = append(agg.frames, entry)

	if assistantText == "" {
		return
	}
	if agg.textTruncated {
		return
	}
	remaining := agg.maxBodyBytes - agg.textBytes
	if remaining <= 0 {
		agg.textTruncated = true
		return
	}
	if len(assistantText) > remaining {
		safe := utf8SafePrefix(assistantText, remaining)
		agg.textBuilder.WriteString(safe)
		agg.textBytes += len(safe)
		agg.textTruncated = true
		return
	}
	agg.textBuilder.WriteString(assistantText)
	agg.textBytes += len(assistantText)
}

// Finalize flushes the aggregated frame summary to the timeline. It is safe to
// call multiple times; subsequent calls are no-ops.
func (agg *KiroFrameAggregator) Finalize(stage string, extra map[string]any) {
	if agg == nil {
		return
	}
	agg.mu.Lock()
	defer agg.mu.Unlock()
	if !agg.enabled {
		// Always emit a metadata-only event so the timeline records that the
		// upstream stream completed even when bodies are off — but only when
		// the timeline is actually enabled. Otherwise stay silent so callers
		// without a real gin context (tests) don't trip nil-pointer paths.
		if !agg.settings.Enabled {
			return
		}
		fields := make(map[string]any, len(extra)+1)
		for k, v := range extra {
			fields[k] = v
		}
		fields["body_capture"] = "disabled"
		WriteGatewayDebugTimelineEvent(agg.settingService, agg.c, stage, fields)
		agg.enabled = false
		return
	}
	fields := make(map[string]any, len(extra)+5)
	for k, v := range extra {
		fields[k] = v
	}
	fields["frame_count"] = agg.frameCount
	if agg.overflowFrames > 0 {
		fields["frames_dropped"] = agg.overflowFrames
	}
	fields["frames"] = agg.frames
	if agg.textBuilder.Len() > 0 {
		fields["assistant_text"] = redactGatewayDebugTimelineBody(agg.textBuilder.String())
		fields["assistant_text_bytes"] = agg.textBuilder.Len()
		if agg.textTruncated {
			fields["assistant_text_truncated"] = true
		}
	}
	WriteGatewayDebugTimelineEvent(agg.settingService, agg.c, stage, fields)
	agg.enabled = false
}
