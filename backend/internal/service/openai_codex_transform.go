package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

var codexModelMap = map[string]string{
	"gpt-5.5":                    "gpt-5.5",
	"gpt-5.4":                    "gpt-5.4",
	"gpt-5.4-mini":               "gpt-5.4-mini",
	"gpt-5.4-nano":               "gpt-5.4-nano",
	"gpt-5.4-none":               "gpt-5.4",
	"gpt-5.4-low":                "gpt-5.4",
	"gpt-5.4-medium":             "gpt-5.4",
	"gpt-5.4-high":               "gpt-5.4",
	"gpt-5.4-xhigh":              "gpt-5.4",
	"gpt-5.4-chat-latest":        "gpt-5.4",
	"gpt-5.3":                    "gpt-5.3-codex",
	"gpt-5.3-none":               "gpt-5.3-codex",
	"gpt-5.3-low":                "gpt-5.3-codex",
	"gpt-5.3-medium":             "gpt-5.3-codex",
	"gpt-5.3-high":               "gpt-5.3-codex",
	"gpt-5.3-xhigh":              "gpt-5.3-codex",
	"gpt-5.3-codex":              "gpt-5.3-codex",
	"gpt-5.3-codex-spark":        "gpt-5.3-codex-spark",
	"gpt-5.3-codex-spark-low":    "gpt-5.3-codex-spark",
	"gpt-5.3-codex-spark-medium": "gpt-5.3-codex-spark",
	"gpt-5.3-codex-spark-high":   "gpt-5.3-codex-spark",
	"gpt-5.3-codex-spark-xhigh":  "gpt-5.3-codex-spark",
	"gpt-5.3-codex-low":          "gpt-5.3-codex",
	"gpt-5.3-codex-medium":       "gpt-5.3-codex",
	"gpt-5.3-codex-high":         "gpt-5.3-codex",
	"gpt-5.3-codex-xhigh":        "gpt-5.3-codex",
	"gpt-5.1-codex":              "gpt-5.3-codex",
	"gpt-5.1-codex-low":          "gpt-5.3-codex",
	"gpt-5.1-codex-medium":       "gpt-5.3-codex",
	"gpt-5.1-codex-high":         "gpt-5.3-codex",
	"gpt-5.1-codex-max":          "gpt-5.3-codex",
	"gpt-5.1-codex-max-low":      "gpt-5.3-codex",
	"gpt-5.1-codex-max-medium":   "gpt-5.3-codex",
	"gpt-5.1-codex-max-high":     "gpt-5.3-codex",
	"gpt-5.1-codex-max-xhigh":    "gpt-5.3-codex",
	"gpt-5.2":                    "gpt-5.2",
	"gpt-5.2-none":               "gpt-5.2",
	"gpt-5.2-low":                "gpt-5.2",
	"gpt-5.2-medium":             "gpt-5.2",
	"gpt-5.2-high":               "gpt-5.2",
	"gpt-5.2-xhigh":              "gpt-5.2",
	"gpt-5.2-codex":              "gpt-5.2",
	"gpt-5.2-codex-low":          "gpt-5.2",
	"gpt-5.2-codex-medium":       "gpt-5.2",
	"gpt-5.2-codex-high":         "gpt-5.2",
	"gpt-5.2-codex-xhigh":        "gpt-5.2",
	"gpt-5.1-codex-mini":         "gpt-5.3-codex",
	"gpt-5.1-codex-mini-medium":  "gpt-5.3-codex",
	"gpt-5.1-codex-mini-high":    "gpt-5.3-codex",
	"gpt-5.1":                    "gpt-5.4",
	"gpt-5.1-none":               "gpt-5.4",
	"gpt-5.1-low":                "gpt-5.4",
	"gpt-5.1-medium":             "gpt-5.4",
	"gpt-5.1-high":               "gpt-5.4",
	"gpt-5.1-chat-latest":        "gpt-5.4",
	"gpt-5-codex":                "gpt-5.3-codex",
	"codex-mini-latest":          "gpt-5.3-codex",
	"gpt-5-codex-mini":           "gpt-5.3-codex",
	"gpt-5-codex-mini-medium":    "gpt-5.3-codex",
	"gpt-5-codex-mini-high":      "gpt-5.3-codex",
	"gpt-5":                      "gpt-5.4",
	"gpt-5-mini":                 "gpt-5.4",
	"gpt-5-nano":                 "gpt-5.4",
}

type codexTransformResult struct {
	Modified        bool
	NormalizedModel string
	PromptCacheKey  string
	Observability   codexTransformObservability
}

type codexTransformObservability struct {
	NeedsToolContinuation         bool
	ModelNormalized               bool
	CompactStoreRemoved           bool
	CompactStreamRemoved          bool
	CompactDeferredToolSearch     bool
	StoreForcedFalse              bool
	StreamForcedTrue              bool
	UnsupportedFieldsStripped     []string
	FunctionsConverted            bool
	FunctionCallConverted         bool
	ToolsNormalized               bool
	ToolChoiceNormalized          bool
	SystemMessagesExtracted       bool
	DefaultInstructionsApplied    bool
	SparkInstructionsApplied      bool
	InputToolRoleNormalized       bool
	InputMessageContentNormalized bool
	InputFiltered                 bool
	InputStringWrapped            bool
	InputItemsBefore              int
	InputItemsAfter               int
}

type codexTransformInputMode int

const (
	codexTransformInputModeStrict codexTransformInputMode = iota
	codexTransformInputModePreservePrefix
)

type codexInputFilterOptions struct {
	rewriteToolContinuationIDs bool
	dropItemReferences         bool
	dropNonToolItemIDs         bool
}

type codexOAuthTransformOptions struct {
	SkipDefaultInstructions bool
	PreserveToolCallIDs     bool
}

var openAIChatGPTInternalUnsupportedFields = []string{
	"user",
	"metadata",
	"prompt_cache_retention",
	"reasoningSummary",
	"safety_identifier",
	"stream_options",
	"max_output_tokens",
	"max_completion_tokens",
	"temperature",
	"top_p",
}

