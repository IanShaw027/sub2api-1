package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

// codexToolNameInvalidChar 匹配 OpenAI Responses API 不允许出现在 function/tool name 中的字符。
// 上游对 tool 定义的 name 以及 function_call / custom_tool_call / mcp_tool_call 等 input item
// 的 name 强制要求满足 ^[a-zA-Z0-9_-]+$，否则返回 400 invalid_request_error。
var codexToolNameInvalidChar = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// sanitizeCodexToolName 将不合法字符替换为下划线，确保满足上游正则。
// 替换是确定性的：相同的原始 name 总是映射到相同的 sanitized name，
// 因而 tool 定义与对应的 function_call / tool_choice 引用之间的对应关系保持一致。
func sanitizeCodexToolName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return trimmed
	}
	return codexToolNameInvalidChar.ReplaceAllString(trimmed, "_")
}

var codexModelMap = map[string]string{
	"gpt-5.5":                    "gpt-5.5",
	"gpt-5.5-pro":                "gpt-5.5-pro",
	"codex-auto-review":          "codex-auto-review",
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
	rewriteToolContinuationIDs                bool
	dropItemReferences                        bool
	dropNonToolItemIDs                        bool
	dropOrphanFunctionCallOutputs             bool
	dropReasoningItems                        bool
	dropReasoningItemsWithoutEncryptedContent bool
}

type codexOAuthTransformOptions struct {
	SkipDefaultInstructions bool
	PreserveToolCallIDs     bool
	IsCompact               bool
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
	codexToolOutputMaxChars          = 10000000
)

func applyCodexOAuthTransform(reqBody map[string]any, isCodexCLI bool, isCompact bool) codexTransformResult {
	return applyCodexOAuthTransformWithInputModeAndFallbackReasonOptions(
		reqBody,
		isCodexCLI,
		isCompact,
		codexTransformInputModeStrict,
		"",
		codexOAuthTransformOptions{},
	)
}

func applyCodexOAuthTransformWithInputMode(
	reqBody map[string]any,
	isCodexCLI bool,
	isCompact bool,
	inputMode codexTransformInputMode,
) codexTransformResult {
	return applyCodexOAuthTransformWithInputModeAndOptions(
		reqBody,
		isCodexCLI,
		isCompact,
		inputMode,
		codexOAuthTransformOptions{},
	)
}

func applyCodexOAuthTransformWithInputModeAndOptions(
	reqBody map[string]any,
	isCodexCLI bool,
	isCompact bool,
	inputMode codexTransformInputMode,
	opts codexOAuthTransformOptions,
) codexTransformResult {
	return applyCodexOAuthTransformWithInputModeAndFallbackReasonOptions(
		reqBody,
		isCodexCLI,
		isCompact,
		inputMode,
		"",
		opts,
	)
}

