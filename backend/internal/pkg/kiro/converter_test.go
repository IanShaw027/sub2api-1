package kiro

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func requireJSONObject(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s has type %T, want map[string]any", name, value)
	}
	return object
}

func requireJSONArray(t *testing.T, value any, name string) []any {
	t.Helper()
	array, ok := value.([]any)
	if !ok {
		t.Fatalf("%s has type %T, want []any", name, value)
	}
	return array
}

func requireJSONString(t *testing.T, value any, name string) string {
	t.Helper()
	text, ok := value.(string)
	if !ok {
		t.Fatalf("%s has type %T, want string", name, value)
	}
	return text
}

func convertedCurrentTools(t *testing.T, body []byte) []map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state := requireJSONObject(t, payload["conversationState"], "conversationState")
	current := requireJSONObject(t, state["currentMessage"], "currentMessage")
	userMsg := requireJSONObject(t, current["userInputMessage"], "userInputMessage")
	ctx := requireJSONObject(t, userMsg["userInputMessageContext"], "userInputMessageContext")
	tools := requireJSONArray(t, ctx["tools"], "tools")

	out := make([]map[string]any, 0, len(tools))
	for i, raw := range tools {
		out = append(out, requireJSONObject(t, raw, fmt.Sprintf("tools[%d]", i)))
	}
	return out
}

func bridgeMetadataFieldValue(t *testing.T, result any) reflect.Value {
	t.Helper()

	value := reflect.ValueOf(result)
	if !value.IsValid() || value.IsNil() {
		t.Fatal("result is nil")
	}
	elem := value.Elem()
	field := elem.FieldByName("BridgeMetadata")
	if !field.IsValid() {
		t.Fatal("BridgeMetadata field not found on ConvertResult")
	}
	if field.IsNil() {
		t.Fatal("BridgeMetadata field is nil")
	}
	return field
}

func TestConvertAnthropicRequestWithModel_DoesNotInjectSyntheticAssistantOrPolicy(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"Follow repository constraints.",
		"messages":[{"role":"user","content":"hello"}]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state, ok := payload["conversationState"].(map[string]any)
	if !ok {
		t.Fatalf("conversationState has type %T, want map[string]any", payload["conversationState"])
	}
	history, ok := state["history"].([]any)
	if !ok {
		t.Fatalf("history has type %T, want []any", state["history"])
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}

	first, ok := history[0].(map[string]any)
	if !ok {
		t.Fatalf("first history item has type %T, want map[string]any", history[0])
	}
	if _, ok := first["assistantResponseMessage"]; ok {
		t.Fatalf("unexpected synthetic assistantResponseMessage in history")
	}
	userMsg, ok := first["userInputMessage"].(map[string]any)
	if !ok {
		t.Fatalf("userInputMessage has type %T, want map[string]any", first["userInputMessage"])
	}
	content := requireJSONString(t, userMsg["content"], "userInputMessage.content")
	if strings.Contains(content, "When the Write or Edit tool has content size limits") {
		t.Fatalf("unexpected injected chunked policy in history content: %q", content)
	}
}

func TestConvertAnthropicRequestWithModel_FiltersBillingHeaderAndInjectsThinkingPrefix(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[
			{"type":"text","text":"{{identity}}"},
			{"type":"text","text":"x-anthropic-billing-header: should-be-removed"},
			{"type":"text","text":"Follow repository constraints."}
		],
		"thinking":{"type":"enabled","budget_tokens":5000},
		"messages":[{"role":"user","content":"hello"}]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state, ok := payload["conversationState"].(map[string]any)
	if !ok {
		t.Fatalf("conversationState has type %T, want map[string]any", payload["conversationState"])
	}
	history, ok := state["history"].([]any)
	if !ok {
		t.Fatalf("history has type %T, want []any", state["history"])
	}
	if len(history) == 0 {
		t.Fatal("history should not be empty")
	}
	firstEntry, ok := history[0].(map[string]any)
	if !ok {
		t.Fatalf("first history entry has type %T, want map[string]any", history[0])
	}
	first, ok := firstEntry["userInputMessage"].(map[string]any)
	if !ok {
		t.Fatalf("userInputMessage has type %T, want map[string]any", firstEntry["userInputMessage"])
	}
	content, ok := first["content"].(string)
	if !ok {
		t.Fatalf("content has type %T, want string", first["content"])
	}
	if strings.Contains(content, "x-anthropic-billing-header") {
		t.Fatalf("billing header leaked into system history: %q", content)
	}
	if !strings.Contains(content, "<thinking_mode>enabled</thinking_mode><max_thinking_length>5000</max_thinking_length>") {
		t.Fatalf("missing thinking prefix in system history: %q", content)
	}
	if !strings.Contains(content, "You are Claude Code, Anthropic's official CLI for Claude.") {
		t.Fatalf("missing canonical Claude Code banner in system history: %q", content)
	}
	if strings.Contains(content, "{{identity}}") {
		t.Fatalf("identity placeholder leaked into system history: %q", content)
	}
	if !strings.Contains(content, "Follow repository constraints.") {
		t.Fatalf("missing system prompt in system history: %q", content)
	}
}

func TestConvertAnthropicRequestWithModel_RewritesKiroIdentityBanner(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"You are Amazon Q Developer, a Kiro coding assistant.\n\nFollow repository constraints.",
		"messages":[{"role":"user","content":"hello"}]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state, ok := payload["conversationState"].(map[string]any)
	if !ok {
		t.Fatalf("conversationState has type %T, want map[string]any", payload["conversationState"])
	}
	history, ok := state["history"].([]any)
	if !ok {
		t.Fatalf("history has type %T, want []any", state["history"])
	}
	if len(history) == 0 {
		t.Fatal("history should not be empty")
	}
	firstEntry, ok := history[0].(map[string]any)
	if !ok {
		t.Fatalf("first history entry has type %T, want map[string]any", history[0])
	}
	first, ok := firstEntry["userInputMessage"].(map[string]any)
	if !ok {
		t.Fatalf("userInputMessage has type %T, want map[string]any", firstEntry["userInputMessage"])
	}
	content, ok := first["content"].(string)
	if !ok {
		t.Fatalf("content has type %T, want string", first["content"])
	}
	if !strings.Contains(content, "You are Claude Code, Anthropic's official CLI for Claude.") {
		t.Fatalf("missing canonical Claude Code banner in system history: %q", content)
	}
	if strings.Contains(content, "Amazon Q") || strings.Contains(content, "Kiro coding assistant") {
		t.Fatalf("upstream identity leaked into system history: %q", content)
	}
}

