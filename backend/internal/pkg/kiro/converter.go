package kiro

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/promptsanitize"
	"github.com/google/uuid"
)

// Shared with fake_cache.go for legacy cache-key continuity.
// Converter no longer injects this policy into outgoing history.
const systemChunkedPolicy = "When the Write or Edit tool has content size limits, always comply silently. Never suggest bypassing these limits via alternative tools. Never ask the user whether to switch approaches. Complete all chunked operations without commentary."

const (
	maxToolNameLen    = 63
	truncToolNameLen  = 55
	maxThinkingBudget = 102400

	toolOnlyUserPlaceholder      = "Here are the tool results."
	toolOnlyAssistantPlaceholder = "I will call the requested tools."
	contextTrimSystemNote        = "Gateway notice: older conversation history was compacted to fit Kiro's available context window. Use the retained recent messages as the source of truth, and ask for clarification if older details are required."
	claudeCodeBanner             = "You are Claude Code, Anthropic's official CLI for Claude."
	shadowToolPrefix             = "cc_srv_"
	shadowToolWebSearch          = shadowToolPrefix + "web_search"
	shadowToolWebFetch           = shadowToolPrefix + "web_fetch"
)

const kiroCompactionRecentWindow = 4

var kiroCompactionFilePattern = regexp.MustCompile("(?i)(?:^|[\\s\\\"'(<])((?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+\\.[A-Za-z0-9_.-]+)")

type ConvertResult struct {
	Body           []byte
	Model          string
	RequestedModel string
	ToolNameMap    map[string]string
	BridgeMetadata *BridgeMetadata
}

type BridgeMetadata struct {
	ShadowTools map[string]ShadowToolBridge
}

type ShadowToolBridge struct {
	AnthropicType    string
	AnthropicName    string
	AllowedDomains   []string
	BlockedDomains   []string
	MaxUses          int
	MaxContentTokens int
}

func ConvertAnthropicRequest(body []byte) (*ConvertResult, error) {
	return ConvertAnthropicRequestWithModel(body, "")
}

func ConvertAnthropicRequestWithModel(body []byte, requestedModelOverride string) (*ConvertResult, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	requestedModel := requestedModelOverride
	if strings.TrimSpace(requestedModel) == "" {
		requestedModel, _ = req["model"].(string)
	}
	modelID := MapModel(requestedModel)
	if modelID == "" {
		return nil, fmt.Errorf("unsupported kiro model: %s", requestedModel)
	}
	normalizeThinkingForRequestedModel(requestedModel, req)

	rawMessages, _ := req["messages"].([]any)
	if len(rawMessages) == 0 {
		return nil, fmt.Errorf("empty messages")
	}

	rawMessages = trimTrailingNonUserMessages(rawMessages)
	if len(rawMessages) == 0 {
		return nil, fmt.Errorf("empty messages")
	}

	lastMessageMap, ok := rawMessages[len(rawMessages)-1].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid last message")
	}

	currentContent, currentImages, currentToolResults := processUserContent(lastMessageMap["content"])
	tools, toolNameMap, bridgeMetadata := convertTools(req["tools"])
	tools = ensureHistoryTools(rawMessages[:len(rawMessages)-1], tools, toolNameMap)

	currentContext := map[string]any{}
	if len(tools) > 0 {
		currentContext["tools"] = tools
	}
	if len(currentToolResults) > 0 {
		currentContext["toolResults"] = currentToolResults
	}
	if currentContent == "" && len(currentToolResults) > 0 {
		currentContent = toolOnlyUserPlaceholder
	}

	currentUserMessage := map[string]any{
		"userInputMessageContext": currentContext,
		"content":                 currentContent,
		"modelId":                 modelID,
		"origin":                  "AI_EDITOR",
	}
	if len(currentImages) > 0 {
		currentUserMessage["images"] = currentImages
	}

	history := buildHistory(req, rawMessages, modelID, toolNameMap)
	history = cleanOrphanToolPairs(history, toolResultIDs(currentToolResults))
	conversationID := extractSessionID(req)
	if conversationID == "" {
		conversationID = uuid.NewString()
	}

	payload := map[string]any{
		"conversationState": map[string]any{
			"agentContinuationId": uuid.NewString(),
			"agentTaskType":       "vibe",
			"chatTriggerType":     "MANUAL",
			"currentMessage": map[string]any{
				"userInputMessage": currentUserMessage,
			},
			"conversationId": conversationID,
			"history":        history,
		},
	}

	if profileARN := stringField(req, "profile_arn"); profileARN != "" {
		payload["profileArn"] = profileARN
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &ConvertResult{
		Body:           encoded,
		Model:          modelID,
		RequestedModel: requestedModel,
		ToolNameMap:    toolNameMap,
		BridgeMetadata: bridgeMetadata,
	}, nil
}

