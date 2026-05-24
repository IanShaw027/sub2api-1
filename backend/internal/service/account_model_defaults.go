package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func defaultAccountModelConfigJSON() string {
	return "{}"
}

func normalizePlatformDefaultAccountModelConfig(raw map[string]DefaultAccountModelConfig) map[string]DefaultAccountModelConfig {
	if len(raw) == 0 {
		return map[string]DefaultAccountModelConfig{}
	}
	out := make(map[string]DefaultAccountModelConfig, len(raw))
	for platform, cfg := range raw {
		platform = strings.TrimSpace(strings.ToLower(platform))
		if platform == "" {
			continue
		}
		normalized := normalizeDefaultAccountModelConfig(cfg)
		if len(normalized.ModelWhitelist) == 0 && len(normalized.ModelMapping) == 0 && len(normalized.CompactModelMapping) == 0 && len(normalized.KiroSubscriptionTypeModelMap) == 0 {
			continue
		}
		out[platform] = normalized
	}
	return out
}

func normalizeDefaultAccountModelConfig(cfg DefaultAccountModelConfig) DefaultAccountModelConfig {
	normalized := DefaultAccountModelConfig{
		ModelWhitelist:      normalizeModelList(cfg.ModelWhitelist),
		ModelMapping:        normalizeStringMap(cfg.ModelMapping),
		CompactModelMapping: normalizeStringMap(cfg.CompactModelMapping),
	}
	if len(cfg.KiroSubscriptionTypeModelMap) > 0 {
		normalized.KiroSubscriptionTypeModelMap = normalizeKiroSubscriptionTypeModelConfig(cfg.KiroSubscriptionTypeModelMap)
	}
	return normalized
}

func normalizeKiroSubscriptionTypeModelConfig(raw map[string]DefaultAccountModelConfig) map[string]DefaultAccountModelConfig {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]DefaultAccountModelConfig, len(raw))
	for key, cfg := range raw {
		normalizedKey := normalizeKiroSubscriptionTypeKey(key)
		if normalizedKey == "" {
			continue
		}
		normalized := DefaultAccountModelConfig{
			ModelWhitelist:      normalizeModelList(cfg.ModelWhitelist),
			ModelMapping:        normalizeStringMap(cfg.ModelMapping),
			CompactModelMapping: normalizeStringMap(cfg.CompactModelMapping),
		}
		if len(normalized.ModelWhitelist) == 0 && len(normalized.ModelMapping) == 0 && len(normalized.CompactModelMapping) == 0 {
			continue
		}
		out[normalizedKey] = normalized
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parsePlatformDefaultAccountModelConfig(raw string) map[string]DefaultAccountModelConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]DefaultAccountModelConfig{}
	}
	var cfg map[string]DefaultAccountModelConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return map[string]DefaultAccountModelConfig{}
	}
	return normalizePlatformDefaultAccountModelConfig(cfg)
}

func normalizeModelList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for from, to := range values {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			continue
		}
		out[from] = to
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func validatePlatformDefaultAccountModelConfig(cfg map[string]DefaultAccountModelConfig) error {
	for platform, item := range cfg {
		platform = strings.TrimSpace(platform)
		if platform == "" {
			return infraerrors.BadRequest("INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", "platform name cannot be empty")
		}
		if err := validateDefaultAccountModelConfig(platform, item, platform == "kiro"); err != nil {
			return err
		}
	}
	return nil
}

