package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResponsesToAnthropicRequest converts a Responses API request into an
// Anthropic Messages request. This is the reverse of AnthropicToResponses and
// enables Anthropic platform groups to accept OpenAI Responses API requests
// by converting them to the native /v1/messages format before forwarding upstream.
func ResponsesToAnthropicRequest(req *ResponsesRequest) (*AnthropicRequest, error) {
	system, messages, err := convertResponsesInputToAnthropic(req.Input)
	if err != nil {
		return nil, err
	}

	out := &AnthropicRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	if len(system) > 0 {
		out.System = system
	}

	// max_output_tokens → max_tokens
	if req.MaxOutputTokens != nil && *req.MaxOutputTokens > 0 {
		out.MaxTokens = *req.MaxOutputTokens
	}
	if out.MaxTokens == 0 {
		// Anthropic requires max_tokens; default to a sensible value.
		out.MaxTokens = 8192
	}

	// Convert tools
	if len(req.Tools) > 0 {
		out.Tools = convertResponsesToAnthropicTools(req.Tools)
	}

	// Convert tool_choice (reverse of convertAnthropicToolChoiceToResponses)
	if len(req.ToolChoice) > 0 {
		tc, err := convertResponsesToAnthropicToolChoice(req.ToolChoice, req.ParallelToolCalls)
		if err != nil {
			return nil, fmt.Errorf("convert tool_choice: %w", err)
		}
		out.ToolChoice = tc
	}

	// reasoning.effort → output_config.effort + thinking
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		effort := mapResponsesEffortToAnthropic(req.Reasoning.Effort)
		out.OutputConfig = &AnthropicOutputConfig{Effort: effort}
		// Enable thinking for non-low efforts
		if effort != "low" {
			out.Thinking = &AnthropicThinking{
				Type:         "enabled",
				BudgetTokens: defaultThinkingBudget(effort),
			}
		}
	}

	return out, nil
}

// defaultThinkingBudget returns a sensible thinking budget based on effort level.
func defaultThinkingBudget(effort string) int {
	switch effort {
	case "low":
		return 1024
	case "medium":
		return 4096
	case "high":
		return 10240
	case "max":
		return 32768
	default:
		return 10240
	}
}

// mapResponsesEffortToAnthropic converts OpenAI Responses reasoning effort to
// Anthropic effort levels. Reverse of mapAnthropicEffortToResponses.
//
//	low    → low
//	medium → medium
//	high   → high
//	xhigh  → max
func mapResponsesEffortToAnthropic(effort string) string {
	if effort == "xhigh" {
		return "max"
	}
	return effort // low→low, medium→medium, high→high, unknown→passthrough
}

