package kiro

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

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
	content, _ := userMsg["content"].(string)
	if strings.Contains(content, "When the Write or Edit tool has content size limits") {
		t.Fatalf("unexpected injected chunked policy in history content: %q", content)
	}
}

func TestConvertAnthropicRequestWithModel_FiltersBillingHeaderAndInjectsThinkingPrefix(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[
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

	history := payload["conversationState"].(map[string]any)["history"].([]any)
	first := history[0].(map[string]any)["userInputMessage"].(map[string]any)
	content := first["content"].(string)
	if strings.Contains(content, "x-anthropic-billing-header") {
		t.Fatalf("billing header leaked into system history: %q", content)
	}
	if !strings.Contains(content, "<thinking_mode>enabled</thinking_mode><max_thinking_length>5000</max_thinking_length>") {
		t.Fatalf("missing thinking prefix in system history: %q", content)
	}
	if !strings.Contains(content, "Follow repository constraints.") {
		t.Fatalf("missing system prompt in system history: %q", content)
	}
}

func TestConvertAnthropicRequestWithModel_FiltersUnsupportedServerTools(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{"type":"server_tool","name":"web_search","description":"search"},
			{"type":"web_search_20250305","name":"web_search_20250305","description":"search v2"},
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

	state := payload["conversationState"].(map[string]any)
	current := state["currentMessage"].(map[string]any)
	userMsg := current["userInputMessage"].(map[string]any)
	ctx := userMsg["userInputMessageContext"].(map[string]any)
	tools, ok := ctx["tools"].([]any)
	if !ok {
		t.Fatalf("tools has type %T, want []any", ctx["tools"])
	}
	if len(tools) != 1 {
		t.Fatalf("tools length = %d, want 1", len(tools))
	}
	spec := tools[0].(map[string]any)["toolSpecification"].(map[string]any)
	if got := spec["name"]; got != "local_tool" {
		t.Fatalf("tool name = %v, want local_tool", got)
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

	current := payload["conversationState"].(map[string]any)["currentMessage"].(map[string]any)
	ctx := current["userInputMessage"].(map[string]any)["userInputMessageContext"].(map[string]any)
	tools := ctx["tools"].([]any)
	spec := tools[0].(map[string]any)["toolSpecification"].(map[string]any)
	if got := spec["name"].(string); got != shortName {
		t.Fatalf("tool spec name = %q, want %q", got, shortName)
	}

	history := payload["conversationState"].(map[string]any)["history"].([]any)
	assistant := history[0].(map[string]any)["assistantResponseMessage"].(map[string]any)
	if got := assistant["toolUses"].([]any)[0].(map[string]any)["name"].(string); got != shortName {
		t.Fatalf("history tool use name = %q, want %q", got, shortName)
	}
	if got := assistant["content"].(string); got == "" {
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

	state := payload["conversationState"].(map[string]any)
	history := state["history"].([]any)
	if len(history) != 5 {
		t.Fatalf("history length = %d, want 5", len(history))
	}

	firstAssistant := history[0].(map[string]any)["assistantResponseMessage"].(map[string]any)
	if _, ok := firstAssistant["toolUses"]; ok {
		t.Fatal("orphan assistant tool_use should be removed")
	}
	if got := firstAssistant["content"].(string); got != "" {
		t.Fatalf("orphan assistant text content = %q, want empty", got)
	}

	firstUser := history[1].(map[string]any)["userInputMessage"].(map[string]any)
	if _, ok := firstUser["userInputMessageContext"]; ok {
		t.Fatal("orphan user tool_result should be removed")
	}
	if got := firstUser["content"].(string); got != "" {
		t.Fatalf("orphan user text content = %q, want empty", got)
	}

	thirdAssistant := history[4].(map[string]any)["assistantResponseMessage"].(map[string]any)
	toolUses := thirdAssistant["toolUses"].([]any)
	if len(toolUses) != 1 {
		t.Fatalf("current-paired assistant toolUses length = %d, want 1", len(toolUses))
	}
	if got := toolUses[0].(map[string]any)["toolUseId"].(string); got != "current-id" {
		t.Fatalf("current-paired assistant toolUseId = %q, want current-id", got)
	}
	if got := thirdAssistant["content"].(string); got != "I will call the requested tools." {
		t.Fatalf("current-paired assistant content = %q, want placeholder", got)
	}

	current := state["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	if got := current["content"].(string); got != "Here are the tool results." {
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

	system := payload["system"].(string)
	if !strings.Contains(system, "compacted to fit Kiro's available context window") {
		t.Fatalf("missing trim note in system: %q", system)
	}

	messages := payload["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("trimmed messages length = %d, want 1", len(messages))
	}
	last := messages[0].(map[string]any)
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

	system := payload["system"].(string)
	if !strings.Contains(system, "Compaction summary:") {
		t.Fatalf("missing compaction summary header: %q", system)
	}
	if !strings.Contains(system, "backend/internal/service/kiro_gateway_service.go") {
		t.Fatalf("missing key file in summary: %q", system)
	}
	if !strings.Contains(system, "Recent user requests:") {
		t.Fatalf("missing recent user requests section: %q", system)
	}

	messages := payload["messages"].([]any)
	if len(messages) != 4 {
		t.Fatalf("compacted messages length = %d, want 4", len(messages))
	}
	last := messages[len(messages)-1].(map[string]any)
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

	messages := payload["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("trimmed messages length = %d, want 1", len(messages))
	}
	first := messages[0].(map[string]any)
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

	compacted, dropped, changed, err := CompactAnthropicRequestToTokenBudget(input, 120)
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

	messages := payload["messages"].([]any)
	if len(messages) == 0 {
		t.Fatal("compacted messages should not be empty")
	}
	first := messages[0].(map[string]any)
	if role := first["role"]; role == "user" {
		content, _ := first["content"].([]any)
		for _, item := range content {
			block, _ := item.(map[string]any)
			if block["type"] == "tool_result" {
				t.Fatalf("compaction kept an orphan leading tool_result: %v", first)
			}
		}
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
