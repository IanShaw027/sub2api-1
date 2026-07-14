package apicompat

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// AnthropicToResponses converts using the request model as the target model.
func AnthropicToResponses(req *AnthropicRequest) (*ResponsesRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("anthropic request is required")
	}
	return AnthropicToResponsesForModel(req, req.Model)
}

// AnthropicToResponsesForModel converts an Anthropic Messages request directly
// into a Responses API request for targetModel. GPT-5.6 and later model
// families support explicit prompt cache breakpoints on input content blocks;
// unsupported cache_control locations remain observable as dropped fields.
func AnthropicToResponsesForModel(req *AnthropicRequest, targetModel string) (*ResponsesRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("anthropic request is required")
	}
	supportsBreakpoints := supportsResponsesPromptCacheBreakpoints(targetModel)
	input, cacheControlDropped, err := convertAnthropicToResponsesInput(req.System, req.Messages, supportsBreakpoints)
	if err != nil {
		return nil, err
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	out := &ResponsesRequest{
		Model:   targetModel,
		Input:   inputJSON,
		Stream:  req.Stream,
		Include: responsesIncludeForAnthropicTools(req.Tools),
	}
	if anthropicRequestHasCacheControl(req) && (!supportsBreakpoints || cacheControlDropped || anthropicToolsHaveCacheControl(req.Tools)) {
		out.DroppedCompatibilityFields = []string{"cache_control"}
	}

	if !isReasoningModel(targetModel) {
		out.Temperature = req.Temperature
		out.TopP = req.TopP
	}

	storeFalse := false
	out.Store = &storeFalse

	if req.MaxTokens > 0 {
		v := req.MaxTokens
		if v < minMaxOutputTokens {
			v = minMaxOutputTokens
		}
		out.MaxOutputTokens = &v
	}

	if len(req.Tools) > 0 {
		out.Tools = convertAnthropicToolsToResponses(req.Tools)
	}

	// Determine reasoning effort: only output_config.effort controls the
	// level; thinking.type is ignored. Default is high when unset (both
	// Anthropic and OpenAI default to high).
	// Anthropic levels map 1:1 to OpenAI: low→low, medium→medium, high→high, max→xhigh.
	effort := "high" // default → both sides' default
	if req.OutputConfig != nil && req.OutputConfig.Effort != "" {
		effort = req.OutputConfig.Effort
	}
	out.Reasoning = &ResponsesReasoning{
		Effort:  mapAnthropicEffortToResponses(effort),
		Summary: "auto",
	}

	// Convert tool_choice
	if len(req.ToolChoice) > 0 {
		tc, parallelToolCalls, err := convertAnthropicToolChoiceToResponses(req.ToolChoice)
		if err != nil {
			return nil, fmt.Errorf("convert tool_choice: %w", err)
		}
		out.ToolChoice = tc
		out.ParallelToolCalls = parallelToolCalls
	}

	return out, nil
}

func supportsResponsesPromptCacheBreakpoints(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.TrimPrefix(model, "openai/")
	if !strings.HasPrefix(model, "gpt-") {
		return false
	}
	version := strings.SplitN(strings.TrimPrefix(model, "gpt-"), "-", 2)[0]
	parts := strings.Split(version, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	if major > 5 {
		return true
	}
	if major < 5 || len(parts) < 2 {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	return err == nil && minor >= 6
}

func anthropicToolsHaveCacheControl(tools []AnthropicTool) bool {
	for _, tool := range tools {
		if tool.CacheControl != nil {
			return true
		}
	}
	return false
}

func anthropicRequestHasCacheControl(req *AnthropicRequest) bool {
	if req == nil {
		return false
	}
	if anthropicRawBlocksHaveCacheControl(req.System) {
		return true
	}
	for _, message := range req.Messages {
		if anthropicRawBlocksHaveCacheControl(message.Content) {
			return true
		}
	}
	for _, tool := range req.Tools {
		if tool.CacheControl != nil {
			return true
		}
	}
	return false
}

func anthropicRawBlocksHaveCacheControl(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return false
	}
	for _, block := range blocks {
		if block.CacheControl != nil || anthropicRawBlocksHaveCacheControl(block.Content) {
			return true
		}
	}
	return false
}

// convertAnthropicToolChoiceToResponses maps Anthropic tool_choice to Responses format.
//
//	{"type":"auto"}            → "auto"
//	{"type":"any"}             → "required"
//	{"type":"none"}            → "none"
//	{"type":"tool","name":"X"} → {"type":"function","name":"X"}
func responsesIncludeForAnthropicTools(tools []AnthropicTool) []string {
	include := []string{"reasoning.encrypted_content"}
	for _, tool := range tools {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(tool.Type)), "web_search") {
			include = append(include, "web_search_call.action.sources")
			break
		}
	}
	return include
}