const (
	codexImageGenerationBridgeMarker = "<sub2api-codex-image-generation>"
	codexImageGenerationBridgeText   = codexImageGenerationBridgeMarker + "\nWhen the user asks for raster image generation or editing, use the OpenAI Responses native `image_generation` tool attached to this request. The local Codex client may not expose an `image_gen` namespace, but that does not mean image generation is unavailable. Do not ask the user to switch to CLI fallback solely because `image_gen` is absent.\n</sub2api-codex-image-generation>"
	codexSparkImageUnsupportedMarker = "<sub2api-codex-spark-image-unsupported>"
	codexSparkImageUnsupportedText   = codexSparkImageUnsupportedMarker + "\nThe current model is gpt-5.3-codex-spark, which does not support image generation, image editing, image input, the `image_generation` tool, or Codex `image_gen`/`$imagegen` workflows. If the user asks for image generation or image editing, clearly explain this model limitation and ask them to switch to a non-Spark Codex model such as gpt-5.3-codex or gpt-5.4. Do not claim that the local environment merely lacks image_gen tooling, and do not suggest CLI fallback as the primary fix while the model remains Spark.\n</sub2api-codex-spark-image-unsupported>"
)

func applyCodexOAuthTransform(reqBody map[string]any, isCodexCLI bool, isCompact bool) codexTransformResult {
	return applyCodexOAuthTransformWithInputModeAndFallbackReason(reqBody, isCodexCLI, isCompact, codexTransformInputModeStrict, "")
}

func applyCodexOAuthTransformWithInputMode(
	reqBody map[string]any,
	isCodexCLI bool,
	isCompact bool,
	inputMode codexTransformInputMode,
) codexTransformResult {
	return applyCodexOAuthTransformWithInputModeAndFallbackReason(reqBody, isCodexCLI, isCompact, inputMode, "")
}

func applyCodexOAuthTransformWithOptions(reqBody map[string]any, opts codexOAuthTransformOptions) codexTransformResult {
	result := applyCodexOAuthTransform(reqBody, false, false)
	if opts.SkipDefaultInstructions {
		if instructions, ok := reqBody["instructions"].(string); ok {
			defaultInstructions := strings.TrimSpace(openai.DefaultInstructions)
			if defaultInstructions == "" {
				defaultInstructions = "You are a helpful coding assistant."
			}
			if strings.TrimSpace(instructions) == defaultInstructions {
				delete(reqBody, "instructions")
				result.Modified = true
				result.Observability.DefaultInstructionsApplied = false
			}
		}
	}
	if v, ok := reqBody["prompt_cache_key"].(string); ok {
		result.PromptCacheKey = strings.TrimSpace(v)
		delete(reqBody, "prompt_cache_key")
		result.Modified = true
	}
	return result
}

