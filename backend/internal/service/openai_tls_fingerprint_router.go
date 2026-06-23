package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
)

type openAITLSFingerprintRuntime struct {
	Profile            *tlsfingerprint.Profile
	UpstreamUserAgent  string
	UpstreamOriginator string
	Matched            bool
}

func (s *OpenAIGatewayService) SetTLSFingerprintRouterService(routerService *TLSFingerprintRouterService) {
	if s == nil {
		return
	}
	s.tlsFPRouterService = routerService
}

func (s *OpenAIGatewayService) resolveOpenAITLSFingerprintRuntime(ctx context.Context, c *gin.Context, account *Account, transport string) openAITLSFingerprintRuntime {
	runtime := openAITLSFingerprintRuntime{}
	if s != nil {
		runtime.Profile = s.resolveOpenAITLSProfile(account)
	}
	// The account-bound template can carry an upstream UA/Originator. A router
	// match may override these below; an empty router value falls back to here,
	// and an empty template value falls back to the built-in defaults in apply().
	seedTLSFingerprintRuntimeHeaders(&runtime, runtime.Profile)
	if s == nil || s.tlsFPRouterService == nil || account == nil {
		return runtime
	}
	if !account.IsOpenAITLSFingerprintEnabled() {
		return runtime
	}
	routerID := account.GetTLSFingerprintRouterID()
	if routerID <= 0 {
		return runtime
	}
	inboundUA := ""
	if c != nil && c.Request != nil {
		inboundUA = c.Request.Header.Get("User-Agent")
	}
	match, ok := s.tlsFPRouterService.MatchRequest(ctx, routerID, inboundUA, transport)
	if !ok {
		logger.LegacyPrintf("service.tls_fp_router", "[TLSFPRouter] no_match account_id=%d transport=%s router_id=%d inbound_ua=%s", account.ID, transport, routerID, inboundUA)
		return runtime
	}
	if s.tlsFPProfileService == nil || match.ProfileID <= 0 {
		return runtime
	}
	profile := s.tlsFPProfileService.ResolveTLSProfileByID(match.ProfileID)
	if profile == nil {
		return runtime
	}
	runtime.Profile = profile
	runtime.UpstreamUserAgent = strings.TrimSpace(match.UpstreamUserAgent)
	runtime.UpstreamOriginator = strings.TrimSpace(match.UpstreamOriginator)
	// A router match overrides the headers, but an empty override falls back to
	// the matched template's own UA/Originator.
	seedTLSFingerprintRuntimeHeaders(&runtime, profile)
	runtime.Matched = true
	logger.LegacyPrintf("service.tls_fp_router",
		"[TLSFPRouter] matched account_id=%d transport=%s profile=%s cipher_suites=%v curves=%v extensions=%v supported_versions=%v key_share_groups=%v psk_modes=%v signature_algorithms=%v alpn=%v upstream_ua=%s upstream_originator=%s",
		account.ID, transport, profile.Name,
		profile.CipherSuites, profile.Curves, profile.Extensions,
		profile.SupportedVersions, profile.KeyShareGroups, profile.PSKModes,
		profile.SignatureAlgorithms, profile.ALPNProtocols,
		runtime.UpstreamUserAgent, runtime.UpstreamOriginator)
	return runtime
}

// seedTLSFingerprintRuntimeHeaders fills any empty runtime UA/Originator from the
// template profile. Existing non-empty values (e.g. a router override) win.
func seedTLSFingerprintRuntimeHeaders(runtime *openAITLSFingerprintRuntime, profile *tlsfingerprint.Profile) {
	if runtime == nil || profile == nil {
		return
	}
	if runtime.UpstreamUserAgent == "" {
		runtime.UpstreamUserAgent = strings.TrimSpace(profile.UserAgent)
	}
	if runtime.UpstreamOriginator == "" {
		runtime.UpstreamOriginator = strings.TrimSpace(profile.Originator)
	}
}

func applyOpenAITLSFingerprintRuntime(req *http.Request, runtime openAITLSFingerprintRuntime) {
	if req == nil {
		return
	}
	if runtime.UpstreamUserAgent != "" {
		req.Header.Set("User-Agent", runtime.UpstreamUserAgent)
	}
	if runtime.UpstreamOriginator != "" {
		req.Header.Set("Originator", runtime.UpstreamOriginator)
	}
}
