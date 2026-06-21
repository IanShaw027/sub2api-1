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
)

// StripToolTurnPlaceholders removes the synthetic tool-turn placeholders the
// gateway injects into upstream requests (Kiro rejects empty message content,
// so tool-only turns are padded with these strings). The upstream sometimes
// echoes a padded placeholder back verbatim as assistant output; left
// unfiltered it leaks into the client stream as a spurious assistant line like
// "I will call the requested tools." This trims a response whose entire visible
// text is one of those placeholders.
func StripToolTurnPlaceholders(assistantText string) string {
	switch strings.TrimSpace(assistantText) {
	case toolOnlyAssistantPlaceholder, toolOnlyUserPlaceholder:
		return ""
	}
	return assistantText
}

// ToolTurnPlaceholderMatch classifies trimmed assistant text against the
// injected tool-turn placeholders for streaming suppression:
//   - exact: the text equals a placeholder in full and must be dropped
//   - prefix: the text is a leading fragment of a placeholder; the caller should
//     keep buffering before deciding
//
// Empty input is treated as a prefix (still undecided).
func ToolTurnPlaceholderMatch(text string) (exact, prefix bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false, true
	}
	for _, ph := range []string{toolOnlyAssistantPlaceholder, toolOnlyUserPlaceholder} {
		if trimmed == ph {
			return true, false
		}
		if strings.HasPrefix(ph, trimmed) {
			prefix = true
		}
	}
	return false, prefix
}

// IsPlaceholderFragment reports whether text (after trimming) is a trailing
// word-boundary substring of a known placeholder. This catches partial echoes
// like "call" (from "I will call the requested tools.") that appear in the
// stream right before a tool_use event but are too short to match the prefix
// check. Only fragments of 3+ characters are considered to avoid false
// positives on common short words.
func IsPlaceholderFragment(text string) bool {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) < 3 {
		return false
	}
	for _, ph := range []string{toolOnlyAssistantPlaceholder, toolOnlyUserPlaceholder} {
		idx := strings.Index(ph, trimmed)
		if idx < 0 {
			continue
		}
		atStart := idx == 0
		atWordBoundary := idx > 0 && (ph[idx-1] == ' ' || ph[idx-1] == '.')
		endIdx := idx + len(trimmed)
		atEnd := endIdx == len(ph)
		atEndWordBoundary := endIdx < len(ph) && (ph[endIdx] == ' ' || ph[endIdx] == '.')
		if (atStart || atWordBoundary) && (atEnd || atEndWordBoundary) {
			return true
		}
	}
	return false
}

// StripTrailingPlaceholderFragment removes a trailing line from assistant text
// if it is a placeholder fragment. Used in non-streaming path when stop_reason
// is tool_use.
func StripTrailingPlaceholderFragment(assistantText string) string {
	trimmed := strings.TrimRight(assistantText, "\n\r ")
	lastNL := strings.LastIndex(trimmed, "\n")
	var tail string
	if lastNL >= 0 {
		tail = trimmed[lastNL+1:]
	} else {
		tail = trimmed
	}
	if IsPlaceholderFragment(tail) {
		if lastNL >= 0 {
			return strings.TrimRight(trimmed[:lastNL], "\n\r ")
		}
		return ""
	}
	return assistantText
}

const kiroCompactionRecentWindow = 4

const (
	ShadowToolPrefix    = "cc_srv_"
	ShadowToolWebSearch = ShadowToolPrefix + "web_search"
	ShadowToolWebFetch  = ShadowToolPrefix + "web_fetch"
)

var kiroCompactionFilePattern = regexp.MustCompile("(?i)(?:^|[\\s\\\"'(<])((?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+\\.[A-Za-z0-9_.-]+)")

type ConvertResult struct {
	Body           []byte
	Model          string
	RequestedModel string
	ToolNameMap    map[string]string
	BridgeMetadata *BridgeMetadata
	ToolMetadata   *ToolMetadata
}

type BridgeMetadata struct {
	ShadowTools map[string]ShadowToolBridge
}

type ToolMetadata struct {
	ResponseTools map[string]ResponseToolBridge
}

