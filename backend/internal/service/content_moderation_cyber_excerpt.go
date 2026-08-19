package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxCyberPolicyInputExcerptRunes = 4000
	maxCyberPolicySemanticItemRunes = 1000
)

type cyberPolicySemanticItem struct {
	label string
	text  string
}

// BuildCyberPolicyInputExcerpt extracts ordered, client-controlled semantic
// content for post-upstream Cyber review. It deliberately includes tool calls
// and tool results, which the latest-user-only moderation extractor skips.
func BuildCyberPolicyInputExcerpt(protocol string, body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var document any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return truncateCyberSemanticText(strings.TrimSpace(string(body)), maxCyberPolicyInputExcerptRunes)
	}
	root, _ := document.(map[string]any)
	if root == nil {
		return truncateCyberSemanticText(compactCyberSemanticJSON(document), maxCyberPolicyInputExcerptRunes)
	}

	items := make([]cyberPolicySemanticItem, 0, 16)
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case ContentModerationProtocolOpenAIChat:
		collectCyberRootInstructions(root, &items)
		collectCyberMessages(root["messages"], &items)
	case ContentModerationProtocolAnthropicMessages:
		collectCyberAnthropicSystem(root["system"], &items)
		collectCyberMessages(root["messages"], &items)
	case ContentModerationProtocolOpenAIResponses:
		collectCyberResponsesRoot(root, &items)
	case ContentModerationProtocolGemini:
		collectCyberGeminiRoot(root, &items)
	case ContentModerationProtocolOpenAIImages:
		appendCyberSemanticItem(&items, "user", semanticCyberValue(root["prompt"]))
	default:
		collectCyberRootInstructions(root, &items)
		collectCyberMessages(root["messages"], &items)
		collectCyberResponsesInput(root["input"], &items)
		collectCyberGeminiRoot(root, &items)
	}
	if len(items) == 0 {
		return truncateCyberSemanticText(compactCyberSemanticJSON(document), maxCyberPolicyInputExcerptRunes)
	}
	return formatCyberSemanticItems(items, maxCyberPolicyInputExcerptRunes)
}

func collectCyberRootInstructions(root map[string]any, items *[]cyberPolicySemanticItem) {
	appendCyberSemanticItem(items, "system", semanticCyberValue(root["instructions"]))
}

func collectCyberAnthropicSystem(value any, items *[]cyberPolicySemanticItem) {
	appendCyberSemanticItem(items, "system", semanticCyberValue(value))
}

func collectCyberMessages(value any, items *[]cyberPolicySemanticItem) {
	messages, _ := value.([]any)
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(cyberString(message["role"])))
		if role == "" {
			role = "message"
		}
		collectCyberMessageContent(role, message["content"], items)
		collectCyberToolCalls(message["tool_calls"], items)
		if call, ok := message["function_call"].(map[string]any); ok {
			appendCyberSemanticItem(items, cyberToolCallLabel(call), cyberToolCallText(call))
		}
	}
}

func collectCyberMessageContent(role string, value any, items *[]cyberPolicySemanticItem) {
	blocks, ok := value.([]any)
	if !ok {
		appendCyberSemanticItem(items, role, semanticCyberValue(value))
		return
	}
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			appendCyberSemanticItem(items, role, semanticCyberValue(raw))
			continue
		}
		switch strings.ToLower(strings.TrimSpace(cyberString(block["type"]))) {
		case "tool_use", "function_call", "tool_call":
			appendCyberSemanticItem(items, cyberToolCallLabel(block), cyberToolCallText(block))
		case "tool_result", "function_call_output":
			text := semanticCyberValue(block["content"])
			if text == "" {
				text = semanticCyberValue(block["output"])
			}
			appendCyberSemanticItem(items, "tool result", text)
		default:
			appendCyberSemanticItem(items, role, semanticCyberValue(block))
		}
	}
}

func collectCyberToolCalls(value any, items *[]cyberPolicySemanticItem) {
	calls, _ := value.([]any)
	for _, raw := range calls {
		call, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		function, _ := call["function"].(map[string]any)
		if function == nil {
			function = call
		}
		appendCyberSemanticItem(items, cyberToolCallLabel(function), cyberToolCallText(function))
	}
}

func cyberToolCallLabel(call map[string]any) string {
	name := strings.TrimSpace(cyberString(call["name"]))
	if name == "" {
		return "assistant tool_call"
	}
	return "assistant tool_call " + name
}

func cyberToolCallText(call map[string]any) string {
	arguments := semanticCyberValue(call["arguments"])
	if arguments == "" {
		arguments = semanticCyberValue(call["input"])
	}
	return arguments
}

func collectCyberResponsesRoot(root map[string]any, items *[]cyberPolicySemanticItem) {
	container := root
	if response, ok := root["response"].(map[string]any); ok {
		container = response
	}
	collectCyberRootInstructions(container, items)
	collectCyberResponsesInput(container["input"], items)
}

func collectCyberResponsesInput(value any, items *[]cyberPolicySemanticItem) {
	switch typed := value.(type) {
	case string:
		appendCyberSemanticItem(items, "user", typed)
	case []any:
		for _, raw := range typed {
			collectCyberResponsesItem(raw, items)
		}
	case map[string]any:
		collectCyberResponsesItem(typed, items)
	}
}

