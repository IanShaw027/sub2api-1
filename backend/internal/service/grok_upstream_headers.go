package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// grokUpstreamUserAgent is kept for compatibility with older Grok request
// tests. Current requests use the pinned default UA from this package.
const grokUpstreamUserAgent = "sub2api-grok/1.0"

// Fixed CLI identity aliases — single source of truth is internal/pkg/xai.
const (
	grokClientVersionHeader    = xai.CLIStableVersion
	grokClientIdentifierHeader = xai.CLIClientIdentifier
	grokClientModeHeader       = xai.CLIClientMode
	grokClientModeInteractive  = "interactive"
)

// defaultGrokUpstreamUserAgent is the pinned Grok CLI / workspace UA.
// Grok upstream must not forward Claude Code / Codex / browser client UAs.
func defaultGrokUpstreamUserAgent() string {
	return xai.CLIUserAgent(xai.ResolveCLIVersion())
}

func applyDefaultGrokUpstreamHeaders(req *http.Request) {
	_ = applyGrokUpstreamHeadersFromAccount(context.Background(), req, nil)
}

// applyGrokUpstreamHeadersFromAccount stamps Grok chat/CLI outbound identity
// with x-grok-client-mode=cli. When account is set, identity comes from the
// device profile (fail closed on load error). When account is nil, today's
// pinned CLI stamp is used so probes keep working without a profile service.
func applyGrokUpstreamHeadersFromAccount(ctx context.Context, req *http.Request, account *Account) error {
	return applyGrokUpstreamHeadersFromAccountWithMode(ctx, req, account, xai.CLIClientMode)
}

// applyGrokInteractiveUpstreamHeadersFromAccount is the live/user-request
// stamp: same profile load as applyGrokUpstreamHeadersFromAccount, but
// x-grok-client-mode=interactive. Fail closed when account is set.
func applyGrokInteractiveUpstreamHeadersFromAccount(ctx context.Context, req *http.Request, account *Account) error {
	return applyGrokUpstreamHeadersFromAccountWithMode(ctx, req, account, grokClientModeInteractive)
}

// applyGrokInteractiveUpstreamHeaders stamps interactive identity onto a
// header map (WS / header-only probes) using the same profile load.
func applyGrokInteractiveUpstreamHeaders(ctx context.Context, headers http.Header, account *Account) error {
	if headers == nil {
		return nil
	}
	return applyGrokInteractiveUpstreamHeadersFromAccount(ctx, &http.Request{Header: headers}, account)
}

func applyGrokUpstreamHeadersFromAccountWithMode(ctx context.Context, req *http.Request, account *Account, clientMode string) error {
	if req == nil {
		return nil
	}

	var profile *AccountDeviceProfile
	if account != nil {
		if ctx == nil {
			ctx = context.Background()
		}
		loadCtx, cancel := context.WithTimeout(ctx, outboundDeviceProfileLoadTimeout)
		loaded, err := LoadOutboundDeviceProfile(loadCtx, account)
		cancel()
		if err != nil {
			return err
		}
		profile = loaded
	}

	sanitizeGrokOutboundHeaders(req)
	stampGrokCLIIdentity(req, profile, clientMode)
	return nil
}

func stampGrokCLIIdentity(req *http.Request, profile *AccountDeviceProfile, clientMode string) {
	if req == nil {
		return
	}
	version := xai.ResolveCLIVersion()
	identifier := grokClientIdentifierHeader

	if profile != nil {
		if v := strings.TrimSpace(profile.ClientVersion); v != "" {
			version = v
		}
		if id := grokProfilePayloadString(profile, "grok_identifier"); id != "" {
			identifier = id
		}
	}

	ua := xai.CLIUserAgent(version)
	if profileUA := grokProfilePayloadString(profile, "user_agent"); profileUA != "" {
		ua = profileUA
	}
	if strings.TrimSpace(clientMode) == "" {
		clientMode = xai.CLIClientMode
	}

	req.Header.Set("User-Agent", ua)
	req.Header.Set("x-grok-client-version", version)
	req.Header.Set("x-grok-client-identifier", identifier)
	req.Header.Set("x-grok-client-mode", clientMode)
}

func grokProfilePayloadString(profile *AccountDeviceProfile, key string) string {
	if profile == nil || profile.ProfilePayload == nil {
		return ""
	}
	raw, ok := profile.ProfilePayload[key]
	if !ok {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func sanitizeGrokOutboundHeaders(req *http.Request) {
	if req == nil || req.Header == nil {
		return
	}
	for name := range req.Header {
		if isUnknownGrokHeader(name) || isGrokGatewayAffinityHeader(name) {
			req.Header.Del(name)
		}
	}
}

func isUnknownGrokHeader(name string) bool {
	canonical := strings.ToLower(strings.TrimSpace(name))
	if !strings.HasPrefix(canonical, "x-grok-") {
		return false
	}
	switch canonical {
	case "x-grok-client-version", "x-grok-client-identifier", "x-grok-conv-id", "x-grok-client-mode":
		return false
	default:
		return true
	}
}

func isGrokGatewayAffinityHeader(name string) bool {
	canonical := strings.ToLower(strings.TrimSpace(name))
	for _, sessionName := range explicitOpenAIHeaderSessionNames {
		if canonical == strings.ToLower(strings.TrimSpace(sessionName)) {
			return true
		}
	}
	switch canonical {
	case "chatgpt-account-id", "x-account-id", "account-id":
		return true
	default:
		return false
	}
}

func applyGrokTLSProfileHeaders(req *http.Request, profile *tlsfingerprint.Profile) {
	// HEAD Profile is TLS-only (no HTTP UserAgent/Originator fields). Always stamp CLI identity.
	applyDefaultGrokUpstreamHeaders(req)
	_ = profile
}

// openAITLSFingerprintRuntime is the resolved TLS fingerprint routing result
// used by OpenAI/Grok outbound header application. Defined here so Grok header
// helpers compile even when the full OpenAI TLS router is not present on HEAD.
type openAITLSFingerprintRuntime struct {
	Profile            *tlsfingerprint.Profile
	UpstreamUserAgent  string
	UpstreamOriginator string
	Matched            bool
}

func applyGrokRuntimeHeaders(req *http.Request, runtime openAITLSFingerprintRuntime) {
	applyDefaultGrokUpstreamHeaders(req)
	if req == nil {
		return
	}
	// Apply Originator only; force CLI UA after so router overrides cannot
	// leak Codex/Claude Code identity onto Grok upstream.
	if originator := strings.TrimSpace(runtime.UpstreamOriginator); originator != "" {
		req.Header.Set("Originator", originator)
	}
	req.Header.Set("User-Agent", defaultGrokUpstreamUserAgent())
}

// resolveGrokUpstreamUserAgent always returns the pinned Grok CLI User-Agent.
// Inbound client UAs (Claude Code, Codex, browsers, libraries) are never forwarded.
func resolveGrokUpstreamUserAgent(_ *gin.Context) string {
	return defaultGrokUpstreamUserAgent()
}