type ResponseToolBridge struct {
	AnthropicType string
	AnthropicName string
	Family        string
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
	tools, toolNameMap, bridgeMetadata, toolMetadata, err := convertTools(req["tools"])
	if err != nil {
		return nil, err
	}
	tools, bridgeMetadata = ensureHistoryShadowTools(rawMessages[:len(rawMessages)-1], tools, bridgeMetadata)
	tools, toolMetadata = ensureHistoryNativeServerTools(rawMessages[:len(rawMessages)-1], tools, toolMetadata)
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
		ToolMetadata:   toolMetadata,
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

// Token estimate calibration constants.
//
// Kiro upstream (Bedrock frame protocol) does not return token usage, so the
// gateway estimates input tokens locally. A raw tiktoken (cl100k_base) count of
// the serialized content badly undercounts what Anthropic actually bills,
// because it ignores per-message structural overhead and—most importantly—the
// large tool-use system scaffold Anthropic injects whenever any tool is present.
//
// These coefficients were calibrated against real Anthropic usage by sending
// controlled request bodies through the official API and regressing the real
// total input tokens against the local component estimates. Fit error stayed
// within ~5% across content-only, multi-turn, and tool-heavy requests.
const (
	kiroTokenEstimateBaseOverhead    = 13  // fixed per-request scaffold
	kiroTokenEstimateContentScale100 = 110 // content multiplier ×100 (1.10)
	kiroTokenEstimatePerMessage      = 4   // per-message structural overhead
	kiroTokenEstimateToolSystemBase  = 493 // tool-use system prompt injected when any tool is present
	kiroTokenEstimatePerTool         = 5   // per-tool fixed overhead (rounded from 4.5)
	kiroTokenEstimateToolSchemaScale = 175 // tool schema multiplier ×100 (1.75)
	// Per tool_use / tool_result content block in the conversation history.
	// Calibrated from multi-turn tool conversations: each tool round-trip pair
	// bills ~38 tokens beyond its text content and per-message overhead, split
	// across the two structural blocks (id/type/tool_use_id wrapping).
	kiroTokenEstimatePerToolBlock = 19
)

func EstimateInputTokens(body []byte) int {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return 0
	}

	var contentBuilder strings.Builder
	if systemText := joinSystem(req["system"]); systemText != "" {
		appendKiroTokenEstimateText(&contentBuilder, systemText)
	}
	messageCount := 0
	toolBlockCount := 0
	if messages, _ := req["messages"].([]any); len(messages) > 0 {
		messageCount = len(messages)
		for _, item := range messages {
			msg, _ := item.(map[string]any)
			appendKiroTokenEstimateContent(&contentBuilder, msg["content"])
			toolBlockCount += countKiroToolBlocks(msg["content"])
		}
	}
	contentTokens := AccurateTokenCount(contentBuilder.String())

	toolCount := 0
	toolSchemaTokens := 0
	if tools, _ := req["tools"].([]any); len(tools) > 0 {
		toolCount = len(tools)
		var toolBuilder strings.Builder
		for _, item := range tools {
			tool, _ := item.(map[string]any)
			appendKiroTokenEstimateText(&toolBuilder, stringField(tool, "name"))
			appendKiroTokenEstimateText(&toolBuilder, stringField(tool, "description"))
			appendKiroTokenEstimateJSON(&toolBuilder, tool["input_schema"])
		}
		toolSchemaTokens = AccurateTokenCount(toolBuilder.String())
	}

	return calibrateKiroInputTokens(contentTokens, messageCount, toolCount, toolSchemaTokens, toolBlockCount)
}

// countKiroToolBlocks counts tool_use and tool_result blocks in a message's
// content so their structural billing overhead can be added during calibration.
func countKiroToolBlocks(content any) int {
	blocks, ok := content.([]any)
	if !ok {
		return 0
	}
	n := 0
	for _, item := range blocks {
		block, _ := item.(map[string]any)
		switch strings.TrimSpace(stringField(block, "type")) {
		case "tool_use", "tool_result":
			n++
		}
	}
	return n
}

