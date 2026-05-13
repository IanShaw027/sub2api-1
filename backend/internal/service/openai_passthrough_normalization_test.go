package service

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAIPassthroughOAuthBody_RemovesUnsupportedUser(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","temperature":0.2,"user":"user_123","metadata":{"user_id":"user_123"},"prompt_cache_retention":"24h","safety_identifier":"sid","stream_options":{"include_usage":true}}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	for _, field := range openAIChatGPTInternalUnsupportedFields {
		require.False(t, gjson.GetBytes(normalized, field).Exists(), "%s should be stripped", field)
	}
	require.True(t, gjson.GetBytes(normalized, "stream").Bool())
	require.False(t, gjson.GetBytes(normalized, "store").Bool())
}

func TestNormalizeOpenAIPassthroughOAuthBody_CompactRemovesUnsupportedUser(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","user":"user_123","metadata":{"user_id":"user_123"},"stream":true,"store":true}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, true)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "user").Exists())
	require.False(t, gjson.GetBytes(normalized, "metadata").Exists())
	require.False(t, gjson.GetBytes(normalized, "stream").Exists())
	require.False(t, gjson.GetBytes(normalized, "store").Exists())
}

func TestNormalizeOpenAIPassthroughOAuthBody_PreservesPromptCacheFriendlyOrdering(t *testing.T) {
	body := []byte(`{"model":"gpt-5.2","stream":false,"store":true,"prompt_cache_key":"cache-ordered-123","instructions":"local-test-instructions","input":"hello ordered world"}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	requireOrderedJSONKeys(t, normalized, "model", "instructions", "prompt_cache_key", "input")
	require.True(t, gjson.GetBytes(normalized, "stream").Bool())
	require.False(t, gjson.GetBytes(normalized, "store").Bool())
	require.Equal(t, "hello ordered world", gjson.GetBytes(normalized, "input.0.content").String())
}

func TestNormalizeOpenAIPassthroughBaseBody_StripsTopPWhenRequested(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","top_p":0.8}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, true)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "top_p").Exists())
}

func TestNormalizeOpenAIPassthroughBaseBody_PreservesTopPWhenNotRequested(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","top_p":0.8}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, 0.8, gjson.GetBytes(normalized, "top_p").Float())
}

func TestNormalizeOpenAIPassthroughOAuthBody_NormalizesToolRoleInput(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"previous_response_id":"resp_123","input":[{"type":"message","role":"user","content":"hi"},{"role":"tool","tool_call_id":"call_123","content":"done"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "function_call_output", gjson.GetBytes(normalized, "input.1.type").String())
	require.Equal(t, "call_123", gjson.GetBytes(normalized, "input.1.call_id").String())
	require.Equal(t, "done", gjson.GetBytes(normalized, "input.1.output").String())
	require.False(t, gjson.GetBytes(normalized, "input.1.role").Exists())
}

func TestNormalizeOpenAIPassthroughOAuthBody_DowngradesOrphanToolOutputWithoutContext(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"input":[{"role":"tool","tool_call_id":"call_123","content":"done"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "message", gjson.GetBytes(normalized, "input.0.type").String())
	require.Equal(t, "user", gjson.GetBytes(normalized, "input.0.role").String())
	require.Equal(t, "done", gjson.GetBytes(normalized, "input.0.content").String())
	require.False(t, gjson.GetBytes(normalized, "input.0.call_id").Exists())
}

func TestNormalizeOpenAIPassthroughOAuthBody_DowngradesOrphanFunctionCallOutputWithoutContext(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"input":[{"type":"function_call_output","call_id":"call_123","output":"done"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "message", gjson.GetBytes(normalized, "input.0.type").String())
	require.Equal(t, "user", gjson.GetBytes(normalized, "input.0.role").String())
	require.Equal(t, "done", gjson.GetBytes(normalized, "input.0.content").String())
	require.False(t, gjson.GetBytes(normalized, "input.0.call_id").Exists())
}

func TestNormalizeOpenAIPassthroughOAuthBody_PreservesFunctionCallOutputWithItemReference(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"input":[{"type":"item_reference","id":"call_123"},{"type":"function_call_output","call_id":"call_123","output":"done"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	_ = changed
	require.JSONEq(t, string(body), string(normalized))
	require.Equal(t, "item_reference", gjson.GetBytes(normalized, "input.0.type").String())
	require.Equal(t, "call_123", gjson.GetBytes(normalized, "input.0.id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(normalized, "input.1.type").String())
	require.Equal(t, "call_123", gjson.GetBytes(normalized, "input.1.call_id").String())
}

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_EnforcesFinalOAuthResponsesConstraints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
	body := []byte(`{"model":"gpt-5.5","stream":true,"max_output_tokens":8192}`)

	finalBody, changed, err := finalizeOpenAIResponsesOAuthUpstreamBody(c, account, "gpt-5.5", body)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(finalBody, "max_output_tokens").Exists())
	require.True(t, gjson.GetBytes(finalBody, "instructions").Exists())
	require.NotEmpty(t, gjson.GetBytes(finalBody, "instructions").String())
	require.True(t, gjson.GetBytes(finalBody, "store").Exists())
	require.False(t, gjson.GetBytes(finalBody, "store").Bool())
	require.True(t, gjson.GetBytes(finalBody, "stream").Bool())
}

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_CompactSkipsDefaultInstructions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(nil))

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
	body := []byte(`{"model":"gpt-5.5","stream":true,"store":true,"max_output_tokens":64}`)

	finalBody, changed, err := finalizeOpenAIResponsesOAuthUpstreamBody(c, account, "gpt-5.5", body)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(finalBody, "max_output_tokens").Exists())
	require.False(t, gjson.GetBytes(finalBody, "store").Exists())
	require.False(t, gjson.GetBytes(finalBody, "stream").Exists())
	require.False(t, gjson.GetBytes(finalBody, "instructions").Exists())
}

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_ContextMarkedMessagesBridgeSkipsDefaultInstructions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	setOpenAICompatMessagesBridgeContext(c, true)

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
	body := []byte(`{"model":"gpt-5.5","stream":true,"store":false,"input":[{"type":"message","role":"user","content":"hello"}]}`)

	finalBody, changed, err := finalizeOpenAIResponsesOAuthUpstreamBody(c, account, "gpt-5.5", body)
	require.NoError(t, err)
	require.False(t, changed)
	require.False(t, gjson.GetBytes(finalBody, "instructions").Exists())
}
