package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/singleflight"
)

type cachedPlatformModelRoutingConfig struct {
	config    map[string]DefaultAccountModelConfig
	expiresAt int64
}

var platformDefaultAccountModelConfigCache atomic.Value
var platformDefaultAccountModelConfigSF singleflight.Group

const platformModelRoutingConfigCacheTTL = 60 * time.Second
const platformModelRoutingConfigErrorTTL = 5 * time.Second
const platformModelRoutingConfigDBTimeout = 5 * time.Second

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
		if defaultAccountModelConfigIsEmpty(normalized) {
			continue
		}
		out[platform] = normalized
	}
	return out
}

func normalizePlatformModelRoutingConfigEntry(cfg DefaultAccountModelConfig) DefaultAccountModelConfig {
	return DefaultAccountModelConfig{
		ModelWhitelist:      normalizeModelList(cfg.ModelWhitelist),
		ModelMapping:        normalizeStringMap(cfg.ModelMapping),
		CompactModelMapping: normalizeStringMap(cfg.CompactModelMapping),
	}
}

func normalizeDefaultAccountModelConfig(cfg DefaultAccountModelConfig) DefaultAccountModelConfig {
	normalized := DefaultAccountModelConfig{
		ModelWhitelist:           normalizeModelList(cfg.ModelWhitelist),
		ModelMapping:             normalizeStringMap(cfg.ModelMapping),
		CompactModelMapping:      normalizeStringMap(cfg.CompactModelMapping),
		TempUnschedulableEnabled: cfg.TempUnschedulableEnabled,
		TempUnschedulableRules:   normalizeDefaultTempUnschedulableRules(cfg.TempUnschedulableRules),
		CustomErrorCodesEnabled:  cfg.CustomErrorCodesEnabled,
		CustomErrorCodes:         normalizeDefaultCustomErrorCodes(cfg.CustomErrorCodes),
	}
	if len(cfg.KiroSubscriptionTypeModelMap) > 0 {
		normalized.KiroSubscriptionTypeModelMap = normalizeKiroSubscriptionTypeModelConfig(cfg.KiroSubscriptionTypeModelMap)
	}
	return normalized
}

