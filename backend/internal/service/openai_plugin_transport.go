package service

import (
	"context"
	"net/http"
)

func (s *OpenAIGatewayService) SetPluginManager(manager *PluginManager) {
	s.pluginManager = manager
}

// doOpenAIUpstream 只在 OpenAI OAuth 能力绑定已启用时把真实请求交给插件。
// 插件返回标准 http.Response，响应解析、错误映射、SSE 和计费仍由现有核心链处理。
// Grok 不进插件，走 leftover pin，避免 TLS 路由器改掉 device-profile UA。
// 未命中插件时走个人 TLS / fingerprint 路径；协议从 URL 推断，避免把
// chat/messages/live 误绑到 responses 模板。
func (s *OpenAIGatewayService) doOpenAIUpstream(request *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	inboundUA, ctx := inboundUAAndContextFromRequest(request)
	if account != nil && account.Platform == PlatformGrok {
		return doOpenAITLSFallback(ctx, s.httpUpstream, request, proxyURL, account, s.tlsFPProfileService, s.tlsFPRouterService, inboundUA)
	}
	if s.pluginManager != nil && request != nil {
		response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(request.Context(), request, proxyURL, account)
		if handled {
			return response, err
		}
	}
	return doOpenAITLSFallback(ctx, s.httpUpstream, request, proxyURL, account, s.tlsFPProfileService, s.tlsFPRouterService, inboundUA)
}

func inboundUAAndContextFromRequest(request *http.Request) (string, context.Context) {
	if request == nil {
		return "", context.Background()
	}
	return request.Header.Get("User-Agent"), request.Context()
}

func doOpenAITLSFallback(
	ctx context.Context,
	upstream HTTPUpstream,
	request *http.Request,
	proxyURL string,
	account *Account,
	profileSvc *TLSFingerprintProfileService,
	routerSvc *TLSFingerprintRouterService,
	inboundUA string,
) (*http.Response, error) {
	if account != nil && account.Platform == PlatformGrok {
		return doLeftoverAccountHTTP(ctx, upstream, request, proxyURL, account, profileSvc, routerSvc, inboundUA, "http", "")
	}
	return doAccountHTTPUpstream(ctx, upstream, request, proxyURL, account, profileSvc, routerSvc, inboundUA, "http", "")
}

// doOpenAIAccountTestUpstream 让 OpenAI OAuth 账号测试与真实转发使用同一插件路径。
// API Key 和未命中插件的账号保持各自原有的 HTTPUpstream 行为。
func (s *AccountTestService) doOpenAIAccountTestUpstream(
	request *http.Request,
	proxyURL string,
	account *Account,
	useTLSFallback bool,
) (*http.Response, error) {
	if s.pluginManager != nil {
		response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(request.Context(), request, proxyURL, account)
		if handled {
			return response, err
		}
	}
	if useTLSFallback {
		return s.httpUpstream.DoWithTLS(
			request,
			proxyURL,
			account.ID,
			account.EffectiveConcurrency(),
			s.tlsFPProfileService.ResolveTLSProfile(account),
		)
	}
	return s.httpUpstream.Do(request, proxyURL, account.ID, account.EffectiveConcurrency())
}
