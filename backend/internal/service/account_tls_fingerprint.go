package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
)

func supportsAccountTLSFingerprint(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformKiro:
		return true
	default:
		return false
	}
}

func (a *Account) isTLSFingerprintFlagEnabled() bool {
	if a == nil || a.Extra == nil {
		return false
	}
	if v, ok := a.Extra["enable_tls_fingerprint"]; ok {
		if enabled, ok := v.(bool); ok {
			return enabled
		}
	}
	return false
}

// GetTLSFingerprintRouterID 获取账号绑定的 TLS 指纹路由 ID。
func (a *Account) GetTLSFingerprintRouterID() int64 {
	if a == nil || a.Extra == nil {
		return 0
	}
	id, _ := tlsFPInt64FromAny(a.Extra["tls_fingerprint_router_id"])
	return id
}

// GetTLSFingerprintBindings 返回账号的「维度 → 模板ID」绑定矩阵。
func (a *Account) GetTLSFingerprintBindings() map[string]int64 {
	if a == nil || a.Extra == nil {
		return nil
	}
	raw, ok := a.Extra["tls_fingerprint_bindings"]
	if !ok || raw == nil {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]int64, len(m))
	for k, v := range m {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		if id, ok := tlsFPInt64FromAny(v); ok {
			out[key] = id
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// GetTLSFingerprintDefaultOS 返回无入站 UA 时使用的默认 OS。
func (a *Account) GetTLSFingerprintDefaultOS() string {
	if a == nil || a.Extra == nil {
		return ""
	}
	if v, ok := a.Extra["tls_fingerprint_default_os"].(string); ok {
		os := strings.ToLower(strings.TrimSpace(v))
		switch os {
		case "windows", "macos", "linux", "ios", "android":
			return os
		}
	}
	return ""
}

func tlsFPInt64FromAny(v any) (int64, bool) {
	switch id := v.(type) {
	case float64:
		return int64(id), true
	case int64:
		return id, true
	case int:
		return int64(id), true
	case json.Number:
		if i, err := id.Int64(); err == nil {
			return i, true
		}
	}
	return 0, false
}

func lookupTLSFingerprintBinding(bindings map[string]int64, os, clientType, protocol string) int64 {
	os = strings.ToLower(strings.TrimSpace(os))
	clientType = strings.ToLower(strings.TrimSpace(clientType))
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if len(bindings) == 0 {
		return 0
	}
	keys := make([]string, 0, 8)
	if os != "" && clientType != "" && protocol != "" {
		keys = append(keys, os+"/"+clientType+"@"+protocol)
	}
	if os != "" && clientType != "" {
		keys = append(keys, os+"/"+clientType)
	}
	if os != "" && protocol != "" {
		keys = append(keys, os+"@"+protocol)
	}
	if clientType != "" && protocol != "" {
		keys = append(keys, clientType+"@"+protocol)
	}
	if os != "" {
		keys = append(keys, os)
	}
	if clientType != "" {
		keys = append(keys, clientType)
	}
	if protocol != "" {
		keys = append(keys, protocol)
	}
	for _, key := range keys {
		if id, ok := bindings[key]; ok && id != 0 {
			return id
		}
	}
	if len(bindings) == 1 {
		for _, id := range bindings {
			if id != 0 {
				return id
			}
		}
	}
	return 0
}

func inferTLSFingerprintOS(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	switch {
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "ios"):
		return "ios"
	case strings.Contains(ua, "android"):
		return "android"
	case strings.Contains(ua, "windows"), strings.Contains(ua, "win32"), strings.Contains(ua, "win64"):
		return "windows"
	case strings.Contains(ua, "mac os"), strings.Contains(ua, "macos"), strings.Contains(ua, "macintosh"), strings.Contains(ua, "darwin"), strings.Contains(ua, "os x"):
		return "macos"
	case strings.Contains(ua, "linux"), strings.Contains(ua, "ubuntu"):
		return "linux"
	default:
		return ""
	}
}

func inferTLSFingerprintClientType(platform, userAgent, originator string) string {
	identity := strings.ToLower(strings.TrimSpace(userAgent + " " + originator))
	switch {
	case strings.Contains(identity, "codex desktop"), strings.Contains(identity, "chatgpt"):
		return "chatgpt-desktop"
	case strings.Contains(identity, "codex_cli_rs"), strings.Contains(identity, "codex-tui"), strings.Contains(identity, "codex_exec"), strings.Contains(identity, "codex"):
		return "codex-cli"
	case strings.Contains(identity, "claude-cli"), strings.Contains(identity, "claude-code"):
		return "claude-code"
	case strings.Contains(identity, "claude desktop"):
		return "claude-desktop"
	case strings.Contains(identity, "gemini-cli"), strings.Contains(identity, "google-gemini"):
		return "gemini-cli"
	case strings.Contains(identity, "antigravity"):
		return "antigravity"
	case strings.Contains(identity, "cursor"):
		return "cursor"
	case strings.Contains(identity, "grokdesktop"):
		return "grok-desktop"
	case platform == PlatformGrok && strings.Contains(identity, "grok"):
		return "grok-web"
	case strings.Contains(identity, "kiroide"):
		return "kiro-ide"
	case platform == PlatformKiro && strings.Contains(identity, "kiro"):
		return "kiro-cli"
	case platform == PlatformOpenAI && strings.Contains(identity, "openai/"):
		return "openai-sdk"
	default:
		return ""
	}
}

func inferTLSFingerprintAccountClientType(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformAnthropic:
		return "claude-code"
	case PlatformOpenAI:
		return "codex-cli"
	case PlatformGemini:
		return "gemini-cli"
	case PlatformAntigravity:
		return "antigravity"
	case PlatformGrok:
		return "grok-desktop"
	case PlatformKiro:
		return "kiro-ide"
	default:
		return ""
	}
}

