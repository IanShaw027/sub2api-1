//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOpenAIChatCompletionsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		want string
	}{
		// 已是 /chat/completions：原样返回
		{"already chat/completions", "https://api.openai.com/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
		// 以 /v1 结尾：追加 /chat/completions
		{"bare /v1", "https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		// 其他情况：追加 /v1/chat/completions
		{"bare domain", "https://api.openai.com", "https://api.openai.com/v1/chat/completions"},
		{"domain with trailing slash", "https://api.openai.com/", "https://api.openai.com/v1/chat/completions"},
		// 第三方上游常见形式
		{"third-party bare domain", "https://api.deepseek.com", "https://api.deepseek.com/v1/chat/completions"},
		{"third-party with path prefix", "https://api.gptgod.online/api", "https://api.gptgod.online/api/v1/chat/completions"},
		{"third-party versioned path", "https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn/api/paas/v4/chat/completions"},
		// 带空白字符
		{"whitespace trimmed", "  https://api.openai.com/v1  ", "https://api.openai.com/v1/chat/completions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildOpenAIChatCompletionsURL(tt.base)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestBuildOpenAIResponsesURL_ProbeURL 锁定 probe/测试端点使用的 URL 构建逻辑，
// 确保 buildOpenAIResponsesURL 对标准 OpenAI base_url 格式均拼出 `/v1/responses`。
func TestBuildOpenAIResponsesURL_ProbeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		want string
	}{
		{"bare domain", "https://api.openai.com", "https://api.openai.com/v1/responses"},
		{"domain trailing slash", "https://api.openai.com/", "https://api.openai.com/v1/responses"},
		{"bare /v1", "https://api.openai.com/v1", "https://api.openai.com/v1/responses"},
		{"already /responses", "https://api.openai.com/v1/responses", "https://api.openai.com/v1/responses"},
		{"third-party bare domain", "https://api.deepseek.com", "https://api.deepseek.com/v1/responses"},
		{"third-party versioned path", "https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn/api/paas/v4/responses"},
		{"only domain, no scheme", "api.gptgod.online", "api.gptgod.online/v1/responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildOpenAIResponsesURL(tt.base)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestForwardAsRawChatCompletions_GrokIsolatesSessionAndConvIDByAPIKey(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}],"stream":false,"prompt_cache_key":"shared-session"}`)
	account := &Account{
		ID:          201,
		Name:        "grok-raw-cc",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "xai-key",
			"base_url": xai.DefaultCLIBaseURL,
		},
	}

	run := func(apiKeyID int64, clientConvID string) (sessionID, convID string) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		if clientConvID != "" {
			c.Request.Header.Set("x-grok-conv-id", clientConvID)
			c.Request.Header.Set("session_id", clientConvID)
		}
		c.Set("api_key", &APIKey{ID: apiKeyID})

		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_grok_raw"}},
			Body: io.NopCloser(strings.NewReader(
				`{"id":"chatcmpl_grok","object":"chat.completion","model":"grok-4.3","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			)),
		}}
		svc := &OpenAIGatewayService{
			cfg:          rawChatCompletionsTestConfig(),
			httpUpstream: upstream,
		}

		result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "shared-session")
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, upstream.lastReq)

		sessionID = upstream.lastReq.Header.Get("session_id")
		convID = upstream.lastReq.Header.Get("x-grok-conv-id")
		require.NotEmpty(t, sessionID)
		require.NotEmpty(t, convID)
		require.NotEqual(t, "shared-session", convID)
		require.NotEqual(t, clientConvID, convID)
		expectedSeed := clientConvID
		if expectedSeed == "" {
			expectedSeed = "shared-session"
		}
		isolatedConvID := isolateOpenAISessionID(apiKeyID, expectedSeed)
		require.Equal(t, generateSessionUUID(isolatedConvID), sessionID)
		require.Equal(t, isolatedConvID, convID)
		require.Equal(t, "shared-session", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
		return sessionID, convID
	}

	idA, convA := run(11, "client-raw-conv")
	idB, convB := run(22, "client-raw-conv")
	require.Equal(t, generateSessionUUID(convA), idA)
	require.Equal(t, generateSessionUUID(convB), idB)
	require.NotEqual(t, idA, idB, "different API keys must not share Grok conversation ids on raw CC")
	require.NotEqual(t, convA, convB, "different API keys must not share Grok conversation ids on raw CC")
}

func TestForwardAsChatCompletions_GrokPassesPromptCacheKeyForSessionIsolation(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("api_key", &APIKey{ID: 77})

	account := &Account{
		ID:          202,
		Name:        "grok-cc-entry",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "xai-key",
			"base_url": xai.DefaultCLIBaseURL,
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_grok_entry"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_grok","object":"chat.completion","model":"grok-4.3","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "cache-key-grok", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)

	expectedConvID := isolateOpenAISessionID(77, "cache-key-grok")
	require.Equal(t, generateSessionUUID(expectedConvID), upstream.lastReq.Header.Get("session_id"))
	require.Equal(t, expectedConvID, upstream.lastReq.Header.Get("x-grok-conv-id"))
	require.Equal(t, "cache-key-grok", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
}

func TestForwardAsRawChatCompletions_ForcesStreamUsageUpstreamAndPassesUsageDownstream(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
		"",
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.4","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13,"prompt_tokens_details":{"cached_tokens":3,"cache_write_tokens":6}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_raw_usage"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 9, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)
	require.Equal(t, 6, result.Usage.CacheCreationInputTokens)
	require.NotNil(t, upstream.lastReq)
	require.NoError(t, upstream.lastReq.Context().Err())
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Contains(t, rec.Body.String(), `"usage"`)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}

func TestForwardAsRawChatCompletions_NonStreamingCapturesCacheWriteUsage(t *testing.T) {
	setGinTestMode()

	tests := []struct {
		name       string
		usageJSON  string
		wantInput  int
		wantOutput int
		wantRead   int
		wantWrite  int
	}{
		{
			name:       "positive cache write",
			usageJSON:  `{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":4,"cache_write_tokens":6}}`,
			wantInput:  12,
			wantOutput: 3,
			wantRead:   4,
			wantWrite:  6,
		},
		{
			name:       "nested zero overrides legacy alias",
			usageJSON:  `{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15,"cache_creation_input_tokens":19,"prompt_tokens_details":{"cached_tokens":4,"cache_write_tokens":0}}`,
			wantInput:  12,
			wantOutput: 3,
			wantRead:   4,
			wantWrite:  0,
		},
		{
			name:       "partial usage keeps input tokens",
			usageJSON:  `{"prompt_tokens":12}`,
			wantInput:  12,
			wantOutput: 0,
			wantRead:   0,
			wantWrite:  0,
		},
		{
			name:       "partial usage keeps output tokens",
			usageJSON:  `{"completion_tokens":3}`,
			wantInput:  0,
			wantOutput: 3,
			wantRead:   0,
			wantWrite:  0,
		},
		{
			name:       "chat completions fields win mixed aliases",
			usageJSON:  `{"prompt_tokens":12,"completion_tokens":3,"input_tokens":99,"output_tokens":88,"prompt_tokens_details":{"cached_tokens":4,"cache_write_tokens":6},"input_tokens_details":{"cached_tokens":44,"cache_write_tokens":66}}`,
			wantInput:  12,
			wantOutput: 3,
			wantRead:   4,
			wantWrite:  6,
		},
		{
			name:       "responses aliases remain a fallback",
			usageJSON:  `{"input_tokens":12,"output_tokens":3,"input_tokens_details":{"cached_tokens":4,"cache_write_tokens":6}}`,
			wantInput:  12,
			wantOutput: 3,
			wantRead:   4,
			wantWrite:  6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}],"stream":false}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")

			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"id":"chatcmpl_cache","object":"chat.completion","model":"gpt-5.6","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":` + tt.usageJSON + `}`,
				)),
			}}
			svc := &OpenAIGatewayService{
				cfg:          rawChatCompletionsTestConfig(),
				httpUpstream: upstream,
			}

			result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.wantInput, result.Usage.InputTokens)
			require.Equal(t, tt.wantOutput, result.Usage.OutputTokens)
			require.Equal(t, tt.wantRead, result.Usage.CacheReadInputTokens)
			require.Equal(t, tt.wantWrite, result.Usage.CacheCreationInputTokens)
		})
	}
}

func TestForwardAsRawChatCompletions_StreamCyberPolicyMarksOpsContext(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"error":{"code":"cyber_policy","message":"blocked by upstream policy"},"usage":{"prompt_tokens":17,"completion_tokens":8,"total_tokens":25}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_raw_cyber"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	mark := GetOpsCyberPolicy(c)
	require.NotNil(t, mark, "streaming passthrough cyber_policy must be marked for handler-side audit/hash recording")
	require.Equal(t, "cyber_policy", mark.Code)
	require.Equal(t, http.StatusOK, mark.UpstreamStatus)
	require.Equal(t, 17, mark.UpstreamInTok)
	require.Equal(t, 8, mark.UpstreamOutTok)
	require.Contains(t, rec.Body.String(), "cyber_policy")
}

func TestBufferRawChatCompletions_CyberPolicyCapturesUsageInMark(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_raw_cyber_usage"}},
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"code":"cyber_policy","message":"blocked"},"usage":{"prompt_tokens":23,"completion_tokens":5,"total_tokens":28}}`,
		)),
	}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}

	result, err := svc.bufferRawChatCompletions(c, resp, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5.4", "gpt-5.4", "gpt-5.4", nil, nil, time.Now())

	require.NoError(t, err)
	require.NotNil(t, result)
	mark := GetOpsCyberPolicy(c)
	require.NotNil(t, mark)
	require.Equal(t, "cyber_policy", mark.Code)
	require.Equal(t, 23, mark.UpstreamInTok)
	require.Equal(t, 5, mark.UpstreamOutTok)
}

