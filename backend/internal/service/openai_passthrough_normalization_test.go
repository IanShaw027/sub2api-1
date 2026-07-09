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
	body := []byte(`{"model":"gpt-5.4","input":"hello","name":"request-name","temperature":0.2,"user":"user_123","metadata":{"user_id":"user_123"},"prompt_cache_retention":"24h","safety_identifier":"sid","stream_options":{"include_usage":true}}`)

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

func TestNormalizeOpenAIPassthroughBaseBody_DropsNoneTypedTools(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","tools":[{"type":"None"},{"type":null},{"name":"run","parameters":{"type":"object"}}],"input":[{"type":"message","role":"user","content":"hi"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "function", gjson.GetBytes(normalized, "tools.0.type").String())
	require.Equal(t, "run", gjson.GetBytes(normalized, "tools.0.name").String())
	require.Equal(t, 1, len(gjson.GetBytes(normalized, "tools").Array()))
}

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_DropsWebSearchPreviewWithoutDeferredTool(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
	body := []byte(`{"model":"gpt-5.5","stream":false,"input":[{"type":"message","role":"user","content":"search later"}],"tools":[{"type":"web_search_preview"}]}`)

	finalBody, changed, err := finalizeOpenAIResponsesOAuthUpstreamBody(c, account, "gpt-5.5", body)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(finalBody, `tools.#(type=="tool_search")`).Exists())
	require.False(t, gjson.GetBytes(finalBody, `tools.#(type=="web_search_preview")`).Exists())
}

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_DropsToolSearchWithoutDeferredTool(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
	body := []byte(`{"model":"gpt-5.5","stream":false,"input":[{"type":"message","role":"user","content":"hello"}],"tools":[{"type":"tool_search"}]}`)

	finalBody, changed, err := finalizeOpenAIResponsesOAuthUpstreamBody(c, account, "gpt-5.5", body)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(finalBody, `tools.#(type=="tool_search")`).Exists())
}

func TestNormalizeOpenAIPassthroughBaseBody_MapsWebSearchPreviewTool(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","tools":[{"type":"function","name":"late_bound","defer_loading":true,"parameters":{"type":"object"}},{"type":"web_search_preview"},{"type":"web_search_preview_2025_03_11"}],"input":[{"type":"message","role":"user","content":"hi"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "function", gjson.GetBytes(normalized, "tools.0.type").String())
	require.True(t, gjson.GetBytes(normalized, "tools.0.defer_loading").Bool())
	require.Equal(t, "tool_search", gjson.GetBytes(normalized, "tools.1.type").String())
	require.Equal(t, "tool_search", gjson.GetBytes(normalized, "tools.2.type").String())
}

func TestNormalizeOpenAIPassthroughBaseBody_StripsAdditionalCodexUnsupportedFields(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","max_tokens":64,"max_tool_calls":8,"web_search":{"enabled":true},"input":[{"type":"message","role":"user","content":"hi"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)

	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "max_tokens").Exists())
	require.False(t, gjson.GetBytes(normalized, "max_tool_calls").Exists())
	require.False(t, gjson.GetBytes(normalized, "web_search").Exists())
}

func TestNormalizeOpenAIPassthroughBaseBody_NormalizesStructuredToolCallArgumentsString(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"tool_search_call","call_id":"call_search","arguments":"{\"query\":\"golang\",\"limit\":5}"},{"type":"function_call","call_id":"call_fn","name":"run","arguments":"{\"cmd\":\"pwd\"}"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "golang", gjson.GetBytes(normalized, "input.0.arguments.query").String())
	require.Equal(t, int64(5), gjson.GetBytes(normalized, "input.0.arguments.limit").Int())
	require.Equal(t, `{"cmd":"pwd"}`, gjson.GetBytes(normalized, "input.1.arguments").String())
}

func TestNormalizeOpenAIPassthroughOAuthBody_NormalizesStructuredToolCallArgumentsString(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"custom_tool_call","call_id":"call_custom","name":"run","arguments":"not json"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "not json", gjson.GetBytes(normalized, "input.0.arguments.value").String())
}

