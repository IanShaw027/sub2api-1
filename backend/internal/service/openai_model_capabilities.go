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
	normalized := normalizeCodexModel(trimmed)
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

func isOpenAICompatCapabilityFamily(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(normalized, "gpt-5") || strings.Contains(normalized, "codex")
}

func supportsCompatPromptCacheKey(normalizedModel string) bool {
	switch strings.TrimSpace(normalizedModel) {
	case "gpt-5.4", "gpt-5.3-codex":
		return true
	default:
		return false
	}
}