// calibrateKiroInputTokens maps raw local component estimates onto the
// Anthropic-billed token scale using the constants documented above.
func calibrateKiroInputTokens(contentTokens, messageCount, toolCount, toolSchemaTokens, toolBlockCount int) int {
	if contentTokens == 0 && messageCount == 0 && toolCount == 0 {
		return 0
	}
	total := kiroTokenEstimateBaseOverhead
	total += contentTokens * kiroTokenEstimateContentScale100 / 100
	total += messageCount * kiroTokenEstimatePerMessage
	total += toolBlockCount * kiroTokenEstimatePerToolBlock
	if toolCount > 0 {
		total += kiroTokenEstimateToolSystemBase
		total += toolCount * kiroTokenEstimatePerTool
		total += toolSchemaTokens * kiroTokenEstimateToolSchemaScale / 100
	}
	if total < 0 {
		return 0
	}
	return total
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
	// Output is generated text passing through the same tiktoken estimator that
	// undercounts Anthropic's tokenizer. Apply the same content scaling factor
	// calibrated for input content (see kiroTokenEstimateContentScale100).
	return AccurateTokenCount(text) * kiroTokenEstimateContentScale100 / 100
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
			case "tool_result", "web_search_tool_result", "web_fetch_tool_result":
				toolResults = append(toolResults, kiroHistoryToolResultFromAnthropicBlock(block))
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
				name, input := KiroHistoryToolUseFromAnthropicBlock(block)
				name = shortenToolName(name)
				rememberToolName(toolNameMap, name, stringField(block, "name"))
				toolUses = append(toolUses, map[string]any{
					"toolUseId": stringField(block, "id"),
					"name":      name,
					"input":     input,
				})
			case "server_tool_use":
				name, input, ok := KiroHistoryServerToolUseFromAnthropicBlock(block)
				if !ok {
					continue
				}
				name = shortenToolName(name)
				toolUses = append(toolUses, map[string]any{
					"toolUseId": stringField(block, "id"),
					"name":      name,
					"input":     input,
				})
			case "thinking", "redacted_thinking":
				continue
			}
		}
	}

	return strings.Join(textParts, "\n"), toolUses
}

func kiroHistoryToolResultFromAnthropicBlock(block map[string]any) map[string]any {
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
	return toolResult
}

func KiroHistoryToolUseFromAnthropicBlock(block map[string]any) (string, any) {
	name := strings.TrimSpace(stringField(block, "name"))
	input := jsonValue(block["input"])
	switch name {
	case "bash":
		return "Bash", input
	case "str_replace_based_edit_tool":
		mappedName, mappedInput := KiroTextEditorToolFromAnthropicInput(input)
		return mappedName, mappedInput
	default:
		return name, input
	}
}

func KiroHistoryServerToolUseFromAnthropicBlock(block map[string]any) (string, any, bool) {
	name := strings.TrimSpace(stringField(block, "name"))
	input := jsonValue(block["input"])
	switch {
	case strings.EqualFold(name, "google_search"):
		return "web_search", input, true
	case strings.HasPrefix(strings.ToLower(name), "web_search"):
		return "web_search", input, true
	case strings.HasPrefix(strings.ToLower(name), "web_fetch"):
		return "web_fetch", input, true
	default:
		return "", nil, false
	}
}

func KiroTextEditorToolFromAnthropicInput(input any) (string, any) {
	obj, _ := input.(map[string]any)
	if obj == nil {
		return "Edit", input
	}
	command := strings.TrimSpace(stringField(obj, "command"))
	switch command {
	case "view":
		return "Read", map[string]any{
			"file_path": firstNonEmptyKiroString(obj, "path", "file_path"),
		}
	case "create":
		return "Write", map[string]any{
			"file_path": firstNonEmptyKiroString(obj, "path", "file_path"),
			"content":   firstNonEmptyKiroString(obj, "file_text", "content"),
		}
	case "str_replace":
		return "Edit", map[string]any{
			"file_path":  firstNonEmptyKiroString(obj, "path", "file_path"),
			"old_string": firstNonEmptyKiroString(obj, "old_str", "old_string"),
			"new_string": firstNonEmptyKiroString(obj, "new_str", "new_string"),
		}
	default:
		return "Edit", input
	}
}

func AnthropicToolUseFromKiroToolUse(name string, input any, metadata *ToolMetadata) (string, any) {
	name = strings.TrimSpace(name)
	if metadata == nil || metadata.ResponseTools == nil {
		return name, input
	}
	bridge, ok := metadata.ResponseTools[name]
	if !ok || strings.TrimSpace(bridge.AnthropicName) == "" {
		return name, input
	}
	switch bridge.Family {
	case "anthropic_text_editor":
		return bridge.AnthropicName, AnthropicTextEditorInputFromKiroToolUse(name, input)
	default:
		return bridge.AnthropicName, input
	}
}

