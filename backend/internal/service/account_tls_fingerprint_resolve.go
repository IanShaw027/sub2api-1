package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
)

// accountTLSFingerprintRuntime is the shared outbound TLS identity for any platform.
type accountTLSFingerprintRuntime struct {
	Profile            *tlsfingerprint.Profile
	UpstreamUserAgent  string
	UpstreamOriginator string
	Matched            bool
}

// resolveAccountTLSFingerprintRuntime selects a TLS profile for an account using:
//  1. account default / transport-bound profile
//  2. optional router match on inbound UA (+ transport)
//  3. OS×client_type bindings matrix when the router emits dimensions
//
// Platforms without inbound UA should pass empty inboundUA; resolution then falls
// back to default_os + bindings / single profile (static path).
func resolveAccountTLSFingerprintRuntime(
	ctx context.Context,
	account *Account,
	profileSvc *TLSFingerprintProfileService,
	routerSvc *TLSFingerprintRouterService,
	inboundUA, transport string,
) accountTLSFingerprintRuntime {
	runtime := accountTLSFingerprintRuntime{}
	if account == nil || !account.IsTLSFingerprintEnabled() || profileSvc == nil {
		return runtime
	}

	if account.Platform == PlatformKiro && account.GetTLSFingerprintDefaultOS() != "" {
		runtime.Profile, _ = profileSvc.resolveTLSProfileForDimension(
			account, account.GetTLSFingerprintDefaultOS(), "kiro-ide", transport, true,
		)
	} else {
		runtime.Profile = profileSvc.ResolveTLSProfileForTransport(account, transport)
	}
	seedAccountTLSFingerprintRuntimeHeaders(&runtime, runtime.Profile)
	if strings.TrimSpace(inboundUA) == "" {
		inboundUA = tlsFingerprintInboundUserAgentFromContext(ctx)
	}

	if routerSvc == nil {
		return runtime
	}
	routerID := account.GetTLSFingerprintRouterID()
	if routerID <= 0 {
		return runtime
	}

	match, ok := routerSvc.MatchRequest(ctx, routerID, inboundUA, transport)
	if !ok {
		logger.LegacyPrintf("service.tls_fp_router",
			"[TLSFPRouter] no_match account_id=%d platform=%s transport=%s router_id=%d inbound_ua=%s",
			account.ID, account.Platform, transport, routerID, inboundUA)
		return runtime
	}

	resolvedOS := strings.ToLower(strings.TrimSpace(match.OS))
	if resolvedOS == "" {
		resolvedOS = inferTLSFingerprintOS(inboundUA)
		if resolvedOS == "" {
			resolvedOS = account.GetTLSFingerprintDefaultOS()
		}
	}
	resolvedClientType := strings.ToLower(strings.TrimSpace(match.ClientType))
	if resolvedClientType == "" {
		resolvedClientType = inferTLSFingerprintClientType(account.Platform, inboundUA, "")
	}

	var profile *tlsfingerprint.Profile
	hasDimensionMatch := resolvedOS != "" || resolvedClientType != ""
	if hasDimensionMatch {
		if p, resolved := profileSvc.ResolveTLSProfileForDimensionMatch(account, resolvedOS, resolvedClientType, transport); resolved {
			profile = p
		}
	}
	if profile == nil && match.ProfileID > 0 {
		profile = profileSvc.resolveRouterProfileByIDForAccount(match.ProfileID, account)
	}
	if profile == nil && hasDimensionMatch {
		profile = builtinDefaultTLSProfile()
	}
	if profile == nil {
		return runtime
	}

	runtime.Profile = profile
	runtime.UpstreamUserAgent = strings.TrimSpace(match.UpstreamUserAgent)
	runtime.UpstreamOriginator = strings.TrimSpace(match.UpstreamOriginator)
	seedAccountTLSFingerprintRuntimeHeaders(&runtime, profile)
	runtime.Matched = true
	logger.LegacyPrintf("service.tls_fp_router",
		"[TLSFPRouter] matched account_id=%d platform=%s transport=%s os=%s client_type=%s profile=%s upstream_ua=%s upstream_originator=%s",
		account.ID, account.Platform, transport, resolvedOS, resolvedClientType, profile.Name,
		runtime.UpstreamUserAgent, runtime.UpstreamOriginator)
	return runtime
}

func inferTLSFingerprintClientType(platform, userAgent, originator string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	identity := strings.ToLower(strings.TrimSpace(userAgent + " " + originator))
	switch {
	case strings.Contains(identity, "codex desktop") || strings.Contains(identity, "chatgpt"):
		return "chatgpt-desktop"
	case strings.Contains(identity, "codex_cli_rs"), strings.Contains(identity, "codex-tui"), strings.Contains(identity, "codex_exec"), strings.Contains(identity, "codex"):
		return "codex-cli"
	case strings.Contains(identity, "claude-cli"), strings.Contains(identity, "claude-code"):
		return "claude-code"
	case strings.Contains(identity, "claude desktop"):
		return "claude-desktop"
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

func inferTLSFingerprintOS(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	switch {
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

func seedAccountTLSFingerprintRuntimeHeaders(runtime *accountTLSFingerprintRuntime, profile *tlsfingerprint.Profile) {
	if runtime == nil {
		return
	}
	if strings.TrimSpace(runtime.UpstreamUserAgent) == "" && profile != nil {
		runtime.UpstreamUserAgent = strings.TrimSpace(profile.UserAgent)
	}
	if strings.TrimSpace(runtime.UpstreamOriginator) == "" && profile != nil {
		runtime.UpstreamOriginator = strings.TrimSpace(profile.Originator)
	}
}

func inboundUserAgentFromGin(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return c.Request.Header.Get("User-Agent")
}

func inboundUserAgentFromHTTP(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.Header.Get("User-Agent")
}
