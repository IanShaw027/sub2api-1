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

// isModelFallbackCandidateStatus 仅当上游状态码可能触发 model fallback 时返回 true。
// 目前只有 400/404 错误响应会走 fallback；成功/流式响应(2xx)一律不参与，故调用方
// 可据此在读取响应体之前提前放行，避免对成功流式响应做全量缓冲与截断。
func isModelFallbackCandidateStatus(statusCode int) bool {
	return statusCode == 400 || statusCode == 404
}

func isUpstreamModelUnavailableForFallback(statusCode int, body []byte) bool {
	upstreamMsg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	rawBody := strings.ToLower(strings.TrimSpace(string(body)))
	return isUpstreamModelUnavailableTextForFallback(statusCode, upstreamMsg, rawBody)
}

func isUpstreamModelUnavailableTextForFallback(statusCode int, parts ...string) bool {
	if !isModelFallbackCandidateStatus(statusCode) {
		return false
	}

	haystack := strings.ToLower(strings.TrimSpace(strings.Join(parts, " ")))
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
