package service

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// defaultBrowserLikeUpstreamUserAgent remains available for non-Grok OpenAI-like
// fingerprint templates that historically shared this constant.
const defaultBrowserLikeUpstreamUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

// Fixed CLI identity aliases — single source of truth is internal/pkg/xai.
const (
	grokClientVersionHeader    = xai.CLIStableVersion
	grokClientIdentifierHeader = xai.CLIClientIdentifier
	grokClientModeHeader       = xai.CLIClientMode
)

// defaultGrokUpstreamUserAgent is the pinned Grok CLI / workspace UA.
// Grok upstream must not forward Claude Code / Codex / browser client UAs.
func defaultGrokUpstreamUserAgent() string {
	return xai.CLIUserAgent(xai.ResolveCLIVersion())
}

func applyDefaultGrokUpstreamHeaders(req *http.Request) {
	if req == nil {
		return
	}
	// Always stamp CLI identity. Do not preserve inbound client UA (Claude Code,
	// Codex, curl, etc.) — xAI chat/CLI surfaces fingerprint the client string.
	req.Header.Set("User-Agent", defaultGrokUpstreamUserAgent())
	req.Header.Set("x-grok-client-version", xai.ResolveCLIVersion())
	req.Header.Set("x-grok-client-identifier", grokClientIdentifierHeader)
}

func applyGrokTLSProfileHeaders(req *http.Request, profile *tlsfingerprint.Profile) {
	applyDefaultGrokUpstreamHeaders(req)
	if req == nil || profile == nil {
		return
	}
	// TLS profile may still supply Originator for JA3-bound routes, but never
	// replace the CLI User-Agent with a non-Grok client string.
	if originator := strings.TrimSpace(profile.Originator); originator != "" {
		req.Header.Set("Originator", originator)
	}
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
