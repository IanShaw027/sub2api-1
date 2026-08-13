package service

import (
	"regexp"
	"strings"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
)

var kiroClaudeModelPattern = regexp.MustCompile(`^claude-(haiku|sonnet|opus)-(\d+)(?:[.-](\d+))?(?:-\d{8})?$`)

func normalizeKiroModelName(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, "*"); idx >= 0 {
		return normalizeKiroModelName(value[:idx]) + "*"
	}

	value = strings.TrimPrefix(value, "models/")
	value = strings.TrimSuffix(value, "-v1:0")
	base, oneMillionContext := stripKiroModelVariantSuffixes(value)
	directAliases := map[string]string{
		"claude-sonnet-4":            "claude-sonnet-4.6",
		"claude-sonnet-4-5":          "claude-sonnet-4.5",
		"claude-sonnet-4.5":          "claude-sonnet-4.5",
		"claude-sonnet-4-5-20250929": "claude-sonnet-4.5",
		"claude-sonnet-4-6":          "claude-sonnet-4.6",
		"claude-sonnet-4.6":          "claude-sonnet-4.6",
		"claude-sonnet-4-7":          "claude-sonnet-4.7",
		"claude-sonnet-4.7":          "claude-sonnet-4.7",
		"claude-sonnet-5":            "claude-sonnet-5",
		"claude-opus-4":              "claude-opus-4.6",
		"claude-opus-4-5":            "claude-opus-4.5",
		"claude-opus-4.5":            "claude-opus-4.5",
		"claude-opus-4-5-20251101":   "claude-opus-4.5",
		"claude-opus-4-6":            "claude-opus-4.6",
		"claude-opus-4.6":            "claude-opus-4.6",
		"claude-opus-4-7":            "claude-opus-4.7",
		"claude-opus-4.7":            "claude-opus-4.7",
		"claude-opus-4-8":            "claude-opus-4.8",
		"claude-opus-4.8":            "claude-opus-4.8",
		"claude-opus-5":              "claude-opus-5",
		"claude-haiku-4":             "claude-haiku-4.5",
		"claude-haiku-4-5":           "claude-haiku-4.5",
		"claude-haiku-4.5":           "claude-haiku-4.5",
		"claude-haiku-4-5-20251001":  "claude-haiku-4.5",
		"claude-haiku-4-6":           "claude-haiku-4.6",
		"claude-haiku-4.6":           "claude-haiku-4.6",
		"claude-haiku-4-7":           "claude-haiku-4.7",
		"claude-haiku-4.7":           "claude-haiku-4.7",
	}
	if mapped, ok := directAliases[base]; ok {
		return mapped
	}

	if matches := kiroClaudeModelPattern.FindStringSubmatch(base); len(matches) >= 3 {
		mapped := "claude-" + matches[1] + "-" + matches[2]
		if len(matches) > 3 && matches[3] != "" {
			mapped += "." + matches[3]
		}
		return mapped
	}

	if oneMillionContext {
		return base
	}
	return base
}

func kiroModelFamily(model string) string {
	base, _ := stripKiroModelVariantSuffixes(normalizeKiroModelName(model))
	if base == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(base, "claude-haiku-"):
		return "haiku"
	case strings.HasPrefix(base, "claude-sonnet-"):
		return "sonnet"
	case strings.HasPrefix(base, "claude-opus-"):
		return "opus"
	default:
		return ""
	}
}

func normalizeKiroModelMapping(raw map[string]string) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	mapping := make(map[string]string, len(raw))
	for from, to := range raw {
		from = normalizeKiroModelName(from)
		to = normalizeKiroModelName(to)
		if from == "" || to == "" {
			continue
		}
		if !isCompatibleKiroModelMappingPair(from, to) {
			continue
		}
		mapping[from] = to
	}
	if len(mapping) == 0 {
		return nil
	}
	return mapping
}

func isCompatibleKiroModelMappingPair(from, to string) bool {
	fromFamily := kiroModelFamily(from)
	toFamily := kiroModelFamily(to)
	// A custom source alias whose family cannot be resolved is passed through as
	// long as the target resolves to a real kiro model. Known source families
	// must still match the target family to keep the cross-family guard.
	if fromFamily == "" {
		return toFamily != ""
	}
	return fromFamily == toFamily
}

func stripKiroModelVariantSuffixes(value string) (base string, oneMillionContext bool) {
	base = value
	for {
		switch {
		case strings.HasSuffix(base, "[1m]"):
			oneMillionContext = true
			base = strings.TrimSuffix(base, "[1m]")
		case strings.HasSuffix(base, "-1m-context"):
			oneMillionContext = true
			base = strings.TrimSuffix(base, "-1m-context")
		case strings.HasSuffix(base, "-context-1m"):
			oneMillionContext = true
			base = strings.TrimSuffix(base, "-context-1m")
		case strings.HasSuffix(base, "-1m"):
			oneMillionContext = true
			base = strings.TrimSuffix(base, "-1m")
		default:
			return base, oneMillionContext
		}
	}
}

func (a *Account) hasExplicitModelMappingEntries() bool {
	if a == nil || a.Credentials == nil {
		return false
	}
	switch raw := a.Credentials["model_mapping"].(type) {
	case map[string]any:
		return len(raw) > 0
	case map[string]string:
		return len(raw) > 0
	default:
		return false
	}
}

func mapKiroModel(account *Account, requestedModel string) string {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return ""
	}
	effectiveModel := requestedModel
	if account != nil {
		if mappedModel, matched := resolveKiroMappedModel(account, requestedModel); matched {
			effectiveModel = mappedModel
		} else if account.hasExplicitModelMappingEntries() {
			return ""
		}
	}
	return kiropkg.MapModel(effectiveModel)
}

func resolveKiroMappedModel(account *Account, requestedModel string) (string, bool) {
	if account == nil {
		return requestedModel, false
	}
	if mappedModel, matched := account.ResolveMappedModel(requestedModel); matched {
		return mappedModel, true
	}

	requestedKiroModel := kiropkg.MapModel(requestedModel)
	if requestedKiroModel == "" {
		return requestedModel, false
	}

	for from, to := range account.GetModelMapping() {
		if kiropkg.MapModel(from) == requestedKiroModel {
			return to, true
		}
	}
	return requestedModel, false
}

func stringsFromRawSlice(raw any) []string {
	switch values := raw.(type) {
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				if trimmed := strings.TrimSpace(text); trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
		return result
	case []string:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	default:
		return nil
	}
}
