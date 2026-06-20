package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, 1, result.Usage.CacheReadInputTokens)
}

func TestGatewayForwardMessages_AutoRouteCCUpstreamPreservesNativeToolsAndUsage(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"glm-5.2","max_tokens":64,"stream":false,"tools":[{"type":"web_search_20250305","name":"web_search"},{"type":"web_fetch_20250910","name":"web_fetch"}],"tool_choice":{"type":"tool","name":"web_fetch"},"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newGatewayTextEndpointContext(http.MethodPost, "/v1/messages", body)
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_msg_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_msg_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_msg_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		`data: {"id":"chatcmpl_msg_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7,"prompt_tokens_details":{"cached_tokens":2}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"upstream-messages-1"}},
			Body:       io.NopCloser(strings.NewReader(upstreamBody)),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)

	result, err := svc.Forward(context.Background(), c, newOpenAICompatCCAutoRouteAccountForTest(), parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Equal(t, "web_search", gjson.GetBytes(upstream.lastBody, "tools.0.type").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "tools.0.function").Exists())
	require.Equal(t, "web_fetch", gjson.GetBytes(upstream.lastBody, "tools.1.type").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "tools.1.function").Exists())
	require.JSONEq(t, `{"type":"web_fetch"}`, gjson.GetBytes(upstream.lastBody, "tool_choice").Raw)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "message", gjson.Get(rec.Body.String(), "type").String())
	require.Equal(t, "ok", gjson.Get(rec.Body.String(), "content.0.text").String())
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, 2, result.Usage.CacheReadInputTokens)
}

func TestGatewayForwardMessages_OpenAICompatCCUpstreamRequiresTextEndpointAutoRoute(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"glm-5.2","max_tokens":64,"stream":false,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newGatewayTextEndpointContext(http.MethodPost, "/v1/messages", body)
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"upstream-messages-plain"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"msg_plain","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"glm-5.2","stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`)),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)
	account := newOpenAICompatCCAutoRouteAccountForTest()
	delete(account.Extra, "text_endpoint_auto_route")

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "/v1/messages", upstream.lastReq.URL.Path)
	require.Equal(t, "beta=true", upstream.lastReq.URL.RawQuery)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", gjson.Get(rec.Body.String(), "content.0.text").String())
}

func TestGatewayForwardAsChatCompletions_AutoRouteCCUpstreamStreamsUsage(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c, rec := newGatewayTextEndpointContext(http.MethodPost, "/v1/chat/completions", body)
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_stream_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_stream_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_stream_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[],"usage":{"prompt_tokens":6,"completion_tokens":3,"total_tokens":9,"prompt_tokens_details":{"cached_tokens":2}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"upstream-chat-stream-1"}},
			Body:       io.NopCloser(strings.NewReader(upstreamBody)),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, newOpenAICompatCCAutoRouteAccountForTest(), body, &ParsedRequest{
		Model:  "glm-5.2",
		Stream: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
	require.Equal(t, 4, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, 2, result.Usage.CacheReadInputTokens)
	require.True(t, result.Stream)
}

func TestGatewayForwardAsResponses_AutoRouteCCUpstreamStreamsUsage(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"glm-5.2","input":"hello","stream":true,"tools":[{"type":"web_fetch"}]}`)
	c, rec := newGatewayTextEndpointContext(http.MethodPost, "/v1/responses", body)
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_stream_resp_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_stream_resp_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_stream_resp_1","object":"chat.completion.chunk","model":"glm-5.2","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":4,"total_tokens":11,"prompt_tokens_details":{"cached_tokens":3}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"upstream-responses-stream-1"}},
			Body:       io.NopCloser(strings.NewReader(upstreamBody)),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}

	result, err := svc.ForwardAsResponses(context.Background(), c, newOpenAICompatCCAutoRouteAccountForTest(), body, &ParsedRequest{
		Model:  "glm-5.2",
		Stream: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "web_fetch", gjson.GetBytes(upstream.lastBody, "tools.0.type").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "response.output_text.delta")
	require.Contains(t, rec.Body.String(), "data: [DONE]")
	require.Equal(t, 4, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)
	require.True(t, result.Stream)
}