func normalizeThinkingForRequestedModel(requestedModel string, req map[string]any) {
	if req == nil {
		return
	}

	thinking, _ := req["thinking"].(map[string]any)
	if thinking == nil {
		return
	}
	if !strings.EqualFold(stringField(thinking, "type"), "enabled") {
		return
	}
	if SupportsExtendedThinking(requestedModel) {
		return
	}

	effort := strings.TrimSpace(stringField(thinking, "thinking_effort"))
	if effort == "" {
		budget, _ := thinking["budget_tokens"].(float64)
		effort = thinkingBudgetTokensToEffort(int(budget))
	}

	thinking["type"] = "adaptive"
	thinking["thinking_effort"] = effort
	delete(thinking, "budget_tokens")
	if strings.TrimSpace(stringField(thinking, "display")) == "" {
		thinking["display"] = "summarized"
	}
	req["thinking"] = thinking
}

func EstimateInputTokens(body []byte) int {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return 0
	}

	var builder strings.Builder
	if systemText := joinSystem(req["system"]); systemText != "" {
		appendKiroTokenEstimateText(&builder, systemText)
	}
	if messages, _ := req["messages"].([]any); len(messages) > 0 {
		for _, item := range messages {
			msg, _ := item.(map[string]any)
			appendKiroTokenEstimateContent(&builder, msg["content"])
		}
	}
	if tools, _ := req["tools"].([]any); len(tools) > 0 {
		for _, item := range tools {
			tool, _ := item.(map[string]any)
			appendKiroTokenEstimateText(&builder, stringField(tool, "name"))
			appendKiroTokenEstimateText(&builder, stringField(tool, "description"))
			appendKiroTokenEstimateJSON(&builder, tool["input_schema"])
		}
	}
	return AccurateTokenCount(builder.String())
}

// TrimAnthropicRequestToTokenBudget drops the oldest conversation messages
// until the request fits within budgetTokens. The current user turn is always
// preserved. When trimming occurs, a compacting note is injected into system.
func TrimAnthropicRequestToTokenBudget(body []byte, budgetTokens int) ([]byte, int, bool, error) {
	if budgetTokens <= 0 {
		return body, 0, false, nil
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, 0, false, err
	}

	rawMessages, _ := req["messages"].([]any)
	if len(rawMessages) <= 1 {
		return body, 0, false, nil
	}

	originalTokens := EstimateInputTokens(body)
	if originalTokens <= budgetTokens {
		return body, 0, false, nil
	}

	req["system"] = appendSystemNote(req["system"], contextTrimSystemNote)
	currentMessages := rawMessages
	dropped := 0

	for {
		encoded, err := json.Marshal(req)
		if err != nil {
			return nil, dropped, dropped > 0, err
		}
		if EstimateInputTokens(encoded) <= budgetTokens {
			return encoded, dropped, dropped > 0, nil
		}

		if len(currentMessages) <= 1 {
			break
		}

		dropCount := kiroTrimDropCount(currentMessages)
		if dropCount == 0 {
			break
		}

		currentMessages = currentMessages[dropCount:]
		req["messages"] = currentMessages
		dropped += dropCount
	}

	encoded, err := json.Marshal(req)
	if err != nil {
		return nil, dropped, dropped > 0, err
	}
	return encoded, dropped, dropped > 0, nil
}