func TestConvertAnthropicRequestWithModel_FiltersUnsupportedServerTools(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{"type":"web_search_20250305","name":"web_search","description":"search v2"},
			{"type":"web_fetch_20250910","name":"web_fetch","description":"fetch v2"},
			{"name":"local_tool","description":"local","input_schema":{"type":"object"}}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	tools := convertedCurrentTools(t, result.Body)
	if len(tools) != 3 {
		t.Fatalf("tools length = %d, want 3", len(tools))
	}
	gotNames := make(map[string]bool, len(tools))
	for i, entry := range tools {
		spec := requireJSONObject(t, entry["toolSpecification"], "toolSpecification")
		name, _ := spec["name"].(string)
		gotNames[name] = true
		switch name {
		case "web_search":
			schema := requireJSONObject(t, requireJSONObject(t, spec["inputSchema"], "inputSchema")["json"], "inputSchema.json")
			require.Equal(t, "object", schema["type"])
			require.Equal(t, []any{"query"}, requireJSONArray(t, schema["required"], fmt.Sprintf("tools[%d].required", i)))
		case "web_fetch":
			schema := requireJSONObject(t, requireJSONObject(t, spec["inputSchema"], "inputSchema")["json"], "inputSchema.json")
			require.Equal(t, "object", schema["type"])
			require.Equal(t, []any{"url"}, requireJSONArray(t, schema["required"], fmt.Sprintf("tools[%d].required", i)))
		}
	}
	for _, expected := range []string{"local_tool", "web_search", "web_fetch"} {
		if !gotNames[expected] {
			t.Fatalf("tools missing %q, got names: %v", expected, gotNames)
		}
	}
	if gotNames["cc_srv_web_search"] || gotNames["cc_srv_web_fetch"] || gotNames["web_search_20250305"] || gotNames["web_fetch_20250910"] {
		t.Fatalf("raw server tool names leaked into converted tools: %v", gotNames)
	}
	require.Nil(t, result.BridgeMetadata)
	require.NotNil(t, result.ToolMetadata)
	require.Equal(t, "anthropic_web_search", result.ToolMetadata.ResponseTools["web_search"].Family)
	require.Equal(t, "anthropic_web_fetch", result.ToolMetadata.ResponseTools["web_fetch"].Family)
}

func TestConvertAnthropicRequestWithModel_ReinforcesAskUserQuestionSchema(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"clean disk safely"}],
		"tools":[
			{
				"name":"AskUserQuestion",
				"description":"Ask the user for a missing decision.",
				"input_schema":{
					"type":"object",
					"properties":{
						"questions":{
							"type":"array",
							"items":{
								"type":"object",
								"properties":{
									"header":{"type":"string"},
									"id":{"type":"string"},
									"question":{"type":"string"},
									"options":{"type":"array"}
								}
							}
						}
					},
					"required":["questions"]
				}
			}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 1)
	spec := requireJSONObject(t, tools[0]["toolSpecification"], "toolSpecification")
	require.Equal(t, "AskUserQuestion", spec["name"])
	require.Contains(t, requireJSONString(t, spec["description"], "description"), "questions[].question")
	schema := requireJSONObject(t, requireJSONObject(t, spec["inputSchema"], "inputSchema")["json"], "inputSchema.json")
	require.Equal(t, []any{"questions"}, requireJSONArray(t, schema["required"], "root.required"))

	properties := requireJSONObject(t, schema["properties"], "root.properties")
	questions := requireJSONObject(t, properties["questions"], "properties.questions")
	items := requireJSONObject(t, questions["items"], "questions.items")
	require.Equal(t, []any{"question"}, requireJSONArray(t, items["required"], "questions.items.required"))
	require.Equal(t, false, items["additionalProperties"])
}

func TestConvertAnthropicRequestWithModel_WebSearchDoesNotUseShadowToolByDefault(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"search"}],
		"tools":[
			{"type":"web_search_20250305","name":"web_search","description":"search v2"}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 1)
	spec := requireJSONObject(t, tools[0]["toolSpecification"], "toolSpecification")
	require.Equal(t, "web_search", requireJSONString(t, spec["name"], "toolSpecification.name"))
	require.NotEqual(t, "cc_srv_web_search", requireJSONString(t, spec["name"], "toolSpecification.name"))
	require.Nil(t, result.BridgeMetadata)
	require.NotNil(t, result.ToolMetadata)
	require.Equal(t, "web_search_20250305", result.ToolMetadata.ResponseTools["web_search"].AnthropicType)
}

func TestConvertAnthropicRequestWithModel_WebSearchContinuationKeepsNativeHistoryTool(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"tools":[{"type":"web_search_20250305","name":"web_search","description":"search v2"}],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"Search for Go"}]},
			{"role":"assistant","content":[{"type":"server_tool_use","id":"srvtoolu_search_1","name":"web_search","input":{"query":"golang"}}]},
			{"role":"user","content":[
				{"type":"web_search_tool_result","tool_use_id":"srvtoolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]},
				{"type":"text","text":"Summarize the result"}
			]}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)
	require.NotNil(t, result.ToolMetadata)
	require.Nil(t, result.BridgeMetadata)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(result.Body, &payload))
	state := requireJSONObject(t, payload["conversationState"], "conversationState")
	history := requireJSONArray(t, state["history"], "conversationState.history")
	require.GreaterOrEqual(t, len(history), 2)
	assistantEntry := requireJSONObject(t, history[1], "history[1]")
	assistantMessage := requireJSONObject(t, assistantEntry["assistantResponseMessage"], "history[1].assistantResponseMessage")
	toolUses := requireJSONArray(t, assistantMessage["toolUses"], "assistantResponseMessage.toolUses")
	require.Len(t, toolUses, 1)
	toolUse := requireJSONObject(t, toolUses[0], "toolUses[0]")
	require.Equal(t, "web_search", requireJSONString(t, toolUse["name"], "toolUses[0].name"))
	require.NotEqual(t, "cc_srv_web_search", requireJSONString(t, toolUse["name"], "toolUses[0].name"))
}

