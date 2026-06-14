package service

import (
	"context"
	"strings"
)

type EffectiveModelRoutingResult struct {
	Model     string
	Supported bool
	Matched   bool
	Source    string
}

func ResolveEffectiveModelRouting(ctx context.Context, settingService *SettingService, account *Account, requestedModel string, compact bool) EffectiveModelRoutingResult {
	requestedModel = strings.TrimSpace(requestedModel)
	result := EffectiveModelRoutingResult{
		Model:     requestedModel,
		Supported: true,
		Source:    "none",
	}
	if requestedModel == "" {
		return result
	}

	if accountHasExplicitModelRouting(account, compact) {
		mappedModel, matched := account.ResolveMappedModel(requestedModel)
		result.Model = strings.TrimSpace(mappedModel)
		result.Matched = matched
		result.Source = "account"
		result.Supported = account.IsModelSupported(requestedModel)
		if compact && result.Supported {
			if compactModel, compactMatched := account.ResolveCompactMappedModel(result.Model); compactMatched {
				if trimmed := strings.TrimSpace(compactModel); trimmed != "" {
					result.Model = trimmed
				}
				result.Matched = true
			}
		}
		if result.Model == "" {
			result.Model = requestedModel
		}
		return result
	}

	if account != nil {
		result.Model = strings.TrimSpace(account.GetMappedModel(requestedModel))
		if result.Model == "" {
			result.Model = requestedModel
		}
		result.Supported = account.IsModelSupported(requestedModel)
		if compact && result.Supported {
			if compactModel, compactMatched := account.ResolveCompactMappedModel(result.Model); compactMatched {
				if trimmed := strings.TrimSpace(compactModel); trimmed != "" {
					result.Model = trimmed
				}
				result.Matched = true
				result.Source = "account"
			}
		}
	}

	return result
}

func ResolveEffectiveMappedModel(ctx context.Context, settingService *SettingService, account *Account, requestedModel string, compact bool) string {
	routing := ResolveEffectiveModelRouting(ctx, settingService, account, requestedModel, compact)
	if model := strings.TrimSpace(routing.Model); model != "" {
		return model
	}
	return strings.TrimSpace(requestedModel)
}

func IsEffectiveModelSupported(ctx context.Context, settingService *SettingService, account *Account, requestedModel string, compact bool) bool {
	return ResolveEffectiveModelRouting(ctx, settingService, account, requestedModel, compact).Supported
}

func accountHasExplicitModelRouting(account *Account, compact bool) bool {
	if account == nil || account.Credentials == nil {
		return false
	}
	if credentialStringMapHasEntries(account.Credentials["model_mapping"]) {
		return true
	}
	if len(stringsFromRawSlice(account.Credentials["model_whitelist"])) > 0 {
		return true
	}
	if compact {
		if credentialStringMapHasEntries(account.Credentials["compact_model_mapping"]) {
			return true
		}
	}
	return false
}

func hasModelRoutingConfigForAccount(ctx context.Context, settingService *SettingService, account *Account) bool {
	return accountHasExplicitModelRouting(account, false)
}
