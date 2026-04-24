package kiro

import "testing"

func TestMapModel_RejectsUnsupportedVersion(t *testing.T) {
	if got := MapModel("claude-opus-4-7"); got != "" {
		t.Fatalf("MapModel(%q) = %q, want empty", "claude-opus-4-7", got)
	}
}

func TestMapModel_MapsPublishedAliases(t *testing.T) {
	tests := map[string]string{
		"claude-sonnet-4":                     "claude-sonnet-4.6",
		"claude-sonnet-4-5-20250929":          "claude-sonnet-4.5",
		"claude-sonnet-4-5-20250929-thinking": "claude-sonnet-4.5-thinking",
		"claude-opus-4.6":                     "claude-opus-4.6",
		"claude-haiku-4-5-20251001":           "claude-haiku-4.5",
	}
	for input, want := range tests {
		if got := MapModel(input); got != want {
			t.Fatalf("MapModel(%q) = %q, want %q", input, got, want)
		}
	}
}