func convertAnthropicToolChoiceToResponses(raw json.RawMessage) (json.RawMessage, *bool, error) {
	var tc struct {
		Type                   string `json:"type"`
		Name                   string `json:"name"`
		DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return nil, nil, err
	}

	var parallelToolCalls *bool
	if tc.DisableParallelToolUse {
		v := false
		parallelToolCalls = &v
	}

	switch tc.Type {
	case "auto":
		out, err := json.Marshal("auto")
		return out, parallelToolCalls, err
	case "any":
		out, err := json.Marshal("required")
		return out, parallelToolCalls, err
	case "none":
		out, err := json.Marshal("none")
		return out, parallelToolCalls, err
	case "tool":
		out, err := json.Marshal(map[string]any{
			"type": "function",
			"name": tc.Name,
		})
		return out, parallelToolCalls, err
	default:
		// Pass through unknown types as-is
		return raw, parallelToolCalls, nil
	}
}

// convertAnthropicToResponsesInput builds the Responses API input items array
// from the Anthropic system field and message list.
func convertAnthropicToResponsesInput(system json.RawMessage, msgs []AnthropicMessage, supportsBreakpoints bool) ([]ResponsesInputItem, bool, error) {
	var out []ResponsesInputItem
	cacheControlDropped := false

	// System prompt → system role input item.
	if len(system) > 0 {
		item, ok, dropped, err := anthropicSystemToResponsesItem(system, supportsBreakpoints)
		if err != nil {
			return nil, false, err
		}
		cacheControlDropped = cacheControlDropped || dropped
		if ok {
			out = append(out, item)
		}
	}

	for _, m := range msgs {
		items, dropped, err := anthropicMsgToResponsesItems(m, supportsBreakpoints)
		if err != nil {
			return nil, false, err
		}
		cacheControlDropped = cacheControlDropped || dropped
		out = append(out, items...)
	}
	return out, cacheControlDropped, nil
}

func anthropicSystemToResponsesItem(raw json.RawMessage, supportsBreakpoints bool) (ResponsesInputItem, bool, bool, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return ResponsesInputItem{}, false, false, nil
		}
		content, err := json.Marshal(s)
		return ResponsesInputItem{Role: "system", Content: content}, true, false, err
	}

	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ResponsesInputItem{}, false, false, err
	}
	if !supportsBreakpoints || !anthropicRawBlocksHaveCacheControl(raw) {
		text, err := parseAnthropicSystemPrompt(raw)
		if err != nil || text == "" {
			return ResponsesInputItem{}, false, anthropicRawBlocksHaveCacheControl(raw), err
		}
		content, err := json.Marshal(text)
		return ResponsesInputItem{Role: "system", Content: content}, true, anthropicRawBlocksHaveCacheControl(raw), err
	}

	parts := make([]ResponsesContentPart, 0, len(blocks))
	dropped := false
	for _, block := range blocks {
		if block.Type != "text" || block.Text == "" {
			dropped = dropped || block.CacheControl != nil
			continue
		}
		part := ResponsesContentPart{Type: "input_text", Text: block.Text}
		if block.CacheControl != nil {
			if canMapAnthropicCacheControlToResponses(block.CacheControl) {
				part.PromptCacheBreakpoint = explicitResponsesPromptCacheBreakpoint()
				dropped = dropped || strings.TrimSpace(block.CacheControl.TTL) != ""
			} else {
				dropped = true
			}
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ResponsesInputItem{}, false, dropped, nil
	}
	content, err := json.Marshal(parts)
	return ResponsesInputItem{Type: "message", Role: "system", Content: content}, true, dropped, err
}