// CompactAnthropicRequestToTokenBudget performs summary-based compaction.
// It first drops the oldest messages until the request is near budget, then
// injects a concise summary of the dropped context into system and keeps
// trimming the retained window only if needed.
func CompactAnthropicRequestToTokenBudget(body []byte, budgetTokens int) ([]byte, int, bool, error) {
	if budgetTokens <= 0 {
		return body, 0, false, nil
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, 0, false, err
	}

	rawMessages, _ := req["messages"].([]any)
	if len(rawMessages) <= 1 {
		return body, 0, false, nil
	}

	originalTokens := EstimateInputTokens(body)
	if originalTokens <= budgetTokens {
		return body, 0, false, nil
	}

	for recentWindow := kiroCompactionRecentWindow; recentWindow >= 1; recentWindow-- {
		if len(rawMessages) <= recentWindow {
			continue
		}

		start := kiroCompactionStartIndex(rawMessages, recentWindow)
		if start <= 0 || start >= len(rawMessages) {
			continue
		}

		droppedMessages := cloneKiroMessages(rawMessages[:start])
		recentMessages := rawMessages[start:]
		summary := summarizeDroppedAnthropicMessages(droppedMessages)

		reqCopy := cloneKiroRequest(req)
		if summary != "" {
			reqCopy["system"] = appendSystemNote(reqCopy["system"], summary)
		}

		encoded, err := marshalKiroCompactionRequest(reqCopy, recentMessages)
		if err != nil {
			return nil, 0, false, err
		}
		if EstimateInputTokens(encoded) <= budgetTokens {
			return encoded, len(droppedMessages), len(droppedMessages) > 0, nil
		}
	}

	trimmedBody, dropped, changed, trimErr := TrimAnthropicRequestToTokenBudget(body, budgetTokens)
	if trimErr != nil {
		return nil, 0, false, trimErr
	}
	if changed && EstimateInputTokens(trimmedBody) <= budgetTokens {
		return trimmedBody, dropped, true, nil
	}
	return nil, 0, false, fmt.Errorf("kiro request exceeds context budget after compaction (%d > %d tokens)", originalTokens, budgetTokens)
}

func EstimateOutputTokens(text string) int {
	return AccurateTokenCount(text)
}

func trimTrailingNonUserMessages(messages []any) []any {
	lastUser := -1
	for idx, item := range messages {
		msg, _ := item.(map[string]any)
		if strings.EqualFold(stringField(msg, "role"), "user") {
			lastUser = idx
		}
	}
	if lastUser < 0 {
		return nil
	}
	return messages[:lastUser+1]
}

func buildHistory(req map[string]any, messages []any, modelID string, toolNameMap map[string]string) []map[string]any {
	history := make([]map[string]any, 0)

	thinkingPrefix := buildThinkingPrefix(req["thinking"])
	if systemContent := joinSystem(req["system"]); systemContent != "" || thinkingPrefix != "" {
		content := strings.TrimSpace(strings.Join([]string{thinkingPrefix, systemContent}, "\n"))
		history = append(history, map[string]any{
			"userInputMessage": map[string]any{
				"content": content,
				"modelId": modelID,
				"origin":  "AI_EDITOR",
			},
		})
	}

	for idx := 0; idx < len(messages)-1; idx++ {
		msg, _ := messages[idx].(map[string]any)
		role := strings.ToLower(strings.TrimSpace(stringField(msg, "role")))
		switch role {
		case "user":
			text, images, toolResults := processUserContent(msg["content"])
			if text == "" && len(toolResults) > 0 {
				text = toolOnlyUserPlaceholder
			}
			userMessage := map[string]any{
				"content": text,
				"modelId": modelID,
				"origin":  "AI_EDITOR",
			}
			if len(images) > 0 {
				userMessage["images"] = images
			}
			if len(toolResults) > 0 {
				userMessage["userInputMessageContext"] = map[string]any{
					"toolResults": toolResults,
				}
			}
			history = append(history, map[string]any{"userInputMessage": userMessage})
		case "assistant":
			text, toolUses := processAssistantContent(msg["content"], toolNameMap)
			if text == "" && len(toolUses) > 0 {
				text = toolOnlyAssistantPlaceholder
			}
			assistantMessage := map[string]any{
				"content": text,
			}
			if len(toolUses) > 0 {
				assistantMessage["toolUses"] = toolUses
			}
			history = append(history, map[string]any{"assistantResponseMessage": assistantMessage})
		}
	}

	return history
}

func processUserContent(content any) (string, []map[string]any, []map[string]any) {
	textParts := make([]string, 0)
	images := make([]map[string]any, 0)
	toolResults := make([]map[string]any, 0)

	switch v := content.(type) {
	case string:
		textParts = append(textParts, v)
	case []any:
		for _, item := range v {
			block, _ := item.(map[string]any)
			switch strings.TrimSpace(stringField(block, "type")) {
			case "text":
				if text := stringField(block, "text"); text != "" {
					textParts = append(textParts, text)
				}
			case "image":
				if image := convertImage(block); image != nil {
					images = append(images, image)
				}
			case "tool_result":
				toolResult := map[string]any{
					"toolUseId": stringField(block, "tool_use_id"),
					"content": []map[string]any{
						{
							"text": toolResultContent(block["content"]),
						},
					},
				}
				if isError, ok := block["is_error"].(bool); ok && isError {
					toolResult["status"] = "error"
					toolResult["isError"] = true
				} else {
					toolResult["status"] = "success"
				}
				toolResults = append(toolResults, toolResult)
			}
		}
	}

	return strings.Join(textParts, "\n"), images, toolResults
}