func applyCodexOAuthTransformWithInputModeAndFallbackReason(
	reqBody map[string]any,
	isCodexCLI bool,
	isCompact bool,
	inputMode codexTransformInputMode,
	fallbackReason string,
) codexTransformResult {
	result := codexTransformResult{}
	// 工具续链需求会影响存储策略与 input 过滤逻辑。
	needsToolContinuation := NeedsToolContinuation(reqBody)
	result.Observability.NeedsToolContinuation = needsToolContinuation
	if needsToolContinuation {
		recordOpenAICompatToolContinuationDetected()
	}

	model := ""
	if v, ok := reqBody["model"].(string); ok {
		model = v
	}
	profile := ResolveCodexRequestProfile(model)
	if profile.UpstreamModel != "" {
		if model != profile.UpstreamModel {
			reqBody["model"] = profile.UpstreamModel
			result.Modified = true
			result.Observability.ModelNormalized = true
		}
		result.NormalizedModel = profile.UpstreamModel
	}
	normalizedModel := result.NormalizedModel
	if normalizedModel == "" {
		normalizedModel = model
	}

	if isCompact {
		if _, ok := reqBody["store"]; ok {
			delete(reqBody, "store")
			result.Modified = true
			result.Observability.CompactStoreRemoved = true
		}
		if _, ok := reqBody["stream"]; ok {
			delete(reqBody, "stream")
			result.Modified = true
			result.Observability.CompactStreamRemoved = true
		}
		if ensureCompactDeferredToolSearchInMap(reqBody) {
			result.Modified = true
			result.Observability.CompactDeferredToolSearch = true
		}
	} else {
		// OAuth 走 ChatGPT internal API 时，store 必须为 false；显式 true 也会强制覆盖。
		// 避免上游返回 "Store must be set to false"。
		if v, ok := reqBody["store"].(bool); !ok || v {
			reqBody["store"] = false
			result.Modified = true
			result.Observability.StoreForcedFalse = true
		}
		if v, ok := reqBody["stream"].(bool); !ok || !v {
			reqBody["stream"] = true
			result.Modified = true
			result.Observability.StreamForcedTrue = true
		}
	}

	// Strip parameters unsupported by codex models via the Responses API.
	unsupportedKeys := []string{
		"user",
		"metadata",
		"prompt_cache_retention",
		"reasoningSummary",
		"safety_identifier",
		"stream_options",
		"max_output_tokens",
		"max_completion_tokens",
		"frequency_penalty",
		"presence_penalty",
		// prompt_cache_retention is a newer Responses API parameter (cache TTL).
		// The ChatGPT internal Codex endpoint rejects it with
		// "Unsupported parameter: prompt_cache_retention". Defense-in-depth
		// for any OAuth path that reaches this transform — the Cursor
		// Responses-shape short-circuit in ForwardAsChatCompletions strips
		// it earlier too, but we keep this line so other OAuth callers are
		// equally protected.
		"prompt_cache_retention",
	}
	if !profile.SupportsTemperature {
		unsupportedKeys = append(unsupportedKeys, "temperature")
	}
	if !profile.SupportsTopP {
		unsupportedKeys = append(unsupportedKeys, "top_p")
	}
	for _, key := range unsupportedKeys {
		if _, ok := reqBody[key]; ok {
			delete(reqBody, key)
			recordOpenAICompatStrippedField(key)
			result.Modified = true
			result.Observability.UnsupportedFieldsStripped = append(result.Observability.UnsupportedFieldsStripped, key)
		}
	}

	// 兼容遗留的 functions 和 function_call，转换为 tools 和 tool_choice
	if functionsRaw, ok := reqBody["functions"]; ok {
		if functions, k := functionsRaw.([]any); k {
			tools := make([]any, 0, len(functions))
			for _, f := range functions {
				tools = append(tools, map[string]any{
					"type":     "function",
					"function": f,
				})
			}
			reqBody["tools"] = tools
		}
		delete(reqBody, "functions")
		result.Modified = true
		result.Observability.FunctionsConverted = true
	}

	if fcRaw, ok := reqBody["function_call"]; ok {
		if fcStr, ok := fcRaw.(string); ok {
			// e.g. "auto", "none"
			reqBody["tool_choice"] = fcStr
		} else if fcObj, ok := fcRaw.(map[string]any); ok {
			// e.g. {"name": "my_func"}
			if name, ok := fcObj["name"].(string); ok && strings.TrimSpace(name) != "" {
				reqBody["tool_choice"] = map[string]any{
					"type": "function",
					"function": map[string]any{
						"name": name,
					},
				}
			}
		}
		delete(reqBody, "function_call")
		result.Modified = true
		result.Observability.FunctionCallConverted = true
	}

	if normalizeCodexTools(reqBody) {
		result.Modified = true
		result.Observability.ToolsNormalized = true
	}
	if normalizeCodexToolChoice(reqBody) {
		result.Modified = true
		result.Observability.ToolChoiceNormalized = true
	}

	if v, ok := reqBody["prompt_cache_key"].(string); ok {
		result.PromptCacheKey = strings.TrimSpace(v)
	}

	// 提取 input 中 role:"system" 消息至 instructions（OAuth 上游不支持 system role）。
	if extractSystemMessagesFromInput(reqBody) {
		result.Modified = true
		result.Observability.SystemMessagesExtracted = true
	}

	// instructions 处理逻辑：根据是否是 Codex CLI 分别调用不同方法
	if applyInstructions(reqBody, isCodexCLI) {
		result.Modified = true
		result.Observability.DefaultInstructionsApplied = true
	}
	if !isCodexSparkModel(normalizedModel) && applyCodexImageGenerationBridgeInstructions(reqBody) {
		result.Modified = true
	}
	if isCodexSparkModel(normalizedModel) && applyCodexSparkImageUnsupportedInstructions(reqBody) {
		result.Modified = true
		result.Observability.SparkInstructionsApplied = true
	}

	// 续链场景保留 item_reference 与 id，避免 call_id 上下文丢失。
	if input, ok := reqBody["input"].([]any); ok {
		result.Observability.InputItemsBefore = len(input)
		inputModified := false
		if normalizedInput, modified := normalizeCodexToolRoleMessages(input); modified {
			input = normalizedInput
			inputModified = true
			result.Modified = true
			result.Observability.InputToolRoleNormalized = true
		}
		if normalizedInput, modified := normalizeCodexMessageContentText(input); modified {
			input = normalizedInput
			inputModified = true
			result.Modified = true
			result.Observability.InputMessageContentNormalized = true
		}
		filterOptions := codexInputFilterOptions{
			rewriteToolContinuationIDs: hasCodexToolContinuationInput(input),
		}
		if !filterOptions.rewriteToolContinuationIDs {
			filterOptions.dropItemReferences = true
			filterOptions.dropNonToolItemIDs = inputMode != codexTransformInputModePreservePrefix
		}
		if inputMode == codexTransformInputModePreservePrefix {
			filterOptions.rewriteToolContinuationIDs = false
		}
		switch strings.TrimSpace(fallbackReason) {
		case "call_id", "input_schema":
			filterOptions.dropNonToolItemIDs = true
		case "item_reference":
			filterOptions.dropItemReferences = true
			filterOptions.dropNonToolItemIDs = true
		}
		filteredInput, filteredModified := filterCodexInputWithOptions(input, filterOptions)
		if filteredModified {
			input = filteredInput
			inputModified = true
			result.Modified = true
			result.Observability.InputFiltered = true
		}
		if inputModified {
			reqBody["input"] = input
		}
		result.Observability.InputItemsAfter = len(input)
	} else if inputStr, ok := reqBody["input"].(string); ok {
		// ChatGPT codex endpoint requires input to be a list, not a string.
		// Convert string input to the expected message array format.
		trimmed := strings.TrimSpace(inputStr)
		if trimmed != "" {
			reqBody["input"] = []any{
				map[string]any{
					"type":    "message",
					"role":    "user",
					"content": inputStr,
				},
			}
		} else {
			reqBody["input"] = []any{}
		}
		result.Modified = true
		result.Observability.InputStringWrapped = true
		result.Observability.InputItemsBefore = 1
		if strings.TrimSpace(inputStr) == "" {
			result.Observability.InputItemsAfter = 0
		} else {
			result.Observability.InputItemsAfter = 1
		}
	}

	return result
}

func ensureCompactDeferredToolSearchInMap(reqBody map[string]any) bool {
	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		return false
	}

	switch tools := rawTools.(type) {
	case []any:
		hasDeferred := false
		hasToolSearch := false
		for _, rawTool := range tools {
			tool, ok := rawTool.(map[string]any)
			if !ok {
				continue
			}
			if v, _ := tool["defer_loading"].(bool); v {
				hasDeferred = true
			}
			if strings.TrimSpace(firstNonEmptyString(tool["type"])) == "tool_search" {
				hasToolSearch = true
			}
		}
		if !hasDeferred || hasToolSearch {
			return false
		}
		reqBody["tools"] = append(tools, map[string]any{"type": "tool_search"})
		return true
	case map[string]any:
		deferLoading, _ := tools["defer_loading"].(bool)
		if !deferLoading {
			return false
		}
		if _, ok := tools["tool_search"]; ok {
			return false
		}
		tools["tool_search"] = map[string]any{"type": "tool_search"}
		return true
	default:
		return false
	}
}

