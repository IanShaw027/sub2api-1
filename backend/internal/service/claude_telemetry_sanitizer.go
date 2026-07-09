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
	Platform          string
	Arch              string
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
	out, _ := SanitizeClaudeOAuthBodyWithWorkDirRewrite(body, profile)
	return out
}

func SanitizeClaudeOAuthBodyWithWorkDirRewrite(body []byte, profile *AccountEnvProfile) ([]byte, *WorkDirRewrite) {
	if len(body) == 0 || !json.Valid(body) {
		return body, nil
	}
	leakStripped := body
	for _, p := range claudeLeakFieldPaths {
		leakStripped = safeDeleteJSONKey(leakStripped, p)
	}
	// 请求体在实际 API 调用路径上，不可丢弃；但 workdir 伪装重写若破坏 JSON，必须退回到
	// “已删除泄露字段但未做 workdir 伪装”的版本，而非退回原始 body（否则 baseUrl/gateway 等
	// 泄露字段会重新出现并外发）。leakStripped 由 safeDeleteJSONKey 作用于合法 JSON，仍合法。
	rewritten, workDirRewrite := RewriteSystemReminderEnvBlocksWithWorkDirRewrite(leakStripped, profile)
	if !json.Valid(rewritten) {
		return leakStripped, nil
	}
	return rewritten, workDirRewrite
}

