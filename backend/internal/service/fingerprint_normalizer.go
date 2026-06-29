package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// runtimeAntiBanPlatforms holds DB-driven per-platform toggles (from System Settings).
// Takes precedence over static config when a key is present.
var runtimeAntiBanPlatforms atomic.Value // stores map[string]bool

// SetRuntimeAntiBanPlatforms updates the runtime anti-ban platform map (called by SettingService on load/save).
func SetRuntimeAntiBanPlatforms(m map[string]bool) {
	if m != nil {
		runtimeAntiBanPlatforms.Store(cloneAntiBanPlatforms(m))
	}
}

func getRuntimeAntiBanFor(platform string) (bool, bool) {
	if v := runtimeAntiBanPlatforms.Load(); v != nil {
		if m, ok := v.(map[string]bool); ok {
			if b, exists := m[platform]; exists {
				return b, true
			}
		}
	}
	return false, false
}

// CanonicalFingerprint is the normalized identity used for anti-fingerprinting.
type CanonicalFingerprint struct {
	DeviceID    string
	AccountUUID string
	SessionID   string
	UserAgent   string
	OS          string
	Arch        string
	Runtime     string
	MemoryMB    int
	HeapMB      int
	CPUInfo     string
	Platform    string
}

// CanonicalFingerprintConfig holds config for the normalizer.
type CanonicalFingerprintConfig struct {
	Enabled             bool
	AntiBanEnabled      bool
	StripProxyHeaders   []string
	SpoofProcessMetrics bool
	TelemetryPaths      []string
	JitterMinMs         int
	JitterMaxMs         int
	PlatformProfiles    map[string]PlatformProfile
	// EnabledByPlatform: per-platform override for the whole anti-ban suite.
	// Key must be explicitly true to enable; absent or false means disabled (default OFF).
	EnabledByPlatform map[string]bool
	// TriggerScanEnabled enables third-party harness phrase scanning/removal
	TriggerScanEnabled bool
	// TriggerPhrases is an ordered (by phrase length desc) list of [phrase, replacement] pairs.
	// Longer phrases are matched first to prevent substring shadowing.
	TriggerPhrases [][2]string
	// SchemaPropRewriteEnabled enables tool schema property renaming
	SchemaPropRewriteEnabled bool
	// SchemaPropRewrites is the property name → replacement mapping
	SchemaPropRewrites map[string]string
}

// PlatformProfile per platform overrides.
type PlatformProfile struct {
	CanonicalUA   string
	JitterMinMs   int
	JitterMaxMs   int
	SpoofMemoryMB int
	SpoofHeapMB   int
	CPUInfo       string
	StripExtra    []string
	BodyStripKeys []string
}

// AccountEnvProfile provides per-account deterministic environment/PII values
// for real Claude Code OAuth clients. Derived from accountID + captured
// Stainless fingerprint so that system prompt + X-Stainless-* are consistent,
// while different accounts on same proxy appear as different "users".
type AccountEnvProfile struct {
	Email         string
	GitUser       string
	WorkDir       string
	Platform      string // darwin / linux / win32
	Shell         string // zsh / bash / unknown
	OSVersion     string
	IsGit         bool   // always true for CC
	Arch          string // arm64 / x64
	StainlessOS   string // MacOS / Linux / Windows
	StainlessArch string // arm64 / x64
}

// envProfileFirstNames is the pool for deterministic GitUser selection.
var envProfileFirstNames = []string{
	"Alex", "Jordan", "Sam", "Taylor", "Morgan", "Casey", "Riley",
	"Quinn", "Avery", "Charlie", "Drew", "Emerson", "Finley", "Harper",
	"Jamie", "Kendall", "Logan", "Parker", "Reese", "Sage", "Devon",
	"Blake", "Cameron", "Dakota", "Elliot", "Hayden", "Jesse", "Lane",
}

// envProfileProjectNames is the pool for deterministic working directory names.
var envProfileProjectNames = []string{
	"webapp", "backend", "api-server", "dashboard", "cli-tool",
	"data-pipeline", "ml-service", "docs", "infra", "sdk",
	"platform", "gateway", "worker", "scheduler", "auth-service",
	"config-manager", "monitoring", "deploy-tools", "test-framework", "core",
}

// FingerprintNormalizer applies canonical fingerprint to requests.
type FingerprintNormalizer struct {
	cfg               *CanonicalFingerprintConfig
	tlsProfileSvc     *TLSFingerprintProfileService
	tlsRouterSvc      *TLSFingerprintRouterService
	identitySvc       *IdentityService
	platformFPManager *PlatformFingerprintManager
}

// NewFingerprintNormalizer creates one.
func NewFingerprintNormalizer(
	tlsProfileSvc *TLSFingerprintProfileService,
	tlsRouterSvc *TLSFingerprintRouterService,
	identitySvc *IdentityService,
	cfg *CanonicalFingerprintConfig,
	platformMgr *PlatformFingerprintManager,
) *FingerprintNormalizer {
	if cfg == nil {
		cfg = &CanonicalFingerprintConfig{
			Enabled:             true,
			AntiBanEnabled:      true,
			StripProxyHeaders:   []string{"x-litellm", "helicone", "cf-aig", "x-portkey", "x-forwarded", "via"},
			SpoofProcessMetrics: true,
			TelemetryPaths:      []string{"/telemetry", "datadog", "sentry", "statsig", "segment", "amplitude", "events"},
			JitterMinMs:         10,
			JitterMaxMs:         150,
		}
	}
	return &FingerprintNormalizer{
		cfg:               cfg,
		tlsProfileSvc:     tlsProfileSvc,
		tlsRouterSvc:      tlsRouterSvc,
		identitySvc:       identitySvc,
		platformFPManager: platformMgr,
	}
}