func AnthropicServerToolUseFromKiroToolUse(name string, input any, metadata *ToolMetadata) (map[string]any, bool) {
	name = strings.TrimSpace(name)
	if metadata == nil || metadata.ResponseTools == nil {
		return nil, false
	}
	bridge, ok := metadata.ResponseTools[name]
	if !ok || !isAnthropicServerToolFamily(bridge.Family) {
		return nil, false
	}
	return map[string]any{
		"type":  "server_tool_use",
		"name":  firstNonEmptyKiroString(map[string]any{"name": bridge.AnthropicName}, "name"),
		"input": input,
	}, true
}

func AnthropicTextEditorInputFromKiroToolUse(name string, input any) any {
	obj, _ := input.(map[string]any)
	if obj == nil {
		return input
	}
	switch strings.TrimSpace(name) {
	case "Read":
		return map[string]any{
			"command": "view",
			"path":    firstNonEmptyKiroString(obj, "file_path", "path"),
		}
	case "Write":
		return map[string]any{
			"command":   "create",
			"path":      firstNonEmptyKiroString(obj, "file_path", "path"),
			"file_text": firstNonEmptyKiroString(obj, "content", "file_text"),
		}
	case "Edit":
		return map[string]any{
			"command": "str_replace",
			"path":    firstNonEmptyKiroString(obj, "file_path", "path"),
			"old_str": firstNonEmptyKiroString(obj, "old_string", "old_str"),
			"new_str": firstNonEmptyKiroString(obj, "new_string", "new_str"),
		}
	default:
		return input
	}
}

func firstNonEmptyKiroString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(stringField(obj, key)); value != "" {
			return value
		}
	}
	return ""
}