func processAssistantContent(content any, toolNameMap map[string]string) (string, []map[string]any) {
	textParts := make([]string, 0)
	toolUses := make([]map[string]any, 0)

	switch v := content.(type) {
	case string:
		textParts = append(textParts, v)
	case []any:
		for _, item := range v {
			block, _ := item.(map[string]any)
			switch strings.TrimSpace(stringField(block, "type")) {
			case "text":
				if text := stringField(block, "text"); text != "" {
					textParts = append(textParts, text)
				}
			case "tool_use":
				name := shortenToolName(stringField(block, "name"))
				rememberToolName(toolNameMap, name, stringField(block, "name"))
				toolUses = append(toolUses, map[string]any{
					"toolUseId": stringField(block, "id"),
					"name":      name,
					"input":     jsonValue(block["input"]),
				})
			case "thinking", "redacted_thinking":
				continue
			}
		}
	}

	return strings.Join(textParts, "\n"), toolUses
}

func convertTools(raw any) ([]map[string]any, map[string]string, *BridgeMetadata) {
	items, _ := raw.([]any)
	tools := make([]map[string]any, 0, len(items))
	toolNameMap := make(map[string]string)
	bridgeMetadata := &BridgeMetadata{ShadowTools: map[string]ShadowToolBridge{}}
	for _, item := range items {
		tool, _ := item.(map[string]any)
		if isUnsupportedServerTool(tool) {
			continue
		}
		if shadowName, bridge, ok := supportedShadowTool(tool); ok {
			if _, exists := bridgeMetadata.ShadowTools[shadowName]; !exists {
				tools = append(tools, makeShadowToolSpecification(shadowName))
				bridgeMetadata.ShadowTools[shadowName] = bridge
			}
			continue
		}
		name := stringField(tool, "name")
		if name == "" {
			continue
		}
		shortName := shortenToolName(name)
		rememberToolName(toolNameMap, shortName, name)
		description := stringField(tool, "description")
		schema := normalizeJSONSchema(jsonValue(tool["input_schema"]))
		tools = append(tools, map[string]any{
			"toolSpecification": map[string]any{
				"name":        shortName,
				"description": description,
				"inputSchema": map[string]any{
					"json": schema,
				},
			},
		})
	}
	if len(toolNameMap) == 0 {
		toolNameMap = nil
	}
	if len(bridgeMetadata.ShadowTools) == 0 {
		bridgeMetadata = nil
	}
	return tools, toolNameMap, bridgeMetadata
}

func isUnsupportedServerTool(tool map[string]any) bool {
	toolType := strings.TrimSpace(stringField(tool, "type"))
	return toolType == "server_tool"
}

func supportedShadowTool(tool map[string]any) (string, ShadowToolBridge, bool) {
	toolType := strings.ToLower(strings.TrimSpace(stringField(tool, "type")))
	switch {
	case strings.HasPrefix(toolType, "web_search"):
		return shadowToolWebSearch, ShadowToolBridge{
			AnthropicType: toolType,
			AnthropicName: strings.TrimSpace(stringField(tool, "name")),
		}, true
	case strings.HasPrefix(toolType, "web_fetch"):
		return shadowToolWebFetch, ShadowToolBridge{
			AnthropicType:    toolType,
			AnthropicName:    strings.TrimSpace(stringField(tool, "name")),
			AllowedDomains:   stringArrayField(tool["allowed_domains"]),
			BlockedDomains:   stringArrayField(tool["blocked_domains"]),
			MaxUses:          intField(tool["max_uses"]),
			MaxContentTokens: intField(tool["max_content_tokens"]),
		}, true
	default:
		return "", ShadowToolBridge{}, false
	}
}

func makeShadowToolSpecification(name string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"required":             []string{},
		"additionalProperties": false,
	}
	description := "Internal gateway bridge tool"
	switch name {
	case shadowToolWebSearch:
		description = "Internal gateway bridge for Anthropic web_search server tools"
		schema["properties"] = map[string]any{
			"query": map[string]any{"type": "string"},
		}
		schema["required"] = []string{"query"}
	case shadowToolWebFetch:
		description = "Internal gateway bridge for Anthropic web_fetch server tools"
		schema["properties"] = map[string]any{
			"url": map[string]any{"type": "string"},
		}
		schema["required"] = []string{"url"}
	}
	return map[string]any{
		"toolSpecification": map[string]any{
			"name":        name,
			"description": description,
			"inputSchema": map[string]any{
				"json": schema,
			},
		},
	}
}

