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

	cfg, hasSystemConfig := platformModelRoutingConfigForAccount(ctx, settingService, account)
	if !hasSystemConfig {
		if account != nil {
			result.Model = strings.TrimSpace(account.GetMappedModel(requestedModel))
			if result.Model == "" {
				result.Model = requestedModel
			}
			result.Supported = account.IsModelSupported(requestedModel)
		}
		return result
	}
	if platformModelRoutingConfigUnavailable(cfg) {
		result.Supported = false
		result.Matched = false
		result.Source = "system_unavailable"
		return result
	}

	mappedModel, matched, hasNormalRules := resolvePlatformModelRoutingBase(accountPlatform(account), cfg, requestedModel)
	if hasNormalRules {
		result.Source = "system"
		result.Supported = matched
		result.Matched = matched
		if matched && strings.TrimSpace(mappedModel) != "" {
			result.Model = strings.TrimSpace(mappedModel)
		}
	} else if account != nil {
		result.Model = strings.TrimSpace(account.GetMappedModel(requestedModel))
		if result.Model == "" {
			result.Model = requestedModel
		}
		result.Supported = account.IsModelSupported(requestedModel)
	}

	if compact && result.Supported {
		if compactModel, compactMatched := resolveRequestedModelInMapping(cfg.CompactModelMapping, result.Model); compactMatched {
			if trimmed := strings.TrimSpace(compactModel); trimmed != "" {
				result.Model = trimmed
			}
			result.Matched = true
			result.Source = "system"
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
	if _, ok := account.Credentials["model_mapping"]; ok {
		return true
	}
	if _, ok := account.Credentials["model_whitelist"]; ok {
		return true
	}
	if compact {
		if _, ok := account.Credentials["compact_model_mapping"]; ok {
			return true
		}
	}
	return false
}

func platformModelRoutingConfigForAccount(ctx context.Context, settingService *SettingService, account *Account) (DefaultAccountModelConfig, bool) {
	platform := accountPlatform(account)
	if platform == "" || settingService == nil {
		return DefaultAccountModelConfig{}, false
	}
	all := settingService.GetPlatformModelRoutingConfig(ctx)
	if len(all) == 0 {
		return DefaultAccountModelConfig{}, false
	}
	if _, unavailable := all[platformModelRoutingUnavailablePlatformKey]; unavailable {
		return DefaultAccountModelConfig{
			ModelWhitelist: []string{platformModelRoutingUnavailablePlatformKey},
		}, true
	}
	cfg, ok := all[strings.ToLower(strings.TrimSpace(platform))]
	if !ok {
		cfg, ok = all[strings.TrimSpace(platform)]
	}
	if !ok {
		return DefaultAccountModelConfig{}, false
	}
	cfg = normalizeDefaultAccountModelConfig(cfg)
	return cfg, hasPlatformModelRoutingConfig(cfg)
}

func hasPlatformModelRoutingConfig(cfg DefaultAccountModelConfig) bool {
	return len(cfg.ModelWhitelist) > 0 || len(cfg.ModelMapping) > 0 || len(cfg.CompactModelMapping) > 0
}

func platformModelRoutingConfigUnavailable(cfg DefaultAccountModelConfig) bool {
	return len(cfg.ModelWhitelist) == 1 &&
		cfg.ModelWhitelist[0] == platformModelRoutingUnavailablePlatformKey &&
		len(cfg.ModelMapping) == 0 &&
		len(cfg.CompactModelMapping) == 0
}

func accountPlatform(account *Account) string {
	if account == nil {
		return ""
	}
	return strings.TrimSpace(account.Platform)
}

func resolvePlatformModelRoutingBase(platform string, cfg DefaultAccountModelConfig, requestedModel string) (string, bool, bool) {
	hasWhitelistRules := len(cfg.ModelWhitelist) > 0
	if !hasWhitelistRules && len(cfg.ModelMapping) == 0 {
		return requestedModel, false, false
	}
	if mapped, matched := resolveRequestedModelInMapping(cfg.ModelMapping, requestedModel); matched {
		return mapped, true, true
	}
	if modelWhitelistSupports(platform, cfg.ModelWhitelist, requestedModel) {
		return requestedModel, true, true
	}
	normalized := normalizeRequestedModelForLookup(platform, requestedModel)
	if normalized != requestedModel {
		if mapped, matched := resolveRequestedModelInMapping(cfg.ModelMapping, normalized); matched {
			return mapped, true, true
		}
		if modelWhitelistSupports(platform, cfg.ModelWhitelist, normalized) {
			return normalized, true, true
		}
	}
	if !hasWhitelistRules {
		return requestedModel, false, false
	}
	return requestedModel, false, true
}

func modelWhitelistSupports(platform string, whitelist []string, requestedModel string) bool {
	if requestedModel == "" || len(whitelist) == 0 {
		return false
	}
	lookup := normalizeRequestedModelForLookup(platform, requestedModel)
	for _, pattern := range whitelist {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if matchWildcard(pattern, requestedModel) || (lookup != requestedModel && matchWildcard(pattern, lookup)) {
			return true
		}
	}
	return false
}