func explicitResponsesPromptCacheBreakpoint() *ResponsesPromptCacheBreakpoint {
	return &ResponsesPromptCacheBreakpoint{Mode: "explicit"}
}

func canMapAnthropicCacheControlToResponses(cacheControl *AnthropicCacheControl) bool {
	return cacheControl != nil && strings.EqualFold(strings.TrimSpace(cacheControl.Type), "ephemeral")
}

// parseAnthropicSystemPrompt handles the Anthropic system field which can be
// a plain string or an array of text blocks.
func parseAnthropicSystemPrompt(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", err
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

// anthropicMsgToResponsesItems converts a single Anthropic message into one
// or more Responses API input items.
func anthropicMsgToResponsesItems(m AnthropicMessage, supportsBreakpoints bool) ([]ResponsesInputItem, bool, error) {
	switch m.Role {
	case "user":
		return anthropicUserToResponses(m.Content, supportsBreakpoints)
	case "assistant":
		items, err := anthropicAssistantToResponses(m.Content)
		return items, anthropicRawBlocksHaveCacheControl(m.Content), err
	default:
		return anthropicUserToResponses(m.Content, supportsBreakpoints)
	}
}

// anthropicUserToResponses handles an Anthropic user message. Content can be a
// plain string or an array of blocks. tool_result blocks are extracted into
// function_call_output items. Image blocks are converted to input_image parts.
func anthropicUserToResponses(raw json.RawMessage, supportsBreakpoints bool) ([]ResponsesInputItem, bool, error) {
	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		parts := []ResponsesContentPart{{Type: "input_text", Text: s}}
		partsJSON, err := json.Marshal(parts)
		if err != nil {
			return nil, false, err
		}
		return []ResponsesInputItem{{Type: "message", Role: "user", Content: partsJSON}}, false, nil
	}

	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, false, err
	}

	var out []ResponsesInputItem
	var toolResultImageParts []ResponsesContentPart
	cacheControlDropped := false

	// Extract tool_result blocks → function_call_output items.
	// Images inside tool_results are extracted separately because the
	// Responses API function_call_output.output only accepts strings.
	for _, b := range blocks {
		if b.Type != "tool_result" {
			continue
		}
		cacheControlDropped = cacheControlDropped || b.CacheControl != nil || anthropicRawBlocksHaveCacheControl(b.Content)
		outputText, imageParts := convertToolResultOutput(b)
		out = append(out, ResponsesInputItem{
			Type:   "function_call_output",
			CallID: toResponsesCallID(b.ToolUseID),
			Output: outputText,
		})
		toolResultImageParts = append(toolResultImageParts, imageParts...)
	}

	// Remaining text + image blocks → user message with content parts.
	// Also include images extracted from tool_results so the model can see them.
	var parts []ResponsesContentPart
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				part := ResponsesContentPart{Type: "input_text", Text: b.Text}
				if b.CacheControl != nil {
					if supportsBreakpoints && canMapAnthropicCacheControlToResponses(b.CacheControl) {
						part.PromptCacheBreakpoint = explicitResponsesPromptCacheBreakpoint()
						cacheControlDropped = cacheControlDropped || strings.TrimSpace(b.CacheControl.TTL) != ""
					} else {
						cacheControlDropped = true
					}
				}
				parts = append(parts, part)
			}
		case "document":
			if part := anthropicDocumentToResponsesPart(b); part != nil {
				if b.CacheControl != nil {
					if supportsBreakpoints && canMapAnthropicCacheControlToResponses(b.CacheControl) {
						part.PromptCacheBreakpoint = explicitResponsesPromptCacheBreakpoint()
						cacheControlDropped = cacheControlDropped || strings.TrimSpace(b.CacheControl.TTL) != ""
					} else {
						cacheControlDropped = true
					}
				}
				parts = append(parts, *part)
			}
		case "image":
			if uri := anthropicImageToDataURI(b.Source); uri != "" {
				part := ResponsesContentPart{Type: "input_image", ImageURL: uri}
				if b.CacheControl != nil {
					if supportsBreakpoints && canMapAnthropicCacheControlToResponses(b.CacheControl) {
						part.PromptCacheBreakpoint = explicitResponsesPromptCacheBreakpoint()
						cacheControlDropped = cacheControlDropped || strings.TrimSpace(b.CacheControl.TTL) != ""
					} else {
						cacheControlDropped = true
					}
				}
				parts = append(parts, part)
			}
		default:
			cacheControlDropped = cacheControlDropped || b.CacheControl != nil
		}
	}
	parts = append(parts, toolResultImageParts...)

	if len(parts) > 0 {
		content, err := json.Marshal(parts)
		if err != nil {
			return nil, false, err
		}
		out = append(out, ResponsesInputItem{Type: "message", Role: "user", Content: content})
	}

	return out, cacheControlDropped, nil
}

