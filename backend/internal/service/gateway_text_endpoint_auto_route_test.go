package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newOpenAICompatCCAutoRouteAccountForTest() *Account {
	account := newAnthropicAPIKeyAccountForTest()
	account.Name = "openai-compat-cc-auto-route"
	account.Credentials["base_url"] = "https://glm.example.compat"
	account.Extra = map[string]any{
		"openai_compat_cc_upstream": true,
		"text_endpoint_auto_route":  true,
	}
	return account
}

func newGatewayTextEndpointContext(method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, rec
}

func TestGatewayForwardAsChatCompletions_AutoRouteCCUpstreamUsesRawChatEndpoint(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c, rec := newGatewayTextEndpointContext(http.MethodPost, "/v1/chat/completions", body)
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"upstream-chat-1"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"chatcmpl_1","object":"chat.completion","model":"glm-5.2","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, newOpenAICompatCCAutoRouteAccountForTest(), body, &ParsedRequest{
		Model:  "glm-5.2",
		Stream: false,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.Equal(t, "Bearer upstream-anthropic-key", upstream.lastReq.Header.Get("Authorization"))
	require.JSONEq(t, `{"model":"glm-5.2","messages":[{"role":"user","content":"hello"}],"stream":false}`, string(upstream.lastBody))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "chatcmpl_1", gjson.Get(rec.Body.String(), "id").String())
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
}

func TestGatewayForwardAsResponses_AutoRouteCCUpstreamUsesRawChatEndpoint(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"glm-5.2","input":"hello","stream":false,"tools":[{"type":"web_search"}]}`)
	c, rec := newGatewayTextEndpointContext(http.MethodPost, "/v1/responses", body)
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"upstream-chat-2"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"id":"chatcmpl_2","object":"chat.completion","model":"glm-5.2","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6,"prompt_tokens_details":{"cached_tokens":1}}}`))),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}

	result, err := svc.ForwardAsResponses(context.Background(), c, newOpenAICompatCCAutoRouteAccountForTest(), body, &ParsedRequest{
		Model:  "glm-5.2",
		Stream: false,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.Equal(t, "Bearer upstream-anthropic-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "web_search", gjson.GetBytes(upstream.lastBody, "tools.0.type").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "tools.0.function").Exists())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "response", gjson.Get(rec.Body.String(), "object").String())
	require.Equal(t, "completed", gjson.Get(rec.Body.String(), "status").String())
	require.Equal(t, 4, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, 1, result.Usage.CacheReadInputTokens)
}