func resolveAccountTLSFingerprintRuntime(
	ctx context.Context,
	account *Account,
	profileSvc *TLSFingerprintProfileService,
	routerSvc *TLSFingerprintRouterService,
	inboundUA, transport, protocol string,
) accountTLSFingerprintRuntime {
	runtime := accountTLSFingerprintRuntime{}
	if account == nil || !account.IsTLSFingerprintEnabled() || profileSvc == nil {
		return runtime
	}

	os := account.GetTLSFingerprintDefaultOS()
	clientType := inferTLSFingerprintAccountClientType(account.Platform)
	if bindingID := lookupTLSFingerprintBinding(account.GetTLSFingerprintBindings(), os, clientType, protocol); bindingID != 0 {
		runtime.Profile = profileSvc.resolveTLSProfileByID(bindingID)
	}
	if runtime.Profile == nil {
		if account.GetTLSFingerprintProfileID() == -1 {
			runtime.Profile = &tlsfingerprint.Profile{Name: "Built-in Default (Node.js 24.x)"}
		} else {
			runtime.Profile = profileSvc.ResolveTLSProfile(account)
		}
	}

	if routerSvc == nil {
		return runtime
	}
	routerID := account.GetTLSFingerprintRouterID()
	if routerID <= 0 {
		return runtime
	}

	match, ok := routerSvc.MatchRequest(ctx, routerID, account.Platform, "", transport, protocol)
	if !ok {
		logger.LegacyPrintf("service.tls_fp_router",
			"[TLSFPRouter] no_match account_id=%d platform=%s protocol=%s router_id=%d",
			account.ID, account.Platform, protocol, routerID)
		return runtime
	}

	resolvedOS := firstNonEmpty(match.OS, os)
	resolvedClient := firstNonEmpty(match.ClientType, clientType)
	profile := profileSvc.resolveTLSProfileByID(lookupTLSFingerprintBinding(
		account.GetTLSFingerprintBindings(), resolvedOS, resolvedClient, firstNonEmpty(match.Protocol, protocol),
	))
	if profile == nil && match.ProfileID != 0 {
		profile = profileSvc.resolveTLSProfileByID(match.ProfileID)
	}
	if profile != nil {
		runtime.Profile = profile
	}
	runtime.UpstreamUserAgent = strings.TrimSpace(match.UpstreamUserAgent)
	runtime.UpstreamOriginator = strings.TrimSpace(match.UpstreamOriginator)
	runtime.Matched = true
	return runtime
}

func (s *TLSFingerprintProfileService) resolveTLSProfileByID(id int64) *tlsfingerprint.Profile {
	if s == nil || id == 0 {
		return nil
	}
	if id > 0 {
		return s.GetProfileByID(id)
	}
	return nil
}

func applyTLSFingerprintRuntimeHeaders(req *http.Request, runtime accountTLSFingerprintRuntime) {
	if req == nil {
		return
	}
	if ua := strings.TrimSpace(runtime.UpstreamUserAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	deleteHeaderAllForms(req.Header, "X-Originator")
	if originator := strings.TrimSpace(runtime.UpstreamOriginator); originator != "" {
		setHeaderRaw(req.Header, "originator", originator)
	}
}

func inboundUserAgentFromGin(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return c.Request.Header.Get("User-Agent")
}

func inboundProtocolFromPath(path string) string {
	p := strings.ToLower(strings.TrimSpace(path))
	switch {
	case strings.Contains(p, "/chat/completions"):
		return "chat_completions"
	case strings.Contains(p, "/responses"):
		return "responses"
	case strings.Contains(p, "/images"):
		return "images"
	case strings.Contains(p, "/embeddings"):
		return "embeddings"
	case strings.Contains(p, "/antigravity"):
		return "antigravity"
	case strings.Contains(p, "kiro"):
		return "kiro"
	case strings.Contains(p, "/v1beta"), strings.Contains(p, "generatecontent"):
		return "gemini"
	case strings.Contains(p, "/messages"):
		return "messages"
	default:
		return ""
	}
}