// GPT-5 family models are reasoning-only on the Responses API and reject
// sampling parameters like temperature/top_p when they are explicitly sent.
func isReasoningModel(model string) bool {
	return strings.HasPrefix(model, "gpt-5")
}

// anthropicAssistantToResponses handles an Anthropic assistant message.
// Text content → assistant message with output_text parts.
// tool_use blocks → function_call items.
// thinking blocks → ignored (OpenAI doesn't accept them as input).
func anthropicAssistantToResponses(raw json.RawMessage) ([]ResponsesInputItem, error) {
	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		parts := []ResponsesContentPart{{Type: "output_text", Text: s}}
		partsJSON, err := json.Marshal(parts)
		if err != nil {
			return nil, err
		}
		return []ResponsesInputItem{{Type: "message", Role: "assistant", Content: partsJSON}}, nil
	}

	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}

	var items []ResponsesInputItem

	// Text content → assistant message with output_text content parts.
	text := extractAnthropicTextFromBlocks(blocks)
	if text != "" {
		parts := []ResponsesContentPart{{Type: "output_text", Text: text}}
		partsJSON, err := json.Marshal(parts)
		if err != nil {
			return nil, err
		}
		items = append(items, ResponsesInputItem{Type: "message", Role: "assistant", Content: partsJSON})
	}

	// tool_use → function_call items.
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		args := anthropicToolUseArguments(b.Input)
		fcID := toResponsesCallID(b.ID)
		items = append(items, ResponsesInputItem{
			Type:      "function_call",
			CallID:    fcID,
			Name:      b.Name,
			Arguments: args,
		})
	}

	return items, nil
}

// toResponsesCallID preserves Anthropic tool IDs as Responses call_id values,
// while accepting transcripts produced by the old fc_ prefixing bridge.
// Claude Code sends tool_result.tool_use_id back verbatim, and the Responses API
// continuation expects that call_id to match the original tool_use id.
func toResponsesCallID(id string) string {
	return fromResponsesCallID(id)
}

// fromResponsesCallID strips the legacy "fc_" prefix that was added during
// request conversion before upstream switched to preserving Anthropic tool IDs.
func fromResponsesCallID(id string) string {
	if after, ok := strings.CutPrefix(id, "fc_"); ok {
		// Only strip if the remainder doesn't look like it was already "fc_" prefixed.
		// E.g. "fc_toolu_xxx" → "toolu_xxx", "fc_call_xxx" → "call_xxx"
		if strings.HasPrefix(after, "toolu_") || strings.HasPrefix(after, "call_") {
			return after
		}
	}
	return id
}

// anthropicImageToDataURI converts an AnthropicImageSource to a data URI string.
// Returns "" if the source is nil or has no data.
func anthropicImageToDataURI(src *AnthropicImageSource) string {
	if src == nil || src.Data == "" {
		return ""
	}
	mediaType := src.MediaType
	if mediaType == "" {
		mediaType = "image/png"
	}
	return "data:" + mediaType + ";base64," + src.Data
}