func normalizeCodexToolChoice(reqBody map[string]any) bool {
	choice, ok := reqBody["tool_choice"]
	if !ok || choice == nil {
		return false
	}
	choiceMap, ok := choice.(map[string]any)
	if !ok {
		return false
	}
	choiceType := strings.TrimSpace(firstNonEmptyString(choiceMap["type"]))
	if choiceType == "" {
		return false
	}
	if choiceType == "function" {
		name := codexToolChoiceFunctionName(choiceMap)
		if name == "" || !codexToolsContainFunctionName(reqBody["tools"], name) {
			reqBody["tool_choice"] = "auto"
			return true
		}
		reqBody["tool_choice"] = map[string]any{
			"type": "function",
			"name": name,
		}
		return true
	}
	if codexToolsContainType(reqBody["tools"], choiceType) {
		return false
	}
	reqBody["tool_choice"] = "auto"
	return true
}

func codexToolChoiceFunctionName(choice map[string]any) string {
	if name := strings.TrimSpace(firstNonEmptyString(choice["name"])); name != "" {
		return name
	}
	if fn, ok := choice["function"].(map[string]any); ok {
		return strings.TrimSpace(firstNonEmptyString(fn["name"]))
	}
	return ""
}

func codexToolsContainType(rawTools any, toolType string) bool {
	tools, ok := rawTools.([]any)
	if !ok || strings.TrimSpace(toolType) == "" {
		return false
	}
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(tool["type"])) == toolType {
			return true
		}
	}
	return false
}

func codexToolsContainFunctionName(rawTools any, name string) bool {
	tools, ok := rawTools.([]any)
	if !ok || strings.TrimSpace(name) == "" {
		return false
	}
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(tool["type"])) != "function" {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(tool["name"])) == strings.TrimSpace(name) {
			return true
		}
	}
	return false
}

func normalizeCodexToolRoleMessages(input []any) ([]any, bool) {
	if len(input) == 0 {
		return input, false
	}

	modified := false
	normalized := make([]any, 0, len(input))
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			normalized = append(normalized, item)
			continue
		}
		role, _ := m["role"].(string)
		if strings.TrimSpace(role) != "tool" {
			normalized = append(normalized, item)
			continue
		}

		callID := firstNonEmptyString(m["call_id"], m["tool_call_id"], m["id"])
		callID = strings.TrimSpace(callID)
		if callID == "" {
			// Responses does not accept role:"tool". If no call id is available,
			// preserve the text as a user message instead of sending invalid input.
			fallback := make(map[string]any, len(m))
			for key, value := range m {
				fallback[key] = value
			}
			fallback["role"] = "user"
			delete(fallback, "tool_call_id")
			normalized = append(normalized, fallback)
			modified = true
			continue
		}

		output := extractTextFromContent(m["content"])
		if output == "" {
			if value, ok := m["output"].(string); ok {
				output = value
			}
		}
		if output == "" && m["content"] != nil {
			if b, err := json.Marshal(m["content"]); err == nil {
				output = string(b)
			}
		}

		normalized = append(normalized, map[string]any{
			"type":    "function_call_output",
			"call_id": callID,
			"output":  output,
		})
		modified = true
	}
	if !modified {
		return input, false
	}
	return normalized, true
}

func normalizeCodexMessageContentText(input []any) ([]any, bool) {
	if len(input) == 0 {
		return input, false
	}

	modified := false
	normalized := make([]any, 0, len(input))
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok || strings.TrimSpace(firstNonEmptyString(m["type"])) != "message" {
			normalized = append(normalized, item)
			continue
		}
		parts, ok := m["content"].([]any)
		if !ok {
			normalized = append(normalized, item)
			continue
		}

		var newItem map[string]any
		var newParts []any
		ensureItemCopy := func() {
			if newItem != nil {
				return
			}
			newItem = make(map[string]any, len(m))
			for key, value := range m {
				newItem[key] = value
			}
			newParts = make([]any, len(parts))
			copy(newParts, parts)
		}

		for i, rawPart := range parts {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			text, hasText := part["text"]
			if !hasText {
				continue
			}
			if _, ok := text.(string); ok {
				continue
			}

			ensureItemCopy()
			newPart := make(map[string]any, len(part))
			for key, value := range part {
				newPart[key] = value
			}
			newPart["text"] = stringifyCodexContentText(text)
			newParts[i] = newPart
			modified = true
		}

		if newItem != nil {
			newItem["content"] = newParts
			normalized = append(normalized, newItem)
			continue
		}
		normalized = append(normalized, item)
	}
	if !modified {
		return input, false
	}
	return normalized, true
}

func stringifyCodexContentText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return fmt.Sprint(v)
	}
}

func normalizeCodexModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "gpt-5.4"
	}
	if isOpenAIImageGenerationModel(model) {
		return model
	}

	modelID := model
	if strings.Contains(modelID, "/") {
		parts := strings.Split(modelID, "/")
		modelID = parts[len(parts)-1]
	}
	if canonical := canonicalizeOpenAIModelAliasSpelling(modelID); canonical != "" {
		modelID = canonical
	}

	if mapped := getNormalizedCodexModel(modelID); mapped != "" {
		return mapped
	}

	normalized := strings.ToLower(modelID)

	if strings.Contains(normalized, "gpt-5.5") || strings.Contains(normalized, "gpt 5.5") {
		return "gpt-5.5"
	}
	if strings.Contains(normalized, "gpt-5.4-mini") || strings.Contains(normalized, "gpt 5.4 mini") {
		return "gpt-5.4-mini"
	}
	if strings.Contains(normalized, "gpt-5.4-nano") || strings.Contains(normalized, "gpt 5.4 nano") {
		return "gpt-5.4-nano"
	}
	if strings.Contains(normalized, "gpt-5.4") || strings.Contains(normalized, "gpt 5.4") {
		return "gpt-5.4"
	}
	if strings.Contains(normalized, "gpt-5.3-codex-spark") || strings.Contains(normalized, "gpt 5.3 codex spark") {
		return "gpt-5.3-codex-spark"
	}
	if strings.Contains(normalized, "gpt-5.3-codex") || strings.Contains(normalized, "gpt 5.3 codex") {
		return "gpt-5.3-codex"
	}
	if strings.Contains(normalized, "gpt-5.3") || strings.Contains(normalized, "gpt 5.3") {
		return "gpt-5.3-codex"
	}
	if strings.Contains(normalized, "gpt-5.2-codex") || strings.Contains(normalized, "gpt 5.2 codex") {
		return "gpt-5.2"
	}
	if strings.Contains(normalized, "gpt-5.2") || strings.Contains(normalized, "gpt 5.2") {
		return "gpt-5.2"
	}
	if strings.Contains(normalized, "gpt-5.1-codex-max") || strings.Contains(normalized, "gpt 5.1 codex max") {
		return "gpt-5.3-codex"
	}
	if strings.Contains(normalized, "gpt-5.1-codex-mini") || strings.Contains(normalized, "gpt 5.1 codex mini") {
		return "gpt-5.3-codex"
	}
	if strings.Contains(normalized, "codex-mini-latest") ||
		strings.Contains(normalized, "gpt-5-codex-mini") ||
		strings.Contains(normalized, "gpt 5 codex mini") {
		return "gpt-5.3-codex"
	}
	if strings.Contains(normalized, "gpt-5.1-codex") || strings.Contains(normalized, "gpt 5.1 codex") {
		return "gpt-5.3-codex"
	}
	if strings.Contains(normalized, "gpt-5.1") || strings.Contains(normalized, "gpt 5.1") {
		return "gpt-5.4"
	}
	if strings.Contains(normalized, "gpt-5-mini") || strings.Contains(normalized, "gpt 5 mini") {
		return "gpt-5.4"
	}
	if strings.Contains(normalized, "gpt-5-nano") || strings.Contains(normalized, "gpt 5 nano") {
		return "gpt-5.4"
	}
	if strings.Contains(normalized, "codex") {
		return modelID
	}
	if strings.Contains(normalized, "gpt-5") || strings.Contains(normalized, "gpt 5") {
		return "gpt-5.4"
	}

	return strings.TrimSpace(modelID)
}

func isCodexSparkModel(model string) bool {
	return normalizeCodexModel(model) == "gpt-5.3-codex-spark"
}

func hasOpenAIImageGenerationTool(reqBody map[string]any) bool {
	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		return false
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false
	}
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(toolMap["type"])) == "image_generation" {
			return true
		}
	}
	return false
}

func stripOpenAIImageGenerationTools(reqBody map[string]any) bool {
	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		return false
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false
	}

	kept := make([]any, 0, len(tools))
	removed := false
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if ok && strings.TrimSpace(firstNonEmptyString(toolMap["type"])) == "image_generation" {
			removed = true
			continue
		}
		kept = append(kept, rawTool)
	}
	if !removed {
		return false
	}
	if len(kept) == 0 {
		delete(reqBody, "tools")
		return true
	}
	reqBody["tools"] = kept
	return true
}

func stripOpenAIImageGenerationToolsBytes(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return body, false, err
	}
	if !stripOpenAIImageGenerationTools(reqBody) {
		return body, false, nil
	}
	normalized, err := marshalOpenAIResponsesRequestBodyOrdered(reqBody)
	if err != nil {
		return body, false, err
	}
	return normalized, true, nil
}

func hasOpenAIInputImage(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	return hasOpenAIInputImageValue(reqBody["input"]) || hasOpenAIInputImageValue(reqBody["messages"])
}

func hasOpenAIInputImageValue(value any) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if hasOpenAIInputImageValue(item) {
				return true
			}
		}
	case map[string]any:
		if strings.TrimSpace(firstNonEmptyString(v["type"])) == "input_image" {
			return true
		}
		if _, ok := v["image_url"]; ok {
			return true
		}
		return hasOpenAIInputImageValue(v["content"])
	}
	return false
}

func validateCodexSparkInput(reqBody map[string]any, model string) error {
	if !isCodexSparkModel(model) || !hasOpenAIInputImage(reqBody) {
		return nil
	}
	return fmt.Errorf("model %q does not support image input", strings.TrimSpace(model))
}

func normalizeOpenAIResponsesImageGenerationTools(reqBody map[string]any) bool {
	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		return false
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false
	}

	modified := false
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok || strings.TrimSpace(firstNonEmptyString(toolMap["type"])) != "image_generation" {
			continue
		}
		if _, ok := toolMap["output_format"]; !ok {
			if value := strings.TrimSpace(firstNonEmptyString(toolMap["format"])); value != "" {
				toolMap["output_format"] = value
				modified = true
			}
		}
		if _, ok := toolMap["output_compression"]; !ok {
			if value, exists := toolMap["compression"]; exists && value != nil {
				toolMap["output_compression"] = value
				modified = true
			}
		}
		if _, ok := toolMap["format"]; ok {
			delete(toolMap, "format")
			modified = true
		}
		if _, ok := toolMap["compression"]; ok {
			delete(toolMap, "compression")
			modified = true
		}
	}
	return modified
}

func ensureOpenAIResponsesImageGenerationTool(reqBody map[string]any) bool {
	if len(reqBody) == 0 {
		return false
	}
	if isCodexSparkModel(firstNonEmptyString(reqBody["model"])) {
		return false
	}

	tool := map[string]any{
		"type":          "image_generation",
		"output_format": "png",
	}

	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		reqBody["tools"] = []any{tool}
		return true
	}

	tools, ok := rawTools.([]any)
	if !ok {
		reqBody["tools"] = []any{tool}
		return true
	}
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(toolMap["type"])) == "image_generation" {
			return false
		}
	}

	reqBody["tools"] = append(tools, tool)
	return true
}

