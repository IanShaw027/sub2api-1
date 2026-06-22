package service

import (
	"context"
	"net/http"
	"strings"

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

func (s *OpenAIGatewayService) resolveOpenAITLSFingerprintRuntime(ctx context.Context, c *gin.Context, account *Account) openAITLSFingerprintRuntime {
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
	inboundOriginator := ""
	if c != nil && c.Request != nil {
		inboundUA = c.Request.Header.Get("User-Agent")
		inboundOriginator = c.Request.Header.Get("Originator")
	}
	match, ok := s.tlsFPRouterService.MatchRequest(ctx, routerID, inboundUA, inboundOriginator)
	if !ok {
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