func validateDefaultAccountModelConfig(platform string, cfg DefaultAccountModelConfig, allowKiroVariants bool) error {
	if err := validateDefaultModelMapping(platform, "model_mapping", cfg.ModelMapping); err != nil {
		return err
	}
	if err := validateDefaultModelMapping(platform, "compact_model_mapping", cfg.CompactModelMapping); err != nil {
		return err
	}
	if len(cfg.KiroSubscriptionTypeModelMap) == 0 {
		return nil
	}
	if !allowKiroVariants {
		return infraerrors.BadRequest("INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", platform+".kiro_subscription_type_model_config is only supported for kiro")
	}
	for key, variant := range cfg.KiroSubscriptionTypeModelMap {
		normalizedKey := normalizeKiroSubscriptionTypeKey(key)
		if normalizedKey == "" {
			return infraerrors.BadRequest("INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", platform+".kiro_subscription_type_model_config contains an empty or unsupported subscription type key")
		}
		if err := validateDefaultModelMapping(platform+".kiro_subscription_type_model_config."+normalizedKey, "model_mapping", variant.ModelMapping); err != nil {
			return err
		}
		if err := validateDefaultModelMapping(platform+".kiro_subscription_type_model_config."+normalizedKey, "compact_model_mapping", variant.CompactModelMapping); err != nil {
			return err
		}
	}
	return nil
}

func validateDefaultModelMapping(platform, key string, mapping map[string]string) error {
	for from, to := range mapping {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			return infraerrors.BadRequest("INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", platform+"."+key+" must only contain non-empty string-to-string mappings")
		}
		if !isValidModelMappingPattern(from) {
			return infraerrors.BadRequest("INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", platform+"."+key+" wildcard * is only allowed at the end of the request model")
		}
		if strings.Contains(to, "*") {
			return infraerrors.BadRequest("INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", platform+"."+key+" target model cannot contain wildcard *")
		}
	}
	return nil
}

func isValidModelMappingPattern(pattern string) bool {
	starIndex := strings.Index(pattern, "*")
	if starIndex == -1 {
		return true
	}
	return starIndex == len(pattern)-1 && strings.LastIndex(pattern, "*") == starIndex
}

