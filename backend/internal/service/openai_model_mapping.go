package service

import "strings"

// resolveOpenAIForwardModel 解析 OpenAI 兼容转发使用的模型。
// defaultMappedModel 只服务于 /v1/messages 的 Claude 系列显式调度映射，
// 不作为普通 OpenAI 请求的未知模型兜底。
func resolveOpenAIForwardModel(account *Account, requestedModel, defaultMappedModel string) string {
	return resolveOpenAIForwardModelWithSelectedFallback(account, requestedModel, defaultMappedModel, "")
}

func resolveOpenAIForwardModelWithSelectedFallback(account *Account, requestedModel, defaultMappedModel, selectedFallbackModel string) string {
	if selectedFallbackModel = strings.TrimSpace(selectedFallbackModel); selectedFallbackModel != "" {
		if account == nil {
			return selectedFallbackModel
		}
		mappedModel, _ := account.ResolveMappedModel(selectedFallbackModel)
		return mappedModel
	}

	if account == nil {
		if defaultMappedModel != "" && claudeMessagesDispatchFamily(requestedModel) != "" {
			return defaultMappedModel
		}
		return requestedModel
	}

	mappedModel, matched := account.ResolveMappedModel(requestedModel)
	if !matched && defaultMappedModel != "" && claudeMessagesDispatchFamily(requestedModel) != "" {
		return defaultMappedModel
	}
	return mappedModel
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