func applyCodexImageGenerationBridgeInstructions(reqBody map[string]any) bool {
	if len(reqBody) == 0 || !hasOpenAIImageGenerationTool(reqBody) {
		return false
	}
	if isCodexSparkModel(firstNonEmptyString(reqBody["model"])) {
		return false
	}

	existing, _ := reqBody["instructions"].(string)
	if strings.Contains(existing, codexImageGenerationBridgeMarker) {
		return false
	}

	existing = strings.TrimRight(existing, " \t\r\n")
	if strings.TrimSpace(existing) == "" {
		reqBody["instructions"] = codexImageGenerationBridgeText
		return true
	}

	reqBody["instructions"] = existing + "\n\n" + codexImageGenerationBridgeText
	return true
}

func applyCodexSparkImageUnsupportedInstructions(reqBody map[string]any) bool {
	if len(reqBody) == 0 {
		return false
	}
	existing, _ := reqBody["instructions"].(string)
	if strings.Contains(existing, codexSparkImageUnsupportedMarker) {
		return false
	}
	existing = strings.TrimRight(existing, " \t\r\n")
	if strings.TrimSpace(existing) == "" {
		reqBody["instructions"] = codexSparkImageUnsupportedText
		return true
	}
	reqBody["instructions"] = existing + "\n\n" + codexSparkImageUnsupportedText
	return true
}

func validateOpenAIResponsesImageModel(reqBody map[string]any, model string) error {
	if !hasOpenAIImageGenerationTool(reqBody) {
		return nil
	}
	model = strings.TrimSpace(model)
	if !isOpenAIImageGenerationModel(model) {
		return nil
	}
	return fmt.Errorf("/v1/responses image_generation requests require a Responses-capable text model; image-only model %q is not allowed", model)
}

func normalizeOpenAIResponsesImageOnlyModel(reqBody map[string]any, mainModel string) bool {
	if len(reqBody) == 0 {
		return false
	}
	mainModel = NormalizeOpenAIImageMainModel(mainModel)
	imageModel := strings.TrimSpace(firstNonEmptyString(reqBody["model"]))
	if !isOpenAIImageGenerationModel(imageModel) {
		return false
	}

	modified := false
	tools, _ := reqBody["tools"].([]any)
	imageToolIndex := -1
	for i, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(toolMap["type"])) == "image_generation" {
			imageToolIndex = i
			break
		}
	}
	if imageToolIndex < 0 {
		tools = append(tools, map[string]any{
			"type":  "image_generation",
			"model": imageModel,
		})
		imageToolIndex = len(tools) - 1
		reqBody["tools"] = tools
		modified = true
	}

	if toolMap, ok := tools[imageToolIndex].(map[string]any); ok {
		if strings.TrimSpace(firstNonEmptyString(toolMap["model"])) == "" {
			toolMap["model"] = imageModel
			modified = true
		}
		for _, key := range []string{
			"size",
			"quality",
			"background",
			"output_format",
			"output_compression",
			"moderation",
			"partial_images",
		} {
			if value, exists := reqBody[key]; exists && value != nil {
				if _, toolHas := toolMap[key]; !toolHas {
					toolMap[key] = value
				}
				delete(reqBody, key)
				modified = true
			}
		}
	}
	if _, exists := reqBody["style"]; exists {
		delete(reqBody, "style")
		modified = true
	}

	if prompt := strings.TrimSpace(firstNonEmptyString(reqBody["prompt"])); prompt != "" {
		if _, hasInput := reqBody["input"]; !hasInput {
			reqBody["input"] = prompt
		}
		delete(reqBody, "prompt")
		modified = true
	}

	if _, ok := reqBody["tool_choice"]; !ok {
		reqBody["tool_choice"] = map[string]any{"type": "image_generation"}
		modified = true
	}
	if imageModel != mainModel {
		modified = true
	}
	reqBody["model"] = mainModel
	return modified
}

func normalizeOpenAIModelForUpstream(account *Account, model string) string {
	if account == nil || account.Type == AccountTypeOAuth {
		return normalizeCodexModel(model)
	}
	return strings.TrimSpace(model)
}

func SupportsVerbosity(model string) bool {
	if !strings.HasPrefix(model, "gpt-") {
		return true
	}

	var major, minor int
	n, _ := fmt.Sscanf(model, "gpt-%d.%d", &major, &minor)

	if major > 5 {
		return true
	}
	if major < 5 {
		return false
	}

	// gpt-5
	if n == 1 {
		return true
	}

	return minor >= 3
}

func getNormalizedCodexModel(modelID string) string {
	if modelID == "" {
		return ""
	}
	if mapped, ok := codexModelMap[modelID]; ok {
		return mapped
	}
	lower := strings.ToLower(modelID)
	for key, value := range codexModelMap {
		if strings.ToLower(key) == lower {
			return value
		}
	}
	return ""
}

// extractTextFromContent extracts plain text from a content value that is either
// a Go string or a []any of content-part maps with type:"text".
func extractTextFromContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, part := range v {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t == "text" {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

// extractSystemMessagesFromInput scans the input array for items with role=="system",
// removes them, and merges their content into reqBody["instructions"].
// If instructions is already non-empty, extracted content is prepended with "\n\n".
// Returns true if any system messages were extracted.
func extractSystemMessagesFromInput(reqBody map[string]any) bool {
	input, ok := reqBody["input"].([]any)
	if !ok || len(input) == 0 {
		return false
	}

	var systemTexts []string
	remaining := make([]any, 0, len(input))

	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			remaining = append(remaining, item)
			continue
		}
		if role, _ := m["role"].(string); role != "system" {
			remaining = append(remaining, item)
			continue
		}
		if text := extractTextFromContent(m["content"]); text != "" {
			systemTexts = append(systemTexts, text)
		}
	}

	if len(systemTexts) == 0 {
		return false
	}

	extracted := strings.Join(systemTexts, "\n\n")
	if existing, ok := reqBody["instructions"].(string); ok && strings.TrimSpace(existing) != "" {
		reqBody["instructions"] = extracted + "\n\n" + existing
	} else {
		reqBody["instructions"] = extracted
	}
	reqBody["input"] = remaining
	return true
}

