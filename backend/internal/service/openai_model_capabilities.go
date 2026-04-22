package service

import "strings"

type CodexRequestProfile struct {
	RequestedModel      string
	UpstreamModel       string
	SupportsVerbosity   bool
	SupportsTemperature bool
	SupportsTopP        bool
}

func ResolveCodexRequestProfile(model string) CodexRequestProfile {
	normalized := normalizeCodexModel(model)
	return CodexRequestProfile{
		RequestedModel:      strings.TrimSpace(model),
		UpstreamModel:       normalized,
		SupportsVerbosity:   SupportsVerbosity(normalized),
		SupportsTemperature: false,
		SupportsTopP:        false,
	}
}
