package service

import (
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const compatPromptCacheKeyPrefix = "compat_cc_"
const compatAnthropicPromptCacheKeyPrefix = "compat_msg_"
const anthropicCachePromptCacheKeyPrefix = "anthropic-cache-"

func shouldAutoInjectPromptCacheKeyForCompat(model string) bool {
	if ResolveOpenAIModelCapabilities(model).SupportsCompatPromptCacheKey {
		return true
	}
	// Grok Claude Code / Codex bridges also benefit from stable session keys.
	return xai.IsGrokModelID(model)
}

func deriveCompatPromptCacheKey(req *apicompat.ChatCompletionsRequest, mappedModel string) string {
	if req == nil {
		return ""
	}

	normalizedModel := normalizeCodexModel(strings.TrimSpace(mappedModel))
	if normalizedModel == "" {
		normalizedModel = normalizeCodexModel(strings.TrimSpace(req.Model))
	}
	if normalizedModel == "" {
		normalizedModel = strings.TrimSpace(req.Model)
	}

	seedParts := []string{"model=" + normalizedModel}
	if req.ReasoningEffort != "" {
		seedParts = append(seedParts, "reasoning_effort="+strings.TrimSpace(req.ReasoningEffort))
	}
	if len(req.ToolChoice) > 0 {
		seedParts = append(seedParts, "tool_choice="+normalizeCompatSeedJSON(req.ToolChoice))
	}
	if len(req.Tools) > 0 {
		if raw, err := json.Marshal(req.Tools); err == nil {
			seedParts = append(seedParts, "tools="+normalizeCompatSeedJSON(raw))
		}
	}
	if len(req.Functions) > 0 {
		if raw, err := json.Marshal(req.Functions); err == nil {
			seedParts = append(seedParts, "functions="+normalizeCompatSeedJSON(raw))
		}
	}

	firstUserCaptured := false
	for _, msg := range req.Messages {
		switch strings.TrimSpace(msg.Role) {
		case "system":
			seedParts = append(seedParts, "system="+normalizeCompatSeedJSON(msg.Content))
		case "user":
			if !firstUserCaptured {
				seedParts = append(seedParts, "first_user="+normalizeCompatSeedJSON(msg.Content))
				firstUserCaptured = true
			}
		}
	}

	return compatPromptCacheKeyPrefix + hashSensitiveValueForLog(strings.Join(seedParts, "|"))
}

func deriveAnthropicCompatPromptCacheKey(req *apicompat.AnthropicRequest, mappedModel string) string {
	if req == nil {
		return ""
	}

	normalizedModel := normalizeCodexModel(strings.TrimSpace(mappedModel))
	if normalizedModel == "" {
		normalizedModel = normalizeCodexModel(strings.TrimSpace(req.Model))
	}
	if normalizedModel == "" {
		normalizedModel = strings.TrimSpace(req.Model)
	}

	seedParts := []string{"model=" + normalizedModel}
	if req.OutputConfig != nil && strings.TrimSpace(req.OutputConfig.Effort) != "" {
		seedParts = append(seedParts, "reasoning_effort="+strings.TrimSpace(req.OutputConfig.Effort))
	}
	if req.Thinking != nil && strings.TrimSpace(req.Thinking.Type) != "" {
		seedParts = append(seedParts, "thinking_type="+strings.TrimSpace(req.Thinking.Type))
	}
	if len(req.ToolChoice) > 0 {
		seedParts = append(seedParts, "tool_choice="+normalizeCompatSeedJSON(req.ToolChoice))
	}
	if len(req.Tools) > 0 {
		if raw, err := json.Marshal(req.Tools); err == nil {
			seedParts = append(seedParts, "tools="+normalizeCompatSeedJSON(raw))
		}
	}
	if anchors := collectAnthropicCacheAnchors(req); len(anchors) > 0 {
		seedParts = append(seedParts, anchors...)
		return anthropicCachePromptCacheKeyPrefix + hashSensitiveValueForLog(strings.Join(seedParts, "|"))
	}
	if len(req.System) > 0 {
		seedParts = append(seedParts, "system="+normalizeCompatSeedJSON(req.System))
	}

	firstUserCaptured := false
	for _, msg := range req.Messages {
		if strings.TrimSpace(msg.Role) != "user" || firstUserCaptured {
			continue
		}
		seedParts = append(seedParts, "first_user="+normalizeCompatSeedJSON(msg.Content))
		firstUserCaptured = true
	}

	return compatAnthropicPromptCacheKeyPrefix + hashSensitiveValueForLog(strings.Join(seedParts, "|"))
}

func normalizeCompatSeedJSON(v json.RawMessage) string {
	if len(v) == 0 {
		return ""
	}
	var tmp any
	if err := json.Unmarshal(v, &tmp); err != nil {
		return string(v)
	}
	out, err := json.Marshal(tmp)
	if err != nil {
		return string(v)
	}
	return string(out)
}

func collectAnthropicCacheAnchors(req *apicompat.AnthropicRequest) []string {
	if req == nil {
		return nil
	}
	var anchors []string
	if anchor := extractAnthropicCacheAnchor(req.System); anchor != "" {
		anchors = append(anchors, "system_anchor="+anchor)
	}
	for _, msg := range req.Messages {
		if anchor := extractAnthropicCacheAnchor(msg.Content); anchor != "" {
			anchors = append(anchors, "message_anchor="+strings.TrimSpace(msg.Role)+":"+anchor)
		}
	}
	return anchors
}

func extractAnthropicCacheAnchor(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	for _, item := range items {
		cacheControl, ok := item["cache_control"].(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(firstNonEmptyString(cacheControl["type"])) == "" {
			continue
		}
		if text := strings.TrimSpace(firstNonEmptyString(item["text"])); text != "" {
			return text
		}
	}
	return ""
}