func anthropicDocumentToResponsesPart(b AnthropicContentBlock) *ResponsesContentPart {
	if b.Source == nil {
		return nil
	}

	part := &ResponsesContentPart{
		Type:     "input_file",
		Filename: strings.TrimSpace(b.Title),
	}

	switch strings.ToLower(strings.TrimSpace(b.Source.Type)) {
	case "base64":
		if b.Source.Data == "" {
			return nil
		}
		part.FileData = b.Source.Data
	case "url":
		if b.Source.URL == "" {
			return nil
		}
		part.FileURL = b.Source.URL
	case "file":
		if b.Source.FileID == "" {
			return nil
		}
		part.FileID = b.Source.FileID
	default:
		return nil
	}

	return part
}

// convertToolResultOutput extracts text and image content from a tool_result
// block. Returns the text as a string for the function_call_output Output
// field, plus any image parts that must be sent in a separate user message
// (the Responses API output field only accepts strings).
func convertToolResultOutput(b AnthropicContentBlock) (string, []ResponsesContentPart) {
	if len(b.Content) == 0 {
		if b.IsError {
			payload, _ := json.Marshal(anthropicToolResultEnvelope{
				Format:  anthropicToolResultEnvelopeFormat,
				Text:    []string{"(empty)"},
				IsError: true,
			})
			return string(payload), nil
		}
		return "(empty)", nil
	}

	// Try plain string content.
	var s string
	if err := json.Unmarshal(b.Content, &s); err == nil {
		if s == "" {
			s = "(empty)"
		}
		if b.IsError {
			payload, _ := json.Marshal(anthropicToolResultEnvelope{
				Format:  anthropicToolResultEnvelopeFormat,
				Text:    []string{s},
				IsError: true,
			})
			return string(payload), nil
		}
		return s, nil
	}

	// Array of content blocks — may contain text and/or images.
	var inner []AnthropicContentBlock
	if err := json.Unmarshal(b.Content, &inner); err != nil {
		if b.IsError {
			payload, _ := json.Marshal(anthropicToolResultEnvelope{
				Format:  anthropicToolResultEnvelopeFormat,
				Text:    []string{"(empty)"},
				IsError: true,
			})
			return string(payload), nil
		}
		return "(empty)", nil
	}

	// Separate text (for function_call_output) from images (for user message).
	var textParts []string
	var imageParts []ResponsesContentPart
	var toolReferences []anthropicToolReferenceEnvelope
	for _, ib := range inner {
		switch ib.Type {
		case "text":
			if ib.Text != "" {
				textParts = append(textParts, ib.Text)
			}
		case "image":
			if uri := anthropicImageToDataURI(ib.Source); uri != "" {
				imageParts = append(imageParts, ResponsesContentPart{Type: "input_image", ImageURL: uri})
			}
		case "tool_reference":
			if ib.ToolName != "" {
				toolReferences = append(toolReferences, anthropicToolReferenceEnvelope{ToolName: ib.ToolName})
			}
		}
	}

	if b.IsError || len(toolReferences) > 0 {
		payload, err := json.Marshal(anthropicToolResultEnvelope{
			Format:         anthropicToolResultEnvelopeFormat,
			Text:           textParts,
			ToolReferences: toolReferences,
			IsError:        b.IsError,
		})
		if err == nil {
			return string(payload), imageParts
		}
	}

	text := strings.Join(textParts, "\n\n")
	if text == "" {
		text = "(empty)"
	}
	return text, imageParts
}