func TestConvertAnthropicRequestWithModel_WebSearchContinuationWithoutToolsRestoresNativeTool(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"user","content":[{"type":"text","text":"Search for Go"}]},
			{"role":"assistant","content":[{"type":"server_tool_use","id":"srvtoolu_search_1","name":"web_search","input":{"query":"golang"}}]},
			{"role":"user","content":[
				{"type":"web_search_tool_result","tool_use_id":"srvtoolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]},
				{"type":"text","text":"Search again if needed"}
			]}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)
	require.NotNil(t, result.ToolMetadata)
	require.Nil(t, result.BridgeMetadata)
	require.Equal(t, "anthropic_web_search", result.ToolMetadata.ResponseTools["web_search"].Family)

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 1)
	spec := requireJSONObject(t, tools[0]["toolSpecification"], "toolSpecification")
	require.Equal(t, "web_search", requireJSONString(t, spec["name"], "toolSpecification.name"))
	schema := requireJSONObject(t, requireJSONObject(t, spec["inputSchema"], "inputSchema")["json"], "inputSchema.json")
	require.Equal(t, []any{"query"}, requireJSONArray(t, schema["required"], "web_search.required"))
}

func TestConvertAnthropicRequestWithModel_WebFetchContinuationWithoutToolsRestoresNativeTool(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"user","content":[{"type":"text","text":"Fetch docs"}]},
			{"role":"assistant","content":[{"type":"server_tool_use","id":"srvtoolu_fetch_1","name":"web_fetch","input":{"url":"https://example.com/start"}}]},
			{"role":"user","content":[
				{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_fetch_1","content":{"type":"web_fetch_result","url":"https://example.com/start","text":"start body"}},
				{"type":"text","text":"Fetch again if needed"}
			]}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)
	require.NotNil(t, result.ToolMetadata)
	require.Nil(t, result.BridgeMetadata)
	require.Equal(t, "anthropic_web_fetch", result.ToolMetadata.ResponseTools["web_fetch"].Family)

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 1)
	spec := requireJSONObject(t, tools[0]["toolSpecification"], "toolSpecification")
	require.Equal(t, "web_fetch", requireJSONString(t, spec["name"], "toolSpecification.name"))
	schema := requireJSONObject(t, requireJSONObject(t, spec["inputSchema"], "inputSchema")["json"], "inputSchema.json")
	require.Equal(t, []any{"url"}, requireJSONArray(t, schema["required"], "web_fetch.required"))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(result.Body, &payload))
	state := requireJSONObject(t, payload["conversationState"], "conversationState")
	history := requireJSONArray(t, state["history"], "conversationState.history")
	require.GreaterOrEqual(t, len(history), 2)
	assistantEntry := requireJSONObject(t, history[1], "history[1]")
	assistantMessage := requireJSONObject(t, assistantEntry["assistantResponseMessage"], "history[1].assistantResponseMessage")
	toolUses := requireJSONArray(t, assistantMessage["toolUses"], "assistantResponseMessage.toolUses")
	require.Len(t, toolUses, 1)
	toolUse := requireJSONObject(t, toolUses[0], "toolUses[0]")
	require.Equal(t, "web_fetch", requireJSONString(t, toolUse["name"], "toolUses[0].name"))
	inputObj := requireJSONObject(t, toolUse["input"], "toolUses[0].input")
	require.Equal(t, "https://example.com/start", inputObj["url"])
}

func TestConvertAnthropicRequestWithModel_GoogleSearchUsesNativeWebSearchTool(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"search"}],
		"tools":[
			{"type":"google_search","name":"web_search","description":"google search"}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 1)
	spec := requireJSONObject(t, tools[0]["toolSpecification"], "toolSpecification")
	require.Equal(t, "web_search", requireJSONString(t, spec["name"], "toolSpecification.name"))
	require.Nil(t, result.BridgeMetadata)
	require.NotNil(t, result.ToolMetadata)
	require.Equal(t, "google_search", result.ToolMetadata.ResponseTools["web_search"].AnthropicType)
	require.Equal(t, "web_search", result.ToolMetadata.ResponseTools["web_search"].AnthropicName)
}

func TestConvertAnthropicRequestWithModel_WebFetchUsesNativeKiroTool(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"fetch docs"}],
		"tools":[
			{"type":"web_fetch_20260318","name":"web_fetch","description":"fetch v2"}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)
	require.Nil(t, result.BridgeMetadata)
	require.NotNil(t, result.ToolMetadata)
	require.Equal(t, "web_fetch_20260318", result.ToolMetadata.ResponseTools["web_fetch"].AnthropicType)
	require.Equal(t, "web_fetch", result.ToolMetadata.ResponseTools["web_fetch"].AnthropicName)

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 1)
	spec := requireJSONObject(t, tools[0]["toolSpecification"], "toolSpecification")
	require.Equal(t, "web_fetch", requireJSONString(t, spec["name"], "toolSpecification.name"))
	require.NotEqual(t, "cc_srv_web_fetch", requireJSONString(t, spec["name"], "toolSpecification.name"))
	schema := requireJSONObject(t, requireJSONObject(t, spec["inputSchema"], "inputSchema")["json"], "inputSchema.json")
	require.Equal(t, []any{"url"}, requireJSONArray(t, schema["required"], "web_fetch.required"))
}

func TestConvertAnthropicRequestWithModel_AcceptsOfficialBashToolFamily(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"run pwd"}],
		"tools":[
			{"type":"bash_20250124","name":"bash","description":"Run shell commands"}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 1)
	spec := requireJSONObject(t, tools[0]["toolSpecification"], "toolSpecification")
	require.Equal(t, "Bash", requireJSONString(t, spec["name"], "toolSpecification.name"))
	schema := requireJSONObject(t, requireJSONObject(t, spec["inputSchema"], "inputSchema")["json"], "inputSchema.json")
	require.Contains(t, requireJSONArray(t, schema["required"], "required"), "command")
}

func TestConvertAnthropicRequestWithModel_AcceptsOfficialTextEditorToolFamily(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"inspect repo files"}],
		"tools":[
			{"type":"text_editor_20250728","name":"str_replace_based_edit_tool","description":"Edit text files"}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 3)
	gotNames := make(map[string]bool, len(tools))
	for _, entry := range tools {
		spec := requireJSONObject(t, entry["toolSpecification"], "toolSpecification")
		gotNames[requireJSONString(t, spec["name"], "toolSpecification.name")] = true
	}
	for _, expected := range []string{"Read", "Write", "Edit"} {
		require.True(t, gotNames[expected], "missing %s in %v", expected, gotNames)
	}
	require.False(t, gotNames["str_replace_based_edit_tool"], "official Anthropic tool name should not be sent to Kiro directly")
}