func convertTools(raw any) ([]map[string]any, map[string]string, *BridgeMetadata, *ToolMetadata, error) {
	items, _ := raw.([]any)
	tools := make([]map[string]any, 0, len(items))
	toolNameMap := make(map[string]string)
	bridgeMetadata := &BridgeMetadata{ShadowTools: map[string]ShadowToolBridge{}}
	toolMetadata := &ToolMetadata{ResponseTools: map[string]ResponseToolBridge{}}
	for _, item := range items {
		tool, _ := item.(map[string]any)
		if unsupportedFamily := unsupportedServerToolFamily(tool); unsupportedFamily != "" {
			return nil, nil, nil, nil, fmt.Errorf("unsupported server-side tool family for Kiro bridge: %s", unsupportedFamily)
		}
		if isUnsupportedServerTool(tool) {
			continue
		}
		if nativeTool, bridge, ok := supportedNativeServerTool(tool); ok {
			tools = append(tools, nativeTool)
			if _, exists := toolMetadata.ResponseTools[bridge.AnthropicName]; !exists {
				toolMetadata.ResponseTools[bridge.AnthropicName] = bridge
			}
			continue
		}
		if officialTools, bridges, ok := supportedOfficialClientTools(tool); ok {
			tools = append(tools, officialTools...)
			for name, bridge := range bridges {
				if _, exists := toolMetadata.ResponseTools[name]; !exists {
					toolMetadata.ResponseTools[name] = bridge
				}
			}
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
		description := reinforceControlPlaneToolDescription(name, stringField(tool, "description"))
		schema := normalizeJSONSchema(jsonValue(tool["input_schema"]))
		schema = reinforceControlPlaneToolSchema(name, schema)
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
	if len(toolMetadata.ResponseTools) == 0 {
		toolMetadata = nil
	}
	return tools, toolNameMap, bridgeMetadata, toolMetadata, nil
}

func reinforceControlPlaneToolDescription(name, description string) string {
	if !strings.EqualFold(strings.TrimSpace(name), "AskUserQuestion") {
		return description
	}
	if strings.Contains(description, "questions[].question") {
		return description
	}
	suffix := "Payload contract: pass questions as an array, and every entry must include questions[].question with the exact user-facing prompt text."
	if strings.TrimSpace(description) == "" {
		return suffix
	}
	return strings.TrimSpace(description) + "\n\n" + suffix
}

func reinforceControlPlaneToolSchema(name string, schema map[string]any) map[string]any {
	if !strings.EqualFold(strings.TrimSpace(name), "AskUserQuestion") {
		return schema
	}
	if schema == nil {
		schema = normalizeJSONSchema(nil)
	}
	appendRequiredString(schema, "questions")

	properties := ensureJSONObjectField(schema, "properties")
	questions := ensureJSONObjectField(properties, "questions")
	if strings.TrimSpace(stringField(questions, "type")) == "" {
		questions["type"] = "array"
	}
	items := ensureJSONObjectField(questions, "items")
	if strings.TrimSpace(stringField(items, "type")) == "" {
		items["type"] = "object"
	}
	itemProperties := ensureJSONObjectField(items, "properties")
	question := ensureJSONObjectField(itemProperties, "question")
	if strings.TrimSpace(stringField(question, "type")) == "" {
		question["type"] = "string"
	}
	if strings.TrimSpace(stringField(question, "description")) == "" {
		question["description"] = "The exact question text to show to the user."
	}
	appendRequiredString(items, "question")
	if _, ok := items["additionalProperties"].(bool); !ok {
		if _, ok := items["additionalProperties"].(map[string]any); !ok {
			items["additionalProperties"] = false
		}
	}

	return schema
}

func ensureJSONObjectField(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return map[string]any{}
	}
	if value, ok := parent[key].(map[string]any); ok && value != nil {
		return value
	}
	value := map[string]any{}
	parent[key] = value
	return value
}

func appendRequiredString(schema map[string]any, field string) {
	if schema == nil || strings.TrimSpace(field) == "" {
		return
	}
	switch required := schema["required"].(type) {
	case []string:
		for _, item := range required {
			if item == field {
				return
			}
		}
		schema["required"] = append(required, field)
	case []any:
		for _, item := range required {
			if text, ok := item.(string); ok && text == field {
				return
			}
		}
		schema["required"] = append(required, field)
	default:
		schema["required"] = []string{field}
	}
}

func isUnsupportedServerTool(tool map[string]any) bool {
	toolType := strings.TrimSpace(stringField(tool, "type"))
	return toolType == "server_tool"
}

func unsupportedServerToolFamily(tool map[string]any) string {
	toolType := strings.ToLower(strings.TrimSpace(stringField(tool, "type")))
	switch {
	case toolType == "":
		return ""
	case toolType == "server_tool":
		return toolType
	case toolType == "google_search":
		return ""
	case strings.HasPrefix(toolType, "web_search"):
		return ""
	case strings.HasPrefix(toolType, "web_fetch"):
		return ""
	case strings.HasPrefix(toolType, "tool_search_"):
		return ""
	case strings.HasPrefix(toolType, "computer_"):
		return toolType
	case strings.HasPrefix(toolType, "bash_"):
		return ""
	case strings.HasPrefix(toolType, "text_editor_"):
		return ""
	default:
		return toolType
	}
}

func supportedOfficialClientTools(tool map[string]any) ([]map[string]any, map[string]ResponseToolBridge, bool) {
	toolType := strings.ToLower(strings.TrimSpace(stringField(tool, "type")))
	switch {
	case strings.HasPrefix(toolType, "bash_"):
		return []map[string]any{makeOfficialBashToolSpecification()}, map[string]ResponseToolBridge{
			"Bash": {AnthropicType: toolType, AnthropicName: "bash", Family: "anthropic_bash"},
		}, true
	case strings.HasPrefix(toolType, "text_editor_"):
		bridge := ResponseToolBridge{AnthropicType: toolType, AnthropicName: "str_replace_based_edit_tool", Family: "anthropic_text_editor"}
		return []map[string]any{
				makeOfficialTextEditorReadToolSpecification(),
				makeOfficialTextEditorWriteToolSpecification(),
				makeOfficialTextEditorEditToolSpecification(),
			}, map[string]ResponseToolBridge{
				"Read":  bridge,
				"Write": bridge,
				"Edit":  bridge,
			}, true
	default:
		return nil, nil, false
	}
}

func supportedNativeServerTool(tool map[string]any) (map[string]any, ResponseToolBridge, bool) {
	toolType := strings.ToLower(strings.TrimSpace(stringField(tool, "type")))
	switch {
	case toolType == "google_search", strings.HasPrefix(toolType, "web_search"):
		return makeNativeServerToolSpecification("web_search", "Search the web using Kiro's native web_search tool.", "query"), ResponseToolBridge{
			AnthropicType: toolType,
			AnthropicName: "web_search",
			Family:        "anthropic_web_search",
		}, true
	case strings.HasPrefix(toolType, "web_fetch"):
		return makeNativeServerToolSpecification("web_fetch", "Fetch a URL using Kiro's native web_fetch tool.", "url"), ResponseToolBridge{
			AnthropicType: toolType,
			AnthropicName: "web_fetch",
			Family:        "anthropic_web_fetch",
		}, true
	default:
		return nil, ResponseToolBridge{}, false
	}
}

func isAnthropicServerToolFamily(family string) bool {
	switch strings.TrimSpace(family) {
	case "anthropic_web_search", "anthropic_web_fetch":
		return true
	default:
		return false
	}
}

func makeNativeServerToolSpecification(name, description, requiredField string) map[string]any {
	return map[string]any{
		"toolSpecification": map[string]any{
			"name":        name,
			"description": description,
			"inputSchema": map[string]any{
				"json": map[string]any{
					"type": "object",
					"properties": map[string]any{
						requiredField: map[string]any{"type": "string"},
					},
					"required":             []string{requiredField},
					"additionalProperties": false,
				},
			},
		},
	}
}

func makeOfficialBashToolSpecification() map[string]any {
	return map[string]any{
		"toolSpecification": map[string]any{
			"name":        "Bash",
			"description": "Execute shell commands for the user. Use this only for the Anthropic bash tool family.",
			"inputSchema": map[string]any{
				"json": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command":     map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
						"timeout":     map[string]any{"type": "integer"},
					},
					"required":             []string{"command"},
					"additionalProperties": false,
				},
			},
		},
	}
}