// convertResponsesInputToAnthropic extracts system prompt and messages from
// a Responses API input array. Returns the system as raw JSON (for Anthropic's
// polymorphic system field) and a list of Anthropic messages.
func convertResponsesInputToAnthropic(inputRaw json.RawMessage) (json.RawMessage, []AnthropicMessage, error) {
	// Try as plain string input.
	var inputStr string
	if err := json.Unmarshal(inputRaw, &inputStr); err == nil {
		content, _ := json.Marshal(inputStr)
		return nil, []AnthropicMessage{{Role: "user", Content: content}}, nil
	}

	var items []ResponsesInputItem
	if err := json.Unmarshal(inputRaw, &items); err != nil {
		return nil, nil, fmt.Errorf("parse responses input: %w", err)
	}

	var system json.RawMessage
	var messages []AnthropicMessage

	for _, item := range items {
		switch {
		case item.Role == "system":
			// System prompt → Anthropic system field
			text := extractTextFromContent(item.Content)
			if text != "" {
				system, _ = json.Marshal(text)
			}

		case item.Type == "function_call":
			if strings.TrimSpace(item.Name) == "webfetch" {
				if assistantMsg, userMsg, hasResult, ok := webFetchHistoryMessagesFromFunctionCall(item); ok {
					messages = append(messages, assistantMsg)
					if hasResult {
						messages = append(messages, userMsg)
					}
					continue
				}
			}
			// function_call → assistant message with tool_use block
			input := json.RawMessage("{}")
			if item.Arguments != "" {
				input = json.RawMessage(item.Arguments)
			}
			block := AnthropicContentBlock{
				Type:  "tool_use",
				ID:    fromResponsesCallIDToAnthropic(item.CallID),
				Name:  item.Name,
				Input: input,
			}
			blockJSON, _ := json.Marshal([]AnthropicContentBlock{block})
			messages = append(messages, AnthropicMessage{
				Role:    "assistant",
				Content: blockJSON,
			})

		case item.Type == "web_search_call":
			assistantMsg, userMsg := webSearchHistoryMessagesFromCall(item)
			messages = append(messages, assistantMsg)
			if len(webSearchSourcesFromCall(item)) > 0 {
				messages = append(messages, userMsg)
			}

		case item.Type == "function_call_output":
			// function_call_output → user message with tool_result block
			outputContent := item.Output
			if outputContent == "" {
				outputContent = "(empty)"
			}
			contentJSON, isError := marshalAnthropicToolResultContent(outputContent)
			block := AnthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: fromResponsesCallIDToAnthropic(item.CallID),
				Content:   contentJSON,
				IsError:   isError,
			}
			blockJSON, _ := json.Marshal([]AnthropicContentBlock{block})
			messages = append(messages, AnthropicMessage{
				Role:    "user",
				Content: blockJSON,
			})

		case item.Role == "user":
			content, err := convertResponsesUserToAnthropicContent(item.Content)
			if err != nil {
				return nil, nil, err
			}
			messages = append(messages, AnthropicMessage{
				Role:    "user",
				Content: content,
			})

		case item.Role == "assistant":
			content, err := convertResponsesAssistantToAnthropicContent(item.Content)
			if err != nil {
				return nil, nil, err
			}
			messages = append(messages, AnthropicMessage{
				Role:    "assistant",
				Content: content,
			})

		default:
			// Unknown role/type — attempt as user message
			if item.Content != nil {
				messages = append(messages, AnthropicMessage{
					Role:    "user",
					Content: item.Content,
				})
			}
		}
	}

	// Repair tool_use/tool_result pairing, then merge consecutive same-role
	// messages (Anthropic requires alternating roles). The first merge groups
	// parallel calls (and their results) so the pairing pass sees them together;
	// the pairing pass may re-split a user turn (e.g. when an injected message
	// sat between a call and its output), so a second merge restores alternation.
	messages = mergeConsecutiveMessages(messages)
	messages = normalizeAnthropicToolPairing(messages)
	messages = mergeConsecutiveMessages(messages)

	return system, messages, nil
}

// normalizeAnthropicToolPairing rebuilds the message sequence so it satisfies
// Anthropic's tool_use/tool_result invariants, which the naive item-by-item
// conversion violates whenever the Responses history interleaves anything
// between a function_call and its function_call_output.
func normalizeAnthropicToolPairing(messages []AnthropicMessage) []AnthropicMessage {
	results := make(map[string]AnthropicContentBlock)
	for _, m := range messages {
		if m.Role != "user" {
			continue
		}
		for _, b := range parseContentBlocks(m.Content) {
			if b.Type == "tool_result" && b.ToolUseID != "" {
				results[b.ToolUseID] = b
			}
		}
	}

	out := make([]AnthropicMessage, 0, len(messages))
	for _, m := range messages {
		blocks := parseContentBlocks(m.Content)
		switch m.Role {
		case "assistant":
			var toolUses, others []AnthropicContentBlock
			for _, b := range blocks {
				if b.Type == "tool_use" {
					toolUses = append(toolUses, b)
				} else {
					others = append(others, b)
				}
			}
			if len(toolUses) == 0 {
				out = append(out, m)
				continue
			}

			kept := make([]AnthropicContentBlock, 0, len(toolUses))
			for _, tu := range toolUses {
				if _, ok := results[tu.ID]; ok {
					kept = append(kept, tu)
				}
			}
			if len(kept) == 0 {
				if len(others) > 0 {
					out = append(out, anthropicMessageFromBlocks("assistant", others))
				}
				continue
			}

			asstBlocks := make([]AnthropicContentBlock, 0, len(others)+len(kept))
			asstBlocks = append(asstBlocks, others...)
			asstBlocks = append(asstBlocks, kept...)
			out = append(out, anthropicMessageFromBlocks("assistant", asstBlocks))

			resBlocks := make([]AnthropicContentBlock, 0, len(kept))
			for _, tu := range kept {
				resBlocks = append(resBlocks, results[tu.ID])
			}
			out = append(out, anthropicMessageFromBlocks("user", resBlocks))

		case "user":
			var nonResult []AnthropicContentBlock
			hasResult := false
			for _, b := range blocks {
				if b.Type == "tool_result" {
					hasResult = true
					continue
				}
				nonResult = append(nonResult, b)
			}
			if !hasResult {
				out = append(out, m)
				continue
			}
			if len(nonResult) > 0 {
				out = append(out, anthropicMessageFromBlocks("user", nonResult))
			}

		default:
			out = append(out, m)
		}
	}
	return out
}

