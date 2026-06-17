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
	match, ok := s.tlsFPRouterService.MatchUserAgent(ctx, routerID, inboundUA)
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
	runtime.Matched = true
	return runtime
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