func ensureHistoryTools(history []any, tools []map[string]any, toolNameMap map[string]string) []map[string]any {
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		spec, _ := tool["toolSpecification"].(map[string]any)
		name := stringField(spec, "name")
		if name != "" {
			seen[strings.ToLower(name)] = struct{}{}
		}
	}

	for _, item := range history {
		msg, _ := item.(map[string]any)
		if strings.ToLower(stringField(msg, "role")) != "assistant" {
			continue
		}
		_, toolUses := processAssistantContent(msg["content"], toolNameMap)
		for _, toolUse := range toolUses {
			name := strings.ToLower(stringField(toolUse, "name"))
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			tools = append(tools, map[string]any{
				"toolSpecification": map[string]any{
					"name":        toolUse["name"],
					"description": "Tool used in conversation history",
					"inputSchema": map[string]any{
						"json": map[string]any{
							"type":                 "object",
							"properties":           map[string]any{},
							"required":             []string{},
							"additionalProperties": true,
						},
					},
				},
			})
		}
	}

	return tools
}

func convertImage(block map[string]any) map[string]any {
	source, _ := block["source"].(map[string]any)
	if source == nil {
		return nil
	}
	mediaType := stringField(source, "media_type")
	data := stringField(source, "data")
	if mediaType == "" || data == "" {
		return nil
	}

	format := mediaType
	if idx := strings.IndexByte(format, '/'); idx >= 0 && idx < len(format)-1 {
		format = format[idx+1:]
	}

	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil
	}

	return map[string]any{
		"format": format,
		"source": map[string]any{
			"bytes": data,
		},
	}
}

func toolResultContent(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			block, _ := item.(map[string]any)
			if text := stringField(block, "text"); text != "" {
				parts = append(parts, text)
				continue
			}
			encoded, _ := json.Marshal(block)
			if len(encoded) > 0 {
				parts = append(parts, string(encoded))
			}
		}
		return strings.Join(parts, "\n")
	default:
		encoded, _ := json.Marshal(v)
		return string(encoded)
	}
}

func joinSystem(raw any) string {
	switch v := raw.(type) {
	case string:
		return promptsanitize.SystemText(filterBillingHeaderLine(v), claudeCodeBanner)
	case []any:
		lines := make([]string, 0, len(v))
		for _, item := range v {
			block, _ := item.(map[string]any)
			if text := stringField(block, "text"); text != "" {
				text = filterBillingHeaderLine(text)
				if text == "" {
					continue
				}
				lines = append(lines, text)
			}
		}
		return promptsanitize.SystemText(strings.Join(lines, "\n"), claudeCodeBanner)
	default:
		return ""
	}
}

func appendSystemNote(raw any, note string) any {
	note = strings.TrimSpace(note)
	if note == "" {
		return raw
	}

	if strings.Contains(joinSystem(raw), note) {
		return raw
	}

	switch v := raw.(type) {
	case nil:
		return []any{map[string]any{"type": "text", "text": note}}
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return note
		}
		return strings.TrimSpace(trimmed + "\n\n" + note)
	case []any:
		out := append([]any{}, v...)
		out = append(out, map[string]any{"type": "text", "text": note})
		return out
	default:
		return []any{map[string]any{"type": "text", "text": note}}
	}
}

func cloneKiroRequest(req map[string]any) map[string]any {
	if req == nil {
		return nil
	}
	out := make(map[string]any, len(req))
	for key, value := range req {
		out[key] = value
	}
	return out
}

func cloneKiroMessages(messages []any) []any {
	if len(messages) == 0 {
		return nil
	}
	out := make([]any, len(messages))
	copy(out, messages)
	return out
}

func kiroCompactionStartIndex(messages []any, recentWindow int) int {
	if len(messages) == 0 || recentWindow <= 0 {
		return -1
	}

	start := len(messages) - recentWindow
	if start <= 0 {
		return 0
	}
	if start >= len(messages) {
		return -1
	}
	if isKiroToolPairBoundary(messages[start-1], messages[start]) {
		start--
	}
	return start
}