func applyCodexOAuthTransformWithOptions(reqBody map[string]any, opts codexOAuthTransformOptions) codexTransformResult {
	result := applyCodexOAuthTransformWithInputModeAndOptions(reqBody, false, false, codexTransformInputModeStrict, opts)
	if opts.SkipDefaultInstructions {
		if removeEmbeddedDefaultInstructions(reqBody) {
			result.Modified = true
			result.Observability.DefaultInstructionsApplied = false
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
	return applyCodexOAuthTransformWithInputModeAndFallbackReasonOptions(
		reqBody,
		isCodexCLI,
		isCompact,
		inputMode,
		fallbackReason,
		codexOAuthTransformOptions{},
	)
}

func applyCodexOAuthTransformWithInputModeAndFallbackReasonOptions(
	reqBody map[string]any,
	isCodexCLI bool,
	isCompact bool,
	inputMode codexTransformInputMode,
	fallbackReason string,
	opts codexOAuthTransformOptions,
) codexTransformResult {
	opts.IsCompact = isCompact
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

	// 请求带 reasoning 时补齐 include:["reasoning.encrypted_content"]，与真实 Codex 对齐
	// （compact 端点形态不同，单独处理，此处跳过）。
	if !opts.IsCompact && ensureCodexReasoningInclude(reqBody) {
		result.Modified = true
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

	if !isCompact {
		if normalizeCodexTools(reqBody) {
			result.Modified = true
			result.Observability.ToolsNormalized = true
		}
		if normalizeCodexToolChoice(reqBody) {
			result.Modified = true
			result.Observability.ToolChoiceNormalized = true
		}
	}

	if v, ok := reqBody["prompt_cache_key"].(string); ok {
		result.PromptCacheKey = strings.TrimSpace(v)
	}

	// ChatGPT internal Codex endpoint does not accept role:"system".
	// Keep the guidance in input as developer for Responses JSON mode, and
	// also mirror it into instructions because Codex OAuth requires it.
	if extractSystemMessagesFromInput(reqBody) {
		result.Modified = true
		result.Observability.SystemMessagesExtracted = true
	}

	// instructions 处理逻辑：根据是否是 Codex CLI 分别调用不同方法
	if !opts.SkipDefaultInstructions && applyInstructions(reqBody, isCodexCLI) {
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
	// gpt-5.3-codex-spark rejects the image_generation tool upstream (HTTP 400,
	// param=tools); Codex CLI advertises it by default, so strip it for spark.
	if isCodexSparkModel(normalizedModel) && stripCodexSparkImageGenerationTools(reqBody) {
		result.Modified = true
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
		case "call_id":
			filterOptions.dropNonToolItemIDs = true
			filterOptions.dropOrphanFunctionCallOutputs = true
		case "input_schema":
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
		if name != "" {
			name = sanitizeCodexToolName(name)
		}
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

func normalizeOpenAIResponsesMessageContentPartTypes(input []any) ([]any, bool) {
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
		role := strings.TrimSpace(firstNonEmptyString(m["role"]))
		if role == "" {
			role = "user"
		}
		parts, ok := m["content"].([]any)
		if !ok {
			normalized = append(normalized, item)
			continue
		}

		targetType := "input_text"
		if role == "assistant" {
			targetType = "output_text"
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
			partType := strings.TrimSpace(firstNonEmptyString(part["type"]))
			switch role {
			case "assistant":
				if partType == "refusal" || partType == "output_text" {
					continue
				}
				if partType != "" && partType != "text" && partType != "input_text" {
					continue
				}
			default:
				if partType == "input_text" {
					continue
				}
				if partType != "" && partType != "text" && partType != "output_text" {
					continue
				}
			}

			ensureItemCopy()
			newPart := make(map[string]any, len(part))
			for key, value := range part {
				newPart[key] = value
			}
			newPart["type"] = targetType
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

	if strings.Contains(normalized, "gpt-5.5-pro") || strings.Contains(normalized, "gpt 5.5 pro") {
		return "gpt-5.5-pro"
	}
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

// stripCodexSparkImageGenerationTools removes image_generation tool entries from
// reqBody["tools"]. gpt-5.3-codex-spark rejects that tool upstream with HTTP 400
// (invalid_request_error, param=tools), and Codex CLI advertises it by default.
func stripCodexSparkImageGenerationTools(reqBody map[string]any) bool {
	return stripOpenAIImageGenerationTools(reqBody)
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
		if size := strings.TrimSpace(firstNonEmptyString(toolMap["size"])); size != "" {
			normalizedSize := normalizeOpenAIImageSize(size)
			if normalizedSize != size {
				toolMap["size"] = normalizedSize
				modified = true
			}
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

func ensureOpenAIResponsesImageGenerationToolChoiceAuto(reqBody map[string]any) bool {
	if len(reqBody) == 0 || !hasOpenAIImageGenerationTool(reqBody) {
		return false
	}
	if isCodexSparkModel(firstNonEmptyString(reqBody["model"])) {
		return false
	}
	if rawChoice, ok := reqBody["tool_choice"]; ok {
		switch choice := rawChoice.(type) {
		case string:
			if strings.TrimSpace(choice) == "image_generation" {
				reqBody["tool_choice"] = "auto"
				return true
			}
		case map[string]any:
			if strings.TrimSpace(firstNonEmptyString(choice["type"])) == "image_generation" {
				reqBody["tool_choice"] = "auto"
				return true
			}
		}
		return false
	}
	reqBody["tool_choice"] = "auto"
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

// extractSystemMessagesFromInput scans input for role=="system", maps those
// items to developer, and mirrors their text into reqBody["instructions"].
// It preserves the input items so Responses JSON mode can still see JSON
// instructions in input messages.
func extractSystemMessagesFromInput(reqBody map[string]any) bool {
	input, ok := reqBody["input"].([]any)
	if !ok || len(input) == 0 {
		return false
	}

	var systemTexts []string
	modified := false
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if role, _ := m["role"].(string); role != "system" {
			continue
		}
		m["role"] = "developer"
		modified = true
		if text := extractTextFromContent(m["content"]); text != "" {
			systemTexts = append(systemTexts, text)
		}
	}

	if len(systemTexts) == 0 {
		return modified
	}

	extracted := strings.Join(systemTexts, "\n\n")
	if existing, ok := reqBody["instructions"].(string); ok && strings.TrimSpace(existing) != "" {
		reqBody["instructions"] = extracted + "\n\n" + existing
	} else {
		reqBody["instructions"] = extracted
	}
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
	reqBody["instructions"] = embeddedDefaultInstructions()
	return true
}

func embeddedDefaultInstructions() string {
	instructions := strings.TrimSpace(openai.DefaultInstructions)
	if instructions == "" {
		return "You are a helpful coding assistant."
	}
	return instructions
}

func removeEmbeddedDefaultInstructions(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	instructions, ok := reqBody["instructions"].(string)
	if !ok {
		return false
	}
	if strings.TrimSpace(instructions) != embeddedDefaultInstructions() {
		return false
	}
	delete(reqBody, "instructions")
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
		if !ok {
			continue
		}
		if stripOpenAIToolSchemaLookaroundPatterns(toolMap) {
			modified = true
		}
		if !openAIStrictFunctionToolEnabled(toolMap) {
			continue
		}
		if normalizeOpenAIStrictFunctionParameters(toolMap) {
			modified = true
		}
	}

	return modified
}

func normalizeOpenAIToolSchemaLookaroundPatterns(reqBody map[string]any) bool {
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
		if !ok {
			continue
		}
		if stripOpenAIToolSchemaLookaroundPatterns(toolMap) {
			modified = true
		}
	}
	return modified
}

func stripOpenAIToolSchemaLookaroundPatterns(toolMap map[string]any) bool {
	if toolMap == nil || strings.TrimSpace(firstNonEmptyString(toolMap["type"])) != "function" {
		return false
	}

	modified := false
	if params, ok := toolMap["parameters"].(map[string]any); ok {
		if stripOpenAIJSONSchemaLookaroundPatterns(params) {
			modified = true
		}
	}
	if function, ok := toolMap["function"].(map[string]any); ok && function != nil {
		if params, ok := function["parameters"].(map[string]any); ok {
			if stripOpenAIJSONSchemaLookaroundPatterns(params) {
				modified = true
			}
		}
	}
	return modified
}

func stripOpenAIJSONSchemaLookaroundPatterns(schema map[string]any) bool {
	if schema == nil {
		return false
	}

	modified := false
	if pattern, ok := schema["pattern"].(string); ok && openAIRegexPatternUsesLookaround(pattern) {
		delete(schema, "pattern")
		modified = true
	}

	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, rawProperty := range properties {
			propertySchema, ok := rawProperty.(map[string]any)
			if !ok {
				continue
			}
			if stripOpenAIJSONSchemaLookaroundPatterns(propertySchema) {
				modified = true
			}
		}
	}

	switch items := schema["items"].(type) {
	case map[string]any:
		if stripOpenAIJSONSchemaLookaroundPatterns(items) {
			modified = true
		}
	case []any:
		for _, rawItem := range items {
			itemSchema, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			if stripOpenAIJSONSchemaLookaroundPatterns(itemSchema) {
				modified = true
			}
		}
	}

	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		rawSchemas, ok := schema[key].([]any)
		if !ok {
			continue
		}
		for _, rawSchema := range rawSchemas {
			childSchema, ok := rawSchema.(map[string]any)
			if !ok {
				continue
			}
			if stripOpenAIJSONSchemaLookaroundPatterns(childSchema) {
				modified = true
			}
		}
	}

	return modified
}

func openAIRegexPatternUsesLookaround(pattern string) bool {
	return strings.Contains(pattern, "(?=") ||
		strings.Contains(pattern, "(?!") ||
		strings.Contains(pattern, "(?<=") ||
		strings.Contains(pattern, "(?<!")
}

func normalizeOpenAIResponseFormatSchemas(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}

	modified := false

	if text, ok := reqBody["text"].(map[string]any); ok {
		if format, ok := text["format"].(map[string]any); ok {
			if strings.TrimSpace(firstNonEmptyString(format["type"])) == "json_schema" {
				if schema, ok := format["schema"].(map[string]any); ok {
					if normalizeOpenAIResponseJSONSchema(schema) {
						modified = true
					}
				}
			}
		}
	}

	if responseFormat, ok := reqBody["response_format"].(map[string]any); ok {
		if strings.TrimSpace(firstNonEmptyString(responseFormat["type"])) == "json_schema" {
			if schema, ok := responseFormat["schema"].(map[string]any); ok {
				if normalizeOpenAIResponseJSONSchema(schema) {
					modified = true
				}
			}
			if jsonSchema, ok := responseFormat["json_schema"].(map[string]any); ok {
				if schema, ok := jsonSchema["schema"].(map[string]any); ok {
					if normalizeOpenAIResponseJSONSchema(schema) {
						modified = true
					}
				}
			}
		}
	}

	return modified
}

func normalizeOpenAIResponseJSONSchema(schema map[string]any) bool {
	if schema == nil {
		return false
	}

	modified := false

	if _, exists := schema["required"]; exists {
		if _, ok := schema["required"].([]any); !ok {
			schema["required"] = []any{}
			modified = true
		}
	}

	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, rawProperty := range properties {
			propertySchema, ok := rawProperty.(map[string]any)
			if !ok {
				continue
			}
			if normalizeOpenAIResponseJSONSchema(propertySchema) {
				modified = true
			}
		}

		required := make([]any, 0, len(properties))
		for _, key := range sortedOpenAIResponseSchemaKeys(properties) {
			required = append(required, key)
		}
		if !openAIResponseSchemaRequiredEquals(schema["required"], required) {
			schema["required"] = required
			modified = true
		}
		if additionalProperties, ok := schema["additionalProperties"].(bool); !ok || additionalProperties {
			schema["additionalProperties"] = false
			modified = true
		}
	}

	isArraySchema := strings.TrimSpace(firstNonEmptyString(schema["type"])) == "array"
	if isArraySchema {
		rawItems, exists := schema["items"]
		if !exists || rawItems == nil {
			schema["items"] = map[string]any{}
			modified = true
		}
	}
	switch items := schema["items"].(type) {
	case map[string]any:
		if normalizeOpenAIResponseJSONSchema(items) {
			modified = true
		}
	case []any:
		for _, rawItem := range items {
			itemSchema, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			if normalizeOpenAIResponseJSONSchema(itemSchema) {
				modified = true
			}
		}
	default:
		if isArraySchema {
			schema["items"] = map[string]any{}
			modified = true
		}
	}

	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		rawSchemas, ok := schema[key].([]any)
		if !ok {
			continue
		}
		for _, rawSchema := range rawSchemas {
			childSchema, ok := rawSchema.(map[string]any)
			if !ok {
				continue
			}
			if normalizeOpenAIResponseJSONSchema(childSchema) {
				modified = true
			}
		}
	}

	return modified
}

func normalizeOpenAIClientJSONSchemaShape(schema map[string]any) bool {
	if schema == nil {
		return false
	}

	modified := false
	if _, exists := schema["required"]; exists {
		if _, ok := schema["required"].([]any); !ok {
			schema["required"] = []any{}
			modified = true
		}
	}

	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, rawProperty := range properties {
			propertySchema, ok := rawProperty.(map[string]any)
			if !ok {
				continue
			}
			if normalizeOpenAIClientJSONSchemaShape(propertySchema) {
				modified = true
			}
		}
	}

	isArraySchema := strings.TrimSpace(firstNonEmptyString(schema["type"])) == "array"
	if isArraySchema {
		rawItems, exists := schema["items"]
		if !exists || rawItems == nil {
			schema["items"] = map[string]any{}
			modified = true
		}
	}
	switch items := schema["items"].(type) {
	case map[string]any:
		if normalizeOpenAIClientJSONSchemaShape(items) {
			modified = true
		}
	case []any:
		for _, rawItem := range items {
			itemSchema, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			if normalizeOpenAIClientJSONSchemaShape(itemSchema) {
				modified = true
			}
		}
	default:
		if isArraySchema {
			schema["items"] = map[string]any{}
			modified = true
		}
	}

	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		rawSchemas, ok := schema[key].([]any)
		if !ok {
			continue
		}
		for _, rawSchema := range rawSchemas {
			childSchema, ok := rawSchema.(map[string]any)
			if !ok {
				continue
			}
			if normalizeOpenAIClientJSONSchemaShape(childSchema) {
				modified = true
			}
		}
	}

	return modified
}

func sortedOpenAIResponseSchemaKeys(properties map[string]any) []string {
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func openAIResponseSchemaRequiredEquals(rawRequired any, expected []any) bool {
	required, ok := rawRequired.([]any)
	if !ok || len(required) != len(expected) {
		return false
	}
	for i := range expected {
		if strings.TrimSpace(firstNonEmptyString(required[i])) != strings.TrimSpace(firstNonEmptyString(expected[i])) {
			return false
		}
	}
	return true
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
			_, hasEncryptedContent := m["encrypted_content"]
			if opts.dropReasoningItems || (opts.dropReasoningItemsWithoutEncryptedContent && !hasEncryptedContent) {
				modified = true
				continue
			}
			newItem := make(map[string]any, len(m))
			for key, value := range m {
				if key == "id" {
					modified = true
					continue
				}
				newItem[key] = value
			}
			if summary, ok := newItem["summary"]; !ok || summary == nil {
				newItem["summary"] = []any{}
				modified = true
			}
			filtered = append(filtered, newItem)
			continue
		}

		// 仅修正真正的 tool/function call 标识，避免误改普通 message/reasoning id；
		// 若 item_reference 指向 legacy call_* 标识，则仅修正该引用本身。
		fixCallIDPrefix := func(id string) string {
			if id == "" || strings.HasPrefix(id, "fc") {
				return id
			}
			if strings.HasPrefix(id, "call_") {
				return "fc_" + strings.TrimPrefix(id, "call_")
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

			if isCodexToolCallOutputItemType(typ) {
				if output, ok := m["output"].(string); ok {
					if truncated, didTruncate := truncateCodexToolOutput(output); didTruncate {
						ensureCopy()
						newItem["output"] = truncated
						modified = true
					}
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
			rawName := strings.TrimSpace(firstNonEmptyString(m["name"]))
			if rawName == "" {
				rawName = strings.TrimSpace(firstNonEmptyString(m["tool_name"]))
				if rawName == "" {
					if function, ok := m["function"].(map[string]any); ok {
						rawName = strings.TrimSpace(firstNonEmptyString(function["name"]))
					}
				}
				if rawName == "" {
					rawName = "tool"
				}
			}
			sanitizedName := sanitizeCodexToolName(rawName)
			if sanitizedName == "" {
				sanitizedName = "tool"
			}
			currentName, _ := m["name"].(string)
			if currentName != sanitizedName {
				ensureCopy()
				newItem["name"] = sanitizedName
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

		if sanitizedItem, sanitized := sanitizeCodexInputItem(newItem); sanitized {
			newItem = sanitizedItem
			modified = true
		}

		filtered = append(filtered, newItem)
	}
	if opts.dropOrphanFunctionCallOutputs {
		next, dropped := dropOrphanFunctionCallOutputs(filtered)
		if dropped {
			filtered = next
			modified = true
		}
	}
	if !modified && len(filtered) == len(input) {
		return input, false
	}
	return filtered, true
}

// dropOrphanFunctionCallOutputs removes function_call_output items whose call_id
// has no matching function_call (or other tool-call) item earlier in the same
// input array. Sending an orphan function_call_output causes the upstream to
// reject the request with "No tool call found for function call output with
// call_id ...". The orphan typically arises when a client persists transcript
// state but the matching function_call has already been pruned (e.g. because
// it lived on a server-side previous_response chain that is no longer reachable).
func dropOrphanFunctionCallOutputs(input []any) ([]any, bool) {
	if len(input) == 0 {
		return input, false
	}
	isCallSourceType := func(typ string) bool {
		switch typ {
		case "function_call",
			"tool_call",
			"local_shell_call",
			"tool_search_call",
			"custom_tool_call",
			"mcp_tool_call",
			"item_reference":
			return true
		default:
			return false
		}
	}
	seenCallIDs := make(map[string]struct{}, len(input))
	filtered := make([]any, 0, len(input))
	dropped := false
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		typ, _ := m["type"].(string)
		if isCodexToolCallOutputItemType(typ) {
			callID, _ := m["call_id"].(string)
			callID = strings.TrimSpace(callID)
			if callID == "" {
				dropped = true
				continue
			}
			if _, matched := seenCallIDs[callID]; !matched {
				dropped = true
				continue
			}
			filtered = append(filtered, item)
			continue
		}
		filtered = append(filtered, item)
		if !isCallSourceType(typ) {
			continue
		}
		var ref string
		if typ == "item_reference" {
			ref, _ = m["id"].(string)
		} else {
			ref = toolCallContextIDFromMap(m)
		}
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		seenCallIDs[ref] = struct{}{}
	}
	if !dropped {
		return input, false
	}
	return filtered, true
}

var codexMessageInputAllowedFields = map[string]struct{}{
	"type":    {},
	"role":    {},
	"content": {},
	"id":      {},
}

func sanitizeCodexInputItem(item map[string]any) (map[string]any, bool) {
	if len(item) == 0 {
		return item, false
	}

	if strings.TrimSpace(firstNonEmptyString(item["type"])) == "message" {
		sanitized := make(map[string]any, len(item))
		modified := false
		for key, value := range item {
			if value == nil {
				modified = true
				continue
			}
			if _, ok := codexMessageInputAllowedFields[key]; !ok {
				modified = true
				continue
			}
			sanitized[key] = value
		}
		if !modified {
			return item, false
		}
		return sanitized, true
	}

	var sanitized map[string]any
	modified := false
	for key, value := range item {
		if value != nil {
			continue
		}
		if sanitized == nil {
			sanitized = make(map[string]any, len(item)-1)
			for existingKey, existingValue := range item {
				sanitized[existingKey] = existingValue
			}
		}
		delete(sanitized, key)
		modified = true
	}
	if !modified {
		return item, false
	}
	return sanitized, true
}

func truncateOpenAIResponsesToolOutputs(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	input, ok := reqBody["input"].([]any)
	if !ok || len(input) == 0 {
		return false
	}
	modified := false
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		itemType := strings.TrimSpace(firstNonEmptyString(m["type"]))
		if isCodexToolCallOutputItemType(itemType) {
			if output, ok := m["output"].(string); ok {
				if truncated, didTruncate := truncateCodexToolOutput(output); didTruncate {
					m["output"] = truncated
					modified = true
				}
			}
		}
		if truncateOpenAIResponsesMessageContentText(m) {
			modified = true
		}
	}
	return modified
}

func needsReqBodyForToolOutputTruncation(body []byte) bool {
	if len(body) <= codexToolOutputMaxChars {
		return false
	}
	if bytes.Contains(body, []byte(`"text"`)) && bytes.Contains(body, []byte(`"content"`)) {
		return true
	}
	for _, itemType := range []string{
		"function_call_output",
		"tool_search_output",
		"custom_tool_call_output",
		"mcp_tool_call_output",
	} {
		if bytes.Contains(body, []byte(itemType)) {
			return true
		}
	}
	return false
}

func truncateOpenAIResponsesMessageContentText(item map[string]any) bool {
	if item == nil || strings.TrimSpace(firstNonEmptyString(item["type"])) != "message" {
		return false
	}
	parts, ok := item["content"].([]any)
	if !ok || len(parts) == 0 {
		return false
	}
	modified := false
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		text, ok := part["text"].(string)
		if !ok {
			continue
		}
		truncated, didTruncate := truncateCodexToolOutput(text)
		if !didTruncate {
			continue
		}
		part["text"] = truncated
		modified = true
	}
	return modified
}

func truncateCodexToolOutput(output string) (string, bool) {
	if codexToolOutputMaxChars <= 0 {
		return "", output != ""
	}
	if len(output) <= codexToolOutputMaxChars {
		return output, false
	}
	chars := 0
	for idx := range output {
		if chars == codexToolOutputMaxChars {
			return output[:idx], true
		}
		chars++
	}
	return output, false
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
	sanitizeToolName := func(toolMap map[string]any) {
		if toolMap == nil {
			return
		}
		if name, ok := toolMap["name"].(string); ok {
			if sanitized := sanitizeCodexToolName(name); sanitized != "" && sanitized != name {
				toolMap["name"] = sanitized
				modified = true
			}
		}
		if function, ok := toolMap["function"].(map[string]any); ok && function != nil {
			if name, ok := function["name"].(string); ok {
				if sanitized := sanitizeCodexToolName(name); sanitized != "" && sanitized != name {
					function["name"] = sanitized
					modified = true
				}
			}
		}
	}
	ensureFunctionShape := func(toolMap map[string]any) {
		if toolMap == nil || strings.TrimSpace(firstNonEmptyString(toolMap["type"])) != "function" {
			return
		}
		function, _ := toolMap["function"].(map[string]any)
		if function == nil {
			function = map[string]any{}
			toolMap["function"] = function
			modified = true
		}
		if name := strings.TrimSpace(firstNonEmptyString(toolMap["name"])); name != "" && strings.TrimSpace(firstNonEmptyString(function["name"])) == "" {
			function["name"] = name
			modified = true
		}
		if description := strings.TrimSpace(firstNonEmptyString(toolMap["description"])); description != "" && strings.TrimSpace(firstNonEmptyString(function["description"])) == "" {
			function["description"] = description
			modified = true
		}
		if _, ok := function["parameters"]; !ok {
			if params, ok := toolMap["parameters"]; ok && params != nil {
				function["parameters"] = params
				modified = true
			}
		}
		if _, ok := function["strict"]; !ok {
			if strict, ok := toolMap["strict"]; ok {
				function["strict"] = strict
				modified = true
			}
		}
	}

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
			ensureFunctionShape(toolMap)
			sanitizeToolName(toolMap)
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
		ensureFunctionShape(toolMap)
		sanitizeToolName(toolMap)

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
	if normalizeOpenAIClientJSONSchemaShape(paramsMap) {
		changed = true
	}
	toolMap["parameters"] = paramsMap
	return changed
}

// defaultCodexSynthInstructions 返回合成路径在 instructions 为空时应填入的默认提示词。
//
// 按 model 选择真实 Codex CLI 的 base instructions；若内嵌 prompt 意外为空，
// 回退到最小占位符以保证字段非空。
func defaultCodexSynthInstructions(model string) string {
	if instructions := strings.TrimSpace(openai.CodexBaseInstructionsForModel(model)); instructions != "" {
		return instructions
	}
	return "You are a helpful coding assistant."
}

// ensureCodexReasoningInclude 在请求带 reasoning 时补齐 include:["reasoning.encrypted_content"]。
//
// 真实 Codex 在 reasoning 存在时总会请求加密推理内容（ChatGPT/store=false 场景下用于上下文回放）。
// 该函数为加法式、幂等：仅在 include 缺失或未包含该项时追加；对非数组的异常 include 不做破坏性改写。
func ensureCodexReasoningInclude(reqBody map[string]any) bool {
	reasoning, ok := reqBody["reasoning"].(map[string]any)
	if !ok || len(reasoning) == 0 {
		return false
	}
	const encrypted = "reasoning.encrypted_content"
	switch existing := reqBody["include"].(type) {
	case nil:
		reqBody["include"] = []any{encrypted}
		return true
	case []any:
		for _, v := range existing {
			if s, ok := v.(string); ok && s == encrypted {
				return false
			}
		}
		reqBody["include"] = append(existing, encrypted)
		return true
	default:
		return false
	}
}

// applyCodexClientMetadata 在请求体补齐 client_metadata["x-codex-installation-id"]，
// 取值为账号真实的 openai_device_id。
func applyCodexClientMetadata(reqBody map[string]any, account *Account) bool {
	if account == nil {
		return false
	}
	deviceID := strings.TrimSpace(account.GetOpenAIDeviceID())
	if deviceID == "" {
		return false
	}
	const key = "x-codex-installation-id"
	switch existing := reqBody["client_metadata"].(type) {
	case map[string]any:
		if v, ok := existing[key].(string); ok && strings.TrimSpace(v) != "" {
			return false
		}
		existing[key] = deviceID
		reqBody["client_metadata"] = existing
		return true
	case map[string]string:
		if strings.TrimSpace(existing[key]) != "" {
			return false
		}
		next := make(map[string]any, len(existing)+1)
		for k, v := range existing {
			next[k] = v
		}
		next[key] = deviceID
		reqBody["client_metadata"] = next
		return true
	case nil:
		reqBody["client_metadata"] = map[string]any{key: deviceID}
		return true
	default:
		return false
	}
}
