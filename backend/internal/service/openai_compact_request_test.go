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

func TestNormalizeOpenAICompactRequestBodyForTest_UsesCodexShape(t *testing.T) {
	t.Parallel()

	original := []byte(`{
		"model":"gpt-5.4",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"compact me"}]}],
		"instructions":"local-test-instructions",
		"tools":[{"type":"function","function":{"name":"apply_patch"}}],
		"parallel_tool_calls":true,
		"reasoning":{"effort":"high"},
		"text":{"verbosity":"low"},
		"prompt_cache_key":"cache-123",
		"metadata":{"source":"test"},
		"previous_response_id":"resp_123",
		"stream":true,
		"store":true
	}`)

	normalized, changed, err := NormalizeOpenAICompactRequestBodyForTest(original)
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, `{
		"model":"gpt-5.4",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"compact me"}]}],
		"instructions":"local-test-instructions",
		"tools":[{"type":"function","function":{"name":"apply_patch"}}],
		"parallel_tool_calls":true,
		"previous_response_id":"resp_123",
		"reasoning":{"effort":"high"},
		"text":{"verbosity":"low"}
	}`, string(normalized))
}

func TestNormalizeOpenAICompactRequestBodyForTest_AlreadyNormalized(t *testing.T) {
	t.Parallel()

	original := []byte(`{"model":"gpt-5.4","input":[],"instructions":"local-test-instructions","tools":[],"parallel_tool_calls":false}`)

	normalized, changed, err := NormalizeOpenAICompactRequestBodyForTest(original)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, string(original), string(normalized))
}

func TestNormalizeOpenAICompactRequestBodyForTest_AddsToolSearchForDeferredTools(t *testing.T) {
	t.Parallel()

	original := []byte(`{
		"model":"gpt-5.5",
		"input":[],
		"tools":[{"type":"function","name":"bash","defer_loading":true}]
	}`)

	normalized, changed, err := NormalizeOpenAICompactRequestBodyForTest(original)
	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, gjson.GetBytes(normalized, "tools.0.defer_loading").Bool())
	require.Equal(t, "tool_search", gjson.GetBytes(normalized, "tools.1.type").String())
}

func TestNormalizeOpenAICompactRequestBodyForTest_PreservesExistingToolSearch(t *testing.T) {
	t.Parallel()

	original := []byte(`{
		"model":"gpt-5.5",
		"input":[],
		"tools":[{"type":"function","name":"bash","defer_loading":true},{"type":"tool_search","mode":"existing"}]
	}`)

	normalized, changed, err := NormalizeOpenAICompactRequestBodyForTest(original)
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, string(original), string(normalized))
}

func TestNormalizeOpenAICompactRequestBodyForTest_AddsToolSearchForObjectDeferredTools(t *testing.T) {
	t.Parallel()

	original := []byte(`{
		"model":"gpt-5.5",
		"input":[],
		"tools":{"defer_loading":true}
	}`)

	normalized, changed, err := NormalizeOpenAICompactRequestBodyForTest(original)
	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, gjson.GetBytes(normalized, "tools.defer_loading").Bool())
	require.Equal(t, "tool_search", gjson.GetBytes(normalized, "tools.tool_search.type").String())
}

func TestOpenAIGatewayService_Forward_OAuthCompactUsesCodexShape(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model":"gpt-5.4",
		"instructions":"local-test-instructions",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"compact me"}]}],
		"tools":[{"type":"function","function":{"name":"apply_patch"}}],
		"parallel_tool_calls":true,
		"reasoning":{"effort":"high"},
		"text":{"verbosity":"low"},
		"prompt_cache_key":"cache-123",
		"metadata":{"source":"test"},
		"max_output_tokens":64,
		"stream":true,
		"store":true
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(body))
	c.Request.Header.Set("Accept", "*/*")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Session_Id", "cache-123")
	c.Request.Header.Set("X-Codex-Installation-Id", "inst-123")
	c.Request.Header.Set("X-Codex-Window-Id", "cache-123:0")
	c.Request.Header.Set("Originator", "codex_exec")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-compact"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"cmp_123","model":"gpt-5.4","usage":{"input_tokens":11,"output_tokens":22}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{CodexImageGenerationBridgeEnabled: true}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          123,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, chatgptCodexURL+"/compact", upstream.lastReq.URL.String())
	require.Equal(t, "*/*", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, "cache-123", upstream.lastReq.Header.Get("Session_Id"))
	require.Empty(t, upstream.lastReq.Header.Get("Conversation_Id"))
	require.Empty(t, upstream.lastReq.Header.Get("Version"))
	require.Empty(t, upstream.lastReq.Header.Get("OpenAI-Beta"))
	require.Equal(t, "inst-123", upstream.lastReq.Header.Get("X-Codex-Installation-Id"))
	require.Equal(t, "cache-123:0", upstream.lastReq.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, "codex_exec", upstream.lastReq.Header.Get("Originator"))

	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "compact me", gjson.GetBytes(upstream.lastBody, "input.0.content.0.text").String())
	instructions := gjson.GetBytes(upstream.lastBody, "instructions").String()
	require.Contains(t, instructions, "local-test-instructions")
	require.Contains(t, instructions, codexImageGenerationBridgeMarker)
	require.Equal(t, "apply_patch", gjson.GetBytes(upstream.lastBody, "tools.0.function.name").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "parallel_tool_calls").Bool())
	require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.Equal(t, "low", gjson.GetBytes(upstream.lastBody, "text.verbosity").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "metadata").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "max_output_tokens").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Exists())
}