func TestConvertAnthropicRequestWithModel_CustomBashWithoutTypeRemainsCustom(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"use custom bash"}],
		"tools":[
			{"name":"Bash","description":"Custom tool named Bash","input_schema":{"type":"object","properties":{"script":{"type":"string"}},"required":["script"]}}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 1)
	spec := requireJSONObject(t, tools[0]["toolSpecification"], "toolSpecification")
	require.Equal(t, "Bash", requireJSONString(t, spec["name"], "toolSpecification.name"))
	schema := requireJSONObject(t, requireJSONObject(t, spec["inputSchema"], "inputSchema")["json"], "inputSchema.json")
	require.Contains(t, requireJSONArray(t, schema["required"], "required"), "script")
	require.NotContains(t, requireJSONArray(t, schema["required"], "required"), "command")
}

func TestConvertAnthropicRequestWithModel_ReplaysOfficialBashHistoryAsKiroBash(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"user","content":"run pwd"},
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_bash","name":"bash","input":{"command":"pwd"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_bash","content":"\/tmp"}]},
			{"role":"user","content":"continue"}
		],
		"tools":[{"type":"bash_20250124","name":"bash"}]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(result.Body, &payload))
	history := requireJSONArray(t, requireJSONObject(t, payload["conversationState"], "conversationState")["history"], "history")
	var found bool
	for _, raw := range history {
		entry := requireJSONObject(t, raw, "history entry")
		assistant, ok := entry["assistantResponseMessage"].(map[string]any)
		if !ok {
			continue
		}
		toolUses := requireJSONArray(t, assistant["toolUses"], "toolUses")
		require.Len(t, toolUses, 1)
		toolUse := requireJSONObject(t, toolUses[0], "toolUses[0]")
		require.Equal(t, "Bash", toolUse["name"])
		inputObj := requireJSONObject(t, toolUse["input"], "toolUses[0].input")
		require.Equal(t, "pwd", inputObj["command"])
		found = true
	}
	require.True(t, found, "assistant tool history not found")
}

func TestConvertAnthropicRequestWithModel_ReplaysOfficialTextEditorHistoryAsKiroFileTool(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"user","content":"edit file"},
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_edit","name":"str_replace_based_edit_tool","input":{"command":"str_replace","path":"\/tmp\/a.txt","offset":12,"limit":34,"old_str":"old","new_str":"new","replace_all":true}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_edit","content":"ok"}]},
			{"role":"user","content":"continue"}
		],
		"tools":[{"type":"text_editor_20250728","name":"str_replace_based_edit_tool"}]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(result.Body, &payload))
	history := requireJSONArray(t, requireJSONObject(t, payload["conversationState"], "conversationState")["history"], "history")
	var found bool
	for _, raw := range history {
		entry := requireJSONObject(t, raw, "history entry")
		assistant, ok := entry["assistantResponseMessage"].(map[string]any)
		if !ok {
			continue
		}
		toolUses := requireJSONArray(t, assistant["toolUses"], "toolUses")
		require.Len(t, toolUses, 1)
		toolUse := requireJSONObject(t, toolUses[0], "toolUses[0]")
		require.Equal(t, "Edit", toolUse["name"])
		inputObj := requireJSONObject(t, toolUse["input"], "toolUses[0].input")
		require.Equal(t, "/tmp/a.txt", inputObj["file_path"])
		require.Equal(t, "old", inputObj["old_string"])
		require.Equal(t, "new", inputObj["new_string"])
		require.Equal(t, float64(12), inputObj["offset"])
		require.Equal(t, float64(34), inputObj["limit"])
		require.Equal(t, true, inputObj["replace_all"])
		found = true
	}
	require.True(t, found, "assistant text editor tool history not found")
}

func TestIntAnyFieldRejectsFractionalNumbers(t *testing.T) {
	obj := map[string]any{
		"int":             12,
		"integer_float":   float64(34),
		"fraction_float":  float64(12.9),
		"huge_float":      float64(1e100),
		"integer_number":  json.Number("56"),
		"fraction_number": json.Number("78.1"),
	}

	value, ok := intAnyField(obj, "int")
	require.True(t, ok)
	require.Equal(t, 12, value)

	value, ok = intAnyField(obj, "integer_float")
	require.True(t, ok)
	require.Equal(t, 34, value)

	_, ok = intAnyField(obj, "fraction_float")
	require.False(t, ok)

	_, ok = intAnyField(obj, "huge_float")
	require.False(t, ok)

	value, ok = intAnyField(obj, "integer_number")
	require.True(t, ok)
	require.Equal(t, 56, value)

	_, ok = intAnyField(obj, "fraction_number")
	require.False(t, ok)
}

func TestConvertAnthropicRequestWithModel_RejectsUnsupportedServerToolFamilies(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{"type":"computer_20250124","name":"computer","display_width_px":1024,"display_height_px":768}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported server-side tool family")
	require.Contains(t, err.Error(), "computer_20250124")
}

func TestConvertAnthropicRequestWithModel_RejectsUnknownServerToolFamiliesFailClosed(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{"type":"container_upload_20260101","name":"container_upload"}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported server-side tool family")
	require.Contains(t, err.Error(), "container_upload_20260101")
}

func TestConvertAnthropicRequestWithModel_ToolSearchServerToolsRemainUntouched(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"search"}],
		"tools":[
			{"type":"tool_search_tool_regex_20251119","name":"tool_search_tool_regex_20251119","description":"regex tool search"},
			{"name":"local_tool","description":"local","input_schema":{"type":"object"}}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	tools := convertedCurrentTools(t, result.Body)
	require.Len(t, tools, 2)

	gotNames := make([]string, 0, len(tools))
	for i, entry := range tools {
		spec := requireJSONObject(t, entry["toolSpecification"], fmt.Sprintf("tools[%d].toolSpecification", i))
		gotNames = append(gotNames, requireJSONString(t, spec["name"], fmt.Sprintf("tools[%d].toolSpecification.name", i)))
	}
	require.Contains(t, gotNames, "tool_search_tool_regex_20251119")
	require.Contains(t, gotNames, "local_tool")
}

