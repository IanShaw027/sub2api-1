package service

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const defaultBrowserLikeUpstreamUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
const defaultGrokUpstreamUserAgent = defaultBrowserLikeUpstreamUserAgent

func applyDefaultGrokUpstreamHeaders(req *http.Request) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.Header.Get("User-Agent")) == "" {
		req.Header.Set("User-Agent", defaultGrokUpstreamUserAgent)
	}
}

func applyGrokTLSProfileHeaders(req *http.Request, profile *tlsfingerprint.Profile) {
	applyDefaultGrokUpstreamHeaders(req)
	if req == nil || profile == nil {
		return
	}
	if ua := strings.TrimSpace(profile.UserAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if originator := strings.TrimSpace(profile.Originator); originator != "" {
		req.Header.Set("Originator", originator)
	}
}

func applyGrokRuntimeHeaders(req *http.Request, runtime openAITLSFingerprintRuntime) {
	applyDefaultGrokUpstreamHeaders(req)
	applyOpenAITLSFingerprintRuntime(req, runtime)
}

func resolveGrokUpstreamUserAgent(c *gin.Context) string {
	fallback := defaultGrokUpstreamUserAgent
	if c == nil {
		return fallback
	}
	ua := strings.TrimSpace(c.GetHeader("User-Agent"))
	if ua == "" {
		return fallback
	}
	lower := strings.ToLower(ua)
	// Keep a stable browser-like identity for generic library agents; pass through
	// real clients (Claude Code / Codex / Grok CLI) so upstream fingerprinting stays coherent.
	switch {
	case strings.HasPrefix(lower, "go-http-client"),
		strings.HasPrefix(lower, "python-"),
		strings.HasPrefix(lower, "axios/"),
		strings.HasPrefix(lower, "node-fetch"),
		strings.HasPrefix(lower, "curl/"),
		strings.HasPrefix(lower, "wget/"):
		return fallback
	default:
		return ua
	}
}
