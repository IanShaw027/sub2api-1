package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ClaudeTelemetrySanitizeOptions controls optional identity/env/process rewrites for
// Claude Code telemetry payloads. Empty fields are ignored; leak fields are always removed.
type ClaudeTelemetrySanitizeOptions struct {
	DeviceID          string
	Email             string
	CanonicalEnv      map[string]any
	ConstrainedMemory int64
	RSSRange          [2]int64
	HeapTotalRange    [2]int64
	HeapUsedRange     [2]int64
}

var systemReminderBlockRe = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)

var claudeLeakFieldPaths = []string{
	"baseUrl",
	"base_url",
	"gateway",
	"metadata.baseUrl",
	"metadata.base_url",
	"metadata.gateway",
	"client_metadata",
	"client_metadata.process",
	"client_metadata.env",
	"client_metadata.baseUrl",
	"client_metadata.base_url",
	"client_metadata.gateway",
}

// SanitizeClaudeOAuthBody applies always-on Claude OAuth request safety cleanup.
// It removes structured proxy/client-metadata leak fields and, when a profile is
// available, rewrites environment details inside Claude Code system-reminder blocks.
func SanitizeClaudeOAuthBody(body []byte, profile *AccountEnvProfile) []byte {
	if len(body) == 0 || !json.Valid(body) {
		return body
	}
	out := body
	for _, p := range claudeLeakFieldPaths {
		out = safeDeleteJSONKey(out, p)
	}
	out = RewriteSystemReminderEnvBlocks(out, profile)
	if !json.Valid(out) {
		return body
	}
	return out
}

// SanitizeClaudeTelemetryBatch rewrites/sanitizes Claude Code event batch payloads.
// It is safe to call even when the caller ultimately drops the telemetry request.
func SanitizeClaudeTelemetryBatch(body []byte, opts ClaudeTelemetrySanitizeOptions) []byte {
	if len(body) == 0 || !json.Valid(body) {
		return body
	}
	out := body
	for _, p := range claudeLeakFieldPaths {
		out = safeDeleteJSONKey(out, p)
	}
	events := gjson.GetBytes(out, "events")
	if !events.IsArray() {
		return out
	}
	for i := range events.Array() {
		base := fmt.Sprintf("events.%d.event_data", i)
		if !gjson.GetBytes(out, base).Exists() {
			continue
		}
		if opts.DeviceID != "" && gjson.GetBytes(out, base+".device_id").Exists() {
			out, _ = sjson.SetBytes(out, base+".device_id", opts.DeviceID)
		}
		if opts.Email != "" && gjson.GetBytes(out, base+".email").Exists() {
			out, _ = sjson.SetBytes(out, base+".email", opts.Email)
		}
		if opts.CanonicalEnv != nil && gjson.GetBytes(out, base+".env").Exists() {
			out, _ = sjson.SetBytes(out, base+".env", opts.CanonicalEnv)
		}
		if gjson.GetBytes(out, base+".process").Exists() {
			out = sanitizeClaudeTelemetryProcess(out, base+".process", opts)
		}
		for _, p := range claudeLeakFieldPaths {
			out = safeDeleteJSONKey(out, base+"."+p)
		}
		if v := gjson.GetBytes(out, base+".additional_metadata"); v.Exists() && v.Type == gjson.String {
			if rewritten, ok := sanitizeBase64JSONLeakMetadata(v.String()); ok {
				out, _ = sjson.SetBytes(out, base+".additional_metadata", rewritten)
			}
		}
	}
	if !json.Valid(out) {
		return body
	}
	return out
}

func sanitizeClaudeTelemetryProcess(body []byte, path string, opts ClaudeTelemetrySanitizeOptions) []byte {
	if opts.ConstrainedMemory > 0 {
		body, _ = sjson.SetBytes(body, path+".constrainedMemory", opts.ConstrainedMemory)
	}
	if v, ok := chooseRangeValue(opts.RSSRange); ok {
		body, _ = sjson.SetBytes(body, path+".rss", v)
	}
	if v, ok := chooseRangeValue(opts.HeapTotalRange); ok {
		body, _ = sjson.SetBytes(body, path+".heapTotal", v)
	}
	if v, ok := chooseRangeValue(opts.HeapUsedRange); ok {
		body, _ = sjson.SetBytes(body, path+".heapUsed", v)
	}
	return body
}

func chooseRangeValue(r [2]int64) (int64, bool) {
	min, max := r[0], r[1]
	if min <= 0 || max <= 0 {
		return 0, false
	}
	if max <= min {
		return min, true
	}
	return min + rand.Int63n(max-min), true
}

func sanitizeBase64JSONLeakMetadata(raw string) (string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || !json.Valid(decoded) {
		return raw, false
	}
	out := decoded
	for _, p := range []string{"baseUrl", "base_url", "gateway"} {
		out = safeDeleteJSONKey(out, p)
	}
	if !json.Valid(out) {
		return raw, false
	}
	return base64.StdEncoding.EncodeToString(out), true
}

// RewriteSystemReminderEnvBlocks rewrites environment fields inside injected
// <system-reminder> blocks only, leaving user-authored text outside unchanged.
func RewriteSystemReminderEnvBlocks(body []byte, profile *AccountEnvProfile) []byte {
	if profile == nil || len(body) == 0 || !json.Valid(body) {
		return body
	}
	out := body
	msgs := gjson.GetBytes(out, "messages")
	if !msgs.IsArray() {
		return out
	}
	for i := range msgs.Array() {
		contentPath := fmt.Sprintf("messages.%d.content", i)
		content := gjson.GetBytes(out, contentPath)
		if content.Type == gjson.String {
			cleaned := rewriteSystemReminderText(content.String(), profile)
			if cleaned != content.String() {
				out, _ = sjson.SetBytes(out, contentPath, cleaned)
			}
			continue
		}
		if !content.IsArray() {
			continue
		}
		for j := range content.Array() {
			textPath := fmt.Sprintf("%s.%d.text", contentPath, j)
			text := gjson.GetBytes(out, textPath)
			if text.Type != gjson.String {
				continue
			}
			cleaned := rewriteSystemReminderText(text.String(), profile)
			if cleaned != text.String() {
				out, _ = sjson.SetBytes(out, textPath, cleaned)
			}
		}
	}
	if !json.Valid(out) {
		return body
	}
	return out
}

func rewriteSystemReminderText(text string, profile *AccountEnvProfile) string {
	return systemReminderBlockRe.ReplaceAllStringFunc(text, func(block string) string {
		return rewriteReminderEnvBlock(block, profile)
	})
}

func rewriteReminderEnvBlock(block string, profile *AccountEnvProfile) string {
	if profile == nil {
		return block
	}
	replacements := []struct {
		re *regexp.Regexp
		to string
	}{
		{regexp.MustCompile(`(?m)(Platform:\s*)[^\n<]+`), "${1}" + profile.Platform},
		{regexp.MustCompile(`(?m)(Shell:\s*)[^\n<]+`), "${1}" + profile.Shell},
		{regexp.MustCompile(`(?m)(OS Version:\s*)[^\n<]+`), "${1}" + profile.OSVersion},
		{regexp.MustCompile(`(?m)((?:Primary )?[Ww]orking directory:\s*)[^\n<]+`), "${1}" + profile.WorkDir},
	}
	out := block
	for _, r := range replacements {
		if strings.TrimSpace(r.to) == "${1}" {
			continue
		}
		out = r.re.ReplaceAllString(out, r.to)
	}
	return out
}
