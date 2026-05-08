package service

import (
	"context"
	"strings"
)

func resolveConfiguredFallbackModel(ctx context.Context, settingService *SettingService, platform, currentModel string) string {
	if settingService == nil || !settingService.IsModelFallbackEnabled(ctx) {
		return ""
	}
	fallbackModel := strings.TrimSpace(settingService.GetFallbackModel(ctx, platform))
	if fallbackModel == "" {
		return ""
	}
	if strings.EqualFold(fallbackModel, strings.TrimSpace(currentModel)) {
		return ""
	}
	return fallbackModel
}

func isUpstreamModelUnavailableForFallback(statusCode int, body []byte) bool {
	if statusCode != 400 && statusCode != 404 {
		return false
	}

	upstreamMsg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	rawBody := strings.ToLower(strings.TrimSpace(string(body)))
	haystack := strings.TrimSpace(upstreamMsg + " " + rawBody)
	if haystack == "" {
		return false
	}

	explicitPhrases := []string{
		"model not found",
		"unknown model",
		"unsupported model",
		"model is not supported",
		"invalid model",
		"unrecognized model",
		"no such model",
	}
	for _, phrase := range explicitPhrases {
		if strings.Contains(haystack, phrase) {
			return true
		}
	}

	hasModelToken := strings.Contains(haystack, "model")
	if !hasModelToken {
		return false
	}
	if strings.Contains(haystack, "does not exist") {
		return true
	}
	if statusCode == 404 && strings.Contains(haystack, "not found") {
		return true
	}
	return false
}