func normalizeDefaultTempUnschedulableRules(rules []TempUnschedulableRule) []TempUnschedulableRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]TempUnschedulableRule, 0, len(rules))
	for _, rule := range rules {
		// keywords 可空（纯错误码匹配）；仅校验 error_code / duration_minutes。
		if rule.ErrorCode < 100 || rule.ErrorCode > 599 || rule.DurationMinutes <= 0 {
			continue
		}
		keywords := make([]string, 0, len(rule.Keywords))
		for _, kw := range rule.Keywords {
			if kw = strings.TrimSpace(kw); kw != "" {
				keywords = append(keywords, kw)
			}
		}
		if len(keywords) == 0 {
			keywords = nil
		}
		out = append(out, TempUnschedulableRule{
			ErrorCode:       rule.ErrorCode,
			Keywords:        keywords,
			DurationMinutes: rule.DurationMinutes,
			Description:     strings.TrimSpace(rule.Description),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeDefaultCustomErrorCodes(codes []int) []int {
	if len(codes) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(codes))
	out := make([]int, 0, len(codes))
	for _, code := range codes {
		if code < 100 || code > 599 {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func defaultAccountModelConfigIsEmpty(cfg DefaultAccountModelConfig) bool {
	return len(cfg.ModelWhitelist) == 0 &&
		len(cfg.ModelMapping) == 0 &&
		len(cfg.CompactModelMapping) == 0 &&
		len(cfg.KiroSubscriptionTypeModelMap) == 0 &&
		len(cfg.TempUnschedulableRules) == 0 &&
		!cfg.TempUnschedulableEnabled &&
		len(cfg.CustomErrorCodes) == 0 &&
		!cfg.CustomErrorCodesEnabled
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
	return validatePlatformModelConfig(cfg, "INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG")
}

func hasAccountDefaultOnlyFields(cfg DefaultAccountModelConfig) bool {
	return cfg.TempUnschedulableEnabled ||
		len(cfg.TempUnschedulableRules) > 0 ||
		cfg.CustomErrorCodesEnabled ||
		len(cfg.CustomErrorCodes) > 0
}

func validatePlatformModelConfig(cfg map[string]DefaultAccountModelConfig, reason string) error {
	for platform, item := range cfg {
		platform = strings.TrimSpace(platform)
		if platform == "" {
			return infraerrors.BadRequest(reason, "platform name cannot be empty")
		}
		if err := validateDefaultAccountModelConfigWithReason(platform, item, strings.EqualFold(platform, PlatformKiro), reason); err != nil {
			return err
		}
	}
	return nil
}

func validateDefaultAccountModelConfigWithReason(platform string, cfg DefaultAccountModelConfig, allowKiroVariants bool, reason string) error {
	if err := validateDefaultModelMappingWithReason(platform, "model_mapping", cfg.ModelMapping, reason); err != nil {
		return err
	}
	if err := validateDefaultModelMappingWithReason(platform, "compact_model_mapping", cfg.CompactModelMapping, reason); err != nil {
		return err
	}
	if cfg.TempUnschedulableEnabled && len(cfg.TempUnschedulableRules) == 0 {
		return infraerrors.BadRequest(reason, platform+".temp_unschedulable_rules must contain at least one rule when temp_unschedulable_enabled is true")
	}
	if err := validateDefaultTempUnschedulableRulesWithReason(platform, cfg.TempUnschedulableRules, reason); err != nil {
		return err
	}
	if err := validateDefaultCustomErrorCodesWithReason(platform, cfg.CustomErrorCodes, reason); err != nil {
		return err
	}
	if len(cfg.KiroSubscriptionTypeModelMap) == 0 {
		return nil
	}
	if !allowKiroVariants {
		return infraerrors.BadRequest(reason, platform+".kiro_subscription_type_model_config is only supported for kiro")
	}
	for key, variant := range cfg.KiroSubscriptionTypeModelMap {
		normalizedKey := normalizeKiroSubscriptionTypeKey(key)
		if normalizedKey == "" {
			return infraerrors.BadRequest(reason, platform+".kiro_subscription_type_model_config contains an empty or unsupported subscription type key")
		}
		if len(variant.KiroSubscriptionTypeModelMap) > 0 {
			return infraerrors.BadRequest(reason, platform+".kiro_subscription_type_model_config."+normalizedKey+".kiro_subscription_type_model_config is not supported")
		}
		if hasAccountDefaultOnlyFields(variant) {
			return infraerrors.BadRequest(reason, platform+".kiro_subscription_type_model_config."+normalizedKey+" contains account-default-only fields that are not supported for subscription model config")
		}
		if err := validateDefaultModelMappingWithReason(platform+".kiro_subscription_type_model_config."+normalizedKey, "model_mapping", variant.ModelMapping, reason); err != nil {
			return err
		}
		if err := validateDefaultModelMappingWithReason(platform+".kiro_subscription_type_model_config."+normalizedKey, "compact_model_mapping", variant.CompactModelMapping, reason); err != nil {
			return err
		}
	}
	return nil
}

func validateDefaultTempUnschedulableRulesWithReason(platform string, rules []TempUnschedulableRule, reason string) error {
	for _, rule := range rules {
		if rule.ErrorCode < 100 || rule.ErrorCode > 599 {
			return infraerrors.BadRequest(reason, platform+".temp_unschedulable_rules error_code must be between 100 and 599")
		}
		if rule.DurationMinutes <= 0 {
			return infraerrors.BadRequest(reason, platform+".temp_unschedulable_rules duration_minutes must be greater than 0")
		}
	}
	return nil
}

func validateDefaultCustomErrorCodesWithReason(platform string, codes []int, reason string) error {
	for _, code := range codes {
		if code < 100 || code > 599 {
			return infraerrors.BadRequest(reason, platform+".custom_error_codes must only contain HTTP status codes between 100 and 599")
		}
	}
	return nil
}

func validateDefaultModelMappingWithReason(platform, key string, mapping map[string]string, reason string) error {
	for from, to := range mapping {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			return infraerrors.BadRequest(reason, platform+"."+key+" must only contain non-empty string-to-string mappings")
		}
		if !isValidModelMappingPattern(from) {
			return infraerrors.BadRequest(reason, platform+"."+key+" wildcard * is only allowed at the end of the request model")
		}
		if strings.Contains(to, "*") {
			return infraerrors.BadRequest(reason, platform+"."+key+" target model cannot contain wildcard *")
		}
		if strings.EqualFold(strings.TrimSpace(platform), PlatformKiro) {
			if !isCompatibleKiroModelMappingPair(from, to) {
				return infraerrors.BadRequest(reason, platform+"."+key+" cannot map across kiro model families")
			}
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
	var staleConfig map[string]DefaultAccountModelConfig
	if cached, ok := platformDefaultAccountModelConfigCache.Load().(*cachedPlatformModelRoutingConfig); ok {
		if cached != nil {
			staleConfig = clonePlatformModelConfigMap(cached.config)
			if time.Now().UnixNano() < cached.expiresAt {
				return clonePlatformModelConfigMap(cached.config)
			}
		}
	}

	result, err, _ := platformDefaultAccountModelConfigSF.Do(SettingKeyPlatformDefaultAccountModelConfig, func() (any, error) {
		if cached, ok := platformDefaultAccountModelConfigCache.Load().(*cachedPlatformModelRoutingConfig); ok {
			if cached != nil {
				staleConfig = clonePlatformModelConfigMap(cached.config)
				if time.Now().UnixNano() < cached.expiresAt {
					return clonePlatformModelConfigMap(cached.config), nil
				}
			}
		}

		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), platformModelRoutingConfigDBTimeout)
		defer cancel()

		cacheShortTTL := func(cfg map[string]DefaultAccountModelConfig) map[string]DefaultAccountModelConfig {
			platformDefaultAccountModelConfigCache.Store(&cachedPlatformModelRoutingConfig{
				config:    clonePlatformModelConfigMap(cfg),
				expiresAt: time.Now().Add(platformModelRoutingConfigErrorTTL).UnixNano(),
			})
			return clonePlatformModelConfigMap(cfg)
		}
		cacheNormalTTL := func(cfg map[string]DefaultAccountModelConfig) map[string]DefaultAccountModelConfig {
			platformDefaultAccountModelConfigCache.Store(&cachedPlatformModelRoutingConfig{
				config:    clonePlatformModelConfigMap(cfg),
				expiresAt: time.Now().Add(platformModelRoutingConfigCacheTTL).UnixNano(),
			})
			return clonePlatformModelConfigMap(cfg)
		}

		raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyPlatformDefaultAccountModelConfig)
		if err != nil {
			if !errors.Is(err, ErrSettingNotFound) && staleConfig != nil {
				return cacheShortTTL(staleConfig), nil
			}
			return cacheShortTTL(map[string]DefaultAccountModelConfig{}), nil
		}

		return cacheNormalTTL(parsePlatformDefaultAccountModelConfig(raw)), nil
	})
	if err != nil {
		return map[string]DefaultAccountModelConfig{}
	}
	if cfg, ok := result.(map[string]DefaultAccountModelConfig); ok {
		return clonePlatformModelConfigMap(cfg)
	}
	return map[string]DefaultAccountModelConfig{}
}

func applyDefaultAccountModelConfigForPlatform(platform string, credentials map[string]any, cfg DefaultAccountModelConfig) map[string]any {
	cfg = normalizeDefaultAccountModelConfig(cfg)
	if credentials == nil {
		credentials = map[string]any{}
	}
	originalKeys := explicitAccountDefaultCredentialKeys(credentials)
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

func explicitAccountDefaultCredentialKeys(credentials map[string]any) map[string]struct{} {
	keys := make(map[string]struct{}, len(credentials))
	for key, value := range credentials {
		switch key {
		case "model_mapping", "compact_model_mapping":
			if !credentialStringMapHasEntries(value) {
				continue
			}
		}
		keys[key] = struct{}{}
	}
	return keys
}

func credentialStringMapHasEntries(raw any) bool {
	switch value := raw.(type) {
	case map[string]any:
		return len(value) > 0
	case map[string]string:
		return len(value) > 0
	default:
		return false
	}
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
	// 临时不可调度 / 自定义错误码默认值：仅在基础注入阶段处理（override 为 kiro 订阅档位的二次套用，不涉及这些账号级字段）。
	if !override {
		if keyNotExplicit(originalKeys, "temp_unschedulable_rules") {
			if rules := normalizeDefaultTempUnschedulableRules(cfg.TempUnschedulableRules); len(rules) > 0 {
				out["temp_unschedulable_rules"] = tempUnschedulableRulesToAnySlice(rules)
			}
		}
		if keyNotExplicit(originalKeys, "temp_unschedulable_enabled") && cfg.TempUnschedulableEnabled {
			out["temp_unschedulable_enabled"] = true
		}
		if keyNotExplicit(originalKeys, "custom_error_codes") {
			if codes := normalizeDefaultCustomErrorCodes(cfg.CustomErrorCodes); len(codes) > 0 {
				out["custom_error_codes"] = intsToAnySlice(codes)
			}
		}
		if keyNotExplicit(originalKeys, "custom_error_codes_enabled") && cfg.CustomErrorCodesEnabled {
			out["custom_error_codes_enabled"] = true
		}
	}
	return out
}

func tempUnschedulableRulesToAnySlice(rules []TempUnschedulableRule) []any {
	if len(rules) == 0 {
		return nil
	}
	out := make([]any, 0, len(rules))
	for _, rule := range rules {
		entry := map[string]any{
			"error_code":       rule.ErrorCode,
			"duration_minutes": rule.DurationMinutes,
		}
		if len(rule.Keywords) > 0 {
			kws := make([]any, 0, len(rule.Keywords))
			for _, kw := range rule.Keywords {
				kws = append(kws, kw)
			}
			entry["keywords"] = kws
		}
		if rule.Description != "" {
			entry["description"] = rule.Description
		}
		out = append(out, entry)
	}
	return out
}

func intsToAnySlice(values []int) []any {
	if len(values) == 0 {
		return nil
	}
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
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

func clonePlatformModelConfigMap(src map[string]DefaultAccountModelConfig) map[string]DefaultAccountModelConfig {
	if len(src) == 0 {
		return map[string]DefaultAccountModelConfig{}
	}
	out := make(map[string]DefaultAccountModelConfig, len(src))
	for platform, cfg := range src {
		out[platform] = cloneDefaultAccountModelConfig(cfg)
	}
	return out
}

func cloneDefaultAccountModelConfig(cfg DefaultAccountModelConfig) DefaultAccountModelConfig {
	cloned := DefaultAccountModelConfig{
		ModelWhitelist:           cloneStringSlice(cfg.ModelWhitelist),
		ModelMapping:             copyStringMap(cfg.ModelMapping),
		CompactModelMapping:      copyStringMap(cfg.CompactModelMapping),
		TempUnschedulableEnabled: cfg.TempUnschedulableEnabled,
		TempUnschedulableRules:   cloneTempUnschedulableRules(cfg.TempUnschedulableRules),
		CustomErrorCodesEnabled:  cfg.CustomErrorCodesEnabled,
		CustomErrorCodes:         cloneIntSlice(cfg.CustomErrorCodes),
	}
	if len(cfg.KiroSubscriptionTypeModelMap) > 0 {
		cloned.KiroSubscriptionTypeModelMap = make(map[string]DefaultAccountModelConfig, len(cfg.KiroSubscriptionTypeModelMap))
		for key, variant := range cfg.KiroSubscriptionTypeModelMap {
			cloned.KiroSubscriptionTypeModelMap[key] = cloneDefaultAccountModelConfig(variant)
		}
	}
	return cloned
}

func cloneTempUnschedulableRules(src []TempUnschedulableRule) []TempUnschedulableRule {
	if len(src) == 0 {
		return nil
	}
	out := make([]TempUnschedulableRule, len(src))
	for i, rule := range src {
		out[i] = rule
		out[i].Keywords = cloneStringSlice(rule.Keywords)
	}
	return out
}

func cloneIntSlice(src []int) []int {
	if len(src) == 0 {
		return nil
	}
	out := make([]int, len(src))
	copy(out, src)
	return out
}

func resolveKiroSubscriptionTypeFromCredentials(credentials map[string]any) string {
	if len(credentials) == 0 {
		return "free"
	}
	for _, key := range []string{"subscription_type", "plan_name", "plan_tier"} {
		if raw, ok := credentials[key]; ok {
			if normalized := normalizeKiroSubscriptionTypeKey(fmt.Sprint(raw)); normalized != "" {
				return normalized
			}
		}
	}
	return "free"
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