// extractAnthropicTextFromBlocks joins all text blocks, ignoring thinking/
// tool_use/tool_result blocks.
func extractAnthropicTextFromBlocks(blocks []AnthropicContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// mapAnthropicEffortToResponses converts Anthropic reasoning effort levels to
// OpenAI Responses API effort levels.
//
// Both APIs default to "high". The mapping is 1:1 for shared levels;
// only Anthropic's "max" (Opus 4.6 exclusive) maps to OpenAI's "xhigh"
// (GPT-5.2+ exclusive) as both represent the highest reasoning tier.
//
//	low    → low
//	medium → medium
//	high   → high
//	max    → xhigh
func mapAnthropicEffortToResponses(effort string) string {
	if effort == "max" {
		return "xhigh"
	}
	return effort // low→low, medium→medium, high→high, unknown→passthrough
}

// convertAnthropicToolsToResponses maps Anthropic tool definitions to
// Responses API tools. Server-side tools like web_search are mapped to their
// OpenAI equivalents; regular tools become function tools.
func convertAnthropicToolsToResponses(tools []AnthropicTool) []ResponsesTool {
	var out []ResponsesTool
	for _, t := range tools {
		if isAnthropicDeferredToolSearch(t) {
			continue
		}
		// Anthropic server tools like "web_search_20250305" → OpenAI {"type":"web_search"}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(t.Type)), "web_search") {
			out = append(out, ResponsesTool{Type: "web_search"})
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(t.Type)), "web_fetch") {
			out = append(out, ResponsesTool{Type: "web_fetch"})
			continue
		}
		out = append(out, ResponsesTool{
			Type:        "function",
			Name:        t.Name,
			Description: strings.TrimSpace(t.Description),
			Parameters:  normalizeToolParameters(t.InputSchema),
			Strict:      t.Strict,
		})
	}
	return out
}

func isAnthropicDeferredToolSearch(tool AnthropicTool) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(tool.Type)), "tool_search")
}

func AugmentClaudeToolDescriptions(tools []ResponsesTool) {
	for i := range tools {
		if tools[i].Type != "function" || tools[i].Name == "" {
			continue
		}
		tools[i].Description = augmentClaudeToolDescription(tools[i].Name, tools[i].Description)
	}
}

func augmentClaudeToolDescription(name, description string) string {
	guidance := claudeToolRoutingGuidance(name)
	description = strings.TrimSpace(description)
	if guidance == "" {
		return description
	}
	if description == "" {
		return guidance
	}
	if strings.Contains(description, guidance) {
		return description
	}
	return guidance + "\n\n" + description
}

func claudeToolRoutingGuidance(name string) string {
	switch canonicalClaudeToolName(name) {
	case "read":
		return "Use this when you need the exact contents of a known file in the local workspace."
	case "grep":
		return "Use this when you need to search the local workspace for specific text, symbols, or patterns."
	case "glob":
		return "Use this when you need to discover files by path pattern, extension, or directory structure."
	case "edit":
		return "Use this when you need to modify an existing local file in place."
	case "write":
		return "Use this when you need to create a file or replace the full contents of a local file."
	case "bash":
		return "Use this when you need shell commands, build steps, tests, or repository state that Read, Grep, and Glob cannot provide."
	case "websearch":
		return "Use this when you need current external information across multiple sources."
	case "webfetch":
		return "Use this when a specific URL is already known and the exact page contents matter."
	case "toolsearch":
		return "Use this when you need to discover which local or MCP-backed tool should be called next."
	case "askuserquestion":
		return "Use this when a missing human decision would materially change the work."
	case "teamcreate":
		return "Use this when the task can be split into independent sub-agents that should work in parallel."
	case "sendmessage":
		return "Use this when you need to coordinate with an existing teammate or return structured status to the team lead."
	}
	return ""
}

// normalizeToolParameters ensures the tool parameter schema is valid for
// OpenAI's Responses API, which requires "properties" on object schemas.
//
//   - nil/empty → {"type":"object","properties":{}}
//   - type=object without properties → adds "properties": {}
//   - otherwise → returned unchanged
func normalizeToolParameters(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 || string(schema) == "null" {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(schema, &m); err != nil {
		return schema
	}

	typ := m["type"]
	if string(typ) != `"object"` {
		return schema
	}

	if _, ok := m["properties"]; ok {
		return schema
	}

	m["properties"] = json.RawMessage(`{}`)
	out, err := json.Marshal(m)
	if err != nil {
		return schema
	}
	return out
}