func TestConvertAnthropicRequestWithModel_HistoryShadowWebFetchRestoresBridgeConstraintsWithoutTools(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"assistant","content":[
				{
					"type":"tool_use",
					"id":"toolu_fetch_1",
					"name":"cc_srv_web_fetch",
					"input":{"url":"https://example.com/a"},
					"_shadow_bridge":{
						"anthropic_type":"web_fetch_20250910",
						"anthropic_name":"web_fetch",
						"allowed_domains":["example.com","docs.example.com"],
						"blocked_domains":["blocked.example.com"],
						"max_uses":2,
						"max_content_tokens":8192
					}
				}
			]},
			{"role":"user","content":[
				{
					"type":"tool_result",
					"tool_use_id":"toolu_fetch_1",
					"content":"ok"
				}
			]},
			{"role":"user","content":"fetch another page"}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	require.NoError(t, err)

	bridgeField := bridgeMetadataFieldValue(t, result)
	shadowTools := bridgeField.Elem().FieldByName("ShadowTools")
	webFetch := shadowTools.MapIndex(reflect.ValueOf("cc_srv_web_fetch"))
	require.True(t, webFetch.IsValid())
	require.Equal(t, "web_fetch_20250910", webFetch.FieldByName("AnthropicType").String())
	require.Equal(t, "web_fetch", webFetch.FieldByName("AnthropicName").String())
	require.Equal(t, 2, int(webFetch.FieldByName("MaxUses").Int()))
	require.Equal(t, 8192, int(webFetch.FieldByName("MaxContentTokens").Int()))
	require.Equal(t, []string{"example.com", "docs.example.com"}, webFetch.FieldByName("AllowedDomains").Interface())
	require.Equal(t, []string{"blocked.example.com"}, webFetch.FieldByName("BlockedDomains").Interface())
}

func TestConvertAnthropicRequestWithModel_DowngradesUnsupportedEnabledThinkingToAdaptivePrefix(t *testing.T) {
	enabledInput := []byte(`{
		"model":"claude-haiku-4.6",
		"thinking":{"type":"enabled","budget_tokens":2048},
		"messages":[{"role":"user","content":"Reply with OK only."}]
	}`)
	adaptiveInput := []byte(`{
		"model":"claude-haiku-4.6",
		"thinking":{"type":"adaptive","thinking_effort":"low","display":"summarized"},
		"messages":[{"role":"user","content":"Reply with OK only."}]
	}`)

	enabledResult, err := ConvertAnthropicRequestWithModel(enabledInput, "claude-haiku-4.6")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel enabled error: %v", err)
	}
	adaptiveResult, err := ConvertAnthropicRequestWithModel(adaptiveInput, "claude-haiku-4.6")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel adaptive error: %v", err)
	}

	enabledPrefix := convertedHistoryPrefix(t, enabledResult.Body)
	adaptivePrefix := convertedHistoryPrefix(t, adaptiveResult.Body)

	if enabledPrefix != adaptivePrefix {
		t.Fatalf("enabled/adaptive prefixes differ\nenabled:  %q\nadaptive: %q", enabledPrefix, adaptivePrefix)
	}
	if !strings.Contains(enabledPrefix, "<thinking_mode>adaptive</thinking_mode>") {
		t.Fatalf("missing adaptive thinking prefix: %q", enabledPrefix)
	}
	if strings.Contains(enabledPrefix, "<thinking_mode>enabled</thinking_mode>") {
		t.Fatalf("unexpected enabled thinking prefix: %q", enabledPrefix)
	}
}