func makeOfficialTextEditorReadToolSpecification() map[string]any {
	return map[string]any{
		"toolSpecification": map[string]any{
			"name":        "Read",
			"description": "Read a file for the Anthropic text editor tool family.",
			"inputSchema": map[string]any{
				"json": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{"type": "string"},
						"offset":    map[string]any{"type": "integer"},
						"limit":     map[string]any{"type": "integer"},
					},
					"required":             []string{"file_path"},
					"additionalProperties": false,
				},
			},
		},
	}
}

func makeOfficialTextEditorWriteToolSpecification() map[string]any {
	return map[string]any{
		"toolSpecification": map[string]any{
			"name":        "Write",
			"description": "Create or overwrite a file for the Anthropic text editor tool family.",
			"inputSchema": map[string]any{
				"json": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{"type": "string"},
						"content":   map[string]any{"type": "string"},
					},
					"required":             []string{"file_path", "content"},
					"additionalProperties": false,
				},
			},
		},
	}
}

func makeOfficialTextEditorEditToolSpecification() map[string]any {
	return map[string]any{
		"toolSpecification": map[string]any{
			"name":        "Edit",
			"description": "Replace text in a file for the Anthropic text editor tool family.",
			"inputSchema": map[string]any{
				"json": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path":   map[string]any{"type": "string"},
						"old_string":  map[string]any{"type": "string"},
						"new_string":  map[string]any{"type": "string"},
						"replace_all": map[string]any{"type": "boolean"},
					},
					"required":             []string{"file_path", "old_string", "new_string"},
					"additionalProperties": false,
				},
			},
		},
	}
}