// SanitizeClaudeTelemetryBatch rewrites/sanitizes Claude Code event batch payloads.
// 返回 (cleaned, ok)：ok=false 表示无法保证脱敏（输入非法 JSON 或改写后破坏 JSON）。
// 该转发/记录路径的唯一目的就是防泄露，故此处 fail-closed —— 调用方必须在 ok=false 时
// 丢弃、绝不回退转发未脱敏的原始 body（否则 baseUrl/gateway/env 会外发到 Anthropic）。
func SanitizeClaudeTelemetryBatch(body []byte, opts ClaudeTelemetrySanitizeOptions) ([]byte, bool) {
	if len(body) == 0 || !json.Valid(body) {
		return nil, false
	}
	out := body
	for _, p := range claudeLeakFieldPaths {
		out = safeDeleteJSONKey(out, p)
	}
	events := gjson.GetBytes(out, "events")
	if !events.IsArray() {
		return out, true
	}
	for i := range events.Array() {
		base := fmt.Sprintf("events.%d.event_data", i)
		if !gjson.GetBytes(out, base).Exists() {
			continue
		}
		if gjson.GetBytes(out, base+".device_id").Exists() {
			if opts.DeviceID != "" {
				out, _ = sjson.SetBytes(out, base+".device_id", opts.DeviceID)
			} else {
				out = safeDeleteJSONKey(out, base+".device_id")
			}
		}
		if gjson.GetBytes(out, base+".email").Exists() {
			if opts.Email != "" {
				out, _ = sjson.SetBytes(out, base+".email", opts.Email)
			} else {
				out = safeDeleteJSONKey(out, base+".email")
			}
		}
		if gjson.GetBytes(out, base+".env").Exists() {
			if opts.CanonicalEnv != nil {
				out, _ = sjson.SetBytes(out, base+".env", opts.CanonicalEnv)
			} else {
				out = safeDeleteJSONKey(out, base+".env")
			}
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
		return nil, false
	}
	return out, true
}

func sanitizeClaudeTelemetryProcess(body []byte, path string, opts ClaudeTelemetrySanitizeOptions) []byte {
	// 剥离会泄露真实部署路径/启动命令的进程字段：这些与 WorkDirRewrite 的伪装目标一致，
	// 若原样外发即暴露真实环境（sanitizeClaudeTelemetryProcess 此前只改内存数值、遗漏了它们）。
	for _, leak := range []string{"cwd", "argv", "argv0", "execPath", "execArgv", "title", "ppid", "version"} {
		body = safeDeleteJSONKey(body, path+"."+leak)
	}
	// process.platform/arch 与 env.platform/arch 必须一致：env 被整体改写成 profile 平台后，
	// process 里若保留真实 OS（如 env=linux vs process=darwin）即自相矛盾且泄露真实系统。
	// 仅在字段已存在时同步（不给原本不上报的客户端凭空添加，避免新增可识别特征）。
	if opts.Platform != "" && gjson.GetBytes(body, path+".platform").Exists() {
		body, _ = sjson.SetBytes(body, path+".platform", opts.Platform)
	}
	if opts.Arch != "" && gjson.GetBytes(body, path+".arch").Exists() {
		body, _ = sjson.SetBytes(body, path+".arch", opts.Arch)
	}
	if opts.ConstrainedMemory > 0 {
		body, _ = sjson.SetBytes(body, path+".constrainedMemory", opts.ConstrainedMemory)
	}
	// 内存三段必须满足物理约束 heapUsed <= heapTotal <= rss，否则任何 sanity check 一眼
	// 识别为合成快照（此前三段独立随机，配置区间重叠时会产出 heapUsed>heapTotal）。
	// 先定 rss，再把 heapTotal 收敛到 <=rss，最后把 heapUsed 收敛到 <=heapTotal。
	rss, hasRSS := chooseRangeValue(opts.RSSRange)
	if hasRSS {
		body, _ = sjson.SetBytes(body, path+".rss", rss)
	}
	heapTotal, hasHeapTotal := chooseRangeValue(opts.HeapTotalRange)
	if hasHeapTotal {
		if hasRSS && heapTotal > rss {
			heapTotal = rss
		}
		body, _ = sjson.SetBytes(body, path+".heapTotal", heapTotal)
	}
	if heapUsed, ok := chooseRangeValue(opts.HeapUsedRange); ok {
		if hasHeapTotal && heapUsed > heapTotal {
			heapUsed = heapTotal
		}
		body, _ = sjson.SetBytes(body, path+".heapUsed", heapUsed)
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
	out, _ := RewriteSystemReminderEnvBlocksWithWorkDirRewrite(body, profile)
	return out
}

func RewriteSystemReminderEnvBlocksWithWorkDirRewrite(body []byte, profile *AccountEnvProfile) ([]byte, *WorkDirRewrite) {
	if profile == nil || len(body) == 0 || !json.Valid(body) {
		return body, nil
	}
	out := body
	var workDirRewrite *WorkDirRewrite
	msgs := gjson.GetBytes(out, "messages")
	if !msgs.IsArray() {
		return out, nil
	}
	for i := range msgs.Array() {
		contentPath := fmt.Sprintf("messages.%d.content", i)
		content := gjson.GetBytes(out, contentPath)
		if content.Type == gjson.String {
			cleaned, wdr := rewriteSystemReminderTextWithWorkDirRewrite(content.String(), profile)
			if workDirRewrite == nil && wdr != nil {
				workDirRewrite = wdr
			}
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
			cleaned, wdr := rewriteSystemReminderTextWithWorkDirRewrite(text.String(), profile)
			if workDirRewrite == nil && wdr != nil {
				workDirRewrite = wdr
			}
			if cleaned != text.String() {
				out, _ = sjson.SetBytes(out, textPath, cleaned)
			}
		}
	}
	if !json.Valid(out) {
		return body, nil
	}
	return out, workDirRewrite
}

func rewriteSystemReminderTextWithWorkDirRewrite(text string, profile *AccountEnvProfile) (string, *WorkDirRewrite) {
	var workDirRewrite *WorkDirRewrite
	out := systemReminderBlockRe.ReplaceAllStringFunc(text, func(block string) string {
		rewritten, wdr := rewriteReminderEnvBlockWithWorkDirRewrite(block, profile)
		if workDirRewrite == nil && wdr != nil {
			workDirRewrite = wdr
		}
		return rewritten
	})
	return out, workDirRewrite
}

func rewriteReminderEnvBlockWithWorkDirRewrite(block string, profile *AccountEnvProfile) (string, *WorkDirRewrite) {
	if profile == nil {
		return block, nil
	}
	var workDirRewrite *WorkDirRewrite
	workDirRe := regexp.MustCompile(`(?m)((?:Primary )?[Ww]orking directory:\s*)([^\n<]+)`)
	if match := workDirRe.FindStringSubmatch(block); len(match) == 3 {
		realDir := strings.TrimSpace(match[2])
		fakeDir := profileWorkDirForRealDir(profile, realDir)
		if realDir != "" && fakeDir != "" && realDir != fakeDir {
			workDirRewrite = &WorkDirRewrite{RealDir: realDir, FakeDir: fakeDir}
		}
	}
	replacements := []struct {
		re *regexp.Regexp
		to string
	}{
		{regexp.MustCompile(`(?m)(Platform:\s*)[^\n<]+`), "${1}" + profile.Platform},
		{regexp.MustCompile(`(?m)(Shell:\s*)[^\n<]+`), "${1}" + profile.Shell},
		{regexp.MustCompile(`(?m)(OS Version:\s*)[^\n<]+`), "${1}" + profile.OSVersion},
		{workDirRe, "${1}" + func() string {
			if workDirRewrite != nil {
				return workDirRewrite.FakeDir
			}
			return profile.WorkDir
		}()},
	}
	out := block
	for _, r := range replacements {
		if strings.TrimSpace(r.to) == "${1}" {
			continue
		}
		out = r.re.ReplaceAllString(out, r.to)
	}
	return out, workDirRewrite
}