// isAntiBanEnabledFor returns whether the anti-ban suite (fingerprint normalizer etc) is enabled for the platform.
// Global enabled + explicit true in EnabledByPlatform (runtime or cfg). Absent/false = off (default OFF).
func (n *FingerprintNormalizer) isAntiBanEnabledFor(platform string) bool {
	if n == nil || n.cfg == nil || !n.cfg.Enabled || !n.cfg.AntiBanEnabled {
		return false
	}
	platform = normalizePlatform(platform)
	// Runtime (DB settings) wins if the platform key is explicitly present.
	if v, ok := getRuntimeAntiBanFor(platform); ok {
		return v
	}
	if n.cfg.EnabledByPlatform != nil {
		if v, ok := n.cfg.EnabledByPlatform[platform]; ok {
			return v
		}
		// if map present but key absent -> default OFF (per-platform is opt-in; "默认应该是关的")
	}
	return false
}

// ResolveCanonical builds the canonical from account and UA.
// Merges TLS Profile/Router UA (with prior linkage), IdentityService data, and platform fallbacks.
func (n *FingerprintNormalizer) ResolveCanonical(ctx context.Context, account *Account, ua string) *CanonicalFingerprint {
	if account == nil {
		return nil
	}
	if account.Type == AccountTypeAPIKey {
		return nil
	}
	c := &CanonicalFingerprint{
		Platform:  string(account.Platform),
		UserAgent: ua,
	}

	// 1. TLS Router/Profile for effective UA and profile-driven values
	if n.tlsRouterSvc != nil && account.IsTLSFingerprintEnabled() {
		routerID := account.GetTLSFingerprintRouterID()
		if routerID > 0 {
			if match, ok := n.tlsRouterSvc.MatchRequest(ctx, routerID, ua, "http"); ok && match.ProfileID > 0 {
				if p := n.tlsProfileSvc.ResolveTLSProfileByID(match.ProfileID); p != nil {
					if match.UpstreamUserAgent != "" {
						c.UserAgent = match.UpstreamUserAgent
					}
					// TLS profiles primarily provide JA3 etc; UA from router match
				}
			}
		}
	}

	// 2. IdentityService for ClientID / session linkage
	if n.identitySvc != nil {
		if fp, err := n.identitySvc.GetOrCreateFingerprint(ctx, account.ID, http.Header{"User-Agent": {ua}}); err == nil && fp != nil {
			if fp.ClientID != "" && c.DeviceID == "" {
				c.DeviceID = fp.ClientID // use as device basis
			}
			if fp.UserAgent != "" {
				c.UserAgent = fp.UserAgent
			}
		}
	}

	// 3. Account extras for account_uuid, device hints
	if accountUUID := account.GetExtraString("account_uuid"); accountUUID != "" {
		c.AccountUUID = accountUUID
	}
	if deviceHint := account.GetExtraString("device_id"); deviceHint != "" && c.DeviceID == "" {
		c.DeviceID = deviceHint
	}

	// 4. Platform fallbacks and stable session
	if c.DeviceID == "" {
		// Use stable session seed logic adapted for device (consistent with identity)
		seed := buildStableSessionSeed(account.ID, "device", "")
		if len(seed) > 16 {
			seed = seed[:16]
		}
		c.DeviceID = seed // short stable device id
	}
	c.SessionID = buildStableSessionSeed(account.ID, "fp-canonical", "")

	// 5. Pull platform profile overrides — UA only as fallback
	prof := n.getPlatformProfile(c.Platform)
	if prof.CanonicalUA != "" && c.UserAgent == ua {
		// Only override if TLS router / identity service didn't set a specific UA
		c.UserAgent = prof.CanonicalUA
	}
	if prof.CPUInfo != "" {
		c.CPUInfo = prof.CPUInfo
	}
	if prof.SpoofMemoryMB > 0 {
		c.MemoryMB = prof.SpoofMemoryMB
	}
	if prof.SpoofHeapMB > 0 {
		c.HeapMB = prof.SpoofHeapMB
	}

	// OS/Arch/Runtime from UA or profile fallbacks (simplified)
	if c.OS == "" {
		c.OS = "Mac OS X"
	}
	if c.Arch == "" {
		c.Arch = "arm64"
	}
	if c.Runtime == "" {
		c.Runtime = "Node.js"
	}

	return c
}

// StripProxyHeaders removes proxy-telemetry headers from the request.
// Safe to call with nil receiver or nil req.
func (n *FingerprintNormalizer) StripProxyHeaders(req *http.Request, platform string) {
	if n == nil || req == nil || n.cfg == nil {
		return
	}
	platform = normalizePlatform(platform)
	if platform == "" || !n.isAntiBanEnabledFor(platform) {
		return
	}
	strips := append([]string{}, n.cfg.StripProxyHeaders...)
	if prof := n.getPlatformProfile(platform); len(prof.StripExtra) > 0 {
		strips = append(strips, prof.StripExtra...)
	}
	strips = append(strips, "x-anthropic-billing-header", "x-anthropic-attribution")
	for _, prefix := range strips {
		lowerPrefix := strings.ToLower(prefix)
		toDelete := []string{}
		for k := range req.Header {
			if strings.HasPrefix(strings.ToLower(k), lowerPrefix) {
				toDelete = append(toDelete, k)
			}
		}
		for _, k := range toDelete {
			delete(req.Header, k)
		}
	}
}