func TestConvertAnthropicRequestWithModel_PreservesEnabledThinkingForSupportedOpus47And48(t *testing.T) {
	for _, model := range []string{"claude-opus-4.7", "claude-opus-4.8"} {
		t.Run(model, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{
				"model":%q,
				"thinking":{"type":"enabled","budget_tokens":2048},
				"messages":[{"role":"user","content":"Reply with OK only."}]
			}`, model))

			result, err := ConvertAnthropicRequestWithModel(input, model)
			require.NoError(t, err)
			prefix := convertedHistoryPrefix(t, result.Body)
			require.Contains(t, prefix, "<thinking_mode>enabled</thinking_mode>")
			require.Contains(t, prefix, "<max_thinking_length>2048</max_thinking_length>")
			require.NotContains(t, prefix, "<thinking_mode>adaptive</thinking_mode>")
		})
	}
}

func TestConvertAnthropicRequestWithModel_TruncatesLongToolNamesAndPreservesMap(t *testing.T) {
	longName := "mcp__server_name_with_really_long_prefix_that_exceeds_limits__tool_name_with_really_long_suffix"
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"` + longName + `","input":{"k":"v"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"done"}]}
		],
		"tools":[
			{"name":"` + longName + `","description":"local","input_schema":{"type":"object"}}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	if len(result.ToolNameMap) != 1 {
		t.Fatalf("tool name map length = %d, want 1", len(result.ToolNameMap))
	}

	var shortName string
	for k, v := range result.ToolNameMap {
		shortName = k
		if v != longName {
			t.Fatalf("tool name map value = %q, want %q", v, longName)
		}
	}
	if len(shortName) > 63 {
		t.Fatalf("short tool name length = %d, want <= 63", len(shortName))
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state := requireJSONObject(t, payload["conversationState"], "conversationState")
	current := requireJSONObject(t, state["currentMessage"], "currentMessage")
	ctx := requireJSONObject(t, requireJSONObject(t, current["userInputMessage"], "userInputMessage")["userInputMessageContext"], "userInputMessageContext")
	tools := requireJSONArray(t, ctx["tools"], "tools")
	spec := requireJSONObject(t, requireJSONObject(t, tools[0], "tools[0]")["toolSpecification"], "toolSpecification")
	if got := requireJSONString(t, spec["name"], "tool spec name"); got != shortName {
		t.Fatalf("tool spec name = %q, want %q", got, shortName)
	}

	history := requireJSONArray(t, state["history"], "history")
	assistant := requireJSONObject(t, requireJSONObject(t, history[0], "history[0]")["assistantResponseMessage"], "assistantResponseMessage")
	toolUses := requireJSONArray(t, assistant["toolUses"], "toolUses")
	if got := requireJSONString(t, requireJSONObject(t, toolUses[0], "toolUses[0]")["name"], "toolUses[0].name"); got != shortName {
		t.Fatalf("history tool use name = %q, want %q", got, shortName)
	}
	if got := requireJSONString(t, assistant["content"], "assistantResponseMessage.content"); got == "" {
		t.Fatal("assistant tool-only history content should not be empty")
	}
}

func TestConvertAnthropicRequestWithModel_CleansOrphanToolPairsAndFillsToolOnlyContent(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"orphan-use","name":"local_tool","input":{}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"orphan-result","content":"orphan result"}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"paired-id","name":"local_tool","input":{}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"paired-id","content":"paired result"}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"current-id","name":"local_tool","input":{}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"current-id","content":"current result"}]}
		],
		"tools":[
			{"name":"local_tool","description":"local","input_schema":{"type":"object"}}
		]
	}`)

	result, err := ConvertAnthropicRequestWithModel(input, "")
	if err != nil {
		t.Fatalf("ConvertAnthropicRequestWithModel error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result.Body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state := requireJSONObject(t, payload["conversationState"], "conversationState")
	history := requireJSONArray(t, state["history"], "history")
	if len(history) != 5 {
		t.Fatalf("history length = %d, want 5", len(history))
	}

	firstAssistant := requireJSONObject(t, requireJSONObject(t, history[0], "history[0]")["assistantResponseMessage"], "assistantResponseMessage")
	if _, ok := firstAssistant["toolUses"]; ok {
		t.Fatal("orphan assistant tool_use should be removed")
	}
	if got := requireJSONString(t, firstAssistant["content"], "assistantResponseMessage.content"); got != "" {
		t.Fatalf("orphan assistant text content = %q, want empty", got)
	}

	firstUser := requireJSONObject(t, requireJSONObject(t, history[1], "history[1]")["userInputMessage"], "userInputMessage")
	if _, ok := firstUser["userInputMessageContext"]; ok {
		t.Fatal("orphan user tool_result should be removed")
	}
	if got := requireJSONString(t, firstUser["content"], "userInputMessage.content"); got != "" {
		t.Fatalf("orphan user text content = %q, want empty", got)
	}

	thirdAssistant := requireJSONObject(t, requireJSONObject(t, history[4], "history[4]")["assistantResponseMessage"], "assistantResponseMessage")
	toolUses := requireJSONArray(t, thirdAssistant["toolUses"], "toolUses")
	if len(toolUses) != 1 {
		t.Fatalf("current-paired assistant toolUses length = %d, want 1", len(toolUses))
	}
	if got := requireJSONString(t, requireJSONObject(t, toolUses[0], "toolUses[0]")["toolUseId"], "toolUseId"); got != "current-id" {
		t.Fatalf("current-paired assistant toolUseId = %q, want current-id", got)
	}
	if got := requireJSONString(t, thirdAssistant["content"], "assistantResponseMessage.content"); got != "I will call the requested tools." {
		t.Fatalf("current-paired assistant content = %q, want placeholder", got)
	}

	current := requireJSONObject(t, requireJSONObject(t, state["currentMessage"], "currentMessage")["userInputMessage"], "userInputMessage")
	if got := requireJSONString(t, current["content"], "userInputMessage.content"); got != "Here are the tool results." {
		t.Fatalf("current tool-only content = %q, want placeholder", got)
	}
}

func TestTrimAnthropicRequestToTokenBudget_DropsOldestMessagesAndAnnotatesSystem(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"Follow repository constraints.",
		"messages":[
			{"role":"user","content":"first turn text text text text text text"},
			{"role":"assistant","content":"middle turn text text text text text"},
			{"role":"user","content":"latest turn text text text"}
		]
	}`)

	trimmed, dropped, changed, err := TrimAnthropicRequestToTokenBudget(input, 1)
	if err != nil {
		t.Fatalf("TrimAnthropicRequestToTokenBudget error: %v", err)
	}
	if !changed {
		t.Fatal("expected request to be trimmed")
	}
	if dropped == 0 {
		t.Fatal("expected at least one message to be dropped")
	}

	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		t.Fatalf("unmarshal trimmed payload: %v", err)
	}

	system := requireJSONString(t, payload["system"], "system")
	if !strings.Contains(system, "compacted to fit Kiro's available context window") {
		t.Fatalf("missing trim note in system: %q", system)
	}

	messages := requireJSONArray(t, payload["messages"], "messages")
	if len(messages) != 1 {
		t.Fatalf("trimmed messages length = %d, want 1", len(messages))
	}
	last := requireJSONObject(t, messages[0], "messages[0]")
	if got := last["role"]; got != "user" {
		t.Fatalf("trimmed last message role = %v, want user", got)
	}
	if got := last["content"]; got != "latest turn text text text" {
		t.Fatalf("trimmed last message content = %v, want latest turn text text text", got)
	}
}

func TestCompactAnthropicRequestToTokenBudget_PreservesRecentWindowAndAddsStructuredSummary(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"Follow repository constraints.",
		"messages":[
			{"role":"user","content":"Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction. Please update backend/internal/service/kiro_gateway_service.go and backend/internal/pkg/kiro/converter.go for compaction."},
			{"role":"assistant","content":"I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now. I am checking backend/internal/service/settings_view.go and backend/internal/service/kiro_gateway_service.go now."},
			{"role":"user","content":"Keep the recent window and summary."},
			{"role":"assistant","content":"Understood."},
			{"role":"user","content":"Make sure compaction preserves the recent tail."},
			{"role":"user","content":"Latest request: preserve billing and recent tail."}
		]
	}`)

	compacted, dropped, changed, err := CompactAnthropicRequestToTokenBudget(input, 500)
	if err != nil {
		t.Fatalf("CompactAnthropicRequestToTokenBudget error: %v", err)
	}
	if !changed {
		t.Fatal("expected request to be compacted")
	}
	if dropped == 0 {
		t.Fatal("expected at least one message to be dropped")
	}

	var payload map[string]any
	if err := json.Unmarshal(compacted, &payload); err != nil {
		t.Fatalf("unmarshal compacted payload: %v", err)
	}

	system := requireJSONString(t, payload["system"], "system")
	if !strings.Contains(system, "Compaction summary:") {
		t.Fatalf("missing compaction summary header: %q", system)
	}
	if !strings.Contains(system, "backend/internal/service/kiro_gateway_service.go") {
		t.Fatalf("missing key file in summary: %q", system)
	}
	if !strings.Contains(system, "Recent user requests:") {
		t.Fatalf("missing recent user requests section: %q", system)
	}

	messages := requireJSONArray(t, payload["messages"], "messages")
	if len(messages) != 4 {
		t.Fatalf("compacted messages length = %d, want 4", len(messages))
	}
	last := requireJSONObject(t, messages[len(messages)-1], "messages[last]")
	if got := last["role"]; got != "user" {
		t.Fatalf("compacted last message role = %v, want user", got)
	}
	if got := last["content"]; got != "Latest request: preserve billing and recent tail." {
		t.Fatalf("compacted last message content = %v, want latest request", got)
	}
}

func TestTrimAnthropicRequestToTokenBudget_PreservesLeadingToolPair(t *testing.T) {
	input := []byte(fmt.Sprintf(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"pair-1","name":"local_tool","input":{"path":"backend/internal/pkg/kiro/converter.go"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"pair-1","content":"tool output"}]},
			{"role":"user","content":"latest request %stail"}
		]
	}`, strings.Repeat("keep ", 60)))

	trimmed, dropped, changed, err := TrimAnthropicRequestToTokenBudget(input, 80)
	if err != nil {
		t.Fatalf("TrimAnthropicRequestToTokenBudget error: %v", err)
	}
	if !changed {
		t.Fatal("expected request to be trimmed")
	}
	if dropped != 2 {
		t.Fatalf("trimmed dropped = %d, want 2 to preserve tool pair", dropped)
	}

	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		t.Fatalf("unmarshal trimmed payload: %v", err)
	}

	messages := requireJSONArray(t, payload["messages"], "messages")
	if len(messages) != 1 {
		t.Fatalf("trimmed messages length = %d, want 1", len(messages))
	}
	first := requireJSONObject(t, messages[0], "messages[0]")
	if got := first["role"]; got != "user" {
		t.Fatalf("trimmed first message role = %v, want user", got)
	}
	if content, ok := first["content"].(string); !ok || !strings.Contains(content, "latest request") {
		t.Fatalf("trimmed first message content = %v, want latest user content", first["content"])
	}
}

func TestCompactAnthropicRequestToTokenBudget_SmallWindowDoesNotSplitToolPair(t *testing.T) {
	input := []byte(fmt.Sprintf(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"pair-1","name":"local_tool","input":{"path":"backend/internal/pkg/kiro/converter.go","mode":"compact"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"pair-1","content":"tool output"}]},
			{"role":"assistant","content":"intermediate response %stail"},
			{"role":"user","content":"latest request %stail"}
		]
	}`, strings.Repeat("summary ", 40), strings.Repeat("budget ", 60)))

	compacted, dropped, changed, err := CompactAnthropicRequestToTokenBudget(input, 150)
	if err != nil {
		t.Fatalf("CompactAnthropicRequestToTokenBudget error: %v", err)
	}
	if !changed {
		t.Fatal("expected request to be compacted")
	}
	if dropped == 0 {
		t.Fatal("expected at least one message to be dropped")
	}

	var payload map[string]any
	if err := json.Unmarshal(compacted, &payload); err != nil {
		t.Fatalf("unmarshal compacted payload: %v", err)
	}

	messages := requireJSONArray(t, payload["messages"], "messages")
	if len(messages) == 0 {
		t.Fatal("compacted messages should not be empty")
	}
	first := requireJSONObject(t, messages[0], "messages[0]")
	if role := first["role"]; role == "user" {
		if content, ok := first["content"].([]any); ok {
			for _, item := range content {
				block := requireJSONObject(t, item, "messages[0].content[]")
				if block["type"] == "tool_result" {
					t.Fatalf("compaction kept an orphan leading tool_result: %v", first)
				}
			}
		}
	}
}

