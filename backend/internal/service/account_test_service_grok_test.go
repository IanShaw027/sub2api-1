package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTestService_TestAccountConnection_GrokOAuthUsesXAIChatCompletions(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73977,
		Name:        "grok-oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n" +
				"data: [DONE]\n\n",
		)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73977/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-4.3", "hello grok", "")
	require.NoError(t, err)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, xai.DefaultBaseURL+"/chat/completions", upstream.lastReq.URL.String())
	require.NotContains(t, upstream.lastReq.URL.Host, "anthropic")
	require.Equal(t, "Bearer grok-access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, "sub2api-grok/1.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
	require.Equal(t, "grok-4.3", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hello grok", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.Contains(t, rec.Body.String(), `"text":"ok"`)
	require.Contains(t, rec.Body.String(), `"success":true`)
}
