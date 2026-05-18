package kiro

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

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
)

type ConvertResult struct {
	Body           []byte
	Model          string
	RequestedModel string
	ToolNameMap    map[string]string
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
	tools, toolNameMap := convertTools(req["tools"])
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
	}, nil
}

func EstimateInputTokens(body []byte) int {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return 0
	}

	var builder strings.Builder
	if systemText := joinSystem(req["system"]); systemText != "" {
		_, _ = builder.WriteString(systemText)
		_, _ = builder.WriteString("\n")
	}
	if messages, _ := req["messages"].([]any); len(messages) > 0 {
		for _, item := range messages {
			msg, _ := item.(map[string]any)
			_, _ = builder.WriteString(extractTextFromContent(msg["content"]))
			_, _ = builder.WriteString("\n")
		}
	}
	if tools, _ := req["tools"].([]any); len(tools) > 0 {
		for _, item := range tools {
			tool, _ := item.(map[string]any)
			_, _ = builder.WriteString(stringField(tool, "name"))
			_, _ = builder.WriteString("\n")
			_, _ = builder.WriteString(stringField(tool, "description"))
			_, _ = builder.WriteString("\n")
		}
	}
	return AccurateTokenCount(builder.String())
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

func convertTools(raw any) ([]map[string]any, map[string]string) {
	items, _ := raw.([]any)
	tools := make([]map[string]any, 0, len(items))
	toolNameMap := make(map[string]string)
	for _, item := range items {
		tool, _ := item.(map[string]any)
		if isUnsupportedServerTool(tool) {
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
	return tools, toolNameMap
}

func isUnsupportedServerTool(tool map[string]any) bool {
	toolType := strings.TrimSpace(stringField(tool, "type"))
	if toolType == "server_tool" {
		return true
	}
	return strings.HasPrefix(toolType, "web_search")
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
		return strings.TrimSpace(filterBillingHeaderLine(v))
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
		return strings.TrimSpace(strings.Join(lines, "\n"))
	default:
		return ""
	}
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
		return fmt.Sprintf("<thinking_mode>enabled</thinking_mode><max_thinking_length>%d</max_thinking_length>", budgetTokens)
	case "adaptive":
		effort := stringField(thinking, "thinking_effort")
		if effort == "" {
			effort = "medium"
		}
		return fmt.Sprintf("<thinking_mode>adaptive</thinking_mode><thinking_effort>%s</thinking_effort>", effort)
	default:
		return ""
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

func jsonValue(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}