func TestForwardAsChatCompletions_TextEndpointAutoRouteForceChatSkipsMessagesUpstream(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamJSON := `{"id":"chatcmpl_auto_route","object":"chat.completion","model":"glm-5.2","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_auto_route_chat"}},
		Body:       io.NopCloser(strings.NewReader(upstreamJSON)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()
	account.Extra = map[string]any{
		"anthropic_messages_upstream":            true,
		"text_endpoint_auto_route":               true,
		openai_compat.ExtraKeyResponsesMode:      string(openai_compat.ResponsesSupportModeForceChatCompletions),
		openai_compat.ExtraKeyResponsesSupported: false,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Empty(t, upstream.lastReq.Header.Get("x-api-key"))
	require.Equal(t, "glm-5.2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
	require.Equal(t, "ok", gjson.Get(rec.Body.String(), "choices.0.message.content").String())
}

func TestForwardAsRawChatCompletions_PreservesDeepSeekReasoningContentNonStreaming(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"deepseek-reasoner","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamJSON := `{"id":"chatcmpl_reasoning","object":"chat.completion","model":"deepseek-reasoner","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"think first","content":"final answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_deepseek_reasoning_json"}},
		Body:       io.NopCloser(strings.NewReader(upstreamJSON)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.Equal(t, "think first", gjson.Get(rec.Body.String(), "choices.0.message.reasoning_content").String())
	require.Equal(t, "final answer", gjson.Get(rec.Body.String(), "choices.0.message.content").String())
}

func TestForwardAsRawChatCompletions_PreservesDeepSeekReasoningContentStreaming(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"deepseek-reasoner","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_reasoning","object":"chat.completion.chunk","model":"deepseek-reasoner","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_reasoning","object":"chat.completion.chunk","model":"deepseek-reasoner","choices":[{"index":0,"delta":{"reasoning_content":"think first"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_reasoning","object":"chat.completion.chunk","model":"deepseek-reasoner","choices":[{"index":0,"delta":{"content":"final answer"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_reasoning","object":"chat.completion.chunk","model":"deepseek-reasoner","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_deepseek_reasoning_stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.Contains(t, rec.Body.String(), `"reasoning_content":"think first"`)
	require.Contains(t, rec.Body.String(), `"content":"final answer"`)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}

func TestForwardAsRawChatCompletions_PreservesDeepSeekReasoningContentInRequest(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"deepseek-v4-pro","messages":[{"role":"user","content":"weather"},{"role":"assistant","reasoning_content":"need tool","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_1","content":"cloudy"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_deepseek_reasoning_request"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_request","object":"chat.completion","model":"deepseek-v4-pro","choices":[{"index":0,"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "need tool", gjson.GetBytes(upstream.lastBody, "messages.1.reasoning_content").String())
	require.Equal(t, "get_weather", gjson.GetBytes(upstream.lastBody, "messages.1.tool_calls.0.function.name").String())
}

func TestForwardAsRawChatCompletions_NormalizesGLMReasoningEffortForUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"xhigh","stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_glm_effort"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_glm","object":"chat.completion","model":"glm-5.2","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
}

func TestForwardAsRawChatCompletions_SilentRefusalTriggersFailover(t *testing.T) {
	setGinTestMode()

	body := largeRawChatCompletionsBody()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_silent","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		"",
		`data: {"id":"chatcmpl_silent","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_silent"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, IsOpenAISilentRefusalErrorBody(failoverErr.ResponseBody))
	require.Equal(t, "OpenAI upstream returned no visible output before stop", c.GetString(OpsUpstreamErrorMessageKey))
	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
	require.False(t, c.Writer.Written(), "silent refusal must not commit a 200 response before failover")
	require.Empty(t, rec.Body.String())
}

func TestForwardAsRawChatCompletions_SilentRefusalToolCallsExempt(t *testing.T) {
	setGinTestMode()

	body := largeRawChatCompletionsBody()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		"",
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":""}}]}}]}`,
		"",
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_tool"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"tool_calls"`)
	require.Contains(t, rec.Body.String(), `"finish_reason":"tool_calls"`)
}

func TestForwardAsRawChatCompletions_StructuredRefusalExempt(t *testing.T) {
	setGinTestMode()

	body := largeRawChatCompletionsBody()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_refusal","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		"",
		`data: {"id":"chatcmpl_refusal","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"refusal":"I can't help with that."},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_refusal"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"refusal":"I can't help with that."`)
	require.Contains(t, rec.Body.String(), `"finish_reason":"stop"`)
}

func TestHandleChatStreamingResponse_SilentRefusalReasoningSummaryExempt(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_reasoning","model":"gpt-5.5"}}`,
		"",
		`data: {"type":"response.reasoning_summary_text.delta","delta":"thinking only"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_reasoning","model":"gpt-5.5","status":"completed"}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_reasoning"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}

	result, err := svc.handleChatStreamingResponse(
		resp,
		c,
		&Account{ID: 1, Platform: PlatformOpenAI},
		"gpt-5.5",
		"gpt-5.5",
		"gpt-5.5",
		time.Now(),
		openAISilentRefusalMinRequestBodyBytes,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"reasoning_content":"thinking only"`)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}

func TestHandleChatStreamingResponse_StructuredRefusalExempt(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_refusal","model":"gpt-5.5","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_refusal","model":"gpt-5.5","status":"completed","output":[{"type":"message","id":"msg_refusal","role":"assistant","status":"completed","content":[{"type":"refusal","refusal":"I can't help with that."}]}]}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_responses_refusal"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}

	result, err := svc.handleChatStreamingResponse(
		resp,
		c,
		&Account{ID: 1, Platform: PlatformOpenAI},
		"gpt-5.5",
		"gpt-5.5",
		"gpt-5.5",
		time.Now(),
		openAISilentRefusalMinRequestBodyBytes,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "I can't help with that.")
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}

func TestHandleChatStreamingResponse_SilentRefusalEmptyCompletedTriggersFailover(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_silent","model":"gpt-5.5","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_silent","model":"gpt-5.5","status":"completed","output":[]}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"rid_responses_silent"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}

	result, err := svc.handleChatStreamingResponse(
		resp,
		c,
		&Account{ID: 1, Name: "openai-oauth", Platform: PlatformOpenAI},
		"gpt-5.5",
		"gpt-5.5",
		"gpt-5.5",
		time.Now(),
		openAISilentRefusalMinRequestBodyBytes,
	)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, IsOpenAISilentRefusalErrorBody(failoverErr.ResponseBody))
	require.False(t, c.Writer.Written(), "silent refusal must not commit a 200 response before failover")
	require.Empty(t, rec.Body.String())
	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
	require.Equal(t, "rid_responses_silent", events[0].UpstreamRequestID)
}

func TestHandleChatStreamingResponse_SilentRefusalEmptyCompletedWithUsageTriggersFailover(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_silent_usage","model":"gpt-5.5","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_silent_usage","model":"gpt-5.5","status":"completed","output":[],"usage":{"input_tokens":21,"output_tokens":0}}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"rid_responses_silent_usage"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}

	result, err := svc.handleChatStreamingResponse(
		resp,
		c,
		&Account{ID: 1, Name: "openai-oauth", Platform: PlatformOpenAI},
		"gpt-5.5",
		"gpt-5.5",
		"gpt-5.5",
		time.Now(),
		openAISilentRefusalMinRequestBodyBytes,
	)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, IsOpenAISilentRefusalErrorBody(failoverErr.ResponseBody))
	require.False(t, c.Writer.Written(), "usage-only terminal events must not release an empty 200 response")
	require.Empty(t, rec.Body.String())
}

func TestForwardAsRawChatCompletions_SilentRefusalReasoningContentExempt(t *testing.T) {
	setGinTestMode()

	body := largeRawChatCompletionsBody()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_reasoning","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		"",
		`data: {"id":"chatcmpl_reasoning","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"reasoning_content":"thinking only"}}]}`,
		"",
		`data: {"id":"chatcmpl_reasoning","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_reasoning_raw"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"reasoning_content":"thinking only"`)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}

func TestForwardAsRawChatCompletions_SilentRefusalNormalContentExempt(t *testing.T) {
	setGinTestMode()

	body := largeRawChatCompletionsBody()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_ok","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		"",
		`data: {"id":"chatcmpl_ok","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
		"",
		`data: {"id":"chatcmpl_ok","object":"chat.completion.chunk","model":"gpt-5.5","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_ok"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"content":"ok"`)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}

func TestForwardAsRawChatCompletions_ClientDisconnectDrainsUsage(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Writer = &openAIChatFailingWriter{ResponseWriter: c.Writer, failAfter: 0}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
		"",
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.4","choices":[],"usage":{"prompt_tokens":17,"completion_tokens":8,"total_tokens":25,"prompt_tokens_details":{"cached_tokens":6}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_raw_disconnect"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 17, result.Usage.InputTokens)
	require.Equal(t, 8, result.Usage.OutputTokens)
	require.Equal(t, 6, result.Usage.CacheReadInputTokens)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
}

func TestForwardAsRawChatCompletions_UpstreamRequestIgnoresClientCancel(t *testing.T) {
	setGinTestMode()

	reqCtx, cancel := context.WithCancel(context.Background())
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(reqCtx)
	c.Request.Header.Set("Content-Type", "application/json")
	cancel()

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.4","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_raw_ctx"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.forwardAsRawChatCompletions(reqCtx, c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.NoError(t, upstream.lastReq.Context().Err())
}

func TestForwardAsRawChatCompletions_FilteredServiceTierDoesNotLeakIntoBillingResult(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false,"service_tier":"priority"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_raw_filtered_tier"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_filtered","object":"chat.completion","model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)),
	}}

	svc := newOpenAIGatewayServiceWithSettings(t, openAIFastFilterPriorityPolicy())
	svc.cfg = rawChatCompletionsTestConfig()
	svc.httpUpstream = upstream

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.False(t, gjson.GetBytes(upstream.lastBody, "service_tier").Exists(), "filtered tier must not reach upstream")
	require.Nil(t, result.ServiceTier, "billing metadata must follow the final upstream body")
}

func TestForwardAsChatCompletions_UnknownResponsesSupportFallbackUsesVersionedChatURL(t *testing.T) {
	setGinTestMode()

	body := []byte(`{"model":"glm-4.5-air","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"not found"}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_raw_fallback"}},
			Body: io.NopCloser(strings.NewReader(
				`{"id":"chatcmpl_1","object":"chat.completion","model":"glm-4.5-air","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
			)),
		},
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()
	account.Credentials["base_url"] = "https://open.bigmodel.cn/api/paas/v4"

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4/responses", upstream.requests[0].URL.String())
	require.Equal(t, "https://open.bigmodel.cn/api/paas/v4/chat/completions", upstream.requests[1].URL.String())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"content":"ok"`)
}

func TestIsOpenAIChatUsageOnlyStreamChunk(t *testing.T) {
	t.Parallel()

	require.True(t, isOpenAIChatUsageOnlyStreamChunk(`{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	require.False(t, isOpenAIChatUsageOnlyStreamChunk(`{"choices":[{"index":0}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	require.False(t, isOpenAIChatUsageOnlyStreamChunk(`{"choices":[]}`))
	require.False(t, isOpenAIChatUsageOnlyStreamChunk(``))
}

func TestEnsureOpenAIChatStreamUsage(t *testing.T) {
	t.Parallel()

	body, err := ensureOpenAIChatStreamUsage([]byte(`{"model":"gpt-5.4"}`))
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(body, "stream_options.include_usage").Bool())

	body, err = ensureOpenAIChatStreamUsage([]byte(`{"model":"gpt-5.4","stream_options":{"include_usage":false}}`))
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(body, "stream_options.include_usage").Bool())
}

func TestBufferRawChatCompletions_RejectsOversizedResponse(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("toolong")),
	}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	svc.cfg.Gateway.UpstreamResponseReadMaxBytes = 3

	result, err := svc.bufferRawChatCompletions(c, resp, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5.4", "gpt-5.4", "gpt-5.4", nil, nil, time.Now())
	require.ErrorIs(t, err, ErrUpstreamResponseBodyTooLarge)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
}

func rawChatCompletionsTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: true,
			},
		},
	}
}

func rawChatCompletionsTestAccount() *Account {
	return &Account{
		ID:          101,
		Name:        "raw-openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
		},
	}
}

func largeRawChatCompletionsBody() []byte {
	return []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"` +
		strings.Repeat("x", openAISilentRefusalMinRequestBodyBytes) +
		`"}],"stream":true}`)
}
