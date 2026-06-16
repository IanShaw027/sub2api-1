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

	if cfg, hasPlatformDefault := platformDefaultModelRoutingConfigForAccount(ctx, settingService, account); hasPlatformDefault {
		mappedModel, matched, hasNormalRules := resolvePlatformModelRoutingBase(accountPlatform(account), cfg, requestedModel)
		if hasNormalRules {
			result.Source = "platform_default"
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
				result.Source = "platform_default"
			}
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
	if accountHasExplicitModelRouting(account, false) {
		return true
	}
	_, ok := platformDefaultModelRoutingConfigForAccount(ctx, settingService, account)
	return ok
}

func platformDefaultModelRoutingConfigForAccount(ctx context.Context, settingService *SettingService, account *Account) (DefaultAccountModelConfig, bool) {
	platform := accountPlatform(account)
	if platform == "" || settingService == nil {
		return DefaultAccountModelConfig{}, false
	}
	all := settingService.GetPlatformDefaultAccountModelConfig(ctx)
	if len(all) == 0 {
		return DefaultAccountModelConfig{}, false
	}
	cfg, ok := all[strings.ToLower(strings.TrimSpace(platform))]
	if !ok {
		cfg, ok = all[strings.TrimSpace(platform)]
	}
	if !ok {
		return DefaultAccountModelConfig{}, false
	}
	cfg = normalizeDefaultAccountModelConfig(cfg)
	if strings.EqualFold(platform, PlatformKiro) && len(cfg.KiroSubscriptionTypeModelMap) > 0 {
		subscriptionType := normalizeKiroSubscriptionTypeKey(resolveKiroSubscriptionTypeFromCredentials(account.Credentials))
		if subscriptionType != "" {
			if variantCfg, ok := cfg.KiroSubscriptionTypeModelMap[subscriptionType]; ok {
				cfg = mergeKiroSubscriptionRuntimeModelConfig(cfg, variantCfg)
			}
		}
	}
	cfg = normalizePlatformModelRoutingConfigEntry(cfg)
	return cfg, hasPlatformModelRoutingConfig(cfg)
}

func accountPlatform(account *Account) string {
	if account == nil {
		return ""
	}
	return strings.TrimSpace(account.Platform)
}

func hasPlatformModelRoutingConfig(cfg DefaultAccountModelConfig) bool {
	return len(cfg.ModelWhitelist) > 0 || len(cfg.ModelMapping) > 0 || len(cfg.CompactModelMapping) > 0
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

func mergeKiroSubscriptionRuntimeModelConfig(base, variant DefaultAccountModelConfig) DefaultAccountModelConfig {
	variant = normalizeDefaultAccountModelConfig(variant)
	out := base
	if len(variant.ModelWhitelist) > 0 {
		out.ModelWhitelist = cloneStringSlice(variant.ModelWhitelist)
	}
	if len(variant.ModelMapping) > 0 {
		out.ModelMapping = mergeStringMaps(out.ModelMapping, variant.ModelMapping)
	}
	if len(variant.CompactModelMapping) > 0 {
		out.CompactModelMapping = mergeStringMaps(out.CompactModelMapping, variant.CompactModelMapping)
	}
	return out
}

func mergeStringMaps(base, override map[string]string) map[string]string {
	out := copyStringMap(base)
	if out == nil {
		out = map[string]string{}
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}