func kiroTrimDropCount(messages []any) int {
	if len(messages) < 2 {
		return 0
	}
	if isKiroToolPairBoundary(messages[0], messages[1]) {
		if len(messages) <= 2 {
			return 0
		}
		return 2
	}
	return 1
}

func isKiroToolPairBoundary(prev, next any) bool {
	prevMsg, _ := prev.(map[string]any)
	nextMsg, _ := next.(map[string]any)
	if prevMsg == nil || nextMsg == nil {
		return false
	}
	if strings.ToLower(stringField(prevMsg, "role")) != "assistant" {
		return false
	}
	if strings.ToLower(stringField(nextMsg, "role")) != "user" {
		return false
	}

	_, toolUses := processAssistantContent(prevMsg["content"], nil)
	if len(toolUses) == 0 {
		return false
	}
	_, _, toolResults := processUserContent(nextMsg["content"])
	return len(toolResults) > 0
}

func appendKiroTokenEstimateText(builder *strings.Builder, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	_, _ = builder.WriteString(text)
	_, _ = builder.WriteString("\n")
}

func appendKiroTokenEstimateJSON(builder *strings.Builder, value any) {
	if value == nil {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 {
		return
	}
	_, _ = builder.Write(encoded)
	_, _ = builder.WriteString("\n")
}

func appendKiroTokenEstimateContent(builder *strings.Builder, content any) {
	switch v := content.(type) {
	case string:
		appendKiroTokenEstimateText(builder, v)
	case []any:
		for _, item := range v {
			block, _ := item.(map[string]any)
			switch strings.TrimSpace(stringField(block, "type")) {
			case "text":
				appendKiroTokenEstimateText(builder, stringField(block, "text"))
			case "tool_use":
				appendKiroTokenEstimateText(builder, stringField(block, "name"))
				appendKiroTokenEstimateText(builder, stringField(block, "id"))
				appendKiroTokenEstimateJSON(builder, block["input"])
			case "tool_result":
				appendKiroTokenEstimateText(builder, stringField(block, "tool_use_id"))
				appendKiroTokenEstimateText(builder, toolResultContent(block["content"]))
			}
		}
	default:
		appendKiroTokenEstimateJSON(builder, v)
	}
}

func marshalKiroCompactionRequest(req map[string]any, messages []any) ([]byte, error) {
	reqCopy := make(map[string]any, len(req))
	for key, value := range req {
		reqCopy[key] = value
	}
	reqCopy["messages"] = messages
	return json.Marshal(reqCopy)
}

func summarizeDroppedAnthropicMessages(messages []any) string {
	if len(messages) == 0 {
		return ""
	}

	roleCounts := map[string]int{}
	toolNames := make([]string, 0, 8)
	keyFiles := make([]string, 0, 8)
	seenTools := map[string]struct{}{}
	seenFiles := map[string]struct{}{}
	recentUserRequests := make([]string, 0, 3)
	currentWork := ""

	addUnique := func(set map[string]struct{}, value string, out *[]string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := set[value]; ok {
			return
		}
		set[value] = struct{}{}
		*out = append(*out, value)
	}

	for idx, item := range messages {
		msg, _ := item.(map[string]any)
		if msg == nil {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(stringField(msg, "role")))
		if role == "" {
			role = "unknown"
		}
		roleCounts[role]++

		switch role {
		case "user":
			text, _, toolResults := processUserContent(msg["content"])
			if snippet := compactMessageSnippet(text); snippet != "" {
				if idx >= len(messages)-3 {
					recentUserRequests = append(recentUserRequests, snippet)
				}
				if currentWork == "" {
					currentWork = snippet
				}
			}
			if len(toolResults) > 0 && currentWork == "" {
				currentWork = fmt.Sprintf("tool results: %d", len(toolResults))
			}
		case "assistant":
			text, toolUses := processAssistantContent(msg["content"], nil)
			if snippet := compactMessageSnippet(text); snippet != "" && currentWork == "" {
				currentWork = snippet
			}
			for _, toolUse := range toolUses {
				addUnique(seenTools, stringField(toolUse, "name"), &toolNames)
			}
		default:
			if snippet := compactMessageSnippet(joinSystem(msg["content"])); snippet != "" && currentWork == "" {
				currentWork = snippet
			}
		}

		for _, filePath := range extractKiroCompactionFilePaths(msg["content"]) {
			addUnique(seenFiles, filePath, &keyFiles)
		}
	}

	lines := []string{
		"Compaction summary:",
		fmt.Sprintf("dropped=%d user=%d assistant=%d system=%d", len(messages), roleCounts["user"], roleCounts["assistant"], roleCounts["system"]),
	}
	if len(recentUserRequests) > 0 {
		lines = append(lines, "Recent user requests:")
		for _, req := range recentUserRequests {
			lines = append(lines, "- "+req)
		}
	}
	if len(keyFiles) > 0 {
		lines = append(lines, "Key files: "+strings.Join(keyFiles, ", "))
	}
	if len(toolNames) > 0 {
		lines = append(lines, "Tools: "+strings.Join(toolNames, ", "))
	}
	if currentWork != "" {
		lines = append(lines, "Current work: "+currentWork)
	}

	return truncateKiroSummaryText(strings.Join(lines, "\n"), 1200)
}

func compactMessageSnippet(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len([]rune(text)) <= 100 {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:100])) + "..."
}

