package service

import "strings"

type OpenAIModelCapabilities struct {
	RequestedModel               string
	UpstreamModel                string
	IsCompatFamily               bool
	SupportsVerbosity            bool
	SupportsTemperature          bool
	SupportsTopP                 bool
	SupportsCompatPromptCacheKey bool
}

type CodexRequestProfile struct {
	RequestedModel      string
	UpstreamModel       string
	SupportsVerbosity   bool
	SupportsTemperature bool
	SupportsTopP        bool
}

func ResolveOpenAIModelCapabilities(model string) OpenAIModelCapabilities {
	trimmed := strings.TrimSpace(model)
	normalized := resolveCompatCapabilityUpstreamModel(trimmed)
	isCompatFamily := isOpenAICompatCapabilityFamily(trimmed)

	return OpenAIModelCapabilities{
		RequestedModel:               trimmed,
		UpstreamModel:                normalized,
		IsCompatFamily:               isCompatFamily,
		SupportsVerbosity:            SupportsVerbosity(normalized),
		SupportsTemperature:          false,
		SupportsTopP:                 false,
		SupportsCompatPromptCacheKey: isCompatFamily && supportsCompatPromptCacheKey(normalized),
	}
}

func ResolveCodexRequestProfile(model string) CodexRequestProfile {
	caps := ResolveOpenAIModelCapabilities(model)
	return CodexRequestProfile{
		RequestedModel:      caps.RequestedModel,
		UpstreamModel:       caps.UpstreamModel,
		SupportsVerbosity:   caps.SupportsVerbosity,
		SupportsTemperature: caps.SupportsTemperature,
		SupportsTopP:        caps.SupportsTopP,
	}
}

func resolveCompatCapabilityUpstreamModel(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "gpt-5.4"
	}
	if isOpenAIImageGenerationModel(trimmed) {
		return trimmed
	}
	if canonical := canonicalizeOpenAIModelAliasSpelling(trimmed); canonical != "" {
		switch canonical {
		case "gpt-5.1", "gpt-5.1-none", "gpt-5.1-low", "gpt-5.1-medium", "gpt-5.1-high", "gpt-5.1-chat-latest":
			return "gpt-5.1"
		case "gpt-5.1-codex", "gpt-5.1-codex-low", "gpt-5.1-codex-medium", "gpt-5.1-codex-high":
			return "gpt-5.1-codex"
		case "gpt-5.1-codex-max", "gpt-5.1-codex-max-low", "gpt-5.1-codex-max-medium", "gpt-5.1-codex-max-high", "gpt-5.1-codex-max-xhigh":
			return "gpt-5.1-codex-max"
		case "gpt-5.1-codex-mini", "gpt-5.1-codex-mini-medium", "gpt-5.1-codex-mini-high":
			return "gpt-5.1-codex-mini"
		case "gpt-5.2-codex", "gpt-5.2-codex-low", "gpt-5.2-codex-medium", "gpt-5.2-codex-high", "gpt-5.2-codex-xhigh":
			return "gpt-5.2-codex"
		case "codex-mini-latest", "gpt-5-codex-mini", "gpt-5-codex-mini-medium", "gpt-5-codex-mini-high":
			return "gpt-5.1-codex-mini"
		case "gpt-5-codex":
			return "gpt-5.1-codex"
		case "gpt-5", "gpt-5-mini", "gpt-5-nano":
			return "gpt-5.1"
		}
	}
	return normalizeCodexModel(trimmed)
}

func isOpenAICompatCapabilityFamily(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(normalized, "gpt-5") || strings.Contains(normalized, "codex")
}

func supportsCompatPromptCacheKey(normalizedModel string) bool {
	normalized := strings.ToLower(strings.TrimSpace(normalizedModel))
	if normalized == "" {
		return false
	}
	return strings.HasPrefix(normalized, "gpt-5") || strings.Contains(normalized, "codex")
}