func TestEstimateInputTokens_CalibratedAgainstAnthropicUsage(t *testing.T) {
	// Calibration anchors: controlled request bodies sent through the real
	// Anthropic API. Estimates must land within 15% of the observed real total
	// input tokens, locking in the calibration constants in converter.go.
	cases := []struct {
		name string
		body string
		real int
	}{
		{"short", `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"What is the capital of France? Answer in one word."}]}`, 34},
		{"tools_1", `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"Read /etc/hostname"}],"tools":[{"name":"Read","description":"Reads a file from the local filesystem.","input_schema":{"type":"object","properties":{"file_path":{"type":"string","description":"The absolute path to the file to read"},"offset":{"type":"integer"},"limit":{"type":"integer"}},"required":["file_path"]}}]}`, 615},
		{"tools_4", `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"Read /etc/hostname"}],"tools":[{"name":"Read","description":"Reads a file from the local filesystem.","input_schema":{"type":"object","properties":{"file_path":{"type":"string","description":"The absolute path to the file to read"},"offset":{"type":"integer"},"limit":{"type":"integer"}},"required":["file_path"]}},{"name":"Write","description":"Writes a file to the local filesystem.","input_schema":{"type":"object","properties":{"file_path":{"type":"string"},"content":{"type":"string"}},"required":["file_path","content"]}},{"name":"Bash","description":"Executes a bash command.","input_schema":{"type":"object","properties":{"command":{"type":"string"},"timeout":{"type":"integer"},"description":{"type":"string"}},"required":["command"]}},{"name":"Edit","description":"Performs exact string replacements in files.","input_schema":{"type":"object","properties":{"file_path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"},"replace_all":{"type":"boolean"}},"required":["file_path","old_string","new_string"]}}]}`, 863},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			est := EstimateInputTokens([]byte(tc.body))
			relErr := float64(est-tc.real) / float64(tc.real)
			if relErr < 0 {
				relErr = -relErr
			}
			if relErr > 0.15 {
				t.Fatalf("estimate = %d, real = %d, rel err = %.1f%% (want <= 15%%)", est, tc.real, relErr*100)
			}
		})
	}
}

