package service

import (
	"context"
	"strings"
)

// resolveOpenAIForwardModel 解析 OpenAI 兼容转发使用的模型。
// defaultMappedModel 只服务于 /v1/messages 的 Claude 系列显式调度映射，
// 不作为普通 OpenAI 请求的未知模型兜底。
func resolveOpenAIForwardModel(account *Account, requestedModel, defaultMappedModel string) string {
	return resolveOpenAIForwardModelWithSelectedFallback(account, requestedModel, defaultMappedModel, "")
}

func resolveOpenAIForwardModelWithSelectedFallback(account *Account, requestedModel, defaultMappedModel, selectedFallbackModel string) string {
	return resolveOpenAIForwardModelWithSettingsAndSelectedFallback(context.Background(), nil, account, requestedModel, defaultMappedModel, selectedFallbackModel)
}

func resolveOpenAIForwardModelWithSettings(ctx context.Context, settingService *SettingService, account *Account, requestedModel, defaultMappedModel string) string {
	return resolveOpenAIForwardModelWithSettingsAndSelectedFallback(ctx, settingService, account, requestedModel, defaultMappedModel, "")
}

func resolveOpenAIForwardModelWithSettingsAndSelectedFallback(ctx context.Context, settingService *SettingService, account *Account, requestedModel, defaultMappedModel, selectedFallbackModel string) string {
	if selectedFallbackModel = strings.TrimSpace(selectedFallbackModel); selectedFallbackModel != "" {
		if account == nil {
			return selectedFallbackModel
		}
		return ResolveEffectiveMappedModel(ctx, settingService, account, selectedFallbackModel, false)
	}

	if account == nil {
		if defaultMappedModel != "" && claudeMessagesDispatchFamily(requestedModel) != "" {
			return defaultMappedModel
		}
		return requestedModel
	}

	routing := ResolveEffectiveModelRouting(ctx, settingService, account, requestedModel, false)
	mappedModel := routing.Model
	matched := routing.Matched
	if !matched && defaultMappedModel != "" && claudeMessagesDispatchFamily(requestedModel) != "" {
		return defaultMappedModel
	}
	return mappedModel
}

// OpenAI OAuth accounts without an explicit model mapping should not absorb
// model families that Codex cannot serve. Unknown aliases remain fail-open so
// channel-level mappings can still rewrite them after account selection.
var openAIOAuthForeignModelPrefixes = []string{
	"deepseek-",
	"glm-",
	"kimi-",
	"moonshot-",
	"qwen-",
	"qwen2-",
	"qwen2.5-",
	"qwen3-",
	"qwen4-",
	"qwq-",
	"minimax-",
	"gemini-",
	"gemma-",
	"grok-",
	"doubao-",
	"hunyuan-",
	"llama-",
	"llama2-",
	"llama3-",
	"meta-llama",
	"mistral-",
	"mixtral-",
	"baichuan-",
	"ernie-",
	"step-",
	"seed-",
	"yi-",
}

func isOpenAIOAuthServableModel(requestedModel string) bool {
	model := strings.ToLower(lastOpenAIModelSegment(requestedModel))
	if model == "" {
		return true
	}
	for _, prefix := range openAIOAuthForeignModelPrefixes {
		if strings.HasPrefix(model, prefix) {
			return false
		}
	}
	return true
}

func resolveOpenAICompactFallbackUpstreamModel(ctx context.Context, settingService *SettingService, account *Account, fallbackModel string) string {
	fallbackModel = strings.TrimSpace(fallbackModel)
	if fallbackModel == "" {
		return ""
	}
	return ResolveEffectiveMappedModel(ctx, settingService, account, fallbackModel, true)
}

// resolveOpenAICompactForwardModel determines the compact-only upstream model
// for /responses/compact requests. It never affects normal /responses traffic.
// When no compact-specific mapping matches, the input model is returned as-is.
func resolveOpenAICompactForwardModel(account *Account, model string) string {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" || account == nil {
		return trimmedModel
	}

	mappedModel, matched := account.ResolveCompactMappedModel(trimmedModel)
	if !matched {
		return trimmedModel
	}
	if trimmedMapped := strings.TrimSpace(mappedModel); trimmedMapped != "" {
		return trimmedMapped
	}
	return trimmedModel
}

// resolveOpenAICompactUpstreamModel first applies the account's normal model
// mapping, then applies the compact-only override. This keeps passthrough and
// non-passthrough compact routing aligned.
func resolveOpenAICompactUpstreamModel(account *Account, model string) string {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		return trimmedModel
	}
	if account != nil {
		trimmedModel = account.GetMappedModel(trimmedModel)
	}
	return resolveOpenAICompactForwardModel(account, trimmedModel)
}