func anthropicMessageFromBlocks(role string, blocks []AnthropicContentBlock) AnthropicMessage {
	content, _ := json.Marshal(blocks)
	return AnthropicMessage{Role: role, Content: content}
}

func webSearchHistoryMessagesFromCall(item ResponsesInputItem) (AnthropicMessage, AnthropicMessage) {
	query := ""
	if item.Action != nil {
		query = strings.TrimSpace(item.Action.Query)
	}
	input, _ := json.Marshal(map[string]any{"query": query})
	assistantBlocks, _ := json.Marshal([]AnthropicContentBlock{{
		Type:  "server_tool_use",
		ID:    restoreResponsesServerToolID(item.ID),
		Name:  "web_search",
		Input: input,
	}})
	userContent, _ := json.Marshal(webSearchSourcesFromCall(item))
	userBlocks, _ := json.Marshal([]AnthropicContentBlock{{
		Type:      "web_search_tool_result",
		ToolUseID: restoreResponsesServerToolID(item.ID),
		Content:   userContent,
	}})
	return AnthropicMessage{Role: "assistant", Content: assistantBlocks}, AnthropicMessage{Role: "user", Content: userBlocks}
}

func webSearchSourcesFromCall(item ResponsesInputItem) []map[string]any {
	if item.Action == nil || len(item.Action.Sources) == 0 {
		return nil
	}
	sources := make([]map[string]any, 0, len(item.Action.Sources))
	for _, source := range item.Action.Sources {
		sources = append(sources, map[string]any{
			"type":  "web_search_result",
			"url":   source.URL,
			"title": source.Title,
		})
	}
	return sources
}

func webFetchHistoryMessagesFromFunctionCall(item ResponsesInputItem) (AnthropicMessage, AnthropicMessage, bool, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(item.Arguments), &payload); err != nil {
		return AnthropicMessage{}, AnthropicMessage{}, false, false
	}
	rawResult, ok := payload["_sub2api_server_tool_result"]
	if ok {
		delete(payload, "_sub2api_server_tool_result")
	}
	input, _ := json.Marshal(payload)
	callID := fromResponsesCallIDToAnthropic(item.CallID)
	assistantBlocks, _ := json.Marshal([]AnthropicContentBlock{{
		Type:  "server_tool_use",
		ID:    callID,
		Name:  "web_fetch",
		Input: input,
	}})
	if !ok {
		return AnthropicMessage{Role: "assistant", Content: assistantBlocks}, AnthropicMessage{}, false, true
	}
	resultJSON, err := json.Marshal(rawResult)
	if err != nil {
		return AnthropicMessage{}, AnthropicMessage{}, false, false
	}
	userBlocks, _ := json.Marshal([]AnthropicContentBlock{{
		Type:      "web_fetch_tool_result",
		ToolUseID: callID,
		Content:   resultJSON,
	}})
	return AnthropicMessage{Role: "assistant", Content: assistantBlocks}, AnthropicMessage{Role: "user", Content: userBlocks}, true, true
}

func restoreResponsesServerToolID(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "srvtoolu_" + generateItemID()
	}
	if strings.HasPrefix(trimmed, "srvtoolu_") {
		return trimmed
	}
	return "srvtoolu_" + trimmed
}

func tryDecodeAnthropicToolResultEnvelope(raw string) (*anthropicToolResultEnvelope, bool) {
	var env anthropicToolResultEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, false
	}
	if env.Format != anthropicToolResultEnvelopeFormat {
		return nil, false
	}
	return &env, true
}

func marshalAnthropicToolResultContent(raw string) (json.RawMessage, bool) {
	if env, ok := tryDecodeAnthropicToolResultEnvelope(raw); ok {
		var blocks []AnthropicContentBlock
		for _, text := range env.Text {
			if text == "" {
				continue
			}
			blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: text})
		}
		for _, ref := range env.ToolReferences {
			if ref.ToolName == "" {
				continue
			}
			blocks = append(blocks, AnthropicContentBlock{Type: "tool_reference", ToolName: ref.ToolName})
		}
		if len(blocks) > 0 {
			contentJSON, _ := json.Marshal(blocks)
			return contentJSON, env.IsError
		}
		if env.IsError {
			contentJSON, _ := json.Marshal([]AnthropicContentBlock{})
			return contentJSON, true
		}
	}

	contentJSON, _ := json.Marshal(raw)
	return contentJSON, false
}