func supportedShadowTool(tool map[string]any) (string, ShadowToolBridge, bool) {
	toolType := strings.ToLower(strings.TrimSpace(stringField(tool, "type")))
	switch {
	case toolType == "google_search", strings.HasPrefix(toolType, "web_search"):
		return ShadowToolWebSearch, ShadowToolBridge{
			AnthropicType: toolType,
			AnthropicName: strings.TrimSpace(stringField(tool, "name")),
		}, true
	case strings.HasPrefix(toolType, "web_fetch"):
		return ShadowToolWebFetch, ShadowToolBridge{
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
	case ShadowToolWebSearch:
		description = "Internal gateway bridge for Anthropic web_search server tools"
		schema["properties"] = map[string]any{
			"query": map[string]any{"type": "string"},
		}
		schema["required"] = []string{"query"}
	case ShadowToolWebFetch:
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

func ensureHistoryShadowTools(history []any, tools []map[string]any, bridgeMetadata *BridgeMetadata) ([]map[string]any, *BridgeMetadata) {
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
		for _, toolUse := range historyShadowToolUses(msg["content"]) {
			if !isShadowToolName(toolUse.Name) {
				continue
			}
			key := strings.ToLower(toolUse.Name)
			if _, ok := seen[key]; !ok {
				tools = append(tools, makeShadowToolSpecification(toolUse.Name))
				seen[key] = struct{}{}
			}
			if bridgeMetadata == nil {
				bridgeMetadata = &BridgeMetadata{ShadowTools: map[string]ShadowToolBridge{}}
			}
			if bridgeMetadata.ShadowTools == nil {
				bridgeMetadata.ShadowTools = map[string]ShadowToolBridge{}
			}
			if existing, ok := bridgeMetadata.ShadowTools[toolUse.Name]; !ok || isShadowBridgeZero(existing) || shadowBridgeHasConstraints(toolUse.Bridge) {
				bridgeMetadata.ShadowTools[toolUse.Name] = mergeShadowBridge(shadowBridgeForName(toolUse.Name), toolUse.Bridge)
			}
		}
	}

	if bridgeMetadata != nil && len(bridgeMetadata.ShadowTools) == 0 {
		bridgeMetadata = nil
	}
	return tools, bridgeMetadata
}

func ensureHistoryNativeServerTools(history []any, tools []map[string]any, toolMetadata *ToolMetadata) ([]map[string]any, *ToolMetadata) {
	required := historyNativeServerTools(history)
	if len(required) == 0 {
		return tools, toolMetadata
	}

	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		spec, _ := tool["toolSpecification"].(map[string]any)
		name := strings.TrimSpace(stringField(spec, "name"))
		if name != "" {
			seen[strings.ToLower(name)] = struct{}{}
		}
	}
	for name, bridge := range required {
		if _, ok := seen[name]; !ok {
			tools = append(tools, nativeServerToolSpecificationForBridge(bridge))
			seen[name] = struct{}{}
		}
		if toolMetadata == nil {
			toolMetadata = &ToolMetadata{ResponseTools: map[string]ResponseToolBridge{}}
		}
		if toolMetadata.ResponseTools == nil {
			toolMetadata.ResponseTools = map[string]ResponseToolBridge{}
		}
		if _, ok := toolMetadata.ResponseTools[name]; !ok {
			toolMetadata.ResponseTools[name] = bridge
		}
	}
	return tools, toolMetadata
}

func historyNativeServerTools(history []any) map[string]ResponseToolBridge {
	out := map[string]ResponseToolBridge{}
	for _, item := range history {
		msg, _ := item.(map[string]any)
		blocks, _ := msg["content"].([]any)
		if len(blocks) == 0 {
			continue
		}
		for _, rawBlock := range blocks {
			block, _ := rawBlock.(map[string]any)
			blockType := strings.TrimSpace(stringField(block, "type"))
			switch blockType {
			case "server_tool_use":
				name := strings.ToLower(strings.TrimSpace(stringField(block, "name")))
				if name == "google_search" || strings.HasPrefix(name, "web_search") {
					out["web_search"] = ResponseToolBridge{AnthropicType: "web_search", AnthropicName: "web_search", Family: "anthropic_web_search"}
				}
				if strings.HasPrefix(name, "web_fetch") {
					out["web_fetch"] = ResponseToolBridge{AnthropicType: "web_fetch", AnthropicName: "web_fetch", Family: "anthropic_web_fetch"}
				}
			case "web_search_tool_result":
				out["web_search"] = ResponseToolBridge{AnthropicType: "web_search", AnthropicName: "web_search", Family: "anthropic_web_search"}
			case "web_fetch_tool_result":
				out["web_fetch"] = ResponseToolBridge{AnthropicType: "web_fetch", AnthropicName: "web_fetch", Family: "anthropic_web_fetch"}
			}
		}
	}
	return out
}

func nativeServerToolSpecificationForBridge(bridge ResponseToolBridge) map[string]any {
	switch bridge.AnthropicName {
	case "web_fetch":
		return makeNativeServerToolSpecification("web_fetch", "Fetch a URL using Kiro's native web_fetch tool.", "url")
	default:
		return makeNativeServerToolSpecification("web_search", "Search the web using Kiro's native web_search tool.", "query")
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

func isShadowToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case ShadowToolWebSearch, ShadowToolWebFetch:
		return true
	default:
		return false
	}
}

func shadowBridgeForName(name string) ShadowToolBridge {
	switch strings.TrimSpace(name) {
	case ShadowToolWebSearch:
		return ShadowToolBridge{
			AnthropicType: "web_search",
			AnthropicName: "web_search",
		}
	case ShadowToolWebFetch:
		return ShadowToolBridge{
			AnthropicType: "web_fetch",
			AnthropicName: "web_fetch",
		}
	default:
		return ShadowToolBridge{}
	}
}

type historyShadowToolUse struct {
	Name   string
	Bridge ShadowToolBridge
}

func historyShadowToolUses(content any) []historyShadowToolUse {
	items, _ := content.([]any)
	if len(items) == 0 {
		return nil
	}
	out := make([]historyShadowToolUse, 0, len(items))
	for _, item := range items {
		block, _ := item.(map[string]any)
		if strings.TrimSpace(stringField(block, "type")) != "tool_use" {
			continue
		}
		name := strings.TrimSpace(stringField(block, "name"))
		if !isShadowToolName(name) {
			continue
		}
		out = append(out, historyShadowToolUse{
			Name:   name,
			Bridge: shadowBridgeFromHistoryBlock(block),
		})
	}
	return out
}

func shadowBridgeFromHistoryBlock(block map[string]any) ShadowToolBridge {
	if bridge := shadowBridgeFromAnyMap(jsonMapField(block, "_shadow_bridge")); !isShadowBridgeZero(bridge) {
		return mergeShadowBridge(shadowBridgeForName(strings.TrimSpace(stringField(block, "name"))), bridge)
	}
	return shadowBridgeForName(strings.TrimSpace(stringField(block, "name")))
}

func shadowBridgeFromAnyMap(raw map[string]any) ShadowToolBridge {
	if len(raw) == 0 {
		return ShadowToolBridge{}
	}
	return ShadowToolBridge{
		AnthropicType:    strings.TrimSpace(stringField(raw, "anthropic_type")),
		AnthropicName:    strings.TrimSpace(stringField(raw, "anthropic_name")),
		AllowedDomains:   stringArrayField(raw["allowed_domains"]),
		BlockedDomains:   stringArrayField(raw["blocked_domains"]),
		MaxUses:          intField(raw["max_uses"]),
		MaxContentTokens: intField(raw["max_content_tokens"]),
	}
}

func mergeShadowBridge(base, override ShadowToolBridge) ShadowToolBridge {
	if strings.TrimSpace(override.AnthropicType) != "" {
		base.AnthropicType = override.AnthropicType
	}
	if strings.TrimSpace(override.AnthropicName) != "" {
		base.AnthropicName = override.AnthropicName
	}
	if len(override.AllowedDomains) > 0 {
		base.AllowedDomains = append([]string(nil), override.AllowedDomains...)
	}
	if len(override.BlockedDomains) > 0 {
		base.BlockedDomains = append([]string(nil), override.BlockedDomains...)
	}
	if override.MaxUses > 0 {
		base.MaxUses = override.MaxUses
	}
	if override.MaxContentTokens > 0 {
		base.MaxContentTokens = override.MaxContentTokens
	}
	return base
}

func isShadowBridgeZero(bridge ShadowToolBridge) bool {
	return strings.TrimSpace(bridge.AnthropicType) == "" &&
		strings.TrimSpace(bridge.AnthropicName) == "" &&
		len(bridge.AllowedDomains) == 0 &&
		len(bridge.BlockedDomains) == 0 &&
		bridge.MaxUses == 0 &&
		bridge.MaxContentTokens == 0
}

func shadowBridgeHasConstraints(bridge ShadowToolBridge) bool {
	return len(bridge.AllowedDomains) > 0 ||
		len(bridge.BlockedDomains) > 0 ||
		bridge.MaxUses > 0 ||
		bridge.MaxContentTokens > 0
}

func jsonMapField(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	value, _ := parent[key].(map[string]any)
	return value
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
			if sessionID := strings.TrimSpace(parsed.SessionID); sessionID != "" {
				return sessionID
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
