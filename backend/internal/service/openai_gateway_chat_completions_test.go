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
	modelPos := strings.Index(bodyStr, `"model":"gpt-5.3-codex"`)
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

func TestForwardAsChatCompletions_BridgesImageOnlyModelToImagesAPI(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-image-2","messages":[{"role":"user","content":"draw a cat"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-image-chat"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1710000010,"usage":{"input_tokens":7,"output_tokens":11,"input_tokens_details":{"cached_tokens":2}},"data":[{"b64_json":"aGVsbG8="}]}`)),
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
		ID:       1,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-image-2", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "/v1/images/generations", upstream.lastReq.URL.Path)
	require.Equal(t, "draw a cat", gjson.GetBytes(upstream.lastBody, "prompt").String())
	require.Equal(t, "gpt-image-2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "image_url", gjson.Get(rec.Body.String(), "choices.0.message.content.0.type").String())
	require.Equal(t, "data:image/png;base64,aGVsbG8=", gjson.Get(rec.Body.String(), "choices.0.message.content.0.image_url.url").String())
	require.Equal(t, int64(7), gjson.Get(rec.Body.String(), "usage.prompt_tokens").Int())
	require.Equal(t, int64(11), gjson.Get(rec.Body.String(), "usage.completion_tokens").Int())
	require.Equal(t, int64(2), gjson.Get(rec.Body.String(), "usage.prompt_tokens_details.cached_tokens").Int())
}

func TestForwardAsChatCompletions_BridgesImageOnlyModelEditsToImagesAPI(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-image-2","messages":[{"role":"user","content":[{"type":"text","text":"replace background"},{"type":"image_url","image_url":{"url":"https://example.com/source.png"}}]}],"stream":false,"background":"transparent"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-image-edit-chat"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1710000011,"data":[{"b64_json":"ZWRpdGVk"}]}`)),
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
		ID:       1,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-image-2", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "/v1/images/edits", upstream.lastReq.URL.Path)
	require.Equal(t, "replace background", gjson.GetBytes(upstream.lastBody, "prompt").String())
	require.Equal(t, "https://example.com/source.png", gjson.GetBytes(upstream.lastBody, "images.0.image_url").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "background").Exists())
	require.Equal(t, "data:image/png;base64,ZWRpdGVk", gjson.Get(rec.Body.String(), "choices.0.message.content.0.image_url.url").String())
}

func TestForwardAsChatCompletions_StreamsImageOnlyModelAsSyntheticChatSSE(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-image-2","messages":[{"role":"user","content":"draw a cat"}],"stream":true,"stream_options":{"include_usage":true}}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-image-stream-chat"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1710000012,"usage":{"input_tokens":9,"output_tokens":13,"input_tokens_details":{"cached_tokens":4}},"data":[{"b64_json":"c3RyZWFtZWQ="}]}`)),
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
		ID:       1,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-image-2", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, "/v1/images/generations", upstream.lastReq.URL.Path)
	bodyText := rec.Body.String()
	require.Contains(t, bodyText, `"role":"assistant"`)
	require.Contains(t, bodyText, `"content":"data:image/png;base64,c3RyZWFtZWQ="`)
	require.Contains(t, bodyText, `"prompt_tokens":9`)
	require.Contains(t, bodyText, `"completion_tokens":13`)
	require.Contains(t, bodyText, `"cached_tokens":4`)
	require.Contains(t, bodyText, "data: [DONE]")
}

func TestForwardAsChatCompletions_StreamsMultipleImagesAsSeparateContentChunks(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-image-2","messages":[{"role":"user","content":"draw two cats"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-image-multi-stream-chat"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1710000013,"data":[{"b64_json":"Zmlyc3Q="},{"b64_json":"c2Vjb25k"}]}`)),
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
		ID:       1,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-image-2", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	bodyText := rec.Body.String()
	require.Contains(t, bodyText, `"content":"data:image/png;base64,Zmlyc3Q="`)
	require.Contains(t, bodyText, `"content":"\ndata:image/png;base64,c2Vjb25k"`)
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

func TestForwardAsChatCompletions_APIKeyUnsupportedResponsesShapeFallsBackToRawChatCompletions(t *testing.T) {
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
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.True(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
	require.Equal(t, "user", gjson.GetBytes(upstream.lastBody, "messages.0.role").String())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
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

func TestForwardAsChatCompletions_OAuth_RetriesCodexCompatFallbackOnceOnMissingToolCall(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","input":[{"type":"item_reference","id":"call_1"},{"type":"function_call_output","call_id":"call_1","output":"ok"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "codex-tui/0.125.0")

	upstream := &httpUpstreamSequenceRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"error":{"message":"No tool call found for function call output with call_id call_1.","type":"invalid_request_error","param":"input"}}`,
				)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_compat_retry_tool_call"}},
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
		ID:          12,
		Name:        "openai-oauth-tool-call",
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
	require.Equal(t, "call_1", gjson.GetBytes(upstream.bodies[0], "input.0.id").String())
	require.Equal(t, "call_1", gjson.GetBytes(upstream.bodies[0], "input.1.call_id").String())
	require.Equal(t, "fc1", gjson.GetBytes(upstream.bodies[1], "input.0.id").String())
	require.Equal(t, "fc1", gjson.GetBytes(upstream.bodies[1], "input.1.call_id").String())
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

func TestHandleChatStreamingResponse_ResponseFailedSuppressesTransientCapacityMessage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_failed_capacity_stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_failed_stream_capacity"}}`,
			`data: {"type":"response.failed","response":{"status":"failed","error":{"message":"Selected model is at capacity. Please try a different model."}}}`,
			`data: [DONE]`,
			"",
		}, "\n"))),
	}

	svc := &OpenAIGatewayService{}
	result, err := svc.handleChatStreamingResponse(resp, c, "gpt-5.1", "gpt-5.1", "gpt-5.1", false, time.Now())
	require.Error(t, err)
	require.NotNil(t, result)
	require.Contains(t, err.Error(), "upstream response failed")
	require.Contains(t, rec.Body.String(), `"message":"Upstream service temporarily unavailable"`)
	require.NotContains(t, rec.Body.String(), "Selected model is at capacity")
	require.NotContains(t, rec.Body.String(), "data: [DONE]")
}

func TestHandleChatBufferedStreamingResponse_ResponseFailedSuppressesTransientCapacityMessage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_failed_capacity_buffered"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.failed","response":{"status":"failed","error":{"message":"Selected model is at capacity. Please try a different model."}}}`,
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
	require.Contains(t, rec.Body.String(), "Upstream service temporarily unavailable")
	require.NotContains(t, rec.Body.String(), "Selected model is at capacity")
}