// ApplyToRequest applies the canonical to req and body.
// Fixed per review: safe body, no bad user_id object, json.Valid guard, non-anthro limits.
// Note: req may be nil (called from buildUpstreamRequest before http.NewRequest);
// header stripping is handled separately via StripProxyHeaders after request creation.
func (n *FingerprintNormalizer) ApplyToRequest(req *http.Request, body []byte, canonical *CanonicalFingerprint) (*http.Request, []byte, error) {
	if canonical == nil || !n.isAntiBanEnabledFor(canonical.Platform) {
		return req, body, nil
	}
	origBody := append([]byte(nil), body...)

	plat := normalizePlatform(canonical.Platform)

	// 1. Headers
	if req != nil {
		strips := append([]string{}, n.cfg.StripProxyHeaders...)
		if prof := n.getPlatformProfile(canonical.Platform); len(prof.StripExtra) > 0 {
			strips = append(strips, prof.StripExtra...)
		}
		strips = append(strips, "x-anthropic-billing-header", "x-anthropic-attribution")
		for _, prefix := range strips {
			toDelete := []string{}
			for k := range req.Header {
				if strings.HasPrefix(strings.ToLower(k), strings.ToLower(prefix)) {
					toDelete = append(toDelete, k)
				}
			}
			for _, k := range toDelete {
				delete(req.Header, k)
			}
		}
		if canonical.UserAgent != "" {
			req.Header.Set("User-Agent", canonical.UserAgent)
		}
	}

	// 2. Body - safe rewrites only (production complete)
	if len(body) > 0 {
		newBody := body

		// Always safe telemetry stripping (all platforms)
		for _, p := range n.cfg.TelemetryPaths {
			key := strings.Trim(p, "/")
			newBody = safeRenameJSONKey(newBody, key)
			newBody = safeRenameJSONKey(newBody, "metadata."+key)
		}

		// Anthropic/Claude specific: metadata.user_id as string only (P1-1 fix)
		if (plat == "anthropic" || plat == "claude") && gjson.GetBytes(newBody, "metadata").Exists() {
			uid := gjson.GetBytes(newBody, "metadata.user_id")
			if uid.Exists() && uid.Type == gjson.String && canonical.DeviceID != "" && canonical.AccountUUID != "" {
				formatted := FormatMetadataUserID(canonical.DeviceID, canonical.AccountUUID, canonical.SessionID, canonical.UserAgent)
				newBody, _ = sjson.SetBytes(newBody, "metadata.user_id", formatted)
			}
		}

		// Attribution block and trigger phrase scanning are Claude/Anthropic-only.
		// Applying them to OpenAI/Grok mutates valid prompt text and changes native
		// request payloads in ways that are not part of the platform fingerprint.
		if plat == "anthropic" || plat == "claude" {
			newBody = stripAttributionBlock(newBody)

			// Trigger phrase scanning: remove known third-party harness identifiers from text fields
			if n.cfg.TriggerScanEnabled && len(n.cfg.TriggerPhrases) > 0 {
				newBody = sanitizeTriggerPhrases(newBody, n.cfg.TriggerPhrases)
			}
		}

		// Platform-aware stripping / renaming (complete for multiple platforms)
		prof := n.getPlatformProfile(canonical.Platform)
		bodyStrip := append([]string{}, prof.BodyStripKeys...)

		switch plat {
		case "anthropic", "claude":
			// Already handled user_id and attribution above
			// Add anthro specific if needed
			for _, k := range []string{"cch", "x_anthropic_cch", "cc_version", "cc_entrypoint"} {
				newBody = safeDeleteJSONKey(newBody, k)
				newBody = safeDeleteJSONKey(newBody, "metadata."+k)
			}
		case "openai", "codex":
			// Careful for images/embeddings/videos - avoid breaking user field if present in schema
			// Only safe renames for fp/telemetry keys
			for _, k := range []string{"session", "context_id", "prompt_id", "installation_id"} {
				if !isSensitiveOpenAIKeyForImages(newBody, k) {
					newBody = safeRenameJSONKey(newBody, k)
				}
			}
		case "grok", "xai":
			for _, k := range []string{"user_id", "session_id", "xai_id", "model_id"} {
				newBody = safeRenameJSONKey(newBody, k)
			}
		case "antigravity":
			for _, k := range []string{"user", "device", "session", "agent_id"} {
				newBody = safeRenameJSONKey(newBody, k)
			}
		default:
			// other platforms: conservative telemetry only
		}

		// Additional profile body strip keys
		for _, k := range bodyStrip {
			if (plat == "openai" || plat == "codex" || plat == "grok" || plat == "xai") && k == "user" {
				continue
			}
			newBody = safeDeleteJSONKey(newBody, k)
			newBody = safeDeleteJSONKey(newBody, "metadata."+k)
		}

		// Machine/device/client fp ids - prefer delete
		for _, k := range []string{"machine_id", "device_id", "client_id"} {
			newBody = safeDeleteJSONKey(newBody, k)
			newBody = safeDeleteJSONKey(newBody, "metadata."+k)
		}

		body = newBody
	}

	// P1-2 / safety: json valid guard, rollback on fail
	if len(body) > 0 && !json.Valid(body) {
		body = origBody
		return req, body, fmt.Errorf("normalizer produced invalid JSON, rolled back")
	}

	// 3. Anthropic/Claude: 真实 CC CLI 不发送 client_metadata（已从 CC 源码
	// paramsFromContext() L1699-1728 确认）。防御性删除第三方客户端可能注入的该字段。
	// 先前此处主动注入 client_metadata.process/env 伪值，但 CC 不发送该字段，
	// 注入反而暴露代理特征。
	if (plat == "anthropic" || plat == "claude") && len(body) > 0 {
		body = safeDeleteJSONKey(body, "client_metadata")
	}

	if req != nil && len(body) > 0 {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}

	return req, body, nil
}

func (n *FingerprintNormalizer) getPlatformProfile(plat string) PlatformProfile {
	if n.cfg.PlatformProfiles != nil {
		if p, ok := n.cfg.PlatformProfiles[plat]; ok {
			return p
		}
	}
	// Fallback to manager hardcoded profiles
	if n.platformFPManager != nil {
		return n.platformFPManager.Get(plat)
	}
	return PlatformProfile{}
}

func normalizePlatform(p string) string {
	return strings.ToLower(strings.TrimSpace(p))
}

func stripAttributionBlock(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	// P1-2 fix: only safe text fields version, no raw byte slicing
	return stripAttributionFromTextFields(b)
}

