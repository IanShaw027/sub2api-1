package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AnthropicRequestToChatCompletions converts an Anthropic Messages request body
// into a Chat Completions request. This enables forwarding Anthropic-format
// requests to OpenAI-compatible upstreams (e.g., opencode.ai CC endpoint).
func AnthropicRequestToChatCompletions(req *AnthropicRequest) (*ChatCompletionsRequest, error) {
	cc := &ChatCompletionsRequest{
		Model:       req.Model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	if req.MaxTokens > 0 {
		mt := req.MaxTokens
		cc.MaxCompletionTokens = &mt
	}

	if len(req.StopSeqs) > 0 {
		stopJSON, _ := json.Marshal(req.StopSeqs)
		cc.Stop = stopJSON
	}

	if req.Stream {
		cc.StreamOptions = &ChatStreamOptions{IncludeUsage: true}
	}

	// Convert system
	if len(req.System) > 0 {
		sysContent := extractAnthropicSystemText(req.System)
		if sysContent != "" {
			raw, _ := json.Marshal(sysContent)
			cc.Messages = append(cc.Messages, ChatMessage{Role: "system", Content: raw})
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		chatMsgs, err := anthropicMessageToChat(msg)
		if err != nil {
			return nil, err
		}
		cc.Messages = append(cc.Messages, chatMsgs...)
	}

	serverToolChoiceTypes := make(map[string]string)
	for _, tool := range req.Tools {
		if chatTool, ok := anthropicServerToolToNativeChatTool(tool); ok {
			cc.Tools = append(cc.Tools, chatTool)
			recordAnthropicServerToolChoiceType(serverToolChoiceTypes, tool, chatTool.Type)
			continue
		}
		if strings.TrimSpace(tool.Type) != "" {
			return nil, fmt.Errorf("unsupported anthropic server tool for chat completions: %s", tool.Type)
		}
		cc.Tools = append(cc.Tools, ChatTool{
			Type: "function",
			Function: &ChatFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	// Convert tool_choice
	if len(req.ToolChoice) > 0 {
		cc.ToolChoice = convertAnthropicToolChoiceToCC(req.ToolChoice, serverToolChoiceTypes)
	}

	// Convert thinking/reasoning effort
	if req.Thinking != nil && (req.Thinking.Type == "enabled" || req.Thinking.Type == "adaptive") {
		cc.ReasoningEffort = "high"
	}
	if req.OutputConfig != nil && req.OutputConfig.Effort != "" {
		cc.ReasoningEffort = req.OutputConfig.Effort
	}

	return cc, nil
}

func anthropicServerToolToNativeChatTool(tool AnthropicTool) (ChatTool, bool) {
	toolType := strings.ToLower(strings.TrimSpace(tool.Type))
	switch {
	case strings.HasPrefix(toolType, "web_search"):
		return ChatTool{Type: "web_search"}, true
	case strings.HasPrefix(toolType, "web_fetch"):
		return ChatTool{Type: "web_fetch"}, true
	default:
		return ChatTool{}, false
	}
}

func recordAnthropicServerToolChoiceType(serverToolChoiceTypes map[string]string, tool AnthropicTool, chatType string) {
	if serverToolChoiceTypes == nil || chatType == "" {
		return
	}
	for _, key := range []string{tool.Name, tool.Type, chatType} {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" {
			serverToolChoiceTypes[key] = chatType
		}
	}
}

func extractAnthropicSystemText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []AnthropicContentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var text string
		for _, b := range blocks {
			if b.Type == "text" {
				text += b.Text
			}
		}
		return text
	}
	return ""
}

func anthropicMessageToChat(msg AnthropicMessage) ([]ChatMessage, error) {
	var contentStr string
	if json.Unmarshal(msg.Content, &contentStr) == nil {
		raw, _ := json.Marshal(contentStr)
		return []ChatMessage{{Role: msg.Role, Content: raw}}, nil
	}

	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil, err
	}

	if msg.Role == "user" {
		return anthropicUserBlocksToChat(blocks)
	}
	return anthropicAssistantBlocksToChat(blocks)
}

func anthropicUserBlocksToChat(blocks []AnthropicContentBlock) ([]ChatMessage, error) {
	var toolMsgs []ChatMessage
	var parts []ChatContentPart

	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, ChatContentPart{Type: "text", Text: b.Text})
		case "document":
			part, err := anthropicDocumentToChat(b)
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case "image":
			if uri := anthropicImageToDataURI(b.Source); uri != "" {
				parts = append(parts, ChatContentPart{
					Type:     "image_url",
					ImageURL: &ChatImageURL{URL: uri},
				})
			}
		case "tool_result":
			content := extractToolResultText(b.Content)
			raw, _ := json.Marshal(content)
			toolMsgs = append(toolMsgs, ChatMessage{
				Role:       "tool",
				Content:    raw,
				ToolCallID: b.ToolUseID,
			})
		}
	}

	msgs := make([]ChatMessage, 0, len(toolMsgs)+1)
	msgs = append(msgs, toolMsgs...)
	if len(parts) > 0 {
		raw, _ := json.Marshal(parts)
		msgs = append(msgs, ChatMessage{Role: "user", Content: raw})
	}
	return msgs, nil
}

func anthropicAssistantBlocksToChat(blocks []AnthropicContentBlock) ([]ChatMessage, error) {
	msg := ChatMessage{Role: "assistant"}
	var textParts string
	var toolCalls []ChatToolCall

	for _, b := range blocks {
		switch b.Type {
		case "text":
			textParts += b.Text
		case "thinking":
			msg.ReasoningContent += b.Thinking
		case "tool_use":
			toolCalls = append(toolCalls, ChatToolCall{
				ID:   b.ID,
				Type: "function",
				Function: ChatFunctionCall{
					Name:      b.Name,
					Arguments: anthropicToolUseArguments(b.Input),
				},
			})
		}
	}

	if textParts != "" {
		raw, _ := json.Marshal(textParts)
		msg.Content = raw
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	return []ChatMessage{msg}, nil
}

func extractToolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []AnthropicContentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var text string
		for _, b := range blocks {
			if b.Type == "text" {
				text += b.Text
			}
		}
		return text
	}
	return string(raw)
}

func convertAnthropicToolChoiceToCC(raw json.RawMessage, serverToolChoiceTypes map[string]string) json.RawMessage {
	var tc struct {
		Type string `json:"type"`
		Name string `json:"name,omitempty"`
	}
	if json.Unmarshal(raw, &tc) != nil {
		return nil
	}
	switch tc.Type {
	case "auto":
		out, _ := json.Marshal("auto")
		return out
	case "any":
		out, _ := json.Marshal("required")
		return out
	case "tool":
		if chatToolType := serverToolChoiceTypes[strings.ToLower(strings.TrimSpace(tc.Name))]; chatToolType != "" {
			out, _ := json.Marshal(struct {
				Type string `json:"type"`
			}{Type: chatToolType})
			return out
		}
		out, _ := json.Marshal(struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}{Type: "function", Function: struct {
			Name string `json:"name"`
		}{Name: tc.Name}})
		return out
	case "none":
		out, _ := json.Marshal("none")
		return out
	}
	return raw
}

func anthropicToolUseArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil {
		return encoded
	}
	return string(raw)
}

func anthropicToolUseInputFromArguments(arguments string) json.RawMessage {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return json.RawMessage("{}")
	}
	if json.Valid([]byte(arguments)) {
		return json.RawMessage(arguments)
	}
	quoted, _ := json.Marshal(arguments)
	return quoted
}
