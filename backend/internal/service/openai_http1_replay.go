package service

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

var openAIRawHTTP1HeaderOrder = []string{
	"accept",
	"accept-language",
	"authorization",
	"chatgpt-account-id",
	"cookie",
	"oai-device-id",
	"oai-session-id",
	"openai-beta",
	"origin",
	"originator",
	"referer",
	"sec-fetch-dest",
	"sec-fetch-mode",
	"sec-fetch-site",
	"session_id",
	"conversation_id",
	"user-agent",
	"content-type",
	"x-client-request-id",
	"thread-id",
	"x-codex-window-id",
	"x-stainless-arch",
	"x-stainless-lang",
	"x-stainless-os",
	"x-stainless-package-version",
	"x-stainless-runtime",
	"x-stainless-runtime-version",
	"x-stainless-retry-count",
	"x-stainless-timeout",
}

func withOpenAIHTTP1RawHeaderReplay(req *http.Request, account *Account, profile *tlsfingerprint.Profile) *http.Request {
	if req == nil || account == nil || account.Type != AccountTypeOAuth || profile == nil {
		return req
	}
	if openAIHTTP1ReplayProfileWantsH2(profile) {
		return req
	}
	targetHost := strings.ToLower(strings.TrimSpace(req.Host))
	if targetHost == "" && req.URL != nil {
		targetHost = strings.ToLower(strings.TrimSpace(req.URL.Hostname()))
	}
	if targetHost != "chatgpt.com" {
		return req
	}
	ctx := WithHTTPUpstreamRequestOptions(req.Context(), HTTPUpstreamRequestOptions{
		RawHTTP1HeaderOrder: append([]string(nil), openAIRawHTTP1HeaderOrder...),
	})
	return req.WithContext(ctx)
}

func openAIHTTP1ReplayProfileWantsH2(profile *tlsfingerprint.Profile) bool {
	if profile == nil {
		return false
	}
	for _, proto := range profile.ALPNProtocols {
		if strings.EqualFold(strings.TrimSpace(proto), "h2") {
			return true
		}
	}
	return false
}
