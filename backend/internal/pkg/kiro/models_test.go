package kiro

import (
	"strings"
	"testing"
)

func TestDefaultModels_IncludeClaude45To48Variants(t *testing.T) {
	required := map[string]bool{
		"claude-haiku-4-6":              false,
		"claude-haiku-4-6-1m":           false,
		"claude-sonnet-4-7":             false,
		"claude-sonnet-4-7-1m":          false,
		"claude-opus-4-7":               false,
		"claude-opus-4-7-1m":            false,
		"claude-opus-4-8":               false,
		"claude-opus-4-8-1m":            false,
		"claude-haiku-4-5-20251001-1m":  false,
		"claude-sonnet-4-5-20250929-1m": false,
		"claude-opus-4-5-20251101-1m":   false,
	}
	for _, model := range DefaultModels {
		if model.ID != "" && containsThinkingSuffix(model.ID) {
			t.Fatalf("DefaultModels should not expose thinking variant %q", model.ID)
		}
		if _, ok := required[model.ID]; ok {
			required[model.ID] = true
		}
	}
	for id, found := range required {
		if !found {
			t.Fatalf("DefaultModels does not include %q", id)
		}
	}
}

func TestMapModel_MapsPublishedAliases(t *testing.T) {
	tests := map[string]string{
		"claude-sonnet-4":                     "claude-sonnet-4.6",
		"claude-sonnet-4-5-20250929":          "claude-sonnet-4.5",
		"claude-sonnet-4-5-20250929-1m":       "claude-sonnet-4.5",
		"claude-haiku-4-6":                    "claude-haiku-4.6",
		"claude-haiku-4.7-1m":                 "claude-haiku-4.7",
		"claude-opus-4.6":                     "claude-opus-4.6",
		"claude-opus-4-6-20260205":            "claude-opus-4.6",
		"claude-opus-4-6-20260205-context-1m": "claude-opus-4.6",
		"claude-opus-4.7[1m]":                 "claude-opus-4.7",
		"claude-opus-4-7[1m]":                 "claude-opus-4.7",
		"claude-opus-4-7":                     "claude-opus-4.7",
		"claude-opus-4-8":                     "claude-opus-4.8",
		"claude-opus-4.8-1m":                  "claude-opus-4.8",
		"claude-haiku-4-5-20251001":           "claude-haiku-4.5",
	}
	for input, want := range tests {
		if got := MapModel(input); got != want {
			t.Fatalf("MapModel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMapModel_SupportsNewMajorsForKnownFamilies(t *testing.T) {
	tests := map[string]string{
		"claude-sonnet-5":     "claude-sonnet-5",
		"claude-sonnet-5-2":   "claude-sonnet-5.2",
		"claude-opus-5":       "claude-opus-5",
		"claude-haiku-5-1m":   "claude-haiku-5",
		"claude-opus-5.1[1m]": "claude-opus-5.1",
	}
	for input, want := range tests {
		if got := MapModel(input); got != want {
			t.Fatalf("MapModel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMapModel_RejectsUnknownClaudeFamilies(t *testing.T) {
	tests := []string{
		"claude-unknown-9-9",
		"claude-snonet-5",
		"claude-fable-5",
	}
	for _, input := range tests {
		if got := MapModel(input); got != "" {
			t.Fatalf("MapModel(%q) = %q, want empty string", input, got)
		}
	}
}

func TestMapModel_RejectsThinkingSuffixAliases(t *testing.T) {
	tests := []string{
		"claude-sonnet-4-5-20250929-thinking",
		"claude-sonnet-4-6-1m-thinking",
		"claude-sonnet-4-7-thinking-1m",
		"claude-opus-4.7-thinking",
	}
	for _, input := range tests {
		if got := MapModel(input); got != "" {
			t.Fatalf("MapModel(%q) = %q, want empty string", input, got)
		}
	}
}

func TestSupportsOneMillionContextModel(t *testing.T) {
	tests := map[string]bool{
		"claude-sonnet-4-6":          true,
		"claude-sonnet-4.6-1m":       true,
		"claude-opus-4.6":            true,
		"claude-opus-4.7[1m]":        true,
		"claude-opus-4-8-1m":         true,
		"claude-sonnet-4-5-20250929": false,
		"claude-haiku-4.6":           false,
	}
	for input, want := range tests {
		if got := SupportsOneMillionContextModel(input); got != want {
			t.Fatalf("SupportsOneMillionContextModel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestSupportsExtendedThinking_IncludesRequiredClaude46To48Models(t *testing.T) {
	tests := map[string]bool{
		"claude-sonnet-4-6":  true,
		"claude-opus-4-6":    true,
		"claude-opus-4-7":    true,
		"claude-opus-4-8":    true,
		"claude-haiku-4.6":   false,
		"totally-not-claude": false,
	}
	for input, want := range tests {
		if got := SupportsExtendedThinking(input); got != want {
			t.Fatalf("SupportsExtendedThinking(%q) = %v, want %v", input, got, want)
		}
	}
}

func containsThinkingSuffix(modelID string) bool {
	return strings.Contains(modelID, "-thinking")
}