func TestEstimateInputTokens_IncludesToolSchemasAndAssistantToolUseInput(t *testing.T) {
	rich := []byte(fmt.Sprintf(`{
		"model":"claude-sonnet-4-6",
		"system":"Follow repository constraints.",
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"local_tool","input":{"path":"backend/internal/pkg/kiro/converter.go","instruction":"%stail"}}]},
			{"role":"user","content":"latest request"}
		],
		"tools":[
			{"name":"local_tool","description":"Local tool","input_schema":{"type":"object","properties":{"instruction":{"type":"string","description":"%stail"}},"required":["instruction"]}}
		]
	}`, strings.Repeat("budget ", 40), strings.Repeat("schema ", 60)))
	lean := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"Follow repository constraints.",
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"local_tool","input":{}}]},
			{"role":"user","content":"latest request"}
		],
		"tools":[
			{"name":"local_tool","description":"Local tool","input_schema":{"type":"object","properties":{},"required":[]}}
		]
	}`)

	richTokens := EstimateInputTokens(rich)
	leanTokens := EstimateInputTokens(lean)
	if richTokens <= leanTokens {
		t.Fatalf("rich token estimate = %d, want greater than lean estimate = %d", richTokens, leanTokens)
	}
}

func TestEstimateInputTokens_IncludesNativeWebToolHistory(t *testing.T) {
	rich := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"user","content":"search go docs"},
			{"role":"assistant","content":[{"type":"server_tool_use","id":"srvtoolu_search_1","name":"web_search","input":{"query":"golang documentation"}}]},
			{"role":"user","content":[{"type":"web_search_tool_result","tool_use_id":"srvtoolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]}]},
			{"role":"assistant","content":[{"type":"server_tool_use","id":"srvtoolu_fetch_1","name":"web_fetch","input":{"url":"https://go.dev/doc"}}]},
			{"role":"user","content":[{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_fetch_1","content":{"type":"web_fetch_result","url":"https://go.dev/doc","text":"Go documentation body"}}]},
			{"role":"user","content":"continue"}
		]
	}`)
	lean := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[
			{"role":"user","content":"search go docs"},
			{"role":"assistant","content":[{"type":"server_tool_use","id":"srvtoolu_search_1","name":"web_search","input":{}}]},
			{"role":"user","content":[{"type":"web_search_tool_result","tool_use_id":"srvtoolu_search_1","content":[]}]},
			{"role":"assistant","content":[{"type":"server_tool_use","id":"srvtoolu_fetch_1","name":"web_fetch","input":{}}]},
			{"role":"user","content":[{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_fetch_1","content":{}}]},
			{"role":"user","content":"continue"}
		]
	}`)

	richTokens := EstimateInputTokens(rich)
	leanTokens := EstimateInputTokens(lean)
	require.Greater(t, richTokens, leanTokens)
	require.GreaterOrEqual(t, richTokens-leanTokens, kiroTokenEstimatePerToolBlock)
}

func TestEstimateInputTokens_HandlesSystemPolymorphism(t *testing.T) {
	stringSystem := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":"Follow repository constraints.",
		"messages":[{"role":"user","content":"latest request"}]
	}`)
	arraySystem := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[{"type":"text","text":"Follow repository constraints."}],
		"messages":[{"role":"user","content":"latest request"}]
	}`)
	nullSystem := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":null,
		"messages":[{"role":"user","content":"latest request"}]
	}`)

	stringTokens := EstimateInputTokens(stringSystem)
	arrayTokens := EstimateInputTokens(arraySystem)
	nullTokens := EstimateInputTokens(nullSystem)

	if stringTokens != arrayTokens {
		t.Fatalf("string system tokens = %d, want array system tokens = %d", stringTokens, arrayTokens)
	}
	if nullTokens <= 0 {
		t.Fatalf("null system tokens = %d, want > 0", nullTokens)
	}
	if nullTokens >= stringTokens {
		t.Fatalf("null system tokens = %d, want less than string system tokens = %d", nullTokens, stringTokens)
	}
}

func convertedHistoryPrefix(t *testing.T, body []byte) string {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal converted payload: %v", err)
	}

	state := requireJSONObject(t, payload["conversationState"], "conversationState")
	history := requireJSONArray(t, state["history"], "history")
	if len(history) == 0 {
		t.Fatal("history is empty")
	}

	first := requireJSONObject(t, history[0], "history[0]")
	userInput := requireJSONObject(t, first["userInputMessage"], "history[0].userInputMessage")
	return requireJSONString(t, userInput["content"], "history[0].userInputMessage.content")
}

func TestStripToolTurnPlaceholders(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"I will call the requested tools.", ""},
		{"Here are the tool results.", ""},
		{"  I will call the requested tools.  ", ""},
		{"Real assistant answer.", "Real assistant answer."},
		{"I will call the requested tools. Then do more.", "I will call the requested tools. Then do more."},
		{"", ""},
	}
	for _, tc := range cases {
		if got := StripToolTurnPlaceholders(tc.in); got != tc.want {
			t.Fatalf("StripToolTurnPlaceholders(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestToolTurnPlaceholderMatch(t *testing.T) {
	cases := []struct {
		in         string
		wantExact  bool
		wantPrefix bool
	}{
		{"", false, true},
		{"I will", false, true},
		{"I will call the requested tools.", true, false},
		{"Here are the tool results.", true, false},
		{"Here are", false, true},
		{"Hello world", false, false},
		{"I will call the requested tools. More", false, false},
	}
	for _, tc := range cases {
		exact, prefix := ToolTurnPlaceholderMatch(tc.in)
		if exact != tc.wantExact || prefix != tc.wantPrefix {
			t.Fatalf("ToolTurnPlaceholderMatch(%q) = (exact=%v, prefix=%v), want (exact=%v, prefix=%v)",
				tc.in, exact, prefix, tc.wantExact, tc.wantPrefix)
		}
	}
}

func TestIsPlaceholderFragment(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"call", true},
		{"call the requested tools.", true},
		{"requested tools.", true},
		{"I will call", true},
		{"Here are the tool results.", true},
		{"tool results.", true},
		{"the tool results.", true},
		{"Hello world", false},
		{"re", false},    // too short
		{"the", true},    // word boundary match in both placeholders
		{"ll", false},    // too short
		{"wil", false},   // not on word boundary
		{"ool", false},   // not on word boundary
		{" call ", true}, // trimmed to "call"
		{"I will call the requested tools. Extra", false},
	}
	for _, tc := range cases {
		if got := IsPlaceholderFragment(tc.in); got != tc.want {
			t.Fatalf("IsPlaceholderFragment(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestStripTrailingPlaceholderFragment(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Some real text.\ncall", "Some real text."},
		{"Some real text.\n\ncall the requested tools.", "Some real text."},
		{"call", ""},
		{"Real content only.", "Real content only."},
		{"Line one.\nLine two.\nI will call", "Line one.\nLine two."},
		{"", ""},
	}
	for _, tc := range cases {
		if got := StripTrailingPlaceholderFragment(tc.in); got != tc.want {
			t.Fatalf("StripTrailingPlaceholderFragment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