// applyInstructions 处理 instructions 字段：仅在 instructions 为空时填充默认值。
func applyInstructions(reqBody map[string]any, isCodexCLI bool) bool {
	return applyEmbeddedDefaultInstructions(reqBody)
}

func applyEmbeddedDefaultInstructions(reqBody map[string]any) bool {
	if !isInstructionsEmpty(reqBody) {
		return false
	}
	instructions := strings.TrimSpace(openai.DefaultInstructions)
	if instructions == "" {
		instructions = "You are a helpful coding assistant."
	}
	reqBody["instructions"] = instructions
	return true
}

// isInstructionsEmpty 检查 instructions 字段是否为空
// 处理以下情况：字段不存在、nil、空字符串、纯空白字符串
func isInstructionsEmpty(reqBody map[string]any) bool {
	val, exists := reqBody["instructions"]
	if !exists {
		return true
	}
	if val == nil {
		return true
	}
	str, ok := val.(string)
	if !ok {
		return true
	}
	return strings.TrimSpace(str) == ""
}

func normalizeOpenAIStrictFunctionToolSchemas(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}

	tools, ok := reqBody["tools"].([]any)
	if !ok || len(tools) == 0 {
		return false
	}

	modified := false
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok || !openAIStrictFunctionToolEnabled(toolMap) {
			continue
		}
		if normalizeOpenAIStrictFunctionParameters(toolMap) {
			modified = true
		}
	}

	return modified
}

func openAIStrictFunctionToolEnabled(toolMap map[string]any) bool {
	if toolMap == nil {
		return false
	}
	if strict, ok := toolMap["strict"].(bool); ok && strict {
		return true
	}
	function, ok := toolMap["function"].(map[string]any)
	if !ok {
		return false
	}
	strict, _ := function["strict"].(bool)
	return strict
}

func normalizeOpenAIStrictFunctionParameters(toolMap map[string]any) bool {
	if toolMap == nil {
		return false
	}

	parameters, ok := toolMap["parameters"].(map[string]any)
	if !ok {
		function, ok := toolMap["function"].(map[string]any)
		if !ok {
			return false
		}
		parameters, ok = function["parameters"].(map[string]any)
		if !ok {
			return false
		}
	}

	properties, ok := parameters["properties"].(map[string]any)
	if !ok {
		return false
	}

	existingRequired, _ := parameters["required"].([]any)
	if len(existingRequired) == 0 {
		return false
	}

	normalizedRequired := make([]string, 0, len(existingRequired))
	seen := make(map[string]struct{}, len(existingRequired))
	modified := false
	for _, raw := range existingRequired {
		key := strings.TrimSpace(firstNonEmptyString(raw))
		if key == "" {
			modified = true
			continue
		}
		if _, exists := properties[key]; !exists {
			modified = true
			continue
		}
		if _, exists := seen[key]; exists {
			modified = true
			continue
		}
		seen[key] = struct{}{}
		normalizedRequired = append(normalizedRequired, key)
	}

	sortedRequired := append([]string(nil), normalizedRequired...)
	sort.Strings(sortedRequired)
	if !modified && len(existingRequired) == len(sortedRequired) {
		matches := true
		for i, key := range sortedRequired {
			if strings.TrimSpace(firstNonEmptyString(existingRequired[i])) != key {
				matches = false
				break
			}
		}
		if matches {
			return false
		}
		modified = true
	}

	normalizedRequiredAny := make([]any, 0, len(sortedRequired))
	for _, key := range sortedRequired {
		normalizedRequiredAny = append(normalizedRequiredAny, key)
	}
	parameters["required"] = normalizedRequiredAny
	return modified
}

func hasCodexToolContinuationInput(input []any) bool {
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		typ := strings.TrimSpace(firstNonEmptyString(m["type"]))
		if typ == "item_reference" || isCodexToolCallItemType(typ) {
			return true
		}
		if strings.TrimSpace(firstNonEmptyString(m["role"])) == "tool" {
			return true
		}
	}
	return false
}

func filterCodexInput(input []any, rewriteToolContinuationIDs bool) []any {
	filtered, _ := filterCodexInputWithOptions(input, codexInputFilterOptions{
		rewriteToolContinuationIDs: rewriteToolContinuationIDs,
		dropItemReferences:         !rewriteToolContinuationIDs,
		dropNonToolItemIDs:         !rewriteToolContinuationIDs,
	})
	return filtered
}

