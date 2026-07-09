package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDoGrokNativeResponsesJSONUsesBuildResponsesURLNotDoubleV1(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          91,
		Name:        "grok-search",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     "https://api.x.ai/v1",
		},
	}
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_search","output":[]}`)),
		},
	}
	svc := &GatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(stubOpenAIAccountRepo{accounts: []Account{*account}}, nil),
	}

	c, _ := gin.CreateTestContext(nil)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/web_search", nil)

	body := []byte(`{"model":"","input":"latest news","tools":[{"type":"web_search"}],"stream":false,"store":false}`)
	resp, err := svc.DoGrokNativeResponsesJSON(context.Background(), c, account, body)
	require.NoError(t, err)
	require.Contains(t, string(resp), "resp_search")

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://api.x.ai/v1/responses", upstream.lastReq.URL.String())
	require.NotContains(t, upstream.lastReq.URL.String(), "/v1/v1/")
	require.Equal(t, "Bearer access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "responses=experimental", upstream.lastReq.Header.Get("OpenAI-Beta"))
	require.Equal(t, xai.DefaultTextModel, gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestDoGrokNativeResponsesJSONUsesGrokTokenProvider(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          92,
		Name:        "grok-token",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "provider-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
	// Without provider, GetAccessToken still works for non-expired tokens via raw credentials.
	// With provider set, path must still succeed and hit the correct CLI base URL.
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_ok"}`)),
		},
	}
	svc := &GatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(stubOpenAIAccountRepo{accounts: []Account{*account}}, nil),
	}

	_, err := svc.DoGrokNativeResponsesJSON(context.Background(), nil, account, []byte(`{"input":"hi","tools":[{"type":"web_search"}]}`))
	require.NoError(t, err)
	require.Equal(t, "Bearer provider-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, xai.DefaultCLIBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, xai.DefaultTextModel, gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestGetOAuthToken_GrokUsesTokenProvider(t *testing.T) {
	account := &Account{
		ID:       93,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "from-provider",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	svc := &GatewayService{
		grokTokenProvider: NewGrokTokenProvider(stubOpenAIAccountRepo{accounts: []Account{*account}}, nil),
	}
	token, kind, err := svc.getOAuthToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "oauth", kind)
	require.Equal(t, "from-provider", token)
}
