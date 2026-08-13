package kiro

import (
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

var DefaultModels = []claude.Model{
	{
		ID:          "claude-sonnet-4-5-20250929",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.5",
		CreatedAt:   "2025-09-29T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-4-5-20250929-1m",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.5 (1M)",
		CreatedAt:   "2025-09-29T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-5-20251101",
		Type:        "model",
		DisplayName: "Claude Opus 4.5",
		CreatedAt:   "2025-11-01T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-5-20251101-1m",
		Type:        "model",
		DisplayName: "Claude Opus 4.5 (1M)",
		CreatedAt:   "2025-11-01T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-4-6",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.6",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-4-6-1m",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.6 (1M)",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-4-7",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.7",
		CreatedAt:   "2026-04-17T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-4-7-1m",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.7 (1M)",
		CreatedAt:   "2026-04-17T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-6",
		Type:        "model",
		DisplayName: "Claude Opus 4.6",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-6-1m",
		Type:        "model",
		DisplayName: "Claude Opus 4.6 (1M)",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-7",
		Type:        "model",
		DisplayName: "Claude Opus 4.7",
		CreatedAt:   "2026-04-17T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-7-1m",
		Type:        "model",
		DisplayName: "Claude Opus 4.7 (1M)",
		CreatedAt:   "2026-04-17T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-8",
		Type:        "model",
		DisplayName: "Claude Opus 4.8",
		CreatedAt:   "2026-06-20T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-8-1m",
		Type:        "model",
		DisplayName: "Claude Opus 4.8 (1M)",
		CreatedAt:   "2026-06-20T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-5",
		Type:        "model",
		DisplayName: "Claude Sonnet 5",
		CreatedAt:   "2026-07-01T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-5-1m",
		Type:        "model",
		DisplayName: "Claude Sonnet 5 (1M)",
		CreatedAt:   "2026-07-01T00:00:00Z",
	},
	{
		ID:          "claude-opus-5",
		Type:        "model",
		DisplayName: "Claude Opus 5",
		CreatedAt:   "2026-07-24T00:00:00Z",
	},
	{
		ID:          "claude-opus-5-1m",
		Type:        "model",
		DisplayName: "Claude Opus 5 (1M)",
		CreatedAt:   "2026-07-24T00:00:00Z",
	},
	{
		ID:          "claude-haiku-4-5-20251001",
		Type:        "model",
		DisplayName: "Claude Haiku 4.5",
		CreatedAt:   "2025-10-01T00:00:00Z",
	},
	{
		ID:          "claude-haiku-4-5-20251001-1m",
		Type:        "model",
		DisplayName: "Claude Haiku 4.5 (1M)",
		CreatedAt:   "2025-10-01T00:00:00Z",
	},
	{
		ID:          "claude-haiku-4-6",
		Type:        "model",
		DisplayName: "Claude Haiku 4.6",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-haiku-4-6-1m",
		Type:        "model",
		DisplayName: "Claude Haiku 4.6 (1M)",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-haiku-4-7",
		Type:        "model",
		DisplayName: "Claude Haiku 4.7",
		CreatedAt:   "2026-04-17T00:00:00Z",
	},
	{
		ID:          "claude-haiku-4-7-1m",
		Type:        "model",
		DisplayName: "Claude Haiku 4.7 (1M)",
		CreatedAt:   "2026-04-17T00:00:00Z",
	},
}

func DefaultModelIDs() []string {
	out := make([]string, 0, len(DefaultModels))
	for _, model := range DefaultModels {
		out = append(out, model.ID)
	}
	return out
}

var kiroModelAliases = map[string]string{
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

var kiroClaudeModelPattern = regexp.MustCompile(`^claude-(haiku|sonnet|opus)-(\d+)(?:[.-](\d+))?(?:-\d{8})?$`)

var kiroOneMillionContextModels = map[string]struct{}{
	"claude-sonnet-4.6": {},
	"claude-sonnet-4.7": {},
	"claude-sonnet-5":   {},
	"claude-opus-4.6":   {},
	"claude-opus-4.7":   {},
	"claude-opus-4.8":   {},
	"claude-opus-5":     {},
}

var kiroExtendedThinkingModels = map[string]struct{}{
	"claude-opus-4.5":   {},
	"claude-opus-4.6":   {},
	"claude-opus-4.7":   {},
	"claude-opus-4.8":   {},
	"claude-opus-5":     {},
	"claude-sonnet-4.5": {},
	"claude-sonnet-4.6": {},
	"claude-sonnet-4.7": {},
	"claude-sonnet-5":   {},
	"claude-haiku-4.5":  {},
}

func MapModel(model string) string {
	normalized := normalizeKiroModelAlias(model)
	if mapped := kiroModelAliases[normalized]; mapped != "" {
		return mapped
	}

	base, oneMillionContext := stripKiroVariantSuffixes(normalized)
	matches := kiroClaudeModelPattern.FindStringSubmatch(base)
	if len(matches) == 0 {
		return ""
	}

	family := matches[1]
	major := matches[2]
	minor := ""
	if len(matches) > 3 {
		minor = matches[3]
	}

	mapped := "claude-" + family + "-" + major
	if minor != "" {
		mapped += "." + minor
	}
	if oneMillionContext {
		return mapped
	}
	return mapped
}

func SupportsOneMillionContextModel(model string) bool {
	mapped := MapModel(model)
	if mapped == "" {
		return false
	}
	_, ok := kiroOneMillionContextModels[mapped]
	return ok
}

func SupportsExtendedThinking(model string) bool {
	mapped := MapModel(model)
	if mapped == "" {
		return false
	}
	_, ok := kiroExtendedThinkingModels[mapped]
	return ok
}

func normalizeKiroModelAlias(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	normalized = strings.TrimPrefix(normalized, "models/")
	normalized = strings.TrimSuffix(normalized, "-v1:0")
	return normalized
}

func stripKiroVariantSuffixes(model string) (base string, oneMillionContext bool) {
	base = model
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
