package service

import (
	"context"
	"encoding/json"
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
		normalized := DefaultAccountModelConfig{
			ModelWhitelist:      normalizeModelList(cfg.ModelWhitelist),
			ModelMapping:        normalizeStringMap(cfg.ModelMapping),
			CompactModelMapping: normalizeStringMap(cfg.CompactModelMapping),
		}
		if len(normalized.ModelWhitelist) == 0 && len(normalized.ModelMapping) == 0 && len(normalized.CompactModelMapping) == 0 {
			continue
		}
		out[platform] = normalized
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
		if err := validateDefaultModelMapping(platform, "model_mapping", item.ModelMapping); err != nil {
			return err
		}
		if err := validateDefaultModelMapping(platform, "compact_model_mapping", item.CompactModelMapping); err != nil {
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
	if credentials == nil {
		credentials = map[string]any{}
	}
	out := make(map[string]any, len(credentials)+2)
	for k, v := range credentials {
		out[k] = v
	}
	if _, exists := out["model_mapping"]; !exists {
		mapping := buildDefaultModelMapping(cfg)
		if len(mapping) > 0 {
			out["model_mapping"] = stringMapToAnyMap(mapping)
		}
	}
	if _, exists := out["compact_model_mapping"]; !exists && len(cfg.CompactModelMapping) > 0 {
		out["compact_model_mapping"] = copyStringMap(cfg.CompactModelMapping)
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
	mapping := make(map[string]string, len(cfg.ModelWhitelist)+len(cfg.ModelMapping))
	for _, model := range cfg.ModelWhitelist {
		model = strings.TrimSpace(model)
		if model == "" || strings.Contains(model, "*") {
			continue
		}
		mapping[model] = model
	}
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