func TestNormalizeOpenAIPassthroughBaseBody_PreservesTopPWhenNotRequested(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","top_p":0.8}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, 0.8, gjson.GetBytes(normalized, "top_p").Float())
}

func TestNormalizeOpenAIPassthroughBaseBody_StripsTopLevelUnsupportedCompatFields(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":"hello"}],"verbosity":"low","enable_thinking":true,"stop_sequences":["END"],"promptCacheKey":"legacy-cache","text":{"verbosity":"low"},"prompt_cache_key":"cache-ok"}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "verbosity").Exists())
	require.False(t, gjson.GetBytes(normalized, "enable_thinking").Exists())
	require.False(t, gjson.GetBytes(normalized, "stop_sequences").Exists())
	require.False(t, gjson.GetBytes(normalized, "promptCacheKey").Exists())
	require.Equal(t, "low", gjson.GetBytes(normalized, "text.verbosity").String())
	require.Equal(t, "cache-ok", gjson.GetBytes(normalized, "prompt_cache_key").String())
}

func TestNormalizeOpenAIPassthroughBaseBody_StripsOnlyToolSchemaLookaroundPatterns(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":"hello"}],"tools":[{"type":"function","name":"read_file","parameters":{"type":"object","properties":{"fileKey":{"type":"string","pattern":"^(?!/tmp/).+$"},"safeKey":{"type":"string","pattern":"^[A-Za-z0-9_-]+$"},"nested":{"type":"object","properties":{"child":{"type":"string","pattern":"(?<=prefix).+"}}}}}}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "tools.0.parameters.properties.fileKey.pattern").Exists())
	require.False(t, gjson.GetBytes(normalized, "tools.0.parameters.properties.nested.properties.child.pattern").Exists())
	require.Equal(t, "^[A-Za-z0-9_-]+$", gjson.GetBytes(normalized, "tools.0.parameters.properties.safeKey.pattern").String())
}

func TestNormalizeOpenAIPassthroughBaseBody_NormalizesResponseFormatSchemaRequired(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","text":{"format":{"type":"json_schema","name":"codex_output_schema","schema":{"type":"object","required":["action_dispatch_maps"],"properties":{"action_dispatch":{"type":"string"},"summary":{"type":"string"}}}}}}`)

	normalized, changed, err := normalizeOpenAIPassthroughBaseBody(body, false, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, `["action_dispatch","summary"]`, gjson.GetBytes(normalized, "text.format.schema.required").Raw)
	require.True(t, gjson.GetBytes(normalized, "text.format.schema.additionalProperties").Exists())
	require.False(t, gjson.GetBytes(normalized, "text.format.schema.additionalProperties").Bool())
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

func TestNormalizeOpenAIPassthroughOAuthBody_NormalizesToolChoiceAndExtractsSystemMessages(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"tools":[{"type":"function","function":{"name":"generate_images","description":"Plan for generating images","parameters":{"type":"object"}}}],"tool_choice":{"type":"function","function":{"name":"generate_images"}},"input":[{"type":"message","role":"system","content":"planner policy"},{"type":"message","role":"user","content":"draw"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "planner policy", gjson.GetBytes(normalized, "instructions").String())
	require.False(t, gjson.GetBytes(normalized, `input.#(role=="system")`).Exists())
	require.Equal(t, "function", gjson.GetBytes(normalized, "tool_choice.type").String())
	require.Equal(t, "generate_images", gjson.GetBytes(normalized, "tool_choice.name").String())
	require.Equal(t, "generate_images", gjson.GetBytes(normalized, "tools.0.name").String())
	require.Equal(t, "Plan for generating images", gjson.GetBytes(normalized, "tools.0.description").String())
	require.Equal(t, "object", gjson.GetBytes(normalized, "tools.0.parameters.type").String())
}

func TestNormalizeOpenAIPassthroughOAuthBody_NormalizesLegacyMessagesIntoInstructionsAndInput(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"messages":[{"role":"system","content":"planner policy"},{"role":"user","content":"draw"}],"tools":[{"type":"function","function":{"name":"generate_images","description":"Plan for generating images","parameters":{"type":"object"}}}],"tool_choice":{"type":"function","function":{"name":"generate_images"}}}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "messages").Exists())
	require.Equal(t, "planner policy", gjson.GetBytes(normalized, "instructions").String())
	require.Equal(t, "developer", gjson.GetBytes(normalized, "input.0.role").String())
	require.Equal(t, "planner policy", gjson.GetBytes(normalized, "input.0.content").String())
	require.Equal(t, "user", gjson.GetBytes(normalized, "input.1.role").String())
	require.Equal(t, "draw", gjson.GetBytes(normalized, "input.1.content").String())
	require.Equal(t, "function", gjson.GetBytes(normalized, "tool_choice.type").String())
	require.Equal(t, "generate_images", gjson.GetBytes(normalized, "tool_choice.name").String())
	require.Equal(t, "generate_images", gjson.GetBytes(normalized, "tools.0.name").String())
}

