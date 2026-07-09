package service

import (
	"net/http"
	"strings"

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