func collectCyberResponsesItem(raw any, items *[]cyberPolicySemanticItem) {
	item, ok := raw.(map[string]any)
	if !ok {
		appendCyberSemanticItem(items, "input", semanticCyberValue(raw))
		return
	}
	typ := strings.ToLower(strings.TrimSpace(cyberString(item["type"])))
	role := strings.ToLower(strings.TrimSpace(cyberString(item["role"])))
	switch typ {
	case "function_call", "tool_call", "custom_tool_call", "computer_call":
		appendCyberSemanticItem(items, cyberToolCallLabel(item), cyberToolCallText(item))
	case "function_call_output", "tool_result", "custom_tool_call_output", "computer_call_output":
		text := semanticCyberValue(item["output"])
		if text == "" {
			text = semanticCyberValue(item["content"])
		}
		appendCyberSemanticItem(items, "tool result", text)
	case "message", "input_text", "output_text", "text", "":
		if role == "" {
			role = "user"
		}
		text := semanticCyberValue(item["content"])
		if text == "" {
			text = semanticCyberValue(item["text"])
		}
		appendCyberSemanticItem(items, role, text)
	default:
		label := typ
		if role != "" {
			label = role + " " + typ
		}
		appendCyberSemanticItem(items, label, semanticCyberValue(item))
	}
}

func collectCyberGeminiRoot(root map[string]any, items *[]cyberPolicySemanticItem) {
	appendCyberSemanticItem(items, "system", semanticCyberValue(root["systemInstruction"]))
	appendCyberSemanticItem(items, "system", semanticCyberValue(root["system_instruction"]))
	contents, _ := root["contents"].([]any)
	for _, raw := range contents {
		content, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(cyberString(content["role"])))
		if role == "model" {
			role = "assistant"
		}
		if role == "" {
			role = "user"
		}
		parts, _ := content["parts"].([]any)
		for _, rawPart := range parts {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			if call, ok := part["functionCall"].(map[string]any); ok {
				appendCyberSemanticItem(items, cyberToolCallLabel(call), semanticCyberValue(call["args"]))
				continue
			}
			if result, ok := part["functionResponse"].(map[string]any); ok {
				appendCyberSemanticItem(items, "tool result "+strings.TrimSpace(cyberString(result["name"])), semanticCyberValue(result["response"]))
				continue
			}
			appendCyberSemanticItem(items, role, semanticCyberValue(part["text"]))
		}
	}
}

func semanticCyberValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, raw := range typed {
			if text := semanticCyberValue(raw); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		typ := strings.ToLower(strings.TrimSpace(cyberString(typed["type"])))
		switch typ {
		case "text", "input_text", "output_text":
			return semanticCyberValue(typed["text"])
		case "tool_use", "function_call", "tool_call":
			return strings.TrimSpace(strings.Join([]string{cyberString(typed["name"]), semanticCyberValue(typed["input"]), semanticCyberValue(typed["arguments"])}, " "))
		case "tool_result", "function_call_output":
			if text := semanticCyberValue(typed["content"]); text != "" {
				return text
			}
			return semanticCyberValue(typed["output"])
		case "image", "input_image", "image_url":
			return "[image omitted]"
		}
		if text := semanticCyberValue(typed["text"]); text != "" {
			return text
		}
		if content := semanticCyberValue(typed["content"]); content != "" {
			return content
		}
		return compactCyberSemanticJSON(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func appendCyberSemanticItem(items *[]cyberPolicySemanticItem, label, text string) {
	label = strings.TrimSpace(label)
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if label == "" {
		label = "input"
	}
	*items = append(*items, cyberPolicySemanticItem{label: label, text: truncateCyberSemanticText(text, maxCyberPolicySemanticItemRunes)})
}

func formatCyberSemanticItems(items []cyberPolicySemanticItem, maxRunes int) string {
	formatted := make([]string, 0, len(items))
	for _, item := range items {
		formatted = append(formatted, fmt.Sprintf("[%s]\n%s", item.label, item.text))
	}
	joined := strings.Join(formatted, "\n\n")
	if utf8.RuneCountInString(joined) <= maxRunes {
		return joined
	}

	// Keep root instructions plus the newest turns. The omission marker makes
	// it explicit that the excerpt is bounded rather than a complete transcript.
	selected := make([]string, 0, len(formatted))
	used := 0
	start := 0
	if len(formatted) > 1 && (items[0].label == "system" || items[0].label == "developer") {
		selected = append(selected, formatted[0])
		used = utf8.RuneCountInString(formatted[0]) + 2
		start = 1
	}
	for index := len(formatted) - 1; index >= start; index-- {
		length := utf8.RuneCountInString(formatted[index]) + 2
		if used+length+64 > maxRunes {
			continue
		}
		selected = append(selected, formatted[index])
		used += length
	}
	for left, right := start, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}
	omitted := len(formatted) - len(selected)
	if omitted > 0 {
		insertAt := 0
		if start == 1 {
			insertAt = 1
		}
		marker := fmt.Sprintf("[... %d earlier semantic item(s) omitted ...]", omitted)
		selected = append(selected, "")
		copy(selected[insertAt+1:], selected[insertAt:])
		selected[insertAt] = marker
	}
	return truncateCyberSemanticText(strings.Join(selected, "\n\n"), maxRunes)
}

func truncateCyberSemanticText(value string, maxRunes int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes-1]) + "…"
}

func compactCyberSemanticJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func cyberString(value any) string {
	text, _ := value.(string)
	return text
}
