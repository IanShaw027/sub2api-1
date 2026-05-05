package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeResponsesRequestServiceTier(t *testing.T) {
	t.Parallel()

	req := &apicompat.ResponsesRequest{ServiceTier: " fast "}
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "priority", req.ServiceTier)

	req.ServiceTier = "flex"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "flex", req.ServiceTier)

	req.ServiceTier = "default"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "default", req.ServiceTier)
}

func TestNormalizeResponsesBodyServiceTier(t *testing.T) {
	t.Parallel()

	body, tier, err := normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"fast"}`))
	require.NoError(t, err)
	require.Equal(t, "priority", tier)
	require.Equal(t, "priority", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"flex"}`))
	require.NoError(t, err)
	require.Equal(t, "flex", tier)
	require.Equal(t, "flex", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"default"}`))
	require.NoError(t, err)
	require.Equal(t, "default", tier)
	require.Equal(t, "default", gjson.GetBytes(body, "service_tier").String())
}

func TestHandleCompatErrorResponseDoesNotExposeUpstreamMessageByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	account := &Account{ID: 1, Name: "openai-oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{"x-request-id": []string{"rid-sensitive"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"sensitive prompt fragment access_token=secret"}}`)),
	}

	_, err := svc.handleCompatErrorResponse(resp, c, account, writeChatCompletionsError)
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.NotContains(t, rec.Body.String(), "sensitive prompt fragment")
	require.NotContains(t, rec.Body.String(), "secret")
	require.Contains(t, rec.Body.String(), "Upstream authentication failed")
}

func TestForwardAsChatCompletions_OAuth_OrdersPromptCacheKeyBeforeInputAfterCodexTransform(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1-codex","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
	c.Request.Header.Set("Originator", "codex_exec")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_ordered"}},
		Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.1-codex"))),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "session-ordered", "gpt-5.1", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	bodyStr := string(upstream.lastBody)
	modelPos := strings.Index(bodyStr, `"model":"gpt-5.1-codex"`)
	promptCachePos := strings.Index(bodyStr, `"prompt_cache_key":"session-ordered"`)
	inputPos := strings.Index(bodyStr, `"input":`)
	require.NotEqual(t, -1, modelPos)
	require.NotEqual(t, -1, promptCachePos)
	require.NotEqual(t, -1, inputPos)
	require.Less(t, modelPos, promptCachePos)
	require.Less(t, promptCachePos, inputPos)
	require.Equal(t, "session-ordered", upstream.lastReq.Header.Get("Session_Id"))
}

func TestForwardAsChatCompletions_APIKey_IncludesPromptCacheKeyInUpstreamBody(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_apikey_prompt_cache"}},
		Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.1"))),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, " session-apikey ", "gpt-5.1", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "session-apikey", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
}

func TestForwardAsChatCompletions_APIKey_ReplacesNonStringPromptCacheKeyInUpstreamBody(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],"prompt_cache_key":123,"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_apikey_prompt_cache_type"}},
		Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.1"))),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled: false,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "session-apikey", "gpt-5.1", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "session-apikey", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
	require.Equal(t, `"session-apikey"`, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").Raw)
}

func TestForwardAsChatCompletions_APIKey_DoesNotInjectDefaultInstructionsInUpstreamBody(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_apikey_instructions"}},
		Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.1"))),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled: false,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, gjson.GetBytes(upstream.lastBody, "instructions").Exists())
}

func TestForwardAsChatCompletions_APIKeyResponsesShape_PreservesLargeIntegerWithoutMapRoundTrip(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],"max_output_tokens":9007199254740993,"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_apikey_precision"}},
		Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.1"))),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled: false,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.JSONEq(t, string(body), string(upstream.lastBody))
	require.Equal(t, "9007199254740993", gjson.GetBytes(upstream.lastBody, "max_output_tokens").Raw)
}

func TestForwardAsChatCompletions_APIKeyUnsupportedResponsesFallsBackToRawChatCompletions(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1-codex","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_apikey_raw_fallback"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_raw","object":"chat.completion","created":1730000000,"model":"gpt-5.4-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled: false,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-apikey-raw",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://upstream.example",
			"model_mapping": map[string]any{
				"gpt-5.4": "gpt-5.4-mini",
			},
		},
		Extra: map[string]any{
			"openai_responses_supported": false,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "session-raw", "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.Equal(t, "gpt-5.4-mini", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "session-raw", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
}

func TestForwardAsChatCompletions_APIKeyUnsupportedResponsesShapeStillUsesResponsesPath(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1-codex","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_responses_shape"}},
		Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.1"))),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled: false,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-apikey-responses-shape",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://upstream.example",
		},
		Extra: map[string]any{
			"openai_responses_supported": false,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "session-responses", "gpt-5.1", "gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "/v1/responses", upstream.lastReq.URL.Path)
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
}

func TestForwardAsChatCompletions_OAuthResponsesShape_IncludesDefaultInstructionsInUpstreamBody(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1-codex","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_oauth_responses_shape"}},
		Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.1"))),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled: false,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.1", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
}

func TestForwardAsChatCompletions_OAuth_SelectedFallbackModelOverridesExplicitCodexRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.1-codex","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_selected_fallback"}},
		Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.4-mini"))),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled: false,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
			"model_mapping": map[string]any{
				"gpt-5.4": "gpt-5.4-mini",
			},
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.4-mini", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestForwardAsChatCompletions_OAuth_RetriesCodexCompatFallbackOnce(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","input":[{"type":"message","id":"msg_123","role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "codex-tui/0.125.0")

	upstream := &httpUpstreamSequenceRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid schema for field input[0].id"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_compat_retry"}},
				Body:       io.NopCloser(strings.NewReader(testResponsesCompletedSSE("gpt-5.4"))),
			},
		},
	}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{Enabled: false},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          11,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.callCount)
	require.Equal(t, "msg_123", gjson.GetBytes(upstream.bodies[0], "input.0.id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "input.0.id").Exists())
}

func testResponsesCompletedSSE(model string) string {
	return strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_chat_1","object":"response","model":"` + model + `","status":"completed","output":[{"type":"message","id":"msg_chat_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
}

func TestHandleChatStreamingResponse_ResponseFailedEmitsErrorSignalWithoutDone(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_failed_stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_failed_stream"}}`,
			`data: {"type":"response.failed","response":{"status":"failed","error":{"message":"upstream stream failed"}}}`,
			`data: [DONE]`,
			"",
		}, "\n"))),
	}

	svc := &OpenAIGatewayService{}
	result, err := svc.handleChatStreamingResponse(resp, c, "gpt-5.1", "gpt-5.1", "gpt-5.1", false, time.Now())
	require.Error(t, err)
	require.NotNil(t, result)
	require.Contains(t, err.Error(), "upstream response failed")
	require.Contains(t, rec.Body.String(), `"error":{"message":"upstream stream failed","type":"upstream_error"}`)
	require.NotContains(t, rec.Body.String(), "data: [DONE]")
}

func TestHandleChatBufferedStreamingResponse_ResponseFailedReturnsJSONError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_failed_buffered"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.failed","response":{"status":"failed","error":{"message":"buffered upstream failed"}}}`,
			`data: [DONE]`,
			"",
		}, "\n"))),
	}

	svc := &OpenAIGatewayService{}
	result, err := svc.handleChatBufferedStreamingResponse(resp, c, "gpt-5.1", "gpt-5.1", "gpt-5.1", time.Now())
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"upstream_error"`)
	require.Contains(t, rec.Body.String(), "buffered upstream failed")
}