func stripAttributionFromTextFields(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	textFieldPaths := []string{"system", "instructions", "prompt", "text", "content"}
	segmentMarkers := []string{"x-anthropic-billing-header:", "cc_version=", "cch="}
	for _, p := range textFieldPaths {
		v := gjson.GetBytes(b, p)
		if v.Exists() && v.Type == gjson.String {
			cleaned := scrubAttributionSegment(v.String(), segmentMarkers)
			if cleaned != v.String() {
				if nb, err := sjson.SetBytes(b, p, cleaned); err == nil {
					b = nb
				}
			}
		}
	}
	// messages array handling — only scan system/developer role messages
	if msgs := gjson.GetBytes(b, "messages"); msgs.IsArray() {
		for i := 0; i < len(msgs.Array()) && i < 20; i++ {
			base := fmt.Sprintf("messages.%d", i)
			role := gjson.GetBytes(b, base+".role").String()
			if role != "system" && role != "developer" {
				continue // skip user/assistant messages to avoid mutating user content
			}
			// content as string
			cp := base + ".content"
			if v := gjson.GetBytes(b, cp); v.Exists() && v.Type == gjson.String {
				cleaned := scrubAttributionSegment(v.String(), segmentMarkers)
				if cleaned != v.String() {
					if nb, err := sjson.SetBytes(b, cp, cleaned); err == nil {
						b = nb
					}
				}
			}
			// parts array (anthropic style)
			if parts := gjson.GetBytes(b, base+".content"); parts.IsArray() {
				for j := 0; j < len(parts.Array()); j++ {
					pp := fmt.Sprintf("%s.content.%d.text", base, j)
					if v := gjson.GetBytes(b, pp); v.Exists() && v.Type == gjson.String {
						cleaned := scrubAttributionSegment(v.String(), segmentMarkers)
						if cleaned != v.String() {
							if nb, err := sjson.SetBytes(b, pp, cleaned); err == nil {
								b = nb
							}
						}
					}
				}
			}
		}
	}
	return b
}

func scrubAttributionSegment(s string, markers []string) string {
	for _, m := range markers {
		for {
			idx := strings.Index(s, m)
			if idx == -1 {
				break
			}
			// Find start of the block (back to newline or quote or space)
			start := idx
			for start > 0 && s[start-1] != '\n' && s[start-1] != '"' && s[start-1] != ' ' && s[start-1] != '{' {
				start--
			}
			// Find end (to newline, quote, or } )
			end := idx + len(m)
			for end < len(s) && s[end] != '\n' && s[end] != '"' && s[end] != '}' && s[end] != ',' {
				end++
			}
			if end < len(s) {
				// consume trailing whitespace/newlines if present
				for end < len(s) && (s[end] == ' ' || s[end] == '\n' || s[end] == '\r') {
					end++
				}
			}
			s = s[:start] + s[end:]
		}
	}
	return s
}

// ---------------------------------------------------------------------------
// PII scrubbing: email & git user.name
// ---------------------------------------------------------------------------
//
// 真实 Claude Code CLI 会在 system prompt 中注入用户的 OAuth 邮箱和 git user.name：
//   - "The user's email address is alice@example.com."
//   - "Git user: Alice\n" (在 gitStatus 块中)
//
// 这些字段对上游 API 完全无作用，但如果透传会泄漏终端用户的 PII。
// scrubSystemPromptPII 在 body 转发前擦除这些信息。

// 预编译正则（避免每次请求重新编译）
var (
	// 匹配 "The user's email address is <anything>." 整句（含可选的前后换行）
	reUserEmail = regexp.MustCompile(`(?m)^[^\S\n]*The user'?s email address is [^\n]+\.\s*\n?`)
	// 匹配 "Git user: <name>" 整行
	reGitUser = regexp.MustCompile(`(?m)^[^\S\n]*Git user:\s*[^\n]+\n?`)

	// 新增：Environment 块字段（真实 CC CLI 在 # Environment 下以 - bullet 形式发送）
	rePrimaryWorkDir    = regexp.MustCompile(`(?m)(Primary working directory:\s*)[^\n]+`)
	reAdditionalWorkDir = regexp.MustCompile(`(?m)^[^\S\n]*-?\s*Additional working director(?:y|ies):?\s*[^\n]*\n?`)
	rePlatform          = regexp.MustCompile(`(?m)(- Platform:\s*)[^\n]+`)
	reShell             = regexp.MustCompile(`(?m)(- Shell:\s*)[^\n]+`)
	reOSVersion         = regexp.MustCompile(`(?m)(- OS Version:\s*)[^\n]+`)
)

// scrubSystemPromptPII 从 body 的 system prompt 文本字段中擦除 userEmail 和 Git user 信息。
// 它同时处理 Anthropic API 的两种 system 格式：
//   - system 为字符串
//   - system 为 [{type:"text", text:"..."}] 数组
func scrubSystemPromptPII(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	system := gjson.GetBytes(b, "system")
	if !system.Exists() {
		return b
	}
	switch {
	case system.Type == gjson.String:
		// system 为纯字符串
		cleaned := scrubPIIInText(system.String())
		if cleaned != system.String() {
			if nb, err := sjson.SetBytes(b, "system", cleaned); err == nil {
				b = nb
			}
		}
	case system.IsArray():
		// system 为 [{type:"text", text:"...", cache_control?:...}, ...]
		for i, block := range system.Array() {
			if block.Get("type").String() != "text" {
				continue
			}
			text := block.Get("text").String()
			if text == "" {
				continue
			}
			cleaned := scrubPIIInText(text)
			if cleaned != text {
				path := fmt.Sprintf("system.%d.text", i)
				if nb, err := sjson.SetBytes(b, path, cleaned); err == nil {
					b = nb
				}
			}
		}
	}
	return b
}

// scrubPIIInText 从文本中移除 email 和 git user 行。
func scrubPIIInText(s string) string {
	s = reUserEmail.ReplaceAllString(s, "")
	s = reGitUser.ReplaceAllString(s, "")
	return s
}

// ----- AccountEnvProfile + deterministic env/PII replacement (CC OAuth real client) -----