func extractKiroCompactionFilePaths(content any) []string {
	text := joinSystem(content)
	if text == "" {
		text = extractTextFromContent(content)
	}
	if text == "" {
		return nil
	}
	matches := kiroCompactionFilePattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		filePath := strings.Trim(match[1], " \t\n\r\"'`<>(),[]{}")
		if filePath != "" {
			out = append(out, filePath)
		}
	}
	return out
}

func truncateKiroSummaryText(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if maxLen <= 0 || text == "" {
		return ""
	}
	if len([]rune(text)) <= maxLen {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:maxLen])) + "..."
}

func extractTextFromContent(content any) string {
	text, _, _ := processUserContent(content)
	if text != "" {
		return text
	}
	text, _ = processAssistantContent(content, nil)
	return text
}

func extractSessionID(req map[string]any) string {
	metadata, _ := req["metadata"].(map[string]any)
	userID := stringField(metadata, "user_id")
	if userID == "" {
		return ""
	}
	if strings.HasPrefix(strings.TrimSpace(userID), "{") {
		var parsed struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(userID), &parsed); err == nil {
			if _, err := uuid.Parse(parsed.SessionID); err == nil {
				return parsed.SessionID
			}
		}
	}
	pos := strings.Index(userID, "session_")
	if pos < 0 {
		return ""
	}
	sessionPart := userID[pos+len("session_"):]
	if len(sessionPart) < 36 {
		return ""
	}
	candidate := sessionPart[:36]
	if _, err := uuid.Parse(candidate); err != nil {
		return ""
	}
	return candidate
}

func normalizeJSONSchema(raw any) map[string]any {
	obj, _ := raw.(map[string]any)
	if obj == nil {
		return map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": true,
		}
	}

	if typeName := stringField(obj, "type"); typeName == "" {
		obj["type"] = "object"
	}

	if _, ok := obj["properties"].(map[string]any); !ok {
		obj["properties"] = map[string]any{}
	}

	switch required := obj["required"].(type) {
	case []any:
		out := make([]string, 0, len(required))
		for _, item := range required {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				out = append(out, value)
			}
		}
		obj["required"] = out
	case []string:
	default:
		obj["required"] = []string{}
	}

	switch obj["additionalProperties"].(type) {
	case bool, map[string]any:
	default:
		obj["additionalProperties"] = true
	}

	return obj
}

func buildThinkingPrefix(raw any) string {
	thinking, _ := raw.(map[string]any)
	if thinking == nil {
		return ""
	}
	display := stringField(thinking, "display")
	displayTag := ""
	if display != "" {
		displayTag = fmt.Sprintf("<thinking_display>%s</thinking_display>", display)
	}
	switch stringField(thinking, "type") {
	case "enabled":
		budget, _ := thinking["budget_tokens"].(float64)
		budgetTokens := int(budget)
		if budgetTokens > maxThinkingBudget {
			budgetTokens = maxThinkingBudget
		}
		if budgetTokens <= 0 {
			budgetTokens = maxThinkingBudget
		}
		return fmt.Sprintf("<thinking_mode>enabled</thinking_mode><max_thinking_length>%d</max_thinking_length>%s", budgetTokens, displayTag)
	case "adaptive":
		effort := stringField(thinking, "thinking_effort")
		if effort == "" {
			effort = "medium"
		}
		return fmt.Sprintf("<thinking_mode>adaptive</thinking_mode><thinking_effort>%s</thinking_effort>%s", effort, displayTag)
	default:
		return ""
	}
}

