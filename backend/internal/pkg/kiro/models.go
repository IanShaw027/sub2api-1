package kiro

import (
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
		ID:          "claude-sonnet-4-5-20250929-thinking",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.5 (Thinking)",
		CreatedAt:   "2025-09-29T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-5-20251101",
		Type:        "model",
		DisplayName: "Claude Opus 4.5",
		CreatedAt:   "2025-11-01T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-5-20251101-thinking",
		Type:        "model",
		DisplayName: "Claude Opus 4.5 (Thinking)",
		CreatedAt:   "2025-11-01T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-4-6",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.6",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-sonnet-4-6-thinking",
		Type:        "model",
		DisplayName: "Claude Sonnet 4.6 (Thinking)",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-6",
		Type:        "model",
		DisplayName: "Claude Opus 4.6",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-opus-4-6-thinking",
		Type:        "model",
		DisplayName: "Claude Opus 4.6 (Thinking)",
		CreatedAt:   "2026-02-06T00:00:00Z",
	},
	{
		ID:          "claude-haiku-4-5-20251001",
		Type:        "model",
		DisplayName: "Claude Haiku 4.5",
		CreatedAt:   "2025-10-01T00:00:00Z",
	},
	{
		ID:          "claude-haiku-4-5-20251001-thinking",
		Type:        "model",
		DisplayName: "Claude Haiku 4.5 (Thinking)",
		CreatedAt:   "2025-10-01T00:00:00Z",
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
	"claude-sonnet-4":                     "claude-sonnet-4.6",
	"claude-sonnet-4-thinking":            "claude-sonnet-4.6-thinking",
	"claude-sonnet-4-5":                   "claude-sonnet-4.5",
	"claude-sonnet-4-5-thinking":          "claude-sonnet-4.5-thinking",
	"claude-sonnet-4.5":                   "claude-sonnet-4.5",
	"claude-sonnet-4.5-thinking":          "claude-sonnet-4.5-thinking",
	"claude-sonnet-4-5-20250929":          "claude-sonnet-4.5",
	"claude-sonnet-4-5-20250929-thinking": "claude-sonnet-4.5-thinking",
	"claude-sonnet-4-6":                   "claude-sonnet-4.6",
	"claude-sonnet-4-6-thinking":          "claude-sonnet-4.6-thinking",
	"claude-sonnet-4.6":                   "claude-sonnet-4.6",
	"claude-sonnet-4.6-thinking":          "claude-sonnet-4.6-thinking",
	"claude-opus-4":                       "claude-opus-4.6",
	"claude-opus-4-thinking":              "claude-opus-4.6-thinking",
	"claude-opus-4-5":                     "claude-opus-4.5",
	"claude-opus-4-5-thinking":            "claude-opus-4.5-thinking",
	"claude-opus-4.5":                     "claude-opus-4.5",
	"claude-opus-4.5-thinking":            "claude-opus-4.5-thinking",
	"claude-opus-4-5-20251101":            "claude-opus-4.5",
	"claude-opus-4-5-20251101-thinking":   "claude-opus-4.5-thinking",
	"claude-opus-4-6":                     "claude-opus-4.6",
	"claude-opus-4-6-thinking":            "claude-opus-4.6-thinking",
	"claude-opus-4.6":                     "claude-opus-4.6",
	"claude-opus-4.6-thinking":            "claude-opus-4.6-thinking",
	"claude-haiku-4":                      "claude-haiku-4.5",
	"claude-haiku-4-thinking":             "claude-haiku-4.5-thinking",
	"claude-haiku-4-5":                    "claude-haiku-4.5",
	"claude-haiku-4-5-thinking":           "claude-haiku-4.5-thinking",
	"claude-haiku-4.5":                    "claude-haiku-4.5",
	"claude-haiku-4.5-thinking":           "claude-haiku-4.5-thinking",
	"claude-haiku-4-5-20251001":           "claude-haiku-4.5",
	"claude-haiku-4-5-20251001-thinking":  "claude-haiku-4.5-thinking",
}

func MapModel(model string) string {
	return kiroModelAliases[normalizeKiroModelAlias(model)]
}

func normalizeKiroModelAlias(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}