// buildAccountEnvProfile derives a stable AccountEnvProfile for the given
// account using SHA256(accountID || salt || field) for all choices. This
// ensures same account always produces identical values (good for cache/CCH
// stability) while different accounts diverge.
func buildAccountEnvProfile(accountID int64, fp *Fingerprint) *AccountEnvProfile {
	if accountID <= 0 {
		accountID = 1 // defensive
	}
	idb := []byte(fmt.Sprintf("%d", accountID))
	salt := []byte("env-profile-v1")

	h := func(field string) []byte {
		sum := sha256.Sum256(append(append(idb, salt...), []byte(field)...))
		return sum[:]
	}

	prof := &AccountEnvProfile{
		IsGit: true,
	}

	// 1. Platform is the root of truth (inferred from captured StainlessOS or hash)
	stainlessOS := ""
	if fp != nil {
		stainlessOS = fp.StainlessOS
	}
	plat := inferPlatformFromStainless(stainlessOS)
	if plat == "" {
		// hash fallback (darwin/linux only, win32 rare)
		b := h("platform")[0]
		if b%2 == 0 {
			plat = "darwin"
		} else {
			plat = "linux"
		}
	}
	prof.Platform = plat

	// 2. StainlessOS / Arch (SDK values) derived from platform
	switch plat {
	case "darwin":
		prof.StainlessOS = "MacOS"
	case "linux":
		prof.StainlessOS = "Linux"
	case "win32":
		prof.StainlessOS = "Windows"
	default:
		prof.StainlessOS = "Linux"
	}

	// 3. GitUser (first name pool)
	gh := h("gituser")[0]
	prof.GitUser = envProfileFirstNames[int(gh)%len(envProfileFirstNames)]

	// 4. Email
	eh := h("email")[:3]
	prof.Email = "user-" + hex.EncodeToString(eh) + "@claude-code.local"

	// 5. WorkDir
	wh := h("workdir")
	dirIdx := int(wh[1]) % len(envProfileProjectNames)
	dirName := envProfileProjectNames[dirIdx]
	uname := strings.ToLower(prof.GitUser)
	switch plat {
	case "darwin":
		prof.WorkDir = "/Users/" + uname + "/projects/" + dirName
	case "linux":
		prof.WorkDir = "/home/" + uname + "/projects/" + dirName
	case "win32":
		prof.WorkDir = `C:\Users\` + uname + `\projects\` + dirName
	default:
		prof.WorkDir = "/home/" + uname + "/projects/" + dirName
	}

	// 6. Shell
	switch plat {
	case "darwin":
		prof.Shell = "zsh"
	case "linux":
		prof.Shell = "bash"
	case "win32":
		prof.Shell = "unknown"
	default:
		prof.Shell = "bash"
	}

	// 7. OSVersion (small pools indexed by hash)
	ovh := h("osversion")[0]
	switch plat {
	case "darwin":
		dvs := []string{"Darwin 24.3.0", "Darwin 25.1.0"}
		prof.OSVersion = dvs[int(ovh)%len(dvs)]
	case "linux":
		lvs := []string{"Linux 6.8.0", "Linux 6.6.4", "Linux 6.10.11"}
		prof.OSVersion = lvs[int(ovh)%len(lvs)]
	case "win32":
		wvs := []string{"Windows 11 Home 10.0.26200", "Windows 11 Pro 10.0.22631"}
		prof.OSVersion = wvs[int(ovh)%len(wvs)]
	default:
		prof.OSVersion = "Linux 6.8.0"
	}

	// 8. Arch
	ah := h("arch")[0]
	switch plat {
	case "darwin":
		das := []string{"arm64", "x64"}
		prof.Arch = das[int(ah)%len(das)]
	case "linux":
		las := []string{"x64", "arm64"}
		prof.Arch = las[int(ah)%len(las)]
	case "win32":
		prof.Arch = "x64"
	default:
		prof.Arch = "arm64"
	}
	prof.StainlessArch = prof.Arch

	return prof
}

func inferPlatformFromStainless(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(s, "mac") || s == "mac os x" {
		return "darwin"
	}
	if s == "linux" {
		return "linux"
	}
	if strings.Contains(s, "win") {
		return "win32"
	}
	return ""
}

// normalizeSystemPromptPII replaces (not deletes) real CC client PII and
// environment fields in the system prompt using a stable per-account profile.
// Falls back to old deletion behavior when profile==nil (e.g. non-OAuth or error).
// Must be called on real CC client bodies (before CCH sign). For mimic paths
// the full system is already rewritten upstream of this.
func normalizeSystemPromptPII(b []byte, profile *AccountEnvProfile) ([]byte, *WorkDirRewrite) {
	if len(b) == 0 {
		return b, nil
	}
	if profile == nil {
		return scrubSystemPromptPII(b), nil
	}

	var wdr *WorkDirRewrite

	system := gjson.GetBytes(b, "system")
	if !system.Exists() {
		return b, nil
	}

	switch {
	case system.Type == gjson.String:
		orig := system.String()
		cleaned, w := normalizeTextPII(orig, profile)
		if w != nil {
			wdr = w
		}
		if cleaned != orig {
			if nb, err := sjson.SetBytes(b, "system", cleaned); err == nil {
				b = nb
			}
		}
	case system.IsArray():
		for i, block := range system.Array() {
			if block.Get("type").String() != "text" {
				continue
			}
			text := block.Get("text").String()
			if text == "" {
				continue
			}
			cleaned, w := normalizeTextPII(text, profile)
			if w != nil && wdr == nil {
				wdr = w
			}
			if cleaned != text {
				path := fmt.Sprintf("system.%d.text", i)
				if nb, err := sjson.SetBytes(b, path, cleaned); err == nil {
					b = nb
				}
			}
		}
	}
	return b, wdr
}

// normalizeTextPII does the actual regex-based replacement for a single text
// block. Captures the first Primary working directory replacement for round-trip.
func normalizeTextPII(s string, profile *AccountEnvProfile) (string, *WorkDirRewrite) {
	if profile == nil {
		return scrubPIIInText(s), nil
	}
	var wdr *WorkDirRewrite

	// email
	s = reUserEmail.ReplaceAllString(s, "The user's email address is "+profile.Email+".\n")

	// git user
	s = reGitUser.ReplaceAllString(s, "Git user: "+profile.GitUser+"\n")

	// Primary working directory (capture original real dir for response reverse)
	if m := rePrimaryWorkDir.FindStringSubmatch(s); len(m) == 2 {
		realDir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(m[0]), m[1]))
		if realDir != "" && realDir != profile.WorkDir {
			wdr = &WorkDirRewrite{RealDir: realDir, FakeDir: profile.WorkDir}
		}
		s = rePrimaryWorkDir.ReplaceAllString(s, "${1}"+profile.WorkDir)
	}

	// Additional working dirs — delete (rare, keeps prompt simpler)
	s = reAdditionalWorkDir.ReplaceAllString(s, "")

	// Platform / Shell / OS Version (bullets)
	s = rePlatform.ReplaceAllString(s, "${1}"+profile.Platform)
	s = reOSVersion.ReplaceAllString(s, "${1}"+profile.OSVersion)

	// Shell — special full line for win32 to match real CC output exactly
	if profile.Platform == "win32" {
		// match the long comment line too
		winShellRe := regexp.MustCompile(`(?m)(- Shell:\s*)[^\n]*`)
		s = winShellRe.ReplaceAllString(s, "${1}"+profile.Shell+" (use Unix shell syntax, not Windows CMD or PowerShell syntax. On Windows, use forward slashes for paths, e.g. \"dir/file.txt\" not \"dir\\file.txt\")")
	} else {
		s = reShell.ReplaceAllString(s, "${1}"+profile.Shell)
	}

	return s, wdr
}

// WorkDirRewrite carries the real<->fake working dir pair so that:
// - request body can have all path occurrences normalized (messages/tool results)
// - SSE response chunks can reverse fake -> real so client tools see original paths.
type WorkDirRewrite struct {
	RealDir string
	FakeDir string
}

// replaceWorkDirInBody performs full-body (JSON string) replacement of
// RealDir -> FakeDir. Handles both / and \\ (win32) escaped forms.
// Must run BEFORE signBillingHeaderCCH.
func replaceWorkDirInBody(body []byte, wdr *WorkDirRewrite) []byte {
	if wdr == nil || wdr.RealDir == "" || wdr.RealDir == wdr.FakeDir {
		return body
	}
	real := []byte(wdr.RealDir)
	fake := []byte(wdr.FakeDir)

	out := bytes.ReplaceAll(body, real, fake)

	// win32 JSON escaped form
	if strings.Contains(wdr.RealDir, `\`) {
		jsonReal := []byte(strings.ReplaceAll(wdr.RealDir, `\`, `\\`))
		jsonFake := []byte(strings.ReplaceAll(wdr.FakeDir, `\`, `\\`))
		out = bytes.ReplaceAll(out, jsonReal, jsonFake)
	}
	return out
}

// reverseWorkDirIfPresent reverses FakeDir -> RealDir in an SSE chunk (or any bytes).
// Used alongside reverse tool name restoration.
func reverseWorkDirIfPresent(v any, chunk []byte) []byte {
	if v == nil {
		return chunk
	}
	// support both *WorkDirRewrite and interface with .Get
	type getter interface{ Get(string) (any, bool) }
	if g, ok := v.(getter); ok {
		if raw, ok := g.Get("claude_work_dir_rewrite"); ok && raw != nil {
			if w, ok := raw.(*WorkDirRewrite); ok && w != nil {
				chunk = doReverseWorkDir(chunk, w)
			}
		}
		return chunk
	}
	if w, ok := v.(*WorkDirRewrite); ok && w != nil {
		return doReverseWorkDir(chunk, w)
	}
	return chunk
}

func doReverseWorkDir(chunk []byte, w *WorkDirRewrite) []byte {
	if w.FakeDir == "" {
		return chunk
	}
	res := bytes.ReplaceAll(chunk, []byte(w.FakeDir), []byte(w.RealDir))
	if strings.Contains(w.FakeDir, `\`) {
		jf := strings.ReplaceAll(w.FakeDir, `\`, `\\`)
		jr := strings.ReplaceAll(w.RealDir, `\`, `\\`)
		res = bytes.ReplaceAll(res, []byte(jf), []byte(jr))
	}
	return res
}

// sanitizeTriggerPhrases scans text fields in body for known third-party harness
// phrases and replaces/removes them. Phrases are matched case-insensitively.
// Pure alphabetic short phrases (≤6 chars) require word boundaries
// to avoid false positives (e.g. "continue" inside "discontinue").
func sanitizeTriggerPhrases(b []byte, phrases [][2]string) []byte {
	if len(b) == 0 || len(phrases) == 0 {
		return b
	}
	// Reuse the same text field paths as stripAttributionFromTextFields
	textFieldPaths := []string{"system", "instructions", "prompt", "text", "content"}
	for _, p := range textFieldPaths {
		v := gjson.GetBytes(b, p)
		if v.Exists() && v.Type == gjson.String {
			cleaned := replaceTriggerPhrasesInText(v.String(), phrases)
			if cleaned != v.String() {
				if nb, err := sjson.SetBytes(b, p, cleaned); err == nil {
					b = nb
				}
			}
		}
	}
	// messages array — only scan system/developer role messages
	if msgs := gjson.GetBytes(b, "messages"); msgs.IsArray() {
		for i := 0; i < len(msgs.Array()) && i < 30; i++ {
			base := fmt.Sprintf("messages.%d", i)
			role := gjson.GetBytes(b, base+".role").String()
			if role != "system" && role != "developer" {
				continue // skip user/assistant messages to avoid mutating user content
			}
			// content as string
			cp := base + ".content"
			if v := gjson.GetBytes(b, cp); v.Exists() && v.Type == gjson.String {
				cleaned := replaceTriggerPhrasesInText(v.String(), phrases)
				if cleaned != v.String() {
					if nb, err := sjson.SetBytes(b, cp, cleaned); err == nil {
						b = nb
					}
				}
			}
			// content as array (Anthropic style)
			if parts := gjson.GetBytes(b, cp); parts.IsArray() {
				for j := 0; j < len(parts.Array()); j++ {
					pp := fmt.Sprintf("%s.content.%d.text", base, j)
					if v := gjson.GetBytes(b, pp); v.Exists() && v.Type == gjson.String {
						cleaned := replaceTriggerPhrasesInText(v.String(), phrases)
						if cleaned != v.String() {
							if nb, err := sjson.SetBytes(b, pp, cleaned); err == nil {
								b = nb
							}
						}
					}
				}
			}
		}
	}
	return b
}

// replaceTriggerPhrasesInText replaces trigger phrases in a text string.
// Short alphabetic phrases need word boundaries; phrases with special chars use exact match.
func replaceTriggerPhrasesInText(s string, phrases [][2]string) string {
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	for _, pair := range phrases {
		phrase, replacement := pair[0], pair[1]
		lowerPhrase := strings.ToLower(phrase)
		needBoundary := triggerPhraseIsPureAlpha(phrase) && len(phrase) <= 6
		offset := 0
		for {
			idx := strings.Index(lower[offset:], lowerPhrase)
			if idx == -1 {
				break
			}
			absIdx := offset + idx
			if needBoundary && !triggerPhraseHasWordBoundary(s, absIdx, len(phrase)) {
				offset = absIdx + len(lowerPhrase)
				continue
			}
			// Replace in original string preserving non-matched case
			s = s[:absIdx] + replacement + s[absIdx+len(phrase):]
			lower = strings.ToLower(s) // rebuild lowercase after splice
			offset = absIdx + len(replacement)
		}
	}
	return s
}

// triggerPhraseIsPureAlpha returns true if the phrase contains only ASCII letters and spaces.
func triggerPhraseIsPureAlpha(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && r != ' ' {
			return false
		}
	}
	return true
}

// triggerPhraseHasWordBoundary checks that the match at [idx, idx+length) has word boundaries.
func triggerPhraseHasWordBoundary(s string, idx int, length int) bool {
	if idx > 0 {
		c := s[idx-1]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			return false
		}
	}
	endIdx := idx + length
	if endIdx < len(s) {
		c := s[endIdx]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			return false
		}
	}
	return true
}

// buildTriggerPhrasePairs converts a map[string]string to a sorted [][2]string slice
// ordered by phrase length descending (longest first to prevent substring shadowing).
func buildTriggerPhrasePairs(m map[string]string) [][2]string {
	if len(m) == 0 {
		return nil
	}
	pairs := make([][2]string, 0, len(m))
	for phrase, replacement := range m {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" || isAmbiguousTriggerPhrase(phrase) {
			continue
		}
		pairs = append(pairs, [2]string{phrase, replacement})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return len(pairs[i][0]) > len(pairs[j][0])
	})
	return pairs
}

func isAmbiguousTriggerPhrase(phrase string) bool {
	switch strings.ToLower(strings.TrimSpace(phrase)) {
	case "cursor", "devin", "zed":
		return true
	default:
		return false
	}
}

func safeRenameJSONKey(b []byte, key string) []byte {
	if len(b) == 0 || key == "" {
		return b
	}
	if gjson.GetBytes(b, key).Exists() {
		val := gjson.GetBytes(b, key)
		if nb, err := sjson.SetBytes(b, key+"_canonical", val.Value()); err == nil {
			b = nb
			if db, err := sjson.DeleteBytes(b, key); err == nil {
				b = db
			}
		}
	}
	return b
}

func safeDeleteJSONKey(b []byte, key string) []byte {
	if len(b) == 0 || key == "" {
		return b
	}
	if nb, err := sjson.DeleteBytes(b, key); err == nil {
		return nb
	}
	return b
}

// PlatformFingerprintManager provides per-platform overrides for canonicalization.
type PlatformFingerprintManager struct {
	profiles map[string]PlatformProfile
}

func NewPlatformFingerprintManager(cfg *config.Config) *PlatformFingerprintManager {
	m := &PlatformFingerprintManager{
		profiles: map[string]PlatformProfile{
			"anthropic": {
				CanonicalUA:   "claude-cli/2.1.161 (external, cli)",
				JitterMinMs:   10,
				JitterMaxMs:   150,
				SpoofMemoryMB: 8192,
				SpoofHeapMB:   4096,
				CPUInfo:       "Apple M2",
				StripExtra:    []string{"x-stainless"},
				BodyStripKeys: []string{"events", "stats"},
			},
			"openai": {
				CanonicalUA:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
				JitterMinMs:   5,
				JitterMaxMs:   80,
				SpoofMemoryMB: 16384,
				SpoofHeapMB:   8192,
				CPUInfo:       "Intel Core i9",
				StripExtra:    []string{"x-litellm"},
				BodyStripKeys: []string{"user", "session"},
			},
			"grok": {
				CanonicalUA:   "grok-cli/1.0",
				JitterMinMs:   8,
				JitterMaxMs:   120,
				SpoofMemoryMB: 8192,
				SpoofHeapMB:   4096,
				CPUInfo:       "AMD Ryzen",
				BodyStripKeys: []string{"user_id", "xai_id"},
			},
			"antigravity": {
				CanonicalUA:   "antigravity-client/0.9",
				JitterMinMs:   10,
				JitterMaxMs:   100,
				SpoofMemoryMB: 4096,
				SpoofHeapMB:   2048,
				BodyStripKeys: []string{"device", "agent_id"},
			},
		},
	}
	if cfg != nil {
		// Field-level merge: config values override hardcoded only when non-zero/non-empty,
		// so a partial config override (e.g. only canonical_ua) doesn't zero out other fields.
		af := cfg.Gateway.AntiFingerprint
		for k, v := range af.PlatformProfiles {
			base := m.profiles[k] // zero-value if not hardcoded — new platform
			if v.CanonicalUA != "" {
				base.CanonicalUA = v.CanonicalUA
			}
			if v.JitterMinMs > 0 {
				base.JitterMinMs = v.JitterMinMs
			}
			if v.JitterMaxMs > 0 {
				base.JitterMaxMs = v.JitterMaxMs
			}
			if v.SpoofMemoryMB > 0 {
				base.SpoofMemoryMB = v.SpoofMemoryMB
			}
			if v.SpoofHeapMB > 0 {
				base.SpoofHeapMB = v.SpoofHeapMB
			}
			if v.CPUInfo != "" {
				base.CPUInfo = v.CPUInfo
			}
			if len(v.StripExtra) > 0 {
				base.StripExtra = v.StripExtra
			}
			if len(v.BodyStripKeys) > 0 {
				base.BodyStripKeys = v.BodyStripKeys
			}
			m.profiles[k] = base
		}
	}
	return m
}

func (m *PlatformFingerprintManager) Get(platform string) PlatformProfile {
	if m.profiles == nil {
		return PlatformProfile{}
	}
	if p, ok := m.profiles[platform]; ok {
		return p
	}
	return PlatformProfile{
		CanonicalUA:   "Mozilla/5.0 (compatible; GenericClient/1.0)",
		JitterMinMs:   10,
		JitterMaxMs:   80,
		SpoofMemoryMB: 8192,
		SpoofHeapMB:   4096,
		CPUInfo:       "Generic CPU",
	}
}

// ProvidePlatformFingerprintManager is the wire provider for PlatformFingerprintManager.
func ProvidePlatformFingerprintManager(cfg *config.Config) *PlatformFingerprintManager {
	return NewPlatformFingerprintManager(cfg)
}

func ProvideFingerprintNormalizer(
	tlsProfile *TLSFingerprintProfileService,
	tlsRouter *TLSFingerprintRouterService,
	identity *IdentityService,
	cfg *config.Config,
	mgr *PlatformFingerprintManager,
) *FingerprintNormalizer {
	normalizerCfg := &CanonicalFingerprintConfig{
		Enabled:             true,
		AntiBanEnabled:      true,
		StripProxyHeaders:   []string{"x-litellm", "helicone", "cf-aig", "x-portkey", "x-forwarded", "via"},
		SpoofProcessMetrics: true,
		TelemetryPaths:      []string{"/telemetry", "datadog", "sentry", "statsig", "segment", "amplitude", "events"},
		JitterMinMs:         10,
		JitterMaxMs:         150,
		PlatformProfiles:    map[string]PlatformProfile{},
	}
	if cfg != nil {
		af := cfg.Gateway.AntiFingerprint
		normalizerCfg.Enabled = af.Enabled
		normalizerCfg.AntiBanEnabled = cfg.Gateway.AntiBan.Enabled
		if len(af.StripProxyHeaders) > 0 {
			normalizerCfg.StripProxyHeaders = af.StripProxyHeaders
		}
		normalizerCfg.SpoofProcessMetrics = af.SpoofProcessMetrics
		if len(af.TelemetryPaths) > 0 {
			normalizerCfg.TelemetryPaths = af.TelemetryPaths
		}
		normalizerCfg.JitterMinMs = af.JitterMinMs
		normalizerCfg.JitterMaxMs = af.JitterMaxMs
		// Trigger phrase scanning
		normalizerCfg.TriggerScanEnabled = af.TriggerScanEnabled
		normalizerCfg.TriggerPhrases = buildTriggerPhrasePairs(af.TriggerPhrases)
		// Schema property rewriting
		normalizerCfg.SchemaPropRewriteEnabled = af.ToolSchemaPropRewriteEnabled
		normalizerCfg.SchemaPropRewrites = af.ToolSchemaPropRewrites
		// Convert platform profiles
		for k, v := range af.PlatformProfiles {
			normalizerCfg.PlatformProfiles[k] = PlatformProfile{
				CanonicalUA:   v.CanonicalUA,
				JitterMinMs:   v.JitterMinMs,
				JitterMaxMs:   v.JitterMaxMs,
				SpoofMemoryMB: v.SpoofMemoryMB,
				SpoofHeapMB:   v.SpoofHeapMB,
				CPUInfo:       v.CPUInfo,
				StripExtra:    v.StripExtra,
				BodyStripKeys: v.BodyStripKeys,
			}
		}

		// Per-platform anti-ban toggles (from gateway.anti_ban.platforms). Default OFF; only explicit true enables.
		ban := cfg.Gateway.AntiBan
		normalizerCfg.AntiBanEnabled = ban.Enabled
		if len(ban.Platforms) > 0 {
			normalizerCfg.EnabledByPlatform = cloneAntiBanPlatforms(ban.Platforms)
		} else if ban.Enabled {
			// no map -> all off (default)
		}
	}
	return NewFingerprintNormalizer(tlsProfile, tlsRouter, identity, normalizerCfg, mgr)
}

// SchemaPropRewrites returns the configured schema property rewrite map if enabled, nil otherwise.
func (n *FingerprintNormalizer) SchemaPropRewrites() map[string]string {
	if n == nil || n.cfg == nil || !n.cfg.SchemaPropRewriteEnabled || len(n.cfg.SchemaPropRewrites) == 0 {
		return nil
	}
	return n.cfg.SchemaPropRewrites
}

// helpers
func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func isSensitiveOpenAIKeyForImages(body []byte, key string) bool {
	// rough heuristic: if images or embeddings present, be conservative on "user"
	if gjson.GetBytes(body, "images").Exists() || gjson.GetBytes(body, "input").Exists() {
		return key == "user"
	}
	return false
}
