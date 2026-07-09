package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const (
	defaultOpenAIMessagesDispatchOpusMappedModel   = "gpt-5.4"
	defaultOpenAIMessagesDispatchSonnetMappedModel = "gpt-5.3-codex"
	defaultOpenAIMessagesDispatchHaikuMappedModel  = "gpt-5.4-mini"
)

func normalizeOpenAIMessagesDispatchMappedModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if normalized := canonicalizeOpenAIModelAliasSpelling(model); normalized != "" {
		return normalized
	}
	return model
}

func resolveOpenAIMessagesDispatchModel(cfg OpenAIMessagesDispatchModelConfig, requestedModel string) (string, bool) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return "", false
	}

	if mappedModel := strings.TrimSpace(cfg.ExactModelMappings[requestedModel]); mappedModel != "" {
		return mappedModel, true
	}

	switch claudeMessagesDispatchFamily(requestedModel) {
	case "opus":
		if mappedModel := strings.TrimSpace(cfg.OpusMappedModel); mappedModel != "" {
			return mappedModel, true
		}
		return defaultOpenAIMessagesDispatchOpusMappedModel, false
	case "sonnet":
		if mappedModel := strings.TrimSpace(cfg.SonnetMappedModel); mappedModel != "" {
			return mappedModel, true
		}
		return defaultOpenAIMessagesDispatchSonnetMappedModel, false
	case "haiku":
		if mappedModel := strings.TrimSpace(cfg.HaikuMappedModel); mappedModel != "" {
			return mappedModel, true
		}
		return defaultOpenAIMessagesDispatchHaikuMappedModel, false
	default:
		return "", false
	}
}

func normalizeOpenAIMessagesDispatchModelConfig(cfg OpenAIMessagesDispatchModelConfig) OpenAIMessagesDispatchModelConfig {
	out := OpenAIMessagesDispatchModelConfig{
		OpusMappedModel:   normalizeOpenAIMessagesDispatchMappedModel(cfg.OpusMappedModel),
		SonnetMappedModel: normalizeOpenAIMessagesDispatchMappedModel(cfg.SonnetMappedModel),
		HaikuMappedModel:  normalizeOpenAIMessagesDispatchMappedModel(cfg.HaikuMappedModel),
	}

	if len(cfg.ExactModelMappings) > 0 {
		out.ExactModelMappings = make(map[string]string, len(cfg.ExactModelMappings))
		for requestedModel, mappedModel := range cfg.ExactModelMappings {
			requestedModel = strings.TrimSpace(requestedModel)
			mappedModel = normalizeOpenAIMessagesDispatchMappedModel(mappedModel)
			if requestedModel == "" || mappedModel == "" {
				continue
			}
			out.ExactModelMappings[requestedModel] = mappedModel
		}
		if len(out.ExactModelMappings) == 0 {
			out.ExactModelMappings = nil
		}
	}

	return out
}

func claudeMessagesDispatchFamily(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(normalized, "claude") {
		return ""
	}
	switch {
	case strings.Contains(normalized, "opus"):
		return "opus"
	case strings.Contains(normalized, "sonnet"):
		return "sonnet"
	case strings.Contains(normalized, "haiku"):
		return "haiku"
	default:
		return ""
	}
}

func (g *Group) ResolveMessagesDispatchModel(requestedModel string) string {
	mappedModel, _ := g.ResolveMessagesDispatchModelWithSource(requestedModel)
	return mappedModel
}

func (g *Group) ResolveMessagesDispatchModelWithSource(requestedModel string) (string, bool) {
	if g == nil {
		return "", false
	}
	if g.Platform == PlatformGrok {
		requestedModel = strings.TrimSpace(requestedModel)
		// Native Grok models pass through without rewrite so clients can pick any grok-*.
		if xai.IsGrokModelID(requestedModel) {
			return "", false
		}
		// Claude Code default model names map onto the current Grok text default.
		// Intentionally fixed (not group-configurable): Grok groups always enable
		// /v1/messages and map all Claude families to DefaultTextModel. Per-family
		// overrides remain an OpenAI-group feature.
		if claudeMessagesDispatchFamily(requestedModel) != "" {
			return xai.DefaultTextModel, false
		}
		return "", false
	}
	cfg := normalizeOpenAIMessagesDispatchModelConfig(g.MessagesDispatchModelConfig)
	return resolveOpenAIMessagesDispatchModel(cfg, requestedModel)
}

func sanitizeGroupMessagesDispatchFields(g *Group) {
	if g == nil || g.Platform == PlatformOpenAI {
		return
	}
	g.AllowMessagesDispatch = false
	g.DefaultMappedModel = ""
	g.MessagesDispatchModelConfig = OpenAIMessagesDispatchModelConfig{}
}
