package kiro

import "testing"

func TestDefaultModels_IncludeClaude45To47Variants(t *testing.T) {
	required := map[string]bool{
		"claude-haiku-4-6":              false,
		"claude-haiku-4-6-1m":           false,
		"claude-sonnet-4-7":             false,
		"claude-sonnet-4-7-thinking-1m": false,
		"claude-opus-4-7":               false,
		"claude-opus-4-7-thinking":      false,
		"claude-opus-4-7-thinking-1m":   false,
		"claude-haiku-4-5-20251001-1m":  false,
		"claude-sonnet-4-5-20250929-1m": false,
		"claude-opus-4-5-20251101-1m":   false,
	}
	for _, model := range DefaultModels {
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
		"claude-sonnet-4-5-20250929-thinking": "claude-sonnet-4.5-thinking",
		"claude-sonnet-4-5-20250929-1m":       "claude-sonnet-4.5-1m",
		"claude-sonnet-4-6-1m-thinking":       "claude-sonnet-4.6-thinking-1m",
		"claude-sonnet-4-7-thinking-1m":       "claude-sonnet-4.7-thinking-1m",
		"claude-haiku-4-6":                    "claude-haiku-4.6",
		"claude-haiku-4.7-1m":                 "claude-haiku-4.7-1m",
		"claude-opus-4.6":                     "claude-opus-4.6",
		"claude-opus-4-6-20260205":            "claude-opus-4.6",
		"claude-opus-4-6-20260205-context-1m": "claude-opus-4.6-1m",
		"claude-opus-4-7":                     "claude-opus-4.7",
		"claude-opus-4.7-thinking":            "claude-opus-4.7-thinking",
		"claude-haiku-4-5-20251001":           "claude-haiku-4.5",
	}
	for input, want := range tests {
		if got := MapModel(input); got != want {
			t.Fatalf("MapModel(%q) = %q, want %q", input, got, want)
		}
	}
}