// extractTextFromContent extracts text from a content field that may be a
// plain string or an array of content parts.
func extractTextFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var texts []string
		for _, p := range parts {
			if (p.Type == "input_text" || p.Type == "output_text" || p.Type == "text") && p.Text != "" {
				texts = append(texts, p.Text)
			}
		}
		return strings.Join(texts, "\n\n")
	}
	return ""
}

// convertResponsesUserToAnthropicContent converts a Responses user message
// content field into Anthropic content blocks JSON.
func convertResponsesUserToAnthropicContent(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.Marshal("") // empty string content
	}

	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return json.Marshal(s)
	}

	// Array of content parts → Anthropic content blocks.
	var parts []ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		// Pass through as-is if we can't parse
		return raw, nil
	}

	var blocks []AnthropicContentBlock
	for _, p := range parts {
		switch p.Type {
		case "input_text", "text":
			if p.Text != "" {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "text",
					Text: p.Text,
				})
			}
		case "input_image":
			src := dataURIToAnthropicImageSource(p.ImageURL)
			if src != nil {
				blocks = append(blocks, AnthropicContentBlock{
					Type:   "image",
					Source: src,
				})
			}
		case "input_file":
			if block := responsesInputFileToAnthropicDocument(p); block != nil {
				blocks = append(blocks, *block)
			}
		}
	}

	if len(blocks) == 0 {
		return json.Marshal("")
	}
	return json.Marshal(blocks)
}

// convertResponsesAssistantToAnthropicContent converts a Responses assistant
// message content field into Anthropic content blocks JSON.
func convertResponsesAssistantToAnthropicContent(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.Marshal([]AnthropicContentBlock{{Type: "text", Text: ""}})
	}

	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return json.Marshal([]AnthropicContentBlock{{Type: "text", Text: s}})
	}

	// Array of content parts → Anthropic content blocks.
	var parts []ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return raw, nil
	}

	var blocks []AnthropicContentBlock
	for _, p := range parts {
		switch p.Type {
		case "output_text", "text":
			if p.Text != "" {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "text",
					Text: p.Text,
				})
			}
		}
	}

	if len(blocks) == 0 {
		blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: ""})
	}
	return json.Marshal(blocks)
}

// fromResponsesCallIDToAnthropic converts an OpenAI function call ID back to
// Anthropic format. Reverses toResponsesCallID.
func fromResponsesCallIDToAnthropic(id string) string {
	// If it has our "fc_" prefix wrapping a known Anthropic prefix, strip it
	if after, ok := strings.CutPrefix(id, "fc_"); ok {
		if strings.HasPrefix(after, "toolu_") || strings.HasPrefix(after, "call_") || strings.HasPrefix(after, "srvtoolu_") {
			return after
		}
	}
	// Generate a synthetic Anthropic tool ID
	if !strings.HasPrefix(id, "toolu_") && !strings.HasPrefix(id, "call_") && !strings.HasPrefix(id, "srvtoolu_") {
		return "toolu_" + id
	}
	return id
}

// dataURIToAnthropicImageSource parses a data URI into an AnthropicImageSource.
func dataURIToAnthropicImageSource(dataURI string) *AnthropicImageSource {
	if !strings.HasPrefix(dataURI, "data:") {
		return nil
	}
	// Format: data:<media_type>;base64,<data>
	rest := strings.TrimPrefix(dataURI, "data:")
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return nil
	}
	mediaType := rest[:semicolonIdx]
	rest = rest[semicolonIdx+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return nil
	}
	data := strings.TrimPrefix(rest, "base64,")
	return &AnthropicImageSource{
		Type:      "base64",
		MediaType: mediaType,
		Data:      data,
	}
}

// mergeConsecutiveMessages merges consecutive messages with the same role
// because Anthropic requires alternating user/assistant turns.
func mergeConsecutiveMessages(messages []AnthropicMessage) []AnthropicMessage {
	if len(messages) <= 1 {
		return messages
	}

	var merged []AnthropicMessage
	for _, msg := range messages {
		if len(merged) == 0 || merged[len(merged)-1].Role != msg.Role {
			merged = append(merged, msg)
			continue
		}

		// Same role — merge content arrays
		last := &merged[len(merged)-1]
		lastBlocks := parseContentBlocks(last.Content)
		newBlocks := parseContentBlocks(msg.Content)
		combined := append(lastBlocks, newBlocks...)
		last.Content, _ = json.Marshal(combined)
	}
	return merged
}