func TestNormalizeOpenAIPassthroughOAuthBody_MergesLegacyMessagesIntoReplayInput(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"previous_response_id":"resp_123","input":[{"type":"function_call_output","call_id":"call_1","output":"done"}],"messages":[{"role":"system","content":"planner policy"},{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"done"}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "messages").Exists())
	require.False(t, gjson.GetBytes(normalized, "previous_response_id").Exists())
	require.Equal(t, "planner policy", gjson.GetBytes(normalized, "instructions").String())
	require.Equal(t, "developer", gjson.GetBytes(normalized, "input.0.role").String())
	require.Equal(t, "planner policy", gjson.GetBytes(normalized, "input.0.content").String())
	require.Equal(t, "function_call", gjson.GetBytes(normalized, "input.1.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(normalized, "input.1.call_id").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(normalized, "input.2.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(normalized, "input.2.call_id").String())
	require.False(t, gjson.GetBytes(normalized, "input.3").Exists())
}

func TestNormalizeOpenAIPassthroughOAuthBody_NormalizesAssistantMessageContentTypes(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","stream":true,"input":[{"role":"user","type":"message","content":[{"type":"input_text","text":"prompt"}]},{"role":"assistant","type":"message","content":[{"type":"input_text","text":"{\"ok\":true}"}]}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "input_text", gjson.GetBytes(normalized, "input.0.content.0.type").String())
	require.Equal(t, "output_text", gjson.GetBytes(normalized, "input.1.content.0.type").String())
	require.Equal(t, "{\"ok\":true}", gjson.GetBytes(normalized, "input.1.content.0.text").String())
}

func TestNormalizeOpenAIPassthroughOAuthBody_DropsUnpersistedReasoningItemsWhenStoreFalse(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"input":[{"type":"message","role":"user","content":"hi"},{"type":"reasoning","id":"rs_0672f12450da0b9c0169f07220a6c08198b68c2455ced99344","summary":[]}]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, `input.#(type=="reasoning")`).Exists())
	require.Equal(t, "message", gjson.GetBytes(normalized, "input.0.type").String())
}

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_EnforcesFinalOAuthResponsesConstraints(t *testing.T) {
	setGinTestMode()

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

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_CompactInjectsDefaultInstructions(t *testing.T) {
	setGinTestMode()

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
	require.True(t, gjson.GetBytes(finalBody, "instructions").Exists())
	require.NotEmpty(t, gjson.GetBytes(finalBody, "instructions").String())
}

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_NormalizesResponseFormatSchemaRequired(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"text":{"format":{"type":"json_schema","name":"codex_output_schema","schema":{"type":"object","required":["action_dispatch_maps"],"properties":{"action_dispatch":{"type":"string"},"summary":{"type":"string"}}}}}}`)

	finalBody, changed, err := finalizeOpenAIResponsesOAuthUpstreamBody(c, account, "gpt-5.4", body)
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, `["action_dispatch","summary"]`, gjson.GetBytes(finalBody, "text.format.schema.required").Raw)
}

func TestFinalizeOpenAIResponsesOAuthUpstreamBody_ContextMarkedMessagesBridgeInjectsDefaultInstructions(t *testing.T) {
	setGinTestMode()

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
	require.True(t, changed)
	require.True(t, gjson.GetBytes(finalBody, "instructions").Exists())
	require.NotEmpty(t, gjson.GetBytes(finalBody, "instructions").String())
}