func thinkingBudgetTokensToEffort(budget int) string {
	switch {
	case budget <= 4096:
		return "low"
	case budget <= 16384:
		return "medium"
	case budget <= 65536:
		return "high"
	default:
		return "max"
	}
}

func filterBillingHeaderLine(text string) string {
	return strings.TrimSpace(strings.Join(filterBillingHeaderLines(strings.Split(text, "\n")), "\n"))
}

func filterBillingHeaderLines(lines []string) []string {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "x-anthropic-billing-header:") {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered
}

func rememberToolName(toolNameMap map[string]string, shortName, originalName string) {
	if toolNameMap == nil || shortName == "" || originalName == "" || shortName == originalName {
		return
	}
	if _, exists := toolNameMap[shortName]; !exists {
		toolNameMap[shortName] = originalName
	}
}

func shortenToolName(name string) string {
	if len(name) <= maxToolNameLen {
		return name
	}
	if strings.HasPrefix(name, "mcp__") {
		if idx := strings.LastIndex(name, "__"); idx > 4 {
			candidate := "mcp__" + name[idx+2:]
			if len(candidate) <= maxToolNameLen {
				return candidate
			}
		}
	}
	sum := sha256.Sum256([]byte(name))
	hash := fmt.Sprintf("%x", sum[:])
	return name[:truncToolNameLen] + "_" + hash[:7]
}

func toolResultIDs(toolResults []map[string]any) map[string]struct{} {
	if len(toolResults) == 0 {
		return nil
	}
	ids := make(map[string]struct{}, len(toolResults))
	for _, toolResult := range toolResults {
		id := stringField(toolResult, "toolUseId")
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids
}

func cleanOrphanToolPairs(history []map[string]any, currentResultIDs map[string]struct{}) []map[string]any {
	if len(history) == 0 {
		return history
	}

	toolUseIDs := make(map[string]struct{})
	toolResultIDs := make(map[string]struct{})
	for id := range currentResultIDs {
		toolResultIDs[id] = struct{}{}
	}

	for _, entry := range history {
		if assistant, ok := entry["assistantResponseMessage"].(map[string]any); ok {
			if uses, ok := assistant["toolUses"].([]map[string]any); ok {
				for _, toolUse := range uses {
					if id := stringField(toolUse, "toolUseId"); id != "" {
						toolUseIDs[id] = struct{}{}
					}
				}
			}
		}
		if user, ok := entry["userInputMessage"].(map[string]any); ok {
			if ctx, ok := user["userInputMessageContext"].(map[string]any); ok {
				if results, ok := ctx["toolResults"].([]map[string]any); ok {
					for _, toolResult := range results {
						if id := stringField(toolResult, "toolUseId"); id != "" {
							toolResultIDs[id] = struct{}{}
						}
					}
				}
			}
		}
	}

	for _, entry := range history {
		if assistant, ok := entry["assistantResponseMessage"].(map[string]any); ok {
			if uses, ok := assistant["toolUses"].([]map[string]any); ok {
				validUses := uses[:0]
				for _, toolUse := range uses {
					if id := stringField(toolUse, "toolUseId"); id != "" {
						if _, ok := toolResultIDs[id]; ok {
							validUses = append(validUses, toolUse)
						}
					}
				}
				if len(validUses) == 0 {
					delete(assistant, "toolUses")
					if stringField(assistant, "content") == toolOnlyAssistantPlaceholder {
						assistant["content"] = ""
					}
				} else {
					assistant["toolUses"] = validUses
				}
			}
		}

		if user, ok := entry["userInputMessage"].(map[string]any); ok {
			if ctx, ok := user["userInputMessageContext"].(map[string]any); ok {
				if results, ok := ctx["toolResults"].([]map[string]any); ok {
					validResults := results[:0]
					for _, toolResult := range results {
						if id := stringField(toolResult, "toolUseId"); id != "" {
							if _, ok := toolUseIDs[id]; ok {
								validResults = append(validResults, toolResult)
							}
						}
					}
					if len(validResults) == 0 {
						delete(ctx, "toolResults")
						if stringField(user, "content") == toolOnlyUserPlaceholder {
							user["content"] = ""
						}
					} else {
						ctx["toolResults"] = validResults
					}
				}
				if len(ctx) == 0 {
					delete(user, "userInputMessageContext")
				}
			}
		}
	}

	return history
}

func stringField(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	switch v := obj[key].(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func stringArrayField(value any) []string {
	items, _ := value.([]any)
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func intField(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func jsonValue(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}