// parseContentBlocks attempts to parse content as []AnthropicContentBlock.
// If it's a string, wraps it in a text block.
func parseContentBlocks(raw json.RawMessage) []AnthropicContentBlock {
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []AnthropicContentBlock{{Type: "text", Text: s}}
	}
	return nil
}

// convertResponsesToAnthropicTools maps Responses API tools to Anthropic format.
// Reverse of convertAnthropicToolsToResponses.
func convertResponsesToAnthropicTools(tools []ResponsesTool) []AnthropicTool {
	var out []AnthropicTool
	for _, t := range tools {
		switch t.Type {
		case "web_search", "google_search", "web_search_20250305":
			out = append(out, AnthropicTool{
				Type: "web_search_20250305",
				Name: "web_search",
			})
		case "web_fetch", "web_fetch_20250910":
			out = append(out, AnthropicTool{
				Type: "web_fetch_20250910",
				Name: "web_fetch",
			})
		case "function":
			out = append(out, AnthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: normalizeAnthropicInputSchema(t.Parameters),
				Strict:      t.Strict,
			})
		default:
			// Pass through unknown tool types
			out = append(out, AnthropicTool{
				Type:        t.Type,
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Parameters,
			})
		}
	}
	return out
}

// normalizeAnthropicInputSchema ensures the input_schema has a "type" field.
func normalizeAnthropicInputSchema(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 || string(schema) == "null" {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return schema
}

// convertResponsesToAnthropicToolChoice maps Responses tool_choice to Anthropic format.
// Reverse of convertAnthropicToolChoiceToResponses.
//
//	"auto"                                     → {"type":"auto"}
//	"required"                                 → {"type":"any"}
//	"none"                                     → {"type":"none"}
//	{"type":"function","function":{"name":"X"}} → {"type":"tool","name":"X"}
//	{"type":"function","name":"X"}              → {"type":"tool","name":"X"}
func convertResponsesToAnthropicToolChoice(raw json.RawMessage, parallelToolCalls *bool) (json.RawMessage, error) {
	disableParallel := parallelToolCalls != nil && !*parallelToolCalls

	// Try as string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "auto":
			return marshalAnthropicToolChoice("auto", "", disableParallel)
		case "required":
			return marshalAnthropicToolChoice("any", "", disableParallel)
		case "none":
			return marshalAnthropicToolChoice("none", "", disableParallel)
		default:
			return raw, nil
		}
	}

	// Try as object with type=function
	var tc struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &tc); err == nil {
		choiceType := strings.ToLower(strings.TrimSpace(tc.Type))
		switch {
		case choiceType == "auto":
			return marshalAnthropicToolChoice("auto", "", disableParallel)
		case choiceType == "required":
			return marshalAnthropicToolChoice("any", "", disableParallel)
		case choiceType == "none":
			return marshalAnthropicToolChoice("none", "", disableParallel)
		case choiceType == "google_search" || strings.HasPrefix(choiceType, "web_search"):
			return marshalAnthropicToolChoice("tool", "web_search", disableParallel)
		case strings.HasPrefix(choiceType, "web_fetch"):
			return marshalAnthropicToolChoice("tool", "web_fetch", disableParallel)
		case choiceType == "function":
			name := strings.TrimSpace(tc.Name)
			if name == "" {
				name = strings.TrimSpace(tc.Function.Name)
			}
			if name != "" {
				return marshalAnthropicToolChoice("tool", name, disableParallel)
			}
		}
	}

	// Pass through unknown
	return raw, nil
}

func marshalAnthropicToolChoice(choiceType string, name string, disableParallel bool) (json.RawMessage, error) {
	out := map[string]any{"type": choiceType}
	if name != "" {
		out["name"] = name
	}
	if disableParallel {
		out["disable_parallel_tool_use"] = true
	}
	return json.Marshal(out)
}

func responsesInputFileToAnthropicDocument(part ResponsesContentPart) *AnthropicContentBlock {
	block := &AnthropicContentBlock{
		Type:  "document",
		Title: strings.TrimSpace(part.Filename),
	}

	switch {
	case strings.TrimSpace(part.FileData) != "":
		block.Source = &AnthropicImageSource{
			Type: "base64",
			Data: part.FileData,
		}
	case strings.TrimSpace(part.FileURL) != "":
		block.Source = &AnthropicImageSource{
			Type: "url",
			URL:  part.FileURL,
		}
	case strings.TrimSpace(part.FileID) != "":
		block.Source = &AnthropicImageSource{
			Type:   "file",
			FileID: part.FileID,
		}
	default:
		return nil
	}

	return block
}
