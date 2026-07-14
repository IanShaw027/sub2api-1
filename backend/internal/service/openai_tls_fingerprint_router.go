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

func (s *OpenAIGatewayService) resolveOpenAITLSFingerprintRuntime(ctx context.Context, c *gin.Context, account *Account, transport string) openAITLSFingerprintRuntime {
	if account != nil && !account.IsOpenAITLSFingerprintEnabled() {
		return openAITLSFingerprintRuntime{}
	}
	return toOpenAITLSFingerprintRuntime(resolveAccountTLSFingerprintRuntime(
		ctx, account, s.tlsFPProfileService, s.tlsFPRouterService, inboundUserAgentFromGin(c), transport,
	))
}

func (s *OpenAIGatewayService) resolveGrokTLSFingerprintRuntime(ctx context.Context, c *gin.Context, account *Account, transport string) openAITLSFingerprintRuntime {
	if account != nil && !account.IsGrokTLSFingerprintEnabled() {
		return openAITLSFingerprintRuntime{}
	}
	return toOpenAITLSFingerprintRuntime(resolveAccountTLSFingerprintRuntime(
		ctx, account, s.tlsFPProfileService, s.tlsFPRouterService, inboundUserAgentFromGin(c), transport,
	))
}

func (s *OpenAIGatewayService) resolveOpenAICompatibleTLSFingerprintRuntime(ctx context.Context, c *gin.Context, account *Account, transport string) openAITLSFingerprintRuntime {
	if account != nil && account.Platform == PlatformGrok {
		return s.resolveGrokTLSFingerprintRuntime(ctx, c, account, transport)
	}
	return s.resolveOpenAITLSFingerprintRuntime(ctx, c, account, transport)
}

func toOpenAITLSFingerprintRuntime(in accountTLSFingerprintRuntime) openAITLSFingerprintRuntime {
	return openAITLSFingerprintRuntime{
		Profile:            in.Profile,
		UpstreamUserAgent:  in.UpstreamUserAgent,
		UpstreamOriginator: in.UpstreamOriginator,
		Matched:            in.Matched,
	}
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
