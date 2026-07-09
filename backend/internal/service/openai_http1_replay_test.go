package service

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func TestWithOpenAIHTTP1RawHeaderReplay_EnablesOnlyForOAuthChatGPTH1Profiles(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/responses", nil)
	require.NoError(t, err)
	req.Host = "chatgpt.com"
	account := &Account{Type: AccountTypeOAuth}
	profile := &tlsfingerprint.Profile{ALPNProtocols: []string{"http/1.1"}}

	req = withOpenAIHTTP1RawHeaderReplay(req, account, profile)
	opts := HTTPUpstreamRequestOptionsFromContext(req.Context())
	require.NotEmpty(t, opts.RawHTTP1HeaderOrder)
	require.True(t, opts.RequiresDedicatedClient())

	h2Req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/responses", nil)
	require.NoError(t, err)
	h2Req.Host = "chatgpt.com"
	h2Req = withOpenAIHTTP1RawHeaderReplay(h2Req, account, &tlsfingerprint.Profile{ALPNProtocols: []string{"h2", "http/1.1"}})
	require.Empty(t, HTTPUpstreamRequestOptionsFromContext(h2Req.Context()).RawHTTP1HeaderOrder)

	apiKeyReq, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/responses", nil)
	require.NoError(t, err)
	apiKeyReq.Host = "chatgpt.com"
	apiKeyReq = withOpenAIHTTP1RawHeaderReplay(apiKeyReq, &Account{Type: AccountTypeAPIKey}, profile)
	require.Empty(t, HTTPUpstreamRequestOptionsFromContext(apiKeyReq.Context()).RawHTTP1HeaderOrder)

	otherHostReq, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", nil)
	require.NoError(t, err)
	otherHostReq = withOpenAIHTTP1RawHeaderReplay(otherHostReq, account, profile)
	require.Empty(t, HTTPUpstreamRequestOptionsFromContext(otherHostReq.Context()).RawHTTP1HeaderOrder)
}