func filterCodexInputWithOptions(input []any, opts codexInputFilterOptions) ([]any, bool) {
	filtered := make([]any, 0, len(input))
	modified := false
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		typ, _ := m["type"].(string)
		if typ == "reasoning" {
			if id, _ := m["id"].(string); strings.HasPrefix(strings.TrimSpace(id), "rs_") {
				modified = true
				continue
			}
		}

		// 仅修正真正的 tool/function call 标识，避免误改普通 message/reasoning id；
		// 若 item_reference 指向 legacy call_* 标识，则仅修正该引用本身。
		fixCallIDPrefix := func(id string) string {
			if id == "" || strings.HasPrefix(id, "fc") {
				return id
			}
			if strings.HasPrefix(id, "call_") {
				return "fc" + strings.TrimPrefix(id, "call_")
			}
			return "fc_" + id
		}

		if typ == "item_reference" {
			if opts.dropItemReferences {
				modified = true
				continue
			}
			newItem := m
			if id, ok := m["id"].(string); ok && opts.rewriteToolContinuationIDs && strings.HasPrefix(id, "call_") {
				newItem = make(map[string]any, len(m))
				for key, value := range m {
					newItem[key] = value
				}
				newItem["id"] = fixCallIDPrefix(id)
				modified = true
			}
			filtered = append(filtered, newItem)
			continue
		}

		newItem := m
		copied := false
		// 仅在需要修改字段时创建副本，避免直接改写原始输入。
		ensureCopy := func() {
			if copied {
				return
			}
			newItem = make(map[string]any, len(m))
			for key, value := range m {
				newItem[key] = value
			}
			copied = true
		}

		if isCodexToolCallItemType(typ) {
			callID, ok := m["call_id"].(string)
			if opts.rewriteToolContinuationIDs && (!ok || strings.TrimSpace(callID) == "") {
				if id, ok := m["id"].(string); ok && strings.TrimSpace(id) != "" {
					callID = id
					ensureCopy()
					newItem["call_id"] = callID
					modified = true
				}
			}

			if opts.rewriteToolContinuationIDs && callID != "" {
				fixedCallID := fixCallIDPrefix(callID)
				if fixedCallID != callID {
					ensureCopy()
					newItem["call_id"] = fixedCallID
					modified = true
				}
			}
		}

		if !isCodexToolCallItemType(typ) {
			if _, exists := newItem["call_id"]; exists {
				ensureCopy()
				delete(newItem, "call_id")
				modified = true
			}
		}

		if codexInputItemRequiresName(typ) {
			if strings.TrimSpace(firstNonEmptyString(m["name"])) == "" {
				name := firstNonEmptyString(m["tool_name"])
				if name == "" {
					if function, ok := m["function"].(map[string]any); ok {
						name = firstNonEmptyString(function["name"])
					}
				}
				if name == "" {
					name = "tool"
				}
				ensureCopy()
				newItem["name"] = name
				modified = true
			}
		}

		if opts.dropNonToolItemIDs && !isCodexToolCallItemType(typ) && !codexInputItemPreservesID(typ) {
			if _, exists := newItem["id"]; exists {
				ensureCopy()
				delete(newItem, "id")
				modified = true
			}
		}

		filtered = append(filtered, newItem)
	}
	if !modified && len(filtered) == len(input) {
		return input, false
	}
	return filtered, true
}

func isCodexToolCallItemType(typ string) bool {
	switch typ {
	case "function_call",
		"tool_call",
		"local_shell_call",
		"tool_search_call",
		"custom_tool_call",
		"mcp_tool_call",
		"function_call_output",
		"mcp_tool_call_output",
		"custom_tool_call_output",
		"tool_search_output":
		return true
	default:
		return false
	}
}

func codexInputItemRequiresName(typ string) bool {
	switch strings.TrimSpace(typ) {
	case "function_call", "custom_tool_call", "mcp_tool_call":
		return true
	default:
		return false
	}
}

func codexInputItemPreservesID(typ string) bool {
	switch strings.TrimSpace(typ) {
	case "image_generation_call", "web_search_call":
		return true
	default:
		return false
	}
}

func normalizeCodexTools(reqBody map[string]any) bool {
	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		return false
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false
	}

	modified := false
	validTools := make([]any, 0, len(tools))

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			// Keep unknown structure as-is to avoid breaking upstream behavior.
			validTools = append(validTools, tool)
			continue
		}

		toolType, _ := toolMap["type"].(string)
		toolType = strings.TrimSpace(toolType)
		if toolType != "function" {
			validTools = append(validTools, toolMap)
			continue
		}

		// OpenAI Responses-style tools use top-level name/parameters.
		if name, ok := toolMap["name"].(string); ok && strings.TrimSpace(name) != "" {
			if normalizeCodexFunctionToolParameters(toolMap) {
				modified = true
			}
			validTools = append(validTools, toolMap)
			continue
		}

		// ChatCompletions-style tools use {type:"function", function:{...}}.
		functionValue, hasFunction := toolMap["function"]
		function, ok := functionValue.(map[string]any)
		if !hasFunction || functionValue == nil || !ok || function == nil {
			// Drop invalid function tools.
			modified = true
			continue
		}

		if _, ok := toolMap["name"]; !ok {
			if name, ok := function["name"].(string); ok && strings.TrimSpace(name) != "" {
				toolMap["name"] = name
				modified = true
			}
		}
		if _, ok := toolMap["description"]; !ok {
			if desc, ok := function["description"].(string); ok && strings.TrimSpace(desc) != "" {
				toolMap["description"] = desc
				modified = true
			}
		}
		if _, ok := toolMap["parameters"]; !ok {
			if params, ok := function["parameters"]; ok {
				toolMap["parameters"] = params
				modified = true
			}
		}
		if _, ok := toolMap["strict"]; !ok {
			if strict, ok := function["strict"]; ok {
				toolMap["strict"] = strict
				modified = true
			}
		}
		if normalizeCodexFunctionToolParameters(toolMap) {
			modified = true
		}

		validTools = append(validTools, toolMap)
	}

	if modified {
		reqBody["tools"] = validTools
	}

	return modified
}

func normalizeCodexFunctionToolParameters(toolMap map[string]any) bool {
	if toolMap == nil || strings.TrimSpace(firstNonEmptyString(toolMap["type"])) != "function" {
		return false
	}

	paramsValue, hasParams := toolMap["parameters"]
	if !hasParams || paramsValue == nil {
		if function, ok := toolMap["function"].(map[string]any); ok && function != nil {
			if fnParams, ok := function["parameters"]; ok && fnParams != nil {
				toolMap["parameters"] = fnParams
				paramsValue = fnParams
				hasParams = true
			}
		}
	}

	if !hasParams || paramsValue == nil {
		toolMap["parameters"] = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		return true
	}

	paramsMap, ok := paramsValue.(map[string]any)
	if !ok {
		toolMap["parameters"] = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		return true
	}

	changed := false
	if typ, ok := paramsMap["type"].(string); !ok || strings.TrimSpace(typ) == "" {
		paramsMap["type"] = "object"
		changed = true
	}
	if _, ok := paramsMap["properties"]; !ok {
		paramsMap["properties"] = map[string]any{}
		changed = true
	}
	toolMap["parameters"] = paramsMap
	return changed
}