func encodePlatformDefaultAccountModelConfig(cfg map[string]DefaultAccountModelConfig) (string, error) {
	if err := validatePlatformDefaultAccountModelConfig(cfg); err != nil {
		return "", err
	}
	cfg = normalizePlatformDefaultAccountModelConfig(cfg)
	if len(cfg) == 0 {
		return defaultAccountModelConfigJSON(), nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *SettingService) GetPlatformDefaultAccountModelConfig(ctx context.Context) map[string]DefaultAccountModelConfig {
	if s == nil || s.settingRepo == nil {
		return map[string]DefaultAccountModelConfig{}
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyPlatformDefaultAccountModelConfig)
	if err != nil {
		return map[string]DefaultAccountModelConfig{}
	}
	return parsePlatformDefaultAccountModelConfig(raw)
}

func applyDefaultAccountModelConfig(credentials map[string]any, cfg DefaultAccountModelConfig) map[string]any {
	return applyDefaultAccountModelConfigForPlatform("", credentials, cfg)
}

func applyDefaultAccountModelConfigForPlatform(platform string, credentials map[string]any, cfg DefaultAccountModelConfig) map[string]any {
	cfg = normalizeDefaultAccountModelConfig(cfg)
	if credentials == nil {
		credentials = map[string]any{}
	}
	originalKeys := make(map[string]struct{}, len(credentials))
	for key := range credentials {
		originalKeys[key] = struct{}{}
	}
	out := applyDefaultAccountModelConfigBaseWithOriginalKeys(credentials, cfg, originalKeys, false)
	if strings.EqualFold(strings.TrimSpace(platform), PlatformKiro) && len(cfg.KiroSubscriptionTypeModelMap) > 0 {
		subscriptionType := normalizeKiroSubscriptionTypeKey(resolveKiroSubscriptionTypeFromCredentials(out))
		if subscriptionType != "" {
			if variantCfg, ok := cfg.KiroSubscriptionTypeModelMap[subscriptionType]; ok {
				out = applyDefaultAccountModelConfigBaseWithOriginalKeys(out, variantCfg, originalKeys, true)
			}
		}
	}
	return out
}

func applyDefaultAccountModelConfigBase(credentials map[string]any, cfg DefaultAccountModelConfig, override bool) map[string]any {
	originalKeys := make(map[string]struct{}, len(credentials))
	for key := range credentials {
		originalKeys[key] = struct{}{}
	}
	return applyDefaultAccountModelConfigBaseWithOriginalKeys(credentials, cfg, originalKeys, override)
}

func applyDefaultAccountModelConfigBaseWithOriginalKeys(credentials map[string]any, cfg DefaultAccountModelConfig, originalKeys map[string]struct{}, override bool) map[string]any {
	if credentials == nil {
		credentials = map[string]any{}
	}
	out := make(map[string]any, len(credentials)+2)
	for k, v := range credentials {
		out[k] = v
	}
	if keyNotExplicit(originalKeys, "model_whitelist") {
		if whitelist := normalizeDefaultModelWhitelist(cfg.ModelWhitelist); len(whitelist) > 0 {
			out["model_whitelist"] = whitelist
		}
	}
	if keyNotExplicit(originalKeys, "model_mapping") {
		mapping := buildDefaultModelMapping(cfg)
		if len(mapping) > 0 {
			if override && hasCredentialValue(out, "model_mapping") {
				out["model_mapping"] = mergeCredentialStringMap(out["model_mapping"], mapping)
			} else {
				out["model_mapping"] = stringMapToAnyMap(mapping)
			}
		}
	}
	if keyNotExplicit(originalKeys, "compact_model_mapping") {
		if len(cfg.CompactModelMapping) > 0 {
			if override && hasCredentialValue(out, "compact_model_mapping") {
				out["compact_model_mapping"] = mergeCredentialStringMap(out["compact_model_mapping"], cfg.CompactModelMapping)
			} else {
				out["compact_model_mapping"] = copyStringMap(cfg.CompactModelMapping)
			}
		}
	}
	return out
}

func keyNotExplicit(originalKeys map[string]struct{}, key string) bool {
	return originalKeys == nil || !hasKey(originalKeys, key)
}

func hasKey(values map[string]struct{}, key string) bool {
	if len(values) == 0 {
		return false
	}
	_, ok := values[key]
	return ok
}

func hasCredentialValue(credentials map[string]any, key string) bool {
	if len(credentials) == 0 {
		return false
	}
	_, exists := credentials[key]
	return exists
}

func mergeCredentialStringMap(existing any, incoming map[string]string) map[string]any {
	out := make(map[string]any, len(incoming))
	switch current := existing.(type) {
	case map[string]any:
		for key, value := range current {
			out[key] = value
		}
	case map[string]string:
		for key, value := range current {
			out[key] = value
		}
	}
	for key, value := range incoming {
		out[key] = value
	}
	return out
}

func stringMapToAnyMap(src map[string]string) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func buildDefaultModelMapping(cfg DefaultAccountModelConfig) map[string]string {
	mapping := make(map[string]string, len(cfg.ModelMapping))
	for from, to := range cfg.ModelMapping {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			continue
		}
		mapping[from] = to
	}
	if len(mapping) == 0 {
		return nil
	}
	return mapping
}

func normalizeDefaultModelWhitelist(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	result := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			result = append(result, model)
		}
	}
	return result
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func resolveKiroSubscriptionTypeFromCredentials(credentials map[string]any) string {
	if len(credentials) == 0 {
		return ""
	}
	for _, key := range []string{"subscription_type", "plan_name", "plan_tier"} {
		if raw, ok := credentials[key]; ok {
			if normalized := normalizeKiroSubscriptionTypeKey(fmt.Sprint(raw)); normalized != "" {
				return normalized
			}
		}
	}
	return ""
}

func normalizeKiroSubscriptionTypeKey(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, "+", "plus")
	switch {
	case strings.Contains(value, "free"):
		return "free"
	case strings.Contains(value, "power"):
		return "power"
	case strings.Contains(value, "proplus"), strings.Contains(value, "plus"):
		return "pro_plus"
	case strings.Contains(value, "pro"):
		return "pro"
	default:
		return value
	}
}
